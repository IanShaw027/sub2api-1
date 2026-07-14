# Grok OAuth HTTP Active Delta Design

## Goal

为 **Grok OAuth** 上游 HTTP Responses 路径建立与 OpenAI HTTP active-delta 对齐的「安全增量」能力：

1. **默认可增量**：续聊时只向上游发送 new input items + `previous_response_id`
2. **安全优先**：本地无法证明前缀安全时，必须全量上传，并主动去掉不可信 `previous_response_id`
3. **失败可恢复**：上游续聊失败时，在**尚未写入客户端响应**的前提下，同账号 full-replay 一次
4. **单一出口**：所有 Grok OAuth 文本 Responses 上游调用共用同一 active-delta 管道
5. **范围收窄**：仅 Grok OAuth；apikey 与其它平台不动

## Best-Practice Principles

实现必须遵守以下原则（来自 OpenAI HTTP/WS active-delta 与本轮设计评审）：

| # | Principle | Rule |
|---|-----------|------|
| P1 | Fail closed | 任一安全条件不满足 → Full，禁止启发式裁剪 |
| P2 | Single egress | 所有 Grok OAuth Responses 上游流量只经 `doGrokResponsesUpstream` |
| P3 | One body view | bind 与 evaluate 使用 **同一** `canonicalBody`（patch + cache identity 之后、delta 之前） |
| P4 | Session key homology | active-delta 的 `sessionHash` 与 Grok cache identity **同源**，禁止两套 seed |
| P5 | Strip unsafe previous | Full 路径主动删除不可信 `previous_response_id`（危险 tool-output 除外） |
| P6 | Unwritten retry only | full-replay 仅当 `!c.Writer.Written()` 且未开始向客户端透传 SSE |
| P7 | Independent feature flag | Grok mutation 只看 Grok 开关，不绑 OpenAI `HTTPIncrementalContinuation` / 不静默绑死 OpenAI active-delta admin 关断 |
| P8 | Probe before assuming store | `store` 语义以 Task 0 探针结果写入 Decision Log；禁止未验证就假设与 OpenAI 完全一致 |
| P9 | Best-effort multi-instance | 进程内/非共享 state 时命中率下降只导致更多 Full，不得导致错误增量 |
| P10 | No second hash dialect | 不修改 OpenAI strict-delta 哈希语义；Grok 字段不兼容时 Full，另开评审再扩 denylist |

## Scope

### In scope（Phase 1 必须）

1. **统一出口** `doGrokResponsesUpstream`（新建），承接：
   - active-delta build / release
   - DoWithTLS + compaction retry + previous full-replay
   - response account bind + session context bind
2. 所有 Grok OAuth 文本 Responses 调用改走该出口，至少包括：
   - `forwardGrokResponses` / `forwardGrokResponsesWithPromptCacheKey`
   - `ForwardAsAnthropic` 中 Grok 分支（Claude `/v1/messages`）
   - `forwardGrokChatCompletionsViaResponses`（Chat bridge）
3. Session context（`connID="http"`）+ strict-delta 判定 + payload 改写
4. previous_response 失败的同账号 full-replay（unwritten only）
5. 配置开关、可观测、单测矩阵（含 bridge 路径）

### Out of scope

1. Grok apikey 账号
2. OpenAI / Anthropic / Gemini / Kiro 等其它平台 continuation 改造
3. 新增数据库表或 migration
4. 对外 API 协议新增字段
5. 改变 usage / 计费口径
6. 图/视频 Imagine 路径
7. 修改 OpenAI strict-delta 哈希/volatile 规则（除非单独评审）

## Problem Statement

xAI Responses 官方支持有状态续聊：`previous_response_id` + 仅 append 新消息，无需每次重发 full conversation。

本仓库 Grok HTTP 路径当前行为：

1. **透传客户端 body**，不主动裁剪 input
2. `patchGrokResponsesBody` 在无 `previous_response_id` 时默认强制 `store=false`
3. 仅有 `response_id → account` 粘连，**没有** session input 哈希上下文
4. Claude Code / Codex 等客户端常见「每轮全量 turns」
5. **多出口**：`/v1/responses`、Claude messages、Chat bridge 各自 build request，无法一致应用增量

