// Package documents verwaltet Dateien (ADR-017/018), die digitale Fahrzeugakte,
// Verknüpfungen mit Einträgen und Fahrzeugbilder (docs/phase-2/10-domaene-documents.md).
package documents

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sh1su/vector/backend/internal/documents/store"
	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/audit"
	"github.com/sh1su/vector/backend/internal/platform/cursor"
	pg "github.com/sh1su/vector/backend/internal/platform/pgconv"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/platform/storage"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

// Service ist der Application Service des Moduls Documents.
type Service struct {
	pool     *pgxpool.Pool
	store    storage.Local
	MaxBytes int64
}

func NewService(pool *pgxpool.Pool, st storage.Local, maxBytes int64) *Service {
	if maxBytes <= 0 {
		maxBytes = 25 << 20
	}
	return &Service{pool: pool, store: st, MaxBytes: maxBytes}
}

// Derivative entspricht dem API-Schema FileDerivative.
type DerivativeView struct {
	Kind      string `json:"kind"`
	MediaType string `json:"media_type"`
}

// FileView entspricht dem API-Schema FileMeta.
type FileView struct {
	ID               uuid.UUID        `json:"id"`
	VehicleID        uuid.UUID        `json:"vehicle_id"`
	OriginalName     string           `json:"original_name"`
	MediaType        string           `json:"media_type"`
	SizeBytes        int64            `json:"size_bytes"`
	SHA256           string           `json:"sha256"`
	ReceivedAt       time.Time        `json:"received_at"`
	CapturedAtClient *time.Time       `json:"captured_at_client"`
	CaptureSource    *string          `json:"capture_source"`
	HasLocation      bool             `json:"has_location"`
	Derivatives      []DerivativeView `json:"derivatives"`
	SupersedesID     *uuid.UUID       `json:"supersedes_id"`
	DuplicateOf      *uuid.UUID       `json:"duplicate_of"`
}

func fileView(f store.DocumentsFile) FileView {
	v := FileView{ID: pg.ID(f.ID), VehicleID: pg.ID(f.VehicleID), OriginalName: f.OriginalName, MediaType: f.MediaType, SizeBytes: f.SizeBytes,
		SHA256: f.Sha256, ReceivedAt: f.ReceivedAt.Time, CapturedAtClient: pg.TSPtr(f.CapturedAtClient), CaptureSource: pg.TP(f.CaptureSource),
		HasLocation: f.HasLocation, SupersedesID: pg.IDP(f.SupersedesID), Derivatives: []DerivativeView{}}
	for _, d := range f.Derivatives {
		v.Derivatives = append(v.Derivatives, DerivativeView{Kind: d, MediaType: "image/jpeg"})
	}
	return v
}

// UploadInput sind die Metadaten eines Uploads.
type UploadInput struct {
	ID               *uuid.UUID
	Name             string
	CaptureSource    string
	CapturedAtClient *time.Time
	AllowDuplicate   bool
	ClientSHA256     string
	Supersedes       *store.DocumentsFile
	Reason           string
}

// headWriter merkt sich die ersten Bytes für die Inhaltserkennung.
type headWriter struct{ buf []byte }

func (h *headWriter) Write(p []byte) (int, error) {
	if n := 512 - len(h.buf); n > 0 {
		if len(p) < n {
			n = len(p)
		}
		h.buf = append(h.buf, p[:n]...)
	}
	return len(p), nil
}

var errTooLarge = errors.New("too large")

type limited struct {
	r   io.Reader
	n   int64
	max int64
}

func (l *limited) Read(p []byte) (int, error) {
	n, err := l.r.Read(p)
	l.n += int64(n)
	if l.n > l.max {
		return n, errTooLarge
	}
	return n, err
}

