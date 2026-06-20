-- Seed TLS fingerprint profiles for common browsers/clients.
-- Enables TLS fingerprint with random profile selection for all OpenAI OAuth accounts.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

-- 1. Chrome 131 (Windows)
INSERT INTO tls_fingerprint_profiles (name, description, enable_grease, cipher_suites, curves, point_formats, signature_algorithms, alpn_protocols, supported_versions, key_share_groups, psk_modes, extensions)
VALUES (
    'Chrome 131 (Windows)',
    'Chrome 131 on Windows 10/11 — Chromium BoringSSL with GREASE',
    true,
    '[4865,4866,4867,49195,49199,49196,49200,52393,52392,49171,49172,156,157,47,53]',
    '[29,23,24]',
    '[0]',
    '[1027,2052,1025,1283,2053,1281,2054,1537]',
    '["h2","http/1.1"]',
    '[772,771]',
    '[29,23]',
    '[1]',
    '[45,0,65037,17513,35,10,13,65281,16,51,23,27,18,43,11,5]'
) ON CONFLICT (name) DO NOTHING;

-- 2. Chrome 131 (macOS)
INSERT INTO tls_fingerprint_profiles (name, description, enable_grease, cipher_suites, curves, point_formats, signature_algorithms, alpn_protocols, supported_versions, key_share_groups, psk_modes, extensions)
VALUES (
    'Chrome 131 (macOS)',
    'Chrome 131 on macOS — same BoringSSL stack, compress_certificate extension',
    true,
    '[4865,4866,4867,49195,49199,49196,49200,52393,52392,49171,49172,156,157,47,53]',
    '[29,23,24]',
    '[0]',
    '[1027,2052,1025,1283,2053,1281,2054,1537]',
    '["h2","http/1.1"]',
    '[772,771]',
    '[29,23]',
    '[1]',
    '[0,65037,23,65281,10,11,35,16,5,13,18,51,45,43,27,17513]'
) ON CONFLICT (name) DO NOTHING;

-- 3. Edge 131 (Windows)
INSERT INTO tls_fingerprint_profiles (name, description, enable_grease, cipher_suites, curves, point_formats, signature_algorithms, alpn_protocols, supported_versions, key_share_groups, psk_modes, extensions)
VALUES (
    'Edge 131 (Windows)',
    'Microsoft Edge 131 on Windows — Chromium-based, same BoringSSL with slightly different extension order',
    true,
    '[4865,4866,4867,49195,49199,49196,49200,52393,52392,49171,49172,156,157,47,53]',
    '[29,23,24]',
    '[0]',
    '[1027,2052,1025,1283,2053,1281,2054,1537]',
    '["h2","http/1.1"]',
    '[772,771]',
    '[29,23]',
    '[1]',
    '[0,65037,23,65281,10,11,35,16,5,13,18,51,45,17513,43,27]'
) ON CONFLICT (name) DO NOTHING;

-- 4. Firefox 133 (Windows)
INSERT INTO tls_fingerprint_profiles (name, description, enable_grease, cipher_suites, curves, point_formats, signature_algorithms, alpn_protocols, supported_versions, key_share_groups, psk_modes, extensions)
VALUES (
    'Firefox 133 (Windows)',
    'Firefox 133 on Windows — Mozilla NSS TLS stack, no GREASE, broader curve support',
    false,
    '[4865,4867,4866,49195,49199,52393,52392,49196,49200,49162,49161,49171,49172,156,157,47,53]',
    '[29,23,24,25,256,257]',
    '[0]',
    '[1027,1283,1539,2052,2053,2054,1025,1281,1537,515,513]',
    '["h2","http/1.1"]',
    '[772,771]',
    '[29,23]',
    '[1]',
    '[0,23,65281,10,11,35,16,5,34,51,43,13,45,28,27,65037]'
) ON CONFLICT (name) DO NOTHING;

-- 5. Firefox 133 (macOS)
INSERT INTO tls_fingerprint_profiles (name, description, enable_grease, cipher_suites, curves, point_formats, signature_algorithms, alpn_protocols, supported_versions, key_share_groups, psk_modes, extensions)
VALUES (
    'Firefox 133 (macOS)',
    'Firefox 133 on macOS — NSS stack, delegated_credentials extension',
    false,
    '[4865,4867,4866,49195,49199,52393,52392,49196,49200,49162,49161,49171,49172,156,157,47,53]',
    '[29,23,24,25,256,257]',
    '[0]',
    '[1027,1283,1539,2052,2053,2054,1025,1281,1537,515,513]',
    '["h2","http/1.1"]',
    '[772,771]',
    '[29,23]',
    '[1]',
    '[0,23,65281,10,11,16,5,34,51,43,13,45,28,35,27,65037]'
) ON CONFLICT (name) DO NOTHING;

-- 6. Safari 18 (macOS)
INSERT INTO tls_fingerprint_profiles (name, description, enable_grease, cipher_suites, curves, point_formats, signature_algorithms, alpn_protocols, supported_versions, key_share_groups, psk_modes, extensions)
VALUES (
    'Safari 18 (macOS)',
    'Safari 18 on macOS Sequoia/Sonoma — Apple Network.framework TLS stack',
    false,
    '[4865,4866,4867,49196,49195,52393,49200,49199,52392,49162,49161,49172,49171,157,156,53,47]',
    '[29,23,24,25]',
    '[0]',
    '[1027,1283,1539,515,2052,2053,2054,1025,1281,1537,513]',
    '["h2","http/1.1"]',
    '[772,771]',
    '[29,23]',
    '[1]',
    '[0,23,65281,10,11,16,5,13,18,51,45,43,27,17513]'
) ON CONFLICT (name) DO NOTHING;

