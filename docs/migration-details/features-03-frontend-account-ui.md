# 功能点清单：前端地基 / 账号管理界面

> 对比基准：旧内容 = `upstream/main`；新内容（含全部个人定制）= merge-tree `5806f923bdf47619b25fed4a69220b16e181164c`。
> 覆盖范围：`cap_files.json` 中 `id` 为 6、9 的两个能力（"前端地基（i18n / 公共组件 / 框架）"、"账号管理界面（授权 / 指纹 / 容量）"），共约 159 个文件，逐文件核对 diff 内容后按"用户/管理员在界面上能感知到什么"整理。

## ⚠️ 数据质量提醒：多个文件在 merge-tree 中存在未解决的 Git 冲突标记

逐文件核对时发现，以下 **7 个文件**在 `5806f923bd...` 这个 merge-tree 里字面上还留着 `<<<<<<< upstream/main` / `|||||||` / `=======` / `>>>>>>> personal-dev` 冲突标记，没有被真正解决——也就是说这些文件当前内容**不是可编译/可运行的最终状态**，从中提炼出的功能点只能代表"合并前两侧的意图"，迁移时必须先手动解决冲突：

- `frontend/src/components/common/PlatformTypeBadge.vue`（4 处冲突，涉及 Grok/Gemini 套餐徽章逻辑）
- `frontend/src/i18n/locales/en/admin/overview.ts`、`frontend/src/i18n/locales/zh/admin/overview.ts`（各 1 处，Grok 语音/搜索定价文案）
- `frontend/src/types/index.ts`（1 处，`AdminGroup.profit_control_enabled` 相关字段）
- `frontend/src/components/account/AccountUsageCell.vue`（1 处）
- `frontend/src/components/account/BulkEditAccountModal.vue`（1 处，"上游倍率自动探测"适用平台范围）
- `frontend/src/components/account/EditAccountModal.vue`（**5 处**，其中一处是"OpenAI 订阅档位手动覆盖"整段功能是否保留）
- `frontend/src/composables/useModelWhitelist.ts`（1 处，Grok 模型列表）

---

## 能力：前端地基（i18n / 公共组件 / 框架）（对应 cap id 6）

### 功能点 1：分组容量徽章只显示"实际当前并发"，去掉了会话数/RPM 子徽章
- 涉及文件：`frontend/src/components/common/GroupCapacityBadge.vue`
- 描述：这个徽章出现在分组选择器、账号列表等多处"分组"旁边。旧版本会同时显示 3 个小徽章——并发（已用/上限）、会话数（已用/上限）、RPM（已用/上限）。新版本**只保留并发徽章**，样式改为"已用[/上限，上限>0 才显示]"，并加了 `title="当前分组真实并发"` 的悬浮提示；会话数和 RPM 两个子徽章被完全移除。这与任务描述里"分组的容量中当前并发按各个分组实际的并发"的例子直接对应。

### 功能点 2：管理员仪表盘新增"本日消耗"“累计消耗”两张统计卡，附费用来源明细
- 涉及文件：`frontend/src/views/admin/DashboardView.vue`
- 描述：后台"仪表盘"页面的统计卡片从 8 张（2 行 × 4 列）变成单行 5 列共 9 张：新增"本日消耗"和"累计消耗"两张卡片，显示当日/累计实际扣费金额；每张卡片鼠标悬停会弹出提示框，把费用拆成 4 行明细——余额消耗、订阅消耗、充值金额、退款金额（今日/累计各一套）。原来"今日 Token"“累计 Token”卡片里一直显示的"实际/账号成本/标准"三段费用文字，改成收进同款悬浮提示里，卡面更简洁。

### 功能点 3：用户仪表盘余额卡片可点击，打开余额变动明细弹窗
- 涉及文件：`frontend/src/components/user/dashboard/UserDashboardStats.vue`、`frontend/src/views/user/DashboardView.vue`
- 描述：用户仪表盘的"余额"卡片变成了一个按钮，鼠标悬停提示"点击查看余额明细"，点击后弹出 `UserBalanceHistoryModal` 展示该用户的余额变动记录，之前余额卡只是纯展示。同时"今日 Token”“累计 Token"卡片新增一行"缓存写入/缓存读取"token 拆分数据（之前只有输入/输出）。

### 功能点 4：用户仪表盘最近使用记录标注"被路由到更贵上游"的请求
- 涉及文件：`frontend/src/components/user/dashboard/UserDashboardRecentUsage.vue`
- 描述："最近使用"列表里，如果某条请求实际服务的模型（`upstream_model`）和用户请求/映射的模型不一致，模型名称右侧会加一个"↳ 实际上游模型"的小字提示；如果这条请求是被自动降级/切换到了计费更贵的上游（`billed_by_higher_priced_upstream`），显示的实际费用数字会变成**红色**（正常是绿色）——让用户一眼看出"这一条为什么比平常贵"。

### 功能点 5：公告"已读/未读"筛选（管理端弹窗 + 用户端铃铛弹层）
- 涉及文件：`frontend/src/api/admin/announcements.ts`、`frontend/src/api/announcements.ts`、`frontend/src/components/admin/announcements/AnnouncementReadStatusDialog.vue`、`frontend/src/components/common/AnnouncementBell.vue`、`frontend/src/stores/announcements.ts`
- 描述：管理端"公告 > 已读状态"弹窗的搜索框旁新增一个"全部/已读/未读"下拉筛选，切换后重新拉取已读状态表格。用户端点击顶部公告铃铛打开的弹层里，顶部新增一排"全部/未读/已读"分段按钮，点击切换会重新拉取对应列表，并且空状态文案会按当前筛选变化（例如"暂无未读公告"）。未读数徽章现在独立于分页拉取单独维护，不会因为翻页/切换筛选而跳变。

