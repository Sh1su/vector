package server

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sh1su/vector/backend/internal/api"
	"github.com/sh1su/vector/backend/internal/documents"
	"github.com/sh1su/vector/backend/internal/platform/problem"
)

// ---------- Dateien ----------

// readUpload liest die Formularfelder vor dem Teil „file“ und übergibt den
// Dateiinhalt gestreamt an fn. Felder nach der Datei werden ignoriert.
func readUpload(mr *multipart.Reader, fn func(fields map[string]string, name string, r io.Reader) error) error {
	fields := map[string]string{}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			return problem.Validation(problem.FieldError{Pointer: "/file", Code: "required", Message: "Datei fehlt."})
		}
		if err != nil {
			return problem.BadRequest("Ungültiger Multipart-Body.")
		}
		if part.FormName() == "file" {
			return fn(fields, part.FileName(), part)
		}
		b, _ := io.ReadAll(io.LimitReader(part, 4096))
		fields[part.FormName()] = string(b)
	}
}

func uploadInput(f map[string]string, name string) (documents.UploadInput, error) {
	in := documents.UploadInput{Name: name, CaptureSource: f["capture_source"], ClientSHA256: f["client_sha256"], AllowDuplicate: f["allow_duplicate"] == "true"}
	switch in.CaptureSource {
	case "", "camera", "gallery", "scanner", "upload":
	default:
		return in, problem.Validation(problem.FieldError{Pointer: "/capture_source", Code: "enum"})
	}
	if in.CaptureSource == "" {
		in.CaptureSource = "upload"
	}
	if v := f["id"]; v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return in, problem.Validation(problem.FieldError{Pointer: "/id", Code: "uuid"})
		}
		in.ID = &id
	}
	if v := f["captured_at_client"]; v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return in, problem.Validation(problem.FieldError{Pointer: "/captured_at_client", Code: "date_time"})
		}
		in.CapturedAtClient = &t
	}
	return in, nil
}

func fileBody(v documents.FileView) (api.FileMeta, error) {
	var out api.FileMeta
	return out, convert(v, &out)
}

func (s *Server) UploadFile(ctx context.Context, req api.UploadFileRequestObject) (api.UploadFileResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var v documents.FileView
	var created bool
	err = readUpload(req.Body, func(f map[string]string, name string, r io.Reader) error {
		in, err := uploadInput(f, name)
		if err != nil {
			return err
		}
		v, created, err = s.d.Documents.Upload(ctx, a, uuid.UUID(req.VehicleId), in, r)
		return err
	})
	if err != nil {
		return nil, err
	}
	body, err := fileBody(v)
	if err != nil {
		return nil, err
	}
	if !created {
		return api.UploadFile200JSONResponse(body), nil
	}
	return api.UploadFile201JSONResponse{Body: body, Headers: api.UploadFile201ResponseHeaders{ETag: etag(1),
		Location: loc("/vehicles/" + v.VehicleID.String() + "/files/" + v.ID.String())}}, nil
}

func (s *Server) ListFiles(ctx context.Context, req api.ListFilesRequestObject) (api.ListFilesResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	limit := 0
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	items, next, err := s.d.Documents.ListFiles(ctx, a, uuid.UUID(req.VehicleId), req.Params.Cursor, limit)
	if err != nil {
		return nil, err
	}
	var out api.ListFiles200JSONResponse
	return out, page(items, next, &out)
}

func (s *Server) GetFile(ctx context.Context, req api.GetFileRequestObject) (api.GetFileResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Documents.GetFile(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.FileId))
	if err != nil {
		return nil, err
	}
	body, err := fileBody(v)
	return api.GetFile200JSONResponse{Body: body, Headers: api.GetFile200ResponseHeaders{ETag: etag(1)}}, err
}

func (s *Server) DeleteFile(ctx context.Context, req api.DeleteFileRequestObject) (api.DeleteFileResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.Documents.DeleteFile(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.FileId)); err != nil {
		return nil, err
	}
	return api.DeleteFile204Response{}, nil
}

