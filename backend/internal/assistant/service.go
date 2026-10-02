package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sh1su/vector/backend/internal/assistant/store"
	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/pg"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

// Config beschreibt den konfigurierten Anbieter (ADR-024). Die Kennzeichnung
// external bestätigt der Admin; sie wird nicht automatisch erkannt.
type Config struct {
	Provider      string // Typ: anthropic | openai_compatible
	Name          string // Anzeigename, z. B. „Claude (Anthropic)“ oder „Ollama lokal“
	Model         string
	External      bool
	DailyLimit    int
	RetentionDays int
}

// Service ist der Application Service des Moduls Assistant.
type Service struct {
	pool *pgxpool.Pool
	deps Deps
	chat Chat
	cfg  Config
	log  *slog.Logger
	Now  func() time.Time
	// Installation liefert die Installationseinstellungen (assistant_enabled).
	Installation func(ctx context.Context) (map[string]any, error)
}

// NewService erzeugt den Service; chat == nil bedeutet „kein Anbieter konfiguriert“.
func NewService(pool *pgxpool.Pool, deps Deps, chat Chat, cfg Config, log *slog.Logger) *Service {
	if cfg.DailyLimit <= 0 {
		cfg.DailyLimit = 200
	}
	if cfg.RetentionDays < 0 {
		cfg.RetentionDays = 30
	}
	return &Service{pool: pool, deps: deps, chat: chat, cfg: cfg, log: log, Now: time.Now}
}

// Provider entspricht chat_provider im Schema AssistantStatus.
type Provider struct {
	Name     string `json:"name"`
	External bool   `json:"external"`
}

// StatusView entspricht dem Schema AssistantStatus.
type StatusView struct {
	Enabled           bool       `json:"enabled"`
	UserEnabled       bool       `json:"user_enabled"`
	ChatProvider      *Provider  `json:"chat_provider"`
	EmbeddingProvider *Provider  `json:"embedding_provider"`
	ConsentRequired   bool       `json:"consent_required"`
	ConsentGivenAt    *time.Time `json:"consent_given_at"`
}

func (s *Service) enabled(ctx context.Context) bool {
	if s.chat == nil || s.Installation == nil {
		return false
	}
	st, err := s.Installation(ctx)
	if err != nil {
		return false
	}
	on, _ := st["assistant_enabled"].(bool)
	return on
}

// Configured meldet, ob ein Anbieter konfiguriert ist (Anzeige in den Admin-Einstellungen).
func (s *Service) Configured() *Provider {
	if s.chat == nil {
		return nil
	}
	return &Provider{Name: s.cfg.Name, External: s.cfg.External}
}

// Status liefert Aktivierung, Anbieter und Zustimmung des Nutzers.
func (s *Service) Status(ctx context.Context, actor kernel.Actor) (StatusView, error) {
	v := StatusView{Enabled: s.enabled(ctx)}
	if !v.Enabled {
		if actor.IsAdmin {
			v.ChatProvider = s.Configured() // Admins sehen, ob ein Anbieter konfiguriert ist
		}
		return v, nil
	}
	v.ChatProvider = s.Configured()
	us, err := store.New(s.pool).GetUserState(ctx, pg.U(actor.AccountID))
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return v, err
	}
	consented := err == nil && us.ConsentProvider.Valid && us.ConsentProvider.String == s.cfg.Name
	v.UserEnabled = err == nil && us.Enabled && (consented || !s.cfg.External)
	v.ConsentRequired = s.cfg.External && !consented
	if consented && us.ConsentAt.Valid {
		t := us.ConsentAt.Time
		v.ConsentGivenAt = &t
	}
	return v, nil
}

func (s *Service) requireEnabled(ctx context.Context) error {
	if !s.enabled(ctx) {
		return problem.NotFound()
	}
	return nil
}

