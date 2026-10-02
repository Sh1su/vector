package server

import "testing"

func TestSettingsPasswordSessionsAndInstallation(t *testing.T) {
	e := newEnv(t)
	c := e.adminClient()
	expect(t, c.do("PATCH", "/me/settings", map[string]any{"time_zone": "Mars/Olympus"}), 422, "invalid time zone")
	expect(t, c.do("PATCH", "/me/settings", map[string]any{"display_units": map[string]any{"distance": "furlong"}}), 422, "invalid unit")
	r := c.do("PATCH", "/me/settings", map[string]any{"display_units": map[string]any{"distance": "mi"}, "maintenance_thresholds": map[string]any{"upcoming_days": 60}})
	expect(t, r, 200, "settings")
	if r.body["display_units"].(map[string]any)["distance"] != "mi" || r.body["display_units"].(map[string]any)["volume"] != "l" {
		t.Fatalf("settings merge: %s", r.raw)
	}
	// Sitzungen
	other := e.client()
	other.login("admin@example.org", pw)
	r = c.do("GET", "/me/sessions", nil)
	expect(t, r, 200, "sessions")
	if len(r.body["items"].([]any)) != 2 {
		t.Fatalf("sessions: %s", r.raw)
	}
	// Passwortwechsel beendet andere Sitzungen
	expect(t, c.do("POST", "/me/password", map[string]any{"current_password": "falsch-falsch-falsch", "new_password": "Noch-sichereres-2027"}), 422, "wrong current")
	expect(t, c.do("POST", "/me/password", map[string]any{"current_password": pw, "new_password": "Noch-sichereres-2027"}), 204, "password")
	expect(t, other.do("GET", "/me", nil), 401, "other session revoked")
	expect(t, c.do("GET", "/me", nil), 200, "own session kept")
	// Installation (nur Admin)
	r = c.do("GET", "/admin/settings", nil)
	expect(t, r, 200, "admin settings")
	if r.body["assistant_enabled"] != false || r.body["odometer_v_max_kmh"].(float64) != 250 {
		t.Fatalf("installation: %s", r.raw)
	}
	expect(t, c.do("PATCH", "/admin/settings", map[string]any{"odometer_v_max_kmh": 300}), 422, "read only")
	r = c.do("PATCH", "/admin/settings", map[string]any{"assistant_enabled": true, "default_thresholds": map[string]any{"upcoming_days": 45}})
	expect(t, r, 200, "patch installation")
	if r.body["default_thresholds"].(map[string]any)["upcoming_days"].(float64) != 45 || r.body["default_thresholds"].(map[string]any)["due_days"].(float64) != 7 {
		t.Fatalf("installation patch: %s", r.raw)
	}
	b, _ := e.extraUser("bob@example.org")
	expect(t, b.do("GET", "/admin/settings", nil), 403, "non-admin")
}
