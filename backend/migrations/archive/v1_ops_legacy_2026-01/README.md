# 运维监控迁移脚本整合说明

## 整合时间
2026-01-05

## 背景
原 026-038 共 13 个运维监控相关的迁移文件过于碎片化，存在以下问题：
1. 多个表被创建后又被删除（如 026 创建 ops_account_status → 032 删除）
2. 迁移步骤过多，不利于从主分支升级
3. 缺乏整体规划，导致数据库结构反复修改

## 整合方案
将 13 个迁移文件整合为 3 个逻辑清晰的文件：

### 新迁移文件

#### 1. **026_ops_schema_v2_core.sql** - 核心表结构
整合来源：026, 027, 031, 032, 033, 036

**内容：**
- 扩展 `ops_error_logs` 表（错误分类 + 深度监控字段）
- 扩展 `usage_logs` 表（延迟分析字段）
- 扩展 `ops_alert_rules` 表（新功能字段）
- 简化 `ops_system_metrics` 表（删除过度监控字段）
- 删除废弃表（避免创建后再删除）

**优势：**
- 一次性完成所有核心表扩展
- 不创建任何会被后续删除的表
- 确保幂等性（IF NOT EXISTS / IF EXISTS）

---

#### 2. **027_ops_features_and_logic.sql** - 功能与逻辑层
整合来源：026, 027, 028, 035, 037

**内容：**
- 创建 `ops_scheduled_reports`（定时报告配置）
- 创建 `ops_metrics_hourly` / `ops_metrics_daily`（预聚合表）
- 创建 `ops_group_availability_configs` / `ops_group_availability_events`（分组可用性监控）
- 创建辅助函数（`calculate_latency_breakdown`）
- 创建视图（`ops_error_detail_view`, `ops_error_stats_by_source`）
- 插入默认数据（告警规则、定时报告）

**优势：**
- 新功能集中管理
- 直接包含百分比阈值支持（037）
- 避免分散在多个文件中

---

#### 3. **028_ops_performance_backfill.sql** - 性能优化与数据回填
整合来源：026, 027, 029, 030, 032, 034, 038

**内容：**
- 数据回填（`ops_error_logs.error_source`, `usage_logs.provider`）
- 创建 17 个 `ops_error_logs` 索引（时间序列、分类、账号、延迟、全文搜索等）
- 创建 4 个 `usage_logs` 索引（延迟、平台、账号）

**优势：**
- 索引创建集中管理
- 避免重复创建相同索引
- 数据回填与索引创建在同一文件中

---

## 归档文件清单

以下文件已归档到本目录：

| 文件名 | 说明 | 状态 |
|--------|------|------|
| 026_ops_error_classification.sql | 错误分类系统 | 已整合到 026, 027, 028 |
| 027_ops_deep_monitoring.sql | 深度监控 | 已整合到 026, 027, 028 |
| 027_ops_deep_monitoring_down.sql | 深度监控回滚 | 已归档 |
| 027_ops_deep_monitoring_README.md | 深度监控说明 | 已归档 |
| 028_ops_preaggregation_tables.sql | 预聚合表 | 已整合到 027 |
| 029_ops_performance_indexes.sql | 性能索引 | 已整合到 028 |
| 030_add_client_ip_index.sql | client_ip 索引 | 已整合到 028 |
| 031_remove_unused_tables.sql | 删除未使用表 | 已整合到 026 |
| 032_remove_ops_account_status.sql | 删除 ops_account_status | 已整合到 026 |
| 033_simplify_ops_metrics.sql | 简化指标 | 已整合到 026 |
| 034_critical_indexes.sql | 核心索引 | 已整合到 028 |
| 035_add_group_availability_monitoring.sql | 分组可用性监控 | 已整合到 027 |
| 036_remove_webhook_notification_channels.sql | 删除 webhook 字段 | 已整合到 026 |
| 037_group_availability_percentage_threshold.sql | 百分比阈值 | 已整合到 027 |
| 038_ops_error_log_filter_indexes.sql | 过滤索引 | 已整合到 028 |

---

## 升级路径

### 从主分支（025 之前）升级
直接运行新的 026, 027, 028 即可，所有功能一次性部署完成。

### 已运行部分旧迁移（026-038）
新迁移文件已确保幂等性：
- 所有 `ALTER TABLE` 使用 `ADD COLUMN IF NOT EXISTS`
- 所有 `CREATE TABLE/INDEX` 使用 `IF NOT EXISTS`
- 所有 `DROP TABLE` 使用 `IF EXISTS`

因此可以安全地重新运行新迁移文件。

---

## 对比优势

| 维度 | 旧方案（13个文件） | 新方案（3个文件） |
|------|-------------------|------------------|
| 文件数量 | 13 个 | 3 个 |
| 创建后删除的表 | 6 个 | 0 个 |
| 索引创建分散度 | 分散在 6 个文件 | 集中在 1 个文件 |
| 升级步骤 | 13 步 | 3 步 |
| 幂等性保证 | 部分 | 完全 |
| 向下兼容性 | 有风险 | 完全兼容 |
| 可维护性 | 低 | 高 |

---

## 验证清单

### 表结构验证
- [ ] `ops_error_logs` 包含 15 个新字段（8个分类 + 7个监控）
- [ ] `usage_logs` 包含 6 个新字段（5个延迟 + 1个provider）
- [ ] `ops_alert_rules` 包含 6 个新字段，无 webhook 字段
- [ ] `ops_system_metrics` 精简为 30 个核心字段
- [ ] `ops_scheduled_reports` 表已创建
- [ ] `ops_metrics_hourly` 和 `ops_metrics_daily` 表已创建
- [ ] `ops_group_availability_configs` 和 `ops_group_availability_events` 表已创建

### 索引验证
- [ ] `ops_error_logs` 有 17 个索引
- [ ] `usage_logs` 有 4 个索引
- [ ] 包含 GIN 全文索引

### 数据验证
- [ ] `ops_error_logs.error_source` 已回填
- [ ] `usage_logs.provider` 已回填
- [ ] 默认告警规则已插入（3条）
- [ ] 默认定时报告已插入（2条）

### 废弃表验证
- [ ] `ops_account_status` 不存在
- [ ] `ops_upstream_stats` 不存在
- [ ] `ops_retry_logs` 不存在
- [ ] `ops_alert_notifications` 不存在
- [ ] `ops_dimension_stats` 不存在
- [ ] `ops_data_retention_config` 不存在

---

## 回滚说明

如需回滚到旧的迁移方案：
1. 删除新的 026, 027, 028 文件
2. 将归档目录中的文件恢复到 `backend/migrations/`
3. 注意：已运行的迁移可能需要手动回滚数据库状态

---

## 联系人
如有疑问，请联系运维团队。
