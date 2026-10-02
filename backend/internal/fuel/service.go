package fuel

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sh1su/vector/backend/internal/fuel/store"
	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/odometer"
	"github.com/sh1su/vector/backend/internal/platform/audit"
	"github.com/sh1su/vector/backend/internal/platform/db"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

const source = "fuel"

// Service ist der Application Service des Moduls Fuel.
type Service struct {
	pool *pgxpool.Pool
	odo  *odometer.Service
	Now  func() time.Time
}

func NewService(pool *pgxpool.Pool, odo *odometer.Service) *Service {
	return &Service{pool: pool, odo: odo, Now: time.Now}
}

// Price ist die Eingabe „Preis pro Einheit“ (FU-07, Schema PriceInput).
type Price struct {
	Value    float64 `json:"value"`
	Currency string  `json:"currency"`
	PerUnit  string  `json:"per_unit"`
}

// Input sind die änderbaren Felder (JSON-Namen wie in der API).
type Input struct {
	OccurredAt     time.Time       `json:"occurred_at"`
	TimeZone       string          `json:"time_zone"`
	TimePrecision  string          `json:"time_precision,omitempty"`
	EnergyCarrier  string          `json:"energy_carrier"`
	Quantity       vehicles.Q      `json:"quantity"`
	FillLevel      string          `json:"fill_level"`
	PreviousMissed bool            `json:"previous_missed"`
	Odometer       *vehicles.Q     `json:"odometer"`
	Cost           *vehicles.Money `json:"cost"`
	PricePerUnit   *Price          `json:"price_per_unit"`
	SocStartPct    *float64        `json:"soc_start_pct"`
	SocEndPct      *float64        `json:"soc_end_pct"`
	ChargeType     *string         `json:"charge_type"`
	Station        *string         `json:"station"`
	Note           string          `json:"note"`
	Tags           []string        `json:"tags"`
}

// Confirmation sind bestätigte Befunde mit Begründung (ADR-010).
type Confirmation struct {
	Codes  []string
	Reason string
}

// IntervalView entspricht dem Schema IntervalResult.
type IntervalView struct {
	Status      string               `json:"status"`
	Reason      *string              `json:"reason"`
	Consumption *kernel.DisplayValue `json:"consumption"`
	Distance    *kernel.DisplayValue `json:"distance"`
	Quantity    *kernel.DisplayValue `json:"quantity"`
}

// View entspricht dem Schema FuelFill.
type View struct {
	ID         uuid.UUID `json:"id"`
	Version    int       `json:"version"`
	VehicleID  uuid.UUID `json:"vehicle_id"`
	CreatedAt  time.Time `json:"created_at"`
	CreatedBy  uuid.UUID `json:"created_by"`
	UpdatedAt  time.Time `json:"updated_at"`
	UpdatedBy  uuid.UUID `json:"updated_by"`
	RecordedAt time.Time `json:"recorded_at"`
	Origin     string    `json:"origin"`
	Input
	OdometerReadingID *uuid.UUID             `json:"odometer_reading_id"`
	OdometerTotal     *odometer.QuantityView `json:"odometer_total"`
	StoredQuantity    odometer.QuantityView  `json:"stored_quantity"`
	UnitPrice         *kernel.DisplayValue   `json:"unit_price"`
	Interval          IntervalView           `json:"interval"`
	deleted           bool
}

func pgU(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }
func pgUp(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgU(*id)
}
func pgText(s *string) pgtype.Text {
	if s == nil || strings.TrimSpace(*s) == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: strings.TrimSpace(*s), Valid: true}
}
func numeric(f *float64) pgtype.Numeric {
	var n pgtype.Numeric
	if f != nil {
		_ = n.Scan(strconv.FormatFloat(*f, 'f', -1, 64))
	}
	return n
}
func numStr(s string) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(s)
	return n
}
func fptr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	f, _ := n.Float64Value()
	v := f.Float64
	return &v
}
func sptr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

// record ist die geprüfte, kanonische Form eines Vorgangs.
type record struct {
	in        Input
	at        kernel.EventTime
	qty       kernel.Quantity
	costMinor *int64
	costCur   *string
}

