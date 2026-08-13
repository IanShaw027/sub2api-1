# 功能点清单：AI 创作工作台/技能市场 / 支付发票订单 / 订阅兑换联盟 / 工单系统

> 对比基准：旧内容 = `upstream/main`；新内容 = commit `5806f923bdf47619b25fed4a69220b16e181164c`。
> 覆盖范围：`cap_files.json` 中 id 17（AI 创作工作台与技能市场）、18（支付/发票/订单）、19（订阅/兑换码/联盟返利）、20（工单系统），共约 216 个文件。

---

## 能力：AI 创作工作台与技能市场（对应 cap id 17）

这是两个共用底层（媒体存储、计费、审核）但业务上独立的子系统：① **AI Studio 创作工作台**（多模态生成 + 提示词库 + 短片生产流水线）；② **技能市场 Skill Market**（发布/审核/安装/运行/结算 AI 技能，含 Docker 沙箱）。全部 108 个文件在 `upstream/main` 中均不存在，即整个子系统是纯新增。

### 功能点 1：AI Studio 六大创作工作台（文本对话/文生图/文生视频/语音/实时通话/短片生产）
- 涉及文件：`backend/internal/service/ai_studio.go`、`backend/internal/service/ai_studio_modality.go`、`backend/internal/service/ai_center.go`、`backend/internal/service/ai_center_service.go`、`backend/internal/handler/ai_handler.go`、`backend/internal/handler/ai_gateway_bridge.go`、`backend/internal/handler/dto/ai.go`、`backend/internal/domain/ai.go`、`backend/internal/server/routes/ai.go`、`frontend/src/api/ai.ts`、`frontend/src/stores/aiStudio.ts`、`frontend/src/utils/studioRuntime.ts`、`frontend/src/utils/studioVisibility.ts`、`frontend/src/utils/studioQuery.ts`、`frontend/src/utils/studioKeyMask.ts`、`frontend/src/views/studio/StudioImageWorkbench.vue`、`frontend/src/views/studio/StudioVideoWorkbench.vue`、`frontend/src/views/studio/StudioAudioWorkbench.vue`、`frontend/src/views/user/AIChatView.vue`
- 描述：AI Studio 把「聊天对话（Chat）」「文生图/图生图（Image）」「文生视频（Video，一次性生成）」「语音（Audio）」「实时语音/视频通话（Realtime）」「多步骤短片生产（Production）」拆成 6 个独立工作台。每个工作台有一个总开关（`ai_studio_enabled`）和一个模态级开关（`AIStudioModalityEnabled`，缺省全开，管理员可单独关闭某个工作台）；每个模态还能单独配置「只对哪些分组（Group）开放」的白名单（`AIStudioModalityGroups`），未配置则对该用户所有可用分组开放。生成任务落地为 `AIGenerationJob`（记录 prompt/negative prompt/尺寸/张数/seed），产出物落地为 `AIAsset`（image/video/audio 三种类型，带可见性 private/unlisted/public 与内容审核状态）。会话式聊天走 `AISession`/`AISessionMessage`，支持多轮、系统提示词、消息状态（accepted/queued/completed/failed）。生成/对话实际算力仍复用网关侧的账号调度与计费（通过 `ai_gateway_bridge.go` 桥接到已有的 Claude/OpenAI/Gemini 网关），Studio 层只负责会话、素材与计费编排。

### 功能点 2：实时语音/视频通话工作台（WebRTC Realtime）
- 涉及文件：`backend/internal/handler/ai_realtime_bridge.go`、`backend/internal/handler/ai_live_bridge.go`、`backend/internal/handler/ai_stt_bridge.go`、`backend/internal/handler/ai_tts_bridge.go`、`frontend/src/utils/studioRealtimeAudio.ts`、`frontend/src/utils/studioRealtimeProtocol.ts`、`frontend/src/utils/studioRealtimeWs.ts`、`frontend/src/views/studio/StudioRealtimeWorkbench.vue`
- 描述：在 Realtime 工作台里用户可以像打电话一样和 AI 进行实时语音/视频对话，底层通过 WebRTC 建连（ICE/STUN/TURN 服务器列表由管理员在设置中配置，最多 16 条，`AIStudioLiveICEServer`），音频协议与信令走自定义的 WS 协议文件（`studioRealtimeProtocol.ts`/`studioRealtimeWs.ts`）。同时提供独立的语音转文字（STT）、文字转语音（TTS）桥接端点，供非实时场景（如文字转语音生成一段音频）单独调用。

