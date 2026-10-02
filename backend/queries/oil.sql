-- name: GetEntry :one
SELECT * FROM oil.entry WHERE id = $1 AND deleted_at IS NULL;

-- name: GetEntryAny :one
SELECT * FROM oil.entry WHERE id = $1;

-- name: ListEntries :many
SELECT * FROM oil.entry
WHERE vehicle_id = sqlc.arg(vehicle_id) AND (sqlc.arg(include_deleted)::boolean OR deleted_at IS NULL)
ORDER BY occurred_at, recorded_at, id;

-- name: InsertEntry :one
INSERT INTO oil.entry (id, vehicle_id, kind, occurred_at, time_zone, time_precision, odometer_reading_id, level_before_pct,
    level_before_input, level_after_pct, level_after_input, oil_added_ml, input_added, input_added_unit, oil_change_fill_ml,
    input_fill, input_fill_unit, oil_specification, oil_brand, oil_product, filter_changed, service_entry_id, note, tags,
    confirmed_anomalies, origin, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $27)
RETURNING *;

-- name: UpdateEntry :one
UPDATE oil.entry SET kind = $2, occurred_at = $3, time_zone = $4, time_precision = $5, odometer_reading_id = $6,
    level_before_pct = $7, level_before_input = $8, level_after_pct = $9, level_after_input = $10, oil_added_ml = $11,
    input_added = $12, input_added_unit = $13, oil_change_fill_ml = $14, input_fill = $15, input_fill_unit = $16,
    oil_specification = $17, oil_brand = $18, oil_product = $19, filter_changed = $20, service_entry_id = $21, note = $22,
    tags = $23, confirmed_anomalies = $24, updated_by = $25, updated_at = now(), version = version + 1
WHERE id = $1 AND version = $26 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteEntry :execrows
UPDATE oil.entry SET deleted_at = now(), updated_by = $2, updated_at = now(), version = version + 1
WHERE id = $1 AND version = $3 AND deleted_at IS NULL;
