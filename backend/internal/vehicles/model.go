// Package vehicles verwaltet Fahrzeugstammdaten und Lebenszyklus
// (docs/phase-2/10-domaene-vehicles.md).
package vehicles

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/sh1su/vector/backend/internal/platform/problem"
)

// Q ist eine Mengenangabe mit Einheit (API QuantityInput).
type Q struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// Money ist ein Geldbetrag (ADR-029).
type Money struct {
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
}

// DateMoney ist ein Datum mit optionalem Betrag (Kauf, Verkauf, Schätzwert).
type DateMoney struct {
	Date  string `json:"date"`
	Price *Money `json:"price,omitempty"`
}

// Input sind die änderbaren Felder (JSON-Namen wie in der API).
type Input struct {
	DisplayName           string             `json:"display_name"`
	VIN                   *string            `json:"vin,omitempty"`
	LicensePlate          *string            `json:"license_plate,omitempty"`
	PlateCountry          *string            `json:"plate_country,omitempty"`
	Make                  *string            `json:"make,omitempty"`
	Model                 *string            `json:"model,omitempty"`
	Variant               *string            `json:"variant,omitempty"`
	ModelYear             *int               `json:"model_year,omitempty"`
	FirstRegistration     *string            `json:"first_registration,omitempty"`
	BodyType              string             `json:"body_type"`
	EngineCode            *string            `json:"engine_code,omitempty"`
	DisplacementCcm       *int               `json:"displacement_ccm,omitempty"`
	PowerKw               *int               `json:"power_kw,omitempty"`
	Transmission          *string            `json:"transmission,omitempty"`
	UsageMeter            string             `json:"usage_meter"`
	EnergyCarriers        []string           `json:"energy_carriers"`
	TankCapacity          map[string]Q       `json:"tank_capacity,omitempty"`
	BatteryUsableCapacity *Q                 `json:"battery_usable_capacity,omitempty"`
	OilDipstickRange      *Q                 `json:"oil_dipstick_range,omitempty"`
	OilCapacity           *Q                 `json:"oil_capacity,omitempty"`
	OdometerRequired      *bool              `json:"odometer_required,omitempty"`
	OwnerTimeZone         *string            `json:"owner_time_zone,omitempty"`
	DefaultCurrency       *string            `json:"default_currency,omitempty"`
	DisplayUnits          map[string]any     `json:"display_units,omitempty"`
	Purchase              *DateMoney         `json:"purchase,omitempty"`
	EstimatedValue        *DateMoney         `json:"estimated_value,omitempty"`
	CustomFields          map[string]*string `json:"custom_fields,omitempty"`
	Note                  *string            `json:"note,omitempty"`
	Tags                  []string           `json:"tags,omitempty"`
}

// View ist die Lesedarstellung (Schema Vehicle der API).
type View struct {
	ID         string     `json:"id"`
	Version    int        `json:"version"`
	CreatedAt  time.Time  `json:"created_at"`
	CreatedBy  string     `json:"created_by"`
	UpdatedAt  time.Time  `json:"updated_at"`
	UpdatedBy  string     `json:"updated_by"`
	RecordedAt time.Time  `json:"recorded_at"`
	Origin     string     `json:"origin"`
	Status     string     `json:"status"`
	Sale       *DateMoney `json:"sale,omitempty"`
	MyRole     string     `json:"my_role"`
	Input
}

// extra wird als JSONB gespeichert.
type extra struct {
	TankCapacity          map[string]Q       `json:"tank_capacity,omitempty"`
	BatteryUsableCapacity *Q                 `json:"battery_usable_capacity,omitempty"`
	OilDipstickRange      *Q                 `json:"oil_dipstick_range,omitempty"`
	OilCapacity           *Q                 `json:"oil_capacity,omitempty"`
	DisplayUnits          map[string]any     `json:"display_units,omitempty"`
	CustomFields          map[string]*string `json:"custom_fields,omitempty"`
	EstimatedValue        *DateMoney         `json:"estimated_value,omitempty"`
}

var (
	bodyTypes     = set("car", "motorcycle", "van", "truck", "camper", "trailer", "tractor", "boat", "other")
	usageMeters   = set("distance", "engine_hours")
	carriers      = set("petrol", "diesel", "lpg", "electricity")
	transmissions = set("manual", "automatic", "other")
	reCountry     = regexp.MustCompile(`^[A-Z]{2}$`)
	reCurrency    = regexp.MustCompile(`^[A-Z]{3}$`)
	reVINChars    = regexp.MustCompile(`^[A-HJ-NPR-Z0-9]+$`)
)

