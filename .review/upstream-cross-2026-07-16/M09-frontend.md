# Module 9: Frontend 管理端与用户端

## 范围摘要
- 对比基线: `BASE=da85cc7e…` / `HEAD=eb64a5c1…` / `UPSTREAM=b960ec19…`（`upstream/main`）
- 路径焦点: `frontend/**`（views/admin、views/user、components、router、stores、api、utils、i18n）
- 变更规模: frontend 为个人分叉最重热点之一；相对上游明显增厚 **AI Studio / Skills / 支付恢复 / 工单 / 管理合规 / Feature Flag 路由矩阵 / 安全 sanitize 工具链**。源码体量以 `frontend/src` 为主（views 百级 + components 二百级 + 大量 vitest）。
- 相对上游的主要能力差异:
  - 管理端合规硬阻断（`adminCompliance` store + 路由守卫 fail-closed + 对话确认）
  - Skills 市场/安装/运行/审核/结算 + AI Studio（chat/image/gallery/prompts）完整前端
  - 支付多路径（Stripe popup/route、Airwallex、微信 OAuth/JSAPI、QR、resume_token 恢复）
  - FeatureFlags 注册表统一侧边栏/路由开关（opt-in / opt-out）
  - XSS 防护工具链：`sanitizeHtml`/`sanitizeSvg`/`sanitizeUrl`/`safeImageUrl`/`sanitizeRedirectPath`
  - API Key 列表掩码 + on-demand reveal；大量契约/守卫/i18n 单测

## 关键路径图
```
浏览器
  → main.ts initFromInjectedConfig (window.__APP_CONFIG__)
  → App.vue fetchPublicSettings
  → vue-router beforeEach
       ├─ checkAuth (localStorage token/user)
       ├─ requiresAuth / requiresAdmin
       ├─ adminCompliance (admin only, fail-closed)
       ├─ ensurePublicSettingsForFeatureRoute
       ├─ FeatureFlags: payment/ticket/affiliate/aiStudio/channelMonitor/availableChannels/riskControl
       ├─ ensureSkillRouteAccess (editable/owned via skills API normalize)
       ├─ simpleMode 路径前缀限制
       └─ backendMode allowlist
  → views (user/admin) → api/* (axios client + token refresh) → backend
  → 支付结果/恢复: /payment/* 公开路由 + localStorage recovery snapshot + public resume/verify API
  → 富文本: marked → DOMPurify/sanitizeHtml → v-html
```

## 发现清单

### [P1] 支付 `pay_url` / 跳转 URL 未做协议白名单校验
- **位置**:
  - `frontend/src/components/payment/paymentFlow.ts:145-242`（`payUrl: result.pay_url || ''` 原样入状态）
  - `frontend/src/views/user/PaymentView.vue:818-944`（`window.open` / `location.href`）
  - `frontend/src/components/payment/PaymentQRDialog.vue:159-162`
  - `frontend/src/components/payment/PaymentStatusPanel.vue:234-237`
  - `frontend/src/views/user/PaymentQRCodeView.vue:23-26,197`（query `pay_url` 直接绑 `href`）
- **相对上游**: 本地支付面显著加厚；跳转路径与上游同类风险叠加
- **问题**: 全链路信任后端/query 返回的 `pay_url`，未走 `sanitizeUrl`（仅允许 `http:`/`https:`）。`PaymentQRCodeView` 更将 query 串直接写入 `<a href>`。若上游响应被污染、中间人、或错误配置回写非 http(s) 协议，会形成 open-redirect / 危险协议导航。
- **影响**: 支付场景高敏；可把已登录用户导向钓鱼页，或在极端协议下触发非预期导航。前端无法替代后端签名校验，但应做最后一道协议/主机策略。
- **证据**:
  - `decidePaymentLaunch` 直接 `payUrl: result.pay_url || ''`
  - 启动分支无条件 `window.location.href = decision.paymentState.payUrl`
  - QR 页 `payUrl.value = String(route.query.pay_url || '')` 后绑定 `href`
