// Package costs erfasst sonstige Kosten, wiederkehrende Kostenpläne und führt
// das Kostenbuch für alle Auswertungen (docs/phase-2/10-domaene-costs.md).
package costs

import (
	"math/big"
	"sort"
	"strconv"
	"time"

	"github.com/sh1su/vector/backend/internal/kernel"
)

// Kategorien sonstiger Kosten.
var Categories = map[string]bool{"tax": true, "insurance": true, "fee": true, "parking": true, "toll": true, "care": true, "financing": true, "other": true}

// PlanDef beschreibt das Raster eines Kostenplans (CO-02).
type PlanDef struct {
	FirstDue time.Time
	Months   int
	Days     int
	EndsOn   *time.Time
}

// At liefert das k-te Vorkommen, immer ab first_due_on gerechnet (MA-02-Monatsaddition).
func (p PlanDef) At(k int) time.Time {
	if p.Months > 0 {
		return kernel.AddMonths(p.FirstDue, k*p.Months)
	}
	return p.FirstDue.AddDate(0, 0, k*p.Days)
}

// Dates listet alle Vorkommen bis einschließlich until (und ends_on).
func (p PlanDef) Dates(until time.Time) []time.Time {
	if p.Months <= 0 && p.Days <= 0 {
		return nil
	}
	var out []time.Time
	for k := 0; ; k++ {
		d := p.At(k)
		if d.After(until) || (p.EndsOn != nil && d.After(*p.EndsOn)) {
			return out
		}
		out = append(out, d)
		if len(out) > 100000 {
			return out
		}
	}
}

// IsOccurrence prüft, ob d ein Rasterpunkt des Plans ist.
func (p PlanDef) IsOccurrence(d time.Time) bool {
	if d.Before(p.FirstDue) || (p.EndsOn != nil && d.After(*p.EndsOn)) {
		return false
	}
	for _, x := range p.Dates(d) {
		if x.Equal(d) {
			return true
		}
	}
	return false
}

// Occurrence ist ein Vorkommen mit Zustand.
type Occurrence struct {
	Due   time.Time
	State string // upcoming | open | confirmed | dismissed
}

// Occurrences bestimmt die Vorkommen bis heute + remind_days_before, ohne
// solche nach dem Verkaufsdatum (CO-02).
func Occurrences(p PlanDef, remindDays int, today time.Time, sale *time.Time, confirmed, dismissed map[string]bool) []Occurrence {
	var out []Occurrence
	for _, d := range p.Dates(today.AddDate(0, 0, remindDays)) {
		if sale != nil && d.After(*sale) {
			break
		}
		key := kernel.FormatDate(d)
		st := "open"
		switch {
		case confirmed[key]:
			st = "confirmed"
		case dismissed[key]:
			st = "dismissed"
		case d.After(today):
			st = "upcoming"
		}
		out = append(out, Occurrence{Due: d, State: st})
	}
	return out
}

// LedgerRow ist eine Zeile des Kostenbuchs in Rechenform.
type LedgerRow struct {
	Category   string
	CostKind   string
	BookedOn   time.Time
	Amount     int64
	Currency   string
	CoversFrom *time.Time
	CoversTo   *time.Time
}

func ratRound(num *big.Int, den int64) int64 {
	return kernel.RoundHalfEven(new(big.Rat).SetFrac(num, big.NewInt(den)))
}

// cumulative ist der gerundete Anteil der ersten i von n Tagen eines Betrags.
// Differenzen kumulierter Werte summieren sich exakt zum Betrag (CO-04).
func cumulative(amount int64, i, n int) int64 {
	num := new(big.Int).Mul(big.NewInt(amount), big.NewInt(int64(i)))
	return ratRound(num, int64(n))
}

func groupKey(groupBy string, r LedgerRow, day time.Time) string {
	switch groupBy {
	case "cost_kind":
		if r.CostKind == "" {
			return "none"
		}
		return r.CostKind
	case "month":
		return day.Format("2006-01")
	case "year":
		return strconv.Itoa(day.Year())
	default:
		return r.Category
	}
}

// Sums sind Summen je Währung und Gruppe.
type Sums map[string]map[string]int64

func (s Sums) add(cur, key string, v int64) {
	if s[cur] == nil {
		s[cur] = map[string]int64{}
	}
	s[cur][key] += v
}

