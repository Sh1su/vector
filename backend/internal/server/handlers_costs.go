package server

import (
	"context"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/sh1su/vector/backend/internal/api"
	"github.com/sh1su/vector/backend/internal/costs"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

func costEntryBody(v costs.EntryView) (api.CostEntry, error) {
	var out api.CostEntry
	return out, convert(v, &out)
}

func (s *Server) ListCostEntrys(ctx context.Context, req api.ListCostEntrysRequestObject) (api.ListCostEntrysResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	p := req.Params
	f := costs.EntryFilter{From: dateP(p.From), To: dateP(p.To), Cursor: p.Cursor, IncludeDeleted: p.IncludeDeleted != nil && *p.IncludeDeleted}
	if p.Limit != nil {
		f.Limit = *p.Limit
	}
	if p.Category != nil {
		c := string(*p.Category)
		f.Category = &c
	}
	items, next, err := s.d.Costs.ListEntries(ctx, a, uuid.UUID(req.VehicleId), f)
	if err != nil {
		return nil, err
	}
	var out api.ListCostEntrys200JSONResponse
	return out, page(items, next, &out)
}

func (s *Server) CreateCostEntry(ctx context.Context, req api.CreateCostEntryRequestObject) (api.CreateCostEntryResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var in costs.EntryInput
	if err := bodyTo(req.Body, &in); err != nil {
		return nil, err
	}
	v, created, err := s.d.Costs.CreateEntry(ctx, a, uuid.UUID(req.VehicleId), (*uuid.UUID)(req.Body.Id), in)
	if err != nil {
		return nil, err
	}
	body, err := costEntryBody(v)
	if err != nil {
		return nil, err
	}
	if !created {
		return api.CreateCostEntry200JSONResponse(body), nil
	}
	return api.CreateCostEntry201JSONResponse{Body: body, Headers: api.CreateCostEntry201ResponseHeaders{ETag: etag(v.Version),
		Location: loc("/vehicles/" + v.VehicleID.String() + "/cost-entries/" + v.ID.String())}}, nil
}

func (s *Server) GetCostEntry(ctx context.Context, req api.GetCostEntryRequestObject) (api.GetCostEntryResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Costs.GetEntry(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.EntryId))
	if err != nil {
		return nil, err
	}
	body, err := costEntryBody(v)
	return api.GetCostEntry200JSONResponse{Body: body, Headers: api.GetCostEntry200ResponseHeaders{ETag: etag(v.Version)}}, err
}

func (s *Server) UpdateCostEntry(ctx context.Context, req api.UpdateCostEntryRequestObject) (api.UpdateCostEntryResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	patch, _, _, err := patchOf(req.Body)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Costs.UpdateEntry(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.EntryId), req.Params.IfMatch, patch)
	if err != nil {
		return nil, err
	}
	body, err := costEntryBody(v)
	return api.UpdateCostEntry200JSONResponse{Body: body, Headers: api.UpdateCostEntry200ResponseHeaders{ETag: etag(v.Version)}}, err
}

func (s *Server) DeleteCostEntry(ctx context.Context, req api.DeleteCostEntryRequestObject) (api.DeleteCostEntryResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.Costs.DeleteEntry(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.EntryId), req.Params.IfMatch); err != nil {
		return nil, err
	}
	return api.DeleteCostEntry204Response{}, nil
}

func costPlanBody(v costs.PlanView) (api.CostPlan, error) {
	var out api.CostPlan
	return out, convert(v, &out)
}

func (s *Server) ListCostPlans(ctx context.Context, req api.ListCostPlansRequestObject) (api.ListCostPlansResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.d.Costs.ListPlans(ctx, a, uuid.UUID(req.VehicleId), req.Params.IncludeDeleted != nil && *req.Params.IncludeDeleted)
	if err != nil {
		return nil, err
	}
	var out api.ListCostPlans200JSONResponse
	return out, page(items, nil, &out)
}

func (s *Server) CreateCostPlan(ctx context.Context, req api.CreateCostPlanRequestObject) (api.CreateCostPlanResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var in costs.PlanInput
	if err := bodyTo(req.Body, &in); err != nil {
		return nil, err
	}
	v, created, err := s.d.Costs.CreatePlan(ctx, a, uuid.UUID(req.VehicleId), (*uuid.UUID)(req.Body.Id), in)
	if err != nil {
		return nil, err
	}
	body, err := costPlanBody(v)
	if err != nil {
		return nil, err
	}
	if !created {
		return api.CreateCostPlan200JSONResponse(body), nil
	}
	return api.CreateCostPlan201JSONResponse{Body: body, Headers: api.CreateCostPlan201ResponseHeaders{ETag: etag(v.Version),
		Location: loc("/vehicles/" + v.VehicleID.String() + "/cost-plans/" + v.ID.String())}}, nil
}

