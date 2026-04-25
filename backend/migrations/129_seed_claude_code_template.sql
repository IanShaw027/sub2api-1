-- Migration: 129_seed_claude_code_template
-- 移除历史上风险较高的“客户端伪装”模板，并提供一个中性的 Anthropic 请求模板示例。

DELETE FROM channel_monitor_request_templates
WHERE provider = 'anthropic'
  AND name = 'Claude Code 伪装';

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
ON CONFLICT (provider, name) DO NOTHING;
