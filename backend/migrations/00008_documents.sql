-- +goose Up
CREATE SCHEMA documents;

-- Dateien (ADR-017/018): Binärdaten nur im Storage, hier Metadaten; Original unveränderlich (I-DO-2).
CREATE TABLE documents.file (
    id                 uuid PRIMARY KEY,
    vehicle_id         uuid NOT NULL REFERENCES vehicles.vehicle (id),
    storage_key        text NOT NULL UNIQUE,
    original_name      text NOT NULL CHECK (length(original_name) BETWEEN 1 AND 255),
    media_type         text NOT NULL,
    size_bytes         bigint NOT NULL CHECK (size_bytes >= 0),
    sha256             text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    received_at        timestamptz NOT NULL DEFAULT now(),
    captured_at_client timestamptz,
    capture_source     text CHECK (capture_source IN ('camera','gallery','scanner','upload','import')),
    derivatives        text[] NOT NULL DEFAULT '{}',
    has_location       boolean NOT NULL DEFAULT false,
    supersedes_id      uuid REFERENCES documents.file (id),
    replace_reason     text,
    upload_time_origin text NOT NULL DEFAULT 'upload' CHECK (upload_time_origin IN ('upload','import')),
    created_by         uuid NOT NULL,
    deleted_at         timestamptz
);
CREATE INDEX file_vehicle_idx ON documents.file (vehicle_id, received_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX file_sha_idx ON documents.file (vehicle_id, sha256) WHERE deleted_at IS NULL;

-- Fahrzeugakte (Auftrag 6.11).
CREATE TABLE documents.document (
    id            uuid PRIMARY KEY,
    vehicle_id    uuid NOT NULL REFERENCES vehicles.vehicle (id),
    doc_type      text NOT NULL CHECK (doc_type IN ('maintenance_manual','owner_manual','technical_doc','service_book','invoice',
                      'workshop_report','inspection_report','registration','insurance','other')),
    nature        text NOT NULL CHECK (nature IN ('specification','record','other')),
    title         text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    document_date date,
    issuer        text CHECK (length(issuer) <= 200),
    language      text CHECK (language ~ '^[a-z]{2}$'),
    note          text NOT NULL DEFAULT '',
    tags          text[] NOT NULL DEFAULT '{}',
    origin        text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    created_by    uuid NOT NULL,
    updated_at    timestamptz NOT NULL DEFAULT now(),
    updated_by    uuid NOT NULL,
    recorded_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz,
    version       integer NOT NULL DEFAULT 1
);
CREATE INDEX document_vehicle_idx ON documents.document (vehicle_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;

-- Seiten eines Dokuments in Reihenfolge (I-DO-4).
CREATE TABLE documents.document_file (
    document_id uuid NOT NULL REFERENCES documents.document (id) ON DELETE CASCADE,
    position    integer NOT NULL,
    file_id     uuid NOT NULL REFERENCES documents.file (id),
    PRIMARY KEY (document_id, position)
);
CREATE INDEX document_file_file_idx ON documents.document_file (file_id);

-- Verknüpfungen mit Einträgen anderer Module (DO-01).
CREATE TABLE documents.attachment (
    id          uuid PRIMARY KEY,
    vehicle_id  uuid NOT NULL REFERENCES vehicles.vehicle (id),
    file_id     uuid REFERENCES documents.file (id),
    document_id uuid REFERENCES documents.document (id),
    target_type text NOT NULL CHECK (target_type IN ('odometer_reading','fuel_fill','oil_entry','service_entry','cost_entry','trip','maintenance_item','note')),
    target_id   uuid NOT NULL,
    role        text NOT NULL CHECK (length(role) BETWEEN 1 AND 40),
    page        integer CHECK (page >= 1),
    created_at  timestamptz NOT NULL DEFAULT now(),
    created_by  uuid NOT NULL,
    deleted_at  timestamptz,
    CHECK ((file_id IS NULL) <> (document_id IS NULL))
);
CREATE INDEX attachment_target_idx ON documents.attachment (target_type, target_id) WHERE deleted_at IS NULL;
CREATE INDEX attachment_file_idx ON documents.attachment (file_id) WHERE deleted_at IS NULL;

-- Fahrzeugbilder (Datei als Bild des Fahrzeugs).
CREATE TABLE documents.vehicle_image (
    id         uuid PRIMARY KEY,
    vehicle_id uuid NOT NULL REFERENCES vehicles.vehicle (id),
    file_id    uuid NOT NULL REFERENCES documents.file (id),
    is_primary boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid NOT NULL,
    UNIQUE (vehicle_id, file_id)
);
CREATE UNIQUE INDEX vehicle_image_primary_uq ON documents.vehicle_image (vehicle_id) WHERE is_primary;

-- +goose Down
DROP SCHEMA documents CASCADE;
