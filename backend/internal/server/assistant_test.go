package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sh1su/vector/backend/internal/assistant"
)

// fakeChat spielt eine feste Folge von Werkzeugaufrufen ab (ohne Netz).
type fakeChat struct {
	calls  []assistant.ToolCall
	result []assistant.ToolResult
}

func (f *fakeChat) Run(ctx context.Context, system string, history []assistant.Turn, tools []assistant.ToolDef, exec assistant.Executor) (assistant.Result, error) {
	res := assistant.Result{Text: "Erledigt."}
	if len(f.calls) > 0 {
		f.result = exec(ctx, f.calls)
		for _, c := range f.calls {
			res.Tools = append(res.Tools, c.Name)
		}
	}
	return res, nil
}

func TestAssistantProposalFlow(t *testing.T) {
	e := newEnv(t)
	c := e.adminClient()
	p := vehiclePayload("Sprinter")
	p["oil_capacity"] = map[string]any{"value": 4.3, "unit": "l"}
	vid := c.do("POST", "/vehicles", p).body["id"].(string)
	expect(t, c.do("POST", "/vehicles/"+vid+"/odometer/readings", reading("2026-09-01T10:00:00+02:00", 143000)), 201, "reading")

	// Standard aus: Status antwortet, alles andere 404
	r := c.do("GET", "/assistant/status", nil)
	expect(t, r, 200, "status")
	if r.body["enabled"] != false || r.body["chat_provider"].(map[string]any)["name"] != "Testanbieter" {
		t.Fatalf("status off: %s", r.raw)
	}
	expect(t, c.do("GET", "/assistant/conversations", nil), 404, "disabled")
	expect(t, c.do("PATCH", "/admin/settings", map[string]any{"assistant_enabled": true}), 200, "enable")
	r = c.do("GET", "/assistant/status", nil)
	if r.body["enabled"] != true || r.body["consent_required"] != true || r.body["user_enabled"] != false {
		t.Fatalf("status on: %s", r.raw)
	}
	expect(t, c.do("POST", "/assistant/conversations", map[string]any{"vehicle_id": vid}), 403, "no consent yet")
	expect(t, c.do("POST", "/assistant/consent", map[string]any{"accept_external_provider": true, "provider_name": "Anderer"}), 422, "wrong provider")
	expect(t, c.do("POST", "/assistant/consent", map[string]any{"accept_external_provider": true, "provider_name": "Testanbieter"}), 200, "consent")
	r = c.do("POST", "/assistant/conversations", map[string]any{"vehicle_id": vid})
	expect(t, r, 201, "conversation")
	conv := r.body["id"].(string)

	// „Ich habe bei 143.520 km 0,7 Liter Öl nachgefüllt.“ (Auftrag 6.13)
	in, _ := json.Marshal(map[string]any{"vehicle_id": vid, "kind": "top_up", "occurred_at": "2026-09-30T08:00:00Z", "odometer": map[string]any{"value": 143520, "unit": "km"},
		"oil_added": map[string]any{"value": 0.7, "unit": "l"}})
	e.chat.calls = []assistant.ToolCall{{ID: "t1", Name: "get_current_odometer", Input: json.RawMessage(`{"vehicle_id":"` + vid + `"}`)},
		{ID: "t2", Name: "create_oil_entry", Input: in}}
	r = c.do("POST", "/assistant/conversations/"+conv+"/messages", map[string]any{"text": "Ich habe bei 143.520 km 0,7 Liter Öl nachgefüllt."})
	expect(t, r, 200, "send")
	raw := string(r.raw)
	if !strings.Contains(raw, "event: tool") || !strings.Contains(raw, "event: message") || !strings.Contains(raw, `"operation":"createOilEntry"`) {
		t.Fatalf("sse: %s", raw)
	}
	if !strings.Contains(e.chat.result[0].Content, "143000") || e.chat.result[1].IsError {
		t.Fatalf("tool results: %+v", e.chat.result)
	}
	// Vorschlag hat nichts gespeichert (Probelauf)
	if n := len(c.do("GET", "/vehicles/"+vid+"/oil-entries", nil).body["items"].([]any)); n != 0 {
		t.Fatalf("dry run wrote %d entries", n)
	}
	msgs := c.do("GET", "/assistant/conversations/"+conv+"/messages", nil).body["items"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("messages: %v", msgs)
	}
	prop := msgs[1].(map[string]any)["proposals"].([]any)[0].(map[string]any)
	pid := prop["id"].(string)
	r = c.do("POST", "/assistant/proposals/"+pid+"/confirm", map[string]any{})
	expect(t, r, 200, "confirm")
	if r.body["status"] != "confirmed" || r.body["result_id"] != pid {
		t.Fatalf("confirm: %s", r.raw)
	}
	expect(t, c.do("POST", "/assistant/proposals/"+pid+"/confirm", map[string]any{}), 200, "confirm idempotent")
	items := c.do("GET", "/vehicles/"+vid+"/oil-entries", nil).body["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["origin"] != "assistant" {
		t.Fatalf("entry: %v", items)
	}
	var kind string
	_ = e.pool.QueryRow(context.Background(), `SELECT actor_kind FROM audit.event WHERE action = 'oil.entry_recorded'`).Scan(&kind)
	if kind != "assistant" {
		t.Fatalf("audit actor: %s", kind)
	}
	expect(t, c.do("POST", "/assistant/proposals/"+pid+"/reject", nil), 200, "reject after confirm keeps status")

	// Befund im Vorschlag (OIL_CAPACITY): Bestätigung nur durch den Nutzer
	in, _ = json.Marshal(map[string]any{"vehicle_id": vid, "kind": "top_up", "occurred_at": "2026-09-30T11:00:00Z", "odometer": map[string]any{"value": 143600, "unit": "km"},
		"oil_added": map[string]any{"value": 5, "unit": "l"}})
	e.chat.calls = []assistant.ToolCall{{ID: "t3", Name: "create_oil_entry", Input: in}}
	r = c.do("POST", "/assistant/conversations/"+conv+"/messages", map[string]any{"text": "5 Liter nachgefüllt"})
	if !strings.Contains(string(r.raw), "OIL_CAPACITY") {
		t.Fatalf("anomaly in proposal: %s", r.raw)
	}
	msgs = c.do("GET", "/assistant/conversations/"+conv+"/messages", nil).body["items"].([]any)
	pid2 := msgs[3].(map[string]any)["proposals"].([]any)[0].(map[string]any)["id"].(string)
	expect(t, c.do("POST", "/assistant/proposals/"+pid2+"/confirm", map[string]any{}), 422, "needs user confirmation")
	expect(t, c.do("POST", "/assistant/proposals/"+pid2+"/confirm", map[string]any{"confirm_anomalies": []string{"OIL_CAPACITY"}, "anomaly_reason": "Ölwechsel ohne Filter"}), 200, "confirmed")

	// Fremdes Fahrzeug: Werkzeuge sehen nur eigene Fahrzeuge
	b, _ := e.extraUser("bob@example.org")
	expect(t, b.do("POST", "/assistant/consent", map[string]any{"accept_external_provider": true, "provider_name": "Testanbieter"}), 200, "bob consent")
	bc := b.do("POST", "/assistant/conversations", map[string]any{}).body["id"].(string)
	e.chat.calls = []assistant.ToolCall{{ID: "t4", Name: "get_current_odometer", Input: json.RawMessage(`{"vehicle_id":"` + vid + `"}`)}}
	b.do("POST", "/assistant/conversations/"+bc+"/messages", map[string]any{"text": "Stand?"})
	if !e.chat.result[0].IsError || strings.Contains(e.chat.result[0].Content, "143") {
		t.Fatalf("foreign vehicle leaked: %+v", e.chat.result)
	}
	expect(t, b.do("POST", "/assistant/proposals/"+pid2+"/reject", nil), 404, "foreign proposal")
	expect(t, b.do("GET", "/assistant/conversations/"+conv+"/messages", nil), 404, "foreign conversation")
	expect(t, c.do("DELETE", "/assistant/conversations/"+conv, nil), 204, "delete conversation")
}
