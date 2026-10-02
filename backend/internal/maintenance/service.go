package maintenance

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/maintenance/store"
	"github.com/sh1su/vector/backend/internal/odometer"
	"github.com/sh1su/vector/backend/internal/platform/audit"
	"github.com/sh1su/vector/backend/internal/platform/db"
	"github.com/sh1su/vector/backend/internal/platform/pg"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

// Service ist der Application Service des Moduls Maintenance.
type Service struct {
	pool *pgxpool.Pool
	odo  *odometer.Service
	Now  func() time.Time
	// UserSettings und InstallSettings liefern die Schwellen-Vorgaben (MA-05).
	UserSettings    func(ctx context.Context, accountID uuid.UUID) (map[string]any, error)
	InstallSettings func(ctx context.Context) (map[string]any, error)
}

func NewService(pool *pgxpool.Pool, odo *odometer.Service) *Service {
	return &Service{pool: pool, odo: odo, Now: time.Now}
}

// ThresholdInput entspricht dem Schema MaintenanceThresholds.
type ThresholdInput struct {
	UpcomingDays     *int        `json:"upcoming_days"`
	DueDays          *int        `json:"due_days"`
	UpcomingDistance *vehicles.Q `json:"upcoming_distance"`
	DueDistance      *vehicles.Q `json:"due_distance"`
}

// Input sind die änderbaren Felder einer Definition (JSON-Namen wie in der API).
type Input struct {
	Title                   string         `json:"title"`
	Description             *string        `json:"description"`
	Category                string         `json:"category"`
	ManufacturerRecommended bool           `json:"manufacturer_recommended"`
	SourceDocumentID        *uuid.UUID     `json:"source_document_id"`
	SourcePage              *int           `json:"source_page"`
	ScheduleMode            string         `json:"schedule_mode"`
	IntervalMonths          *int           `json:"interval_months"`
	IntervalDays            *int           `json:"interval_days"`
	IntervalDistance        *vehicles.Q    `json:"interval_distance"`
	AnchorDate              *string        `json:"anchor_date"`
	AnchorOdometer          *vehicles.Q    `json:"anchor_odometer"`
	DueDateOnce             *string        `json:"due_date_once"`
	DueOdometerOnce         *vehicles.Q    `json:"due_odometer_once"`
	Thresholds              ThresholdInput `json:"thresholds"`
	Active                  *bool          `json:"active"`
	Note                    string         `json:"note"`
}

// StatusView entspricht dem Schema DueStatus.
type StatusView struct {
	ItemID            uuid.UUID              `json:"item_id"`
	VehicleID         uuid.UUID              `json:"-"`
	Title             string                 `json:"title"`
	Level             string                 `json:"level"`
	ReasonTrigger     *string                `json:"reason_trigger"`
	DueDate           *string                `json:"due_date"`
	DaysRemaining     *int                   `json:"days_remaining"`
	DueTotal          *odometer.QuantityView `json:"due_total"`
	DistanceRemaining *kernel.DisplayValue   `json:"distance_remaining"`
	EstimatedDueDate  *string                `json:"estimated_due_date"`
	Estimated         bool                   `json:"estimated"`
	estimated         *time.Time
}

// View entspricht dem Schema MaintenanceItem.
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
	Status StatusView `json:"status"`
}

// CompletionView entspricht dem Schema MaintenanceCompletion.
type CompletionView struct {
	ID                uuid.UUID   `json:"id"`
	Kind              string      `json:"kind"`
	CompletedOn       string      `json:"completed_on"`
	CompletedOdometer *vehicles.Q `json:"completed_odometer"`
	ServiceEntryID    *uuid.UUID  `json:"service_entry_id"`
	Reason            *string     `json:"reason"`
}

type inputs struct {
	IntervalDistance *vehicles.Q `json:"interval_distance,omitempty"`
	AnchorOdometer   *vehicles.Q `json:"anchor_odometer,omitempty"`
	DueOdometerOnce  *vehicles.Q `json:"due_odometer_once,omitempty"`
	UpcomingDistance *vehicles.Q `json:"upcoming_distance,omitempty"`
	DueDistance      *vehicles.Q `json:"due_distance,omitempty"`
}

type record struct {
	in                                      Input
	intervalDist, anchorTotal, dueTotalOnce pgtype.Int8
	upcomingDist, dueDist                   pgtype.Int8
	anchorDate, dueDateOnce                 pgtype.Date
}

