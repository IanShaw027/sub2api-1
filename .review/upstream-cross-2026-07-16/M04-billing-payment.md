# Module 4: 计费支付用量订阅

## 范围摘要
- **对比基线**: BASE=`da85cc7e` → HEAD=`eb64a5c1`；UPSTREAM=`b960ec19`（`upstream/main`）
- **本模块变更规模**: 体量大，覆盖 `backend/internal/payment/**`、`service/payment_*`、`billing_*`、`usage_billing`、`subscription_*`、`invoice_*`、`user_platform_quota_*`、`batch_image_*`、相关 handler/repository/migrations 接缝
- **相对上游的主要能力差异**:
  - 统一后扣费：`UsageBillingRepository.Apply` + `usage_billing_dedup` 幂等表，替代多步非原子扣费
  - 支付履约硬化：lease token、余额履约单事务、订阅「assign + SUCCESS 哨兵」同事务、取消/过期后 webhook 恢复
  - 退款：先扣余额/订阅天数作冻结持有 → 网关退款 → pending 终结/回补；已开票 ISSUED 需 force
  - 发票：`InvoiceService.Create` 事务 + `FOR UPDATE` + 活跃链接反查
  - 用户×平台配额：Redis 权威 + dirty flusher 绝对快照刷库（默认 `flusher_enabled=false`）
  - partial billing：流式终态错误补记；OpenAI 路径有 cyber 去重，Anthropic 主路径缺失
  - simple/standard：`RunModeSimple` 跳过预检与扣费，仅写 usage log

## 关键路径图

```
[支付]
Webhook → VerifyNotification → HandlePaymentNotification
  → confirmPayment(金额/provider 校验)
  → toPaid(CAS → PAID) / alreadyProcessed(恢复 cancelled/expired/failed)
  → executeFulfillment
      ├ balance: lease(RECHARGING) → doBalanceInTx(redeem+AddBalance+COMPLETED) → cache invalidate + affiliate
      └ subscription: lease → claim audit → assignSubscriptionExactlyOnce(tx: assign+SUCCESS) → markCompleted

[后扣费]
API Key auth(preflight 余额/订阅/平台配额/RPM)
  → Gateway Forward
  → RecordUsage / RecordCyberPolicyUsageLog
      → buildUsageBillingCommand
      → usage_billing_repo.Apply(dedup claim + 单事务 effects)
      → finalizePostUsageBilling(cache queue + platform quota incr + notify)
      → writeUsageLogBestEffort

[平台配额]
preflight: Redis entry / DB miss
post: Incr Redis(+dirty) → flusher 周期 SPOP+绝对快照 UPSERT
  或 flusher 关: 同步/聚合 DB IncrementUsageWithReset

[退款]
PrepareRefund → ExecuteRefund(status CAS → 先扣余额/天数 → gwRefund)
  → success: markRefundOk + void invoice/credit-note flag
  → pending: 审计持久化 balanceHeld → 终结成功保留扣减 / 失败回补
```

## 发现清单

### [P0] Anthropic/Claude 主路径 cyber 计费与 partial RecordUsage 双重扣费
- **位置**:
  - `backend/internal/handler/gateway_handler.go:930` + `:1015-1049`
  - `backend/internal/handler/gateway_cyber_policy.go:156-175`
  - 对照（正确）`backend/internal/handler/openai_gateway_handler.go:598-610`
- **相对上游**: 本地修改/扩展（cyber + partial billing 接缝）
- **问题**: Anthropic messages 在 `err != nil` 时先调用 `recordGatewayCyberPolicyIfMarked(..., forwardErrored=true)`，异步 `RecordCyberPolicyUsageLog` 已按 cyber token 扣费；同一错误分支在 `result != nil` 时又无条件 `RecordUsage`。OpenAI 路径有 `GetOpsCyberPolicy` 守卫并明确注释「勿二次 RecordUsage」，Claude 路径缺失。
- **影响**: 内容策略拦截且返回 partial/usage 的请求会**双重扣余额/订阅用量**。且两边 request_id 解析不同（cyber 用响应头 `X-Request-Id`，RecordUsage 用 `client:`/`local:`/`upstream`），`usage_billing_dedup` **无法去重**。
- **证据**:
  - cyber：`forwardErrored && gwSvc.RecordCyberPolicyUsageLog(...)`（`gateway_cyber_policy.go:156-175`）
  - partial：`if result != nil { ... RecordUsage(...) }` 无 cyber 判断（`gateway_handler.go:1015-1049`）
  - OpenAI 对照：`if service.GetOpsCyberPolicy(c) != nil { ... return }`（`openai_gateway_handler.go:602-609`）
