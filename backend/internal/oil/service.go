package oil

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/odometer"
	"github.com/sh1su/vector/backend/internal/oil/store"
	"github.com/sh1su/vector/backend/internal/platform/audit"
	"github.com/sh1su/vector/backend/internal/platform/db"
	"github.com/sh1su/vector/backend/internal/platform/pg"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

const source = "oil"

// IncreaseFactor: Hinweis, wenn die aktuelle Reihe den Median der letzten drei
// abgeschlossenen Reihen um mehr als 50 % übersteigt (OI-04, konfigurierbar).
var IncreaseFactor = 1.5

// Service ist der Application Service des Moduls Oil.
type Service struct {
	pool *pgxpool.Pool
	odo  *odometer.Service
	Now  func() time.Time
}

func NewService(pool *pgxpool.Pool, odo *odometer.Service) *Service {
	return &Service{pool: pool, odo: odo, Now: time.Now}
}

// Level entspricht dem Schema OilLevelInput (E-13).
type Level struct {
	Percent *float64 `json:"percent"`
	Step    *string  `json:"step"`
}

// Input sind die änderbaren Felder (JSON-Namen wie in der API).
type Input struct {
	OccurredAt       time.Time   `json:"occurred_at"`
	TimeZone         string      `json:"time_zone"`
	TimePrecision    string      `json:"time_precision,omitempty"`
	Kind             string      `json:"kind"`
	Odometer         *vehicles.Q `json:"odometer"`
	LevelBefore      *Level      `json:"level_before"`
	LevelAfter       *Level      `json:"level_after"`
	OilAdded         *vehicles.Q `json:"oil_added"`
	OilChangeFill    *vehicles.Q `json:"oil_change_fill"`
	OilSpecification *string     `json:"oil_specification"`
	OilBrand         *string     `json:"oil_brand"`
	OilProduct       *string     `json:"oil_product"`
	FilterChanged    *bool       `json:"filter_changed"`
	ServiceEntryID   *uuid.UUID  `json:"service_entry_id"`
	Note             string      `json:"note"`
	Tags             []string    `json:"tags"`
}

// Confirmation sind bestätigte Befunde mit Begründung (ADR-010).
type Confirmation struct {
	Codes  []string
	Reason string
}

// PairView entspricht dem Schema OilPairResult.
type PairView struct {
	Status             string               `json:"status"`
	Reason             *string              `json:"reason"`
	ConsumptionPer1000 *kernel.DisplayValue `json:"consumption_per_1000"`
	Basis              *string              `json:"basis"`
	Negative           bool                 `json:"negative"`
}

// View entspricht dem Schema OilEntry.
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
	OdometerReadingID     *uuid.UUID             `json:"odometer_reading_id"`
	OdometerTotal         *odometer.QuantityView `json:"odometer_total"`
	LevelBeforePct        *float64               `json:"level_before_pct"`
	LevelAfterPct         *float64               `json:"level_after_pct"`
	LevelAfterPctComputed *float64               `json:"level_after_pct_computed"`
	SeriesID              uuid.UUID              `json:"series_id"`
	Pair                  PairView               `json:"pair"`
}

type record struct {
	in          Input
	at          kernel.EventTime
	beforePct   *float64
	beforeInput *string
	afterPct    *float64
	afterInput  *string
	added       *kernel.Quantity
	fill        *kernel.Quantity
}

func level(l *Level, ptr string, add func(p, c, m string)) (*float64, *string) {
	if l == nil || (l.Percent == nil && l.Step == nil) {
		return nil, nil
	}
	if l.Percent != nil && l.Step != nil {
		add(ptr, "exclusive", "Entweder Prozent oder Stufe angeben.")
		return nil, nil
	}
	if l.Percent != nil {
		if *l.Percent < -50 || *l.Percent > 150 {
			add(ptr+"/percent", "range", "Ölstand außerhalb −50 … 150 % (OI-05).")
			return nil, nil
		}
		v, in := kernel.Round(*l.Percent, 2), "percent"
		return &v, &in
	}
	pct, ok := Steps[*l.Step]
	if !ok {
		add(ptr+"/step", "enum", "")
		return nil, nil
	}
	in := *l.Step
	return pct, &in
}

func volume(q *vehicles.Q, ptr string, add func(p, c, m string)) *kernel.Quantity {
	if q == nil {
		return nil
	}
	c, err := kernel.ToCanonical(q.Value, q.Unit)
	if err != nil || c.CanonicalUnit != kernel.UnitMilliliter {
		add(ptr+"/unit", "unit", "Volumeneinheit erwartet (ml, l, qt).")
		return nil
	}
	if c.Canonical <= 0 {
		add(ptr+"/value", "range", "Die Menge muss größer als 0 sein.")
		return nil
	}
	return &c
}

