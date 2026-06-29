# 批量编辑账号 - TLS 指纹支持

## 概述

为账号批量编辑功能添加了 TLS 指纹模拟配置支持，允许管理员批量为符合条件的账号启用和配置 TLS 指纹伪装。

## 修改内容

### 后端

后端已经支持通过 `extra` 字段批量更新 TLS 指纹配置，无需修改。

- API 端点：`POST /api/v1/admin/accounts/bulk-update`
- 支持的字段：
  - `extra.enable_tls_fingerprint`: boolean - 是否启用 TLS 指纹
  - `extra.tls_fingerprint_profile_id`: number | null - TLS 指纹配置文件 ID（-1 表示随机选择）
  - `extra.tls_fingerprint_router_id`: number | null - TLS 指纹路由器 ID

### 前端

修改文件：`frontend/src/components/account/BulkEditAccountModal.vue`

#### 1. 新增导入

```typescript
import type { TLSFingerprintProfileOption, SelectableTLSFingerprintProfileOption } from '@/components/account/tlsFingerprintProfileOptions'
import { getSelectableTLSFingerprintProfiles, formatTLSFingerprintProfileOptionLabel } from '@/components/account/tlsFingerprintProfileOptions'
import * as tlsFingerprintProfileAPI from '@/api/admin/tlsFingerprintProfile'
import * as tlsFingerprintRouterAPI from '@/api/admin/tlsFingerprintRouter'
```

#### 2. 新增计算属性

- `allTLSFingerprintCapable`: 判断选中的账号是否全部支持 TLS 指纹
  - Anthropic OAuth/SetupToken
  - OpenAI（所有类型）
  - Kiro OAuth

#### 3. 新增状态变量

```typescript
const enableTLSFingerprint = ref(false)           // 是否启用 TLS 指纹设置
const tlsFingerprintEnabled = ref(false)          // TLS 指纹开关状态
const tlsFingerprintProfileId = ref<number | null>(null)  // 选中的 Profile ID
const tlsFingerprintRouterId = ref<number | null>(null)   // 选中的 Router ID
const tlsFingerprintProfiles = ref<TLSFingerprintProfileOption[]>([])  // Profile 列表
const tlsFingerprintRouters = ref<{ id: number; name: string }[]>([])  // Router 列表
```

#### 4. 新增 UI 部分

在 RPM 限制和分组设置之间添加了 TLS 指纹配置区块：

- **显示条件**：`v-if="allTLSFingerprintCapable"` - 仅当所有选中账号支持 TLS 指纹时显示
- **配置项**：
  - 主开关：启用/禁用 TLS 指纹
  - Profile 选择：
    - 使用默认（null）
    - 随机选择（-1）
    - 指定 Profile ID
  - Router 选择：
    - 不使用路由器（null）
    - 指定 Router ID

#### 5. 数据加载

在组件初始化时加载 TLS 指纹配置数据：

```typescript
tlsFingerprintProfileAPI.list().then((profiles) => {
  tlsFingerprintProfiles.value = profiles.map(p => ({ id: p.id, name: p.name, platform: p.platform }))
})

tlsFingerprintRouterAPI.list().then((routers) => {
  tlsFingerprintRouters.value = routers.filter(r => r.enabled).map(r => ({ id: r.id, name: r.name }))
})
```

#### 6. 提交逻辑

在 `buildUpdatePayload` 函数中添加 TLS 指纹字段处理：

```typescript
if (enableTLSFingerprint.value) {
  const extra = ensureExtra()
  if (tlsFingerprintEnabled.value) {
    extra.enable_tls_fingerprint = true
    extra.tls_fingerprint_profile_id = tlsFingerprintProfileId.value
    extra.tls_fingerprint_router_id = tlsFingerprintRouterId.value
  } else {
    // 关闭 TLS 指纹
    extra.enable_tls_fingerprint = false
    extra.tls_fingerprint_profile_id = null
    extra.tls_fingerprint_router_id = null
  }
}
```

## 使用方法

### 批量启用 TLS 指纹

1. 在账号列表页选择支持 TLS 指纹的账号（Anthropic OAuth/SetupToken、OpenAI、Kiro OAuth）
2. 点击"批量编辑"按钮
3. 勾选"TLS Fingerprint"选项
4. 打开 TLS 指纹开关
5. 选择 Profile：
   - "使用默认"：使用内置的默认配置
   - "随机选择"：每次请求随机选择一个可用的 Profile
   - 指定 Profile：使用特定的 TLS 指纹配置
6. （可选）选择 Router：用于根据 UA、Transport 等自动路由到不同的 Profile
7. 点击"更新"按钮

### 批量禁用 TLS 指纹

1. 选择账号并点击"批量编辑"
2. 勾选"TLS Fingerprint"选项
3. 关闭 TLS 指纹开关
4. 点击"更新"按钮

## 测试验证

可以通过以下 SQL 验证批量更新是否成功：

```sql
-- 查看 group=14 的账号 TLS 指纹配置
SELECT 
    id, 
    name,
    extra->>'enable_tls_fingerprint' as tls_enabled,
    extra->>'tls_fingerprint_profile_id' as profile_id,
    extra->>'tls_fingerprint_router_id' as router_id
FROM accounts 
WHERE id IN (SELECT account_id FROM account_groups WHERE group_id = 14)
AND deleted_at IS NULL
LIMIT 20;
```

## 支持的账号类型

根据 `backend/internal/handler/dto/mappers.go:supportsAccountTLSFingerprint()` 函数：

1. **Anthropic OAuth/SetupToken** - Claude OAuth 账号和 SetupToken 账号
2. **OpenAI（所有类型）** - OpenAI API Key、OAuth、Codex 等
3. **Kiro OAuth** - Kiro 平台的 OAuth 账号

## 注意事项

1. **平台限制**：批量编辑时，只有当所有选中的账号都支持 TLS 指纹时，才会显示 TLS 指纹配置选项
2. **Profile 平台匹配**：Profile 可以绑定到特定平台，选择时会显示平台不匹配的警告
3. **Router 优先级**：如果同时设置了 Profile ID 和 Router ID，Router 会根据请求特征动态选择 Profile
4. **默认行为**：
   - Profile ID = null：使用内置默认配置
   - Profile ID = -1：随机选择可用 Profile
   - Router ID = null：不使用路由器，直接使用指定的 Profile

## 相关文件

- 前端组件：`frontend/src/components/account/BulkEditAccountModal.vue`
- 后端 Handler：`backend/internal/handler/admin/account_handler.go`
- TLS 指纹 Profile API：`frontend/src/api/admin/tlsFingerprintProfile.ts`
- TLS 指纹 Router API：`frontend/src/api/admin/tlsFingerprintRouter.ts`
- 工具函数：`frontend/src/components/account/tlsFingerprintProfileOptions.ts`
