package odometer

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/odometer/store"
	"github.com/sh1su/vector/backend/internal/platform/audit"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

// Batch bündelt Messpunkt-Änderungen eines Quellmoduls (Service, Fahrten …) in
// dessen Transaktion (I-ODO-3, SH-03, TR-02). Alle Kandidaten werden gemeinsam
// geprüft, damit sämtliche Befunde in einer 422-Antwort erscheinen.
type Batch struct {
	ctx       context.Context
	s         *Service
	tx        pgx.Tx
	Meta      vehicles.Meta
	vehicleID uuid.UUID
	segs      Segments
	valid     []Reading
	ops       []batchOp
	removed   map[uuid.UUID]bool
	anomalies []problem.Anomaly
}

type batchOp struct {
	kind      string // add | replace | remove
	cand      Reading
	qty       kernel.Quantity
	in        Input
	source    string
	sourceRef *uuid.UUID
	old       store.OdometerReading
}

// Begin sperrt das Fahrzeug (serialisiert Messpunkte) und lädt den Zustand.
// Die Rechteprüfung ist Sache des aufrufenden Moduls.
func (s *Service) Begin(ctx context.Context, tx pgx.Tx, vehicleID uuid.UUID) (*Batch, error) {
	meta, err := vehicles.LoadMeta(ctx, tx, vehicleID, true)
	if err != nil {
		return nil, err
	}
	segs, valid, err := s.state(ctx, tx, vehicleID)
	if err != nil {
		return nil, err
	}
	return &Batch{ctx: ctx, s: s, tx: tx, Meta: meta, vehicleID: vehicleID, segs: segs, valid: valid, removed: map[uuid.UUID]bool{}}, nil
}

// Snapshot liefert Abschnitte und gültige Messpunkte (ohne Rechteprüfung, für andere Module).
func (s *Service) Snapshot(ctx context.Context, db store.DBTX, vehicleID uuid.UUID) (Segments, []Reading, error) {
	return s.state(ctx, db, vehicleID)
}

// others sind die gültigen Messpunkte ohne entfernte/ersetzte, plus bereits geplante neue.
func (b *Batch) others() []Reading {
	out := make([]Reading, 0, len(b.valid)+len(b.ops))
	for _, r := range b.valid {
		if !b.removed[r.ID] {
			out = append(out, r)
		}
	}
	for _, op := range b.ops {
		if op.kind != "remove" {
			out = append(out, op.cand)
		}
	}
	Sort(out)
	return out
}

// check validiert einen Kandidaten; label kennzeichnet die Befunde (z. B. „Endstand“).
func (b *Batch) check(in Input, pointer, label string) (Reading, kernel.Quantity, error) {
	cand, qty, err := b.s.prepare(b.Meta, in)
	if err != nil {
		var pe *problem.Error
		if errors.As(err, &pe) {
			for i := range pe.Errors {
				if pe.Errors[i].Pointer == "/value/unit" || pe.Errors[i].Pointer == "/value/value" {
					pe.Errors[i].Pointer = pointer + pe.Errors[i].Pointer[len("/value"):]
				}
			}
		}
		return cand, qty, err
	}
	var found []problem.Anomaly
	if a := CheckFuture(cand, b.s.Now()); a != nil {
		found = append(found, *a)
	}
	found = append(found, CheckNeighbors(cand, b.others(), b.segs, b.Meta.UsageMeter, b.s.cfg)...)
	for _, a := range found {
		if label != "" {
			a.Message = label + ": " + a.Message
		}
		b.anomalies = append(b.anomalies, a)
	}
	return cand, qty, nil
}

// Add plant einen neuen Messpunkt des Quelleintrags.
func (b *Batch) Add(in Input, source string, sourceRef uuid.UUID, pointer, label string) error {
	cand, qty, err := b.check(in, pointer, label)
	if err != nil {
		return err
	}
	ref := sourceRef
	b.ops = append(b.ops, batchOp{kind: "add", cand: cand, qty: qty, in: in, source: source, sourceRef: &ref})
	return nil
}

