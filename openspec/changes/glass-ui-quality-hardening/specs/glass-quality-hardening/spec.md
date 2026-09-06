# Glass Quality Hardening

## ADDED Requirements

### Requirement: 全站玻璃主题一致性

The frontend SHALL use shared semantic tokens, one workspace ambient layer and one status pulse definition across both Glass themes.

#### Scenario: 暗色主题层次
- **GIVEN** `html[data-theme="glass-dark"]`
- **WHEN** 渲染卡片、表格、侧栏、分区线和控件
- **THEN** 装饰性边界使用 `--border`，控件边界使用 `--border-strong`，且不存在额外 dark theme CSS 分支

#### Scenario: 工作区背景
- **WHEN** 页面滚动或切换路由
- **THEN** 环境光只绘制一次，不出现双影或亮度叠加

#### Scenario: 状态动画
- **WHEN** 渲染 live/status/pulse 元素
- **THEN** 使用唯一 `s2a-pulse` 定义，包含 3px 常驻光环，并尊重 `prefers-reduced-motion`

### Requirement: 功能保真

The redesign SHALL preserve existing information, discoverable navigation, actions, permissions, settings and workflows; it SHALL NOT newly hide selected fields or remove controls to fit a viewport. Existing overflow menus, table scrolling and complete mobile cards MAY provide equivalent access. Formerly direct list commands SHALL remain directly discoverable; the explicitly specified compact topbar More menu is a separate responsive layout contract.

#### Scenario: 路由和导航
- **WHEN** 比较重构前后的路由和导航基线
- **THEN** path 集合、权限、feature flag 和移动 Drawer 入口不减少

#### Scenario: 列表信息
- **WHEN** 在 1440px、1024px、768px 和 390px 渲染 Keys/Accounts/Usage 等列表
- **THEN** 原有关键字段、筛选、排序、批量操作、导出、分页和行菜单仍可见且可操作；不得出现文字重叠

#### Scenario: 弹窗和锚点
- **WHEN** 打开创建、编辑、删除、测试、复制、重新授权和详情弹窗
- **THEN** 原有事件、确认、取消、loading、错误态以及 `data-tour`/`data-testid`/id 锚点保持有效

#### Scenario: 信息数值与精度
- **WHEN** 查看移动 Keys、Profile 和风控记录筛选
- **THEN** 累计计费与可重置额度分别展示，所有已配置限额周期和重置时间可读，账户角色/状态/余额/并发/注册时间保持可见，风控起止筛选保留小时和分钟精度

#### Scenario: 端点信息
- **WHEN** 使用桌面或移动端的 API 端点
- **THEN** 所有端点、管理员配置的说明、复制与测速入口均保留；协议提示不得覆盖管理员非空说明

#### Scenario: 原有直接操作
- **WHEN** 查看分组、代理、订阅、工单、Keys 和监控行，或选中账户进行批量操作
- **THEN** 原本直接显示的操作保留显式入口及既有禁用/权限条件，按选中项编辑与按全部筛选结果编辑不相互替代

#### Scenario: 输入与焦点
- **WHEN** 使用键盘确认 Passkey 删除，或应用/管理工单快捷回复模板
- **THEN** 保留原有 Enter 提交与密码禁用条件，模板正文摘要可直接阅读，操作完成后焦点回到仍可见的正确按钮且无循环 ref 类型错误

#### Scenario: 图表数据保真
- **WHEN** 图表容器变窄
- **THEN** 时间轴可稀疏标签但不丢弃数据点，首尾时间仍可识别；离屏截图必须在实际绘制就绪后采集，不能将空画布误判为通过

### Requirement: 页面模板一致性

The frontend SHALL share layout and control primitives while preserving their existing data, event and accessibility contracts.

#### Scenario: ListPage
- **WHEN** a list view renders its table and pagination
- **THEN** 分页位于表格卡 footer，表头、行高、空态、loading 和筛选条遵循统一原语

#### Scenario: SettingsPage
- **WHEN** an administrator views or edits settings
- **THEN** 使用 SettingsPageLayout/SettingRow；页面只有一个主保存动作，dirty、reset 和 loading 语义一致

#### Scenario: Responsive topbar
- **WHEN** the viewport uses a compact desktop or tablet width
- **THEN** 768–1023px 顶栏不换行；次要动作通过 More 菜单访问，页面内容起点稳定

### Requirement: 质量门禁

The change SHALL provide reproducible automated checks and browser evidence, with explicit limitations for unexecuted or mock-only workflows.

#### Scenario: 静态与运行验证
- **WHEN** the pinned frontend toolchain runs validation
- **THEN** `vue-tsc`、ESLint、`ui-lint`、contrast、i18n parity、功能保真 diff 和 Vitest 均可在固定 pnpm 版本下执行

#### Scenario: 截图矩阵
- **WHEN** the seven core routes are reviewed at 1440, 1024, 768 and 390 pixels in both themes
- **THEN** 核心页面至少完成 light/dark × desktop/tablet/mobile 截图，并记录空态、有数据态、弹窗和下拉交互态
