# Module 8: Schema / Migration / Ent 领域模型

## 范围摘要
- 对比基线: `BASE=da85cc7e` → `HEAD=eb64a5c1`；上游对照 `UPSTREAM=b960ec19`（`upstream/main`）
- 本模块审查路径:
  - `backend/migrations/**`（含 runner 回归测 `auth_identity_payment_migrations_regression_test.go`）
  - `backend/ent/schema/**`、`backend/ent/migrate/**`
  - `backend/internal/domain/**`
  - `backend/internal/repository/migrations_runner.go` + integration 对齐测试
  - `backend/cmd/sync_checksums/**`（checksum 运维入口，与 M10 交界）
  - wire 仅只读：DI 不直接驱动迁移，迁移在启动路径 `ApplyMigrations` 执行
- 变更规模: 迁移树体量极大（约 001–215+，大量同号并行文件）；本地相对上游显著增厚 AI Studio/Skill、批量生图、发票、IP 多账号、OpenAI OAuth capacity、outbox claim、余额缓存 outbox 等 schema
- 相对上游的主要能力差异（本地增强）:
  - 自定义 SQL 迁移 runner（非 Atlas 主路径）+ SHA256 不可变校验 + 历史误改 allowlist
  - `*_notx.sql` + `CREATE/DROP INDEX CONCURRENTLY` 在线索引路径 + 部分唯一约束 precheck
  - 支付/发票/affiliate/subscription fulfillment 的 partial unique 加固
  - AI Skill / AI Center / batch image / invoice / IP security / capacity hourly 等运营表
  - `sync_checksums` 部署期 allowlist 同步与 snapshot restore
  - domain 平台常量含 `grok`/`kiro`/`sora` 与网关平台列表分流

## 关键路径图
```
[进程启动]
    │
    ▼
ApplyMigrations(db)  ── pg_advisory_lock(694208311321144027)
    │
    ├─ CREATE schema_migrations (filename PK, checksum, applied_at)
    ├─ embed FS: backend/migrations/*.sql  → sort.Strings(filename)
    │
    ├─ 已应用:
    │     checksum 相等 → skip
    │     不等 → IsMigrationChecksumCompatible(allowlist) ? skip : 硬失败
    │
    ├─ 未应用:
    │     validateMigrationExecutionMode
    │       ├─ 普通 *.sql  → BEGIN; Exec(整文件); INSERT schema_migrations; COMMIT
    │       └─ *_notx.sql → prepareNonTransactional*(dup/invalid-index)
    │                        → 逐条 CREATE/DROP INDEX CONCURRENTLY
    │                        → INSERT schema_migrations（无事务包裹 DDL）
    │
    └─ ensureAtlasBaselineAligned（仅当 atlas_schema_revisions 为空时插入 HEAD 基线）

[部署脚本 deploy.sh]
    sync_checksums --backup-file  → 按 allowlist 改写 DB checksum
    restart binary → ApplyMigrations forward-only
    失败 rollback → restore checksum 快照 + 旧二进制（不回滚 DDL）  ← 见 M10 P0
```

## 发现清单

### [P0] 启动期对热表做事务内全表扫描/回填/建索引，大库可长时间阻塞流量
- **位置**:
  - `backend/migrations/207_ip_multi_account_security.sql:7-27`（`usage_logs` 聚合回填 `users.ips` + 事务内 `CREATE INDEX ... ON usage_logs`）
  - `backend/migrations/213_backfill_usage_log_video_seconds.sql:1-4`（`usage_logs` 全表 `UPDATE`）
  - `backend/migrations/191_restore_usage_request_type_cyber_value.sql:22-51`（`usage_logs`/`ops_error_logs` 多段全表 `UPDATE`）
  - `backend/migrations/198_widen_batch_image_wallet_precision.sql:3-5`（`users.balance`/`frozen_balance` `ALTER TYPE`，需重写列）
  - Runner: `backend/internal/repository/migrations_runner.go:259-281`（普通迁移整文件包事务）
