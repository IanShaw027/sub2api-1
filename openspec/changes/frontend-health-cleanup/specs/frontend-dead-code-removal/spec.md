## Purpose

定义前端死代码、重复实现、遗留分支与未使用依赖的识别标准和处置规则，使代码库中每个源文件、组件与依赖都有唯一、可达、被使用的理由。

## ADDED Requirements

### Requirement: 入口不可达的源文件为零
系统 SHALL 提供 `scripts/dead-files.mjs`：从 `src/main.ts`、`src/App.vue`、`src/router/index.ts` 出发解析 `import / import() / require / components 注册 / defineAsyncComponent` 构建可达图；`src/**`（排除测试、`*.d.ts`、语言文件）中不可达的文件数 MUST 为 0；仅被测试引用的文件 MUST 被接回入口或连同测试删除，MUST NOT 长期存在。

#### Scenario: 基线
- **WHEN** 在 2026-09-03 基线上运行
- **THEN** 报告 25 个不可达文件（14 个无人引用、6 个仅测试引用、5 个被其它不可达文件引用），共约 3,697 行

#### Scenario: 新增孤儿组件
- **WHEN** 提交新增 `components/foo/Bar.vue` 但无任何引用
- **THEN** 门禁 MUST 失败并列出该文件

### Requirement: 同一 UI 概念只有一个实现
`components/ui/*` MUST 是按钮、字段、选择器、开关、分页、弹层、徽章、统计卡、骨架、空态的唯一实现；`components/common` 中同职责组件 MUST 先降级为向 `ui` 转发的兼容包装（保持 props / emits / slots / 测试通过），引用归零后 MUST 删除；`components/ticket/` MUST 并入 `components/tickets/`；`views/admin/ops/components/OpsErrorDetailModal.vue` 与 `OpsErrorDetailsModal.vue` MUST 合并为一个。

#### Scenario: BaseDialog 包装期
- **WHEN** `common/BaseDialog.vue` 改为 `UiModal` 包装
- **THEN** 82 个引用方 MUST 无需改动即通过各自 spec，`BaseDialog.spec.ts` 全部断言通过，外观与 `UiModal` 一致

#### Scenario: 收敛完成
- **WHEN** 运行 `scripts/dead-files.mjs --duplicates`
- **THEN** `common/{StatusBadge,StatCard,Toggle,Input,TextArea,Select,Pagination,BaseDialog,Skeleton}.vue` MUST 不存在或引用数为 0 并列入下一批删除

### Requirement: 遗留分支与版本后缀清理
被 `legacy` / `@deprecated` / `V1` 标记且默认关闭、无部署依赖的分支 MUST 删除或提取到独立兼容模块；每个保留的 `legacy` 标记 MUST 附带保留原因与移除条件注释；渠道状态 V1 / V2 / 壳视图 MUST 在确认默认模式后收敛（保留的一方成为唯一视图）。

#### Scenario: legacy 设置字段
- **WHEN** 审阅 `api/admin/settings.ts` 中 132 处 `legacy`
- **THEN** 每处 MUST 归类为 保留（附原因）/ 删除（附提交），不得有未归类项

### Requirement: 未使用依赖为零
`package.json` 中每个 `dependencies` 项 MUST 在 `src/**`、`vite.config.ts`、`index.html`、`postcss / tailwind` 配置或脚本中至少有一处引用；`scripts/deps-check.mjs`（或 `knip`）报告的未使用依赖 MUST 为 0；两个功能重叠的依赖 MUST 只保留一个。

#### Scenario: 基线
- **WHEN** 运行依赖检查
- **THEN** MUST 报告 `@lobehub/icons`（22MB，0 引用）为未使用；移除后 `node_modules` 体积下降且 `ModelIcon.vue` / `ProviderIcon.vue` 的内联 SVG 不受影响

### Requirement: 测试与源码同生共死
每个 spec 文件 MUST 对应存在且入口可达的源文件；删除源文件时 MUST 同步删除仅覆盖它的 spec；MUST NOT 为保住测试而保留死代码。

#### Scenario: 删除 PaymentMethodChart
- **WHEN** 删除 `components/admin/payment/PaymentMethodChart.vue`
- **THEN** 若存在仅测试它的 spec，MUST 一并删除；`vitest run` 仍为 0 失败
