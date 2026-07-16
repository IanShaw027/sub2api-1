# Sub2API 全量深度审计报告

| 字段 | 值 |
|------|-----|
| **报告日期** | 2026-07-16 |
| **分支** | `personal-dev` |
| **HEAD** | `eb64a5c1678eeb3dd4111e9a366e0b963f58a1e2` (`eb64a5c16`) |
| **对照基线** | `AUDIT_REPORT_2026-07-14.md`（约 3 天窗口深审） |
| **自 07-14 以来提交** | ~34 commits（含 gateway/security/scheduler 修复与 merge） |
| **审计模式** | 只读静态深审 + 六路领域子代理并行 + 主会话对关键路径源码交叉核验 |
| **方法** | explore 子代理 ×6（Billing / Auth / Gateway / Scheduler / Frontend / Deploy）→ 主代理复验 P0/P1 证据 → 汇总 |

---

## 1. 执行摘要

### 1.1 一句话结论

相对 07-14 审计，**资金双扣、嵌入 JWT 外泄、WS 无 capability 门闩、outbox watermark 回退** 等历史 P0 **主路径已关闭**；工程在计费幂等、OAuth 多副本 consume、调度 claim/lease、CORS fail-closed 等方面成熟度明显上升。  
当前更突出的是：**密钥/支付配置明文落库**、**部署示例可被直接用于生产的危险默认**、**网关 Chat/Images partial 错误落穿成功路径**、**Skill 退款后 replay 无法再扣 marketplace 费**，以及若干认证原子性与前端存储面问题。  
**不建议在未处理下方「发布阻塞清单」的情况下作为对外 SaaS 收款基线**；内部/个人环境可继续迭代。

### 1.2 严重度统计（去重后）

| 等级 | 数量 | 含义 |
|------|------|------|
| **P0 阻塞** | 3 | 部署默认面可导致会话伪造/公网裸奔；支付密钥明文 = 商户接管面 |
| **P1 应修** | 14 | 资金旁路一致性、网关调度失真、认证原子性、密钥 at-rest、前端 footgun |
| **P2 改进** | 16+ | 默认 TTL、iframe sandbox、软限流死代码、文档/测试对齐 |
| **已关闭（旧 P0）** | 5 | 见 §3 |

### 1.3 发布建议

| 场景 | 建议 |
|------|------|
| 内部 dev / 个人环境 | 可继续迭代；立刻改 JWT/admin 示例与公网绑定习惯 |
| 多副本生产 | **注意**：proxy expiry 后 full-account 缓存热同步缺口；OAuth refresh 在 Redis 故障时无锁降级 |
| 对外 SaaS / 收款上线 | **阻塞**：支付配置明文（P0-1）、API Key 明文落库（P1）、Skill 退款 replay（P1）未处理前不建议 |
| 仅 OpenAI 网关热修 | 至少先修 Chat/Images partial 落穿 + soft RL 语义澄清 |

### 1.4 审计范围与方法

#### 六路子代理分工

| # | 领域 | 焦点 | 子代理结论摘要 |
|---|------|------|----------------|
| 1 | Billing / Payment / Skill | 双扣、webhook、ledger、partial | 旧双扣已修；新 P1：退款后 reference 卡死、支付明文、Skill 可透支 |
| 2 | Auth / OAuth / Session | JWT embed、OAuth consume、API key reveal、邀请/重置 | 旧 P0 已修；新 P1：API key 明文、邀请 fail-open、重置非原子 |
| 3 | Gateway / WS / Protocol | partial、prewrite、soft RL、compact、web-search | Chat/Images 落穿；soft RL classify 恒 false；web-search orphan fix 正确 |
| 4 | Scheduler / Account pool | watermark、coalesce、proxy、Grok active-delta | watermark 已 monotonic 且死代码；proxy expiry 缓存缺口 |
| 5 | Frontend | XSS、bulk-edit、支付 UI、token 存储 | localStorage JWT；bulk 过滤模式假阴性；注册密码 sessionStorage |
| 6 | Deploy / Infra / CI | 默认密钥、compose、迁移、secret_scan | example 弱 JWT/admin123；0.0.0.0 绑定；checksum 软失败 |

