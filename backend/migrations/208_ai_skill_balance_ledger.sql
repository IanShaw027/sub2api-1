CREATE TABLE IF NOT EXISTS ai_skill_balance_ledger (
    id BIGSERIAL PRIMARY KEY,
    reference VARCHAR(191) NOT NULL,
    operation VARCHAR(16) NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount DECIMAL(20,8) NOT NULL,
    balance_before DECIMAL(20,8),
    balance_after DECIMAL(20,8),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_id VARCHAR(64),
    usage_log_id BIGINT,
    api_key_id BIGINT,
    group_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_ai_skill_balance_ledger_reference_operation
        UNIQUE (reference, operation),
    CONSTRAINT chk_ai_skill_balance_ledger_operation
        CHECK (operation IN ('charge', 'refund')),
    CONSTRAINT chk_ai_skill_balance_ledger_positive_amount
        CHECK (amount > 0),
    CONSTRAINT chk_ai_skill_balance_ledger_balances_complete
        CHECK ((balance_before IS NULL) = (balance_after IS NULL))
);

CREATE INDEX IF NOT EXISTS idx_ai_skill_balance_ledger_user_created_at
    ON ai_skill_balance_ledger (user_id, created_at DESC);

COMMENT ON TABLE ai_skill_balance_ledger IS
    'Idempotency ledger for atomic AI skill buyer balance charges and refunds';