func cleanName(n string) string {
	n = filepath.Base(strings.ReplaceAll(n, "\\", "/"))
	n = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, n)
	if n == "" || n == "." || n == "/" {
		n = "datei"
	}
	for len(n) > 255 || !utf8.ValidString(n) {
		n = n[:len(n)-1]
	}
	return n
}

// Upload nimmt eine Datei an: Größenlimit, Inhaltserkennung, SHA-256, Ableitungen
// ohne EXIF, Dubletten-Hinweis (DO-02). created=false: vorhandene Datei (duplicate_of).
func (s *Service) Upload(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, in UploadInput, r io.Reader) (FileView, bool, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleEditor); err != nil {
		return FileView{}, false, err
	}
	if _, err := vehicles.LoadMeta(ctx, s.pool, vehicleID, false); err != nil {
		return FileView{}, false, err
	}
	fid := kernel.NewID()
	if in.ID != nil {
		fid = *in.ID
		if ex, err := store.New(s.pool).GetFile(ctx, pg.U(fid)); err == nil {
			if pg.ID(ex.VehicleID) == vehicleID {
				return fileView(ex), false, nil // idempotente Wiederholung (ADR-021)
			}
			return FileView{}, false, problem.Conflict("Die ID ist bereits vergeben.")
		}
	}
	key := fid.String()
	h := sha256.New()
	head := &headWriter{}
	lr := &limited{r: r, max: s.MaxBytes}
	size, err := s.store.Put(key, io.TeeReader(lr, io.MultiWriter(h, head)))
	if err != nil {
		if errors.Is(err, errTooLarge) {
			return FileView{}, false, problem.Validation(problem.FieldError{Pointer: "/file", Code: "too_large",
				Message: "Die Datei ist größer als erlaubt (" + itoa(int(s.MaxBytes>>20)) + " MB)."})
		}
		return FileView{}, false, err
	}
	fail := func(e error) (FileView, bool, error) { _ = s.store.Delete(key); return FileView{}, false, e }
	if size == 0 {
		return fail(problem.Validation(problem.FieldError{Pointer: "/file", Code: "empty", Message: "Die Datei ist leer."}))
	}
	mt := DetectMediaType(head.buf)
	if mt == "" {
		return fail(problem.Validation(problem.FieldError{Pointer: "/file", Code: "media_type",
			Message: "Dateityp nicht erlaubt. Erlaubt sind JPEG, PNG, WebP, HEIC, PDF und Text (erkannt am Inhalt, nicht an der Endung)."}))
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if in.ClientSHA256 != "" && !strings.EqualFold(in.ClientSHA256, sum) {
		return fail(problem.Validation(problem.FieldError{Pointer: "/client_sha256", Code: "mismatch", Message: "Prüfsumme weicht ab; Übertragung fehlerhaft."}))
	}
	q := store.New(s.pool)
	if !in.AllowDuplicate && in.Supersedes == nil {
		if ex, err := q.FileBySha(ctx, store.FileByShaParams{VehicleID: pg.U(vehicleID), Sha256: sum}); err == nil {
			_ = s.store.Delete(key)
			v := fileView(ex)
			id := v.ID
			v.DuplicateOf = &id
			return v, false, nil
		}
	}
	var derivs []Derivative
	var hasGPS bool
	if strings.HasPrefix(mt, "image/") {
		f, err := s.store.Open(key)
		if err != nil {
			return fail(err)
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return fail(err)
		}
		derivs, hasGPS = MakeDerivatives(data, mt)
	}
	kinds := []string{}
	for _, d := range derivs {
		if _, err := s.store.Put(key+"."+d.Kind+".jpg", bytes.NewReader(d.Data)); err != nil {
			return fail(err)
		}
		kinds = append(kinds, d.Kind)
	}
	var src *string
	if in.CaptureSource != "" {
		cs := in.CaptureSource
		src = &cs
	}
	var out FileView
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		p := store.InsertFileParams{ID: pg.U(fid), VehicleID: pg.U(vehicleID), StorageKey: key, OriginalName: cleanName(in.Name), MediaType: mt,
			SizeBytes: size, Sha256: sum, CapturedAtClient: pg.TSP(in.CapturedAtClient), CaptureSource: pg.T(src), Derivatives: kinds,
			HasLocation: hasGPS, CreatedBy: pg.U(actor.AccountID)}
		action := "documents.file_uploaded"
		if in.Supersedes != nil {
			p.SupersedesID, p.ReplaceReason = in.Supersedes.ID, pg.T(&in.Reason)
			action = "documents.file_replaced"
		}
		row, err := store.New(tx).InsertFile(ctx, p)
		if err != nil {
			return err
		}
		if in.Supersedes != nil {
			q := store.New(tx)
			o, n := in.Supersedes.ID, row.ID
			if err := q.RepointDocumentFiles(ctx, store.RepointDocumentFilesParams{OldID: o, NewID: n}); err != nil {
				return err
			}
			if err := q.RepointAttachments(ctx, store.RepointAttachmentsParams{OldID: o, NewID: n}); err != nil {
				return err
			}
			if err := q.RepointImages(ctx, store.RepointImagesParams{OldID: o, NewID: n}); err != nil {
				return err
			}
		}
		out = fileView(row)
		return audit.Write(ctx, tx, actor, audit.Event{Action: action, VehicleID: &vehicleID, ObjectType: "file", ObjectID: fid,
			Changes: map[string]any{"sha256": sum, "size": size, "media_type": mt}, Reason: in.Reason})
	})
	if err != nil {
		return fail(err)
	}
	return out, true, nil
}

