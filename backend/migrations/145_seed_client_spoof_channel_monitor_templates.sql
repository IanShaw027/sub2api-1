-- Migration: 145_seed_client_spoof_channel_monitor_templates
-- 为渠道监控补齐三套常用“客户端伪装”模板：
--   1) Anthropic Claude Code 伪装
--   2) OpenAI Codex 客户端伪装（best-effort，监控固定走 /v1/chat/completions）
--   3) Gemini CLI 客户端伪装
--
-- 要点：
--   - 新库初始化：在 128/129 之后继续落到最终模板集合
--   - 老库升级：把历史上的示例模板收敛为当前实际使用模板
--   - 全部使用 ON CONFLICT ... DO UPDATE，保持幂等

DELETE FROM channel_monitor_request_templates
WHERE provider = 'anthropic'
  AND name = 'Anthropic 请求模板示例';

INSERT INTO channel_monitor_request_templates (
    name,
    provider,
    description,
    extra_headers,
    body_override_mode,
    body_override,
    created_at,
    updated_at
)
VALUES (
    'Claude Code 伪装',
    'anthropic',
    '完整模拟 Claude Code 客户端：UA + anthropic-beta + system + metadata.user_id，对需要 Claude Code 客户端形态的 Anthropic 上游更友好。',
    '{
      "X-App": "cli",
      "User-Agent": "claude-cli/2.1.114 (external, sdk-cli)",
      "anthropic-beta": "claude-code-20250219,interleaved-thinking-2025-05-14,context-management-2025-06-27,prompt-caching-scope-2026-01-05,advisor-tool-2026-03-01",
      "anthropic-version": "2023-06-01",
      "anthropic-dangerous-direct-browser-access": "true"
    }'::jsonb,
    'merge',
    '{
      "system": [
        {
          "text": "You are Claude Code, Anthropic''s official CLI for Claude.",
          "type": "text"
        }
      ],
      "metadata": {
        "user_id": "user_0000000000000000000000000000000000000000000000000000000000000000_account_00000000-0000-0000-0000-000000000000_session_00000000-0000-0000-0000-000000000000"
      }
    }'::jsonb,
    NOW(),
    NOW()
)
ON CONFLICT (provider, name) DO UPDATE SET
    description = EXCLUDED.description,
    extra_headers = EXCLUDED.extra_headers,
    body_override_mode = EXCLUDED.body_override_mode,
    body_override = EXCLUDED.body_override,
    updated_at = NOW();

INSERT INTO channel_monitor_request_templates (
    name,
    provider,
    description,
    extra_headers,
    body_override_mode,
    body_override,
    created_at,
    updated_at
)
VALUES (
    'OpenAI Codex 客户端伪装',
    'openai',
    'Best-effort 伪装 Codex/OpenAI JS 客户端指纹。注意：监控固定走 /v1/chat/completions，无法完整复刻 /responses 专属链路。',
    '{
      "Originator": "codex_cli_rs",
      "User-Agent": "codex_cli_rs/0.125.0",
      "OpenAI-Beta": "assistants=v2",
      "X-Stainless-Lang": "js",
      "X-Stainless-Package-Version": "6.26.0",
      "X-Stainless-OS": "MacOS",
      "X-Stainless-Arch": "arm64",
      "X-Stainless-Runtime": "node",
      "X-Stainless-Runtime-Version": "v24.9.0",
      "X-Stainless-Retry-Count": "0",
      "X-Stainless-Timeout": "60000"
    }'::jsonb,
    'merge',
    '{
      "user": "codex_cli_rs"
    }'::jsonb,
    NOW(),
    NOW()
)
ON CONFLICT (provider, name) DO UPDATE SET
    description = EXCLUDED.description,
    extra_headers = EXCLUDED.extra_headers,
    body_override_mode = EXCLUDED.body_override_mode,
    body_override = EXCLUDED.body_override,
    updated_at = NOW();

INSERT INTO channel_monitor_request_templates (
    name,
    provider,
    description,
    extra_headers,
    body_override_mode,
    body_override,
    created_at,
    updated_at
)
VALUES (
    'Gemini CLI 客户端伪装',
    'gemini',
    '伪装 Gemini CLI 请求风格：复用项目内 GeminiCLI User-Agent，并补 systemInstruction，适合 generateContent 监控。',
    '{
      "User-Agent": "GeminiCLI/0.1.5 (Windows; AMD64)"
    }'::jsonb,
    'merge',
    '{
      "systemInstruction": {
        "parts": [
          {
            "text": "You are a helpful AI assistant."
          }
        ]
      }
    }'::jsonb,
    NOW(),
    NOW()
)
ON CONFLICT (provider, name) DO UPDATE SET
    description = EXCLUDED.description,
    extra_headers = EXCLUDED.extra_headers,
    body_override_mode = EXCLUDED.body_override_mode,
    body_override = EXCLUDED.body_override,
    updated_at = NOW();