func (s *Service) normalize(meta vehicles.Meta, in Input) (record, []problem.Anomaly, error) {
	var errs []problem.FieldError
	var anomalies []problem.Anomaly
	add := func(p, c, m string) { errs = append(errs, problem.FieldError{Pointer: p, Code: c, Message: m}) }
	r := record{in: in}
	et, err := kernel.NormalizeEventTime(in.OccurredAt, in.TimeZone, in.TimePrecision)
	if err != nil {
		add("/time_zone", "time", err.Error())
	}
	r.at = et
	switch in.EnergyCarrier {
	case "petrol", "diesel", "lpg", "electricity":
	default:
		add("/energy_carrier", "enum", "")
	}
	if in.FillLevel != "full" && in.FillLevel != "partial" {
		add("/fill_level", "enum", "")
	}
	electric := in.EnergyCarrier == "electricity"
	q, err := kernel.ToCanonical(in.Quantity.Value, in.Quantity.Unit)
	switch {
	case err != nil:
		add("/quantity/unit", "unit", "")
	case electric != (q.CanonicalUnit == kernel.UnitWattHour) || (!electric && q.CanonicalUnit != kernel.UnitMilliliter):
		add("/quantity/unit", "unit", "Strom wird in kWh/Wh erfasst, Kraftstoff als Volumen (I-FU-1).")
	case q.Canonical <= 0:
		add("/quantity/value", "range", "Die Menge muss größer als 0 sein.")
	}
	r.qty = q
	if in.Cost != nil && in.PricePerUnit != nil {
		add("/price_per_unit", "exclusive", "Entweder Gesamtbetrag oder Preis pro Einheit angeben.")
	}
	if in.Cost != nil {
		if len(in.Cost.Currency) != 3 || strings.ToUpper(in.Cost.Currency) != in.Cost.Currency {
			add("/cost/currency", "pattern", "ISO 4217")
		}
		if in.Cost.AmountMinor < 0 {
			add("/cost/amount_minor", "range", "")
		}
		a, c := in.Cost.AmountMinor, in.Cost.Currency
		r.costMinor, r.costCur = &a, &c
	}
	if in.PricePerUnit != nil && err == nil {
		p := in.PricePerUnit
		if len(p.Currency) != 3 || strings.ToUpper(p.Currency) != p.Currency {
			add("/price_per_unit/currency", "pattern", "ISO 4217")
		} else if p.Value < 0 {
			add("/price_per_unit/value", "range", "")
		} else if total, perr := kernel.PriceTotal(p.Value, p.Currency, p.PerUnit, q); perr != nil {
			add("/price_per_unit/per_unit", "unit", "Die Preiseinheit passt nicht zur Menge.")
		} else {
			c := p.Currency
			r.costMinor, r.costCur = &total, &c
		}
	}
	if !electric && (in.SocStartPct != nil || in.SocEndPct != nil || in.ChargeType != nil) {
		add("/soc_start_pct", "electricity_only", "Ladezustand und Ladeart nur bei Strom.")
	}
	for _, f := range []struct {
		p string
		v *float64
	}{{"/soc_start_pct", in.SocStartPct}, {"/soc_end_pct", in.SocEndPct}} {
		if f.v != nil && (*f.v < 0 || *f.v > 100) {
			add(f.p, "range", "0–100 %")
		}
	}
	if in.ChargeType != nil && *in.ChargeType != "ac" && *in.ChargeType != "dc" && *in.ChargeType != "unknown" {
		add("/charge_type", "enum", "")
	}
	if in.Station != nil && len([]rune(*in.Station)) > 200 {
		add("/station", "length", "")
	}
	if len(in.Tags) > 20 {
		add("/tags", "max_items", "")
	}
	if len(errs) > 0 {
		return r, nil, problem.Validation(errs...)
	}
	// FU-08
	if !contains(meta.EnergyCarriers, in.EnergyCarrier) {
		anomalies = append(anomalies, problem.Anomaly{Code: "F2", Confirmable: false, Message: "Dieser Energieträger ist am Fahrzeug nicht hinterlegt."})
	}
	if in.SocStartPct != nil && in.SocEndPct != nil && *in.SocEndPct <= *in.SocStartPct {
		anomalies = append(anomalies, problem.Anomaly{Code: "F3", Confirmable: false, Message: "Der Ladezustand am Ende muss größer sein als am Beginn."})
	}
	if c := meta.TankCapacityMl[in.EnergyCarrier]; c > 0 && q.Canonical*100 > c*110 {
		anomalies = append(anomalies, problem.Anomaly{Code: "F1", Confirmable: true, Message: "Die Menge ist größer als der Tankinhalt laut Fahrzeugdaten."})
	}
	future := false
	if et.Precision == kernel.PrecisionDateOnly {
		future = kernel.LocalDate(et.At, et.TimeZone).After(kernel.LocalDate(s.Now(), et.TimeZone))
	} else {
		future = et.At.After(s.Now().Add(5 * time.Minute))
	}
	if future && in.Odometer == nil {
		anomalies = append(anomalies, problem.Anomaly{Code: "P4", Confirmable: false, Message: "Der Zeitpunkt liegt in der Zukunft."})
	}
	return r, anomalies, nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// planOdometer plant den Messpunkt (I-FU-3/4).
func (s *Service) planOdometer(ctx context.Context, tx pgx.Tx, meta vehicles.Meta, id uuid.UUID, r record) (*odometer.OwnedPlan, error) {
	if r.in.Odometer == nil {
		if meta.OdometerRequired {
			return nil, problem.Validation(problem.FieldError{Pointer: "/odometer", Code: "required", Message: "Für dieses Fahrzeug ist der Kilometerstand Pflicht."})
		}
		return s.odo.PlanOwned(ctx, tx, meta, source, id, nil)
	}
	return s.odo.PlanOwned(ctx, tx, meta, source, id, &odometer.Input{OccurredAt: r.at.At, TimeZone: r.at.TimeZone, Precision: r.at.Precision,
		Value: r.in.Odometer.Value, Unit: r.in.Odometer.Unit})
}

// confirm wertet alle Befunde gemeinsam aus (FU-08: eine Antwort, eine Bestätigung).
func confirm(own []problem.Anomaly, plan *odometer.OwnedPlan, c Confirmation) ([]odometer.AnomalyRecord, []odometer.AnomalyRecord, error) {
	all := append(append([]problem.Anomaly{}, own...), plan.Anomalies()...)
	records, err := odometer.ConfirmOrReject(all, c.Codes, c.Reason)
	if err != nil {
		return nil, nil, err
	}
	var odo []odometer.AnomalyRecord
	for _, r := range records {
		if strings.HasPrefix(r.Code, "P") {
			odo = append(odo, r)
		}
	}
	return records, odo, nil
}

func rowParams(r record) (string, string, string, int64, pgtype.Numeric, string) {
	return r.at.TimeZone, r.at.Precision, r.in.EnergyCarrier, r.qty.Canonical, numStr(r.qty.InputValue), r.qty.InputUnit
}

func priceCols(r record) (pgtype.Numeric, pgtype.Text, pgtype.Text) {
	if r.in.PricePerUnit == nil {
		return pgtype.Numeric{}, pgtype.Text{}, pgtype.Text{}
	}
	p := r.in.PricePerUnit
	return numeric(&p.Value), pgtype.Text{String: p.Currency, Valid: true}, pgtype.Text{String: p.PerUnit, Valid: true}
}

func costCols(r record) (pgtype.Int8, pgtype.Text) {
	if r.costMinor == nil {
		return pgtype.Int8{}, pgtype.Text{}
	}
	return pgtype.Int8{Int64: *r.costMinor, Valid: true}, pgtype.Text{String: *r.costCur, Valid: true}
}

func nonNil(t []string) []string {
	if t == nil {
		return []string{}
	}
	return t
}

// Create erfasst einen Vorgang mit Messpunkt (RecordFill).
func (s *Service) Create(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, id *uuid.UUID, in Input, c Confirmation, units kernel.Units) (View, bool, error) {
	var out View
	created := true
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := identity.Authorize(ctx, tx, actor, vehicleID, identity.RoleEditor); err != nil {
			return err
		}
		meta, err := vehicles.LoadMeta(ctx, tx, vehicleID, true)
		if err != nil {
			return err
		}
		q := store.New(tx)
		fid := kernel.NewID()
		if id != nil {
			fid = *id
			ex, err := q.GetFillAny(ctx, pgU(fid))
			if err == nil {
				cur := inputOf(ex, nil)
				if uuid.UUID(ex.VehicleID.Bytes) == vehicleID && ex.DeletedAt.Valid == false && cur.OccurredAt.Equal(in.OccurredAt) &&
					cur.Quantity == in.Quantity && cur.EnergyCarrier == in.EnergyCarrier && cur.FillLevel == in.FillLevel {
					created = false
					out, err = s.viewOne(ctx, tx, vehicleID, fid, units)
					return err
				}
				return problem.Conflict("Ein Tankvorgang mit dieser ID existiert mit anderem Inhalt.")
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		r, own, err := s.normalize(meta, in)
		if err != nil {
			return err
		}
		plan, err := s.planOdometer(ctx, tx, meta, fid, r)
		if err != nil {
			return err
		}
		records, odoRecords, err := confirm(own, plan, c)
		if err != nil {
			return err
		}
		readingID, err := s.odo.ApplyOwned(ctx, tx, actor, plan, odoRecords)
		if err != nil {
			return err
		}
		tz, prec, carrier, qty, iq, iu := rowParams(r)
		cm, cc := costCols(r)
		pp, pc, pu := priceCols(r)
		ra, _ := json.Marshal(nonNilRecords(records))
		if _, err := q.InsertFill(ctx, store.InsertFillParams{ID: pgU(fid), VehicleID: pgU(vehicleID), OccurredAt: pgtype.Timestamptz{Time: r.at.At, Valid: true},
			TimeZone: tz, TimePrecision: prec, EnergyCarrier: carrier, Quantity: qty, InputQuantity: iq, InputUnit: iu, FillLevel: in.FillLevel,
			PreviousMissed: in.PreviousMissed, OdometerReadingID: pgUp(readingID), CostAmountMinor: cm, CostCurrency: cc,
			InputPricePerUnit: pp, InputPriceCurrency: pc, InputPriceUnit: pu, SocStartPct: numeric(in.SocStartPct), SocEndPct: numeric(in.SocEndPct),
			ChargeType: pgText(in.ChargeType), Station: pgText(in.Station), Note: in.Note, Tags: nonNil(in.Tags), ConfirmedAnomalies: ra,
			Origin: odometer.OriginOf(actor), CreatedBy: pgU(actor.AccountID)}); err != nil {
			return err
		}
		if err := audit.Write(ctx, tx, actor, audit.Event{Action: "fuel.fill_recorded", VehicleID: &vehicleID, ObjectType: "fuel_fill", ObjectID: fid,
			Changes: map[string]any{"quantity": qty, "energy_carrier": carrier, "cost_minor": r.costMinor, "anomalies": records}, Reason: c.Reason}); err != nil {
			return err
		}
		out, err = s.viewOne(ctx, tx, vehicleID, fid, units)
		return err
	})
	return out, created, err
}

func nonNilRecords(r []odometer.AnomalyRecord) []odometer.AnomalyRecord {
	if r == nil {
		return []odometer.AnomalyRecord{}
	}
	return r
}

// load liefert ein Fahrzeug-Vorgang mit Rechteprüfung am geladenen Objekt.
func (s *Service) loadFill(ctx context.Context, tx pgx.Tx, actor kernel.Actor, vehicleID, fillID uuid.UUID, role string) (store.FuelFill, error) {
	row, err := store.New(tx).GetFill(ctx, pgU(fillID))
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && uuid.UUID(row.VehicleID.Bytes) != vehicleID) {
		return row, problem.NotFound()
	}
	if err != nil {
		return row, err
	}
	_, err = identity.Authorize(ctx, tx, actor, uuid.UUID(row.VehicleID.Bytes), role)
	return row, err
}

