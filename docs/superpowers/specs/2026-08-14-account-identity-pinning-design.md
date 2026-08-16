# 账号设备档案与个人形态网关（修订稿 v2.1）

**日期:** 2026-08-14  
**状态:** 修订稿 v2.1。已吸收两轮 Claude / GPT 审查，并补上第二轮未闭环项。  
**表名:** `account_device_profiles`（避开已占用的 `auth_identity` / 登录 Identity）  
**约束:** 每账号并发目标 10–15；`max_sessions` 与并发解耦；粘性可逃逸。

审查结论：方向成立，原稿不能开工。本版只改必须修的合同，建议项写入 §13。

## 1. 目标与非目标

**目标：** 每个上游账号呈现为 **一台长期稳定的官方客户端安装**。设备字段、软件包、TLS 模板、出口不闪。同时在飞 turn ≤ 10–15。

**非目标：** 不抄官方正文 / CCH；不写 WAF 绕过；不把多路压进 1 条 thread；不按请求重握手；不把「全天顶满 12 路」伪装成闲聊。

**效果上限：** 设备层高；会话层像重度 IDE；行为层不在范围内。

## 2. 并发、会话、连接（三套数，禁止混用）

| 项 | 合同 | 校验 / 生效范围 |
|---|---|---|
| **同时在飞 turn** | `EffectiveConcurrency()` | 见 §2.1。放行/WaitPlan/WS/探针/HTTP 池 **只认这个函数**，禁止再读裸 `account.Concurrency`。`LoadFactor` 只排序，不放行 |
| **逻辑会话登记** | `max_sessions`，与并发无关 | 0=不限制；1–10000。默认仍只对 Anthropic OAuth/SetupToken 生效（现状）。其他平台要开必须显式 extra |
| **会话空闲** | `session_idle_timeout_minutes` 默认 5 | 1–1440。到期从登记集合摘掉，「用一下就走」不永久占坑 |
| **出站 session : thread** | **1 : 1**（该 session 的 `thread_id` 由其派生，`window_id=thread:0`） | **禁止**把 session A 的请求复用到 session B 的 thread |
| **连接数** | 见 §2.3，**不等于**「永远 1–2 条」 | 与 `transport_family` 绑定 |
| 出口 | 一号一个 `proxy_id` | 禁止按请求轮换 ASN。代理过期 fallback / 灾难恢复可换，必须 drain 旧连接 |
| RPM 粘性缓冲 | 只按并发或手写 | 禁止 `conc + max_sessions`。见 §2.4 |
| 粘性 / 新会话槽位 | 有锚点才粘；TTL 1h（可配 24h） | 见 §2.2：先抢 N；满则同号等 30s（期间只抢普通槽）；截止时仍无普通空位才借 20% 临时槽；Burst 也满才换号 |
| 无明确 session 头 | **保留正文 fallback** | 见 §7 |

不承诺 1000 个会话同时在飞。硬顶是 `BurstConcurrency()`（约 1.2N），日常决策线是 N。

### 2.1 `EffectiveConcurrency()`

唯一入口。禁止业务代码再读裸 `account.Concurrency` 做放行。

```
if concurrency 在 1..32: 用它
if concurrency ≤ 0 或 >32: 用平台默认（不是无限）
平台默认（只用于回退，不改 Ent 列默认值 3）：
  - anthropic / openai 的 OAuth、setup-token：12
  - grok OAuth：1（保持现网 normalize）
  - 其余：3
```

**新建** anthropic/openai OAuth 在 **service 创建路径** 写 12。不改 `ent/schema/account.go` 的 `Default(3)`，不动存量，不改 Grok。导入/批量必须走同一校验，越界拒绝。

`ConcurrencyService.AcquireAccountSlotForGroup`：`maxConcurrency<=0` 改为走 `EffectiveConcurrency()`，fail-closed。  
所有 `account.Concurrency` 直接入槽 / WaitPlan.MaxConcurrency / `tryAcquireAccountSlot` / WS live / `resolvePoolSettings` 的调用点都改走 `EffectiveConcurrency()` 或 `BurstConcurrency()`。`concurrencyService == nil` 的 fail-open 改为 fail-closed。

