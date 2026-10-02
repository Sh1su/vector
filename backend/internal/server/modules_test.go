package server

import (
	"context"
	"strings"
	"testing"
)

// Integrationstests Iteration 2: ServiceHistory, Maintenance, Costs, Trips.

func num(m map[string]any, path ...string) float64 {
	var cur any = m
	for _, p := range path {
		mm, ok := cur.(map[string]any)
		if !ok {
			return -1
		}
		cur = mm[p]
	}
	f, _ := cur.(float64)
	return f
}

func serviceEntry(at string, km float64, items ...map[string]any) map[string]any {
	return map[string]any{"occurred_at": at, "time_zone": "Europe/Berlin", "time_precision": "date_only", "kind": "inspection",
		"title": "Inspektion", "currency": "EUR", "odometer": map[string]any{"value": km, "unit": "km"}, "cost_items": items}
}

func item(kind string, amount int) map[string]any {
	return map[string]any{"kind": kind, "amount_minor": amount}
}

// S-1 bis S-5, M-8 über die API, C-9 (Kostenbuch).
func TestServiceEntryWithCompletionsAndLedger(t *testing.T) {
	e := newEnv(t)
	c := e.adminClient()
	vid := c.do("POST", "/vehicles", vehiclePayload("Golf")).body["id"].(string)
	base := "/vehicles/" + vid
	oil := c.do("POST", base+"/maintenance-items", map[string]any{"title": "Ölwechsel", "category": "fluids", "schedule_mode": "from_last_completion",
		"interval_months": 12, "interval_distance": map[string]any{"value": 15000, "unit": "km"}})
	expect(t, oil, 201, "maintenance item")
	oilID := oil.body["id"].(string)
	if oil.body["status"].(map[string]any)["level"] != "unknown" {
		t.Fatalf("item without completion: %s", oil.raw)
	}
	filter := c.do("POST", base+"/maintenance-items", map[string]any{"title": "Innenraumfilter", "category": "filters", "schedule_mode": "from_last_completion", "interval_months": 24})
	filterID := filter.body["id"].(string)
	// I-MA-1: ohne Auslöser → 422
	expect(t, c.do("POST", base+"/maintenance-items", map[string]any{"title": "x", "category": "other", "schedule_mode": "from_last_completion"}), 422, "I-MA-1")

	// S-1 + S-2
	body := serviceEntry("2026-03-10T12:00:00+01:00", 45000, item("parts", 18000), item("labor", 24000), map[string]any{"kind": "other", "label": "Entsorgung", "amount_minor": 1250})
	body["completes_maintenance_item_ids"] = []string{oilID, filterID}
	r := c.do("POST", base+"/service-entries", body)
	expect(t, r, 201, "S-1")
	if num(r.body, "totals", "total", "amount_minor") != 43250 || num(r.body, "totals", "parts", "amount_minor") != 18000 ||
		num(r.body, "totals", "other", "amount_minor") != 1250 || num(r.body, "odometer_total", "canonical") != 45_000_000 {
		t.Fatalf("S-1 totals: %s", r.raw)
	}
	sid := r.body["id"].(string)
	st := c.do("GET", base+"/maintenance-items/"+oilID, nil).body["status"].(map[string]any)
	if st["due_date"] != "2027-03-10" || num(st, "due_total", "canonical") != 60_000_000 {
		t.Fatalf("S-2 due: %v", st)
	}
	// Messpunkt gehört dem Serviceeintrag (I-ODO-3): direkte Korrektur/Löschung → 409
	rid := r.body["odometer_reading_id"].(string)
	expect(t, c.do("POST", base+"/odometer/readings/"+rid+"/corrections", map[string]any{"value": map[string]any{"value": 1, "unit": "km"}, "reason": "x"},
		"If-Match", `"1"`), 409, "I-ODO-3 correction")
	expect(t, c.do("DELETE", base+"/odometer/readings/"+rid, nil, "If-Match", `"1"`), 409, "I-ODO-3 delete")

	// S-4: Datum korrigieren → Erledigungen folgen
	r = c.do("PATCH", base+"/service-entries/"+sid, map[string]any{"occurred_at": "2026-03-12T12:00:00+01:00"}, "If-Match", `"1"`)
	expect(t, r, 200, "S-4")
	st = c.do("GET", base+"/maintenance-items/"+oilID, nil).body["status"].(map[string]any)
	if st["due_date"] != "2027-03-12" {
		t.Fatalf("S-4 due: %v", st)
	}
	var n int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM maintenance.completion WHERE service_entry_id = $1`, sid).Scan(&n)
	if n != 2 {
		t.Fatalf("S-3/S-4: %d completions", n)
	}
	// S-6: Art ändern, gleicher Eintrag
	r = c.do("PATCH", base+"/service-entries/"+sid, map[string]any{"kind": "maintenance"}, "If-Match", `"2"`)
	expect(t, r, 200, "S-6")
	if r.body["id"] != sid || r.body["odometer_reading_id"] == rid {
		t.Fatalf("S-6: %s", r.raw)
	}
	rid = r.body["odometer_reading_id"].(string) // S-4 hat den Messpunkt korrigiert (I-ODO-4)
	// Kosten landen im Kostenbuch
	rep := c.do("GET", base+"/cost-report?from=2026-01-01&to=2026-12-31&group_by=cost_kind", nil)
	expect(t, rep, 200, "cost report")
	cur := rep.body["currencies"].([]any)[0].(map[string]any)
	if cur["running_total_minor"].(float64) != 43250 {
		t.Fatalf("ledger: %s", rep.raw)
	}
	sum := c.do("GET", base+"/service/summary?from=2026-01-01&to=2026-12-31", nil)
	expect(t, sum, 200, "summary")
	if num(sum.body["currencies"].([]any)[0].(map[string]any), "by_kind", "maintenance") != 43250 {
		t.Fatalf("SH-02: %s", sum.raw)
	}

	// I-SH-1: weder Positionen noch „Kosten unbekannt“
	bad := serviceEntry("2026-04-01T12:00:00+02:00", 46000)
	expect(t, c.do("POST", base+"/service-entries", bad), 422, "I-SH-1")
	bad["cost_unknown"] = true
	expect(t, c.do("POST", base+"/service-entries", bad), 201, "cost unknown")
	// SH-03: Zukunft
	expect(t, c.do("POST", base+"/service-entries", serviceEntry("2026-12-01T12:00:00+01:00", 50000, item("labor", 0))), 422, "future")
	// Plausibilität: rückläufiger Stand → P1, bestätigbar
	p1 := serviceEntry("2026-05-01T12:00:00+02:00", 40000, item("labor", 0))
	r = c.do("POST", base+"/service-entries", p1)
	expect(t, r, 422, "P1")
	if !strings.Contains(string(r.raw), `"P1"`) {
		t.Fatalf("P1: %s", r.raw)
	}

	// S-5 / C-9: Löschen entfernt Erledigungen, Messpunkt und Kostenbuch-Zeilen
	expect(t, c.do("DELETE", base+"/service-entries/"+sid, nil, "If-Match", `"3"`), 204, "S-5")
	st = c.do("GET", base+"/maintenance-items/"+oilID, nil).body["status"].(map[string]any)
	if st["level"] != "unknown" {
		t.Fatalf("S-5 status: %v", st)
	}
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM costs.ledger WHERE source_id = $1`, sid).Scan(&n)
	if n != 0 {
		t.Fatalf("C-9 ledger rows: %d", n)
	}
	var del bool
	_ = e.pool.QueryRow(context.Background(), `SELECT deleted_at IS NOT NULL FROM odometer.reading WHERE id = $1`, rid).Scan(&del)
	if !del {
		t.Fatal("S-5 reading not deleted")
	}
}