### 功能点 3：AI 生成素材画廊、生命周期与存储计费
- 涉及文件：`backend/internal/service/ai_asset_lifecycle.go`、`backend/internal/service/ai_asset_lifecycle_errors.go`、`backend/internal/handler/ai_asset_extend.go`、`backend/internal/repository/ai_asset_lifecycle_repo.go`、`backend/internal/service/ai_studio_storage_billing.go`、`backend/internal/service/ai_studio_storage_billing_service.go`、`backend/internal/service/ai_studio_storage_debit_alerter.go`、`backend/internal/repository/ai_studio_storage_billing_repo.go`、`frontend/src/views/user/AIGalleryView.vue`、`frontend/src/views/admin/AIStorageChargesView.vue`
- 描述：每个生成出来的图片/视频/音频素材（`AIAsset`）有完整生命周期：`pending → ready → hidden/deleted/expired → purged`。素材默认有过期时间（`expires_at`）和过期后的宽限期（`purge_after`，全局默认 7 天，1-365 天可调），宽限期内到期素材从存储中真正物理删除。用户可以在「素材画廊」（AIGalleryView）里浏览/管理自己历史生成的素材，可延长保留期（`ai_asset_extend.go`）。存储按「每 GB 每天」计费（`group_storage_price_per_gb_day`），系统按天定时对仍在保留期内的素材扣费；若某次扣费失败（如余额不足或数据库异常），系统会给运维发一封告警邮件（按 charge_id 限流，同一笔最多 15 分钟一封，避免重试风暴），管理员可以在后台「存储扣费」页面（AIStorageChargesView）看到失败记录并手动重试扣费。

### 功能点 4：提示词模板库（Prompt Library）与内容审核治理
- 涉及文件：`frontend/src/components/ai/AiPromptEditorDialog.vue`、`frontend/src/views/user/AIPromptLibraryView.vue`、`frontend/src/views/admin/AIPromptGovernanceView.vue`、`frontend/src/views/admin/AIArtworkGovernanceView.vue`、`backend/internal/handler/admin/ai_handler.go`、`backend/internal/handler/dto/ai.go`
- 描述：用户可以把常用 prompt 保存为「提示词模板」并带版本历史（`AIPromptTemplateVersion`），设置可见性（private/unlisted/public）分享给其他人在库里浏览使用。管理员在「提示词治理」「作品治理」两个后台页面里可以对用户发布的 prompt 模板、生成任务、生成资产、公开画作分别做审核：`ModeratePromptTemplate`/`ModerateGenerationJob`/`ModerateAsset`/`UpdateArtwork`/`DeleteArtwork`，审核结果体现为 `moderation_state`（normal/forced_private/blocked），一旦被强制转私有或屏蔽，该内容即使原本是 public 也不再对其他用户可见，且会记入审计日志（`AIAuditLog`）。

### 功能点 5：技能创建、多版本编辑与发布（Skill 编辑器）
- 涉及文件：`backend/internal/service/ai_skill.go`、`backend/internal/service/ai_skill_service.go`、`backend/internal/service/ai_skill_domain_service.go`、`backend/internal/domain/ai_skill.go`、`backend/internal/handler/ai_skill_handler.go`、`backend/internal/handler/dto/ai_skill.go`、`backend/internal/repository/ai_skill_repo.go`、`backend/internal/repository/ai_skill_repo_helpers.go`、`frontend/src/api/skills.ts`、`frontend/src/stores/skillsCenter.ts`、`frontend/src/types/skills.ts`、`frontend/src/components/skills/SkillTypeEditor.vue`、`frontend/src/components/skills/SkillVariableForm.vue`、`frontend/src/components/skills/SkillVariableSchemaEditor.vue`、`frontend/src/components/skills/paths.ts`、`frontend/src/components/skills/presentation.ts`、`frontend/src/views/user/SkillEditorView.vue`
- 描述：任何用户都能创建「技能」并发布到市场供其他用户使用。技能有三种类型：`prompt_chat`（对话型 prompt 技能）、`prompt_image`（图像生成型 prompt 技能）、`script`（自定义脚本，跑在沙箱里，见功能点 8）。技能定义输入/输出 JSON Schema（`SkillVariableSchemaEditor`），支持多版本迭代，每次编辑生成一个新版本（草稿 draft），版本有独立的审核状态机（`draft → pending → approved/rejected`）。技能整体可见性分 `private/unlisted/public`，另有一个独立的「源码可见性」开关 `source_visibility`（public/hidden）：付费技能（`price_mode=paid`）默认自动隐藏源码/prompt，不公开可见性的技能不允许把源码设为公开——即免费才能让别人看到怎么实现的，收费的默认只能用不能看内容，除非作者主动打开。

