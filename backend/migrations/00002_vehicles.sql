-- +goose Up
CREATE TABLE vehicles.vehicle (
    id                 uuid PRIMARY KEY,
    display_name       text NOT NULL CHECK (length(display_name) BETWEEN 1 AND 80),
    vin                text,
    license_plate      text,
    plate_country      text CHECK (plate_country ~ '^[A-Z]{2}$'),
    make               text,
    model              text,
    variant            text,
    model_year         integer CHECK (model_year BETWEEN 1885 AND 2100),
    first_registration date,
    body_type          text NOT NULL CHECK (body_type IN ('car','motorcycle','van','truck','camper','trailer','tractor','boat','other')),
    engine_code        text,
    displacement_ccm   integer CHECK (displacement_ccm >= 0),
    power_kw           integer CHECK (power_kw >= 0),
    transmission       text CHECK (transmission IN ('manual','automatic','other')),
    usage_meter        text NOT NULL CHECK (usage_meter IN ('distance','engine_hours')),
    energy_carriers    text[] NOT NULL DEFAULT '{}',
    odometer_required  boolean NOT NULL DEFAULT true,
    owner_time_zone    text NOT NULL,
    default_currency   char(3) NOT NULL CHECK (default_currency ~ '^[A-Z]{3}$'),
    status             text NOT NULL DEFAULT 'active' CHECK (status IN ('active','sold','archived')),
    purchase_date      date,
    purchase_amount    bigint,
    purchase_currency  char(3),
    sale_date          date,
    sale_amount        bigint,
    sale_currency      char(3),
    note               text NOT NULL DEFAULT '',
    tags               text[] NOT NULL DEFAULT '{}',
    extra              jsonb NOT NULL DEFAULT '{}'::jsonb,
    origin             text NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    created_by         uuid NOT NULL,
    updated_at         timestamptz NOT NULL DEFAULT now(),
    updated_by         uuid NOT NULL,
    recorded_at        timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz,
    version            integer NOT NULL DEFAULT 1,
    CHECK (sale_date IS NULL OR purchase_date IS NULL OR sale_date >= purchase_date),
    CHECK ((status = 'sold') = (sale_date IS NOT NULL))
);

CREATE TABLE identity.vehicle_membership (
    vehicle_id uuid NOT NULL REFERENCES vehicles.vehicle (id),
    account_id uuid NOT NULL REFERENCES identity.account (id),
    role       text NOT NULL CHECK (role IN ('owner','editor','viewer')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (vehicle_id, account_id)
);
CREATE INDEX membership_account_idx ON identity.vehicle_membership (account_id);

CREATE TABLE audit.event (
    id               uuid PRIMARY KEY,
    occurred_at      timestamptz NOT NULL DEFAULT now(),
    actor_account_id uuid,
    actor_kind       text NOT NULL CHECK (actor_kind IN ('user','api_token','assistant','import','system')),
    action           text NOT NULL,
    vehicle_id       uuid,
    object_type      text NOT NULL,
    object_id        uuid NOT NULL,
    changes          jsonb NOT NULL DEFAULT '{}'::jsonb,
    reason           text,
    request_id       text
);
CREATE INDEX audit_vehicle_idx ON audit.event (vehicle_id, occurred_at DESC);

-- +goose Down
DROP TABLE audit.event;
DROP TABLE identity.vehicle_membership;
DROP TABLE vehicles.vehicle;
