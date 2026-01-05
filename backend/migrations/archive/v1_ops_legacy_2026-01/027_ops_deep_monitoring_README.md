# 运维深度监控系统 - 数据库迁移说明

## 迁移文件

- **UP**: `027_ops_deep_monitoring.sql`
- **DOWN**: `027_ops_deep_monitoring_down.sql`
- **创建时间**: 2026-01-03
- **参考文档**: `docs/ops-monitoring/deep-planning.md`

## 变更概述

本次迁移实现了运维深度监控系统的数据模型扩展,支持四层监控架构(L1服务器/L2 API/L3上游/L4客户)。

### 1. 扩展 `ops_error_logs` 表

新增字段:
- `time_to_first_token_ms` (BIGINT): 首Token延迟,流式响应场景核心指标
- `auth_latency_ms` (BIGINT): 认证延迟
- `routing_latency_ms` (BIGINT): 路由决策延迟
- `upstream_latency_ms` (BIGINT): 上游请求延迟
- `response_latency_ms` (BIGINT): 响应处理延迟
- `request_body` (JSONB): 请求体(脱敏后),限制10KB
- `user_agent` (TEXT): 用户代理字符串

新增索引:
- `idx_ops_error_logs_ttft`: 优化TTFT查询
- `idx_ops_error_logs_upstream_latency`: 优化延迟查询

### 2. 扩展 `usage_logs` 表

新增字段(与ops_error_logs对齐):
- `time_to_first_token_ms` (BIGINT)
- `auth_latency_ms` (BIGINT)
- `routing_latency_ms` (BIGINT)
- `upstream_latency_ms` (BIGINT)
- `response_latency_ms` (BIGINT)
- `provider` (VARCHAR(50)): 上游供应商(从accounts.platform回填)

新增索引:
- `idx_usage_logs_ttft`: 优化TTFT查询
- `idx_usage_logs_provider`: 优化按供应商查询
- `idx_usage_logs_provider_created`: 优化时间范围查询

### 3. 新增 `ops_upstream_stats` 表

**用途**: 上游三级统计(platform → auth_type → account_id)

**核心字段**:
- 三级维度: `platform`, `auth_type`, `account_id`
- 统计周期: `stat_time`, `period_minutes` (5/15/60/1440分钟)
- 请求统计: `total_requests`, `success_count`, `error_count`, `success_rate`
- 延迟统计: `latency_avg`, `latency_p50`, `latency_p95`, `latency_p99`, `ttft_median`
- 错误分布: `error_401_count`, `error_403_count`, `error_429_count`, `error_500_count`, `error_502_count`, `error_504_count`, `error_timeout_count`
- 账号状态: `account_status`, `consecutive_errors`, `last_error_time`, `last_error_message`

**索引策略**:
- 唯一索引: `(platform, auth_type, account_id, stat_time, period_minutes)` 确保数据唯一性
- 查询索引: 按platform、account_id、时间、状态优化查询

### 4. 新增 `ops_retry_logs` 表

**用途**: 记录请求重试日志,用于验证问题是否已解决

**核心字段**:
- `retry_id` (VARCHAR(100)): 重试唯一标识
- `original_error_id` (BIGINT): 关联原始错误
- `use_original_params` (BOOLEAN): 是否使用原始请求参数
- `override_account_id` (BIGINT): 可选,指定使用特定账号重试
- `status` (VARCHAR(50)): success/failed
- `http_code`, `latency_ms`, `time_to_first_token_ms`: 重试结果指标
- `initiated_by` (VARCHAR(100)): 操作人员,用于审计

### 5. 新增辅助视图

#### `ops_upstream_health_dashboard`
- 上游账号健康仪表盘
- 展示最近1小时各账号的健康状况
- 包含错误分布、延迟指标、TTFT中位数

#### `ops_error_detail_view`
- 错误详情下钻视图
- 整合L1~L4层监控信息
- 用于快速定位问题根因

### 6. 新增辅助函数

#### `calculate_latency_breakdown()`
- 计算延迟各阶段占比
- 返回JSONB格式,包含各阶段耗时和占比
- 用于生成延迟瀑布图数据

## 执行迁移

### 方式1: 使用项目迁移工具

```bash
cd backend
# 执行迁移
make migrate-up
# 或
go run cmd/migrate/main.go up
```

### 方式2: 手动执行SQL

```bash
# 执行UP迁移
psql -U your_user -d your_database -f backend/migrations/027_ops_deep_monitoring.sql

# 回滚(如需要)
psql -U your_user -d your_database -f backend/migrations/027_ops_deep_monitoring_down.sql
```

### 方式3: Docker环境

```bash
docker exec -i postgres_container psql -U postgres -d sub2api < backend/migrations/027_ops_deep_monitoring.sql
```

## 验证迁移

执行迁移后,检查以下内容:

### 1. 检查表结构

