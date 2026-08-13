# 功能点清单：TLS指纹 / 内容审核 / 容量预测 / 渠道监控v2 / 运维监控 / 备份 / 认证增强

> 对比基准：旧内容 = `upstream/main`；新内容 = commit `5806f923bdf47619b25fed4a69220b16e181164c`。
> 覆盖范围：`cap_files.json` 中 id 21（TLS 指纹伪装）、22（内容审核）、23（容量预测）、24（渠道监控 v2）、25（运维监控/系统日志/IP安全）、26（备份系统）、27（认证增强），共约 187 个文件。

---

## 能力：TLS 指纹伪装（对应 cap id 21）

32 个文件，全部为个人 fork 新增（`upstream/main` 中不存在）。新增子系统给每个上游账号配置 TLS/HTTP2 指纹伪装策略，让出站请求的 JA3/JA4、密码套件、扩展、ALPN、ClientHello 结构模拟真实客户端（如 Claude Desktop、Grok CLI、Kiro IDE），而不是 Go 默认的指纹，用来绕过上游的机器人检测。

### 功能点 1：按入站 User-Agent 推断客户端并路由到伪装画像
- 涉及文件：`backend/internal/service/tls_fingerprint_router_service.go`、`backend/internal/handler/admin/tls_fingerprint_router_handler.go`、`backend/internal/repository/tls_fingerprint_router_repo.go`、`backend/internal/repository/tls_fingerprint_router_cache.go`、`backend/internal/model/tls_fingerprint_router.go`、`backend/internal/service/account_tls_fingerprint_resolve.go`、`backend/internal/service/tls_fingerprint_request_context.go`、`backend/internal/service/anti_ban_platforms.go`
- 描述：路由组件解析入站请求的 User-Agent，推断操作系统（Windows/macOS/Linux 等）和客户端类型（如 Claude Desktop、Grok CLI、Kiro IDE），再按这两个维度查找对应的伪装指纹画像（Profile），并把解析结果通过请求上下文（`tls_fingerprint_request_context.go`）传递给下游实际发起出站连接的传输层。路由规则、解析结果均带缓存层减少重复解析开销。

### 功能点 2：账号级指纹策略配置（模式/回退/严格程度）
- 涉及文件：`backend/internal/model/account_tls_fingerprint_policy.go`、`backend/ent/schema/account_tls_fingerprint_policy.go`、`backend/internal/repository/account_tls_fingerprint_policy_repo.go`
- 描述：管理员可以为每个上游账号单独配置指纹伪装策略：使用哪种模式（如强制使用某画像/自动路由/关闭伪装）、当找不到匹配画像时的回退策略（回退到默认 Go/Node.js 指纹还是拒绝）、以及严格程度（是否要求画像的客户端版本范围与实际请求严格匹配）。

### 功能点 3：从真实抓包 ClientHello 导入指纹画像
- 涉及文件：`backend/internal/service/tls_fingerprint_profile_import.go`、`backend/internal/pkg/tlsfingerprint/parser/client_hello_parser.go`、`backend/internal/pkg/tlsfingerprint/parser/observed_client_hello.go`、`backend/internal/pkg/tlsfingerprint/parser/ja3_ja4.go`
- 描述：支持把一段真实抓包得到的原始 ClientHello 字节直接解析并导入为一个新的指纹画像，相当于"克隆"一个真实客户端（如某个特定版本的 Claude Desktop）的完整 TLS 握手行为，同时计算出该画像对应的 JA3/JA4 指纹值用于后续核对。

### 功能点 4：指纹画像管理后台（OS × 客户端类型 × 传输方式维度）
- 涉及文件：`backend/internal/service/tls_fingerprint_profile_service.go`、`backend/internal/repository/tls_fingerprint_profile_repo.go`、`backend/internal/handler/admin/tls_fingerprint_profile_handler.go`、`backend/internal/model/tls_fingerprint_profile.go`
- 描述：后台按「操作系统 × 客户端类型 × 传输方式（H1/H2）」三个维度管理指纹画像列表，每个画像记录校验状态（是否已验证有效）与校验时间戳，并支持声明该画像适用的客户端版本范围（version range），避免用旧版本客户端的指纹去冒充新版本。

