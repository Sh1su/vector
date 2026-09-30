package costs

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/sh1su/vector/backend/internal/costs/store"
	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/audit"
	"github.com/sh1su/vector/backend/internal/platform/mergepatch"
	pg "github.com/sh1su/vector/backend/internal/platform/pgconv"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

func jsonRaw(b []byte) json.RawMessage { return json.RawMessage(b) }

// PlanInput sind die änderbaren Felder eines Kostenplans.
type PlanInput struct {
	Category         string       `json:"category"`
	Title            string       `json:"title"`
	Amount           kernel.Money `json:"amount"`
	IntervalMonths   *int         `json:"interval_months,omitempty"`
	IntervalDays     *int         `json:"interval_days,omitempty"`
	FirstDueOn       string       `json:"first_due_on"`
	EndsOn           *string      `json:"ends_on,omitempty"`
	RemindDaysBefore *int         `json:"remind_days_before,omitempty"`
	Active           *bool        `json:"active,omitempty"`
	Note             string       `json:"note"`
}

// PlanView entspricht dem API-Schema CostPlan.
type PlanView struct {
	kernel.EntityMeta
	PlanInput
}

func planView(p store.CostsPlan) PlanView {
	upd, rec, by := p.UpdatedAt.Time, p.RecordedAt.Time, pg.ID(p.UpdatedBy)
	remind, active := int(p.RemindDaysBefore), p.Active
	return PlanView{
		EntityMeta: kernel.EntityMeta{ID: pg.ID(p.ID), Version: int(p.Version), VehicleID: pg.ID(p.VehicleID), CreatedAt: p.CreatedAt.Time,
			CreatedBy: pg.ID(p.CreatedBy), UpdatedAt: &upd, UpdatedBy: &by, RecordedAt: &rec, Origin: p.Origin},
		PlanInput: PlanInput{Category: p.Category, Title: p.Title, Amount: kernel.Money{AmountMinor: p.AmountMinor, Currency: p.Currency},
			IntervalMonths: pg.I4P(p.IntervalMonths), IntervalDays: pg.I4P(p.IntervalDays), FirstDueOn: kernel.FormatDate(*pg.DateP(p.FirstDueOn)),
			EndsOn: datePtrString(pg.DateP(p.EndsOn)), RemindDaysBefore: &remind, Active: &active, Note: p.Note},
	}
}

func planDef(p store.CostsPlan) PlanDef {
	d := PlanDef{FirstDue: *pg.DateP(p.FirstDueOn), EndsOn: pg.DateP(p.EndsOn)}
	if m := pg.I4P(p.IntervalMonths); m != nil {
		d.Months = *m
	}
	if x := pg.I4P(p.IntervalDays); x != nil {
		d.Days = *x
	}
	return d
}

func (in *PlanInput) validate() (time.Time, *time.Time, error) {
	var errs []problem.FieldError
	add := func(p, c, m string) { errs = append(errs, problem.FieldError{Pointer: p, Code: c, Message: m}) }
	in.Title = strings.TrimSpace(in.Title)
	if n := len([]rune(in.Title)); n < 1 || n > 200 {
		add("/title", "length", "1–200 Zeichen")
	}
	if !Categories[in.Category] {
		add("/category", "enum", "")
	}
	if !kernel.ValidCurrency(in.Amount.Currency) {
		add("/amount/currency", "pattern", "ISO 4217")
	}
	// I-CO-1: genau eines der Intervalle, strikt > 0
	switch {
	case (in.IntervalMonths == nil) == (in.IntervalDays == nil):
		add("/interval_months", "one_of", "Genau eines von Monaten oder Tagen angeben (I-CO-1).")
	case in.IntervalMonths != nil && *in.IntervalMonths <= 0:
		add("/interval_months", "range", "Intervall muss größer als 0 sein (I-CO-1).")
	case in.IntervalDays != nil && *in.IntervalDays <= 0:
		add("/interval_days", "range", "Intervall muss größer als 0 sein (I-CO-1).")
	}
	first, err := kernel.ParseDate(in.FirstDueOn)
	if err != nil {
		add("/first_due_on", "date", "")
	}
	var ends *time.Time
	if in.EndsOn != nil {
		e, err := kernel.ParseDate(*in.EndsOn)
		if err != nil {
			add("/ends_on", "date", "")
		} else if e.Before(first) {
			add("/ends_on", "order", "Ende liegt vor der ersten Fälligkeit.")
		} else {
			ends = &e
		}
	}
	if in.RemindDaysBefore == nil {
		d := 30
		in.RemindDaysBefore = &d
	} else if *in.RemindDaysBefore < 0 || *in.RemindDaysBefore > 365 {
		add("/remind_days_before", "range", "0–365")
	}
	if in.Active == nil {
		t := true
		in.Active = &t
	}
	if len(errs) > 0 {
		return first, ends, problem.Validation(errs...)
	}
	return first, ends, nil
}

