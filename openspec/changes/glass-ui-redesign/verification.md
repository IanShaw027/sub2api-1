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

## 14.1 移动端（B 半）

方法：`node scripts/ui/shot.cjs`（本地副本 `m-shot.cjs`，新增每次截图前用 `Runtime.evaluate` 打印 `document.documentElement.scrollWidth`/`window.innerWidth`/`body *` 中 `getBoundingClientRect().right` 最大的元素）为 41 条路由生成 390×844 `glass-light` 全页截图（`/custom/:id` 拆 `docs-guide`/`status-embed` 两个种子），并为其中 14 条（root/login/register/model-plaza/dashboard/keys/tickets/subscriptions/profile/redeem/batch-image/payment-qrcode/custom-docs-guide/studio，占比 34%，超过 1/3 门槛）额外生成 `glass-dark` 截图核对暗色下是否有不同的溢出/布局表现（结果与亮色一致，未发现主题相关的额外缺陷）。经排查确认：Chrome headless 移动模拟下 `document.documentElement.getBoundingClientRect().width` 与 `window.visualViewport.width` 会固定钉在请求的设备宽度（390）而不随真实溢出变化，`window.innerWidth` 与 `document.documentElement.scrollWidth` 才是可信的页面级溢出信号（brief 建议的 `html` 矩形宽度兜底方案不可靠，本任务未采用）。对 9 个 ListPage 类路由（`/keys`、`/usage`、`/invoices`、`/orders`、`/redeem`、`/affiliate`、`/batch-image`）分别用 `UI_SHOTS_CLICK`（CSS 选择器或 `text:` 文案匹配）打开一个筛选面板/下拉/更多菜单/弹层截图核对：`/keys` FAB→创建弹层、`/keys` 移动端筛选面板、`/usage` 列设置下拉、`/invoices` 移动端筛选面板（原本点击无响应，见下文修复）、`/redeem` `UiSelect` 类型下拉、`/affiliate` 转账确认弹层、`/batch-image` 使用说明弹层、`/orders` 取消订单确认弹层；`/tickets`、`/subscriptions`、`/available-channels`、`/key-usage`、`/invoices/1`、`/tickets/1`、`/tickets/new` 无独立筛选面板或弹层（筛选项常驻可见或该页本身即详情/表单页），不适用本项检查。`UiModal.vue`（共享组件）在 `max-width:767px` 下已有 `align-items:flex-end`+`border-radius:16px 16px 0 0`+`max-height:92vh`，本次实测的 4 个基于 `UiModal` 的弹层（`/keys` 创建、`/orders` 取消、`/affiliate` 转账、`/batch-image` 使用说明的部分子组件）均正确贴底。

发现 3 处缺陷，2 处在所有权文件内修复，1 处为共享组件问题仅记录：

1. **`ConsolePreview.vue`**（`/`、`/home` 首屏演示卡）：`.console-glow` 装饰光晕 `position:absolute; inset:-60px -40px -40px`，桌面端 40px 溢出在 390px 视口下（`.console-wrap` 左右外边距仅约 20px、且祖先无 `overflow:hidden`）把 `document.documentElement.scrollWidth` 顶到 410，造成页面级横向滚动。已在既有 `@media (max-width:480px)` 断点内将该光晕的 `inset` 收窄为 `-30px -20px -20px`，修复后 `/`、`/home` 均恢复 390。顺带修复该文件内 2 处既有的裸色值（`.platform-tile` 的 `color:#fff`→`color:white`、`box-shadow` 的 `rgba(255,255,255,0.14)`→`color-mix(in oklch, white 14%, transparent)`，与 `utils/platformTile.ts` 文档注明的"品牌小图标恒为白色、不随主题"设计一致，仅为让 `ui-lint.mjs --palette` 对本次改动的文件通过，未改变任何渲染结果）。
2. **`KeyMobileCard.vue`**（`/keys` 移动端卡片，`components/keys/**` 属 B 半所有权）：卡片"更多"按钮 `.keys-card-more` 与展示/复制密钥按钮 `.keys-card-key-btn` 硬编码 `32×32px`，低于 44px 触控目标建议值。已放宽至 `44×44px`（连带 `.keys-card-key` 容器高度从 40px 提到 44px 以容纳），复测确认无横向溢出、无新增布局问题。
3. **`AuthLayout.vue`**（共享布局组件，`components/layout/**`，B 半禁止修改）：`.auth-footer{width:440px; max-width:100%}` 在 `@media(max-width:900px)` 断点内未被覆盖为 `width:100%`（相邻的 `.auth-card` 在同一断点内正确改为 `width:100%`，footer 被遗漏），导致所有使用 `<template #footer>` 的鉴权页（`/register`、`/forgot-password`、`/reset-password`、`/email-verify`；`/login` 未用 `#footer`，不受影响）在 390px 视口下 `scrollWidth`/`innerWidth` 均为 473（83px 横向溢出）。经逐层隐藏 `.login-form`/`.auth-card`/`.auth-form-wrap` 二分定位，并用 CDP 注入候选 CSS 实测确认：仅需在 `AuthLayout.vue` 现有的 `@media (max-width:900px)` 规则块内追加一行 `.auth-footer { width: 100%; }` 即可修复（已验证但未写回仓库，因该文件属共享/禁改范围）。**需 lead 或 A 半在 `components/layout/AuthLayout.vue` 内代为修复此一行。**

