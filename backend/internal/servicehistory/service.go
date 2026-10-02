// Package servicehistory dokumentiert durchgeführte Arbeiten mit Kostenpositionen,
// Teilen, Messpunkt und Wartungserledigungen (docs/phase-2/10-domaene-servicehistory.md).
package servicehistory

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

	"github.com/sh1su/vector/backend/internal/costs"
	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/maintenance"
	"github.com/sh1su/vector/backend/internal/odometer"
	"github.com/sh1su/vector/backend/internal/platform/audit"
	"github.com/sh1su/vector/backend/internal/platform/cursor"
	"github.com/sh1su/vector/backend/internal/platform/mergepatch"
	pg "github.com/sh1su/vector/backend/internal/platform/pgconv"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/servicehistory/store"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

// Service ist der Application Service des Moduls ServiceHistory.
type Service struct {
	pool *pgxpool.Pool
	odo  *odometer.Service
	Now  func() time.Time
}

func NewService(pool *pgxpool.Pool, odo *odometer.Service) *Service {
	return &Service{pool: pool, odo: odo, Now: time.Now}
}

var kinds = map[string]bool{"maintenance": true, "inspection": true, "repair": true, "upgrade": true}
var costKinds = map[string]bool{"parts": true, "labor": true, "other": true}

// CostItem entspricht dem API-Schema CostItem.
type CostItem struct {
	ID          *uuid.UUID `json:"id,omitempty"`
	Kind        string     `json:"kind"`
	Label       *string    `json:"label,omitempty"`
	AmountMinor int64      `json:"amount_minor"`
}

// PartLine entspricht dem API-Schema PartLine.
type PartLine struct {
	ID             *uuid.UUID `json:"id,omitempty"`
	Name           string     `json:"name"`
	PartNumber     *string    `json:"part_number,omitempty"`
	Quantity       float64    `json:"quantity"`
	QuantityUnit   *string    `json:"quantity_unit,omitempty"`
	UnitPriceMinor *int64     `json:"unit_price_minor,omitempty"`
	CostItemID     *uuid.UUID `json:"cost_item_id,omitempty"`
}

// Input sind die änderbaren Felder eines Serviceeintrags (JSON wie API).
type Input struct {
	OccurredAt    time.Time    `json:"occurred_at"`
	TimeZone      string       `json:"time_zone"`
	TimePrecision string       `json:"time_precision,omitempty"`
	Kind          string       `json:"kind"`
	Category      *string      `json:"category,omitempty"`
	Title         string       `json:"title"`
	Description   *string      `json:"description,omitempty"`
	Odometer      *vehicles.Q  `json:"odometer,omitempty"`
	Currency      string       `json:"currency"`
	ProviderName  *string      `json:"provider_name,omitempty"`
	InvoiceNumber *string      `json:"invoice_number,omitempty"`
	CostUnknown   bool         `json:"cost_unknown"`
	CostItems     []CostItem   `json:"cost_items"`
	Parts         []PartLine   `json:"parts"`
	Completes     *[]uuid.UUID `json:"completes_maintenance_item_ids,omitempty"`
	Note          string       `json:"note"`
	Tags          []string     `json:"tags"`
}

// Totals entspricht dem API-Schema ServiceTotals (SH-01).
type Totals struct {
	Total kernel.Money `json:"total"`
	Parts kernel.Money `json:"parts"`
	Labor kernel.Money `json:"labor"`
	Other kernel.Money `json:"other"`
}

// View entspricht dem API-Schema ServiceEntry.
type View struct {
	kernel.EntityMeta
	Input
	OdometerReadingID *uuid.UUID             `json:"odometer_reading_id"`
	OdometerTotal     *odometer.QuantityView `json:"odometer_total"`
	Totals            *Totals                `json:"totals"`
}

// Confirmation sind bestätigte Plausibilitätsbefunde (ADR-010).
type Confirmation struct {
	Codes  []string
	Reason string
}

// ComputeTotals bildet die Summe je Kostenart (SH-01).
func ComputeTotals(items []CostItem, currency string) Totals {
	t := Totals{Total: kernel.Money{Currency: currency}, Parts: kernel.Money{Currency: currency}, Labor: kernel.Money{Currency: currency},
		Other: kernel.Money{Currency: currency}}
	for _, it := range items {
		t.Total.AmountMinor += it.AmountMinor
		switch it.Kind {
		case "parts":
			t.Parts.AmountMinor += it.AmountMinor
		case "labor":
			t.Labor.AmountMinor += it.AmountMinor
		default:
			t.Other.AmountMinor += it.AmountMinor
		}
	}
	return t
}

