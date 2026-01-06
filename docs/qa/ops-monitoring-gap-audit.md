# 运维监控模块：待实现/对接缺口清单（全局审查）

目标：把「后端已有但前端没接」「前端有元素/类型但没行为」「前后端字段/语义不一致」一次性梳理出来，便于后续按优先级逐项补齐或删减（避免过度设计）。

> 说明：本项目当前要求 **Webhook 完全不支持**，因此本清单不包含任何 webhook 落地项。

---

## 1. 现状：后端 Ops 能力/接口总览（/api/v1/admin/ops）

### 1.1 已被前端实际使用（对接完整）

- `GET /admin/ops/dashboard/overview`：Ops 首页概览（`frontend/src/views/admin/ops/OpsDashboard.vue` 使用）
- `GET /admin/ops/dashboard/providers`：Provider 健康对比（`frontend/src/views/admin/ops/OpsDashboard.vue` 使用）
- `GET /admin/ops/dashboard/latency-histogram`：延迟直方图（`frontend/src/views/admin/ops/OpsDashboard.vue` 使用）
- `GET /admin/ops/dashboard/errors/distribution`：错误分布（`frontend/src/views/admin/ops/OpsDashboard.vue` 使用）
- `GET /admin/ops/metrics`：最新系统指标（`frontend/src/views/admin/ops/OpsDashboard.vue` 使用）
- `GET /admin/ops/metrics/history`：系统指标历史（`frontend/src/views/admin/ops/OpsDashboard.vue` 使用）
- `GET /admin/ops/errors`：分页错误日志（`frontend/src/views/admin/ops/OpsDashboard.vue` 使用）
- `GET /admin/ops/errors/:id`：错误详情（`frontend/src/components/admin/ErrorDetailModal.vue` 使用）
- `POST /admin/ops/errors/:id/retry`：重试请求（`frontend/src/views/admin/ops/components/OpsErrorLogTable.vue` 使用）
- `GET /admin/ops/requests`：请求明细（`frontend/src/views/admin/ops/components/OpsRequestDetailsModal.vue` 使用）
- `GET /admin/ops/ws/qps`：QPS/TPS WebSocket（`frontend/src/api/admin/ops.ts` + `OpsDashboard.vue` 使用）
- `GET /admin/ops/alert-rules` / `POST` / `PUT` / `DELETE`：告警规则管理（`frontend/src/views/admin/SettingsView.vue` + `frontend/src/views/admin/ops/components/OpsAlertRulesCard.vue` 使用）
- `GET /admin/ops/alert-events`：告警事件历史（`frontend/src/views/admin/ops/components/OpsAlertEventsCard.vue` 使用）
- `GET/PUT /admin/ops/email-notification/config`：邮件通知配置（`frontend/src/views/admin/SettingsView.vue` + `frontend/src/views/admin/ops/components/OpsEmailNotificationCard.vue` 使用）
- `GET/PUT /admin/ops/runtime/alert`：告警运行时设置（含静默）`frontend/src/views/admin/SettingsView.vue` + `frontend/src/views/admin/ops/components/OpsRuntimeSettingsCard.vue` 使用
- `GET/PUT /admin/ops/runtime/group-availability`：分组可用性运行时设置 `frontend/src/views/admin/SettingsView.vue` + `frontend/src/views/admin/ops/components/OpsRuntimeSettingsCard.vue` 使用
- `GET /admin/ops/group-availability/status`：分组可用性状态（`frontend/src/views/admin/ops/components/OpsGroupAvailabilityCard.vue` 使用）
- `PUT /admin/ops/group-availability/configs/:groupId`：分组可用性配置写入（`OpsGroupAvailabilityCard.vue` / `OpsConfigDialog.vue` 使用）
- `GET /admin/ops/group-availability/events`：分组可用性事件历史（`frontend/src/views/admin/ops/components/OpsGroupAvailabilityEventsCard.vue` 使用）
- `GET /admin/ops/error-stats`：错误统计聚合（`frontend/src/views/admin/ops/components/OpsErrorAnalyticsCard.vue` 使用）
- `GET /admin/ops/error-timeseries`：错误时序曲线（`frontend/src/views/admin/ops/components/OpsErrorAnalyticsCard.vue` 使用）
- `GET /admin/ops/errors/by-ip`、`GET /admin/ops/errors/by-ip/:ip`：按 IP 聚合与明细（`frontend/src/views/admin/ops/components/OpsErrorByIPCard.vue` 使用）
- `GET /admin/ops/account-status`：账号状态概览（`frontend/src/views/admin/ops/components/OpsAccountStatusCard.vue` 使用）

### 1.2 后端已有，但前端未接入（功能缺口/可优化方向）

- `GET /admin/ops/error-logs`：旧的「limit 列表」接口
  - 现状：已下线（仓库不再注册该路由）；统一使用分页 `GET /admin/ops/errors`

---

## 2. 现状：前端 Ops 页面/组件/交互总览

入口：`frontend/src/router/index.ts` 路由 `/admin/ops` 指向 `frontend/src/views/admin/ops/OpsDashboard.vue`。

### 2.1 前端已有且行为闭环

