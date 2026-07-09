ALTER TABLE groups
    VALIDATE CONSTRAINT groups_rpm_limit_non_negative;

ALTER TABLE users
    VALIDATE CONSTRAINT users_rpm_limit_non_negative;

ALTER TABLE user_group_rate_multipliers
    VALIDATE CONSTRAINT user_group_rate_multipliers_rpm_override_non_negative;

ALTER TABLE groups
    VALIDATE CONSTRAINT groups_search_price_per_1k_non_negative;

ALTER TABLE groups
    VALIDATE CONSTRAINT groups_audio_realtime_price_per_min_non_negative;

ALTER TABLE groups
    VALIDATE CONSTRAINT groups_audio_tts_price_per_million_chars_non_negative;

ALTER TABLE groups
    VALIDATE CONSTRAINT groups_audio_stt_price_per_hour_non_negative;
