package identity

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/sh1su/vector/backend/internal/identity/store"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

func enumOK(v any, allowed ...string) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	for _, a := range allowed {
		if s == a {
			return true
		}
	}
	return false
}

// validateThresholds prüft das Schema MaintenanceThresholds (MA-05).
func validateThresholds(ptr string, v any, errs *[]problem.FieldError) {
	if v == nil {
		return
	}
	m, ok := v.(map[string]any)
	if !ok {
		*errs = append(*errs, problem.FieldError{Pointer: ptr, Code: "type"})
		return
	}
	for _, k := range []string{"upcoming_days", "due_days"} {
		if x, ok := m[k]; ok && x != nil {
			if f, ok := x.(float64); !ok || f < 0 || f != float64(int(f)) {
				*errs = append(*errs, problem.FieldError{Pointer: ptr + "/" + k, Code: "range"})
			}
		}
	}
	for _, k := range []string{"upcoming_distance", "due_distance"} {
		if x, ok := m[k]; ok && x != nil {
			q, ok := x.(map[string]any)
			val, vok := q["value"].(float64)
			if !ok || !vok || val < 0 || !enumOK(q["unit"], "km", "mi", "m", "h", "s") {
				*errs = append(*errs, problem.FieldError{Pointer: ptr + "/" + k, Code: "quantity"})
			}
		}
	}
}

// validateSettings prüft die Nutzereinstellungen nach dem Merge (Schema UserSettings).
func validateSettings(st map[string]any) error {
	var errs []problem.FieldError
	add := func(p, c string) { errs = append(errs, problem.FieldError{Pointer: p, Code: c}) }
	if v, ok := st["language"]; ok && !enumOK(v, "de", "en") {
		add("/language", "enum")
	}
	if v, ok := st["time_zone"]; ok {
		if s, _ := v.(string); s == "" {
			add("/time_zone", "time_zone")
		} else if _, err := time.LoadLocation(s); err != nil {
			add("/time_zone", "time_zone")
		}
	}
	if v, ok := st["default_currency"]; ok {
		if s, _ := v.(string); len(s) != 3 || s < "AAA" || s > "ZZZ" {
			add("/default_currency", "pattern")
		}
	}
	if du, ok := st["display_units"].(map[string]any); ok {
		allowed := map[string][]string{"distance": {"km", "mi"}, "volume": {"l", "gal_us", "gal_imp"},
			"consumption": {"l_per_100km", "km_per_l", "mpg_us", "mpg_uk"}, "electric_consumption": {"kwh_per_100km", "km_per_kwh", "mi_per_kwh"},
			"oil_volume": {"ml", "l", "qt_us", "qt_imp"}}
		for k, v := range du {
			if a, ok := allowed[k]; !ok || !enumOK(v, a...) {
				add("/display_units/"+k, "enum")
			}
		}
	}
	if v, ok := st["vehicle_identifier"]; ok && !enumOK(v, "license_plate", "vin", "custom_field") {
		add("/vehicle_identifier", "enum")
	}
	validateThresholds("/maintenance_thresholds", st["maintenance_thresholds"], &errs)
	if len(errs) > 0 {
		return problem.Validation(errs...)
	}
	return nil
}

// ChangePassword prüft das aktuelle Passwort, setzt das neue und beendet alle
// anderen Sitzungen (ID-03).
func (s *Service) ChangePassword(ctx context.Context, accountID, sessionID uuid.UUID, current, next string) error {
	q := store.New(s.pool)
	a, err := q.GetAccountByID(ctx, pgUUID(accountID))
	if err != nil {
		return err
	}
	if !a.PasswordHash.Valid || !VerifyPassword(current, a.PasswordHash.String) {
		return problem.Validation(problem.FieldError{Pointer: "/current_password", Code: "invalid_credentials", Message: "Das aktuelle Passwort stimmt nicht."})
	}
	if err := CheckPasswordPolicy(next); err != nil {
		return problem.Validation(problem.FieldError{Pointer: "/new_password", Code: "policy", Message: "Passwort: " + err.Error()})
	}
	hash, err := HashPassword(next)
	if err != nil {
		return err
	}
	if err := q.UpdatePasswordHash(ctx, store.UpdatePasswordHashParams{ID: a.ID, PasswordHash: pgtype.Text{String: hash, Valid: true}}); err != nil {
		return err
	}
	return q.RevokeOtherSessions(ctx, store.RevokeOtherSessionsParams{AccountID: a.ID, ID: pgUUID(sessionID)})
}

