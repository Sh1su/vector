package kernel

import "testing"

func TestPriceTotal(t *testing.T) {
	// U-7: 1,799 €/l × 55,20 l → 99,30 €
	q, _ := ToCanonical(55.20, "l")
	if got, err := PriceTotal(1.799, "EUR", "l", q); err != nil || got != 9930 {
		t.Fatalf("U-7: %d %v", got, err)
	}
	// Preis je Gallone bei Menge in Litern
	q, _ = ToCanonical(3.785411784, "l")
	if got, _ := PriceTotal(4, "USD", "gal_us", q); got != 400 {
		t.Fatalf("gal: %d", got)
	}
	if got, _ := PriceTotal(0.005, "EUR", "l", Quantity{Canonical: 1000, CanonicalUnit: UnitMilliliter, InputValue: "1", InputUnit: "l"}); got != 1 {
		t.Fatalf("half up: %d", got)
	}
}
