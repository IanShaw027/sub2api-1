-- Removes the synthetic TLS fingerprint seed data that was replaced by live capture.
-- This is intentionally a forward cleanup migration because some environments may
-- already have applied 171_seed_tls_fingerprint_profiles.sql before it was removed.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

WITH synthetic_profiles(name, description) AS (
    VALUES
        ('Chrome 131 (Windows)', 'Chrome 131 on Windows 10/11 — Chromium BoringSSL with GREASE'),
        ('Chrome 131 (macOS)', 'Chrome 131 on macOS — same BoringSSL stack, compress_certificate extension'),
        ('Edge 131 (Windows)', 'Microsoft Edge 131 on Windows — Chromium-based, same BoringSSL with slightly different extension order'),
        ('Firefox 133 (Windows)', 'Firefox 133 on Windows — Mozilla NSS TLS stack, no GREASE, broader curve support'),
        ('Firefox 133 (macOS)', 'Firefox 133 on macOS — NSS stack, delegated_credentials extension'),
        ('Safari 18 (macOS)', 'Safari 18 on macOS Sequoia/Sonoma — Apple Network.framework TLS stack'),
        ('Safari 18 (iOS)', 'Safari 18 on iOS 18 — Apple Network.framework, fewer legacy ciphers'),
        ('Chrome 126 (Android)', 'Chrome 126 on Android — older Chromium BoringSSL without post-quantum'),
        ('Node.js 24.x', 'Node.js 24 OpenSSL 3.x TLS stack — matches Claude Code / API SDK clients'),
        ('Node.js 22.x', 'Node.js 22 LTS OpenSSL 3.x — no ECH extension, http/1.1 only'),
        ('Go 1.22 net/http', 'Go 1.22 standard library crypto/tls — common for API backends and CLI tools'),
        ('Python httpx (OpenSSL 3.x)', 'Python httpx/aiohttp with default OpenSSL 3.x configuration — common API client')
)
DELETE FROM tls_fingerprint_profiles p
USING synthetic_profiles s
WHERE p.name = s.name
  AND p.description = s.description;

UPDATE accounts
SET extra = (COALESCE(extra, '{}'::jsonb) - 'enable_tls_fingerprint' - 'tls_fingerprint_profile_id')
WHERE platform = 'openai'
  AND type = 'oauth'
  AND COALESCE(extra, '{}'::jsonb) = '{"enable_tls_fingerprint": true, "tls_fingerprint_profile_id": -1}'::jsonb;
