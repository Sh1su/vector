package server

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/sh1su/vector/backend/internal/api"
	"github.com/sh1su/vector/backend/internal/platform/mergepatch"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/trips"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

func tripBody(v trips.View) (api.Trip, error) {
	var out api.Trip
	return out, convert(v, &out)
}

// tripPatch liefert eine Funktion, die den Patch auf die Fahrt anwendet.
func tripPatch(patch []byte) func(*trips.Input) error {
	return func(in *trips.Input) error {
		var out trips.Input
		if err := mergepatch.Apply(*in, patch, &out); err != nil {
			return problem.BadRequest("Ungültiger Patch.")
		}
		*in = out
		return nil
	}
}

func (s *Server) ListTrips(ctx context.Context, req api.ListTripsRequestObject) (api.ListTripsResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	p := req.Params
	f := trips.Filter{From: dateP(p.From), To: dateP(p.To), Cursor: p.Cursor, IncludeHistory: p.IncludeHistory != nil && *p.IncludeHistory}
	if p.Limit != nil {
		f.Limit = *p.Limit
	}
	items, next, err := s.d.Trips.List(ctx, a, uuid.UUID(req.VehicleId), f)
	if err != nil {
		return nil, err
	}
	var out api.ListTrips200JSONResponse
	return out, page(items, next, &out)
}

func (s *Server) CreateTrip(ctx context.Context, req api.CreateTripRequestObject) (api.CreateTripResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var in trips.Input
	if err := bodyTo(req.Body, &in); err != nil {
		return nil, err
	}
	if in.EndedAt == nil || in.EndOdometer == nil {
		return nil, problem.Validation(problem.FieldError{Pointer: "/ended_at", Code: "required", Message: "Nachträglich erfasste Fahrten brauchen Ende und Endstand."})
	}
	conf := trips.Confirmation{Codes: confirmList(req.Body.ConfirmAnomalies), Reason: str(req.Body.AnomalyReason)}
	photos := trips.Photos{Start: nullUUID(req.Body.StartPhotoId), End: nullUUID(req.Body.EndPhotoId)}
	v, created, err := s.d.Trips.Record(ctx, a, uuid.UUID(req.VehicleId), (*uuid.UUID)(req.Body.Id), in, photos, conf)
	if err != nil {
		return nil, err
	}
	body, err := tripBody(v)
	if err != nil {
		return nil, err
	}
	if !created {
		return api.CreateTrip200JSONResponse(body), nil
	}
	return api.CreateTrip201JSONResponse{Body: body, Headers: api.CreateTrip201ResponseHeaders{ETag: etag(v.Version),
		Location: loc("/vehicles/" + v.VehicleID.String() + "/trips/" + v.ID.String())}}, nil
}

func (s *Server) StartTrip(ctx context.Context, req api.StartTripRequestObject) (api.StartTripResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var in trips.Input
	if err := bodyTo(req.Body, &in); err != nil {
		return nil, err
	}
	in.EndedAt, in.EndOdometer, in.EndLocation = nil, nil, nil
	conf := trips.Confirmation{Codes: confirmList(req.Body.ConfirmAnomalies), Reason: str(req.Body.AnomalyReason)}
	v, _, err := s.d.Trips.Record(ctx, a, uuid.UUID(req.VehicleId), (*uuid.UUID)(req.Body.Id), in, trips.Photos{Start: nullUUID(req.Body.StartPhotoId)}, conf)
	if err != nil {
		return nil, err
	}
	body, err := tripBody(v)
	if err != nil {
		return nil, err
	}
	return api.StartTrip201JSONResponse{Body: body, Headers: api.StartTrip201ResponseHeaders{ETag: etag(v.Version),
		Location: loc("/vehicles/" + v.VehicleID.String() + "/trips/" + v.ID.String())}}, nil
}

func (s *Server) GetTrip(ctx context.Context, req api.GetTripRequestObject) (api.GetTripResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Trips.Get(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.TripId))
	if err != nil {
		return nil, err
	}
	body, err := tripBody(v)
	return api.GetTrip200JSONResponse{Body: body, Headers: api.GetTrip200ResponseHeaders{ETag: etag(v.Version)}}, err
}

func (s *Server) UpdateOpenTrip(ctx context.Context, req api.UpdateOpenTripRequestObject) (api.UpdateOpenTripResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	patch, codes, reason, err := patchOf(req.Body)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Trips.UpdateOpen(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.TripId), req.Params.IfMatch, tripPatch(patch),
		trips.Confirmation{Codes: codes, Reason: reason})
	if err != nil {
		return nil, err
	}
	body, err := tripBody(v)
	return api.UpdateOpenTrip200JSONResponse{Body: body, Headers: api.UpdateOpenTrip200ResponseHeaders{ETag: etag(v.Version)}}, err
}

