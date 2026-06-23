-- Unifies TLS fingerprint capture persistence for Task 2:
-- - evolve capture task/sample tables to the session-aware schema
-- - create session/session_event tables
-- - keep all DDL idempotent for repeated startup migration runs

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE tls_fingerprint_capture_tasks
    ADD COLUMN IF NOT EXISTS transport_targets JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS transport_counts JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS capture_filters JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS sample_schema_version INT NOT NULL DEFAULT 2,
    ADD COLUMN IF NOT EXISTS task_stats JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE tls_fingerprint_capture_tasks
SET transport_targets = COALESCE(transport_targets, '{}'::jsonb),
    transport_counts = COALESCE(transport_counts, '{}'::jsonb),
    capture_filters = COALESCE(capture_filters, '{}'::jsonb),
    task_stats = COALESCE(task_stats, '{}'::jsonb),
    sample_schema_version = COALESCE(sample_schema_version, 2)
WHERE transport_targets IS NULL
   OR transport_counts IS NULL
   OR capture_filters IS NULL
   OR task_stats IS NULL
   OR sample_schema_version IS NULL;

ALTER TABLE tls_fingerprint_capture_samples
    ADD COLUMN IF NOT EXISTS session_id VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS transport VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS replay_hash VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS ja3_raw TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS ja3_hash VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS ja4 VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS request_path TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS http_method VARCHAR(16) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS is_websocket BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS websocket_protocol VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS client_type VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS model VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS request_kind VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS streaming BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS response_mode VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS http2_fingerprint TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS stainless_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS replay_profile JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

ALTER TABLE tls_fingerprint_capture_samples
    ALTER COLUMN fingerprint_hash TYPE VARCHAR(128);

UPDATE tls_fingerprint_capture_samples
SET replay_hash = CASE
        WHEN BTRIM(COALESCE(replay_hash, '')) <> '' THEN BTRIM(replay_hash)
        WHEN POSITION('::' IN COALESCE(fingerprint_hash, '')) > 0 THEN SPLIT_PART(fingerprint_hash, '::', 1)
        ELSE COALESCE(fingerprint_hash, '')
    END,
    transport = CASE
        WHEN BTRIM(COALESCE(transport, '')) <> '' THEN BTRIM(transport)
        WHEN POSITION('::' IN COALESCE(fingerprint_hash, '')) > 0 THEN SPLIT_PART(fingerprint_hash, '::', 2)
        ELSE COALESCE(transport, '')
    END,
    ja3_raw = COALESCE(ja3_raw, ''),
    ja3_hash = COALESCE(ja3_hash, ''),
    ja4 = COALESCE(ja4, ''),
    session_id = COALESCE(session_id, ''),
    request_path = COALESCE(request_path, ''),
    http_method = COALESCE(http_method, ''),
    websocket_protocol = COALESCE(websocket_protocol, ''),
    client_type = COALESCE(client_type, ''),
    model = COALESCE(model, ''),
    request_kind = COALESCE(request_kind, ''),
    response_mode = COALESCE(response_mode, ''),
    http2_fingerprint = COALESCE(http2_fingerprint, ''),
    stainless_metadata = COALESCE(stainless_metadata, '{}'::jsonb),
    replay_profile = COALESCE(replay_profile, '{}'::jsonb),
    captured_at = COALESCE(captured_at, created_at, NOW())
WHERE replay_hash IS NULL
   OR replay_hash = ''
   OR ja3_raw IS NULL
   OR ja3_hash IS NULL
   OR ja4 IS NULL
   OR transport IS NULL
   OR session_id IS NULL
   OR request_path IS NULL
   OR http_method IS NULL
   OR websocket_protocol IS NULL
   OR client_type IS NULL
   OR model IS NULL
   OR request_kind IS NULL
   OR response_mode IS NULL
   OR http2_fingerprint IS NULL
   OR stainless_metadata IS NULL
   OR replay_profile IS NULL
   OR captured_at IS NULL;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'tls_fingerprint_capture_samples'
          AND column_name = 'profile'
    ) THEN
        EXECUTE $sql$
            UPDATE tls_fingerprint_capture_samples
            SET replay_profile = COALESCE(NULLIF(replay_profile, '{}'::jsonb), profile, '{}'::jsonb)
            WHERE replay_profile = '{}'::jsonb
               OR replay_profile IS NULL
        $sql$;
    END IF;
END $$;

DROP INDEX IF EXISTS idx_tls_fp_capture_samples_task_hash;
DROP INDEX IF EXISTS idx_tls_fp_capture_samples_task_replay_transport;
CREATE UNIQUE INDEX IF NOT EXISTS idx_tls_fp_capture_samples_task_replay_transport_unique
    ON tls_fingerprint_capture_samples (task_id, replay_hash, transport);
CREATE INDEX IF NOT EXISTS idx_tls_fp_capture_samples_task_hash
    ON tls_fingerprint_capture_samples (task_id, fingerprint_hash);

CREATE INDEX IF NOT EXISTS idx_tls_fp_capture_samples_task_session_transport
    ON tls_fingerprint_capture_samples (task_id, session_id, transport);