func (s *Service) normalize(meta vehicles.Meta, in Input) (record, []problem.Anomaly, error) {
	var errs []problem.FieldError
	add := func(p, c, m string) { errs = append(errs, problem.FieldError{Pointer: p, Code: c, Message: m}) }
	r := record{in: in}
	et, err := kernel.NormalizeEventTime(in.OccurredAt, in.TimeZone, in.TimePrecision)
	if err != nil {
		add("/time_zone", "time", err.Error())
	}
	r.at = et
	r.beforePct, r.beforeInput = level(in.LevelBefore, "/level_before", add)
	r.afterPct, r.afterInput = level(in.LevelAfter, "/level_after", add)
	r.added = volume(in.OilAdded, "/oil_added", add)
	r.fill = volume(in.OilChangeFill, "/oil_change_fill", add)
	switch in.Kind {
	case KindCheck:
		if r.beforeInput == nil {
			add("/level_before", "required", "Eine Messung braucht den gemessenen Ölstand (I-OI-1).")
		}
		if in.OilAdded != nil || in.LevelAfter != nil {
			add("/oil_added", "not_allowed", "Eine Messung hat keine Nachfüllmenge.")
		}
	case KindTopUp:
		if in.OilAdded == nil {
			add("/oil_added", "required", "Eine Nachfüllung braucht die Menge (I-OI-1).")
		}
	case KindOilChange:
		if in.OilAdded != nil {
			add("/oil_added", "not_allowed", "Beim Ölwechsel die Einfüllmenge unter oil_change_fill angeben.")
		}
	default:
		add("/kind", "enum", "")
	}
	if in.Kind != KindOilChange && (in.OilChangeFill != nil || in.FilterChanged != nil) {
		add("/oil_change_fill", "not_allowed", "Nur beim Ölwechsel.")
	}
	if in.ServiceEntryID != nil {
		add("/service_entry_id", "not_found", "Der Serviceeintrag existiert nicht.")
	}
	for _, f := range []struct {
		p string
		v *string
		n int
	}{{"/oil_specification", in.OilSpecification, 200}, {"/oil_brand", in.OilBrand, 100}, {"/oil_product", in.OilProduct, 100}} {
		if f.v != nil && len([]rune(*f.v)) > f.n {
			add(f.p, "length", "")
		}
	}
	if len(in.Tags) > 20 {
		add("/tags", "max_items", "")
	}
	if len(errs) > 0 {
		return r, nil, problem.Validation(errs...)
	}
	var anomalies []problem.Anomaly
	if r.added != nil && meta.OilCapacityMl > 0 && r.added.Canonical > meta.OilCapacityMl {
		anomalies = append(anomalies, problem.Anomaly{Code: "OIL_CAPACITY", Confirmable: true, Message: "Die Menge ist größer als die Füllmenge des Motors."})
	}
	if in.Kind == KindTopUp && r.afterPct == nil && r.beforePct != nil && r.added != nil && meta.OilRangeMl > 0 {
		if *r.beforePct+float64(r.added.Canonical)/float64(meta.OilRangeMl)*100 > 120 {
			anomalies = append(anomalies, problem.Anomaly{Code: "OIL_OVERFILL", Confirmable: true, Message: "Möglicherweise überfüllt (berechneter Stand über 120 %)."})
		}
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

func (s *Service) planOdometer(ctx context.Context, tx pgx.Tx, meta vehicles.Meta, id uuid.UUID, r record) (*odometer.OwnedPlan, error) {
	if r.in.Odometer == nil {
		if meta.OdometerRequired {
			return nil, problem.Validation(problem.FieldError{Pointer: "/odometer", Code: "required", Message: "Für dieses Fahrzeug ist der Kilometerstand Pflicht (I-OI-2)."})
		}
		return s.odo.PlanOwned(ctx, tx, meta, source, id, nil)
	}
	return s.odo.PlanOwned(ctx, tx, meta, source, id, &odometer.Input{OccurredAt: r.at.At, TimeZone: r.at.TimeZone, Precision: r.at.Precision,
		Value: r.in.Odometer.Value, Unit: r.in.Odometer.Unit})
}

func confirm(own []problem.Anomaly, plan *odometer.OwnedPlan, c Confirmation) ([]odometer.AnomalyRecord, []odometer.AnomalyRecord, error) {
	all := append(append([]problem.Anomaly{}, own...), plan.Anomalies()...)
	records, err := odometer.ConfirmOrReject(all, c.Codes, c.Reason)
	if err != nil {
		return nil, nil, err
	}
	odo := []odometer.AnomalyRecord{}
	for _, r := range records {
		if len(r.Code) == 2 && r.Code[0] == 'P' {
			odo = append(odo, r)
		}
	}
	if records == nil {
		records = []odometer.AnomalyRecord{}
	}
	return records, odo, nil
}

type cols struct {
	beforePct, afterPct pgtype.Numeric
	beforeIn, afterIn   pgtype.Text
	added, fill         pgtype.Int8
	addedIn, fillIn     pgtype.Numeric
	addedUnit, fillUnit pgtype.Text
}

func colsOf(r record) cols {
	var c cols
	c.beforePct, c.afterPct = pg.Num(r.beforePct), pg.Num(r.afterPct)
	c.beforeIn, c.afterIn = pg.Text(r.beforeInput), pg.Text(r.afterInput)
	if r.added != nil {
		c.added, c.addedIn, c.addedUnit = pgtype.Int8{Int64: r.added.Canonical, Valid: true}, pg.NumStr(r.added.InputValue), pgtype.Text{String: r.added.InputUnit, Valid: true}
	}
	if r.fill != nil {
		c.fill, c.fillIn, c.fillUnit = pgtype.Int8{Int64: r.fill.Canonical, Valid: true}, pg.NumStr(r.fill.InputValue), pgtype.Text{String: r.fill.InputUnit, Valid: true}
	}
	return c
}

func nonNil(t []string) []string {
	if t == nil {
		return []string{}
	}
	return t
}

// Create erfasst einen Eintrag inklusive Messpunkt (RecordOilEntry).
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
		eid := kernel.NewID()
		if id != nil {
			eid = *id
			ex, err := q.GetEntryAny(ctx, pg.U(eid))
			if err == nil {
				cur := inputOf(ex, nil)
				if uuid.UUID(ex.VehicleID.Bytes) == vehicleID && !ex.DeletedAt.Valid && cur.Kind == in.Kind && cur.OccurredAt.Equal(in.OccurredAt) {
					created = false
					out, err = s.viewOne(ctx, tx, vehicleID, eid, units)
					return err
				}
				return problem.Conflict("Ein Öleintrag mit dieser ID existiert mit anderem Inhalt.")
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		r, own, err := s.normalize(meta, in)
		if err != nil {
			return err
		}
		plan, err := s.planOdometer(ctx, tx, meta, eid, r)
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
		k := colsOf(r)
		ra, _ := json.Marshal(records)
		if _, err := q.InsertEntry(ctx, store.InsertEntryParams{ID: pg.U(eid), VehicleID: pg.U(vehicleID), Kind: in.Kind,
			OccurredAt: pgtype.Timestamptz{Time: r.at.At, Valid: true}, TimeZone: r.at.TimeZone, TimePrecision: r.at.Precision,
			OdometerReadingID: pg.Up(readingID), LevelBeforePct: k.beforePct, LevelBeforeInput: k.beforeIn, LevelAfterPct: k.afterPct,
			LevelAfterInput: k.afterIn, OilAddedMl: k.added, InputAdded: k.addedIn, InputAddedUnit: k.addedUnit, OilChangeFillMl: k.fill,
			InputFill: k.fillIn, InputFillUnit: k.fillUnit, OilSpecification: pg.Text(in.OilSpecification), OilBrand: pg.Text(in.OilBrand),
			OilProduct: pg.Text(in.OilProduct), FilterChanged: pg.Bool(in.FilterChanged), Note: in.Note, Tags: nonNil(in.Tags),
			ConfirmedAnomalies: ra, Origin: odometer.OriginOf(actor), CreatedBy: pg.U(actor.AccountID)}); err != nil {
			return err
		}
		action := "oil.entry_recorded"
		if in.Kind == KindOilChange {
			action = "oil.oil_change_recorded"
		}
		if err := audit.Write(ctx, tx, actor, audit.Event{Action: action, VehicleID: &vehicleID, ObjectType: "oil_entry", ObjectID: eid,
			Changes: map[string]any{"kind": in.Kind, "anomalies": records}, Reason: c.Reason}); err != nil {
			return err
		}
		out, err = s.viewOne(ctx, tx, vehicleID, eid, units)
		return err
	})
	return out, created, err
}

func (s *Service) loadEntry(ctx context.Context, tx pgx.Tx, actor kernel.Actor, vehicleID, entryID uuid.UUID, role string) (store.OilEntry, error) {
	row, err := store.New(tx).GetEntry(ctx, pg.U(entryID))
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && uuid.UUID(row.VehicleID.Bytes) != vehicleID) {
		return row, problem.NotFound()
	}
	if err != nil {
		return row, err
	}
	_, err = identity.Authorize(ctx, tx, actor, uuid.UUID(row.VehicleID.Bytes), role)
	return row, err
}

