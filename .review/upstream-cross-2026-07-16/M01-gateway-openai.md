# Module 1: Gateway OpenAI 核心协议

## 范围摘要
- 对比基线: `upstream/main` (`b960ec19807ea81d6d83cad63e84ad45fdf7e08a`) … `HEAD` (`eb64a5c1678eeb3dd4111e9a366e0b963f58a1e2`)；merge-base 参考 `BASE=da85cc7e…`
- 本模块为 personal-dev 相对上游分叉最深的热点之一：OpenAI Responses/Chat/WS/Compact 协议面、partial billing、compact SSE bridge、WS v2 passthrough relay、failover/`error committed` 语义
- 相对上游的主要能力差异:
  - HTTP Responses 与 passthrough 双路径完整化；流式 `response.failed` / cyber policy / soft rate-limit / TTFT watchdog
  - **Partial billing**：流式/WS 终态错误带回 `OpenAIForwardResult` 并 `RecordUsage`，避免客户端断流逃费
  - **Remote compact v2**：body-signal 提升、SSE keepalive（防代理掐断）、unary→SSE bridge、context overflow 同账号自动 compact 重试（path suffix override + failover body）
  - **WS v2**：`openai_ws_v2` caddy-style 双向 relay、passthrough/http_bridge/ctx_pool 模式路由、turn 级 `OnTurnComplete` 计费
  - **error committed**：`openAIForwardErrorAlreadyCommunicated`、compact heartbeat 200 固化后 in-band `response.failed`
  - Anthropic Messages bridge over OpenAI、Grok/xAI cache identity 与 web_search 计次

> 说明：审查环境未执行 `git diff` 命令工具，结论基于 HEAD 实现与测试对照、以及模块内明确的分叉语义注释/回归用例（#3777/#3875/#3887 等）。

## 关键路径图

```
Client
  ├─ HTTP POST /v1/responses | /v1/chat/completions | /v1/responses/compact
  │    → OpenAIGatewayHandler.Responses / ChatCompletions
  │    → (compact normalize + SSE keepalive)
  │    → SelectAccount → OpenAIGatewayService.Forward
  │         ├─ WS 上游 (protocol resolver → ws_v2 / http bridge)
  │         └─ HTTP 上游 (passthrough | transform)
  │              handleStreaming* / handleNonStreaming*
  │              → partial result + err | success result
  │    → handler: failover loop | partial RecordUsage | success RecordUsage
  │
  └─ WS ingress /v1/responses (client websocket)
       → ProxyResponsesWebSocketFromClient
            ├─ mode=passthrough → openai_ws_v2.RunEntry relay
            │     OnTurnComplete → AfterTurn → RecordUsage
            └─ mode=ctx_pool/http_bridge → openai_ws_forwarder turn loop
                 partial/terminal → AfterTurn → RecordUsage
```

**error committed 语义（期望）**  
1. 未写下游：可 failover / 可改 HTTP 状态码 JSON 错误  
2. 仅 compact keepalive 注释字节：视为未写语义响应（`OpenAICompactKeepaliveAdjustedWrittenSize`）  
3. 已写 SSE/WS 帧：禁止跨账号重放；错误必须 in-band 终止（`response.failed` / close frame）  
4. Service 已写上游终态错误后返回 err：handler 不得再 append 通用 fallback（`openAIForwardErrorAlreadyCommunicated`）

## 发现清单

### [P0] OpenAI Messages bridge：partial 错误路径只打日志、依赖 fall-through 计费且把失败报成调度成功
- **位置**: `backend/internal/handler/openai_gateway_handler.go:1209-1329`
- **相对上游**: 本地修改（Anthropic-over-OpenAI bridge 与 partial billing 加固）
- **问题**:
  1. `err != nil && result != nil` 时仅 `Warn("openai_messages.forward_partial_error_result")`，**没有**像 Responses 路径那样显式 `submitOpenAIUsageRecordTask` 后 `return`，而是落到下方“成功”分支统一 `RecordUsage`。
  2. 同一 fall-through 中无条件调用 `ReportOpenAIAccountScheduleResult(account.ID, true, …)`，把**失败 turn 记成调度成功**，污染 TTFT/失败率调度信号。
  3. fall-through **不**调用 `ensureAnthropicErrorResponse`：若 service 未写出终态错误事件，客户端拿到半截流且无错误帧。
