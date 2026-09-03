## Why

`feat/glass-ui-redesign` 分支已经引入了 Glass 令牌和一批 `components/ui` 组件，但对 Claude Design 原型（`Sub2API Redesign.dc.html` + `Sidebar.dc.html`）的还原度很低：只有外壳、首页、登录和少数页面换了皮，其余 60 多条路由仍是旧 Tailwind 调色板样式与新令牌的混合体；同一页面里新旧单元格、按钮、徽章并存，暗色主题与移动端只在个别页面被检查过。上一轮并行重构在半途被会话限额打断，工作树里留下了 3 个白屏/无法编译的视图和 26 条失败测试。

这不是一次局部换肤，而是一次以原型为唯一视觉基准的**前端视图层重建**：需要一份可验收的正式规范——令牌 → 组件 → 页面模块 → 页面模板 → 全站——以及一份覆盖全部路由、带明确完成标准的任务清单，让后续分批实施的每一片都能被客观地判定"达标 / 不达标"，而不是靠感觉。

## What Changes

- **令牌层成为唯一颜色/尺寸来源**：`tokens.css` 与原型 `:root` / `[data-theme=glass-dark]` 逐值对齐（oklch 色值、阴影、字体栈、圆角），Tailwind 调色板类全部映射到令牌；视图层禁止硬编码颜色与 `dark:` 变体；显示字体改为仓库自托管，不再引用 Google Fonts。
- **共享组件库成为唯一视觉实现**：`components/ui/**`、`components/common/**`、`style.css` 全局类按原型 07 组件规范给出精确尺寸契约（按钮 34/42、字段 36、徽章 22、表头 42、行 58–60、分页 28、圆角 8/10/12/14/16 等）；页面只做数据拼装，不再各自定义卡片、按钮、输入框样式。
- **64 条路由归入 12 种页面模板并逐一重建**：LandingLayout、AuthLayout、CallbackStatus、SetupWizard、DashboardPage、ListPage、DetailPage、SettingsPage、ActionPage、PaymentFlow、EmbedPage、NotFound。原型直接出稿的 6 个屏幕做像素级还原；其余页面按同模板配方补全，并给出每条路由的结构要求。
- **模态框、抽屉、表单、空态、加载态、Toast、提示气泡统一**：所有弹层走 `UiModal`/`UiDrawer`/`ConfirmDialog`，所有表单行走 `SettingRow`/`FieldLabel` 节奏，所有空态走 `EmptyState`，禁止页面内自绘。
- **新增一致性与观感门禁**：遗留类扫描（`rounded-2xl` / `shadow-lg` / `bg-white` / 调色板类 / 十六进制色）零命中；每条路由 亮 × 暗 × 1440 × 390 四张截图进入评审矩阵；i18n 键 zh/en 双语完整；`vue-tsc` 与 `vitest` 全绿；观感评分表（层级、留白、对齐、密度、对比度、玻璃层次、状态完整性、动效、暗色对等、移动端）逐页打分并达到阈值。
- **保留现有行为**：`<script setup>` 逻辑、API、store、router、i18n 既有键、`data-tour` / `data-testid` / `id` 锚点、DataTable 引擎能力（sticky 列、虚拟滚动、列设置、移动端卡片）全部保留。
- **修复上一轮遗留的破损**：`admin/RedeemView.vue`（新旧模板叠加）、`user/KeysView.vue`（模板引用缺失的 helper）、`admin/TicketDetailView.vue`（缺 3 个 handler）、`zh/misc.ts` 重复键。

## Capabilities

### New Capabilities

- `glass-design-tokens`：主题令牌、字体、圆角、阴影、间距节奏、明暗主题与主色切换的契约；Tailwind 映射与禁止项。
- `glass-ui-components`：共享基础组件（按钮、字段、选择器、开关、分段控件、徽章、标签、卡片、统计卡、表格、分页、筛选行、设置行、弹层、Toast、空态、骨架、进度条、品牌底板）的尺寸、状态与行为契约。
- `glass-page-templates`：12 种页面模板的结构契约、每条路由与模板的映射、原型直接覆盖屏幕的像素级要求、响应式与移动端要求。
- `glass-ui-quality-gates`：UI 一致性与观感验收门禁——静态扫描、截图矩阵、暗色对等、i18n 完整性、可访问性对比度、类型检查与测试、观感评分表。

### Modified Capabilities

无。仓库当前 `openspec/specs/` 下没有已归档的前端视觉 capability；本变更不改变任何后端或已归档能力的需求语义。

## Impact

- **前端视图层**：`frontend/src/style.css`、`styles/tokens.css`、`tailwind.config.js`、`index.html`、`components/ui/**`、`components/common/**`、`components/layout/**`、全部 `views/**`、`components/{account,admin,auth,channels,charts,home,keys,modelPlaza,payment,ticket,tickets,user}/**`、`features/{channel-monitor-v2,creation,prompt-audit}/**`。
- **i18n**：`src/i18n/locales/zh/**` 与 `en/**` 新增展示文案键，不删除、不重命名既有键。
- **静态资源**：新增自托管字体文件（Manrope / Inter / JetBrains Mono，OFL 许可）到 `frontend/public/fonts/`。
- **测试**：更新与 DOM 结构合法变化相关的选择器断言；新增令牌/遗留类扫描测试与组件尺寸结构测试；不删除既有断言。
- **不受影响**：后端、路由表、Pinia store、API 客户端、类型定义、功能开关、权限与鉴权、Backend 模式与简单模式的显隐逻辑。
- **兼容性**：无对外 API 变化；页面 URL、查询参数、localStorage 键不变；用户可见的功能入口一个不少。
- **执行方式**：按 `tasks.md` 分 15 组、每组 4–6 个并行代理、每组完成后提交一次；上一轮 20 个代理同时运行触发 429 限额的教训写入 design。

## Execution References

- `ui-standards.md`：一致性与观感标准速查表（精确 px、类型阶、控件尺寸、模板配方、观感评分表、反模式清单）。
- `route-matrix.md`：64 条路由 × 模板 × 负责文件 × 当前状态 × 任务编号。
- `verification.md`：门禁执行脚本、截图矩阵、评审记录格式。
