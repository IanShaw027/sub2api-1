# Sub2API 近 3 天改动深度审计报告

| 字段 | 值 |
|------|-----|
| **报告日期** | 2026-07-14 |
| **分支** | `personal-dev` |
| **Base** | `e316ebf52838a89d57fc790981cce7520f819ac8` (`e316ebf52`) |
| **Head** | `b098f7dc6fe3decfcfce639b7114082eee465e56` (`b098f7dc6`) |
| **窗口** | 约近 3 天（含大量 upstream merge） |
| **提交量** | ~224 commits |
| **变更路径** | ~694+ unique paths（全量 diff 含 generated/ent 可达 2000+） |
| **审计模式** | 只读静态深审 + 主会话对 P0 项源码交叉核验 |
| **方法** | 10 个领域子代理并行审查，主代理汇总与复验 |

---

## 1. 执行摘要

### 1.1 一句话结论

近 3 天改动**工程成熟度明显上升**（双扣防护、WS/HTTP partial billing、scheduler lag/coalesce、OAuth Redis 单次 consume、Grok active-delta 安全模型、支付 channel 删除、admin bulk 多平台拦截等），但存在若干 **资金双扣 / JWT 外泄 / 漏计费 / 连接假探活 / 多副本调度水位回退** 类问题，**不建议在未处理 P0 的情况下作为对外发布基线**。

### 1.2 严重度统计（去重后）

| 等级 | 数量 | 含义 |
|------|------|------|
| **P0 阻塞** | 5 | 资金损失、会话劫持、系统性漏计费、生产连接风暴、多副本数据面不一致 |
| **P1 应修** | 22 | 默认开关、OAuth 缺口、协议 wire 偏差、运维 footgun、安全收口残留 |
| **P2 改进** | 18+ | 文案过时、测试缺口、文档不一致、边界体验 |

### 1.3 发布建议

| 场景 | 建议 |
|------|------|
| 内部 dev / 个人环境 | 可继续迭代，但 P0-1/P0-2 建议立刻修 |
| 多副本生产 | **阻塞**：P0-4、P0-5 未修前多副本风险高 |
| 对外 SaaS / 收款上线 | **阻塞**：P0-1、P0-2、P1 Stripe secret 未修前不建议 |
| 仅 OpenAI 网关热修 | 至少先修 P0-3、P0-4 |

### 1.4 审计范围与方法

#### 十路子代理分工

| # | 领域 | 焦点 |
|---|------|------|
| 1 | Grok / xAI | active-delta、free 24h、SSO→Build、billing UI、base_url |
| 2 | Billing / Usage | 双扣、Skill 结算、长上下文、余额缓存/outbox |
| 3 | OpenAI / Codex 网关 | keepalive、WS、partial billing、tool ID、manifest |
| 4 | Scheduler / 账号池 | outbox lag、rebuild coalesce、auto-pause、proxy expiry |
| 5 | OAuth / Auth / Session | consume、redirect、凭证脱敏、API key、CORS |
| 6 | Media / Image / Video | 尺寸计费、终态归一、Grok media、hosted tool |
| 7 | apicompat / 多协议 | stop_reason、namespace tools、cache、Kiro |
| 8 | Infra | H2 keepalive、cache fence、WS probe、TLS fingerprint |
| 9 | Security / Ops / Deploy | 支付泄露面、迁移、Apple container、审核、secret_scan |
| 10 | Frontend | 嵌入页 JWT、支付、bulk-edit、i18n、DataTable |

#### 主会话交叉核验（P0）

以下项已在主工作区直接读源码确认，**非仅依赖子代理自报**：

| 项 | 核验结果 |
|----|----------|
| Skill `ChargeUserBalance` 忽略 Reference | **确认**：`wire.go` 仅 `DeductBalance` |
| 结算注释承诺幂等 | **确认**：`ai_skill_settlement_service.go:92-95` |
| embed URL 写入 `token` query | **确认**：`embedded-url.ts:29-30` |
| Images partial 用 `ImageCount > 0` | **确认**：`openai_images.go:361-368` |
| WS prewrite ping 无 capability 门闩 | **确认**：`shouldOpenAIWSSessionPrewritePing` 只看 idle |
| Outbox watermark 普通 SET | **确认**：`scheduler_cache.go:495-497` |
| Grok active-delta 默认 true | **确认**：`config.go` viper default `true` |

#### 局限

- 本报告为**静态深审**，未在本轮重跑全量 `go test -tags=unit ./...` / `pnpm test:run` / 双实例竞态压测。
- 窗口含大量 **upstream merge**，部分问题可能是合并前技术债；结论锚定 **当前 HEAD 行为**。
- 子代理对个别路径无 shell `git diff` 时，以当前工作树实现为准。

---

## 2. 变更主题地图

