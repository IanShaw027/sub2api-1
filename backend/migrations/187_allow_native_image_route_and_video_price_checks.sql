-- 187_allow_native_image_route_and_video_price_checks.sql
-- Align DB constraints with Grok/native image routing and Ent video-price validation.

ALTER TABLE groups
    DROP CONSTRAINT IF EXISTS groups_image_generation_route_check;

ALTER TABLE groups
    ADD CONSTRAINT groups_image_generation_route_check
    CHECK (image_generation_route IN ('codex', 'web2api', 'native')) NOT VALID;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'groups_video_price_480p_per_sec_non_negative'
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_video_price_480p_per_sec_non_negative
            CHECK (video_price_480p_per_sec IS NULL OR video_price_480p_per_sec >= 0) NOT VALID;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'groups_video_price_720p_per_sec_non_negative'
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_video_price_720p_per_sec_non_negative
            CHECK (video_price_720p_per_sec IS NULL OR video_price_720p_per_sec >= 0) NOT VALID;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'groups_video_price_1080p_per_sec_non_negative'
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_video_price_1080p_per_sec_non_negative
            CHECK (video_price_1080p_per_sec IS NULL OR video_price_1080p_per_sec >= 0) NOT VALID;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'groups_video_price_4k_per_sec_non_negative'
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_video_price_4k_per_sec_non_negative
            CHECK (video_price_4k_per_sec IS NULL OR video_price_4k_per_sec >= 0) NOT VALID;
    END IF;
END $$;
