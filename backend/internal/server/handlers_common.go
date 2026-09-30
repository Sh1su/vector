package server

import (
	"encoding/json"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

// dateP wandelt einen optionalen API-Datumsparameter in ein Kalenderdatum.
func dateP(d *openapi_types.Date) *time.Time {
	if d == nil {
		return nil
	}
	t := kernel.Date(d.Time)
	return &t
}

func etag(version int) *string {
	e := vehicles.ETag(version)
	return &e
}

func loc(path string) *string {
	l := apiPrefix + path
	return &l
}

// bodyTo überführt einen generierten Request-Body in die Eingabe eines Service.
func bodyTo(body, dst any) error {
	if err := convert(body, dst); err != nil {
		return problem.BadRequest("Der Request-Body ist ungültig.")
	}
	return nil
}

// patchOf serialisiert einen Merge-Patch-Body und trennt die Befundbestätigung ab.
func patchOf(body any) ([]byte, []string, string, error) {
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
	_ = json.Unmarshal(m["confirm_anomalies"], &codes)
	_ = json.Unmarshal(m["anomaly_reason"], &reason)
	delete(m, "confirm_anomalies")
	delete(m, "anomaly_reason")
	patch, _ := json.Marshal(m)
	return patch, codes, reason, nil
}

// page baut eine Seitenantwort {items, next_cursor} im generierten Typ.
func page(items any, next *string, dst any) error {
	return convert(map[string]any{"items": items, "next_cursor": next}, dst)
}