- **相对上游**: 本地新增/加厚的治理与计费 schema；执行模型与上游同类「启动自动 migrate」一致，但本地热表变更更激进
- **问题**:
  1. 迁移在 **服务启动** 自动执行，且 advisory lock 串行化多实例——期间其他实例也在等锁。
  2. `207` 在事务内对 `usage_logs` 做 `ARRAY_AGG(DISTINCT ip_address) GROUP BY user_id` 再 `UPDATE users`，并对 `usage_logs` 建普通（非 CONCURRENTLY）索引；大分区表会长时间持锁/高 IO。
  3. `213`/`191` 对 `usage_logs` 全表写放大，与 README「Avoid long-running UPDATE/ALTER on hot tables」自相矛盾（`backend/migrations/README.md:7-11`）。
  4. `198` 改 `users.balance` 精度会触发表重写 + `AccessExclusiveLock`，支付/计费高峰升级风险极高。
- **影响**: 生产升级窗口可演变为 **全站不可用**（启动卡在 migrate）；副本滚动升级时旧流量仍打热表，锁冲突放大。属生产可用性 P0。
- **证据**:
  ```sql
  -- 207: 启动事务内热表聚合 + 索引
  WITH historical_ips AS (
      SELECT user_id, ARRAY_AGG(DISTINCT ip_address) AS ips
      FROM usage_logs ...
  ) UPDATE users ...
  CREATE INDEX IF NOT EXISTS idx_usage_logs_ip_created_at_security ON usage_logs (...);
  ```
  ```sql
  -- 213: 无批次、无限流
  UPDATE usage_logs SET video_seconds = video_duration_seconds
  WHERE video_seconds IS NULL AND video_duration_seconds IS NOT NULL;
  ```
- **建议**:
  1. 热表回填拆成离线 job / batched `UPDATE ... WHERE ctid` / 按分区推进；启动迁移只做「加可空列 + 默认值」。
  2. `usage_logs` 索引一律 `*_notx.sql` + `CREATE INDEX CONCURRENTLY IF NOT EXISTS`，并纳入 `prepareInvalidIndexRetry`。
  3. `ALTER TYPE` 类钱包精度变更要求维护窗口 + 逻辑备份；考虑 `USING` 前预检锁等待与表体积。
  4. CI/发布门禁：对匹配 `usage_logs|payment_orders|users` 的非 notx 迁移做静态扫描告警。
- **交叉关注**: M05（IP 安全）、M04（余额精度/计费）、M06（outbox）、M10（部署窗口）

### [P0] 迁移编号前缀大规模碰撞 + 字典序调度，与上游 merge 时极易「同号异义 / 静默双轨」
- **位置**: `backend/migrations/` 全树；调度 `migrations_runner.go:175-181`（`sort.Strings(files)` 按 **完整文件名** 排序，**不解析**数字版本）
- **相对上游**: 本地与上游各自用三位/四位前缀抢号；TASKS 已知「remote prefix collision / 216 capacity repair」类事件。当前 HEAD 容量相关为 `214_*`/`215_*`，**无** `216_repair_*` 文件——说明碰撞后靠人工改号止血，但系统性缺陷仍在。
- **问题**:
  1. **同号多文件已是常态**，例如：
     - `006_*` ×3、`028_*` ×3、`101_*` ×4、`162_*` ×4、`191_*` ×2（另有 `191a_*`）
     - `176_add_tls_fingerprint_profile_transport.sql` 与 `176_tls_fingerprint_capture_unification.sql` **同号不同义**
  2. 执行顺序完全依赖字符串排序：`191_add_*` → `191_restore_*` → `191a_validate_*` 碰巧正确，但 `006b_*` 插在 `006_*` 之后依赖 `b` 字符，**无机器校验依赖边**。
  3. 与 `upstream/main` 合并时：双方若各自新增 `216_foo.sql` 与 `216_bar.sql`，git 可能 **无文本冲突**（不同文件名），runner 会 **两个都执行**；若一方改名修复、另一方已 applied 旧名，则出现「一边 schema_migrations 有 A、另一边有 B、物理 schema 部分重叠」的不可逆分叉。
  4. Atlas 基线取 `ORDER BY filename DESC LIMIT 1`（`migrations_runner.go:608-627`），在前缀碰撞世界里「最新文件名」≠「语义最新版本」。
