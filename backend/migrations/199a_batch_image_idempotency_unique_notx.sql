-- Create the owner-scoped unique index first so the legacy non-unique
-- idempotency lookup index remains available if CREATE fails. Drop the
-- old index only after uniqueness is in place.
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS batch_image_jobs_idempotency_owner_uq
    ON batch_image_jobs (user_id, api_key_id, idempotency_key) NULLS NOT DISTINCT
    WHERE idempotency_key IS NOT NULL AND idempotency_key <> '';

DROP INDEX CONCURRENTLY IF EXISTS batch_image_jobs_idempotency_key_idx;
