// Package fuel erfasst Tank- und Ladevorgänge und berechnet Verbrauch und
// Energiepreise (docs/phase-2/10-domaene-fuel.md).
package fuel

import (
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/sh1su/vector/backend/internal/kernel"
)

// Status eines Vorgangs in der Verbrauchsberechnung (Schema IntervalResult).
const (
	StatusNone          = "none"
	StatusAnchor        = "anchor"
	StatusComputed      = "computed"
	StatusNotComputable = "not_computable"
)

// Gründe für nicht berechenbare Intervalle (FU-03).
const (
	ReasonPreviousMissed      = "previous_missed"
	ReasonNonPositiveDistance = "non_positive_distance"
	ReasonNoAnchor            = "no_anchor"
	ReasonNoOdometer          = "no_odometer"
)

// Fill ist ein Vorgang in der für die Regeln nötigen Form.
type Fill struct {
	ID         uuid.UUID
	At         time.Time
	TimeZone   string
	RecordedAt time.Time
	Carrier    string
	Qty        int64 // ml bzw. Wh
	Full       bool
	PrevMissed bool
	Total      *int64 // Gesamtlaufleistung des Messpunkts; nil = ohne Stand
	SocStart   *float64
	SocEnd     *float64
}

// Less ordnet Vorgänge eindeutig: Zeitpunkt, Erfassungszeit, ID (Übersicht §3.1).
func Less(a, b Fill) bool {
	if !a.At.Equal(b.At) {
		return a.At.Before(b.At)
	}
	if !a.RecordedAt.Equal(b.RecordedAt) {
		return a.RecordedAt.Before(b.RecordedAt)
	}
	return a.ID.String() < b.ID.String()
}

// Sort sortiert Vorgänge in fachlicher Ordnung.
func Sort(fs []Fill) { sort.SliceStable(fs, func(i, j int) bool { return Less(fs[i], fs[j]) }) }

// Result ist das Ergebnis eines Vorgangs: Abschluss eines Intervalls, Anker
// oder Teil eines Intervalls (FU-02/FU-03).
type Result struct {
	Status string
	Reason string
	Qty    int64
	Dist   int64
}

// Interval ist ein abgeschlossenes Intervall (berechenbar oder nicht).
type Interval struct {
	ClosingID  uuid.UUID
	ClosingAt  time.Time
	TimeZone   string
	Qty        int64
	Dist       int64
	Computable bool
	Reason     string
}

// Intervals berechnet FU-02/FU-03 für die sortierten Vorgänge **eines**
// Energieträgers. Bei Strom (Netzbezug, FU-05.1) ist jeder Ladevorgang mit Stand
// Anker bzw. Abschluss, unabhängig vom Füllstand.
func Intervals(fills []Fill, electric bool) (map[uuid.UUID]Result, []Interval) {
	res := make(map[uuid.UUID]Result, len(fills))
	var out []Interval
	var anchor *Fill
	var acc int64
	missed := false
	for i := range fills {
		f := fills[i]
		closes := (f.Full || electric) && f.Total != nil
		if anchor == nil {
			if closes {
				anchor, acc, missed = &fills[i], 0, false
				res[f.ID] = Result{Status: StatusAnchor}
			} else if f.Total == nil {
				res[f.ID] = Result{Status: StatusNone, Reason: ReasonNoOdometer}
			} else {
				res[f.ID] = Result{Status: StatusNone, Reason: ReasonNoAnchor}
			}
			continue
		}
		acc += f.Qty
		if f.PrevMissed {
			missed = true
		}
		if !closes {
			r := Result{Status: StatusNone}
			if f.Total == nil {
				r.Reason = ReasonNoOdometer
			}
			res[f.ID] = r
			continue
		}
		iv := Interval{ClosingID: f.ID, ClosingAt: f.At, TimeZone: f.TimeZone, Qty: acc, Dist: *f.Total - *anchor.Total}
		switch {
		case missed:
			iv.Reason = ReasonPreviousMissed
		case iv.Dist <= 0:
			iv.Reason = ReasonNonPositiveDistance
		default:
			iv.Computable = true
		}
		r := Result{Status: StatusComputed, Qty: iv.Qty, Dist: iv.Dist}
		if !iv.Computable {
			r = Result{Status: StatusNotComputable, Reason: iv.Reason, Qty: iv.Qty, Dist: iv.Dist}
		}
		res[f.ID] = r
		out = append(out, iv)
		anchor, acc, missed = &fills[i], 0, false
	}
	return res, out
}

