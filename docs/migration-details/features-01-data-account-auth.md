# 功能点清单：数据库结构与迁移 / 账号用户凭证底座 / 登录鉴权与 API Key 安全

> 对比基准：旧内容 = `upstream/main`；新内容（含全部个人定制）= merge-tree `5806f923bdf47619b25fed4a69220b16e181164c`。
> 覆盖范围：`cap_files.json` 中 `id` 为 0、1、2 的三个能力，共约 255 个文件，逐文件核对 diff 内容后按“用户/管理员能感知到的具体行为”整理。

---

## 能力：数据库结构与迁移（对应 cap id 0）

这批文件大多是给"其他能力"（AI 创作工作台、技能市场、支付发票、工单、TLS 指纹、渠道监控 v2 等）建表，本身不构成账号/用户/鉴权侧的可感知功能。下面先讲清楚**与账号、用户、登录鉴权直接相关**的表结构变化（这些是本次三个能力真正依赖的底座），其余大批量迁移在文末归类列出。

### 功能点 1：API Key 数据库存储从"明文"改为"哈希 + 可选加密"
- 涉及文件：`backend/ent/schema/api_key.go`、`backend/migrations/216_protect_user_api_keys.sql`、`backend/migrations/162_deleted_api_key_audit.sql`、`backend/migrations/232_auth_cache_invalidation_outbox.sql`、`backend/migrations/243_fix_protected_api_key_invalidation.sql`
- 描述：`api_keys` 表新增 `lookup_hash`（SHA-256 查找指纹，唯一索引）、`key_ciphertext`（AES-256-GCM 加密后的明文，可选）、`key_prefix`（用于列表搜索/展示的非敏感前缀）三个字段；旧的 `key` 列不再存明文，改存查找哈希以兼容历史唯一约束；原先对 `key` 做全文模糊搜索的 trigram 索引被删除，改用 `key_prefix` 的 B-tree 索引做前缀搜索。同时新增 `deleted_api_key_audits` 表在删除审计里也只存哈希，不再保留明文。这是账号安全的底层改造，直接支撑下面"登录鉴权与 API Key 安全"能力里的 Reveal（查看明文）与鉴权缓存哈希化。
- 另新增 `auth_cache_invalidation_outbox` 表：把"编辑/删除 API Key 后要失效鉴权缓存"这件事做成数据库事务性的可靠队列（outbox 模式），即使当时 Redis 不可用，失效事件也会被持久化并保证后续重试投递，不会出现"改了 Key 但缓存还没失效"的窗口。

### 功能点 2：用户表新增 `token_version`（强制下线）与 IP 多账号风控字段
- 涉及文件：`backend/ent/schema/user.go`、`backend/migrations/133_add_user_token_version.sql`、`backend/migrations/207_ip_multi_account_security.sql`
- 描述：`users` 表新增 `token_version BIGINT`列，配合登录鉴权改造实现"改密/管理员强制下线后，所有已发出的旧 access token 立即失效"（详见下面认证能力的对应功能点）。同时新增 `ips TEXT[]`（记录用户历史使用过的最多 256 个 IP）和 `ip_history_saturated` 字段，并建表 `ip_security_activity` 记录每次请求的 IP/UA/来源，用于短时间窗口内检测"同一 IP 被多个账号使用"的风控场景（迁移脚本对老用户按历史 `usage_logs` 做了一次性的 IP 回填，避免存量用户被误判为"新 IP"）。

### 功能点 3：新增 Passkey（无密码 / WebAuthn）登录凭据表
- 涉及文件：`backend/migrations/240_passkey_credentials.sql`
- 描述：新增 `passkey_user_handles` 与 `passkey_credentials` 两张表，为用户提供基于设备生物识别/安全密钥的无密码登录方式（凡是能在浏览器/系统里注册 WebAuthn 凭据的设备都可以绑定）。这是全新的登录方式底座（对应的 handler/前端在其他能力批次中，这里只确认数据表是本次新增）。

### 功能点 4：分组（Group）新增"前台展示名"与"用户可自选线路"
- 涉及文件：`backend/ent/schema/group.go`、`backend/migrations/140_add_group_display_name_and_user_selectable.sql`、`backend/migrations/169_align_group_display_name_length.sql`
- 描述：`groups` 表新增 `display_name`（前台展示名，为空则回退到内部 `name`）与 `user_selectable BOOLEAN DEFAULT true`（是否允许用户在前台自行选择这条线路）。这两个字段直接支撑下面"账号/用户/凭证底座"能力里的"前台线路自选"功能。

