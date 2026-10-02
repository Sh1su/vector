// Package odometer ist die einzige Quelle für Zählerstände eines Fahrzeugs
// (ADR-009, docs/phase-2/10-domaene-odometer.md).
package odometer

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

// Status eines Messpunkts.
const (
	StatusValid            = "valid"
	StatusConfirmedAnomaly = "confirmed_anomaly"
	StatusSuperseded       = "superseded"
)

// Größe, die der Zähler misst.
const (
	MeterDistance    = "distance"
	MeterEngineHours = "engine_hours"
)

// Reading ist ein Messpunkt in der für Regeln nötigen Form.
type Reading struct {
	ID         uuid.UUID
	At         time.Time
	TimeZone   string
	Precision  string
	RecordedAt time.Time
	Value      int64 // Zählerwert, kanonisch (m bzw. s)
	Status     string
	Source     string
	SourceRef  *uuid.UUID
	InputValue float64 // Originaleingabe (ADR-007)
	InputUnit  string
}

// Segment ist ein Zählerabschnitt. Abschnitt 1 beginnt implizit „immer“ (Start = Nullzeit).
type Segment struct {
	ID         uuid.UUID
	Seq        int
	StartedAt  time.Time
	StartMeter int64
	Offset     int64
}

// Less ordnet Ereignisse eindeutig: Zeitpunkt, Erfassungszeit, ID (Übersicht §3.1).
func Less(a, b Reading) bool {
	if !a.At.Equal(b.At) {
		return a.At.Before(b.At)
	}
	if !a.RecordedAt.Equal(b.RecordedAt) {
		return a.RecordedAt.Before(b.RecordedAt)
	}
	return a.ID.String() < b.ID.String()
}

// Sort sortiert Messpunkte in fachlicher Ordnung.
func Sort(rs []Reading) { sort.SliceStable(rs, func(i, j int) bool { return Less(rs[i], rs[j]) }) }

// Segments ist die sortierte Liste der Zählerabschnitte inkl. implizitem Abschnitt 1.
type Segments []Segment

// NewSegments ergänzt den impliziten ersten Abschnitt und sortiert.
func NewSegments(first uuid.UUID, stored []Segment) Segments {
	out := Segments{{ID: first, Seq: 1}}
	out = append(out, stored...)
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out
}

// At liefert den Abschnitt, der zum Zeitpunkt t gilt (I-ODO-2).
func (s Segments) At(t time.Time) Segment {
	cur := s[0]
	for _, seg := range s[1:] {
		if !seg.StartedAt.After(t) {
			cur = seg
		}
	}
	return cur
}

// Last ist der aktuelle Abschnitt.
func (s Segments) Last() Segment { return s[len(s)-1] }

// Total ist die Gesamtlaufleistung eines Messpunkts.
func (s Segments) Total(r Reading) int64 { return r.Value + s.At(r.At).Offset }

// Config enthält die Schwellen der Plausibilitätsprüfung.
type Config struct {
	VMaxKmh int // P3, Default 250 km/h
}

// CheckFuture prüft ODO-02: Zeitpunkt mehr als 5 Minuten in der Zukunft bzw.
// Kalenderdatum nach heute (date_only). Nicht bestätigbar.
func CheckFuture(r Reading, now time.Time) *problem.Anomaly {
	future := false
	if r.Precision == kernel.PrecisionDateOnly {
		future = kernel.LocalDate(r.At, r.TimeZone).After(kernel.LocalDate(now, r.TimeZone))
	} else {
		future = r.At.After(now.Add(5 * time.Minute))
	}
	if !future {
		return nil
	}
	return &problem.Anomaly{Code: "P4", Confirmable: false, Message: "Der Zeitpunkt liegt in der Zukunft."}
}

