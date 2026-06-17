-- Create TLS fingerprint routers for UA-based profile routing.
CREATE TABLE IF NOT EXISTS tls_fingerprint_routers (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    rules JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tls_fingerprint_routers_enabled
    ON tls_fingerprint_routers(enabled);
