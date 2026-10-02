package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"gopkg.in/yaml.v3"

	"github.com/sh1su/vector/backend/internal/assistant"
	"github.com/sh1su/vector/backend/internal/fuel"
	"github.com/sh1su/vector/backend/internal/identity"
	istore "github.com/sh1su/vector/backend/internal/identity/store"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/maintenance"
	"github.com/sh1su/vector/backend/internal/odometer"
	"github.com/sh1su/vector/backend/internal/oil"
	"github.com/sh1su/vector/backend/internal/platform/db"
	"github.com/sh1su/vector/backend/internal/vehicles"
	vstore "github.com/sh1su/vector/backend/internal/vehicles/store"
)

// Integrationstests gegen PostgreSQL. Aufruf mit
// VECTRA_TEST_DATABASE_URL=postgres://… go test ./internal/server/

const setupToken = "test-setup-token"

type env struct {
	t    *testing.T
	srv  *httptest.Server
	pool *pgxpool.Pool
	chat *fakeChat
}

func newEnv(t *testing.T) *env {
	url := os.Getenv("VECTRA_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("VECTRA_TEST_DATABASE_URL nicht gesetzt")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE assistant.request_log, assistant.proposal, assistant.message, assistant.conversation, assistant.user_state, audit.event, maintenance.completion, maintenance.item, fuel.fill, oil.entry, odometer.reading, odometer.segment, identity.vehicle_membership, vehicles.vehicle, identity.session, identity.account`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE identity.installation_settings SET settings = '{}'`); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ids := identity.NewService(pool, setupToken, log)
	veh := vehicles.NewService(pool)
	veh.Settings = ids.Settings
	veh.HasReadings = func(ctx context.Context, q vstore.DBTX, id uuid.UUID) (bool, error) { return odometer.HasReadings(ctx, q, id) }
	odo := odometer.NewService(pool, odometer.Config{VMaxKmh: 250})
	odo.Now = func() time.Time { return time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC) }
	fu := fuel.NewService(pool, odo)
	fu.Now = odo.Now
	oi := oil.NewService(pool, odo)
	oi.Now = odo.Now
	ma := maintenance.NewService(pool, odo)
	ma.Now = odo.Now
	ma.UserSettings = ids.Settings
	ma.InstallSettings = ids.Installation
	chat := &fakeChat{}
	as := assistant.NewService(pool, assistant.Deps{Vehicles: veh, Odometer: odo, Fuel: fu, Oil: oi, Maintenance: ma,
		Units: func(ctx context.Context, id uuid.UUID) kernel.Units { st, _ := ids.Settings(ctx, id); return kernel.UnitsFromSettings(st) }},
		chat, assistant.Config{Provider: "fake", Name: "Testanbieter", Model: "fake-1", External: true, DailyLimit: 5, RetentionDays: 30}, log)
	as.Installation = ids.Installation
	as.Now = odo.Now
	srv := httptest.NewServer(Handler(Deps{Assistant: as, Identity: ids, Vehicles: veh, Odometer: odo, Fuel: fu, Oil: oi, Maintenance: ma, Log: log, CookieSecure: false, OdometerVMaxKmh: 250}))
	t.Cleanup(func() { srv.Close(); pool.Close() })
	return &env{t: t, srv: srv, pool: pool, chat: chat}
}

type client struct {
	e    *env
	http *http.Client
	csrf string
}

func (e *env) client() *client {
	jar, _ := cookiejar.New(nil)
	return &client{e: e, http: &http.Client{Jar: jar}}
}

type resp struct {
	status int
	header http.Header
	body   map[string]any
	raw    []byte
}

func (c *client) do(method, path string, body any, headers ...string) resp {
	c.e.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.e.srv.URL+"/api/v1"+path, r)
	if body != nil {
		ct := "application/json"
		if method == http.MethodPatch {
			ct = "application/merge-patch+json"
		}
		req.Header.Set("Content-Type", ct)
	}
	if c.csrf != "" {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := resp{status: res.StatusCode, header: res.Header, raw: raw}
	_ = json.Unmarshal(raw, &out.body)
	for _, ck := range res.Cookies() {
		if ck.Name == "vectra_csrf" {
			c.csrf = ck.Value
		}
	}
	return out
}

func (c *client) login(email, pw string) {
	c.e.t.Helper()
	if r := c.do("POST", "/auth/login", map[string]any{"email": email, "password": pw}); r.status != 200 {
		c.e.t.Fatalf("login: %d %s", r.status, r.raw)
	}
}