func (in *Input) validate(meta vehicles.Meta) error {
	var errs []problem.FieldError
	add := func(p, c, m string) { errs = append(errs, problem.FieldError{Pointer: p, Code: c, Message: m}) }
	in.Title = strings.TrimSpace(in.Title)
	if n := len([]rune(in.Title)); n < 1 || n > 200 {
		add("/title", "length", "1–200 Zeichen")
	}
	if !kinds[in.Kind] {
		add("/kind", "enum", "")
	}
	if in.Category != nil {
		c := strings.TrimSpace(*in.Category)
		if c == "" {
			in.Category = nil
		} else if len([]rune(c)) > 60 {
			add("/category", "length", "")
		} else {
			in.Category = &c
		}
	}
	if in.Currency == "" {
		in.Currency = meta.DefaultCurrency
	}
	if !kernel.ValidCurrency(in.Currency) {
		add("/currency", "pattern", "ISO 4217")
	}
	if in.TimePrecision == "" {
		in.TimePrecision = kernel.PrecisionExact
	}
	if et, err := kernel.NormalizeEventTime(in.OccurredAt, in.TimeZone, in.TimePrecision); err != nil {
		add("/time_zone", "time", err.Error())
	} else {
		in.OccurredAt = et.At
	}
	// I-SH-1: Positionen oder „Kosten unbekannt“, nicht beides
	if in.CostUnknown && len(in.CostItems) > 0 {
		add("/cost_items", "cost_unknown", "Bei unbekannten Kosten keine Kostenpositionen angeben (I-SH-1).")
	}
	if !in.CostUnknown && len(in.CostItems) == 0 {
		add("/cost_items", "required", "Mindestens eine Kostenposition (auch 0,00) oder „Kosten unbekannt“ angeben (I-SH-1).")
	}
	var sum int64
	for i, c := range in.CostItems {
		p := "/cost_items/" + strconv.Itoa(i)
		if !costKinds[c.Kind] {
			add(p+"/kind", "enum", "")
		}
		if c.AmountMinor < 0 && c.Kind != "other" {
			add(p+"/amount_minor", "range", "Negative Beträge nur als Gutschrift (Sonstiges).")
		}
		if c.Label != nil && len([]rune(*c.Label)) > 200 {
			add(p+"/label", "length", "")
		}
		sum += c.AmountMinor
	}
	if sum < 0 {
		add("/cost_items", "negative_total", "Die Summe des Eintrags darf nicht negativ sein (SH-03).")
	}
	for i, p := range in.Parts {
		ptr := "/parts/" + strconv.Itoa(i)
		if n := len([]rune(strings.TrimSpace(p.Name))); n < 1 || n > 200 {
			add(ptr+"/name", "length", "1–200 Zeichen")
		}
		if p.Quantity < 0 {
			add(ptr+"/quantity", "range", "")
		}
	}
	if in.Odometer == nil && meta.OdometerRequired {
		add("/odometer", "required", "Für dieses Fahrzeug ist der Kilometerstand Pflicht.")
	}
	if len(in.Tags) > 20 {
		add("/tags", "max_items", "")
	}
	if in.Tags == nil {
		in.Tags = []string{}
	}
	if in.CostItems == nil {
		in.CostItems = []CostItem{}
	}
	if in.Parts == nil {
		in.Parts = []PartLine{}
	}
	if len(errs) > 0 {
		return problem.Validation(errs...)
	}
	return nil
}

func (s *Service) checkTime(in Input, meta vehicles.Meta) error {
	cand := odometer.Reading{At: in.OccurredAt, TimeZone: in.TimeZone, Precision: in.TimePrecision}
	if a := odometer.CheckFuture(cand, s.Now()); a != nil {
		a.Message = "Der Zeitpunkt liegt in der Zukunft. Geplante Arbeiten gehören in die Wartung."
		return problem.Plausibility([]problem.Anomaly{*a})
	}
	if meta.SaleDate != nil && kernel.LocalDate(in.OccurredAt, in.TimeZone).After(*meta.SaleDate) {
		return problem.Validation(problem.FieldError{Pointer: "/occurred_at", Code: "after_sale", Message: "Der Zeitpunkt liegt nach dem Verkaufsdatum."})
	}
	return nil
}

