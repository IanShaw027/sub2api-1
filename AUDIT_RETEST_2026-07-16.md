# Sub2API 审计复验报告

| 字段 | 值 |
|------|-----|
| **复验日期** | 2026-07-16 |
| **对照基线** | `AUDIT_REPORT_2026-07-16.md`（HEAD `eb64a5c16` 静态深审） |
| **当前状态** | 工作树 **未提交** 修复（约 203 files, +7489 / -1791） |
| **复验模式** | 主会话源码交叉核验 + 定向 unit tests |
| **结论** | **P0 全部关闭；P1 绝大多数已关闭**；剩余 1 个 P1 级调度缓存窗口 + 若干 P2 |

---

## 1. 执行摘要

修复质量整体**扎实**：支付 at-rest 加密、部署默认面、Skill ledger/attempt、Chat/Images partial 对齐、API Key hash、邀请/重置原子性、密码策略、Bulk 平台硬拦、OAuth refresh fail-closed、前端 token 存储模型等均有**可验证源码 + 测试**证据。

| 基线项 | 复验结果 |
|--------|----------|
| **P0 × 3** | **3/3 关闭** |
| **P1 × 14** | **13 关闭 / 1 部分（proxy 热同步）/ Soft RL 按产品语义关闭** |
| **P1-13 JWT localStorage** | **关闭（升级）**：access 内存 + refresh HttpOnly cookie |
| 定向单测 | `config` / `handler` partial / `service` Skill&Payment&Bulk / `repository` reset — **通过** |

### 1.1 当前发布建议（相对基线更新）

| 场景 | 建议 |
|------|------|
| 内部 dev / 个人 | **可继续** |
| 对外 SaaS / 收款 | **可进入预发布**，需：提交本批修复、配置稳定 `TOTP_ENCRYPTION_KEY`、跑迁移 `216_protect_user_api_keys`、确认 Redis 密码与 JWT |
| 多副本生产 | **可上**，建议补齐 proxy expiry 后 `syncSchedulerAccountSnapshots`（见残余 P1-R1）后再压测 |

---

## 2. P0 复验（全部关闭）

### ✅ P0-1 支付渠道配置 at-rest 加密

| 检查 | 结果 |
|------|------|
| `encryptConfig` | 调用 `payment.Encrypt`，非明文 JSON |
| 密钥 | `ProvideEncryptionKey`：须 **显式配置** 的 32-byte TOTP encryption key（自动生成 key **拒绝** 用于支付） |
| 迁移 | `MigrateProviderConfigsToEncrypted`：存在 instance 时强制 32-byte key |
| 读路径 | `decryptConfig` 仍兼容历史明文（迁移期） |
| 测试 | `TestPaymentConfigServiceEncryptConfigRoundTrip` 等存在 |

```go
// payment_config_providers.go
encrypted, err := payment.Encrypt(string(data), s.encryptionKey)
```

**运维注意（非漏洞）**：支付加密与 TOTP 共用 `TOTP_ENCRYPTION_KEY`；多副本必须固定同一密钥，否则无法解密历史 config / resume token。

---

### ✅ P0-2 示例弱 JWT / admin123

| 检查 | 结果 |
|------|------|
| `deploy/config.example.yaml` `jwt.secret` | `""` + openssl 注释 |
| `admin_password` | `""`；注释标明 legacy 不创建管理员 |
| `isWeakJWTSecret` | 含 `change-this-to-a-secure-random-string` + 全同字符检测 |
| `Validate()` | weak secret → error |
| 测试 | `config` WeakJWT 相关 — **PASS** |

---

### ✅ P0-3 compose 默认绑定

| 检查 | 结果 |
|------|------|
| `docker-compose.yml` | `BIND_HOST:-127.0.0.1` |
| `docker-compose.standalone.yml` | 同左 |
| `.env.example` | `BIND_HOST=127.0.0.1` + 公网需显式说明 |
| `SERVER_HOST=0.0.0.0` | 容器内监听合理；端口映射默认本机 |

---

## 3. P1 复验

### ✅ P1-1 Skill 退款后 replay 新 reference

- `aiSkillSettlementChargeReference(runID, attempt)` → `ai_skill_run:{id}` / `ai_skill_run:{id}:attempt:N`
- 元数据 key `_sub2api_charge_attempt`
- 单测断言 `attempt:2` 与 safe/unsafe replay 语义 — **PASS**

### ✅ P1-2 Skill 扣款 fail-closed

```sql
WHERE id = $2 AND deleted_at IS NULL AND balance >= $1
-- → ErrInsufficientBalance
```