- **影响**: 合并上游/多环境晋升时 **最高频的数据面事故源**；checksum 只能保护「同 filename 内容不变」，**不能**保护「同序号不同 filename」。
- **证据**: 目录内大量同号并行文件（list `backend/migrations/101_*`、`162_*`、`176_*`、`191_*`）；runner 仅 `sort.Strings`；README 仍写「Sequential number」仿佛全局唯一（`README.md:23-27`）与现实不符。
- **建议**:
  1. 短期：维护 `migrations/MANIFEST` 或 CI 检查「数字前缀全局唯一（含 a/b 后缀规则）」；禁止再开同号文件。
  2. 合并上游 SOP：先 `comm -12 <(local prefixes) <(upstream prefixes)`，冲突一律 **双方改用更高未占用号** 并写 repair 迁移，而非 in-place 改已 applied 文件。
  3. 中期：迁移 ID 改为单调 `YYYYMMDDHHMMSS_desc.sql` 或引入单版本链（goose/atlas 版本表），淘汰三位数抢号。
  4. 对已知容量线：文档固化 `214` snapshot / `215` hourly 为本地权威，上游若占用同号必须 remap。
- **交叉关注**: M06（outbox 152/153/202/208/210）、M04（支付 119/120）、M10（发布协调）

### [P1] Checksum「兼容放行」不回写 DB，且历史 in-place 改 migration 依赖不断膨胀的 allowlist
- **位置**:
  - `backend/internal/repository/migrations_runner.go:77-113`（`migrationChecksumCompatibilityRules`，含 148/151/156/169/176×2/181/185/187/188/195/199 等）
  - 同文件 `203-222`：allowlist 命中后 **`continue`，不 UPDATE schema_migrations.checksum**
  - `backend/cmd/sync_checksums/main.go:28-31,198-227`（部署时才按同一 allowlist 改写）
  - restore 路径 `171-196`：**无 allowlist**，可整表写回任意快照
- **相对上游**: 本地为消化误改历史 migration 而加的运维补偿；上游若未同步 allowlist 则无法启动同一 DB
- **问题**:
  1. 设计原则写「applied migration 不可改」（README L65-73），实现却用 allowlist **合法化改写**，且条目持续增加（注释明确 `273032e08` 改了已落地文件）。
  2. 启动路径只 **跳过校验** 不收敛 DB 状态 → 未跑 `sync_checksums` 的环境永久依赖 allowlist 内存表；删规则即大面积起不来。
  3. 与 M10 叠加：`deploy.sh` 失败 restore checksum **不回滚 DDL**，可造成 schema 新 / checksum 旧 / 二进制旧 三方错位。
- **影响**: 多环境 checksum 漂移；合并上游时 allowlist 冲突；误操作 restore 可掩盖真实篡改。
- **证据**:
  ```go
  if existing != checksum {
      if isMigrationChecksumCompatible(name, existing, checksum) {
          continue // DB 仍保留 old checksum
      }
      return fmt.Errorf("migration %s checksum mismatch ...")
  }
  ```
- **建议**:
  1. 冻结：禁止再改已 applied SQL；新变更只加 forward 文件。
  2. allowlist 命中时可选 **一次性 UPDATE checksum→file**（或强制部署必须 sync），让 DB 收敛到当前树。
  3. restore 与 sync 对称：restore 也只允许 allowlist 对或「schema head 未前进」时执行。
  4. CI 计算 tree checksum 集合，检测「已发布 tag 中出现的 migration 内容是否变化」。
- **交叉关注**: M10 deploy/checksum；全环境升级手册

### [P1] 唯一索引加固路径不一致：部分有重复 precheck + invalid index 重试，部分只有注释
- **位置**:
  - 有完整保护: `120_*_notx` / `138_*_notx` / `195a_*_notx` / `199a_*_notx`（`prepareNonTransactionalMigration` + `findDuplicate*`）
  - 缺口: `backend/migrations/191_add_user_affiliate_ledger_reverse_action_unique_notx.sql`（注释要求人工去重，**runner 无 precheck、无 invalid index drop**）
  - 事务内唯一索引: `207` `idx_ip_security_activity_user_ip`、`176` `idx_tls_fp_capture_samples_task_replay_transport_unique`（非 CONCURRENTLY）
