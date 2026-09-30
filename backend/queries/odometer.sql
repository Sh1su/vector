-- name: ListSegments :many
SELECT * FROM odometer.segment WHERE vehicle_id = $1 ORDER BY sequence_no;

-- name: InsertSegment :one
INSERT INTO odometer.segment (id, vehicle_id, sequence_no, started_at, time_zone, start_meter_value, "offset", reason, note, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: ListValidReadings :many
SELECT * FROM odometer.reading
WHERE vehicle_id = $1 AND deleted_at IS NULL AND status <> 'superseded'
ORDER BY occurred_at, recorded_at, id;

-- name: CountReadings :one
SELECT count(*) FROM odometer.reading WHERE vehicle_id = $1 AND deleted_at IS NULL;

-- name: GetReading :one
SELECT * FROM odometer.reading WHERE id = $1 AND deleted_at IS NULL;

-- name: InsertReading :one
INSERT INTO odometer.reading (id, vehicle_id, occurred_at, time_zone, time_precision, value, input_value, input_unit,
    source, source_ref, status, confirmed_anomalies, supersedes_id, photo_file_id, note, origin, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
RETURNING *;

-- name: MarkSuperseded :execrows
UPDATE odometer.reading SET status = 'superseded', version = version + 1
WHERE id = $1 AND version = $2 AND status <> 'superseded' AND deleted_at IS NULL;

-- name: SoftDeleteReading :execrows
UPDATE odometer.reading SET deleted_at = now(), version = version + 1
WHERE id = $1 AND version = $2 AND deleted_at IS NULL;

-- name: ListReadingsPage :many
SELECT * FROM odometer.reading
WHERE vehicle_id = sqlc.arg(vehicle_id) AND deleted_at IS NULL
  AND (sqlc.arg(include_superseded)::boolean OR status <> 'superseded')
  AND (sqlc.narg(from_ts)::timestamptz IS NULL OR occurred_at >= sqlc.narg(from_ts)::timestamptz)
  AND (sqlc.narg(to_ts)::timestamptz IS NULL OR occurred_at < sqlc.narg(to_ts)::timestamptz)
  AND (sqlc.narg(before_ts)::timestamptz IS NULL OR (occurred_at, id) < (sqlc.narg(before_ts)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY occurred_at DESC, id DESC
LIMIT sqlc.arg(lim);

-- name: GetSuccessor :one
SELECT id FROM odometer.reading WHERE supersedes_id = $1 AND deleted_at IS NULL;
