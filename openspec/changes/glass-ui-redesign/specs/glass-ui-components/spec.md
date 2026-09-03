## Purpose

定义 Sub2API 前端共享基础组件（`components/ui`、`components/common`、`style.css` 全局类）的尺寸、状态与行为契约，使所有页面只能通过这些组件与类获得视觉表现，页面不得自绘按钮、字段、徽章、卡片与弹层。

## ADDED Requirements

### Requirement: 按钮尺寸与变体
系统 SHALL 提供按钮变体 `primary | secondary | ghost | danger | success | warning | icon` 与尺寸 `xs(26) | sm(28–32) | md(34) | lg/hero(42)`。默认（md）高 34px、圆角 10、内边距 primary `0 14px` / 其它 `0 12px`、字号 13/600、图标 16px、图标与文字间距 6px。primary 背景 `var(--accent)`、白字、阴影 `0 8px 20px -10px var(--accent)`，hover `filter: brightness(1.07)`，active `scale(.98)`；secondary 背景 `surface 80%`、1px `--border`、`inset 0 1px 0 var(--btn-hi), 0 1px 2px rgba(16,24,40,.06)`；ghost 透明、文字 muted；danger 背景 `danger 12%`、文字 `--danger-text`；disabled `opacity .45`；icon 变体 34×34（表格内 28×28、圆角 8）。loading 态 MUST 显示 `.spinner` 并禁用点击。

#### Scenario: 页面头部主操作
- **WHEN** 任意 `PageHeader` 的 `#actions` 插槽内渲染 `<Button variant="primary">`
- **THEN** 计算高度 MUST 为 34px、圆角 10px、背景等于 `--accent`，且左侧图标（如有）为 16px

#### Scenario: 表格行操作
- **WHEN** DataTable 操作列渲染 `.icon-btn`
- **THEN** 尺寸 MUST 为 28×28、圆角 8、透明背景、图标 15px、hover 背景 `foreground 6%`

#### Scenario: 支付品牌按钮
- **WHEN** 渲染 `.btn-stripe / .btn-alipay / .btn-wxpay / .btn-airwallex`
- **THEN** 高度 MUST 为 32、圆角 9、品牌色 `#635bff / #00AEEF / #2BB741 / #14171A`，且 MUST NOT 随主题变化

### Requirement: 字段、文本域、搜索框
系统 SHALL 提供 `.field`（`TextInput` / `Input` / `TextArea` / `SearchInput` 共用）：高 36px、圆角 12、内边距 `0 12px`、字号 13、背景 `surface 85%`、1px `--border`、`box-shadow: var(--field-shadow)`；占位符 `--muted`；focus 边框 `--accent` + 3px 主色 18% 外环；error 边框 `--danger` + 3px danger 14% 外环；disabled `opacity .6`。`.input-lg` 高 40（登录 / 金额输入），文本域最小高 96、内边距 `10px 12px`。搜索框左侧 15px 图标、`padding-left 36px`、筛选行中宽 260。标签 `FieldLabel` 12.5/600、间距 6；提示 12 muted；错误文案 12 `--danger-text`；必填星号 `--danger-text`。前缀 / 后缀图标 muted 16px。

#### Scenario: 密码字段显隐
- **WHEN** 登录页密码字段渲染
- **THEN** 高度 MUST 为 40px（`.input-lg`），右侧 16px 眼睛图标 muted，点击切换明文且字段高度不变

#### Scenario: 校验失败
- **WHEN** 表单字段校验失败
- **THEN** 字段 MUST 呈 error 态，下方 12px `--danger-text` 文案，字段高度不变、不挤压相邻控件

### Requirement: 选择器与筛选药丸
系统 SHALL 提供 `Select` / `UiSelect`：触发器为 `.field` 配方（36px），`variant="pill"` 时渲染为 `.filter-pill`（`label muted · value 600 · 14px chevron`，激活态主色边框 + 主色 10% 背景 + 可选 × 清除）；下拉面板 `.dropdown`（圆角 12、内边距 6、`surface 92% + blur(20px)`、`--shadow-pop`、最大高 320 滚动）；选项高 36、圆角 9、悬停 `foreground 5%`、选中 `accent 10%` 背景 + 主色文字 + 右侧 14px 对勾；搜索行 32px `.field`；分组标题 `.dropdown-label` 11/600 大写 muted；分隔线 `.dropdown-divider`。键盘 ↑↓ Enter Esc MUST 可用。

#### Scenario: 筛选行平台选择
- **WHEN** 账号列表筛选行渲染平台选择器
- **THEN** 触发器 MUST 显示"平台 全部"（标签 muted、值 600）、高 36、圆角 12；选择 Claude 后值变为"Claude"且边框变为 `--accent`

