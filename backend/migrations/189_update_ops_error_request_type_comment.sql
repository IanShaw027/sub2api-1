-- 189_update_ops_error_request_type_comment.sql
-- Keep ops_error_logs.request_type documentation aligned with service.RequestType.

COMMENT ON COLUMN ops_error_logs.request_type IS 'Request type enum: 0=unknown, 1=sync, 2=stream, 3=ws_v2, 4=image, 5=image_web_bridge, 6=cyber, 7=video. Matches usage_logs.request_type semantics.';
