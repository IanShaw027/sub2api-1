CREATE TABLE IF NOT EXISTS balance_cache_outbox (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    lease_until TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT
);

CREATE INDEX IF NOT EXISTS idx_balance_cache_outbox_ready
    ON balance_cache_outbox (available_at, id)
    WHERE lease_until IS NULL;

CREATE OR REPLACE FUNCTION enqueue_balance_cache_invalidation()
RETURNS TRIGGER AS $$
BEGIN
    -- Credits can make a rejected user eligible; crossing the exhausted
    -- boundary must also be durable so a stale positive cache cannot admit
    -- later requests. Ordinary deductions stay on the synchronous hot-cache
    -- path and do not create one outbox row per usage record.
    IF NEW.balance > OLD.balance OR (NEW.balance <= 0 AND OLD.balance > 0) THEN
        INSERT INTO balance_cache_outbox (user_id) VALUES (NEW.id);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_users_balance_cache_outbox ON users;
CREATE TRIGGER trg_users_balance_cache_outbox
AFTER UPDATE OF balance ON users
FOR EACH ROW
WHEN (NEW.balance > OLD.balance OR (NEW.balance <= 0 AND OLD.balance > 0))
EXECUTE FUNCTION enqueue_balance_cache_invalidation();