func (s *Server) GetCostPlan(ctx context.Context, req api.GetCostPlanRequestObject) (api.GetCostPlanResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Costs.GetPlan(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.PlanId))
	if err != nil {
		return nil, err
	}
	body, err := costPlanBody(v)
	return api.GetCostPlan200JSONResponse{Body: body, Headers: api.GetCostPlan200ResponseHeaders{ETag: etag(v.Version)}}, err
}

func (s *Server) UpdateCostPlan(ctx context.Context, req api.UpdateCostPlanRequestObject) (api.UpdateCostPlanResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	patch, _, _, err := patchOf(req.Body)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Costs.UpdatePlan(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.PlanId), req.Params.IfMatch, patch)
	if err != nil {
		return nil, err
	}
	body, err := costPlanBody(v)
	return api.UpdateCostPlan200JSONResponse{Body: body, Headers: api.UpdateCostPlan200ResponseHeaders{ETag: etag(v.Version)}}, err
}

func (s *Server) DeleteCostPlan(ctx context.Context, req api.DeleteCostPlanRequestObject) (api.DeleteCostPlanResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.Costs.DeletePlan(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.PlanId), req.Params.IfMatch); err != nil {
		return nil, err
	}
	return api.DeleteCostPlan204Response{}, nil
}

func (s *Server) ListCostOccurrences(ctx context.Context, req api.ListCostOccurrencesRequestObject) (api.ListCostOccurrencesResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	items, more, err := s.d.Costs.PendingOccurrences(ctx, a, uuid.UUID(req.VehicleId))
	if err != nil {
		return nil, err
	}
	var body api.CostOccurrenceList
	if err := convert(map[string]any{"items": items, "more_open": more}, &body); err != nil {
		return nil, err
	}
	return api.ListCostOccurrences200JSONResponse{Body: body}, nil
}

func (s *Server) ConfirmCostOccurrence(ctx context.Context, req api.ConfirmCostOccurrenceRequestObject) (api.ConfirmCostOccurrenceResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var in struct {
		Amount     *kernel.Money `json:"amount"`
		IncurredOn *string       `json:"incurred_on"`
	}
	if err := bodyTo(req.Body, &in); err != nil {
		return nil, err
	}
	var inc *time.Time
	if in.IncurredOn != nil {
		d, err := kernel.ParseDate(*in.IncurredOn)
		if err != nil {
			return nil, problem.Validation(problem.FieldError{Pointer: "/incurred_on", Code: "date"})
		}
		inc = &d
	}
	v, _, err := s.d.Costs.ConfirmOccurrence(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.PlanId), kernel.Date(req.DueOn.Time), in.Amount, inc)
	if err != nil {
		return nil, err
	}
	body, err := costEntryBody(v)
	return api.ConfirmCostOccurrence200JSONResponse{Body: body, Headers: api.ConfirmCostOccurrence200ResponseHeaders{ETag: etag(v.Version)}}, err
}

func (s *Server) DismissCostOccurrence(ctx context.Context, req api.DismissCostOccurrenceRequestObject) (api.DismissCostOccurrenceResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.Costs.DismissOccurrence(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.PlanId), kernel.Date(req.DueOn.Time), req.Body.Reason); err != nil {
		return nil, err
	}
	return api.DismissCostOccurrence204Response{}, nil
}

func (s *Server) reportQuery(ctx context.Context, a kernel.Actor, from, to *openapi_types.Date, groupBy, allocation *string) costs.ReportQuery {
	rq := costs.ReportQuery{From: dateP(from), To: dateP(to), DistanceUnit: s.displayUnit(ctx, a, "distance")}
	if groupBy != nil {
		rq.GroupBy = *groupBy
	}
	if allocation != nil {
		rq.Allocation = *allocation
	}
	return rq
}

func (s *Server) GetCostReport(ctx context.Context, req api.GetCostReportRequestObject) (api.GetCostReportResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	p := req.Params
	rep, err := s.d.Costs.CostReport(ctx, a, uuid.UUID(req.VehicleId), s.reportQuery(ctx, a, p.From, p.To, (*string)(p.GroupBy), (*string)(p.Allocation)))
	if err != nil {
		return nil, err
	}
	var body api.CostReport
	if err := convert(rep, &body); err != nil {
		return nil, err
	}
	return api.GetCostReport200JSONResponse{Body: body}, nil
}

func (s *Server) GetFleetCostReport(ctx context.Context, req api.GetFleetCostReportRequestObject) (api.GetFleetCostReportResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	p := req.Params
	rq := s.reportQuery(ctx, a, p.From, p.To, (*string)(p.GroupBy), (*string)(p.Allocation))
	items, err := s.d.Costs.FleetCostReport(ctx, a, rq)
	if err != nil {
		return nil, err
	}
	today := kernel.FormatDate(kernel.Date(time.Now()))
	from, to := today, today
	if len(items) > 0 {
		from, to = items[0].Report.From, items[0].Report.To
	}
	var body api.FleetCostReport
	if err := convert(map[string]any{"from": from, "to": to, "vehicles": items}, &body); err != nil {
		return nil, err
	}
	return api.GetFleetCostReport200JSONResponse{Body: body}, nil
}
