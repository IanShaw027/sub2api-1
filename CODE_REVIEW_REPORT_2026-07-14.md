# Sub2API 近 3 天改动代码审查报告

## 1. 审查概况

| 字段 | 值 |
|------|-----|
| 报告日期 | 2026-07-14 |
| 分支 | `personal-dev` |
| 审查基线 | `94a22b62f7b963b6b671e3fc292ad0af609e6143` |
| 审查终点 | `b098f7dc6fe3decfcfce639b7114082eee465e56` |
| 审查范围 | 过去 3 天提交形成的净差异，包含期间合入的 upstream 改动 |
| 变更规模 | 2420 个文件 |
| 审查方式 | 10 个子代理分区审查，主代理去重及源码交叉核验 |
| 代码修改 | 无，本报告仅记录审查结果 |

本报告独立于根目录现有的 `AUDIT_REPORT_2026-07-14.md`，不包含或覆盖该文档的 Grok 审查内容。

## 2. 结论摘要

本轮共确认 26 个问题：

| 严重度 | 数量 | 主要风险 |
|--------|------|----------|
| P0 / Critical | 4 | 服务进程崩溃、真实余额双扣、调度事件永久丢失、跨会话上下文泄漏 |
| P1 / High | 12 | 协议失败、调度快照陈旧、OAuth 重放/封禁绕过、错误计费、账户路由错误 |
| P2 / Medium | 10 | 故障恢复缺口、缓存资格绕过、静默截断、前端状态机与批量编辑竞态 |

不建议在 P0 问题修复前将当前 HEAD 作为生产发布基线。多副本部署还应优先处理 scheduler outbox 和缓存失效问题。

## 3. P0 / Critical

### P0-1. Images OAuth fan-out 错误路径可终止服务进程

- 位置：`backend/internal/service/openai_images_responses.go:1799`
- 触发：OAuth 图片请求 `n > 1`，任一子请求返回普通 4xx、内容拒绝或 cyber 错误。
- 原因：fan-out goroutine 使用 `c.Copy()`；Gin 复制上下文的底层 ResponseWriter 为 nil，但错误处理和响应收集路径仍会调用 `Header()`、`JSON()` 等写响应操作。
- 影响：goroutine 发生 nil pointer panic，且不经过 Gin recovery，可直接终止整个服务进程。
- 建议：fan-out 子任务不得持有可写 Gin context；将错误转换为纯数据结果，由主 goroutine 统一写响应，并增加 `n > 1` 子请求失败测试。

### P0-2. AI Skill settlement 重放可重复扣除买家余额

- 位置：`backend/internal/service/ai_skill_settlement_service.go:92`、`backend/internal/service/wire.go:336`
- 触发：扣款成功，但 settlement 最终状态更新在重试后仍失败；两分钟后 stale pending 被重新执行。
- 原因：结算服务明确要求 `ChargeUserBalance` 按 `ai_skill_run:<runID>` reference 幂等，但生产适配器忽略 `input.Reference`，每次都直接调用 `DeductBalance`。
- 影响：同一个 skill run 可多次扣除真实用户余额。
- 建议：引入带唯一 reference 的余额 ledger，并在同一事务中 claim reference 和扣款；补充“扣款成功、状态提交失败、重放后只扣一次”的测试。

### P0-3. Scheduler outbox watermark 会永久跳过迟提交事件

- 位置：`backend/internal/repository/scheduler_outbox_repo.go:30`、`backend/internal/service/scheduler_snapshot_service.go:389`
- 触发：事务 A 分配 ID `N` 但尚未提交；事务 B 分配并提交 `N+1`；poller 处理 `N+1` 后推进 watermark，随后 A 才提交。
- 原因：PostgreSQL sequence 顺序不等于事务提交顺序，但实现使用 `id > watermark` 作为唯一消费条件。
- 影响：事件 `N` 永久不可见，相关调度快照持续陈旧；cleanup 还可能最终删除未处理事件。
- 建议：改用显式 claimed/processed 状态，或只推进经过事务验证的连续已提交区间，不能直接把 sequence ID 当作提交游标。

### P0-4. Grok active-delta 可在独立对话之间串接上下文

