CREATE OR REPLACE FUNCTION enqueue_scheduler_account_change()
RETURNS TRIGGER AS $$
DECLARE
    target_id BIGINT;
    event_name TEXT;
    event_key TEXT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        target_id := OLD.id;
    ELSE
        target_id := NEW.id;
    END IF;
    IF TG_OP = 'DELETE' THEN
        event_name := 'full_rebuild';
        event_key := 'dbtrigger:full_rebuild';
        INSERT INTO scheduler_outbox (event_type, dedup_key)
        VALUES (event_name, event_key)
        ON CONFLICT (dedup_key) WHERE dedup_key IS NOT NULL DO NOTHING;
    ELSE
        event_name := 'account_changed';
        event_key := 'dbtrigger:account:' || target_id::TEXT;
        INSERT INTO scheduler_outbox (event_type, account_id, dedup_key)
        VALUES (event_name, target_id, event_key)
        ON CONFLICT (dedup_key) WHERE dedup_key IS NOT NULL DO NOTHING;
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_accounts_scheduler_insert_delete ON accounts;
CREATE TRIGGER trg_accounts_scheduler_insert_delete
AFTER INSERT OR DELETE ON accounts
FOR EACH ROW EXECUTE FUNCTION enqueue_scheduler_account_change();

DROP TRIGGER IF EXISTS trg_accounts_scheduler_relevant_update ON accounts;
CREATE TRIGGER trg_accounts_scheduler_relevant_update
AFTER UPDATE OF
    platform, type, proxy_id, concurrency, priority, status, schedulable,
    rate_limited_at, rate_limit_reset_at, overload_until,
    session_window_start, session_window_end, session_window_status,
    temp_unschedulable_until, temp_unschedulable_reason,
    expires_at, auto_pause_on_expired, rate_multiplier, load_factor,
    proxy_fallback_origin_id, parent_account_id, quota_dimension, deleted_at
ON accounts
FOR EACH ROW
WHEN (
    (to_jsonb(NEW) - ARRAY['updated_at', 'last_used_at', 'credentials', 'extra', 'name', 'notes', 'error_message'])
    IS DISTINCT FROM
    (to_jsonb(OLD) - ARRAY['updated_at', 'last_used_at', 'credentials', 'extra', 'name', 'notes', 'error_message'])
)
EXECUTE FUNCTION enqueue_scheduler_account_change();

-- credentials and extra contain both high-frequency operational data and
-- scheduler inputs. Compare only the subset persisted in scheduler snapshots
-- so token refreshes and telemetry writes do not cause bucket rebuild storms.
CREATE OR REPLACE FUNCTION scheduler_relevant_json_subset(document JSONB, keys TEXT[])
RETURNS JSONB AS $$
    SELECT COALESCE(jsonb_object_agg(entry.key, entry.value), '{}'::jsonb)
    FROM jsonb_each(COALESCE(document, '{}'::jsonb)) AS entry
    WHERE entry.key = ANY(keys);
$$ LANGUAGE SQL IMMUTABLE PARALLEL SAFE;

CREATE OR REPLACE FUNCTION enqueue_scheduler_account_json_change()
RETURNS TRIGGER AS $$
DECLARE
    credential_keys CONSTANT TEXT[] := ARRAY[
        'model_mapping', 'compact_model_mapping', 'api_key', 'project_id',
        'oauth_type', 'tier_id', 'plan_type', 'plan_name',
        'gemini_paid_tier_id', 'gemini_paid_tier_name',
        'gemini_current_tier_id', 'gemini_current_tier_name',
        'gemini_status', 'gemini_status_reason',
        'quota_query_last_error', 'quota_query_last_error_at'
    ];
    extra_keys CONSTANT TEXT[] := ARRAY[
        'model_rate_limits', 'mixed_scheduling', 'window_cost_limit',
        'window_cost_sticky_reserve', 'max_sessions',
        'session_idle_timeout_minutes', 'oauth_type', 'tier_id',
        'plan_type', 'plan_name', 'gemini_paid_tier_id',
        'gemini_paid_tier_name', 'gemini_current_tier_id',
        'gemini_current_tier_name', 'gemini_status', 'gemini_status_reason',
        'quota_query_last_error', 'quota_query_last_error_at',
        'openai_oauth_responses_websockets_v2_enabled',
        'openai_oauth_responses_websockets_v2_mode',
        'openai_apikey_responses_websockets_v2_enabled',
        'openai_apikey_responses_websockets_v2_mode',
        'responses_websockets_v2_enabled', 'openai_ws_enabled',
        'openai_ws_force_http', 'openai_responses_mode',
        'openai_responses_supported', 'codex_5h_used_percent',
        'codex_7d_used_percent', 'codex_5h_reset_at', 'codex_7d_reset_at',
        'codex_5h_reset_after_seconds', 'codex_7d_reset_after_seconds',
        'codex_usage_updated_at', 'auto_pause_5h_threshold',
        'auto_pause_7d_threshold', 'auto_pause_5h_disabled',
        'auto_pause_7d_disabled'
    ];
BEGIN
    IF scheduler_relevant_json_subset(NEW.credentials, credential_keys)
        IS NOT DISTINCT FROM scheduler_relevant_json_subset(OLD.credentials, credential_keys)
       AND scheduler_relevant_json_subset(NEW.extra, extra_keys)
        IS NOT DISTINCT FROM scheduler_relevant_json_subset(OLD.extra, extra_keys) THEN
        RETURN NEW;
    END IF;

    INSERT INTO scheduler_outbox (event_type, account_id, dedup_key)
    VALUES ('account_changed', NEW.id, 'dbtrigger:account:' || NEW.id::TEXT)
    ON CONFLICT (dedup_key) WHERE dedup_key IS NOT NULL DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_accounts_scheduler_json_update ON accounts;
CREATE TRIGGER trg_accounts_scheduler_json_update
AFTER UPDATE OF credentials, extra ON accounts
FOR EACH ROW EXECUTE FUNCTION enqueue_scheduler_account_json_change();

CREATE OR REPLACE FUNCTION enqueue_scheduler_group_change()
RETURNS TRIGGER AS $$
DECLARE
    target_id BIGINT;
    event_key TEXT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        target_id := OLD.id;
    ELSE
        target_id := NEW.id;
    END IF;
    event_key := 'dbtrigger:group:' || target_id::TEXT;
    INSERT INTO scheduler_outbox (event_type, group_id, dedup_key)
    VALUES ('group_changed', target_id, event_key)
    ON CONFLICT (dedup_key) WHERE dedup_key IS NOT NULL DO NOTHING;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_groups_scheduler_change ON groups;
CREATE TRIGGER trg_groups_scheduler_change
AFTER INSERT OR UPDATE OR DELETE ON groups
FOR EACH ROW EXECUTE FUNCTION enqueue_scheduler_group_change();

CREATE OR REPLACE FUNCTION enqueue_scheduler_membership_change()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO scheduler_outbox (event_type, dedup_key)
    VALUES ('full_rebuild', 'dbtrigger:full_rebuild')
    ON CONFLICT (dedup_key) WHERE dedup_key IS NOT NULL DO NOTHING;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_account_groups_scheduler_change ON account_groups;
CREATE TRIGGER trg_account_groups_scheduler_change
AFTER INSERT OR UPDATE OR DELETE ON account_groups
FOR EACH ROW EXECUTE FUNCTION enqueue_scheduler_membership_change();
