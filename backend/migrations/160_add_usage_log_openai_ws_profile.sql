-- Add OpenAI WS connection profile and reuse flags to usage_logs to compare
-- neutral-pool reuse vs one-shot vs HTTP latency after rollout.
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS openai_ws_profile TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS openai_ws_conn_reused BOOLEAN NOT NULL DEFAULT FALSE;