OpenAI 侧已有成熟实现：`openai_ws_delta_shadow.go` + `openai_http_active_delta.go`。  
Grok OAuth 应 **复用同一安全内核**，并用 **单一出口** 挂载，而不是另写裁剪启发式或只改一条路径。

## Design Summary

采用 **方案 A（修订）**：复用 OpenAI strict-delta 内核 + Grok 专属 gate/session + **强制单一 Responses 出口**。

| 结果 | 上游 payload |
|------|----------------|
| Safe incremental | `input = only-new-items`，`previous_response_id = cached.lastResponseID`，`store` 按探针策略 |
| Full | 完整 `input`，**删除** `previous_response_id`（见 P5 例外），`store` 按探针策略 |

本地判定失败只产生 Full，禁止「猜最后 N 条」。

## Architecture

```
client body (often full history)
        │
        ▼
path-specific convert (Responses | Claude→Responses | Chat→Responses)
        │
        ▼
reject unsupported tools
patchGrokResponsesBody
resolveGrokCacheIdentity + applyGrokResponsesCacheIdentity
        │
        ▼
canonicalBody  ←── 唯一 body 视图（P3）
sessionHash    ←── 与 cache identity 同源（P4）
        │
        ▼
┌─────────────────────────────────────┐
│     doGrokResponsesUpstream         │  ← 唯一上游出口（P2）
│  buildGrokHTTPActiveDeltaPayload    │
│    ├─ applied  → delta body         │
│    └─ else     → full + strip prev  │
│  buildGrokResponsesRequest          │
│  DoWithTLS                          │
│  retries:                           │
│    1) compaction (existing)         │
│    2) previous full-replay if       │
│       unwritten && (applied \|\|     │
│       had_previous_stripped_path)   │
│  on success:                        │
│    bindHTTPResponseAccount          │
│    bindGrokHTTPResponseSessionContext│
│      (hashes from canonicalBody)    │
└─────────────────────────────────────┘
```

### Components

#### 1. `doGrokResponsesUpstream`（必须）

签名语义（实现可微调）：

```text
doGrokResponsesUpstream(ctx, c, account, canonicalBody, originalModel, reqStream, startTime) (*OpenAIForwardResult, error)
```

职责：

- active-delta / full 决策
- HTTP 调用与重试
- bind account + session context
- 释放 session in-flight

**禁止** 在 Claude/Chat 路径再次直接 `buildGrokResponsesRequest` + `DoWithTLS` 绕过本函数。

#### 2. `buildGrokHTTPActiveDeltaPayload`

- Gate：`PlatformGrok && AccountTypeOAuth && gateway.grok.http_active_delta_enabled`
- 复用：`evaluateOpenAIWSDeltaShadowCandidate` + `buildOpenAIWSActiveDeltaPayload`
- 输入：**canonicalBody** + 同源 `sessionHash`
- 输出：body、applied、sessionHash、shadow log、sessionOwner
- **不**依赖 OpenAI 的 `openAIHTTPIncrementalContinuationEnabled` / `openai_http_previous_response_id_supported`

#### 3. `resolveGrokActiveDeltaSessionHash`（P4）

与 cache identity 同源，避免两套 seed 导致永不命中：

```text
identity = resolveGrokCacheIdentity(c, body, explicitKey, upstreamModel)
if identity == "":
  // 无稳定会话信号 → 不 bind、不增量（Full only）
  return ""
sessionHash = GenerateSessionHash 语义上等价于对「可复现的 identity seed」做 api_key 隔离哈希
// 推荐：直接用 identity 作为 session seed 的规范化输入
// sessionHash = hash(api_key_id + ":" + identity) 或现有 deriveOpenAIRequestScopedSessionHashes
```

规则：

- **bind 与下一轮 evaluate 必须使用同一 sessionHash 算法**
- 调用 `GenerateSessionHash(c, canonicalBody)` 时，`canonicalBody` 必须已含与上游一致的 `prompt_cache_key`（cache identity），或显式传入 identity 作为 seed——二者择一并在代码注释钉死
- 无 identity → 不写 session context（P1 fail closed）

