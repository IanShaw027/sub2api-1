-- 188_add_group_audio_search_price_checks.sql
-- Add non-negative checks for audio/search pricing without mutating migration 184.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'groups_search_price_per_1k_non_negative'
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_search_price_per_1k_non_negative
            CHECK (search_price_per_1k IS NULL OR search_price_per_1k >= 0);
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'groups_audio_realtime_price_per_min_non_negative'
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_audio_realtime_price_per_min_non_negative
            CHECK (audio_realtime_price_per_min IS NULL OR audio_realtime_price_per_min >= 0);
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'groups_audio_tts_price_per_million_chars_non_negative'
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_audio_tts_price_per_million_chars_non_negative
            CHECK (audio_tts_price_per_million_chars IS NULL OR audio_tts_price_per_million_chars >= 0);
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'groups_audio_stt_price_per_hour_non_negative'
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_audio_stt_price_per_hour_non_negative
            CHECK (audio_stt_price_per_hour IS NULL OR audio_stt_price_per_hour >= 0);
    END IF;
END $$;
