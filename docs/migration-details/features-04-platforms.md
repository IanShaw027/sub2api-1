# 功能点清单：AI 平台接入（Claude / OpenAI / Grok / Kiro / Gemini / Antigravity）

> 对比基准：旧内容 = `upstream/main`；新内容 = `5806f923bdf47619b25fed4a69220b16e181164c`（personal-dev 合并树）。
> 覆盖范围：`cap_files.json` 中 id 10、11、12、13、14、15、16 共 7 个能力、约 296 个文件（118 个短文件名按 `internal/service` 优先解析到具体路径；另有 ~23 个旧路径在最终树中已被合并进同目录的其他文件，其内容已体现在下方对应文件的 diff 中，不重复列出）。
> 方法说明：文件量极大（尤其 `openai_gateway_service.go` 单文件新增 1.6 万行、`gateway_service.go` 新增 1.1 万行），采用"函数签名索引 + 关键片段抽查"的方式逐文件过 diff，再按主题合并成功能点；小文件全部打开确认。

---

## 能力：Claude / Anthropic 网关（对应 cap id 10）

### 功能点 1：Claude Code CLI 身份伪装与反封号请求改写
- 涉及文件：`backend/internal/service/claude_antiban_gate.go`、`gateway_service.go`（`injectClaudeCodePrompt`/`applyClaudeCodeMimicHeaders`/`normalizeClaudeOAuthRequestBody`/`applyClaudeCodeOAuthMimicryToBody`/`systemIncludesClaudeCodePrompt` 等）
- 描述：账号级开关控制是否对 OAuth 账号的请求做"伪装成官方 Claude Code CLI"处理——按需注入官方 system prompt、补齐/改写请求头使其与真实 CLI 一致、按账号环境画像重写 `metadata.user_id`。管理员可对每个账号单独决定是否启用该反封号模拟，降低被上游风控识别为"网关流量"的概率。

### 功能点 2：Claude Code 计费头签名（CCH）伪造与校验
- 涉及文件：`gateway_service.go`（`composeClaudeCodeBillingVersion`/`signBillingHeaderCCH`/`verifyCCHFromRealCLI`）
- 描述：新增生成并签发一个模拟真实 Claude Code CLI 版本号的计费头（CC Header），并提供与真实 CLI 签名比对校验的机制，使代理请求在计费/审计维度上与官方客户端一致，降低被上游按"非官方客户端"识别的风险。

### 功能点 3：OAuth 账号自定义 System Prompt 注入块
- 涉及文件：`gateway_service.go`（`parseClaudeOAuthSystemPromptBlocksConfig`/`buildClaudeOAuthSystemPromptBlocksJSON`/`rewriteSystemForNonClaudeCodeWithPromptBlocks`/`ValidateClaudeOAuthSystemPromptBlocksConfig`）
- 描述：管理员可为 Claude OAuth 账号配置一组自定义的额外 system prompt 文本块（含各自的 cache_control 设置），网关在转发非 Claude Code 来源的请求时自动注入这些块，用于统一注入合规声明/使用须知等文本，且可校验配置合法性。

### 功能点 4：Prompt Cache TTL 强制改写为 1 小时
- 涉及文件：`gateway_service.go`（`injectAnthropicCacheControlTTL1h`/`forceEphemeralCacheControlTTL`/`shouldInjectAnthropicCacheTTL1h`/`enforceCacheControlLimit`）
- 描述：账号级开关可强制把请求中所有 `cache_control` 的 TTL 改写为 1 小时（Anthropic 长缓存），并对超出数量限制的 cache_control 断点做裁剪，用于统一控制缓存成本/命中率策略。

### 功能点 5：Thinking 签名校验失败自动纠正重试
- 涉及文件：`gateway_service.go`（`shouldRectifySignatureError`/`isSignatureErrorPattern`/`isThinkingBlockSignatureError`/`matchSignaturePatterns`）
- 描述：识别上游返回的"thinking block 签名校验失败"类错误（可配置匹配模式），命中时自动触发对请求体的纠正与重试，而不是直接把错误透传给客户端，减少因签名不匹配导致的失败率。

### 功能点 6：count_tokens 不支持时自动降级模型重试
- 涉及文件：`gateway_service.go`（`isCountTokensUnsupported404`/`maybeRetryAnthropicModelFallback`/`resolveAnthropicFallbackUpstreamModel`）+ `backend/internal/service/model_fallback.go`（cap4）
- 描述：当上游对某模型的 `/v1/messages/count_tokens` 返回不支持（404）时，自动按配置的回退模型重试一次而不是直接报错给客户端。

### 功能点 7：`anthropic-beta` Header 按账号/模型的黑白名单策略
- 涉及文件：`gateway_service.go`（`evaluateBetaPolicy`/`betaPolicyScopeMatches`/`matchModelWhitelist`/`resolveRuleAction`/`checkBetaPolicyBlockForTokens`/`resolveBedrockBetaTokensForRequest`）
- 描述：新增一套可配置规则引擎，按账号类型（OAuth/Bedrock）、模型白名单，决定放行、剥离或直接拒绝请求中的特定 `anthropic-beta` token（例如某些 beta 功能只对特定模型开放），比原生 Anthropic 网关多了一层精细化的 beta 特性管控。

### 功能点 8：AWS Bedrock 上游完整转发通道
- 涉及文件：`gateway_service.go`（`ApplyBedrockCCCompat`/`forwardBedrock`/`executeBedrockUpstream`/`handleBedrockUpstreamErrors`/`buildUpstreamRequestBedrock*`/`handleBedrockNonStreamingResponse`）、`backend/internal/service/bedrock_request.go`（cap15）
- 描述：账号类型为 `bedrock` 时，新增完整的 AWS Bedrock 专用请求构建、鉴权、流式/非流式响应处理与错误分类通道，并支持"Claude Code 兼容模式"（`ApplyBedrockCCCompat`）把 Claude Code 客户端请求适配到 Bedrock 的调用约定。

### 功能点 9：Google Vertex AI 上游转发通道
- 涉及文件：`gateway_service.go`（`buildUpstreamRequestAnthropicVertex`/`filterVertexBetaTokens`）
- 描述：新增 Vertex AI 上的 Claude 模型转发支持，构建 Vertex 专用请求并按 Vertex 限制过滤不支持的 beta token。

### 功能点 10：自定义中转 Base URL（Custom Relay）
- 涉及文件：`gateway_service.go`（`buildCustomRelayURL`/`validateUpstreamBaseURL`）
- 描述：账号可配置自定义上游 Base URL（自建反代/中转），网关对该 URL 做校验（含 SSRF 防护）后再转发，而不局限于官方 Anthropic 端点。