#### 4. `bindGrokHTTPResponseSessionContext`

- 仅 OAuth + 开关开 + 非空 responseID + 非空 sessionHash
- **hashes 只来自 canonicalBody 的 input**（delta 前全量）
- `connID = "http"`
- `inputOnlyContext = true`（对齐 OpenAI HTTP）
- TTL 复用 `openAIWSSessionStickyTTL`

#### 5. Full path strip previous（P5）

当 `!applied` 时，在发上游前：

```text
if has unsafe/stale previous and NOT (tool-output continuation that restore refuses to strip):
  delete previous_response_id
  ensure store per store policy
```

复用/对齐 `restoreOpenAIHTTPActiveDeltaFullReplayBody` 的 strip 语义。  
危险 `function_call_output` 等：不注入 previous，也不假装可续链；按现有 tool 安全规则 fail closed。

#### 6. Error classifier + full-replay（P6）

识别：

- `previous_response_not_found`
- message 含 previous response + not found
- unsupported parameter + previous_response_id

触发 full-replay **全部**满足：

1. `!c.Writer.Written()`
2. 尚未开始向客户端写 SSE 事件
3. 本请求尚未做过 previous full-replay
4. 可恢复出 full body（优先 `canonicalBody` / delta 前快照）

否则走现有 error/failover，**禁止**半流式后重放。

## Safety Rules

全部满足才允许增量；任一失败 → Full + strip previous（P5）。

1. `account.Platform == grok` 且 `account.Type == oauth`
2. `gateway.grok.http_active_delta_enabled == true`（Grok 独立开关）
3. 非 compact 路径（compact 跳过 active-delta 与 cache identity，与现网一致）
4. 非空 `sessionHash`（同源 identity 可得）
5. `TrySessionInFlight` 成功（同 session 并发 → Full）
6. session context 存在且 `connID == "http"`
7. sticky 账号一致：
   - `cached.accountID == selected account`
   - 客户端 previous 若存在，则 `GetResponseAccount` 绑定账号必须匹配
8. 客户端 previous 若存在，必须等于 `cached.lastResponseID`
9. 非危险 tool-output 形态
10. strict prefix：`cached.materializedHashes` 是当前 full input 的严格前缀
11. non-input 指纹允许 active delta
12. delta 非空；`cached.lastResponseID` 非空
13. evaluate 的 payload 与 bind 时 **同一 canonicalBody 归一化规则**

**禁止**：

- 无前缀证明时裁剪到「最后 N 条」
- 跨账号 previous 续聊
- 客户端已收流后 full-replay
- 绕过 `doGrokResponsesUpstream` 的 Grok OAuth 文本上游调用
- 在 raw vs patched 混用哈希

## Store Semantics（P8）

### Task 0 — 上线前探针（硬前置）

用真实 Grok OAuth 账号验证：

| Case | Request | Expect |
|------|---------|--------|
| A | turn1 `store=false`，记录 `response.id` | 成功 |
| B | turn2 `store=false` + `previous_response_id` + **仅** new input | 成功或明确错误码 |
| C | turn1 `store=true`，turn2 `store=false` + previous + only-new | 若 B 失败，验证 C 是否成功 |

结果写入 Decision Log，并锁定默认策略：

| Probe result | Default policy |
|--------------|----------------|
| B 成功 | 全 turn `store=false`（对齐 OpenAI ZDR active-delta） |
| B 失败且 C 成功 | 首轮/Full create `store=true`，增量 turn `store=false` + previous |
| B/C 均失败 | **默认关闭 active-delta mutation** 或仅 shadow，不得默认 true 上线 |

未完成 Task 0 前，实现可合入但 **默认 `http_active_delta_enabled=false` 或仅 shadow**；探针通过后再默认 true。

### 运行时表（探针通过后）

| Turn | store | previous_response_id |
|------|-------|----------------------|
| 首轮 / Full | 按 Decision Log | 不发送 |
| Safe incremental | 按 Decision Log（通常 false） | cached.lastResponseID |

配置预留：

- `gateway.grok.http_active_delta_require_store_on_create`（默认由探针决定）

## Configuration