| 主题 | 代表提交/方向 | 主要路径 |
|------|----------------|----------|
| Grok OAuth HTTP active-delta | 设计文档 + 实现默认开启 | `openai_gateway_grok_active_delta.go`, `docs/superpowers/specs/*active-delta*` |
| Grok free 24h / billing usage | rolling 24h 估算、官方 billing 拉取 | `account_usage_service.go`, `grok_quota_service.go`, `AccountUsageCell.vue` |
| Grok SSO → Build OAuth | SSO device 转换、去 raw SSO 落库 | `pkg/xai/sso_device.go`, CreateAccountModal |
| OpenAI 图片 keepalive / 终态 | nonstream keepalive、status→completed | `openai_images*.go`, `openai_gateway_service.go` |
| WS lifecycle / preemption | ingress lease、session preemption | `openai_ws_*.go` |
| HTTP/2 keep-alive PING | Codex 死连接驱逐 | `http_upstream.go` |
| 双扣 / 余额 defer | usage_billing_dedup、中间件不硬拦余额 | `usage_billing_repo.go`, `api_key_auth.go` |
| AI Skill 计费归因 | marketplace settle + token RecordUsage | `ai_skill_settlement_service.go` |
| Scheduler 减负 | lag 修复、不再对 expiry full_rebuild | `scheduler_snapshot_service.go` |
| OAuth 多副本 session | Redis TryConsume | `redissession/store.go` |
| 支付 channel 删除 | 防内部 AI 渠道配置泄露 | `routes/payment.go` |
| Admin bulk 护栏 | 禁混合平台 model mapping | `admin_service.go`, BulkEditAccountModal |
| 前端支付 / 工单 / Skill 市集 | 大量 views + specs | `frontend/src/views/**` |
| Ops host 过滤 / Server-Timing | 可观测性 | `ops_repo.go`, `servertiming/**` |

---

## 3. P0 阻塞问题（必须修）

### P0-1. AI Skill 结算：`ChargeUserBalance` 无 Reference 幂等 → 可双扣

| 字段 | 内容 |
|------|------|
| **领域** | Billing |
| **严重度** | P0 — 真实资金 |
| **置信度** | 高（源码交叉核验） |

**现象**

结算层明确假设扣费按 run reference 幂等：

```go
// backend/internal/service/ai_skill_settlement_service.go
// Stale pending (crash after charge / before status flip) can be reclaimed;
// ChargeUserBalance is keyed by run reference and must be idempotent.
```

随后使用 `ai_skill_run:<runID>` 再次扣费，但生产实现**完全不读 Reference**：

```go
// backend/internal/service/wire.go — aiSkillBalanceCharger.ChargeUserBalance
if err := c.userRepo.DeductBalance(ctx, input.UserID, input.Amount); err != nil {
    return nil, err
}
```

`skillkit` 适配器同样无幂等。

**复现路径**

1. `Settle` 成功扣买家余额
2. 在 `UpdateSettlement(status=settled)` 前进程崩溃（代码已有 3 次重试，失败仍可返回 error）
3. 行保持 `pending`
4. `time.Since(UpdatedAt) >= 2m` 后 reclaim → **再次 DeductBalance**

`run_id` 唯一索引只防重复 settlement 行，**不防重复扣余额**。现有测试覆盖 settled 短路，**未**覆盖 charge 成功 + pending 卡住 + 重放的金额断言。

**影响**

市场 skill 按次费双扣（真实资金）。

**修复建议**

1. 引入 `balance_ledger` 或复用 `usage_billing_dedup` 同类表：`UNIQUE(reference)`，同事务 claim + deduct。
2. 或 pending reclaim 前查 ledger，已 charge 则跳过扣费只修 status。
3. 单测：`charge 成功 → UpdateSettlement 失败 → 2min 后 Settle → 余额只减一次`。
4. stub 必须按 `Reference` 去重，避免假绿。

---

### P0-2. 自定义嵌入页把 JWT 明文塞进第三方 URL

| 字段 | 内容 |
|------|------|
| **领域** | Frontend / Security |
| **严重度** | P0 — 会话劫持 |
| **置信度** | 高（源码交叉核验） |

**证据**

```ts
// frontend/src/utils/embedded-url.ts
if (authToken) {
  url.searchParams.set(EMBEDDED_AUTH_TOKEN_QUERY_KEY, authToken) // key = "token"
}
```

`CustomPageView.vue` 将结果用于 `<iframe :src>` 与新标签打开，且 **无 sandbox**（对比 HomeView 有 sandbox）。

**影响**

- 任意管理员配置的 `custom_menu_items.url` 可收到用户 JWT + `user_id` + 完整 `src_url`。
- 泄漏面：第三方 access log、Referer、浏览器历史、“在新标签打开”。
- 持有 JWT 可完整冒充用户直至 token 失效/吊销。

**修复建议**

1. **禁止** session JWT 进 query。
2. 改 short-lived embed ticket / postMessage 握手 / 后端代理。
3. iframe 默认 sandbox + 域名 allowlist。
4. 审计历史菜单 URL 是否已收到 token。

