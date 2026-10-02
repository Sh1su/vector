-- name: GetFill :one
SELECT * FROM fuel.fill WHERE id = $1 AND deleted_at IS NULL;

-- name: GetFillAny :one
SELECT * FROM fuel.fill WHERE id = $1;

-- name: ListFills :many
SELECT * FROM fuel.fill
WHERE vehicle_id = sqlc.arg(vehicle_id) AND (sqlc.arg(include_deleted)::boolean OR deleted_at IS NULL)
ORDER BY occurred_at, recorded_at, id;

-- name: InsertFill :one
INSERT INTO fuel.fill (id, vehicle_id, occurred_at, time_zone, time_precision, energy_carrier, quantity, input_quantity, input_unit,
    fill_level, previous_missed, odometer_reading_id, cost_amount_minor, cost_currency, input_price_per_unit, input_price_currency,
    input_price_unit, soc_start_pct, soc_end_pct, charge_type, station, note, tags, confirmed_anomalies, origin, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $26)
RETURNING *;

-- name: UpdateFill :one
UPDATE fuel.fill SET occurred_at = $2, time_zone = $3, time_precision = $4, energy_carrier = $5, quantity = $6, input_quantity = $7,
    input_unit = $8, fill_level = $9, previous_missed = $10, odometer_reading_id = $11, cost_amount_minor = $12, cost_currency = $13,
    input_price_per_unit = $14, input_price_currency = $15, input_price_unit = $16, soc_start_pct = $17, soc_end_pct = $18,
    charge_type = $19, station = $20, note = $21, tags = $22, confirmed_anomalies = $23, updated_by = $24,
    updated_at = now(), version = version + 1
WHERE id = $1 AND version = $25 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteFill :execrows
UPDATE fuel.fill SET deleted_at = now(), updated_by = $2, updated_at = now(), version = version + 1
WHERE id = $1 AND version = $3 AND deleted_at IS NULL;
