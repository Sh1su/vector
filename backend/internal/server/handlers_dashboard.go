package server

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/sh1su/vector/backend/internal/api"
	"github.com/sh1su/vector/backend/internal/assistant"
	"github.com/sh1su/vector/backend/internal/odometer"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

// nullUUID liest eine optionale, nullbare UUID aus dem Request.
func nullUUID(n nullable.Nullable[openapi_types.UUID]) *uuid.UUID {
	if !n.IsSpecified() || n.IsNull() {
		return nil
	}
	id := uuid.UUID(n.MustGet())
	return &id
}

// ReadDashboardPhoto wertet ein hochgeladenes Tachofoto aus (nichts wird gespeichert).
func (s *Server) ReadDashboardPhoto(ctx context.Context, req api.ReadDashboardPhotoRequestObject) (api.ReadDashboardPhotoResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	vehicleID, fileID := uuid.UUID(req.VehicleId), uuid.UUID(req.FileId)
	if s.d.Documents == nil {
		return nil, problem.NotFound()
	}
	f, err := s.d.Documents.GetFile(ctx, a, vehicleID, fileID)
	if err != nil {
		return nil, err
	}
	hasPreview := false
	for _, d := range f.Derivatives {
		hasPreview = hasPreview || d.Kind == "preview"
	}
	if !hasPreview {
		return nil, problem.Validation(problem.FieldError{Pointer: "/file_id", Code: "not_image", Message: "Das Tachofoto muss ein JPEG, PNG oder WebP sein."})
	}
	c, err := s.d.Documents.Open(ctx, a, vehicleID, fileID, "preview")
	if err != nil {
		return nil, err
	}
	img, err := io.ReadAll(io.LimitReader(c.Reader, 10<<20))
	c.Reader.Close()
	if err != nil {
		return nil, err
	}

	capturedAt := f.ReceivedAt
	if f.CapturedAtClient != nil {
		capturedAt = *f.CapturedAtClient
	}
	segs, valid, meta, err := s.d.Odometer.Query(ctx, a, vehicleID)
	if err != nil {
		return nil, err
	}
	unit := s.displayUnit(ctx, a, meta.UsageMeter)
	hint := assistant.DashboardHint{VehicleName: meta.DisplayName, Unit: unit}
	last := odometer.ValueAt(valid, segs, capturedAt)
	var lastQ *api.QuantityInput
	if last.Kind != odometer.KindUnknown && meta.UsageMeter != odometer.MeterEngineHours {
		d := display(last.Meter, unit)
		hint.LastOdometer = &d.Value
		lastQ = &api.QuantityInput{Value: d.Value, Unit: api.UnitCode(unit)}
	}

	r, err := s.d.Assistant.ReadDashboard(ctx, a, img, c.MediaType, hint)
	if err != nil {
		return nil, err
	}
	out := api.DashboardReading{FileId: openapi_types.UUID(fileID), Readable: r.Readable, CapturedAt: capturedAt,
		Confidence: api.DashboardReadingConfidence(r.Confidence), Notes: r.Notes, WarningLights: r.WarningLights,
		Odometer: nullable.NewNullNullable[api.QuantityInput](), TripMeter: nullable.NewNullNullable[api.QuantityInput](),
		Range: nullable.NewNullNullable[api.QuantityInput](), LastOdometer: nullable.NewNullNullable[api.QuantityInput]()}
	odoUnit := unit
	if r.OdometerUnit != nil {
		odoUnit = *r.OdometerUnit
	}
	if r.OdometerValue != nil {
		out.Odometer = nullable.NewNullableWithValue(api.QuantityInput{Value: *r.OdometerValue, Unit: api.UnitCode(odoUnit)})
	}
	if r.TripMeterValue != nil {
		out.TripMeter = nullable.NewNullableWithValue(api.QuantityInput{Value: *r.TripMeterValue, Unit: api.UnitCode(odoUnit)})
	}
	if r.RangeValue != nil {
		ru := odoUnit
		if r.RangeUnit != nil {
			ru = *r.RangeUnit
		}
		out.Range = nullable.NewNullableWithValue(api.QuantityInput{Value: *r.RangeValue, Unit: api.UnitCode(ru)})
	}
	if lastQ != nil {
		out.LastOdometer = nullable.NewNullableWithValue(*lastQ)
	}
	if r.FuelLevelPercent != nil {
		out.FuelLevelPercent = nullable.NewNullableWithValue(*r.FuelLevelPercent)
	}
	if r.OutsideTemperature != nil {
		out.OutsideTemperatureC = nullable.NewNullableWithValue(*r.OutsideTemperature)
	}
	if r.DashboardTime != nil {
		out.DashboardTime = nullable.NewNullableWithValue(*r.DashboardTime)
	}
	out.Summary = dashboardSummary(r)
	return api.ReadDashboardPhoto200JSONResponse(out), nil
}

// dashboardSummary fasst Nebenanzeigen für die Notiz der Fahrt zusammen.
func dashboardSummary(r assistant.DashboardReading) string {
	var parts []string
	if r.FuelLevelPercent != nil {
		parts = append(parts, fmt.Sprintf("Tank %.0f %%", *r.FuelLevelPercent))
	}
	if r.RangeValue != nil {
		u := "km"
		if r.RangeUnit != nil {
			u = *r.RangeUnit
		}
		parts = append(parts, fmt.Sprintf("Reichweite %.0f %s", *r.RangeValue, u))
	}
	if r.OutsideTemperature != nil {
		parts = append(parts, strings.Replace(fmt.Sprintf("%.1f °C", *r.OutsideTemperature), ".", ",", 1))
	}
	if len(r.WarningLights) > 0 {
		parts = append(parts, "Warnleuchten: "+strings.Join(r.WarningLights, ", "))
	}
	return strings.Join(parts, ", ")
}