// Manuelle Erledigung mit Idempotency-Key, Auslassen mit Pflichtbegründung, Fälligkeits-Feed.
func TestMaintenanceCompletionsAndFeed(t *testing.T) {
	e := newEnv(t)
	c := e.adminClient()
	vid := c.do("POST", "/vehicles", vehiclePayload("Golf")).body["id"].(string)
	base := "/vehicles/" + vid
	hu := c.do("POST", base+"/maintenance-items", map[string]any{"title": "HU", "category": "legal_inspection", "schedule_mode": "from_last_completion",
		"interval_months": 24, "anchor_date": "2024-06-30"}).body["id"].(string)
	// M-9: überfällig
	feed := c.do("GET", "/me/due", nil)
	expect(t, feed, 200, "feed")
	items := feed.body["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["level"] != "overdue" || items[0].(map[string]any)["vehicle_id"] != vid {
		t.Fatalf("M-9 feed: %s", feed.raw)
	}
	path := base + "/maintenance-items/" + hu + "/completions"
	expect(t, c.do("POST", path, map[string]any{"kind": "skipped", "completed_on": "2026-09-01"}), 422, "skip without reason")
	r := c.do("POST", path, map[string]any{"kind": "done", "completed_on": "2026-09-01"}, "Idempotency-Key", "k1")
	expect(t, r, 201, "complete")
	expect(t, c.do("POST", path, map[string]any{"kind": "done", "completed_on": "2026-09-01"}, "Idempotency-Key", "k1"), 200, "idempotent")
	if n := len(c.do("GET", path, nil).body["items"].([]any)); n != 1 {
		t.Fatalf("completions: %d", n)
	}
	st := c.do("GET", base+"/maintenance/status", nil).body["items"].([]any)[0].(map[string]any)
	if st["level"] != "ok" || st["due_date"] != "2028-09-01" {
		t.Fatalf("after completion: %v", st)
	}
	expect(t, c.do("DELETE", path+"/"+r.body["id"].(string), nil), 204, "undo")
	if lvl := c.do("GET", base+"/maintenance/status", nil).body["items"].([]any)[0].(map[string]any)["level"]; lvl != "overdue" {
		t.Fatalf("after undo: %v", lvl)
	}
}

