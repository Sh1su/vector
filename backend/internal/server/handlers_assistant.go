package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/sh1su/vector/backend/internal/api"
	"github.com/sh1su/vector/backend/internal/assistant"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

// Assistent (ADR-024, ADR-026): ohne Konfiguration gibt es die Endpunkte nicht (404).
func (s *Server) assistantActor(ctx context.Context) (kernel.Actor, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return a, err
	}
	if !s.d.Assistant.Enabled() {
		return a, problem.NotFound()
	}
	if a.Restricted() {
		return a, problem.Forbidden("Der Chat-Assistent steht nur angemeldeten Sitzungen zur Verfügung; API-Tokens nutzen den MCP-Server.")
	}
	return a, nil
}

func statusBody(v assistant.StatusView) api.AssistantStatus {
	out := api.AssistantStatus{Enabled: v.Enabled, UserEnabled: v.UserEnabled, ConsentRequired: v.ConsentRequired}
	if v.Enabled {
		out.ChatProvider = nullable.NewNullableWithValue(struct {
			External bool   `json:"external"`
			Name     string `json:"name"`
		}{External: true, Name: v.ProviderName})
		out.Model = nullable.NewNullableWithValue(v.Model)
		ws := v.WebSearch
		out.WebSearch = &ws
	}
	if v.ConsentGivenAt != nil {
		out.ConsentGivenAt = nullable.NewNullableWithValue(*v.ConsentGivenAt)
	}
	mcp := apiPrefix + mcpPath
	out.McpUrl = &mcp
	return out
}

func (s *Server) GetAssistantStatus(ctx context.Context, _ api.GetAssistantStatusRequestObject) (api.GetAssistantStatusResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Assistant.Status(ctx, a)
	if err != nil {
		return nil, err
	}
	// Auch ohne Chat-Anbieter: Status zeigt die MCP-Adresse (enabled=false).
	return api.GetAssistantStatus200JSONResponse{Body: statusBody(v)}, nil
}

func (s *Server) GiveAssistantConsent(ctx context.Context, req api.GiveAssistantConsentRequestObject) (api.GiveAssistantConsentResponseObject, error) {
	a, err := s.assistantActor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Assistant.GiveConsent(ctx, a, req.Body.AcceptExternalProvider, req.Body.ProviderName)
	if err != nil {
		return nil, err
	}
	return api.GiveAssistantConsent200JSONResponse{Body: statusBody(v)}, nil
}

func (s *Server) RevokeAssistantConsent(ctx context.Context, _ api.RevokeAssistantConsentRequestObject) (api.RevokeAssistantConsentResponseObject, error) {
	a, err := s.assistantActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.Assistant.RevokeConsent(ctx, a); err != nil {
		return nil, err
	}
	return api.RevokeAssistantConsent204Response{}, nil
}

func conversationBody(v assistant.ConversationView) api.Conversation {
	id, created := v.ID, v.CreatedAt
	out := api.Conversation{Id: &id, CreatedAt: &created}
	if v.VehicleID != nil {
		out.VehicleId = nullable.NewNullableWithValue(*v.VehicleID)
	}
	if v.Title != nil {
		out.Title = nullable.NewNullableWithValue(*v.Title)
	}
	return out
}

func (s *Server) ListConversations(ctx context.Context, _ api.ListConversationsRequestObject) (api.ListConversationsResponseObject, error) {
	a, err := s.assistantActor(ctx)
	if err != nil {
		return nil, err
	}
	list, err := s.d.Assistant.Conversations(ctx, a)
	if err != nil {
		return nil, err
	}
	out := api.ConversationPage{Items: []api.Conversation{}}
	for _, c := range list {
		out.Items = append(out.Items, conversationBody(c))
	}
	return api.ListConversations200JSONResponse(out), nil
}

func (s *Server) CreateConversation(ctx context.Context, req api.CreateConversationRequestObject) (api.CreateConversationResponseObject, error) {
	a, err := s.assistantActor(ctx)
	if err != nil {
		return nil, err
	}
	var vid *uuid.UUID
	if req.Body.VehicleId.IsSpecified() && !req.Body.VehicleId.IsNull() {
		v, _ := req.Body.VehicleId.Get()
		vid = &v
	}
	var title *string
	if req.Body.Title.IsSpecified() && !req.Body.Title.IsNull() {
		t, _ := req.Body.Title.Get()
		title = &t
	}
	v, created, err := s.d.Assistant.CreateConversation(ctx, a, (*uuid.UUID)(req.Body.Id), vid, title)
	if err != nil {
		return nil, err
	}
	if !created {
		return api.CreateConversation200JSONResponse(conversationBody(v)), nil
	}
	return api.CreateConversation201JSONResponse{Body: conversationBody(v),
		Headers: api.CreateConversation201ResponseHeaders{Location: loc("/assistant/conversations/" + v.ID.String())}}, nil
}

func (s *Server) DeleteConversation(ctx context.Context, req api.DeleteConversationRequestObject) (api.DeleteConversationResponseObject, error) {
	a, err := s.assistantActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.Assistant.DeleteConversation(ctx, a, uuid.UUID(req.ConversationId)); err != nil {
		return nil, err
	}
	return api.DeleteConversation204Response{}, nil
}