// CreatePlan legt einen wiederkehrenden Kostenplan an.
func (s *Service) CreatePlan(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, id *uuid.UUID, in PlanInput) (PlanView, bool, error) {
	first, ends, err := in.validate()
	if err != nil {
		return PlanView{}, false, err
	}
	var out PlanView
	created := true
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := identity.Authorize(ctx, tx, actor, vehicleID, identity.RoleEditor); err != nil {
			return err
		}
		q := store.New(tx)
		pid := kernel.NewID()
		if id != nil {
			pid = *id
			if ex, err := q.GetPlan(ctx, pg.U(pid)); err == nil {
				v := planView(ex)
				if v.VehicleID == vehicleID && v.Title == in.Title && v.Amount == in.Amount && v.FirstDueOn == in.FirstDueOn {
					out, created = v, false
					return nil
				}
				return problem.Conflict("Ein Kostenplan mit dieser ID existiert mit anderem Inhalt.")
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		row, err := q.InsertPlan(ctx, store.InsertPlanParams{ID: pg.U(pid), VehicleID: pg.U(vehicleID), Category: in.Category, Title: in.Title,
			AmountMinor: in.Amount.AmountMinor, Currency: in.Amount.Currency, IntervalMonths: pg.I4(in.IntervalMonths), IntervalDays: pg.I4(in.IntervalDays),
			FirstDueOn: pg.D(first), EndsOn: pg.DP(ends), RemindDaysBefore: int32(*in.RemindDaysBefore), Active: *in.Active, Note: in.Note,
			Origin: kernel.OriginOf(actor), CreatedBy: pg.U(actor.AccountID)})
		if err != nil {
			return err
		}
		out = planView(row)
		return audit.Write(ctx, tx, actor, audit.Event{Action: "costs.plan_defined", VehicleID: &vehicleID, ObjectType: "cost_plan", ObjectID: pid})
	})
	return out, created, err
}

func loadPlan(ctx context.Context, db store.DBTX, actor kernel.Actor, vehicleID, id uuid.UUID, need string) (store.CostsPlan, error) {
	p, err := store.New(db).GetPlan(ctx, pg.U(id))
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (pg.ID(p.VehicleID) != vehicleID || p.DeletedAt.Valid)) {
		return p, problem.NotFound()
	}
	if err != nil {
		return p, err
	}
	_, err = identity.Authorize(ctx, db, actor, pg.ID(p.VehicleID), need)
	return p, err
}

// GetPlan liest einen Kostenplan.
func (s *Service) GetPlan(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID) (PlanView, error) {
	p, err := loadPlan(ctx, s.pool, actor, vehicleID, id, identity.RoleViewer)
	return planView(p), err
}

// UpdatePlan ändert einen Plan (Merge Patch, If-Match). Bereits bestätigte Einträge bleiben unverändert.
func (s *Service) UpdatePlan(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID, ifMatch string, patch []byte) (PlanView, error) {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return PlanView{}, err
	}
	var out PlanView
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := loadPlan(ctx, tx, actor, vehicleID, id, identity.RoleEditor)
		if err != nil {
			return err
		}
		v := planView(cur)
		if int(cur.Version) != version {
			return problem.PreconditionFailed(v)
		}
		var in PlanInput
		if err := mergepatch.Apply(v.PlanInput, patch, &in); err != nil {
			return problem.BadRequest("Ungültiger Merge Patch.")
		}
		// Wird eines der Intervalle gesetzt, entfällt das andere.
		if mergepatch.Has(patch, "interval_days") && in.IntervalDays != nil && !mergepatch.Has(patch, "interval_months") {
			in.IntervalMonths = nil
		}
		if mergepatch.Has(patch, "interval_months") && in.IntervalMonths != nil && !mergepatch.Has(patch, "interval_days") {
			in.IntervalDays = nil
		}
		first, ends, err := in.validate()
		if err != nil {
			return err
		}
		row, err := store.New(tx).UpdatePlan(ctx, store.UpdatePlanParams{ID: cur.ID, Category: in.Category, Title: in.Title, AmountMinor: in.Amount.AmountMinor,
			Currency: in.Amount.Currency, IntervalMonths: pg.I4(in.IntervalMonths), IntervalDays: pg.I4(in.IntervalDays), FirstDueOn: pg.D(first),
			EndsOn: pg.DP(ends), RemindDaysBefore: int32(*in.RemindDaysBefore), Active: *in.Active, Note: in.Note, UpdatedBy: pg.U(actor.AccountID),
			Version: cur.Version})
		if errors.Is(err, pgx.ErrNoRows) {
			return problem.PreconditionFailed(nil)
		}
		if err != nil {
			return err
		}
		out = planView(row)
		return audit.Write(ctx, tx, actor, audit.Event{Action: "costs.plan_updated", VehicleID: &vehicleID, ObjectType: "cost_plan", ObjectID: id, Changes: jsonRaw(patch)})
	})
	return out, err
}

