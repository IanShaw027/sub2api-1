-- User-level exit lock: when enabled, only historical users.ips are allowed;
-- unknown client IPs are rejected and globally banned.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS pin_known_ips BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS pin_known_ips_enabled_at TIMESTAMPTZ NULL;

COMMENT ON COLUMN users.pin_known_ips IS 'When true, API requests from IPs not in users.ips are rejected and the IP is globally banned';
COMMENT ON COLUMN users.pin_known_ips_enabled_at IS 'Timestamp when pin_known_ips was last enabled';
