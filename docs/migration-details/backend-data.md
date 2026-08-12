# 迁移明细 B：后端数据层与接入层（ent / handler / repository / pkg / server 等）

> 基准：`git diff upstream/main...personal-dev --numstat`，共 1151 个文件。
> 重要度：P0 核心（缺了核心工作流不可用或安全回退）/ P1 重要（明显功能或稳定性增强）/ P2 可选（便利、锦上添花）/ P3 建议放弃（一次性产物、被上游取代、过时）。
> 取舍栏默认 ☐ 待定，由用户填写。
> 说明：本明细覆盖 `backend/` 中除 `internal/service` 以外的全部领域；service 层见附录 A / 明细 A。

## 1. ent schema（数据库实体定义）（建议批次 0，整体重要度 P0）

全部数据库实体的源定义，是批次 0 的决策核心：每保留一个 schema，就连带保留对应生成代码与 migrations。新增 19 个实体（发票、AI Studio/Skill、TLS 指纹策略），修改 14 个上游实体。迁移方式：直接拷贝 schema 文件后 `go generate ./ent`。⚠️ 注意 `group.go` 假删除风险（personal-dev 删掉了上游 profit_control_* 三字段，合并时必须保留上游声明）。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `backend/ent/schema/auth_identity_schema_test.go` | +1/-1 | schema 契约测试 | ☐ |
| P1 | `backend/ent/schema/channel_monitor.go` | +1/-1 | 渠道监控新增 kiro/grok provider 枚举 | ☐ |
| P1 | `backend/ent/schema/channel_monitor_request_template.go` | +1/-1 | 渠道监控请求模板 | ☐ |
| P1 | `backend/ent/schema/ai_skill_run.go` | +100/-0 | AI Studio/Skill 新增实体 | ☐ |
| P1 | `backend/ent/schema/ai_skill_review.go` | +104/-0 | AI Studio/Skill 新增实体 | ☐ |
| P0 | `backend/ent/schema/tls_fingerprint_profile.go` | +104/-0 | 指纹画像新增 20+ 字段（H2、头模板、扩展回放） | ☐ |
| P1 | `backend/ent/schema/ai_skill.go` | +113/-0 | AI Studio/Skill 新增实体 | ☐ |
| P1 | `backend/ent/schema/ai_skill_settlement.go` | +113/-0 | AI Studio/Skill 新增实体 | ☐ |
| P1 | `backend/ent/schema/ai_asset.go` | +117/-0 | AI Studio/Skill 新增实体 | ☐ |
| P1 | `backend/ent/schema/ai_skill_version.go` | +123/-0 | AI Studio/Skill 新增实体 | ☐ |
| P1 | `backend/ent/schema/payment_order.go` | +14/-0 | 发票关联、履约租约 | ☐ |
| P1 | `backend/ent/schema/payment_provider_instance.go` | +2/-0 | 支付渠道实例、防重索引 | ☐ |
| P0 | `backend/ent/schema/user_platform_quota.go` | +2/-1 | 新增 kiro/grok 平台枚举维度 | ☐ |
| P0 | `backend/ent/schema/usage_log.go` | +21/-9 | 视频计费重构、WS 复用统计、tls_fingerprint 记录 | ☐ |
| P0 | `backend/ent/schema/api_key.go` | +22/-2 | API Key 安全存储（lookup_hash+AES-GCM+前缀），安全回退项 | ☐ |
| P0 | `backend/ent/schema/user.go` | +24/-3 | 余额 decimal(22,10)、token_version、Skill 级联边 | ☐ |
| P1 | `backend/ent/schema/payment_audit_log.go` | +3/-0 | 支付审计日志 | ☐ |
| P0 | `backend/ent/schema/account_tls_fingerprint_policy.go` | +36/-0 | 账号级指纹策略 | ☐ |
| P0 | `backend/ent/schema/account_tls_fingerprint_binding.go` | +37/-0 | 账号-指纹绑定 | ☐ |
| P1 | `backend/ent/schema/proxy.go` | +4/-1 | backup_proxy 边改名 fallback_sources（影响生成代码调用点） | ☐ |
| P1 | `backend/ent/schema/batch_image_job.go` | +6/-1 | 幂等索引改 (user_id, api_key_id, idempotency_key) | ☐ |
| P1 | `backend/ent/schema/ai_skill_install.go` | +63/-0 | AI Studio/Skill 新增实体 | ☐ |
| P1 | `backend/ent/schema/ai_audit_log.go` | +66/-0 | AI Studio/Skill 新增实体 | ☐ |
| P1 | `backend/ent/schema/ai_skill_like.go` | +66/-0 | AI Studio/Skill 新增实体 | ☐ |
| P1 | `backend/ent/schema/invoice_order.go` | +69/-0 | 发票-订单互斥占用 | ☐ |
| P0 | `backend/ent/schema/tls_fingerprint_router.go` | +70/-0 | TLS 指纹路由 | ☐ |
| P1 | `backend/ent/schema/ai_skill_schema_test.go` | +71/-0 | schema 契约测试 | ☐ |
| P1 | `backend/ent/schema/ai_session.go` | +75/-0 | AI Studio/Skill 新增实体 | ☐ |
| P1 | `backend/ent/schema/ai_prompt_template_version.go` | +78/-0 | AI Studio/Skill 新增实体 | ☐ |
| P1 | `backend/ent/schema/invoice.go` | +81/-0 | 发票主体（合并开票、状态机） | ☐ |
| P1 | `backend/ent/schema/ai_prompt_template.go` | +83/-0 | AI Studio/Skill 新增实体 | ☐ |
| P1 | `backend/ent/schema/ai_session_message.go` | +83/-0 | AI Studio/Skill 新增实体 | ☐ |
| P0 | `backend/ent/schema/group.go` | +87/-26 | 图片/视频/存储计费字段 ⚠️ 保留上游 profit_control_* 三字段（假删除风险） | ☐ |
| P1 | `backend/ent/schema/ai_generation_job.go` | +89/-0 | AI Studio/Skill 新增实体 | ☐ |

## 2. ent 生成代码（建议批次 0，重要度跟随对应 schema）

`go generate ./ent` 的产物，**全部不手工迁移**：对应 schema 落地后重新生成即可，diff 行数仅供参考。按实体分组列出便于与 schema 决策对照；放弃某 schema 则对应生成代码自动消失。`ent/migrate/` 下两个手写测试文件除外，需按普通代码迁移。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | AccountTLSFingerprintBinding 系生成代码（7 文件）<details><summary>7 个文件</summary>`backend/ent/accounttlsfingerprintbinding/accounttlsfingerprintbinding.go`<br>`backend/ent/accounttlsfingerprintbinding.go`<br>`backend/ent/accounttlsfingerprintbinding_update.go`<br>`backend/ent/accounttlsfingerprintbinding/where.go`<br>`backend/ent/accounttlsfingerprintbinding_query.go`<br>`backend/ent/accounttlsfingerprintbinding_delete.go`<br>`backend/ent/accounttlsfingerprintbinding_create.go`</details> | +2999/-0 | TLS 指纹；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P0 | AccountTLSFingerprintPolicy 系生成代码（7 文件）<details><summary>7 个文件</summary>`backend/ent/accounttlsfingerprintpolicy_create.go`<br>`backend/ent/accounttlsfingerprintpolicy/accounttlsfingerprintpolicy.go`<br>`backend/ent/accounttlsfingerprintpolicy.go`<br>`backend/ent/accounttlsfingerprintpolicy_query.go`<br>`backend/ent/accounttlsfingerprintpolicy/where.go`<br>`backend/ent/accounttlsfingerprintpolicy_update.go`<br>`backend/ent/accounttlsfingerprintpolicy_delete.go`</details> | +3257/-0 | TLS 指纹；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P1 | AIAsset 系生成代码（7 文件）<details><summary>7 个文件</summary>`backend/ent/aiasset/where.go`<br>`backend/ent/aiasset_update.go`<br>`backend/ent/aiasset_create.go`<br>`backend/ent/aiasset/aiasset.go`<br>`backend/ent/aiasset.go`<br>`backend/ent/aiasset_query.go`<br>`backend/ent/aiasset_delete.go`</details> | +8414/-0 | AI Studio/Skill；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P1 | AIAuditLog 系生成代码（7 文件）<details><summary>7 个文件</summary>`backend/ent/aiauditlog_create.go`<br>`backend/ent/aiauditlog/aiauditlog.go`<br>`backend/ent/aiauditlog.go`<br>`backend/ent/aiauditlog_query.go`<br>`backend/ent/aiauditlog/where.go`<br>`backend/ent/aiauditlog_update.go`<br>`backend/ent/aiauditlog_delete.go`</details> | +4101/-0 | AI Studio/Skill；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P1 | AIGenerationJob 系生成代码（7 文件）<details><summary>7 个文件</summary>`backend/ent/aigenerationjob/where.go`<br>`backend/ent/aigenerationjob_update.go`<br>`backend/ent/aigenerationjob_create.go`<br>`backend/ent/aigenerationjob/aigenerationjob.go`<br>`backend/ent/aigenerationjob.go`<br>`backend/ent/aigenerationjob_query.go`<br>`backend/ent/aigenerationjob_delete.go`</details> | +5959/-0 | AI Studio/Skill；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P1 | AIPromptTemplate 系生成代码（7 文件）<details><summary>7 个文件</summary>`backend/ent/aiprompttemplate/where.go`<br>`backend/ent/aiprompttemplate_update.go`<br>`backend/ent/aiprompttemplate_create.go`<br>`backend/ent/aiprompttemplate/aiprompttemplate.go`<br>`backend/ent/aiprompttemplate.go`<br>`backend/ent/aiprompttemplate_query.go`<br>`backend/ent/aiprompttemplate_delete.go`</details> | +6386/-0 | AI Studio/Skill；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P1 | AIPromptTemplateVersion 系生成代码（7 文件）<details><summary>7 个文件</summary>`backend/ent/aiprompttemplateversion_create.go`<br>`backend/ent/aiprompttemplateversion/aiprompttemplateversion.go`<br>`backend/ent/aiprompttemplateversion.go`<br>`backend/ent/aiprompttemplateversion_query.go`<br>`backend/ent/aiprompttemplateversion/where.go`<br>`backend/ent/aiprompttemplateversion_delete.go`<br>`backend/ent/aiprompttemplateversion_update.go`</details> | +4571/-0 | AI Studio/Skill；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P1 | AISession 系生成代码（7 文件）<details><summary>7 个文件</summary>`backend/ent/aisession_update.go`<br>`backend/ent/aisession_create.go`<br>`backend/ent/aisession/aisession.go`<br>`backend/ent/aisession.go`<br>`backend/ent/aisession_query.go`<br>`backend/ent/aisession/where.go`<br>`backend/ent/aisession_delete.go`</details> | +5125/-0 | AI Studio/Skill；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P1 | AISessionMessage 系生成代码（7 文件）<details><summary>7 个文件</summary>`backend/ent/aisessionmessage/where.go`<br>`backend/ent/aisessionmessage_update.go`<br>`backend/ent/aisessionmessage_create.go`<br>`backend/ent/aisessionmessage/aisessionmessage.go`<br>`backend/ent/aisessionmessage.go`<br>`backend/ent/aisessionmessage_query.go`<br>`backend/ent/aisessionmessage_delete.go`</details> | +5097/-0 | AI Studio/Skill；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P1 | AISkill 系生成代码（7 文件）<details><summary>7 个文件</summary>`backend/ent/aiskill_query.go`<br>`backend/ent/aiskill/where.go`<br>`backend/ent/aiskill_update.go`<br>`backend/ent/aiskill_create.go`<br>`backend/ent/aiskill/aiskill.go`<br>`backend/ent/aiskill.go`<br>`backend/ent/aiskill_delete.go`</details> | +9331/-0 | AI Studio/Skill；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P1 | AISkillInstall 系生成代码（7 文件）<details><summary>7 个文件</summary>`backend/ent/aiskillinstall/aiskillinstall.go`<br>`backend/ent/aiskillinstall.go`<br>`backend/ent/aiskillinstall/where.go`<br>`backend/ent/aiskillinstall_update.go`<br>`backend/ent/aiskillinstall_create.go`<br>`backend/ent/aiskillinstall_query.go`<br>`backend/ent/aiskillinstall_delete.go`</details> | +2495/-0 | AI Studio/Skill；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P1 | AISkillLike 系生成代码（7 文件）<details><summary>7 个文件</summary>`backend/ent/aiskilllike_create.go`<br>`backend/ent/aiskilllike/aiskilllike.go`<br>`backend/ent/aiskilllike.go`<br>`backend/ent/aiskilllike/where.go`<br>`backend/ent/aiskilllike_query.go`<br>`backend/ent/aiskilllike_update.go`<br>`backend/ent/aiskilllike_delete.go`</details> | +3487/-0 | AI Studio/Skill；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P1 | AISkillReview 系生成代码（7 文件）<details><summary>7 个文件</summary>`backend/ent/aiskillreview_update.go`<br>`backend/ent/aiskillreview_create.go`<br>`backend/ent/aiskillreview/aiskillreview.go`<br>`backend/ent/aiskillreview.go`<br>`backend/ent/aiskillreview_query.go`<br>`backend/ent/aiskillreview_delete.go`<br>`backend/ent/aiskillreview/where.go`</details> | +5342/-0 | AI Studio/Skill；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P1 | AISkillRun 系生成代码（7 文件）<details><summary>7 个文件</summary>`backend/ent/aiskillrun_update.go`<br>`backend/ent/aiskillrun_create.go`<br>`backend/ent/aiskillrun/aiskillrun.go`<br>`backend/ent/aiskillrun.go`<br>`backend/ent/aiskillrun_query.go`<br>`backend/ent/aiskillrun_delete.go`<br>`backend/ent/aiskillrun/where.go`</details> | +5542/-0 | AI Studio/Skill；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P1 | AISkillSettlement 系生成代码（7 文件）<details><summary>7 个文件</summary>`backend/ent/aiskillsettlement_update.go`<br>`backend/ent/aiskillsettlement_create.go`<br>`backend/ent/aiskillsettlement/aiskillsettlement.go`<br>`backend/ent/aiskillsettlement.go`<br>`backend/ent/aiskillsettlement_delete.go`<br>`backend/ent/aiskillsettlement_query.go`<br>`backend/ent/aiskillsettlement/where.go`</details> | +5941/-0 | AI Studio/Skill；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P1 | AISkillVersion 系生成代码（7 文件）<details><summary>7 个文件</summary>`backend/ent/aiskillversion/where.go`<br>`backend/ent/aiskillversion_update.go`<br>`backend/ent/aiskillversion_create.go`<br>`backend/ent/aiskillversion/aiskillversion.go`<br>`backend/ent/aiskillversion.go`<br>`backend/ent/aiskillversion_delete.go`<br>`backend/ent/aiskillversion_query.go`</details> | +7585/-0 | AI Studio/Skill；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P0 | ApiKey 系生成代码（5 文件）<details><summary>5 个文件</summary>`backend/ent/apikey_update.go`<br>`backend/ent/apikey/where.go`<br>`backend/ent/apikey_create.go`<br>`backend/ent/apikey/apikey.go`<br>`backend/ent/apikey.go`</details> | +684/-4 | API Key 安全存储；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P1 | ChannelMonitor 系生成代码（1 文件）<details><summary>1 个文件</summary>`backend/ent/channelmonitor/channelmonitor.go`</details> | +2/-1 | 渠道监控 v2；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P1 | ChannelMonitorRequestTemplate 系生成代码（1 文件）<details><summary>1 个文件</summary>`backend/ent/channelmonitorrequesttemplate/channelmonitorrequesttemplate.go`</details> | +2/-1 | 渠道监控 v2；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P0 | Group 系生成代码（5 文件）<details><summary>5 个文件</summary>`backend/ent/group_update.go`<br>`backend/ent/group_create.go`<br>`backend/ent/group/group.go`<br>`backend/ent/group.go`<br>`backend/ent/group/where.go`</details> | +3667/-717 | 核心/上游修改实体；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P1 | Invoice 系生成代码（7 文件）<details><summary>7 个文件</summary>`backend/ent/invoice/where.go`<br>`backend/ent/invoice_update.go`<br>`backend/ent/invoice_create.go`<br>`backend/ent/invoice/invoice.go`<br>`backend/ent/invoice.go`<br>`backend/ent/invoice_query.go`<br>`backend/ent/invoice_delete.go`</details> | +5996/-0 | 发票/支付；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P1 | InvoiceOrder 系生成代码（7 文件）<details><summary>7 个文件</summary>`backend/ent/invoiceorder/invoiceorder.go`<br>`backend/ent/invoiceorder.go`<br>`backend/ent/invoiceorder/where.go`<br>`backend/ent/invoiceorder_update.go`<br>`backend/ent/invoiceorder_query.go`<br>`backend/ent/invoiceorder_delete.go`<br>`backend/ent/invoiceorder_create.go`</details> | +2902/-0 | 发票/支付；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P1 | PaymentOrder 系生成代码（5 文件）<details><summary>5 个文件</summary>`backend/ent/paymentorder_update.go`<br>`backend/ent/paymentorder/where.go`<br>`backend/ent/paymentorder_create.go`<br>`backend/ent/paymentorder/paymentorder.go`<br>`backend/ent/paymentorder.go`</details> | +869/-3 | 发票/支付；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P1 | PaymentProviderInstance 系生成代码（5 文件）<details><summary>5 个文件</summary>`backend/ent/paymentproviderinstance/paymentproviderinstance.go`<br>`backend/ent/paymentproviderinstance.go`<br>`backend/ent/paymentproviderinstance/where.go`<br>`backend/ent/paymentproviderinstance_update.go`<br>`backend/ent/paymentproviderinstance_create.go`</details> | +136/-1 | 发票/支付；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P0 | Proxy 系生成代码（6 文件）<details><summary>6 个文件</summary>`backend/ent/proxy.go`<br>`backend/ent/proxy_update.go`<br>`backend/ent/proxy/where.go`<br>`backend/ent/proxy/proxy.go`<br>`backend/ent/proxy_create.go`<br>`backend/ent/proxy_query.go`</details> | +369/-34 | 核心/上游修改实体；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P0 | TLSFingerprintProfile 系生成代码（5 文件）<details><summary>5 个文件</summary>`backend/ent/tlsfingerprintprofile/where.go`<br>`backend/ent/tlsfingerprintprofile_create.go`<br>`backend/ent/tlsfingerprintprofile/tlsfingerprintprofile.go`<br>`backend/ent/tlsfingerprintprofile.go`<br>`backend/ent/tlsfingerprintprofile_update.go`</details> | +3810/-6 | TLS 指纹；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P0 | TLSFingerprintRouter 系生成代码（7 文件）<details><summary>7 个文件</summary>`backend/ent/tlsfingerprintrouter/tlsfingerprintrouter.go`<br>`backend/ent/tlsfingerprintrouter.go`<br>`backend/ent/tlsfingerprintrouter/where.go`<br>`backend/ent/tlsfingerprintrouter_update.go`<br>`backend/ent/tlsfingerprintrouter_query.go`<br>`backend/ent/tlsfingerprintrouter_create.go`<br>`backend/ent/tlsfingerprintrouter_delete.go`</details> | +2621/-0 | TLS 指纹；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P0 | UsageLog 系生成代码（5 文件）<details><summary>5 个文件</summary>`backend/ent/usagelog_update.go`<br>`backend/ent/usagelog/where.go`<br>`backend/ent/usagelog_create.go`<br>`backend/ent/usagelog/usagelog.go`<br>`backend/ent/usagelog.go`</details> | +1202/-308 | 核心/上游修改实体；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P0 | User 系生成代码（6 文件）<details><summary>6 个文件</summary>`backend/ent/user.go`<br>`backend/ent/user_update.go`<br>`backend/ent/user/where.go`<br>`backend/ent/user/user.go`<br>`backend/ent/user_create.go`<br>`backend/ent/user_query.go`</details> | +3594/-342 | 核心/上游修改实体；随 `go generate ./ent` 重新生成，不手工迁移 | ☐ |
| P0 | ent 运行时/公共生成代码（9 文件）<details><summary>9 个文件</summary>`backend/ent/runtime/runtime.go`<br>`backend/ent/migrate/schema.go`<br>`backend/ent/hook/hook.go`<br>`backend/ent/client.go`<br>`backend/ent/ent.go`<br>`backend/ent/predicate/predicate.go`<br>`backend/ent/intercept/intercept.go`<br>`backend/ent/tx.go`<br>`backend/ent/mutation.go`</details> | +67010/-26538 | client/mutation/tx/predicate/hook/intercept/runtime/migrate·schema，随 `go generate ./ent` 生成 | ☐ |
| P0 | `backend/ent/migrate/ai_skill_center_test.go` | +67/-0 | 手写迁移契约测试（非生成，需迁移） | ☐ |
| P0 | `backend/ent/migrate/auth_identity_fk_ondelete_test.go` | +33/-0 | 手写迁移契约测试（非生成，需迁移） | ☐ |