- **影响**:
  - 调度器误判健康账户 → 粘性/权重偏向坏号，放大 429/5xx
  - 协议不完整导致客户端重试，叠加 partial 已计费 → 用户体感双重扣费/重复请求
  - 与 Responses 路径语义分叉，后续修 partial 极易只修一边
- **证据**:
```1209:1288:backend/internal/handler/openai_gateway_handler.go
		if err != nil {
			if result != nil {
				// ... cyber early return ...
				// Bill any partial result (token and/or image), not only ImageCount>0.
				reqLog.Warn("openai_messages.forward_partial_error_result",
					// ...
				)
				// 注意：这里没有 return，也没有 submitOpenAIUsageRecordTask
			} else {
				// failover / ensureAnthropicErrorResponse + return
			}
		}
		if result != nil {
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, true, result.FirstTokenMs)
		}
		// ... 无条件 RecordUsage（成功路径）...
```
  对照 Responses 正确写法（partial 显式计费 + `return`）：`openai_gateway_handler.go:598-650`  
  对照 Anthropic 主 gateway（partial 计费后仍 `ensureForwardErrorResponse`）：`gateway_handler.go:544-608`
- **建议**:
  - Messages partial 分支对齐 Responses：显式 `RecordUsage` + `ReportOpenAIAccountScheduleResult(..., false, ...)` + `ensureAnthropicErrorResponse`（若尚未 committed）+ `return`
  - 补单测：`result!=nil && err!=nil` 时计费一次、调度失败、且错误帧写出
- **交叉关注**: M04 计费；M06 调度评分

### [P0] HTTP 流式 partial 错误在 handler 侧“只计费不收口”，存在 error 未 committed 的半开流
- **位置**:
  - Handler: `backend/internal/handler/openai_gateway_handler.go:598-650`
  - Service 缺口例: `backend/internal/service/openai_gateway_service.go:10234-10246`、`8870-8880`
- **相对上游**: 本地修改（partial billing 加固引入）
- **问题**: Responses handler 在 `err != nil && result != nil` 时提交 partial usage 后**直接 return**，不再走 `openAIForwardErrorAlreadyCommunicated` / `ensureForwardErrorResponse`。这假设 service 层一定已写出协议终态。但至少以下路径返回 **billable partial + error** 且**不写** `response.failed`/`error` 事件：
  - `finalizeStream`：`clientOutputStarted==true` 且缺少 terminal → `"stream usage incomplete: missing terminal event"`
  - passthrough 同构路径：`stream usage incomplete: missing terminal event`
- **影响**:
  - Codex CLI / openai-python Responses 客户端常见 `"stream closed before response.completed"`，触发盲重连
  - 重连再次消耗上游配额并再次计费（partial 已扣 + 新请求再扣）
  - 与 compact/SSE 上“必须 in-band 终止”的设计目标直接冲突
- **证据**:
```598:650:backend/internal/handler/openai_gateway_handler.go
		if err != nil {
			if result != nil {
				// ... RecordUsage(partial) ...
				reqLog.Warn("openai.forward_partial_error_result", ...)
				return // 无 ensureForwardErrorResponse
			}
```
```10234:10246:backend/internal/service/openai_gateway_service.go
	finalizeStream := func() (*openaiStreamingResult, error) {
		if !sawTerminalEvent {
			if !openAIStreamClientOutputStarted(c, clientOutputStarted) {
				return resultWithUsage(), s.newOpenAIStreamFailoverError(...)
			}
			return resultWithUsage(), fmt.Errorf("stream usage incomplete: missing terminal event")
			// 无 sendErrorEvent / writeOpenAIResponsesFailedSSE
		}
```
- **建议**:
  - handler partial 分支在 return 前调用 `openAIForwardErrorAlreadyCommunicated`；未 committed 则 `ensureForwardErrorResponse` / `writeResponsesFailedSSE`
  - 或 service 在所有 `clientOutputStarted && err` 路径统一 `sendErrorEvent`/`response.failed`
  - 单测：上游 SSE 在 `response.created` 后 EOF → 客户端体含 `response.failed` + 仅一次 usage
- **交叉关注**: M04（重连双计费）；M03（Responses 终止事件契约）