#### 主会话交叉核验（抽样）

| 项 | 核验结果 |
|----|----------|
| Skill `ChargeUserBalance` → ledger + Reference | **已修复**：`wire.go:382-415` → `ApplyAISkillBalanceCharge` |
| `ai_skill_balance_ledger` UNIQUE(reference, operation) | **确认**：`migrations/209_ai_skill_balance_ledger.sql` |
| embed URL 不再写 `token` query | **已修复**：`embedded-url.ts` 仅 theme/lang/user_id/src_* |
| Outbox watermark monotonic Lua | **确认**：`scheduler_cache.go:109-118,512-519`；生产 poll 走 claim/lease |
| WS prewrite capability 门闩 | **确认**：`shouldOpenAIWSSessionPrewritePing` 检查 `SupportsIdlePingWithoutReader` |
| Images/WS partial 不再仅看 ImageCount | **确认**：handler 注释与 `result != nil` 路径 |
| 支付 `encryptConfig` 明文 JSON | **确认**：`payment_config_providers.go:555-564` |
| Chat Completions partial 落穿 | **确认**：`openai_chat_completions.go:263-341` 无 return 后 `Report*(true)` |
| Soft RL classify 恒 false | **确认**：`openai_ws_forwarder.go:8573-8586` |
| Proxy expiry 无 post-commit SetAccount | **确认**：`proxy_repo.go:489-549` 仅 outbox |
| Bulk 后端 mixed platform 校验 | **确认**：`validateBulkModelRoutingUpdatePlatforms` 仅在 Credentials/Extra 非空时生效 |
| config.example 弱 JWT + admin123 | **确认**：`deploy/config.example.yaml:980,1078`；weak 列表未覆盖 |

#### 规模快照

| 指标 | 约值 |
|------|------|
| `backend/internal/service` Go 文件 | 1127 |
| `backend/internal/handler` Go 文件 | 325 |
| `backend/internal/repository` Go 文件 | 285 |
| `frontend/src` Vue/TS | 810 |
| 模块路径 | `github.com/Wei-Shaw/sub2api` |
| 技术栈 | Go 1.26 + Gin + Ent + Wire / Vue 3 + pnpm / PostgreSQL + Redis |

#### 局限

- 本报告为**静态深审**，未在本轮重跑全量 `go test -tags=unit ./...` / `pnpm test:run` / 双实例竞态压测。
- 支付 provider 签名做了 EasyPay/Stripe/Airwallex 路径抽查，非密码学逐字节审计。
- 子代理结论凡标「主会话核验」者已二次读源码；其余为高置信静态分析。

---

## 2. 相对 07-14 审计的变化地图

| 07-14 问题 | 当前状态 | 证据 |
|------------|----------|------|
| P0-1 Skill `ChargeUserBalance` 无 Reference 双扣 | **已关闭** | `ai_skill_balance_ledger` + `wire.go` ledger 路径；集成测并发 16 次仅 1 次改余额 |
| P0-2 嵌入页 JWT 写 query | **已关闭** | `embedded-url.ts`；单测 `token` 断言 false；Ops WS 用 `Sec-WebSocket-Protocol` |
| P0-3 Images partial 仅 `ImageCount>0` 漏计 token | **已关闭（主流）** | Responses/WS/Messages 按 `result!=nil`；Chat/Images 计费条件已放宽但 **落穿成功路径新问题** |
| P0-4 WS prewrite ping 无 capability 门闩 | **已关闭** | `SupportsIdlePingWithoutReader`；coder 路径显式 false |
| P0-5 Outbox watermark 普通 SET 多副本回退 | **已关闭（且死代码）** | monotonic Lua；生产 `pollOutbox` 用 claim/lease，不读写 watermark |
| 双扣 / usage_billing_dedup | **加固** | 事务 claim + fingerprint；余额 fail-closed `balance >= amount` |
| OAuth Redis 单次 consume | **已落地** | `redissession.TryConsume` 多平台 |
| 支付 channel 删除防泄露 | **仍 fail-closed** | 无 channel 列表；密钥明文是独立问题 |
| Admin bulk 多平台 | **后端部分硬拦** | Credentials/Extra 混合平台 BadRequest；前端过滤模式仍有假阴性 |

