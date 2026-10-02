// Package trips erfasst Fahrten mit Start/Ende, berechneter Strecke, Kategorien
// und Korrekturfassungen (docs/phase-2/10-domaene-trips.md).
package trips

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/odometer"
	"github.com/sh1su/vector/backend/internal/platform/audit"
	"github.com/sh1su/vector/backend/internal/platform/cursor"
	pg "github.com/sh1su/vector/backend/internal/platform/pgconv"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/trips/store"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

// Status einer Fahrt.
const (
	StatusOpen       = "open"
	StatusClosed     = "closed"
	StatusSuperseded = "superseded"
	StatusCancelled  = "cancelled"
)

// Service ist der Application Service des Moduls Trips.
type Service struct {
	pool *pgxpool.Pool
	odo  *odometer.Service
	Now  func() time.Time
	// Unit liefert die Anzeigeeinheit der Strecke für den Akteur (km, mi, h).
	Unit func(ctx context.Context, actor kernel.Actor, meter string) string
}

func NewService(pool *pgxpool.Pool, odo *odometer.Service) *Service {
	return &Service{pool: pool, odo: odo, Now: time.Now, Unit: func(_ context.Context, _ kernel.Actor, meter string) string {
		if meter == odometer.MeterEngineHours {
			return "h"
		}
		return "km"
	}}
}

// Input sind die Felder einer Fahrt (JSON wie API).
type Input struct {
	StartedAt       time.Time   `json:"started_at"`
	EndedAt         *time.Time  `json:"ended_at"`
	TimeZone        string      `json:"time_zone"`
	StartOdometer   vehicles.Q  `json:"start_odometer"`
	EndOdometer     *vehicles.Q `json:"end_odometer"`
	StartLocation   *string     `json:"start_location"`
	EndLocation     *string     `json:"end_location"`
	Purpose         *string     `json:"purpose"`
	CategoryID      uuid.UUID   `json:"category_id"`
	DriverAccountID *uuid.UUID  `json:"driver_account_id"`
	Note            string      `json:"note"`
}

// View entspricht dem API-Schema Trip.
type View struct {
	kernel.EntityMeta
	Input
	RootID       uuid.UUID            `json:"root_id"`
	Status       string               `json:"status"`
	SupersedesID *uuid.UUID           `json:"supersedes_id"`
	ChangeReason *string              `json:"change_reason"`
	Distance     *kernel.DisplayValue `json:"distance"`
	GapBefore    *kernel.DisplayValue `json:"gap_before"`
	distance     *int64               // kanonisch, für Auswertungen
}

// Confirmation sind bestätigte Plausibilitätsbefunde (ADR-010).
type Confirmation struct {
	Codes  []string
	Reason string
}

// ---------- Kategorien ----------

// CategoryView entspricht dem API-Schema TripCategory.
type CategoryView struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	Kind            string    `json:"kind"`
	PurposeRequired bool      `json:"purpose_required"`
	Active          bool      `json:"active"`
	Version         int       `json:"version"`
}

var kindLabel = map[string]string{"private": "Privat", "business": "Geschäftlich", "commute": "Arbeitsweg", "other": "Sonstige"}

func categoryView(c store.TripsCategory) CategoryView {
	return CategoryView{ID: pg.ID(c.ID), Name: c.Name, Kind: c.Kind, PurposeRequired: c.PurposeRequired, Active: c.Active, Version: int(c.Version)}
}

// ensureDefaults legt die Standardkategorien eines Fahrzeugs an (idempotent).
func ensureDefaults(ctx context.Context, db store.DBTX, vehicleID, by uuid.UUID) error {
	q := store.New(db)
	for _, k := range []string{"private", "business", "commute", "other"} {
		key := k
		if err := q.EnsureDefaultCategory(ctx, store.EnsureDefaultCategoryParams{ID: pg.U(kernel.NewID()), VehicleID: pg.U(vehicleID),
			Name: kindLabel[k], Kind: k, PurposeRequired: k == "business", DefaultKey: pg.T(&key), CreatedBy: pg.U(by)}); err != nil {
			return err
		}
	}
	return nil
}

// ListCategories listet die Kategorien (legt Standardkategorien bei Bedarf an).
func (s *Service) ListCategories(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID) ([]CategoryView, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return nil, err
	}
	if _, err := vehicles.LoadMeta(ctx, s.pool, vehicleID, false); err != nil {
		return nil, err
	}
	if err := ensureDefaults(ctx, s.pool, vehicleID, actor.AccountID); err != nil {
		return nil, err
	}
	rows, err := store.New(s.pool).ListCategories(ctx, pg.U(vehicleID))
	if err != nil {
		return nil, err
	}
	out := []CategoryView{}
	for _, r := range rows {
		out = append(out, categoryView(r))
	}
	return out, nil
}

// CategoryInput sind die Felder einer Kategorie.
type CategoryInput struct {
	Name            string `json:"name"`
	Kind            string `json:"kind"`
	PurposeRequired bool   `json:"purpose_required"`
	Active          *bool  `json:"active,omitempty"`
}