var categories = map[string]bool{"service": true, "legal_inspection": true, "tires": true, "fluids": true, "brakes": true, "filters": true, "other": true}

func allowedUnit(meter, unit string) bool {
	if meter == odometer.MeterEngineHours {
		return unit == "h" || unit == "s"
	}
	return unit == "km" || unit == "mi" || unit == "m"
}

func normalize(meta vehicles.Meta, in Input) (record, error) {
	var errs []problem.FieldError
	add := func(p, c, m string) { errs = append(errs, problem.FieldError{Pointer: p, Code: c, Message: m}) }
	in.Title = strings.TrimSpace(in.Title)
	r := record{in: in}
	if n := len([]rune(in.Title)); n < 1 || n > 200 {
		add("/title", "length", "1–200 Zeichen")
	}
	if !categories[in.Category] {
		add("/category", "enum", "")
	}
	if in.SourceDocumentID != nil {
		add("/source_document_id", "not_found", "Das Dokument existiert nicht.")
	}
	dist := func(q *vehicles.Q, ptr string, positive bool) pgtype.Int8 {
		if q == nil {
			return pgtype.Int8{}
		}
		if !allowedUnit(meta.UsageMeter, q.Unit) {
			add(ptr+"/unit", "unit", "Einheit passt nicht zur Zählergröße des Fahrzeugs (I-MA-4).")
			return pgtype.Int8{}
		}
		c, err := kernel.ToCanonical(q.Value, q.Unit)
		if err != nil || c.Canonical < 0 || (positive && c.Canonical == 0) {
			add(ptr+"/value", "range", "")
			return pgtype.Int8{}
		}
		return pgtype.Int8{Int64: c.Canonical, Valid: true}
	}
	date := func(s *string, ptr string) pgtype.Date {
		if s == nil || *s == "" {
			return pgtype.Date{}
		}
		t, err := time.Parse("2006-01-02", *s)
		if err != nil {
			add(ptr, "date", "")
			return pgtype.Date{}
		}
		return pgtype.Date{Time: t, Valid: true}
	}
	r.intervalDist = dist(in.IntervalDistance, "/interval_distance", true)
	r.anchorTotal = dist(in.AnchorOdometer, "/anchor_odometer", false)
	r.dueTotalOnce = dist(in.DueOdometerOnce, "/due_odometer_once", false)
	r.upcomingDist = dist(in.Thresholds.UpcomingDistance, "/thresholds/upcoming_distance", false)
	r.dueDist = dist(in.Thresholds.DueDistance, "/thresholds/due_distance", false)
	r.anchorDate = date(in.AnchorDate, "/anchor_date")
	r.dueDateOnce = date(in.DueDateOnce, "/due_date_once")
	if in.IntervalMonths != nil && *in.IntervalMonths <= 0 {
		add("/interval_months", "range", "")
	}
	if in.IntervalDays != nil && *in.IntervalDays <= 0 {
		add("/interval_days", "range", "")
	}
	if in.IntervalMonths != nil && in.IntervalDays != nil {
		add("/interval_days", "exclusive", "Höchstens Monate oder Tage angeben.")
	}
	for _, t := range []struct {
		p string
		v *int
	}{{"/thresholds/upcoming_days", in.Thresholds.UpcomingDays}, {"/thresholds/due_days", in.Thresholds.DueDays}} {
		if t.v != nil && *t.v < 0 {
			add(t.p, "range", "")
		}
	}
	if in.SourcePage != nil && *in.SourcePage < 1 {
		add("/source_page", "range", "")
	}
	switch in.ScheduleMode {
	case ModeOnce:
		if !r.dueDateOnce.Valid && !r.dueTotalOnce.Valid {
			add("/due_date_once", "required", "Einmalige Aufgaben brauchen ein Fälligkeitsdatum oder einen Fälligkeitsstand (I-MA-1).")
		}
		if in.IntervalMonths != nil || in.IntervalDays != nil || in.IntervalDistance != nil {
			add("/schedule_mode", "intervals_not_allowed", "Einmalige Aufgaben haben kein Intervall.")
		}
	case ModeFromLast, ModeGrid:
		if in.IntervalMonths == nil && in.IntervalDays == nil && in.IntervalDistance == nil {
			add("/interval_months", "required", "Mindestens ein Zeit- oder Distanzintervall angeben (I-MA-1).")
		}
		if in.ScheduleMode == ModeGrid && in.IntervalDistance != nil && in.AnchorOdometer == nil {
			add("/anchor_odometer", "required", "Ein festes Raster nach Distanz braucht einen Startstand.")
		}
		if in.ScheduleMode == ModeGrid && (in.IntervalMonths != nil || in.IntervalDays != nil) && in.AnchorDate == nil {
			add("/anchor_date", "required", "Ein festes Raster nach Zeit braucht ein Startdatum.")
		}
	default:
		add("/schedule_mode", "enum", "")
	}
	if len(errs) > 0 {
		return r, problem.Validation(errs...)
	}
	return r, nil
}

