package vehicles

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/audit"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/vehicles/store"
)

// Meta sind die Fahrzeugangaben, die andere Module brauchen (Übersicht §5).
type Meta struct {
	ID               uuid.UUID
	UsageMeter       string
	OdometerRequired bool
	OwnerTimeZone    string
	Status           string
	SaleDate         *time.Time
	DisplayName      string
	DefaultCurrency  string
	PurchaseDate     *time.Time
	Purchase         *Money
	Sale             *Money
	EstimatedValue   *DateMoney
	EnergyCarriers   []string
	// Kanonische Kapazitäten aus den Stammdaten (Fuel, Oil); 0 = unbekannt.
	TankCapacityMl map[string]int64
	BatteryWh      int64
	OilRangeMl     int64
	OilCapacityMl  int64
}

func canonicalOf(q *Q) int64 {
	if q == nil {
		return 0
	}
	c, err := kernel.ToCanonical(q.Value, q.Unit)
	if err != nil {
		return 0
	}
	return c.Canonical
}

// Service ist der Application Service des Moduls Vehicles.
type Service struct {
	pool *pgxpool.Pool
	// HasReadings wird vom Modul Odometer gesetzt (I-VE-3), ohne Importzyklus.
	HasReadings func(ctx context.Context, db store.DBTX, vehicleID uuid.UUID) (bool, error)
	// Settings liefert Nutzervorgaben (Zeitzone, Währung) für neue Fahrzeuge.
	Settings func(ctx context.Context, accountID uuid.UUID) (map[string]any, error)
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func pgU(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }
func pgT(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}
func pgI(i *int) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*i), Valid: true}
}
func pgD(s *string) pgtype.Date {
	if s == nil {
		return pgtype.Date{}
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}
func strp(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}
func intp(t pgtype.Int4) *int {
	if !t.Valid {
		return nil
	}
	i := int(t.Int32)
	return &i
}
func datep(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format("2006-01-02")
	return &s
}

func toView(v store.VehiclesVehicle, role string) View {
	var ex extra
	_ = json.Unmarshal(v.Extra, &ex)
	odo := v.OdometerRequired
	tz, cur, note := v.OwnerTimeZone, v.DefaultCurrency, v.Note
	in := Input{
		DisplayName: v.DisplayName, VIN: strp(v.Vin), LicensePlate: strp(v.LicensePlate), PlateCountry: strp(v.PlateCountry),
		Make: strp(v.Make), Model: strp(v.Model), Variant: strp(v.Variant), ModelYear: intp(v.ModelYear),
		FirstRegistration: datep(v.FirstRegistration), BodyType: v.BodyType, EngineCode: strp(v.EngineCode),
		DisplacementCcm: intp(v.DisplacementCcm), PowerKw: intp(v.PowerKw), Transmission: strp(v.Transmission),
		UsageMeter: v.UsageMeter, EnergyCarriers: v.EnergyCarriers, OdometerRequired: &odo, OwnerTimeZone: &tz,
		DefaultCurrency: &cur, Note: &note, Tags: v.Tags,
		TankCapacity: ex.TankCapacity, BatteryUsableCapacity: ex.BatteryUsableCapacity, OilDipstickRange: ex.OilDipstickRange,
		OilCapacity: ex.OilCapacity, DisplayUnits: ex.DisplayUnits, CustomFields: ex.CustomFields, EstimatedValue: ex.EstimatedValue,
	}
	if v.PurchaseDate.Valid {
		in.Purchase = &DateMoney{Date: v.PurchaseDate.Time.Format("2006-01-02")}
		if v.PurchaseAmount.Valid {
			in.Purchase.Price = &Money{AmountMinor: v.PurchaseAmount.Int64, Currency: v.PurchaseCurrency.String}
		}
	}
	view := View{
		ID: uuid.UUID(v.ID.Bytes).String(), Version: int(v.Version), CreatedAt: v.CreatedAt.Time, CreatedBy: uuid.UUID(v.CreatedBy.Bytes).String(),
		UpdatedAt: v.UpdatedAt.Time, UpdatedBy: uuid.UUID(v.UpdatedBy.Bytes).String(), RecordedAt: v.RecordedAt.Time,
		Origin: v.Origin, Status: v.Status, MyRole: role, Input: in,
	}
	if v.SaleDate.Valid {
		view.Sale = &DateMoney{Date: v.SaleDate.Time.Format("2006-01-02")}
		if v.SaleAmount.Valid {
			view.Sale.Price = &Money{AmountMinor: v.SaleAmount.Int64, Currency: v.SaleCurrency.String}
		}
	}
	return view
}

func extraJSON(in Input) []byte {
	b, _ := json.Marshal(extra{TankCapacity: in.TankCapacity, BatteryUsableCapacity: in.BatteryUsableCapacity,
		OilDipstickRange: in.OilDipstickRange, OilCapacity: in.OilCapacity, DisplayUnits: in.DisplayUnits,
		CustomFields: in.CustomFields, EstimatedValue: in.EstimatedValue})
	return b
}

// checkAnomalies wertet harte Fehler und Befunde aus (Muster ADR-010).
func checkAnomalies(errs []problem.FieldError, anomalies []problem.Anomaly, confirm []string) error {
	if len(errs) > 0 {
		return problem.Validation(errs...)
	}
	confirmed := map[string]bool{}
	for _, c := range confirm {
		confirmed[c] = true
	}
	var open []problem.Anomaly
	for _, a := range anomalies {
		if !a.Confirmable || !confirmed[a.Code] {
			open = append(open, a)
		}
	}
	if len(open) > 0 {
		return problem.Plausibility(anomalies)
	}
	return nil
}

func (s *Service) vinDuplicate(ctx context.Context, db store.DBTX, actor kernel.Actor, vin *string, self uuid.UUID) ([]problem.Anomaly, error) {
	if vin == nil {
		return nil, nil
	}
	members, err := identity.MemberVehicles(ctx, db, actor.AccountID)
	if err != nil {
		return nil, err
	}
	ids := make([]pgtype.UUID, 0, len(members))
	for id := range members {
		ids = append(ids, pgU(id))
	}
	rows, err := store.New(db).VehiclesWithVIN(ctx, store.VehiclesWithVINParams{Vin: pgT(vin), ID: pgU(self), Ids: ids})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	id := uuid.UUID(rows[0].ID.Bytes)
	return []problem.Anomaly{{Code: "VIN_DUPLICATE", Confirmable: true, RelatedID: &id,
		Message: "Diese FIN ist bereits bei „" + rows[0].DisplayName + "“ hinterlegt."}}, nil
}

// Create legt ein Fahrzeug an; der Anlegende wird Eigentümer (I-VE-1).
// Existiert die ID bereits mit gleichem Inhalt, ist das Anlegen idempotent (ADR-006).
func (s *Service) Create(ctx context.Context, actor kernel.Actor, id *uuid.UUID, in Input, confirm []string) (View, bool, error) {
	if in.OwnerTimeZone == nil || in.DefaultCurrency == nil {
		if st, err := s.Settings(ctx, actor.AccountID); err == nil {
			if tz, ok := st["time_zone"].(string); ok && in.OwnerTimeZone == nil {
				in.OwnerTimeZone = &tz
			}
			if c, ok := st["default_currency"].(string); ok && in.DefaultCurrency == nil {
				in.DefaultCurrency = &c
			}
		}
	}
	if in.OdometerRequired == nil {
		t := true
		in.OdometerRequired = &t
	}
	errs, anomalies := in.validate()
	vid := kernel.NewID()
	if id != nil {
		vid = *id
	}
	var out View
	created := true
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := store.New(tx)
		if id != nil {
			existing, err := q.GetVehicleIncludingDeleted(ctx, pgU(vid))
			if err == nil {
				role, _ := identity.RoleOf(ctx, tx, vid, actor.AccountID)
				if role == "" {
					return problem.Conflict("Die ID ist bereits vergeben.")
				}
				v := toView(existing, role)
				if sameInput(v.Input, in) {
					out, created = v, false
					return nil
				}
				return problem.Conflict("Ein Fahrzeug mit dieser ID existiert mit anderem Inhalt.")
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		dup, err := s.vinDuplicate(ctx, tx, actor, in.VIN, vid)
		if err != nil {
			return err
		}
		if err := checkAnomalies(errs, append(anomalies, dup...), confirm); err != nil {
			return err
		}
		p := store.InsertVehicleParams{
			ID: pgU(vid), DisplayName: in.DisplayName, Vin: pgT(in.VIN), LicensePlate: pgT(in.LicensePlate), PlateCountry: pgT(in.PlateCountry),
			Make: pgT(in.Make), Model: pgT(in.Model), Variant: pgT(in.Variant), ModelYear: pgI(in.ModelYear), FirstRegistration: pgD(in.FirstRegistration),
			BodyType: in.BodyType, EngineCode: pgT(in.EngineCode), DisplacementCcm: pgI(in.DisplacementCcm), PowerKw: pgI(in.PowerKw),
			Transmission: pgT(in.Transmission), UsageMeter: in.UsageMeter, EnergyCarriers: in.EnergyCarriers, OdometerRequired: *in.OdometerRequired,
			OwnerTimeZone: deref(in.OwnerTimeZone, "UTC"), DefaultCurrency: deref(in.DefaultCurrency, "EUR"), Note: deref(in.Note, ""),
			Tags: nonNil(in.Tags), Extra: extraJSON(in), Origin: "web", CreatedBy: pgU(actor.AccountID),
		}
		if in.Purchase != nil {
			p.PurchaseDate = pgD(&in.Purchase.Date)
			if in.Purchase.Price != nil {
				p.PurchaseAmount = pgtype.Int8{Int64: in.Purchase.Price.AmountMinor, Valid: true}
				p.PurchaseCurrency = pgtype.Text{String: in.Purchase.Price.Currency, Valid: true}
			}
		}
		row, err := q.InsertVehicle(ctx, p)
		if err != nil {
			return err
		}
		if err := identity.AddMembership(ctx, tx, vid, actor.AccountID, identity.RoleOwner); err != nil {
			return err
		}
		out = toView(row, identity.RoleOwner)
		return audit.Write(ctx, tx, actor, audit.Event{Action: "vehicle.created", VehicleID: &vid, ObjectType: "vehicle", ObjectID: vid})
	})
	return out, created, err
}

func sameInput(a, b Input) bool {
	ja, _ := json.Marshal(normalized(a))
	jb, _ := json.Marshal(normalized(b))
	return string(ja) == string(jb)
}

func deref(s *string, d string) string {
	if s == nil {
		return d
	}
	return *s
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// Get liest ein Fahrzeug mit Rechteprüfung (Leser).
func (s *Service) Get(ctx context.Context, actor kernel.Actor, id uuid.UUID) (View, error) {
	role, err := identity.Authorize(ctx, s.pool, actor, id, identity.RoleViewer)
	if err != nil {
		return View{}, err
	}
	v, err := store.New(s.pool).GetVehicle(ctx, pgU(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return View{}, problem.NotFound()
	}
	return toView(v, role), err
}

// Page ist eine Seite der Fahrzeugliste.
type Page struct {
	Items      []View
	NextCursor *string
}

// List liefert alle Fahrzeuge, an denen das Konto Mitglied ist.
func (s *Service) List(ctx context.Context, actor kernel.Actor, status *string, cursor *string, limit int) (Page, error) {
	members, err := identity.MemberVehicles(ctx, s.pool, actor.AccountID)
	if err != nil {
		return Page{}, err
	}
	ids := make([]pgtype.UUID, 0, len(members))
	for id := range members {
		ids = append(ids, pgU(id))
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	p := store.ListVehiclesParams{Ids: ids, Status: pgT(status), Lim: int32(limit + 1)}
	if cursor != nil {
		name, id, ok := decodeCursor(*cursor)
		if !ok {
			return Page{}, problem.Validation(problem.FieldError{Pointer: "/cursor", Code: "invalid"})
		}
		p.AfterName, p.AfterID = pgtype.Text{String: name, Valid: true}, pgU(id)
	}
	rows, err := store.New(s.pool).ListVehicles(ctx, p)
	if err != nil {
		return Page{}, err
	}
	var page Page
	for i, r := range rows {
		if i == limit {
			c := encodeCursor(rows[i-1].DisplayName, uuid.UUID(rows[i-1].ID.Bytes))
			page.NextCursor = &c
			break
		}
		page.Items = append(page.Items, toView(r, members[uuid.UUID(r.ID.Bytes)]))
	}
	sort.SliceStable(page.Items, func(i, j int) bool { return page.Items[i].DisplayName < page.Items[j].DisplayName })
	return page, nil
}

// Update wendet einen JSON Merge Patch (RFC 7396) an; If-Match ist Pflicht (ADR-012).
func (s *Service) Update(ctx context.Context, actor kernel.Actor, id uuid.UUID, ifMatch string, patch []byte, confirm []string) (View, error) {
	version, err := parseETag(ifMatch)
	if err != nil {
		return View{}, err
	}
	var out View
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		role, err := identity.Authorize(ctx, tx, actor, id, identity.RoleEditor)
		if err != nil {
			return err
		}
		q := store.New(tx)
		cur, err := q.LockVehicle(ctx, pgU(id))
		if errors.Is(err, pgx.ErrNoRows) {
			return problem.NotFound()
		}
		if err != nil {
			return err
		}
		curView := toView(cur, role)
		if int(cur.Version) != version {
			return problem.PreconditionFailed(curView)
		}
		merged, err := applyMergePatch(curView.Input, patch)
		if err != nil {
			return problem.BadRequest("Ungültiger Merge Patch.")
		}
		errs, anomalies := merged.validate()
		if merged.UsageMeter != cur.UsageMeter && s.HasReadings != nil {
			has, err := s.HasReadings(ctx, tx, id)
			if err != nil {
				return err
			}
			if has {
				return problem.Conflict("Die Zählergröße ist nicht mehr änderbar, weil Messpunkte existieren (I-VE-3).")
			}
		}
		if merged.VIN != nil && (cur.Vin.String != *merged.VIN) {
			dup, err := s.vinDuplicate(ctx, tx, actor, merged.VIN, id)
			if err != nil {
				return err
			}
			anomalies = append(anomalies, dup...)
		}
		if err := checkAnomalies(errs, anomalies, confirm); err != nil {
			return err
		}
		p := store.UpdateVehicleParams{
			ID: pgU(id), DisplayName: merged.DisplayName, Vin: pgT(merged.VIN), LicensePlate: pgT(merged.LicensePlate),
			PlateCountry: pgT(merged.PlateCountry), Make: pgT(merged.Make), Model: pgT(merged.Model), Variant: pgT(merged.Variant),
			ModelYear: pgI(merged.ModelYear), FirstRegistration: pgD(merged.FirstRegistration), BodyType: merged.BodyType,
			EngineCode: pgT(merged.EngineCode), DisplacementCcm: pgI(merged.DisplacementCcm), PowerKw: pgI(merged.PowerKw),
			Transmission: pgT(merged.Transmission), UsageMeter: merged.UsageMeter, EnergyCarriers: merged.EnergyCarriers,
			OdometerRequired: merged.OdometerRequired == nil || *merged.OdometerRequired,
			OwnerTimeZone: deref(merged.OwnerTimeZone, cur.OwnerTimeZone), DefaultCurrency: deref(merged.DefaultCurrency, cur.DefaultCurrency),
			Status: cur.Status, SaleDate: cur.SaleDate, SaleAmount: cur.SaleAmount, SaleCurrency: cur.SaleCurrency,
			Note: deref(merged.Note, ""), Tags: nonNil(merged.Tags), Extra: extraJSON(merged), UpdatedBy: pgU(actor.AccountID), Version: cur.Version,
		}
		if merged.Purchase != nil {
			p.PurchaseDate = pgD(&merged.Purchase.Date)
			if merged.Purchase.Price != nil {
				p.PurchaseAmount = pgtype.Int8{Int64: merged.Purchase.Price.AmountMinor, Valid: true}
				p.PurchaseCurrency = pgtype.Text{String: merged.Purchase.Price.Currency, Valid: true}
			}
		}
		row, err := q.UpdateVehicle(ctx, p)
		if err != nil {
			return err
		}
		out = toView(row, role)
		return audit.Write(ctx, tx, actor, audit.Event{Action: "vehicle.updated", VehicleID: &id, ObjectType: "vehicle", ObjectID: id,
			Changes: json.RawMessage(patch)})
	})
	return out, err
}

// ChangeStatus setzt verkauft/archiviert/aktiv (Eigentümer, §2.3).
func (s *Service) ChangeStatus(ctx context.Context, actor kernel.Actor, id uuid.UUID, ifMatch, status string, sale *DateMoney) (View, error) {
	version, err := parseETag(ifMatch)
	if err != nil {
		return View{}, err
	}
	var out View
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		role, err := identity.Authorize(ctx, tx, actor, id, identity.RoleOwner)
		if err != nil {
			return err
		}
		q := store.New(tx)
		cur, err := q.LockVehicle(ctx, pgU(id))
		if err != nil {
			return problem.NotFound()
		}
		if int(cur.Version) != version {
			return problem.PreconditionFailed(toView(cur, role))
		}
		v := toView(cur, role)
		p := store.UpdateVehicleParams{ID: cur.ID, DisplayName: cur.DisplayName, Vin: cur.Vin, LicensePlate: cur.LicensePlate,
			PlateCountry: cur.PlateCountry, Make: cur.Make, Model: cur.Model, Variant: cur.Variant, ModelYear: cur.ModelYear,
			FirstRegistration: cur.FirstRegistration, BodyType: cur.BodyType, EngineCode: cur.EngineCode, DisplacementCcm: cur.DisplacementCcm,
			PowerKw: cur.PowerKw, Transmission: cur.Transmission, UsageMeter: cur.UsageMeter, EnergyCarriers: cur.EnergyCarriers,
			OdometerRequired: cur.OdometerRequired, OwnerTimeZone: cur.OwnerTimeZone, DefaultCurrency: cur.DefaultCurrency, Status: status,
			PurchaseDate: cur.PurchaseDate, PurchaseAmount: cur.PurchaseAmount, PurchaseCurrency: cur.PurchaseCurrency,
			Note: cur.Note, Tags: cur.Tags, Extra: cur.Extra, UpdatedBy: pgU(actor.AccountID), Version: cur.Version}
		switch status {
		case "sold":
			if sale == nil {
				return problem.Validation(problem.FieldError{Pointer: "/sale", Code: "required", Message: "Verkaufsdatum ist Pflicht."})
			}
			p.SaleDate = pgD(&sale.Date)
			if !p.SaleDate.Valid {
				return problem.Validation(problem.FieldError{Pointer: "/sale/date", Code: "date"})
			}
			if cur.PurchaseDate.Valid && p.SaleDate.Time.Before(cur.PurchaseDate.Time) {
				return problem.Validation(problem.FieldError{Pointer: "/sale/date", Code: "before_purchase", Message: "Verkauf vor Kauf (I-VE-4)."})
			}
			if sale.Price != nil {
				p.SaleAmount = pgtype.Int8{Int64: sale.Price.AmountMinor, Valid: true}
				p.SaleCurrency = pgtype.Text{String: sale.Price.Currency, Valid: true}
			}
		case "active", "archived":
		default:
			return problem.Validation(problem.FieldError{Pointer: "/status", Code: "enum"})
		}
		row, err := q.UpdateVehicle(ctx, p)
		if err != nil {
			return err
		}
		out = toView(row, role)
		return audit.Write(ctx, tx, actor, audit.Event{Action: "vehicle.status_changed", VehicleID: &id, ObjectType: "vehicle", ObjectID: id,
			Changes: map[string]any{"status": map[string]string{"old": v.Status, "new": status}}})
	})
	return out, err
}

