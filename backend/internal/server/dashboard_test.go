package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sh1su/vector/backend/internal/assistant"
)

// Fahrt per Tachofoto: Foto hochladen, auswerten lassen, Fahrt mit Foto starten und beenden.
func TestTripDashboardPhotos(t *testing.T) {
	fake := &fakeClaude{}
	claude := httptest.NewServer(fake)
	defer claude.Close()
	e := newEnvWith(t, func(d *Deps, pool *pgxpool.Pool) {
		d.Assistant = assistant.NewService(pool, assistant.Config{Provider: "anthropic", APIKey: "test", Model: "claude-sonnet-4-5", BaseURL: claude.URL})
	})
	c := e.adminClient()
	vid := c.do("POST", "/vehicles", vehiclePayload("Sprinter")).body["id"].(string)
	base := "/vehicles/" + vid
	expect(t, c.do("POST", base+"/odometer/readings", map[string]any{"occurred_at": "2026-09-20T08:00:00+02:00", "time_zone": "Europe/Berlin",
		"value": map[string]any{"value": 98000, "unit": "km"}}), 201, "reading")
	private := c.do("GET", base+"/trip-categories", nil).body["items"].([]any)[0].(map[string]any)["id"].(string)

	photo := jpegWithGPS(t, 64, 48)
	up := c.upload(base+"/files", "tacho-start.jpg", photo, "capture_source", "camera", "captured_at_client", "2026-09-21T07:58:00+02:00")
	expect(t, up, 201, "upload start photo")
	startPhoto := up.body["id"].(string)

	// Ohne Zustimmung keine Übertragung an den Anbieter
	expect(t, c.do("POST", base+"/files/"+startPhoto+"/dashboard-reading", nil), 409, "consent required")
	expect(t, c.do("POST", "/assistant/consent", map[string]any{"accept_external_provider": true, "provider_name": "Anthropic (Claude)"}), 200, "consent")

	fake.replies = []map[string]any{msg("tool_use", map[string]any{"type": "tool_use", "id": "toolu_1", "name": "report_dashboard", "input": map[string]any{
		"readable": true, "odometer_value": 98312, "odometer_unit": "km", "trip_meter_value": 12.4, "fuel_level_percent": 75, "range_value": 520,
		"range_unit": "km", "outside_temperature_c": 18.5, "dashboard_time": "07:58", "warning_lights": []string{}, "confidence": "high", "notes": ""}})}
	r := c.do("POST", base+"/files/"+startPhoto+"/dashboard-reading", nil)
	expect(t, r, 200, "dashboard reading")
	if num(r.body, "odometer", "value") != 98312 || num(r.body, "last_odometer", "value") != 98000 || r.body["confidence"] != "high" ||
		r.body["summary"] != "Tank 75 %, Reichweite 520 km, 18,5 °C" || !strings.HasPrefix(r.body["captured_at"].(string), "2026-09-21T05:58:00") {
		t.Fatalf("reading: %s", r.raw)
	}
	// Anfrage enthält das Bild und erzwingt das Werkzeug
	req := fake.requests[len(fake.requests)-1]
	content := req["messages"].([]any)[0].(map[string]any)["content"].([]any)
	if content[0].(map[string]any)["type"] != "image" || req["tool_choice"].(map[string]any)["name"] != "report_dashboard" ||
		!strings.Contains(content[1].(map[string]any)["text"].(string), "98000 km") {
		t.Fatalf("request: %v", req)
	}

	// Fahrt mit Startfoto starten
	start := map[string]any{"started_at": "2026-09-21T07:58:00+02:00", "time_zone": "Europe/Berlin", "category_id": private,
		"start_odometer": map[string]any{"value": 98312, "unit": "km"}, "start_photo_id": startPhoto, "note": "Start: Tank 75 %"}
	open := c.do("POST", base+"/trips/start", start)
	expect(t, open, 201, "start with photo")
	if open.body["start_photo_id"] != startPhoto || open.body["end_photo_id"] != nil || open.body["note"] != "Start: Tank 75 %" {
		t.Fatalf("start: %s", open.raw)
	}
	tid := open.body["id"].(string)

	// Fremdes Foto (anderes Fahrzeug) wird abgelehnt (I-DO-1)
	other := c.do("POST", "/vehicles", vehiclePayload("Vito")).body["id"].(string)
	foreign := c.upload("/vehicles/"+other+"/files", "x.jpg", jpegWithGPS(t, 32, 32)).body["id"].(string)
	fin := map[string]any{"ended_at": "2026-09-21T09:10:00+02:00", "end_odometer": map[string]any{"value": 98401, "unit": "km"}, "end_photo_id": foreign}
	expect(t, c.do("POST", base+"/trips/"+tid+"/finish", fin, "If-Match", `"1"`), 422, "foreign photo")

	end := c.upload(base+"/files", "tacho-ende.jpg", jpegWithGPS(t, 48, 64), "capture_source", "camera")
	expect(t, end, 201, "upload end photo")
	fin["end_photo_id"] = end.body["id"]
	fin["note"] = "Start: Tank 75 %\nEnde: Tank 60 %"
	done := c.do("POST", base+"/trips/"+tid+"/finish", fin, "If-Match", `"1"`)
	expect(t, done, 200, "finish with photo")
	if done.body["end_photo_id"] != end.body["id"] || done.body["start_photo_id"] != startPhoto || num(done.body, "distance", "value") != 89 ||
		done.body["note"] != "Start: Tank 75 %\nEnde: Tank 60 %" {
		t.Fatalf("finish: %s", done.raw)
	}
	// Fotos bleiben bei Korrekturen erhalten und sind als Verknüpfungen sichtbar
	cor := c.do("POST", base+"/trips/"+tid+"/corrections", map[string]any{"end_odometer": map[string]any{"value": 98410, "unit": "km"}, "reason": "Foto nachgelesen"}, "If-Match", `"2"`)
	expect(t, cor, 201, "correction")
	if cor.body["start_photo_id"] != startPhoto || cor.body["end_photo_id"] != end.body["id"] {
		t.Fatalf("correction photos: %s", cor.raw)
	}
	att := c.do("GET", base+"/attachments?target_type=trip&target_id="+tid, nil)
	expect(t, att, 200, "attachments")
	if n := len(att.body["items"].([]any)); n != 2 {
		t.Fatalf("attachments: %s", att.raw)
	}
	// Verwendete Fotos lassen sich nicht löschen (DO-06)
	expect(t, c.do("DELETE", base+"/files/"+startPhoto, nil), 409, "photo in use")

	// Ohne konfigurierten Anbieter: 503 mit Hinweis auf manuelle Eingabe
	e2 := newEnv(t)
	c2 := e2.adminClient()
	v2 := c2.do("POST", "/vehicles", vehiclePayload("Golf")).body["id"].(string)
	f2 := c2.upload("/vehicles/"+v2+"/files", "t.jpg", photo).body["id"].(string)
	expect(t, c2.do("POST", "/vehicles/"+v2+"/files/"+f2+"/dashboard-reading", nil), 503, "disabled")
}
