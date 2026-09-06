## Purpose

定义 Glass 重构的 UI 一致性与观感验收门禁：哪些检查必须自动化、哪些必须由截图评审完成、每个门禁的通过阈值，以及一个页面被判定"达标"的完整定义，使一致性与观感成为可复核的交付物而不是主观评价。

## ADDED Requirements

### Requirement: 静态一致性扫描零命中
系统 SHALL 提供 `frontend/scripts/ui-lint.mjs`（并接入 `npm run lint:ui` 与 vitest `designTokens.spec.ts`），扫描 `src/**/*.{vue,ts,css}`（排除 `__tests__`）：`dark:`、`bg-white`、`border-gray-`、`text-gray-`、`bg-gray-`、`rounded-2xl`、`rounded-3xl`、`shadow-lg`、`shadow-md`、`shadow-xl`、`shadow-2xl`、`text-lg font-semibold`、`fonts.googleapis.com`、`fonts.gstatic.com` MUST 为 0 命中；十六进制 / `rgb(` / `hsl(` 颜色字面量 MUST 只出现在白名单（`style.css`、`styles/tokens.css`、`utils/platformTile.ts`、`components/icons/**`、`components/common/PlatformIcon.vue`、`components/common/ModelIcon.vue`、支付品牌按钮类、第三方登录品牌标记）；`views/**` scoped 样式中颜色 / 圆角 / 阴影 / 字号 / 字重声明 MUST 为 0（白名单：`HomeView.vue`、`auth/**` 装饰）。

#### Scenario: 提交前扫描
- **WHEN** 运行 `npm run lint:ui`
- **THEN** 退出码 MUST 为 0，输出每类模式的命中数均为 0，白名单外的颜色字面量列表为空

#### Scenario: 引入违规类
- **WHEN** 某视图新增 `class="rounded-2xl shadow-lg"`
- **THEN** `lint:ui` MUST 失败并打印文件路径、行号与匹配片段

### Requirement: 国际化完整且不泄漏键名
zh 与 en 的键集合 MUST 完全一致（`scripts/i18n-diff.mjs` 差集为空）；同一文件 MUST 无重复键（`vue-tsc` TS1117 为 0）；任何截图或渲染 DOM 中 MUST NOT 出现形如 `^[a-z]+(\.[a-zA-Z0-9_]+)+$` 的原始键名文本。

#### Scenario: 键集合比较
- **WHEN** 运行 `node scripts/i18n-diff.mjs`
- **THEN** 输出 `zh-only: 0, en-only: 0`

#### Scenario: 页面文案检查
- **WHEN** 对任意路由执行截图脚本并用 CDP 抓取 `document.body.innerText`
- **THEN** 文本中 MUST 不匹配原始键名正则

### Requirement: 截图矩阵覆盖全部路由
每条可渲染路由 MUST 至少产出 4 张截图：亮 1440、暗 1440、亮 390×844、暗 390×844（表单 / 弹层类页面再加"打开主弹层"一张）；参数化路由用 mock 数据 id；截图存放 `openspec/changes/glass-ui-redesign/screens/<route-slug>/{light,dark,mobile-light,mobile-dark}.png`，并在 `verification.md` 登记。截图 MUST 在 mock 后端有数据与无数据两种状态下至少各覆盖一次列表页。

#### Scenario: 生成矩阵
- **WHEN** 运行 `bash scripts/ui-shots.sh all`
- **THEN** 64 条路由（参数化路由取样例）MUST 全部产出 4 张图，缺失即失败并列出缺失项

### Requirement: 观感评分表逐页达标
每个页面 MUST 由评审者（lead 代理或人工）按 `ui-standards.md` 的 10 项观感评分（层级、留白与节奏、对齐与栅格、密度、对比度与可读性、玻璃层次与阴影、状态完整性、微交互、暗色对等、移动端）每项 0–2 分打分并记录在 `verification.md`；总分 MUST ≥ 17/20，且"暗色对等""状态完整性""对齐与栅格"MUST 各为 2 分；原型出稿的 6 个屏幕 MUST 为 20/20 且与参考图逐区块对照无 >1px 偏差。

