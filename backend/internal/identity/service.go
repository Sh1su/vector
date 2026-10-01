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
// Die Android-App bleibt angemeldet: Leerlauf 90 Tage, höchstens 365 Tage (Gerät mit
// Bildschirmsperre, Sitzung in den Einstellungen jederzeit beendbar).
const (
	sessionIdle     = 7 * 24 * time.Hour
	sessionAbsolute = 30 * 24 * time.Hour
	appIdle         = 90 * 24 * time.Hour
	appAbsolute     = 365 * 24 * time.Hour
)

func sessionLimits(kind string) (idle, absolute time.Duration) {
	if kind == "android" {
		return appIdle, appAbsolute
	}
	return sessionIdle, sessionAbsolute
}

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
func (s *Service) Login(ctx context.Context, email, password, clientKind, clientIP, userAgent string) (NewSession, error) {
	if clientKind != "android" {
		clientKind = "web"
	}
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
	idle, absolute := sessionLimits(clientKind)
	abs := now.Add(absolute)
	if err := q.InsertSession(ctx, store.InsertSessionParams{ID: pgUUID(kernel.NewID()), AccountID: a.ID, TokenHash: hashToken(token),
		CsrfToken: csrf, ClientKind: clientKind, UserAgent: truncate(userAgent, 300),
		IdleExpiresAt: pgtype.Timestamptz{Time: now.Add(idle), Valid: true}, AbsoluteExpiresAt: pgtype.Timestamptz{Time: abs, Valid: true}}); err != nil {
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

// SessionView ist eine eigene Sitzung für die Einstellungsseite.
type SessionView struct {
	ID         uuid.UUID
	ClientKind string
	UserAgent  string
	CreatedAt  time.Time
	LastSeenAt time.Time
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
		idle, _ := sessionLimits(row.ClientKind)
		_ = q.TouchSession(ctx, store.TouchSessionParams{ID: row.ID, IdleExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(idle), Valid: true}})
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

// ChangePassword prüft das bisherige Passwort, setzt das neue und beendet alle anderen Sitzungen.
func (s *Service) ChangePassword(ctx context.Context, accountID, sessionID uuid.UUID, current, next string) error {
	q := store.New(s.pool)
	a, err := q.GetAccountByID(ctx, pgUUID(accountID))
	if err != nil {
		return err
	}
	key := "pw|" + accountID.String()
	if wait := s.throttle.wait(key); wait > 0 {
		return problem.TooManyRequests("Zu viele Fehlversuche. Bitte später erneut versuchen.")
	}
	if !a.PasswordHash.Valid || !VerifyPassword(current, a.PasswordHash.String) {
		s.throttle.fail(key)
		return problem.Validation(problem.FieldError{Pointer: "/current_password", Code: "invalid_credentials", Message: "Das bisherige Passwort stimmt nicht."})
	}
	s.throttle.reset(key)
	if err := CheckPasswordPolicy(next); err != nil {
		return problem.Validation(problem.FieldError{Pointer: "/new_password", Code: "weak_password", Message: err.Error()})
	}
	hash, err := HashPassword(next)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		qt := store.New(tx)
		if err := qt.UpdatePasswordHash(ctx, store.UpdatePasswordHashParams{ID: pgUUID(accountID), PasswordHash: pgtype.Text{String: hash, Valid: true}}); err != nil {
			return err
		}
		return qt.RevokeOtherSessions(ctx, store.RevokeOtherSessionsParams{AccountID: pgUUID(accountID), ID: pgUUID(sessionID)})
	})
}

// Sessions listet die gültigen Sitzungen eines Kontos, zuletzt aktive zuerst.
func (s *Service) Sessions(ctx context.Context, accountID uuid.UUID) ([]SessionView, error) {
	rows, err := store.New(s.pool).ListSessions(ctx, pgUUID(accountID))
	if err != nil {
		return nil, err
	}
	out := make([]SessionView, 0, len(rows))
	for _, r := range rows {
		out = append(out, SessionView{ID: uuid.UUID(r.ID.Bytes), ClientKind: r.ClientKind, UserAgent: r.UserAgent, CreatedAt: r.CreatedAt.Time, LastSeenAt: r.LastSeenAt.Time})
	}
	return out, nil
}

// RevokeSession beendet eine eigene Sitzung (auch die aktuelle).
func (s *Service) RevokeSession(ctx context.Context, accountID, sessionID uuid.UUID) error {
	n, err := store.New(s.pool).RevokeOwnSession(ctx, store.RevokeOwnSessionParams{ID: pgUUID(sessionID), AccountID: pgUUID(accountID)})
	if err != nil {
		return err
	}
	if n == 0 {
		return problem.NotFound()
	}
	return nil
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

// OwnerSettings liefert die Einstellungen des Fahrzeughalters (z. B. Wartungsschwellen als
// Vorgabe für alle Definitionen des Fahrzeugs); leer, wenn es keinen Halter gibt.
func (s *Service) OwnerSettings(ctx context.Context, db store.DBTX, vehicleID uuid.UUID) map[string]any {
	raw, err := store.New(db).VehicleOwnerSettings(ctx, pgUUID(vehicleID))
	if err != nil {
		return nil
	}
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return m
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
