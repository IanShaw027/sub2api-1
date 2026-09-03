## Purpose

定义 Sub2API 前端 64 条路由所归属的 12 种页面模板的结构契约，规定原型直接出稿的屏幕必须像素级还原、其余页面必须按同模板配方补全，以及桌面 / 平板 / 移动端三档布局行为。

## ADDED Requirements

### Requirement: 路由与模板的映射唯一
每条路由 MUST 归入且仅归入下列模板之一，并使用对应布局骨架：LandingLayout（`/home /model-plaza /legal/:documentId /key-usage`）、AuthLayout（`/login /register /email-verify /forgot-password /reset-password /auth/dingtalk/email-completion`）、CallbackStatus（`/auth/callback /auth/linuxdo/callback /auth/wechat/callback /auth/wechat/payment/callback /auth/dingtalk/callback /auth/oidc/callback /payment/result`）、SetupWizard（`/setup`）、DashboardPage（`/dashboard /admin/dashboard /admin/ops /admin/orders/dashboard /monitor`）、ListPage（`/keys /usage /orders /invoices /tickets /subscriptions /available-channels /admin/users /admin/groups /admin/accounts /admin/subscriptions /admin/channels/pricing /admin/channels/monitor /admin/plugins /admin/announcements /admin/proxies /admin/redeem /admin/promo-codes /admin/usage /admin/audit-logs /admin/risk-control /admin/prompt-audit /admin/affiliates/invites /admin/affiliates/rebates /admin/affiliates/transfers /admin/orders /admin/orders/invoices /admin/orders/plans /admin/tickets`）、DetailPage（`/invoices/:id /tickets/:id /tickets/new /admin/tickets/:id`）、SettingsPage（`/admin/settings /profile`）、ActionPage（`/redeem /affiliate /batch-image`）、PaymentFlow（`/purchase /payment/qrcode /payment/stripe /payment/airwallex /payment/stripe-popup`）、EmbedPage（`/custom/:id`、`/purchase` iframe 模式、`/studio`）、NotFound（`/:pathMatch(.*)*`）。`route-matrix.md` MUST 与路由表一致。

#### Scenario: 新增路由
- **WHEN** 路由表新增一条路由
- **THEN** `route-matrix.md` MUST 同步指明其模板，且页面 MUST 使用该模板的布局骨架

### Requirement: 原型出稿屏幕像素级还原
`/home`（原型 01）、`/login`（02）、`/admin/dashboard`（03）、`/admin/accounts`（04）、`/keys`（05）、`/admin/settings`（06）及 390px 画板（08：首页 / 仪表盘抽屉 / 密钥卡片模式）MUST 在 1440×（页面高）与 390×844 视口下与原型逐区块一致：区块顺序、栅格列比（如仪表盘 Hero `1.25fr 1fr`、趋势 `2fr 1fr`、健康 / 事件 `1fr 1fr`；设置 `224px 1fr`；密钥顶部 `1.2fr 1.2fr 1fr`）、间距、控件尺寸、类型阶、徽章形状、表头 / 行高、空态样式的偏差 MUST ≤ 1px；文案 MUST 来自 i18n 且不得出现原始键名。

#### Scenario: 管理员仪表盘 Hero
- **WHEN** 1440px 渲染 `/admin/dashboard`
- **THEN** Hero 卡 MUST 圆角 16、内边距 `26px 30px`、左列问候 12.5 muted + 30/800 标题（数字主色）+ 13.5 muted 段落（最大宽 540）+ 34px primary / secondary（带红色角标）按钮，右列 3 张 `.glass-inset`（服务状态脉冲点 + 17/700、实时 RPM 24/800 + 绿色增量、实时 TPM 24/800 + 平均耗时），右半部点阵纹理与两处径向光，无内容溢出

#### Scenario: 账号管理表格列
- **WHEN** 1440px 渲染 `/admin/accounts`
- **THEN** 列 MUST 依次为 复选 · 名称/ID · 平台 · 类型 · 容量 · 状态 · 可调度 · 今日统计 · 用量窗口 5h/7d · 优先级 · 最近使用 · 操作，每列单元格样式与原型 04 一致（名称 600 + `#id · 分组` 11.5 mono muted；平台 22px 底板；类型 `.tag`；容量 mono 12.5；状态圆点徽章；可调度 32×18 开关；今日统计 600 + 11.5 muted；两条 5px 用量条 + 30px 百分比；两个 28px 图标按钮）