// Delete löscht ein Fahrzeug weich (Eigentümer, Frist 30 Tage).
func (s *Service) Delete(ctx context.Context, actor kernel.Actor, id uuid.UUID, ifMatch string) error {
	version, err := parseETag(ifMatch)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		role, err := identity.Authorize(ctx, tx, actor, id, identity.RoleOwner)
		if err != nil {
			return err
		}
		q := store.New(tx)
		n, err := q.SoftDeleteVehicle(ctx, store.SoftDeleteVehicleParams{ID: pgU(id), UpdatedBy: pgU(actor.AccountID), Version: int32(version)})
		if err != nil {
			return err
		}
		if n == 0 {
			cur, err := q.GetVehicle(ctx, pgU(id))
			if err != nil {
				return problem.NotFound()
			}
			return problem.PreconditionFailed(toView(cur, role))
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "vehicle.deleted", VehicleID: &id, ObjectType: "vehicle", ObjectID: id})
	})
}

// LoadMeta liest die für andere Module nötigen Angaben; lock sperrt die Zeile
// für serialisierte Änderungen (z. B. Messpunkte eines Fahrzeugs).
func LoadMeta(ctx context.Context, db store.DBTX, id uuid.UUID, lock bool) (Meta, error) {
	q := store.New(db)
	var v store.VehiclesVehicle
	var err error
	if lock {
		v, err = q.LockVehicle(ctx, pgU(id))
	} else {
		v, err = q.GetVehicle(ctx, pgU(id))
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return Meta{}, problem.NotFound()
	}
	if err != nil {
		return Meta{}, err
	}
	m := Meta{ID: id, UsageMeter: v.UsageMeter, OdometerRequired: v.OdometerRequired, OwnerTimeZone: v.OwnerTimeZone, Status: v.Status, DisplayName: v.DisplayName}
	m.DefaultCurrency = v.DefaultCurrency
	m.EnergyCarriers, m.TankCapacityMl = v.EnergyCarriers, map[string]int64{}
	var capEx extra
	_ = json.Unmarshal(v.Extra, &capEx)
	for k, q := range capEx.TankCapacity {
		q := q
		m.TankCapacityMl[k] = canonicalOf(&q)
	}
	m.BatteryWh, m.OilRangeMl, m.OilCapacityMl = canonicalOf(capEx.BatteryUsableCapacity), canonicalOf(capEx.OilDipstickRange), canonicalOf(capEx.OilCapacity)
	if v.SaleDate.Valid {
		t := v.SaleDate.Time
		m.SaleDate = &t
		if v.SaleAmount.Valid {
			m.Sale = &Money{AmountMinor: v.SaleAmount.Int64, Currency: v.SaleCurrency.String}
		}
	}
	if v.PurchaseDate.Valid {
		t := v.PurchaseDate.Time
		m.PurchaseDate = &t
		if v.PurchaseAmount.Valid {
			m.Purchase = &Money{AmountMinor: v.PurchaseAmount.Int64, Currency: v.PurchaseCurrency.String}
		}
	}
	var ex extra
	if json.Unmarshal(v.Extra, &ex) == nil {
		m.EstimatedValue = ex.EstimatedValue
	}
	return m, nil
}

