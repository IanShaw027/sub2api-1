# personal-dev → personal-main 分批迁移清单

> 生成时间：2026-08-13。对比基准：`git diff upstream/main...personal-dev`（三点 diff，merge-base = upstream v0.1.173 `48eb3766`）。
> 配套流程见 `PERSONAL_FORK_WORKFLOW.md` 第五节。旧历史存档：tag `archive/personal-dev-20260813`。
> 由 4 个并发分析代理产出（后端服务层 / 后端数据接入层 / 前端 / 基础设施），本文档为汇总。

## 总览

| 领域 | 文件数 | 行数变化 |
|---|---|---|
| 后端 service 层 | 980 | +272,910 / -78,450 |
| 后端 ent（schema 34 个源文件，其余为生成代码） | 222 | +180,525 / -28,001 |
| 后端 handler | 294 | +58,747 / -14,852 |
| 后端 repository | 208 | +37,292 / -8,817 |
| 后端其他（server/pkg/domain/config/cmd/migrations 等） | ~420 | 约 +42,000 |
| 前端 | 609 | +138,943 / -23,401 |
| 工具/部署/CI/文档 | 154 | +22,977 / -546 |

## 全局批次规划（跨领域汇总）

原则：数据库结构最先；被所有模块依赖的骨架和基础设施其次；平台网关按依赖链推进；管理端和 SaaS 业务收尾。
每批完成后必须：`go build ./... && go test -tags=unit ./...`（前端批次加 `pnpm run typecheck && pnpm run test:run`），
并 `go generate ./cmd/server` 校验 wire DI 图。

| 批次 | 内容 | 来源模块 |
|---|---|---|
| **0. 数据库 schema** | `ent/schema/` 34 个文件 + migrations 重新编号（⚠️ 见风险1）+ `go generate ./ent` | ent schema 变更 |
| **1. 基础设施** | CI workflows、安全扫描工具（secret_scan/govuln/pnpm audit）、`deploy.sh`+测试、构建溯源、`.gitignore`、SECURITY.md、pnpm-workspace.yaml overrides | 建议迁移清单 |
| **2. 后端基础骨架** | Account-Core、Auth-ApiKey（含 API Key 安全存储重构）、Setting-Config、Scheduling-TempUnsched、Billing-Usage 底座、Security-Fingerprint、Content-Moderation、脊柱文件最小骨架、usage/billing/调度缓存 repo | 服务层模块 1-5,7,8；数据层同名条目 |
| **3. AI Studio / Skill（零耦合大模块）** | ai_studio/ai_skill/media 全套 service+handler+repo+skillrunner 沙箱 + 前端 Studio/Skills 页面 + skill-runner 部署示例 | 服务层模块 22；前端批次 3/4 |
| **4. Claude/Anthropic 网关核心** | gateway_service.go 及转发/遥测/anti-ban 全套 | 服务层模块 9 |
| **5. OpenAI 系列（拆 3 个子批）** | 5a OAuth 调度与容量 → 5b Gateway Core（骨架→Codex 转换→compat 层）→ 5c WS 转发 → 5d Images | 服务层模块 10-13 |
| **6. 各平台接入（可并行）** | Kiro/Bedrock（含 pkg/kiro）、Gemini、Antigravity、Grok（依赖批次5）+ 对应前端账号组件 | 服务层模块 14-17；前端批次 1 剩余 |
| **7. 管理端** | Admin-Management（admin_service、渠道监控 v2、设置 handler）+ 前端 SettingsView 分区迁移 | 服务层模块 20；前端批次 8 |
| **8. SaaS 业务** | Payment/Invoice、Subscription-Redeem、Ticket、Affiliate、认证增强（钉钉/微信/TOTP） | 服务层模块 18-19；前端批次 5/6/7/9 |
| **9. 运维观测收尾** | Ops-Monitoring、Capacity-Forecast 已在批2后即可插入、IP 安全、备份 | 服务层模块 6,21 |

