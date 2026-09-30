// Package kernel enthält fachliche Wertobjekte, die alle Module gleich nutzen
// (docs/phase-2/10-domaene-uebersicht.md §3).
package kernel

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
)

// Kanonische Einheiten (ADR-007).
const (
	UnitMeter       = "m"
	UnitSecond      = "s"
	UnitMilliliter  = "ml"
	UnitWattHour    = "Wh"
)

// Faktor je Eingabeeinheit: kanonische Einheit und exakter Umrechnungsfaktor.
var inputUnits = map[string]struct {
	canonical string
	factor    *big.Rat
}{
	"m":       {UnitMeter, rat("1")},
	"km":      {UnitMeter, rat("1000")},
	"mi":      {UnitMeter, rat("1609.344")},
	"s":       {UnitSecond, rat("1")},
	"h":       {UnitSecond, rat("3600")},
	"ml":      {UnitMilliliter, rat("1")},
	"l":       {UnitMilliliter, rat("1000")},
	"gal_us":  {UnitMilliliter, rat("3785.411784")},
	"gal_imp": {UnitMilliliter, rat("4546.09")},
	"qt_us":   {UnitMilliliter, rat("946.352946")},
	"qt_imp":  {UnitMilliliter, rat("1136.5225")},
	"Wh":      {UnitWattHour, rat("1")},
	"kWh":     {UnitWattHour, rat("1000")},
}

func rat(s string) *big.Rat {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		panic("invalid rational " + s)
	}
	return r
}

// ErrUnknownUnit meldet einen unbekannten Einheitencode.
var ErrUnknownUnit = errors.New("unknown unit")

// CanonicalUnitOf liefert die kanonische Einheit eines Eingabe-Einheitencodes.
func CanonicalUnitOf(unit string) (string, error) {
	u, ok := inputUnits[unit]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownUnit, unit)
	}
	return u.canonical, nil
}

// Quantity ist ein gespeicherter Messwert: kanonisch plus Originaleingabe.
type Quantity struct {
	Canonical     int64
	CanonicalUnit string
	InputValue    string // Dezimaldarstellung wie eingegeben
	InputUnit     string
}

// ToCanonical rechnet eine Eingabe exakt in die kanonische Einheit um und
// rundet kaufmännisch-gerade (half-even) auf ganze Einheiten (ODO-01).
// Der float64-Wert aus JSON wird über seine kürzeste Dezimaldarstellung gelesen,
// damit z. B. 0.7 exakt als 7/10 und nicht als Binärnäherung verrechnet wird.
func ToCanonical(value float64, unit string) (Quantity, error) {
	u, ok := inputUnits[unit]
	if !ok {
		return Quantity{}, fmt.Errorf("%w: %q", ErrUnknownUnit, unit)
	}
	dec := strconv.FormatFloat(value, 'f', -1, 64)
	r, ok := new(big.Rat).SetString(dec)
	if !ok {
		return Quantity{}, fmt.Errorf("invalid number %v", value)
	}
	r.Mul(r, u.factor)
	return Quantity{Canonical: RoundHalfEven(r), CanonicalUnit: u.canonical, InputValue: dec, InputUnit: unit}, nil
}

// RoundHalfEven rundet eine rationale Zahl auf die nächste ganze Zahl, bei
// genau .5 auf die gerade Nachbarzahl.
func RoundHalfEven(r *big.Rat) int64 {
	num, den := new(big.Int).Set(r.Num()), r.Denom()
	q, m := new(big.Int).QuoRem(num, den, new(big.Int))
	// Rest verdoppelt mit Nenner vergleichen (Beträge).
	twice := new(big.Int).Mul(new(big.Int).Abs(m), big.NewInt(2))
	cmp := twice.Cmp(den)
	step := big.NewInt(1)
	if num.Sign() < 0 {
		step = big.NewInt(-1)
	}
	if cmp > 0 || (cmp == 0 && q.Bit(0) == 1) {
		q.Add(q, step)
	}
	return q.Int64()
}

// FromCanonical rechnet einen kanonischen Wert in eine Anzeigeeinheit um.
func FromCanonical(canonical int64, unit string) (float64, error) {
	u, ok := inputUnits[unit]
	if !ok {
		return 0, fmt.Errorf("%w: %q", ErrUnknownUnit, unit)
	}
	r := new(big.Rat).SetInt64(canonical)
	r.Quo(r, u.factor)
	f, _ := r.Float64()
	return f, nil
}
