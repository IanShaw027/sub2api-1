-- Adds platform classification for TLS fingerprint templates and live capture task tables.
-- Capture dedupe is intentionally based only on the replayable TLS fingerprint hash.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE tls_fingerprint_profiles
    ADD COLUMN IF NOT EXISTS platform VARCHAR(50) NOT NULL DEFAULT '';

COMMENT ON COLUMN tls_fingerprint_profiles.platform IS 'Optional platform classification; empty means shared by all platforms';

CREATE TABLE IF NOT EXISTS tls_fingerprint_capture_tasks (
    id           BIGSERIAL PRIMARY KEY,
    name         VARCHAR(120) NOT NULL DEFAULT 'TLS fingerprint capture',
    status       VARCHAR(20)  NOT NULL DEFAULT 'running',
    token        VARCHAR(96)  NOT NULL UNIQUE,
    targets      JSONB        NOT NULL DEFAULT '{}'::jsonb,
    counts       JSONB        NOT NULL DEFAULT '{}'::jsonb,
    ua_keywords  JSONB        NOT NULL DEFAULT '[]'::jsonb,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_tls_fp_capture_tasks_status_created
    ON tls_fingerprint_capture_tasks (status, created_at DESC);

COMMENT ON TABLE tls_fingerprint_capture_tasks IS 'Live TLS fingerprint capture tasks';
COMMENT ON COLUMN tls_fingerprint_capture_tasks.targets IS 'Per-platform target counts, e.g. {"openai": 100}';
COMMENT ON COLUMN tls_fingerprint_capture_tasks.ua_keywords IS 'Inbound User-Agent substrings used only for capture matching, not dedupe';

CREATE TABLE IF NOT EXISTS tls_fingerprint_capture_samples (
    id               BIGSERIAL PRIMARY KEY,
    task_id          BIGINT       NOT NULL REFERENCES tls_fingerprint_capture_tasks(id) ON DELETE CASCADE,
    platform         VARCHAR(50)  NOT NULL DEFAULT '',
    user_agent       TEXT         NOT NULL DEFAULT '',
    fingerprint_hash VARCHAR(64)  NOT NULL,
    profile          JSONB        NOT NULL,
    raw_payload      TEXT         NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tls_fp_capture_samples_task_hash
    ON tls_fingerprint_capture_samples (task_id, fingerprint_hash);

CREATE INDEX IF NOT EXISTS idx_tls_fp_capture_samples_task_platform
    ON tls_fingerprint_capture_samples (task_id, platform);

COMMENT ON TABLE tls_fingerprint_capture_samples IS 'Unique captured TLS fingerprints per capture task';
COMMENT ON COLUMN tls_fingerprint_capture_samples.fingerprint_hash IS 'SHA-256 over replayable TLS fields only; platform and User-Agent are excluded';