### ✅ P1-3 API Key 非明文落库

- Schema：`lookup_hash` + `key_ciphertext` + `key_prefix`；legacy `key` 写 hash
- 迁移：`216_protect_user_api_keys.sql`
- `GetByKey` / `GetByKeyForAuth`：优先 `LookupHashEQ`，兼容旧明文行
- Create 路径写 hash + 可选密文（reveal）

### ✅ P1-4 邮箱注册邀请码原子化

- `CreateWithInvitation` 路径；Use 失败映射 `ErrInvitationCodeInvalid`
- 不再「Use 失败仍放行」

### ✅ P1-5 密码重置原子 claim

- `ClaimPasswordResetToken` Redis Lua 原子预占
- `Complete` / `Restore` 支持更新失败回滚
- 旧 Verify+Delete 竞态窗口关闭

### ✅ P1-6 密码 min=8

- 注册 / 重置：`binding:"required,min=8"`
- 前端 Register 校验 `< 8`

### ✅ P1-7 Chat / Images partial 对齐 Responses

Chat / Images 在 `err!=nil && result!=nil` 时：

1. `RecordUsage`（partial）  
2. `ReportOpenAIAccountScheduleResult(..., false, ...)`  
3. `ensureForwardErrorResponse`  
4. **`return`**  

handler 定向测 — **PASS**。

### ✅ P1-8 Soft RL「恒 false」— 按产品语义关闭

`classifyOpenAIWSSoftRateLimitAdvisory` **已删除**。  
热路径注释明确：`codex.rate_limits` **仅遥测**，不因占用百分比 eviction / failover。  
仍 `recordOpenAIWSCodexRateLimitSnapshot`。硬 429 / `error` / `response.failed` 仍走错误分类。

**判定**：不再是「死代码假装防护」，而是**有意产品语义**。若运维期望「≥90% 主动 temp-unsched」，需另开需求，不算本批回归。

### ⚠️ P1-9 Proxy expiry 热同步 — **仍部分开放（降为 P1-R1）**

`SweepExpiredProxies` 仍为：事务内改投 + outbox `account_bulk_changed`，**commit 后无 `syncSchedulerAccountSnapshots`**。  
`ProxyExpiryService.runOnce` 也不做额外 cache 刷。  
`hydrateSelectedAccount` 仍读 scheduler snapshot full account。

**影响**：outbox rebuild 完成前，请求可能短暂使用缓存中的旧 `proxy_id`。  
**严重度**：仍 P1（出网路径），但有 outbox 收敛，窗口通常秒级。  
**建议**：对齐 auto-pause，commit 后 `syncSchedulerAccountSnapshots(changedIDs)`。

### ✅ P1-10 OAuth refresh Redis 锁 fail-closed

- `lockErr` 重试后 → `ErrServiceUnavailable`（`oauth_refresh_lock_unavailable_fail_closed`）
- **不再**无锁双刷降级

### ✅ P1-11 Bulk 多平台

| 层 | 行为 |
|----|------|
| 前端 | `selectedPlatforms.length !== 1` 拒绝打开 bulk（含过滤模式空 platforms） |
| 后端 | `validateBulkUpdateTargetPlatform`：**任意 bulk** 必须单平台（不仅 Credentials/Extra） |

### ✅ P1-12 注册密码不再进 sessionStorage

- 仅 email / promo / invitation / countdown
- 注释要求验证页重输密码；`formData.password = ''`

### ✅ P1-13 JWT / Refresh 存储模型升级

| 凭证 | 存储 |
|------|------|
| Access token | **进程内存**（`authSession.setAccessToken`） |
| Refresh | **HttpOnly cookie**（`setRefreshTokenCookie` / `withCredentials`） |
| 旧 localStorage keys | `clearLegacyAuthStorage` 清理 |

XSS 仍可能窃取**内存中的 access**（短生命周期），但 **refresh 不再 JS 可读**——相对基线显著加固。

### ✅ P1-14 Redis 密码强制（compose）

```yaml
REDIS_PASSWORD=${REDIS_PASSWORD:?REDIS_PASSWORD is required}
# redis 启动 --requirepass
```

`.env.example` 仍可空（需部署时填），compose **fail-closed** 正确。

---

## 4. 其它基线项 / P2 抽查

| 项 | 状态 |
|----|------|
| install.sh checksum 缺失 | **已 hard fail**（`print_error` + exit） |
| OAuth hash 清理 | **已** `history.replaceState` 清 hash |
| secret_scan placeholder | **已扩展** `is_probably_placeholder` |
| CustomPage iframe sandbox | **未改**（仍无 sandbox）→ 残余 P2 |
| logredact 默认键 | **未扩** authorization / api_key / sk- → 残余 P2 |
| 支付加密复用 TOTP key | 有意设计；须文档/运维固定密钥 |