**自 07-14 以来代表性加固提交方向**：`fix/api-double-billing`、Skill ledger、WS partial/preemption、scheduler outbox durable、identity/financial harden、API key mask + on-demand reveal、security signing/retention、web-search orphan strip、compact-model fallback 等。

---

## 3. 已关闭的旧 P0（保留证据，避免回归）

### ✅ P0-1 Skill 买家扣费幂等

- 表：`ai_skill_balance_ledger`，`UNIQUE (reference, operation)`
- 写入：`ON CONFLICT DO NOTHING` + 重复读回 `Duplicate`
- 接线：`ProvideAISkillBalanceCharger` → `ApplyAISkillBalanceCharge` / `Refund`
- 结算注释仍依赖 reference 幂等（`ai_skill_settlement_service.go`）

### ✅ P0-2 嵌入 JWT

- `buildEmbeddedUrl` 不再设置 `token`
- 测试：`frontend/src/utils/__tests__/embedded-url.spec.ts`

### ✅ P0-3 / Partial 漏计 token（主流路径）

- WS AfterTurn 明确「token 和/或 image」均计费
- Responses 路径 partial → RecordUsage → schedule failure → 终态错误

### ✅ P0-4 Prewrite capability

- OAuth + reused + idle + `SupportsIdlePingWithoutReader`
- coder WebSocket 实现返回 false，避免假死

### ✅ P0-5 Watermark 回退

- Lua 字符串/长度单调比较
- 业务主路径已迁移到 outbox claim token + lease

---

## 4. P0 阻塞问题（当前必须处理）

### P0-1. 支付渠道配置密钥明文写入数据库

| 字段 | 内容 |
|------|------|
| **领域** | Payment / Secrets |
| **严重度** | P0 — 商户密钥与 webhook 伪造面 |
| **置信度** | 高（主会话核验） |

**现象**

```go
// backend/internal/service/payment_config_providers.go
// encryptConfig serialises a provider config for storage.
// New records are written as plaintext JSON; the historical AES-GCM wrapping
// has been dropped but decryptConfig still accepts old ciphertext during migration.
func (s *PaymentConfigService) encryptConfig(cfg map[string]string) (string, error) {
    data, err := json.Marshal(cfg)
    // ...
    return string(data), nil
}
```

**影响**

- DB 备份、只读副本、SQL 注入级读权限、运维误导出 → Stripe/Airwallex/微信/支付宝 **webhookSecret、私钥、API key 全量泄露**
- 攻击者可伪造 webhook 入账（若再配合订单状态机漏洞）或盗用商户能力
- 与「Admin GET 脱敏」目标部分对冲：存储面已是明文

**修复建议**

1. 恢复 at-rest 加密（AES-GCM / KMS envelope），敏感字段单独加密
2. Admin API 仅返回脱敏视图；轮换所有已明文落库密钥
3. 审计历史 `payment_provider_instances.config` 行并强制 re-encrypt 迁移

---

### P0-2. 部署示例弱 JWT 可通过长度校验 + 默认管理员口令

| 字段 | 内容 |
|------|------|
| **领域** | Deploy / Auth |
| **严重度** | P0 — 运维照抄即会话可伪造 |
| **置信度** | 高（主会话核验） |

**现象**

- `deploy/config.example.yaml`：
  - `jwt.secret: "change-this-to-a-secure-random-string"`（长度 35 ≥ 32）
  - `default.admin_password: "admin123"`
- `isWeakJWTSecret` 仅枚举 `change-me-in-production` / `secret` 等，**不包含**上述 example 值

**影响**

- 照抄 example 上线 → JWT 可被猜测/已知 → 任意用户会话伪造
- 默认 admin 口令可被扫库

**修复建议**

