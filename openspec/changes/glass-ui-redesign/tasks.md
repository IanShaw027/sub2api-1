## 0. 止血与基线（单代理串行，先于一切）

- [x] 0.1 修复 `views/admin/RedeemView.vue`：删除第 504–1081 行残留的旧模板，为新模板补齐 script 中缺失的 `summaryChips / filters / applyStatusChip / filterTypeOptions / filterStatusOptions / reloadFromFirstPage / handleSearchCommit / searchQuery / selectedCount / batchUpdating / openBatchUpdateDialog / handleExportCodes` 等成员；验证 `vue-tsc --noEmit` 对该文件 0 错误、`vitest run src/views/admin/__tests__/RedeemView*` 通过、`/admin/redeem` 截图非白屏
- [x] 0.2 修复 `views/user/KeysView.vue`：为新模板补齐 `openCcsImportPicker / endpointCards / showMobileFilters / statusSegmentOptions / activeSortLabel / hasIpRestriction / isKeyRevealed / toggleKeyReveal / formatCost / quotaBarClass / hasRateLimitUsage / rowMenuKey / runRowAction` 等成员并把 `handleDelete / resetQuotaUsed / resetRateLimitUsage / formatResetTime` 重新接回模板（行菜单）；验证 tsc 0 错误、`KeysView.spec.ts` 9 条用例通过、`/keys` 截图非白屏
- [x] 0.3 修复 `views/admin/TicketDetailView.vue`：补 `goBack / statusActionOptions / handleStatusSelect`；验证 tsc 0 错误、截图正常
- [x] 0.4 确认 `i18n/locales/zh/misc.ts` 重复键已删除，运行 `node scripts/i18n-diff.mjs`（任务 15.2 提供；此处先用临时脚本）验证 zh/en 键差集为 0
- [x] 0.5 字体自托管：下载 Manrope 600/700/800、Inter 400/500/600/700、JetBrains Mono 400/500 的 woff2 到 `public/fonts/`，在 `src/styles/fonts.css` 写 `@font-face`（`font-display: swap`），`index.html` 移除 Google Fonts 链接、只 preload Manrope 800 与 Inter 400；验证 `designTokens.spec.ts` 通过、离线加载 `/home` 标题为 Manrope、总字体体积 ≤ 400KB
- [x] 0.6 修复其余失败测试到全绿：`HomeView.compact.spec`（5）、`AccountsView.bulkEdit / sparkShadow / usageWindowsHint`（4）、`TencentCaptchaActionGate`（2）、`PaymentMethodSelector`（2）、`AffiliateView`（1）、`BaseDialog`（1）、`TablePageLayout`（1）；只更新合法变化的选择器，不删除断言；验证 `vitest run` 0 失败
- [x] 0.7 `vue-tsc --noEmit` 0 错误、`vitest run` 0 失败、`npm run build` 成功；提交 `chore(glass): stabilize tree after interrupted redesign run (tasks 0.1–0.7)`
- [x] 0.8 把原型渲染脚本固化为 `frontend/scripts/ui/proto-shots.cjs`（输入 `.dc.html` 路径，输出每个画板 PNG，亮 / 暗两套），生成 `openspec/changes/glass-ui-redesign/reference/{light,dark}/*.png`；验证 9 个画板 × 2 主题 = 18 张图存在
- [x] 0.9 固化截图工具 `frontend/scripts/ui/shot.cjs`（+ `scripts/ui/dev-preview.sh`）（封装 mock 后端 + Vite + CDP 截图；参数 route / w / h / role / theme / locale / full）与 `frontend/scripts/mock/server.js`；验证 `bash scripts/ui-shots.sh /admin/accounts 1440 1000 admin dark zh` 产出图片

## 1. 令牌与主题（lead 单代理）

- [x] 1.1 逐值核对 `styles/tokens.css` 亮 / 暗两组与 `ui-standards.md §1` 一致（含 `--code-bg --shadow --shadow-hover --shadow-pop --btn-hi --field-shadow --thumb`）；验证 `designTokens.spec.ts` 新增断言读取 computed style 并通过
- [x] 1.2 增加 `html[data-accent=sky|indigo|teal|violet]` 覆盖，默认 blue；验证设置 `data-accent="teal"` 后 `--accent` 变化且按钮 / 焦点环随之变化（结构测试）
- [x] 1.3 核对 `--display / --font-body / --font-mono` 字体栈与自托管 `@font-face` 对应；验证 `document.fonts.check('800 24px Manrope')` 为 true
- [x] 1.4 核对 `tailwind.config.js` 调色板 → 令牌映射（red/rose/orange/amber/yellow → danger/warning，emerald/green/teal → success，blue/indigo/sky/violet/purple → accent，gray/slate/zinc/neutral → surface/muted/border），确认 `<alpha-value>` 透明度修饰符可用；验证 `bg-red-50/50 text-emerald-600 border-blue-500` 编译产物含 `color-mix(...var(--danger)...)`
- [x] 1.5 定义并核对 `--bg-public`（48px 网格 + 三处径向光）与 `--bg-workspace`（仅环境光）；验证 `/login` 与 `/admin/accounts` 截图背景差异符合 spec
- [x] 1.6 全局微交互：`::selection`、`.btn:active scale(.98)`、滚动条 hover 显示、焦点环 3px 主色 18%、`prefers-reduced-motion` 关闭动画；验证 `style.css` 中对应规则存在且键盘聚焦字段截图有焦点环
- [x] 1.7 提交 `feat(glass): tokens verified against prototype (tasks 1.1–1.6)`