---

## 5. 定向测试证据

```text
go test -tags=unit ./internal/service/ -run 'PaymentConfigServiceEncrypt|AISkill.*Replay|AISkill.*Charge|BulkUpdate.*Mixed|BulkUpdate.*Platform'
→ ok

go test -tags=unit ./internal/config/ -run 'WeakJWT|JWT'
→ ok

go test -tags=unit ./internal/handler/ -run 'ChatCompletions.*Partial|Images.*Partial|PartialError'
→ ok

go test -tags=unit ./internal/repository/ -run 'ClaimPasswordReset|PasswordReset'
→ ok
```

> 未跑全量 `./...` / 前端 vitest / 集成 testcontainers；上线前建议补全。

---

## 6. 残余问题清单

### P1-R1. Proxy expiry 后 full-account 缓存未热同步

- **位置**：`backend/internal/repository/proxy_repo.go` `SweepExpiredProxies`；`proxy_expiry_service.go`
- **建议**：commit 后调用与 auto-pause 相同的 `syncSchedulerAccountSnapshots`
- **验证**：双实例 sweep → 立即 hydrate 断言 `proxy_id` 已更新

### P2-R1. CustomPage 嵌入 iframe 无 sandbox

- `CustomPageView.vue` 仍裸 iframe；HomeView 有 sandbox 可参考

### P2-R2. logredact 默认敏感键不全

- 仍缺 `authorization` / `api_key` / `x-api-key` / `sk-` 启发式

### P2-R3. 支付加密密钥与 TOTP 绑定

- 非缺陷，但需运维清单：生产强制 `TOTP_ENCRYPTION_KEY` 稳定 32-byte hex，并备份

### P2-R4. 本批修复未提交

- 工作树 200+ 文件变更；合入前建议：`golangci-lint`、前端 `typecheck`/`test:run`、迁移 dry-run

---

## 7. 对照矩阵（基线 → 复验）

| ID | 标题 | 基线 | 复验 |
|----|------|------|------|
| P0-1 | 支付 config 明文 | 开放 | **关闭** |
| P0-2 | 弱 JWT / admin123 example | 开放 | **关闭** |
| P0-3 | BIND 0.0.0.0 | 开放 | **关闭** |
| P1-1 | Skill replay reference | 开放 | **关闭** |
| P1-2 | Skill 余额透支 | 开放 | **关闭** |
| P1-3 | API Key 明文 | 开放 | **关闭** |
| P1-4 | 邀请 fail-open | 开放 | **关闭** |
| P1-5 | 重置非原子 | 开放 | **关闭** |
| P1-6 | 密码 min=6 | 开放 | **关闭** |
| P1-7 | Chat/Images partial 落穿 | 开放 | **关闭** |
| P1-8 | Soft RL 死代码 | 开放 | **关闭（产品语义）** |
| P1-9 | Proxy 缓存热同步 | 开放 | **部分开放** |
| P1-10 | OAuth 无锁刷新 | 开放 | **关闭** |
| P1-11 | Bulk 多平台 | 开放 | **关闭** |
| P1-12 | 注册密码 sessionStorage | 开放 | **关闭** |
| P1-13 | JWT localStorage | 开放 | **关闭（内存+HttpOnly）** |
| P1-14 | Redis 空密码 | 开放 | **关闭（compose 强制）** |

**关闭率**：P0 100%；P1 约 **93%**（13/14 关闭，1 部分）。

---

## 8. 复验结论

本批修复**达到可预发布水平**，显著好于 `AUDIT_REPORT_2026-07-16` 基线：

1. 资金与密钥面（支付加密、Skill 幂等/透支、API Key hash）处理到位  
2. 部署默认面（JWT example、bind、Redis 密码）收紧正确  
3. 网关 partial 调度信号与 Responses 对齐  
4. 会话模型升级到 access 内存 + refresh HttpOnly，属于超出预期的加固  

**合并前仅建议强制跟进：**

1. 修复 **P1-R1** proxy 热同步（小改、高收益）  
2. 提交 + 全量测试 + 迁移演练  
3. 生产配置检查清单：`JWT_SECRET`、`TOTP_ENCRYPTION_KEY`、`REDIS_PASSWORD`、`BIND_HOST`

---

*复验：2026-07-16 · 只读 · 工作树未提交修复 · 对照 `AUDIT_REPORT_2026-07-16.md`*