#### Scenario: API 密钥顶部
- **WHEN** 1440px 渲染 `/keys`
- **THEN** 顶部 MUST 为 `1.2fr 1.2fr 1fr` 三卡：两张 `EndpointCard`（Anthropic / OpenAI 协议端点，mono URL + 复制）与一张三列迷你统计（密钥总数 / 启用中 success-text / 今日消费）；筛选行 MUST 为 260px 搜索 + 分组药丸 + 状态分段控件 + 右侧"按创建时间排序"12.5 muted

#### Scenario: 移动端密钥
- **WHEN** 390×844 渲染 `/keys`
- **THEN** MUST 为 3 张迷你统计卡横排 → 44px 搜索 + 44px 筛选按钮 → `.chip-filter` 行（全部 5 / 活跃 3 / 已过期 1 / 已禁用 1，激活为 `--foreground` 底）→ 密钥卡片列表 → 右下 52px 主色 FAB"创建密钥"，底部 `ListFade` 渐隐

### Requirement: DashboardPage 结构
DashboardPage MUST 依次为：`PageHeader`（仅 `/admin/dashboard` 用 hero 变体）→ `StatCard` 4 列网格（gap 12，值 28/800，增量药丸，sparkline）→ `2fr 1fr` 图表卡（标题 14/600 + 副题 12 muted + 右侧 `segmented-sm`；CSS 柱状图：14 根柱、高 150、圆角 `6 6 3 3`、末柱实心主色渐变、其余主色 50→16% 渐变、刻度 10.5 muted tabular；或 chart.js 以令牌着色：主色填充、`--border` 网格、10.5 muted 刻度、`.tooltip-bubble` 提示）+ 分布列表卡（行：mono 名称 12 + 右侧 `cost · pct%` muted + 6px 主色渐变进度，透明度递减）→ `1fr 1fr` 列表卡（平台健康：`120px 1fr 150px` 行，22px 底板 + 名称 600 + 8px 三段堆叠条 success/warning/danger 2px 间隔 + 右侧 muted tabular；最近事件：`44px 8px 1fr` 行，mono 11.5 muted 时间 + 8px tone 圆点 + 12.5 文案）。无数据的区块 MUST 以同样式空态占位，不得空白或消失。<768px：单列；Hero 用 390 变体（渐变环、40px 值、48px 柱、3 个日期标签）；统计卡 2×2 值 24。

#### Scenario: 用户仪表盘
- **WHEN** 1440px 渲染 `/dashboard`
- **THEN** MUST 无 Hero，直接 `PageHeader` + 8 张统计卡（4 列）+ `2fr 1fr` Token 趋势 / 模型分布 + `1fr 1fr` 平台拆分 / 最近使用；chart.js 图表网格线不得深于 `--border`，图例 12 muted，刻度 10.5

#### Scenario: 运维监控无告警
- **WHEN** `/admin/ops` 告警列表为空
- **THEN** 告警卡 MUST 保留标题并显示虚线空态，其它区块正常

### Requirement: ListPage 结构
ListPage MUST 依次为：`PageHeader` + 右侧操作（34px secondary：图标刷新 34×34、工具、`更多` 下拉；一个 34px primary 带加号）→ 可选 `.summary-chip` 行（N 列 gap 10，20/800 值，tone 圆点，`.is-active` 与状态筛选联动）→ 筛选行（gap 8：260px 搜索、`.filter-pill` 选择、`SegmentedControl` 状态、`DateRangePicker` 36px、右侧 12.5 muted"已选 n / total"与 36×36 列设置按钮）→ 玻璃卡（圆角 14、溢出隐藏、`flex:1`）内 `DataTable` → `.table-footer` 分页。单元格 MUST 使用统一单元格组件：名称 600 + `#id · meta` 11.5 mono muted、22px 品牌底板、`.tag` 类型、圆点徽章状态、紧凑开关布尔值、`.progress-thin` 配额、tabular 数字、mono 时间戳 12.5 muted、28px 图标按钮操作（编辑铅笔 + `…` 下拉）。创建 / 编辑 / 批量 / 导入 / 导出 MUST 走 `UiModal` / `UiDrawer`（`SettingRow` 或双列表单网格、36px 字段、ghost 取消 + primary 确认）。<768px：DataTable 卡片模式，筛选进入 44px 按钮 + 抽屉，主操作变为 52px FAB。

#### Scenario: 用户管理
- **WHEN** 1440px 渲染 `/admin/users`
- **THEN** 顶部 MUST 是 3 个 `.summary-chip`（总计 / 启用 / 管理员）而不是三段式大数字卡；筛选行搜索 260px + 状态 `SegmentedControl`；表格列 用户（头像 28 + 邮箱 600 + `#id · 用户名` 11.5 mono muted）· 角色 `.tag` · 余额 tabular + 充值链接 · 状态圆点徽章 · 最后活跃 mono 12.5 muted · 创建时间 · 操作 28px 图标按钮；无横向溢出到操作列的重叠

