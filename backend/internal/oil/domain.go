// Package oil erfasst Ölstandsmessungen, Nachfüllungen und Ölwechsel und
// berechnet Nachfüllrate und Ölverbrauch (docs/phase-2/10-domaene-oil.md).
package oil

import (
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/sh1su/vector/backend/internal/kernel"
)

// Arten eines Eintrags.
const (
	KindCheck     = "check"
	KindTopUp     = "top_up"
	KindOilChange = "oil_change"
)

// Stufen des Ölstands (E-13) und ihr Prozentwert; below_min/above_max sind nicht verwertbar.
var Steps = map[string]*float64{
	"min": ptr(0.0), "quarter": ptr(25.0), "half": ptr(50.0), "three_quarters": ptr(75.0), "max": ptr(100.0),
	"below_min": nil, "above_max": nil,
}

func ptr[T any](v T) *T { return &v }

// Entry ist ein Eintrag in der für die Regeln nötigen Form.
type Entry struct {
	ID          uuid.UUID
	Kind        string
	At          time.Time
	TimeZone    string
	RecordedAt  time.Time
	Total       *int64   // Gesamtlaufleistung (m)
	LevelBefore *float64 // verwertbarer Stand vor bzw. bei der Messung
	LevelAfter  *float64 // gemessener Stand danach
	AddedMl     int64    // nur top_up
	FillMl      int64    // nur oil_change (Einfüllmenge)
}

// Less ordnet Einträge eindeutig (Übersicht §3.1).
func Less(a, b Entry) bool {
	if !a.At.Equal(b.At) {
		return a.At.Before(b.At)
	}
	if !a.RecordedAt.Equal(b.RecordedAt) {
		return a.RecordedAt.Before(b.RecordedAt)
	}
	return a.ID.String() < b.ID.String()
}

// Sort sortiert in fachlicher Ordnung.
func Sort(es []Entry) { sort.SliceStable(es, func(i, j int) bool { return Less(es[i], es[j]) }) }

// AfterLevel liefert L_nach (OI-02) und ob er berechnet statt gemessen ist.
func AfterLevel(e Entry, rangeMl int64) (*float64, bool) {
	switch e.Kind {
	case KindCheck:
		return e.LevelBefore, false
	case KindTopUp:
		if e.LevelAfter != nil {
			return e.LevelAfter, false
		}
		if e.LevelBefore != nil && rangeMl > 0 {
			v := *e.LevelBefore + float64(e.AddedMl)/float64(rangeMl)*100
			return &v, true
		}
	case KindOilChange:
		return e.LevelAfter, false // keine Annahme „bis Max gefüllt“
	}
	return nil, false
}

// Pair ist das Ergebnis OI-02 eines Eintrags mit dem vorigen verwertbaren Punkt.
type Pair struct {
	Status   string // computed | not_computable | none
	Reason   string // no_range | no_level | non_positive_distance | series_start
	Basis    string // volume | percent_points
	Value    float64
	Dist     int64
	Negative bool
	MidTotal int64
}

// Series ist eine Messreihe (OI-00) mit Kennzahlen (OI-04).
type Series struct {
	ID         uuid.UUID
	StartEntry *Entry
	Entries    []Entry
	Pairs      map[uuid.UUID]Pair
	Usable     int
	SumValue   float64 // ml bzw. Prozentpunkte der berechenbaren Paare
	SumDist    int64
	Basis      string
}

// PreSeriesID ist die stabile ID der Reihe „vor erstem erfassten Ölwechsel“.
func PreSeriesID(vehicleID uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(vehicleID, []byte("oil-series-before-first-change"))
}

