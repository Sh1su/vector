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
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/odometer/store"
	"github.com/sh1su/vector/backend/internal/platform/audit"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

// Service ist der Application Service des Moduls Odometer.
type Service struct {
	pool *pgxpool.Pool
	cfg  Config
	Now  func() time.Time
}

func NewService(pool *pgxpool.Pool, cfg Config) *Service {
	return &Service{pool: pool, cfg: cfg, Now: time.Now}
}

// Input ist die Eingabe eines Messpunkts.
type Input struct {
	ID          *uuid.UUID
	OccurredAt  time.Time
	TimeZone    string
	Precision   string
	Value       float64
	Unit        string
	PhotoFileID *uuid.UUID
	Note        string
	Confirm     []string
	Reason      string
}

// QuantityView entspricht dem API-Schema Quantity.
type QuantityView struct {
	Canonical     int64    `json:"canonical"`
	CanonicalUnit string   `json:"canonical_unit"`
	InputValue    *float64 `json:"input_value,omitempty"`
	InputUnit     *string  `json:"input_unit,omitempty"`
}

// AnomalyRecord ist ein bestätigter Befund mit Begründung.
type AnomalyRecord struct {
	Code        string `json:"code"`
	Confirmable bool   `json:"confirmable"`
	Message     string `json:"message,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

// View entspricht dem API-Schema OdometerReading.
type View struct {
	ID                 uuid.UUID       `json:"id"`
	Version            int             `json:"version"`
	VehicleID          uuid.UUID       `json:"vehicle_id"`
	OccurredAt         time.Time       `json:"occurred_at"`
	TimeZone           string          `json:"time_zone"`
	TimePrecision      string          `json:"time_precision"`
	MeterValue         QuantityView    `json:"meter_value"`
	Total              QuantityView    `json:"total"`
	SegmentID          uuid.UUID       `json:"segment_id"`
	Source             string          `json:"source"`
	SourceRef          *uuid.UUID      `json:"source_ref"`
	Status             string          `json:"status"`
	SupersedesID       *uuid.UUID      `json:"supersedes_id"`
	SupersededByID     *uuid.UUID      `json:"superseded_by_id"`
	ConfirmedAnomalies []AnomalyRecord `json:"confirmed_anomalies"`
	PhotoFileID        *uuid.UUID      `json:"photo_file_id"`
	Note               string          `json:"note"`
	Origin             string          `json:"origin"`
	CreatedAt          time.Time       `json:"created_at"`
	CreatedBy          uuid.UUID       `json:"created_by"`
	RecordedAt         time.Time       `json:"recorded_at"`
}

func pgU(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }
func pgUp(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgU(*id)
}
func uptr(u pgtype.UUID) *uuid.UUID {
	if !u.Valid {
		return nil
	}
	id := uuid.UUID(u.Bytes)
	return &id
}

func canonicalUnit(meter string) string {
	if meter == MeterEngineHours {
		return kernel.UnitSecond
	}
	return kernel.UnitMeter
}

func toReading(r store.OdometerReading) Reading {
	return Reading{ID: uuid.UUID(r.ID.Bytes), At: r.OccurredAt.Time, TimeZone: r.TimeZone, Precision: r.TimePrecision,
		RecordedAt: r.RecordedAt.Time, Value: r.Value, Status: r.Status}
}

func (s *Service) view(r store.OdometerReading, segs Segments, meter string, successor *uuid.UUID) View {
	cu := canonicalUnit(meter)
	iv, _ := r.InputValue.Float64Value()
	f := iv.Float64
	unit := r.InputUnit
	seg := segs.At(r.OccurredAt.Time)
	var anomalies []AnomalyRecord
	_ = json.Unmarshal(r.ConfirmedAnomalies, &anomalies)
	if anomalies == nil {
		anomalies = []AnomalyRecord{}
	}
	return View{ID: uuid.UUID(r.ID.Bytes), Version: int(r.Version), VehicleID: uuid.UUID(r.VehicleID.Bytes), OccurredAt: r.OccurredAt.Time,
		TimeZone: r.TimeZone, TimePrecision: r.TimePrecision,
		MeterValue: QuantityView{Canonical: r.Value, CanonicalUnit: cu, InputValue: &f, InputUnit: &unit},
		Total:      QuantityView{Canonical: r.Value + seg.Offset, CanonicalUnit: cu},
		SegmentID:  seg.ID, Source: r.Source, SourceRef: uptr(r.SourceRef), Status: r.Status, SupersedesID: uptr(r.SupersedesID),
		SupersededByID: successor, ConfirmedAnomalies: anomalies, PhotoFileID: uptr(r.PhotoFileID), Note: r.Note, Origin: r.Origin,
		CreatedAt: r.CreatedAt.Time, CreatedBy: uuid.UUID(r.CreatedBy.Bytes), RecordedAt: r.RecordedAt.Time}
}

// state lädt Abschnitte und gültige Messpunkte eines Fahrzeugs.
func (s *Service) state(ctx context.Context, db store.DBTX, vehicleID uuid.UUID) (Segments, []Reading, error) {
	q := store.New(db)
	rows, err := q.ListSegments(ctx, pgU(vehicleID))
	if err != nil {
		return nil, nil, err
	}
	stored := make([]Segment, 0, len(rows))
	for _, r := range rows {
		stored = append(stored, Segment{ID: uuid.UUID(r.ID.Bytes), Seq: int(r.SequenceNo), StartedAt: r.StartedAt.Time, StartMeter: r.StartMeterValue, Offset: r.Offset})
	}
	segs := NewSegments(vehicleID, stored)
	rs, err := q.ListValidReadings(ctx, pgU(vehicleID))
	if err != nil {
		return nil, nil, err
	}
	valid := make([]Reading, 0, len(rs))
	for _, r := range rs {
		valid = append(valid, toReading(r))
	}
	Sort(valid)
	return segs, valid, nil
}

func allowedUnit(meter, unit string) bool {
	if meter == MeterEngineHours {
		return unit == "h" || unit == "s"
	}
	return unit == "km" || unit == "mi" || unit == "m"
}

// confirmOrReject wertet Befunde aus (ADR-010): nicht bestätigbare → 422,
// bestätigbare ohne Bestätigung → 422; bestätigte → Liste für die Speicherung.
func confirmOrReject(anomalies []problem.Anomaly, confirm []string, reason string) ([]AnomalyRecord, error) {
	if len(anomalies) == 0 {
		return []AnomalyRecord{}, nil
	}
	confirmed := map[string]bool{}
	for _, c := range confirm {
		confirmed[c] = true
	}
	var records []AnomalyRecord
	for _, a := range anomalies {
		if !a.Confirmable || !confirmed[a.Code] {
			return nil, problem.Plausibility(anomalies)
		}
		records = append(records, AnomalyRecord{Code: a.Code, Confirmable: true, Message: a.Message, Reason: reason})
	}
	if strings.TrimSpace(reason) == "" {
		return nil, problem.Validation(problem.FieldError{Pointer: "/anomaly_reason", Code: "required", Message: "Begründung für die Bestätigung fehlt."})
	}
	return records, nil
}

// prepare validiert Eingabe, Einheit und Zeit; liefert den Kandidaten.
func (s *Service) prepare(meta vehicles.Meta, in Input) (Reading, kernel.Quantity, error) {
	var errs []problem.FieldError
	if !allowedUnit(meta.UsageMeter, in.Unit) {
		errs = append(errs, problem.FieldError{Pointer: "/value/unit", Code: "unit", Message: "Einheit passt nicht zur Zählergröße des Fahrzeugs."})
	}
	if in.Value < 0 {
		errs = append(errs, problem.FieldError{Pointer: "/value/value", Code: "range"})
	}
	et, err := kernel.NormalizeEventTime(in.OccurredAt, in.TimeZone, in.Precision)
	if err != nil {
		errs = append(errs, problem.FieldError{Pointer: "/time_zone", Code: "time", Message: err.Error()})
	}
	if len(errs) > 0 {
		return Reading{}, kernel.Quantity{}, problem.Validation(errs...)
	}
	q, err := kernel.ToCanonical(in.Value, in.Unit)
	if err != nil {
		return Reading{}, kernel.Quantity{}, problem.Validation(problem.FieldError{Pointer: "/value/unit", Code: "unit"})
	}
	if meta.SaleDate != nil && kernel.LocalDate(et.At, et.TimeZone).After(*meta.SaleDate) {
		return Reading{}, kernel.Quantity{}, problem.Validation(problem.FieldError{Pointer: "/occurred_at", Code: "after_sale", Message: "Der Zeitpunkt liegt nach dem Verkaufsdatum."})
	}
	id := kernel.NewID()
	if in.ID != nil {
		id = *in.ID
	}
	return Reading{ID: id, At: et.At, TimeZone: et.TimeZone, Precision: et.Precision, RecordedAt: s.Now(), Value: q.Canonical, Status: StatusValid}, q, nil
}

func numeric(dec string) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(dec)
	return n
}

// Create erfasst einen Messpunkt (ODO-01 bis ODO-03). source/sourceRef setzen
// andere Module (Tanken, Service …); manuell: "manual", nil.
func (s *Service) Create(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, in Input, source string, sourceRef *uuid.UUID) (View, bool, error) {
	var out View
	created := true
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := identity.Authorize(ctx, tx, actor, vehicleID, identity.RoleEditor); err != nil {
			return err
		}
		meta, err := vehicles.LoadMeta(ctx, tx, vehicleID, true) // serialisiert Messpunkte je Fahrzeug
		if err != nil {
			return err
		}
		q := store.New(tx)
		if in.ID != nil {
			ex, err := q.GetReading(ctx, pgU(*in.ID))
			if err == nil {
				segs, _, err := s.state(ctx, tx, vehicleID)
				if err != nil {
					return err
				}
				cand, _, perr := s.prepare(meta, in)
				if perr == nil && uuid.UUID(ex.VehicleID.Bytes) == vehicleID && ex.OccurredAt.Time.Equal(cand.At) && ex.Value == cand.Value && ex.Note == in.Note {
					out, created = s.view(ex, segs, meta.UsageMeter, nil), false
					return nil
				}
				return problem.Conflict("Ein Messpunkt mit dieser ID existiert mit anderem Inhalt.")
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		cand, qty, err := s.prepare(meta, in)
		if err != nil {
			return err
		}
		if a := CheckFuture(cand, s.Now()); a != nil {
			return problem.Plausibility([]problem.Anomaly{*a})
		}
		segs, valid, err := s.state(ctx, tx, vehicleID)
		if err != nil {
			return err
		}
		records, err := confirmOrReject(CheckNeighbors(cand, valid, segs, meta.UsageMeter, s.cfg), in.Confirm, in.Reason)
		if err != nil {
			return err
		}
		status := StatusValid
		if len(records) > 0 {
			status = StatusConfirmedAnomaly
		}
		ra, _ := json.Marshal(records)
		row, err := q.InsertReading(ctx, store.InsertReadingParams{ID: pgU(cand.ID), VehicleID: pgU(vehicleID),
			OccurredAt: pgtype.Timestamptz{Time: cand.At, Valid: true}, TimeZone: cand.TimeZone, TimePrecision: cand.Precision,
			Value: cand.Value, InputValue: numeric(qty.InputValue), InputUnit: qty.InputUnit, Source: source, SourceRef: pgUp(sourceRef),
			Status: status, ConfirmedAnomalies: ra, PhotoFileID: pgUp(in.PhotoFileID), Note: in.Note, Origin: originOf(actor),
			CreatedBy: pgU(actor.AccountID)})
		if err != nil {
			return err
		}
		out = s.view(row, segs, meta.UsageMeter, nil)
		return audit.Write(ctx, tx, actor, audit.Event{Action: "odometer.reading_recorded", VehicleID: &vehicleID, ObjectType: "odometer_reading",
			ObjectID: cand.ID, Changes: map[string]any{"value": cand.Value, "status": status, "anomalies": records}, Reason: in.Reason})
	})
	return out, created, err
}

func originOf(a kernel.Actor) string {
	if a.Kind == "api_token" {
		return "api"
	}
	return "web"
}

// Correct ersetzt einen Messpunkt durch einen neuen (I-ODO-4).
func (s *Service) Correct(ctx context.Context, actor kernel.Actor, vehicleID, readingID uuid.UUID, ifMatch string, in Input, hasValue, hasTime bool) (View, error) {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return View{}, err
	}
	if strings.TrimSpace(in.Reason) == "" {
		return View{}, problem.Validation(problem.FieldError{Pointer: "/reason", Code: "required", Message: "Begründung der Korrektur fehlt."})
	}
	var out View
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := store.New(tx)
		old, err := q.GetReading(ctx, pgU(readingID))
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && uuid.UUID(old.VehicleID.Bytes) != vehicleID) {
			return problem.NotFound()
		}
		if err != nil {
			return err
		}
		// Rechteprüfung am geladenen Objekt (ID-01).
		if _, err := identity.Authorize(ctx, tx, actor, uuid.UUID(old.VehicleID.Bytes), identity.RoleEditor); err != nil {
			return err
		}
		meta, err := vehicles.LoadMeta(ctx, tx, vehicleID, true)
		if err != nil {
			return err
		}
		segs, valid, err := s.state(ctx, tx, vehicleID)
		if err != nil {
			return err
		}
		if int(old.Version) != version {
			return problem.PreconditionFailed(s.view(old, segs, meta.UsageMeter, nil))
		}
		if old.Status == StatusSuperseded {
			return problem.Conflict("Dieser Messpunkt wurde bereits ersetzt.")
		}
		if !hasValue {
			iv, _ := old.InputValue.Float64Value()
			in.Value, in.Unit = iv.Float64, old.InputUnit
		}
		if !hasTime {
			in.OccurredAt, in.TimeZone, in.Precision = old.OccurredAt.Time, old.TimeZone, old.TimePrecision
		}
		in.ID = nil
		if in.Note == "" {
			in.Note = old.Note
		}
		cand, qty, err := s.prepare(meta, in)
		if err != nil {
			return err
		}
		if a := CheckFuture(cand, s.Now()); a != nil {
			return problem.Plausibility([]problem.Anomaly{*a})
		}
		others := valid[:0:0]
		for _, r := range valid {
			if r.ID != readingID {
				others = append(others, r)
			}
		}
		records, err := confirmOrReject(CheckNeighbors(cand, others, segs, meta.UsageMeter, s.cfg), in.Confirm, in.Reason)
		if err != nil {
			return err
		}
		if n, err := q.MarkSuperseded(ctx, store.MarkSupersededParams{ID: old.ID, Version: old.Version}); err != nil || n == 0 {
			if err != nil {
				return err
			}
			return problem.PreconditionFailed(nil)
		}
		status := StatusValid
		if len(records) > 0 {
			status = StatusConfirmedAnomaly
		}
		ra, _ := json.Marshal(records)
		row, err := q.InsertReading(ctx, store.InsertReadingParams{ID: pgU(cand.ID), VehicleID: pgU(vehicleID),
			OccurredAt: pgtype.Timestamptz{Time: cand.At, Valid: true}, TimeZone: cand.TimeZone, TimePrecision: cand.Precision,
			Value: cand.Value, InputValue: numeric(qty.InputValue), InputUnit: qty.InputUnit, Source: old.Source, SourceRef: old.SourceRef,
			Status: status, ConfirmedAnomalies: ra, SupersedesID: old.ID, PhotoFileID: old.PhotoFileID, Note: in.Note,
			Origin: originOf(actor), CreatedBy: pgU(actor.AccountID)})
		if err != nil {
			return err
		}
		out = s.view(row, segs, meta.UsageMeter, nil)
		return audit.Write(ctx, tx, actor, audit.Event{Action: "odometer.reading_corrected", VehicleID: &vehicleID, ObjectType: "odometer_reading",
			ObjectID: cand.ID, Reason: in.Reason, Changes: map[string]any{"supersedes": readingID,
				"value": map[string]int64{"old": old.Value, "new": cand.Value}, "anomalies": records}})
	})
	return out, err
}

// Delete löscht einen manuellen Messpunkt weich (I-ODO-3).
func (s *Service) Delete(ctx context.Context, actor kernel.Actor, vehicleID, readingID uuid.UUID, ifMatch string) error {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := store.New(tx)
		r, err := q.GetReading(ctx, pgU(readingID))
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && uuid.UUID(r.VehicleID.Bytes) != vehicleID) {
			return problem.NotFound()
		}
		if err != nil {
			return err
		}
		if _, err := identity.Authorize(ctx, tx, actor, uuid.UUID(r.VehicleID.Bytes), identity.RoleEditor); err != nil {
			return err
		}
		if r.Source != "manual" {
			return problem.Conflict("Dieser Messpunkt gehört zu einem anderen Eintrag (" + r.Source + ") und wird dort geändert (I-ODO-3).")
		}
		if int(r.Version) != version {
			return problem.PreconditionFailed(nil)
		}
		if n, err := q.SoftDeleteReading(ctx, store.SoftDeleteReadingParams{ID: r.ID, Version: r.Version}); err != nil || n == 0 {
			if err != nil {
				return err
			}
			return problem.PreconditionFailed(nil)
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "odometer.reading_deleted", VehicleID: &vehicleID, ObjectType: "odometer_reading", ObjectID: readingID})
	})
}

// Get liest einen Messpunkt.
func (s *Service) Get(ctx context.Context, actor kernel.Actor, vehicleID, readingID uuid.UUID) (View, error) {
	q := store.New(s.pool)
	r, err := q.GetReading(ctx, pgU(readingID))
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && uuid.UUID(r.VehicleID.Bytes) != vehicleID) {
		return View{}, problem.NotFound()
	}
	if err != nil {
		return View{}, err
	}
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return View{}, err
	}
	meta, err := vehicles.LoadMeta(ctx, s.pool, vehicleID, false)
	if err != nil {
		return View{}, err
	}
	segs, _, err := s.state(ctx, s.pool, vehicleID)
	if err != nil {
		return View{}, err
	}
	var succ *uuid.UUID
	if id, err := q.GetSuccessor(ctx, r.ID); err == nil {
		succ = uptr(id)
	}
	return s.view(r, segs, meta.UsageMeter, succ), nil
}

// ListFilter filtert die Historie.
type ListFilter struct {
	From, To          *time.Time
	IncludeSuperseded bool
	Cursor            *string
	Limit             int
}

// List liefert die Historie, neueste zuerst.
func (s *Service) List(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, f ListFilter) ([]View, *string, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return nil, nil, err
	}
	meta, err := vehicles.LoadMeta(ctx, s.pool, vehicleID, false)
	if err != nil {
		return nil, nil, err
	}
	segs, _, err := s.state(ctx, s.pool, vehicleID)
	if err != nil {
		return nil, nil, err
	}
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	p := store.ListReadingsPageParams{VehicleID: pgU(vehicleID), IncludeSuperseded: f.IncludeSuperseded, Lim: int32(f.Limit + 1)}
	if f.From != nil {
		p.FromTs = pgtype.Timestamptz{Time: *f.From, Valid: true}
	}
	if f.To != nil {
		p.ToTs = pgtype.Timestamptz{Time: *f.To, Valid: true}
	}
	if f.Cursor != nil {
		ts, id, ok := decodeCursor(*f.Cursor)
		if !ok {
			return nil, nil, problem.Validation(problem.FieldError{Pointer: "/cursor", Code: "invalid"})
		}
		p.BeforeTs, p.BeforeID = pgtype.Timestamptz{Time: ts, Valid: true}, pgU(id)
	}
	rows, err := store.New(s.pool).ListReadingsPage(ctx, p)
	if err != nil {
		return nil, nil, err
	}
	succ := map[uuid.UUID]uuid.UUID{}
	for _, r := range rows {
		if r.SupersedesID.Valid {
			succ[uuid.UUID(r.SupersedesID.Bytes)] = uuid.UUID(r.ID.Bytes)
		}
	}
	var out []View
	var next *string
	for i, r := range rows {
		if i == f.Limit {
			c := encodeCursor(rows[i-1].OccurredAt.Time, uuid.UUID(rows[i-1].ID.Bytes))
			next = &c
			break
		}
		var sp *uuid.UUID
		if id, ok := succ[uuid.UUID(r.ID.Bytes)]; ok {
			sp = &id
		}
		out = append(out, s.view(r, segs, meta.UsageMeter, sp))
	}
	if out == nil {
		out = []View{}
	}
	return out, next, nil
}

// Query ist der Einstieg für Current/ValueAt/Distance mit Rechteprüfung.
func (s *Service) Query(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID) (Segments, []Reading, vehicles.Meta, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return nil, nil, vehicles.Meta{}, err
	}
	meta, err := vehicles.LoadMeta(ctx, s.pool, vehicleID, false)
	if err != nil {
		return nil, nil, vehicles.Meta{}, err
	}
	segs, valid, err := s.state(ctx, s.pool, vehicleID)
	return segs, valid, meta, err
}

// SegmentView entspricht dem API-Schema OdometerSegment.
type SegmentView struct {
	ID              uuid.UUID    `json:"id"`
	SequenceNo      int          `json:"sequence_no"`
	StartedAt       *time.Time   `json:"started_at"`
	StartMeterValue QuantityView `json:"start_meter_value"`
	Offset          QuantityView `json:"offset"`
	Reason          string       `json:"reason"`
}

// Segments listet die Zählerabschnitte (inkl. implizitem Abschnitt 1).
func (s *Service) ListSegments(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID) ([]SegmentView, error) {
	segs, _, meta, err := s.Query(ctx, actor, vehicleID)
	if err != nil {
		return nil, err
	}
	cu := canonicalUnit(meta.UsageMeter)
	out := make([]SegmentView, 0, len(segs))
	for _, sg := range segs {
		v := SegmentView{ID: sg.ID, SequenceNo: sg.Seq, Reason: "initial",
			StartMeterValue: QuantityView{Canonical: sg.StartMeter, CanonicalUnit: cu}, Offset: QuantityView{Canonical: sg.Offset, CanonicalUnit: cu}}
		if sg.Seq > 1 {
			t := sg.StartedAt
			v.StartedAt, v.Reason = &t, "replacement"
		}
		out = append(out, v)
	}
	return out, nil
}

// SegmentInput beschreibt einen Tachotausch oder Überlauf (ODO-07).
type SegmentInput struct {
	StartedAt     time.Time
	TimeZone      string
	OldFinal      *Input // optional: letzter Stand des alten Instruments
	StartMeter    float64
	StartUnit     string
	Reason        string
	Note          string
	Confirm       []string
	AnomalyReason string
}

// CreateSegment beginnt einen neuen Zählerabschnitt.
func (s *Service) CreateSegment(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, in SegmentInput) (SegmentView, error) {
	var out SegmentView
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := identity.Authorize(ctx, tx, actor, vehicleID, identity.RoleEditor); err != nil {
			return err
		}
		meta, err := vehicles.LoadMeta(ctx, tx, vehicleID, true)
		if err != nil {
			return err
		}
		if in.Reason != "replacement" && in.Reason != "rollover" {
			return problem.Validation(problem.FieldError{Pointer: "/reason", Code: "enum"})
		}
		if !allowedUnit(meta.UsageMeter, in.StartUnit) {
			return problem.Validation(problem.FieldError{Pointer: "/start_meter_value/unit", Code: "unit"})
		}
		if in.StartedAt.After(s.Now().Add(5 * time.Minute)) {
			return problem.Validation(problem.FieldError{Pointer: "/started_at", Code: "future"})
		}
		segs, valid, err := s.state(ctx, tx, vehicleID)
		if err != nil {
			return err
		}
		last := segs.Last()
		if last.Seq > 1 && !in.StartedAt.After(last.StartedAt) {
			return problem.Validation(problem.FieldError{Pointer: "/started_at", Code: "order", Message: "Muss nach dem Beginn des aktuellen Abschnitts liegen."})
		}
		oldSeg := segs.At(in.StartedAt)
		var oldFinal int64
		if in.OldFinal != nil {
			in.OldFinal.OccurredAt = in.StartedAt.Add(-time.Second)
			in.OldFinal.TimeZone = in.TimeZone
			in.OldFinal.Precision = kernel.PrecisionExact
			in.OldFinal.Confirm, in.OldFinal.Reason = in.Confirm, in.AnomalyReason
			cand, qty, err := s.prepare(meta, *in.OldFinal)
			if err != nil {
				return err
			}
			if _, err := confirmOrReject(CheckNeighbors(cand, valid, segs, meta.UsageMeter, s.cfg), in.Confirm, in.AnomalyReason); err != nil {
				return err
			}
			if _, err := store.New(tx).InsertReading(ctx, store.InsertReadingParams{ID: pgU(cand.ID), VehicleID: pgU(vehicleID),
				OccurredAt: pgtype.Timestamptz{Time: cand.At, Valid: true}, TimeZone: cand.TimeZone, TimePrecision: cand.Precision,
				Value: cand.Value, InputValue: numeric(qty.InputValue), InputUnit: qty.InputUnit, Source: "manual", Status: StatusValid,
				ConfirmedAnomalies: []byte("[]"), Note: "Letzter Stand vor Tachotausch", Origin: originOf(actor), CreatedBy: pgU(actor.AccountID)}); err != nil {
				return err
			}
			oldFinal = cand.Value
		} else {
			v := ValueAt(valid, segs, in.StartedAt)
			if v.Kind == KindUnknown {
				return problem.Validation(problem.FieldError{Pointer: "/old_final_value", Code: "required", Message: "Letzter Stand des alten Instruments fehlt."})
			}
			oldFinal = v.Total - oldSeg.Offset
		}
		start, err := kernel.ToCanonical(in.StartMeter, in.StartUnit)
		if err != nil {
			return problem.Validation(problem.FieldError{Pointer: "/start_meter_value/unit", Code: "unit"})
		}
		offset := NewSegmentOffset(oldFinal, oldSeg.Offset, start.Canonical)
		id := kernel.NewID()
		row, err := store.New(tx).InsertSegment(ctx, store.InsertSegmentParams{ID: pgU(id), VehicleID: pgU(vehicleID), SequenceNo: int32(last.Seq + 1),
			StartedAt: pgtype.Timestamptz{Time: in.StartedAt, Valid: true}, TimeZone: in.TimeZone, StartMeterValue: start.Canonical,
			Offset: offset, Reason: in.Reason, Note: in.Note, CreatedBy: pgU(actor.AccountID)})
		if err != nil {
			return err
		}
		cu := canonicalUnit(meta.UsageMeter)
		t := row.StartedAt.Time
		out = SegmentView{ID: id, SequenceNo: int(row.SequenceNo), StartedAt: &t, Reason: row.Reason,
			StartMeterValue: QuantityView{Canonical: row.StartMeterValue, CanonicalUnit: cu}, Offset: QuantityView{Canonical: row.Offset, CanonicalUnit: cu}}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "odometer.segment_started", VehicleID: &vehicleID, ObjectType: "odometer_segment",
			ObjectID: id, Changes: map[string]any{"offset": offset, "old_final": oldFinal}})
	})
	return out, err
}

// HasReadings meldet, ob ein Fahrzeug Messpunkte hat (für I-VE-3).
func HasReadings(ctx context.Context, db store.DBTX, vehicleID uuid.UUID) (bool, error) {
	n, err := store.New(db).CountReadings(ctx, pgU(vehicleID))
	return n > 0, err
}
