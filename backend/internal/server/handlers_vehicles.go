package server

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"

	"github.com/sh1su/vector/backend/internal/api"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

func vehicleBody(v vehicles.View) (api.Vehicle, *string, error) {
	var out api.Vehicle
	err := convert(v, &out)
	et := vehicles.ETag(v.Version)
	return out, &et, err
}

func confirmList(p *[]string) []string {
	if p == nil {
		return nil
	}
	return *p
}

func (s *Server) ListVehicles(ctx context.Context, req api.ListVehiclesRequestObject) (api.ListVehiclesResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var status *string
	if req.Params.Status != nil {
		st := string(*req.Params.Status)
		status = &st
	}
	limit := 0
	if req.Params.Limit != nil {
		limit = int(*req.Params.Limit)
	}
	page, err := s.d.Vehicles.List(ctx, a, status, (*string)(req.Params.Cursor), limit)
	if err != nil {
		return nil, err
	}
	out := api.ListVehicles200JSONResponse{Items: []api.Vehicle{}}
	for _, v := range page.Items {
		b, _, err := vehicleBody(v)
		if err != nil {
			return nil, err
		}
		out.Items = append(out.Items, b)
	}
	if page.NextCursor != nil {
		out.NextCursor = nullable.NewNullableWithValue(*page.NextCursor)
	} else {
		out.NextCursor = nullable.NewNullNullable[string]()
	}
	return out, nil
}

func (s *Server) CreateVehicle(ctx context.Context, req api.CreateVehicleRequestObject) (api.CreateVehicleResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var in vehicles.Input
	if err := convert(req.Body, &in); err != nil {
		return nil, problem.BadRequest("")
	}
	v, created, err := s.d.Vehicles.Create(ctx, a, (*uuid.UUID)(req.Body.Id), in, confirmList(req.Body.ConfirmAnomalies))
	if err != nil {
		return nil, err
	}
	body, et, err := vehicleBody(v)
	if err != nil {
		return nil, err
	}
	if !created {
		return api.CreateVehicle200JSONResponse(body), nil
	}
	loc := apiPrefix + "/vehicles/" + v.ID
	return api.CreateVehicle201JSONResponse{Body: body, Headers: api.CreateVehicle201ResponseHeaders{ETag: et, Location: &loc}}, nil
}

func (s *Server) GetVehicle(ctx context.Context, req api.GetVehicleRequestObject) (api.GetVehicleResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Vehicles.Get(ctx, a, uuid.UUID(req.VehicleId))
	if err != nil {
		return nil, err
	}
	body, et, err := vehicleBody(v)
	if err != nil {
		return nil, err
	}
	return api.GetVehicle200JSONResponse{Body: body, Headers: api.GetVehicle200ResponseHeaders{ETag: et}}, nil
}

func (s *Server) UpdateVehicle(ctx context.Context, req api.UpdateVehicleRequestObject) (api.UpdateVehicleResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(req.Body)
	if err != nil {
		return nil, problem.BadRequest("")
	}
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	delete(m, "confirm_anomalies")
	delete(m, "anomaly_reason")
	patch, _ := json.Marshal(m)
	v, err := s.d.Vehicles.Update(ctx, a, uuid.UUID(req.VehicleId), req.Params.IfMatch, patch, confirmList(req.Body.ConfirmAnomalies))
	if err != nil {
		return nil, err
	}
	body, et, err := vehicleBody(v)
	if err != nil {
		return nil, err
	}
	return api.UpdateVehicle200JSONResponse{Body: body, Headers: api.UpdateVehicle200ResponseHeaders{ETag: et}}, nil
}

func (s *Server) DeleteVehicle(ctx context.Context, req api.DeleteVehicleRequestObject) (api.DeleteVehicleResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.Vehicles.Delete(ctx, a, uuid.UUID(req.VehicleId), req.Params.IfMatch); err != nil {
		return nil, err
	}
	return api.DeleteVehicle204Response{}, nil
}

func (s *Server) ChangeVehicleStatus(ctx context.Context, req api.ChangeVehicleStatusRequestObject) (api.ChangeVehicleStatusResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var sale *vehicles.DateMoney
	if req.Body.Sale.IsSpecified() && !req.Body.Sale.IsNull() {
		sale = &vehicles.DateMoney{}
		if err := convert(req.Body.Sale.MustGet(), sale); err != nil {
			return nil, problem.BadRequest("")
		}
	}
	v, err := s.d.Vehicles.ChangeStatus(ctx, a, uuid.UUID(req.VehicleId), req.Params.IfMatch, string(req.Body.Status), sale)
	if err != nil {
		return nil, err
	}
	body, et, err := vehicleBody(v)
	if err != nil {
		return nil, err
	}
	return api.ChangeVehicleStatus200JSONResponse{Body: body, Headers: api.ChangeVehicleStatus200ResponseHeaders{ETag: et}}, nil
}
