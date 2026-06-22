-- Adds native TLS capture metadata columns used by the capture sample repository.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE tls_fingerprint_capture_samples
    ADD COLUMN IF NOT EXISTS originator VARCHAR(50) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS raw_client_hello BYTEA;

COMMENT ON COLUMN tls_fingerprint_capture_samples.originator IS 'Client-provided capture origin marker, e.g. SDK or tool name';
COMMENT ON COLUMN tls_fingerprint_capture_samples.raw_client_hello IS 'Raw TLS ClientHello record bytes captured by native capture listener';
