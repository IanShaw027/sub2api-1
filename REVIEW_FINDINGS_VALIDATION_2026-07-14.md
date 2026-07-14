# 近 3 日代码审查问题逐项验证与最佳实践

- 验证日期：2026-07-14
- 验证对象：`AUDIT_REPORT_2026-07-14.md`（70 项）与 `CODE_REVIEW_REPORT_2026-07-14.md`（26 项）
- 代码基线：以 `b098f7dc6` 为审查起点，在当前工作树执行修复；原两份报告未修改
- 输出性质：事实核验、最佳实践与修复执行记录

## 1. 结论摘要

按两份报告的原始编号逐项计数（重复项仍分别保留，以便追溯）：

| 判定 | 旧报告 | 新报告 | 合计 |
|---|---:|---:|---:|
| 确认 | 40 | 24 | 64 |
| 部分确认 / 严重度或表述需修正 | 7 | 1 | 8 |
| 设计、产品或运维风险，不是已证明代码缺陷 | 16 | 1 | 17 |
| 当前不成立 / 已有防护 | 6 | 0 | 6 |
| 依赖外部协议，尚不能仅凭仓库确认 | 1 | 0 | 1 |
| 合计 | 70 | 26 | 96 |

最重要的校正结论：

1. 需要继续按阻断级处理：Images OAuth fan-out 的进程 panic、Skill settlement 双扣、scheduler sequence 游标漏事件、Grok active-delta 跨对话串接、JWT 进入第三方嵌入 URL。
2. 旧报告把若干真实行为定性过重：余额透支是测试锁定的产品策略；Google `?key=` 是兼容协议取舍；匿名支付订单 verify 是受限状态 oracle；iframe 是安全加固问题但不等同于同源 XSS。
3. 旧报告有 6 项当前不成立：Images JSON keepalive 不会破坏 JSON、Videos failover 未证明会在 partial write 后换号、TLS capture 默认不保存 body、`CurrentToolName` 不是死字段、Skill 后端确实做所有权校验，以及 active-delta “默认值违反旧设计文档”本身不是独立代码缺陷。
4. `backend/migrations` 的重复编号 `202` 已用测试实际复现；这是确定的 CI 基线失败。

### 1.1 修复执行状态

本节按原报告编号记录当前工作树的修复结果。重复问题保留多个编号，便于从两份报告回查；“已修复”表示已有回归测试且全量 unit 通过，不等同于所有设计风险均已关闭。