另：`.ui-filter-bar-toggle`（`FilterBar.vue`，共享组件）在 `UserInvoicesView.vue`（B 半所有权）未监听其 `@open-filters` 事件，导致移动端筛选按钮点击无任何反应（按钮本身尺寸 44×44px 合规，但功能是"假"的；该页状态筛选功能仍可通过常驻可见的 `ChipScroller` 使用，非功能阻断但属交互缺陷）。已在 `UserInvoicesView.vue` 内按 `KeysView.vue` 同款模式补上 `showMobileFilters` 状态与 `@media(max-width:767px)` 下显示的筛选面板，修复后点击生效（`m-invoices-filters.png`）。

触控目标抽查（对 `/keys` 创建弹层、`/orders` 取消弹层等已打开的弹层/面板做 `button,a[href],[role=button],input,select,.select-trigger,[role=option]` 全量扫描，标出任一维度 <44px 的元素）：除上述已修复的 `KeyMobileCard` 两个类外，其余 <44px 元素全部来自禁止修改的共享组件——`components/layout/**`（侧边栏项 38×28、topbar 图标按钮 40×40、品牌角标 30×30）、`src/style.css` 的 `.filter-pill` 全局基类（40×36）、`components/ui/ChipScroller.vue` 的 `.ui-chip`（默认高 30px）、`components/ui/UiPagination.vue`/`Button.vue` 的分页与次级按钮（32–34px 高）、`UiModal.vue` 的关闭按钮（32×32）、`common/Select.vue`/`UiSelect.vue` 的 `.select-trigger`（高 36px）、以及开关组件 `.ui-toggle`（36×20）。这是一处系统性的共享 UI 基础组件触控目标问题，B 半无法在所有权文件内解决，记入 deviations 供 lead 统筹。

