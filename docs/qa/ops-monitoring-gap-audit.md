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
- `GET/PUT /admin/ops/email-notification/config`：邮件通知配置（`frontend/src/views/admin/ops/components/OpsEmailNotificationCard.vue` 使用）
- `GET/PUT /admin/ops/runtime/alert`：告警运行时设置（含静默）`frontend/src/views/admin/ops/components/OpsRuntimeSettingsCard.vue` 使用
- `GET/PUT /admin/ops/runtime/group-availability`：分组可用性运行时设置 `frontend/src/views/admin/ops/components/OpsRuntimeSettingsCard.vue` 使用
- `GET /admin/ops/group-availability/status`：分组可用性状态（`frontend/src/views/admin/ops/components/OpsGroupAvailabilityCard.vue` 使用）
- `PUT /admin/ops/group-availability/configs/:groupId`：分组可用性配置写入（`OpsGroupAvailabilityCard.vue` / `OpsConfigDialog.vue` 使用）

### 1.2 后端已有，但前端未接入（功能缺口/可优化方向）

- `GET /admin/ops/alert-rules` / `POST` / `PUT` / `DELETE`：告警规则 CRUD
  - 前端：`frontend/src/api/admin/ops.ts` 已有 API 封装，但 Ops 页面没有入口/界面承载（详见第 2 节）
- `GET /admin/ops/alert-events`：告警事件历史
  - 前端：没有对应 API 封装、没有 UI
- `GET /admin/ops/group-availability/events`：分组可用性告警事件历史
  - 前端：没有对应 UI（只展示实时 status，不展示历史事件）
- `GET /admin/ops/error-stats`：错误统计聚合（支持 group_by=platform/phase/severity）
  - 前端：没有对应 UI/图表
- `GET /admin/ops/error-timeseries`：错误时序曲线
  - 前端：没有对应 UI/图表
- `GET /admin/ops/errors/by-ip`、`GET /admin/ops/errors/by-ip/:ip`：按 IP 聚合与明细
  - 前端：没有对应 UI（对恶意流量定位很有价值）
- `GET /admin/ops/account-status`：账号状态概览
  - 前端：没有对应 UI（目前 Ops 页面也没有账号健康看板）
- `GET /admin/ops/error-logs`：旧的「limit 列表」接口
  - 前端：已切换到分页 `GET /admin/ops/errors`，该接口目前属于冗余/兼容层

---

## 2. 现状：前端 Ops 页面/组件/交互总览

入口：`frontend/src/router/index.ts` 路由 `/admin/ops` 指向 `frontend/src/views/admin/ops/OpsDashboard.vue`。

### 2.1 前端已有且行为闭环

- 指标看板 + 图表：`frontend/src/views/admin/ops/components/OpsMetricsCharts.vue`
- 错误日志列表/筛选/分页/重试：`frontend/src/views/admin/ops/components/OpsErrorLogTable.vue`
- 请求明细弹窗（图表/卡片 drill-down）：`frontend/src/views/admin/ops/components/OpsRequestDetailsModal.vue`
- 分组可用性卡片：`frontend/src/views/admin/ops/components/OpsGroupAvailabilityCard.vue`
- 邮件通知配置卡片：`frontend/src/views/admin/ops/components/OpsEmailNotificationCard.vue`
- 运行时设置卡片（含全局静默）：`frontend/src/views/admin/ops/components/OpsRuntimeSettingsCard.vue`

### 2.2 前端「有元素但无行为」（当前最明显的断点）

- Ops 顶部工具条的「Platform / Group」全局筛选器：
  - 组件：`frontend/src/views/admin/ops/components/OpsDashboardHeader.vue`
  - 现状：Header 内部维护 `selectedPlatform` / `selectedGroupId`，并会 `emit('update:platform')`、`emit('update:group')`
  - 缺口：`frontend/src/views/admin/ops/OpsDashboard.vue` **未监听** `@update:platform` / `@update:group`，因此 UI 选择对任何数据请求都没有影响（典型“假控件”）
  - 影响：用户以为在过滤全局数据，但实际没有过滤，容易导致排障误判
  - 建议：
    - 方案 A（补齐行为）：将 platform/group 作为顶层 state，应用到 `GET /admin/ops/errors`、`GET /admin/ops/requests`（后端已支持 `platform`/`group_id` 参数），必要时将 selection 映射到现有 `errorFilters`；
    - 方案 B（去除过度设计）：如果暂不做“全局过滤”，直接移除这两个筛选控件，避免误导。

### 2.3 前端存在“半成品/文档超前/类型错配”

- `frontend/src/views/admin/ops/components/OpsConfigDialog.vue`
  - 现状：已在 `frontend/src/views/admin/GroupsView.vue` 中作为「分组监控配置弹窗」被使用
  - 缺口：`frontend/src/views/admin/ops/components/README.md` 描述的 Tab1 告警规则、Tab3 邮件配置等内容 **并未在组件中实现**（README 明显过期）
  - 建议：更新 README 以匹配实际能力，或补齐 Tabs（但要注意避免重复入口：Ops 页已有 `OpsEmailNotificationCard` / `OpsRuntimeSettingsCard`）