| 修复主题 | 对应编号 | 状态与验证 |
|---|---|---|
| Skill settlement 幂等扣款 | 旧 P0-1；新 P0-2 | 已修复：新增唯一 reference ledger，claim、余额、cache outbox 同事务；并发与重放测试通过。 |
| 嵌入页 JWT query 泄漏 | 旧 P0-2 | 已修复：第三方 URL 不再携带 JWT；前端回归测试通过。 |
| Images partial 计费与 fan-out 写响应 | 旧 P0-3；新 P0-1 | 已修复：worker 只返回纯数据，主 goroutine 写响应；token-only partial 继续结算；定向 race 通过。 |
| Scheduler sequence watermark 与 lock miss | 旧 P0-5；新 P0-3、P1-2 | 已修复：PostgreSQL claim/lease/ack，lock miss nack/retry，真实 backlog；service/repository unit 与定向 race 通过。 |
| Scheduler 状态与 outbox 原子性 | 旧 P1-S3；新 P1-3 | 已修复本报告指出的 auto-pause、proxy-expiry 路径：状态、账号改投与 outbox 同事务；enqueue 失败回滚。 |
| Grok active-delta 跨会话串接 | 新 P0-4 | 已修复：仅显式会话标识启用 active-delta；内容 hash 只用于无状态 prompt cache。 |
| H2/WS 状态机与 prewrite ping | 旧 P0-4、P1-O1；新 P1-1、P1-4、P2-9 | 已修复：双层流控、单 reader、真实 ping/pong、close deadline、共享状态加锁、transport capability 门闩；定向 race 通过。 |
| OAuth 单次消费、state、redirect 绑定 | 旧 P1-A1/A2/A3/A4/A5/A7；新 P1-5、P2-3 | 已修复：六类 store 原子消费；Gemini Redis store；Claude/Grok 强制 state；redirect 只读 session；Redis Set 失败仅对本实例 local-only session 回退。 |
| 敏感凭证 update merge 清单 | 旧 P1-A6 | 已修复：更新合并复用统一脱敏 secret 清单，并有集合一致性测试。 |
| Stripe secret 与 Airwallex 恢复 | 旧 P1-B5、P1-F1 | 已修复：Stripe client secret 只从本地恢复快照读取；URL query 被拒绝；Airwallex launch kind 可持久化恢复。 |
| Kiro/Grok 前端异步生命周期 | 新 P1-8/P1-9、P2-6/P2-8 | 已修复：OAuth base URL 不再误 pin；modal close/unmount 取消轮询；loading 覆盖验证加写库；重授权使用最新账号且移除第二次 clear-error。 |
| Migration 重号 | 新 P1-10 | 已修复：迁移重排到 210/211，新 ledger/scheduler 迁移使用 207-209；完整 migrations unit 通过。 |
| Codex 工具名与 WS 图片 usage | 新 P1-11/P1-12 | 已修复：归一化碰撞 fail closed，请求级反向映射恢复结构化 name；弱终态 replacement 回滚 ImageOutputTokens；定向 race 通过。 |
| 协议终态与 namespace tools | 旧 P1-O4/O5/O6/O7/O9、P2-7/P2-9 | 已修复：incomplete event、safety stop reason、namespace 展平碰撞检测、非流图片终态、native image instructions、流 finalize stop reason 均有 contract tests。 |
| Grok 配额陈旧时间戳 | 新 P2-5 | 已修复：所有权威窗口刷新失败时保留旧 UpdatedAt 并返回错误，不再伪装成新快照。 |
| Markdown iframe 与 CI tag 注入 | 旧 P1-F2；新 P2-10 | 已修复：Markdown 使用默认 sanitizer，不再放行 iframe；release tag/message 经环境变量进入 shell。 |
| 确认提交门闩与死类型/旧文案 | 旧 P2-1/P2-13/P2-16 | 已修复：Grok 并发文案跟随后端；订阅 handler 重检 canSubmit；删除未使用 PaymentChannel。 |
| Balance outbox 分组确认 | 旧 P1-B2 | 已修复：按用户分组 invalidation；成功用户单独 ack，失败用户只 nack 自己的事件，避免整批重放。 |
| 图片逐图计费与实际尺寸接线 | 旧 P1-B3、P2-10；新 P1-7 | 已修复：混合尺寸按 tier 分组计价；Images 非流、流、pending finalize 与 fan-out 均回填实际输出尺寸；显式零输出不再按请求 `n` 计费。 |
| Grok multipart 超限 | 新 P2-4 | 已修复：读取 `limit+1` 检测超限，handler 显式返回 413，不再向上游转发截断 part。 |
| Scheduler 局部缓存与 bulk 去重 | 旧 P1-S2/P1-S4 | 已修复：单个账号 meta 缺失只跳过该账号，全部缺失才回源；`account_bulk_changed` 使用 pending dedup key。 |
| Filtered bulk edit 放大与 TOCTOU | 旧 P1-F3；新 P2-7 | 已修复：大集合不再为 UI metadata 逐页拉全量；后端按最终匹配账号复验 Credentials/Extra 单平台约束。 |
| WS preemption 跨实例所有权 | 旧 P1-O2 | 已修复：Redis Lua 原子 get-and-claim，唯一 owner token，compare-and-refresh heartbeat 与 compare-and-delete release；并发及定向 race 通过。 |
| Grok free-tier 路由入口一致性 | 旧 P1-G3 | 已修复：原生 Responses、Claude bridge 与 Chat bridge 对 Grok OAuth 使用相同 native tools 路由；API-key 账号不注入。 |
| Concurrency release 瞬时故障 | 旧 P1-S6 | 已修复 release 缺口：幂等 release 使用受 context 约束的三次短退避重试，耗尽后保留 TTL/cleanup 兜底。heartbeat 原本就会在后续 ticker 周期重试，报告中“失败后仅 log”需据此收窄。 |
| Affiliate rebate 崩溃恢复 | 新 P2-1 | 已修复：completed balance/subscription 重放及并发完成分支都会重试幂等返利；repair 扫描缺 APPLIED/SKIPPED 终态的已完成订单。 |
| Minimum reserve cache invalidation | 新 P2-2 | 已修复并校正表述：reserve 配置本身不缓存；余额首次跨入 `0 < balance < reserve` 时，扣费与 balance outbox 同事务，失败回滚。 |
| Ops host 查询边界 | 旧 P2-14 | 已修复：List/Cleanup 共用 255-byte 上限，超限在 repository 前返回 `OPS_SYSTEM_LOG_HOST_TOO_LONG`。 |
| Grok Free 本地额度门闩 | 旧 P1-B4 | 已修复：仅明确 FREE 的 Grok OAuth 账号启用可配置滚动窗口软门闩，默认 2M、95%、24h；统计失败 fail-open。用量 API 返回同一策略和动态窗口数据，前端不再硬编码 2M，并保留旧 24h 字段兼容。 |
| Skill runtime 与 marketplace 结算契约 | 旧 P1-B7 | 已修复并明确产品契约：上游实际 token 用量保持收费；执行前先持久化不可重放的 settlement intent，执行成功后确认并结算。暂时失败返回推理结果与 `settlement_deferred`，持久化状态可独立幂等重放；执行失败的 intent 永久禁止误扣款。 |
| Grok OAuth 429 换号 | 旧 P1-G5 | 已修复：删除首次换号即停止的 Grok 特例，使用通用 `gateway.max_account_switches`（默认 10）；保留 response-committed 防重放和 10 秒风暴保护，且 Grok/OpenAI 风暴计数按平台隔离，避免交叉误熔断。 |
| Moderation 数据最小化 | 旧 P1-X4 | 已修复：默认不保存输入摘要，命中默认留存降为 30 天；关键词原文改为短 SHA-256 规则指纹，日志与数据库均不再写原词。管理员仍可显式开启经 secret 脱敏、240 字符截断的摘要留存。 |
| Anthropic `cache_control` 到 Responses | 旧 P2-8 | 已修复并按最新官方契约模型门控：GPT-5.6+ 的 system/user 文本、图片、文件映射为标准 `prompt_cache_breakpoint`；旧模型不发送该字段，tools、assistant、tool_result、TTL 等无等价语义继续显式丢弃并计入兼容指标。 |

用户要求澄清的旧 P1-B4/P1-B7、P1-G5、P1-X4 与 P2-8 已按确认后的契约全部关闭；原两份审查报告保持原样，修复状态只在本文追踪。

明确保留为契约或产品决策的项目不应自动改代码：余额 overdraft、disabled identity 回收、Google query key 兼容、匿名订单 verify，以及多副本部署策略。

### 1.2 修复后自动化验证

- `cd frontend && pnpm run test:run`：271 个测试文件、1812 项测试全部通过。
- `cd frontend && pnpm run typecheck && pnpm run lint:check`：通过。
- `cd frontend && pnpm run build`：production build 通过。
- `cd backend && go test -tags=unit ./...`：全部通过，包含 service、repository、handler 与 migrations。
- `cd backend && go build ./cmd/server` 及 `go build -tags embed ./cmd/server`：普通与嵌入前端构建均通过。
- H2/WS、Images fan-out、OAuth consume、WS image usage 的新增定向 `go test -race`：通过。
- `git diff --check` 与 release workflow YAML 解析：通过。
- PostgreSQL testcontainers 集成用例已补充；当前机器 Docker 不可用，容器测试由 harness 跳过，不能把该项写成已执行通过。