- 位置：`backend/internal/service/openai_gateway_grok_cache.go:45`、`backend/internal/service/openai_gateway_grok_active_delta.go:45`
- 触发：同一 API key 下两个没有显式 session header 或 `prompt_cache_key` 的对话，具有相同的 model、tools、system 和首条 user 内容。
- 原因：自动 session identity 不包含可靠的客户端会话标识，active-delta 又直接以该 identity 保存 `lastResponseID` 和 materialized hashes。
- 影响：后续请求可能接到另一个对话的隐藏上游上下文；共享 API key 时构成跨用户内容泄漏。
- 建议：没有显式稳定会话标识时禁用 active-delta，或生成真正独立且由客户端持续回传的会话 token。

## 4. P1 / High

### P1-1. WebSocket-over-H2 缺少发送流量控制

- 位置：`backend/internal/service/openai_ws_client_h2.go:365`
- 触发：Codex 请求帧超过对端 connection 或 stream window，默认通常为 65,535 字节。
- 影响：合规 H2 上游可返回 `RST_STREAM`、`GOAWAY` 或 `FLOW_CONTROL_ERROR`，大上下文请求稳定失败。
- 建议：维护连接和 stream window，处理 `SETTINGS_INITIAL_WINDOW_SIZE` 与 `WINDOW_UPDATE`，仅在窗口允许时写 DATA。

### P1-2. Bucket 锁竞争被当成事件处理成功

- 位置：`backend/internal/service/scheduler_snapshot_service.go:698`
- 触发：一个副本正在重建 bucket，另一个副本处理该 bucket 的新 outbox 事件并获取锁失败。
- 影响：新事件未应用但 watermark 已推进；若锁持有者读取的是较旧 DB 状态，陈旧快照将长期保留。
- 建议：锁 miss 返回可重试错误并阻止 watermark 越过该事件，或实现能保证再次重建的合并机制。

### P1-3. 账号/代理状态更新与 scheduler outbox 非原子

- 位置：`backend/internal/repository/proxy_repo.go:527`、`backend/internal/repository/account_repo.go:1657`
- 触发：代理到期改投或账号自动暂停已提交，随后进程退出、context 取消或 outbox 写入失败。
- 影响：其他实例继续使用暂停账号、旧代理或旧 proxy metadata；无 fallback 的过期代理甚至完全不会产生事件。
- 建议：在数据更新的同一事务中写 outbox；无 fallback 时也必须失效引用该代理的账号或相关 bucket。

### P1-4. Prewrite ping 会误杀健康的 coder/websocket 连接

- 位置：`backend/internal/service/openai_ws_forwarder.go:4511`、`backend/internal/service/openai_ws_client.go:427`
- 触发：默认 coder/websocket transport 的 OAuth session-bound 连接空闲超过 prewrite threshold 后被复用。
- 影响：健康连接因没有 reader 消费 pong 而超时，被标记 broken，引发延迟、连接抖动和上游重连风暴。
- 建议：prewrite predicate 必须检查 `lease.SupportsIdlePingWithoutReader()`。

### P1-5. Memory-only OAuth session 不能保证单次消费

- 位置：`backend/internal/pkg/antigravity/oauth.go:383`、`backend/internal/pkg/openai/oauth.go:158`
- 触发：未启用 Redis 时，两个并发 callback 使用同一 session ID、state 和授权码。
- 影响：两个请求都可通过消费检查并尝试 token exchange，造成重复账户操作或抢先消耗授权码。
- 建议：在 store mutex 下原子查找并删除/标记 session；通用 OAuth、OpenAI、Antigravity 实现需同时修复。

### P1-6. Disabled/suspended 用户的 OAuth identity 可被新账户接管

- 位置：`backend/internal/handler/auth_oauth_pending_flow.go:1104`
- 触发：已有 OAuth identity 指向非 active 用户，攻击者再次使用该上游身份完成注册或绑定。
- 影响：管理侧停用账户后，用户可以通过新本地账户重新获得同一上游登录身份，绕过封禁策略。
- 建议：只要原 user 记录仍存在就保持 ownership conflict；回收身份必须经过显式管理员解绑或删除流程。

### P1-7. Grok 空图片响应仍按请求数量计费

- 位置：`backend/internal/service/grok_media.go:985`
- 触发：上游 2xx 返回 `{"data":[]}`，请求中的 `n > 0`。
- 影响：用户没有收到图片却被按 `n` 张扣费。
- 建议：区分“无法解析数量”和“明确返回空数组”；明确空数组必须计 0，并增加相应契约测试。

### P1-8. Grok OAuth 创建被固定路由到 Public API

