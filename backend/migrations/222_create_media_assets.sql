-- Media assets: private-bucket object index for invoices, tickets, avatars.
-- Visibility is application-level; the S3 bucket itself stays private.

CREATE TABLE IF NOT EXISTS media_assets (
    id                  BIGSERIAL PRIMARY KEY,
    owner_user_id       BIGINT NOT NULL,
    biz_type            VARCHAR(32) NOT NULL,
    biz_id              VARCHAR(128) NOT NULL DEFAULT '',
    storage_key         VARCHAR(512) NOT NULL,
    sha256              VARCHAR(64) NOT NULL,
    mime                VARCHAR(128) NOT NULL,
    filename            VARCHAR(255) NOT NULL DEFAULT '',
    size                BIGINT NOT NULL,
    visibility          VARCHAR(16) NOT NULL,
    status              VARCHAR(16) NOT NULL DEFAULT 'ready',
    storage_profile_id  VARCHAR(64) NOT NULL DEFAULT 'backup',
    public_base_url     TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT media_assets_visibility_check CHECK (visibility IN ('public', 'private')),
    CONSTRAINT media_assets_biz_type_check CHECK (biz_type IN ('invoice', 'ticket', 'avatar', 'image_task')),
    CONSTRAINT media_assets_status_check CHECK (status IN ('ready', 'deleted')),
    CONSTRAINT media_assets_size_check CHECK (size >= 0)
);

CREATE INDEX IF NOT EXISTS idx_media_assets_biz ON media_assets (biz_type, biz_id);
CREATE INDEX IF NOT EXISTS idx_media_assets_owner ON media_assets (owner_user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_media_assets_storage_key ON media_assets (storage_key);

COMMENT ON TABLE media_assets IS 'Application-level media objects stored in a private S3-compatible bucket';
COMMENT ON COLUMN media_assets.visibility IS 'public: served via GET /api/v1/media/public/:id; private: HMAC download';
COMMENT ON COLUMN media_assets.public_base_url IS 'Optional CDN base recorded for public objects; clients still use the gateway';
