# personal-dev × upstream/main 十模块交叉 Review 综合报告

**日期**: 2026-07-16  
**对比**: `HEAD=eb64a5c1` (personal-dev) vs `upstream/main=b960ec19` (Wei-Shaw/sub2api)  
**Merge-base**: `da85cc7e`  
**规模**: ~2433 files / +693k −120k lines；本地相对 merge-base ~1119 commits，上游 ~214 commits（分叉严重）  
**方法**: 10 个独立只读子代理按模块深度审查 + 主代理对 P0 证据抽检 + 跨模块接缝合成  

产物目录: `.review/upstream-cross-2026-07-16/`

| 模块 | 报告 | 风险 | 发现 |
|------|------|------|------|
| M01 Gateway OpenAI | [M01-gateway-openai.md](./M01-gateway-openai.md) | High | P0×2 P1×3 P2×3 P3×2 |
| M02 多平台适配 | [M02-platforms.md](./M02-platforms.md) | High | P1×4 P2×6 P3×2 |
| M03 apicompat | [M03-apicompat.md](./M03-apicompat.md) | High | P1×4 P2×5 P3×2 |
| M04 计费支付 | [M04-billing-payment.md](./M04-billing-payment.md) | High | P0×1 P1×4 P2×5 P3×2 |
| M05 鉴权安全 | [M05-auth-security.md](./M05-auth-security.md) | **Critical** | P0×2 P1×3 P2×5 P3×2 |
| M06 调度账户 | [M06-scheduler-accounts.md](./M06-scheduler-accounts.md) | High | P1×6 P2×5 P3×2 |
| M07 AI Skills | [M07-ai-skills.md](./M07-ai-skills.md) | **Critical** | P0×2 P1×4 P2×3 P3×2 |
| M08 Schema/迁移 | [M08-schema-migrations.md](./M08-schema-migrations.md) | High | P0×2 P1×4 P2×3 P3×2 |
| M09 Frontend | [M09-frontend.md](./M09-frontend.md) | High | P1×4 P2×5 P3×3 |
| M10 部署 CI | [M10-deploy-ci.md](./M10-deploy-ci.md) | High | P0×2 P1×5 P2×5 P3×2 |

**合计（去重前）**: 约 **P0×11 · P1×41 · P2×45 · P3×21 ≈ 118** 条；下文交叉簇合并后为可执行优先级清单。

---

## 一、总判断

1. **不要整包合入 upstream/main。** 分叉深度（AI Studio/Skills、Grok/Kiro、发票/工单、TLS 指纹、IP 多账号、自定义迁移 runner）已远超 cherry-pick 友好区间。
2. **当前分支生产可用性的主风险不在「缺功能」，而在「终态一致性」**：
   - 协议半开流 × partial 计费 × failover 调度成功误报
   - 鉴权 2FA / 多实例封禁旁路
   - Skills 控制面注入与 test 免计费
   - 迁移编号碰撞 + 热表启动 DDL + deploy 回滚三方错位
3. **建议策略**: **先修再合**（安全/计费/协议 P0）→ **模块化边界继续分叉**（Skills/Grok/Kiro/发票）→ **可回馈上游的 hardened 补丁单独 PR**（outbox claim、bulk 跨平台拦截、secret scan、部分 gateway 终态守卫）。

---

## 二、跨模块交叉簇（比单模块更重要）

### 簇 A — 计费 × 协议终态 × Failover（M01 × M02 × M03 × M04）

| ID | 级别 | 交叉点 | 证据锚点 | 影响 |
|----|------|--------|----------|------|
| A1 | **P0** | OpenAI Messages bridge：partial 错误 fall-through 仍 `ReportOpenAIAccountScheduleResult(true)` + `RecordUsage` | `openai_gateway_handler.go` ~1209–1324 | 失败当成功调度；半失败扣费；与 Responses 路径不一致 |
| A2 | **P0** | HTTP 流式 partial：计费后 return，部分路径未保证 `response.failed`/error committed | M01；`gateway_handler_*.go` partial 分支 | 客户端盲重连 → 叠加扣费 |
| A3 | **P0** | Anthropic/Claude 主路径 cyber 与 partial `RecordUsage` 双扣；OpenAI 已有 `GetOpsCyberPolicy` 守卫 | M04 vs M01 OpenAI 守卫 | 同 turn 双扣费（request_id 不同时 dedup 失效） |
| A4 | **P1** | Kiro native web continuation：已写流仍 `UpstreamFailoverError`，handler 早退跳过 partial 计费 | M02 | **漏计费**（与 A1 方向相反，同属终态契约破裂） |
| A5 | **P1** | WS v2 passthrough 失败 `AfterTurn(nil)` 丢 usage；compact failover gin 状态跨账号粘连 | M01 | 漏计费 + 错误 compact 模型/能力 |
| A6 | **P1** | apicompat：CC usage 丢 `cache_creation`；stream 终态 reasoning item ID 重生成 | M03 | 计费偏差 + 多轮 tool 失忆 |

