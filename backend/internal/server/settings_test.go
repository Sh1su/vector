package server

import (
	"net/http"
	"testing"
	"time"
)

// Wartungsbücher: Katalog, Übernehmen mit Basis ab Erstzulassung, Wiederholung überspringt.
func TestMaintenanceBooks(t *testing.T) {
	e := newEnv(t)
	c := e.adminClient()
	r := c.do("GET", "/maintenance-books", nil)
	expect(t, r, 200, "books")
	ids := map[string]bool{}
	for _, b := range r.body["items"].([]any) {
		ids[b.(map[string]any)["id"].(string)] = true
	}
	if !ids["hyundai-tucson-nx4"] || !ids["leapmotor-b10"] {
		t.Fatalf("books: %s", r.raw)
	}
	r = c.do("GET", "/maintenance-books?make=leapmotor", nil)
	if items := r.body["items"].([]any); len(items) != 1 {
		t.Fatalf("filter by make: %s", r.raw)
	}

	vid := c.do("POST", "/vehicles", vehiclePayload("Tucson")).body["id"].(string)
	base := "/vehicles/" + vid
	apply := map[string]any{"anchor_date": "2024-05-15", "anchor_odometer": map[string]any{"value": 0, "unit": "km"}, "since_new": true}
	r = c.do("POST", base+"/maintenance-books/hyundai-tucson-nx4/apply", apply)
	expect(t, r, 200, "apply")
	created := r.body["created"].([]any)
	if len(created) != 11 || len(r.body["skipped"].([]any)) != 0 {
		t.Fatalf("apply: %s", r.raw)
	}
	byTitle := map[string]map[string]any{}
	for _, it := range created {
		m := it.(map[string]any)
		byTitle[m["title"].(string)] = m
	}
	// HU: erstes Intervall 36 Monate ab Erstzulassung, danach 24
	if st := byTitle["Hauptuntersuchung (HU/AU)"]["status"].(map[string]any); st["due_date"] != "2027-05-15" {
		t.Fatalf("HU due: %v", st)
	}
	// Kühlmittel: erstmals 210.000 km bzw. 10 Jahre
	if st := byTitle["Kühlmittel"]["status"].(map[string]any); st["due_date"] != "2034-05-15" || num(st, "due_total", "canonical") != 210_000_000 {
		t.Fatalf("coolant due: %v", st)
	}
	if st := byTitle["Inspektion"]["status"].(map[string]any); st["due_date"] != "2025-05-15" || num(st, "due_total", "canonical") != 15_000_000 {
		t.Fatalf("inspection due: %v", st)
	}
	// Wiederholung legt nichts doppelt an
	r = c.do("POST", base+"/maintenance-books/hyundai-tucson-nx4/apply", apply)
	expect(t, r, 200, "apply again")
	if len(r.body["created"].([]any)) != 0 || len(r.body["skipped"].([]any)) != 11 {
		t.Fatalf("apply again: %s", r.raw)
	}
	// Auswahl einzelner Positionen
	vid2 := c.do("POST", "/vehicles", vehiclePayload("B10")).body["id"].(string)
	r = c.do("POST", "/vehicles/"+vid2+"/maintenance-books/leapmotor-b10/apply", map[string]any{"item_keys": []string{"inspection", "brake-fluid"}})
	expect(t, r, 200, "apply subset")
	if len(r.body["created"].([]any)) != 2 {
		t.Fatalf("subset: %s", r.raw)
	}
	expect(t, c.do("POST", base+"/maintenance-books/unbekannt/apply", map[string]any{}), 404, "unknown book")
	// Leser dürfen nicht übernehmen
	other, _ := e.extraUser("leser@example.org")
	expect(t, other.do("POST", base+"/maintenance-books/leapmotor-b10/apply", map[string]any{}), 404, "foreign vehicle")
}

// Passwort ändern beendet andere Sitzungen; Sitzungen auflisten und beenden; App-Sitzung lang.
func TestPasswordAndSessions(t *testing.T) {
	e := newEnv(t)
	c := e.adminClient()
	app := e.client()
	r := app.do("POST", "/auth/login", map[string]any{"email": "admin@example.org", "password": pw, "client_kind": "android"})
	expect(t, r, 200, "android login")
	var exp time.Time
	for _, ck := range (&http.Response{Header: r.header}).Cookies() {
		if ck.Name == "vectra_session" {
			exp = ck.Expires
		}
	}
	if time.Until(exp) < 300*24*time.Hour {
		t.Fatalf("android session too short: %v", exp)
	}

	r = c.do("GET", "/me/sessions", nil)
	expect(t, r, 200, "sessions")
	items := r.body["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("sessions: %s", r.raw)
	}
	var appID string
	current := 0
	for _, it := range items {
		m := it.(map[string]any)
		if m["current"] == true {
			current++
		}
		if m["client_kind"] == "android" {
			appID = m["id"].(string)
		}
	}
	if current != 1 || appID == "" {
		t.Fatalf("sessions: %s", r.raw)
	}

	expect(t, c.do("POST", "/me/password", map[string]any{"current_password": "falsch-falsch-falsch", "new_password": "Neues-Passwort-2026!"}), 422, "wrong current")
	expect(t, c.do("POST", "/me/password", map[string]any{"current_password": pw, "new_password": "kurz"}), 422, "weak")
	expect(t, c.do("POST", "/me/password", map[string]any{"current_password": pw, "new_password": "Neues-Passwort-2026!"}), 204, "change")
	// Andere Sitzung beendet, eigene bleibt
	expect(t, app.do("GET", "/me", nil), 401, "app session revoked")
	expect(t, c.do("GET", "/me", nil), 200, "own session")
	expect(t, e.client().do("POST", "/auth/login", map[string]any{"email": "admin@example.org", "password": pw}), 422, "old password")

	app2 := e.client()
	app2.login("admin@example.org", "Neues-Passwort-2026!")
	var id2 string
	for _, it := range c.do("GET", "/me/sessions", nil).body["items"].([]any) {
		if m := it.(map[string]any); m["current"] != true {
			id2 = m["id"].(string)
		}
	}
	expect(t, c.do("DELETE", "/me/sessions/"+id2, nil), 204, "revoke")
	expect(t, app2.do("GET", "/me", nil), 401, "revoked")
	expect(t, c.do("DELETE", "/me/sessions/"+id2, nil), 404, "revoke twice")
}

// MA-05: Schwellen aus den Einstellungen des Halters gelten für alle Definitionen ohne eigene Schwelle.
func TestOwnerThresholdsFromSettings(t *testing.T) {
	e := newEnv(t)
	c := e.adminClient()
	vid := c.do("POST", "/vehicles", vehiclePayload("Golf")).body["id"].(string)
	anchor := time.Now().AddDate(0, -10, 0).Format(time.DateOnly)
	r := c.do("POST", "/vehicles/"+vid+"/maintenance-items", map[string]any{"title": "Inspektion", "category": "service",
		"schedule_mode": "from_last_completion", "interval_months": 12, "anchor_date": anchor})
	expect(t, r, 201, "item")
	if lvl := r.body["status"].(map[string]any)["level"]; lvl != "ok" {
		t.Fatalf("default threshold: %v", lvl)
	}
	expect(t, c.do("PATCH", "/me/settings", map[string]any{"maintenance_thresholds": map[string]any{"upcoming_days": 90}}), 200, "settings")
	st := c.do("GET", "/vehicles/"+vid+"/maintenance/status", nil).body["items"].([]any)[0].(map[string]any)
	if st["level"] != "upcoming" {
		t.Fatalf("owner threshold: %v", st)
	}
}
