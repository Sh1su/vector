package server

import (
	"context"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"

	"github.com/sh1su/vector/backend/internal/api"
	"github.com/sh1su/vector/backend/internal/oil"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

func oilBody(v oil.View) (api.OilEntry, *string, error) {
	var out api.OilEntry
	err := convert(v, &out)
	et := vehicles.ETag(v.Version)
	return out, &et, err
}

func (s *Server) CreateOilEntry(ctx context.Context, req api.CreateOilEntryRequestObject) (api.CreateOilEntryResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var in oil.Input
	if err := convert(req.Body, &in); err != nil {
		return nil, problem.BadRequest("")
	}
	c := oil.Confirmation{Codes: confirmList(req.Body.ConfirmAnomalies), Reason: str(req.Body.AnomalyReason)}
	v, created, err := s.d.Oil.Create(ctx, a, uuid.UUID(req.VehicleId), (*uuid.UUID)(req.Body.Id), in, c, s.units(ctx, a))
	if err != nil {
		return nil, err
	}
	body, et, err := oilBody(v)
	if err != nil {
		return nil, err
	}
	if !created {
		return api.CreateOilEntry200JSONResponse(body), nil
	}
	loc := apiPrefix + "/vehicles/" + v.VehicleID.String() + "/oil-entries/" + v.ID.String()
	return api.CreateOilEntry201JSONResponse{Body: body, Headers: api.CreateOilEntry201ResponseHeaders{ETag: et, Location: &loc}}, nil
}

func (s *Server) GetOilEntry(ctx context.Context, req api.GetOilEntryRequestObject) (api.GetOilEntryResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Oil.Get(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.EntryId), s.units(ctx, a))
	if err != nil {
		return nil, err
	}
	body, et, err := oilBody(v)
	if err != nil {
		return nil, err
	}
	return api.GetOilEntry200JSONResponse{Body: body, Headers: api.GetOilEntry200ResponseHeaders{ETag: et}}, nil
}

func (s *Server) UpdateOilEntry(ctx context.Context, req api.UpdateOilEntryRequestObject) (api.UpdateOilEntryResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	patch, codes, reason, err := splitPatch(req.Body)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Oil.Update(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.EntryId), req.Params.IfMatch, patch,
		oil.Confirmation{Codes: codes, Reason: reason}, s.units(ctx, a))
	if err != nil {
		return nil, err
	}
	body, et, err := oilBody(v)
	if err != nil {
		return nil, err
	}
	return api.UpdateOilEntry200JSONResponse{Body: body, Headers: api.UpdateOilEntry200ResponseHeaders{ETag: et}}, nil
}

func (s *Server) DeleteOilEntry(ctx context.Context, req api.DeleteOilEntryRequestObject) (api.DeleteOilEntryResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.Oil.Delete(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.EntryId), req.Params.IfMatch); err != nil {
		return nil, err
	}
	return api.DeleteOilEntry204Response{}, nil
}

func (s *Server) ListOilEntrys(ctx context.Context, req api.ListOilEntrysRequestObject) (api.ListOilEntrysResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	p := req.Params
	f := oil.ListFilter{From: datePtr(p.From), To: datePtr(p.To), Cursor: p.Cursor, Limit: limitOf(p.Limit),
		IncludeDeleted: p.IncludeDeleted != nil && *p.IncludeDeleted}
	items, next, err := s.d.Oil.List(ctx, a, uuid.UUID(req.VehicleId), f, s.units(ctx, a))
	if err != nil {
		return nil, err
	}
	out := api.ListOilEntrys200JSONResponse{Items: []api.OilEntry{}, NextCursor: nullable.NewNullNullable[string]()}
	if err := convert(items, &out.Items); err != nil {
		return nil, err
	}
	if next != nil {
		out.NextCursor = nullable.NewNullableWithValue(*next)
	}
	return out, nil
}

func (s *Server) GetOilStatistics(ctx context.Context, req api.GetOilStatisticsRequestObject) (api.GetOilStatisticsResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Oil.Statistics(ctx, a, uuid.UUID(req.VehicleId), datePtr(req.Params.From), datePtr(req.Params.To), s.units(ctx, a))
	if err != nil {
		return nil, err
	}
	var out api.OilStatistics
	if err := convert(v, &out); err != nil {
		return nil, err
	}
	return api.GetOilStatistics200JSONResponse{Body: out}, nil
}

func (s *Server) ListOilSeries(ctx context.Context, req api.ListOilSeriesRequestObject) (api.ListOilSeriesResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.d.Oil.Series(ctx, a, uuid.UUID(req.VehicleId), s.units(ctx, a))
	if err != nil {
		return nil, err
	}
	out := api.ListOilSeries200JSONResponse{Items: []api.OilSeries{}, NextCursor: nullable.NewNullNullable[string]()}
	if err := convert(items, &out.Items); err != nil {
		return nil, err
	}
	return out, nil
}