// Update ändert einen Eintrag (JSON Merge Patch, If-Match).
func (s *Service) Update(ctx context.Context, actor kernel.Actor, vehicleID, entryID uuid.UUID, ifMatch string, patch []byte, c Confirmation, units kernel.Units) (View, error) {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return View{}, err
	}
	var out View
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		row, err := s.loadEntry(ctx, tx, actor, vehicleID, entryID, identity.RoleEditor)
		if err != nil {
			return err
		}
		meta, err := vehicles.LoadMeta(ctx, tx, vehicleID, true)
		if err != nil {
			return err
		}
		if int(row.Version) != version {
			cur, _ := s.viewOne(ctx, tx, vehicleID, entryID, units)
			return problem.PreconditionFailed(cur)
		}
		segs, valid, err := s.odo.State(ctx, tx, vehicleID)
		if err != nil {
			return err
		}
		var or *odometer.OwnedReading
		if o, ok := odometer.Owned(valid, segs, source)[entryID]; ok {
			or = &o
		}
		cur := inputOf(row, or)
		// Ein Ölstand ist entweder Prozent oder Stufe: ein neuer Wert ersetzt das ganze Objekt.
		var keys map[string]json.RawMessage
		_ = json.Unmarshal(patch, &keys)
		if _, ok := keys["level_before"]; ok {
			cur.LevelBefore = nil
		}
		if _, ok := keys["level_after"]; ok {
			cur.LevelAfter = nil
		}
		var in Input
		if err := kernel.MergePatch(cur, patch, &in); err != nil {
			return problem.BadRequest("Ungültiger Merge Patch.")
		}
		r, own, err := s.normalize(meta, in)
		if err != nil {
			return err
		}
		plan, err := s.planOdometer(ctx, tx, meta, entryID, r)
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
		k := colsOf(r)
		ra, _ := json.Marshal(records)
		if _, err := store.New(tx).UpdateEntry(ctx, store.UpdateEntryParams{ID: row.ID, Kind: in.Kind,
			OccurredAt: pgtype.Timestamptz{Time: r.at.At, Valid: true}, TimeZone: r.at.TimeZone, TimePrecision: r.at.Precision,
			OdometerReadingID: pg.Up(readingID), LevelBeforePct: k.beforePct, LevelBeforeInput: k.beforeIn, LevelAfterPct: k.afterPct,
			LevelAfterInput: k.afterIn, OilAddedMl: k.added, InputAdded: k.addedIn, InputAddedUnit: k.addedUnit, OilChangeFillMl: k.fill,
			InputFill: k.fillIn, InputFillUnit: k.fillUnit, OilSpecification: pg.Text(in.OilSpecification), OilBrand: pg.Text(in.OilBrand),
			OilProduct: pg.Text(in.OilProduct), FilterChanged: pg.Bool(in.FilterChanged), Note: in.Note, Tags: nonNil(in.Tags),
			ConfirmedAnomalies: ra, UpdatedBy: pg.U(actor.AccountID), Version: row.Version}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return problem.PreconditionFailed(nil)
			}
			return err
		}
		if err := audit.Write(ctx, tx, actor, audit.Event{Action: "oil.entry_updated", VehicleID: &vehicleID, ObjectType: "oil_entry", ObjectID: entryID,
			Changes: map[string]any{"patch": json.RawMessage(patch), "anomalies": records}, Reason: c.Reason}); err != nil {
			return err
		}
		out, err = s.viewOne(ctx, tx, vehicleID, entryID, units)
		return err
	})
	return out, err
}

