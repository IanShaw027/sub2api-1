# Module 3: API 兼容层 apicompat

## 范围摘要
- **对比基线**: BASE=`da85cc7e` … HEAD=`eb64a5c1`；UPSTREAM=`b960ec19`（`upstream/main`）
- **路径焦点**: `backend/internal/pkg/apicompat/**`（~40 源/测文件 + `testdata/openai_claude_compat/`），只读参考 service 接缝：
  - `openai_gateway_messages.go`（Anthropic Messages ↔ Responses）
  - `gateway_forward_as_chat_completions.go` / `openai_gateway_forward_cc_to_messages.go`（CC → Responses → Anthropic 双跳）
  - `gateway_forward_as_responses.go` / `openai_gateway_responses_chat_fallback.go` / `gateway_forward_openai_compat_cc.go`（Responses ↔ CC）
  - `openai_responses_namespace.go`（namespace flatten/restore）
  - `gemini_chat_completions_compat_service.go`（Gemini 上同样走双跳链）
- **相对上游的主要能力差异**（本分支为协议兼容热点，远大于上游精简实现）:
  - 完整 Anthropic Messages ↔ OpenAI Responses 双向（请求/响应/流式状态机）
  - Chat Completions ↔ Responses bridge（含 DeepSeek tool pairing 修复、custom/tool_search、namespace flatten）
  - 生产双跳链：`ChatCompletionsToResponses` → `ResponsesToAnthropicRequest`（及反向 `AnthropicToResponsesResponse` → `ResponsesToChatCompletions`）
  - Codex namespace 工具摊平/还原、`additional_tools` 提取、stream wire Marshal 强制字段
  - cache usage 多别名反序列化 + Anthropic/Responses 语义互转
  - golden path fixtures（`tool_name_round_trip` / `refusal_envelope`）+ 大量 unit tests

## 关键路径图

```
[客户端协议]                    [apicompat]                         [上游]
─────────────                   ──────────                         ─────
POST /v1/messages ──AnthropicToResponsesForModel──► Responses ──► OpenAI
                 ◄──ResponsesToAnthropic / StreamState────────────

POST /v1/responses ──ResponsesToAnthropicRequest──► Messages ──► Anthropic
                  ◄──AnthropicToResponsesResponse / Stream─────

POST /v1/chat/completions
  ├─ ChatCompletionsToResponses ──► Responses ──► OpenAI
  └─ ChatCompletionsToResponses
       → ResponsesToAnthropicRequest ──► Messages ──► Anthropic/Kiro/Gemini
     回程: AnthropicToResponsesResponse → ResponsesToChatCompletions
       或 AnthropicEventToCCChunks（直转，丢 cache_creation）

POST /v1/responses (CC fallback)
  ──ResponsesToChatCompletionsRequest──► Chat ──► DeepSeek/GLM…
  ◄──ChatCompletionsChunkToResponsesEvents / Finalize──

Namespace (OAuth HTTP): FlattenResponsesNamespacesExcept → upstream
                        RestoreResponsesNamespaceCalls ← downstream
```

协议状态机核心：
1. **请求**: 消息/role 交替、tool_use↔tool_result 邻接、namespace 扁平名、call_id 前缀兼容
2. **流式**: created → output_item.added → deltas → *.done → completed/incomplete；缺事件由 Finalize* 合成
3. **用量**: Anthropic `input` 不含 cache；Responses/CC `input/prompt` 含 cache，转换时加减

## 发现清单