// Consent schaltet den Assistenten für den Nutzer ein. Bei externem Anbieter
// ist die ausdrückliche Zustimmung mit Nennung des Anbieters Pflicht (ADR-024).
func (s *Service) Consent(ctx context.Context, actor kernel.Actor, accept bool, provider string) (StatusView, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return StatusView{}, err
	}
	if s.cfg.External && (!accept || provider != s.cfg.Name) {
		return StatusView{}, problem.Validation(problem.FieldError{Pointer: "/accept_external_provider", Code: "consent_required",
			Message: "Bitte der Übertragung an " + s.cfg.Name + " ausdrücklich zustimmen."})
	}
	if err := store.New(s.pool).UpsertUserState(ctx, store.UpsertUserStateParams{AccountID: pg.U(actor.AccountID), Enabled: true,
		ConsentProvider: pgtype.Text{String: s.cfg.Name, Valid: true}, ConsentAt: pgtype.Timestamptz{Time: s.Now(), Valid: true}}); err != nil {
		return StatusView{}, err
	}
	return s.Status(ctx, actor)
}

// RevokeConsent schaltet den Assistenten für den Nutzer ab.
func (s *Service) RevokeConsent(ctx context.Context, actor kernel.Actor) error {
	if err := s.requireEnabled(ctx); err != nil {
		return err
	}
	return store.New(s.pool).UpsertUserState(ctx, store.UpsertUserStateParams{AccountID: pg.U(actor.AccountID), Enabled: false})
}

func (s *Service) requireUser(ctx context.Context, actor kernel.Actor) error {
	st, err := s.Status(ctx, actor)
	if err != nil {
		return err
	}
	if !st.Enabled {
		return problem.NotFound()
	}
	if !st.UserEnabled {
		return problem.Forbidden("Bitte den Assistenten zuerst in den Einstellungen bzw. auf der Assistent-Seite einschalten.")
	}
	return nil
}

// ConversationView entspricht dem Schema Conversation.
type ConversationView struct {
	ID        uuid.UUID  `json:"id"`
	Title     *string    `json:"title"`
	VehicleID *uuid.UUID `json:"vehicle_id"`
	CreatedAt time.Time  `json:"created_at"`
}

func convView(c store.AssistantConversation) ConversationView {
	return ConversationView{ID: uuid.UUID(c.ID.Bytes), Title: pg.TextPtr(c.Title), VehicleID: pg.UUIDPtr(c.VehicleID), CreatedAt: c.CreatedAt.Time}
}

// Conversations listet die Unterhaltungen des Nutzers.
func (s *Service) Conversations(ctx context.Context, actor kernel.Actor) ([]ConversationView, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}
	rows, err := store.New(s.pool).ListConversations(ctx, pg.U(actor.AccountID))
	if err != nil {
		return nil, err
	}
	out := []ConversationView{}
	for _, r := range rows {
		out = append(out, convView(r))
	}
	return out, nil
}

// CreateConversation beginnt eine Unterhaltung, optional mit Fahrzeugbezug.
func (s *Service) CreateConversation(ctx context.Context, actor kernel.Actor, vehicleID *uuid.UUID, title *string) (ConversationView, error) {
	if err := s.requireUser(ctx, actor); err != nil {
		return ConversationView{}, err
	}
	if vehicleID != nil {
		if _, err := identity.Authorize(ctx, s.pool, actor, *vehicleID, identity.RoleViewer); err != nil {
			return ConversationView{}, err
		}
	}
	c, err := store.New(s.pool).InsertConversation(ctx, store.InsertConversationParams{ID: pg.U(kernel.NewID()), AccountID: pg.U(actor.AccountID),
		VehicleID: pg.Up(vehicleID), Title: pg.Text(title)})
	if err != nil {
		return ConversationView{}, err
	}
	return convView(c), nil
}

// DeleteConversation löscht eine Unterhaltung mit allen Nachrichten (ADR-024).
func (s *Service) DeleteConversation(ctx context.Context, actor kernel.Actor, id uuid.UUID) error {
	if err := s.requireEnabled(ctx); err != nil {
		return err
	}
	n, err := store.New(s.pool).DeleteConversation(ctx, store.DeleteConversationParams{ID: pg.U(id), AccountID: pg.U(actor.AccountID)})
	if err != nil {
		return err
	}
	if n == 0 {
		return problem.NotFound()
	}
	return nil
}

