-- name: InsertEntry :one
INSERT INTO service.entry (id, vehicle_id, kind, category, title, description, occurred_at, time_zone, time_precision, odometer_reading_id,
    currency, provider_name, invoice_number, cost_unknown, note, tags, origin, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $18)
RETURNING *;

-- name: GetEntry :one
SELECT * FROM service.entry WHERE id = $1;

-- name: UpdateEntry :one
UPDATE service.entry SET kind = $2, category = $3, title = $4, description = $5, occurred_at = $6, time_zone = $7, time_precision = $8,
    odometer_reading_id = $9, currency = $10, provider_name = $11, invoice_number = $12, cost_unknown = $13, note = $14, tags = $15,
    updated_by = $16, updated_at = now(), version = version + 1
WHERE id = $1 AND version = $17 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteEntry :execrows
UPDATE service.entry SET deleted_at = now(), updated_by = $2, updated_at = now(), version = version + 1
WHERE id = $1 AND version = $3 AND deleted_at IS NULL;

-- name: ListEntries :many
SELECT * FROM service.entry e
WHERE e.vehicle_id = sqlc.arg(vehicle_id)
  AND (sqlc.arg(include_deleted)::boolean OR e.deleted_at IS NULL)
  AND (sqlc.narg(from_ts)::timestamptz IS NULL OR e.occurred_at >= sqlc.narg(from_ts)::timestamptz)
  AND (sqlc.narg(to_ts)::timestamptz IS NULL OR e.occurred_at < sqlc.narg(to_ts)::timestamptz)
  AND (sqlc.narg(kind)::text IS NULL OR e.kind = sqlc.narg(kind)::text)
  AND (sqlc.narg(q)::text IS NULL OR e.title ILIKE '%' || sqlc.narg(q)::text || '%' OR e.description ILIKE '%' || sqlc.narg(q)::text || '%'
       OR e.provider_name ILIKE '%' || sqlc.narg(q)::text || '%' OR e.invoice_number ILIKE '%' || sqlc.narg(q)::text || '%'
       OR EXISTS (SELECT 1 FROM service.part_line p WHERE p.entry_id = e.id
                  AND (p.name ILIKE '%' || sqlc.narg(q)::text || '%' OR p.part_number ILIKE '%' || sqlc.narg(q)::text || '%')))
  AND (sqlc.narg(before_ts)::timestamptz IS NULL OR (e.occurred_at, e.id) < (sqlc.narg(before_ts)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY e.occurred_at DESC, e.id DESC
LIMIT sqlc.arg(lim);

-- name: ListAllEntries :many
SELECT * FROM service.entry WHERE vehicle_id = $1 AND deleted_at IS NULL ORDER BY occurred_at, id;

-- name: InsertCostItem :exec
INSERT INTO service.cost_item (id, entry_id, position, kind, label, amount_minor) VALUES ($1, $2, $3, $4, $5, $6);

-- name: DeleteCostItems :exec
DELETE FROM service.cost_item WHERE entry_id = $1;

-- name: ListCostItems :many
SELECT * FROM service.cost_item WHERE entry_id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY entry_id, position;

-- name: InsertPartLine :exec
INSERT INTO service.part_line (id, entry_id, position, name, part_number, quantity, quantity_unit, unit_price_minor, cost_item_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: DeletePartLines :exec
DELETE FROM service.part_line WHERE entry_id = $1;

-- name: ListPartLines :many
SELECT * FROM service.part_line WHERE entry_id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY entry_id, position;
