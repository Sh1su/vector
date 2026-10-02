-- name: GetUserState :one
SELECT * FROM assistant.user_state WHERE account_id = $1;

-- name: UpsertUserState :exec
INSERT INTO assistant.user_state (account_id, enabled, consent_provider, consent_at) VALUES ($1, $2, $3, $4)
ON CONFLICT (account_id) DO UPDATE SET enabled = EXCLUDED.enabled, consent_provider = EXCLUDED.consent_provider, consent_at = EXCLUDED.consent_at;

-- name: InsertConversation :one
INSERT INTO assistant.conversation (id, account_id, vehicle_id, title) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetConversation :one
SELECT * FROM assistant.conversation WHERE id = $1 AND account_id = $2;

-- name: ListConversations :many
SELECT * FROM assistant.conversation WHERE account_id = $1 ORDER BY updated_at DESC LIMIT 100;

-- name: TouchConversation :exec
UPDATE assistant.conversation SET updated_at = now(), title = COALESCE(title, sqlc.narg(title)) WHERE id = $1;

-- name: DeleteConversation :execrows
DELETE FROM assistant.conversation WHERE id = $1 AND account_id = $2;

-- name: InsertMessage :one
INSERT INTO assistant.message (id, conversation_id, role, text, citations) VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: ListMessages :many
SELECT * FROM assistant.message WHERE conversation_id = $1 ORDER BY created_at, id;

-- name: InsertProposal :one
INSERT INTO assistant.proposal (id, conversation_id, account_id, vehicle_id, operation, body, anomalies, status, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending', $8) RETURNING *;

-- name: AttachProposals :exec
UPDATE assistant.proposal SET message_id = $1 WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: GetProposal :one
SELECT * FROM assistant.proposal WHERE id = $1 AND account_id = $2;

-- name: ListProposalsForConversation :many
SELECT * FROM assistant.proposal WHERE conversation_id = $1 ORDER BY created_at;

-- name: DecideProposal :one
UPDATE assistant.proposal SET status = $2, result_id = $3, body = $4, decided_at = now() WHERE id = $1 AND status = 'pending' RETURNING *;

-- name: InsertRequestLog :exec
INSERT INTO assistant.request_log (id, account_id, provider, model, input_tokens, output_tokens, tools, outcome)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: CountRequestsSince :one
SELECT count(*) FROM assistant.request_log WHERE account_id = $1 AND occurred_at >= $2;

-- name: DeleteOldConversations :execrows
DELETE FROM assistant.conversation WHERE updated_at < $1;
