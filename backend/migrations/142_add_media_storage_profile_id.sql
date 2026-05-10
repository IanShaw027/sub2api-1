ALTER TABLE media_assets
    ADD COLUMN IF NOT EXISTS storage_profile_id VARCHAR(128) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_media_assets_storage_profile_id
    ON media_assets (storage_profile_id)
    WHERE storage_profile_id <> '';

COMMENT ON COLUMN media_assets.storage_profile_id IS '对象存储配置 ID，用于多存储 profile 下稳定定位媒体文件';
