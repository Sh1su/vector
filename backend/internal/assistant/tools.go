// Package assistant stellt Werkzeuge über Fahrzeugdaten bereit – für den Chat mit Claude
// (ADR-024, ADR-026) und für externe KI-Clients über den MCP-Server (ADR-032).
//
// Werkzeuge haben keine eigene Fachlogik: Sie rufen die öffentliche API intern mit den Rechten
// des Nutzers auf (Caller). Rechte, Validierung, Plausibilität und Audit gelten deshalb genau
// wie bei einem direkten API-Aufruf.
package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sh1su/vector/backend/internal/kernel"
)

// Caller führt eine Anfrage gegen die eigene API aus (Akteur aus dem Kontext). Liefert Status und Body.
type Caller func(ctx context.Context, method, path string, body any, headers map[string]string) (int, []byte)

// APIError ist eine Fehlerantwort der API (Problem Details, ADR-013).
type APIError struct {
	Status  int
	Problem map[string]any
}

func (e *APIError) Error() string {
	if d, _ := e.Problem["detail"].(string); d != "" {
		return d
	}
	if errs, ok := e.Problem["errors"].([]any); ok && len(errs) > 0 {
		parts := []string{}
		for _, x := range errs {
			if m, ok := x.(map[string]any); ok {
				msg, _ := m["message"].(string)
				if msg == "" {
					msg = fmt.Sprintf("%v (%v)", m["pointer"], m["code"])
				}
				parts = append(parts, msg)
			}
		}
		return strings.Join(parts, "; ")
	}
	if t, _ := e.Problem["title"].(string); t != "" {
		return t
	}
	return fmt.Sprintf("Fehler %d", e.Status)
}

// Anomalies liefert die Plausibilitätsbefunde einer 422 (ADR-010).
func (e *APIError) Anomalies() []any {
	a, _ := e.Problem["anomalies"].([]any)
	return a
}

func call(ctx context.Context, c Caller, method, path string, body any, headers map[string]string) (any, error) {
	status, raw := c(ctx, method, path, body, headers)
	var out any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	if status >= 400 {
		p, _ := out.(map[string]any)
		return nil, &APIError{Status: status, Problem: p}
	}
	return out, nil
}

func getMap(ctx context.Context, c Caller, path string) (map[string]any, error) {
	v, err := call(ctx, c, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	m, _ := v.(map[string]any)
	return m, nil
}

func items(m map[string]any) []map[string]any {
	out := []map[string]any{}
	list, _ := m["items"].([]any)
	for _, x := range list {
		if mm, ok := x.(map[string]any); ok {
			out = append(out, mm)
		}
	}
	return out
}

// pick übernimmt nur die genannten Felder (Datensparsamkeit, ADR-024).
func pick(m map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			out[k] = v
		}
	}
	return out
}

// Plan ist eine vorbereitete Schreibaktion: im Chat ein Vorschlag (ADR-026), über MCP sofort ausgeführt.
type Plan struct {
	Operation string
	VehicleID uuid.UUID
	Summary   string
	Body      map[string]any
}

// Tool ist ein Werkzeug für das Modell. Genau eines von Read oder Prepare ist gesetzt.
type Tool struct {
	Name        string
	Description string
	Schema      map[string]any
	Read        func(ctx context.Context, c Caller, in map[string]any) (any, error)
	Prepare     func(ctx context.Context, c Caller, in map[string]any) (Plan, error)
}

func (t Tool) Writes() bool { return t.Prepare != nil }

// ---------- Eingaben ----------

type inputError struct{ msg string }

func (e inputError) Error() string { return e.msg }

func str(in map[string]any, k string) string {
	s, _ := in[k].(string)
	return strings.TrimSpace(s)
}

func num(in map[string]any, k string) (float64, bool) {
	switch v := in[k].(type) {
	case float64:
		return v, true
	case string:
		f, err := parseNumber(v)
		return f, err == nil
	}
	return 0, false
}

func parseNumber(s string) (float64, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), " ", "")
	if strings.Contains(s, ",") {
		s = strings.ReplaceAll(strings.ReplaceAll(s, ".", ""), ",", ".")
	}
	var f float64
	_, err := fmt.Sscan(s, &f)
	return f, err
}

func vehicleID(in map[string]any) (uuid.UUID, error) {
	id, err := uuid.Parse(str(in, "vehicle_id"))
	if err != nil {
		return uuid.Nil, inputError{"vehicle_id fehlt oder ist ungültig – erst list_vehicles aufrufen."}
	}
	return id, nil
}

func needKm(in map[string]any, k string) (float64, error) {
	v, ok := num(in, k)
	if !ok || v < 0 || math.IsInf(v, 0) || math.IsNaN(v) {
		return 0, inputError{k + " fehlt oder ist keine Zahl."}
	}
	return v, nil
}

// vehicle lädt das Fahrzeug (Zeitzone, Einheit, Währung) und prüft dabei die Rechte.
func vehicle(ctx context.Context, c Caller, id uuid.UUID) (map[string]any, error) {
	return getMap(ctx, c, "/vehicles/"+id.String())
}

func zoneOf(v map[string]any) string {
	if z, _ := v["owner_time_zone"].(string); z != "" {
		return z
	}
	return "Europe/Berlin"
}

func unitOf(v map[string]any) string {
	if v["usage_meter"] == "engine_hours" {
		return "h"
	}
	return "km"
}

func currencyOf(v map[string]any) string {
	if c, _ := v["default_currency"].(string); c != "" {
		return c
	}
	return "EUR"
}

