package server

import (
	"context"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"

	"github.com/sh1su/vector/backend/internal/api"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

func (s *Server) ListApiTokens(ctx context.Context, _ api.ListApiTokensRequestObject) (api.ListApiTokensResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.d.Identity.ApiTokens(ctx, a.AccountID)
	if err != nil {
		return nil, err
	}
	out := api.ListApiTokens200JSONResponse{Items: []api.ApiToken{}, NextCursor: nullable.NewNullNullable[string]()}
	if err := convert(items, &out.Items); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Server) CreateApiToken(ctx context.Context, req api.CreateApiTokenRequestObject) (api.CreateApiTokenResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	b := req.Body
	scopes := make([]string, 0, len(b.Scopes))
	for _, sc := range b.Scopes {
		scopes = append(scopes, string(sc))
	}
	var vids *[]uuid.UUID
	if b.VehicleIds.IsSpecified() && !b.VehicleIds.IsNull() {
		list := []uuid.UUID{}
		for _, id := range b.VehicleIds.MustGet() {
			list = append(list, uuid.UUID(id))
		}
		vids = &list
	}
	v, err := s.d.Identity.CreateApiToken(ctx, a, b.Name, scopes, vids, b.ExpiresAt)
	if err != nil {
		return nil, err
	}
	var body api.ApiToken
	if err := convert(v, &body); err != nil {
		return nil, err
	}
	return api.CreateApiToken201JSONResponse{Body: body}, nil
}

func (s *Server) RevokeApiToken(ctx context.Context, req api.RevokeApiTokenRequestObject) (api.RevokeApiTokenResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.Identity.RevokeApiToken(ctx, a.AccountID, uuid.UUID(req.TokenId)); err != nil {
		return nil, err
	}
	return api.RevokeApiToken204Response{}, nil
}

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
