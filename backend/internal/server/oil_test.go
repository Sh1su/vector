package server

import (
	"strings"
	"testing"
)

func oilEntry(at, kind string, km float64) map[string]any {
	return map[string]any{"occurred_at": at, "time_zone": "Europe/Berlin", "kind": kind, "odometer": map[string]any{"value": km, "unit": "km"}}
}

func TestOilEntriesSeriesAndStatistics(t *testing.T) {
	e := newEnv(t)
	c := e.adminClient()
	p := vehiclePayload("Golf")
	p["oil_dipstick_range"] = map[string]any{"value": 1000, "unit": "ml"}
	p["oil_capacity"] = map[string]any{"value": 4.3, "unit": "l"}
	vid := c.do("POST", "/vehicles", p).body["id"].(string)
	expect(t, c.do("PATCH", "/me/settings", map[string]any{"display_units": map[string]any{"oil_volume": "ml"}}), 200, "settings")
	base := "/vehicles/" + vid + "/oil-entries"
	e1 := oilEntry("2026-02-20T00:00:00+01:00", "check", 49800)
	e1["level_before"] = map[string]any{"percent": 60}
	e2 := oilEntry("2026-03-01T00:00:00+01:00", "oil_change", 50000)
	e2["level_after"] = map[string]any{"step": "max"}
	e2["oil_change_fill"] = map[string]any{"value": 4.3, "unit": "l"}
	e2["filter_changed"] = true
	e3 := oilEntry("2026-04-01T12:00:00+02:00", "check", 51500)
	e3["level_before"] = map[string]any{"percent": 70}
	e4 := oilEntry("2026-04-20T12:00:00+02:00", "top_up", 52500)
	e4["level_before"] = map[string]any{"percent": 40}
	e4["oil_added"] = map[string]any{"value": 500, "unit": "ml"}
	e5 := oilEntry("2026-05-16T00:00:00+02:00", "check", 54000)
	e5["level_before"] = map[string]any{"step": "three_quarters"} // L-10: ¾ → 75 %
	var last map[string]any
	for _, b := range []map[string]any{e1, e2, e3, e4, e5} {
		r := c.do("POST", base, b)
		expect(t, r, 201, "oil entry")
		last = r.body
	}
	if last["level_before_pct"].(float64) != 75 {
		t.Fatalf("L-10: %v", last)
	}
	// Paar 4→5: (90 − 75) % = 150 ml / 1 500 km = 100 ml/1 000 km
	pair := last["pair"].(map[string]any)
	if pair["status"] != "computed" || pair["consumption_per_1000"].(map[string]any)["value"].(float64) != 100 {
		t.Fatalf("pair: %v", pair)
	}
	r := c.do("GET", "/vehicles/"+vid+"/oil/series", nil)
	expect(t, r, 200, "series")
	items := r.body["items"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["status"] != "ok" {
		t.Fatalf("series: %s", r.raw)
	}
	// Reihe ab Ölwechsel: (300 + 300 + 150) ml / 4 000 km = 187,5 → 188 ml/1 000 km
	if v := items[0].(map[string]any)["consumption_per_1000"].(map[string]any)["value"].(float64); v != 188 {
		t.Fatalf("series consumption: %v", v)
	}
	// L-6: 500 ml / 4 000 km = 125 ml/1 000 km, Hinweis Ölwechsel
	r = c.do("GET", "/vehicles/"+vid+"/oil/statistics?from=2026-03-01&to=2026-05-15", nil)
	expect(t, r, 200, "stats")
	if r.body["top_up_rate"].(map[string]any)["value"].(float64) != 125 || !strings.Contains(string(r.raw), "oil_change_in_range") ||
		r.body["oil_change_fill_total"].(map[string]any)["value"].(float64) != 4300 {
		t.Fatalf("L-6: %s", r.raw)
	}
	// L-8: Zeitraum ohne Nachfüllung mit Strecke → 0
	r = c.do("GET", "/vehicles/"+vid+"/oil/statistics?from=2026-03-02&to=2026-04-01", nil)
	if r.body["top_up_rate"].(map[string]any)["value"].(float64) != 0 {
		t.Fatalf("L-8: %s", r.raw)
	}
	// L-7: kein Messpunkt vor dem Zeitraum → nicht verfügbar
	r = c.do("GET", "/vehicles/"+vid+"/oil/statistics?from=2025-01-01&to=2025-02-01", nil)
	if r.body["top_up_rate_unavailable_reason"] != "distance_unknown" {
		t.Fatalf("L-7: %s", r.raw)
	}
	// L-11: 5 000 ml bei 4 300 ml Füllmenge → bestätigbar
	big := oilEntry("2026-05-20T12:00:00+02:00", "top_up", 54100)
	big["oil_added"] = map[string]any{"value": 5, "unit": "l"}
	r = c.do("POST", base, big)
	expect(t, r, 422, "L-11")
	if !strings.Contains(string(r.raw), "OIL_CAPACITY") {
		t.Fatalf("L-11: %s", r.raw)
	}
	big["confirm_anomalies"] = []string{"OIL_CAPACITY"}
	big["anomaly_reason"] = "Testfall"
	expect(t, c.do("POST", base, big), 201, "L-11 confirmed")
	// I-OI-1: Messung ohne Stand
	bad := oilEntry("2026-05-21T12:00:00+02:00", "check", 54200)
	expect(t, c.do("POST", base, bad), 422, "check without level")
	// OI-05: außerhalb −50…150 %
	bad["level_before"] = map[string]any{"percent": 160}
	expect(t, c.do("POST", base, bad), 422, "level range")
	// Ändern und Löschen
	id := last["id"].(string)
	r = c.do("PATCH", base+"/"+id, map[string]any{"level_before": map[string]any{"percent": 60}}, "If-Match", `"1"`)
	expect(t, r, 200, "patch")
	if r.body["pair"].(map[string]any)["consumption_per_1000"].(map[string]any)["value"].(float64) != 200 {
		t.Fatalf("L-4 after patch: %s", r.raw)
	}
	expect(t, c.do("DELETE", base+"/"+id, nil, "If-Match", `"2"`), 204, "delete")
	expect(t, c.do("GET", base+"/"+id, nil), 404, "deleted")
}
