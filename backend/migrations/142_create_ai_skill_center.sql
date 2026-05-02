CREATE TABLE IF NOT EXISTS ai_skills (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    skill_type VARCHAR(32) NOT NULL,
    title VARCHAR(200) NOT NULL,
    summary TEXT,
    description TEXT,
    category VARCHAR(100),
    tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    visibility VARCHAR(32) NOT NULL DEFAULT 'private',
    source_visibility VARCHAR(32) NOT NULL DEFAULT 'public',
    billing_mode VARCHAR(32) NOT NULL DEFAULT 'per_request',
    price DECIMAL(20,8) NOT NULL DEFAULT 0,
    current_version_id BIGINT,
    published_version_id BIGINT,
    latest_approved_version_id BIGINT,
    latest_version INTEGER NOT NULL DEFAULT 0,
    like_count INTEGER NOT NULL DEFAULT 0,
    run_count INTEGER NOT NULL DEFAULT 0,
    total_income DECIMAL(20,8) NOT NULL DEFAULT 0,
    cover_asset_id BIGINT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_id VARCHAR(64),
    usage_log_id BIGINT,
    api_key_id BIGINT,
    group_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT chk_ai_skills_skill_type
        CHECK (skill_type IN ('prompt_chat', 'prompt_image', 'script')),
    CONSTRAINT chk_ai_skills_visibility
        CHECK (visibility IN ('private', 'unlisted', 'public')),
    CONSTRAINT chk_ai_skills_source_visibility
        CHECK (source_visibility IN ('public', 'hidden')),
    CONSTRAINT chk_ai_skills_billing_mode
        CHECK (billing_mode IN ('per_request')),
    CONSTRAINT chk_ai_skills_non_negative_price
        CHECK (price >= 0),
    CONSTRAINT chk_ai_skills_non_negative_latest_version
        CHECK (latest_version >= 0),
    CONSTRAINT chk_ai_skills_non_negative_like_count
        CHECK (like_count >= 0),
    CONSTRAINT chk_ai_skills_non_negative_run_count
        CHECK (run_count >= 0),
    CONSTRAINT chk_ai_skills_non_negative_total_income
        CHECK (total_income >= 0)
);

CREATE TABLE IF NOT EXISTS ai_skill_versions (
    id BIGSERIAL PRIMARY KEY,
    skill_id BIGINT NOT NULL REFERENCES ai_skills(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    review_status VARCHAR(32) NOT NULL DEFAULT 'draft',
    content_format VARCHAR(64),
    runtime VARCHAR(64),
    source_content TEXT NOT NULL DEFAULT '',
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    input_schema JSONB NOT NULL DEFAULT '{}'::jsonb,
    output_schema JSONB NOT NULL DEFAULT '{}'::jsonb,
    change_note TEXT,
    submitted_at TIMESTAMPTZ,
    reviewed_at TIMESTAMPTZ,
    reviewer_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    review_note TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_id VARCHAR(64),
    usage_log_id BIGINT,
    api_key_id BIGINT,
    group_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT chk_ai_skill_versions_version_positive
        CHECK (version > 0),
    CONSTRAINT chk_ai_skill_versions_review_status
        CHECK (review_status IN ('draft', 'pending', 'approved', 'rejected'))
);

CREATE TABLE IF NOT EXISTS ai_skill_reviews (
    id BIGSERIAL PRIMARY KEY,
    skill_id BIGINT NOT NULL REFERENCES ai_skills(id) ON DELETE CASCADE,
    version_id BIGINT NOT NULL REFERENCES ai_skill_versions(id) ON DELETE CASCADE,
    submitter_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reviewer_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    submit_note TEXT,
    review_note TEXT,
    snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    reviewed_at TIMESTAMPTZ,
    request_id VARCHAR(64),
    usage_log_id BIGINT,
    api_key_id BIGINT,
    group_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_ai_skill_reviews_status
        CHECK (status IN ('pending', 'approved', 'rejected', 'canceled'))
);

CREATE TABLE IF NOT EXISTS ai_skill_runs (
    id BIGSERIAL PRIMARY KEY,
    skill_id BIGINT NOT NULL REFERENCES ai_skills(id) ON DELETE CASCADE,
    version_id BIGINT NOT NULL REFERENCES ai_skill_versions(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    run_mode VARCHAR(32) NOT NULL DEFAULT 'use',
    status VARCHAR(32) NOT NULL DEFAULT 'queued',
    billing_mode VARCHAR(32) NOT NULL DEFAULT 'per_request',
    price DECIMAL(20,8) NOT NULL DEFAULT 0,
    input_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    output_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_message TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_id VARCHAR(64),
    usage_log_id BIGINT,
    api_key_id BIGINT,
    group_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_ai_skill_runs_mode
        CHECK (run_mode IN ('test', 'use')),
    CONSTRAINT chk_ai_skill_runs_status
        CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'canceled')),
    CONSTRAINT chk_ai_skill_runs_billing_mode
        CHECK (billing_mode IN ('per_request')),
    CONSTRAINT chk_ai_skill_runs_non_negative_price
        CHECK (price >= 0)
);