- 位置：`frontend/src/components/account/CreateAccountModal.vue:5932`
- 触发：管理员使用普通授权码创建 Grok OAuth/Build 账户。
- 影响：前端强制写入 `https://api.x.ai/v1`，覆盖后端 OAuth 默认 CLI proxy 及系统 `grok_default_base_url_mode`，导致鉴权和额度行为错误。
- 建议：OAuth 类型不要写账户级 public API base URL；仅 API key 类型采用该默认值。

### P1-9. 关闭 Kiro IDC 弹窗后轮询仍会继续修改账户

- 位置：`frontend/src/composables/useKiroOAuth.ts:123`、`frontend/src/components/account/CreateAccountModal.vue:5235`
- 触发：进入 IDC continuation 后关闭弹窗，再在外部验证页完成授权。
- 影响：旧 Promise 继续轮询并静默创建或重授权账户，违反用户取消操作的预期。
- 建议：close 和 unmount 时调用 cancel/reset，并在持久化前验证当前请求 generation 未被取消。

### P1-10. 两个 migration 使用相同数字前缀 202

- 位置：`backend/migrations/202_add_usage_logs_api_key_latest_ip_index_notx.sql:1`、`backend/migrations/202_transactional_scheduler_outbox_triggers.sql:1`
- 触发：运行 backend unit suite 或依赖文件名排序执行 migration。
- 影响：`TestMigrationFilenameNumericPrefixesStayDeliberate` 确定性失败，迁移顺序不再明确。
- 建议：给其中一个 migration 分配新的唯一编号并同步相关契约测试/文档。

### P1-11. Codex 工具名归一化不可逆且存在碰撞

- 位置：`backend/internal/service/openai_codex_transform.go:22`
- 触发：工具名包含 `.`、`/` 等非法字符，或两个名称归一化后相同。
- 影响：`read.file` 返回为 `read_file` 后客户端无法分派；`foo.bar` 和 `foo/bar` 同时变成 `foo_bar`，产生工具歧义。
- 建议：保存请求级双向映射，并在响应时恢复原名；检测归一化碰撞并返回明确错误。

### P1-12. WS v2 弱终态替换会重复累计图片 token

- 位置：`backend/internal/service/openai_ws_v2/passthrough_relay.go:940`
- 触发：同一 response 先收到带图片 usage 的 incomplete/cancelled，随后收到 completed/done。
- 影响：弱终态撤销遗漏 `ImageOutputTokens`，强终态再累加后形成双计费。
- 建议：usage delta 必须覆盖所有 token 字段，并增加弱终态到强终态的图片 usage 测试。

## 5. P2 / Medium

### P2-1. Subscription 邀请返利存在不可自动恢复的崩溃窗口

- 位置：`backend/internal/service/payment_fulfillment.go:762`
- 触发：订单标记 completed 后、执行邀请返利前进程退出。
- 影响：重复 webhook 对 completed 订单直接返回；repair 命令不扫描 subscription，邀请人可能永久漏返利。
- 建议：将订单完成和返利 claim 纳入同一可恢复状态机，completed 重放也应检查遗漏的 post-commit 操作。

### P2-2. Balance outbox 未覆盖 minimum balance reserve 边界

- 位置：`backend/migrations/201_balance_cache_outbox.sql:22`
- 触发：扣费后余额从 reserve 上方降至 reserve 下方但仍大于 0，同时同步 Redis invalidation 失败。
- 影响：旧缓存继续判定余额可用，最长可在缓存 TTL 内绕过最低余额保护。
- 建议：durable invalidation 需要覆盖实际计费资格边界，或对所有余额减少统一写 outbox。

### P2-3. Redis session 写入失败后不回退本地副本

- 位置：`backend/internal/pkg/openai/oauth.go:98`，相同模式存在于通用 OAuth、xAI、Antigravity 和 Kiro。
- 触发：生成授权 URL 时 Redis 暂时不可用或超时。
- 影响：接口仍返回授权 URL，但 callback 只读 Redis 并报 session not found，即使回到原实例也失败。
- 建议：不要吞掉 remote `Set` 错误；若保留 hybrid store，remote 错误时应回退本地副本并记录可观测错误。

### P2-4. Grok multipart 图片超过 50 MiB 时被静默截断

- 位置：`backend/internal/service/grok_media.go:251`
- 触发：单个 multipart part 超过 50 MiB、总请求仍低于全局 256 MiB 限制。
- 影响：截断后的损坏数据被继续编码并发送上游，产生难以理解的上游失败，极端情况下还可能发生错误计费。
- 建议：读取 `limit + 1` 并显式检测溢出，返回本地 413/400；不要吞掉 `NextPart` 或 read error。

