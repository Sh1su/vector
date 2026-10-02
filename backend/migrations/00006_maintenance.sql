-- +goose Up
CREATE SCHEMA maintenance;

CREATE TABLE maintenance.item (
    id                       uuid PRIMARY KEY,
    vehicle_id               uuid NOT NULL REFERENCES vehicles.vehicle (id),
    title                    text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    description              text,
    category                 text NOT NULL CHECK (category IN ('service','legal_inspection','tires','fluids','brakes','filters','other')),
    manufacturer_recommended boolean NOT NULL DEFAULT false,
    source_document_id       uuid,
    source_page              integer CHECK (source_page >= 1),
    schedule_mode            text NOT NULL CHECK (schedule_mode IN ('once','from_last_completion','fixed_grid')),
    interval_months          integer CHECK (interval_months > 0),
    interval_days            integer CHECK (interval_days > 0),
    interval_distance        bigint CHECK (interval_distance > 0),
    anchor_date              date,
    anchor_total             bigint CHECK (anchor_total >= 0),
    due_date_once            date,
    due_total_once           bigint CHECK (due_total_once >= 0),
    upcoming_days            integer CHECK (upcoming_days >= 0),
    due_days                 integer CHECK (due_days >= 0),
    upcoming_distance        bigint CHECK (upcoming_distance >= 0),
    due_distance             bigint CHECK (due_distance >= 0),
    inputs                   jsonb NOT NULL DEFAULT '{}'::jsonb, -- Originaleingaben der Mengen (ADR-007)
    active                   boolean NOT NULL DEFAULT true,
    note                     text NOT NULL DEFAULT '',
    origin                   text NOT NULL,
    created_at               timestamptz NOT NULL DEFAULT now(),
    created_by               uuid NOT NULL,
    updated_at               timestamptz NOT NULL DEFAULT now(),
    updated_by               uuid NOT NULL,
    recorded_at              timestamptz NOT NULL DEFAULT now(),
    deleted_at               timestamptz,
    version                  integer NOT NULL DEFAULT 1,
    CHECK (interval_months IS NULL OR interval_days IS NULL),
    -- I-MA-1: mindestens ein Auslöser
    CHECK (CASE WHEN schedule_mode = 'once' THEN due_date_once IS NOT NULL OR due_total_once IS NOT NULL
                ELSE interval_months IS NOT NULL OR interval_days IS NOT NULL OR interval_distance IS NOT NULL END)
);
CREATE INDEX item_vehicle_idx ON maintenance.item (vehicle_id) WHERE deleted_at IS NULL;

CREATE TABLE maintenance.completion (
    id               uuid PRIMARY KEY,
    item_id          uuid NOT NULL REFERENCES maintenance.item (id),
    vehicle_id       uuid NOT NULL REFERENCES vehicles.vehicle (id),
    kind             text NOT NULL CHECK (kind IN ('done','skipped')),
    completed_on     date NOT NULL,
    completed_total  bigint CHECK (completed_total >= 0),
    completed_input  jsonb,
    service_entry_id uuid,
    reason           text,
    idempotency_key  text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    created_by       uuid NOT NULL,
    deleted_at       timestamptz,
    CHECK (kind = 'done' OR (reason IS NOT NULL AND length(reason) > 0))
);
CREATE UNIQUE INDEX completion_service_uq ON maintenance.completion (item_id, service_entry_id) WHERE service_entry_id IS NOT NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX completion_idem_uq ON maintenance.completion (item_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX completion_item_idx ON maintenance.completion (item_id) WHERE deleted_at IS NULL;

-- +goose Down
DROP SCHEMA maintenance CASCADE;