func expect(t *testing.T, r resp, status int, what string) {
	t.Helper()
	if r.status != status {
		t.Fatalf("%s: status %d, want %d: %s", what, r.status, status, r.raw)
	}
}

const pw = "Sicheres-Passwort-2026"

// adminClient richtet die Installation ein und meldet den Admin an.
func (e *env) adminClient() *client {
	c := e.client()
	expect(e.t, c.do("POST", "/auth/setup", map[string]any{"setup_token": setupToken, "email": "Admin@Example.org", "display_name": "Admin", "password": pw}), 201, "setup")
	c.login("admin@example.org", pw)
	return c
}

// extraUser legt ein weiteres aktives Konto direkt an (Einladungen folgen in Iteration 3).
func (e *env) extraUser(email string) (*client, uuid.UUID) {
	hash, _ := identity.HashPassword(pw)
	id := kernel.NewID()
	_, err := istore.New(e.pool).InsertAccount(context.Background(), istore.InsertAccountParams{ID: pgtype.UUID{Bytes: id, Valid: true},
		Email: email, DisplayName: email, Status: "active", PasswordHash: pgtype.Text{String: hash, Valid: true}})
	if err != nil {
		e.t.Fatal(err)
	}
	c := e.client()
	c.login(email, pw)
	return c, id
}

func vehiclePayload(name string) map[string]any {
	return map[string]any{"display_name": name, "body_type": "car", "usage_meter": "distance", "energy_carriers": []string{"petrol"}}
}

func TestSetupLoginAndSession(t *testing.T) {
	e := newEnv(t)
	anon := e.client()
	expect(t, anon.do("GET", "/health", nil), 200, "health public")
	expect(t, anon.do("GET", "/me", nil), 401, "me without session")
	expect(t, anon.do("POST", "/auth/setup", map[string]any{"setup_token": "falsch", "email": "a@b.de", "display_name": "A", "password": pw}), 403, "wrong setup token")
	expect(t, anon.do("POST", "/auth/setup", map[string]any{"setup_token": setupToken, "email": "a@b.de", "display_name": "A", "password": "kurz"}), 422, "weak password")
	admin := e.adminClient()
	expect(t, anon.do("POST", "/auth/setup", map[string]any{"setup_token": setupToken, "email": "x@b.de", "display_name": "X", "password": pw}), 403, "setup only once (token consumed)")
	r := admin.do("GET", "/me", nil)
	expect(t, r, 200, "me")
	if r.body["email"] != "admin@example.org" || r.body["is_admin"] != true {
		t.Fatalf("me: %s", r.raw)
	}
	expect(t, anon.do("POST", "/auth/login", map[string]any{"email": "admin@example.org", "password": "falsch-falsch-falsch"}), 422, "wrong password")
	// CSRF: ohne Header wird eine zustandsändernde Anfrage abgelehnt.
	saved := admin.csrf
	admin.csrf = ""
	expect(t, admin.do("POST", "/vehicles", vehiclePayload("Golf")), 403, "missing csrf")
	admin.csrf = saved
	expect(t, admin.do("POST", "/auth/logout", nil), 204, "logout")
	expect(t, admin.do("GET", "/me", nil), 401, "me after logout")
}

func TestVehiclesVE01AndIdempotency(t *testing.T) {
	e := newEnv(t)
	c := e.adminClient()
	p := vehiclePayload("Golf")
	p["vin"] = "wvw zzz 1k z 6w 123456"
	r := c.do("POST", "/vehicles", p)
	expect(t, r, 201, "create")
	if r.body["vin"] != "WVWZZZ1KZ6W123456" || r.body["my_role"] != "owner" || r.header.Get("ETag") != `"1"` {
		t.Fatalf("V-1: %s etag=%s", r.raw, r.header.Get("ETag"))
	}
	// V-2: unzulässiges Zeichen → 422, nicht bestätigbar
	p2 := vehiclePayload("Oldie")
	p2["vin"] = "WVWZOZ1KZ6W123456"
	expect(t, c.do("POST", "/vehicles", p2), 422, "V-2")
	// V-3: 13 Zeichen → Befund, nach Bestätigung gespeichert
	p3 := vehiclePayload("Käfer")
	p3["vin"] = "1182345678901"
	r = c.do("POST", "/vehicles", p3)
	expect(t, r, 422, "V-3 warn")
	if !strings.Contains(string(r.raw), "VIN_NONSTANDARD") {
		t.Fatalf("V-3 anomaly missing: %s", r.raw)
	}
	p3["confirm_anomalies"] = []string{"VIN_NONSTANDARD"}
	expect(t, c.do("POST", "/vehicles", p3), 201, "V-3 confirmed")
	// Idempotentes Anlegen mit Client-ID (ADR-006)
	id := kernel.NewID().String()
	p4 := vehiclePayload("Transit")
	p4["id"] = id
	expect(t, c.do("POST", "/vehicles", p4), 201, "create with id")
	expect(t, c.do("POST", "/vehicles", p4), 200, "repeat same content")
	p4["display_name"] = "Anders"
	expect(t, c.do("POST", "/vehicles", p4), 409, "same id other content")
	// Liste sortiert nach Namen
	r = c.do("GET", "/vehicles", nil)
	expect(t, r, 200, "list")
	if n := len(r.body["items"].([]any)); n != 3 {
		t.Fatalf("list count %d", n)
	}
}