## 3. migrations 数据库迁移（建议批次 0，整体重要度 P0）

SQL 迁移与 schema 一一配套，取舍跟随对应功能模块。**编号碰撞处理策略：保留上游编号不动，personal-dev 独有迁移从 personal-main 当前最大编号之后重新编号**，并存档新旧编号映射表；重编号后需同步 `schema_migrations` 校验和（配合 sync_checksums 工具）。纯新增区间可按序续接。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | 修改的上游迁移（7 文件）<details><summary>7 个文件</summary>`backend/migrations/125_add_channel_monitors.sql` (+3/-3)<br>`backend/migrations/126_add_channel_monitor_aggregation.sql` (+4/-4)<br>`backend/migrations/131_affiliate_rebate_hardening.sql` (+49/-10)<br>`backend/migrations/157_user_platform_quotas_add_grok.sql` (+0/-16)<br>`backend/migrations/173_allow_cyber_blocked_usage_request_type.sql` (+0/-8)<br>`backend/migrations/174_group_web_search_price_per_call.sql` (+0/-3)<br>`backend/migrations/176_channel_monitor_grok_provider.sql` (+0/-39)</details> | +56/-83 | 直接改到已存在的上游迁移文件（渠道监控/联盟/grok 枚举/请求类型），需以补丁方式合入上游对应文件，不新建编号 | ☐ |
| P3 | 上游迁移被重编号（25 文件）<details><summary>25 个文件</summary>`backend/migrations/161_add_opus48_to_model_mapping.sql` (+0/-0)<br>`backend/migrations/162_deleted_api_key_audit.sql` (+0/-0)<br>`backend/migrations/163_ops_metrics_ttft_sample_count.sql` (+0/-0)<br>`backend/migrations/164_ops_error_log_api_key_prefix.sql` (+0/-0)<br>`backend/migrations/165_add_ops_error_logs_user_time_index_notx.sql` (+1/-1)<br>`backend/migrations/166_proxy_expiry_fallback.sql` (+0/-0)<br>`backend/migrations/167_account_group_scheduler_indexes_notx.sql` (+0/-0)<br>`backend/migrations/204_add_usage_log_long_context_billing.sql` (+0/-0)<br>`backend/migrations/205_default_openai_long_context_billing.sql` (+0/-0)<br>`backend/migrations/206_add_ops_system_logs_host.sql` (+0/-0)<br>`backend/migrations/206a_add_ops_system_logs_host_index_notx.sql` (+0/-0)<br>`backend/migrations/211_add_usage_logs_api_key_latest_ip_index_notx.sql` (+0/-0)<br>`backend/migrations/228_composite_model_routes.sql` (+1/-1)<br>`backend/migrations/229_prompt_audit.sql` (+0/-0)<br>`backend/migrations/230_prompt_audit_full_prompt.sql` (+0/-0)<br>`backend/migrations/231_ops_ingress_reject_aggregates.sql` (+0/-0)<br>`backend/migrations/232_auth_cache_invalidation_outbox.sql` (+0/-0)<br>`backend/migrations/233_group_reasoning_effort_policy.sql` (+0/-0)<br>`backend/migrations/234_alipay_mobile_precreate_deep_link.sql` (+0/-0)<br>`backend/migrations/235_group_auth_cache_image_generation.sql` (+0/-0)<br>`backend/migrations/236_add_usage_log_session_id.sql` (+0/-0)<br>`backend/migrations/237_allow_live_usage_request_type.sql` (+1/-1)<br>`backend/migrations/238_add_group_allow_live.sql` (+0/-0)<br>`backend/migrations/239_add_users_email_alias_dedup_index_notx.sql` (+0/-0)<br>`backend/migrations/240_passkey_credentials.sql` (+0/-0)</details> | +3/-3 | 上游自身迁移被 personal-dev 挪号（如 144→161）；迁到 personal-main 应保留上游原编号，这些重命名条目本身建议放弃 | ☐ |
| P0 | personal-dev 新增·编号碰撞区间(128–220)（81 文件）<details><summary>81 个文件</summary>`backend/migrations/128_add_support_tickets.sql` (+88/-0)<br>`backend/migrations/132_affiliate_policy_limits.sql` (+151/-0)<br>`backend/migrations/133_add_user_token_version.sql` (+2/-0)<br>`backend/migrations/134_align_channel_monitor_request_template_constraints.sql` (+52/-0)<br>`backend/migrations/135_clean_channel_monitor_soft_deleted_rows.sql` (+40/-0)<br>`backend/migrations/136_align_channel_monitor_template_index_and_cleanup.sql` (+47/-0)<br>`backend/migrations/137_align_channel_monitor_core_indexes.sql` (+45/-0)<br>`backend/migrations/137_subscription_fulfillment_claim_dedupe.sql` (+51/-0)<br>`backend/migrations/138_subscription_fulfillment_claim_unique_notx.sql` (+27/-0)<br>`backend/migrations/139_add_group_images2api_pricing.sql` (+7/-0)<br>`backend/migrations/140_add_group_display_name_and_user_selectable.sql` (+9/-0)<br>`backend/migrations/140_create_media_assets.sql` (+45/-0)<br>`backend/migrations/141_add_media_thumbnail_mime_type.sql` (+4/-0)<br>`backend/migrations/142_add_media_storage_profile_id.sql` (+8/-0)<br>`backend/migrations/142_create_ai_skill_center.sql` (+263/-0)<br>`backend/migrations/143_create_ai_skill_installs.sql` (+14/-0)<br>`backend/migrations/144_create_ai_center_core.sql` (+374/-0)<br>`backend/migrations/145_seed_client_spoof_channel_monitor_templates.sql` (+164/-0)<br>`backend/migrations/147_usage_log_higher_priced_upstream.sql` (+4/-0)<br>`backend/migrations/148_expand_usage_log_request_type_check.sql` (+18/-0)<br>`backend/migrations/149_add_group_image_generation_route.sql` (+41/-0)<br>`backend/migrations/150_add_invoice_applications.sql` (+51/-0)<br>`backend/migrations/151_apply_rpm_parallel_constraints_and_replace_claude_code_template.sql` (+118/-0)<br>`backend/migrations/152_add_group_openai_image_main_model.sql` (+2/-0)<br>`backend/migrations/152_payment_refund_self_service.sql` (+11/-0)<br>`backend/migrations/153_add_dashboard_billing_split_costs.sql` (+40/-0)<br>`backend/migrations/154_add_usage_logs_user_created_at_covering_index_notx.sql` (+3/-0)<br>`backend/migrations/155_add_ticket_message_attachments.sql` (+2/-0)<br>`backend/migrations/156_user_platform_quotas_add_kiro.sql` (+12/-0)<br>`backend/migrations/157_create_invoices_and_invoice_orders.sql` (+73/-0)<br>`backend/migrations/158_backfill_invoices_from_applications.sql` (+117/-0)<br>`backend/migrations/159_ai_skill_versions_add_approved_artifact_digest.sql` (+18/-0)<br>`backend/migrations/160_add_usage_log_openai_ws_profile.sql` (+4/-0)<br>`backend/migrations/161_channel_monitor_add_kiro_provider.sql` (+13/-0)<br>`backend/migrations/162_create_codex_invite_reset_history.sql` (+19/-0)<br>`backend/migrations/162_create_tls_fingerprint_routers.sql` (+13/-0)<br>`backend/migrations/168_add_usage_log_provider.sql` (+3/-0)<br>`backend/migrations/169_align_group_display_name_length.sql` (+15/-0)<br>`backend/migrations/170_ai_skill_creator_earnings_source_run.sql` (+13/-0)<br>`backend/migrations/171_tls_fingerprint_capture_tasks.sql` (+51/-0)<br>`backend/migrations/172_cleanup_synthetic_tls_fingerprint_seed.sql` (+32/-0)<br>`backend/migrations/173_tls_fingerprint_parametric_extensions.sql` (+14/-0)<br>`backend/migrations/174_tls_fingerprint_capture_sample_native_columns.sql` (+11/-0)<br>`backend/migrations/175_add_tls_fingerprint_profile_ua_originator.sql` (+12/-0)<br>`backend/migrations/176_add_tls_fingerprint_profile_transport.sql` (+1/-0)<br>`backend/migrations/176_tls_fingerprint_capture_unification.sql` (+286/-0)<br>`backend/migrations/177_tls_fingerprint_profile_replay_fields.sql` (+11/-0)<br>`backend/migrations/178_add_tls_fingerprint_capture_session_event_raw_payload.sql` (+7/-0)<br>`backend/migrations/179_align_tls_fingerprint_capture_sample_uniqueness.sql` (+25/-0)<br>`backend/migrations/180_update_tls_fingerprint_capture_sample_comments.sql` (+13/-0)<br>`backend/migrations/181_user_platform_quotas_add_grok.sql` (+10/-0)<br>`backend/migrations/182_channel_monitor_add_grok_provider.sql` (+13/-0)<br>`backend/migrations/183_add_group_video_pricing.sql` (+22/-0)<br>`backend/migrations/184_add_group_audio_search_pricing.sql` (+17/-0)<br>`backend/migrations/185_expand_usage_log_request_type_check.sql` (+18/-0)<br>`backend/migrations/186_add_tls_fingerprint_profile_os_client_type.sql` (+19/-0)<br>`backend/migrations/187_allow_native_image_route_and_video_price_checks.sql` (+52/-0)<br>`backend/migrations/188_add_group_audio_search_price_checks.sql` (+45/-0)<br>`backend/migrations/189_update_ops_error_request_type_comment.sql` (+4/-0)<br>`backend/migrations/190_clear_non_grok_video_generation_config.sql` (+19/-0)<br>`backend/migrations/191_add_user_affiliate_ledger_reverse_action_unique_notx.sql` (+18/-0)<br>`backend/migrations/191_restore_usage_request_type_cyber_value.sql` (+53/-0)<br>`backend/migrations/192_audit_retention_created_at_indexes_notx.sql` (+21/-0)<br>`backend/migrations/192_ops_sticky_schedule_events.sql` (+24/-0)<br>`backend/migrations/193_add_usage_log_video_billing_details.sql` (+5/-0)<br>`backend/migrations/193_add_usage_log_video_billing_details_index_notx.sql` (+3/-0)<br>`backend/migrations/193_create_usage_user_daily_cost.sql` (+21/-0)<br>`backend/migrations/194_add_tls_fingerprint_profile_http2_fingerprint.sql` (+5/-0)<br>`backend/migrations/195_add_invoice_order_active_unique_guard.sql` (+47/-0)<br>`backend/migrations/196_validate_hot_table_check_constraints.sql` (+17/-0)<br>`backend/migrations/197_validate_post_release_non_negative_checks.sql` (+20/-0)<br>`backend/migrations/198_widen_batch_image_wallet_precision.sql` (+5/-0)<br>`backend/migrations/199_batch_image_idempotency_unique.sql` (+16/-0)<br>`backend/migrations/200_add_payment_fulfillment_lease_token.sql` (+5/-0)<br>`backend/migrations/201_balance_cache_outbox.sql` (+34/-0)<br>`backend/migrations/202_transactional_scheduler_outbox_triggers.sql` (+161/-0)<br>`backend/migrations/203_group_web_search_price_per_call.sql` (+3/-0)<br>`backend/migrations/217_add_openai_ws_usage_observability.sql` (+19/-0)<br>`backend/migrations/218_affiliate_inviter_bound_at.sql` (+9/-0)<br>`backend/migrations/219_add_media_public_base_url.sql` (+24/-0)<br>`backend/migrations/220_create_backup_records.sql` (+63/-0)</details> | +3236/-0 | ⚠️ 与上游同号不同内容，**必须从 personal-main 当前最大编号之后重新编号**并存档新旧映射，配合 sync_checksums 重算校验和 | ☐ |
| P0 | personal-dev 新增·续接区间(≥221)（43 文件）<details><summary>43 个文件</summary>`backend/migrations/146_add_ai_studio_feature_flag.sql` (+3/-0)<br>`backend/migrations/148a_validate_usage_log_request_type_check.sql` (+2/-0)<br>`backend/migrations/185a_validate_usage_log_request_type_check.sql` (+2/-0)<br>`backend/migrations/191a_validate_usage_log_request_type_check.sql` (+2/-0)<br>`backend/migrations/195a_add_invoice_order_active_unique_guard_notx.sql` (+9/-0)<br>`backend/migrations/199a_batch_image_idempotency_unique_notx.sql` (+8/-0)<br>`backend/migrations/207_ip_multi_account_security.sql` (+73/-0)<br>`backend/migrations/208_scheduler_outbox_claims.sql` (+3/-0)<br>`backend/migrations/209_ai_skill_balance_ledger.sql` (+30/-0)<br>`backend/migrations/210_scheduler_outbox_claim_indexes_notx.sql` (+7/-0)<br>`backend/migrations/212_seed_tls_fingerprint_dimension_router_packs.sql` (+60/-0)<br>`backend/migrations/213_backfill_usage_log_video_seconds.sql` (+10/-0)<br>`backend/migrations/214_openai_oauth_capacity_history.sql` (+72/-0)<br>`backend/migrations/215_openai_oauth_capacity_hourly.sql` (+38/-0)<br>`backend/migrations/216_protect_user_api_keys.sql` (+33/-0)<br>`backend/migrations/221_add_tls_fingerprint_capture_task_lease.sql` (+15/-0)<br>`backend/migrations/222_add_tls_fingerprint_router_platform.sql` (+24/-0)<br>`backend/migrations/223_account_capacity_forecast.sql` (+154/-0)<br>`backend/migrations/224_add_usage_log_tls_fingerprint.sql` (+6/-0)<br>`backend/migrations/225_add_tls_fingerprint_profile_lifecycle.sql` (+18/-0)<br>`backend/migrations/226_add_account_tls_fingerprint_policy.sql` (+39/-0)<br>`backend/migrations/227_add_tls_fingerprint_http_header_template.sql` (+11/-0)<br>`backend/migrations/241_validate_usage_log_request_type_check.sql` (+2/-0)<br>`backend/migrations/242_coalesce_scheduler_trigger_events.sql` (+70/-0)<br>`backend/migrations/243_fix_protected_api_key_invalidation.sql` (+169/-0)<br>`backend/migrations/244_support_ticket_unread_badge_indexes_notx.sql` (+7/-0)<br>`backend/migrations/245_channel_monitor_v2.sql` (+119/-0)<br>`backend/migrations/246_invoice_admin_reminder.sql` (+5/-0)<br>`backend/migrations/247_invoice_admin_reminder_index_notx.sql` (+3/-0)<br>`backend/migrations/248_ticket_reminder_cleanup.sql` (+5/-0)<br>`backend/migrations/249_ticket_reminder_indexes_notx.sql` (+12/-0)<br>`backend/migrations/250_ai_assets_lifecycle.sql` (+25/-0)<br>`backend/migrations/251_studio_production_projects.sql` (+54/-0)<br>`backend/migrations/252_studio_storage_charges.sql` (+37/-0)<br>`backend/migrations/255_group_storage_price_per_gb_day.sql` (+19/-0)<br>`backend/migrations/256_validate_group_storage_price_check.sql` (+2/-0)<br>`backend/migrations/257_channel_monitor_mode.sql` (+5/-0)<br>`backend/migrations/258_channel_monitor_v2_ignored_error_categories.sql` (+3/-0)<br>`backend/migrations/259_channel_monitor_v2_seed_popular_models.sql` (+139/-0)<br>`backend/migrations/260_channel_monitor_v2_health_thresholds.sql` (+33/-0)<br>`backend/migrations/261_channel_monitor_v2_fixed_rollups.sql` (+95/-0)<br>`backend/migrations/262_channel_monitor_v2_rollup_permissions.sql` (+15/-0)<br>`backend/migrations/263_channel_monitor_v2_refresh_5m.sql` (+6/-0)</details> | +1444/-0 | personal-dev 纯新增、编号不与上游冲突，可按序续接（重编号后仍需保证相对顺序） | ☐ |
| P0 | `backend/migrations/auth_identity_payment_migrations_regression_test.go` | +244/-30 | 迁移安全/契约回归测试（重编号后需同步更新期望编号） | ☐ |
| P0 | `backend/migrations/backup_records_migration_test.go` | +22/-0 | 迁移安全/契约回归测试（重编号后需同步更新期望编号） | ☐ |
| P0 | `backend/migrations/channel_monitor_grok_provider_migration_test.go` | +0/-20 | 迁移安全/契约回归测试（重编号后需同步更新期望编号） | ☐ |
| P0 | `backend/migrations/invoice_backfill_contract_test.go` | +232/-0 | 迁移安全/契约回归测试（重编号后需同步更新期望编号） | ☐ |
| P0 | `backend/migrations/latest_api_key_ip_index_test.go` | +1/-1 | 迁移安全/契约回归测试（重编号后需同步更新期望编号） | ☐ |
| P0 | `backend/migrations/migration_safety_contract_test.go` | +319/-0 | 迁移安全/契约回归测试（重编号后需同步更新期望编号） | ☐ |
| P0 | `backend/migrations/openai_long_context_billing_migration_test.go` | +2/-2 | 迁移安全/契约回归测试（重编号后需同步更新期望编号） | ☐ |
| P2 | `backend/migrations/README.md` | +24/-0 | 迁移目录说明文档 | ☐ |