## 2. 判定方法

- **确认**：当前基线存在报告描述的控制流或数据流，且可从源码、测试或实际命令直接证明。
- **部分确认**：行为存在，但影响、触发条件、严重度或修复方向被报告夸大或混合了多个命题。
- **设计/产品/运维风险**：代码行为明确，但是否属于缺陷取决于产品契约、部署模型或治理要求。
- **当前不成立**：当前代码存在相反防护，或报告所述故障路径无法成立。
- **待外部协议验证**：仓库能证明缺少变换，但无法证明上游一定要求该变换。

验证方式包括：逐条读取调用链和测试、检查最近 3 日 diff、运行定向测试，以及对照 PostgreSQL、IETF、Gin、Stripe、GitHub、Redis、WHATWG、OWASP 和 AWS 的一手资料。未修改两份原报告。

### 2.1 教学式阅读路线

这 96 条问题可以归约为五个可复用的工程模型：

1. **崩溃窗口**：先产生不可逆副作用，再提交“已完成”状态。若进程在两步之间崩溃，重试会重复副作用。Skill 双扣就是 `扣余额 -> 崩溃 -> settlement 仍 pending -> 重试再扣`。检查方法是沿调用链标出每个持久化点，并逐点假设进程退出。
2. **Dual write**：同一业务动作分别写数据库和 Redis/outbox，且不在一个原子事务中。第一步成功、第二步失败时，下游永远看不到事件。正确做法不是“多重试几次”，而是业务行和 outbox 同事务，消费者再按至少一次投递做幂等。
3. **编号顺序不等于提交顺序**：PostgreSQL sequence 在事务开始阶段分配值。事务 A 取得 N 后停顿，事务 B 取得 N+1 并先提交完全合法。因此 `id > watermark` 只能表达编号大小，不能证明所有更小事务都已提交。
4. **一次性授权事务**：OAuth 的 session/state/PKCE/redirect URI 是同一笔授权事务的绑定材料。`consume` 必须是原子状态迁移，而不是“先 Get，稍后 Delete”；否则两个并发 callback 都可能通过检查。
5. **响应提交边界**：HTTP header/body 一旦写出，就不能再可靠更换 status 或换上游重试。网关必须在写出前决定 failover，写出后只能完成当前协议、记录 partial usage 或关闭连接。keepalive 的核心风险是 status 语义提前固定，而不是 JSON 前出现空白。

阅读矩阵时，可先找每行对应的模型，再看它是“代码违反模型”，还是“产品明确选择了另一种契约”。后者应写 ADR、指标和测试，而不是简单按 bug 修复。

## 3. 旧报告逐项验证（70/70）

### 3.1 P0（5 项）

| ID | 判定 | 核验结论与证据 | 最佳实践 |
|---|---|---|---|
| P0-1 | 确认 | `ai_skill_settlement_service.go:92` 允许回收 stale pending；`wire.go:336` 忽略 `Reference` 直接扣余额，扣款成功而状态提交失败后会再次扣款。与新报告 P0-2 重复。 | BP-01 |
| P0-2 | 确认 | `frontend/src/utils/embedded-url.ts:25` 把完整 JWT 写入第三方 URL 的 `token` query，且附加当前页面 URL；浏览器历史、Referer 和第三方日志均成为泄漏面。 | BP-04、BP-11 |
| P0-3 | 确认 | `backend/internal/handler/openai_images.go:362` 只有 `ImageCount > 0` 的 partial error 才继续记账；只有 token usage、图片数为 0 的失败结果被提前返回。是否为 P0 取决于实际漏损规模，建议按 P1 财务正确性修复。 | BP-09 |
| P0-4 | 确认 | session prewrite 路径未检查 `SupportsIdlePingWithoutReader`；coder WS 必须由 reader 消费 pong，健康空闲连接可被误判。与新报告 P1-4 重复，严重度更适合 P1。 | BP-05 |
| P0-5 | 确认 | watermark 使用普通 Redis `SET` 可被慢实例回写较小值；`rebuildBucket` 锁 miss 返回 nil 并被确认。新报告 P1-2覆盖锁问题；新报告 P0-3进一步识别了 sequence 提交乱序。 | BP-02、BP-08 |

### 3.2 P1：资金与计费（7 项）

| ID | 判定 | 核验结论与证据 | 最佳实践 |
|---|---|---|---|
| P1-B1 | 设计/产品风险 | `usage_billing_repo.go:182` 条件扣减失败后允许 overdraft，集成测试明确断言负余额。这不是意外回归；问题是预检 reserve 与最终结算策略是否应允许并发超卖。 | BP-16 |
| P1-B2 | 确认 | `billing_cache_service.go:271` 任一用户 invalidation 失败即对整批 `ids` Nack，已成功项会重复执行。主要影响是放大 Redis 压力和恢复延迟。 | BP-02、BP-15 |
| P1-B3 | 确认 | `ImageOutputSizes` 计费解析存在，但主图片路径很少填充；已明确接线的主要是 Grok media 和 WS bridge。output 4K、input 1K 时可能按低档计费。 | BP-09 |
| P1-B4 | 确认 | 2M/24h 只在 `AccountUsageCell.vue:614` 展示；后端计算 `GrokLocalUsage24h`，但调度器没有相应门闩，仍等上游 429。 | BP-09、BP-16 |
| P1-B5 | 确认 | `PaymentView.vue:830`、`:1046` 把 Stripe `client_secret` 放入 `/payment/stripe` query，`StripePaymentView.vue:140` 再从 query 读取；Stripe 明确要求不得存储、记录或暴露给客户以外的人。 | BP-04 |
| P1-B6 | 设计/演进风险 | `calculateOpenAIRecordUsageCost` 在 `WebSearchCalls > 0` 时只算 per-call 搜索费。当前 alpha/search 路径本来就不报告 token，尚无实际漏计；若未来端点同时返回 token，必须改为组合计费。 | BP-09、BP-15 |
| P1-B7 | 确认 | Skill runtime 先 `RecordUsage`，再执行 marketplace `Settle`；二者没有统一事务或补偿状态机，后者失败时买家已承担 token 费用。是否退款需产品契约，但双阶段失败窗口真实存在。 | BP-01、BP-02 |