## ⚠️ 关键风险（迁移前必读）

1. **migrations 编号冲突**：personal-dev 与 upstream 在 128–220 号区间独立编号，**79 个编号真实碰撞**（同号不同内容，如 144/157/172/181/185/191/192/193/199/206/217-220）。不能直接复制；personal-dev 独有的迁移逻辑需从 personal-main 当前最大编号之后**重新编号**，并生成编号映射表存档。221–263 号为 personal-dev 纯新增，可按序续接。
2. **前端 i18n 三套并存**：personal-dev 有 monolith 的 `locales/{en,zh}.ts`+`.json`（约 3.5 万行）与上游模块化目录并存，靠运行时三路合并。**迁移前先做 i18n 去重归位**，把 monolith key 拆回模块化文件，恢复上游单一 loader，否则每个批次都会重复撞冲突。
3. **脊柱文件不能一次性搬运**：`openai_gateway_service.go`(195 提交/24 模块)、`gateway_service.go`(81/24)、`setting_service.go`、`admin_service.go`、`account.go`、`wire.go`、`domain_constants.go`、`ratelimit_service.go` 等被几乎所有模块共同修改。策略：early 批次先迁"能编译的最小骨架"，之后每个模块批次对它们做**增量 patch**，不整体覆盖。
4. **group.go 假删除警报**：personal-dev 的 ent schema 里没有 `profit_control_*` 三字段（改走 repository 原始 SQL），但上游有。合并时**必须保留上游这三个字段声明**，否则破坏上游利润控制调度。
5. **上游文件删除/重组**：personal-dev 删除并重组了上游多个文件（gateway_forward.go 系、setting_*.go 系、admin_{account,group,proxy,user}.go、antigravity_gateway_*.go 系、openai_ws_forwarder_*.go 系）。迁移这些模块时是"结构性替换"而非叠加，注意上游后续更新会落在旧文件名上，同步时需人工映射。

---

# 附录 A：后端服务层（backend/internal/service）

- 总体规模：980 个文件，约 +272,910 / -78,450 行（新增 388、修改 550、删除上游 42）
- 提交粒度粗，脊柱文件在几乎每个功能提交中被顺带修改，**只能按文件语义聚类迁移，不能按提交切分**。

### 1. 账号核心实体与账号连通性测试（Account-Core）
- **功能**：Account/AccountService 核心实体、凭证持久化与脱敏、并发槽位、账号连通性测试、User 服务。所有平台账号的公共数据模型层。
- **文件**：新增 `account_model_defaults.go`、`account_response_rewrite_rule*.go` 等 11 个；修改 `account.go`、`account_service.go`、`account_credentials_*.go`、`account_test_service*.go`、`concurrency_service.go`、`user*.go`、`vertex_service_account.go` 等 24 个。
- **规模**：35 个文件，约 +9,373 / -2,801 行
- **耦合**：全仓库耦合度最高之一；每个平台模块都往里加字段/分支。先落地骨架，各平台批次追加。
- **建议批次**：early

### 2. 认证 / API Key / Token 缓存基础设施（Auth-ApiKey）
- **功能**：登录鉴权（邮箱绑定/OAuth 自动绑定）、API Key 鉴权缓存与失效、OAuth token 刷新/缓存/池健康检查、幂等请求。
- **文件**：新增 `api_key_secret_protection*.go`、`oauth_session_persistence*.go` 等 7 个；修改 `api_key*.go`、`auth_*.go`、`oauth_*.go`、`token_*.go` 等 37 个。
- **规模**：44 个文件，约 +3,866 / -703 行
- **耦合**：被所有平台网关间接依赖，自身内聚。
- **建议批次**：early

