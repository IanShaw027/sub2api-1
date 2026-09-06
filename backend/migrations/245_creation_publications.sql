-- Only an explicit media upload creates a public record. Existing image jobs remain private.
CREATE TABLE IF NOT EXISTS creation_publications (
    id BIGSERIAL PRIMARY KEY,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    request_id UUID NOT NULL,
    title VARCHAR(160) NOT NULL,
    prompt TEXT NOT NULL DEFAULT '',
    model VARCHAR(128) NOT NULL DEFAULT '',
    kind VARCHAR(16) NOT NULL CHECK (kind IN ('image', 'video')),
    status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'published')),
    mime VARCHAR(64) NOT NULL CHECK (mime IN ('image/png', 'image/jpeg', 'image/gif', 'image/webp', 'video/mp4', 'video/webm')),
    size BIGINT NOT NULL CHECK (size > 0 AND size <= 67108864),
    storage_key TEXT NOT NULL UNIQUE,
    storage_profile_id VARCHAR(128) NOT NULL,
    sha256 VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    withdrawn_at TIMESTAMPTZ,
    CONSTRAINT creation_publications_owner_request_unique UNIQUE (owner_user_id, request_id)
);

CREATE INDEX IF NOT EXISTS creation_publications_gallery_idx
    ON creation_publications (created_at DESC, id DESC) WHERE withdrawn_at IS NULL AND status = 'published';
CREATE INDEX IF NOT EXISTS creation_publications_kind_gallery_idx
    ON creation_publications (kind, created_at DESC, id DESC) WHERE withdrawn_at IS NULL AND status = 'published';
