// Package maintenance beschreibt, was wann fällig wird: Wartungsdefinitionen,
// Erledigungen und die berechnete Fälligkeit (docs/phase-2/10-domaene-maintenance.md).
package maintenance

import (
	"math"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/sh1su/vector/backend/internal/kernel"
)

// Stufen (MA-04), aufsteigend nach Dringlichkeit.
const (
	LevelCompleted = "completed"
	LevelUnknown   = "unknown"
	LevelOK        = "ok"
	LevelUpcoming  = "upcoming"
	LevelDue       = "due"
	LevelOverdue   = "overdue"
)

var levelRank = map[string]int{LevelCompleted: 0, LevelUnknown: 1, LevelOK: 2, LevelUpcoming: 3, LevelDue: 4, LevelOverdue: 5}

// Rank liefert die Rangfolge einer Stufe (höher = dringender).
func Rank(level string) int { return levelRank[level] }

// Planungsarten.
const (
	ModeOnce          = "once"
	ModeFromLast      = "from_last_completion"
	ModeFixedGrid     = "fixed_grid"
	TriggerTime       = "time"
	TriggerDistance   = "distance"
	CompletionDone    = "done"
	CompletionSkipped = "skipped"
)

// Thresholds sind die aufgelösten Schwellen einer Definition (MA-05).
type Thresholds struct {
	UpcomingDays, DueDays         int
	UpcomingDistance, DueDistance int64
}

// DefaultThresholds sind die Installationsvorgaben (MA-05): 30/7 Tage und
// 1 500/500 km bzw. 20/5 h bei Stundenzählern.
func DefaultThresholds(engineHours bool) Thresholds {
	if engineHours {
		return Thresholds{UpcomingDays: 30, DueDays: 7, UpcomingDistance: 20 * 3600, DueDistance: 5 * 3600}
	}
	return Thresholds{UpcomingDays: 30, DueDays: 7, UpcomingDistance: 1_500_000, DueDistance: 500_000}
}

