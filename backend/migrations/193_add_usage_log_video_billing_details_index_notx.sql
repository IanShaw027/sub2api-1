CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_video_billing_created_at
    ON usage_logs (created_at)
    WHERE billing_mode = 'video';