- **相对上游**: 本地支付/发票路径已沉淀最佳实践，但后加的 affiliate reverse / TLS / IP 表未统一套用
- **问题**:
  1. `191` reverse unique：存量若已有双 clawback 行，启动 `CREATE UNIQUE INDEX CONCURRENTLY` 直接失败；失败留下 `INVALID` 索引时，因 **未注册** `prepareInvalidIndexRetry`，`IF NOT EXISTS` 会 **跳过重建**，约束长期不生效 → 可再次双扣。
  2. `176` 在事务内删重 + 建 unique：大表锁；与 notx 规范不一致。
  3. `195`/`199` 数据整形与 notx 索引拆分是正确模式，但 `157` 注释仍声称「order_id 无唯一约束、互斥下放到应用层」（`157_create_invoices_and_invoice_orders.sql:69-71`），与 `195`/`invoice_order.go` 双层约束 **文档漂移**，合并评审易误判。
- **影响**: 计费/分销资金路径上唯一性 **假满足**；启动失败不可自愈。
- **证据**:
  ```go
  // prepareNonTransactionalMigration switch 仅白名单:
  // 120, 138, 151, 153, 211, 195a, 199a —— 无 191 reverse
  ```
  ```sql
  -- 191: 仅注释 PREREQUISITE，无 DO $$ 重复检测
  CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS
    idx_user_affiliate_ledger_skill_run_reverse_unique ...
  ```
- **建议**:
  1. 为 `191` 增加 `findDuplicateCreatorEarningReverse` + invalid index 清理，与 138 同级。
  2. 规范：凡 `CREATE UNIQUE INDEX CONCURRENTLY` 必须 (a) 前置重复查询 fail-closed (b) 注册 prepare (c) IF NOT EXISTS。
  3. 修正 `157` 注释或加 pointer 到 `195`，避免契约误读。
- **交叉关注**: M04 发票/支付；M07 skill 分销；M06 无

### [P1] Ent schema / domain 与真实 DB 漂移：关键列与运营表不在 Ent 模型中
- **位置**:
  - DB 有、`ent/schema/user.go` **无**: `ips`、`ip_history_saturated`（migration `207`；读写走 `backend/internal/repository/ip_security_repo.go` 裸 SQL）
  - DB 有、Ent 无实体: `openai_oauth_capacity_hourly`、`openai_oauth_quota_snapshots/periods`（214/215）、`balance_cache_outbox`（201）、`ip_security_*`（207）、`usage_user_daily_cost`（193）、`ai_skill_balance_ledger`（209）、`ops_sticky_schedule_events`（192）等
  - `usage_logs.video_duration_seconds`（172）与 `video_seconds`（193/213）双列并存；Ent 仅建模 `video_seconds`（`ent/schema/usage_log.go:161-165`）
  - `domain/constants.go` 平台列表含 `grok`/`kiro`；部分 quota CHECK 靠迁移 156/181 扩展，与 Ent validate 未必同源
- **相对上游**: 本地大量「SQL-first 运营表」；Ent 只覆盖部分 OLTP 实体
- **问题**:
  1. **双写真相源分裂**：IP 历史字段 Ent 不可见 → `go generate ./ent` / 未来 `ent migrate` 可能 **丢掉列** 或生成不一致 diff。
  2. 集成测试 `migrations_schema_integration_test.go` 校验了 scheduler_outbox claim 列等，但 **未** 断言 `users.ips`、capacity 表、invoice `is_active`、钱包 `decimal(22,10)` 等新关键面。
  3. `video_duration_seconds` 废弃列残留：旧节点若仍写入 duration 列、新节点读 seconds，依赖 213 已跑完；滚动升级窗口存在读空。
- **影响**: 生成代码/二次迁移误删列；跨版本混部读到 NULL 视频秒数 → 计费展示/审计偏差。
- **证据**: `user.go` Fields 止于 `token_version`，无 ips；`ip_security_repo.go:24,59` 直接 SQL 读写；`213` 注释称 duration 可 drop 但仍未 drop。
- **建议**:
  1. 要么把 `ips`/`ip_history_saturated` 补进 Ent schema（Optional），要么在 `AGENTS.md`/迁移 README 明确「非 Ent 表清单」+ 禁止 ent 自动 DDL。
  2. 扩展 integration 对齐测试覆盖 195/198/201/207/214/215 关键列与索引。
  3. 视频列：双写窗口结束后用独立迁移 drop `video_duration_seconds`；混部期间 service 读路径 `COALESCE(video_seconds, video_duration_seconds)`。