### 3. 反封号与 TLS 指纹伪装（Security-Fingerprint）
- **功能**：TLS 指纹路由/画像导入、指纹归一化、IP 安全检测、anti-ban 平台策略、账号级 TLS 指纹策略绑定。
- **文件**：新增 27 个（`tls_fingerprint_*.go`、`fingerprint_normalizer*.go`、`ip_security*.go`、`anti_ban_platforms.go`、`kiro_tls_profile.go`、`openai_tls_fingerprint_router*.go` 等）；修改 5 个。
- **规模**：32 个文件，约 +9,480 / -117 行（几乎纯新增）
- **耦合**：被各平台请求构造调用，本体独立性很好。
- **建议批次**：early

### 4. 系统设置服务（Setting-Config）
- **功能**：全局配置读写中心，承载几乎所有功能开关。personal-dev 把上游的 setting_features/oauth/parse/public/update 等文件合并进单一 `setting_service.go`。
- **文件**：新增 14 个单测；修改 11 个；**删除上游 6 个**（被合并）。
- **规模**：31 个文件，约 +10,098 / -6,738 行
- **耦合**：第二大脊柱文件（60 提交/23 模块），与 `domain_constants.go` 强绑定。
- **建议批次**：early（接受持续增量）

### 5. 账号调度、限流熔断与临时下线（Scheduling-TempUnsched）
- **功能**：调度阈值评估、模型级限流、模型 fallback、组容量、429 冷却、可恢复失败策略、错误透传、临时下线、定时可用性测试、跨平台模型路由、调度快照。
- **文件**：新增 23 个；修改 43 个（`ratelimit_service.go`、`temp_unsched.go`、`scheduler_snapshot_service.go`、`error_passthrough_*.go` 等）。
- **规模**：66 个文件，约 +6,288 / -1,884 行
- **耦合**：所有平台网关的失败降级路径都经过这里；与 Account-Core、Billing-Usage 强耦合。
- **建议批次**：early

### 6. 容量预测服务（Capacity-Forecast）
- **功能**：账号容量预测框架 + 各平台容量 Provider，后台展示"账号还能撑多久"。
- **文件**：14 个文件全部新增（`capacity_forecast_*.go`、`capacity_provider_*.go`），零修改上游。
- **规模**：约 +3,296 行
- **耦合**：仅依赖各平台账号字段，独立性极佳。
- **建议批次**：early（紧跟 Account-Core 之后随时可插入）

### 7. 计费与用量统计（Billing-Usage）
- **功能**：请求计费、计费缓存、账号用量统计、定价与模型定价解析、视频/图片计费、发票、用户平台配额、用量清理归档、批量入库 worker pool。
- **文件**：新增 14 个（`invoice_service*.go`、`usage_model_resolution*.go`、`balance_cache_outbox.go` 等）；修改 35 个（`billing_service.go`、`pricing_service.go`、`usage_*.go` 等）。
- **规模**：49 个文件，约 +9,010 / -1,544 行
- **耦合**：所有网关请求闭环的必经环节。
- **建议批次**：early（基础能力先行，各平台批次追加计费分支）

### 8. 内容审核（Content-Moderation）
- **功能**：请求/响应内容审核（关键词、邮件通知、规避检测、输入过滤）。
- **文件**：9 个文件，约 +5,603 / -579 行
- **耦合**：被 gateway_service 调用，自身内聚。
- **建议批次**：early

### 9. Anthropic/Claude 网关核心（Claude-Anthropic-Gateway）
- **功能**：网关主干——gateway_service.go 本体、协议桥接转发、账号选择、计费头、websearch、工具改写、debug 时间线、Kiro/Bedrock 走 Claude 协议兼容层、遥测转发脱敏、anti-ban 门控、thinking 协议。
- **文件**：新增 30 个；修改上游 37 个；**删除上游 10 个**（gateway_forward.go、gateway_bedrock.go、gateway_scheduling.go 等被拆分重组）。
- **规模**：77 个文件，约 +20,926 / -11,467 行
- **耦合**：全仓库耦合度最高（81 提交/24 模块），所有平台模块直接调用或被它调用。
- **建议批次**：early-middle（在所有平台专属网关之前落地骨架）

