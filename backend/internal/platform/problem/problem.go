// Package problem bildet fachliche Fehler auf RFC 9457 Problem Details ab (ADR-013).
package problem

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

const typeBase = "https://vectra.app/problems/"

// FieldError zeigt per JSON Pointer auf ein fehlerhaftes Feld.
type FieldError struct {
	Pointer string `json:"pointer"`
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

// Anomaly ist ein Plausibilitätsbefund (ADR-010).
type Anomaly struct {
	Code        string     `json:"code"`
	Confirmable bool       `json:"confirmable"`
	Message     string     `json:"message,omitempty"`
	RelatedID   *uuid.UUID `json:"related_id,omitempty"`
}

// Error ist ein fachlicher Fehler mit HTTP-Status.
type Error struct {
	Status    int          `json:"status"`
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Detail    string       `json:"detail,omitempty"`
	Errors    []FieldError `json:"errors,omitempty"`
	Anomalies []Anomaly    `json:"anomalies,omitempty"`
	Current   any          `json:"current,omitempty"`
	RequestID string       `json:"request_id,omitempty"`
}

func (e *Error) Error() string { return e.Title + ": " + e.Detail }

func newErr(status int, typ, title, detail string) *Error {
	return &Error{Status: status, Type: typeBase + typ, Title: title, Detail: detail}
}

// Konstruktoren für die häufigen Fälle.
func BadRequest(detail string) *Error   { return newErr(400, "bad-request", "Ungültige Anfrage", detail) }
func Unauthorized() *Error              { return newErr(401, "unauthorized", "Nicht angemeldet", "") }
func Forbidden(detail string) *Error    { return newErr(403, "forbidden", "Keine Berechtigung", detail) }
func NotFound() *Error                  { return newErr(404, "not-found", "Nicht gefunden", "") }
func Conflict(detail string) *Error     { return newErr(409, "conflict", "Konflikt", detail) }
func PreconditionRequired() *Error      { return newErr(428, "precondition-required", "If-Match fehlt", "") }
func NotImplemented() *Error            { return newErr(501, "not-implemented", "Noch nicht umgesetzt", "") }
func TooManyRequests(detail string) *Error {
	return newErr(429, "too-many-requests", "Zu viele Anfragen", detail)
}

// PreconditionFailed meldet eine veraltete Version und liefert den aktuellen Stand mit.
func PreconditionFailed(current any) *Error {
	e := newErr(412, "precondition-failed", "Version veraltet", "Der Datensatz wurde inzwischen geändert.")
	e.Current = current
	return e
}

// Validation meldet ungültige Felder.
func Validation(errs ...FieldError) *Error {
	e := newErr(422, "validation", "Ungültige Eingabe", "")
	e.Errors = errs
	return e
}

// Plausibility meldet Befunde; bestätigbare können mit confirm_anomalies bestätigt werden.
func Plausibility(anomalies []Anomaly) *Error {
	e := newErr(422, "plausibility", "Plausibilitätsprüfung", "Bitte Befunde prüfen und bestätigen oder korrigieren.")
	e.Anomalies = anomalies
	return e
}

// Write schreibt einen Fehler als application/problem+json. Unbekannte Fehler
// werden als 500 ohne interne Details ausgegeben.
func Write(w http.ResponseWriter, err error, requestID string) {
	var pe *Error
	if !errors.As(err, &pe) {
		pe = newErr(500, "internal", "Interner Fehler", "")
	}
	out := *pe
	out.RequestID = requestID
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(out.Status)
	_ = json.NewEncoder(w).Encode(out)
}
