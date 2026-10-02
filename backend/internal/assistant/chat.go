package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sh1su/vector/backend/internal/assistant/store"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

// Config steuert den Chat (ADR-024). Ohne Provider ist der Assistent aus.
type Config struct {
	Provider      string // "" (aus) | "anthropic"
	APIKey        string
	Model         string
	BaseURL       string // nur für Tests/Proxies
	WebSearch     bool
	DailyLimit    int // Modellanfragen je Nutzer und Tag
	RetentionDays int // Aufbewahrung der Unterhaltungen
	MaxTokens     int64
}

const providerName = "Anthropic (Claude)"

// Service ist der Chat-Assistent. Caller wird vom Server gesetzt.
type Service struct {
	pool   *pgxpool.Pool
	cfg    Config
	client anthropic.Client
	Caller Caller
	Now    func() time.Time
}

func NewService(pool *pgxpool.Pool, cfg Config) *Service {
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 4096
	}
	if cfg.DailyLimit == 0 {
		cfg.DailyLimit = 200
	}
	if cfg.RetentionDays == 0 {
		cfg.RetentionDays = 30
	}
	opts := []option.RequestOption{option.WithAPIKey(cfg.APIKey), option.WithMaxRetries(2), option.WithRequestTimeout(90 * time.Second)}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	return &Service{pool: pool, cfg: cfg, client: anthropic.NewClient(opts...), Now: time.Now}
}

func (s *Service) Enabled() bool {
	return s != nil && s.cfg.Provider == "anthropic" && s.cfg.APIKey != "" && s.cfg.Model != ""
}

