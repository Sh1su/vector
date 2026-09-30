// Package server verbindet die generierte API-Schnittstelle mit den Application
// Services der Module (ADR-001, ADR-003, ADR-014). Handler enthalten keine Fachlogik.
package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/sh1su/vector/backend/internal/api"
	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/odometer"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

// Deps sind die Services, die der Server braucht.
type Deps struct {
	Identity     *identity.Service
	Vehicles     *vehicles.Service
	Odometer     *odometer.Service
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
	api.HandlerWithOptions(strict, api.ChiServerOptions{
		BaseRouter: apiRouter,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			var rh *api.RequiredHeaderError
			if errors.As(err, &rh) && rh.ParamName == "If-Match" {
				problem.Write(w, problem.PreconditionRequired(), requestID(r))
				return
			}
			problem.Write(w, problem.Validation(problem.FieldError{Pointer: "", Code: "parameter", Message: err.Error()}), requestID(r))
		},
	})
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
