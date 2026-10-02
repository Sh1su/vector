-- +goose Up
-- Assistent (ADR-024, ADR-026). Ohne Aktivierung bleiben die Tabellen leer.
CREATE SCHEMA assistant;

CREATE TABLE assistant.user_state (
    account_id       uuid PRIMARY KEY REFERENCES identity.account (id),
    enabled          boolean NOT NULL DEFAULT false,
    consent_provider text,
    consent_at       timestamptz
);

CREATE TABLE assistant.conversation (
    id         uuid PRIMARY KEY,
    account_id uuid NOT NULL REFERENCES identity.account (id),
    vehicle_id uuid REFERENCES vehicles.vehicle (id),
    title      text CHECK (length(title) <= 200),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX conversation_account_idx ON assistant.conversation (account_id, updated_at DESC);

CREATE TABLE assistant.message (
    id              uuid PRIMARY KEY,
    conversation_id uuid NOT NULL REFERENCES assistant.conversation (id) ON DELETE CASCADE,
    role            text NOT NULL CHECK (role IN ('user','assistant')),
    text            text NOT NULL,
    citations       jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX message_conversation_idx ON assistant.message (conversation_id, created_at);

CREATE TABLE assistant.proposal (
    id              uuid PRIMARY KEY,
    conversation_id uuid REFERENCES assistant.conversation (id) ON DELETE SET NULL,
    message_id      uuid REFERENCES assistant.message (id) ON DELETE SET NULL,
    account_id      uuid NOT NULL REFERENCES identity.account (id),
    vehicle_id      uuid NOT NULL REFERENCES vehicles.vehicle (id),
    operation       text NOT NULL,
    body            jsonb NOT NULL,
    anomalies       jsonb NOT NULL DEFAULT '[]'::jsonb,
    status          text NOT NULL CHECK (status IN ('pending','confirmed','rejected','expired')),
    expires_at      timestamptz NOT NULL,
    result_id       uuid,
    created_at      timestamptz NOT NULL DEFAULT now(),
    decided_at      timestamptz
);
CREATE INDEX proposal_message_idx ON assistant.proposal (message_id);

-- Protokoll ohne Inhalte (ADR-024): Zeit, Nutzer, Anbieter, Modell, Token, Werkzeuge.
CREATE TABLE assistant.request_log (
    id            uuid PRIMARY KEY,
    account_id    uuid NOT NULL,
    occurred_at   timestamptz NOT NULL DEFAULT now(),
    provider      text NOT NULL,
    model         text NOT NULL,
    input_tokens  bigint NOT NULL DEFAULT 0,
    output_tokens bigint NOT NULL DEFAULT 0,
    tools         text[] NOT NULL DEFAULT '{}',
    outcome       text NOT NULL
);
CREATE INDEX request_log_account_idx ON assistant.request_log (account_id, occurred_at);

-- +goose Down
DROP SCHEMA assistant CASCADE;
