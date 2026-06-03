# Platform Scheduling Thresholds And Temp Unsched Reason Design

## Goal

把“达到某个使用率百分比就暂停账号调度，窗口刷新后自动恢复”从 OpenAI OAuth 扩展为系统级的分平台配置：

1. 每个平台单独配置一个阈值百分比
2. `100%` 表示该平台关闭此功能
3. 各平台按自己当前已经存在的原生窗口或已持久化窗口信号生效
4. 命中阈值后统一落到现有 `temp_unschedulable_until / temp_unschedulable_reason`
5. 所有 temp unsched 记录都必须有非空、可读、可诊断的 reason

## Scope

本轮覆盖：

1. 系统设置新增“分平台调度停用阈值”JSON map
2. OpenAI、Anthropic、Gemini、Kiro、Antigravity 五个平台的阈值判断接入
3. temp unsched reason 的统一结构、仓储兜底和管理端展示修复
4. 管理端 Settings 页面与 temp unsched 详情弹窗
5. 相关后端与前端测试

本轮不覆盖：

1. 用户级、分组级、账号级覆盖阈值
2. 新增数据库实体列或 migration
3. 重新设计现有限流、429、手动 `schedulable=false` 语义

## Settings Design

### Storage Shape

采用单个 JSON key，而不是拆成多个独立 setting key。

新增系统设置键：

- `account_scheduling_thresholds`

值形态：

```json
{
  "openai": 100,
  "anthropic": 100,
  "gemini": 100,
  "kiro": 100,
  "antigravity": 100
}
```

语义：

1. 每个平台是 `1-100` 的整数阈值
2. `100` 表示关闭该平台的自动停调阈值
3. key 缺失时按 `100` 处理
4. `nil` request 表示“不修改”
5. 非 `nil` request 表示整体替换

### Why JSON Map

这个仓库里最合适的现成模式是 `default_platform_quotas`：

1. 一个 setting key
2. 一个平台 map
3. 后端统一校验和补齐平台
4. 前端以平台表格形式展示

因此本轮沿用同样的“单 key + 平台 map”模式，而不是为五个平台分别加五个 setting key。

## Platform Signal Model

### OpenAI

现有运行态信号已经足够直接使用：

1. `codex_5h_used_percent`
2. `codex_5h_reset_at`
3. `codex_7d_used_percent`
4. `codex_7d_reset_at`

来源：

1. 正常响应头快照
2. 429 响应头快照
3. Codex probe
4. 账号测试链路

本轮直接使用，不新增新的 OpenAI 窗口采集。

### Anthropic

现有运行态信号也足够直接使用：

1. `session_window_utilization` + `session_window_end` 表示 5h
2. `passive_usage_7d_utilization` + `passive_usage_7d_reset` 表示 7d

来源：

1. 上游成功响应头
2. 429 头部
3. OAuth usage 主动查询后回写被动缓存

本轮直接使用，不新增新的 Anthropic 窗口采集。

### Gemini

Gemini 当前有两类可用窗口语义：

1. 日窗口：`gemini_usage_raw` 中的 bucket snapshot，可反推出 Pro/Flash 的利用率和 reset time
2. 分钟窗口：现有 `PreCheckUsage` / `PreCheckUsageBatch` 已实时根据 usage logs + quota policy 计算 RPM 窗口和 reset time

本轮规则：

1. 日窗口阈值使用 `gemini_usage_raw` 持久化快照
2. 分钟窗口阈值接入现有 precheck 计算结果，在请求路径上实时触发 temp unsched

因此 Gemini 可以在本轮实现，无需新增 DB 列。

### Kiro

Kiro 现有真实语义是总额度/月度额度：

1. `CurrentUsage / UsageLimit`
2. `ResetAt()`
3. 月度、赠送、试用、总额度 breakdown

但这些信号当前主要在 usage fetch 链路里，不是 scheduler 现成可读的 `Account.Extra` 快照。

因此本轮要先补一层轻量持久化快照，例如：

1. `kiro_sched_utilization`
2. `kiro_sched_reset_at`
3. `kiro_sched_usage_updated_at`

来源使用现有 `getKiroUsage(...)` / `FetchUsageLimits(...)` 的结果，不新增新的上游接口。

### Antigravity

Antigravity 当前真实信号是“模型级 quota 利用率 + reset time”：

1. `UsageInfo.AntigravityQuota[model].Utilization`
2. `UsageInfo.AntigravityQuota[model].ResetTime`
3. `AICredits` 额度状态
4. `model_rate_limits` 是已经 100% 触发后的运行态，不够支持低于 100 的阈值

因此本轮要补一层账号级聚合快照：

1. 取所有模型 quota 中利用率最高的 scope
2. 当多个 scope 同时达到阈值时，取更晚的 reset time
3. 将聚合结果持久化到 `Account.Extra`

建议快照字段：

1. `antigravity_sched_utilization`
2. `antigravity_sched_reset_at`
3. `antigravity_sched_scope`
4. `antigravity_sched_usage_updated_at`

来源使用现有 `AntigravityQuotaFetcher.FetchQuota(...)` 的结果，不新增新的上游接口。

## Threshold Evaluation Semantics

### Common Rule

当平台阈值 `< 100`，且该平台当前存在未过期的窗口/快照信号时：

1. 任一窗口利用率 `>= threshold`
2. 则账号进入 temp unschedulable
3. `until` 取所有命中窗口中最晚的 reset time

这样可以避免：

1. 一个较短窗口恢复后，较长窗口仍超限却被提前恢复
2. 需要等下一次请求才能重新发现更长窗口仍在高位

### Missing Reset Time

如果利用率达阈值，但当前没有可用 reset time：

1. 不新增 temp unsched
2. 记录 warning
3. 保留已有运行态

