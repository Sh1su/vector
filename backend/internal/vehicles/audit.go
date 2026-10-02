package vehicles

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/vehicles/store"
)

// AuditView entspricht dem Schema AuditEvent (Änderungshistorie, ADR-011).
type AuditView struct {
	ID             uuid.UUID      `json:"id"`
	OccurredAt     time.Time      `json:"occurred_at"`
	ActorAccountID *uuid.UUID     `json:"actor_account_id"`
	ActorKind      string         `json:"actor_kind"`
	Action         string         `json:"action"`
	ObjectType     string         `json:"object_type"`
	ObjectID       uuid.UUID      `json:"object_id"`
	Changes        map[string]any `json:"changes"`
	Reason         *string        `json:"reason"`
	RequestID      *string        `json:"request_id"`
}

// AuditEvents liefert die Änderungshistorie eines Fahrzeugs, neueste zuerst.
func (s *Service) AuditEvents(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, objectID *uuid.UUID, cursor *string, limit int) ([]AuditView, *string, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return nil, nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	p := store.ListAuditEventsParams{VehicleID: pgU(vehicleID), Lim: int32(limit + 1)}
	if objectID != nil {
		p.ObjectID = pgU(*objectID)
	}
	if cursor != nil {
		c, ok := kernel.DecodeCursor(*cursor)
		if !ok {
			return nil, nil, problem.Validation(problem.FieldError{Pointer: "/cursor", Code: "invalid"})
		}
		p.BeforeTs, p.BeforeID = pgtype.Timestamptz{Time: c.At, Valid: true}, pgU(c.ID)
	}
	rows, err := store.New(s.pool).ListAuditEvents(ctx, p)
	if err != nil {
		return nil, nil, err
	}
	out := []AuditView{}
	var next *string
	for i, r := range rows {
		if i == limit {
			c := kernel.Cursor{At: rows[i-1].OccurredAt.Time, ID: uuid.UUID(rows[i-1].ID.Bytes)}.Encode()
			next = &c
			break
		}
		v := AuditView{ID: uuid.UUID(r.ID.Bytes), OccurredAt: r.OccurredAt.Time, ActorKind: r.ActorKind, Action: r.Action, ObjectType: r.ObjectType,
			ObjectID: uuid.UUID(r.ObjectID.Bytes), Changes: map[string]any{}}
		if r.ActorAccountID.Valid {
			id := uuid.UUID(r.ActorAccountID.Bytes)
			v.ActorAccountID = &id
		}
		_ = json.Unmarshal(r.Changes, &v.Changes)
		if v.Changes == nil {
			v.Changes = map[string]any{}
		}
		if r.Reason.Valid {
			v.Reason = &r.Reason.String
		}
		if r.RequestID.Valid {
			v.RequestID = &r.RequestID.String
		}
		out = append(out, v)
	}
	return out, next, nil
}
