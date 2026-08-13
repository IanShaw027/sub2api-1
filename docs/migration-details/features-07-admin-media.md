# 功能点清单：管理端 / 媒体存储

对比基准：旧内容 `upstream/main`，新内容 merge-tree `5806f923bdf47619b25fed4a69220b16e181164c`。
说明：`5806f923b...` 里有 6 个文件（`admin_service.go`、`group_handler.go`、`backup_service.go`、`channel.go`、`group.go`、`GroupsView.vue`）仍带有字面的 `<<<<<<< upstream/main / ||||||| / ======= / >>>>>>> personal-dev` 冲突标记（说明这几个文件在 upstream 合并时产生了未解决冲突）。分析时统一取 `personal-dev` 一侧的内容作为"目标新行为"，upstream 一侧视为旧行为对比基准。

---

## 能力：管理端（账号 / 设置 / 运营）（对应 cap id 28）

### 功能点 1：用户列表点击用户名/邮箱 → 跳转到"该用户的用量明细"（不是独立的用户 Dashboard 页面）
- 涉及文件：`frontend/src/views/admin/UsersView.vue`
- 描述：用户管理列表里，**用户名/邮箱单元格被改成了一个可点击按钮**（新增下划线样式提示可点击）。点击后触发 `handleUserUsageJump(user)`：
  1. 计算"最近 24 小时"的起止日期；
  2. 在**当前标签页**（`window.open(url, '_self')`，不是新开页面）跳转到管理端已有的 **`/admin/usage`（用量统计页）**，并在 URL 上带上 `user_id=<该用户ID>&start_date=&end_date=`（近 24 小时）三个查询参数。
  - 也就是说，落地页是**已存在的全局"用量统计"页面**（`UsageView.vue`，用量明细/错误请求/用户排行三个 tab 共用的页面），只是自动预填了"用户=该用户、时间=近24小时"两个筛选条件，本质是**用量记录的按用户筛选跳转**，**不存在**一个专门的"用户详情/Dashboard"页面（没有余额历史、并发、订单等模块汇总展示；那些信息分别在其它按钮打开的独立弹窗里，见功能点 2/3/4）。
  - 全仓库搜索确认：路由表（`frontend/src/router/index.ts`）里只有一条 `/admin/users` → `UsersView.vue`，没有 `/admin/users/:id` 之类的详情路由。

### 功能点 2：用户列表新增"用量统计"列（今日余额/今日订阅/近30日）+ 头像展示 + 并发实时排序
- 涉及文件：`frontend/src/views/admin/UsersView.vue`、`frontend/src/components/admin/user/UserPlatformQuotaModal.vue`（并发保护修复）、`admin_service.go`（`backend/internal/service/admin_service.go`）、`backend/internal/handler/admin/user_handler.go`、`frontend/src/api/admin/users.ts`
- 描述：
  - **用量统计列改造**：原来的"用量"列依赖前端另发一次批量接口 (`getBatchUsersUsage`) 拉取，且按平台（anthropic/openai/gemini/antigravity）拆成 4 个子列；现在改为请求列表接口时带 `include_usage_stats=true`，服务端在 `ListUsers`/`populateUsageStatsForUsers` 阶段直接把每个用户的 `today_balance_actual_cost`（今日余额消费）、`today_subscription_actual_cost`（今日订阅消费）、`total_actual_cost`（近30日消费）附着在用户对象上一次性返回；表格里改成三行展示"今日余额 / 今日订阅 / 近30日"，不再按平台拆分列。
  - **头像展示**：单元格头像圆圈现在会显示用户真实头像（第三方登录同步来的 `avatar_url`，经 `safeImageUrl` 校验），没有头像才回退成邮箱首字母。后端新增 `populateUserAvatarBestEffort`，最佳努力地为列表里每个用户挂上头像 URL。
  - **并发实时排序（新）**：列表右上角新增一个"排序设置"下拉菜单（`showSortDropdown`），可选：今日余额 / 今日订阅 / 近30日 / **当前并发** / **可用并发**，选中后按该维度重新拉取整页数据并排序；并发列表头本身也可以直接点击图标切换排序。后端新增 `isLiveUserConcurrencySort`/`listUsersByLiveConcurrency`/`loadUserConcurrencyForSort`/`sortUsersByLiveConcurrency`：当排序键是 `current_concurrency`（当前并发）或 `available_concurrency`（并发上限-当前并发）时，会把符合筛选条件的**全部**用户一次性取出、批量查出每个人的实时并发占用，在内存里排好序后再分页返回（因为并发数据是运行态缓存，无法在 SQL 层排序）。

