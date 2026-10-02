package documents

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/sh1su/vector/backend/internal/documents/store"
	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/audit"
	"github.com/sh1su/vector/backend/internal/platform/mergepatch"
	pg "github.com/sh1su/vector/backend/internal/platform/pgconv"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

func itoa(i int) string { return strconv.Itoa(i) }

// Dokumenttypen mit Default-nature (§2.1).
var docTypes = map[string]string{
	"maintenance_manual": "specification", "owner_manual": "specification", "technical_doc": "specification",
	"service_book": "record", "invoice": "record", "workshop_report": "record", "inspection_report": "record",
	"registration": "other", "insurance": "other", "other": "other",
}

// DocInput sind die Felder eines Dokuments (JSON wie API).
type DocInput struct {
	DocType      string      `json:"doc_type"`
	Nature       string      `json:"nature,omitempty"`
	Title        string      `json:"title"`
	DocumentDate *string     `json:"document_date,omitempty"`
	Issuer       *string     `json:"issuer,omitempty"`
	Language     *string     `json:"language,omitempty"`
	FileIDs      []uuid.UUID `json:"file_ids"`
	Note         string      `json:"note"`
	Tags         []string    `json:"tags"`
}

// DocView entspricht dem API-Schema Document.
type DocView struct {
	kernel.EntityMeta
	DocInput
}

func (in *DocInput) validate() (*time.Time, error) {
	var errs []problem.FieldError
	add := func(p, c, m string) { errs = append(errs, problem.FieldError{Pointer: p, Code: c, Message: m}) }
	in.Title = strings.TrimSpace(in.Title)
	if n := len([]rune(in.Title)); n < 1 || n > 200 {
		add("/title", "length", "1–200 Zeichen")
	}
	def, ok := docTypes[in.DocType]
	if !ok {
		add("/doc_type", "enum", "")
	}
	if in.Nature == "" {
		in.Nature = def
	} else if in.Nature != "specification" && in.Nature != "record" && in.Nature != "other" {
		add("/nature", "enum", "")
	}
	var date *time.Time
	if in.DocumentDate != nil {
		d, err := kernel.ParseDate(*in.DocumentDate)
		if err != nil {
			add("/document_date", "date", "")
		}
		date = &d
	}
	if len(in.FileIDs) == 0 { // I-DO-4
		add("/file_ids", "required", "Ein Dokument braucht mindestens eine Datei.")
	}
	if in.Language != nil && (len(*in.Language) != 2 || strings.ToLower(*in.Language) != *in.Language) {
		add("/language", "pattern", "ISO 639-1")
	}
	if len(in.Tags) > 20 {
		add("/tags", "max_items", "")
	}
	if in.Tags == nil {
		in.Tags = []string{}
	}
	if len(errs) > 0 {
		return nil, problem.Validation(errs...)
	}
	return date, nil
}