func (s *Server) FinishTrip(ctx context.Context, req api.FinishTripRequestObject) (api.FinishTripResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	b := req.Body
	end := vehicles.Q{Value: b.EndOdometer.Value, Unit: string(b.EndOdometer.Unit)}
	var endLoc *string
	if b.EndLocation.IsSpecified() && !b.EndLocation.IsNull() {
		l := b.EndLocation.MustGet()
		endLoc = &l
	}
	v, err := s.d.Trips.Finish(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.TripId), req.Params.IfMatch, b.EndedAt, end, endLoc, trips.FinishExtra{Photo: nullUUID(b.EndPhotoId), Note: b.Note},
		trips.Confirmation{Codes: confirmList(b.ConfirmAnomalies), Reason: str(b.AnomalyReason)})
	if err != nil {
		return nil, err
	}
	body, err := tripBody(v)
	return api.FinishTrip200JSONResponse{Body: body, Headers: api.FinishTrip200ResponseHeaders{ETag: etag(v.Version)}}, err
}

func (s *Server) CorrectTrip(ctx context.Context, req api.CorrectTripRequestObject) (api.CorrectTripResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	patch, codes, anomalyReason, err := patchOf(req.Body)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	_ = json.Unmarshal(patch, &m)
	delete(m, "reason")
	patch, _ = json.Marshal(m)
	v, err := s.d.Trips.Correct(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.TripId), req.Params.IfMatch, tripPatch(patch), req.Body.Reason,
		trips.Confirmation{Codes: codes, Reason: anomalyReason})
	if err != nil {
		return nil, err
	}
	body, err := tripBody(v)
	if err != nil {
		return nil, err
	}
	return api.CorrectTrip201JSONResponse{Body: body, Headers: api.CorrectTrip201ResponseHeaders{ETag: etag(v.Version),
		Location: loc("/vehicles/" + v.VehicleID.String() + "/trips/" + v.ID.String())}}, nil
}

func (s *Server) CancelTrip(ctx context.Context, req api.CancelTripRequestObject) (api.CancelTripResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Trips.Cancel(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.TripId), req.Params.IfMatch, req.Body.Reason)
	if err != nil {
		return nil, err
	}
	body, err := tripBody(v)
	return api.CancelTrip200JSONResponse{Body: body, Headers: api.CancelTrip200ResponseHeaders{ETag: etag(v.Version)}}, err
}

func (s *Server) GetTripHistory(ctx context.Context, req api.GetTripHistoryRequestObject) (api.GetTripHistoryResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.d.Trips.History(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.TripId))
	if err != nil {
		return nil, err
	}
	var out api.GetTripHistory200JSONResponse
	return out, page(items, nil, &out)
}

func (s *Server) GetTripReport(ctx context.Context, req api.GetTripReportRequestObject) (api.GetTripReportResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	rep, err := s.d.Trips.Report(ctx, a, uuid.UUID(req.VehicleId), dateP(req.Params.From), dateP(req.Params.To))
	if err != nil {
		return nil, err
	}
	var body api.TripReport
	if err := convert(rep, &body); err != nil {
		return nil, err
	}
	return api.GetTripReport200JSONResponse{Body: body}, nil
}

func (s *Server) ListTripCategories(ctx context.Context, req api.ListTripCategoriesRequestObject) (api.ListTripCategoriesResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.d.Trips.ListCategories(ctx, a, uuid.UUID(req.VehicleId))
	if err != nil {
		return nil, err
	}
	var out api.ListTripCategories200JSONResponse
	return out, page(items, nil, &out)
}

func (s *Server) CreateTripCategory(ctx context.Context, req api.CreateTripCategoryRequestObject) (api.CreateTripCategoryResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var in trips.CategoryInput
	if err := bodyTo(req.Body, &in); err != nil {
		return nil, err
	}
	v, err := s.d.Trips.CreateCategory(ctx, a, uuid.UUID(req.VehicleId), in)
	if err != nil {
		return nil, err
	}
	var body api.TripCategory
	if err := convert(v, &body); err != nil {
		return nil, err
	}
	return api.CreateTripCategory201JSONResponse{Body: body, Headers: api.CreateTripCategory201ResponseHeaders{ETag: etag(v.Version)}}, nil
}

func (s *Server) UpdateTripCategory(ctx context.Context, req api.UpdateTripCategoryRequestObject) (api.UpdateTripCategoryResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var patch map[string]any
	if err := convert(req.Body, &patch); err != nil {
		return nil, problem.BadRequest("")
	}
	v, err := s.d.Trips.UpdateCategory(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.CategoryId), req.Params.IfMatch, patch)
	if err != nil {
		return nil, err
	}
	var body api.TripCategory
	if err := convert(v, &body); err != nil {
		return nil, err
	}
	return api.UpdateTripCategory200JSONResponse{Body: body, Headers: api.UpdateTripCategory200ResponseHeaders{ETag: etag(v.Version)}}, nil
}
