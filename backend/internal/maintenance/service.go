package maintenance

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/maintenance/store"
	"github.com/sh1su/vector/backend/internal/odometer"
	"github.com/sh1su/vector/backend/internal/platform/audit"
	"github.com/sh1su/vector/backend/internal/platform/mergepatch"
	pg "github.com/sh1su/vector/backend/internal/platform/pgconv"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

// Service ist der Application Service des Moduls Maintenance.
type Service struct {
	pool *pgxpool.Pool
	odo  *odometer.Service
	Now  func() time.Time
}

func NewService(pool *pgxpool.Pool, odo *odometer.Service) *Service {
	return &Service{pool: pool, odo: odo, Now: time.Now}
}

var categories = map[string]bool{"service": true, "legal_inspection": true, "tires": true, "fluids": true, "brakes": true, "filters": true, "other": true}

// ThresholdsInput entspricht dem API-Schema MaintenanceThresholds.
type ThresholdsInput struct {
	UpcomingDays     *int        `json:"upcoming_days,omitempty"`
	DueDays          *int        `json:"due_days,omitempty"`
	UpcomingDistance *vehicles.Q `json:"upcoming_distance,omitempty"`
	DueDistance      *vehicles.Q `json:"due_distance,omitempty"`
}

// ItemInput sind die änderbaren Felder einer Definition (JSON wie API).
type ItemInput struct {
	Title                   string          `json:"title"`
	Description             *string         `json:"description,omitempty"`
	Category                string          `json:"category"`
	ManufacturerRecommended bool            `json:"manufacturer_recommended"`
	SourceDocumentID        *uuid.UUID      `json:"source_document_id,omitempty"`
	SourcePage              *int            `json:"source_page,omitempty"`
	ScheduleMode            string          `json:"schedule_mode"`
	IntervalMonths          *int            `json:"interval_months,omitempty"`
	IntervalDays            *int            `json:"interval_days,omitempty"`
	IntervalDistance        *vehicles.Q     `json:"interval_distance,omitempty"`
	AnchorDate              *string         `json:"anchor_date,omitempty"`
	AnchorOdometer          *vehicles.Q     `json:"anchor_odometer,omitempty"`
	DueDateOnce             *string         `json:"due_date_once,omitempty"`
	DueOdometerOnce         *vehicles.Q     `json:"due_odometer_once,omitempty"`
	Thresholds              ThresholdsInput `json:"thresholds"`
	Active                  *bool           `json:"active,omitempty"`
	Note                    string          `json:"note"`
}

// DueStatusView entspricht dem API-Schema DueStatus.
type DueStatusView struct {
	ItemID            uuid.UUID              `json:"item_id"`
	VehicleID         uuid.UUID              `json:"vehicle_id"`
	Title             string                 `json:"title"`
	Level             string                 `json:"level"`
	ReasonTrigger     *string                `json:"reason_trigger"`
	DueDate           *string                `json:"due_date"`
	DaysRemaining     *int                   `json:"days_remaining"`
	DueTotal          *odometer.QuantityView `json:"due_total"`
	DistanceRemaining *kernel.DisplayValue   `json:"distance_remaining"`
	EstimatedDueDate  *string                `json:"estimated_due_date"`
	Estimated         bool                   `json:"estimated"`
}

// ItemView entspricht dem API-Schema MaintenanceItem.
type ItemView struct {
	kernel.EntityMeta
	ItemInput
	Status DueStatusView `json:"status"`
}

// CompletionView entspricht dem API-Schema MaintenanceCompletion.
type CompletionView struct {
	ID                uuid.UUID   `json:"id"`
	Kind              string      `json:"kind"`
	CompletedOn       string      `json:"completed_on"`
	CompletedOdometer *vehicles.Q `json:"completed_odometer"`
	ServiceEntryID    *uuid.UUID  `json:"service_entry_id"`
	Reason            *string     `json:"reason"`
}

func dateStr(d *time.Time) *string {
	if d == nil {
		return nil
	}
	s := kernel.FormatDate(*d)
	return &s
}

// qty rechnet einen kanonischen Wert in die gespeicherte Eingabeeinheit zurück.
func qty(v *int64, unit string) *vehicles.Q {
	if v == nil {
		return nil
	}
	f, err := kernel.FromCanonical(*v, unit)
	if err != nil {
		return nil
	}
	return &vehicles.Q{Value: kernel.Round(f, 3), Unit: unit}
}