### 功能点 5：运行时命中/回退统计
- 涉及文件：`backend/internal/service/tls_fingerprint_usage.go`
- 描述：运行时统计某个指纹画像被命中使用的请求数，以及回退到默认指纹（Node.js 默认签名）的请求数，两者的比例在管理端可见，用于评估伪装覆盖率/画像库是否需要补充。

### 功能点 6：底层 TLS/HTTP2 指纹重放传输层
- 涉及文件：`backend/internal/pkg/tlsfingerprint/dialer.go`、`backend/internal/pkg/tlsfingerprint/http2/fingerprint.go`、`backend/internal/pkg/tlsfingerprint/transport/types.go`、`backend/internal/pkg/tlsfingerprint/replay/replay_profile.go`、`backend/internal/pkg/tlsfingerprint/replay/replay_projection.go`、`backend/internal/service/fingerprint_normalizer.go`、`backend/internal/model/http_header_template.go`、`backend/internal/repository/http_upstream_h1_replay.go`、`backend/internal/repository/http_upstream_h2_fingerprint_replay.go`
- 描述：自定义的 TLS dialer 和 HTTP/2 帧层实现，按选定画像重放密码套件顺序、TLS 扩展顺序、ALPN 协商、HTTP/2 SETTINGS 帧参数与头部顺序等底层细节，这些是让出站流量在包级别就与真实客户端一致的核心实现，同时管理 H1/H2 两种传输路径各自的重放配置与 HTTP 头部模板。

### 功能点 7：平台专属指纹适配（Kiro / OpenAI）
- 涉及文件：`backend/internal/service/kiro_tls_profile.go`、`backend/internal/service/openai_fingerprint_body.go`、`backend/internal/service/openai_tls_fingerprint_router.go`
- 描述：针对 Kiro（IDE 客户端伪装）和 OpenAI（请求体字段与指纹路由的联动）分别做了平台专属的适配逻辑，因为不同上游平台对指纹伪装的容忍度和检测点不同。

> 说明：`backend/internal/server/middleware/ip_security.go` 在能力 21 的文件清单里出现是文件归属重复——该文件的真正实现属于能力 25（见下方"IP 多账号安全"）,与 TLS 指纹伪装本身无关。

---

## 能力：内容审核（对应 cap id 22）

8 个文件，全部是"修改"而非"新增"——说明内容审核本身是 `upstream` 已有的大型功能，个人 fork 是在其基础上做增强。涉及文件：`backend/internal/handler/admin/content_moderation_handler.go`、`backend/internal/handler/content_moderation_helper.go`、`backend/internal/repository/content_moderation_hash_cache.go`、`backend/internal/repository/content_moderation_repo.go`、`backend/internal/service/content_moderation.go`、`backend/internal/service/content_moderation_email.go`、`backend/internal/service/content_moderation_input.go`、`backend/internal/service/content_moderation_keyword_matcher.go`。

### 功能点 1：扫描范围扩展为全历史消息 + 多模态输入
- 涉及文件：`content_moderation.go`、`content_moderation_input.go`
- 描述：审核扫描范围从原来"只看最新一条消息"扩展为扫描所有历史消息，且支持多模态（embeddings）输入的审核。

### 功能点 2：反绕过加固（XML 标签剥离 / 长度上限 / Unicode 归一化）
- 涉及文件：`content_moderation_input.go`
- 描述：通用剥离输入中的 XML 标签防止利用标签嵌套绕过关键词检测；对输入长度设置上限以控制审核服务商 API 调用成本；对输入做 Unicode 归一化，用于捕获同形字符（如全角/特殊 Unicode 变体字符）伪装绕过关键词匹配的攻击。

### 功能点 3：关键词匹配算法升级为类 Aho-Corasick 自动机
- 涉及文件：`content_moderation_keyword_matcher.go`
- 描述：关键词匹配从原先的逐词/正则式匹配升级为类 Aho-Corasick 的自动机算法，单次扫描可同时匹配所有关键词，提升大关键词库场景下的匹配性能。

### 功能点 4：审核服务商 API Key 多账号轮换 + 自身限流
- 涉及文件：`content_moderation.go`
- 描述：审核服务商（如第三方内容审核 API）的 API Key 支持配置多个账号做轮换调用；同时对审核服务商自身的 API Key 调用加了限流保护，用 Lua 脚本实现 RPM/RPD/TPM 三维限流（每分钟请求数/每日请求数/每分钟 Token 数），避免审核调用量过大导致被服务商限流或产生额外费用。