- **交叉关注**: M05 IP；M04 视频计费；M06 capacity；M07 skill ledger

### [P1] `request_type` 枚举语义曾碰撞，数据修复靠启发式 UPDATE，存在误分类残留
- **位置**:
  - 服务枚举: `backend/internal/service/usage_log.go:16-26`（4=cyber 历史钉死，8=image，6=legacy cyber alias，7=video）
  - 迁移链: `148`→`185`→`191_restore`→`191a_validate`（及 checksum allowlist 条目）
  - 测试: `usage_log_cyber_test.go` 覆盖读路径消歧，不覆盖 DB 全量回填正确性
- **相对上游**: 本地 image/cyber/video 枚举演进比上游更曲折；checksum 规则证明文件被改过
- **问题**:
  1. `191` 将 `request_type=4` 且带 image 元数据的行改为 8，再把 `6→4`。若历史 cyber 行碰巧带有 image 字段噪声，会被 **改成 image**。
  2. 约束多次 `DROP/ADD ... NOT VALID` 后 `VALIDATE`：大表 validate 仍可能长时间 `ShareUpdateExclusiveLock`。
  3. 枚举真相在 service 包而非 domain/DB 枚举类型，统计 SQL 易写错魔法数。
- **影响**: 用量看板/风控（cyber）与图片用量统计交叉污染；属数据正确性高风险。
- **证据**: `191_restore_usage_request_type_cyber_value.sql:22-37` 启发式 WHERE；`RequestTypeCyberBlocked = 4` 注释「禁止复用」。
- **建议**: 抽样校验生产 `request_type IN (4,6,8)` 分布；必要时按 `inbound_endpoint`+时间窗二次 repair 迁移；把枚举表固化到 `domain` 并单测与 CHECK 约束生成同源。
- **交叉关注**: M04 用量统计；M03 协议桥 image 路径

### [P2] 前向-only 迁移 + 无 down：与 README「Write reversible migrations」矛盾，回滚只能靠备份
- **位置**: `backend/migrations/README.md:12-14,61,141-144`；runner 无 down 解析；`ApplyMigrations` 只 forward
- **相对上游**: 同模式；本地文档自相矛盾更明显
- **问题**: README 同时写「deploy 不回滚 schema / 必须快照」与「Always provide working Down migration / make migrate-down」——runner **不会**执行 goose Down 段，且明确警告不要把 Down 写进同一文件。
- **影响**: 运维按文档尝试 down 会得到虚假安全感；真实回滚依赖逻辑备份。
- **建议**: 删除/改写「reversible / migrate-down」过时段落；统一为 forward-only + 备份门禁。
- **交叉关注**: M10

### [P2] 事务内 `CREATE INDEX IF NOT EXISTS` 广泛用于新表可接受，但混入热表时绕过 notx 护栏
- **位置**: `207`/`214`/`215`/`192`/`193`/`201`/`209` 等；校验仅禁止 **字面 CONCURRENTLY** 出现在非 notx（`validateMigrationExecutionMode`）
- **相对上游**: 本地
- **问题**: 护栏不阻止「事务内对已有大表建普通索引」。新空表（capacity_hourly）可接受；`usage_logs` 不可接受（见 P0）。
- **影响**: 规范靠人工，回归成本高。
- **建议**: 静态分析：若 SQL 触及 `usage_logs|payment_orders|accounts` 且含 `CREATE INDEX` 非 CONCURRENTLY → CI fail。
- **交叉关注**: M10 CI

### [P2] `ensureAtlasBaselineAligned` 在空 atlas 表时插入「filename 字典序最大」伪 HEAD，双轨迁移工具危险
- **位置**: `migrations_runner.go:555-593,608-627`
- **相对上游**: 兼容残留
- **问题**: 若有人启用 Atlas 对同一 DB 做 diff apply，会认为已在「最新」版本而 **跳过** 真实未同步的中间语义；与 legacy `schema_migrations` 双轨。
- **影响**: 工具链误用导致漏迁移或重复 DDL。
- **建议**: 文档标明 Atlas 表仅占位；或写入 **全部** legacy 版本而非单一 HEAD；CI 禁止混用。
- **交叉关注**: M10