func (r record) inputsJSON() []byte {
	b, _ := json.Marshal(inputs{IntervalDistance: r.in.IntervalDistance, AnchorOdometer: r.in.AnchorOdometer, DueOdometerOnce: r.in.DueOdometerOnce,
		UpcomingDistance: r.in.Thresholds.UpcomingDistance, DueDistance: r.in.Thresholds.DueDistance})
	return b
}

func i4(p *int) pgtype.Int4 {
	if p == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*p), Valid: true}
}

func i4p(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Int32)
	return &i
}

func datePtr(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format("2006-01-02")
	return &s
}

func inputOf(r store.MaintenanceItem) Input {
	var ins inputs
	_ = json.Unmarshal(r.Inputs, &ins)
	active := r.Active
	return Input{Title: r.Title, Description: pg.TextPtr(r.Description), Category: r.Category, ManufacturerRecommended: r.ManufacturerRecommended,
		SourceDocumentID: pg.UUIDPtr(r.SourceDocumentID), SourcePage: i4p(r.SourcePage), ScheduleMode: r.ScheduleMode,
		IntervalMonths: i4p(r.IntervalMonths), IntervalDays: i4p(r.IntervalDays), IntervalDistance: ins.IntervalDistance,
		AnchorDate: datePtr(r.AnchorDate), AnchorOdometer: ins.AnchorOdometer, DueDateOnce: datePtr(r.DueDateOnce), DueOdometerOnce: ins.DueOdometerOnce,
		Thresholds: ThresholdInput{UpcomingDays: i4p(r.UpcomingDays), DueDays: i4p(r.DueDays), UpcomingDistance: ins.UpcomingDistance, DueDistance: ins.DueDistance},
		Active:     &active, Note: r.Note}
}

// Create legt eine Definition an (DefineItem); idempotent mit Client-ID.
func (s *Service) Create(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, id *uuid.UUID, in Input) (View, bool, error) {
	var out View
	created := true
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := identity.Authorize(ctx, tx, actor, vehicleID, identity.RoleEditor); err != nil {
			return err
		}
		meta, err := vehicles.LoadMeta(ctx, tx, vehicleID, false)
		if err != nil {
			return err
		}
		q := store.New(tx)
		iid := kernel.NewID()
		if id != nil {
			iid = *id
			ex, err := q.GetItemAny(ctx, pg.U(iid))
			if err == nil {
				if uuid.UUID(ex.VehicleID.Bytes) == vehicleID && !ex.DeletedAt.Valid && ex.Title == strings.TrimSpace(in.Title) && ex.ScheduleMode == in.ScheduleMode {
					created = false
					out, err = s.viewOne(ctx, tx, actor, ex)
					return err
				}
				return problem.Conflict("Eine Wartung mit dieser ID existiert mit anderem Inhalt.")
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		r, err := normalize(meta, in)
		if err != nil {
			return err
		}
		active := in.Active == nil || *in.Active
		row, err := q.InsertItem(ctx, store.InsertItemParams{ID: pg.U(iid), VehicleID: pg.U(vehicleID), Title: r.in.Title, Description: pg.Text(in.Description),
			Category: in.Category, ManufacturerRecommended: in.ManufacturerRecommended, SourceDocumentID: pg.Up(in.SourceDocumentID), SourcePage: i4(in.SourcePage),
			ScheduleMode: in.ScheduleMode, IntervalMonths: i4(in.IntervalMonths), IntervalDays: i4(in.IntervalDays), IntervalDistance: r.intervalDist,
			AnchorDate: r.anchorDate, AnchorTotal: r.anchorTotal, DueDateOnce: r.dueDateOnce, DueTotalOnce: r.dueTotalOnce,
			UpcomingDays: i4(in.Thresholds.UpcomingDays), DueDays: i4(in.Thresholds.DueDays), UpcomingDistance: r.upcomingDist, DueDistance: r.dueDist,
			Inputs: r.inputsJSON(), Active: active, Note: in.Note, Origin: odometer.OriginOf(actor), CreatedBy: pg.U(actor.AccountID)})
		if err != nil {
			return err
		}
		if err := audit.Write(ctx, tx, actor, audit.Event{Action: "maintenance.item_defined", VehicleID: &vehicleID, ObjectType: "maintenance_item",
			ObjectID: iid, Changes: map[string]any{"title": r.in.Title, "mode": in.ScheduleMode}}); err != nil {
			return err
		}
		out, err = s.viewOne(ctx, tx, actor, row)
		return err
	})
	return out, created, err
}