### 功能点 5：分组级限流豁免 + 用户级封号豁免名单
- 涉及文件：`content_moderation.go`
- 描述：新增分组级别的限流豁免——某些分组的审核请求可以不受上述审核限流约束；新增用户级别的自动封号豁免名单，按用户 ID 或邮箱配置白名单，白名单内用户即使触发审核规则也不会被自动封号。

### 功能点 6："临界值/未达封禁阈值"记录机制（near miss）
- 涉及文件：`content_moderation.go`、`content_moderation_repo.go`
- 描述：新增对"命中审核规则但未达到封禁阈值"内容的记录机制，这类内容不会触发封号，但会被记录下来供人工复查（near miss 日志），用于发现规则边界是否需要调整。

### 功能点 7：命中缓存重新设计（按 hash 存储 + 90 天 TTL + 分页管理页）
- 涉及文件：`content_moderation_hash_cache.go`、`content_moderation_repo.go`、`content_moderation_handler.go`
- 描述：命中缓存从原来单个无界集合重新设计为按内容 hash 单独存储、90 天 TTL 自动过期，并增加按时间的有序集合索引方便按时间范围查询；缓存元数据更丰富（内容摘录、触发动作、命中分类、命中关键词、涉及模型、所属分组、用户邮箱），并支持批量删除和分页列表查询，对应管理端一个新的"已标记内容"管理页面。

### 功能点 8：移除 ProxyID 字段
- 涉及文件：`content_moderation.go`、`content_moderation_repo.go`
- 描述：审核记录中移除了 ProxyID 字段，属于数据模型简化。

### 功能点 9：关键词邻近窗口容忍度配置
- 涉及文件：`content_moderation_keyword_matcher.go`
- 描述：关键词匹配支持"临近窗口"容忍度配置（可调的邻近距离参数），允许关键词片段之间存在一定距离仍判定为命中，同时减少因过度严格匹配导致的误报。

### 功能点 10：输入摘录存储的隐私开关
- 涉及文件：`content_moderation_handler.go`、`content_moderation.go`
- 描述：管理员可以配置系统是否存储触发审核的输入内容摘录，关闭后只记录命中元数据（分类/关键词/动作等）不保留原文，用于隐私合规场景。

---

## 能力：容量预测（对应 cap id 23）

13 个文件，全部新增。预测引擎基于历史用量日志（按小时/平台聚合），预测账号在配额周期内还能撑多久（**USD 计价，不是并发/RPM**）。

### 功能点 1：加权消费速度预测算法
- 涉及文件：`backend/internal/service/capacity_forecast_pure.go`、`backend/internal/service/capacity_forecast_types.go`
- 描述：用"昨天"和"前天"两天的历史消费模式作为权重基础，结合"今天"已经过去的时间占比，按比例外推出当前的实时消费速度预测值，属于纯函数式的预测算法实现（不含 IO）。

### 功能点 2：配额窗口滚动重置的余额曲线模拟
- 涉及文件：`backend/internal/service/capacity_forecast_service.go`
- 描述：模拟配额窗口滚动重置后的余额曲线，支持组合模拟两类窗口：5 小时短周期窗口（类似 Codex 那种短周期限流窗口）与 7 天/30 天长周期窗口，输出跨窗口的余额变化轨迹。

### 功能点 3：核心预测指标输出
- 涉及文件：`backend/internal/service/capacity_forecast_service.go`、`backend/internal/service/capacity_forecast_types.go`
- 描述：核心输出指标包括：当前可用余额（USD）、预测消费速度、预计何时出现缺口（capacity shortfall）、账号补号建议。

### 功能点 4：多平台配额探测适配层
- 涉及文件：`backend/internal/service/capacity_provider.go`、`backend/internal/service/capacity_provider_common.go`、`backend/internal/service/capacity_provider_anthropic.go`、`backend/internal/service/capacity_provider_antigravity.go`、`backend/internal/service/capacity_provider_gemini.go`、`backend/internal/service/capacity_provider_grok.go`、`backend/internal/service/capacity_provider_kiro.go`、`backend/internal/service/capacity_provider_openai.go`
- 描述：为 Anthropic、Antigravity、Gemini、Grok、Kiro、OpenAI 六个平台各自实现一个 provider 适配器，负责按各平台特有的方式主动探测该账号在上游的真实配额/余额情况，为预测引擎提供"地面真值"校准数据。

