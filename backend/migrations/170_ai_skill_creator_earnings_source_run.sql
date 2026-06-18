ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS source_skill_run_id BIGINT NULL REFERENCES ai_skill_runs(id) ON DELETE SET NULL;

COMMENT ON COLUMN user_affiliate_ledger.source_skill_run_id IS '产生创作者收益的 AI Skill 运行记录；非技能收益流水为 NULL';

CREATE INDEX IF NOT EXISTS idx_user_affiliate_ledger_source_skill_run_id
    ON user_affiliate_ledger(source_skill_run_id)
    WHERE source_skill_run_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_affiliate_ledger_skill_run_action_unique
    ON user_affiliate_ledger(user_id, source_skill_run_id, action)
    WHERE action = 'creator_earning'
      AND source_skill_run_id IS NOT NULL;
