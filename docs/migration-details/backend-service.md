# 迁移明细 A：后端 Service 层（backend/internal/service）

> 基准：`git diff upstream/main...personal-dev --numstat`，共 980 个文件。
> 重要度：P0 核心（缺了核心工作流不可用或安全回退）/ P1 重要（明显功能或稳定性增强）/ P2 可选（便利、锦上添花）/ P3 建议放弃（一次性产物、被上游取代、过时）。
> 取舍栏默认 ☐ 待定，由用户填写。
> 测试文件与对应实现文件合并成一行时，说明栏注明“含 N 个测试文件”，文件栏已列出全部文件名，确保 980 个路径全部可见。
> “删除上游文件”指 personal-dev 删除/重组的上游文件（numstat 显示 0 新增），迁移这些模块时为结构性替换而非叠加。

## 1. 账号核心实体与账号连通性测试（Account-Core）（建议批次 early（批次2），整体重要度 P0）

Account/AccountService 核心实体、凭证持久化与脱敏、并发槽位、账号连通性测试、User 服务，是所有平台账号的公共数据模型层。全仓库耦合度最高之一，每个平台模块都往里加字段/分支。建议先落地能编译的最小骨架，再由各平台批次对脊柱文件做增量 patch，不整体覆盖。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `account.go` | +881/-194 | 账号核心实体与领域逻辑（脊柱文件，48 提交/23 模块）；账号数据模型的公共底座，各平台批次对其增量追加字段/分支。 | ☐ |
| P0 | `account_service.go` | +605/-15 | 账号业务服务（脊柱文件）；账号增删改查、凭证管理、平台分支的主入口。 | ☐ |
| P0 | `concurrency_service.go` `concurrency_service_test.go` | +369/-154 | 账号并发槽位控制服务；限制单账号并发请求数。（含 1 个测试文件） | ☐ |
| P0 | `user_service.go` `user_service_test.go` | +368/-178 | 用户服务（脊柱文件，20 提交/23 模块）；用户增删改、余额、平台配额级联。（含 1 个测试文件） | ☐ |
| P1 | `account_credentials_persistence.go` | +4/-0 | 账号凭证持久化 | ☐ |
| P1 | `account_credentials_redact.go` `account_credentials_redact_test.go` | +7/-1 | 账号凭证脱敏（含 1 个测试文件） | ☐ |
| P1 | `account_model_defaults.go` | +707/-0 | 账号级模型默认值与映射解析（纯新增），支撑各平台默认模型/别名。 | ☐ |
| P1 | `account_response_rewrite_rule.go` `account_response_rewrite_rule_test.go` | +133/-0 | 账号响应改写规则（含 1 个测试文件） | ☐ |
| P1 | `account_test_service.go` | +1332/-1497 | 账号连通性测试服务；对各平台账号发探测请求验证凭证可用性。 | ☐ |
| P1 | `identity_service.go` | +4/-1 | 身份服务 | ☐ |
| P1 | `user.go` | +22/-8 | 用户 | ☐ |
| P1 | `vertex_service_account.go` | +3/-2 | vertex服务账号 | ☐ |
| P2 | `account_base_url_test.go` | +8/-8 | 账号基础URL（测试） | ☐ |
| P2 | `account_service_test_credentials_test.go` | +424/-0 | 账号服务凭证（测试） | ☐ |
| P2 | `account_test_service_ops_test.go` | +256/-0 | 账号服务运维（测试） | ☐ |
| P2 | `account_wildcard_test.go` | +124/-0 | 账号通配（测试） | ☐ |
| P2 | `concurrency_slot_cleanup_test.go` | +24/-0 | 并发槽位清理（测试） | ☐ |
| P2 | `identity_service_order_test.go` | +12/-0 | 身份服务订单（测试） | ☐ |
| P2 | `user_service_media_avatar_test.go` | +536/-0 | 用户服务媒体头像（测试） | ☐ |
| P2 | `user_service_update_fields_test.go` | +2/-2 | 用户服务更新字段（测试） | ☐ |

## 2. 认证 / API Key / Token 缓存基础设施（Auth-ApiKey）（建议批次 early（批次2），整体重要度 P0）

登录鉴权（邮箱绑定/OAuth 自动绑定）、API Key 鉴权缓存与失效、通用 OAuth token 刷新/缓存/池健康、幂等请求、TOTP。被所有平台网关间接依赖，自身内聚。含 API Key 安全存储重构（lookup_hash + AES-256-GCM），属安全项。建议整体搬运。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `api_key_secret_protection.go` `api_key_secret_protection_test.go` | +241/-0 | API Key 密文保护（lookup_hash + AES-256-GCM + 前缀），安全存储核心。（含 1 个测试文件） | ☐ |
| P0 | `api_key_service.go` | +350/-181 | API Key 业务服务（脊柱）；Key 创建/校验/配额，含安全存储重构。 | ☐ |
| P0 | `auth_service.go` | +372/-96 | 登录鉴权服务；邮箱/OAuth 登录、注册、身份同步。 | ☐ |
| P1 | `api_key.go` | +15/-8 | APIKey | ☐ |
| P1 | `api_key_auth_cache.go` | +13/-7 | APIKey认证缓存 | ☐ |
| P1 | `api_key_auth_cache_impl.go` | +107/-68 | APIKey认证缓存impl | ☐ |
| P1 | `api_key_auth_cache_invalidate.go` | +48/-11 | APIKey认证缓存失效 | ☐ |
| P1 | `auth_email_binding.go` | +4/-2 | 认证邮件绑定 | ☐ |
| P1 | `auth_email_oauth_auto.go` `auth_email_oauth_auto_test.go` | +3/-3 | 认证邮件OAuthauto（含 1 个测试文件） | ☐ |
| P1 | `auth_oauth_email_flow.go` `auth_oauth_email_flow_test.go` | +27/-4 | 认证OAuth邮件flow（含 1 个测试文件） | ☐ |
| P1 | `auth_oauth_first_bind.go` | +1/-0 | 认证OAuth首绑定 | ☐ |
| P1 | `idempotency.go` `idempotency_test.go` | +1/-1 | 幂等（含 1 个测试文件） | ☐ |
| P1 | `oauth_refresh_api.go` `oauth_refresh_api_test.go` | +347/-96 | OAuth刷新API（含 1 个测试文件） | ☐ |
| P1 | `oauth_service.go` `oauth_service_test.go` | +108/-31 | OAuth服务（含 1 个测试文件） | ☐ |
| P1 | `oauth_session_persistence.go` `oauth_session_persistence_test.go` | +15/-0 | OAuth会话持久化（含 1 个测试文件） | ☐ |
| P1 | `refresh_policy.go` | +11/-0 | 刷新策略 | ☐ |
| P1 | `refresh_token_cache.go` | +30/-0 | 刷新Token缓存 | ☐ |
| P1 | `token_cache_invalidator.go` `token_cache_invalidator_test.go` | +9/-6 | Token缓存失效器（含 1 个测试文件） | ☐ |
| P1 | `token_cache_key.go` `token_cache_key_test.go` | +6/-0 | Token缓存Key（含 1 个测试文件） | ☐ |
| P1 | `token_refresh_service.go` `token_refresh_service_test.go` | +210/-83 | Token刷新服务（含 1 个测试文件） | ☐ |
| P1 | `totp_service.go` | +7/-3 | TOTP服务 | ☐ |
| P2 | `api_key_auth_cache_version_test.go` | +3/-3 | APIKey认证缓存version（测试） | ☐ |
| P2 | `api_key_service_cache_test.go` | +294/-9 | APIKey服务缓存（测试） | ☐ |
| P2 | `api_key_service_delete_test.go` | +20/-2 | APIKey服务删除（测试） | ☐ |
| P2 | `api_key_service_quota_test.go` | +8/-5 | APIKey服务quota（测试） | ☐ |
| P2 | `auth_registration_atomicity_test.go` | +226/-0 | 认证注册原子性（测试） | ☐ |
| P2 | `auth_service_email_bind_test.go` | +75/-0 | 认证服务邮件绑定（测试） | ☐ |
| P2 | `auth_service_identity_sync_test.go` | +8/-1 | 认证服务身份同步（测试） | ☐ |
| P2 | `auth_service_nil_config_test.go` | +36/-0 | 认证服务nil配置（测试） | ☐ |
| P2 | `auth_service_platform_quota_test.go` | +29/-0 | 认证服务平台quota（测试） | ☐ |
| P2 | `auth_service_register_test.go` | +42/-6 | 认证服务register（测试） | ☐ |
| P2 | `oauth_service_nil_dependency_test.go` | +58/-0 | OAuth服务nildependency（测试） | ☐ |
| P2 | `token_refresh_pool_health_test.go` | +2/-1 | Token刷新连接池健康（测试） | ☐ |
| P2 | `token_refresher_test.go` | +24/-0 | Token刷新器（测试） | ☐ |

## 3. 反封号与 TLS 指纹伪装（Security-Fingerprint）（建议批次 early，整体重要度 P0）

TLS 指纹路由/画像导入/归一化、账号级指纹策略绑定、IP 安全检测、anti-ban 平台策略。几乎纯新增，本体独立性好，被各平台请求构造调用。建议整体搬运。其中 ip_security 属真实安全控制。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `fingerprint_normalizer.go` | +1347/-0 | 指纹归一化核心；请求指纹的规范化与 PII 处理。 | ☐ |
| P0 | `ip_security.go` `ip_security_test.go` | +760/-0 | IP 安全检测；风险 IP 识别与拦截，安全控制。（含 1 个测试文件） | ☐ |
| P1 | `account_tls_fingerprint_policy.go` `account_tls_fingerprint_policy_test.go` | +294/-0 | 账号TLS指纹策略（含 1 个测试文件） | ☐ |
| P1 | `account_tls_fingerprint_resolve.go` `account_tls_fingerprint_resolve_test.go` | +403/-0 | 账号TLS指纹解析（含 1 个测试文件） | ☐ |
| P1 | `anti_ban_platforms.go` | +40/-0 | antibanplatforms | ☐ |
| P1 | `fingerprint_normalizer_api_key_test.go` | +199/-0 | 指纹归一化APIKey（测试） | ☐ |
| P1 | `fingerprint_normalizer_pii_test.go` | +558/-0 | 指纹归一化pii（测试） | ☐ |
| P1 | `fingerprint_normalizer_platform_test.go` | +201/-0 | 指纹归一化平台（测试） | ☐ |
| P1 | `kiro_tls_profile.go` | +162/-0 | KiroTLS画像 | ☐ |
| P1 | `openai_fingerprint_aware_transport_test.go` | +182/-0 | OpenAI指纹aware传输（测试） | ☐ |
| P1 | `openai_fingerprint_body.go` | +57/-0 | OpenAI指纹body | ☐ |
| P1 | `openai_tls_fingerprint_dimension_test.go` | +246/-0 | OpenAITLS指纹维度（测试） | ☐ |
| P1 | `openai_tls_fingerprint_router.go` `openai_tls_fingerprint_router_test.go` | +140/-0 | OpenAITLS指纹路由（含 1 个测试文件） | ☐ |
| P1 | `openai_tls_fingerprint_router_seed_test.go` | +52/-0 | OpenAITLS指纹路由种子（测试） | ☐ |
| P1 | `openai_tls_profile_test.go` | +415/-0 | OpenAITLS画像（测试） | ☐ |
| P1 | `tls_fingerprint_dimension_binding_test.go` | +141/-0 | TLS指纹维度绑定（测试） | ☐ |
| P1 | `tls_fingerprint_profile_import.go` `tls_fingerprint_profile_import_test.go` | +709/-0 | TLS指纹画像导入（含 1 个测试文件） | ☐ |
| P1 | `tls_fingerprint_profile_service.go` | +409/-29 | TLS指纹画像服务 | ☐ |
| P1 | `tls_fingerprint_request_context.go` | +25/-0 | TLS指纹请求上下文 | ☐ |
| P1 | `tls_fingerprint_router_service.go` `tls_fingerprint_router_service_test.go` | +415/-0 | TLS指纹路由服务（含 1 个测试文件） | ☐ |
| P1 | `tls_fingerprint_usage.go` | +59/-0 | TLS指纹用量 | ☐ |

## 4. 系统设置服务（Setting-Config）（建议批次 early（接受持续增量），整体重要度 P0）

