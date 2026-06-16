-- Snapshot account platform onto usage rows so analytics do not depend on
-- joining mutable account metadata.
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT '';