// SessionView entspricht dem Schema Session.
type SessionView struct {
	ID         uuid.UUID `json:"id"`
	ClientKind string    `json:"client_kind"`
	UserAgent  string    `json:"user_agent"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	Current    bool      `json:"current"`
}

// Sessions listet die aktiven Sitzungen eines Kontos.
func (s *Service) Sessions(ctx context.Context, accountID, current uuid.UUID) ([]SessionView, error) {
	rows, err := store.New(s.pool).ListSessions(ctx, pgUUID(accountID))
	if err != nil {
		return nil, err
	}
	out := []SessionView{}
	for _, r := range rows {
		id := uuid.UUID(r.ID.Bytes)
		out = append(out, SessionView{ID: id, ClientKind: r.ClientKind, UserAgent: r.UserAgent, CreatedAt: r.CreatedAt.Time,
			LastSeenAt: r.LastSeenAt.Time, Current: id == current})
	}
	return out, nil
}

// RevokeSession beendet eine eigene Sitzung.
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

// DefaultInstallation sind die Vorgaben der Installation (MA-05, ADR-024).
func DefaultInstallation() map[string]any {
	return map[string]any{
		"registration_open": false,
		"default_thresholds": map[string]any{"upcoming_days": float64(30), "due_days": float64(7),
			"upcoming_distance": map[string]any{"value": float64(1500), "unit": "km"},
			"due_distance":      map[string]any{"value": float64(500), "unit": "km"}},
		"retention_days":    float64(30),
		"assistant_enabled": false,
	}
}

// Installation liefert die gespeicherten Installationseinstellungen mit Vorgaben.
func (s *Service) Installation(ctx context.Context) (map[string]any, error) {
	raw, err := store.New(s.pool).GetInstallationSettings(ctx)
	if err != nil {
		return nil, err
	}
	out := DefaultInstallation()
	var stored map[string]any
	if json.Unmarshal(raw, &stored) == nil {
		mergePatch(out, stored)
	}
	return out, nil
}

// installationWritable sind die Felder, die ein Admin über die API ändern kann.
// Andere Felder (z. B. odometer_v_max_kmh, upload_max_bytes) kommen aus der
// Umgebung des Servers (ADR-030) und sind hier nur lesbar.
var installationWritable = map[string]bool{"registration_open": true, "default_thresholds": true, "retention_days": true, "assistant_enabled": true}

// PatchInstallation wendet einen Merge Patch an (nur Administratoren).
func (s *Service) PatchInstallation(ctx context.Context, admin uuid.UUID, patch map[string]any) (map[string]any, error) {
	var errs []problem.FieldError
	for k := range patch {
		if !installationWritable[k] {
			errs = append(errs, problem.FieldError{Pointer: "/" + k, Code: "read_only", Message: "Wird über die Server-Konfiguration gesetzt."})
		}
	}
	cur, err := s.Installation(ctx)
	if err != nil {
		return nil, err
	}
	mergePatch(cur, patch)
	for _, k := range []string{"registration_open", "assistant_enabled"} {
		if _, ok := cur[k].(bool); !ok {
			errs = append(errs, problem.FieldError{Pointer: "/" + k, Code: "type"})
		}
	}
	if r, ok := cur["retention_days"].(float64); !ok || r < 1 || r != float64(int(r)) {
		errs = append(errs, problem.FieldError{Pointer: "/retention_days", Code: "range"})
	}
	validateThresholds("/default_thresholds", cur["default_thresholds"], &errs)
	if len(errs) > 0 {
		return nil, problem.Validation(errs...)
	}
	b, _ := json.Marshal(cur)
	if err := store.New(s.pool).UpdateInstallationSettings(ctx, store.UpdateInstallationSettingsParams{Settings: b, UpdatedBy: pgUUID(admin)}); err != nil {
		return nil, err
	}
	return cur, nil
}