// Delete löscht einen Eintrag weich, inklusive Messpunkt.
func (s *Service) Delete(ctx context.Context, actor kernel.Actor, vehicleID, entryID uuid.UUID, ifMatch string) error {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return err
	}
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		row, err := s.loadEntry(ctx, tx, actor, vehicleID, entryID, identity.RoleEditor)
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
		plan, err := s.odo.PlanOwned(ctx, tx, meta, source, entryID, nil)
		if err != nil {
			return err
		}
		if _, err := s.odo.ApplyOwned(ctx, tx, actor, plan, nil); err != nil {
			return err
		}
		n, err := store.New(tx).SoftDeleteEntry(ctx, store.SoftDeleteEntryParams{ID: row.ID, UpdatedBy: pg.U(actor.AccountID), Version: row.Version})
		if err != nil {
			return err
		}
		if n == 0 {
			return problem.PreconditionFailed(nil)
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "oil.entry_deleted", VehicleID: &vehicleID, ObjectType: "oil_entry", ObjectID: entryID})
	})
}

func levelOf(pct pgtype.Numeric, in pgtype.Text) *Level {
	if !in.Valid {
		return nil
	}
	if in.String == "percent" {
		return &Level{Percent: pg.NumPtr(pct)}
	}
	s := in.String
	return &Level{Step: &s}
}