func odoInput(in Input) odometer.Input {
	return odometer.Input{OccurredAt: in.OccurredAt, TimeZone: in.TimeZone, Precision: in.TimePrecision, Value: in.Odometer.Value, Unit: in.Odometer.Unit}
}

// writeChildren ersetzt Positionen und Teile (UpdateServiceEntry ersetzt als Ganzes).
func writeChildren(ctx context.Context, q *store.Queries, entryID uuid.UUID, in *Input) error {
	if err := q.DeleteCostItems(ctx, pg.U(entryID)); err != nil {
		return err
	}
	if err := q.DeletePartLines(ctx, pg.U(entryID)); err != nil {
		return err
	}
	for i := range in.CostItems {
		c := &in.CostItems[i]
		if c.ID == nil {
			id := kernel.NewID()
			c.ID = &id
		}
		if err := q.InsertCostItem(ctx, store.InsertCostItemParams{ID: pg.U(*c.ID), EntryID: pg.U(entryID), Position: int32(i), Kind: c.Kind,
			Label: pg.T(c.Label), AmountMinor: c.AmountMinor}); err != nil {
			return err
		}
	}
	for i := range in.Parts {
		p := &in.Parts[i]
		if p.ID == nil {
			id := kernel.NewID()
			p.ID = &id
		}
		if err := q.InsertPartLine(ctx, store.InsertPartLineParams{ID: pg.U(*p.ID), EntryID: pg.U(entryID), Position: int32(i), Name: strings.TrimSpace(p.Name),
			PartNumber: pg.T(p.PartNumber), Quantity: pg.Numeric(strconv.FormatFloat(p.Quantity, 'f', -1, 64)), QuantityUnit: pg.T(p.QuantityUnit),
			UnitPriceMinor: pg.I8(p.UnitPriceMinor), CostItemID: pg.UP(p.CostItemID)}); err != nil {
			return err
		}
	}
	return nil
}

func ledgerRows(in Input) []costs.LedgerRow {
	day := kernel.LocalDate(in.OccurredAt, in.TimeZone)
	var rows []costs.LedgerRow
	for _, c := range in.CostItems {
		rows = append(rows, costs.LedgerRow{Category: in.Kind, CostKind: c.Kind, BookedOn: day, Amount: c.AmountMinor, Currency: in.Currency})
	}
	return rows
}

// sideEffects schreibt Erledigungen (SH-04) und Kostenbuch (CO-01) in der Transaktion des Eintrags.
func (s *Service) sideEffects(ctx context.Context, tx pgx.Tx, actor kernel.Actor, meta vehicles.Meta, entryID uuid.UUID, readingID *uuid.UUID,
	in Input, completes *[]uuid.UUID) error {
	var total *int64
	if readingID != nil {
		infos, err := s.odo.ReadingInfos(ctx, tx, meta.ID, []uuid.UUID{*readingID})
		if err != nil {
			return err
		}
		if ri, ok := infos[*readingID]; ok {
			t := ri.Total
			total = &t
		}
	}
	on := kernel.LocalDate(in.OccurredAt, meta.OwnerTimeZone)
	if err := maintenance.RecordFromService(ctx, tx, meta.ID, entryID, completes, on, total, actor.AccountID); err != nil {
		return err
	}
	return costs.ReplaceLedger(ctx, tx, meta.ID, "service", entryID, ledgerRows(in))
}

