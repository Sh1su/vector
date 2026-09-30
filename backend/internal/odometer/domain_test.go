package odometer

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sh1su/vector/backend/internal/kernel"
)

// Soll-Beispiele aus docs/phase-2/10-domaene-odometer.md §8.

const berlin = "Europe/Berlin"

var vehicleSeg = uuid.MustParse("00000000-0000-7000-8000-000000000001")

func at(s string) time.Time {
	loc, _ := time.LoadLocation(berlin)
	t, err := time.ParseInLocation("2006-01-02 15:04", s, loc)
	if err != nil {
		panic(err)
	}
	return t.UTC()
}

func rd(when string, km float64, prec string) Reading {
	et, _ := kernel.NormalizeEventTime(at(when), berlin, prec)
	return Reading{ID: kernel.NewID(), At: et.At, TimeZone: berlin, Precision: et.Precision, RecordedAt: et.At, Value: int64(km * 1000), Status: StatusValid}
}


func hasCode(list []string, c string) bool {
	for _, x := range list {
		if x == c {
			return true
		}
	}
	return false
}

func check(c Reading, others ...Reading) []string {
	segs := NewSegments(vehicleSeg, nil)
	var out []string
	for _, a := range CheckNeighbors(c, others, segs, MeterDistance, Config{VMaxKmh: 250}) {
		out = append(out, a.Code)
	}
	return out
}

func TestO1_BackwardsIsP1AndCurrentIsLatest(t *testing.T) {
	a := rd("2026-03-10 12:00", 45000, "")
	b := rd("2026-03-12 08:00", 44500, "")
	if got := check(b, a); !hasCode(got, "P1") {
		t.Fatalf("want P1, got %v", got)
	}
	b.Status = StatusConfirmedAnomaly
	segs := NewSegments(vehicleSeg, nil)
	cur := Current([]Reading{a, b}, segs)
	if cur.Total != 44_500_000 {
		t.Fatalf("current = %d, want 44 500 km (spätester, nicht Maximum)", cur.Total)
	}
}

func TestO2_JumpIsP3(t *testing.T) {
	a := rd("2026-03-01 10:00", 10000, "")
	b := rd("2026-03-01 11:00", 10400, "")
	if got := check(b, a); !hasCode(got, "P3") {
		t.Fatalf("want P3, got %v", got)
	}
}

func TestO3_DateOnlyUsesWidestInterval(t *testing.T) {
	a := rd("2026-03-01 00:00", 10000, kernel.PrecisionDateOnly)
	b := rd("2026-03-02 00:00", 10900, kernel.PrecisionDateOnly)
	if got := check(b, a); len(got) != 0 {
		t.Fatalf("want none, got %v", got)
	}
}

func TestO4_EqualValueIsNoAnomaly(t *testing.T) {
	a := rd("2026-03-01 10:00", 10000, "")
	b := rd("2026-03-01 00:00", 10000, kernel.PrecisionDateOnly)
	if got := check(b, a); len(got) != 0 {
		t.Fatalf("want none, got %v", got)
	}
}

func TestP2_OvertakesFollowing(t *testing.T) {
	next := rd("2026-03-10 12:00", 45000, "")
	c := rd("2026-03-05 12:00", 46000, "")
	if got := check(c, next); !hasCode(got, "P2") {
		t.Fatalf("want P2, got %v", got)
	}
}

func TestO5_OdometerReplacement(t *testing.T) {
	old := rd("2026-05-31 12:00", 180000, "")
	off := NewSegmentOffset(old.Value, 0, 50_000_000)
	if off != 130_000_000 {
		t.Fatalf("offset = %d", off)
	}
	segs := NewSegments(vehicleSeg, []Segment{{ID: kernel.NewID(), Seq: 2, StartedAt: at("2026-06-01 00:00"), StartMeter: 50_000_000, Offset: off}})
	later := rd("2026-06-10 12:00", 52000, "")
	cur := Current([]Reading{old, later}, segs)
	if cur.Meter != 52_000_000 || cur.Total != 182_000_000 {
		t.Fatalf("meter %d total %d", cur.Meter, cur.Total)
	}
	// Monotonie wird je Abschnitt geprüft: 52 000 nach 180 000 ist kein P1.
	var got []string
	for _, a := range CheckNeighbors(later, []Reading{old}, segs, MeterDistance, Config{}) {
		got = append(got, a.Code)
	}
	if len(got) != 0 {
		t.Fatalf("want none across segments, got %v", got)
	}
}