func qOf(n pgtype.Numeric, u pgtype.Text) *vehicles.Q {
	if !n.Valid || !u.Valid {
		return nil
	}
	return &vehicles.Q{Value: *pg.NumPtr(n), Unit: u.String}
}

func inputOf(r store.OilEntry, or *odometer.OwnedReading) Input {
	in := Input{OccurredAt: r.OccurredAt.Time, TimeZone: r.TimeZone, TimePrecision: r.TimePrecision, Kind: r.Kind,
		LevelBefore: levelOf(r.LevelBeforePct, r.LevelBeforeInput), LevelAfter: levelOf(r.LevelAfterPct, r.LevelAfterInput),
		OilAdded: qOf(r.InputAdded, r.InputAddedUnit), OilChangeFill: qOf(r.InputFill, r.InputFillUnit),
		OilSpecification: pg.TextPtr(r.OilSpecification), OilBrand: pg.TextPtr(r.OilBrand), OilProduct: pg.TextPtr(r.OilProduct),
		FilterChanged: pg.BoolPtr(r.FilterChanged), ServiceEntryID: pg.UUIDPtr(r.ServiceEntryID), Note: r.Note, Tags: nonNil(r.Tags)}
	if or != nil {
		in.Odometer = &vehicles.Q{Value: or.InputValue, Unit: or.InputUnit}
	}
	return in
}

// vehicleData sind alle Einträge eines Fahrzeugs mit Messreihen.
type vehicleData struct {
	meta     vehicles.Meta
	rows     []store.OilEntry
	owned    map[uuid.UUID]odometer.OwnedReading
	segs     odometer.Segments
	valid    []odometer.Reading
	entries  []Entry
	series   []Series
	seriesOf map[uuid.UUID]uuid.UUID
}

func toEntry(r store.OilEntry, owned map[uuid.UUID]odometer.OwnedReading) Entry {
	e := Entry{ID: uuid.UUID(r.ID.Bytes), Kind: r.Kind, At: r.OccurredAt.Time, TimeZone: r.TimeZone, RecordedAt: r.RecordedAt.Time,
		LevelBefore: pg.NumPtr(r.LevelBeforePct), LevelAfter: pg.NumPtr(r.LevelAfterPct)}
	if r.OilAddedMl.Valid {
		e.AddedMl = r.OilAddedMl.Int64
	}
	if r.OilChangeFillMl.Valid {
		e.FillMl = r.OilChangeFillMl.Int64
	}
	if o, ok := owned[e.ID]; ok {
		t := o.Total
		e.Total = &t
	}
	return e
}

func (s *Service) data(ctx context.Context, q store.DBTX, vehicleID uuid.UUID, includeDeleted bool) (vehicleData, error) {
	var d vehicleData
	meta, err := vehicles.LoadMeta(ctx, q, vehicleID, false)
	if err != nil {
		return d, err
	}
	rows, err := store.New(q).ListEntries(ctx, store.ListEntriesParams{VehicleID: pg.U(vehicleID), IncludeDeleted: includeDeleted})
	if err != nil {
		return d, err
	}
	segs, valid, err := s.odo.State(ctx, q, vehicleID)
	if err != nil {
		return d, err
	}
	d = vehicleData{meta: meta, rows: rows, owned: odometer.Owned(valid, segs, source), segs: segs, valid: valid, seriesOf: map[uuid.UUID]uuid.UUID{}}
	for _, r := range rows {
		if !r.DeletedAt.Valid {
			d.entries = append(d.entries, toEntry(r, d.owned))
		}
	}
	Sort(d.entries)
	d.series = BuildSeries(vehicleID, d.entries, meta.OilRangeMl)
	for _, sr := range d.series {
		for _, e := range sr.Entries {
			d.seriesOf[e.ID] = sr.ID
		}
	}
	return d, nil
}