### 功能点 6：技能审核流程（提交审核 → 管理员批准/拒绝/下线/强制私有）
- 涉及文件：`backend/internal/service/ai_skill_review_service.go`、`backend/internal/handler/admin/ai_skill_handler.go`、`frontend/src/views/admin/SkillReviewView.vue`、`frontend/src/views/admin/SkillGovernanceView.vue`、`frontend/src/components/skills/admin/SkillReviewDetailCard.vue`、`frontend/src/components/skills/admin/SkillActionDialog.vue`、`frontend/src/components/skills/admin/SkillAdminStatusBadge.vue`、`frontend/src/components/skills/admin/SkillAdminTimelineCard.vue`、`frontend/src/components/skills/admin/SkillAdminMetricGrid.vue`、`frontend/src/views/user/SkillVersionsView.vue`
- 描述：作者把某个版本提交审核后状态变为 `pending`，只有 `draft`/`rejected` 状态才允许再次提交，`pending` 状态才允许被审核，避免重复提交或跳过审核。管理员在「技能审核」页面逐条查看提交内容并批准/拒绝（`ApproveVersion`/`RejectVersion`），批准后该版本才允许被发布和被其他用户运行（`CanUseAISkillVersion`/`CanPublishAISkillVersion`）；对于 `script` 类型技能，批准的瞬间系统会对最终打包产物计算一次内容摘要并写入 `approved_artifact_digest`——这个摘要之后会被沙箱运行时用来校验「实际执行的代码字节」和「当时被人工审核过的代码字节」完全一致，防止作者审核通过后偷偷替换代码（`审核后再篡改` 攻击）。管理员在「技能治理」页面还能对已发布的技能做运营处置：`DisableVersion`（下线某版本）、`ForcePrivateSkill`（强制转私有，不下线但对其他用户不可见）。用户自己可以在「版本管理」页看到每个版本的审核状态与审核意见。

### 功能点 7：技能 Docker 沙箱运行时（针对 script 类型）
- 涉及文件：`backend/internal/integration/skillrunner/types.go`、`backend/internal/integration/skillrunner/docker.go`、`backend/internal/integration/skillrunner/dispatch.go`、`backend/internal/integration/skillrunner/executor.go`、`backend/internal/integration/skillrunner/inspector.go`、`backend/internal/integration/skillrunner/errors.go`、`backend/internal/service/ai_skill_runtime_gateway.go`、`backend/internal/service/ai_skill_run_service.go`、`backend/internal/handler/skillkit/module.go`、`backend/internal/handler/skillkit/queries.go`、`frontend/src/views/admin/SkillRuntimeMonitorView.vue`、`frontend/src/views/user/SkillRunsView.vue`、`deploy/skill-runner/*`
- 描述：`script` 类型技能以一个压缩包上传，包内必须有清单文件 `skill.yaml`（声明运行时 `python3.11` 或 `node20`、入口文件、协议版本），系统先做静态检查（archive 最大 16MB、单文件最大 4MB、最多 128 个文件、路径长度限制，防止 zip bomb/恶意包）。运行时（`Runner.Dispatch`）会为每次调用创建一个 Docker 容器：只读根文件系统、禁止访问网络（除非管理员显式放开）、去除所有 Linux capability、`no-new-privileges`、非 root 用户运行、CPU/内存/进程数/超时/临时盘/输出大小均有硬限制（默认 0.5 核、256MB、64 进程、10 秒超时、64MB tmp、64KB 输出）。输入输出通过容器内固定路径的 JSON 文件传递（`/sandbox/input/request.json` → `/sandbox/output/response.json`）。默认策略要求「必须有已批准的审核记录，且该记录的摘要要和本次要跑的代码包摘要完全一致」才允许执行（对应功能点 6 的防篡改摘要）。运行分「测试模式 test」和「正式使用模式 use」，运行状态机为 `queued → running → succeeded/failed/canceled`。管理员在「技能运行时监控」页面能看到所有正在跑/跑过的 sandbox 执行情况；用户在「我的运行记录」页面能看到自己触发过的运行历史。

### 功能点 8：技能计费结算与创作者收益分成
- 涉及文件：`backend/internal/service/ai_skill_settlement_service.go`、`backend/internal/repository/ai_skill_balance_ledger_repo.go`、`backend/internal/repository/affiliate_repo.go`（`CreditCreatorEarnings`/`ReverseCreatorEarnings`）、`frontend/src/views/admin/SkillSettlementView.vue`、`frontend/src/views/user/SkillRevenueView.vue`
- 描述：付费技能按「每次调用收费」（`billing_mode=per_request`），使用者调用一次即从余额里扣费，扣款金额按平台抽成比例（`PlatformCommissionRate`）拆成两份：一部分归平台，剩余部分记入技能作者的「创作者收益」账本（复用与联盟返利同一套 `user_affiliate_ledger` 台账机制，action 记为 `creator_earning`）。为保证「先调用上游产生真实费用、再扣钱」不会因为进程崩溃/重试而多扣或漏扣，结算走两阶段「意向-确认」模型：先创建一个待确认的结算意向（`CreateIntent`，标记 awaiting），只有上游真正接受了这次请求才 `ConfirmIntent` 标记 confirmed，随后才真正执行扣费与分成；如果上游从未成功接收请求，意向会被标记失败且永远不会被重放扣费。若给作者分成这一步失败或分成金额校验不一致，系统会自动把已经从买家账上扣的钱退回去（补偿事务），确保「要么完整扣款+完整分成，要么什么都不发生」，不会出现「买家被扣钱但作者没收到钱」的中间状态。免费技能（price=0）结算直接标记为 `skipped`，不产生任何扣款。作者可以在「收益」页面看到自己技能带来的收入明细；管理员在「结算管理」页面能看到全平台结算记录，并可以对失败的结算手动重放（`ReplaySkillSettlement`）。

