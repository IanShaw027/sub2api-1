ALTER TABLE user_platform_quotas
    VALIDATE CONSTRAINT user_platform_quotas_platform_check;

ALTER TABLE groups
    VALIDATE CONSTRAINT groups_image_generation_route_check;

ALTER TABLE groups
    VALIDATE CONSTRAINT groups_video_price_480p_per_sec_non_negative;

ALTER TABLE groups
    VALIDATE CONSTRAINT groups_video_price_720p_per_sec_non_negative;

ALTER TABLE groups
    VALIDATE CONSTRAINT groups_video_price_1080p_per_sec_non_negative;

ALTER TABLE groups
    VALIDATE CONSTRAINT groups_video_price_4k_per_sec_non_negative;