// CheckNeighbors prüft ODO-03 gegen den vorigen und den nächsten gültigen
// Messpunkt desselben Abschnitts. others darf den Kandidaten und einen gerade
// ersetzten Messpunkt nicht enthalten.
func CheckNeighbors(c Reading, others []Reading, segs Segments, meter string, cfg Config) []problem.Anomaly {
	seg := segs.At(c.At)
	var prev, next *Reading
	for i := range others {
		o := others[i]
		if o.Status == StatusSuperseded || segs.At(o.At).Seq != seg.Seq {
			continue
		}
		if Less(o, c) {
			if prev == nil || Less(*prev, o) {
				prev = &others[i]
			}
		} else if next == nil || Less(o, *next) {
			next = &others[i]
		}
	}
	var out []problem.Anomaly
	if prev != nil && c.Value < prev.Value {
		id := prev.ID
		out = append(out, problem.Anomaly{Code: "P1", Confirmable: true, RelatedID: &id,
			Message: fmt.Sprintf("Der Stand ist kleiner als der vorige Messpunkt (%s).", fmtValue(prev.Value, meter))})
	}
	if next != nil && c.Value > next.Value {
		id := next.ID
		out = append(out, problem.Anomaly{Code: "P2", Confirmable: true, RelatedID: &id,
			Message: fmt.Sprintf("Der Stand ist größer als der folgende Messpunkt (%s).", fmtValue(next.Value, meter))})
	}
	for _, n := range []*Reading{prev, next} {
		if n != nil && tooFast(c, *n, meter, cfg) {
			id := n.ID
			out = append(out, problem.Anomaly{Code: "P3", Confirmable: true, RelatedID: &id,
				Message: "Unrealistischer Sprung gegenüber einem benachbarten Messpunkt."})
			break
		}
	}
	return out
}

// tooFast: Geschwindigkeit zum Nachbarn über v_max (Distanz) bzw. Betriebszeit
// größer als die verstrichene Zeit (Motorstunden). Bei date_only gilt der
// größtmögliche Abstand (Kalendertage + 1) × 24 h, sonst mindestens 1 Minute.
func tooFast(a, b Reading, meter string, cfg Config) bool {
	dv := math.Abs(float64(a.Value - b.Value))
	if dv == 0 {
		return false
	}
	var dt time.Duration
	if a.Precision == kernel.PrecisionDateOnly || b.Precision == kernel.PrecisionDateOnly {
		da, db := kernel.LocalDate(a.At, a.TimeZone), kernel.LocalDate(b.At, b.TimeZone)
		days := math.Abs(da.Sub(db).Hours() / 24)
		dt = time.Duration(days+1) * 24 * time.Hour
	} else {
		dt = a.At.Sub(b.At)
		if dt < 0 {
			dt = -dt
		}
		if dt < time.Minute {
			dt = time.Minute
		}
	}
	if meter == MeterEngineHours {
		return dv > dt.Seconds()
	}
	vmax := cfg.VMaxKmh
	if vmax <= 0 {
		vmax = 250
	}
	kmh := (dv / 1000) / dt.Hours()
	return kmh > float64(vmax)
}

func fmtValue(v int64, meter string) string {
	if meter == MeterEngineHours {
		return groupDE(float64(v)/3600) + " h"
	}
	return groupDE(float64(v)/1000) + " km"
}

// groupDE formatiert eine Zahl deutsch mit Tausenderpunkt und höchstens einer
// Nachkommastelle (Meldungstexte; die API-Werte bleiben sprachunabhängig).
func groupDE(f float64) string {
	neg := f < 0
	if neg {
		f = -f
	}
	tenths := int64(math.Round(f * 10))
	whole, frac := tenths/10, tenths%10
	s := fmt.Sprint(whole)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	if frac != 0 {
		s += "," + fmt.Sprint(frac)
	}
	if neg {
		s = "-" + s
	}
	return s
}

// ValueKind beschreibt, wie ein Stand zu einem Zeitpunkt zustande kam (ODO-06).
type ValueKind string

const (
	KindExact          ValueKind = "exact"
	KindInterpolated   ValueKind = "interpolated"
	KindCarriedForward ValueKind = "carried_forward"
	KindUnknown        ValueKind = "unknown"
)

