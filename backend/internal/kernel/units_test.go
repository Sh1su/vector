package kernel

import (
	"math/big"
	"testing"
)

func TestToCanonical(t *testing.T) {
	cases := []struct {
		v    float64
		unit string
		want int64
		cu   string
	}{
		{143520, "km", 143_520_000, UnitMeter},
		{1, "mi", 1609, UnitMeter},     // 1609.344 → 1609
		{0.7, "l", 700, UnitMilliliter}, // exakt, keine Binärnäherung
		{1, "gal_us", 3785, UnitMilliliter},
		{1, "gal_imp", 4546, UnitMilliliter},
		{1.5, "h", 5400, UnitSecond},
		{45.2, "kWh", 45200, UnitWattHour},
		{0.0005, "km", 0, UnitMeter}, // 0.5 m → gerade 0
		{0.0015, "km", 2, UnitMeter}, // 1.5 m → gerade 2
	}
	for _, c := range cases {
		q, err := ToCanonical(c.v, c.unit)
		if err != nil {
			t.Fatal(err)
		}
		if q.Canonical != c.want || q.CanonicalUnit != c.cu {
			t.Errorf("%v %s: got %d %s, want %d %s", c.v, c.unit, q.Canonical, q.CanonicalUnit, c.want, c.cu)
		}
	}
	if _, err := ToCanonical(1, "furlong"); err == nil {
		t.Error("unknown unit accepted")
	}
}

func TestRoundHalfEven(t *testing.T) {
	for s, want := range map[string]int64{"2.5": 2, "3.5": 4, "-2.5": -2, "2.4999": 2, "2.5001": 3, "-3.5": -4} {
		r, _ := new(big.Rat).SetString(s)
		if got := RoundHalfEven(r); got != want {
			t.Errorf("%s: got %d want %d", s, got, want)
		}
	}
}

func TestFromCanonical(t *testing.T) {
	v, _ := FromCanonical(1_609_344, "mi")
	if v != 1000 {
		t.Errorf("got %v", v)
	}
}
