package oil

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

func at(m, d int) time.Time { return time.Date(2026, time.Month(m), d, 12, 0, 0, 0, time.UTC) }

func example() []Entry {
	t := func(km int64) *int64 { v := km * 1000; return &v }
	f := func(v float64) *float64 { return &v }
	es := []Entry{
		{ID: uuid.New(), Kind: KindCheck, At: at(2, 20), Total: t(49800), LevelBefore: f(60)},
		{ID: uuid.New(), Kind: KindOilChange, At: at(3, 1), Total: t(50000), LevelAfter: f(100), FillMl: 4300},
		{ID: uuid.New(), Kind: KindCheck, At: at(4, 1), Total: t(51500), LevelBefore: f(70)},
		{ID: uuid.New(), Kind: KindTopUp, At: at(4, 20), Total: t(52500), LevelBefore: f(40), AddedMl: 500},
		{ID: uuid.New(), Kind: KindCheck, At: at(5, 15), Total: t(54000), LevelBefore: f(60)},
	}
	return es
}

func r(v float64) float64 { return math.Round(v*100) / 100 }

func TestOilExample(t *testing.T) {
	es := example()
	vid := uuid.New()
	ss := BuildSeries(vid, es, 1000)
	if len(ss) != 2 || ss[0].ID != PreSeriesID(vid) || ss[1].ID != es[1].ID {
		t.Fatalf("series: %+v", ss)
	}
	// L-1: kein Paar über den Ölwechsel hinweg
	if p := ss[1].Pairs[es[1].ID]; p.Status != "none" || p.Reason != "series_start" {
		t.Fatalf("L-1: %+v", p)
	}
	want := map[int]float64{2: 200, 3: 300, 4: 200} // L-2, L-3, L-4
	for i, w := range want {
		p := ss[1].Pairs[es[i].ID]
		if p.Status != "computed" || r(Per1000(p.Value, p.Dist, "km")) != w {
			t.Errorf("L-%d: %+v", i, p)
		}
	}
	// L-5
	if v := Per1000(ss[1].SumValue, ss[1].SumDist, "km"); r(v) != 225 {
		t.Fatalf("L-5: %v", v)
	}
	// L-6: Nachfüllrate 01.03.–15.05. bei 4 000 km
	st := Totals(es, at(3, 1).Add(-12*time.Hour), at(5, 16).Add(-12*time.Hour))
	if st.AddedMl != 500 || !st.OilChangeInRange || r(Per1000(float64(st.AddedMl), 4_000_000, "km")) != 125 || st.ChangeFillMl != 4300 {
		t.Fatalf("L-6: %+v", st)
	}
}

func TestOilWithoutRange(t *testing.T) {
	es := example()
	ss := BuildSeries(uuid.New(), es, 0)
	// L-9: 30 Prozentpunkte / 1 500 km = 20 %-Punkte/1 000 km; Paar 4→5 nicht berechenbar
	p := ss[1].Pairs[es[2].ID]
	if p.Basis != "percent_points" || r(Per1000(p.Value, p.Dist, "km")) != 20 {
		t.Fatalf("L-9: %+v", p)
	}
	if p := ss[1].Pairs[es[4].ID]; p.Status != "not_computable" {
		t.Fatalf("L-9 4→5: %+v", p)
	}
}

func TestTooFew(t *testing.T) {
	// L-12: Ölwechsel ohne gemessenen Stand und eine Messung
	tot := func(km int64) *int64 { v := km * 1000; return &v }
	lv := 80.0
	es := []Entry{{ID: uuid.New(), Kind: KindOilChange, At: at(1, 1), Total: tot(1000)}, {ID: uuid.New(), Kind: KindCheck, At: at(2, 1), Total: tot(2000), LevelBefore: &lv}}
	ss := BuildSeries(uuid.New(), es, 1000)
	if ss[0].Usable != 1 || ss[0].SumDist != 0 {
		t.Fatalf("L-12: %+v", ss[0])
	}
	if p := ss[0].Pairs[es[1].ID]; p.Status != "not_computable" || p.Reason != "no_level" {
		t.Fatalf("L-12 pair: %+v", p)
	}
}

func TestNegative(t *testing.T) {
	tot := func(km int64) *int64 { v := km * 1000; return &v }
	a, b := 50.0, 60.0
	es := []Entry{{ID: uuid.New(), Kind: KindCheck, At: at(1, 1), Total: tot(1000), LevelBefore: &a}, {ID: uuid.New(), Kind: KindCheck, At: at(2, 1), Total: tot(2000), LevelBefore: &b}}
	p := BuildSeries(uuid.New(), es, 1000)[0].Pairs[es[1].ID]
	if !p.Negative || p.Value != -100 {
		t.Fatalf("negative: %+v", p)
	}
}
