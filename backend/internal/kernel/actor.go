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
	// Nur bei API-Tokens: erlaubte Scopes und optional erlaubte Fahrzeuge (nil = alle).
	Scopes     []string
	VehicleIDs []uuid.UUID
}

// VehicleAllowed meldet, ob ein API-Token auf das Fahrzeug beschränkt ist (ADR-016).
func (a Actor) VehicleAllowed(id uuid.UUID) bool {
	if a.VehicleIDs == nil {
		return true
	}
	for _, v := range a.VehicleIDs {
		if v == id {
			return true
		}
	}
	return false
}

// HasScope meldet, ob der Akteur einen Scope besitzt. Sitzungen haben alle Scopes.
func (a Actor) HasScope(scope string) bool {
	if a.Kind != "api_token" {
		return true
	}
	for _, s := range a.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

type actorKey struct{}

// WithActor legt den Akteur im Kontext ab.
func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, a)
}

// ActorFrom liest den Akteur aus dem Kontext.
func ActorFrom(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(actorKey{}).(Actor)
	return a, ok
}

// NewID erzeugt eine UUIDv7 (ADR-006).
func NewID() uuid.UUID { return uuid.Must(uuid.NewV7()) }