func itemInput(r store.MaintenanceItem) ItemInput {
	active := r.Active
	u := r.DistanceUnit
	return ItemInput{Title: r.Title, Description: pg.TP(r.Description), Category: r.Category, ManufacturerRecommended: r.ManufacturerRecommended,
		SourceDocumentID: pg.IDP(r.SourceDocumentID), SourcePage: pg.I4P(r.SourcePage), ScheduleMode: r.ScheduleMode,
		IntervalMonths: pg.I4P(r.IntervalMonths), IntervalDays: pg.I4P(r.IntervalDays), IntervalDistance: qty(pg.I8P(r.IntervalDistance), u),
		AnchorDate: dateStr(pg.DateP(r.AnchorDate)), AnchorOdometer: qty(pg.I8P(r.AnchorTotal), u), DueDateOnce: dateStr(pg.DateP(r.DueDateOnce)),
		DueOdometerOnce: qty(pg.I8P(r.DueTotalOnce), u),
		Thresholds: ThresholdsInput{UpcomingDays: pg.I4P(r.UpcomingDays), DueDays: pg.I4P(r.DueDays), UpcomingDistance: qty(pg.I8P(r.UpcomingDistance), u),
			DueDistance: qty(pg.I8P(r.DueDistance), u)},
		Active: &active, Note: r.Note}
}

func meta(r store.MaintenanceItem) kernel.EntityMeta {
	upd, rec, by := r.UpdatedAt.Time, r.RecordedAt.Time, pg.ID(r.UpdatedBy)
	return kernel.EntityMeta{ID: pg.ID(r.ID), Version: int(r.Version), VehicleID: pg.ID(r.VehicleID), CreatedAt: r.CreatedAt.Time,
		CreatedBy: pg.ID(r.CreatedBy), UpdatedAt: &upd, UpdatedBy: &by, RecordedAt: &rec, Origin: r.Origin}
}

func domainItem(r store.MaintenanceItem, engineHours bool) Item {
	it := Item{Mode: r.ScheduleMode, AnchorDate: pg.DateP(r.AnchorDate), AnchorTotal: pg.I8P(r.AnchorTotal), DueDateOnce: pg.DateP(r.DueDateOnce),
		DueTotalOnce: pg.I8P(r.DueTotalOnce), Thresholds: DefaultThresholds(engineHours)}
	if v := pg.I4P(r.IntervalMonths); v != nil {
		it.Months = *v
	}
	if v := pg.I4P(r.IntervalDays); v != nil {
		it.Days = *v
	}
	if v := pg.I8P(r.IntervalDistance); v != nil {
		it.Distance = *v
	}
	// MA-05: Schwellen der Definition, sonst Vorgabe – nie über Definitionen hinweg
	if v := pg.I4P(r.UpcomingDays); v != nil {
		it.Thresholds.UpcomingDays = *v
	}
	if v := pg.I4P(r.DueDays); v != nil {
		it.Thresholds.DueDays = *v
	}
	if v := pg.I8P(r.UpcomingDistance); v != nil {
		it.Thresholds.UpcomingDistance = *v
	}
	if v := pg.I8P(r.DueDistance); v != nil {
		it.Thresholds.DueDistance = *v
	}
	return it
}

// parsed ist eine validierte Definition in Speicherform.
type parsed struct {
	intervalDistance, anchorTotal, dueTotalOnce, upcomingDistance, dueDistance *int64
	anchorDate, dueDateOnce                                                    *time.Time
	unit                                                                       string
}