全局配置读写中心，承载几乎所有功能开关。personal-dev 把上游的 setting_features/gateway_runtime/oauth/parse/public/update 合并进单一 setting_service.go（结构性替换，删除上游 6 文件）。第二大脊柱（60 提交/23 模块），与 domain_constants.go 强绑定。建议 early 迁最小骨架，各模块批次增量 patch。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `domain_constants.go` | +176/-53 | 领域常量（脊柱，41 提交/23 模块）；平台/账号类型等常量，新增平台必碰。 | ☐ |
| P0 | `setting_service.go` | +7614/-142 | 系统设置服务（第二大脊柱，60 提交/23 模块）；全局配置读写中心，承载所有功能开关。 | ☐ |
| P1 | `setting_features.go` | +0/-1177 | 设置功能开关（删除上游文件） | ☐ |
| P1 | `setting_gateway_runtime.go` | +0/-1010 | 设置网关运行时（删除上游文件） | ☐ |
| P1 | `setting_oauth.go` | +0/-1015 | 设置OAuth（删除上游文件） | ☐ |
| P1 | `setting_parse.go` | +0/-1305 | 设置解析（删除上游文件） | ☐ |
| P1 | `setting_public.go` | +0/-817 | 设置公开（删除上游文件） | ☐ |
| P1 | `setting_update.go` | +0/-830 | 设置更新（删除上游文件） | ☐ |
| P1 | `settings_view.go` | +225/-69 | 设置视图 | ☐ |
| P2 | `setting_service_admin_api_key_test.go` | +47/-0 | 设置服务管理端APIKey（测试） | ☐ |
| P2 | `setting_service_antiban_test.go` | +31/-0 | 设置服务反封号（测试） | ☐ |
| P2 | `setting_service_auth_source_defaults_test.go` | +49/-0 | 设置服务认证source默认（测试） | ☐ |
| P2 | `setting_service_backend_mode_test.go` | +16/-0 | 设置服务backend模式（测试） | ☐ |
| P2 | `setting_service_cached_nil_repo_test.go` | +72/-0 | 设置服务cachednilrepo（测试） | ☐ |
| P2 | `setting_service_claude_oauth_system_prompt_test.go` | +11/-0 | 设置服务ClaudeOAuthsystemprompt（测试） | ☐ |
| P2 | `setting_service_claude_telemetry_test.go` | +40/-0 | 设置服务Claude遥测（测试） | ☐ |
| P2 | `setting_service_codex_policy_test.go` | +18/-0 | 设置服务Codex策略（测试） | ☐ |
| P2 | `setting_service_gateway_forwarding_nil_repo_test.go` | +28/-0 | 设置服务网关forwardingnilrepo（测试） | ☐ |
| P2 | `setting_service_model_plaza_update_test.go` | +33/-0 | 设置服务模型广场更新（测试） | ☐ |
| P2 | `setting_service_nil_config_parse_test.go` | +39/-0 | 设置服务nil配置解析（测试） | ☐ |
| P2 | `setting_service_nil_repo_defaults_test.go` | +59/-0 | 设置服务nilrepo默认（测试） | ☐ |
| P2 | `setting_service_oauth_nil_repo_test.go` | +152/-0 | 设置服务OAuthnilrepo（测试） | ☐ |
| P2 | `setting_service_platform_quota_test.go` | +31/-5 | 设置服务平台quota（测试） | ☐ |
| P2 | `setting_service_platform_threshold_test.go` | +6/-16 | 设置服务平台阈值（测试） | ☐ |
| P2 | `setting_service_policy_setters_nil_repo_test.go` | +70/-0 | 设置服务策略setternilrepo（测试） | ☐ |
| P2 | `setting_service_public_test.go` | +78/-33 | 设置服务公开（测试） | ☐ |
| P2 | `setting_service_runtime_nil_repo_test.go` | +99/-0 | 设置服务运行时nilrepo（测试） | ☐ |
| P2 | `setting_service_scalar_nil_repo_defaults_test.go` | +75/-0 | 设置服务标量nilrepo默认（测试） | ☐ |
| P2 | `setting_service_ticket_templates_test.go` | +144/-0 | 设置服务工单templates（测试） | ☐ |
| P2 | `setting_service_update_test.go` | +870/-308 | 设置服务更新（测试） | ☐ |
| P2 | `tencent_captcha_settings_test.go` | +8/-11 | 腾讯验证码设置（测试） | ☐ |

## 5. 账号调度、限流熔断与临时下线（Scheduling-TempUnsched）（建议批次 early，整体重要度 P0）

调度阈值评估、模型级限流、模型 fallback、组容量、429 冷却、可恢复失败策略、错误透传、临时下线、定时可用性测试、跨平台模型路由、调度快照。所有平台网关的失败降级路径都经过这里，与 Account-Core、Billing-Usage 强耦合。ratelimit_service.go/scheduler_snapshot_service.go 为脊柱，做增量 patch。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `ratelimit_service.go` | +1091/-299 | 限流与熔断服务（脊柱，32 提交/23 模块）；429/401/403 冷却、模型级限流。 | ☐ |
| P0 | `scheduler_snapshot_service.go` | +240/-136 | 调度快照服务；账号候选池快照的构建、预热与增量维护。 | ☐ |
| P0 | `temp_unsched.go` `temp_unsched_test.go` | +85/-1 | 临时下线（含 1 个测试文件） | ☐ |
| P1 | `account_scheduling_threshold_eval.go` | +0/-29 | 账号调度阈值评估（删除上游文件） | ☐ |
| P1 | `account_scheduling_threshold_snapshot_cleanup.go` `account_scheduling_threshold_snapshot_cleanup_test.go` | +18/-0 | 账号调度阈值快照清理（含 1 个测试文件） | ☐ |
| P1 | `error_passthrough_runtime.go` `error_passthrough_runtime_test.go` | +1/-1 | 错误透传运行时（含 1 个测试文件） | ☐ |
| P1 | `error_passthrough_service.go` `error_passthrough_service_test.go` | +50/-10 | 错误透传服务（含 1 个测试文件） | ☐ |
| P1 | `group_capacity_service.go` `group_capacity_service_test.go` | +29/-65 | 分组容量服务（含 1 个测试文件） | ☐ |
| P1 | `group_model_unsupported.go` `group_model_unsupported_test.go` | +192/-0 | 分组模型unsupported（含 1 个测试文件） | ☐ |
| P1 | `model_fallback.go` `model_fallback_test.go` | +71/-0 | 模型fallback（含 1 个测试文件） | ☐ |
| P1 | `model_rate_limit.go` `model_rate_limit_test.go` | +5/-0 | 模型速率限制（含 1 个测试文件） | ☐ |
| P1 | `platform_model_routing.go` `platform_model_routing_test.go` | +253/-0 | 平台模型路由（含 1 个测试文件） | ☐ |
| P1 | `recoverable_failure_policy.go` `recoverable_failure_policy_test.go` | +71/-0 | 可恢复失败策略（含 1 个测试文件） | ☐ |
| P1 | `scheduled_test_port.go` | +1/-0 | scheduled端口 | ☐ |
| P1 | `scheduled_test_runner_service.go` `scheduled_test_runner_service_test.go` | +78/-1 | scheduled执行器服务（含 1 个测试文件） | ☐ |
| P1 | `scheduled_test_service.go` `scheduled_test_service_test.go` | +10/-0 | scheduled服务（含 1 个测试文件） | ☐ |
| P1 | `scheduler_cache.go` | +0/-2 | 调度器缓存（删除上游文件） | ☐ |
| P1 | `scheduler_outbox.go` | +14/-18 | 调度器outbox | ☐ |
| P1 | `scheduler_shuffle_test.go` | +12/-0 | 调度器shuffle（测试） | ☐ |
| P1 | `scheduler_snapshot_full_rebuild_lifecycle_test.go` | +5/-3 | 调度器快照fullrebuild生命周期（测试） | ☐ |
| P1 | `scheduler_snapshot_hydration_cache_test.go` | +78/-0 | 调度器快照预热缓存（测试） | ☐ |
| P1 | `scheduler_snapshot_hydration_test.go` | +45/-64 | 调度器快照预热（测试） | ☐ |
| P1 | `scheduler_snapshot_outbox_cleanup_test.go` | +119/-818 | 调度器快照outbox清理（测试） | ☐ |
| P1 | `scheduler_snapshot_retirement_test.go` | +7/-0 | 调度器快照retirement（测试） | ☐ |
| P1 | `scheduler_snapshot_service_token_test.go` | +68/-0 | 调度器快照服务Token（测试） | ☐ |
| P2 | `account_scheduling_threshold_eval_test.go` | +0/-31 | 账号调度阈值评估（测试）（删除上游文件） | ☐ |
| P2 | `account_scheduling_threshold_integration_test.go` | +1/-1 | 账号调度阈值集成（测试） | ☐ |
| P2 | `error_policy_test.go` | +22/-2 | 错误策略（测试） | ☐ |
| P2 | `overload_cooldown_test.go` | +14/-0 | overload冷却（测试） | ☐ |
| P2 | `platform_model_routing_test_helper_test.go` | +6/-0 | 平台模型路由helper（测试） | ☐ |
| P2 | `rate_limit_429_cooldown_test.go` | +634/-3 | 速率限制429冷却（测试） | ☐ |
| P2 | `ratelimit_account_repo_shared_test.go` | +172/-0 | 限流账号reposhared（测试） | ☐ |
| P2 | `ratelimit_service_401_test.go` | +78/-52 | 限流服务401（测试） | ☐ |
| P2 | `ratelimit_service_403_test.go` | +95/-0 | 限流服务403（测试） | ☐ |
| P2 | `ratelimit_service_clear_test.go` | +250/-1 | 限流服务清空（测试） | ☐ |
| P2 | `ratelimit_service_model_not_found_test.go` | +18/-0 | 限流服务模型notfound（测试） | ☐ |
| P2 | `ratelimit_session_window_nil_account_test.go` | +26/-0 | 限流会话窗口nil账号（测试） | ☐ |
| P2 | `recoverable_failure_policy_ingress_test.go` | +103/-0 | 可恢复失败策略入口（测试） | ☐ |
| P2 | `temp_unsched_threshold_test.go` | +449/-0 | 临时下线阈值（测试） | ☐ |

## 6. 容量预测服务（Capacity-Forecast）（建议批次 early（紧跟 Account-Core），整体重要度 P2）

账号容量预测框架 + 各平台容量 Provider，后台展示“账号还能撑多久”。全部新增，仅依赖各平台账号字段，独立性极佳。可整体搬运。属后台便利观测能力。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P2 | `capacity_forecast_pure.go` `capacity_forecast_pure_test.go` | +500/-0 | 容量预测pure（含 1 个测试文件） | ☐ |
| P2 | `capacity_forecast_service.go` `capacity_forecast_service_test.go` | +627/-0 | 容量预测服务（含 1 个测试文件） | ☐ |
| P2 | `capacity_forecast_types.go` | +184/-0 | 容量预测类型 | ☐ |
| P2 | `capacity_provider.go` | +99/-0 | 容量Provider | ☐ |
| P2 | `capacity_provider_anthropic.go` | +191/-0 | 容量ProviderAnthropic | ☐ |
| P2 | `capacity_provider_antigravity.go` | +135/-0 | 容量ProviderAntigravity | ☐ |
| P2 | `capacity_provider_common.go` | +107/-0 | 容量Providercommon | ☐ |
| P2 | `capacity_provider_gemini.go` | +185/-0 | 容量ProviderGemini | ☐ |
| P2 | `capacity_provider_grok.go` | +164/-0 | 容量ProviderGrok | ☐ |
| P2 | `capacity_provider_kiro.go` | +119/-0 | 容量ProviderKiro | ☐ |
| P2 | `capacity_provider_new_platforms_test.go` | +330/-0 | 容量Providernewplatforms（测试） | ☐ |
| P2 | `capacity_provider_openai.go` | +126/-0 | 容量ProviderOpenAI | ☐ |

## 7. 计费与用量统计（Billing-Usage）（建议批次 early（基础能力先行），整体重要度 P0）

请求计费、计费缓存、账号用量统计、定价与模型定价解析、视频计费、发票、用户平台配额、用量清理归档、批量入库 worker pool。所有网关请求闭环的必经环节。建议基础能力先行，各平台批次追加计费分支。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `account_usage_service.go` `account_usage_service_test.go` | +1369/-152 | 账号用量统计服务；按账号聚合 token/成本用量。（含 1 个测试文件） | ☐ |
| P0 | `billing_cache_service.go` | +622/-76 | 计费缓存服务 | ☐ |
| P0 | `billing_service.go` `billing_service_test.go` | +621/-312 | 计费服务（脊柱，22 提交/20 模块）；请求计费闭环主入口。（含 1 个测试文件） | ☐ |
| P0 | `pricing_service.go` `pricing_service_test.go` | +98/-109 | 定价服务；模型定价解析与用户售价倍率计算。（含 1 个测试文件） | ☐ |
| P0 | `usage_record_worker_pool.go` `usage_record_worker_pool_test.go` | +123/-78 | 用量记录worker连接池（含 1 个测试文件） | ☐ |
| P1 | `account_stats_pricing.go` `account_stats_pricing_test.go` | +23/-7 | 账号统计定价（含 1 个测试文件） | ☐ |
| P1 | `account_usage_service_batch_test.go` | +3/-124 | 账号用量服务batch（测试） | ☐ |
| P1 | `balance_cache_outbox.go` | +17/-0 | 余额缓存outbox | ☐ |
| P1 | `balance_notify_check_test.go` | +39/-0 | 余额notifycheck（测试） | ☐ |
| P1 | `balance_notify_service.go` | +12/-0 | 余额notify服务 | ☐ |
| P1 | `billing_cache_service_balance_test.go` | +349/-2 | 计费缓存服务余额（测试） | ☐ |
| P1 | `billing_cache_service_nil_config_test.go` | +29/-0 | 计费缓存服务nil配置（测试） | ☐ |
| P1 | `billing_cache_service_rpm_test.go` | +153/-9 | 计费缓存服务RPM（测试） | ☐ |
| P1 | `billing_cache_service_singleflight_test.go` | +4/-0 | 计费缓存服务singleflight（测试） | ☐ |
| P1 | `billing_cache_service_user_platform_quota_test.go` | +115/-6 | 计费缓存服务用户平台quota（测试） | ☐ |
| P1 | `billing_search_audio_cost_test.go` | +3/-3 | 计费搜索音频成本（测试） | ☐ |
| P1 | `billing_service_image_test.go` | +22/-0 | 计费服务图像（测试） | ☐ |
| P1 | `billing_service_unified_test.go` | +145/-0 | 计费服务unified（测试） | ☐ |
| P1 | `force_cache_billing_test.go` | +2/-58 | force缓存计费（测试） | ☐ |
| P1 | `invoice_service.go` `invoice_service_test.go` | +932/-0 | invoice服务（含 1 个测试文件） | ☐ |
| P1 | `media_price_config.go` | +3/-49 | 媒体price配置 | ☐ |
| P1 | `model_pricing_resolver.go` `model_pricing_resolver_test.go` | +3/-6 | 模型定价解析器（含 1 个测试文件） | ☐ |
| P1 | `pricing_service_nil_config_test.go` | +21/-0 | 定价服务nil配置（测试） | ☐ |
| P1 | `usage_billing.go` | +9/-6 | 用量计费 | ☐ |
| P1 | `usage_billing_selection.go` | +40/-0 | 用量计费选择 | ☐ |
| P1 | `usage_cleanup.go` | +12/-10 | 用量清理 | ☐ |
| P1 | `usage_cleanup_service.go` `usage_cleanup_service_test.go` | +11/-0 | 用量清理服务（含 1 个测试文件） | ☐ |
| P1 | `usage_log.go` | +96/-15 | 用量日志 | ☐ |
| P1 | `usage_log_cyber_test.go` | +18/-0 | 用量日志cyber（测试） | ☐ |
| P1 | `usage_model_resolution.go` `usage_model_resolution_test.go` | +61/-0 | 用量模型解析（含 1 个测试文件） | ☐ |
| P1 | `usage_service.go` | +7/-5 | 用量服务 | ☐ |
| P1 | `usage_user_daily_cost_aggregator.go` | +161/-0 | 用量用户日成本聚合器 | ☐ |
| P1 | `usage_user_daily_cost_state.go` | +17/-0 | 用量用户日成本状态 | ☐ |
| P1 | `user_platform_quota_db_aggregator.go` `user_platform_quota_db_aggregator_test.go` | +248/-0 | 用户平台quotadb聚合器（含 1 个测试文件） | ☐ |
| P1 | `user_platform_quota_flusher.go` `user_platform_quota_flusher_test.go` | +24/-15 | 用户平台quotaflusher（含 1 个测试文件） | ☐ |
| P1 | `user_platform_quota_port.go` | +8/-0 | 用户平台quota端口 | ☐ |
| P1 | `user_rpm_cache.go` | +2/-2 | 用户RPM缓存 | ☐ |
| P1 | `video_billing.go` `video_billing_test.go` | +114/-97 | 视频计费（含 1 个测试文件） | ☐ |

