CREATE TABLE IF NOT EXISTS ai_sessions (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    user_id BIGINT NOT NULL,
    title VARCHAR(200) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    system_prompt TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_message_at TIMESTAMPTZ,
    request_id VARCHAR(64),
    usage_log_id BIGINT,
    api_key_id BIGINT,
    group_id BIGINT
);

CREATE TABLE IF NOT EXISTS ai_prompt_templates (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    user_id BIGINT NOT NULL,
    title VARCHAR(200) NOT NULL,
    description TEXT,
    category VARCHAR(100),
    tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    visibility VARCHAR(32) NOT NULL DEFAULT 'private',
    moderation_state VARCHAR(32) NOT NULL DEFAULT 'normal',
    current_version INTEGER NOT NULL DEFAULT 1,
    content TEXT NOT NULL DEFAULT '',
    model_hint VARCHAR(100),
    cover_asset_id BIGINT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_id VARCHAR(64),
    usage_log_id BIGINT,
    api_key_id BIGINT,
    group_id BIGINT
);

CREATE TABLE IF NOT EXISTS ai_prompt_template_versions (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_id BIGINT NOT NULL,
    version INTEGER NOT NULL,
    title VARCHAR(200) NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    model_hint VARCHAR(100),
    variables JSONB NOT NULL DEFAULT '[]'::jsonb,
    change_note TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_id VARCHAR(64),
    usage_log_id BIGINT,
    api_key_id BIGINT,
    group_id BIGINT,
    template_id BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS ai_generation_jobs (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_id BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'queued',
    model VARCHAR(100) NOT NULL,
    prompt TEXT NOT NULL DEFAULT '',
    negative_prompt TEXT,
    size VARCHAR(32),
    image_count INTEGER NOT NULL DEFAULT 1,
    seed BIGINT,
    error_message TEXT,
    parameters JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_id VARCHAR(64),
    usage_log_id BIGINT,
    api_key_id BIGINT,
    group_id BIGINT,
    prompt_template_id BIGINT,
    session_id BIGINT
);

CREATE TABLE IF NOT EXISTS ai_assets (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    user_id BIGINT NOT NULL,
    asset_type VARCHAR(32) NOT NULL DEFAULT 'image',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    visibility VARCHAR(32) NOT NULL DEFAULT 'private',
    moderation_state VARCHAR(32) NOT NULL DEFAULT 'normal',
    storage_kind VARCHAR(32),
    storage_path TEXT,
    source_url TEXT,
    mime_type VARCHAR(100),
    width INTEGER,
    height INTEGER,
    byte_size BIGINT,
    checksum VARCHAR(128),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_id VARCHAR(64),
    usage_log_id BIGINT,
    api_key_id BIGINT,
    group_id BIGINT,
    generation_job_id BIGINT,
    prompt_template_id BIGINT,
    session_id BIGINT
);

CREATE TABLE IF NOT EXISTS ai_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    operator_user_id BIGINT,
    owner_user_id BIGINT,
    entity_type VARCHAR(64) NOT NULL,
    entity_id BIGINT,
    action VARCHAR(64) NOT NULL,
    reason TEXT,
    before_state JSONB NOT NULL DEFAULT '{}'::jsonb,
    after_state JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_id VARCHAR(64),
    usage_log_id BIGINT,
    api_key_id BIGINT,
    group_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ai_session_messages (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_id BIGINT NOT NULL,
    reply_to_message_id BIGINT,
    role VARCHAR(32) NOT NULL DEFAULT 'user',
    status VARCHAR(32) NOT NULL DEFAULT 'accepted',
    content TEXT NOT NULL DEFAULT '',
    content_parts JSONB NOT NULL DEFAULT '[]'::jsonb,
    model VARCHAR(100),
    provider VARCHAR(50),
    error_message TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_id VARCHAR(64),
    usage_log_id BIGINT,
    api_key_id BIGINT,
    group_id BIGINT,
    session_id BIGINT NOT NULL
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ai_prompt_template_versions_ai_prompt_templates_versions'
    ) THEN
        ALTER TABLE ai_prompt_template_versions
            ADD CONSTRAINT ai_prompt_template_versions_ai_prompt_templates_versions
            FOREIGN KEY (template_id) REFERENCES ai_prompt_templates (id) ON DELETE NO ACTION;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ai_generation_jobs_ai_prompt_templates_generation_jobs'
    ) THEN
        ALTER TABLE ai_generation_jobs
            ADD CONSTRAINT ai_generation_jobs_ai_prompt_templates_generation_jobs
            FOREIGN KEY (prompt_template_id) REFERENCES ai_prompt_templates (id) ON DELETE SET NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ai_generation_jobs_ai_sessions_generation_jobs'
    ) THEN
        ALTER TABLE ai_generation_jobs
            ADD CONSTRAINT ai_generation_jobs_ai_sessions_generation_jobs
            FOREIGN KEY (session_id) REFERENCES ai_sessions (id) ON DELETE SET NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ai_assets_ai_generation_jobs_assets'
    ) THEN
        ALTER TABLE ai_assets
            ADD CONSTRAINT ai_assets_ai_generation_jobs_assets
            FOREIGN KEY (generation_job_id) REFERENCES ai_generation_jobs (id) ON DELETE SET NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ai_assets_ai_prompt_templates_assets'
    ) THEN
        ALTER TABLE ai_assets
            ADD CONSTRAINT ai_assets_ai_prompt_templates_assets
            FOREIGN KEY (prompt_template_id) REFERENCES ai_prompt_templates (id) ON DELETE SET NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ai_assets_ai_sessions_assets'
    ) THEN
        ALTER TABLE ai_assets
            ADD CONSTRAINT ai_assets_ai_sessions_assets
            FOREIGN KEY (session_id) REFERENCES ai_sessions (id) ON DELETE SET NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ai_session_messages_ai_sessions_messages'
    ) THEN
        ALTER TABLE ai_session_messages
            ADD CONSTRAINT ai_session_messages_ai_sessions_messages
            FOREIGN KEY (session_id) REFERENCES ai_sessions (id) ON DELETE NO ACTION;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS aisession_user_id
    ON ai_sessions (user_id);
