INSERT INTO settings (key, value, updated_at)
VALUES
    ('affiliate_enabled', 'false', NOW()),
    ('affiliate_rebate_cap', '0', NOW()),
    ('affiliate_rebate_invitee_limit', '0', NOW()),
    ('affiliate_signup_bonus', '0', NOW())
ON CONFLICT (key) DO NOTHING;

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS source_order_id BIGINT NULL REFERENCES payment_orders(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS base_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS rebate_rate DECIMAL(10,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS invitee_slot_claimed BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_user_affiliate_ledger_source_user
ON user_affiliate_ledger(user_id, source_user_id);

CREATE INDEX IF NOT EXISTS idx_user_affiliate_ledger_order
ON user_affiliate_ledger(source_order_id);

WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY user_id, source_order_id, action ORDER BY id) AS rn
    FROM user_affiliate_ledger
    WHERE action = 'accrue'
      AND source_order_id IS NOT NULL
)
DELETE FROM user_affiliate_ledger l
USING ranked r
WHERE l.id = r.id
  AND r.rn > 1;
-- The hot unique index rollout moved to
-- 138_subscription_fulfillment_claim_unique_notx.sql so upgrades use
-- concurrent unique-index builds outside this transaction.

COMMENT ON COLUMN user_affiliate_ledger.source_order_id IS '触发邀请返利的支付订单ID';
COMMENT ON COLUMN user_affiliate_ledger.base_amount IS '触发返利的用户消费金额';
COMMENT ON COLUMN user_affiliate_ledger.rebate_rate IS '返利发生时使用的百分比';
COMMENT ON COLUMN user_affiliate_ledger.invitee_slot_claimed IS '该流水是否占用被邀请消费人数名额';