1. example 改为空值或故意非法短串，并在注释中要求 `openssl rand -hex 32`
2. 扩展 weak 检测：子串 `change-this` / `your_` / `example`；启动 fail-closed
3. 文档与 `docker-deploy.sh` 强制生成并禁止占位

---

### P0-3. 生产 compose 默认 `BIND_HOST=0.0.0.0` 暴露管理面/API

| 字段 | 内容 |
|------|------|
| **领域** | Deploy |
| **严重度** | P0 — 公网裸奔面（依赖云安全组时降级为 P1） |
| **置信度** | 高 |

**现象**

```yaml
# deploy/docker-compose.yml
ports:
  - "${BIND_HOST:-0.0.0.0}:${SERVER_PORT:-8080}:8080"
```

`.env.example` 同样默认 `BIND_HOST=0.0.0.0`。  
对比：`docker-compose.dev.yml` 默认 `127.0.0.1`。

**影响**

- 云主机一键部署后 8080 对公网开放 Admin UI + Gateway + Webhook 端点
- 与弱密钥示例叠加时风险急剧放大

**修复建议**

1. 默认 `127.0.0.1`，文档说明公网必须经 Caddy/Nginx TLS 反代
2. install/docker-deploy 交互确认公网暴露
3. 健康检查可本地，业务端口不默认 WAN

---

## 5. P1 应修问题

### P1-1. Skill 退款补偿后同一 reference 无法再扣费（marketplace 白送风险）

| 字段 | 内容 |
|------|------|
| **领域** | Billing / Skill |
| **严重度** | P1 |
| **置信度** | 高 |

**路径**

1. `Settle` 买家扣款成功 → 创作者入账失败 → `failAfterBuyerCharge` 退款成功，status=`failed`
2. `ReplaySettlement` 允许 replay
3. 再 `ChargeUserBalance(reference=ai_skill_run:{runID})`
4. ledger 已有 charge+refund → `Refunded=true` → `ErrAISkillBalanceChargeRefunded`

生产 charger：

```go
if ledger.Refunded {
    return nil, ErrAISkillBalanceChargeRefunded
}
```

单测 stub **不实现** Refunded 语义，却期望第二次 charge 成功（`ai_skill_service_test.go` 约 977 行）——**测试与生产语义分裂**。

**修复**：replay 使用新 reference（`…:attempt:N`），或允许 refunded 后插入新 attempt；对齐单测。

---

### P1-2. Skill 扣款允许余额为负（与 usage billing fail-closed 不一致）

```sql
UPDATE users SET balance = balance - $1
WHERE id = $2 AND deleted_at IS NULL
-- 无 balance >= amount
```

对比 `usage_billing_repo.go`：`AND balance >= $1` + `ErrInsufficientBalance`。

**影响**：并发 Skill settle 可把余额打负。  
**修复**：对齐 fail-closed，或书面产品契约 + 指标告警。

---

### P1-3. 用户 API Key 明文落库

- Schema：`api_keys.key` 唯一明文（`ent/schema/api_key.go`）
- 认证：`Where(KeyEQ(key))`
- 正面：列表/详情 masked；`GET /keys/:id/value` 所有权校验 + 可选 TOTP step-up

**影响**：DB 备份 = 全站网关密钥库。  
**修复**：`key_hash` + prefix 展示；创建时仅一次返回明文；迁移双读后切哈希。

---

### P1-4. 邮箱注册邀请码消费 fail-open / 非事务

```go
// auth_service.go — 邮箱注册路径
if err := s.redeemRepo.Use(...); err != nil {
    // 邀请码标记失败不影响注册，只记录日志
}
```

OAuth 路径已是 Create+Use 同事务。  
**影响**：一码一用可被并发/瞬时 DB 失败击穿。  
**修复**：与 OAuth 对齐，Use 失败回滚用户。

---

### P1-5. 密码重置 token 非原子 consume

`Verify` 后 `Delete`；Delete 失败仅打日志。  
**影响**：并发双 POST 可能双用同一重置链接（窗口短）。  
**修复**：Redis `GETDEL` / Lua compare-and-delete。