### 功能点 3：编辑用户弹窗新增"已绑定第三方登录身份"展示卡片；密码强度提升；用户 API Key 列表脱敏
- 涉及文件：`frontend/src/components/admin/user/UserEditModal.vue`、`backend/internal/handler/admin/user_handler.go`、`admin_service.go`
- 描述：
  - 编辑用户弹窗顶部新增一块信息卡：左侧显示用户头像+邮箱+用户名，右侧列出该用户**已绑定**的第三方登录身份（LinuxDo / OIDC / 微信 / GitHub / Google，各自的头像+昵称/展示名+账号标识），来自新接口 `GET /admin/users/:id` 现在会额外返回 `auth_bindings`/`identity_bindings`（后端新增 `AdminService.GetUserIdentitySummaries`）。原来弹窗里的"角色"下拉框（普通用户/管理员切换）从这个编辑表单里移除了。
  - 管理员创建/修改用户时密码最小长度从 6 位提升到 8 位。
  - 管理员查看某用户的 API Key 列表（`GetUserAPIKeys`）现在返回脱敏后的 key（`APIKeyFromServiceMasked`），不再是可以直接复制出明文完整密钥的版本；同时接口加了访问审计日志（记录操作管理员ID/来源IP等）。

### 功能点 4：用户平台配额弹窗支持 Kiro 平台
- 涉及文件：`frontend/src/components/admin/user/UserPlatformQuotaModal.vue`、`frontend/src/components/user/UserPlatformQuotaCell.vue`、`frontend/src/api/admin/users.ts`
- 描述：平台配额（每日/每周/每月限额）弹窗与列表徽标里的平台顺序新增 `kiro`（原来只有 anthropic/openai/gemini/antigravity/grok）；同时修复了弹窗快速切换用户时的竟态问题（用 session id 标记每次打开，避免上一个用户的加载结果串到下一个用户界面上）。

### 功能点 5：管理员仪表盘新增指标 / 用量筛选新增 billing_mode 维度
- 涉及文件：`backend/internal/handler/admin/dashboard_handler.go`、`backend/internal/handler/admin/dashboard_query_cache.go`、`backend/internal/handler/admin/dashboard_snapshot_v2_handler.go`、`backend/internal/repository/dashboard_aggregation_repo.go`
- 描述：这是"管理员仪表盘新增指标"的核心落点。
  - **`GetStats`（仪表盘总览统计接口）新增字段，"今日"和"累计"各一份，共 10 个新指标**：
    - `total_account_cost` / `today_account_cost`：按账号倍率折算的实际上游成本（区别于对用户的计费）；
    - `total_balance_actual_cost` / `today_balance_actual_cost`：**按余额付费**部分的实际扣费金额；
    - `total_subscription_actual_cost` / `today_subscription_actual_cost`：**按订阅套餐消耗**部分（不实扣但纳入统计）的金额；
    - `total_recharge_amount` / `today_recharge_amount`：**充值金额**统计；
    - `total_refund_amount` / `today_refund_amount`：**退款金额**统计。
    - 底层由 `dashboard_aggregation_repo.go` 的小时/日汇总 SQL 新增 `balance_actual_cost`/`subscription_actual_cost` 两列聚合支撑（对应数据库迁移 `153_add_dashboard_billing_split_costs.sql`）。
  - **仪表盘用量趋势/模型统计/分组统计（`GetUsageTrend`/`GetModelStats`/`GetGroupStats`）新增 `billing_mode` 筛选维度**，取代了原来的"上游模型不一致审计"（`upstream_model_mismatch`）筛选，即管理员现在可以按 token/per_request/image/video/search/audio 等计费模式来切片查看趋势/模型/分组统计，而不能再按"上游返回模型是否与请求模型不一致"筛选。
  - `GetUserBreakdown`（用户维度用量排行下钻）新增 `billing_mode`、`exclude_admin`（排除管理员自己产生的用量）两个筛选参数。
  - `GetStats`（仪表盘）本身的时间范围能力增强：新增 `GetDashboardStatsWithRange`，支持按自定义起止时间统计（原来固定语义）。