### Requirement: 开关、复选框、分段控件
`ToggleSwitch` 表单尺寸 36×20、紧凑（表格内）32×18；开：背景 `--accent`、`inset 0 1px 2px rgba(0,0,0,.18), 0 0 0 3px accent 16%`；关：`--surface-tertiary` + 1px `--border` + `--field-shadow`；拇指 16px（紧凑 14px）`var(--thumb)` 渐变 + `0 1px 3px rgba(0,0,0,.28)`；过渡 150ms。`Checkbox` 16×16、圆角 5、1.5px 边框，选中 `--accent` 白色 11px 对勾，indeterminate 显示横线。`SegmentedControl` 高 36（`segmented-sm` 30）、外框圆角 11、内边距 3、背景 `surface-secondary 80%` + 1px `--border`、项 `0 12px` 圆角 8 字号 12.5/600，选中项 `--surface` + `0 1px 2px rgba(0,0,0,.08)`，未选中 muted；键盘 ←→ MUST 可切换。

#### Scenario: 表格可调度开关
- **WHEN** 账号列表"可调度"列渲染
- **THEN** 开关 MUST 为 32×18 紧凑尺寸，行高不因开关增加

#### Scenario: 状态分段筛选
- **WHEN** API 密钥筛选行渲染 全部 / 启用 / 已过期 / 已禁用
- **THEN** MUST 使用 `SegmentedControl`，高 36，选中项白底浮起，其余 muted

### Requirement: 徽章、标签、计数与提示
`StatusBadge`（`.badge`）高 22、圆角 999、内边距 `0 8px`、11.5/600、`inset 0 0 0 1px color-mix(in oklch, currentColor 22%, transparent)`，tone：success（`success 16%` / `--success-text`）、warning（`warning 18%` / `--warning-text`）、danger（`danger 14%` / `--danger-text`）、muted（`--surface-secondary` / `--muted`）、accent（`accent 12%` / `--accent`）；可选 6px currentColor 圆点，前置 6px 间距；实时态圆点 MUST 用 `s2a-pulse` 动画。`.tag` 中性类型标签 11.5/600、内边距 `3px 7px`、圆角 6、`--surface-secondary` / muted；`.tag-accent/-success/-warning/-danger` 同色系。`GroupBadge` 保留分组色相：背景 `color-mix(in oklch, <hue> 16%, transparent)`、文字 `color-mix(in oklch, <hue> 70%, var(--foreground))`、6px 色相圆点。`.count-badge` 18px 圆形 `--danger` 白字 10.5/700（侧栏 / 按钮角标）。`HelpTooltip` 触发器 15px 圆 `?`（1.5px muted 边框、10/700），气泡 `.tooltip-bubble`：`--foreground` 底 / `--background` 字、11.5/500、内边距 `6px 10px`、圆角 8、6px 箭头、最大宽 260。

#### Scenario: 账号状态列
- **WHEN** 账号状态为 正常 / 限流 / 异常 / 已暂停
- **THEN** MUST 分别渲染 success / warning / danger / muted 徽章，高 22，带 6px 圆点，同列所有徽章左对齐且高度一致

#### Scenario: 提示气泡在暗色下
- **WHEN** 暗色主题悬停 `HelpTooltip`
- **THEN** 气泡 MUST 为浅底（`--foreground` 在暗色为浅色）深字，对比度 ≥ 4.5:1

### Requirement: 卡片家族
系统 SHALL 提供以下卡片家族且所有页面 MUST 只通过它们承载分区内容。`GlassCard`（`.glass-card`）：`surface 72%` + 1px `border 85%` + `blur(20px)` + `--shadow`，圆角 14，`variant: glass | solid | transparent | flat`，`padding: sm 12 | md 16–18 | lg 20–24`，`hover` 启用悬停阴影；`.glass-ring` 追加主色渐变 1px 发丝环（选中态）；`.glass-inset` 卡内嵌板 `surface 70%` 圆角 12 内边距 `14px 16px`。`.card-header` 内边距 `16px 20px 12px` + 底边线，`.card-title` 15/600，`.card-subtitle` 12.5 muted，`.card-body` 20，`.card-footer` `12px 20px` 顶边线。`StatCard`：内边距 `14px 16px`、标签 12/600 muted、值 28/800 tabular、副文 12 muted、增量药丸 11/600 `2px 6px` 圆角 6 tone 底、右下 96×28 sparkline（面积 accent 14%、线 accent 1.8）。`MiniStatCard` 3 列：值 22/800、标签 12/600 muted。`.summary-chip`：高 64 圆角 12 玻璃底，标签 12/600 muted + 6px tone 圆点，值 20/800，`.is-active` 主色边框。`EndpointCard`：标签 12/600 muted + 右侧 `.tag`，URL mono 13.5/500 + 28px 复制按钮（`surface-secondary` 底），说明 12 muted。