// DeletePlan löscht einen Plan weich; bestätigte Kosteneinträge bleiben erhalten.
func (s *Service) DeletePlan(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID, ifMatch string) error {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := loadPlan(ctx, tx, actor, vehicleID, id, identity.RoleEditor)
		if err != nil {
			return err
		}
		if int(cur.Version) != version {
			return problem.PreconditionFailed(planView(cur))
		}
		if n, err := store.New(tx).SoftDeletePlan(ctx, store.SoftDeletePlanParams{ID: cur.ID, UpdatedBy: pg.U(actor.AccountID), Version: cur.Version}); err != nil || n == 0 {
			if err != nil {
				return err
			}
			return problem.PreconditionFailed(nil)
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "costs.plan_deleted", VehicleID: &vehicleID, ObjectType: "cost_plan", ObjectID: id})
	})
}

// ListPlans listet die Pläne eines Fahrzeugs.
func (s *Service) ListPlans(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, includeDeleted bool) ([]PlanView, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return nil, err
	}
	rows, err := store.New(s.pool).ListPlans(ctx, store.ListPlansParams{VehicleID: pg.U(vehicleID), IncludeDeleted: includeDeleted})
	if err != nil {
		return nil, err
	}
	out := []PlanView{}
	for _, r := range rows {
		out = append(out, planView(r))
	}
	return out, nil
}

// OccurrenceView entspricht dem API-Schema CostOccurrence.
type OccurrenceView struct {
	PlanID      uuid.UUID    `json:"plan_id"`
	DueOn       string       `json:"due_on"`
	State       string       `json:"state"`
	CostEntryID *uuid.UUID   `json:"cost_entry_id"`
	Amount      kernel.Money `json:"amount"`
	Title       string       `json:"title"`
}

// maxOpen begrenzt die Liste offener Vorkommen (CO-02).
const maxOpen = 24

// PendingOccurrences liefert anstehende und offene Vorkommen aller aktiven Pläne.
func (s *Service) PendingOccurrences(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID) ([]OccurrenceView, int, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return nil, 0, err
	}
	return s.pending(ctx, s.pool, vehicleID)
}

func (s *Service) pending(ctx context.Context, db store.DBTX, vehicleID uuid.UUID) ([]OccurrenceView, int, error) {
	meta, err := vehicles.LoadMeta(ctx, db, vehicleID, false)
	if err != nil {
		return nil, 0, err
	}
	q := store.New(db)
	plans, err := q.ListPlans(ctx, store.ListPlansParams{VehicleID: pg.U(vehicleID)})
	if err != nil {
		return nil, 0, err
	}
	conf, err := q.ConfirmedOccurrences(ctx, pg.U(vehicleID))
	if err != nil {
		return nil, 0, err
	}
	dis, err := q.ListDismissals(ctx, pg.U(vehicleID))
	if err != nil {
		return nil, 0, err
	}
	confirmed, dismissed := map[uuid.UUID]map[string]bool{}, map[uuid.UUID]map[string]bool{}
	for _, c := range conf {
		pid := pg.ID(c.RecurringPlanID)
		if confirmed[pid] == nil {
			confirmed[pid] = map[string]bool{}
		}
		confirmed[pid][kernel.FormatDate(*pg.DateP(c.PlanOccurrenceOn))] = true
	}
	for _, d := range dis {
		pid := pg.ID(d.PlanID)
		if dismissed[pid] == nil {
			dismissed[pid] = map[string]bool{}
		}
		dismissed[pid][kernel.FormatDate(*pg.DateP(d.DueOn))] = true
	}
	today := kernel.Today(s.Now(), meta.OwnerTimeZone)
	var open, upcoming []OccurrenceView
	for _, p := range plans {
		if !p.Active {
			continue
		}
		for _, o := range Occurrences(planDef(p), int(p.RemindDaysBefore), today, meta.SaleDate, confirmed[pg.ID(p.ID)], dismissed[pg.ID(p.ID)]) {
			v := OccurrenceView{PlanID: pg.ID(p.ID), DueOn: kernel.FormatDate(o.Due), State: o.State, Title: p.Title,
				Amount: kernel.Money{AmountMinor: p.AmountMinor, Currency: p.Currency}}
			switch o.State {
			case "open":
				open = append(open, v)
			case "upcoming":
				upcoming = append(upcoming, v)
			}
		}
	}
	sort.SliceStable(open, func(i, j int) bool { return open[i].DueOn < open[j].DueOn })
	more := 0
	if len(open) > maxOpen {
		more = len(open) - maxOpen
		open = open[more:] // die jüngsten bleiben sichtbar
	}
	out := append(open, upcoming...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].DueOn < out[j].DueOn })
	if out == nil {
		out = []OccurrenceView{}
	}
	return out, more, nil
}