- 告警规则前端类型与后端/数据库语义不一致（会导致未来 UI 对接必炸）
  - 前端：`frontend/src/views/admin/ops/types.ts` 中 `AlertSeverity = 'critical' | 'warning' | 'info'`
  - 后端/DB：`backend/migrations/019_ops_alerts.sql` 中 `ops_alert_rules.severity VARCHAR(4) DEFAULT 'P1'`，以及种子数据 `P1/P2`
  - 结论：`AlertRule.severity` 在“系统告警规则”场景应该是 `P0..P3` 这类短码，而不是 `critical/warning/info`（后者用于分组可用性事件更合理）
  - 建议：拆分类型：
    - 例如 `OpsAlertRuleSeverity = 'P0'|'P1'|'P2'|'P3'`
    - 与 `GroupAvailabilitySeverity = 'critical'|'warning'|'info'` 分离，避免混用

---

## 3. 前后端对接不完整点（按优先级）

### P0（会误导或影响日常排障）

1) **Ops Header 的 platform/group 筛选器无效**
   - 见第 2.2

2) **告警规则 metric_type 兼容性问题：DB 种子包含 `http2_errors`，但 API 校验不允许**
   - DB 种子：`backend/migrations/021_seed_ops_alert_rules_more.sql` 包含 `metric_type='http2_errors'`
   - API 校验白名单：`backend/internal/handler/admin/ops_handler.go` 的 `validOpsAlertMetricTypes` 不含 `http2_errors`
   - 影响：规则存在于库中，但通过 API 更新/保存时会被拒绝（Create/Update 都会走校验）
   - 建议：二选一
     - 方案 A：将 `http2_errors` 加回允许列表，并保证 metrics collector 能产出该指标
     - 方案 B：迁移/清理掉该规则与指标，避免“库里有但 UI/API 不可编辑”的僵尸配置

### P1（后端能力存在但前端缺少入口/对接）

1) 告警规则 CRUD（后端 + 前端 API 已有，但 Ops 页面缺 UI）
   - 后端：`backend/internal/server/routes/admin.go`
   - 前端 API：`frontend/src/api/admin/ops.ts`（`listAlertRules/create/update/delete`）
   - 缺口：无 UI/无入口（可能是规划未完成）

2) 告警事件历史（后端已有，前端缺少 API + UI）
   - 后端：`GET /admin/ops/alert-events`
   - 前端：未实现

3) 分组可用性事件历史（后端已有，前端缺少 UI）
   - 后端：`GET /admin/ops/group-availability/events`
   - 前端：未实现

4) IP 维度的错误聚合（后端已有，前端缺少 UI）
   - 后端：`GET /admin/ops/errors/by-ip`、`/errors/by-ip/:ip`
   - 前端：未实现

5) 错误 stats / timeseries（后端已有，前端缺少图表）
   - 后端：`GET /admin/ops/error-stats`、`GET /admin/ops/error-timeseries`
   - 前端：未实现

6) 账号状态概览（后端已有，前端缺少 UI）
   - 后端：`GET /admin/ops/account-status`
   - 前端：未实现

### P2（可用性/一致性/过度设计风险）

1) Alert silencing 的 entries（按 rule_id / severity 细粒度静默）后端已有，但前端只支持 global
   - 后端：`backend/internal/service/ops_alert_service.go` 的 `isSilenced()` 支持 `cfg.Entries`
   - 前端：`frontend/src/views/admin/ops/components/OpsRuntimeSettingsCard.vue` 只展示/编辑 global 字段（`entries` 仅做兜底初始化）
   - 建议：如果确实需要“维护窗口仅静默某一类规则”，再补 UI；否则可以将 entries 视作高级功能，明确隐藏/不支持。

2) 旧 cron 代码未被接入（可能是历史遗留/过度设计）
   - `backend/internal/cron/cron.go`、`backend/internal/cron/ops_aggregator.go`、`backend/internal/cron/ops_cleanup.go`
   - 现状：仓库内存在，但无任何地方 `NewManager()` 或 `AddJob()`，因此不会运行
   - 当前实际聚合由 `backend/internal/service/ops_aggregation_service.go` 的循环实现
   - 建议：要么删除 cron 目录相关代码以减少认知负担，要么补齐 wiring 并明确其职责边界（避免与 OpsAggregationService 重复）

---

## 4. 建议的“下一步补齐顺序”（避免重复建设）

1) 先处理 P0：修正/移除 Ops Header 的 platform/group“假控件”
2) 明确告警规则的严重级别体系（P0..P3 vs critical/warning/info）并修正前端类型
3) 决定是否需要“告警规则管理 UI”
   - 若需要：补 UI + 同时补 `alert-events` 浏览入口（否则很难追溯告警是否触发/是否发出邮件）
4) 决定是否需要 IP 维度聚合与错误时序图（更偏排障/安全运营）