### 功能点 5：用户平台配额（`user_platform_quotas`）扩展支持 Kiro / Grok 平台
- 涉及文件：`backend/ent/schema/user_platform_quota.go`、`backend/migrations/156_user_platform_quotas_add_kiro.sql`、`backend/migrations/157_user_platform_quotas_add_grok.sql`、`backend/migrations/181_user_platform_quotas_add_grok.sql`
- 描述：`user_platform_quotas.platform` 的 CHECK 约束从只允许 `anthropic/openai/gemini/antigravity` 逐步扩展到含 `kiro`、`grok`，使得管理员可以对这两个新增平台单独设置用户级日/周/月限额（后台"用户平台配额"编辑弹窗新增 Kiro/Grok 两个 tab）。

### 功能点 6：迁移执行器与校验工具重构
- 涉及文件：`backend/internal/repository/migrations_runner.go`（291 行新增/71 行删除）、`backend/cmd/sync_checksums/main.go`（新文件，289 行）、`backend/migrations/README.md`
- 描述：迁移执行器本身做了较大重构（错误处理、幂等重试、并发迁移文件的 checksum 校验逻辑更严格），并新增 `sync_checksums` 命令行工具，用于在迁移文件内容确认无害变更后重新计算并同步已记录的 checksum（避免"迁移文件内容被规范化/加注释后，历史环境校验失败拒绝启动"）。这是纯运维/开发工具改进，管理员/用户不会直接感知，但保证了新增的上百个迁移文件能够安全滚动升级。

### 无新增可感知功能的文件（仅列文件名+一句说明）
以下文件均为迁移脚本或纯数据结构改动，服务于本次三个能力之外的其他子系统（AI 创作工作台、技能市场、支付/发票、工单、TLS 指纹反检测、渠道监控 v2、备份、内容审核等），或是针对上述已列出功能点的索引优化/约束校验补丁，不构成额外的账号/用户/鉴权可感知行为：

- `backend/ent/schema/ai_asset.go`、`ai_audit_log.go`、`ai_generation_job.go`、`ai_prompt_template.go`、`ai_prompt_template_version.go`、`ai_session.go`、`ai_session_message.go`、`ai_skill.go`、`ai_skill_install.go`、`ai_skill_like.go`、`ai_skill_review.go`、`ai_skill_run.go`、`ai_skill_settlement.go`、`ai_skill_version.go`：AI 创作工作台/技能市场的数据模型，属于其他能力批次。
- `backend/ent/schema/invoice.go`、`invoice_order.go`、`payment_audit_log.go`、`payment_order.go`、`payment_provider_instance.go`：发票与支付相关字段/表，属于支付能力批次。
- `backend/ent/schema/account_tls_fingerprint_binding.go`、`account_tls_fingerprint_policy.go`、`tls_fingerprint_profile.go`、`tls_fingerprint_router.go`：TLS 指纹反检测数据模型，属于反封号能力批次（账号侧的调用点在 `account.go` 里有涉及，见下文账号能力功能点）。
- `backend/ent/schema/channel_monitor.go`、`channel_monitor_request_template.go`：仅字段类型/注释微调，无行为变化。
- `backend/ent/schema/batch_image_job.go`、`payment_provider_instance.go`、`proxy.go`（微调字段）：小幅字段增补，服务于图片生成计费与代理到期策略，非本次三个能力的核心行为。
- `backend/ent/schema/usage_log.go`：新增计费维度字段（provider、tls 指纹、long-context 等），属于计费与用量统计能力。
- 迁移脚本中约 130 个文件（`backend/migrations/125_*` 至 `263_*` 里除功能点 1-5 提到的以外的全部文件，例如 `142_create_ai_skill_center.sql`、`144_create_ai_center_core.sql`、`157_create_invoices_and_invoice_orders.sql`、`220_create_backup_records.sql`、`245_channel_monitor_v2.sql`、`171_tls_fingerprint_capture_tasks.sql` 等）：均为上述其他能力模块建表/加索引/修约束，或是针对已有表的性能索引（`CREATE INDEX CONCURRENTLY`）、CHECK 约束校验（`VALIDATE CONSTRAINT`）收尾脚本，不产生新的账号/用户/鉴权行为。

---

## 能力：账号 / 用户 / 凭证底座（对应 cap id 1）

