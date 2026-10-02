package server

import (
	"context"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"

	"github.com/sh1su/vector/backend/internal/api"
	"github.com/sh1su/vector/backend/internal/maintenance"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

func itemBody(v maintenance.View) (api.MaintenanceItem, *string, error) {
	var out api.MaintenanceItem
	err := convert(v, &out)
	et := vehicles.ETag(v.Version)
	return out, &et, err
}

func (s *Server) ListMaintenanceTemplates(ctx context.Context, _ api.ListMaintenanceTemplatesRequestObject) (api.ListMaintenanceTemplatesResponseObject, error) {
	if _, err := mustActor(ctx); err != nil {
		return nil, err
	}
	out := api.ListMaintenanceTemplates200JSONResponse{Items: []api.MaintenanceTemplate{}, NextCursor: nullable.NewNullNullable[string]()}
	if err := convert(maintenance.Templates, &out.Items); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Server) ListMaintenanceItems(ctx context.Context, req api.ListMaintenanceItemsRequestObject) (api.ListMaintenanceItemsResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	p := req.Params
	items, err := s.d.Maintenance.List(ctx, a, uuid.UUID(req.VehicleId), p.Active, p.IncludeDeleted != nil && *p.IncludeDeleted)
	if err != nil {
		return nil, err
	}
	out := api.ListMaintenanceItems200JSONResponse{Items: []api.MaintenanceItem{}, NextCursor: nullable.NewNullNullable[string]()}
	if err := convert(items, &out.Items); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Server) CreateMaintenanceItem(ctx context.Context, req api.CreateMaintenanceItemRequestObject) (api.CreateMaintenanceItemResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var in maintenance.Input
	if err := convert(req.Body, &in); err != nil {
		return nil, problem.BadRequest("")
	}
	v, created, err := s.d.Maintenance.Create(ctx, a, uuid.UUID(req.VehicleId), (*uuid.UUID)(req.Body.Id), in)
	if err != nil {
		return nil, err
	}
	body, et, err := itemBody(v)
	if err != nil {
		return nil, err
	}
	if !created {
		return api.CreateMaintenanceItem200JSONResponse(body), nil
	}
	loc := apiPrefix + "/vehicles/" + v.VehicleID.String() + "/maintenance-items/" + v.ID.String()
	return api.CreateMaintenanceItem201JSONResponse{Body: body, Headers: api.CreateMaintenanceItem201ResponseHeaders{ETag: et, Location: &loc}}, nil
}

func (s *Server) GetMaintenanceItem(ctx context.Context, req api.GetMaintenanceItemRequestObject) (api.GetMaintenanceItemResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Maintenance.Get(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.ItemId))
	if err != nil {
		return nil, err
	}
	body, et, err := itemBody(v)
	if err != nil {
		return nil, err
	}
	return api.GetMaintenanceItem200JSONResponse{Body: body, Headers: api.GetMaintenanceItem200ResponseHeaders{ETag: et}}, nil
}

func (s *Server) UpdateMaintenanceItem(ctx context.Context, req api.UpdateMaintenanceItemRequestObject) (api.UpdateMaintenanceItemResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	patch, _, _, err := splitPatch(req.Body)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Maintenance.Update(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.ItemId), req.Params.IfMatch, patch)
	if err != nil {
		return nil, err
	}
	body, et, err := itemBody(v)
	if err != nil {
		return nil, err
	}
	return api.UpdateMaintenanceItem200JSONResponse{Body: body, Headers: api.UpdateMaintenanceItem200ResponseHeaders{ETag: et}}, nil
}

func (s *Server) DeleteMaintenanceItem(ctx context.Context, req api.DeleteMaintenanceItemRequestObject) (api.DeleteMaintenanceItemResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.Maintenance.Delete(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.ItemId), req.Params.IfMatch); err != nil {
		return nil, err
	}
	return api.DeleteMaintenanceItem204Response{}, nil
}

func (s *Server) ListCompletions(ctx context.Context, req api.ListCompletionsRequestObject) (api.ListCompletionsResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.d.Maintenance.Completions(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.ItemId))
	if err != nil {
		return nil, err
	}
	out := api.ListCompletions200JSONResponse{Items: []api.MaintenanceCompletion{}, NextCursor: nullable.NewNullNullable[string]()}
	if err := convert(items, &out.Items); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Server) CreateCompletion(ctx context.Context, req api.CreateCompletionRequestObject) (api.CreateCompletionResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var in maintenance.CompletionInput
	if err := convert(req.Body, &in); err != nil {
		return nil, problem.BadRequest("")
	}
	key := ""
	if req.Params.IdempotencyKey != nil {
		key = string(*req.Params.IdempotencyKey)
	}
	v, created, err := s.d.Maintenance.Complete(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.ItemId), key, in)
	if err != nil {
		return nil, err
	}
	var body api.MaintenanceCompletion
	if err := convert(v, &body); err != nil {
		return nil, err
	}
	if !created {
		return api.CreateCompletion200JSONResponse(body), nil
	}
	return api.CreateCompletion201JSONResponse{Body: body}, nil
}

func (s *Server) DeleteCompletion(ctx context.Context, req api.DeleteCompletionRequestObject) (api.DeleteCompletionResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.Maintenance.DeleteCompletion(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.ItemId), uuid.UUID(req.EntryId)); err != nil {
		return nil, err
	}
	return api.DeleteCompletion204Response{}, nil
}

func (s *Server) GetMaintenanceStatus(ctx context.Context, req api.GetMaintenanceStatusRequestObject) (api.GetMaintenanceStatusResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.d.Maintenance.Status(ctx, a, uuid.UUID(req.VehicleId))
	if err != nil {
		return nil, err
	}
	out := api.GetMaintenanceStatus200JSONResponse{Items: []api.DueStatus{}, NextCursor: nullable.NewNullNullable[string]()}
	if err := convert(items, &out.Items); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Server) GetDueFeed(ctx context.Context, req api.GetDueFeedRequestObject) (api.GetDueFeedResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	min := ""
	if req.Params.MinLevel != nil {
		min = string(*req.Params.MinLevel)
	}
	items, err := s.d.Maintenance.DueFeed(ctx, a, min)
	if err != nil {
		return nil, err
	}
	out := api.GetDueFeed200JSONResponse{Items: []api.DueStatus{}, NextCursor: nullable.NewNullNullable[string]()}
	if err := convert(items, &out.Items); err != nil {
		return nil, err
	}
	return out, nil
}