### 功能点 6：管理员合规确认弹窗新增"服务暂不可用"状态与重试
- 涉及文件：`frontend/src/components/admin/AdminComplianceDialog.vue`、`frontend/src/stores/adminCompliance.ts`
- 描述：管理员登录后弹出的"部署与运营合规确认"弹窗，如果后端合规状态查询失败（网络错误/5xx），不再是简单报错，而是切换成一个蓝色提示框，标题变成"合规状态暂不可用"，正文说明"为安全起见管理页面保持锁定"，并提供一个带 loading 动画的"重试"按钮，重试成功后自动恢复正常确认流程。

### 功能点 7：侧边栏导航支持二级展开分组，新增未读数红色角标
- 涉及文件：`frontend/src/components/layout/AppSidebar.vue`、`frontend/src/components/layout/SidebarNavBadge.vue`（新文件）、`frontend/src/composables/useNavigationBadges.ts`（新文件）、`frontend/src/navigation/simpleMode.ts`（新文件）
- 描述：
  - 侧边栏导航项现在支持"分组+子菜单"形式：点击带子菜单的一级菜单会展开/收起（箭头旋转动画），下方缩进显示子菜单项。用于新的"创作台"（对话/图片/生视频/音频/Realtime/视频创作/资产库/提示词）、管理员"AI 治理"（提示词治理/作品治理/存储计费对账）、管理员"技能治理"（技能审核/技能治理/运行监控/技能结算）等子菜单。
  - 导航项旁边会出现红色数字角标（超过 99 显示"99+"），侧边栏收起时角标会移到图标右上角。角标数字来源：用户端"工单"未读提醒数、"创作台>视频创作"里正在渲染/合成中的任务数；管理端"工单"未读数、"订单管理>发票申请"未读数。角标每 60 秒自动刷新一次，切换路由、切回浏览器标签页、或应用内其他地方触发了 `sub2api:navigation-badges-changed` 事件时也会刷新。
  - 新增顶层导航项：用户端"工单"、"创作台"及其子菜单、"技能中心"/"已安装技能"；管理端"工单"、"AI 治理"子菜单、"技能治理"子菜单、订单管理下的"发票申请"。
  - 原来的"批量生图"导航项被整体移除（这个 fork 里没有这个功能）。
  - 侧边栏顶部站点 Logo 文字不再是可点击链接，改为纯文本。

### 功能点 8：顶部导航新增"下载工具"链接与"联系客服"二维码弹窗
- 涉及文件：`frontend/src/components/layout/AppHeader.vue`、`frontend/src/components/common/SupportQRCodesButton.vue`（新文件）
- 描述：
  - 顶部导航栏新增"下载工具"链接（管理员在系统设置里配置了 `download_tools_url` 才会显示），点击在新标签页打开。
  - 新增"联系客服"按钮（原来只是用户下拉菜单里一行纯文字联系方式），点击弹出对话框展示一张或多张客服二维码图片（每张可配一段说明文字），二维码由管理员在系统设置里配置；如果没配二维码则回退显示旧的纯文本联系方式。
  - 顶部余额展示简化为一个 `$金额` 数字；upstream 版本悬停余额有"可用/冻结/总余额"三行明细的浮层，这个 fork 里**没有**这个浮层（与本 fork 不引入"冻结余额"概念一致，属于相对上游的简化/缺失，迁移时需要留意）。

### 功能点 9：API Key 明文改为"按需揭示"，配合内联 TOTP 二次验证
- 涉及文件：`frontend/src/api/keys.ts`、`frontend/src/api/client.ts`、`frontend/src/views/user/KeysView.vue`
- 描述：API Key 列表/详情接口不再直接返回明文，用户点击"复制"“查看用法”“导入到 CCS”等需要真实 key 值的操作时，前端会调用新的 `GET /keys/{id}/value` 单独取值。如果账号开启了 TOTP 两步验证，这次取值请求可能返回"需要验证码"（不会像普通 401 那样被强制退出登录），此时会弹出一个内联的 6 位验证码输入框，输入正确后才能拿到 key 明文并继续原来的操作（复制/查看用法/导入）。

### 功能点 10：多端登录会话架构调整（跨标签页协调刷新，支付页不强制退出）
- 涉及文件：`frontend/src/api/client.ts`、`frontend/src/stores/auth.ts`
- 描述：Access Token 不再写入 `localStorage`（改为仅存内存），Refresh Token 变成 HttpOnly Cookie，通过 `/auth/refresh` 换取；多个标签页同时需要刷新令牌时，通过浏览器 Web Locks API（或退化到 localStorage 锁）协调，避免同时发出多次刷新请求。用户能感知到的效果：在支付结果页 (`/payment/result`) 或公开订单查询页遇到 401 不会被强制跳转登录页（避免支付完成后被误踢下线）。

### 功能点 11：账号相关公共组件全面接入 Kiro / Sora 品牌色与图标
- 涉及文件：`frontend/src/components/common/platformBrandIcons.ts`（新文件）、`GroupBadge.vue`、`GroupOptionItem.vue`、`PlatformIcon.vue`、`PlatformTypeBadge.vue`、`ModelIcon.vue`、`frontend/src/utils/platformColors.ts`
- 描述：分组徽章、平台图标、模型图标等所有展示"平台"的地方新增 `kiro`（青色）和 `sora`（玫红色）两种配色和图标；Grok 图标换成真实的 xAI 官方标志（原来是占位路径）。`PlatformTypeBadge` 里 Gemini 套餐徽章新增 Free/Pro/Ultra/AI Studio Free/AI Studio 按量付费/GCP Standard/GCP Enterprise 的分类展示；OpenAI 团队版账号且 `organization_role` 为 owner/admin/leader 时，账号徽章旁会多一个紫色"Team Leader"小标签。*（注：此组件文件本身存在未解决的合并冲突，见文首提醒）*

### 功能点 12：账号/批量编辑新增"顺序轮转代理"选项
- 涉及文件：`frontend/src/components/common/ProxyRotationSelector.vue`（新文件）、`frontend/src/components/common/ProxySelector.vue`
- 描述：代理选择下拉框新增一个特殊选项"顺序轮转"，选中后下方展开一个可搜索、可勾选多个代理的列表，已选代理显示为带序号的列表，支持上移/下移调整顺序、单独移除，并显示"已选择 N 个代理，将按上方顺序循环分配"的提示。原来一个账号/一批账号只能绑定一个固定代理，现在可以设置成按顺序轮流使用一组代理。