### 功能点 9：技能市场浏览、安装与点赞
- 涉及文件：`frontend/src/views/user/SkillMarketView.vue`、`frontend/src/views/user/SkillDetailView.vue`、`frontend/src/views/user/SkillMySkillsView.vue`、`frontend/src/components/skills/SkillCard.vue`、`frontend/src/components/skills/SkillCenterNav.vue`
- 描述：所有 `visibility=public` 且已发布过版本的技能会出现在「技能市场」列表里供其他用户浏览、按分类/关键字搜索。用户可以「安装」一个技能到自己的「我的技能」列表（类似收藏 + 快捷访问，`SetSkillInstall`/`HasSkillInstall`），安装数会作为技能热度指标展示；也可以对技能点赞（`AISkillLikeInput`）。技能拥有者查看自己创建的技能列表时能看到全部信息，其他人只能看到已发布且公开可见的部分（`CanReadAISkill`/`CanListAISkillInLibrary` 权限判断），私有内容/未发布版本永远不会出现在市场里。

### 功能点 10：AI 短片生产工作台（多步骤视频生产流水线）
- 涉及文件：`backend/internal/service/studio_production.go`、`backend/internal/service/studio_production_ffmpeg.go`、`backend/internal/handler/studio_production_handler.go`、`backend/internal/handler/studio_production_render.go`、`backend/internal/repository/studio_production_repo.go`、`frontend/src/api/studioProduction.ts`、`frontend/src/views/studio/StudioProductionWorkbench.vue`
- 描述：这是区别于「一次性文生视频」的多步骤短片创作流水线，强制走固定的六个阶段，每一步都有状态门禁（不满足前置条件不能进入下一步）：① 创建项目并撰写创意简介（Brief）；② 定义角色/场景等「实体」（Entity），可为每个实体生成参考图（角色一致性用图）；③ 锁定「故事圣经」（Bible，角色/世界观设定一旦锁定后续步骤才能继续）；④ 基于 Bible 自动生成分镜脚本（Storyboard，每个镜头带描述、可引用角色参考图保证画面一致性）并生成分镜示意图；⑤ 锁定分镜；⑥ 按镜头逐个渲染成视频短片（`RenderShots`，后台异步任务，带渲染时长上限与并发渲染互斥锁，避免同一项目被重复渲染）；⑦ 把所有镜头视频用 ffmpeg 拼接成最终成片（Assemble）。整个流程有进度百分比展示（`ComputeStudioProjectProgress`），渲染/拼接任务用带主机名+所有权令牌的分布式锁防止多实例部署下重复执行同一个渲染任务。

### 涉及但未详细展开的辅助/基础设施文件
- `backend/internal/service/media.go`、`media_ingest.go`、`media_request_base_url.go`、`media_service.go`、`object_storage_config.go`：通用对象存储服务（上传/下载签名 URL/缩略图/可见性控制），被 AI 素材、技能封面、工单附件、发票文件等多个模块共用，本身不是 AI 专属功能。
- `frontend/src/api/admin/ai.ts`、`admin/media.ts`、`admin/skills.ts`：对应上述后端能力的管理端 API 客户端封装，无独立行为。

---

## 能力：支付 / 发票 / 订单（对应 cap id 18）

### 功能点 1：支付下单、多渠道网关与幂等履约
- 涉及文件：`backend/internal/service/payment_service.go`、`payment_order.go`、`payment_config_service.go`、`payment_config_providers.go`、`payment_config_plans.go`、`payment_config_limits.go`、`backend/internal/payment/registry.go`、`backend/internal/payment/load_balancer.go`、`backend/internal/payment/provider/alipay.go`、`stripe.go`、`wxpay.go`、`airwallex.go`、`backend/internal/handler/payment_handler.go`、`backend/internal/server/routes/payment.go`
- 描述：支持支付宝、微信支付、Stripe、Airwallex 四家支付渠道，同渠道可配置多个「渠道实例」（比如两个不同的微信商户号）并做负载均衡/限额分流。下单时校验：该支付方式是否被管理员启用（`EnabledTypes`）、当日限额、未完成订单数上限、余额充值是否被全局关闭；订阅订单额外要求已配置正数「USD→CNY 汇率」才允许用人民币渠道下单（未配置直接拒单，不会把美元数值当人民币金额收款——这是本次改动新增的资金安全兜底）。支付渠道配置在数据库中现在强制加密存储（AES-256-GCM），旧的「明文兜底读取」逻辑被移除，读不出配置直接报错而不是静默当空——避免因为解密失败而悄悄丢失商户密钥配置却无人发现。

