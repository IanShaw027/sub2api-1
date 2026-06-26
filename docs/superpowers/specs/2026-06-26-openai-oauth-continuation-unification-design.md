# OpenAI OAuth Continuation Unification Design

## Goal

为 OpenAI OAuth `/v1/responses` 路径建立一套统一的 continuation 架构，满足以下目标：

1. 尽可能优先走增量上传，而不是全量重放
2. 优先命中低延迟、低错误率的热续链路径
3. 在错误发生前通过本地条件判断规避不安全/高概率失败的请求
4. 在无法继续增量时，使用可恢复窗口做最佳回退，而不是盲目丢掉 `previous_response_id`
5. 尽可能保持同会话、同账号、同连接的 sticky 语义
6. 让 HTTP continuation 成为正式能力，而不是当前“仅 WS 支持、HTTP 默认删 anchor”的半实现状态

## Scope

本轮覆盖：

1. OpenAI OAuth `/v1/responses` continuation 统一决策
2. OAuth `store=false` 主 lane 的 `Hot WS -> Full Rebuild` ladder，以及可选 durable continuation lane
3. `previous_response_id`、sticky account、sticky conn、session rebuild window 的统一状态管理
4. `previous_response_not_found`、unsafe tool continuation、compact cutover 等恢复策略
5. 相关配置开关、可观测性、回归测试

本轮不覆盖：

1. 非 OpenAI 平台 continuation 体系重构
2. Anthropic / Gemini continuation 语义统一
3. 新增数据库表或 migration
4. 对外 API 协议新增字段
5. 改变已有 usage 计量口径

## Problem Statement

当前实现已经具备一部分 continuation 能力，但仍存在四个结构性问题：

1. **HTTP continuation 不是正式能力**
   - 当前代码在非 WS transport 下会删掉 `previous_response_id`
   - `SelectAccountByPreviousResponseID(...)` 也只在 WSv2 transport 下生效
   - 结果是官方支持的 HTTP continuation 在本仓库内并未真正打通

2. **decision/fallback 逻辑分散**
   - `openai_gateway_service.go`
   - `openai_ws_forwarder.go`
   - `openai_ws_v2_passthrough_adapter.go`
   三处各自持有一套 continuation 语义，容易出现路径漂移

3. **恢复策略过度依赖“删 anchor 重试”**
   - 当前 `previous_response_not_found` 在很多分支上仍以“删除 `previous_response_id` 后重发当前 turn”为主
   - 对 tool/reasoning 场景，这不是最佳回退，且可能导致语义损失

4. **缺少 canonical rebuild window**
   - 当前 state store 能保存：
     - `response_id -> account_id`
     - `response_id -> conn_id`
     - `session -> conn`
     - strict-delta shadow context
   - 但缺少“冷恢复/新链重建时必须要有的最小可恢复窗口”

## Design Summary

本轮采用一套统一 continuation planner，把 continuation 行为从“分支 if/else”改成“显式状态机”。

统一 planner 只输出四种动作：

1. `HotWSIncremental`
2. `ColdHTTPIncremental`
3. `FullRebuild`
4. `RejectUnsafeContinuation`

配套新增两类状态：

1. `session -> latest_response_id`
2. `session -> canonical rebuild window`

这样可以把 continuation 设计成两条 lane：

1. **OAuth `store=false` 主 lane**：`Hot WS Incremental -> Full Rebuild`
2. **可选 durable lane**：仅在显式允许 persisted continuation 时，才允许 `Cold HTTP Incremental`

## Continuation Lanes

### Lane A: Hot WS Incremental

触发条件：

1. 当前会话存在活跃 upstream WS
2. 请求带 `previous_response_id`
3. `previous_response_id` 与当前会话最新 response 可对齐
4. 未命中 `background`
5. 未处于 standalone compact cutover
6. 未触发 tool/reasoning 不安全条件

行为：

