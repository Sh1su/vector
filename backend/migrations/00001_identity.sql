-- +goose Up
CREATE SCHEMA identity;
CREATE SCHEMA vehicles;
CREATE SCHEMA odometer;
CREATE SCHEMA audit;

CREATE TABLE identity.account (
    id             uuid PRIMARY KEY,
    email          text NOT NULL CHECK (email = lower(email) AND position('@' in email) > 1),
    email_verified boolean NOT NULL DEFAULT false,
    display_name   text NOT NULL CHECK (length(display_name) BETWEEN 1 AND 100),
    status         text NOT NULL CHECK (status IN ('invited', 'active', 'disabled', 'deleted')),
    is_admin       boolean NOT NULL DEFAULT false,
    password_hash  text,
    settings       jsonb NOT NULL DEFAULT '{}'::jsonb,
    last_login_at  timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    version        integer NOT NULL DEFAULT 1
);
CREATE UNIQUE INDEX account_email_uq ON identity.account (email);

CREATE TABLE identity.session (
    id                  uuid PRIMARY KEY,
    account_id          uuid NOT NULL REFERENCES identity.account (id),
    token_hash          bytea NOT NULL UNIQUE,
    csrf_token          text NOT NULL,
    client_kind         text NOT NULL CHECK (client_kind IN ('web', 'android')),
    user_agent          text NOT NULL DEFAULT '',
    created_at          timestamptz NOT NULL DEFAULT now(),
    last_seen_at        timestamptz NOT NULL DEFAULT now(),
    idle_expires_at     timestamptz NOT NULL,
    absolute_expires_at timestamptz NOT NULL,
    revoked_at          timestamptz
);
CREATE INDEX session_account_idx ON identity.session (account_id);

-- +goose Down
DROP SCHEMA audit CASCADE;
DROP SCHEMA odometer CASCADE;
DROP SCHEMA vehicles CASCADE;
DROP SCHEMA identity CASCADE;
