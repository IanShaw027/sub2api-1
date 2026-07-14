# Grok OAuth HTTP Active Delta Design

## Goal

为 **Grok OAuth** 上游 HTTP Responses 路径建立与 OpenAI HTTP active-delta 对齐的「安全增量」能力：

1. 默认可增量：续聊时只向上游发送 new input items + `previous_response_id`
2. 安全优先：本地无法证明前缀安全时，必须全量上传
3. 失败可恢复：上游续聊失败时，同账号 full-replay 重试一次
4. 不扩大平台范围：仅 Grok OAuth；apikey 与其它平台不动

## Scope

### In scope

1. Grok OAuth → `forwardGrokResponses` / `forwardGrokResponsesWithPromptCacheKey`
2. 已转换为 xAI Responses body 的 Grok 文本路径（含会落到上述 forward 的桥接入口）
3. Session context bind（`connID="http"`）+ strict-delta 判定 + payload 改写
4. `previous_response_not_found` / unsupported previous_response 的同账号 full-replay 重试
5. 配置开关、可观测日志、单测矩阵

### Out of scope

1. Grok apikey 账号
2. OpenAI / Anthropic / Gemini / Kiro 等其它平台 continuation 改造
3. 新增数据库表或 migration
4. 对外 API 协议新增字段
5. 改变 usage / 计费口径
6. 图/视频 Imagine 路径（非文本 Responses 续聊）

## Problem Statement

xAI Responses 官方支持有状态续聊：`previous_response_id` + 仅 append 新消息，无需每次重发 full conversation。

本仓库 Grok HTTP 路径当前行为：

1. **透传客户端 body**，不主动裁剪 input
2. `patchGrokResponsesBody` 在无 `previous_response_id` 时默认强制 `store=false`
3. 仅有 `response_id → account` 粘连（`bindHTTPResponseAccount`），**没有** session input 哈希上下文
4. Claude Code / Codex 等客户端常见「每轮全量 turns」形态，导致上游重复接收历史

OpenAI 侧已有成熟实现：

- WS / HTTP strict-delta：`openai_ws_delta_shadow.go` + `openai_http_active_delta.go`
- 安全条件：account sticky、session context、input 前缀哈希、non-input 指纹、tool continuation 保护
- 不安全 → 全量；上游 previous 失败 → full-replay 一次

Grok OAuth 应复用同一安全内核，而不是另写一套裁剪启发式。

## Design Summary

采用 **方案 A：复用 OpenAI HTTP active-delta 内核，挂到 Grok OAuth 路径**。

统一决策只有两种结果：

| 结果 | 上游 payload |
|------|----------------|
| Safe incremental | `input = only-new-items`，`previous_response_id = cached.lastResponseID`，`store=false` |
| Full | 完整 `input`，删除 `previous_response_id`（若非 tool-output 危险场景需要保留则不走本路径），`store=false` |

本地判定失败只会产生 Full，不会产生「猜最后 N 条」的半增量。

## Architecture

```
client body (often full history)
        │
        ▼
reject unsupported tools / patchGrokResponsesBody / cache identity
        │
        ▼
buildGrokHTTPActiveDeltaPayload
  - gate: PlatformGrok + OAuth + feature flag
  - reuse: evaluateOpenAIWSDeltaShadowCandidate + buildOpenAIWSActiveDeltaPayload
        │
        ├─ applied  → delta body
        └─ not applied → full body (safe default)
        │
        ▼
buildGrokResponsesRequest + DoWithTLS
        │
        ├─ error: previous_response* + delta was applied
        │     → restore full original body, same account, retry once
        ├─ error: compaction blob (existing)
        │     → existing compaction sanitize retry once
        └─ success
              → bindHTTPResponseAccount(responseID)
              → bindGrokHTTPResponseSessionContext(
                    full original input hashes, connID="http", lastResponseID)
```

### Components

1. **`buildGrokHTTPActiveDeltaPayload`**
   - 输入：patch 后的 body、account、client previous_response_id（若有）
   - 输出：`body`、是否 applied、sessionHash、shadow log、sessionOwner
   - 与 OpenAI `buildOpenAIHTTPActiveDeltaPayload` 同构，gate 改为 Grok OAuth

2. **`bindGrokHTTPResponseSessionContext`**
   - 成功响应后写入 session context
   - **必须用 delta 前的全量 original input** 计算 `materializedHashes`（与 OpenAI HTTP 一致）
   - `connID` 固定 `"http"`，避免与 OpenAI WS 上下文串扰