---

### P0-3. Images 路径 partial error 计费口径落后

| 字段 | 内容 |
|------|------|
| **领域** | OpenAI Gateway / Media |
| **严重度** | P0 — 系统性漏计费 |
| **置信度** | 高（源码交叉核验） |

**现象**

Responses / Chat / WS 已统一为：**只要有 partial result 就记账**。Images 仍是：

```go
// backend/internal/handler/openai_images.go
if result != nil && result.ImageCount > 0 {
    // warn 后继续，可记账
} else {
    // ensureForwardErrorResponse + return → 漏计费
}
```

当 `result != nil && ImageCount == 0` 但已有 input/output tokens（OAuth SSE 中途失败、client disconnect 后 drain 到 usage）时，**不记账**。

**影响**

图片 OAuth/流式失败路径可白嫖上游 token；与其他端点行为不一致。

**修复建议**

与 Chat 对齐：`result != nil` 即记账（cyber 除外）。补单测：token>0、ImageCount==0、forward error → `RecordUsage` 被调用。顺带放宽 client disconnect 后的 interval timeout，避免截断 drain。

---

### P0-4. Codex WS session prewrite ping 可误杀健康连接

| 字段 | 内容 |
|------|------|
| **领域** | Infra / OpenAI WS |
| **严重度** | P0 — 生产连接健康 |
| **置信度** | 高（源码交叉核验） |

**现象**

- `coderOpenAIWSClientConn.SupportsIdlePingWithoutReader() == false`（Ping 等 pong，控制帧只由 Read 消费）。
- 背景 sweep 与 multi-turn preflight **尊重**该门闩。
- **session prewrite** 路径不尊重：

```go
// shouldOpenAIWSSessionPrewritePing — 只检查 OAuth + Reused + session-bound + idle
return lease.ConnIdleDuration() >= threshold
// 调用处直接 PingWithTimeout → 失败则 MarkBrokenFor("prewrite_ping_fail")
```

**影响**

默认 coder/websocket 路径上，复用 session idle 超过阈值后可**确定性** ping 超时，驱逐健康连接，触发 fallback/重连风暴，污染 “upstream flaky” 指标。

**修复建议**

1. prewrite 增加 `lease.SupportsIdlePingWithoutReader()` 门闩。
2. 单测：coder-like conn idle 过阈值 → **不**调用 Ping / 不 MarkBroken。
3. 中期：idle-ping 改为 opt-in；H2 实现真正 ping-wait-pong 或 HTTP/2 PING。

---

### P0-5. Scheduler outbox 多副本：水位可回退 + 锁 miss 当成功

| 字段 | 内容 |
|------|------|
| **领域** | Scheduler |
| **严重度** | P0 — 多副本一致性（单实例风险较低） |
| **置信度** | 高（控制流交叉核验） |

**现象 A — 水位非单调**

```go
// backend/internal/repository/scheduler_cache.go
func (c *schedulerCache) SetOutboxWatermark(ctx context.Context, id int64) error {
    return c.rdb.Set(ctx, schedulerOutboxWatermarkKey, strconv.FormatInt(id, 10), 0).Err()
}
```

每个进程都启动 outbox worker，无 leader lock。慢实例可在快实例推进后 `SET` 更小水位 → **回退**。

**现象 B — rebuildBucket 锁 miss 当成功**

拿不到 bucket 锁时 `return nil`，outbox 视为 handled，watermark 仍可推进 → 实际未重建。

**影响**

- 事件重复处理 → rebuild 风暴、lag 指标失真。
- 更糟：水位越过未真正应用的 full_rebuild / bulk 工作 → 调度快照长期陈旧。
- 与 auto-pause/proxy 已改为 targeted 事件的收益部分抵消。

**修复建议**

1. 单 leader 消费（Redis/PG 锁），或
2. 水位 Lua：`SET only if new > old` + fencing token。
3. 锁 miss 不得 Ack；指标 `scheduler_rebuild_lock_miss_total`。

---

## 4. P1 重要问题

### 4.1 资金 / 计费

#### P1-B1. 网关扣费允许透支；预检与落库不一致

预检：`balance <= 0` 或 `< MinimumBalanceReserve` 拒绝。
落库：条件扣减失败后**无条件再扣**，余额可负（测试明确断言 overdraft）。

这是产品取舍（先服务后结算），但与 “conservative reserve” 叙事冲突：预检不挡并发超卖。

**建议：** 明确产品策略；若不允许透支则 hard fail；若允许则监控 `BalanceOverdrafted` 与负余额告警。补 N 并发 `Apply` 压测（`-count=30`）。

#### P1-B2. Balance cache outbox 单点失败 Nack 整批

`InvalidateUserBalance` 循环中任一用户失败，对**整批** claimed ids Nack。已成功 invalidate 的用户也被重新排队 → Redis DEL 风暴 + 充值后短暂仍拒。

