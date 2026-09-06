## Context

见 `proposal.md - Why`。设计层面需要知道的当前状态：

- 原型是 Claude Design 的 `.dc.html` 画布（8 个 1440px 画板 + 1 个 390px 移动端画板），令牌与 AgentBox `glass-color.css` 同名；组件规范画板给出了 Button / StatusBadge / Field / Select / Toggle / Segmented / SidebarItem / 类型阶 / 圆角的精确 px；页尾给出 64 条路由 → 12 种模板的覆盖矩阵、"从原 sub2api 保留的组件与交互"和组件映射表。
- 工作树里已有一版基础层：`styles/tokens.css`（令牌）、`style.css`（全局类 `.btn* .field .badge .tag .glass-card* .table* .dropdown* .modal* .toast* .summary-chip .filter-pill .segmented .switch .progress* .empty-state* .skeleton .notice*` 等）、`components/ui/`（23 个组件）、`AppSidebar` / `AppHeader` / `CommandPalette` / `MobileDrawer`、首页、登录、注册。这些已通过截图确认与原型一致，本变更在其上做**核对与修正**而不是推倒重来。
- 上一轮 20 个代理并行导致会话限额 429，全部中断；三个视图处于半改状态无法编译；`index.html` 引入了 Google Fonts，与既有测试 `designTokens.spec.ts` 的"不得引用 Google Fonts"规则冲突。
- 视觉验证基础设施已恢复：Node mock 后端（:8091，含 `/setup/seed` 登录态种子路由）+ Vite（:3777）+ CDP 截图脚本，可对任意路由按角色 / 主题 / 语言 / 视口出图。
- 前端约束：Vue 3 + `<script setup>` + Tailwind 3 + vue-i18n；DataTable.vue 是唯一表格引擎（sticky 列、虚拟滚动、列设置持久化、<768px 自动卡片）；路由、store、API 客户端不可动。

## Goals / Non-Goals

**Goals:**

- 令牌 → 组件 → 页面模块 → 页面模板 → 全站 五级一致：任何页面截图放到一起都像同一个产品的同一版本。
- 原型直接出稿的 6 个屏幕（首页、登录、管理员仪表盘、账号管理、API 密钥、系统设置）与 390px 画板做到像素级还原（尺寸误差 ≤ 1px，色值直接引用令牌）。
- 其余 58 条路由按 12 种模板配方补全，结构、间距、控件尺寸与出稿页面完全一致。
- 亮 / 暗 / 桌面 / 移动 四种形态全部达标，暗色不是"能看"而是与亮色同等完成度。
- 全部旧样式残留清零，验收可以用脚本和截图矩阵客观复核。
- 现有功能、锚点、测试、i18n 键零丢失。

**Non-Goals:**

- 不改后端、路由表、store、API 客户端、权限逻辑、功能开关语义。
- 不新增业务功能，不改变表单校验与提交行为。
- 不做主色（accent）切换 UI；令牌层预留 `data-accent`，但本变更只交付 Blue。
- 不迁移 chart.js 到其它图表库；只对其做令牌化样式约束。
- 不重写 DataTable 引擎的交互逻辑。

## Decisions

1. **令牌是唯一颜色来源，Tailwind 调色板类映射到令牌而不是删除。**
   仓库有 ~200 个 Vue 文件在用 `bg-red-50 / text-emerald-600` 之类的类。逐个删掉成本高、易漏；把 Tailwind `colors` 重映射为 `color-mix(in oklch, var(--token) …)` 后旧模板立即上主题，然后再用扫描门禁把新代码限制在语义类（`text-muted / bg-surface-2 / border-line / text-danger-text`）上。
   备选：删除调色板类、只允许语义类——需要一次性改 200+ 文件，且中途无法验收；否决。

2. **`dark:` 变体全面禁止，明暗完全由令牌切换。**
   原型的暗色只是一组令牌覆盖；保留 `dark:` 会产生两套真值。扫描门禁把 `dark:` 计为 0 命中。当前已为 0，必须保持。

3. **显示字体自托管，不引用 Google Fonts。**
   原型用 Manrope 800 做标题与大数字、Inter 做正文、JetBrains Mono 做代码。既有测试禁止 Google Fonts（国内部署不可达、隐私），因此把三套 woff2（OFL 许可，仅所需字重：Manrope 600–800、Inter 400–700、JetBrains Mono 400/500）放入 `frontend/public/fonts/`，`index.html` 用 `<link rel=preload>` + `@font-face`，中文回落 PingFang SC / Hiragino Sans GB / Microsoft YaHei / Noto Sans SC。
   备选：只用系统字体——标题失去原型的 Manrope 特征，观感明显下降；否决。若用户不接受体积（约 350KB），退化方案是只托管 Manrope。

4. **样式下沉到组件与全局类，页面 `<style scoped>` 只允许布局。**
   页面级 scoped 样式只允许 `display / grid / gap / width / margin` 类布局声明和 `@media` 断点；出现颜色、圆角、阴影、字号即视为违规（门禁 4.4）。这样一个组件修一次全站生效。
   备选：允许页面自定义——就是现状，导致不一致；否决。