---

### P1-6. 密码策略 min=6

`auth_handler.go` 注册/重置 `binding:"required,min=6"`；前端部分 UI 已提示 8 位。  
**修复**：统一 min≥8 + 复杂度，与前端 Profile 对齐。

---

### P1-7. Chat Completions / Images：partial 错误落穿成功路径

**证据（Chat）**：`err!=nil && result!=nil` 时仅 Warn，**无 return**，随后：

```go
h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, true, result.FirstTokenMs)
// + RecordUsage
```

Responses 正确路径：partial 计费 → `reportOpenAIAccountScheduleFailure` → `ensureForwardErrorResponse` → `return`。

| 维度 | 后果 |
|------|------|
| 计费 | 通常只一次（非双扣） |
| 调度健康 | 失败记成功 → 坏账号继续调度 |
| 客户端 | 终态错误帧可能缺失 |

**修复**：对齐 Responses 模板。

---

### P1-8. Soft rate-limit：`classifyOpenAIWSSoftRateLimitAdvisory` 恒 false

```go
func classifyOpenAIWSSoftRateLimitAdvisory(message []byte) (string, bool) {
    // codex.rate_limits → return "", false
    return "", false
}
```

热路径 `MarkBroken` / soft RL failover / temp-unsched **基本不可达**；注释/测试/persist 逻辑与热路径脱节。  
**修复**：删除 dead code 或恢复阈值+credits 条件并与 `persistOpenAIWSSoftRateLimitAdvisory` 对齐。

---

### P1-9. Proxy expiry 后 full-account 缓存未热同步

`SweepExpiredProxies`：事务内改 `proxy_id` + outbox，**commit 后不 `syncSchedulerAccountSnapshots`**（对比 auto-pause 有 post-commit sync）。  
`sched:meta` 不含 ProxyID；hydrate 读 `sched:acc:` full account。

**影响**：outbox rebuild 完成前，请求可能继续走 **已过期代理** 出网。  
**修复**：commit 后 `SetAccount` 热同步，或 hydrate 强制读 DB proxy 字段。

---

### P1-10. OAuth refresh：Redis 锁失败时无锁降级

`oauth_refresh_api.go`：`lockErr != nil` 时继续无分布式锁刷新。  
**影响**：Redis 抖动时多副本双刷；`invalid_grant` 恢复仅部分兜住。  
**修复**：短重试 / wait-for-cache；失败则 503 而非无锁双刷。

---

### P1-11. 前端 Bulk-edit 过滤模式多平台假阴性

大过滤集（`total > page`）且未选 platform 时：

```ts
const selectedPlatforms = platform ? [platform] : []
// isMixedPlatform = length > 1 → false when []
```

可提交 model mapping 到混合平台集合。  
后端 `validateBulkModelRoutingUpdatePlatforms` **仅在 Credentials/Extra 非空时**拒绝混合平台——纯其它字段 bulk 可能仍通过。

**修复**：

1. 前端：`selectedPlatforms.length !== 1` 即禁用提交  
2. 后端：filtered bulk 解析最终 ID 集后始终校验平台集合（不仅 Credentials/Extra）

---

### P1-12. 注册明文密码写入 sessionStorage

`RegisterView.vue` → `sessionStorage.register_data.password` → 邮件验证页读出。  
**影响**：XSS / 同设备 / 扩展可读明文密码。  
**修复**：仅存 email；验证页重输密码或后端 registration intent。

---

### P1-13. JWT + Refresh 存 localStorage（XSS = 会话接管）

`stores/auth.ts`：`auth_token` / `refresh_token` 均 localStorage；`api/client.ts` 每次读 Bearer。  
Admin 角色有 `adminRoleVerified` 防本地伪造（好），但 **token 窃取仍是账户接管**。

**中长期**：HttpOnly Secure cookie + 短 access + BFF；短期强化 CSP、杜绝未消毒 `v-html`。

---

### P1-14. Redis 默认空密码 + secret_scan 覆盖窄

