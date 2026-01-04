# OpsConfigDialog 使用示例

## 在 OpsDashboard.vue 中集成

```vue
<script setup lang="ts">
import { ref } from 'vue'
import OpsConfigDialog from './components/OpsConfigDialog.vue'

// ... 其他代码

const showConfigDialog = ref(false)

function openConfig() {
  showConfigDialog.value = true
}
</script>

<template>
  <AppLayout>
    <div class="space-y-6 pb-12">
      <!-- 在 Header 中添加配置按钮 -->
      <div class="flex justify-between items-center">
        <h1>运维监控</h1>
        <el-button type="primary" @click="openConfig">
          <el-icon><Setting /></el-icon>
          配置
        </el-button>
      </div>

      <!-- ... 其他内容 -->
    </div>

    <!-- 配置弹窗 -->
    <OpsConfigDialog v-model="showConfigDialog" />
  </AppLayout>
</template>
```

## 功能说明

### Tab 1: 告警规则配置
- 列出所有告警规则
- 支持创建、编辑、删除规则
- 支持启用/禁用规则
- 配置指标类型、阈值、告警级别等

### Tab 2: 分组可用性监控
- 显示所有分组的监控状态
- 配置最低可用账号数阈值
- 配置告警级别和通知方式
- 显示当前健康状态

### Tab 3: 邮件通知配置
- 配置告警邮件（收件人、级别、速率限制）
- 配置定时报告（每日、每周、错误摘要、账户健康）

## API 端点

需要后端实现以下 API：

- `GET /api/admin/ops/alert-rules` - 获取告警规则列表
- `POST /api/admin/ops/alert-rules` - 创建告警规则
- `PUT /api/admin/ops/alert-rules/:id` - 更新告警规则
- `DELETE /api/admin/ops/alert-rules/:id` - 删除告警规则
- `GET /api/admin/ops/group-availability/status` - 获取分组可用性状态
- `PUT /api/admin/ops/group-availability/configs/:groupId` - 更新分组配置
- `GET /api/admin/ops/email-notification/config` - 获取邮件通知配置
- `PUT /api/admin/ops/email-notification/config` - 更新邮件通知配置