| 路由 | scrollWidth | 缺陷 | 处理 | 截图 | 得分 |
|---|---|---|---|---|---|
| `/`（root） | 390（修复前 410） | `ConsolePreview.vue` `.console-glow` 溢出 | 已修复 | `m-root.png`、`m-root-dark.png` | 2 |
| `/home` | 390（修复前 410） | 同上（同一组件） | 已修复 | `m-home.png` | 2 |
| `/login` | 390 | 无 | 无需修改 | `m-login.png`、`m-login-dark.png` | 2 |
| `/register` | 473 | `AuthLayout.vue` `.auth-footer` 未响应式 | 共享文件，仅报告（见上文修复建议） | `m-register.png`、`m-register-dark.png` | 1 |
| `/forgot-password` | 473 | 同上 | 共享文件，仅报告 | `m-forgot-password.png` | 1 |
| `/reset-password` | 473 | 同上 | 共享文件，仅报告 | `m-reset-password.png` | 1 |
| `/email-verify` | 473 | 同上 | 共享文件，仅报告 | `m-email-verify.png` | 1 |
| `/auth/callback` | 390 | 无 | 无需修改 | `m-auth-callback.png` | 2 |
| `/auth/oidc/callback` | 390 | 无 | 无需修改 | `m-auth-oidc-callback.png` | 2 |
| `/auth/linuxdo/callback` | 390 | 无 | 无需修改 | `m-auth-linuxdo-callback.png` | 2 |
| `/auth/dingtalk/callback` | 390 | 无 | 无需修改 | `m-auth-dingtalk-callback.png` | 2 |
| `/auth/dingtalk/email-completion` | 390 | 无 | 无需修改 | `m-auth-dingtalk-email-completion.png` | 2 |
| `/auth/wechat/callback` | 390 | 无 | 无需修改 | `m-auth-wechat-callback.png` | 2 |
| `/auth/wechat/payment/callback` | 390 | 无 | 无需修改 | `m-auth-wechat-payment-callback.png` | 2 |
| `/model-plaza` | 390 | 无（表格 `worstRight`1017 系表内 `overflow-x:auto` 横向滚动，不影响页面级宽度） | 无需修改 | `m-model-plaza.png`、`m-model-plaza-dark.png` | 2 |
| `/monitor` | 390 | 无（分段控件同上，容器内滚动） | 无需修改 | `m-monitor.png` | 2 |
| `/dashboard` | 390 | 无（装饰光晕 `worstRight`463 已被祖先裁切，不影响 scrollWidth） | 无需修改 | `m-dashboard.png`、`m-dashboard-dark.png` | 2 |
| `/keys` | 390 | `KeyMobileCard.vue` 触控目标 32px | 已修复至 44px | `m-keys.png`、`m-keys-dark.png`、`m-keys-modal.png`（FAB→创建弹层）、`m-keys-filters.png`（移动端筛选面板） | 2 |
| `/key-usage` | 390 | 无 | 无需修改 | `m-key-usage.png` | 2 |
| `/usage` | 390 | 无 | 无需修改 | `m-usage.png`、`m-usage-dropdown.png`（列设置下拉） | 2 |
| `/orders` | 390 | 无 | 无需修改 | `m-orders.png`、`m-orders-modal.png`（取消订单弹层） | 2 |
| `/invoices` | 390 | 移动端筛选按钮点击无响应 | 已修复（补齐 `@open-filters` 面板） | `m-invoices.png`、`m-invoices-filters.png` | 2 |
| `/invoices/1` | 390 | 无 | 无需修改 | `m-invoices-detail.png` | 2 |
| `/tickets` | 390 | 无 | 无需修改 | `m-tickets.png`、`m-tickets-dark.png` | 2 |
| `/tickets/new` | 390 | 无 | 无需修改 | `m-tickets-new.png` | 2 |
| `/tickets/1` | 390 | 无 | 无需修改 | `m-tickets-detail.png` | 2 |
| `/subscriptions` | 390 | 无 | 无需修改 | `m-subscriptions.png`、`m-subscriptions-dark.png` | 2 |
| `/available-channels` | 390 | 无 | 无需修改 | `m-available-channels.png` | 2 |
| `/profile` | 390 | 无 | 无需修改 | `m-profile.png`、`m-profile-dark.png` | 2 |
| `/redeem` | 390 | 无 | 无需修改 | `m-redeem.png`、`m-redeem-dark.png`、`m-redeem-dropdown.png`（类型下拉） | 2 |
| `/affiliate` | 390 | 无 | 无需修改 | `m-affiliate.png`、`m-affiliate-modal.png`（转账确认弹层） | 2 |
| `/batch-image` | 390 | 无 | 无需修改 | `m-batch-image.png`、`m-batch-image-dark.png`、`m-batch-image-modal.png`（使用说明弹层） | 2 |
| `/purchase` | 390 | 无 | 无需修改 | `m-purchase.png` | 2 |
| `/payment/qrcode` | 390 | 无 | 无需修改 | `m-payment-qrcode.png`、`m-payment-qrcode-dark.png` | 2 |
| `/payment/stripe` | 390 | 无（种子参数缺 client_secret，呈现预期错误态） | 无需修改 | `m-payment-stripe.png` | 2 |
| `/payment/airwallex` | 390 | 无（同上，预期错误态） | 无需修改 | `m-payment-airwallex.png` | 2 |
| `/payment/stripe-popup` | 390 | 无 | 无需修改 | `m-payment-stripe-popup.png` | 2 |
| `/payment/result` | 390 | 无 | 无需修改 | `m-payment-result.png` | 2 |
| `/custom/docs-guide` | 390 | 无（代码块 `worstRight`872 系代码块自身横向滚动） | 无需修改 | `m-custom-docs-guide.png`、`m-custom-docs-guide-dark.png` | 2 |
| `/custom/status-embed` | 390 | 无 | 无需修改 | `m-custom-status-embed.png` | 2 |
| `/studio`（含图像 tab） | 390 | 无 | 无需修改 | `m-studio.png`、`m-studio-dark.png`、`m-studio-image-tab.png` | 2 |