### 功能点 6：用量统计（Usage）页面增强：排除管理员开关 / 新计费类型标签 / 清理任务过滤
- 涉及文件：`frontend/src/components/admin/usage/UsageFilters.vue`、`frontend/src/components/admin/usage/UsageTable.vue`、`frontend/src/components/admin/usage/UsageCleanupDialog.vue`、`frontend/src/views/admin/UsageView.vue`、`backend/internal/handler/admin/usage_handler.go`、`frontend/src/api/admin/usage.ts`、`frontend/src/api/usage.ts`、`frontend/src/utils/usageRequestType.ts`、`frontend/src/views/user/UsageView.vue`
- 描述：
  - 用量列表/统计筛选栏新增一个"**排除管理员**"开关（`exclude_admin`），打开后统计和列表都不再包含管理员账号自己测试产生的用量；同时把原来的"上游模型审计（一致/不一致）"筛选去掉，改为按 `billing_mode` 筛选。
  - 请求类型筛选新增 `image_web_bridge`（图生图桥接）、`image`（图片）、`video`（视频）三种类型标签（原来只有 sync/stream/ws_v2/live/cyber）。
  - 用量明细行的悬浮 tooltip：图片计费模式明细拆分为"图片小计 + Token小计 + 行合计"三部分；新增 `video` 计费模式的专属明细展示（分辨率/时长/单价/小计）。
  - "清理旧用量记录"任务（`UsageCleanupDialog`）的创建条件里新增 `billing_mode`、`exclude_admin` 两个过滤维度，即管理员可以只清理某种计费模式或排除管理员自己的用量记录。
  - 用户自己的用量页 (`user/UsageView.vue`) 导出文本也同步补上了 image/video/image_web_bridge 的中文标签。

### 功能点 7：公告已读状态管理支持"全部/已读/未读"筛选
- 涉及文件：`backend/internal/handler/admin/announcement_handler.go`、`backend/internal/handler/announcement_handler.go`、`backend/internal/service/announcement.go`、`backend/internal/service/announcement_service.go`、`backend/internal/repository/announcement_read_repo.go`
- 描述：管理端"公告管理 → 查看已读状态"列表接口新增 `read_status` 查询参数（`all`/`read`/`unread`），可以只看已读或只看未读用户；仓储层新增 `ListUserReadStatus`，直接在数据库层按公告受众条件（订阅分组/余额条件）+ 已读表关联查询，替换了原来"先查全部符合受众的用户再逐个查订阅、逐个算是否命中"的低效实现。用户自己查看公告列表的接口（`ListForUser`）也从"仅未读"布尔开关改成了同样的三态 `read_status` 语义。

### 功能点 8：分组管理定价体系全面升级（图片/视频路由化定价、显式搜索&语音单价、存储计费、复制分组）
- 涉及文件：`frontend/src/views/admin/GroupsView.vue`、`frontend/src/views/admin/groupsOpenAIImagePricing.ts`、`frontend/src/views/admin/groupsModelsList.ts`、`frontend/src/components/admin/group/GroupRPMOverridesModal.vue`、`frontend/src/components/admin/group/GroupRateMultipliersModal.vue`、`frontend/src/components/admin/channel/codexImageGenerationBridge.ts`、`frontend/src/api/admin/groups.ts`、`backend/internal/handler/admin/group_handler.go`、`group.go`（`backend/internal/service/group.go`）、`group_service.go`、`admin_group_duplicate.go`、`admin_service.go`
- 描述：这是分组编辑表单里改动面最大的一块，多个字段合并成一条完整描述：
  - **移除**：原来的"利润控制"（`ProfitControlEnabled`/`ProfitMinMargin`/`ProfitSafetyBuffer`）配置整块被删除；原来的"批量图片生成折扣/持有倍率"（`BatchImageDiscountMultiplier`/`BatchImageHoldMultiplier`）配置也被移除。
  - **图片生成改为"路由"模型**：新增 `image_generation_route`（`codex` / `web2api` / `native`）。OpenAI 平台固定走 `codex` 路由（前端自动锁定，无需手选）、Grok 平台固定走 `native` 路由并把倍率锁定为 1，只有 Gemini 等其它平台才允许"独立倍率 + 1K/2K/4K 阶梯定价"自定义；新增 `openai_image_main_model`（OpenAI 图片主模型选择）、`images2api_price_1k/2k/4k`（Images2API 路由下的阶梯定价）。
  - **视频生成从"按模型倍率"改为"按分辨率每秒显式定价"**：新增 `allow_video_generation` 开关 + `video_generation_route`（目前固定 `native`）+ `video_price_480p_per_sec`/`720p_per_sec`/`1080p_per_sec`/`4k_per_sec` 四档每秒单价，取代了旧的 `video_rate_multiplier` + `VideoModelPrices`（按模型族×分辨率的倍率表）机制。
  - **新增显式定价项**：搜索工具调用单价（`search_price_per_1k`）、Grok 语音 Realtime/TTS/STT 三档单价（`audio_realtime_price_per_min`/`audio_tts_price_per_million_chars`/`audio_stt_price_per_hour`）、**AI Studio 存储计费单价**（`storage_price_per_gb_day`，USD/GiB/自然日）。
  - **分组自身信息新增**：`display_name`（展示名，供面向用户的分组选择器/可用渠道页面显示更友好的名字，替代内部 `name`）、`refund_rate_multiplier`（退款倍率，编辑表单里已可见）、`user_selectable`（是否允许用户自主选择该分组）。
  - **分组"复制"功能**（`admin_group_duplicate.go`）同步支持复制上述所有新字段。
  - **分组统计接口真正实现**：`GET /admin/groups/:id/stats` 原来是写死返回全 0（代码里明确写着"Return mock data for now"），现在真正查询该分组下 API Key 总数/活跃数，并结合仪表盘用量数据补上总请求数与总费用。
  - **分组用量汇总接口**（`GetUsageSummary`）新增按分组 ID 列表批量查询（`?ids=1,2,3`），不再必须一次性查全部分组。
  - `GetGroupAPIKeys` 返回的 Key 列表增加脱敏（`APIKeyFromServiceMasked`）并限制单页最多 100 条。
  - `RPM 覆盖 / 费率倍率`弹窗里搜索用户时不再额外拉取用量统计与订阅数据（性能优化），平台名称展示改用统一的 i18n 兼容函数。