- 指标看板 + 图表：`frontend/src/views/admin/ops/components/OpsMetricsCharts.vue`
- 错误日志列表/筛选/分页/重试：`frontend/src/views/admin/ops/components/OpsErrorLogTable.vue`
- 请求明细弹窗（图表/卡片 drill-down）：`frontend/src/views/admin/ops/components/OpsRequestDetailsModal.vue`
- 分组可用性卡片：`frontend/src/views/admin/ops/components/OpsGroupAvailabilityCard.vue`
- 邮件通知配置：已迁移到 `frontend/src/views/admin/SettingsView.vue`（使用 `OpsEmailNotificationCard.vue`）
- 运行时设置（含全局静默）：已迁移到 `frontend/src/views/admin/SettingsView.vue`（使用 `OpsRuntimeSettingsCard.vue`）

### 2.2 前端「有元素但无行为」（当前最明显的断点）

- Ops 顶部工具条的「Platform / Group」全局筛选器：
  - 组件：`frontend/src/views/admin/ops/components/OpsDashboardHeader.vue`
  - 现状：已接入（`frontend/src/views/admin/ops/OpsDashboard.vue` 监听 `@update:platform` / `@update:group`）
  - 注意：目前主要用于错误日志筛选（不会影响概览/图表类接口），如需“全局过滤”需进一步扩展后端 query 维度与前端透传。

### 2.3 前端存在“半成品/文档超前/类型错配”

- `frontend/src/views/admin/ops/components/OpsConfigDialog.vue`
  - 现状：已在 `frontend/src/views/admin/GroupsView.vue` 中作为「分组监控配置弹窗」被使用
  - 缺口：`frontend/src/views/admin/ops/components/README.md` 描述的 Tab1 告警规则、Tab3 邮件配置等内容 **并未在组件中实现**（README 明显过期）
  - 建议：更新 README 以匹配实际能力，避免重复入口（当前邮件通知/运行时设置已迁移到系统设置页）

- 告警规则 severity 类型错配问题
  - 已修复：Ops 告警规则 `severity` 使用 `P0..P3`，分组可用性使用 `critical/warning/info`，两者已在前端类型中分离（见 `frontend/src/views/admin/ops/types.ts`）。

---

## 3. 前后端对接不完整点（按优先级）

### P0（会误导或影响日常排障）

1) **Ops Header 的 platform/group 筛选器无效**
   - 已修复：筛选器事件已接入（见第 2.2）。注意目前主要用于错误日志筛选。

2) **告警规则 metric_type 兼容性问题：`http2_errors` 已废弃**
   - 已处理：种子不再写入 `http2_errors` 规则；同时保留迁移清理（`backend/migrations/029_ops_cleanup_deprecated_alert_rules.sql`）用于删除历史存量数据；服务端也不再暴露该指标类型。

### P1（后端能力存在但前端缺少入口/对接）

1) 告警规则 CRUD
   - 已实现：`frontend/src/views/admin/SettingsView.vue`（系统设置页入口，使用 `frontend/src/views/admin/ops/components/OpsAlertRulesCard.vue`）

2) 告警事件历史（后端已有，前端缺少 API + UI）
   - 后端：`GET /admin/ops/alert-events`
   - 已实现：`frontend/src/views/admin/ops/components/OpsAlertEventsCard.vue`

3) 分组可用性事件历史（后端已有，前端缺少 UI）
   - 后端：`GET /admin/ops/group-availability/events`
   - 已实现：`frontend/src/views/admin/ops/components/OpsGroupAvailabilityEventsCard.vue`

4) IP 维度的错误聚合（后端已有，前端缺少 UI）
   - 后端：`GET /admin/ops/errors/by-ip`、`/errors/by-ip/:ip`
   - 已实现：`frontend/src/views/admin/ops/components/OpsErrorByIPCard.vue` + `frontend/src/views/admin/ops/components/OpsErrorsByIPModal.vue`

5) 错误 stats / timeseries（后端已有，前端缺少图表）
   - 后端：`GET /admin/ops/error-stats`、`GET /admin/ops/error-timeseries`
   - 已实现：`frontend/src/views/admin/ops/components/OpsErrorAnalyticsCard.vue`

6) 账号状态概览（后端已有，前端缺少 UI）
   - 后端：`GET /admin/ops/account-status`
   - 已实现：`frontend/src/views/admin/ops/components/OpsAccountStatusCard.vue`

### P2（可用性/一致性/过度设计风险）

1) Alert silencing 的 entries（按 rule_id / severity 细粒度静默）后端已有，但前端只支持 global
   - 后端：`backend/internal/service/ops_alert_service.go` 的 `isSilenced()` 支持 `cfg.Entries`
   - 前端：`frontend/src/views/admin/ops/components/OpsRuntimeSettingsCard.vue` 只展示/编辑 global 字段（`entries` 仅做兜底初始化）
   - 建议：如果确实需要“维护窗口仅静默某一类规则”，再补 UI；否则可以将 entries 视作高级功能，明确隐藏/不支持。

2) 旧 cron 代码未被接入（可能是历史遗留/过度设计）
   - 已清理：移除 `backend/internal/cron/*` 与未生效的 cron schedule 配置项，避免与 `backend/internal/service/ops_aggregation_service.go` 的循环逻辑形成“双系统”。

---

## 4. 建议的“下一步补齐顺序”（避免重复建设）

1) 先处理 P0：修正/移除 Ops Header 的 platform/group“假控件”
2) 明确告警规则的严重级别体系（P0..P3 vs critical/warning/info）并修正前端类型
3) 决定是否需要“告警规则管理 UI”
   - 若需要：补 UI + 同时补 `alert-events` 浏览入口（否则很难追溯告警是否触发/是否发出邮件）
4) 决定是否需要 IP 维度聚合与错误时序图（更偏排障/安全运营）
