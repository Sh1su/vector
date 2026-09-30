-- name: InsertFile :one
INSERT INTO documents.file (id, vehicle_id, storage_key, original_name, media_type, size_bytes, sha256, captured_at_client, capture_source,
    derivatives, has_location, supersedes_id, replace_reason, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING *;

-- name: GetFile :one
SELECT * FROM documents.file WHERE id = $1;

-- name: FileBySha :one
SELECT * FROM documents.file WHERE vehicle_id = $1 AND sha256 = $2 AND deleted_at IS NULL ORDER BY received_at LIMIT 1;

-- name: ListFiles :many
SELECT * FROM documents.file
WHERE vehicle_id = sqlc.arg(vehicle_id) AND deleted_at IS NULL
  AND (sqlc.narg(before_ts)::timestamptz IS NULL OR (received_at, id) < (sqlc.narg(before_ts)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY received_at DESC, id DESC
LIMIT sqlc.arg(lim);

-- name: SoftDeleteFile :execrows
UPDATE documents.file SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: FileReferences :one
SELECT (SELECT count(*) FROM documents.document_file df JOIN documents.document d ON d.id = df.document_id WHERE df.file_id = $1 AND d.deleted_at IS NULL)
     + (SELECT count(*) FROM documents.attachment a WHERE a.file_id = $1 AND a.deleted_at IS NULL)
     + (SELECT count(*) FROM documents.vehicle_image i WHERE i.file_id = $1) AS refs;

-- name: RepointDocumentFiles :exec
UPDATE documents.document_file SET file_id = sqlc.arg(new_id) WHERE file_id = sqlc.arg(old_id);

-- name: RepointAttachments :exec
UPDATE documents.attachment SET file_id = sqlc.arg(new_id) WHERE file_id = sqlc.arg(old_id) AND deleted_at IS NULL;

-- name: RepointImages :exec
UPDATE documents.vehicle_image SET file_id = sqlc.arg(new_id) WHERE file_id = sqlc.arg(old_id);

-- name: InsertDocument :one
INSERT INTO documents.document (id, vehicle_id, doc_type, nature, title, document_date, issuer, language, note, tags, origin, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $12)
RETURNING *;

-- name: GetDocument :one
SELECT * FROM documents.document WHERE id = $1;

-- name: UpdateDocument :one
UPDATE documents.document SET doc_type = $2, nature = $3, title = $4, document_date = $5, issuer = $6, language = $7, note = $8, tags = $9,
    updated_by = $10, updated_at = now(), version = version + 1
WHERE id = $1 AND version = $11 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteDocument :execrows
UPDATE documents.document SET deleted_at = now(), updated_by = $2, updated_at = now(), version = version + 1
WHERE id = $1 AND version = $3 AND deleted_at IS NULL;

-- name: ListDocuments :many
SELECT * FROM documents.document d
WHERE d.vehicle_id = sqlc.arg(vehicle_id) AND d.deleted_at IS NULL
  AND (sqlc.narg(doc_type)::text IS NULL OR d.doc_type = sqlc.narg(doc_type)::text)
  AND (sqlc.narg(nature)::text IS NULL OR d.nature = sqlc.narg(nature)::text)
  AND (sqlc.narg(q)::text IS NULL OR d.title ILIKE '%' || sqlc.narg(q)::text || '%' OR d.issuer ILIKE '%' || sqlc.narg(q)::text || '%'
       OR d.note ILIKE '%' || sqlc.narg(q)::text || '%' OR array_to_string(d.tags, ' ') ILIKE '%' || sqlc.narg(q)::text || '%')
ORDER BY coalesce(d.document_date, d.created_at::date) DESC, d.id DESC
LIMIT sqlc.arg(lim);

-- name: DeleteDocumentFiles :exec
DELETE FROM documents.document_file WHERE document_id = $1;

-- name: InsertDocumentFile :exec
INSERT INTO documents.document_file (document_id, position, file_id) VALUES ($1, $2, $3);

-- name: DocumentFiles :many
SELECT document_id, file_id FROM documents.document_file WHERE document_id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY document_id, position;

-- name: InsertAttachment :one
INSERT INTO documents.attachment (id, vehicle_id, file_id, document_id, target_type, target_id, role, page, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetAttachment :one
SELECT * FROM documents.attachment WHERE id = $1;

-- name: ListAttachments :many
SELECT * FROM documents.attachment
WHERE vehicle_id = sqlc.arg(vehicle_id) AND deleted_at IS NULL
  AND (sqlc.narg(target_type)::text IS NULL OR target_type = sqlc.narg(target_type)::text)
  AND (sqlc.narg(target_id)::uuid IS NULL OR target_id = sqlc.narg(target_id)::uuid)
ORDER BY created_at, id;

-- name: SoftDeleteAttachment :execrows
UPDATE documents.attachment SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: ListImages :many
SELECT * FROM documents.vehicle_image WHERE vehicle_id = $1 ORDER BY is_primary DESC, created_at;

-- name: ClearPrimaryImage :exec
UPDATE documents.vehicle_image SET is_primary = false WHERE vehicle_id = $1 AND is_primary;

-- name: UpsertImage :one
INSERT INTO documents.vehicle_image (id, vehicle_id, file_id, is_primary, created_by) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (vehicle_id, file_id) DO UPDATE SET is_primary = EXCLUDED.is_primary
RETURNING *;

-- name: GetImage :one
SELECT * FROM documents.vehicle_image WHERE id = $1;

-- name: DeleteImage :execrows
DELETE FROM documents.vehicle_image WHERE id = $1;

-- name: PromoteFirstImage :exec
UPDATE documents.vehicle_image vi SET is_primary = true
WHERE vi.id = (SELECT x.id FROM documents.vehicle_image x WHERE x.vehicle_id = $1 ORDER BY x.created_at LIMIT 1)
  AND NOT EXISTS (SELECT 1 FROM documents.vehicle_image y WHERE y.vehicle_id = $1 AND y.is_primary);