// Create legt einen Serviceeintrag an: Positionen, Teile, Messpunkt und
// Erledigungen in einer Transaktion (RecordServiceEntry).
func (s *Service) Create(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, id *uuid.UUID, in Input, conf Confirmation) (View, bool, error) {
	var out View
	created := true
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := identity.Authorize(ctx, tx, actor, vehicleID, identity.RoleEditor); err != nil {
			return err
		}
		batch, err := s.odo.Begin(ctx, tx, vehicleID)
		if err != nil {
			return err
		}
		q := store.New(tx)
		eid := kernel.NewID()
		if id != nil {
			eid = *id
			if ex, err := q.GetEntry(ctx, pg.U(eid)); err == nil {
				v, err := s.view(ctx, tx, ex)
				if err != nil {
					return err
				}
				norm := in
				_ = norm.validate(batch.Meta)
				if v.VehicleID == vehicleID && v.Title == norm.Title && v.OccurredAt.Equal(norm.OccurredAt) && v.Kind == norm.Kind &&
					sameTotal(v.Totals, ComputeTotals(norm.CostItems, norm.Currency)) {
					out, created = v, false
					return nil
				}
				return problem.Conflict("Ein Serviceeintrag mit dieser ID existiert mit anderem Inhalt.")
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		if err := in.validate(batch.Meta); err != nil {
			return err
		}
		if in.Odometer != nil {
			if err := batch.Add(odoInput(in), "service", eid, "/odometer", "Kilometerstand"); err != nil {
				return err
			}
		} else if err := s.checkTime(in, batch.Meta); err != nil {
			return err
		}
		ids, err := batch.Commit(ctx, actor, conf.Codes, conf.Reason)
		if err != nil {
			return err
		}
		var readingID *uuid.UUID
		if len(ids) > 0 {
			readingID = &ids[0]
		}
		row, err := q.InsertEntry(ctx, store.InsertEntryParams{ID: pg.U(eid), VehicleID: pg.U(vehicleID), Kind: in.Kind, Category: pg.T(in.Category),
			Title: in.Title, Description: pg.T(in.Description), OccurredAt: pg.TS(in.OccurredAt), TimeZone: in.TimeZone, TimePrecision: in.TimePrecision,
			OdometerReadingID: pg.UP(readingID), Currency: in.Currency, ProviderName: pg.T(in.ProviderName), InvoiceNumber: pg.T(in.InvoiceNumber),
			CostUnknown: in.CostUnknown, Note: in.Note, Tags: in.Tags, Origin: kernel.OriginOf(actor), CreatedBy: pg.U(actor.AccountID)})
		if err != nil {
			return err
		}
		if err := writeChildren(ctx, q, eid, &in); err != nil {
			return err
		}
		completes := in.Completes
		if completes == nil {
			completes = &[]uuid.UUID{}
		}
		if err := s.sideEffects(ctx, tx, actor, batch.Meta, eid, readingID, in, completes); err != nil {
			return err
		}
		if out, err = s.view(ctx, tx, row); err != nil {
			return err
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "service.entry_recorded", VehicleID: &vehicleID, ObjectType: "service_entry", ObjectID: eid,
			Changes: map[string]any{"kind": in.Kind, "title": in.Title, "totals": out.Totals, "completes": completes}, Reason: conf.Reason})
	})
	return out, created, err
}

func sameTotal(a *Totals, b Totals) bool {
	if a == nil {
		return b.Total.AmountMinor == 0
	}
	return a.Total == b.Total
}