**建议：** per-event Ack/Nack。

#### P1-B3. `ImageOutputSizes` 主路径未接线 → output 优先计费多数失效

`RecordUsage` 会 `ApplyOpenAIImageBillingResolution`，单测要求 output 4K 压过 input 1K。但 Images API / OAuth images / streaming responses 组装结果时几乎不写 `ImageOutputSizes`（主要只有 Grok media / WS bridge 写）。

**影响：** 请求 1K/auto、上游吐 4K 时系统性少收。

**建议：** 所有生图成功路径接 `imageCounter.Sizes()`。

#### P1-B4. Free Grok 24h 额度只 UI 估算、不进调度

`GrokLocalUsage24h` + 前端硬编码 `GROK_FREE_TOKEN_LIMIT = 2_000_000` 仅展示。Gateway 仍调度该账号直至上游 429。

**影响：** 共享池 Free 账号被打穿 → 级联 429。

**建议：** 超阈值 temp-unsched；2M 前后端共享可配置常量。

#### P1-B5. Stripe `client_secret` 进入前端路由 query

`PaymentView` 将 `client_secret` 放入 `/payment/stripe` query；路由 `requiresAuth: false`。

**影响：** history / 分享链接 / 代理日志泄漏支付意图密钥。

**建议：** 仅 `order_id + resume_token` 进 URL；secret 放 sessionStorage 或 resume 接口返回；进入后 `replace` 清 query。

#### P1-B6. `WebSearchCalls > 0` 分支只计搜索费

`openai_gateway_service` 在 `WebSearchCalls > 0` 时直接 `CalculateWebSearchCost`，不 merge token。Alpha search 当前无 token 则正确；未来扩展易系统性漏计 token。

#### P1-B7. AI Skill token 计费与 marketplace 结算解耦

Runtime 先 `RecordUsage`，再 `Settle`。Settle 失败：token 已扣、run 标 failed。资金路径无统一事务/补偿。

---

### 4.2 Grok / xAI

#### P1-G1. Active-delta 默认 `enabled=true`，与设计 Task 0 冲突

设计要求：未完成 Task 0 探针前应默认 `false` 或 shadow。
实现：`viper.SetDefault("gateway.grok.http_active_delta_enabled", true)`，`require_store_on_create` 默认 false。

**风险：** store=false + previous 语义若不成立 → full-replay 放大、二次上游调用窗口。

#### P1-G2. OAuth `base_url` 显式 pin 静默丢弃

`GetGrokBaseURLOr`：`ValidateTrustedBaseURL` 失败则静默回退默认，无 error/log。第三方/自定义上游配置形同虚设。

#### P1-G3. Free-tier cache 工具注入仅 Chat bridge

Responses / Claude messages 路径 `applyGrokResponsesCacheIdentity(..., false)` 不注入 → Free 主路径仍难命中 prompt cache。

#### P1-G4. Active-delta 会话 store 默认进程内

多副本命中率接近 0，仍有 build/evaluate 开销。需文档化 Redis 共享 store 前提。

#### P1-G5. Grok 429 第一次 account switch 即停止 failover

避免 storm 的意图正确，但池内其他健康账号无法承接同请求。

#### P1-G6. SSO device 硬编码 DefaultClientID；password 路径依赖 YesCaptcha + 明文密码进 admin API

---

### 4.3 OAuth / Auth / Session

#### P1-A1. Memory 模式 TryConsume 非单次占用

Claude / OpenAI / Antigravity：无 Redis 时 “consume” 仅检查 session 仍存在，**不原子 claim**。xAI/Kiro 内存路径有 mutex consumed 标志。

#### P1-A2. Gemini OAuth：无 Redis share、无 single-use consume

多副本 session not found；同副本并发 double-redeem。

#### P1-A3. Grok redirect_uri 仍可在 exchange 时被请求覆盖

OpenAI 已 allowlist + exchange 只用 session。Grok 仍接受 `input.RedirectURI` 覆盖。

#### P1-A4. Grok state 对 bare code 可选；失败仍 Delete session

CSRF 绑定弱于 peer；错误 state 可 burn 合法 session（DoS 面，session_id 熵缓解）。

#### P1-A5. Claude exchange 无 client state 校验

依赖 admin-only + PKCE + secret session_id，弱于 OpenAI/Antigravity。

#### P1-A6. 凭证脱敏列表 vs update-merge 敏感 key 不同步

DTO redact 含 `id_token`、`session_key`、`private_key`、AWS/SA keys；
`isSensitiveCredentialKey`（update merge）缺多项。注释要求同步，实际未同步 → 空 patch 可清密钥。

#### P1-A7. Redis Session Set 失败被 `_ =` 吞掉

多实例 OAuth split-brain。

#### P1-A8. Google `?key=` 仍接受于 `/v1beta*`