### 功能点 1：分组级实时并发展示（管理端可感知：分组容量按"实际当前并发"而非理论上限统计）
- 涉及文件：`backend/internal/repository/concurrency_cache.go`、`backend/internal/service/concurrency_service.go`
- 描述：新增账号并发槽位之外的**分组维度并发跟踪**（Redis key `concurrency:group:{groupID}`，独立的活跃索引 `concurrency:group:active_index`）。请求获取账号并发槽位的同时，如果该请求走的是某个具体分组，会同步在这个分组的槠位集合里登记一条记录并可原子读取 `GetGroupConcurrency` / `GetGroupConcurrencyBatch`。这使得后台"分组"页面可以展示"这个分组当前实际有多少个请求在处理中"这一实时数字，而不是只能看分组配置的并发上限（这正是本次任务描述里给的例子）。
- 同一批改动里还新增了**长连接心跳续约**：请求获取并发槽位后，服务会每隔一段时间（默认 5 分钟）自动"重新占用"同一个槽位以刷新 Redis TTL，避免长时间的流式/长连接请求因为超过槽位默认 TTL 而被后台清理误判为"已释放"，造成并发数在展示上和调度上出现"虚假空闲"。
- 释放槽位、清理过期槽位、启动清理等逻辑同步支持了账号+分组两段一起释放（`AcquireAccountSlotForGroup`/`ReleaseAccountSlotForGroup`），失败时有指数退避重试（最多 3 次）而不是失败一次就放弃。

### 功能点 2：用户 / 分组每分钟 RPM 的原子限流与实时展示
- 涉及文件：`backend/internal/repository/user_rpm_cache.go`
- 描述：新增 `TryIncrementUserAndGroupRPM` 原子操作——之前的实现是"先递增计数、再和上限比较"，在高并发下会出现瞬时超过限流阈值的竟态；新实现用一个 Lua 脚本把"读取当前计数 → 判断是否超限 → 递增"整体做成原子操作，超限直接拒绝且不产生错误递增，从而使 RPM 限流真正生效而不是"事后才发现超了"。同时把 key 结构改造为 Redis Cluster hash-tag 兼容格式（`rpm:ug:{uid:minute}:{gid}`），并对旧 key 格式做双写兼容，保证滚动升级期间新旧实例的计数不丢失、不错乱。`GetUserRPM`/`GetUserGroupRPM` 只读接口本身就是给"用户实时 RPM"展示用的数据源。

### 功能点 3：API Key 列表新增按"当前并发"排序，详情页展示实时并发数
- 涉及文件：`backend/internal/service/api_key_service.go`（cap2 文件，与 cap1 的并发能力共同构成本功能）、`backend/internal/service/concurrency_service.go`
- 描述：用户在"我的 API Key"页面现在可以按"当前并发数"对 Key 列表排序（`sort_by=current_concurrency`），点开某个 Key 的详情也会附带它当前的实时并发占用数（`GetByID` 现在会调用并发服务批量查询并回填 `CurrentConcurrency`），而不再是只能看到静态的限额配置。

### 功能点 4：前台"线路"（可选分组）自选功能
- 涉及文件：`backend/internal/handler/available_channel_handler.go`、`backend/internal/service/api_key_service.go`（`GetAvailableRouteGroups`，cap2 文件）、`backend/ent/schema/group.go`（cap0）
- 描述：新增 `GetAvailableRouteGroups`：只返回同时满足"分组处于活跃状态 + 管理员标记为 `user_selectable=true` + 当前用户至少有一个绑定该分组且未过期/未超额的 API Key"三个条件的分组列表，并把展示名统一回退到 `DisplayLabel()`（`display_name` 为空则显示 `name`）。这是"渠道状态/可用线路"页面给普通用户展示"我可以直接切换使用哪些线路"的数据来源，比原来的 `GetAvailableGroups`（仅按订阅/权限过滤、不检查是否已绑定 key）更贴近用户实际可用性。

### 功能点 5：账号级"响应改写规则"（管理员可对上游异常响应做自定义替换）
- 涉及文件：`backend/internal/service/account_response_rewrite_rule.go`（新文件）
- 描述：账号可以在 `credentials.response_rewrite_rules` 里配置一组规则：按上游返回的 HTTP 状态码和/或响应体关键词匹配（支持"任一命中"或"全部命中"两种模式），命中后把响应替换成管理员自定义的提示文案。这是后台账号编辑弹窗里"自定义错误码"能力（`CustomErrorCodesForm.vue`）的后端实现，让管理员可以把某些上游的技术性错误改写成对最终用户更友好的说明。