// Update ändert einen Vorgang per JSON Merge Patch (UpdateFill, If-Match).
func (s *Service) Update(ctx context.Context, actor kernel.Actor, vehicleID, fillID uuid.UUID, ifMatch string, patch []byte, c Confirmation, units kernel.Units) (View, error) {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return View{}, err
	}
	var out View
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		row, err := s.loadFill(ctx, tx, actor, vehicleID, fillID, identity.RoleEditor)
		if err != nil {
			return err
		}
		meta, err := vehicles.LoadMeta(ctx, tx, vehicleID, true)
		if err != nil {
			return err
		}
		if int(row.Version) != version {
			cur, _ := s.viewOne(ctx, tx, vehicleID, fillID, units)
			return problem.PreconditionFailed(cur)
		}
		segs, valid, err := s.odo.State(ctx, tx, vehicleID)
		if err != nil {
			return err
		}
		owned := odometer.Owned(valid, segs, source)
		var or *odometer.OwnedReading
		if o, ok := owned[fillID]; ok {
			or = &o
		}
		cur := inputOf(row, or)
		// Gesamtbetrag und Preis pro Einheit schließen sich aus: der neue Wert ersetzt den alten.
		var keys map[string]json.RawMessage
		_ = json.Unmarshal(patch, &keys)
		if v, ok := keys["price_per_unit"]; ok && string(v) != "null" {
			cur.Cost = nil
		}
		if v, ok := keys["cost"]; ok && string(v) != "null" {
			cur.PricePerUnit = nil
		}
		var in Input
		if err := kernel.MergePatch(cur, patch, &in); err != nil {
			return problem.BadRequest("Ungültiger Merge Patch.")
		}
		r, own, err := s.normalize(meta, in)
		if err != nil {
			return err
		}
		plan, err := s.planOdometer(ctx, tx, meta, fillID, r)
		if err != nil {
			return err
		}
		records, odoRecords, err := confirm(own, plan, c)
		if err != nil {
			return err
		}
		readingID, err := s.odo.ApplyOwned(ctx, tx, actor, plan, odoRecords)
		if err != nil {
			return err
		}
		tz, prec, carrier, qty, iq, iu := rowParams(r)
		cm, cc := costCols(r)
		pp, pc, pu := priceCols(r)
		ra, _ := json.Marshal(nonNilRecords(records))
		if _, err := store.New(tx).UpdateFill(ctx, store.UpdateFillParams{ID: row.ID, OccurredAt: pgtype.Timestamptz{Time: r.at.At, Valid: true},
			TimeZone: tz, TimePrecision: prec, EnergyCarrier: carrier, Quantity: qty, InputQuantity: iq, InputUnit: iu, FillLevel: in.FillLevel,
			PreviousMissed: in.PreviousMissed, OdometerReadingID: pgUp(readingID), CostAmountMinor: cm, CostCurrency: cc,
			InputPricePerUnit: pp, InputPriceCurrency: pc, InputPriceUnit: pu, SocStartPct: numeric(in.SocStartPct), SocEndPct: numeric(in.SocEndPct),
			ChargeType: pgText(in.ChargeType), Station: pgText(in.Station), Note: in.Note, Tags: nonNil(in.Tags), ConfirmedAnomalies: ra,
			UpdatedBy: pgU(actor.AccountID), Version: row.Version}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return problem.PreconditionFailed(nil)
			}
			return err
		}
		if err := audit.Write(ctx, tx, actor, audit.Event{Action: "fuel.fill_updated", VehicleID: &vehicleID, ObjectType: "fuel_fill", ObjectID: fillID,
			Changes: map[string]any{"patch": json.RawMessage(patch), "anomalies": records}, Reason: c.Reason}); err != nil {
			return err
		}
		out, err = s.viewOne(ctx, tx, vehicleID, fillID, units)
		return err
	})
	return out, err
}