## 4. 迁移执行器与校验工具（建议批次 0，整体重要度 P0）

migrations 运行器的增强（checksum 校验、notx 迁移、安全契约测试）与配套 CLI。是重编号策略能落地的前提，应与批次 0 同批迁移。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `backend/cmd/sync_checksums/main.go` · `backend/cmd/sync_checksums/main_test.go` | +448/-0 | 实现（含测试） | ☐ |
| P0 | `backend/internal/repository/migrations_runner.go` | +292/-72 | 实现 | ☐ |
| P0 | `backend/internal/repository/cache_sync_outbox_migration_integration_test.go` | +89/-0 | 集成测试 | ☐ |
| P0 | `backend/internal/repository/migrations_runner_checksum_test.go` | +156/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/migrations_runner_extra_test.go` | +211/-46 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/migrations_runner_notx_test.go` | +126/-34 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/migrations_schema_integration_test.go` | +225/-83 | 集成测试 | ☐ |
| P0 | `backend/internal/repository/openai_long_context_billing_migration_integration_test.go` | +3/-1 | 集成测试 | ☐ |

## 5. 用量计费与调度缓存底座（建议批次 2，整体重要度 P0）

所有网关请求闭环的必经环节：用量日志入库/查询/统计（personal-dev 把上游拆分的 usage_log_repo_* 合并回单文件并大幅重写）、计费缓存、余额 outbox、调度器缓存与事务性 outbox。与 service 层 Billing-Usage 模块同批迁移；行数大但内聚，建议整体搬运后跑集成测试。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `backend/internal/handler/usage_handler.go` | +2/-1 | handler | ☐ |
| P0 | `backend/internal/handler/usage_record_task_context.go` | +43/-0 | 实现 | ☐ |
| P0 | `backend/internal/pkg/usagestats/usage_log_types.go` | +59/-68 | 实现 | ☐ |
| P0 | `backend/internal/repository/balance_cache_outbox_repo.go` | +76/-0 | 仓储实现 | ☐ |
| P0 | `backend/internal/repository/billing_cache.go` | +219/-14 | 缓存 | ☐ |
| P0 | `backend/internal/repository/scheduler_cache.go` · `backend/internal/repository/scheduler_cache_test.go` | +557/-272 | 缓存（含测试） | ☐ |
| P0 | `backend/internal/repository/scheduler_outbox_repo.go` · `backend/internal/repository/scheduler_outbox_repo_test.go` | +218/-209 | 仓储实现（含测试） | ☐ |
| P0 | `backend/internal/repository/usage_billing_repo.go` | +44/-6 | 仓储实现 | ☐ |
| P0 | `backend/internal/repository/usage_cleanup_repo.go` · `backend/internal/repository/usage_cleanup_repo_test.go` | +27/-2 | 仓储实现（含测试） | ☐ |
| P0 | `backend/internal/repository/usage_log_repo.go` | +5079/-63 | 仓储实现 | ☐ |
| P0 | `backend/internal/repository/usage_log_repo_dashboard.go` | +0/-628 | 实现 | ☐ |
| P0 | `backend/internal/repository/usage_log_repo_insert.go` | +0/-1362 | 实现 | ☐ |
| P0 | `backend/internal/repository/usage_log_repo_query.go` | +0/-772 | 实现 | ☐ |
| P0 | `backend/internal/repository/usage_log_repo_stats.go` | +0/-1148 | 实现 | ☐ |
| P0 | `backend/internal/repository/usage_log_repo_trend.go` | +0/-826 | 实现 | ☐ |
| P0 | `backend/internal/repository/usage_log_sticky_schedule.go` | +54/-0 | 实现 | ☐ |
| P0 | `backend/internal/repository/usage_user_daily_cost_repo.go` · `backend/internal/repository/usage_user_daily_cost_repo_test.go` | +331/-0 | 仓储实现（含测试） | ☐ |
| P0 | `backend/resources/model-pricing/model_prices_and_context_window.json` | +241/-74 | 实现 | ☐ |
| P0 | `backend/internal/handler/usage_handler_daily_test.go` | +1/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/usage_handler_request_type_test.go` | +5/-3 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/usage_record_submit_task_test.go` | +49/-3 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/usage_record_task_fallback_test.go` | +0/-77 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/billing_cache_subscription_fence_test.go` | +58/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/scheduler_cache_integration_test.go` | +372/-8 | 集成测试 | ☐ |
| P0 | `backend/internal/repository/scheduler_cache_last_used_unit_test.go` | +9/-38 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/scheduler_cache_unit_test.go` | +82/-317 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/usage_billing_repo_batch_image_hold_test.go` | +85/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/usage_billing_repo_integration_test.go` | +74/-0 | 集成测试 | ☐ |
| P0 | `backend/internal/repository/usage_billing_repo_unit_test.go` | +74/-115 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/usage_log_repo_breakdown_test.go` | +2/-2 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/usage_log_repo_integration_test.go` | +465/-35 | 集成测试 | ☐ |
| P0 | `backend/internal/repository/usage_log_repo_request_type_test.go` | +312/-34 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/usage_log_repo_unit_test.go` | +95/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/usage_log_session_id_unit_test.go` | +8/-8 | 单元测试 | ☐ |

## 6. 账号核心与上游转发底座（建议批次 2，整体重要度 P0）

账号仓储、并发槽位缓存、临时下线计数、RPM 限流缓存与 HTTP 上游转发核心（http_upstream 大改）。被所有平台网关依赖，必须在平台批次之前落地。与 service 层 Account-Core / Scheduling-TempUnsched 同批。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `backend/internal/guard/cloudflare_backoff.go` | +60/-0 | 实现 | ☐ |
| P0 | `backend/internal/model/error_passthrough_rule.go` | +18/-9 | 实现 | ☐ |
| P0 | `backend/internal/pkg/httpclient/pool.go` · `backend/internal/pkg/httpclient/pool_test.go` | +793/-104 | 实现（含测试） | ☐ |
| P0 | `backend/internal/pkg/reqclientpool/reqclient_pool.go` | +82/-0 | 实现 | ☐ |
| P0 | `backend/internal/repository/account_repo.go` | +203/-96 | 仓储实现 | ☐ |
| P0 | `backend/internal/repository/concurrency_cache.go` | +212/-203 | 缓存 | ☐ |
| P0 | `backend/internal/repository/gateway_cache.go` | +176/-48 | 缓存 | ☐ |
| P0 | `backend/internal/repository/http_upstream.go` · `backend/internal/repository/http_upstream_test.go` | +1198/-240 | 实现（含测试） | ☐ |
| P0 | `backend/internal/repository/req_client_pool.go` · `backend/internal/repository/req_client_pool_test.go` | +55/-106 | 实现（含测试） | ☐ |
| P0 | `backend/internal/repository/scheduled_test_repo.go` | +5/-0 | 仓储实现 | ☐ |
| P0 | `backend/internal/repository/temp_unsched_counter_cache.go` · `backend/internal/repository/temp_unsched_counter_cache_test.go` | +513/-0 | 缓存（含测试） | ☐ |
| P0 | `backend/internal/repository/user_rpm_cache.go` | +393/-32 | 缓存 | ☐ |
| P0 | `backend/internal/repository/account_repo_auto_pause_test.go` | +30/-2 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/account_repo_integration_test.go` | +23/-0 | 集成测试 | ☐ |
| P0 | `backend/internal/repository/account_repo_ollama_cloud_usage_test.go` | +2/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/account_repo_temp_unsched_test.go` | +19/-12 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/account_repo_threshold_snapshot_test.go` | +36/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/account_repo_upstream_billing_probe_cas_test.go` | +5/-196 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/account_repo_upstream_billing_probe_due_test.go` | +2/-2 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/account_repo_upstream_billing_probe_update_test.go` | +5/-2 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/concurrency_cache_integration_test.go` | +178/-54 | 集成测试 | ☐ |
| P0 | `backend/internal/repository/gateway_cache_integration_test.go` | +20/-3 | 集成测试 | ☐ |
| P0 | `backend/internal/repository/gateway_cache_preemption_test.go` | +108/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/http_upstream_benchmark_test.go` | +1/-1 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/http_upstream_dial_test.go` | +130/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/http_upstream_dial_timeout_test.go` | +5/-6 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/http_upstream_http2_keepalive_test.go` | +3/-3 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/http_upstream_ops_failure_test.go` | +106/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/upstream_billing_probe_persistence_integration_test.go` | +2/-0 | 集成测试 | ☐ |
| P0 | `backend/internal/repository/user_rpm_cache_integration_test.go` | +147/-0 | 集成测试 | ☐ |

