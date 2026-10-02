-- +goose Up
CREATE SCHEMA oil;

CREATE TABLE oil.entry (
    id                  uuid PRIMARY KEY,
    vehicle_id          uuid NOT NULL REFERENCES vehicles.vehicle (id),
    kind                text NOT NULL CHECK (kind IN ('check','top_up','oil_change')),
    occurred_at         timestamptz NOT NULL,
    time_zone           text NOT NULL,
    time_precision      text NOT NULL CHECK (time_precision IN ('exact','date_only')),
    odometer_reading_id uuid REFERENCES odometer.reading (id),
    level_before_pct    numeric(5,2) CHECK (level_before_pct BETWEEN -50 AND 150),
    level_before_input  text CHECK (level_before_input IN ('percent','min','quarter','half','three_quarters','max','below_min','above_max')),
    level_after_pct     numeric(5,2) CHECK (level_after_pct BETWEEN -50 AND 150),
    level_after_input   text CHECK (level_after_input IN ('percent','min','quarter','half','three_quarters','max','below_min','above_max')),
    oil_added_ml        bigint CHECK (oil_added_ml > 0),
    input_added         numeric(14,4),
    input_added_unit    text,
    oil_change_fill_ml  bigint CHECK (oil_change_fill_ml > 0),
    input_fill          numeric(14,4),
    input_fill_unit     text,
    oil_specification   text,
    oil_brand           text,
    oil_product         text,
    filter_changed      boolean,
    service_entry_id    uuid,
    note                text NOT NULL DEFAULT '',
    tags                text[] NOT NULL DEFAULT '{}',
    confirmed_anomalies jsonb NOT NULL DEFAULT '[]'::jsonb,
    origin              text NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    created_by          uuid NOT NULL,
    updated_at          timestamptz NOT NULL DEFAULT now(),
    updated_by          uuid NOT NULL,
    recorded_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz,
    version             integer NOT NULL DEFAULT 1,
    CHECK ((kind = 'top_up') = (oil_added_ml IS NOT NULL)),
    CHECK (kind = 'oil_change' OR (oil_change_fill_ml IS NULL AND filter_changed IS NULL))
);
CREATE INDEX entry_vehicle_time_idx ON oil.entry (vehicle_id, occurred_at, recorded_at, id) WHERE deleted_at IS NULL;

-- +goose Down
DROP SCHEMA oil CASCADE;