## 8. 内容审核（Content-Moderation）（建议批次 early，整体重要度 P0）

请求/响应内容审核（关键词匹配、邻近度、规避检测、输入过滤、邮件通知）。被 gateway_service 调用，自身内聚。属安全能力。建议整体搬运。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `content_moderation.go` `content_moderation_test.go` | +2215/-347 | 内容审核核心；关键词/规避检测、请求响应过滤，安全能力。（含 1 个测试文件） | ☐ |
| P1 | `content_moderation_cyber_test.go` | +144/-14 | 内容审核cyber（测试） | ☐ |
| P1 | `content_moderation_email.go` | +1/-1 | 内容审核邮件 | ☐ |
| P1 | `content_moderation_evasion_test.go` | +109/-0 | 内容审核规避（测试） | ☐ |
| P1 | `content_moderation_input.go` `content_moderation_input_test.go` | +201/-19 | 内容审核输入（含 1 个测试文件） | ☐ |
| P1 | `content_moderation_keyword_matcher.go` | +100/-15 | 内容审核关键词匹配器 | ☐ |
| P1 | `content_moderation_proximity_test.go` | +392/-0 | 内容审核proximity（测试） | ☐ |

## 9. Anthropic/Claude 网关核心（Claude-Anthropic-Gateway）（建议批次 批次4，整体重要度 P0）

网关主干——gateway_service.go 本体、协议桥接转发、账号选择、计费头、websearch、工具改写、debug 时间线、count_tokens、共享上游传输层、Kiro/Bedrock 走 Claude 协议兼容层、thinking 协议。全仓库耦合度最高（81 提交/24 模块）。personal-dev 把 gateway_forward/scheduling/upstream_* 等拆分重组（删除上游 10 文件），属结构性替换。建议 early-middle 落地骨架，之后增量 patch。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `composite_platform.go` | +0/-30 | 复合分组目标平台解析（上游文件被 personal-dev 删除/重组）。（删除上游文件） | ☐ |
| P0 | `gateway_anthropic_passthrough.go` | +0/-849 | 网关Anthropic透传（删除上游文件） | ☐ |
| P0 | `gateway_bedrock.go` | +0/-412 | 网关Bedrock（删除上游文件） | ☐ |
| P0 | `gateway_claude_oauth_body.go` | +0/-1258 | 网关ClaudeOAuthbody（删除上游文件） | ☐ |
| P0 | `gateway_count_tokens.go` | +0/-610 | 网关计数tokens（删除上游文件） | ☐ |
| P0 | `gateway_forward.go` | +0/-986 | 网关转发（删除上游文件） | ☐ |
| P0 | `gateway_scheduling.go` | +0/-2583 | 网关调度（删除上游文件） | ☐ |
| P0 | `gateway_service.go` | +11234/-240 | 网关主干服务（全仓库最高耦合，81 提交/24 模块）；Claude 协议转发、账号选择、计费头。 | ☐ |
| P0 | `gateway_upstream_request.go` | +0/-919 | 网关上游请求（删除上游文件） | ☐ |
| P0 | `gateway_upstream_response.go` | +0/-1487 | 网关上游响应（删除上游文件） | ☐ |
| P0 | `gateway_usage_billing.go` | +0/-1144 | 网关用量计费（删除上游文件） | ☐ |
| P0 | `upstream_path_guard.go` `upstream_path_guard_test.go` | +7/-2 | 上游 URL 路径片段护栏；闭集允许清单，防路径结构注入，安全护栏。（含 1 个测试文件） | ☐ |
| P1 | `claude_antiban_gate.go` | +27/-0 | Claude反封号门控 | ☐ |
| P1 | `claude_token_provider.go` `claude_token_provider_test.go` | +3/-3 | ClaudeTokenProvider（含 1 个测试文件） | ☐ |
| P1 | `gateway_anthropic_model_mapping.go` `gateway_anthropic_model_mapping_test.go` | +68/-0 | 网关Anthropic模型映射（含 1 个测试文件） | ☐ |
| P1 | `gateway_billing_block.go` `gateway_billing_block_test.go` | +88/-11 | 网关计费拦截（含 1 个测试文件） | ☐ |
| P1 | `gateway_billing_header.go` `gateway_billing_header_test.go` | +7/-4 | 网关计费头（含 1 个测试文件） | ☐ |
| P1 | `gateway_forward_as_chat_completions.go` `gateway_forward_as_chat_completions_test.go` | +85/-37 | 网关转发asChatCompletions（含 1 个测试文件） | ☐ |
| P1 | `gateway_forward_as_responses.go` `gateway_forward_as_responses_test.go` | +243/-102 | 网关转发asResponses（含 1 个测试文件） | ☐ |
| P1 | `gateway_forward_as_responses_ingress.go` | +120/-0 | 网关转发asResponses入口 | ☐ |
| P1 | `gateway_forward_messages_to_cc.go` | +344/-0 | 网关转发messagestocc | ☐ |
| P1 | `gateway_messages_cache.go` | +6/-1 | 网关messages缓存 | ☐ |
| P1 | `gateway_record_usage_image_quota_test.go` | +96/-0 | 网关记录用量图像quota（测试） | ☐ |
| P1 | `gateway_record_usage_test.go` | +301/-42 | 网关记录用量（测试） | ☐ |
| P1 | `gateway_request.go` `gateway_request_test.go` | +45/-6 | 网关请求（含 1 个测试文件） | ☐ |
| P1 | `gateway_service_bedrock_beta_test.go` | +77/-0 | 网关服务Bedrockbeta（测试） | ☐ |
| P1 | `gateway_service_billing_nil_deps_test.go` | +143/-0 | 网关服务计费nildeps（测试） | ☐ |
| P1 | `gateway_tool_rewrite.go` `gateway_tool_rewrite_test.go` | +615/-24 | 网关工具改写（含 1 个测试文件） | ☐ |
| P1 | `gateway_upstream_attempt.go` `gateway_upstream_attempt_test.go` | +145/-0 | 网关上游attempt（含 1 个测试文件） | ☐ |
| P1 | `gateway_websearch_block_filter.go` `gateway_websearch_block_filter_test.go` | +84/-77 | 网关websearch拦截过滤（含 1 个测试文件） | ☐ |
| P1 | `gateway_websearch_emulation.go` `gateway_websearch_emulation_test.go` | +12/-6 | 网关websearch模拟（含 1 个测试文件） | ☐ |
| P1 | `http_upstream_port.go` | +28/-0 | HTTP上游端口 | ☐ |
| P1 | `http_upstream_request_options.go` | +53/-0 | HTTP上游请求options | ☐ |
| P1 | `inbound_endpoint_local.go` | +20/-0 | inbound端点local | ☐ |
| P1 | `request_metadata.go` | +15/-0 | 请求metadata | ☐ |
| P1 | `session_id.go` | +33/-0 | 会话ID | ☐ |
| P1 | `thinking_protocol.go` `thinking_protocol_test.go` | +2/-2 | thinking协议（含 1 个测试文件） | ☐ |
| P1 | `upstream_models.go` `upstream_models_test.go` | +8/-6 | 上游models（含 1 个测试文件） | ☐ |
| P1 | `upstream_response_limit.go` `upstream_response_limit_test.go` | +1/-1 | 上游响应限制（含 1 个测试文件） | ☐ |
| P1 | `upstream_status.go` | +22/-0 | 上游status | ☐ |
| P1 | `upstream_transport_error.go` `upstream_transport_error_test.go` | +191/-0 | 上游传输错误（含 1 个测试文件） | ☐ |
| P1 | `websearch_config.go` `websearch_config_test.go` | +16/-4 | websearch配置（含 1 个测试文件） | ☐ |
| P2 | `claude_oauth_body_integration_test.go` | +296/-0 | ClaudeOAuthbody集成（测试） | ☐ |
| P2 | `claude_telemetry_forward.go` `claude_telemetry_forward_test.go` | +167/-0 | Claude遥测转发（含 1 个测试文件） | ☐ |
| P2 | `claude_telemetry_sanitizer.go` `claude_telemetry_sanitizer_test.go` | +300/-0 | Claude遥测脱敏器（含 1 个测试文件） | ☐ |
| P2 | `gateway_access_token_nil_test.go` | +57/-0 | 网关accessTokennil（测试） | ☐ |
| P2 | `gateway_account_selection_test.go` | +12/-0 | 网关账号选择（测试） | ☐ |
| P2 | `gateway_anthropic_apikey_passthrough_test.go` | +182/-162 | 网关AnthropicAPIKey透传（测试） | ☐ |
| P2 | `gateway_anthropic_vertex_service_account_test.go` | +1/-1 | 网关Anthropicvertex服务账号（测试） | ☐ |
| P2 | `gateway_channel_restriction_test.go` | +3/-3 | 网关渠道限制（测试） | ☐ |
| P2 | `gateway_context_management_test.go` | +1/-0 | 网关上下文management（测试） | ☐ |
| P2 | `gateway_count_tokens_ops_test.go` | +249/-0 | 网关计数tokens运维（测试） | ☐ |
| P2 | `gateway_debug_env_test.go` | +67/-1 | 网关调试env（测试） | ☐ |
| P2 | `gateway_debug_timeline.go` `gateway_debug_timeline_test.go` | +316/-0 | 网关调试时间线（含 1 个测试文件） | ☐ |
| P2 | `gateway_debug_timeline_body.go` | +273/-0 | 网关调试时间线body | ☐ |
| P2 | `gateway_forward_as_ops_test.go` | +928/-0 | 网关转发as运维（测试） | ☐ |
| P2 | `gateway_forward_nil_account_test.go` | +49/-0 | 网关转发nil账号（测试） | ☐ |
| P2 | `gateway_hotpath_optimization_test.go` | +31/-0 | 网关热路径优化（测试） | ☐ |
| P2 | `gateway_multiplatform_test.go` | +255/-13 | 网关多平台（测试） | ☐ |
| P2 | `gateway_prompt_test.go` | +25/-2 | 网关prompt（测试） | ☐ |
| P2 | `gateway_request_invalid_json_test.go` | +0/-52 | 网关请求invalidjson（测试）（删除上游文件） | ☐ |
| P2 | `gateway_sanitize_test.go` | +18/-0 | 网关sanitize（测试） | ☐ |
| P2 | `gateway_search_billing_test.go` | +97/-0 | 网关搜索计费（测试） | ☐ |
| P2 | `gateway_session_wait_test.go` | +64/-0 | 网关会话wait（测试） | ☐ |
| P2 | `gateway_streaming_test.go` | +30/-6 | 网关流式（测试） | ☐ |
| P2 | `gateway_text_endpoint_auto_route_test.go` | +347/-0 | 网关text端点autoroute（测试） | ☐ |
| P2 | `upstream_transport_failover_test.go` | +166/-0 | 上游传输故障切换（测试） | ☐ |

## 10. OpenAI 账号调度与 OAuth 容量管理（OpenAI-OAuth-Scheduling）（建议批次 批次5a，整体重要度 P0）

OpenAI(Codex) 专属账号调度器（粘性会话/权重）、OAuth token provider、OAuth 容量时间序列、APIKey 健康熔断、CRS 账号同步、shadow 影子路由、利润控制准入、Codex 邀请重置。openai_account_scheduler.go 是 OpenAI 系二号脊柱（61 提交/23 模块）。OpenAI 系列第一棒。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `openai_account_scheduler.go` `openai_account_scheduler_test.go` | +2075/-1094 | OpenAI 账号调度器（OpenAI 系二号脊柱，61 提交/23 模块）；粘性会话/权重调度。（含 1 个测试文件） | ☐ |
| P0 | `openai_oauth_service.go` | +144/-28 | OpenAIOAuth服务 | ☐ |
| P0 | `openai_token_provider.go` `openai_token_provider_test.go` | +57/-17 | OpenAITokenProvider（含 1 个测试文件） | ☐ |
| P1 | `codex_invite_reset_service.go` `codex_invite_reset_service_test.go` | +787/-0 | Codex 账号邀请重置服务（纯新增）。（含 1 个测试文件） | ☐ |
| P1 | `crs_sync_service.go` | +31/-2 | CRS同步服务 | ☐ |
| P1 | `openai_account_runtime_block_fastpath.go` `openai_account_runtime_block_fastpath_test.go` | +114/-14 | OpenAI账号运行时拦截fastpath（含 1 个测试文件） | ☐ |
| P1 | `openai_account_runtime_snapshot.go` | +31/-0 | OpenAI账号运行时快照 | ☐ |
| P1 | `openai_account_scheduler_compact_test.go` | +57/-22 | OpenAI账号调度器compact（测试） | ☐ |
| P1 | `openai_account_scheduler_upstream_cost_test.go` | +1/-1 | OpenAI账号调度器上游成本（测试） | ☐ |
| P1 | `openai_apikey_health_breaker.go` `openai_apikey_health_breaker_test.go` | +228/-0 | OpenAIAPIKey健康熔断（含 1 个测试文件） | ☐ |
| P1 | `openai_apikey_responses_probe.go` | +1/-1 | OpenAIAPIKeyResponses探测 | ☐ |
| P1 | `openai_oauth_capacity.go` `openai_oauth_capacity_test.go` | +715/-0 | OpenAIOAuth容量（含 1 个测试文件） | ☐ |
| P1 | `openai_oauth_capacity_timeseries.go` `openai_oauth_capacity_timeseries_test.go` | +1011/-0 | OpenAIOAuth容量时间序列（含 1 个测试文件） | ☐ |
| P1 | `openai_profit_control.go` | +5/-0 | 分组利润控制准入过滤（配套 migration 192/193 的 profit_* 字段）。 | ☐ |
| P1 | `openai_quota_service.go` | +146/-82 | OpenAIquota服务 | ☐ |
| P1 | `openai_sticky_schedule_ops.go` `openai_sticky_schedule_ops_test.go` | +54/-0 | OpenAI粘性schedule运维（含 1 个测试文件） | ☐ |
| P2 | `crs_sync_service_openai_test.go` | +77/-0 | CRS同步服务OpenAI（测试） | ☐ |
| P2 | `openai_capacity_shed_test.go` | +175/-0 | OpenAI容量卸载（测试） | ☐ |
| P2 | `openai_oauth_model_support_test.go` | +22/-5 | OpenAIOAuth模型支撑（测试） | ☐ |
| P2 | `openai_oauth_service_nil_client_test.go` | +41/-0 | OpenAIOAuth服务nil客户端（测试） | ☐ |
| P2 | `openai_oauth_service_nil_proxy_test.go` | +47/-0 | OpenAIOAuth服务nilproxy（测试） | ☐ |
| P2 | `openai_oauth_service_redirect_test.go` | +106/-0 | OpenAIOAuth服务redirect（测试） | ☐ |
| P2 | `openai_oauth_service_refresh_test.go` | +44/-3 | OpenAIOAuth服务刷新（测试） | ☐ |
| P2 | `openai_oauth_service_state_test.go` | +88/-6 | OpenAIOAuth服务状态（测试） | ☐ |
| P2 | `openai_profit_control_paths_test.go` | +4/-4 | OpenAI利润控制paths（测试） | ☐ |
| P2 | `openai_quota_spark_window_test.go` | +5/-3 | OpenAIquotaspark窗口（测试） | ☐ |
| P2 | `shadow_routing.go` | +4/-10 | spark 影子账号调度准入；母账号凭据可用性 fail-closed 判定。 | ☐ |

