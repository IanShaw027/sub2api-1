-- Separate global facts from the real ungrouped bucket and discard historical
-- estimates that were derived from the account pool at read time.

-- Existing group_id=0 rows are ambiguous: older code used the same key for
-- global and ungrouped requests. Rebuild them from usage_logs under distinct keys.
DELETE FROM openai_oauth_capacity_hourly
WHERE group_id = 0;

UPDATE openai_oauth_capacity_hourly
SET capacity_5h_usd = NULL,
    available_5h_usd = NULL,
    used_percent_5h = NULL,
    capacity_7d_usd = NULL,
    available_7d_usd = NULL,
    used_percent_7d = NULL,
    computed_at = NOW()
WHERE capacity_5h_usd IS NOT NULL
   OR available_5h_usd IS NOT NULL
   OR used_percent_5h IS NOT NULL
   OR capacity_7d_usd IS NOT NULL
   OR available_7d_usd IS NOT NULL
   OR used_percent_7d IS NOT NULL;

ALTER TABLE openai_oauth_capacity_hourly
    ALTER COLUMN group_id SET DEFAULT -1;

COMMENT ON COLUMN openai_oauth_capacity_hourly.group_id IS
    '-1 = global total; 0 = ungrouped; positive values are usage_logs.group_id at request time.';
