package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/sh1su/vector/backend/internal/identity"
)

func fill(at string, km float64, liters float64, full bool) map[string]any {
	lvl := "partial"
	if full {
		lvl = "full"
	}
	b := map[string]any{"occurred_at": at, "time_zone": "Europe/Berlin", "energy_carrier": "petrol", "fill_level": lvl,
		"quantity": map[string]any{"value": liters, "unit": "l"}}
	if km > 0 {
		b["odometer"] = map[string]any{"value": km, "unit": "km"}
	}
	return b
}

func TestFuelFillsConsumptionAndOdometer(t *testing.T) {
	e := newEnv(t)
	c := e.adminClient()
	p := vehiclePayload("Golf")
	p["tank_capacity"] = map[string]any{"petrol": map[string]any{"value": 50, "unit": "l"}}
	p["odometer_required"] = false
	vid := c.do("POST", "/vehicles", p).body["id"].(string)
	base := "/vehicles/" + vid + "/fuel-fills"
	// Datensatz A
	rows := []map[string]any{fill("2026-03-01T12:00:00+01:00", 10000, 40, true), fill("2026-03-10T12:00:00+01:00", 10350, 20, false),
		fill("2026-03-20T12:00:00+01:00", 10800, 35, true), fill("2026-04-05T12:00:00+02:00", 11400, 42, true),
		fill("2026-04-20T12:00:00+02:00", 12000, 36, true), fill("2026-04-30T12:00:00+02:00", 0, 10, false), fill("2026-05-10T12:00:00+02:00", 12500, 25, true)}
	rows[3]["previous_missed"] = true
	rows[2]["price_per_unit"] = map[string]any{"value": 1.799, "currency": "EUR", "per_unit": "l"}
	var ids []string
	for i, b := range rows {
		r := c.do("POST", base, b)
		expect(t, r, 201, "fill")
		ids = append(ids, r.body["id"].(string))
		if i == 2 {
			// U-7: 1,799 €/l × 35 l = 62,965 → 62,97 €; U-1 Intervall
			if r.body["cost"].(map[string]any)["amount_minor"].(float64) != 6297 {
				t.Fatalf("U-7: %s", r.raw)
			}
			iv := r.body["interval"].(map[string]any)
			if iv["status"] != "computed" || iv["consumption"].(map[string]any)["value"].(float64) != 6.88 {
				t.Fatalf("U-1: %s", r.raw)
			}
		}
	}
	r := c.do("GET", "/vehicles/"+vid+"/fuel/consumption?energy_carrier=petrol", nil)
	expect(t, r, 200, "consumption")
	cons := r.body["consumption"].(map[string]any)
	if cons["average"].(map[string]any)["value"].(float64) != 6.63 || cons["not_computable_intervals"].(float64) != 1 || len(cons["monthly"].([]any)) != 3 {
		t.Fatalf("U-5/U-6: %s", r.raw)
	}
	// Messpunkte mit Herkunft „fuel“ wurden angelegt
	odo := c.do("GET", "/vehicles/"+vid+"/odometer/readings?limit=50", nil).body["items"].([]any)
	if len(odo) != 6 || odo[0].(map[string]any)["source"] != "fuel" {
		t.Fatalf("readings: %d", len(odo))
	}
	// Messpunkt eines Tankvorgangs lässt sich nicht direkt löschen (I-ODO-3)
	rid := odo[0].(map[string]any)["id"].(string)
	expect(t, c.do("DELETE", "/vehicles/"+vid+"/odometer/readings/"+rid, nil, "If-Match", `"1"`), 409, "owned reading delete")
	// U-8: Menge größer als Tank → bestätigbarer Befund F1, zusammen mit P1 in einer Antwort
	b := fill("2026-05-20T12:00:00+02:00", 12000, 60, true)
	r = c.do("POST", base, b)
	expect(t, r, 422, "U-8")
	if !strings.Contains(string(r.raw), `"F1"`) || !strings.Contains(string(r.raw), `"P1"`) {
		t.Fatalf("U-8 anomalies: %s", r.raw)
	}
	b["confirm_anomalies"] = []string{"F1", "P1"}
	b["anomaly_reason"] = "Kanister mitgefüllt, Tacho vertippt"
	expect(t, c.do("POST", base, b), 201, "U-8 confirmed")
	// F2: Energieträger nicht am Fahrzeug → nicht bestätigbar
	d := fill("2026-05-21T12:00:00+02:00", 0, 10, true)
	d["energy_carrier"] = "diesel"
	d["confirm_anomalies"] = []string{"F2"}
	d["anomaly_reason"] = "x"
	expect(t, c.do("POST", base, d), 422, "F2")
	// Änderung des Stands = Korrektur des Messpunkts (I-FU-4)
	last := ids[4]
	r = c.do("PATCH", base+"/"+last, map[string]any{"odometer": map[string]any{"value": 11900, "unit": "km"}}, "If-Match", `"1"`)
	expect(t, r, 200, "patch odometer")
	if r.body["odometer_total"].(map[string]any)["canonical"].(float64) != 11_900_000 || r.header.Get("ETag") != `"2"` {
		t.Fatalf("patch: %s", r.raw)
	}
	n := 0
	for _, it := range c.do("GET", "/vehicles/"+vid+"/odometer/readings?include_superseded=true&limit=50", nil).body["items"].([]any) {
		if it.(map[string]any)["status"] == "superseded" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("superseded readings: %d", n)
	}
	// Löschen entfernt auch den Messpunkt (I-FU-3)
	expect(t, c.do("DELETE", base+"/"+last, nil, "If-Match", `"2"`), 204, "delete fill")
	if got := len(c.do("GET", "/vehicles/"+vid+"/odometer/readings?limit=50", nil).body["items"].([]any)); got != 6 {
		t.Fatalf("readings after delete: %d", got)
	}
	r = c.do("GET", base+"?limit=3", nil)
	if len(r.body["items"].([]any)) != 3 || r.body["next_cursor"] == nil {
		t.Fatalf("page: %s", r.raw)
	}
	r = c.do("GET", base+"?limit=3&cursor="+r.body["next_cursor"].(string), nil)
	if len(r.body["items"].([]any)) != 3 {
		t.Fatalf("page 2: %s", r.raw)
	}
}