func per1000Display(v float64, dist int64, basis string, units kernel.Units) *kernel.DisplayValue {
	if dist <= 0 {
		return nil
	}
	per := Per1000(v, dist, units.Distance)
	if basis == "percent_points" {
		return &kernel.DisplayValue{Value: kernel.Round(per, 1), Unit: "%-Punkte/1000 " + units.Distance}
	}
	f, _ := kernel.FromCanonical(1, units.OilVolume)
	digits := 0
	if units.OilVolume != "ml" {
		digits = 3
	}
	return &kernel.DisplayValue{Value: kernel.Round(per*f, digits), Unit: kernel.UnitLabel(units.OilVolume) + "/1000 " + units.Distance}
}

func (s *Service) view(d vehicleData, r store.OilEntry, units kernel.Units) View {
	id := uuid.UUID(r.ID.Bytes)
	var or *odometer.OwnedReading
	if o, ok := d.owned[id]; ok {
		or = &o
	}
	v := View{ID: id, Version: int(r.Version), VehicleID: uuid.UUID(r.VehicleID.Bytes), CreatedAt: r.CreatedAt.Time, CreatedBy: uuid.UUID(r.CreatedBy.Bytes),
		UpdatedAt: r.UpdatedAt.Time, UpdatedBy: uuid.UUID(r.UpdatedBy.Bytes), RecordedAt: r.RecordedAt.Time, Origin: r.Origin, Input: inputOf(r, or),
		LevelBeforePct: pg.NumPtr(r.LevelBeforePct), LevelAfterPct: pg.NumPtr(r.LevelAfterPct), SeriesID: d.seriesOf[id], Pair: PairView{Status: "none"}}
	if or != nil {
		rid := or.ID
		v.OdometerReadingID = &rid
		v.OdometerTotal = &odometer.QuantityView{Canonical: or.Total, CanonicalUnit: kernel.UnitMeter}
	}
	if r.DeletedAt.Valid {
		return v
	}
	e := toEntry(r, d.owned)
	if after, computed := AfterLevel(e, d.meta.OilRangeMl); computed && after != nil {
		x := kernel.Round(*after, 1)
		v.LevelAfterPctComputed = &x
	}
	for _, sr := range d.series {
		if p, ok := sr.Pairs[id]; ok {
			pv := PairView{Status: p.Status, Negative: p.Negative}
			if p.Reason != "" {
				reason := p.Reason
				pv.Reason = &reason
			}
			if p.Status == "computed" {
				basis := p.Basis
				pv.Basis = &basis
				pv.ConsumptionPer1000 = per1000Display(p.Value, p.Dist, p.Basis, units)
			}
			v.Pair = pv
		}
	}
	return v
}

func (s *Service) viewOne(ctx context.Context, q store.DBTX, vehicleID, entryID uuid.UUID, units kernel.Units) (View, error) {
	d, err := s.data(ctx, q, vehicleID, false)
	if err != nil {
		return View{}, err
	}
	for _, r := range d.rows {
		if uuid.UUID(r.ID.Bytes) == entryID {
			return s.view(d, r, units), nil
		}
	}
	return View{}, problem.NotFound()
}

// Get liest einen Eintrag.
func (s *Service) Get(ctx context.Context, actor kernel.Actor, vehicleID, entryID uuid.UUID, units kernel.Units) (View, error) {
	row, err := store.New(s.pool).GetEntry(ctx, pg.U(entryID))
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && uuid.UUID(row.VehicleID.Bytes) != vehicleID) {
		return View{}, problem.NotFound()
	}
	if err != nil {
		return View{}, err
	}
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return View{}, err
	}
	return s.viewOne(ctx, s.pool, vehicleID, entryID, units)
}

// ListFilter filtert die Liste.
type ListFilter struct {
	From, To       *time.Time
	IncludeDeleted bool
	Cursor         *string
	Limit          int
}