func (s *Server) ReplaceFile(ctx context.Context, req api.ReplaceFileRequestObject) (api.ReplaceFileResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var v documents.FileView
	err = readUpload(req.Body, func(f map[string]string, name string, r io.Reader) error {
		var err error
		v, err = s.d.Documents.Replace(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.FileId), f["reason"], name, r)
		return err
	})
	if err != nil {
		return nil, err
	}
	body, err := fileBody(v)
	if err != nil {
		return nil, err
	}
	return api.ReplaceFile201JSONResponse{Body: body, Headers: api.ReplaceFile201ResponseHeaders{ETag: etag(1),
		Location: loc("/vehicles/" + v.VehicleID.String() + "/files/" + v.ID.String())}}, nil
}

// fileResponse liefert Binärinhalte mit den Headern aus ADR-017: Download als
// Anhang, Vorschaubilder inline; immer nosniff und CSP sandbox.
type fileResponse struct{ c documents.Content }

func (f fileResponse) write(w http.ResponseWriter) error {
	defer f.c.Reader.Close()
	h := w.Header()
	h.Set("Content-Type", f.c.MediaType)
	h.Set("Content-Length", strconv.FormatInt(f.c.Size, 10))
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("Cache-Control", "private, max-age=3600")
	disp := "attachment"
	if f.c.Inline {
		disp = "inline"
	}
	h.Set("Content-Disposition", mime.FormatMediaType(disp, map[string]string{"filename": strings.ReplaceAll(f.c.Name, "\"", "")}))
	w.WriteHeader(http.StatusOK)
	_, err := io.Copy(w, f.c.Reader)
	return err
}

func (f fileResponse) VisitDownloadFileResponse(w http.ResponseWriter) error     { return f.write(w) }
func (f fileResponse) VisitPreviewFileResponse(w http.ResponseWriter) error      { return f.write(w) }
func (f fileResponse) VisitDownloadOriginalResponse(w http.ResponseWriter) error { return f.write(w) }

func (s *Server) DownloadFile(ctx context.Context, req api.DownloadFileRequestObject) (api.DownloadFileResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	c, err := s.d.Documents.Open(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.FileId), "content")
	if err != nil {
		return nil, err
	}
	return fileResponse{c}, nil
}

func (s *Server) PreviewFile(ctx context.Context, req api.PreviewFileRequestObject) (api.PreviewFileResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	kind := "preview"
	if req.Params.Size != nil {
		kind = string(*req.Params.Size)
	}
	c, err := s.d.Documents.Open(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.FileId), kind)
	if err != nil {
		return nil, err
	}
	return fileResponse{c}, nil
}

func (s *Server) DownloadOriginal(ctx context.Context, req api.DownloadOriginalRequestObject) (api.DownloadOriginalResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	c, err := s.d.Documents.Open(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.FileId), "original")
	if err != nil {
		return nil, err
	}
	return fileResponse{c}, nil
}

// ---------- Dokumente ----------

func docBody(v documents.DocView) (api.Document, error) {
	var out api.Document
	return out, convert(v, &out)
}

func (s *Server) ListDocuments(ctx context.Context, req api.ListDocumentsRequestObject) (api.ListDocumentsResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	f := documents.DocFilter{}
	if req.Params.DocType != nil {
		t := string(*req.Params.DocType)
		f.DocType = &t
	}
	if req.Params.Nature != nil {
		n := string(*req.Params.Nature)
		f.Nature = &n
	}
	if req.Params.Limit != nil {
		f.Limit = *req.Params.Limit
	}
	items, err := s.d.Documents.ListDocuments(ctx, a, uuid.UUID(req.VehicleId), f)
	if err != nil {
		return nil, err
	}
	var out api.ListDocuments200JSONResponse
	return out, page(items, nil, &out)
}

func (s *Server) CreateDocument(ctx context.Context, req api.CreateDocumentRequestObject) (api.CreateDocumentResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var in documents.DocInput
	if err := bodyTo(req.Body, &in); err != nil {
		return nil, err
	}
	v, created, err := s.d.Documents.CreateDocument(ctx, a, uuid.UUID(req.VehicleId), (*uuid.UUID)(req.Body.Id), in)
	if err != nil {
		return nil, err
	}
	body, err := docBody(v)
	if err != nil {
		return nil, err
	}
	if !created {
		return api.CreateDocument200JSONResponse(body), nil
	}
	return api.CreateDocument201JSONResponse{Body: body, Headers: api.CreateDocument201ResponseHeaders{ETag: etag(v.Version),
		Location: loc("/vehicles/" + v.VehicleID.String() + "/documents/" + v.ID.String())}}, nil
}

