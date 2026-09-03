# 度量基线（2026-09-03，HEAD efc598468 + 工作树）

机器：8 核，Node 20.19，Vite 5，pnpm 安装的 node_modules 307MB。

## 构建与检查

| 项 | 数值 | 说明 |
|---|---|---|
| `vite build`（去掉 checker 插件） | 24.9s | 1,228 模块，225 个 chunk |
| `vue-tsc --noEmit` | 38s | 与构建串行（`build` 脚本 = `vue-tsc -b && vite build`）；`vite-plugin-checker` 在 build 中再跑一次 |
| `vitest run` | 60.3s | 307 文件 / 2,114 用例（环境 160s、collect 79s、transform 24s 累计） |
| JS 产物 | 5.87MB 原始 / 1.60MB gzip | |
| CSS 产物 | 299KB 原始 / 45KB gzip（主 CSS 181KB / 27KB） | 主 CSS 内 235 处 hex 颜色 |

### 最大 chunk（原始 KB）

| chunk | 原始 | gzip | 内容 |
|---|---|---|---|
| `AccountsView-*.js` | 836 | 178 | 账号页 + CreateAccountModal(7.4k 行) + EditAccountModal(5.9k) + BulkEdit(2.4k) 等静态导入 |
| `index-DLoR92UG.js` | 437 | 133 | en 语言包 |
| `vendor-ui-*.js` | 431 | 143 | `xlsx`（7.3MB 依赖）+ `@vueuse/core` |
| `index-RwfSaHd3.js` | 419 | 141 | zh 语言包 |
| `SettingsView-*.js` | 401 | — | 14k 行单文件 |
| `vendor-misc-*.js` | 292 | 98 | axios / marked / dompurify / driver.js / qrcode / file-saver / draggable / tanstack … 首屏引用 |
| `index-bnkZw0HK.js` | 214 | 63 | 入口 |
| `OpsDashboard-*.js` | 204 | — | |
| `GroupsView-*.js` | 199 | — | 7k 行单文件 |
| `vendor-chart-*.js` | 178 | — | chart.js + vue-chartjs |
| `AppLayout-*.js` | 126 | — | |
| `vendor-vue-*.js` | 116 | — | |
| `UsersView-*.js` | 109 | — | |
| `vendor-i18n-*.js` | 63 | — | 含运行时消息编译器（JIT） |

首屏（`index.html` 直接引用）：入口 214 + vendor-vue 116 + vendor-i18n 63 + vendor-misc 292 + CSS 181 + 语言包 ~420（异步但阻塞渲染）≈ 1.29MB 原始。

## 国际化

| 项 | 数值 |
|---|---|
| 语言 | zh、en |
| 键数 | 各 8,847 |
| 行数 / 体积 | 20,462 行；zh 548KB、en 576KB 源码 |
| 最大文件 | `admin/accounts.ts` 1,856 行、`admin/settings.ts` 1,503、`admin/overview.ts` 1,257、`dashboard.ts` 1,023、`misc.ts` 905、`admin/ops.ts` 822、`admin/channels.ts` 776、`admin/resources.ts` 648 |
| 加载方式 | 整包 `import('./locales/zh')`；index.ts spread 合并；运行时 JIT 编译（vite alias 到 runtime 版 + `@intlify/message-compiler`） |
| 无引用键（静态分析，含动态前缀保留） | 1,193（13.5%）：`admin.*` 826（accounts 192、dataManagement 132、settings 104、ops 92、users 87、groups 38、redeem 38、availableChannels 33、proxies 31、dashboard 22）、`payment.*` 110（admin 54、errors 45）、`tickets.*` 49、`usage.*` 24、`home.*` 21、`availableChannels.*` 17、`onboarding.*` 17、`profile.*` 17、`keys.*` 15、`auth.*` 12、`channelStatus.*` 10、`common.*` 10、`dashboard.*` 10 |
| zh/en 差集 | 5 / 5（工作树，重构中新增未同步） |
| 重复键 | zh/misc.ts 8 处（已修） |

清单：`scripts/audit/i18n-unused.txt`（随本变更任务 0.3 迁入 `frontend/scripts/`）。

## 死代码与重复

| 项 | 数值 |
|---|---|
| 源文件 | 931（含 307 spec）；非测试 622 |
| 入口可达 | 597 |
| 不可达 | 25（3,697 行） |

不可达清单：

- 无人引用（14，1,709 行）：`views/admin/ops/components/OpsRuntimeSettingsCard.vue`(536)、`OpsEmailNotificationCard.vue`(441)、`components/payment/StripePaymentInline.vue`(212)、`components/user/PlatformUsageBreakdown.vue`(103)、`components/admin/payment/PaymentMethodChart.vue`(91)、`components/ticket/TicketCategoryForm.vue`(74)、`components/admin/payment/TopUsersLeaderboard.vue`(68)、`components/common/Skeleton.vue`(50)、`components/user/profile/ProfileAccountBindingsCard.vue`(39)、`components/common/StatusBadge.vue`(36)、`components/ui/index.ts`(25)、`components/common/index.ts`(14)、`components/tickets/TicketInfoItem.vue`(13)、`views/auth/index.ts`(7)
- 仅测试引用（6，1,485 行）：`components/admin/account/OpenAIOAuthCapacityDialog.vue`(587)+`api/admin/oauthCapacity.ts`(72)、`components/payment/PaymentQRDialog.vue`(311)、`components/admin/payment/AdminOrderTable.vue`(238)、`AdminOrderDetail.vue`(165)、`components/keys/EndpointPopover.vue`(141)、`composables/useForm.ts`(43)
- 仅被桶文件引用：`ui/UiDrawer.vue`、`ui/SettingRow.vue`、`ui/SettingsSection.vue`（Glass 重构将接入）、`common/StatCard.vue`