### 3.3 P1：Grok / xAI（6 项）

| ID | 判定 | 核验结论与证据 | 最佳实践 |
|---|---|---|---|
| P1-G1 | 当前不成立 | 配置默认 true 与当前代码注释、测试一致；旧设计 Task 0 已过时，不能单独认定为缺陷。真实缺陷是无显式会话标识时仍从内容派生 identity，见新报告 P0-4。 | BP-15、BP-16 |
| P1-G2 | 部分确认 | `ValidateTrustedBaseURL` 失败后静默回官方地址，观测和 UX 不佳；但 OAuth 仅允许官方 hosts 是明确 SSRF/凭据外送安全策略，不能据此推定第三方 OAuth 应被支持。 | BP-11、BP-15 |
| P1-G3 | 确认 | Grok Responses/Claude 路径传 `applyGrokResponsesCacheIdentity(..., false)`，只有 Chat bridge 注入 free-tier tools，功能覆盖不一致。 | BP-07、BP-16 |
| P1-G4 | 设计/部署风险 | active-delta session context 默认是 `openai_ws_state_store.go:167` 的进程内 map；单实例正确，多副本命中率和一致性依赖共享 store/粘性路由。 | BP-08、BP-15 |
| P1-G5 | 确认 | `openai_account_runtime_block_fastpath.go:225` 在第一次 Grok OAuth 429 account switch 后停止继续 failover，健康池账号可能未被尝试。这是防风暴策略，但当前折中确实牺牲成功率。 | BP-07、BP-16 |
| P1-G6 | 部分确认 | DefaultClientID 是上游设备客户端常量，不能仅凭硬编码定为漏洞；password 管理流确实接收明文密码并依赖 YesCaptcha，且 YesCaptcha HTTP 调用缺少有效超时。应定为管理面凭据最小化和可用性问题。 | BP-04、BP-14 |

### 3.4 P1：OAuth / Auth / Session（8 项）

| ID | 判定 | 核验结论与证据 | 最佳实践 |
|---|---|---|---|
| P1-A1 | 确认 | Claude/OpenAI/Antigravity 的 memory `TryConsumeSession` 只 Get、不原子删除或 claim；并发 exchange 可同时通过。与新报告 P1-5 重复。 | BP-03 |
| P1-A2 | 确认 | Gemini session store 仅本地 map，无 Redis 共享，也无单次 consume；多副本 callback 丢失和同实例并发兑换均成立。 | BP-03、BP-08 |
| P1-A3 | 确认 | `grok_oauth_service.go:140` 允许 exchange 输入覆盖 session 中的 redirect URI，弱于创建时绑定同一 redirect。 | BP-03 |
| P1-A4 | 确认 | Grok bare code 可不带 state，且 session delete defer 在校验前设置，错误 state 能烧毁合法 session。高熵 session ID 降低攻击面，但不替代一次性 state。 | BP-03 |
| P1-A5 | 确认 | Claude exchange 没有客户端 state 输入比对，直接使用 session state；admin-only 与 PKCE 是缓解，不是 state 绑定本身。 | BP-03 |
| P1-A6 | 部分确认 | 两套敏感 key 列表确实不同；但“空 patch 必然清密钥”不准确：普通缺失字段会保留，风险发生在未被 merge 识别的 secret 被显式传空时。 | BP-04、BP-15 |
| P1-A7 | 确认 | Redis session `Set` 错误被 `_ =` 吞掉，读取路径又优先/只读 Redis 时会形成 split-brain。与新报告 P2-3 重复。 | BP-03、BP-15 |
| P1-A8 | 设计/兼容风险 | Google `/v1beta` 接受 `?key=` 是 Gemini 协议兼容设计，不是鉴权绕过；但 query secret 会进入代理/访问日志，应脱敏并优先 header。 | BP-04 |

### 3.5 P1：OpenAI / Codex / Media / 协议（10 项）

| ID | 判定 | 核验结论与证据 | 最佳实践 |
|---|---|---|---|
| P1-O1 | 确认 | `openai_ws_client_h2.go:342` 的 Ping 只写 WS ping frame即返回，不等待 reader 验证 pong，健康检查可假阳性。 | BP-05 |
| P1-O2 | 确认 | WS preemption claim 是 GET+SET，释放才有 compare-and-delete Lua；跨实例存在双 owner 窗口。 | BP-08 |
| P1-O3 | 当前不成立 | 当前有专用 `StopOpenAIImagesJSONKeepaliveCommitted`；心跳只写 JSON 合法 whitespace，late error 也有测试覆盖。200 status 已提交是协议折中，但“body 混杂/损坏”不成立。 | BP-07 |
| P1-O4 | 确认 | `chatcompletions_responses_bridge.go:1238` 对 `status=incomplete` 仍发 `response.completed` 事件，事件名和终态语义冲突。 | BP-07 |
| P1-O5 | 确认 | `anthropicStopReasonToCC` 不识别 `refusal`/`content_filter`，默认映射为 `stop`，安全终态信息丢失。 | BP-07 |
| P1-O6 | 确认 | Responses→Anthropic 只处理 function/custom/tool_search；namespace tool 未展开 children，和 Chat 轴不对称。 | BP-07 |
| P1-O7 | 确认 | image generation terminal normalization 主要在 SSE/compact 路径；非流 JSON passthrough 可保留 `generating` 与已有 result 的矛盾终态。 | BP-07 |
| P1-O8 | 待外部协议验证 | 仓库可确认 edit/extension 没有与 create 对称的 Grok 请求规范化；但需用 xAI 官方契约或真实响应证明参数一定不兼容，当前不能断言必然 400。 | BP-07 |
| P1-O9 | 确认 | `openai_codex_transform.go:961` 把 namespace image tool 也当成 native image_generation 已附加，仍会注入误导 instructions。 | BP-07 |
| P1-O10 | 当前不成立 | 当前 `UpstreamFailoverError` 路径均在响应写入前；partial-write 错误走普通 error，不再换号。仅“没有 writer size 检查”不足以证明双写。 | BP-07 |