## 11. OpenAI Codex/Responses 网关核心（OpenAI-Gateway-Core）（建议批次 批次5b（自身拆 3 子批），整体重要度 P0）

service 层最大模块——Responses/Chat Completions/Codex 协议互转、模型映射别名、会话续接、compact、Claude-compat、embeddings、audio、live、agent identity、粘性会话、web profile。openai_gateway_service.go 195 提交涉及全部 24 模块，与 Claude 网关双向耦合，删除上游 11 文件属结构性替换。建议自身拆 3 子批：骨架+模型映射 → Codex 转换/续接 → compact/compat 兼容层。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `openai_codex_transform.go` `openai_codex_transform_test.go` | +2113/-512 | Codex 协议双向转换核心；Responses↔Chat↔Codex 报文互转。（含 1 个测试文件） | ☐ |
| P0 | `openai_gateway_cc_pipeline.go` | +0/-349 | OpenAI网关ccpipeline（删除上游文件） | ☐ |
| P0 | `openai_gateway_forward.go` | +0/-1149 | OpenAI网关转发（删除上游文件） | ☐ |
| P0 | `openai_gateway_messages_chat_fallback.go` | +0/-262 | OpenAI网关messagesChatfallback（删除上游文件） | ☐ |
| P0 | `openai_gateway_passthrough.go` | +0/-1580 | OpenAI网关透传（删除上游文件） | ☐ |
| P0 | `openai_gateway_request_body.go` | +0/-1478 | OpenAI网关请求body（删除上游文件） | ☐ |
| P0 | `openai_gateway_response_handling.go` | +0/-1791 | OpenAI网关响应处理（删除上游文件） | ☐ |
| P0 | `openai_gateway_scheduling.go` | +0/-1499 | OpenAI网关调度（删除上游文件） | ☐ |
| P0 | `openai_gateway_service.go` `openai_gateway_service_test.go` | +16127/-161 | OpenAI 网关主干（195 提交涉及全部 24 模块）；Responses/Chat/Codex 转发核心。（含 1 个测试文件） | ☐ |
| P0 | `openai_gateway_upstream_errors.go` | +0/-683 | OpenAI网关上游errors（删除上游文件） | ☐ |
| P0 | `openai_gateway_usage.go` | +0/-950 | OpenAI网关用量（删除上游文件） | ☐ |
| P0 | `openai_messages_digest_session.go` | +0/-104 | OpenAImessages摘要会话（删除上游文件） | ☐ |
| P0 | `openai_messages_todo_guard.go` | +0/-37 | OpenAImessagestodo护栏（删除上游文件） | ☐ |
| P1 | `gateway_forward_openai_compat_cc.go` | +723/-0 | 网关转发OpenAI兼容cc | ☐ |
| P1 | `openai_agent_identity.go` | +44/-44 | OpenAIagent身份 | ☐ |
| P1 | `openai_alpha_search.go` `openai_alpha_search_test.go` | +9/-3 | OpenAIalpha搜索（含 1 个测试文件） | ☐ |
| P1 | `openai_audio.go` `openai_audio_test.go` | +230/-0 | OpenAI音频（含 1 个测试文件） | ☐ |
| P1 | `openai_auto_compaction.go` `openai_auto_compaction_test.go` | +199/-0 | OpenAIautocompaction（含 1 个测试文件） | ☐ |
| P1 | `openai_client_transport.go` `openai_client_transport_test.go` | +2/-1 | OpenAI客户端传输（含 1 个测试文件） | ☐ |
| P1 | `openai_client_visible_upstream_error.go` `openai_client_visible_upstream_error_test.go` | +153/-0 | OpenAI客户端可见上游错误（含 1 个测试文件） | ☐ |
| P1 | `openai_codex_identity.go` `openai_codex_identity_test.go` | +48/-4 | OpenAICodex身份（含 1 个测试文件） | ☐ |
| P1 | `openai_codex_identity_extra.go` `openai_codex_identity_extra_test.go` | +59/-0 | OpenAICodex身份extra（含 1 个测试文件） | ☐ |
| P1 | `openai_codex_models_service.go` `openai_codex_models_service_test.go` | +20/-5 | OpenAICodexmodels服务（含 1 个测试文件） | ☐ |
| P1 | `openai_codex_probe_headers.go` | +79/-0 | OpenAICodex探测头 | ☐ |
| P1 | `openai_codex_tool_names.go` | +141/-0 | OpenAICodex工具名 | ☐ |
| P1 | `openai_compact_probe.go` | +71/-0 | OpenAIcompact探测 | ☐ |
| P1 | `openai_compact_sse_keepalive.go` `openai_compact_sse_keepalive_test.go` | +1/-5 | OpenAIcompactssekeepalive（含 1 个测试文件） | ☐ |
| P1 | `openai_compact_stream_bridge.go` `openai_compact_stream_bridge_test.go` | +46/-0 | OpenAIcompact流桥接（含 1 个测试文件） | ☐ |
| P1 | `openai_compat_dropped_fields.go` | +12/-0 | OpenAI兼容丢弃字段 | ☐ |
| P1 | `openai_compat_model.go` `openai_compat_model_test.go` | +27/-1 | OpenAI兼容模型（含 1 个测试文件） | ☐ |
| P1 | `openai_compat_prompt_cache_key.go` `openai_compat_prompt_cache_key_test.go` | +56/-67 | OpenAI兼容prompt缓存Key（含 1 个测试文件） | ☐ |
| P1 | `openai_content_session_seed.go` `openai_content_session_seed_test.go` | +16/-25 | OpenAI内容会话种子（含 1 个测试文件） | ☐ |
| P1 | `openai_cyber_policy.go` `openai_cyber_policy_test.go` | +81/-13 | OpenAIcyber策略（含 1 个测试文件） | ☐ |
| P1 | `openai_cyber_session_block.go` `openai_cyber_session_block_test.go` | +55/-0 | OpenAIcyber会话拦截（含 1 个测试文件） | ☐ |
| P1 | `openai_embeddings.go` `openai_embeddings_test.go` | +896/-11 | OpenAIEmbeddings（含 1 个测试文件） | ☐ |
| P1 | `openai_endpoint_url.go` `openai_endpoint_url_test.go` | +38/-4 | OpenAI端点URL（含 1 个测试文件） | ☐ |
| P1 | `openai_first_output_timeout.go` | +28/-0 | OpenAI首输出超时 | ☐ |
| P1 | `openai_gateway_chat_completions.go` `openai_gateway_chat_completions_test.go` | +1137/-428 | OpenAI网关ChatCompletions（含 1 个测试文件） | ☐ |
| P1 | `openai_gateway_chat_completions_raw.go` `openai_gateway_chat_completions_raw_test.go` | +679/-144 | OpenAI网关ChatCompletionsraw（含 1 个测试文件） | ☐ |
| P1 | `openai_gateway_count_tokens.go` `openai_gateway_count_tokens_test.go` | +21/-8 | OpenAI网关计数tokens（含 1 个测试文件） | ☐ |
| P1 | `openai_gateway_forward_cc_to_messages.go` | +253/-0 | OpenAI网关转发cctomessages | ☐ |
| P1 | `openai_gateway_messages.go` `openai_gateway_messages_test.go` | +1064/-804 | OpenAI网关messages（含 1 个测试文件） | ☐ |
| P1 | `openai_gateway_model_availability.go` | +1/-15 | OpenAI网关模型availability | ☐ |
| P1 | `openai_gateway_record_usage_audit_test.go` | +153/-0 | OpenAI网关记录用量审计（测试） | ☐ |
| P1 | `openai_gateway_record_usage_test.go` | +1119/-76 | OpenAI网关记录用量（测试） | ☐ |
| P1 | `openai_gateway_request_body_failover.go` | +81/-0 | OpenAI网关请求body故障切换 | ☐ |
| P1 | `openai_gateway_responses_chat_fallback.go` `openai_gateway_responses_chat_fallback_test.go` | +254/-47 | OpenAI网关ResponsesChatfallback（含 1 个测试文件） | ☐ |
| P1 | `openai_gateway_service_codex_cli_only_test.go` | +9/-9 | OpenAI网关服务Codexclionly（测试） | ☐ |
| P1 | `openai_gateway_service_codex_snapshot_test.go` | +36/-0 | OpenAI网关服务Codex快照（测试） | ☐ |
| P1 | `openai_gateway_service_hotpath_test.go` | +154/-2 | OpenAI网关服务热路径（测试） | ☐ |
| P1 | `openai_gateway_service_replay_protocol_regression_test.go` | +177/-0 | OpenAI网关服务回放协议回归（测试） | ☐ |
| P1 | `openai_gateway_service_sse_audit_test.go` | +319/-0 | OpenAI网关服务sse审计（测试） | ☐ |
| P1 | `openai_http1_replay.go` `openai_http1_replay_test.go` | +73/-0 | OpenAIHTTP1回放（含 1 个测试文件） | ☐ |
| P1 | `openai_http_active_delta.go` `openai_http_active_delta_test.go` | +350/-0 | OpenAIHTTPactive增量（含 1 个测试文件） | ☐ |
| P1 | `openai_live.go` | +3/-1 | OpenAI实时 | ☐ |
| P1 | `openai_messages_bridge.go` | +16/-1 | OpenAImessages桥接 | ☐ |
| P1 | `openai_messages_continuation.go` | +32/-1 | OpenAImessages续接 | ☐ |
| P1 | `openai_messages_dispatch.go` `openai_messages_dispatch_test.go` | +57/-38 | OpenAImessages分发（含 1 个测试文件） | ☐ |
| P1 | `openai_model_alias.go` | +92/-38 | OpenAI模型别名 | ☐ |
| P1 | `openai_model_capabilities.go` `openai_model_capabilities_test.go` | +92/-0 | OpenAI模型能力（含 1 个测试文件） | ☐ |
| P1 | `openai_model_mapping.go` `openai_model_mapping_test.go` | +59/-25 | OpenAI模型映射（含 1 个测试文件） | ☐ |
| P1 | `openai_oauth_passthrough_test.go` | +2018/-1107 | OpenAIOAuth透传（测试） | ☐ |
| P1 | `openai_request_ordered_marshal.go` `openai_request_ordered_marshal_test.go` | +99/-0 | OpenAI请求有序序列化（含 1 个测试文件） | ☐ |
| P1 | `openai_responses_ingress_normalization.go` `openai_responses_ingress_normalization_test.go` | +461/-0 | OpenAIResponses入口归一化（含 1 个测试文件） | ☐ |
| P1 | `openai_responses_lite_tools.go` | +22/-0 | OpenAIResponseslitetools | ☐ |
| P1 | `openai_responses_namespace.go` | +64/-81 | OpenAIResponses命名空间 | ☐ |
| P1 | `openai_responses_session_window.go` `openai_responses_session_window_test.go` | +60/-0 | OpenAIResponses会话窗口（含 1 个测试文件） | ☐ |
| P1 | `openai_silent_refusal.go` | +34/-95 | OpenAI静默拒答 | ☐ |
| P1 | `openai_sse_json_documents.go` | +61/-0 | OpenAIssejsondocuments | ☐ |
| P1 | `openai_sticky_compat.go` `openai_sticky_compat_test.go` | +115/-9 | OpenAI粘性兼容（含 1 个测试文件） | ☐ |
| P1 | `openai_tool_continuation.go` `openai_tool_continuation_test.go` | +86/-25 | OpenAI工具续接（含 1 个测试文件） | ☐ |
| P1 | `openai_web_profile.go` `openai_web_profile_test.go` | +946/-0 | OpenAIweb画像（含 1 个测试文件） | ☐ |
| P1 | `ops_openai_token_stats_models.go` | +9/-7 | 运维OpenAIToken统计models | ☐ |
| P2 | `account_test_service_openai_compact_test.go` | +10/-5 | 账号服务OpenAIcompact（测试） | ☐ |
| P2 | `account_test_service_openai_test.go` | +72/-40 | 账号服务OpenAI（测试） | ☐ |
| P2 | `gin_test_mode_test.go` | +22/-0 | gin模式（测试） | ☐ |
| P2 | `golden_test_helpers_test.go` | +180/-0 | goldenhelpers（测试） | ☐ |
| P2 | `openai_agent_identity_compat_test.go` | +16/-24 | OpenAIagent身份兼容（测试） | ☐ |
| P2 | `openai_alpha_search_billing_test.go` | +6/-6 | OpenAIalpha搜索计费（测试） | ☐ |
| P2 | `openai_cache_probe_test.go` | +160/-0 | OpenAI缓存探测（测试） | ☐ |
| P2 | `openai_chat_max_output_retry_test.go` | +99/-0 | OpenAIChatmax输出重试（测试） | ☐ |
| P2 | `openai_client_restriction_detector_test.go` | +1/-1 | OpenAI客户端限制detector（测试） | ☐ |
| P2 | `openai_codex_function_call_id_test.go` | +60/-5 | OpenAICodex函数调用ID（测试） | ☐ |
| P2 | `openai_codex_message_item_id_test.go` | +5/-5 | OpenAICodex消息itemID（测试） | ☐ |
| P2 | `openai_codex_orphan_function_call_output_test.go` | +189/-0 | OpenAICodex孤儿函数调用输出（测试） | ☐ |
| P2 | `openai_codex_ratelimit_headers_test.go` | +83/-0 | OpenAICodex限流头（测试） | ☐ |
| P2 | `openai_compact_model_mapping_test.go` | +204/-3 | OpenAIcompact模型映射（测试） | ☐ |
| P2 | `openai_compact_request_test.go` | +193/-0 | OpenAIcompact请求（测试） | ☐ |
| P2 | `openai_compat_golden_test.go` | +69/-0 | OpenAI兼容golden（测试） | ☐ |
| P2 | `openai_compat_nil_account_test.go` | +65/-0 | OpenAI兼容nil账号（测试） | ☐ |
| P2 | `openai_compat_observability.go` `openai_compat_observability_test.go` | +117/-0 | OpenAI兼容可观测（含 1 个测试文件） | ☐ |
| P2 | `openai_compat_observability_bound_test.go` | +30/-0 | OpenAI兼容可观测bound（测试） | ☐ |
| P2 | `openai_continuation_session_test.go` | +29/-0 | OpenAI续接会话（测试） | ☐ |
| P2 | `openai_cursor_warmup_pipeline_test.go` | +12/-0 | OpenAIcursor预热pipeline（测试） | ☐ |
| P2 | `openai_cyber_policy_integration_test.go` | +117/-0 | OpenAIcyber策略集成（测试） | ☐ |
| P2 | `openai_fast_policy_ws_test.go` | +22/-2 | OpenAIfast策略ws（测试） | ☐ |
| P2 | `openai_gateway_audio_billing_test.go` | +57/-0 | OpenAI网关音频计费（测试） | ☐ |
| P2 | `openai_gateway_bridge_stream_test.go` | +183/-0 | OpenAI网关桥接流（测试） | ☐ |
| P2 | `openai_gateway_force_codex_cli_test.go` | +21/-0 | OpenAI网关forceCodexcli（测试） | ☐ |
| P2 | `openai_gateway_messages_chat_fallback_test.go` | +23/-432 | OpenAI网关messagesChatfallback（测试） | ☐ |
| P2 | `openai_gateway_messages_failed_response_test.go` | +0/-185 | OpenAI网关messagesfailed响应（测试）（删除上游文件） | ☐ |
| P2 | `openai_gateway_messages_nil_guard_test.go` | +16/-0 | OpenAI网关messagesnil护栏（测试） | ☐ |
| P2 | `openai_gateway_messages_transport_failover_test.go` | +0/-92 | OpenAI网关messages传输故障切换（测试）（删除上游文件） | ☐ |
| P2 | `openai_gateway_messages_usage_test.go` | +17/-0 | OpenAI网关messages用量（测试） | ☐ |
| P2 | `openai_gateway_nil_account_test.go` | +28/-0 | OpenAI网关nil账号（测试） | ☐ |
| P2 | `openai_gateway_partial_billing_test.go` | +199/-0 | OpenAI网关部分计费（测试） | ☐ |
| P2 | `openai_gateway_passthrough_image_intent_test.go` | +1/-1 | OpenAI网关透传图像intent（测试） | ☐ |
| P2 | `openai_gateway_request_body_reasoning_test.go` | +61/-79 | OpenAI网关请求bodyreasoning（测试） | ☐ |
| P2 | `openai_gateway_response_failed_passthrough_test.go` | +181/-2 | OpenAI网关响应failed透传（测试） | ☐ |
| P2 | `openai_gateway_search_billing_test.go` | +113/-0 | OpenAI网关搜索计费（测试） | ☐ |
| P2 | `openai_gateway_search_surcharge_test.go` | +6/-3 | OpenAI网关搜索附加费（测试） | ☐ |
| P2 | `openai_gateway_timeline.go` | +314/-0 | OpenAI网关时间线 | ☐ |
| P2 | `openai_gpt56_max_test.go` | +61/-10 | OpenAIgpt56max（测试） | ☐ |
| P2 | `openai_gpt56_support_test.go` | +151/-0 | OpenAIgpt56支撑（测试） | ☐ |
| P2 | `openai_live_lifecycle_test.go` | +2/-0 | OpenAI实时生命周期（测试） | ☐ |
| P2 | `openai_messages_continuation_compat_test.go` | +20/-0 | OpenAImessages续接兼容（测试） | ☐ |
| P2 | `openai_messages_continuation_reaper_test.go` | +36/-0 | OpenAImessages续接reaper（测试） | ☐ |
| P2 | `openai_originator_header_test.go` | +38/-0 | OpenAIoriginator头（测试） | ☐ |
| P2 | `openai_passthrough_normalization_test.go` | +395/-30 | OpenAI透传归一化（测试） | ☐ |
| P2 | `openai_privacy_retry_test.go` | +24/-0 | OpenAI隐私重试（测试） | ☐ |
| P2 | `openai_request_body_limit_failover_test.go` | +22/-1 | OpenAI请求body限制故障切换（测试） | ☐ |
| P2 | `openai_responses_tool_schema_test.go` | +3/-2 | OpenAIResponses工具schema（测试） | ☐ |
| P2 | `openai_responses_usage_capture_test.go` | +102/-0 | OpenAIResponses用量capture（测试） | ☐ |
| P2 | `openai_routing_hint_test.go` | +1/-0 | OpenAI路由提示（测试） | ☐ |
| P2 | `openai_sse_concatenated_json_test.go` | +12/-5 | OpenAIsseconcatenatedjson（测试） | ☐ |
| P2 | `openai_stream_failed_event_test.go` | +49/-0 | OpenAI流failedevent（测试） | ☐ |
| P2 | `openai_unsupported_previous_response_id_test.go` | +56/-0 | OpenAIunsupportedprevious响应ID（测试） | ☐ |
| P2 | `ops_openai_token_stats_test.go` | +2/-2 | 运维OpenAIToken统计（测试） | ☐ |
| P2 | `ratelimit_service_openai_test.go` | +126/-30 | 限流服务OpenAI（测试） | ☐ |
| P2 | `sticky_session_test.go` | +2/-2 | 粘性会话（测试） | ☐ |
| P2 | `testdata/openai_claude_compat/chat_prompt_cache_key_injection_apikey/chat_request.json` `testdata/openai_claude_compat/chat_prompt_cache_key_injection_apikey/expected_downstream_response.json` `testdata/openai_claude_compat/chat_prompt_cache_key_injection_apikey/expected_upstream_request.json` `testdata/openai_claude_compat/chat_prompt_cache_key_injection_apikey/upstream_sse.txt` | +33/-0 | OpenAI↔Claude 兼容 golden 测试夹具（chat_prompt_cache_key_injection_apikey）；请求/上游/下游/SSE 样例。 | ☐ |
| P2 | `testdata/openai_claude_compat/chat_prompt_cache_key_ordering_oauth/chat_request.json` `testdata/openai_claude_compat/chat_prompt_cache_key_ordering_oauth/expected_downstream_response.json` `testdata/openai_claude_compat/chat_prompt_cache_key_ordering_oauth/expected_upstream_request.json` `testdata/openai_claude_compat/chat_prompt_cache_key_ordering_oauth/upstream_sse.txt` | +39/-0 | OpenAI↔Claude 兼容 golden 测试夹具（chat_prompt_cache_key_ordering_oauth）；请求/上游/下游/SSE 样例。 | ☐ |
| P2 | `testdata/openai_claude_compat/previous_response_id_http_strip/anthropic_request.json` `testdata/openai_claude_compat/previous_response_id_http_strip/expected_downstream_response.json` `testdata/openai_claude_compat/previous_response_id_http_strip/expected_upstream_request.json` `testdata/openai_claude_compat/previous_response_id_http_strip/upstream_sse.txt` | +33/-0 | OpenAI↔Claude 兼容 golden 测试夹具（previous_response_id_http_strip）；请求/上游/下游/SSE 样例。 | ☐ |
| P2 | `testdata/openai_claude_compat/prompt_cache_key_ordering/anthropic_request.json` `testdata/openai_claude_compat/prompt_cache_key_ordering/expected_downstream_response.json` `testdata/openai_claude_compat/prompt_cache_key_ordering/expected_upstream_request.json` `testdata/openai_claude_compat/prompt_cache_key_ordering/upstream_sse.txt` | +35/-0 | OpenAI↔Claude 兼容 golden 测试夹具（prompt_cache_key_ordering）；请求/上游/下游/SSE 样例。 | ☐ |
| P3 | `cch_verify_all_test.go` | +94/-0 | cch校验全量（测试） | ☐ |