- compose：`REDIS_PASSWORD` 默认可空  
- `tools/secret_scan.py` 不扫 example 弱 JWT、`admin123`、占位串  

**修复**：bundled Redis 强制密码；扩展 secret_scan 规则。

---

## 6. P2 改进项（摘要）

| ID | 标题 | 说明 |
|----|------|------|
| P2-1 | Skill pending reclaim 非 CAS | 2min 墙钟；双扣被 ledger 挡，但噪声/误 reclaim |
| P2-2 | legacy `DeductBalance` 允许透支 | 退款旁路；与统一 billing 策略分裂 |
| P2-3 | Creator reverse 唯一索引不全 | 仅 `creator_earning` partial unique |
| P2-4 | logredact 默认键不全 | 缺 authorization / api_key / sk- 启发式 |
| P2-5 | Reveal TOTP 非强制 + totp_code query | 仅用户已开 2FA 时要求；query 进 access log |
| P2-6 | JWT 默认 24h | 对 localStorage SPA 窗口偏大 |
| P2-7 | CustomPage iframe 无 sandbox | 对比 HomeView 有 sandbox；embed 传 user_id/src_url |
| P2-8 | OAuth callback hash token 未清理 | 历史记录残留 |
| P2-9 | Session preemption 非 Redis 时 Get+Set | 生产需 atomic claim |
| P2-10 | Grok active-delta 默认 on + force store | 客户端须显式 store=false；多副本依赖共享 state store |
| P2-11 | Outbox watermark API 死代码 | 易误导回退 cursor 模型 |
| P2-12 | 多副本 lagFailures/full rebuild herd | 进程内计数 |
| P2-13 | install.sh checksum 缺失仅 warning | 应 hard fail |
| P2-14 | trusted_proxies example 重复键 | YAML 后写覆盖前写 |
| P2-15 | apicompat refusal/未知 stop_reason 默认 completed | wire 偏差 |
| P2-16 | xlsx 审计例外至 2026-10 | Admin 导出路径；需到期复审 |
| P2-17 | docker-compose.dev 硬编码 prod 向 GCP 资源名 | 信息泄露/误连 |
| P2-18 | 支付 instance 删除后历史退款依赖 live secret | fail-closed 非白送，但运维体验差 |

---

## 7. 分域健康评估

### 7.1 计费与支付 — **B+**

| 控制 | 状态 |
|------|------|
| usage_billing_dedup 事务 claim + fingerprint | 强 |
| 统一路径余额 fail-closed | 强 |
| 支付 webhook 验签 + 金额 + CAS + lease | 强（Stripe/EasyPay/Airwallex 抽查） |
| 退款 RequestID / 状态机 | 强 |
| Skill 买家 ledger 幂等 | 强 |
| Skill 退款后 replay / 透支 | **弱** |
| 支付密钥 at-rest | **弱（明文）** |
| simple mode 跳过计费 | 产品显式；非 API 越权，防运维误配 |

### 7.2 认证与会话 — **B+**

| 控制 | 状态 |
|------|------|
| Admin 读 DB 角色 + TokenVersion | 强 |
| Refresh 哈希 + 轮转 + reuse 吊销 | 强 |
| JWT secret ≥32 + HMAC only | 强 |
| OAuth TryConsume 多副本 | 强 |
| Open redirect 净化 | 强 |
| CORS 空列表拒绝 / `*` 禁 credentials | 强 |
| API Key 明文 / 邀请 fail-open / 重置竞态 | **弱** |
| 密码 min=6 | **弱** |

### 7.3 网关与协议 — **B**

| 控制 | 状态 |
|------|------|
| Responses/Messages/WS partial + cyber 防双计 | 强 |
| 流已写出禁跨账号 switch | 强 |
| web-search orphan server_tool_use 成对剥离 | 强（最近 fix） |
| compact 自动压实安全门槛 | 强 |
| Prewrite capability | 强 |
| Chat/Images partial 落穿 | **弱** |
| Soft RL 热路径 | **弱（死代码）** |

### 7.4 调度与账号池 — **B+**