CREATE INDEX IF NOT EXISTS aisession_status
    ON ai_sessions (status);
CREATE INDEX IF NOT EXISTS aisession_request_id
    ON ai_sessions (request_id);
CREATE INDEX IF NOT EXISTS aisession_usage_log_id
    ON ai_sessions (usage_log_id);
CREATE INDEX IF NOT EXISTS aisession_api_key_id
    ON ai_sessions (api_key_id);
CREATE INDEX IF NOT EXISTS aisession_group_id
    ON ai_sessions (group_id);
CREATE INDEX IF NOT EXISTS aisession_user_id_updated_at
    ON ai_sessions (user_id, updated_at);
CREATE INDEX IF NOT EXISTS aisession_user_id_last_message_at
    ON ai_sessions (user_id, last_message_at);

CREATE INDEX IF NOT EXISTS aisessionmessage_session_id
    ON ai_session_messages (session_id);
CREATE INDEX IF NOT EXISTS aisessionmessage_user_id
    ON ai_session_messages (user_id);
CREATE INDEX IF NOT EXISTS aisessionmessage_role
    ON ai_session_messages (role);
CREATE INDEX IF NOT EXISTS aisessionmessage_status
    ON ai_session_messages (status);
CREATE INDEX IF NOT EXISTS aisessionmessage_request_id
    ON ai_session_messages (request_id);
CREATE INDEX IF NOT EXISTS aisessionmessage_usage_log_id
    ON ai_session_messages (usage_log_id);
CREATE INDEX IF NOT EXISTS aisessionmessage_api_key_id
    ON ai_session_messages (api_key_id);
CREATE INDEX IF NOT EXISTS aisessionmessage_group_id
    ON ai_session_messages (group_id);
CREATE INDEX IF NOT EXISTS aisessionmessage_session_id_created_at
    ON ai_session_messages (session_id, created_at);

CREATE INDEX IF NOT EXISTS aiprompttemplate_user_id
    ON ai_prompt_templates (user_id);
CREATE INDEX IF NOT EXISTS aiprompttemplate_visibility
    ON ai_prompt_templates (visibility);
CREATE INDEX IF NOT EXISTS aiprompttemplate_moderation_state
    ON ai_prompt_templates (moderation_state);
CREATE INDEX IF NOT EXISTS aiprompttemplate_request_id
    ON ai_prompt_templates (request_id);
CREATE INDEX IF NOT EXISTS aiprompttemplate_usage_log_id
    ON ai_prompt_templates (usage_log_id);
CREATE INDEX IF NOT EXISTS aiprompttemplate_api_key_id
    ON ai_prompt_templates (api_key_id);
CREATE INDEX IF NOT EXISTS aiprompttemplate_group_id
    ON ai_prompt_templates (group_id);
CREATE INDEX IF NOT EXISTS aiprompttemplate_user_id_updated_at
    ON ai_prompt_templates (user_id, updated_at);
CREATE INDEX IF NOT EXISTS aiprompttemplate_visibility_moderation_state
    ON ai_prompt_templates (visibility, moderation_state);

CREATE INDEX IF NOT EXISTS aiprompttemplateversion_template_id
    ON ai_prompt_template_versions (template_id);
