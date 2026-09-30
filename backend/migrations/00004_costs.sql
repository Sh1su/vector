-- +goose Up
CREATE SCHEMA costs;

-- Wiederkehrende Kostenpläne (CO-02, I-CO-1).
CREATE TABLE costs.plan (
    id                 uuid PRIMARY KEY,
    vehicle_id         uuid NOT NULL REFERENCES vehicles.vehicle (id),
    category           text NOT NULL CHECK (category IN ('tax','insurance','fee','parking','toll','care','financing','other')),
    title              text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    amount_minor       bigint NOT NULL,
    currency           char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    interval_months    integer CHECK (interval_months > 0),
    interval_days      integer CHECK (interval_days > 0),
    first_due_on       date NOT NULL,
    ends_on            date,
    remind_days_before integer NOT NULL DEFAULT 30 CHECK (remind_days_before BETWEEN 0 AND 365),
    active             boolean NOT NULL DEFAULT true,
    note               text NOT NULL DEFAULT '',
    origin             text NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    created_by         uuid NOT NULL,
    updated_at         timestamptz NOT NULL DEFAULT now(),
    updated_by         uuid NOT NULL,
    recorded_at        timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz,
    version            integer NOT NULL DEFAULT 1,
    CHECK ((interval_months IS NULL) <> (interval_days IS NULL)),
    CHECK (ends_on IS NULL OR ends_on >= first_due_on)
);
CREATE INDEX plan_vehicle_idx ON costs.plan (vehicle_id) WHERE deleted_at IS NULL;

-- Sonstige Kosten (Steuer, Versicherung …).
CREATE TABLE costs.entry (
    id                 uuid PRIMARY KEY,
    vehicle_id         uuid NOT NULL REFERENCES vehicles.vehicle (id),
    category           text NOT NULL CHECK (category IN ('tax','insurance','fee','parking','toll','care','financing','other')),
    title              text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    incurred_on        date NOT NULL,
    time_zone          text NOT NULL,
    covers_from        date,
    covers_to          date,
    amount_minor       bigint NOT NULL,
    currency           char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    recurring_plan_id  uuid REFERENCES costs.plan (id),
    plan_occurrence_on date,
    note               text NOT NULL DEFAULT '',
    tags               text[] NOT NULL DEFAULT '{}',
    origin             text NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    created_by         uuid NOT NULL,
    updated_at         timestamptz NOT NULL DEFAULT now(),
    updated_by         uuid NOT NULL,
    recorded_at        timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz,
    version            integer NOT NULL DEFAULT 1,
    CHECK ((covers_from IS NULL) = (covers_to IS NULL)),
    CHECK (covers_from IS NULL OR covers_from <= covers_to),
    CHECK ((recurring_plan_id IS NULL) = (plan_occurrence_on IS NULL))
);
CREATE INDEX entry_vehicle_idx ON costs.entry (vehicle_id, incurred_on DESC, id DESC) WHERE deleted_at IS NULL;
-- I-CO-2: je Vorkommen höchstens ein (nicht gelöschter) Eintrag.
CREATE UNIQUE INDEX entry_plan_occurrence_uq ON costs.entry (recurring_plan_id, plan_occurrence_on)
    WHERE recurring_plan_id IS NOT NULL AND deleted_at IS NULL;

-- Verworfene Vorkommen (DismissOccurrence).
CREATE TABLE costs.occurrence_dismissal (
    plan_id    uuid NOT NULL REFERENCES costs.plan (id),
    due_on     date NOT NULL,
    reason     text NOT NULL CHECK (length(reason) BETWEEN 1 AND 500),
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid NOT NULL,
    PRIMARY KEY (plan_id, due_on)
);

-- Kostenbuch: Projektion aller Kosten, gespeist in der Transaktion der Quelle (CO-01, I-CO-3).
CREATE TABLE costs.ledger (
    id            uuid PRIMARY KEY,
    vehicle_id    uuid NOT NULL REFERENCES vehicles.vehicle (id),
    source_module text NOT NULL CHECK (source_module IN ('fuel','service','costs')),
    source_id     uuid NOT NULL,
    category      text NOT NULL,
    cost_kind     text CHECK (cost_kind IN ('parts','labor','other')),
    booked_on     date NOT NULL,
    amount_minor  bigint NOT NULL,
    currency      char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    covers_from   date,
    covers_to     date
);
CREATE INDEX ledger_vehicle_idx ON costs.ledger (vehicle_id, booked_on);
CREATE INDEX ledger_source_idx ON costs.ledger (source_module, source_id);

-- +goose Down
DROP SCHEMA costs CASCADE;
