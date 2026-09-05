# 验证记录

## 门禁执行顺序（每组提交前）

1. `npm run lint:ui`（15.1）→ 0 命中
2. `node scripts/i18n-diff.mjs`（15.2）→ zh-only 0 / en-only 0 / 重复键 0
3. `node scripts/anchor-diff.mjs <组内文件> --base <组前提交>`（15.3）→ 丢失列表为空
4. `vue-tsc --noEmit` → 0 错误
5. `vitest run` → 0 失败
6. `bash scripts/ui-shots.sh <组内路由>` → 每路由 4 张图（亮 / 暗 × 1440 / 390）+ 弹层打开态；原型出稿屏幕另跑 `pixel-diff`
7. 观感评分登记到下表

## 环境

- mock 后端：`frontend/scripts/mock/server.js`（:8091，`/setup/seed?role=admin|user|guest&theme=&locale=&to=`）
- Vite：`VITE_DEV_PROXY_TARGET=http://127.0.0.1:8091 vite --port 3777`
- 截图：`frontend/scripts/ui-shots.sh`（CDP，`ws` 依赖已在 devDependencies）
- 原型参考图：`reference/{light,dark}/0N_*.png`（`scripts/proto-shots.js`）

## 基线（2026-09-03，重构前工作树）

| 检查 | 结果 |
|---|---|
| `vue-tsc --noEmit` | 304 错误（RedeemView 202、KeysView 93、TicketDetailView 9、zh/misc.ts 8 重复键 → 已修） |
| `vitest run` | 307 文件：12 失败 / 26 用例失败 |
| 白屏路由 | `/keys`、`/admin/redeem` |
| 遗留类 | `rounded-2xl` 73 处 / 30 文件；`rounded-xl` 213 / 77；`shadow-lg` 61 / 27；`shadow-xl` 13 / 11；`shadow-md` 8 / 5；`bg-white` 5 / 4；`text-lg font-semibold` 47 / 24；`dark:` 0；hex 颜色 495 / 40（含合法品牌 SVG） |
| Google Fonts 外链 | `index.html` 1 处（违反 `designTokens.spec`） |

## 观感评分登记（每路由一行；分数 0–2）

| 路由 | 主题/视口 | 层级 | 留白 | 对齐 | 密度 | 对比 | 玻璃 | 状态 | 动效 | 暗色 | 移动 | 总分 | 对照原型差异 | 结论 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `/home` | 基线 | 2 | 2 | 1 | 2 | 2 | 2 | 1 | 2 | ? | ? | — | 导航项 / Hero 副文案 / 社证行 / "对比官方订阅"标题 / 页脚条款链接与原型不符 | 待 5.1 |
| `/login` | 基线 | 2 | 2 | 2 | 2 | 2 | 2 | 1 | 2 | ? | ? | — | 待像素 diff | 待 6.1 |
| `/admin/dashboard` | 基线 | 1 | 1 | 0 | 2 | 0 | 2 | 1 | 1 | ? | ? | — | 原始键名泄漏；趋势图 2 根粗柱；Hero 右列溢出 | 待 7.x |
| `/admin/accounts` | 基线 | 1 | 1 | 1 | 1 | 0 | 1 | 1 | 1 | ? | ? | — | 键名泄漏；旧单元格；旧筛选 select；蓝色批量条 | 待 8.x |
| `/keys` | 基线 | — | — | — | — | — | — | — | — | — | — | 0 | 白屏 | 待 0.2 / 9.x |
| `/admin/settings` | 基线 | 2 | 2 | 2 | 2 | 2 | 2 | 1 | 2 | ? | ? | — | 通用 tab 接近；其余 tab 未核 | 待 10.x |
| `/admin/users` | 基线 | 1 | 1 | 0 | 1 | 2 | 1 | 1 | 1 | ? | ? | — | 三段大数字卡；操作列重叠；旧单元格 | 待 11.1 |
| `/dashboard` | 基线 | 2 | 1 | 1 | 2 | 2 | 1 | 1 | 2 | ? | ? | — | 图表默认色；平台拆分卡中卡 | 待 12.1 |
| `/profile` | 基线 | 1 | 1 | 1 | 1 | 2 | 0 | 1 | 1 | ? | ? | — | Hero 横幅 + 卡中卡；非 SettingsPage 布局 | 待 12.10 |
| `/admin/orders` | 基线 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | ? | ? | — | 空态达标；侧栏底部遮挡"使用记录" | 待 4.1 |
| `/register` | 基线 | 2 | 2 | 2 | 2 | 2 | 2 | 1 | 2 | ? | ? | — | 待像素 diff | 待 6.2 |
| `/admin/redeem` | 基线 | — | — | — | — | — | — | — | — | — | — | 0 | 白屏 | 待 0.1 / 11.9 |

（其余 52 条路由在对应任务完成后登记。）

## 门禁执行记录

