package costs

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sh1su/vector/backend/internal/costs/store"
	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/odometer"
	"github.com/sh1su/vector/backend/internal/platform/audit"
	"github.com/sh1su/vector/backend/internal/platform/cursor"
	"github.com/sh1su/vector/backend/internal/platform/mergepatch"
	pg "github.com/sh1su/vector/backend/internal/platform/pgconv"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

// Service ist der Application Service des Moduls Costs.
type Service struct {
	pool *pgxpool.Pool
	odo  *odometer.Service
	Now  func() time.Time
}

func NewService(pool *pgxpool.Pool, odo *odometer.Service) *Service {
	return &Service{pool: pool, odo: odo, Now: time.Now}
}

// ---------- Kostenbuch (CO-01) ----------

// ReplaceLedger ersetzt die Kostenbuch-Zeilen einer Quelle in der Transaktion
// der Quelle (I-CO-3). Leere rows entfernen die Quelle.
func ReplaceLedger(ctx context.Context, db store.DBTX, vehicleID uuid.UUID, module string, sourceID uuid.UUID, rows []LedgerRow) error {
	q := store.New(db)
	if err := q.DeleteLedgerSource(ctx, store.DeleteLedgerSourceParams{SourceModule: module, SourceID: pg.U(sourceID)}); err != nil {
		return err
	}
	for _, r := range rows {
		var kind *string
		if r.CostKind != "" {
			k := r.CostKind
			kind = &k
		}
		if err := q.InsertLedger(ctx, store.InsertLedgerParams{ID: pg.U(kernel.NewID()), VehicleID: pg.U(vehicleID), SourceModule: module,
			SourceID: pg.U(sourceID), Category: r.Category, CostKind: pg.T(kind), BookedOn: pg.D(r.BookedOn), AmountMinor: r.Amount,
			Currency: r.Currency, CoversFrom: pg.DP(r.CoversFrom), CoversTo: pg.DP(r.CoversTo)}); err != nil {
			return err
		}
	}
	return nil
}

// ---------- Sonstige Kosten ----------

// EntryInput sind die änderbaren Felder eines Kosteneintrags (JSON wie API).
type EntryInput struct {
	Category   string       `json:"category"`
	Title      string       `json:"title"`
	IncurredOn string       `json:"incurred_on"`
	TimeZone   string       `json:"time_zone,omitempty"`
	CoversFrom *string      `json:"covers_from,omitempty"`
	CoversTo   *string      `json:"covers_to,omitempty"`
	Amount     kernel.Money `json:"amount"`
	Note       string       `json:"note"`
	Tags       []string     `json:"tags"`
}

// EntryView entspricht dem API-Schema CostEntry.
type EntryView struct {
	kernel.EntityMeta
	EntryInput
	RecurringPlanID  *uuid.UUID `json:"recurring_plan_id"`
	PlanOccurrenceOn *string    `json:"plan_occurrence_on"`
}

func datePtrString(d *time.Time) *string {
	if d == nil {
		return nil
	}
	s := kernel.FormatDate(*d)
	return &s
}

func entryView(e store.CostsEntry) EntryView {
	upd, rec, by := e.UpdatedAt.Time, e.RecordedAt.Time, pg.ID(e.UpdatedBy)
	tags := e.Tags
	if tags == nil {
		tags = []string{}
	}
	return EntryView{
		EntityMeta: kernel.EntityMeta{ID: pg.ID(e.ID), Version: int(e.Version), VehicleID: pg.ID(e.VehicleID), CreatedAt: e.CreatedAt.Time,
			CreatedBy: pg.ID(e.CreatedBy), UpdatedAt: &upd, UpdatedBy: &by, RecordedAt: &rec, Origin: e.Origin},
		EntryInput: EntryInput{Category: e.Category, Title: e.Title, IncurredOn: kernel.FormatDate(*pg.DateP(e.IncurredOn)), TimeZone: e.TimeZone,
			CoversFrom: datePtrString(pg.DateP(e.CoversFrom)), CoversTo: datePtrString(pg.DateP(e.CoversTo)),
			Amount: kernel.Money{AmountMinor: e.AmountMinor, Currency: e.Currency}, Note: e.Note, Tags: tags},
		RecurringPlanID: pg.IDP(e.RecurringPlanID), PlanOccurrenceOn: datePtrString(pg.DateP(e.PlanOccurrenceOn)),
	}
}

type parsedEntry struct {
	incurred, from, to *time.Time
}