### [P1] WS v2 passthrough 失败收口 `AfterTurn(..., result=nil)`，丢弃 relay 已解析 usage；且 `BeforeWriteClient` 在观测 usage 之前拦截
- **位置**:
  - `backend/internal/service/openai_ws_v2_passthrough_adapter.go:825-894`
  - `backend/internal/service/openai_ws_v2/passthrough_relay.go:466-492`
  - Handler 对 nil result 直接放弃计费: `openai_gateway_handler.go:2121-2124`
- **相对上游**: 本地新增（openai_ws_v2 数据面）
- **问题**:
  1. relay 失败时 adapter 已构造带 `relayResult.Usage` 的 `result`，但 `AfterTurn(turnCount+1, nil, nil, turnErr)` 传 **nil result**。
  2. Handler：`if result == nil { return }` → **漏计费**。
  3. `runUpstreamToClient` 先 `beforeWriteClient` 后 `observeUpstreamMessage`：若 early `response.failed` 触发 failover reject，该帧 usage **永不解析**（对“未写下游可重试”可接受；对“写下游后的失败帧”依赖后续 observe，顺序脆弱）。
- **影响**:
  - passthrough 长连接在 mid-turn 失败 / idle timeout / upstream read fail 时，已完成 turn 靠 `OnTurnComplete` 尚可；**当前 turn** 若已有 terminal usage 但走了失败收口，可能漏记
  - 与 HTTP/WS forwarder 的 `buildOpenAIWSPartialForwardResult` 语义不一致
- **证据**:
```825:894:backend/internal/service/openai_ws_v2_passthrough_adapter.go
	result := &OpenAIForwardResult{ /* Usage from relayResult */ ... }
	// ...
	turnErr := wrapOpenAIWSIngressTurnError(relayExit.Stage, relayErr, relayExit.WroteDownstream)
	if hooks != nil && hooks.AfterTurn != nil {
		if hooks.BeforeTurn == nil || int(startedTurns.Load()) > turnCount {
			hooks.AfterTurn(turnCount+1, nil, nil, turnErr) // result 被丢掉
		}
	}
```
```2121:2124:backend/internal/handler/openai_gateway_handler.go
				if turnErr != nil {
					if result == nil {
						return
					}
```
```466:492:backend/internal/service/openai_ws_v2/passthrough_relay.go
		if beforeWriteClient != nil {
			if err := beforeWriteClient(msgType, payload, wroteDownstream); err != nil {
				// return BEFORE observeUpstreamMessage / emitTurnComplete
				return
			}
		}
		observedEvent = observeUpstreamMessage(...)
		emitTurnComplete(...)
```
- **建议**:
  - 失败收口：`AfterTurn(..., resultOrPartial, turnErr)`，至少在 `WroteDownstream || usage>0` 时传入
  - 考虑先 observe/parse usage 再执行 failover 决策（或在 reject 路径解析 usage 填入 RelayExit）
  - 单测：response.failed + wroteDownstream / mid-stream EOF 必须触发 RecordUsage
- **交叉关注**: M04；M06（WS 池/账号失败信号）

### [P1] Context-overflow 自动 compact 的 gin context 状态在跨账号 failover 后仍粘连
- **位置**:
  - 设置: `backend/internal/service/openai_gateway_service.go:7174-7211`
  - 读取 body: `Forward` `getOpenAIFailoverRequestBody` `4764-4767`
  - 读取 path suffix: `openAIResponsesRequestPathSuffix` `12529-12536`
  - 清理: `ClearOpenAICompatRequestState` `261-265`（**不**清理 failover body / path suffix / compact stream mark）
  - Handler 换号: `openai_gateway_handler.go:738`
- **相对上游**: 本地新增（remote compaction auto-retry）
- **问题**: 同账号 context overflow 时写入：
  - `openai_responses_upstream_path_suffix_override=/compact`
  - `openai_failover_request_body=<compact body>`
  - `MarkOpenAICompactClientStream`
  
  随后若 compact 请求再遇 429 并 `UpstreamFailoverError`，handler `ClearOpenAICompatRequestState` **只清** parsed body cache + stream replay state，**不清**上述三键。下一账号 `Forward` 会强制 compact body + `/compact` 后缀。