func TestO6_Interpolation(t *testing.T) {
	segs := NewSegments(vehicleSeg, nil)
	rs := []Reading{rd("2026-01-01 00:00", 10000, ""), rd("2026-01-31 00:00", 13000, "")}
	v := ValueAt(rs, segs, at("2026-01-16 00:00"))
	if v.Kind != KindInterpolated || v.Total != 11_500_000 {
		t.Fatalf("got %+v", v)
	}
}

func TestO7_MonthDistance(t *testing.T) {
	segs := NewSegments(vehicleSeg, nil)
	rs := []Reading{rd("2026-01-01 00:00", 10000, ""), rd("2026-01-20 00:00", 12000, ""), rd("2026-02-10 00:00", 14100, "")}
	d := DistanceBetween(rs, segs, at("2026-01-01 00:00"), at("2026-02-01 00:00"))
	if !d.Known || d.Meters != 3_200_000 {
		t.Fatalf("got %+v", d)
	}
}

func TestO8_BeforeFirstIsUnknown(t *testing.T) {
	segs := NewSegments(vehicleSeg, nil)
	rs := []Reading{rd("2026-05-01 12:00", 20000, "")}
	if d := DistanceBetween(rs, segs, at("2026-04-01 00:00"), at("2026-06-01 00:00")); d.Known {
		t.Fatalf("want unknown, got %+v", d)
	}
	if v := ValueAt(rs, segs, at("2026-06-01 00:00")); v.Kind != KindCarriedForward || v.Total != 20_000_000 {
		t.Fatalf("carry forward: %+v", v)
	}
}

func TestO10_CorrectionWins(t *testing.T) {
	segs := NewSegments(vehicleSeg, nil)
	a := rd("2026-01-01 12:00", 2000, "")
	typo := rd("2026-01-05 12:00", 20000, "")
	typo.Status = StatusSuperseded
	fixed := rd("2026-01-05 12:00", 2400, "")
	// ersetzte Messpunkte sind nicht in der gültigen Liste
	cur := Current([]Reading{a, fixed}, segs)
	if cur.Total != 2_400_000 {
		t.Fatalf("got %d", cur.Total)
	}
}

func TestFutureCheck(t *testing.T) {
	now := at("2026-09-30 12:00")
	if CheckFuture(rd("2026-09-30 12:04", 1, ""), now) != nil {
		t.Error("4 min in future must pass")
	}
	if a := CheckFuture(rd("2026-09-30 12:06", 1, ""), now); a == nil || a.Confirmable {
		t.Error("6 min in future must be rejected, not confirmable")
	}
	if CheckFuture(rd("2026-09-30 00:00", 1, kernel.PrecisionDateOnly), now) != nil {
		t.Error("today date_only must pass")
	}
	if CheckFuture(rd("2026-10-01 00:00", 1, kernel.PrecisionDateOnly), now) == nil {
		t.Error("tomorrow date_only must be rejected")
	}
}

func TestEngineHoursJump(t *testing.T) {
	segs := NewSegments(vehicleSeg, nil)
	a := Reading{ID: kernel.NewID(), At: at("2026-03-01 10:00"), TimeZone: berlin, Precision: "exact", Value: 3600 * 100, Status: StatusValid}
	b := Reading{ID: kernel.NewID(), At: at("2026-03-01 11:00"), TimeZone: berlin, Precision: "exact", Value: 3600 * 102, Status: StatusValid}
	if as := CheckNeighbors(b, []Reading{a}, segs, MeterEngineHours, Config{}); len(as) != 1 || as[0].Code != "P3" {
		t.Fatalf("2 h Betrieb in 1 h muss P3 sein: %v", as)
	}
}
