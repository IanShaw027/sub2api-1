# Ops-Monitoring 模块测试指南

## 分支信息

- **分支名称**: `feature/ops-monitoring`
- **基于**: `main` 分支
- **提交**: 已完成，包含所有 ops 功能

## 模块统计

- **文件数量**: 166 个文件变更
- **代码行数**: 31,033 行新增代码
- **数据库表**: 9 张表
- **后端文件**: 81 个（Service: 34, Repository: 19, Handler: 11, 其他: 17）
- **前端组件**: 17 个 Vue 组件

## 快速开始

### 1. 切换到测试分支

```bash
git checkout feature/ops-monitoring
```

### 2. 数据库迁移

ops 模块的数据库 schema 已包含在迁移文件中：

```bash
# 查看迁移文件
ls -lh backend/migrations/*ops*

# 输出：
# 030_ops_drop_legacy_tables.sql  (1.2K)
# 031_ops_schema.sql              (30K, 834行)
```

**自动迁移**（推荐）：
```bash
# 启动应用时会自动执行迁移
cd backend
go run cmd/server/main.go
```

**手动迁移**（可选）：
```bash
# 如果需要手动执行
psql -U postgres -d sub2api -f backend/migrations/030_ops_drop_legacy_tables.sql
psql -U postgres -d sub2api -f backend/migrations/031_ops_schema.sql
```

### 3. 配置 ops 模块

在 `backend/config.yaml` 或环境变量中配置：

```yaml
ops:
  enabled: true  # 启用 ops 模块
  use_preaggregated_tables: true  # 使用预聚合表

  cleanup:
    enabled: true
    schedule: "0 2 * * *"  # 每天凌晨2点清理
    error_log_retention_days: 7
    minute_metrics_retention_days: 3
    hourly_metrics_retention_days: 30

  metrics_collector_cache:
    enabled: true
    ttl_seconds: 60

  aggregation:
    enabled: true
    hourly_schedule: "0 * * * *"  # 每小时执行
    daily_schedule: "0 1 * * *"   # 每天凌晨1点执行
```

### 4. 启动服务

```bash
# 后端
cd backend
go run cmd/server/main.go

# 前端（新终端）
cd frontend
pnpm install
pnpm run dev
```

### 5. 访问 Ops 仪表板

打开浏览器访问：
```
http://localhost:5173/admin/ops
```

## 功能测试清单

### 核心功能

- [ ] **错误日志记录**
  - 触发一个错误请求（如无效 API Key）
  - 检查 ops_error_logs 表是否有记录
  - 验证敏感信息是否已脱敏

- [ ] **错误日志查询**
  - 访问 Ops 仪表板
  - 查看错误日志列表
  - 测试过滤功能（platform, severity, search）
  - 测试分页功能
  - 点击错误查看详情

- [ ] **性能指标采集**
  - 等待 1 分钟（指标收集器每分钟执行一次）
  - 检查 ops_system_metrics 表是否有记录
  - 查看仪表板上的 QPS/TPS/延迟/错误率

- [ ] **仪表板展示**
  - 查看核心指标卡片
  - 查看指标图表（Chart.js）
  - 测试时间范围选择（5m/30m/1h/6h/24h）
  - 测试刷新按钮

### 高级功能

- [ ] **WebSocket 实时推送**
  - 打开仪表板
  - 观察 QPS/TPS 是否实时更新（2秒间隔）
  - 检查浏览器开发者工具的 WebSocket 连接

- [ ] **告警系统**
  - 创建告警规则（如错误率 > 10%）
  - 触发告警条件
  - 检查 ops_alert_events 表
  - 验证邮件通知（如果配置了邮件）

- [ ] **分组可用性监控**
  - 配置分组可用性规则
  - 查看分组可用性状态
  - 测试可用性事件记录

- [ ] **预聚合表**
  - 等待 1 小时（小时级聚合）
  - 检查 ops_metrics_hourly 表
  - 查看历史数据查询性能

- [ ] **数据清理**
  - 等待清理任务执行（默认每天凌晨2点）
  - 或手动触发清理
  - 验证过期数据是否被删除

## 数据库表验证

