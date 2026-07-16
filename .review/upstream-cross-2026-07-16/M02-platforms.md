# Module 2: 多平台上游适配

## 范围摘要
- 对比基线: `upstream/main`=`b960ec19807ea81d6d83cad63e84ad45fdf7e08a` → `HEAD`=`eb64a5c1678eeb3dd4111e9a366e0b963f58a1e2`（merge-base `da85cc7e…`）
- 本模块变更规模（相对上游分叉面，按路径焦点粗估）:
  - **Kiro**: `backend/internal/pkg/kiro/**` + `service/kiro_*` + OAuth/usage/handler，体量大（gateway 单文件 >4.7k 行；假缓存 + first-event timeout + native web continuation 完整链路）
  - **Grok/xAI**: `backend/internal/pkg/xai/**` + `service/grok_*` + `openai_gateway_grok_*`（quota probe、HTTP active-delta、free soft-gate、media）
  - **Gemini**: `pkg/geminicli/**` + `service/gemini_*`（messages compat、shared-pool cooldown、sticky session）
  - **Antigravity**: `pkg/antigravity/**` + `service/antigravity_*`（credits overages、internal500 渐进惩罚、smart retry、sticky）
  - **Claude/Anthropic**: `pkg/claude`、`pkg/oauth`、`service/claude_*`（telemetry 脱敏转发、token provider、session）
- 相对上游的主要能力差异:
  - Kiro 全链路（Anthropic Messages ↔ Kiro frame 协议、假 prompt-cache 计费、首事件超时阈值冷却、profile 级 failover 排除、OAuth IDC continuation 安全校验）
  - Grok OAuth 独立 active-delta（与 OpenAI WS 增量开关解耦，默认开启）、quota/billing probe、free 账号 soft-gate
  - Antigravity credits overages 二次请求 + INTERNAL 500 渐进惩罚 + 进程内 model capacity 去重
  - Claude telemetry 仅 OAuth 账号转发且 fail-closed 脱敏
  - 多平台 OAuth session Redis 共享 + 写失败 local-only 回退

## 关键路径图

```
Client
  │
  ├─ POST /v1/messages (Anthropic 协议)
  │     handler/gateway_handler
  │       ├─ platform=kiro        → KiroGatewayService.Forward
  │       │     prepareFakeCachePlan → ConvertAnthropicRequest
  │       │     → HTTPUpstream (TLS fingerprint) → eventstream frames
  │       │     → first-forwardable watchdog / native web continuation
  │       │     → resolveFakeCacheUsage → RecordUsage
  │       ├─ platform=antigravity → AntigravityGatewayService.Forward
  │       │     sticky session → smart retry / credits overages / capacity cooldown
  │       └─ platform=anthropic   → GatewayService.Forward (+ Claude sanitizer/telemetry)
  │
  ├─ POST /v1/responses|/v1/chat/completions (OpenAI 协议)
  │     handler/openai_gateway_handler
  │       platform=grok → callGrokResponsesHTTP
  │         buildGrokHTTPActiveDeltaPayload / prepareGrokFullUpstreamBody
  │         → xAI Responses；失败可 full-replay + invalidate session
  │
  ├─ POST /v1beta/* (Gemini)
  │     gemini_v1beta_handler → GeminiMessagesCompatService
  │       sticky session → shared-pool OAuth cooldown / AI Studio 路径分流
  │
  └─ Admin OAuth 回调
        kiro/grok/gemini/antigravity_oauth_handler
          SessionStore.TryConsumeSession → token exchange
          Kiro 额外: IDC/external-idp continuation（issuer/start_url allowlist）
```

## 发现清单

### [P1] Kiro native web-tool continuation 失败时可能漏记已流式输出用量
- **位置**: `backend/internal/service/kiro_gateway_service.go:2325-2339`；对照主路径 partial billing `backend/internal/handler/gateway_handler.go:1010-1049`
- **相对上游**: 本地新增（Kiro 原生 web_search/web_fetch 自动续写）
- **问题**: 主 SSE 流已 `streamStarted=true` 并向客户端写出 content 后，`startKiroNativeWebToolContinuation` 成功拿到 continuation 响应，若后续 `kiroFrameFailure` 触发 `handleFrameFailure(..., writeClientError=false)`，函数以 `return nil, err` 退出。`handleFrameFailureWithCooldown` 在可 failover 状态码下返回 `*UpstreamFailoverError` 且 **不附带** `buildKiroPartialStreamResult()`。  
  handler 对 failover 分支在「已写流」时只 `handleFailoverExhausted` 并 `return`，**不会**进入下方 `if result != nil { RecordUsage }` 的 partial 计费路径。