func set(v ...string) map[string]bool {
	m := map[string]bool{}
	for _, s := range v {
		m[s] = true
	}
	return m
}

// NormalizeVIN entfernt Leerzeichen und Bindestriche und setzt Großbuchstaben (VE-01).
func NormalizeVIN(v string) string {
	v = strings.ToUpper(v)
	return strings.NewReplacer(" ", "", "-", "", "\t", "").Replace(v)
}

// validate prüft das Input und normalisiert es. Liefert harte Fehler und bestätigbare Befunde.
func (in *Input) validate() ([]problem.FieldError, []problem.Anomaly) {
	var errs []problem.FieldError
	var anomalies []problem.Anomaly
	add := func(ptr, code, msg string) { errs = append(errs, problem.FieldError{Pointer: ptr, Code: code, Message: msg}) }
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if n := len([]rune(in.DisplayName)); n < 1 || n > 80 {
		add("/display_name", "length", "1–80 Zeichen")
	}
	if !bodyTypes[in.BodyType] {
		add("/body_type", "enum", "")
	}
	if !usageMeters[in.UsageMeter] {
		add("/usage_meter", "enum", "")
	}
	seen := map[string]bool{}
	for i, c := range in.EnergyCarriers {
		if !carriers[c] || seen[c] {
			add("/energy_carriers/"+itoa(i), "enum", "unbekannt oder doppelt")
		}
		seen[c] = true
	}
	if in.EnergyCarriers == nil {
		in.EnergyCarriers = []string{}
	}
	if in.Transmission != nil && !transmissions[*in.Transmission] {
		add("/transmission", "enum", "")
	}
	if in.PlateCountry != nil && !reCountry.MatchString(*in.PlateCountry) {
		add("/plate_country", "pattern", "ISO 3166-1 alpha-2")
	}
	if in.DefaultCurrency != nil && !reCurrency.MatchString(*in.DefaultCurrency) {
		add("/default_currency", "pattern", "ISO 4217")
	}
	if in.OwnerTimeZone != nil {
		if _, err := time.LoadLocation(*in.OwnerTimeZone); err != nil || *in.OwnerTimeZone == "" {
			add("/owner_time_zone", "time_zone", "IANA-Zeitzone")
		}
	}
	if in.ModelYear != nil && (*in.ModelYear < 1885 || *in.ModelYear > 2100) {
		add("/model_year", "range", "")
	}
	for _, d := range []struct {
		p string
		v *string
	}{{"/first_registration", in.FirstRegistration}} {
		if d.v != nil {
			if _, err := time.Parse("2006-01-02", *d.v); err != nil {
				add(d.p, "date", "")
			}
		}
	}
	if in.Purchase != nil {
		if _, err := time.Parse("2006-01-02", in.Purchase.Date); err != nil {
			add("/purchase/date", "date", "")
		}
		if in.Purchase.Price != nil && !reCurrency.MatchString(in.Purchase.Price.Currency) {
			add("/purchase/price/currency", "pattern", "")
		}
	}
	if in.DisplacementCcm != nil && *in.DisplacementCcm < 0 {
		add("/displacement_ccm", "range", "")
	}
	if in.PowerKw != nil && *in.PowerKw < 0 {
		add("/power_kw", "range", "")
	}
	if len(in.Tags) > 20 {
		add("/tags", "max_items", "")
	}
	// VE-01
	if in.VIN != nil {
		v := NormalizeVIN(*in.VIN)
		if v == "" {
			in.VIN = nil
		} else {
			in.VIN = &v
			if !reVINChars.MatchString(v) {
				add("/vin", "vin_chars", "Nur A–Z (ohne I, O, Q) und Ziffern.")
			} else if len(v) != 17 {
				anomalies = append(anomalies, problem.Anomaly{Code: "VIN_NONSTANDARD", Confirmable: true,
					Message: "Die FIN hat nicht 17 Zeichen (z. B. Fahrzeuge vor 1981 oder Anhänger)."})
			}
		}
	}
	return errs, anomalies
}

func itoa(i int) string { b, _ := json.Marshal(i); return string(b) }
