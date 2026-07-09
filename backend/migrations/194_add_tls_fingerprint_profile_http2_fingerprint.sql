ALTER TABLE tls_fingerprint_profiles
    ADD COLUMN IF NOT EXISTS http2_fingerprint TEXT NOT NULL DEFAULT '';

COMMENT ON COLUMN tls_fingerprint_profiles.http2_fingerprint IS
    'Optional captured HTTP/2 fingerprint preserved for fidelity and h2-aware deduplication';