func pg(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func pgOpt(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pg(*id)
}

// ---------- Status und Zustimmung ----------

type StatusView struct {
	Enabled         bool
	UserEnabled     bool
	ProviderName    string
	Model           string
	WebSearch       bool
	ConsentRequired bool
	ConsentGivenAt  *time.Time
}

func (s *Service) Status(ctx context.Context, actor kernel.Actor) (StatusView, error) {
	if s == nil {
		return StatusView{ProviderName: providerName, ConsentRequired: true}, nil
	}
	v := StatusView{Enabled: s.Enabled(), ProviderName: providerName, Model: s.cfg.Model, WebSearch: s.cfg.WebSearch, ConsentRequired: true}
	if !v.Enabled {
		return v, nil
	}
	c, err := store.New(s.pool).GetConsent(ctx, pg(actor.AccountID))
	if err == nil {
		t := c.GivenAt.Time
		v.ConsentGivenAt = &t
		v.UserEnabled = true
		v.ConsentRequired = false
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return v, err
	}
	return v, nil
}

func (s *Service) GiveConsent(ctx context.Context, actor kernel.Actor, accept bool, provider string) (StatusView, error) {
	if !accept || provider != providerName {
		return StatusView{}, problem.Validation(problem.FieldError{Pointer: "/accept_external_provider", Code: "consent_required",
			Message: "Die Zustimmung muss ausdrücklich für " + providerName + " erteilt werden."})
	}
	if _, err := store.New(s.pool).UpsertConsent(ctx, store.UpsertConsentParams{AccountID: pg(actor.AccountID), Provider: provider}); err != nil {
		return StatusView{}, err
	}
	return s.Status(ctx, actor)
}

func (s *Service) RevokeConsent(ctx context.Context, actor kernel.Actor) error {
	return store.New(s.pool).DeleteConsent(ctx, pg(actor.AccountID))
}

// ---------- Unterhaltungen ----------

type ConversationView struct {
	ID        uuid.UUID
	VehicleID *uuid.UUID
	Title     *string
	CreatedAt time.Time
}

func convView(c store.AssistantConversation) ConversationView {
	v := ConversationView{ID: uuid.UUID(c.ID.Bytes), CreatedAt: c.CreatedAt.Time}
	if c.VehicleID.Valid {
		id := uuid.UUID(c.VehicleID.Bytes)
		v.VehicleID = &id
	}
	if c.Title.Valid {
		t := c.Title.String
		v.Title = &t
	}
	return v
}

func (s *Service) CreateConversation(ctx context.Context, actor kernel.Actor, id *uuid.UUID, vehicleID *uuid.UUID, title *string) (ConversationView, bool, error) {
	q := store.New(s.pool)
	cid := kernel.NewID()
	if id != nil {
		if ex, err := q.GetConversation(ctx, store.GetConversationParams{ID: pg(*id), AccountID: pg(actor.AccountID)}); err == nil {
			return convView(ex), false, nil
		}
		cid = *id
	}
	if vehicleID != nil {
		if _, err := s.Caller.getVehicle(ctx, *vehicleID); err != nil {
			return ConversationView{}, false, problem.Validation(problem.FieldError{Pointer: "/vehicle_id", Code: "unknown_vehicle"})
		}
	}
	t := pgtype.Text{}
	if title != nil && strings.TrimSpace(*title) != "" {
		t = pgtype.Text{String: truncate(strings.TrimSpace(*title), 200), Valid: true}
	}
	c, err := q.InsertConversation(ctx, store.InsertConversationParams{ID: pg(cid), AccountID: pg(actor.AccountID), VehicleID: pgOpt(vehicleID), Title: t})
	if err != nil {
		return ConversationView{}, false, err
	}
	return convView(c), true, nil
}

func (c Caller) getVehicle(ctx context.Context, id uuid.UUID) (map[string]any, error) {
	return vehicle(ctx, c, id)
}

func (s *Service) Conversations(ctx context.Context, actor kernel.Actor) ([]ConversationView, error) {
	q := store.New(s.pool)
	_ = q.PurgeOldConversations(ctx, store.PurgeOldConversationsParams{AccountID: pg(actor.AccountID),
		UpdatedAt: pgtype.Timestamptz{Time: s.Now().AddDate(0, 0, -s.cfg.RetentionDays), Valid: true}})
	rows, err := q.ListConversations(ctx, pg(actor.AccountID))
	if err != nil {
		return nil, err
	}
	out := make([]ConversationView, 0, len(rows))
	for _, r := range rows {
		out = append(out, convView(r))
	}
	return out, nil
}

func (s *Service) DeleteConversation(ctx context.Context, actor kernel.Actor, id uuid.UUID) error {
	n, err := store.New(s.pool).DeleteConversation(ctx, store.DeleteConversationParams{ID: pg(id), AccountID: pg(actor.AccountID)})
	if err != nil {
		return err
	}
	if n == 0 {
		return problem.NotFound()
	}
	return nil
}

func (s *Service) conversation(ctx context.Context, actor kernel.Actor, id uuid.UUID) (store.AssistantConversation, error) {
	c, err := store.New(s.pool).GetConversation(ctx, store.GetConversationParams{ID: pg(id), AccountID: pg(actor.AccountID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return c, problem.NotFound()
	}
	return c, err
}

// ---------- Nachrichten und Vorschläge ----------

type ProposalView struct {
	ID        uuid.UUID
	Operation string
	VehicleID uuid.UUID
	Summary   string
	Body      map[string]any
	Anomalies []any
	Status    string
	ExpiresAt time.Time
	ResultID  *uuid.UUID
}

type MessageView struct {
	ID        uuid.UUID
	Role      string
	Text      string
	Proposals []ProposalView
	CreatedAt time.Time
}

func (s *Service) proposalView(p store.AssistantProposal) ProposalView {
	v := ProposalView{ID: uuid.UUID(p.ID.Bytes), Operation: p.Operation, VehicleID: uuid.UUID(p.VehicleID.Bytes), Summary: p.Summary,
		Status: p.Status, ExpiresAt: p.ExpiresAt.Time}
	_ = json.Unmarshal(p.Body, &v.Body)
	_ = json.Unmarshal(p.Anomalies, &v.Anomalies)
	if v.Anomalies == nil {
		v.Anomalies = []any{}
	}
	if v.Status == "pending" && s.Now().After(v.ExpiresAt) {
		v.Status = "expired"
	}
	if p.ResultID.Valid {
		id := uuid.UUID(p.ResultID.Bytes)
		v.ResultID = &id
	}
	return v
}

func (s *Service) messageViews(ctx context.Context, actor kernel.Actor, rows []store.AssistantMessage) ([]MessageView, error) {
	var ids []pgtype.UUID
	for _, r := range rows {
		ids = append(ids, r.ProposalIds...)
	}
	props := map[uuid.UUID]ProposalView{}
	if len(ids) > 0 {
		list, err := store.New(s.pool).ListProposals(ctx, store.ListProposalsParams{AccountID: pg(actor.AccountID), Ids: ids})
		if err != nil {
			return nil, err
		}
		for _, p := range list {
			props[uuid.UUID(p.ID.Bytes)] = s.proposalView(p)
		}
	}
	out := make([]MessageView, 0, len(rows))
	for _, r := range rows {
		m := MessageView{ID: uuid.UUID(r.ID.Bytes), Role: r.Role, Text: r.Text, CreatedAt: r.CreatedAt.Time, Proposals: []ProposalView{}}
		for _, id := range r.ProposalIds {
			if p, ok := props[uuid.UUID(id.Bytes)]; ok {
				m.Proposals = append(m.Proposals, p)
			}
		}
		out = append(out, m)
	}
	return out, nil
}

func (s *Service) Messages(ctx context.Context, actor kernel.Actor, convID uuid.UUID) ([]MessageView, error) {
	if _, err := s.conversation(ctx, actor, convID); err != nil {
		return nil, err
	}
	rows, err := store.New(s.pool).ListMessages(ctx, pg(convID))
	if err != nil {
		return nil, err
	}
	return s.messageViews(ctx, actor, rows)
}

var statusLabel = map[string]string{"pending": "wartet auf Bestätigung", "confirmed": "bestätigt und gespeichert", "rejected": "vom Nutzer verworfen", "expired": "abgelaufen"}

// history baut den Verlauf für das Modell aus den gespeicherten Texten (ohne Werkzeugdetails).
func history(msgs []MessageView) []anthropic.MessageParam {
	out := []anthropic.MessageParam{}
	for _, m := range msgs {
		text := m.Text
		for _, p := range m.Proposals {
			text += fmt.Sprintf("\n[Vorschlag %s: %s – %s]", p.ID, p.Summary, statusLabel[p.Status])
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		if m.Role == "user" {
			out = append(out, anthropic.NewUserMessage(anthropic.NewTextBlock(text)))
		} else {
			out = append(out, anthropic.NewAssistantMessage(anthropic.NewTextBlock(text)))
		}
	}
	return out
}

const systemPrompt = `Du bist der Assistent von Vectra, einer Anwendung zur Fahrzeugverwaltung (Kilometerstände, Fahrtenbuch, Wartung, Servicehistorie, Kosten). Du sprichst Deutsch, kurz und freundlich; die Eingaben des Nutzers sind oft diktiert und können Tippfehler oder ausgeschriebene Zahlen enthalten („hundertdreiundvierzigtausend fünfhundert“ = 143500).

Arbeitsweise:
- Daten liest und schreibst du nur über die Werkzeuge. Erfinde keine Werte. Ist das Fahrzeug unklar und hat der Nutzer mehrere, frage nach (oder nimm das Fahrzeug aus dem Kontext).
- Schreibende Werkzeuge speichern nichts sofort: Sie erzeugen einen Vorschlag, den der Nutzer in der App mit einem Klick bestätigt. Sage deshalb „Ich habe … vorbereitet, bitte bestätigen“, nie „gespeichert“.
- Fahrtenbuch: „Ich fahre jetzt los, Stand 143.520, Kundentermin bei Müller“ → start_trip mit Kategorie business und dem Zweck. „Bin angekommen, 143.580“ → finish_trip. Nachträgliche Fahrten mit record_trip. Arbeitsweg = commute, privat = private. Für Geschäftsfahrten brauchst du einen Zweck; fehlt er, frage nach.
- Ein genannter Kilometerstand ohne Fahrt → record_odometer.
- Wartungsbücher: Ist für das Modell eins hinterlegt (list_maintenance_books), schlage apply_maintenance_book vor. Sonst recherchiere die Herstellerintervalle (falls die Websuche verfügbar ist) und schlage create_maintenance_plan mit der Quelle vor. Kennzeichne Intervalle als Richtwerte, die der Nutzer mit seinem Serviceheft abgleichen soll. Frage nach Motor/Baujahr, wenn die Intervalle davon abhängen.
- Websuche nur für öffentliche Herstellerinformationen. Gib niemals persönliche Daten (Kennzeichen, FIN, Namen, Orte) in Suchanfragen ein.
- Werkzeugfehler mit Plausibilitätsbefunden: erkläre sie verständlich; der Nutzer kann sie beim Bestätigen mit Begründung annehmen oder den Wert korrigieren.
- Zahlen im deutschen Format (143.520 km, 12,50 €).`

func (s *Service) toolParams() []anthropic.ToolUnionParam {
	var out []anthropic.ToolUnionParam
	for _, t := range Tools() {
		props, _ := t.Schema["properties"].(map[string]any)
		req, _ := t.Schema["required"].([]string)
		tp := anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: anthropic.ToolInputSchemaParam{Properties: props, Required: req, ExtraFields: map[string]any{"additionalProperties": false}},
		}
		out = append(out, anthropic.ToolUnionParam{OfTool: &tp})
	}
	if s.cfg.WebSearch {
		out = append(out, anthropic.ToolUnionParam{OfWebSearchTool20250305: &anthropic.WebSearchTool20250305Param{MaxUses: anthropic.Int(5)}})
	}
	return out
}

var progressLabel = map[string]string{
	"list_vehicles": "Lese Fahrzeuge", "get_vehicle_overview": "Lese Fahrzeugdaten", "list_trips": "Lese Fahrtenbuch",
	"list_maintenance": "Lese Wartungen", "list_service_entries": "Lese Servicehistorie", "get_cost_summary": "Lese Kosten",
	"list_maintenance_books": "Suche Wartungsbücher", "web_search": "Suche im Web",
}

// Progress meldet Zwischenschritte (für SSE).
type Progress func(event string, data any)

// Send verarbeitet eine Nutzernachricht: Werkzeuge ausführen bzw. Vorschläge anlegen, Antwort speichern.
func (s *Service) Send(ctx context.Context, actor kernel.Actor, convID uuid.UUID, text string, progress Progress) (MessageView, error) {
	text = strings.TrimSpace(text)
	if text == "" || len([]rune(text)) > 8000 {
		return MessageView{}, problem.Validation(problem.FieldError{Pointer: "/text", Code: "length"})
	}
	st, err := s.Status(ctx, actor)
	if err != nil {
		return MessageView{}, err
	}
	if st.ConsentRequired {
		return MessageView{}, problem.Conflict("Bitte zuerst der Übertragung an " + providerName + " zustimmen.")
	}
	q := store.New(s.pool)
	n, err := q.CountRequestsSince(ctx, store.CountRequestsSinceParams{AccountID: pg(actor.AccountID), CreatedAt: pgtype.Timestamptz{Time: s.Now().Add(-24 * time.Hour), Valid: true}})
	if err != nil {
		return MessageView{}, err
	}
	if int(n) >= s.cfg.DailyLimit {
		return MessageView{}, problem.TooManyRequests("Tageslimit des Assistenten erreicht.")
	}
	conv, err := s.conversation(ctx, actor, convID)
	if err != nil {
		return MessageView{}, err
	}
	prev, err := s.Messages(ctx, actor, convID)
	if err != nil {
		return MessageView{}, err
	}
	if _, err := q.InsertMessage(ctx, store.InsertMessageParams{ID: pg(kernel.NewID()), ConversationID: conv.ID, Role: "user", Text: text, ProposalIds: []pgtype.UUID{}}); err != nil {
		return MessageView{}, err
	}
	messages := append(history(prev), anthropic.NewUserMessage(anthropic.NewTextBlock(text)))

	// Kontext ändert sich je Anfrage und steht deshalb hinter dem gecachten Teil (Prompt-Caching).
	loc, vehicleLine := time.Local, ""
	if conv.VehicleID.Valid {
		vid := uuid.UUID(conv.VehicleID.Bytes)
		if v, err := s.Caller.getVehicle(ctx, vid); err == nil {
			vehicleLine = fmt.Sprintf(" Aktuelles Fahrzeug in der App: %v (vehicle_id %s, Zeitzone %s).", v["display_name"], vid, zoneOf(v))
			if l, err := time.LoadLocation(zoneOf(v)); err == nil {
				loc = l
			}
		}
	}
	ctxText := "Heute: " + s.Now().In(loc).Format("Monday, 2006-01-02 15:04 MST") + "." + vehicleLine
	if !s.cfg.WebSearch {
		ctxText += " Websuche ist nicht verfügbar."
	}
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(s.cfg.Model),
		MaxTokens: s.cfg.MaxTokens,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt, CacheControl: anthropic.NewCacheControlEphemeralParam()},
			{Text: ctxText},
		},
		Tools:    s.toolParams(),
		Messages: messages,
	}
	tools := map[string]Tool{}
	for _, t := range Tools() {
		tools[t.Name] = t
	}
	var proposals []uuid.UUID
	var reply []string
	var used []string
	for i := 0; i < 10; i++ {
		resp, err := s.client.Messages.New(ctx, params)
		if err != nil {
			return MessageView{}, s.apiError(err)
		}
		_ = q.InsertRequestLog(ctx, store.InsertRequestLogParams{ID: pg(kernel.NewID()), AccountID: pg(actor.AccountID), Provider: "anthropic", Model: s.cfg.Model,
			InputTokens: int32(resp.Usage.InputTokens + resp.Usage.CacheReadInputTokens + resp.Usage.CacheCreationInputTokens), OutputTokens: int32(resp.Usage.OutputTokens), Tools: used})
		params.Messages = append(params.Messages, resp.ToParam())
		var turnText []string
		var results []anthropic.ContentBlockParamUnion
		for _, block := range resp.Content {
			switch b := block.AsAny().(type) {
			case anthropic.TextBlock:
				if t := strings.TrimSpace(b.Text); t != "" {
					turnText = append(turnText, t)
				}
			case anthropic.ServerToolUseBlock:
				progress("status", progressLabel["web_search"]+" …")
				used = append(used, "web_search")
			case anthropic.ToolUseBlock:
				used = append(used, b.Name)
				out, isErr, pid := s.runTool(ctx, actor, conv, tools, b, progress)
				if pid != nil {
					proposals = append(proposals, *pid)
				}
				results = append(results, anthropic.NewToolResultBlock(b.ID, out, isErr))
			}
		}
		reply = turnText
		switch resp.StopReason {
		case anthropic.StopReasonToolUse:
			params.Messages = append(params.Messages, anthropic.NewUserMessage(results...))
			continue
		case anthropic.StopReasonPauseTurn:
			continue // serverseitige Websuche läuft weiter
		case anthropic.StopReasonRefusal:
			reply = []string{"Dabei kann ich leider nicht helfen."}
		case anthropic.StopReasonMaxTokens:
			reply = append(reply, "(Antwort gekürzt)")
		}
		break
	}
	answer := strings.Join(reply, "\n\n")
	if answer == "" && len(proposals) > 0 {
		answer = "Ich habe es vorbereitet – bitte bestätigen."
	}
	if answer == "" {
		answer = "Dazu habe ich gerade keine Antwort."
	}
	pids := make([]pgtype.UUID, 0, len(proposals))
	for _, id := range proposals {
		pids = append(pids, pg(id))
	}
	row, err := q.InsertMessage(ctx, store.InsertMessageParams{ID: pg(kernel.NewID()), ConversationID: conv.ID, Role: "assistant", Text: answer, ProposalIds: pids})
	if err != nil {
		return MessageView{}, err
	}
	_ = q.TouchConversation(ctx, store.TouchConversationParams{ID: conv.ID, Title: pgtype.Text{String: truncate(text, 60), Valid: true}})
	views, err := s.messageViews(ctx, actor, []store.AssistantMessage{row})
	if err != nil {
		return MessageView{}, err
	}
	return views[0], nil
}