### 功能点 11：账号选择新增负载感知与多级排序策略
- 涉及文件：`gateway_service.go`（`SelectAccountWithLoadAwareness`/`selectAccountWithMixedScheduling`/`filterByMinPriority`/`filterByMinLoadRate`/`filterBySoonestReset`/`selectByLRU`/`sortAccountsByPriorityAndLastUsed`/`shuffleWithinSortGroups`/`withWindowCostPrefetch`/`withRPMPrefetch`）
- 描述：账号调度新增"负载感知选择"：先按最小优先级分组过滤，再按窗口成本占用率（`isAccountSchedulableForWindowCost`）、RPM 占用率（`isAccountSchedulableForRPM`，均支持预取批量查询避免 N+1）依次过滤候选账号，再按"最快重置时间"筛选，最后同组内按 LRU（最久未用优先）或随机打散（同组打散防止总是选到同一个账号）挑选，比原生仅按优先级+最后使用时间排序更精细。同时支持混合调度模式（`selectAccountWithMixedScheduling`，跨平台账号池混合调度）与传统"legacy order"兼容路径。

### 功能点 12：账号选择失败的详细诊断与统计
- 涉及文件：`gateway_service.go`（`logDetailedSelectionFailure`/`collectSelectionFailureStats`/`diagnoseSelectionFailure`/`summarizeSelectionFailureStats`）
- 描述：当没有可用账号时，新增按"限流中/配额耗尽/模型不支持/被临时下线"等维度统计候选账号的具体拦截原因并写入详细日志，便于运维快速定位"为什么这个分组突然没账号可用"。

### 功能点 13：会话粘性（Sticky Session）与会话级配额
- 涉及文件：`gateway_service.go`（`checkAndRegisterSession`/`sessionQuotaAllows`/`RegisterSessionAfterAcquire`/`ClearStickySession`/`buildStableSessionSeed`/`GenerateSessionUUID`）
- 描述：同一会话（按 session hash）优先固定路由到同一账号，并支持会话级配额限制（同一会话在同一账号上的调用次数上限），账号不可用时可主动清除粘性绑定。

### 功能点 14：Claude Code 客户端空闲期伪心跳（No-op Delta Keepalive）
- 涉及文件：`gateway_service.go`（`shouldUseClaudeCodeNoopDeltaKeepalive`/`buildClaudeCodeNoopDeltaKeepalive`/`claudeCodeKeepaliveDeltaTypeForContentBlock`）
- 描述：识别到客户端是 Claude Code CLI 且流式响应长时间无实际增量时，插入空的 SSE delta 事件维持连接不被客户端/中间代理判定超时断开。

### 功能点 15：全链路调试时间线（Gateway Debug Timeline）
- 涉及文件：`gateway_service.go`（`WriteGatewayDebugTimelineEvent`/`RecordGatewayDebugTimelineBody`）、`backend/internal/handler/gateway_handler.go`（`emitGatewayDebugTimelineRequestReceived`/`AccountSelected`/`SlotAcquired`/`AttemptFinished`）
- 描述：新增按请求维度记录"收到请求→选中账号→获取并发槽位→请求结束"各阶段耗时与关键字段的 JSONL 时间线文件，支持按目录大小与保留天数自动清理（`cleanupGatewayDebugTimelineSizePressureLocked`），并可选记录请求/响应体（含针对 Kiro 帧聚合的专门聚合器 `KiroFrameAggregator`），用于线上问题回放排查。

### 功能点 16：Claude Code 遥测（Telemetry）批量转发与隐私脱敏
- 涉及文件：`backend/internal/handler/claude_telemetry_handler.go`、`claude_telemetry_forward.go`、`claude_telemetry_sanitizer.go`
- 描述：新增 `ClaudeTelemetryBatch` 端点转发 Claude Code 客户端的埋点/统计批量上报请求到上游，转发前对请求体做隐私脱敏（`SanitizeClaudeTelemetryBatch`）——包括改写/清除本地工作目录路径（`RewriteSystemReminderEnvBlocksWithWorkDirRewrite`）、清除 Base64 编码的敏感元数据泄露（`sanitizeBase64JSONLeakMetadata`），避免用户本地环境信息随遥测上报泄露给上游。

### 功能点 17：内容审核会话级拦截（Cyber Policy）
- 涉及文件：`backend/internal/handler/gateway_cyber_policy.go`
- 描述：一旦某会话被内容审核标记为违规（cyber policy），后续同会话请求直接在网关层拦截拒绝并记入 Ops 审计事件（`enqueueCyberSessionBlockedOpsEntry`），无需每次都重新过一遍完整审核流程，同时异步落库违规记录（含账号、模型、渠道用量字段）。

### 功能点 18：Grok 原生 WebSearch / X Search 独立计费端点
- 涉及文件：`backend/internal/handler/gateway_handler.go`（`WebSearch` 方法、`doGrokNativeXSearch`/`doGrokNativeWebSearch`/`buildGrokWebSearchPrompt`/`extractGrokWebSearchSources`）、`backend/internal/handler/openai_x_search.go`（cap14）
- 描述：新增独立的 `WebSearch`/`XSearch` HTTP 端点（仅 `platform=grok` 的分组可调用），通过构造合成 prompt 强制 Grok 走其原生 `web_search`/`x_search` 工具并解析结构化结果（标题/URL），把 Grok 的搜索能力包装成一个通用可单独计费的搜索 API，而不需要客户端自己拼 chat 请求。

### 功能点 19：工具名 / JSON 属性名双向重写（跨协议桥接防冲突）
- 涉及文件：`backend/internal/service/gateway_tool_rewrite.go`
- 描述：新增对请求体 JSON Schema 属性名与工具名的重写/流式增量还原能力：请求方向按配置表把属性名替换掉（`rewriteSchemaProperties`），返回方向（含 SSE 流式增量、跨多个 delta 片段拼接的场景）再还原回客户端期望的原始名字（`restorePropNamesInDeltaFragment`/`splitPendingPropNameKeyFragment`），用于避免与上游内建工具命名冲突或规避特征检测。

### 功能点 20：渠道级模型限制与用户/分组计费倍率
- 涉及文件：`gateway_service.go`（`ResolveChannelMapping`/`checkChannelPricingRestriction`/`isUpstreamModelRestrictedByChannel`/`ResolveUserGroupRateMultiplier`）
- 描述：新增按"渠道"（分组）维度限制某些模型是否可用/需要重定向映射，以及按"用户+分组"组合解析自定义计费倍率（覆盖分组默认倍率），支撑更细粒度的差异化定价。

### 功能点 21：长上下文（Long Context）独立计费维度
- 涉及文件：`gateway_service.go`（`RecordUsageWithLongContext`/`RecordUsageLongContextInput`）
- 描述：新增当请求上下文超过阈值时按更高单价（"higher priced upstream"，对应 `migrations/147_usage_log_higher_priced_upstream.sql`）单独记账的计费路径，与普通 token 计费分离统计。