func (in *CategoryInput) validate() error {
	in.Name = strings.TrimSpace(in.Name)
	var errs []problem.FieldError
	if n := len([]rune(in.Name)); n < 1 || n > 60 {
		errs = append(errs, problem.FieldError{Pointer: "/name", Code: "length", Message: "1–60 Zeichen"})
	}
	if kindLabel[in.Kind] == "" {
		errs = append(errs, problem.FieldError{Pointer: "/kind", Code: "enum"})
	}
	if in.Active == nil {
		t := true
		in.Active = &t
	}
	if len(errs) > 0 {
		return problem.Validation(errs...)
	}
	return nil
}

// CreateCategory legt eine Kategorie an.
func (s *Service) CreateCategory(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, in CategoryInput) (CategoryView, error) {
	if err := in.validate(); err != nil {
		return CategoryView{}, err
	}
	var out CategoryView
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := identity.Authorize(ctx, tx, actor, vehicleID, identity.RoleEditor); err != nil {
			return err
		}
		id := kernel.NewID()
		row, err := store.New(tx).InsertCategory(ctx, store.InsertCategoryParams{ID: pg.U(id), VehicleID: pg.U(vehicleID), Name: in.Name, Kind: in.Kind,
			PurposeRequired: in.PurposeRequired, Active: *in.Active, CreatedBy: pg.U(actor.AccountID)})
		if err != nil {
			return err
		}
		out = categoryView(row)
		return audit.Write(ctx, tx, actor, audit.Event{Action: "trip.category_created", VehicleID: &vehicleID, ObjectType: "trip_category", ObjectID: id})
	})
	return out, err
}

// UpdateCategory ändert eine Kategorie (Merge Patch auf die Felder, If-Match).
func (s *Service) UpdateCategory(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID, ifMatch string, patch map[string]any) (CategoryView, error) {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return CategoryView{}, err
	}
	var out CategoryView
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := store.New(tx)
		c, err := q.GetCategory(ctx, pg.U(id))
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && pg.ID(c.VehicleID) != vehicleID) {
			return problem.NotFound()
		}
		if err != nil {
			return err
		}
		if _, err := identity.Authorize(ctx, tx, actor, pg.ID(c.VehicleID), identity.RoleEditor); err != nil {
			return err
		}
		if int(c.Version) != version {
			return problem.PreconditionFailed(categoryView(c))
		}
		active := c.Active
		in := CategoryInput{Name: c.Name, Kind: c.Kind, PurposeRequired: c.PurposeRequired, Active: &active}
		if v, ok := patch["name"].(string); ok {
			in.Name = v
		}
		if v, ok := patch["kind"].(string); ok {
			in.Kind = v
		}
		if v, ok := patch["purpose_required"].(bool); ok {
			in.PurposeRequired = v
		}
		if v, ok := patch["active"].(bool); ok {
			in.Active = &v
		}
		if err := in.validate(); err != nil {
			return err
		}
		row, err := q.UpdateCategory(ctx, store.UpdateCategoryParams{ID: c.ID, Name: in.Name, Kind: in.Kind, PurposeRequired: in.PurposeRequired,
			Active: *in.Active, Version: c.Version})
		if errors.Is(err, pgx.ErrNoRows) {
			return problem.PreconditionFailed(nil)
		}
		if err != nil {
			return err
		}
		out = categoryView(row)
		return audit.Write(ctx, tx, actor, audit.Event{Action: "trip.category_updated", VehicleID: &vehicleID, ObjectType: "trip_category", ObjectID: id, Changes: patch})
	})
	return out, err
}

// ---------- Fahrten ----------

