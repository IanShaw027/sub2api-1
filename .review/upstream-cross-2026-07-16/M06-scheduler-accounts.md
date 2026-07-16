# Module 6: 调度账户并发与 Outbox

## 范围摘要
- 对比基线: `BASE=da85cc7…` → `HEAD=eb64a5c…`，上游 `UPSTREAM=b960ec1…`（`upstream/main`）
- 本模块焦点路径:
  - `backend/internal/service/scheduler_*.go`、`gateway_service.go`（SelectAccount / sticky / wait plan）
  - `backend/internal/service/concurrency_service.go`、`openai_gateway_service.go` / `openai_account_scheduler.go`
  - `backend/internal/repository/scheduler_outbox_repo.go`、`scheduler_cache.go`、`concurrency_cache.go`、`account_repo.go`
  - `backend/internal/service/admin_service.go`（BulkUpdateAccounts）
  - `backend/internal/service/openai_oauth_capacity*.go`
  - `backend/migrations/036_*` / `152_*` / `153_*` / `202_transactional_scheduler_outbox_triggers.sql`
- 相对上游的主要能力差异（本地/分叉侧强化点）:
  - 调度快照 + `scheduler_outbox` claim/ack/lease + lag/backlog 触发全量重建
  - DB trigger 事务内入队 outbox（membership 变更直接 `full_rebuild`）
  - 多实例并发槽位（Redis ZSET + requestID 前缀 + 心跳续约 + 进程重启 stale cleanup）
  - Anthropic 负载感知分层（模型路由 → sticky → load → fallback wait）
  - OpenAI 独立调度器（sticky reserve / fresh-session admission / temporary recovery wait）
  - temp-unsched 规则 + 阈值窗口 + failover 同账号重试耗尽冷却
  - bulk update 跨平台 credentials/extra 硬拦截；OpenAI OAuth capacity planning 面板

## 关键路径图
```
Admin/Repo 写 accounts|account_groups|groups
  ├─ (同事务) enqueue scheduler_outbox (+ 可选 DB trigger 二次入队, dedup 合并)
  └─ syncSchedulerAccountSnapshot (紧急路径: rate-limit/temp-unsched 立即刷 Redis account)

SchedulerSnapshotService.pollOutbox (每实例)
  ClaimPending(FOR UPDATE SKIP LOCKED, lease=2.5m, 清 dedup_key)
  → handle event (account/group/bulk/last_used/full_rebuild)
  → rebuildBucket (TryLockBucket + DB load + SetSnapshot)
  → AckClaim(token) 或 ReleaseClaim

Gateway / OpenAI Gateway 请求
  ListSchedulableAccounts(snapshot cache → 受控 DB fallback)
  → sticky / model-routing / load-batch 排序
  → AcquireAccountSlotForGroup (Redis ZSET, multi-instance 原子)
  → hydrateSelectedAccount (从 account cache 取完整凭据)
  → handler: WaitPlan 时 IncrementAccountWaitCount + AcquireWithWaitTimeout
  上游失败 → RateLimitService / TempUnscheduleRetryableError
           → SetTempUnschedulable + outbox + snapshot 同步
           → failover 排除 excludedIDs 重新 Select
```

## 发现清单

### [P1] Outbox lag/backlog 统计把「已 claim 未 ack」事件当成 pending
- **位置**: `backend/internal/repository/scheduler_outbox_repo.go:107-127`；调用方 `backend/internal/service/scheduler_snapshot_service.go:745-800`
- **相对上游**: 本地强化 outbox lag rebuild 后的语义漏洞
- **问题**:
  - `OldestPendingCreatedAt` = `SELECT MIN(created_at) FROM scheduler_outbox`（无过滤 `claimed_at`）
  - `PendingCount` = `SELECT COUNT(*) FROM scheduler_outbox`（同样包含已 claim 行）
  - claim 后处理超时为 `outboxEventTimeout=2m`，而默认 `OutboxLagRebuildSeconds=10`、`OutboxLagRebuildFailures=3`
  - 任何耗时 rebuild（大分组、锁竞争）都会让「正在处理」的事件把 lag 推高，误触发 full rebuild；backlog 阈值同理虚高
