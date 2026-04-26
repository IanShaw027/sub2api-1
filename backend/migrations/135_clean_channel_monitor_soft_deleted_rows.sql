-- Migration: 135_clean_channel_monitor_soft_deleted_rows
-- If an environment applied the temporary soft-delete schema but has not yet
-- dropped deleted_at, physically remove soft-deleted monitor history/rollup rows
-- before dropping the columns. Environments that already applied migration 127
-- with the original drop-column behavior will skip these blocks.

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