// instant liest einen Zeitpunkt (RFC 3339 oder Datum) oder nimmt jetzt; Zukunft ist nicht erlaubt.
func instant(in map[string]any, k string, zone string) (string, error) {
	s := str(in, k)
	if s == "" {
		return time.Now().UTC().Truncate(time.Second).Format(time.RFC3339), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}
	if d, err := time.Parse(time.DateOnly, s); err == nil {
		loc, _ := time.LoadLocation(zone)
		if loc == nil {
			loc = time.UTC
		}
		return time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, loc).UTC().Format(time.RFC3339), nil
	}
	return "", inputError{k + " muss ein Datum (JJJJ-MM-TT) oder Zeitpunkt (RFC 3339) sein."}
}

func day(in map[string]any, k string, zone string) (string, error) {
	s := str(in, k)
	if s == "" {
		loc, _ := time.LoadLocation(zone)
		if loc == nil {
			loc = time.UTC
		}
		return time.Now().In(loc).Format(time.DateOnly), nil
	}
	if _, err := time.Parse(time.DateOnly, s); err != nil {
		return "", inputError{k + " muss ein Datum im Format JJJJ-MM-TT sein."}
	}
	return s, nil
}

func fmtNum(v float64) string {
	s := fmt.Sprintf("%.0f", v)
	if v != math.Trunc(v) {
		s = strings.Replace(fmt.Sprintf("%.1f", v), ".", ",", 1)
	}
	// Tausenderpunkte
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	intPart, frac, _ := strings.Cut(s, ",")
	var b strings.Builder
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	out := b.String()
	if frac != "" {
		out += "," + frac
	}
	if neg {
		out = "-" + out
	}
	return out
}

func minorUnits(amount float64, currency string) int64 {
	digits := 2
	switch currency {
	case "JPY", "KRW", "ISK", "CLP", "VND", "PYG", "UGX", "XAF", "XOF", "XPF":
		digits = 0
	case "BHD", "KWD", "OMR", "JOD", "TND", "LYD", "IQD":
		digits = 3
	}
	return int64(math.Round(amount * math.Pow10(digits)))
}

// ---------- Schemas ----------

func obj(required []string, props map[string]any) map[string]any {
	if required == nil {
		required = []string{}
	}
	return map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
}

var (
	pVehicle = map[string]any{"type": "string", "description": "ID des Fahrzeugs (aus list_vehicles)"}
	pWhen    = map[string]any{"type": "string", "description": "Zeitpunkt RFC 3339 oder Datum JJJJ-MM-TT; weglassen = jetzt"}
	pDate    = map[string]any{"type": "string", "description": "Datum JJJJ-MM-TT; weglassen = heute"}
	pKm      = func(d string) map[string]any { return map[string]any{"type": "number", "description": d} }
	pText    = func(d string) map[string]any { return map[string]any{"type": "string", "description": d} }
	pEnum    = func(d string, vals ...string) map[string]any {
		return map[string]any{"type": "string", "enum": vals, "description": d}
	}
)