## 2. 全局类与 ui 组件库核对（2 个代理：A 按钮/字段/选择/开关/徽章/标签；B 卡片/表格/分页/弹层/反馈）

- [x] 2.1 `.btn*` 与 `ui/Button.vue`：变体 × 尺寸矩阵与 spec 一致（34/32/42/26、圆角 10/9/12/8、阴影、hover/active/disabled/loading）；验证 `designSystem.structure.spec.ts` 新增尺寸断言通过、组件预览截图对照原型 07
- [x] 2.2 `.field .input-lg .input-error` 与 `ui/TextInput.vue`、`ui/FieldLabel.vue`：36/40、圆角 12、`--field-shadow`、focus/error 外环、前后缀图标、文本域 96；验证结构测试 + 截图
- [x] 2.3 `ui/UiSelect.vue` + `.filter-pill` + `.dropdown*`：触发器 36、pill 变体、面板 92% 玻璃 + blur、选项 36、选中态对勾、搜索行 32、键盘导航；验证 `UiSelect.spec` 与截图
- [x] 2.4 `ui/ToggleSwitch.vue`（36×20 / 32×18）、`ui/Checkbox.vue`（16 r5，indeterminate）、`ui/SegmentedControl.vue`（36 / 30，键盘 ←→）；验证结构测试与截图
- [x] 2.5 `ui/StatusBadge.vue` + `.badge* .tag* .count-badge .chip*`：22px、tone 五档、圆点、pulse；验证结构测试与暗色对比度 ≥ 4.5:1（`scripts/check-contrast.js` 扩展）
- [x] 2.1a `Button` 尺寸命名对齐规范（当前 默认=34、`md`=42、新增 `xs`=26 / `sm`=32）：把 `md` 改为 34、新增 `lg`=42，并更新唯一调用方 `ForgotPasswordView.vue` 与 `Button.spec.ts`；验证结构测试通过
- [x] 2.6 `ui/GlassCard.vue` + `.glass-card* .glass-ring .glass-inset .card-header/title/subtitle/body/footer`：72% 玻璃、圆角 14、变体、hover 阴影、禁止嵌套；验证结构测试与截图
- [x] 2.7 `ui/StatCard.vue`、`ui/MiniStatCard.vue`、`.summary-chip*`、`ui/EndpointCard.vue`、`ui/ProgressBar.vue`：尺寸与 spec 一致，sparkline 插槽，阈值配色；验证结构测试与截图
- [x] 2.8 `ui/PageHeader.vue`（compact / hero，<768 标题 20）、`ui/FilterBar.vue`（插槽、右侧元信息、<768 折叠为 44px 按钮 + 抽屉）、`ui/SettingsSection.vue` / `ui/SettingRow.vue`（240px 网格、14 20、单列断点）；验证结构测试与截图
- [x] 2.9 `ui/UiModal.vue`、`ui/UiDrawer.vue`、`common/ConfirmDialog.vue`、`common/BaseDialog.vue`（别名）：宽度档位、头尾结构、动画 160ms、焦点陷阱、Esc、移动端贴底；验证 `UiModal.spec` / `BaseDialog.spec` 通过、390 截图贴底
- [x] 2.10 `.toast* .notice-* .empty-state* .skeleton .spinner .tooltip-bubble .code .code-block .log-block .kbd .divider`：与 spec 尺寸一致；验证结构测试与截图
- [x] 2.11 `ui/Fab.vue`（52 r16）、`ui/ChipScroller.vue`、`ui/ListFade.vue`、`ui/UiPagination.vue`（28px、当前页主色）；验证结构测试
- [x] 2.12a 把各视图各自复制的 `.summary-row / .filter-row / .filter-search / .filter-count` scoped 布局类提升为 `style.css` 全局类（5 列 → 3 列 → 1 列断点、260px 搜索、右侧元信息），并在 RedeemView / TicketsView / AdminOrdersView 中改用；验证三页截图不变
- [x] 2.12 新建 5 个布局骨架 `components/layout/{PublicPageLayout,DashboardPageLayout,DetailPageLayout,SettingsPageLayout}.vue` 并把 `TablePageLayout.vue` 对齐 ListPage 配方（内容区 8/24/24/20、gap 14、桌面固定头 + 滚动表格、移动端卡片流）；验证 `TablePageLayout.spec` 通过、每个骨架有结构测试
- [x] 2.13 编写 `components/ui/README.md` 组件 → 尺寸 → 使用页面 映射（同原型组件映射表）；验证与 `ui-standards.md §4` 一致
- [x] 2.14 提交 `feat(glass): ui primitives aligned to component spec (tasks 2.1–2.13)`