// Ersteinrichtung ohne konfiguriertes Token nur, solange kein Konto existiert;
// Änderungshistorie und Spezifikation sind per API abrufbar.
func TestSetupStatusHistoryAndSpec(t *testing.T) {
	e := newEnv(t)
	anon := e.client()
	r := anon.do("GET", "/auth/setup", nil)
	expect(t, r, 200, "setup status")
	if r.body["setup_required"] != true || r.body["token_required"] != true {
		t.Fatalf("status: %s", r.raw)
	}
	ids := identity.NewService(e.pool, "", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := ids.Setup(context.Background(), "", "first@example.org", "Erste", pw); err != nil {
		t.Fatalf("tokenless setup: %v", err)
	}
	if _, err := ids.Setup(context.Background(), "", "second@example.org", "Zweite", pw); err == nil {
		t.Fatal("second setup must fail")
	}
	if r = anon.do("GET", "/auth/setup", nil); r.body["setup_required"] != false {
		t.Fatalf("status after setup: %s", r.raw)
	}
	c := e.client()
	c.login("first@example.org", pw)
	vid := c.do("POST", "/vehicles", vehiclePayload("A")).body["id"].(string)
	expect(t, c.do("POST", "/vehicles/"+vid+"/odometer/readings", reading("2026-09-01T10:00:00+02:00", 1000)), 201, "reading")
	r = c.do("GET", "/vehicles/"+vid+"/audit-events", nil)
	expect(t, r, 200, "audit")
	if !strings.Contains(string(r.raw), "odometer.reading_recorded") {
		t.Fatalf("audit: %s", r.raw)
	}
	res, err := http.Get(e.srv.URL + "/api/v1/openapi.json")
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("openapi.json: %v", err)
	}
	res.Body.Close()
}