// checkFiles prüft, dass alle Dateien zum Fahrzeug gehören (I-DO-1).
func checkFiles(ctx context.Context, q *store.Queries, vehicleID uuid.UUID, ids []uuid.UUID) error {
	for i, id := range ids {
		f, err := q.GetFile(ctx, pg.U(id))
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && (pg.ID(f.VehicleID) != vehicleID || f.DeletedAt.Valid)) {
			return problem.Validation(problem.FieldError{Pointer: "/file_ids/" + itoa(i), Code: "not_found", Message: "Datei gehört nicht zu diesem Fahrzeug (I-DO-1)."})
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func writePages(ctx context.Context, q *store.Queries, docID uuid.UUID, ids []uuid.UUID) error {
	if err := q.DeleteDocumentFiles(ctx, pg.U(docID)); err != nil {
		return err
	}
	for i, id := range ids {
		if err := q.InsertDocumentFile(ctx, store.InsertDocumentFileParams{DocumentID: pg.U(docID), Position: int32(i), FileID: pg.U(id)}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) docViews(ctx context.Context, db store.DBTX, rows []store.DocumentsDocument) ([]DocView, error) {
	out := []DocView{}
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]pgtype.UUID, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	pages, err := store.New(db).DocumentFiles(ctx, ids)
	if err != nil {
		return nil, err
	}
	byDoc := map[uuid.UUID][]uuid.UUID{}
	for _, p := range pages {
		byDoc[pg.ID(p.DocumentID)] = append(byDoc[pg.ID(p.DocumentID)], pg.ID(p.FileID))
	}
	for _, r := range rows {
		upd, rec, by := r.UpdatedAt.Time, r.RecordedAt.Time, pg.ID(r.UpdatedBy)
		tags := r.Tags
		if tags == nil {
			tags = []string{}
		}
		var date *string
		if d := pg.DateP(r.DocumentDate); d != nil {
			s := kernel.FormatDate(*d)
			date = &s
		}
		out = append(out, DocView{
			EntityMeta: kernel.EntityMeta{ID: pg.ID(r.ID), Version: int(r.Version), VehicleID: pg.ID(r.VehicleID), CreatedAt: r.CreatedAt.Time,
				CreatedBy: pg.ID(r.CreatedBy), UpdatedAt: &upd, UpdatedBy: &by, RecordedAt: &rec, Origin: r.Origin},
			DocInput: DocInput{DocType: r.DocType, Nature: r.Nature, Title: r.Title, DocumentDate: date, Issuer: pg.TP(r.Issuer), Language: pg.TP(r.Language),
				FileIDs: byDoc[pg.ID(r.ID)], Note: r.Note, Tags: tags},
		})
	}
	return out, nil
}

// CreateDocument legt ein Dokument der Fahrzeugakte an.
func (s *Service) CreateDocument(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, id *uuid.UUID, in DocInput) (DocView, bool, error) {
	date, err := in.validate()
	if err != nil {
		return DocView{}, false, err
	}
	var out DocView
	created := true
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := identity.Authorize(ctx, tx, actor, vehicleID, identity.RoleEditor); err != nil {
			return err
		}
		q := store.New(tx)
		did := kernel.NewID()
		if id != nil {
			did = *id
			if ex, err := q.GetDocument(ctx, pg.U(did)); err == nil {
				vs, err := s.docViews(ctx, tx, []store.DocumentsDocument{ex})
				if err != nil {
					return err
				}
				if vs[0].VehicleID == vehicleID && vs[0].Title == in.Title && vs[0].DocType == in.DocType {
					out, created = vs[0], false
					return nil
				}
				return problem.Conflict("Ein Dokument mit dieser ID existiert mit anderem Inhalt.")
			}
		}
		if err := checkFiles(ctx, q, vehicleID, in.FileIDs); err != nil {
			return err
		}
		row, err := q.InsertDocument(ctx, store.InsertDocumentParams{ID: pg.U(did), VehicleID: pg.U(vehicleID), DocType: in.DocType, Nature: in.Nature,
			Title: in.Title, DocumentDate: pg.DP(date), Issuer: pg.T(in.Issuer), Language: pg.T(in.Language), Note: in.Note, Tags: in.Tags,
			Origin: kernel.OriginOf(actor), CreatedBy: pg.U(actor.AccountID)})
		if err != nil {
			return err
		}
		if err := writePages(ctx, q, did, in.FileIDs); err != nil {
			return err
		}
		vs, err := s.docViews(ctx, tx, []store.DocumentsDocument{row})
		if err != nil {
			return err
		}
		out = vs[0]
		return audit.Write(ctx, tx, actor, audit.Event{Action: "documents.document_created", VehicleID: &vehicleID, ObjectType: "document", ObjectID: did})
	})
	return out, created, err
}

func loadDoc(ctx context.Context, db store.DBTX, actor kernel.Actor, vehicleID, id uuid.UUID, need string) (store.DocumentsDocument, error) {
	d, err := store.New(db).GetDocument(ctx, pg.U(id))
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (pg.ID(d.VehicleID) != vehicleID || d.DeletedAt.Valid)) {
		return d, problem.NotFound()
	}
	if err != nil {
		return d, err
	}
	_, err = identity.Authorize(ctx, db, actor, pg.ID(d.VehicleID), need)
	return d, err
}

// GetDocument liest ein Dokument.
func (s *Service) GetDocument(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID) (DocView, error) {
	d, err := loadDoc(ctx, s.pool, actor, vehicleID, id, identity.RoleViewer)
	if err != nil {
		return DocView{}, err
	}
	vs, err := s.docViews(ctx, s.pool, []store.DocumentsDocument{d})
	if err != nil {
		return DocView{}, err
	}
	return vs[0], nil
}

