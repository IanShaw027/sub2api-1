-- 035_add_group_availability_monitoring.sql
-- 添加分组可用性监控功能

-- 分组可用性监控配置表
CREATE TABLE IF NOT EXISTS ops_group_availability_configs (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,

    -- 监控配置
    enabled BOOLEAN NOT NULL DEFAULT false,
    min_available_accounts INT NOT NULL DEFAULT 1,

    -- 告警配置
    notify_email BOOLEAN NOT NULL DEFAULT true,
    notify_webhook BOOLEAN NOT NULL DEFAULT false,
    webhook_url TEXT,

    -- 告警级别 (critical/warning/info)
    severity VARCHAR(20) NOT NULL DEFAULT 'warning',

    -- 冷却期（分钟）- 避免重复告警
    cooldown_minutes INT NOT NULL DEFAULT 30,

    -- 时间戳
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- 唯一约束：每个分组只能有一个配置
    UNIQUE(group_id)
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_ops_group_availability_configs_group_id ON ops_group_availability_configs(group_id);
CREATE INDEX IF NOT EXISTS idx_ops_group_availability_configs_enabled ON ops_group_availability_configs(enabled);

-- 分组可用性告警事件表
CREATE TABLE IF NOT EXISTS ops_group_availability_events (
    id BIGSERIAL PRIMARY KEY,
    config_id BIGINT NOT NULL REFERENCES ops_group_availability_configs(id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,

    -- 告警状态 (firing/resolved)
    status VARCHAR(20) NOT NULL DEFAULT 'firing',
    severity VARCHAR(20) NOT NULL,

    -- 告警详情
    title TEXT NOT NULL,
    description TEXT NOT NULL,

    -- 指标值
    available_accounts INT NOT NULL,
    threshold_accounts INT NOT NULL,
    total_accounts INT NOT NULL,

    -- 通知状态
    email_sent BOOLEAN NOT NULL DEFAULT false,
    webhook_sent BOOLEAN NOT NULL DEFAULT false,

    -- 时间戳
    fired_at TIMESTAMP NOT NULL,
    resolved_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chk_ops_group_availability_events_status CHECK (status IN ('firing', 'resolved'))
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_ops_group_availability_events_config_id ON ops_group_availability_events(config_id);
CREATE INDEX IF NOT EXISTS idx_ops_group_availability_events_group_id ON ops_group_availability_events(group_id);
CREATE INDEX IF NOT EXISTS idx_ops_group_availability_events_status ON ops_group_availability_events(status);
CREATE INDEX IF NOT EXISTS idx_ops_group_availability_events_fired_at ON ops_group_availability_events(fired_at DESC);

-- 注释
COMMENT ON TABLE ops_group_availability_configs IS '分组可用性监控配置';
COMMENT ON TABLE ops_group_availability_events IS '分组可用性告警事件';

COMMENT ON COLUMN ops_group_availability_configs.enabled IS '是否启用监控';
COMMENT ON COLUMN ops_group_availability_configs.min_available_accounts IS '最低可用账号数阈值';
COMMENT ON COLUMN ops_group_availability_configs.severity IS '告警级别: critical/warning/info';
COMMENT ON COLUMN ops_group_availability_configs.cooldown_minutes IS '冷却期（分钟），避免重复告警';

COMMENT ON COLUMN ops_group_availability_events.status IS '告警状态: firing/resolved';
COMMENT ON COLUMN ops_group_availability_events.available_accounts IS '当前可用账号数';
COMMENT ON COLUMN ops_group_availability_events.threshold_accounts IS '阈值账号数';
COMMENT ON COLUMN ops_group_availability_events.total_accounts IS '总账号数';
