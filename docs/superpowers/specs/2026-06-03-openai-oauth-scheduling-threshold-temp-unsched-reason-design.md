# OpenAI OAuth Scheduling Threshold And Temp Unsched Reason Design

## Goal

为系统增加一个全局 OpenAI OAuth 调度停用阈值配置：

1. 对所有 OpenAI OAuth 账号生效
2. 基于上游 Codex 已有的 `5h / 7d` 使用率百分比信号判断
3. 任一窗口达到阈值时，账号自动进入“临时不可调度”
4. 对应窗口刷新后，账号自动恢复调度
5. 阈值为 `100%` 时，该功能完全不生效

同时修复当前“临时不可调度有时没有具体原因”的问题，确保所有 temp unschedulable 记录都能返回非空、可读、可诊断的原因。

## Scope

本轮覆盖：

1. 系统设置新增 OpenAI OAuth 全局调度停用阈值
2. OpenAI OAuth Codex `5h / 7d` 快照与 temp unschedulable 运行态联动
3. temp unschedulable reason 的统一结构、写入规范和读取兜底
4. 管理端 Settings 页面与 temp unschedulable 详情展示
5. 相关后端与前端测试

本轮不覆盖：

1. OpenAI API Key 账号
2. 非 OpenAI 平台账号
3. 账号级或分组级阈值覆盖
4. 基于月窗口的提前停调
5. 新增数据库实体列或 migration

## Background

当前仓库里，OpenAI OAuth 已经有可复用的 Codex 运行时信号：

1. `codex_5h_used_percent`
2. `codex_5h_reset_at`
3. `codex_7d_used_percent`
4. `codex_7d_reset_at`
5. `codex_usage_updated_at`

这些信号的来源已经存在：

1. OpenAI 429 响应头
2. OpenAI 正常响应头快照持久化
3. OpenAI Codex probe
4. OpenAI 账号测试链路

同时，现有调度链路已经把以下运行态纳入统一“不可调度”口径：

1. `schedulable = false`
2. `rate_limit_reset_at > NOW()`
3. `overload_until > NOW()`
4. `temp_unschedulable_until > NOW()`

因此，本轮最小改动路径不是新增新的 SQL 过滤字段，而是复用现有 `temp_unschedulable_until / temp_unschedulable_reason`。

## Approach Options

### Option A: 只对现有 `5h / 7d` 上游信号生效

优点：

1. 完全复用现有 OpenAI OAuth 运行态字段
2. 不需要推断或发明月度额度语义
3. 调度层不需要新增专用口径，直接复用 temp unschedulable
4. 改动最小，行为最稳定

缺点：

1. 本轮不能实现“月窗口低于 100% 时提前停调”

### Option B: 额外引入本地月度预算配置

优点：

1. 可以在本轮凑出“5h / 7d / 月”三个窗口

缺点：

1. 月窗口会变成本网关本地成本语义，不是上游真实月额度语义
2. 会把一个“单阈值”需求扩成“两套配置”
3. 容易和用户理解的“OpenAI 月度上游窗口”产生偏差

### Option C: 额外补一条上游月度额度探测链路

优点：

1. 语义最完整，真正同时覆盖 `5h / 7d / 月`

缺点：

1. 当前仓库里没有现成月度 percent/reset 运行态
2. 上游接口不稳定，探测成本与风险最高
3. 会显著扩大本轮范围

推荐采用 Option A。

## Settings Design

新增系统设置键：

- `openai_oauth_sched_pause_threshold_percent`

语义：

1. 取值范围 `1-100`
2. 默认值 `100`
3. 仅对 `platform=openai` 且 `type=oauth` 的账号生效
4. 只基于 Codex `5h / 7d` 使用率百分比判断
5. `100` 表示关闭功能，不做提前停调

### Backend Changes

需要同步扩展：

1. `backend/internal/service/domain_constants.go`
2. `backend/internal/service/setting_service.go`
3. `backend/internal/handler/admin/setting_handler.go`
4. `backend/internal/handler/dto/settings.go`

### Frontend Changes

需要同步扩展：

1. `frontend/src/api/admin/settings.ts`
2. `frontend/src/views/admin/SettingsView.vue`
3. 中英文 i18n 文案

### UI Placement

字段放在现有 OpenAI 调度配置区块，和以下项同一区域：

1. `openai_advanced_scheduler_enabled`
2. `openai_sticky_reserve_percent`
3. `openai_sticky_wait_timeout_seconds`

文案必须明确说明：

