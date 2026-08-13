# 功能点清单：计费用量底座 / 调度限流 / 系统设置 / 依赖注入 / CI

> 对应能力编号：cap id 3（计费与用量统计底座）、cap id 4（账号调度/限流/临时下线）、cap id 5（系统设置中心）、cap id 7（依赖注入与公共工具）、cap id 8（CI 与安全扫描）。
> 基准 diff：`upstream/main` → 个人 fork（`personal-dev`），共 111 个文件。

## 能力：计费与用量统计底座（对应 cap id 3）

### 功能点 1：管理员仪表盘新增消费拆分与活跃用户统计
- 涉及文件：`backend/internal/repository/usage_log_repo.go`（新函数 `fillDashboardConsumptionStats`、`fillDashboardPaymentFlowStats`）
- 描述：新增按支付方式分类的实际消费额（余额支付 vs 订阅支付）、充值金额、退款金额，分 today/累计两个维度；新增活跃用户统计（区间内计数 + 实时每小时活跃数）。

### 功能点 2：分组消费统计
- 涉及文件：`usage_log_repo.go`（新函数 `GetGroupUsageSummaryByIDs`）
- 描述：管理端分组管理页面展示每个分组的总消费和当日消费金额。

### 功能点 3：用量明细新增"上游模型不匹配"筛选与"排除管理员用量"选项
- 涉及文件：`usage_log_repo.go`、相关 dto 文件
- 描述：管理员可筛选出"实际使用模型"与"请求模型"不同的记录，用于追踪模型 fallback/映射对计费的影响；新增"排除管理员自身用量"选项，避免管理员测试请求影响统计准确性。

### 功能点 4：套利防护计费逻辑
- 涉及文件：`usage_billing_selection.go`
- 描述：当请求模型被路由映射/fallback到另一个模型时，按两者中价格更高的一方计费，防止用户用便宜模型名请求却拿到更贵的实际服务；用量日志记录命中的实际定价档位。

### 功能点 5：计费模型来源追踪与映射链记录
- 涉及文件：`usage_model_resolution.go`，dto 字段（`UpstreamModel`、`ModelMappingChain`）
- 描述：新增逻辑追踪计费模型来源（渠道映射/原始请求/显式指定），维护候选模型列表确保多层映射下依然选中正确定价；`UpstreamModel` 记录实际命中模型，`ModelMappingChain` 记录完整映射链路，用量明细页面可查看。

### 功能点 6：视频计费新增 4K 档位
- 涉及文件：`video_billing.go`
- 描述：原来只支持 480p/720p/1080p 自动判档，现在扩展支持到 4K。

### 功能点 7：网络搜索/语音计费
- 描述：网络搜索工具调用按千次计费；语音服务支持"按时长"或"按次"两种计费模式可选。

### 功能点 8：多模态图片输入拆分计费
- 涉及文件：`account_stats_pricing.go`
- 描述：输入包含图片时按图片专属费率计费而非文本费率，费率未配置则回退文本费率。

### 功能点 9：模型系列兜底定价表
- 涉及文件：`pricing_service.go`
- 描述：新增兜底价格表，避免价格库缺失时被错误计为 0 费用。

### 功能点 10：发票管理
- 涉及文件：`invoice_service.go`（全新文件）
- 描述：用户可合并多笔订单申请开票（填写公司名/税号），管理员审核并上传发票文件，状态机 APPLIED/ISSUED/CANCELLED。该功能已在能力 18 的报告中详细覆盖，此处仅标注归属，不重复展开。

### 功能点 11：余额缓存最终一致性（Outbox 模式）
- 涉及文件：`balance_cache_outbox.go`、`balance_cache_outbox_repo.go`
- 描述：Outbox 模式保障余额缓存更新，失败自动重试不丢失。

### 功能点 12：每用户每日消费预聚合（纯后台性能底座）
- 涉及文件：`usage_user_daily_cost_aggregator.go`、`usage_user_daily_cost_state.go`
- 描述：按业务日/业务时区周期性全量重算每用户每日消费金额，支撑管理后台海量用户按消费金额排序的查询性能。无用户可感知行为变化。