### 功能点 2：订单履约幂等与「迟到支付」自动纠正
- 涉及文件：`backend/internal/service/payment_fulfillment.go`、`payment_order_lifecycle.go`、`payment_order_expiry_service.go`
- 描述：订单从「待支付 pending」到「已支付 paid」到「已完成 completed（余额到账/套餐生效）」全程用数据库状态机 + 履约租约令牌（`FulfillmentLeaseToken`）保证「即使进程崩溃重启/webhook 重复投递，同一笔订单也只会被履约一次」。新增能力：如果订单已经被系统判定「超时取消/过期」，但支付渠道随后又确认这笔钱其实付成功了（比如用户点了取消但同时手机上完成了支付），系统会自动把订单重新标记为已支付并照常发放余额/开通套餐，而不是让这笔钱变成「孤儿支付」需要人工介入对账。定时任务会主动查询微信等渠道里长时间挂起的待支付订单，避免因为 webhook 丢失而一直卡在待支付状态。

### 功能点 3：用户自助退款（含订阅按用量扣减 + 发票合规拦截）
- 涉及文件：`backend/internal/service/payment_refund.go`、`payment_amounts.go`、`payment_plan_validity.go`、`frontend/src/components/payment/*`、`frontend/src/components/admin/payment/AdminRefundDialog.vue`、`frontend/src/views/user/UserOrdersView.vue`、`frontend/src/views/admin/orders/AdminOrdersView.vue`
- 描述：这是本次新增的重要能力——退款不再只支持余额充值订单，订阅订单也能申请退款。用户申请退款前可以先查询「退款预览」（`GetRefundPreview`）：系统会算出这笔订单最多能退多少钱——余额充值订单最多退到「当前余额」为止（防止超额套现）；订阅订单则按「已使用天数对应的用量成本」扣减后计算剩余可退金额（用量 = 该订阅周期内产生的实际计费成本，按订阅倍率折算），退款金额越大扣减的订阅天数也越多（按比例向上取整）。如果该支付渠道被管理员标记为「允许用户自助退款」，退款会立即自动执行；否则进入「待审核」状态等管理员人工处理。新增合规拦截：如果这笔订单已经开具了正式发票（`ISSUED` 状态），系统会直接拒绝退款（提示需要走红字发票冲销），管理员必须显式 force 才能强制退款；仅仅是「已申请但还没开票」的发票会在退款成功后自动取消。退款到账后会按已退款比例，从对应联盟返利中按比例追回（见联盟返利能力）。

### 功能点 4：发票申请、合并开票、审核开具与作废（重点功能）
- 涉及文件：`backend/internal/service/invoice_service.go`（跨 cap 归属计费能力，但由 cap18 的 handler 暴露）、`backend/internal/handler/payment_handler.go`（`CreateInvoice`/`ListMyInvoices`/`GetInvoice`/`CancelInvoice`/`GetInvoiceDownloadURL`）、`backend/internal/handler/admin/payment_handler.go`（`ListInvoices`/`GetInvoiceDetail`/`UploadInvoiceFile`/`CancelInvoice`/`ResendInvoiceEmail`）、`backend/internal/payment/registry.go`（`InvoiceEnabled` 渠道开关）、`frontend/src/views/user/UserInvoicesView.vue`、`frontend/src/views/user/UserInvoiceDetailView.vue`、`frontend/src/views/admin/orders/AdminInvoiceApplicationsView.vue`、`frontend/src/views/user/UserOrdersView.vue`（勾选订单发起开票）
- 描述：**完整业务流程**——① 用户只能对状态为「已完成 completed」且所属支付渠道被管理员打开了 `invoice_enabled` 的订单申请开票；② 用户在「我的订单」页面勾选**一笔或多笔**订单（最多 100 笔），填写发票抬头、税号、收票邮箱、联系人、联系电话、备注，一次提交生成**一张**发票记录，把选中的多笔订单金额汇总为该发票的开票金额（合并开票，一对多关系存在 `invoice_orders` 关联表）；每笔订单只能被一张「有效」发票占用，重复申请会被拒绝。③ 发票状态机只有三态：`APPLIED（已申请，待开）→ ISSUED（已开具）` 或 `APPLIED → CANCELLED（已作废）`；**没有「重开/红冲」状态**——已经 `ISSUED` 的发票不能再被取消或修改，只能作废流程停留在申请阶段。④ 管理员在后台「发票申请」列表页看到所有用户申请（可按状态/关键字筛选，未读申请会有小红点提醒；打开详情即自动标记已读），对 `APPLIED` 状态的申请上传发票文件（PDF/图片）即完成开票，系统随之给用户发一封带下载链接的邮件通知（首次自动发送失败不影响开票结果，管理员还能在详情页手动点「重发邮件」）；对还没开具的申请可以直接作废（`CancelInvoice`），作废后对应订单的「已被占用」标记会解除，允许用户重新申请。⑤ 用户自己也能在「已申请」阶段主动取消发票申请；「已开具」阶段可以下载发票文件、查看发票关联的所有订单明细。⑥ 与退款联动：已开具发票的订单默认禁止退款（见功能点 3），防止已经报税/入账的订单被悄悄退款造成账务不一致。

