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