- **影响**: 上游配额与客户端可见输出已发生，但本地可能完全漏计费；同时可能把本应终态的错误包装成 failover 语义（虽有 stream-written 守卫阻止切换账号，但计费侧仍断）。
- **证据**:
  ```go
  // kiro_gateway_service.go:2338-2339
  if failureErr := kiroFrameFailure(frame); failureErr != nil {
      return nil, s.handleFrameFailure(ctx, c, account, ..., failureErr, false)
  }
  // 对比 post-start 主循环: return buildKiroPartialStreamResult(), handledErr
  ```
  ```go
  // gateway_handler.go:986-992 + 1015
  if errors.As(err, &failoverErr) {
      if gatewayFailoverStreamAlreadyWritten(...) { handleFailoverExhausted(...); return }
      ...
  }
  if result != nil { RecordUsage(...) } // failover 早退时永远走不到
  ```
- **建议**: continuation 失败路径统一 `return buildKiroPartialStreamResult(), nonFailoverErr`（或自定义「已提交流、禁止切换」错误类型）；单测覆盖「主流转 tool_use → continuation exception → 仍 RecordUsage」。
- **交叉关注**: M01（failover/committed 语义）、M04（漏计费）

### [P1] Grok 请求路径 token 刷新窗口为 1 小时，与缓存 skew 严重不一致
- **位置**: `backend/internal/service/grok_token_refresher.go:10-39`；`backend/internal/service/grok_token_provider.go:15-92,99-119`
- **相对上游**: 本地新增 / 本地修改（Grok OAuth 平台）
- **问题**: `grokTokenRefreshSkew = time.Hour` 同时用于：
  1. 后台/统一刷新器 `NeedsRefresh`（剩余寿命 < 1h 即刷新）
  2. 请求路径 `GetAccessToken` 的 `needsRefresh` 判定（`time.Until(expires) <= grokTokenRefreshSkew`）
  而缓存写入仅扣 `5 * time.Minute`（`grokTokenCacheSkew`）。结果：大量仍有 5–60 分钟有效的 access_token 在热路径被强制同步刷新（`RefreshIfNeeded` + 8s 超时）；刷新失败时策略为 `ProviderRefreshErrorReturn`（借用 Antigravity policy），直接失败并 temp-unsched 10 分钟，**不会**降级使用仍有效的旧 token。
- **影响**: 高 QPS 下刷新风暴、上游 OAuth 限流、可调度账号被错误冷却；可用性与调度稳定性风险明确。
- **证据**:
  ```go
  // grok_token_refresher.go
  const grokTokenRefreshSkew = time.Hour
  return time.Until(*expiresAt) < refreshWindow // 最小也被抬到 1h
  // grok_token_provider.go
  needsRefresh := expiresAt == nil || time.Until(*expiresAt) <= grokTokenRefreshSkew
  ...
  if err != nil {
      p.markTempUnschedulable(account, err)
      if p.refreshPolicy.OnRefreshError == ProviderRefreshErrorReturn { return "", err }
  }
  ```
- **建议**: 拆分 `requestRefreshSkew`（建议 2–5min）与 `backgroundRefreshSkew`（可 30–60min）；请求路径失败时若 token 仍未过期应降级使用；补并发刷新单测。
- **交叉关注**: M06（temp-unsched/cooldown）、M05（OAuth 刷新安全）

