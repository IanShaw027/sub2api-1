-- Updates capture sample comments to match replayable request-observation semantics.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

COMMENT ON TABLE tls_fingerprint_capture_samples IS
    'Unique replayable capture request observations per capture task';

COMMENT ON COLUMN tls_fingerprint_capture_samples.fingerprint_hash IS
    'SHA-256 over replayable request observation fields used for sample uniqueness; replay_hash stores the TLS-only replay fingerprint';

COMMENT ON COLUMN tls_fingerprint_capture_samples.replay_hash IS
    'TLS replay fingerprint derived from ClientHello-only replay characteristics';