- **影响**: 多实例/高峰时 outbox lag rebuild 抖动，重复全量重建放大 Redis/DB 压力，调度快照抖动导致短暂选错池或 DB fallback
- **证据**:
  ```go
  // OldestPendingCreatedAt
  SELECT MIN(created_at) FROM scheduler_outbox
  // PendingCount
  SELECT COUNT(*) FROM scheduler_outbox
  ```
  对比 claim 条件使用 `claimed_at IS NULL OR claimed_at < NOW()-lease`，但 lag 指标未对齐。
  单测 `PollOutboxLagIgnoresAcknowledgedEvent` 只覆盖 **ack 后表空**，不覆盖 claim 中/处理中窗口。
- **建议**:
  - lag/backlog 仅统计 claimable 行: `claimed_at IS NULL OR claimed_at < NOW()-lease`
  - 或单独暴露 `in_flight` 指标，避免与 lag rebuild 阈值混用
  - 补集成测: claim 后未 ack 时不得累计 lagFailures / 不得触发 backlog rebuild
- **交叉关注**: M08（outbox schema/index）、M10（运维告警噪声）

### [P1] Claim 清空 `dedup_key` 后未在 release 时恢复，处理中窗口可插入重复逻辑事件
- **位置**: `backend/internal/repository/scheduler_outbox_repo.go:44-48,98-104,156-164`；`scheduler_snapshot_service.go:379-385`
- **相对上游**: 本地 claim-lease + pending dedup 组合
- **问题**:
  - `ClaimPending` 成功时 `dedup_key = NULL`
  - `ReleaseClaim` 只清 `claimed_at/claim_token`，**不恢复** dedup
  - 因此「claim → 处理中/失败 release」窗口内，同 `account_id` 的 `account_changed` 可再次 INSERT（DB trigger 与 app enqueue 的 dedup 失效）
  - 与 P1-lag 叠加：重复事件拉长队列 → 更易 lag rebuild
- **影响**: outbox 风暴、重复 bucket rebuild、多实例下 CPU/Redis 写放大；一般不丢更新（偏向 at-least-once），但会损害稳定性
- **证据**: claim SQL 显式 `dedup_key = NULL`；`ReleaseClaim` 仅 `SET claimed_at=NULL, claim_token=NULL`
- **建议**:
  - release 时恢复 dedup（需 claim 时暂存 dedup 或按 event 重算）
  - 或 claim 期间保留 dedup，仅在 Ack 删除时消除；用 `(dedup_key) WHERE pending` 与 lease 语义一致
  - 对 `account_groups` membership 的 `full_rebuild` 单 key 去重也要验证 claim 窗口
- **交叉关注**: M08 迁移 `152/153`、DB trigger `202_transactional_scheduler_outbox_triggers.sql`

### [P1] Gateway 负载分母用 `EffectiveLoadFactor`，抢槽上限用 `Concurrency` —— score/权重口径不一致
- **位置**:
  - 负载: `gateway_service.go:2121-2138,2377-2400` + `concurrency_cache.go:972-975`
  - 抢槽: `gateway_service.go:2167,2423` → `tryAcquireAccountSlot(..., account.Concurrency)`
  - `account.go:146-157` `EffectiveLoadFactor()`
- **相对上游**: 本地 load-batch 调度强化后的一致性 bug
- **问题**:
  - `LoadRate = (currentConcurrency + waiting) * 100 / MaxConcurrency`，其中 `MaxConcurrency` 传入的是 `EffectiveLoadFactor()`（可配置 `load_factor`，默认回落 concurrency）
  - 真正 `AcquireAccountSlot` 的上限是 `account.Concurrency`
  - 当 `load_factor > concurrency` 时：账号在 `LoadRate<100` 时仍可能槽位已满，造成无效尝试与错误排序
  - 当 `load_factor < concurrency` 时：过早被判满载（`LoadRate>=100`），**人为压低可用容量**，把流量挤到其他账号/等待队列
- **影响**: 池内权重失真 → 热点账号过载或容量浪费；与 OpenAI 路径（`selectionMaxConcurrency` 同时用于负载与 acquire）行为分叉
- **证据**:
  ```go
  MaxConcurrency: acc.EffectiveLoadFactor(), // load batch
  // ...
  if loadInfo.LoadRate < 100 { ... }
  tryAcquireAccountSlot(ctx, id, groupID, item.account.Concurrency) // hard limit
  ```
