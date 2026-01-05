-- Cleanup deprecated ops alert rules
--
-- Note: http2_errors metric is no longer produced (see service.metricValue), so any existing
-- rules using it will never trigger and may confuse operators.
--
-- This migration is idempotent.

DELETE FROM ops_alert_rules
WHERE metric_type = 'http2_errors';