### 功能点 9：账号导入增强（ZIP 压缩包批量导入 + 去重模式 + 兼容 Kiro 原始导出格式）
- 涉及文件：`backend/internal/handler/admin/account_archive_import.go`、`backend/internal/handler/admin/account_data.go`
- 描述：
  - **新增 `POST /admin/accounts/import/archive`**：管理员可以直接上传一个 ZIP 压缩包（最大 64MiB，最多 5000 个文件），后端会遍历包内每个 `.json` 文件自动判断类型——是 sub2api 自身导出的备份格式（含 `accounts`/`proxies` 数组）还是 Codex OAuth 单文件会话格式（含 `access_token`/`tokens`），混合识别后分别复用已有的"数据导入"和"Codex 会话导入"管线一次性批量导入账号与代理；按文件内容 SHA256 做幂等，避免重复点击产生重复数据。
  - **原有单文件 JSON 导入**（`account_data.go`）新增"去重模式"（`none`/`overwrite`/`ignore`），导入结果统计新增"已更新数量"/"已跳过数量"（原来只有创建/失败两种）；同时兼容**Kiro 客户端原始导出格式**（顶层是账号数组而非标准备份对象），会自动从每条记录的 `kiro_*_raw` 嵌套字段中提取正确的凭证结构后再走标准导入流程。

### 功能点 10：账号"复制"（克隆）功能
- 涉及文件：`admin_account_duplicate.go`（`backend/internal/service/admin_account_duplicate.go`）
- 描述：管理员可以对已有账号执行"复制"（对应账号操作菜单里的复制按钮），新建一个独立账号并继承凭证、代理、分组绑定优先级、并发/倍率等配置，但会清空大量运行态/配额使用状态（如 Codex/Grok/Antigravity 用量快照、上游计费探针缓存等一长串 runtime extra 键），新账号默认**不可调度**（需要管理员手动确认后再开启），避免复制后意外分流真实流量；仅支持 apikey/upstream/bedrock/service_account 类型账号，"影子账号"和轮换凭证类型账号不支持复制；基于幂等 key 防止重复点击生成多份副本。

### 功能点 11：批量编辑账号增强（批量绑定分组 / 自动触发 WS 连接池重整 / 代理联动影子账号）
- 涉及文件：`admin_service.go`
- 描述：批量编辑账号（Bulk Update）新增支持一次性把选中的账号批量绑定到指定分组集合；批量修改代理/并发/状态/可调度/凭证/Extra 中任意一项后，会自动判断是否影响 OpenAI 长连接池（WS Pool），命中则自动触发一次连接池重整（`TriggerOpenAIWSPoolReconcile`），无需人工重启；批量修改代理时，如果账号有关联的"影子账号"，会自动把新代理同步传播给影子账号。此外新增账号创建/更新/批量编辑时对"TLS 指纹路由绑定平台是否匹配"的校验，以及分组"仅允许 OAuth 账号"规则在批量编辑路径下也生效（原来可能只在单个创建/更新时校验）。