## 12. OpenAI 实时 WebSocket 转发（OpenAI-WS）（建议批次 批次5c，整体重要度 P0）

Codex WS 长连接转发、WS v2 passthrough、delta shadow 对比、会话抢占、预检、HTTP↔WS 桥接、连接池与状态存储。几乎重写级改造（删除上游 5 文件为结构性替换）。对外仅少数入口函数，内部自洽，单独成批。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `openai_ws_forwarder.go` `openai_ws_forwarder_test.go` | +9214/-125 | OpenAI Codex WS 转发器（重写级巨型文件）；WS 长连接转发主干。（含 1 个测试文件） | ☐ |
| P0 | `openai_ws_forwarder_ingress.go` | +0/-1722 | OpenAIws转发器入口（删除上游文件） | ☐ |
| P0 | `openai_ws_forwarder_logutil.go` | +0/-691 | OpenAIws转发器日志工具（删除上游文件） | ☐ |
| P0 | `openai_ws_forwarder_payload.go` | +0/-751 | OpenAIws转发器payload（删除上游文件） | ☐ |
| P0 | `openai_ws_forwarder_support.go` | +0/-758 | OpenAIws转发器支撑（删除上游文件） | ☐ |
| P0 | `openai_ws_forwarder_v2.go` | +0/-796 | OpenAIws转发器v2（删除上游文件） | ☐ |
| P0 | `openai_ws_pool.go` `openai_ws_pool_test.go` | +1864/-676 | OpenAI WS 连接池；长连接复用、抢占与回收。（含 1 个测试文件） | ☐ |
| P0 | `openai_ws_state_store.go` `openai_ws_state_store_test.go` | +1057/-56 | OpenAI WS 会话状态存储；WS 会话状态的持久与恢复。（含 1 个测试文件） | ☐ |
| P1 | `openai_ws_client.go` `openai_ws_client_test.go` | +313/-24 | OpenAIws客户端（含 1 个测试文件） | ☐ |
| P1 | `openai_ws_client_h2.go` | +874/-0 | OpenAIws客户端H2 | ☐ |
| P1 | `openai_ws_http_bridge.go` `openai_ws_http_bridge_test.go` | +240/-183 | OpenAIwsHTTP桥接（含 1 个测试文件） | ☐ |
| P1 | `openai_ws_pool_reconciler.go` | +174/-0 | OpenAIws连接池对账器 | ☐ |
| P1 | `openai_ws_preflight.go` `openai_ws_preflight_test.go` | +32/-0 | OpenAIws预检（含 1 个测试文件） | ☐ |
| P1 | `openai_ws_protocol_forward_test.go` | +3854/-431 | OpenAIws协议转发（测试） | ☐ |
| P1 | `openai_ws_session_preemption.go` `openai_ws_session_preemption_test.go` | +301/-0 | OpenAIws会话抢占（含 1 个测试文件） | ☐ |
| P1 | `openai_ws_transient_compat.go` | +73/-0 | OpenAIwstransient兼容 | ☐ |
| P1 | `openai_ws_v2/passthrough_relay.go` `openai_ws_v2/passthrough_relay_test.go` | +349/-117 | 透传relay（含 1 个测试文件） | ☐ |
| P1 | `openai_ws_v2_passthrough_adapter.go` `openai_ws_v2_passthrough_adapter_test.go` | +528/-139 | OpenAIwsv2透传适配器（含 1 个测试文件） | ☐ |
| P2 | `admin_service_openai_ws_reconcile_test.go` | +473/-0 | 管理端服务OpenAIws对账（测试） | ☐ |
| P2 | `openai_ws_account_sticky_test.go` | +87/-19 | OpenAIws账号粘性（测试） | ☐ |
| P2 | `openai_ws_context_compaction_test.go` | +690/-0 | OpenAIws上下文compaction（测试） | ☐ |
| P2 | `openai_ws_delta_shadow.go` `openai_ws_delta_shadow_test.go` | +1329/-0 | OpenAIws增量影子（含 1 个测试文件） | ☐ |
| P2 | `openai_ws_fallback_test.go` | +1085/-11 | OpenAIwsfallback（测试） | ☐ |
| P2 | `openai_ws_forwarder_hotpath_optimization_test.go` | +1/-0 | OpenAIws转发器热路径优化（测试） | ☐ |
| P2 | `openai_ws_forwarder_ingress_session_test.go` | +1943/-262 | OpenAIws转发器入口会话（测试） | ☐ |
| P2 | `openai_ws_forwarder_ingress_test.go` | +577/-39 | OpenAIws转发器入口（测试） | ☐ |
| P2 | `openai_ws_forwarder_retry_payload_test.go` | +14/-0 | OpenAIws转发器重试payload（测试） | ☐ |
| P2 | `openai_ws_forwarder_success_test.go` | +1796/-253 | OpenAIws转发器成功（测试） | ☐ |
| P2 | `openai_ws_forwarder_timeout_test.go` | +48/-0 | OpenAIws转发器超时（测试） | ☐ |
| P2 | `openai_ws_guard_nil_test.go` | +144/-0 | OpenAIws护栏nil（测试） | ☐ |
| P2 | `openai_ws_mode_log_test.go` | +268/-0 | OpenAIws模式日志（测试） | ☐ |
| P2 | `openai_ws_nil_account_test.go` | +246/-0 | OpenAIwsnil账号（测试） | ☐ |
| P2 | `openai_ws_passthrough_turn_pricing_test.go` | +5/-14 | OpenAIws透传turn定价（测试） | ☐ |
| P2 | `openai_ws_pool_runtime_test.go` | +760/-0 | OpenAIws连接池运行时（测试） | ☐ |
| P2 | `openai_ws_ratelimit_signal_test.go` | +567/-35 | OpenAIws限流信号（测试） | ☐ |
| P2 | `openai_ws_state_store_bound_test.go` | +89/-0 | OpenAIws状态存储bound（测试） | ☐ |
| P2 | `openai_ws_synthetic_usage_test.go` | +28/-0 | OpenAIws合成用量（测试） | ☐ |
| P2 | `openai_ws_temp_diag.go` | +173/-0 | OpenAIws临时诊断 | ☐ |
| P2 | `openai_ws_v2/passthrough_relay_internal_test.go` | +203/-4 | 透传relay内部（测试） | ☐ |
| P2 | `openai_ws_v2_passthrough_lifecycle_test.go` | +2/-1 | OpenAIwsv2透传生命周期（测试） | ☐ |