原因是没有 reset time 就无法满足“窗口刷新后自动恢复”的基本语义。

### Feature-Owned Source

所有由本功能产生的 temp unsched reason 统一使用：

- `source=account_scheduling_threshold`

并在 payload 中附带：

1. `platform`
2. `window` 或 `scope`
3. `threshold_percent`
4. `used_percent`
5. `until_unix`
6. `triggered_at_unix`
7. `error_message`

这样在 setting 变化或窗口失效后，只清这个功能自己产生的 block，不误清其他来源。

## Runtime Integration Strategy

### Shared Evaluator

需要增加统一 evaluator，而不是各平台各写一套临时逻辑。

职责：

1. 读取系统设置中的平台阈值
2. 根据账号平台解析该平台的当前窗口快照
3. 判断是否命中阈值
4. 构造 feature-owned temp unsched reason
5. 决定设置、刷新或清除 temp unsched

### Integration Points

#### OpenAI

接入所有现有 Codex 快照写入点：

1. `RateLimitService.persistOpenAICodexSnapshot`
2. `OpenAIGatewayService.updateCodexUsageSnapshot`
3. `AccountUsageService.persistOpenAICodexProbeSnapshot`
4. `AccountTestService` OpenAI 快照更新点

#### Anthropic

接入现有 passive/active usage 更新点：

1. `RateLimitService.UpdateSessionWindow`
2. `AccountUsageService.syncActiveToPassive`
3. 任何现有写回 `session_window_utilization` / `passive_usage_7d_utilization` 的链路

#### Gemini

接入两个层面：

1. `gemini_usage_raw` 更新或回填后做 daily 阈值同步
2. `PreCheckUsage` / `PreCheckUsageBatch` 中对 minute / daily 结果做实时阈值同步

#### Kiro

在 `getKiroUsage(...)` 成功拿到 usage 后，把调度快照写回 `Account.Extra`，然后同步 temp unsched。

#### Antigravity

在 `getAntigravityUsage(...)` / `AntigravityQuotaFetcher.FetchQuota(...)` 成功后，把聚合后的账号级阈值快照写回 `Account.Extra`，然后同步 temp unsched。

## Setting Change Reconciliation

管理员修改 `account_scheduling_thresholds` 后，不能等下一次快照写入才生效。

因此需要一次异步重算：

1. 扫描活动账号
2. 按平台读取当前已存快照
3. 重新判断是否命中阈值
4. 命中的补写 feature-owned temp unsched
5. 已不命中的只清除 `source=account_scheduling_threshold` 的记录

不要求阻塞 settings 保存接口，但要求保存后尽快生效。

## Temp Unsched Reason Normalization

### Problem

当前 temp unsched reason 来源不统一：

1. 有些是结构化 JSON
2. 有些是纯文本
3. 有些链路允许写空字符串
4. 前端详情页会看到空白原因

### New Rule

以后所有 temp unsched 写入必须满足：

1. `temp_unschedulable_reason` 非空
2. 优先写结构化 JSON
3. `error_message` 必须可直接展示给管理员

### Repository Safety Net

`accountRepository.SetTempUnschedulable(...)` 增加最后一道兜底：

1. 若 reason 为空
2. 仓储层自动生成最小 payload
3. 至少包含：
   - `source=unspecified`
   - `until_unix`
   - `triggered_at_unix`
   - `error_message=Temporary unschedulable (reason unavailable)`

### Read Compatibility

`tempUnschedStateFromStoredReason(...)` 继续兼容：

1. JSON reason
2. 纯文本 reason
3. 空字符串旧数据 -> 默认 fallback message

## Admin UI

### Settings Page

在现有调度设置区域新增“分平台阈值”表格：

1. 一行一个平台
2. 一个 `1-100` 输入框
3. 默认值 `100`

平台列表使用统一常量：

1. `openai`
2. `anthropic`
3. `gemini`
4. `kiro`
5. `antigravity`

### Temp Unsched Status Modal

现有弹窗继续复用，但需要两处修正：

1. `error_message` 为空时显示 fallback，不显示空白
2. `rule_index < 0` 时显示 `-`，不显示 `0`

## Testing Plan

### Backend Unit Tests

1. settings map 读写、默认值、边界值、缺省平台补齐
2. OpenAI evaluator：5h / 7d / 双窗口 / `100=disabled`
3. Anthropic evaluator：5h / 7d / 双窗口
4. Gemini evaluator：daily snapshot、minute precheck、shared/pro/flash 取最高利用率
5. Kiro evaluator：持久化月度快照后的阈值判断
6. Antigravity evaluator：聚合模型 quota 后的阈值判断
7. feature-owned reason 只清自己
8. repository 空 reason 兜底
9. 旧空 reason / 纯文本 reason 读取兼容

### Backend Integration / Behavior Tests

1. 命中阈值后 `IsSchedulable()` 返回 false
2. `ListSchedulableByGroupID*` 自动排除
3. 分组可用数自动减少
4. OpenAI / 通用 scheduler 都自动排除

### Frontend Tests

1. Settings 页加载、编辑、保存平台阈值表
2. `100` 的回显与 payload 正确
3. TempUnschedStatusModal 的 fallback message 和负 rule index 显示正确

## Risks

### Risk: 误清除其他来源的 temp unsched

缓解：

1. 只清 `source=account_scheduling_threshold`

### Risk: Kiro / Antigravity 的调度阈值依赖快照刷新

缓解：

1. 本轮补 `Account.Extra` 持久化快照
2. 后续需要时再扩展为更主动的 runtime collection

### Risk: Gemini minute / daily 语义与 snapshot 混用

缓解：

1. minute 只在现有 precheck 实时路径处理
2. daily 只基于 persisted snapshot
3. reason 中写清具体 window
