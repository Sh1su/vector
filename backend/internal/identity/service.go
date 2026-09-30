// Package identity verwaltet Konten, Anmeldung, Sitzungen und Rollen (ADR-015, ADR-016,
// docs/phase-2/10-domaene-identity.md).
package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sh1su/vector/backend/internal/identity/store"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

// Sitzungsdauer (ID-02): Leerlauf 7 Tage, höchstens 30 Tage.
const (
	sessionIdle     = 7 * 24 * time.Hour
	sessionAbsolute = 30 * 24 * time.Hour
)

type Account struct {
	ID            uuid.UUID
	Email         string
	EmailVerified bool
	DisplayName   string
	Status        string
	IsAdmin       bool
}

type Service struct {
	pool       *pgxpool.Pool
	setupToken string
	log        *slog.Logger
	throttle   *throttle
}

func NewService(pool *pgxpool.Pool, setupToken string, log *slog.Logger) *Service {
	return &Service{pool: pool, setupToken: setupToken, log: log, throttle: newThrottle()}
}

// PrepareSetup erzeugt ein Einmal-Setup-Token, falls noch kein Konto existiert
// und keines konfiguriert ist, und schreibt es ins Log (ADR-015).
func (s *Service) PrepareSetup(ctx context.Context) error {
	n, err := store.New(s.pool).CountAccounts(ctx)
	if err != nil || n > 0 {
		return err
	}
	if s.setupToken == "" {
		s.setupToken = randomToken(18)
	}
	s.log.Warn("Ersteinrichtung: noch kein Konto vorhanden. Setup-Token für POST /api/v1/auth/setup", "setup_token", s.setupToken)
	return nil
}

func toAccount(a store.IdentityAccount) Account {
	return Account{ID: uuid.UUID(a.ID.Bytes), Email: a.Email, EmailVerified: a.EmailVerified, DisplayName: a.DisplayName, Status: a.Status, IsAdmin: a.IsAdmin}
}

// Setup legt das erste Administratorkonto an. Nur mit gültigem Setup-Token und
// nur solange kein Konto existiert.
func (s *Service) Setup(ctx context.Context, token, email, name, password string) (Account, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if s.setupToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(s.setupToken)) != 1 {
		return Account{}, problem.Forbidden("Setup-Token ungültig.")
	}
	if err := CheckPasswordPolicy(password); err != nil {
		return Account{}, problem.Validation(problem.FieldError{Pointer: "/password", Code: "weak_password", Message: err.Error()})
	}
	if !validEmail(email) {
		return Account{}, problem.Validation(problem.FieldError{Pointer: "/email", Code: "invalid_email"})
	}
	hash, err := HashPassword(password)
	if err != nil {
		return Account{}, err
	}
	var acc Account
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// Serialisiert parallele Setup-Versuche.
		if _, err := tx.Exec(ctx, "LOCK TABLE identity.account IN EXCLUSIVE MODE"); err != nil {
			return err
		}
		q := store.New(tx)
		n, err := q.CountAccounts(ctx)
		if err != nil {
			return err
		}
		if n > 0 {
			return problem.Conflict("Die Installation ist bereits eingerichtet.")
		}
		a, err := q.InsertAccount(ctx, store.InsertAccountParams{ID: pgUUID(kernel.NewID()), Email: email, DisplayName: name,
			Status: "active", IsAdmin: true, PasswordHash: pgtype.Text{String: hash, Valid: true}})
		acc = toAccount(a)
		return err
	})
	if err == nil {
		s.setupToken = ""
	}
	return acc, err
}

// NewSession ist das Ergebnis einer Anmeldung: Klartext-Token für das Cookie und CSRF-Token.
type NewSession struct {
	Account   Account
	Token     string
	CSRFToken string
	Expires   time.Time
}

// Login prüft E-Mail und Passwort und legt eine Web-Sitzung an (ID-02, ID-03).
func (s *Service) Login(ctx context.Context, email, password, clientIP, userAgent string) (NewSession, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	key := clientIP + "|" + email
	if wait := s.throttle.wait(key); wait > 0 {
		return NewSession{}, problem.TooManyRequests("Zu viele Fehlversuche. Bitte später erneut versuchen.")
	}
	q := store.New(s.pool)
	a, err := q.GetAccountByEmail(ctx, email)
	fail := problem.Validation(problem.FieldError{Pointer: "/password", Code: "invalid_credentials", Message: "E-Mail oder Passwort falsch."})
	if errors.Is(err, pgx.ErrNoRows) {
		VerifyPassword(password, dummyHash)
		s.throttle.fail(key)
		return NewSession{}, fail
	}
	if err != nil {
		return NewSession{}, err
	}
	if a.Status != "active" || !a.PasswordHash.Valid || !VerifyPassword(password, a.PasswordHash.String) {
		s.throttle.fail(key)
		return NewSession{}, fail
	}
	s.throttle.reset(key)
	token, csrf := randomToken(32), randomToken(32)
	now := time.Now()
	abs := now.Add(sessionAbsolute)
	if err := q.InsertSession(ctx, store.InsertSessionParams{ID: pgUUID(kernel.NewID()), AccountID: a.ID, TokenHash: hashToken(token),
		CsrfToken: csrf, ClientKind: "web", UserAgent: truncate(userAgent, 300),
		IdleExpiresAt: pgtype.Timestamptz{Time: now.Add(sessionIdle), Valid: true}, AbsoluteExpiresAt: pgtype.Timestamptz{Time: abs, Valid: true}}); err != nil {
		return NewSession{}, err
	}
	_ = q.MarkLogin(ctx, a.ID)
	return NewSession{Account: toAccount(a), Token: token, CSRFToken: csrf, Expires: abs}, nil
}