1. 仅对 OpenAI OAuth 生效
2. 基于上游 Codex `5h / 7d` 使用率
3. 任一窗口达到阈值即临时停止调度
4. 窗口刷新后自动恢复
5. `100` 表示关闭

## Runtime Evaluation Design

### Canonical Rule

当以下条件同时满足时，账号进入 temp unschedulable：

1. 账号是 OpenAI OAuth
2. 全局阈值 `< 100`
3. 存在未过期的 Codex 窗口数据
4. `codex_5h_used_percent >= threshold` 或 `codex_7d_used_percent >= threshold`

### Window Selection

1. 仅一个窗口达到阈值时，`until` 使用该窗口的 `reset_at`
2. 两个窗口都达到阈值时，`until` 使用两个窗口中更晚的 `reset_at`

这样可以避免：

1. 5h 提前恢复但 7d 仍超阈值时短暂重新参与调度
2. 需要依赖下一次上游请求才能重新发现 7d 仍然超限

### Missing Reset Time

若 `used_percent >= threshold` 但对应窗口没有可用的 `reset_at`：

1. 不设置新的 temp unschedulable
2. 记录 warning 日志
3. 保留已有运行态不主动扩大封禁

原因：

1. 没有恢复时间就无法实现“窗口刷新后自动恢复”
2. 不能因为缺失 reset time 而把账号无限期停掉

### Disabled State

当阈值为 `100` 时：

1. 不新增任何基于该功能的 temp unschedulable
2. 若某账号当前 temp unschedulable 是本功能产生的，则应清除
3. 若当前 temp unschedulable 来源于其他功能，则绝不清除

## Feature-Owned Temp Unsched Design

本轮新增一个明确的 feature-owned source：

- `openai_oauth_sched_pause_threshold`

所有由该功能生成的 temp unsched reason 都写成结构化 JSON，并至少包含：

1. `source`
2. `until_unix`
3. `triggered_at_unix`
4. `error_message`

对阈值功能还额外包含：

1. `window`，取值 `5h` / `7d` / `5h+7d`
2. `threshold_percent`
3. `used_percent_5h`（可选）
4. `used_percent_7d`（可选）

推荐错误消息格式：

- `OpenAI OAuth scheduling paused: Codex 7d usage 91% reached threshold 90%; resumes when the 7d window resets`

当两个窗口都命中时：

- `OpenAI OAuth scheduling paused: Codex 5h/7d usage reached threshold 90%; resumes after the later window resets`

## Shared Helper Design

不要在多个服务里各自手写 threshold 判断。需要增加一个统一 helper，负责：

1. 基于 `extra` 解析 `5h / 7d` 使用率与 reset time
2. 读取系统设置阈值
3. 决定是否设置 temp unschedulable
4. 决定是否清除该功能自己写入的 temp unschedulable
5. 构造结构化 reason

这个 helper 应作为以下链路的唯一入口：

1. `RateLimitService.persistOpenAICodexSnapshot`
2. `OpenAIGatewayService.updateCodexUsageSnapshot`
3. `AccountUsageService.persistOpenAICodexProbeSnapshot`
4. `AccountTestService` 中 OpenAI Codex 快照更新点

目标：

1. 429、正常响应、probe、账号测试都遵循同一规则
2. 设置值变化后，行为不会因写入链路不同而漂移

## Why Temp Unsched Instead Of SQL Threshold Filtering

本轮明确不采用“在账号查询 SQL 上直接加 `codex_*_used_percent` 条件”的方式。

原因：

1. 当前仓库所有调度口径已经统一使用 temp unschedulable
2. 复用 temp unsched 能自动覆盖：
   - 账号仓储的 schedulable 列表
   - 分组可用数统计
   - sticky session 清理
   - OpenAI 专用 scheduler
3. 不需要在多个 SQL 和多个 service 里重复加入新逻辑
4. 原因、恢复时间、管理端展示都可以直接沿用现有 temp unsched 基础设施

## Setting Change Reconciliation

仅靠“下一次上游快照写入”还不够，因为管理员可能会直接修改系统设置。

因此当 `openai_oauth_sched_pause_threshold_percent` 变化时，需要触发一次异步重算：

1. 扫描活动中的 OpenAI OAuth 账号
2. 基于当前已存的 Codex `5h / 7d` 快照重新计算
3. 对达到阈值的账号补写 feature-owned temp unsched
4. 对当前由该功能产生、但已经不应继续阻断的账号清除 temp unsched

这次重算必须遵循“只清自己写的 block”的规则。

不要求同步阻塞设置保存请求，但要求保存后尽快生效。

## Temp Unsched Reason Normalization

### Current Problem