## 3. common 组件（2 个代理：A 表格族；B 其余）

- [x] 3.1 `common/DataTable.vue`：表头 42 / 行 58–60 / hover 左轨 / sticky 列背景跟随 / 排序图标 / 复选外观 / 骨架行 / 卡内空态 / 横向阴影 / 滚动条；验证 `DataTable*.spec` 通过、`/admin/users` 截图
- [x] 3.2 `common/Pagination.vue`：容器 10 16、文案数字加粗、28px 项、每页数 pill、移动端按钮；验证 spec 与截图
- [x] 3.3 新建统一单元格组件 `components/common/cells/{NameIdCell,PlatformCell,TypeTagCell,StatusCell,UsageWindowCell,TodayStatsCell,MonoCell,TimeCell,ActionsCell}.vue`（改造自 `components/account/*Cell.vue` 与 `AccountTableActions.vue` 的可复用部分）；验证各有结构测试与 story 式截图
- [x] 3.4 `common/Select.vue`（含 pill 变体）、`SearchInput.vue`、`Input.vue`、`TextArea.vue`、`Toggle.vue`、`DateRangePicker.vue`、`ProxySelector.vue`、`ProxyRotationSelector.vue`、`GroupSelector.vue`、`GroupOptionItem.vue`：`.field` 36 配方；验证各 spec 与截图
- [x] 3.5 `common/StatusBadge.vue`、`GroupBadge.vue`、`GroupCapacityBadge.vue`、`PlatformTypeBadge.vue`、`HelpTooltip.vue`、`StatCard.vue`（改为 `ui/StatCard` 的薄包装或删除并迁移引用）；验证 spec 与截图
- [x] 3.6 `common/Toast.vue`、`EmptyState.vue`、`Skeleton.vue`、`LoadingSpinner.vue`、`NavigationProgress.vue`、`AnnouncementBell.vue`、`AnnouncementPopup.vue`、`ExportProgressDialog.vue`、`ImageUpload.vue`、`IpGeoBatchToolbar.vue`、`IpGeoCell.vue`、`MonitorQuotaView.vue`、`SubscriptionProgressMini.vue`、`AutoRefreshButton.vue`、`SupportQRCodesButton.vue`、`LocaleSwitcher.vue`、`VersionBadge.vue`：清除遗留类、对齐尺寸；验证 `vitest run src/components/common` 全绿、`lint:ui` 对目录 0 命中
- [x] 3.7 更新 `components/common/README.md`；提交 `feat(glass): common components (tasks 3.1–3.6)`

## 4. 布局壳（1 个代理）

- [x] 4.1 `AppSidebar.vue`：对照 `Sidebar.dc.html` 与参考图核对分类、28px 项、激活态配方、折叠 60px、底部主题 / 用户块不遮挡导航末项（当前 `/admin/orders` 截图有遮挡）；验证 1440 / 900 / 390 截图 + `AppSidebar.spec`
- [x] 4.2 `AppHeader.vue`：面包屑、⌘K 搜索、图标按钮 34、余额药丸、用户菜单 `.dropdown`；验证截图 + spec
- [x] 4.3 `MobileDrawer.vue`、`CommandPalette.vue`、`AppLayout.vue`：抽屉与命令面板走 `UiDrawer` / 弹层配方；验证 390 截图 + spec
- [x] 4.4 `AuthLayout.vue` 与 `PublicPageLayout.vue`：品牌面板组件化（登录 / 注册 / 忘记密码共用）；验证结构测试
- [x] 4.5 `styles/onboarding.css`、`announcement-markdown.css`：令牌化；验证 `lint:ui`
- [x] 4.6 提交 `feat(glass): app shell (tasks 4.1–4.5)`

## 5. 公开页（2 个代理：A 首页像素 diff；B 其余公开页）

