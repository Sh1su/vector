package kernel

import (
	"context"

	"github.com/google/uuid"
)

// Actor ist der angemeldete Akteur eines Requests.
type Actor struct {
	AccountID uuid.UUID
	IsAdmin   bool
	SessionID uuid.UUID
	Kind      string // user | api_token | assistant | import | system
	RequestID string
	// Nur bei API-Tokens: erlaubte Scopes und (optional) Fahrzeuge. nil = keine Einschränkung.
	Scopes     []string
	VehicleIDs []uuid.UUID
}

// Scopes für API-Tokens (Schema Scope).
const (
	ScopeVehiclesRead  = "vehicles:read"
	ScopeEntriesWrite  = "entries:write"
	ScopeEntriesDelete = "entries:delete"
)

// Restricted: Akteur unterliegt Token-Scopes.
func (a Actor) Restricted() bool { return a.Kind == "api_token" }

// HasScope prüft einen Scope; Sitzungen haben alle Scopes ihrer Rolle.
func (a Actor) HasScope(s string) bool {
	if !a.Restricted() {
		return true
	}
	for _, x := range a.Scopes {
		if x == s {
			return true
		}
	}
	return false
}

// MayAccess prüft die Fahrzeugbeschränkung eines Tokens.
func (a Actor) MayAccess(vehicleID uuid.UUID) bool {
	if a.VehicleIDs == nil {
		return true
	}
	for _, id := range a.VehicleIDs {
		if id == vehicleID {
			return true
		}
	}
	return false
}

type actorKey struct{}

// WithActor legt den Akteur im Kontext ab.
func WithActor(ctx context.Context, a Actor) context.Context { return context.WithValue(ctx, actorKey{}, a) }

// ActorFrom liest den Akteur aus dem Kontext.
func ActorFrom(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(actorKey{}).(Actor)
	return a, ok
}

// NewID erzeugt eine UUIDv7 (ADR-006).
func NewID() uuid.UUID { return uuid.Must(uuid.NewV7()) }