// Delete löscht einen Vorgang weich, inklusive Messpunkt (I-FU-3).
func (s *Service) Delete(ctx context.Context, actor kernel.Actor, vehicleID, fillID uuid.UUID, ifMatch string) error {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return err
	}
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		row, err := s.loadFill(ctx, tx, actor, vehicleID, fillID, identity.RoleEditor)
		if err != nil {
			return err
		}
		meta, err := vehicles.LoadMeta(ctx, tx, vehicleID, true)
		if err != nil {
			return err
		}
		if int(row.Version) != version {
			return problem.PreconditionFailed(nil)
		}
		plan, err := s.odo.PlanOwned(ctx, tx, meta, source, fillID, nil)
		if err != nil {
			return err
		}
		if _, err := s.odo.ApplyOwned(ctx, tx, actor, plan, nil); err != nil {
			return err
		}
		n, err := store.New(tx).SoftDeleteFill(ctx, store.SoftDeleteFillParams{ID: row.ID, UpdatedBy: pgU(actor.AccountID), Version: row.Version})
		if err != nil {
			return err
		}
		if n == 0 {
			return problem.PreconditionFailed(nil)
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "fuel.fill_deleted", VehicleID: &vehicleID, ObjectType: "fuel_fill", ObjectID: fillID})
	})
}