5. **DataTable 只换皮不换引擎；列内容用"单元格组件"统一。**
   名称/ID、平台底板、类型标签、状态徽章、用量窗口、今日统计、操作按钮各自是可复用的单元格组件（`components/common/cells/*` 或既有 `components/account/*Cell.vue` 改造），账号 / 密钥 / 用户 / 订单等列表共用。

6. **12 种页面模板落成 5 个布局骨架组件。**
   `TablePageLayout`（ListPage）、`DashboardPageLayout`、`DetailPageLayout`、`SettingsPageLayout`、`PublicPageLayout`（Landing / Auth / Callback / Setup / NotFound 共用背景与卡片壳）。页面只填插槽。

7. **弹层只有三种壳：`UiModal`、`UiDrawer`、`ConfirmDialog`；`BaseDialog` 作为 `UiModal` 的兼容别名保留。**
   账号 / 密钥 / 设置里的 30 多个对话框逐个迁移；宽度只允许 440 / 560 / 720 / 960 四档。

8. **i18n 补齐策略：只新增，不删除、不重命名。**
   新键同时写 zh / en；门禁脚本比较两侧键集合差异为 0；原始键名（如 `admin.dashboard.heroTitle`）出现在截图里即判失败。

9. **验证以截图矩阵为准，代理必须"看图再报告"。**
   每个页面任务的完成定义包含：亮 1440、暗 1440、亮 390 三张图（表单类页面再加暗 390），代理 Read 图片并逐项对照配方；lead 复核。mock 数据缺失导致的空白要说明，不许改视图"糊"过去。

10. **执行节奏：每批 4–6 个代理，批间提交。**
    上一轮 20 个并行代理烧尽限额且全部丢失。按 `tasks.md` 分组顺序执行；每组内文件所有权互斥；每组完成、门禁通过后 `git commit` 一次；lead 只做基础层、门禁与合并。

11. **原型渲染为参考图。**
    用 Chrome 把 `redesign.html` 的每个画板渲成 PNG 放到 `openspec/changes/glass-ui-redesign/reference/`（或 scratchpad），代理对照参考图而不是 200KB 的 HTML。

## Risks / Trade-offs

- [原型只覆盖 6 个屏幕，其余页面靠配方补全，代理理解偏差] → `ui-standards.md` 给每种模板逐层 px 配方 + 反模式清单；同模板页面由同一代理或同一批完成，lead 横向比对截图。
- [SettingsView 13k 行、AccountsView 与 KeysView 各 2k+ 行，整文件重写易丢功能] → 按 tab / 按列 / 按对话框逐块替换，每块后跑该视图的 spec；`data-tour` / `data-testid` / `id` 锚点用 grep 前后比对必须一致。
- [DataTable 引擎与新单元格组件耦合，sticky 列背景 / hover 色不一致] → 单元格背景一律透明，sticky 列背景由引擎给 `var(--surface)` 并跟随 hover；结构测试锁定。
- [chart.js 图表在暗色下网格线 / tooltip 用默认色] → 提供 `chartTheme()` helper 读令牌，所有图表实例只从它取色；暗色截图核对。
- [自托管字体增加首屏体积] → 只带所需字重、`font-display: swap`、preload 仅 Manrope 800 与 Inter 400；总量控制在 400KB 内。
- [并行代理同时编辑 locale 文件产生冲突或重复键] → locale 文件只允许 `Edit` 加唯一锚点，禁止 `Write`；门禁做重复键与 zh/en 差集检查。
- [旧测试断言依赖旧 DOM 类名] → 只更新合法变化的选择器，不删除断言；每组提交前全量 vitest。
- [再次触发会话限额] → 决策 10；另外每个代理任务限定在 1 个视图或 ≤ 6 个组件，并要求在中途每完成一个文件就保证可编译。

## Migration Plan

1. 组 0（止血）：修复三个破损视图与重复键，字体自托管，tsc / vitest 全绿，提交基线。
2. 组 1–4（基础层）：令牌核对、全局类与 ui 组件核对、common 组件、布局壳。提交。
3. 组 5–6（公开页与认证页）。提交。
4. 组 7–10（原型出稿的 4 个工作区页面：仪表盘、账号、密钥、设置）。提交。
5. 组 11（管理员列表与详情页，分 3 批）。每批提交。
6. 组 12（用户侧页面，分 3 批）。每批提交。
7. 组 13–14（暗色与移动端全站核对）。提交。
8. 组 15（门禁脚本固化、README、最终截图矩阵）。提交并可合并。

回滚：每组一个提交，任意一组出问题只需 revert 该组；令牌层与组件层向后兼容，页面未迁移前仍可通过 Tailwind 映射正常显示。

## Resolved Questions

- 自托管字体：用户已接受（2026-09-03）。只带 latin / latin-ext 子集与所需字重，中文回落系统字体。
- 移动端底部是否需要 Tab Bar（原型 390 画板只有抽屉 + FAB，没有 Tab Bar）。默认按原型不做；若要做，追加一个任务，不影响其它任务。