### 无新增可感知功能的文件
- `backend/internal/service/gateway_forward.go`、`gateway_usage_billing.go`、`backend/internal/handler/gateway_web_search.go`：与 `upstream/main` 完全一致（diff 为空），当前实际逻辑已被拆分/合并到其他文件（如上文各功能点所在文件）。
- `backend/internal/pkg/claude/constants.go`、`thinking_protocol.go`、`upstream_response_limit.go`、`upstream_path_guard.go`、`upstream_models.go`：仅常量值/阈值微调或函数签名增加可选参数，无独立可感知行为变化。

---

## 能力：OpenAI / Codex 网关（对应 cap id 11，121 个文件，规模最大）

### 功能点 1：Claude Messages ↔ OpenAI Responses / Chat Completions 全双向协议桥接
- 涉及文件：`backend/internal/pkg/apicompat/` 全部文件（`anthropic_to_responses*.go`、`responses_to_anthropic*.go`、`chatcompletions_responses_bridge*.go`、`anthropic_to_cc_*.go`、`cc_to_anthropic_response.go`、`responses_to_chatcompletions.go`、`chatcompletions_to_responses.go`、`claude_tool_names.go`、`types.go`）、`backend/internal/service/openai_gateway_messages.go`、`openai_gateway_forward_cc_to_messages.go`、`gateway_forward_openai_compat_cc.go`、`openai_gateway_chat_completions_raw.go`
- 描述：新增一套完整、双向、支持流式 SSE 的协议转换层，使得：① 调用方用 Claude `/v1/messages` 格式请求也能打到只支持 OpenAI Responses/Chat Completions 的账号（自动转换请求体、工具定义、工具调用、stop_reason、usage 统计，再把响应转换回 Anthropic SSE 事件流）；② 反过来 OpenAI 格式客户端也可以调用 Claude 账号。包含 prompt cache 断点映射、web_search/web_fetch 工具结果互转、reasoning/thinking 内容互转、工具名命名空间化（避免同名冲突，`NamespaceToolNames`）等细节。这是让同一批 API Key 可以跨平台（Claude 协议壳 + OpenAI 账号池，或反之）调用的核心能力。

### 功能点 2：Codex OAuth 请求规范化与客户端仿真层
- 涉及文件：`backend/internal/service/openai_codex_transform.go`（3195 行）、`openai_codex_identity.go`、`openai_codex_identity_extra.go`、`openai_codex_probe_headers.go`、`openai_codex_tool_names.go`、`openai_agent_identity.go`
- 描述：新增大量针对 Codex CLI 官方客户端行为的仿真与请求修正逻辑：剥离/降级不被账号支持的原生图像生成工具、修正不合法的 JSON Schema（如剥离上游不支持的正则 lookaround 模式）、规范化工具调用 ID 前缀、清理孤立的 tool_call 输出、按模型自动补默认 system instructions、注入 Codex CLI 专属客户端元数据与探测头（`applyOpenAICodexProbeIdentityHeaders`）、Agent 身份任务注册（`registerAgentIdentityTask`），使代理请求在协议细节上与真实 Codex CLI 客户端保持一致。

### 功能点 3：OpenAI 账号高级调度器（权重随机 + 会话粘性预留 + 配额自动熔断）
- 涉及文件：`backend/internal/service/openai_account_scheduler.go`（3937 行）
- 描述：新增可配置的账号选择算法：支持"加权随机 Top-K"选择（`buildOpenAIWeightedSelectionOrder`/`selectTopKOpenAICandidates`，可配置各维度权重覆盖）；为粘性会话预留一定比例的并发槽位（`openAIStickyReservePercent`），避免新会话把粘性会话的槽位占满；基于账号错误率/TTFT 的运行时评分（`runtimeSnapshot`）参与排序；当账号配额使用率超过可配置阈值时自动暂停调度该账号（`shouldAutoPauseOpenAIAccountByQuota`/`shouldAutoPauseGrokAccountByQuota`，对 Grok/Codex 分别有专属阈值判定）；针对 Cloudflare 挑战/封锁维护逐账号退避等级（`openAICFBackoffLevel`）与 Grok OAuth 429 风暴检测（`isGrokOAuth429Storm`）。

### 功能点 4：Responses ↔ Chat Completions 账号级互转转发
- 涉及文件：`gateway_forward_openai_compat_cc.go`、`openai_gateway_forward_cc_to_messages.go`
- 描述：当账号只原生支持 Chat Completions 而客户端请求 Responses（或反之）时，网关自动做请求/响应格式互转后转发，账号能力与客户端协议解耦。

### 功能点 5：Codex 上下文压缩（Compact）与会话窗口续接
- 涉及文件：`backend/internal/service/openai_auto_compaction.go`、`openai_responses_session_window.go`、`openai_gateway_service.go`（`normalizeOpenAICompactRequestBody`/`ensureOpenAICompactDeferredToolSearch`/`bindOpenAIResponsesSessionWindow`/`rebuildOpenAIResponsesPayloadFromSessionWindow`）
- 描述：支持 Codex 客户端的远程上下文压缩（compact）请求识别与重试（压缩失败时的专用 SSE 错误类型），并新增"会话窗口"缓存以便压缩后能从缓存重建被压缩前的完整上下文续接对话，避免每次压缩都从头重放。

### 功能点 6：Codex 邀请额度重置（Invite Reset）管理功能
- 涉及文件：`backend/internal/service/codex_invite_reset_service.go`、`backend/internal/handler/admin/codex_invite_reset_handler.go`、`backend/internal/repository/codex_invite_reset_history_repo.go`
- 描述：新增完整的后台功能——调用 Codex 上游的"邀请好友得额度"接口，管理员可对指定账号发送邀请邮件、消费邀请额度（Consume Credit）来重置/追加该账号的可用配额，并记录历史操作，用于账号侧的额度自助恢复运营手段。

### 功能点 7：OpenAI OAuth 容量分小时统计与规划
- 涉及文件：`backend/internal/service/openai_oauth_capacity.go`、`openai_oauth_capacity_timeseries.go`、`backend/internal/repository/openai_oauth_capacity.go`
- 描述：新增按小时聚合 OAuth 账号的消耗/花费快照（`UpsertOpenAIOAuthCapacityHourlyFacts`）、历史区间消耗查询，为管理端的 OpenAI OAuth 容量规划面板提供数据支撑（详见 `docs/OPENAI_OAUTH_CAPACITY_PLANNING_CN.md`）。