## 7. 用户、分组与平台配额（建议批次 2，整体重要度 P0）

用户/分组仓储重写（余额精度、邀请原子性、批量删除）、用户平台配额（多平台维度）、前台用户与可用渠道接口。是计费与调度的数据基础，建议早期整体迁移。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `backend/internal/handler/available_channel_handler.go` · `backend/internal/handler/available_channel_handler_test.go` | +125/-60 | handler（含测试） | ☐ |
| P0 | `backend/internal/handler/user_handler.go` · `backend/internal/handler/user_handler_test.go` | +300/-11 | handler（含测试） | ☐ |
| P0 | `backend/internal/repository/group_repo.go` | +117/-96 | 仓储实现 | ☐ |
| P0 | `backend/internal/repository/simple_mode_default_groups.go` | +5/-0 | 实现 | ☐ |
| P0 | `backend/internal/repository/user_group_rate_repo.go` | +19/-4 | 仓储实现 | ☐ |
| P0 | `backend/internal/repository/user_platform_quota_repo.go` | +151/-12 | 仓储实现 | ☐ |
| P0 | `backend/internal/repository/user_platform_quota_service_adapter.go` | +35/-0 | 实现 | ☐ |
| P0 | `backend/internal/repository/user_repo.go` | +915/-170 | 仓储实现 | ☐ |
| P0 | `backend/internal/handler/dto/group_video_mapper_test.go` | +72/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/group_entity_video_test.go` | +32/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/group_repo_duplicate_integration_test.go` | +109/-0 | 集成测试 | ☐ |
| P0 | `backend/internal/repository/group_repo_integration_test.go` | +212/-16 | 集成测试 | ☐ |
| P0 | `backend/internal/repository/simple_mode_default_groups_integration_test.go` | +2/-94 | 集成测试 | ☐ |
| P0 | `backend/internal/repository/user_avatar_list_test.go` | +39/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/user_platform_quota_adapter_test.go` | +3/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/user_platform_quota_repo_integration_test.go` | +46/-0 | 集成测试 | ☐ |
| P0 | `backend/internal/repository/user_platform_quota_repo_snapshot_test.go` | +58/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/user_platform_quota_upsert_test.go` | +9/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/user_repo_batch_image_delete_integration_test.go` | +149/-0 | 集成测试 | ☐ |
| P0 | `backend/internal/repository/user_repo_delete_atomicity_integration_test.go` | +1/-1 | 集成测试 | ☐ |
| P0 | `backend/internal/repository/user_repo_email_lookup_unit_test.go` | +28/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/user_repo_integration_test.go` | +83/-35 | 集成测试 | ☐ |
| P0 | `backend/internal/repository/user_repo_invitation_atomicity_test.go` | +115/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/user_repo_sort_integration_test.go` | +172/-2 | 集成测试 | ☐ |

## 8. API Key 安全存储与鉴权缓存（建议批次 2，整体重要度 P0）

API Key 存储安全模型重构（lookup_hash + AES-256-GCM 密文 + 前缀展示），属于安全回退类 P0：不迁移则新库仍以明文/弱哈希存 key。含鉴权缓存失效 outbox、OAuth 会话 Redis 存储等基础设施。迁移需连同 migrations 216/243 与 schema api_key.go 同批。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `backend/internal/handler/admin/apikey_handler.go` | +1/-1 | handler | ☐ |
| P0 | `backend/internal/handler/api_key_handler.go` | +134/-11 | handler | ☐ |
| P0 | `backend/internal/pkg/oauth/oauth.go` · `backend/internal/pkg/oauth/oauth_test.go` | +162/-14 | OAuth（含测试） | ☐ |
| P0 | `backend/internal/pkg/redissession/store.go` · `backend/internal/pkg/redissession/store_test.go` | +86/-25 | 实现（含测试） | ☐ |
| P0 | `backend/internal/repository/api_key_repo.go` | +468/-176 | 仓储实现 | ☐ |
| P0 | `backend/internal/server/middleware/api_key_auth.go` · `backend/internal/server/middleware/api_key_auth_test.go` | +68/-17 | 中间件（含测试） | ☐ |
| P0 | `backend/internal/server/middleware/api_key_auth_google.go` · `backend/internal/server/middleware/api_key_auth_google_test.go` | +7/-19 | 中间件（含测试） | ☐ |
| P0 | `backend/internal/handler/api_key_handler_reveal_test.go` | +60/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/dto/api_key_mapper_last_used_test.go` | +12/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/dto/api_key_mapper_mask_test.go` | +36/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/oauth/oauth_redis_fallback_test.go` | +39/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/api_key_repo_integration_test.go` | +24/-27 | 集成测试 | ☐ |
| P0 | `backend/internal/repository/api_key_repo_messages_dispatch_unit_test.go` | +143/-12 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/api_key_repo_quota_unit_test.go` | +33/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/api_key_secret_protection_unit_test.go` | +122/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/auth_cache_invalidation_outbox_repo_test.go` | +1/-1 | 单元测试 | ☐ |

## 9. TLS 指纹反检测（建议批次 2，整体重要度 P0）

反封号核心：uTLS 拨号器、ClientHello 解析（JA3/JA4）、H1/H2 指纹回放、指纹路由与账号级策略绑定。本体独立性好（几乎纯新增），但被各平台请求构造调用，建议在平台网关批次之前落地。放弃则显著提高上游封号风险。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `backend/internal/handler/admin/tls_fingerprint_profile_handler.go` · `backend/internal/handler/admin/tls_fingerprint_profile_handler_test.go` | +562/-50 | handler（含测试） | ☐ |
| P0 | `backend/internal/handler/admin/tls_fingerprint_router_handler.go` · `backend/internal/handler/admin/tls_fingerprint_router_handler_test.go` | +303/-0 | handler（含测试） | ☐ |
| P0 | `backend/internal/model/account_tls_fingerprint_policy.go` | +37/-0 | 实现 | ☐ |
| P0 | `backend/internal/model/http_header_template.go` · `backend/internal/model/http_header_template_test.go` | +140/-0 | 实现（含测试） | ☐ |
| P0 | `backend/internal/model/tls_fingerprint_profile.go` · `backend/internal/model/tls_fingerprint_profile_test.go` | +596/-26 | 实现（含测试） | ☐ |
| P0 | `backend/internal/model/tls_fingerprint_router.go` | +122/-0 | 实现 | ☐ |
| P0 | `backend/internal/pkg/tlsfingerprint/dialer.go` | +298/-38 | 实现 | ☐ |
| P0 | `backend/internal/pkg/tlsfingerprint/http2/fingerprint.go` · `backend/internal/pkg/tlsfingerprint/http2/fingerprint_test.go` | +403/-0 | 实现（含测试） | ☐ |
| P0 | `backend/internal/pkg/tlsfingerprint/parser/client_hello_parser.go` · `backend/internal/pkg/tlsfingerprint/parser/client_hello_parser_test.go` | +605/-0 | 实现（含测试） | ☐ |
| P0 | `backend/internal/pkg/tlsfingerprint/parser/ja3_ja4.go` · `backend/internal/pkg/tlsfingerprint/parser/ja3_ja4_test.go` | +357/-0 | 实现（含测试） | ☐ |
| P0 | `backend/internal/pkg/tlsfingerprint/parser/observed_client_hello.go` | +39/-0 | 实现 | ☐ |
| P0 | `backend/internal/pkg/tlsfingerprint/replay/replay_profile.go` | +64/-0 | 实现 | ☐ |
| P0 | `backend/internal/pkg/tlsfingerprint/replay/replay_projection.go` · `backend/internal/pkg/tlsfingerprint/replay/replay_projection_test.go` | +490/-0 | 实现（含测试） | ☐ |
| P0 | `backend/internal/pkg/tlsfingerprint/transport/types.go` | +10/-0 | 实现 | ☐ |
| P0 | `backend/internal/repository/account_tls_fingerprint_policy_repo.go` · `backend/internal/repository/account_tls_fingerprint_policy_repo_test.go` | +413/-0 | 仓储实现（含测试） | ☐ |
| P0 | `backend/internal/repository/http_upstream_h1_replay.go` · `backend/internal/repository/http_upstream_h1_replay_test.go` | +312/-0 | 实现（含测试） | ☐ |
| P0 | `backend/internal/repository/http_upstream_h2_fingerprint_replay.go` · `backend/internal/repository/http_upstream_h2_fingerprint_replay_test.go` | +713/-0 | 实现（含测试） | ☐ |
| P0 | `backend/internal/repository/tls_fingerprint_profile_repo.go` | +227/-24 | 仓储实现 | ☐ |
| P0 | `backend/internal/repository/tls_fingerprint_router_cache.go` | +114/-0 | 缓存 | ☐ |
| P0 | `backend/internal/repository/tls_fingerprint_router_repo.go` | +446/-0 | 仓储实现 | ☐ |
| P0 | `backend/internal/handler/admin/account_tls_fingerprint_preview_handler_test.go` | +42/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/tlsfingerprint/dialer_integration_test.go` | +4/-3 | 集成测试 | ☐ |
| P0 | `backend/internal/pkg/tlsfingerprint/dialer_proxy_test.go` | +274/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/tlsfingerprint/dialer_spec_test.go` | +109/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/http_upstream_h2_test.go` | +284/-0 | 单元测试 | ☐ |

## 10. 内容审核（建议批次 2，整体重要度 P1）

请求/响应内容审核的哈希缓存与仓储、管理端审核配置接口。被 gateway_service 调用但自身内聚，可随批次 2 或紧随其后迁移。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `backend/internal/handler/admin/content_moderation_handler.go` · `backend/internal/handler/admin/content_moderation_handler_test.go` | +206/-37 | handler（含测试） | ☐ |
| P1 | `backend/internal/handler/content_moderation_helper.go` · `backend/internal/handler/content_moderation_helper_test.go` | +73/-2 | 实现（含测试） | ☐ |
| P1 | `backend/internal/repository/content_moderation_hash_cache.go` · `backend/internal/repository/content_moderation_hash_cache_test.go` | +1016/-9 | 缓存（含测试） | ☐ |
| P1 | `backend/internal/repository/content_moderation_repo.go` · `backend/internal/repository/content_moderation_repo_test.go` | +173/-9 | 仓储实现（含测试） | ☐ |

## 11. Claude/Anthropic 网关接入层（建议批次 4，整体重要度 P0）