- **影响**:
  - 测试 `TestOpenAIGatewayService_ContextCompactionRetryKeepsInboundPathAndReusesCompactOnFailover`（`openai_oauth_passthrough_test.go:2524-2542`）**有意**验证二次 Forward 仍走 compact——说明产品接受“粘连到下一账号”。
  - 风险在于：下一账号 `!AllowsOpenAICompact()`、或 compact model 映射不同、或非 Codex 客户端被 `MarkOpenAICompactClientStream` 带进 SSE bridge 语义，导致 404/协议错配/错误的 stream 合成。
  - `contextCompactionRetried` 是 **Forward 局部变量**，换号后新 Forward 若再 overflow 可能再次尝试（通常 body 已是 compact，安全阀部分有效）。
- **证据**:
```7174:7211:backend/internal/service/openai_gateway_service.go
		if !contextCompactionRetried && !isCompact && reqStream &&
			isOpenAIContextCompactionStatus(...) && isOpenAIContextWindowError(...) &&
			isOpenAIContextRemoteCompactionV2Request(c, body) && account.AllowsOpenAICompact() {
			// ...
			setOpenAIResponsesUpstreamPathSuffixOverride(c, "/compact")
			setOpenAIFailoverRequestBody(c, body)
			MarkOpenAICompactClientStream(c)
			reqStream = false
			contextCompactionRetried = true
			continue
		}
```
```261:265:backend/internal/service/openai_gateway_service.go
func ClearOpenAICompatRequestState(c *gin.Context) {
	clearOpenAIRequestBodyCache(c)
	ClearOpenAIStreamRetryReplayState(c)
}
```
- **建议**:
  - 明确产品语义并文档化：“overflow compact 状态跨账号继承”
  - 换号前校验目标账号 `AllowsOpenAICompact()`；否则清理 override 并回退原始 body
  - `ClearOpenAICompatRequestState` 扩展或新增 `ClearOpenAICompactFailoverState`，由 handler 在“不应继承”时调用
  - 单测：下一账号 `openai_compact_supported=false` 时不得再打 `/compact`
- **交叉关注**: M06 账号能力过滤；M02 若其他平台复用 suffix override

### [P1] Compact heartbeat “error committed” 与 failover 守卫整体正确，但依赖全路径 Stop+AdjustedSize，遗漏点会静默损坏 SSE
- **位置**:
  - Keepalive: `backend/internal/service/openai_compact_sse_keepalive.go:17-179`
  - Bridge: `backend/internal/service/openai_compact_stream_bridge.go:57-180`
  - Handler: `handleStreamingAwareError` / `errorResponse` / `openAIForwardErrorAlreadyCommunicated`：`openai_gateway_handler.go:2628-2813`、`726-732`、`2753-2794`
- **相对上游**: 本地新增（#3887/#3875）
- **问题**: 设计本身扎实（首拍延迟、注释心跳、AdjustedWrittenSize 排除心跳、committed 后强制 `response.failed`）。残存风险是**覆盖面**：
  - 任何未调用 `StopOpenAICompactSSEKeepaliveCommitted` / `writeOpenAICompactAware*` 的写回（中间件、panic 外路径、新加 error helper）仍可能在 200 SSE 后 `c.JSON`，污染流。
  - `openAIForwardErrorAlreadyCommunicated` 前缀匹配硬编码（`openai ws error event:` 等），新错误字符串格式漂移会双写 `response.failed`。
- **影响**: Codex 盲重连 + 重复 compact 配额消耗（正是 #3887 要修的问题的回归形态）
- **证据**:
```129:179:backend/internal/service/openai_compact_sse_keepalive.go
// StopOpenAICompactSSEKeepaliveCommitted ... 报告响应头是否已被心跳提交为 200
// OpenAICompactKeepaliveAdjustedWrittenSize ... 排除心跳字节
```
```2753:2794:backend/internal/handler/openai_gateway_handler.go
func openAIForwardErrorAlreadyCommunicated(...) bool {
	// 前缀白名单：
	// "openai ws error event:", "upstream response failed:", ...
}
```
- **建议**:
  - 错误写回统一走 `writeOpenAICompactAwareJSONError` / `handleStreamingAwareError`
  - `AlreadyCommunicated` 改为 context flag（`MarkResponseCommitted` / service 显式 `SetUpstreamErrorWritten`），避免字符串前缀
  - lint/测试：compact stream mark 下禁止裸 `c.JSON`
- **交叉关注**: 可观测性（ops stream error mark）