func (b *Batch) owned(id uuid.UUID) (store.OdometerReading, error) {
	r, err := store.New(b.tx).GetReading(b.ctx, pgU(id))
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && uuid.UUID(r.VehicleID.Bytes) != b.vehicleID) {
		return r, problem.Conflict("Der Messpunkt des Eintrags existiert nicht mehr.")
	}
	if err == nil && r.Status == StatusSuperseded {
		return r, problem.Conflict("Der Messpunkt des Eintrags wurde bereits ersetzt.")
	}
	return r, err
}

// Replace plant die Korrektur eines eigenen Messpunkts (I-ODO-4).
func (b *Batch) Replace(oldID uuid.UUID, in Input, pointer, label string) error {
	old, err := b.owned(oldID)
	if err != nil {
		return err
	}
	b.removed[oldID] = true
	cand, qty, err := b.check(in, pointer, label)
	if err != nil {
		return err
	}
	b.ops = append(b.ops, batchOp{kind: "replace", cand: cand, qty: qty, in: in, old: old, source: old.Source, sourceRef: uptr(old.SourceRef)})
	return nil
}

// Remove plant das Löschen eines eigenen Messpunkts (Quelleintrag gelöscht).
func (b *Batch) Remove(id uuid.UUID) error {
	old, err := b.owned(id)
	if err != nil {
		return err
	}
	b.removed[id] = true
	b.ops = append(b.ops, batchOp{kind: "remove", old: old})
	return nil
}

// Same meldet, ob die Eingabe denselben Messpunkt ergibt wie ein bestehender
// (dann braucht es keine Korrektur).
func (b *Batch) Same(readingID uuid.UUID, in Input) bool {
	r, err := store.New(b.tx).GetReading(b.ctx, pgU(readingID))
	if err != nil {
		return false
	}
	cand, _, err := b.s.prepare(b.Meta, in)
	return err == nil && cand.Value == r.Value && cand.At.Equal(r.OccurredAt.Time) && cand.Precision == r.TimePrecision
}

// Commit wertet alle Befunde aus (ADR-010) und schreibt die Änderungen. Liefert
// die IDs der neuen Messpunkte in der Reihenfolge von Add/Replace.
func (b *Batch) Commit(ctx context.Context, actor kernel.Actor, confirm []string, reason string) ([]uuid.UUID, error) {
	records, err := confirmOrReject(b.anomalies, confirm, reason)
	if err != nil {
		return nil, err
	}
	status := StatusValid
	if len(records) > 0 {
		status = StatusConfirmedAnomaly
	}
	ra, _ := json.Marshal(records)
	q := store.New(b.tx)
	var ids []uuid.UUID
	for _, op := range b.ops {
		switch op.kind {
		case "remove":
			if n, err := q.SoftDeleteReading(ctx, store.SoftDeleteReadingParams{ID: op.old.ID, Version: op.old.Version}); err != nil || n == 0 {
				if err != nil {
					return nil, err
				}
				return nil, problem.PreconditionFailed(nil)
			}
			id := uuid.UUID(op.old.ID.Bytes)
			if err := audit.Write(ctx, b.tx, actor, audit.Event{Action: "odometer.reading_deleted", VehicleID: &b.vehicleID,
				ObjectType: "odometer_reading", ObjectID: id, Changes: map[string]any{"source": op.old.Source}}); err != nil {
				return nil, err
			}
			continue
		case "replace":
			if n, err := q.MarkSuperseded(ctx, store.MarkSupersededParams{ID: op.old.ID, Version: op.old.Version}); err != nil || n == 0 {
				if err != nil {
					return nil, err
				}
				return nil, problem.PreconditionFailed(nil)
			}
		}
		p := store.InsertReadingParams{ID: pgU(op.cand.ID), VehicleID: pgU(b.vehicleID),
			OccurredAt: pgtype.Timestamptz{Time: op.cand.At, Valid: true}, TimeZone: op.cand.TimeZone, TimePrecision: op.cand.Precision,
			Value: op.cand.Value, InputValue: numeric(op.qty.InputValue), InputUnit: op.qty.InputUnit, Source: op.source, SourceRef: pgUp(op.sourceRef),
			Status: status, ConfirmedAnomalies: ra, Note: op.in.Note, Origin: originOf(actor), CreatedBy: pgU(actor.AccountID)}
		action := "odometer.reading_recorded"
		changes := map[string]any{"value": op.cand.Value, "source": op.source, "status": status, "anomalies": records}
		if op.kind == "replace" {
			p.SupersedesID = op.old.ID
			p.PhotoFileID = op.old.PhotoFileID
			action = "odometer.reading_corrected"
			changes["supersedes"] = uuid.UUID(op.old.ID.Bytes)
			changes["value"] = map[string]int64{"old": op.old.Value, "new": op.cand.Value}
		}
		if _, err := q.InsertReading(ctx, p); err != nil {
			return nil, err
		}
		if err := audit.Write(ctx, b.tx, actor, audit.Event{Action: action, VehicleID: &b.vehicleID, ObjectType: "odometer_reading",
			ObjectID: op.cand.ID, Changes: changes, Reason: reason}); err != nil {
			return nil, err
		}
		ids = append(ids, op.cand.ID)
	}
	b.ops = nil
	return ids, nil
}

