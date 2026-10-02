package server

import (
	"testing"
)

func TestMaintenanceItemsStatusAndTemplates(t *testing.T) {
	e := newEnv(t)
	c := e.adminClient()
	vid := c.do("POST", "/vehicles", vehiclePayload("Sprinter")).body["id"].(string)
	expect(t, c.do("POST", "/vehicles/"+vid+"/odometer/readings", reading("2026-09-01T10:00:00+02:00", 58700)), 201, "reading")
	base := "/vehicles/" + vid + "/maintenance-items"

	r := c.do("GET", "/maintenance-templates", nil)
	expect(t, r, 200, "templates")
	var tpl map[string]any
	for _, it := range r.body["items"].([]any) {
		if it.(map[string]any)["id"] == "mb-sprinter-vs30-cdi-2019" {
			tpl = it.(map[string]any)
		}
	}
	if tpl == nil || len(tpl["items"].([]any)) < 10 {
		t.Fatalf("template missing: %s", r.raw)
	}

	// Ölwechsel 12 Monate / 15 000 km, zuletzt 10.03.2026 bei 45 000 km (Anker) → M-1/M-2
	oil := map[string]any{"title": "Ölwechsel", "category": "service", "schedule_mode": "from_last_completion", "interval_months": 12,
		"interval_distance": map[string]any{"value": 15000, "unit": "km"}, "anchor_date": "2026-03-10", "anchor_odometer": map[string]any{"value": 45000, "unit": "km"}}
	r = c.do("POST", base, oil)
	expect(t, r, 201, "create")
	st := r.body["status"].(map[string]any)
	if st["level"] != "upcoming" || st["reason_trigger"] != "distance" || st["due_date"] != "2027-03-10" || st["distance_remaining"].(map[string]any)["value"].(float64) != 1300 {
		t.Fatalf("M-2: %s", r.raw)
	}
	id := r.body["id"].(string)
	// HU nur Zeit, überfällig (M-9)
	hu := map[string]any{"title": "HU", "category": "legal_inspection", "schedule_mode": "from_last_completion", "interval_months": 24, "anchor_date": "2024-06-01"}
	huID := c.do("POST", base, hu).body["id"].(string)
	// I-MA-1: ohne Auslöser
	expect(t, c.do("POST", base, map[string]any{"title": "x", "category": "other", "schedule_mode": "from_last_completion"}), 422, "I-MA-1")
	// I-MA-4: Stunden bei km-Fahrzeug
	bad := map[string]any{"title": "x", "category": "other", "schedule_mode": "from_last_completion", "interval_distance": map[string]any{"value": 10, "unit": "h"}}
	expect(t, c.do("POST", base, bad), 422, "I-MA-4")

	r = c.do("GET", "/vehicles/"+vid+"/maintenance/status", nil)
	expect(t, r, 200, "status")
	items := r.body["items"].([]any)
	if items[0].(map[string]any)["item_id"] != huID || items[0].(map[string]any)["level"] != "overdue" {
		t.Fatalf("MA-07 order: %s", r.raw)
	}
	// Erledigung mit Idempotency-Key (ADR-012), M-8
	done := map[string]any{"kind": "done", "completed_on": "2026-09-01", "completed_odometer": map[string]any{"value": 58700, "unit": "km"}}
	r = c.do("POST", base+"/"+id+"/completions", done, "Idempotency-Key", "k1")
	expect(t, r, 201, "complete")
	expect(t, c.do("POST", base+"/"+id+"/completions", done, "Idempotency-Key", "k1"), 200, "complete repeat")
	r = c.do("GET", base+"/"+id, nil)
	st = r.body["status"].(map[string]any)
	if st["due_date"] != "2027-09-01" || st["due_total"].(map[string]any)["canonical"].(float64) != 73_700_000 || st["level"] != "ok" {
		t.Fatalf("M-8: %s", r.raw)
	}
	if n := len(c.do("GET", base+"/"+id+"/completions", nil).body["items"].([]any)); n != 1 {
		t.Fatalf("completions: %d", n)
	}
	expect(t, c.do("POST", base+"/"+huID+"/completions", map[string]any{"kind": "skipped", "completed_on": "2026-09-01"}), 422, "skip without reason")
	// Fälligkeitsfeed über alle Fahrzeuge
	r = c.do("GET", "/me/due?min_level=due", nil)
	expect(t, r, 200, "due feed")
	if len(r.body["items"].([]any)) != 1 {
		t.Fatalf("due feed: %s", r.raw)
	}
	// Ändern und Deaktivieren
	r = c.do("PATCH", base+"/"+huID, map[string]any{"active": false}, "If-Match", `"1"`)
	expect(t, r, 200, "deactivate")
	if n := len(c.do("GET", "/vehicles/"+vid+"/maintenance/status", nil).body["items"].([]any)); n != 1 {
		t.Fatalf("inactive still in status: %d", n)
	}
	expect(t, c.do("DELETE", base+"/"+huID, nil, "If-Match", `"2"`), 204, "delete")
	expect(t, c.do("GET", base+"/"+huID, nil), 404, "deleted")
}