func TestVehicleUpdateLocking(t *testing.T) {
	e := newEnv(t)
	c := e.adminClient()
	r := c.do("POST", "/vehicles", vehiclePayload("Golf"))
	id := r.body["id"].(string)
	expect(t, c.do("PATCH", "/vehicles/"+id, map[string]any{"license_plate": "B-VX 123"}), 428, "patch without If-Match")
	r = c.do("PATCH", "/vehicles/"+id, map[string]any{"license_plate": "B-VX 123"}, "If-Match", `"1"`)
	expect(t, r, 200, "patch")
	if r.body["license_plate"] != "B-VX 123" || r.header.Get("ETag") != `"2"` {
		t.Fatalf("patch: %s", r.raw)
	}
	r = c.do("PATCH", "/vehicles/"+id, map[string]any{"license_plate": nil}, "If-Match", `"1"`)
	expect(t, r, 412, "stale If-Match")
	if r.body["current"] == nil {
		t.Fatal("412 without current")
	}
	r = c.do("PATCH", "/vehicles/"+id, map[string]any{"license_plate": nil}, "If-Match", `"2"`)
	expect(t, r, 200, "null removes field")
	if _, ok := r.body["license_plate"]; ok {
		t.Fatalf("license_plate not removed: %s", r.raw)
	}
	// I-VE-3: Zählergröße unveränderlich, sobald Messpunkte existieren
	expect(t, c.do("POST", "/vehicles/"+id+"/odometer/readings", reading("2026-09-01T10:00:00+02:00", 1000)), 201, "reading")
	expect(t, c.do("PATCH", "/vehicles/"+id, map[string]any{"usage_meter": "engine_hours"}, "If-Match", `"3"`), 409, "I-VE-3")
	expect(t, c.do("DELETE", "/vehicles/"+id, nil, "If-Match", `"3"`), 204, "delete")
	expect(t, c.do("GET", "/vehicles/"+id, nil), 404, "deleted vehicle")
}

func reading(at string, km float64) map[string]any {
	return map[string]any{"occurred_at": at, "time_zone": "Europe/Berlin", "value": map[string]any{"value": km, "unit": "km"}}
}

