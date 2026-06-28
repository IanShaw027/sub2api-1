-- 183_add_group_video_pricing.sql
-- Add video generation pricing and controls, consistent with image 1k/2k/4k model.
-- Video charged by resolution tier + per second.

ALTER TABLE groups ADD COLUMN IF NOT EXISTS allow_video_generation BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS video_generation_route VARCHAR(20) NOT NULL DEFAULT 'native';

-- Video pricing per resolution per second (USD)
ALTER TABLE groups ADD COLUMN IF NOT EXISTS video_price_480p_per_sec DECIMAL(20,8);
ALTER TABLE groups ADD COLUMN IF NOT EXISTS video_price_720p_per_sec DECIMAL(20,8);
ALTER TABLE groups ADD COLUMN IF NOT EXISTS video_price_1080p_per_sec DECIMAL(20,8);
ALTER TABLE groups ADD COLUMN IF NOT EXISTS video_price_4k_per_sec DECIMAL(20,8);

COMMENT ON COLUMN groups.allow_video_generation IS '是否允许该分组使用视频生成能力';
COMMENT ON COLUMN groups.video_generation_route IS '视频生成路由 (provider native)';

COMMENT ON COLUMN groups.video_price_480p_per_sec IS '480p 视频生成单价 (USD per second)';
COMMENT ON COLUMN groups.video_price_720p_per_sec IS '720p 视频生成单价 (USD per second)';
COMMENT ON COLUMN groups.video_price_1080p_per_sec IS '1080p 视频生成单价 (USD per second)';
COMMENT ON COLUMN groups.video_price_4k_per_sec IS '4K 视频生成单价 (USD per second)';

-- Also add for legacy images2api style if needed, but start with standard
