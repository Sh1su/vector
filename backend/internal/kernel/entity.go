package kernel

import (
	"time"

	"github.com/google/uuid"
)

// EntityMeta entspricht den API-Schemas EntityMeta und VehicleScoped.
type EntityMeta struct {
	ID         uuid.UUID  `json:"id"`
	Version    int        `json:"version"`
	VehicleID  uuid.UUID  `json:"vehicle_id"`
	CreatedAt  time.Time  `json:"created_at"`
	CreatedBy  uuid.UUID  `json:"created_by"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
	UpdatedBy  *uuid.UUID `json:"updated_by,omitempty"`
	RecordedAt *time.Time `json:"recorded_at,omitempty"`
	Origin     string     `json:"origin,omitempty"`
}

// DisplayValue ist ein berechneter Wert in Anzeigeeinheit (API DisplayValue).
type DisplayValue struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// OriginOf bildet den Akteur auf die Herkunft eines Eintrags ab.
func OriginOf(a Actor) string {
	if a.Kind == "api_token" {
		return "api"
	}
	return "web"
}

// Round rundet einen Anzeigewert auf digits Nachkommastellen.
func Round(v float64, digits int) float64 {
	p := 1.0
	for i := 0; i < digits; i++ {
		p *= 10
	}
	if v < 0 {
		return -float64(int64(-v*p+0.5)) / p
	}
	return float64(int64(v*p+0.5)) / p
}
