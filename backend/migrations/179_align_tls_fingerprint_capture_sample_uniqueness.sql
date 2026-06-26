-- Aligns capture sample uniqueness with replayable request observations and backfills
-- missing session_event columns for upgraded databases.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

DROP INDEX IF EXISTS idx_tls_fp_capture_samples_task_replay_transport_unique;
DROP INDEX IF EXISTS idx_tls_fp_capture_samples_task_hash;

CREATE UNIQUE INDEX IF NOT EXISTS idx_tls_fp_capture_samples_task_hash_unique
    ON tls_fingerprint_capture_samples (task_id, fingerprint_hash);

CREATE INDEX IF NOT EXISTS idx_tls_fp_capture_samples_task_replay_transport_platform
    ON tls_fingerprint_capture_samples (task_id, replay_hash, transport, platform);

ALTER TABLE tls_fingerprint_capture_session_events
    ADD COLUMN IF NOT EXISTS session_ref BIGINT NULL REFERENCES tls_fingerprint_capture_sessions(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS platform VARCHAR(50) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS transport VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS event_type VARCHAR(64) NOT NULL DEFAULT 'session_observed',
    ADD COLUMN IF NOT EXISTS replayable BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS sample_id BIGINT NULL REFERENCES tls_fingerprint_capture_samples(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS replay_hash VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS raw_payload TEXT NOT NULL DEFAULT '';