// Tools ist der Werkzeugkasten. Lesende Werkzeuge laufen sofort, schreibende liefern einen Plan.
func Tools() []Tool {
	return []Tool{
		{
			Name:        "list_vehicles",
			Description: "Listet die Fahrzeuge des Nutzers mit ID, Name, Kennzeichen, Marke/Modell, Rolle und aktuellem Kilometerstand. Zuerst aufrufen, wenn die vehicle_id nicht bekannt ist.",
			Schema:      obj(nil, map[string]any{}),
			Read:        listVehicles,
		},
		{
			Name:        "get_vehicle_overview",
			Description: "Überblick über ein Fahrzeug: aktueller Kilometerstand, laufende Fahrt, fällige Wartungen und Fahrtkategorien.",
			Schema:      obj([]string{"vehicle_id"}, map[string]any{"vehicle_id": pVehicle}),
			Read:        overview,
		},
		{
			Name:        "list_trips",
			Description: "Letzte Fahrten (Fahrtenbuch) eines Fahrzeugs mit Start/Ende, Kilometern, Kategorie und Zweck.",
			Schema:      obj([]string{"vehicle_id"}, map[string]any{"vehicle_id": pVehicle, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 50}}),
			Read:        listTrips,
		},
		{
			Name:        "list_maintenance",
			Description: "Geplante Wartungen eines Fahrzeugs mit Intervall und Fälligkeit (Level, Datum, Rest-km).",
			Schema:      obj([]string{"vehicle_id"}, map[string]any{"vehicle_id": pVehicle}),
			Read:        listMaintenance,
		},
		{
			Name:        "list_service_entries",
			Description: "Servicehistorie (Werkstattbesuche) eines Fahrzeugs mit Datum, Kilometerstand und Kosten.",
			Schema:      obj([]string{"vehicle_id"}, map[string]any{"vehicle_id": pVehicle, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 50}}),
			Read:        listService,
		},
		{
			Name:        "get_cost_summary",
			Description: "Kosten eines Fahrzeugs für ein Jahr: Summe, je Kategorie, je km und je Tag.",
			Schema:      obj([]string{"vehicle_id"}, map[string]any{"vehicle_id": pVehicle, "year": map[string]any{"type": "integer", "description": "Jahr; weglassen = laufendes Jahr"}}),
			Read:        costSummary,
		},
		{
			Name:        "list_maintenance_books",
			Description: "Hinterlegte Wartungsbücher (Herstellerintervalle als Vorlage), optional nach Marke gefiltert. Gibt es keins für das Modell, kann ein Wartungsplan mit create_maintenance_plan angelegt werden.",
			Schema:      obj(nil, map[string]any{"make": pText("Marke, z. B. Hyundai")}),
			Read:        listBooks,
		},
		{
			Name:        "record_odometer",
			Description: "Kilometerstand (bzw. Betriebsstunden) erfassen. Ein Wert pro Aufruf.",
			Schema: obj([]string{"vehicle_id", "value"}, map[string]any{"vehicle_id": pVehicle, "value": pKm("Stand in km (bzw. h)"),
				"occurred_at": pWhen, "note": pText("Notiz (optional)")}),
			Prepare: prepareOdometer,
		},
		{
			Name: "start_trip",
			Description: "Fahrt im Fahrtenbuch beginnen: Startkilometerstand, Kategorie und Zweck. Für Geschäftsfahrten ist der Zweck Pflicht. " +
				"Die Kategorie ist private, business, commute oder other bzw. der Name einer eigenen Kategorie.",
			Schema: obj([]string{"vehicle_id", "start_km", "category"}, map[string]any{"vehicle_id": pVehicle, "start_km": pKm("Kilometerstand bei Abfahrt"),
				"category": pText("private | business | commute | other oder Kategoriename"), "purpose": pText("Zweck/Anlass, z. B. Kundentermin Müller"),
				"start_location": pText("Startort (optional)"), "started_at": pWhen}),
			Prepare: prepareStartTrip,
		},
		{
			Name:        "finish_trip",
			Description: "Laufende Fahrt beenden: Endkilometerstand und Ziel.",
			Schema: obj([]string{"vehicle_id", "end_km"}, map[string]any{"vehicle_id": pVehicle, "end_km": pKm("Kilometerstand bei Ankunft"),
				"end_location": pText("Ziel (optional)"), "ended_at": pWhen}),
			Prepare: prepareFinishTrip,
		},
		{
			Name:        "record_trip",
			Description: "Abgeschlossene Fahrt nachtragen (Start und Ende in einem Schritt).",
			Schema: obj([]string{"vehicle_id", "start_km", "end_km", "category"}, map[string]any{"vehicle_id": pVehicle,
				"start_km": pKm("Kilometerstand bei Abfahrt"), "end_km": pKm("Kilometerstand bei Ankunft"),
				"category": pText("private | business | commute | other oder Kategoriename"), "purpose": pText("Zweck/Anlass"),
				"start_location": pText("Startort"), "end_location": pText("Ziel"), "started_at": pWhen, "ended_at": pWhen}),
			Prepare: prepareRecordTrip,
		},
		{
			Name:        "add_cost",
			Description: "Sonstige Kosten erfassen (Parken, Maut, Versicherung, Steuer, Pflege …). Werkstattkosten stattdessen mit add_service_entry.",
			Schema: obj([]string{"vehicle_id", "category", "title", "amount"}, map[string]any{"vehicle_id": pVehicle,
				"category": pEnum("Kategorie", "tax", "insurance", "fee", "parking", "toll", "care", "financing", "other"),
				"title":    pText("Bezeichnung"), "amount": pKm("Betrag in der Währung, z. B. 12.50"),
				"currency": pText("ISO-Währung; weglassen = Fahrzeugwährung"), "date": pDate}),
			Prepare: prepareCost,
		},
		{
			Name:        "add_service_entry",
			Description: "Werkstattbesuch/Service in der Servicehistorie erfassen, optional mit Kosten und erledigten Wartungen.",
			Schema: obj([]string{"vehicle_id", "title", "kind"}, map[string]any{"vehicle_id": pVehicle, "title": pText("z. B. Inspektion 60.000 km"),
				"kind": pEnum("Art", "maintenance", "inspection", "repair", "upgrade"), "date": pDate, "km": pKm("Kilometerstand (optional)"),
				"provider": pText("Werkstatt (optional)"), "parts": pKm("Teilekosten"), "labor": pKm("Arbeitskosten"), "other": pKm("Sonstige Kosten"),
				"currency":                       pText("ISO-Währung; weglassen = Fahrzeugwährung"),
				"completes_maintenance_item_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "IDs erledigter Wartungen (aus list_maintenance)"}}),
			Prepare: prepareService,
		},
		{
			Name:        "complete_maintenance",
			Description: "Eine geplante Wartung als erledigt markieren (ohne Serviceeintrag).",
			Schema: obj([]string{"vehicle_id", "item_id"}, map[string]any{"vehicle_id": pVehicle, "item_id": pText("ID der Wartung (aus list_maintenance)"),
				"date": pDate, "km": pKm("Kilometerstand (optional)")}),
			Prepare: prepareComplete,
		},
		{
			Name:        "apply_maintenance_book",
			Description: "Hinterlegtes Wartungsbuch (aus list_maintenance_books) für ein Fahrzeug übernehmen. since_new=true rechnet ab Erstzulassung (anchor_date = Erstzulassung, anchor_km = 0).",
			Schema: obj([]string{"vehicle_id", "book_id"}, map[string]any{"vehicle_id": pVehicle, "book_id": pText("ID des Wartungsbuchs"),
				"since_new": map[string]any{"type": "boolean"}, "anchor_date": pText("Erstzulassung bzw. letzte Inspektion JJJJ-MM-TT"), "anchor_km": pKm("Stand damals")}),
			Prepare: prepareBook,
		},
		{
			Name: "create_maintenance_plan",
			Description: "Wartungsplan für ein Fahrzeug anlegen, wenn kein Wartungsbuch hinterlegt ist – z. B. aus Herstellerangaben, die per Websuche recherchiert wurden. " +
				"Jede Position braucht Titel, Kategorie und mindestens ein Intervall. source nennt die Quelle (URL oder „Serviceheft“).",
			Schema: obj([]string{"vehicle_id", "source", "items"}, map[string]any{"vehicle_id": pVehicle, "source": pText("Quelle der Intervalle"),
				"anchor_date": pText("letzte Durchführung bzw. Erstzulassung JJJJ-MM-TT (optional)"), "anchor_km": pKm("Stand damals (optional)"),
				"items": map[string]any{"type": "array", "minItems": 1, "maxItems": 30, "items": obj([]string{"title", "category"}, map[string]any{
					"title":           pText("z. B. Ölwechsel"),
					"category":        pEnum("Kategorie", "service", "legal_inspection", "tires", "fluids", "brakes", "filters", "other"),
					"interval_months": map[string]any{"type": "integer", "minimum": 1},
					"interval_km":     pKm("Intervall in km"),
					"note":            pText("Hinweis, z. B. Motorvariante"),
				})}}),
			Prepare: preparePlan,
		},
	}
}