- **建议**:
  - 统一：load 分母与 acquire 上限同源（建议以 `Concurrency` 为硬上限；`LoadFactor` 若要做 soft weight，应进入排序 score 而非 ZSET 分母）
  - 单测覆盖 `load_factor != concurrency` 的排序与可用性过滤
- **交叉关注**: M01/M02 gateway failover 选号质量；前端 load_factor 配置文案

### [P1] `PreferSoonestReset` 配置项存在但 `schedulingConfig()` 未注入，功能永久关闭
- **位置**: `config.go:1211-1214`；`gateway_service.go:2412-2414` vs `2491-2525`
- **相对上游**: 本地新增 use-it-or-lose-it 开关但接线缺失
- **问题**: `SelectAccountWithLoadAwareness` 读取 `cfg.PreferSoonestReset`，但 `schedulingConfig()` 只拷贝 sticky/fallback/load_batch 等字段，**从不**赋值 `PreferSoonestReset`，Go zero-value 恒为 `false`
- **影响**: 运维按文档打开 `prefer_soonest_reset` 无效；会话窗口即将重置的账号无法被优先抽干，配额浪费（尤其 Anthropic OAuth session window）
- **证据**: `schedulingConfig` 返回结构无 `cfg.PreferSoonestReset = runtimeCfg.PreferSoonestReset`
- **建议**: 一行注入 + 单测断言 runtime 配置透传；OpenAI path 若需要同类策略单独评估
- **交叉关注**: M04 配额窗口、OpenAI capacity planning

### [P1] WaitPlan 路径提前 `RegisterSession`，等待失败/超时不释放 → max_sessions 被占坑
- **位置**: `gateway_service.go:2186-2198,2280-2295,2450-2460`；`checkAndRegisterSession:3168-3190`；handler 等待 `gateway_handler.go:423-473`
- **相对上游**: 本地 session limit + wait plan 接缝
- **问题**:
  - 返回 WaitPlan 前调用 `checkAndRegisterSession`（Redis ZSET 注册 session）
  - handler 侧等待失败（queue full / timeout / client cancel）只 `DecrementAccountWaitCount` + 不拿槽，**没有 UnregisterSession**
  - 会话坑位依赖 idle timeout 自然过期
- **影响**: Anthropic OAuth/SetupToken 开了 `max_sessions` 时，高峰排队失败可快速耗尽会话配额 → 后续真实请求被拒，表现为「有账号但不可调度」；与 sticky 叠加后更难自愈
- **证据**: service 层 WaitPlan 分支 `checkAndRegisterSession` 成功即返回；repository `session_limit_cache` 仅有 Register/Refresh，无对称 Unregister API 被 handler 使用
- **建议**:
  - WaitPlan 阶段只做 **只读** 会话名额预检，真正拿到 slot 后再 Register
  - 或提供 `UnregisterSession`/compare-and-delete，在 wait 失败路径释放
  - 单测: wait timeout 后 active session count 不增加
- **交叉关注**: M01 Anthropic messages failover、M05 会话粘性

### [P1] 调度 Redis 元数据缓存保留明文 `api_key`
- **位置**: `backend/internal/repository/scheduler_cache.go:842-876`（`filterSchedulerCredentials` 白名单含 `"api_key"`）
- **相对上游**: 本地 snapshot 凭据过滤策略
- **问题**: 调度元数据本意为「去密钥后的选号视图」，却把 `api_key` 写入 Redis account/snapshot；hydration 测试也依赖缓存内 `sk-live`
- **影响**: Redis 被拖库/越权读时直接泄漏上游 API Key；多副本共享缓存扩大暴露面。OAuth token 虽被滤掉，API Key 账号仍高危
- **证据**: 白名单显式包含 `api_key`；`scheduler_snapshot_hydration_test.go` 期望 hydrate 后 `GetOpenAIApiKey()=="sk-live"`
- **建议**:
  - snapshot 列表路径去掉 secret；选中后强制 `GetByID`/专用 secret store hydrate
  - 若为性能保留，至少加密字段 + ACL 隔离 + 审计
- **交叉关注**: M05 密钥治理

