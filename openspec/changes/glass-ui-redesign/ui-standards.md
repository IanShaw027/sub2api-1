# UI 一致性与观感标准（速查表）

本表是 `specs/*` 的可执行版本：代理与评审者在实施和验收时直接对照这里的数字与配方。原型渲染参考图在 `reference/`（由 `scripts/proto-shots.js` 从 `Sub2API Redesign.dc.html` 渲染，亮 / 暗各一套）。

> 优先级：原型出稿 > 本表配方 > 现有实现。只有两种情况允许偏离原型：(a) 原型与项目实际不符（功能不存在 / 数据不存在 / 平台差异），(b) 有明显更好的呈现且经 lead 确认。所有偏离必须登记到 `deviations.md`（路由、原型位置、偏离内容、理由、截图）。

## 1. 令牌（唯一颜色来源）

| 令牌 | 亮 | 暗 |
|---|---|---|
| `--background` | oklch(97.02% .0015 262.89) | oklch(12% .0015 262.89) |
| `--foreground` | oklch(21% .012 262) | oklch(95% .004 262) |
| `--surface` | oklch(100% 0 0) | oklch(21.03% .003 262.89) |
| `--surface-secondary` | oklch(95.24% .0023 262.89) | oklch(25.7% .0023 262.89) |
| `--surface-tertiary` | oklch(92.5% .003 262.89) | oklch(30% .003 262.89) |
| `--muted` | oklch(55.17% .006 259.82) | oklch(70.5% .006 259.82) |
| `--border` | oklch(90% .003 259.82) | oklch(28% .003 259.82) |
| `--accent` | oklch(62.31% .1881 259.82) | 同 |
| `--success` / `-text` | oklch(73.29% .1946 151.55) / oklch(45% .15 151.55) | 同 / = `--success` |
| `--warning` / `-text` | oklch(78.19% .1593 73.04) / oklch(52% .14 73.04) | 同 / = `--warning` |
| `--danger` / `-text` | oklch(65.32% .2342 26.45) / oklch(52% .2 26.45) | 同 / oklch(72% .2 26.45) |
| `--code-bg` | oklch(20% .01 262) | oklch(15% .008 262) |
| `--shadow` | inset 0 1px 0 0 rgba(255,255,255,.75), 0 1px 2px rgba(16,24,40,.04), 0 20px 40px -24px rgba(16,24,40,.16) | inset 0 1px 0 0 rgba(255,255,255,.07), 0 20px 40px -24px rgba(0,0,0,.6) |
| `--shadow-hover` | … 0 28px 48px -24px rgba(16,24,40,.24) | … 0 28px 48px -24px rgba(0,0,0,.8) |
| `--btn-hi` | rgba(255,255,255,.65) | rgba(255,255,255,.06) |
| `--field-shadow` | inset 0 1px 2px rgba(16,24,40,.05) | inset 0 1px 2px rgba(0,0,0,.35) |

派生色一律 `color-mix(in oklch, var(--x) N%, transparent)`。常用档位：主色 5%（行 hover）、10%（药丸激活底 / 下拉选中）、12%（accent 徽章 / 我方气泡）、16%（开关外环）、18%（焦点环 / 头像底）、22%（选区）；success 16% / warning 18% / danger 12–14%（徽章底）；foreground 4%（密钥行底）、5%（悬停）、6%（图标按钮 hover）、9%（点阵纹理）。

语义 Tailwind 类：`text-muted text-accent text-danger-text text-success-text text-warning-text bg-surface bg-surface-2 bg-surface-3 bg-accent border-line`。禁止：`dark:`、`bg-white`、`*-gray-*`、hex / rgb 字面量（白名单见 quality-gates）。

## 2. 字体与类型阶

字体：display `Manrope`（600/700/800）→ Inter → 中文栈；body `Inter` → 中文栈；mono `JetBrains Mono`。自托管 woff2，`font-display: swap`。

