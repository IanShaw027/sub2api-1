-- Codex 邀请/重置操作历史流水表：记录每次邀请/重置的触发与上游结果。
-- 仅记录操作流水（谁、何时、对哪个账号、内容、成功/失败），不维护配额计数——
-- "还剩几次" 永远实时查 ChatGPT 上游，避免与上游不一致。
CREATE TABLE IF NOT EXISTS codex_invite_reset_history (
    id               BIGSERIAL PRIMARY KEY,
    account_id       BIGINT NOT NULL,
    action_type      VARCHAR(16) NOT NULL,             -- 'invite' | 'consume'
    operator_user_id BIGINT,                            -- 操作管理员；系统触发为 NULL
    emails           JSONB NOT NULL DEFAULT '[]',       -- invite：请求邮箱；consume 为空
    failed_emails    JSONB NOT NULL DEFAULT '[]',       -- invite：上游返回失败邮箱
    credit_id        VARCHAR(128) NOT NULL DEFAULT '',  -- consume：消费的 credit
    success          BOOLEAN NOT NULL,
    result_code      VARCHAR(64) NOT NULL DEFAULT '',   -- consume 上游 code，如 reset
    message          TEXT NOT NULL DEFAULT '',          -- 上游 message / 截断错误
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_codex_invite_reset_history_account
    ON codex_invite_reset_history (account_id, created_at DESC);