// ReadingInfo ist die Sicht eines Quellmoduls auf „seinen“ Messpunkt.
type ReadingInfo struct {
	ID         uuid.UUID
	Value      int64 // Zählerwert, kanonisch
	Total      int64 // Gesamtlaufleistung
	InputValue float64
	InputUnit  string
	OccurredAt time.Time
}

// ReadingInfos lädt Messpunkte (auch ersetzte, für Fassungshistorien) mit Gesamtlaufleistung.
func (s *Service) ReadingInfos(ctx context.Context, db store.DBTX, vehicleID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]ReadingInfo, error) {
	out := map[uuid.UUID]ReadingInfo{}
	if len(ids) == 0 {
		return out, nil
	}
	segs, _, err := s.state(ctx, db, vehicleID)
	if err != nil {
		return nil, err
	}
	q := store.New(db)
	for _, id := range ids {
		if _, ok := out[id]; ok {
			continue
		}
		r, err := q.GetReading(ctx, pgU(id))
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		iv, _ := r.InputValue.Float64Value()
		out[id] = ReadingInfo{ID: id, Value: r.Value, Total: r.Value + segs.At(r.OccurredAt.Time).Offset, InputValue: iv.Float64,
			InputUnit: r.InputUnit, OccurredAt: r.OccurredAt.Time}
	}
	return out, nil
}

// DailyRate schätzt die Tagesleistung (ODO-08) in kanonischen Einheiten je Tag.
func DailyRate(valid []Reading, segs Segments, now time.Time) (float64, bool) {
	if len(valid) == 0 {
		return 0, false
	}
	from := now.AddDate(0, 0, -90)
	n := 0
	for _, r := range valid {
		if !r.At.Before(from) && !r.At.After(now) {
			n++
		}
	}
	if n >= 2 {
		if d := DistanceBetween(valid, segs, from, now); d.Known {
			return float64(d.Meters) / 90, true
		}
	}
	first := valid[0]
	days := now.Sub(first.At).Hours() / 24
	if days < 14 {
		return 0, false
	}
	d := DistanceBetween(valid, segs, first.At, now)
	if !d.Known {
		return 0, false
	}
	return float64(d.Meters) / days, true
}

// Total berechnet die Gesamtlaufleistung einer Eingabe, ohne sie zu planen
// (für harte Regeln wie I-TR-3 vor der Plausibilitätsprüfung).
func (b *Batch) Total(in Input) (int64, error) {
	cand, _, err := b.s.prepare(b.Meta, in)
	if err != nil {
		return 0, err
	}
	return cand.Value + b.segs.At(cand.At).Offset, nil
}