### P2-5. Grok quota 刷新失败会把旧数据伪装成新快照

- 位置：`backend/internal/service/grok_quota_service.go:230`
- 触发：已有快照后，credits 和 monthly 请求同时失败。
- 影响：旧窗口被复制、`UpdatedAt` 被刷新且无 FetchError，管理界面在 TTL 内不再重试并显示为刚更新。
- 建议：两主窗口都失败时返回错误并保留原时间；部分成功时只更新成功窗口。

### P2-6. Kiro/Grok 创建按钮在写库完成前重新启用

- 位置：`frontend/src/components/account/CreateAccountModal.vue:6024`、`frontend/src/composables/useKiroOAuth.ts:242`
- 触发：凭据验证完成但 `/admin/accounts` POST 尚未返回时再次点击授权按钮。
- 影响：可并发创建重复账户及重复发起密码登录/SSO 请求。
- 建议：按钮 loading 合并 modal submitting 状态，并对整个验证加创建流程使用单一 in-flight guard。

### P2-7. Filtered bulk edit 存在平台隔离 TOCTOU

- 位置：`frontend/src/views/admin/AccountsView.vue:2103`、`frontend/src/components/account/BulkEditAccountModal.vue:2073`
- 触发：预览后、提交前，另一管理员新增匹配 filter 的异平台账户或改变现有账户类型。
- 影响：OpenAI WS、passthrough、compact、TLS 等 extra-only 配置可能写入异平台账户，违反批量编辑平台隔离约束。
- 建议：提交固定 ID 快照，或由服务端原子校验 expected platform/account types。

### P2-8. Kiro 重授权后的冗余 clear-error 会把成功误报为失败

- 位置：`frontend/src/components/admin/account/ReAuthAccountModal.vue:520`
- 触发：`kiro-reauthorize` 已成功更新凭据，随后第二个 `clearError` 请求网络失败。
- 影响：UI 报授权失败且批量流程错误计数，用户重试时可能再次覆盖已成功账户。
- 建议：直接使用 reauthorize 返回的更新账户，不再发送重复 clear-error。

### P2-9. H2 WS close 可永久阻塞且 frame size 存在数据竞争

- 位置：`backend/internal/service/openai_ws_client_h2.go:311`、`backend/internal/service/openai_ws_client_h2.go:361`
- 触发：对端在连接期间更新 SETTINGS，或半开连接/发送缓冲区满时关闭连接。
- 影响：`peerMaxFrameSize` 无锁读写产生 race；无 deadline 的 close frame 写入可卡住连接池淘汰和优雅停机。
- 建议：为动态设置增加同步保护，并使用有上限的 close context，失败后直接关闭底层连接。

### P2-10. Release tag 被直接插值到 shell 脚本

- 位置：`.github/workflows/release.yml:44`、`.github/workflows/release.yml:155`、`.github/workflows/release.yml:228`
- 触发：release tag 包含 shell 元字符。
- 影响：GitHub expression 在 shell 解析前展开，可能在带发布和 registry 权限的 runner 上形成命令注入。
- 建议：通过 step `env` 传入 tag、始终使用引号展开，并用严格 release tag 正则校验。

## 6. 自动化验证

| 命令 | 结果 |
|------|------|
| `cd frontend && pnpm run typecheck` | 通过 |
| `cd frontend && pnpm run lint:check` | 通过 |
| `cd frontend && pnpm run test:run` | 通过，271 个测试文件、1806 个测试 |
| `cd backend && go test -tags=unit ./...` | 失败；唯一确认失败为 migration 数字前缀 `202` 冲突，其余已执行 package 通过 |

定向测试虽覆盖了大量 happy path，但当前缺少以下关键故障注入场景：进程在 post-commit 步骤之间退出、Redis 短暂不可用、多副本 outbox 交错提交、bucket 锁竞争、OAuth callback 并发重放、H2 flow control、大图片 multipart 边界，以及弹窗卸载后的异步任务取消。

## 7. 建议处理顺序

1. 先修复 Images fan-out panic、Skill 双扣、scheduler watermark 和 Grok 会话串线四个 P0。
2. 随后处理 scheduler 锁/事务 outbox、OAuth 单次消费、H2 flow control 和 WS prewrite ping。
3. 修正 migration 编号后重新运行完整 backend unit suite。
4. 为前端 OAuth 生命周期、重复提交和 filtered bulk edit 增加竞态测试。
5. 最后补齐 Redis、支付履约、quota 刷新和 multipart 限制的故障注入覆盖。