`LoadFactor` / `EffectiveLoadFactor()`：**只用于候选排序和面板展示**，不参与放行。槽位始终按 N/Burst。若 `LoadFactor < N`，该号会更早排到后面，但满不满只看 `in_flight` 与 N。

```
N      = EffectiveConcurrency()
overflow = max(1, round(N * 0.2))   // 10→2，12→2，15→3
Burst  = N + overflow               // 约 120%
```

H1 的 `maxIdleConns` / `maxIdleConnsPerHost` / `maxConnsPerHost` **三个都用 Burst**，否则临时槽拿到了却进不了 idle，下次重握手。

`LoadRate` 分母 = `N`（不是 Burst）。溢出时面板可以 `>100`。过滤与放行 **禁止**再用 `LoadRate < 100`。

### 2.2 槽位：先等 30 秒普通槽，满点才借临时槽

最佳设计（审查后定稿）：**临时槽是「等满 30 秒仍没有普通空位」的奖赏，不是等待期间的快捷通道。**

每个请求入场只看 **N**，不看 120%。

1. `TryAcquire(N)` 成功 → 普通槽，立刻走。
2. 失败 → 同号等待，**等待期内每次轮询只 `TryAcquire(N)`**，禁止 `TryAcquireBurst`。
3. 30 秒内抢到普通槽 → 走。
4. **30 秒截止必须再打两枪（原子，不能直接当超时）：**  
   先 `TryAcquire(N)`；仍失败再 `TryAcquireBurst(Burst)`。  
   Burst 成功 → 临时槽。Burst 失败（普通+临时都满）→ **本请求换号**。
5. 换号后的账号：**只做即时 `TryAcquire(N)`**，不再各等 30 秒、不再给临时槽（临时槽只在「已经等满 30 秒」的那个号上挣到）。单请求换号总预算建议 30–45s。`PreserveStickyBinding` 与本流程 **同批**，下一请求仍先回原号。
6. 等待队列满 → 也换号，不直接 429。
7. 不健康 / 持久额度尽 / 不可调度 → 立刻换号，不等 30 秒。短时忙（仅并发满）保留绑定。

因此 13/12 时新到的请求仍须自己等满 30 秒，不能立刻再吃临时槽。两个等待者同时在截止点抢 Burst，由 Redis Lua 串行，不会超过 Burst。

统一返回值（scheduler / service / handler 同一套）：

- `AcquiredNormal` / `AcquiredBurst`
- `WaitThenRetry`（仍在 30s 内）
- `SwitchAccountPreserveBinding`（截止两枪都失败，或 wait_queue_full）
- `AbortRequest`（单请求换号预算用尽）
- `InfrastructureError`

**谁换号：** `waitForSlotWithPingTimeout` 在截止两枪失败后返回可识别结果。`streamStarted=false` 时，`failover_loop` 用 `RecordConcurrencyTimeout` 把该号加入 `FailedAccountIDs` 后 `continue`，**禁止** `handleConcurrencyError` 写终态。`streamStarted=true` 时不得换号，才允许 `handleConcurrencyError` 结束本请求。换号接线与 30s 超时 **同批上线**。

**候选池：** 现网 `LoadRate < 100` 会在抢槽前剔除满号，新会话走不到梯子。改为：

- 有 `in_flight < N` 的号 → 新会话 **即时扇出** 到这些号（不傻等）。
- 池内都 `≥ N` 才对选中的号走 30s 梯子（粘性已绑定的号即使 `LoadRate=100` 也必须留在候选里，不能被过滤掉）。
- 过滤条件用 `in_flight < Burst` 判断「还能不能作为换号目标」，不用 `LoadRate < 100`。

`PreserveStickyBinding` **与本槽位流程同批**，禁止留到 P2。选号期 `bindGatewayStickySessionDuringSelection` / `bindOpenAIStickySessionDuringSelection` 在 preserve 时不得 `SetSessionAccountID(新号)`。