- **建议**:
  1. 统一 `assertPaymentLaunchUrl(url)`：仅 `http:`/`https:`，可选同站相对路径白名单（`/payment/*`）。
  2. `PaymentQRCodeView` 禁止信任 query 中的任意 `pay_url`，改为 orderId + 服务端查询或 recovery snapshot 校验字段。
  3. 单测覆盖 `javascript:`、`data:`、`//evil`、非白名单 host。
- **交叉关注**: M04 支付后端 pay_url 生成与签名；M05 开放重定向治理

### [P1] 支付恢复快照把 `clientSecret` 持久化到 localStorage
- **位置**:
  - `frontend/src/components/payment/paymentFlow.ts:10,33-50,256-328`
  - `frontend/src/views/user/PaymentView.vue:407-410`（`writePaymentRecoverySnapshot`）
- **相对上游**: 本地新增/强化的支付恢复能力
- **问题**: `PaymentRecoverySnapshot.clientSecret` 以明文 JSON 写入 `localStorage['payment.recovery.current']`。同域任意 XSS、共享设备、浏览器扩展可读 Stripe/Airwallex client_secret，并在过期前复用恢复流。
- **影响**: 支付会话劫持窗口；与前端 XSS 面叠加后风险放大。
- **证据**: `writePaymentRecoverySnapshot` → `storage.setItem(key, JSON.stringify(snapshot))`，snapshot 必含 `clientSecret: string`。
- **建议**:
  1. 敏感字段改 `sessionStorage` 或内存 + 仅存 `resume_token`/`order_id`。
  2. 恢复时优先 `resolveOrderPublicByResumeToken`，由后端下发短期 secret。
  3. 写入前最小化字段；成功/终态立即清除（已有部分清除逻辑，需覆盖所有分支）。
- **交叉关注**: M04 支付恢复 token 设计；M05 XSS 纵深防御

### [P1] Skills 所有权/可编辑性在客户端可被 `localStorage.auth_user` 合成放大
- **位置**:
  - `frontend/src/api/skills.ts:104-114,436-472`
  - `frontend/src/router/index.ts:1104-1131`（`ensureSkillRouteAccess`）
- **相对上游**: 本地 Skills 前端新增
- **问题**: `normalizeSkillSummary` 在后端未显式给 `owned/editable` 时，用 `localStorage.auth_user.id === owner_id` 合成 `owned`，并默认 `editable = owned`；`can_view_source` 也默认 `!sourceLocked || owned`。路由守卫基于该归一化结果决定是否放行 `/skills/:id/edit|versions|runs|revenue`。攻击者可改本地 user JSON（无需改 JWT role）让 UI 认为自己是 owner。
- **影响**:
  - **前端授权绕过**：编辑/收益页可被打开（若后端仍返回内容则可能泄露源码/变量 schema）。
  - 写操作仍应被 API 拒绝，但属于“UI 当真、后端兜底”的脆弱模式；且 `currentUserId()` 与 JWT 主体无绑定校验。
- **证据**:
  ```ts
  const owned = asBoolean(source.owned ?? source.is_owner, false)
    || (ownerId !== null && ownerId === currentUserId())
  editable: asBoolean(source.editable ?? source.can_edit, owned)
  can_view_source: asBoolean(..., !sourceLocked || owned)
  ```
  守卫: `if (requiresSkillEditable && !skill.editable) return detail`
- **建议**:
  1. 所有权/可编辑/可看源 **只信后端布尔字段**；缺失时默认 `false`，禁止 localStorage 回退。
  2. 后端 skill detail 对非 owner 强制剥离 `content`（已有 redacted 测例，需保证生产契约）。
  3. 守卫失败时统一 dashboard，并增加“伪造 owned”单测。
- **交叉关注**: M07 Skills 权限与源码脱敏；M05 会话完整性

### [P1] 管理员身份在首屏导航窗口信任 localStorage 中的 `role`
- **位置**:
  - `frontend/src/stores/auth.ts:89-95,107-124`
  - `frontend/src/router/index.ts:1203-1263`
