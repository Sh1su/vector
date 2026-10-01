package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sh1su/vector/backend/internal/assistant"
)

// mcpCall sendet eine JSON-RPC-Nachricht an /mcp mit Bearer-Token.
func mcpCall(t *testing.T, e *env, token, method string, params any) (int, map[string]any) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	req, _ := http.NewRequest("POST", e.srv.URL+"/api/v1/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func toolText(t *testing.T, out map[string]any) (string, bool) {
	t.Helper()
	r, ok := out["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result: %v", out)
	}
	c := r["content"].([]any)[0].(map[string]any)
	return c["text"].(string), r["isError"] == true
}

func TestMCPWithApiToken(t *testing.T) {
	e := newEnv(t)
	c := e.adminClient()
	vid := c.do("POST", "/vehicles", vehiclePayload("Octavia")).body["id"].(string)
	other := c.do("POST", "/vehicles", vehiclePayload("Zweitwagen")).body["id"].(string)
	expires := time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)

	r := c.do("POST", "/me/api-tokens", map[string]any{"name": "Claude Desktop", "scopes": []string{"vehicles:read", "entries:write"}, "expires_at": expires})
	expect(t, r, 201, "token")
	token := r.body["token"].(string)
	if !strings.HasPrefix(token, "vct_") {
		t.Fatalf("token: %s", r.raw)
	}
	if list := c.do("GET", "/me/api-tokens", nil).body["items"].([]any); len(list) != 1 || list[0].(map[string]any)["token"] != nil {
		t.Fatalf("list must not reveal token: %v", list)
	}
	expect(t, c.do("POST", "/me/api-tokens", map[string]any{"name": "x", "scopes": []string{"admin"}, "expires_at": expires}), 422, "admin scope")

	// Ohne/mit falschem Token
	if st, _ := mcpCall(t, e, "", "initialize", map[string]any{}); st != 401 {
		t.Fatalf("no token: %d", st)
	}
	if st, _ := mcpCall(t, e, "vct_falsch", "initialize", map[string]any{}); st != 401 {
		t.Fatalf("bad token: %d", st)
	}
	// Token gilt nur für /mcp, nicht für die übrige API
	req, _ := http.NewRequest("GET", e.srv.URL+"/api/v1/vehicles", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if res, _ := http.DefaultClient.Do(req); res.StatusCode != 401 {
		t.Fatalf("bearer outside mcp: %d", res.StatusCode)
	}

	st, out := mcpCall(t, e, token, "initialize", map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "test"}})
	if st != 200 || out["result"].(map[string]any)["protocolVersion"] != "2025-06-18" {
		t.Fatalf("initialize: %d %v", st, out)
	}
	_, out = mcpCall(t, e, token, "tools/list", nil)
	names := map[string]bool{}
	for _, x := range out["result"].(map[string]any)["tools"].([]any) {
		names[x.(map[string]any)["name"].(string)] = true
	}
	for _, n := range []string{"list_vehicles", "record_odometer", "start_trip", "finish_trip", "create_maintenance_plan", "apply_maintenance_book"} {
		if !names[n] {
			t.Fatalf("tool %s missing: %v", n, names)
		}
	}

	_, out = mcpCall(t, e, token, "tools/call", map[string]any{"name": "list_vehicles", "arguments": map[string]any{}})
	text, isErr := toolText(t, out)
	if isErr || !strings.Contains(text, "Octavia") || !strings.Contains(text, "Zweitwagen") {
		t.Fatalf("list_vehicles: %s", text)
	}
	_, out = mcpCall(t, e, token, "tools/call", map[string]any{"name": "record_odometer", "arguments": map[string]any{"vehicle_id": vid, "value": 52000, "occurred_at": "2026-09-01T08:00:00Z"}})
	if text, isErr := toolText(t, out); isErr || !strings.Contains(text, "52.000") {
		t.Fatalf("record_odometer: %s", text)
	}
	// Kleinerer Stand → Befund P1; Bestätigung nur mit Begründung
	args := map[string]any{"vehicle_id": vid, "value": 51000, "occurred_at": "2026-09-10T08:00:00Z"}
	_, out = mcpCall(t, e, token, "tools/call", map[string]any{"name": "record_odometer", "arguments": args})
	if text, isErr := toolText(t, out); !isErr || !strings.Contains(text, "P1") {
		t.Fatalf("anomaly expected: %s", text)
	}
	args["confirm_anomalies"] = []string{"P1"}
	args["anomaly_reason"] = "Tacho getauscht"
	_, out = mcpCall(t, e, token, "tools/call", map[string]any{"name": "record_odometer", "arguments": args})
	if text, isErr := toolText(t, out); isErr {
		t.Fatalf("confirmed: %s", text)
	}
	// Fahrtenbuch über MCP
	_, out = mcpCall(t, e, token, "tools/call", map[string]any{"name": "start_trip", "arguments": map[string]any{"vehicle_id": other, "start_km": 1000,
		"category": "business", "purpose": "Kundentermin", "started_at": "2026-09-20T08:00:00Z"}})
	if text, isErr := toolText(t, out); isErr {
		t.Fatalf("start_trip: %s", text)
	}
	_, out = mcpCall(t, e, token, "tools/call", map[string]any{"name": "finish_trip", "arguments": map[string]any{"vehicle_id": other, "end_km": 1042, "ended_at": "2026-09-20T09:00:00Z"}})
	if text, isErr := toolText(t, out); isErr || !strings.Contains(text, "42") {
		t.Fatalf("finish_trip: %s", text)
	}
	trips := c.do("GET", "/vehicles/"+other+"/trips", nil).body["items"].([]any)
	if len(trips) != 1 || trips[0].(map[string]any)["status"] != "closed" {
		t.Fatalf("trip: %v", trips)
	}
	// Geschäftsfahrt ohne Zweck → Rückfrage
	_, out = mcpCall(t, e, token, "tools/call", map[string]any{"name": "start_trip", "arguments": map[string]any{"vehicle_id": other, "start_km": 1042, "category": "business"}})
	if text, isErr := toolText(t, out); !isErr || !strings.Contains(text, "Zweck") {
		t.Fatalf("purpose required: %s", text)
	}
	// Wartungsplan aus Recherche
	_, out = mcpCall(t, e, token, "tools/call", map[string]any{"name": "create_maintenance_plan", "arguments": map[string]any{"vehicle_id": vid, "source": "Skoda Serviceheft",
		"anchor_date": "2026-03-01", "anchor_km": 45000, "items": []any{
			map[string]any{"title": "Inspektion", "category": "service", "interval_months": 24, "interval_km": 30000},
			map[string]any{"title": "Bremsflüssigkeit", "category": "brakes", "interval_months": 24}}}})
	if text, isErr := toolText(t, out); isErr || !strings.Contains(text, "2 Wartungen angelegt") {
		t.Fatalf("plan: %s", text)
	}

	// Nur-Lese-Token mit Fahrzeugbeschränkung
	r = c.do("POST", "/me/api-tokens", map[string]any{"name": "Lesen", "scopes": []string{"vehicles:read"}, "vehicle_ids": []string{vid}, "expires_at": expires})
	expect(t, r, 201, "readonly token")
	ro := r.body["token"].(string)
	_, out = mcpCall(t, e, ro, "tools/list", nil)
	for _, x := range out["result"].(map[string]any)["tools"].([]any) {
		if x.(map[string]any)["name"] == "record_odometer" {
			t.Fatal("write tool listed for read-only token")
		}
	}
	_, out = mcpCall(t, e, ro, "tools/call", map[string]any{"name": "record_odometer", "arguments": map[string]any{"vehicle_id": vid, "value": 60000}})
	if _, isErr := toolText(t, out); !isErr {
		t.Fatal("read-only token wrote")
	}
	_, out = mcpCall(t, e, ro, "tools/call", map[string]any{"name": "list_vehicles", "arguments": map[string]any{}})
	if text, _ := toolText(t, out); strings.Contains(text, "Zweitwagen") {
		t.Fatalf("vehicle restriction: %s", text)
	}
	_, out = mcpCall(t, e, ro, "tools/call", map[string]any{"name": "list_trips", "arguments": map[string]any{"vehicle_id": other}})
	if _, isErr := toolText(t, out); !isErr {
		t.Fatal("restricted vehicle readable")
	}

	// Widerruf
	tid := r.body["id"].(string)
	expect(t, c.do("DELETE", "/me/api-tokens/"+tid, nil), 204, "revoke")
	if st, _ := mcpCall(t, e, ro, "ping", nil); st != 401 {
		t.Fatalf("revoked token: %d", st)
	}
	// Sitzung ohne CSRF darf /mcp nicht nutzen
	req, _ = http.NewRequest("POST", e.srv.URL+"/api/v1/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	if res, _ := c.http.Do(req); res.StatusCode != 403 {
		t.Fatalf("csrf: %d", res.StatusCode)
	}
}