- [x] 5.1 首页像素 diff：用 `scripts/pixel-diff.mjs`（pixelmatch）对比 `reference/light/01_首页.png` 与 `/home` 1440 截图，输出差异热图；逐区块修正：导航（登录 / 立即开始按钮尺寸与间距；语言 / 主题切换保留但按 34px 图标按钮排版并登记偏离）、Hero 副文案与社证行（接站点副标题 / 公开统计，无数据时用 i18n 默认文案）、控制台预览 URL、"对比官方订阅"区标题与表格、页脚 服务条款 / 隐私政策 链接（来自 `login_agreement_documents`）；验证差异像素 < 0.5%（排除动态文案区域）、`HomeView*.spec` 通过
- [x] 5.2 首页暗色与 390：对照 `reference/dark/01_首页.png` 与 `08_移动端` 首页画板（顶栏 + 汉堡、Hero 单列、卡片纵向、CTA 全宽）；验证两张截图 + 偏离登记
- [x] 5.3 `components/home/*`（ConsolePreview / HomeCodeTabs / 定价表 / 对比表 / CTA / 页脚）抽成组件并写结构测试；验证 spec 通过
- [x] 5.4 `/model-plaza`（`ModelPlazaView.vue` + `components/modelPlaza/*`）：公开页壳 + 30/800 标题 + 分组玻璃卡表格（42 表头、20px 底板、tabular 价格、`.tag` 倍率）、嵌入模式无壳；验证亮 / 暗 / 390 截图 + spec
- [x] 5.5 `/key-usage`（`KeyUsageView.vue`）：公开页壳、40px 密钥字段 + 42px 主按钮、结果统计卡 + 表格；验证截图 + spec
- [x] 5.6 `/legal/:documentId`：公开页壳 + 玻璃卡 Markdown（`announcement-markdown` 令牌化）；验证截图
- [x] 5.7 `/:pathMatch(.*)*`（`NotFoundView.vue`）：64/800 主色渐变 404 + 返回按钮；验证截图
- [x] 5.8 提交 `feat(glass): public pages (tasks 5.1–5.7)`

## 6. 认证、回调、安装向导（2 个代理：A 登录/注册/密码/验证；B 回调/向导）

- [x] 6.1 `/login` 像素 diff（`reference/light/02_登录.png`）：品牌面板 `48px 56px`、46/800 标题、特性行、底部徽章；卡片 440 / 36 / r20 / `.glass-ring`、字段 40、主按钮 42、OAuth 40 排布（首个全宽其余两列）、页脚条款；验证差异 < 0.5%、暗色 + 390 截图、`views/auth/__tests__` 全绿
- [x] 6.2 `/register`：同卡片配方，邀请码 / 优惠码内联校验态、条款勾选；验证截图 + spec
- [x] 6.3 `/forgot-password`、`/reset-password`：同卡片配方，成功态 `.notice-success`；验证截图
- [x] 6.4 `/email-verify`、`/auth/dingtalk/email-completion`：48px 6 位码字段（mono 22 tracking .4em）、重发倒计时 ghost 按钮；验证截图
- [x] 6.5 新建 `components/auth/CallbackStatusCard.vue` 并用于 6 个回调视图：44px tone 圆、20/800、13.5 muted、42 按钮、`.code-block`；验证每个回调 成功 / 失败 / 加载 三态截图 + spec
- [x] 6.6 `/setup`（`SetupWizardView.vue`）：640 卡、4 步步进器、`SettingRow`、34 连接测试 + 内联徽章、42 完成；验证截图（mock 返回 `needs_setup:true` 分支）
- [x] 6.7 `components/auth/{EmailOAuthButtons,LinuxDoOAuthSection,DingTalkOAuthSection,OidcOAuthSection,WechatOAuthSection,LoginAgreementPrompt,TotpLoginModal}.vue`：40px secondary + 18px 品牌标、`UiModal` 44px 码字段；验证 `components/auth/__tests__` 全绿
- [x] 6.8 提交 `feat(glass): auth and public status pages (tasks 6.1–6.7)`

## 7. 管理员仪表盘 · 原型 03（1 个代理）

- [x] 7.1 补齐 `admin.dashboard.*` 全部新文案键（zh + en）：heroGreeting* / heroTitle* / heroSummary / heroIssues / viewOpsMonitor / handleAbnormalAccounts / serviceStatus / liveRpm / liveTpm / todayNew / activeRatio / abnormalCount / statusError / cacheHitRate / requestTrend / trendSubtitle / costShort / platformHealth / recentEvents / noPlatformHealth* / noEvents*；验证截图无原始键名、`i18n-diff` 为 0
- [x] 7.2 Hero 像素对齐参考图 03：右列三张 `.glass-inset` 不溢出（`grid 1.25fr 1fr`、min-width 0）、点阵纹理 + 双径向光；验证 diff < 1%
- [x] 7.3 请求趋势卡：14 根 CSS 柱（高 150、r 6 6 3 3、末柱实心、其余渐变、10.5 刻度），分段 请求 / Token / 费用 切换数据源；验证截图与参考图一致、`DashboardView.spec` 通过
- [x] 7.4 8 张 `StatCard`（4 列、增量药丸、sparkline 来自趋势端点，无序列则最近 12 点平线）；验证截图
- [x] 7.5 平台健康（`120px 1fr 150px`、22 底板、8px 三段堆叠条）与最近事件（`44px 8px 1fr`）绑定真实字段，无数据用同风格空态；验证截图
- [x] 7.6 保留原有功能（日期范围、用户 / 密钥消费明细弹层、刷新、Top 12 用户趋势、快捷操作）并迁到 `.filter-pill / segmented / UiModal`；390 用 08 画板仪表盘变体；验证亮 / 暗 / 390 截图、`anchor-diff` 无丢失；提交 `feat(glass): admin dashboard (tasks 7.1–7.6)`

