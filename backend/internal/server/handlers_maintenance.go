package server

import (
	"context"

	"github.com/google/uuid"

	"github.com/sh1su/vector/backend/internal/api"
	"github.com/sh1su/vector/backend/internal/maintenance"
	"github.com/sh1su/vector/backend/internal/servicehistory"
)

func maintenanceBody(v maintenance.ItemView) (api.MaintenanceItem, error) {
	var out api.MaintenanceItem
	return out, convert(v, &out)
}

func (s *Server) ListMaintenanceItems(ctx context.Context, req api.ListMaintenanceItemsRequestObject) (api.ListMaintenanceItemsResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.d.Maintenance.ListItems(ctx, a, uuid.UUID(req.VehicleId), req.Params.IncludeDeleted != nil && *req.Params.IncludeDeleted, req.Params.Active)
	if err != nil {
		return nil, err
	}
	var out api.ListMaintenanceItems200JSONResponse
	return out, page(items, nil, &out)
}

func (s *Server) CreateMaintenanceItem(ctx context.Context, req api.CreateMaintenanceItemRequestObject) (api.CreateMaintenanceItemResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var in maintenance.ItemInput
	if err := bodyTo(req.Body, &in); err != nil {
		return nil, err
	}
	v, created, err := s.d.Maintenance.CreateItem(ctx, a, uuid.UUID(req.VehicleId), (*uuid.UUID)(req.Body.Id), in)
	if err != nil {
		return nil, err
	}
	body, err := maintenanceBody(v)
	if err != nil {
		return nil, err
	}
	if !created {
		return api.CreateMaintenanceItem200JSONResponse(body), nil
	}
	return api.CreateMaintenanceItem201JSONResponse{Body: body, Headers: api.CreateMaintenanceItem201ResponseHeaders{ETag: etag(v.Version),
		Location: loc("/vehicles/" + v.VehicleID.String() + "/maintenance-items/" + v.ID.String())}}, nil
}

func (s *Server) GetMaintenanceItem(ctx context.Context, req api.GetMaintenanceItemRequestObject) (api.GetMaintenanceItemResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Maintenance.GetItem(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.ItemId))
	if err != nil {
		return nil, err
	}
	body, err := maintenanceBody(v)
	return api.GetMaintenanceItem200JSONResponse{Body: body, Headers: api.GetMaintenanceItem200ResponseHeaders{ETag: etag(v.Version)}}, err
}

func (s *Server) UpdateMaintenanceItem(ctx context.Context, req api.UpdateMaintenanceItemRequestObject) (api.UpdateMaintenanceItemResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	patch, _, _, err := patchOf(req.Body)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Maintenance.UpdateItem(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.ItemId), req.Params.IfMatch, patch)
	if err != nil {
		return nil, err
	}
	body, err := maintenanceBody(v)
	return api.UpdateMaintenanceItem200JSONResponse{Body: body, Headers: api.UpdateMaintenanceItem200ResponseHeaders{ETag: etag(v.Version)}}, err
}

func (s *Server) DeleteMaintenanceItem(ctx context.Context, req api.DeleteMaintenanceItemRequestObject) (api.DeleteMaintenanceItemResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.Maintenance.DeleteItem(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.ItemId), req.Params.IfMatch); err != nil {
		return nil, err
	}
	return api.DeleteMaintenanceItem204Response{}, nil
}

func (s *Server) ListCompletions(ctx context.Context, req api.ListCompletionsRequestObject) (api.ListCompletionsResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.d.Maintenance.ListCompletions(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.ItemId))
	if err != nil {
		return nil, err
	}
	var out api.ListCompletions200JSONResponse
	return out, page(items, nil, &out)
}

func (s *Server) CreateCompletion(ctx context.Context, req api.CreateCompletionRequestObject) (api.CreateCompletionResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var in maintenance.CompletionInput
	if err := bodyTo(req.Body, &in); err != nil {
		return nil, err
	}
	v, created, err := s.d.Maintenance.Complete(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.ItemId), in, req.Params.IdempotencyKey)
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
	return api.CreateCompletion201JSONResponse{Body: body, Headers: api.CreateCompletion201ResponseHeaders{ETag: etag(1)}}, nil
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
	items, err := s.d.Maintenance.DueStatus(ctx, a, uuid.UUID(req.VehicleId))
	if err != nil {
		return nil, err
	}
	var out api.GetMaintenanceStatus200JSONResponse
	return out, page(items, nil, &out)
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
	var out api.GetDueFeed200JSONResponse
	return out, page(items, nil, &out)
}