// ---------- Lesen ----------

func listVehicles(ctx context.Context, c Caller, _ map[string]any) (any, error) {
	m, err := getMap(ctx, c, "/vehicles?limit=200")
	if err != nil {
		return nil, err
	}
	actor, _ := kernel.ActorFrom(ctx)
	out := []map[string]any{}
	for _, v := range items(m) {
		id, _ := uuid.Parse(fmt.Sprint(v["id"]))
		if !actor.MayAccess(id) {
			continue
		}
		row := pick(v, "id", "display_name", "license_plate", "make", "model", "model_year", "first_registration", "status", "my_role", "default_currency")
		row["unit"] = unitOf(v)
		if cur, err := getMap(ctx, c, "/vehicles/"+id.String()+"/odometer/current"); err == nil {
			if mv, ok := cur["meter_value"].(map[string]any); ok {
				if cn, ok := mv["canonical"].(float64); ok {
					if unitOf(v) == "h" {
						row["current"] = math.Round(cn/360) / 10
					} else {
						row["current"] = math.Floor(cn / 1000)
					}
				}
			}
		}
		out = append(out, row)
	}
	return map[string]any{"vehicles": out}, nil
}

func overview(ctx context.Context, c Caller, in map[string]any) (any, error) {
	id, err := vehicleID(in)
	if err != nil {
		return nil, err
	}
	v, err := vehicle(ctx, c, id)
	if err != nil {
		return nil, err
	}
	base := "/vehicles/" + id.String()
	out := map[string]any{"vehicle": pick(v, "id", "display_name", "license_plate", "make", "model", "first_registration", "owner_time_zone", "default_currency")}
	if cur, err := getMap(ctx, c, base+"/odometer/current"); err == nil {
		out["odometer"] = pick(cur, "meter_value", "total", "as_of", "kind")
	}
	if t, err := getMap(ctx, c, base+"/trips?limit=5"); err == nil {
		for _, tr := range items(t) {
			if tr["status"] == "open" {
				out["open_trip"] = pick(tr, "id", "started_at", "start_odometer", "purpose", "category_id", "start_location")
			}
		}
	}
	if s, err := getMap(ctx, c, base+"/maintenance/status"); err == nil {
		due := []map[string]any{}
		for _, d := range items(s) {
			due = append(due, pick(d, "item_id", "title", "level", "due_date", "days_remaining", "distance_remaining", "estimated_due_date"))
		}
		out["maintenance"] = due
	}
	if cats, err := getMap(ctx, c, base+"/trip-categories"); err == nil {
		list := []map[string]any{}
		for _, cat := range items(cats) {
			list = append(list, pick(cat, "id", "name", "kind", "purpose_required"))
		}
		out["trip_categories"] = list
	}
	return out, nil
}

func limitOf(in map[string]any, def int) int {
	if v, ok := num(in, "limit"); ok && v >= 1 && v <= 50 {
		return int(v)
	}
	return def
}

func listTrips(ctx context.Context, c Caller, in map[string]any) (any, error) {
	id, err := vehicleID(in)
	if err != nil {
		return nil, err
	}
	m, err := getMap(ctx, c, fmt.Sprintf("/vehicles/%s/trips?limit=%d", id, limitOf(in, 10)))
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, t := range items(m) {
		out = append(out, pick(t, "id", "status", "started_at", "ended_at", "start_odometer", "end_odometer", "distance", "category_id", "purpose", "start_location", "end_location"))
	}
	return map[string]any{"trips": out}, nil
}

func listMaintenance(ctx context.Context, c Caller, in map[string]any) (any, error) {
	id, err := vehicleID(in)
	if err != nil {
		return nil, err
	}
	m, err := getMap(ctx, c, "/vehicles/"+id.String()+"/maintenance-items")
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, it := range items(m) {
		row := pick(it, "id", "title", "category", "schedule_mode", "interval_months", "interval_distance", "active")
		if st, ok := it["status"].(map[string]any); ok {
			row["status"] = pick(st, "level", "due_date", "days_remaining", "distance_remaining", "estimated_due_date")
		}
		out = append(out, row)
	}
	return map[string]any{"maintenance": out}, nil
}

