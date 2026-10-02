// Package maintenance beschreibt, was wann fällig wird, und berechnet
// Fälligkeit und Dringlichkeit (docs/phase-2/10-domaene-maintenance.md).
package maintenance

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

// Stufen (MA-04), dazu unknown (nicht bewertbar) und completed (einmalig erledigt).
const (
	LevelCompleted = "completed"
	LevelUnknown   = "unknown"
	LevelOK        = "ok"
	LevelUpcoming  = "upcoming"
	LevelDue       = "due"
	LevelOverdue   = "overdue"
)

var rank = map[string]int{LevelCompleted: -1, LevelUnknown: 0, LevelOK: 1, LevelUpcoming: 2, LevelDue: 3, LevelOverdue: 4}

// Rank liefert die Ordnung einer Stufe.
func Rank(level string) int { return rank[level] }

// Planungsarten.
const (
	ModeOnce     = "once"
	ModeFromLast = "from_last_completion"
	ModeGrid     = "fixed_grid"
)

// Thresholds sind die aufgelösten Schwellen einer Definition (MA-05).
type Thresholds struct {
	UpcomingDays, DueDays         int
	UpcomingDistance, DueDistance int64
}

// Def ist eine Definition in der für die Regeln nötigen Form. Daten sind
// Kalenderdaten (UTC-Mitternacht) in der Zeitzone des Fahrzeugs.
type Def struct {
	Mode         string
	Months, Days int
	Distance     int64
	AnchorDate   *time.Time
	AnchorTotal  *int64
	DueDateOnce  *time.Time
	DueTotalOnce *int64
	Created      time.Time
	Thresholds   Thresholds
}

// Completion ist eine Erledigung (done oder skipped zählen gleich, MA-01).
type Completion struct {
	ID    uuid.UUID
	On    time.Time
	Total *int64
}

// SortCompletions ordnet nach Datum, Stand, ID (MA-01).
func SortCompletions(cs []Completion) {
	sort.SliceStable(cs, func(i, j int) bool {
		a, b := cs[i], cs[j]
		if !a.On.Equal(b.On) {
			return a.On.Before(b.On)
		}
		at, bt := int64(-1), int64(-1)
		if a.Total != nil {
			at = *a.Total
		}
		if b.Total != nil {
			bt = *b.Total
		}
		if at != bt {
			return at < bt
		}
		return a.ID.String() < b.ID.String()
	})
}

// Env ist der Bewertungskontext.
type Env struct {
	Today     time.Time
	Current   *int64                     // aktuelle Gesamtlaufleistung (ODO-04), nil = unbekannt
	DailyRate float64                    // ODO-08, 0 = unbekannt
	ValueAt   func(day time.Time) *int64 // ODO-06 am Tag 12:00
}

// Status ist das Ergebnis MA-04 bis MA-07.
type Status struct {
	Level             string
	Reason            string // time | distance | ""
	DueDate           *time.Time
	DaysRemaining     *int
	DueTotal          *int64
	DistanceRemaining *int64
	EstimatedDate     *time.Time
	Estimated         bool
}

// AddMonths addiert Monate; fehlt der Zieltag, gilt der Monatsletzte (MA-02).
func AddMonths(d time.Time, n int) time.Time {
	y, m := d.Year(), int(d.Month())-1+n
	y += m / 12
	m %= 12
	if m < 0 {
		m += 12
		y--
	}
	last := time.Date(y, time.Month(m+2), 0, 0, 0, 0, 0, time.UTC).Day()
	day := d.Day()
	if day > last {
		day = last
	}
	return time.Date(y, time.Month(m+1), day, 0, 0, 0, 0, time.UTC)
}

func (d Def) hasTime() bool { return d.Months > 0 || d.Days > 0 }

func (d Def) addTime(base time.Time, k int) time.Time {
	if d.Months > 0 {
		return AddMonths(base, k*d.Months)
	}
	return base.AddDate(0, 0, k*d.Days)
}

func days(a, b time.Time) int { return int(a.Sub(b).Hours() / 24) }

// gridIndex: nächster offener Rasterpunkt nach den Erledigungen (MA-02 fixed_grid).
func gridIndex(values []int64, point func(k int) int64) int {
	next := 1
	for _, v := range values {
		kc := 0
		for point(kc+1) <= v {
			kc++
		}
		if kc > next {
			next = kc
		}
		next++
	}
	return next
}

// dueDate berechnet die nächste Fälligkeit nach Zeit (MA-02).
func dueDate(d Def, cs []Completion) *time.Time {
	if d.Mode == ModeOnce {
		return d.DueDateOnce
	}
	if !d.hasTime() {
		return nil
	}
	anchor := d.Created
	if d.AnchorDate != nil {
		anchor = *d.AnchorDate
	}
	if d.Mode == ModeGrid {
		var vals []int64
		for _, c := range cs {
			vals = append(vals, c.On.Unix())
		}
		k := gridIndex(vals, func(k int) int64 { return d.addTime(anchor, k).Unix() })
		t := d.addTime(anchor, k)
		return &t
	}
	base := anchor
	if len(cs) > 0 {
		base = cs[len(cs)-1].On
	}
	t := d.addTime(base, 1)
	return &t
}

