package maintenance

import (
	"testing"
	"time"

	"github.com/sh1su/vector/backend/internal/kernel"
)

func d(s string) time.Time   { t, _ := kernel.ParseDate(s); return t }
func dp(s string) *time.Time { t := d(s); return &t }
func km(v float64) *int64    { m := int64(v * 1000); return &m }

// Ölwechsel: from_last_completion, 12 Monate / 15 000 km, erledigt 10.03.2026 bei 45 000 km.
func oil() (Item, []Completion) {
	return Item{Mode: ModeFromLast, Months: 12, Distance: 15_000_000, Thresholds: DefaultThresholds(false)},
		[]Completion{{On: d("2026-03-10"), Total: km(45000)}}
}

func env(today string, current float64) Env { return Env{Today: d(today), Current: km(current)} }

func TestM1toM4(t *testing.T) {
	it, cs := oil()
	if got := kernel.FormatDate(*it.DueDate(cs)); got != "2027-03-10" || *it.DueTotal(cs) != 60_000_000 {
		t.Fatalf("M-1: %s %d", got, *it.DueTotal(cs))
	}
	st := Evaluate(it, cs, env("2026-09-30", 58700))
	if st.Level != LevelUpcoming || st.Reason != TriggerDistance || *st.DaysRemaining != 161 || *st.Remaining != 1_300_000 {
		t.Fatalf("M-2: %+v", st)
	}
	if st := Evaluate(it, cs, env("2026-09-30", 59600)); st.Level != LevelDue {
		t.Fatalf("M-3: %+v", st)
	}
	if st := Evaluate(it, cs, env("2026-09-30", 60000)); st.Level != LevelDue {
		t.Fatalf("M-4 Gleichstand: %+v", st)
	}
	if st := Evaluate(it, cs, env("2026-09-30", 60001)); st.Level != LevelOverdue {
		t.Fatalf("M-4 +1: %+v", st)
	}
}

func TestM5FixedGridMonthEnd(t *testing.T) {
	it := Item{Mode: ModeFixedGrid, Months: 1, AnchorDate: dp("2026-01-31"), Thresholds: DefaultThresholds(false)}
	var cs []Completion
	want := []string{"2026-02-28", "2026-03-31", "2026-04-30"}
	for i, w := range want {
		if got := kernel.FormatDate(*it.DueDate(cs)); got != w {
			t.Fatalf("M-5 step %d: %s want %s", i, got, w)
		}
		cs = append(cs, Completion{On: *it.DueDate(cs)})
	}
}

func TestM6DueDay(t *testing.T) {
	it, cs := oil()
	it.Distance = 0
	st := Evaluate(it, cs, env("2027-03-10", 0))
	if st.Level != LevelDue || *st.DaysRemaining != 0 {
		t.Fatalf("M-6 day: %+v", st)
	}
	if st := Evaluate(it, cs, env("2027-03-11", 0)); st.Level != LevelOverdue {
		t.Fatalf("M-6 next day: %+v", st)
	}
}

// M-7: Schwellen gelten nur für die eigene Definition.
func TestM7OwnThresholds(t *testing.T) {
	a := Item{Mode: ModeOnce, DueDateOnce: dp("2027-04-18"), Thresholds: Thresholds{UpcomingDays: 365, DueDays: 7}}
	b := Item{Mode: ModeOnce, DueDateOnce: dp("2027-01-08"), Thresholds: DefaultThresholds(false)}
	e := env("2026-09-30", 0)
	if Evaluate(a, nil, e).Level != LevelUpcoming || Evaluate(b, nil, e).Level != LevelOK {
		t.Fatal("M-7")
	}
}

func TestM8NewCompletion(t *testing.T) {
	it, cs := oil()
	cs = append(cs, Completion{On: d("2027-03-12"), Total: km(60400)})
	SortCompletions(cs)
	if kernel.FormatDate(*it.DueDate(cs)) != "2028-03-12" || *it.DueTotal(cs) != 75_400_000 {
		t.Fatal("M-8")
	}
}

func TestM9StaysOverdue(t *testing.T) {
	it := Item{Mode: ModeFromLast, Months: 24, Thresholds: DefaultThresholds(false)}
	cs := []Completion{{On: d("2024-06-30")}}
	if st := Evaluate(it, cs, env("2026-09-30", 0)); st.Level != LevelOverdue {
		t.Fatalf("M-9: %+v", st)
	}
}

func TestM11NextDue(t *testing.T) {
	e := Env{Today: d("2026-09-30"), Current: km(50000), DailyRate: 50_000, RateKnown: true}
	timeItem := Item{Mode: ModeOnce, DueDateOnce: dp("2026-10-30"), Thresholds: DefaultThresholds(false)}
	kmItem := Item{Mode: ModeOnce, DueTotalOnce: km(51000), Thresholds: DefaultThresholds(false)}
	rs := []Ranked{{Status: Evaluate(timeItem, nil, e)}, {Status: Evaluate(kmItem, nil, e)}}
	rs[1].ItemID[0] = 1
	SortNextDue(rs)
	if rs[0].ItemID[0] != 1 || !rs[0].Status.Estimated || kernel.FormatDate(*rs[0].Status.EstimatedDate) != "2026-10-20" {
		t.Fatalf("M-11: %+v", rs)
	}
}

func grid() Item {
	return Item{Mode: ModeFixedGrid, Months: 12, Distance: 15_000_000, AnchorDate: dp("2025-04-01"), AnchorTotal: km(35000), Thresholds: DefaultThresholds(false)}
}

func TestM12EarlyCompletion(t *testing.T) {
	it := grid()
	if kernel.FormatDate(*it.DueDate(nil)) != "2026-04-01" || *it.DueTotal(nil) != 50_000_000 {
		t.Fatal("M-12 first")
	}
	cs := []Completion{{On: d("2026-03-10"), Total: km(45000)}}
	if kernel.FormatDate(*it.DueDate(cs)) != "2027-04-01" || *it.DueTotal(cs) != 65_000_000 {
		t.Fatalf("M-12: %s %d", kernel.FormatDate(*it.DueDate(cs)), *it.DueTotal(cs))
	}
}

func TestM13LateCompletion(t *testing.T) {
	it := grid()
	cs := []Completion{{On: d("2028-05-01"), Total: km(83000)}}
	if kernel.FormatDate(*it.DueDate(cs)) != "2029-04-01" || *it.DueTotal(cs) != 95_000_000 {
		t.Fatalf("M-13: %s %d", kernel.FormatDate(*it.DueDate(cs)), *it.DueTotal(cs))
	}
}

func TestOnceCompletedAndUnknown(t *testing.T) {
	it := Item{Mode: ModeOnce, DueTotalOnce: km(1000), Thresholds: DefaultThresholds(false)}
	if Evaluate(it, nil, Env{Today: d("2026-01-01")}).Level != LevelUnknown {
		t.Fatal("unknown without odometer")
	}
	if Evaluate(it, []Completion{{On: d("2026-01-01")}}, Env{Today: d("2026-01-01")}).Level != LevelCompleted {
		t.Fatal("completed")
	}
}