CREATE TABLE IF NOT EXISTS tls_fingerprint_capture_sessions (
    id                    BIGSERIAL PRIMARY KEY,
    task_id               BIGINT       NOT NULL REFERENCES tls_fingerprint_capture_tasks(id) ON DELETE CASCADE,
    session_id            VARCHAR(128) NOT NULL,
    client_ip             VARCHAR(128) NOT NULL DEFAULT '',
    platform              VARCHAR(50)  NOT NULL DEFAULT '',
    user_agent            TEXT         NOT NULL DEFAULT '',
    originator            VARCHAR(50)  NOT NULL DEFAULT '',
    alpn_negotiated       VARCHAR(32)  NOT NULL DEFAULT '',
    raw_client_hello      BYTEA        NULL,
    observed_client_hello JSONB        NOT NULL DEFAULT '{}'::jsonb,
    replay_profile        JSONB        NOT NULL DEFAULT '{}'::jsonb,
    derived_fingerprint   JSONB        NOT NULL DEFAULT '{}'::jsonb,
    session_status        VARCHAR(64)  NOT NULL DEFAULT 'observed',
    error_summary         TEXT         NOT NULL DEFAULT '',
    opened_at             TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    closed_at             TIMESTAMPTZ  NULL,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

ALTER TABLE tls_fingerprint_capture_sessions
    ADD COLUMN IF NOT EXISTS client_ip VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS alpn_negotiated VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS raw_client_hello BYTEA NULL,
    ADD COLUMN IF NOT EXISTS observed_client_hello JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS replay_profile JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS derived_fingerprint JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS session_status VARCHAR(64) NOT NULL DEFAULT 'observed',
    ADD COLUMN IF NOT EXISTS error_summary TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS opened_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS closed_at TIMESTAMPTZ NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_tls_fp_capture_sessions_task_session
    ON tls_fingerprint_capture_sessions (task_id, session_id);

CREATE INDEX IF NOT EXISTS idx_tls_fp_capture_sessions_task_platform
    ON tls_fingerprint_capture_sessions (task_id, platform);

CREATE TABLE IF NOT EXISTS tls_fingerprint_capture_session_events (
    id                 BIGSERIAL PRIMARY KEY,
    task_id            BIGINT       NOT NULL REFERENCES tls_fingerprint_capture_tasks(id) ON DELETE CASCADE,
    session_ref        BIGINT       NULL REFERENCES tls_fingerprint_capture_sessions(id) ON DELETE SET NULL,
    session_id         VARCHAR(128) NOT NULL DEFAULT '',
    event_id           VARCHAR(128) NOT NULL DEFAULT '',
    platform           VARCHAR(50)  NOT NULL DEFAULT '',
    transport          VARCHAR(32)  NOT NULL DEFAULT '',
    event_type         VARCHAR(64)  NOT NULL,
    request_sequence   INT          NOT NULL DEFAULT 0,
    stream_id          VARCHAR(128) NOT NULL DEFAULT '',
    request_path       TEXT         NOT NULL DEFAULT '',
    http_method        VARCHAR(16)  NOT NULL DEFAULT '',
    is_websocket       BOOLEAN      NOT NULL DEFAULT FALSE,
    websocket_protocol VARCHAR(64)  NOT NULL DEFAULT '',
    client_type        VARCHAR(64)  NOT NULL DEFAULT '',
    model              VARCHAR(255) NOT NULL DEFAULT '',
    request_kind       VARCHAR(64)  NOT NULL DEFAULT '',
    streaming          BOOLEAN      NOT NULL DEFAULT FALSE,
    response_mode      VARCHAR(64)  NOT NULL DEFAULT '',
    user_agent         TEXT         NOT NULL DEFAULT '',
    originator         VARCHAR(50)  NOT NULL DEFAULT '',
    stainless_metadata JSONB        NOT NULL DEFAULT '{}'::jsonb,
    headers_snapshot   JSONB        NOT NULL DEFAULT '{}'::jsonb,
    body_summary       TEXT         NOT NULL DEFAULT '',
    event_status       VARCHAR(64)  NOT NULL DEFAULT 'observed',
    event_error        TEXT         NOT NULL DEFAULT '',
    replayable         BOOLEAN      NOT NULL DEFAULT TRUE,
    sample_id          BIGINT       NULL REFERENCES tls_fingerprint_capture_samples(id) ON DELETE SET NULL,
    replay_hash        VARCHAR(64)  NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

ALTER TABLE tls_fingerprint_capture_session_events
    ADD COLUMN IF NOT EXISTS event_id VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS request_sequence INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS stream_id VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS request_path TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS http_method VARCHAR(16) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS is_websocket BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS websocket_protocol VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS client_type VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS model VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS request_kind VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS streaming BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS response_mode VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS user_agent TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS originator VARCHAR(50) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS stainless_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS headers_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS body_summary TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS event_status VARCHAR(64) NOT NULL DEFAULT 'observed',
    ADD COLUMN IF NOT EXISTS event_error TEXT NOT NULL DEFAULT '';

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'tls_fingerprint_capture_session_events'
          AND column_name = 'error'
    ) AND NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'tls_fingerprint_capture_session_events'
          AND column_name = 'event_error'
    ) THEN
        ALTER TABLE tls_fingerprint_capture_session_events RENAME COLUMN error TO event_error;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_tls_fp_capture_session_events_task_session_ref_created
    ON tls_fingerprint_capture_session_events (task_id, session_ref, created_at);

CREATE INDEX IF NOT EXISTS idx_tls_fp_capture_session_events_task_session_id_created
    ON tls_fingerprint_capture_session_events (task_id, session_id, created_at);

CREATE INDEX IF NOT EXISTS idx_tls_fp_capture_session_events_task_sample
    ON tls_fingerprint_capture_session_events (task_id, sample_id);