### [P1] Antigravity `MODEL_CAPACITY_EXHAUSTED` 去重 cooldown 仅进程内存，多实例失效
- **位置**: `backend/internal/service/antigravity_gateway_service.go:82-86,292-315,396-404`
- **相对上游**: 本地修改 / 上游可能无对等逻辑
- **问题**: `modelCapacityExhaustedUntil` 是 package 级 `map` + `sync.RWMutex`，注释写明「避免多个并发请求同时对同一模型进行容量耗尽重试」，但状态不进 Redis/DB。多副本部署下每个实例各自最多重试 60 次 × 1s，全局放大为 `N_replicas × 60` 次上游冲击。
- **影响**: 上游容量事件时跨实例重试雪崩；延迟与 503 放大；与 sticky/切换逻辑叠加后可观测性差。
- **证据**:
  ```go
  var (
      modelCapacityExhaustedMu    sync.RWMutex
      modelCapacityExhaustedUntil = make(map[string]time.Time)
  )
  // 成功/耗尽时仅改本地 map
  modelCapacityExhaustedUntil[modelName] = time.Now().Add(antigravityModelCapacityCooldown)
  ```
- **建议**: 使用 Redis key（model 维度 SET NX + TTL）或复用现有 temp-unsched/model rate-limit 基础设施；单测用 fake redis 覆盖跨「逻辑实例」。
- **交叉关注**: M06（调度/冷却一致性）

### [P1] Grok active probe 对非 429 的 4xx/5xx 也会写入 quota snapshot Extra
- **位置**: `backend/internal/service/grok_quota_service.go:136-170`
- **相对上游**: 本地新增
- **问题**: `ObserveQuotaHeaders` 后无条件 `UpdateExtra(grok_quota_snapshot)`，随后才判断 `status>=400` 返回错误。401/403/5xx 的空/噪声 header 会覆盖此前有效的 rate-limit 观测；`HeadersObserved=false` 的 no-header probe 也会持久化，下游调度/展示可能误读「刚刚探测过但无配额信息」。
- **影响**: 配额可视化失真；若调度依赖 snapshot 新鲜度，可能错误放行/错误限流（与 free soft-gate 叠加时更难排查）。
- **证据**:
  ```go
  snapshot := xai.ObserveQuotaHeaders(resp.Header, resp.StatusCode, "active_probe")
  _ = s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{ grokQuotaSnapshotExtraKey: snapshot })
  ...
  if resp.StatusCode >= 400 {
      return nil, infraerrors.Newf(..., "GROK_QUOTA_PROBE_UPSTREAM_ERROR", ...)
  }
  ```
- **建议**: 仅在 `status < 400 || status == 429` 且（header 有用或显式 no-header 策略）时持久化；错误路径保留旧 snapshot 并单独记 `last_probe_error`。
- **交叉关注**: M04（配额）、M06（调度阈值）

### [P2] Kiro first-event timeout 冷却仅 30s，且阈值计数依赖 Redis 计数器可用性
- **位置**: `backend/internal/service/kiro_gateway_service.go:49-54,72,482-519,2262-2264,4492-4496`
- **相对上游**: 本地新增
- **问题**: 60s 内无 forwardable event 会关 body 并 failover；需 2 分钟窗口内连续 3 次才 temp-unsched 30s。若 `tempUnschedCounter` 不可用则只打 warn 不冷却；阈值未达时账号仍可被反复选中（每次占满 60s 连接/槽位）。profile 排除依赖 `ListByPlatform(PlatformKiro)` 全表扫描，账号池大时成为 failover 热路径开销。
- **影响**: 坏账号/坏 profile 在阈值前可持续拖慢请求；计数器故障时 debuff 失效。
- **证据**: `kiroFirstEventTimeoutCooldown = 30 * time.Second`；`IncrementTempUnschedThreshold` 失败直接 return；`ListByPlatform` 过滤同 `profile_arn`。
- **建议**: 提高冷却或按 profile 维度共享计数；profile 排除走索引查询；counter 不可用时降级为单次冷却。
- **交叉关注**: M06

### [P2] Kiro fake-cache 为进程内 ristretto，多实例/滚动发布下 cache 命中与计费不一致
- **位置**: `backend/internal/service/kiro_fake_cache.go`；`backend/internal/pkg/kiro/fake_cache.go:74-102,208-261`；`kiro_gateway_service.go:2949-3065`
- **相对上游**: 本地新增
- **问题**: Session progress / prefix keys 仅存本进程。sticky 未命中或实例漂移时，下一轮 `HasEffectiveCachedTokens=false`，会按「全量 cache write scaling」重新生成 creation tokens，与客户端感知的「应有 cache read」不一致。`HitRateScale` 默认 85（settings `defaultKiroCacheHitRateScale`），刻意少记 cache_creation——这是产品策略，但跨实例漂移会放大为用户账单抖动。
- **影响**: 计费可重复性下降；运维滚动时账单尖刺/回落难解释。
- **证据**: `kiro:fakecache:v2:user:%d:key:%d:...` 仅作内存 key；`commitFakeCachePlan` 写 ristretto；测试覆盖 scaling 与 progress CAS，无跨进程一致性测试。
- **建议**: 关键 progress 外置 Redis（TTL=prefix TTL）或强制 session sticky 到实例；文档明确「cache 模拟非强一致」。
- **交叉关注**: M04、M06

