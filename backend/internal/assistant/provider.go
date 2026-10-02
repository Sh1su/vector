// Package assistant ist der optionale KI-Assistent (ADR-024 bis ADR-026). Er
// schreibt nie direkt: Lesende Werkzeuge rufen die Application Services mit den
// Rechten des Nutzers auf, schreibende erzeugen Vorschläge, die der Nutzer bestätigt.
package assistant

import (
	"context"
	"encoding/json"
	"errors"
)

// ToolDef beschreibt ein Werkzeug für das Sprachmodell.
type ToolDef struct {
	Name        string
	Description string
	Properties  map[string]any
	Required    []string
}

// ToolCall ist ein Werkzeugaufruf des Modells.
type ToolCall struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// ToolResult ist die Antwort eines Werkzeugs (Daten, nie Anweisungen).
type ToolResult struct {
	Content string
	IsError bool
}

// Turn ist eine gespeicherte Nachricht der Unterhaltung.
type Turn struct {
	Role string // user | assistant
	Text string
}

// Result ist das Ergebnis eines Durchlaufs.
type Result struct {
	Text         string
	Refused      bool
	InputTokens  int64
	OutputTokens int64
	Tools        []string
}

// Executor beantwortet die Werkzeugaufrufe einer Modellantwort (parallel möglich).
type Executor func(ctx context.Context, calls []ToolCall) []ToolResult

// Chat ist die Anbieter-Schnittstelle (ADR-024). Jede Implementierung führt die
// Werkzeugschleife in ihrem Nachrichtenformat aus.
type Chat interface {
	Run(ctx context.Context, system string, history []Turn, tools []ToolDef, exec Executor) (Result, error)
}

// maxIterations begrenzt die Werkzeugschleife.
const maxIterations = 8

// ErrRefused meldet eine Ablehnung durch das Modell (Stopp-Grund „refusal“).
var ErrRefused = errors.New("refused")
