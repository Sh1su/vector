package assistant

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Anthropic ist der Adapter für Claude über das offizielle Go-SDK (ADR-024).
type Anthropic struct {
	client    anthropic.Client
	model     string
	fallbacks bool
}

// NewAnthropic erzeugt den Adapter. baseURL ist optional (z. B. Proxy).
// fallbacks aktiviert den serverseitigen Ausweichpfad bei Ablehnungen.
func NewAnthropic(apiKey, baseURL, model string, fallbacks bool, timeout time.Duration) *Anthropic {
	opts := []option.RequestOption{option.WithAPIKey(apiKey), option.WithRequestTimeout(timeout), option.WithMaxRetries(2)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	return &Anthropic{client: anthropic.NewClient(opts...), model: model, fallbacks: fallbacks}
}

func (a *Anthropic) Run(ctx context.Context, system string, history []Turn, tools []ToolDef, exec Executor) (Result, error) {
	var res Result
	msgs := make([]anthropic.BetaMessageParam, 0, len(history))
	for _, t := range history {
		block := anthropic.NewBetaTextBlock(t.Text)
		if t.Role == "assistant" {
			msgs = append(msgs, anthropic.BetaMessageParam{Role: anthropic.BetaMessageParamRoleAssistant, Content: []anthropic.BetaContentBlockParamUnion{block}})
		} else {
			msgs = append(msgs, anthropic.NewBetaUserMessage(block))
		}
	}
	defs := make([]anthropic.BetaToolUnionParam, 0, len(tools))
	for _, t := range tools {
		tp := anthropic.BetaToolParam{Name: t.Name, Description: anthropic.String(t.Description),
			InputSchema: anthropic.BetaToolInputSchemaParam{Properties: t.Properties, Required: t.Required}}
		defs = append(defs, anthropic.BetaToolUnionParam{OfTool: &tp})
	}
	params := anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(a.model),
		MaxTokens: 16000,
		System:    []anthropic.BetaTextBlockParam{{Text: system}},
		Tools:     defs,
	}
	if a.fallbacks {
		params.Fallbacks = anthropic.BetaFallbacksParamOfDefault()
		params.Betas = []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01}
	}
	for i := 0; i < maxIterations; i++ {
		params.Messages = msgs
		resp, err := a.client.Beta.Messages.New(ctx, params)
		if err != nil {
			return res, err
		}
		res.InputTokens += resp.Usage.InputTokens
		res.OutputTokens += resp.Usage.OutputTokens
		if resp.StopReason == anthropic.BetaStopReasonRefusal {
			res.Refused = true
			return res, nil
		}
		msgs = append(msgs, resp.ToParam())
		var text []string
		var calls []ToolCall
		for _, block := range resp.Content {
			switch b := block.AsAny().(type) {
			case anthropic.BetaTextBlock:
				text = append(text, b.Text)
			case anthropic.BetaToolUseBlock:
				calls = append(calls, ToolCall{ID: b.ID, Name: b.Name, Input: json.RawMessage(b.JSON.Input.Raw())})
				res.Tools = append(res.Tools, b.Name)
			}
		}
		if resp.StopReason != anthropic.BetaStopReasonToolUse || len(calls) == 0 {
			res.Text = strings.TrimSpace(strings.Join(text, "\n\n"))
			return res, nil
		}
		results := exec(ctx, calls)
		blocks := make([]anthropic.BetaContentBlockParamUnion, 0, len(calls))
		for i, c := range calls {
			blocks = append(blocks, anthropic.NewBetaToolResultBlock(c.ID, results[i].Content, results[i].IsError))
		}
		msgs = append(msgs, anthropic.NewBetaUserMessage(blocks...))
	}
	res.Text = "Ich konnte die Anfrage nicht in wenigen Schritten beantworten. Bitte formuliere sie genauer."
	return res, nil
}
