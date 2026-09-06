## Purpose

定义 Sub2API 前端 Glass 主题的令牌契约：颜色、字体、圆角、阴影、间距节奏、明暗主题与主色切换方式，以及视图层引用颜色与尺寸的唯一合法途径，使全站任何界面都从同一组令牌派生。

## ADDED Requirements

### Requirement: 主题令牌与原型同名同值
系统 SHALL 在 `html[data-theme]` 上提供与原型完全同名的 CSS 自定义属性：`--canvas --background --foreground --surface --surface-secondary --surface-tertiary --muted --border --accent --success --success-text --warning --warning-text --danger --danger-text --code-bg --shadow --shadow-hover --btn-hi --field-shadow --thumb --display --font-body --font-mono`，并 MUST 使用原型给出的 oklch 色值（亮：`--background oklch(97.02% 0.0015 262.89)`、`--foreground oklch(21% 0.012 262)`、`--accent oklch(62.31% 0.1881 259.82)`、`--success oklch(73.29% 0.1946 151.55)`、`--warning oklch(78.19% 0.1593 73.04)`、`--danger oklch(65.32% 0.2342 26.45)`、`--border oklch(90% 0.003 259.82)`、`--muted oklch(55.17% 0.006 259.82)`；暗：`--background oklch(12% 0.0015 262.89)`、`--surface oklch(21.03% 0.003 262.89)`、`--border oklch(28% 0.003 259.82)`、`--muted oklch(70.5% 0.006 259.82)`、`--foreground oklch(95% 0.004 262)`）。

#### Scenario: 亮色主题下读取令牌
- **WHEN** `html[data-theme="glass-light"]`（或未设置 data-theme）时读取 `getComputedStyle(document.documentElement)` 的上述属性
- **THEN** 每个属性 MUST 解析为原型亮色表中对应的值，误差不超过 oklch 各分量 0.5%

#### Scenario: 暗色主题下读取令牌
- **WHEN** `html[data-theme="glass-dark"]` 时读取上述属性
- **THEN** 每个属性 MUST 解析为原型暗色表中对应的值，且 `--success-text` / `--warning-text` MUST 等于 `--success` / `--warning`，`--danger-text` MUST 为 `oklch(72% 0.2 26.45)`

#### Scenario: 切换主题不刷新页面
- **WHEN** 用户在顶栏或侧栏切换明暗
- **THEN** 只有 `html[data-theme]` 属性改变，所有可见界面 MUST 在同一帧内随令牌变化，不得出现未跟随变化的固定色块

### Requirement: 主色可通过 data-accent 切换且默认为 Blue
系统 SHALL 支持 `html[data-accent]` 取值 `blue | sky | indigo | teal | violet`，默认 `blue`；非 blue 值只覆盖 `--accent`。本变更 MUST NOT 提供切换 UI，但令牌层 MUST 已就绪。

#### Scenario: 设置 data-accent=teal
- **WHEN** 开发者在 html 上设置 `data-accent="teal"`
- **THEN** `--accent` MUST 变为 `oklch(60.02% 0.1039 184.73)`，其余令牌不变，所有使用主色的按钮、链接、焦点环、激活态随之变化

### Requirement: 字体栈与自托管
系统 SHALL 定义 `--display: 'Manrope','Inter','PingFang SC','Hiragino Sans GB','Microsoft YaHei','Noto Sans SC',sans-serif`、`--font-body: 'Inter','PingFang SC','Hiragino Sans GB','Microsoft YaHei','Noto Sans SC',sans-serif`、`--font-mono: 'JetBrains Mono',ui-monospace,SFMono-Regular,Menlo,monospace`。Manrope（600/700/800）、Inter（400/500/600/700）、JetBrains Mono（400/500）MUST 以 woff2 自托管于 `frontend/public/fonts/`，`font-display: swap`；`index.html` 与 `src/**` MUST NOT 引用 `fonts.googleapis.com` 或 `fonts.gstatic.com`。

#### Scenario: 离线部署加载首页
- **WHEN** 在无法访问公网字体服务的环境打开 `/home`
- **THEN** 标题 MUST 以 Manrope 800 渲染，正文以 Inter 渲染，代码以 JetBrains Mono 渲染，控制台 MUST 无字体加载失败

#### Scenario: 静态检查
- **WHEN** 运行 `designTokens.spec.ts`
- **THEN** "不引用 Google Fonts" 断言 MUST 通过

### Requirement: 类型阶固定
系统 SHALL 只使用以下字号 / 字重组合（单位 px）：Hero 标题 30/800/-.03em（登录品牌面板 46/800/-.035em）、页面标题 24/800/-.03em、统计值 28/800/-.04em、汇总值 20/800/-.03em、迷你统计值 22/800、弹层标题 16/800、卡片标题 15/600、图表卡标题 14/600、正文 13/400、按钮 13/600（卡内 12.5/600）、字段 13、表头 11/600 大写 字距 .06em、徽章 11.5/600、说明 12–12.5 muted、图表刻度 10.5。显示级（Hero / 页面标题 / 统计值 / 汇总值 / 弹层标题）MUST 使用 `var(--display)`；所有数字 MUST `font-variant-numeric: tabular-nums`；ID、密钥、模型名、URL、时间戳 MUST 使用 `var(--font-mono)`。