### [P2] Outbox 单事件失败会中断整批 poll，放大 lag
- **位置**: `scheduler_snapshot_service.go:359-397`
- **相对上游**: 本地 poll 循环
- **问题**: handle 失败 → `ReleaseClaim` → `return`，本轮剩余最多 199 个 claimable 事件不处理，直到下一 poll interval
- **影响**: 与 P1 lag 误判叠加时，故障事件可卡住同实例吞吐；多实例可部分缓解（SKIP LOCKED），但坏事件反复 claim/fail 仍占 lease
- **建议**: 单事件失败 continue；对 poison message 计次进死信/延迟；区分 `ErrSchedulerBucketLocked`（可立即 requeue）与硬错误
- **交叉关注**: M10 可观测性

### [P2] `account_groups` 变更 trigger 强制 `full_rebuild`（粗粒度）
- **位置**: `backend/migrations/202_transactional_scheduler_outbox_triggers.sql:145-160`
- **相对上游**: 本地 transactional outbox
- **问题**: membership 任意 INSERT/UPDATE/DELETE 都写 `full_rebuild`（dedup key 固定 `dbtrigger:full_rebuild`）。正确性偏好强，但大集群绑定调整会触发全局 rebuild
- **影响**: 管理端批量改绑定时调度缓存抖动；与 app 侧 `account_groups_changed` 精细路径并存，语义重叠
- **建议**: trigger 改为带 old/new group_ids 的 `account_groups_changed`/`account_bulk_changed`；full_rebuild 仅作 lag 兜底
- **交叉关注**: M08 schema、M09 bulk bind UI

### [P2] BulkUpdate credentials 使用 JSONB `||` 浅合并；跨平台已拦，但同平台异构 type 仍可能污染
- **位置**: `admin_service.go:3790-4002`；`account_repo.go:1859-1867`
- **相对上游**: 本地 bulk + 跨平台防护
- **问题**:
  - **已修复点**: `validateBulkModelRoutingUpdatePlatforms` 拒绝跨平台 credentials/extra（有单测 `RejectsModelMappingAcrossMixedPlatforms`）——符合 AGENTS.md 约束
  - 残留: 同平台不同 type（如 OpenAI oauth + apikey）批量写 `credentials`/`extra` 时，`||` 整键覆盖 `model_mapping`；平台特异键可能写到不应持有的账号类型
  - OpenAI `extra` 批量已降级为逐账号 Update（避免 long-context 等字段误伤），但 credentials 仍走统一 BulkUpdate
- **影响**: 运维误操作导致 mapping/能力开关污染；调度侧按 mapping 过滤模型 → 大面积 404/不可用
- **建议**: credentials bulk 也按 type 分组或仅允许安全键子集（concurrency/priority/load_factor 走列更新）
- **交叉关注**: M09 管理端 bulk 表单、M02 模型映射

### [P2] Gateway vs OpenAI 调度策略分叉，failover 接缝行为不一致
- **位置**: `gateway_service.go` Layer1-3 vs `openai_gateway_service.go:selectAccountWithLoadAwareness`
- **相对上游**: 本地双调度器
- **问题**:
  - Gateway: `LoadRate` 含 waiting；OpenAI: 可用判定主要看 `CurrentConcurrency < limit`，waiting 仅影响排序
  - OpenAI 有 sticky reserve / temporary recovery `NotBefore` wait；Gateway temp-unsched 更依赖 snapshot 字段
  - 两者 `schedulingConfig()` 都丢了 `PreferSoonestReset`
- **影响**: 同集群不同协议入口「看起来有容量/实际排队」体验不一致；排障困难
- **建议**: 抽取共享 `AccountAdmission` 策略接口；文档化差异；对齐 waiting 是否计入饱和
- **交叉关注**: M01 OpenAI、M02 多平台