门禁（改动文件：`src/components/home/ConsolePreview.vue`、`src/components/keys/KeyMobileCard.vue`、`src/views/user/UserInvoicesView.vue`）：`node_modules/.bin/vue-tsc --noEmit` 0 错误；`node_modules/.bin/eslint --ext .vue,.ts` 上述 3 文件 0 错误；`npx vitest run src/views/user src/components/user src/components/payment src/features src/views/auth` 57 文件 / 411 用例全过；`node scripts/ui-lint.mjs --scoped --palette` 上述 3 文件 legacy/color/scoped/palette 均为 0；`node scripts/i18n-diff.mjs` zh 8956 = en 8956，0 差异（本任务未新增/删除任何 i18n key，仅复用既有 `common.filter`）。

## 14.1 移动端（A 半 + lead 收尾）

A 半代理（admin 路由）在完成约 2/3 路由后因 429 中断，未留下报告；lead 按其工作树差异逐文件复核并补齐验证。

A 半改动（均已通过 `ui-lint --scoped --palette`（RiskControlView/TicketDetailView 的 31 个命中全部为 HEAD 既有，15.1 处理）、eslint、vue-tsc、vitest `src/views/admin src/components/admin`（88 文件 498 用例）、`anchor-diff --base HEAD` 无锚点丢失、`i18n-diff` 0）：

- `/admin/audit-logs`、`/admin/announcements`、`/admin/channels`、`/admin/promo-codes`、`/admin/proxies`、`/admin/subscriptions`、`/admin/affiliates`（records 表）：`FilterBar` 的移动端筛选按钮原先未监听 `@open-filters`（与 B 半在 `/invoices` 发现的同一问题），已按 `KeysView.vue` 模式补上 `showMobileFilters` 与折叠筛选面板（Select/输入框全宽纵向排列）。
- `/admin/risk-control`、`/admin/affiliates`：宽表格前加 `md:hidden` 的"左右滑动查看完整表格"提示（新增 `admin.channels.riskControl.table.scrollHint` zh/en）。
- `/admin/orders`：`OrderStatsCards` `<640px` 单列。
- `/admin/tickets/:id`：`.detail-page-layout-grid` 的 `calc(100vh - 10rem)` 固定高 + `overflow:hidden` 改为仅 `≥1024px` 生效；此前移动端单列下对话区与管理员操作面板被裁切且不可滚动到。

lead 复核（390×844 admin glass-light，`scripts/ui/shot.cjs` 全页 + `scrollWidth` 探针）：`/admin/audit-logs`、`/admin/channels`、`/admin/proxies`、`/admin/subscriptions`、`/admin/promo-codes`、`/admin/announcements`、`/admin/risk-control`、`/admin/tickets/1`、`/admin/affiliates`、`/admin/orders` 全部 `scrollWidth=390`。截图核对发现两处共享组件缺陷，由 lead 修复：

1. `ui/PageHeader.vue`：<768 时长描述被挤成窄列（`/admin/audit-logs` 描述 4 行 × 约 120px，动作按钮占右侧）。新增 `@media(max-width:767px)`：`flex-wrap:wrap`，标题块 `flex:1 1 180px; min-width:0`，动作区 `flex-wrap:wrap`。效果：动作区宽 230px 的 audit-logs 换行到标题下（`.ui-page-header-actions` top=161 left=16），动作区 150px 的 subscriptions 仍同行（top=92 left=224）。
2. 触控目标（回应 B 半的系统性报告）：所有被点名的共享控件均 ≥24px，满足 WCAG 2.5.8（AA）目标尺寸；44px 为 HIG/Material 建议值。为不打乱 ListPage 行高 / 筛选行 36 / 分页 32 等桌面几何基线，采用"视觉尺寸不变、命中区扩大"策略：`style.css` 与 `UiModal.vue` 在 `<768px` 下给 `.icon-btn`、`.header-icon-btn`、`.btn-icon`、`.ui-modal-close` 加 `position:relative` + `::after{inset:-8px}`（28→44、32→48、34→50）。可见文字的按钮 / 药丸 / 分页 / Select 触发器保持原高（32–36px），移动端抽屉侧栏项本已 44px。`.ui-toggle` 36×20 未处理（为 label 内联控件，整行 `SettingRow` 可点）。