// List liefert Einträge, neueste zuerst, mit Paarverbrauch.
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
	rows := append([]store.OilEntry{}, d.rows...)
	sort.SliceStable(rows, func(i, j int) bool {
		if !rows[i].OccurredAt.Time.Equal(rows[j].OccurredAt.Time) {
			return rows[i].OccurredAt.Time.After(rows[j].OccurredAt.Time)
		}
		return uuid.UUID(rows[i].ID.Bytes).String() > uuid.UUID(rows[j].ID.Bytes).String()
	})
	out := []View{}
	var next *string
	for _, r := range rows {
		day := kernel.LocalDate(r.OccurredAt.Time, r.TimeZone)
		if (f.From != nil && day.Before(*f.From)) || (f.To != nil && day.After(*f.To)) {
			continue
		}
		if cur != nil {
			at, id := r.OccurredAt.Time, uuid.UUID(r.ID.Bytes)
			if at.After(cur.At) || (at.Equal(cur.At) && id.String() >= cur.ID.String()) {
				continue
			}
		}
		if len(out) == f.Limit {
			last := out[len(out)-1]
			c := kernel.Cursor{At: last.OccurredAt, ID: last.ID}.Encode()
			next = &c
			break
		}
		out = append(out, s.view(d, r, units))
	}
	return out, next, nil
}

// SeriesView entspricht dem Schema OilSeries.
type SeriesView struct {
	ID                 uuid.UUID            `json:"id"`
	StartedByEntryID   *uuid.UUID           `json:"started_by_entry_id"`
	StartedAt          *time.Time           `json:"started_at"`
	DistanceSinceStart *kernel.DisplayValue `json:"distance_since_start"`
	ConsumptionPer1000 *kernel.DisplayValue `json:"consumption_per_1000"`
	Basis              *string              `json:"basis"`
	UsableMeasurements int                  `json:"usable_measurements"`
	Status             string               `json:"status"`
	value              *float64
}

func (s *Service) seriesViews(d vehicleData, units kernel.Units) []SeriesView {
	cur := odometer.Current(d.valid, d.segs)
	out := make([]SeriesView, 0, len(d.series))
	for i, sr := range d.series {
		v := SeriesView{ID: sr.ID, UsableMeasurements: sr.Usable, Status: "ok"}
		if sr.Usable < 2 {
			v.Status = "too_few_measurements"
		}
		if sr.StartEntry != nil {
			id, at := sr.StartEntry.ID, sr.StartEntry.At
			v.StartedByEntryID, v.StartedAt = &id, &at
			var end *int64
			if i+1 < len(d.series) {
				end = d.series[i+1].StartEntry.Total
			} else if cur.Kind != odometer.KindUnknown {
				t := cur.Total
				end = &t
			}
			if end != nil && sr.StartEntry.Total != nil {
				v.DistanceSinceStart = &kernel.DisplayValue{Value: kernel.Display(*end-*sr.StartEntry.Total, units.Distance, 0), Unit: units.Distance}
			}
		}
		if sr.SumDist > 0 && v.Status == "ok" {
			basis := sr.Basis
			v.Basis = &basis
			v.ConsumptionPer1000 = per1000Display(sr.SumValue, sr.SumDist, sr.Basis, units)
			x := Per1000(sr.SumValue, sr.SumDist, "km")
			v.value = &x
		}
		out = append(out, v)
	}
	return out
}

// Series listet die Messreihen, neueste zuerst (OilSeries).
func (s *Service) Series(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, units kernel.Units) ([]SeriesView, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return nil, err
	}
	d, err := s.data(ctx, s.pool, vehicleID, false)
	if err != nil {
		return nil, err
	}
	v := s.seriesViews(d, units)
	for i, j := 0, len(v)-1; i < j; i, j = i+1, j-1 {
		v[i], v[j] = v[j], v[i]
	}
	return v, nil
}

// MonthView entspricht dem Schema MonthlyValue.
type MonthView struct {
	Month    string               `json:"month"`
	Value    *kernel.DisplayValue `json:"value"`
	Distance *kernel.DisplayValue `json:"distance"`
	Quantity *kernel.DisplayValue `json:"quantity"`
}

// StatsView entspricht dem Schema OilStatistics.
type StatsView struct {
	From                       string               `json:"from"`
	To                         string               `json:"to"`
	TopUpRate                  *kernel.DisplayValue `json:"top_up_rate"`
	TopUpRateUnavailableReason *string              `json:"top_up_rate_unavailable_reason"`
	TotalAdded                 kernel.DisplayValue  `json:"total_added"`
	TopUpCount                 int                  `json:"top_up_count"`
	OilChangeCount             int                  `json:"oil_change_count"`
	OilChangeFillTotal         *kernel.DisplayValue `json:"oil_change_fill_total"`
	Hints                      []string             `json:"hints"`
	MonthlyTopUpRate           []MonthView          `json:"monthly_top_up_rate"`
}

