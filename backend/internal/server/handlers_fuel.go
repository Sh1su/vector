package server

import (
	"context"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"

	"github.com/sh1su/vector/backend/internal/api"
	"github.com/sh1su/vector/backend/internal/fuel"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

func fillBody(v fuel.View) (api.FuelFill, *string, error) {
	var out api.FuelFill
	err := convert(v, &out)
	et := vehicles.ETag(v.Version)
	return out, &et, err
}

func (s *Server) CreateFuelFill(ctx context.Context, req api.CreateFuelFillRequestObject) (api.CreateFuelFillResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var in fuel.Input
	if err := convert(req.Body, &in); err != nil {
		return nil, problem.BadRequest("")
	}
	c := fuel.Confirmation{Codes: confirmList(req.Body.ConfirmAnomalies), Reason: str(req.Body.AnomalyReason)}
	v, created, err := s.d.Fuel.Create(ctx, a, uuid.UUID(req.VehicleId), (*uuid.UUID)(req.Body.Id), in, c, s.units(ctx, a))
	if err != nil {
		return nil, err
	}
	body, et, err := fillBody(v)
	if err != nil {
		return nil, err
	}
	if !created {
		return api.CreateFuelFill200JSONResponse(body), nil
	}
	loc := apiPrefix + "/vehicles/" + v.VehicleID.String() + "/fuel-fills/" + v.ID.String()
	return api.CreateFuelFill201JSONResponse{Body: body, Headers: api.CreateFuelFill201ResponseHeaders{ETag: et, Location: &loc}}, nil
}

func (s *Server) GetFuelFill(ctx context.Context, req api.GetFuelFillRequestObject) (api.GetFuelFillResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Fuel.Get(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.FillId), s.units(ctx, a))
	if err != nil {
		return nil, err
	}
	body, et, err := fillBody(v)
	if err != nil {
		return nil, err
	}
	return api.GetFuelFill200JSONResponse{Body: body, Headers: api.GetFuelFill200ResponseHeaders{ETag: et}}, nil
}

func (s *Server) UpdateFuelFill(ctx context.Context, req api.UpdateFuelFillRequestObject) (api.UpdateFuelFillResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	patch, codes, reason, err := splitPatch(req.Body)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Fuel.Update(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.FillId), req.Params.IfMatch, patch,
		fuel.Confirmation{Codes: codes, Reason: reason}, s.units(ctx, a))
	if err != nil {
		return nil, err
	}
	body, et, err := fillBody(v)
	if err != nil {
		return nil, err
	}
	return api.UpdateFuelFill200JSONResponse{Body: body, Headers: api.UpdateFuelFill200ResponseHeaders{ETag: et}}, nil
}

func (s *Server) DeleteFuelFill(ctx context.Context, req api.DeleteFuelFillRequestObject) (api.DeleteFuelFillResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.Fuel.Delete(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.FillId), req.Params.IfMatch); err != nil {
		return nil, err
	}
	return api.DeleteFuelFill204Response{}, nil
}

func (s *Server) ListFuelFills(ctx context.Context, req api.ListFuelFillsRequestObject) (api.ListFuelFillsResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	p := req.Params
	f := fuel.ListFilter{From: datePtr(p.From), To: datePtr(p.To), Cursor: p.Cursor, Limit: limitOf(p.Limit),
		IncludeDeleted: p.IncludeDeleted != nil && *p.IncludeDeleted}
	if p.EnergyCarrier != nil {
		f.Carrier = string(*p.EnergyCarrier)
	}
	items, next, err := s.d.Fuel.List(ctx, a, uuid.UUID(req.VehicleId), f, s.units(ctx, a))
	if err != nil {
		return nil, err
	}
	out := api.ListFuelFills200JSONResponse{Items: []api.FuelFill{}, NextCursor: nullable.NewNullNullable[string]()}
	if err := convert(items, &out.Items); err != nil {
		return nil, err
	}
	if next != nil {
		out.NextCursor = nullable.NewNullableWithValue(*next)
	}
	return out, nil
}

func (s *Server) GetConsumptionSummary(ctx context.Context, req api.GetConsumptionSummaryRequestObject) (api.GetConsumptionSummaryResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	p := req.Params
	v, err := s.d.Fuel.Consumption(ctx, a, uuid.UUID(req.VehicleId), string(p.EnergyCarrier), datePtr(p.From), datePtr(p.To), s.units(ctx, a))
	if err != nil {
		return nil, err
	}
	var out api.ConsumptionSummary
	if err := convert(v, &out); err != nil {
		return nil, err
	}
	return api.GetConsumptionSummary200JSONResponse{Body: out}, nil
}