// UpdateDocument ändert ein Dokument (Merge Patch, If-Match).
func (s *Service) UpdateDocument(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID, ifMatch string, patch []byte) (DocView, error) {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return DocView{}, err
	}
	var out DocView
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := loadDoc(ctx, tx, actor, vehicleID, id, identity.RoleEditor)
		if err != nil {
			return err
		}
		vs, err := s.docViews(ctx, tx, []store.DocumentsDocument{cur})
		if err != nil {
			return err
		}
		if int(cur.Version) != version {
			return problem.PreconditionFailed(vs[0])
		}
		var in DocInput
		if err := mergepatch.Apply(vs[0].DocInput, patch, &in); err != nil {
			return problem.BadRequest("Ungültiger Merge Patch.")
		}
		if mergepatch.Has(patch, "doc_type") && !mergepatch.Has(patch, "nature") {
			in.Nature = ""
		}
		date, err := in.validate()
		if err != nil {
			return err
		}
		q := store.New(tx)
		if err := checkFiles(ctx, q, vehicleID, in.FileIDs); err != nil {
			return err
		}
		row, err := q.UpdateDocument(ctx, store.UpdateDocumentParams{ID: cur.ID, DocType: in.DocType, Nature: in.Nature, Title: in.Title,
			DocumentDate: pg.DP(date), Issuer: pg.T(in.Issuer), Language: pg.T(in.Language), Note: in.Note, Tags: in.Tags,
			UpdatedBy: pg.U(actor.AccountID), Version: cur.Version})
		if errors.Is(err, pgx.ErrNoRows) {
			return problem.PreconditionFailed(nil)
		}
		if err != nil {
			return err
		}
		if err := writePages(ctx, q, id, in.FileIDs); err != nil {
			return err
		}
		nv, err := s.docViews(ctx, tx, []store.DocumentsDocument{row})
		if err != nil {
			return err
		}
		out = nv[0]
		return audit.Write(ctx, tx, actor, audit.Event{Action: "documents.document_updated", VehicleID: &vehicleID, ObjectType: "document", ObjectID: id,
			Changes: json.RawMessage(patch)})
	})
	return out, err
}

// DeleteDocument löscht weich (DO-06); die Dateien bleiben erhalten.
func (s *Service) DeleteDocument(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID, ifMatch string) error {
	version, err := vehicles.ParseETag(ifMatch)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := loadDoc(ctx, tx, actor, vehicleID, id, identity.RoleEditor)
		if err != nil {
			return err
		}
		if int(cur.Version) != version {
			return problem.PreconditionFailed(nil)
		}
		if n, err := store.New(tx).SoftDeleteDocument(ctx, store.SoftDeleteDocumentParams{ID: cur.ID, UpdatedBy: pg.U(actor.AccountID), Version: cur.Version}); err != nil || n == 0 {
			if err != nil {
				return err
			}
			return problem.PreconditionFailed(nil)
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "documents.document_deleted", VehicleID: &vehicleID, ObjectType: "document", ObjectID: id})
	})
}

// DocFilter filtert die Fahrzeugakte.
type DocFilter struct {
	DocType, Nature, Q *string
	Limit              int
}

// ListDocuments listet Dokumente, neueste zuerst; q durchsucht Titel, Aussteller, Notiz und Schlagwörter.
func (s *Service) ListDocuments(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, f DocFilter) ([]DocView, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return nil, err
	}
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 200
	}
	if f.Q != nil && strings.TrimSpace(*f.Q) == "" {
		f.Q = nil
	}
	rows, err := store.New(s.pool).ListDocuments(ctx, store.ListDocumentsParams{VehicleID: pg.U(vehicleID), DocType: pg.T(f.DocType), Nature: pg.T(f.Nature),
		Q: pg.T(f.Q), Lim: int32(f.Limit)})
	if err != nil {
		return nil, err
	}
	return s.docViews(ctx, s.pool, rows)
}

// ---------- Verknüpfungen ----------

// AttachmentView entspricht dem API-Schema Attachment.
type AttachmentView struct {
	ID         uuid.UUID  `json:"id"`
	FileID     *uuid.UUID `json:"file_id"`
	DocumentID *uuid.UUID `json:"document_id"`
	TargetType string     `json:"target_type"`
	TargetID   uuid.UUID  `json:"target_id"`
	Role       string     `json:"role"`
	Page       *int       `json:"page"`
}