| 日期 | 任务组 | lint:ui | i18n-diff | anchor-diff | vue-tsc | vitest | 截图矩阵 | 评分 | 提交 |
|---|---|---|---|---|---|---|---|---|---|
| 2026-09-03 | 1–4（令牌 / ui 基元 / 通用组件 / 应用壳） | 0 | 0（8861/8861） | 无丢失 | 0 | 318 文件 / 2170 通过 | 侧栏 + 壳层亮暗截图 | — | 4159c2eee |
| 2026-09-03 | 5–6（公开页 / 认证与状态页） | 0 | 0（8871/8871） | 无丢失（各代理 grep 对比） | 0 | 327 文件 / 2208 通过；vite build 通过 | `/home` 亮 5.5% / 暗 5.7%（各 120px 带 \|dy\|≤3）；`/login` 3.6%（各带 dy=0，1440×863）；`/register`、`/model-plaza`、`/key-usage`、`/legal/terms`、404、6 个回调、`/setup` 亮暗截图 | — | 见提交 |
| 2026-09-04 | 9（API 密钥页） | 0（eslint） | 0（8926/8926，vite-node 深比对） | 未实现（15.3 待做；按 `data-tour`/spec grep 核对无丢失） | 0 | 通用 + ui + keys 目录 308 通过（含更新后的 `designSystem.structure.spec`） | `/keys` 1440×900：亮 6.21% / 暗 7.64%（offset 34；各 120px 带 \|dy\|≤1）；探针：h1 74..109、端点卡 146..257、筛选 271..307、表卡 321..876、thead 43、行 59、分页栏 824..875，与 `reference/proto-geometry.md` 逐项一致；390 亮暗截图 `keys-mobile-*.png` | — | 见提交 |
| 2026-09-04 | 7（管理员仪表盘） | 0（eslint） | 0（8926/8926） | 未实现（同上） | 0 | `DashboardView.spec` 2/2 | `/admin/dashboard` 1440×1080：亮 13.71% / 暗 15.35%（残差为真实 mock 数据与原型假数据的内容差异）；探针：Hero 74..294、StatCard 网格 310..562、图表行 577..821、底部两栏 838..1063（原型 74..293 / 309..558 / 574..817 / 833..1056，均 ≤5px）；第 5 段用户趋势 + 快捷操作保留并允许滚动（deviations 裁定）；390 亮暗 `dash-mobile-*.png` | — | 见提交 |
| 2026-09-04 | 8（账号管理页 + 弹层拆分阶段 1） | 0（eslint） | 0（8926/8926） | 未实现（15.3 待做；8A 已按 spec/`data-tour` grep 核对） | 0（settings 拆分进行中的错误除外） | views/admin + account + admin/account + common + composables 105 文件 / 796 通过 | `/admin/accounts` 1440×1080：亮 8.57% / 暗 9.81%（offset 34）；探针：统计条 146..199、筛选 213..249、thead 264..306、行 61（306/367/428/489）、页脚 1006..1055；shift-profile 0–360 带 dy≤1，360–840 带 dy 4–8（mock 文案与原型不同导致的局部换行）；8B：CreateAccountModal 7464→6033，platform/*Panel、ModelRestrictionEditor、VertexServiceAccountPanel、bulk/* 拆出 | — | 见提交 |

## 备注（组 5–6 发现，超出任务范围，待后续处理）

- 路由 `/:pathMatch(.*)*` 未设 `meta.requiresAuth: false`，游客访问未知路径会被守卫重定向到 `/login`（首次提交起就存在；router 不在本变更范围）。建议在 frontend-health-cleanup 组 7 顺带修复。
- `vite.config.ts` 开发代理 `'/setup'` 前缀会拦截 SPA 路由 `/setup` 本身；建议收窄为 `/setup/status` 等具体路径（或 `bypass` HTML 请求）。
- mock（`scripts/mock/server.js`）缺 `/v1/usage`（网关 Bearer Key）与微信支付回调 resume-token 端点，`/key-usage` 结果态与支付回调成功态无法截图；LinuxDo / WeChat 品牌色 hex 字面量按 brief 第 2 条例外保留。
- 像素比对方法（2026-09-04 修正）：参考图 PNG 0–31 为标签栏、32/33 为 artboard 边框，内部从 y=34 开始；截图高度 = 参考图高 − 35（登录 860、仪表盘/账号 1080、密钥 900、设置 1180），`pixel-diff.mjs --ref-offset 0,34`，`shift-profile.mjs --ref-offset 0,34 --x0 224`；元素级定位用 `UI_SHOTS_PROBE='sel|sel' node scripts/ui/shot.cjs …` 打印 top/left/h/w，与 headless Chrome 实测的原型几何（proto-probe，见提交内 `reference/proto-geometry.md`）逐项比对。组 5–6 的 `/login` 3.6% 是按旧口径（863 / offset 32）得到的，重新按 860 / offset 34 复核（2026-09-04，1440×860 / offset 34）：3.46%，7 个 120px 带 |dy|≤1，通过。
- 截图脚本主题参数：应用 `localStorage.theme` 只认 `light|dark`，mock `/setup/seed` 现在把 `glass-dark|glass-light` 归一化，`shot.cjs … glass-dark` 才会真正切到暗色（此前暗色截图实际是亮色，导致暗色 diff 虚高 98%）。`pixel-diff.mjs` 参数解析已修，`--ref-offset 0,34` 不再被当成热图路径（之前会在 `frontend/` 生成名为 `0,34` 的文件）。
- 门禁脚本（2026-09-04 起可用）：`npm run lint:ui`（`scripts/ui-lint.mjs --scoped`，遗留类 / 颜色字面量 / views scoped 非布局声明，白名单含 `i18n/locales`）、`npm run i18n:diff`（vite-node 深比对 + 重复键）、`node scripts/anchor-diff.mjs <file> --base 65529a248 --scope <子组件目录>`（锚点集合比对，允许因拆分迁移到 scope 目录）。组 9 之后的当前树基线：legacy 181（views/admin 80、components/admin 25、components/user 24、views/user 20、channel-monitor-v2 14）、color 361（views/admin 77、channel-monitor-v2 70、components/admin 53、components/charts 42、views/user 30）、scoped 242（views/admin 175、views/user 49、KeyUsageView 11、views/public 7）。组 11–14 各代理负责把所属路由的命中清零；15.1 以 0 收口。KeysView anchor-diff：v-model `groupSearchQuery`、handler `changeGroup` 迁入 `KeyGroupPicker`，`keys.rateLimit*`/`common.name`/`common.total` 改为字符串三元/列定义，非丢失。
- 壳层几何修正（组 7–9 复核时发现）：原型 header 为 content-box，`height:60px + padding-top:6px` 实际 66px；紧凑页头 h1 24px 的 normal 行高渲染为 35px、描述 13px 为 19px。已改 `AppHeader .topbar` 66px、`PageHeader`/`.page-title`/`.page-description` 行高、`TablePageLayout` 高度 100vh−98。修正后 `/keys` 页头（h1 74..109、描述 113..132、首块内容 146）与原型逐像素一致。

### 8B 阶段 2（账号弹窗拆分，2026-09-04）
- vue-tsc：account 范围 0 错误（其余错误来自 11B 进行中的 ProxiesView）；eslint 0；vitest account 范围 341/341；ui-lint（BulkEdit/bulk/*/Create/Edit）legacy 0 / color 0 / scoped 0。
- 未完成：Create/Edit 弹窗仍 5963/5372 行，见 deviations。

### 组 11 批次 1（users / groups / subscriptions / proxies，2026-09-04）
- vue-tsc 0（范围内）；eslint 0；vitest views/admin/__tests__ + components/admin/proxies 223/223；ui-lint 4 页 + 新组件 total 0；i18n parity 0/0/0（overview.ts 仅 users 块新增 `selectedOfTotal`）。
- anchor-diff：Users/Groups 各「丢失」7 个 handler（状态 chip、行菜单迁入 ActionsCell / GroupSortModal），Subscriptions 3 个（`closeAssignModal` 逻辑进入 SubscriptionAssignDialog，`handleResetQuota/handleRevoke` 经菜单包装仍在），Proxies 34 个（create/edit 表单合并进 ProxyFormModal、批量操作经 `runMoreMenuAction` 包装）——均为迁移非丢失。
- 探针（1440，light）：h1 74/35，filter-bar 146 h36，thead 211 h42（subscriptions / proxies）；无参考图，按 ListPage 配方核验。
- 行数：ProxiesView 2140→1324，SubscriptionsView 1557→1128，UsersView 1940；GroupsView 6372（Create/Edit 弹层拆分延后，见 deviations，health-cleanup 6.4 标 [~]）。

### 11.7 / 11.8（plugins / announcements，2026-09-04）
- eslint 0；vitest 3/3；ui-lint 4 文件 total 0；i18n parity 8899/8899；anchor-diff 无丢失。
- 探针：h1 74/35；announcements filter-bar 146 h36，thead h42。
- lead 补充：mock 新增 `GET /api/v1/admin/plugins`（2 个种子插件，含 compatibility 版本字段）；此前无路由时回落到分页信封导致插件页空白（11D 发现，非回归）。

