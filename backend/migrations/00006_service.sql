-- +goose Up
CREATE SCHEMA service;

-- Serviceeinträge (docs/phase-2/10-domaene-servicehistory.md).
CREATE TABLE service.entry (
    id                  uuid PRIMARY KEY,
    vehicle_id          uuid NOT NULL REFERENCES vehicles.vehicle (id),
    kind                text NOT NULL CHECK (kind IN ('maintenance','inspection','repair','upgrade')),
    category            text CHECK (length(category) <= 60),
    title               text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    description         text,
    occurred_at         timestamptz NOT NULL,
    time_zone           text NOT NULL,
    time_precision      text NOT NULL CHECK (time_precision IN ('exact','date_only')),
    odometer_reading_id uuid REFERENCES odometer.reading (id),
    currency            char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    provider_name       text,
    invoice_number      text,
    cost_unknown        boolean NOT NULL DEFAULT false,
    note                text NOT NULL DEFAULT '',
    tags                text[] NOT NULL DEFAULT '{}',
    origin              text NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    created_by          uuid NOT NULL,
    updated_at          timestamptz NOT NULL DEFAULT now(),
    updated_by          uuid NOT NULL,
    recorded_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz,
    version             integer NOT NULL DEFAULT 1
);
CREATE INDEX entry_vehicle_time_idx ON service.entry (vehicle_id, occurred_at DESC, id DESC) WHERE deleted_at IS NULL;

CREATE TABLE service.cost_item (
    id           uuid PRIMARY KEY,
    entry_id     uuid NOT NULL REFERENCES service.entry (id) ON DELETE CASCADE,
    position     integer NOT NULL,
    kind         text NOT NULL CHECK (kind IN ('parts','labor','other')),
    label        text CHECK (length(label) <= 200),
    amount_minor bigint NOT NULL
);
CREATE INDEX cost_item_entry_idx ON service.cost_item (entry_id, position);

CREATE TABLE service.part_line (
    id               uuid PRIMARY KEY,
    entry_id         uuid NOT NULL REFERENCES service.entry (id) ON DELETE CASCADE,
    position         integer NOT NULL,
    name             text NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
    part_number      text CHECK (length(part_number) <= 100),
    quantity         numeric(14,4) NOT NULL CHECK (quantity >= 0),
    quantity_unit    text CHECK (length(quantity_unit) <= 20),
    unit_price_minor bigint,
    cost_item_id     uuid
);
CREATE INDEX part_line_entry_idx ON service.part_line (entry_id, position);

-- +goose Down
DROP SCHEMA service CASCADE;