// fakeClaude simuliert die Messages API: erst ein Werkzeugaufruf, dann eine Antwort.
type fakeClaude struct {
	mu       sync.Mutex
	requests []map[string]any
	replies  []map[string]any
}

func (f *fakeClaude) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	f.mu.Lock()
	f.requests = append(f.requests, body)
	reply := f.replies[0]
	f.replies = f.replies[1:]
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(reply)
}

func msg(stop string, content ...map[string]any) map[string]any {
	return map[string]any{"id": "msg_1", "type": "message", "role": "assistant", "model": "claude-sonnet-4-5", "content": content,
		"stop_reason": stop, "stop_sequence": nil, "usage": map[string]any{"input_tokens": 100, "output_tokens": 20}}
}

// sse liest alle Ereignisse eines Server-Sent-Events-Stroms.
func sse(t *testing.T, c *client, path string, body any) []struct {
	Event string
	Data  map[string]any
} {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", c.e.srv.URL+"/api/v1"+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", c.csrf)
	res, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("sse: %d %s", res.StatusCode, raw)
	}
	var out []struct {
		Event string
		Data  map[string]any
	}
	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	ev := ""
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			ev = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			var d map[string]any
			_ = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &d)
			out = append(out, struct {
				Event string
				Data  map[string]any
			}{ev, d})
		}
	}
	return out
}