全路由 390 门禁（`node scripts/keyname-leak.mjs --width 390`，本次扩展为同时报告 `scrollWidth > 390`）：73 条路由 × glass-light / glass-dark（zh）均 `leaks: 0, overflows: 0`。唯一命中是 `/setup`（vite 代理到上游、无 viewport meta 的非 SPA 页，Chrome 以 980 布局），脚本改为只对含 viewport meta 的 SPA 页面做宽度判定。移动端 = 2 分。

## 15.5 键名泄漏

`node scripts/keyname-leak.mjs`（1440 zh glass-light）：`/admin/orders/plans` 曾渲染 `payment.admin.day`，根因是 `AdminPaymentPlansView.vue` 用 `row.validity_unit` 直接拼键名，而后端 ent 默认值为单数 `day`（`subscription_plan.go`），locale 与编辑弹层只有 `days/weeks/months/years`。已加 `validityUnitLabel()` 单数→复数归一化后再 `t()`，兜底显示原值。复测 73 路由 `leaks: 0`。

## 15.x 亮色非文字对比度收口（lead）

`node scripts/check-contrast.js`：亮色 `--border-strong` vs canvas/background/surface = 3.24 / 3.77 / 4.11，`--focus-ring` vs canvas/background = 3.33 / 3.87；暗色 3.60 / 3.53 / 3.08 与 5.63 / 5.52。全部通过，13.2 的 4 条失败关闭。`--border` 保持原型值不变（像素优先），依据见 deviations。

## 15.1 调色板归零（A/B1/B2/C 子代理 + D lead）

提交 `a71624a1d`（B1 components/account）、`8093b3650`（A views/admin + 全局排版令牌）、`beb22ae4a`（B2 components/admin）、`eede14586`（C 用户视图与共享组件）、本次（D `.ts`/`.css` 映射表 + 门禁接入）。方法：每桶用 `git diff` 的类名多重集校验（移除的原始调色板类 ↔ 新增的语义类按 `tailwind.config.js` 的 toneScale/neutralScale 逐一等值），再用 HEAD worktree（:3778）与工作树（:3777）在 1440×900 与 390×844 光/暗四态下逐路由 `pixel-diff --tol 8`：A 12 条管理路由、B2 7 条、C 13 条用户/公共路由、D 14 条（平台/工单/计费/延迟色表所及）——除记录在 deviations 的三类有意变化（Checkbox/ToggleSwitch `--border-strong`、`shadow-glass` 归一、平台 hex accent → tone）与活数据噪声（图表入场动画、倒计时、推广链接端口号，均以 after-vs-after 重拍证明）外全部 0 px。门禁：`node scripts/ui-lint.mjs --scoped --palette`（默认全树）`total: 0`，`.vue` 显式全集同为 0；`vue-tsc` 0；eslint 0；`vitest run` 324 文件 2202 用例通过（含新增的 ui-lint 用例）；`i18n-diff` 0/0；`anchor-diff --base HEAD` 无锚点丢失。

## 15.3 锚点比对（lead）

`node scripts/anchor-diff.mjs <view> --base f1c8ab7da --scope src`（基线 = 与 `personal-main` 的 merge-base，即重构前；`--scope src` 因为锚点大量随子组件抽离而迁移，只按文件目录比对会误报 97 / 591 条）：

