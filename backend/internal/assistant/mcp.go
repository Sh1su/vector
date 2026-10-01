package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

// MCP-Server (Model Context Protocol, Transport „Streamable HTTP“, ADR-032). Zustandslos:
// jede POST-Anfrage enthält eine JSON-RPC-Nachricht, die Antwort ist application/json.
// Externe KI-Clients (z. B. Claude Desktop, Claude Code) melden sich mit einem API-Token an;
// sie bestätigen Werkzeugaufrufe selbst, deshalb führen schreibende Werkzeuge hier direkt aus.

var mcpVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

const mcpInstructions = "Vectra verwaltet Fahrzeugdaten: Kilometerstände, Fahrtenbuch, Wartung, Servicehistorie und Kosten. " +
	"Rufe zuerst list_vehicles auf, um die vehicle_id zu erfahren. Kilometerstände sind Gesamtlaufleistungen in km. " +
	"Lehnt Vectra einen Wert als unplausibel ab (z. B. kleiner als der letzte Stand), frage den Nutzer, ob der Wert stimmt, " +
	"und wiederhole den Aufruf nur mit seiner Begründung in anomaly_reason und den Befund-Codes in confirm_anomalies."

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// MCP liefert den HTTP-Handler. caller führt API-Aufrufe im Namen des Akteurs aus.
func MCP(caller Caller, version string) http.Handler {
	tools := Tools()
	byName := map[string]Tool{}
	for _, t := range tools {
		byName[t.Name] = t
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "nur POST (kein SSE-Strom)", http.StatusMethodNotAllowed)
			return
		}
		// Schutz vor DNS-Rebinding: Browser-Anfragen nur vom eigenen Host.
		if o := r.Header.Get("Origin"); o != "" {
			if u, err := url.Parse(o); err != nil || u.Host != r.Host {
				problem.Write(w, problem.Forbidden("Origin nicht erlaubt."), "")
				return
			}
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			problem.Write(w, problem.BadRequest(""), "")
			return
		}
		var reqs []rpcRequest
		batch := strings.HasPrefix(strings.TrimSpace(string(raw)), "[")
		if batch {
			err = json.Unmarshal(raw, &reqs)
		} else {
			var one rpcRequest
			err = json.Unmarshal(raw, &one)
			reqs = []rpcRequest{one}
		}
		if err != nil {
			writeJSON(w, rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32700, "Parse error"}})
			return
		}
		var out []rpcResponse
		for _, req := range reqs {
			if len(req.ID) == 0 { // Benachrichtigung
				continue
			}
			res, rerr := handleRPC(r.Context(), caller, version, tools, byName, req)
			resp := rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: res, Error: rerr}
			out = append(out, resp)
		}
		if len(out) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if batch {
			writeJSON(w, out)
			return
		}
		writeJSON(w, out[0])
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

var confirmProps = map[string]any{
	"confirm_anomalies": map[string]any{"type": "array", "items": map[string]any{"type": "string"},
		"description": "Nur nach Rückfrage beim Nutzer: Codes der Befunde, die er als korrekt bestätigt"},
	"anomaly_reason": map[string]any{"type": "string", "description": "Begründung des Nutzers für die Bestätigung"},
}

func mcpSchema(t Tool) map[string]any {
	if !t.Writes() {
		return t.Schema
	}
	props := map[string]any{}
	for k, v := range t.Schema["properties"].(map[string]any) {
		props[k] = v
	}
	for k, v := range confirmProps {
		props[k] = v
	}
	s := map[string]any{}
	for k, v := range t.Schema {
		s[k] = v
	}
	s["properties"] = props
	return s
}

func handleRPC(ctx context.Context, caller Caller, version string, tools []Tool, byName map[string]Tool, req rpcRequest) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		v := mcpVersions[0]
		for _, s := range mcpVersions {
			if s == p.ProtocolVersion {
				v = s
			}
		}
		return map[string]any{
			"protocolVersion": v,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "vectra", "title": "Vectra Fahrzeugverwaltung", "version": version},
			"instructions":    mcpInstructions,
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		actor, _ := kernel.ActorFrom(ctx)
		list := []map[string]any{}
		for _, t := range tools {
			if t.Writes() && !actor.HasScope(kernel.ScopeEntriesWrite) {
				continue
			}
			list = append(list, map[string]any{"name": t.Name, "description": t.Description, "inputSchema": mcpSchema(t),
				"annotations": map[string]any{"readOnlyHint": !t.Writes(), "destructiveHint": false, "idempotentHint": !t.Writes(), "openWorldHint": false}})
		}
		return map[string]any{"tools": list}, nil
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{-32602, "Ungültige Parameter"}
		}
		t, ok := byName[p.Name]
		if !ok {
			return nil, &rpcError{-32602, "Unbekanntes Werkzeug " + p.Name}
		}
		if p.Arguments == nil {
			p.Arguments = map[string]any{}
		}
		text, isErr := RunDirect(ctx, caller, t, p.Arguments)
		return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": isErr}, nil
	}
	return nil, &rpcError{-32601, "Methode nicht unterstützt: " + req.Method}
}

// RunDirect führt ein Werkzeug sofort aus (MCP). Liefert den Text für das Modell und ob es ein Fehler ist.
func RunDirect(ctx context.Context, caller Caller, t Tool, in map[string]any) (string, bool) {
	if !t.Writes() {
		res, err := t.Read(ctx, caller, in)
		if err != nil {
			return toolError(err), true
		}
		b, _ := json.Marshal(res)
		return string(b), false
	}
	actor, _ := kernel.ActorFrom(ctx)
	if !actor.HasScope(kernel.ScopeEntriesWrite) {
		return "Das Token darf nur lesen (Scope entries:write fehlt).", true
	}
	plan, err := t.Prepare(ctx, caller, in)
	if err != nil {
		return toolError(err), true
	}
	var conf *Confirmation
	if codes, ok := in["confirm_anomalies"].([]any); ok && len(codes) > 0 {
		conf = &Confirmation{Reason: str(in, "anomaly_reason")}
		for _, c := range codes {
			conf.Codes = append(conf.Codes, fmt.Sprint(c))
		}
		if conf.Reason == "" {
			return "anomaly_reason fehlt: Ohne Begründung des Nutzers werden Befunde nicht bestätigt.", true
		}
	}
	id, msg, err := Execute(ctx, caller, plan, kernel.NewID(), conf)
	if err != nil {
		return toolError(err), true
	}
	out := map[string]any{"done": plan.Summary, "message": msg}
	if id != nil {
		out["id"] = id.String()
	}
	b, _ := json.Marshal(out)
	return string(b), false
}

// toolError formuliert Fehler so, dass das Modell sinnvoll reagieren kann.
func toolError(err error) string {
	if ae, ok := err.(*APIError); ok {
		if an := ae.Anomalies(); len(an) > 0 {
			b, _ := json.Marshal(an)
			return "Vectra hält den Wert für unplausibel (ADR-010). Befunde: " + string(b) +
				". Frage den Nutzer, ob der Wert stimmt. Nur bestätigbare Befunde (confirmable=true) lassen sich mit seiner Begründung bestätigen; sonst den Wert korrigieren."
		}
		switch ae.Status {
		case 404:
			return "Nicht gefunden oder kein Zugriff: " + ae.Error()
		case 403:
			return "Keine Berechtigung: " + ae.Error()
		}
		return ae.Error()
	}
	if IsInput(err) {
		return err.Error()
	}
	return "Interner Fehler: " + err.Error()
}
