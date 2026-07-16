# Module 7: AI Studio / Skills / Session

## 范围摘要
- 对比基线: `BASE=da85cc7e` → `HEAD=eb64a5c1`，对照 `UPSTREAM=b960ec19`（`upstream/main`）
- 本模块变更规模: **本地新增/深度扩展** AI Studio 全栈（上游 `main` 基本无对等 skill/session 市场能力）
- 路径焦点:
  - `backend/internal/handler/ai_skill_handler*.go`、`ai_handler.go`、`ai_gateway_bridge.go`、`handler/skillkit/**`、`handler/admin/ai_skill_handler.go`
  - `backend/internal/service/ai_skill*.go`、`ai_center*.go`、`ai_studio.go`
  - `backend/internal/integration/skillrunner/**`
  - `backend/ent/schema/ai_*`（只读）、`deploy/skill-runner/**`（只读参考）
- 相对上游的主要能力差异:
  - Skill 市场: 创建/版本/审核/安装/发布/运行/结算/收益
  - Script skill 沙箱执行器（Docker CLI + zip bundle inspect）
  - Prompt chat/image skill 经 OpenAI 网关转发并做 token 归因
  - AI Center: session/message、prompt template、generation job、gallery/asset
  - 双层领域模型: domain 表模型 + service DTO，经 `repository/wire.go` adapter 映射

## 关键路径图

```
[User JWT]
  → AIHandler (skills / sessions / gallery)
    → skillkit.Module
        DomainService / SkillService / VersionService / ReviewService / RunService / SettlementService / Queries
    → AISkillRunService.Prepare/Execute
        1) 鉴权版本(approved) + use 模式强制 api_key_id
        2) CreateRun
        3) CreateIntent(settlement awaiting)
        4) RuntimeGateway.Execute
             ├ prompt_chat/image → OpenAIGateway Forward* + RecordUsage(BillingAPIKey)
             └ script → skillrunner Inspect → ReviewGate → Dispatch(Docker)
        5) ConfirmIntent → SettleIntent(买家扣款 + 创作者分成)
        失败 → FailIntentForDispatch(不可 replay 计费)
```

Script 沙箱:

```
zip archive → BundleInspector(路径/大小/symlink 限制)
  → DockerPlanner(ro root, network=none, cap-drop ALL, no-new-privileges, uid 65532)
  → DockerCLIExecutor(`docker run`)
  → /sandbox/output/response.json
```

## 发现清单

### [P0] Test 模式可绕过 API Key 消耗上游 token（平台白嫖）
- **位置**:
  - `backend/internal/service/ai_skill_run_service.go:86-90`（仅 use 强制 api_key）
  - `backend/internal/service/ai_skill_run_service.go:420-430`（仅 use 强制 BillingAPIKey）
  - `backend/internal/service/ai_skill_runtime_gateway.go:327-330`（`apiKey == nil` 直接 skip RecordUsage）
  - `backend/internal/handler/ai_skill_handler.go:621-626`（公开 `TestSkill` 入口）
- **相对上游**: 本地新增
- **问题**: `AISkillRunModeTest` 不要求 `trace.api_key_id`，且 OpenAI runtime 在无 BillingAPIKey 时**故意不记用量**。任意能读到已发布 skill 的用户（public/unlisted）可调用 `/test` 触发真实上游 chat/image 转发，平台账户被消耗且买家余额/配额不扣。
- **影响**: 资金/配额损失；可被批量刷接口形成持续成本攻击。这是 skill 与网关计费接缝的最高优先级漏洞。
- **证据**:
  - Prepare 仅对 `use` 校验 API key；test 走完整 `runtimeGateway.Execute`
  - `recordAISkillOpenAIUsage`: “No billing key means test-mode / free attribution path — skip intentionally”
  - 单测 `TestAISkillRunServicePrepare*` 以 test 模式执行且无 API key，视为合法路径
- **建议**:
  1. Test 模式也必须绑定调用者自有 API Key 并 `RecordUsage`；或
  2. Test 仅允许 skill 所有者，且走独立配额/日限额 + 强制计费；或
  3. Test 改为 dry-run（不调用上游），仅校验模板/参数
  4. 默认 fail-closed：无 BillingAPIKey 时禁止任何会触发上游的 skill 类型
- **交叉关注**: M01/M04 网关计费与 usage；M05 鉴权面