// inputOf bildet eine gespeicherte Zeile auf die Eingabeform ab.
func inputOf(r store.FuelFill, or *odometer.OwnedReading) Input {
	iq, _ := r.InputQuantity.Float64Value()
	in := Input{OccurredAt: r.OccurredAt.Time, TimeZone: r.TimeZone, TimePrecision: r.TimePrecision, EnergyCarrier: r.EnergyCarrier,
		Quantity: vehicles.Q{Value: iq.Float64, Unit: r.InputUnit}, FillLevel: r.FillLevel, PreviousMissed: r.PreviousMissed,
		SocStartPct: fptr(r.SocStartPct), SocEndPct: fptr(r.SocEndPct), ChargeType: sptr(r.ChargeType), Station: sptr(r.Station),
		Note: r.Note, Tags: nonNil(r.Tags)}
	if or != nil {
		in.Odometer = &vehicles.Q{Value: or.InputValue, Unit: or.InputUnit}
	}
	if r.InputPricePerUnit.Valid {
		in.PricePerUnit = &Price{Value: *fptr(r.InputPricePerUnit), Currency: r.InputPriceCurrency.String, PerUnit: r.InputPriceUnit.String}
	} else if r.CostAmountMinor.Valid {
		in.Cost = &vehicles.Money{AmountMinor: r.CostAmountMinor.Int64, Currency: r.CostCurrency.String}
	}
	return in
}

// vehicleData sind alle Vorgänge eines Fahrzeugs mit Messpunkten, berechnet.
type vehicleData struct {
	meta    vehicles.Meta
	rows    []store.FuelFill
	owned   map[uuid.UUID]odometer.OwnedReading
	results map[uuid.UUID]Result
	byCarr  map[string][]Fill
}