网关 handler 主干：请求入口、账号选择失败处理、流式错误事件、故障切换（failover）、cyber 反封号策略、遥测上报。与 service 层 gateway_service 强耦合，是全仓库耦合度最高的区域之一，建议按"最小可编译骨架 → 增量 patch"方式迁移，不整体覆盖。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `backend/internal/handler/claude_telemetry_handler.go` | +80/-0 | handler | ☐ |
| P0 | `backend/internal/handler/endpoint.go` · `backend/internal/handler/endpoint_test.go` | +114/-141 | 实现（含测试） | ☐ |
| P0 | `backend/internal/handler/failover_headers.go` | +77/-0 | 实现 | ☐ |
| P0 | `backend/internal/handler/failover_loop.go` · `backend/internal/handler/failover_loop_test.go` | +194/-19 | 实现（含测试） | ☐ |
| P0 | `backend/internal/handler/gateway_cyber_policy.go` | +276/-0 | 实现 | ☐ |
| P0 | `backend/internal/handler/gateway_handler.go` | +1069/-361 | handler | ☐ |
| P0 | `backend/internal/handler/gateway_handler_chat_completions.go` | +59/-79 | 实现 | ☐ |
| P0 | `backend/internal/handler/gateway_handler_responses.go` · `backend/internal/handler/gateway_handler_responses_test.go` | +133/-73 | 实现（含测试） | ☐ |
| P0 | `backend/internal/handler/gateway_helper.go` | +78/-3 | 实现 | ☐ |
| P0 | `backend/internal/handler/gateway_web_search.go` · `backend/internal/handler/gateway_web_search_test.go` | +81/-509 | 实现（含测试） | ☐ |
| P0 | `backend/internal/handler/stream_error_event.go` · `backend/internal/handler/stream_error_event_test.go` | +142/-13 | 实现（含测试） | ☐ |
| P0 | `backend/internal/handler/upstream_forward_error.go` · `backend/internal/handler/upstream_forward_error_test.go` | +364/-0 | 实现（含测试） | ☐ |
| P0 | `backend/internal/pkg/claude/constants.go` · `backend/internal/pkg/claude/constants_test.go` | +39/-5 | 实现（含测试） | ☐ |
| P0 | `backend/internal/repository/claude_oauth_service.go` | +5/-1 | OAuth | ☐ |
| P0 | `backend/internal/repository/claude_usage_service.go` · `backend/internal/repository/claude_usage_service_test.go` | +20/-1 | 实现（含测试） | ☐ |
| P0 | `backend/internal/handler/gateway_count_tokens_test.go` | +101/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/gateway_group_model_unsupported_selection_test.go` | +223/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/gateway_group_model_unsupported_test.go` | +78/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/gateway_handler_error_fallback_test.go` | +93/-88 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/gateway_handler_stream_failover_test.go` | +49/-5 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/gateway_handler_usage_test.go` | +0/-1 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/gateway_handler_warmup_intercept_unit_test.go` | +96/-3 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/gateway_helper_hotpath_test.go` | +46/-69 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/gateway_models_test.go` | +25/-235 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/gateway_nil_guard_test.go` | +21/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/no_account_error_test.go` | +11/-0 | 单元测试 | ☐ |

## 12. apicompat 协议转换层（建议批次 5b，整体重要度 P0）

Anthropic ↔ Responses ↔ Chat Completions 三协议互转的纯函数库，OpenAI/Claude 双向兼容的地基。无外部依赖、测试覆盖极好，可整体搬运；与 OpenAI 网关核心（批次 5b）同批。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `backend/internal/pkg/apicompat/anthropic_to_cc_request.go` · `backend/internal/pkg/apicompat/anthropic_to_cc_request_test.go` | +370/-0 | 实现（含测试） | ☐ |
| P0 | `backend/internal/pkg/apicompat/anthropic_to_cc_response.go` | +237/-0 | 实现 | ☐ |
| P0 | `backend/internal/pkg/apicompat/anthropic_to_responses.go` | +422/-81 | 实现 | ☐ |
| P0 | `backend/internal/pkg/apicompat/anthropic_to_responses_response.go` · `backend/internal/pkg/apicompat/anthropic_to_responses_response_test.go` | +956/-49 | 实现（含测试） | ☐ |
| P0 | `backend/internal/pkg/apicompat/cc_to_anthropic_response.go` · `backend/internal/pkg/apicompat/cc_to_anthropic_response_test.go` | +541/-0 | 实现（含测试） | ☐ |
| P0 | `backend/internal/pkg/apicompat/chatcompletions_anthropic_bridge.go` · `backend/internal/pkg/apicompat/chatcompletions_anthropic_bridge_test.go` | +5/-4 | 实现（含测试） | ☐ |
| P0 | `backend/internal/pkg/apicompat/chatcompletions_responses_bridge.go` · `backend/internal/pkg/apicompat/chatcompletions_responses_bridge_test.go` | +508/-486 | 实现（含测试） | ☐ |
| P0 | `backend/internal/pkg/apicompat/chatcompletions_responses_bridge_ug20.go` | +205/-0 | 实现 | ☐ |
| P0 | `backend/internal/pkg/apicompat/chatcompletions_to_responses.go` | +151/-29 | 实现 | ☐ |
| P0 | `backend/internal/pkg/apicompat/claude_tool_names.go` | +41/-0 | 实现 | ☐ |
| P0 | `backend/internal/pkg/apicompat/responses_namespace.go` · `backend/internal/pkg/apicompat/responses_namespace_test.go` | +22/-1 | 实现（含测试） | ☐ |
| P0 | `backend/internal/pkg/apicompat/responses_stream_event_wire.go` · `backend/internal/pkg/apicompat/responses_stream_event_wire_test.go` | +14/-6 | 实现（含测试） | ☐ |
| P0 | `backend/internal/pkg/apicompat/responses_to_anthropic.go` | +571/-156 | 实现 | ☐ |
| P0 | `backend/internal/pkg/apicompat/responses_to_anthropic_request.go` · `backend/internal/pkg/apicompat/responses_to_anthropic_request_test.go` | +950/-92 | 实现（含测试） | ☐ |
| P0 | `backend/internal/pkg/apicompat/responses_to_chatcompletions.go` | +285/-20 | 实现 | ☐ |
| P0 | `backend/internal/pkg/apicompat/testdata/openai_claude_compat/refusal_envelope/expected_anthropic.json` | +15/-0 | 实现 | ☐ |
| P0 | `backend/internal/pkg/apicompat/testdata/openai_claude_compat/refusal_envelope/responses.json` | +18/-0 | 实现 | ☐ |
| P0 | `backend/internal/pkg/apicompat/testdata/openai_claude_compat/tool_name_round_trip/expected_anthropic.json` | +11/-0 | 实现 | ☐ |
| P0 | `backend/internal/pkg/apicompat/testdata/openai_claude_compat/tool_name_round_trip/responses.json` | +22/-0 | 实现 | ☐ |
| P0 | `backend/internal/pkg/apicompat/testdata/openai_claude_compat/tool_name_round_trip/tools.json` | +6/-0 | 实现 | ☐ |
| P0 | `backend/internal/pkg/apicompat/types.go` | +285/-101 | 实现 | ☐ |
| P0 | `backend/internal/pkg/apicompat/anthropic_responses_golden_test.go` | +41/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/apicompat/anthropic_responses_test.go` | +1251/-234 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/apicompat/chatcompletions_responses_bridge_custom_tools_test.go` | +34/-5 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/apicompat/chatcompletions_responses_stream_lifecycle_test.go` | +16/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/apicompat/chatcompletions_responses_test.go` | +912/-314 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/apicompat/chatcompletions_x_search_test.go` | +64/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/apicompat/golden_test_helpers_test.go` | +68/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/apicompat/responses_anthropic_cache_creation_test.go` | +21/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/apicompat/responses_input_item_namespace_test.go` | +57/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/apicompat/responses_to_anthropic_cc_chain_test.go` | +1/-1 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/apicompat/responses_to_anthropic_invalid_blocks_test.go` | +26/-13 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/apicompat/responses_to_anthropic_parallel_tool_test.go` | +0/-1 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/apicompat/responses_to_anthropic_read_tool_test.go` | +35/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/apicompat/responses_to_anthropic_request_invalid_args_test.go` | +22/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/apicompat/responses_to_anthropic_stream_test.go` | +403/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/apicompat/responses_to_anthropic_tool_pairing_test.go` | +19/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/apicompat/responses_to_anthropic_tools_test.go` | +35/-4 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/apicompat/responses_to_chatcompletions_codex_events_test.go` | +142/-3 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/apicompat/streaming_stop_reason_test.go` | +8/-0 | 单元测试 | ☐ |

## 13. OpenAI/Codex 网关接入层（建议批次 5，整体重要度 P0）

OpenAI 系 handler 全套：Responses/Chat Completions 入口、凭证故障切换、图像生成、embeddings/audio/live、模型选择错误分类。openai_gateway_handler.go 及其测试是脊柱文件（+4400 行测试），建议按 5a/5b/5c/5d 子批推进，与 service 层 OpenAI-Gateway-Core 对齐。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `backend/internal/handler/openai_account_response_rewrite.go` · `backend/internal/handler/openai_account_response_rewrite_test.go` | +248/-0 | 实现（含测试） | ☐ |
| P0 | `backend/internal/handler/openai_alpha_search.go` | +16/-26 | 实现 | ☐ |
| P0 | `backend/internal/handler/openai_audio.go` | +193/-0 | 实现 | ☐ |
| P0 | `backend/internal/handler/openai_chat_completions.go` · `backend/internal/handler/openai_chat_completions_test.go` | +596/-79 | 实现（含测试） | ☐ |
| P0 | `backend/internal/handler/openai_codex_models_handler.go` · `backend/internal/handler/openai_codex_models_handler_test.go` | +11/-2 | handler（含测试） | ☐ |
| P0 | `backend/internal/handler/openai_embeddings.go` · `backend/internal/handler/openai_embeddings_test.go` | +111/-33 | 实现（含测试） | ☐ |
| P0 | `backend/internal/handler/openai_gateway_count_tokens.go` | +12/-12 | 实现 | ☐ |
| P0 | `backend/internal/handler/openai_gateway_handler.go` · `backend/internal/handler/openai_gateway_handler_test.go` | +5917/-2017 | handler（含测试） | ☐ |
| P0 | `backend/internal/handler/openai_images.go` · `backend/internal/handler/openai_images_test.go` | +1062/-65 | 实现（含测试） | ☐ |
| P0 | `backend/internal/handler/openai_live.go` · `backend/internal/handler/openai_live_test.go` | +21/-2 | 实现（含测试） | ☐ |
| P0 | `backend/internal/handler/openai_schedule_result.go` | +51/-0 | 实现 | ☐ |
| P0 | `backend/internal/handler/openai_selection_error.go` · `backend/internal/handler/openai_selection_error_test.go` | +511/-0 | 实现（含测试） | ☐ |
| P0 | `backend/internal/pkg/openai/constants.go` · `backend/internal/pkg/openai/constants_test.go` | +27/-3 | 实现（含测试） | ☐ |
| P0 | `backend/internal/pkg/openai/instructions.txt` | +55/-57 | 实现 | ☐ |
| P0 | `backend/internal/pkg/openai/instructions_gpt5_2.txt` | +1/-1 | 实现 | ☐ |
| P0 | `backend/internal/pkg/openai/oauth.go` · `backend/internal/pkg/openai/oauth_test.go` | +225/-26 | OAuth（含测试） | ☐ |
| P0 | `backend/internal/pkg/openai/request.go` | +5/-0 | 实现 | ☐ |
| P0 | `backend/internal/repository/batch_image_repo.go` | +63/-6 | 仓储实现 | ☐ |
| P0 | `backend/internal/handler/image_concurrency_limiter_test.go` | +31/-40 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/openai_gateway_compact_body_signal_test.go` | +122/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/openai_gateway_compact_log_test.go` | +22/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/openai_gateway_credential_failover_loop_test.go` | +32/-6 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/openai_gateway_credential_failover_test.go` | +94/-21 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/openai_gateway_cyber_test.go` | +749/-7 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/openai_gateway_failover_test.go` | +207/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/openai_gateway_lenient_json_test.go` | +55/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/openai_gateway_usage_context_test.go` | +20/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/openai_images_controls_test.go` | +39/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/openai_images_failover_test.go` | +23/-8 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/openai_profit_slot_recheck_test.go` | +6/-6 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/openai_quota_platform_contract_test.go` | +8/-5 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/openai_responses_failover_cancel_test.go` | +146/-18 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/openai_video_billing_test.go` | +24/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/openai_ws_turn_pricing_test.go` | +2/-6 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/openai/oauth_redis_fallback_test.go` | +39/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/pkg/openai/oauth_userinfo_test.go` | +157/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/repository/batch_image_repo_integration_test.go` | +128/-10 | 集成测试 | ☐ |
| P0 | `backend/internal/server/middleware/openai_fast_policy_forwarding_test.go` | +3/-1 | 单元测试 | ☐ |

## 14. OpenAI OAuth 容量与 Codex 账号运维（建议批次 5a，整体重要度 P1）

OpenAI OAuth 账号容量时间序列（历史/小时表）、Codex 邀请重置记录、Codex 账号批量导入。为管理端"账号还能撑多久"视图与容量调度提供数据。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `backend/internal/handler/admin/account_codex_import.go` · `backend/internal/handler/admin/account_codex_import_test.go` | +159/-105 | 实现（含测试） | ☐ |
| P1 | `backend/internal/handler/admin/codex_invite_reset_handler.go` | +114/-0 | handler | ☐ |
| P1 | `backend/internal/handler/admin/openai_oauth_handler.go` · `backend/internal/handler/admin/openai_oauth_handler_test.go` | +304/-85 | handler（含测试） | ☐ |
| P1 | `backend/internal/repository/codex_invite_reset_history_repo.go` | +139/-0 | 仓储实现 | ☐ |
| P1 | `backend/internal/repository/openai_oauth_capacity.go` | +703/-0 | OAuth | ☐ |
| P1 | `backend/internal/repository/openai_oauth_service.go` | +1/-1 | OAuth | ☐ |
| P1 | `backend/internal/server/routes/admin_openai_oauth_capacity_routes_test.go` | +39/-0 | 单元测试 | ☐ |

## 15. Kiro / Bedrock 接入（建议批次 6，整体重要度 P1）

Kiro（Amazon Q/CodeWhisperer 协议）完整接入：协议转换器、Office/PDF 文档解析、fake cache、机器指纹、token 计数；含 invokeMCP 用的 webfetch 沙箱与身份伪装 promptsanitize。几乎纯新增、零侵入，可整体搬运；若决定放弃 Kiro 平台则本模块（含对应 schema 枚举、migrations）可整组放弃。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `backend/cmd/kiro_native_capture/main.go` | +763/-0 | 实现 | ☐ |
| P1 | `backend/internal/handler/admin/kiro_oauth_handler.go` · `backend/internal/handler/admin/kiro_oauth_handler_test.go` | +619/-0 | handler（含测试） | ☐ |
| P1 | `backend/internal/pkg/kiro/converter.go` · `backend/internal/pkg/kiro/converter_test.go` | +4044/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/pkg/kiro/document_limits.go` | +106/-0 | 实现 | ☐ |
| P1 | `backend/internal/pkg/kiro/document_office.go` · `backend/internal/pkg/kiro/document_office_test.go` | +584/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/pkg/kiro/document_pdf.go` · `backend/internal/pkg/kiro/document_pdf_test.go` | +313/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/pkg/kiro/fake_cache.go` · `backend/internal/pkg/kiro/fake_cache_test.go` | +1474/-0 | 缓存（含测试） | ☐ |
| P1 | `backend/internal/pkg/kiro/machine_id.go` · `backend/internal/pkg/kiro/machine_id_test.go` | +106/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/pkg/kiro/models.go` · `backend/internal/pkg/kiro/models_test.go` | +398/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/pkg/kiro/testdata/sample.docx` | 二进制 | 实现 | ☐ |
| P1 | `backend/internal/pkg/kiro/testdata/sample.xlsx` | 二进制 | 实现 | ☐ |
| P1 | `backend/internal/pkg/kiro/testdata/sample_shared.xlsx` | 二进制 | 实现 | ☐ |
| P1 | `backend/internal/pkg/kiro/testdata/sample_text.pdf` | +72/-0 | 实现 | ☐ |
| P1 | `backend/internal/pkg/kiro/token_counter.go` · `backend/internal/pkg/kiro/token_counter_test.go` | +105/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/pkg/kiro/user_agent.go` · `backend/internal/pkg/kiro/user_agent_test.go` | +79/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/pkg/promptsanitize/structure.go` · `backend/internal/pkg/promptsanitize/structure_test.go` | +176/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/pkg/promptsanitize/system.go` | +54/-0 | 实现 | ☐ |
| P1 | `backend/internal/pkg/webfetch/fetcher.go` · `backend/internal/pkg/webfetch/fetcher_test.go` | +1411/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/pkg/webfetch/types.go` | +54/-0 | 实现 | ☐ |
| P1 | `backend/internal/handler/admin/account_handler_kiro_test.go` | +373/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/setting_handler_kiro_runtime_test.go` | +217/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/gateway_handler_bedrock_test.go` | +20/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/pkg/kiro/converter_tool_result_unit_test.go` | +79/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/server/routes/admin_kiro_account_routes_test.go` | +44/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/server/routes/admin_kiro_routes_test.go` | +46/-0 | 单元测试 | ☐ |