### 功能点 5：支付渠道 Webhook 回调解析与幂等确认
- 涉及文件：`backend/internal/handler/payment_webhook_handler.go`、`backend/internal/service/payment_webhook_provider.go`
- 描述：Webhook 收到未知订单号时现在会直接确认（ACK）回调而不再报错，避免支付渠道因为查不到订单就无限重试推送；新增从 Stripe/微信等结构化 JSON payload 里的多个可能字段（`metadata.out_trade_no`、`client_reference_id` 等）智能提取商户订单号，用于在同一渠道配置了多个商户实例时正确路由到发起支付时使用的那个实例配置上做签名校验。

### 功能点 6：订单/发票/退款管理端与统计看板
- 涉及文件：`backend/internal/handler/admin/payment_handler.go`、`payment_stats.go`、`frontend/src/components/admin/payment/AdminOrderDetail.vue`、`AdminOrderTable.vue`、`OrderStatsCards.vue`、`PaymentMethodChart.vue`、`TopUsersLeaderboard.vue`、`frontend/src/views/admin/orders/AdminOrdersView.vue`、`AdminPaymentDashboardView.vue`
- 描述：管理员订单列表新增按日期区间筛选；订单详情、导出等接口统一返回该订单是否已被开票、开票状态，方便客服判断能否受理退款。仪表盘统计支付方式分布、充值 Top 用户榜等图表组件为既有能力的界面呈现，无新的业务规则。

### 功能点 7：套餐（订阅计划）有效期单位扩展
- 涉及文件：`backend/internal/service/payment_config_plans.go`、`payment_plan_validity.go`、`frontend/src/views/admin/orders/PlanEditDialog.vue`、`AdminPaymentPlansView.vue`
- 描述：订阅套餐的有效期单位由原来的「天/周/月」三种扩展为「天/周/月/年」四种，年按 365 天折算、月按 30 天折算。

### 无新增可感知功能的文件（内部健壮性重构，用户侧行为不变）
- `backend/internal/payment/types.go`、`backend/internal/payment/provider/stripe.go`（部分）、`backend/internal/service/payment_resume_lookup.go`、`payment_resume_service.go`：为退款/查询请求补充稳定的幂等请求 ID（provider 侧防重复退款），以及支付恢复令牌（resume token）的签名校验加固与新旧密钥轮换窗口——纯后端安全加固，用户感知不到差异。
- `backend/internal/service/payment_visible_method_instances.go`：多渠道实例存在歧义时的报错行为从「静默兜底」改为「强制要求管理员显式配置来源渠道」，属于配置态防呆而非用户可见功能。
- `backend/internal/service/promo_service.go`：修复了一处缓存失效的并发竞态与空指针防御，无行为变化。
- `frontend/src/utils/paymentPlanValidity.ts`、`frontend/src/components/user/orders/OrdersTabBar.vue`、`frontend/src/types/payment.ts`：类型定义与小工具函数随后端字段增补同步更新，无独立功能。

---

## 能力：订阅 / 兑换码 / 联盟返利（对应 cap id 19）