func (in *ItemInput) validate(meta vehicles.Meta) (parsed, error) {
	var errs []problem.FieldError
	add := func(p, c, m string) { errs = append(errs, problem.FieldError{Pointer: p, Code: c, Message: m}) }
	out := parsed{unit: "km"}
	if meta.UsageMeter == odometer.MeterEngineHours {
		out.unit = "h"
	}
	in.Title = strings.TrimSpace(in.Title)
	if n := len([]rune(in.Title)); n < 1 || n > 200 {
		add("/title", "length", "1–200 Zeichen")
	}
	if !categories[in.Category] {
		add("/category", "enum", "")
	}
	switch in.ScheduleMode {
	case ModeOnce, ModeFromLast, ModeFixedGrid:
	default:
		add("/schedule_mode", "enum", "")
	}
	if in.IntervalMonths != nil && in.IntervalDays != nil {
		add("/interval_days", "one_of", "Höchstens eines von Monaten oder Tagen.")
	}
	for p, v := range map[string]*int{"/interval_months": in.IntervalMonths, "/interval_days": in.IntervalDays} {
		if v != nil && *v <= 0 {
			add(p, "range", "Intervall muss größer als 0 sein (I-MA-2).")
		}
	}
	unitSet := false
	dist := func(p string, q *vehicles.Q) *int64 {
		if q == nil {
			return nil
		}
		ok := q.Unit == "km" || q.Unit == "mi" || q.Unit == "m"
		if meta.UsageMeter == odometer.MeterEngineHours {
			ok = q.Unit == "h" || q.Unit == "s"
		}
		if !ok {
			add(p+"/unit", "unit", "Einheit passt nicht zur Zählergröße des Fahrzeugs (I-MA-4).")
			return nil
		}
		c, err := kernel.ToCanonical(q.Value, q.Unit)
		if err != nil || q.Value < 0 {
			add(p+"/value", "range", "")
			return nil
		}
		if !unitSet && q.Unit != "m" && q.Unit != "s" {
			out.unit, unitSet = q.Unit, true
		}
		return &c.Canonical
	}
	out.intervalDistance = dist("/interval_distance", in.IntervalDistance)
	if out.intervalDistance != nil && *out.intervalDistance <= 0 {
		add("/interval_distance/value", "range", "Intervall muss größer als 0 sein (I-MA-2).")
	}
	out.anchorTotal = dist("/anchor_odometer", in.AnchorOdometer)
	out.dueTotalOnce = dist("/due_odometer_once", in.DueOdometerOnce)
	out.upcomingDistance = dist("/thresholds/upcoming_distance", in.Thresholds.UpcomingDistance)
	out.dueDistance = dist("/thresholds/due_distance", in.Thresholds.DueDistance)
	date := func(p string, s *string) *time.Time {
		if s == nil {
			return nil
		}
		d, err := kernel.ParseDate(*s)
		if err != nil {
			add(p, "date", "")
			return nil
		}
		return &d
	}
	out.anchorDate = date("/anchor_date", in.AnchorDate)
	out.dueDateOnce = date("/due_date_once", in.DueDateOnce)
	for p, v := range map[string]*int{"/thresholds/upcoming_days": in.Thresholds.UpcomingDays, "/thresholds/due_days": in.Thresholds.DueDays} {
		if v != nil && *v < 0 {
			add(p, "range", "")
		}
	}
	if in.SourcePage != nil && *in.SourcePage < 1 {
		add("/source_page", "range", "")
	}
	hasTime := in.IntervalMonths != nil || in.IntervalDays != nil
	// I-MA-1: mindestens ein Auslöser
	switch in.ScheduleMode {
	case ModeOnce:
		if in.DueDateOnce == nil && in.DueOdometerOnce == nil {
			add("/due_date_once", "required", "Fälligkeitsdatum oder -stand angeben (I-MA-1).")
		}
	case ModeFromLast, ModeFixedGrid:
		if !hasTime && in.IntervalDistance == nil {
			add("/interval_months", "required", "Zeit- oder Distanzintervall angeben (I-MA-1).")
		}
		if in.ScheduleMode == ModeFixedGrid {
			if hasTime && in.AnchorDate == nil {
				add("/anchor_date", "required", "Festes Raster braucht ein Startdatum.")
			}
			if in.IntervalDistance != nil && in.AnchorOdometer == nil {
				add("/anchor_odometer", "required", "Festes Raster braucht einen Startstand.")
			}
		}
	}
	if in.Description != nil && len([]rune(*in.Description)) > 5000 {
		add("/description", "length", "")
	}
	if in.Active == nil {
		t := true
		in.Active = &t
	}
	if len(errs) > 0 {
		return out, problem.Validation(errs...)
	}
	return out, nil
}