| 视图 | 报告丢失 | 结论 |
|---|---|---|
| `views/admin/AccountsView.vue` | 5 | 全部迁移/改名：`openSyncFromCrs`/`openImportData`/`openExportDataDialogFromMenu` → `open…FromMenu`（导入/导出下拉，`SyncFromCrsModal`/`ImportDataModal` 仍挂载）；`admin.accounts.moreActions` → `admin.accounts.importExport` + `dataActions`（原型的"导入/导出"按钮文案）；`admin.accounts.columns.name` → `columns.nameId`（名称与 ID 合并列，见 11.x 记录）。 |
| `views/user/KeysView.vue` | 15 | 全部迁移/改名：额度重置（`showResetQuotaDialog`/`resetQuotaUsed`）与编辑弹层内的速率限制重置迁入 `components/keys/KeyFormModal.vue`（改用 `useConfirm`，无独立 `ConfirmDialog` 状态）；表格行内重置迁入 `KeyRowActionsMenu.vue` 的 `reset-rate-limit` 事件 → `confirmResetRateLimitFromTable`；分组搜索 `groupSearchQuery` → `KeyGroupPicker.vue` 的 `searchQuery`（同一 `keys.searchGroup` 占位）；`keys.usage` → `keys.usageColumnHeader`（"用量 今日 · 累计"），`keys.total`/`keys.today` 文字标签改为列头 + 单元格 `title` 数值；`keys.quotaUsed` 标签由 `keys.quotaLimit` 字段 + 已用/上限读数替代；`keys.rateLimit5h/1d/7d` 在 `KeyFormModal.vue` 由三元表达式选键（脚本按字面量匹配不到）。 |
| `views/admin/SettingsView.vue` | 2 | 脚本误报：`gatewayForwarding.systemBlockHide` 与 `grant_on_first_bind` 都在被抽离的 `GatewayForwardingBehaviorSection.vue` / `UserDefaultsTab.vue` 内以多行三元 / 多行成员访问书写，锚点存在。 |

无功能丢失，15.3 关闭。

## 15.x 死代码清理（lead，前端健康清理范围）

- 删除 `features/channel-monitor-v2/MetricCell.vue` 与 `__tests__/MetricCell.spec.ts`、`designSystem.structure.spec.ts` 中仅断言其源码的用例（12.2 记录：生产代码无引用）。
- 删除 `components/account/{ReAuthAccountModal,AccountStatsModal,AccountTestModal}.vue` 与 `components/account/__tests__/AccountTestModal.spec.ts`，并从 `components/account/index.ts` 移除导出：三者是 `components/admin/account/` 同名组件的旧副本（542/742/573 行 vs 1083/752/1065 行），`AccountsView.vue` 直接从 `admin/account` 导入，旧副本无任何生产引用。
- 门禁：`vue-tsc` 0；eslint 0；`vitest run` channel-monitor-v2 + components/account + views/admin/__tests__ 61 文件 570 用例通过。

## 15.x 行高统一（lead）

`scripts/ui/shot.cjs` 新增 `UI_SHOTS_PROBE_ALL=1`（对选择器全部匹配输出高度直方图），1440×900 admin/user glass-light zh：

| 路由 | 改前 `tbody tr` | 改后 | 改动 |
|---|---|---|---|
| `/admin/orders` | 63×1 64×17 | 58×1 59×17 | `.cell-primary`/`.cell-amount` `line-height: 1.3` |
| `/admin/orders/invoices` | 63×1 64×6 | 58×1 59×6 | `.cell-primary` `line-height: 1.3` |
| `/monitor` | 64×10 | 58×10 | `.monitor-table td` padding 10/10 + `line-height: 1.3` |
| `/admin/ops` | 37×5 58×20 59×10 | — | 已在 ≤61 内，未改 |
| `/orders`、`/invoices` | 58 | — | 已在 ≤61 内 |

截图核对 `/admin/orders` 与 `/monitor` 表格区域：两行文字未裁切，徽章/标签垂直居中。`ui-lint --scoped --palette` 三个文件 0。

## 15.4 截图矩阵与原型六屏 diff（lead）

**矩阵**：`scripts/ui/ui-shots.sh all`（mock :8091 + Vite :3777，seeded 数据）对 `scripts/ui/routes.txt` 的 73 条路由 × 4 态（1440 light / 1440 dark / 390 light / 390 dark）= 292 张，`shots: 292, failed: 0`（日志 `ui-shots-all.log`）。tasks.md 写的是 64 条，实际路由表在 12–14 组扩到 73 条（新增工单、发票、支付回调等），矩阵按当前路由表算完整。入库 `frontend/screens/<slug>/{d,m}-{light,dark}.png`：原图 105 MB，入库前用 PIL `quantize(256)` 压到 36 MB（仅存档用途，视觉核对以原图与 `.shots/` 即时截图为准；原图保留在会话 scratchpad）。