func listService(ctx context.Context, c Caller, in map[string]any) (any, error) {
	id, err := vehicleID(in)
	if err != nil {
		return nil, err
	}
	m, err := getMap(ctx, c, fmt.Sprintf("/vehicles/%s/service-entries?limit=%d", id, limitOf(in, 10)))
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, e := range items(m) {
		out = append(out, pick(e, "id", "occurred_at", "kind", "title", "odometer", "provider_name", "totals"))
	}
	return map[string]any{"service_entries": out}, nil
}

func costSummary(ctx context.Context, c Caller, in map[string]any) (any, error) {
	id, err := vehicleID(in)
	if err != nil {
		return nil, err
	}
	y := time.Now().Year()
	if v, ok := num(in, "year"); ok && v > 1900 && v < 2200 {
		y = int(v)
	}
	m, err := getMap(ctx, c, fmt.Sprintf("/vehicles/%s/cost-report?from=%d-01-01&to=%d-12-31&group_by=category", id, y, y))
	if err != nil {
		return nil, err
	}
	return pick(m, "from", "to", "distance", "currencies", "hints"), nil
}

func listBooks(ctx context.Context, c Caller, in map[string]any) (any, error) {
	path := "/maintenance-books"
	if mk := str(in, "make"); mk != "" {
		path += "?make=" + url.QueryEscape(mk)
	}
	m, err := getMap(ctx, c, path)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, b := range items(m) {
		row := pick(b, "id", "make", "model", "variant", "title", "source")
		titles := []string{}
		for _, it := range b["items"].([]any) {
			if mm, ok := it.(map[string]any); ok {
				titles = append(titles, fmt.Sprint(mm["title"]))
			}
		}
		row["items"] = titles
		out = append(out, row)
	}
	return map[string]any{"books": out}, nil
}

// ---------- Schreiben (Pläne) ----------

func prepareOdometer(ctx context.Context, c Caller, in map[string]any) (Plan, error) {
	id, err := vehicleID(in)
	if err != nil {
		return Plan{}, err
	}
	v, err := vehicle(ctx, c, id)
	if err != nil {
		return Plan{}, err
	}
	val, err := needKm(in, "value")
	if err != nil {
		return Plan{}, err
	}
	at, err := instant(in, "occurred_at", zoneOf(v))
	if err != nil {
		return Plan{}, err
	}
	body := map[string]any{"occurred_at": at, "time_zone": zoneOf(v), "time_precision": "exact", "value": map[string]any{"value": val, "unit": unitOf(v)}}
	if n := str(in, "note"); n != "" {
		body["note"] = n
	}
	return Plan{Operation: "createOdometerReading", VehicleID: id, Body: body,
		Summary: fmt.Sprintf("Kilometerstand %s %s für %v", fmtNum(val), unitOf(v), v["display_name"])}, nil
}

// category sucht die Fahrtkategorie nach Art oder Name.
func category(ctx context.Context, c Caller, id uuid.UUID, want string) (map[string]any, error) {
	m, err := getMap(ctx, c, "/vehicles/"+id.String()+"/trip-categories")
	if err != nil {
		return nil, err
	}
	w := strings.ToLower(strings.TrimSpace(want))
	alias := map[string]string{"privat": "private", "geschäftlich": "business", "dienstlich": "business", "arbeitsweg": "commute", "sonstiges": "other"}
	if a, ok := alias[w]; ok {
		w = a
	}
	var byKind map[string]any
	for _, cat := range items(m) {
		if cat["active"] == false {
			continue
		}
		if strings.EqualFold(fmt.Sprint(cat["name"]), want) || fmt.Sprint(cat["id"]) == want {
			return cat, nil
		}
		if byKind == nil && cat["kind"] == w {
			byKind = cat
		}
	}
	if byKind != nil {
		return byKind, nil
	}
	return nil, inputError{"Fahrtkategorie „" + want + "“ gibt es nicht. Verfügbar sind private, business, commute, other oder eigene Kategorien (get_vehicle_overview)."}
}

func tripCommon(ctx context.Context, c Caller, in map[string]any) (uuid.UUID, map[string]any, map[string]any, error) {
	id, err := vehicleID(in)
	if err != nil {
		return id, nil, nil, err
	}
	v, err := vehicle(ctx, c, id)
	if err != nil {
		return id, nil, nil, err
	}
	cat, err := category(ctx, c, id, str(in, "category"))
	if err != nil {
		return id, nil, nil, err
	}
	if cat["purpose_required"] == true && str(in, "purpose") == "" {
		return id, nil, nil, inputError{"Für die Kategorie „" + fmt.Sprint(cat["name"]) + "“ ist ein Zweck Pflicht – bitte beim Nutzer nachfragen."}
	}
	return id, v, cat, nil
}

func optional(body map[string]any, in map[string]any, keys ...string) {
	for _, k := range keys {
		if s := str(in, k); s != "" {
			body[k] = s
		}
	}
}

func prepareStartTrip(ctx context.Context, c Caller, in map[string]any) (Plan, error) {
	id, v, cat, err := tripCommon(ctx, c, in)
	if err != nil {
		return Plan{}, err
	}
	km, err := needKm(in, "start_km")
	if err != nil {
		return Plan{}, err
	}
	at, err := instant(in, "started_at", zoneOf(v))
	if err != nil {
		return Plan{}, err
	}
	body := map[string]any{"started_at": at, "time_zone": zoneOf(v), "start_odometer": map[string]any{"value": km, "unit": "km"}, "category_id": cat["id"]}
	optional(body, in, "purpose", "start_location")
	sum := fmt.Sprintf("Fahrt starten bei %s km · %v", fmtNum(km), cat["name"])
	if p := str(in, "purpose"); p != "" {
		sum += " · " + p
	}
	return Plan{Operation: "startTrip", VehicleID: id, Body: body, Summary: sum}, nil
}