### [P2] Partial billing 与 failover 的边界函数正确，但 zero-usage partial 仍会写 usage log / 触发计费流水
- **位置**:
  - `openaiStreamingErrorBillsPartial`: `openai_gateway_service.go:7505-7517`
  - `buildOpenAI*PartialForwardResult`: `7439-7550`
  - Handler 无条件 RecordUsage: `openai_gateway_handler.go:617-643`
  - `RecordUsage` 对 zero token 仍落库：`openai_gateway_record_usage_*_test.go` 明确期望
- **相对上游**: 本地修改
- **问题**: 设计选择是 “terminal 非 failover 一律 partial bill”。当 usage 全 0（仅 `response.created` 后失败、或 usage 解析失败）仍异步 `RecordUsage`，产生 0 成本流水与调度副作用（403 counter reset 等在 RecordUsage 入口）。
- **影响**: 用量噪音、审计膨胀；极端情况下与 dedup 键碰撞风险（取决于 request_id 是否为空）
- **证据**: `openaiStreamingErrorBillsPartial` 只排除 `UpstreamFailoverError`；handler 不检查 `hasUsageTokens`
- **建议**: handler/service 对 “无 token/无 image/无 search” 的 partial 跳过扣费，仅记 ops 事件；或强制合成 request_id
- **交叉关注**: M04

### [P2] Web-search history strip 仅覆盖 Anthropic messages 形态，OpenAI Responses 多轮 web_search_call 历史无对等外科手术式剥离
- **位置**:
  - Anthropic: `backend/internal/service/gateway_websearch_block_filter.go:22-145`（`FilterWebSearchHistoryBlocks`，sjson 保真）
  - 调用点: `gateway_service.go` messages 路径
  - OpenAI: 仅有 search **计次**（`countOpenAISearchCalls*` / `SearchCount`），`openai_codex_transform` 保留 `web_search_call` id，无 history strip
- **相对上游**: 本地/共享能力；OpenAI 侧未见对等实现
- **问题**: TASK 关注的 “web-search history strip” 在 OpenAI 协议路径上**基本缺席**。多轮 Responses 若携带上游不能接受的 `web_search_call` / 本地仿真结果，依赖上游 400 + failover，而非像 Anthropic 那样在入站剥离。
- **影响**: OpenAI/Codex 多轮 web search 会话在部分账号/模型上稳定性差；与 Anthropic 行为不一致
- **证据**: `FilterWebSearchHistoryBlocks` 只处理 `messages[].content` 的 `server_tool_use` / `web_search_tool_result`；OpenAI input 项类型无对应 filter
- **建议**: 若产品需要，在 Responses 入站增加 `web_search_call` 历史清理（注意 encrypted/reasoning 与 tool 配对）；否则在文档标明 “仅 Anthropic messages 支持”
- **交叉关注**: M03 apicompat；M02 Grok web_search

### [P2] 架构与分叉成本：OpenAI 协议状态机高度集中且双实现
- **位置**:
  - `openai_gateway_service.go`（1 万+ 行级聚合：Forward、stream、passthrough、compact、usage）
  - `openai_ws_forwarder.go` + `openai_ws_v2/*` 双数据面
  - Handler 内 Responses / Messages / WS 三套 failover 循环
- **相对上游**: 本地大幅扩展
- **问题**: 同名概念多套实现（`handleStreamingResponse` vs `handleStreamingResponsePassthrough` vs WS v1 vs WS v2），partial/error-committed/cyber 修复需要四处同步；与上游合并冲突面积极大。
- **影响**: 回归成本高；本次发现的 Messages vs Responses partial 不一致即是症状
- **建议**: 抽取统一 `StreamTerminalPolicy{BillPartial, WriteFailed, AllowFailover}`；WS v1/v2 共享 partial 构造
- **交叉关注**: 全局可维护性；M10 CI 是否有协议 golden

### [P3] `openAIForwardErrorAlreadyCommunicated` / compact 路径注释与测试较完整，属正向资产
- **位置**: `openai_gateway_handler.go:2753+`；`openai_compact_*_test.go`；`openai_gateway_partial_billing_test.go`；`openai_oauth_passthrough_test.go` compaction 用例
- **相对上游**: 本地新增
- **问题**: 非缺陷；记录为可合并的优质防护。但仍依赖人工记得扩展前缀列表（见 P1）。
- **影响**: 无直接负面；降低 #3887 类事故再现率
- **证据**: 单测覆盖 heartbeat adjusted size、body-signal remote v2 不提升 path、compaction failover 复用 body
- **建议**: 保持；将字符串前缀升级为 flag 后保留测试
- **交叉关注**: 无