- **建议**: 与 OpenAI 对齐：partial 分支先判断 `GetOpsCyberPolicy`；或 cyber 与 partial 共用同一 `resolveUsageBillingRequestID` 并保证 fingerprint 一致。补集成测试：cyber 命中 + non-nil result 只 Apply 一次。
- **交叉关注**: M01 Gateway / M05 内容审核（cyber 标记来源）

### [P1] Gemini messages 兼容路径 partial 补记且无 cyber 去重/记录
- **位置**: `backend/internal/handler/gateway_handler.go:515-583`（Gemini 分支）
- **相对上游**: 本地修改
- **问题**: 该错误分支在 `result != nil` 时直接 partial `RecordUsage`，**未**调用 `recordGatewayCyberPolicyIfMarked`，也无 cyber 守卫。若上游 cyber 标记已写入 gin context（或未来接入），会漏 cyber 审计或与其它路径语义漂移；即便当前 Gemini 少见 cyber mark，partial 补记与 Anthropic 主路径策略不一致，回归面大。
- **影响**: 计费归因/风控日志不完整；与 Claude 双扣风险同类结构债。
- **证据**: 该段仅 `recordOpsForwardLatencies` + failover/partial，无 cyber 调用；对比同文件 Claude 路径 `:930`。
- **建议**: 统一 gateway 错误终态：先 cyber record（带 guard），再 partial（跳过已 cyber）。
- **交叉关注**: M01 / M02 Gemini

### [P1] 余额后扣允许透支（soft overdraft），预检与实扣语义断裂
- **位置**: `backend/internal/repository/usage_billing_repo.go:262-291`；预检 `billing_cache_service.go:1195-1210`；`MinimumBalanceReserve` 仅预检
- **相对上游**: 本地强化后扣路径后仍保留该语义
- **问题**: `deductUsageBillingBalance` 先尝试 `balance >= amount`，失败则**无条件** `balance = balance - amount`（可负）。预检只保证「发起时」余额/reserve，长请求/并发 in-flight 可把余额打穿。`BalanceOverdrafted` 仅日志字段，无自动拦截/告警升级。
- **影响**: 高并发或大额单请求下用户负债；订阅用户不受此路径，余额模式 SaaS 资金风险。
- **证据**: 二次 `UPDATE ... WHERE id=$2 AND deleted_at IS NULL RETURNING balance` 无余额下限；注释写明 sufficient-balance guard missed 仍记账。
- **建议**: 配置化硬限制（禁止负余额 / 最大透支额）；透支时记指标+告警；可选「超 reserve 拒绝 Apply 并标记需人工」。
- **交叉关注**: M05 鉴权预检、M08 schema（若加约束）

### [P1] 平台配额 flusher 与 admin 写竞态可回写旧 usage（启用 flusher 时）
- **位置**: `backend/internal/service/user_platform_quota_flusher.go:181-190`；`flushOneBatch` SPOP→BatchGet→UPSERT
- **相对上游**: 本地新增 flusher
- **问题**: 代码自承：admin `ResetExpiredWindow`/`UpsertForUser`「先写 DB 再 DeleteCache」时，已 SPOP 的旧快照可覆盖 admin 新值；member 已出脏集无法拦截。默认 `user_platform_quota_flusher_enabled=false` 降低暴露，但生产若开启则实锤。
- **影响**: 强制重置未过期窗口后短暂（或直到下次窗口过期自愈前）**配额 enforcement 读到偏低/偏高旧 usage**；Redis 仍权威时可被错误 refresh。
- **证据**: 同文件注释 L181-190；FK 失败整批丢弃不 Readd（L198-206）导致低活跃 key DB 镜像偏低。
- **建议**: 启用前上 OCC version；或 admin 写走「写 DB + 写 Redis 绝对快照 + 代 SPOP 屏障」；FK 失败改为逐行重试。
- **交叉关注**: M08 迁移（version 列）、M06 运维配置默认值