### 功能点 8：流式失败重放去重（Stream Retry Replay）
- 涉及文件：`openai_gateway_service.go`（`openAIStreamRetryReplayState`/`beginOpenAIStreamRetryReplayAttempt`/`filterFrame`/`recordEmittedFrame`）
- 描述：流式响应中途失败切换账号重试时，记录已经发给客户端的增量内容签名，重试响应流回放时自动跳过已发送过的重复内容，避免客户端收到重复文本。

### 功能点 9：静默拒绝（Silent Refusal）检测
- 涉及文件：`openai_gateway_chat_completions_raw.go`（`IsOpenAISilentRefusalErrorBody`）
- 描述：识别上游返回"表面成功但内容为空拒绝"的响应模式，将其转换为可失败转移（failover）的错误而不是当作正常空回复返回给客户端。

### 功能点 10：Chat Completions 触发图像生成的自动桥接
- 涉及文件：`openai_gateway_chat_completions.go`（`forwardImageOnlyChatCompletions`/`buildOpenAIImagesRequestFromChatCompletions`/`writeChatCompletionsImageBridgeStream`）
- 描述：识别到 Chat Completions 请求实质是"仅要求生成图片"时，自动桥接到图像生成接口处理并把结果包装回 Chat Completions 响应/流格式，无需客户端改用专门的图像接口。

### 功能点 11：Embeddings / Speech / Transcriptions 端点代理
- 涉及文件：`backend/internal/service/openai_embeddings.go`、`openai_audio.go`（`ForwardOpenAISpeech`/`ForwardOpenAITranscriptions`）、`backend/internal/handler/openai_embeddings.go`、`openai_audio.go`
- 描述：新增 `/v1/embeddings`、`/v1/audio/speech`、`/v1/audio/transcriptions` 三个端点的完整透传代理（含错误处理与响应头过滤）。

### 功能点 12：OpenAI 侧内容审核 Cyber Policy
- 涉及文件：`backend/internal/service/openai_cyber_policy.go`、`openai_cyber_session_block.go`
- 描述：与 Claude 网关对称的内容审核会话拦截机制（`detectOpenAICyberPolicy`/`HandleOpenAICyberPolicy`），针对 OpenAI/Codex 协议的请求/响应内容做同样的违规检测与会话级封禁。

### 功能点 13：Reasoning Effort 自动派生与轻量 JSON Patch
- 涉及文件：`openai_gateway_service.go`（`deriveOpenAIReasoningEffortFromModel`/`openAIRequestView`/`MarkPatchSet`/`ApplyPatches`）
- 描述：按模型名自动推断默认的 reasoning effort（如模型名带 `-high`/`-low` 后缀），并引入一种"仅记录 JSON Patch、按需再应用"的请求体轻量修改机制，避免每处小改动都重新反序列化整个大请求体，降低大请求（尤其带完整对话历史）时的 CPU 开销。

### 功能点 14：多层次上游错误精细分类与客户端可见化
- 涉及文件：`openai_client_visible_upstream_error.go`、`openai_gateway_service.go`（`isOpenAIContextWindowError`/`isOpenAIUnsupportedReasoningEnabledError`/`isOpenAIInvalidEncryptedContentError`/`isOpenAIUnsupportedPreviousResponseIDError`/`isOpenAILargeRequestUpstreamError`/`classifyOpenAICodexCompatFallback`）
- 描述：新增对上游返回错误的十余种细分类型判定（上下文超限、加密推理内容失效、previous_response_id 不被支持、请求体过大等），分别决定"直接透传给客户端可读信息 / 静默降级重试 / 触发账号切换"等不同处理路径，比通用错误处理更精细。

### 无新增可感知功能的文件
- `openai_gateway_usage.go`、`openai_gateway_upstream_errors.go`、`openai_gateway_scheduling.go`、`openai_gateway_response_handling.go`、`openai_gateway_passthrough.go`、`openai_gateway_forward.go`：与 `upstream/main` 完全一致，逻辑已整体迁移进 `openai_gateway_service.go`（该文件本身是本能力中变更量最大的文件，见上文各功能点）。
- `backend/internal/pkg/openai/instructions_gpt5_2.txt`：与上游一致，未变更。

---

## 能力：OpenAI 实时 WebSocket / 语音（对应 cap id 12）

### 功能点 1：自研 HTTP/2 WebSocket 客户端（反指纹检测）
- 涉及文件：`openai_ws_client.go`、`openai_ws_client_h2.go`
- 描述：新增一套不依赖标准 WebSocket 库、直接基于 HTTP/2 帧手工实现的 WS 客户端（`dialOpenAIWSH2`/`handleH2Frame`/`writeWebSocketFrame`），使实时语音/对话连接也能应用与普通 HTTP 请求一致的 TLS/HTTP2 指纹伪装策略，而不是被标准 WS 库的默认指纹暴露。

### 功能点 2：WebSocket 连接池化与预热
- 涉及文件：`openai_ws_pool.go`、`openai_ws_pool_reconciler.go`
- 描述：新增按账号维护"中性"（未绑定具体会话）WS 连接池，后台协程周期性预热到目标空闲数量、健康检查、按空闲/过期原因驱逐连接，并支持按路由亲和性挑选"最空闲"连接复用，减少每次请求都重新握手带来的延迟。

### 功能点 3：增量上下文（Active Delta）优化与影子校验
- 涉及文件：`openai_ws_delta_shadow.go`、`openai_gateway_grok_active_delta.go`（Grok 版）
- 描述：新增基于对话历史内容哈希的"前缀未变化则只发增量"优化——通过对比历史 item 的规范化哈希判断是否可以只发送新增部分而非整个对话重放，同时提供"影子模式"（同时计算全量与增量结果比对，不一致时自动回退全量），显著降低长对话在 WS/HTTP 上的重复传输开销。

### 功能点 4：会话抢占（Session Preemption）
- 涉及文件：`openai_ws_session_preemption.go`
- 描述：新增分布式所有权声明机制，防止同一会话被两个并发请求同时抢占连接，检测到所有权丢失时主动取消并向调用方返回专门的"会话被抢占"错误，而不是产生数据错乱。

### 功能点 5：WS↔HTTP 双向降级桥接
- 涉及文件：`openai_ws_http_bridge.go`、`openai_gateway_service.go`（`shouldFallbackOpenAIWSToHTTP`/`prepareOpenAIWSContinuationFailoverBody`）
- 描述：让普通 HTTP 客户端的 Responses 请求可以透明地经由已池化的 WS 连接处理（降延迟）；WS 连接失败时可自动降级为普通 HTTP 全量重放请求，保证功能不中断。

### 功能点 6：WS 会话状态持久化与清理
- 涉及文件：`openai_ws_state_store.go`
- 描述：新增 response_id↔账号↔连接、会话上下文哈希、"上一次响应"绑定等状态的存取（区分本地内存与 Redis 持久化两级），配套 TTL 与周期性清理协程，支撑多副本部署下的会话连续性。