// C-3 (Vorkommen bestätigen, idempotent), C-4, sonstige Kosten, Kennzahlen.
func TestCostPlansAndReport(t *testing.T) {
	e := newEnv(t)
	c := e.adminClient()
	vid := c.do("POST", "/vehicles", vehiclePayload("Golf")).body["id"].(string)
	base := "/vehicles/" + vid
	plan := map[string]any{"category": "tax", "title": "Kfz-Steuer", "amount": map[string]any{"amount_minor": 18000, "currency": "EUR"},
		"interval_months": 12, "first_due_on": "2026-04-15"}
	bad := map[string]any{}
	for k, v := range plan {
		bad[k] = v
	}
	bad["interval_months"] = 0
	expect(t, c.do("POST", base+"/cost-plans", bad), 422, "C-4")
	pid := c.do("POST", base+"/cost-plans", plan).body["id"].(string)
	occ := c.do("GET", base+"/cost-occurrences", nil)
	expect(t, occ, 200, "occurrences")
	items := occ.body["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["state"] != "open" || items[0].(map[string]any)["due_on"] != "2026-04-15" {
		t.Fatalf("C-3 occurrences: %s", occ.raw)
	}
	confirm := base + "/cost-plans/" + pid + "/occurrences/2026-04-15/confirm"
	r1 := c.do("POST", confirm, map[string]any{})
	expect(t, r1, 200, "confirm")
	r2 := c.do("POST", confirm, map[string]any{})
	if r1.body["id"] != r2.body["id"] {
		t.Fatal("C-3 confirm not idempotent")
	}
	expect(t, c.do("POST", base+"/cost-plans/"+pid+"/occurrences/2026-04-16/confirm", map[string]any{}), 404, "not an occurrence")
	if n := len(c.do("GET", base+"/cost-occurrences", nil).body["items"].([]any)); n != 0 {
		t.Fatalf("after confirm: %d", n)
	}
	// Versicherung mit Leistungszeitraum, zeitanteilig (C-2-Muster)
	expect(t, c.do("POST", base+"/cost-entries", map[string]any{"category": "insurance", "title": "Versicherung", "incurred_on": "2026-01-02",
		"covers_from": "2026-01-01", "covers_to": "2026-12-31", "amount": map[string]any{"amount_minor": 60000, "currency": "EUR"}}), 201, "insurance")
	expect(t, c.do("POST", base+"/cost-entries", map[string]any{"category": "parking", "title": "Parken", "incurred_on": "2026-01-05",
		"amount": map[string]any{"amount_minor": 4000, "currency": "CHF"}}), 201, "chf")
	rep := c.do("GET", base+"/cost-report?from=2026-01-01&to=2026-01-31&allocation=prorated", nil)
	expect(t, rep, 200, "report")
	curs := rep.body["currencies"].([]any)
	if len(curs) != 2 {
		t.Fatalf("C-7: %s", rep.raw)
	}
	for _, x := range curs {
		m := x.(map[string]any)
		if m["currency"] == "EUR" && m["running_total_minor"].(float64) != 5096 {
			t.Fatalf("C-2 prorated: %s", rep.raw)
		}
	}
	if rep.body["distance"] != nil {
		t.Fatalf("C-8 distance: %s", rep.raw)
	}
	expect(t, c.do("GET", "/me/cost-report", nil), 200, "fleet")
}

func tripPayload(start, end string, s, en float64, cat string) map[string]any {
	return map[string]any{"started_at": start, "ended_at": end, "time_zone": "Europe/Berlin", "category_id": cat,
		"start_odometer": map[string]any{"value": s, "unit": "km"}, "end_odometer": map[string]any{"value": en, "unit": "km"}}
}

