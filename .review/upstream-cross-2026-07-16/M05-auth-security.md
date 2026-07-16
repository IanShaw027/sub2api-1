# Module 5: 鉴权安全密钥与治理

## 范围摘要
- 对比基线: `BASE=da85cc7e` → `HEAD=eb64a5c1`；上游对照 `UPSTREAM=b960ec19`（`upstream/main`）
- 本模块审查路径: middleware 鉴权链、JWT/API Key/Admin Key、OAuth pending session、TOTP 2FA、IP 多账号短窗口封禁、API Key 掩码/Reveal、content moderation auto-ban CAS、合规确认、密钥/审计相关日志
- 相对上游的主要能力差异（本地增强）:
  - API Key List/Detail 强制掩码 + 独立 `GET /keys/:id/value` on-demand Reveal
  - 短窗口多账号 IP 检测/封禁（`IPSecurityService` + 中间件 `IPSecurityBlock`）
  - OAuth pending 会话 + browser session binding + completion code 哈希
  - TOTP 2FA（密码登录、OAuth bind-login 路径）
  - Admin compliance 硬门禁（HTTP 423）
  - Content moderation auto-ban 的 status-only CAS（`DisableUserForContentModeration`）
  - Refresh token 轮转 + TokenVersion 吊销
  - 删除 API Key 写入 `deleted_api_key_audits` 审计

## 关键路径图
```
[Client HTTP]
    │
    ├─ IPSecurityBlock (http.go 最外层) ──► IsBlocked(in-memory fallback)
    │
    ├─ /v1/*  APIKeyAuth ──► GetByKey(auth cache) → user/group/IP ACL → billing enforcement → Gateway
    │
    ├─ /api/v1/* JWTAuth ──► ValidateToken(HMAC) → DB user → TokenVersion → handlers
    │     ├─ /keys  List/Get 掩码
    │     ├─ /keys/:id/value  Reveal 明文（仅 JWT + 所有权）
    │     └─ /user/totp/*  setup/enable/disable
    │
    ├─ /api/v1/auth/*
    │     ├─ login ──► password → (Totp? temp_token : TokenPair)
    │     ├─ login/2fa ──► temp_token + TOTP → TokenPair
    │     └─ oauth/* ──► pending session cookies → /oauth/pending/exchange → TokenPair(?)
    │
    └─ /api/v1/admin/* AdminAuth(JWT|x-api-key) → AdminComplianceGuard → admin handlers
              └─ content moderation auto-ban CAS → users.status=disabled
```

## 发现清单

### [P0] OAuth 已绑定身份登录可完全绕过 TOTP 2FA
- **位置**: `backend/internal/handler/auth_oauth_pending_flow.go:353-368`、`2162-2262`；对照 `auth_handler.go:287-301`、`auth_oauth_pending_flow.go:1867-1885`
- **相对上游**: 本地新增/强化 TOTP 后出现的治理缺口（密码登录与 OAuth bind-login 有 2FA，已绑定 OAuth 登录 exchange 无 2FA）
- **问题**:  
  1. 密码登录路径在 `user.TotpEnabled` 时只返回 `requires_2fa` + `temp_token`，不发 TokenPair。  
  2. OAuth **绑定已有账户**（`bindPendingOAuthLogin`）同样检查 TOTP。  
  3. 但已绑定第三方身份的常规 OAuth 登录：callback 创建 `intent=login` + `TargetUserID` 的 pending session，前端 `POST /auth/oauth/pending/exchange` 走 `pendingOAuthCompletionCanIssueTokenPair` → **直接 `GenerateTokenPair`**，全程不读 `TotpEnabled`。  
  测试 `TestExchangePendingOAuthCompletionExistingLoginWithSuggestedProfileSkipsAdoptionPrompt` 也固定期望直接返回 `access_token`，未覆盖 2FA。
- **影响**: 开启 2FA 的用户只要 OAuth 身份仍绑定（或攻击者控制对应 OAuth 账号），即可绕过第二因子拿到 access/refresh token。**2FA 对 OAuth 主登录路径失效**，属明确认证绕过。
- **证据**:
  ```go
  // pendingOAuthCompletionCanIssueTokenPair: 仅检查 intent/target/step，无 TOTP
  func pendingOAuthCompletionCanIssueTokenPair(...) bool {
      if !strings.EqualFold(session.Intent, oauthIntentLogin) { return false }
      if session.TargetUserID == nil || *session.TargetUserID <= 0 { return false }
      ...
      return strings.TrimSpace(pendingSessionStringValue(payload, "step")) == ""
  }
  // ExchangePendingOAuthCompletion:
  if canIssueTokenPair {
      tokenPair, err := h.authService.GenerateTokenPair(...) // 无 TotpEnabled 检查
  }
  ```