## 13. OpenAI 图像生成与计费（OpenAI-Images）（建议批次 批次5d，整体重要度 P1）

图像生成、批量图片任务、图片计费倍率、输出核算、Codex 图像桥接、图片遥测。紧随 OpenAI-Gateway-Core。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `batch_image_billing_hold.go` | +7/-0 | batch图像计费hold | ☐ |
| P1 | `batch_image_public.go` `batch_image_public_test.go` | +17/-10 | batch图像公开（含 1 个测试文件） | ☐ |
| P1 | `codex_image_generation_bridge.go` | +8/-3 | Codex图像生成桥接 | ☐ |
| P1 | `image_billing_multiplier.go` `image_billing_multiplier_test.go` | +41/-0 | 图像计费multiplier（含 1 个测试文件） | ☐ |
| P1 | `image_billing_size.go` `image_billing_size_test.go` | +27/-34 | 图像计费size（含 1 个测试文件） | ☐ |
| P1 | `image_generation_intent.go` `image_generation_intent_test.go` | +355/-70 | 图像生成intent（含 1 个测试文件） | ☐ |
| P1 | `image_output_accounting.go` `image_output_accounting_test.go` | +16/-7 | 图像输出accounting（含 1 个测试文件） | ☐ |
| P1 | `openai_images.go` `openai_images_test.go` | +1587/-124 | OpenAI图像（含 1 个测试文件） | ☐ |
| P1 | `openai_images_responses.go` | +1050/-727 | OpenAI图像Responses | ☐ |
| P2 | `account_test_service_openai_image_test.go` | +194/-6 | 账号服务OpenAI图像（测试） | ☐ |
| P2 | `batch_image_cleanup_test.go` | +2/-2 | batch图像清理（测试） | ☐ |
| P2 | `batch_image_provider_vertex_test.go` | +3/-3 | batch图像Providervertex（测试） | ☐ |
| P2 | `batch_image_settlement_test.go` | +21/-0 | batch图像结算（测试） | ☐ |
| P2 | `openai_image_generation_controls_test.go` | +86/-63 | OpenAI图像生成controls（测试） | ☐ |
| P2 | `openai_image_intent_hint_test.go` | +3/-3 | OpenAI图像intent提示（测试） | ☐ |
| P2 | `openai_images_actual_size_test.go` | +101/-0 | OpenAI图像actualsize（测试） | ☐ |
| P2 | `openai_images_http_upstream_test.go` | +49/-0 | OpenAI图像HTTP上游（测试） | ☐ |
| P2 | `openai_images_incomplete_test.go` | +21/-2 | OpenAI图像incomplete（测试） | ☐ |
| P2 | `openai_images_json_keepalive_test.go` | +10/-1 | OpenAI图像jsonkeepalive（测试） | ☐ |
| P2 | `openai_images_telemetry.go` `openai_images_telemetry_test.go` | +304/-0 | OpenAI图像遥测（含 1 个测试文件） | ☐ |
| P2 | `openai_images_web_profile_integration_test.go` | +174/-0 | OpenAI图像web画像集成（测试） | ☐ |
| P2 | `ratelimit_service_openai_image_test.go` | +8/-2 | 限流服务OpenAI图像（测试） | ☐ |

## 14. Grok（xAI）集成（Grok）（建议批次 批次6（须在 OpenAI 系列之后），整体重要度 P1）

Grok OAuth/quota、专属限流、音频/媒体、借 OpenAI Chat Completions 协议壳接入 Grok 上游。网关桥接寄生在 OpenAI 兼容层，强依赖 OpenAI-Gateway-Core。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `gateway_service_grok_websearch_test.go` | +124/-0 | 网关服务Grokwebsearch（测试） | ☐ |
| P1 | `grok_audio.go` `grok_audio_test.go` | +95/-162 | Grok音频（含 1 个测试文件） | ☐ |
| P1 | `grok_credential_failure.go` `grok_credential_failure_test.go` | +20/-4 | Grokcredential失败（含 1 个测试文件） | ☐ |
| P1 | `grok_free_quota_gate.go` `grok_free_quota_gate_test.go` | +109/-189 | Grok免费quota门控（含 1 个测试文件） | ☐ |
| P1 | `grok_media.go` | +944/-579 | Grok媒体 | ☐ |
| P1 | `grok_oauth_service.go` `grok_oauth_service_test.go` | +65/-72 | GrokOAuth服务（含 1 个测试文件） | ☐ |
| P1 | `grok_quota_fetcher.go` `grok_quota_fetcher_test.go` | +46/-89 | Grokquotafetcher（含 1 个测试文件） | ☐ |
| P1 | `grok_quota_service.go` `grok_quota_service_test.go` | +367/-333 | Grokquota服务（含 1 个测试文件） | ☐ |
| P1 | `grok_token_provider.go` `grok_token_provider_test.go` | +183/-77 | GrokTokenProvider（含 1 个测试文件） | ☐ |
| P1 | `grok_token_refresher.go` | +22/-39 | GrokToken刷新器 | ☐ |
| P1 | `grok_upstream_errors.go` `grok_upstream_errors_test.go` | +56/-0 | Grok上游errors（含 1 个测试文件） | ☐ |
| P1 | `grok_upstream_headers.go` `grok_upstream_headers_test.go` | +22/-14 | Grok上游头（含 1 个测试文件） | ☐ |
| P1 | `grok_upstream_url.go` | +42/-23 | Grok上游URL | ☐ |
| P1 | `grok_video_billing.go` | +213/-0 | Grok视频计费 | ☐ |
| P1 | `openai_gateway_grok.go` `openai_gateway_grok_test.go` | +1950/-740 | OpenAI网关Grok（含 1 个测试文件） | ☐ |
| P1 | `openai_gateway_grok_active_delta.go` `openai_gateway_grok_active_delta_test.go` | +1050/-0 | OpenAI网关Grokactive增量（含 1 个测试文件） | ☐ |
| P1 | `openai_gateway_grok_cache.go` `openai_gateway_grok_cache_test.go` | +96/-80 | OpenAI网关Grok缓存（含 1 个测试文件） | ☐ |
| P1 | `openai_gateway_grok_chat_bridge.go` `openai_gateway_grok_chat_bridge_test.go` | +88/-514 | OpenAI网关GrokChat桥接（含 1 个测试文件） | ☐ |
| P1 | `openai_gateway_grok_compact.go` | +0/-233 | OpenAI网关Grokcompact（删除上游文件） | ☐ |
| P2 | `account_grok_media_eligibility_test.go` | +19/-23 | 账号Grok媒体eligibility（测试） | ☐ |
| P2 | `account_test_service_grok_test.go` | +694/-347 | 账号服务Grok（测试） | ☐ |
| P2 | `grok_base_url_mode_test.go` | +86/-48 | Grok基础URL模式（测试） | ☐ |
| P2 | `grok_media_forward_test.go` | +891/-0 | Grok媒体转发（测试） | ☐ |
| P2 | `grok_oauth_service_nil_client_test.go` | +45/-0 | GrokOAuth服务nil客户端（测试） | ☐ |
| P2 | `grok_search_count_test.go` | +31/-0 | Grok搜索计数（测试） | ☐ |
| P2 | `openai_gateway_grok_bridge_test.go` | +411/-0 | OpenAI网关Grok桥接（测试） | ☐ |
| P2 | `openai_gateway_grok_search_billing_test.go` | +4/-4 | OpenAI网关Grok搜索计费（测试） | ☐ |

## 15. Kiro / Bedrock 网关（Kiro-Bedrock）（建议批次 批次6，整体重要度 P0）

Kiro 完整网关栈——OAuth、token、gateway/usage service、invokeMCP 沙箱、错误解析、fake cache、Bedrock 适配。几乎纯新增、零侵入，本体可整体搬运，集成点集中在 gateway_service/account/admin_service/setting_service/TLS 指纹，等骨架就位后接入。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `kiro_gateway_service.go` `kiro_gateway_service_test.go` | +5189/-0 | Kiro 网关服务（纯新增巨型文件）；Kiro 转发/协议适配全栈。（含 1 个测试文件） | ☐ |
| P0 | `kiro_oauth_service.go` `kiro_oauth_service_test.go` | +2540/-0 | Kiro OAuth 服务（纯新增）；Kiro 授权码/token 全流程。（含 1 个测试文件） | ☐ |
| P0 | `kiro_token_provider.go` `kiro_token_provider_test.go` | +283/-0 | KiroTokenProvider（含 1 个测试文件） | ☐ |
| P0 | `kiro_usage_service.go` | +793/-0 | Kiro用量服务 | ☐ |
| P1 | `bedrock_request.go` `bedrock_request_test.go` | +9/-7 | Bedrock请求（含 1 个测试文件） | ☐ |
| P1 | `gateway_kiro_openai_compat.go` `gateway_kiro_openai_compat_test.go` | +129/-0 | 网关KiroOpenAI兼容（含 1 个测试文件） | ☐ |
| P1 | `gateway_service_kiro_delegate_test.go` | +130/-0 | 网关服务Kiro委派（测试） | ☐ |
| P1 | `kiro_error_detail.go` `kiro_error_detail_test.go` | +179/-0 | Kiro错误detail（含 1 个测试文件） | ☐ |
| P1 | `kiro_fake_cache.go` | +65/-0 | Kirofake缓存 | ☐ |
| P1 | `kiro_gateway_service_prestart_failover_test.go` | +312/-0 | Kiro网关服务prestart故障切换（测试） | ☐ |
| P1 | `kiro_gateway_service_unit_test.go` | +741/-0 | Kiro网关服务单元（测试） | ☐ |
| P1 | `kiro_invokemcp.go` `kiro_invokemcp_test.go` | +325/-0 | KiroinvokeMCP（含 1 个测试文件） | ☐ |
| P1 | `kiro_token_refresher.go` `kiro_token_refresher_test.go` | +452/-0 | KiroToken刷新器（含 1 个测试文件） | ☐ |
| P2 | `account_kiro_mapping_passthrough_test.go` | +117/-0 | 账号Kiro映射透传（测试） | ☐ |
| P2 | `account_service_kiro_oauth_group_test.go` | +506/-0 | 账号服务KiroOAuth分组（测试） | ☐ |
| P2 | `account_test_service_kiro_test.go` | +9/-0 | 账号服务Kiro（测试） | ☐ |
| P2 | `account_test_service_kiro_tls_test.go` | +319/-0 | 账号服务KiroTLS（测试） | ☐ |
| P2 | `account_usage_service_kiro_retry_test.go` | +158/-0 | 账号用量服务Kiro重试（测试） | ☐ |
| P2 | `account_usage_service_kiro_test.go` | +894/-0 | 账号用量服务Kiro（测试） | ☐ |
| P2 | `admin_service_kiro_oauth_group_test.go` | +128/-0 | 管理端服务KiroOAuth分组（测试） | ☐ |
| P2 | `admin_service_kiro_validation_test.go` | +703/-0 | 管理端服务Kirovalidation（测试） | ☐ |
| P2 | `kiro_code_execution_sandbox_test.go` | +153/-0 | Kirocodeexecution沙箱（测试） | ☐ |
| P2 | `kiro_default_test_stubs_test.go` | +406/-0 | Kirodefaultstubs（测试） | ☐ |
| P2 | `kiro_gateway_nil_account_test.go` | +49/-0 | Kiro网关nil账号（测试） | ☐ |
| P2 | `kiro_model_routing_test.go` | +28/-0 | Kiro模型路由（测试） | ☐ |
| P2 | `kiro_oauth_redis_fallback_test.go` | +42/-0 | KiroOAuthredisfallback（测试） | ☐ |
| P2 | `kiro_oauth_service_security_test.go` | +373/-0 | KiroOAuth服务security（测试） | ☐ |
| P2 | `kiro_oauth_service_state_error_test.go` | +152/-0 | KiroOAuth服务状态错误（测试） | ☐ |
| P2 | `kiro_oauth_service_unit_test.go` | +91/-0 | KiroOAuth服务单元（测试） | ☐ |
| P2 | `kiro_sidecar_transport_test.go` | +900/-0 | Kirosidecar传输（测试） | ☐ |
| P2 | `kiro_usage_service_unit_test.go` | +19/-0 | Kiro用量服务单元（测试） | ☐ |
| P2 | `ratelimit_service_kiro_test.go` | +122/-0 | 限流服务Kiro（测试） | ☐ |
| P2 | `setting_service_kiro_runtime_test.go` | +283/-0 | 设置服务Kiro运行时（测试） | ☐ |

## 16. Gemini 网关（Gemini）（建议批次 批次6，整体重要度 P1）

