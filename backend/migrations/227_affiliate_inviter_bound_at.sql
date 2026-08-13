ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS inviter_bound_at TIMESTAMPTZ NULL;

UPDATE user_affiliates
SET inviter_bound_at = created_at
WHERE inviter_id IS NOT NULL
  AND inviter_bound_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_affiliates_inviter_bound_at
    ON user_affiliates (inviter_bound_at)
    WHERE inviter_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_affiliate_signup_bonus_once
    ON user_affiliate_ledger (user_id)
    WHERE action = 'signup_bonus';

COMMENT ON COLUMN user_affiliates.inviter_bound_at IS '邀请关系绑定时间，返利有效期从此刻起算';