### 功能点 1：联盟返利规则（邀请绑定、多重返利上限、冻结期、退款追缴）
- 涉及文件：`backend/internal/repository/affiliate_repo.go`、`frontend/src/views/user/AffiliateView.vue`、`frontend/src/views/admin/affiliates/AdminAffiliateRecordsTable.vue`、`frontend/src/api/admin/affiliate.ts`、`frontend/src/api/admin/affiliates.ts`
- 描述：用户通过专属邀请链接（`/register?aff=邀请码`）邀请新用户注册后与邀请人绑定（一次性绑定，不可更改）。被邀请人充值/订阅时，邀请人按配置的返利比例（`rebate_rate`）获得返利，规则受三层上限约束：① 邀请人「终身累计返利上限」（`rebate_cap`，0 表示不限）；② 「单个被邀请人累计返利上限」（`per_invitee_cap`）；③ 「有效邀请人数上限」（`invitee_limit`）——即只有前 N 个被邀请人的消费才计入返利，超过名额后新邀请的人消费不再产生返利（但已占坑的老用户不受影响）。返利可配置「冻结期」（`freeze_hours`）：一定小时数内新产生的返利先进入「冻结额度」，到期后才转入「可用额度」，可用额度用户可以手动「转入余额」（`transferQuota`）变现。**退款追缴**：当某笔订单被退款（无论全额或部分）时，之前因这笔订单产生的返利会按「累计退款比例」等比例追缴回来，追缴顺序依次是：冻结额度 → 可用额度 → 已经转入余额的部分（即使已经花掉也会把用户余额扣成负数）——防止用户「先充值产生返利、拿到返利、再把充值全额退款」的套利漏洞。此外还新增「新用户注册奖励」（`ApplySignupBonus`，一次性）以及技能市场创作者收益（`CreditCreatorEarnings`，见 AI 技能能力）共用同一套账本（`user_affiliate_ledger`）。用户在「我的邀请」页面能看到总返利、邀请人数、已产生返利的被邀请人数、剩余可返利名额、每个被邀请人的消费/返利明细账本弹窗。

### 功能点 2：兑换码系统（余额/并发/订阅/邀请四类）与批量运营
- 涉及文件：`backend/internal/service/redeem_service.go`、`backend/internal/repository/redeem_code_repo.go`、`backend/internal/handler/admin/redeem_handler.go`、`backend/internal/handler/redeem_handler.go`
- 描述：兑换码支持四种类型：余额（balance）、并发数（concurrency）、订阅（subscription）、邀请（invitation）。管理员批量修改兑换码属性（分组等）时新增校验：如果这批码里有「订阅类型」的码，改动后必须仍然关联一个真实存在且类型为「订阅制」的分组，否则拒绝整批修改，防止误操作产生指向无效分组的订阅码。管理员后台的兑换码统计面板从此前的「写死返回 0 的占位数据」变成了真实统计（总数/未使用/已使用/已过期数量、总面值、已分发面值、按类型分布）。用户端新增分页版本的「余额变动历史」接口，支持按记录类型筛选并同时返回历史累计充值总额。

### 功能点 3：订阅分配、续期与用量窗口一致性加固
- 涉及文件：`backend/internal/service/subscription_service.go`、`subscription_maintenance_queue.go`、`backend/internal/repository/user_subscription_repo.go`
- 描述：给用户分配/续期订阅时，如果已有订阅记录会走「加锁续期」而不是「先读后写」，避免并发下单/兑换码redeem 与支付履约同时触发续期时互相覆盖对方的写入结果（数据一致性加固）；订阅用量缓存加入分片写屏障（fence），避免「缓存刚失效又被并发请求用旧数据重新填充」的经典缓存击穿问题。这些改动不改变用户能观察到的订阅续期规则或用量重置周期（日历日对齐日窗口、滚动对齐周/月窗口），纯内部并发正确性修复。

### 无新增可感知功能的文件
- `backend/internal/repository/promo_code_repo.go`、`backend/cmd/repair_affiliate_rebates/main.go`：`promo_code_repo.go` 只是补上事务上下文传递；`repair_affiliate_rebates` 是一次性运维修复工具（补跑历史上因 bug 漏发的联盟返利），非面向用户的常驻功能。

---

## 能力：工单系统（对应 cap id 20）

### 功能点 1：用户按分类创建工单，支持结构化表单
- 涉及文件：`backend/internal/handler/ticket_handler.go`、`backend/internal/handler/dto/ticket.go`、`frontend/src/api/tickets.ts`、`frontend/src/components/tickets/TicketCategoryForm.vue`、`TicketCreateDialog.vue`、`TicketEditorCard.vue`、`forms/TicketFormConsult.vue`、`forms/TicketFormRefund.vue`、`forms/TicketFormConcurrency.vue`、`forms/TicketFormRate.vue`、`forms/TicketFormOther.vue`、`frontend/src/utils/tickets.ts`、`frontend/src/views/user/TicketCreateView.vue`、`frontend/src/views/user/TicketsView.vue`
- 描述：用户创建工单必须先选择「分类」，五种分类各自对应一个专用结构化表单而不是纯文本框：① **咨询（consult）**——一段问题描述；② **退款申请（refund）**——订单号、期望退款金额、理由、证据说明；③ **并发数申请（concurrency_apply）**——当前并发数（自动带出用户当前值）、目标并发数、使用场景说明；④ **费率申请（rate_apply）**——从用户当前可用的「标准订阅类分组」里勾选一个或多个分组，展示每个分组的基础倍率与该用户的专属特殊倍率，提交时把「生效倍率」快照进表单一并提交，避免后续分组倍率调整导致申请内容对不上；⑤ **其他（other）**——自由描述。表单内容以 JSON（`form_payload`）存储并按分类做必填字段前端校验。