// Item ist eine Definition in Rechenform (Distanzen kanonisch als Gesamtlaufleistung).
type Item struct {
	Mode         string
	Months, Days int
	Distance     int64 // 0 = kein Distanzintervall
	AnchorDate   *time.Time
	AnchorTotal  *int64
	DueDateOnce  *time.Time
	DueTotalOnce *int64
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

// Env ist die Lage zum Bewertungszeitpunkt.
type Env struct {
	Today     time.Time // Kalenderdatum in owner_time_zone
	Current   *int64    // aktuelle Gesamtlaufleistung (ODO-04), nil = unbekannt
	DailyRate float64   // kanonische Einheiten je Tag (ODO-08)
	RateKnown bool
}

// Status ist das Ergebnis der Bewertung (API DueStatus).
type Status struct {
	Level         string
	Reason        string // time | distance | ""
	DueDate       *time.Time
	DaysRemaining *int
	DueTotal      *int64
	Remaining     *int64 // Rest in kanonischen Einheiten
	EstimatedDate *time.Time
	Estimated     bool
}

type trigger struct {
	kind      string
	level     string
	estimated *time.Time
}

func (it Item) hasTime() bool {
	if it.Mode == ModeOnce {
		return it.DueDateOnce != nil
	}
	return it.Months > 0 || it.Days > 0
}

func (it Item) hasDistance() bool {
	if it.Mode == ModeOnce {
		return it.DueTotalOnce != nil
	}
	return it.Distance > 0
}

func (it Item) datePoint(anchor time.Time, k int) time.Time {
	if it.Months > 0 {
		return kernel.AddMonths(anchor, k*it.Months)
	}
	return anchor.AddDate(0, 0, k*it.Days)
}

// largestDateK ist der größte Rasterindex k ≥ 0 mit g_k ≤ d, sonst −1.
func (it Item) largestDateK(anchor, d time.Time) int {
	if d.Before(anchor) {
		return -1
	}
	var k int
	if it.Months > 0 {
		k = ((d.Year()-anchor.Year())*12 + int(d.Month()) - int(anchor.Month())) / it.Months
	} else {
		k = kernel.DaysBetween(anchor, d) / it.Days
	}
	for k > 0 && it.datePoint(anchor, k).After(d) {
		k--
	}
	for !it.datePoint(anchor, k+1).After(d) {
		k++
	}
	return k
}

// gridNext wertet die Erledigungen gegen das Raster aus (MA-02, fixed_grid):
// jede Erledigung deckt max(nächster offener Punkt, größter Punkt ≤ Erledigung).
func gridNext(largestK func(i int) int, n int) int {
	next := 1
	for i := 0; i < n; i++ {
		c := largestK(i)
		if c < next {
			c = next
		}
		next = c + 1
	}
	return next
}

// DueDate berechnet fällig_am (MA-02); nil = nicht bewertbar.
func (it Item) DueDate(cs []Completion) *time.Time {
	switch it.Mode {
	case ModeOnce:
		return it.DueDateOnce
	case ModeFromLast:
		var base *time.Time
		if len(cs) > 0 {
			b := cs[len(cs)-1].On
			base = &b
		} else {
			base = it.AnchorDate
		}
		if base == nil {
			return nil
		}
		d := it.datePoint(*base, 1)
		return &d
	case ModeFixedGrid:
		if it.AnchorDate == nil {
			return nil
		}
		a := *it.AnchorDate
		next := gridNext(func(i int) int { return it.largestDateK(a, cs[i].On) }, len(cs))
		d := it.datePoint(a, next)
		return &d
	}
	return nil
}

// DueTotal berechnet fällig_bei (MA-03); Erledigungen ohne Stand müssen vorher
// über ValueAt ergänzt sein. nil = nicht bewertbar.
func (it Item) DueTotal(cs []Completion) *int64 {
	switch it.Mode {
	case ModeOnce:
		return it.DueTotalOnce
	case ModeFromLast:
		var base *int64
		if len(cs) > 0 {
			base = cs[len(cs)-1].Total
		} else {
			base = it.AnchorTotal
		}
		if base == nil {
			return nil
		}
		v := *base + it.Distance
		return &v
	case ModeFixedGrid:
		if it.AnchorTotal == nil {
			return nil
		}
		a := *it.AnchorTotal
		next := gridNext(func(i int) int {
			t := cs[i].Total
			if t == nil || *t < a {
				return -1
			}
			return int((*t - a) / it.Distance)
		}, len(cs))
		v := a + int64(next)*it.Distance
		return &v
	}
	return nil
}

// Evaluate bewertet eine Definition (MA-04 bis MA-07). cs muss nach MA-01 sortiert sein.
func Evaluate(it Item, cs []Completion, env Env) Status {
	if it.Mode == ModeOnce && len(cs) > 0 {
		return Status{Level: LevelCompleted}
	}
	var st Status
	var trigs []trigger
	if it.hasTime() {
		if due := it.DueDate(cs); due != nil {
			rem := kernel.DaysBetween(env.Today, *due)
			st.DueDate, st.DaysRemaining = due, &rem
			lv := LevelOK
			switch {
			case env.Today.After(*due):
				lv = LevelOverdue
			case rem <= it.Thresholds.DueDays:
				lv = LevelDue
			case rem <= it.Thresholds.UpcomingDays:
				lv = LevelUpcoming
			}
			d := *due
			trigs = append(trigs, trigger{kind: TriggerTime, level: lv, estimated: &d})
		}
	}
	if it.hasDistance() {
		if due := it.DueTotal(cs); due != nil {
			st.DueTotal = due
			if env.Current != nil {
				rem := *due - *env.Current
				st.Remaining = &rem
				lv := LevelOK
				switch {
				case *env.Current > *due:
					lv = LevelOverdue
				case rem <= it.Thresholds.DueDistance:
					lv = LevelDue
				case rem <= it.Thresholds.UpcomingDistance:
					lv = LevelUpcoming
				}
				t := trigger{kind: TriggerDistance, level: lv}
				if env.RateKnown && env.DailyRate > 0 {
					e := env.Today.AddDate(0, 0, int(math.Ceil(float64(rem)/env.DailyRate)))
					t.estimated = &e
				}
				trigs = append(trigs, t)
			}
		}
	}
	if len(trigs) == 0 {
		st.Level = LevelUnknown
		return st
	}
	best := trigs[0]
	for _, t := range trigs[1:] {
		if Rank(t.level) > Rank(best.level) || (Rank(t.level) == Rank(best.level) && earlier(t.estimated, best.estimated)) {
			best = t
		}
	}
	st.Level, st.Reason = best.level, best.kind
	// geschätztes Datum: das frühere der Auslöser (MA-07)
	for _, t := range trigs {
		if t.estimated != nil && (st.EstimatedDate == nil || t.estimated.Before(*st.EstimatedDate)) {
			e := *t.estimated
			st.EstimatedDate, st.Estimated = &e, t.kind == TriggerDistance
		}
	}
	return st
}

func earlier(a, b *time.Time) bool {
	if a == nil {
		return false
	}
	return b == nil || a.Before(*b)
}

// Ranked ist eine bewertete Definition für die Sortierung nach MA-07.
type Ranked struct {
	ItemID uuid.UUID
	Status Status
}

// SortNextDue sortiert nach Stufe absteigend, dann geschätztem Datum aufsteigend;
// Definitionen ohne Datum stehen am Ende ihrer Stufe (MA-07).
func SortNextDue(rs []Ranked) {
	sort.SliceStable(rs, func(i, j int) bool {
		a, b := rs[i].Status, rs[j].Status
		if Rank(a.Level) != Rank(b.Level) {
			return Rank(a.Level) > Rank(b.Level)
		}
		return earlier(a.EstimatedDate, b.EstimatedDate)
	})
}
