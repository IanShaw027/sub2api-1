-- 移除所有运维监控相关的 Webhook 通知渠道字段
-- 原因：根据最新需求，仅支持邮件通知，不再支持 Webhook。

-- 1. 处理告警规则表
ALTER TABLE ops_alert_rules DROP COLUMN IF EXISTS notify_webhook;
ALTER TABLE ops_alert_rules DROP COLUMN IF EXISTS webhook_url;

-- 2. 处理告警事件记录表
ALTER TABLE ops_alert_events DROP COLUMN IF EXISTS webhook_sent;

-- 3. 处理分组可用性配置表
ALTER TABLE ops_group_availability_configs DROP COLUMN IF EXISTS notify_webhook;
ALTER TABLE ops_group_availability_configs DROP COLUMN IF EXISTS webhook_url;

-- 4. 处理分组可用性事件记录表
ALTER TABLE ops_group_availability_events DROP COLUMN IF EXISTS webhook_sent;