| 控制 | 状态 |
|------|------|
| Outbox claim/lease/SKIP LOCKED | 强 |
| 并发水位 Redis Lua | 强 |
| 限流/temp-unsched meta 热更新 | 强 |
| Coalesce full rebuild（进程内） | 强 |
| Proxy expiry 缓存传播 | **弱** |
| OAuth refresh Redis 故障降级 | **中** |

### 7.5 前端 — **B**

| 控制 | 状态 |
|------|------|
| DOMPurify 主路径 | 强 |
| Admin role 不信任 localStorage 伪造 | 强 |
| API key on-demand reveal | 强 |
| 支付 URL scheme 白名单 / recovery 不存 clientSecret | 强 |
| postMessage origin 校验 | 强 |
| Token localStorage / 注册密码 sessionStorage | **弱** |
| Bulk 过滤模式 | **弱** |

### 7.6 部署与供应链 — **C+**

| 控制 | 状态 |
|------|------|
| 非 root 容器 / 迁移 advisory lock + checksum | 强 |
| CORS / trusted_proxies 默认 fail-closed（运行时） | 强 |
| skill-runner 沙箱示例 | 强 |
| govulncheck 钉版本 CI | 中上 |
| example 弱密钥 / 0.0.0.0 / Redis 空密 | **弱** |
| secret_scan 覆盖 | **窄** |
| install checksum 软失败 | **弱** |

---

## 8. 优先修复路线图

### 立即（发布前 / 本周）

1. **P0-1** 支付配置 at-rest 加密 + 密钥轮换  
2. **P0-2** 净化 config.example + 扩展 weak JWT 检测  
3. **P0-3** compose 默认绑定 localhost  
4. **P1-7** Chat/Images partial 对齐 Responses  
5. **P1-9** Proxy expiry post-commit 热同步  
6. **P1-1 / P1-2** Skill replay reference + 余额守卫  

### 短期（1–2 周）

7. **P1-3** API Key hash-at-rest 迁移方案  
8. **P1-4 / P1-5 / P1-6** 邀请事务化、重置原子、密码策略  
9. **P1-8** Soft RL 语义落地或删死代码  
10. **P1-11 / P1-12** Bulk 前后端 fail-closed + 去掉 sessionStorage 密码  
11. **P1-10 / P1-14** OAuth refresh 失败策略 + Redis 强制密码  

### 中期

12. JWT/Refresh 迁出 localStorage（HttpOnly）  
13. logredact / secret_scan 扩展  
14. 清理 watermark 死 API、lag 跨副本聚合  
15. iframe sandbox、OAuth hash 清理、Grok multi-replica 文档  
16. 镜像 pin digest + 容器 CVE 扫描  

---

## 9. 测试与验证建议（修复后）

| 场景 | 建议命令/方法 |
|------|----------------|
| Skill 双扣回归 | `go test -tags=unit ./internal/service/ -run Skill` + ledger 集成测；补「charge→refund→replay」断言 |
| 支付 webhook | provider 单测 + 伪造签名拒绝 + 金额篡改拒绝 |
| Chat/Images partial | 单测：`err!=nil && result!=nil` → schedule failure + 非 2xx 终态 |
| Bulk mixed platform | 前端 spec + 后端 filtered bulk 集成 |
| Proxy expiry | 双实例：sweep 后立即 hydrate 断言 proxy_id |
| 并发水位 | 既有 Lua 测 + `-count=30` 压 refresh/claim |
| 前端 | `pnpm --dir frontend run test:run` + `typecheck` |
| 全量后端 | `cd backend && go test -tags=unit ./...` |

---

## 10. 正面控制清单（保持）