### 无新增可感知功能的文件
- （无，本能力全部文件均为新增内容，upstream/main 中不存在同名对照或差异巨大，无"纯重构无变化"文件。）

---

## 能力：OpenAI 图像生成（对应 cap id 13）

### 功能点 1：图片尺寸规范化、校验与自动纠偏
- 涉及文件：`openai_images.go`（`normalizeOpenAIImageSize`/`validateOpenAIImageDimensions`/`nearestOpenAIImageSize`/`classifyUnknownOpenAIImageSizeTier`）
- 描述：对客户端传入的任意图片尺寸自动规整/纠正到上游支持的最近标准尺寸档位，并对超限尺寸/像素数做校验拒绝，避免因尺寸参数不规范导致上游报错。

### 功能点 2：OAuth 账号"网页会话桥接"生图（非 API Key 计费通道）
- 涉及文件：`openai_images_responses.go`、`openai_images.go`（`ensureOpenAIImageSessionCredentials`/`buildOpenAIImageConversationRequest`/`buildOpenAIConversationAsyncStatusRequestTarget`）、`openai_images_telemetry.go`
- 描述：新增通过模拟 ChatGPT 网页版会话（Cookie/Conversation API）驱动生图的通道，用于订阅制 OAuth 账号（而非按 API 调用计费的账号）也能生成图片；配套完整的"挑战/指纹/网络状态"遥测记录（Cloudflare 挑战识别、UA/TLS 指纹来源追踪），并支持多账号"扇出"（fan-out，同时向多个账号发起以提高成功率）尝试。

### 功能点 3：图生图 / 局部重绘（Inpainting）的多态输入兼容
- 涉及文件：`openai_images.go`（`UsesLegacyInpainting`/`normalizeOpenAIImagesMaskUpload`/`resizeOpenAIImageMask`）
- 描述：兼容旧版"上传原图+蒙版做局部重绘"的图片编辑请求格式，自动按目标尺寸重采样蒙版图。

### 功能点 4：Sora 风格视频生成端点桥接
- 涉及文件：`openai_images.go`（`OpenAIVideoRequest`/`ParseOpenAIVideoRequest`）、`backend/internal/handler/openai_images.go`（`Videos` 方法）
- 描述：新增 `/v1/videos` 兼容端点解析与账号转发骨架，为后续对接 Sora / Grok 视频生成能力提供统一入口（Grok 视频生成复用此协议桥接，见 Grok 能力功能点）。

### 功能点 5：图像/视频计费维度扩展
- 涉及文件：`image_billing_multiplier.go`、`image_billing_size.go`、`batch_image_billing_hold.go`
- 描述：新增按图片尺寸档位、是否使用固定"非文本定价"（`usesFixedNonTextPricing`）等维度解析计费倍率，并为批量生图任务提供计费预扣与基于幂等键的重复提交回放（`replayBatchImageSubmission`）——重复提交同一批次请求直接返回原结果而不重复扣费。

### 功能点 6：图像生成工具"意图检测"与分组开关
- 涉及文件：`image_generation_intent.go`
- 描述：新增对请求体的静态分析，判定某次 Chat Completions / Responses 请求是否隐含"要求生成图片"的意图（含检查工具声明、工具选择、命名空间化工具引用等多种形态），并提供按分组维度开启/关闭"OpenAI 生图桥接""Images2API 桥接""视频生成"的开关（`GroupAllowsOpenAIImagesCodex`/`GroupAllowsOpenAIImages2API`/`GroupAllowsVideoGeneration`）。

### 无新增可感知功能的文件
- （本能力 10 个文件均为围绕上述几条功能点的新增/重写代码，无单独"无变化"文件。）

---

## 能力：Grok（xAI）集成（对应 cap id 14，重点关注）

> Grok 是当前活跃开发方向，以下尽量列全具体功能点。

### 功能点 1：Grok OAuth 设备码登录 + SSO Token 自动授权
- 涉及文件：`backend/internal/repository/grok_oauth_client.go`（`RequestDeviceCode`/`PollDeviceToken`/`AutoAuthorizeDeviceCode`/`autoAuthorizeGrokDeviceCode`）、`grok_oauth_service.go`
- 描述：支持标准 OAuth 设备码流程（生成设备码→轮询换 token），并额外实现"用已登录的 SSO Cookie Token 自动批准设备码授权"的自动化流程——管理员只需提供一个 grok.com 的 SSO token，系统即可自动完成设备码授权的批准环节，无需人工在浏览器里点击确认。

### 功能点 2：Grok 账单/订阅信息解析与套餐识别
- 涉及文件：`backend/internal/pkg/xai/billing.go`
- 描述：新增解析 xAI 账单 API 返回的"每周额度""每月额度""订阅信息"响应，计算周/月额度使用率（`WeeklyUtilization`/`MonthlyUtilization`）与周期边界，并按额度上限+订阅层级综合推断账号的规范化套餐名（`CanonicalGrokPlan`，如 Free/SuperGrok/Premium+），为管理端展示 Grok 账号的真实套餐与剩余额度提供数据源。

### 功能点 3：Grok 免费额度门控与调度过滤
- 涉及文件：`grok_free_quota_gate.go`
- 描述：区分"免费"与"付费"Grok OAuth 账号，按滚动时间窗口统计免费账号的实际用量并在调度候选列表阶段直接过滤掉已耗尽免费额度的账号（`filterGrokFreeQuotaAccounts`），避免请求被路由到已知会失败的免费账号上。

### 功能点 4：按配额阈值自动暂停账号调度
- 涉及文件：`openai_account_scheduler.go`（`shouldAutoPauseGrokAccountByQuota`/`shouldAutoPauseGrokQuotaWindow`/`grokQuotaSnapshotStaleForPause`）
- 描述：当 Grok 账号的额度窗口使用率超过可配置阈值时自动将其从调度池中暂停，并对过期（stale）的额度快照单独判定是否仍应暂停，避免因缓存过期数据误判。

### 功能点 5：Grok 专属上游错误分类与差异化冷却
- 涉及文件：`openai_gateway_grok.go`（`handleGrokAccountUpstreamError`、`isGrokRiskControlSpendingLimit`/`isGrokCreditsOrSubscriptionRequired`/`isGrokModelScopedPaymentRequired`/`isGrokFreeQuotaExhaustedError`/`isGrokTransientThrottleError`/`isGrokRequestScopedForbidden`）、`grok_upstream_errors.go`
- 描述：新增十种以上针对 xAI 特有错误响应的分类判定：风控消费限额、需要订阅/购买额度、按模型区分的"需付费"错误（并按模型单独持久化冷却，`persistGrokModelPaymentRequired`）、免费额度耗尽、瞬时限流（含从响应头解析精确的重置时间 `grokTransientThrottleCooldownFor`/`grokRateLimitResetTime`）、"重型模型"专属长冷却（`isGrokHeavyTransientModel`/`persistGrokTransientModelCooldown`）等，分别驱动不同的重试/切换账号/冷却时长策略。