// CreateItem legt eine Definition an.
func (s *Service) CreateItem(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, id *uuid.UUID, in ItemInput) (ItemView, bool, error) {
	var out ItemView
	created := true
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := identity.Authorize(ctx, tx, actor, vehicleID, identity.RoleEditor); err != nil {
			return err
		}
		meta, err := vehicles.LoadMeta(ctx, tx, vehicleID, false)
		if err != nil {
			return err
		}
		p, err := in.validate(meta)
		if err != nil {
			return err
		}
		q := store.New(tx)
		iid := kernel.NewID()
		if id != nil {
			iid = *id
			if ex, err := q.GetItem(ctx, pg.U(iid)); err == nil {
				if pg.ID(ex.VehicleID) == vehicleID && ex.Title == in.Title && ex.ScheduleMode == in.ScheduleMode {
					out, err = s.view(ctx, tx, ex)
					created = false
					return err
				}
				return problem.Conflict("Eine Wartungsdefinition mit dieser ID existiert mit anderem Inhalt.")
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		row, err := q.InsertItem(ctx, store.InsertItemParams{ID: pg.U(iid), VehicleID: pg.U(vehicleID), Title: in.Title, Description: pg.T(in.Description),
			Category: in.Category, ManufacturerRecommended: in.ManufacturerRecommended, SourceDocumentID: pg.UP(in.SourceDocumentID),
			SourcePage: pg.I4(in.SourcePage), ScheduleMode: in.ScheduleMode, IntervalMonths: pg.I4(in.IntervalMonths), IntervalDays: pg.I4(in.IntervalDays),
			IntervalDistance: pg.I8(p.intervalDistance), AnchorDate: pg.DP(p.anchorDate), AnchorTotal: pg.I8(p.anchorTotal), DueDateOnce: pg.DP(p.dueDateOnce),
			DueTotalOnce: pg.I8(p.dueTotalOnce), UpcomingDays: pg.I4(in.Thresholds.UpcomingDays), DueDays: pg.I4(in.Thresholds.DueDays),
			UpcomingDistance: pg.I8(p.upcomingDistance), DueDistance: pg.I8(p.dueDistance), DistanceUnit: p.unit, Active: *in.Active, Note: in.Note,
			Origin: kernel.OriginOf(actor), CreatedBy: pg.U(actor.AccountID)})
		if err != nil {
			return err
		}
		if out, err = s.view(ctx, tx, row); err != nil {
			return err
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "maintenance.item_defined", VehicleID: &vehicleID, ObjectType: "maintenance_item", ObjectID: iid})
	})
	return out, created, err
}

func loadItem(ctx context.Context, db store.DBTX, actor kernel.Actor, vehicleID, id uuid.UUID, need string) (store.MaintenanceItem, error) {
	r, err := store.New(db).GetItem(ctx, pg.U(id))
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (pg.ID(r.VehicleID) != vehicleID || r.DeletedAt.Valid)) {
		return r, problem.NotFound()
	}
	if err != nil {
		return r, err
	}
	_, err = identity.Authorize(ctx, db, actor, pg.ID(r.VehicleID), need)
	return r, err
}

// GetItem liest eine Definition mit aktueller Fälligkeit.
func (s *Service) GetItem(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID) (ItemView, error) {
	r, err := loadItem(ctx, s.pool, actor, vehicleID, id, identity.RoleViewer)
	if err != nil {
		return ItemView{}, err
	}
	return s.view(ctx, s.pool, r)
}

// UpdateItem ändert eine Definition (Merge Patch, If-Match).
func (s *Service) UpdateItem(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID, ifMatch string, patch []byte) (ItemView, error) {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return ItemView{}, err
	}
	var out ItemView
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := loadItem(ctx, tx, actor, vehicleID, id, identity.RoleEditor)
		if err != nil {
			return err
		}
		if int(cur.Version) != version {
			v, _ := s.view(ctx, tx, cur)
			return problem.PreconditionFailed(v)
		}
		var in ItemInput
		if err := mergepatch.Apply(itemInput(cur), patch, &in); err != nil {
			return problem.BadRequest("Ungültiger Merge Patch.")
		}
		if mergepatch.Has(patch, "interval_days") && in.IntervalDays != nil && !mergepatch.Has(patch, "interval_months") {
			in.IntervalMonths = nil
		}
		if mergepatch.Has(patch, "interval_months") && in.IntervalMonths != nil && !mergepatch.Has(patch, "interval_days") {
			in.IntervalDays = nil
		}
		meta, err := vehicles.LoadMeta(ctx, tx, vehicleID, false)
		if err != nil {
			return err
		}
		p, err := in.validate(meta)
		if err != nil {
			return err
		}
		row, err := store.New(tx).UpdateItem(ctx, store.UpdateItemParams{ID: cur.ID, Title: in.Title, Description: pg.T(in.Description),
			Category: in.Category, ManufacturerRecommended: in.ManufacturerRecommended, SourceDocumentID: pg.UP(in.SourceDocumentID),
			SourcePage: pg.I4(in.SourcePage), ScheduleMode: in.ScheduleMode, IntervalMonths: pg.I4(in.IntervalMonths), IntervalDays: pg.I4(in.IntervalDays),
			IntervalDistance: pg.I8(p.intervalDistance), AnchorDate: pg.DP(p.anchorDate), AnchorTotal: pg.I8(p.anchorTotal), DueDateOnce: pg.DP(p.dueDateOnce),
			DueTotalOnce: pg.I8(p.dueTotalOnce), UpcomingDays: pg.I4(in.Thresholds.UpcomingDays), DueDays: pg.I4(in.Thresholds.DueDays),
			UpcomingDistance: pg.I8(p.upcomingDistance), DueDistance: pg.I8(p.dueDistance), DistanceUnit: p.unit, Active: *in.Active, Note: in.Note,
			UpdatedBy: pg.U(actor.AccountID), Version: cur.Version})
		if errors.Is(err, pgx.ErrNoRows) {
			return problem.PreconditionFailed(nil)
		}
		if err != nil {
			return err
		}
		if out, err = s.view(ctx, tx, row); err != nil {
			return err
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "maintenance.item_updated", VehicleID: &vehicleID, ObjectType: "maintenance_item",
			ObjectID: id, Changes: rawJSON(patch)})
	})
	return out, err
}