### 功能点 13：账号池页面按平台差异化的用量/配额展示
- 涉及文件：`account_usage_service.go`（对应"账号池"管理页面）
- 描述：
  - Kiro 账号支持实时配额查询 + 失效凭证自动重试刷新 token；
  - Grok 账号区分"订阅消费限额"与"免费额度"两套估算逻辑（精确数据不可用时按免费档位估算）；
  - Gemini 账号无官方配额数据时从错误响应中提取配额快照兜底展示。

### 功能点 14：API Key 实时并发字段
- 涉及文件：dto 层新增 `CurrentConcurrency` 字段（严格属于能力 7 文件范围，此处说明业务含义）
- 描述：用户自己的 API Key 列表和管理端都能看到每个 Key 当前的实时并发请求数。

### 功能点 15：最低余额储备（minBalanceReserve）机制
- 描述：新增可配置的"最低余额储备"概念（区别于余额=0），余额低于该阈值时触发缓存失效联动，用于余额不足预警和网关限流判断依据。

### ⚠️ 需迁移时确认的疑似退化点
- 用户仪表盘统计里"平台维度用量拆分"看起来被移除了，需要在正式迁移前确认是刻意简化还是遗漏。

### 无新增可感知功能的文件
- `scheduler_cache.go` / `scheduler_outbox_repo.go`：分布式锁（ClaimToken 机制）保证多实例部署下调度缓存重建的一致性。
- `usage_record_worker_pool.go`：过载时内联执行或丢弃任务（带丢弃计数），防止阻塞主请求路径。
- `user_platform_quota_db_aggregator.go`：250ms 窗口内合并多个配额增量请求后一次写库，纯性能优化。

---

## 能力：账号调度 / 限流 / 临时下线（对应 cap id 4）

### 功能点 1：模型不支持时返回可用模型列表
- 涉及文件：`group_model_unsupported.go`（全新文件）
- 描述：用户请求的模型不被分组内任何账号支持时，API 返回该分组实际支持的模型列表，而非笼统报错。

### 功能点 2：模型 Fallback
- 涉及文件：`model_fallback.go`（全新文件）
- 描述：上游返回 400/404"模型不存在"错误时，自动切换到管理员配置的备用模型重试；全局开关 + 按平台单独配置备用模型（管理端配置面见能力 5）。

### 功能点 3：平台级默认模型映射
- 涉及文件：`platform_model_routing.go`（全新文件）
- 描述：平台级默认模型映射与白名单配置，作为账号自己没配置时的兜底，包含精简版模型变体和订阅特定覆盖。

### 功能点 4：Responses API 续接失败自动重试
- 涉及文件：`recoverable_failure_policy.go`（全新文件）
- 描述：OpenAI Responses API continuation 失败（如"previous response not found"、无效 anchor 错误）时，网关自动用兜底策略重试。

### 功能点 5：分组按实际并发展示（用户特别点名确认）
- 涉及文件：`group_capacity_service.go`
- 描述：分组并发计算从"累加各账号 Redis 并发槎位求和"改为直接调用 `GetGroupConcurrency` / `GetGroupConcurrencyBatch` 查询真实的分组级并发聚合计数器。**已确认存在**——这正是分组容量按实际并发（而非各账号理论上限相加）展示的具体实现（详见文末复述）。

### 功能点 6：限流服务多项具体新规则
- 涉及文件：`ratelimit_service.go`（核心限流服务，约 1346 行）
- 描述：
  - OpenAI 图片生成路由独立于文本路由的 429 冷却机制，输入图片按分钟/按天独立限额跟踪；
  - Kiro 的 429 响应从多个可能的 JSON 路径精确解析限额重置时间；
  - Grok 区分"消费限额"和"免费额度耗尽"两种限流状态，各自独立的冷却与重置时间解析逻辑；
  - API Key 池账号检测"永久计费封禁"错误码/消息后立即标记不可用（区别于临时冷却）；
  - Gemini Code Assist 区分"需要人工验证的 403"（带验证链接，走临时冷却）与普通 403（更严重处理）；
  - **阈值化临时下线机制（本能力最核心的新功能）**：不再是命中一次限流/超时就立即下线账号，而是要求在可配置的时间窗口内累计命中 N 次才真正触发临时下线；此阈值（启用开关、命中次数、窗口分钟数）可以在管理后台针对每条规则单独配置，全局生效；支持仅按错误码匹配（不要求关键词）；流式超时下线场景同样支持这套阈值计数机制；触发时把命中次数详情结构化记录进临时下线状态，运维在错误信息里能直接看到触发次数。