### 功能点 6：Grok Responses 协议请求净化与推理后缀解析
- 涉及文件：`openai_gateway_grok.go`（`patchGrokResponsesBody`/`sanitizeGrokResponsesInput`/`sanitizeGrokResponsesTools`/`parseGrokThinkingSuffix`/`normalizeGrokResponsesReasoningEffort`/`dropGrokToolChoiceWithoutTools`）
- 描述：新增大量针对 xAI 实际接口与 OpenAI 标准 Responses 协议之间差异的净化逻辑：解析模型名里的推理强度后缀（如 `grok-4-fast-high` → base model + effort）、修正工具定义/工具选择的格式差异、清理无工具时残留的 tool_choice、规范化推理相关字段，使标准 OpenAI 格式客户端可以正常调用 Grok。

### 功能点 7：跨模型"加密推理内容"透传校验
- 涉及文件：`openai_gateway_grok.go`（`isLikelyValidGrokEncryptedContent`/`inspectGrokEncryptedContent`/`isDecodableClaudeThinkingSignature`/`isKnownGeminiThoughtSignatureEnvelope`/`byteEntropyRatio`）
- 描述：新增基于字节熵与结构特征的启发式检测，判断某段"加密推理内容"到底是合法的 Grok 加密块、还是被跨协议桥接错误带入的 Claude thinking 签名或 Gemini thought 签名信封，避免因协议混用导致 Grok 上游拒绝请求。

### 功能点 8：Grok HTTP 侧增量上下文（Active Delta）
- 涉及文件：`openai_gateway_grok_active_delta.go`
- 描述：为 Grok 的 HTTP Responses 端点实现与 OpenAI WS 类似的"仅发送增量上下文"优化，含按分支种子隔离并发分支会话（`grokActiveDeltaBranchSeed`）、恢复性错误识别后自动重放全量（`isGrokPreviousResponseRecoveryError`）、`store` 策略强制项（`applyGrokActiveDeltaStorePolicy`）。

### 功能点 9：Grok "团队"级速率限制识别
- 涉及文件：`openai_gateway_grok.go`（`withGrokTeamRateLimitModel`/`grokTeamRateLimitModelContextKey`）
- 描述：识别并单独标记 xAI 团队/组织级别（而非单账号级别）的速率限制场景，与普通账号级限流区分处理。

### 功能点 10：Grok 视频生成完整桥接（创建/编辑/查询）
- 涉及文件：`grok_media.go`（约 2000 行）、`openai_embeddings.go`（`ForwardVideos`/`forwardGrokVideoContentViaRetrieve`）
- 描述：新增将 Grok 原生视频生成/编辑 API 包装成 OpenAI `/v1/videos`（Sora 风格）兼容协议的完整桥接：宽高比/分辨率/时长参数归一化、图生视频的参考图输入处理、multipart 编辑请求组装、异步任务状态轮询与结果 URL/内容检索、请求/响应模型名双向映射（`buildOpenAIVideoCreateResponseFromGrok`/`buildOpenAIVideoRetrieveResponseFromGrok`）。

### 功能点 11：Grok 视频生成计费（创建时快照 + 完成时结算 + 防重复）
- 涉及文件：`grok_video_billing.go`
- 描述：视频创建时先落一份"待计费快照"（模型/时长/分辨率），等异步查询到生成完成状态后才真正计费，使用 Claim 原子声明防止同一任务被重复计费；并按 `(groupID, requestID, userID, apiKeyID)` 做归属绑定，防止跨用户抢占他人视频任务的计费权。

### 功能点 12：Grok 语音（Realtime Voice）支持
- 涉及文件：`grok_audio.go`、`backend/internal/handler/grok_audio.go`
- 描述：新增 Grok 实时语音会话端点支持，包含语音专属用量估算（当上游未返回精确 token 数时按音频时长/内容估算，`estimateGrokVoiceAudioUsage`）和语音终止错误的专门处理。

### 功能点 13：Grok 原生 X Search 独立端点
- 涉及文件：`backend/internal/handler/openai_x_search.go`
- 描述：新增 `XSearch` 端点，将 Grok 原生的 X（Twitter）实时搜索能力包装成独立可调用的 API（区别于聊天场景内的工具调用），并复用与 WebSearch 相同的计费/审计路径。

### 功能点 14：Grok 账号导入后台自动探测
- 涉及文件：`backend/internal/handler/admin/grok_import_probe.go`
- 描述：管理员批量导入 Grok 账号时，新增后台异步调度器逐个探测新账号的可用性/额度，不阻塞导入接口本身的响应。

### 功能点 15：Grok 凭证失败协同与刷新抖动
- 涉及文件：`grok_credential_failure.go`、`grok_token_refresher.go`（`grokTokenRefreshWindowWithJitter`）
- 描述：用 context 标记"当前请求正在处理该账号的凭证失败"以避免并发请求重复触发刷新/下线逻辑；token 刷新时间窗口按账号 ID 加随机抖动，避免同一批账号同时触发刷新造成流量尖峰或被上游识别为批量刷新行为。

### 功能点 16：Grok CLI 请求头仿真
- 涉及文件：`grok_upstream_headers.go`（`applyGrokCLIHeaders`）
- 描述：为转发到 xAI 上游的请求补齐/覆盖模拟 Grok 官方 CLI 客户端的请求头，降低被识别为第三方代理流量的概率。

### 无新增可感知功能的文件
- `backend/internal/pkg/xai/cli_identity.go`：diff 主要为常量/字符串值调整，无新增独立行为。
- `backend/internal/handler/admin/grok_oauth_handler.go`：整体是新文件，但对外只是标准 OAuth 管理端 CRUD 接口封装，具体行为已在上方"OAuth 设备码登录"功能点中体现，不单独计为一条。

---

## 能力：Kiro / Bedrock 网关（对应 cap id 15，几乎纯新增）

### 功能点 1：AWS IDC / 外部 IdP / 社交登录三种 OAuth 流程
- 涉及文件：`kiro_oauth_service.go`
- 描述：同时支持 AWS Identity Center 设备码流程（含"继续会话"续接）、外部 IdP（如企业 SSO/Microsoft）OIDC 授权码流程（含 IdP 元数据自动发现与端点合法性校验）、以及社交登录，三种登录方式统一收敛为账号凭证，并按登录方式自动推导 IDC 区域（`deriveKiroIDCRegionFromIssuerURL`）。