### 功能点 5：管理端容量 API + 定时刷新任务
- 涉及文件：`backend/internal/handler/admin/capacity_handler.go`、`backend/internal/repository/capacity_forecast_repo.go`
- 描述：管理端提供容量时间序列查询接口（查看某账号历史容量趋势）以及主动触发探测上游真实配额的接口；后台有一个每 10 分钟运行一次的定时任务负责刷新所有账号的预测数据。

> **确认结论**：容量预测这个能力里没有"实时并发"数据，全部是基于 USD 和限流窗口利用率的预测，**不涉及"分组按实际并发展示"这个功能点**（该功能点实际在能力 4/6，见文末补充说明）。

---

## 能力：渠道监控 v2（对应 cap id 24）

42 个文件，其中只有 2 个是全新文件（中间件 `channel_monitor_probe.go` 和账号安全事件弹窗组件），其余 40 个都是在 `upstream` 已有的"渠道监控 v2"基础功能上做修改。upstream 已有基础能力包括：90 分钟到 30 天的时间范围筛选、按平台等维度分组、多种健康模式视图、RPM/TPM/错误率/缓存命中率/耗时百分位指标表格、趋势图、错误分布、用户维度分析、健康阈值矩阵热力图。

### 功能点 1：本地探测自应答中间件（避免监控探测消耗真实上游配额）
- 涉及文件：`backend/internal/server/middleware/channel_monitor_probe.go`（全新）
- 描述：渠道监控的健康探测请求带特殊 header 时，该中间件直接在本地短路返回签名验证过的模拟响应，请求根本不会真的打到上游、不消耗真实的配额/Token——这是为了让监控系统自我监控（健康探测）时不浪费真实上游额度。

### 功能点 2：人工调整监控项主模型"7 天可用率"展示值
- 涉及文件：`backend/internal/repository/channel_monitor_v2_repo.go`、`frontend/src/components/admin/monitor/MonitorAvailabilityAdjustDialog.vue`
- 描述：管理端新增一个接口，可以人工调整某个监控项主模型显示的"7 天可用率"百分比，实现方式是直接修改历史记录行——更像是展示层的手动修正工具，不代表真实监控数据被重新计算。

### 功能点 3：v1 管理端接口功能开关网关（灰度控制）
- 涉及文件：`backend/internal/handler/admin/channel_monitor_handler.go`
- 描述：所有 v1 管理端接口新增功能开关（feature flag）网关控制，可以灰度开启新版渠道监控相关能力。

### 功能点 4：新增 Kiro 平台类型接入监控
- 涉及文件：`backend/internal/repository/channel_monitor_v2_aggregation.go`、`frontend/src/components/user/monitor/ProviderIcon.vue`
- 描述：新增 Kiro 作为受监控的平台类型，纳入渠道监控 v2 的聚合统计和图标展示体系。

### 功能点 5：Postgres 咨询锁防止定时检查重复并发执行
- 涉及文件：`backend/internal/repository/channel_monitor_repo.go`
- 描述：用 Postgres 咨询锁（advisory lock）防止同一个监控项的定时健康检查在多实例部署下被并发重复执行，避免重复探测浪费配额或产生冲突写入。

### 功能点 6：数据留存策略简化为统一 35 天窗口
- 涉及文件：`backend/internal/repository/channel_monitor_v2_repo.go`、`backend/internal/repository/channel_monitor_v2_aggregation.go`
- 描述：数据留存策略从此前复杂的"按表分级"规则简化为统一固定 35 天留存窗口，配合更精简的错误分类体系。

### 功能点 7：错误详情接口移除管理员权限网关
- 涉及文件：`backend/internal/handler/security_audit_errors.go`（`GetErrors`）
- 描述：移除了 `includeAdmin` 参数网关，错误详情现在对所有调用方都直接返回（原来会区分调用方是否为管理员来决定返回的详情粒度）。**需要在迁移时确认这是刻意的权限简化还是需要重新加回权限检查**——这是一个待确认的风险点。

