-- Adds OS and client-type classification to TLS fingerprint profiles so that
-- templates can be filtered/bound per operating system (windows/macos/linux)
-- and per client (e.g. codex-cli, chatgpt-desktop, claude-code, browser).
--
-- Both columns default to '' which means "agnostic" and preserves the existing
-- behaviour of every current profile (no OS/client restriction).

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE tls_fingerprint_profiles
    ADD COLUMN IF NOT EXISTS os VARCHAR(20) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS client_type VARCHAR(50) NOT NULL DEFAULT '';

COMMENT ON COLUMN tls_fingerprint_profiles.os IS 'Optional OS classification (windows/macos/linux); empty means OS-agnostic / shared';
COMMENT ON COLUMN tls_fingerprint_profiles.client_type IS 'Optional client classification within a platform (codex-cli/chatgpt-desktop/claude-code/browser); empty means client-agnostic';

CREATE INDEX IF NOT EXISTS idx_tls_fingerprint_profiles_dims
    ON tls_fingerprint_profiles (platform, os, client_type);