func trimPtr(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

// check validiert Felder, Kategorie, Fahrer und Überlappung (TR-02). self ist die
// root_id der Fahrt selbst (bei Korrektur/Abschluss), die nicht mit sich kollidiert.
func (s *Service) check(ctx context.Context, tx pgx.Tx, actor kernel.Actor, vehicleID uuid.UUID, in *Input, self *uuid.UUID, newCategory bool) error {
	var errs []problem.FieldError
	add := func(p, c, m string) { errs = append(errs, problem.FieldError{Pointer: p, Code: c, Message: m}) }
	if _, err := time.LoadLocation(in.TimeZone); err != nil || in.TimeZone == "" {
		add("/time_zone", "time_zone", "IANA-Zeitzone")
	}
	in.StartLocation, in.EndLocation, in.Purpose = trimPtr(in.StartLocation), trimPtr(in.EndLocation), trimPtr(in.Purpose)
	for p, v := range map[string]*string{"/start_location": in.StartLocation, "/end_location": in.EndLocation} {
		if v != nil && len([]rune(*v)) > 200 {
			add(p, "length", "")
		}
	}
	if in.Purpose != nil && len([]rune(*in.Purpose)) > 500 {
		add("/purpose", "length", "")
	}
	if in.EndedAt != nil && !in.EndedAt.After(in.StartedAt) { // I-TR-2
		add("/ended_at", "order", "Das Ende muss nach dem Beginn liegen (I-TR-2).")
	}
	if (in.EndedAt == nil) != (in.EndOdometer == nil) {
		add("/end_odometer", "required", "Ende und Endstand gehören zusammen.")
	}
	if err := ensureDefaults(ctx, tx, vehicleID, actor.AccountID); err != nil {
		return err
	}
	cat, err := store.New(tx).GetCategory(ctx, pg.U(in.CategoryID))
	switch {
	case errors.Is(err, pgx.ErrNoRows) || (err == nil && pg.ID(cat.VehicleID) != vehicleID):
		add("/category_id", "not_found", "Kategorie gehört nicht zu diesem Fahrzeug.")
	case err != nil:
		return err
	default:
		if newCategory && !cat.Active {
			add("/category_id", "inactive", "Kategorie ist deaktiviert.")
		}
		if cat.PurposeRequired && in.Purpose == nil {
			add("/purpose", "required", "Für die Kategorie „"+cat.Name+"“ ist ein Zweck Pflicht.")
		}
	}
	if in.DriverAccountID == nil {
		id := actor.AccountID
		in.DriverAccountID = &id
	} else if role, err := identity.RoleOf(ctx, tx, vehicleID, *in.DriverAccountID); err != nil {
		return err
	} else if role == "" {
		add("/driver_account_id", "not_member", "Fahrer muss Mitglied des Fahrzeugs sein.")
	}
	if len(errs) > 0 {
		return problem.Validation(errs...)
	}
	// I-TR-1: gültige Fahrten überlappen nicht ([start, ende), laufend = ∞)
	others, err := store.New(tx).ValidTrips(ctx, pg.U(vehicleID))
	if err != nil {
		return err
	}
	far := time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
	end := far
	if in.EndedAt != nil {
		end = *in.EndedAt
	}
	for _, o := range others {
		if self != nil && pg.ID(o.RootID) == *self {
			continue
		}
		oe := far
		if o.EndedAt.Valid {
			oe = o.EndedAt.Time
		}
		if in.StartedAt.Before(oe) && o.StartedAt.Time.Before(end) {
			id := pg.ID(o.ID)
			msg := "Die Fahrt überschneidet sich mit einer anderen Fahrt."
			if !o.EndedAt.Valid {
				msg = "Es läuft noch eine Fahrt; sie reicht bis heute."
			}
			return problem.Plausibility([]problem.Anomaly{{Code: "TR_OVERLAP", Confirmable: false, Message: msg, RelatedID: &id}})
		}
	}
	return nil
}

func odoIn(at time.Time, tz string, q vehicles.Q) odometer.Input {
	return odometer.Input{OccurredAt: at, TimeZone: tz, Precision: kernel.PrecisionExact, Value: q.Value, Unit: q.Unit}
}

// checkTotals prüft I-TR-3 (hart, nicht bestätigbar).
func checkTotals(b *odometer.Batch, in Input) error {
	if in.EndOdometer == nil || in.EndedAt == nil {
		return nil
	}
	st, err := b.Total(odoIn(in.StartedAt, in.TimeZone, in.StartOdometer))
	if err != nil {
		return err
	}
	et, err := b.Total(odoIn(*in.EndedAt, in.TimeZone, *in.EndOdometer))
	if err != nil {
		return err
	}
	if et < st {
		return problem.Validation(problem.FieldError{Pointer: "/end_odometer", Code: "below_start", Message: "Der Endstand liegt unter dem Startstand (I-TR-3)."})
	}
	return nil
}

func (s *Service) insert(ctx context.Context, tx pgx.Tx, actor kernel.Actor, vehicleID, id, root uuid.UUID, in Input, startID uuid.UUID, endID *uuid.UUID,
	status string, supersedes *uuid.UUID, reason *string) (store.TripsTrip, error) {
	return store.New(tx).InsertTrip(ctx, store.InsertTripParams{ID: pg.U(id), RootID: pg.U(root), VehicleID: pg.U(vehicleID), StartedAt: pg.TS(in.StartedAt),
		EndedAt: pg.TSP(in.EndedAt), TimeZone: in.TimeZone, StartReadingID: pg.U(startID), EndReadingID: pg.UP(endID), StartLocation: pg.T(in.StartLocation),
		EndLocation: pg.T(in.EndLocation), Purpose: pg.T(in.Purpose), CategoryID: pg.U(in.CategoryID), DriverAccountID: pg.UP(in.DriverAccountID),
		Note: in.Note, Status: status, SupersedesID: pg.UP(supersedes), ChangeReason: pg.T(reason), Origin: kernel.OriginOf(actor), CreatedBy: pg.U(actor.AccountID)})
}

// Record erfasst eine abgeschlossene Fahrt nachträglich (RecordTrip) oder startet
// eine laufende (StartTrip, ohne Ende).
func (s *Service) Record(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, id *uuid.UUID, in Input, conf Confirmation) (View, bool, error) {
	var out View
	created := true
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := identity.Authorize(ctx, tx, actor, vehicleID, identity.RoleEditor); err != nil {
			return err
		}
		b, err := s.odo.Begin(ctx, tx, vehicleID)
		if err != nil {
			return err
		}
		tid := kernel.NewID()
		if id != nil {
			tid = *id
			if ex, err := store.New(tx).GetTrip(ctx, pg.U(tid)); err == nil {
				if pg.ID(ex.VehicleID) == vehicleID && ex.StartedAt.Time.Equal(in.StartedAt) && ex.CategoryID.Bytes == in.CategoryID {
					out, err = s.one(ctx, tx, actor, ex)
					created = false
					return err
				}
				return problem.Conflict("Eine Fahrt mit dieser ID existiert mit anderem Inhalt.")
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		if err := s.check(ctx, tx, actor, vehicleID, &in, nil, true); err != nil {
			return err
		}
		if err := checkTotals(b, in); err != nil {
			return err
		}
		if err := b.Add(odoIn(in.StartedAt, in.TimeZone, in.StartOdometer), "trip_start", tid, "/start_odometer", "Startstand"); err != nil {
			return err
		}
		status := StatusOpen
		if in.EndedAt != nil {
			status = StatusClosed
			if err := b.Add(odoIn(*in.EndedAt, in.TimeZone, *in.EndOdometer), "trip_end", tid, "/end_odometer", "Endstand"); err != nil {
				return err
			}
		}
		ids, err := b.Commit(ctx, actor, conf.Codes, conf.Reason)
		if err != nil {
			return err
		}
		var endID *uuid.UUID
		if len(ids) > 1 {
			endID = &ids[1]
		}
		row, err := s.insert(ctx, tx, actor, vehicleID, tid, tid, in, ids[0], endID, status, nil, nil)
		if err != nil {
			if isUnique(err) {
				return problem.Plausibility([]problem.Anomaly{{Code: "TR_OVERLAP", Message: "Es läuft bereits eine Fahrt."}})
			}
			return err
		}
		action := "trip.started"
		if status == StatusClosed {
			action = "trip.recorded"
		}
		if out, err = s.one(ctx, tx, actor, row); err != nil {
			return err
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: action, VehicleID: &vehicleID, ObjectType: "trip", ObjectID: tid, Reason: conf.Reason})
	})
	return out, created, err
}

