-- Add per-user Requests-Per-Minute cap.
-- rpm_limit: 用户全局 RPM 上限（0 = 不限制）。
-- 用户级上限始终生效，与分组/override 限制并行叠加。
-- 计数键：rpm:u:{user_id}:{minute}。
ALTER TABLE users ADD COLUMN IF NOT EXISTS rpm_limit integer NOT NULL DEFAULT 0;

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'users_rpm_limit_non_negative'
	) THEN
		ALTER TABLE users
			ADD CONSTRAINT users_rpm_limit_non_negative
			CHECK (rpm_limit >= 0);
	END IF;
END $$;

COMMENT ON COLUMN users.rpm_limit IS '用户级 RPM 全局上限；0 表示不限制；与分组/override 限制并行生效。';
