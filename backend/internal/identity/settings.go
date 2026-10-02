package identity

import (
	"time"

	"github.com/sh1su/vector/backend/internal/platform/problem"
)

func enumOK(v any, allowed ...string) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	for _, a := range allowed {
		if s == a {
			return true
		}
	}
	return false
}

// validateThresholds prüft das Schema MaintenanceThresholds (MA-05).
func validateThresholds(ptr string, v any, errs *[]problem.FieldError) {
	if v == nil {
		return
	}
	m, ok := v.(map[string]any)
	if !ok {
		*errs = append(*errs, problem.FieldError{Pointer: ptr, Code: "type"})
		return
	}
	for _, k := range []string{"upcoming_days", "due_days"} {
		if x, ok := m[k]; ok && x != nil {
			if f, ok := x.(float64); !ok || f < 0 || f != float64(int(f)) {
				*errs = append(*errs, problem.FieldError{Pointer: ptr + "/" + k, Code: "range"})
			}
		}
	}
	for _, k := range []string{"upcoming_distance", "due_distance"} {
		if x, ok := m[k]; ok && x != nil {
			q, ok := x.(map[string]any)
			val, vok := q["value"].(float64)
			if !ok || !vok || val < 0 || !enumOK(q["unit"], "km", "mi", "m", "h", "s") {
				*errs = append(*errs, problem.FieldError{Pointer: ptr + "/" + k, Code: "quantity"})
			}
		}
	}
}

// validateSettings prüft die Nutzereinstellungen nach dem Merge (Schema UserSettings).
func validateSettings(st map[string]any) error {
	var errs []problem.FieldError
	add := func(p, c string) { errs = append(errs, problem.FieldError{Pointer: p, Code: c}) }
	if v, ok := st["language"]; ok && !enumOK(v, "de", "en") {
		add("/language", "enum")
	}
	if v, ok := st["time_zone"]; ok {
		if s, _ := v.(string); s == "" {
			add("/time_zone", "time_zone")
		} else if _, err := time.LoadLocation(s); err != nil {
			add("/time_zone", "time_zone")
		}
	}
	if v, ok := st["default_currency"]; ok {
		if s, _ := v.(string); len(s) != 3 || s < "AAA" || s > "ZZZ" {
			add("/default_currency", "pattern")
		}
	}
	if du, ok := st["display_units"].(map[string]any); ok {
		allowed := map[string][]string{"distance": {"km", "mi"}, "volume": {"l", "gal_us", "gal_imp"},
			"consumption": {"l_per_100km", "km_per_l", "mpg_us", "mpg_uk"}, "electric_consumption": {"kwh_per_100km", "km_per_kwh", "mi_per_kwh"},
			"oil_volume": {"ml", "l", "qt_us", "qt_imp"}}
		for k, v := range du {
			if a, ok := allowed[k]; !ok || !enumOK(v, a...) {
				add("/display_units/"+k, "enum")
			}
		}
	}
	if v, ok := st["vehicle_identifier"]; ok && !enumOK(v, "license_plate", "vin", "custom_field") {
		add("/vehicle_identifier", "enum")
	}
	validateThresholds("/maintenance_thresholds", st["maintenance_thresholds"], &errs)
	if len(errs) > 0 {
		return problem.Validation(errs...)
	}
	return nil
}