func prepareFinishTrip(ctx context.Context, c Caller, in map[string]any) (Plan, error) {
	id, err := vehicleID(in)
	if err != nil {
		return Plan{}, err
	}
	v, err := vehicle(ctx, c, id)
	if err != nil {
		return Plan{}, err
	}
	m, err := getMap(ctx, c, "/vehicles/"+id.String()+"/trips?limit=20")
	if err != nil {
		return Plan{}, err
	}
	var open map[string]any
	for _, t := range items(m) {
		if t["status"] == "open" {
			open = t
			break
		}
	}
	if open == nil {
		return Plan{}, inputError{"Es läuft keine Fahrt. Mit record_trip lässt sich eine Fahrt vollständig nachtragen."}
	}
	km, err := needKm(in, "end_km")
	if err != nil {
		return Plan{}, err
	}
	at, err := instant(in, "ended_at", zoneOf(v))
	if err != nil {
		return Plan{}, err
	}
	body := map[string]any{"trip_id": open["id"], "ended_at": at, "end_odometer": map[string]any{"value": km, "unit": "km"}}
	optional(body, in, "end_location")
	sum := fmt.Sprintf("Fahrt beenden bei %s km", fmtNum(km))
	if so, ok := open["start_odometer"].(map[string]any); ok {
		if s, ok := so["value"].(float64); ok && km >= s {
			sum += fmt.Sprintf(" (%s km)", fmtNum(km-s))
		}
	}
	return Plan{Operation: "finishTrip", VehicleID: id, Body: body, Summary: sum}, nil
}

func prepareRecordTrip(ctx context.Context, c Caller, in map[string]any) (Plan, error) {
	id, v, cat, err := tripCommon(ctx, c, in)
	if err != nil {
		return Plan{}, err
	}
	start, err := needKm(in, "start_km")
	if err != nil {
		return Plan{}, err
	}
	end, err := needKm(in, "end_km")
	if err != nil {
		return Plan{}, err
	}
	if end < start {
		return Plan{}, inputError{"end_km liegt unter start_km."}
	}
	ended, err := instant(in, "ended_at", zoneOf(v))
	if err != nil {
		return Plan{}, err
	}
	started := ended
	if str(in, "started_at") != "" {
		if started, err = instant(in, "started_at", zoneOf(v)); err != nil {
			return Plan{}, err
		}
	}
	body := map[string]any{"started_at": started, "ended_at": ended, "time_zone": zoneOf(v),
		"start_odometer": map[string]any{"value": start, "unit": "km"}, "end_odometer": map[string]any{"value": end, "unit": "km"}, "category_id": cat["id"]}
	optional(body, in, "purpose", "start_location", "end_location")
	sum := fmt.Sprintf("Fahrt %s → %s km (%s km) · %v", fmtNum(start), fmtNum(end), fmtNum(end-start), cat["name"])
	if p := str(in, "purpose"); p != "" {
		sum += " · " + p
	}
	return Plan{Operation: "createTrip", VehicleID: id, Body: body, Summary: sum}, nil
}

func prepareCost(ctx context.Context, c Caller, in map[string]any) (Plan, error) {
	id, err := vehicleID(in)
	if err != nil {
		return Plan{}, err
	}
	v, err := vehicle(ctx, c, id)
	if err != nil {
		return Plan{}, err
	}
	amount, ok := num(in, "amount")
	if !ok {
		return Plan{}, inputError{"amount fehlt."}
	}
	cur := strings.ToUpper(str(in, "currency"))
	if cur == "" {
		cur = currencyOf(v)
	}
	d, err := day(in, "date", zoneOf(v))
	if err != nil {
		return Plan{}, err
	}
	body := map[string]any{"category": str(in, "category"), "title": str(in, "title"), "incurred_on": d, "time_zone": zoneOf(v),
		"amount": map[string]any{"amount_minor": minorUnits(amount, cur), "currency": cur}}
	return Plan{Operation: "createCostEntry", VehicleID: id, Body: body,
		Summary: fmt.Sprintf("Kosten %s: %s %s am %s", str(in, "title"), strings.Replace(fmt.Sprintf("%.2f", amount), ".", ",", 1), cur, d)}, nil
}

func prepareService(ctx context.Context, c Caller, in map[string]any) (Plan, error) {
	id, err := vehicleID(in)
	if err != nil {
		return Plan{}, err
	}
	v, err := vehicle(ctx, c, id)
	if err != nil {
		return Plan{}, err
	}
	cur := strings.ToUpper(str(in, "currency"))
	if cur == "" {
		cur = currencyOf(v)
	}
	d, err := day(in, "date", zoneOf(v))
	if err != nil {
		return Plan{}, err
	}
	at, _ := instant(map[string]any{"d": d}, "d", zoneOf(v))
	body := map[string]any{"occurred_at": at, "time_zone": zoneOf(v), "time_precision": "date_only", "kind": str(in, "kind"), "title": str(in, "title"), "currency": cur}
	if km, ok := num(in, "km"); ok {
		body["odometer"] = map[string]any{"value": km, "unit": "km"}
	}
	if p := str(in, "provider"); p != "" {
		body["provider_name"] = p
	}
	costItems := []map[string]any{}
	total := 0.0
	for _, k := range []string{"parts", "labor", "other"} {
		if a, ok := num(in, k); ok && a != 0 {
			costItems = append(costItems, map[string]any{"kind": k, "amount_minor": minorUnits(a, cur)})
			total += a
		}
	}
	if len(costItems) == 0 {
		body["cost_unknown"] = true
	} else {
		body["cost_items"] = costItems
	}
	if ids, ok := in["completes_maintenance_item_ids"].([]any); ok && len(ids) > 0 {
		body["completes_maintenance_item_ids"] = ids
	}
	sum := fmt.Sprintf("Service „%s“ am %s", str(in, "title"), d)
	if total > 0 {
		sum += fmt.Sprintf(" · %s %s", strings.Replace(fmt.Sprintf("%.2f", total), ".", ",", 1), cur)
	}
	return Plan{Operation: "createServiceEntry", VehicleID: id, Body: body, Summary: sum}, nil
}

