-- Batch image prices are persisted at scale 10. Preserve the wallet's existing
-- 12 integer digits while widening balance operations to the same scale.
ALTER TABLE users
    ALTER COLUMN balance TYPE DECIMAL(22,10) USING balance::DECIMAL(22,10),
    ALTER COLUMN frozen_balance TYPE DECIMAL(22,10) USING frozen_balance::DECIMAL(22,10);
