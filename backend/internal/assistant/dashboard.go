package assistant

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/sh1su/vector/backend/internal/assistant/store"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

// DashboardHint ist Kontext für die Erkennung eines Tachofotos.
type DashboardHint struct {
	VehicleName  string
	LastOdometer *float64 // letzter bekannter Stand in Unit
	Unit         string   // km | mi
}

// DashboardReading sind die aus einem Tachofoto gelesenen Werte. Nichts davon wird
// gespeichert; die App übernimmt die Werte erst nach Bestätigung (ADR-026).
type DashboardReading struct {
	Readable           bool     `json:"readable"`
	OdometerValue      *float64 `json:"odometer_value"`
	OdometerUnit       *string  `json:"odometer_unit"`
	TripMeterValue     *float64 `json:"trip_meter_value"`
	FuelLevelPercent   *float64 `json:"fuel_level_percent"`
	RangeValue         *float64 `json:"range_value"`
	RangeUnit          *string  `json:"range_unit"`
	OutsideTemperature *float64 `json:"outside_temperature_c"`
	DashboardTime      *string  `json:"dashboard_time"`
	WarningLights      []string `json:"warning_lights"`
	Confidence         string   `json:"confidence"`
	Notes              string   `json:"notes"`
}

const dashboardTool = "report_dashboard"

const dashboardPrompt = `Du liest Fotos von Fahrzeug-Kombiinstrumenten (Tacho) aus. Melde die Werte ausschließlich über das Werkzeug report_dashboard.
Regeln:
- odometer_value ist der Gesamtkilometerstand (Odometer, meist 5–7 Stellen, oft mit „km“). Nicht mit dem Tageskilometerzähler (Trip, oft mit Nachkommastelle, „Trip A/B“) verwechseln; den meldest du als trip_meter_value.
- Nur Werte melden, die im Bild wirklich zu lesen sind; sonst null. Nichts schätzen oder aus dem Hinweis übernehmen.
- fuel_level_percent: Tankanzeige als Prozent (z. B. 3/4 = 75). Bei Elektroautos den Akkustand.
- range_value: angezeigte Reichweite; dashboard_time: angezeigte Uhrzeit als HH:MM.
- warning_lights: aktive Warn-/Kontrollleuchten kurz auf Deutsch (z. B. „Motorkontrollleuchte“, „Serviceanzeige“).
- confidence: high, wenn der Gesamtkilometerstand eindeutig lesbar ist; medium bei Unsicherheit einzelner Ziffern; low sonst.
- readable=false, wenn das Bild kein Kombiinstrument zeigt oder nichts lesbar ist.
- notes: kurzer deutscher Hinweis, z. B. welche Ziffer unsicher ist oder dass der Stand unter dem letzten bekannten liegt.`

func numProp(desc string) map[string]any {
	return map[string]any{"type": []string{"number", "null"}, "description": desc}
}

func strProp(desc string, enum ...string) map[string]any {
	p := map[string]any{"type": []string{"string", "null"}, "description": desc}
	if len(enum) > 0 {
		vals := []any{}
		for _, e := range enum {
			vals = append(vals, e)
		}
		p["enum"] = append(vals, nil)
	}
	return p
}

func dashboardToolParam() anthropic.ToolUnionParam {
	props := map[string]any{
		"readable":              map[string]any{"type": "boolean"},
		"odometer_value":        numProp("Gesamtkilometerstand"),
		"odometer_unit":         strProp("Einheit des Gesamtstands", "km", "mi"),
		"trip_meter_value":      numProp("Tageskilometerzähler"),
		"fuel_level_percent":    numProp("Tank- bzw. Akkustand in Prozent"),
		"range_value":           numProp("angezeigte Reichweite"),
		"range_unit":            strProp("Einheit der Reichweite", "km", "mi"),
		"outside_temperature_c": numProp("Außentemperatur in °C"),
		"dashboard_time":        strProp("angezeigte Uhrzeit HH:MM"),
		"warning_lights":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"confidence":            map[string]any{"type": "string", "enum": []string{"high", "medium", "low"}},
		"notes":                 map[string]any{"type": "string"},
	}
	tp := anthropic.ToolParam{
		Name:        dashboardTool,
		Description: anthropic.String("Meldet die aus dem Tachofoto gelesenen Werte."),
		InputSchema: anthropic.ToolInputSchemaParam{Properties: props, Required: []string{"readable", "odometer_value", "confidence", "notes"},
			ExtraFields: map[string]any{"additionalProperties": false}},
	}
	return anthropic.ToolUnionParam{OfTool: &tp}
}