// Battery berechnet den Batterieverbrauch (FU-05.2) aus aufeinanderfolgenden
// Ladevorgängen a → b mit Ladezustand am Ende von a und am Beginn von b.
// Ohne Kapazität liefert die Funktion nichts (keine Schätzung aus Ladungen).
func Battery(fills []Fill, capacityWh int64) []Interval {
	if capacityWh <= 0 {
		return nil
	}
	var out []Interval
	for i := 1; i < len(fills); i++ {
		a, b := fills[i-1], fills[i]
		if a.SocEnd == nil || b.SocStart == nil || a.Total == nil || b.Total == nil {
			continue
		}
		energy := int64((*a.SocEnd - *b.SocStart) / 100 * float64(capacityWh))
		iv := Interval{ClosingID: b.ID, ClosingAt: b.At, TimeZone: b.TimeZone, Qty: energy, Dist: *b.Total - *a.Total}
		switch {
		case b.PrevMissed:
			iv.Reason = ReasonPreviousMissed
		case energy <= 0 || iv.Dist <= 0:
			iv.Reason = ReasonNonPositiveDistance
		default:
			iv.Computable = true
		}
		out = append(out, iv)
	}
	return out
}

// Month ist ein gewichteter Monatswert (FU-04): Menge und Distanz der
// berechenbaren Intervalle, deren Abschluss im Monat liegt.
type Month struct {
	Month string // JJJJ-MM
	Qty   int64
	Dist  int64
}

// Summary fasst die Intervalle eines Zeitraums zusammen (FU-04).
type Summary struct {
	Qty           int64
	Dist          int64
	Computed      int
	NotComputable int
	Monthly       []Month
}

// Summarize bildet Durchschnitt und Monatswerte über alle Intervalle, deren
// Abschluss (Kalenderdatum in der Zeitzone des Vorgangs) in [from, to] liegt.
// Leere Grenzen bedeuten „unbegrenzt“.
func Summarize(ivs []Interval, from, to *time.Time) Summary {
	var s Summary
	idx := map[string]int{}
	for _, iv := range ivs {
		d := kernel.LocalDate(iv.ClosingAt, iv.TimeZone)
		if (from != nil && d.Before(*from)) || (to != nil && d.After(*to)) {
			continue
		}
		if !iv.Computable {
			s.NotComputable++
			continue
		}
		s.Computed++
		s.Qty += iv.Qty
		s.Dist += iv.Dist
		key := d.Format("2006-01")
		i, ok := idx[key]
		if !ok {
			i = len(s.Monthly)
			idx[key] = i
			s.Monthly = append(s.Monthly, Month{Month: key})
		}
		s.Monthly[i].Qty += iv.Qty
		s.Monthly[i].Dist += iv.Dist
	}
	sort.Slice(s.Monthly, func(i, j int) bool { return s.Monthly[i].Month < s.Monthly[j].Month })
	return s
}

// Display rechnet einen Verbrauch (Menge je Meter) in die Anzeigeeinheit um
// (ADR-007). unit ist die Einstellung `consumption` bzw. `electric_consumption`.
func Display(qty, dist int64, unit string) (float64, string) {
	x := float64(qty) / float64(dist) // ml/m bzw. Wh/m
	switch unit {
	case "km_per_l":
		return 1 / x, "km/l"
	case "mpg_us":
		return 3785.411784 / (x * 1609.344), "mpg (US)"
	case "mpg_uk":
		return 4546.09 / (x * 1609.344), "mpg (UK)"
	case "km_per_kwh":
		return 1 / x, "km/kWh"
	case "mi_per_kwh":
		return 1 / x / 1.609344, "mi/kWh"
	case "kwh_per_100km":
		return x * 100, "kWh/100km"
	}
	return x * 100, "l/100km"
}