### [P3] Grok Free-tier 注入 `web_search`+`tool_choice=none` 的 cache 路由技巧有语义副作用面
- **位置**: `backend/internal/service/openai_gateway_grok_cache.go:116-151`
- **相对上游**: 本地/平台特化
- **问题**: 为走 cache-capable tier，对无 tools 请求注入 native search tools 且 `tool_choice=none`。意图正确，但依赖 “intentSourceBody 仍可见原 tools” 防误伤；若上游未来忽略 `tool_choice=none` 会误触发 search 计费。
- **影响**: 低概率计费/行为漂移
- **证据**: `applyGrokResponsesCacheIdentity` 注释与 `intentSourceBody` 检查
- **建议**: 监控 SearchCount 异常；上游契约变更时加探测
- **交叉关注**: M02 Grok；M04 search 计费

## 与上游合并风险
- **冲突热点文件**:
  - `backend/internal/service/openai_gateway_service.go`
  - `backend/internal/service/openai_ws_forwarder.go`
  - `backend/internal/handler/openai_gateway_handler.go`
  - `backend/internal/service/openai_ws_v2/**`（上游可能完全没有或结构不同）
  - compact 三件套：`openai_compact_sse_keepalive.go` / `openai_compact_stream_bridge.go` / `openai_auto_compaction.go`
- **语义漂移点（同名不同义）**:
  - `ClearOpenAICompatRequestState`：名字像“清全部兼容态”，实际**不**清 compact failover body/path override
  - `ReportOpenAIAccountScheduleResult(..., true)`：Messages partial 失败仍传 true
  - `Forward` 的 `result!=nil && err!=nil`：表示 “已 committed 的可计费失败”，不是 “可重试中间态”；handler 必须区分 failover vs partial
  - `remote_compaction_v2`：handler 侧 body-signal **不**改 path；service 侧 overflow **改** suffix override——两套 compact 入口
  - `openaiStreamingErrorBillsPartial`：凡非 `UpstreamFailoverError` 都 bill，包括协议不完整

## 测试与验证缺口
- **缺失/不足**:
  1. Messages bridge：`result!=nil && err!=nil` 的计费 + 错误帧 + 调度结果 集成测试
  2. Responses：上游 SSE 在 preamble 后 EOF → 必须有 `response.failed` + 单次 usage
  3. WS v2：`AfterTurn` 失败路径携带 partial usage；`BeforeWriteClient` reject 与 usage 解析顺序
  4. Compact failover → 目标账号不支持 compact 的负例
  5. `openAIForwardErrorAlreadyCommunicated` 对新错误格式的回归（flag 化后）
- **已有较强覆盖**（应保留）:
  - partial builder 单测：`openai_gateway_partial_billing_test.go`
  - compact keepalive / body-signal / overflow retry：`openai_compact_*`、`openai_oauth_passthrough_test.go`
  - stream failed passthrough：`openai_gateway_response_failed_passthrough_test.go`
  - WS v2 relay 基础：`openai_ws_v2/passthrough_relay_*_test.go`（但缺 billing 挂钩）

## 模块结论
- **整体风险评级: High**
  - 未发现明显的“未鉴权即可打上游”类 P0 安全洞；核心风险在 **计费×协议终态×failover** 三角一致性。
  - 两处 P0 均可在生产造成：调度误判、半开流盲重连、重连叠加 partial 扣费。
- **是否建议合入上游 / 继续分叉 / 先修再合**:
  - **先修再合**（至少修 P0×2 与 WS v2 partial P1）。
  - 即使修完，OpenAI 协议面与上游分叉过深，宜 **继续分叉** 并用模块边界（shared terminal policy）降低回灌成本，不宜期待干净一次性合入。
- **Top 3 必须处理项**:
  1. **(P0)** Responses/Messages partial 错误路径：计费后必须保证协议终态 committed（或明确 AlreadyCommunicated），禁止半开流 return
  2. **(P0)** Messages bridge：partial 不得 `ReportOpenAIAccountScheduleResult(success=true)`；对齐 Responses 控制流
  3. **(P1)** WS v2 passthrough 失败 `AfterTurn` 传入 partial result；compact failover 状态机对账号能力做校验/清理
