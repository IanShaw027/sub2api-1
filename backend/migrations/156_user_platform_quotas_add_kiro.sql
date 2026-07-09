-- 扩展 user_platform_quotas.platform CHECK 约束，纳入 kiro 平台。
-- 142_user_platform_quotas.sql 创建表时把 platform 限制在
-- ('anthropic','openai','gemini','antigravity')，导致 finalizePostUsageBilling
-- 在 platform=kiro 时大量触发 user_platform_quotas_platform_check 违例 ALERT。
-- 这里把约束替换为包含 kiro 的新版本，幂等：旧名约束存在则先 drop。

ALTER TABLE user_platform_quotas
    DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check;

ALTER TABLE user_platform_quotas
    ADD CONSTRAINT user_platform_quotas_platform_check
    CHECK (platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'kiro')) NOT VALID;