#### Scenario: 空的优惠码列表
- **WHEN** `/admin/promo-codes` 无数据
- **THEN** 页头、筛选行、表头与空态 MUST 全部呈现，空态在卡内居中，页面不得只剩一行"暂无数据"

### Requirement: DetailPage 结构
DetailPage MUST 为：`PageHeader`（标题行左侧 34px secondary 返回按钮）→ `1fr 320px` 网格 gap 14（<1024px 单列）：主玻璃卡消息时间线（行间距 16；32px 圆头像 `accent 18%` 底 + 13/700 首字；气泡圆角 12、内边距 `12px 14px`、13/1.6，对方 `--surface-secondary`、自己 `accent 12%`；时间 12.5 mono muted；附件为 `.chip`）；侧卡 `SettingRow` 式元信息（标签 12.5 muted / 值 13/600，1px 分隔）与状态 `UiSelect`；底部输入区 `.field` 文本域（最小高 96）+ 回复模板 `.filter-pill` + 34px primary。

#### Scenario: 管理员工单详情
- **WHEN** 1440px 渲染 `/admin/tickets/:id`
- **THEN** 布局 MUST 为主时间线 + 320px 侧卡，管理员消息为主色 12% 气泡靠右，用户消息为 surface-secondary 靠左，头像 32px，返回按钮 34px

### Requirement: SettingsPage 结构
SettingsPage MUST 为：`PageHeader` + 右侧"● n 项未保存的更改"12.5 `--warning-text` + 34px secondary 重置 + primary 保存 → `224px 1fr` 网格 gap 14：左列玻璃卡（内边距 8）分区导航（项高 34、`0 10px`、圆角 9、13px；激活 600 + 主色 + `surface 92%` 底 + `inset 0 1px 0 var(--btn-hi), 0 1px 3px rgba(16,24,40,.10), 0 0 0 1px border 80%`；悬停 `foreground 5%`；未保存 tab 右侧 6px warning 圆点）+ 部署信息卡（12 muted 键值行：版本 mono、PostgreSQL / Redis 已连接 success-text、Codex 版本同步 mono）；右列玻璃卡序列（`.card-header` + `SettingRow`）；重复列表编辑器（自定义端点、菜单项、OAuth 提供方、邮件模板）为头部 32px secondary "添加" + 行网格 `180px 1fr 1fr 32px` gap 12 内边距 `12px 20px` + 32px danger 图标删除；Logo 上传为 44px `.brand-mark-xl` 预览 + 32px secondary 上传 + ghost 移除；危险区独立卡片、标题 `--danger-text`、`.btn-danger`。<768px：分区导航变为 `.field` 外观的 select 或横向滚动药丸。

#### Scenario: 个人设置
- **WHEN** 1440px 渲染 `/profile`
- **THEN** MUST 采用与 `/admin/settings` 相同的 `224px 1fr` 布局：左侧 资料 / 安全 / 通知 / 绑定 / 危险区 导航，右侧 `SettingRow` 卡片；头像上传、密码修改、Passkey、2FA、余额提醒各为一张卡；不得再使用大幅 Hero 头像横幅与嵌套卡片

### Requirement: ActionPage、PaymentFlow、EmbedPage 结构
ActionPage MUST 为最大宽 720 居中列（gap 14）：玻璃卡（标题块 + 36px 字段 + 34px primary）→ 结果 `.notice-success/-danger` → 说明卡（`.card-title` + 12.5 muted 列表）→ `MiniStatCard` / 3 列统计；邀请页邀请链接为 `EndpointCard` 式复制行，返利记录为 ListPage 表格。PaymentFlow：套餐卡玻璃卡圆角 16 内边距 20，选中 `.glass-ring`，价格 28/800 tabular，特性行 13 + 12px success 对勾，42px 全宽 primary；金额输入 40px `.field.input-lg` 带 `$` 前缀；支付品牌按钮 32px；二维码页 440 居中卡、220px 二维码在 `--surface-secondary` 圆角 12 盒内、轮询状态圆点徽章；结果页与 CallbackStatus 同款。EmbedPage：玻璃卡圆角 16 包裹 iframe / Markdown，`.card-header` 标题 + 34px secondary"新窗口打开"；创作中心保留三栏，会话项为侧栏项配方（36px）、输入区 `.field` 文本域 + 34px primary、消息气泡同 DetailPage。

#### Scenario: 兑换页
- **WHEN** 1440px 渲染 `/redeem`
- **THEN** 内容 MUST 居中 720px：兑换码字段 36px + 34px primary，兑换成功后出现 `.notice-success`，下方说明卡与最近兑换记录表

