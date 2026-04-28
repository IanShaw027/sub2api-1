-- Clean up historical duplicate payment audit sentinels before the
-- non-transactional unique index migration runs.
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
