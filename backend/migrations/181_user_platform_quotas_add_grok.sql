-- 扩展 user_platform_quotas.platform CHECK 约束，纳入 grok 平台。
-- 156_user_platform_quotas_add_kiro.sql 已把约束扩展到 kiro；
-- 本迁移继续保持与 service.AllowedQuotaPlatforms / ent 校验一致。

ALTER TABLE user_platform_quotas
    DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check;

ALTER TABLE user_platform_quotas
    ADD CONSTRAINT user_platform_quotas_platform_check
    CHECK (platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'kiro', 'grok'));
