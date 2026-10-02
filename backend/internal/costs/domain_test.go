package costs

import (
	"testing"
	"time"

	"github.com/sh1su/vector/backend/internal/kernel"
)

func d(s string) time.Time   { t, _ := kernel.ParseDate(s); return t }
func dp(s string) *time.Time { t := d(s); return &t }

func januaryRows() []LedgerRow {
	return []LedgerRow{
		{Category: "energy", BookedOn: d("2026-01-10"), Amount: 30000, Currency: "EUR"},
		{Category: "maintenance", CostKind: "parts", BookedOn: d("2026-01-15"), Amount: 20000, Currency: "EUR"},
		{Category: "maintenance", CostKind: "labor", BookedOn: d("2026-01-15"), Amount: 25000, Currency: "EUR"},
		{Category: "insurance", BookedOn: d("2026-01-02"), Amount: 60000, Currency: "EUR", CoversFrom: dp("2026-01-01"), CoversTo: dp("2026-12-31")},
	}
}

// C-1: Summe nach Zahlungsdatum.
func TestC1PaymentDate(t *testing.T) {
	s := Aggregate(januaryRows(), d("2026-01-01"), d("2026-01-31"), "category", "payment")
	if got := s.Total("EUR"); got != 135000 {
		t.Fatalf("C-1 total %d", got)
	}
	if s["EUR"]["maintenance"] != 45000 {
		t.Fatalf("C-1 maintenance %d", s["EUR"]["maintenance"])
	}
}

// C-2: zeitanteilig 600 × 31/365 = 50,96 €; Summe 800,96 €.
func TestC2Prorated(t *testing.T) {
	s := Aggregate(januaryRows(), d("2026-01-01"), d("2026-01-31"), "category", "prorated")
	if s["EUR"]["insurance"] != 5096 || s.Total("EUR") != 80096 {
		t.Fatalf("C-2: %v", s)
	}
	// Monatsweise summiert sich die Versicherung exakt zum Betrag (CO-04).
	year := Aggregate(januaryRows()[3:], d("2026-01-01"), d("2026-12-31"), "month", "prorated")
	if year.Total("EUR") != 60000 || len(year["EUR"]) != 12 {
		t.Fatalf("CO-04 months: %v", year)
	}
}

// C-3: Plan Kfz-Steuer jährlich ab 15.04.2026, Erinnerung 30 Tage.
func TestC3Occurrences(t *testing.T) {
	p := PlanDef{FirstDue: d("2026-04-15"), Months: 12}
	if occ := Occurrences(p, 30, d("2026-03-15"), nil, nil, nil); len(occ) != 0 {
		t.Fatalf("15.03.: %v", occ)
	}
	occ := Occurrences(p, 30, d("2026-03-16"), nil, nil, nil)
	if len(occ) != 1 || occ[0].State != "upcoming" {
		t.Fatalf("16.03.: %v", occ)
	}
	occ = Occurrences(p, 30, d("2026-04-15"), nil, nil, nil)
	if occ[0].State != "open" {
		t.Fatalf("15.04.: %v", occ)
	}
	occ = Occurrences(p, 30, d("2026-04-15"), nil, map[string]bool{"2026-04-15": true}, nil)
	if occ[0].State != "confirmed" {
		t.Fatalf("confirmed: %v", occ)
	}
	// Verkauf vor dem nächsten Vorkommen: nicht mehr angeboten.
	if occ := Occurrences(p, 30, d("2027-05-01"), dp("2027-01-01"), nil, nil); len(occ) != 1 {
		t.Fatalf("after sale: %v", occ)
	}
	// Monatsende bleibt am Anker (31.01. → 28.02. → 31.03.).
	m := PlanDef{FirstDue: d("2026-01-31"), Months: 1}
	if got := kernel.FormatDate(m.At(2)); got != "2026-03-31" || kernel.FormatDate(m.At(1)) != "2026-02-28" {
		t.Fatalf("month grid %s", got)
	}
	if !m.IsOccurrence(d("2026-02-28")) || m.IsOccurrence(d("2026-03-28")) {
		t.Fatal("IsOccurrence")
	}
}

// C-5: 273 Besitztage; C-6: Wertverlust.
func TestC5C6(t *testing.T) {
	if n := OwnershipDays(d("2026-01-01"), d("2026-09-30"), d("2026-01-01"), d("2026-09-30")); n != 273 {
		t.Fatalf("C-5 %d", n)
	}
	dep, ok, _ := ComputeDepreciation(dp("2024-01-01"), &kernel.Money{AmountMinor: 2000000, Currency: "EUR"}, dp("2025-01-01"),
		&kernel.Money{AmountMinor: 1500000, Currency: "EUR"}, nil, nil)
	if !ok || dep.Total != 500000 || dep.Days != 366 {
		t.Fatalf("C-6 %+v", dep)
	}
	_, ok, hint := ComputeDepreciation(dp("2024-01-01"), &kernel.Money{AmountMinor: 2000000, Currency: "EUR"}, dp("2025-01-01"),
		&kernel.Money{AmountMinor: 1500000, Currency: "CHF"}, nil, nil)
	if ok || hint == "" {
		t.Fatal("currency mismatch")
	}
}

// C-7: getrennte Summen je Währung.
func TestC7Currencies(t *testing.T) {
	s := Aggregate([]LedgerRow{{Category: "energy", BookedOn: d("2026-02-01"), Amount: 5000, Currency: "EUR"},
		{Category: "energy", BookedOn: d("2026-02-03"), Amount: 4000, Currency: "CHF"}}, d("2026-02-01"), d("2026-02-28"), "month", "payment")
	if s.Total("EUR") != 5000 || s.Total("CHF") != 4000 || len(s.Currencies()) != 2 {
		t.Fatalf("C-7 %v", s)
	}
}
