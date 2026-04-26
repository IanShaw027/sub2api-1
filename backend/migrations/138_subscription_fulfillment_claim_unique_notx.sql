-- Ensure subscription fulfillment is claimed exactly once per order, even under
-- concurrent webhook/retry workers. This migration intentionally runs outside a
-- transaction: CREATE/DROP INDEX CONCURRENTLY avoids blocking the hot audit log
-- table, and the migration runner drops a stale invalid target index before retry.
DROP INDEX CONCURRENTLY IF EXISTS idx_payment_audit_logs_order_action_uniq;

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_payment_audit_logs_order_action_uniq
ON payment_audit_logs(order_id, action)
WHERE action IN (
    'AFFILIATE_REBATE_APPLIED',
    'AFFILIATE_REBATE_SKIPPED',
    'SUBSCRIPTION_FULFILLMENT_CLAIMED',
    'SUBSCRIPTION_SUCCESS'
);