实现文件：`concurrency_service.go`、`concurrency_cache.go`（Lua 同一 key 传 N 或 Burst）、`gateway_helper.go`、`failover_loop.go`、`gateway_handler.go`、`gateway_handler_responses.go`、`gateway_handler_chat_completions.go`、`openai_gateway_handler.go`、`gemini_v1beta_handler.go`、`gateway_web_search.go`、`openai_account_scheduler.go`、`gateway_scheduling.go`、`openai_gateway_scheduling.go`。现网 `LoadRate < 100` 过滤点：`gateway_scheduling.go`、`openai_gateway_scheduling.go`。`MaxWaiting` ≥ `max(3, overflow+2)`。

**已开流 / WS：**

- SSE/流 **已经写出首包**（`streamStarted=true`）→ **禁止换号**。截止两枪失败则结束本请求（`AbortRequest`），不得半路切账号。
- 活着的 WS / live lease **计入** 该号 `in_flight`，占 N/Burst，与 HTTP turn 同一把尺。

**换到 B 号时的身份：** 设备/TLS/出口用 **B 的档案**（一号一安装）。session/thread 用 B 的 `session_namespace` 重新派生，表现为 B 上的新窗口，**禁止**把 A 的 thread 带到 B。下一请求仍先回 A（Preserve）。

### 2.3 连接合同

现状 TLS 路径是 H1（`ForceAttemptHTTP2=false`，默认 ALPN `http/1.1`），12 路并发 = 最多 12 条 TCP。原稿「1 主 + 1 备」在 H1 上会把 12 路串行化，删除该措辞。

| `transport_family` | 连接预算 | 何时可用 |
|---|---|---|
| `h1`（默认，P0–P1） | `maxIdleConns` / `maxIdleConnsPerHost` / `maxConnsPerHost` **三个都 = Burst** | 只抬 MaxConns 会让临时槽进不了 idle，下次重握手 |
| `h2` | 1 主 + ≤1 备用；流数承载并发 | 仅当 utls ALPN 含 `h2` **且** transport 真正启用 HTTP/2。Codex/Kiro 若档案声称 h2 但接线未完成，**禁止写出 h2**，降为 h1 |

连接预算 **独立于** 请求并发字段名，但 H1 下数值可以相等。禁止用「请求并发」去证明「只有两条连接」。

隔离与 key（**cacheKey 和 poolKey 都要带 TLS 身份**，只改 poolKey 会重建但仍可能串档案）：

| isolation | cacheKey / poolKey 必含 |
|---|---|
| `account` / `account_proxy`（默认） | `account_id, proxy_id, tls_profile_id, transport_family, Burst` |
| `proxy` | `proxy_id, tls_profile_id, transport_family`；**不按账号调 Burst**（两号 Burst 不同会整池拆建）。不同 TLS 档案仍不得共池 |

跨账号复用断言只在 `account` / `account_proxy` 生效。不在 P0 废弃 `proxy` 隔离。

### 2.4 RPM buffer 与前端回灌

后端：`GetRPMStickyBuffer` =

- extra **显式存在** `rpm_sticky_buffer` 且为 1–10000 → 用它
- 否则 `max(EffectiveConcurrency(), max(baseRPM/5, 1))`
- **禁止**再加 `max_sessions`

前端：`EditAccountModal` **不得**把接口算出来的 buffer 写回 extra。DTO：`base_rpm>0` 时，仅当 extra 带该键才返回手动值。加一次迁移：删除「等于当时 `conc+sess` 或 `base/5`」的自动回写键。`AccountCapacityCell` 显示与后端同一公式。

存量 `concurrency=3` 的号：新公式会比旧 `conc+max_sessions` **变小**。这是合同，不是回归。要更大缓冲就手写 `rpm_sticky_buffer`，或把并发改到 10–15。

## 3. 架构

```
下游请求
  → 解析锚点（§7，含正文 fallback）
  → 调度选号（粘性可逃逸）
  → 解析 canonical 账号（shadow → parent）取设备档案
  → GetOrCreate 基线档案（只读学习关闭时到此为止）
  → 若学习开启且过「官方门禁 + 版本注册表」且版本更高 → 先 Validate 再 CAS
  → 解析出站身份（档案设备 + 本 session 的 thread + 新 turn）
  → ValidateOutboundBundle（失败用上一份合法档案，不发半包）
  → EffectiveConcurrency 槽
  → 连接池按 §2.3 isolation 组 key（必含 tls_profile_id + transport_family）
  → 上游
```