### 功能点 6：账号连通性测试大幅扩展为"多模态专项测试"
- 涉及文件：`backend/internal/service/account_test_service.go`（新增/改动约 2500 行）
- 描述：后台"测试账号连接"功能从原来基本只能测一次文本对话，扩展为按平台/能力专项测试：
  - Grok 账号新增测试文本对话、图片生成、视频生成（含轮询生成结果）、网页搜索（x_search）、TTS 语音合成、STT 语音识别、Realtime 实时语音会话等多条独立测试通路，并在测试过程中就地更新账号的额度/风控状态快照（例如探测到 Grok 消费限额、账号是否命中风控）。
  - OpenAI 账号新增区分 Chat Completions / Responses / Compact 三种协议路径的测试，以及两种不同图片生成端点（API 直连与网关代理）的独立测试，覆盖到 OAuth 图片生成场景。
  - Kiro 账号新增专用测试通路（`testKiroAccountConnection`），会读取 Kiro 运行时设置后走真实的凭据校验。
  - 这意味着管理员在账号池点击"测试"，现在能够针对不同平台的图片、视频、语音、搜索等具体能力分别验证是否可用，而不是只知道"文本对话通不通"。

### 功能点 7：Kiro 账号支持三种鉴权方式，且能自动识别/归一化不同来源的凭据字段
- 涉及文件：`backend/internal/service/account_service.go`
- 描述：新增账号级 Kiro 凭据校验与归一化逻辑，识别 `auth_method` 为 `idc`（AWS IAM Identity Center / BuilderID，企业 SSO）、`external_idp`（外部身份提供商，专门适配了 Microsoft Entra ID/AzureAD 的 issuer_url 自动推导 token endpoint）、`social`（社交登录）三种鉴权方式，并且能把粘贴进来的、大小写风格不同的字段（如 `accessToken` vs `access_token`）自动归一化成统一格式，同时校验 refresh_token 是否明显被截断（例如结尾是"..."）。`idc` 模式下强制要求填写 `client_id`/`client_secret`，`external_idp` 模式下强制要求 `client_id` 和可推导的 token endpoint，缺一会在保存时直接报错，避免管理员录入不完整的凭据后账号在运行时才报错。

### 功能点 8：账号更新时的凭据合并语义修正 + 分组"仅允许 OAuth 账号"限制
- 涉及文件：`backend/internal/service/account_service.go`、`backend/internal/service/account_credentials_redact.go`
- 描述：编辑账号凭据时，之前是"整体替换"（`SanitizeStoredCredentials` 覆盖式），现在改成显式合并补丁（`mergeAccountCredentialsForAccountUpdate`）：只更新表单里真正填写/勾选清空的字段，敏感字段（access_token、refresh_token、client_secret 等）如果表单没有显式给出新值，不会被空值误删。同时敏感凭据清单新增了 `sso_token`、`client_secret`（Kiro IdC/BuilderID 的 OAuth 客户端密钥）、`agent_private_key`/`agent_runtime_id`/`task_id`（OpenAI Agent Identity 相关）等键，确保这些新增的凭据类型也会被正确脱敏、不出现在前端响应里。另外新增校验：如果某个分组被管理员标记为"仅允许 OAuth 账号"（`RequireOAuthOnly`），尝试把一个 API Key 类型的账号加入该分组会被后端直接拒绝并提示分组名称。

### 功能点 9：平台级"默认账号模型配置"模板
- 涉及文件：`backend/internal/service/account_model_defaults.go`（新文件，707 行）
- 描述：管理员可以为每个平台预先配置一套默认参数模板（模型白名单、模型映射/紧凑模式映射、临时下线规则、自定义错误码等），新建该平台的账号时自动套用这套默认值，不需要每次新建账号都重新手填一遍。对应后台"账号池"新建账号向导里的"使用平台默认配置"选项（`PlatformDefaultAccountModelConfigForm.vue`）。

### 功能点 10：管理员/用户安全防护：禁止误删最后一个管理员、违规用户自动禁用、有未完成任务的用户禁止删除
- 涉及文件：`backend/internal/repository/user_repo.go`
- 描述：
  - `UpdatePreservingLastAdmin`：把管理员降级为普通用户时，会在事务里锁定并统计当前所有管理员，如果目标用户是最后一个管理员则直接拒绝这次降级，避免平台被误操作成"没有任何管理员"。
  - `DisableUserForContentModeration`：内容审核命中违规规则时可以自动把用户状态改为禁用，但明确排除管理员角色，不会误禁后台管理员账号。
  - `EnsureUserCanDeleteBatchImageState`：删除用户前会检查该用户是否还有冻结余额（批量出图预扣款未结算）或未完成的批量出图任务，任一条件成立就拒绝删除，避免删除用户后产生资金/任务孤儿数据。