Keys 进 query → access log / Referer。

---

### 4.4 OpenAI / Codex / Media / 协议

#### P1-O1. H2 WebSocket Ping 只写不读 pong

`openAIWSH2ClientConn.Ping` 写 opcode 0x9 即返回；未实现 `SupportsIdlePingWithoutReader` 时默认当可 idle ping → 假活。

#### P1-O2. 跨实例 WS preemption claim 非原子

GET previous + plain SET；释放有 Lua CAD。双活窗口约 watch interval（2s）。长 turn 无 TTL heartbeat。

#### P1-O3. Images JSON keepalive 后通用 `errorResponse` 未对称处理

Compact SSE 有 committed 分支；Images JSON 心跳提交 200 后走通用 error 可能 body 混杂。

#### P1-O4. Chat→Responses finalize 对 incomplete 仍发 `response.completed` 事件类型

`status=incomplete` 但 `Type=response.completed`。测试锁定错误 wire。Codex 严格客户端可能误判成功。

#### P1-O5. Anthropic→Chat 丢 refusal / content_filter

`anthropicStopReasonToCC` 无 `refusal` → 默认 `stop`。安全过滤像正常完成。

#### P1-O6. Namespace tools 未在 Responses→Anthropic 展平

Chat 轴有 flatten/restore；Anth 轴 namespace 进 default pass-through，children 丢弃。

#### P1-O7. 非流 JSON / passthrough 未做 image_generation 终态归一

流式已 `normalizeCompletedImageGenerationStatus`；真 JSON passthrough 原样写出 → SDK 见 `generating`+result 可能挂起。

#### P1-O8. Grok video edits/extensions 缺请求规范化

计费路径含 edits/extensions；OpenAI create 形态归一只对 `/v1/videos`。edit 参数易 400。

#### P1-O9. Codex bridge：不注入 hosted tool 正确，但 instructions 仍可能对 namespace 误导

`hasOpenAIImageGenerationTool` 把 namespace 当 image tool → 仍附加 “native image_generation tool attached” 文案。

#### P1-O10. Videos failover 不检查 writer size

Images 有 size 快照防 double-write；Videos 直接换号。

---

### 4.5 Scheduler / Infra

#### P1-S1. full-rebuild coalesce 仅进程内

多副本仍可并发 full rebuild；叠加 P0-5。

#### P1-S2. Snapshot 单账号 meta 洞 → 整 bucket miss

nil meta 导致整快照 miss → DB fallback 风暴。

#### P1-S3. Auto-pause / proxy-expiry outbox enqueue 失败仅 log

有 immediate snapshot + `IsSchedulable` 兜底；group 成员滞后仍可能。

#### P1-S4. `account_bulk_changed` 不 dedup

sweep 风暴可再推高 outbox。

#### P1-S5. H2 keepalive 仅标准 OpenAI H2

指纹 H2 禁用 keep-alive（有意）；Claude/Gemini 默认 H2 仍可能 NAT 死连接。

#### P1-S6. Concurrency slot release / heartbeat renew 失败仅 log

Redis 抖动下 slot 泄漏至 TTL。

---

### 4.6 Frontend / Security / Ops

#### P1-F1. Airwallex recovery 漏 `airwallex_route`

`isPaymentLaunchKind` 未包含该值 → 刷新/回跳恢复分支错误。

#### P1-F2. 自定义 Markdown 允许 `iframe` 标签

DOMPurify `ADD_TAGS: ['iframe']` → 管理员内容嵌任意外域 iframe。

#### P1-F3. Bulk-edit filtered 预览按 100/页拉全量只为算 platform

大过滤集管理端卡顿 / API 放大。

#### P1-X1. 匿名 `POST /payment/public/orders/verify` 残留

响应已最小字段；仍可探测状态，expired 路径可触发上游对账。

#### P1-X2. Migration checksum 兼容白名单膨胀

正确兼容历史误改，但制度上易变成“可改已应用迁移”后门。

#### P1-X3. Migration 注释 strip 仅整行 `--`

`/* */` / 完整 SQL tokenizer 缺失；当前多 fail-closed。

#### P1-X4. Content moderation 把 keyword/excerpt 打进 slog + DB

脱敏 ≠ 去 PII；审计 retention 风险。

#### P1-X5. Apple container 默认 `BIND_HOST=0.0.0.0`

ACCESS_HOST 打印 127.0.0.1 易误判仅本机。

#### P1-X6. secret_scan 覆盖面窄

仅 private key / GitHub / Stripe / sk- / AIza；缺 AWS、DSN、JWT 熵等。

#### P1-X7. CRS capture proxy 默认 loopback 好，body 仍落盘

`--listen 0.0.0.0` 误用风险。

---

## 5. P2 改进项（摘要）

