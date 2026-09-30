package costs

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/sh1su/vector/backend/internal/costs/store"
	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/odometer"
	pg "github.com/sh1su/vector/backend/internal/platform/pgconv"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

// ReportQuery beschreibt eine Auswertung.
type ReportQuery struct {
	From, To     *time.Time
	GroupBy      string // category | cost_kind | month | year
	Allocation   string // payment | prorated
	DistanceUnit string // Anzeigeeinheit der Strecke (km, mi, h)
}

// Group ist eine Summe je Gruppe (API CostGroup).
type Group struct {
	Key         string `json:"key"`
	AmountMinor int64  `json:"amount_minor"`
}

// DepreciationView entspricht CostCurrencyReport.depreciation.
type DepreciationView struct {
	TotalMinor   int64                `json:"total_minor"`
	PerDay       *kernel.DisplayValue `json:"per_day"`
	PerDistance  *kernel.DisplayValue `json:"per_distance"`
	Estimated    bool                 `json:"estimated"`
	Appreciation bool                 `json:"appreciation"`
}

// CurrencyReport entspricht dem API-Schema CostCurrencyReport.
type CurrencyReport struct {
	Currency          string               `json:"currency"`
	RunningTotalMinor int64                `json:"running_total_minor"`
	Groups            []Group              `json:"groups"`
	PerDistance       *kernel.DisplayValue `json:"per_distance"`
	PerDay            *kernel.DisplayValue `json:"per_day"`
	Depreciation      *DepreciationView    `json:"depreciation"`
	TCOMinor          *int64               `json:"tco_minor"`
}

// Report entspricht dem API-Schema CostReport.
type Report struct {
	From          string               `json:"from"`
	To            string               `json:"to"`
	Allocation    string               `json:"allocation"`
	GroupBy       string               `json:"group_by"`
	Distance      *kernel.DisplayValue `json:"distance"`
	OwnershipDays *int                 `json:"ownership_days"`
	Currencies    []CurrencyReport     `json:"currencies"`
	Hints         []string             `json:"hints"`
}

func toLedgerRows(rows []store.CostsLedger) []LedgerRow {
	out := make([]LedgerRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, LedgerRow{Category: r.Category, CostKind: r.CostKind.String, BookedOn: *pg.DateP(r.BookedOn), Amount: r.AmountMinor,
			Currency: r.Currency, CoversFrom: pg.DateP(r.CoversFrom), CoversTo: pg.DateP(r.CoversTo)})
	}
	return out
}

// CostReport berechnet die Kennzahlen eines Fahrzeugs (Leser).
func (s *Service) CostReport(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, rq ReportQuery) (Report, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return Report{}, err
	}
	return s.report(ctx, vehicleID, rq)
}

func perUnit(amount int64, currency string, per float64, unit string, digits int) *kernel.DisplayValue {
	if per <= 0 {
		return nil
	}
	return &kernel.DisplayValue{Value: kernel.Round(kernel.MajorAmount(amount, currency)/per, digits), Unit: currency + "/" + unit}
}

