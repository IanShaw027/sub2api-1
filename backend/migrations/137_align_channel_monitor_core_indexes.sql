-- Migration: 137_align_channel_monitor_core_indexes
-- Align early channel monitor index names with the current Ent schema.
-- Do not edit already-published 125/126 migrations; normalize live schemas here.

ALTER INDEX IF EXISTS idx_channel_monitors_enabled_last_checked
    RENAME TO channelmonitor_enabled_last_checked_at;
ALTER INDEX IF EXISTS idx_channel_monitors_provider
    RENAME TO channelmonitor_provider;
ALTER INDEX IF EXISTS idx_channel_monitors_group_name
    RENAME TO channelmonitor_group_name;

ALTER INDEX IF EXISTS idx_channel_monitor_histories_monitor_model_checked
    RENAME TO channelmonitorhistory_monitor_id_model_checked_at;
ALTER INDEX IF EXISTS idx_channel_monitor_histories_checked_at_id
    RENAME TO channelmonitorhistory_checked_at;

ALTER INDEX IF EXISTS idx_channel_monitor_daily_rollups_unique
    RENAME TO channelmonitordailyrollup_monitor_id_model_bucket_date;
ALTER INDEX IF EXISTS idx_channel_monitor_daily_rollups_bucket_id
    RENAME TO channelmonitordailyrollup_bucket_date;

CREATE INDEX IF NOT EXISTS channelmonitor_enabled_last_checked_at
    ON channel_monitors (enabled, last_checked_at);
CREATE INDEX IF NOT EXISTS channelmonitor_provider
    ON channel_monitors (provider);
CREATE INDEX IF NOT EXISTS channelmonitor_group_name
    ON channel_monitors (group_name);

CREATE INDEX IF NOT EXISTS channelmonitorhistory_monitor_id_model_checked_at
    ON channel_monitor_histories (monitor_id, model, checked_at DESC);
CREATE INDEX IF NOT EXISTS channelmonitorhistory_checked_at
    ON channel_monitor_histories (checked_at);

CREATE UNIQUE INDEX IF NOT EXISTS channelmonitordailyrollup_monitor_id_model_bucket_date
    ON channel_monitor_daily_rollups (monitor_id, model, bucket_date);
CREATE INDEX IF NOT EXISTS channelmonitordailyrollup_bucket_date
    ON channel_monitor_daily_rollups (bucket_date);

DROP INDEX IF EXISTS idx_channel_monitors_enabled_last_checked;
DROP INDEX IF EXISTS idx_channel_monitors_provider;
DROP INDEX IF EXISTS idx_channel_monitors_group_name;
DROP INDEX IF EXISTS idx_channel_monitor_histories_monitor_model_checked;
DROP INDEX IF EXISTS idx_channel_monitor_histories_checked_at_id;
DROP INDEX IF EXISTS idx_channel_monitor_daily_rollups_unique;
DROP INDEX IF EXISTS idx_channel_monitor_daily_rollups_bucket_id;
