-- Detect short-window IP reuse across accounts without adding per-request database writes.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS ips TEXT[] NOT NULL DEFAULT '{}'::text[],
    ADD COLUMN IF NOT EXISTS ip_history_saturated BOOLEAN NOT NULL DEFAULT FALSE;

-- Seed existing API request history once so established users are not treated as new-IP users.
WITH historical_ips AS (
    SELECT user_id, ARRAY_AGG(DISTINCT ip_address) AS ips
    FROM usage_logs
    WHERE ip_address IS NOT NULL AND ip_address <> ''
    GROUP BY user_id
), combined AS (
    SELECT u.id, ARRAY(
        SELECT DISTINCT value
        FROM UNNEST(u.ips || historical_ips.ips) AS value
    ) AS ips
    FROM users AS u
    JOIN historical_ips ON historical_ips.user_id = u.id
)
UPDATE users AS u
SET ips = (combined.ips)[1:256],
    ip_history_saturated = u.ip_history_saturated OR CARDINALITY(combined.ips) > 256
FROM combined
WHERE u.id = combined.id;

CREATE INDEX IF NOT EXISTS idx_usage_logs_ip_created_at_security
    ON usage_logs (ip_address, created_at DESC, user_id, api_key_id);

CREATE TABLE IF NOT EXISTS ip_security_activity (
    id BIGSERIAL PRIMARY KEY,
    ip_address VARCHAR(45) NOT NULL,
    peer_ip VARCHAR(45) NOT NULL DEFAULT '',
    forwarded_for TEXT NOT NULL DEFAULT '',
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source VARCHAR(16) NOT NULL CHECK (source IN ('web', 'apikey')),
    api_key_id BIGINT NOT NULL DEFAULT 0,
    method VARCHAR(16) NOT NULL DEFAULT '',
    path TEXT NOT NULL DEFAULT '',
    request_id VARCHAR(64) NOT NULL DEFAULT '',
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ip_security_activity_user_ip
    ON ip_security_activity (user_id, ip_address);
CREATE INDEX IF NOT EXISTS idx_ip_security_activity_ip_first_seen
    ON ip_security_activity (ip_address, first_seen_at DESC);

CREATE TABLE IF NOT EXISTS ip_security_bans (
    id BIGSERIAL PRIMARY KEY,
    ip_address VARCHAR(45) NOT NULL UNIQUE,
    status VARCHAR(16) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'whitelisted', 'released')),
    reason TEXT NOT NULL DEFAULT '',
    account_threshold INTEGER NOT NULL,
    window_minutes INTEGER NOT NULL,
    detected_account_count INTEGER NOT NULL DEFAULT 0,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    released_at TIMESTAMPTZ,
    released_by BIGINT REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_ip_security_bans_status_created
    ON ip_security_bans (status, created_at DESC);

INSERT INTO settings (key, value, updated_at) VALUES
    ('ip_multi_account_ban_enabled', 'false', NOW()),
    ('ip_multi_account_ban_window_minutes', '10', NOW()),
    ('ip_multi_account_ban_threshold', '4', NOW()),
    ('ip_multi_account_ban_learning_until', TO_CHAR((NOW() + INTERVAL '72 hours') AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), NOW())
ON CONFLICT (key) DO NOTHING;