### [P1] Chat→Responses 终态 `response.output` 的 reasoning item ID 与流式事件不一致
- **位置**: `backend/internal/pkg/apicompat/chatcompletions_responses_bridge.go:1555-1565`（`chatOutput`）对比 `1287-1338`（`ensureChatReasoningItem` / `closeChatReasoningItem`）
- **相对上游**: 本地新增（CC bridge 为本地大块）
- **问题**: 流式路径用 `state.ReasoningItemID` 打开/关闭 reasoning item；但 `FinalizeChatCompletionsResponsesStream` 写入 `response.completed.response.output` 时，`chatOutput()` 对 reasoning 调用 `generateItemID()` **重新生成** ID。message 用了 `nonEmpty(state.MessageItemID, …)`，tool 用 `toolCallItemID`，唯独 reasoning 不一致。
- **影响**: Codex/严格客户端若用 `item_id` 关联流式 delta 与最终 `output[]`，会出现“流里有 rs_xxx、终态是另一个 id”的幽灵 item；多轮 `previous_response_id`/本地历史拼接时可能丢 reasoning 或重复。
- **证据**:
```go
// stream open
state.ReasoningItemID = generateItemID()
// final output
outputs = append(outputs, ResponsesOutput{
    Type: "reasoning",
    ID:   generateItemID(), // 新 ID，非 state.ReasoningItemID
    ...
})
```
- **建议**: `chatOutput` 中 reasoning/message/tool 一律复用 stream 生命周期 ID（`state.ReasoningItemID` / `MessageItemID` / `ToolItemIDs`）；加 stream lifecycle 断言：`completed.output[i].id == 对应 output_item.added.item.id`。
- **交叉关注**: M01 Gateway OpenAI 核心（CC fallback / WS 出口消费 `response.completed`）

### [P1] Responses→Anthropic **请求**路径丢弃 `custom_tool_call` / `tool_search_call` 历史
- **位置**: `backend/internal/pkg/apicompat/responses_to_anthropic_request.go:130-219`（`convertResponsesInputToAnthropic`）
- **相对上游**: 本地新增/扩展
- **问题**: 响应侧 `ResponsesToAnthropic` / 流式 `resToAnthHandleOutputItemAdded` 已映射 `custom_tool_call`、`tool_search_call` → Anthropic `tool_use`（有单测）。但**请求**转换 `switch` 只处理 `function_call` / `function_call_output` / `web_search_call` + role messages。Codex 多轮历史中的 `custom_tool_call`（exec/apply_patch）与 `tool_search_call` 落入 `default`：无 `Content` 则整项丢弃。
- **影响**: 客户端走 Responses、上游是 Anthropic 平台组（`gateway_forward_as_responses.go` → `ResponsesToAnthropicRequest`）时，多轮 tool 历史被截断 → 上游 400 pairing 失败，或模型“失忆”已执行的 patch/exec。CC 双跳链若历史经 Responses 中间态同样受影响。
- **证据**:
  - 请求 switch 无 `custom_tool_call` / `tool_search_call` 分支（全文 grep `responses_to_anthropic_request.go` 无匹配）
  - 响应侧有完整映射：`responses_to_anthropic.go:83-96` 与 `responses_to_anthropic_stream_test.go` custom/tool_search 用例
  - 生产接缝：`gateway_forward_as_responses.go:67` 调用 `ResponsesToAnthropicRequest`
- **建议**: 与响应侧对称：`custom_tool_call` → `tool_use`（`input` 用 freeform JSON 编码）、`tool_search_call` → `tool_use` name=`tool_search`；output 侧对应 `custom_tool_call_output` / `tool_search_output` 若 wire 存在则一并处理。补 CC chain / Responses request 单测。
- **交叉关注**: M01（Responses 入口）、M02（Anthropic/Kiro 上游）

### [P1] Anthropic↔CC 直转路径丢失 `cache_creation`（计费/用量语义）
- **位置**:
  - `backend/internal/pkg/apicompat/cc_to_anthropic_response.go:443-460`（`CcUsageToAnthropic`）
  - `backend/internal/pkg/apicompat/anthropic_to_cc_response.go:143-191`（`BuildUsageChunk` / `BuildNonStreamingResponse`）
- **相对上游**: 本地新增
- **问题**:
  1. `CcUsageToAnthropic` 只减 `CachedTokens` 得 `input_tokens`，**不读** `CacheWriteTokens` / `CacheCreationTokens`，故 `CacheCreationInputTokens` 恒为 0。
  2. `AnthropicEventToCCChunks` 的 usage 回写只映射 `Input/Output/CacheRead`，**不映射** `CacheCreationInputTokens`；`BuildNonStreamingResponse` 甚至连 `PromptTokensDetails.CachedTokens` 都省略。
  3. 对比：`anthropicUsageFromResponsesUsage` / `AnthropicToResponsesResponse` 已正确做 cache 加减并有 `responses_anthropic_cache_creation_test.go`。