### 3.6 P1：Scheduler / Infra（6 项）

| ID | 判定 | 核验结论与证据 | 最佳实践 |
|---|---|---|---|
| P1-S1 | 设计/部署风险 | full-rebuild coalesce 是进程内状态，多副本仍可同时 rebuild；正确性有其他兜底，主要是负载放大。 | BP-08、BP-15 |
| P1-S2 | 确认 | `scheduler_cache.go:140` 任一账号 meta key 缺失就使整个 bucket miss，导致 DB fallback 放大。 | BP-14、BP-15 |
| P1-S3 | 确认 | proxy/account 状态更新成功后才 best-effort 写 outbox，二者非原子；enqueue 失败只 log。与新报告 P1-3 重复。 | BP-02 |
| P1-S4 | 设计/吞吐风险 | `account_bulk_changed` 不在 dedup 支持列表，事件风暴可放大 rebuild；未证明会直接造成数据错误。 | BP-02、BP-15 |
| P1-S5 | 设计/架构加固 | 主动 H2 keepalive 仅显式 OpenAI H2 transport；其他默认 H2 依赖 TCP/请求活动。是否必须启用取决于 NAT/LB idle timeout。 | BP-05、BP-15 |
| P1-S6 | 确认 | slot release/heartbeat renew 失败只 log；release 失败会占槽直到 TTL，持续 Redis 故障可造成假满。 | BP-08、BP-15 |

### 3.7 P1：Frontend / Security / Ops（10 项）

| ID | 判定 | 核验结论与证据 | 最佳实践 |
|---|---|---|---|
| P1-F1 | 确认 | `paymentFlow.ts:330` 的 launch-kind 集合漏 `airwallex_route`，刷新/回跳恢复分支不完整。 | BP-10 |
| P1-F2 | 部分确认 | `CustomPageView.vue:279` 显式允许 iframe，确实缺 host policy/sandbox；但 iframe 是独立 browsing context，不应直接定性为同源 XSS。 | BP-11 |
| P1-F3 | 确认 | filtered bulk preview 按每页 100 拉完所有账户，只为推断平台；大集合会造成前端/API 放大。 | BP-10、BP-14 |
| P1-X1 | 设计/安全风险 | 匿名 verify 端点存在且 expired 可触发上游对账，但返回字段最小；应定性为状态 oracle、枚举/对账 DoS 风险，而非直接账户接管。 | BP-04、BP-14 |
| P1-X2 | 设计/治理风险 | checksum allowlist 是精确 filename/checksum pair，sync 工具仍拒绝任意 mismatch，不是通用后门；风险在于未来例外膨胀。 | BP-12 |
| P1-X3 | 部分确认 | migration runner 仍使用 `strings.Split(";")` 和简单整行 `--` strip，解析脆弱；当前更常见结果是 fail-closed/误判，未证明可绕过。 | BP-12 |
| P1-X4 | 确认 | moderation 把 keyword 写 slog，把仅做 secret-redaction 的 excerpt 写 DB；普通 PII 并未移除，存在 retention/访问面风险。 | BP-11、BP-15 |
| P1-X5 | 设计/运维风险 | Apple container 示例默认 `BIND_HOST=0.0.0.0`，属于默认暴露面；不是应用逻辑 bug，部署文档应明确 firewall/反代边界。 | BP-14、BP-15 |
| P1-X6 | 设计/治理风险 | `tools/secret_scan.py` 只覆盖少量高置信模式，工具描述也如此；应补充 entropy/provider 扫描，但不能据此说当前已有 secret 泄漏。 | BP-04、BP-15 |
| P1-X7 | 当前不成立 | `tls_fingerprint_capture_service.go:653` 默认不存 body，只有显式 `store_body=true` 才落盘；“body 仍默认落盘”与当前代码相反。监听地址误配仍是独立运维风险。 | BP-11、BP-14 |

### 3.8 P2（18 项）

