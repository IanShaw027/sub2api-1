## 0. 基线与度量脚本（不改产品代码）

- [ ] 0.1 新建 `frontend/scripts/health/dead-files.mjs`（入口可达图：`import / import() / require / defineAsyncComponent / components:{}`，`@/` 与相对路径解析，输出 无人引用 / 仅测试引用 / 仅被不可达引用 三类），并验证在基线上输出 25 个文件与 `baseline.md` 一致
- [ ] 0.2 新建 `scripts/health/deps-check.mjs`（或接入 `knip`）：对 `dependencies` 统计 `src/**`、`vite.config.ts`、`index.html`、`postcss/tailwind` 配置、`package.json` scripts 中的引用；验证基线报告 `@lobehub/icons` 0 引用
- [ ] 0.3 新建 `scripts/health/i18n-unused.mjs` 与 `scripts/health/i18n-dynamic-keys.json`（前缀白名单，初始收录 `` t(`…${}`) `` 扫描出的前缀）；`scripts/health/i18n-diff.mjs`（zh/en 差集 + 重复键）；验证基线报告约 1,193 个无引用键、差集 5/5
- [ ] 0.4 新建 `scripts/health/any-count.mjs`、`file-size.mjs`（> 1,500 行告警、> 3,000 行失败，白名单 `health-allowlist.json` 初始收录 21 个超限文件 + 任务号）；验证基线 357 / 21
- [ ] 0.5 新建 `scripts/health/bundle-report.mjs`：读取 `rollup-plugin-visualizer` 的 JSON（`--template raw-data`）输出 chunk 表、首屏合计、预算比对；验证基线数字与 `baseline.md` 一致
- [ ] 0.6 `package.json` 增加 `health` 脚本串联 0.1–0.5 并写 `health-baseline.json`；验证 `npm run health` 输出 7 行指标表且基线状态为 FAIL（记录阈值）
- [ ] 0.7 提交 `chore(health): add code health measurement scripts and baseline (tasks 0.1–0.6)`

## 1. 依赖与死文件清理

- [ ] 1.1 移除 `@lobehub/icons` 依赖并重新安装；验证 `ModelIcon.vue` / `ProviderIcon.vue` 渲染不变（截图）、`node_modules` 减少约 22MB、`deps-check` 0
- [ ] 1.2 删除无人引用的 14 个文件：`OpsRuntimeSettingsCard.vue`、`OpsEmailNotificationCard.vue`、`StripePaymentInline.vue`、`PlatformUsageBreakdown.vue`、`PaymentMethodChart.vue`、`TopUsersLeaderboard.vue`、`components/ticket/TicketCategoryForm.vue`（目录一并删除）、`common/Skeleton.vue`、`common/StatusBadge.vue`、`ProfileAccountBindingsCard.vue`、`tickets/TicketInfoItem.vue`、`components/ui/index.ts`、`components/common/index.ts`、`views/auth/index.ts`；连同只覆盖它们的 spec；验证 `dead-files` 无人引用为 0、`vitest run` 0 失败、`vue-tsc` 0 错误
- [x] 1.3 逐个处置仅测试引用的 6 个文件：`PaymentQRDialog.vue`（若支付二维码页应使用它则接回，否则删）、`AdminOrderTable.vue` / `AdminOrderDetail.vue`（`AdminOrdersView` 已自实现则删）、`EndpointPopover.vue`（`KeysView` 改用 `EndpointCard` 则删）、`useForm.ts`（无用则删）、`OpenAIOAuthCapacityDialog.vue` + `api/admin/oauthCapacity.ts` + 其 spec（已决定删除：c664f063c 已把入口作为 `PlatformCapacityDialog` 的重复项移除）；验证 `dead-files` 仅测试引用为 0
- [ ] 1.4 删除 `ui/UiDrawer / SettingRow / SettingsSection` 对桶文件的依赖（改为直接路径导入，`glass-ui-redesign` 会接入它们）；验证不可达为 0
- [ ] 1.5 处理 `file-saver`（改用原生 `a[download]`）与 `console.log` 1 处；验证 `deps-check` 0、`console.log` 0
- [ ] 1.6 `vitest run` 0 失败、`vue-tsc` 0 错误、`npm run health` 中 dead-files / deps 两项 PASS；提交 `chore(health): remove unreachable files and unused deps (tasks 1.1–1.5)`