1. 保留 `previous_response_id`
2. 仅发送新增 input
3. 维持同连接/同账号 sticky
4. OpenAI OAuth 主 lane 固定保持 `store=false`

### Lane B: Cold Durable Incremental

触发条件：

1. 请求不走活跃 WS 热路径
2. 请求带 `previous_response_id`
3. lane 被显式标记为允许 persisted continuation
4. planner 已确认该 anchor 不依赖活跃 WS 的内存缓存
5. 未处于 compact cutover
6. 未命中 `previous_response_not_found`

行为：

1. 保留 `previous_response_id`
2. 只发送新增 input
3. 优先同账号 continuation
4. 不属于 OAuth `store=false` 主 lane 默认行为
5. 仅在显式 durable/实验 lane 下启用

### Lane C: Full Rebuild

触发条件：

1. OAuth `store=false` 主 lane 失去热 WS continuation 条件
2. `previous_response_not_found`
3. tool continuation 缺少足够 replay context
4. reasoning continuation 缺少必要恢复窗口
5. standalone compact 之后的 cutover
6. 明确从旧分叉点继续，而不是最近一轮
7. planner 判定当前 continuation 高风险或不安全

行为：

1. 丢弃旧 `previous_response_id`
2. 使用 canonical rebuild window 重建请求
3. 开启新链
4. 对 OAuth lane 继续保持 `store=false`

### Lane D: Reject Unsafe Continuation

触发条件：

1. `function_call_output` 但没有完整 tool context
2. `previous_response_id` 不是合法 `resp_*`
3. 其他 planner 可静态判定的不安全请求

行为：

1. 不发上游
2. 直接返回明确本地错误
3. 保留排障日志与拒绝原因

## Planner Contract

新增文件：

- `backend/internal/service/openai_responses_continuation_planner.go`
- `backend/internal/service/openai_responses_continuation_planner_test.go`

planner 输入结构建议包含：

1. `AccountType`
2. `TransportDecision`
3. `PreviousResponseID`
4. `SessionHash`
5. `PromptCacheKey`
6. `HasFunctionCallOutput`
7. `HasReasoning`
8. `HasBackground`
9. `StoreDisabled`
10. `LiveWSAvailable`
11. `StickyAccountHit`
12. `StickyConnHit`
13. `SessionWindowAvailable`
14. `CompactCutover`
15. `RecoveryReason`
16. `DurableContinuationAllowed`
17. `PersistedContinuationAvailable`

planner 输出结构：

1. `Action`
2. `PreservePreviousResponseID`
3. `PreserveStickyAccount`
4. `PreserveStickyConn`
5. `RequiresFullReplayWindow`
6. `Reason`

关键原则：

1. planner 决定 continuation 策略
2. transport executor 只负责执行 planner 的决定
3. 所有 fallback 先转换为 planner 输入，再统一决定动作
4. OAuth `store=false` 主 lane 不把 HTTP cold continuation 作为默认 fallback

## Sticky Model

### Sticky Layers

为尽可能确保粘性会话，本轮保留并补全三层 sticky：

1. `response_id -> account_id`
   - 跨 transport 的 same-account continuation anchor
2. `response_id -> conn_id`
   - 热 WS continuation 的 same-conn anchor
3. `session -> latest_response_id / rebuild window`
   - 断连后冷恢复、新链重建的 anchor

### Force HTTP Semantics

现有 `force_http` 语义是：

1. 禁止 upstream WS transport
2. 同时忽略 `previous_response_id` sticky

本轮调整为：

1. `force_http` 仅表示 transport forced to HTTP
2. 对 OAuth `store=false` 主 lane，不再尝试“默认 HTTP 冷续链”
3. planner 需在 `force_http` 下优先判定：
   - 是否存在显式 durable continuation lane
   - 否则直接 `FullRebuild`

这样可以让 `force_http` 从“忽略 sticky 的旧行为”升级为“禁止热 WS，但不伪造 HTTP persisted continuation”。

### Sticky Scope

同账号 sticky 应继续作用于：