### 功能点 13：CLI 使用指南弹窗新增 Kiro 标签，重写 Grok/Codex 配置模板
- 涉及文件：`frontend/src/components/keys/UseKeyModal.vue`
- 描述：API Key 详情页"如何使用"弹窗新增 Kiro 标签（生成 Anthropic 格式的 CLI 配置）；Grok/Codex 相关标签生成的 `config.toml`/环境变量示例统一改用 "sub2api" 作为自定义 provider 名称（原来是零散命名或直接叫 "OpenAI"），并区分了 WebSocket 与非 WebSocket 两套 Codex 配置模板，新增 `grok-4.20-multi-agent-0309` 模型条目。

### 功能点 14：管理员公告编辑器支持直接插入图片
- 涉及文件：`frontend/src/views/admin/AnnouncementsView.vue`
- 描述：新建/编辑公告表单的正文输入框下方新增"插入图片"按钮，选择一张或多张图片后自动上传并在光标位置插入 `![文件名](图片地址)` 格式的 Markdown，上传期间显示"正在上传"提示。

### 功能点 15：用户自定义页面（CustomPageView）支持需要登录态才能加载的图片
- 涉及文件：`frontend/src/views/user/CustomPageView.vue`
- 描述：管理员配置的自定义菜单 Markdown 页面里，如果引用了需要携带登录凭证才能访问的图片（`/api/v1/pages/.../images/...`），页面渲染后会自动带上 Authorization 头重新拉取这些图片并替换成本地 Blob URL 显示，而不是直接显示成裂图（之前图片走的是普通 `<img src>`，没有凭证会加载失败）。

### 功能点 16：简易模式（RUN_MODE=simple）路由限制统一为一份显式清单
- 涉及文件：`frontend/src/navigation/simpleMode.ts`（新文件）、`frontend/src/router/index.ts`
- 描述：简易模式下会被隐藏/拦截跳转回首页的页面范围明显扩大且更规范：用量、工单、可用渠道、订阅/购买/订单/兑换码/推荐返利，以及管理端的用户/分组/渠道/订阅/AI 治理/技能治理/风控/兑换码/优惠码/返利/订单管理，全部列在一份统一的前缀清单里（此前只有 4-5 个零散路径）。

### 功能点 17：Settings 页面新增多个功能开关卡片（无独立入口，均在"系统设置"内）
- 涉及文件：`frontend/src/views/admin/SettingsView.vue`、`frontend/src/components/admin/PlatformDefaultAccountModelConfigForm.vue`（新文件）
- 描述：系统设置页仍是原来的 9 个 Tab（通用/协议/功能/安全/用户/网关/支付/邮件/备份），但内容新增：
  - **功能 Tab**："AI 创作中心"整卡——总开关之后可分别开关对话/图片/生视频/视频创作/音频/Realtime 六种能力，每种能力可单独多选允许使用的分组；还有"过期后保留天数"、"计费时区"下拉（上海/东京/新加坡/香港/伦敦/柏林/纽约/洛杉矶/芝加哥/悉尼）、以及一段"OpenAI Live ICE/TURN"WebRTC 配置的 JSON 文本框。
  - **通用（站点）Tab**：新增"支持二维码"可增删列表编辑器（图片+备注），对应前面提到的顶部"联系客服"弹窗数据来源。
  - **网关 Tab**：新增"Kiro Runtime"卡片（版本号/commit/系统版本/Node 版本/沙箱执行命令，以及提示缓存命中率百分比、最小分块 tokens、独立/前缀 TTL 秒数）；新增"临时不可调度触发阈值"卡片（全局默认"M 分钟内累计 N 次错误"才标记临时不可调度）；新增"OpenAI API Key 健康熔断"卡片（窗口分钟数/失败阈值/冷却分钟数，自动临时禁用异常的 OpenAI API Key 账号）；新增"OpenAI 过载同号重试"卡片（对"模型已满载"“服务器过载"类错误优先在同一账号重试 N 次再切号）；并嵌入了新的 `PlatformDefaultAccountModelConfigForm` 组件——按平台（Anthropic/OpenAI/Gemini/Antigravity/Kiro/Grok）Tab 切换，统一配置模型白名单、模型映射、精简模型映射、Kiro 按订阅档位的独立覆盖（JSON）、临时不可调度规则（含一键预设按钮）、自定义错误码，取代了原来散落在各处的部分配置。

### 功能点 18：相对上游的功能缺失/简化（迁移取舍参考）
- 人机验证：管理设置里的"腾讯天御验证码"“阿里云验证码 2.0"两个服务商选项被整体移除，只保留 Cloudflare Turnstile。
- 首页"简洁首页"展示开关被移除。
- OpenAI Codex 客户端版本"自动同步"功能（每 6 小时抓取官方最新稳定版并固定 User-Agent/版本头）被移除，替换成一个静态可填的 User-Agent 字符串 + 通用的按平台"反封号"开关表。
- Grok 视频生成、Codex 网页搜索的计费方式从"按分组倍率叠加"改为"管理员填的就是最终价格，不再叠加分组倍率"。
- 登录/注册密码最小长度从 6 位提高到 8 位。
- 分组/渠道定价区域的"利润控制"（最低毛利率%+安全缓冲%，用于排除会亏本的账号）整块功能被移除。