当前 temp unschedulable reason 的来源不统一：

1. 有的写结构化 JSON
2. 有的直接写纯文本
3. 有的链路允许写空字符串
4. 读取时已经存在“缓存为空再回库兜底”的补丁，说明空原因在真实运行中确实出现

### New Rule

以后所有 temp unsched 写入都必须满足：

1. `temp_unschedulable_reason` 非空
2. 优先使用结构化 JSON
3. `error_message` 必须是可直接展示给管理员的可读文本

### Repository Safety Net

在 `accountRepository.SetTempUnschedulable(...)` 增加最后一道兜底：

1. 若调用方传入空 reason
2. 仓储层自动生成一个最小结构化 reason
3. 至少保证：
   - `until_unix`
   - `triggered_at_unix`
   - `source=unspecified`
   - `error_message=Temporary unschedulable (reason unavailable)`

这样即使未来又有新分支漏写 reason，也不会再落库为空值。

### Read Compatibility

`tempUnschedStateFromStoredReason(...)` 继续兼容历史数据：

1. JSON reason 正常解析
2. 纯文本 reason 包装为 `error_message`
3. 空字符串 reason 返回一个带默认 `error_message` 的状态，而不是空白消息

## Existing Write Paths To Normalize

以下 temp unsched 写入分支需要统一改成走公共 reason builder 或同等结构化写法：

1. OAuth 401 cooldown
2. OpenAI 403 temporary cooldown
3. token refresh retry exhausted
4. stream timeout temp unsched
5. retry exhausted / failover temp unsched
6. TTFT watchdog timeout
7. Antigravity 的 Google config error / empty response 快捷 temp unsched
8. temp_unschedulable_rules 命中后通过 `persistTempUnschedulableState(...)` 写入的链路

对已有结构化链路：

1. 维持兼容字段
2. 补充统一 `source`
3. 保证 `error_message` 非空

## Admin UI Impact

### Settings Page

新增一个数字输入：

- `OpenAI OAuth 调度停用阈值 (%)`

约束：

1. 最小 `1`
2. 最大 `100`
3. 默认显示 `100`

### Temp Unsched Status Modal

现有 `TempUnschedStatusModal` 基本可复用，只需要做两处修正：

1. `error_message` 为空时显示后端兜底值，不显示空白
2. `rule_index < 0` 时，前端不要显示 `0`，而是显示 `-`

第二点是因为系统级 temp unsched（例如流超时、全局阈值）不属于账号自定义 rule 列表，当前 `rule_index + 1` 会把 `-1` 显示成 `0`。

## Testing Plan

### Backend Unit Tests

1. 设置读写、默认值与边界校验
2. `threshold=100` 时功能禁用
3. `5h` 达阈值时写 temp unsched
4. `7d` 达阈值时写 temp unsched
5. `5h + 7d` 同时达阈值时取更晚的 `reset_at`
6. 窗口已过期时不阻断
7. 缺失 `reset_at` 时不新增阻断
8. 当前 reason 为 feature-owned 时，条件不再满足可正确清除
9. 当前 reason 为其他来源时，条件不再满足不得误清除
10. 所有 temp unsched reason builder 输出非空 `error_message`
11. repository 空 reason 安全兜底
12. 历史空 reason / 纯文本 reason 的读取兼容

### Backend Integration / Behavior Tests

1. OpenAI OAuth 账号被该功能 temp unsched 后，`IsSchedulable()` 返回 false
2. `ListSchedulableByGroupID*` 自动排除
3. 分组可用数自动减少
4. OpenAI account scheduler 自动排除

### Frontend Tests

1. Settings 页加载、编辑、保存新字段
2. `100` 的 payload 与回显正确
3. TempUnschedStatusModal 对空消息显示兜底文案
4. `rule_index = -1` 不再显示 `0`

## Risks And Mitigations

### Risk: 误清除其他来源的 temp unsched

缓解：

1. 只对 `source=openai_oauth_sched_pause_threshold` 的记录做自动清除
2. 其他来源只读不动

### Risk: 5h 恢复后 7d 仍高但账号提前恢复

缓解：

1. 当多个窗口同时命中时，`until` 取更晚的 reset time

### Risk: 上游快照不完整

缓解：

1. 缺失 reset time 时不新增阻断
2. 记录 warning，保留可诊断信息

## Out Of Scope Follow-Up

若后续要支持“月窗口低于 100% 就提前停调”，下一轮需要二选一：

1. 增加真正的上游月度 percent/reset 信号采集
2. 明确引入一套本地月度预算配置，并接受其不是上游原生语义

本轮不做这项扩展。