func (s *Service) report(ctx context.Context, vehicleID uuid.UUID, rq ReportQuery) (Report, error) {
	meta, err := vehicles.LoadMeta(ctx, s.pool, vehicleID, false)
	if err != nil {
		return Report{}, err
	}
	today := kernel.Today(s.Now(), meta.OwnerTimeZone)
	to := today
	if rq.To != nil {
		to = *rq.To
	}
	from := time.Date(to.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	if rq.From != nil {
		from = *rq.From
	}
	if to.Before(from) {
		return Report{}, problem.Validation(problem.FieldError{Pointer: "/to", Code: "order", Message: "`to` liegt vor `from`."})
	}
	if rq.GroupBy == "" {
		rq.GroupBy = "category"
	}
	if rq.Allocation == "" {
		rq.Allocation = "payment"
	}
	if rq.DistanceUnit == "" {
		rq.DistanceUnit = "km"
	}
	if meta.UsageMeter == odometer.MeterEngineHours {
		rq.DistanceUnit = "h"
	}
	ledger, err := store.New(s.pool).ListLedger(ctx, pg.U(vehicleID))
	if err != nil {
		return Report{}, err
	}
	rows := toLedgerRows(ledger)
	sums := Aggregate(rows, from, to, rq.GroupBy, rq.Allocation)
	segs, valid, err := s.odo.Snapshot(ctx, s.pool, vehicleID)
	if err != nil {
		return Report{}, err
	}
	rep := Report{From: kernel.FormatDate(from), To: kernel.FormatDate(to), Allocation: rq.Allocation, GroupBy: rq.GroupBy,
		Currencies: []CurrencyReport{}, Hints: []string{}}

	// CO-05: Distanz im Zeitraum über ODO-05
	tz := meta.OwnerTimeZone
	dist := odometer.DistanceBetween(valid, segs, kernel.StartOfDay(from, tz), kernel.StartOfDay(to.AddDate(0, 0, 1), tz))
	var distance float64
	if dist.Known {
		distance, _ = kernel.FromCanonical(dist.Meters, rq.DistanceUnit)
		rep.Distance = &kernel.DisplayValue{Value: kernel.Round(distance, 1), Unit: rq.DistanceUnit}
	}

	// Besitzzeitraum: Kauf bzw. erster Eintrag bis Tag vor Verkauf bzw. heute
	var start *time.Time
	if meta.PurchaseDate != nil {
		start = meta.PurchaseDate
	} else {
		for _, r := range rows {
			if start == nil || r.BookedOn.Before(*start) {
				b := r.BookedOn
				start = &b
			}
		}
		if len(valid) > 0 {
			d := kernel.LocalDate(valid[0].At, valid[0].TimeZone)
			if start == nil || d.Before(*start) {
				start = &d
			}
		}
	}
	end := today
	if meta.SaleDate != nil {
		end = meta.SaleDate.AddDate(0, 0, -1)
	}
	days := 0
	if start != nil {
		days = OwnershipDays(from, to, *start, end)
		rep.OwnershipDays = &days
	}

	// CO-06: Wertverlust (über die gesamte Besitzdauer)
	var est *kernel.Money
	var estDate *time.Time
	if meta.EstimatedValue != nil && meta.EstimatedValue.Price != nil {
		if d, err := kernel.ParseDate(meta.EstimatedValue.Date); err == nil {
			est = &kernel.Money{AmountMinor: meta.EstimatedValue.Price.AmountMinor, Currency: meta.EstimatedValue.Price.Currency}
			estDate = &d
		}
	}
	var purchase, sale *kernel.Money
	if meta.Purchase != nil {
		purchase = &kernel.Money{AmountMinor: meta.Purchase.AmountMinor, Currency: meta.Purchase.Currency}
	}
	if meta.Sale != nil {
		sale = &kernel.Money{AmountMinor: meta.Sale.AmountMinor, Currency: meta.Sale.Currency}
	}
	dep, hasDep, hint := ComputeDepreciation(meta.PurchaseDate, purchase, meta.SaleDate, sale, est, estDate)
	if hint != "" {
		rep.Hints = append(rep.Hints, hint)
	}

	currencies := sums.Currencies()
	if hasDep {
		found := false
		for _, c := range currencies {
			found = found || c == dep.Currency
		}
		if !found {
			currencies = append(currencies, dep.Currency)
			sort.Strings(currencies)
		}
	}
	for _, c := range currencies {
		cr := CurrencyReport{Currency: c, RunningTotalMinor: sums.Total(c), Groups: []Group{}}
		for k, v := range sums[c] {
			cr.Groups = append(cr.Groups, Group{Key: k, AmountMinor: v})
		}
		sort.Slice(cr.Groups, func(i, j int) bool { return cr.Groups[i].Key < cr.Groups[j].Key })
		if dist.Known {
			cr.PerDistance = perUnit(cr.RunningTotalMinor, c, distance, rq.DistanceUnit, 2)
		}
		cr.PerDay = perUnit(cr.RunningTotalMinor, c, float64(days), "Tag", 2)
		if hasDep && dep.Currency == c {
			dv := &DepreciationView{TotalMinor: dep.Total, Estimated: dep.Estimated, Appreciation: dep.Appreciation}
			if !dep.Appreciation {
				dv.PerDay = perUnit(dep.Total, c, float64(dep.Days), "Tag", 2)
				d := odometer.DistanceBetween(valid, segs, kernel.StartOfDay(dep.From, tz), kernel.StartOfDay(dep.To.AddDate(0, 0, 1), tz))
				if d.Known {
					dd, _ := kernel.FromCanonical(d.Meters, rq.DistanceUnit)
					dv.PerDistance = perUnit(dep.Total, c, dd, rq.DistanceUnit, 2)
				}
				tco := cr.RunningTotalMinor + dep.Total
				cr.TCOMinor = &tco
			}
			cr.Depreciation = dv
		}
		rep.Currencies = append(rep.Currencies, cr)
	}
	if len(rep.Currencies) > 1 {
		rep.Hints = append(rep.Hints, "Beträge in mehreren Währungen werden getrennt summiert; es gibt keine Gesamtsumme (ADR-029).")
	}
	if !dist.Known {
		rep.Hints = append(rep.Hints, "Kosten je Strecke nicht verfügbar: Für den Zeitraum fehlen Kilometerstände.")
	}
	if hasDep {
		rep.Hints = append(rep.Hints, "Der Wertverlust bezieht sich auf die gesamte Besitzdauer.")
	}
	return rep, nil
}

// FleetVehicleReport ist ein Eintrag des FleetCostReport.
type FleetVehicleReport struct {
	VehicleID   uuid.UUID `json:"vehicle_id"`
	DisplayName string    `json:"display_name"`
	Report      Report    `json:"report"`
}

// FleetCostReport wertet alle Fahrzeuge des Kontos aus.
func (s *Service) FleetCostReport(ctx context.Context, actor kernel.Actor, rq ReportQuery) ([]FleetVehicleReport, error) {
	members, err := identity.MemberVehicles(ctx, s.pool, actor.AccountID)
	if err != nil {
		return nil, err
	}
	out := []FleetVehicleReport{}
	for id := range members {
		meta, err := vehicles.LoadMeta(ctx, s.pool, id, false)
		if err != nil {
			continue // gelöschtes Fahrzeug
		}
		r, err := s.report(ctx, id, rq)
		if err != nil {
			return nil, err
		}
		out = append(out, FleetVehicleReport{VehicleID: id, DisplayName: meta.DisplayName, Report: r})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DisplayName < out[j].DisplayName })
	return out, nil
}
