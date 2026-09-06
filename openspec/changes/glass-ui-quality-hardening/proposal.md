# Glass UI 质量加固与功能保真

## Why

`glass-ui-redesign` 已完成大范围视觉迁移，但 Review 发现全站仍有会影响一致性、可读性和交付验证的遗留问题：暗色边框过重、工作区环境光重复绘制、状态动画被重复定义、设置页出现两个主保存按钮、API Key 表格在桌面宽度下列内容重叠、平板顶栏换行、列表分页位置不一致，以及若干组件存在双重样式真值。

同时，视觉改动必须满足功能保真：原有路由、导航入口、表格字段、筛选/排序/批量操作、弹窗、权限和移动端工作流不得因为折叠、默认隐藏、响应式压缩或组件迁移而丢失或不可达。

## What Changes

- 修复四项全站 P0：暗色边框、重复环境光、重复保存按钮、重复 `s2a-pulse`。
- 修复 API Key 表格可读性、分页位置、顶栏稳定性、激活态/焦点环/动效等 P1 一致性问题。
- 收敛 Button、StatusBadge、Dialog、StatCard、DataTable 的双重样式真值。
- 补齐空状态、骨架加载、tour 提示框、暗色 hover、移动端和无障碍验证。
- 建立改动前后路由、导航、字段、操作和锚点的功能保真检查。
- 修复 pnpm/lockfile 环境阻塞，使 typecheck、lint、UI lint、contrast 和测试可复现。
- 将 `--muted`、边框决策、截图和 UI 规范同步回设计稿与文档。

## Non-goals

- 不改变后端、API、Pinia store、权限、计费、功能开关或业务校验。
- 不删除原有路由、导航项、数据字段、列设置、批量操作和弹窗能力。
- 不新增 AgentBox/OpenClaw 业务模块；本变更只加固当前 Sub2API Glass 实现。
- 不迁移图表库，不重写 DataTable 引擎，不改变 URL、query、localStorage 键。

## Capabilities

### New Capabilities

- `glass-quality-hardening`: Consolidated Glass theme consistency, feature preservation, responsive layout and executable verification requirements.

### Modified Capabilities

- None. This change adds a hardening capability; the historical `glass-ui-redesign` delta specs remain intact.

## Impact

主要影响 `frontend/src/styles/tokens.css`、`style.css`、布局壳、Settings、Keys、DataTable、认证表单、共享 UI 组件、文档和前端验证脚本。所有视图层变更必须保留既有 `data-tour`、`data-testid`、DOM id 和可访问名称，除非在迁移记录中明确登记等价替代。
