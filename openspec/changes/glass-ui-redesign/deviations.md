| 路由 | 原型区块 | 原型呈现 | 实际呈现 | 类型(不符/更优) | 理由 | 截图 |
|---|---|---|---|---|---|---|
| `/home` | 顶部导航 | 仅 登录 / 立即开始 | 增加 语言切换 + 主题切换 | 不符（项目已有 LocaleSwitcher 与主题切换） | 功能必须保留；按 34px 图标按钮排版，待 5.1 复核 | reference/light/01_首页.png vs screens/home/light.png |
| 全站（`--muted` 亮色） | ui-standards.md §1 令牌表 | `oklch(55.17% 0.006 259.82)` | `oklch(50% 0.006 259.82)` | 更优（对比度） | 55.17% 的 L 值在亮色 `--surface-secondary`/`--surface-tertiary` 上不足 4.5:1；50% 是满足 WCAG 1.4.3 的最小安全值（见 tokens.css 注释与 efc598468 提交） | reference/light/07_组件规范.png |
| 全站（`--border` 暗色） | ui-standards.md §1 令牌表 | `oklch(28% 0.003 259.82)` | `oklch(51% 0.003 259.82)` | 更优（对比度） | 28% 的 L 值在暗色 `--surface`/`--background` 上不足 3:1，字段/卡片边框在暗色下几乎不可见（WCAG 1.4.11）；51% 是满足非文本对比度的最小安全值（见 tokens.css 注释与 efc598468 提交） | reference/dark/07_组件规范.png |
