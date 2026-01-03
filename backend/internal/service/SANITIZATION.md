# 错误信息脱敏策略

## 概述

为了保护敏感信息不被记录到错误日志中，我们实现了统一的脱敏策略。所有错误日志在存储到数据库之前都会经过脱敏处理。

## 脱敏规则

### 1. API Key 脱敏
- **策略**: 只保留前 8 位字符，其余替换为 `***`
- **匹配模式**:
  - `sk-` 开头的 OpenAI 风格 API Key（至少 8 个字符）
  - `key=`, `apikey=`, `api_key=` 等键值对形式（至少 12 个字符）
- **示例**:
  - `sk-1234567890abcdefghij` → `sk-12345***`
  - `key=abc123def456ghi789` → `key=abc123de***`

### 2. Token 脱敏
- **策略**: 完全脱敏为 `***`
- **匹配模式**:
  - Bearer Token: `Bearer <token>`
  - 键值对形式: `token=<value>`
- **示例**:
  - `Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9` → `***`
  - `token=xyz789abc123` → `***`

### 3. Email 脱敏
- **策略**: 使用 SHA256 哈希处理，保留前 16 位哈希值
- **匹配模式**: 标准 Email 格式
- **示例**:
  - `user@example.com` → `email_f02f61d33aac1c8d`

### 4. URL 查询参数脱敏
- **策略**: 保留参数名，值替换为 `***`
- **匹配模式**: URL 中的敏感查询参数
  - `key`, `apikey`, `api_key`
  - `token`, `access_token`, `refresh_token`
  - `client_secret`
- **示例**:
  - `https://api.example.com/v1/chat?api_key=secret123` → `https://api.example.com/v1/chat?api_key=***`

## 应用范围

脱敏策略应用于以下 `OpsErrorLog` 字段：
- `Message`: 错误消息
- `ErrorBody`: 错误响应体
- `UpstreamErrorMessage`: 上游错误消息
- `UpstreamErrorDetail`: 上游错误详情
- `RequestBody`: 请求体（已有独立的脱敏逻辑）

## 实现位置

- **文件**: `backend/internal/service/ops_service.go`
- **核心函数**:
  - `sanitizeErrorMessage()`: 统一脱敏入口
  - `sanitizeAPIKey()`: API Key 脱敏
  - `sanitizeToken()`: Token 脱敏
  - `hashSensitiveData()`: 哈希处理
- **应用点**: `RecordError()` 方法在存储错误日志前调用

## 测试

测试文件: `backend/internal/service/ops_sanitize_test.go`

运行测试:
```bash
go test -v ./internal/service -run TestSanitize
```

## 注意事项

1. **调试信息保留**: 脱敏策略在保护敏感信息的同时，保留了足够的调试信息（如 API Key 前 8 位）
2. **性能考虑**: 正则表达式在包初始化时编译，避免运行时重复编译
3. **哈希一致性**: 相同的敏感数据会产生相同的哈希值，便于追踪同一用户的错误
4. **扩展性**: 可以通过修改正则表达式轻松添加新的敏感模式

## 未来改进

- 考虑添加更多敏感模式（如信用卡号、手机号等）
- 支持自定义脱敏规则配置
- 添加脱敏性能监控