func TestAssistantChatProposals(t *testing.T) {
	fake := &fakeClaude{}
	claude := httptest.NewServer(fake)
	defer claude.Close()
	e := newEnvWith(t, func(d *Deps, pool *pgxpool.Pool) {
		d.Assistant = assistant.NewService(pool, assistant.Config{Provider: "anthropic", APIKey: "test", Model: "claude-sonnet-4-5", BaseURL: claude.URL, WebSearch: true})
	})
	c := e.adminClient()
	vid := c.do("POST", "/vehicles", vehiclePayload("Octavia")).body["id"].(string)

	st := c.do("GET", "/assistant/status", nil)
	expect(t, st, 200, "status")
	if st.body["enabled"] != true || st.body["consent_required"] != true || st.body["model"] != "claude-sonnet-4-5" {
		t.Fatalf("status: %s", st.raw)
	}
	conv := c.do("POST", "/assistant/conversations", map[string]any{"vehicle_id": vid})
	expect(t, conv, 201, "conversation")
	cid := conv.body["id"].(string)
	// Ohne Zustimmung keine Anfrage an den Anbieter (ADR-024)
	expect(t, c.do("POST", "/assistant/conversations/"+cid+"/messages", map[string]any{"text": "Hallo"}), 409, "consent required")
	expect(t, c.do("POST", "/assistant/consent", map[string]any{"accept_external_provider": true, "provider_name": "falsch"}), 422, "wrong provider")
	expect(t, c.do("POST", "/assistant/consent", map[string]any{"accept_external_provider": true, "provider_name": "Anthropic (Claude)"}), 200, "consent")

	fake.replies = []map[string]any{
		msg("tool_use", map[string]any{"type": "text", "text": "Ich trage die Fahrt ein."},
			map[string]any{"type": "tool_use", "id": "toolu_1", "name": "start_trip", "input": map[string]any{"vehicle_id": vid, "start_km": 52000,
				"category": "business", "purpose": "Kundentermin Müller", "started_at": "2026-09-29T07:30:00Z"}}),
		msg("end_turn", map[string]any{"type": "text", "text": "Ich habe die Fahrt vorbereitet – bitte bestätigen."}),
	}
	events := sse(t, c, "/assistant/conversations/"+cid+"/messages", map[string]any{"text": "Ich fahre jetzt los, Kilometerstand 52.000, Kundentermin bei Müller"})
	var final, proposal map[string]any
	for _, ev := range events {
		switch ev.Event {
		case "message":
			final = ev.Data
		case "proposal":
			proposal = ev.Data
		case "error":
			t.Fatalf("error event: %v", ev.Data)
		}
	}
	if final == nil || proposal == nil || !strings.Contains(final["text"].(string), "bestätigen") {
		t.Fatalf("events: %v", events)
	}
	props := final["proposals"].([]any)
	if len(props) != 1 || props[0].(map[string]any)["status"] != "pending" || !strings.Contains(props[0].(map[string]any)["summary"].(string), "52.000") {
		t.Fatalf("proposal: %v", props)
	}
	// Anfrage an das Modell: Werkzeuge, Websuche, System mit Fahrzeugkontext, Modell
	req0 := fake.requests[0]
	if req0["model"] != "claude-sonnet-4-5" || !strings.Contains(mustJSON(req0["system"]), vid) || !strings.Contains(mustJSON(req0["tools"]), "web_search_20250305") {
		t.Fatalf("request: %s", mustJSON(req0))
	}
	// Zweite Anfrage enthält das Werkzeugergebnis „Vorschlag … noch nicht gespeichert“
	if !strings.Contains(mustJSON(fake.requests[1]["messages"]), "Noch nicht gespeichert") {
		t.Fatalf("tool result: %s", mustJSON(fake.requests[1]["messages"]))
	}
	// Noch nichts gespeichert
	if trips := c.do("GET", "/vehicles/"+vid+"/trips", nil).body["items"].([]any); len(trips) != 0 {
		t.Fatalf("written before confirmation: %v", trips)
	}
	pid := proposal["id"].(string)
	r := c.do("POST", "/assistant/proposals/"+pid+"/confirm", map[string]any{})
	expect(t, r, 200, "confirm")
	if r.body["status"] != "confirmed" || r.body["result_id"] == nil {
		t.Fatalf("confirm: %s", r.raw)
	}
	trips := c.do("GET", "/vehicles/"+vid+"/trips", nil).body["items"].([]any)
	if len(trips) != 1 || trips[0].(map[string]any)["purpose"] != "Kundentermin Müller" || trips[0].(map[string]any)["status"] != "open" {
		t.Fatalf("trip: %v", trips)
	}
	expect(t, c.do("POST", "/assistant/proposals/"+pid+"/confirm", map[string]any{}), 200, "confirm twice is idempotent")
	expect(t, c.do("POST", "/assistant/proposals/"+pid+"/reject", nil), 409, "reject confirmed")

	// Verlauf: Vorschlagsstatus fließt in die nächste Anfrage ein
	fake.replies = []map[string]any{
		msg("tool_use", map[string]any{"type": "tool_use", "id": "toolu_2", "name": "record_odometer", "input": map[string]any{"vehicle_id": vid, "value": 40000, "occurred_at": "2026-09-29T08:00:00Z"}}),
		msg("end_turn", map[string]any{"type": "text", "text": "Bitte bestätigen."}),
	}
	events = sse(t, c, "/assistant/conversations/"+cid+"/messages", map[string]any{"text": "Stand 40.000"})
	if !strings.Contains(mustJSON(fake.requests[2]["messages"]), "bestätigt und gespeichert") {
		t.Fatalf("history: %s", mustJSON(fake.requests[2]["messages"]))
	}
	var pid2 string
	for _, ev := range events {
		if ev.Event == "proposal" {
			pid2 = ev.Data["id"].(string)
		}
	}
	// Unplausibel (kleiner als 52.000): 422 mit Befund; Bestätigung mit Begründung
	r = c.do("POST", "/assistant/proposals/"+pid2+"/confirm", map[string]any{})
	expect(t, r, 422, "anomaly")
	if !strings.Contains(string(r.raw), "P1") {
		t.Fatalf("anomaly: %s", r.raw)
	}
	r = c.do("POST", "/assistant/proposals/"+pid2+"/confirm", map[string]any{"confirm_anomalies": []string{"P1", "P3"}, "anomaly_reason": "Tacho getauscht"})
	expect(t, r, 200, "confirm with reason")

	msgs := c.do("GET", "/assistant/conversations/"+cid+"/messages", nil).body["items"].([]any)
	if len(msgs) != 4 {
		t.Fatalf("messages: %v", msgs)
	}
	if list := c.do("GET", "/assistant/conversations", nil).body["items"].([]any); len(list) != 1 || list[0].(map[string]any)["title"] == nil {
		t.Fatalf("conversations: %v", list)
	}
	// Fremde Vorschläge sind unsichtbar
	other, _ := e.extraUser("zweite@example.org")
	expect(t, other.do("POST", "/assistant/proposals/"+pid+"/confirm", map[string]any{}), 404, "foreign proposal")
	expect(t, c.do("DELETE", "/assistant/conversations/"+cid, nil), 204, "delete conversation")
	expect(t, c.do("DELETE", "/assistant/consent", nil), 204, "revoke consent")
}

func TestAssistantDisabledIs404(t *testing.T) {
	e := newEnv(t)
	c := e.adminClient()
	r := c.do("GET", "/assistant/status", nil)
	expect(t, r, 200, "status")
	if r.body["enabled"] != false || r.body["mcp_url"] != "/api/v1/mcp" {
		t.Fatalf("status: %s", r.raw)
	}
	expect(t, c.do("POST", "/assistant/conversations", map[string]any{}), 404, "disabled")
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