func isUnique(err error) bool { return err != nil && strings.Contains(err.Error(), "SQLSTATE 23505") }

func load(ctx context.Context, db store.DBTX, actor kernel.Actor, vehicleID, id uuid.UUID, need string) (store.TripsTrip, error) {
	t, err := store.New(db).GetTrip(ctx, pg.U(id))
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && pg.ID(t.VehicleID) != vehicleID) {
		return t, problem.NotFound()
	}
	if err != nil {
		return t, err
	}
	_, err = identity.Authorize(ctx, db, actor, pg.ID(t.VehicleID), need)
	return t, err
}

// Finish schließt eine laufende Fahrt ab (FinishTrip).
func (s *Service) Finish(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID, ifMatch string, endedAt time.Time, end vehicles.Q,
	endLocation *string, conf Confirmation) (View, error) {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return View{}, err
	}
	var out View
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		t, err := load(ctx, tx, actor, vehicleID, id, identity.RoleEditor)
		if err != nil {
			return err
		}
		b, err := s.odo.Begin(ctx, tx, vehicleID)
		if err != nil {
			return err
		}
		if t.Status != StatusOpen {
			return problem.Conflict("Die Fahrt läuft nicht mehr.")
		}
		if int(t.Version) != version {
			return problem.PreconditionFailed(nil)
		}
		in, err := s.input(ctx, tx, t)
		if err != nil {
			return err
		}
		in.EndedAt, in.EndOdometer, in.EndLocation = &endedAt, &end, endLocation
		root := pg.ID(t.RootID)
		if err := s.check(ctx, tx, actor, vehicleID, &in, &root, false); err != nil {
			return err
		}
		if err := checkTotals(b, in); err != nil {
			return err
		}
		if err := b.Add(odoIn(endedAt, in.TimeZone, end), "trip_end", root, "/end_odometer", "Endstand"); err != nil {
			return err
		}
		ids, err := b.Commit(ctx, actor, conf.Codes, conf.Reason)
		if err != nil {
			return err
		}
		row, err := store.New(tx).FinishTrip(ctx, store.FinishTripParams{ID: t.ID, EndedAt: pg.TS(endedAt), EndReadingID: pg.U(ids[0]),
			EndLocation: pg.T(in.EndLocation), UpdatedBy: pg.U(actor.AccountID), Version: t.Version})
		if errors.Is(err, pgx.ErrNoRows) {
			return problem.PreconditionFailed(nil)
		}
		if err != nil {
			return err
		}
		if out, err = s.one(ctx, tx, actor, row); err != nil {
			return err
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "trip.finished", VehicleID: &vehicleID, ObjectType: "trip", ObjectID: id, Reason: conf.Reason})
	})
	return out, err
}

