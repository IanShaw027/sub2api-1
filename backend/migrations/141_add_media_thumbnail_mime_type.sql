ALTER TABLE media_assets
    ADD COLUMN IF NOT EXISTS thumbnail_mime_type VARCHAR(255) NOT NULL DEFAULT '';

COMMENT ON COLUMN media_assets.thumbnail_mime_type IS '缩略图 MIME 类型，供缩略图专用下载路由返回正确的 Content-Type';
