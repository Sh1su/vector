-- +goose Up
-- Persönliche API-Tokens (ADR-015/ADR-016): gespeichert wird nur der Hash.
CREATE TABLE identity.api_token (
    id           uuid PRIMARY KEY,
    account_id   uuid NOT NULL REFERENCES identity.account (id),
    name         text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    token_hash   bytea NOT NULL UNIQUE,
    prefix       text NOT NULL,
    scopes       text[] NOT NULL CHECK (cardinality(scopes) >= 1),
    vehicle_ids  uuid[],
    expires_at   timestamptz NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    revoked_at   timestamptz
);
CREATE INDEX api_token_account_idx ON identity.api_token (account_id);

-- +goose Down
DROP TABLE identity.api_token;