```sql
-- 检查ops_error_logs新增字段
SELECT column_name, data_type, is_nullable
FROM information_schema.columns
WHERE table_name = 'ops_error_logs'
  AND column_name IN (
    'time_to_first_token_ms',
    'auth_latency_ms',
    'routing_latency_ms',
    'upstream_latency_ms',
    'response_latency_ms',
    'request_body',
    'user_agent'
  );

-- 检查usage_logs新增字段
SELECT column_name, data_type, is_nullable
FROM information_schema.columns
WHERE table_name = 'usage_logs'
  AND column_name IN (
    'time_to_first_token_ms',
    'auth_latency_ms',
    'routing_latency_ms',
    'upstream_latency_ms',
    'response_latency_ms',
    'provider'
  );
```

### 2. 检查新表

```sql
-- 检查ops_upstream_stats表
SELECT COUNT(*) FROM ops_upstream_stats;

-- 检查ops_retry_logs表
SELECT COUNT(*) FROM ops_retry_logs;
```

### 3. 检查视图

```sql
-- 检查ops_upstream_health_dashboard视图
SELECT * FROM ops_upstream_health_dashboard LIMIT 1;

-- 检查ops_error_detail_view视图
SELECT * FROM ops_error_detail_view LIMIT 1;
```

### 4. 检查索引

```sql
-- 检查ops_error_logs索引
SELECT indexname, indexdef
FROM pg_indexes
WHERE tablename = 'ops_error_logs'
  AND indexname IN ('idx_ops_error_logs_ttft', 'idx_ops_error_logs_upstream_latency');

-- 检查usage_logs索引
SELECT indexname, indexdef
FROM pg_indexes
WHERE tablename = 'usage_logs'
  AND indexname IN ('idx_usage_logs_ttft', 'idx_usage_logs_provider', 'idx_usage_logs_provider_created');

-- 检查ops_upstream_stats索引
SELECT indexname, indexdef
FROM pg_indexes
WHERE tablename = 'ops_upstream_stats';
```

### 5. 检查辅助函数

```sql
-- 测试calculate_latency_breakdown函数
SELECT calculate_latency_breakdown(50, 100, 14800, 234);
-- 预期返回: {"auth_ms": 50, "routing_ms": 100, "upstream_ms": 14800, "response_ms": 234, "total_ms": 15184, "auth_pct": 0.33, "routing_pct": 0.66, "upstream_pct": 97.47, "response_pct": 1.54}
```

### 6. 检查数据保留配置

```sql
SELECT * FROM ops_data_retention_config
WHERE table_name IN ('ops_upstream_stats', 'ops_retry_logs');
```

## 数据迁移注意事项

### 1. provider字段回填

迁移脚本会自动回填`usage_logs.provider`字段:
```sql
UPDATE usage_logs ul
SET provider = a.platform
FROM accounts a
WHERE ul.account_id = a.id AND ul.provider IS NULL;
```

检查回填结果:
```sql
SELECT provider, COUNT(*) as count
FROM usage_logs
GROUP BY provider;
```

### 2. 性能影响

- **ops_error_logs**: 新增7个字段,影响较小(错误日志量相对较少)
- **usage_logs**: 新增6个字段,表记录较多时需关注性能
- **新增索引**: 可能需要一些时间构建,建议在业务低峰期执行

### 3. 存储空间

新增表的预估空间占用:
- `ops_upstream_stats`: ~10MB/天(5分钟粒度,假设50个账号)
- `ops_retry_logs`: 极小(仅记录手动重试操作)

## 回滚说明

如果迁移后出现问题,可执行回滚:

```bash
psql -U your_user -d your_database -f backend/migrations/027_ops_deep_monitoring_down.sql
```

**⚠️ 警告**: 回滚将删除以下数据:
- `ops_upstream_stats` 表的所有统计数据
- `ops_retry_logs` 表的所有重试日志
- `ops_error_logs` 和 `usage_logs` 新增字段的数据

建议在回滚前备份数据:
```bash
pg_dump -U your_user -d your_database -t ops_upstream_stats > backup_upstream_stats.sql
pg_dump -U your_user -d your_database -t ops_retry_logs > backup_retry_logs.sql
```

## 下一步操作

完成数据库迁移后,需要:

1. **更新后端代码**:
   - 修改ORM模型(ent/gorm schema)
   - 实现延迟追踪中间件
   - 实现请求体脱敏函数
   - 实现上游统计聚合任务
   - 实现请求重试API

2. **更新前端代码**:
   - 实现L3上游监控页面
   - 实现延迟瀑布图组件
   - 实现错误详情弹窗优化
   - 实现一键重试功能

3. **配置定时任务**:
   - 上游统计聚合(每5分钟)
   - 数据降采样(每天)
   - 数据清理(根据保留策略)

## 相关文档

- [运维深度监控规划文档](../../docs/ops-monitoring/deep-planning.md)
- [迁移脚本源文件](./027_ops_deep_monitoring.sql)
- [回滚脚本源文件](./027_ops_deep_monitoring_down.sql)

## 联系方式

如有问题,请联系:
- Backend Team
- Ops Team

---

**创建时间**: 2026-01-03
**维护者**: Backend Team