## 2. i18n 预编译与无引用键第一轮

- [ ] 2.1 引入 `@intlify/unplugin-vue-i18n`（`include: src/i18n/locales/**`，`strictMessage: false`，`escapeHtml: false`），移除 `@intlify/message-compiler` 直接依赖与 JIT 相关配置，保留 `vue-i18n` runtime 别名；验证 CSP `script-src 'self'` 下 `/keys` 分页文案插值正确、`vendor-i18n` < 40KB
- [ ] 2.2 删除整块无引用命名空间（人工确认后）：`admin.dataManagement.*`（132）、`payment.errors.*`（45）、`tickets.errors.*`（17）、`usage.errors.*`（9）、`home.painPoints.*`（9）、`home.providers.*`（7）、`channelStatus.columns.*`（6）等；验证 `i18n-unused` 下降 ≥ 220、`keyname-leak` 0、截图矩阵抽检 10 条路由无缺失
- [ ] 2.3 删除 `admin.accounts.*`（192）、`admin.settings.*`（104）、`admin.ops.*`（92）、`admin.users.*`（87）中的无引用键，动态前缀（如 `admin.accounts.platforms.*`、`admin.accounts.dayOfWeek.*`）先登记白名单再判定；验证 `i18n-unused` ≤ 300
- [ ] 2.4 修复 zh/en 差集 5/5；验证 `i18n-diff` 0/0；提交 `chore(i18n): precompile messages and drop unreferenced namespaces (tasks 2.1–2.4)`

## 3. 构建并行化与重依赖按需加载

- [ ] 3.1 `package.json`：`build` → `vite build`，`typecheck` 独立；`vite.config.ts` 仅 `mode === 'development'` 注册 `checker`；CI 工作流并行 job（typecheck / lint:check / test:run / build），`typecheck` 必需；验证 `npm run build` ≤ 20s、CI 通过
- [ ] 3.2 `xlsx` 动态导入（使用点 `await import('xlsx')`，按钮 loading 态）+ `manualChunks` 独立 `vendor-xlsx`，从 `vendor-ui` 移出；验证首页网络面板无 xlsx、导出功能正常
- [ ] 3.3 `chart.js` / `vue-chartjs` 图表组件改 `defineAsyncComponent`；`driver.js`、`qrcode`、`marked` + `dompurify`、`@airwallex/components-sdk` 在使用点动态导入并各自分块；`vendor-misc` 只剩 axios / vueuse / tanstack / draggable；验证首屏 JS ≤ 700KB 原始、各功能可用
- [ ] 3.4 `manualChunks` 重写：`vendor-vue`、`vendor-i18n`、`vendor-http`（axios）、`vendor-utils`（vueuse / tanstack / draggable）、`vendor-xlsx`、`vendor-chart`、`vendor-markdown`、`vendor-qrcode`、`vendor-tour`、`vendor-stripe`、`vendor-airwallex`；解决 build 警告中 `stores/app.ts` 等被 `i18n/index.ts` 动态导入又被静态导入的混用（改为静态导入）；验证 chunk ≤ 180、无该类警告
- [ ] 3.5 `bundle-report` 预算表写入 `scripts/health/bundle-budget.json`；验证 `npm run health` 中 bundle 项按预算评估；提交 `perf(build): parallel typecheck and on-demand heavy deps (tasks 3.1–3.5)`

## 4. i18n 按域拆分与第二轮清理

- [ ] 4.1 重组 `i18n/locales/{zh,en}/` 为 `core/ landing/ user/ admin-core/ admin-settings/ admin-ops/ admin-channels/` 七个域目录（文件内容机械搬迁，键路径不变）；`i18n/index.ts` 提供 `loadNamespaces(locale, domains)`，`setLocaleMessage` 改为 `mergeLocaleMessage`；验证 `i18n-diff` 0/0、单测通过
- [ ] 4.2 路由 `meta.i18n` 声明域（管理员路由 `admin-core` + 各自域；用户路由 `user`；公开路由 `landing`），`router.beforeResolve` 等待加载；侧栏 / 顶栏只依赖 `core`；验证 `/login` 首屏语言块 ≤ 120KB、跳转 `/admin/accounts` 无键名闪现（`keyname-leak` 全路由 0）
- [ ] 4.3 `setLocale` 只重载已加载的域；`LocaleSwitcher` 切换期间显示加载态；验证切换语言后当前页文案完整、`document.documentElement.lang` 正确
- [ ] 4.4 保留 `VITE_I18N_SPLIT=0` 回退到整包加载直到组 7；验证两种模式测试均通过
- [ ] 4.5 无引用键第二轮：删除剩余零散键至 0（白名单前缀除外）；验证 `i18n-unused` 0、`keyname-leak` 0、截图矩阵全路由抽检
- [ ] 4.6 提交 `perf(i18n): split locales by navigation domain with route-level loading (tasks 4.1–4.5)`