### 功能点 11：用户资料新增 GitHub / Google 第三方登录绑定展示、邀请返利明细账本、后台"维护模式"
- 涉及文件：`backend/internal/service/user_service.go`、`backend/internal/handler/user_handler.go`
- 描述：
  - 个人资料页的"第三方登录绑定"区域新增 GitHub、Google 两个身份提供商（连同已有的 LinuxDo/OIDC/微信/钉钉），并且身份摘要现在会带上头像 URL（之前该字段被强制隐藏不返回给前端）。
  - 新增 `GET /api/v1/user/aff/invitees/:id/ledger`：用户可以查看某一个被自己邀请的下线用户的返利流水明细（每笔金额、基准消费金额、返利比例、是否占用了"邀请名额"），而不再只能看到汇总数字。
  - 新增"后台模式"（`EnsureBackendModeAllowsUser` / `BackendModeUserGuard`）：管理员在系统设置里开启后，前台所有普通用户的自助操作（绑定/解绑第三方身份、改资料等）会被禁止，只允许管理员登录和操作——用于系统维护窗口期临时关闭用户自助入口。

### 功能点 12：用户头像支持对象存储上传（含失败自动清理）
- 涉及文件：`backend/internal/service/user_service.go`
- 描述：头像上传从只能存内联 base64（受限于 100KB）扩展为可以经由对象存储服务（`MediaService`）上传持久化文件，并新增失败时的清理回调（`AvatarCleanupFunc`），上传后续步骤失败会自动删除刚上传的媒体文件，避免存储空间里残留孤儿文件；同时头像解码新增对 WebP 格式的支持。

### 功能点 13：代理池管理增强：多级故障转移修复、批量测试/质量检测、编辑接口"清空字段"语义修复
- 涉及文件：`backend/internal/repository/proxy_repo.go`、`backend/internal/handler/admin/proxy_handler.go`、`backend/internal/handler/admin/proxy_data.go`、`backend/internal/repository/proxy_probe_service.go`、`backend/internal/repository/proxy_latency_cache.go`
- 描述：
  - 修复了代理到期自动切换的一个 bug：之前只有"从未被切换过"的账号（`proxy_fallback_origin_id IS NULL`）才会在代理到期时自动改投备用代理；如果账号已经从 A 切到过 B，B 再过期时账号不会被二次改投，会一直挂在已失效的代理上。现在改成只要账号当前绑定的代理过期就会被改投，并用 `COALESCE` 保留最初的 origin 记录，多级备用链（A→B→C）终于能正确接力。
  - 后台代理池新增"批量测试连通性"（按当前筛选条件，一次性并发测试所有匹配的代理，返回成功/失败汇总）和"批量质量检测"（返回健康/警告/被 Cloudflare 挑战/失败四档汇总）两个批量操作入口，不再需要逐个点开测试。
  - 修复"编辑代理"接口的一个数据丢失 bug：之前"清空过期时间/清空 fallback 模式/清空备用代理 ID"和"不修改该字段"在请求参数里是同一种表现（都是零值/缺省），导致这几个字段实际上无法被清空。现在通过读取原始请求 JSON 判断字段是否显式出现，正确区分"用户想清空"和"用户没填这个字段"。
  - 代理出口 IP 探测新增两个探测源（`ifconfig.me`、`icanhazip.com` 的纯文本 IP 响应），降低探测因为单一第三方服务不可用而失败的概率；代理质量检测结果缓存改为带 TTL（15 分钟自动过期）并做了增量合并写入，避免历史质量快照无限堆积或被后续写入整体覆盖丢失字段。

### 功能点 14：反 Cloudflare 拦截的渐进退避冷却
- 涉及文件：`backend/internal/guard/cloudflare_backoff.go`（新文件）
- 描述：新增一个通用的指数退避计算工具（默认起始冷却 10 秒，最多每次乘 3 倍，上限 120 秒），供账号/代理探测在遇到 Cloudflare 拦截时计算"下次再探测前要等多久"，避免短时间内反复触发同一账号/代理的人机验证挑战导致状态进一步恶化。