func (s *Service) loadItem(ctx context.Context, q store.DBTX, actor kernel.Actor, vehicleID, itemID uuid.UUID, role string) (store.MaintenanceItem, error) {
	row, err := store.New(q).GetItem(ctx, pg.U(itemID))
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && uuid.UUID(row.VehicleID.Bytes) != vehicleID) {
		return row, problem.NotFound()
	}
	if err != nil {
		return row, err
	}
	_, err = identity.Authorize(ctx, q, actor, uuid.UUID(row.VehicleID.Bytes), role)
	return row, err
}

// Update ändert eine Definition (JSON Merge Patch, If-Match).
func (s *Service) Update(ctx context.Context, actor kernel.Actor, vehicleID, itemID uuid.UUID, ifMatch string, patch []byte) (View, error) {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return View{}, err
	}
	var out View
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		row, err := s.loadItem(ctx, tx, actor, vehicleID, itemID, identity.RoleEditor)
		if err != nil {
			return err
		}
		if int(row.Version) != version {
			cur, _ := s.viewOne(ctx, tx, actor, row)
			return problem.PreconditionFailed(cur)
		}
		meta, err := vehicles.LoadMeta(ctx, tx, vehicleID, false)
		if err != nil {
			return err
		}
		var in Input
		if err := kernel.MergePatch(inputOf(row), patch, &in); err != nil {
			return problem.BadRequest("Ungültiger Merge Patch.")
		}
		r, err := normalize(meta, in)
		if err != nil {
			return err
		}
		active := in.Active == nil || *in.Active
		row, err = store.New(tx).UpdateItem(ctx, store.UpdateItemParams{ID: row.ID, Title: r.in.Title, Description: pg.Text(in.Description),
			Category: in.Category, ManufacturerRecommended: in.ManufacturerRecommended, SourceDocumentID: pg.Up(in.SourceDocumentID), SourcePage: i4(in.SourcePage),
			ScheduleMode: in.ScheduleMode, IntervalMonths: i4(in.IntervalMonths), IntervalDays: i4(in.IntervalDays), IntervalDistance: r.intervalDist,
			AnchorDate: r.anchorDate, AnchorTotal: r.anchorTotal, DueDateOnce: r.dueDateOnce, DueTotalOnce: r.dueTotalOnce,
			UpcomingDays: i4(in.Thresholds.UpcomingDays), DueDays: i4(in.Thresholds.DueDays), UpcomingDistance: r.upcomingDist, DueDistance: r.dueDist,
			Inputs: r.inputsJSON(), Active: active, Note: in.Note, UpdatedBy: pg.U(actor.AccountID), Version: row.Version})
		if errors.Is(err, pgx.ErrNoRows) {
			return problem.PreconditionFailed(nil)
		}
		if err != nil {
			return err
		}
		if err := audit.Write(ctx, tx, actor, audit.Event{Action: "maintenance.item_updated", VehicleID: &vehicleID, ObjectType: "maintenance_item",
			ObjectID: itemID, Changes: map[string]any{"patch": json.RawMessage(patch)}}); err != nil {
			return err
		}
		out, err = s.viewOne(ctx, tx, actor, row)
		return err
	})
	return out, err
}

// Delete löscht eine Definition weich.
func (s *Service) Delete(ctx context.Context, actor kernel.Actor, vehicleID, itemID uuid.UUID, ifMatch string) error {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return err
	}
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		row, err := s.loadItem(ctx, tx, actor, vehicleID, itemID, identity.RoleEditor)
		if err != nil {
			return err
		}
		if int(row.Version) != version {
			return problem.PreconditionFailed(nil)
		}
		n, err := store.New(tx).SoftDeleteItem(ctx, store.SoftDeleteItemParams{ID: row.ID, UpdatedBy: pg.U(actor.AccountID), Version: row.Version})
		if err != nil {
			return err
		}
		if n == 0 {
			return problem.PreconditionFailed(nil)
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "maintenance.item_deleted", VehicleID: &vehicleID, ObjectType: "maintenance_item", ObjectID: itemID})
	})
}