重复组件引用计数（非测试）：`BaseDialog` 82 vs `UiModal` 7；`common/Select` 61 vs `UiSelect` 4；`common/Pagination` 31 vs `UiPagination` 6；`ConfirmDialog` 28；`EmptyState` 22；`LoadingSpinner` 18；`common/Toggle` 15 vs `ToggleSwitch` 3；`ui/StatusBadge` 14 vs `common/StatusBadge` 0；`ui/StatCard` 5 vs `common/StatCard` 1；`common/Input` 2 / `TextArea` 2 vs `TextInput` 3；`common/Skeleton` 0；`ticket/TicketCategoryForm` 0 vs `tickets/TicketCategoryForm` 2；`OpsErrorDetailModal` 3 vs `OpsErrorDetailsModal` 1；`ChannelStatusView` / `V1View` / `V2View` 各 1（壳 + 两版）。

## 依赖

| 依赖 | 引用点 | 体积 | 备注 |
|---|---|---|---|
| `@lobehub/icons` | 0 | 22MB | 图标已内联到 `ModelIcon.vue` / `ProviderIcon.vue`，依赖可删 |
| `xlsx` | 1 | 7.3MB | 并入首屏 `vendor-ui` |
| `chart.js` + `vue-chartjs` | 16 + 17 | 6.3MB | 独立块但仪表盘静态导入 |
| `driver.js` | 5 | 204KB | 引导，应按需 |
| `marked` + `dompurify` | 7 + 8 | — | 公告 / 法律文档，应按需 |
| `qrcode` | 4 | — | 支付页，应按需 |
| `@stripe/stripe-js` | 6 | — | 已单独分块 |
| `file-saver` | 2 | — | 可用原生 `a[download]` 替代 |
| `vue-draggable-plus` | 2 | — | |
| `@tanstack/vue-virtual` | 2 | — | DataTable 虚拟滚动 |

## 类型与标记

| 项 | 数值 |
|---|---|
| `: any` / `as any` | 357 |
| `@ts-ignore` | 0 |
| `console.log` | 1 |
| `eslint-disable` | 5 |
| `legacy` 标记 | 132（集中在 `api/admin/settings.ts`） |
| tsconfig | `strict`、`noUnusedLocals`、`noUnusedParameters` 已开 |

## 超大文件（行）

`SettingsView.vue` 14,077 · `CreateAccountModal.vue` 7,464 · `GroupsView.vue` 7,015 · `EditAccountModal.vue` 5,932 · `AccountsView.vue` 3,498 · `BatchImageGuideView.vue` 2,695 · `types/index.ts` 2,534 · `BulkEditAccountModal.vue` 2,443 · `KeysView.vue` 2,443 · `RiskControlView.vue` 2,372 · `ProxiesView.vue` 2,140 · `DashboardView.vue` 2,123 · `UsersView.vue` 1,909 · `UseKeyModal.vue` 1,869 · `api/admin/settings.ts` 1,834 · `AccountUsageCell.vue` 1,759 · `ChannelsView.vue` 1,721 · `RedeemView.vue` 1,684 · `OpsDashboardHeader.vue` 1,628 · `SubscriptionsView.vue` 1,466 · `api/admin/ops.ts` 1,363

## 功能开关与遗留分支（摘自 `audit/flags-legacy-report.md`）

- 功能开关注册表 `utils/featureFlags.ts`：`channelMonitor`（默认开）、`payment`（默认开）、`ticket`（默认开）、`availableChannels / modelPlaza / pluginManagement / riskControl / affiliate / creationCenter`（默认关）；管理端另有 `adminSettings` 的 `ops_monitoring_enabled`（localStorage 缓存，默认开）与 `payment enabled`（默认关）两个不在注册表内的开关；`batchImageAccess` 由用户密钥推导；`isSimpleMode` 为每用户 `run_mode`。
- router 守卫等待 `fetchPublicSettings()` 且只在确认加载后才禁用路由；侧栏 `isFeatureFlagEnabled` 未做同样处理，只靠默认值。
- `channel_monitor_mode / channel_monitor_hide_throughput / channel_monitor_show_quota` 不在 `stores/app.ts` 回退默认中，未设置时分别按 `v1 / false / false` 处理 → **V1 是当前默认**；管理端同时保留 `v2` 与 `legacy` 两个 tab。
- `legacy` 出现于 89 个文件，绝大多数是**在用的兼容代码**：WeChat `wechat_connect_mode` 双写（settings.ts + SettingsView）、四个 OAuth 回调视图各自复制的 `readLegacyFragmentLogin`（约 11 处 × 4）、`UseKeyModal` 的 `codexAuthMode: 'legacy' | 'api-key'`（用户可选，默认 legacy）、`PaymentResultView` 旧支付回跳参数回退、`UsageView` 的 `requestTypeToLegacyStream`、`EditAccountModal` 的 `legacyBaseUrl` 迁移、`api/auth.ts` 的 `wechat_oauth_enabled` 回退。没有发现纯死分支。
- 四个最大文件内无 `v-if="false"`、无 >10 行注释模板、eslint 无未使用变量、无已删除平台的分支。
- 307 个 spec 无断裂导入；11 个无引用组件均无 spec；最慢 spec：`SettingsView.spec` 5.2s、`useRoutePrefetch.spec` 2.7s、`EditAccountModal.spec` 1.8s、`MobileDrawer.spec` 1.7s。
- `any`：365 处 / 91 文件（`views/admin` 96、`ops/components` 44、`components/account` 38、`components/common` 34、`composables` 30）；`.eslintrc.cjs` 全局关闭 `no-explicit-any`，`KeyUsageView.vue` 两处 `eslint-disable` 该规则为无效残留。