| Key | Default | Meaning |
|-----|---------|---------|
| `gateway.grok.http_active_delta_enabled` | **探针通过前 `false`；通过后 `true`** | Grok OAuth payload mutation 总开关 |
| `gateway.grok.http_active_delta_require_store_on_create` | 探针决定 | 首轮/Full 是否 store=true |
| sticky TTL | 复用 OpenAI WS session/response TTL | context 与 response→account |

**明确不依赖：**

- `gateway.openai_ws.http_incremental_continuation_enabled`（OpenAI 默认 false）
- 账号 extra `openai_http_previous_response_id_supported`

**Shadow 日志：** 可复用 `logOpenAIWSDeltaShadow` 结构；mutation 门闩 **只**看 Grok 开关。若需与 OpenAI 共用 admin shadow，必须在日志中区分 `platform=grok`，且 **关闭 OpenAI active 不得静默关闭 Grok**（P7）。

## Integration Points

### Path coverage matrix（Phase 1 验收用）

| Client entry | Convert | Upstream egress |
|--------------|---------|-----------------|
| `POST /v1/responses` (Grok group) | patch + cache id | **must** `doGrokResponsesUpstream` |
| `POST /v1/messages` (Claude → Grok) | Anthropic→Responses | **must** `doGrokResponsesUpstream` |
| `POST /v1/chat/completions` (Grok bridge) | Chat→Responses | **must** `doGrokResponsesUpstream` |
| Grok Chat raw（bridge 不合格） | raw CC | 不走 active-delta（非 Responses） |
| Imagine / videos | n/a | out of scope |
| Grok apikey 文本 | 任意 | **不**走 active-delta |

### `doGrokResponsesUpstream` 内部顺序

1. 断言 account 为 Grok OAuth（否则调用方错误）
2. 保存 `canonicalBody` 快照
3. `buildGrokHTTPActiveDeltaPayload`
4. 若 `!applied`：strip previous（P5）
5. build request + headers（session_id / x-grok-conv-id 与 cache identity 一致）
6. DoWithTLS 循环：
   - compaction retry（现有）
   - previous full-replay（P6）
7. 成功：handle stream/non-stream → bind account → bind session context(canonicalBody)
8. defer：release in-flight if owner

### Sticky scheduling

- 继续 `bindHTTPResponseAccount`
- 调度侧 previous_response sticky 保持；换账号时 safety rule 7 强制 Full
- multi-instance 下无共享 store → 更多 Full（P9），语义仍正确

## Failure Recovery Order

1. Compaction blob decode retry（最多 1 次）
2. previous_response failure → full-replay（最多 1 次，且 P6）
3. 现有 failover / error response

Full-replay 后：

- `activeDeltaApplied = false`
- 不再二次 previous 重试
- 成功仍用 **canonicalBody** hashes bind session context

## Observability

1. `logOpenAIWSDeltaShadow` 字段复用，附加 `platform=grok` 或 logger 前缀
2. 专用行：
   - `grok_http_active_delta_applied ...`
   - `grok_http_active_delta_skipped reason=...`（含 `no_session_identity` / `feature_disabled` / `prefix_break_*`）
   - `grok_http_active_delta_full_replay_retry ...`
   - `grok_http_active_delta_full_replay_blocked reason=already_written`
3. `OpenAIForwardResult` 可填现有 delta 统计字段，便于命中率看板
4. 指标建议（若已有 ops 管道）：`delta_applied_total` / `delta_full_total{reason=}` / `full_replay_total`

## Testing

### Unit / integration

1. First turn full bind（canonicalBody hashes）
2. Strict prefix second turn → only-new + previous
3. Prefix rewrite → Full + **无** previous
4. Account mismatch → Full + strip previous
5. Client previous ≠ cached → Full + strip previous
6. previous_response_not_found → full-replay 成功
7. Tool continuation unsafe → 不增量
8. Apikey → 完全不进入
9. Compaction + previous retry 标志共存
10. Session inflight → 第二请求 Full
11. **Claude messages 路径**进入 `doGrokResponsesUpstream`（mock 断言）
12. **Chat bridge 路径**进入 `doGrokResponsesUpstream`
13. **sanitize 后**多轮仍能 prefix match（空 content / unsupported tool 剥离）
14. **同源 sessionHash**：仅 cache identity、无 client session header 时多轮可增量
15. **无 identity**：永不 bind
16. **Written 后**不 full-replay
17. Stream 成功取 responseID 并 bind
18. 绕过出口的回归：静态检查或测试保证 Grok OAuth 无第二处 DoWithTLS Responses