// SessionInfo ist eine aufgelöste, gültige Sitzung.
type SessionInfo struct {
	SessionID uuid.UUID
	AccountID uuid.UUID
	CSRFToken string
}

// ResolveSession prüft ein Sitzungs-Token und verlängert die Leerlaufzeit.
func (s *Service) ResolveSession(ctx context.Context, token string) (SessionInfo, bool, error) {
	q := store.New(s.pool)
	row, err := q.GetActiveSession(ctx, hashToken(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionInfo{}, false, nil
	}
	if err != nil {
		return SessionInfo{}, false, err
	}
	// Leerlaufzeit höchstens einmal pro Minute fortschreiben.
	if time.Since(row.LastSeenAt.Time) > time.Minute {
		_ = q.TouchSession(ctx, store.TouchSessionParams{ID: row.ID, IdleExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(sessionIdle), Valid: true}})
	}
	return SessionInfo{SessionID: uuid.UUID(row.ID.Bytes), AccountID: uuid.UUID(row.AccountID.Bytes), CSRFToken: row.CsrfToken}, true, nil
}

func (s *Service) Logout(ctx context.Context, sessionID uuid.UUID) error {
	return store.New(s.pool).RevokeSession(ctx, pgUUID(sessionID))
}

func (s *Service) Account(ctx context.Context, id uuid.UUID) (Account, error) {
	a, err := store.New(s.pool).GetAccountByID(ctx, pgUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, problem.Unauthorized()
	}
	return toAccount(a), err
}

func (s *Service) UpdateDisplayName(ctx context.Context, id uuid.UUID, name string) (Account, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 100 {
		return Account{}, problem.Validation(problem.FieldError{Pointer: "/display_name", Code: "length"})
	}
	a, err := store.New(s.pool).UpdateAccountDisplayName(ctx, store.UpdateAccountDisplayNameParams{ID: pgUUID(id), DisplayName: name})
	return toAccount(a), err
}

// Settings sind Nutzereinstellungen als JSON (Schema UserSettings der API).
func (s *Service) Settings(ctx context.Context, id uuid.UUID) (map[string]any, error) {
	a, err := store.New(s.pool).GetAccountByID(ctx, pgUUID(id))
	if err != nil {
		return nil, err
	}
	out := DefaultSettings()
	var stored map[string]any
	if err := json.Unmarshal(a.Settings, &stored); err == nil {
		mergePatch(out, stored)
	}
	return out, nil
}

// PatchSettings wendet einen JSON Merge Patch (RFC 7396) an.
func (s *Service) PatchSettings(ctx context.Context, id uuid.UUID, patch map[string]any) (map[string]any, error) {
	cur, err := s.Settings(ctx, id)
	if err != nil {
		return nil, err
	}
	mergePatch(cur, patch)
	b, _ := json.Marshal(cur)
	if _, err := store.New(s.pool).UpdateAccountSettings(ctx, store.UpdateAccountSettingsParams{ID: pgUUID(id), Settings: b}); err != nil {
		return nil, err
	}
	return cur, nil
}

// DefaultSettings: Vorgaben für neue Konten (metrisch, Deutsch, Euro).
func DefaultSettings() map[string]any {
	return map[string]any{
		"language": "de", "time_zone": "Europe/Berlin", "default_currency": "EUR",
		"display_units": map[string]any{"distance": "km", "volume": "l", "consumption": "l_per_100km",
			"electric_consumption": "kwh_per_100km", "oil_volume": "l"},
		"show_exif_location": false, "store_exif_location": false, "vehicle_identifier": "license_plate",
	}
}

func mergePatch(dst, patch map[string]any) {
	for k, v := range patch {
		if v == nil {
			delete(dst, k)
			continue
		}
		if pm, ok := v.(map[string]any); ok {
			if dm, ok := dst[k].(map[string]any); ok {
				mergePatch(dm, pm)
				continue
			}
		}
		dst[k] = v
	}
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func hashToken(t string) []byte { h := sha256.Sum256([]byte(t)); return h[:] }

func validEmail(e string) bool {
	at := strings.IndexByte(e, '@')
	return at > 0 && at < len(e)-3 && strings.Contains(e[at:], ".") && !strings.ContainsAny(e, " \t\n")
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// throttle zählt Fehlversuche je IP und E-Mail; ab dem 6. Versuch wächst die
// Wartezeit exponentiell bis 15 Minuten (ID-03). Zustand im Speicher genügt für
// eine Instanz; bei mehreren Instanzen folgt eine Datenbankvariante.
type throttle struct {
	mu sync.Mutex
	m  map[string]*attempts
}

type attempts struct {
	n     int
	until time.Time
}

func newThrottle() *throttle { return &throttle{m: map[string]*attempts{}} }

func (t *throttle) wait(k string) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	if a, ok := t.m[k]; ok {
		return time.Until(a.until)
	}
	return 0
}

func (t *throttle) fail(k string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	a := t.m[k]
	if a == nil {
		a = &attempts{}
		t.m[k] = a
	}
	a.n++
	if a.n > 5 {
		d := time.Second << min(a.n-6, 10)
		if d > 15*time.Minute {
			d = 15 * time.Minute
		}
		a.until = time.Now().Add(d)
	}
}

func (t *throttle) reset(k string) {
	t.mu.Lock()
	delete(t.m, k)
	t.mu.Unlock()
}