## 5. 组件收敛（与 glass-ui-redesign 组 2–3 同批）

- [ ] 5.1 `common/BaseDialog.vue` 改为 `ui/UiModal` 薄包装（`size` → 宽度档位映射、`title/footer/close` 插槽转发、`scrollTop` 重置行为保留）；验证 `BaseDialog.spec` 全绿、82 个引用方 spec 全绿、打开态截图与 `UiModal` 一致
- [ ] 5.2 `common/Select.vue`（766 行，唯一实现）本体承载 Glass 样式与 pill 变体；`ui/UiSelect.vue` 改为纯 re-export 并透传 `clearable / creatable / remote / variant / valueKey / labelKey`；验证 `Select*.spec`、`UiSelect.spec` 与 61 个引用方 spec 全绿、两种导入路径渲染一致
- [ ] 5.3 `common/Pagination.vue` 同 5.2 处理（`ui/UiPagination.vue` 变 re-export）；`common/Toggle.vue` 的 15 处引用直接替换为 `ui/ToggleSwitch`（props 超集，无需映射）后删除 `Toggle.vue`；`common/Input.vue` / `TextArea.vue` → `ui/TextInput` 包装；`common/StatCard.vue` 删除（仅桶文件引用）；验证各 spec 全绿
- [ ] 5.4 `components/tickets/` 吸收 `ticket/`（已在 1.2 删除）后统一导出路径；`OpsErrorDetailsModal.vue`（列表）重命名为 `OpsErrorListModal.vue` 以与 `OpsErrorDetailModal.vue`（单条详情）区分，更新 `OpsDashboard.vue` 引用；验证 `ops` spec 全绿
- [ ] 5.5 渠道状态视图：`ChannelStatusView.vue` 壳改为按模式 `defineAsyncComponent` 加载 V1 / V2（避免两版同时进入 `/monitor` 块）；把 `channel_monitor_mode / hide_throughput / show_quota` 补进 `stores/app.ts` 回退默认对象并写明默认值；V1 退役与否按 design Open Question 由用户决定后另开任务；验证 `/monitor` 两种模式截图 + spec
- [ ] 5.6 `dead-files --duplicates` 报告为空；提交 `refactor(ui): converge duplicate components onto components/ui (tasks 5.1–5.5)`

## 6. 超大文件拆分（与 glass-ui-redesign 组 8 / 10 / 11 同批）

- [ ] 6.1 `components/account/CreateAccountModal.vue`（7,464 行）按平台面板拆为 `components/account/platform/{Anthropic,OpenAI,Gemini,Antigravity,Grok,Kiro,Ollama,Generic}Panel.vue`（`defineModel` 直通、`defineAsyncComponent` 加载）；验证 `CreateAccountModal.spec` 全绿、主文件 ≤ 1,500 行、`AccountsView` 块 ≤ 300KB
- [ ] 6.2 `EditAccountModal.vue`（5,932 行）复用 6.1 的平台面板；验证 spec 全绿、≤ 1,500 行
- [x] 6.3 `views/admin/SettingsView.vue`（14,077 行）按 tab 拆为 `components/admin/settings/tabs/{General,Agreement,Features,Security,UserDefaults,Gateway,Payment,Email,Backup}Tab.vue`（单一 `<form id="settings-form">` 与 `v-show` 保留，settings 对象经 `defineModel`）；验证 `SettingsView*.spec` 全绿、主文件 ≤ 1,500 行、块 ≤ 300KB
- [ ] 6.4 `views/admin/GroupsView.vue`（7,015 行）拆出弹层到 `components/admin/group/*`；`AccountsView.vue`、`BulkEditAccountModal.vue`、`KeysView.vue`、`RiskControlView.vue`、`ProxiesView.vue`、`DashboardView.vue`、`UsersView.vue`、`UseKeyModal.vue`、`AccountUsageCell.vue`、`ChannelsView.vue`、`RedeemView.vue`、`OpsDashboardHeader.vue`、`BatchImageGuideView.vue`、`SubscriptionsView.vue` 按弹层 / 面板拆分至 ≤ 1,500 行；验证各 spec 全绿、`anchor-diff` 无丢失
- [ ] 6.5 `types/index.ts`（2,534 行）按域拆为 `types/{account,user,group,payment,settings,ops,...}.ts` 并由 `index.ts` re-export；`api/admin/settings.ts`（1,834 行）按 tab 域拆分；验证 `vue-tsc` 0 错误
- [ ] 6.6 `health-allowlist.json` 清空；提交 `refactor(frontend): split oversized views and modals into lazy sub-components (tasks 6.1–6.5)`