## 16. Grok（xAI）接入（建议批次 6，整体重要度 P1）

Grok OAuth/配额/计费（pkg/xai）、音频与媒体接口、X 搜索、时间线。强依赖 OpenAI 网关兼容层（借 Chat Completions 协议壳），必须排在批次 5 之后。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `backend/internal/handler/admin/grok_import_probe.go` · `backend/internal/handler/admin/grok_import_probe_test.go` | +74/-199 | 实现（含测试） | ☐ |
| P1 | `backend/internal/handler/admin/grok_oauth_handler.go` · `backend/internal/handler/admin/grok_oauth_handler_test.go` | +185/-494 | handler（含测试） | ☐ |
| P1 | `backend/internal/handler/grok_audio.go` · `backend/internal/handler/grok_audio_test.go` | +119/-126 | 实现（含测试） | ☐ |
| P1 | `backend/internal/handler/grok_media.go` · `backend/internal/handler/grok_media_test.go` | +69/-482 | 实现（含测试） | ☐ |
| P1 | `backend/internal/handler/openai_grok_timeline.go` · `backend/internal/handler/openai_grok_timeline_test.go` | +157/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/handler/openai_x_search.go` · `backend/internal/handler/openai_x_search_test.go` | +122/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/pkg/xai/billing.go` · `backend/internal/pkg/xai/billing_test.go` | +517/-404 | 实现（含测试） | ☐ |
| P1 | `backend/internal/pkg/xai/cli_identity.go` · `backend/internal/pkg/xai/cli_identity_test.go` | +8/-13 | 实现（含测试） | ☐ |
| P1 | `backend/internal/pkg/xai/models.go` · `backend/internal/pkg/xai/models_test.go` | +81/-124 | 实现（含测试） | ☐ |
| P1 | `backend/internal/pkg/xai/oauth.go` · `backend/internal/pkg/xai/oauth_test.go` | +126/-68 | OAuth（含测试） | ☐ |
| P1 | `backend/internal/repository/grok_oauth_client.go` · `backend/internal/repository/grok_oauth_client_test.go` | +331/-74 | 上游客户端（含测试） | ☐ |
| P1 | `backend/internal/handler/admin/grok_import_probe_handler_test.go` | +0/-124 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/grok_audio_billing_test.go` | +28/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/pkg/xai/oauth_redis_fallback_test.go` | +18/-12 | 单元测试 | ☐ |
| P1 | `backend/internal/pkg/xai/quota_test.go` | +0/-19 | 单元测试 | ☐ |
| P1 | `backend/internal/pkg/xai/sso_device_test.go` | +7/-10 | 单元测试 | ☐ |
| P1 | `backend/internal/repository/usage_log_repo_grok_recovery_integration_test.go` | +78/-0 | 集成测试 | ☐ |
| P1 | `backend/internal/repository/usage_log_repo_grok_recovery_test.go` | +50/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/server/routes/admin_grok_routes_test.go` | +53/-0 | 单元测试 | ☐ |

## 17. Gemini 与 Antigravity 接入（建议批次 6，整体重要度 P1）

Gemini CLI（CodeAssist）与 Antigravity 的 OAuth/请求转换增强：OAuth Redis 降级、token 缓存、v1beta 入口重写；personal-dev 删除了上游 Drive 客户端。改动集中在平台专属文件，相对独立。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `backend/internal/handler/admin/gemini_oauth_handler.go` · `backend/internal/handler/admin/gemini_oauth_handler_test.go` | +260/-17 | handler（含测试） | ☐ |
| P1 | `backend/internal/handler/gemini_v1beta_handler.go` | +183/-47 | handler | ☐ |
| P1 | `backend/internal/pkg/antigravity/client.go` · `backend/internal/pkg/antigravity/client_test.go` | +9/-1 | 实现（含测试） | ☐ |
| P1 | `backend/internal/pkg/antigravity/gemini_types.go` | +0/-9 | 实现 | ☐ |
| P1 | `backend/internal/pkg/antigravity/oauth.go` · `backend/internal/pkg/antigravity/oauth_test.go` | +153/-11 | OAuth（含测试） | ☐ |
| P1 | `backend/internal/pkg/antigravity/request_transformer.go` · `backend/internal/pkg/antigravity/request_transformer_test.go` | +71/-32 | 实现（含测试） | ☐ |
| P1 | `backend/internal/pkg/geminicli/codeassist_types.go` · `backend/internal/pkg/geminicli/codeassist_types_test.go` | +291/-13 | 实现（含测试） | ☐ |
| P1 | `backend/internal/pkg/geminicli/drive_client.go` · `backend/internal/pkg/geminicli/drive_client_test.go` | +0/-175 | 上游客户端（含测试） | ☐ |
| P1 | `backend/internal/pkg/geminicli/oauth.go` · `backend/internal/pkg/geminicli/oauth_test.go` | +192/-35 | OAuth（含测试） | ☐ |
| P1 | `backend/internal/pkg/geminicli/token_types.go` | +1/-0 | 实现 | ☐ |
| P1 | `backend/internal/repository/gemini_drive_client.go` | +0/-9 | 上游客户端 | ☐ |
| P1 | `backend/internal/repository/gemini_oauth_client.go` | +1/-1 | 上游客户端 | ☐ |
| P1 | `backend/internal/repository/gemini_token_cache.go` · `backend/internal/repository/gemini_token_cache_test.go` | +65/-4 | 缓存（含测试） | ☐ |
| P1 | `backend/internal/repository/geminicli_codeassist_client.go` | +82/-16 | 上游客户端 | ☐ |
| P1 | `backend/internal/pkg/antigravity/oauth_redis_fallback_test.go` | +41/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/pkg/geminicli/oauth_redis_fallback_test.go` | +39/-0 | 单元测试 | ☐ |

## 18. AI Studio / Skill / 视频生产（建议批次 3，整体重要度 P1）