- **建议**:  
  - exchange 发 token 前统一调用与密码登录相同的 2FA 门闩；若启用则返回 `requires_2fa` + temp session（可复用 `CreateLoginSession`/`CreatePendingOAuthBindLoginSession`）。  
  - 增加回归测试：`TotpEnabled=true` 的 existing-identity OAuth login 不得直接返回 token。  
  - 审计所有 OAuth provider（linuxdo/wechat/oidc/dingtalk/email）的「已有身份登录」汇合点，确保只经过该门闩。
- **交叉关注**: M09 前端 OAuth callback/exchange 需同步处理 `requires_2fa`；M07 若 AI 会话依赖同一登录态亦受影响。

### [P0] 多实例下 IP 多账号封禁 `IsBlocked` 不读共享状态，封禁可被旁路
- **位置**: `backend/internal/service/ip_security.go:213-232`、`285-303`、`497-537`；入口 `backend/internal/server/http.go` 的 `IPSecurityBlock`
- **相对上游**: 本地新增能力，实现不完整
- **问题**:  
  - `IsBlocked` 在 `stateLoaded==true` 时**只查本进程** `fallback`/`whitelist` map；未命中即 `return false`。  
  - `Observe` 创建 ban 时会 `SAdd ipsec:{state}:banned`，但 **`IsBlocked` 从未 `SIsMember` 读该 Redis 集合**。  
  - 实例 A 触发封禁后，实例 B/C（已 WarmCache、`stateLoaded=true`）在重启或本地再次 CreateBan 前**不会拦截**该 IP。  
  - 多副本部署时，攻击者可对未持有 ban 状态的实例继续登录/打 API。
- **影响**: 短窗口多账号治理在水平扩展下失效；与「最外层硬拦截」的设计意图相反。属生产可用性/安全控制失效。
- **证据**:
  ```go
  func (s *IPSecurityService) IsBlocked(...) bool {
      ...
      if _, ok := s.fallback[ip]; ok { return true }
      loaded := s.stateLoaded
      ...
      if loaded { return false } // 其它实例写入的 Redis ban 被忽略
      return s.fallbackBlocked(ctx, ip)
  }
  ```
- **建议**:  
  - `IsBlocked` 热路径：本地 miss 后查 Redis `SISMEMBER ipsec:{state}:banned`（带现有 circuit breaker），命中再写回本地。  
  - 或订阅/轮询 ban 变更；CreateBan 后发布 pub/sub 使其它实例 invalidate。  
  - 集成测试：双 service 实例，A ban → B IsBlocked 必须 true。
- **交叉关注**: M06 多实例调度；M10 部署副本数 >1 时风险放大。

### [P1] Refresh Token 复用检测未吊销 Token Family（轮转安全不完整）
- **位置**: `backend/internal/service/auth_service.go:1633-1707`；`backend/internal/repository/refresh_token_cache.go`
- **相对上游**: 本地/共享轮转实现偏离最佳实践
- **问题**: `RefreshTokenPair` 在 token 不存在时仅日志 `"possible reuse attack"` 并返回 `ErrRefreshTokenInvalid`，**不会**根据 family 撤销仍存活的后继 refresh/access 会话。标准 refresh rotation 在检测到 reuse 时应 `DeleteTokenFamily`。当前攻击者若曾窃取旧 refresh，在合法用户轮转后重放只会失败，但若存在竞态窗口（两请求同时 Get 同一 token 再各自 Delete/Generate），可能分叉出两个有效 family 成员且无 reuse 告警吊销。
- **影响**: 会话劫持后的 blast radius 不能「一次 reuse 废全家」；与代码注释中的 reuse attack 语义不符。
- **证据**:
  ```go
  if errors.Is(err, ErrRefreshTokenNotFound) {
      logger.LegacyPrintf(..., "possible reuse attack")
      return nil, ErrRefreshTokenInvalid // 未 DeleteTokenFamily
  }
  ```
- **建议**:  
  - 维护 `family → last_token_hash` 或 `used_token` 标记；reuse 时 `DeleteTokenFamily` + bump `TokenVersion`（可选）。  
  - Get+Delete 使用 Redis Lua 原子化，消除双发竞态。  
  - 增加 reuse 集成测试。
- **交叉关注**: M09 前端 refresh 并发；M10 会话运维。

### [P1] API Key 明文落库 + 删除审计表保留完整明文
- **位置**: `backend/internal/service/api_key_service.go:458-461`；`backend/internal/repository/api_key_repo.go:381-395`（`deleted_api_key_audits` INSERT `key` 列）
- **相对上游**: 上游常见模式，本地审计增强反而扩大明文面
- **问题**: 活跃 key 以明文存 `api_keys.key`；软删除时把**完整明文**写入 `deleted_api_key_audits`。DB 备份、只读副本、SQL 注入、运维导出均可直接复用历史 key（审计保留期内）。Auth cache 用 SHA256 作缓存键，但权威存储仍是明文。
- **影响**: 密钥材料在静态存储层无哈希保护；删除后仍可从审计表还原完整密钥。
- **证据**:
  ```sql
  INSERT INTO deleted_api_key_audits (key, api_key_id, user_id, key_name, deleted_at)
  SELECT key, id, user_id, name, NOW() FROM api_keys WHERE id=$1 ...
  ```