- **相对上游**: 本地与上游同类模式；合规守卫增加了 admin 路径重要性
- **问题**: `checkAuth()` 同步 `JSON.parse(localStorage.auth_user)` 后即 `isAdmin = role==='admin'`；`refreshUser` 异步。守卫在 `isAuthenticated`（token+user 均在）后立刻放行 `requiresAdmin` 路由。篡改本地 `role` 可短暂加载管理 SPA 壳（数据仍靠 API 403，但增加信息探测面，并与合规弹窗逻辑交互）。
- **影响**: 非资金级直通，但是管理面攻击面与合规 UX 的竞态；配合缓存的 admin 页面组件可能触发敏感 API 探测。
- **证据**: `isAdmin` 纯读 `user.value?.role`；`checkAuth` 先恢复再 `refreshUser(...).catch`。
- **建议**:
  1. 对 `requiresAdmin` 路由在 `refreshUser` 完成前阻塞或显示 pending。
  2. `role` 变更时强制重算合规状态并 `router.replace`。
  3. 关键 API 已有后端鉴权——保持，不把 SPA 当安全边界。
- **交叉关注**: M05 鉴权；M09 合规守卫

### [P2] Simple Mode 未限制用户侧 AI Studio / Skills 路由
- **位置**:
  - `frontend/src/navigation/simpleMode.ts:1-33`
  - `frontend/src/router/index.ts:214-396,1369-1376`
  - `frontend/src/components/layout/AppSidebar.vue:883-895`（AI/Skills 菜单无 `hideInSimpleMode`）
- **相对上游**: 本地新增 AI/Skills 后未同步 simple 模式矩阵
- **问题**: simple 限制列表覆盖支付/工单/订阅/管理资源等，但 **未** 包含 `/ai/*`、`/skills/*`。侧边栏 AI/技能入口也未 `hideInSimpleMode`。简易模式本意隐藏 SaaS 能力，却仍可直达技能市场与 AI 创作（仅受 `ai_studio_enabled` 控制）。
- **影响**: 运行模式语义漂移；计费/复杂度面在 simple 部署仍暴露。
- **证据**: `SIMPLE_MODE_RESTRICTED_PREFIXES` 有 `/admin/skills` 无 `/skills`/`/ai`；用户菜单 AI/Skills 无 hide 标记。
- **建议**: 将 `/ai`、`/skills` 纳入限制（或显式产品确认 simple 允许）；侧边栏同步 `hideInSimpleMode`；补 guards 测例。
- **交叉关注**: M07 AI Studio 产品边界；运行模式文档

### [P2] Feature Flag 在 public settings 加载失败时 fail-open（opt-out）/ 不阻断
- **位置**:
  - `frontend/src/utils/featureFlags.ts:145-167`
  - `frontend/src/router/index.ts:1095-1102,1297-1367`
  - `frontend/src/router/__tests__/feature-access.spec.ts:242-255`（明确“失败不当作禁用”）
- **相对上游**: 本地 FeatureFlags 体系
- **问题**: 守卫仅在 `isFeatureFlagResolved && !enabled` 时拒绝。settings 拉取失败时 payment（opt-out）按默认 **启用** 放行；opt-in 标志默认隐藏但仍可能在注入半残配置下抖动。测试还锁定了“失败不禁用”行为。
- **影响**: 运维关闭 payment/ticket 后若注入失败，用户仍可能进入支付 UI（下单应被后端拒）；造成支持噪音与短暂越权 UI。
- **建议**: 对资金相关路由（payment）在 unresolved 时 **deny 或 pending**；或区分“显式 false / 缺失 / 拉取失败”。
- **交叉关注**: M04 支付开关；后端 `PublicSettingsInjectionPayload` 漂移测试

### [P2] 自定义菜单 iframe 信任管理员 URL，并向第三方嵌入页泄漏 `user_id` 与完整 `src_url`
- **位置**:
  - `frontend/src/views/user/CustomPageView.vue:177-191,107-111`
  - `frontend/src/utils/embedded-url.ts:15-40`
