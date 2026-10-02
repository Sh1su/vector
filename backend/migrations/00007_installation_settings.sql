-- +goose Up
-- Einstellungen der Installation (Schema InstallationSettings), genau eine Zeile.
CREATE TABLE identity.installation_settings (
    id         boolean PRIMARY KEY DEFAULT true CHECK (id),
    settings   jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by uuid,
    version    integer NOT NULL DEFAULT 1
);
INSERT INTO identity.installation_settings (id) VALUES (true);

-- +goose Down
DROP TABLE identity.installation_settings;