func volDisplay(ml int64, units kernel.Units) kernel.DisplayValue {
	digits := 3
	if units.OilVolume == "ml" {
		digits = 0
	}
	return kernel.DisplayValue{Value: kernel.Display(ml, units.OilVolume, digits), Unit: kernel.UnitLabel(units.OilVolume)}
}

func dayStart(d time.Time, tz string) time.Time {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
}

// Statistics berechnet OI-01, OI-03 und OI-04 für einen Zeitraum (Kalenderdaten, inklusive).
func (s *Service) Statistics(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, from, to *time.Time, units kernel.Units) (StatsView, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return StatsView{}, err
	}
	d, err := s.data(ctx, s.pool, vehicleID, false)
	if err != nil {
		return StatsView{}, err
	}
	tz := d.meta.OwnerTimeZone
	if to == nil {
		t := kernel.LocalDate(s.Now(), tz)
		to = &t
	}
	if from == nil {
		t := to.AddDate(-1, 0, 0)
		if len(d.entries) > 0 {
			if first := LocalDay(d.entries[0]); first.Before(t) {
				t = first
			}
		}
		from = &t
	}
	if to.Before(*from) {
		return StatsView{}, problem.Validation(problem.FieldError{Pointer: "/to", Code: "order", Message: "`to` liegt vor `from`."})
	}
	t1, t2 := dayStart(*from, tz), dayStart(to.AddDate(0, 0, 1), tz)
	st := Totals(d.entries, t1, t2)
	out := StatsView{From: from.Format("2006-01-02"), To: to.Format("2006-01-02"), TotalAdded: volDisplay(st.AddedMl, units),
		TopUpCount: st.TopUps, OilChangeCount: st.OilChanges, Hints: []string{}, MonthlyTopUpRate: []MonthView{}}
	out.TopUpRate, out.TopUpRateUnavailableReason = s.rate(d, st.AddedMl, t1, t2, units)
	if st.HasChangeFill {
		v := volDisplay(st.ChangeFillMl, units)
		out.OilChangeFillTotal = &v
	}
	if st.OilChangeInRange {
		out.Hints = append(out.Hints, "oil_change_in_range")
	}
	if increase(s.seriesViews(d, units)) {
		out.Hints = append(out.Hints, "consumption_increase")
	}
	for m := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC); !m.After(*to); m = m.AddDate(0, 1, 0) {
		a, b := dayStart(m, tz), dayStart(m.AddDate(0, 1, 0), tz)
		if a.Before(t1) {
			a = t1
		}
		if b.After(t2) {
			b = t2
		}
		ms := Totals(d.entries, a, b)
		rate, _ := s.rate(d, ms.AddedMl, a, b, units)
		q := volDisplay(ms.AddedMl, units)
		mv := MonthView{Month: m.Format("2006-01"), Value: rate, Quantity: &q}
		if dist := odometer.DistanceBetween(d.valid, d.segs, a, b); dist.Known {
			mv.Distance = &kernel.DisplayValue{Value: kernel.Display(dist.Meters, units.Distance, 0), Unit: units.Distance}
		}
		out.MonthlyTopUpRate = append(out.MonthlyTopUpRate, mv)
	}
	return out, nil
}

// rate: OI-01 mit den Randfällen „Distanz unbekannt“ und „Distanz = 0“.
func (s *Service) rate(d vehicleData, addedMl int64, t1, t2 time.Time, units kernel.Units) (*kernel.DisplayValue, *string) {
	dist := odometer.DistanceBetween(d.valid, d.segs, t1, t2)
	if !dist.Known {
		r := "distance_unknown"
		return nil, &r
	}
	if dist.Meters <= 0 {
		r := "distance_zero"
		return nil, &r
	}
	return per1000Display(float64(addedMl), dist.Meters, "volume", units), nil
}

// increase: aktuelle Reihe > Median der letzten drei abgeschlossenen Reihen × Faktor (OI-04).
func increase(series []SeriesView) bool {
	if len(series) < 2 || series[len(series)-1].value == nil {
		return false
	}
	var closed []float64
	for i := len(series) - 2; i >= 0 && len(closed) < 3; i-- {
		if series[i].value != nil {
			closed = append(closed, *series[i].value)
		}
	}
	if len(closed) == 0 {
		return false
	}
	m := Median(closed)
	return m > 0 && *series[len(series)-1].value > m*IncreaseFactor
}
