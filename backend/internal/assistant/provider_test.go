package assistant

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func echoExec(_ context.Context, calls []ToolCall) []ToolResult {
	out := make([]ToolResult, len(calls))
	for i, c := range calls {
		out[i] = ToolResult{Content: `{"data":` + string(c.Input) + `}`}
	}
	return out
}

var testTools = []ToolDef{{Name: "get_current_odometer", Description: "x", Properties: map[string]any{"vehicle_id": map[string]any{"type": "string"}}, Required: []string{"vehicle_id"}}}

func TestAnthropicToolLoop(t *testing.T) {
	var bodies []map[string]any
	var beta string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var b map[string]any
		_ = json.Unmarshal(raw, &b)
		bodies = append(bodies, b)
		beta = r.Header.Get("anthropic-beta")
		w.Header().Set("Content-Type", "application/json")
		if len(bodies) == 1 {
			_, _ = io.WriteString(w, `{"id":"m1","type":"message","role":"assistant","model":"claude-opus-5-5","stop_reason":"tool_use",
				"content":[{"type":"tool_use","id":"tu1","name":"get_current_odometer","input":{"vehicle_id":"v1"}}],"usage":{"input_tokens":10,"output_tokens":5}}`)
			return
		}
		_, _ = io.WriteString(w, `{"id":"m2","type":"message","role":"assistant","model":"claude-opus-5-5","stop_reason":"end_turn",
			"content":[{"type":"text","text":"143.520 km"}],"usage":{"input_tokens":20,"output_tokens":7}}`)
	}))
	defer srv.Close()
	a := NewAnthropic("test", srv.URL, "claude-opus-5-5", true, 10*time.Second)
	res, err := a.Run(context.Background(), "sys", []Turn{{Role: "user", Text: "Stand?"}}, testTools, echoExec)
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "143.520 km" || res.InputTokens != 30 || len(res.Tools) != 1 {
		t.Fatalf("result: %+v", res)
	}
	if bodies[0]["fallbacks"] != "default" || !strings.Contains(beta, "server-side-fallback-2026-07-01") {
		t.Fatalf("fallbacks: %v %s", bodies[0]["fallbacks"], beta)
	}
	msgs := bodies[1]["messages"].([]any)
	last := msgs[len(msgs)-1].(map[string]any)["content"].([]any)[0].(map[string]any)
	if last["type"] != "tool_result" || last["tool_use_id"] != "tu1" {
		t.Fatalf("tool result turn: %v", last)
	}
}

func TestAnthropicRefusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"m1","type":"message","role":"assistant","model":"claude-opus-5-5","stop_reason":"refusal","content":[],"usage":{"input_tokens":1,"output_tokens":1}}`)
	}))
	defer srv.Close()
	res, err := NewAnthropic("test", srv.URL, "claude-opus-5-5", false, 10*time.Second).Run(context.Background(), "sys", []Turn{{Role: "user", Text: "x"}}, nil, echoExec)
	if err != nil || !res.Refused {
		t.Fatalf("refusal: %+v %v", res, err)
	}
}

func TestOpenAICompatibleToolLoop(t *testing.T) {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		var b struct {
			Messages []map[string]any `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&b)
		if n == 1 {
			_, _ = io.WriteString(w, `{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":null,"tool_calls":[{"id":"c1","type":"function","function":{"name":"get_current_odometer","arguments":"{\"vehicle_id\":\"v1\"}"}}]}}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`)
			return
		}
		if b.Messages[len(b.Messages)-1]["role"] != "tool" {
			t.Errorf("tool message missing: %v", b.Messages)
		}
		_, _ = io.WriteString(w, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"fertig"}}]}`)
	}))
	defer srv.Close()
	res, err := NewOpenAICompatible(srv.URL, "", "llama", 10*time.Second).Run(context.Background(), "sys", []Turn{{Role: "user", Text: "x"}}, testTools, echoExec)
	if err != nil || res.Text != "fertig" || len(res.Tools) != 1 {
		t.Fatalf("%+v %v", res, err)
	}
}