### 功能点 12：联盟返利后台管理服务增强（对应 cap19 前台页面，服务代码落在本能力）
- 涉及文件：`affiliate_service.go`
- 描述：`affiliate_service.go` 新增了成套的管理员侵入式操作能力：查改/重置指定用户的邀请码（`AdminUpdateUserAffCode`/`AdminResetUserAffCode`）、为单个或批量用户设置专属返利比例覆盖全局默认（`AdminSetUserRebateRate`/`AdminBatchSetUserRebateRate`）、查看某用户的邀请返利概况（`AdminGetUserOverview`）、分页查看邀请记录/返利记录/余额转账记录（`AdminListInviteRecords`/`AdminListRebateRecords`/`AdminListTransferRecords`）；此外新增"新用户注册奖励"（`ApplySignupBonus`）与"AI 技能创作者收益结算"（`CreditCreatorEarnings`/`ReverseCreatorEarnings`）两条与返利系统打通的资金流。对应的前端管理页面（`AdminAffiliateRecordsTable.vue`）归类在 cap19，本次未展开其 UI 细节。

### 功能点 13：系统设置新增多项配置分区，并与媒体存储打通
- 涉及文件：`backend/internal/handler/dto/settings.go`、`backend/internal/handler/admin/setting_handler.go`（合并了已删除的 `setting_handler_audit.go`/`setting_handler_email.go`/`setting_handler_runtime.go`/`setting_handler_update.go` 四个文件的全部内容）、`backend/internal/handler/setting_handler.go`（公开设置传递部分）
- 描述：
  - **新增独立可配置分区（各自带 GET/UPDATE 接口）**：`OpenAIRetryableOverloadSettings`（OpenAI 选中模型容量/服务器过载的同号重试策略）、`OpenAIAPIKeyHealthBreakerSettings`（OpenAI API Key 健康熔断）、`TempUnschedThresholdSettings`（临时不可调度规则的窗口阈值）。
  - **新增设置字段（部分列举）**：客服二维码列表 `SupportQRCodes`（图文，用于登录页/首页展示联系方式）、下载工具链接 `DownloadToolsURL`、AI Studio 相关开关与分模态白名单/分组、AI 资产宽限天数、计费时区、Live ICE 服务器；IP 多账号封禁风控参数（阈值/窗口/学习期）；**各第三方登录来源（邮箱/LinuxDo/OIDC/微信/GitHub/Google/钉钉）可以单独配置默认平台配额**，覆盖系统全局默认；联盟返利参数扩展为开关+封顶金额+邀请数上限+注册奖励等更细的字段；工单功能开关 `TicketEnabled`。
  - **移除**：腾讯云验证码（Tencent Captcha）与阿里云验证码（Aliyun Captcha）相关配置字段被整体删除；独立的"面板 API 限流设置"接口被移除。
  - **设置里的图片字段接入媒体存储**：保存系统设置时，新引用的图片（如二维码图）会被自动登记进"媒体存储"服务管理，保存成功后不再被引用的旧图片文件会被自动清理，避免孤儿文件堆积（`ingestSettingsMediaReferences`/`cleanupIngestedSettingsMediaReferences`，与 cap29 的媒体存储直接打通）。
  - **保存设置事务性增强**：更新设置的接口新增 `rollbackAdminSettingsUpdate`，如果保存过程中途失败，会自动回滚已经生效的部分改动，避免设置处于中间不一致状态。

### 功能点 14：邮件通知系统新增事件类型与队列可靠性增强
- 涉及文件：`backend/internal/service/notification_email_service.go`、`backend/internal/service/email_queue_service.go`、`backend/internal/service/email_service.go`
- 描述：邮件通知模板新增两种事件类型 —— **发票开具通知**（`invoice.issued`）与**工单回复通知**（`ticket.reply`），管理员可在设置里为这两类事件自定义/预览/恢复官方模板文案；邮件发送队列新增"通知类"任务类型（`TaskTypeNotification`），失败自动重试最多 3 次（间隔 500ms），队列容量从 100 扩大到 2048，避免通知邮件在高峰期被直接丢弃。