func (s *Service) data(ctx context.Context, q store.DBTX, vehicleID uuid.UUID, includeDeleted bool) (vehicleData, error) {
	var d vehicleData
	meta, err := vehicles.LoadMeta(ctx, q, vehicleID, false)
	if err != nil {
		return d, err
	}
	rows, err := store.New(q).ListFills(ctx, store.ListFillsParams{VehicleID: pgU(vehicleID), IncludeDeleted: includeDeleted})
	if err != nil {
		return d, err
	}
	segs, valid, err := s.odo.State(ctx, q, vehicleID)
	if err != nil {
		return d, err
	}
	d = vehicleData{meta: meta, rows: rows, owned: odometer.Owned(valid, segs, source), results: map[uuid.UUID]Result{}, byCarr: map[string][]Fill{}}
	for _, r := range rows {
		if r.DeletedAt.Valid {
			continue
		}
		f := Fill{ID: uuid.UUID(r.ID.Bytes), At: r.OccurredAt.Time, TimeZone: r.TimeZone, RecordedAt: r.RecordedAt.Time, Carrier: r.EnergyCarrier,
			Qty: r.Quantity, Full: r.FillLevel == "full", PrevMissed: r.PreviousMissed, SocStart: fptr(r.SocStartPct), SocEnd: fptr(r.SocEndPct)}
		if o, ok := d.owned[f.ID]; ok {
			t := o.Total
			f.Total = &t
		}
		d.byCarr[f.Carrier] = append(d.byCarr[f.Carrier], f)
	}
	for c, fs := range d.byCarr {
		Sort(fs)
		res, _ := Intervals(fs, c == "electricity")
		for k, v := range res {
			d.results[k] = v
		}
	}
	return d, nil
}

func (s *Service) view(d vehicleData, r store.FuelFill, units kernel.Units) View {
	id := uuid.UUID(r.ID.Bytes)
	var or *odometer.OwnedReading
	if o, ok := d.owned[id]; ok {
		or = &o
	}
	iq, _ := r.InputQuantity.Float64Value()
	iu := r.InputUnit
	cu := kernel.UnitMilliliter
	if r.EnergyCarrier == "electricity" {
		cu = kernel.UnitWattHour
	}
	v := View{ID: id, Version: int(r.Version), VehicleID: uuid.UUID(r.VehicleID.Bytes), CreatedAt: r.CreatedAt.Time, CreatedBy: uuid.UUID(r.CreatedBy.Bytes),
		UpdatedAt: r.UpdatedAt.Time, UpdatedBy: uuid.UUID(r.UpdatedBy.Bytes), RecordedAt: r.RecordedAt.Time, Origin: r.Origin, Input: inputOf(r, or),
		StoredQuantity: odometer.QuantityView{Canonical: r.Quantity, CanonicalUnit: cu, InputValue: &iq.Float64, InputUnit: &iu}, deleted: r.DeletedAt.Valid}
	if r.CostAmountMinor.Valid {
		v.Cost = &vehicles.Money{AmountMinor: r.CostAmountMinor.Int64, Currency: r.CostCurrency.String}
		v.UnitPrice = unitPrice(r.CostAmountMinor.Int64, r.CostCurrency.String, r.Quantity, r.EnergyCarrier, units)
	}
	if or != nil {
		rid := or.ID
		v.OdometerReadingID = &rid
		v.OdometerTotal = &odometer.QuantityView{Canonical: or.Total, CanonicalUnit: meterUnit(d.meta)}
	}
	v.Interval = intervalView(d.results[id], r.EnergyCarrier, units, d.meta)
	if r.DeletedAt.Valid {
		v.Interval = IntervalView{Status: StatusNone}
	}
	return v
}

func meterUnit(m vehicles.Meta) string {
	if m.UsageMeter == odometer.MeterEngineHours {
		return kernel.UnitSecond
	}
	return kernel.UnitMeter
}

// unitPrice berechnet FU-06 in der Anzeigeeinheit (€/l, €/gal, €/kWh).
func unitPrice(minor int64, currency string, qty int64, carrier string, units kernel.Units) *kernel.DisplayValue {
	if qty <= 0 {
		return nil
	}
	unit := units.Volume
	if carrier == "electricity" {
		unit = "kWh"
	}
	amount, _ := kernel.FromCanonical(qty, unit)
	if amount <= 0 {
		return nil
	}
	major := float64(minor)
	for i := 0; i < kernel.MinorDigits(currency); i++ {
		major /= 10
	}
	return &kernel.DisplayValue{Value: kernel.Round(major/amount, 3), Unit: currency + "/" + kernel.UnitLabel(unit)}
}

func distanceDisplay(m int64, units kernel.Units, meta vehicles.Meta) *kernel.DisplayValue {
	if meta.UsageMeter == odometer.MeterEngineHours {
		return &kernel.DisplayValue{Value: kernel.Display(m, "h", 1), Unit: "h"}
	}
	return &kernel.DisplayValue{Value: kernel.Display(m, units.Distance, 1), Unit: units.Distance}
}

