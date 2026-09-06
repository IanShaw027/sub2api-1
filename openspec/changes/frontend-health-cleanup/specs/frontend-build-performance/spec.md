## Purpose

定义前端构建时长、产物体积、分块预算、按需加载与类型检查并行化的契约，使本地构建与 CI 的耗时和首屏负载有明确上限并可持续度量。

## ADDED Requirements

### Requirement: 构建与类型检查并行
`npm run build` MUST 只执行 `vite build`；`npm run typecheck` MUST 独立执行 `vue-tsc --noEmit`；CI MUST 并行运行 `typecheck`、`lint:check`、`test:run`、`build` 并把 `typecheck` 设为必需检查；`vite-plugin-checker` MUST 仅在开发模式注册。

#### Scenario: 本地构建
- **WHEN** 在基线机器（8 核）执行 `npm run build`
- **THEN** 耗时 MUST ≤ 20s（基线：`vite build` 24.9s + `vue-tsc` 38s 串行 ≈ 63s）

#### Scenario: 类型错误
- **WHEN** 源码含类型错误
- **THEN** `npm run typecheck` MUST 失败，CI MUST 阻止合并，`npm run build` 仍可产出用于预览

### Requirement: 产物体积预算
构建产物 MUST 满足：首屏关键 JS（入口 + `vendor-vue` + `vendor-i18n` + `core` 语言块 + 布局壳）≤ 700KB 原始 / 250KB gzip；任意单个路由块 ≤ 300KB 原始；任意 vendor 块 ≤ 250KB 原始；CSS ≤ 200KB 原始；chunk 总数 ≤ 180。`scripts/bundle-report.mjs` MUST 输出与基线的差值并在超预算时失败。

#### Scenario: 基线
- **WHEN** 在 2026-09-03 基线构建
- **THEN** 报告 JS 5.87MB / 1.60MB gzip，225 块，`AccountsView` 836KB、`vendor-ui` 431KB、`SettingsView` 401KB、语言块 zh 419KB / en 437KB、`vendor-misc` 292KB 超预算

#### Scenario: 超预算提交
- **WHEN** 某提交使 `UsersView` 块从 110KB 增至 320KB
- **THEN** `bundle-report` MUST 失败并指出块名、当前值、预算与基线

### Requirement: 重依赖按需加载
`xlsx`、`chart.js` + `vue-chartjs`、`driver.js`、`qrcode`、`marked` + `dompurify`、`@stripe/stripe-js`、`@airwallex/components-sdk` MUST 只在使用点动态导入并各自独立分块；`index.html` 首屏 MUST NOT 引用包含这些库的块。

#### Scenario: 打开首页
- **WHEN** 打开 `/home`
- **THEN** 网络面板 MUST 不出现 `xlsx`、`chart`、`driver`、`qrcode`、`marked` 相关块

#### Scenario: 导出 Excel
- **WHEN** 用户在使用记录页点击导出
- **THEN** 系统 MUST 在点击后加载 `vendor-xlsx` 块并完成导出，按钮显示 loading 态

### Requirement: 超大源文件拆分
单个 `.vue` / `.ts` 源文件 MUST ≤ 1,500 行（语言文件与生成文件除外）；超过者 MUST 按 tab / 平台 / 弹层机械拆分为懒加载子组件，行为与测试不变。

#### Scenario: 基线
- **WHEN** 统计基线
- **THEN** 报告 `SettingsView.vue` 14,077、`CreateAccountModal.vue` 7,464、`GroupsView.vue` 7,015、`EditAccountModal.vue` 5,932、`AccountsView.vue` 3,498、`BatchImageGuideView.vue` 2,695、`BulkEditAccountModal.vue` 2,443、`KeysView.vue` 2,443、`RiskControlView.vue` 2,372、`ProxiesView.vue` 2,140、`DashboardView.vue` 2,123、`UsersView.vue` 1,909、`UseKeyModal.vue` 1,869、`AccountUsageCell.vue` 1,759、`ChannelsView.vue` 1,721、`RedeemView.vue` 1,684、`OpsDashboardHeader.vue` 1,628 超限

#### Scenario: 拆分后
- **WHEN** `CreateAccountModal` 按平台拆分
- **THEN** `AccountsView` 路由块 MUST ≤ 300KB，各平台面板为独立块，`CreateAccountModal.spec` 全部通过

### Requirement: 测试套件时长
`vitest run` 全量 MUST ≤ 45s（基线 60s，307 文件 / 2,114 用例）；删除死代码测试、共享 jsdom 环境与合并重复 setup 后达成。

#### Scenario: CI 测试
- **WHEN** CI 运行 `test:run`
- **THEN** 耗时 MUST ≤ 45s 且 0 失败