1. WS hot continuation
2. 显式 durable cold continuation
3. rebuild 后的后续新链

同连接 sticky 只作用于：

1. 活跃 WS continuation
2. strict-delta / active-delta 热路径

## Canonical Rebuild Window

新增文件：

- `backend/internal/service/openai_responses_session_window.go`
- `backend/internal/service/openai_responses_session_window_test.go`

该窗口是 recovery 与 new-chain rebuild 的 canonical source-of-truth。

它不保存无限原文，而是保存“下一次恢复所需最小窗口”：

1. `latest_response_id`
2. `prompt_cache_key`
3. `full replay input sequence` 或 compacted window
4. 必要的 reasoning items
5. 必要的 tool call / tool output replay context
6. `compact_cutover` 标记

### Why This Is Needed

没有 canonical rebuild window 时，系统只能依赖：

1. 当前请求体
2. 上一轮局部内存状态
3. 活跃 WS 的热上下文

这对 tool/reasoning 场景不够稳定，也无法支撑可靠 rebuild fallback。

### Storage Requirements

该窗口不能只存在进程内内存。

最低要求：

1. 本地热缓存用于低延迟读取
2. 共享缓存（GatewayCache/Redis）用于跨进程恢复
3. 共享缓存不可用时，系统只能降级成“同进程 best-effort”

若没有共享状态，就不能把该方案称为“最佳 fallback”。

### Relationship With Existing WS State Store

现有 `OpenAIWSStateStore` 保留，继续负责：

1. account sticky
2. conn sticky
3. session conn / turn state
4. strict-delta shadow context

新增 session window 负责：

1. latest response lineage
2. rebuild input window
3. compact cutover state

两者都属于 continuation state，但职责不同，不应混成单一“热缓存”。

## Performance Levers

本轮 continuation 重构只是性能优化的一部分。若目标是最佳 TTFT 和更高有效输出速率，还必须显式纳入以下杠杆：

1. `context_management.compact_threshold`
   - 优先减少超长链导致的上下文膨胀
2. WS warm-up / `generate:false`
   - 在工具密集场景下预热热连接
3. `service_tier=priority`
   - 对高价值、低延迟敏感流量显式使用优先级
4. 稳定的 `prompt_cache_key`
   - 保持会话级缓存与窗口标识稳定
5. 每会话单 active response
   - 避免单连接并发 response 拉长尾延迟

因此，“最佳 TTFT / token throughput”必须作为单独 phase 规划，而不是默认由 continuation 重构自动获得。

## Error Prevention

planner 必须在请求进入 transport 前完成以下静态规避：

1. `background=true`
   - 对 OAuth `store=false` lane 直接拒绝或切 durable lane
2. 非法 `previous_response_id`
   - 直接本地报错，不发上游
3. `function_call_output` 缺少 replay context
   - 不发上游，直接 `RejectUnsafeContinuation`
4. `reasoning` 但本地没有恢复材料
   - 不做 continuation，直接 `FullRebuild`
5. standalone compact cutover
   - 禁止继续旧 anchor
6. 明确旧分叉 continuation
   - 禁止假装成最近一轮 follow-up

目标是尽量把：

1. `previous_response_not_found`
2. unsafe tool continuation
3. invalid encrypted reasoning continuation

从“上游 4xx 后恢复”提前成“本地判定后规避”。

## Fallback Ladder

OAuth `store=false` 主 lane 的统一 ladder：

1. `HotWSIncremental`
2. `FullRebuild`
3. `RejectUnsafeContinuation`

可选 durable lane 才允许：

1. `ColdHTTPIncremental`
2. `FullRebuild`

### previous_response_not_found

目标语义：

1. 一旦命中 `previous_response_not_found`
2. 立刻废弃该 anchor
3. 不再继续“删 anchor 后只发当前 turn”
4. OAuth `store=false` 主 lane 直接改判为 `FullRebuild`
5. 用 canonical rebuild window 开新链

### Tool Continuation

