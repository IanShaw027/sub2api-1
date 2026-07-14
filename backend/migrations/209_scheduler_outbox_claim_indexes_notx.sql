CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_scheduler_outbox_claimable
    ON scheduler_outbox (created_at, id)
    WHERE claimed_at IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_scheduler_outbox_claim_expiry
    ON scheduler_outbox (claimed_at)
    WHERE claimed_at IS NOT NULL;
