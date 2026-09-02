-- Creation center tables and supporting schema updates.

CREATE TABLE IF NOT EXISTS creation_sessions (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id    BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    title       VARCHAR(200) NOT NULL DEFAULT '',
    model       VARCHAR(100) NOT NULL DEFAULT '',
    mode        VARCHAR(16) NOT NULL DEFAULT 'chat',
    status      VARCHAR(32) NOT NULL DEFAULT 'active',
    metadata    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT creation_sessions_mode_check CHECK (mode IN ('chat', 'image')),
    CONSTRAINT creation_sessions_status_check CHECK (status IN ('active', 'archived'))
);

CREATE INDEX IF NOT EXISTS creation_sessions_user_id_idx ON creation_sessions (user_id);
CREATE INDEX IF NOT EXISTS creation_sessions_group_id_idx ON creation_sessions (group_id);
CREATE INDEX IF NOT EXISTS creation_sessions_user_id_status_idx ON creation_sessions (user_id, status);
CREATE INDEX IF NOT EXISTS creation_sessions_user_id_updated_at_idx ON creation_sessions (user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS creation_messages (
    id            BIGSERIAL PRIMARY KEY,
    session_id    BIGINT NOT NULL REFERENCES creation_sessions(id) ON DELETE CASCADE,
    role          VARCHAR(32) NOT NULL,
    content       JSONB NOT NULL,
    model         VARCHAR(100),
    input_tokens  INTEGER,
    output_tokens INTEGER,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT creation_messages_role_check CHECK (role IN ('user', 'assistant', 'system'))
);

CREATE INDEX IF NOT EXISTS creation_messages_session_id_idx ON creation_messages (session_id);
CREATE INDEX IF NOT EXISTS creation_messages_session_id_created_at_idx ON creation_messages (session_id, created_at);

CREATE TABLE IF NOT EXISTS creation_image_jobs (
    id                BIGSERIAL PRIMARY KEY,
    session_id        BIGINT REFERENCES creation_sessions(id) ON DELETE SET NULL,
    user_id           BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id          BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    status            VARCHAR(32) NOT NULL DEFAULT 'pending',
    model             VARCHAR(100) NOT NULL DEFAULT '',
    prompt            TEXT NOT NULL DEFAULT '',
    media_asset_id    BIGINT REFERENCES media_assets(id) ON DELETE SET NULL,
    provider_task_id  VARCHAR(128),
    error             TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT creation_image_jobs_status_check CHECK (status IN ('pending', 'processing', 'completed', 'failed'))
);

CREATE INDEX IF NOT EXISTS creation_image_jobs_user_id_idx ON creation_image_jobs (user_id);
CREATE INDEX IF NOT EXISTS creation_image_jobs_session_id_idx ON creation_image_jobs (session_id);
CREATE INDEX IF NOT EXISTS creation_image_jobs_user_id_created_at_idx ON creation_image_jobs (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS creation_image_jobs_provider_task_id_idx ON creation_image_jobs (provider_task_id);

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS purpose VARCHAR(32) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS api_keys_user_id_group_id_purpose_idx
    ON api_keys (user_id, group_id, purpose)
    WHERE deleted_at IS NULL;

ALTER TABLE media_assets
    ADD CONSTRAINT media_assets_biz_type_check_v3
    CHECK (biz_type IN (
        'invoice',
        'ticket',
        'avatar',
        'site_logo',
        'support_qr',
        'announcement',
        'payment_help',
        'image_task',
        'creation_image'
    )) NOT VALID;

ALTER TABLE media_assets
    VALIDATE CONSTRAINT media_assets_biz_type_check_v3;

ALTER TABLE media_assets
    DROP CONSTRAINT IF EXISTS media_assets_biz_type_check;

ALTER TABLE media_assets
    RENAME CONSTRAINT media_assets_biz_type_check_v3 TO media_assets_biz_type_check;
