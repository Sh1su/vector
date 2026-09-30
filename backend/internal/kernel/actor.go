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