func load(ctx context.Context, db store.DBTX, actor kernel.Actor, vehicleID, id uuid.UUID, need string) (store.ServiceEntry, error) {
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

// Get liest einen Serviceeintrag.
func (s *Service) Get(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID) (View, error) {
	e, err := load(ctx, s.pool, actor, vehicleID, id, identity.RoleViewer)
	if err != nil {
		return View{}, err
	}
	return s.view(ctx, s.pool, e)
}

// Update wendet einen Merge Patch an; Positionen und Teile werden als Ganzes
// ersetzt, der Messpunkt wird korrigiert (I-SH-3), Erledigungen folgen (SH-04).
func (s *Service) Update(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID, ifMatch string, patch []byte, conf Confirmation) (View, error) {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return View{}, err
	}
	var out View
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := load(ctx, tx, actor, vehicleID, id, identity.RoleEditor)
		if err != nil {
			return err
		}
		batch, err := s.odo.Begin(ctx, tx, vehicleID)
		if err != nil {
			return err
		}
		curView, err := s.view(ctx, tx, cur)
		if err != nil {
			return err
		}
		if int(cur.Version) != version {
			return problem.PreconditionFailed(curView)
		}
		var in Input
		if err := mergepatch.Apply(curView.Input, patch, &in); err != nil {
			return problem.BadRequest("Ungültiger Merge Patch.")
		}
		if mergepatch.Has(patch, "cost_items") && len(in.CostItems) > 0 && !mergepatch.Has(patch, "cost_unknown") {
			in.CostUnknown = false
		}
		if in.CostUnknown && !mergepatch.Has(patch, "cost_items") {
			in.CostItems = nil
		}
		if err := in.validate(batch.Meta); err != nil {
			return err
		}
		old := pg.IDP(cur.OdometerReadingID)
		readingID := old
		switch {
		case in.Odometer != nil && old != nil:
			if !batch.Same(*old, odoInput(in)) {
				if err := batch.Replace(*old, odoInput(in), "/odometer", "Kilometerstand"); err != nil {
					return err
				}
			}
		case in.Odometer != nil:
			if err := batch.Add(odoInput(in), "service", id, "/odometer", "Kilometerstand"); err != nil {
				return err
			}
		case old != nil:
			if err := batch.Remove(*old); err != nil {
				return err
			}
			readingID = nil
		}
		if in.Odometer == nil {
			if err := s.checkTime(in, batch.Meta); err != nil {
				return err
			}
		}
		ids, err := batch.Commit(ctx, actor, conf.Codes, conf.Reason)
		if err != nil {
			return err
		}
		if len(ids) > 0 {
			readingID = &ids[0]
		}
		q := store.New(tx)
		row, err := q.UpdateEntry(ctx, store.UpdateEntryParams{ID: cur.ID, Kind: in.Kind, Category: pg.T(in.Category), Title: in.Title,
			Description: pg.T(in.Description), OccurredAt: pg.TS(in.OccurredAt), TimeZone: in.TimeZone, TimePrecision: in.TimePrecision,
			OdometerReadingID: pg.UP(readingID), Currency: in.Currency, ProviderName: pg.T(in.ProviderName), InvoiceNumber: pg.T(in.InvoiceNumber),
			CostUnknown: in.CostUnknown, Note: in.Note, Tags: in.Tags, UpdatedBy: pg.U(actor.AccountID), Version: cur.Version})
		if errors.Is(err, pgx.ErrNoRows) {
			return problem.PreconditionFailed(nil)
		}
		if err != nil {
			return err
		}
		if err := writeChildren(ctx, q, id, &in); err != nil {
			return err
		}
		var completes *[]uuid.UUID
		if mergepatch.Has(patch, "completes_maintenance_item_ids") {
			completes = in.Completes
			if completes == nil {
				completes = &[]uuid.UUID{}
			}
		}
		if err := s.sideEffects(ctx, tx, actor, batch.Meta, id, readingID, in, completes); err != nil {
			return err
		}
		if out, err = s.view(ctx, tx, row); err != nil {
			return err
		}
		changes := map[string]any{"patch": json.RawMessage(patch)}
		if cur.Kind != in.Kind { // I-SH-4
			changes["kind"] = map[string]string{"old": cur.Kind, "new": in.Kind}
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "service.entry_updated", VehicleID: &vehicleID, ObjectType: "service_entry", ObjectID: id,
			Changes: changes, Reason: conf.Reason})
	})
	return out, err
}

// Delete löscht weich: Messpunkt, Erledigungen und Kostenbuch-Zeilen verschwinden
// in derselben Transaktion; Fälligkeiten richten sich wieder nach der vorherigen Erledigung.
func (s *Service) Delete(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID, ifMatch string) error {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := load(ctx, tx, actor, vehicleID, id, identity.RoleEditor)
		if err != nil {
			return err
		}
		batch, err := s.odo.Begin(ctx, tx, vehicleID)
		if err != nil {
			return err
		}
		if int(cur.Version) != version {
			return problem.PreconditionFailed(nil)
		}
		if r := pg.IDP(cur.OdometerReadingID); r != nil {
			if err := batch.Remove(*r); err != nil {
				return err
			}
			if _, err := batch.Commit(ctx, actor, nil, ""); err != nil {
				return err
			}
		}
		if n, err := store.New(tx).SoftDeleteEntry(ctx, store.SoftDeleteEntryParams{ID: cur.ID, UpdatedBy: pg.U(actor.AccountID), Version: cur.Version}); err != nil || n == 0 {
			if err != nil {
				return err
			}
			return problem.PreconditionFailed(nil)
		}
		if err := maintenance.RemoveServiceCompletions(ctx, tx, id); err != nil {
			return err
		}
		if err := costs.ReplaceLedger(ctx, tx, vehicleID, "service", id, nil); err != nil {
			return err
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "service.entry_deleted", VehicleID: &vehicleID, ObjectType: "service_entry", ObjectID: id})
	})
}