### 功能点 7：调度即时可见性
- 涉及文件：`scheduler_snapshot_service.go`（`UpdateLastUsedInCache`）
- 描述：让调度系统立刻看到最新的账号选择记录，不用等数据库同步完成，直接影响账号轮转/负载均衡的实时公平性（无独立 UI，属于底层机制增强）。

### 功能点 8：定时测试任务优化
- 涉及文件：`scheduled_test_runner_service.go`
- 描述：自动跳过临时下线/禁用/过期账号，不再浪费探测请求消耗真实配额；账号被删除后对应测试计划自动禁用自己（而非反复报错重试）。

### ⚠️ 需核实（已核实，结论见下）
- `account_scheduling_threshold_eval.go`：该文件在合并树快照（`git show <merge-tree-hash>:backend/internal/service/account_scheduling_threshold_eval.go`）中确认存在**未解决的冲突标记**（`<<<<<<< upstream/main` / `=======` / `>>>>>>> personal-dev`，位于第 197/251/253 行），merge-tree 版本不可直接采信。
  - 用两点 diff（`git diff -w upstream/main personal-dev -- backend/internal/service/account_scheduling_threshold_eval.go`）核实后发现：`personal-dev` 分支相对 `upstream/main` 移除了 `openAIThresholdCandidate` 函数的 `now time.Time` 参数，以及移除了 `openAIQuotaWindowReset(extra, window, now) || openAICodexSnapshotStaleForPause(extra, now)` 陈旧/已重置快照跳过判断逻辑。
  - 进一步核实发现：当前仓库分支（`personal-main`，含 commit `3d3aee2e7 fix(openai): skip stale and reset Codex snapshots in scheduling threshold evaluator`）**已经包含**这段陈旧快照跳过逻辑，且与 `upstream/main` 完全一致（`git diff -w upstream/main HEAD` 对该文件为空）；而 `personal-dev` 分支相对缺失这段逻辑，是较旧/未同步的状态。
  - **结论**：迁移时应以 `personal-main`（当前分支）现有实现为准（保留陈旧快照跳过判断），不要采信 merge-tree 里的冲突产物，也不要采信 `personal-dev` 分支里缺失该判断的旧版本。这属于合并冲突产物问题，不是一个新的疑似退化功能点。

---

## 能力：系统设置中心（对应 cap id 5）

> 说明：原来分散在 `setting_gateway_runtime.go`、`setting_oauth.go`、`setting_parse.go`、`setting_public.go` 四个文件的内容被合并进统一的 `setting_service.go`（纯文件结构重组，不代表功能）。个人 fork 在合并后的基础上新增约 85 个具体的可配置项/管理方法，归类为以下管理后台配置组，每一项基本直接对应后台设置页面里的一个开关/表单项。

### 功能点 1：Admin API Key 管理组
- 描述：生成/删除/状态查询，独立于普通用户 API Key 的管理员级密钥。

### 功能点 2：按注册渠道差异化默认额度
- 描述：Email/DingTalk/微信/LinuxDo/OIDC/GitHub/Google 等每种注册方式可单独配置默认余额、并发上限、RPM 限额，用于防止通过特定渠道批量刷号；"注册时授予"与"首次绑定时授予"是两个独立的触发开关。

### 功能点 3：注册/登录反爬开关+密钥
- 描述：验证码之外的另一层防护机制。

### 功能点 4：Claude 客户端遥测数据处理模式配置

### 功能点 5：前端自定义菜单项
- 描述：管理员可以填写原始 JSON 来配置侧边栏自定义菜单。

### 功能点 6：新用户默认额度包
- 描述：余额、并发、RPM、平台配额的默认值配置。

