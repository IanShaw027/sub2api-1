-- Add per-group Requests-Per-Minute limit.
-- rpm_limit: 分组统一 RPM 上限（0 = 不限制）。
-- 与用户级 users.rpm_limit 同时生效；可被 user-group rpm_override 覆盖。
-- 计数键：rpm:ug:{user_id}:{group_id}:{minute}。
ALTER TABLE groups ADD COLUMN IF NOT EXISTS rpm_limit integer NOT NULL DEFAULT 0;

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'groups_rpm_limit_non_negative'
	) THEN
		ALTER TABLE groups
			ADD CONSTRAINT groups_rpm_limit_non_negative
			CHECK (rpm_limit >= 0);
	END IF;
END $$;

COMMENT ON COLUMN groups.rpm_limit IS '分组 RPM 上限；0 表示不限制；与用户级 rpm_limit 并行生效，可被 user-group rpm_override 覆盖。';