| 用途 | 字号/字重/字距 | 字体 |
|---|---|---|
| 登录品牌标题 | 46 / 800 / -.035em / lh 1.1 | display |
| Hero 标题 | 30 / 800 / -.03em | display |
| 页面标题 | 24 / 800 / -.03em | display |
| 统计值 | 28 / 800 / -.04em / lh 1.05 / tabular | display |
| 实时 RPM/TPM | 24 / 800 / tabular | display |
| 迷你统计值 | 22 / 800 | display |
| 汇总值 | 20 / 800 / -.03em | display |
| 弹层标题 / 状态页标题 | 16 / 800（状态页 20） | display |
| 卡片标题 | 15 / 600 | body |
| 图表卡标题 | 14 / 600 | body |
| 正文 / 字段 / 按钮 | 13（按钮 600） | body |
| 卡内小按钮 / 分段控件 | 12.5 / 600 | body |
| 说明 / 时间 | 12–12.5 / 400 / muted | body 或 mono |
| 徽章 / 标签 / 表头 | 11.5 / 600（表头 11 大写 .06em） | body |
| 图表刻度 | 10.5 / muted / tabular | body |
| 角标 | 10.5 / 700 | body |

数字全部 `tabular-nums`；ID / 密钥 / 模型名 / URL / 时间戳 mono；标题不换行截断用 `text-overflow: ellipsis`。

## 3. 圆角 · 阴影 · 间距

圆角：5 复选框 · 6 类型标签/角标 · 6–7 品牌底板 · 8 图标按钮/分页/骨架 · 9 卡内按钮/侧栏项/设置导航 · 10 按钮 · 11 分段外框 · 12 字段/下拉/通知/嵌板/气泡容器 · 14 卡片/表格容器 · 16 Hero/套餐/嵌入/弹层 · 20 认证卡/品牌面板 · 999 徽章/开关/头像/进度。

阴影：卡 `--shadow`；悬停 `--shadow-hover`；弹层 / 下拉 `--shadow-pop`；primary 按钮 `0 8px 20px -10px var(--accent)`；hero 主按钮 `0 12px 28px -12px var(--accent)`；品牌按钮 `0 6px 14px -8px <brand>`。

间距只用 4 / 6 / 8 / 10 / 12 / 14 / 16 / 20 / 24（公开页分段 32 / 48 / 80）。内容区 `8px 24px 24px 20px`；页面块 gap 14–16；卡片网格 gap 12；汇总条 gap 10；筛选行 gap 8；按钮组 gap 8；图标与文字 6；标签与字段 6；表格单元格 `0 16px`。

## 4. 控件尺寸

| 控件 | 尺寸 |
|---|---|
| 按钮 md / 卡内 / hero / xs | h34 r10 · h32 r9 · h42 r12 · h26 r8 |
| 图标按钮 页头 / 表格 | 34×34 r10 · 28×28 r8 |
| 字段 / 大字段 / 文本域 | h36 r12 · h40 · min-h 96 |
| 搜索框（筛选行） | h36 w260，图标 15 @ left 12，padding-left 36 |
| 筛选药丸 | h36 r12，`label muted · value 600 · chevron 14` |
| 分段控件 / 小 | h36 r11（项 r8）· h30 |
| 开关 表单 / 紧凑 | 36×20 · 32×18 |
| 复选框 | 16 r5 边 1.5 |
| 徽章 / 标签 | h22 r999 · `3px 7px` r6 |
| 汇总卡 | h64 r12 |
| 统计卡 | padding 14 16，sparkline 96×28 |
| 表头 / 行 / 分页项 | h43 · h59（密钥）/ h61（账号） · 28×28 r8 |
| 桌面顶栏 / 紧凑页头 | 顶栏盒高 66（60 + 上内边 6，原型为 content-box）；h1 24/35、描述 13/19、下距 14 → 首块内容 y=146 |
| 品牌底板 / 行内图标 / 侧栏图标 | 20–24 · 15–16 · 17 |
| 头像（时间线 / 顶栏） | 32 · 28 |
| 弹层宽度档 | 440 / 560 / 720 / 960；抽屉 480 / 640 |
| 设置行 | grid 240px 1fr，gap 12 24，padding 14 20 |
| 分区导航项 | h34 r9 padding 0 10 |
| 移动端触控 / FAB | ≥44 · 52 r16 |