// UpdateOpen ändert eine laufende Fahrt (EditOpenTrip); set enthält die Felder aus dem Patch.
func (s *Service) UpdateOpen(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID, ifMatch string, apply func(*Input) error, conf Confirmation) (View, error) {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return View{}, err
	}
	var out View
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		t, err := load(ctx, tx, actor, vehicleID, id, identity.RoleEditor)
		if err != nil {
			return err
		}
		b, err := s.odo.Begin(ctx, tx, vehicleID)
		if err != nil {
			return err
		}
		if t.Status != StatusOpen {
			return problem.Conflict("Abgeschlossene Fahrten werden über eine Korrektur geändert (TR-03).")
		}
		if int(t.Version) != version {
			v, _ := s.one(ctx, tx, actor, t)
			return problem.PreconditionFailed(v)
		}
		in, err := s.input(ctx, tx, t)
		if err != nil {
			return err
		}
		if err := apply(&in); err != nil {
			return err
		}
		root := pg.ID(t.RootID)
		if err := s.check(ctx, tx, actor, vehicleID, &in, &root, pg.ID(t.CategoryID) != in.CategoryID); err != nil {
			return err
		}
		startID := pg.ID(t.StartReadingID)
		if !b.Same(startID, odoIn(in.StartedAt, in.TimeZone, in.StartOdometer)) {
			if err := b.Replace(startID, odoIn(in.StartedAt, in.TimeZone, in.StartOdometer), "/start_odometer", "Startstand"); err != nil {
				return err
			}
		}
		ids, err := b.Commit(ctx, actor, conf.Codes, conf.Reason)
		if err != nil {
			return err
		}
		if len(ids) > 0 {
			startID = ids[0]
		}
		row, err := store.New(tx).UpdateOpenTrip(ctx, store.UpdateOpenTripParams{ID: t.ID, StartedAt: pg.TS(in.StartedAt), StartReadingID: pg.U(startID),
			StartLocation: pg.T(in.StartLocation), Purpose: pg.T(in.Purpose), CategoryID: pg.U(in.CategoryID), DriverAccountID: pg.UP(in.DriverAccountID),
			Note: in.Note, UpdatedBy: pg.U(actor.AccountID), Version: t.Version})
		if errors.Is(err, pgx.ErrNoRows) {
			return problem.PreconditionFailed(nil)
		}
		if err != nil {
			return err
		}
		if out, err = s.one(ctx, tx, actor, row); err != nil {
			return err
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "trip.updated", VehicleID: &vehicleID, ObjectType: "trip", ObjectID: id})
	})
	return out, err
}

// Correct legt eine neue Fassung einer abgeschlossenen Fahrt an (TR-03).
func (s *Service) Correct(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID, ifMatch string, apply func(*Input) error, reason string,
	conf Confirmation) (View, error) {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return View{}, err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return View{}, problem.Validation(problem.FieldError{Pointer: "/reason", Code: "required", Message: "Begründung der Korrektur fehlt."})
	}
	if conf.Reason == "" {
		conf.Reason = reason
	}
	var out View
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		t, err := load(ctx, tx, actor, vehicleID, id, identity.RoleEditor)
		if err != nil {
			return err
		}
		b, err := s.odo.Begin(ctx, tx, vehicleID)
		if err != nil {
			return err
		}
		if t.Status != StatusClosed {
			return problem.Conflict("Nur abgeschlossene, gültige Fahrten werden korrigiert.")
		}
		if int(t.Version) != version {
			v, _ := s.one(ctx, tx, actor, t)
			return problem.PreconditionFailed(v)
		}
		in, err := s.input(ctx, tx, t)
		if err != nil {
			return err
		}
		if err := apply(&in); err != nil {
			return err
		}
		if in.EndedAt == nil || in.EndOdometer == nil {
			return problem.Validation(problem.FieldError{Pointer: "/ended_at", Code: "required", Message: "Eine abgeschlossene Fahrt braucht Ende und Endstand."})
		}
		root := pg.ID(t.RootID)
		if err := s.check(ctx, tx, actor, vehicleID, &in, &root, pg.ID(t.CategoryID) != in.CategoryID); err != nil {
			return err
		}
		if err := checkTotals(b, in); err != nil {
			return err
		}
		startID, endID := pg.ID(t.StartReadingID), pg.ID(t.EndReadingID)
		startIn, endIn := odoIn(in.StartedAt, in.TimeZone, in.StartOdometer), odoIn(*in.EndedAt, in.TimeZone, *in.EndOdometer)
		replaceStart, replaceEnd := !b.Same(startID, startIn), !b.Same(endID, endIn)
		if replaceStart {
			if err := b.Replace(startID, startIn, "/start_odometer", "Startstand"); err != nil {
				return err
			}
		}
		if replaceEnd {
			if err := b.Replace(endID, endIn, "/end_odometer", "Endstand"); err != nil {
				return err
			}
		}
		ids, err := b.Commit(ctx, actor, conf.Codes, conf.Reason)
		if err != nil {
			return err
		}
		if replaceStart {
			startID, ids = ids[0], ids[1:]
		}
		if replaceEnd {
			endID = ids[0]
		}
		q := store.New(tx)
		if _, err := q.SetTripStatus(ctx, store.SetTripStatusParams{ID: t.ID, Status: StatusSuperseded, UpdatedBy: pg.U(actor.AccountID), Version: t.Version}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return problem.PreconditionFailed(nil)
			}
			return err
		}
		nid, old := kernel.NewID(), pg.ID(t.ID)
		row, err := s.insert(ctx, tx, actor, vehicleID, nid, root, in, startID, &endID, StatusClosed, &old, &reason)
		if err != nil {
			return err
		}
		if out, err = s.one(ctx, tx, actor, row); err != nil {
			return err
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "trip.corrected", VehicleID: &vehicleID, ObjectType: "trip", ObjectID: nid,
			Changes: map[string]any{"supersedes": old, "root_id": root}, Reason: reason})
	})
	return out, err
}