- **影响**: 生产路径 `gateway_forward_messages_to_cc.go`（`CcUsageToAnthropic`）、`openai_gateway_forward_cc_to_messages.go`（`AnthropicEventToCCChunks`+`BuildUsageChunk`）上报用量时 **漏计 cache write**。若计费层依赖 Anthropic 语义的 `cache_creation_input_tokens` 或 CC 的 `cache_write`，会出现漏计/低估；与 Responses 路径用量不一致，难排查。
- **证据**:
```go
// CcUsageToAnthropic — 无 cache creation
inputTokens := u.PromptTokens - cacheReadTokens
au := &AnthropicUsage{
    InputTokens: inputTokens, OutputTokens: u.CompletionTokens,
    CacheReadInputTokens: cacheReadTokens,
}
// BuildUsageChunk — 无 CacheCreation
chunk.Usage = &ChatUsage{
    PromptTokens: s.Usage.InputTokens, // Anthropic 语义，未加回 cache
    ...
}
```
- **建议**: 与 `anthropicUsageFromResponsesUsage` / `ChatUsageToResponsesUsage` 对齐：CC→Anthropic 时 `input = prompt - cache_read - cache_write`，写出 `CacheCreationInputTokens`；Anthropic→CC 时 `prompt = input + cache_read + cache_creation`，填 `PromptTokensDetails`。补 round-trip 单测。
- **交叉关注**: M04 计费支付用量（用量字段是计费输入）

### [P1] 生产双跳链（CC→Responses→Anthropic）固有信息损失 + 无字段丢失可观测性
- **位置**:
  - 接缝：`gateway_forward_as_chat_completions.go:52-61`、`openai_gateway_forward_cc_to_messages.go:39-47`、`gemini_chat_completions_compat_service.go:45-50`
  - 转换：`chatcompletions_to_responses.go:57-136`、`responses_to_anthropic_request.go:13-70`
- **相对上游**: 本地架构选择（中转 Responses 再转 Anthropic）
- **问题**: 双跳把 Chat Completions 先变成 Responses 中间态再变 Anthropic。中间态类型/映射**未覆盖**若干客户端字段，且 `DroppedCompatibilityFields` 仅在 `AnthropicToResponses*` 侧使用：
  - `ChatCompletionsRequest.Stop` → `ChatCompletionsToResponses` **完全不映射** → Anthropic `stop_sequences` 丢失
  - CC `stream` 被强制 `Stream: true`（有意），但客户端非流式语义靠回程重组，中间失败面更大
  - `ResponsesToAnthropicRequest` 不透传 Responses 的 `Text`/`response_format`、`PreviousResponseID`、`PromptCacheKey`（后两者本就该由 service 层管，但 Text format 会静默丢）
  - assistant `thinking` 在 `anthropicAssistantToResponses` 被显式忽略；CC 侧 reasoning 经 `<thinking>` 文本包裹，再经 Anthropic 请求时不会变成 thinking block
- **影响**: 客户端以为发了 stop/json_schema/reasoning 约束，上游实际未收到 → 难复现的行为漂移。Gemini/Kiro/Anthropic 平台组上所有 CC 客户端共用此链，blast radius 大。
- **证据**: `ChatCompletionsToResponses` 返回体无 `Stop` 字段处理；`ResponsesToAnthropicRequest` 无 `Text`/`StopSeqs`；生产注释明确 “chained conversion”。
- **建议**:
  1. 优先考虑生产路径改走 `AnthropicRequestToChatCompletions` 的**逆**方向专用映射（减少一跳），或
  2. 在双跳各 hop 填充 `DroppedCompatibilityFields` 并接入已有 `recordOpenAICompatDroppedFields` 可观测性；
  3. 至少补：`stop` → `stop_sequences`、`response_format` 降级策略文档化。
- **交叉关注**: M01、M02、M04（用量）、M06（无直接）