// ReadDashboard liest Kilometerstand und weitere Anzeigen aus einem Tachofoto
// (JPEG/PNG/WebP). Es gelten dieselbe Zustimmung und dasselbe Tageslimit wie im Chat.
func (s *Service) ReadDashboard(ctx context.Context, actor kernel.Actor, img []byte, mediaType string, hint DashboardHint) (DashboardReading, error) {
	if !s.Enabled() {
		return DashboardReading{}, &problem.Error{Status: 503, Type: "https://vectra.app/problems/assistant-disabled", Title: "Bilderkennung nicht eingerichtet",
			Detail: "Die Tacho-Erkennung braucht den KI-Assistenten (VECTRA_ASSISTANT_PROVIDER, ANTHROPIC_API_KEY). Den Stand bitte von Hand eintragen."}
	}
	st, err := s.Status(ctx, actor)
	if err != nil {
		return DashboardReading{}, err
	}
	if st.ConsentRequired {
		return DashboardReading{}, problem.Conflict("Bitte zuerst im Assistenten der Übertragung an " + providerName + " zustimmen.")
	}
	q := store.New(s.pool)
	n, err := q.CountRequestsSince(ctx, store.CountRequestsSinceParams{AccountID: pg(actor.AccountID), CreatedAt: pgtype.Timestamptz{Time: s.Now().Add(-24 * time.Hour), Valid: true}})
	if err != nil {
		return DashboardReading{}, err
	}
	if int(n) >= s.cfg.DailyLimit {
		return DashboardReading{}, problem.TooManyRequests("Tageslimit des Assistenten erreicht.")
	}
	ctxText := "Fahrzeug: " + hint.VehicleName + "."
	if hint.LastOdometer != nil {
		ctxText += fmt.Sprintf(" Letzter bekannter Stand: %.0f %s (nur zur Plausibilisierung, nicht übernehmen).", *hint.LastOdometer, hint.Unit)
	}
	params := anthropic.MessageNewParams{
		Model:      anthropic.Model(s.cfg.Model),
		MaxTokens:  1024,
		System:     []anthropic.TextBlockParam{{Text: dashboardPrompt}},
		Tools:      []anthropic.ToolUnionParam{dashboardToolParam()},
		ToolChoice: anthropic.ToolChoiceUnionParam{OfTool: &anthropic.ToolChoiceToolParam{Name: dashboardTool}},
		Messages: []anthropic.MessageParam{anthropic.NewUserMessage(
			anthropic.NewImageBlockBase64(mediaType, base64.StdEncoding.EncodeToString(img)),
			anthropic.NewTextBlock(ctxText),
		)},
	}
	resp, err := s.client.Messages.New(ctx, params)
	if err != nil {
		return DashboardReading{}, s.apiError(err)
	}
	_ = q.InsertRequestLog(ctx, store.InsertRequestLogParams{ID: pg(kernel.NewID()), AccountID: pg(actor.AccountID), Provider: "anthropic", Model: s.cfg.Model,
		InputTokens: int32(resp.Usage.InputTokens + resp.Usage.CacheReadInputTokens + resp.Usage.CacheCreationInputTokens), OutputTokens: int32(resp.Usage.OutputTokens),
		Tools: []string{dashboardTool}})
	for _, b := range resp.Content {
		if tu, ok := b.AsAny().(anthropic.ToolUseBlock); ok && tu.Name == dashboardTool {
			var out DashboardReading
			if err := json.Unmarshal([]byte(tu.JSON.Input.Raw()), &out); err != nil {
				break
			}
			return normalizeReading(out), nil
		}
	}
	return DashboardReading{}, &problem.Error{Status: 502, Type: "https://vectra.app/problems/assistant-provider", Title: "KI-Anbieter",
		Detail: "Das Foto konnte nicht ausgewertet werden. Bitte den Stand von Hand eintragen."}
}

// normalizeReading verwirft unsinnige Werte, damit die App nur Plausibles vorbelegt.
func normalizeReading(r DashboardReading) DashboardReading {
	unit := func(u *string) *string {
		if u == nil {
			return nil
		}
		v := strings.ToLower(strings.TrimSpace(*u))
		if v != "km" && v != "mi" {
			return nil
		}
		return &v
	}
	r.OdometerUnit, r.RangeUnit = unit(r.OdometerUnit), unit(r.RangeUnit)
	if r.OdometerValue != nil && (*r.OdometerValue < 0 || *r.OdometerValue > 9_999_999) {
		r.OdometerValue = nil
	}
	if r.FuelLevelPercent != nil && (*r.FuelLevelPercent < 0 || *r.FuelLevelPercent > 100) {
		r.FuelLevelPercent = nil
	}
	if r.DashboardTime != nil {
		if _, err := time.Parse("15:04", *r.DashboardTime); err != nil {
			r.DashboardTime = nil
		}
	}
	switch r.Confidence {
	case "high", "medium", "low":
	default:
		r.Confidence = "low"
	}
	if r.WarningLights == nil {
		r.WarningLights = []string{}
	}
	if r.OdometerValue == nil {
		r.Readable = r.Readable && (r.FuelLevelPercent != nil || r.TripMeterValue != nil)
	}
	return r
}