### 功能点 15：渠道计费模式扩展（video/search/audio）与渠道模型价格同步支持 Grok
- 涉及文件：`channel.go`（`backend/internal/service/channel.go`）、`channel_service.go`、`channel_available.go`、`backend/internal/handler/admin/channel_handler.go`
- 描述：渠道模型定价的 `billing_mode` 从原来的 `token`/`per_request`/`image` 三种扩展到新增 `video`/`search`/`audio` 三种（对应功能点 8 里分组级的视频/搜索/语音显式定价）；"渠道 → 模型价格同步"功能新增对 Grok 平台的支持（返回内置模型列表，因为 LiteLLM 定价目录里没有 xai/grok 条目）；"可用渠道"（面向用户展示的聚合渠道页）现在会用分组的展示名（`display_name`）而不是内部名称展示，并且**只展示当前分组下确有可调度账号真正支持的模型**（新增按分组已绑定的可调度账号能力二次过滤），避免展示实际不可用的模型。

### 功能点 16：渠道监控（v1 引擎）增强：支持 Kiro、本地探测自应答、附加检测模型、手动触发与人工修正可用率
- 涉及文件：`channel_monitor_aggregator.go`、`channel_monitor_checker.go`、`channel_monitor_const.go`、`channel_monitor_probe.go`（新文件）、`channel_monitor_runner.go`、`channel_monitor_service.go`、`channel_monitor_template_service.go`、`channel_monitor_template_types.go`、`channel_monitor_types.go`、`channel_monitor_v2.go`、`channel_monitor_v2_aggregator.go`、`channel_monitor_validate.go`
- 描述（渠道监控前端页面归类在 cap24，本次只描述这批后端服务文件的能力变化）：
  - **新增 Kiro 供应商监控支持**（原来只有 openai/anthropic/gemini/grok）。
  - **新增"本地探测自应答"机制**（`channel_monitor_probe.go` 全新文件）：网关对外发出的渠道监控探测请求会带上一个短期签名 header（HMAC + 45 秒有效期 + 防重放 nonce）；当请求恰好命中网关自己签发的内置算术题探测（且未使用自定义 body_override）时，网关可以直接在本地生成正确答案返回，**不会真的转发给上游 AI 服务商**，从而避免"自我监控"消耗真实上游账号配额或产生真实计费。
  - **新增"附加检测模型"**：单个监控除主模型外，最多可再配置 20 个附加模型（每个最长 200 字符）一起做测活，用于同时观测同一渠道下多个模型的可用性。
  - **自定义请求体校验加强**：OpenAI 的自定义 body_override 现在必须显式带 `model` 字段（原来只校验 messages/instructions+input）。
  - **worker 队列增加缓冲**（`monitorWorkerQueueSize`），避免启动时或同一时刻批量触发检测时因 worker 忙而直接漏检。
  - **支持管理员手动立即触发一次检测**（`RunManual` + 分布式运行锁 `AcquireChannelMonitorRunLock`，防止同一监控项被并发重复触发）。
  - **支持人工修正某监控项的 7 天可用率**（`AdjustPrimaryAvailability7d`，对应前端 `MonitorAvailabilityAdjustDialog.vue`），管理员可以手动把展示的可用率百分比调整为期望值（系统按样本行数反推需要改动多少条历史记录）。
  - 监控运行时会持续侦听功能开关变化，功能被关闭时自动暂停调度、重新打开后自动恢复（`watchRuntime`/`pauseForDisabledFeature`/`syncRuntime`）。
  - 自定义 header 模板的禁用清单扩大：新增禁止覆盖 `authorization`/`x-api-key`/`anthropic-version` 等鉴权与协议相关 header，防止自定义探测模板绕过账号凭证保护。

### 功能点 17：审计日志自动清理后台任务（新）
- 涉及文件：`audit_retention_service.go`（新文件）
- 描述：新增一个纯后台定时任务（不在设置页面暴露开关，通过 `config.yaml` 的 `AuditRetention` 配置段控制），按配置的保留天数（默认 180 天）、清理间隔（默认 1 小时）、单批删除条数（默认 1000 条）周期性清理三张纯追加审计表（`ai_audit_logs`、`codex_invite_reset_history`、`deleted_api_key_audits`）中的过期数据，防止这些表无限增长拖慢查询；单表清理失败不影响其它表继续清理。

