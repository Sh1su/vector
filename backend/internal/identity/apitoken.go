package identity

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/sh1su/vector/backend/internal/identity/store"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

// Scopes der API-Tokens (ADR-016).
var Scopes = map[string]bool{"vehicles:read": true, "entries:write": true, "entries:delete": true, "sharing:manage": true, "admin": true}

// TokenPrefix kennzeichnet Vectra-Tokens (erleichtert Secret-Scanning).
const TokenPrefix = "vct_"

// ApiTokenView entspricht dem Schema ApiToken.
type ApiTokenView struct {
	ID         uuid.UUID   `json:"id"`
	Name       string      `json:"name"`
	Scopes     []string    `json:"scopes"`
	VehicleIDs []uuid.UUID `json:"vehicle_ids"`
	ExpiresAt  time.Time   `json:"expires_at"`
	CreatedAt  time.Time   `json:"created_at"`
	LastUsedAt *time.Time  `json:"last_used_at"`
	Token      string      `json:"token,omitempty"`
}

func tokenView(t store.IdentityApiToken) ApiTokenView {
	v := ApiTokenView{ID: uuid.UUID(t.ID.Bytes), Name: t.Name, Scopes: t.Scopes, ExpiresAt: t.ExpiresAt.Time, CreatedAt: t.CreatedAt.Time}
	if t.VehicleIds != nil {
		v.VehicleIDs = []uuid.UUID{}
		for _, id := range t.VehicleIds {
			v.VehicleIDs = append(v.VehicleIDs, uuid.UUID(id.Bytes))
		}
	}
	if t.LastUsedAt.Valid {
		l := t.LastUsedAt.Time
		v.LastUsedAt = &l
	}
	return v
}

// CreateApiToken legt ein Token an; der Klartext wird nur hier einmal geliefert.
// Nur aus einer angemeldeten Sitzung, nie mit einem Token selbst.
func (s *Service) CreateApiToken(ctx context.Context, actor kernel.Actor, name string, scopes []string, vehicleIDs *[]uuid.UUID, expires time.Time) (ApiTokenView, error) {
	if actor.Kind == "api_token" {
		return ApiTokenView{}, problem.Forbidden("API-Tokens werden nur in der Web-App angelegt.")
	}
	var errs []problem.FieldError
	name = strings.TrimSpace(name)
	if n := len([]rune(name)); n < 1 || n > 100 {
		errs = append(errs, problem.FieldError{Pointer: "/name", Code: "length"})
	}
	if len(scopes) == 0 {
		errs = append(errs, problem.FieldError{Pointer: "/scopes", Code: "required"})
	}
	for _, sc := range scopes {
		if !Scopes[sc] {
			errs = append(errs, problem.FieldError{Pointer: "/scopes", Code: "enum", Message: sc})
		}
		if sc == "admin" && !actor.IsAdmin {
			errs = append(errs, problem.FieldError{Pointer: "/scopes", Code: "admin_only", Message: "Nur Administratoren können den Scope admin vergeben."})
		}
	}
	now := time.Now()
	if !expires.After(now) || expires.After(now.AddDate(2, 0, 0)) {
		errs = append(errs, problem.FieldError{Pointer: "/expires_at", Code: "range", Message: "Ablauf in der Zukunft, höchstens 2 Jahre."})
	}
	var vids []pgtype.UUID
	if vehicleIDs != nil {
		vids = []pgtype.UUID{}
		for _, id := range *vehicleIDs {
			if role, err := RoleOf(ctx, s.pool, id, actor.AccountID); err != nil {
				return ApiTokenView{}, err
			} else if role == "" {
				errs = append(errs, problem.FieldError{Pointer: "/vehicle_ids", Code: "not_found", Message: id.String()})
			}
			vids = append(vids, pgUUID(id))
		}
	}
	if len(errs) > 0 {
		return ApiTokenView{}, problem.Validation(errs...)
	}
	secret := TokenPrefix + randomToken(32)
	t, err := store.New(s.pool).InsertApiToken(ctx, store.InsertApiTokenParams{ID: pgUUID(kernel.NewID()), AccountID: pgUUID(actor.AccountID), Name: name,
		TokenHash: hashToken(secret), Prefix: secret[:len(TokenPrefix)+6], Scopes: scopes, VehicleIds: vids,
		ExpiresAt: pgtype.Timestamptz{Time: expires, Valid: true}})
	if err != nil {
		return ApiTokenView{}, err
	}
	v := tokenView(t)
	v.Token = secret
	return v, nil
}

// ApiTokens listet die aktiven Tokens eines Kontos.
func (s *Service) ApiTokens(ctx context.Context, accountID uuid.UUID) ([]ApiTokenView, error) {
	rows, err := store.New(s.pool).ListApiTokens(ctx, pgUUID(accountID))
	if err != nil {
		return nil, err
	}
	out := []ApiTokenView{}
	for _, r := range rows {
		out = append(out, tokenView(r))
	}
	return out, nil
}

// RevokeApiToken widerruft ein eigenes Token.
func (s *Service) RevokeApiToken(ctx context.Context, accountID, id uuid.UUID) error {
	n, err := store.New(s.pool).RevokeApiToken(ctx, store.RevokeApiTokenParams{ID: pgUUID(id), AccountID: pgUUID(accountID)})
	if err != nil {
		return err
	}
	if n == 0 {
		return problem.NotFound()
	}
	return nil
}

// TokenInfo ist ein aufgelöstes API-Token.
type TokenInfo struct {
	TokenID    uuid.UUID
	AccountID  uuid.UUID
	Scopes     []string
	VehicleIDs []uuid.UUID
}

// ResolveApiToken prüft ein Bearer-Token.
func (s *Service) ResolveApiToken(ctx context.Context, secret string) (TokenInfo, bool, error) {
	if !strings.HasPrefix(secret, TokenPrefix) {
		return TokenInfo{}, false, nil
	}
	q := store.New(s.pool)
	t, err := q.GetActiveApiToken(ctx, hashToken(secret))
	if errors.Is(err, pgx.ErrNoRows) {
		return TokenInfo{}, false, nil
	}
	if err != nil {
		return TokenInfo{}, false, err
	}
	if !t.LastUsedAt.Valid || time.Since(t.LastUsedAt.Time) > time.Minute {
		_ = q.TouchApiToken(ctx, t.ID)
	}
	v := tokenView(t)
	return TokenInfo{TokenID: v.ID, AccountID: uuid.UUID(t.AccountID.Bytes), Scopes: t.Scopes, VehicleIDs: v.VehicleIDs}, true, nil
}