func proposalBody(p assistant.ProposalView) api.Proposal {
	out := api.Proposal{Id: p.ID, Operation: p.Operation, VehicleId: p.VehicleID, Body: p.Body, Status: api.ProposalStatus(p.Status), ExpiresAt: p.ExpiresAt}
	sum := p.Summary
	out.Summary = &sum
	var an []api.Anomaly
	_ = convert(p.Anomalies, &an)
	out.Anomalies = &an
	if p.ResultID != nil {
		out.ResultId = nullable.NewNullableWithValue(openapi_types.UUID(*p.ResultID))
	}
	return out
}

func messageBody(m assistant.MessageView) api.AssistantMessage {
	id, created := m.ID, m.CreatedAt
	role := api.AssistantMessageRole(m.Role)
	props := make([]api.Proposal, 0, len(m.Proposals))
	for _, p := range m.Proposals {
		props = append(props, proposalBody(p))
	}
	return api.AssistantMessage{Id: &id, CreatedAt: &created, Role: &role, Text: m.Text, Proposals: &props}
}

func (s *Server) ListAssistantMessages(ctx context.Context, req api.ListAssistantMessagesRequestObject) (api.ListAssistantMessagesResponseObject, error) {
	a, err := s.assistantActor(ctx)
	if err != nil {
		return nil, err
	}
	list, err := s.d.Assistant.Messages(ctx, a, uuid.UUID(req.ConversationId))
	if err != nil {
		return nil, err
	}
	out := api.AssistantMessagePage{Items: []api.AssistantMessage{}}
	for _, m := range list {
		out.Items = append(out.Items, messageBody(m))
	}
	return api.ListAssistantMessages200JSONResponse(out), nil
}

// sseResponse streamt Zwischenschritte (status, proposal) und zuletzt die Antwort (message) oder einen Fehler (error).
type sseResponse struct {
	run func(emit func(event string, data any)) (any, error)
}

func (r sseResponse) VisitSendAssistantMessageResponse(w http.ResponseWriter) error {
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Now().Add(5 * time.Minute)) // Modell und Websuche brauchen länger als üblich
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	emit := func(event string, data any) {
		b, _ := json.Marshal(data)
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
		_ = rc.Flush()
	}
	emit("status", map[string]string{"text": "Denke nach …"})
	res, err := r.run(emit)
	if err != nil {
		pe, ok := err.(*problem.Error)
		if !ok {
			pe = &problem.Error{Status: 500, Type: "https://vectra.app/problems/internal", Title: "Interner Fehler"}
		}
		emit("error", pe)
		return nil
	}
	emit("message", res)
	return nil
}

func (s *Server) SendAssistantMessage(ctx context.Context, req api.SendAssistantMessageRequestObject) (api.SendAssistantMessageResponseObject, error) {
	a, err := s.assistantActor(ctx)
	if err != nil {
		return nil, err
	}
	conv := uuid.UUID(req.ConversationId)
	// Vorprüfungen als normale Fehlerantwort, damit der Client sie ohne Stream behandeln kann.
	st, err := s.d.Assistant.Status(ctx, a)
	if err != nil {
		return nil, err
	}
	if st.ConsentRequired {
		return nil, problem.Conflict("Bitte zuerst der Übertragung an " + st.ProviderName + " zustimmen.")
	}
	if _, err := s.d.Assistant.Messages(ctx, a, conv); err != nil {
		return nil, err
	}
	text := req.Body.Text
	return sseResponse{run: func(emit func(string, any)) (any, error) {
		m, err := s.d.Assistant.Send(ctx, a, conv, text, func(event string, data any) {
			if p, ok := data.(assistant.ProposalView); ok {
				emit(event, proposalBody(p))
				return
			}
			if t, ok := data.(string); ok {
				emit(event, map[string]string{"text": t})
				return
			}
			emit(event, data)
		})
		if err != nil {
			s.d.Log.Warn("assistant", "err", err)
			return nil, err
		}
		return messageBody(m), nil
	}}, nil
}

func (s *Server) ConfirmProposal(ctx context.Context, req api.ConfirmProposalRequestObject) (api.ConfirmProposalResponseObject, error) {
	a, err := s.assistantActor(ctx)
	if err != nil {
		return nil, err
	}
	var conf *assistant.Confirmation
	var edited map[string]any
	if req.Body != nil {
		if req.Body.Body != nil {
			edited = *req.Body.Body
		}
		if req.Body.ConfirmAnomalies != nil && len(*req.Body.ConfirmAnomalies) > 0 {
			reason := str(req.Body.AnomalyReason)
			if reason == "" {
				return nil, problem.Validation(problem.FieldError{Pointer: "/anomaly_reason", Code: "required"})
			}
			conf = &assistant.Confirmation{Codes: *req.Body.ConfirmAnomalies, Reason: reason}
		}
	}
	p, err := s.d.Assistant.Confirm(ctx, a, uuid.UUID(req.ProposalId), edited, conf)
	if err != nil {
		return nil, err
	}
	return api.ConfirmProposal200JSONResponse{Body: proposalBody(p)}, nil
}

func (s *Server) RejectProposal(ctx context.Context, req api.RejectProposalRequestObject) (api.RejectProposalResponseObject, error) {
	a, err := s.assistantActor(ctx)
	if err != nil {
		return nil, err
	}
	p, err := s.d.Assistant.Reject(ctx, a, uuid.UUID(req.ProposalId))
	if err != nil {
		return nil, err
	}
	return api.RejectProposal200JSONResponse{Body: proposalBody(p)}, nil
}
