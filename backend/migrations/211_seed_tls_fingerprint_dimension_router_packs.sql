-- Seed ops-friendly TLS fingerprint router packs that emit OS/client_type dimensions
-- (not hard-coded profile IDs). Operators bind concrete profiles per account via
-- tls_fingerprint_bindings after importing capture samples.
-- Idempotent: only inserts when the pack name does not already exist.

INSERT INTO tls_fingerprint_routers (name, description, enabled, rules)
SELECT
    'openai-dimension-pack',
    'OpenAI/Codex dimension router: match inbound UA to os/client_type for account binding matrix resolution.',
    TRUE,
    '[
      {"name":"Codex CLI","enabled":true,"transport":"","match_type":"contains","pattern":"codex_cli_rs","case_sensitive":false,"tls_fingerprint_profile_id":0,"os":"","client_type":"codex-cli"},
      {"name":"Codex TUI","enabled":true,"transport":"","match_type":"contains","pattern":"codex-tui","case_sensitive":false,"tls_fingerprint_profile_id":0,"os":"","client_type":"codex-cli"},
      {"name":"Codex Desktop","enabled":true,"transport":"","match_type":"contains","pattern":"Codex Desktop","case_sensitive":false,"tls_fingerprint_profile_id":0,"os":"","client_type":"chatgpt-desktop"},
      {"name":"ChatGPT Desktop","enabled":true,"transport":"","match_type":"contains","pattern":"ChatGPT","case_sensitive":false,"tls_fingerprint_profile_id":0,"os":"","client_type":"chatgpt-desktop"},
      {"name":"Node OpenAI SDK","enabled":true,"transport":"","match_type":"contains","pattern":"OpenAI/","case_sensitive":false,"tls_fingerprint_profile_id":0,"os":"","client_type":"openai-sdk"}
    ]'::jsonb
WHERE NOT EXISTS (
    SELECT 1 FROM tls_fingerprint_routers WHERE name = 'openai-dimension-pack'
);

INSERT INTO tls_fingerprint_routers (name, description, enabled, rules)
SELECT
    'claude-dimension-pack',
    'Claude Code dimension router: match inbound UA to os/client_type for account bindings.',
    TRUE,
    '[
      {"name":"Claude Code","enabled":true,"transport":"","match_type":"contains","pattern":"claude-cli","case_sensitive":false,"tls_fingerprint_profile_id":0,"os":"","client_type":"claude-code"},
      {"name":"Claude Code alt","enabled":true,"transport":"","match_type":"contains","pattern":"Claude-Code","case_sensitive":false,"tls_fingerprint_profile_id":0,"os":"","client_type":"claude-code"},
      {"name":"Claude Desktop","enabled":true,"transport":"","match_type":"contains","pattern":"Claude","case_sensitive":false,"tls_fingerprint_profile_id":0,"os":"","client_type":"claude-desktop"}
    ]'::jsonb
WHERE NOT EXISTS (
    SELECT 1 FROM tls_fingerprint_routers WHERE name = 'claude-dimension-pack'
);

INSERT INTO tls_fingerprint_routers (name, description, enabled, rules)
SELECT
    'grok-dimension-pack',
    'Grok/xAI dimension router: match inbound UA to os/client_type for account bindings.',
    TRUE,
    '[
      {"name":"Grok Desktop","enabled":true,"transport":"","match_type":"contains","pattern":"GrokDesktop","case_sensitive":false,"tls_fingerprint_profile_id":0,"os":"","client_type":"grok-desktop"},
      {"name":"Grok Browser","enabled":true,"transport":"","match_type":"contains","pattern":"Grok","case_sensitive":false,"tls_fingerprint_profile_id":0,"os":"","client_type":"grok-web"}
    ]'::jsonb
WHERE NOT EXISTS (
    SELECT 1 FROM tls_fingerprint_routers WHERE name = 'grok-dimension-pack'
);

INSERT INTO tls_fingerprint_routers (name, description, enabled, rules)
SELECT
    'kiro-dimension-pack',
    'Kiro dimension router: match inbound UA to os/client_type for account bindings (static default_os still works without inbound UA).',
    TRUE,
    '[
      {"name":"Kiro IDE","enabled":true,"transport":"","match_type":"contains","pattern":"KiroIDE","case_sensitive":false,"tls_fingerprint_profile_id":0,"os":"","client_type":"kiro-ide"},
      {"name":"Kiro CLI","enabled":true,"transport":"","match_type":"contains","pattern":"Kiro","case_sensitive":false,"tls_fingerprint_profile_id":0,"os":"","client_type":"kiro-cli"}
    ]'::jsonb
WHERE NOT EXISTS (
    SELECT 1 FROM tls_fingerprint_routers WHERE name = 'kiro-dimension-pack'
);
