## Why

前端在功能快速累积后出现了三类结构性负担：(1) 国际化包过大——zh / en 各约 8.8k 条、20k 行、420KB 原始 / 135KB gzip 的单一语言块在首屏整块下载，其中约 1.2k 条键（13%）已无任何引用，且消息在浏览器运行时编译；(2) 构建与检查慢——`npm run build` = `vue-tsc -b`（约 38s）+ `vite build`（约 25s），产物 225 个 chunk、JS 5.9MB 原始 / 1.6MB gzip，单个路由块 `AccountsView` 达 836KB（把 7.4k 行创建账号弹层与 5.9k 行编辑弹层一起打进去），`SettingsView` 401KB，`xlsx` 7.3MB 依赖被并入首屏 `vendor-ui`，完全未被引用的 `@lobehub/icons`（22MB）仍在依赖树中；(3) 死代码与重复实现——25 个源文件从入口不可达（约 3.7k 行），`components/common` 与新的 `components/ui` 各有一套 StatusBadge / StatCard / Toggle / Select / Pagination / Dialog，`ticket/` 与 `tickets/` 目录并存，渠道状态存在 V1 / V2 / 壳三份视图，`api/admin/settings.ts` 里 132 处 `legacy` 标记，357 处 `any`。这些问题拖慢每一次构建、测试与页面加载，也让正在进行的 Glass 重构不得不在两套组件之间做选择。

这次以一份可度量的基线（本提案附录）为起点，把清理、收敛与构建提速做成可验收的变更，并给出防止回退的门禁。

## What Changes

- **国际化瘦身与按需加载**：删除无引用键；按导航域拆分语言包（公共 / 用户 / 管理员 / 设置 / 运维 …）并按路由懒加载；用 `@intlify/unplugin-vue-i18n` 在构建期预编译消息、去掉运行时消息编译器；zh / en 键集合一致性成为门禁；首屏语言块从 ~420KB 原始降到 ≤ 120KB。
- **死代码清理**：删除入口不可达且无测试价值的文件（含 `OpsRuntimeSettingsCard`、`OpsEmailNotificationCard`、`StripePaymentInline`、`PlatformUsageBreakdown`、`PaymentMethodChart`、`TopUsersLeaderboard`、`ticket/TicketCategoryForm`、`common/Skeleton`、`common/StatusBadge`、`TicketInfoItem`、`ProfileAccountBindingsCard`、三个无用 `index.ts` 桶文件）；对仅被测试引用的 6 个文件逐个决定"接回 / 删除连同测试"；移除 `@lobehub/icons` 依赖；清理 `legacy` 分支与已确认不再使用的功能开关分支。
- **重复实现收敛**：`common/{StatusBadge,StatCard,Toggle,Input,TextArea,Select,Pagination,BaseDialog,Skeleton}` 收敛到 `components/ui/*`（`BaseDialog` 82 处、`Select` 61 处、`Pagination` 31 处引用通过兼容包装分批迁移）；`ticket/` 并入 `tickets/`；`OpsErrorDetailModal` 与 `OpsErrorDetailsModal` 合并；渠道状态 V1 / V2 依据默认模式收敛为一个视图族。
- **构建与检查提速**：`vue-tsc` 从 `build` 脚本移到独立 `typecheck`（CI 并行执行），`vite build` 不再串行等待类型检查；`xlsx`、`chart.js`、`driver.js`、`qrcode`、`marked` 改为按需动态导入；超大视图（`SettingsView` 14k 行、`CreateAccountModal` 7.4k、`GroupsView` 7k、`EditAccountModal` 5.9k）按 tab / 平台拆成懒加载子组件；chunk 大小预算与体积报告进入 CI。
- **代码健康门禁**：不可达文件、未使用依赖、未使用 i18n 键、zh/en 差集、`any` 数量、单文件行数、chunk 体积 五项指标写入脚本与 CI，并在每次 PR 上报告差值。
- **不改变**：任何用户可见行为、后端 API、路由路径、localStorage 键、功能开关语义；仅移除确认无引用的代码与文案。

## Capabilities

### New Capabilities

- `frontend-i18n-lean-loading`：语言资源的组织、按需加载、预编译、无引用键清理与 zh/en 一致性契约。
- `frontend-dead-code-removal`：不可达代码、重复组件、遗留分支与未使用依赖的识别标准与删除 / 收敛规则。
- `frontend-build-performance`：构建时长、产物体积、chunk 预算、按需加载与类型检查并行化的契约。
- `frontend-code-health-gates`：可持续的代码健康度量与 CI 门禁。

### Modified Capabilities

无。与 `glass-ui-redesign` 的关系：本变更的组件收敛以 `components/ui/*` 为目标，与 Glass 重构同向；执行顺序见 design。

## Impact

- **文件**：删除约 25 个源文件与对应死测试；`components/common/*` 中 9 个组件降级为兼容包装后逐步删除；`i18n/locales/**` 重组为按域目录；`vite.config.ts`、`package.json`、`tsconfig`、CI 工作流；新增 `scripts/{ui-lint,i18n-diff,i18n-unused,deps-check,dead-files,bundle-report}.mjs`。
- **依赖**：移除 `@lobehub/icons`；新增 devDependency `@intlify/unplugin-vue-i18n`、`knip`（或等价）、`rollup-plugin-visualizer`。
- **测试**：删除仅服务于死代码的 spec；其余不变；新增门禁脚本的自测。
- **运行时**：首屏 JS 减少约 300KB 原始（语言块 + xlsx + vendor 拆分）；语言切换改为按域懒加载；CSP 环境下仍无 `unsafe-eval`（预编译消息）。
- **风险**：i18n 键删除依赖静态引用分析，动态拼接键需要显式登记白名单；组件收敛期间两套 API 并存，需要包装层保证 props / emits 兼容。

## Execution References

- `baseline.md`：2026-09-03 的度量基线（构建时长、chunk 表、依赖体积、不可达文件清单、无引用键统计、重复组件引用计数、`any` 与 `legacy` 分布）。
- `verification.md`：门禁脚本、目标阈值与执行记录。
