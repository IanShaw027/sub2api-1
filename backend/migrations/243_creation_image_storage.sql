-- Retain image results independently of the expiring Redis task record.
ALTER TABLE creation_image_jobs
    ADD COLUMN IF NOT EXISTS media_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS storage_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS storage_key TEXT NOT NULL DEFAULT '';