// Value ist ein Stand zu einem Zeitpunkt.
type Value struct {
	At        time.Time
	Kind      ValueKind
	Meter     int64
	Total     int64
	SegmentID uuid.UUID
	ReadingID *uuid.UUID
}

// ValueAt berechnet den Stand zum Zeitpunkt t (ODO-06). valid muss nach
// fachlicher Ordnung sortiert sein und nur gültige Messpunkte enthalten.
func ValueAt(valid []Reading, segs Segments, t time.Time) Value {
	v := Value{At: t, Kind: KindUnknown}
	if len(valid) == 0 {
		return v
	}
	seg := segs.At(t)
	v.SegmentID = seg.ID
	// letzter Messpunkt mit At <= t, erster mit At > t
	idx := sort.Search(len(valid), func(i int) bool { return valid[i].At.After(t) })
	if idx == 0 {
		return v // vor dem ersten Messpunkt: unbekannt
	}
	a := valid[idx-1]
	ta := segs.Total(a)
	switch {
	case a.At.Equal(t):
		v.Kind, v.Total = KindExact, ta
		id := a.ID
		v.ReadingID = &id
	case idx == len(valid):
		v.Kind, v.Total = KindCarriedForward, ta
	default:
		b := valid[idx]
		tb := segs.Total(b)
		v.Kind = KindInterpolated
		if tb < ta {
			v.Total = ta // bestätigte rückläufige Anomalie: stückweise konstant
		} else {
			num := new(big.Rat).SetInt64((tb - ta))
			num.Mul(num, big.NewRat(t.Sub(a.At).Nanoseconds(), b.At.Sub(a.At).Nanoseconds()))
			v.Total = ta + kernel.RoundHalfEven(num)
		}
	}
	v.Meter = v.Total - seg.Offset
	return v
}

// Current liefert den aktuellen Stand (ODO-04): spätester gültiger Messpunkt im
// aktuellen Abschnitt; ohne Messpunkt im Abschnitt dessen Startwert; sonst unbekannt.
func Current(valid []Reading, segs Segments) Value {
	last := segs.Last()
	for i := len(valid) - 1; i >= 0; i-- {
		r := valid[i]
		if segs.At(r.At).Seq == last.Seq {
			id := r.ID
			return Value{At: r.At, Kind: KindExact, Meter: r.Value, Total: r.Value + last.Offset, SegmentID: last.ID, ReadingID: &id}
		}
	}
	if last.Seq > 1 {
		return Value{At: last.StartedAt, Kind: KindExact, Meter: last.StartMeter, Total: last.StartMeter + last.Offset, SegmentID: last.ID}
	}
	return Value{Kind: KindUnknown, SegmentID: last.ID}
}

// Distance ist das Ergebnis von ODO-05.
type Distance struct {
	Known      bool
	Meters     int64
	FromKind   ValueKind
	ToKind     ValueKind
	HasAnomaly bool
}

// DistanceBetween berechnet die Strecke zwischen t1 und t2 (ODO-05).
func DistanceBetween(valid []Reading, segs Segments, t1, t2 time.Time) Distance {
	a, b := ValueAt(valid, segs, t1), ValueAt(valid, segs, t2)
	d := Distance{FromKind: a.Kind, ToKind: b.Kind}
	if a.Kind == KindUnknown || b.Kind == KindUnknown {
		return d
	}
	d.Known, d.Meters = true, b.Total-a.Total
	for _, r := range valid {
		if r.Status == StatusConfirmedAnomaly && !r.At.Before(t1) && !r.At.After(t2) {
			d.HasAnomaly = true
		}
	}
	return d
}

// NewSegmentOffset berechnet den Versatz eines neuen Abschnitts (ODO-07):
// offset_neu = (letzter Zählerwert alt + offset_alt) − Startwert neu.
func NewSegmentOffset(oldFinal, oldOffset, startMeter int64) int64 {
	return oldFinal + oldOffset - startMeter
}
