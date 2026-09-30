package server

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"

	"github.com/sh1su/vector/backend/internal/api"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/odometer"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

func readingBody(v odometer.View) (api.OdometerReading, *string, error) {
	var out api.OdometerReading
	err := convert(v, &out)
	et := vehicles.ETag(v.Version)
	return out, &et, err
}

func precision(p *api.TimePrecision) string {
	if p == nil {
		return ""
	}
	return string(*p)
}

func (s *Server) CreateOdometerReading(ctx context.Context, req api.CreateOdometerReadingRequestObject) (api.CreateOdometerReadingResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	b := req.Body
	in := odometer.Input{ID: (*uuid.UUID)(b.Id), OccurredAt: b.OccurredAt, TimeZone: b.TimeZone, Precision: precision(b.TimePrecision),
		Value: b.Value.Value, Unit: string(b.Value.Unit), Note: str(b.Note), Confirm: confirmList(b.ConfirmAnomalies), Reason: str(b.AnomalyReason)}
	if b.PhotoFileId.IsSpecified() && !b.PhotoFileId.IsNull() {
		id := uuid.UUID(b.PhotoFileId.MustGet())
		in.PhotoFileID = &id
	}
	v, created, err := s.d.Odometer.Create(ctx, a, uuid.UUID(req.VehicleId), in, "manual", nil)
	if err != nil {
		return nil, err
	}
	body, et, err := readingBody(v)
	if err != nil {
		return nil, err
	}
	if !created {
		return api.CreateOdometerReading200JSONResponse(body), nil
	}
	loc := apiPrefix + "/vehicles/" + v.VehicleID.String() + "/odometer/readings/" + v.ID.String()
	return api.CreateOdometerReading201JSONResponse{Body: body, Headers: api.CreateOdometerReading201ResponseHeaders{ETag: et, Location: &loc}}, nil
}

func (s *Server) GetOdometerReading(ctx context.Context, req api.GetOdometerReadingRequestObject) (api.GetOdometerReadingResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Odometer.Get(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.ReadingId))
	if err != nil {
		return nil, err
	}
	body, et, err := readingBody(v)
	if err != nil {
		return nil, err
	}
	return api.GetOdometerReading200JSONResponse{Body: body, Headers: api.GetOdometerReading200ResponseHeaders{ETag: et}}, nil
}

func (s *Server) ListOdometerReadings(ctx context.Context, req api.ListOdometerReadingsRequestObject) (api.ListOdometerReadingsResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	p := req.Params
	f := odometer.ListFilter{Cursor: (*string)(p.Cursor), IncludeSuperseded: p.IncludeSuperseded != nil && *p.IncludeSuperseded}
	if p.Limit != nil {
		f.Limit = int(*p.Limit)
	}
	if p.From != nil {
		t := p.From.Time
		f.From = &t
	}
	if p.To != nil {
		t := p.To.Time.AddDate(0, 0, 1) // inklusive
		f.To = &t
	}
	items, next, err := s.d.Odometer.List(ctx, a, uuid.UUID(req.VehicleId), f)
	if err != nil {
		return nil, err
	}
	out := api.ListOdometerReadings200JSONResponse{Items: []api.OdometerReading{}, NextCursor: nullable.NewNullNullable[string]()}
	for _, v := range items {
		b, _, err := readingBody(v)
		if err != nil {
			return nil, err
		}
		out.Items = append(out.Items, b)
	}
	if next != nil {
		out.NextCursor = nullable.NewNullableWithValue(*next)
	}
	return out, nil
}

func (s *Server) DeleteOdometerReading(ctx context.Context, req api.DeleteOdometerReadingRequestObject) (api.DeleteOdometerReadingResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.Odometer.Delete(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.ReadingId), req.Params.IfMatch); err != nil {
		return nil, err
	}
	return api.DeleteOdometerReading204Response{}, nil
}