### 功能点 7：全局模型 Fallback 开关 + 按平台配置备用模型
- 描述：对应能力 4 的 `model_fallback.go` 的管理端配置面。

### 功能点 8：网关调试时间线开关
- 描述：开启后按请求抓取详细的分阶段耗时数据，用于诊断/运维视图。

### 功能点 9："身份伪装"功能
- 描述：开关 + 自定义提示词，注入系统提示让模型对客户端自报不同身份（反检测/品牌定制用途）。

### 功能点 10：Kiro 运行时配置包
- 描述：专属参数集合。

### 功能点 11：按平台的账号调度阈值
- 描述：可配置的用量百分比触发点，超过自动触发临时下线，对应能力 4 机制的管理面。

### 功能点 12：过载冷却参数配置

### 功能点 13：流式超时的阈值+动作+窗口分钟数配置
- 描述：阈值+动作（临时下线 or 报错）+窗口分钟数配置，对应能力 4 流式超时阈值机制的管理面。

### 功能点 14：OpenAI API Key 健康断路器
- 描述：滚动窗口统计 Key 池失败率，超过阈值自动跳闸熔断。

### 功能点 15：邮箱验证/密码重置/注册控制
- 描述：强制邮箱验证开关、密码重置开关、注册开关+域名后缀白名单+注册配额限制。

### 功能点 16：邀请码/推广码
- 描述：注册时强制填邀请码开关，推广码兑换功能开关，各自独立配置。

### 功能点 17：会话绑定与二次验证
- 描述：会话绑定 IP/设备开关，敏感操作（如触发备份）要求二次身份验证（Step-up Auth）。

### 功能点 18：未分组 API Key 是否允许被调度的开关

### 功能点 19：站点品牌自定义配置

### 功能点 20：Passkey/WebAuthn 登录支持及相关配置项

### 功能点 21：TOTP 两步验证 + 加密密钥有效性校验

### 功能点 22：Grok 运行时配置
- 描述：增量流优化开关、模型映射覆盖、媒体 URL 处理方式。

### 功能点 23：OpenAI WebSocket 连接池与增量运行时参数配置

---

## 能力：依赖注入与公共工具（对应 cap id 7）

### 功能点 1：SSRF 防护范围大幅扩展
- 涉及文件：`backend/internal/util/urlvalidator/validator.go`
- 描述：除了原有的环回地址/私有地址/本地链路地址拦截，新增对 IPv4 映射 IPv6、NAT64、CGNAT 地址段、文档保留地址、6to4 地址等特殊用途地址段的拦截。重要的安全加固，扩大了后端 SSRF 攻击防护面。

### 功能点 2：IP 解析工具函数
- 描述：`GetSecurityClientIP` / `GetPeerIP`，支撑 API Key 的 IP 限制、审计日志、会话绑定等功能的底层能力。

### 功能点 3：功能开关的后端强制中间件
- 描述：`TicketFeatureGuard` / `AffiliateFeatureGuard`——管理员在系统设置里关闭工单/邀请返利功能时，后端直接对相关 API 返回 403，而不只是前端隐藏入口（真正的后端强制，不是仅前端遮蔽）。

### 功能点 4：CORS 配置更新
- 描述：新增对 `X-Sub2API-Refresh`、`X-TOTP-Code` 请求头的支持，以及 `Location`、`X-Studio-Live-Call-Id` 响应头的暴露，配合 TOTP 登录、Refresh Token Cookie、Studio Live 流媒体等新功能。

### 功能点 5：Codex 协议头透传
- 涉及文件：`responseheaders.go`
- 描述：透传 `x-codex-turn-state`、`x-reasoning-included` 响应头给客户端，保证 Codex CLI 多轮对话/工具续接行为正确，且不会重复计算推理 token。

### 功能点 6：静态资源热修复
- 涉及文件：`embed_on.go`
- 描述：管理员可以不重新编译二进制文件就替换/热修复前端静态资源。

### 功能点 7：签名下载链接 Host 派生
- 描述：`requestBaseURL` 从请求的 Host 头派生基础 URL 来构建签名下载链接，确保链接指向客户端实际访问的域名/host。

