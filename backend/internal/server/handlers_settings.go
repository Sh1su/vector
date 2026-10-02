package server

import (
	"context"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"

	"github.com/sh1su/vector/backend/internal/api"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

func (s *Server) ChangePassword(ctx context.Context, req api.ChangePasswordRequestObject) (api.ChangePasswordResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.Identity.ChangePassword(ctx, a.AccountID, a.SessionID, str(req.Body.CurrentPassword), str(req.Body.NewPassword)); err != nil {
		return nil, err
	}
	return api.ChangePassword204Response{}, nil
}

func (s *Server) ListSessions(ctx context.Context, _ api.ListSessionsRequestObject) (api.ListSessionsResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.d.Identity.Sessions(ctx, a.AccountID, a.SessionID)
	if err != nil {
		return nil, err
	}
	out := api.ListSessions200JSONResponse{Items: []api.Session{}, NextCursor: nullable.NewNullNullable[string]()}
	if err := convert(items, &out.Items); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Server) RevokeSession(ctx context.Context, req api.RevokeSessionRequestObject) (api.RevokeSessionResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.Identity.RevokeSession(ctx, a.AccountID, uuid.UUID(req.SessionId)); err != nil {
		return nil, err
	}
	return api.RevokeSession204Response{}, nil
}

func mustAdmin(ctx context.Context) (kernel.Actor, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return a, err
	}
	if !a.IsAdmin {
		return a, problem.Forbidden("Nur für Administratoren.")
	}
	return a, nil
}

func (s *Server) installationBody(st map[string]any) (api.InstallationSettings, error) {
	st["odometer_v_max_kmh"] = s.d.OdometerVMaxKmh
	if _, ok := st["oidc_providers"]; !ok {
		st["oidc_providers"] = []any{}
	}
	var out api.InstallationSettings
	err := convert(st, &out)
	return out, err
}

func (s *Server) AdminGetSettings(ctx context.Context, _ api.AdminGetSettingsRequestObject) (api.AdminGetSettingsResponseObject, error) {
	if _, err := mustAdmin(ctx); err != nil {
		return nil, err
	}
	st, err := s.d.Identity.Installation(ctx)
	if err != nil {
		return nil, err
	}
	body, err := s.installationBody(st)
	if err != nil {
		return nil, err
	}
	return api.AdminGetSettings200JSONResponse{Body: body}, nil
}

func (s *Server) AdminUpdateSettings(ctx context.Context, req api.AdminUpdateSettingsRequestObject) (api.AdminUpdateSettingsResponseObject, error) {
	a, err := mustAdmin(ctx)
	if err != nil {
		return nil, err
	}
	var patch map[string]any
	if err := convert(req.Body, &patch); err != nil {
		return nil, problem.BadRequest("")
	}
	st, err := s.d.Identity.PatchInstallation(ctx, a.AccountID, patch)
	if err != nil {
		return nil, err
	}
	body, err := s.installationBody(st)
	if err != nil {
		return nil, err
	}
	return api.AdminUpdateSettings200JSONResponse{Body: body}, nil
}