#### Scenario: 套餐选择
- **WHEN** 渲染 `/purchase` 并选中一个套餐
- **THEN** 被选套餐卡 MUST 显示 `.glass-ring` 主色发丝环，价格 28/800，其它卡无环；CTA 42px 全宽

### Requirement: 公开页与认证页结构
LandingLayout MUST 为：顶部导航（品牌标 + 链接 + 语言 / 主题 / 登录 / 立即开始）+ 玻璃卡分段（首页按原型 01：Hero 双栏 + 控制台预览、三步接入 + 代码标签页、模型定价表、对比表、CTA、页脚；模型广场 / 用量查询 / 法律文档用同导航 + 30/800 页标题 + 玻璃卡内容）。AuthLayout MUST 为 `1.1fr 1fr` 网格：左品牌面板（`margin 20px 0 20px 20px`、圆角 20、内边距 `48px 56px`、玻璃 55% + 点阵纹理 + 右下 520px 径向光；`.brand-mark-lg` + 站名 16/700；46/800 标题 + 15 muted 段落 + 3 条特性行；底部 `服务正常` 徽章 + 已支持平台 12.5 muted）+ 右侧 440px 卡（内边距 36、圆角 20、玻璃 70% + `.glass-ring`；标题 26/800 + 13.5 muted；字段 40px；主按钮 42px；分隔"或使用其他继续"；OAuth 40px secondary，首个全宽其余两列；页脚 12.5 muted 与 11.5 条款链接）；≤900px 单列、隐藏品牌面板、卡片全宽内边距 24、按钮 44。CallbackStatus MUST 为 `.public-page` 居中 440 卡：44px tone 圆（spinner / 对勾 / ×）、20/800 标题、13.5 muted 描述、42px 按钮、`.code-block` 展示代码 / state。SetupWizard：640px 卡 + 4 步步进器（28px 圆、激活主色、完成 success 对勾、2px 连接线）+ `SettingRow` + 34px secondary 连接测试（内联状态徽章）+ 42px 完成按钮。NotFound：`.public-page` 玻璃卡、64/800 主色渐变 404、返回按钮。

#### Scenario: 注册页与登录页共用品牌面板
- **WHEN** 分别渲染 `/login` 与 `/register`
- **THEN** 左侧品牌面板 MUST 完全相同（同组件），右侧卡片宽 440、字段 40px、主按钮 42px；注册页邀请码 / 优惠码校验态使用 success-text / danger-text 12px 内联提示

#### Scenario: OAuth 回调失败
- **WHEN** `/auth/callback?error=…`
- **THEN** MUST 渲染 CallbackStatus 卡：44px danger 圆 ×、20/800 标题、13.5 muted 错误描述、42px primary 返回登录、错误码在 `.code-block`

### Requirement: 响应式三档
所有页面 MUST 满足：≥1024px 桌面布局；768–1023px 侧栏收为 60px 图标轨、统计网格 2 列、表格保持；≤767px 单列、DataTable 卡片模式、页面内边距 16、页面标题 20、触控目标 ≥44px、弹层贴底、筛选进入抽屉、主操作 FAB；390×844 MUST 无横向滚动、无文字截断遮挡。

#### Scenario: 平板账号列表
- **WHEN** 900px 渲染 `/admin/accounts`
- **THEN** 侧栏 MUST 为 60px 图标轨，表格横向滚动且 sticky 首列 / 操作列可见，汇总条 3 列换行

#### Scenario: 手机设置页
- **WHEN** 390px 渲染 `/admin/settings`
- **THEN** 分区导航 MUST 变为 36px `.field` 外观 select，`SettingRow` 单列（标签在上、控件在下），保存按钮吸底可见

### Requirement: 所有状态完整
每个页面与列表 MUST 具备并统一实现：加载（骨架，与最终布局同形）、空（`EmptyState`）、错误（`.notice-danger` + 重试按钮）、成功反馈（Toast）、禁用 / 只读、悬停 / 激活 / 焦点；状态切换 MUST 不引起布局跳动（骨架占位与内容等高）。

#### Scenario: 列表加载
- **WHEN** `/admin/users` 请求进行中
- **THEN** 表格 MUST 显示 5 行 `.skeleton` 骨架行（高 58），表头保持，分页栏保持，不得整页替换为居中 spinner

#### Scenario: 请求失败
- **WHEN** 列表接口返回 5xx
- **THEN** 卡内 MUST 显示 `.notice-danger`（16px 图标 + 13px 文案 + 32px secondary 重试），页头与筛选行仍可用
