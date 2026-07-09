-- 1) Normalize historical affiliate rebate rate values.
-- Legacy compatibility treated 0<x<=1 as fractional inputs (e.g. 0.2 => 20%).
-- We now use pure percentage semantics, so convert persisted fractional values once.
WITH parsed_settings AS (
    SELECT id,
           CASE
               WHEN value ~ '^-?[0-9]+([.][0-9]+)?$' THEN value::numeric
               ELSE NULL
           END AS numeric_value
    FROM settings
    WHERE key = 'affiliate_rebate_rate'
)
UPDATE settings s
SET value = to_char((p.numeric_value * 100), 'FM999999990.########'),
    updated_at = NOW()
FROM parsed_settings p
WHERE s.id = p.id
  AND p.numeric_value > 0
  AND p.numeric_value <= 1;

-- 2) Affiliate ledger for accrual/transfer traceability.
CREATE TABLE IF NOT EXISTS user_affiliate_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action VARCHAR(32) NOT NULL,
    amount DECIMAL(20,8) NOT NULL,
    source_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_affiliate_ledger_user_id ON user_affiliate_ledger(user_id);
CREATE INDEX IF NOT EXISTS idx_user_affiliate_ledger_action ON user_affiliate_ledger(action);

COMMENT ON TABLE user_affiliate_ledger IS '邀请返利资金流水（累计/转入）';
COMMENT ON COLUMN user_affiliate_ledger.action IS 'accrue|transfer';

CREATE TABLE IF NOT EXISTS payment_audit_logs_migration_archive (
    original_id BIGINT PRIMARY KEY,
    archived_from_migration VARCHAR(128) NOT NULL,
    archived_reason VARCHAR(128) NOT NULL,
    row_data JSONB NOT NULL,
    archived_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3) Enforce idempotency only for affiliate rebate audit actions.
-- The hot partial unique index rollout moved to
-- 138_subscription_fulfillment_claim_unique_notx.sql so Postgres upgrades do
-- not build it inside this transaction.
WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY order_id, action ORDER BY id) AS rn
    FROM payment_audit_logs
    WHERE action IN ('AFFILIATE_REBATE_APPLIED', 'AFFILIATE_REBATE_SKIPPED')
)
INSERT INTO payment_audit_logs_migration_archive (
    original_id,
    archived_from_migration,
    archived_reason,
    row_data
)
SELECT p.id,
       '131_affiliate_rebate_hardening',
       'duplicate affiliate rebate audit sentinel',
       to_jsonb(p)
FROM payment_audit_logs p
JOIN ranked r ON r.id = p.id
WHERE r.rn > 1
ON CONFLICT (original_id) DO NOTHING;

WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY order_id, action ORDER BY id) AS rn
    FROM payment_audit_logs
    WHERE action IN ('AFFILIATE_REBATE_APPLIED', 'AFFILIATE_REBATE_SKIPPED')
)
DELETE FROM payment_audit_logs p
USING ranked r
WHERE p.id = r.id
  AND r.rn > 1;

-- 4) Prevent retroactive affiliate rebate issuance for legacy completed balance orders.
INSERT INTO payment_audit_logs (order_id, action, detail, operator, created_at)
SELECT po.id::text,
       'AFFILIATE_REBATE_SKIPPED',
       '{"reason":"baseline before affiliate rebate idempotency rollout"}',
       'system',
       NOW()
FROM payment_orders po
WHERE po.order_type = 'balance'
  AND po.status = 'COMPLETED'
  AND NOT EXISTS (
      SELECT 1
      FROM payment_audit_logs pal
      WHERE pal.order_id = po.id::text
        AND pal.action IN ('AFFILIATE_REBATE_APPLIED', 'AFFILIATE_REBATE_SKIPPED')
  );