func attachmentView(a store.DocumentsAttachment) AttachmentView {
	return AttachmentView{ID: pg.ID(a.ID), FileID: pg.IDP(a.FileID), DocumentID: pg.IDP(a.DocumentID), TargetType: a.TargetType,
		TargetID: pg.ID(a.TargetID), Role: a.Role, Page: pg.I4P(a.Page)}
}

// targetTables ordnet Verknüpfungsziele ihren Tabellen zu; nur für die
// Fahrzeugprüfung I-DO-1 (lesend). Fehlt ein Modul noch, ist das Ziel nicht verfügbar.
var targetTables = map[string]string{
	"odometer_reading": "odometer.reading", "service_entry": "service.entry", "cost_entry": "costs.entry",
	"trip": "trips.trip", "maintenance_item": "maintenance.item",
}

// Attach hängt eine Datei oder ein Dokument an einen Eintrag desselben Fahrzeugs.
func (s *Service) Attach(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, in AttachmentView) (AttachmentView, error) {
	if (in.FileID == nil) == (in.DocumentID == nil) {
		return AttachmentView{}, problem.Validation(problem.FieldError{Pointer: "/file_id", Code: "one_of", Message: "Genau eine Datei oder ein Dokument angeben."})
	}
	in.Role = strings.TrimSpace(in.Role)
	if in.Role == "" || len(in.Role) > 40 {
		return AttachmentView{}, problem.Validation(problem.FieldError{Pointer: "/role", Code: "length"})
	}
	table, ok := targetTables[in.TargetType]
	if !ok {
		return AttachmentView{}, problem.Validation(problem.FieldError{Pointer: "/target_type", Code: "unavailable", Message: "Dieses Ziel ist noch nicht verfügbar."})
	}
	var out AttachmentView
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := identity.Authorize(ctx, tx, actor, vehicleID, identity.RoleEditor); err != nil {
			return err
		}
		var target uuid.UUID
		err := tx.QueryRow(ctx, "SELECT vehicle_id FROM "+table+" WHERE id = $1", in.TargetID).Scan(&target)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && target != vehicleID) {
			return problem.Validation(problem.FieldError{Pointer: "/target_id", Code: "vehicle", Message: "Ziel gehört nicht zu diesem Fahrzeug (I-DO-1)."})
		}
		if err != nil {
			return err
		}
		q := store.New(tx)
		if in.FileID != nil {
			if err := checkFiles(ctx, q, vehicleID, []uuid.UUID{*in.FileID}); err != nil {
				return err
			}
		} else {
			d, err := q.GetDocument(ctx, pg.U(*in.DocumentID))
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && (pg.ID(d.VehicleID) != vehicleID || d.DeletedAt.Valid)) {
				return problem.Validation(problem.FieldError{Pointer: "/document_id", Code: "vehicle", Message: "Dokument gehört nicht zu diesem Fahrzeug (I-DO-1)."})
			}
			if err != nil {
				return err
			}
		}
		id := kernel.NewID()
		row, err := q.InsertAttachment(ctx, store.InsertAttachmentParams{ID: pg.U(id), VehicleID: pg.U(vehicleID), FileID: pg.UP(in.FileID),
			DocumentID: pg.UP(in.DocumentID), TargetType: in.TargetType, TargetID: pg.U(in.TargetID), Role: in.Role, Page: pg.I4(in.Page),
			CreatedBy: pg.U(actor.AccountID)})
		if err != nil {
			return err
		}
		out = attachmentView(row)
		return audit.Write(ctx, tx, actor, audit.Event{Action: "documents.attached", VehicleID: &vehicleID, ObjectType: "attachment", ObjectID: id,
			Changes: map[string]any{"target_type": in.TargetType, "target_id": in.TargetID}})
	})
	return out, err
}

// ListAttachments listet Verknüpfungen (optional je Ziel).
func (s *Service) ListAttachments(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, targetType *string, targetID *uuid.UUID) ([]AttachmentView, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return nil, err
	}
	rows, err := store.New(s.pool).ListAttachments(ctx, store.ListAttachmentsParams{VehicleID: pg.U(vehicleID), TargetType: pg.T(targetType), TargetID: pg.UP(targetID)})
	if err != nil {
		return nil, err
	}
	out := []AttachmentView{}
	for _, r := range rows {
		out = append(out, attachmentView(r))
	}
	return out, nil
}

