-- Stores replay parameters for TLS extensions that cannot be reproduced from
-- extension IDs alone.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE tls_fingerprint_profiles
    ADD COLUMN IF NOT EXISTS compress_cert_algos JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS delegated_credentials_algorithms JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS application_settings_protocols JSONB NOT NULL DEFAULT '[]'::jsonb;

COMMENT ON COLUMN tls_fingerprint_profiles.compress_cert_algos IS 'compress_certificate(27) algorithms in ClientHello order';
COMMENT ON COLUMN tls_fingerprint_profiles.delegated_credentials_algorithms IS 'delegated_credentials(34) signature algorithms in ClientHello order';
COMMENT ON COLUMN tls_fingerprint_profiles.application_settings_protocols IS 'ALPS/application_settings(17513/17613) protocol list';