// Filter filtert die Liste (Art, Zeitraum, Volltext).
type Filter struct {
	From, To       *time.Time
	Kind, Q        *string
	IncludeDeleted bool
	Cursor         *string
	Limit          int
}

// List liefert Serviceeinträge, neueste zuerst.
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
	p := store.ListEntriesParams{VehicleID: pg.U(vehicleID), IncludeDeleted: f.IncludeDeleted, Kind: pg.T(f.Kind), Lim: int32(f.Limit + 1)}
	if f.Q != nil && strings.TrimSpace(*f.Q) != "" {
		qs := strings.TrimSpace(*f.Q)
		p.Q = pg.T(&qs)
	}
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
	rows, err := store.New(s.pool).ListEntries(ctx, p)
	if err != nil {
		return nil, nil, err
	}
	var next *string
	if len(rows) > f.Limit {
		last := rows[f.Limit-1]
		c := cursor.Encode(last.OccurredAt.Time.UTC().Format(time.RFC3339Nano), pg.ID(last.ID).String())
		next = &c
		rows = rows[:f.Limit]
	}
	out, err := s.views(ctx, s.pool, rows)
	return out, next, err
}

func (s *Service) view(ctx context.Context, db store.DBTX, e store.ServiceEntry) (View, error) {
	vs, err := s.views(ctx, db, []store.ServiceEntry{e})
	if err != nil {
		return View{}, err
	}
	return vs[0], nil
}

// views baut die Lesedarstellungen mit Positionen, Teilen, Messpunkt und Erledigungen.
func (s *Service) views(ctx context.Context, db store.DBTX, rows []store.ServiceEntry) ([]View, error) {
	out := []View{}
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]pgtype.UUID, 0, len(rows))
	var readingIDs []uuid.UUID
	for _, r := range rows {
		ids = append(ids, r.ID)
		if id := pg.IDP(r.OdometerReadingID); id != nil {
			readingIDs = append(readingIDs, *id)
		}
	}
	q := store.New(db)
	items, err := q.ListCostItems(ctx, ids)
	if err != nil {
		return nil, err
	}
	parts, err := q.ListPartLines(ctx, ids)
	if err != nil {
		return nil, err
	}
	byItems, byParts := map[uuid.UUID][]CostItem{}, map[uuid.UUID][]PartLine{}
	for _, c := range items {
		id := pg.ID(c.ID)
		byItems[pg.ID(c.EntryID)] = append(byItems[pg.ID(c.EntryID)], CostItem{ID: &id, Kind: c.Kind, Label: pg.TP(c.Label), AmountMinor: c.AmountMinor})
	}
	for _, p := range parts {
		id := pg.ID(p.ID)
		byParts[pg.ID(p.EntryID)] = append(byParts[pg.ID(p.EntryID)], PartLine{ID: &id, Name: p.Name, PartNumber: pg.TP(p.PartNumber),
			Quantity: pg.Float(p.Quantity), QuantityUnit: pg.TP(p.QuantityUnit), UnitPriceMinor: pg.I8P(p.UnitPriceMinor), CostItemID: pg.IDP(p.CostItemID)})
	}
	infos, err := s.odo.ReadingInfos(ctx, db, pg.ID(rows[0].VehicleID), readingIDs)
	if err != nil {
		return nil, err
	}
	meta, err := vehicles.LoadMeta(ctx, db, pg.ID(rows[0].VehicleID), false)
	if err != nil {
		return nil, err
	}
	cu := kernel.UnitMeter
	if meta.UsageMeter == odometer.MeterEngineHours {
		cu = kernel.UnitSecond
	}
	for _, r := range rows {
		eid := pg.ID(r.ID)
		completes, err := maintenance.ServiceItemIDs(ctx, db, eid)
		if err != nil {
			return nil, err
		}
		upd, rec, by := r.UpdatedAt.Time, r.RecordedAt.Time, pg.ID(r.UpdatedBy)
		tags := r.Tags
		if tags == nil {
			tags = []string{}
		}
		v := View{
			EntityMeta: kernel.EntityMeta{ID: eid, Version: int(r.Version), VehicleID: pg.ID(r.VehicleID), CreatedAt: r.CreatedAt.Time,
				CreatedBy: pg.ID(r.CreatedBy), UpdatedAt: &upd, UpdatedBy: &by, RecordedAt: &rec, Origin: r.Origin},
			Input: Input{OccurredAt: r.OccurredAt.Time, TimeZone: r.TimeZone, TimePrecision: r.TimePrecision, Kind: r.Kind, Category: pg.TP(r.Category),
				Title: r.Title, Description: pg.TP(r.Description), Currency: r.Currency, ProviderName: pg.TP(r.ProviderName),
				InvoiceNumber: pg.TP(r.InvoiceNumber), CostUnknown: r.CostUnknown, CostItems: byItems[eid], Parts: byParts[eid], Completes: &completes,
				Note: r.Note, Tags: tags},
			OdometerReadingID: pg.IDP(r.OdometerReadingID),
		}
		if v.CostItems == nil {
			v.CostItems = []CostItem{}
		}
		if v.Parts == nil {
			v.Parts = []PartLine{}
		}
		if rid := v.OdometerReadingID; rid != nil {
			if ri, ok := infos[*rid]; ok {
				v.Odometer = &vehicles.Q{Value: ri.InputValue, Unit: ri.InputUnit}
				v.OdometerTotal = &odometer.QuantityView{Canonical: ri.Total, CanonicalUnit: cu}
			}
		}
		if !r.CostUnknown {
			t := ComputeTotals(v.CostItems, r.Currency)
			v.Totals = &t
		}
		out = append(out, v)
	}
	return out, nil
}