| ID | 判定 | 核验结论与证据 | 最佳实践 |
|---|---|---|---|
| P2-1 | 确认 | Grok 并发限制已放开，但中英文 i18n 仍显示默认限制 1，属于确定的文案漂移。 | BP-15 |
| P2-2 | 确认 | `AccountUsageCell.vue:783` 综合 plan/tier/entitlement/billing object 推断 free，和其他 billing bar 判断并非单一规范；边界快照可显示错误条形。 | BP-15、BP-16 |
| P2-3 | 设计/命名风险 | `baseURLAllowedHosts` 实际是 OAuth trusted official hosts；命名未表达信任边界，但行为本身是收紧而非“third-party 放宽”。 | BP-11、BP-15 |
| P2-4 | 设计/演进风险 | plan-gated 识别依赖归一化后的固定英文短语，当前测试锁定且可用；上游换文案会失效，应增加 code 优先和 unmatched metric。 | BP-15 |
| P2-5 | 部分确认 | `kiro.DefaultModels` 当前确实未列 sonnet-5，但能力表已支持 sonnet-5；1M 使用 900k 是明确安全预算，不应表述为错误的 1M。 | BP-15、BP-16 |
| P2-6 | 当前不成立 | `CurrentToolName`、`CurrentToolArgs`、`CurrentToolHadDelta` 在 Responses→Anthropic 流转换中被读写且有测试，不是死字段。 | BP-15 |
| P2-7 | 确认 | `pause_turn` 的非流转换输出 `status=completed`，客户端无法从 Responses status 区分服务端工具暂停。 | BP-07 |
| P2-8 | 确认 | 注释声称保留 `cache_control`，但 system block 被压成纯文本、message/tool block 也没有携带该字段，转换中实际丢失。 | BP-07 |
| P2-9 | 确认 | `FinalizeAnthropicResponsesStream` 忽略 state 已解析 stop reason，流异常结束时强制 synthetic completed。 | BP-07 |
| P2-10 | 确认 | `ResolveImageBillingSize` 选所有 output 中最高 tier，`CalculateImageCost` 再乘总张数；混合 4K+1K 会按 4K×2。若上游逐图定价，这会 overbill。 | BP-09、BP-16 |
| P2-11 | 部分确认 | keepalive 后 late error 只能保持 HTTP 200，status 语义确需文档/指标；但 whitespace+JSON 合法，报告隐含的 body 损坏不成立。 | BP-07、BP-15 |
| P2-12 | 设计/维护风险 | pool-mode 本地 retry 与通用 FailoverState 两套机制并存，增加预算/观测不一致概率；当前没有直接故障复现。 | BP-07、BP-15 |
| P2-13 | 确认 | `frontend/src/types/payment.ts:155` 的 `PaymentChannel` 仅定义无使用，是可删除的死类型。 | BP-15 |
| P2-14 | 确认 | Ops host list/filter 没有显式长度上限；admin-only 显著降低风险，仍应与其他查询参数统一 bound。 | BP-14 |
| P2-15 | 当前不成立 | 前端 `editable/owned` 确实只是 UX，但后端 `AISkillService` 使用 `GetSkillByCreatorAndID` / `GetVersionByCreatorAndID` 强制所有权，因此不存在仅靠前端授权。 | BP-10 |
| P2-16 | 确认 | `confirmSubscribe` 只检查 plan/submitting，没有像 recharge 一样重新检查 `canSubmitSubscription`；按钮 disabled 是 UI 缓解。 | BP-10 |
| P2-17 | 设计/性能风险 | admin path 或 UI marker 在认证前创建 collector，最终 header 仍要求 admin；没有信息泄漏，只是未认证请求的有界开销。 | BP-14、BP-15 |
| P2-18 | 设计/极低概率风险 | subscription generation fence TTL 为 24h；极慢 DB load/异常暂停越过 TTL 时旧 generation 可重用。正常请求上下文远短于 TTL，属于理论边界。 | BP-08、BP-14 |

## 4. 新报告逐项验证（26/26）

### 4.1 P0（4 项）

| ID | 判定 | 核验结论与证据 | 最佳实践 |
|---|---|---|---|
| P0-1 | 确认 | `openai_images_responses.go:1799` 在 fan-out goroutine 中使用 `c.Copy()`；Gin 官方实现把 copy 的底层 `ResponseWriter` 置 nil，子路径仍调用写响应方法时会在 Gin recovery 作用域外 panic。 | BP-05、BP-06 |
| P0-2 | 确认 | 与旧 P0-1 相同：settlement reclaim + 生产 charger 忽略唯一 Reference，可重复扣款。 | BP-01 |
| P0-3 | 确认 | outbox `BIGSERIAL` ID 在事务提交前分配，poller 用 `id > watermark`；事务 N 晚于 N+1 提交时 N 会永久被越过。PostgreSQL 明确说明 sequence 不是事务提交序。 | BP-02 |
| P0-4 | 确认 | `resolveGrokCacheIdentity` 无显式 session 时回退 `deriveOpenAIContentSessionSeed`；相同首轮内容生成同 identity，active-delta 持久化 `lastResponseID`，可跨独立对话串接。 | BP-03、BP-07 |

### 4.2 P1（12 项）

| ID | 判定 | 核验结论与证据 | 最佳实践 |
|---|---|---|---|
| P1-1 | 确认 | H2 WS 发送 DATA 未维护 connection/stream send window，也未等待 WINDOW_UPDATE；大帧可违反 RFC 9113 flow-control credit。 | BP-05 |
| P1-2 | 确认 | `scheduler_snapshot_service.go:698` 拿不到 bucket lock 返回 nil，worker 将事件当成功并推进消费状态。 | BP-02、BP-08 |
| P1-3 | 确认 | `proxy_repo.go:489`、`account_repo.go:1657` 均在状态事务之后 best-effort enqueue，构成 dual write。与旧 P1-S3 重复。 | BP-02 |
| P1-4 | 确认 | 与旧 P0-4 相同：prewrite ping 忽略 transport capability，误杀 coder/websocket 健康连接。 | BP-05 |
| P1-5 | 确认 | 与旧 P1-A1 相同：memory-only session consume 不是原子单次消费。 | BP-03 |
| P1-6 | 设计/安全策略风险 | 代码和测试明确允许 disabled/suspended owner 的 OAuth identity 被新用户接管。这不是实现偏离，但会把“禁用主体”和“身份永久保留”解耦；必须由产品/合规明确回收期限。 | BP-03、BP-16 |
| P1-7 | 确认 | Grok 图片响应明确 `data: []` 时，usage fallback 仍使用请求 `n`，会对零输出计费。 | BP-09 |
| P1-8 | 确认 | 普通 Grok OAuth 创建最终补入 `https://api.x.ai/v1`，覆盖后端 OAuth 默认 CLI/system base URL 选择。 | BP-10、BP-15 |
| P1-9 | 确认 | Kiro IDC polling 只有 cancel token；创建/重授权 modal 关闭只卸载组件，没有 onUnmounted cancel，旧 Promise 仍能落库。 | BP-10 |
| P1-10 | 确认 | 两个 `202_*` migration 同时存在；实际运行 `go test -tags=unit ./migrations -run 'Test.*Migration.*Prefix' -count=1` 确定失败。 | BP-12 |
| P1-11 | 确认 | Codex 工具名规范化是多对一，碰撞后 map 无法可靠恢复原名；跨轮工具调用可能指向错误工具。 | BP-07 |
| P1-12 | 确认 | WS v2 弱终态替换只回滚常规 usage delta，未回滚 `ImageOutputTokens`，替换事件会重复累计图片 token。 | BP-09 |