// ETag bildet die Version auf einen starken ETag ab.
func ETag(version int) string { return `"` + strconv.Itoa(version) + `"` }

func parseETag(s string) (int, error) {
	if s == "" {
		return 0, problem.PreconditionRequired()
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, problem.PreconditionFailed(nil)
	}
	return n, nil
}

// ParseETag ist die exportierte Variante für andere Module.
func ParseETag(s string) (int, error) { return parseETag(s) }

func applyMergePatch(cur Input, patch []byte) (Input, error) {
	base, _ := json.Marshal(cur)
	var doc map[string]any
	_ = json.Unmarshal(base, &doc)
	var p map[string]any
	if err := json.Unmarshal(patch, &p); err != nil {
		return Input{}, err
	}
	merge(doc, p)
	b, _ := json.Marshal(doc)
	var out Input
	err := json.Unmarshal(b, &out)
	return out, err
}

func merge(dst, patch map[string]any) {
	for k, v := range patch {
		if v == nil {
			delete(dst, k)
			continue
		}
		if pm, ok := v.(map[string]any); ok {
			if dm, ok := dst[k].(map[string]any); ok {
				merge(dm, pm)
				continue
			}
		}
		dst[k] = v
	}
}

func encodeCursor(name string, id uuid.UUID) string {
	b, _ := json.Marshal([]string{name, id.String()})
	return base64URL(b)
}

func decodeCursor(c string) (string, uuid.UUID, bool) {
	b, err := unbase64URL(c)
	if err != nil {
		return "", uuid.Nil, false
	}
	var parts []string
	if json.Unmarshal(b, &parts) != nil || len(parts) != 2 {
		return "", uuid.Nil, false
	}
	id, err := uuid.Parse(parts[1])
	return parts[0], id, err == nil
}

// normalized gleicht Vorgabewerte an, damit gespeicherte und gesendete Eingaben
// vergleichbar sind (idempotentes Anlegen).
func normalized(in Input) Input {
	empty := ""
	if in.Note == nil {
		in.Note = &empty
	}
	if len(in.Tags) == 0 {
		in.Tags = nil
	}
	if len(in.EnergyCarriers) == 0 {
		in.EnergyCarriers = nil
	}
	if len(in.TankCapacity) == 0 {
		in.TankCapacity = nil
	}
	if len(in.DisplayUnits) == 0 {
		in.DisplayUnits = nil
	}
	if len(in.CustomFields) == 0 {
		in.CustomFields = nil
	}
	return in
}