## 8. 账号管理 · 原型 04（2 个代理：A 页面与表格；B 弹层）

- [x] 8.1 补齐 `admin.accounts.*` 新文案键（summary.* / bulkEditHeader / importExport / selectedOfTotal / columns.nameId 等）；验证截图无键名泄漏
- [x] 8.2 页头操作折叠：刷新 34 图标、批量编辑、导入 / 导出、`更多` `.dropdown`（自动刷新、容量预测、工具）、右侧 `添加账号` primary；验证截图与参考图 04 头部一致
- [x] 8.3 汇总条 5 个 `.summary-chip` 联动状态筛选；筛选行 260 搜索 + 平台 / 分组 / 类型 pill + 可调度激活 pill + 右侧已选 / 列设置 36×36；删除旧的独立"自动刷新"行与蓝色批量操作条（批量条改为表格内 `.notice-info` 行或页头计数）；验证截图
- [x] 8.4 表格列按 spec 顺序用统一单元格组件重做（名称/ID、平台底板、`.tag` 类型、mono 容量、圆点状态、32×18 开关、今日统计、5h/7d 两条 `.progress-thin`、优先级、最近使用、28px 编辑 + `…`）；保留列设置、sticky、影子行、安全 base_url 链接、用量提示等既有行为；验证 `AccountsView*.spec`（含 bulkEdit / sparkShadow / usageWindowsHint）全绿、1440 截图 diff < 1%
- [x] 8.5 行高 60、页脚"显示 1–8，共 86 条 · 每页 20 条" + 28px 分页；390 卡片模式按 08 密钥卡片配方；验证截图
- [~] 8.6 弹层批次一（`components/account/`）：`CreateAccountModal`、`EditAccountModal`、`BulkEditAccountModal`、`AccountTestModal`、`QuotaLimitCard`、`QuotaNotifyToggle`、`ModelWhitelistSelector`、`AccountGroupsCell` → `UiModal` 720/960 + `SettingRow` / 双列表单 + 36 字段 + `ToggleSwitch` + `UiSelect`；验证各 spec 全绿、打开态截图
- [~] 8.7 弹层批次二：其余 `components/account/*`（OAuth 流程、导入 / 导出、容量预测、Kiro / Antigravity / Grok 特有面板、代理选择、模型映射、限额编辑等）；验证 `vitest run src/components/account` 全绿、`lint:ui` 0 命中
- [x] 8.8 提交 `feat(glass): accounts page and dialogs (tasks 8.1–8.7)`

## 9. API 密钥 · 原型 05（1 个代理）

- [x] 9.1 顶部 `1.2fr 1.2fr 1fr`：两张 `EndpointCard`（api_base_url + custom_endpoints，> 2 个时自适应网格）+ `MiniStatCard`；验证截图与参考图 05 一致
- [x] 9.2 页头：用量查询 / 导入到 CC Switch（`hide_ccs_import_button`）/ 创建密钥（`data-tour="keys-create-btn"`）；筛选行 260 搜索 + 分组 pill + 状态 `SegmentedControl` + 排序文案；列设置按钮移到筛选行右端；验证截图
- [x] 9.3 表格列按 spec：名称/ID、`.code` 密钥 + 眼睛 / 复制 26px、分组 `.tag-accent`、并发 mono、用量今日 / 累计、速率限制、过期时间三色、最近使用、状态、`使用` 文字按钮 + `…` 菜单（编辑 / 启停 / 删除 / 重置配额 / 重置速率 / 查看用量）；行 58；验证 `KeysView.spec` 全绿、`anchor-diff` 无丢失、1440 截图 diff < 1%
- [x] 9.4 弹层：创建 / 编辑密钥、使用密钥（`UseKeyModal`）、删除确认、CC Switch 导入 → `UiModal` + `SettingRow`；验证 `UseKeyModal.spec` 全绿、打开态截图
- [x] 9.5 390 卡片模式按 08 画板（3 迷你统计 → 44 搜索 + 筛选 → `.chip-filter` → 密钥卡 → 52px FAB + `ListFade`）；验证 390 亮 / 暗截图 diff < 1%
- [x] 9.6 提交 `feat(glass): api keys page (tasks 9.1–9.5)`

## 10. 系统设置 · 原型 06（2 个代理：A 通用/条款/功能/安全/用户默认；B 网关/支付/邮件/备份 + 子弹层）