**统一修复原则**:
- 定义单一 **TurnOutcome** 状态机：`{partial_usage, client_terminal_written, schedule_success, failoverable}`
- 所有平台 handler 共用：`if client_written && !terminal → write terminal; bill partial once; schedule=success only if !err || committed_partial_ok`
- Cyber 与 partial 互斥：Claude/Gemini 对齐 OpenAI 的 `GetOpsCyberPolicy` 早退

### 簇 B — 鉴权 × 前端信任边界（M05 × M09）

| ID | 级别 | 交叉点 | 证据锚点 | 影响 |
|----|------|--------|----------|------|
| B1 | **P0** | `ExchangePendingOAuthCompletion` 在 `canIssueTokenPair` 时直接 `GenerateTokenPair`，**无 TOTP**；而 `bindPendingOAuthLogin` 有 TOTP | `auth_oauth_pending_flow.go` ~1867 vs ~2251 | OAuth 登录绕过 2FA |
| B2 | **P0** | `IPSecurityService.IsBlocked` 在 `stateLoaded` 时只查本机 map，热路径不读 Redis ban | `ip_security.go` ~213–233, ~289–302 | 多实例封禁可被旁路 |
| B3 | **P1** | API Key reveal 无 step-up；前端支付 `clientSecret` 进 localStorage；`pay_url` 无协议白名单 | M05 + M09 | XSS/共享设备/开放重定向放大密钥面 |
| B4 | **P1** | Skills `owned/editable` 与 admin role 可被 `localStorage.auth_user` 抬权（UI） | M09 | 需后端强兜底；前端不得合成授权 |

### 簇 C — Skills 平台成本与沙箱（M07 × M04 × M10）

| ID | 级别 | 交叉点 | 影响 |
|----|------|--------|------|
| C1 | **P0** | Test 模式可不绑 BillingAPIKey 打真实上游 | 平台配额/成本白嫖 |
| C2 | **P0** | Run parameters 可注入 `tools`/`functions` 等控制面 | 越权扩展模型能力 + 放大计费 |
| C3 | **P1** | Script timeout 无上限；媒体 URL best-effort 回退削弱 SSRF 边界 | DoS / 出站 |
| C4 | **P1** | skill-runner 部署示例 hardened，但应用层仍可「轻量 Docker」路径 | 公网开放前必须强制 hardened runner |

### 簇 D — 调度 × Outbox × 多实例（M06 × M02 × M01）

| ID | 级别 | 交叉点 | 影响 |
|----|------|--------|------|
| D1 | **P1** | Outbox lag 把已 claim 算 pending + claim 清 dedup 不恢复 | 误触发 full rebuild / 重复逻辑事件 |
| D2 | **P1** | LoadFactor vs Concurrency 分母分裂；`PreferSoonestReset` 死开关 | 调度偏差 |
| D3 | **P1** | WaitPlan 提前 RegisterSession 失败不释放 | max_sessions 占坑 |
| D4 | **P1** | 调度 Redis 缓存明文 `api_key`；Grok refresh skew=1h；Antigravity capacity cooldown 仅内存 | 密钥面 + 刷新风暴 + 多副本雪崩 |

### 簇 E — 迁移 × 部署回滚（M08 × M10）

| ID | 级别 | 交叉点 | 证据 | 影响 |
|----|------|--------|------|------|
| E1 | **P0** | 迁移前缀大规模碰撞（同号多文件常态，如 101×4、162×4），字典序调度 | `backend/migrations/` 实测 62 个重复前缀 | 与上游 merge **无文本冲突却双执行/漏执行** |
| E2 | **P0** | 启动热表事务 DDL/DML（207 usage_logs 聚合+索引、213/191 全表 UPDATE、198 钱包 ALTER TYPE） | M08 | 升级锁死全站 |
| E3 | **P0** | `deploy.sh` 失败回滚：restore checksum + 旧二进制，**不回滚已 Apply 的 DDL** | M10 | schema 新 / binary 旧 / checksum 被 restore → 三方错位 |
| E4 | **P1** | local compose URL allowlist 默认关 + 0.0.0.0；compose.dev 硬编码 prod GCP | M10 | 误部署暴露面 |

---

## 三、主代理证据抽检结论

| 发现 | 抽检结果 | 说明 |
|------|----------|------|
| A1 Messages partial → schedule success | **确认** | `err!=nil && result!=nil` 非 cyber 仅 Warn，fall-through 到 `Report...(true)` + `RecordUsage` |
| B1 OAuth exchange 绕过 TOTP | **确认** | bind 路径有 TOTP；exchange 发 token 路径无 TOTP 分支 |
| B2 IsBlocked 多实例 | **确认** | `stateLoaded==true` 时直接 `return false`（除非本机 fallback）；ban 写入本机 map+Redis，他实例热路径不读 Redis set |
| E1 迁移前缀碰撞 | **确认** | 101/162 各 4 文件；重复前缀约 62 组 |
| C1 Skills test 免 key | **部分确认** | `buildExecutionRequest` 仅 `use` 强制 API key；test 可无 `BillingAPIKey` 继续。是否真实打上游取决于 runtime 分支——按 M07 报告应在 runtime 再加硬拦 |
| A3 Claude cyber 双扣 | **可信，待单测钉死** | OpenAI 路径明确 cyber early-return；gateway Claude partial 无对等守卫（grep 可见） |

