package server

import (
	"context"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"

	"github.com/sh1su/vector/backend/internal/api"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

func (s *Server) ListAuditEvents(ctx context.Context, req api.ListAuditEventsRequestObject) (api.ListAuditEventsResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	p := req.Params
	items, next, err := s.d.Vehicles.AuditEvents(ctx, a, uuid.UUID(req.VehicleId), (*uuid.UUID)(p.ObjectId), p.Cursor, limitOf(p.Limit))
	if err != nil {
		return nil, err
	}
	out := api.ListAuditEvents200JSONResponse{Items: []api.AuditEvent{}, NextCursor: nullable.NewNullNullable[string]()}
	if err := convert(items, &out.Items); err != nil {
		return nil, problem.BadRequest("")
	}
	if next != nil {
		out.NextCursor = nullable.NewNullableWithValue(*next)
	}
	return out, nil
}