func (s *Service) loadFile(ctx context.Context, db store.DBTX, actor kernel.Actor, vehicleID, id uuid.UUID, need string) (store.DocumentsFile, string, error) {
	f, err := store.New(db).GetFile(ctx, pg.U(id))
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (pg.ID(f.VehicleID) != vehicleID || f.DeletedAt.Valid)) {
		return f, "", problem.NotFound()
	}
	if err != nil {
		return f, "", err
	}
	role, err := identity.Authorize(ctx, db, actor, pg.ID(f.VehicleID), need)
	return f, role, err
}

// GetFile liest Metadaten.
func (s *Service) GetFile(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID) (FileView, error) {
	f, _, err := s.loadFile(ctx, s.pool, actor, vehicleID, id, identity.RoleViewer)
	return fileView(f), err
}

// ListFiles listet alle Dateien des Fahrzeugs, neueste zuerst.
func (s *Service) ListFiles(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, cur *string, limit int) ([]FileView, *string, error) {
	if _, err := identity.Authorize(ctx, s.pool, actor, vehicleID, identity.RoleViewer); err != nil {
		return nil, nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	p := store.ListFilesParams{VehicleID: pg.U(vehicleID), Lim: int32(limit + 1)}
	if cur != nil {
		parts, ok := cursor.Decode(*cur, 2)
		if !ok {
			return nil, nil, problem.Validation(problem.FieldError{Pointer: "/cursor", Code: "invalid"})
		}
		ts, e1 := time.Parse(time.RFC3339Nano, parts[0])
		id, e2 := uuid.Parse(parts[1])
		if e1 != nil || e2 != nil {
			return nil, nil, problem.Validation(problem.FieldError{Pointer: "/cursor", Code: "invalid"})
		}
		p.BeforeTs, p.BeforeID = pg.TS(ts), pg.U(id)
	}
	rows, err := store.New(s.pool).ListFiles(ctx, p)
	if err != nil {
		return nil, nil, err
	}
	var next *string
	if len(rows) > limit {
		last := rows[limit-1]
		c := cursor.Encode(last.ReceivedAt.Time.UTC().Format(time.RFC3339Nano), pg.ID(last.ID).String())
		next, rows = &c, rows[:limit]
	}
	out := []FileView{}
	for _, r := range rows {
		out = append(out, fileView(r))
	}
	return out, next, nil
}

// DeleteFile löscht weich, solange nichts mehr auf die Datei verweist (DO-06).
func (s *Service) DeleteFile(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		f, _, err := s.loadFile(ctx, tx, actor, vehicleID, id, identity.RoleEditor)
		if err != nil {
			return err
		}
		q := store.New(tx)
		refs, err := q.FileReferences(ctx, f.ID)
		if err != nil {
			return err
		}
		if refs > 0 {
			return problem.Conflict("Die Datei wird noch von einem Dokument, einem Eintrag oder als Fahrzeugbild verwendet.")
		}
		if _, err := q.SoftDeleteFile(ctx, f.ID); err != nil {
			return err
		}
		return audit.Write(ctx, tx, actor, audit.Event{Action: "documents.file_deleted", VehicleID: &vehicleID, ObjectType: "file", ObjectID: id})
	})
}

