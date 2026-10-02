package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAICompatible spricht jede Chat-Completions-Schnittstelle dieses Formats an,
// z. B. Ollama (http://ollama:11434/v1), llama.cpp oder vLLM (ADR-024).
type OpenAICompatible struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

func NewOpenAICompatible(baseURL, apiKey, model string, timeout time.Duration) *OpenAICompatible {
	return &OpenAICompatible{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, model: model, http: &http.Client{Timeout: timeout}}
}

type oaMessage struct {
	Role       string       `json:"role"`
	Content    *string      `json:"content"`
	ToolCalls  []oaToolCall `json:"tool_calls,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
}

type oaToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func sp(s string) *string { return &s }

func (o *OpenAICompatible) Run(ctx context.Context, system string, history []Turn, tools []ToolDef, exec Executor) (Result, error) {
	var res Result
	msgs := []oaMessage{{Role: "system", Content: sp(system)}}
	for _, t := range history {
		msgs = append(msgs, oaMessage{Role: t.Role, Content: sp(t.Text)})
	}
	var defs []map[string]any
	for _, t := range tools {
		defs = append(defs, map[string]any{"type": "function", "function": map[string]any{"name": t.Name, "description": t.Description,
			"parameters": map[string]any{"type": "object", "properties": t.Properties, "required": t.Required}}})
	}
	for i := 0; i < maxIterations; i++ {
		body, _ := json.Marshal(map[string]any{"model": o.model, "messages": msgs, "tools": defs, "tool_choice": "auto"})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return res, err
		}
		req.Header.Set("Content-Type", "application/json")
		if o.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+o.apiKey)
		}
		resp, err := o.http.Do(req)
		if err != nil {
			return res, err
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			return res, fmt.Errorf("chat provider: HTTP %d", resp.StatusCode)
		}
		var out struct {
			Choices []struct {
				Message      oaMessage `json:"message"`
				FinishReason string    `json:"finish_reason"`
			} `json:"choices"`
			Usage struct {
				PromptTokens     int64 `json:"prompt_tokens"`
				CompletionTokens int64 `json:"completion_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(raw, &out); err != nil || len(out.Choices) == 0 {
			return res, fmt.Errorf("chat provider: invalid response")
		}
		res.InputTokens += out.Usage.PromptTokens
		res.OutputTokens += out.Usage.CompletionTokens
		m := out.Choices[0].Message
		if out.Choices[0].FinishReason == "content_filter" {
			res.Refused = true
			return res, nil
		}
		if len(m.ToolCalls) == 0 {
			if m.Content != nil {
				res.Text = strings.TrimSpace(*m.Content)
			}
			return res, nil
		}
		m.Role = "assistant"
		msgs = append(msgs, m)
		calls := make([]ToolCall, 0, len(m.ToolCalls))
		for _, tc := range m.ToolCalls {
			args := tc.Function.Arguments
			if args == "" {
				args = "{}"
			}
			calls = append(calls, ToolCall{ID: tc.ID, Name: tc.Function.Name, Input: json.RawMessage(args)})
			res.Tools = append(res.Tools, tc.Function.Name)
		}
		results := exec(ctx, calls)
		for i, c := range calls {
			msgs = append(msgs, oaMessage{Role: "tool", ToolCallID: c.ID, Content: sp(results[i].Content)})
		}
	}
	res.Text = "Ich konnte die Anfrage nicht in wenigen Schritten beantworten. Bitte formuliere sie genauer."
	return res, nil
}
