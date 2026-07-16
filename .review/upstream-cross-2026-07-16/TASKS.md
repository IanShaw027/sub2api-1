# 十模块分区（禁止越权写代码；可只读参考邻接模块）

基线:
- BASE=`da85cc7e47882090b115d664afe8e39b37aa7417`
- HEAD=`eb64a5c1678eeb3dd4111e9a366e0b963f58a1e2`
- UPSTREAM=`b960ec19807ea81d6d83cad63e84ad45fdf7e08a` (upstream/main)

每路先读: `.review/upstream-cross-2026-07-16/BRIEF.md`
产物写入对应 `M0N-*.md`

## M01 Gateway OpenAI 核心协议
路径焦点:
- `backend/internal/handler/openai_*`
- `backend/internal/handler/gateway_handler_*`
- `backend/internal/service/` 中 openai/responses/chat/ws/compact/websearch 相关
- `backend/internal/service/openai_ws_v2/`
- `backend/internal/pkg/openai/`
重点: WS forwarding、partial billing、compact fallback、web-search history strip、failover、error committed 语义

## M02 多平台上游适配
路径焦点: kiro/grok/xai/gemini/antigravity/claude/anthropic 相关 service/handler/pkg
重点: OAuth continuation、quota probe、cache write scaling、first-event timeout、协议上报、cooldown

## M03 API 兼容层 apicompat
路径焦点: `backend/internal/pkg/apicompat/**`
重点: Anthropic↔Responses、ChatCompletions bridge、tool ID、cache usage、namespace tools、golden fixtures

## M04 计费支付用量订阅
路径焦点: payment/billing/usage/subscription/invoice/balance/quota/price 相关 service/handler/payment
重点: 双重/漏计费、fulfillment 原子性、subscription window、platform quota flusher、invoice

## M05 鉴权安全密钥与治理
路径焦点: auth/security/api-key/jwt/oauth session/2fa/password/IP multi-account/moderation/middleware
重点: 越权、密钥掩码/reveal、短窗口多账号 IP、step-up、审计绕过

## M06 调度账户并发与 Outbox
路径焦点: scheduler/account selection/concurrency/outbox/temp-unsched/cooldown/pool/weight
重点: 多实例槽位、权重校验、outbox 耐久、bulk edit 跨平台污染

## M07 AI Studio / Skills / Session
路径焦点: skill/aisession/aigallery/prompt/generation/aiasset/skillkit/skillrunner
重点: skill 计费归因、执行沙箱边界、权限、runner 隔离

## M08 Schema / Migration / Ent 领域模型
路径焦点: `backend/ent/schema/**`, `backend/migrations/**`, domain constants, wire 相关
重点: 迁移冲突/可重复执行、checksum、索引/唯一约束存量、schema 与代码漂移

## M09 Frontend 管理端与用户端
路径焦点: `frontend/**`
重点: 路由守卫、权限、支付/票据/技能 UI 与 API 契约、XSS/URL 安全、i18n、测试覆盖真实性

## M10 部署 CI 工具与运维面
路径焦点: `deploy/**`, `.github/**`, `tools/**`, Dockerfile*, Makefile, deploy.sh, SECURITY.md, goreleaser
重点: 发布脚本注入、secret scan、migration checksum sync、skill-runner 部署、CI 缺口
