-- Enforce one active internal creation key per user and group across instances.
WITH duplicate_creation_keys AS (
    SELECT id
    FROM (
        SELECT id,
               ROW_NUMBER() OVER (PARTITION BY user_id, group_id ORDER BY id) AS row_number
        FROM api_keys
        WHERE deleted_at IS NULL AND purpose = 'creation'
    ) ranked
    WHERE ranked.row_number > 1
)
UPDATE api_keys
SET deleted_at = NOW(), updated_at = NOW()
WHERE id IN (SELECT id FROM duplicate_creation_keys);

CREATE UNIQUE INDEX IF NOT EXISTS api_keys_creation_user_group_unique_idx
    ON api_keys (user_id, group_id)
    WHERE deleted_at IS NULL AND purpose = 'creation';