### 功能点 8：账号页"网络安全事件"弹窗
- 涉及文件：`frontend/src/components/admin/account/AccountCyberEventsModal.vue`（全新，upstream/main 中不存在）
- 描述：账号管理页面新增"网络安全事件"弹窗，展示该账号触发风控策略（`cyber_policy`）的事件汇总（次数、最近发生时间）+ 分页列表（涉及模型、状态码、关联消息）。对应的后端聚合/分页查询能力（`GetAccountCyberSummaries`/`ListAccountCyberEvents`）实现在能力 25 的 `ops_repo_account_cyber.go`（新增文件）与 `ops_service.go` 中。

> **确认结论**：渠道监控 v2 这个能力也没有"分组按实际并发/RPM 展示"的功能，均为健康探测、可用率、错误分布等既有监控维度的增强，不涉及并发展示（见文末补充说明）。

---

## 能力：运维监控 / 系统日志 / IP 安全（对应 cap id 25）

37 个文件，9 个新增，28 个修改。

### 功能点 1：IP 多账号安全检测与自动封禁（`ip_security.go`）
- 涉及文件：`backend/internal/service/ip_security.go`（全新，760 行）、`backend/internal/repository/ip_security_repo.go`（新增）、`backend/internal/handler/admin/ip_security_handler.go`（新增）、`backend/internal/server/middleware/ip_security.go`
- 描述：这是能力 21 文件清单里 `ip_security.go` 误重复归属的真正实现。跟踪每个用户最多 **256 个**历史使用过的 IP，记录每个 IP 首次使用时间。检测逻辑：同一个 IP 在一个滚动时间窗口（默认 **10 分钟**）内被超过阈值数量（默认 **4 个**）的不同用户账号使用，判定为"共享/异常 IP"。触发后自动在网关中间件层直接封禁该 IP（返回 403），除非该 IP 在白名单里。有初始学习宽限期，避免系统刚上线时因历史数据不足而误封。管理端功能：查看封禁列表、查看触发封禁时的关联账号活动详情、解封并加白名单、管理白名单列表；封禁窗口时长和账号数阈值都在系统设置里可配置。

### 功能点 2：管理员错误重放/重试（`ops_retry.go`）
- 涉及文件：`backend/internal/service/ops_retry.go`（新增）
- 描述：管理员在错误日志详情弹窗里可以对失败请求做"重放"，支持两种模式：`client`（走完整网关/调度器管道重放，允许最多切换 **3 次**账号）或 `upstream`（直接重放到同一个上游账号，不切换）。重放间隔限制为同一条错误最少 **10 秒**一次（`opsRetryMinIntervalPerError`），避免管理员误触连续重放。重放结果会返回响应预览供管理员核对。

### 功能点 3：系统日志新增 host 字段过滤 + Kiro 组件日志统一接入
- 涉及文件：`backend/internal/service/ops_system_log_service.go`、`backend/internal/service/ops_system_log_sink.go`、`backend/internal/pkg/logger/logger.go`、`frontend/src/views/admin/ops/components/OpsSystemLogTable.vue`
- 描述：系统日志支持多实例部署下按主机名（host）筛选日志；同时把 Kiro 相关组件的日志也纳入统一的业务日志 + 审计日志体系。管理端有可搜索的日志面板，支持按 host/组件/关键词过滤。

### 功能点 4：审计/邀请历史数据定时清理
- 涉及文件：`backend/internal/repository/audit_retention_repo.go`（新增）
- 描述：AI 审计日志、Codex 邀请重置历史、已删除 API Key 审计日志的定时清理任务——纯内部存储/性能优化，无直接用户可感知的功能变化。

### 功能点 5：账号"网络安全事件"聚合与分页查询（配合能力 24 的前端弹窗）
- 涉及文件：`backend/internal/repository/ops_repo_account_cyber.go`（新增）、`backend/internal/service/ops_service.go`
- 描述：新增 `GetAccountCyberSummaries`（按账号批量统计触发风控策略事件的次数与最近发生时间）与 `ListAccountCyberEvents`（按账号分页列出具体事件，包含请求 ID、模型、状态码、关联消息）两个能力，为能力 24 中新增的 `AccountCyberEventsModal.vue` 弹窗提供数据支撑。