### 组 10 系统设置（10A + 10B，2026-09-04）
- vue-tsc 0（settings 范围）；eslint 0；vitest settings/backup/ui 111/111；ui-lint total 0（`repo#123` issue 引用误报已在 ui-lint 排除）；i18n parity 8905/8905。
- pixel-diff 通用 tab：light 9.66% / dark 7.90%（General 卡片保留全部真实字段，高度约为原型 2 倍，为主要差异来源）。
- 探针：内容 146；导航 224 + 934；卡间距 12；SettingRow 67/84；各 tab 卡片 top 146。
- 行数：GatewayTab 42（拆为 gateway/ 15 个分区）；SecurityTab 3012、useSettingsForm.ts 2709 仍超 1500（后续 6.3 remainder）。
- anchor-diff：tabs 0 lost（GatewayTab 加 `--scope gateway`）；BackupView 3 处重命名（两步确认函数、`mediaEnabled` computed）。

### 11.5 / 11.6（channels pricing / monitor，2026-09-04）
- vue-tsc 0；eslint 0；vitest channel/monitor + views/admin 257/257；ui-lint total 0；i18n parity 8905/8905。
- anchor-diff：ChannelsView `handleDelete` 迁入 ActionsCell items 回调（仍被引用），ChannelMonitorView 0 lost。
- 探针：h1 74/35，filter-bar 146 h36，thead h42。行数：ChannelsView 1722→372（ChannelFormDialog 1392 等），ChannelMonitorView 402 + monitor/ 14 文件。
- 待 lead：mock 缺 `channel-monitors` 种子 / `history` 路由 / `channel_monitor_mode` 可切 v1（legacy 页只能空态）；`useChannelMonitorFormat.ts` 硬编码调色板色划归 12.2（monitor v2）一并整改。

### 11.9 / 11.10 / 11.11（redeem / promo-codes / affiliates，2026-09-04）
- vue-tsc 0；eslint 0；vitest views/admin + components/admin 348/348；ui-lint total 0；i18n parity 8905/8905。
- RedeemView 1202→676（RedeemGenerateModal / RedeemBatchUpdateModal / RedeemResultModal 抽出）；PromoCodesView 仅修 1 处字面色；affiliates 4 视图审查后无需改动。
- 探针 promo-codes：h1 74/35，thead h42；行高无法核验（mock 5 个列表端点落入 emptyPage）。
- 代理发现并修复：RedeemBatchUpdateModal 抽取时缺 UiModal/Select import 导致弹层内容泄漏到正文流。
- 待 lead：mock 补 `/admin/redeem-codes`、`/admin/promo-codes`、`/admin/affiliates/{invites,rebates,transfers}` 种子。

### 11.19 – 11.22（tickets / ticket detail / usage / audit-logs，2026-09-04）
- vue-tsc 0；eslint 0；vitest views/admin + usage 246/246；ui-lint legacy 0 / color 0 / scoped 10（Tickets 两页的字号字重，与 AccountsView 121 同属 15.x 全局清零项）；i18n 0/0/0；anchor-diff 4 视图无丢失。
- 探针 tickets：h1 74/35，summary-row 146 h53，filter-row 213 h36，thead 264 h42；audit-logs thead h42。行高无法核验（mock tickets / audit-logs 无种子，见 lead 待办）。
- usage：7 处 shadow-lg/xl → token；`components/charts/*` 调色板色归 12.1（chartTheme）。
- 转交 11H：OpsErrorDetailModal 用 `.code-block`。

### 11.12 / 11.15 – 11.18（orders / invoices / plans / payment dashboard + payment 组件，2026-09-04）
- vue-tsc 0；eslint 0；vitest views/admin + payment 328/328；ui-lint total 0（AdminOrdersView 等三个视图用视图根局部 `--cell-fs-*` 变量满足 scoped 检查——记为 15.x 待统一为全局排版刻度）；i18n 8906/8906 0/0/0；anchor-diff 丢 1（`@click="days=d"` → SegmentedControl v-model）。
- 探针（lead 补 mock 后）：orders / invoices thead 197 h42、行 64；plans 行 64/58；payment dashboard hero h1 95（主仪表盘 130，二级页轻量 hero，接受）。行 64 → ≤61 归入 15.x 行高统一清单。
- 品牌色集中到 `components/payment/paymentBrandColors.ts`；DailyRevenueChart 改 chartTheme()。
- lead 已补 mock：`/admin/payment/dashboard`（符合 DashboardStats 契约）、`/admin/payment/orders`(+`/:id`)、`/admin/payment/plans`、`/admin/payment/invoices`(+`/:id`)。
- 11.9/11.11 复核（lead 补 mock 后）：redeem 行 58、affiliates 三页行 59（原 90，三行叠 → NameIdCell 两行）；日期筛选 TextInput type=date h36。共享待办：抽 `components/ui/DateInput.vue`（tickets / affiliates / audit-logs / usage 共用），归 15.x。

## 11G 复核（commit 4263f23ef）
- /admin/tickets：thead 42 w1160（卡内 1172，无横向溢出）；行 61；筛选行 36（DateRangePicker + ToggleSwitch 替换原生控件）。
- /admin/audit-logs：thead 42；行 60。
- 回复模板：`/admin/tickets/reply-templates` 端点正确，工单 501 页面渲染 3 个模板 chip。
- 门禁：eslint/vue-tsc 通过；ui-lint scoped 无新增；i18n parity 8906/8906；anchor-diff TicketsView 丢失 4 个（原生日期框 aria-label ×2、common.yes/no ×2，为控件替换的预期变化）。

## mock：channel monitors（commit 4e42cb81e）
- v1 `/channel-monitors`、`/admin/channel-monitors(/:id|/:id/history)`、`/channel-monitors/:id/status`；v2 `/(admin/)channel-monitor-v2/{dimensions,snapshot,matrix,models,errors,users}` + `/admin/channel-monitor-v2/config`（确定性生成，health score 0–100）。
- `/monitor` 首屏渲染确认；现状缺陷：StatCard 行被趋势卡遮挡 → 交 12B。

