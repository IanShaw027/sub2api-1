-- Migration: 136_align_channel_monitor_template_index_and_cleanup
-- Follow-up for channel monitor schema drift:
--   1) Align channel_monitors.template_id index name/definition with Ent
--      (channelmonitor_template_id, non-partial).
--   2) Safety cleanup for environments that still have deleted_at columns after
--      earlier soft-delete experiments. This intentionally does not rewrite
--      already published migrations 127/135.

DROP INDEX IF EXISTS idx_channel_monitors_template_id;

CREATE INDEX IF NOT EXISTS channelmonitor_template_id
    ON channel_monitors (template_id);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'channel_monitor_histories'
          AND column_name = 'deleted_at'
    ) THEN
        DELETE FROM channel_monitor_histories
        WHERE deleted_at IS NOT NULL;

        DROP INDEX IF EXISTS idx_channel_monitor_histories_deleted_at;

        ALTER TABLE channel_monitor_histories
            DROP COLUMN IF EXISTS deleted_at;
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'channel_monitor_daily_rollups'
          AND column_name = 'deleted_at'
    ) THEN
        DELETE FROM channel_monitor_daily_rollups
        WHERE deleted_at IS NOT NULL;

        DROP INDEX IF EXISTS idx_channel_monitor_daily_rollups_deleted_at;

        ALTER TABLE channel_monitor_daily_rollups
            DROP COLUMN IF EXISTS deleted_at;
    END IF;
END $$;