### 10. OpenAI 账号调度与 OAuth 容量管理（OpenAI-OAuth-Scheduling）
- **功能**：OpenAI(Codex) 专属账号调度器（粘性会话/权重）、OAuth token provider、OAuth 容量时间序列、APIKey 健康熔断、CRS 账号同步、shadow 路由。
- **文件**：新增 11 个；修改 20 个。
- **规模**：31 个文件，约 +11,269 / -3,927 行
- **耦合**：`openai_account_scheduler.go` 是 OpenAI 系二号脊柱（61 提交/23 模块）。
- **建议批次**：middle（OpenAI 系列第一棒）

### 11. OpenAI Codex/Responses 网关核心（OpenAI-Gateway-Core）
- **功能**：service 层最大模块——Responses/Chat Completions/Codex 协议互转、模型映射别名、会话续接、compact、Claude-compat、embeddings、audio、live、agent identity、粘性会话、profit control。
- **文件**：新增 75 个；修改上游 84 个；**删除上游 11 个**。
- **规模**：170 个文件，约 +44,438 / -16,452 行
- **耦合**：`openai_gateway_service.go` 195 提交涉及全部 24 模块；与 Claude 网关双向耦合。
- **建议批次**：middle，**自身拆 3 个子批**：骨架+模型映射 → Codex 转换/续接 → compact/compat 兼容层

### 12. OpenAI 实时 WebSocket 转发（OpenAI-WS）
- **功能**：Codex WS 长连接转发、WS v2 passthrough、delta shadow 对比、会话抢占、预检、HTTP↔WS 桥接。几乎重写级改造。
- **文件**：新增 19 个；修改 25 个；删除上游 5 个。
- **规模**：49 个文件，约 +36,358 / -8,102 行
- **耦合**：对外仅少数入口函数，内部自洽。
- **建议批次**：middle（紧随 OpenAI-Gateway-Core，单独成批）

### 13. OpenAI 图像生成与计费（OpenAI-Images）
- **功能**：图像生成、批量图片任务、图片计费倍率、输出核算、Codex 图像桥接。
- **规模**：27 个文件，约 +6,643 / -1,597 行
- **建议批次**：middle（紧随 OpenAI-Gateway-Core）

### 14. Grok（xAI）集成（Grok）
- **功能**：Grok OAuth/quota、专属限流、音频/媒体、借 OpenAI Chat Completions 协议壳接入 Grok 上游。
- **文件**：新增 6 个；修改 31 个；删除 1 个。
- **规模**：38 个文件，约 +10,623 / -6,894 行
- **耦合**：**强依赖 OpenAI-Gateway-Core**（网关桥接寄生在 OpenAI 兼容层）。
- **建议批次**：middle-late（必须在 OpenAI 系列之后）

### 15. Kiro / Bedrock 网关（Kiro-Bedrock）
- **功能**：Kiro 完整网关栈——OAuth、token、gateway/usage service、invokeMCP 沙箱、错误解析、fake cache、TLS profile、Bedrock 适配。
- **文件**：新增 26 个；修改上游仅 2 个。
- **规模**：28 个文件，约 +20,479 / -22 行（几乎纯新增，零侵入）
- **耦合**：集成点集中在 gateway_service、account、admin_service、setting_service、TLS 指纹。
- **建议批次**：middle（本体可整体搬运，等骨架就位后接入集成点）

### 16. Gemini 网关（Gemini）
- **规模**：19 个文件，约 +5,382 / -979 行；相对独立，改动都在 Gemini 专属文件。
- **建议批次**：middle（可与 Kiro/Antigravity 并行）