// Total liefert die Summe einer Währung.
func (s Sums) Total(cur string) int64 {
	var t int64
	for _, v := range s[cur] {
		t += v
	}
	return t
}

// Currencies liefert die Währungen sortiert.
func (s Sums) Currencies() []string {
	out := make([]string, 0, len(s))
	for c := range s {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// Aggregate summiert das Kostenbuch im Zeitraum [from, to] (CO-03, CO-04).
// allocation: payment (Buchungsdatum) oder prorated (tagesgenau über den Leistungszeitraum).
func Aggregate(rows []LedgerRow, from, to time.Time, groupBy, allocation string) Sums {
	out := Sums{}
	for _, r := range rows {
		if allocation == "prorated" && r.CoversFrom != nil && r.CoversTo != nil {
			n := kernel.DaysBetween(*r.CoversFrom, *r.CoversTo) + 1
			s, e := *r.CoversFrom, *r.CoversTo
			if s.Before(from) {
				s = from
			}
			if e.After(to) {
				e = to
			}
			for seg := s; !seg.After(e); {
				// Abschnitt bis zum Ende der Gruppe (Monat/Jahr) oder bis e
				segEnd := e
				switch groupBy {
				case "month":
					if me := time.Date(seg.Year(), seg.Month()+1, 0, 0, 0, 0, 0, time.UTC); me.Before(segEnd) {
						segEnd = me
					}
				case "year":
					if ye := time.Date(seg.Year(), 12, 31, 0, 0, 0, 0, time.UTC); ye.Before(segEnd) {
						segEnd = ye
					}
				}
				i := kernel.DaysBetween(*r.CoversFrom, seg)
				j := kernel.DaysBetween(*r.CoversFrom, segEnd) + 1
				part := cumulative(r.Amount, j, n) - cumulative(r.Amount, i, n)
				out.add(r.Currency, groupKey(groupBy, r, seg), part)
				seg = segEnd.AddDate(0, 0, 1)
			}
			continue
		}
		if r.BookedOn.Before(from) || r.BookedOn.After(to) {
			continue
		}
		out.add(r.Currency, groupKey(groupBy, r, r.BookedOn), r.Amount)
	}
	return out
}

// OwnershipDays zählt die Besitztage im Zeitraum inklusive beider Grenzen (CO-05).
// start: Kaufdatum bzw. erster Eintrag; end: Tag vor Verkauf bzw. heute.
func OwnershipDays(from, to, start, end time.Time) int {
	if start.After(from) {
		from = start
	}
	if end.Before(to) {
		to = end
	}
	if to.Before(from) {
		return 0
	}
	return kernel.DaysBetween(from, to) + 1
}

// Depreciation ist der Wertverlust nach CO-06.
type Depreciation struct {
	Total        int64
	Currency     string
	Estimated    bool
	Appreciation bool
	Days         int
	From, To     time.Time
}

// ComputeDepreciation berechnet den Wertverlust aus Kauf und Verkauf bzw. Schätzwert.
// ok=false: nicht berechenbar; hint erklärt den Grund.
func ComputeDepreciation(purchaseDate *time.Time, purchase *kernel.Money, saleDate *time.Time, sale *kernel.Money,
	estimate *kernel.Money, estimateDate *time.Time) (Depreciation, bool, string) {
	if purchaseDate == nil || purchase == nil {
		return Depreciation{}, false, ""
	}
	d := Depreciation{Currency: purchase.Currency, From: *purchaseDate}
	var other *kernel.Money
	switch {
	case saleDate != nil && sale != nil:
		other, d.To = sale, saleDate.AddDate(0, 0, -1)
	case saleDate == nil && estimate != nil && estimateDate != nil:
		other, d.To, d.Estimated = estimate, *estimateDate, true
	default:
		return Depreciation{}, false, ""
	}
	if other.Currency != purchase.Currency {
		return Depreciation{}, false, "Kauf- und Verkaufspreis haben unterschiedliche Währungen; kein Wertverlust berechnet."
	}
	d.Total = purchase.AmountMinor - other.AmountMinor
	d.Appreciation = d.Total < 0
	d.Days = kernel.DaysBetween(d.From, d.To) + 1
	return d, true, ""
}