#### Scenario: 页面评审
- **WHEN** 评审 `/admin/users` 四张截图
- **THEN** `verification.md` MUST 出现该路由的 10 项分数、总分、对照原型 04 的差异列表与处理结论；总分 < 17 的页面 MUST 回到任务清单重做

### Requirement: 暗色主题对等
暗色截图 MUST 与亮色截图在布局、层级、状态可见性上完全对等；暗色下文字与背景对比度 MUST ≥ 4.5:1（正文）/ 3:1（大字与图标），徽章 tone 文字在其底色上 ≥ 4.5:1；MUST NOT 出现亮色残留（白底块、浅灰边、黑色文字在深底）。

#### Scenario: 对比度抽检
- **WHEN** 对暗色下 `--muted` 文字在 `--surface` 上、`--danger-text` 在 `danger 14%` 底上做对比度计算
- **THEN** 结果 MUST 分别 ≥ 4.5:1

#### Scenario: 图表暗色
- **WHEN** 暗色渲染 `/dashboard` 的 chart.js 图表
- **THEN** 网格线 MUST 为 `--border`，刻度文字 `--muted`，tooltip 为 `.tooltip-bubble` 样式，无白色背景块

### Requirement: 类型检查与测试全绿
`cd frontend && vue-tsc --noEmit` MUST 0 错误；`vitest run` MUST 0 失败；与 DOM 结构合法变化相关的测试只允许更新选择器与期望值，MUST NOT 删除断言或跳过用例；新增结构测试锁定：按钮高度 / 圆角、字段 36px、徽章 22px、表头 42px、行高 ≥ 58、分页 28px、弹层宽度档位、`SettingRow` 240px 网格。

#### Scenario: 组件尺寸结构测试
- **WHEN** 运行 `vitest run src/components/ui/__tests__/designSystem.structure.spec.ts`
- **THEN** 断言 MUST 覆盖上述尺寸并全部通过

### Requirement: 功能与锚点零丢失
每个视图改造前后，`data-tour`、`data-testid`、`id`、`aria-label`、路由跳转目标、事件处理器名（`@click="xxx"` 的 xxx 集合）、i18n 键引用集合 MUST 由脚本 `scripts/anchor-diff.mjs <file>` 比较为"新 ⊇ 旧"；`v-model` 绑定集合 MUST 完全相同。

#### Scenario: 账号页改造后比对
- **WHEN** 运行 `node scripts/anchor-diff.mjs src/views/admin/AccountsView.vue --base HEAD`
- **THEN** 输出丢失的锚点 / 处理器 / v-model 列表 MUST 为空

### Requirement: 可访问性基线
所有交互控件 MUST 可键盘到达并有可见焦点环；图标按钮 MUST 有 `aria-label`；弹层 MUST 有 `role="dialog"`、`aria-modal`、焦点陷阱；表格表头 MUST 是 `<th scope="col">`；状态徽章颜色 MUST 伴随文字（不得只靠颜色）；`prefers-reduced-motion` 下 MUST 禁用非必要动画。

#### Scenario: 键盘遍历筛选行
- **WHEN** 在 `/admin/accounts` 用 Tab 依次聚焦搜索、筛选药丸、分段控件、列设置按钮
- **THEN** 每个控件 MUST 显示主色焦点环且顺序与视觉顺序一致

### Requirement: 门禁顺序与提交纪律
任何一组任务在提交前 MUST 依次通过：`lint:ui` → `i18n-diff` → `anchor-diff`（组内文件）→ `vue-tsc` → `vitest` → 截图矩阵（组内路由）→ 观感评分登记；任一失败 MUST NOT 提交。每组提交信息 MUST 引用任务组编号。

#### Scenario: 提交一组列表页
- **WHEN** 管理员列表批次 1 完成
- **THEN** `verification.md` MUST 记录 7 项门禁结果与时间，提交信息形如 `feat(glass): admin list pages batch 1 (tasks 11.1–11.8)`