### [P2] call_id 前缀策略三处不对称，存在历史/跨协议 pairing 风险
- **位置**:
  - `anthropic_to_responses.go:493-511`（`toResponsesCallID`/`fromResponsesCallID`）
  - `responses_to_anthropic_request.go:573-586`（`fromResponsesCallIDToAnthropic`）
  - `responses_to_anthropic.go:79` 响应侧用 `fromResponsesCallID`（非 ToAnthropic 版）
- **相对上游**: 本地演进（注释写明从 `fc_` 前缀桥切到保留 Anthropic ID）
- **问题**:
  | 函数 | 剥 `fc_` | 处理 `srvtoolu_` | 裸 ID 合成 `toolu_` |
  |------|---------|------------------|---------------------|
  | `fromResponsesCallID` | 仅当 remainder 为 `toolu_`/`call_` | 否 | 否 |
  | `fromResponsesCallIDToAnthropic` | 含 `srvtoolu_` | 是 | 是 |
  | 响应 `ResponsesToAnthropic` | 用前者 | — | — |

  结果：
  - 请求历史 `call_id="abc"` → Anthropic `toolu_abc`；若模型回 `toolu_abc` 再经 `toResponsesCallID` 保留，客户端看到的 ID 已变。
  - 遗留 `fc_srvtoolu_x` 在 Responses 往返中**不会**被 `fromResponsesCallID` 剥掉，但 ToAnthropic 会剥。
  - 响应路径与请求路径对同一 call_id 规范化不一致。
- **影响**: 多轮 tool 续聊、failover 换账户、Claude Code 回放 transcript 时偶发 “tool_result 找不到 tool_use”。现有单测覆盖了 `fc_toolu_123` 剥离（`anthropic_responses_test.go:2035`），未覆盖 `srvtoolu_` / 裸 ID 合成的全链路 round-trip。
- **建议**: 收敛为**单一** `normalizeToolCallID(direction)`；响应/请求共用；对合成前缀做可逆标记或禁止合成（要求上游/客户端使用合法前缀）。补 golden round-trip。
- **交叉关注**: M01、M02

### [P2] Stream 终态 `chatOutput` 在“仅 tool、无文本”时仍可能插入空 message（边界）
- **位置**: `chatcompletions_responses_bridge.go:1567-1578`
- **相对上游**: 本地
- **问题**: 条件 `if state.MessageItemID != "" || len(state.ToolCalls) == 0`：当**有** tool calls 且从未打开 message（`MessageItemID==""`）时**不**插入 message——正确。但若上游先推了空 content 导致 `ensureChatToResponsesMessageItem` 已分配 ID 再出 tool，终态会带空 `output_text` message + function_call，与部分上游/Codex 期望的 “tool-only output” 形状不一致。
- **影响**: 中等；多数客户端可忽略空 text，但 strict schema 校验或 UI 计数可能多一个 item。
- **证据**: `ensureChatToResponsesMessageItem` 在首个非空 content 时分配；空 content 不分配。风险主要在“空字符串 content delta 是否触发”的上游差异。
- **建议**: tool-only 完成时若 `Text.Len()==0` 且曾误开 message，Finalize 时从 output 剥离空 message item。
- **交叉关注**: M01

### [P2] Golden fixtures 覆盖面与真实协议一致性不足
- **位置**: `anthropic_responses_golden_test.go` + `testdata/openai_claude_compat/{tool_name_round_trip,refusal_envelope}/`
- **相对上游**: 本地
- **问题**:
  - 仅 **2** 个 fixture，且 expectation 是 `equals_paths` 稀疏断言，不是完整 wire snapshot。
  - 只测 `ResponsesToAnthropic` 非流式；**无**请求转换、**无**流式 SSE 序列、**无** CC bridge、**无** namespace restore 的 golden。
  - `tool_name_round_trip` 验证 `__ReadFile` 名称映射与 `call_1` 保留——有价值，但不覆盖 `fc_` 剥离、cache usage 分解、custom_tool 等。
  - `refusal_envelope` 验证 failed+content_filter → stop_reason/usage 形状，但 `usage.cache_creation_input_tokens` 未在 equals 中断言（fixture 上游 usage 也无 creation）。