### 无新增可感知功能的文件
以下文件在本次两个能力范围内主要是构建工具配置、纯内部重构、i18n key 清理、bug 修复或纯支撑代码，没有产生新的、用户/管理员可直接感知的界面行为：
- `frontend/audit.json`、`frontend/pnpm-workspace.yaml`、`frontend/vitest.config.ts`：依赖/构建工具配置调整。
- `frontend/src/App.vue`：新增 AI Studio store 的登出重置、语言切换时刷新页面标题、启动时强制刷新一次公共设置——纯内部初始化逻辑调整。
- `frontend/src/api/admin/dashboard.ts`、`frontend/src/api/admin/index.ts`、`frontend/src/api/admin/settings.ts`、`frontend/src/api/index.ts`、`frontend/src/api/url.ts`：为上述功能点（消耗拆分、AI Studio 设置、网关 URL 拼接修复等）提供的接口/类型定义层，本身不构成独立界面功能。
- `frontend/src/components/Guide/steps.ts`：仅把类型导入方式从值导入改成 `type` 导入，无行为变化。
- `frontend/src/components/charts/EndpointDistributionChart.vue`、`GroupDistributionChart.vue`、`ModelDistributionChart.vue`：修复了切换筛选条件/展开明细行时的竞态条件与空值展示 bug，交互本身（点击展开用户明细）没有变化。
- `frontend/src/components/common/AnnouncementPopup.vue`：Markdown 渲染器改为按需异步加载（性能优化），展示效果不变。
- `frontend/src/components/common/BaseDialog.vue`、`HelpTooltip.vue`、`NavigationProgress.vue`、`Pagination.vue`、`Select.vue`、`Toast.vue`：仅把硬编码的英文 `aria-label`/提示文案改成走 i18n，无障碍属性文案而已，视觉和交互不变。
- `frontend/src/components/common/DataTable.vue`：修复表头/固定列 z-index 遮挡全局弹窗的样式 bug，勾选列不再被误算进"数据列"。
- `frontend/src/composables/useAutoRefresh.ts`：默认刷新间隔支持响应式绑定管理员配置值（内部实现变化，用户看到的仍是同一组刷新间隔选项）。
- `frontend/src/composables/useOnboardingTour.ts`：新手引导库改为按需异步加载，减少首屏体积，引导流程本身不变。
- `frontend/src/composables/useTableLoader.ts`：把 vueuse 的防抖函数换成自实现版本，行为等价。
- `frontend/src/i18n/index.ts`：locale 文件合并逻辑与开发环境下的 key 冲突告警，纯内部机制。
- `frontend/src/i18n/locales/{en,zh}/admin/{accounts,channels,overview,settings}.ts`、`{en,zh}/common.ts`、`{en,zh}/dashboard.ts`：以上各功能点对应的文案 key，本身不是独立功能（内容已体现在具体功能点描述里）。
- `frontend/src/router/index.ts`、`router/meta.d.ts`：为上述新增路由（创作台/技能中心/工单/发票申请等）和 Gemini 项目 ID 恢复步骤提供路由与守卫逻辑，界面表现已在对应功能点里说明。
- `frontend/src/stores/README.md`：纯文档更新。
- `frontend/src/stores/adminCompliance.ts`、`adminSettings.ts`、`app.ts`、`auth.ts`、`index.ts`：为上述具体功能点（合规弹窗状态机、平台默认配置缓存、公共设置缓存、登录会话架构）提供的状态管理实现，行为已在对应功能点描述。
- `frontend/src/types/index.ts`：全局 TypeScript 类型定义调整，为以上功能点提供数据结构支撑，本身不是界面功能（该文件存在未解决合并冲突，见文首提醒）。
- `frontend/src/utils/branding.ts`：站点 Logo 为空时的 favicon 回退逻辑微调。
- `frontend/src/utils/embedded-url.ts`：内嵌 iframe 页面 URL 不再携带认证 token 查询参数（安全加固，非可见功能）。
- `frontend/src/utils/featureFlags.ts`：新增 AI Studio/工单等功能开关的判定函数，供上述侧边栏/路由功能点调用。
- `frontend/src/utils/i18n.ts`：新增状态/平台/支付方式等到 i18n key 的映射工具函数，纯代码复用。
- `frontend/src/utils/safeImageUrl.ts`、`sanitize.ts`、`url.ts`：图片地址/HTML/重定向路径的安全校验工具函数（防 XSS、开放重定向），不改变正常使用时的界面表现。
- `frontend/src/views/HomeView.vue`：首页自定义 HTML 内容改为经过 DOMPurify 消毒后再渲染，iframe 模式增加 sandbox 限制属性——安全加固，正常内容展示效果不变。
- `frontend/src/views/KeyUsageView.vue`：修复用量环形图切换日期范围时的动画竞态 bug，视觉效果不变。
- `frontend/src/views/NotFoundView.vue`：把硬编码英文文案换成 i18n，文案本身翻译对应即可，无行为变化。

---

## 能力：账号管理界面（授权 / 指纹 / 容量）（对应 cap id 9）

### 功能点 1：新增 Kiro 平台完整授权向导（OAuth 多步流程 + 直接填 API Key）
- 涉及文件：`frontend/src/api/admin/kiro.ts`（新文件）、`frontend/src/composables/useKiroOAuth.ts`（新文件）、`frontend/src/components/account/KiroAuthorizationFlow.vue`（新文件，1071 行）、`KiroDiagnosticChips.vue`（新文件）、`CreateAccountModal.vue`、`ReAuthAccountModal.vue`
- 描述：后台"添加账号"新增 Kiro 平台，账号类型可选"OAuth"或"API Key"两种（类似 Antigravity 的 OAuth/Upstream 二选一）：
  - **API Key 方式**：直接粘贴 API Key，附带 region/auth_region 等字段。
  - **OAuth 方式**：又分两种输入模式——"手动 OAuth 授权"（生成授权链接 → 浏览器完成登录 → 粘贴回调 URL，链接可一键复制）和"手动填写 Refresh Token"（直接粘贴已有 refresh token，支持一次粘贴多行批量建号，并可选认证方式：社交登录/IDC/外部身份提供商，各自展开对应的 client_id/密钥/issuer_url/scopes 等字段）。
  - 展开"高级字段"可看到并填写 region/auth_region/api_region/Profile ARN/机器 ID；Profile ARN 旁有"发现 Profile"按钮，能查询该凭据在 AWS 下可用的 Profile 列表并从下拉框选择，不用凭空手填 ARN。重新授权时会用一个小徽章提示当前 Profile 是"自动"还是"手动固定"状态。
  - 如果授权过程中触发了 AWS Identity Center 设备码流程，会弹出一个琥珀色面板：展示一个大号可复制的用户验证码、"打开验证页面"链接、剩余有效时间倒计时和等待动画，并提供"取消"按钮。
  - 如果触发的是外部身份提供商（如 Microsoft）授权，则弹出蓝色面板展示另一个授权链接及其 client_id/redirect_uri/issuer/scopes 等信息。
  - Kiro 账号行/编辑器上会显示一组小徽章（诊断信息）：Profile 模式（自动/手动）、Profile ID、登录方式、状态原因，不用打开编辑器也能大致看出账号健康状况。

