-- Per-account outbound identity. One row per canonical account_id.
-- Redis may project this row; it is never the authority.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

CREATE TABLE IF NOT EXISTS account_device_profiles (
    id                    BIGSERIAL    PRIMARY KEY,
    account_id            BIGINT       NOT NULL UNIQUE REFERENCES accounts(id) ON DELETE CASCADE,
    revision              BIGINT       NOT NULL DEFAULT 1,
    schema_version        INTEGER      NOT NULL DEFAULT 1,
    platform              VARCHAR(32)  NOT NULL,
    client_family         VARCHAR(32)  NOT NULL,
    installation_id       VARCHAR(64)  NOT NULL,
    device_id             VARCHAR(64)  NOT NULL,
    client_id             VARCHAR(64)  NOT NULL DEFAULT '',
    machine_id            VARCHAR(64)  NOT NULL,
    gateway_account_uuid  VARCHAR(64)  NOT NULL,
    session_namespace     VARCHAR(64)  NOT NULL,
    os_family             VARCHAR(32)  NOT NULL,
    arch                  VARCHAR(32)  NOT NULL,
    runtime               VARCHAR(32)  NOT NULL,
    runtime_version       VARCHAR(64)  NOT NULL,
    client_version        VARCHAR(64)  NOT NULL,
    tls_profile_id        BIGINT       REFERENCES tls_fingerprint_profiles(id) ON DELETE SET NULL,
    transport_family      VARCHAR(8)   NOT NULL,
    profile_payload       JSONB        NOT NULL DEFAULT '{}'::jsonb,
    learned_from          VARCHAR(32)  NOT NULL,
    learning_enabled      BOOLEAN      NOT NULL DEFAULT false,
    version_upgraded_at   TIMESTAMPTZ,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT account_device_profiles_revision_check
        CHECK (revision >= 1),
    CONSTRAINT account_device_profiles_schema_version_check
        CHECK (schema_version BETWEEN 1 AND 100),
    CONSTRAINT account_device_profiles_platform_check
        CHECK (platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kiro')),
    CONSTRAINT account_device_profiles_client_family_check
        CHECK (client_family IN ('claude-code', 'codex-cli', 'grok-cli', 'kiro-ide', 'gemini-cli', 'antigravity')),
    CONSTRAINT account_device_profiles_os_family_check
        CHECK (char_length(os_family) BETWEEN 1 AND 32),
    CONSTRAINT account_device_profiles_arch_check
        CHECK (char_length(arch) BETWEEN 1 AND 32),
    CONSTRAINT account_device_profiles_runtime_check
        CHECK (char_length(runtime) BETWEEN 1 AND 32),
    CONSTRAINT account_device_profiles_session_namespace_check
        CHECK (session_namespace ~ '^[0-9a-f]{32,64}$'),
    CONSTRAINT account_device_profiles_tls_profile_id_check
        CHECK (tls_profile_id IS NULL OR tls_profile_id > 0),
    CONSTRAINT account_device_profiles_transport_family_check
        CHECK (transport_family IN ('h1', 'h2')),
    CONSTRAINT account_device_profiles_learned_from_check
        CHECK (learned_from IN ('baseline', 'official_traffic', 'baseline_floor')),
    CONSTRAINT account_device_profiles_profile_payload_check
        CHECK (jsonb_typeof(profile_payload) = 'object')
);

COMMENT ON TABLE account_device_profiles IS 'Canonical outbound device identity; one row per account; Redis is only a projection';
COMMENT ON COLUMN account_device_profiles.account_id IS 'UNIQUE FK to accounts; shadow accounts must reuse the parent row';
COMMENT ON COLUMN account_device_profiles.revision IS 'CAS token; do not treat schema_version or client_version as CAS';
COMMENT ON COLUMN account_device_profiles.gateway_account_uuid IS 'Gateway-generated UUID; never copy extra.account_uuid';
COMMENT ON COLUMN account_device_profiles.session_namespace IS '32-64 lowercase hex; immutable after insert';
COMMENT ON COLUMN account_device_profiles.tls_profile_id IS 'NULL or positive FK to tls_fingerprint_profiles; -1 is illegal';
COMMENT ON COLUMN account_device_profiles.transport_family IS 'h1 or h2; P1 baselines write h1';
COMMENT ON COLUMN account_device_profiles.learning_enabled IS 'Per-profile learning switch; default false; do not fleet-enable';
