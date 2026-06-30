-- 190_clear_non_grok_video_generation_config.sql
-- Videos are Grok/xAI-only. Remove stale video flags/prices from non-Grok groups.

UPDATE groups
SET allow_video_generation = false,
    video_generation_route = 'native',
    video_price_480p_per_sec = NULL,
    video_price_720p_per_sec = NULL,
    video_price_1080p_per_sec = NULL,
    video_price_4k_per_sec = NULL
WHERE platform IS DISTINCT FROM 'grok'
  AND (
      allow_video_generation IS DISTINCT FROM false
      OR video_generation_route IS DISTINCT FROM 'native'
      OR video_price_480p_per_sec IS NOT NULL
      OR video_price_720p_per_sec IS NOT NULL
      OR video_price_1080p_per_sec IS NOT NULL
      OR video_price_4k_per_sec IS NOT NULL
  );