// --- Bewertung ---

// evalEnv bündelt alles, was die Bewertung der Definitionen eines Fahrzeugs braucht.
type evalEnv struct {
	meta        vehicles.Meta
	env         Env
	completions map[uuid.UUID][]Completion
	defaults    Thresholds
	units       kernel.Units
}

func thresholdsFrom(m map[string]any, meter string, base Thresholds) Thresholds {
	t := base
	th, _ := m["maintenance_thresholds"].(map[string]any)
	if th == nil {
		th, _ = m["default_thresholds"].(map[string]any)
	}
	if th == nil {
		return t
	}
	if v, ok := th["upcoming_days"].(float64); ok {
		t.UpcomingDays = int(v)
	}
	if v, ok := th["due_days"].(float64); ok {
		t.DueDays = int(v)
	}
	dist := func(k string, dst *int64) {
		q, ok := th[k].(map[string]any)
		if !ok {
			return
		}
		v, _ := q["value"].(float64)
		u, _ := q["unit"].(string)
		if !allowedUnit(meter, u) {
			return
		}
		if c, err := kernel.ToCanonical(v, u); err == nil {
			*dst = c.Canonical
		}
	}
	dist("upcoming_distance", &t.UpcomingDistance)
	dist("due_distance", &t.DueDistance)
	return t
}

// defaultsFor löst die Vorgaben auf: Installation, dann Nutzer (MA-05).
func (s *Service) defaultsFor(ctx context.Context, accountID uuid.UUID, meter string) Thresholds {
	t := Thresholds{UpcomingDays: 30, DueDays: 7, UpcomingDistance: 1_500_000, DueDistance: 500_000}
	if meter == odometer.MeterEngineHours {
		t.UpcomingDistance, t.DueDistance = 20*3600, 5*3600
	}
	if s.InstallSettings != nil {
		if m, err := s.InstallSettings(ctx); err == nil {
			t = thresholdsFrom(m, meter, t)
		}
	}
	if s.UserSettings != nil {
		if m, err := s.UserSettings(ctx, accountID); err == nil {
			t = thresholdsFrom(m, meter, t)
		}
	}
	return t
}

func (s *Service) prepareEval(ctx context.Context, q store.DBTX, actor kernel.Actor, vehicleID uuid.UUID) (evalEnv, error) {
	var e evalEnv
	meta, err := vehicles.LoadMeta(ctx, q, vehicleID, false)
	if err != nil {
		return e, err
	}
	segs, valid, err := s.odo.State(ctx, q, vehicleID)
	if err != nil {
		return e, err
	}
	now := s.Now()
	e = evalEnv{meta: meta, completions: map[uuid.UUID][]Completion{}, defaults: s.defaultsFor(ctx, actor.AccountID, meta.UsageMeter)}
	if s.UserSettings != nil {
		if m, err := s.UserSettings(ctx, actor.AccountID); err == nil {
			e.units = kernel.UnitsFromSettings(m)
		}
	}
	if e.units.Distance == "" {
		e.units = kernel.UnitsFromSettings(nil)
	}
	e.env = Env{Today: kernel.LocalDate(now, meta.OwnerTimeZone)}
	if cur := odometer.Current(valid, segs); cur.Kind != odometer.KindUnknown {
		t := cur.Total
		e.env.Current = &t
	}
	if rate, ok := odometer.DailyRate(valid, segs, now); ok {
		e.env.DailyRate = rate
	}
	loc, err := time.LoadLocation(meta.OwnerTimeZone)
	if err != nil {
		loc = time.UTC
	}
	e.env.ValueAt = func(day time.Time) *int64 {
		v := odometer.ValueAt(valid, segs, time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, loc))
		if v.Kind == odometer.KindUnknown {
			return nil
		}
		return &v.Total
	}
	rows, err := store.New(q).ListCompletionsForVehicle(ctx, pg.U(vehicleID))
	if err != nil {
		return e, err
	}
	for _, c := range rows {
		id := uuid.UUID(c.ItemID.Bytes)
		e.completions[id] = append(e.completions[id], Completion{ID: uuid.UUID(c.ID.Bytes), On: c.CompletedOn.Time, Total: pg.Int8Ptr(c.CompletedTotal)})
	}
	for k := range e.completions {
		SortCompletions(e.completions[k])
	}
	return e, nil
}