> 编排原则：不盲信 agent 分级；上表为宿主机代码/迁移树抽检。未抽检项仍具排查价值，落地前应补回归测试。

---

## 四、必须先处理的 Top 12（跨模块排序）

1. **[B1/P0]** OAuth `pending/exchange` 发 token 强制 TOTP（对齐 bind/密码登录）
2. **[B2/P0]** `IsBlocked` 热路径读 Redis ban / 订阅失效广播
3. **[A1+A2/P0]** OpenAI/Claude 流式 partial：协议终态 + schedule 结果 + 单次计费契约
4. **[A3/P0]** Claude/Gemini cyber 与 partial 互斥，对齐 OpenAI
5. **[C1+C2/P0]** Skills：test 禁止真实上游或强制计费；parameters 白名单剥离 tools/functions
6. **[E1/P0]** CI 强制迁移前缀唯一；既有碰撞建 rename/manifest 策略
7. **[E2/P0]** 热表迁移改 notx/批处理/运维窗口；禁止启动事务内 usage_logs 全表写
8. **[E3/P0]** deploy 回滚语义：禁止「DDL 已前进 + checksum restore」；失败即停并人工 runbook
9. **[A4/P1]** Kiro continuation 已写流 → 非 failover + partial bill
10. **[D1/P1]** Outbox lag/dedup claim 语义修复
11. **[B3/P1]** pay_url 白名单 + 去掉 localStorage clientSecret + reveal step-up
12. **[A6/P1]** apicompat stream item ID + cache_creation + custom_tool 请求历史

---

## 五、分模块一句话结论

| 模块 | 结论 |
|------|------|
| M01 | 协议状态机双实现（HTTP/WS）+ partial 路径是最大生产雷区；先统一 TurnOutcome |
| M02 | Kiro/Grok 为本地主分叉；计费漏记与多实例 cooldown/刷新策略优先 |
| M03 | 兼容层中心节点；双跳链信息损失需可观测；修 ID/cache/custom 再合 |
| M04 | 资金路径 High；双扣/透支/发票互斥/配额 flusher 需产品+工程决策 |
| M05 | **Critical**；2FA 旁路与多实例 ban 必须先修 |
| M06 | Outbox/权重/session 占坑会放大全站故障；Redis 明文 key 降敏 |
| M07 | **Critical**；公网前先关 test 白嫖与控制面注入 |
| M08 | Runner 护栏尚可，编号体系与热表迁移不可上生产大库硬升 |
| M09 | 支付跳转与 secret 存储、授权合成是真安全项；其余多为分叉维护债 |
| M10 | 发布/回滚/默认暴露面与 M08 强耦合；local 默认值勿合上游 |

---

## 六、与上游协作建议

**可回馈上游的 hardened 补丁（小 PR）**
- Bulk 跨平台编辑拦截
- Outbox claim-token / lag 统计修正（修完后）
- secret_scan + govulncheck 流水线
- OpenAI partial/error-committed 守卫（若上游同类路径存在）

**应继续分叉、用模块边界隔离**
- AI Studio / Skills / skill-runner
- Grok / Kiro 全栈
- 发票 / 工单 / 部分支付恢复 UX
- TLS fingerprint capture 体系
- IP multi-account 治理（修好多实例后再考虑上游化）

**合并战术**
1. 定期 `merge upstream/main` 只收核心 gateway/auth/payment 主干
2. 个人功能用清晰目录与 feature flag 隔离，避免污染 `openai_gateway_handler.go` 巨型文件
3. 迁移采用 **全局唯一前缀命名空间**（如 `pdev_2xx_`）+ CI 检测，杜绝与上游同号

---

## 七、建议的下一步执行序列（非本轮范围，仅路线）

**Week 0（止血）**: B1 B2 A1 A2 A3 C1 C2  
**Week 1（稳定）**: E1 E2 E3 D1 A4 A5 B3  
**Week 2（协议/兼容）**: A6 + M03 请求历史 + M02 refresh/cooldown  
**持续**: 模块边界重构巨型 handler；上游 merge 流水线 + 迁移碰撞 CI  

每项落地必须带 **回归测试**（尤其 partial×cyber×failover、OAuth+TOTP、双实例 ban、Skills test mode）。

---

## 八、产物索引

```
.review/upstream-cross-2026-07-16/
  BRIEF.md                 # 共享审查规范
  TASKS.md                 # 十模块分区
  META.txt                 # BASE/HEAD/UPSTREAM
  M01-gateway-openai.md
  M02-platforms.md
  M03-apicompat.md
  M04-billing-payment.md
  M05-auth-security.md
  M06-scheduler-accounts.md
  M07-ai-skills.md
  M08-schema-migrations.md
  M09-frontend.md
  M10-deploy-ci.md
  CROSS-SYNTHESIS.md       # 本文件
```

