-- name: CountAccounts :one
SELECT count(*) FROM identity.account;

-- name: InsertAccount :one
INSERT INTO identity.account (id, email, display_name, status, is_admin, password_hash)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAccountByEmail :one
SELECT * FROM identity.account WHERE email = $1;

-- name: GetAccountByID :one
SELECT * FROM identity.account WHERE id = $1;

-- name: MarkLogin :exec
UPDATE identity.account SET last_login_at = now() WHERE id = $1;

-- name: UpdateAccountSettings :one
UPDATE identity.account SET settings = $2, updated_at = now(), version = version + 1 WHERE id = $1 RETURNING *;

-- name: UpdateAccountDisplayName :one
UPDATE identity.account SET display_name = $2, updated_at = now(), version = version + 1 WHERE id = $1 RETURNING *;

-- name: InsertSession :exec
INSERT INTO identity.session (id, account_id, token_hash, csrf_token, client_kind, user_agent, idle_expires_at, absolute_expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetActiveSession :one
SELECT s.id, s.account_id, s.csrf_token, s.idle_expires_at, s.absolute_expires_at, s.last_seen_at
FROM identity.session s
JOIN identity.account a ON a.id = s.account_id
WHERE s.token_hash = $1 AND s.revoked_at IS NULL
  AND s.idle_expires_at > now() AND s.absolute_expires_at > now()
  AND a.status = 'active';

-- name: TouchSession :exec
UPDATE identity.session SET last_seen_at = now(), idle_expires_at = LEAST($2, absolute_expires_at) WHERE id = $1;

-- name: RevokeSession :exec
UPDATE identity.session SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL;

-- name: ListSessions :many
SELECT id, client_kind, user_agent, created_at, last_seen_at FROM identity.session
WHERE account_id = $1 AND revoked_at IS NULL AND idle_expires_at > now() AND absolute_expires_at > now()
ORDER BY last_seen_at DESC;

-- name: RevokeOwnSession :execrows
UPDATE identity.session SET revoked_at = now() WHERE id = $1 AND account_id = $2 AND revoked_at IS NULL;

-- name: GetRole :one
SELECT role FROM identity.vehicle_membership WHERE vehicle_id = $1 AND account_id = $2;

-- name: InsertMembership :exec
INSERT INTO identity.vehicle_membership (vehicle_id, account_id, role) VALUES ($1, $2, $3);

-- name: ListMemberVehicleIDs :many
SELECT vehicle_id, role FROM identity.vehicle_membership WHERE account_id = $1;

-- name: UpdatePasswordHash :exec
UPDATE identity.account SET password_hash = $2, updated_at = now(), version = version + 1 WHERE id = $1;

-- name: RevokeOtherSessions :exec
UPDATE identity.session SET revoked_at = now() WHERE account_id = $1 AND id <> $2 AND revoked_at IS NULL;

-- name: GetInstallationSettings :one
SELECT settings FROM identity.installation_settings WHERE id;

-- name: UpdateInstallationSettings :exec
UPDATE identity.installation_settings SET settings = $1, updated_by = $2, updated_at = now(), version = version + 1 WHERE id;
