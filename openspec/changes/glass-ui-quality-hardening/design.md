# Design

## Invariants

1. 令牌是颜色、边框、阴影、字号和圆角的唯一真值；页面 scoped CSS 只负责布局。
2. 原有功能入口必须保持可发现。折叠分组必须有明确展开状态，移动端必须有等价入口。
3. 信息存在不等于功能保真：关键字段必须在目标视口可读，不能被裁剪、重叠或仅依赖 hover。
4. 明暗主题只通过令牌切换；不得新增主题专用分支。
5. 主操作唯一醒目；同一语义的 Button、Badge、Modal、StatCard 和分页只保留一个视觉真值。

## Token decisions

- `glass-dark --border` 回到 `oklch(28% 0.003 259.82)`。
- `--border-strong` 只用于控件边界和焦点环。
- `.app-shell` 与 `.app-shell-glow` 只保留一层环境光。
- `--muted` 采用当前实现的 50% 值，并同步 `ui-standards.md`、设计稿和参考截图。
- `s2a-pulse` 只保留设计版 keyframes，并补充 3px 常驻光环。

## Layout and interaction decisions

- Keys 表格在桌面端保证日期、状态、操作列不重叠；低宽度启用优先级列隐藏或横向滚动，移动端使用既有卡片模式。
- TablePageLayout 将 pagination 放进表格卡 footer，所有 ListPage 统一。
- 768–1023px 顶栏保持单行；次要操作进入 More 菜单。
- Settings 使用 SettingsPageLayout/SettingRow，删除重复保存按钮。
- 认证页统一 TextInput、FieldLabel、错误语义和密码 suffix。
- 空数据使用 EmptyState，加载使用 Skeleton，禁止无上下文整页 spinner。

## Functional preservation protocol

实施前导出基线并实施后比较：

- 路由 path 集合
- 侧栏和移动 Drawer 的 NavItem path 集合
- DataTable columns key 集合和默认可见性
- button/link/menu action 标识
- `data-tour`、`data-testid`、id 和 aria-label 集合
- 每个受影响页面的 API/store 调用和事件处理器

任何差异必须是已登记的等价迁移；未解释的减少视为阻塞。