### 4.3 P2（10 项）

| ID | 判定 | 核验结论与证据 | 最佳实践 |
|---|---|---|---|
| P2-1 | 确认 | 订单 fulfillment 提交后才执行 affiliate rebate；两者之间崩溃会留下已履约、返利未执行状态，当前依赖后续人工/外部触发而非必然自动恢复。 | BP-01、BP-02 |
| P2-2 | 确认 | balance outbox 覆盖余额变化，但 minimum reserve 设置变化没有等价 invalidation，缓存预检可短时沿用旧边界。 | BP-02、BP-15 |
| P2-3 | 确认 | Redis session Set 错误被吞；当 Redis 配置存在时读取不回退本地副本，授权开始成功但 callback 失败。与旧 P1-A7 重复。 | BP-03、BP-15 |
| P2-4 | 确认 | Grok multipart 使用约 50 MiB 有界读取，但超限没有返回 413/显式错误，而是转发截断内容，造成难诊断上游失败。 | BP-14 |
| P2-5 | 确认 | quota fetch 全失败时保留旧数据却更新成功/刷新时间，使陈旧快照看起来刚刷新。 | BP-15 |
| P2-6 | 确认 | Kiro/Grok 验证 composable 在验证完成即清 loading，外层账户写库仍在进行；窗口内可再次点击并创建重复账户。 | BP-10 |
| P2-7 | 确认 | filtered preview 后只保存 filter，提交时后端重新解析目标；期间集合跨平台变化形成 TOCTOU，部分 extra-only 字段缺平台复验。 | BP-10 |
| P2-8 | 确认 | `kiro-reauthorize` 已清错后，前端又单独调用 clear-error；第二次请求失败会把已成功重授权误报为失败。 | BP-10、BP-15 |
| P2-9 | 确认 | H2 close 没有 deadline，peer 不响应可永久阻塞；`peerMaxFrameSize` 跨 reader/writer 并发访问缺同步，应以 `go test -race` 增加覆盖。 | BP-05 |
| P2-10 | 部分确认 | release tag 直接插入 shell 的行为存在，GitHub 官方也建议不可信表达式经 env 传入；但能创建 release tag 的 maintainer 通常已拥有等价代码执行权限，当前威胁模型下 P2 偏高。 | BP-13、BP-16 |

## 5. 校正后的处理优先级

### 立即阻断发布

1. Images OAuth fan-out 禁止子 goroutine 写 Gin context，并在 goroutine 边界 recover/转纯数据结果。
2. Skill balance charge 以 `ai_skill_run:<runID>` 建唯一 ledger，并把 claim 与扣款放进同一数据库事务。
3. Scheduler outbox 停止用 sequence ID 作为提交游标；改 claimed/processed 状态或 `FOR UPDATE SKIP LOCKED` worker。
4. 无显式稳定会话标识时禁用 Grok active-delta；不要用内容 hash 代替 conversation ID。
5. 禁止把 JWT、Stripe client secret 和其他 bearer secret 放入 URL；嵌入页改短期、单用途 ticket。
6. 修复重复 migration `202`，恢复 backend unit baseline。

### 下一批高优先级

- H2 flow control、close deadline、真实 ping/pong 与 race；scheduler lock miss 不得 ack；状态写与 outbox 同事务。
- OAuth memory consume 原子化，Gemini 补共享 session store，redirect/state/PKCE 统一模板。
- 修复 Kiro polling 生命周期、创建 loading 窗口、filtered bulk edit 服务端平台复验。
- 按实际输出计费：零图片不计图片数、mixed sizes 逐图计价、WS replacement 回滚全部 usage delta。
- 协议桥接统一终态、stop reason、namespace tool 和 cache_control contract tests。

### 需产品或部署决策

- 是否允许余额 overdraft、Grok free 2M 是否作为调度硬门闩、disabled identity 何时可回收。
- OAuth 是否只信任官方 xAI hosts、匿名订单 verify 是否保留、各上游是否需要主动 H2 keepalive。
- PII/moderation excerpt retention、Apple container 默认监听地址、Server-Timing 未认证开销预算。

## 6. 最佳实践索引

