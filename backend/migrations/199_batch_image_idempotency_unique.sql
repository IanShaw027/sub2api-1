DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM batch_image_jobs
        WHERE idempotency_key IS NOT NULL AND idempotency_key <> ''
        GROUP BY user_id, api_key_id, idempotency_key
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'cannot enforce batch image idempotency uniqueness: duplicate keys exist'
            USING ERRCODE = '23505';
    END IF;
END $$;

-- The unique index is created concurrently in
-- 199a_batch_image_idempotency_unique_notx.sql after this duplicate precheck.
