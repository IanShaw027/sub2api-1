-- Migration: 134_align_channel_monitor_request_template_constraints
-- Align request-template index/FK names with Ent generated schema while preserving
-- compatibility for databases that already applied migration 128.

DO $$
BEGIN
    IF to_regclass('public.channel_monitor_request_templates_provider_name') IS NOT NULL
       AND to_regclass('public.channelmonitorrequesttemplate_provider_name') IS NULL THEN
        ALTER INDEX channel_monitor_request_templates_provider_name
            RENAME TO channelmonitorrequesttemplate_provider_name;
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS channelmonitorrequesttemplate_provider_name
    ON channel_monitor_request_templates (provider, name);

DROP INDEX IF EXISTS channel_monitor_request_templates_provider_name;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.table_constraints
        WHERE table_schema = 'public'
          AND table_name = 'channel_monitors'
          AND constraint_name = 'channel_monitors_template_id_fkey'
    ) AND NOT EXISTS (
        SELECT 1
        FROM information_schema.table_constraints
        WHERE table_schema = 'public'
          AND table_name = 'channel_monitors'
          AND constraint_name = 'channel_monitors_channel_monitor_request_templates_request_template'
    ) THEN
        ALTER TABLE channel_monitors
            RENAME CONSTRAINT channel_monitors_template_id_fkey
            TO channel_monitors_channel_monitor_request_templates_request_template;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.table_constraints
        WHERE table_schema = 'public'
          AND table_name = 'channel_monitors'
          AND constraint_name = 'channel_monitors_channel_monitor_request_templates_request_template'
    ) THEN
        ALTER TABLE channel_monitors
            ADD CONSTRAINT channel_monitors_channel_monitor_request_templates_request_template
            FOREIGN KEY (template_id)
            REFERENCES channel_monitor_request_templates (id)
            ON DELETE SET NULL;
    END IF;
END $$;
