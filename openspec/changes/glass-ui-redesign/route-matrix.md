# 路由 × 模板 × 文件 × 状态 × 任务

状态基线：2026-09-03，工作树（上一轮 20 个代理被 429 中断后的状态），依据截图与代码核对。
状态含义：`未开始` 旧样式；`部分` 已换部分组件但不达标；`接近` 结构对齐待像素核对；`破损` 无法编译或白屏；`达标` 通过全部门禁。

## LandingLayout

| 路由 | 视图 / 组件 | 原型 | 状态 | 任务 |
|---|---|---|---|---|
| `/home` | `views/HomeView.vue`, `components/home/*` | 01 | 接近（文案 / 对比区标题 / 页脚 / 导航项与原型不符，需像素 diff） | 5.1–5.3 |
| `/model-plaza` | `views/ModelPlazaView.vue`, `components/modelPlaza/*` | 同模板 | 未开始 | 5.4 |
| `/key-usage` | `views/KeyUsageView.vue` | 同模板 | 未开始 | 5.5 |
| `/legal/:documentId` | `views/public/LegalDocumentView.vue` | 同模板 | 未开始 | 5.6 |

## AuthLayout

| 路由 | 视图 / 组件 | 原型 | 状态 | 任务 |
|---|---|---|---|---|
| `/login` | `views/auth/LoginView.vue`, `components/auth/*`, `layout/AuthLayout.vue` | 02 | 接近（需像素 diff、暗色、390） | 6.1 |
| `/register` | `views/auth/RegisterView.vue` | 同模板 | 接近 | 6.2 |
| `/forgot-password` | `views/auth/ForgotPasswordView.vue` | 同模板 | 未开始 | 6.3 |
| `/reset-password` | `views/auth/ResetPasswordView.vue` | 同模板 | 未开始 | 6.3 |
| `/email-verify` | `views/auth/EmailVerifyView.vue` | 同模板 | 未开始 | 6.4 |
| `/auth/dingtalk/email-completion` | `views/auth/DingTalkEmailCompletionView.vue` | 同模板 | 未开始 | 6.4 |

## CallbackStatus

| 路由 | 视图 | 状态 | 任务 |
|---|---|---|---|
| `/auth/callback` | `views/auth/OAuthCallbackView.vue` | 未开始 | 6.5 |
| `/auth/linuxdo/callback` | `views/auth/LinuxDoCallbackView.vue` | 未开始 | 6.5 |
| `/auth/wechat/callback` | `views/auth/WechatCallbackView.vue` | 未开始 | 6.5 |
| `/auth/wechat/payment/callback` | `views/auth/WechatPaymentCallbackView.vue` | 未开始 | 6.5 |
| `/auth/dingtalk/callback` | `views/auth/DingTalkCallbackView.vue` | 未开始 | 6.5 |
| `/auth/oidc/callback` | `views/auth/OidcCallbackView.vue` | 未开始 | 6.5 |
| `/payment/result` | `views/user/PaymentResultView.vue` | 未开始 | 12.14 |

## SetupWizard / NotFound

| 路由 | 视图 | 状态 | 任务 |
|---|---|---|---|
| `/setup` | `views/setup/SetupWizardView.vue` | 未开始 | 6.6 |
| `/:pathMatch(.*)*` | `views/NotFoundView.vue` | 未开始 | 5.7 |

## DashboardPage

| 路由 | 视图 / 组件 | 原型 | 状态 | 任务 |
|---|---|---|---|---|
| `/admin/dashboard` | `views/admin/DashboardView.vue`, `components/admin/dashboard/*` | 03 | 部分（i18n 键泄漏、趋势图错误、Hero 右列溢出） | 7.1–7.6 |
| `/dashboard` | `views/user/DashboardView.vue`, `components/user/dashboard/*`, `components/charts/*` | 同模板 | 部分（统计卡已换；图表默认色；平台拆分卡中卡） | 12.1 |
| `/monitor` | `views/user/ChannelStatus*View.vue`, `components/user/monitor/*`, `features/channel-monitor-v2/*` | 同模板 | 未开始（仅 MetricCell） | 12.2 |
| `/admin/ops` | `views/admin/ops/**` | 同模板 | 未开始 | 11.23 |
| `/admin/orders/dashboard` | `views/admin/orders/AdminPaymentDashboardView.vue` | 同模板 | 未开始 | 11.17 |

