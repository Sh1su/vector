package odometer

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/odometer/store"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

// Messpunkte, die einem anderen Eintrag gehören (Tanken, Öl …; I-ODO-3, I-FU-3/4).
// Der besitzende Service plant in seiner Transaktion, sammelt die Befunde aus
// ODO-02/ODO-03 zusammen mit den eigenen (FU-08, OI-05) und wendet den Plan erst
// nach der gemeinsamen Bestätigung an.

// OwnedPlan beschreibt die nötige Änderung am eigenen Messpunkt.
type OwnedPlan struct {
	vehicleID uuid.UUID
	meter     string
	source    string
	ref       uuid.UUID
	existing  *store.OdometerReading
	cand      Reading
	qty       kernel.Quantity
	note      string
	action    string // none | insert | replace | remove
	anomalies []problem.Anomaly
}

// Anomalies sind die Befunde des Messpunkts (P1–P4).
func (p *OwnedPlan) Anomalies() []problem.Anomaly { return p.anomalies }

// Total liefert die Gesamtlaufleistung des geplanten Messpunkts (nach Anwendung).
func (p *OwnedPlan) Value() (int64, bool) {
	switch p.action {
	case "insert", "replace":
		return p.cand.Value, true
	case "none":
		if p.existing != nil {
			return p.existing.Value, true
		}
	}
	return 0, false
}

// PlanOwned plant den Messpunkt eines Eintrags. in == nil entfernt ihn.
func (s *Service) PlanOwned(ctx context.Context, tx pgx.Tx, meta vehicles.Meta, source string, ref uuid.UUID, in *Input) (*OwnedPlan, error) {
	q := store.New(tx)
	p := &OwnedPlan{vehicleID: meta.ID, meter: meta.UsageMeter, source: source, ref: ref, action: "none"}
	ex, err := q.GetOwnedReading(ctx, store.GetOwnedReadingParams{VehicleID: pgU(meta.ID), Source: source, SourceRef: pgU(ref)})
	if err == nil {
		p.existing = &ex
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if in == nil {
		if p.existing != nil {
			p.action = "remove"
		}
		return p, nil
	}
	in.ID = nil
	cand, qty, err := s.prepare(meta, *in)
	if err != nil {
		var pe *problem.Error
		if errors.As(err, &pe) {
			for i := range pe.Errors {
				pe.Errors[i].Pointer = "/odometer" + strings.TrimPrefix(strings.TrimPrefix(pe.Errors[i].Pointer, "/value"), "/occurred_at")
			}
		}
		return nil, err
	}
	if p.existing != nil && p.existing.Value == cand.Value && p.existing.OccurredAt.Time.Equal(cand.At) {
		return p, nil
	}
	p.cand, p.qty, p.note = cand, qty, in.Note
	if a := CheckFuture(cand, s.Now()); a != nil {
		p.anomalies = append(p.anomalies, *a)
	}
	segs, valid, err := s.state(ctx, tx, meta.ID)
	if err != nil {
		return nil, err
	}
	others := valid[:0:0]
	for _, r := range valid {
		if p.existing == nil || r.ID != uuid.UUID(p.existing.ID.Bytes) {
			others = append(others, r)
		}
	}
	p.anomalies = append(p.anomalies, CheckNeighbors(cand, others, segs, meta.UsageMeter, s.cfg)...)
	p.action = "insert"
	if p.existing != nil {
		p.action = "replace"
	}
	return p, nil
}

// ApplyOwned führt den Plan aus und liefert die ID des gültigen Messpunkts.
// records sind die bestätigten Befunde des Messpunkts.
func (s *Service) ApplyOwned(ctx context.Context, tx pgx.Tx, actor kernel.Actor, p *OwnedPlan, records []AnomalyRecord) (*uuid.UUID, error) {
	q := store.New(tx)
	switch p.action {
	case "none":
		if p.existing == nil {
			return nil, nil
		}
		id := uuid.UUID(p.existing.ID.Bytes)
		return &id, nil
	case "remove":
		n, err := q.SoftDeleteReading(ctx, store.SoftDeleteReadingParams{ID: p.existing.ID, Version: p.existing.Version})
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return nil, problem.PreconditionFailed(nil)
		}
		return nil, nil
	}
	status := StatusValid
	if len(records) > 0 {
		status = StatusConfirmedAnomaly
	}
	if records == nil {
		records = []AnomalyRecord{}
	}
	ra, _ := json.Marshal(records)
	params := store.InsertReadingParams{ID: pgU(p.cand.ID), VehicleID: pgU(p.vehicleID),
		OccurredAt: pgtype.Timestamptz{Time: p.cand.At, Valid: true}, TimeZone: p.cand.TimeZone, TimePrecision: p.cand.Precision,
		Value: p.cand.Value, InputValue: numeric(p.qty.InputValue), InputUnit: p.qty.InputUnit, Source: p.source, SourceRef: pgU(p.ref),
		Status: status, ConfirmedAnomalies: ra, Note: p.note, Origin: originOf(actor), CreatedBy: pgU(actor.AccountID)}
	if p.action == "replace" {
		n, err := q.MarkSuperseded(ctx, store.MarkSupersededParams{ID: p.existing.ID, Version: p.existing.Version})
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return nil, problem.PreconditionFailed(nil)
		}
		params.SupersedesID = p.existing.ID
		params.PhotoFileID = p.existing.PhotoFileID
	}
	if _, err := q.InsertReading(ctx, params); err != nil {
		return nil, err
	}
	id := p.cand.ID
	return &id, nil
}

// ConfirmOrReject wertet gesammelte Befunde aus (ADR-010) und liefert die
// bestätigten Einträge mit Begründung.
func ConfirmOrReject(anomalies []problem.Anomaly, confirm []string, reason string) ([]AnomalyRecord, error) {
	return confirmOrReject(anomalies, confirm, reason)
}

// State lädt Abschnitte und gültige Messpunkte (für Berechnungen anderer Module).
func (s *Service) State(ctx context.Context, db store.DBTX, vehicleID uuid.UUID) (Segments, []Reading, error) {
	return s.state(ctx, db, vehicleID)
}

// OwnedReading ist der gültige Messpunkt eines besitzenden Eintrags.
type OwnedReading struct {
	Reading
	Total int64
}

// Owned ordnet jedem besitzenden Eintrag seinen gültigen Messpunkt zu
// (Korrekturen über die Kilometerseite eingeschlossen).
func Owned(valid []Reading, segs Segments, source string) map[uuid.UUID]OwnedReading {
	out := map[uuid.UUID]OwnedReading{}
	for _, r := range valid {
		if r.Source == source && r.SourceRef != nil {
			out[*r.SourceRef] = OwnedReading{Reading: r, Total: segs.Total(r)}
		}
	}
	return out
}

// DailyRate liefert die durchschnittliche Tagesleistung (ODO-08) in kanonischer
// Einheit pro Tag; ok = false, wenn unbekannt.
func DailyRate(valid []Reading, segs Segments, now time.Time) (float64, bool) {
	if len(valid) < 2 {
		return 0, false
	}
	from := now.AddDate(0, 0, -90)
	n := 0
	for _, r := range valid {
		if !r.At.Before(from) && !r.At.After(now) {
			n++
		}
	}
	days := 90.0
	if n < 2 {
		first := valid[0].At
		days = now.Sub(first).Hours() / 24
		if days < 14 {
			return 0, false
		}
		from = first
	}
	d := DistanceBetween(valid, segs, from, now)
	if !d.Known || d.Meters < 0 {
		return 0, false
	}
	return float64(d.Meters) / days, true
}