### 功能点 2：Codex "邀请好友重置额度"管理弹窗 + 表格内快捷操作
- 涉及文件：`frontend/src/api/admin/accounts.ts`（`getCodexInviteResetStatus`/`sendCodexInviteResetInvite`/`consumeCodexInviteReset`/`getCodexInviteResetHistory`）、`frontend/src/components/account/CodexInviteResetModal.vue`（新文件）、`frontend/src/components/account/AccountUsageCell.vue`
- 描述：OpenAI OAuth 账号新增一个操作，打开"邀请重置"弹窗可看到：当前可用的重置次数、资格规则说明、是否需要用户同意、待处理/已兑换的额度列表（附带被邀请人的用户 ID/头像）。管理员可以在这里直接输入一个或多个邮箱发送邀请（生成新的重置额度），再选一条额度点击"消耗"来立即重置该账号的用量窗口。展开"历史记录"可看到该账号所有邀请/消耗操作的分页列表（操作人、邮箱、成功/失败、消息、时间）。账号列表表格里也直接嵌了一个精简版：一个显示可用次数的徽章、"查询"按钮和"重置"按钮，不用打开弹窗即可操作。

### 功能点 3：账号级 TLS 指纹策略——精细化到"操作系统 × 客户端类型 × 传输方式"矩阵
- 涉及文件：`frontend/src/api/admin/tlsFingerprintPolicy.ts`（新文件）、`frontend/src/components/account/TLSFingerprintBindingMatrix.vue`（新文件）、`TLSFingerprintPolicyModeControl.vue`（新文件）、`CreateAccountModal.vue`、`EditAccountModal.vue`、`BulkEditAccountModal.vue`
- 描述：Anthropic/OpenAI/Kiro/Grok 的 OAuth 或 API Key 账号（以及 Gemini/Antigravity）在新建、编辑、批量编辑三个地方都能配置账号自己的 TLS 指纹策略：
  - **模式**：关闭 / 平台默认 / 高级（三态分段开关）。
  - 高级模式下可选一个具体**指纹模板**或"随机"，也可选一个**指纹路由**（见功能点 9）。
  - **默认操作系统**（Windows/macOS/Linux/不设置）、**回退方式**（原生传输 vs 内置兼容模板）。
  - **绑定矩阵**：逐行添加"操作系统 × 客户端类型 × 传输方式（HTTP/1.1、HTTP/2、WebSocket-H1、WebSocket-H2）"组合，每一行单独选一个指纹模板（或随机），行与行的维度组合重复会标红提示"重复绑定会被忽略"。
  - 提供"预览"能力：输入一种传输方式/模拟 User-Agent，实时展示会命中哪个模板/路由、最终发往上游的 User-Agent/Originator，以及命中原因；没有可用模板会明确提示"当前策略与传输方式没有可用的 TLS 指纹模板"。

### 功能点 4：批量编辑账号从"允许跨平台"改为"必须锁定单一平台"，并新增多个批量开关
- 涉及文件：`frontend/src/components/account/BulkEditAccountModal.vue`
- 描述：以前跨平台批量编辑账号只是弹出黄色警告仍可继续；现在**只要当前筛选/勾选跨了一个以上平台，整个弹窗就会提示"批量编辑必须明确限定到单一平台"并禁用保存按钮**，必须先把筛选条件缩小到一个平台。同一批新增了三个批量开关/功能：OpenAI 的"允许生图"开关（取代已移除的"摊平 Codex namespace 工具"兼容开关）、"Text 端点自动路由"开关、以及可对多个账号一次性套用的 TLS 指纹策略配置（见功能点 3）；代理选择支持前面提到的"顺序轮转"多代理配置。

### 功能点 5：管理端 TLS 指纹模板库大幅扩充可调项，支持批量导入抓包结果
- 涉及文件：`frontend/src/api/admin/tlsFingerprintProfile.ts`、`frontend/src/components/admin/TLSFingerprintProfilesModal.vue`（整体重写）
- 描述："TLS 指纹模板"管理弹窗从"表格+独立弹窗表单+粘贴 YAML"改成了搜索/平台过滤列表 + 内联创建/编辑表单（不再弹二级弹窗），可调字段大幅增加：
  - 分类信息：平台、传输方式（HTTP/1.1、HTTP/2、WebSocket-H1、WebSocket-H2）、操作系统、客户端类型、质量状态（未验证/已验证/已拒绝/已弃用）、来源类型（手动/抓包/导入）、验证时间、客户端版本范围。
  - 发往上游的身份信息：User-Agent、Originator、HTTP/2 指纹字符串，以及一个结构化的"HTTP Header 模板"（显式头部顺序 + 每个头部允许的取值），让整个 HTTP 请求的"形态"而不只是 TLS 层能被一致地伪装。
  - 底层 TLS 参数：GREASE 开关、加密套件、曲线、点格式、签名算法（含单独的证书签名算法）、ALPN 协议、支持的协议版本、密钥交换组、PSK 模式、扩展列表、扩展负载（原始字节）、证书压缩算法、委托凭据算法、应用层设置协议。
  - 新增"导入抓包结果"功能：选择平台后粘贴一个或一组抓包生成的模板 JSON，批量导入并报告成功导入数/重复跳过数（按指纹哈希去重）。