- **影响**: 协议回归主要靠分散 unit tests；合并上游/重构状态机时易漏边角，且与真实 Claude/Codex 抓包缺少可 diff 的契约。
- **建议**: 扩展 golden：① 流式 event 序列（至少 tool_use + text）；② `ResponsesToAnthropicRequest` pairing；③ cache_creation round-trip；④ custom_tool_call。期望可逐步从 path-equals 升到有意 omit 的完整 JSON。
- **交叉关注**: 全模块测试债

### [P2] Namespace 摊平存在双实现，命名策略可能漂移
- **位置**:
  - Map 路径：`responses_namespace.go`（`FlattenResponsesNamespacesExcept` / `RestoreResponsesNamespaceCalls`）— service HTTP 入口用
  - Struct 路径：`chatcompletions_responses_bridge_ug20.go` + `responsesToolsToChatTools` — CC bridge 用
  - Anthropic 工具：`convertResponsesToAnthropicTools` 内再 flatten 一次
- **相对上游**: 本地
- **问题**: 三处都调用 `flattenNamespaceToolName` / `joinNamespace`，但冲突检测、preserved namespace（`image_gen`）、`tool_choice` 改写、input 重写范围不完全相同。例如 Flatten 会改写 `input` 里 `function_call` 的 namespace 字段；CC bridge 在 `buildChatMessagesFromItems` 用 `responseFunctionChatName`；Anthropic tools 路径 flatten 名称但不改写 history 中的 namespace 调用（history 走 `function_call` name 字段，若客户端已扁平则 OK，若仍带 namespace 字段则 `ResponsesInputItem` **无 Namespace 字段**——`types.go:321-338` 的 InputItem 没有 `Namespace`！）。
- **影响**: 带 `namespace` 的 `function_call` 经 `json.Unmarshal` 到 `ResponsesInputItem` 时 **namespace 被静默丢弃**，`convertResponsesInputToAnthropic` 只看到短名 `name`，与 tools 里 flatten 后的长名不一致 → Anthropic 侧 tool 名与 call 名错配。
- **证据**: `ResponsesInputItem` 无 `Namespace`/`namespace` 字段；`ResponsesOutput` 有 `Namespace`（`types.go:442`）。请求/响应类型不对称。
- **建议**: 给 `ResponsesInputItem` 增加 `Namespace`；请求转换时 `name = flatten(namespace, name)` 或保留结构化 server tool；统一只保留一条 flatten 实现。
- **交叉关注**: M01 `openai_responses_namespace.go`

### [P2] `AnthropicToResponses` 丢弃 assistant thinking / cache_control 工具位，仅 sidecar 记录部分丢失
- **位置**: `anthropic_to_responses.go:20-45, 441-444, 115-122`
- **相对上游**: 本地
- **问题**: `thinking` blocks 在 assistant→input 时被忽略（注释：OpenAI 不接受）；tools 上的 `cache_control` 无法映射时记入 `DroppedCompatibilityFields`，但 assistant thinking **不**记入 sidecar。`supportsResponsesPromptCacheBreakpoints` 仅 gpt-5.6+，旧模型全部 drop cache breakpoints。
- **影响**: Claude Code → OpenAI 兼容路径多轮时丢失内部推理上下文；cache 命中率下降（有 Dropped 指标尚可观测）。thinking 丢失无指标。
- **建议**: thinking 可降级为 `instructions` 附录或 encrypted reasoning 回传（若 `include` 已要 `reasoning.encrypted_content`）；至少记 `DroppedCompatibilityFields: ["thinking"]`。
- **交叉关注**: M01 prompt cache / continuation

### [P3] `ResponsesToChatCompletions` 静默吞掉 `web_search_call`
- **位置**: `responses_to_chatcompletions.go:79-81`
- **相对上游**: 本地
- **问题**: 注释 “silently consumed”；Chat 协议无对等 server tool 项时合理，但客户端无法得知发生过搜索（Anthropic 路径会合成 `server_tool_use`）。
- **影响**: 低；功能可用，可观测性差。
- **建议**: 可选映射为注释性 system/tool 消息或 usage 元数据。
- **交叉关注**: M01 web-search strip