- **相对上游**: 本地/分叉自定义页能力
- **问题**: iframe `src` 仅校验 `http(s)` 前缀；`buildEmbeddedUrl` 追加 `user_id`、`src_host`、`src_url=window.location.href`。被入侵的管理员账户或恶意菜单配置可把登录用户导到第三方并带上身份线索与完整控制台 URL（可能含 query）。
- **影响**: 隐私泄漏 / 钓鱼承载页；属信任管理员模型，但缺少 URL allowlist 与最小参数集。
- **建议**: 管理端保存菜单时校验 URL；嵌入参数可开关；默认不传 `src_url` 全路径；考虑 CSP `frame-src`。
- **交叉关注**: M05 管理配置安全

### [P2] `ImageUpload` image 模式预览未走 `safeImageUrl`
- **位置**: `frontend/src/components/common/ImageUpload.vue:17-21,136-144`
- **相对上游**: 本地组件；`safeImageUrl` 已在 UsersView 等处使用但不一致
- **问题**: SVG 模式有 `sanitizeSvg`，image 模式直接 `:src="modelValue"`，`FileReader.readAsDataURL` 可产生任意 `data:*`。若组件被绑定到可持久化且可被其他用户看见的字段，存在历史浏览器 SVG/HTML data URL 风险；与项目已有 `safeImageUrl` 策略不一致。
- **影响**: 目前多用于管理员本地预览，风险中等；一致性债务。
- **建议**: image 预览统一 `safeImageUrl`；限制 accept 与 data URL MIME。
- **交叉关注**: 头像/站点 logo 上传链路

### [P2] 核心路由守卫单测大量“模拟实现”，与生产守卫漂移
- **位置**:
  - `frontend/src/router/__tests__/guards.spec.ts:63-126`（手写 `simulateGuard`）
  - 对照生产 `frontend/src/router/index.ts:1197-1392`
- **相对上游**: 本地测试债
- **问题**: `simulateGuard` **省略** admin compliance、feature flags、skill access、public settings await 等关键分支；只测 auth/simple/backendMode。真实守卫行为改了，模拟测仍可绿。`feature-access.spec.ts` / `skills-routes.spec.ts` 覆盖部分真实守卫，但 auth 主文件仍是空壳风险最高的那块。
- **影响**: 回归虚假安全感；本次审查中的合规 fail-closed 等只能靠另一文件兜底。
- **建议**: 废弃 simulate，统一用 `feature-access` 的 hoisted `beforeEach` 注入测真实 guard；或导出纯函数 `evaluateNavigation` 供生产与测试共用。
- **交叉关注**: 全模块测试策略

### [P3] Backend mode allowlist 使用 `startsWith`，前缀过宽
- **位置**: `frontend/src/router/index.ts:1159-1194`
- **相对上游**: 本地 backend mode
- **问题**: `path.startsWith(allowedPath)` 使 `/login` 匹配 `/loginXXX`，`/payment/result` 匹配 `/payment/result-evil`。当前路由表未必注册这些路径，但是 allowlist 语义松散。
- **影响**: 低；未来新增路径时可能误放行。
- **建议**: 精确匹配或 `path === p || path.startsWith(p + '/')`。
- **交叉关注**: 无

### [P3] i18n 三套来源（json / 巨型 ts / split 目录）维护成本高
- **位置**: `frontend/src/i18n/locales/{en,zh}.{json,ts}` + `en/` `zh/` 分包；`i18n/__tests__/adminLocaleParity.spec.ts` 等
- **相对上游**: 本地与上游均有膨胀；个人分叉新增 skills/ai/payment 键更多
- **问题**: 存在运行时 merge 与 key 碰撞测试，但仍需人工维护多源；漏键多表现为 UI 原文/空白而非构建失败。
- **影响**: 合并上游冲突热点；文案回归靠抽样 required keys。
- **建议**: 收敛单一事实源；CI 对 `t('...')` 静态抽取做 missing-key 检查。
- **交叉关注**: 上游合并