对 `function_call_output`：

1. 若存在完整 replay context，可走 hot/cold continuation
2. 若不存在完整 replay context，则不能降级成“删 anchor 重发当前输出”
3. 必须 `RejectUnsafeContinuation` 或 `FullRebuild`

### Compact Cutover

对 standalone `/responses/compact`：

1. compaction 结果形成新的 rebuild source
2. 后续应从 compacted window 开新链
3. 禁止继续旧 `previous_response_id`

## Observability

新增或统一以下字段：

1. `continuation_action`
   - `hot_ws_incremental`
   - `cold_http_incremental`
   - `full_rebuild`
   - `reject_unsafe`
2. `continuation_reason`
3. `sticky_account_hit`
4. `sticky_conn_hit`
5. `session_window_hit`
6. `compact_cutover`
7. `rebuild_source`
   - `session_window`
   - `compacted_window`
   - `full_replay`
8. `continuation_lane`
   - `oauth_store_false_primary`
   - `durable_optional`

### Why It Matters

没有这层统一观测，就无法区分：

1. 真正高命中的 hot continuation
2. 被动退化到 rebuild 或 durable continuation
3. 因条件不满足而频繁重建链

这会让“尽可能增量发送”的目标无法量化。

## Config Flags

建议新增三个灰度开关：

1. `gateway.openai_ws.http_incremental_continuation_enabled`
2. `gateway.openai_ws.http_incremental_sticky_enabled`
3. `gateway.openai_ws.rebuild_fallback_enabled`

默认：

1. Phase 1 先开 `rebuild_fallback_enabled`
2. Phase 2 再评估 `http_incremental_continuation_enabled`
3. Phase 3 最后按 durable lane 灰度 `http_incremental_sticky_enabled`

## Rollout Strategy

### Phase 0

保留当前已完成的窄修复：

1. OAuth WS continuation 保持 `store=false`

### Phase 1

先建立正确 fallback 面：

1. 新增 planner
2. 新增共享 session rebuild window
3. `previous_response_not_found` 切到 `FullRebuild`
4. 暂不开放默认 HTTP cold continuation

### Phase 2

补显式 durable continuation：

1. 仅在显式 durable lane 下允许 HTTP continuation
2. durable lane 支持 same-account sticky
3. `force_http` 变成“禁止热 WS，但不伪造 persisted continuation”

### Phase 3

性能专项：

1. `context_management.compact_threshold`
2. WS warm-up / `generate:false`
3. `service_tier=priority`
4. 统一 TTFT / throughput telemetry

## Testing Strategy

必须新增以下测试面：

1. OAuth `store=false` 主 lane：失去热 WS 后直接 rebuild
2. `previous_response_not_found` -> rebuild window -> 新链
3. tool continuation 缺上下文时本地拒绝
4. standalone compact cutover 禁用旧 anchor
5. durable lane 下 HTTP continuation correctness
6. `force_http` 下 durable lane 的 same-account sticky

主要测试文件：

1. `backend/internal/service/openai_ws_protocol_forward_test.go`
2. `backend/internal/service/openai_oauth_passthrough_test.go`
3. `backend/internal/service/openai_ws_account_sticky_test.go`
4. `backend/internal/service/openai_ws_state_store_test.go`
5. 新增 planner / session window 测试文件

## Risks

最大回退风险不在 `store=false`，而在旧代码默认假设：

1. 非 WS = 不支持 continuation
2. `force_http` = 忽略 sticky
3. `previous_response_not_found` = 安全删除 anchor 重试

因此本轮不能一次性全并，必须按 phase 灰度。

## Recommendation

直接按本 spec 推进：

1. 统一 planner
2. 共享 canonical rebuild window
3. `Hot WS -> Full Rebuild` 主 ladder
4. durable lane 灰度 continuation
5. 性能专项 phase

这是在当前仓库上实现“尽可能增量发送 + 低错误 + 最佳回退 + 尽量 sticky”的最稳方案。
