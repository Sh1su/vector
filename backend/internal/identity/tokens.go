package identity

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/sh1su/vector/backend/internal/identity/store"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

// API-Tokens für Integrationen wie den MCP-Server. Das Klartext-Token gibt es nur einmal beim
// Anlegen; gespeichert wird der SHA-256. Präfix „vct_“, damit Secret-Scanner es erkennen.
const tokenPrefix = "vct_"

// Scopes, die ein Token tragen darf (Schema Scope). admin und sharing:manage vergibt die
// Anwendung nicht an Tokens.
var allowedScopes = map[string]bool{kernel.ScopeVehiclesRead: true, kernel.ScopeEntriesWrite: true, kernel.ScopeEntriesDelete: true}

type TokenView struct {
	ID         uuid.UUID
	Name       string
	Scopes     []string
	VehicleIDs []uuid.UUID
	ExpiresAt  time.Time
	CreatedAt  time.Time
	LastUsedAt *time.Time
	Token      string // nur beim Anlegen
}

func tokenView(t store.IdentityApiToken) TokenView {
	v := TokenView{ID: uuid.UUID(t.ID.Bytes), Name: t.Name, Scopes: t.Scopes, ExpiresAt: t.ExpiresAt.Time, CreatedAt: t.CreatedAt.Time}
	if t.VehicleIds != nil {
		v.VehicleIDs = []uuid.UUID{}
		for _, id := range t.VehicleIds {
			v.VehicleIDs = append(v.VehicleIDs, uuid.UUID(id.Bytes))
		}
	}
	if t.LastUsedAt.Valid {
		lu := t.LastUsedAt.Time
		v.LastUsedAt = &lu
	}
	return v
}

// CreateToken legt ein Token an. Fahrzeugbeschränkung nur auf eigene Fahrzeuge; Ablauf höchstens 1 Jahr.
func (s *Service) CreateToken(ctx context.Context, accountID uuid.UUID, name string, scopes []string, vehicleIDs []uuid.UUID, expires time.Time) (TokenView, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 100 {
		return TokenView{}, problem.Validation(problem.FieldError{Pointer: "/name", Code: "length"})
	}
	if len(scopes) == 0 {
		return TokenView{}, problem.Validation(problem.FieldError{Pointer: "/scopes", Code: "required"})
	}
	for _, sc := range scopes {
		if !allowedScopes[sc] {
			return TokenView{}, problem.Validation(problem.FieldError{Pointer: "/scopes", Code: "not_allowed", Message: "Scope " + sc + " ist für Tokens nicht erlaubt."})
		}
	}
	if !slices.Contains(scopes, kernel.ScopeVehiclesRead) {
		scopes = append(scopes, kernel.ScopeVehiclesRead)
	}
	now := time.Now()
	if !expires.After(now) || expires.After(now.Add(366*24*time.Hour)) {
		return TokenView{}, problem.Validation(problem.FieldError{Pointer: "/expires_at", Code: "range", Message: "Ablauf muss in der Zukunft und höchstens ein Jahr entfernt liegen."})
	}
	var vids []pgtype.UUID
	if vehicleIDs != nil {
		mine, err := MemberVehicles(ctx, s.pool, accountID)
		if err != nil {
			return TokenView{}, err
		}
		vids = []pgtype.UUID{}
		for _, id := range vehicleIDs {
			if _, ok := mine[id]; !ok {
				return TokenView{}, problem.Validation(problem.FieldError{Pointer: "/vehicle_ids", Code: "unknown_vehicle"})
			}
			vids = append(vids, pgUUID(id))
		}
	}
	plain := tokenPrefix + randomToken(32)
	row, err := store.New(s.pool).InsertApiToken(ctx, store.InsertApiTokenParams{ID: pgUUID(kernel.NewID()), AccountID: pgUUID(accountID), Name: name,
		TokenHash: hashToken(plain), Scopes: scopes, VehicleIds: vids, ExpiresAt: pgtype.Timestamptz{Time: expires, Valid: true}})
	if err != nil {
		return TokenView{}, err
	}
	v := tokenView(row)
	v.Token = plain
	return v, nil
}

func (s *Service) Tokens(ctx context.Context, accountID uuid.UUID) ([]TokenView, error) {
	rows, err := store.New(s.pool).ListApiTokens(ctx, pgUUID(accountID))
	if err != nil {
		return nil, err
	}
	out := make([]TokenView, 0, len(rows))
	for _, r := range rows {
		out = append(out, tokenView(r))
	}
	return out, nil
}

func (s *Service) RevokeToken(ctx context.Context, accountID, id uuid.UUID) error {
	n, err := store.New(s.pool).RevokeApiToken(ctx, store.RevokeApiTokenParams{ID: pgUUID(id), AccountID: pgUUID(accountID)})
	if err != nil {
		return err
	}
	if n == 0 {
		return problem.NotFound()
	}
	return nil
}

// ResolveToken prüft ein Bearer-Token und liefert den Akteur mit Scopes und Fahrzeugbeschränkung.
func (s *Service) ResolveToken(ctx context.Context, plain string) (kernel.Actor, bool, error) {
	if !strings.HasPrefix(plain, tokenPrefix) {
		return kernel.Actor{}, false, nil
	}
	q := store.New(s.pool)
	row, err := q.GetActiveApiToken(ctx, hashToken(plain))
	if errors.Is(err, pgx.ErrNoRows) {
		return kernel.Actor{}, false, nil
	}
	if err != nil {
		return kernel.Actor{}, false, err
	}
	if !row.LastUsedAt.Valid || time.Since(row.LastUsedAt.Time) > time.Minute {
		_ = q.TouchApiToken(ctx, row.ID)
	}
	a := kernel.Actor{AccountID: uuid.UUID(row.AccountID.Bytes), Kind: "api_token", Scopes: row.Scopes}
	if row.VehicleIds != nil {
		a.VehicleIDs = []uuid.UUID{}
		for _, id := range row.VehicleIds {
			a.VehicleIDs = append(a.VehicleIDs, uuid.UUID(id.Bytes))
		}
	}
	return a, true, nil
}