### [P3] XSS 主路径总体可控，但仍有多处 `v-html`
- **位置**: HomeView / LegalDocumentView / CustomPageView / Announcement* / AdminComplianceDialog / AppSidebar SVG / UseKeyModal highlight / KeyUsageView 常量 SVG
- **相对上游**: 本地加强了 DOMPurify 与单测（`sanitize.spec.ts`、`safeImageUrl.spec.ts`）
- **问题**: 公告/合规/自定义页均 `marked` + sanitize，方向正确；UseKeyModal 对 apiKey/baseUrl 做了 `escapeHtml` 后高亮，较好。风险残留主要在“消毒配置默认 + 管理员可信内容”模型，而非明显未消毒用户输入。
- **影响**: 低-中（依赖 DOMPurify 默认配置与管理员可信）。
- **建议**: 对 markdown 统一封装 `renderMarkdownSafe()`；禁止新代码直接 `v-html` 未消毒字符串（eslint 规则）。
- **交叉关注**: M05

## 与上游合并风险
- **冲突热点文件**:
  - `frontend/src/router/index.ts`（路由矩阵膨胀）
  - `frontend/src/components/layout/AppSidebar.vue`
  - `frontend/src/i18n/locales/en.ts` / `zh.ts`（超大文案）
  - `frontend/src/api/client.ts`（401/支付恢复/合规 423）
  - `frontend/src/views/admin/SettingsView.vue`
  - 支付相关 `views/user/Payment*.vue`、`components/payment/**`
- **语义漂移点**:
  - `ai_studio_enabled` 同时闸门 AI 与 Skills（产品语义耦合）
  - FeatureFlags opt-in/out 默认与后端 injection 必须同步，否则菜单闪烁/误藏
  - `ADMIN_COMPLIANCE` 423 事件与路由 hard-block：上游若无此模型，合并需双侧兼容
  - Skills API 归一化层对多字段别名兼容 → 掩盖后端契约不稳定
- **分叉策略暗示**: 前端已从“上游控制台皮肤”变为“独立 SaaS 控制台”；继续与 upstream/main 逐文件合并成本高，更适合主题化模块边界或长期 fork。

## 测试与验证缺口
- **已有较真测例**:
  - `router/__tests__/feature-access.spec.ts`：合规 fail-closed、payment settings 等待
  - `utils/__tests__/sanitize.spec.ts` / `safeImageUrl.spec.ts` / `url.spec.ts`
  - `components/payment/__tests__/paymentFlow.spec.ts`：启动决策矩阵
  - `api/__tests__/skills.spec.ts`：redacted content 不合成空源码
  - `api/__tests__/client.spec.ts`：支付结果页 401 不踢登录
  - i18n parity / no key collision 部分覆盖
- **缺口**:
  - 生产 `beforeEach` 全路径（skill + simple + backend + compliance 组合）无集成测
  - `pay_url` 危险协议、recovery snapshot 含 secret 的安全测缺失
  - Skills `owned` localStorage 伪造测缺失
  - Simple mode × AI/Skills 矩阵测缺失
  - 自定义页 iframe 参数泄漏无测
  - E2E 支付/技能真实浏览器流依赖后端，前端单测无法替代

## 模块结论
- **整体风险评级: High**
- **是否建议合入上游 / 继续分叉 / 先修再合**: **先修再合（安全与授权相关 P1）**；功能面（Skills/AI/支付恢复/合规）与上游差异过大，更现实是 **继续分叉并建立模块化边界**，不宜幻想干净合入 upstream/main。
- **Top 3 必须处理项**:
  1. 支付 `pay_url`/跳转 URL 协议白名单 + QR query 去信任（P1）
  2. 支付 recovery 去掉 localStorage 中的 `clientSecret`（P1）
  3. Skills 所有权/可编辑/可看源禁止 localStorage 合成，只信后端显式字段（P1）