### [P3] `isReasoningModel` 前缀匹配过宽
- **位置**: `anthropic_to_responses.go:437-438`
- **相对上游**: 本地
- **问题**: `strings.HasPrefix(model, "gpt-5")` 会匹配未来非 reasoning 的 `gpt-50-...` 或自定义名；导致 temperature/top_p 被剥离。
- **影响**: 低/前瞻。
- **建议**: 与 `supportsResponsesPromptCacheBreakpoints` 一样做版本解析，或显式 allowlist。
- **交叉关注**: 无

## 与上游合并风险
- **冲突热点文件**:
  - `backend/internal/pkg/apicompat/*` 整体（上游若仍薄封装，本分支是完整协议栈，合并几乎必冲突）
  - `chatcompletions_responses_bridge.go` / `responses_to_anthropic*.go` / `types.go`（类型字段增删影响面最大）
  - service 接缝：`openai_gateway_messages.go`、`gateway_forward_as_*.go`、`openai_responses_namespace.go`
- **语义漂移点（同名不同义）**:
  - `input_tokens`：Anthropic 不含 cache vs Responses/CC 含 cache；`anthropicUsageFromResponsesUsage` 与 `CcUsageToAnthropic` **行为不一致**（后者漏 creation）
  - `call_id` / `tool_use.id`：本地曾用 `fc_` 前缀，现“保留原样 + 剥离遗留”；与上游若仍加前缀则 transcript 不兼容
  - `tool_search` / `custom`：本地有代理 schema 与 freeform 编码；上游可能直接拒绝
  - namespace 扁平名 `ns__child` vs `ns.child`（含 `.` 时走 joinNamespace）——合并时若上游用另一种规则会破坏 restore
- **分叉成本**: **高**。apicompat 是 personal-dev 的协议护城河；合入上游需整包移植 + 接缝改造，不宜逐文件 cherry-pick。

## 测试与验证缺口
- **已有强项**:
  - tool pairing（Responses→Anthropic + CC chain）覆盖 orphan/parallel/developer 插入
  - stream lifecycle（reasoning 先 open、tool finalize、wire 必选字段）
  - cache_creation 在 Responses↔Anthropic 路径有专用测试
  - custom/tool_search 在**响应/流式**侧有测试
- **缺口**:
  1. `custom_tool_call` **请求**历史 → Anthropic 的转换与 pairing（对应 P1）
  2. `chatOutput` item id 与 stream 一致性（对应 P1）
  3. `CcUsageToAnthropic` / `BuildUsageChunk` cache_creation round-trip（对应 P1）
  4. `ResponsesInputItem.namespace` 丢弃后的 tool 名错配
  5. 双跳链 stop/response_format 字段保留的契约测试
  6. Golden 仅 2 条 path-equals；无流式/请求 golden
  7. 与真实 Codex/Claude Code 抓包的端到端对比（超出 unit 范围，建议 fixtures 来自生产脱敏流量）

## 模块结论
- **整体风险评级**: **High**
  - 非资金直接 P0，但协议兼容层处于所有跨平台转发的中心；P1 项会导致生产 400、tool 失忆、cache 用量偏差，且双跳链放大损失。
- **是否建议合入上游 / 继续分叉 / 先修再合**:
  - **先修再合**（优先修 P1：终态 ID、custom 请求历史、CC usage cache_creation；并补 namespace InputItem 字段）。
  - 合入上游应以 **整包 apicompat + service 接缝** 为单元，不建议碎片合并。
  - 在修复前 **继续分叉** 可维持 Codex/Claude 兼容能力，但分叉成本与测试债会继续累积。
- **Top 3 必须处理项**:
  1. **统一并修复 call/item 生命周期 ID**：`chatOutput` 复用 stream item id；收敛 `fromResponsesCallID*` 为一套可逆规则。
  2. **补齐 Responses→Anthropic 请求对 custom/tool_search/namespace 历史的映射**，避免多轮 Codex 工具链在 Anthropic 上游被截断。
  3. **对齐所有路径的 cache usage 语义**（尤其 `CcUsageToAnthropic` / Anthropic→CC），并与 M04 计费输入核对，防止 cache write 漏计。