// Replace legt eine neue Fassung an (I-DO-2); Verweise zeigen danach auf die neue Datei.
func (s *Service) Replace(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID, reason, name string, r io.Reader) (FileView, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return FileView{}, problem.Validation(problem.FieldError{Pointer: "/reason", Code: "required", Message: "Begründung fehlt."})
	}
	old, _, err := s.loadFile(ctx, s.pool, actor, vehicleID, id, identity.RoleEditor)
	if err != nil {
		return FileView{}, err
	}
	v, _, err := s.Upload(ctx, actor, vehicleID, UploadInput{Name: name, Supersedes: &old, Reason: reason, CaptureSource: "upload"}, r)
	return v, err
}

// Content ist ein auszuliefernder Inhalt.
type Content struct {
	Reader    io.ReadCloser
	Size      int64
	MediaType string
	Name      string
	Inline    bool
}

// Open liefert einen Inhalt nach Rechteprüfung (ADR-017/018):
// kind = content (bereinigte Fassung bei Bildern), thumbnail/preview (JPEG ohne EXIF),
// original (unverändert, nur Bearbeiter).
func (s *Service) Open(ctx context.Context, actor kernel.Actor, vehicleID, id uuid.UUID, kind string) (Content, error) {
	need := identity.RoleViewer
	if kind == "original" {
		need = identity.RoleEditor
	}
	f, role, err := s.loadFile(ctx, s.pool, actor, vehicleID, id, need)
	if err != nil {
		return Content{}, err
	}
	has := func(k string) bool {
		for _, d := range f.Derivatives {
			if d == k {
				return true
			}
		}
		return false
	}
	key, mt, name, inline := f.StorageKey, f.MediaType, f.OriginalName, false
	switch kind {
	case "thumbnail", "preview":
		if !has(kind) {
			return Content{}, problem.Validation(problem.FieldError{Pointer: "/size", Code: "no_preview", Message: "Für diesen Dateityp gibt es keine Vorschau."})
		}
		key, mt, inline = f.StorageKey+"."+kind+".jpg", "image/jpeg", true
		name = strings.TrimSuffix(name, filepath.Ext(name)) + ".jpg"
	case "content":
		if strings.HasPrefix(f.MediaType, "image/") {
			if has("preview") {
				key, mt = f.StorageKey+".preview.jpg", "image/jpeg"
				name = strings.TrimSuffix(name, filepath.Ext(name)) + ".jpg"
			} else if role == identity.RoleViewer {
				return Content{}, problem.Forbidden("Für dieses Bild gibt es keine bereinigte Fassung; das Original ist Bearbeitern vorbehalten (ADR-018).")
			}
		}
	}
	file, err := s.store.Open(key)
	if err != nil {
		return Content{}, err
	}
	st, err := file.Stat()
	if err != nil {
		file.Close()
		return Content{}, err
	}
	return Content{Reader: file, Size: st.Size(), MediaType: mt, Name: name, Inline: inline}, nil
}