### 功能点 2：工单状态流转（提交 → 处理 → 撤回可编辑重新提交 → 关闭）
- 涉及文件：`backend/internal/repository/ticket_repo.go`、`backend/internal/handler/ticket_handler.go`（`Withdraw`/`Update`/`Resubmit`/`Close`）
- 描述：状态机为 `submitted（已提交）→ processing（处理中）→ waiting_user/waiting_admin（等待用户/等待客服回复）→ resolved（已解决）→ closed（已关闭）`，另有独立分支 `withdrawn（已撤回）`。**用户在工单还没被处理完之前可以主动「撤回」**（仅 submitted/processing/waiting_admin 状态允许），撤回后工单进入可编辑状态，用户可以修改标题和表单内容后「重新提交」（版本号 `revision_no` 递增，避免多端并发编辑冲突用乐观锁 `expected_revision_no` 校验，冲突则拒绝提交）；重新提交会生成一条新的表单修订记录（`support_ticket_revisions`），保留每一次提交内容的历史。用户任何时候都能主动「关闭」自己的工单（已关闭/已撤回状态除外）。管理员手动切换状态受限制：只有当「最后一条回复是客服发的」时才允许把工单切到「等待用户回复」或「已解决」，避免客服在没回复用户的情况下就把工单标记已解决；已关闭/已撤回的工单不允许再回复（`ErrTicketReplyLocked`）。

### 功能点 3：客服/用户双向回复，支持图片/文件附件
- 涉及文件：`backend/internal/handler/ticket_handler.go`（`Reply`/`resolveAttachmentsForUser`）、`backend/internal/handler/admin/ticket_handler.go`（`Reply`/`resolveAttachmentsForAdmin`）、`frontend/src/components/tickets/TicketConversationPane.vue`
- 描述：用户和客服在同一个对话线程里互相回复（类似聊天窗口），回复可以只带附件不带文字，也可以只带文字。附件走通用媒体上传（`biz_type=ticket`），上传后的图片会显示缩略图预览并可点击查看大图/下载，非图片文件显示文件名和大小。每条回复会做「10 秒内容重复」去重（同一发送者、同类型、同内容、同附件在 10 秒内重复提交视为同一次操作，不会插入两条重复消息），用于防止网络重试导致的重复回复。管理员在回复框里可以从「快捷回复模板」里一键插入预设话术，模板可由管理员在后台自行增删改（`TicketReplyTemplatesDialog.vue`）。

### 功能点 4：客服后台管理（列表、筛选、未读提醒、状态切换）
- 涉及文件：`backend/internal/handler/admin/ticket_handler.go`、`frontend/src/views/admin/TicketsView.vue`、`frontend/src/views/admin/TicketDetailView.vue`
- 描述：管理员后台工单列表支持按状态、分类、关键字（标题/工单号/用户名/邮箱）、日期区间、「仅看未读」「仅看有提醒」筛选；每条工单独立维护「客服未读」和「用户未读」两个标记，任一方回复后自动置对方为未读、己方已读，打开详情页自动标记已读，未读工单在导航栏上有数字角标提示（`useNavigationBadges`）。管理员在工单详情页可以直接切换工单状态（受上文提到的转移规则约束）。

### 涉及但未详细展开的辅助文件
- `frontend/src/components/tickets/TicketDetailPane.vue`、`TicketInfoItem.vue`：详情页信息展示的纯 UI 组件，无独立业务逻辑。
- `frontend/src/views/user/TicketDetailView.vue`：用户侧详情页，整合了功能点 2/3 的撤回/编辑/回复/关闭动作，无新增规则。

> 说明：工单的「邮件通知」机制（用户回复时通知所有管理员、管理员回复时通知用户）实现在 `backend/internal/service/ticket_service.go` 与 `ticket.go`（属于 cap id 28「管理端服务」范围，不在本次 17/18/19/20 文件清单内，但为完整描述工单业务流程在此说明：两侧回复都会异步排队一封邮件通知对方，失败仅记录日志不影响回复本身）。

---

## 附：能力对应 cap id 与文件规模

| 能力 | cap id | 文件数 | 功能点数 |
|---|---|---|---|
| AI 创作工作台与技能市场 | 17 | 108 | 10（+2 组辅助文件说明） |
| 支付 / 发票 / 订单 | 18 | 70 | 7（+1 组无新增功能说明） |
| 订阅 / 兑换码 / 联盟返利 | 19 | 14 | 3（+1 组无新增功能说明） |
| 工单系统 | 20 | 24 | 4（+1 组辅助文件说明） |