func TestOdometerPlausibilityAndCorrection(t *testing.T) {
	e := newEnv(t)
	c := e.adminClient()
	vid := c.do("POST", "/vehicles", vehiclePayload("Golf")).body["id"].(string)
	base := "/vehicles/" + vid + "/odometer"
	expect(t, c.do("GET", base+"/current", nil), 200, "current unknown")
	expect(t, c.do("POST", base+"/readings", reading("2026-03-10T12:00:00+01:00", 45000)), 201, "O-1 first")
	// O-1: rückläufig → P1
	r := c.do("POST", base+"/readings", reading("2026-03-12T08:00:00+01:00", 44500))
	expect(t, r, 422, "O-1 P1")
	if !strings.Contains(string(r.raw), `"P1"`) {
		t.Fatalf("P1 missing: %s", r.raw)
	}
	b := reading("2026-03-12T08:00:00+01:00", 44500)
	b["confirm_anomalies"] = []string{"P1"}
	expect(t, c.do("POST", base+"/readings", b), 422, "confirmation without reason")
	b["anomaly_reason"] = "Vorheriger Wert war ein Tippfehler"
	r = c.do("POST", base+"/readings", b)
	expect(t, r, 201, "O-1 confirmed")
	if r.body["status"] != "confirmed_anomaly" {
		t.Fatalf("status: %s", r.raw)
	}
	cur := c.do("GET", base+"/current", nil)
	if cur.body["total"].(map[string]any)["canonical"].(float64) != 44_500_000 {
		t.Fatalf("O-1 current: %s", cur.raw)
	}
	// O-2: Sprung → P3
	r = c.do("POST", base+"/readings", reading("2026-03-12T09:00:00+01:00", 44900))
	expect(t, r, 422, "O-2 P3")
	// ODO-02: Zukunft → nicht bestätigbar
	f := reading("2026-10-05T12:00:00+02:00", 50000)
	f["confirm_anomalies"] = []string{"P4"}
	f["anomaly_reason"] = "x"
	expect(t, c.do("POST", base+"/readings", f), 422, "future")
	// Einheit passt nicht zur Zählergröße
	h := reading("2026-03-20T12:00:00+01:00", 45000)
	h["value"] = map[string]any{"value": 10, "unit": "h"}
	expect(t, c.do("POST", base+"/readings", h), 422, "unit mismatch")

	// O-10: Korrektur statt Überschreiben
	first := c.do("GET", base+"/readings?limit=10", nil).body["items"].([]any)
	var id45 string
	for _, it := range first {
		m := it.(map[string]any)
		if m["meter_value"].(map[string]any)["canonical"].(float64) == 45_000_000 {
			id45 = m["id"].(string)
		}
	}
	corr := map[string]any{"value": map[string]any{"value": 44000, "unit": "km"}, "reason": "Tippfehler"}
	expect(t, c.do("POST", base+"/readings/"+id45+"/corrections", corr), 428, "correction without If-Match")
	r = c.do("POST", base+"/readings/"+id45+"/corrections", corr, "If-Match", `"1"`)
	expect(t, r, 201, "correction")
	if r.body["supersedes_id"] != id45 {
		t.Fatalf("supersedes: %s", r.raw)
	}
	old := c.do("GET", base+"/readings/"+id45, nil)
	if old.body["status"] != "superseded" || old.body["superseded_by_id"] != r.body["id"] {
		t.Fatalf("old reading: %s", old.raw)
	}
	expect(t, c.do("POST", base+"/readings/"+id45+"/corrections", corr, "If-Match", `"2"`), 409, "correct superseded")
	// Historie mit und ohne ersetzte Messpunkte
	if n := len(c.do("GET", base+"/readings", nil).body["items"].([]any)); n != 2 {
		t.Fatalf("valid history: %d", n)
	}
	if n := len(c.do("GET", base+"/readings?include_superseded=true", nil).body["items"].([]any)); n != 3 {
		t.Fatalf("full history: %d", n)
	}
	// Audit enthält Bestätigung mit Begründung
	var n int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit.event WHERE action='odometer.reading_recorded' AND reason IS NOT NULL`).Scan(&n)
	if n != 1 {
		t.Fatalf("audit with reason: %d", n)
	}
}

func TestOdometerDistanceAndDelete(t *testing.T) {
	e := newEnv(t)
	c := e.adminClient()
	vid := c.do("POST", "/vehicles", vehiclePayload("Golf")).body["id"].(string)
	base := "/vehicles/" + vid + "/odometer"
	for _, rd := range []map[string]any{reading("2026-01-01T00:00:00+01:00", 10000), reading("2026-01-20T00:00:00+01:00", 12000), reading("2026-02-10T00:00:00+01:00", 14100)} {
		expect(t, c.do("POST", base+"/readings", rd), 201, "O-7 reading")
	}
	r := c.do("GET", base+"/distance?from=2026-01-01T00:00:00%2B01:00&to=2026-02-01T00:00:00%2B01:00", nil)
	expect(t, r, 200, "distance")
	if r.body["distance"].(map[string]any)["canonical"].(float64) != 3_200_000 || r.body["display"].(map[string]any)["value"].(float64) != 3200 {
		t.Fatalf("O-7: %s", r.raw)
	}
	r = c.do("GET", base+"/distance?from=2025-12-01T00:00:00Z&to=2026-01-15T00:00:00Z", nil)
	if r.body["status"] != "unknown" {
		t.Fatalf("O-8: %s", r.raw)
	}
	items := c.do("GET", base+"/readings", nil).body["items"].([]any)
	last := items[0].(map[string]any)
	expect(t, c.do("DELETE", base+"/readings/"+last["id"].(string), nil, "If-Match", `"9"`), 412, "stale delete")
	expect(t, c.do("DELETE", base+"/readings/"+last["id"].(string), nil, "If-Match", `"1"`), 204, "delete manual")
	// Tachotausch O-5 über die API
	seg := map[string]any{"started_at": "2026-06-01T00:00:00+02:00", "time_zone": "Europe/Berlin", "reason": "replacement",
		"old_final_value": map[string]any{"value": 18000, "unit": "km"}, "start_meter_value": map[string]any{"value": 5000, "unit": "km"}}
	r = c.do("POST", base+"/segments", seg)
	expect(t, r, 201, "segment")
	if r.body["offset"].(map[string]any)["canonical"].(float64) != 13_000_000 {
		t.Fatalf("offset: %s", r.raw)
	}
	expect(t, c.do("POST", base+"/readings", reading("2026-06-10T12:00:00+02:00", 5200)), 201, "reading in new segment (no P1)")
	cur := c.do("GET", base+"/current", nil)
	if cur.body["total"].(map[string]any)["canonical"].(float64) != 18_200_000 || cur.body["meter_value"].(map[string]any)["canonical"].(float64) != 5_200_000 {
		t.Fatalf("O-5 current: %s", cur.raw)
	}
}

// I-2 / ID-01: Rechte werden am geladenen Objekt geprüft.
func TestAuthorizationAcrossVehicles(t *testing.T) {
	e := newEnv(t)
	a := e.adminClient()
	vA := a.do("POST", "/vehicles", vehiclePayload("A")).body["id"].(string)
	rA := a.do("POST", "/vehicles/"+vA+"/odometer/readings", reading("2026-09-01T10:00:00+02:00", 1000)).body["id"].(string)
	b, bID := e.extraUser("bob@example.org")
	vB := b.do("POST", "/vehicles", vehiclePayload("B")).body["id"].(string)
	// Bob ist Bearbeiter an keinem Fahrzeug von A: alles 404, auch über den Pfad seines eigenen Fahrzeugs.
	expect(t, b.do("GET", "/vehicles/"+vA, nil), 404, "foreign vehicle")
	expect(t, b.do("GET", "/vehicles/"+vB+"/odometer/readings/"+rA, nil), 404, "foreign reading via own vehicle path")
	expect(t, b.do("POST", "/vehicles/"+vB+"/odometer/readings/"+rA+"/corrections",
		map[string]any{"value": map[string]any{"value": 1, "unit": "km"}, "reason": "x"}, "If-Match", `"1"`), 404, "I-2 correct foreign reading")
	// Leser darf lesen, aber nicht schreiben.
	if err := identity.AddMembership(context.Background(), e.pool, uuid.MustParse(vA), bID, identity.RoleViewer); err != nil {
		t.Fatal(err)
	}
	expect(t, b.do("GET", "/vehicles/"+vA+"/odometer/readings/"+rA, nil), 200, "viewer reads")
	expect(t, b.do("POST", "/vehicles/"+vA+"/odometer/readings", reading("2026-09-02T10:00:00+02:00", 1100)), 403, "viewer writes")
	expect(t, b.do("DELETE", "/vehicles/"+vA, nil, "If-Match", `"1"`), 403, "viewer deletes vehicle")
	if n := len(b.do("GET", "/vehicles", nil).body["items"].([]any)); n != 2 {
		t.Fatalf("bob sees %d vehicles", n)
	}
}

func TestNotImplementedIs501(t *testing.T) {
	e := newEnv(t)
	c := e.adminClient()
	r := c.do("GET", "/me/api-tokens", nil)
	expect(t, r, 501, "not implemented")
	if r.header.Get("Content-Type") != "application/problem+json" {
		t.Fatalf("content type %s", r.header.Get("Content-Type"))
	}
}

// Die öffentlichen Pfade müssen genau den Operationen mit `security: []` entsprechen.
func TestPublicPathsMatchSpec(t *testing.T) {
	raw, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]struct {
			Security *[]map[string][]string `yaml:"security"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	public := map[string]bool{}
	for p, ops := range spec.Paths {
		for _, op := range ops {
			if op.Security != nil && len(*op.Security) == 0 {
				public[p] = true
			}
		}
	}
	for p := range public {
		if !publicPaths[p] {
			t.Errorf("spec public, server not: %s", p)
		}
	}
	for p := range publicPaths {
		if !public[p] {
			t.Errorf("server public, spec not: %s", p)
		}
	}
}