完全独立的新子系统：AI 创作工作台（多模态生成、资产生命周期、存储计费）、Skill 技能市场（安装/点赞/评审/结算、Docker 沙箱运行时）、Studio 视频生产管线（ffmpeg）。零修改上游、仅 wire 与路由有集成点，可最早整体搬运，也最容易整组放弃——是本文档中"一票取舍"收益最大的模块。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `backend/internal/domain/ai.go` | +296/-0 | 实现 | ☐ |
| P1 | `backend/internal/domain/ai_skill.go` | +532/-0 | 实现 | ☐ |
| P1 | `backend/internal/handler/admin/ai_handler.go` | +700/-0 | handler | ☐ |
| P1 | `backend/internal/handler/admin/ai_skill_handler.go` | +408/-0 | handler | ☐ |
| P1 | `backend/internal/handler/ai_asset_extend.go` | +106/-0 | 实现 | ☐ |
| P1 | `backend/internal/handler/ai_gateway_bridge.go` · `backend/internal/handler/ai_gateway_bridge_test.go` | +1857/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/handler/ai_handler.go` | +1010/-0 | handler | ☐ |
| P1 | `backend/internal/handler/ai_live_bridge.go` · `backend/internal/handler/ai_live_bridge_test.go` | +411/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/handler/ai_realtime_bridge.go` · `backend/internal/handler/ai_realtime_bridge_test.go` | +263/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/handler/ai_skill_handler.go` · `backend/internal/handler/ai_skill_handler_test.go` | +2585/-0 | handler（含测试） | ☐ |
| P1 | `backend/internal/handler/ai_stt_bridge.go` · `backend/internal/handler/ai_stt_bridge_test.go` | +943/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/handler/ai_tts_bridge.go` · `backend/internal/handler/ai_tts_bridge_test.go` | +537/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/handler/ai_video_bridge.go` · `backend/internal/handler/ai_video_bridge_test.go` | +695/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/handler/dto/ai.go` | +572/-0 | DTO | ☐ |
| P1 | `backend/internal/handler/dto/ai_skill.go` | +890/-0 | DTO | ☐ |
| P1 | `backend/internal/handler/skillkit/module.go` · `backend/internal/handler/skillkit/module_test.go` | +87/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/handler/skillkit/queries.go` · `backend/internal/handler/skillkit/queries_test.go` | +1117/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/handler/studio_production_handler.go` | +667/-0 | handler | ☐ |
| P1 | `backend/internal/handler/studio_production_render.go` · `backend/internal/handler/studio_production_render_test.go` | +845/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/integration/skillrunner/dispatch.go` | +262/-0 | 实现 | ☐ |
| P1 | `backend/internal/integration/skillrunner/docker.go` | +105/-0 | 实现 | ☐ |
| P1 | `backend/internal/integration/skillrunner/errors.go` | +8/-0 | 实现 | ☐ |
| P1 | `backend/internal/integration/skillrunner/executor.go` · `backend/internal/integration/skillrunner/executor_test.go` | +492/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/integration/skillrunner/inspector.go` | +296/-0 | 实现 | ☐ |
| P1 | `backend/internal/integration/skillrunner/types.go` | +283/-0 | 实现 | ☐ |
| P1 | `backend/internal/repository/ai_asset_lifecycle_repo.go` | +249/-0 | 仓储实现 | ☐ |
| P1 | `backend/internal/repository/ai_center_repo.go` | +1236/-0 | 仓储实现 | ☐ |
| P1 | `backend/internal/repository/ai_skill_balance_ledger_repo.go` · `backend/internal/repository/ai_skill_balance_ledger_repo_test.go` | +421/-0 | 仓储实现（含测试） | ☐ |
| P1 | `backend/internal/repository/ai_skill_repo.go` | +1530/-0 | 仓储实现 | ☐ |
| P1 | `backend/internal/repository/ai_skill_repo_helpers.go` · `backend/internal/repository/ai_skill_repo_helpers_test.go` | +1042/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/repository/ai_studio_storage_billing_repo.go` · `backend/internal/repository/ai_studio_storage_billing_repo_test.go` | +555/-0 | 仓储实现（含测试） | ☐ |
| P1 | `backend/internal/repository/studio_production_repo.go` | +748/-0 | 仓储实现 | ☐ |
| P1 | `backend/internal/server/routes/ai.go` | +124/-0 | 路由注册 | ☐ |
| P1 | `backend/internal/domain/ai_visibility_test.go` | +154/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/ai_media_base_url_test.go` | +99/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/ai_skill_settlement_handler_test.go` | +196/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/ai_media_base_url_test.go` | +72/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/ai_skill_handler_visibility_test.go` | +71/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/ai_skill_list_filters_test.go` | +102/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/ai_skill_media_migration_test.go` | +169/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/ai_video_download_security_test.go` | +22/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/dto/ai_asset_test.go` | +90/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/dto/ai_runtime_test.go` | +35/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/dto/ai_skill_visibility_test.go` | +89/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/studio_production_ffmpeg_test.go` | +32/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/integration/skillrunner/skillrunner_test.go` | +422/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/repository/ai_center_repo_matches_asset_filter_test.go` | +72/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/repository/ai_center_repo_prompt_template_test.go` | +190/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/repository/ai_skill_balance_ledger_repo_integration_test.go` | +116/-0 | 集成测试 | ☐ |
| P1 | `backend/internal/repository/ai_skill_repo_list_filters_integration_test.go` | +314/-0 | 集成测试 | ☐ |
| P1 | `backend/internal/repository/ai_skill_wire_digest_test.go` | +87/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/repository/ai_skill_wire_pricing_test.go` | +42/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/repository/studio_production_job_lock_test.go` | +55/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/server/api_skill_contract_test.go` | +413/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/server/routes/ai_feature_gate_test.go` | +199/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/server/routes/ai_routes_test.go` | +60/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/server/routes/skill_routes_test.go` | +91/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/server/routes/user_ai_routes_test.go` | +104/-0 | 单元测试 | ☐ |

## 19. 媒体存储（建议批次 3 前置，整体重要度 P1）

通用媒体对象存储（本地/S3 profile、缩略图、公开 URL）。AI Studio 资产、工单附件、发票文件都依赖它，需在这三者之前落地。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `backend/internal/handler/admin/media_handler.go` | +265/-0 | handler | ☐ |
| P1 | `backend/internal/handler/dto/media.go` · `backend/internal/handler/dto/media_test.go` | +152/-0 | DTO（含测试） | ☐ |
| P1 | `backend/internal/handler/media_handler.go` · `backend/internal/handler/media_handler_test.go` | +371/-0 | handler（含测试） | ☐ |
| P1 | `backend/internal/repository/media_object_store.go` | +161/-0 | 实现 | ☐ |
| P1 | `backend/internal/repository/media_repository.go` · `backend/internal/repository/media_repository_test.go` | +387/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/server/routes/media.go` | +48/-0 | 路由注册 | ☐ |
| P1 | `backend/internal/handler/media_handler_public_test.go` | +91/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/server/routes/media_routes_test.go` | +55/-0 | 单元测试 | ☐ |

## 20. 渠道监控 v2（建议批次 7，整体重要度 P1）

渠道健康监控定制：探测中间件、请求模板、聚合与 rollup。⚠️ 上游已自带 channel_monitor_v2 骨架（194-206 号迁移），本模块是在其上的定制 diff，迁移时先对照上游现状裁剪。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `backend/internal/handler/admin/channel_monitor_handler.go` · `backend/internal/handler/admin/channel_monitor_handler_test.go` | +388/-6 | handler（含测试） | ☐ |
| P1 | `backend/internal/handler/admin/channel_monitor_template_handler.go` · `backend/internal/handler/admin/channel_monitor_template_handler_test.go` | +107/-1 | handler（含测试） | ☐ |
| P1 | `backend/internal/handler/channel_monitor_user_handler.go` · `backend/internal/handler/channel_monitor_user_handler_test.go` | +55/-4 | handler（含测试） | ☐ |
| P1 | `backend/internal/repository/channel_monitor_repo.go` · `backend/internal/repository/channel_monitor_repo_test.go` | +317/-20 | 仓储实现（含测试） | ☐ |
| P1 | `backend/internal/repository/channel_monitor_v2_aggregation.go` | +44/-175 | 实现 | ☐ |
| P1 | `backend/internal/repository/channel_monitor_v2_repo.go` · `backend/internal/repository/channel_monitor_v2_repo_test.go` | +80/-334 | 仓储实现（含测试） | ☐ |
| P1 | `backend/internal/server/middleware/channel_monitor_probe.go` · `backend/internal/server/middleware/channel_monitor_probe_test.go` | +247/-0 | 中间件（含测试） | ☐ |
| P1 | `backend/internal/server/routes/channel_monitor_feature_gate_test.go` | +14/-53 | 单元测试 | ☐ |

## 21. 管理端账号运维（建议批次 7 / 随各平台批，整体重要度 P1）

管理端账号 CRUD 大改（+1171 行）、归档导入、批量凭证更新、数据富化。account_handler.go 被所有平台批次顺带修改，建议先迁骨架、各平台批次增量补丁。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `backend/internal/handler/admin/account_archive_import.go` · `backend/internal/handler/admin/account_archive_import_test.go` | +461/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/handler/admin/account_data.go` | +433/-37 | 实现 | ☐ |
| P1 | `backend/internal/handler/admin/account_handler.go` | +1171/-525 | handler | ☐ |
| P1 | `backend/internal/handler/admin/request_base_url.go` · `backend/internal/handler/admin/request_base_url_test.go` | +105/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/handler/admin/account_data_enrich_test.go` | +81/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/account_data_handler_test.go` | +542/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/account_handler_available_models_test.go` | +107/-97 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/account_handler_batch_delete_test.go` | +0/-173 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/account_handler_cyber_test.go` | +88/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/account_handler_getbyid_test.go` | +123/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/account_handler_list_test.go` | +1/-1 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/account_handler_long_context_billing_test.go` | +0/-165 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/account_handler_mixed_channel_test.go` | +0/-265 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/account_handler_passthrough_test.go` | +1/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/account_handler_refresh_tier_test.go` | +316/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/account_handler_test_mode_test.go` | +60/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/batch_update_credentials_test.go` | +153/-1 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/dto/account_mapper_test.go` | +227/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/dto/credentials_redact_test.go` | +10/-1 | 单元测试 | ☐ |

## 22. 代理池（建议批次 7，整体重要度 P1）

代理管理增强：批量操作、延迟探测缓存、到期 fallback（schema proxy.go 的 backup_proxy→fallback_sources 边改名与此配套）。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `backend/internal/handler/admin/proxy_data.go` | +24/-16 | 实现 | ☐ |
| P1 | `backend/internal/handler/admin/proxy_handler.go` | +232/-12 | handler | ☐ |
| P1 | `backend/internal/repository/proxy_latency_cache.go` · `backend/internal/repository/proxy_latency_cache_test.go` | +174/-6 | 缓存（含测试） | ☐ |
| P1 | `backend/internal/repository/proxy_probe_service.go` · `backend/internal/repository/proxy_probe_service_test.go` | +42/-6 | 实现（含测试） | ☐ |
| P1 | `backend/internal/repository/proxy_repo.go` | +68/-49 | 仓储实现 | ☐ |
| P1 | `backend/internal/handler/admin/proxy_handler_batch_test.go` | +274/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/proxy_handler_update_test.go` | +34/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/repository/proxy_expiry_integration_test.go` | +45/-0 | 集成测试 | ☐ |
| P1 | `backend/internal/repository/proxy_repo_upstream_billing_probe_test.go` | +5/-14 | 单元测试 | ☐ |

## 23. 管理端设置（建议批次 7，整体重要度 P1）

personal-dev 把上游拆分的 setting_handler_{audit,email,runtime,update}.go 全部合并回单一 setting_handler.go（+5343/-236，结构性替换而非叠加）。⚠️ 上游后续更新会落在被删的旧文件名上，同步时需人工映射；建议按配置分区逐段迁移，与前端 SettingsView 同节奏。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `backend/internal/handler/admin/setting_handler.go` | +5343/-236 | handler | ☐ |
| P1 | `backend/internal/handler/admin/setting_handler_audit.go` | +0/-863 | 实现 | ☐ |
| P1 | `backend/internal/handler/admin/setting_handler_email.go` | +0/-347 | 实现 | ☐ |
| P1 | `backend/internal/handler/admin/setting_handler_runtime.go` | +0/-436 | 实现 | ☐ |
| P1 | `backend/internal/handler/admin/setting_handler_update.go` | +0/-2458 | 实现 | ☐ |
| P1 | `backend/internal/handler/dto/settings.go` | +161/-62 | DTO | ☐ |
| P1 | `backend/internal/handler/setting_handler.go` | +6/-5 | handler | ☐ |
| P1 | `backend/internal/handler/admin/setting_handler_auth_source_defaults_test.go` | +372/-9 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/setting_handler_custom_menu_test.go` | +55/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/setting_handler_default_account_model_config_test.go` | +57/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/setting_handler_feature_switch_compat_test.go` | +370/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/setting_handler_media_migration_test.go` | +1003/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/setting_handler_missing_fields_test.go` | +58/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/setting_handler_model_plaza_audit_test.go` | +35/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/setting_handler_partial_payload_test.go` | +7/-114 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/setting_handler_payment_config_test.go` | +56/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/setting_handler_platform_quota_test.go` | +2/-31 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/setting_handler_platform_threshold_test.go` | +44/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/dto/public_settings_injection_schema_test.go` | +0/-2 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/setting_handler_public_test.go` | +3/-0 | 单元测试 | ☐ |

## 24. 管理端运营（仪表盘/用户/分组/公告）（建议批次 7，整体重要度 P1）

管理端仪表盘（请求类型细分、缓存）、用户/分组管理、公告已读、合规中间件等日常运营面板。依赖用量计费底座的数据，放在管理端批次统一收口。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `backend/internal/handler/admin/announcement_handler.go` | +2/-0 | handler | ☐ |
| P1 | `backend/internal/handler/admin/channel_handler.go` | +11/-1 | handler | ☐ |
| P1 | `backend/internal/handler/admin/dashboard_handler.go` | +55/-49 | handler | ☐ |
| P1 | `backend/internal/handler/admin/dashboard_query_cache.go` | +62/-74 | 缓存 | ☐ |
| P1 | `backend/internal/handler/admin/dashboard_snapshot_v2_handler.go` | +50/-57 | handler | ☐ |
| P1 | `backend/internal/handler/admin/group_handler.go` | +289/-169 | handler | ☐ |
| P1 | `backend/internal/handler/admin/usage_handler.go` | +63/-70 | handler | ☐ |
| P1 | `backend/internal/handler/admin/user_handler.go` | +74/-15 | handler | ☐ |
| P1 | `backend/internal/handler/announcement_handler.go` | +17/-2 | handler | ☐ |
| P1 | `backend/internal/repository/announcement_read_repo.go` | +117/-0 | 仓储实现 | ☐ |
| P1 | `backend/internal/repository/dashboard_aggregation_repo.go` | +19/-3 | 仓储实现 | ☐ |
| P1 | `backend/internal/repository/dashboard_cache.go` · `backend/internal/repository/dashboard_cache_test.go` | +37/-0 | 缓存（含测试） | ☐ |
| P1 | `backend/internal/server/middleware/admin_auth.go` · `backend/internal/server/middleware/admin_auth_test.go` | +43/-1 | 中间件（含测试） | ☐ |
| P1 | `backend/internal/server/middleware/admin_compliance.go` · `backend/internal/server/middleware/admin_compliance_test.go` | +21/-1 | 中间件（含测试） | ☐ |
| P1 | `backend/internal/handler/admin/admin_basic_handlers_test.go` | +145/-1 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/admin_helpers_test.go` | +2/-2 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/admin_service_stub_test.go` | +69/-21 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/announcement_handler_sort_test.go` | +96/-5 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/compliance_handler_test.go` | +49/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/dashboard_handler_cache_test.go` | +1/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/dashboard_handler_request_type_test.go` | +143/-73 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/dashboard_handler_user_breakdown_test.go` | +24/-27 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/group_handler_stats_test.go` | +82/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/group_handler_usage_summary_test.go` | +67/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/token_cache_invalidator_test.go` | +18/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/usage_cleanup_handler_test.go` | +40/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/user_handler_activity_test.go` | +67/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/user_handler_role_stepup_test.go` | +2/-2 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/user_platform_quota_admin_test.go` | +4/-3 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/admin/user_platform_quotas_handler_test.go` | +3/-3 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/dto/user_mapper_activity_test.go` | +22/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/repository/announcement_read_status_test.go` | +162/-0 | 单元测试 | ☐ |

## 25. 发票与支付（建议批次 8，整体重要度 P1）

支付渠道 provider 增强（支付宝/微信/Stripe/Airwallex）、支付订单履约租约、发票系统（合并开票、状态机、后台提醒）。与网关解耦、独立成批，低风险。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `backend/internal/handler/admin/payment_handler.go` | +308/-88 | handler | ☐ |
| P1 | `backend/internal/handler/payment_handler.go` | +321/-88 | handler | ☐ |
| P1 | `backend/internal/handler/payment_webhook_handler.go` · `backend/internal/handler/payment_webhook_handler_test.go` | +280/-3 | handler（含测试） | ☐ |
| P1 | `backend/internal/payment/crypto.go` | +0/-10 | 实现 | ☐ |
| P1 | `backend/internal/payment/load_balancer.go` · `backend/internal/payment/load_balancer_test.go` | +87/-41 | 实现（含测试） | ☐ |
| P1 | `backend/internal/payment/provider/airwallex.go` · `backend/internal/payment/provider/airwallex_test.go` | +42/-1 | 实现（含测试） | ☐ |
| P1 | `backend/internal/payment/provider/alipay.go` · `backend/internal/payment/provider/alipay_test.go` | +133/-8 | 实现（含测试） | ☐ |
| P1 | `backend/internal/payment/provider/stripe.go` · `backend/internal/payment/provider/stripe_test.go` | +50/-66 | 实现（含测试） | ☐ |
| P1 | `backend/internal/payment/provider/wxpay.go` · `backend/internal/payment/provider/wxpay_test.go` | +33/-1 | 实现（含测试） | ☐ |
| P1 | `backend/internal/payment/registry.go` | +8/-4 | 实现 | ☐ |
| P1 | `backend/internal/payment/types.go` | +10/-8 | 实现 | ☐ |
| P1 | `backend/internal/server/routes/payment.go` | +51/-5 | 路由注册 | ☐ |
| P1 | `backend/internal/handler/admin/payment_handler_invoice_test.go` | +258/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/payment_handler_response_test.go` | +43/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/payment_handler_resume_test.go` | +144/-4 | 单元测试 | ☐ |
| P1 | `backend/internal/server/routes/payment_public_security_test.go` | +186/-0 | 单元测试 | ☐ |

## 26. 订阅、兑换码与联盟返利（建议批次 8，整体重要度 P2）

联盟返利加固（台账反向操作去重、返利修复 CLI）、兑换码与订阅小改。业务量小、独立，可放最后扫尾；不做分销可整组放弃联盟部分。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P2 | `backend/cmd/repair_affiliate_rebates/main.go` · `backend/cmd/repair_affiliate_rebates/main_test.go` | +254/-0 | 实现（含测试） | ☐ |
| P2 | `backend/internal/handler/admin/redeem_handler.go` · `backend/internal/handler/admin/redeem_handler_test.go` | +162/-28 | handler（含测试） | ☐ |
| P2 | `backend/internal/handler/redeem_handler.go` | +45/-0 | handler | ☐ |
| P2 | `backend/internal/repository/affiliate_repo.go` · `backend/internal/repository/affiliate_repo_test.go` | +1098/-47 | 仓储实现（含测试） | ☐ |
| P2 | `backend/internal/repository/promo_code_repo.go` | +2/-1 | 仓储实现 | ☐ |
| P2 | `backend/internal/repository/redeem_code_repo.go` | +105/-12 | 仓储实现 | ☐ |
| P2 | `backend/internal/repository/user_subscription_repo.go` | +13/-0 | 仓储实现 | ☐ |
| P2 | `backend/internal/repository/affiliate_repo_integration_test.go` | +852/-2 | 集成测试 | ☐ |
| P2 | `backend/internal/repository/redeem_code_repo_integration_test.go` | +99/-0 | 集成测试 | ☐ |
| P2 | `backend/internal/repository/redeem_code_repo_sum_positive_balance_unit_test.go` | +95/-0 | 单元测试 | ☐ |
| P2 | `backend/internal/server/routes/admin_affiliate_routes_test.go` | +53/-0 | 单元测试 | ☐ |

## 27. 工单系统（建议批次 8，整体重要度 P1）