权威：PostgreSQL `account_device_profiles`。Redis 只做 DB 投影：写 DB 必失效；上线丢弃或一次性导入旧 `fingerprint:` 键，禁止两套权威打架。

所有写入（创建、学习、基线抬升、admin Create/Update/Bulk/Import/`UpdateExtra`）必须经过同一组 `Validate*`。

## 4. 数据模型

`account_device_profiles`：

- `id`、`account_id` UNIQUE（**只给 canonical 账号**；shadow 不插行）
- `revision` bigint NOT NULL  — CAS 用，与 `schema_version`、`client_version` 分开
- `schema_version` int 1–100
- `platform` 必须 = `accounts.platform`（含 `gemini` / `antigravity` / `grok`）
- `client_family`：`claude-code` / `codex-cli` / `grok-cli` / `kiro-ide` / `gemini-cli` / `antigravity`
- `installation_id`、`device_id`、`client_id`、`machine_id`、`gateway_account_uuid`（Claude 出站 account 段，网关生成，**禁止**用 `extra.account_uuid`）
- `session_namespace` 32–64 hex，不可改
- `os_family` / `arch` / `runtime` / `runtime_version` / `client_version`
- `tls_profile_id` NULL 或正整数 FK，**禁止 -1**
- `transport_family` `h1`|`h2`
- `profile_payload` jsonb：只存 **版本化白名单键**，未知键拒绝或丢弃不落库
- `learned_from`：`baseline` | `official_traffic` | `baseline_floor`
- `learning_enabled` bool 默认 false（P1a 只建基线；P1b 再开学习）
- 时间戳：`created_at` / `updated_at` / `version_upgraded_at`

Shadow：`IsShadow()` 读 `parent_account_id` 的档案。并发槽仍按 **本行** `EffectiveConcurrency()`（不改变 spark 现状）。禁止为 shadow 插入第二份设备行。

迁移：`backend/migrations/234_account_device_profiles.sql` + Atlas；ent 用 `entsql.Annotation` 对齐 CHECK，禁止只靠 ent 自动迁。

## 5. 合法性校验

非法：**整包拒写**，留旧行，`identity_reject`。出站不发半包。Validate 放 `service/account_device_profile.go`。

### 5.1 通用

- `platform`：`anthropic` / `openai` / `gemini` / `antigravity` / `grok` / `kiro`，与账号行一致
- `client_family` 与 platform 匹配表命中
- `os_family` 学习时接受 Stainless 全集合（`macos/linux/windows/freebsd/openbsd/ios/android/other/unknown`）；未知值 **跳过学习、不拒出站**
- `arch`：`arm64/x64/x32/arm/unknown/other`，同上
- `runtime`：已知集合 + 官方请求带来的 `node/bun/deno/codex_cli_rs/grok-shell`；未知跳过学习
- `runtime_version` / `client_version`：长度 ≤64；禁 `-local/-dev/+build` 写入档案
- `tls_profile_id`：存在、≠-1、与 family+os+transport **兼容表**命中（扩 `TLSFingerprintProfile.Validate`）
- `profile_payload`：object ≤8KB，键 ∈ 当前 schema 白名单
- 字符串：trim，禁 CTL/NUL；UA≤256；单头≤512
- extra 容量：`ValidateAccountCapacityExtra` — concurrency 1–32 或空；max_sessions 0–10000；idle 1–1440；`rpm_sticky_buffer` 空或 1–10000；`codex_fingerprint_mode` 枚举；`enable_tls_fingerprint` bool；`tls_fingerprint_profile_id` ≠-1。`parseExtraInt` 不得默默截断 float 后入库

### 5.2 版本注册表（学习上界）

形态像官方 **不够**。学习/升级只接受：

1. `client_version` 落在服务端 **SoftwareBundleRegistry**（平台 + family → 完整软件包：UA 模板、originator、Stainless、Grok 四元、最低/最高版本），且  
2. `CompareSemver(candidate, profile) > 0`，且  
3. 整包 `ValidateSoftwareBundle` 通过。