### 17. Antigravity 网关（Antigravity）
- **文件**：新增 3 个；修改 19 个；**删除上游 5 个**（gateway_claude/gemini/retry/streaming/upstream 被合并重写）。
- **规模**：27 个文件，约 +5,532 / -4,344 行（重写比例高）
- **建议批次**：middle

### 18. 支付服务（Payment）
- **功能**：支付订单全生命周期、支付配置、webhook、统计。与网关解耦。
- **规模**：33 个文件，约 +8,051 / -1,633 行
- **建议批次**：late（独立、低风险）

### 19. 订阅与兑换码（Subscription-Redeem）
- **规模**：10 个文件，约 +971 / -132 行（最小模块）
- **建议批次**：late（扫尾）

### 20. 管理端服务（Admin-Management）
- **功能**：AdminService 巨型服务（账号/分组/用户/代理批量操作）、渠道监控 v2、渠道广场、工单、通知邮件、公告、审计留存、备份、TOTP。上游 admin_account/group/proxy/user 被合并进单一 admin_service.go。
- **文件**：新增 32 个；修改 66 个；删除上游 4 个。
- **规模**：102 个文件，约 +20,332 / -8,061 行（文件数最多）
- **耦合**：所有模块的汇总点（57 提交/24 模块）。
- **建议批次**：late（渠道监控可提前；admin_service 本体等对应平台迁移完再补管理入口）

### 21. 运维监控与系统日志（Ops-Monitoring）
- **规模**：36 个文件，约 +2,548 / -227 行；纯可观测性，不阻塞核心链路。
- **建议批次**：late

### 22. AI Studio / AI Skill 创作工作台（AI-Studio-Skill）
- **功能**：完全独立新子系统——AI 创作工作室（多模态/存储计费/扣费告警）、AI Skill 技能市场（运行时网关/结算/审核）、AI Center、资产生命周期、媒体接入、对象存储、视频生产管线（ffmpeg）。
- **文件**：45 个文件**全部新增**，零修改上游。
- **规模**：约 +20,794 行
- **耦合**：仅 wire.go、admin_service、billing 有浅层集成点。
- **建议批次**：early（零耦合、风险极低，可最先整体搬运）

### 23. 依赖注入装配（Wire-DI）
- `wire.go`/`wire_test.go`，约 +1,043 / -194 行。**不单独排期，每批迁移顺带追加注册**，批后跑 `go generate ./cmd/server`。

### 共享文件缠绕矩阵（3 个以上模块共同修改）

| 文件 | 非合并提交数 | 涉及模块数 |
|---|---|---|
| `openai_gateway_service.go` | 195 | 24 |
| `gateway_service.go` | 81 | 24 |
| `openai_account_scheduler.go` | 61 | 23 |
| `setting_service.go` | 60 | 23 |
| `admin_service.go` | 57 | 24 |
| `wire.go` | 54 | 24 |
| `account.go` | 48 | 23 |
| `domain_constants.go` | 41 | 23 |
| `ratelimit_service.go` | 32 | 23 |
| `account_service.go` | 23 | 23 |
| `billing_service.go` | 22 | 20 |
| `user_service.go` | 20 | 23 |
| `group.go` | 13 | 16 |

迁移策略：early 批次先迁到能编译的最小骨架；每个模块批次对这些文件做增量 patch；wire 每批结束跑 generate 校验。

---

# 附录 B：后端数据层与接入层（ent/handler/repository/基础设施）

### ent schema 变更

**新增实体（19 个）**：

| 实体 | 用途 |
|---|---|
| `Invoice` / `InvoiceOrder` | 发票系统：合并开票、状态机、订单互斥占用 |
| `AIAsset` / `AIGenerationJob` | AI Studio 生成资产与任务队列 |
| `AIPromptTemplate` / `AIPromptTemplateVersion` | 提示词模板及版本 |
| `AISession` / `AISessionMessage` / `AIAuditLog` | AI Studio 会话与审计 |
| `AISkill` / `AISkillVersion` / `AISkillInstall` / `AISkillLike` / `AISkillReview` / `AISkillRun` / `AISkillSettlement` | 技能市场全套 |
| `TLSFingerprintRouter` / `AccountTLSFingerprintPolicy` / `AccountTLSFingerprintBinding` | TLS 指纹路由与账号级策略 |