// ConfirmOccurrence erzeugt den Kosteneintrag eines Vorkommens; idempotent je
// (Plan, Datum) (I-CO-2). created=false bei Wiederholung.
func (s *Service) ConfirmOccurrence(ctx context.Context, actor kernel.Actor, vehicleID, planID uuid.UUID, dueOn time.Time,
	amount *kernel.Money, incurredOn *time.Time) (EntryView, bool, error) {
	var out EntryView
	created := true
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		p, err := loadPlan(ctx, tx, actor, vehicleID, planID, identity.RoleEditor)
		if err != nil {
			return err
		}
		meta, err := vehicles.LoadMeta(ctx, tx, vehicleID, true) // serialisiert Bestätigungen
		if err != nil {
			return err
		}
		if !planDef(p).IsOccurrence(dueOn) {
			return problem.NotFound()
		}
		q := store.New(tx)
		if ex, err := q.EntryForOccurrence(ctx, store.EntryForOccurrenceParams{RecurringPlanID: p.ID, PlanOccurrenceOn: pg.D(dueOn)}); err == nil {
			out, created = entryView(ex), false
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if n, err := q.IsDismissed(ctx, store.IsDismissedParams{PlanID: p.ID, DueOn: pg.D(dueOn)}); err != nil || n > 0 {
			if err != nil {
				return err
			}
			return problem.Conflict("Dieses Vorkommen wurde verworfen.")
		}
		m := kernel.Money{AmountMinor: p.AmountMinor, Currency: p.Currency}
		if amount != nil {
			if !kernel.ValidCurrency(amount.Currency) {
				return problem.Validation(problem.FieldError{Pointer: "/amount/currency", Code: "pattern"})
			}
			m = *amount
		}
		inc := dueOn
		if incurredOn != nil {
			inc = *incurredOn
		}
		eid := kernel.NewID()
		row, err := q.InsertEntry(ctx, store.InsertEntryParams{ID: pg.U(eid), VehicleID: pg.U(vehicleID), Category: p.Category, Title: p.Title,
			IncurredOn: pg.D(inc), TimeZone: meta.OwnerTimeZone, AmountMinor: m.AmountMinor, Currency: m.Currency, RecurringPlanID: p.ID,
			PlanOccurrenceOn: pg.D(dueOn), Note: "", Tags: []string{}, Origin: kernel.OriginOf(actor), CreatedBy: pg.U(actor.AccountID)})
		if err != nil {
			return err
		}
		if err := ReplaceLedger(ctx, tx, vehicleID, "costs", eid, entryLedger(row)); err != nil {
			return err
		}
		out = entryView(row)
		return audit.Write(ctx, tx, actor, audit.Event{Action: "costs.occurrence_confirmed", VehicleID: &vehicleID, ObjectType: "cost_entry", ObjectID: eid,
			Changes: map[string]any{"plan_id": planID, "due_on": kernel.FormatDate(dueOn), "amount": m}})
	})
	return out, created, err
}

// DismissOccurrence schließt ein Vorkommen ohne Eintrag ab (idempotent).
func (s *Service) DismissOccurrence(ctx context.Context, actor kernel.Actor, vehicleID, planID uuid.UUID, dueOn time.Time, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" || len([]rune(reason)) > 500 {
		return problem.Validation(problem.FieldError{Pointer: "/reason", Code: "required", Message: "Begründung fehlt."})
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		p, err := loadPlan(ctx, tx, actor, vehicleID, planID, identity.RoleEditor)
		if err != nil {
			return err
		}
		if !planDef(p).IsOccurrence(dueOn) {
			return problem.NotFound()
		}
		q := store.New(tx)
		if _, err := q.EntryForOccurrence(ctx, store.EntryForOccurrenceParams{RecurringPlanID: p.ID, PlanOccurrenceOn: pg.D(dueOn)}); err == nil {
			return problem.Conflict("Dieses Vorkommen ist bereits bestätigt.")
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		n, err := q.InsertDismissal(ctx, store.InsertDismissalParams{PlanID: p.ID, DueOn: pg.D(dueOn), Reason: reason, CreatedBy: pg.U(actor.AccountID)})
		if err != nil || n == 0 {
			return err
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "costs.occurrence_dismissed", VehicleID: &vehicleID, ObjectType: "cost_plan", ObjectID: planID,
			Changes: map[string]any{"due_on": kernel.FormatDate(dueOn)}, Reason: reason})
	})
}
