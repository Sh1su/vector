package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"

	"github.com/sh1su/vector/backend/internal/api"
	"github.com/sh1su/vector/backend/internal/assistant"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

func (s *Server) assistantSvc() (*assistant.Service, error) {
	if s.d.Assistant == nil {
		return nil, problem.NotFound()
	}
	return s.d.Assistant, nil
}

func (s *Server) GetAssistantStatus(ctx context.Context, _ api.GetAssistantStatusRequestObject) (api.GetAssistantStatusResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	st := assistant.StatusView{}
	if s.d.Assistant != nil {
		if st, err = s.d.Assistant.Status(ctx, a); err != nil {
			return nil, err
		}
	}
	var out api.AssistantStatus
	if err := convert(st, &out); err != nil {
		return nil, err
	}
	return api.GetAssistantStatus200JSONResponse{Body: out}, nil
}

func (s *Server) GiveAssistantConsent(ctx context.Context, req api.GiveAssistantConsentRequestObject) (api.GiveAssistantConsentResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := s.assistantSvc()
	if err != nil {
		return nil, err
	}
	st, err := svc.Consent(ctx, a, req.Body.AcceptExternalProvider, req.Body.ProviderName)
	if err != nil {
		return nil, err
	}
	var out api.AssistantStatus
	if err := convert(st, &out); err != nil {
		return nil, err
	}
	return api.GiveAssistantConsent200JSONResponse{Body: out}, nil
}

func (s *Server) RevokeAssistantConsent(ctx context.Context, _ api.RevokeAssistantConsentRequestObject) (api.RevokeAssistantConsentResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := s.assistantSvc()
	if err != nil {
		return nil, err
	}
	if err := svc.RevokeConsent(ctx, a); err != nil {
		return nil, err
	}
	return api.RevokeAssistantConsent204Response{}, nil
}

func (s *Server) ListConversations(ctx context.Context, _ api.ListConversationsRequestObject) (api.ListConversationsResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := s.assistantSvc()
	if err != nil {
		return nil, err
	}
	items, err := svc.Conversations(ctx, a)
	if err != nil {
		return nil, err
	}
	out := api.ListConversations200JSONResponse{Items: []api.Conversation{}, NextCursor: nullable.NewNullNullable[string]()}
	if err := convert(items, &out.Items); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Server) CreateConversation(ctx context.Context, req api.CreateConversationRequestObject) (api.CreateConversationResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := s.assistantSvc()
	if err != nil {
		return nil, err
	}
	var vid *uuid.UUID
	var title *string
	if req.Body != nil {
		if req.Body.VehicleId.IsSpecified() && !req.Body.VehicleId.IsNull() {
			id := uuid.UUID(req.Body.VehicleId.MustGet())
			vid = &id
		}
		if req.Body.Title.IsSpecified() && !req.Body.Title.IsNull() {
			t := req.Body.Title.MustGet()
			title = &t
		}
	}
	c, err := svc.CreateConversation(ctx, a, vid, title)
	if err != nil {
		return nil, err
	}
	var body api.Conversation
	if err := convert(c, &body); err != nil {
		return nil, err
	}
	loc := apiPrefix + "/assistant/conversations/" + c.ID.String()
	return api.CreateConversation201JSONResponse{Body: body, Headers: api.CreateConversation201ResponseHeaders{Location: &loc}}, nil
}

func (s *Server) DeleteConversation(ctx context.Context, req api.DeleteConversationRequestObject) (api.DeleteConversationResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := s.assistantSvc()
	if err != nil {
		return nil, err
	}
	if err := svc.DeleteConversation(ctx, a, uuid.UUID(req.ConversationId)); err != nil {
		return nil, err
	}
	return api.DeleteConversation204Response{}, nil
}

func (s *Server) ListAssistantMessages(ctx context.Context, req api.ListAssistantMessagesRequestObject) (api.ListAssistantMessagesResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := s.assistantSvc()
	if err != nil {
		return nil, err
	}
	items, err := svc.Messages(ctx, a, uuid.UUID(req.ConversationId))
	if err != nil {
		return nil, err
	}
	out := api.ListAssistantMessages200JSONResponse{Items: []api.AssistantMessage{}, NextCursor: nullable.NewNullNullable[string]()}
	if err := convert(items, &out.Items); err != nil {
		return nil, err
	}
	return out, nil
}

// sseResponse führt die Werkzeugschleife aus und streamt Zwischenstände als
// Server-Sent Events; das letzte Ereignis `message` enthält die AssistantMessage.
type sseResponse struct {
	ctx   context.Context
	svc   *assistant.Service
	actor kernel.Actor
	conv  uuid.UUID
	text  string
}

func (r sseResponse) VisitSendAssistantMessageResponse(w http.ResponseWriter) error {
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Minute)) // lokale Modelle brauchen Zeit
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	var mu sync.Mutex
	send := func(event string, data any) {
		b, _ := json.Marshal(data)
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
		_ = rc.Flush()
	}
	send("status", map[string]string{"state": "thinking"})
	msg, err := r.svc.Send(r.ctx, r.actor, r.conv, r.text, send)
	if err != nil {
		pe, ok := err.(*problem.Error)
		if !ok {
			pe = &problem.Error{Status: 500, Title: "Interner Fehler"}
		}
		send("error", pe)
		return nil
	}
	send("message", msg)
	return nil
}

func (s *Server) SendAssistantMessage(ctx context.Context, req api.SendAssistantMessageRequestObject) (api.SendAssistantMessageResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := s.assistantSvc()
	if err != nil {
		return nil, err
	}
	// Vorprüfungen vor dem Start des Stroms, damit Fehler als Problem Details ankommen.
	st, err := svc.Status(ctx, a)
	if err != nil {
		return nil, err
	}
	if !st.Enabled {
		return nil, problem.NotFound()
	}
	if !st.UserEnabled {
		return nil, problem.Forbidden("Bitte den Assistenten zuerst einschalten.")
	}
	if _, err := svc.Messages(ctx, a, uuid.UUID(req.ConversationId)); err != nil {
		return nil, err
	}
	return sseResponse{ctx: ctx, svc: svc, actor: a, conv: uuid.UUID(req.ConversationId), text: req.Body.Text}, nil
}

func (s *Server) ConfirmProposal(ctx context.Context, req api.ConfirmProposalRequestObject) (api.ConfirmProposalResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := s.assistantSvc()
	if err != nil {
		return nil, err
	}
	var edited map[string]any
	var codes []string
	reason := ""
	if req.Body != nil {
		if req.Body.Body != nil {
			edited = *req.Body.Body
		}
		codes = confirmList(req.Body.ConfirmAnomalies)
		reason = str(req.Body.AnomalyReason)
	}
	p, err := svc.Confirm(ctx, a, uuid.UUID(req.ProposalId), edited, codes, reason)
	if err != nil {
		return nil, err
	}
	var out api.Proposal
	if err := convert(p, &out); err != nil {
		return nil, err
	}
	return api.ConfirmProposal200JSONResponse{Body: out}, nil
}

func (s *Server) RejectProposal(ctx context.Context, req api.RejectProposalRequestObject) (api.RejectProposalResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := s.assistantSvc()
	if err != nil {
		return nil, err
	}
	p, err := svc.Reject(ctx, a, uuid.UUID(req.ProposalId))
	if err != nil {
		return nil, err
	}
	var out api.Proposal
	if err := convert(p, &out); err != nil {
		return nil, err
	}
	return api.RejectProposal200JSONResponse{Body: out}, nil
}