### 无新增可感知功能的文件（仅列文件名+一句说明）
- `backend/internal/repository/http_upstream.go`：TLS 指纹伪装传输层（HTTP/1.1、HTTP/2、CONNECT/SOCKS5 代理经路的底层实现），是反封号能力的内部支撑，不在本次三个能力的范围内单独构成新行为。
- `backend/internal/pkg/httpclient/pool.go`：新增经过 DNS 解析校验的 HTTP 客户端（防止 SSRF/DNS rebinding 攻击），用于代理连通性测试等场景的安全加固，属于内部安全实现，不改变用户可见行为。
- `backend/internal/pkg/reqclientpool/reqclient_pool.go` 与 `backend/internal/repository/req_client_pool.go`：把原来分散在 repository 层的"共享 HTTP 客户端池"逻辑抽成独立公共包，供 service 层复用，纯代码结构重构。
- `backend/internal/repository/group_repo.go`、`backend/ent/schema/group.go` 中除 `display_name`/`user_selectable` 之外的字段（图片/视频生成路由、按秒计价、退款倍率、搜索/语音定价等）：均属于计费与图片/视频生成能力范畴，不在本次三个能力文档展开。
- `backend/internal/repository/user_platform_quota_repo.go`、`backend/internal/repository/user_platform_quota_service_adapter.go`：把"周期性把 Redis 中的配额用量刷新进数据库"的批量写入从逐条改成多行 UPSERT，并修复了与管理员手动改配额之间的写入顺序竟态，是内部数据一致性优化，不产生新的管理界面行为。
- `backend/internal/repository/user_group_rate_repo.go`：批量设置用户级 RPM 覆盖值时增加了去重与"必须 >= 0"校验，属于健壮性修复。
- `backend/internal/repository/temp_unsched_counter_cache.go`：账号"临时下线规则"命中次数的 Redis 计数缓存实现细节调整（按规则指纹而非规则序号做 key，避免规则顺序调整后计数错位），支撑的是已有的临时下线规则功能（该功能主体在调度能力批次），本身不是新功能。
- `backend/internal/repository/scheduled_test_repo.go`：为定时测试计划新增了一个"禁用"方法，属于配套的小工具方法。
- `backend/internal/repository/simple_mode_default_groups.go`：简易模式下自动创建的默认分组列表里加入了 Kiro 平台，属于配置数据补充。
- `backend/internal/service/identity_service.go`：伪装的客户端指纹版本号字符串更新（如 Node.js 运行时版本号），以及一处 nil 账号防护，不改变功能行为。
- `backend/internal/service/vertex_service_account.go`：分布式锁的返回值签名从"是否成功"改成"lease token + 是否成功"，纯内部接口适配，无行为变化。
- `backend/internal/model/error_passthrough_rule.go`：平台常量从本地硬编码改成引用 `domain` 包的统一常量，避免多处定义漂移，无行为变化。
- `backend/internal/service/account_credentials_persistence.go`：新增一个内部小工具函数（浅拷贝 map），无行为变化。

---

## 能力：登录鉴权与 API Key 安全（对应 cap id 2）

### 功能点 1：API Key 明文改为哈希+可选加密存储，新增"查看明文"功能（需 TOTP 二次验证）
- 涉及文件：`backend/internal/service/api_key_secret_protection.go`（新文件）、`backend/internal/service/api_key.go`、`backend/internal/service/api_key_service.go`、`backend/internal/repository/api_key_repo.go`、`backend/internal/handler/api_key_handler.go`、`backend/internal/service/api_key_auth_cache_impl.go`、`backend/internal/handler/admin/apikey_handler.go`
- 描述：这是本次鉴权安全改造的核心。API Key 落库时会计算 `HashAPIKeyLookup`（SHA-256）作为查找指纹，可选再用 AES-256-GCM 加密出一份密文用于日后"查看明文"；数据库里不再直接存可用的明文密钥。
  - 用户侧新增 `Reveal` 接口（`GET .../api-keys/:id/reveal`）：只有 Key 的所有者才能查看明文，且如果该用户开启了 TOTP 两步验证、平台也开启了 TOTP，前端必须先验证一次动态验证码（通过 `X-TOTP-Code` 请求头，或作为兼容手段放在请求体里）才能拿到明文，返回时带上 `no-store` 等禁止缓存的响应头。
  - Key 列表/详情接口原来会直接把明文 Key 吐给前端，现在统一改成 `dto.APIKeyFromServiceMasked`（脱敏 DTO，只带前缀），只有"创建成功那一刻"和主动调用 `Reveal` 时才会返回一次完整明文。
  - 对存量的老版本明文 Key，系统会在被访问到时"顺手"自动迁移（补齐 `lookup_hash`/`key_prefix`/可选密文），管理员也可以跑一次性的批量迁移（`MigrateAPIKeySecrets`），迁移过程中如果发现某条记录的哈希/密文和查找指纹不匹配（数据被篡改或损坏），会跳过并记日志而不是中断整个迁移。
  - 删除 Key 的审计记录（`deleted_api_key_audits`）也从存明文改成存哈希。
  - Key 的搜索从原来"模糊匹配明文"改成"按名称模糊匹配 + 按前缀模糊匹配"，鉴权查询从"精确匹配明文列"改成"按哈希查、同时兼容尚未迁移的历史明文行"。