// Cancel storniert eine Fahrt; ihre Messpunkte werden gelöscht (TR-03).
func (s *Service) Cancel(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID, ifMatch, reason string) (View, error) {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return View{}, err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return View{}, problem.Validation(problem.FieldError{Pointer: "/reason", Code: "required", Message: "Begründung des Stornos fehlt."})
	}
	var out View
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		t, err := load(ctx, tx, actor, vehicleID, id, identity.RoleEditor)
		if err != nil {
			return err
		}
		b, err := s.odo.Begin(ctx, tx, vehicleID)
		if err != nil {
			return err
		}
		if t.Status != StatusOpen && t.Status != StatusClosed {
			return problem.Conflict("Diese Fassung ist nicht mehr gültig.")
		}
		if int(t.Version) != version {
			return problem.PreconditionFailed(nil)
		}
		for _, rid := range []*uuid.UUID{pg.IDP(t.StartReadingID), pg.IDP(t.EndReadingID)} {
			if rid != nil {
				if err := b.Remove(*rid); err != nil {
					return err
				}
			}
		}
		if _, err := b.Commit(ctx, actor, nil, ""); err != nil {
			return err
		}
		row, err := store.New(tx).SetTripStatus(ctx, store.SetTripStatusParams{ID: t.ID, Status: StatusCancelled, ChangeReason: pg.T(&reason),
			UpdatedBy: pg.U(actor.AccountID), Version: t.Version})
		if err != nil {
			return err
		}
		if out, err = s.one(ctx, tx, actor, row); err != nil {
			return err
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "trip.cancelled", VehicleID: &vehicleID, ObjectType: "trip", ObjectID: id, Reason: reason})
	})
	return out, err
}

// Get liest eine Fahrt (jede Fassung).
func (s *Service) Get(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID) (View, error) {
	t, err := load(ctx, s.pool, actor, vehicleID, id, identity.RoleViewer)
	if err != nil {
		return View{}, err
	}
	return s.one(ctx, s.pool, actor, t)
}

// Filter filtert die Fahrtenliste.
type Filter struct {
	From, To       *time.Time
	IncludeHistory bool
	Cursor         *string
	Limit          int
}

// List liefert Fahrten, neueste zuerst (Standard: nur gültige Fassungen).
func (s *Service) List(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, f Filter) ([]View, *string, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return nil, nil, err
	}
	meta, err := vehicles.LoadMeta(ctx, s.pool, vehicleID, false)
	if err != nil {
		return nil, nil, err
	}
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	p := store.ListTripsParams{VehicleID: pg.U(vehicleID), IncludeHistory: f.IncludeHistory, Lim: int32(f.Limit + 1)}
	if f.From != nil {
		p.FromTs = pg.TS(kernel.StartOfDay(*f.From, meta.OwnerTimeZone))
	}
	if f.To != nil {
		p.ToTs = pg.TS(kernel.StartOfDay(f.To.AddDate(0, 0, 1), meta.OwnerTimeZone))
	}
	if f.Cursor != nil {
		parts, ok := cursor.Decode(*f.Cursor, 2)
		if !ok {
			return nil, nil, problem.Validation(problem.FieldError{Pointer: "/cursor", Code: "invalid"})
		}
		ts, err1 := time.Parse(time.RFC3339Nano, parts[0])
		cid, err2 := uuid.Parse(parts[1])
		if err1 != nil || err2 != nil {
			return nil, nil, problem.Validation(problem.FieldError{Pointer: "/cursor", Code: "invalid"})
		}
		p.BeforeTs, p.BeforeID = pg.TS(ts), pg.U(cid)
	}
	rows, err := store.New(s.pool).ListTrips(ctx, p)
	if err != nil {
		return nil, nil, err
	}
	var next *string
	if len(rows) > f.Limit {
		last := rows[f.Limit-1]
		c := cursor.Encode(last.StartedAt.Time.UTC().Format(time.RFC3339Nano), pg.ID(last.ID).String())
		next = &c
		rows = rows[:f.Limit]
	}
	out, err := s.views(ctx, s.pool, actor, vehicleID, rows)
	return out, next, err
}