// ProposalView entspricht dem Schema Proposal.
type ProposalView struct {
	ID        uuid.UUID         `json:"id"`
	Operation string            `json:"operation"`
	VehicleID uuid.UUID         `json:"vehicle_id"`
	Body      map[string]any    `json:"body"`
	Anomalies []problem.Anomaly `json:"anomalies"`
	Status    string            `json:"status"`
	ExpiresAt time.Time         `json:"expires_at"`
	ResultID  *uuid.UUID        `json:"result_id"`
	messageID *uuid.UUID
}

func (s *Service) proposalView(p store.AssistantProposal) ProposalView {
	v := ProposalView{ID: uuid.UUID(p.ID.Bytes), Operation: p.Operation, VehicleID: uuid.UUID(p.VehicleID.Bytes), Status: p.Status,
		ExpiresAt: p.ExpiresAt.Time, ResultID: pg.UUIDPtr(p.ResultID), messageID: pg.UUIDPtr(p.MessageID), Anomalies: []problem.Anomaly{}}
	_ = json.Unmarshal(p.Body, &v.Body)
	_ = json.Unmarshal(p.Anomalies, &v.Anomalies)
	if v.Status == "pending" && s.Now().After(v.ExpiresAt) {
		v.Status = "expired"
	}
	return v
}

func (s *Service) saveProposal(ctx context.Context, actor kernel.Actor, conv, id, vid uuid.UUID, op string, body map[string]any, anomalies []problem.Anomaly) (ProposalView, error) {
	b, _ := json.Marshal(body)
	if anomalies == nil {
		anomalies = []problem.Anomaly{}
	}
	a, _ := json.Marshal(anomalies)
	p, err := store.New(s.pool).InsertProposal(ctx, store.InsertProposalParams{ID: pg.U(id), ConversationID: pg.U(conv), AccountID: pg.U(actor.AccountID),
		VehicleID: pg.U(vid), Operation: op, Body: b, Anomalies: a, ExpiresAt: pgtype.Timestamptz{Time: s.Now().Add(30 * time.Minute), Valid: true}})
	if err != nil {
		return ProposalView{}, err
	}
	return s.proposalView(p), nil
}

// MessageView entspricht dem Schema AssistantMessage.
type MessageView struct {
	ID        uuid.UUID        `json:"id"`
	Role      string           `json:"role"`
	Text      string           `json:"text"`
	Citations []map[string]any `json:"citations"`
	Proposals []ProposalView   `json:"proposals"`
	CreatedAt time.Time        `json:"created_at"`
}

