-- name: InsertVehicle :one
INSERT INTO vehicles.vehicle (
    id, display_name, vin, license_plate, plate_country, make, model, variant, model_year, first_registration,
    body_type, engine_code, displacement_ccm, power_kw, transmission, usage_meter, energy_carriers, odometer_required,
    owner_time_zone, default_currency, purchase_date, purchase_amount, purchase_currency, note, tags, extra,
    origin, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $28)
RETURNING *;

-- name: GetVehicle :one
SELECT * FROM vehicles.vehicle WHERE id = $1 AND deleted_at IS NULL;

-- name: GetVehicleIncludingDeleted :one
SELECT * FROM vehicles.vehicle WHERE id = $1;

-- name: LockVehicle :one
SELECT * FROM vehicles.vehicle WHERE id = $1 AND deleted_at IS NULL FOR UPDATE;

-- name: ListVehicles :many
SELECT * FROM vehicles.vehicle
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND deleted_at IS NULL
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (sqlc.narg(after_name)::text IS NULL OR (display_name, id) > (sqlc.narg(after_name)::text, sqlc.narg(after_id)::uuid))
ORDER BY display_name, id
LIMIT sqlc.arg(lim);

-- name: UpdateVehicle :one
UPDATE vehicles.vehicle SET
    display_name = $2, vin = $3, license_plate = $4, plate_country = $5, make = $6, model = $7, variant = $8,
    model_year = $9, first_registration = $10, body_type = $11, engine_code = $12, displacement_ccm = $13,
    power_kw = $14, transmission = $15, usage_meter = $16, energy_carriers = $17, odometer_required = $18,
    owner_time_zone = $19, default_currency = $20, status = $21, purchase_date = $22, purchase_amount = $23,
    purchase_currency = $24, sale_date = $25, sale_amount = $26, sale_currency = $27, note = $28, tags = $29,
    extra = $30, updated_by = $31, updated_at = now(), version = version + 1
WHERE id = $1 AND version = $32 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteVehicle :execrows
UPDATE vehicles.vehicle SET deleted_at = now(), updated_by = $2, updated_at = now(), version = version + 1
WHERE id = $1 AND version = $3 AND deleted_at IS NULL;

-- name: RestoreVehicle :one
UPDATE vehicles.vehicle SET deleted_at = NULL, updated_by = $2, updated_at = now(), version = version + 1
WHERE id = $1 AND deleted_at IS NOT NULL AND deleted_at > now() - interval '30 days'
RETURNING *;

-- name: VehiclesWithVIN :many
SELECT id, display_name FROM vehicles.vehicle
WHERE vin = $1 AND id <> $2 AND id = ANY(sqlc.arg(ids)::uuid[]) AND deleted_at IS NULL;

-- name: InsertAudit :exec
INSERT INTO audit.event (id, actor_account_id, actor_kind, action, vehicle_id, object_type, object_id, changes, reason, request_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);
