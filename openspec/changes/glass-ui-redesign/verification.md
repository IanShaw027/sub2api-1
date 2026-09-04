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

## 备注（组 5–6 发现，超出任务范围，待后续处理）

- 路由 `/:pathMatch(.*)*` 未设 `meta.requiresAuth: false`，游客访问未知路径会被守卫重定向到 `/login`（首次提交起就存在；router 不在本变更范围）。建议在 frontend-health-cleanup 组 7 顺带修复。
- `vite.config.ts` 开发代理 `'/setup'` 前缀会拦截 SPA 路由 `/setup` 本身；建议收窄为 `/setup/status` 等具体路径（或 `bypass` HTML 请求）。
- mock（`scripts/mock/server.js`）缺 `/v1/usage`（网关 Bearer Key）与微信支付回调 resume-token 端点，`/key-usage` 结果态与支付回调成功态无法截图；LinuxDo / WeChat 品牌色 hex 字面量按 brief 第 2 条例外保留。
- 像素比对方法（2026-09-04 修正）：参考图 PNG 0–31 为标签栏、32/33 为 artboard 边框，内部从 y=34 开始；截图高度 = 参考图高 − 35（登录 860、仪表盘/账号 1080、密钥 900、设置 1180），`pixel-diff.mjs --ref-offset 0,34`，`shift-profile.mjs --ref-offset 0,34 --x0 224`；元素级定位用 `UI_SHOTS_PROBE='sel|sel' node scripts/ui/shot.cjs …` 打印 top/left/h/w，与 headless Chrome 实测的原型几何（proto-probe，见提交内 `reference/proto-geometry.md`）逐项比对。组 5–6 的 `/login` 3.6% 是按旧口径（863 / offset 32）得到的，重新按 860 / offset 34 复核（2026-09-04，1440×860 / offset 34）：3.46%，7 个 120px 带 |dy|≤1，通过。
- 截图脚本主题参数：应用 `localStorage.theme` 只认 `light|dark`，mock `/setup/seed` 现在把 `glass-dark|glass-light` 归一化，`shot.cjs … glass-dark` 才会真正切到暗色（此前暗色截图实际是亮色，导致暗色 diff 虚高 98%）。`pixel-diff.mjs` 参数解析已修，`--ref-offset 0,34` 不再被当成热图路径（之前会在 `frontend/` 生成名为 `0,34` 的文件）。
- 壳层几何修正（组 7–9 复核时发现）：原型 header 为 content-box，`height:60px + padding-top:6px` 实际 66px；紧凑页头 h1 24px 的 normal 行高渲染为 35px、描述 13px 为 19px。已改 `AppHeader .topbar` 66px、`PageHeader`/`.page-title`/`.page-description` 行高、`TablePageLayout` 高度 100vh−98。修正后 `/keys` 页头（h1 74..109、描述 113..132、首块内容 146）与原型逐像素一致。