Gemini messages/chat completions 兼容、OAuth、token provider、quota、Code Assist 客户端。相对独立，改动都在 Gemini 专属文件，可与 Kiro/Antigravity 并行。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `gemini_chat_completions_compat_service.go` | +622/-18 | GeminiChatCompletions兼容服务 | ☐ |
| P1 | `gemini_messages_compat_service.go` `gemini_messages_compat_service_test.go` | +682/-188 | Geminimessages兼容服务（含 1 个测试文件） | ☐ |
| P1 | `gemini_oauth_service.go` `gemini_oauth_service_test.go` | +966/-486 | GeminiOAuth服务（含 1 个测试文件） | ☐ |
| P1 | `gemini_quota.go` `gemini_quota_test.go` | +1/-2 | Geminiquota（含 1 个测试文件） | ☐ |
| P1 | `gemini_token_cache.go` | +2/-2 | GeminiToken缓存 | ☐ |
| P1 | `gemini_token_provider.go` `gemini_token_provider_test.go` | +104/-38 | GeminiTokenProvider（含 1 个测试文件） | ☐ |
| P1 | `gemini_token_refresher.go` | +1/-1 | GeminiToken刷新器 | ☐ |
| P1 | `geminicli_codeassist.go` | +2/-0 | GeminiCliCode Assist | ☐ |
| P2 | `account_gemini_test.go` | +327/-0 | 账号Gemini（测试） | ☐ |
| P2 | `account_test_service_gemini_test.go` | +188/-2 | 账号服务Gemini（测试） | ☐ |
| P2 | `gemini_aistudio_get_ops_test.go` | +337/-0 | Geminiaistudioget运维（测试） | ☐ |
| P2 | `gemini_aistudio_nil_account_test.go` | +28/-0 | Geminiaistudionil账号（测试） | ☐ |
| P2 | `gemini_error_policy_test.go` | +28/-3 | Gemini错误策略（测试） | ☐ |
| P2 | `gemini_messages_nil_account_test.go` | +42/-0 | Geminimessagesnil账号（测试） | ☐ |
| P2 | `gemini_multiplatform_test.go` | +154/-11 | Gemini多平台（测试） | ☐ |
| P2 | `gemini_oauth_service_nil_config_test.go` | +43/-0 | GeminiOAuth服务nil配置（测试） | ☐ |
| P2 | `gemini_oauth_service_nil_dependency_test.go` | +84/-0 | GeminiOAuth服务nildependency（测试） | ☐ |

## 17. Antigravity 网关（Antigravity）（建议批次 批次6，整体重要度 P1）

Antigravity relay 网关、OAuth、quota、credits/overages、smart retry、内部 500 惩罚。personal-dev 把 gateway_claude/gemini/retry/streaming/upstream 合并重写进单一 antigravity_gateway_service.go（删除上游 5 文件），重写比例高，属结构性替换。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `antigravity_credits_overages.go` `antigravity_credits_overages_test.go` | +32/-0 | Antigravitycredits超额（含 1 个测试文件） | ☐ |
| P1 | `antigravity_gateway_claude.go` | +0/-757 | Antigravity网关Claude（删除上游文件） | ☐ |
| P1 | `antigravity_gateway_compat.go` `antigravity_gateway_compat_test.go` | +23/-0 | Antigravity网关兼容（含 1 个测试文件） | ☐ |
| P1 | `antigravity_gateway_gemini.go` | +0/-554 | Antigravity网关Gemini（删除上游文件） | ☐ |
| P1 | `antigravity_gateway_retry.go` | +0/-1279 | Antigravity网关重试（删除上游文件） | ☐ |
| P1 | `antigravity_gateway_service.go` `antigravity_gateway_service_test.go` | +4421/-21 | Antigravity网关服务（含 1 个测试文件） | ☐ |
| P1 | `antigravity_gateway_streaming.go` | +0/-1227 | Antigravity网关流式（删除上游文件） | ☐ |
| P1 | `antigravity_gateway_upstream.go` | +0/-382 | Antigravity网关上游（删除上游文件） | ☐ |
| P1 | `antigravity_internal500_penalty.go` `antigravity_internal500_penalty_test.go` | +31/-4 | Antigravityinternal500惩罚（含 1 个测试文件） | ☐ |
| P1 | `antigravity_oauth_service.go` `antigravity_oauth_service_test.go` | +53/-36 | AntigravityOAuth服务（含 1 个测试文件） | ☐ |
| P1 | `antigravity_quota_fetcher.go` `antigravity_quota_fetcher_test.go` | +63/-0 | Antigravityquotafetcher（含 1 个测试文件） | ☐ |
| P1 | `antigravity_token_provider.go` `antigravity_token_provider_test.go` | +3/-3 | AntigravityTokenProvider（含 1 个测试文件） | ☐ |
| P1 | `antigravity_token_refresher.go` | +1/-9 | AntigravityToken刷新器 | ☐ |
| P1 | `gateway_service_antigravity_whitelist_test.go` | +19/-0 | 网关服务Antigravity白名单（测试） | ☐ |
| P2 | `antigravity_default_test_stubs_test.go` | +5/-0 | Antigravitydefaultstubs（测试） | ☐ |
| P2 | `antigravity_gateway_nil_account_test.go` | +57/-0 | Antigravity网关nil账号（测试） | ☐ |
| P2 | `antigravity_oauth_service_nil_proxy_test.go` | +53/-0 | AntigravityOAuth服务nilproxy（测试） | ☐ |
| P2 | `antigravity_rate_limit_test.go` | +50/-1 | Antigravity速率限制（测试） | ☐ |
| P2 | `antigravity_runtime_model_routing_test.go` | +29/-0 | Antigravity运行时模型路由（测试） | ☐ |
| P2 | `antigravity_single_account_retry_test.go` | +23/-0 | Antigravitysingle账号重试（测试） | ☐ |
| P2 | `antigravity_smart_retry_test.go` | +147/-3 | Antigravitysmart重试（测试） | ☐ |

## 18. 支付服务（Payment）（建议批次 批次8（独立、低风险），整体重要度 P1）

支付订单全生命周期、支付配置、履约、退款、webhook、统计、促销码。与网关解耦，独立低风险。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `payment_amounts.go` | +44/-2 | 支付金额 | ☐ |
| P1 | `payment_config_limits.go` `payment_config_limits_test.go` | +10/-0 | 支付配置limits（含 1 个测试文件） | ☐ |
| P1 | `payment_config_plans.go` | +5/-4 | 支付配置套餐 | ☐ |
| P1 | `payment_config_providers.go` `payment_config_providers_test.go` | +41/-28 | 支付配置提供商（含 1 个测试文件） | ☐ |
| P1 | `payment_config_service.go` `payment_config_service_test.go` | +124/-84 | 支付配置服务（含 1 个测试文件） | ☐ |
| P1 | `payment_fulfillment.go` `payment_fulfillment_test.go` | +589/-208 | 支付履约（含 1 个测试文件） | ☐ |
| P1 | `payment_order.go` | +50/-9 | 支付订单 | ☐ |
| P1 | `payment_order_expiry_service.go` `payment_order_expiry_service_test.go` | +24/-16 | 支付订单过期服务（含 1 个测试文件） | ☐ |
| P1 | `payment_order_lifecycle.go` `payment_order_lifecycle_test.go` | +278/-76 | 支付订单生命周期（含 1 个测试文件） | ☐ |
| P1 | `payment_plan_validity.go` | +38/-0 | 支付套餐有效性 | ☐ |
| P1 | `payment_refund.go` `payment_refund_test.go` | +670/-174 | 支付退款（含 1 个测试文件） | ☐ |
| P1 | `payment_resume_lookup.go` `payment_resume_lookup_test.go` | +5/-2 | 支付恢复查找（含 1 个测试文件） | ☐ |
| P1 | `payment_resume_service.go` `payment_resume_service_test.go` | +82/-29 | 支付恢复服务（含 1 个测试文件） | ☐ |
| P1 | `payment_service.go` | +98/-35 | 支付服务 | ☐ |
| P1 | `payment_stats.go` | +17/-1 | 支付统计 | ☐ |
| P1 | `payment_visible_method_instances.go` | +19/-15 | 支付可见方式实例 | ☐ |
| P1 | `payment_webhook_provider.go` `payment_webhook_provider_test.go` | +6/-1 | 支付webhookProvider（含 1 个测试文件） | ☐ |
| P1 | `promo_service.go` `promo_service_test.go` | +9/-5 | 促销码服务（含 1 个测试文件） | ☐ |
| P2 | `payment_config_nil_repo_test.go` | +33/-0 | 支付配置nilrepo（测试） | ☐ |
| P2 | `payment_config_plans_validation_test.go` | +31/-2 | 支付配置套餐validation（测试） | ☐ |
| P2 | `payment_fulfillment_affiliate_retry_test.go` | +553/-0 | 支付履约联盟返利重试（测试） | ☐ |
| P2 | `payment_fulfillment_order_not_found_test.go` | +141/-0 | 支付履约订单notfound（测试） | ☐ |
| P2 | `payment_order_provider_snapshot_test.go` | +14/-3 | 支付订单Provider快照（测试） | ☐ |
| P2 | `payment_order_result_test.go` | +6/-9 | 支付订单result（测试） | ☐ |

## 19. 订阅与兑换码（Subscription-Redeem）（建议批次 批次8（扫尾），整体重要度 P2）

订阅分配/维护队列、兑换码生成与统计。最小模块，扫尾迁移。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P2 | `redeem_service.go` | +111/-24 | 兑换码服务 | ☐ |
| P2 | `redeem_service_batch_update_test.go` | +196/-1 | 兑换码服务batch更新（测试） | ☐ |
| P2 | `redeem_service_redeem_test.go` | +4/-0 | 兑换码服务兑换码（测试） | ☐ |
| P2 | `redeem_service_stats_test.go` | +91/-0 | 兑换码服务统计（测试） | ☐ |
| P2 | `subscription_assign_idempotency_test.go` | +212/-21 | 订阅分配幂等（测试） | ☐ |
| P2 | `subscription_cache_generation_test.go` | +172/-0 | 订阅缓存生成（测试） | ☐ |
| P2 | `subscription_maintenance_queue.go` `subscription_maintenance_queue_test.go` | +2/-2 | 订阅维护队列（含 1 个测试文件） | ☐ |
| P2 | `subscription_monthly_window_test.go` | +1/-1 | 订阅月度窗口（测试） | ☐ |
| P2 | `subscription_service.go` | +180/-81 | 订阅服务 | ☐ |

## 20. 管理端服务（Admin-Management）（建议批次 批次7，整体重要度 P0）

AdminService 巨型服务（账号/分组/用户/代理批量操作）、渠道监控 v2、渠道广场、工单、通知邮件、公告、审计留存、备份、联盟返利、代理池、分组管理。所有模块的汇总点（57 提交/24 模块）。上游 admin_account/group/proxy/user 被合并进 admin_service.go（删除上游 4 文件）。渠道监控可提前，admin_service 本体等对应平台迁移完再补管理入口。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `admin_service.go` | +5508/-154 | 管理端巨型服务（汇总点，57 提交/24 模块）；账号/分组/用户/代理批量操作。 | ☐ |
| P0 | `group.go` `group_test.go` | +334/-112 | 分组实体与领域逻辑（脊柱，13 提交/16 模块）；⚠️合并时须保留上游 profit_control 三字段。（含 1 个测试文件） | ☐ |
| P1 | `admin_account.go` | +0/-1692 | 管理端账号（删除上游文件） | ☐ |
| P1 | `admin_account_duplicate.go` | +259/-0 | 管理端账号去重 | ☐ |
| P1 | `admin_compliance.go` | +1/-1 | 管理端合规 | ☐ |
| P1 | `admin_group.go` | +0/-1259 | 管理端分组（删除上游文件） | ☐ |
| P1 | `admin_group_duplicate.go` `admin_group_duplicate_test.go` | +15/-19 | 管理端分组去重（含 1 个测试文件） | ☐ |
| P1 | `admin_proxy.go` | +0/-608 | 管理端proxy（删除上游文件） | ☐ |
| P1 | `admin_user.go` | +0/-1320 | 管理端用户（删除上游文件） | ☐ |
| P1 | `affiliate_service.go` `affiliate_service_test.go` | +611/-271 | 联盟返利服务（含 1 个测试文件） | ☐ |
| P1 | `announcement.go` | +18/-0 | 公告 | ☐ |
| P1 | `announcement_service.go` `announcement_service_test.go` | +20/-49 | 公告服务（含 1 个测试文件） | ☐ |
| P1 | `audit_retention_service.go` `audit_retention_service_test.go` | +137/-0 | 审计留存服务（含 1 个测试文件） | ☐ |
| P1 | `backup_record_repository.go` | +56/-0 | 备份记录repository | ☐ |
| P1 | `backup_service.go` `backup_service_test.go` | +383/-153 | 备份服务（含 1 个测试文件） | ☐ |
| P1 | `channel.go` | +34/-24 | 渠道 | ☐ |
| P1 | `channel_available.go` `channel_available_test.go` | +103/-54 | 渠道可用性（含 1 个测试文件） | ☐ |
| P1 | `channel_monitor_aggregator.go` | +12/-6 | 渠道监控聚合器 | ☐ |
| P1 | `channel_monitor_checker.go` | +181/-154 | 渠道监控检查器 | ☐ |
| P1 | `channel_monitor_const.go` | +42/-17 | 渠道监控常量 | ☐ |
| P1 | `channel_monitor_probe.go` `channel_monitor_probe_test.go` | +220/-0 | 渠道监控探测（含 1 个测试文件） | ☐ |
| P1 | `channel_monitor_runner.go` `channel_monitor_runner_test.go` | +228/-32 | 渠道监控执行器（含 1 个测试文件） | ☐ |
| P1 | `channel_monitor_service.go` | +99/-2 | 渠道监控服务 | ☐ |
| P1 | `channel_monitor_ssrf.go` `channel_monitor_ssrf_test.go` | +4/-42 | 渠道监控SSRF（含 1 个测试文件） | ☐ |
| P1 | `channel_monitor_template_service.go` | +10/-2 | 渠道监控模板服务 | ☐ |
| P1 | `channel_monitor_template_types.go` | +1/-1 | 渠道监控模板类型 | ☐ |
| P1 | `channel_monitor_types.go` | +15/-0 | 渠道监控类型 | ☐ |
| P1 | `channel_monitor_v2.go` `channel_monitor_v2_test.go` | +41/-217 | 渠道监控v2（含 1 个测试文件） | ☐ |
| P1 | `channel_monitor_v2_aggregator.go` `channel_monitor_v2_aggregator_test.go` | +161/-253 | 渠道监控v2聚合器（含 1 个测试文件） | ☐ |
| P1 | `channel_monitor_validate.go` | +57/-0 | 渠道监控校验 | ☐ |
| P1 | `channel_service.go` `channel_service_test.go` | +17/-0 | 渠道服务（含 1 个测试文件） | ☐ |
| P1 | `email_queue_service.go` `email_queue_service_test.go` | +93/-31 | 邮件队列服务（含 1 个测试文件） | ☐ |
| P1 | `email_service.go` | +79/-6 | 邮件服务 | ☐ |
| P1 | `group_service.go` | +82/-11 | 分组服务 | ☐ |
| P1 | `notification_email_service.go` `notification_email_service_test.go` | +189/-4 | 通知邮件服务（含 1 个测试文件） | ☐ |
| P1 | `proxy_service.go` `proxy_service_test.go` | +106/-15 | proxy服务（含 1 个测试文件） | ☐ |
| P1 | `ticket.go` | +206/-0 | 工单 | ☐ |
| P1 | `ticket_service.go` `ticket_service_test.go` | +672/-0 | 工单服务（含 1 个测试文件） | ☐ |
| P2 | `admin_account_concurrency_test.go` | +41/-7 | 管理端账号并发（测试） | ☐ |
| P2 | `admin_account_upstream_billing_probe_test.go` | +30/-4 | 管理端账号上游计费探测（测试） | ☐ |
| P2 | `admin_service_account_credentials_test.go` | +369/-0 | 管理端服务账号凭证（测试） | ☐ |
| P2 | `admin_service_account_model_defaults_test.go` | +299/-0 | 管理端服务账号模型默认（测试） | ☐ |
| P2 | `admin_service_apikey_test.go` | +6/-0 | 管理端服务APIKey（测试） | ☐ |
| P2 | `admin_service_auth_identity_binding_test.go` | +31/-2 | 管理端服务认证身份绑定（测试） | ☐ |
| P2 | `admin_service_bulk_update_test.go` | +348/-46 | 管理端服务批量更新（测试） | ☐ |
| P2 | `admin_service_clear_error_test.go` | +13/-6 | 管理端服务清空错误（测试） | ☐ |
| P2 | `admin_service_create_user_test.go` | +10/-0 | 管理端服务创建用户（测试） | ☐ |
| P2 | `admin_service_delete_test.go` | +282/-28 | 管理端服务删除（测试） | ☐ |
| P2 | `admin_service_email_identity_sync_test.go` | +5/-1 | 管理端服务邮件身份同步（测试） | ☐ |
| P2 | `admin_service_get_deleted_test.go` | +20/-1 | 管理端服务get已删除（测试） | ☐ |
| P2 | `admin_service_group_rate_test.go` | +16/-0 | 管理端服务分组速率（测试） | ☐ |
| P2 | `admin_service_group_stats_test.go` | +23/-0 | 管理端服务分组统计（测试） | ☐ |
| P2 | `admin_service_group_test.go` | +281/-558 | 管理端服务分组（测试） | ☐ |
| P2 | `admin_service_list_users_test.go` | +177/-15 | 管理端服务列表用户（测试） | ☐ |
| P2 | `admin_service_privacy_nil_test.go` | +21/-0 | 管理端服务隐私nil（测试） | ☐ |
| P2 | `admin_service_proxy_update_test.go` | +59/-0 | 管理端服务proxy更新（测试） | ☐ |
| P2 | `admin_service_redeem_generate_test.go` | +189/-0 | 管理端服务兑换码generate（测试） | ☐ |
| P2 | `admin_service_refresh_credentials_test.go` | +151/-0 | 管理端服务刷新凭证（测试） | ☐ |
| P2 | `admin_service_search_test.go` | +4/-0 | 管理端服务搜索（测试） | ☐ |
| P2 | `admin_service_spark_shadow_test.go` | +93/-0 | 管理端服务spark影子（测试） | ☐ |
| P2 | `admin_service_update_balance_test.go` | +7/-2 | 管理端服务更新余额（测试） | ☐ |
| P2 | `admin_service_update_user_rpm_test.go` | +53/-1 | 管理端服务更新用户RPM（测试） | ☐ |
| P2 | `backup_service_nil_repo_test.go` | +101/-0 | 备份服务nilrepo（测试） | ☐ |
| P2 | `channel_monitor_checker_body_test.go` | +287/-117 | 渠道监控检查器body（测试） | ☐ |
| P2 | `channel_monitor_probe_retirement_test.go` | +62/-37 | 渠道监控探测retirement（测试） | ☐ |
| P2 | `channel_monitor_service_template_test.go` | +608/-0 | 渠道监控服务模板（测试） | ☐ |
| P2 | `channel_plaza_test.go` | +6/-0 | 渠道广场（测试） | ☐ |
| P2 | `channel_test.go` | +0/-1 | 渠道（测试）（删除上游文件） | ☐ |
| P2 | `email_service_nil_cache_test.go` | +72/-0 | 邮件服务nil缓存（测试） | ☐ |
| P2 | `email_service_nil_repo_test.go` | +31/-0 | 邮件服务nilrepo（测试） | ☐ |
| P2 | `notification_email_service_nil_repo_test.go` | +110/-0 | 通知邮件服务nilrepo（测试） | ☐ |

