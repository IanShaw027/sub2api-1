# OpsConfigDialog

该组件是一个 **“分组可用性监控”批量配置弹窗**，目前用于在分组管理页快速配置分组监控策略。

注意：本 README 曾包含「告警规则配置」「邮件通知配置」等 Tab 的描述，但当前代码中并未实现这些 Tab。
告警规则/事件、邮件通知、运行时设置等功能已经迁移到 `/admin/ops` 运维监控页的独立卡片组件中。

## 使用位置

- `frontend/src/views/admin/GroupsView.vue` 中作为 “监控配置” 按钮打开的弹窗

## 功能范围（当前实现）

- 批量启用/禁用分组可用性监控
- 批量设置最低可用账号数阈值
- 批量设置告警级别（critical / warning / info）
- 应用策略模板（严格/标准/宽松）

## 依赖的 API 端点

- `GET /api/v1/admin/ops/group-availability/status`：读取分组可用性状态（用于列表与选择）
- `PUT /api/v1/admin/ops/group-availability/configs/:groupId`：更新分组监控配置
