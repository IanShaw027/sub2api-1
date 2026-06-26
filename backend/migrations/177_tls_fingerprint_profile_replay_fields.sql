-- Persists replay-only TLS fields that became mandatory after Task 1 replay unification.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE tls_fingerprint_profiles
    ADD COLUMN IF NOT EXISTS signature_algorithms_cert JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS extension_payloads JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN tls_fingerprint_profiles.signature_algorithms_cert IS 'signature_algorithms_cert(50) values required for faithful replay';
COMMENT ON COLUMN tls_fingerprint_profiles.extension_payloads IS 'Unknown/generic TLS extension payloads required for canonical replay';
