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

WITH rebate_audits AS (
    SELECT po.id AS order_id,
           po.user_id AS invitee_user_id,
           invitee_aff.inviter_id,
           po.amount AS base_amount,
           rebate_detail.rebate_amount,
           pal.created_at AS audit_created_at,
           CASE
               WHEN po.amount > 0 THEN ROUND((rebate_detail.rebate_amount / po.amount) * 100, 4)
               ELSE NULL
           END AS derived_rebate_rate
    FROM payment_audit_logs pal
    CROSS JOIN LATERAL (
        SELECT substring(
            pal.detail
            FROM '"rebateAmount"[[:space:]]*:[[:space:]]*(-?[0-9]+(\.[0-9]+)?)'
        )::numeric AS rebate_amount
    ) rebate_detail
    JOIN payment_orders po ON po.id::text = pal.order_id
    JOIN user_affiliates invitee_aff ON invitee_aff.user_id = po.user_id
    WHERE pal.action = 'AFFILIATE_REBATE_APPLIED'
      AND rebate_detail.rebate_amount IS NOT NULL
),
ranked_matches AS (
    SELECT ual.id AS ledger_id,
           ra.order_id,
           ra.base_amount,
           ra.derived_rebate_rate,
           COUNT(*) OVER (PARTITION BY ra.order_id) AS order_match_count,
           COUNT(*) OVER (PARTITION BY ual.id) AS ledger_match_count,
           ROW_NUMBER() OVER (
               PARTITION BY ual.id
               ORDER BY ABS(EXTRACT(EPOCH FROM (ual.created_at - ra.audit_created_at))), ra.order_id
           ) AS ledger_rank
    FROM rebate_audits ra
    JOIN user_affiliate_ledger ual
      ON ual.action = 'accrue'
     AND ual.source_order_id IS NULL
     AND ual.user_id = ra.inviter_id
     AND ual.source_user_id = ra.invitee_user_id
     AND ABS(ual.amount - ra.rebate_amount) < 0.00000001
     AND ual.created_at BETWEEN ra.audit_created_at - INTERVAL '10 minutes'
                            AND ra.audit_created_at + INTERVAL '10 minutes'
)
UPDATE user_affiliate_ledger ual
SET source_order_id = ranked_matches.order_id,
    base_amount = CASE
        WHEN COALESCE(ual.base_amount, 0) = 0 AND ranked_matches.base_amount IS NOT NULL
            THEN ranked_matches.base_amount
        ELSE ual.base_amount
    END,
    rebate_rate = CASE
        WHEN COALESCE(ual.rebate_rate, 0) = 0 AND ranked_matches.derived_rebate_rate IS NOT NULL
            THEN ranked_matches.derived_rebate_rate
        ELSE ual.rebate_rate
    END,
    invitee_slot_claimed = CASE
        WHEN ual.source_user_id IS NOT NULL THEN TRUE
        ELSE ual.invitee_slot_claimed
    END,
    updated_at = NOW()
FROM ranked_matches
WHERE ual.id = ranked_matches.ledger_id
  AND ranked_matches.order_match_count = 1
  AND ranked_matches.ledger_match_count = 1
  AND ranked_matches.ledger_rank = 1
  AND NOT EXISTS (
      SELECT 1
      FROM user_affiliate_ledger existing
      WHERE existing.source_order_id = ranked_matches.order_id
        AND existing.action = 'accrue'
        AND existing.id <> ual.id
  );

-- Legacy accrue rows predate invitee_slot_claimed. Mark them as claimed so
-- historical rebated invitees keep counting toward the new stats/limit logic.
UPDATE user_affiliate_ledger
SET invitee_slot_claimed = TRUE
WHERE action = 'accrue'
  AND source_user_id IS NOT NULL
  AND invitee_slot_claimed = FALSE;

CREATE INDEX IF NOT EXISTS idx_user_affiliate_ledger_source_user
ON user_affiliate_ledger(user_id, source_user_id);

CREATE INDEX IF NOT EXISTS idx_user_affiliate_ledger_order
ON user_affiliate_ledger(source_order_id);

CREATE TABLE IF NOT EXISTS user_affiliate_ledger_migration_archive (
    original_id BIGINT PRIMARY KEY,
    archived_from_migration VARCHAR(128) NOT NULL,
    archived_reason VARCHAR(128) NOT NULL,
    row_data JSONB NOT NULL,
    archived_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY user_id, source_order_id, action ORDER BY id) AS rn
    FROM user_affiliate_ledger
    WHERE action = 'accrue'
      AND source_order_id IS NOT NULL
)
INSERT INTO user_affiliate_ledger_migration_archive (
    original_id,
    archived_from_migration,
    archived_reason,
    row_data
)
SELECT l.id,
       '132_affiliate_policy_limits',
       'duplicate affiliate ledger accrue row',
       to_jsonb(l)
FROM user_affiliate_ledger l
JOIN ranked r ON r.id = l.id
WHERE r.rn > 1
ON CONFLICT (original_id) DO NOTHING;

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
