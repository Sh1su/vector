-- +goose Up
CREATE SCHEMA trips;

-- Fahrtkategorien je Fahrzeug; default_key kennzeichnet die Standardkategorien.
CREATE TABLE trips.category (
    id               uuid PRIMARY KEY,
    vehicle_id       uuid NOT NULL REFERENCES vehicles.vehicle (id),
    name             text NOT NULL CHECK (length(name) BETWEEN 1 AND 60),
    kind             text NOT NULL CHECK (kind IN ('private','business','commute','other')),
    purpose_required boolean NOT NULL DEFAULT false,
    active           boolean NOT NULL DEFAULT true,
    default_key      text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    created_by       uuid NOT NULL,
    version          integer NOT NULL DEFAULT 1,
    UNIQUE (vehicle_id, default_key)
);

-- Fahrten mit Fassungen (TR-03): root_id verbindet alle Fassungen.
CREATE TABLE trips.trip (
    id                uuid PRIMARY KEY,
    root_id           uuid NOT NULL,
    vehicle_id        uuid NOT NULL REFERENCES vehicles.vehicle (id),
    started_at        timestamptz NOT NULL,
    ended_at          timestamptz,
    time_zone         text NOT NULL,
    start_reading_id  uuid NOT NULL REFERENCES odometer.reading (id),
    end_reading_id    uuid REFERENCES odometer.reading (id),
    start_location    text CHECK (length(start_location) <= 200),
    end_location      text CHECK (length(end_location) <= 200),
    purpose           text CHECK (length(purpose) <= 500),
    category_id       uuid NOT NULL REFERENCES trips.category (id),
    driver_account_id uuid,
    note              text NOT NULL DEFAULT '',
    status            text NOT NULL CHECK (status IN ('open','closed','superseded','cancelled')),
    supersedes_id     uuid REFERENCES trips.trip (id),
    change_reason     text,
    origin            text NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    created_by        uuid NOT NULL,
    updated_at        timestamptz NOT NULL DEFAULT now(),
    updated_by        uuid NOT NULL,
    recorded_at       timestamptz NOT NULL DEFAULT now(),
    version           integer NOT NULL DEFAULT 1,
    CHECK (ended_at IS NULL OR ended_at > started_at),                 -- I-TR-2
    CHECK ((status = 'open') = (ended_at IS NULL) OR status = 'cancelled')
);
CREATE INDEX trip_vehicle_idx ON trips.trip (vehicle_id, started_at DESC, id DESC);
CREATE INDEX trip_root_idx ON trips.trip (root_id);
-- höchstens eine laufende Fahrt je Fahrzeug (folgt aus I-TR-1)
CREATE UNIQUE INDEX trip_open_uq ON trips.trip (vehicle_id) WHERE status = 'open';

-- +goose Down
DROP SCHEMA trips CASCADE;
