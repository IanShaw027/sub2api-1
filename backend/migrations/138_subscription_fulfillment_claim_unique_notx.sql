-- Ensure subscription fulfillment and affiliate fulfillment sentinels are
-- claimed exactly once per key, even under concurrent webhook/retry workers.
-- This migration intentionally runs outside a transaction: CREATE/DROP INDEX
-- CONCURRENTLY avoids blocking the hot tables, the migration runner performs a
-- duplicate payment_audit_logs/order_id+action precheck, duplicate
-- user_affiliate_ledger/user_id+source_order_id+action precheck, duplicate
-- user_affiliate_ledger/user_id+action precheck, and drops any stale invalid
-- target index before retry.
DROP INDEX CONCURRENTLY IF EXISTS idx_payment_audit_logs_order_action_uniq;

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_payment_audit_logs_order_action_uniq
ON payment_audit_logs(order_id, action)
WHERE action IN (
    'AFFILIATE_REBATE_APPLIED',
    'AFFILIATE_REBATE_SKIPPED',
    'SUBSCRIPTION_FULFILLMENT_CLAIMED',
    'SUBSCRIPTION_SUCCESS'
);

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_user_affiliate_ledger_order_action_unique
ON user_affiliate_ledger(user_id, source_order_id, action)
WHERE source_order_id IS NOT NULL
  AND action = 'accrue';

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_user_affiliate_signup_bonus_once
ON user_affiliate_ledger(user_id, action)
WHERE action = 'signup_bonus';