// DeleteItem löscht eine Definition weich.
func (s *Service) DeleteItem(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID, ifMatch string) error {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := loadItem(ctx, tx, actor, vehicleID, id, identity.RoleEditor)
		if err != nil {
			return err
		}
		if int(cur.Version) != version {
			return problem.PreconditionFailed(nil)
		}
		if n, err := store.New(tx).SoftDeleteItem(ctx, store.SoftDeleteItemParams{ID: cur.ID, UpdatedBy: pg.U(actor.AccountID), Version: cur.Version}); err != nil || n == 0 {
			if err != nil {
				return err
			}
			return problem.PreconditionFailed(nil)
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "maintenance.item_deleted", VehicleID: &vehicleID, ObjectType: "maintenance_item", ObjectID: id})
	})
}

// ListItems listet die Definitionen mit Fälligkeit.
func (s *Service) ListItems(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, includeDeleted bool, active *bool) ([]ItemView, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return nil, err
	}
	rows, err := store.New(s.pool).ListItems(ctx, store.ListItemsParams{VehicleID: pg.U(vehicleID), IncludeDeleted: includeDeleted, Active: boolP(active)})
	if err != nil {
		return nil, err
	}
	ev, err := s.env(ctx, s.pool, vehicleID)
	if err != nil {
		return nil, err
	}
	out := []ItemView{}
	for _, r := range rows {
		st, err := s.evaluate(ctx, s.pool, ev, r)
		if err != nil {
			return nil, err
		}
		out = append(out, ItemView{EntityMeta: meta(r), ItemInput: itemInput(r), Status: st})
	}
	return out, nil
}

// ---------- Fälligkeit ----------

type evalEnv struct {
	Env
	meta  vehicles.Meta
	segs  odometer.Segments
	valid []odometer.Reading
}

func (s *Service) env(ctx context.Context, db store.DBTX, vehicleID uuid.UUID) (evalEnv, error) {
	meta, err := vehicles.LoadMeta(ctx, db, vehicleID, false)
	if err != nil {
		return evalEnv{}, err
	}
	segs, valid, err := s.odo.Snapshot(ctx, db, vehicleID)
	if err != nil {
		return evalEnv{}, err
	}
	now := s.Now()
	e := evalEnv{Env: Env{Today: kernel.Today(now, meta.OwnerTimeZone)}, meta: meta, segs: segs, valid: valid}
	if cur := odometer.Current(valid, segs); cur.Kind != odometer.KindUnknown {
		t := cur.Total
		e.Current = &t
	}
	e.DailyRate, e.RateKnown = odometer.DailyRate(valid, segs, now)
	return e, nil
}