func (in *EntryInput) validate(meta vehicles.Meta) (parsedEntry, error) {
	var errs []problem.FieldError
	add := func(p, c, m string) { errs = append(errs, problem.FieldError{Pointer: p, Code: c, Message: m}) }
	var out parsedEntry
	in.Title = strings.TrimSpace(in.Title)
	if n := len([]rune(in.Title)); n < 1 || n > 200 {
		add("/title", "length", "1–200 Zeichen")
	}
	if !Categories[in.Category] {
		add("/category", "enum", "")
	}
	if d, err := kernel.ParseDate(in.IncurredOn); err != nil {
		add("/incurred_on", "date", "")
	} else {
		out.incurred = &d
	}
	if in.TimeZone == "" {
		in.TimeZone = meta.OwnerTimeZone
	}
	if _, err := time.LoadLocation(in.TimeZone); err != nil {
		add("/time_zone", "time_zone", "IANA-Zeitzone")
	}
	if !kernel.ValidCurrency(in.Amount.Currency) {
		add("/amount/currency", "pattern", "ISO 4217")
	}
	if (in.CoversFrom == nil) != (in.CoversTo == nil) {
		add("/covers_to", "required", "Leistungszeitraum: Beginn und Ende angeben (I-CO-4).")
	} else if in.CoversFrom != nil {
		f, err1 := kernel.ParseDate(*in.CoversFrom)
		t, err2 := kernel.ParseDate(*in.CoversTo)
		if err1 != nil || err2 != nil {
			add("/covers_from", "date", "")
		} else if t.Before(f) {
			add("/covers_to", "order", "Ende liegt vor dem Beginn (I-CO-4).")
		} else {
			out.from, out.to = &f, &t
		}
	}
	if len(in.Tags) > 20 {
		add("/tags", "max_items", "")
	}
	if len([]rune(in.Note)) > 10000 {
		add("/note", "length", "")
	}
	if in.Tags == nil {
		in.Tags = []string{}
	}
	if len(errs) > 0 {
		return out, problem.Validation(errs...)
	}
	return out, nil
}

func entryLedger(e store.CostsEntry) []LedgerRow {
	return []LedgerRow{{Category: e.Category, BookedOn: *pg.DateP(e.IncurredOn), Amount: e.AmountMinor, Currency: e.Currency,
		CoversFrom: pg.DateP(e.CoversFrom), CoversTo: pg.DateP(e.CoversTo)}}
}

// CreateEntry legt sonstige Kosten an (idempotent mit Client-ID, ADR-006).
func (s *Service) CreateEntry(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, id *uuid.UUID, in EntryInput) (EntryView, bool, error) {
	var out EntryView
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
		eid := kernel.NewID()
		q := store.New(tx)
		if id != nil {
			eid = *id
			if ex, err := q.GetEntry(ctx, pg.U(eid)); err == nil {
				v := entryView(ex)
				if v.VehicleID == vehicleID && v.Title == in.Title && v.IncurredOn == in.IncurredOn && v.Amount == in.Amount && v.Category == in.Category {
					out, created = v, false
					return nil
				}
				return problem.Conflict("Ein Kosteneintrag mit dieser ID existiert mit anderem Inhalt.")
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		row, err := q.InsertEntry(ctx, store.InsertEntryParams{ID: pg.U(eid), VehicleID: pg.U(vehicleID), Category: in.Category, Title: in.Title,
			IncurredOn: pg.D(*p.incurred), TimeZone: in.TimeZone, CoversFrom: pg.DP(p.from), CoversTo: pg.DP(p.to), AmountMinor: in.Amount.AmountMinor,
			Currency: in.Amount.Currency, Note: in.Note, Tags: in.Tags, Origin: kernel.OriginOf(actor), CreatedBy: pg.U(actor.AccountID)})
		if err != nil {
			return err
		}
		if err := ReplaceLedger(ctx, tx, vehicleID, "costs", eid, entryLedger(row)); err != nil {
			return err
		}
		out = entryView(row)
		return audit.Write(ctx, tx, actor, audit.Event{Action: "costs.entry_recorded", VehicleID: &vehicleID, ObjectType: "cost_entry", ObjectID: eid,
			Changes: map[string]any{"amount": in.Amount, "category": in.Category}})
	})
	return out, created, err
}

// loadEntry lädt einen Eintrag und prüft Fahrzeug und Rolle am Objekt (ID-01).
func loadEntry(ctx context.Context, db store.DBTX, actor kernel.Actor, vehicleID, id uuid.UUID, need string) (store.CostsEntry, error) {
	e, err := store.New(db).GetEntry(ctx, pg.U(id))
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (pg.ID(e.VehicleID) != vehicleID || e.DeletedAt.Valid)) {
		return e, problem.NotFound()
	}
	if err != nil {
		return e, err
	}
	_, err = identity.Authorize(ctx, db, actor, pg.ID(e.VehicleID), need)
	return e, err
}

// GetEntry liest einen Kosteneintrag.
func (s *Service) GetEntry(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID) (EntryView, error) {
	e, err := loadEntry(ctx, s.pool, actor, vehicleID, id, identity.RoleViewer)
	return entryView(e), err
}

