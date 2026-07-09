-- Non-transactional migration: CREATE INDEX CONCURRENTLY cannot run in a transaction.
--
-- Partial unique index covering creator earnings REVERSAL rows so concurrent
-- reversals of the same skill run cannot double-insert and double claw back
-- aff_quota / aff_history_quota. The reversal path already serializes via
-- FOR UPDATE + post-lock precheck in affiliate_repo.go ReverseCreatorEarnings;
-- this index is defense-in-depth and also makes the ON CONFLICT DO NOTHING guard
-- effective.
--
-- PREREQUISITE: if this fails with a duplicate key error, existing history already
-- contains duplicate 'creator_earning_reverse' rows for the same
-- (user_id, source_skill_run_id) — evidence of prior double clawback. Deduplicate
-- (keep the earliest row per group, delete the rest, reconcile aff_quota manually)
-- first, then re-run. Do NOT drop the uniqueness requirement.
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_user_affiliate_ledger_skill_run_reverse_unique
    ON user_affiliate_ledger(user_id, source_skill_run_id, action)
    WHERE action = 'creator_earning_reverse'
    AND source_skill_run_id IS NOT NULL;