注册表来源：编译期基线 ∪ 面板已发布版本。不在表内的「很高的合法形态版本」**不学**（防 999 与伪造高版本）。出站仍可用当前档案。

比较用标准 semver（含 prerelease），替换只比数字段的 `CompareVersions`。

### 5.3 Codex

- 版本：`NormalizeCodexClientVersion` 且 ≥0.144.0 **且 ≤ 注册表最高**
- UA/`originator`/`version` 三元相等；`PairCodexClientIdentity`；学习门收窄为官方 `codex_cli_rs` TUI/CLI，不接受宽泛 `Codex ` family
- `installation_id` / `openai_device_id`：`uuid.Parse` 且 version/variant 合法。`accounts.extra.openai_device_id` **写入时**同样校验（现在 `"dev-xyz"` 能出站）
- 出站 `originator` 小写；禁止 `X-Originator`
- header 尽量走 Claude 已有的 `headerWireCasing` / 顺序（P1）
- `turn_id`+`turn_started_at_unix_ms` 同一 bundle 只生成一次；换号 failover 算新 turn
- 默认 mode=`session`；`full` 不作新建默认

### 5.4 Claude

- 学习 UA 必须 **精确** `claude-cli`（`isAcceptableFingerprintUserAgent` 对非 cc 过宽，不能直接当学习门）
- 主版本 ≤ 注册表最高（可保留 +0 缓冲，去掉「+2 任意高」）
- Stainless 整包与 UA 同一次官方请求或同注册表行；禁止 merge 出新 UA+旧 Stainless
- 出站 `user_id`：`user_{device_id}_account_{gateway_account_uuid}_session_{derived}`
- **禁止** `RewriteUserID` 再使用 `extra.account_uuid`
- mimic 必须吃档案，禁止 `DefaultHeaders` 后盖
- 同一逻辑头不得两种大小写两个值
- `session_id_masking_enabled`：P0 停止生成 15 分钟随机 ID；键保留并标 deprecated；迁移清键，不静默改用户可观测开关含义后不告知

### 5.5 Grok

- 单次出站内部自洽：Token-Auth / identifier / version / **该请求的** UA
- **不要求** billing 探针与主路径全球四处相等（探针可用 `grok-pager` 形态）
- version：`IsSupportedCLIVersion` 且 ≤ 注册表最高
- 未知 `x-grok-*` 丢弃；不向 cli-chat-proxy 透传网关 affinity 头
- 订阅 token 不打 `api.x.ai`

### 5.6 Kiro

- `NormalizeMachineID`；双头 machineId 与 IDE 版本相同
- 建号已持久化 machine_id、refresh 已保留（现网已修）。本方案只把 machine_id **迁进档案**，断开与 refresh_token 的派生
- `SystemVersion` / `NodeVersion` / `x-amzn-kiro-commit` 进档案或由 machineId+注册表派生，禁止全局设置一改全号齐跳

### 5.7 TLS / 连接 / 调度

- **TLS 只由档案决定**：`os_family` / `client_family` / `tls_profile_id`。入站 UA **不**调用 `inferTLSFingerprintOS` / `inferTLSFingerprintClientType` 选模板
- `tls_fingerprint_router_id` / `bindings` / `default_os`：要么废弃覆盖 UA/originator，要么纳入同一 Validate；P0 至少禁止 bindings 按请求改 OS
- 连接池 **cacheKey 与 poolKey** 都必须含 `tls_profile_id` + `transport_family`（现状都不含）。P0 必改，不能留 P2。组键规则见 §2.3
- 禁 `req.Close=true`；禁 profile `-1`
- 声称 h2 必须 ALPN 与 http2 transport 同时真；否则写 h1
- `GenerateSessionHash`：**保留** metadata → cacheable → 正文摘要三级；只加长度上限。禁止「无头则空 hash」

### 5.8 学习 CAS

```
candidate = 注册表命中的整包（不是下游零散头 merge）
ValidateSoftwareBundle(candidate) 失败 → 不写
semver(candidate) ≤ profile → 不写
事务内：UPDATE … WHERE account_id=? AND revision=?
  写入新包 + revision+1
提交前再读最终行 ValidateOutbound
同进程 per-account mutex，减少空转 CAS
```

