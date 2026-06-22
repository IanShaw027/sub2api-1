-- Adds optional upstream User-Agent and Originator metadata to TLS fingerprint
-- profiles so an applied template can drive those request headers.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE tls_fingerprint_profiles
    ADD COLUMN IF NOT EXISTS user_agent VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS originator VARCHAR(50) NOT NULL DEFAULT '';

COMMENT ON COLUMN tls_fingerprint_profiles.user_agent IS 'Optional upstream User-Agent sent when this template is applied; empty falls back to the built-in default';
COMMENT ON COLUMN tls_fingerprint_profiles.originator IS 'Optional upstream Originator header sent when this template is applied; empty falls back to the built-in default';