## 5. 页面模板配方（结构逐层）

见 `specs/glass-page-templates/spec.md`。每个模板的骨架组件：`PublicPageLayout`（Landing / Auth / Callback / Setup / NotFound）、`DashboardPageLayout`、`TablePageLayout`（ListPage）、`DetailPageLayout`、`SettingsPageLayout`。页面只能放数据与插槽内容。

## 6. 观感评分表（每页 10 项 × 0–2 分，≥17 达标；原型出稿页 20）

| # | 维度 | 2 分 | 1 分 | 0 分 |
|---|---|---|---|---|
| 1 | 层级 | 标题 > 卡标题 > 正文 > 说明 四级清晰，主操作唯一醒目 | 有一处层级混淆 | 多处同权重 / 主操作不突出 |
| 2 | 留白与节奏 | 间距全部取自节奏表，块间等距 | 1–2 处偏离 | 拥挤或忽疏忽密 |
| 3 | 对齐与栅格 | 所有卡 / 列 / 按钮基线对齐，栅格列比与配方一致 | 一处错位 | 明显错位 / 溢出 |
| 4 | 密度 | 与原型同屏信息量（表格 8–10 行 / 屏，卡片 4 列） | 略疏或略密 | 大片空白或塞满 |
| 5 | 对比度与可读性 | 正文 ≥4.5:1，说明 muted 可读，数字等宽 | 一处偏弱 | 灰上灰 / 键名泄漏 |
| 6 | 玻璃层次与阴影 | 卡片一层玻璃 + 嵌板，无嵌套阴影 | 一处嵌套 | 多层阴影 / 平面白卡 |
| 7 | 状态完整性 | 加载 / 空 / 错误 / 禁用 / 悬停 / 焦点全部实现且同风格 | 缺一种 | 缺两种以上 |
| 8 | 微交互 | hover 上浮、active 缩放、过渡 150–200ms、焦点环 | 部分缺失 | 无过渡 / 默认 outline |
| 9 | 暗色对等 | 暗色截图与亮色布局一致、无亮色残留、对比达标 | 一处残留 | 未适配 |
| 10 | 移动端 | 390 无横滚、单列、卡片模式、FAB、触控 ≥44 | 一处溢出 / 小目标 | 桌面布局硬缩 |

## 7. 反模式清单（出现即扣分或判失败）

- 原始 i18n 键名出现在界面（`admin.dashboard.heroTitle`）。
- 同一列表中新旧单元格并存（如账号页容量列还是旧的红绿小胶囊）。
- 页面自绘按钮 / 输入框 / 徽章 / 卡片；`rounded-2xl shadow-lg bg-white` 残留。
- 卡中卡阴影叠加；统计"大数字三段卡"替代 `.summary-chip`。
- 图表默认色（chart.js 蓝紫 / 白底 tooltip / 深灰网格）。
- 表格操作列与前一列重叠、sticky 列背景与行不一致。
- 空态只有一行"暂无数据"、加载态整页 spinner、错误态 alert()。
- 暗色下白底块 / 浅灰边框 / 黑字。
- 移动端出现横向滚动、桌面表格硬缩、弹层居中被键盘顶出。
- 侧栏底部用户块遮住导航末项（当前 `/admin/orders` 截图可见）。
- 两个功能相同的组件（`common/StatusBadge` 与 `ui/StatusBadge`）在同一页混用。

## 8. 原型偏离登记模板（`deviations.md`）

```
| 路由 | 原型区块 | 原型呈现 | 实际呈现 | 类型(不符/更优) | 理由 | 截图 |
```