### 功能点 2：Refresh Token 原子轮转 + 精确区分"并发合法重试"与"真正被重放攻击"
- 涉及文件：`backend/internal/service/auth_service.go`、`backend/internal/service/refresh_token_cache.go`
- 描述：原来的刷新流程是"读取旧 token → 删除旧 token → 生成新 token"三步分离，在并发请求下会出现"两个请求同时用同一个 refresh token 刷新，一个成功一个报错"，而报错的那次和真正的重放攻击（token 被盗用后攻击者尝试复用）在原实现里是同一种错误提示，用户体验和安全排查都很模糊。现在改成：
  - `ConsumeRefreshToken` 把"读取+删除+写入已使用标记"合并成一次原子 Redis 操作，只有一个并发请求能真正消费掉某个 token。
  - 没抢到的请求会去查"已使用标记"上记录的消费时间：如果在很短的宽限期内（5 秒），判定为"并发重试"，只是提示客户端重试，不会牵连撤销整个 token 家族；如果超出宽限期还有人来用这个已经用过的 token，判定为"重放攻击"，会撤销整个 token 家族（相当于强制这条登录会话链路的所有设备重新登录）。
  - 用户可感知的差异：正常情况下双开页面/网络抖动重试不会再把自己的登录会话踢掉，而真正的 token 泄露重放会被更彻底地处理（一撞就清空整条会话链）。

### 功能点 3：新增 `token_version`，改密/强制下线后已发出的 access token 立即失效
- 涉及文件：`backend/internal/service/auth_service.go`、`backend/internal/service/totp_service.go`、`backend/ent/schema/user.go`（cap0）
- 描述：以前"退出所有设备"主要依赖撤销 refresh token，已经签发出去、还没过期的 access token 理论上还能继续用一段时间。现在给用户表加了真正的数据库列 `token_version`，改密码、管理员强制下线都会让它自增，而 access/refresh token 的校验都要求携带的版本号必须等于当前数据库里的版本号，版本不匹配立即拒绝——也就是说改密码这一刻起，所有已经登录的设备上的旧 token 立刻失效，不需要等它自然过期。TOTP 二步验证的临时登录会话（`TotpLoginSession`）现在也带上了 `TokenVersion`，防止在两步验证等待期间密码被改也不影响这个安全判断。

### 功能点 4：密码强度要求从最少 6 位提升到最少 8 位，多处流程统一校验
- 涉及文件：`backend/internal/service/auth_service.go`、`backend/internal/service/auth_email_binding.go`、`backend/internal/service/auth_oauth_email_flow.go`、`backend/internal/handler/user_handler.go`
- 描述：注册、改密码、忘记密码重置、邮箱首次绑定密码、OAuth 登录后完成邮箱注册这几条路径，原来密码最小长度要求不统一（有的 6 位、有的干脆没校验），现在统一收敛到一个 `validateNewPassword` 校验（最少 8 位），并统一错误码 `PASSWORD_TOO_SHORT`。用户改密码接口（`ChangePasswordRequest`）的字段校验规则也从 `min=6` 改成 `min=8`。

### 功能点 5：忘记密码重置流程改为原子"占用-完成/归还"三段式，避免同一验证码被并发重复消费
- 涉及文件：`backend/internal/service/auth_service.go`
- 描述：原来的重置密码是"校验验证码 → 消费验证码 → 改密码"，如果改密码这一步中途失败，验证码已经被消费掉了、用户还得重新申请一次。现在改成先做完所有校验和耗时的密码哈希计算，再原子"占用"一次性验证码（`ClaimPasswordResetToken`），改密成功后才真正"完成"消费（`CompletePasswordResetToken`），如果中途任何一步失败，会把这次占用"归还"（`RestorePasswordResetToken`），验证码依然可以重新使用，不会平白无故作废。