### [P0] 运行参数可注入 OpenAI tools/functions 等控制面字段
- **位置**: `backend/internal/service/ai_skill_runtime_gateway.go:637-669`（`applyAISkillChatRuntimeParameters`）
- **相对上游**: 本地新增
- **问题**: 将 run `parameters` 中的 `tools`、`tool_choice`、`functions`、`function_call`、`stop`、`max_tokens` 等原样写入上游 chat body。Skill 作者模板本意是变量替换，但调用方可借 parameters 扩展模型能力（工具调用、超大输出等），绕过 skill 定义的契约与审核范围。
- **影响**: 审核过的 prompt skill 可被变成任意 tool-using agent；放大 token 成本；若上游工具侧有效果则构成能力越权。
- **证据**:
  ```go
  for _, key := range []string{"stop", "tools", "tool_choice", "functions", "function_call"} {
      if value, ok := params[key]; ok && value != nil {
          body[key] = value
      }
  }
  ```
- **建议**: allowlist 仅保留温度类采样参数；禁止 tools/functions；`max_tokens` 设硬顶；schema 校验 variable_schema 之外的 key 丢弃。
- **交叉关注**: M01 OpenAI 协议；M03 apicompat tool ID

### [P1] Script TimeoutSeconds 无上限，可覆盖沙箱默认 10s 造成资源耗尽
- **位置**:
  - `backend/internal/service/ai_skill.go:833-835`（`<=0` 才默认 30，无 max）
  - `backend/internal/handler/ai_skill_handler.go:1040`（创建版本默认 30，可被 content 覆盖）
  - `backend/internal/service/ai_skill_runtime_gateway.go:432-445`（把 Timeout 传给 Docker）
  - `backend/internal/integration/skillrunner/types.go:134-141`（默认 10s 可被覆盖）
- **相对上游**: 本地新增
- **问题**: 作者可设极大 `timeout_seconds`（例如 86400），`DispatchRequest.Timeout` 与 post-plan 赋值均会抬高容器存活时间。配合并发 run，可占满 Docker/CPU/内存。
- **影响**: 多租户 DoS；skill-runner 宿主机可用性风险。
- **建议**: 服务端硬夹紧（如 1–30s，默认 10s）；admin 配置；拒绝超过 policy.Limits.Timeout 的值。
- **交叉关注**: M10 skill-runner 部署

### [P1] 媒体摄取失败时 best-effort 回退原始 URL，削弱 SSRF/内容边界
- **位置**:
  - `backend/internal/handler/ai_skill_handler.go:1188-1200`（失败返回 `{URL: source}`）
  - `backend/internal/handler/ai_skill_handler.go:1214-1216`（media 未启用直接透传 source）
  - 下游: `ai_skill_runtime_gateway.go:618-625` / `694-712` 将 attachment URL 塞进 OpenAI image_url
- **相对上游**: 本地新增
- **问题**: `IngestImageReference` 自身有较完整 SSRF 防护（`media_ingest.go` URL allowlist / private IP / redirect 校验），但 skill 路径在失败或 storage disabled 时**静默回退原始 URL**。结果：
  1. 未经验证的外链进入 run 记录与上游请求；
  2. 依赖上游拉取 image_url 时形成“二阶 SSRF/数据外带”；
  3. cover/attachment 存储与权限模型被绕过。
- **影响**: 安全边界不一致；审计/合规不可追踪的外部资源。
- **证据**: `storeSkillMediaReferenceWithIDBestEffort` 注释式行为：`return skillMediaReference{URL: source}, nil`
- **建议**: fail-closed——ingest 失败直接 400；storage disabled 时禁止远程 URL，仅允许已托管 media_id/data URL。
- **交叉关注**: M05 安全；M09 前端上传契约

### [P1] Docker 沙箱为“进程隔离轻量模型”，缺 seccomp/用户命名空间/镜像最小化
- **位置**:
  - `backend/internal/integration/skillrunner/docker.go:63-102`
  - `backend/internal/integration/skillrunner/executor.go:92-165`
  - `backend/internal/integration/skillrunner/types.go:151-163`（`python:3.11-slim` / `node:20-bookworm-slim`）
  - `deploy/skill-runner/README.md`（phase-1 合同）