### [P2] Antigravity client_secret 硬编码默认值进仓库
- **位置**: `backend/internal/pkg/antigravity/oauth.go:76-87`
- **相对上游**: 上游可能同在；本地仍持有
- **问题**: `defaultClientSecret = "GOCSPX-..."` 明文；虽可用 env 覆盖，但仓库泄露等于公开 OAuth 客户端密钥（Google 公共客户端场景下风险取决于 Google 侧绑定策略，但仍属密钥卫生违规）。
- **影响**: 供应链/合规审查失败；若客户端被滥用可能影响配额池。
- **证据**: `var defaultClientSecret = "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"`
- **建议**: 强制 env/配置注入，无 secret 时拒绝启动 OAuth 流程；轮换密钥。
- **交叉关注**: M05、M10

### [P2] Antigravity INTERNAL 500 惩罚注释与常量不一致（可维护性/运维误判）
- **位置**: `backend/internal/service/antigravity_internal500_penalty.go:13-34`
- **相对上游**: 本地新增
- **问题**: 常量 `Tier1=30m, Tier2=2h, Tier3=3→SetError`，函数注释写「10 分钟 / 10 小时 / 永久」。运维按注释排障会误判窗口。
- **影响**: 事故响应错误；非直接运行时 bug。
- **证据**: 注释 L32-34 vs 常量 L15-17。
- **建议**: 修正注释或生成文档从常量导出。
- **交叉关注**: M06

### [P2] Grok free soft-gate 与 active probe 双轨，语义易漂移
- **位置**: `backend/internal/service/grok_free_quota_gate.go:115-196`；`grok_quota_service.go:174-201`
- **相对上游**: 本地新增
- **问题**: soft-gate 用本地 usage_log 滚动窗口 token 统计（默认 2M×95%/24h），仅过滤 `subscription_tier/plan_type==free`；真正上游配额靠 probe/billing。stats 失败 fail-open；gate 命中直接从候选列表移除但不写 temp-unsched。多实例下 stats 延迟 + soft 缓存（默认 5s）可导致短时过载。
- **影响**: free 账号可能仍打满上游或被过早摘除；与 probe snapshot 展示不一致。
- **证据**: `filterGrokFreeQuotaAccounts` fail-open；`isExplicitGrokFreeOAuthAccount` 仅字符串 free。
- **建议**: 统一以 billing snapshot 为权威；soft-gate 命中写短 cooldown 便于观测。
- **交叉关注**: M04、M06

### [P2] OAuth session Redis 写失败退化为 process-local，多实例 continuation 可能丢会话
- **位置**: `backend/internal/pkg/xai/oauth.go:122-149`；同类：`pkg/oauth`、`pkg/geminicli`、`pkg/antigravity`、`service` Kiro session store
- **相对上游**: 本地与上游共享模式；本地强化了 localOnly 防复活
- **问题**: Redis SET 失败时 session 标 `localOnly`，仅本进程可 `TryConsume`。LB 把 callback 打到其他副本会「session not found」。Kiro IDC continuation 多步更敏感。
- **影响**: OAuth 绑定失败率在 Redis 抖动时上升；非安全绕过（localOnly 防止跨节点误用陈旧本地副本，方向正确）。
- **证据**: `log.Printf("xai oauth session Redis write failed; using process-local fallback")` + `localOnly`。
- **建议**: 写失败对用户返回 503 重试；metrics 告警；Kiro continuation 状态强制 Redis。
- **交叉关注**: M05