### [P1] 发票活跃互斥仅应用层，无 DB partial unique
- **位置**: `backend/migrations/157_create_invoices_and_invoice_orders.sql:69-73`；`invoice_service.go:267-355`
- **相对上游**: 本地新增发票
- **问题**: 迁移明确「不在 DB 建 partial unique」；依赖 `SELECT FOR UPDATE` + 活跃链接查询 + 约束错误字符串匹配。SQLite/部分驱动跳过 `ForUpdate`（`:269-271`）。并发双开票在 PG 上大概率被事务串行挡住，但缺少硬唯一时运维手工写库/未来改事务边界易破。
- **影响**: 极端并发或非 PG 路径可能一张订单挂多张 APPLIED 发票；财务对账风险。
- **证据**: migration 注释 L69-71；`isInvoiceOrderActiveConstraintError` 靠错误信息模糊匹配。
- **建议**: 增加 `UNIQUE(order_id) WHERE is_active` partial index；Create 失败映射为明确 Conflict。
- **交叉关注**: M08 Schema/Migration

### [P2] 退款「先扣余额后调网关」在 rollback 失败时资损/用户受损
- **位置**: `payment_refund.go:518-561`、`:1183-1203`、`:1121-1128`
- **相对上游**: 本地修改（pending hold 语义已加固）
- **问题**: 成功路径设计合理（pending 持有、审计先于状态）。但 `RollbackRefund` 失败只写 `REFUND_ROLLBACK_FAILED` 审计；重试靠 `hasAuditLog` **跳过再次扣减**（避免双扣），用户余额可能已扣而网关未退——需人工。订阅 revoke 后 rollback 用 `ExtendSubscription` 未必等价恢复。
- **影响**: 低概率但资金相关；依赖 oncall 审计。
- **证据**: `ExecuteRefund` 扣减后 `gwRefund`；rollback 失败 return false + CRITICAL 日志。
- **建议**: 引入 refund_ledger 状态机；rollback 失败入 outbox 自动重试；订阅扣减用可逆凭证而非 revoke+extend。
- **交叉关注**: M10 运维告警

### [P2] 余额缓存异步 QueueDeduct 可丢更新 → 短窗超卖
- **位置**: `billing_cache_service.go:361-366`、`:396+` drop 逻辑；`gateway_service.go:10135-10150`
- **相对上游**: 本地/共享设计
- **问题**: 扣费 DB 成功后 `QueueDeductBalance`；队列满则 drop，Redis 余额偏高，预检放行直到耗尽路径 `InvalidateUserBalance` 或 DB 回源。
- **影响**: 瞬时超并发请求；与 soft overdraft 叠加放大负债。
- **证据**: `logCacheWriteDrop`；`syncBalanceCacheAfterDeduction` 仅在低于 reserve 时同步 invalidate。
- **建议**: 关键路径同步 DECR；drop 时同步 invalidate；指标告警。
- **交叉关注**: M06 缓存/outbox

### [P2] legacy `postUsageBilling` 无 dedup 且分步失败可部分记账
- **位置**: `gateway_service.go:9891-9963`、`applyUsageBilling` `:10059-10061`
- **相对上游**: 保留兜底
- **问题**: `usageBillingRepo == nil` 时走 legacy：订阅/余额/APIKey/账号配额各自独立写，失败只 log，无 `usage_billing_dedup`。生产应注入 repo，但降级/错误装配会静默进入。
- **影响**: 双重调用或中途失败导致漏扣/多重不一致。
- **证据**: 注释 “tests or degraded mode”；`if repo == nil { postUsageBilling; return true }`。
- **建议**: 生产启动断言 repo 非 nil；legacy 打 ERROR 指标。
- **交叉关注**: Wire/DI（M08）

### [P2] 批图结算 capture 成功与 MarkSettled 非同一事务
- **位置**: `batch_image_settlement.go:152-180`；hold 幂等 `usage_billing_repo.go:114-174`
- **相对上游**: 本地新增
- **问题**: capture 与 job 终态分步。依赖 capture request_id 幂等使重试安全；若 MarkSettled 持续失败，资金已 capture 但任务非 completed，用户感知卡住（重试可修复）。release 指纹冲突当成功（防毒消息）可能掩盖真实未释放。
- **影响**: 运维复杂性；极端下冻结/实扣与 job 状态短暂不一致。
- **证据**: `captureBatchImageBalanceHold` 后 `MarkBatchImageJobSettled`；`release` 对 `ErrUsageBillingRequestConflict` 当成功。
- **建议**: 状态机 + 对账任务；settled 标记与 billing 同 outbox。
- **交叉关注**: M07 媒体任务