func (s *Service) evaluate(ctx context.Context, db store.DBTX, ev evalEnv, r store.MaintenanceItem) (DueStatusView, error) {
	rows, err := store.New(db).ListCompletions(ctx, r.ID)
	if err != nil {
		return DueStatusView{}, err
	}
	cs := make([]Completion, 0, len(rows))
	for _, c := range rows {
		comp := Completion{ID: pg.ID(c.ID), On: *pg.DateP(c.CompletedOn), Total: pg.I8P(c.CompletedTotal)}
		if comp.Total == nil { // MA-03: Stand zum Erledigungsdatum 12:00
			if v := odometer.ValueAt(ev.valid, ev.segs, kernel.Noon(comp.On, ev.meta.OwnerTimeZone)); v.Kind != odometer.KindUnknown {
				t := v.Total
				comp.Total = &t
			}
		}
		cs = append(cs, comp)
	}
	SortCompletions(cs)
	engine := ev.meta.UsageMeter == odometer.MeterEngineHours
	v := DueStatusView{ItemID: pg.ID(r.ID), VehicleID: pg.ID(r.VehicleID), Title: r.Title, Level: LevelUnknown}
	if !r.Active {
		return v, nil
	}
	st := Evaluate(domainItem(r, engine), cs, ev.Env)
	v.Level = st.Level
	if st.Reason != "" {
		reason := st.Reason
		v.ReasonTrigger = &reason
	}
	v.DueDate, v.DaysRemaining, v.EstimatedDueDate, v.Estimated = dateStr(st.DueDate), st.DaysRemaining, dateStr(st.EstimatedDate), st.Estimated
	cu := kernel.UnitMeter
	if engine {
		cu = kernel.UnitSecond
	}
	if st.DueTotal != nil {
		v.DueTotal = &odometer.QuantityView{Canonical: *st.DueTotal, CanonicalUnit: cu}
	}
	if st.Remaining != nil {
		f, _ := kernel.FromCanonical(*st.Remaining, r.DistanceUnit)
		v.DistanceRemaining = &kernel.DisplayValue{Value: kernel.Round(f, 1), Unit: r.DistanceUnit}
	}
	return v, nil
}

func (s *Service) view(ctx context.Context, db store.DBTX, r store.MaintenanceItem) (ItemView, error) {
	ev, err := s.env(ctx, db, pg.ID(r.VehicleID))
	if err != nil {
		return ItemView{}, err
	}
	st, err := s.evaluate(ctx, db, ev, r)
	return ItemView{EntityMeta: meta(r), ItemInput: itemInput(r), Status: st}, err
}

// DueStatus bewertet alle aktiven Definitionen eines Fahrzeugs, sortiert nach MA-07.
func (s *Service) DueStatus(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID) ([]DueStatusView, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return nil, err
	}
	return s.dueStatus(ctx, vehicleID)
}

func (s *Service) dueStatus(ctx context.Context, vehicleID uuid.UUID) ([]DueStatusView, error) {
	t := true
	rows, err := store.New(s.pool).ListItems(ctx, store.ListItemsParams{VehicleID: pg.U(vehicleID), Active: boolP(&t)})
	if err != nil {
		return nil, err
	}
	ev, err := s.env(ctx, s.pool, vehicleID)
	if err != nil {
		return nil, err
	}
	var out []DueStatusView
	for _, r := range rows {
		v, err := s.evaluate(ctx, s.pool, ev, r)
		if err != nil {
			return nil, err
		}
		if v.Level != LevelCompleted {
			out = append(out, v)
		}
	}
	SortViews(out)
	if out == nil {
		out = []DueStatusView{}
	}
	return out, nil
}

// SortViews sortiert nach MA-07.
func SortViews(vs []DueStatusView) {
	rs := make([]Ranked, len(vs))
	byID := map[uuid.UUID]DueStatusView{}
	for i, v := range vs {
		var est *time.Time
		if v.EstimatedDueDate != nil {
			d, _ := kernel.ParseDate(*v.EstimatedDueDate)
			est = &d
		}
		rs[i] = Ranked{ItemID: v.ItemID, Status: Status{Level: v.Level, EstimatedDate: est}}
		byID[v.ItemID] = v
	}
	SortNextDue(rs)
	for i, r := range rs {
		vs[i] = byID[r.ItemID]
	}
}

// DueFeed liefert die Fälligkeiten aller Fahrzeuge des Kontos ab einer Mindeststufe.
func (s *Service) DueFeed(ctx context.Context, actor kernel.Actor, minLevel string) ([]DueStatusView, error) {
	members, err := identity.MemberVehicles(ctx, s.pool, actor.AccountID)
	if err != nil {
		return nil, err
	}
	if minLevel == "" {
		minLevel = LevelUpcoming
	}
	out := []DueStatusView{}
	for id := range members {
		if _, err := vehicles.LoadMeta(ctx, s.pool, id, false); err != nil {
			continue // gelöschtes Fahrzeug
		}
		vs, err := s.dueStatus(ctx, id)
		if err != nil {
			return nil, err
		}
		for _, v := range vs {
			if Rank(v.Level) >= Rank(minLevel) {
				out = append(out, v)
			}
		}
	}
	SortViews(out)
	return out, nil
}