禁止「先 CAS 再 Validate、失败再回滚」跨事务。

## 6. 在线学习

P1a：只建校验过的基线档案，`learning_enabled=false`。  
P1b：按账号/平台打开学习。下游官方请求若版本在注册表内且更高，整包替换。第三方不写。

OS/Arch/installation/machine/device/`gateway_account_uuid`：先到先得，升级不改。  
Claude 真实 account UUID：永不学。  
TLS ClientHello：不从下游学；runtime 大变则换注册表里的兼容模板并 drain 该号旧 idle 连接。

基线抬升：注册表地板升高且整包合法时，抬软件包，不动设备字段。

## 7. 会话与粘性锚点

出站：

```
logical_anchor = 第一非空：
  session-id / session_id
  metadata.user_id 的 session 段
  conversation_id
  X-Claude-Code-Session-Id
  Responses/Chat 的既有正文锚点（appendResponsesSessionAnchorFromRaw 等）
  可缓存内容 / 会话上下文摘要（现网第三级，保留）

session_id = UUID(HMAC(session_namespace, "sess:"+anchor))
thread_id  = UUID(HMAC(session_namespace, "thread:"+session_id))  // 从属于该 session
window_id  = thread_id + ":0"
turn_id    = 每请求 UUIDv7（failover 换号则新 turn）
```

`client_instance_key` 不再使用。没有跨 session 的 thread LRU。活 thread 集合不必用 max_sessions 去砍；空闲会话自然过期即可。同时在飞仍靠并发槽。

## 8. 收敛范围

**钉死：** installation / device / gateway_account_uuid / machine / 软件包 / OS·Arch / TLS 模板 / 出口 / 连接池身份。  
**限额：** 同时 turn = EffectiveConcurrency。  
**按锚点派生：** 出站 session 与其唯一 thread。  
**每请求新值：** turn / request id / Amz-Sdk-Invocation-Id。  
**不收敛：** max_sessions、空闲超时、粘性逃逸、RPM、分组优先级、正文 fallback。

## 9. 分阶段（修订）

**P0 止血（不改表）——槽位与 Preserve 必须同批，禁止槽位先上、绑定后上**

1. mimic 使用 fingerprint/档案，不再 `DefaultHeaders` 覆盖  
2. 禁 TLS `-1`；TLS 选模板改为档案/账号固定 OS，不按入站 UA  
3. 连接池 cacheKey+poolKey 加 `tls_profile_id`/`transport_family`；H1 三个连接上限都 = Burst  
4. `originator` 小写，去掉 `X-Originator`  
5. `EffectiveConcurrency()` / `BurstConcurrency()` 全调用点收口；fail-closed；**不**改 Ent 默认 3  
6. RPM 公式 + DTO/前端停止回灌 + 显示公式对齐  
7. `openai_device_id` 写入校验  
8. 停止 15 分钟 session mask 的新随机（键先留着）  
9. §2.2 整条梯子：30s 只抢 N → 截止两枪 → Burst → 换号回 failover；`PreserveStickyBinding` 同批  
10. 候选过滤改为 `in_flight < Burst`；已绑定粘性号不得因 LoadRate=100 被剔除  
11. `LoadFactor` 只排序不放行

**P1a 档案（不开学习）**

表 + Validate* + 全入口 extra 校验 + GetOrCreate 基线 + Redis 改投影 + Claude `gateway_account_uuid` + 全出站入口读档案（HTTP/WS/passthrough/探针）+ shadow 解析。

**P1b 学习**

注册表 + CAS 整包升级 + `learning_enabled`。

**P1c 传输正确性（原 P2/P3 前置）**

Codex/Kiro 若要 h2：ALPN+http2 接线完成前不得声称 h2。H1 三字段 = Burst **已在 P0**，本阶段只做 h2 接线。

**P2** Kiro 全局版本迁档案；Codex header 大小写/顺序。`PreserveStickyBinding` **已升到 P0，不在本阶段。**

## 10. 测试（增补审查项）