### Concurrency

session bind / in-flight 相关测试 `-count` 加压（项目规范）。

## Implementation Sketch

| File | Change |
|------|--------|
| `openai_gateway_grok_active_delta.go` | sessionHash 同源、build/bind/release、strip full path |
| `openai_gateway_grok.go` | 抽出 `doGrokResponsesUpstream`；forward 改调它 |
| `openai_gateway_messages.go` | Grok 分支改调统一出口 |
| `openai_gateway_grok_chat_bridge.go` | bridge 改调统一出口 |
| `config.go` + `deploy/config.example.yaml` | Grok 开关与 store-on-create |
| `openai_gateway_grok_active_delta_test.go` | 矩阵测试 |
| Task 0 探针记录 | Decision Log 或 `docs/...` 附录 |

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| store=false 不支持 previous | Task 0 探针；未通过不默认 true |
| session 双 seed 导致 0 命中 | P4 同源强制 |
| 只改 responses、Claude 不增量 | P2 单一出口 + 路径矩阵验收 |
| patch 改 input 导致 prefix 永破 | P3 单一 canonicalBody |
| 半流式后重放污染客户端 | P6 unwritten only |
| 多实例 state 不共享 | P9 Full 降级正确 |
| OpenAI 关 active 误伤 Grok | P7 独立开关 |
| 误裁语义 | P1 strict prefix only |

## Success Criteria

1. Path matrix 中三条 Grok OAuth 文本路径 **全部**经 `doGrokResponsesUpstream`
2. 同 session、严格前缀续聊时上游 input **仅为**增量 items
3. 任意不安全情况：全量 input + **无**错误 previous_response_id
4. previous 上游失败且 unwritten：同账号 full-replay 一次成功（全量可成功时）
5. 已 written：不 full-replay，无重复/错乱客户端事件
6. Grok apikey 行为与改前一致
7. 无 session identity 时行为为 Full，且不写脏 session context
8. Task 0 探针结果已记录；默认开关与 store 策略与之一致
9. 相关单测通过；关键并发路径 `-count` 无 flaky

## Decision Log

| Decision | Choice | Why |
|----------|--------|-----|
| Scope | 仅 Grok OAuth | 用户指定；降风险 |
| Fallback | 对齐 OpenAI HTTP + P5 strip | 安全优先 |
| Kernel | 复用 OpenAI strict-delta | 避免第二套哈希 |
| Egress | 强制 `doGrokResponsesUpstream` | 评审 I1；Claude/Chat 主流量 |
| Session key | 与 Grok cache identity 同源 | 评审 I2 |
| Body view | patch+cache 后唯一 canonicalBody | 评审 I3 |
| Flag | Grok 独立；探针前默认 false | 评审 C1/I4 + 最佳实践 |
| OpenAI coupling | 不依赖 OpenAI HTTP incremental | 避免默认 false 误伤 |
| Retry | unwritten only | 评审 I5 |
| Full strip previous | 是 | 评审 I6 |

### Task 0 probe results（实现时填写）

| Case | Result | Date | Notes |
|------|--------|------|-------|
| A store=false create | _TBD_ | | |
| B store=false previous only-new | _TBD_ | | |
| C store=true create then previous | _TBD_ | | |
| Locked defaults | `http_active_delta_enabled=_` `require_store_on_create=_` | | |

## Non-goals Reminder

本设计 **不是** SSE token streaming 改造，也 **不是** 强制客户端只发增量。  
客户端仍可发全量 history；网关在证明安全后自行裁剪上游 payload。

## Implementation readiness

| Gate | Status |
|------|--------|
| Design principles P1–P10 | Locked |
| Path matrix | Locked |
| Task 0 probe | **Required before default-on** |
| Implementation plan | Next after this spec approval |