- [x] 10.1 页头 dirty 指示（"● n 项未保存的更改"）+ 重置 + 保存（`form="settings-form"`）；左侧分区导航 34px 项 + 未保存 6px 圆点 + 部署信息卡（版本 / PostgreSQL / Redis / Codex 版本同步）；<768 `.field` select；验证截图与参考图 06 一致、`SettingsView.spec` 通过
- [x] 10.2 通用设置 tab：全部行 `SettingRow`（文本 420 / 数字 120 / 开关 36×20 / Logo 44 预览 + 32 上传 + ghost 移除 / 二维码上传虚线框 / 首页内容文本域）；验证截图 diff < 1%（参考图 502–517 行）
- [x] 10.3 登录条款 tab（文档列表编辑器 `180px 1fr 1fr 32px` 行网格 + 32 添加 + 32 danger 删除）；验证截图
- [x] 10.4 功能开关 tab（每行开关 + 危险提示 12 warning-text）；验证截图
- [x] 10.5 安全与认证 tab（OAuth 提供方列表编辑器、Turnstile / 腾讯验证码、TOTP、Passkey、IP 策略）；验证截图 + 相关 spec
- [x] 10.6 用户默认值 tab（数字字段 120、分组 `UiSelect`、`OpenAIFastPolicyUserSelector` 令牌化）；验证截图
- [x] 10.7 网关服务 tab（自定义端点列表编辑器、超时 / 重试数字字段、模型映射表格）；验证截图
- [x] 10.8 支付设置 tab（渠道卡片 + 品牌按钮保留、汇率 / 限额字段、订阅套餐入口）；验证截图 + `payment` 相关 spec
- [x] 10.9 邮件设置 tab（SMTP 字段、`EmailTemplateEditor.vue` 文本域 + 预览玻璃卡、测试邮件 `UiModal`）；验证截图
- [x] 10.10 数据备份 tab（`BackupView.vue` / dataManagement：S3 配置 `SettingRow`、历史记录 DataTable、恢复确认 `ConfirmDialog`）；验证截图 + spec
- [x] 10.11 `lint:ui` 对 `SettingsView.vue` 与 `components/admin/settings/**` 0 命中、`anchor-diff` 无 v-model 丢失、`vue-tsc` 0 错误；提交 `feat(glass): system settings (tasks 10.1–10.10)`

## 11. 管理员列表、详情与仪表盘（3 批，每批 4 个代理；每个代理 1–2 个路由）

批次 1（用户与资源 / 渠道）
- [x] 11.1 `/admin/users`（`UsersView.vue` + `components/admin/users/*` + 用户弹层）：ListPage 配方，`.summary-chip` 替换三段卡，统一单元格，操作列不重叠，属性配置 / 筛选设置 / 列设置进入 `更多` 与筛选行；验证截图（亮 / 暗 / 390）+ spec + `anchor-diff`
- [x] 11.2 `/admin/groups`（`GroupsView.vue` + `components/admin/groups/*`，含模型路由 / Claude Max 模拟 / 分组容量弹层）；验证同上
- [x] 11.3 `/admin/subscriptions`（`SubscriptionsView.vue` + 分配 / 延期弹层）；验证同上
- [x] 11.4 `/admin/proxies`（`ProxiesView.vue` + 代理弹层 + IP 地理批量工具条）；验证同上
- [x] 11.5 `/admin/channels/pricing`（`ChannelsView.vue` + `components/admin/channels/*`：模型定价表格、倍率 `.tag`、批量编辑弹层）；验证同上
- [x] 11.6 `/admin/channels/monitor`（`ChannelMonitorView.vue`：状态玻璃卡网格 + 圆点徽章 + 8px pulse + 96×28 sparkline + 详情弹层）；验证同上
- [x] 11.7 `/admin/plugins`（`PluginsView.vue`：卡片网格或 ListPage、安装 / 配置弹层）；验证同上
- [x] 11.8 `/admin/announcements`（`AnnouncementsView.vue`：ListPage + Markdown 编辑器 `.field` 文本域 220 + 预览卡 + 受众 / 排期 `SettingRow`）；验证同上；提交 `feat(glass): admin lists batch 1 (tasks 11.1–11.8)`