**修改的上游实体（14 个）要点**：
- `api_key.go`：API Key 存储安全模型重构（lookup_hash + AES-256-GCM 密文 + 前缀），影响面广，早期批次。
- `group.go`：图片/视频计费字段、AI 存储计费、展示字段、退款折算。**⚠️ 见风险4（profit_control 假删除）**。
- `tls_fingerprint_profile.go`：新增 20+ 字段（H2 指纹、头模板、TLS 扩展回放）。
- `usage_log.go`：视频计费重构、WS 复用统计、tls_fingerprint 记录。
- `user.go`：余额精度 decimal(22,10)、token_version、Skill 级联边。
- `payment_order.go` / `payment_provider_instance.go` / `payment_audit_log.go`：发票关联、履约租约、防重索引。
- `proxy.go`：backup_proxy 边改名 fallback_sources（影响生成代码调用点）。
- `user_platform_quota.go` / `channel_monitor*.go`：新增 kiro 平台枚举（与 Kiro 批次同批）。
- `batch_image_job.go`：幂等索引改 (user_id, api_key_id, idempotency_key)。

**migrations**：128–220 区间 79 个编号碰撞（见风险1）；221–263 共 43 个文件纯新增可续接；重排作为批次 0 的独立子任务并存档编号映射表。

### 功能模块聚类（handler/repository/pkg）

| 功能 | 主要文件 | 规模 | 建议批次 |
|---|---|---|---|
| ent schema + migrations 重排 | `ent/schema/*`、migrations 重编号 | +1,929 / +5,583 | **批次 0** |
| API Key 安全存储重构 | api_key handler/mapper/repo | ~670+ | 早期 |
| Kiro/Bedrock 接入 | kiro handler + `pkg/kiro/`（22 文件 7,360 行：converter/document/token_counter 等） | ~8,500 | 与 Kiro service 同批 |
| TLS 指纹反检测 | 指纹 handler/repo + `pkg/tlsfingerprint/`(2,653) + H1/H2 replay(~2,750) | ~8,800 | 中期 |
| Grok 增强 | grok handler + `pkg/xai/`(757) | ~1,400 | 与 Grok service 同批 |
| AI Studio/Skill/Studio Production | `handler/ai_*.go` 20+ 文件、skill repo(5,854)、skillrunner Docker 沙箱(1,868) | ~22,000 | AI 大批次 |
| 发票/支付增强 | payment/invoice handler+repo | ~3,300 | 中期 |
| 工单系统 | ticket handler+repo | ~2,800 | 独立小批次 |
| 媒体存储 | media handler+repo | ~1,400 | 早中期（AI Asset、发票文件依赖） |
| 备份系统 | backup handler+repo | ~600 | 独立小批次 |
| 渠道监控 v2 | channel_monitor handler+repo | ~1,000 | 中期 |
| OpenAI/Codex 网关 handler | `handler/openai_*.go` 39 个文件 | ~10,577 | 与 service 子批对齐 |
| 管理端账号/OAuth | `admin/account_*.go`、各平台 oauth handler | ~5,326 | 早中期 |
| 管理端设置 | `admin/setting_handler*.go` | ~7,619 | 中后期 |
| 前台 OAuth 登录 | `auth_*.go`（钉钉/微信/oidc/linuxdo） | ~3,836 | 独立低风险 |
| 代理池 | proxy handler+repo | ~1,200 | 中期 |
| 运维观测 | ops/capacity/ip_security handler+repo | ~4,000 | 中后期 |
| 用量计费底座 | usage_billing/usage_log/scheduler_cache/outbox repo | ~12,000 | **早期** |
| 用户平台配额/分组 | user_platform_quota/group/user repo | ~2,800 | 早期 |

