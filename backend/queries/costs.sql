-- name: InsertEntry :one
INSERT INTO costs.entry (id, vehicle_id, category, title, incurred_on, time_zone, covers_from, covers_to, amount_minor, currency,
    recurring_plan_id, plan_occurrence_on, note, tags, origin, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $16)
RETURNING *;

-- name: GetEntry :one
SELECT * FROM costs.entry WHERE id = $1;

-- name: UpdateEntry :one
UPDATE costs.entry SET category = $2, title = $3, incurred_on = $4, time_zone = $5, covers_from = $6, covers_to = $7,
    amount_minor = $8, currency = $9, note = $10, tags = $11, updated_by = $12, updated_at = now(), version = version + 1
WHERE id = $1 AND version = $13 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteEntry :execrows
UPDATE costs.entry SET deleted_at = now(), updated_by = $2, updated_at = now(), version = version + 1
WHERE id = $1 AND version = $3 AND deleted_at IS NULL;

-- name: ListEntries :many
SELECT * FROM costs.entry
WHERE vehicle_id = sqlc.arg(vehicle_id)
  AND (sqlc.arg(include_deleted)::boolean OR deleted_at IS NULL)
  AND (sqlc.narg(from_date)::date IS NULL OR incurred_on >= sqlc.narg(from_date)::date)
  AND (sqlc.narg(to_date)::date IS NULL OR incurred_on <= sqlc.narg(to_date)::date)
  AND (sqlc.narg(category)::text IS NULL OR category = sqlc.narg(category)::text)
  AND (sqlc.narg(before_date)::date IS NULL OR (incurred_on, id) < (sqlc.narg(before_date)::date, sqlc.narg(before_id)::uuid))
ORDER BY incurred_on DESC, id DESC
LIMIT sqlc.arg(lim);

-- name: EntryForOccurrence :one
SELECT * FROM costs.entry WHERE recurring_plan_id = $1 AND plan_occurrence_on = $2 AND deleted_at IS NULL;

-- name: ConfirmedOccurrences :many
SELECT recurring_plan_id, plan_occurrence_on, id FROM costs.entry
WHERE vehicle_id = $1 AND recurring_plan_id IS NOT NULL AND deleted_at IS NULL;

-- name: InsertPlan :one
INSERT INTO costs.plan (id, vehicle_id, category, title, amount_minor, currency, interval_months, interval_days, first_due_on, ends_on,
    remind_days_before, active, note, origin, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $15)
RETURNING *;

-- name: GetPlan :one
SELECT * FROM costs.plan WHERE id = $1;

-- name: UpdatePlan :one
UPDATE costs.plan SET category = $2, title = $3, amount_minor = $4, currency = $5, interval_months = $6, interval_days = $7,
    first_due_on = $8, ends_on = $9, remind_days_before = $10, active = $11, note = $12, updated_by = $13, updated_at = now(), version = version + 1
WHERE id = $1 AND version = $14 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeletePlan :execrows
UPDATE costs.plan SET deleted_at = now(), updated_by = $2, updated_at = now(), version = version + 1
WHERE id = $1 AND version = $3 AND deleted_at IS NULL;

-- name: ListPlans :many
SELECT * FROM costs.plan
WHERE vehicle_id = sqlc.arg(vehicle_id) AND (sqlc.arg(include_deleted)::boolean OR deleted_at IS NULL)
ORDER BY title, id;

-- name: InsertDismissal :execrows
INSERT INTO costs.occurrence_dismissal (plan_id, due_on, reason, created_by) VALUES ($1, $2, $3, $4)
ON CONFLICT DO NOTHING;

-- name: ListDismissals :many
SELECT d.plan_id, d.due_on FROM costs.occurrence_dismissal d JOIN costs.plan p ON p.id = d.plan_id WHERE p.vehicle_id = $1;

-- name: IsDismissed :one
SELECT count(*) FROM costs.occurrence_dismissal WHERE plan_id = $1 AND due_on = $2;

-- name: DeleteLedgerSource :exec
DELETE FROM costs.ledger WHERE source_module = $1 AND source_id = $2;

-- name: InsertLedger :exec
INSERT INTO costs.ledger (id, vehicle_id, source_module, source_id, category, cost_kind, booked_on, amount_minor, currency, covers_from, covers_to)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: ListLedger :many
SELECT * FROM costs.ledger WHERE vehicle_id = $1 ORDER BY booked_on, id;
