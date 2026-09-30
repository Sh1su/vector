package server

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

const (
	sessionCookie = "vectra_session"
	csrfCookie    = "vectra_csrf"
	csrfHeader    = "X-CSRF-Token"
	apiPrefix     = "/api/v1"
)

// publicPaths sind die Operationen mit `security: []` in der Spezifikation.
// Ein Test prüft die Übereinstimmung mit api/openapi.yaml.
var publicPaths = map[string]bool{
	"/auth/setup":                  true,
	"/auth/login":                  true,
	"/auth/oidc/start":             true,
	"/auth/oidc/callback":          true,
	"/auth/token":                  true,
	"/auth/password-reset":         true,
	"/auth/password-reset/confirm": true,
	"/health":                      true,
}

type ctxKey int

const requestIDKey ctxKey = 1

func requestID(r *http.Request) string {
	if v, ok := r.Context().Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// withRequestID vergibt jeder Anfrage eine ID (Logs, Problem Details, Audit).
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := kernel.NewID().String()
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(c int) { s.status = c; s.ResponseWriter.WriteHeader(c) }

// withLogging protokolliert jede Anfrage ohne Inhalte (ADR-030).
func withLogging(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("panic", "err", rec, "request_id", requestID(r))
				problem.Write(w, nil, requestID(r))
				sw.status = 500
			}
			log.Info("http", "method", r.Method, "path", r.URL.Path, "status", sw.status,
				"ms", time.Since(start).Milliseconds(), "request_id", requestID(r))
		}()
		next.ServeHTTP(sw, r)
	})
}

// withSecurityHeaders setzt die zentralen Sicherheits-Header (ADR-003).
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; style-src 'self'; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(self)")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// withBodyLimit begrenzt JSON-Bodies auf 1 MiB.
func withBodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		next.ServeHTTP(w, r)
	})
}

// withAuth löst die Sitzung auf, legt den Akteur in den Kontext und prüft
// CSRF für zustandsändernde Anfragen (ADR-015). Öffentliche Pfade sind ausgenommen.
func withAuth(ids *identity.Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, apiPrefix)
		ctx := r.Context()
		if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
			info, ok, err := ids.ResolveSession(ctx, c.Value)
			if err != nil {
				problem.Write(w, err, requestID(r))
				return
			}
			if ok {
				acc, err := ids.Account(ctx, info.AccountID)
				if err != nil {
					problem.Write(w, err, requestID(r))
					return
				}
				if unsafe(r.Method) && !publicPaths[path] {
					got := r.Header.Get(csrfHeader)
					if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(info.CSRFToken)) != 1 {
						problem.Write(w, problem.Forbidden("CSRF-Token fehlt oder ist ungültig."), requestID(r))
						return
					}
				}
				ctx = kernel.WithActor(ctx, kernel.Actor{AccountID: info.AccountID, IsAdmin: acc.IsAdmin, SessionID: info.SessionID,
					Kind: "user", RequestID: requestID(r)})
			}
		}
		if _, ok := kernel.ActorFrom(ctx); !ok && !publicPaths[path] {
			problem.Write(w, problem.Unauthorized(), requestID(r))
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func unsafe(m string) bool {
	return m == http.MethodPost || m == http.MethodPut || m == http.MethodPatch || m == http.MethodDelete
}