- **建议**:  
  - 中长期：存 `key_hash`（pepper+HMAC/SHA256）+ 可展示前缀；认证只比对哈希。  
  - 审计表改为 hash/prefix，禁止存完整明文。  
  - 过渡期对 `deleted_api_key_audits` 加密 at-rest 并缩短 retention（`audit_retention_service` 已有清理，确认生产配置）。
- **交叉关注**: M08 schema/migration；M10 备份与 secret scan。

### [P1] API Key Reveal 无 step-up（仅 JWT 所有权）
- **位置**: `backend/internal/handler/api_key_handler.go:142-179`；路由 `backend/internal/server/routes/user.go:67`
- **相对上游**: 本地新增 on-demand reveal（相对「列表永不返回明文」是进步），但缺二次验证
- **问题**: `Reveal` 只校验 JWT 主体与 `key.UserID` 一致，无密码/TOTP/近期 re-auth。XSS、被盗 access token、共享浏览器会话可直接拉全量 key。虽有 `api_key.reveal` 审计日志与 `Cache-Control: no-store`，不能阻止在线滥用。
- **影响**: 掩码治理被「任意已登录会话一发 GET」抵消；高价值密钥面暴露。
- **证据**:
  ```go
  func (h *APIKeyHandler) Reveal(c *gin.Context) {
      subject, ok := middleware2.GetAuthSubjectFromContext(c)
      ...
      if key.UserID != subject.UserID { response.NotFound(...); return }
      response.Success(c, gin.H{"key": key.Key}) // 无 password/totp
  }
  ```
- **建议**: 启用 TOTP 的用户 Reveal 强制 TOTP；否则要求密码确认或短时 step-up token；可选 IP/设备绑定与速率限制。
- **交叉关注**: M09 前端 Reveal UX；与 P0 OAuth 2FA 绕过叠加时风险更高。

### [P2] Admin API Key 一律映射为 `GetFirstAdmin`，审计与合规主体错误
- **位置**: `backend/internal/server/middleware/admin_auth.go:118-150`；合规 `admin_compliance.go:85-90`
- **相对上游**: 共享/本地均有 Admin API Key 模式
- **问题**: 任意有效 `x-api-key` 将 `ContextKeyUser` 设为**第一个** admin 用户。多管理员部署时：操作审计、compliance ack（按 `adminUserID` 分 key）全部记到 first admin；无法区分真实操作者；first admin 被禁用时行为依赖 `GetFirstAdmin` 实现。
- **影响**: 审计归因失败、合规确认语义漂移；事件响应无法追责。
- **建议**: Admin API Key 绑定创建者/服务账号实体；或独立 `system` 主体 + 强制操作审计字段 `auth_method=admin_api_key`。
- **交叉关注**: M10 运维密钥轮换；M09 管理端显示。

### [P2] Admin API Key 使用 `ConstantTimeCompare` 但长度不等时提前返回（次要时序）
- **位置**: `backend/internal/server/middleware/admin_auth.go:132`
- **相对上游**: 实现细节
- **问题**: Go `subtle.ConstantTimeCompare` 在长度不等时立即返回 0，存在长度侧信道。Admin key 固定前缀+64 hex，实际利用难度低。
- **影响**: 低；理论上可探测长度。
- **建议**: 先 hash 两边再比较，或固定长度 pad 后比较。
- **交叉关注**: 无。

### [P2] 密码策略过弱（min=6）且登录依赖 Turnstile 可配置关闭
- **位置**: `backend/internal/handler/auth_handler.go:60`、`677`；`auth_oauth_pending_flow.go:71`
- **相对上游**: 共享弱默认
- **问题**: 注册/改密/OAuth 建号均 `min=6`，无复杂度要求。暴力破解主要靠 Turnstile/路由限流；Turnstile 关闭时仅靠 `auth_rate_limit`。
- **影响**: 弱口令账户易被撞库；与 2FA 组合时仍影响未开 2FA 用户。
- **建议**: 提高最小长度与复杂度；失败登录指数退避；强制管理员 2FA。
- **交叉关注**: M09 注册表单校验。