### 功能点 18：备份系统对接对象存储配置（与媒体存储共用同一套 Profile 机制）
- 涉及文件：`backend/internal/service/backup_service.go`、`backend/internal/service/backup_record_repository.go`（新文件，接口骨架，见"无新增可感知功能"说明）
- 描述：后台"数据备份"页面新增**对象存储配置管理**能力：`GetObjectStorageSettings`/`UpdateObjectStorageSettings` 让管理员配置一个或多个 S3 兼容存储 Profile（用于把数据库备份文件上传到远端），`TestObjectStorageProfile` 提供"测试连接"能力验证凭证是否有效；密钥类字段在接口返回时会做脱敏处理（`sanitizeObjectStorageSettingsForResponse`/`preserveObjectStorageSecrets`）。这套存储 Profile 机制与 cap29"媒体存储"共用（`SetMediaStorageConfigProvider`），即数据库备份与用户媒体文件上传可以配置到同一个或不同的 S3 兼容存储上。

### 功能点 19：工单系统底层领域模型落地（对应 cap20 前台/后台页面）
- 涉及文件：`ticket.go`（新文件，`backend/internal/service/ticket.go`）、`ticket_service.go`（新文件）
- 描述：定义了工单分类（咨询/退款/并发申请/费率申请/其他）、状态机（已提交→处理中→待用户/待管理员→已解决/已关闭/已撤回）、发送人角色（用户/管理员/系统）、消息类型等常量及配套错误码，并实现工单创建/回复/状态流转等服务逻辑。这是 cap20"工单系统"前后台页面（`TicketsView.vue`/`TicketDetailView.vue` 等，归类在 cap20）背后的核心状态机，本文件本身不含 UI。

### 功能点 20：用户自助查看余额变动历史（面向登录用户本人，非管理端）
- 涉及文件：`frontend/src/components/user/UserBalanceHistoryModal.vue`（新文件）
- 描述：与管理端无关，是用户自己在个人仪表盘/个人资料页可以打开的一个弹窗，展示自己的余额/并发变动历史（充值、管理员调整、订阅分配等），支持按类型筛选（余额/管理员调余额/并发/管理员调并发/订阅）+分页，顶部展示当前余额与累计充值金额。该组件被 `views/user/DashboardView.vue` 和 `views/user/ProfileView.vue`（均不在本次能力范围内）引用。

### 无新增可感知功能的文件（cap 28）
- `admin_group.go`：与 upstream 完全一致，无改动。
- `admin_account.go`、`admin_proxy.go`、`admin_user.go`：内容被整体搬迁合并进 `admin_service.go`，属于纯代码重组（原文件被删除），没有产生独立的新行为（新增的行为已在功能点 2/3/9/10/11 中体现）。
- `admin_compliance.go`（`backend/internal/service`）：仅把"设置服务不可用时静默返回默认值"改成了显式报错，属于健壮性修正。
- `backend/internal/server/middleware/admin_auth.go`：Token 版本校验改成调用统一的 `ResolveUserTokenVersion` 辅助函数，纯内部重构。
- `backend/internal/server/middleware/admin_compliance.go`：去掉了一个多余的 nil 判断分支，纯内部重构。
- `backend/internal/repository/dashboard_cache.go`：给 Redis 客户端未初始化的情况加了 nil 防护，避免 panic，无行为变化。
- `channel_monitor_aggregator.go`：把"批量取最新状态"拆成可复用的内部函数，避免同一批数据被查两次，纯性能重构。
- `channel_monitor_ssrf.go`：私网 IP 判定逻辑改为复用项目里通用的 `urlvalidator` 工具包，无行为变化。
- `proxy_service.go`：代理测试连通性逻辑内部重构（引入公共 proxy 工具包、补充 nil 仓储防护），默认测试地址、超时等行为不变。
- `frontend/src/views/admin/groupsModelsList.ts`：新增过滤掉以 `*` 结尾的通配符模型名，属于很小的展示层修正。

---

## 能力：媒体存储（对应 cap id 29）

对象存储后端：**仅支持 S3 协议兼容的对象存储**（AWS S3，或任何兼容 S3 API 的服务，如 MinIO / Cloudflare R2 / 阿里云 OSS 的 S3 兼容模式等），通过可配置的 Endpoint、Region、AccessKeyID/SecretAccessKey、Bucket、是否强制路径样式（path-style）接入；**没有实现本地磁盘存储驱动**。存储配置以"Profile"形式管理，与 cap28 的"数据备份"对象存储配置共用同一套机制（见 cap28 功能点 18），媒体资产表（`media_assets`）里每条记录都会记录自己绑定的 `storage_profile_id` 与可选的自定义公开访问域名 `public_base_url`。