#### Scenario: 页面标题
- **WHEN** 任意工作区页面渲染 `PageHeader`
- **THEN** 标题计算样式 MUST 为 24px、800、letter-spacing -0.72px（-.03em）、font-family 以 Manrope 开头

#### Scenario: 表格数字列
- **WHEN** 任意 DataTable 单元格显示金额、次数、百分比
- **THEN** 该元素 MUST 具有 `font-variant-numeric: tabular-nums`，同列数字右对齐或等宽对齐

### Requirement: 圆角、阴影与间距节奏固定
系统 SHALL 只使用圆角 5（复选框）、6–7（品牌底板 / 类型标签）、8（分页项 / 图标按钮 / 小圆角）、9（卡内按钮 / 侧栏项）、10（按钮）、11（分段控件外框）、12（字段 / 下拉 / 通知）、14（卡片 / 表格容器）、16（Hero 卡 / 套餐卡 / 嵌入容器）、18（原型画板外框，不用于应用）、20（认证卡 / 品牌面板）、999（徽章 / 开关 / 头像）；卡片阴影 MUST 用 `var(--shadow)`，悬停 MUST 用 `var(--shadow-hover)`，弹层 MUST 用 `var(--shadow-pop)`；间距 MUST 取自 4 / 6 / 8 / 10 / 12 / 14 / 16 / 20 / 24 / 32（32 仅公开页分段）；工作区内容区内边距 MUST 为 `8px 24px 24px 20px`，页面块间距 MUST 为 14–16px。

#### Scenario: 静态扫描圆角
- **WHEN** 扫描 `src/**/*.vue` 与 `style.css` 中的 `border-radius` 与 Tailwind `rounded-*`
- **THEN** MUST 不出现 `rounded-2xl`、`rounded-3xl`、`rounded-xl`（卡片上）、`border-radius: 18px` 之外的非列表值

#### Scenario: 卡片悬停
- **WHEN** 鼠标悬停在带 `hover` 的 `GlassCard` / 统计卡 / 汇总卡上
- **THEN** 阴影 MUST 切换为 `var(--shadow-hover)`，位移 MUST ≤ 1px，过渡 150–200ms

### Requirement: 视图层禁止硬编码颜色与 dark 变体
视图层（`src/views/**`、`src/components/**`、`src/features/**`）MUST NOT 出现十六进制 / rgb / hsl 字面量颜色、Tailwind `dark:` 变体、`bg-white`、`border-gray-*`、`text-gray-*`、`bg-gray-*`。例外：支付品牌按钮 `.btn-stripe/.btn-alipay/.btn-wxpay/.btn-airwallex`、第三方登录品牌标记、品牌 SVG 图标文件、`platformTile.ts` 中的品牌底板色。Tailwind 调色板类（`bg-red-50`、`text-emerald-600` 等）MUST 通过 `tailwind.config.js` 映射到令牌，新代码 MUST 优先使用语义类 `text-muted text-accent text-danger-text text-success-text text-warning-text bg-surface bg-surface-2 bg-surface-3 bg-accent border-line`。

#### Scenario: 遗留类扫描
- **WHEN** 运行门禁脚本 `scripts/ui-lint.mjs`
- **THEN** `dark:`、`bg-white`、`border-gray-`、`text-gray-`、`bg-gray-`、`rounded-2xl`、`shadow-lg`、`shadow-md`、`shadow-xl`、`text-lg font-semibold` 的命中数 MUST 为 0；十六进制颜色命中 MUST 仅出现在例外清单文件中

#### Scenario: 调色板类在暗色下
- **WHEN** 旧模板中 `bg-red-50 text-red-700` 在暗色主题渲染
- **THEN** 背景 MUST 解析为 `color-mix(in oklch, var(--danger) …, transparent)` 派生色，文字为 `var(--danger-text)`，不得出现亮色的浅粉底

### Requirement: 公开页与工作区背景
公开页（首页、认证、回调、安装向导、404、用量查询、模型广场、法律文档）MUST 使用 `--bg-public`：48px 网格纹理 + 左上主色 22% / 左下主色 10% / 右上 success 14% 三处径向环境光叠加在 `--background` 上；工作区页面 MUST 使用 `--bg-workspace`：只保留环境光，不带网格。玻璃卡片 MUST 为 `surface 70–72% + border 85% + backdrop-filter blur(20px)`。

#### Scenario: 登录页背景
- **WHEN** 渲染 `/login`
- **THEN** 页面根元素背景 MUST 含 48px 网格与三处径向光；品牌面板与登录卡 MUST 是玻璃卡（可见背景纹理透出）

#### Scenario: 管理员页面背景
- **WHEN** 渲染 `/admin/accounts`
- **THEN** 内容区背景 MUST 无网格纹理，仅环境光；表格容器为玻璃卡

### Requirement: 全局微交互与滚动条
系统 SHALL 提供 `::selection` 主色 22%、按钮 `:active` `scale(.98)`、滚动条仅悬停显示（8px 拇指 `color-mix(muted 45%)`，Firefox `thin`）、焦点环 `0 0 0 3px color-mix(in oklch, var(--accent) 18%, transparent)`。

#### Scenario: 键盘聚焦字段
- **WHEN** 用 Tab 聚焦任意 `.field`
- **THEN** 边框 MUST 变为 `var(--accent)` 且出现 3px 主色 18% 外环，不得使用浏览器默认 outline