全新工单子系统：用户/管理端双侧 handler、状态机流转、未读角标、提醒清理。独立小批次，依赖媒体存储（附件）。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `backend/internal/handler/admin/ticket_handler.go` · `backend/internal/handler/admin/ticket_handler_test.go` | +708/-0 | handler（含测试） | ☐ |
| P1 | `backend/internal/handler/dto/ticket.go` | +133/-0 | DTO | ☐ |
| P1 | `backend/internal/handler/ticket_handler.go` · `backend/internal/handler/ticket_handler_test.go` | +857/-0 | handler（含测试） | ☐ |
| P1 | `backend/internal/repository/ticket_repo.go` · `backend/internal/repository/ticket_repo_test.go` | +630/-0 | 仓储实现（含测试） | ☐ |
| P1 | `backend/internal/repository/ticket_repo_create_retry_test.go` | +183/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/repository/ticket_repo_transition_test.go` | +277/-0 | 单元测试 | ☐ |

## 28. 前台认证与 OAuth 登录（建议批次 8，整体重要度 P1）

钉钉/微信/LinuxDo/OIDC/邮箱 OAuth 登录、pending 绑定流、refresh token cookie 轮换、token_version 会话吊销、Passkey 小改。独立低风险，可提前。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `backend/internal/handler/auth_dingtalk_oauth.go` · `backend/internal/handler/auth_dingtalk_oauth_test.go` | +408/-12 | OAuth（含测试） | ☐ |
| P1 | `backend/internal/handler/auth_email_oauth.go` · `backend/internal/handler/auth_email_oauth_test.go` | +228/-64 | OAuth（含测试） | ☐ |
| P1 | `backend/internal/handler/auth_handler.go` | +180/-76 | handler | ☐ |
| P1 | `backend/internal/handler/auth_linuxdo_oauth.go` · `backend/internal/handler/auth_linuxdo_oauth_test.go` | +168/-31 | OAuth（含测试） | ☐ |
| P1 | `backend/internal/handler/auth_oauth_pending_flow.go` · `backend/internal/handler/auth_oauth_pending_flow_test.go` | +1475/-591 | OAuth（含测试） | ☐ |
| P1 | `backend/internal/handler/auth_oidc_oauth.go` · `backend/internal/handler/auth_oidc_oauth_test.go` | +85/-23 | OAuth（含测试） | ☐ |
| P1 | `backend/internal/handler/auth_refresh_cookie.go` · `backend/internal/handler/auth_refresh_cookie_test.go` | +178/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/handler/auth_wechat_oauth.go` · `backend/internal/handler/auth_wechat_oauth_test.go` | +125/-16 | OAuth（含测试） | ☐ |
| P1 | `backend/internal/handler/passkey_handler.go` · `backend/internal/handler/passkey_handler_test.go` | +22/-105 | handler（含测试） | ☐ |
| P1 | `backend/internal/middleware/rate_limiter.go` · `backend/internal/middleware/rate_limiter_test.go` | +54/-24 | 中间件（含测试） | ☐ |
| P1 | `backend/internal/repository/email_cache.go` · `backend/internal/repository/email_cache_test.go` | +166/-22 | 缓存（含测试） | ☐ |
| P1 | `backend/internal/repository/refresh_token_cache.go` · `backend/internal/repository/refresh_token_cache_test.go` | +186/-3 | 缓存（含测试） | ☐ |
| P1 | `backend/internal/repository/user_profile_identity_repo.go` | +49/-0 | 仓储实现 | ☐ |
| P1 | `backend/internal/server/middleware/jwt_auth.go` · `backend/internal/server/middleware/jwt_auth_test.go` | +92/-88 | 中间件（含测试） | ☐ |
| P1 | `backend/internal/server/middleware/optional_jwt_auth.go` | +1/-1 | 中间件 | ☐ |
| P1 | `backend/internal/server/routes/auth.go` | +15/-18 | 路由注册 | ☐ |
| P1 | `backend/internal/handler/auth_current_user_test.go` | +94/-12 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/auth_refresh_token_handler_test.go` | +892/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/auth_session_revocation_test.go` | +3/-5 | 单元测试 | ☐ |
| P1 | `backend/internal/repository/user_profile_identity_repo_contract_test.go` | +12/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/server/routes/auth_rate_limit_test.go` | +19/-1 | 单元测试 | ☐ |
| P1 | `backend/internal/server/routes/passkey_routes_test.go` | +38/-0 | 单元测试 | ☐ |

## 29. 运维观测与 IP 安全（建议批次 9，整体重要度 P1）

运维错误日志大改（ops_error_logger +912）、请求详情/趋势/预聚合、审计留存、IP 多账号安全检测、prompt 审计路由。纯可观测性不阻塞核心链路，但 ops_error_logger 与网关错误路径耦合较深。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `backend/internal/handler/admin/ip_security_handler.go` | +96/-0 | handler | ☐ |
| P1 | `backend/internal/handler/admin/ops_dashboard_handler.go` | +1/-1 | handler | ☐ |
| P1 | `backend/internal/handler/admin/ops_handler.go` | +86/-51 | handler | ☐ |
| P1 | `backend/internal/handler/ops_error_logger.go` · `backend/internal/handler/ops_error_logger_test.go` | +1478/-929 | 实现（含测试） | ☐ |
| P1 | `backend/internal/handler/ops_latency.go` · `backend/internal/handler/ops_latency_test.go` | +126/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/handler/security_audit_errors.go` | +13/-0 | 实现 | ☐ |
| P1 | `backend/internal/pkg/logger/logger.go` | +19/-0 | 实现 | ☐ |
| P1 | `backend/internal/pkg/servertiming/http.go` | +4/-0 | 实现 | ☐ |
| P1 | `backend/internal/repository/audit_retention_repo.go` | +54/-0 | 仓储实现 | ☐ |
| P1 | `backend/internal/repository/ip_security_repo.go` | +263/-0 | 仓储实现 | ☐ |
| P1 | `backend/internal/repository/ops_repo.go` | +455/-27 | 仓储实现 | ☐ |
| P1 | `backend/internal/repository/ops_repo_account_cyber.go` · `backend/internal/repository/ops_repo_account_cyber_test.go` | +256/-0 | 实现（含测试） | ☐ |
| P1 | `backend/internal/repository/ops_repo_dashboard.go` | +5/-5 | 实现 | ☐ |
| P1 | `backend/internal/repository/ops_repo_openai_token_stats.go` · `backend/internal/repository/ops_repo_openai_token_stats_test.go` | +21/-3 | 实现（含测试） | ☐ |
| P1 | `backend/internal/repository/ops_repo_preagg.go` | +5/-5 | 实现 | ☐ |
| P1 | `backend/internal/repository/ops_repo_request_details.go` · `backend/internal/repository/ops_repo_request_details_test.go` | +138/-16 | 实现（含测试） | ☐ |
| P1 | `backend/internal/repository/ops_repo_trends.go` · `backend/internal/repository/ops_repo_trends_test.go` | +209/-30 | 实现（含测试） | ☐ |
| P1 | `backend/internal/server/middleware/ip_security.go` | +19/-0 | 中间件 | ☐ |
| P1 | `backend/internal/server/middleware/server_timing.go` · `backend/internal/server/middleware/server_timing_test.go` | +10/-0 | 中间件（含测试） | ☐ |
| P1 | `backend/internal/handler/admin/ops_system_log_handler_test.go` | +40/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/ops_capture_writer_nil_test.go` | +391/-25 | 单元测试 | ☐ |
| P1 | `backend/internal/handler/ops_ingress_reject_capture_test.go` | +1/-1 | 单元测试 | ☐ |
| P1 | `backend/internal/pkg/logger/stdlog_bridge_test.go` | +4/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/repository/ops_error_where_test.go` | +66/-14 | 单元测试 | ☐ |
| P1 | `backend/internal/repository/ops_repo_args_test.go` | +10/-1 | 单元测试 | ☐ |
| P1 | `backend/internal/repository/ops_repo_error_where_test.go` | +12/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/repository/ops_repo_recovered_telemetry_test.go` | +63/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/repository/ops_repo_replay_cleanup_test.go` | +0/-19 | 单元测试 | ☐ |
| P1 | `backend/internal/repository/ops_repo_resolution_test.go` | +42/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/repository/ops_repo_system_logs_test.go` | +15/-0 | 单元测试 | ☐ |
| P1 | `backend/internal/repository/ops_write_pressure_integration_test.go` | +75/-6 | 集成测试 | ☐ |
| P1 | `backend/internal/securityaudit/prompt_repository_integration_test.go` | +1/-1 | 集成测试 | ☐ |
| P1 | `backend/internal/server/routes/prompt_audit_route_coverage_test.go` | +12/-9 | 单元测试 | ☐ |

## 30. 容量预测（建议批次 9，可在批次 2 后随时插入，整体重要度 P2）

账号容量预测数据层与管理端只读接口，独立性极佳，随 service 层 Capacity-Forecast 模块一起迁移即可。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P2 | `backend/internal/handler/admin/capacity_handler.go` | +122/-0 | handler | ☐ |
| P2 | `backend/internal/repository/capacity_forecast_repo.go` | +296/-0 | 仓储实现 | ☐ |

## 31. 备份系统（建议批次 9，整体重要度 P2）

数据库备份：pg_dump 封装加固与备份记录表。独立小批次。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P2 | `backend/internal/handler/admin/backup_handler.go` | +5/-5 | handler | ☐ |
| P2 | `backend/internal/repository/backup_pg_dumper.go` · `backend/internal/repository/backup_pg_dumper_test.go` | +31/-10 | 实现（含测试） | ☐ |
| P2 | `backend/internal/repository/backup_record_repo.go` · `backend/internal/repository/backup_record_repo_test.go` | +563/-0 | 仓储实现（含测试） | ☐ |

## 32. 公共工具库（随需迁移，整体重要度 P2）

小型无状态工具：URL 校验（SSRF 防护）、日志脱敏、响应头工具、入站 base URL 推导等。被多个模块引用，体量小，建议在批次 2 一次性带入。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P2 | `backend/internal/handler/request_base_url.go` · `backend/internal/handler/request_base_url_test.go` | +117/-0 | 实现（含测试） | ☐ |
| P2 | `backend/internal/pkg/ip/ip.go` | +13/-0 | 实现 | ☐ |
| P2 | `backend/internal/util/httputil/httputil.go` · `backend/internal/util/httputil/httputil_test.go` | +20/-1 | 实现（含测试） | ☐ |
| P2 | `backend/internal/util/logredact/redact.go` | +4/-2 | 实现 | ☐ |
| P2 | `backend/internal/util/responseheaders/responseheaders.go` · `backend/internal/util/responseheaders/responseheaders_test.go` | +37/-0 | 实现（含测试） | ☐ |
| P2 | `backend/internal/util/urlvalidator/validator.go` · `backend/internal/util/urlvalidator/validator_test.go` | +134/-5 | 实现（含测试） | ☐ |
| P2 | `backend/internal/repository/github_release_service_test.go` | +1/-73 | 单元测试 | ☐ |
| P2 | `backend/internal/repository/time_test.go` | +9/-0 | 单元测试 | ☐ |

## 33. 装配与依赖（每批次都要碰，整体重要度 P0）

wire DI 装配、路由注册、共享 DTO、domain 常量、config、构建与依赖清单。**不单独排期**：每个功能批次顺带增量追加对应注册项，批后 `go generate ./cmd/server` 校验；本节文件禁止整体覆盖粘贴，全部按增量 patch 处理。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `backend/.golangci.yml` | +4/-0 | 实现 | ☐ |
| P0 | `backend/Dockerfile` | +2/-1 | 实现 | ☐ |
| P0 | `backend/cmd/server/main.go` · `backend/cmd/server/main_test.go` | +164/-39 | 实现（含测试） | ☐ |
| P0 | `backend/cmd/server/wire.go` | +42/-13 | DI 装配 | ☐ |
| P0 | `backend/cmd/server/wire_gen.go` · `backend/cmd/server/wire_gen_test.go` | +249/-97 | 实现（含测试） | ☐ |
| P0 | `backend/go.mod` | +6/-3 | 实现 | ☐ |
| P0 | `backend/go.sum` | +7/-11 | 实现 | ☐ |
| P0 | `backend/internal/config/config.go` · `backend/internal/config/config_test.go` | +795/-297 | 实现（含测试） | ☐ |
| P0 | `backend/internal/domain/constants.go` · `backend/internal/domain/constants_test.go` | +37/-54 | 实现（含测试） | ☐ |
| P0 | `backend/internal/handler/dto/mappers.go` | +329/-179 | DTO | ☐ |
| P0 | `backend/internal/handler/dto/types.go` | +131/-92 | DTO | ☐ |
| P0 | `backend/internal/handler/handler.go` | +13/-2 | 实现 | ☐ |
| P0 | `backend/internal/handler/wire.go` | +263/-79 | DI 装配 | ☐ |
| P0 | `backend/internal/repository/wire.go` | +1246/-2 | DI 装配 | ☐ |
| P0 | `backend/internal/server/http.go` | +13/-0 | 实现 | ☐ |
| P0 | `backend/internal/server/middleware/backend_mode_guard.go` · `backend/internal/server/middleware/backend_mode_guard_test.go` | +131/-6 | 中间件（含测试） | ☐ |
| P0 | `backend/internal/server/middleware/cors.go` · `backend/internal/server/middleware/cors_test.go` | +7/-3 | 中间件（含测试） | ☐ |
| P0 | `backend/internal/server/middleware/middleware.go` | +2/-0 | 中间件 | ☐ |
| P0 | `backend/internal/server/middleware/wire.go` | +13/-1 | 中间件 | ☐ |
| P0 | `backend/internal/server/router.go` | +6/-3 | 实现 | ☐ |
| P0 | `backend/internal/server/routes/admin.go` | +171/-37 | 路由注册 | ☐ |
| P0 | `backend/internal/server/routes/common.go` · `backend/internal/server/routes/common_test.go` | +488/-7 | 路由注册（含测试） | ☐ |
| P0 | `backend/internal/server/routes/gateway.go` · `backend/internal/server/routes/gateway_test.go` | +431/-418 | 路由注册（含测试） | ☐ |
| P0 | `backend/internal/server/routes/user.go` | +53/-3 | 路由注册 | ☐ |
| P0 | `backend/internal/setup/setup.go` · `backend/internal/setup/setup_test.go` | +48/-156 | 实现（含测试） | ☐ |
| P0 | `backend/internal/testutil/stubs.go` | +9/-0 | 实现 | ☐ |
| P0 | `backend/internal/web/embed_on.go` | +10/-9 | 实现 | ☐ |
| P0 | `backend/internal/config/deploy_security_contract_test.go` | +89/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/config/webauthn_test.go` | +1/-1 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/dto/mappers_usage_test.go` | +59/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/handler/page_handler_test.go` | +20/-14 | 单元测试 | ☐ |
| P0 | `backend/internal/server/api_contract_test.go` | +748/-403 | 单元测试 | ☐ |
| P0 | `backend/internal/server/routes/admin_missing_feature_routes_test.go` | +62/-0 | 单元测试 | ☐ |
| P0 | `backend/internal/web/embed_test.go` | +29/-0 | 单元测试 | ☐ |

## 完整性核对

核对脚本（临时置于 `/tmp/migration-analysis/check.py`，不纳入仓库）从本文档提取所有 `` `backend/…` `` 反引号路径，解析 numstat 的 rename 语法（`{旧 => 新}` 归一为新路径）后做集合 diff。

- numstat 文件总数：**1151**（去重后 1151）
- 本文档收录唯一路径：**1151**，路径提及次数：1151
- 遗漏（在 numstat 不在文档）：**0**
- 多余（在文档不在 numstat）：**0**
- 重复提及：**0**
- 模块数（章节）：**33**

按重要度统计（每个文件按其所在行的重要度归一计数）：

| 重要度 | 文件数 |
|---|---|
| P0 | 547 |
| P1 | 545 |
| P2 | 34 |
| P3 | 25 |
| 合计 | 1151 |

**核对结果：通过 1151/1151，零遗漏、零多余、零重复。**