-- 7. Safari 18 (iOS)
INSERT INTO tls_fingerprint_profiles (name, description, enable_grease, cipher_suites, curves, point_formats, signature_algorithms, alpn_protocols, supported_versions, key_share_groups, psk_modes, extensions)
VALUES (
    'Safari 18 (iOS)',
    'Safari 18 on iOS 18 — Apple Network.framework, fewer legacy ciphers',
    false,
    '[4865,4866,4867,49196,49195,52393,49200,49199,52392,49172,49171,157,156,53,47]',
    '[29,23,24,25]',
    '[0]',
    '[1027,1283,1539,515,2052,2053,2054,1025,1281,1537]',
    '["h2","http/1.1"]',
    '[772,771]',
    '[29,23]',
    '[1]',
    '[0,23,65281,10,11,16,5,13,18,51,45,43,27]'
) ON CONFLICT (name) DO NOTHING;

-- 8. Chrome 126 (Android)
INSERT INTO tls_fingerprint_profiles (name, description, enable_grease, cipher_suites, curves, point_formats, signature_algorithms, alpn_protocols, supported_versions, key_share_groups, psk_modes, extensions)
VALUES (
    'Chrome 126 (Android)',
    'Chrome 126 on Android — older Chromium BoringSSL without post-quantum',
    true,
    '[4865,4866,4867,49195,49199,49196,49200,52393,52392,49171,49172,156,157,47,53]',
    '[29,23,24]',
    '[0]',
    '[1027,2052,1025,1283,2053,1281,2054,1537]',
    '["h2","http/1.1"]',
    '[772,771]',
    '[29,23]',
    '[1]',
    '[0,23,65281,10,11,35,16,5,13,18,51,45,43,27]'
) ON CONFLICT (name) DO NOTHING;

-- 9. Node.js 24.x
INSERT INTO tls_fingerprint_profiles (name, description, enable_grease, cipher_suites, curves, point_formats, signature_algorithms, alpn_protocols, supported_versions, key_share_groups, psk_modes, extensions)
VALUES (
    'Node.js 24.x',
    'Node.js 24 OpenSSL 3.x TLS stack — matches Claude Code / API SDK clients',
    false,
    '[4865,4866,4867,49195,49199,49196,49200,52393,52392,49161,49171,49162,49172,156,157,47,53]',
    '[29,23,24]',
    '[0]',
    '[1027,2052,1025,1283,2053,1281,2054,1537,513]',
    '["http/1.1"]',
    '[772,771]',
    '[29]',
    '[1]',
    '[0,65037,23,65281,10,11,35,16,5,13,18,51,45,43]'
) ON CONFLICT (name) DO NOTHING;

-- 10. Node.js 22.x
INSERT INTO tls_fingerprint_profiles (name, description, enable_grease, cipher_suites, curves, point_formats, signature_algorithms, alpn_protocols, supported_versions, key_share_groups, psk_modes, extensions)
VALUES (
    'Node.js 22.x',
    'Node.js 22 LTS OpenSSL 3.x — no ECH extension, http/1.1 only',
    false,
    '[4865,4866,4867,49195,49199,49196,49200,52393,52392,49161,49171,49162,49172,156,157,47,53]',
    '[29,23,24]',
    '[0]',
    '[1027,2052,1025,1283,2053,1281,2054,1537,513]',
    '["http/1.1"]',
    '[772,771]',
    '[29]',
    '[1]',
    '[0,23,65281,10,11,35,16,5,13,18,51,45,43]'
) ON CONFLICT (name) DO NOTHING;

-- 11. Go net/http 1.22
INSERT INTO tls_fingerprint_profiles (name, description, enable_grease, cipher_suites, curves, point_formats, signature_algorithms, alpn_protocols, supported_versions, key_share_groups, psk_modes, extensions)
VALUES (
    'Go 1.22 net/http',
    'Go 1.22 standard library crypto/tls — common for API backends and CLI tools',
    false,
    '[4865,4866,4867,49195,49199,49196,49200,52393,52392,49171,49172,156,157,47,53]',
    '[29,23,24,25]',
    '[0]',
    '[2052,1027,2053,1283,2054,1539,2055,1025,1281,1537]',
    '["h2","http/1.1"]',
    '[772,771]',
    '[29,23]',
    '[1]',
    '[0,23,65281,10,11,43,13,45,51,5,16,35]'
) ON CONFLICT (name) DO NOTHING;

-- 12. Python httpx/aiohttp (OpenSSL 3.x)
INSERT INTO tls_fingerprint_profiles (name, description, enable_grease, cipher_suites, curves, point_formats, signature_algorithms, alpn_protocols, supported_versions, key_share_groups, psk_modes, extensions)
VALUES (
    'Python httpx (OpenSSL 3.x)',
    'Python httpx/aiohttp with default OpenSSL 3.x configuration — common API client',
    false,
    '[4865,4866,4867,49195,49199,49196,49200,52393,52392,49171,49172,156,157,47,53]',
    '[29,23,24]',
    '[0]',
    '[1027,2052,1025,1283,2053,1281,2054,1537,513]',
    '["h2","http/1.1"]',
    '[772,771]',
    '[29,23]',
    '[1]',
    '[0,23,65281,10,11,35,16,5,13,51,45,43,18]'
) ON CONFLICT (name) DO NOTHING;

-- Enable TLS fingerprint with random profile selection (-1) for all OpenAI OAuth accounts
UPDATE accounts
SET extra = COALESCE(extra, '{}'::jsonb)
          || '{"enable_tls_fingerprint": true, "tls_fingerprint_profile_id": -1}'::jsonb
WHERE platform = 'openai'
  AND type = 'oauth'
  AND (extra IS NULL OR NOT (extra ? 'enable_tls_fingerprint'));