## ListPage

| 路由 | 视图 / 组件 | 原型 | 状态 | 任务 |
|---|---|---|---|---|
| `/admin/accounts` | `views/admin/AccountsView.vue`, `components/account/*`, `components/admin/account/*` | 04 | 部分（头部 / 汇总 / 筛选换了但键泄漏；表格列旧；30+ 弹层未动） | 8.1–8.8 |
| `/keys` | `views/user/KeysView.vue`, `components/keys/*` | 05 | 破损（模板引用缺失 helper，白屏） | 9.1–9.6 |
| `/admin/users` | `views/admin/UsersView.vue`, `components/admin/users/*` | 同 04 | 部分（旧三段统计卡、旧单元格、操作列重叠） | 11.1 |
| `/admin/groups` | `views/admin/GroupsView.vue`, `components/admin/groups/*` | 同 04 | 未开始 | 11.2 |
| `/admin/subscriptions` | `views/admin/SubscriptionsView.vue` | 同 04 | 未开始 | 11.3 |
| `/admin/proxies` | `views/admin/ProxiesView.vue` | 同 04 | 未开始 | 11.4 |
| `/admin/channels/pricing` | `views/admin/ChannelsView.vue`, `components/admin/channels/*` | 同 04 | 未开始 | 11.5 |
| `/admin/channels/monitor` | `views/admin/ChannelMonitorView.vue` | 同 04 + 状态卡 | 未开始 | 11.6 |
| `/admin/plugins` | `views/admin/PluginsView.vue` | 同 04 | 未开始 | 11.7 |
| `/admin/announcements` | `views/admin/AnnouncementsView.vue` | 同 04 + 编辑器 | 未开始 | 11.8 |
| `/admin/redeem` | `views/admin/RedeemView.vue` | 同 04 | 破损（新旧模板叠加） | 11.9 |
| `/admin/promo-codes` | `views/admin/PromoCodesView.vue` | 同 04 | 未开始 | 11.10 |
| `/admin/affiliates/invites` | `views/admin/affiliates/AdminAffiliateInvitesView.vue` | 同 04 | 未开始 | 11.11 |
| `/admin/affiliates/rebates` | `…/AdminAffiliateRebatesView.vue`, `AdminAffiliateRecordsTable.vue` | 同 04 | 未开始 | 11.11 |
| `/admin/affiliates/transfers` | `…/AdminAffiliateTransfersView.vue` | 同 04 | 未开始 | 11.11 |
| `/admin/orders` | `views/admin/orders/AdminOrdersView.vue` | 同 04 | 接近（空态达标；有数据态待核） | 11.15 |
| `/admin/orders/invoices` | `…/AdminInvoiceApplicationsView.vue` | 同 04 | 接近 | 11.16 |
| `/admin/orders/plans` | `…/AdminPaymentPlansView.vue`, `PlanEditDialog.vue` | 同 04 | 未开始 | 11.18 |
| `/admin/tickets` | `views/admin/TicketsView.vue` | 同 04 | 部分 | 11.19 |
| `/admin/usage` | `views/admin/UsageView.vue` | 同 04 | 未开始 | 11.21 |
| `/admin/audit-logs` | `views/admin/AuditLogView.vue` | 同 04 | 未开始 | 11.22 |
| `/admin/risk-control` | `views/admin/RiskControlView.vue` | 同 04 + 规则卡 | 未开始 | 11.24 |
| `/admin/prompt-audit` | `features/prompt-audit/*` | 同 04 | 部分（已有 tabs 令牌化） | 11.25 |
| `/usage` | `views/user/UsageView.vue` | 同 04 | 部分 | 12.3 |
| `/orders` | `views/user/UserOrdersView.vue`, `components/payment/OrderStatusBadge.vue` | 同 04 | 部分 | 12.4 |
| `/invoices` | `views/user/UserInvoicesView.vue` | 同 04 | 未开始 | 12.5 |
| `/tickets` | `views/user/TicketsView.vue`, `components/tickets/*` | 同 04 | 未开始 | 12.6 |
| `/subscriptions` | `views/user/SubscriptionsView.vue` | 同 04 | 未开始 | 12.8 |
| `/available-channels` | `views/user/AvailableChannelsView.vue`, `components/channels/*` | 卡片网格 | 未开始 | 12.9 |