// dueTotal berechnet die nächste Fälligkeit nach Distanz (MA-03).
func dueTotal(d Def, cs []Completion, env Env) *int64 {
	if d.Mode == ModeOnce {
		return d.DueTotalOnce
	}
	if d.Distance <= 0 {
		return nil
	}
	totalOf := func(c Completion) *int64 {
		if c.Total != nil {
			return c.Total
		}
		if env.ValueAt != nil {
			return env.ValueAt(c.On)
		}
		return nil
	}
	if d.Mode == ModeGrid {
		if d.AnchorTotal == nil {
			return nil
		}
		var vals []int64
		for _, c := range cs {
			if t := totalOf(c); t != nil {
				vals = append(vals, *t)
			}
		}
		k := gridIndex(vals, func(k int) int64 { return *d.AnchorTotal + int64(k)*d.Distance })
		v := *d.AnchorTotal + int64(k)*d.Distance
		return &v
	}
	var base *int64
	if len(cs) > 0 {
		base = totalOf(cs[len(cs)-1])
	} else {
		base = d.AnchorTotal
	}
	if base == nil {
		return nil
	}
	v := *base + d.Distance
	return &v
}

func levelFor(rem, dueTh, upTh int64) string {
	switch {
	case rem < 0:
		return LevelOverdue
	case rem <= dueTh:
		return LevelDue
	case rem <= upTh:
		return LevelUpcoming
	}
	return LevelOK
}

// Evaluate bewertet eine Definition (MA-01 bis MA-07). cs muss nach MA-01 sortiert sein.
func Evaluate(d Def, cs []Completion, env Env) Status {
	if d.Mode == ModeOnce && len(cs) > 0 {
		return Status{Level: LevelCompleted}
	}
	st := Status{Level: LevelUnknown}
	timeLevel, distLevel := "", ""
	var timeEst, distEst *time.Time
	if dd := dueDate(d, cs); dd != nil {
		rem := days(*dd, env.Today)
		st.DueDate, st.DaysRemaining = dd, &rem
		timeLevel = levelFor(int64(rem), int64(d.Thresholds.DueDays), int64(d.Thresholds.UpcomingDays))
		timeEst = dd
	}
	if dt := dueTotal(d, cs, env); dt != nil {
		st.DueTotal = dt
		if env.Current != nil {
			rem := *dt - *env.Current
			st.DistanceRemaining = &rem
			distLevel = levelFor(rem, d.Thresholds.DueDistance, d.Thresholds.UpcomingDistance)
			if env.DailyRate > 0 {
				dd := 0
				if rem > 0 {
					dd = int(float64(rem) / env.DailyRate)
				}
				t := env.Today.AddDate(0, 0, dd)
				distEst = &t
			} else if rem < 0 {
				t := env.Today
				distEst = &t
			}
		}
	}
	switch {
	case timeLevel == "" && distLevel == "":
		return st
	case distLevel == "" || (timeLevel != "" && Rank(timeLevel) > Rank(distLevel)):
		st.Level, st.Reason = timeLevel, "time"
	case timeLevel == "" || Rank(distLevel) > Rank(timeLevel):
		st.Level, st.Reason = distLevel, "distance"
	default: // gleiche Stufe: der Auslöser, der nach Prognose zuerst erreicht wird
		st.Level, st.Reason = timeLevel, "time"
		if distEst != nil && timeEst != nil && distEst.Before(*timeEst) {
			st.Reason = "distance"
		}
	}
	// MA-07: geschätztes Datum = das frühere
	switch {
	case timeEst != nil && (distEst == nil || !distEst.Before(*timeEst)):
		st.EstimatedDate = timeEst
	case distEst != nil:
		st.EstimatedDate, st.Estimated = distEst, true
	}
	return st
}

// Item ist eine bewertete Definition für die Sortierung.
type Item struct {
	ID     uuid.UUID
	Status Status
}

// SortNextDue ordnet nach Stufe absteigend, dann geschätztem Datum (MA-07);
// reine Distanzdefinitionen ohne Prognose stehen am Ende ihrer Stufe.
func SortNextDue(items []Item) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].Status, items[j].Status
		if Rank(a.Level) != Rank(b.Level) {
			return Rank(a.Level) > Rank(b.Level)
		}
		if a.EstimatedDate == nil || b.EstimatedDate == nil {
			return a.EstimatedDate != nil
		}
		return a.EstimatedDate.Before(*b.EstimatedDate)
	})
}
