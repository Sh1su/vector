-- +goose Up
CREATE SCHEMA fuel;

CREATE TABLE fuel.fill (
    id                   uuid PRIMARY KEY,
    vehicle_id           uuid NOT NULL REFERENCES vehicles.vehicle (id),
    occurred_at          timestamptz NOT NULL,
    time_zone            text NOT NULL,
    time_precision       text NOT NULL CHECK (time_precision IN ('exact','date_only')),
    energy_carrier       text NOT NULL CHECK (energy_carrier IN ('petrol','diesel','lpg','electricity')),
    quantity             bigint NOT NULL CHECK (quantity > 0),
    input_quantity       numeric(14,4) NOT NULL,
    input_unit           text NOT NULL,
    fill_level           text NOT NULL CHECK (fill_level IN ('full','partial')),
    previous_missed      boolean NOT NULL DEFAULT false,
    odometer_reading_id  uuid REFERENCES odometer.reading (id),
    cost_amount_minor    bigint CHECK (cost_amount_minor >= 0),
    cost_currency        char(3) CHECK (cost_currency ~ '^[A-Z]{3}$'),
    input_price_per_unit numeric(14,6),
    input_price_currency char(3),
    input_price_unit     text,
    soc_start_pct        numeric(5,2) CHECK (soc_start_pct BETWEEN 0 AND 100),
    soc_end_pct          numeric(5,2) CHECK (soc_end_pct BETWEEN 0 AND 100),
    charge_type          text CHECK (charge_type IN ('ac','dc','unknown')),
    station              text,
    note                 text NOT NULL DEFAULT '',
    tags                 text[] NOT NULL DEFAULT '{}',
    confirmed_anomalies  jsonb NOT NULL DEFAULT '[]'::jsonb,
    origin               text NOT NULL,
    created_at           timestamptz NOT NULL DEFAULT now(),
    created_by           uuid NOT NULL,
    updated_at           timestamptz NOT NULL DEFAULT now(),
    updated_by           uuid NOT NULL,
    recorded_at          timestamptz NOT NULL DEFAULT now(),
    deleted_at           timestamptz,
    version              integer NOT NULL DEFAULT 1,
    CHECK ((cost_amount_minor IS NULL) = (cost_currency IS NULL)),
    CHECK (soc_start_pct IS NULL OR soc_end_pct IS NULL OR soc_end_pct > soc_start_pct)
);
CREATE INDEX fill_vehicle_time_idx ON fuel.fill (vehicle_id, occurred_at, recorded_at, id) WHERE deleted_at IS NULL;

-- +goose Down
DROP SCHEMA fuel CASCADE;
