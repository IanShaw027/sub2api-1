-- Protect user API keys at rest.
--
-- Application startup performs the data transformation:
--   key (legacy plaintext) -> lookup_hash + key_ciphertext + key_prefix
-- The legacy key column is then overwritten with lookup_hash so it no longer
-- contains directly usable credentials. key_ciphertext is optional: deployments
-- without a stable AES-GCM key use hash-only mode and disable Reveal.

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS lookup_hash VARCHAR(64),
    ADD COLUMN IF NOT EXISTS key_ciphertext TEXT,
    ADD COLUMN IF NOT EXISTS key_prefix VARCHAR(32) NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS api_keys_lookup_hash_unique
    ON api_keys (lookup_hash)
    WHERE lookup_hash IS NOT NULL;

CREATE INDEX IF NOT EXISTS api_keys_key_prefix_index
    ON api_keys (key_prefix);

-- The historical trigram index would now index hashes rather than useful
-- search text. Prefix search uses the compact B-tree index above.
DROP INDEX IF EXISTS idx_api_keys_key_trgm;

COMMENT ON COLUMN api_keys.lookup_hash IS
    'SHA-256 fingerprint used for API key authentication lookup';
COMMENT ON COLUMN api_keys.key_ciphertext IS
    'AES-256-GCM encrypted API key for owner-authorized reveal';
COMMENT ON COLUMN api_keys.key_prefix IS
    'Non-sensitive API key prefix used for display and search';

-- Historical deleted_api_key_audits.key rows may still contain plaintext.
-- Startup migration hashes those values in Go so pgcrypto is not required.
