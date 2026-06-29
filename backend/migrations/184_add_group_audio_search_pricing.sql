-- 184_add_group_audio_search_pricing.sql
-- Add explicit group-level pricing for search/tools and audio/voice (Grok + general non-text modalities).
-- These are set directly on the group (like image/video prices) and intended to be used without (or with platform-specific) text RateMultiplier.
-- Reference: OpenAI image pricing (explicit per size on group, resolved multiplier=1 for OpenAI platform).

ALTER TABLE groups ADD COLUMN IF NOT EXISTS search_price_per_1k DECIMAL(20,8);

ALTER TABLE groups ADD COLUMN IF NOT EXISTS audio_realtime_price_per_min DECIMAL(20,8);
ALTER TABLE groups ADD COLUMN IF NOT EXISTS audio_tts_price_per_million_chars DECIMAL(20,8);
ALTER TABLE groups ADD COLUMN IF NOT EXISTS audio_stt_price_per_hour DECIMAL(20,8);

COMMENT ON COLUMN groups.search_price_per_1k IS '搜索/工具调用显式价格 per 1000 calls (USD)，如 grok web_search / x_search 等，不走文本倍率';
COMMENT ON COLUMN groups.audio_realtime_price_per_min IS '语音 Realtime 每分钟显式价格 (USD)';
COMMENT ON COLUMN groups.audio_tts_price_per_million_chars IS 'TTS 每百万字符显式价格 (USD)';
COMMENT ON COLUMN groups.audio_stt_price_per_hour IS 'STT 每小时显式价格 (USD)';

-- Note: image/video prices already exist and can be used/shared for Grok image/video when enabled.
