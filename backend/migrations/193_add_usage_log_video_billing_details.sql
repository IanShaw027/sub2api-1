ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS video_resolution VARCHAR(20),
    ADD COLUMN IF NOT EXISTS video_seconds INTEGER,
    ADD COLUMN IF NOT EXISTS video_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS video_unit_price NUMERIC(20,10);

CREATE INDEX IF NOT EXISTS idx_usage_logs_video_billing_created_at
    ON usage_logs (created_at)
    WHERE billing_mode = 'video';
