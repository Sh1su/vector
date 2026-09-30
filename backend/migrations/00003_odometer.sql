-- +goose Up
CREATE TABLE odometer.segment (
    id                uuid PRIMARY KEY,
    vehicle_id        uuid NOT NULL REFERENCES vehicles.vehicle (id),
    sequence_no       integer NOT NULL CHECK (sequence_no >= 1),
    started_at        timestamptz NOT NULL,          -- Abschnitt 1: '-infinity'
    time_zone         text NOT NULL,
    start_meter_value bigint NOT NULL CHECK (start_meter_value >= 0),
    "offset"          bigint NOT NULL,
    reason            text NOT NULL CHECK (reason IN ('initial','replacement','rollover')),
    note              text NOT NULL DEFAULT '',
    created_at        timestamptz NOT NULL DEFAULT now(),
    created_by        uuid NOT NULL,
    UNIQUE (vehicle_id, sequence_no),
    UNIQUE (vehicle_id, started_at)
);

CREATE TABLE odometer.reading (
    id                 uuid PRIMARY KEY,
    vehicle_id         uuid NOT NULL REFERENCES vehicles.vehicle (id),
    occurred_at        timestamptz NOT NULL,
    time_zone          text NOT NULL,
    time_precision     text NOT NULL CHECK (time_precision IN ('exact','date_only')),
    value              bigint NOT NULL CHECK (value >= 0),
    input_value        numeric(14,4) NOT NULL,
    input_unit         text NOT NULL,
    source             text NOT NULL CHECK (source IN ('manual','fuel','oil','trip_start','trip_end','service','import','assistant')),
    source_ref         uuid,
    status             text NOT NULL CHECK (status IN ('valid','confirmed_anomaly','superseded')),
    confirmed_anomalies jsonb NOT NULL DEFAULT '[]'::jsonb,
    supersedes_id      uuid REFERENCES odometer.reading (id),
    photo_file_id      uuid,
    note               text NOT NULL DEFAULT '',
    origin             text NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    created_by         uuid NOT NULL,
    recorded_at        timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz,
    version            integer NOT NULL DEFAULT 1
);
CREATE UNIQUE INDEX reading_supersedes_uq ON odometer.reading (supersedes_id) WHERE supersedes_id IS NOT NULL;
CREATE INDEX reading_vehicle_time_idx ON odometer.reading (vehicle_id, occurred_at, recorded_at, id) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE odometer.reading;
DROP TABLE odometer.segment;
