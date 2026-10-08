-- name: EnsureDefaultCategory :exec
INSERT INTO trips.category (id, vehicle_id, name, kind, purpose_required, default_key, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (vehicle_id, default_key) DO NOTHING;

-- name: ListCategories :many
SELECT * FROM trips.category WHERE vehicle_id = $1 ORDER BY default_key IS NULL, array_position(ARRAY['private','business','commute','other']::text[], default_key), name, id;

-- name: GetCategory :one
SELECT * FROM trips.category WHERE id = $1;

-- name: InsertCategory :one
INSERT INTO trips.category (id, vehicle_id, name, kind, purpose_required, active, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateCategory :one
UPDATE trips.category SET name = $2, kind = $3, purpose_required = $4, active = $5, version = version + 1
WHERE id = $1 AND version = $6
RETURNING *;

-- name: InsertTrip :one
INSERT INTO trips.trip (id, root_id, vehicle_id, started_at, ended_at, time_zone, start_reading_id, end_reading_id, start_location,
    end_location, purpose, category_id, driver_account_id, note, status, supersedes_id, change_reason, origin, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $19)
RETURNING *;

-- name: GetTrip :one
SELECT * FROM trips.trip WHERE id = $1;

-- name: UpdateOpenTrip :one
UPDATE trips.trip SET started_at = $2, start_reading_id = $3, start_location = $4, purpose = $5, category_id = $6,
    driver_account_id = $7, note = $8, updated_by = $9, updated_at = now(), version = version + 1
WHERE id = $1 AND version = $10 AND status = 'open'
RETURNING *;

-- name: FinishTrip :one
UPDATE trips.trip SET ended_at = $2, end_reading_id = $3, end_location = $4, note = sqlc.arg(note), status = 'closed', updated_by = $5,
    updated_at = now(), version = version + 1
WHERE id = $1 AND version = $6 AND status = 'open'
RETURNING *;

-- name: SetTripStatus :one
UPDATE trips.trip SET status = $2, change_reason = COALESCE(sqlc.narg(change_reason), change_reason), updated_by = $3,
    updated_at = now(), version = version + 1
WHERE id = $1 AND version = $4 AND status IN ('open', 'closed')
RETURNING *;

-- name: ValidTrips :many
SELECT * FROM trips.trip WHERE vehicle_id = $1 AND status IN ('open', 'closed') ORDER BY started_at, id;

-- name: ListTrips :many
SELECT * FROM trips.trip
WHERE vehicle_id = sqlc.arg(vehicle_id)
  AND (sqlc.arg(include_history)::boolean OR status IN ('open', 'closed'))
  AND (sqlc.narg(from_ts)::timestamptz IS NULL OR started_at >= sqlc.narg(from_ts)::timestamptz)
  AND (sqlc.narg(to_ts)::timestamptz IS NULL OR started_at < sqlc.narg(to_ts)::timestamptz)
  AND (sqlc.narg(before_ts)::timestamptz IS NULL OR (started_at, id) < (sqlc.narg(before_ts)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY started_at DESC, id DESC
LIMIT sqlc.arg(lim);

-- name: TripHistory :many
SELECT * FROM trips.trip WHERE root_id = $1 ORDER BY created_at, id;

-- Tachofotos (Start/Ende) hängen als Verknüpfung an der ersten Fassung (root_id), damit Korrekturen sie behalten.

-- name: TripPhotoFile :one
SELECT id, vehicle_id, media_type, derivatives, deleted_at FROM documents.file WHERE id = $1;

-- name: ReplaceTripPhoto :exec
UPDATE documents.attachment SET deleted_at = now()
WHERE target_type = 'trip' AND target_id = $1 AND role = $2 AND deleted_at IS NULL;

-- name: InsertTripPhoto :exec
INSERT INTO documents.attachment (id, vehicle_id, file_id, target_type, target_id, role, created_by)
VALUES ($1, $2, $3, 'trip', $4, $5, $6);

-- name: ListTripPhotos :many
SELECT target_id, role, file_id FROM documents.attachment
WHERE target_type = 'trip' AND target_id = ANY(sqlc.arg(root_ids)::uuid[]) AND role IN ('tacho_start', 'tacho_end')
  AND deleted_at IS NULL AND file_id IS NOT NULL
ORDER BY created_at;