### 功能点 6：其余运维数据层支撑（无独立新功能点）
- 涉及文件：`ops_service.go`、`ops_repo.go`、`ops_repo_dashboard.go`、`ops_repo_openai_token_stats.go`、`ops_repo_preagg.go`、`ops_repo_request_details.go`、`ops_repo_trends.go`、`ops_account_availability.go`、`ops_cleanup_executor.go`、`ops_cleanup_service.go`、`ops_error_log_filter.go`、`ops_ingress_reject.go`、`ops_models.go`、`ops_port.go`、`ops_request_details.go`、`ops_settings.go`、`ops_settings_models.go`、`ops_trend_models.go`、`ops_upstream_context.go`、`ops_upstream_failure_sink.go`、`ops_user_error.go`、`dashboard_service.go`、`ops_error_logger.go`、`ops_latency.go`、`ops_dashboard_handler.go`、`ops_handler.go`、`server_timing.go` 等
- 描述：主要是配合上述功能点（尤其是重试、host 过滤、cyber 事件、错误分类）的数据层字段扩展和查询逻辑调整，无独立的新功能点。

> **确认结论**：运维监控/系统日志/IP安全这个能力也没有"分组按实际并发/RPM 展示"的功能——该功能点真正在能力 4/6，不在 21-27 范围内（见文末补充说明）。

---

## 能力：备份系统（对应 cap id 26）

8 个文件。备份基础功能是 `upstream` 已有的，个人 fork 做了扩展。

### 功能点 1：多命名 S3 兼容存储 Profile
- 涉及文件：`backend/internal/handler/admin/backup_handler.go`、`frontend/src/api/admin/backup.ts`
- 描述：支持配置多个命名的 S3 兼容存储 Profile，备份存储和媒体存储可以分别使用不同的 Profile，不需要共享同一套对象存储配置。

### 功能点 2：`pg_dump` 排除自身备份表
- 涉及文件：`backend/internal/repository/backup_pg_dumper.go`
- 描述：执行 `pg_dump` 时排除 `backup_records`/`backup_metadata` 表本身，避免备份数据循环嵌套备份自己（备份记录里包含备份记录）。

### 功能点 3：备份记录仓储重新设计（run token + 版本号状态机、孤儿任务恢复、定时计划、旧版元数据导入）
- 涉及文件：`backend/internal/repository/backup_record_repo.go`、`frontend/src/views/admin/BackupView.vue`
- 描述：备份记录用 run token + 版本号做并发安全的状态机，避免同一次备份被并发写坏；启动时自动检测并恢复"孤儿"（因进程异常中断而卡住）的备份任务，将其标记为失败；支持配置定时计划备份；支持导入旧版备份系统留下的元数据，做平滑迁移。

### 功能点 4：前端"IP 安全"标签页（对应能力 25 的 IP 封禁管理 UI）
- 涉及文件：`frontend/src/views/admin/RiskControlView.vue`、`frontend/src/api/admin/ipSecurity.ts`、`frontend/src/api/admin/riskControl.ts`
- 描述：风控中心新增"IP 安全"标签页，对应能力 25 中 IP 多账号安全检测的管理 UI：封禁列表管理、按状态筛选、详情钻取（查看触发封禁时的关联账号活动）、解封/加白名单操作。

---

## 能力：认证增强（对应 cap id 27）

45 个文件，5 个新增，40 个修改——DingTalk/微信 OAuth 等大部分基础设施是 `upstream` 已有的，个人 fork 做的是新增文件 + 细节增强。新增文件：`backend/internal/handler/auth_refresh_cookie.go`（cookie 刷新处理）、`frontend/src/components/auth/TotpRevealDialog.vue` + `frontend/src/composables/useTotpReveal.ts`（TOTP 密钥二次查看功能）、`frontend/src/utils/authSession.ts`（会话工具）、`frontend/src/views/auth/dingtalkEmailCompletionPayload.ts`（钉钉邮箱补全载荷构造）。

### 功能点 1：Refresh Token 存储改为 HttpOnly + Secure + SameSite Cookie
- 涉及文件：`backend/internal/handler/auth_refresh_cookie.go`（新增）、`backend/internal/repository/refresh_token_cache.go`、`backend/internal/server/middleware/jwt_auth.go`、`backend/internal/server/middleware/optional_jwt_auth.go`、`frontend/src/utils/authSession.ts`（新增）、`frontend/src/api/auth.ts`
- 描述：Refresh Token 存储方式从原来存 localStorage 改为 HttpOnly + Secure + SameSite Cookie，防止 XSS 窃取；配套自定义 `X-Sub2API-Refresh` header 做 CSRF 防护，并校验请求 Origin 是否在配置的 CORS 白名单内。

