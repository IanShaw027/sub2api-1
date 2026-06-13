-- Seed OpenAI API key temp-unschedulable rules.
--
-- Safety:
-- 1. Run the preview SELECTs first and review the account list/counts.
-- 2. The UPDATE section ends with ROLLBACK by default.
-- 3. After explicit confirmation, change the final ROLLBACK to COMMIT.

-- Preview: total target accounts.
SELECT COUNT(*) AS target_openai_apikey_accounts
FROM accounts
WHERE LOWER(platform) = 'openai'
  AND type = 'apikey'
  AND deleted_at IS NULL;

-- Preview: pool-mode split. Pool-mode accounts skip local marking unless custom error handling is enabled.
SELECT
  COALESCE(credentials->>'pool_mode', 'false') AS pool_mode,
  COALESCE(credentials->>'custom_error_codes_enabled', 'false') AS custom_error_codes_enabled,
  COUNT(*) AS account_count
FROM accounts
WHERE LOWER(platform) = 'openai'
  AND type = 'apikey'
  AND deleted_at IS NULL
GROUP BY 1, 2
ORDER BY 1, 2;

-- Preview: affected account list and existing temp-unsched state.
SELECT
  id,
  name,
  status,
  COALESCE(credentials->>'pool_mode', 'false') AS pool_mode,
  COALESCE(credentials->>'temp_unschedulable_enabled', 'false') AS temp_unschedulable_enabled,
  jsonb_array_length(
    CASE
      WHEN jsonb_typeof(credentials->'temp_unschedulable_rules') = 'array'
        THEN credentials->'temp_unschedulable_rules'
      ELSE '[]'::jsonb
    END
  ) AS existing_rule_count
FROM accounts
WHERE LOWER(platform) = 'openai'
  AND type = 'apikey'
  AND deleted_at IS NULL
ORDER BY id;

BEGIN;

WITH desired_rules AS (
  SELECT *
  FROM (
    VALUES
      (1, jsonb_build_object(
        'error_code', 502,
        'keywords', jsonb_build_array('Upstream service temporarily unavailable'),
        'duration_minutes', 10,
        'description', 'OpenAI upstream service temporarily unavailable'
      )),
      (2, jsonb_build_object(
        'error_code', 502,
        'keywords', jsonb_build_array('Upstream request failed'),
        'duration_minutes', 10,
        'description', 'OpenAI upstream request failed'
      )),
      (3, jsonb_build_object(
        'error_code', 503,
        'keywords', jsonb_build_array('Service temporarily unavailable'),
        'duration_minutes', 10,
        'description', 'OpenAI service temporarily unavailable'
      )),
      (4, jsonb_build_object(
        'error_code', 503,
        'keywords', jsonb_build_array('overloaded'),
        'duration_minutes', 10,
        'description', 'OpenAI currently overloaded'
      )),
      (5, jsonb_build_object(
        'error_code', 500,
        'keywords', jsonb_build_array('upstream connection failed', 'Upstream transport error'),
        'duration_minutes', 10,
        'description', 'OpenAI upstream connection or transport error'
      )),
      (6, jsonb_build_object(
        'error_code', 502,
        'keywords', jsonb_build_array('Upstream access forbidden'),
        'duration_minutes', 10,
        'description', 'OpenAI upstream access forbidden'
      ))
  ) AS rules(ord, rule)
),
target_accounts AS (
  SELECT
    id,
    COALESCE(credentials, '{}'::jsonb) AS credentials
  FROM accounts
  WHERE LOWER(platform) = 'openai'
    AND type = 'apikey'
    AND deleted_at IS NULL
),
existing_rules AS (
  SELECT
    target_accounts.id,
    existing.rule,
    existing.ord
  FROM target_accounts
  CROSS JOIN LATERAL jsonb_array_elements(
    CASE
      WHEN jsonb_typeof(target_accounts.credentials->'temp_unschedulable_rules') = 'array'
        THEN target_accounts.credentials->'temp_unschedulable_rules'
      ELSE '[]'::jsonb
    END
  ) WITH ORDINALITY AS existing(rule, ord)
),
merged_rule_rows AS (
  SELECT
    id,
    rule,
    0 AS source_order,
    ord
  FROM existing_rules

  UNION ALL

  SELECT
    target_accounts.id,
    desired_rules.rule,
    1 AS source_order,
    desired_rules.ord
  FROM target_accounts
  CROSS JOIN desired_rules
  WHERE NOT EXISTS (
    SELECT 1
    FROM existing_rules
    WHERE existing_rules.id = target_accounts.id
      AND existing_rules.rule->>'error_code' = desired_rules.rule->>'error_code'
      AND existing_rules.rule->>'duration_minutes' = desired_rules.rule->>'duration_minutes'
      AND COALESCE(existing_rules.rule->'keywords', '[]'::jsonb) = desired_rules.rule->'keywords'
  )
),
merged_rules AS (
  SELECT
    id,
    jsonb_agg(rule ORDER BY source_order, ord) AS rules
  FROM merged_rule_rows
  GROUP BY id
),
updated_accounts AS (
  UPDATE accounts AS a
  SET
    credentials = jsonb_set(
      jsonb_set(
        COALESCE(a.credentials, '{}'::jsonb),
        '{temp_unschedulable_rules}',
        merged_rules.rules,
        true
      ),
      '{temp_unschedulable_enabled}',
      'true'::jsonb,
      true
    ),
    updated_at = NOW()
  FROM merged_rules
  WHERE a.id = merged_rules.id
  RETURNING
    a.id,
    a.name,
    a.status,
    jsonb_array_length(a.credentials->'temp_unschedulable_rules') AS final_rule_count
)
SELECT *
FROM updated_accounts
ORDER BY id;

-- Change to COMMIT only after the preview and returned account list are approved.
ROLLBACK;