```sql
-- 检查所有 ops 表是否创建成功
SELECT tablename
FROM pg_tables
WHERE tablename LIKE 'ops_%'
ORDER BY tablename;

-- 应该看到 9 张表：
-- ops_alert_events
-- ops_alert_rules
-- ops_error_logs
-- ops_group_availability_configs
-- ops_group_availability_events
-- ops_metrics_daily
-- ops_metrics_hourly
-- ops_scheduled_reports
-- ops_system_metrics

-- 检查错误日志表
SELECT COUNT(*) FROM ops_error_logs;

-- 检查系统指标表
SELECT COUNT(*) FROM ops_system_metrics;

-- 查看最近的错误日志
SELECT
    id,
    error_phase,
    error_type,
    severity,
    status_code,
    platform,
    created_at
FROM ops_error_logs
ORDER BY created_at DESC
LIMIT 10;

-- 查看最近的系统指标
SELECT
    id,
    request_count,
    success_count,
    error_count,
    qps,
    latency_p99,
    success_rate,
    created_at
FROM ops_system_metrics
ORDER BY created_at DESC
LIMIT 10;
```

## 性能测试

### 1. 错误日志写入性能

```bash
# 使用 ab 或 wrk 进行压力测试
ab -n 1000 -c 10 -H "Authorization: Bearer invalid_token" http://localhost:8080/api/v1/admin/ops/dashboard/overview

# 检查错误日志记录成功率
SELECT COUNT(*) FROM ops_error_logs WHERE created_at > NOW() - INTERVAL '1 minute';
```

### 2. 仪表板查询性能

```bash
# 测试仪表板查询延迟
time curl -H "Authorization: Bearer YOUR_ADMIN_TOKEN" http://localhost:8080/api/v1/admin/ops/dashboard/overview?timeRange=24h

# 应该在 500ms 以内
```

### 3. 预聚合表查询性能

```sql
-- 测试预聚合表查询
EXPLAIN ANALYZE
SELECT
    bucket_start,
    platform,
    request_count,
    error_count,
    avg_latency_ms
FROM ops_metrics_hourly
WHERE bucket_start >= NOW() - INTERVAL '24 hours'
ORDER BY bucket_start DESC;
```

## 常见问题

### Q1: 数据库迁移失败

**问题**: 执行迁移时报错 "relation already exists"

**解决**:
```sql
-- 检查表是否已存在
SELECT tablename FROM pg_tables WHERE tablename LIKE 'ops_%';

-- 如果表已存在但结构不对，可以删除重建
DROP TABLE IF EXISTS ops_error_logs CASCADE;
DROP TABLE IF EXISTS ops_system_metrics CASCADE;
-- ... 删除其他表

-- 然后重新执行迁移
```

### Q2: Ops 模块未启用

**问题**: 访问 /admin/ops 返回 404

**解决**:
1. 检查配置文件 `ops.enabled: true`
2. 检查环境变量 `OPS_ENABLED=true`
3. 重启服务

### Q3: 指标不更新

**问题**: 仪表板上的指标不更新

**解决**:
1. 检查后台服务是否启动（查看日志）
2. 检查 ops_system_metrics 表是否有新记录
3. 检查 Redis 连接是否正常
4. 检查分布式锁是否正常工作

### Q4: WebSocket 连接失败

**问题**: 实时推送不工作

**解决**:
1. 检查浏览器控制台的 WebSocket 错误
2. 检查 CORS 配置
3. 检查 WebSocket Origin 策略
4. 尝试禁用浏览器扩展

### Q5: 邮件通知不发送

**问题**: 告警邮件未收到

**解决**:
1. 检查邮件配置（SMTP 服务器、端口、认证）
2. 检查邮件限流设置（默认 10封/小时）
3. 检查 ops_alert_events 表的 email_sent 字段
4. 查看服务日志中的邮件发送错误

## 回滚方案

如果测试发现问题需要回滚：

```bash
# 1. 切换回 main 分支
git checkout main

# 2. 删除 ops 相关表（可选）
psql -U postgres -d sub2api << EOF
DROP TABLE IF EXISTS ops_error_logs CASCADE;
DROP TABLE IF EXISTS ops_system_metrics CASCADE;
DROP TABLE IF EXISTS ops_alert_rules CASCADE;
DROP TABLE IF EXISTS ops_alert_events CASCADE;
DROP TABLE IF EXISTS ops_scheduled_reports CASCADE;
DROP TABLE IF EXISTS ops_metrics_hourly CASCADE;
DROP TABLE IF EXISTS ops_metrics_daily CASCADE;
DROP TABLE IF EXISTS ops_group_availability_configs CASCADE;
DROP TABLE IF EXISTS ops_group_availability_events CASCADE;
EOF

# 3. 重启服务
```

## 下一步

测试完成后，如果功能正常，可以：

1. 合并到 main 分支
2. 或者基于此分支进行精简重构
3. 或者提出具体的改进建议

## 联系方式

如有问题，请查看：
- 代码审查报告：`docs/ops-monitoring/03-code-review-findings.md`
- 需求文档：`docs/ops-monitoring/02-requirements.md`
- 重构计划：`docs/ops-monitoring/04-refactoring-plan.md`