func quantityDisplay(q int64, carrier string, units kernel.Units) *kernel.DisplayValue {
	if carrier == "electricity" {
		return &kernel.DisplayValue{Value: kernel.Display(q, "kWh", 2), Unit: "kWh"}
	}
	return &kernel.DisplayValue{Value: kernel.Display(q, units.Volume, 2), Unit: kernel.UnitLabel(units.Volume)}
}

func consumptionDisplay(qty, dist int64, carrier string, units kernel.Units) *kernel.DisplayValue {
	if dist <= 0 || qty <= 0 {
		return nil
	}
	u := units.Consumption
	if carrier == "electricity" {
		u = units.ElectricConsumption
	}
	v, label := Display(qty, dist, u)
	return &kernel.DisplayValue{Value: kernel.Round(v, 2), Unit: label}
}

func intervalView(r Result, carrier string, units kernel.Units, meta vehicles.Meta) IntervalView {
	if r.Status == "" {
		r.Status = StatusNone
	}
	v := IntervalView{Status: r.Status}
	if r.Reason != "" {
		reason := r.Reason
		v.Reason = &reason
	}
	if r.Status == StatusComputed || r.Status == StatusNotComputable {
		v.Quantity = quantityDisplay(r.Qty, carrier, units)
		v.Distance = distanceDisplay(r.Dist, units, meta)
	}
	if r.Status == StatusComputed {
		v.Consumption = consumptionDisplay(r.Qty, r.Dist, carrier, units)
	}
	return v
}

func (s *Service) viewOne(ctx context.Context, q store.DBTX, vehicleID, fillID uuid.UUID, units kernel.Units) (View, error) {
	d, err := s.data(ctx, q, vehicleID, false)
	if err != nil {
		return View{}, err
	}
	for _, r := range d.rows {
		if uuid.UUID(r.ID.Bytes) == fillID {
			return s.view(d, r, units), nil
		}
	}
	return View{}, problem.NotFound()
}

// Get liest einen Vorgang.
func (s *Service) Get(ctx context.Context, actor kernel.Actor, vehicleID, fillID uuid.UUID, units kernel.Units) (View, error) {
	row, err := store.New(s.pool).GetFill(ctx, pgU(fillID))
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && uuid.UUID(row.VehicleID.Bytes) != vehicleID) {
		return View{}, problem.NotFound()
	}
	if err != nil {
		return View{}, err
	}
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return View{}, err
	}
	return s.viewOne(ctx, s.pool, vehicleID, fillID, units)
}

// ListFilter filtert die Liste.
type ListFilter struct {
	From, To       *time.Time // Kalenderdaten, inklusive
	Carrier        string
	IncludeDeleted bool
	Cursor         *string
	Limit          int
}

// List liefert Vorgänge, neueste zuerst, mit Intervallverbrauch (ListFills).
func (s *Service) List(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, f ListFilter, units kernel.Units) ([]View, *string, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return nil, nil, err
	}
	d, err := s.data(ctx, s.pool, vehicleID, f.IncludeDeleted)
	if err != nil {
		return nil, nil, err
	}
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	var cur *kernel.Cursor
	if f.Cursor != nil {
		c, ok := kernel.DecodeCursor(*f.Cursor)
		if !ok {
			return nil, nil, problem.Validation(problem.FieldError{Pointer: "/cursor", Code: "invalid"})
		}
		cur = &c
	}
	rows := append([]store.FuelFill{}, d.rows...)
	sort.SliceStable(rows, func(i, j int) bool { return after(rows[i], rows[j]) })
	out := []View{}
	var next *string
	for _, r := range rows {
		if f.Carrier != "" && r.EnergyCarrier != f.Carrier {
			continue
		}
		day := kernel.LocalDate(r.OccurredAt.Time, r.TimeZone)
		if (f.From != nil && day.Before(*f.From)) || (f.To != nil && day.After(*f.To)) {
			continue
		}
		if cur != nil && !before(r, *cur) {
			continue
		}
		if len(out) == f.Limit {
			last := out[len(out)-1]
			c := kernel.Cursor{At: last.OccurredAt, ID: last.ID}.Encode()
			next = &c
			break
		}
		v := s.view(d, r, units)
		v.OccurredAt = r.OccurredAt.Time
		out = append(out, v)
	}
	return out, next, nil
}

func after(a, b store.FuelFill) bool {
	if !a.OccurredAt.Time.Equal(b.OccurredAt.Time) {
		return a.OccurredAt.Time.After(b.OccurredAt.Time)
	}
	return a.ID.String() > b.ID.String()
}

