package server

import (
	"context"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/sh1su/vector/backend/internal/api"
	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

func tokenBody(v identity.TokenView) api.ApiToken {
	id, created := v.ID, v.CreatedAt
	out := api.ApiToken{Id: &id, Name: v.Name, ExpiresAt: v.ExpiresAt, CreatedAt: &created}
	for _, s := range v.Scopes {
		out.Scopes = append(out.Scopes, api.Scope(s))
	}
	if v.VehicleIDs != nil {
		ids := make([]openapi_types.UUID, 0, len(v.VehicleIDs))
		ids = append(ids, v.VehicleIDs...)
		out.VehicleIds = nullable.NewNullableWithValue(ids)
	}
	if v.LastUsedAt != nil {
		out.LastUsedAt = nullable.NewNullableWithValue(*v.LastUsedAt)
	}
	if v.Token != "" {
		t := v.Token
		out.Token = &t
	}
	return out
}

func (s *Server) ListApiTokens(ctx context.Context, _ api.ListApiTokensRequestObject) (api.ListApiTokensResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if a.Restricted() {
		return nil, problem.Forbidden("Tokens verwaltet nur eine angemeldete Sitzung.")
	}
	list, err := s.d.Identity.Tokens(ctx, a.AccountID)
	if err != nil {
		return nil, err
	}
	out := api.ApiTokenPage{Items: []api.ApiToken{}}
	for _, v := range list {
		out.Items = append(out.Items, tokenBody(v))
	}
	return api.ListApiTokens200JSONResponse(out), nil
}

func (s *Server) CreateApiToken(ctx context.Context, req api.CreateApiTokenRequestObject) (api.CreateApiTokenResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if a.Restricted() {
		return nil, problem.Forbidden("Tokens verwaltet nur eine angemeldete Sitzung.")
	}
	b := req.Body
	scopes := make([]string, 0, len(b.Scopes))
	for _, sc := range b.Scopes {
		scopes = append(scopes, string(sc))
	}
	var vids []uuid.UUID
	if b.VehicleIds.IsSpecified() && !b.VehicleIds.IsNull() {
		list, _ := b.VehicleIds.Get()
		vids = append([]uuid.UUID{}, list...)
	}
	v, err := s.d.Identity.CreateToken(ctx, a.AccountID, b.Name, scopes, vids, b.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return api.CreateApiToken201JSONResponse{Body: tokenBody(v), Headers: api.CreateApiToken201ResponseHeaders{Location: loc("/me/api-tokens/" + v.ID.String())}}, nil
}

func (s *Server) RevokeApiToken(ctx context.Context, req api.RevokeApiTokenRequestObject) (api.RevokeApiTokenResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if a.Restricted() {
		return nil, problem.Forbidden("Tokens verwaltet nur eine angemeldete Sitzung.")
	}
	if err := s.d.Identity.RevokeToken(ctx, a.AccountID, uuid.UUID(req.TokenId)); err != nil {
		return nil, err
	}
	return api.RevokeApiToken204Response{}, nil
}
