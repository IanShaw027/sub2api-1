UPDATE usage_logs
SET video_seconds = video_duration_seconds
WHERE video_seconds IS NULL
  AND video_duration_seconds IS NOT NULL;

COMMENT ON COLUMN usage_logs.video_duration_seconds IS
    'Deprecated compatibility column: backfilled into video_seconds by migration 213; no new writes after this migration and safe to drop after all pre-213 nodes are retired';

COMMENT ON COLUMN usage_logs.video_seconds IS
    'Canonical generated video duration in seconds';