CREATE INDEX IF NOT EXISTS aiprompttemplateversion_user_id
    ON ai_prompt_template_versions (user_id);
CREATE INDEX IF NOT EXISTS aiprompttemplateversion_request_id
    ON ai_prompt_template_versions (request_id);
CREATE INDEX IF NOT EXISTS aiprompttemplateversion_usage_log_id
    ON ai_prompt_template_versions (usage_log_id);
CREATE INDEX IF NOT EXISTS aiprompttemplateversion_api_key_id
    ON ai_prompt_template_versions (api_key_id);
CREATE INDEX IF NOT EXISTS aiprompttemplateversion_group_id
    ON ai_prompt_template_versions (group_id);
CREATE UNIQUE INDEX IF NOT EXISTS aiprompttemplateversion_template_id_version
    ON ai_prompt_template_versions (template_id, version);

CREATE INDEX IF NOT EXISTS aigenerationjob_user_id
    ON ai_generation_jobs (user_id);
CREATE INDEX IF NOT EXISTS aigenerationjob_session_id
    ON ai_generation_jobs (session_id);
CREATE INDEX IF NOT EXISTS aigenerationjob_prompt_template_id
    ON ai_generation_jobs (prompt_template_id);
CREATE INDEX IF NOT EXISTS aigenerationjob_status
    ON ai_generation_jobs (status);
CREATE INDEX IF NOT EXISTS aigenerationjob_request_id
    ON ai_generation_jobs (request_id);
CREATE INDEX IF NOT EXISTS aigenerationjob_usage_log_id
    ON ai_generation_jobs (usage_log_id);
CREATE INDEX IF NOT EXISTS aigenerationjob_api_key_id
    ON ai_generation_jobs (api_key_id);
CREATE INDEX IF NOT EXISTS aigenerationjob_group_id
    ON ai_generation_jobs (group_id);
CREATE INDEX IF NOT EXISTS aigenerationjob_user_id_created_at
    ON ai_generation_jobs (user_id, created_at);

CREATE INDEX IF NOT EXISTS aiasset_user_id
    ON ai_assets (user_id);
CREATE INDEX IF NOT EXISTS aiasset_generation_job_id
    ON ai_assets (generation_job_id);
CREATE INDEX IF NOT EXISTS aiasset_session_id
    ON ai_assets (session_id);
CREATE INDEX IF NOT EXISTS aiasset_prompt_template_id
    ON ai_assets (prompt_template_id);
CREATE INDEX IF NOT EXISTS aiasset_status
    ON ai_assets (status);
CREATE INDEX IF NOT EXISTS aiasset_visibility
    ON ai_assets (visibility);
CREATE INDEX IF NOT EXISTS aiasset_moderation_state
    ON ai_assets (moderation_state);
CREATE INDEX IF NOT EXISTS aiasset_request_id
    ON ai_assets (request_id);
CREATE INDEX IF NOT EXISTS aiasset_usage_log_id
    ON ai_assets (usage_log_id);
CREATE INDEX IF NOT EXISTS aiasset_api_key_id
    ON ai_assets (api_key_id);
CREATE INDEX IF NOT EXISTS aiasset_group_id
    ON ai_assets (group_id);
CREATE INDEX IF NOT EXISTS aiasset_user_id_created_at
    ON ai_assets (user_id, created_at);

CREATE INDEX IF NOT EXISTS aiauditlog_operator_user_id
    ON ai_audit_logs (operator_user_id);
CREATE INDEX IF NOT EXISTS aiauditlog_owner_user_id
    ON ai_audit_logs (owner_user_id);
CREATE INDEX IF NOT EXISTS aiauditlog_entity_type
    ON ai_audit_logs (entity_type);
CREATE INDEX IF NOT EXISTS aiauditlog_entity_id
    ON ai_audit_logs (entity_id);
CREATE INDEX IF NOT EXISTS aiauditlog_action
    ON ai_audit_logs (action);
CREATE INDEX IF NOT EXISTS aiauditlog_request_id
    ON ai_audit_logs (request_id);
CREATE INDEX IF NOT EXISTS aiauditlog_usage_log_id
    ON ai_audit_logs (usage_log_id);
CREATE INDEX IF NOT EXISTS aiauditlog_api_key_id
    ON ai_audit_logs (api_key_id);
CREATE INDEX IF NOT EXISTS aiauditlog_group_id
    ON ai_audit_logs (group_id);
CREATE INDEX IF NOT EXISTS aiauditlog_created_at
    ON ai_audit_logs (created_at);
CREATE INDEX IF NOT EXISTS aiauditlog_entity_type_entity_id_created_at
    ON ai_audit_logs (entity_type, entity_id, created_at);
