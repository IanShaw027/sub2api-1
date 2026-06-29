# TLS 指纹批量编辑功能 - 测试指南

## ✅ 功能已实现

### 数据库验证

**已验证**：group=14 的 109 个 OpenAI OAuth 账号已成功配置 TLS 指纹

```sql
-- 验证配置
SELECT 
    COUNT(*) as total,
    COUNT(CASE WHEN extra->>'enable_tls_fingerprint' = 'true' THEN 1 END) as enabled_count,
    COUNT(CASE WHEN extra->>'tls_fingerprint_router_id' = '2' THEN 1 END) as router_count
FROM accounts 
WHERE id IN (SELECT account_id FROM account_groups WHERE group_id = 14)
AND deleted_at IS NULL;

-- 结果：109 个账号全部启用，全部使用 router_id=2 (codex-client-router)
```

## 🧪 前端 UI 测试

### 准备工作

1. 启动前端开发服务器：
```bash
cd /opt/sub2api/frontend
npm run dev
```

2. 浏览器访问：`http://localhost:5173`（或配置的端口）

3. 登录管理员账号

### 测试步骤

#### 测试 1：验证 UI 显示条件

1. 进入"账号管理"页面
2. 筛选出 OpenAI 账号
3. 选择多个 OpenAI 账号
4. 点击"批量编辑"按钮
5. **预期**：应该能看到"TLS Fingerprint"配置区块

#### 测试 2：验证平台限制

1. 同时选择 OpenAI 和 Claude 账号（混合平台）
2. 点击"批量编辑"
3. **预期**：不应显示"TLS Fingerprint"配置区块

#### 测试 3：验证 Profile 选项加载

1. 选择支持的账号类型（OpenAI）
2. 打开批量编辑
3. 勾选"TLS Fingerprint"选项
4. 启用 TLS 指纹开关
5. **预期**：
   - Profile 下拉框应显示：
     - "使用默认"
     - "随机选择"（如果有可用 Profile）
     - 各个 Profile 选项（带平台标识）

#### 测试 4：验证 Router 选项加载

1. 在 TLS 指纹配置中
2. 查看 Router 下拉框
3. **预期**：
   - 应显示 "不使用路由器"
   - 应显示 "codex-client-router"
   - 只显示已启用的路由器

#### 测试 5：批量启用 TLS 指纹

1. 选择 5-10 个 OpenAI 账号
2. 打开批量编辑
3. 勾选"TLS Fingerprint"
4. 启用开关
5. 选择 Profile（如"随机选择"）
6. 选择 Router（如"codex-client-router"）
7. 点击"更新"
8. **预期**：显示成功消息

**验证**：
```sql
SELECT id, name, 
       extra->>'enable_tls_fingerprint' as enabled,
       extra->>'tls_fingerprint_profile_id' as profile,
       extra->>'tls_fingerprint_router_id' as router
FROM accounts 
WHERE id IN (选择的账号ID列表)
AND deleted_at IS NULL;
```

#### 测试 6：批量禁用 TLS 指纹

1. 选择已启用 TLS 指纹的账号
2. 打开批量编辑
3. 勾选"TLS Fingerprint"
4. 关闭开关
5. 点击"更新"
6. **预期**：显示成功消息

**验证**：
```sql
-- 应该看到 enable_tls_fingerprint = false 或 null
SELECT id, name, 
       extra->>'enable_tls_fingerprint' as enabled
FROM accounts 
WHERE id IN (选择的账号ID列表)
AND deleted_at IS NULL;
```

#### 测试 7：部分成功处理

1. 选择包含一些无效账号的列表
2. 批量更新 TLS 指纹
3. **预期**：应显示部分成功的消息，列出成功和失败的账号数

#### 测试 8：验证数据持久化

1. 批量启用 TLS 指纹
2. 刷新页面
3. 打开单个账号的编辑页面
4. **预期**：应该看到 TLS 指纹配置正确显示

## 🔍 功能验证清单

- [ ] UI 只对支持的账号类型显示
- [ ] 混合平台时不显示 TLS 指纹选项
- [ ] Profile 列表正确加载
- [ ] Router 列表正确加载（只显示已启用）
- [ ] 可以批量启用 TLS 指纹
- [ ] 可以批量禁用 TLS 指纹
- [ ] 可以选择不同的 Profile（默认/随机/指定）
- [ ] 可以选择不同的 Router
- [ ] 数据正确保存到数据库
- [ ] 与单个账号编辑的行为一致
- [ ] 错误处理正确（部分成功/全部失败）

## 📊 当前配置状态

```
Database: sub2api
Group: 14
Accounts: 109 个 OpenAI OAuth 账号
Status: ✅ 已全部配置 TLS 指纹
Router: codex-client-router (ID: 2)
```

## 🐛 可能的问题排查

### Profile 列表为空
- 检查：`GET /api/v1/admin/tls-fingerprint-profiles` 接口
- 验证：数据库中是否有 Profile 记录

### Router 列表为空
- 检查：`GET /api/v1/admin/tls-fingerprint-routers` 接口
- 验证：是否有启用的 Router（`enabled = true`）

### 更新失败
- 检查：浏览器控制台错误信息
- 验证：后端日志
- 确认：账号是否支持 TLS 指纹

### UI 不显示
- 确认：选择的账号类型是否支持
- 检查：是否混合了多个平台
- 验证：`allTLSFingerprintCapable` 计算属性

## 📚 相关资源

- 详细文档：`/opt/sub2api/BULK_EDIT_TLS_FINGERPRINT.md`
- 实现总结：`/opt/sub2api/TLS_FINGERPRINT_BULK_EDIT_SUMMARY.md`
- 前端组件：`/opt/sub2api/frontend/src/components/account/BulkEditAccountModal.vue`

## ✨ 预期行为

### 启用 TLS 指纹时
```json
{
  "extra": {
    "enable_tls_fingerprint": true,
    "tls_fingerprint_profile_id": -1,  // 或具体 ID，或 null
    "tls_fingerprint_router_id": 2     // 或 null
  }
}
```

### 禁用 TLS 指纹时
```json
{
  "extra": {
    "enable_tls_fingerprint": false,
    "tls_fingerprint_profile_id": null,
    "tls_fingerprint_router_id": null
  }
}
```

## 🎯 验证完成标准

1. ✅ 所有测试步骤通过
2. ✅ 数据库记录正确
3. ✅ UI 显示正常
4. ✅ 错误处理得当
5. ✅ 与现有功能兼容