// ---------- ServiceHistory ----------

func serviceBody(v servicehistory.View) (api.ServiceEntry, error) {
	var out api.ServiceEntry
	return out, convert(v, &out)
}

func (s *Server) ListServiceEntrys(ctx context.Context, req api.ListServiceEntrysRequestObject) (api.ListServiceEntrysResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	p := req.Params
	f := servicehistory.Filter{From: dateP(p.From), To: dateP(p.To), Q: p.Q, Cursor: p.Cursor, IncludeDeleted: p.IncludeDeleted != nil && *p.IncludeDeleted}
	if p.Limit != nil {
		f.Limit = *p.Limit
	}
	if p.Kind != nil {
		k := string(*p.Kind)
		f.Kind = &k
	}
	items, next, err := s.d.Service.List(ctx, a, uuid.UUID(req.VehicleId), f)
	if err != nil {
		return nil, err
	}
	var out api.ListServiceEntrys200JSONResponse
	return out, page(items, next, &out)
}

func (s *Server) CreateServiceEntry(ctx context.Context, req api.CreateServiceEntryRequestObject) (api.CreateServiceEntryResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var in servicehistory.Input
	if err := bodyTo(req.Body, &in); err != nil {
		return nil, err
	}
	conf := servicehistory.Confirmation{Codes: confirmList(req.Body.ConfirmAnomalies), Reason: str(req.Body.AnomalyReason)}
	v, created, err := s.d.Service.Create(ctx, a, uuid.UUID(req.VehicleId), (*uuid.UUID)(req.Body.Id), in, conf)
	if err != nil {
		return nil, err
	}
	body, err := serviceBody(v)
	if err != nil {
		return nil, err
	}
	if !created {
		return api.CreateServiceEntry200JSONResponse(body), nil
	}
	return api.CreateServiceEntry201JSONResponse{Body: body, Headers: api.CreateServiceEntry201ResponseHeaders{ETag: etag(v.Version),
		Location: loc("/vehicles/" + v.VehicleID.String() + "/service-entries/" + v.ID.String())}}, nil
}

func (s *Server) GetServiceEntry(ctx context.Context, req api.GetServiceEntryRequestObject) (api.GetServiceEntryResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Service.Get(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.EntryId))
	if err != nil {
		return nil, err
	}
	body, err := serviceBody(v)
	return api.GetServiceEntry200JSONResponse{Body: body, Headers: api.GetServiceEntry200ResponseHeaders{ETag: etag(v.Version)}}, err
}

func (s *Server) UpdateServiceEntry(ctx context.Context, req api.UpdateServiceEntryRequestObject) (api.UpdateServiceEntryResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	patch, codes, reason, err := patchOf(req.Body)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Service.Update(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.EntryId), req.Params.IfMatch, patch,
		servicehistory.Confirmation{Codes: codes, Reason: reason})
	if err != nil {
		return nil, err
	}
	body, err := serviceBody(v)
	return api.UpdateServiceEntry200JSONResponse{Body: body, Headers: api.UpdateServiceEntry200ResponseHeaders{ETag: etag(v.Version)}}, err
}

func (s *Server) DeleteServiceEntry(ctx context.Context, req api.DeleteServiceEntryRequestObject) (api.DeleteServiceEntryResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.Service.Delete(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.EntryId), req.Params.IfMatch); err != nil {
		return nil, err
	}
	return api.DeleteServiceEntry204Response{}, nil
}

func (s *Server) GetServiceSummary(ctx context.Context, req api.GetServiceSummaryRequestObject) (api.GetServiceSummaryResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	sum, err := s.d.Service.ServiceSummary(ctx, a, uuid.UUID(req.VehicleId), dateP(req.Params.From), dateP(req.Params.To))
	if err != nil {
		return nil, err
	}
	var body api.ServiceSummary
	if err := convert(sum, &body); err != nil {
		return nil, err
	}
	return api.GetServiceSummary200JSONResponse{Body: body}, nil
}
