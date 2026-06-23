-- Persists raw request payload for every capture session event so multi-turn samples remain fully replayable.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE tls_fingerprint_capture_session_events
    ADD COLUMN IF NOT EXISTS raw_payload TEXT NOT NULL DEFAULT '';
