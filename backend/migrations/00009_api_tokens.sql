-- +goose Up
-- API-Tokens für Integrationen (MCP-Server, ADR-015). Gespeichert wird nur der SHA-256 des Tokens.
CREATE TABLE identity.api_token (
    id           uuid PRIMARY KEY,
    account_id   uuid NOT NULL REFERENCES identity.account (id),
    name         text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    token_hash   bytea NOT NULL UNIQUE,
    scopes       text[] NOT NULL,
    vehicle_ids  uuid[],
    expires_at   timestamptz NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    revoked_at   timestamptz
);
CREATE INDEX api_token_account_idx ON identity.api_token (account_id);

-- +goose Down
DROP TABLE identity.api_token;
