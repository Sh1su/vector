package fuel

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

func ptr[T any](v T) *T { return &v }

func day(m, d int) time.Time { return time.Date(2026, time.Month(m), d, 12, 0, 0, 0, time.UTC) }

// Datensatz A (Benzin) aus docs/phase-2/10-domaene-fuel.md §7.
func datasetA() []Fill {
	rows := []struct {
		m, d   int
		km     int64
		l      int64
		full   bool
		missed bool
	}{{3, 1, 10000, 40, true, false}, {3, 10, 10350, 20, false, false}, {3, 20, 10800, 35, true, false},
		{4, 5, 11400, 42, true, true}, {4, 20, 12000, 36, true, false}, {4, 30, -1, 10, false, false}, {5, 10, 12500, 25, true, false}}
	var out []Fill
	for i, r := range rows {
		f := Fill{ID: uuid.New(), At: day(r.m, r.d), TimeZone: "Europe/Berlin", RecordedAt: day(r.m, r.d).Add(time.Duration(i)), Carrier: "petrol",
			Qty: r.l * 1000, Full: r.full, PrevMissed: r.missed}
		if r.km >= 0 {
			f.Total = ptr(r.km * 1000)
		}
		out = append(out, f)
	}
	return out
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func TestDatasetA(t *testing.T) {
	fills := datasetA()
	res, ivs := Intervals(fills, false)
	if res[fills[0].ID].Status != StatusAnchor {
		t.Fatalf("first: %+v", res[fills[0].ID])
	}
	// U-1
	r := res[fills[2].ID]
	if v, _ := Display(r.Qty, r.Dist, "l_per_100km"); r.Status != StatusComputed || round2(v) != 6.88 {
		t.Fatalf("U-1: %+v %v", r, v)
	}
	if v, _ := Display(r.Qty, r.Dist, "mpg_us"); round2(v) != 34.21 {
		t.Fatalf("U-1 mpg: %v", v)
	}
	// U-2
	if r := res[fills[3].ID]; r.Status != StatusNotComputable || r.Reason != ReasonPreviousMissed {
		t.Fatalf("U-2: %+v", r)
	}
	// U-3
	if r := res[fills[4].ID]; r.Status != StatusComputed || r.Qty != 36000 || r.Dist != 600000 {
		t.Fatalf("U-3: %+v", r)
	}
	// U-4: Vorgang ohne Stand zählt zur Menge
	if r := res[fills[6].ID]; r.Qty != 35000 || r.Dist != 500000 {
		t.Fatalf("U-4: %+v", r)
	}
	if res[fills[5].ID].Reason != ReasonNoOdometer {
		t.Fatalf("no odometer: %+v", res[fills[5].ID])
	}
	// U-5
	s := Summarize(ivs, nil, nil)
	if v, _ := Display(s.Qty, s.Dist, ""); round2(v) != 6.63 || s.Computed != 3 || s.NotComputable != 1 {
		t.Fatalf("U-5: %+v %v", s, v)
	}
	// U-6
	want := map[string]float64{"2026-03": 6.88, "2026-04": 6.00, "2026-05": 7.00}
	if len(s.Monthly) != 3 {
		t.Fatalf("U-6: %+v", s.Monthly)
	}
	for _, m := range s.Monthly {
		if v, _ := Display(m.Qty, m.Dist, ""); round2(v) != want[m.Month] {
			t.Errorf("U-6 %s: %v", m.Month, v)
		}
	}
	// Zeitraum: nur April
	from, to := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	if s := Summarize(ivs, &from, &to); s.Computed != 1 || s.NotComputable != 1 {
		t.Fatalf("range: %+v", s)
	}
}

func TestYearsNotMerged(t *testing.T) {
	ivs := []Interval{{ClosingAt: time.Date(2025, 3, 5, 12, 0, 0, 0, time.UTC), TimeZone: "UTC", Qty: 10, Dist: 100, Computable: true},
		{ClosingAt: time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC), TimeZone: "UTC", Qty: 20, Dist: 100, Computable: true}}
	if s := Summarize(ivs, nil, nil); len(s.Monthly) != 2 || s.Monthly[0].Month != "2025-03" {
		t.Fatalf("%+v", s.Monthly)
	}
}

func TestElectricity(t *testing.T) {
	// U-9: Batterieverbrauch
	a := Fill{ID: uuid.New(), At: day(5, 1), TimeZone: "UTC", Qty: 40000, Total: ptr(int64(20_000_000)), SocEnd: ptr(80.0)}
	b := Fill{ID: uuid.New(), At: day(5, 5), TimeZone: "UTC", Qty: 35000, Total: ptr(int64(20_250_000)), SocStart: ptr(30.0)}
	ivs := Battery([]Fill{a, b}, 60000)
	if len(ivs) != 1 || !ivs[0].Computable {
		t.Fatalf("U-9: %+v", ivs)
	}
	if v, u := Display(ivs[0].Qty, ivs[0].Dist, "kwh_per_100km"); round2(v) != 12 || u != "kWh/100km" {
		t.Fatalf("U-9: %v %s", v, u)
	}
	// U-11: Kapazität unbekannt
	if Battery([]Fill{a, b}, 0) != nil {
		t.Fatal("U-11")
	}
	// U-10: Netzbezug, jeder Ladevorgang mit Stand ist Anker/Abschluss
	c := Fill{ID: uuid.New(), At: day(5, 20), TimeZone: "UTC", Qty: 145000, Total: ptr(int64(21_200_000))}
	_, grid := Intervals([]Fill{a, b, c}, true)
	s := Summarize(grid, nil, nil)
	if v, _ := Display(s.Qty, s.Dist, "kwh_per_100km"); s.Computed != 2 || round2(v) != 15 {
		t.Fatalf("U-10: %+v %v", s, v)
	}
}
