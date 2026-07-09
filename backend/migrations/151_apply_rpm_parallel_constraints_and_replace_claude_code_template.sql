-- Apply post-release RPM semantics/constraints and replace the historical Claude Code spoof seed
-- using a new migration instead of mutating already-published migration files.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'groups_rpm_limit_non_negative'
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_rpm_limit_non_negative
            CHECK (rpm_limit >= 0) NOT VALID;
    END IF;
END $$;

COMMENT ON COLUMN groups.rpm_limit IS '分组 RPM 上限；0 表示不限制；与用户级 rpm_limit 并行生效，可被 user-group rpm_override 覆盖。';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'users_rpm_limit_non_negative'
    ) THEN
        ALTER TABLE users
            ADD CONSTRAINT users_rpm_limit_non_negative
            CHECK (rpm_limit >= 0) NOT VALID;
    END IF;
END $$;

COMMENT ON COLUMN users.rpm_limit IS '用户级 RPM 全局上限；0 表示不限制；与分组/override 限制并行生效。';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'user_group_rate_multipliers_rpm_override_non_negative'
    ) THEN
        ALTER TABLE user_group_rate_multipliers
            ADD CONSTRAINT user_group_rate_multipliers_rpm_override_non_negative
            CHECK (rpm_override IS NULL OR rpm_override >= 0) NOT VALID;
    END IF;
END $$;

INSERT INTO channel_monitor_request_templates (
    name, provider, description, extra_headers, body_override_mode, body_override
)
VALUES (
    'Anthropic 请求模板示例',
    'anthropic',
    '示例模板：仅保留官方 API 必需的 anthropic-version 头，供管理员按需扩展。',
    '{"anthropic-version":"2023-06-01"}'::jsonb,
    'off',
    '{}'::jsonb
)
ON CONFLICT (provider, name) DO UPDATE SET
    description = EXCLUDED.description,
    extra_headers = EXCLUDED.extra_headers,
    body_override_mode = EXCLUDED.body_override_mode,
    body_override = EXCLUDED.body_override,
    updated_at = NOW();

CREATE TABLE IF NOT EXISTS channel_monitor_request_templates_migration_archive (
    original_id BIGINT PRIMARY KEY,
    archived_from_migration VARCHAR(128) NOT NULL,
    archived_reason VARCHAR(128) NOT NULL,
    row_data JSONB NOT NULL,
    archived_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DO $$
DECLARE
    v_example_id BIGINT;
    v_legacy_id BIGINT;
BEGIN
    SELECT id
    INTO v_example_id
    FROM channel_monitor_request_templates
    WHERE provider = 'anthropic'
      AND name = 'Anthropic 请求模板示例'
    ORDER BY id
    LIMIT 1;

    SELECT id
    INTO v_legacy_id
    FROM channel_monitor_request_templates
    WHERE provider = 'anthropic'
      AND name = 'Claude Code 伪装'
    ORDER BY id
    LIMIT 1;

    IF v_example_id IS NOT NULL
       AND v_legacy_id IS NOT NULL
       AND v_example_id <> v_legacy_id THEN
        UPDATE channel_monitors
        SET template_id = v_example_id
        WHERE template_id = v_legacy_id;

        INSERT INTO channel_monitor_request_templates_migration_archive (
            original_id,
            archived_from_migration,
            archived_reason,
            row_data
        )
        SELECT c.id,
               '151_apply_rpm_parallel_constraints_and_replace_claude_code_template',
               'legacy claude code template replaced by anthropic example template',
               to_jsonb(c)
        FROM channel_monitor_request_templates c
        WHERE c.id = v_legacy_id
        ON CONFLICT (original_id) DO NOTHING;

        DELETE FROM channel_monitor_request_templates
        WHERE id = v_legacy_id;
    END IF;
END $$;