### 功能点 6：新增独立的"TLS 指纹路由"实体 + 覆盖率仪表盘
- 涉及文件：`frontend/src/api/admin/tlsFingerprintRouter.ts`（新文件）、`frontend/src/components/admin/TLSFingerprintRoutersModal.vue`（新文件）、`frontend/src/views/admin/AccountsView.vue`
- 描述：账号池"⋮ 工具"下拉菜单新增"TLS 指纹路由"入口。打开后：
  - 顶部是一张**覆盖率**汇总表，按平台展示：总账号数、已开启指纹的账号数、其中可调度的数量、有效绑定数、"降级"（配置有问题/匹配不到）账号数，以及服务启动以来"命中路由成功解析"与"回退默认"的实时计数——让管理员知道自己配的路由规则在真实流量里到底生效没生效。
  - 下方是路由 CRUD 列表，每个路由可一键启用/停用，显示规则条数徽章。新建/编辑一个路由时可添加任意数量规则：每条规则可单独启用、设置是否大小写敏感、按传输方式过滤（含旧版兼容值 + TLS 重放能力提示）、匹配方式（包含/前缀/精确/正则）、匹配模式、绑定目标模板，以及可选的操作系统/客户端类型维度覆盖、上游 User-Agent/Originator 覆盖。

### 功能点 7：账号池新增"OpenAI OAuth 分组容量"与"平台容量预测"两个大盘弹窗
- 涉及文件：`frontend/src/components/admin/account/OpenAIOAuthCapacityDialog.vue`（新文件）、`PlatformCapacityDialog.vue`（新文件）、`frontend/src/api/admin/accounts.ts`、`frontend/src/api/admin/capacity.ts`（新文件）、`frontend/src/views/admin/AccountsView.vue`
- 描述：
  - **"OAuth 容量"**（账号列表操作项）：可按"全部分组（去重）"或单个分组查看，顶部 KPI（OAuth 账号数、可调度数、错误数、限流中数）；一张 5h/7d 容量窗口表（已消耗/推算容量/可用额度/已用占比）；预计新增额度、预计周期消耗/缺口（附高/中/低置信度与组合速率）；一张"套餐与补号建议"表（每套餐 5h/7d 单号容量、备选补号建议数量——明确提示"各套餐互斥，勿相加"）；限流剩余时间分布条形统计；最近 3 小时 vs 前一天同时段消耗对比；一张每小时消耗/可用额度趋势图（实线=已消耗、虚线=预测、区分"已定格（持久化）"与"暂无可用推算"的小时状态）。
  - **"容量预测"**（账号列表操作项，独立的"容量预测"按钮）：按平台 + 分组筛选，KPI 卡片（当前可用额度、未来预测消耗、首次缺口时间、建议补充账号数）；一张消耗/可用额度趋势图，用阴影区域标出"预计缺口"时段；下方是容量事件时间线（恢复/缺口事件，附账号数、窗口、金额，缺口事件还带"建议补充 N 个账号"的具体建议）。
  - 该弹窗额外提供**"探测账号"**功能：向所选平台/分组下的 OAuth 账号发送一次轻量真实测试请求以确认账号是否真的可用（默认只探测限流中的账号，勾选"包含正常账号"可扩大范围），探测过程中显示进度提示，完成后给出汇总（探测数/正常数/限流数/失败数/耗时）及可展开的失败明细列表。

### 功能点 8：账号列表新增"Cyber 事件"标记与查看入口
- 涉及文件：`frontend/src/components/admin/account/AccountCyberEventsModal.vue`（新文件）、`frontend/src/api/admin/accounts.ts`（`getCyberEvents`、`include_cyber_summary` 参数）、`frontend/src/views/admin/AccountsView.vue`
- 描述：OpenAI OAuth 账号如果被上游判定过"cyber"风控事件，账号名称下方会出现一个红色"Cyber: N"小徽章（带盾牌图标），账号名称整行文字也会变红提示；点击徽章直接打开"Cyber 事件"弹窗，展示按 request_id 去重后的事件列表（时间、模型、HTTP 状态码、错误信息、request_id），支持分页与刷新，每条记录还可以点进一步查看详情。

### 功能点 9：账号列表表格顶部新增"容量预测"“一键启用超额"工具按钮
- 涉及文件：`frontend/src/views/admin/AccountsView.vue`
- 描述："创建账号"按钮旁新增"容量预测"按钮（打开功能点 7 的平台容量预测弹窗）；如果池子里有 Kiro 账号，还会出现"一键启用超额"按钮，点击后弹出确认框，确认后对所有 Kiro 账号批量开启"允许超额"。这两个按钮同时也复制在"⋮ 工具"下拉菜单里，工具菜单里还新增了"TLS 指纹路由"入口（功能点 6）。

### 功能点 10：账号使用量表格单元格按平台展示更丰富的信息
- 涉及文件：`frontend/src/components/account/AccountUsageCell.vue`
- 描述：账号列表里"使用情况"这一列，不同平台展示内容更细：
  - **Kiro OAuth**：新增"30d"配额进度条，下方一行紧凑文字显示"额度上限 / 超额情况 / 已用金额"。
  - **Grok OAuth**：显示套餐/资格徽章；有官方计费数据时显示 7d/30d 官方用量条并附对齐的本地请求数/Token数/费用统计；显示预付余额/月度限额/超额用量的组合信息行；免费套餐账号新增一条滚动 24 小时（或可配置窗口）免费额度条，达到管理员配置的"软限制"阈值后变成琥珀色提醒。
  - **Gemini**：遇到需要人工验证的"forbidden"状态时，直接给出"打开验证"链接和"复制链接"按钮，而不是只显示一段报错文字。

### 功能点 11：Grok 账号"探测"结果展示信息更丰富
- 涉及文件：`frontend/src/components/account/GrokQuotaProbeCell.vue`
- 描述：点击 Grok 账号旁的"探测"按钮后，结果不再只显示一个周消耗百分比，而是把请求数窗口、Token 窗口的"剩余/上限·重置时间"、重试等待倒计时、订阅档位、资格状态全部拼接展示在一行；探测按钮旁新增一个禁用状态的"重置不支持"按钮占位，明确告诉管理员 Grok 没有像 OpenAI Codex 那样的手动配额重置操作。