1. **统一 usage billing 事务**：dedup claim + 余额/配额同事务 fail-closed  
2. **Skill 买家 ledger UNIQUE(reference, operation)**  
3. **支付 webhook**：验签 → 金额 → 状态 CAS → fulfillment lease  
4. **Admin 鉴权读 DB 角色** + TokenVersion 吊销  
5. **Refresh token 哈希存储 + family 轮转 + reuse 检测**  
6. **OAuth 多副本 TryConsume** + state 校验顺序正确  
7. **CORS / TrustedProxies / URL allowlist 默认安全**  
8. **API Key 列表脱敏 + Reveal 所有权校验 + 网关禁 query key**  
9. **流式已写出禁止跨账号 failover**（防双计/双帧）  
10. **web-search 与 orphan server_tool_use 成对剥离**  
11. **Outbox claim/lease + SKIP LOCKED**  
12. **并发水位 Redis Lua + 心跳续租**  
13. **迁移 advisory lock + checksum 防篡改**  
14. **非 root 容器 + skill-runner 沙箱示例**  
15. **前端 DOMPurify + open redirect 消毒 + adminRoleVerified**  
16. **支付 recovery 不持久化 clientSecret**  

---

## 11. 附录 A：关键文件索引

| 主题 | 路径 |
|------|------|
| Skill 结算 | `backend/internal/service/ai_skill_settlement_service.go` |
| Skill ledger | `backend/internal/repository/ai_skill_balance_ledger_repo.go` |
| Skill charger 接线 | `backend/internal/service/wire.go` |
| Usage billing | `backend/internal/repository/usage_billing_repo.go` |
| 支付配置加密 | `backend/internal/service/payment_config_providers.go` |
| 支付 webhook | `backend/internal/handler/payment_webhook_handler.go` |
| Chat partial | `backend/internal/handler/openai_chat_completions.go` |
| Images partial | `backend/internal/handler/openai_images.go` |
| Responses 正确 partial | `backend/internal/handler/openai_gateway_handler.go` |
| WS soft RL | `backend/internal/service/openai_ws_forwarder.go` |
| Prewrite ping | 同上 `shouldOpenAIWSSessionPrewritePing` |
| Scheduler outbox | `backend/internal/service/scheduler_snapshot_service.go` |
| Scheduler cache | `backend/internal/repository/scheduler_cache.go` |
| Proxy expiry | `backend/internal/repository/proxy_repo.go` |
| Bulk platform | `backend/internal/service/admin_service.go` `validateBulkModelRoutingUpdatePlatforms` |
| Embed URL | `frontend/src/utils/embedded-url.ts` |
| Auth store | `frontend/src/stores/auth.ts` |
| Bulk UI | `frontend/src/views/admin/AccountsView.vue` / `BulkEditAccountModal.vue` |
| 配置 example | `deploy/config.example.yaml` |
| Compose | `deploy/docker-compose.yml` |
| 安全策略 | `SECURITY.md` |

---

## 12. 附录 B：子代理与主会话分工记录

| 角色 | 产出 |
|------|------|
| Billing explore | 8 findings + 已修复对照 |
| Auth explore | 11 findings + 正面控制 12 项 |
| Gateway explore | Chat/Images P0 级行为问题 + soft RL + 协议偏差 |
| Scheduler explore | watermark 死代码澄清 + proxy P0 级缓存缺口 |
| Frontend explore | localStorage/bulk/sessionStorage 等高风险 |
| Deploy explore | 默认面 P0 三项 |
| 主会话 | 交叉核验上表；去重合并；撰写本报告 |

---

## 13. 结论

Sub2API 在 **计费幂等、网关防双计主路径、OAuth 多副本、调度 claim、CORS/配置校验** 上已具备可生产的骨架；07-14 报告中的资金/会话类 P0 多数已在 HEAD 关闭。  

当前发布风险重心已从「会不会双扣/JWT 进 URL」转向：

1. **密钥与支付配置的存储形态**（明文）  
2. **部署默认是否安全**（example/compose）  
3. **边缘路径一致性**（Chat/Images 调度信号、Skill 退款 replay、proxy 缓存）  
4. **前端凭证持有模型**（localStorage / sessionStorage）  

完成 §8「立即」清单后，可重新评估对外 SaaS 基线；完成「短期」清单后，多副本生产信心显著提高。

---

*报告生成：2026-07-16 · 只读审计 · 不修改业务代码 · HEAD `eb64a5c16`*