// CurrencySummary ist ein Eintrag von ServiceSummary.currencies.
type CurrencySummary struct {
	Currency   string           `json:"currency"`
	ByKind     map[string]int64 `json:"by_kind"`
	ByCostKind map[string]int64 `json:"by_cost_kind"`
	TotalMinor int64            `json:"total_minor"`
}

// Summary entspricht dem API-Schema ServiceSummary (SH-02).
type Summary struct {
	From               string            `json:"from"`
	To                 string            `json:"to"`
	Currencies         []CurrencySummary `json:"currencies"`
	Entries            int               `json:"entries"`
	EntriesCostUnknown int               `json:"entries_cost_unknown"`
}

// Summarize summiert Einträge im Zeitraum je Art und Kostenart, getrennt je Währung (SH-02).
func Summarize(entries []View, from, to time.Time) Summary {
	sum := Summary{From: kernel.FormatDate(from), To: kernel.FormatDate(to), Currencies: []CurrencySummary{}}
	byCur := map[string]*CurrencySummary{}
	for _, e := range entries {
		day := kernel.LocalDate(e.OccurredAt, e.TimeZone)
		if day.Before(from) || day.After(to) {
			continue
		}
		sum.Entries++
		if e.CostUnknown {
			sum.EntriesCostUnknown++
			continue
		}
		cs := byCur[e.Currency]
		if cs == nil {
			cs = &CurrencySummary{Currency: e.Currency, ByKind: map[string]int64{}, ByCostKind: map[string]int64{}}
			byCur[e.Currency] = cs
		}
		for _, c := range e.CostItems {
			cs.ByKind[e.Kind] += c.AmountMinor
			cs.ByCostKind[c.Kind] += c.AmountMinor
			cs.TotalMinor += c.AmountMinor
		}
	}
	for _, c := range byCur {
		sum.Currencies = append(sum.Currencies, *c)
	}
	sort.Slice(sum.Currencies, func(i, j int) bool { return sum.Currencies[i].Currency < sum.Currencies[j].Currency })
	return sum
}

// ServiceSummary liefert die Zusammenfassung (Standard: laufendes Jahr).
func (s *Service) ServiceSummary(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, from, to *time.Time) (Summary, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return Summary{}, err
	}
	meta, err := vehicles.LoadMeta(ctx, s.pool, vehicleID, false)
	if err != nil {
		return Summary{}, err
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
		return Summary{}, problem.Validation(problem.FieldError{Pointer: "/to", Code: "order"})
	}
	rows, err := store.New(s.pool).ListAllEntries(ctx, pg.U(vehicleID))
	if err != nil {
		return Summary{}, err
	}
	vs, err := s.views(ctx, s.pool, rows)
	if err != nil {
		return Summary{}, err
	}
	return Summarize(vs, f, t), nil
}