## DetailPage

| 路由 | 视图 / 组件 | 状态 | 任务 |
|---|---|---|---|
| `/admin/tickets/:id` | `views/admin/TicketDetailView.vue` | 破损（缺 3 个 handler） | 11.20 |
| `/tickets/:id` | `views/user/TicketDetailView.vue`, `components/tickets/TicketConversationPane.vue`, `TicketDetailPane.vue` | 部分 | 12.7 |
| `/tickets/new` | `views/user/TicketCreateView.vue` | 未开始 | 12.7 |
| `/invoices/:id` | `views/user/UserInvoiceDetailView.vue` | 未开始 | 12.5 |

## SettingsPage

| 路由 | 视图 / 组件 | 原型 | 状态 | 任务 |
|---|---|---|---|---|
| `/admin/settings` | `views/admin/SettingsView.vue`（13k 行）, `components/admin/settings/*`, `views/admin/settings/*` | 06 | 部分（通用 tab 接近；其余 8 个 tab、dirty 指示、子弹层未动） | 10.1–10.11 |
| `/profile` | `views/user/ProfileView.vue`, `components/user/profile/*` | 同 06 | 未开始（仍是 Hero 横幅 + 卡中卡） | 12.10 |

## ActionPage

| 路由 | 视图 | 状态 | 任务 |
|---|---|---|---|
| `/redeem` | `views/user/RedeemView.vue` | 部分 | 12.11 |
| `/affiliate` | `views/user/AffiliateView.vue` | 部分（1 条测试失败） | 12.11 |
| `/batch-image` | `views/user/BatchImageGuideView.vue` | 未开始 | 12.12 |

## PaymentFlow

| 路由 | 视图 / 组件 | 状态 | 任务 |
|---|---|---|---|
| `/purchase` | `views/user/PaymentView.vue`, `components/payment/*` | 部分（AmountInput / MethodSelector 改了，2 条测试失败） | 12.13 |
| `/payment/qrcode` | `views/user/PaymentQRCodeView.vue` | 未开始 | 12.14 |
| `/payment/stripe` | `views/user/StripePaymentView.vue` | 未开始 | 12.14 |
| `/payment/airwallex` | `views/user/AirwallexPaymentView.vue` | 未开始 | 12.14 |
| `/payment/stripe-popup` | `views/user/StripePopupView.vue` | 未开始 | 12.14 |

## EmbedPage

| 路由 | 视图 / 组件 | 状态 | 任务 |
|---|---|---|---|
| `/custom/:id` | `views/user/CustomPageView.vue` | 接近 | 12.15 |
| `/studio/chat`、`/studio/image`、`/studio/video`、`/studio/voice`、`/studio/gallery` | `features/creation/**` | WorkspacePage：统一 AppLayout、PageHeader、侧栏分类和 Glass 控件 | 12.16 / 2026-09-06 用户要求 |
| `/studio?mode=...` | 路由兼容重定向 | 保留 prompt，跳转对应子菜单路由 | 2026-09-06 |

## 共享层（非路由）

| 层 | 文件 | 状态 | 任务 |
|---|---|---|---|
| 令牌 | `styles/tokens.css`, `tailwind.config.js`, `index.html`, 字体 | 接近（需逐值核对；字体外链违规） | 1.x |
| 全局类 | `style.css` | 接近（需逐类核对尺寸） | 2.x |
| ui 组件 | `components/ui/*`（23） | 接近 | 2.x |
| common 组件 | `components/common/*`（41） | 部分（17 个已重写，其余 24 未动） | 3.x |
| 布局壳 | `components/layout/*`（11） | 接近（侧栏底部遮挡导航末项） | 4.x |
| 账号弹层 | `components/account/*`（35） | 部分（仅清理 shadow/bg-white） | 8.x |
| 管理员组件 | `components/admin/*`（62） | 未开始 | 11.x |
| 用户组件 | `components/user/*`（32） | 部分（dashboard 4 个） | 12.x |
| 工单组件 | `components/tickets/*`（12）, `components/ticket/*`（1） | 部分 | 12.6–12.7 |
| 支付组件 | `components/payment/*`（12） | 部分 | 12.13 |
| 图表 | `components/charts/*`（5） | 部分 | 12.1 |