3. **Full-replay restore**
   - 直接复用 `restoreOpenAIHTTPActiveDeltaFullReplayBody`
   - 删除 `previous_response_id`，强制 `store=false`

4. **Error classifier**
   - 复用/扩展 existing previous_response 分类逻辑，兼容 xAI 文案：
     - `previous_response_not_found`
     - message 含 previous response + not found
     - unsupported parameter + previous_response_id（若出现）

## Safety Rules

全部满足才允许增量；任一失败 → 全量。

1. `account.Platform == grok` 且 `account.Type == oauth`
2. Grok HTTP active-delta 开关开启，且全局 active-delta 开关开启（与 OpenAI 共用 `openAIWSActiveDeltaEnabled`，或 Grok 独立开关但默认复用同一 shadow 内核）
3. 非 compact 特殊路径（若 Grok compact 路径存在则跳过 active-delta）
4. 可计算非空 `sessionHash`（`GenerateSessionHash`）
5. session in-flight 获取成功（同 session 并发 inflight → 不增量）
6. session context 存在且 `connID == "http"`
7. sticky 账号一致：
   - cached.accountID == selected account
   - 若客户端带 previous_response_id，则 `GetResponseAccount` 绑定账号必须匹配
8. 若客户端带 previous_response_id，必须等于 cached.lastResponseID
9. 非危险 tool-output 形态（`HasToolContinuationOutputInRawPayload` 等现有 fallback）
10. strict prefix：cached.materializedHashes 是当前 full input 的严格前缀
11. non-input 指纹允许 active delta
12. delta 非空；cached.lastResponseID 非空

**禁止**：

- 无前缀证明时裁剪到「最后一条 user」
- 跨账号续聊 previous_response_id
- 在无法 restore 全量 original 时仍对上游发残缺 delta

## Store Semantics

对齐 OpenAI HTTP active-delta 的 `store=false` 续链模型：

| Turn | store | previous_response_id |
|------|-------|----------------------|
| 首轮 / 全量回退 | `false` | 不发送 |
| 安全增量 | `false` | cached.lastResponseID |

`patchGrokResponsesBody` 现有逻辑：

- 无 previous 时默认 `store=false`：保持
- 有 previous 时不强行 `store=false`：保持（增量路径会显式写 `store=false`）

**预留（非默认）**：若实测 xAI Grok 在 `store=false` 下无法 previous 续聊，增加可选配置：

- `Gateway.Grok.HTTPActiveDeltaRequireStoreOnCreate`（默认 `false`）
- 仅在「首轮 full create」时写 `store=true`，增量 turn 仍 `store=false`

默认实现不猜；依赖 full-replay 兜底与观测数据再决定是否打开。

## Configuration

建议新增（命名可在实现时微调，语义固定）：

| Key | Default | Meaning |
|-----|---------|---------|
| `gateway.grok.http_active_delta_enabled` | `true` | Grok OAuth HTTP 安全增量总开关 |
| （复用）`OPENAI_WS_ACTIVE_DELTA_DISABLED` / admin shadow 开关 | 现网 | 关闭则 Grok 也不做 payload mutation |
| sticky TTL | 复用 OpenAI WS session/response TTL | session context 与 response→account TTL |

与 OpenAI `HTTPIncrementalContinuationEnabled`（默认 false）**解耦**，避免 Grok 增量被 OpenAI 开关误关。

## Integration Points

### Primary: `forwardGrokResponsesWithPromptCacheKey`

顺序：

1. 现有 tool reject + `patchGrokResponsesBody` + cache identity
2. 保存 `originalPatchedBody`（delta 前快照）
3. `buildGrokHTTPActiveDeltaPayload`
4. 若 applied，替换 `patchedBody`，记 log
5. 构建 request / DoWithTLS
6. 错误环：
   - 现有 compaction retry
   - **新增** previous_response full-replay retry（仅 applied 时，最多 1 次）
7. 成功：
   - 现有 usage snapshot
   - stream/non-stream handle
   - `bindHTTPResponseAccount`
   - `bindGrokHTTPResponseSessionContext(originalPatchedBody, responseID)`

### Secondary paths

- `forwardGrokChatCompletionsViaResponses`：若最终走独立 upstream 而不经 `forwardGrokResponses*`，需在同一 Responses 出口复用 active-delta，或统一改走 `forwardGrokResponses*`。实现时优先 **单一出口**，避免双路径漂移。
- Claude `/v1/messages` → Grok Responses：若 body 已是 Responses 且进入 `forwardGrokResponses*`，自动受益；若仍每轮全量 messages 映射，prefix 不匹配则自然全量（正确降级）。

