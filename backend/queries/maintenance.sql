-- name: InsertItem :one
INSERT INTO maintenance.item (id, vehicle_id, title, description, category, manufacturer_recommended, source_document_id, source_page,
    schedule_mode, interval_months, interval_days, interval_distance, anchor_date, anchor_total, due_date_once, due_total_once,
    upcoming_days, due_days, upcoming_distance, due_distance, distance_unit, active, note, origin, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $25)
RETURNING *;

-- name: GetItem :one
SELECT * FROM maintenance.item WHERE id = $1;

-- name: UpdateItem :one
UPDATE maintenance.item SET title = $2, description = $3, category = $4, manufacturer_recommended = $5, source_document_id = $6,
    source_page = $7, schedule_mode = $8, interval_months = $9, interval_days = $10, interval_distance = $11, anchor_date = $12,
    anchor_total = $13, due_date_once = $14, due_total_once = $15, upcoming_days = $16, due_days = $17, upcoming_distance = $18,
    due_distance = $19, distance_unit = $20, active = $21, note = $22, updated_by = $23, updated_at = now(), version = version + 1
WHERE id = $1 AND version = $24 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteItem :execrows
UPDATE maintenance.item SET deleted_at = now(), updated_by = $2, updated_at = now(), version = version + 1
WHERE id = $1 AND version = $3 AND deleted_at IS NULL;

-- name: ListItems :many
SELECT * FROM maintenance.item
WHERE vehicle_id = sqlc.arg(vehicle_id)
  AND (sqlc.arg(include_deleted)::boolean OR deleted_at IS NULL)
  AND (sqlc.narg(active)::boolean IS NULL OR active = sqlc.narg(active)::boolean)
ORDER BY title, id;

-- name: ListCompletions :many
SELECT * FROM maintenance.completion WHERE item_id = $1 ORDER BY completed_on, completed_total NULLS FIRST, id;

-- name: ListVehicleCompletions :many
SELECT * FROM maintenance.completion WHERE vehicle_id = $1;

-- name: GetCompletion :one
SELECT * FROM maintenance.completion WHERE id = $1;

-- name: CompletionByKey :one
SELECT * FROM maintenance.completion WHERE item_id = $1 AND idempotency_key = $2;

-- name: InsertCompletion :one
INSERT INTO maintenance.completion (id, item_id, vehicle_id, service_entry_id, kind, completed_on, completed_total, reason, idempotency_key, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpsertServiceCompletion :one
INSERT INTO maintenance.completion (id, item_id, vehicle_id, service_entry_id, kind, completed_on, completed_total, created_by)
VALUES ($1, $2, $3, $4, 'done', $5, $6, $7)
ON CONFLICT (item_id, service_entry_id) WHERE service_entry_id IS NOT NULL
DO UPDATE SET completed_on = EXCLUDED.completed_on, completed_total = EXCLUDED.completed_total
RETURNING *;

-- name: DeleteCompletion :execrows
DELETE FROM maintenance.completion WHERE id = $1;

-- name: ServiceCompletions :many
SELECT * FROM maintenance.completion WHERE service_entry_id = $1 ORDER BY item_id;

-- name: DeleteServiceCompletions :exec
DELETE FROM maintenance.completion WHERE service_entry_id = $1 AND NOT (item_id = ANY(sqlc.arg(keep)::uuid[]));