func prepareComplete(ctx context.Context, c Caller, in map[string]any) (Plan, error) {
	id, err := vehicleID(in)
	if err != nil {
		return Plan{}, err
	}
	v, err := vehicle(ctx, c, id)
	if err != nil {
		return Plan{}, err
	}
	item, err := getMap(ctx, c, "/vehicles/"+id.String()+"/maintenance-items/"+url.PathEscape(str(in, "item_id")))
	if err != nil {
		return Plan{}, err
	}
	d, err := day(in, "date", zoneOf(v))
	if err != nil {
		return Plan{}, err
	}
	body := map[string]any{"item_id": item["id"], "kind": "done", "completed_on": d}
	if km, ok := num(in, "km"); ok {
		body["completed_odometer"] = map[string]any{"value": km, "unit": "km"}
	}
	return Plan{Operation: "completeMaintenance", VehicleID: id, Body: body, Summary: fmt.Sprintf("Wartung „%v“ erledigt am %s", item["title"], d)}, nil
}

func prepareBook(ctx context.Context, c Caller, in map[string]any) (Plan, error) {
	id, err := vehicleID(in)
	if err != nil {
		return Plan{}, err
	}
	if _, err := vehicle(ctx, c, id); err != nil {
		return Plan{}, err
	}
	books, err := getMap(ctx, c, "/maintenance-books")
	if err != nil {
		return Plan{}, err
	}
	var book map[string]any
	for _, b := range items(books) {
		if b["id"] == str(in, "book_id") {
			book = b
		}
	}
	if book == nil {
		return Plan{}, inputError{"Wartungsbuch nicht gefunden – list_maintenance_books aufrufen."}
	}
	body := map[string]any{"book_id": book["id"], "since_new": in["since_new"] == true}
	if d := str(in, "anchor_date"); d != "" {
		if _, err := time.Parse(time.DateOnly, d); err != nil {
			return Plan{}, inputError{"anchor_date muss JJJJ-MM-TT sein."}
		}
		body["anchor_date"] = d
	}
	if km, ok := num(in, "anchor_km"); ok {
		body["anchor_odometer"] = map[string]any{"value": km, "unit": "km"}
	}
	n := len(book["items"].([]any))
	return Plan{Operation: "applyMaintenanceBook", VehicleID: id, Body: body, Summary: fmt.Sprintf("Wartungsbuch „%v“ übernehmen (%d Positionen)", book["title"], n)}, nil
}

func preparePlan(ctx context.Context, c Caller, in map[string]any) (Plan, error) {
	id, err := vehicleID(in)
	if err != nil {
		return Plan{}, err
	}
	if _, err := vehicle(ctx, c, id); err != nil {
		return Plan{}, err
	}
	list, _ := in["items"].([]any)
	if len(list) == 0 || len(list) > 30 {
		return Plan{}, inputError{"items braucht 1 bis 30 Positionen."}
	}
	source := str(in, "source")
	if source == "" {
		return Plan{}, inputError{"source fehlt – Quelle der Intervalle angeben."}
	}
	anchorDate := str(in, "anchor_date")
	if anchorDate != "" {
		if _, err := time.Parse(time.DateOnly, anchorDate); err != nil {
			return Plan{}, inputError{"anchor_date muss JJJJ-MM-TT sein."}
		}
	}
	anchorKm, hasKm := num(in, "anchor_km")
	out := []any{}
	titles := []string{}
	for i, x := range list {
		it, _ := x.(map[string]any)
		title := str(it, "title")
		cat := str(it, "category")
		if title == "" || cat == "" {
			return Plan{}, inputError{fmt.Sprintf("Position %d: title und category sind Pflicht.", i+1)}
		}
		item := map[string]any{"title": title, "category": cat, "schedule_mode": "from_last_completion",
			"note": strings.TrimSpace("Quelle: " + source + ". " + str(it, "note"))}
		has := false
		if m, ok := num(it, "interval_months"); ok && m >= 1 {
			item["interval_months"] = int(m)
			has = true
		}
		if km, ok := num(it, "interval_km"); ok && km > 0 {
			item["interval_distance"] = map[string]any{"value": km, "unit": "km"}
			has = true
		}
		if !has {
			return Plan{}, inputError{fmt.Sprintf("Position %d (%s): mindestens ein Intervall (Monate oder km) angeben.", i+1, title)}
		}
		if anchorDate != "" {
			item["anchor_date"] = anchorDate
		}
		if hasKm {
			item["anchor_odometer"] = map[string]any{"value": anchorKm, "unit": "km"}
		}
		out = append(out, item)
		titles = append(titles, title)
	}
	return Plan{Operation: "createMaintenancePlan", VehicleID: id, Body: map[string]any{"source": source, "items": out},
		Summary: fmt.Sprintf("Wartungsplan anlegen (%d Positionen: %s)", len(out), strings.Join(titles, ", "))}, nil
}