### 装配文件（每批都要碰）

`cmd/server/wire.go`+`wire_gen.go`、`handler/wire.go`(+342)、`repository/wire.go`(+1,248)、`server/router.go`、`server/routes/{common,admin,gateway}.go`、`server/routes/{ai,media}.go`（新）、`handler/handler.go`、`handler/dto/{mappers,types,settings}.go`(+954/-335)、`domain/constants.go`（平台常量，新增平台必碰）。

### 新增 Go 依赖

`tiktoken-go v0.1.8`（token 计数）、`rsc.io/pdf v0.1.1`（Kiro PDF）、`regexp2`（间接）、`x/text`/`protobuf` 提升为直接依赖、`x/image` 升级。常规 `go mod tidy` 即可。

---

# 附录 C：前端（frontend）

- 总体规模：609 个文件（新增 266 / 修改 342 / 删除 1），约 +138,943 / -23,401 行

### 功能模块

| 功能 | 代表文件 | 规模 | 对应后端 | 建议批次 |
|---|---|---|---|---|
| 账号池增强（TLS指纹/多平台OAuth/容量） | EditAccountModal(+2,368/-1,208)、KiroAuthorizationFlow、TLSFingerprint* 组件、各平台 oauth api/composable | 150+ 文件，~+28,000 | Kiro/Grok/Gemini/Antigravity/OpenAI 容量/TLS 指纹 | **前端批次1（先行）** |
| AI 创作工作台（Studio） | `views/studio/*Workbench.vue`、AIChat/Gallery/PromptLibrary、治理页、`stores/aiStudio.ts` | 90+ 文件，~+12,000 | AI-Studio-Skill | 批次3 |
| 技能中心（Skills） | Skill{Market,Detail,Editor,...}View、`components/skills/*`、`stores/skillsCenter.ts` | 60+ 文件，~+10,500 | AI Skill | 批次4 |
| 工单系统 | Ticket*View、`components/tickets/*` | ~25 文件，~+4,700 | Ticket | 批次5 |
| 发票/订单增强 | UserInvoices*、AdminInvoiceApplications、payment 组件改造 | ~25 文件，~+5,000 | Invoice/Payment | 批次6 |
| 联盟返利 | affiliate api/types/视图 | ~8 文件，~+500 | Affiliate | 批次7（可并入6） |
| 渠道监控v2/Ops 加固 | channel-monitor-v2、ops 面板组件与测试 | ~45 文件，~+5,500（上游已有骨架，做定制 diff 而非搬迁） | 渠道监控/Ops | 批次8 |
| 认证增强 | 钉钉/微信回调、TOTP 组件、验证码 | ~20 文件，~+2,000 | Auth | 批次9 |
| 通用组件/UI | common 组件 30 个小改、导航角标系统 | ~40 文件，~+2,500 | 无 | 批次1（随基础设施） |
| 备份/IP安全 | BackupView step-up、ipSecurity api | ~8 文件，~+1,100 | Backup/风控 | 批次10 |

### 公共缠绕文件

- **i18n（重大风险，见风险2）**：monolith `locales/{en,zh}.ts/.json`（+35,286 行）与模块化目录并存，`i18n/index.ts` 三路合并。迁移前先去重归位、删 monolith、恢复上游 loader。
- `router/index.ts`(+1,860)：所有新页面路由，按批次增量插入。
- `types/index.ts`(+626)：迁移时按功能拆分独立类型文件。
- `views/admin/SettingsView.vue`(+7,335/-4,936，近乎重写)：**单独立项，逐配置分区迁移，禁止整体粘贴**。
- `AppSidebar/AppHeader`、`useNavigationBadges`+`navigation/simpleMode`（批次1 落地）、`api/client`、`stores/{app,auth}`。

### 依赖变化

