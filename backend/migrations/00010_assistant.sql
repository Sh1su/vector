-- +goose Up
-- Assistent (ADR-024, ADR-026, ADR-032): Zustimmung, Unterhaltungen, Vorschläge, Protokoll.
CREATE SCHEMA assistant;

CREATE TABLE assistant.consent (
    account_id uuid PRIMARY KEY,
    provider   text NOT NULL,
    given_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE assistant.conversation (
    id         uuid PRIMARY KEY,
    account_id uuid NOT NULL,
    vehicle_id uuid,
    title      text CHECK (title IS NULL OR length(title) <= 200),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX conversation_account_idx ON assistant.conversation (account_id, updated_at DESC);

CREATE TABLE assistant.message (
    id              uuid PRIMARY KEY,
    conversation_id uuid NOT NULL REFERENCES assistant.conversation (id) ON DELETE CASCADE,
    role            text NOT NULL CHECK (role IN ('user', 'assistant')),
    text            text NOT NULL,
    proposal_ids    uuid[] NOT NULL DEFAULT '{}',
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX message_conversation_idx ON assistant.message (conversation_id, created_at);

CREATE TABLE assistant.proposal (
    id              uuid PRIMARY KEY,
    account_id      uuid NOT NULL,
    conversation_id uuid REFERENCES assistant.conversation (id) ON DELETE CASCADE,
    vehicle_id      uuid NOT NULL,
    operation       text NOT NULL,
    summary         text NOT NULL,
    body            jsonb NOT NULL,
    anomalies       jsonb NOT NULL DEFAULT '[]'::jsonb,
    status          text NOT NULL CHECK (status IN ('pending', 'confirmed', 'rejected', 'expired')),
    expires_at      timestamptz NOT NULL,
    result_id       uuid,
    created_at      timestamptz NOT NULL DEFAULT now(),
    decided_at      timestamptz
);
CREATE INDEX proposal_account_idx ON assistant.proposal (account_id, created_at DESC);

-- Metadaten je Modellanfrage (ADR-024), ohne Inhalte.
CREATE TABLE assistant.request_log (
    id            uuid PRIMARY KEY,
    account_id    uuid NOT NULL,
    provider      text NOT NULL,
    model         text NOT NULL,
    input_tokens  integer NOT NULL,
    output_tokens integer NOT NULL,
    tools         text[] NOT NULL DEFAULT '{}',
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX request_log_account_idx ON assistant.request_log (account_id, created_at);

-- +goose Down
DROP SCHEMA assistant CASCADE;