// ---------- Ausführen ----------

// Confirmation bestätigt Plausibilitätsbefunde (ADR-010).
type Confirmation struct {
	Codes  []string
	Reason string
}

func withConfirm(body map[string]any, conf *Confirmation) map[string]any {
	out := map[string]any{}
	for k, v := range body {
		out[k] = v
	}
	if conf != nil && len(conf.Codes) > 0 {
		out["confirm_anomalies"] = conf.Codes
		out["anomaly_reason"] = conf.Reason
	}
	return out
}

func idOf(v any) *uuid.UUID {
	m, _ := v.(map[string]any)
	if id, err := uuid.Parse(fmt.Sprint(m["id"])); err == nil {
		return &id
	}
	return nil
}

// Execute führt einen Plan aus. key macht die Ausführung idempotent (Client-IDs, ADR-006).
// Ergebnis: ID des angelegten Objekts (falls eines) und eine kurze Meldung.
func Execute(ctx context.Context, c Caller, p Plan, key uuid.UUID, conf *Confirmation) (*uuid.UUID, string, error) {
	base := "/vehicles/" + p.VehicleID.String()
	b := withConfirm(p.Body, conf)
	switch p.Operation {
	case "createOdometerReading":
		b["id"] = key.String()
		r, err := call(ctx, c, http.MethodPost, base+"/odometer/readings", b, nil)
		return idOf(r), "Kilometerstand gespeichert.", err
	case "startTrip":
		b["id"] = key.String()
		r, err := call(ctx, c, http.MethodPost, base+"/trips/start", b, nil)
		return idOf(r), "Fahrt gestartet.", err
	case "createTrip":
		b["id"] = key.String()
		r, err := call(ctx, c, http.MethodPost, base+"/trips", b, nil)
		return idOf(r), "Fahrt eingetragen.", err
	case "finishTrip":
		tripID := fmt.Sprint(b["trip_id"])
		delete(b, "trip_id")
		trip, err := getMap(ctx, c, base+"/trips/"+url.PathEscape(tripID))
		if err != nil {
			return nil, "", err
		}
		if trip["status"] != "open" {
			return idOf(trip), "Die Fahrt war bereits beendet.", nil
		}
		r, err := call(ctx, c, http.MethodPost, base+"/trips/"+url.PathEscape(tripID)+"/finish", b, map[string]string{"If-Match": fmt.Sprintf(`"%v"`, trip["version"])})
		return idOf(r), "Fahrt beendet.", err
	case "createCostEntry":
		b["id"] = key.String()
		r, err := call(ctx, c, http.MethodPost, base+"/cost-entries", b, nil)
		return idOf(r), "Kosten gespeichert.", err
	case "createServiceEntry":
		b["id"] = key.String()
		r, err := call(ctx, c, http.MethodPost, base+"/service-entries", b, nil)
		return idOf(r), "Serviceeintrag gespeichert.", err
	case "completeMaintenance":
		itemID := fmt.Sprint(b["item_id"])
		delete(b, "item_id")
		r, err := call(ctx, c, http.MethodPost, base+"/maintenance-items/"+url.PathEscape(itemID)+"/completions", b, map[string]string{"Idempotency-Key": key.String()})
		return idOf(r), "Wartung als erledigt markiert.", err
	case "applyMaintenanceBook":
		bookID := fmt.Sprint(b["book_id"])
		delete(b, "book_id")
		r, err := call(ctx, c, http.MethodPost, base+"/maintenance-books/"+url.PathEscape(bookID)+"/apply", b, nil)
		if err != nil {
			return nil, "", err
		}
		m, _ := r.(map[string]any)
		created, _ := m["created"].([]any)
		skipped, _ := m["skipped"].([]any)
		return nil, fmt.Sprintf("%d Wartungen angelegt, %d bereits vorhanden.", len(created), len(skipped)), nil
	case "createMaintenancePlan":
		existing, err := getMap(ctx, c, base+"/maintenance-items")
		if err != nil {
			return nil, "", err
		}
		have := map[string]bool{}
		for _, it := range items(existing) {
			have[strings.ToLower(fmt.Sprint(it["title"]))] = true
		}
		list, _ := b["items"].([]any)
		created, skipped := 0, 0
		for _, x := range list {
			it, _ := x.(map[string]any)
			title := fmt.Sprint(it["title"])
			if have[strings.ToLower(title)] {
				skipped++
				continue
			}
			body := map[string]any{}
			for k, v := range it {
				body[k] = v
			}
			body["id"] = uuid.NewSHA1(key, []byte(title)).String()
			if _, err := call(ctx, c, http.MethodPost, base+"/maintenance-items", body, nil); err != nil {
				return nil, "", fmt.Errorf("%s: %w", title, err)
			}
			created++
		}
		return nil, fmt.Sprintf("%d Wartungen angelegt, %d bereits vorhanden.", created, skipped), nil
	}
	return nil, "", errors.New("unbekannte Operation " + p.Operation)
}

// IsInput meldet Eingabefehler, die das Modell selbst beheben bzw. erfragen soll.
func IsInput(err error) bool {
	var ie inputError
	return errors.As(err, &ie)
}