// ---------- Erledigungen ----------

func completionView(c store.MaintenanceCompletion, unit string) CompletionView {
	return CompletionView{ID: pg.ID(c.ID), Kind: c.Kind, CompletedOn: kernel.FormatDate(*pg.DateP(c.CompletedOn)),
		CompletedOdometer: qty(pg.I8P(c.CompletedTotal), unit), ServiceEntryID: pg.IDP(c.ServiceEntryID), Reason: pg.TP(c.Reason)}
}

// ListCompletions listet die Erledigungen einer Definition.
func (s *Service) ListCompletions(ctx context.Context, actor kernel.Actor, vehicleID, itemID uuid.UUID) ([]CompletionView, error) {
	it, err := loadItem(ctx, s.pool, actor, vehicleID, itemID, identity.RoleViewer)
	if err != nil {
		return nil, err
	}
	rows, err := store.New(s.pool).ListCompletions(ctx, it.ID)
	if err != nil {
		return nil, err
	}
	out := []CompletionView{}
	for i := len(rows) - 1; i >= 0; i-- { // neueste zuerst
		out = append(out, completionView(rows[i], it.DistanceUnit))
	}
	return out, nil
}

// CompletionInput ist eine manuelle Erledigung oder Auslassung.
type CompletionInput struct {
	Kind        string      `json:"kind"`
	CompletedOn string      `json:"completed_on"`
	Odometer    *vehicles.Q `json:"completed_odometer,omitempty"`
	Reason      *string     `json:"reason,omitempty"`
}

// Complete erledigt manuell (done) oder lässt aus (skipped, Begründung Pflicht).
// Mit Idempotency-Key liefert eine Wiederholung dieselbe Erledigung (ADR-012).
func (s *Service) Complete(ctx context.Context, actor kernel.Actor, vehicleID, itemID uuid.UUID, in CompletionInput, key *string) (CompletionView, bool, error) {
	var out CompletionView
	created := true
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		it, err := loadItem(ctx, tx, actor, vehicleID, itemID, identity.RoleEditor)
		if err != nil {
			return err
		}
		q := store.New(tx)
		if key != nil && *key != "" {
			if ex, err := q.CompletionByKey(ctx, store.CompletionByKeyParams{ItemID: it.ID, IdempotencyKey: pg.T(key)}); err == nil {
				out, created = completionView(ex, it.DistanceUnit), false
				return nil
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		meta, err := vehicles.LoadMeta(ctx, tx, vehicleID, false)
		if err != nil {
			return err
		}
		var errs []problem.FieldError
		if in.Kind != CompletionDone && in.Kind != CompletionSkipped {
			errs = append(errs, problem.FieldError{Pointer: "/kind", Code: "enum"})
		}
		on, err := kernel.ParseDate(in.CompletedOn)
		if err != nil {
			errs = append(errs, problem.FieldError{Pointer: "/completed_on", Code: "date"})
		} else if on.After(kernel.Today(s.Now(), meta.OwnerTimeZone)) {
			errs = append(errs, problem.FieldError{Pointer: "/completed_on", Code: "future", Message: "Das Datum liegt in der Zukunft."})
		}
		var reason *string
		if in.Reason != nil && strings.TrimSpace(*in.Reason) != "" {
			r := strings.TrimSpace(*in.Reason)
			reason = &r
		}
		if in.Kind == CompletionSkipped && reason == nil {
			errs = append(errs, problem.FieldError{Pointer: "/reason", Code: "required", Message: "Begründung für das Auslassen fehlt."})
		}
		var total *int64
		if in.Odometer != nil {
			c, err := kernel.ToCanonical(in.Odometer.Value, in.Odometer.Unit)
			if err != nil || in.Odometer.Value < 0 {
				errs = append(errs, problem.FieldError{Pointer: "/completed_odometer", Code: "unit"})
			} else {
				total = &c.Canonical
			}
		}
		if len(errs) > 0 {
			return problem.Validation(errs...)
		}
		cid := kernel.NewID()
		row, err := q.InsertCompletion(ctx, store.InsertCompletionParams{ID: pg.U(cid), ItemID: it.ID, VehicleID: it.VehicleID, Kind: in.Kind,
			CompletedOn: pg.D(on), CompletedTotal: pg.I8(total), Reason: pg.T(reason), IdempotencyKey: pg.T(key), CreatedBy: pg.U(actor.AccountID)})
		if err != nil {
			return err
		}
		out = completionView(row, it.DistanceUnit)
		return audit.Write(ctx, tx, actor, audit.Event{Action: "maintenance.completion_changed", VehicleID: &vehicleID, ObjectType: "maintenance_completion",
			ObjectID: cid, Changes: map[string]any{"item_id": itemID, "kind": in.Kind, "completed_on": in.CompletedOn}, Reason: deref(reason)})
	})
	return out, created, err
}