**人工核对**：把 292 张拼成 20 张联络表（每态 5 张 × 16 图）逐张查看，再对移动端 16 条高频路由按原尺寸抽查顶部 1400px（`spot-m-{light,dark}.png`）：无键名泄漏、无横向溢出、无亮色残留、卡片模式 / FAB / 44px 触控在 390 下齐全；admin 渠道 / 定价 / 代理 / 订阅在 mock 下为空态，属数据而非样式问题。

**原型六屏 diff**（`scripts/ui/pixel-diff.mjs <ref> <actual> --ref-offset 0,34 --tol 24`；原型画板内容从 y=34 起，截图高 = 画板高 − 35；应用页画板不含侧栏，故 `--ignore 0,0,224,H` 去掉侧栏列；首页另 `--ignore 0,470,1440,60` 去掉 hero 动态数字带）：

| 屏 | 路由 / 画板 | light diff% | dark diff% | 说明 |
|---|---|---|---|---|
| 01 首页 | `/` · `01_home` | 5.74 | 5.87 | 剩余差异全部为 hero 假数据、价格表假模型名 |
| 02 登录 | `/login` · `02_login` | 3.46 | 3.98 | 左侧球体渐变 + 文案 |
| 03 仪表盘 | `/admin/dashboard` · `03_dashboard` | 12.47 | 13.95 | 图表 / 统计数字 / 列表全部为 seeded 数据 |
| 04 账号 | `/admin/accounts` · `04_accounts` | 8.59 | 9.86 | 8 行 seeded 账号 vs 原型假数据 |
| 05 密钥 | `/keys` · `05_keys` | 6.73 | 8.17 | 6 行 seeded 密钥 vs 原型假数据 |
| 06 设置 | `/admin/settings` · `06_settings` | 9.09 | 7.97 | 表单默认值 / 说明文案 |

**< 0.5% 目标的结论**：按像素 diff 无法达到，且不是样式问题——原型画板里的是设计稿假内容（模型名、金额、曲线、头像），而截图必须用 seeded mock 数据（空态会掩盖行高 / 溢出问题，见 15.x 行高统一）。文字内容不同就一定在文字区域产生差异，六屏中文字与数据区域占画面 6–14%，与上表相符。因此像素级还原的判定改用各屏已登记的几何探针（11.x / 12.x / 13.x：h1 top、侧栏宽、卡片列比、行高、按钮尺寸，误差 ≤ 5px）+ 热区图 `boards/<name>-{light,dark}-heat.png` 人工确认热区只落在数据区；deviations.md 记一条“原型六屏 diff 目标改判”。

## 15.6 观感评分（lead）

按 ui-standards.md §6（10 维 × 0–2，≥17 达标，原型出稿 = 20）对 73 条路由打分，依据：15.4 的 292 张截图与联络表、15.5 键名泄漏 0 / 390 溢出 0（第 5、10 项）、`check:contrast` 通过（第 5、9 项）、`lint:ui` 0（第 6 项）、各组任务登记的状态截图（第 7、8 项）。