### 功能点 6：Claude OAuth 授权会话支持多实例部署（Redis 存储）+ CSRF state 校验
- 涉及文件：`backend/internal/service/oauth_service.go`、`backend/internal/pkg/oauth/oauth.go`、`backend/internal/pkg/redissession/store.go`、`backend/internal/service/oauth_session_persistence.go`（新文件）
- 描述：之前给 Claude 账号做 OAuth 授权时，"授权发起"和"授权回调"这两步的会话状态只存在发起请求那台后端实例的内存里，如果部署了多台后端实例做负载均衡，回调请求被负载均衡到另一台机器就会报"session not found"。现在会话状态可以选配 Redis 存储（`NewRedisSessionStore`），多实例之间共享；同时授权码交换阶段新增了 `state` 参数的**常量时间比较校验**（防止 CSRF/时序攻击），并且会话被标记为"一次性消费"（`TryConsumeSession`，基于 Redis `SET NX` 实现跨实例互斥），避免同一份授权码被重复兑换。

### 功能点 7：注册/首次绑定赠送余额与并发写入可审计流水，前端提示"奖励已发放"
- 涉及文件：`backend/internal/service/auth_service.go`、`backend/internal/service/auth_email_oauth_auto.go`、`backend/internal/service/auth_oauth_email_flow.go`、`backend/internal/service/auth_oauth_first_bind.go`
- 描述：新用户注册/OAuth 首次登录会按配置自动赠送默认余额和并发额度，之前这笔赠送只体现在用户当前余额数字上，事后无法追溯"这笔钱是不是注册赠送的"。现在每次赠送都会额外写一条兑换码风格的历史流水（`recordSignupGrantHistory`/`recordFirstBindGrantHistory`，类型标记为管理员余额/并发调整，备注注明来源和渠道），方便后续审计对账；同时如果命中了邀请返利的注册奖励，会把奖励金额加到本次返回结果里并生成一条运行时提示消息（"通过邀请链接注册奖励 X.XX 余额已发放"），前端登录/注册成功页可以直接展示这条提示，而不是让用户自己发现余额多了却不知道为什么。

### 功能点 8：OAuth Token 刷新分布式锁健壮性提升 + 新增"管理员配额探测"专用刷新路径
- 涉及文件：`backend/internal/service/oauth_refresh_api.go`、`backend/internal/service/token_refresh_service.go`、`backend/internal/service/refresh_policy.go`
- 描述：多实例部署下对同一账号并发刷新 token 时抢分布式锁的逻辑改成"带超时的轮询重试"（默认最多等 300ms，每 25ms 探测一次），抢锁失败或 Redis 报错时会尝试直接从数据库/缓存里"捡漏"看是否已经有别的实例刚刚刷新成功（避免所有实例都在傻等或都去刷新造成上游限流）；分布式锁的释放也改成基于"租约令牌"而不是简单删除 key，避免释放了别的实例持有的锁。另外新增了一条"管理员配额探测"专用路径：管理员在后台点击查看 Grok 等平台账号的当前配额时，即使这个账号当前处于禁用/异常状态，也能刷新它的 token 来获取最新配额（普通业务请求路径不允许对异常账号刷新）。Kiro 平台的刷新策略（`KiroProviderRefreshPolicy`）也在这批改动里被正式纳入统一的刷新框架和后台自动刷新注册表。

### 功能点 9：鉴权中间件去重、清理不一致的余额判断逻辑
- 涉及文件：`backend/internal/server/middleware/api_key_auth.go`、`backend/internal/server/middleware/api_key_auth_google.go`
- 描述：之前在"订阅模式关闭或未注入订阅服务"分支里，中间件会额外做一次"余额是否低于配置的预留阈值"检查并 403，这个阈值原本只是给账单缓存预检用的保守下限，被误用成了鉴权硬门槛，会导致某些已配置该阈值的存量部署升级后，余额 0~阈值之间的用户在所有端点被静默拒绝。这批改动把这段有歧义的判断整体删除，统一收敛为"只在余额 <=0 时拒绝"这一个更明确的语义，Google 兼容路径下的同类判断也同步删除，行为更一致。

### 无新增可感知功能的文件（仅列文件名+一句说明）
- `backend/internal/service/idempotency.go`：一处错误提示文案微调（"executor is nil" → "executor is required"），无行为变化。
- `backend/internal/service/token_cache_invalidator.go`、`backend/internal/service/token_cache_key.go`：OAuth token 缓存失效逻辑新增了对 Kiro、Sora 平台的覆盖，并修正了 Gemini 场景下一处会误删有效缓存 key 的逻辑；属于对已有"账号 token 刷新后清理缓存"机制的平台覆盖补全，不产生新的用户可见入口。
- `backend/internal/service/oauth_session_persistence.go`：仅一个内部错误构造辅助函数，本身已作为功能点 6 的一部分描述。