### [P2] 订阅窗口维护热路径曾依赖异步，当前 auth 已同步 EnsureWindowMaintenance
- **位置**: `subscription_service.go:1028-1115`；`api_key_auth.go:200-211`
- **相对上游**: 本地修改（auth 侧已改为 needsMaintenance 时同步 Ensure）
- **问题**: `ValidateAndCheckLimits` 仍会在内存清零过期窗口 usage；若其它调用方只调 Validate 不 Ensure，仍可能用「内存已重置、DB 未重置」快照做后续逻辑。`DoWindowMaintenance` 异步队列仍存在。
- **影响**: 非 auth 调用面若复用 Validate 可能误判限额。
- **证据**: `ValidateAndCheckLimits` 注释「DB 由 DoWindowMaintenance 异步」；auth 已同步 Ensure。
- **建议**: 废弃「仅 Validate」对外用法；统一强制 Ensure API。
- **交叉关注**: M05 鉴权

### [P3] simple 模式跳过全部计费检查与扣费
- **位置**: `api_key_auth.go:136-149`；`billing_cache_service.go:997-999`；`gateway_service.go:10557-10563`
- **相对上游**: 共享设计
- **问题**: 配置错误把生产设为 simple 会免费放行。属运行模式契约，非逻辑 bug。
- **影响**: 运维误配资损。
- **建议**: 启动日志高亮 RunMode；生产部署校验禁止 simple（除非显式 allow）。
- **交叉关注**: M10 部署

### [P3] 支付取消/过期 grace 后仍可恢复履约（有意）
- **位置**: `payment_fulfillment.go:154-255`；`paymentGraceMinutes=5`
- **相对上游**: 本地加固
- **问题**: grace 外 cancelled/expired 仍 `markTerminalOrderPaidAndReload` 履约，防用户已付未到账。正确性上偏「宁可履约」，需财务知悉可能对已取消单发货。
- **影响**: 产品/财务流程，非纯 bug。
- **建议**: 仪表盘暴露 ORDER_RECOVERED / PAYMENT_AFTER_* 审计。
- **交叉关注**: 无

## 与上游合并风险
- **冲突热点文件**:
  - `backend/internal/service/gateway_service.go`（RecordUsage/billing 巨石）
  - `backend/internal/service/payment_fulfillment.go` / `payment_refund.go`
  - `backend/internal/service/billing_cache_service.go`
  - `backend/internal/handler/gateway_handler.go` / `openai_gateway_handler.go`
  - `backend/internal/repository/usage_billing_repo.go`
  - migrations `071+` dedup、`138` audit unique、`157` invoices、platform quota 相关
- **语义漂移点**:
  - 上游可能仍是分步扣费；本地 `usage_billing_dedup` 为权威幂等键（`request_id+api_key_id`）
  - 订阅履约 SUCCESS 哨兵与 assign 同事务——合并时勿回退到「先 assign 再写审计」
  - platform quota：Redis 权威 vs DB 镜像；flusher 开关改变写路径
  - OpenAI 已修 cyber 双扣，Anthropic 未对齐——合并时易只带一侧

## 测试与验证缺口
- **缺**: Anthropic/Gemini「cyber 命中 + partial result」只扣一次的集成测试（OpenAI 有 cyber idempotent 单测思路）
- **缺**: `usage_billing` 并发透支上限/reserve 交叉的资金不变量测试
- **缺**: flusher_enabled=true 下 admin reset 与 flush 交错的竞态测试
- **缺**: 发票并发 Create 双请求（真 PG `FOR UPDATE`）压测
- **有**: payment fulfillment/refund 大量 unit 测试；usage_billing_repo integration；invoice atomic rollback 测试；batch image settlement 幂等测试
- **建议验证**:
  - `go test -tags=unit ./internal/service/ -run 'Payment|Invoice|Billing|UsageBilling|BatchImage|PlatformQuota'`
  - `go test -tags=integration ./internal/repository/ -run UsageBilling`
  - 手工：webhook 重放、退款 pending 终结、流式中断 partial 账单对账

## 模块结论
- **整体风险评级**: **High**
- **是否建议合入上游 / 继续分叉 / 先修再合**: **先修再合**（至少修 P0 Anthropic cyber 双扣；P1 透支策略与发票唯一约束应有明确决策）
- **Top 3 必须处理项**:
  1. **修复 Anthropic/Claude cyber + partial 双重计费**（对齐 OpenAI 守卫 + 统一 billing request_id）
  2. **明确余额透支策略**（禁止负余额或限额 + 可观测性；避免与缓存 drop 叠加）
  3. **发票 order 活跃唯一下沉 DB**；启用 platform quota flusher 前补 OCC/竞态防护