#### Scenario: 仪表盘统计网格
- **WHEN** `/admin/dashboard` 渲染 8 张 `StatCard`
- **THEN** 4 列网格间距 12，每张卡高度一致（同一行内），值 28/800 tabular，sparkline 存在且不溢出卡片

#### Scenario: 嵌套卡片禁止
- **WHEN** 任意玻璃卡内部需要分区
- **THEN** MUST 使用 `.glass-inset` 或分隔线，MUST NOT 再嵌套一层 `.glass-card`（阴影叠加）

### Requirement: 数据表格与分页
`DataTable` SHALL 保留 sticky 首列 / 操作列、行选择、虚拟滚动、列设置持久化、排序持久化、<768px 卡片模式；视觉 MUST 为：表头高 42、内边距 `0 16px`、11/600 大写字距 .06em muted、背景 `surface-secondary 45%`；排序激活 10px 主色三角、未激活 muted 双箭头；行高 58–60（上下内边距 12）、单元格 13px `0 16px`、底边 1px `--border`；行 hover `accent 5%` 背景 + `inset 3px 0 0 var(--accent)` 左轨；sticky 列背景 `var(--surface)` 并跟随 hover；复选列使用 `Checkbox` 外观；加载态 `.skeleton` 行；空态在表头下方卡内居中（`EmptyState`）；横向滚动两端渐变阴影；滚动条仅 hover 显示。卡片模式：玻璃卡内边距 14，标签 11 muted / 值 13/600 网格，主操作在卡底。`Pagination`：容器 `10px 16px` 12.5 muted 顶边线；文案"显示 a–b，共 n 条 · 每页 <b>20</b> 条"（数字 `--foreground`）；按钮 28×28 圆角 8 1px `--border` 透明底，当前页 `--accent` 白字 600；每页数选择为 `.filter-pill`；移动端前后翻页为 `.btn-secondary .btn-sm`。

#### Scenario: 账号列表桌面
- **WHEN** 1440px 渲染 `/admin/accounts`
- **THEN** 表头高 MUST 42、行高 60、hover 行出现 3px 主色左轨，sticky 操作列背景与行背景一致无色差

#### Scenario: 空列表
- **WHEN** `/admin/orders` 无数据
- **THEN** 表头 MUST 仍可见，其下 MUST 渲染虚线空态框（24px muted 图标、12.5/600 标题、可选主色 11.5/600 操作链接），分页栏 MUST 显示"共 0 条"

#### Scenario: 移动端卡片
- **WHEN** 390px 渲染 `/keys`
- **THEN** 每条记录 MUST 是玻璃卡：名称 15/600 + `#id · 分组` 12 muted、右上状态徽章、40px 密钥行（`foreground 4%` 底、32px 复制按钮）、4 列迷你统计（11 标签 / 13/600 值）

### Requirement: 筛选行、页头与设置行
`FilterBar` SHALL 提供 `#search #filters #trailing` 插槽，`display:flex; gap:8px; align-items:center`，右侧 `margin-left:auto` 的 12.5 muted 元信息（如"已选 <b>0</b> / 86"）与 36×36 列设置图标按钮；<768px 折叠为 44px 搜索 + 44px 筛选按钮，筛选进入 `UiDrawer`。`PageHeader`：标题 24/800、描述 13 muted、`#actions` 右对齐 gap 8；`variant="hero"` 仅仪表盘（圆角 16、内边距 `26px 30px`、点阵纹理 + 两处径向光）；<768px 标题 20、操作换行。`SettingsSection` = 玻璃卡 + `.card-header`；`SettingRow`：网格 `240px 1fr`、`gap 12px 24px`、内边距 `14px 20px`、行间 1px `--border`（末行无）、标签 13/600 + 提示 12 muted、控件列文本字段最大宽 420、数字字段宽 120、危险提示 12 `--warning-text` + 三角图标；<768px 单列。

#### Scenario: 设置页通用 tab
- **WHEN** 渲染 `/admin/settings` 通用设置
- **THEN** 每行 MUST 是 `SettingRow`：左 240px 标签列，右控件列，字段 36px，行间 1px 分隔，卡片圆角 14 溢出隐藏