CREATE TABLE IF NOT EXISTS ai_skill_likes (
    id BIGSERIAL PRIMARY KEY,
    skill_id BIGINT NOT NULL REFERENCES ai_skills(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    request_id VARCHAR(64),
    usage_log_id BIGINT,
    api_key_id BIGINT,
    group_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ai_skill_settlements (
    id BIGSERIAL PRIMARY KEY,
    skill_id BIGINT NOT NULL REFERENCES ai_skills(id) ON DELETE CASCADE,
    version_id BIGINT NOT NULL REFERENCES ai_skill_versions(id) ON DELETE CASCADE,
    run_id BIGINT NOT NULL REFERENCES ai_skill_runs(id) ON DELETE CASCADE,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    buyer_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    billing_mode VARCHAR(32) NOT NULL DEFAULT 'per_request',
    amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    quota_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    quota_applied_at TIMESTAMPTZ,
    balance_transferred_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_id VARCHAR(64),
    usage_log_id BIGINT,
    api_key_id BIGINT,
    group_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_ai_skill_settlements_status
        CHECK (status IN ('pending', 'quota_accrued', 'transferred', 'canceled')),
    CONSTRAINT chk_ai_skill_settlements_billing_mode
        CHECK (billing_mode IN ('per_request')),
    CONSTRAINT chk_ai_skill_settlements_non_negative_amount
        CHECK (amount >= 0),
    CONSTRAINT chk_ai_skill_settlements_non_negative_quota_amount
        CHECK (quota_amount >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_skill_versions_skill_version
    ON ai_skill_versions (skill_id, version);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_skill_reviews_pending_version
    ON ai_skill_reviews (version_id)
    WHERE status = 'pending';

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_skill_likes_skill_user
    ON ai_skill_likes (skill_id, user_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_skill_settlements_run_id
    ON ai_skill_settlements (run_id);

CREATE INDEX IF NOT EXISTS idx_ai_skills_user_id
    ON ai_skills (user_id);

CREATE INDEX IF NOT EXISTS idx_ai_skills_skill_type
    ON ai_skills (skill_type);

CREATE INDEX IF NOT EXISTS idx_ai_skills_visibility_published
    ON ai_skills (visibility, published_version_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_ai_skills_group_id
    ON ai_skills (group_id)
    WHERE group_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ai_skills_updated_at
    ON ai_skills (updated_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_ai_skill_versions_skill_id
    ON ai_skill_versions (skill_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_ai_skill_versions_review_status
    ON ai_skill_versions (review_status)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_ai_skill_versions_reviewed_at
    ON ai_skill_versions (reviewed_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_ai_skill_reviews_skill_id
    ON ai_skill_reviews (skill_id);

CREATE INDEX IF NOT EXISTS idx_ai_skill_reviews_reviewer_status
    ON ai_skill_reviews (reviewer_user_id, status)
    WHERE reviewer_user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ai_skill_runs_skill_id
    ON ai_skill_runs (skill_id);

CREATE INDEX IF NOT EXISTS idx_ai_skill_runs_version_id
    ON ai_skill_runs (version_id);

CREATE INDEX IF NOT EXISTS idx_ai_skill_runs_user_created_at
    ON ai_skill_runs (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ai_skill_runs_status
    ON ai_skill_runs (status);

CREATE INDEX IF NOT EXISTS idx_ai_skill_likes_user_id
    ON ai_skill_likes (user_id);

CREATE INDEX IF NOT EXISTS idx_ai_skill_settlements_skill_id
    ON ai_skill_settlements (skill_id);

CREATE INDEX IF NOT EXISTS idx_ai_skill_settlements_owner_created_at
    ON ai_skill_settlements (owner_user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ai_skill_settlements_status
    ON ai_skill_settlements (status);

COMMENT ON TABLE ai_skills IS '技能中心主表';
COMMENT ON TABLE ai_skill_versions IS '技能版本表，所有测试/使用/上架均基于审核通过版本';
COMMENT ON TABLE ai_skill_reviews IS '技能审核记录表';
COMMENT ON TABLE ai_skill_runs IS '技能运行记录表';
COMMENT ON TABLE ai_skill_likes IS '技能点赞表';
COMMENT ON TABLE ai_skill_settlements IS '技能收益结算表，收益先进入返利额度后再转余额';

COMMENT ON COLUMN ai_skills.skill_type IS '技能类型：prompt_chat / prompt_image / script';
COMMENT ON COLUMN ai_skills.source_visibility IS '源内容可见性：public / hidden；收费公开技能默认 hidden';
COMMENT ON COLUMN ai_skills.billing_mode IS '收费模式，第一期仅支持 per_request';
COMMENT ON COLUMN ai_skills.published_version_id IS '当前上架版本，只允许引用审核通过版本';
COMMENT ON COLUMN ai_skills.latest_approved_version_id IS '最近一次审核通过的版本';
COMMENT ON COLUMN ai_skill_versions.review_status IS '版本审核状态：draft / pending / approved / rejected';
COMMENT ON COLUMN ai_skill_reviews.status IS '审核单状态：pending / approved / rejected / canceled';
COMMENT ON COLUMN ai_skill_runs.run_mode IS '运行模式：test / use';
COMMENT ON COLUMN ai_skill_settlements.status IS '结算状态：pending / quota_accrued / transferred / canceled';