| ID | 说明 |
|----|------|
| P2-1 | 前端 i18n 仍写 “Grok OAuth concurrency limited to 1”，后端已放开 |
| P2-2 | Free 判定前后端不完全一致（tier free vs billing bars） |
| P2-3 | Grok baseURLAllowedHosts 命名与 third-party 放宽语义漂移 |
| P2-4 | Plan-gated cooldown 短语匹配脆；需 metric |
| P2-5 | Kiro DefaultModels 未列 sonnet-5；1M budget 实为 900k |
| P2-6 | apicompat 死字段 `CurrentToolName` 等未清理 |
| P2-7 | `pause_turn` → Responses completed，客户端难区分 |
| P2-8 | Anthropic→Responses 声称保留 cache_control，工具/块上实际丢失 |
| P2-9 | FinalizeAnthropicResponsesStream 忽略已解析 stop_reason |
| P2-10 | 混合图片尺寸按最高 tier × count 整单计价（可能 overbill） |
| P2-11 | Keepalive 200 + error body：运维打开配置后 status 语义变化需文档 |
| P2-12 | Pool-mode retry 本地循环 vs FailoverState 双实现 |
| P2-13 | 前端 PaymentChannel 死类型残留 |
| P2-14 | Ops list host 查询长度未 bound（admin-only） |
| P2-15 | Skill 路由 `editable/owned` 仅 UX 门闩，依赖后端强制 |
| P2-16 | confirmSubscribe 未与 recharge 对称检查 canSubmit |
| P2-17 | Server-Timing 启用后 admin 路径对未认证请求也建 collector（无 header 泄露，仅开销） |
| P2-18 | 生成 fence key 24h TTL 边界（低概率） |

---

## 6. 分领域详细评估

### 6.1 Grok / xAI — 质量：中上

**亮点**

- Active-delta 安全模型：strict prefix、in-flight 互斥、strip previous、tool-output fail-closed。
- 单一 HTTP egress `callGrokResponsesHTTP`。
- Billing 主动拉取 + weekly/monthly 对齐。
- SSO→Build 后不落库 raw `sso_token`。
- Media 强制官方 API，与 CLI text 分离。
- SearchCount 按实际 tool call 计费。

**主要风险**

P1-G1~G6、P1-B4、P0 无直接 Grok 独有资金洞（free 调度属容量风险）。

### 6.2 Billing / Usage — 质量：网关强 / Skill 弱

**亮点**

- `usage_billing_dedup` + fingerprint + archive 防重放。
- 余额 generation fence。
- 长上下文 bool 约束 + 真实金额断言。
- 余额检查 defer 到 eligibility，中间件不误杀。

**主要风险**

P0-1 是本窗口**最高危资金问题**；另见 P1-B1~B7。

### 6.3 OpenAI / Codex 网关 — 质量：高，有已知洞

**亮点**

- Keepalive 与 failover 字节口径拆开。
- WS/HTTP partial billing 骨架（Images 除外）。
- Committed error 防双终态。
- Tool-call ID 前缀、plan-gated cooldown、manifest SWR。

**主要风险**

P0-3、P0-4、P1-O1~O3。

### 6.4 Scheduler — 质量：单实例优，多副本危

**亮点**

- Pending lag 用未消费事件（有测试）。
- Full rebuild trailing coalesce（进程内）。
- Auto-pause/proxy 不再 full_rebuild。
- Snapshot CAS 版本激活。

**主要风险**

P0-5、P1-S1~S4。

### 6.5 OAuth / Auth — 质量：Redis 路径好，移植不全

**亮点**

- Redis TryConsume SET NX。
- OpenAI redirect 教科书级修复。
- API key 拒 query `api_key`；CORS `*` 强制无 credentials。
- DTO 凭证 redact。

**主要风险**

P1-A1~A8。

### 6.6 Media — 质量：keepalive 优，计费采集差

**亮点**

- JSON keepalive adjusted size + 回归测。
- 流式终态归一矩阵。
- Grok media 官方 API 路由。
- Hosted vs namespace 注入侧测试。

**主要风险**

P1-B3、P1-O7~O10。

### 6.7 apicompat — 质量：高，wire 有缺口

**亮点**

- Cache 别名 round-trip。
- Anth→Res max_tokens→incomplete。
- Read 工具 delta 即时推送。
- Kiro fake-cache growth 缩放。
- Anthropic OAuth exact model。

**主要风险**

P1-O4~O6、P2-8/9。

### 6.8 Infra — 质量：高

**亮点**

- OpenAI H2 ReadIdleTimeout 针对 NAT 死连接。
- Generation-fenced cache。
- Concurrency heartbeats。
- TLS account 隔离 key。
- Ops per-attempt failure sink。
- Server-Timing 默认关 + admin 门闸。

**主要风险**

P0-4、P1-O1、P1-S5/S6。

### 6.9 Security / Ops / Deploy — 质量：主路径加固，收口未完

**亮点**