package.json 与上游完全一致；新增 `pnpm-workspace.yaml`（安全 overrides：js-cookie 3.0.7、form-data>=4.0.6、postcss>=8.5.18、allowBuilds esbuild/vue-demi）。直接补 overrides 后 `pnpm install` 重新生成锁文件即可。

---

# 附录 D：工具与基础设施（tools/deploy/docs/CI/根级）

- 总体：154 个文件，+22,977 / -546 行（116 个纯新增）

### 建议迁移

| 路径 | 用途 | 批次 |
|---|---|---|
| `.github/workflows/backend-ci.yml` | pnpm 构建校验、go mod tidy 漂移检测、embed 构建校验、docker-image job | CI（早期） |
| `.github/workflows/release.yml` | RELEASE_REF/TAG 统一、shell 注入修复、build_metadata 集成 | CI |
| `.github/workflows/security-scan.yml` + `audit-exceptions.yml` + `govuln-exceptions.yml`（新） | govulncheck 固定版本+豁免校验、secret scan | 安全扫描 |
| `tools/check_govuln_exceptions.py`（新 115）、`check_pnpm_audit_exceptions.py`（改）、`secret_scan.py`（新 237）、`test_security_scan_tools.py`（新 227） | 安全扫描工具链 | 安全扫描 |
| `deploy.sh`（新 628）+ `tools/test_deploy_script.py`（新 649） | 生产部署主脚本（部署锁/备份回滚/sha256），**两文件同批缺一不可** | 部署（较早） |
| `deploy/build_metadata.sh`（新 98）+ Dockerfile*/goreleaser ldflags | 构建溯源（⚠️ 需 backend main 包变量支持，跨模块） | 构建溯源 |
| `deploy/install.sh`、`apple-container.sh`、`docker-compose*.yml`、`build_image.sh` | 绑定地址收紧 127.0.0.1、checksum 强制、REDIS_PASSWORD 必填（**MEDIA_* 段等 Media 迁移后再加**） | 部署安全默认值 |
| `deploy/Caddyfile`（新 111）+ `sub2api.service`（新 27） | 反代模板 + systemd 加固单元（Caddyfile 末尾 media 段按需裁剪） | 部署模板 |
| `Makefile` | test-frontend 全量化、secret-scan target（与前端批次协调） | 构建脚本 |
| `SECURITY.md`（新 89）、`.gitignore`(+24) | 通用 | 低风险早迁 |

### 建议放弃（一次性产物）

- `.review/upstream-cross-2026-07-16/*`、`.review/upstream-full-2026-07-19/*`：绑定历史 SHA 的审查快照（**但 CONSOLIDATED.md 里记录的 bug 清单在迁移对应模块时值得人工核对**）。
- 根级 `AUDIT_REPORT_*`、`CODE_REVIEW_REPORT_*`、`REVIEW_FINDINGS_VALIDATION_*`（~2,400 行）：历史审计报告。
- `docs/upstream-merge/2026-*.md`：旧分支的合并操作记录，已失去上下文。
- `tools/openai_*_apifox.json`、`openai_images_test.py`+测试、`openai_oauth_responses_probe.py`+测试、`crs_capture_proxy.js`、`seed_openai_temp_unsched_rules.sql`：一次性联调/调试产物。
- `deploy/skill-runner/runtime/*/input/request.json`：示例占位数据。

### 待定（需要用户决策）

- `docs/superpowers/{plans,specs}/*.md`（10 篇 4,602 行设计稿）：跟随各自后端功能批次迁移。
- `docs/UPSTREAM_MERGE_GUIDE.md` + 语义合并方法论文档：为旧策略写的，是否改写为模块化迁移版。
- `CHANGELOG.md`：版本号体系不连续，保留历史还是从空白开始。
- `README*` 改动：九成是赞助商 churn，**不整体迁移**，手动补实质修改（Go 徽章、SECURITY 链接等）。