## Failure Recovery Order

同一请求内重试顺序：

1. Compaction blob decode retry（现有，最多 1 次）
2. Active-delta previous_response failure → full-replay（新增，最多 1 次）
3. 现有 failover / error response

Full-replay 后：

- `activeDeltaApplied = false`
- 不再对同请求二次 previous 重试
- 成功仍 bind session context（用 full original hashes）

## Observability

1. 复用 `logOpenAIWSDeltaShadow`（fallback_reason / prefix_match / delta_items 等）
2. Grok 专用 info 行：
   - `grok_http_active_delta_applied account_id=... session=... previous_response_id=... delta_items=... full_items=...`
   - `grok_http_active_delta_full_replay_retry account_id=... reason=...`
3. `OpenAIForwardResult` 可填充现有 `OpenAIWSDelta*` 字段，便于 ops 统计增量命中率

## Testing

单测（`openai_gateway_grok_*` / 新 `openai_gateway_grok_active_delta_test.go`）：

1. **First turn full bind**：上游收全量；session context 写入 lastResponseID + hashes
2. **Strict prefix second turn**：上游 body 仅 delta items + previous_response_id + store=false
3. **Prefix rewrite**：历史被改 → 全量，无 previous_response_id
4. **Account mismatch**：cached/response 绑定其它账号 → 全量
5. **Client previous mismatch**：客户端 previous ≠ cached → 全量
6. **previous_response_not_found retry**：delta 请求 400 → full-replay 成功
7. **Tool continuation unsafe**：function_call_output 危险形态 → 不增量
8. **Apikey account**：不进入 active-delta
9. **Compaction + delta coexistence**：两种 retry 标志互不吞掉
10. **Session inflight**：同 session 并发 → 第二请求不增量

并发回归：对 session bind/in-flight 相关测试用 `-count` 加压（遵循项目并发测试规范）。

## Implementation Sketch

建议文件：

| File | Change |
|------|--------|
| `backend/internal/service/openai_gateway_grok_active_delta.go` | Grok gate + build/bind/release 封装 |
| `backend/internal/service/openai_gateway_grok.go` | 接入 forward 循环、retry、bind |
| `backend/internal/config/config.go` + example yaml | `gateway.grok.http_active_delta_enabled` |
| `backend/internal/service/openai_gateway_grok_active_delta_test.go` | 矩阵测试 |

不修改 OpenAI strict-delta 哈希语义；若 Grok 需要额外 volatile 字段，先用 fallback 全量，再评估是否扩展 denylist（单独评审）。

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| xAI 在 store=false 下不支持 previous 续聊 | full-replay 兜底；观测后可选 RequireStoreOnCreate |
| sessionHash 不稳定导致几乎不增量 | 复用现有 GenerateSessionHash + Grok cache identity；加多轮同 session 测试 |
| bridge 路径重复实现 | 强制单一 Responses 出口 |
| 与 compaction 重试互相覆盖 | 独立 tried 标志 + 明确顺序 |
| 误裁 input 导致语义错误 | 仅 strict prefix 证明后裁剪；否则全量 |

## Success Criteria

1. Grok OAuth 同 session 严格前缀续聊时，上游请求 body 的 input 仅为增量 items
2. 任何无法证明安全的情况，上游收到全量 input 且无错误 previous_response_id
3. previous_response 上游失败时，同账号 full-replay 一次且客户端最终成功（在全量可成功的前提下）
4. Grok apikey 行为与改前一致
5. 相关单测通过；关键并发路径 `-count` 加压无 flaky

## Decision Log

| Decision | Choice | Why |
|----------|--------|-----|
| Scope | 仅 Grok OAuth | 用户指定；降低风险面 |
| Fallback | 严格对齐 OpenAI HTTP | 本地不安全→全量；上游 previous 失败→full-replay |
| Kernel | 复用 OpenAI strict-delta | 避免第二套哈希/安全语义 |
| Default flag | Grok 开关默认 true | 用户要求「默认走增量」；仍可用配置关闭 |
| OpenAI flag coupling | 解耦 | OpenAI HTTP incremental 默认 false，不能绑死 Grok |

## Non-goals Reminder

本设计 **不是** SSE token streaming 改造，也 **不是** 强制客户端只发增量。  
客户端仍可发全量 history；网关在证明安全后自行裁剪上游 payload。