func before(r store.FuelFill, c kernel.Cursor) bool {
	if !r.OccurredAt.Time.Equal(c.At) {
		return r.OccurredAt.Time.Before(c.At)
	}
	return uuid.UUID(r.ID.Bytes).String() < c.ID.String()
}

// SeriesView entspricht dem Schema ConsumptionSeries.
type SeriesView struct {
	Average                *kernel.DisplayValue `json:"average"`
	ComputedIntervals      int                  `json:"computed_intervals"`
	NotComputableIntervals int                  `json:"not_computable_intervals"`
	Monthly                []MonthView          `json:"monthly"`
}

// MonthView entspricht dem Schema MonthlyValue.
type MonthView struct {
	Month    string               `json:"month"`
	Value    *kernel.DisplayValue `json:"value"`
	Distance *kernel.DisplayValue `json:"distance"`
	Quantity *kernel.DisplayValue `json:"quantity"`
}

// ConsumptionView entspricht dem Schema ConsumptionSummary.
type ConsumptionView struct {
	EnergyCarrier            string               `json:"energy_carrier"`
	From                     string               `json:"from"`
	To                       string               `json:"to"`
	Consumption              *SeriesView          `json:"consumption"`
	Grid                     *SeriesView          `json:"grid"`
	Battery                  *SeriesView          `json:"battery"`
	BatteryUnavailableReason *string              `json:"battery_unavailable_reason"`
	AverageUnitPrice         *kernel.DisplayValue `json:"average_unit_price"`
}

func seriesView(sum Summary, carrier string, units kernel.Units, meta vehicles.Meta) *SeriesView {
	v := &SeriesView{Average: consumptionDisplay(sum.Qty, sum.Dist, carrier, units), ComputedIntervals: sum.Computed,
		NotComputableIntervals: sum.NotComputable, Monthly: []MonthView{}}
	for _, m := range sum.Monthly {
		v.Monthly = append(v.Monthly, MonthView{Month: m.Month, Value: consumptionDisplay(m.Qty, m.Dist, carrier, units),
			Distance: distanceDisplay(m.Dist, units, meta), Quantity: quantityDisplay(m.Qty, carrier, units)})
	}
	return v
}

// Consumption berechnet Durchschnitt und Monatswerte (FU-04/FU-05).
// Ohne Zeitraum gilt: alles bis heute.
func (s *Service) Consumption(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, carrier string, from, to *time.Time, units kernel.Units) (ConsumptionView, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return ConsumptionView{}, err
	}
	d, err := s.data(ctx, s.pool, vehicleID, false)
	if err != nil {
		return ConsumptionView{}, err
	}
	fills := d.byCarr[carrier]
	if from == nil {
		t := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
		if len(fills) > 0 {
			t = kernel.LocalDate(fills[0].At, fills[0].TimeZone)
		}
		from = &t
	}
	if to == nil {
		t := kernel.LocalDate(s.Now(), d.meta.OwnerTimeZone)
		to = &t
	}
	out := ConsumptionView{EnergyCarrier: carrier, From: from.Format("2006-01-02"), To: to.Format("2006-01-02")}
	_, ivs := Intervals(fills, carrier == "electricity")
	sum := Summarize(ivs, from, to)
	if carrier == "electricity" {
		out.Grid = seriesView(sum, carrier, units, d.meta)
		if d.meta.BatteryWh <= 0 {
			r := "capacity_unknown"
			out.BatteryUnavailableReason = &r
		} else if b := Battery(fills, d.meta.BatteryWh); len(b) == 0 {
			r := "soc_missing"
			out.BatteryUnavailableReason = &r
		} else {
			out.Battery = seriesView(Summarize(b, from, to), carrier, units, d.meta)
		}
	} else {
		out.Consumption = seriesView(sum, carrier, units, d.meta)
	}
	// Ø Preis pro Einheit über Vorgänge mit Betrag im Zeitraum, je erste Währung.
	var minor, qty int64
	cur := ""
	for _, r := range d.rows {
		day := kernel.LocalDate(r.OccurredAt.Time, r.TimeZone)
		if r.DeletedAt.Valid || r.EnergyCarrier != carrier || !r.CostAmountMinor.Valid || day.Before(*from) || day.After(*to) {
			continue
		}
		if cur == "" {
			cur = r.CostCurrency.String
		}
		if r.CostCurrency.String == cur {
			minor += r.CostAmountMinor.Int64
			qty += r.Quantity
		}
	}
	if cur != "" {
		out.AverageUnitPrice = unitPrice(minor, cur, qty, carrier, units)
	}
	return out, nil
}