| ID | 实践 | 对本仓库的落地方式 |
|---|---|---|
| BP-01 | 财务副作用必须幂等 | 使用唯一业务 reference ledger；同一事务执行 claim、扣款和状态迁移；重试返回首个结果。 |
| BP-02 | 避免 dual write，消费者至少一次且幂等 | domain row 与 outbox 同事务；消费者使用 claimed/processed 状态、逐事件 ack/nack；不得把数据库 sequence 当提交顺序。 |
| BP-03 | OAuth transaction 必须一次性、绑定完整上下文 | 高熵 session、短 TTL、原子 consume；state/PKCE S256；redirect URI 从 session 固定读取；多副本使用共享 store。 |
| BP-04 | bearer secret 不进入 URL | JWT、API key、PaymentIntent client secret 不放 query；用 Authorization header、内存/sessionStorage 或短期单用途 ticket；日志统一脱敏。 |
| BP-05 | H2/WS 实现完整协议状态机 | 同时维护 connection/stream flow-control window；reader 处理 control frame；ping 等待 pong；close 有 deadline；共享字段同步并跑 race tests。 |
| BP-06 | request context 的异步使用只读化 | Gin `Context.Copy` 只用于读取请求数据；worker 返回纯数据，主 goroutine 独占响应写入；异步边界有 panic containment。 |
| BP-07 | 网关转换以 contract test 保证信息守恒 | 明确 committed boundary；禁止 partial write 后 failover；对 terminal status、stop reason、tool name/ID、cache 字段做双向 golden tests。缓存能力按真实上游模型门控：有标准等价字段才映射，否则显式降级并计量。 |
| BP-08 | 分布式锁必须有 ownership 和 fencing | 随机 owner token、compare-and-delete/extend、TTL 与工作时长匹配；lock miss 是 retry，不是成功；generation/fencing token 单调。 |
| BP-09 | 计费依据可验证的实际产出 | partial usage 与产出数分开；按每张实际尺寸计价；retry/replacement delta 可逆；所有副作用带幂等键。 |
| BP-10 | UI 异步任务绑定组件和提交生命周期 | `AbortController`/generation token；unmount/close 取消；loading 覆盖验证和写库全过程；后端在提交时重新验证平台与权限。 |
| BP-11 | 内容与外部 URL 按最小权限处理 | iframe host allowlist + sandbox；默认不落 body；PII 与 secret 分级脱敏和 retention；trusted-host 失败显式报错。 |
| BP-12 | migration 不可变且编号唯一 | CI 强制唯一前缀/checksum；已发布 migration 不修改；使用明确 SQL parser/metadata，例外必须精确且有到期治理。 |
| BP-13 | CI 不可信值不直接生成 shell | GitHub expression 先传到环境变量或 action 参数；必要时严格验证 tag grammar，并最小化 token permissions。 |
| BP-14 | 所有资源输入有硬上限和显式失败 | body/host/filter/page 限制在入口验证；超限返回 4xx，不静默截断；网络和 close 操作均有 timeout。 |
| BP-15 | 失败必须可观测，配置和 UI 只有一个事实源 | 不吞 store/outbox/heartbeat 错误；记录 stale age、lock miss、unmatched protocol；共享常量/枚举由后端契约生成或统一定义。 |
| BP-16 | 产品策略先形成可测试契约 | overdraft、identity recycle、free quota、failover budget、headroom 等必须有 ADR、指标和边界测试，避免被误判为实现 bug。 |

## 7. 一手资料

- [PostgreSQL Sequence Functions](https://www.postgresql.org/docs/current/functions-sequence.html)：`nextval` 不随事务回滚回收，sequence 不提供 gapless/提交顺序保证。
- [RFC 9113: HTTP/2](https://www.rfc-editor.org/rfc/rfc9113.html)：DATA flow control、connection/stream window、WINDOW_UPDATE 与协议错误要求。
- [RFC 9700: OAuth 2.0 Security Best Current Practice](https://www.rfc-editor.org/rfc/rfc9700.html)：PKCE、一次性 state、redirect URI 与 authorization code injection 防护。
- [RFC 7636: PKCE](https://www.rfc-editor.org/rfc/rfc7636.html)：authorization code interception 与 S256 code verifier/challenge。
- [Gin Context.Copy source](https://github.com/gin-gonic/gin/blob/master/context.go)：copy 的 `writermem.ResponseWriter` 被显式置 nil，适合 goroutine 读取而非写响应。
- [Stripe PaymentIntent client_secret](https://docs.stripe.com/api/payment_intents/object#payment_intent_object-client_secret)：不得存储、记录或暴露给客户以外的人。
- [Stripe Idempotent Requests](https://docs.stripe.com/api/idempotent_requests)：用唯一 idempotency key 安全重试，避免重复副作用。
- [OpenAI Prompt Caching](https://developers.openai.com/api/docs/guides/prompt-caching/)：GPT-5.6 及后续模型支持 Responses 内容块级显式 cache breakpoint；旧模型和不支持的 block 会拒绝该字段。
- [AWS Transactional Outbox Pattern](https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/transactional-outbox.html)：同事务写业务数据与 outbox，消费者需幂等并处理重复消息。
- [Redis Distributed Locks](https://redis.io/docs/latest/develop/clients/patterns/distributed-locks/)：唯一 owner value、TTL、compare-and-delete/extend 与锁有效期。
- [GitHub Actions Secure Use](https://docs.github.com/en/actions/security-for-github-actions/security-guides/security-hardening-for-github-actions)：不可信表达式经中间环境变量或 action 参数传递，避免 inline script injection。
- [OWASP Information Exposure Through Query Strings](https://owasp.org/www-community/vulnerabilities/Information_exposure_through_query_strings_in_url)：HTTPS 也不能消除 Referer、日志、历史和缓存中的 query secret 泄漏。
- [WHATWG iframe sandbox](https://html.spec.whatwg.org/multipage/iframe-embed-object.html#attr-iframe-sandbox)：sandbox 默认限制 origin、脚本、表单、导航与弹窗，按 token 最小化放开。
- [Go Data Race Detector](https://go.dev/doc/articles/race_detector)：使用 `go test -race` 验证运行时并发访问。

## 8. 验证记录与局限

- 初次定向命令 `go test -tags=unit ./migrations -run 'Test.*Migration.*Prefix' -count=1` 曾复现两个未登记的 `202_*` migration；修复重排后完整 migrations unit 已通过。
- 源码核验覆盖了两份报告全部 96 个原始编号；重复项没有合并计数。
- Grok video edit/extension 的上游请求契约缺少当前可用的官方字段级说明，因此旧 P1-O8 保留“待外部协议验证”，不把推测写成事实。
- 本轮修复没有改写两份原始审查报告；修复状态只记录在本文，避免与 Grok 审查文档混写。
- Docker 不可用，因此新增 PostgreSQL 并发/事务集成用例尚未在本机实际启动容器；unit、sqlmock 与 migration 契约不能完全替代真实 PostgreSQL 验证。
