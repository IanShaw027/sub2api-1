-- TLS fingerprint routers: match inbound OS / client / protocol / UA to a profile.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

CREATE TABLE IF NOT EXISTS tls_fingerprint_routers (
    id          BIGSERIAL    PRIMARY KEY,
    name        VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    enabled     BOOLEAN      NOT NULL DEFAULT true,
    rules       JSONB        NOT NULL DEFAULT '[]'::jsonb,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tls_fingerprint_routers_enabled
    ON tls_fingerprint_routers (enabled);

COMMENT ON TABLE tls_fingerprint_routers IS 'TLS fingerprint routers that select a profile by OS, client, protocol, or UA';
COMMENT ON COLUMN tls_fingerprint_routers.rules IS 'Ordered first-match-wins rules; conditions inside a rule are AND';