- Payment channels 删除完整。
- 迁移 advisory lock、notx 限制、checksum 基线。
- Bulk mixed-platform credentials 硬拦。
- Apple container ownership/lock/600 env。

**主要风险**

P1-X1~X7；本领域无 P0。

### 6.10 Frontend — 质量：产品面成熟，有安全阻塞

**亮点**

- 支付 recovery 状态机模块化。
- Bulk-edit 前后端双重护栏。
- DataTable 虚拟滚动根因修复。
- i18n 静态 key 对账。
- 测试密度高于“堆页面”。

**主要风险**

P0-2、P1-B5、P1-F1~F3。

---

## 7. 交叉问题矩阵（跨领域）

| 问题簇 | 涉及领域 | 说明 |
|--------|----------|------|
| **幂等 / 双扣** | Billing, Skill | 网关 dedup 强；Skill charge 假幂等 |
| **Partial billing 一致性** | OpenAI, Media, WS | Chat/Res/WS 已对齐；Images 落后 |
| **探活语义** | WS pool, H2, prewrite | 三处策略不一致 → 假死/假活 |
| **多副本一致性** | Scheduler outbox, OAuth session, WS preempt, active-delta store | 进程内正确 ≠ 集群正确 |
| **OAuth redirect/state** | OpenAI 已修，Grok/Claude/Gemini 参差 | 安全基线未统一 |
| **凭证敏感列表** | Redact vs merge | 单一事实源缺失 |
| **Free 额度** | Grok usage UI vs 调度 | 展示与控制分裂 |
| **支付 secret 生命周期** | Frontend URL, 公开 resume API | query 反模式 + 匿名探测残留 |

---

## 8. 推荐修复路线图

### 阶段 0 — 立即（1–2 天，阻塞发布）

| 序 | 项 | 负责暗示路径 |
|----|-----|----------------|
| 1 | P0-1 Skill charge 幂等 + 金额回归 | `ai_skill_settlement_service.go`, `wire.go` |
| 2 | P0-2 去掉 embed JWT query + sandbox | `embedded-url.ts`, `CustomPageView.vue` |
| 3 | P0-3 Images partial 计费对齐 | `handler/openai_images.go` |
| 4 | P0-4 prewrite ping capability 门闩 | `openai_ws_forwarder.go` |
| 5 | P1-B5 Stripe secret 出 URL | `PaymentView.vue`, `StripePaymentView.vue` |

### 阶段 1 — 本周（多副本 / 安全基线）

| 序 | 项 |
|----|-----|
| 6 | P0-5 Scheduler leader + 单调水位 |
| 7 | P1-A1/A2 统一 consume + Gemini Redis |
| 8 | P1-A3/A4 Grok redirect + 强制 state |
| 9 | P1-A6 敏感凭证单一事实源 |
| 10 | P1-G1 active-delta 默认 false 或完成 Task 0 记录 |
| 11 | P1-B4 Free 24h 进调度 |
| 12 | P1-B3 ImageOutputSizes 全路径 |

### 阶段 2 — 两周内（协议与体验）

| 序 | 项 |
|----|-----|
| 13 | P1-O4/O5/O6 apicompat wire 缺口 |
| 14 | P1-O1/O2 H2 ping 与 preemption 原子 claim |
| 15 | P1-O7/O8/O9 media 终态与 video edit 归一 |
| 16 | P1-B2 outbox per-event Ack |
| 17 | P1-F1 Airwallex recovery |
| 18 | P1-X1 匿名 verify sunset |

### 阶段 3 — 持续治理

- Migration whitelist 冻结流程
- secret_scan 扩展
- 透支策略产品化 + 指标
- 双实例竞态测试（outbox、preempt、OAuth consume）
- 清理过时 i18n / 死类型

---

## 9. 验证清单（修复后必跑）

### 后端

```bash
cd backend
go test -tags=unit ./internal/service/ -count=1
go test -tags=unit ./internal/handler/ -count=1
go test -tags=unit ./internal/repository/ -count=1
go test -tags=unit ./internal/pkg/apicompat/ -count=1
# 并发敏感
go test -tags=unit ./internal/service/ -run 'Settlement|Outbox|Preempt|Watermark|TryConsume' -count=30
```

### 前端

```bash
cd frontend
pnpm run test:run
pnpm run typecheck
pnpm run lint:check
```

### 专项场景（手工/集成）

| 场景 | 期望 |
|------|------|
| Skill settle 中途 kill → 2min 后重放 | 余额只减一次 |
| Custom page 第三方 iframe | URL **无** token |
| Images OAuth 中途断流 token>0 ImageCount=0 | 记账 |
| Codex WS session idle > threshold | **不**因 prewrite ping 断连 |
| 双实例 outbox 并发 | 水位单调、无长期漏 rebuild |
| Stripe 支付跳转 | URL 无 client_secret |
| Grok free 超 2M | 不再被调度（修 P1-B4 后） |

---