## 12.1 用户仪表盘（12A）
- probe 1440×1080 user：hero top 74 h219；h1 top 130；工具条 101–129；StatCard 网格 top 309（列 284 gap 12）；图表行 2fr:1fr w773/w387。纵向下推源于用户端「按平台拆分」模块（proto 03 无）。
- charts/* 全部 chartTheme()；vitest 16 files / 77 通过；eslint/vue-tsc 0；i18n 8917/8917；anchor-diff 丢失 2（PageHeader→hero，键改名）。
- 复核修复：hero 工具条与小卡重叠（absolute→文档流，min-height 219）。
- 待裁决（15.x）：`layout/DashboardPageLayout.vue` 无消费者，admin/user 仪表盘均为内联 hero；决定：保留内联写法，15 阶段删除该组件及其 spec 或收敛为共享骨架。

## 12.2 渠道监控 /monitor（12B）
- probe 1440×1080 user：h1 top 130 h35；StatCard top 309 h110（与用户仪表盘 117 同量级）；筛选行 60（内容 36 + padding）；thead 42；行 64。
- ChannelStatusV2View 947→339，逻辑抽至 `useChannelMonitorV2.ts`，模板拆 MonitorToolbar/MonitorDataTabs；anchor-diff 59 项全部迁移到子文件（补回 4 个遗漏 aria-label）。
- 门禁：eslint 0；vue-tsc 0（本组文件）；ui-lint scoped 全 0；i18n 0/0/0；vitest 12 files / 73 通过（含 components/user）。
- 亮/暗/390 截图确认；原 StatCard 被趋势卡遮挡缺陷消除。

## 12.3–12.5 用户侧用量 / 订单 / 发票（组 12C）

- 提交：`d8a888f80`（i18n 键随 `0fdb94f70` 带入）。
- 拆分：`UsageView.vue` 945 → 370，逻辑抽到 `views/user/usage/useUserUsage.ts`（753）。`UserInvoiceDetailView.vue` 重写为 DetailPage（233）。
- 几何：h1 top 74 / h 35；筛选行 h36（y=146 / 270）；thead 42；行 58–61；`/invoices/:id` 主栏自 y=146 起、侧栏 320；390 单列无横向溢出。
- 截图：`.shots/12c-{usage,orders,invoices,invoice-detail}-{light,dark,mobile}.png`、`12c-invoice-cancel-modal.png`、`12c-orders-cancel-modal.png`（含数据行，非空态）。
- 门禁：vue-tsc 0；eslint 0；ui-lint --scoped legacy/color/scoped 全 0（lead 复核一致）；i18n-diff 0；anchor-diff（UsageView，base 65529a248）无锚点丢失；vitest 183/187，4 条失败位于 `components/user/profile/__tests__/totp-timer-cleanup.spec.ts`，属组 12E 并发进行中目录，与本组无关，待 12E 落地后复核。

## 11.23–11.25 运维监控 / 风险控制 / 提示审计（组 11H，含 lead 复核）

- 提交：`4a24ecf22`（返工主体）、`8c8dff2dd`（风险控制表头 42px）。
- lead 实测（1440×1080，seeded）：`/admin/ops` h1 top 74 / h 36；`/admin/risk-control` h1 74/35、thead 42、行 62；`/admin/prompt-audit` h1 74/35、thead 42、行 62。代理报告的 ops「h1 top=68」为误测，已验收基线 `/admin/accounts` 实测同为 74/35。
- `/admin/ops` 表格枚举：系统日志 thead 42 / 行 58；预警规则 thead 41 / 行 59；「OpenAI Token 请求统计」thead 37 / 行 37（仪表盘卡内紧凑统计表，不在 11.23 要求的列表页表格范围内，登记为偏差）。
- 移动端 390：`/admin/ops` h1 单行（代理修复 scoped `:deep()` 落在同一元素导致规则失效的问题，加 `.ops-header-shell` 祖先层）。暗色 lead 亲自查看 `/admin/ops`，令牌化正确、图表取色随主题。
- 门禁：eslint 0；`RiskControlView.spec` 4/4；ops + risk-control + prompt-audit 共 16 文件 65 用例通过；ui-lint 本次改动文件零新增命中。
- 复核附带发现并单独修复（外壳，非本组所有权）：移动端顶栏公告铃铛缺失，`#topbar-mobile-bell` 传送目标在同组件模板内解析过早导致铃铛无处渲染，改用 `<Teleport defer>` 并对空目标隐藏（提交 `e0cdfdd34`）。

## 12.6–12.7 用户侧工单（组 12D，含 lead 复核）

- 提交：`0fdb94f70`（代理中途 WIP）+ 本次收尾提交；`9f76c4f97`（提交前 `sanitizeTicketPayload` 归一化）、`debb67e7a`（删除 `TicketCreateDialog.vue`）为同组前置修复。
- 结构：建单由弹层改为独立路由 `/tickets/new`；列表页套 TablePageLayout + DataTable + ActionsCell；详情页套 DetailPageLayout（会话主区 + 侧栏信息）。
- lead 实测几何（1440×900，seeded，11 条工单）：h1 top 74 / h 35；首屏内容 y=146；筛选行 h36；thead 42；行 61。分页区随数据量出现。
- 截图：`.shots/12d-{tickets,detail,create}-{light,dark,mobile}.png`、`12d-create-rate.png`、lead 复核 `lead-12d-tickets.png`。
- 门禁（lead 亲自复跑）：vue-tsc 0；eslint（三视图 + `components/tickets`）0；ui-lint --scoped total 0；i18n-diff zh/en 8943 对齐、0 差异；`vitest run src/views/user src/components/tickets` 13 文件 / 68 用例全通过。
- anchor-diff 报 7 处 lost，逐条核实：3 处建单 handler 与 1 处状态 chip handler 属结构性重构的必然结果（已登记偏差）；`closeTicketItem` 与 `tickets.messages.created` 为工具正则局限导致的假阳性（前者经 `ActionsCell` 数组内箭头函数绑定，后者实际在未纳入 scope 的 `TicketCreateView.vue` 中使用）。
- lead 复核附带处理：删除确认无引用的死组件 `components/tickets/TicketInfoItem.vue`；修复共享组件 `components/layout/SettingsPageLayout.vue` 在缺省 `#nav` 时把内容挤进 224px 轨道的缺陷（改为默认单列 + `.has-nav` 双栏，双栏消费者零影响，`vitest run src/components/layout` 8 文件 / 60 用例通过）。

## 12.8–12.10 用户侧订阅 / 可用渠道 / 个人资料（组 12E，含 lead 复核）

- 提交：`112e33fb5`（12.8–12.9 WIP）+ 本次批次 2 收尾提交。
- 12.10 `/profile` 首轮复核被打回（内容列居中、首块 top 123、密码表单标签错位、约 25 处调色板类），返工后 lead 实测：h1 top 74 / h 35 / left 244；`.settings-page-layout-grid` top 146 left 244；`.ui-settings-section` top 146 left 482 w 934——与已验收 `/admin/settings` 基线逐项一致。暗色同几何；390 宽下导航栈在内容上方（section top 368），无横向溢出。
- 12.8 `/subscriptions`：h1 74；三张订阅卡自 146 起（w 383）。12.9 `/available-channels`：筛选行 146 / h36；首卡 lead 将 `space-y-6` 收为 14px 间距后 top 196（原 206）。
- 门禁（lead 亲自复跑）：vue-tsc 0；eslint 0；`vitest run src/views/user src/components/user src/components/payment src/features/creation` 36 文件 / 244 用例全过；ui-lint `--scoped --palette` 个人资料范围 0；i18n-diff zh/en 8954 对齐、0 差异；anchor-diff `ProfileView.vue` 无丢失。
- 截图：`.shots/g12-profile-{light,dark,390}.png`、`g12-subs-light.png`、`g12-channels-light.png`。

## 12.11 兑换 / 邀请返利（组 12F，含 lead 复核）

- 组 12F 代理在 12.12 中途随进程退出，12.11 已完成部分由 lead 单独验收。
- lead 修复：两页的 `btn-primary` 按钮缺 `.btn` 基类（`.btn` 提供 inline-flex/高度，`btn-primary` 仅配色），导致图标脱离文本流（兑换按钮图标漂到左上角、转入余额按钮图标叠在文字上方）。
- 几何：h1 74 / 35；首块 146（redeem 三张 StatCard，affiliate 四张）；最近活动 / 已邀请用户表 thead 42、行 58。内容列居中且宽度各异（redeem 718、affiliate 1080、purchase 896），记入 15.x ActionPage 宽度统一。
- 门禁：vue-tsc 0；eslint 0；vitest（上同）；ui-lint `--scoped --palette`：`RedeemView.vue:409` 1 处字面 `font-size:12.5px`（15.x 排版令牌）；i18n-diff 0；anchor-diff 两视图无丢失。
- 截图：`.shots/g12-redeem-light.png`、`g12-affiliate-light.png`。

## 12.13–12.14 充值 / 支付流程（组 12G，含 lead 复核）

- 组 12G 代理随进程退出，工作已落盘，lead 独立验收。`PaymentView.vue` 拆出 `views/user/payment/usePurchaseFlow.ts` 与 `components/payment/{PurchaseTabSwitcher,RechargePanel,ActiveSubscriptionsList,SubscriptionConfirmCard,RenewalPlanModal,CheckoutHelpCard,PaymentResultStatusCard}.vue`。
- lead 修复：`PurchaseTabSwitcher` 改为 `generic="K extends string"` 以匹配 `'recharge' | 'subscription'` 联合类型（vue-tsc TS2322）；`Stripe/Airwallex/StripePopup/StripePaymentInline/SubscriptionPlanCard` 中 23 处调色板类换成 `danger/success/warning` 语义色阶（Stripe 品牌渐变上的 `text-indigo-200` → `text-white/80`）。
- 几何：`/purchase` h1 74；tab 切换器 146 / h 48；充值账户卡 219，宽 896 居中。`/payment/qrcode`、`/payment/stripe`、`/payment/airwallex` 440 宽卡自 74/106 起；`/payment/result`、`/payment/stripe-popup` 居中 440 卡。
- 门禁：vue-tsc 0；eslint 0；`vitest run src/components/payment + PaymentView/stripeLazyLoading/paymentWechatResume spec` 12 文件 / 100 用例全过；ui-lint 12G 触及文件 palette 0（`PaymentProviderDialog/PaymentStatusPanel/ProviderCard` 未触及文件仍余 28 处，列入 15.x）；`PaymentQRCodeView.vue:122` 二维码 `#FFFFFF` 与 TotpSetupModal 同理保留；anchor-diff `PaymentView/StripePaymentView/PaymentResultView` 无丢失。
- 截图：`.shots/g12-purchase-light.png`、`g12-payment-{result,qrcode,stripe,airwallex,stripe-popup}-light.png`。


## 12.12 / 12.15 / 12.16 批量生图 / 自定义页 / 创作中心（组 12F-resume、12H-resume，含 lead 复核）

| 路由 | 视口 | 探针 | 结果 |
|---|---|---|---|
| `/batch-image`（种子 8 条） | 1440 亮 | h1 74/35/244；`.layout-section-fixed` 146 h36；`thead th` h42；`tbody tr` h61；`table` w1166 ≤ 容器 1172；页脚 h51 | 达标（`.shots/12f-batch-seeded.png`） |
| `/batch-image` | 1440 暗 | 同上几何；徽章 / 成功 / 失败色令牌化 | 达标（`12f-batch-seeded-dark.png`） |
| `/batch-image` | 390 亮 | `html` w390 无横向滚动，表格切卡片模式，筛选单列 | 达标（`12f-batch-seeded-390.png`） |
| `/custom/docs-guide` | 1440 亮 | `.glass-card.embed-card` top74 h772 w1172（EmbedPage 无 PageHeader） | 达标（`12f-custom-docs-guide-light.png`） |
| `/custom/status-embed` | 1440 亮 | `.custom-embed-frame` top166 h770 w1170，无双滚动条 | 达标（`12f-custom-status-embed-light.png`） |
| `/studio` 聊天 | 1440 亮 / 暗 | h1 74/35；三栏 `.studio-layout` 146 h730 w1172；会话 280 / 主栏 544 / 右栏 320，全部视口内滚动 | 达标（`12h-chat.png`、`12h-dark.png`） |
| `/studio` 图像（种子 4 任务） | 1440 亮 / 暗 | 主栏 `TaskGrid` 2 列卡 h332；右栏 `TokenStats` 常驻 `4 个任务` | 达标（`12h-image-seeded.png`、`-dark.png`） |
| `/studio` | 390 亮 | `html` h844 无纵向溢出（三段式面板切换） | 达标（`12h-390.png`） |

门禁：`vue-tsc` 0；eslint 0；vitest `src/views/user src/components/user` 20 文件 109 用例 + `src/features/creation` 7 文件 56 用例通过；ui-lint `--scoped --palette` 所有权文件 0；i18n zh/en 8955 = 8955；anchor-diff 无丢失（两处迁移见 deviations）。

## 13.2 对比度门禁（lead）

`node scripts/check-contrast.js`：glass-light / glass-dark 各 5 tone × 3 底色徽章 + tooltip 全部 PASS（最低为亮色 warning-on-canvas 4.60、danger-on-canvas 4.61）；令牌调整后 `npx vitest run src/components/ui src/__tests__` 30 文件 110 用例通过。亮色残余 4 条非文字失败（border / accent-on-canvas）延至 15.x。

## 13.1 暗色对等（B 半）

方法：`node scripts/ui/shot.cjs 13b-<slug>-{dark,light} <path> 1440 900 <role> glass-{dark,light} zh 1` 生成 41 路由 × 2 主题共 82 张全页截图（`/custom/:id` 拆 `docs-guide`/`status-embed` 两个种子；`/studio` 另用 `studio-image-tab.cjs` 补切到"图像"标签页的暗色截图），逐张核对白底块、浅灰边、黑字、未令牌化图表/徽章/表格/弹层等缺陷；并用一次性 CDP 点击脚本（`click-shot.cjs`，`UI_SHOTS_CLICK` 支持 CSS 选择器与 `text:` 文案匹配）分别打开一个弹层（`/keys` 的 `UseKeyModal`）、一个下拉（`/redeem` 的 `UiSelect` 类型筛选）、一个 Toast（`/affiliate` 复制邀请链接触发的失败提示）截图核对。全部 41 条路由 + 3 个交互态截图均未发现暗色主题缺陷（背景/边框/文字/徽章/图表/弹层均正确读取 `--surface`/`--surface-2`/`--border`/`--foreground`/`--muted` 等令牌，随 `data-theme="glass-dark"` 正确切换），故本任务零代码改动。

几何抽查（`/redeem`，glass-dark，与亮色基线比对确认暗色未引入布局偏移）：`.ui-page-header-title` top=74 left=244 h=35；`.ui-page-header` top=74 h=58；首个内容块（`.glass-card` StatCard）top=146 —— 与 `brief-common.md` 基线（h1 top74/h35/left244，首块 top146）逐项一致。

| 路由 | 缺陷 | 处理 | 截图路径 | 得分 |
|---|---|---|---|---|
| `/`（root） | 无 | 无需修改 | `.shots/13b-root-{dark,light}.png` | 2 |
| `/home` | 无 | 无需修改 | `.shots/13b-home-{dark,light}.png` | 2 |
| `/login` | 无 | 无需修改 | `.shots/13b-login-{dark,light}.png` | 2 |
| `/register` | 无 | 无需修改 | `.shots/13b-register-{dark,light}.png` | 2 |
| `/forgot-password` | 无 | 无需修改 | `.shots/13b-forgot-password-{dark,light}.png` | 2 |
| `/reset-password` | 无 | 无需修改 | `.shots/13b-reset-password-{dark,light}.png` | 2 |
| `/email-verify` | 无 | 无需修改 | `.shots/13b-email-verify-{dark,light}.png` | 2 |
| `/auth/callback` | 无 | 无需修改 | `.shots/13b-auth-callback-{dark,light}.png` | 2 |
| `/auth/oidc/callback` | 无 | 无需修改 | `.shots/13b-auth-oidc-callback-{dark,light}.png` | 2 |
| `/auth/linuxdo/callback` | 无 | 无需修改 | `.shots/13b-auth-linuxdo-callback-{dark,light}.png` | 2 |
| `/auth/dingtalk/callback` | 无 | 无需修改 | `.shots/13b-auth-dingtalk-callback-{dark,light}.png` | 2 |
| `/auth/dingtalk/email-completion` | 无 | 无需修改 | `.shots/13b-auth-dingtalk-email-completion-{dark,light}.png` | 2 |
| `/auth/wechat/callback` | 无 | 无需修改 | `.shots/13b-auth-wechat-callback-{dark,light}.png` | 2 |
| `/auth/wechat/payment/callback` | 无 | 无需修改 | `.shots/13b-auth-wechat-payment-callback-{dark,light}.png` | 2 |
| `/model-plaza` | 无 | 无需修改 | `.shots/13b-model-plaza-{dark,light}.png` | 2 |
| `/monitor` | 无 | 无需修改 | `.shots/13b-monitor-{dark,light}.png` | 2 |
| `/dashboard` | 无 | 无需修改 | `.shots/13b-dashboard-{dark,light}.png` | 2 |
| `/keys` | 无（`UseKeyModal` 弹层实测：代码块固定深色+浅色文字为既有设计，非缺陷） | 无需修改 | `.shots/13b-keys-{dark,light}.png`、`13b-keys-usemodal-dark.png` | 2 |
| `/key-usage` | 无 | 无需修改 | `.shots/13b-key-usage-{dark,light}.png` | 2 |
| `/usage` | 无 | 无需修改 | `.shots/13b-usage-{dark,light}.png` | 2 |
| `/orders` | 无 | 无需修改 | `.shots/13b-orders-{dark,light}.png` | 2 |
| `/invoices` | 无 | 无需修改 | `.shots/13b-invoices-{dark,light}.png` | 2 |
| `/invoices/1` | 无 | 无需修改 | `.shots/13b-invoices-detail-{dark,light}.png` | 2 |
| `/tickets` | 无 | 无需修改 | `.shots/13b-tickets-{dark,light}.png` | 2 |
| `/tickets/new` | 无 | 无需修改 | `.shots/13b-tickets-new-{dark,light}.png` | 2 |
| `/tickets/1` | 无 | 无需修改 | `.shots/13b-tickets-detail-{dark,light}.png` | 2 |
| `/subscriptions` | 无 | 无需修改 | `.shots/13b-subscriptions-{dark,light}.png` | 2 |
| `/available-channels` | 无 | 无需修改 | `.shots/13b-available-channels-{dark,light}.png` | 2 |
| `/profile` | 无 | 无需修改 | `.shots/13b-profile-{dark,light}.png` | 2 |
| `/redeem` | 无（`UiSelect` 下拉实测正常，见几何抽查） | 无需修改 | `.shots/13b-redeem-{dark,light}.png`、`13b-redeem-dropdown-dark.png` | 2 |
| `/affiliate` | 无（Toast "复制失败" 实测：深红底白字，令牌化正确） | 无需修改 | `.shots/13b-affiliate-{dark,light}.png`、`13b-affiliate-toast-dark.png` | 2 |
| `/batch-image` | 无 | 无需修改 | `.shots/13b-batch-image-{dark,light}.png` | 2 |
| `/purchase` | 无 | 无需修改 | `.shots/13b-purchase-{dark,light}.png` | 2 |
| `/payment/qrcode` | 无 | 无需修改 | `.shots/13b-payment-qrcode-{dark,light}.png` | 2 |
| `/payment/stripe` | 无（种子参数不含 client_secret，呈现预期的"缺少订单ID或支付密钥"错误态，卡片本身令牌化正确） | 无需修改 | `.shots/13b-payment-stripe-{dark,light}.png` | 2 |
| `/payment/airwallex` | 无（同上，"缺少 Airwallex 支付参数"预期错误态） | 无需修改 | `.shots/13b-payment-airwallex-{dark,light}.png` | 2 |
| `/payment/stripe-popup` | 无 | 无需修改 | `.shots/13b-payment-stripe-popup-{dark,light}.png` | 2 |
| `/payment/result` | 无 | 无需修改 | `.shots/13b-payment-result-{dark,light}.png` | 2 |
| `/custom/docs-guide` | 无 | 无需修改 | `.shots/13b-custom-docs-guide-{dark,light}.png` | 2 |
| `/custom/status-embed` | 无（内嵌 iframe 展示的是外部登录页原型，非本站主题范围） | 无需修改 | `.shots/13b-custom-status-embed-{dark,light}.png` | 2 |
| `/studio`（聊天 + 图像 tab） | 无 | 无需修改 | `.shots/13b-studio-{dark,light}.png`、`13b-studio-image-dark.png` | 2 |

门禁：`node_modules/.bin/vue-tsc --noEmit` 0 错误；`npx vitest run src/views/user src/components/user src/components/payment src/features` 46 文件 / 306 用例全过；`node scripts/i18n-diff.mjs` zh 8955 = en 8955，0 差异。因本任务零代码改动，未触发 `ui-lint.mjs`/`eslint` 的逐文件门禁（无所有权文件被修改）。

## 13.1 暗色对等（A 半）

方法：`click-shot.cjs`（`scripts/ui/shot.cjs` 的本地副本，新增 `UI_SHOTS_CLICK`——支持 CSS 选择器与 `text:` 文案匹配——及 `UI_SHOTS_PROBE` 几何探针）为 29 条 `/admin/*` 路由 + `/setup`、`/legal/:documentId`、404 共 32 条路由生成暗/亮双主题全页截图（含 `/admin/channels/pricing`、`/admin/channels/monitor`、`/admin/tickets/:id`、`/admin/orders/{dashboard,invoices,plans}`、`/admin/affiliates/{invites,rebates,transfers}` 等子路由），逐张核对白底块、浅灰边、黑字、未令牌化图表/下拉/弹层/Toast/骨架屏等缺陷类别。另对 9 个 ListPage 分别用 `UI_SHOTS_CLICK` 打开一个创建/分配弹层截图（accounts/users/groups/channels/proxies/promo-codes/announcements/subscriptions/redeem 的批量修改弹层），并针对 `/admin/accounts` 额外打开行内更多菜单（`AccountActionMenu`）、"查看统计"弹层（`AccountStatsModal`）与工具栏"容量预测"弹层（`PlatformCapacityDialog`）逐一截图核对。

修复前 `vite.config.ts` 的 `/setup` 代理与 SPA 路由前缀冲突导致 `/setup` 首次截图空白（属共享配置，非本任务所有权文件，未修改，见 deviations）；改用种子参数 `to=%2F__blank` 规避后正常截图。`router.ts` 通配 404 路由缺少 `requiresAuth: false` 导致访客被重定向到 `/login` 而非看到 404 页（同属共享文件，未修改，见 deviations），截图改用已登录 admin 角色验证 404 页暗色渲染。

发现两处真实暗色缺陷，均已在所有权文件内用 `color-mix(in oklch, var(--token) X%, transparent)` 类的语义令牌修复：
1. **`AccountStatsModal.vue`**（"查看统计"弹层）：Row 1 四张统计卡片使用字面 `border-emerald-200/blue-200/amber-200/purple-200 bg-gradient-to-br from-X-50 to-white`——`to-white` 是 Tailwind 未重映射的字面白色（`tailwind.config.js` 只把具名调色板颜色如 `emerald/blue/purple` 重映射到 `success/accent` 语义色阶，`white`/`black` 不受影响），暗色主题下在卡片右侧留下明显白色色块；Row2/Row3 另有 6 处 `bg-cyan-100/bg-indigo-100/bg-teal-100/bg-rose-100/bg-lime-100` 图标底色同属未随主题变化的浅色实底。全部替换为对应语义色 `color-mix` 背景/边框，`text-purple-600`/`text-indigo-600` 等替换为 `text-accent`。图表部分（Chart.js 数据集与坐标轴颜色）原为 `isDarkMode.value ? hexA : hexB` 三元字面色，重构为 `@/utils/chartTheme.ts` 的 `useChartTheme()`/`alpha()`（项目既有的图表令牌化工具，`design.md` 明确要求 "chart.js 图表只对其做令牌化样式约束"），随主题与强调色切换器联动。
2. **`PlatformCapacityDialog.vue`**（工具栏"容量预测"弹层）：KPI 卡片同款 `from-X-50 to-white` 白色块（4 张）、两处 `border-red-200 bg-red-50` 错误提示条为浅红实底、图表沿用与 (1) 相同的字面色三元表达式，均按同一模式修复；图例色块 `bg-blue-500`/`bg-emerald-500`/`bg-red-400/60` 也改为 `bg-accent`/`bg-[var(--success)]`/`color-mix(danger)`，与图表本身的强调色令牌保持一致（该弹层依赖的容量预测接口在 mock 后端未实现，加载态卡在 spinner，KPI/图表区域改用与 (1) 完全同构、已截图验证的修复模式，未能在本环境内对该弹层做逐像素截图复核）。

`AccountActionMenu.vue`（账号行"更多"菜单，`components/admin/account/**`）另发现 8 处裸 Tailwind 调色板类（`text-indigo-500/text-orange-500/text-sky-500/text-purple-600/text-teal-600/text-sky-600/text-green-500`）与 1 处旧版 `shadow-lg ring-1 ring-black/5`，一并替换为 `text-accent/text-warning-text/text-success-text` 与 `border border-line shadow-[var(--shadow-pop)]`（后者是仓库内 15+ 处组件已采用的既有模式）。复核 `tailwind.config.js` 后确认：**全部** Tailwind 具名调色板颜色（含 `purple/indigo/sky/teal/orange/cyan/lime/fuchsia/pink/violet` 等，而不仅是 `red/orange/amber/emerald/green/rose`）都已被重映射到 `accent/success/warning/danger/neutral` 五个语义色阶，故这些类本身在暗色主题下其实已能正确取色；替换为显式令牌类是更符合仓库"禁止裸调色板类"铁律的写法，但并非修复"暗色渲染错误"意义上的缺陷——真正的暗色缺陷信号是與 `white`/`black` 字面值组合的部分（如 `to-white`、`bg-red-50`+`border-red-200` 这类浅底实色）。据此，经 grep 找到的另外 21 个仍含裸调色板类的所有权外/未直接触达文件（`OpsErrorDetailModal.vue`、`UsersView.vue`、`GroupsView.vue`、`OpsSwitchRateTrendChart.vue`、`OpsConcurrencyCard.vue`、`RiskControlView.vue`、`AdminRefundDialog.vue`、`OpsLatencyChart.vue`、`UsageStatsCards.vue`、`OpsErrorLogTable.vue`、`opsFormatters.ts`、`ReAuthAccountModal.vue`、`PaymentMethodChart.vue`、`AccountTestModal.vue`、`GroupRateMultipliersModal.vue`、`UsageTable.vue`、`GroupRPMOverridesModal.vue`、`GroupSortModal.vue`、`GroupReplaceModal.vue`、`SecurityTab.vue`、`UserAllowedGroupsModal.vue`、`UserBalanceHistoryModal.vue`）经抽样确认同样已被 Tailwind 重映射为语义色阶，未发现额外白底块/黑字类暗色缺陷，故未在本任务内改动，留给 `--palette` gate 转正的 group 15 做统一的类名规范化清理。

几何抽查（`/admin/accounts`，glass-dark，与 `brief-common.md` 基线比对确认修复未引入布局偏移）：`.ui-page-header-title` top=74 left=244 h=35 w=723；`.ui-page-header` top=74 left=244 h=58 w=1172；首个 `.glass-card` top=263 h=613 w=1172（该页 header 下方先有筛选栏，故首块 top 大于 B 半基线的 146，属页面自身既有布局，非回归）。

| 路由 | 缺陷 | 处理 | 截图路径 | 得分 |
|---|---|---|---|---|
| `/admin/dashboard` | 无 | 无需修改 | `.shots/13a-dashboard-{dark,light}.png` | 2 |
| `/admin/accounts` | `AccountActionMenu.vue` 裸调色板类+旧版阴影；`AccountStatsModal.vue` 4 张统计卡片 `to-white` 白色块+6 处浅色图标底+图表字面色；`PlatformCapacityDialog.vue` 同款白色块 KPI 卡+浅红错误条+图表字面色 | 三文件均已用语义令牌 `color-mix()`/`useChartTheme()` 修复，见上文与 deviations | `.shots/13a-accounts-{dark,light}.png`、`13a-accounts-modal-dark.png`、`13a-accounts-actionmenu-dark.png`、`13a-accounts-statsmodal-dark-fixed.png`、`13a-platform-capacity-dark-fixed.png` | 1 |
| `/admin/users` | 无 | 无需修改 | `.shots/13a-users-{dark,light}.png`、`13a-users-modal-dark.png` | 2 |
| `/admin/groups` | 无 | 无需修改 | `.shots/13a-groups-{dark,light}.png`、`13a-groups-modal-dark.png` | 2 |
| `/admin/subscriptions` | 无 | 无需修改 | `.shots/13a-subscriptions-{dark,light}.png`、`13a-subscriptions-modal-dark.png` | 2 |
| `/admin/proxies` | 无 | 无需修改 | `.shots/13a-proxies-{dark,light}.png`、`13a-proxies-modal-dark.png` | 2 |
| `/admin/channels` | 无 | 无需修改 | `.shots/13a-channels-{dark,light}.png`、`13a-channels-modal-dark.png` | 2 |
| `/admin/channels/pricing` | 无 | 无需修改 | `.shots/13a-channels-pricing-{dark,light}.png` | 2 |
| `/admin/channels/monitor` | 无 | 无需修改 | `.shots/13a-channels-monitor-{dark,light}.png` | 2 |
| `/admin/plugins` | 无 | 无需修改 | `.shots/13a-plugins-{dark,light}.png` | 2 |
| `/admin/announcements` | 无 | 无需修改 | `.shots/13a-announcements-{dark,light}.png`、`13a-announcements-modal-dark.png` | 2 |
| `/admin/redeem` | 无 | 无需修改 | `.shots/13a-redeem-{dark,light}.png`、`13a-redeem-modal-dark.png` | 2 |
| `/admin/promo-codes` | 无 | 无需修改 | `.shots/13a-promo-codes-{dark,light}.png`、`13a-promo-codes-modal-dark.png` | 2 |
| `/admin/affiliates` | 无 | 无需修改 | `.shots/13a-affiliates-{dark,light}.png` | 2 |
| `/admin/affiliates/invites` | 无 | 无需修改 | `.shots/13a-affiliates-invites-{dark,light}.png` | 2 |
| `/admin/affiliates/rebates` | 无 | 无需修改 | `.shots/13a-affiliates-rebates-{dark,light}.png` | 2 |
| `/admin/affiliates/transfers` | 无 | 无需修改 | `.shots/13a-affiliates-transfers-{dark,light}.png` | 2 |
| `/admin/orders` | 无 | 无需修改 | `.shots/13a-orders-{dark,light}.png` | 2 |
| `/admin/orders/dashboard` | 无 | 无需修改 | `.shots/13a-orders-dashboard-{dark,light}.png` | 2 |
| `/admin/orders/invoices` | 无 | 无需修改 | `.shots/13a-orders-invoices-{dark,light}.png` | 2 |
| `/admin/orders/plans` | 无 | 无需修改 | `.shots/13a-orders-plans-{dark,light}.png` | 2 |
| `/admin/tickets` | 无 | 无需修改 | `.shots/13a-tickets-{dark,light}.png` | 2 |
| `/admin/tickets/:id` | 无 | 无需修改 | `.shots/13a-tickets-detail-{dark,light}.png` | 2 |
| `/admin/usage` | 无 | 无需修改 | `.shots/13a-usage-{dark,light}.png` | 2 |
| `/admin/audit-logs` | 无 | 无需修改 | `.shots/13a-audit-logs-{dark,light}.png` | 2 |
| `/admin/ops` | 无 | 无需修改 | `.shots/13a-ops-{dark,light}.png` | 2 |
| `/admin/risk-control` | 无 | 无需修改 | `.shots/13a-risk-control-{dark,light}.png` | 2 |
| `/admin/prompt-audit` | 无 | 无需修改 | `.shots/13a-prompt-audit-{dark,light}.png` | 2 |
| `/admin/settings` | 无 | 无需修改 | `.shots/13a-settings-{dark,light}.png` | 2 |
| `/setup`（SetupWizardView） | 无（`vite.config.ts` `/setup` 代理冲突为共享配置问题，非页面自身暗色缺陷，见 deviations） | 无需修改 | `.shots/13a-setup-{dark,light}.png` | 2 |
| `/legal/:documentId` | 无 | 无需修改 | `.shots/13a-legal-{dark,light}.png` | 2 |
| 404（NotFoundView） | 无（`router.ts` 通配路由缺 `requiresAuth: false` 导致访客被拦截到 `/login`，为共享路由配置问题，非页面自身暗色缺陷，见 deviations） | 无需修改 | `.shots/13a-notfound-admin-{dark,light}.png` | 2 |

门禁：`node_modules/.bin/vue-tsc --noEmit` 0 错误；`npx vitest run src/views/admin src/components/admin src/features/prompt-audit` 73 文件 / 416 用例，1 个预置失败（`ChannelMonitorView.grok.spec.ts` 断言按钮 class 应含字面量 `zinc`，但被测组件早已改用 `border-[var(--muted)]` 等令牌类，与本任务改动的三个文件无关，attributable 于既有基线，非本次引入）；`node scripts/i18n-diff.mjs` zh 8955 = en 8955，0 差异；逐文件 `ui-lint.mjs --scoped --palette` 与 `eslint --ext .vue,.ts`：`AccountActionMenu.vue`、`AccountStatsModal.vue`、`PlatformCapacityDialog.vue` 均为 0/0/0/0 与 0 error。

## 14.2 平板 900px 抽检（lead）

| 路由 | 探针（900×900 亮） | 结论 |
|---|---|---|
| `/admin/dashboard` `/dashboard` | `aside` 12/60 图标轨；`html` w900；统计卡 2 列 | 达标（`t-admin-dashboard.png`、`t-dashboard.png`） |
| `/admin/accounts` `/admin/users` `/keys` | 内容区 left92 w784；`.table-page-layout` top146；表格横向滚动、首列/操作列 sticky | 修复后达标（`t-admin-users-fix.png`、`-dark.png`、`t-keys-fix.png`）；修复前操作列表头半透明，被盖表头文字透出（`t-admin-accounts.png`） |
| `/admin/usage` `/usage` | 统计卡 2 列、环图 + 表格同卡单列堆叠 | 达标（`t-admin-usage.png`、`t-usage.png`） |
| `/admin/settings` | 设置导航 + 分区两栏保持 | 达标（`t-admin-settings.png`） |
| `/purchase` | 单列 ActionPage 全宽 784 | 达标（`t-purchase.png`） |
| `/studio` | 三栏在 900 折为纵向堆叠（会话列表 → 主栏 → 统计），`html` h1846 | 达标（`t-studio.png`） |

10 条路由 `html` 宽度均为 900，无横向溢出。门禁：`DataTable.vue` eslint 0、vitest `src/components/common` 22 文件 93 用例通过、ui-lint 5 处 `rgba()` 为既有 sticky 阴影渐变（HEAD 相同，15.x 处理）。
