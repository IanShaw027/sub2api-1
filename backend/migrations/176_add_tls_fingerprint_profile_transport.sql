-- +migrate Up
ALTER TABLE tls_fingerprint_profiles ADD COLUMN IF NOT EXISTS transport VARCHAR(20) NOT NULL DEFAULT '';

-- +migrate Down
ALTER TABLE tls_fingerprint_profiles DROP COLUMN IF EXISTS transport;
