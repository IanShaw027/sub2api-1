-- Clean up historical duplicate payment audit sentinels before the
-- non-transactional unique index migration runs.
CREATE TABLE IF NOT EXISTS payment_audit_logs_migration_archive (
    original_id BIGINT PRIMARY KEY,
    archived_from_migration VARCHAR(128) NOT NULL,
    archived_reason VARCHAR(128) NOT NULL,
    row_data JSONB NOT NULL,
    archived_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY order_id, action ORDER BY id) AS rn
    FROM payment_audit_logs
    WHERE action IN (
        'AFFILIATE_REBATE_APPLIED',
        'AFFILIATE_REBATE_SKIPPED',
        'SUBSCRIPTION_FULFILLMENT_CLAIMED',
        'SUBSCRIPTION_SUCCESS'
    )
)
INSERT INTO payment_audit_logs_migration_archive (
    original_id,
    archived_from_migration,
    archived_reason,
    row_data
)
SELECT p.id,
       '137_subscription_fulfillment_claim_dedupe',
       'duplicate payment audit sentinel',
       to_jsonb(p)
FROM payment_audit_logs p
JOIN ranked r ON r.id = p.id
WHERE r.rn > 1
ON CONFLICT (original_id) DO NOTHING;

WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY order_id, action ORDER BY id) AS rn
    FROM payment_audit_logs
    WHERE action IN (
        'AFFILIATE_REBATE_APPLIED',
        'AFFILIATE_REBATE_SKIPPED',
        'SUBSCRIPTION_FULFILLMENT_CLAIMED',
        'SUBSCRIPTION_SUCCESS'
    )
)
DELETE FROM payment_audit_logs p
USING ranked r
WHERE p.id = r.id
  AND r.rn > 1;
