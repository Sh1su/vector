-- name: GetConsent :one
SELECT * FROM assistant.consent WHERE account_id = $1;

-- name: UpsertConsent :one
INSERT INTO assistant.consent (account_id, provider) VALUES ($1, $2)
ON CONFLICT (account_id) DO UPDATE SET provider = EXCLUDED.provider, given_at = now()
RETURNING *;

-- name: DeleteConsent :exec
DELETE FROM assistant.consent WHERE account_id = $1;

-- name: InsertConversation :one
INSERT INTO assistant.conversation (id, account_id, vehicle_id, title) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetConversation :one
SELECT * FROM assistant.conversation WHERE id = $1 AND account_id = $2;

-- name: ListConversations :many
SELECT * FROM assistant.conversation WHERE account_id = $1 ORDER BY updated_at DESC LIMIT 50;

-- name: TouchConversation :exec
UPDATE assistant.conversation SET updated_at = now(), title = COALESCE(title, sqlc.narg(title)) WHERE id = $1;

-- name: DeleteConversation :execrows
DELETE FROM assistant.conversation WHERE id = $1 AND account_id = $2;

-- name: InsertMessage :one
INSERT INTO assistant.message (id, conversation_id, role, text, proposal_ids) VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: ListMessages :many
SELECT * FROM assistant.message WHERE conversation_id = $1 ORDER BY created_at, id;

-- name: PurgeOldConversations :exec
DELETE FROM assistant.conversation WHERE account_id = $1 AND updated_at < $2;

-- name: InsertProposal :one
INSERT INTO assistant.proposal (id, account_id, conversation_id, vehicle_id, operation, summary, body, status, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending', $8) RETURNING *;

-- name: GetProposal :one
SELECT * FROM assistant.proposal WHERE id = $1 AND account_id = $2;

-- name: LockProposal :one
SELECT * FROM assistant.proposal WHERE id = $1 AND account_id = $2 FOR UPDATE;

-- name: ListProposals :many
SELECT * FROM assistant.proposal WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND account_id = $1;

-- name: DecideProposal :one
UPDATE assistant.proposal SET status = $2, result_id = $3, anomalies = $4, body = $5, decided_at = now() WHERE id = $1 RETURNING *;

-- name: SetProposalAnomalies :exec
UPDATE assistant.proposal SET anomalies = $2 WHERE id = $1;

-- name: InsertRequestLog :exec
INSERT INTO assistant.request_log (id, account_id, provider, model, input_tokens, output_tokens, tools) VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: CountRequestsSince :one
SELECT count(*) FROM assistant.request_log WHERE account_id = $1 AND created_at >= $2;