### 功能点 2：Anthropic Messages ↔ Kiro/CodeWhisperer 原生协议双向转换
- 涉及文件：`backend/internal/pkg/kiro/converter.go`（2500+ 行）
- 描述：新增完整的协议转换层：把 Claude Messages 请求转换为 Kiro CodeWhisperer 原生请求格式（历史消息重建、图片转换、文档文本抽取拼接、思考模式按模型归一化），并把 Kiro 原生工具调用/工具结果转换回 Anthropic 工具块格式；对不被 Kiro 原生支持的服务端工具（web_search/code_execution/bash 等）提供"影子工具"（Shadow Tool）桥接策略，使这些工具在 Kiro 账号上也能"看起来"被支持。同时包含按 token 预算裁剪/压缩历史对话（`TrimAnthropicRequestToTokenBudget`/`CompactAnthropicRequestToTokenBudget`）与身份声明脱敏（`SanitizeIdentityText`，避免模型自称"Kiro/Amazon Q"泄露底层身份）。

### 功能点 3：文档解析支持（PDF / DOCX / XLSX）
- 涉及文件：`backend/internal/pkg/kiro/document_pdf.go`、`document_office.go`、`document_limits.go`
- 描述：新增对上传附件中 PDF、Word（docx）、Excel（xlsx）文件的文本抽取能力（含按 rune 数限制截断防止超大文档撑爆上下文），使 Kiro 账号也能处理文档类附件请求。

### 功能点 4："伪缓存"（Fake Cache）计费统计
- 涉及文件：`backend/internal/pkg/kiro/fake_cache.go`
- 描述：Kiro 上游原生不返回 prompt cache 命中/创建的 token 统计，此处新增基于消息前缀链哈希的确定性"虚拟缓存"计算逻辑，合成出与 Anthropic 缓存计费口径一致的 cache_read/cache_creation token 数，使 Kiro 账号在计费与用量展示上与原生 Claude 账号保持一致的用户体验。

### 功能点 5：Web Search / Web Fetch 工具的双通道实现（原生 MCP + 自建影子服务）
- 涉及文件：`kiro_invokemcp.go`、`backend/internal/pkg/webfetch/fetcher.go`、`webfetch/types.go`
- 描述：优先通过 Kiro 官方 MCP（Model Context Protocol）沙箱调用真实的 web_search/web_fetch；当账号不满足 MCP 调用条件时（`kiroMCPEligible`），回退到网关自建的抓取服务（`webfetch.Fetcher`，内置 SSRF 防护：解析后 IP 校验、私网地址拦截、支持 HTTP/SOCKS 代理链、HTML 正文与标题提取、内容大小/字符集限制）本地执行，两种方式对客户端呈现一致的工具结果格式。

### 功能点 6：代码执行影子工具（Explicit Sandbox）
- 涉及文件：`kiro_gateway_service.go`（`executeCodeInExplicitSandbox`/`isCodeExecutionToolState`）
- 描述：当模型触发 `code_execution` 类工具调用时，新增在网关侧显式沙箱中执行代码并把 stdout/stderr 作为工具结果回填，为 Kiro 账号补齐官方不原生支持的代码执行能力。

### 功能点 7：Kiro 流式响应重建与思考块渲染
- 涉及文件：`kiro_gateway_service.go`（`forwardStream`、`writeKiroThinkingBlockStart/Delta/SignatureDelta/Stop`、`writeKiroShadowStreamBlock`）
- 描述：解析 Kiro 私有的二进制事件帧格式，重建为标准 Anthropic SSE 事件流（含 thinking 块的开始/增量/签名增量/结束事件、影子工具调用块的渲染），并在模型中途停止但存在未完成的原生 Web 工具调用时自动发起续接请求（`startKiroNativeWebToolContinuation`）补全结果。

### 功能点 8：Kiro 专属错误分类与账号自愈策略
- 涉及文件：`kiro_error_detail.go`、`kiro_gateway_service.go`（`shouldKiroFailover`/`shouldKiroRetrySameAccount`/`kiro429LooksShortBurst`/`markKiroFailureUnschedulable`/`maybeMarkKiroFirstEventTimeout`）
- 描述：区分配额耗尽、短时 429 突发（同账号重试）与需要切换账号的错误；对首字节响应连续超时的账号自动标记临时不可调度；token 失效时自动刷新后原地重试一次而非直接失败。

### 功能点 9：Kiro 用量/套餐/超额开关查询
- 涉及文件：`kiro_usage_service.go`
- 描述：新增查询账号当月/月度/赠送/试用额度明细、可用 Profile 与可用模型列表（按多个 AWS 区域探测回退），以及远程切换账号"允许超额计费"开关的能力，支撑管理端账号容量面板。

### 功能点 10：CodeWhisperer 客户端身份仿真
- 涉及文件：`backend/internal/pkg/kiro/machine_id.go`、`user_agent.go`
- 描述：按凭证派生稳定的 Machine ID，并生成与真实 Kiro/CodeWhisperer 桌面客户端一致的 User-Agent（区分 Streaming/Runtime/SSO-OIDC 三种场景），降低被上游识别为非官方客户端的概率。

### 功能点 11：离线协议抓包回放调试工具
- 涉及文件：`backend/cmd/kiro_native_capture/main.go`
- 描述：新增独立命令行工具，可对指定账号直接发起原生工具调用请求并抓取/解析原始响应帧，用于逆向分析 Kiro 协议细节，属开发调试辅助工具而非线上功能。

### 无新增可感知功能的文件
- （本能力 25 个文件绝大部分为 Kiro/Bedrock 全新子系统，upstream/main 基本不存在对应实现，无"纯重构"文件；`backend/internal/service/bedrock_request.go` 在本次统计范围内 diff 为空，实际 Bedrock 转发逻辑见 Claude 能力"功能点 8"。）

---

## 能力：Gemini 与 Antigravity 接入（对应 cap id 16）

### 功能点 1：Antigravity 多上游 URL 智能重试
- 涉及文件：`antigravity_gateway_service.go`（`antigravityRetryLoop`/`handleSmartRetry`/`shouldAntigravityFallbackToNextURL`）
- 描述：新增对多个可配置上游 URL 的顺序尝试机制：解析上游返回的"智能重试"信息判断是应该换一个 URL、换一个账号、还是原地等待重试，并区分"URL 级限流"与"账号级限流"分别处理，比单一固定 Endpoint 的直连方式更容错。

### 功能点 2：Antigravity 双协议（Gemini 原生 + Claude 兼容）统一转发
- 涉及文件：`antigravity_gateway_service.go`（`Forward`/`ForwardGemini`/`stripThinkingFromClaudeRequest`/`stripSignatureSensitiveBlocksFromClaudeRequest`）、`antigravity_gateway_claude.go`、`antigravity_gateway_gemini.go`
- 描述：同一账号池可同时处理原生 Gemini 协议请求与 Claude Messages 兼容协议请求，转发前按目标模型能力剥离不兼容的 thinking/签名敏感块，转发后统一走同一套流式/非流式响应处理与用量提取逻辑。