// BuildSeries teilt sortierte Einträge in Messreihen und berechnet die Paare.
func BuildSeries(vehicleID uuid.UUID, entries []Entry, rangeMl int64) []Series {
	var out []Series
	cur := Series{ID: PreSeriesID(vehicleID)}
	flush := func() {
		if len(cur.Entries) > 0 || cur.StartEntry != nil {
			out = append(out, cur)
		}
	}
	for i := range entries {
		e := entries[i]
		if e.Kind == KindOilChange {
			flush()
			cur = Series{ID: e.ID, StartEntry: &entries[i]}
		}
		cur.Entries = append(cur.Entries, e)
	}
	flush()
	for i := range out {
		computePairs(&out[i], rangeMl)
	}
	return out
}

func computePairs(s *Series, rangeMl int64) {
	s.Pairs = map[uuid.UUID]Pair{}
	s.Basis = "volume"
	if rangeMl <= 0 {
		s.Basis = "percent_points"
	}
	type point struct {
		level float64
		total *int64
	}
	var last *point
	var added int64
	for i, e := range s.Entries {
		if e.LevelBefore != nil || e.LevelAfter != nil {
			s.Usable++
		}
		p := Pair{Status: "none"}
		if i == 0 && e.Kind == KindOilChange {
			p.Reason = "series_start"
		}
		if e.LevelBefore != nil && e.Kind != KindOilChange {
			switch {
			case last == nil:
				if i > 0 {
					p = Pair{Status: "not_computable", Reason: "no_level"}
				}
			case last.total == nil || e.Total == nil || *e.Total-*last.total <= 0:
				p = Pair{Status: "not_computable", Reason: "non_positive_distance"}
			case rangeMl <= 0 && added > 0:
				p = Pair{Status: "not_computable", Reason: "no_range"}
			default:
				drop := last.level - *e.LevelBefore
				dist := *e.Total - *last.total
				v := drop
				basis := "percent_points"
				if rangeMl > 0 {
					v = drop/100*float64(rangeMl) + float64(added)
					basis = "volume"
				}
				p = Pair{Status: "computed", Basis: basis, Value: v, Dist: dist, Negative: v < 0, MidTotal: *last.total + dist/2}
				s.SumValue += v
				s.SumDist += dist
			}
		}
		s.Pairs[e.ID] = p
		// Nächster Ausgangspunkt
		if after, _ := AfterLevel(e, rangeMl); after != nil {
			last, added = &point{level: *after, total: e.Total}, 0
		} else if e.LevelBefore != nil || e.Kind == KindOilChange {
			last, added = nil, 0 // Stand danach unbekannt: Kette unterbrochen
		} else {
			added += e.AddedMl
		}
	}
}

// Per1000 rechnet Verbrauch je Distanz in „je 1 000 km“ bzw. „je 1 000 mi“ um.
func Per1000(value float64, distM int64, distUnit string) float64 {
	per := 1_000_000.0
	if distUnit == "mi" {
		per = 1_609_344
	}
	return value / float64(distM) * per
}

// Stats ist das Ergebnis OI-01/OI-03 für einen Zeitraum.
type Stats struct {
	AddedMl          int64
	TopUps           int
	OilChanges       int
	ChangeFillMl     int64
	HasChangeFill    bool
	OilChangeInRange bool
}

// Totals berechnet OI-03 für Einträge im Zeitraum [from, to) (Zeitpunkte).
func Totals(entries []Entry, from, to time.Time) Stats {
	var s Stats
	for _, e := range entries {
		if e.At.Before(from) || !e.At.Before(to) {
			continue
		}
		switch e.Kind {
		case KindTopUp:
			s.AddedMl += e.AddedMl
			s.TopUps++
		case KindOilChange:
			s.OilChanges++
			s.OilChangeInRange = true
			if e.FillMl > 0 {
				s.ChangeFillMl += e.FillMl
				s.HasChangeFill = true
			}
		}
	}
	return s
}

// Median der Werte.
func Median(v []float64) float64 {
	c := append([]float64{}, v...)
	sort.Float64s(c)
	n := len(c)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return c[n/2]
	}
	return (c[n/2-1] + c[n/2]) / 2
}

// LocalDay ist das Kalenderdatum eines Eintrags.
func LocalDay(e Entry) time.Time { return kernel.LocalDate(e.At, e.TimeZone) }
