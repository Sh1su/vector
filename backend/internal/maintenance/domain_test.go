package maintenance

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func date(y, m, d int) time.Time { return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC) }
func km(v int64) *int64          { x := v * 1000; return &x }

var defaults = Thresholds{UpcomingDays: 30, DueDays: 7, UpcomingDistance: 1_500_000, DueDistance: 500_000}

func oilChange() (Def, []Completion) {
	return Def{Mode: ModeFromLast, Months: 12, Distance: 15_000_000, Thresholds: defaults},
		[]Completion{{ID: uuid.New(), On: date(2026, 3, 10), Total: km(45000)}}
}

func TestOilChangeLevels(t *testing.T) {
	d, cs := oilChange()
	// M-1, M-2
	st := Evaluate(d, cs, Env{Today: date(2026, 9, 30), Current: km(58700)})
	if !st.DueDate.Equal(date(2027, 3, 10)) || *st.DueTotal != 60_000_000 || st.Level != LevelUpcoming || st.Reason != "distance" || *st.DaysRemaining != 161 {
		t.Fatalf("M-1/M-2: %+v", st)
	}
	// M-3, M-4
	for cur, want := range map[int64]string{59600: LevelDue, 60000: LevelDue, 60001: LevelOverdue} {
		if st := Evaluate(d, cs, Env{Today: date(2026, 9, 30), Current: km(cur)}); st.Level != want {
			t.Errorf("M-3/4 at %d: %s", cur, st.Level)
		}
	}
	// M-6
	if st := Evaluate(d, cs, Env{Today: date(2027, 3, 10), Current: km(50000)}); st.Level != LevelDue || *st.DaysRemaining != 0 {
		t.Fatalf("M-6: %+v", st)
	}
	if st := Evaluate(d, cs, Env{Today: date(2027, 3, 11), Current: km(50000)}); st.Level != LevelOverdue {
		t.Fatalf("M-6 next day: %+v", st)
	}
	// M-8: neue Erledigung
	cs2 := append(cs, Completion{ID: uuid.New(), On: date(2027, 3, 12), Total: km(60400)})
	st = Evaluate(d, cs2, Env{Today: date(2027, 3, 12), Current: km(60400)})
	if !st.DueDate.Equal(date(2028, 3, 12)) || *st.DueTotal != 75_400_000 {
		t.Fatalf("M-8: %+v", st)
	}
	// Stand unbekannt: nur Zeit
	if st := Evaluate(d, cs, Env{Today: date(2026, 9, 30)}); st.Level != LevelOK || st.Reason != "time" {
		t.Fatalf("unknown odometer: %+v", st)
	}
}

func TestFixedGrid(t *testing.T) {
	// M-5
	a := date(2026, 1, 31)
	d := Def{Mode: ModeGrid, Months: 1, AnchorDate: &a, Thresholds: defaults}
	if st := Evaluate(d, nil, Env{Today: date(2026, 2, 1)}); !st.DueDate.Equal(date(2026, 2, 28)) {
		t.Fatalf("M-5 first: %v", st.DueDate)
	}
	cs := []Completion{{ID: uuid.New(), On: date(2026, 2, 28)}}
	if st := Evaluate(d, cs, Env{Today: date(2026, 3, 1)}); !st.DueDate.Equal(date(2026, 3, 31)) {
		t.Fatalf("M-5 second: %v", st.DueDate)
	}
	cs = append(cs, Completion{ID: uuid.New(), On: date(2026, 3, 31)})
	if st := Evaluate(d, cs, Env{Today: date(2026, 4, 1)}); !st.DueDate.Equal(date(2026, 4, 30)) {
		t.Fatalf("M-5 third: %v", st.DueDate)
	}
	// M-12
	a2 := date(2025, 4, 1)
	g := Def{Mode: ModeGrid, Months: 12, Distance: 15_000_000, AnchorDate: &a2, AnchorTotal: km(35000), Thresholds: defaults}
	st := Evaluate(g, []Completion{{ID: uuid.New(), On: date(2026, 3, 10), Total: km(45000)}}, Env{Today: date(2026, 3, 10), Current: km(45000)})
	if !st.DueDate.Equal(date(2027, 4, 1)) || *st.DueTotal != 65_000_000 {
		t.Fatalf("M-12: %+v", st)
	}
	// M-13
	st = Evaluate(g, []Completion{{ID: uuid.New(), On: date(2028, 5, 1), Total: km(83000)}}, Env{Today: date(2028, 5, 1), Current: km(83000)})
	if !st.DueDate.Equal(date(2029, 4, 1)) || *st.DueTotal != 95_000_000 {
		t.Fatalf("M-13: %+v", st)
	}
}

func TestThresholdsPerItemAndNextDue(t *testing.T) {
	today := date(2026, 1, 1)
	aA, aB := today.AddDate(0, 0, 200-365), today.AddDate(0, 0, 100-365)
	A := Def{Mode: ModeFromLast, Days: 365, AnchorDate: &aA, Thresholds: Thresholds{UpcomingDays: 365, DueDays: 7}}
	B := Def{Mode: ModeFromLast, Days: 365, AnchorDate: &aB, Thresholds: defaults}
	// M-7
	if a, b := Evaluate(A, nil, Env{Today: today}), Evaluate(B, nil, Env{Today: today}); a.Level != LevelUpcoming || b.Level != LevelOK {
		t.Fatalf("M-7: %s %s", a.Level, b.Level)
	}
	// M-11
	aT := today.AddDate(0, 0, 30-365)
	T := Def{Mode: ModeFromLast, Days: 365, AnchorDate: &aT, Thresholds: Thresholds{UpcomingDays: 30, DueDays: 7}}
	K := Def{Mode: ModeFromLast, Distance: 10_000_000, AnchorTotal: km(41000), Thresholds: defaults}
	env := Env{Today: today, Current: km(50000), DailyRate: 50_000}
	items := []Item{{ID: uuid.New(), Status: Evaluate(T, nil, env)}, {ID: uuid.New(), Status: Evaluate(K, nil, env)}}
	SortNextDue(items)
	if items[0].Status.Reason != "distance" || !items[0].Status.Estimated || items[0].Status.Level != LevelUpcoming {
		t.Fatalf("M-11: %+v", items)
	}
	// M-9: HU überfällig bleibt überfällig
	aH := date(2023, 10, 1)
	H := Def{Mode: ModeFromLast, Months: 24, AnchorDate: &aH, Thresholds: defaults}
	if st := Evaluate(H, nil, Env{Today: date(2026, 1, 1)}); st.Level != LevelOverdue {
		t.Fatalf("M-9: %+v", st)
	}
	// once
	due := date(2026, 10, 15)
	O := Def{Mode: ModeOnce, DueDateOnce: &due, Thresholds: defaults}
	if st := Evaluate(O, []Completion{{ID: uuid.New(), On: date(2026, 10, 1)}}, Env{Today: date(2026, 10, 2)}); st.Level != LevelCompleted {
		t.Fatalf("once: %+v", st)
	}
	if st := Evaluate(Def{Mode: ModeFromLast, Distance: 1000, Thresholds: defaults}, nil, Env{Today: today}); st.Level != LevelUnknown {
		t.Fatalf("unknown: %+v", st)
	}
}
