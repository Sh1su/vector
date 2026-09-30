// Package audit schreibt die append-only Änderungshistorie (ADR-011) in der
// Transaktion der fachlichen Änderung.
package audit

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/sh1su/vector/backend/internal/kernel"
)

type Event struct {
	Action     string
	VehicleID  *uuid.UUID
	ObjectType string
	ObjectID   uuid.UUID
	Changes    any
	Reason     string
}

func Write(ctx context.Context, tx pgx.Tx, actor kernel.Actor, e Event) error {
	ch, err := json.Marshal(e.Changes)
	if err != nil || e.Changes == nil {
		ch = []byte("{}")
	}
	kind := actor.Kind
	if kind == "" {
		kind = "user"
	}
	var reason *string
	if e.Reason != "" {
		reason = &e.Reason
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit.event (id, actor_account_id, actor_kind, action, vehicle_id, object_type, object_id, changes, reason, request_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		kernel.NewID(), actor.AccountID, kind, e.Action, e.VehicleID, e.ObjectType, e.ObjectID, ch, reason, actor.RequestID)
	return err
}
