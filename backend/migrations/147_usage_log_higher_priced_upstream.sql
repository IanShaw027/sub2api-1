ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS billed_by_higher_priced_upstream BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN usage_logs.billed_by_higher_priced_upstream IS 'True when billing chose the higher upstream-model price over the requested-model price for this record.';