- **相对上游**: 本地新增
- **问题（已有防护）**: network `none`、read-only rootfs、`cap-drop ALL`、`no-new-privileges`、非 root `65532`、pids/memory/cpu 限制、zip 防穿越/禁 symlink——方向正确。
- **问题（缺口）**:
  1. 无自定义 seccomp/AppArmor/gVisor；
  2. 官方 slim 镜像攻击面大（解释器标准库可本地做大量工作）；
  3. 依赖宿主机 Docker socket 权限，escape 即 host RCE；
  4. 作者可控 Environment 注入容器（`dispatch.go:104-108`）；
  5. scratch 输出目录 `chmod 0o733`（`executor.go:175-177`）在共享 host 上可被其他本地用户干扰；
  6. 用户侧创建 script skill 虽被 handler 拒绝（`AI_SKILL_SCRIPT_CREATION_UNSUPPORTED`），但运行路径与审核后执行已接通——一旦放开创建，审核质量成为唯一闸门。
- **影响**: 审核通过的恶意 script 在容器逃逸或 kernel bug 场景下危及网关主机；即使无逃逸也可做本地算力/磁盘打满。
- **建议**: 独立 runner 节点 + rootless/user-ns；seccomp profile；固定 digest 的最小化 runtime 镜像；禁止任意 ENV key；scratch 使用仅当前 uid 可写；生产默认关闭 script 直到 hardened runner 就绪。
- **交叉关注**: M10 部署与 skill-runner

### [P1] Domain/Service 双状态机映射脆弱：Disable 语义可能“看起来 approved”
- **位置**:
  - `backend/internal/repository/wire.go:821-854`（`domainReviewStatus` / `serviceVersionStatus`）
  - 特别是 `AISkillVersionStatusDisabled → domain.AISkillVersionReviewStatusApproved`
  - `CreateReview` 对 `AISkillReviewActionDisabled` 直接 `return nil`（`wire.go:290-291`）
  - `GetLatestApprovedVersionBySkillID` 靠 metadata `service_status_override=disabled` 过滤
- **相对上游**: 本地新增
- **问题**: service 层 Status（draft/submitted/approved/rejected/disabled）与 domain ReviewStatus（draft/pending/approved/rejected）不同构。Disable 在 domain 列上仍可能是 approved，仅靠 metadata override 区分。任何绕过 adapter、直接读 `review_status` 的路径（含 SQL queries、未来迁移）可能把 disabled 当可运行。
- **影响**: 治理“下架”不彻底 → 继续 run/结算；审计状态失真。
- **证据**: `domainReviewStatus(Disabled)=Approved`；`CreateSkillRun` 只查 `domain.CanUseAISkillVersion(review_status)`（`ai_skill_repo.go:1075-1077`），**不看** service override——若 run 创建走 domain repo 且 version 已被 disable 但 review_status 仍 approved，仍可通过。
- **建议**: 单一状态源；disable 应改 `review_status` 或独立 `lifecycle_status` 列并在 CreateSkillRun 强制检查；去掉仅 metadata 开关。
- **交叉关注**: M08 schema/migration

### [P2] 市场结算与 token 计费双轨：上游成功后结算可 defer，存在收入延迟/运维债
- **位置**: `backend/internal/service/ai_skill_run_service.go:233-264`、`ai_skill_settlement_service.go:90-201,213-403`
- **相对上游**: 本地新增
- **问题**: 设计上 token 用量先记、marketplace 结算独立状态机；Confirm/Settle 失败时 run 仍交付且 `SettlementDeferred=true`。intent 的 awaiting/failed 防 replay 写得较严谨，但缺少可见的自动 reconcile worker 说明。
- **影响**: 平台抽成/创作者分成延迟；需依赖 admin `ReplaySkillSettlement`；运维遗漏则漏收。
- **建议**: 后台 job 扫描 pending/confirmed 超时结算；指标告警；文档化 SLO。
- **交叉关注**: M04 计费支付

### [P2] Session 消息仅为持久化“笔记本”，无内容审核/无服务端生成约束
- **位置**: `backend/internal/service/ai_center_service.go:198-264`、`handler/ai_handler.go:302-335`
- **相对上游**: 本地新增
- **问题**: `SendMessage` 可直接写入任意 role/content，并可 `CreateAssistantReply` 由客户端指定 assistant 内容；无 moderation、无长度硬限、无与网关生成的绑定。
- **影响**: 若产品层把 session 当“AI 对话可信记录”，可被伪造；存储膨胀；XSS 风险取决于前端渲染（交 M09）。
- **建议**: 限制可写 role；assistant 仅服务端生成；接入 content_moderation；消息大小配额。
- **交叉关注**: M05 moderation；M09 前端