func defOf(r store.MaintenanceItem, defaults Thresholds) Def {
	d := Def{Mode: r.ScheduleMode, Distance: r.IntervalDistance.Int64, AnchorTotal: pg.Int8Ptr(r.AnchorTotal), DueTotalOnce: pg.Int8Ptr(r.DueTotalOnce),
		Created: time.Date(r.CreatedAt.Time.Year(), r.CreatedAt.Time.Month(), r.CreatedAt.Time.Day(), 0, 0, 0, 0, time.UTC), Thresholds: defaults}
	if r.IntervalMonths.Valid {
		d.Months = int(r.IntervalMonths.Int32)
	}
	if r.IntervalDays.Valid {
		d.Days = int(r.IntervalDays.Int32)
	}
	if r.AnchorDate.Valid {
		t := r.AnchorDate.Time
		d.AnchorDate = &t
	}
	if r.DueDateOnce.Valid {
		t := r.DueDateOnce.Time
		d.DueDateOnce = &t
	}
	if r.UpcomingDays.Valid {
		d.Thresholds.UpcomingDays = int(r.UpcomingDays.Int32)
	}
	if r.DueDays.Valid {
		d.Thresholds.DueDays = int(r.DueDays.Int32)
	}
	if r.UpcomingDistance.Valid {
		d.Thresholds.UpcomingDistance = r.UpcomingDistance.Int64
	}
	if r.DueDistance.Valid {
		d.Thresholds.DueDistance = r.DueDistance.Int64
	}
	return d
}

func (e evalEnv) status(r store.MaintenanceItem) StatusView {
	id := uuid.UUID(r.ID.Bytes)
	v := StatusView{ItemID: id, VehicleID: uuid.UUID(r.VehicleID.Bytes), Title: r.Title, Level: LevelUnknown}
	if !r.Active {
		return v
	}
	st := Evaluate(defOf(r, e.defaults), e.completions[id], e.env)
	v.Level, v.Estimated, v.DaysRemaining = st.Level, st.Estimated, st.DaysRemaining
	if st.Reason != "" {
		reason := st.Reason
		v.ReasonTrigger = &reason
	}
	if st.DueDate != nil {
		s := st.DueDate.Format("2006-01-02")
		v.DueDate = &s
	}
	if st.EstimatedDate != nil {
		s := st.EstimatedDate.Format("2006-01-02")
		v.EstimatedDueDate, v.estimated = &s, st.EstimatedDate
	}
	cu, du := kernel.UnitMeter, e.units.Distance
	if e.meta.UsageMeter == odometer.MeterEngineHours {
		cu, du = kernel.UnitSecond, "h"
	}
	if st.DueTotal != nil {
		v.DueTotal = &odometer.QuantityView{Canonical: *st.DueTotal, CanonicalUnit: cu}
	}
	if st.DistanceRemaining != nil {
		v.DistanceRemaining = &kernel.DisplayValue{Value: kernel.Display(*st.DistanceRemaining, du, 0), Unit: du}
	}
	return v
}

func (s *Service) viewOf(e evalEnv, r store.MaintenanceItem) View {
	return View{ID: uuid.UUID(r.ID.Bytes), Version: int(r.Version), VehicleID: uuid.UUID(r.VehicleID.Bytes), CreatedAt: r.CreatedAt.Time,
		CreatedBy: uuid.UUID(r.CreatedBy.Bytes), UpdatedAt: r.UpdatedAt.Time, UpdatedBy: uuid.UUID(r.UpdatedBy.Bytes), RecordedAt: r.RecordedAt.Time,
		Origin: r.Origin, Input: inputOf(r), Status: e.status(r)}
}

func (s *Service) viewOne(ctx context.Context, q store.DBTX, actor kernel.Actor, r store.MaintenanceItem) (View, error) {
	e, err := s.prepareEval(ctx, q, actor, uuid.UUID(r.VehicleID.Bytes))
	if err != nil {
		return View{}, err
	}
	return s.viewOf(e, r), nil
}

// Get liest eine Definition mit Fälligkeit.
func (s *Service) Get(ctx context.Context, actor kernel.Actor, vehicleID, itemID uuid.UUID) (View, error) {
	row, err := s.loadItem(ctx, s.pool, actor, vehicleID, itemID, identity.RoleViewer)
	if err != nil {
		return View{}, err
	}
	return s.viewOne(ctx, s.pool, actor, row)
}