- `concurrency≤0` 不得无限放行  
- `max_sessions=1000` 不抬 thread 跨 session 复用、不抬 RPM  
- 两个不同 session 不得共用 thread_id  
- 正文锚点：Responses 无 session 头仍能粘  
- 入站 Windows UA + 档案 macos → TLS 仍用 macos 模板  
- 学习：注册表外高版本不写；只改 UA 不改 Stainless → 拒  
- extra 经 Bulk/Import 写 `-1` / 并发 999 → 拒  
- RewriteUserID 不含真实 account_uuid  
- 前端保存账号不把计算 buffer 写回 extra  
- shadow 不插入第二行档案  
- 13/12 时新请求必须等满 30s，不能直接 Burst  
- 等待期内不得 `TryAcquireBurst`  
- 截止两枪：先 N 再 Burst；都失败才换号且原绑定不变  
- wait_queue_full 走换号，不 429  
- 换号后即时 `TryAcquire(N)`，不再等 30s  
- 临时槽释放后连接进 idle，不重握手  
- proxy 隔离下两号交替不因 Burst 不同拆整池  
- 粘性号 LoadRate=100 仍能进梯子  
- `streamStarted=true` 后截止失败不得换号  
- WS live lease 占 N/Burst  
- 换到 B 后 thread 来自 B 的 namespace，不是 A 的 thread  
- `concurrencyService == nil` fail-closed

## 11. 明确不做

不抄正文；不随机设备/TLS；不跨账号连接（account 隔离模式）；不用 `full` 撑高并发；不按请求重握手；不把学习默认对全库打开；管理端无静默重置。

## 12. 文件落点（补审查漏项）

新：`ent/schema/account_device_profile.go`、`migrations/234_account_device_profiles.sql`、`service/account_device_profile.go`、`service/account_device_service.go`、`service/software_bundle_registry.go`、`repository/account_device_profile_repo.go`  
改：`concurrency_service.go`、`concurrency_cache.go`、`account.go`、`openai_codex_fingerprint.go`、`identity_service.go`、`gateway_upstream_request.go`、`account_tls_fingerprint.go`、`http_upstream.go`（`buildCacheKey`/`resolvePoolSettings`/`getClientEntryWithTLS`）、`openai_account_scheduler.go`、`gateway_scheduling.go`、`openai_gateway_scheduling.go`、`openai_profit_control.go`、`gateway_service.go`、`handler/gateway_helper.go`、`handler/failover_loop.go`、`gateway_handler.go`、`gateway_handler_responses.go`、`gateway_handler_chat_completions.go`、`openai_gateway_handler.go`、`gemini_v1beta_handler.go`、`gateway_web_search.go`、admin account handler/dto/Bulk/Import、`dto/mappers.go`、`EditAccountModal.vue`、`AccountCapacityCell.vue`、`wire.go`  
`go generate ./ent` 与 `./cmd/server`；所有 `IdentityService` 构造变更的 test stub。

## 13. 建议项（本版不阻塞开工）

- 表/服务命名已改为 device，避免与登录 identity 混  
- Kiro refresh「未持久化」不当 P0（现网已修）  
- os/arch 已放宽学习枚举  
- Grok 探针与主路径解耦已写入  
- 代理 isolation 不在 P0 废弃  
- 终身字段可用 DB trigger 防误 UPDATE（P2）

## 14. 第二轮审查闭环（v2.1）

| 原缺口 | 本版 |
|---|---|
| `PreserveStickyBinding` 写在 §2.2 却标 P2 | 升 P0，与槽位同批 |
| 超时只 `handleConcurrencyError` | 未开流 → failover；已开流 → 禁止换号 |
| `LoadRate < 100` 剔满号 | 过滤改 `in_flight < Burst`；粘性号永不因 100 剔除 |
| `LoadFactor` 第四个数 | 只排序，不放行 |
| H1 只抬 `MaxConnsPerHost` | 三个 idle/max 都 = Burst |
| cacheKey 不含 TLS | cacheKey 与 poolKey 都带；proxy 隔离不按账号调 Burst |
| §9 P1c 仍写「预算=并发」 | 改为 Burst，且已在 P0 |
| 换到 B 的身份 | B 档案 + B 新 session/thread |
| WS / nil service | live 占槽；nil fail-closed |
