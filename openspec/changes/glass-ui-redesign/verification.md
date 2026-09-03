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

## 备注（组 5–6 发现，超出任务范围，待后续处理）

- 路由 `/:pathMatch(.*)*` 未设 `meta.requiresAuth: false`，游客访问未知路径会被守卫重定向到 `/login`（首次提交起就存在；router 不在本变更范围）。建议在 frontend-health-cleanup 组 7 顺带修复。
- `vite.config.ts` 开发代理 `'/setup'` 前缀会拦截 SPA 路由 `/setup` 本身；建议收窄为 `/setup/status` 等具体路径（或 `bypass` HTML 请求）。
- mock（`scripts/mock/server.js`）缺 `/v1/usage`（网关 Bearer Key）与微信支付回调 resume-token 端点，`/key-usage` 结果态与支付回调成功态无法截图；LinuxDo / WeChat 品牌色 hex 字面量按 brief 第 2 条例外保留。
- 像素比对方法已固化到 brief-common（截图高度 = 参考图高 − 32、`--ref-offset 0,32`、`shift-profile.mjs` 逐带 dy/dx）。
