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

DROP INDEX IF EXISTS batch_image_jobs_idempotency_key_idx;

CREATE UNIQUE INDEX batch_image_jobs_idempotency_owner_uq
    ON batch_image_jobs (user_id, api_key_id, idempotency_key) NULLS NOT DISTINCT
    WHERE idempotency_key IS NOT NULL AND idempotency_key <> '';