// DeleteCompletion nimmt eine manuelle Erledigung zurück; solche aus Serviceeinträgen
// werden über den Eintrag geändert (SH-04).
func (s *Service) DeleteCompletion(ctx context.Context, actor kernel.Actor, vehicleID, itemID, completionID uuid.UUID) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		it, err := loadItem(ctx, tx, actor, vehicleID, itemID, identity.RoleEditor)
		if err != nil {
			return err
		}
		q := store.New(tx)
		c, err := q.GetCompletion(ctx, pg.U(completionID))
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && c.ItemID != it.ID) {
			return problem.NotFound()
		}
		if err != nil {
			return err
		}
		if c.ServiceEntryID.Valid {
			return problem.Conflict("Diese Erledigung stammt aus einem Serviceeintrag und wird dort geändert.")
		}
		if _, err := q.DeleteCompletion(ctx, c.ID); err != nil {
			return err
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "maintenance.completion_changed", VehicleID: &vehicleID, ObjectType: "maintenance_completion",
			ObjectID: completionID, Changes: map[string]any{"deleted": true, "item_id": itemID}})
	})
}

// RecordFromService setzt die Erledigungen eines Serviceeintrags (SH-04) in dessen
// Transaktion. itemIDs == nil behält die bisherige Auswahl und übernimmt nur Datum und Stand.
func RecordFromService(ctx context.Context, tx pgx.Tx, vehicleID, entryID uuid.UUID, itemIDs *[]uuid.UUID, on time.Time, total *int64, createdBy uuid.UUID) error {
	q := store.New(tx)
	var ids []uuid.UUID
	if itemIDs == nil {
		rows, err := q.ServiceCompletions(ctx, pg.U(entryID))
		if err != nil {
			return err
		}
		for _, r := range rows {
			ids = append(ids, pg.ID(r.ItemID))
		}
	} else {
		seen := map[uuid.UUID]bool{}
		for i, id := range *itemIDs {
			if seen[id] {
				continue
			}
			seen[id] = true
			it, err := q.GetItem(ctx, pg.U(id))
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && (pg.ID(it.VehicleID) != vehicleID || it.DeletedAt.Valid)) {
				return problem.Validation(problem.FieldError{Pointer: "/completes_maintenance_item_ids/" + itoa(i), Code: "not_found",
					Message: "Wartungsdefinition gehört nicht zu diesem Fahrzeug."})
			}
			if err != nil {
				return err
			}
			ids = append(ids, id)
		}
	}
	keep := make([]pgUUID, 0, len(ids))
	for _, id := range ids {
		keep = append(keep, pg.U(id))
	}
	if err := q.DeleteServiceCompletions(ctx, store.DeleteServiceCompletionsParams{ServiceEntryID: pg.U(entryID), Keep: keep}); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := q.UpsertServiceCompletion(ctx, store.UpsertServiceCompletionParams{ID: pg.U(kernel.NewID()), ItemID: pg.U(id),
			VehicleID: pg.U(vehicleID), ServiceEntryID: pg.U(entryID), CompletedOn: pg.D(on), CompletedTotal: pg.I8(total), CreatedBy: pg.U(createdBy)}); err != nil {
			return err
		}
	}
	return nil
}

// RemoveServiceCompletions entfernt alle Erledigungen eines gelöschten Serviceeintrags.
func RemoveServiceCompletions(ctx context.Context, tx pgx.Tx, entryID uuid.UUID) error {
	return store.New(tx).DeleteServiceCompletions(ctx, store.DeleteServiceCompletionsParams{ServiceEntryID: pg.U(entryID), Keep: []pgUUID{}})
}

// ServiceItemIDs liefert die durch einen Serviceeintrag erledigten Definitionen.
func ServiceItemIDs(ctx context.Context, db store.DBTX, entryID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := store.New(db).ServiceCompletions(ctx, pg.U(entryID))
	if err != nil {
		return nil, err
	}
	out := []uuid.UUID{}
	for _, r := range rows {
		out = append(out, pg.ID(r.ItemID))
	}
	return out, nil
}
