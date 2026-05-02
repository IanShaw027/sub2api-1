CREATE TABLE IF NOT EXISTS media_assets (
    id BIGSERIAL PRIMARY KEY,
    biz_type VARCHAR(64) NOT NULL,
    biz_id VARCHAR(128) NOT NULL DEFAULT '',
    bucket VARCHAR(128) NOT NULL,
    object_key TEXT NOT NULL,
    thumbnail_object_key TEXT NOT NULL DEFAULT '',
    visibility VARCHAR(16) NOT NULL DEFAULT 'private',
    mime_type VARCHAR(255) NOT NULL DEFAULT '',
    size_bytes BIGINT NOT NULL DEFAULT 0,
    width INTEGER,
    height INTEGER,
    sha256 CHAR(64) NOT NULL DEFAULT '',
    owner_user_id BIGINT,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    original_file_name VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS media_assets_bucket_object_key_key
    ON media_assets (bucket, object_key);

CREATE INDEX IF NOT EXISTS idx_media_assets_biz
    ON media_assets (biz_type, biz_id);

CREATE INDEX IF NOT EXISTS idx_media_assets_owner_user_id
    ON media_assets (owner_user_id)
    WHERE owner_user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_media_assets_visibility
    ON media_assets (visibility);

CREATE INDEX IF NOT EXISTS idx_media_assets_status
    ON media_assets (status);

CREATE INDEX IF NOT EXISTS idx_media_assets_created_at
    ON media_assets (created_at DESC);

COMMENT ON TABLE media_assets IS '统一媒体资源表，承载头像、公告图片、工单图片、AI 生成图及后续论坛图片等';
COMMENT ON COLUMN media_assets.biz_type IS '业务类型，例如 avatar / announcement / ticket / ai_image / ai_thumbnail / forum';
COMMENT ON COLUMN media_assets.biz_id IS '业务对象标识，允许为空字符串表示待绑定';
COMMENT ON COLUMN media_assets.visibility IS '可见性：public / private';
COMMENT ON COLUMN media_assets.status IS '状态：active / deleted';
