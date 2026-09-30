package server

import (
	"context"

	"encoding/json"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"net"
	"net/http"
	"time"

	"github.com/sh1su/vector/backend/internal/api"
	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

func mustActor(ctx context.Context) (kernel.Actor, error) {
	a, ok := kernel.ActorFrom(ctx)
	if !ok {
		return a, problem.Unauthorized()
	}
	return a, nil
}

func accountBody(a identity.Account) api.Account {
	st := api.AccountStatus(a.Status)
	return api.Account{Id: a.ID, Email: openapi_types.Email(a.Email), EmailVerified: &a.EmailVerified, DisplayName: a.DisplayName, IsAdmin: a.IsAdmin, Status: st}
}

func (s *Server) GetHealth(ctx context.Context, _ api.GetHealthRequestObject) (api.GetHealthResponseObject, error) {
	return api.GetHealth200JSONResponse{Body: api.Health{Status: api.HealthStatusPass}}, nil
}

func (s *Server) SetupInstallation(ctx context.Context, req api.SetupInstallationRequestObject) (api.SetupInstallationResponseObject, error) {
	b := req.Body
	acc, err := s.d.Identity.Setup(ctx, str(b.SetupToken), string(b.Email), b.DisplayName, str(b.Password))
	if err != nil {
		return nil, err
	}
	return api.SetupInstallation201JSONResponse{Body: accountBody(acc)}, nil
}

// loginResponse setzt Sitzungs- und CSRF-Cookie (ADR-015).
type loginResponse struct {
	acc    api.Account
	sess   identity.NewSession
	secure bool
}

func (l loginResponse) VisitLoginResponse(w http.ResponseWriter) error {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: l.sess.Token, Path: "/", Expires: l.sess.Expires,
		HttpOnly: true, Secure: l.secure, SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: csrfCookie, Value: l.sess.CSRFToken, Path: "/", Expires: l.sess.Expires,
		HttpOnly: false, Secure: l.secure, SameSite: http.SameSiteStrictMode})
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(200)
	return json.NewEncoder(w).Encode(l.acc)
}

func (s *Server) Login(ctx context.Context, req api.LoginRequestObject) (api.LoginResponseObject, error) {
	ip, ua := clientInfo(ctx)
	sess, err := s.d.Identity.Login(ctx, string(req.Body.Email), str(req.Body.Password), ip, ua)
	if err != nil {
		return nil, err
	}
	return loginResponse{acc: accountBody(sess.Account), sess: sess, secure: s.d.CookieSecure}, nil
}

type logoutResponse struct{ secure bool }

func (l logoutResponse) VisitLogoutResponse(w http.ResponseWriter) error {
	for _, n := range []string{sessionCookie, csrfCookie} {
		http.SetCookie(w, &http.Cookie{Name: n, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(0, 0), Secure: l.secure, HttpOnly: n == sessionCookie})
	}
	w.WriteHeader(204)
	return nil
}

func (s *Server) Logout(ctx context.Context, _ api.LogoutRequestObject) (api.LogoutResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.Identity.Logout(ctx, a.SessionID); err != nil {
		return nil, err
	}
	return logoutResponse{secure: s.d.CookieSecure}, nil
}

func (s *Server) GetMe(ctx context.Context, _ api.GetMeRequestObject) (api.GetMeResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	acc, err := s.d.Identity.Account(ctx, a.AccountID)
	if err != nil {
		return nil, err
	}
	return api.GetMe200JSONResponse{Body: accountBody(acc)}, nil
}

func (s *Server) UpdateMe(ctx context.Context, req api.UpdateMeRequestObject) (api.UpdateMeResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	acc, err := s.d.Identity.Account(ctx, a.AccountID)
	if err != nil {
		return nil, err
	}
	if req.Body.DisplayName != nil {
		if acc, err = s.d.Identity.UpdateDisplayName(ctx, a.AccountID, *req.Body.DisplayName); err != nil {
			return nil, err
		}
	}
	return api.UpdateMe200JSONResponse{Body: accountBody(acc)}, nil
}

func (s *Server) GetSettings(ctx context.Context, _ api.GetSettingsRequestObject) (api.GetSettingsResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	st, err := s.d.Identity.Settings(ctx, a.AccountID)
	if err != nil {
		return nil, err
	}
	var out api.UserSettings
	if err := convert(st, &out); err != nil {
		return nil, err
	}
	return api.GetSettings200JSONResponse{Body: out}, nil
}

func (s *Server) UpdateSettings(ctx context.Context, req api.UpdateSettingsRequestObject) (api.UpdateSettingsResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var patch map[string]any
	if err := convert(req.Body, &patch); err != nil {
		return nil, problem.BadRequest("")
	}
	st, err := s.d.Identity.PatchSettings(ctx, a.AccountID, patch)
	if err != nil {
		return nil, err
	}
	var out api.UserSettings
	if err := convert(st, &out); err != nil {
		return nil, err
	}
	return api.UpdateSettings200JSONResponse{Body: out}, nil
}

// clientInfo liest IP und User-Agent, die withClientInfo im Kontext ablegt.
func clientInfo(ctx context.Context) (string, string) {
	if ci, ok := ctx.Value(clientInfoKey).(clientInfoVal); ok {
		return ci.ip, ci.ua
	}
	return "", ""
}

type clientInfoVal struct{ ip, ua string }

const clientInfoKey ctxKey = 2

func withClientInfo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), clientInfoKey, clientInfoVal{ip: ip, ua: r.UserAgent()})))
	})
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