### 功能点 1：媒体文件上传/下载/管理全套接口（全新子系统）
- 涉及文件：`backend/internal/handler/media_handler.go`（用户端，新文件）、`backend/internal/handler/admin/media_handler.go`（管理端，新文件）、`backend/internal/handler/dto/media.go`（新文件）、`backend/internal/repository/media_object_store.go`（新文件，S3 客户端实现）、`backend/internal/repository/media_repository.go`（新文件，数据库仓储）、`backend/internal/server/routes/media.go`（新文件，路由注册）
- 描述：这是一个全新的、独立于其它业务的通用媒体资产管理子系统，被 AI 创作工作台的素材、发票附件、用户头像等业务复用。具体能力：
  - **上传**：`POST /v1/media/upload`（登录用户自助上传，随请求带 `biz_type`/`biz_id`/`visibility` 等标记，可同时附一张缩略图）；`POST /v1/admin/media/upload`（管理员上传，可指定 `owner_user_id` 代任意用户上传）。单文件默认最大 **64MiB**（可由存储运行时配置覆盖）；上传时服务端会自动计算文件的 SHA256 摘要、探测图片的宽高（支持 JPEG/PNG/GIF/WebP）、识别 MIME 类型。
  - **下载 / 访问**：
    - 公开资源：`GET /v1/media/public/:id`（及 `/thumbnail` 缩略图变体），无需鉴权，带 `Cache-Control: public, max-age=300`；
    - 私有资源：登录用户需先调用 `POST /v1/media/:id/presign-download`（或缩略图版本）换取一个带签名、默认 **15 分钟**有效期的临时下载直链，或走后端反代的签名直链 `GET /v1/media/download/:id?expires=&sig=`（响应带 `Content-Disposition: attachment` 且做了文件名清洗，防止响应头注入）；
    - 管理员对任意媒体资产也有对应的 `PresignDownload`/`PresignThumbnailDownload` 接口，不受资产是否属于自己限制。
  - **管理**：`GET /v1/admin/media` 列表（支持按 `biz_type`/`biz_id`/`visibility`/`status`/`owner_user_id`/关键字搜索 + 分页）；`GET/DELETE /v1/admin/media/:id`；`POST /v1/admin/media/:id/visibility` 修改可见性（公开/私有）。用户端也有对应的自助 `GetByID`/`Delete`/`UpdateVisibility`（仅限自己的资产）。
  - **下载直链自适应访问域名**：新增 `backend/internal/handler/admin/request_base_url.go`（在 cap28 文件列表中，但专为媒体下载链接服务），签名下载链接会使用**用户实际访问网关时用的域名**反向拼接，而不是写死配置的对象存储域名，避免跨域问题、也避免把真实的对象存储端点暴露给客户端。

### 无新增可感知功能的文件（cap 29）
（cap29 分配的 6 个文件均为全新文件，已在上方功能点中完整覆盖，无需单独列出"无新增"条目。）

---

## 附：对用户问题的直接结论

1. **"用户列表点击进入用户 dashboard"这个说法不完全准确**：点击用户名/邮箱确实会跳转，但跳转目标是管理端**已有的"用量统计"页面**（`/admin/usage`），并自动带上 `user_id` + 近24小时的时间范围筛选，本质是"按该用户筛选用量记录"，**不是**一个汇总展示余额/并发/订单等信息的专属"用户详情/Dashboard"页面。若想看该用户更全的信息，需要分别点开"编辑"（看资料+绑定身份）、"余额历史"、"平台配额"等各自独立的弹窗按钮，这些信息目前是分散在多个弹窗里，没有整合成单一详情页。
2. **"管理员仪表盘新增了哪些具体指标"**：核心新增是仪表盘总览统计接口 `GetStats` 新增的 **10 个字段**（今日+累计各 5 个）：账号成本（`account_cost`）、按余额付费的实际消费（`balance_actual_cost`）、按订阅消耗的金额（`subscription_actual_cost`）、充值金额（`recharge_amount`）、退款金额（`refund_amount`）。配合数据库迁移 `153_add_dashboard_billing_split_costs.sql`。此外仪表盘的用量趋势/模型统计/分组统计三个图表新增了按 `billing_mode`（计费模式：token/per_request/image/video/search/audio）筛选的能力（替代了原来的"上游模型不一致"审计筛选），用户维度下钻新增"排除管理员自身用量"选项。