## 21. 运维监控与系统日志（Ops-Monitoring）（建议批次 批次9，整体重要度 P2）

运维观测、错误日志过滤、上游失败落库、健康分、系统日志 sink、指标采集、仪表盘。纯可观测性，不阻塞核心链路。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P2 | `dashboard_service.go` `dashboard_service_test.go` | +115/-52 | 仪表盘服务（含 1 个测试文件） | ☐ |
| P2 | `ops_account_availability.go` | +1/-1 | 运维账号availability | ☐ |
| P2 | `ops_cleanup_executor.go` | +3/-1 | 运维清理executor | ☐ |
| P2 | `ops_cleanup_service.go` | +1/-0 | 运维清理服务 | ☐ |
| P2 | `ops_error_log_filter.go` | +104/-0 | 运维错误日志过滤 | ☐ |
| P2 | `ops_health_score_test.go` | +2/-2 | 运维健康分数（测试） | ☐ |
| P2 | `ops_ingress_reject.go` `ops_ingress_reject_test.go` | +5/-1 | 运维入口reject（含 1 个测试文件） | ☐ |
| P2 | `ops_metrics_collector_test.go` | +1/-1 | 运维指标采集器（测试） | ☐ |
| P2 | `ops_models.go` | +114/-9 | 运维models | ☐ |
| P2 | `ops_monitoring_nil_service_test.go` | +27/-0 | 运维monitoringnil服务（测试） | ☐ |
| P2 | `ops_port.go` | +68/-2 | 运维端口 | ☐ |
| P2 | `ops_repo_mock_test.go` | +51/-7 | 运维repomock（测试） | ☐ |
| P2 | `ops_request_details.go` | +5/-3 | 运维请求details | ☐ |
| P2 | `ops_retry.go` | +854/-0 | 运维重试 | ☐ |
| P2 | `ops_retry_context_test.go` | +79/-0 | 运维重试上下文（测试） | ☐ |
| P2 | `ops_retry_thinking_test.go` | +41/-0 | 运维重试thinking（测试） | ☐ |
| P2 | `ops_service.go` | +258/-76 | 运维服务 | ☐ |
| P2 | `ops_service_batch_test.go` | +33/-35 | 运维服务batch（测试） | ☐ |
| P2 | `ops_service_redaction_test.go` | +57/-0 | 运维服务redaction（测试） | ☐ |
| P2 | `ops_service_user_error_test.go` | +74/-0 | 运维服务用户错误（测试） | ☐ |
| P2 | `ops_settings.go` | +37/-4 | 运维设置 | ☐ |
| P2 | `ops_settings_advanced_test.go` | +36/-12 | 运维设置高级（测试） | ☐ |
| P2 | `ops_settings_models.go` | +15/-13 | 运维设置models | ☐ |
| P2 | `ops_system_log_service.go` `ops_system_log_service_test.go` | +31/-8 | 运维system日志服务（含 1 个测试文件） | ☐ |
| P2 | `ops_system_log_sink.go` `ops_system_log_sink_test.go` | +4/-0 | 运维system日志落库（含 1 个测试文件） | ☐ |
| P2 | `ops_trend_models.go` | +20/-8 | 运维trendmodels | ☐ |
| P2 | `ops_upstream_context.go` | +121/-10 | 运维上游上下文 | ☐ |
| P2 | `ops_upstream_failure_sink.go` `ops_upstream_failure_sink_test.go` | +242/-0 | 运维上游失败落库（含 1 个测试文件） | ☐ |
| P2 | `ops_user_error.go` `ops_user_error_test.go` | +15/-13 | 运维用户错误（含 1 个测试文件） | ☐ |
| P2 | `ops_user_error_cyber_test.go` | +2/-1 | 运维用户错误cyber（测试） | ☐ |

## 22. AI Studio / AI Skill 创作工作台（AI-Studio-Skill）（建议批次 批次3（零耦合、最先整体搬运），整体重要度 P1）

完全独立新子系统——AI 创作工作室（多模态/存储计费/扣费告警）、AI Skill 技能市场（运行时网关/结算/审核）、AI Center、资产生命周期、媒体接入、对象存储、视频生产管线（ffmpeg）。全部新增、零修改上游，仅 wire/admin_service/billing 有浅层集成点。风险极低，可最先整体搬运。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `ai_asset_lifecycle.go` `ai_asset_lifecycle_test.go` | +426/-0 | ai资产生命周期（含 1 个测试文件） | ☐ |
| P1 | `ai_asset_lifecycle_errors.go` | +8/-0 | ai资产生命周期errors | ☐ |
| P1 | `ai_center.go` | +357/-0 | ai中心 | ☐ |
| P1 | `ai_center_service.go` `ai_center_service_test.go` | +1354/-0 | ai中心服务（含 1 个测试文件） | ☐ |
| P1 | `ai_skill.go` | +914/-0 | ai技能 | ☐ |
| P1 | `ai_skill_domain_service.go` | +121/-0 | ai技能领域服务 | ☐ |
| P1 | `ai_skill_review_service.go` | +141/-0 | ai技能审核服务 | ☐ |
| P1 | `ai_skill_run_service.go` | +745/-0 | ai技能运行服务 | ☐ |
| P1 | `ai_skill_runtime_gateway.go` `ai_skill_runtime_gateway_test.go` | +1720/-0 | ai技能运行时网关（含 1 个测试文件） | ☐ |
| P1 | `ai_skill_service.go` `ai_skill_service_test.go` | +318/-0 | ai技能服务（含 1 个测试文件） | ☐ |
| P1 | `ai_skill_settlement_service.go` | +546/-0 | ai技能结算服务 | ☐ |
| P1 | `ai_studio.go` | +31/-0 | ai工作室 | ☐ |
| P1 | `ai_studio_modality.go` `ai_studio_modality_test.go` | +360/-0 | ai工作室模态（含 1 个测试文件） | ☐ |
| P1 | `ai_studio_storage_billing.go` `ai_studio_storage_billing_test.go` | +313/-0 | ai工作室存储计费（含 1 个测试文件） | ☐ |
| P1 | `ai_studio_storage_billing_service.go` | +705/-0 | ai工作室存储计费服务 | ☐ |
| P1 | `ai_studio_storage_debit_alerter.go` `ai_studio_storage_debit_alerter_test.go` | +222/-0 | ai工作室存储扣费告警器（含 1 个测试文件） | ☐ |
| P1 | `media.go` | +133/-0 | 媒体 | ☐ |
| P1 | `media_ingest.go` `media_ingest_test.go` | +336/-0 | 媒体接入（含 1 个测试文件） | ☐ |
| P1 | `media_request_base_url.go` | +34/-0 | 媒体请求基础URL | ☐ |
| P1 | `media_service.go` | +1020/-0 | 媒体服务 | ☐ |
| P1 | `object_storage_config.go` `object_storage_config_test.go` | +653/-0 | 对象存储配置（含 1 个测试文件） | ☐ |
| P1 | `studio_production.go` `studio_production_test.go` | +2173/-0 | 工作室生产（含 1 个测试文件） | ☐ |
| P1 | `studio_production_ffmpeg.go` `studio_production_ffmpeg_test.go` | +159/-0 | 工作室生产ffmpeg（含 1 个测试文件） | ☐ |
| P2 | `ai_skill_balance_charger_test.go` | +71/-0 | ai技能余额扣费（测试） | ☐ |
| P2 | `ai_skill_run_service_billing_test.go` | +169/-0 | ai技能运行服务计费（测试） | ☐ |
| P2 | `ai_skill_run_service_runtime_test.go` | +673/-0 | ai技能运行服务运行时（测试） | ☐ |
| P2 | `ai_studio_nil_repo_test.go` | +18/-0 | ai工作室nilrepo（测试） | ☐ |
| P2 | `skill_billing_api_key_repo_stub_test.go` | +39/-0 | 技能计费APIKeyrepostub（测试） | ☐ |
| P2 | `testdata/skills/script_node_echo/README.md` `testdata/skills/script_node_echo/main.mjs` `testdata/skills/script_node_echo/skill.yaml` | +66/-0 | AI Skill 运行时测试夹具（script_node_echo）；技能脚本样例。 | ☐ |
| P2 | `testdata/skills/script_python_echo/README.md` `testdata/skills/script_python_echo/main.py` `testdata/skills/script_python_echo/skill.yaml` | +73/-0 | AI Skill 运行时测试夹具（script_python_echo）；技能脚本样例。 | ☐ |

## 23. 依赖注入装配（Wire-DI）（建议批次 每批顺带，整体重要度 P0）

wire.go/wire_test.go 服务层 DI 装配。不单独排期，每批迁移顺带追加注册，批后跑 go generate ./cmd/server 校验 wire 图。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `wire.go` `wire_test.go` | +962/-169 | 服务层 DI 装配（脊柱，54 提交/24 模块）；每批迁移顺带追加注册。（含 1 个测试文件） | ☐ |

## 完整性核对

核对方法：临时脚本 `/tmp/check_service_doc.py` 从本文档提取所有反引号包裹、以 `backend/internal/service/` 为前缀补全后的服务层文件路径，与 `numstat-A-service.txt` 的 980 条路径做双向 diff。

核对结果：**覆盖 980/980，零遗漏零多余**（`numstat` 980 条路径 ⇄ 文档文件栏提取 980 条路径双向 diff 均为空）。

各重要度文件数：P0 = 86，P1 = 532，P2 = 361，P3 = 1，合计 980。

各模块文件数：Account-Core 24、Auth-ApiKey 44、Security-Fingerprint 27、Setting-Config 31、Scheduling-TempUnsched 51、Capacity-Forecast 14、Billing-Usage 50、Content-Moderation 9、Claude-Anthropic-Gateway 87、OpenAI-OAuth-Scheduling 35、OpenAI-Gateway-Core 183、OpenAI-WS 50、OpenAI-Images 29、Grok 40、Kiro-Bedrock 41、Gemini 21、Antigravity 28、Payment 35、Subscription-Redeem 10、Admin-Management 88、Ops-Monitoring 36、AI-Studio-Skill 45、Wire-DI 2，合计 980。