// List liefert alle Definitionen eines Fahrzeugs, nach Dringlichkeit sortiert (MA-07).
func (s *Service) List(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, active *bool, includeDeleted bool) ([]View, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return nil, err
	}
	e, err := s.prepareEval(ctx, s.pool, actor, vehicleID)
	if err != nil {
		return nil, err
	}
	rows, err := store.New(s.pool).ListItems(ctx, store.ListItemsParams{VehicleID: pg.U(vehicleID), IncludeDeleted: includeDeleted})
	if err != nil {
		return nil, err
	}
	out := []View{}
	for _, r := range rows {
		if active != nil && r.Active != *active {
			continue
		}
		out = append(out, s.viewOf(e, r))
	}
	sortViews(out)
	return out, nil
}

func sortViews(v []View) {
	items := make([]Item, len(v))
	idx := map[uuid.UUID]View{}
	for i, x := range v {
		items[i] = Item{ID: x.ID, Status: Status{Level: x.Status.Level, EstimatedDate: x.Status.estimated}}
		idx[x.ID] = x
	}
	SortNextDue(items)
	for i, it := range items {
		v[i] = idx[it.ID]
	}
}

// Status liefert die Fälligkeit aller aktiven Definitionen (DueStatus, MA-07).
func (s *Service) Status(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID) ([]StatusView, error) {
	t := true
	views, err := s.List(ctx, actor, vehicleID, &t, false)
	if err != nil {
		return nil, err
	}
	out := make([]StatusView, 0, len(views))
	for _, v := range views {
		out = append(out, v.Status)
	}
	return out, nil
}

// DueFeed liefert Fälligkeiten über alle Fahrzeuge des Nutzers ab einer Mindeststufe.
func (s *Service) DueFeed(ctx context.Context, actor kernel.Actor, minLevel string) ([]StatusView, error) {
	if minLevel == "" {
		minLevel = LevelUpcoming
	}
	if _, ok := rank[minLevel]; !ok {
		return nil, problem.Validation(problem.FieldError{Pointer: "/min_level", Code: "enum"})
	}
	ids, err := identity.MemberVehicles(ctx, s.pool, actor.AccountID)
	if err != nil {
		return nil, err
	}
	var all []View
	for vid := range ids {
		views, err := s.List(ctx, actor, vid, nil, false)
		if err != nil {
			var pe *problem.Error
			if errors.As(err, &pe) && pe.Status == 404 {
				continue
			}
			return nil, err
		}
		for _, v := range views {
			if v.Input.Active != nil && *v.Input.Active && Rank(v.Status.Level) >= Rank(minLevel) {
				all = append(all, v)
			}
		}
	}
	sortViews(all)
	out := make([]StatusView, 0, len(all))
	for _, v := range all {
		out = append(out, v.Status)
	}
	return out, nil
}

// --- Erledigungen ---

// CompletionInput ist eine manuelle Erledigung (Complete/Skip).
type CompletionInput struct {
	Kind              string      `json:"kind"`
	CompletedOn       string      `json:"completed_on"`
	CompletedOdometer *vehicles.Q `json:"completed_odometer"`
	Reason            *string     `json:"reason"`
}

func completionView(c store.MaintenanceCompletion) CompletionView {
	v := CompletionView{ID: uuid.UUID(c.ID.Bytes), Kind: c.Kind, CompletedOn: c.CompletedOn.Time.Format("2006-01-02"),
		ServiceEntryID: pg.UUIDPtr(c.ServiceEntryID), Reason: pg.TextPtr(c.Reason)}
	if len(c.CompletedInput) > 0 {
		var q vehicles.Q
		if json.Unmarshal(c.CompletedInput, &q) == nil && q.Unit != "" {
			v.CompletedOdometer = &q
		}
	}
	return v
}