// History liefert alle Fassungen der Fahrt (ADR-011, TR-03).
func (s *Service) History(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID) ([]View, error) {
	t, err := load(ctx, s.pool, actor, vehicleID, id, identity.RoleViewer)
	if err != nil {
		return nil, err
	}
	rows, err := store.New(s.pool).TripHistory(ctx, t.RootID)
	if err != nil {
		return nil, err
	}
	return s.views(ctx, s.pool, actor, vehicleID, rows)
}

func (s *Service) input(ctx context.Context, db store.DBTX, t store.TripsTrip) (Input, error) {
	ids := []uuid.UUID{pg.ID(t.StartReadingID)}
	if e := pg.IDP(t.EndReadingID); e != nil {
		ids = append(ids, *e)
	}
	infos, err := s.odo.ReadingInfos(ctx, db, pg.ID(t.VehicleID), ids)
	if err != nil {
		return Input{}, err
	}
	return inputOf(t, infos), nil
}

func inputOf(t store.TripsTrip, infos map[uuid.UUID]odometer.ReadingInfo) Input {
	in := Input{StartedAt: t.StartedAt.Time, EndedAt: pg.TSPtr(t.EndedAt), TimeZone: t.TimeZone, StartLocation: pg.TP(t.StartLocation),
		EndLocation: pg.TP(t.EndLocation), Purpose: pg.TP(t.Purpose), CategoryID: pg.ID(t.CategoryID), DriverAccountID: pg.IDP(t.DriverAccountID), Note: t.Note}
	if ri, ok := infos[pg.ID(t.StartReadingID)]; ok {
		in.StartOdometer = vehicles.Q{Value: ri.InputValue, Unit: ri.InputUnit}
	}
	if e := pg.IDP(t.EndReadingID); e != nil {
		if ri, ok := infos[*e]; ok {
			in.EndOdometer = &vehicles.Q{Value: ri.InputValue, Unit: ri.InputUnit}
		}
	}
	return in
}

func (s *Service) one(ctx context.Context, db store.DBTX, actor kernel.Actor, t store.TripsTrip) (View, error) {
	vs, err := s.views(ctx, db, actor, pg.ID(t.VehicleID), []store.TripsTrip{t})
	if err != nil {
		return View{}, err
	}
	return vs[0], nil
}

// views baut Lesedarstellungen mit Strecke (TR-01) und Lücke zur vorigen Fahrt (TR-04).
func (s *Service) views(ctx context.Context, db store.DBTX, actor kernel.Actor, vehicleID uuid.UUID, rows []store.TripsTrip) ([]View, error) {
	out := []View{}
	if len(rows) == 0 {
		return out, nil
	}
	meta, err := vehicles.LoadMeta(ctx, db, vehicleID, false)
	if err != nil {
		return nil, err
	}
	valid, err := store.New(db).ValidTrips(ctx, pg.U(vehicleID))
	if err != nil {
		return nil, err
	}
	var ids []uuid.UUID
	for _, t := range append(append([]store.TripsTrip{}, rows...), valid...) {
		ids = append(ids, pg.ID(t.StartReadingID))
		if e := pg.IDP(t.EndReadingID); e != nil {
			ids = append(ids, *e)
		}
	}
	infos, err := s.odo.ReadingInfos(ctx, db, vehicleID, ids)
	if err != nil {
		return nil, err
	}
	unit := s.Unit(ctx, actor, meta.UsageMeter)
	disp := func(v int64) *kernel.DisplayValue {
		f, _ := kernel.FromCanonical(v, unit)
		return &kernel.DisplayValue{Value: kernel.Round(f, 1), Unit: unit}
	}
	// Lücken zwischen aufeinanderfolgenden gültigen Fahrten (TR-04)
	gaps := map[uuid.UUID]int64{}
	for i := 1; i < len(valid); i++ {
		prev, cur := valid[i-1], valid[i]
		pe := pg.IDP(prev.EndReadingID)
		if pe == nil {
			continue
		}
		a, okA := infos[*pe]
		b, okB := infos[pg.ID(cur.StartReadingID)]
		if okA && okB && b.Total > a.Total {
			gaps[pg.ID(cur.ID)] = b.Total - a.Total
		}
	}
	for _, t := range rows {
		upd, rec, by := t.UpdatedAt.Time, t.RecordedAt.Time, pg.ID(t.UpdatedBy)
		v := View{
			EntityMeta: kernel.EntityMeta{ID: pg.ID(t.ID), Version: int(t.Version), VehicleID: vehicleID, CreatedAt: t.CreatedAt.Time,
				CreatedBy: pg.ID(t.CreatedBy), UpdatedAt: &upd, UpdatedBy: &by, RecordedAt: &rec, Origin: t.Origin},
			Input: inputOf(t, infos), RootID: pg.ID(t.RootID), Status: t.Status, SupersedesID: pg.IDP(t.SupersedesID), ChangeReason: pg.TP(t.ChangeReason),
		}
		if e := pg.IDP(t.EndReadingID); e != nil {
			a, okA := infos[pg.ID(t.StartReadingID)]
			b, okB := infos[*e]
			if okA && okB {
				d := b.Total - a.Total
				v.Distance, v.distance = disp(d), &d
			}
		}
		if g, ok := gaps[pg.ID(t.ID)]; ok && (t.Status == StatusOpen || t.Status == StatusClosed) {
			v.GapBefore = disp(g)
		}
		out = append(out, v)
	}
	return out, nil
}