### 功能点 8：`config.go` 部署级配置扩展
- 描述：`AntiBanConfig`（每平台反封号开关，对应管理端网关设置里的可见开关）、媒体存储配置、请求幂等性处理、审计日志保留策略等。

### 功能点 9：DTO 新字段
- 描述：API Key 的 `CurrentConcurrency`（实时并发数，业务含义见能力 3 功能点 14）；Admin User DTO 的 `TodayActualCost` 等消费字段（对应管理端用户管理页面）；`UpstreamModel`/`ModelMappingChain`（用量明细展示实际命中模型与完整映射链，业务含义见能力 3 功能点 5）；`BillingMode` 筛选字段。

### 功能点 10：新增管理端路由
- 描述：几个新的设置相关接口（`temp-unsched-threshold`、`openai-apikey-health-breaker`、`openai-overload-retry` 等），对应能力 5 新设置项的后端 API。

### ⚠️ 需核实事项（已核实，结论见下）
- 原疑点：`audit-logs` 接口的 `StepUpAuthMiddleware` 参数被移除，需要确认是否是权限简化还是遗漏。
- 核实方式：对比 `backend/internal/server/routes/admin.go` 中 `registerAuditLogRoutes` 函数在 `upstream/main` 与 `personal-dev` 的 diff。
- **结论：确认是有意的设计变更，不是遗漏。** upstream 版本签名为 `registerAuditLogRoutes(admin *gin.RouterGroup, h *handler.Handlers, _ middleware.StepUpAuthMiddleware)`（参数本身在 upstream 就已经是未使用的 `_`）；personal-dev 直接删除了该未使用参数，同时保留一条明确的中文注释："清空需现场 TOTP 校验（在 handler 内强制），不复用 step-up sudo 窗口"。也就是说 `/admin/audit-logs/clear` 的二次校验并未被削弱，只是把校验逻辑从路由层的 step-up 中间件下沉到了 handler 内部强制做实时 TOTP 校验，避免误用此前用户在其他敏感操作里刚建立的 step-up sudo 时间窗口去豁免清空审计日志这类高风险操作。

### 功能点 11：进程/服务器基础设施改动
- 涉及文件：`main.go`/`http.go`/`handler.go`/`middleware.go`
- 描述：
  - 版本信息新增构建溯源数据（dirty 标记/源码哈希/前端构建哈希，`--version` 可查看）；
  - 优雅关闭机制（有序停止定时任务后再退出，新增环境变量 `SERVER_SHUTDOWN_TIMEOUT` 配置超时时长）；
  - 可信代理显式配置（未配置时禁用信任链并在 release 模式下警告）；
  - 全局 IP 安全中间件应用到所有路由；
  - 新增 `ContextKeySkipAPIKeyBilling` 允许特定请求路径跳过计费和配额检查。

---

## 能力：CI 与安全扫描（对应 cap id 8）

### 功能点 1：CI 安全扫描基础设施（纯工具链改动，无用户可感知功能）
- 涉及文件：GitHub Actions 工作流、`tools/check_govuln_exceptions.py`、`tools/check_pnpm_audit_exceptions.py`、`tools/secret_scan.py`（全新文件）
- 描述：GitHub Actions 工作流新增安全扫描步骤（`govuln`/`pnpm audit` 异常清单排除机制）；新增密钥泄露扫描脚本。这些改动只影响 CI 流水线本身，不影响运行时用户/管理员可感知的任何行为。

---

## 汇总

| 能力 | cap id | 功能点数（含无新增可感知功能条目单独计） |
|---|---|---|
| 计费与用量统计底座 | 3 | 15 个功能点 + 1 个⚠️疑似退化点 + 3 个无感知内部改动文件 |
| 账号调度/限流/临时下线 | 4 | 8 个功能点 + 1 个已核实的⚠️事项 |
| 系统设置中心 | 5 | 23 个配置组（对应约 85 个具体配置项） |
| 依赖注入与公共工具 | 7 | 10 个功能点 + 1 个已核实的⚠️事项 + 1 个基础设施改动条目 |
| CI 与安全扫描 | 8 | 1 个纯 CI 基础设施改动条目 |