## 10. 做得好的地方（应保留的模式）

1. **网关计费幂等**：`(request_id, api_key_id)` + fingerprint + archive — Skill 应复用此模式。
2. **Keepalive 与 failover 字节口径拆分**：`AdjustedWrittenSize` 排除心跳 — 多路径应统一。
3. **Generation-fenced cache invalidation**：Lua INCR+DEL + CAS fill。
4. **OAuth Redis TryConsume**：多副本正确原语；应移植到所有 provider 内存路径。
5. **OpenAI redirect allowlist + exchange 忽略 caller URI**：Grok 应对齐。
6. **Scheduler pending lag 语义**：用未消费事件 — 测试锁定意图。
7. **Active-delta fail-closed 安全模型**：默认开关问题外，设计内核扎实。
8. **Admin bulk 多平台 credentials 硬拒绝**：符合 AGENTS 约束。
9. **支付 channel 删除 + 最小 public verify 字段**：泄露面收敛方向正确。
10. **前端支付状态机 + i18n 静态对账 + DataTable 根因修复**：产品工程化信号强。

---

## 11. 附录

### 11.1 审计元数据

| 项 | 值 |
|----|-----|
| 仓库 | `/Users/ianshaw/Documents/code/personal/sub2api` |
| 模块 | `github.com/Wei-Shaw/sub2api` |
| 报告文件 | `AUDIT_REPORT_2026-07-14.md`（本文件） |
| 审计类型 | Read-only multi-agent deep review |
| 子代理数 | 10 |
| 主会话复验 | P0 全项 + 若干 P1 锚点 |

### 11.2 关键路径速查

| 议题 | 路径 |
|------|------|
| Skill 假幂等 | `backend/internal/service/ai_skill_settlement_service.go` |
| Charge 实现 | `backend/internal/service/wire.go` |
| Embed JWT | `frontend/src/utils/embedded-url.ts` |
| Images partial | `backend/internal/handler/openai_images.go` |
| WS prewrite | `backend/internal/service/openai_ws_forwarder.go` |
| Watermark | `backend/internal/repository/scheduler_cache.go` |
| Active-delta 默认 | `backend/internal/config/config.go` |
| Free 24h | `backend/internal/service/account_usage_service.go` |
| Stripe query | `frontend/src/views/user/PaymentView.vue` |
| Grok redirect | `backend/internal/service/grok_oauth_service.go` |
| 凭证 redact | `backend/internal/service/account_credentials_redact.go` |
| 凭证 merge | `backend/internal/service/account_service.go` |
| Chat incomplete event | `backend/internal/pkg/apicompat/chatcompletions_responses_bridge.go` |
| Image size resolve | `backend/internal/service/image_billing_size.go` |
| H2 keepalive | `backend/internal/repository/http_upstream.go` |
| Payment public | `backend/internal/server/routes/payment.go` |

### 11.3 子代理产出状态

| # | 领域 | 状态 | 产出要点 |
|---|------|------|----------|
| 1 | Grok | 完成 | 3 Critical + 8 Important（汇总后部分降为 P1） |
| 2 | Billing | 完成 | Skill 双扣为最高危 |
| 3 | OpenAI | 完成 | Images partial + H2 ping + preemption |
| 4 | Scheduler | 完成 | 水位 + 锁 miss |
| 5 | Auth | 完成 | consume/redirect/list drift |
| 6 | Media | 完成 | ImageOutputSizes + 终态 + video edit |
| 7 | apicompat | 完成 | incomplete event / refusal / namespace |
| 8 | Infra | 完成 | prewrite ping 与 H2 策略 |
| 9 | Security | 完成 | 无 P0；支付/迁移/deploy |
| 10 | Frontend | 完成 | JWT embed + Stripe secret |

### 11.4 可选后续深度审计（未在本报告展开）

若需要继续加深，建议再派发：

1. **双实例集成竞态专项**（outbox watermark、WS preemption、OAuth consume）— 需可写测试环境。
2. **资金路径端到端账本审计**（usage_log ↔ balance ledger ↔ skill settlement 三表对账脚本）。
3. **OpenAI/Grok 上游真实协议探针**（active-delta Task 0、plan-gated 文案、4K image size 字段）。
4. **前端 XSS/CSP 全量**（所有 `v-html` / marked / DOMPurify 配置矩阵）。
5. **迁移 whitelist 逐条语义等价证明**（历史 checksum 规则逐文件 diff）。

---

## 12. 签署

| 角色 | 说明 |
|------|------|
| 审计编排 | Grok CLI 主代理 |
| 领域审查 | 10 × general-purpose 只读子代理 |
| 交叉核验 | 主代理源码复读 P0 |
| 结论效力 | 静态审计意见；不替代正式安全渗透与账务对账 |

**总体判断：可继续在 `personal-dev` 迭代；对外/多副本发布前必须关闭第 3 节 P0 清单。**
