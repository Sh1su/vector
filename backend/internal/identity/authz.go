package identity

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/sh1su/vector/backend/internal/identity/store"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

// Rollen je Fahrzeug (ADR-016), aufsteigend.
const (
	RoleViewer = "viewer"
	RoleEditor = "editor"
	RoleOwner  = "owner"
)

var roleRank = map[string]int{RoleViewer: 1, RoleEditor: 2, RoleOwner: 3}

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

// RoleOf liefert die Rolle eines Kontos am Fahrzeug oder "" ohne Mitgliedschaft.
func RoleOf(ctx context.Context, db store.DBTX, vehicleID, accountID uuid.UUID) (string, error) {
	role, err := store.New(db).GetRole(ctx, store.GetRoleParams{VehicleID: pgUUID(vehicleID), AccountID: pgUUID(accountID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return role, err
}

// Authorize prüft die Rolle des Akteurs an einem Fahrzeug (ID-01). Die
// vehicleID muss aus dem geladenen Objekt stammen, nie aus dem Request.
// Ohne Mitgliedschaft: 404 (kein Informationsleck, ADR-013); zu geringe Rolle: 403.
func Authorize(ctx context.Context, db store.DBTX, actor kernel.Actor, vehicleID uuid.UUID, need string) (string, error) {
	role, err := RoleOf(ctx, db, vehicleID, actor.AccountID)
	if err != nil {
		return "", err
	}
	if role == "" {
		return "", problem.NotFound()
	}
	if roleRank[role] < roleRank[need] {
		return role, problem.Forbidden("Diese Aktion erfordert die Rolle " + need + ".")
	}
	return role, nil
}

// AddMembership legt eine Mitgliedschaft an (in der Transaktion des Aufrufers).
func AddMembership(ctx context.Context, db store.DBTX, vehicleID, accountID uuid.UUID, role string) error {
	return store.New(db).InsertMembership(ctx, store.InsertMembershipParams{VehicleID: pgUUID(vehicleID), AccountID: pgUUID(accountID), Role: role})
}

// MemberVehicles liefert alle Fahrzeuge mit Rolle eines Kontos.
func MemberVehicles(ctx context.Context, db store.DBTX, accountID uuid.UUID) (map[uuid.UUID]string, error) {
	rows, err := store.New(db).ListMemberVehicleIDs(ctx, pgUUID(accountID))
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]string, len(rows))
	for _, r := range rows {
		out[uuid.UUID(r.VehicleID.Bytes)] = r.Role
	}
	return out, nil
}