func (s *Server) GetDocument(ctx context.Context, req api.GetDocumentRequestObject) (api.GetDocumentResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Documents.GetDocument(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.DocumentId))
	if err != nil {
		return nil, err
	}
	body, err := docBody(v)
	return api.GetDocument200JSONResponse{Body: body, Headers: api.GetDocument200ResponseHeaders{ETag: etag(v.Version)}}, err
}

func (s *Server) UpdateDocument(ctx context.Context, req api.UpdateDocumentRequestObject) (api.UpdateDocumentResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	patch, _, _, err := patchOf(req.Body)
	if err != nil {
		return nil, err
	}
	v, err := s.d.Documents.UpdateDocument(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.DocumentId), req.Params.IfMatch, patch)
	if err != nil {
		return nil, err
	}
	body, err := docBody(v)
	return api.UpdateDocument200JSONResponse{Body: body, Headers: api.UpdateDocument200ResponseHeaders{ETag: etag(v.Version)}}, err
}

func (s *Server) DeleteDocument(ctx context.Context, req api.DeleteDocumentRequestObject) (api.DeleteDocumentResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.Documents.DeleteDocument(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.DocumentId), req.Params.IfMatch); err != nil {
		return nil, err
	}
	return api.DeleteDocument204Response{}, nil
}

// ---------- Verknüpfungen ----------

func (s *Server) ListAttachments(ctx context.Context, req api.ListAttachmentsRequestObject) (api.ListAttachmentsResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.d.Documents.ListAttachments(ctx, a, uuid.UUID(req.VehicleId), req.Params.TargetType, (*uuid.UUID)(req.Params.TargetId))
	if err != nil {
		return nil, err
	}
	var out api.ListAttachments200JSONResponse
	return out, page(items, nil, &out)
}

func (s *Server) CreateAttachment(ctx context.Context, req api.CreateAttachmentRequestObject) (api.CreateAttachmentResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	var in documents.AttachmentView
	if err := bodyTo(req.Body, &in); err != nil {
		return nil, err
	}
	v, err := s.d.Documents.Attach(ctx, a, uuid.UUID(req.VehicleId), in)
	if err != nil {
		return nil, err
	}
	var body api.Attachment
	if err := convert(v, &body); err != nil {
		return nil, err
	}
	return api.CreateAttachment201JSONResponse{Body: body, Headers: api.CreateAttachment201ResponseHeaders{ETag: etag(1)}}, nil
}

func (s *Server) DeleteAttachment(ctx context.Context, req api.DeleteAttachmentRequestObject) (api.DeleteAttachmentResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.Documents.Detach(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.AttachmentId)); err != nil {
		return nil, err
	}
	return api.DeleteAttachment204Response{}, nil
}

// ---------- Fahrzeugbilder ----------

func (s *Server) ListVehicleImages(ctx context.Context, req api.ListVehicleImagesRequestObject) (api.ListVehicleImagesResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.d.Documents.ListImages(ctx, a, uuid.UUID(req.VehicleId))
	if err != nil {
		return nil, err
	}
	var out api.ListVehicleImages200JSONResponse
	return out, page(items, nil, &out)
}

func (s *Server) AddVehicleImage(ctx context.Context, req api.AddVehicleImageRequestObject) (api.AddVehicleImageResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	primary := req.Body.Primary != nil && *req.Body.Primary
	v, err := s.d.Documents.AddImage(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.Body.FileId), primary)
	if err != nil {
		return nil, err
	}
	var body api.VehicleImage
	if err := convert(v, &body); err != nil {
		return nil, err
	}
	return api.AddVehicleImage201JSONResponse{Body: body, Headers: api.AddVehicleImage201ResponseHeaders{ETag: etag(1)}}, nil
}

func (s *Server) RemoveVehicleImage(ctx context.Context, req api.RemoveVehicleImageRequestObject) (api.RemoveVehicleImageResponseObject, error) {
	a, err := mustActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.Documents.RemoveImage(ctx, a, uuid.UUID(req.VehicleId), uuid.UUID(req.ImageId)); err != nil {
		return nil, err
	}
	return api.RemoveVehicleImage204Response{}, nil
}
