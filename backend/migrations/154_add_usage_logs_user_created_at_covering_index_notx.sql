CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_user_created_at_covering
    ON usage_logs (user_id, created_at DESC)
    INCLUDE (actual_cost, subscription_id);