// runTool führt lesende Werkzeuge aus und legt für schreibende einen Vorschlag an (ADR-026).
func (s *Service) runTool(ctx context.Context, actor kernel.Actor, conv store.AssistantConversation, tools map[string]Tool, b anthropic.ToolUseBlock, progress Progress) (string, bool, *uuid.UUID) {
	t, ok := tools[b.Name]
	if !ok {
		return "Unbekanntes Werkzeug.", true, nil
	}
	var in map[string]any
	if err := json.Unmarshal([]byte(b.JSON.Input.Raw()), &in); err != nil || in == nil {
		in = map[string]any{}
	}
	if !t.Writes() {
		if l := progressLabel[t.Name]; l != "" {
			progress("status", l+" …")
		}
		out, isErr := RunDirect(ctx, s.Caller, t, in)
		return out, isErr, nil
	}
	progress("status", "Bereite Vorschlag vor …")
	plan, err := t.Prepare(ctx, s.Caller, in)
	if err != nil {
		return toolError(err), true, nil
	}
	body, _ := json.Marshal(plan.Body)
	id := kernel.NewID()
	p, err := store.New(s.pool).InsertProposal(ctx, store.InsertProposalParams{ID: pg(id), AccountID: pg(actor.AccountID), ConversationID: conv.ID,
		VehicleID: pg(plan.VehicleID), Operation: plan.Operation, Summary: plan.Summary, Body: body,
		ExpiresAt: pgtype.Timestamptz{Time: s.Now().Add(30 * time.Minute), Valid: true}})
	if err != nil {
		return "Interner Fehler beim Anlegen des Vorschlags.", true, nil
	}
	progress("proposal", s.proposalView(p))
	out, _ := json.Marshal(map[string]any{"proposal_id": id.String(), "summary": plan.Summary,
		"status": "Vorschlag angelegt – wartet auf Bestätigung durch den Nutzer in der App. Noch nicht gespeichert."})
	return string(out), false, &id
}