// Detach löst eine Verknüpfung (Soft-Delete).
func (s *Service) Detach(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := store.New(tx)
		a, err := q.GetAttachment(ctx, pg.U(id))
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && (pg.ID(a.VehicleID) != vehicleID || a.DeletedAt.Valid)) {
			return problem.NotFound()
		}
		if err != nil {
			return err
		}
		if _, err := identity.Authorize(ctx, tx, actor, vehicleID, identity.RoleEditor); err != nil {
			return err
		}
		if _, err := q.SoftDeleteAttachment(ctx, a.ID); err != nil {
			return err
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "documents.detached", VehicleID: &vehicleID, ObjectType: "attachment", ObjectID: id})
	})
}

// ---------- Fahrzeugbilder ----------

// ImageView entspricht dem API-Schema VehicleImage.
type ImageView struct {
	ID      uuid.UUID `json:"id"`
	FileID  uuid.UUID `json:"file_id"`
	Primary bool      `json:"primary"`
}

// ListImages listet die Fahrzeugbilder, Hauptbild zuerst.
func (s *Service) ListImages(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID) ([]ImageView, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return nil, err
	}
	rows, err := store.New(s.pool).ListImages(ctx, pg.U(vehicleID))
	if err != nil {
		return nil, err
	}
	out := []ImageView{}
	for _, r := range rows {
		out = append(out, ImageView{ID: pg.ID(r.ID), FileID: pg.ID(r.FileID), Primary: r.IsPrimary})
	}
	return out, nil
}

// AddImage verwendet eine hochgeladene Bilddatei als Fahrzeugbild. Das erste Bild
// oder primary=true wird Hauptbild.
func (s *Service) AddImage(ctx context.Context, actor kernel.Actor, vehicleID, fileID uuid.UUID, primary bool) (ImageView, error) {
	var out ImageView
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		f, _, err := s.loadFile(ctx, tx, actor, vehicleID, fileID, identity.RoleEditor)
		if err != nil {
			if pe, ok := err.(*problem.Error); ok && pe.Status == 404 {
				return problem.Validation(problem.FieldError{Pointer: "/file_id", Code: "not_found", Message: "Datei gehört nicht zu diesem Fahrzeug."})
			}
			return err
		}
		if len(f.Derivatives) == 0 {
			return problem.Validation(problem.FieldError{Pointer: "/file_id", Code: "not_image", Message: "Als Fahrzeugbild eignen sich JPEG, PNG oder WebP."})
		}
		q := store.New(tx)
		existing, err := q.ListImages(ctx, pg.U(vehicleID))
		if err != nil {
			return err
		}
		if len(existing) == 0 {
			primary = true
		}
		if primary {
			if err := q.ClearPrimaryImage(ctx, pg.U(vehicleID)); err != nil {
				return err
			}
		}
		row, err := q.UpsertImage(ctx, store.UpsertImageParams{ID: pg.U(kernel.NewID()), VehicleID: pg.U(vehicleID), FileID: f.ID, IsPrimary: primary,
			CreatedBy: pg.U(actor.AccountID)})
		if err != nil {
			return err
		}
		out = ImageView{ID: pg.ID(row.ID), FileID: pg.ID(row.FileID), Primary: row.IsPrimary}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "vehicle.image_added", VehicleID: &vehicleID, ObjectType: "vehicle_image", ObjectID: out.ID})
	})
	return out, err
}

// RemoveImage entfernt ein Fahrzeugbild; die Datei bleibt erhalten.
func (s *Service) RemoveImage(ctx context.Context, actor kernel.Actor, vehicleID, imageID uuid.UUID) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := store.New(tx)
		img, err := q.GetImage(ctx, pg.U(imageID))
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && pg.ID(img.VehicleID) != vehicleID) {
			return problem.NotFound()
		}
		if err != nil {
			return err
		}
		if _, err := identity.Authorize(ctx, tx, actor, vehicleID, identity.RoleEditor); err != nil {
			return err
		}
		if _, err := q.DeleteImage(ctx, img.ID); err != nil {
			return err
		}
		if err := q.PromoteFirstImage(ctx, pg.U(vehicleID)); err != nil {
			return err
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "vehicle.image_removed", VehicleID: &vehicleID, ObjectType: "vehicle_image", ObjectID: imageID})
	})
}