### [P2] ListSkillRuns 仅技能所有者可查，买家无法审计自己的付费 run
- **位置**: `backend/internal/handler/ai_skill_handler.go:544-547`（`GetSkillByUserAndID`）
- **相对上游**: 本地新增
- **问题**: 执行路径允许非 owner use 公共 skill，但 run 列表强制 owner。买家缺少自助对账入口（仅靠 settlement/admin）。
- **影响**: 计费可观测性差；客诉与对账成本上升（产品/合规债，非直接资金漏洞）。
- **建议**: 增加 `scope=mine` 按 `user_id=caller` 列 run；owner 看 skill 维度。
- **交叉关注**: M04、M09

### [P3] 架构边界：skillkit.Queries 直接 `*sql.DB`
- **位置**: `backend/internal/handler/skillkit/module.go`、`queries.go`
- **相对上游**: 本地新增
- **问题**: handler 层模块持有 SQL 查询，部分绕过 repository 端口；与项目 depguard“handler 不碰 DB”精神冲突（虽封装在 skillkit）。
- **影响**: 可维护性/合并成本；测试替身困难。
- **建议**: 迁入 repository 实现 service 端口。
- **交叉关注**: M08

### [P3] 正面设计（非缺陷，记录以防误报）
- use 模式 API Key 归属校验：`resolveBillingAPIKey` 拒绝外键（有单测）
- GroupID 不信任客户端 Trace：`resolveAISkillGroupID` 优先 BillingAPIKey.GroupID
- Script 审批 digest 绑定：`validateAISkillScriptApproval` fail-closed；审批时 `computeAISkillVersionApprovedDigest`
- ArchivePath 限根目录 + 禁 symlink（`resolveAISkillArchiveFilePath`）
- Zip 路径规范化/禁穿越/禁嵌套压缩与可执行后缀
- Settlement intent awaiting/failed 防“上游失败后 replay 扣款”
- 用户创建 script skill 暂时关闭（`validateUserSkillCreateType`）

## 与上游合并风险
- **冲突热点文件**:
  - 几乎全部为本地新增；与上游直接文本冲突概率低
  - 间接冲突点: `backend/internal/service/wire.go` DI 图、`handler/wire.go`、`OpenAIGatewayService` 被 skill runtime 耦合、migrations 序号（`143+` AI 表）
- **语义漂移点**:
  - domain vs service 的 skill version/run/settlement 状态枚举同名不同义（`approved`/`disabled`/`dispatched`）
  - domain `AISkillBillingModePerRequest` vs service `per_run`/`free`
  - run status: service `prepared/dispatched/succeeded` ↔ domain `queued/running/succeeded`
- **分叉成本**: 高——整模块是独立产品面；合入上游需整包迁移 schema + 网关接缝 + runner 运维，而非 cherry-pick

## 测试与验证缺口
- **有覆盖**: archive 路径穿越/symlink、digest 审批绑定、API key 归属、settlement intent fail/confirm/defer、部分 handler 可见性
- **缺覆盖**:
  1. Test 模式真实上游 + 无 API key 的计费回归（应定义为失败用例）
  2. parameters 注入 tools 的拒绝测试
  3. TimeoutSeconds 上界
  4. media best-effort 回退是否允许外链
  5. Disable 后 CreateSkillRun/Execute 全路径拒绝（含 domain review_status 仍为 approved）
  6. 并发 SettleIntent / Charge 幂等压测（`-count=30`）
  7. Session 消息越权/伪造 assistant 的 API 契约测试

## 模块结论
- **整体风险评级: Critical**
- **是否建议合入上游 / 继续分叉 / 先修再合**: **先修再合**（至少消除 P0：test 免 token 计费 + parameters 控制面注入）；script 沙箱在 hardened runner 前不建议默认对公网开放创建/执行
- **Top 3 必须处理项**:
  1. **关闭 test 模式免费上游消耗**（强制 BillingAPIKey 或禁止上游调用）
  2. **收紧 run parameters → OpenAI body 的字段 allowlist**（禁 tools/functions，夹紧 max_tokens）
  3. **统一 version disable 状态并在 CreateRun/Execute 强制生效；script timeout 硬顶 + 媒体 URL fail-closed**
