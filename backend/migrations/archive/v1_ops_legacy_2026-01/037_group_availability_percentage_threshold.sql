-- 037_group_availability_percentage_threshold.sql
-- 为分组可用性监控增加“可用账号占比阈值”和阈值模式（count/percentage/both）

-- 配置表：新增阈值模式与百分比阈值
ALTER TABLE ops_group_availability_configs
  ADD COLUMN IF NOT EXISTS threshold_mode VARCHAR(20) NOT NULL DEFAULT 'count',
  ADD COLUMN IF NOT EXISTS min_available_percentage DOUBLE PRECISION NOT NULL DEFAULT 0;

-- 约束：threshold_mode 合法值
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'chk_ops_group_availability_configs_threshold_mode'
  ) THEN
    ALTER TABLE ops_group_availability_configs
      ADD CONSTRAINT chk_ops_group_availability_configs_threshold_mode
      CHECK (threshold_mode IN ('count', 'percentage', 'both'));
  END IF;
END $$;

-- 约束：百分比范围 0-100（0 表示不启用该阈值）
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'chk_ops_group_availability_configs_min_available_percentage'
  ) THEN
    ALTER TABLE ops_group_availability_configs
      ADD CONSTRAINT chk_ops_group_availability_configs_min_available_percentage
      CHECK (min_available_percentage >= 0 AND min_available_percentage <= 100);
  END IF;
END $$;

COMMENT ON COLUMN ops_group_availability_configs.threshold_mode IS '阈值模式: count/percentage/both';
COMMENT ON COLUMN ops_group_availability_configs.min_available_percentage IS '最低可用账号占比阈值(0-100)，0 表示未启用该阈值';