### [P3] domain 常量与前端/迁移注释多处「保持一致」靠人工
- **位置**: `backend/internal/domain/constants.go` Antigravity/Bedrock 默认 mapping；注释要求与前端 `useModelWhitelist.ts` 同步
- **相对上游**: 分叉热点
- **问题**: 无生成器/单测锁定两侧 mapping 一致（虽有 `constants_test.go` 但未必锁前端）。
- **影响**: 模型路由静默偏离。
- **建议**: 契约测试读前端导出或共享 JSON。
- **交叉关注**: M02、M09

### [P3] wire / Ent 生成物与 schema 手改流程依赖开发者纪律
- **位置**: `backend/cmd/server/wire.go`、`wire_gen.go`；`go generate ./ent`
- **相对上游**: 同
- **问题**: 本模块未见 wire 手改异常；但 schema 增字段后若漏 generate，编译期才能发现，迁移与 Ent 不同步时运行期才爆。
- **建议**: CI `go generate` + diff clean。
- **交叉关注**: M10

## 与上游合并风险
- **冲突热点文件**:
  - `backend/migrations/*` 同号文件海（101/142/152/162/176/191/195…）
  - `backend/internal/repository/migrations_runner.go`（allowlist、prepare 白名单、notx 校验）
  - `backend/cmd/sync_checksums/**`
  - `backend/ent/schema/{user,usage_log,group,invoice_order,payment_order,account}.go`
  - `backend/internal/domain/constants.go`
- **语义漂移点**:
  - `request_type` 数值语义（4 cyber vs 曾用作 image）
  - `invoice_orders` 互斥：157 注释「仅应用层」→ 195 DB partial unique
  - `video_duration_seconds` vs `video_seconds`
  - OpenAI capacity：本地 `214/215` 表结构 vs 上游可能另起编号/列名
  - 钱包精度 `decimal(22,10)` 本地 198
- **合并策略建议**: 以 **迁移文件名不可变 + 前缀唯一 CI** 为硬门禁；冲突时双方 remap 到新号并写 data repair，禁止改写已 applied 内容；allowlist 变更需双端评审。

## 测试与验证缺口
- **已有**:
  - `migrations_runner_notx_test.go`：CONCURRENTLY/notx 语法护栏、120/138 precheck mock
  - `migrations_schema_integration_test.go`：幂等再 apply、auth/payment/channel monitor/AI center 列对齐
  - `auth_identity_payment_migrations_regression_test.go`：关键迁移 SQL 形态快照
  - `sync_checksums/main_test.go`：allowlist 外拒绝改写
  - `usage_log_cyber_test.go`：枚举读路径
- **缺口**:
  - 无测试锁定「数字前缀全局唯一」
  - 无对 `207/213/191/198` 热表迁移的「禁止事务内大索引/全表 UPDATE」静态门禁
  - integration 未覆盖 `users.ips`、`invoice_orders.is_active`、capacity 表、`balance_cache_outbox`、钱包精度
  - `191` reverse unique 无 duplicate precheck 单测
  - 无「allowlist 命中后 DB checksum 是否收敛」行为测试
  - 无与上游 migration 前缀集合的 diff 作业（合并预检）

## 模块结论
- **整体风险评级: High**
  - 迁移执行器本身（advisory lock、checksum 严格默认、notx 护栏、部分 unique precheck）质量不低；
  - 风险集中在：**编号体系崩坏下的合并碰撞**、**启动期热表 DDL/DML**、**checksum 运维与 schema 回滚不对称**、**Ent/SQL 双真相源**。
- **是否建议合入上游 / 继续分叉 / 先修再合**:
  - **先修再合（迁移编号与热表路径）**；合入上游前必须完成前缀冲突表 + 热表迁移拆批。
  - AI/发票/IP/capacity 等表结构可继续分叉，但 runner/allowlist 变更应尽量回馈上游以免双轨。
- **Top 3 必须处理项**:
  1. **迁移前缀唯一性治理 + 与上游 merge 的 remap SOP**（消除同号异义静默双执行）。
  2. **热表启动迁移治理**：`207`/`213`/`191`/`198` 改为 notx/离线批处理，避免升级锁死。
  3. **唯一索引与 checksum 运维收敛**：补齐 `191` precheck/invalid-retry；allowlist 命中回写 checksum；禁止危险 restore 三方错位（协同 M10）。