### Requirement: 弹层、抽屉与确认框
`UiModal`：遮罩 `foreground 40%` + `blur(6px)`；面板 `--surface` 圆角 16 `--shadow-pop`，宽度只允许 440 / 560 / 720 / 960；头部 `16px 20px 12px`、标题 16/800 display、副标题 12.5 muted、32px 关闭按钮；主体 `20px` 可滚动（最大高 `80vh`）；底部 `12px 20px` 顶边线，按钮右对齐 gap 8（ghost 取消 + primary 确认）；进入 / 离开 160ms 缩放 0.98→1 + 淡入；Esc 关闭、焦点陷阱、关闭后焦点回到触发器；`BaseDialog` MUST 是同一外观的兼容包装。`UiDrawer`：右侧 480 / 640 宽，同头尾结构，`<768px` 全宽。`ConfirmDialog`：440 宽，44px tone 圆图标（danger / warning / accent），标题 16/800，描述 13 muted，确认按钮按 tone（危险操作 `.btn-danger`）。<768px 弹层 MUST 底部对齐、圆角只在顶部 16、最大高 `92vh`。

#### Scenario: 删除账号确认
- **WHEN** 点击账号行"删除"
- **THEN** MUST 打开 `ConfirmDialog`，红色 44px 图标圆、`.btn-danger` 确认、Esc 可取消，背景为模糊遮罩

#### Scenario: 创建密钥对话框移动端
- **WHEN** 390px 打开创建密钥
- **THEN** 面板 MUST 贴底、顶部圆角 16、内容可滚动、底部按钮固定可见

### Requirement: 反馈类组件
系统 SHALL 提供以下反馈类组件，页面 MUST NOT 自绘等价物。`Toast`：卡片 `12px 14px` 圆角 12 `--surface` + `--border` + `--shadow-hover`，22px tone 圆图标（16% 底 + tone 文字色），标题 13/600、正文 12 muted、28px 关闭热区（14px 图标）、底部 2px tone 进度条，最多堆叠 4 条，位置与动画保持现状。`.notice-info/-success/-warning/-danger`：圆角 12、tone 10–14% 底、1px tone 22% 边框、13px 文字、16px 图标。`EmptyState`：默认虚线框（1px dashed `--border`、圆角 12、内边距 24、24px muted 图标 stroke 1.6、12.5/600 标题、11.5/600 主色操作链接或 `.btn-primary .btn-sm`）；`size="lg"` 整页空态（40px 图标、14/600 标题、12.5 muted 描述）。`Skeleton` / `.skeleton`：`surface-secondary → surface-tertiary → surface-secondary` 90° 渐变、1.4s 扫光、圆角 5。`LoadingSpinner` / `.spinner`：2px currentColor 环、16 / 20 / 24 三档。`NavigationProgress`：3px、`accent → accent 60% white` 渐变、`0 0 8px accent` 微光。`ProgressBar`：6px（`.progress-thin` 5px）圆角 999 `--surface-tertiary` 轨道，阈值 ≥70 warning、≥90 danger，否则 accent；可选右侧 30px 宽百分比 11 muted。

#### Scenario: 保存成功
- **WHEN** 设置保存成功
- **THEN** MUST 弹出 success Toast：绿色 22px 圆对勾、13/600 标题，底部进度条为 `--success`，无左侧色条

#### Scenario: 用量窗口 92%
- **WHEN** 账号 7d 用量为 92%
- **THEN** `.progress-thin` 填充 MUST 为 `--danger`，右侧 `92%` 11px muted 右对齐

### Requirement: 品牌底板与图标
`PlatformIcon` / `ModelIcon` SHALL 继续使用仓库内官方 SVG；表格与首页中的品牌 MUST 承载于 `platformTileBackground(platform)` 的 20–24px 圆角 6–7 底板（白色图标、`inset 0 0 0 1px rgba(255,255,255,.14)` 内描边）；行内图标统一 15–16px、stroke 1.8；侧栏图标 17px。第三方登录标记 18px。

#### Scenario: 平台列
- **WHEN** 账号列表平台列渲染 Claude
- **THEN** MUST 显示 22px 橙色底板 + 白色 Claude 标 + 13/500 平台名，底板与文字间距 8

### Requirement: 页面级样式只允许布局声明
`src/views/**` 与页面专属组件的 `<style scoped>` MUST 只包含布局属性（display / grid / flex / gap / width / height / margin / padding / position / overflow / order / `@media`）；颜色、圆角、阴影、字号、字重、边框样式 MUST 来自全局类或 ui 组件。若组件缺少所需变体，MUST 扩展组件而不是在页面内覆盖。

#### Scenario: 页面样式扫描
- **WHEN** 运行 `scripts/ui-lint.mjs --scoped`
- **THEN** `views/**` 的 scoped 样式中 `color: | background: | border-radius: | box-shadow: | font-size: | font-weight:` 出现次数 MUST 为 0（白名单：`views/HomeView.vue` 的装饰性渐变、`auth/**` 品牌面板装饰）
