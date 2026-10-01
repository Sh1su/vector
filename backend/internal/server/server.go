// Package server verbindet die generierte API-Schnittstelle mit den Application
// Services der Module (ADR-001, ADR-003, ADR-014). Handler enthalten keine Fachlogik.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sh1su/vector/backend/internal/api"
	"github.com/sh1su/vector/backend/internal/assistant"
	"github.com/sh1su/vector/backend/internal/costs"
	"github.com/sh1su/vector/backend/internal/documents"
	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/maintenance"
	mstore "github.com/sh1su/vector/backend/internal/maintenance/store"
	"github.com/sh1su/vector/backend/internal/odometer"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/servicehistory"
	"github.com/sh1su/vector/backend/internal/trips"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

// Deps sind die Services, die der Server braucht.
type Deps struct {
	Identity     *identity.Service
	Vehicles     *vehicles.Service
	Odometer     *odometer.Service
	Costs        *costs.Service
	Maintenance  *maintenance.Service
	Service      *servicehistory.Service
	Trips        *trips.Service
	Documents    *documents.Service
	Assistant    *assistant.Service
	Log          *slog.Logger
	CookieSecure bool
	WebDir       string
	Ping         func(r *http.Request) error
}

// Server implementiert api.StrictServerInterface. Nicht umgesetzte Operationen
// liefern über api.NotImplemented 501.
type Server struct {
	api.NotImplemented
	d Deps
}

var _ api.StrictServerInterface = (*Server)(nil)

// Handler baut den vollständigen HTTP-Handler.
func Handler(d Deps) http.Handler {
	s := &Server{d: d}
	if d.Trips != nil {
		d.Trips.Unit = func(ctx context.Context, a kernel.Actor, meter string) string { return s.displayUnit(ctx, a, meter) }
	}
	if d.Maintenance != nil && d.Identity != nil {
		d.Maintenance.OwnerSettings = func(ctx context.Context, db mstore.DBTX, vehicleID uuid.UUID) map[string]any {
			return d.Identity.OwnerSettings(ctx, db, vehicleID)
		}
	}
	paramError := func(w http.ResponseWriter, r *http.Request, err error) {
		var rh *api.RequiredHeaderError
		if errors.As(err, &rh) && rh.ParamName == "If-Match" {
			problem.Write(w, problem.PreconditionRequired(), requestID(r))
			return
		}
		problem.Write(w, problem.Validation(problem.FieldError{Pointer: "", Code: "parameter", Message: err.Error()}), requestID(r))
	}
	strict := api.NewStrictHandlerWithOptions(s, nil, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			problem.Write(w, problem.BadRequest("Der Request-Body ist ungültig."), requestID(r))
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			if err == api.ErrNotImplemented {
				err = problem.NotImplemented()
			}
			var pe *problem.Error
			if !asProblem(err, &pe) {
				d.Log.Error("request failed", "err", err, "request_id", requestID(r))
			}
			problem.Write(w, err, requestID(r))
		},
	})
	r := chi.NewRouter()
	r.Use(withRequestID, func(h http.Handler) http.Handler { return withLogging(d.Log, h) }, withSecurityHeaders)
	apiRouter := chi.NewRouter()
	apiRouter.Use(withBodyLimit, withClientInfo, func(h http.Handler) http.Handler { return withAuth(d.Identity, h) })
	api.HandlerWithOptions(strict, api.ChiServerOptions{BaseRouter: apiRouter, ErrorHandlerFunc: paramError})

	// Interner Aufrufweg für Assistent und MCP: dieselbe API, Akteur aus dem Kontext statt Cookie.
	inner := chi.NewRouter()
	inner.Use(withBodyLimit)
	api.HandlerWithOptions(strict, api.ChiServerOptions{BaseRouter: inner, ErrorHandlerFunc: paramError})
	caller := internalCaller(inner)
	if d.Assistant != nil {
		d.Assistant.Caller = caller
	}
	apiRouter.Handle(mcpPath, assistant.MCP(caller, "1.0"))
	apiRouter.NotFound(func(w http.ResponseWriter, r *http.Request) { problem.Write(w, problem.NotFound(), requestID(r)) })
	r.Mount(apiPrefix, apiRouter)
	r.Get(apiPrefix+"/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "siehe api/openapi.yaml", http.StatusNotImplemented)
	})
	if d.WebDir != "" {
		r.NotFound(spa(d.WebDir))
	}
	return r
}

// spa liefert die statische Web-App aus; unbekannte Pfade erhalten index.html.
func spa(dir string) http.HandlerFunc {
	fs := http.FileServer(http.Dir(dir))
	return func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fs.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	}
}

func asProblem(err error, pe **problem.Error) bool {
	p, ok := err.(*problem.Error)
	if ok {
		*pe = p
	}
	return ok
}

// convert überführt Service-Darstellungen per JSON in die generierten Typen.
func convert(src, dst any) error {
	b, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

// internalCaller ruft die API ohne Netz auf; Rechte, Validierung und Audit gelten unverändert.
func internalCaller(h http.Handler) assistant.Caller {
	return func(ctx context.Context, method, path string, body any, headers map[string]string) (int, []byte) {
		var rd io.Reader
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				return 400, nil
			}
			rd = bytes.NewReader(b)
		}
		// Den Routing-Kontext der äußeren Anfrage entfernen, sonst routet chi nach deren Pfad.
		ctx = context.WithValue(ctx, chi.RouteCtxKey, nil)
		req, err := http.NewRequestWithContext(ctx, method, path, rd)
		if err != nil {
			return 400, nil
		}
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code, rec.Body.Bytes()
	}
}