### 功能点 3：账号连通性测试端点
- 涉及文件：`antigravity_gateway_service.go`（`TestConnection`/`buildGeminiTestRequest`/`buildClaudeTestRequest`）
- 描述：新增供管理端"账号测试"按钮调用的最小化探测请求构建与结果解析，可分别用 Gemini 或 Claude 协议直接验证一个 Antigravity 账号的凭证与项目配置是否可用。

### 功能点 4：Antigravity 内部协议包装与身份补丁
- 涉及文件：`antigravity_gateway_service.go`（`wrapV1InternalRequest`/`unwrapV1InternalResponse`/`injectIdentityPatchToGeminiRequest`）
- 描述：新增将请求包装为 Antigravity 内部 V1 协议格式并在响应侧还原，同时向 Gemini 请求注入客户端身份补丁字段，以匹配 Antigravity 后端对请求格式的隐性要求。

### 功能点 5：模型级速率限制与容量冷却
- 涉及文件：`antigravity_gateway_service.go`（`setAntigravityModelRateLimits`/`setAntigravityGlobalModelCapacityCooldown`/`parseAntigravitySmartRetryInfo`）、`antigravity_internal500_penalty.go`
- 描述：按"账号+模型"维度而非整个账号维度记录限流冷却时间，解析上游返回的精确重置时间；连续遇到内部 500 错误的账号会被施加专门的临时下线惩罚（区别于普通错误）。

### 功能点 6：Gemini OAuth 三种账号模式区分路由
- 涉及文件：`gemini_messages_compat_service.go`（`isGeminiProjectRoutedOAuthAccount`/`isGeminiSharedPoolOAuthAccount`/`isGeminiAIStudioCompatibleOAuthAccount`/`shouldUseGeminiOAuthProjectStreamingBridge`）
- 描述：区分"独立 GCP 项目路由""共享账号池""AI Studio 兼容"三种 Gemini OAuth 账号类型，分别采用不同的流式桥接与调度策略，而非统一当作一种账号处理。

### 功能点 7：Gemini Code Assist 项目 Onboarding 与配额校验
- 涉及文件：`gemini_oauth_service.go`（`fetchProjectID`/`buildGeminiCodeAssistValidationError`/`buildGeminiCodeAssistIneligibleError`/`buildGeminiCodeAssistProjectIDRequiredError`）、`backend/internal/pkg/geminicli/codeassist_types.go`
- 描述：新增在账号授权阶段自动探测/校验该 Google 账号是否已开通 Code Assist、是否需要绑定自有 GCP 项目、是否命中"不合格层级"（Ineligible Tier）等情形，并把具体不合格原因转成可读错误提示给管理员，避免授权成功但实际不可用的"僵尸账号"。

### 功能点 8：Gemini AI Studio GET 端点透传与失败转移
- 涉及文件：`gemini_messages_compat_service.go`（`ForwardAIStudioGET`/`forwardGeminiAIStudioGETWithFailover`）
- 描述：新增对 Gemini AI Studio 系列 GET 类端点（如文件/模型列表）的透传代理，并在特定失败状态码时自动切换到下一个可用账号重试。

### 功能点 9：Claude / OpenAI 协议访问 Gemini 账号的兼容桥接
- 涉及文件：`gemini_messages_compat_service.go`（`convertGeminiToClaudeMessage`）、`gemini_chat_completions_compat_service.go`（`geminiResponseToResponses`）
- 描述：与 Claude/OpenAI 网关的桥接思路一致，新增把 Gemini 原生响应转换为 Claude Messages 或 OpenAI Responses 格式的适配层，使这两种协议的客户端也能透明调用 Gemini 账号（含多模态图片输出部分的合并处理）。

### 功能点 10：Gemini 429 同账号重试抑制与模型回退
- 涉及文件：`gemini_messages_compat_service.go`（`shouldStopGeminiSameAccountRetryOn429`/`maybeRetryGeminiModelFallback`）
- 描述：命中 429 时判断是否应立即停止在同一账号上重试转而切号，以及模型不可用时按配置自动回退到备用模型重试，逻辑与 Claude 网关的对应机制（功能点 6）对称。

### 功能点 11：Gemini Token 刷新分布式锁
- 涉及文件：`backend/internal/repository/gemini_token_cache.go`（`AcquireRefreshLock`/`ReleaseRefreshLock`）
- 描述：新增基于 Redis 的刷新锁，避免同一账号的并发请求同时触发多次 token 刷新造成竞态或刷新风暴。

### 无新增可感知功能的文件
- `antigravity_token_provider.go`、`antigravity_token_refresher.go`、`gemini_quota.go`、`gemini_token_cache.go`（service 层）、`gemini_token_refresher.go`、`geminicli_codeassist.go`：diff 中无新增导出函数/类型，主要是接入上文各功能点所需的胶水代码（依赖注入、字段透传），未构成独立可感知行为。

---

## 附：文件解析说明

`cap_files.json` 中以下 23 个短文件名在目标树 `5806f923b` 中已不存在（对应内容已合并进同目录下的其他现存文件，已在上文相应功能点中体现，不单独出现在文件列表里）：

`gateway_anthropic_passthrough.go`、`gateway_bedrock.go`、`gateway_claude_oauth_body.go`、`gateway_count_tokens.go`、`gateway_scheduling.go`、`gateway_upstream_request.go`、`gateway_upstream_response.go`（均并入 `backend/internal/service/gateway_service.go`）、
`openai_gateway_cc_pipeline.go`、`openai_gateway_messages_chat_fallback.go`、`openai_gateway_request_body.go`（并入 `openai_gateway_service.go` / `openai_gateway_messages.go`）、
`openai_ws_forwarder_ingress.go`、`openai_ws_forwarder_logutil.go`、`openai_ws_forwarder_payload.go`、`openai_ws_forwarder_support.go`、`openai_ws_forwarder_v2.go`、`openai_ws_v2/passthrough_relay.go`（并入 `openai_ws_forwarder.go`）、
`openai_gateway_grok_compact.go`（并入 `openai_gateway_grok.go`）、
`antigravity_gateway_claude.go`、`antigravity_gateway_gemini.go`、`antigravity_gateway_retry.go`、`antigravity_gateway_streaming.go`、`antigravity_gateway_upstream.go`（并入 `antigravity_gateway_service.go`）、
`backend/internal/pkg/geminicli/drive_client.go`、`backend/internal/repository/gemini_drive_client.go`——这两个文件在 `upstream/main` 中存在，但在目标树中已被删除（未发现同名替代实现），即本次合并相对上游是**移除**了 Gemini Drive 客户端相关代码，而非新增，故不计入"新增功能点"。