### 功能点 12：Gemini 账号配额徽章分类更细，附悬浮说明与官方文档链接
- 涉及文件：`frontend/src/components/account/AccountQuotaInfo.vue`
- 描述：账号列表里 Gemini 账号旁的配额小徽章现在能区分：Vertex AI 服务账号、Code Assist 的 GCP Standard/Enterprise、Google One 的 Free/Pro/Ultra、AI Studio 的 Free/按量付费，各自配色不同；鼠标悬停徽章旁的信息图标会弹出说明该档位适用的限流政策（渠道名称+限流描述），并附一个指向对应 Google 官方文档（Vertex 配额/Gemini Code Assist 配额/Gemini API 限流）的链接。

### 功能点 13：账号状态徽章的倒计时改为实时跳动
- 涉及文件：`frontend/src/components/account/AccountStatusIndicator.vue`、`frontend/src/components/admin/account/AccountActionMenu.vue`
- 描述：账号处于限流/过载/临时不可调度/单模型限流状态时，徽章上显示的"剩余时间"以前是打开页面那一刻的静态快照，现在会每秒刷新跳动（有倒计时才启动计时器，没有则不启动，避免大量账号同时挂计时器）；账号"⋮"操作菜单里"恢复状态"是否出现的判断，也从一次性时间戳改成了同一套实时时钟，避免菜单开着的时候状态其实已经变了但菜单没更新。

### 功能点 14：OAuth 账号创建/重新授权后自动生成账号名称
- 涉及文件：`frontend/src/utils/oauthAccountName.ts`（新文件）、`frontend/src/composables/useAccountOAuth.ts`、`useAntigravityOAuth.ts`、`useGeminiOAuth.ts`、`useGrokOAuth.ts`、`useKiroOAuth.ts`、`useOpenAIOAuth.ts`
- 描述：新建或重新授权任意平台的 OAuth 账号时，如果没有手动填名字，账号名称会自动根据授权返回的信息生成——通常是邮箱地址，再拼上平台特有的补充信息（例如 Antigravity 拼项目 ID/套餐；OpenAI 团队版拼工作区名称）。重新授权时，新拿到的凭据字段会与账号原有的凭据/额外信息做合并而不是整体覆盖，管理员之前手动填的地区/Profile ARN 等字段在重新授权后不会丢失。

### 功能点 15：Gemini 授权向导新增"缺少 Project ID"恢复步骤与资格问题引导
- 涉及文件：`frontend/src/components/account/OAuthAuthorizationFlow.vue`、`ReAuthAccountModal.vue`、`frontend/src/components/admin/account/ReAuthAccountModal.vue`
- 描述：创建/重新授权 Gemini Code Assist 账号时，如果后端提示需要手动指定 GCP Project ID（多种具体错误码），向导会多出第 4 步：一个琥珀色面板，附"如何获取 Project ID"的帮助链接和一个输入框，填完点"重新生成"即可继续，而不是只看到一段报错文字。另外新增两种失败引导：Google 账号"需要年龄验证"（说明与 GCP 无关，给出重试/换其他授权方式的步骤）和"套餐不符合资格"（说明当前 Google 套餐不满足要求），都给出具体的分步指引。授权前，向导顶部还有一个可展开的琥珀色提示，事先说明如何在 GCP 建项目并开启 Code Assist API。

### 功能点 16：账号连通性测试新增"图片测试路由"选择
- 涉及文件：`frontend/src/components/account/AccountTestModal.vue`
- 描述："测试账号连接"弹窗里，如果所选测试模型支持 OpenAI 图片生成，模型选择框下方会新增一个"图片测试路由"下拉框，管理员可以选择走哪条图片生成路由（对应分组配置的 Codex/Web2API/Native 三种路由）来测试，而不是固定走同一条路径。

### 功能点 17：管理员数据导入支持归档文件（.zip/.cpa）与去重策略
- 涉及文件：`frontend/src/api/admin/accounts.ts`（`importArchive`）、`frontend/src/components/admin/account/ImportDataModal.vue`
- 描述：账号池"导入数据"弹窗除了原来的 JSON 文件，现在还能选择一个 `.zip` 或 `.cpa` 归档文件（如果归档里混合了"标准备份对象"和"Kiro 账号数组"两种格式会被拒绝并给出明确提示）；新增"去重方式"下拉（不处理/覆盖已存在/忽略已存在），导入结果区域会分别展示 JSON 导入结果和归档导入结果（含每种子格式各自的创建/更新/跳过/失败数，以及归档内的解析错误列表）。

### 功能点 18：代理列表新增"测试全部匹配结果"“质量检测全部匹配结果"
- 涉及文件：`frontend/src/api/admin/proxies.ts`（`batchTestAllFiltered`/`batchQualityCheckAllFiltered`）、`frontend/src/views/admin/ProxiesView.vue`
- 描述：代理管理页面在没有勾选任何行的情况下点击"批量测试"/"批量质量检测"，会先弹出确认框（"测试全部 N 个匹配的代理？"），确认后由后端一次性对当前筛选条件下的所有代理执行检测并返回汇总结果（测试：总数/成功/失败；质量检测：总数/健康/警告/存在挑战/失败），不再需要前端分页拉取全部代理列表再逐个调用。

### 功能点 19：自定义错误码支持一键预设常见码，429/529 加二次确认
- 涉及文件：`frontend/src/components/account/CustomErrorCodesForm.vue`（新文件）
- 描述：账号编辑（及平台默认配置）里的"自定义错误码"支持点击预设按钮快速勾选常见 HTTP 状态码，也可手动输入任意 100-599 范围的码；如果勾选的是 429 或 529（这两个码会导致账号被停止调度），会先弹出浏览器确认框提示风险，确认后才真正加入。