// Complete erfasst eine Erledigung (done) oder ein Auslassen (skipped, Begründung Pflicht).
func (s *Service) Complete(ctx context.Context, actor kernel.Actor, vehicleID, itemID uuid.UUID, idemKey string, in CompletionInput) (CompletionView, bool, error) {
	var out CompletionView
	created := true
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		item, err := s.loadItem(ctx, tx, actor, vehicleID, itemID, identity.RoleEditor)
		if err != nil {
			return err
		}
		q := store.New(tx)
		if idemKey != "" {
			if ex, err := q.GetCompletionByKey(ctx, store.GetCompletionByKeyParams{ItemID: item.ID, IdempotencyKey: pgtype.Text{String: idemKey, Valid: true}}); err == nil {
				out, created = completionView(ex), false
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
		if in.Kind != "done" && in.Kind != "skipped" {
			errs = append(errs, problem.FieldError{Pointer: "/kind", Code: "enum"})
		}
		on, perr := time.Parse("2006-01-02", in.CompletedOn)
		if perr != nil {
			errs = append(errs, problem.FieldError{Pointer: "/completed_on", Code: "date"})
		} else if on.After(kernel.LocalDate(s.Now(), meta.OwnerTimeZone)) {
			errs = append(errs, problem.FieldError{Pointer: "/completed_on", Code: "future", Message: "Das Datum liegt in der Zukunft."})
		}
		reason := ""
		if in.Reason != nil {
			reason = strings.TrimSpace(*in.Reason)
		}
		if in.Kind == "skipped" && reason == "" {
			errs = append(errs, problem.FieldError{Pointer: "/reason", Code: "required", Message: "Beim Auslassen ist eine Begründung Pflicht."})
		}
		var total pgtype.Int8
		var input []byte
		if in.CompletedOdometer != nil {
			if !allowedUnit(meta.UsageMeter, in.CompletedOdometer.Unit) {
				errs = append(errs, problem.FieldError{Pointer: "/completed_odometer/unit", Code: "unit"})
			} else if c, err := kernel.ToCanonical(in.CompletedOdometer.Value, in.CompletedOdometer.Unit); err == nil {
				total = pgtype.Int8{Int64: c.Canonical, Valid: true}
				input, _ = json.Marshal(in.CompletedOdometer)
			}
		}
		if len(errs) > 0 {
			return problem.Validation(errs...)
		}
		var key pgtype.Text
		if idemKey != "" {
			key = pgtype.Text{String: idemKey, Valid: true}
		}
		var rs pgtype.Text
		if reason != "" {
			rs = pgtype.Text{String: reason, Valid: true}
		}
		cid := kernel.NewID()
		row, err := q.InsertCompletion(ctx, store.InsertCompletionParams{ID: pg.U(cid), ItemID: item.ID, VehicleID: pg.U(vehicleID), Kind: in.Kind,
			CompletedOn: pgtype.Date{Time: on, Valid: true}, CompletedTotal: total, CompletedInput: input, Reason: rs, IdempotencyKey: key,
			CreatedBy: pg.U(actor.AccountID)})
		if err != nil {
			return err
		}
		out = completionView(row)
		return audit.Write(ctx, tx, actor, audit.Event{Action: "maintenance.item_" + map[string]string{"done": "completed", "skipped": "skipped"}[in.Kind],
			VehicleID: &vehicleID, ObjectType: "maintenance_completion", ObjectID: cid, Changes: map[string]any{"item_id": itemID, "on": in.CompletedOn}, Reason: reason})
	})
	return out, created, err
}

// Completions listet die Erledigungen einer Definition, neueste zuerst.
func (s *Service) Completions(ctx context.Context, actor kernel.Actor, vehicleID, itemID uuid.UUID) ([]CompletionView, error) {
	item, err := s.loadItem(ctx, s.pool, actor, vehicleID, itemID, identity.RoleViewer)
	if err != nil {
		return nil, err
	}
	rows, err := store.New(s.pool).ListCompletions(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	out := []CompletionView{}
	for _, r := range rows {
		out = append(out, completionView(r))
	}
	return out, nil
}

// DeleteCompletion nimmt eine Erledigung zurück; die Fälligkeit ergibt sich neu (M-8).
func (s *Service) DeleteCompletion(ctx context.Context, actor kernel.Actor, vehicleID, itemID, completionID uuid.UUID) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		item, err := s.loadItem(ctx, tx, actor, vehicleID, itemID, identity.RoleEditor)
		if err != nil {
			return err
		}
		q := store.New(tx)
		c, err := q.GetCompletion(ctx, pg.U(completionID))
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && c.ItemID != item.ID) {
			return problem.NotFound()
		}
		if err != nil {
			return err
		}
		if c.ServiceEntryID.Valid {
			return problem.Conflict("Diese Erledigung stammt aus einem Serviceeintrag und wird dort geändert.")
		}
		if _, err := q.SoftDeleteCompletion(ctx, c.ID); err != nil {
			return err
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "maintenance.completion_deleted", VehicleID: &vehicleID, ObjectType: "maintenance_completion", ObjectID: completionID})
	})
}
