package server

import (
	"context"
	"encoding/json"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

// units liefert die Anzeigeeinheiten des Nutzers (ADR-007).
func (s *Server) units(ctx context.Context, a kernel.Actor) kernel.Units {
	st, err := s.d.Identity.Settings(ctx, a.AccountID)
	if err != nil {
		return kernel.UnitsFromSettings(nil)
	}
	return kernel.UnitsFromSettings(st)
}

// splitPatch trennt einen Merge Patch von den Bestätigungsfeldern (ADR-010).
func splitPatch(body any) ([]byte, []string, string, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, nil, "", problem.BadRequest("")
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, nil, "", problem.BadRequest("")
	}
	var codes []string
	var reason string
	if v, ok := m["confirm_anomalies"]; ok {
		_ = json.Unmarshal(v, &codes)
	}
	if v, ok := m["anomaly_reason"]; ok {
		_ = json.Unmarshal(v, &reason)
	}
	delete(m, "confirm_anomalies")
	delete(m, "anomaly_reason")
	patch, _ := json.Marshal(m)
	return patch, codes, reason, nil
}

func datePtr(d *openapi_types.Date) *time.Time {
	if d == nil {
		return nil
	}
	t := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	return &t
}

func limitOf(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