// UpdateEntry wendet einen Merge Patch an (If-Match).
func (s *Service) UpdateEntry(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID, ifMatch string, patch []byte) (EntryView, error) {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return EntryView{}, err
	}
	var out EntryView
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := loadEntry(ctx, tx, actor, vehicleID, id, identity.RoleEditor)
		if err != nil {
			return err
		}
		curView := entryView(cur)
		if int(cur.Version) != version {
			return problem.PreconditionFailed(curView)
		}
		var in EntryInput
		if err := mergepatch.Apply(curView.EntryInput, patch, &in); err != nil {
			return problem.BadRequest("Ungültiger Merge Patch.")
		}
		meta, err := vehicles.LoadMeta(ctx, tx, vehicleID, false)
		if err != nil {
			return err
		}
		p, err := in.validate(meta)
		if err != nil {
			return err
		}
		row, err := store.New(tx).UpdateEntry(ctx, store.UpdateEntryParams{ID: cur.ID, Category: in.Category, Title: in.Title, IncurredOn: pg.D(*p.incurred),
			TimeZone: in.TimeZone, CoversFrom: pg.DP(p.from), CoversTo: pg.DP(p.to), AmountMinor: in.Amount.AmountMinor, Currency: in.Amount.Currency,
			Note: in.Note, Tags: in.Tags, UpdatedBy: pg.U(actor.AccountID), Version: cur.Version})
		if errors.Is(err, pgx.ErrNoRows) {
			return problem.PreconditionFailed(nil)
		}
		if err != nil {
			return err
		}
		if err := ReplaceLedger(ctx, tx, vehicleID, "costs", id, entryLedger(row)); err != nil {
			return err
		}
		out = entryView(row)
		return audit.Write(ctx, tx, actor, audit.Event{Action: "costs.entry_updated", VehicleID: &vehicleID, ObjectType: "cost_entry", ObjectID: id,
			Changes: jsonRaw(patch)})
	})
	return out, err
}

// DeleteEntry löscht weich; die Kostenbuch-Zeilen verschwinden in derselben Transaktion.
func (s *Service) DeleteEntry(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID, ifMatch string) error {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := loadEntry(ctx, tx, actor, vehicleID, id, identity.RoleEditor)
		if err != nil {
			return err
		}
		if int(cur.Version) != version {
			return problem.PreconditionFailed(entryView(cur))
		}
		if n, err := store.New(tx).SoftDeleteEntry(ctx, store.SoftDeleteEntryParams{ID: cur.ID, UpdatedBy: pg.U(actor.AccountID), Version: cur.Version}); err != nil || n == 0 {
			if err != nil {
				return err
			}
			return problem.PreconditionFailed(nil)
		}
		if err := ReplaceLedger(ctx, tx, vehicleID, "costs", id, nil); err != nil {
			return err
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "costs.entry_deleted", VehicleID: &vehicleID, ObjectType: "cost_entry", ObjectID: id})
	})
}

// EntryFilter filtert die Liste.
type EntryFilter struct {
	From, To       *time.Time
	Category       *string
	IncludeDeleted bool
	Cursor         *string
	Limit          int
}

// ListEntries liefert Kosteneinträge, neueste zuerst.
func (s *Service) ListEntries(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, f EntryFilter) ([]EntryView, *string, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return nil, nil, err
	}
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	p := store.ListEntriesParams{VehicleID: pg.U(vehicleID), IncludeDeleted: f.IncludeDeleted, FromDate: pg.DP(f.From), ToDate: pg.DP(f.To),
		Category: pg.T(f.Category), Lim: int32(f.Limit + 1)}
	if f.Cursor != nil {
		parts, ok := cursor.Decode(*f.Cursor, 2)
		d, err1 := kernel.ParseDate(parts0(parts))
		cid, err2 := uuid.Parse(parts1(parts))
		if !ok || err1 != nil || err2 != nil {
			return nil, nil, problem.Validation(problem.FieldError{Pointer: "/cursor", Code: "invalid"})
		}
		p.BeforeDate, p.BeforeID = pg.D(d), pg.U(cid)
	}
	rows, err := store.New(s.pool).ListEntries(ctx, p)
	if err != nil {
		return nil, nil, err
	}
	out := []EntryView{}
	var next *string
	for i, r := range rows {
		if i == f.Limit {
			c := cursor.Encode(kernel.FormatDate(*pg.DateP(rows[i-1].IncurredOn)), pg.ID(rows[i-1].ID).String())
			next = &c
			break
		}
		out = append(out, entryView(r))
	}
	return out, next, nil
}

func parts0(p []string) string {
	if len(p) > 0 {
		return p[0]
	}
	return ""
}

func parts1(p []string) string {
	if len(p) > 1 {
		return p[1]
	}
	return ""
}