func (s *Service) apiError(err error) error {
	var ae *anthropic.Error
	if errors.As(err, &ae) {
		switch ae.StatusCode {
		case 401, 403:
			return &problem.Error{Status: 502, Type: "https://vectra.app/problems/assistant-provider", Title: "KI-Anbieter",
				Detail: "Der API-Schlüssel für " + providerName + " wird abgelehnt. Bitte den Administrator informieren."}
		case 429:
			return problem.TooManyRequests("Der KI-Anbieter ist gerade ausgelastet. Bitte gleich noch einmal versuchen.")
		case 400, 404:
			return &problem.Error{Status: 502, Type: "https://vectra.app/problems/assistant-provider", Title: "KI-Anbieter",
				Detail: "Anfrage an " + providerName + " abgelehnt (Modell " + s.cfg.Model + "?): " + truncate(ae.Error(), 300)}
		}
	}
	if errors.Is(err, context.Canceled) {
		return err
	}
	return &problem.Error{Status: 502, Type: "https://vectra.app/problems/assistant-provider", Title: "KI-Anbieter",
		Detail: providerName + " ist nicht erreichbar. Bitte später erneut versuchen."}
}

// Confirm führt einen Vorschlag aus – mit den Rechten des Nutzers und Herkunft „assistant“ (ADR-026).
func (s *Service) Confirm(ctx context.Context, actor kernel.Actor, id uuid.UUID, edited map[string]any, conf *Confirmation) (ProposalView, error) {
	q := store.New(s.pool)
	p, err := q.GetProposal(ctx, store.GetProposalParams{ID: pg(id), AccountID: pg(actor.AccountID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return ProposalView{}, problem.NotFound()
	}
	if err != nil {
		return ProposalView{}, err
	}
	v := s.proposalView(p)
	switch v.Status {
	case "confirmed":
		return v, nil
	case "rejected", "expired":
		return v, problem.Conflict("Der Vorschlag ist " + statusLabel[v.Status] + ".")
	}
	body := v.Body
	for k, val := range edited {
		if k == "trip_id" || k == "item_id" || k == "book_id" {
			continue // Bezug bleibt fest
		}
		body[k] = val
	}
	plan := Plan{Operation: v.Operation, VehicleID: v.VehicleID, Summary: v.Summary, Body: body}
	actor.Kind = "assistant"
	resultID, _, err := Execute(kernel.WithActor(ctx, actor), s.Caller, plan, id, conf)
	newBody, _ := json.Marshal(body)
	if err != nil {
		var ae *APIError
		if errors.As(err, &ae) {
			if an := ae.Anomalies(); len(an) > 0 {
				b, _ := json.Marshal(an)
				_ = q.SetProposalAnomalies(ctx, store.SetProposalAnomaliesParams{ID: p.ID, Anomalies: b})
			}
			return v, ae.AsProblem()
		}
		return v, err
	}
	an, _ := json.Marshal([]any{})
	p, err = q.DecideProposal(ctx, store.DecideProposalParams{ID: p.ID, Status: "confirmed", ResultID: pgOpt(resultID), Anomalies: an, Body: newBody})
	if err != nil {
		return v, err
	}
	return s.proposalView(p), nil
}

func (s *Service) Reject(ctx context.Context, actor kernel.Actor, id uuid.UUID) (ProposalView, error) {
	q := store.New(s.pool)
	p, err := q.GetProposal(ctx, store.GetProposalParams{ID: pg(id), AccountID: pg(actor.AccountID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return ProposalView{}, problem.NotFound()
	}
	if err != nil {
		return ProposalView{}, err
	}
	if p.Status != "pending" {
		v := s.proposalView(p)
		if v.Status == "rejected" {
			return v, nil
		}
		return v, problem.Conflict("Der Vorschlag ist " + statusLabel[v.Status] + ".")
	}
	p, err = q.DecideProposal(ctx, store.DecideProposalParams{ID: p.ID, Status: "rejected", Anomalies: p.Anomalies, Body: p.Body})
	if err != nil {
		return ProposalView{}, err
	}
	return s.proposalView(p), nil
}

// AsProblem gibt die ursprüngliche Fehlerantwort der API unverändert weiter.
func (e *APIError) AsProblem() *problem.Error {
	b, _ := json.Marshal(e.Problem)
	var p problem.Error
	_ = json.Unmarshal(b, &p)
	if p.Status == 0 {
		p.Status = e.Status
		p.Title = e.Error()
	}
	return &p
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}