### [P2] 多实例槽位心跳续约失败仅打日志，不主动 fail 请求
- **位置**: `concurrency_service.go:496-511,334-360`；`concurrency_cache.go:66-108`
- **相对上游**: 本地 long-request heartbeat
- **问题**: 心跳 `AcquireAccountSlot` 续约失败只 `LegacyPrintf`；若 Redis 抖动导致 score 过期被 ZREMRANGE，其他实例可抢走槽位 → 超卖
- **影响**: 超长请求（> slot TTL 且心跳失败）期间账号并发可被击穿，触发上游 429，再进入 temp-unsched 反馈环
- **建议**: 连续 N 次 renew 失败时 cancel 请求 context（对齐 OpenAI WS ingress lease lost 语义）；指标化 renew 失败率
- **交叉关注**: M01 长流式、M10 指标

### [P3] OpenAI OAuth capacity overview 缓存仅进程内，多副本 force 刷新 best-effort
- **位置**: `openai_oauth_capacity.go:207-242`
- **相对上游**: 本地 capacity planning
- **问题**: 注释已承认 multi-replica 下 force 只更新本进程；管理端看到的容量可能跨实例不一致
- **影响**: 运维决策噪声，非请求路径正确性
- **建议**: Redis 共享缓存 + version；或 sticky admin session
- **交叉关注**: M09 capacity UI

### [P3] sticky / scheduling 入口 Info 级日志偏多
- **位置**: `gateway_service.go:1837-1846` 等 `slog.Info("sticky.scheduler_entry")`
- **相对上游**: 本地调试残留倾向
- **问题**: 每请求 Info 日志在高 QPS 下成本高
- **影响**: 日志噪声、成本；不直接影响正确性
- **建议**: 降为 Debug 或采样；保留 structured 字段给 tracing
- **交叉关注**: M10

## 与上游合并风险
- **冲突热点文件**:
  - `backend/internal/service/gateway_service.go`（SelectAccount 巨型函数）
  - `backend/internal/service/openai_gateway_service.go` / `openai_account_scheduler.go`
  - `backend/internal/service/concurrency_service.go` + `repository/concurrency_cache.go`
  - `backend/internal/service/scheduler_snapshot_service.go` + `repository/scheduler_outbox_repo.go`
  - `backend/internal/service/admin_service.go` BulkUpdate
  - `backend/migrations/202_transactional_scheduler_outbox_triggers.sql` 及 outbox 索引迁移
- **语义漂移点（同名不同义）**:
  - `MaxConcurrency` 在 load-batch 中是 **LoadFactor 分母**，在 acquire/WaitPlan 中是 **硬槽位**
  - `account_changed`：app enqueue 带 `group_ids` payload；DB trigger 可能仅 `account_id` + 不同 dedup 命名空间（`scheduler_outbox:sha` vs `dbtrigger:account:{id}`）
  - `full_rebuild`：interval / lag / membership trigger / 显式 outbox 多源合流，coalesce 逻辑本地特有
  - OpenAI `selectionMaxConcurrency`（含 sticky reserve） vs Anthropic `account.Concurrency`

## 测试与验证缺口
- 缺: claim 中 outbox **不得**抬升 lag/backlog 的集成测
- 缺: claim 清空 dedup 后并发 enqueue 行为契约测
- 缺: `load_factor != concurrency` 的排序/过滤/抢槽一致性测
- 缺: `PreferSoonestReset` 配置透传测（当前功能死代码）
- 缺: WaitPlan 失败后 session count 不泄漏测
- 有: outbox ack/release/lock-miss 单测；bulk 跨平台拒绝单测；slot heartbeat/release 单测；full rebuild coalesce 单测；OpenAI capacity 纯函数测
- 建议压测: 多实例 `-count` 下 slot 超卖、outbox lag rebuild 风暴、temp-unsched + failover 交叉

## 模块结论
- **整体风险评级: High**
- **是否建议合入上游 / 继续分叉 / 先修再合**: **先修再合**（尤其 outbox lag 误触发、load 权重口径、session 占坑、Redis 明文 api_key）；bulk 跨平台防护与 claim-token outbox 方向值得回馈上游，但需先收口上述 P1
- **Top 3 必须处理项**:
  1. **Outbox lag/backlog 仅统计 claimable 行 + claim/dedup 生命周期闭环**（防误全量重建与队列风暴）
  2. **统一 LoadFactor/Concurrency 调度口径**（或明确 soft-weight 模型），并修好 `PreferSoonestReset` 配置透传
  3. **WaitPlan 与 max_sessions 解耦 + 调度缓存去掉明文 api_key**（可用性 + 安全）
