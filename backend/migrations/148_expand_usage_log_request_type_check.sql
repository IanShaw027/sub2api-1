-- Allow newer image request types in usage_logs.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'usage_logs_request_type_check'
          AND conrelid = 'usage_logs'::regclass
    ) THEN
        ALTER TABLE usage_logs
            DROP CONSTRAINT usage_logs_request_type_check;
    END IF;

    ALTER TABLE usage_logs
        ADD CONSTRAINT usage_logs_request_type_check
        CHECK (request_type IN (0, 1, 2, 3, 4, 5)) NOT VALID;
END
$$;