### 功能点 20：账号级"响应改写规则"（把上游报错替换成自定义文案）
- 涉及文件：`frontend/src/components/account/responseRewriteRules.ts`（新逻辑）
- 描述：账号可以配置一组规则，按上游返回的 HTTP 状态码和/或关键词（"任一命中"或"全部命中"两种匹配模式）匹配后，把最终展示给使用者的错误消息替换成管理员自定义的文案，用于把技术性、可能暴露内部信息的上游报错改写成更友好/更安全的提示。

### 功能点 21：临时不可调度规则编辑器新增一键预设
- 涉及文件：`frontend/src/components/account/TempUnschedRulesForm.vue`（新文件）
- 描述：账号级和平台默认配置里的"临时不可调度"规则列表，新增一排"+ 预设"快捷按钮，点一下就能预填好一整条规则（错误码/关键词/持续分钟数/说明），覆盖常见场景：过载（529）、限流（429）、通用不可用（503），以及 4 个 OpenAI 专属预设（上游暂时不可用/上游请求失败/上游访问被拒绝/上游连接失败），减少手动逐字段填写。

### 功能点 22：Grok / OpenAI 相关规格变化（迁移取舍参考，非纯新增）
- Grok OAuth 重新授权的输入方式重新整理为"手动输入 SSO"（粘贴 SSO Token，自动完成授权并建号）和"邮箱密码输入"（按行输入 email----password，服务端登录后换成 OAuth 凭据，密码不落库），移除了原来的批量 SSO Cookie 导入（3 路并发转换）方式；重新授权（区别于批量新建）现在**只接受一条凭据**，多行输入会被拦截并提示。
- Gemini/Grok 的"OAuth 能力预检"接口被移除，向导不再提前查询服务器支持哪些 OAuth 选项，直接展示所有选项，出错交给后端处理。
- **需要人工确认的开放问题**：管理端和通用的两个"测试账号连接"弹窗（`components/admin/account/AccountTestModal.vue`、`components/account/AccountTestModal.vue`）里已经**完全没有 Grok 相关代码**——upstream 原有的 Grok 专属测试模式（文本/图片/视频/网页搜索/TTS/STT/Realtime 共 7 种，各自带上传字段和结果预览）在这两个文件里被整体移除，管理员测试 Grok 账号时只能走通用的默认/精简两态测试路径。但另一份迁移清单（cap 1，`backend/internal/service/account_test_service.go`）显示后端仍然实现了 Grok 多模态测试的完整能力——需要在迁移时确认这个能力入口是否挪到了本次未覆盖的其它文件，还是前后端确实出现了能力缺口。

### 无新增可感知功能的文件
以下文件本次改动主要是类型定义、内部工具函数、局部 bug 修复或小的文案调整，没有产生独立于上述功能点之外的、用户/管理员可感知的新界面行为：
- `frontend/src/api/admin/antigravity.ts`、`frontend/src/api/admin/gemini.ts`：为功能点 14/15 提供的类型/接口调整（新增 `plan_type`/`privacy_mode` 字段、移除能力预检接口），无独立界面。
- `frontend/src/api/admin/capacity.ts`：功能点 7"容量预测"弹窗的接口定义文件。
- `frontend/src/components/account/ModelMappingEditor.vue`：功能点 17 的通用"模型映射"编辑器组件（从→到两列表单），被多处复用，本身不是独立功能。
- `frontend/src/components/account/ModelWhitelistSelector.vue`：修复了"上游模型同步"按钮在 Grok 平台会必然报错的问题（Grok 后端不支持模型同步），从可用平台列表里移除了 Grok，属于 bug 修复。
- `frontend/src/components/account/UsageProgressBar.vue`：底层进度条组件新增 `cyan` 配色和"空数据也显示统计行"选项，供功能点 10 的 Kiro/Grok 展示复用。
- `frontend/src/components/account/credentialsBuilder.ts`：编辑账号时"预热请求"选项关闭后的凭据清理逻辑微调，无界面变化。
- `frontend/src/components/account/customErrorCodes.ts`、`tempUnschedRules.ts`、`textEndpointAutoRoute.ts`、`tlsFingerprintProfileOptions.ts`：分别为功能点 19/21/3/4 提供的纯逻辑辅助函数（校验、格式转换、平台适用性判断），无独立界面。
- `frontend/src/components/admin/account/AccountStatsModal.vue`：仅补了一个 `watch` 的 `immediate: true`，修复了首次打开可能不刷新数据的 bug。
- `frontend/src/components/admin/account/AccountTableFilters.vue`：平台筛选下拉新增 "Kiro" 选项，属于功能点 11（Kiro 全平台铺开）的延伸，无独立交互。
- `frontend/src/components/admin/account/ScheduledTestsPanel.vue`：把几处硬编码英文错误提示改成走 i18n，无行为变化。
- `frontend/src/composables/useGeminiOAuth.ts`、`useGrokOAuth.ts`、`useOpenAIOAuth.ts`：为功能点 14/15/22 提供的 OAuth 状态管理逻辑，界面表现已在对应功能点中说明。
- `frontend/src/composables/useModelWhitelist.ts`：更新了模型目录列表（新增 Kiro 模型族、刷新 Grok/Gemini 型号）和预设映射，供账号编辑器的"模型白名单/映射"选择框使用，属于数据更新而非交互变化（该文件存在未解决的合并冲突，见文首提醒）。
- `frontend/src/utils/accountConcurrency.ts`：一个"并发数至少为 1"的取值 clamp 工具函数。
- `frontend/src/utils/accountSelection.ts`：批量"全选所有匹配结果"翻页拉取逻辑新增了一个分页回调参数，供上层统计进度用，交互本身在功能点 4 已覆盖。
- `frontend/src/utils/accountUsageRefresh.ts`：新增 Gemini 账号使用量的缓存刷新 key 计算逻辑，纯内部缓存失效判断。
- `frontend/src/utils/geminiExtra.ts`、`geminiOAuthType.ts`：Gemini 账号凭据里判断"是 Google One 还是 Code Assist"“套餐档位归一化"的纯逻辑函数，供功能点 12/15 复用。