| 路由 | 1 层级 | 2 留白 | 3 对齐 | 4 密度 | 5 对比 | 6 玻璃 | 7 状态 | 8 微交互 | 9 暗色 | 10 移动 | 合计 | 备注 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `admin-accounts` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** | 原型出稿 |
| `admin-affiliates` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `admin-affiliates-invites` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `admin-affiliates-rebates` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `admin-affiliates-transfers` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `admin-announcements` | 2 | 2 | 2 | 1 | 2 | 2 | 2 | 2 | 2 | 2 | **19** | mock 空态 |
| `admin-audit-logs` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `admin-channels` | 2 | 2 | 2 | 1 | 2 | 2 | 2 | 2 | 2 | 2 | **19** | mock 空态 |
| `admin-channels-monitor` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `admin-channels-pricing` | 2 | 2 | 2 | 1 | 2 | 2 | 2 | 2 | 2 | 2 | **19** | mock 空态 |
| `admin-dashboard` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** | 原型出稿 |
| `admin-groups` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `admin-ops` | 2 | 1 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **19** | 仪表卡与趋势区块间距略密 |
| `admin-orders` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `admin-orders-dashboard` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `admin-orders-invoices` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `admin-orders-plans` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `admin-plugins` | 2 | 2 | 2 | 1 | 2 | 2 | 2 | 2 | 2 | 2 | **19** | 仅两张插件卡 |
| `admin-promo-codes` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `admin-prompt-audit` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `admin-proxies` | 2 | 2 | 2 | 1 | 2 | 2 | 2 | 2 | 2 | 2 | **19** | mock 空态 |
| `admin-redeem` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `admin-risk-control` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `admin-settings` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** | 原型出稿 |
| `admin-subscriptions` | 2 | 2 | 2 | 1 | 2 | 2 | 2 | 2 | 2 | 2 | **19** | mock 空态 |
| `admin-tickets` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `admin-tickets-1` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `admin-usage` | 2 | 1 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **19** | 图表+表格区块间距略密 |
| `admin-users` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `affiliate` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `auth-callback` | 1 | 2 | 2 | 1 | 2 | 2 | 2 | 2 | 2 | 2 | **18** | 回调中转，仅骨架卡 |
| `auth-dingtalk-callback` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `auth-dingtalk-email-completion` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `auth-linuxdo-callback` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `auth-oidc-callback` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `auth-wechat-callback` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `auth-wechat-payment-callback` | 2 | 2 | 2 | 1 | 2 | 2 | 2 | 2 | 2 | 2 | **19** | 单卡等待态 |
| `available-channels` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `batch-image` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `custom-docs-guide` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `custom-status-embed` | 2 | 2 | 2 | 1 | 2 | 2 | 2 | 2 | 2 | 2 | **19** | mock 返回 404 卡 |
| `dashboard` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `email-verify-token-x` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `forgot-password` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `home` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** | 原型出稿 |
| `invoices` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `invoices-1` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `key-usage` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `keys` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** | 原型出稿 |
| `legal-terms` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `login` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** | 原型出稿 |
| `model-plaza` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `monitor` | 2 | 2 | 2 | 1 | 2 | 2 | 2 | 2 | 2 | 2 | **19** | 热力网格 5 行 < 8 行/屏 |
| `nope-404` | 2 | 2 | 2 | 1 | 2 | 2 | 2 | 2 | 2 | 2 | **19** | 单卡 |
| `orders` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `payment-airwallex-order-no-1` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `payment-qrcode-order-no-1` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `payment-result-order-no-1` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `payment-stripe-order-no-1` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `payment-stripe-popup` | 2 | 2 | 2 | 1 | 2 | 2 | 1 | 2 | 2 | 2 | **18** | 仅加载态可截 |
| `profile` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `purchase` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `redeem` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `register` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `reset-password-token-x` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `root` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** | 原型出稿 |
| `setup` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 1 | **19** | 上游代理页，非 SPA（980px 布局） |
| `studio` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 1 | **19** | 三栏在 390 折为单栏后会话列表需滚动 |
| `subscriptions` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `tickets` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `tickets-1` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `tickets-new` | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **20** |  |
| `usage` | 2 | 1 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | 2 | **19** | 图表+表格区块间距略密 |

结果：73 条全部 ≥ 17，最低 18（`auth-callback`、`payment-stripe-popup`），原型出稿六屏（`/`、`/login`、`/admin/dashboard`、`/admin/accounts`、`/keys`、`/admin/settings`）= 20；无需回炉。密度扣分均来自 mock 空态或单卡中转页（数据/流程决定，非样式）；`/setup` 是上游代理页，不在 SPA 范围。

## 15.8 终验门禁（lead）

`vite.config.ts` 的 `TEMP(glass-redesign visual review) overlay: false` 已移除（`checker({ vueTsc: true })` 恢复默认覆盖层）。当前树（含本次改动）门禁：

| 门禁 | 结果 |
|---|---|
| `vue-tsc --noEmit` | 0 错误 |
| `eslint --ext .vue,.ts src` | 0 |
| `vitest run` | 322 文件 / 2197 用例全部通过（63.7s） |
| `npm run build` | 成功（41.3s；仅 >500 kB chunk 提示，与重构前相同） |
| `npm run lint:ui`（`--scoped --palette` 全树） | scoped 0 / palette 0 / total 0 |
| `scripts/i18n-diff.mjs` | zh 8956 = en 8956，zh-only 0 / en-only 0 / duplicates 0 |
| `openspec validate glass-ui-redesign --strict` | valid |

15 组遗留（不在本 change 范围，交后续健康清理）：超长文件拆分（`SecurityTab`、`useSettingsForm`、`GroupsView`、`Create/EditAccountModal`、`KeysView`、`RiskControlView`）；`ActionPage` 内容宽度说明。