func (s *Server) CorrectOdometerReading(ctx context.Context, req api.CorrectOdometerReadingRequestObject) (api.CorrectOdometerReadingResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	b := req.Body
	in := odometer.Input{Reason: b.Reason, Confirm: confirmList(b.ConfirmAnomalies), Precision: precision(b.TimePrecision)}
	// Für die Korrektur ist die Begründung zugleich Begründung bestätigter Befunde, falls keine eigene angegeben ist.
	if b.AnomalyReason != nil {
		in.Reason = *b.AnomalyReason
	}
	hasValue, hasTime := b.Value != nil, b.OccurredAt != nil
	if hasValue {
		in.Value, in.Unit = b.Value.Value, string(b.Value.Unit)
	}
	if hasTime {
		in.OccurredAt = *b.OccurredAt
		in.TimeZone = str(b.TimeZone)
	}
	if in.Reason == "" {
		in.Reason = b.Reason
	}
	v, err := s.d.Odometer.Correct(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.ReadingId), req.Params.IfMatch, in, hasValue, hasTime)
	if err != nil {
		return nil, err
	}
	body, et, err := readingBody(v)
	if err != nil {
		return nil, err
	}
	loc := apiPrefix + "/vehicles/" + v.VehicleID.String() + "/odometer/readings/" + v.ID.String()
	return api.CorrectOdometerReading201JSONResponse{Body: body, Headers: api.CorrectOdometerReading201ResponseHeaders{ETag: et, Location: &loc}}, nil
}

// displayUnit bestimmt die Anzeigeeinheit des Nutzers für Zählerstände.
func (s *Server) displayUnit(ctx context.Context, a kernel.Actor, meter string) string {
	if meter == odometer.MeterEngineHours {
		return "h"
	}
	st, err := s.d.Identity.Settings(ctx, a.AccountID)
	if err == nil {
		if du, ok := st["display_units"].(map[string]any); ok {
			if d, ok := du["distance"].(string); ok && (d == "km" || d == "mi") {
				return d
			}
		}
	}
	return "km"
}

func display(canonical int64, unit string) api.DisplayValue {
	v, _ := kernel.FromCanonical(canonical, unit)
	return api.DisplayValue{Value: math.Round(v*10) / 10, Unit: unit}
}

func quantity(canonical int64, meter string) api.Quantity {
	cu := api.CanonicalUnit(kernel.UnitMeter)
	if meter == odometer.MeterEngineHours {
		cu = api.CanonicalUnit(kernel.UnitSecond)
	}
	return api.Quantity{Canonical: canonical, CanonicalUnit: cu}
}

func (s *Server) valueBody(ctx context.Context, a kernel.Actor, v odometer.Value, meter string) api.OdometerValue {
	out := api.OdometerValue{At: v.At, Kind: api.OdometerValueKind(v.Kind)}
	if v.Kind == odometer.KindUnknown {
		out.MeterValue, out.Total, out.Display = nullable.NewNullNullable[api.Quantity](), nullable.NewNullNullable[api.Quantity](), nullable.NewNullNullable[api.DisplayValue]()
		return out
	}
	out.MeterValue = nullable.NewNullableWithValue(quantity(v.Meter, meter))
	out.Total = nullable.NewNullableWithValue(quantity(v.Total, meter))
	out.SegmentId = nullable.NewNullableWithValue(v.SegmentID)
	if v.ReadingID != nil {
		out.ReadingId = nullable.NewNullableWithValue(*v.ReadingID)
	}
	out.Display = nullable.NewNullableWithValue(display(v.Meter, s.displayUnit(ctx, a, meter)))
	return out
}

func (s *Server) GetCurrentOdometer(ctx context.Context, req api.GetCurrentOdometerRequestObject) (api.GetCurrentOdometerResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	segs, valid, meta, err := s.d.Odometer.Query(ctx, a, uuid.UUID(req.VehicleId))
	if err != nil {
		return nil, err
	}
	return api.GetCurrentOdometer200JSONResponse{Body: s.valueBody(ctx, a, odometer.Current(valid, segs), meta.UsageMeter)}, nil
}