批次 2（运营）
- [x] 11.9 `/admin/redeem`（在 0.1 基础上完成 ListPage 配方、生成 / 批量更新 / 导出弹层）；验证截图 + `RedeemView*.spec`
- [x] 11.10 `/admin/promo-codes`（`PromoCodesView.vue`）；验证同上
- [x] 11.11 `/admin/affiliates/{invites,rebates,transfers}`（3 视图 + `AdminAffiliateRecordsTable.vue`）；验证同上
- [x] 11.12 `components/admin/payment/*` 与 `components/payment/*` 中管理员侧组件令牌化；验证 spec
- [x] 11.15 `/admin/orders`（有数据态核对：订单号 mono、用户、实付 tabular、支付方式 `.tag`、状态徽章、时间、操作；退款 / 详情弹层）；验证截图
- [x] 11.16 `/admin/orders/invoices`（有数据态核对 + 审批弹层）；验证截图
- [x] 11.17 `/admin/orders/dashboard`（`AdminPaymentDashboardView.vue`：DashboardPage 配方，chart.js 令牌化 via `chartTheme()`）；验证亮 / 暗截图
- [x] 11.18 `/admin/orders/plans`（`AdminPaymentPlansView.vue` + `PlanEditDialog.vue`）；验证截图；提交 `feat(glass): admin lists batch 2 (tasks 11.9–11.18)`

批次 3（工单 / 审计 / 运维）
- [x] 11.19 `/admin/tickets`（`TicketsView.vue` + `components/admin/tickets/*`：ListPage + 分类 / 模板弹层）；验证截图 + spec
- [x] 11.20 `/admin/tickets/:id`（`TicketDetailView.vue`：DetailPage 配方，在 0.3 基础上完成时间线 / 侧卡 / 回复模板 pill / 状态 `UiSelect`）；验证截图 + spec
- [x] 11.21 `/admin/usage`（`UsageView.vue` + `components/admin/usage/*`：ListPage + 日期范围 + 明细弹层 `.code-block`）；验证截图
- [x] 11.22 `/admin/audit-logs`（`AuditLogView.vue`：ListPage + 详情弹层 JSON `.code-block`）；验证截图
- [ ] 11.23 `/admin/ops`（`views/admin/ops/**` 20 个文件：DashboardPage 配方；错误 / 系统日志表 ListPage；6 个图表 `chartTheme()`；4 个弹层 `UiModal`；`OpsErrorDetailsModal`（列表）与 `OpsErrorDetailModal`（详情）是协作对，不合并，仅统一外观）；验证亮 / 暗 / 390 截图 + `ops` spec
- [ ] 11.24 `/admin/risk-control`（`RiskControlView.vue`：规则 `SettingRow` 卡 + 事件 ListPage + 严重度徽章 + `.code-block` 内容）；验证截图
- [ ] 11.25 `/admin/prompt-audit`（`features/prompt-audit/*`：节点池卡片、事件表、详情抽屉）；验证截图 + `prompt-audit` spec；提交 `feat(glass): admin lists batch 3 (tasks 11.19–11.25)`

## 12. 用户侧页面（3 批，每批 4 个代理）

批次 1（仪表盘 / 监控 / 记录）
- [x] 12.1 `/dashboard`（`user/DashboardView.vue` + `components/user/dashboard/*` + `components/charts/*`：DashboardPage 配方；平台拆分改为 `1fr 1fr 1fr` 嵌板而非卡中卡；`TokenUsageTrend` / `ModelDistributionChart` 用 `chartTheme()`；图例 12 muted）；验证亮 / 暗 / 390 截图 + spec
- [x] 12.2 `/monitor`（`ChannelStatusV2View.vue` + `features/channel-monitor-v2/*` + `components/user/monitor/*`；V1 视图同步令牌化或按 feature flag 保留最小改动并登记）；验证截图 + spec
- [ ] 12.3 `/usage`（`UsageView.vue`：ListPage + 日期范围 + 明细弹层）；验证截图 + spec
- [ ] 12.4 `/orders`（`UserOrdersView.vue` + `OrderStatusBadge.vue` + `OrderTable.vue`）；验证截图 + spec
- [ ] 12.5 `/invoices`、`/invoices/:id`（`UserInvoicesView.vue`、`UserInvoiceDetailView.vue`：ListPage + DetailPage）；验证截图；提交 `feat(glass): user pages batch 1 (tasks 12.1–12.5)`

批次 2（工单 / 订阅 / 渠道 / 设置）
- [ ] 12.6 `/tickets`（`TicketsView.vue` + `components/tickets/*` 列表相关 + `components/ticket/*` 合并）；验证截图 + spec
- [ ] 12.7 `/tickets/:id`、`/tickets/new`（`TicketDetailView.vue`、`TicketCreateView.vue`、`TicketConversationPane.vue`、`TicketDetailPane.vue`、`TicketCreateDialog` 等）；验证截图 + spec
- [ ] 12.8 `/subscriptions`（`SubscriptionsView.vue` + `SubscriptionProgressMini`）；验证截图
- [ ] 12.9 `/available-channels`（`AvailableChannelsView.vue` + `components/channels/*`：玻璃卡网格、24 底板、`.chip` 模型、tabular 价格）；验证截图
- [ ] 12.10 `/profile`（`ProfileView.vue` + `components/user/profile/*`：SettingsPage 配方，去掉 Hero 横幅与卡中卡；资料 / 安全（密码、Passkey、2FA `UiModal`）/ 通知（余额提醒）/ 绑定 / 危险区）；验证亮 / 暗 / 390 截图 + `profile` spec；提交 `feat(glass): user pages batch 2 (tasks 12.6–12.10)`