// ---------- Auswertung (TR-05) ----------

// Share entspricht dem API-Schema DistanceShare.
type Share struct {
	Key      string              `json:"key"`
	Label    string              `json:"label"`
	Distance kernel.DisplayValue `json:"distance"`
	Trips    int                 `json:"trips"`
	SharePct float64             `json:"share_pct"`
}

// Report entspricht dem API-Schema TripReport.
type Report struct {
	From       string               `json:"from"`
	To         string               `json:"to"`
	Total      kernel.DisplayValue  `json:"total"`
	ByKind     []Share              `json:"by_kind"`
	ByCategory []Share              `json:"by_category"`
	ByDriver   []Share              `json:"by_driver"`
	Unassigned *kernel.DisplayValue `json:"unassigned"`
	Hints      []string             `json:"hints"`
}

type acc struct {
	label string
	dist  int64
	n     int
}

// Report wertet abgeschlossene Fahrten aus, deren Beginn im Zeitraum liegt.
func (s *Service) Report(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, from, to *time.Time) (Report, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return Report{}, err
	}
	meta, err := vehicles.LoadMeta(ctx, s.pool, vehicleID, false)
	if err != nil {
		return Report{}, err
	}
	t := kernel.Today(s.Now(), meta.OwnerTimeZone)
	if to != nil {
		t = *to
	}
	f := time.Date(t.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	if from != nil {
		f = *from
	}
	if t.Before(f) {
		return Report{}, problem.Validation(problem.FieldError{Pointer: "/to", Code: "order"})
	}
	if err := ensureDefaults(ctx, s.pool, vehicleID, actor.AccountID); err != nil {
		return Report{}, err
	}
	q := store.New(s.pool)
	valid, err := q.ValidTrips(ctx, pg.U(vehicleID))
	if err != nil {
		return Report{}, err
	}
	cats, err := q.ListCategories(ctx, pg.U(vehicleID))
	if err != nil {
		return Report{}, err
	}
	catByID := map[uuid.UUID]store.TripsCategory{}
	for _, c := range cats {
		catByID[pg.ID(c.ID)] = c
	}
	views, err := s.views(ctx, s.pool, actor, vehicleID, valid)
	if err != nil {
		return Report{}, err
	}
	unit := s.Unit(ctx, actor, meta.UsageMeter)
	byKind, byCat, byDriver := map[string]*acc{}, map[string]*acc{}, map[string]*acc{}
	bump := func(m map[string]*acc, key, label string, d int64) {
		if m[key] == nil {
			m[key] = &acc{label: label}
		}
		m[key].dist += d
		m[key].n++
	}
	var total int64
	for _, v := range views {
		day := kernel.LocalDate(v.StartedAt, v.TimeZone)
		if v.Status != StatusClosed || v.distance == nil || day.Before(f) || day.After(t) {
			continue
		}
		d := *v.distance
		total += d
		c := catByID[v.CategoryID]
		bump(byKind, c.Kind, kindLabel[c.Kind], d)
		bump(byCat, v.CategoryID.String(), c.Name, d)
		if v.DriverAccountID != nil {
			bump(byDriver, v.DriverAccountID.String(), identity.DisplayName(ctx, s.pool, *v.DriverAccountID), d)
		}
	}
	disp := func(v int64) kernel.DisplayValue {
		x, _ := kernel.FromCanonical(v, unit)
		return kernel.DisplayValue{Value: kernel.Round(x, 1), Unit: unit}
	}
	shares := func(m map[string]*acc) []Share {
		out := []Share{}
		for k, a := range m {
			pct := 0.0
			if total > 0 {
				pct = kernel.Round(float64(a.dist)*100/float64(total), 1)
			}
			out = append(out, Share{Key: k, Label: a.label, Distance: disp(a.dist), Trips: a.n, SharePct: pct})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Distance.Value > out[j].Distance.Value })
		return out
	}
	rep := Report{From: kernel.FormatDate(f), To: kernel.FormatDate(t), Total: disp(total), ByKind: shares(byKind), ByCategory: shares(byCat),
		ByDriver: shares(byDriver), Hints: []string{}}
	segs, readings, err := s.odo.Snapshot(ctx, s.pool, vehicleID)
	if err != nil {
		return Report{}, err
	}
	tz := meta.OwnerTimeZone
	if d := odometer.DistanceBetween(readings, segs, kernel.StartOfDay(f, tz), kernel.StartOfDay(t.AddDate(0, 0, 1), tz)); d.Known {
		u := d.Meters - total
		if u < 0 {
			u = 0
		}
		dv := disp(u)
		rep.Unassigned = &dv
		rep.Hints = append(rep.Hints, "„Nicht zugeordnet“ ist die Strecke laut Kilometerstand abzüglich der erfassten Fahrten; Fahrten über die Zeitraumgrenze können den Wert leicht verschieben.")
	}
	return rep, nil
}