func (s *Server) GetOdometerValueAt(ctx context.Context, req api.GetOdometerValueAtRequestObject) (api.GetOdometerValueAtResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	segs, valid, meta, err := s.d.Odometer.Query(ctx, a, uuid.UUID(req.VehicleId))
	if err != nil {
		return nil, err
	}
	return api.GetOdometerValueAt200JSONResponse{Body: s.valueBody(ctx, a, odometer.ValueAt(valid, segs, req.Params.At), meta.UsageMeter)}, nil
}

func (s *Server) GetOdometerDistance(ctx context.Context, req api.GetOdometerDistanceRequestObject) (api.GetOdometerDistanceResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	from, to := req.Params.From, req.Params.To
	if to.Before(from) {
		return nil, problem.Validation(problem.FieldError{Pointer: "/to", Code: "order", Message: "`to` liegt vor `from`."})
	}
	segs, valid, meta, err := s.d.Odometer.Query(ctx, a, uuid.UUID(req.VehicleId))
	if err != nil {
		return nil, err
	}
	d := odometer.DistanceBetween(valid, segs, from, to)
	out := api.OdometerDistance{From: from, To: to, Status: api.OdometerDistanceStatusUnknown,
		Distance: nullable.NewNullNullable[api.Quantity](), Display: nullable.NewNullNullable[api.DisplayValue]()}
	var flags []api.OdometerDistanceFlags
	if d.Known {
		out.Status = api.OdometerDistanceStatusKnown
		out.Distance = nullable.NewNullableWithValue(quantity(d.Meters, meta.UsageMeter))
		out.Display = nullable.NewNullableWithValue(display(d.Meters, s.displayUnit(ctx, a, meta.UsageMeter)))
		if d.FromKind == odometer.KindInterpolated || d.ToKind == odometer.KindInterpolated {
			flags = append(flags, api.OdometerDistanceFlagsInterpolated)
		}
		if d.FromKind == odometer.KindCarriedForward || d.ToKind == odometer.KindCarriedForward {
			flags = append(flags, api.OdometerDistanceFlagsCarriedForward)
		}
		if d.HasAnomaly {
			flags = append(flags, api.OdometerDistanceFlagsConfirmedAnomalyInRange)
		}
	}
	if flags == nil {
		flags = []api.OdometerDistanceFlags{}
	}
	out.Flags = &flags
	return api.GetOdometerDistance200JSONResponse{Body: out}, nil
}

func (s *Server) ListOdometerSegments(ctx context.Context, req api.ListOdometerSegmentsRequestObject) (api.ListOdometerSegmentsResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	segs, err := s.d.Odometer.ListSegments(ctx, a, uuid.UUID(req.VehicleId))
	if err != nil {
		return nil, err
	}
	out := api.ListOdometerSegments200JSONResponse{Items: []api.OdometerSegment{}, NextCursor: nullable.NewNullNullable[string]()}
	if err := convert(segs, &out.Items); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Server) CreateOdometerSegment(ctx context.Context, req api.CreateOdometerSegmentRequestObject) (api.CreateOdometerSegmentResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	b := req.Body
	in := odometer.SegmentInput{StartedAt: b.StartedAt, TimeZone: b.TimeZone, StartMeter: b.StartMeterValue.Value, StartUnit: string(b.StartMeterValue.Unit),
		Reason: string(b.Reason), Note: str(b.Note), Confirm: confirmList(b.ConfirmAnomalies), AnomalyReason: str(b.AnomalyReason)}
	if _, err := time.LoadLocation(b.TimeZone); err != nil {
		return nil, problem.Validation(problem.FieldError{Pointer: "/time_zone", Code: "time_zone"})
	}
	if b.OldFinalValue != nil {
		in.OldFinal = &odometer.Input{Value: b.OldFinalValue.Value, Unit: string(b.OldFinalValue.Unit)}
	}
	seg, err := s.d.Odometer.CreateSegment(ctx, a, uuid.UUID(req.VehicleId), in)
	if err != nil {
		return nil, err
	}
	var body api.OdometerSegment
	if err := convert(seg, &body); err != nil {
		return nil, err
	}
	return api.CreateOdometerSegment201JSONResponse{Body: body}, nil
}
