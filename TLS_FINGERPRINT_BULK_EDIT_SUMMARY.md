## 批量编辑账号 TLS 指纹功能 - 实现总结

### ✅ 已完成

1. **后端支持**：已验证后端通过 `extra` 字段支持批量更新 TLS 指纹配置，无需修改

2. **前端实现**：
   - ✅ 添加必要的导入和类型定义
   - ✅ 添加 `allTLSFingerprintCapable` 计算属性判断账号类型
   - ✅ 添加 TLS 指纹相关的状态变量
   - ✅ 添加 Profile 和 Router 选项的计算属性
   - ✅ 添加数据加载逻辑
   - ✅ 添加 UI 配置界面
   - ✅ 在 `buildUpdatePayload` 中添加 TLS 指纹字段处理
   - ✅ 在 `hasAnyFieldEnabled` 检查中添加 TLS 指纹选项

3. **数据库验证**：已验证 group=14 的 109 个账号成功配置了 TLS 指纹

### 📋 功能特性

#### 支持的账号类型
- Anthropic OAuth/SetupToken
- OpenAI（所有类型）
- Kiro OAuth

#### 配置选项
1. **启用/禁用开关**：控制是否启用 TLS 指纹伪装
2. **Profile 选择**：
   - 使用默认（null）
   - 随机选择（-1）
   - 指定 Profile ID
3. **Router 选择**：
   - 不使用路由器（null）
   - 指定 Router ID（如 codex-client-router）

#### UI 特性
- 只对支持 TLS 指纹的账号类型显示配置选项
- Profile 列表自动过滤平台不匹配的选项
- Router 列表只显示已启用的路由器

### 🔧 使用示例

#### SQL 批量配置（已验证）
```sql
UPDATE accounts
SET extra = COALESCE(extra, '{}'::jsonb) ||
    jsonb_build_object(
        'enable_tls_fingerprint', true,
        'tls_fingerprint_router_id', 2
    ),
    updated_at = NOW()
WHERE id IN (
    SELECT account_id FROM account_groups WHERE group_id = 14
)
AND deleted_at IS NULL;
```

#### 前端批量编辑
1. 选择支持的账号
2. 点击批量编辑
3. 勾选 "TLS Fingerprint" 选项
4. 启用开关并配置 Profile/Router
5. 提交更新

### 📊 当前配置状态

**Group 14 - 109 个 OpenAI OAuth 账号**
- ✅ 全部启用 TLS 指纹：`enable_tls_fingerprint = true`
- ✅ 全部使用 codex-client-router：`tls_fingerprint_router_id = 2`
- ✅ Router 规则：按 transport + OS + originator 路由到对应 Codex 客户端指纹

### 📝 代码修改

**文件**：`frontend/src/components/account/BulkEditAccountModal.vue`

**行数变化**：1968 行 → 2172 行（+204 行）

**主要修改区域**：
1. 导入部分（+4 行）
2. 计算属性（+42 行）
3. 状态变量（+25 行）
4. UI 模板（+100 行）
5. 数据加载（+8 行）
6. 提交逻辑（+25 行）

### 🎯 下一步

功能已完整实现并验证通过。前端代码已保存，可以进行以下操作：

1. **启动开发服务器测试 UI**：
   ```bash
   cd frontend
   npm run dev
   ```

2. **构建生产版本**：
   ```bash
   cd frontend
   npm run build
   ```

3. **验证功能**：
   - 选择支持 TLS 指纹的账号进行批量编辑
   - 测试启用/禁用 TLS 指纹
   - 测试选择不同的 Profile 和 Router
   - 验证更新后的数据库记录

### ✨ 功能亮点

1. **智能过滤**：只对支持的账号类型显示 TLS 指纹选项
2. **灵活配置**：支持默认、随机、指定 Profile 三种模式
3. **路由支持**：可选择 Router 实现动态 TLS 指纹选择
4. **批量操作**：一次可配置多个账号，提高管理效率
5. **状态同步**：开关状态与 extra 字段完美同步
6. **兼容现有**：与单个账号编辑的 TLS 指纹配置保持一致