### 功能点 2：TOTP 密钥二次查看功能
- 涉及文件：`frontend/src/components/auth/TotpRevealDialog.vue`（新增）、`frontend/src/composables/useTotpReveal.ts`（新增）、`frontend/src/components/user/profile/TotpSetupModal.vue`、`frontend/src/components/user/profile/TotpDisableDialog.vue`
- 描述：用户设置好 TOTP 后如果想再看一次密钥/二维码，需要重新走一次身份验证才能查看，防止绕过初始设置流程窃取密钥。

### 功能点 3：API Key 明文查看的 TOTP 二次校验
- 涉及文件：`backend/internal/handler/auth_handler.go`、`backend/internal/middleware/rate_limiter.go`
- 描述：查看已保存 API Key 的明文值时，如果账号开启了 TOTP，需要在请求头 `X-TOTP-Code` 里带上 6 位验证码；验证码错误会有内联报错且可重试。

### 功能点 4：DingTalk OAuth 邮箱补全流程
- 涉及文件：`backend/internal/handler/auth_dingtalk_oauth.go`、`backend/internal/handler/auth_oauth_pending_flow.go`、`frontend/src/views/auth/DingTalkCallbackView.vue`、`frontend/src/views/auth/DingTalkEmailCompletionView.vue`、`frontend/src/views/auth/dingtalkEmailCompletionPayload.ts`（新增）
- 描述：钉钉 OAuth 不返回邮箱，登录后跳转到专门的"邮箱补全"页面，要求用户填邮箱 + 密码（可选验证码/邀请码），并可选择是否采用钉钉的昵称/头像作为 Sub2API 资料。

### 功能点 5：OAuth 注册流程细节加固
- 涉及文件：`backend/internal/handler/auth_email_oauth.go`、`backend/internal/repository/email_cache.go`、`backend/internal/repository/user_profile_identity_repo.go`、`frontend/src/utils/registrationEmailPolicy.ts`、`frontend/src/components/user/profile/ProfileAvatarCard.vue`
- 描述：密码最小长度提升到 8 位；"账号占用检测"逻辑改为只检查活跃用户（软删除账号不再阻止身份重新绑定）；头像采用与事务提交同步（失败自动回滚清理，不留孤儿头像文件）。

---

## 补充说明：关于"实时并发/RPM"功能的确认

用户要求确认"容量预测"（能力 23）和"渠道监控 v2"（能力 24）里是否有"分组按实际并发展示"的功能。**结论：没有，这两个能力都不含这个功能。**

真正实现"分组按实际并发展示"这个功能点的文件（`ops_concurrency.go`、`OpsConcurrencyCard.vue`、`UserConcurrencyCell.vue`、`group_capacity_service.go`、`GroupCapacityBadge.vue`）**不在能力 21-27 范围内**：

- `ops_concurrency.go`、`OpsConcurrencyCard.vue`、`UserConcurrencyCell.vue` 三个文件在 `upstream` 和个人分支之间**完全没有差异**——是 upstream 自带功能，不是个人定制点。
- 真正有个人定制的是 `group_capacity_service.go`（属于能力 4）和 `GroupCapacityBadge.vue`（属于能力 6）：分组并发计算从"累加各账号并发槎位"改成了直接查询真实的分组级并发聚合数据（`GetGroupConcurrency`），前端徽章同步做了简化，去掉了 RPM/会话数子徽章，只保留并发徽章 +"当前分组真实并发"提示。

这个功能点已经/会在能力 4、6 对应的报告中体现，本报告（能力 21-27）中的容量预测、渠道监控 v2、运维监控三个能力均**不包含**该功能。

---

## 附：能力对应 cap id 与文件规模

| 能力 | cap id | 文件数 | 新增/修改 | 功能点数 |
|---|---|---|---|---|
| TLS 指纹伪装 | 21 | 32 | 全部新增 | 7 |
| 内容审核 | 22 | 8 | 全部修改 | 10 |
| 容量预测 | 23 | 13 | 全部新增 | 5 |
| 渠道监控 v2 | 24 | 42 | 2 新增 / 40 修改 | 8 |
| 运维监控/系统日志/IP安全 | 25 | 37 | 9 新增 / 28 修改 | 6 |
| 备份系统 | 26 | 8 | 部分新增/修改 | 4 |
| 认证增强 | 27 | 45 | 5 新增 / 40 修改 | 5 |