### [P2] TOTP 验证失败限流依赖缓存；缓存错误时 fail-open
- **位置**: `backend/internal/service/totp_service.go:339-343`、`393-396`
- **相对上游**: 本地 TOTP 实现
- **问题**: `GetVerifyAttempts` 出错时不拦截；`IncrementVerifyAttempts` 错误被忽略。Redis 故障窗口可无限试 TOTP（6 位 + 时间窗仍有成本，但放大）。
- **影响**: 降级时 2FA 抗暴力能力下降。
- **建议**: 缓存错误 fail-closed 或回落 DB/内存限流；验证成功再删 session 已有，失败计数必须可靠。
- **交叉关注**: M10 Redis 高可用。

### [P2] Content moderation 兼容路径 Update 全量用户对象（非 CAS）
- **位置**: `backend/internal/service/content_moderation.go:2360-2383`；生产 CAS: `repository/user_repo.go:398-426`
- **相对上游**: 本地 CAS 为主，fallback 保留
- **问题**: 生产 `DisableUserForContentModeration` 为 status-only CAS，正确。fallback `userRepo.Update` 可能覆盖并发字段（余额/profile）。若测试桩/错误装配走到 fallback，有丢更新风险。
- **影响**: 正常 Wire 装配风险低；错误装配时中等。
- **建议**: 生产路径强制要求 `contentModerationUserDisabler`，否则 fail closed 并 metric；删除或隔离 fallback。
- **交叉关注**: M04 余额一致性；M08 用户状态字段。

### [P3] TOTP Debug 日志打印 secret 前缀
- **位置**: `backend/internal/service/totp_service.go:239-246`、`375-391`；`auth_handler.go:336-356`
- **相对上游**: 本地调试残留
- **问题**: `slog.Debug` 输出 `secret_prefix`、email 等。生产若开 debug 级别构成敏感信息泄露。
- **影响**: 配置错误时泄露 2FA 材料片段。
- **建议**: 删除 secret 相关 debug 字段；email 用 mask。
- **交叉关注**: M10 日志采集。

### [P3] API Key 掩码保留前后 4 字符
- **位置**: `backend/internal/handler/dto/mappers.go:152-161`
- **相对上游**: 常见 UX 折中
- **问题**: `sk-ab***xy12` 风格便于识别，也便于与日志前缀关联枚举。与 ops 前 8 字符前缀策略叠加时信息更多。
- **影响**: 低；需配合存储哈希改造。
- **建议**: 仅显示固定前缀（如 `sk-` + 4）不回显后缀；ops 仅存 hash 前缀。
- **交叉关注**: M10 Ops 日志。

## 与上游合并风险
- **冲突热点文件**:
  - `backend/internal/server/middleware/{api_key_auth,jwt_auth,admin_auth,ip_security}.go`
  - `backend/internal/handler/{auth_handler,auth_oauth_pending_flow,api_key_handler}.go`
  - `backend/internal/service/{auth_service,ip_security,content_moderation,api_key_service,totp_service}.go`
  - `backend/internal/server/routes/{user,admin}.go`
  - `SECURITY.md`（本地有 operator hardening 段落）
- **语义漂移点**:
  - 「列表 API Key」：本地强制 Mask；上游若仍返回明文，合并时前端/契约测试会冲突。
  - 「OAuth 登录完成」：本地 pending session + exchange；上游可能仍 fragment/token 直出。
  - 「2FA 覆盖面」：密码有、OAuth 主登录无 → 文档若写「全站 2FA」则与实现不符。
  - Admin compliance HTTP 423：上游可能无此门闩。
  - IP multi-account ban：纯本地能力，合并上游需整包带上 schema/redis key。

## 测试与验证缺口
- **缺失**: OAuth existing-identity login + `TotpEnabled=true` 不得直接发 token（P0）。
- **缺失**: 双实例 IP ban 传播 / `IsBlocked` 读 Redis（P0）。
- **缺失**: Refresh token reuse → family 吊销（P1）。
- **不足**: Reveal step-up、Admin API Key 归因、TOTP 限流 fail-open。
- **已有较好覆盖**: pending OAuth browser mismatch、bind-login 2FA、Login2FA TokenVersion、moderation CAS 单测/集成、API Key 所有权 NotFound、auth rate limit。

## 模块结论
- **整体风险评级: Critical**
- **是否建议合入上游 / 继续分叉 / 先修再合**: **先修再合**。掩码 Reveal、pending session、moderation CAS、compliance 等方向正确且可合，但 **OAuth 2FA 绕过** 与 **多实例 IP 封禁失效** 必须在对外宣称「安全治理能力」前修复；否则分叉特性会放大虚假安全感。
- **Top 3 必须处理项**:
  1. **(P0)** OAuth `pending/exchange` 已绑定登录强制 TOTP，与密码登录对齐  
  2. **(P0)** `IPSecurityService.IsBlocked` 读取共享 Redis ban 状态，修复多实例旁路  
  3. **(P1)** Refresh reuse 吊销 family + API Key 明文/审计收敛（哈希存储或至少审计不落明文）+ Reveal step-up