// T-1 bis T-9.
func TestTrips(t *testing.T) {
	e := newEnv(t)
	c := e.adminClient()
	vid := c.do("POST", "/vehicles", vehiclePayload("Golf")).body["id"].(string)
	base := "/vehicles/" + vid
	cats := c.do("GET", base+"/trip-categories", nil).body["items"].([]any)
	if len(cats) != 4 {
		t.Fatalf("default categories: %d", len(cats))
	}
	private := cats[0].(map[string]any)["id"].(string)
	business := cats[1].(map[string]any)["id"].(string)
	r := c.do("POST", base+"/trips", tripPayload("2026-09-01T08:00:00+02:00", "2026-09-01T08:45:00+02:00", 12000, 12038, private))
	expect(t, r, 201, "T-1")
	if num(r.body, "distance", "value") != 38 {
		t.Fatalf("T-1: %s", r.raw)
	}
	tid := r.body["id"].(string)
	r = c.do("POST", base+"/trips", tripPayload("2026-09-01T08:30:00+02:00", "2026-09-01T10:00:00+02:00", 12040, 12100, private))
	expect(t, r, 422, "T-2 overlap")
	if !strings.Contains(string(r.raw), "TR_OVERLAP") {
		t.Fatalf("T-2: %s", r.raw)
	}
	expect(t, c.do("POST", base+"/trips", tripPayload("2026-09-02T08:00:00+02:00", "2026-09-02T09:00:00+02:00", 12100, 11990, private)), 422, "T-5")
	expect(t, c.do("POST", base+"/trips", tripPayload("2026-09-02T08:00:00+02:00", "2026-09-02T09:00:00+02:00", 12100, 12150, business)), 422, "T-7")
	// T-3: angrenzend (halboffenes Intervall), gleicher Stand ist keine Anomalie
	b := tripPayload("2026-09-01T08:45:00+02:00", "2026-09-01T09:30:00+02:00", 12038, 12080, business)
	b["purpose"] = "Kunde Müller"
	expect(t, c.do("POST", base+"/trips", b), 201, "T-3 adjacent")
	// T-8: Lücke zur vorigen Fahrt
	r = c.do("POST", base+"/trips", tripPayload("2026-09-01T12:00:00+02:00", "2026-09-01T12:30:00+02:00", 12100, 12120, private))
	expect(t, r, 201, "T-8")
	if num(r.body, "gap_before", "value") != 20 {
		t.Fatalf("T-8 gap: %s", r.raw)
	}
	cid := r.body["id"].(string)
	// T-6: Korrektur als neue Fassung
	r = c.do("POST", base+"/trips/"+cid+"/corrections", map[string]any{"end_odometer": map[string]any{"value": 12165, "unit": "km"}, "reason": "Tippfehler"}, "If-Match", `"1"`)
	expect(t, r, 201, "T-6")
	if num(r.body, "distance", "value") != 65 || r.body["supersedes_id"] != cid {
		t.Fatalf("T-6: %s", r.raw)
	}
	hist := c.do("GET", base+"/trips/"+cid+"/history", nil).body["items"].([]any)
	if len(hist) != 2 || hist[0].(map[string]any)["status"] != "superseded" || num(hist[0].(map[string]any), "distance", "value") != 20 {
		t.Fatalf("T-6 history: %v", hist)
	}
	_ = tid
	// T-4: laufende Fahrt blockiert Nachträge danach
	start := map[string]any{"started_at": "2026-09-03T07:00:00+02:00", "time_zone": "Europe/Berlin", "category_id": private,
		"start_odometer": map[string]any{"value": 12200, "unit": "km"}}
	open := c.do("POST", base+"/trips/start", start)
	expect(t, open, 201, "start")
	expect(t, c.do("POST", base+"/trips", tripPayload("2026-09-03T12:00:00+02:00", "2026-09-03T13:00:00+02:00", 12300, 12310, private)), 422, "T-4")
	oid := open.body["id"].(string)
	fin := c.do("POST", base+"/trips/"+oid+"/finish", map[string]any{"ended_at": "2026-09-03T08:00:00+02:00", "end_odometer": map[string]any{"value": 12250, "unit": "km"}}, "If-Match", `"1"`)
	expect(t, fin, 200, "finish")
	if fin.body["status"] != "closed" || num(fin.body, "distance", "value") != 50 {
		t.Fatalf("finish: %s", fin.raw)
	}
	// T-9: Storno zählt nicht in Summen
	expect(t, c.do("POST", base+"/trips/"+oid+"/cancel", map[string]any{"reason": "doppelt"}, "If-Match", `"2"`), 200, "T-9 cancel")
	rep := c.do("GET", base+"/trip-report?from=2026-09-01&to=2026-09-30", nil)
	expect(t, rep, 200, "report")
	if num(rep.body, "total", "value") != 145 {
		t.Fatalf("TR-05 total: %s", rep.raw)
	}
	if n := len(c.do("GET", base+"/trips", nil).body["items"].([]any)); n != 3 {
		t.Fatalf("valid trips: %d", n)
	}
}