func (s *Service) conversation(ctx context.Context, actor kernel.Actor, id uuid.UUID) (store.AssistantConversation, error) {
	c, err := store.New(s.pool).GetConversation(ctx, store.GetConversationParams{ID: pg.U(id), AccountID: pg.U(actor.AccountID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return c, problem.NotFound()
	}
	return c, err
}

// Messages listet die Nachrichten einer Unterhaltung mit ihren Vorschlägen.
func (s *Service) Messages(ctx context.Context, actor kernel.Actor, convID uuid.UUID) ([]MessageView, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}
	c, err := s.conversation(ctx, actor, convID)
	if err != nil {
		return nil, err
	}
	q := store.New(s.pool)
	rows, err := q.ListMessages(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	props, err := q.ListProposalsForConversation(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	byMsg := map[uuid.UUID][]ProposalView{}
	for _, p := range props {
		v := s.proposalView(p)
		if v.messageID != nil {
			byMsg[*v.messageID] = append(byMsg[*v.messageID], v)
		}
	}
	out := []MessageView{}
	for _, r := range rows {
		id := uuid.UUID(r.ID.Bytes)
		m := MessageView{ID: id, Role: r.Role, Text: r.Text, Citations: []map[string]any{}, Proposals: byMsg[id], CreatedAt: r.CreatedAt.Time}
		if m.Proposals == nil {
			m.Proposals = []ProposalView{}
		}
		out = append(out, m)
	}
	return out, nil
}

// Send verarbeitet eine Nutzernachricht: Werkzeugschleife mit dem Anbieter,
// Vorschläge statt Schreibzugriffen, Protokoll ohne Inhalte (ADR-024/026).
// progress meldet Zwischenschritte für den SSE-Strom.
func (s *Service) Send(ctx context.Context, actor kernel.Actor, convID uuid.UUID, text string, progress func(string, any)) (MessageView, error) {
	if err := s.requireUser(ctx, actor); err != nil {
		return MessageView{}, err
	}
	text = strings.TrimSpace(text)
	if text == "" || len([]rune(text)) > 8000 {
		return MessageView{}, problem.Validation(problem.FieldError{Pointer: "/text", Code: "length", Message: "1–8000 Zeichen"})
	}
	c, err := s.conversation(ctx, actor, convID)
	if err != nil {
		return MessageView{}, err
	}
	q := store.New(s.pool)
	since := s.Now().Add(-24 * time.Hour)
	if n, err := q.CountRequestsSince(ctx, store.CountRequestsSinceParams{AccountID: pg.U(actor.AccountID), OccurredAt: pgtype.Timestamptz{Time: since, Valid: true}}); err != nil {
		return MessageView{}, err
	} else if int(n) >= s.cfg.DailyLimit {
		return MessageView{}, problem.TooManyRequests("Tageslimit des Assistenten erreicht.")
	}
	history, err := q.ListMessages(ctx, c.ID)
	if err != nil {
		return MessageView{}, err
	}
	if _, err := q.InsertMessage(ctx, store.InsertMessageParams{ID: pg.U(kernel.NewID()), ConversationID: c.ID, Role: "user", Text: text, Citations: []byte("[]")}); err != nil {
		return MessageView{}, err
	}
	title := text
	if r := []rune(title); len(r) > 60 {
		title = string(r[:60]) + "…"
	}
	_ = q.TouchConversation(ctx, store.TouchConversationParams{ID: c.ID, Title: pgtype.Text{String: title, Valid: true}})
	turns := make([]Turn, 0, len(history)+1)
	if len(history) > 20 {
		history = history[len(history)-20:]
	}
	for _, h := range history {
		turns = append(turns, Turn{Role: h.Role, Text: h.Text})
	}
	turns = append(turns, Turn{Role: "user", Text: text})

	sess := &session{svc: s, actor: actor, conversation: uuid.UUID(c.ID.Bytes), progress: progress}
	res, runErr := s.chat.Run(ctx, systemPrompt(s.Now(), pg.UUIDPtr(c.VehicleID)), turns, Tools(), sess.exec)
	outcome := "ok"
	answer := res.Text
	switch {
	case runErr != nil:
		outcome = "error"
		s.log.Warn("assistant provider failed", "err", runErr, "request_id", actor.RequestID)
		answer = "Der Sprachmodell-Anbieter ist gerade nicht erreichbar. Bitte später erneut versuchen."
	case res.Refused:
		outcome = "refused"
		answer = "Dazu kann ich leider keine Antwort geben. Bitte formuliere die Frage anders."
	case answer == "" && len(sess.proposals) > 0:
		answer = "Ich habe einen Vorschlag vorbereitet. Bitte prüfe und bestätige ihn."
	case answer == "":
		answer = "Dazu habe ich keine Antwort gefunden."
	}
	_ = q.InsertRequestLog(context.WithoutCancel(ctx), store.InsertRequestLogParams{ID: pg.U(kernel.NewID()), AccountID: pg.U(actor.AccountID), Provider: s.cfg.Name,
		Model: s.cfg.Model, InputTokens: res.InputTokens, OutputTokens: res.OutputTokens, Tools: nonNil(sortedNames(res.Tools)), Outcome: outcome})
	m, err := q.InsertMessage(context.WithoutCancel(ctx), store.InsertMessageParams{ID: pg.U(kernel.NewID()), ConversationID: c.ID, Role: "assistant", Text: answer, Citations: []byte("[]")})
	if err != nil {
		return MessageView{}, err
	}
	ids := make([]pgtype.UUID, 0, len(sess.proposals))
	for _, p := range sess.proposals {
		ids = append(ids, pg.U(p))
	}
	if len(ids) > 0 {
		if err := q.AttachProposals(context.WithoutCancel(ctx), store.AttachProposalsParams{MessageID: m.ID, Ids: ids}); err != nil {
			return MessageView{}, err
		}
	}
	msgs, err := s.Messages(context.WithoutCancel(ctx), actor, convID)
	if err != nil {
		return MessageView{}, err
	}
	for _, x := range msgs {
		if x.ID == uuid.UUID(m.ID.Bytes) {
			return x, nil
		}
	}
	return MessageView{ID: uuid.UUID(m.ID.Bytes), Role: "assistant", Text: answer, Citations: []map[string]any{}, Proposals: []ProposalView{}, CreatedAt: m.CreatedAt.Time}, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (s *Service) loadProposal(ctx context.Context, actor kernel.Actor, id uuid.UUID) (store.AssistantProposal, error) {
	p, err := store.New(s.pool).GetProposal(ctx, store.GetProposalParams{ID: pg.U(id), AccountID: pg.U(actor.AccountID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return p, problem.NotFound()
	}
	return p, err
}

// Confirm führt einen Vorschlag aus: Rechte, Plausibilität und Audit wie beim
// direkten API-Aufruf, Herkunft „assistant“ (ADR-026). Befunde bestätigt der Nutzer.
func (s *Service) Confirm(ctx context.Context, actor kernel.Actor, id uuid.UUID, edited map[string]any, confirm []string, reason string) (ProposalView, error) {
	if err := s.requireUser(ctx, actor); err != nil {
		return ProposalView{}, err
	}
	p, err := s.loadProposal(ctx, actor, id)
	if err != nil {
		return ProposalView{}, err
	}
	v := s.proposalView(p)
	switch v.Status {
	case "confirmed":
		return v, nil // Wiederholung: idempotent
	case "rejected", "expired":
		return v, problem.Conflict("Der Vorschlag ist " + map[string]string{"rejected": "verworfen", "expired": "abgelaufen"}[v.Status] + ".")
	}
	body := v.Body
	if edited != nil {
		body, err = s.completeBody(ctx, actor, v.Operation, v.VehicleID, mustJSON(edited))
		if err != nil {
			return ProposalView{}, err
		}
	}
	as := actor
	as.Kind = "assistant"
	resultID, err := s.execute(ctx, as, v.Operation, v.VehicleID, v.ID, body, confirm, reason)
	if err != nil {
		return ProposalView{}, err
	}
	b, _ := json.Marshal(body)
	row, err := store.New(s.pool).DecideProposal(ctx, store.DecideProposalParams{ID: p.ID, Status: "confirmed", ResultID: pg.U(resultID), Body: b})
	if errors.Is(err, pgx.ErrNoRows) {
		p, _ = s.loadProposal(ctx, actor, id)
		return s.proposalView(p), nil
	}
	if err != nil {
		return ProposalView{}, err
	}
	return s.proposalView(row), nil
}

func mustJSON(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

// Reject verwirft einen Vorschlag.
func (s *Service) Reject(ctx context.Context, actor kernel.Actor, id uuid.UUID) (ProposalView, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return ProposalView{}, err
	}
	p, err := s.loadProposal(ctx, actor, id)
	if err != nil {
		return ProposalView{}, err
	}
	if v := s.proposalView(p); v.Status != "pending" {
		return v, nil
	}
	row, err := store.New(s.pool).DecideProposal(ctx, store.DecideProposalParams{ID: p.ID, Status: "rejected", Body: p.Body})
	if errors.Is(err, pgx.ErrNoRows) {
		p, _ = s.loadProposal(ctx, actor, id)
		return s.proposalView(p), nil
	}
	if err != nil {
		return ProposalView{}, err
	}
	return s.proposalView(row), nil
}

// Cleanup löscht Unterhaltungen nach der Aufbewahrungsfrist (ADR-024).
func (s *Service) Cleanup(ctx context.Context) (int64, error) {
	cutoff := s.Now().AddDate(0, 0, -s.cfg.RetentionDays)
	return store.New(s.pool).DeleteOldConversations(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true})
}