批次 3（操作 / 支付 / 嵌入 / 创作）
- [ ] 12.11 `/redeem`、`/affiliate`（ActionPage 配方；邀请链接 `EndpointCard`、返利统计 `StatCard`、记录表格；修复 `AffiliateView.spec` 移动端断言）；验证截图 + spec
- [ ] 12.12 `/batch-image`（`BatchImageGuideView.vue`）；验证截图
- [ ] 12.13 `/purchase`（`PaymentView.vue` + `components/payment/*`：套餐卡 r16 + `.glass-ring`、金额 40 字段 `$` 前缀、品牌按钮 32、iframe 模式无壳；修复 `PaymentMethodSelector.spec` 两条）；验证截图 + spec
- [ ] 12.14 `/payment/{qrcode,stripe,airwallex,stripe-popup,result}`（440 卡、220 二维码盒、轮询徽章、结果页 `CallbackStatusCard`）；验证截图
- [ ] 12.15 `/custom/:id`（`CustomPageView.vue`：EmbedPage 配方）；验证截图
- [ ] 12.16 `/studio`（`features/creation/**`：三栏保留、会话项 36 侧栏配方、`ComposerBar` `.field`、消息气泡 DetailPage 配方、`TaskCard` 内边距、图片历史网格）；验证截图 + `creation` spec；提交 `feat(glass): user pages batch 3 (tasks 12.11–12.16)`

## 13. 暗色主题全站核对（2 个代理，各半路由）

- [ ] 13.1 生成全部路由暗色 1440 截图，逐张对照亮色：无白底块 / 浅灰边 / 黑字，图表 / tooltip / 下拉 / 弹层 / Toast / 骨架均令牌化；验证 `verification.md` 登记 64 条路由"暗色对等 = 2 分"
- [ ] 13.2 对比度抽检脚本 `scripts/check-contrast.js` 扩展到徽章 5 tone × 2 主题、muted-on-surface、danger-text-on-danger-14%、tooltip；验证全部 ≥ 4.5:1（大字 ≥ 3:1）

## 14. 移动端全站核对（2 个代理，各半路由）

- [ ] 14.1 生成全部路由 390×844 亮 / 暗截图：无横向滚动（CDP 检查 `scrollWidth <= 390`）、单列、卡片模式、FAB、触控 ≥ 44、弹层贴底、筛选抽屉；验证 `verification.md` 登记"移动端 = 2 分"
- [ ] 14.2 平板 900px 抽检 10 条代表路由（侧栏 60 图标轨、2 列统计、表格 sticky 列）；验证截图登记

## 15. 门禁固化与收尾（lead）

- [~] 15.1 `scripts/ui-lint.mjs` + `npm run lint:ui`：遗留类零命中、颜色字面量白名单、views scoped 样式属性扫描；接入 `designTokens.spec.ts`；验证在当前树上退出码 0
- [x] 15.2 `scripts/i18n-diff.mjs`：zh/en 键差集与重复键检查；验证输出 `zh-only: 0, en-only: 0`
- [~] 15.3 `scripts/anchor-diff.mjs <file> --base <rev>`：`data-tour / data-testid / id / aria-label / @handler / v-model / t('key')` 集合比对；验证对 `AccountsView.vue`、`KeysView.vue`、`SettingsView.vue` 相对重构前基线无丢失
- [ ] 15.4 `scripts/ui-shots.sh all` + `scripts/pixel-diff.mjs`：64 条路由 × 4 态截图矩阵入库 `screens/`，6 个原型出稿屏幕 diff 报告 < 0.5%；验证矩阵完整
- [ ] 15.5 `scripts/keyname-leak.mjs`：CDP 抓取每条路由 `innerText` 检查原始 i18n 键名正则；验证 0 命中
- [ ] 15.6 观感评分：`verification.md` 登记 64 条路由 × 10 项分数，全部 ≥ 17，原型出稿 6 屏 = 20；不达标路由回到对应任务重做
- [ ] 15.7 `deviations.md` 完整（每条偏离有路由 / 原型区块 / 理由 / 截图）；`components/ui/README.md`、`components/common/README.md`、`frontend/README` 的设计系统章节更新；验证文档存在
- [ ] 15.8 移除 `vite.config.ts` 中 `TEMP(glass-redesign visual review)` 的 `overlay: false`；`vue-tsc` 0 错误、`vitest run` 0 失败、`npm run build` 成功、`lint:ui` 0；提交 `feat(glass): quality gates and final verification (tasks 15.1–15.8)`；`openspec validate glass-ui-redesign --strict` 通过
