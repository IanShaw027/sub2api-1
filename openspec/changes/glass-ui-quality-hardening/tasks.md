# Tasks

## 0. 基线与依赖环境

- [x] 0.1 固定项目 pnpm 版本，迁移或同步 overrides，重新生成配套 `pnpm-lock.yaml`（文件已交付工作树；未请求 git commit）。
- [x] 0.2 保存改动前路由、导航、列、操作和锚点基线；脚本输出可比较 JSON。
- [x] 0.3 固化 seeded mock 和截图命令，确保空态/有数据态/弹窗态可复现。
- [x] 0.4 运行并记录当前 typecheck、lint、ui-lint、contrast、i18n、Vitest 基线。

## 1. P0 全站修复

- [x] 1.1 调整 dark `--border`/`--border-strong`，更新注释和 contrast 测试。
- [x] 1.2 删除 AppLayout 与 token 背景的重复环境光，核对滚动和路由切换截图。
- [x] 1.3 Settings 删除重复保存按钮，保留统一 Button、loading、dirty 和 reset 行为。
- [x] 1.4 合并唯一 `s2a-pulse` keyframes，补常驻光环和 reduced-motion 测试。

## 2. P1 共享真值收敛

- [x] 2.1 Button、StatusBadge、Dialog、StatCard 建立单一实现或兼容别名。
- [x] 2.2 DataTable sticky/hover 背景全部改为令牌，删除主题选择器分支。
- [x] 2.3 统一 accent 文本、focus ring、激活态阴影和 dropdown/modal 动效。
- [x] 2.4 侧栏 Logo、头像、底部主题/用户块改用共享类和语义色。
- [x] 2.5 统一 `tour-*` 提示框为 notice 语义样式。

## 3. 页面与响应式修复

- [x] 3.1 修复 KeysDataTable 固定列宽、日期 ellipsis、低宽度列策略和移动卡片回退。
- [x] 3.2 将 TablePageLayout pagination 统一放入表格卡 footer。
- [x] 3.3 SettingsView 迁移 SettingsPageLayout/SettingRow，移除 Tailwind 类选择器。
- [x] 3.4 AppHeader 在 768–1023px 保持单行，增加 More 菜单并保留所有原动作。
- [x] 3.5 登录/注册/找回密码统一 TextInput、FieldLabel、错误态和 suffix 行为。
- [x] 3.6 Dashboard/Settings/列表页使用 Skeleton/EmptyState，修复空图表大面积空白。
- [x] 3.7 暗色 hover 增加可见边框反馈；折叠侧栏保留弱归属锚点。
- [x] 3.8 恢复移动 Keys 累计费用、多周期限额及重置时间、全部端点及测速、自定义说明。
- [x] 3.9 恢复 Profile 账户概览、404 控制台入口和监控刷新控件的可访问名称。
- [ ] 3.10 修复历史操作核对中发现的工单模板焦点/预览、Passkey 删除键盘提交、风控时间精度及旧直达操作入口。
- [x] 3.11 管理员请求趋势在窄容器中稀疏显示时间轴标签，保留全部数据点。

## 4. 功能保真与可访问性

- [x] 4.1 比较前后路由和导航集合，确认无入口减少。
- [x] 4.2 比较所有受影响 DataTable columns、默认可见性和列设置迁移。
- [ ] 4.3 比较按钮、链接、菜单 action、API/store 调用和事件处理器。
- [ ] 4.4 比较 `data-tour`、`data-testid`、id、aria-label；差异必须登记等价替代。
- [ ] 4.5 验证 Tab 巡航、Drawer 焦点、Modal 焦点恢复、键盘关闭和 reduced-motion。
- [ ] 4.6 验证 mobile Keys/Accounts/Usage/Settings 的筛选、批量、导出、分页和弹窗工作流。

## 5. 规范同步与最终门禁

- [ ] 5.1 将 muted、border、pulse、分页、顶栏等决策同步到 `ui-standards.md` 和设计稿。
- [ ] 5.2 更新旧视觉文档，删除 Indigo/Gray/dark:bg 旧规范残留。
- [ ] 5.3 补齐 light/dark × 1440/1024/768/390 截图矩阵及交互态截图。
- [ ] 5.4 在本轮最终代码上运行 `vue-tsc --noEmit`、`pnpm run lint:check`、`pnpm run lint:ui`、`pnpm run check:contrast`、`pnpm run i18n:diff`、`pnpm run test:run` 和生产构建（不能复用前一批修改的通过结论）。
- [ ] 5.5 生成最终 Review 报告，列出已修复问题、登记偏差、未完成项和截图证据。