## 7. 遗留分支、类型债与门禁接入

- [ ] 7.1 依据 `audit/flags-legacy-report.md` 处置 `legacy` 标记：`api/admin/settings.ts` 的 WeChat `wechat_connect_mode` 双写兼容（`resolveWeChatConnectModeCapabilities / deriveWeChatConnectStoredMode`，SettingsView 加载与保存均在用）保留并补 `// legacy: 旧后端只读 wechat_connect_mode; remove when 后端 ≥ <版本>` 注释；`PaymentResultView` 的 `trade_status/out_trade_no` 回退、`UsageView` 的 `requestTypeToLegacyStream`、`EditAccountModal` 的 `legacyBaseUrl` 迁移、`api/auth.ts` 的 `wechat_oauth_enabled` 回退、`imageUsage` legacy 尺寸标签 逐条标注保留原因与移除条件；`KeyUsageView.vue` 两处对已全局关闭规则的 `eslint-disable no-explicit-any` 删除；验证所有 `legacy` 出现处都带原因注释
- [ ] 7.2 OAuth 回调去重：`WechatCallbackView / OidcCallbackView / LinuxDoCallbackView / DingTalkCallbackView` 四处复制的 `readLegacyFragmentLogin`（URL hash 携带 token 的旧回调机制）抽成 `composables/useOAuthCallbackLogin.ts`，保留回退行为；验证四个 `*CallbackView.spec` 全绿、行数合计下降 ≥ 150
- [ ] 7.3 `any` 从 365（91 文件；`AccountsView` 21、`ProxiesView` 18、`DataTable` 18、`ReAuthAccountModal` 17、`CreateAccountModal` 17、`BatchImageGuideView` 16 …）降到 ≤ 150，并把 `.eslintrc.cjs` 的 `no-explicit-any` 从 `off` 改为 `warn`；优先 `api/**` 返回类型、`types/index.ts`、事件处理器参数；验证 `any-count` PASS
- [ ] 7.4 `eslint-disable` 5 处逐一加原因或移除；验证 ≤ 5 且带原因
- [ ] 7.5 vitest 提速：删除死测试后合并重复 `setup`、`environmentMatchGlobs` 只对需要 DOM 的 spec 用 jsdom、开启 `pool: threads` + `isolate: false` 评估；最慢用例（`SettingsView.spec` 5.2s、`useRoutePrefetch.spec` 2.7s、`EditAccountModal.spec` 1.8s、`MobileDrawer.spec` 1.7s）拆分或去掉真实计时器；验证 `vitest run` ≤ 45s、0 失败
- [ ] 7.6 功能开关一致性：`AppSidebar` 的 `isFeatureFlagEnabled` 与 router 守卫采用同一"设置未加载时不隐藏"策略；`adminSettings` 的 `ops_monitoring / payment` 缓存标志与 `FeatureFlags` 注册表合并为单一来源；验证 `AppSidebar.sections.spec` 与 router spec 全绿
- [ ] 7.7 CI：`health` 与 `bundle-report` 结果作为检查摘要 / PR 评论输出差值；`health-baseline.json` 更新；验证一次 PR 上可见差值表
- [ ] 7.8 文档：`frontend/README.md` 增加"代码健康门禁"与"i18n 域与动态键白名单"章节；提交 `chore(health): legacy cleanup, type debt and CI gates (tasks 7.1–7.7)`；`openspec validate frontend-health-cleanup --strict` 通过