### [P3] Claude telemetry 成功/失败均对调用方返回 200 语义的 status 包装
- **位置**: `backend/internal/service/claude_telemetry_forward.go:20-53`
- **相对上游**: 本地新增
- **问题**: 多处 `return http.StatusOK, err` / 脱敏失败静默丢弃。对防泄露正确（fail-closed），但上游可观测性依赖 slog，调用方难区分「已转发 / 已丢弃」。
- **影响**: 运维排障成本；非资金/安全回归。
- **建议**: 内部结果枚举 + metrics（forwarded/dropped/no_account）。
- **交叉关注**: M10

### [P3] Kiro `agentContinuationId` 每请求新建 UUID，无跨 turn 上游会话亲和
- **位置**: `backend/internal/pkg/kiro/converter.go:275-286`
- **相对上游**: 本地新增
- **问题**: 每转生成新 `agentContinuationId`，仅 `conversationId` 来自 session。依赖客户端 session 与 sticky；无上游 continuation 句柄复用。
- **影响**: 与「会话亲和」产品预期可能不符；属设计选择，记录为分叉认知点。
- **建议**: 若上游支持，将 continuation id 绑定 sticky session store。
- **交叉关注**: M06

## 与上游合并风险
- **冲突热点文件**
  - `backend/internal/service/antigravity_gateway_service.go`（体积大、retry/credits/sticky 交织）
  - `backend/internal/service/gemini_messages_compat_service.go`（cooldown/平台分流）
  - `backend/internal/handler/gateway_handler.go` / `openai_gateway_handler.go`（failover/partial billing 与多平台分流）
  - `backend/internal/config/config.go`（`gateway.grok.*`、`antigravity_fallback_*` 配置块）
  - `backend/internal/pkg/apicompat/**`（M03 边界，但 Grok/Claude 协议桥接共用）
- **语义漂移点（同名不同义）**
  - OpenAI `http_incremental_continuation_enabled`（默认 false） vs Grok `gateway.grok.http_active_delta_enabled`（默认 true）——都叫 active-delta，开关与 store 策略不同
  - `ForceCacheBilling` / sticky：Antigravity 账号切换与 OpenAI previous_response sticky 语义不同
  - Kiro fake `cache_creation`/`cache_read` 为模拟值，非上游真实 prompt cache
  - `temp_unsched` reason keyword：`kiro_first_event_timeout` / `kiro_transport_failure` / `kiro_generation_failure` 与通用 rate-limit 并存
  - Grok token refresh skew=1h vs 其他平台常见分钟级

## 测试与验证缺口
- **已有较强覆盖**: Kiro first-event threshold/profile exclusion；fake-cache scaling/progress CAS；Grok OAuth TryConsume 并发；Kiro IDC issuer allowlist；Antigravity single-account retry / internal500；Grok active-delta 单测文件存在
- **缺口**:
  1. Kiro native web continuation **失败 + 已写流** 的计费/非 failover 契约测试（对应 P1）
  2. Grok request-path refresh：skew 边界、刷新失败但 token 未过期的行为
  3. Antigravity capacity cooldown 跨实例（或至少抽象 store 接口可测）
  4. Grok probe 错误状态码不覆盖有效 snapshot 的回归测试
  5. fake-cache 多实例/无 sticky 下账单抖动的集成场景（可文档化接受范围）
  6. 与 `upstream/main` 的三方 merge dry-run（antigravity_gateway / gateway_handler）未在本 review 中执行

## 模块结论
- **整体风险评级: High**
- **是否建议合入上游 / 继续分叉 / 先修再合**: **先修再合（至少清 P1）**；该模块是相对上游的最大功能分叉面之一，Kiro/Grok 几乎为本地平台能力，直接合入上游冲突与语义漂移成本高。建议：
  1. 先修复计费/刷新/多实例冷却三类 P1
  2. 将 Kiro/Grok 以清晰边界模块化（减少改动 `gateway_handler` 中枢）
  3. 再与上游协商 cherry-pick 非平台私有的通用改进（OAuth localOnly、telemetry sanitizer 等）
- **Top 3 必须处理项**
  1. **Kiro continuation 失败漏计费**（流已提交却 `result=nil` + failover 早退）
  2. **Grok token 1h 刷新窗口**导致热路径刷新风暴与错误冷却
  3. **Antigravity capacity cooldown 进程内 map** 在多副本下失效
