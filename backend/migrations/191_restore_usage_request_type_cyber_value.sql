-- Restore request_type=4 as the canonical cyber_policy value and move image rows to 8.
-- request_type=4 was already persisted as cyber before the image enum reused it; keep old image rows
-- distinguishable by image metadata while the data migration rewrites them to the new image value.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'usage_logs_request_type_check'
          AND conrelid = 'usage_logs'::regclass
    ) THEN
        ALTER TABLE usage_logs
            DROP CONSTRAINT usage_logs_request_type_check;
    END IF;

    ALTER TABLE usage_logs
        ADD CONSTRAINT usage_logs_request_type_check
        CHECK (request_type IN (0, 1, 2, 3, 4, 5, 6, 7, 8)) NOT VALID;
END
$$;

UPDATE usage_logs
SET request_type = 8
WHERE request_type = 4
  AND (
      image_count > 0
      OR image_output_tokens > 0
      OR billing_mode = 'image'
      OR inbound_endpoint ILIKE '%/images/%'
      OR upstream_endpoint ILIKE '%/images/%'
      OR inbound_endpoint ILIKE '%/images2api/%'
      OR upstream_endpoint ILIKE '%/images2api/%'
  );

UPDATE usage_logs
SET request_type = 4
WHERE request_type = 6;

UPDATE ops_error_logs
SET request_type = 8
WHERE request_type = 4
  AND (
      inbound_endpoint ILIKE '%/images/%'
      OR upstream_endpoint ILIKE '%/images/%'
      OR inbound_endpoint ILIKE '%/images2api/%'
      OR upstream_endpoint ILIKE '%/images2api/%'
  );

UPDATE ops_error_logs
SET request_type = 4
WHERE request_type = 6;

COMMENT ON COLUMN ops_error_logs.request_type IS 'Request type enum: 0=unknown, 1=sync, 2=stream, 3=ws_v2, 4=cyber, 5=image_web_bridge, 6=legacy_cyber_alias, 7=video, 8=image. Matches usage_logs.request_type semantics.';
