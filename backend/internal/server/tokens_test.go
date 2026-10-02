package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sh1su/vector/backend/internal/identity"
)

func TestSetupStatusAndTokenlessSetup(t *testing.T) {
	e := newEnv(t)
	anon := e.client()
	r := anon.do("GET", "/auth/setup", nil)
	expect(t, r, 200, "setup status")
	if r.body["setup_required"] != true || r.body["token_required"] != true {
		t.Fatalf("status: %s", r.raw)
	}
	// Ohne konfiguriertes Token: Ersteinrichtung offen, aber nur solange kein Konto existiert.
	ids := identity.NewService(e.pool, "", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := ids.Setup(context.Background(), "", "first@example.org", "Erste", pw); err != nil {
		t.Fatalf("tokenless setup: %v", err)
	}
	if _, err := ids.Setup(context.Background(), "", "second@example.org", "Zweite", pw); err == nil || !strings.Contains(err.Error(), "bereits eingerichtet") {
		t.Fatalf("second setup must fail: %v", err)
	}
	r = anon.do("GET", "/auth/setup", nil)
	if r.body["setup_required"] != false {
		t.Fatalf("status after setup: %s", r.raw)
	}
}

func TestApiTokensScopesAndAudit(t *testing.T) {
	e := newEnv(t)
	c := e.adminClient()
	vA := c.do("POST", "/vehicles", vehiclePayload("A")).body["id"].(string)
	vB := c.do("POST", "/vehicles", vehiclePayload("B")).body["id"].(string)
	expect(t, c.do("POST", "/vehicles/"+vA+"/odometer/readings", reading("2026-09-01T10:00:00+02:00", 1000)), 201, "reading")
	exp := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	r := c.do("POST", "/me/api-tokens", map[string]any{"name": "Grafana", "scopes": []string{"vehicles:read"}, "expires_at": exp, "vehicle_ids": []string{vA}})
	expect(t, r, 201, "create token")
	secret := r.body["token"].(string)
	if !strings.HasPrefix(secret, "vct_") {
		t.Fatalf("token: %s", r.raw)
	}
	tokenID := r.body["id"].(string)
	bearer := func(method, path string, body any) resp {
		anon := e.client()
		return anon.do(method, path, body, "Authorization", "Bearer "+secret)
	}
	// Lesen erlaubt, nur Fahrzeug A sichtbar
	r = bearer("GET", "/vehicles", nil)
	expect(t, r, 200, "list via token")
	if n := len(r.body["items"].([]any)); n != 1 {
		t.Fatalf("token sees %d vehicles", n)
	}
	expect(t, bearer("GET", "/vehicles/"+vA+"/odometer/current", nil), 200, "read A")
	expect(t, bearer("GET", "/vehicles/"+vB, nil), 404, "B not visible")
	// Schreiben ohne Scope verboten, auch ohne CSRF-Header kein 403 wegen CSRF, sondern wegen Scope
	r = bearer("POST", "/vehicles/"+vA+"/odometer/readings", reading("2026-09-02T10:00:00+02:00", 1100))
	expect(t, r, 403, "write without scope")
	if !strings.Contains(string(r.raw), "entries:write") {
		t.Fatalf("scope message: %s", r.raw)
	}
	// Token kann keine Tokens anlegen
	expect(t, bearer("POST", "/me/api-tokens", map[string]any{"name": "x", "scopes": []string{"vehicles:read"}, "expires_at": exp}), 403, "token creates token")
	// Änderungshistorie
	r = bearer("GET", "/vehicles/"+vA+"/audit-events", nil)
	expect(t, r, 200, "audit")
	if !strings.Contains(string(r.raw), "odometer.reading_recorded") {
		t.Fatalf("audit: %s", r.raw)
	}
	// Schreib-Token: Herkunft api
	r = c.do("POST", "/me/api-tokens", map[string]any{"name": "Import", "scopes": []string{"vehicles:read", "entries:write"}, "expires_at": exp})
	w := r.body["token"].(string)
	anon := e.client()
	r = anon.do("POST", "/vehicles/"+vB+"/odometer/readings", reading("2026-09-02T10:00:00+02:00", 500), "Authorization", "Bearer "+w)
	expect(t, r, 201, "write with scope")
	if r.body["origin"] != "api" {
		t.Fatalf("origin: %s", r.raw)
	}
	// Liste, Widerruf
	if n := len(c.do("GET", "/me/api-tokens", nil).body["items"].([]any)); n != 2 {
		t.Fatalf("tokens: %d", n)
	}
	expect(t, c.do("DELETE", "/me/api-tokens/"+tokenID, nil), 204, "revoke")
	expect(t, bearer("GET", "/vehicles", nil), 401, "revoked token")
	expect(t, e.client().do("GET", "/vehicles", nil, "Authorization", "Bearer vct_falsch"), 401, "unknown token")
	// Spezifikation abrufbar
	res, err := http.Get(e.srv.URL + "/api/v1/openapi.json")
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("openapi.json: %v %v", err, res.StatusCode)
	}
	res.Body.Close()
}
