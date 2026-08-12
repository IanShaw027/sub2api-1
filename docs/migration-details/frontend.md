# 迁移明细 C：前端（frontend）

> 基准：git diff upstream/main...personal-dev --numstat，共 609 个文件。
> 重要度：P0 核心（缺了核心工作流不可用或安全回退）/ P1 重要（明显功能或稳定性增强）/ P2 可选（便利、锦上添花）/ P3 建议放弃（一次性产物、被上游取代、过时）。
> 取舍栏默认 ☐ 待定，由用户填写。
>
> 说明：测试文件（.spec.ts）与对应实现同行列出，行数栏用「测 +x/-y」标注；标注「删除」的行表示该文件在 personal-dev 中被删除，迁移时需同步执行删除动作。

## 1. 账号池增强：TLS 指纹 / 多平台 OAuth / 容量（建议批次 1，整体重要度 P0）

本 fork 的核心价值所在：管理端上游账号池的全部增强，包括 Kiro（AWS）新平台授权流、Grok/Gemini/Antigravity/OpenAI OAuth 扩展、TLS 指纹（档案/策略/路由器）、平台容量对话框、临时不可调度规则、响应改写、自定义错误码、代理池增强等，共 119 个文件。对应后端「管理端账号/OAuth + TLS 指纹 + 容量」模块，前后端必须同批落地。建议整目录搬迁 components/account 与 components/admin/account，再增量合入 api 层改动。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `frontend/src/api/admin/accounts.ts` | +402/-48 | 管理端账号 CRUD/批量操作/用量批查 API，新增 TLS 指纹绑定、临时不可调度等接口 | ☐ 待定 |
| P0 | `frontend/src/api/admin/capacity.ts` | +139/-0 | 平台容量查询 API（新增），支撑容量对话框 | ☐ 待定 |
| P0 | `frontend/src/api/admin/kiro.ts` | +171/-0 | Kiro（AWS Builder ID/IdC）授权与账号管理 API（新增平台） | ☐ 待定 |
| P0 | `frontend/src/api/admin/tlsFingerprintPolicy.ts` | +73/-0 | TLS 指纹策略（全局/平台模式切换）API | ☐ 待定 |
| P0 | `frontend/src/api/admin/tlsFingerprintProfile.ts` | +74/-1 | TLS 指纹档案管理 API | ☐ 待定 |
| P0 | `frontend/src/api/admin/tlsFingerprintRouter.ts`<br>`frontend/src/api/admin/__tests__/tlsFingerprintRouter.spec.ts` | +122/-0；测 +60/-0 | TLS 指纹路由器（collector 出口节点）API 及契约测试 | ☐ 待定 |
| P1 | `frontend/src/api/admin/grok.ts`<br>`frontend/src/api/admin/__tests__/grok.spec.ts`<br>`frontend/src/api/__tests__/admin.grok.spec.ts` | +86/-88；测 +38/-0、0/-53（旧测删除） | Grok 账号 OAuth/配额探测 API 重构；旧位置测试删除、迁至 admin 子目录 | ☐ 待定 |
| P1 | `frontend/src/api/admin/gemini.ts` | +9/-16 | Gemini OAuth API 调整（配合 OAuth 类型扩展） | ☐ 待定 |
| P1 | `frontend/src/api/admin/antigravity.ts` | +2/-0 | Antigravity OAuth API 微调 | ☐ 待定 |
| P1 | `frontend/src/api/admin/proxies.ts` | +44/-0 | 代理池 API 增强（批量导入/轮换策略） | ☐ 待定 |
| P1 | `frontend/src/api/__tests__/settings.kiroRuntime.spec.ts` | +125/-0 | Kiro 运行时设置 API 契约测试（对应 settings API 的 Kiro 段） | ☐ 待定 |
| P0 | `frontend/src/components/account/CreateAccountModal.vue`<br>`frontend/src/components/account/__tests__/CreateAccountModal.spec.ts` | +1704/-761；测 +1386/-200 | 管理员新建上游账号弹窗，承载所有平台/所有账号类型的创建流程，改动极重 | ☐ 待定 |
| P0 | `frontend/src/components/account/EditAccountModal.vue`<br>`frontend/src/components/account/__tests__/EditAccountModal.spec.ts`<br>`frontend/src/components/account/__tests__/EditAccountModal.grokUpstream.spec.ts` | +2368/-1208；测 +1930/-523、+18/-1 | 管理员编辑账号弹窗（近乎重写）：TLS 指纹绑定、模型映射、临时不可调度、响应改写等全部入口 | ☐ 待定 |
| P0 | `frontend/src/components/account/KiroAuthorizationFlow.vue`<br>`frontend/src/components/account/__tests__/KiroAuthorizationFlow.spec.ts` | +1071/-0；测 +313/-0 | Kiro 平台授权向导（设备码/回调轮询），管理员添加 Kiro 账号的核心流程 | ☐ 待定 |
| P0 | `frontend/src/components/account/OAuthAuthorizationFlow.vue`<br>`frontend/src/components/account/__tests__/OAuthAuthorizationFlow.spec.ts` | +350/-180；测 +244/-0 | 通用 OAuth 授权流组件（各平台复用），管理员账号授权主路径 | ☐ 待定 |
| P0 | `frontend/src/components/account/TLSFingerprintBindingMatrix.vue`<br>`frontend/src/components/account/__tests__/TLSFingerprintBindingMatrix.spec.ts` | +200/-0；测 +87/-0 | 账号级 TLS 指纹绑定矩阵（按端点选择档案/路由器），管理员用 | ☐ 待定 |
| P0 | `frontend/src/components/account/TLSFingerprintPolicyModeControl.vue`<br>`frontend/src/components/account/__tests__/TLSFingerprintPolicyModeControl.spec.ts` | +38/-0；测 +25/-0 | TLS 指纹策略模式切换控件（继承/强制/关闭），管理员用 | ☐ 待定 |
| P0 | `frontend/src/components/account/tlsFingerprintProfileOptions.ts`<br>`frontend/src/components/account/__tests__/tlsFingerprintProfileOptions.spec.ts` | +138/-0；测 +117/-0 | TLS 指纹档案下拉选项构建逻辑（含路由器分组） | ☐ 待定 |
| P1 | `frontend/src/components/account/AccountUsageCell.vue`<br>`frontend/src/components/account/__tests__/AccountUsageCell.spec.ts` | +773/-688；测 +2134/-368 | 账号列表用量单元格（多平台配额窗口/5h 窗口/重置倒计时），管理员账号页信息密度核心 | ☐ 待定 |
| P1 | `frontend/src/components/account/AccountQuotaInfo.vue`<br>`frontend/src/components/account/__tests__/AccountQuotaInfo.spec.ts` | +290/-105；测 +199/-0 | 账号配额详情展示块（各平台配额/限额），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/account/AccountStatusIndicator.vue`<br>`frontend/src/components/account/__tests__/AccountStatusIndicator.spec.ts` | +82/-26；测 +133/-56 | 账号状态指示灯（新增限流/临时不可调度等状态），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/account/AccountTestModal.vue`<br>`frontend/src/components/account/__tests__/AccountTestModal.spec.ts` | +339/-228；测 +47/-15 | 账号连通性测试弹窗（多端点/流式测试），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/account/BulkEditAccountModal.vue`<br>`frontend/src/components/account/__tests__/BulkEditAccountModal.spec.ts` | +453/-97；测 +271/-135 | 账号批量编辑弹窗（注意：禁止跨平台混编模型映射），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/account/ReAuthAccountModal.vue` | +128/-36 | 账号重新授权弹窗（旧版通用），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/account/GrokQuotaProbeCell.vue`<br>`frontend/src/components/account/__tests__/GrokQuotaProbeCell.spec.ts` | +50/-19；测 +10/-6 | Grok 配额探测结果单元格，管理员账号列表用 | ☐ 待定 |
| P1 | `frontend/src/components/account/KiroDiagnosticChips.vue`<br>`frontend/src/components/account/__tests__/KiroDiagnosticChips.spec.ts` | +87/-0；测 +37/-0 | Kiro 账号诊断标签（区域/订阅类型等），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/account/ModelMappingEditor.vue` | +112/-0 | 账号级模型映射编辑器（请求模型→上游模型），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/account/ModelWhitelistSelector.vue` | +3/-1 | 模型白名单选择器微调，管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/account/CustomErrorCodesForm.vue`<br>`frontend/src/components/account/customErrorCodes.ts` | +147/-0、+25/-0 | 账号自定义错误码规则表单及解析逻辑（指定上游错误码的处置方式），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/account/TempUnschedRulesForm.vue` | +264/-0 | 临时不可调度规则表单（按错误特征自动摘除账号），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/account/tempUnschedRules.ts`<br>`frontend/src/components/account/__tests__/tempUnschedRules.spec.ts` | +103/-0；测 +496/-0 | 临时不可调度规则的序列化/校验逻辑及大量用例 | ☐ 待定 |
| P1 | `frontend/src/components/account/TempUnschedStatusModal.vue`<br>`frontend/src/components/account/__tests__/TempUnschedStatusModal.spec.ts` | +84/-52；测 +213/-0 | 账号临时不可调度状态查看/解除弹窗，管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/account/UsageProgressBar.vue`<br>`frontend/src/components/account/__tests__/UsageProgressBar.spec.ts` | +37/-28；测 +28/-26 | 账号用量进度条（阈值配色调整），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/account/responseRewriteRules.ts`<br>`frontend/src/components/account/__tests__/responseRewriteRules.spec.ts` | +110/-0；测 +88/-0 | 账号级响应改写规则的构建/校验逻辑 | ☐ 待定 |
| P1 | `frontend/src/components/account/textEndpointAutoRoute.ts`<br>`frontend/src/components/account/__tests__/textEndpointAutoRoute.spec.ts` | +81/-0；测 +52/-0 | 文本端点自动路由配置逻辑（按模型分流到不同端点） | ☐ 待定 |
| P1 | `frontend/src/components/account/credentialsBuilder.ts`<br>`frontend/src/components/account/__tests__/credentialsBuilder.spec.ts` | +7/-2；测 0/-2 | 账号凭据构建器小改（配合新平台字段） | ☐ 待定 |
| P2 | `frontend/src/components/account/CodexInviteResetModal.vue` | +402/-0 | OpenAI Codex 邀请重置工具弹窗，管理员用（便利工具） | ☐ 待定 |
| P0 | `frontend/src/components/admin/TLSFingerprintProfilesModal.vue`<br>`frontend/src/components/admin/__tests__/TLSFingerprintProfilesModal.spec.ts` | +101/-596；测 +100/-0 | TLS 指纹档案管理弹窗（重构瘦身，路由器逻辑拆出），管理员用 | ☐ 待定 |
| P0 | `frontend/src/components/admin/TLSFingerprintRoutersModal.vue`<br>`frontend/src/components/admin/__tests__/TLSFingerprintRoutersModal.spec.ts` | +576/-0；测 +278/-0 | TLS 指纹路由器管理弹窗（collector 节点增删/健康状态），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/PlatformDefaultAccountModelConfigForm.vue` | +260/-0 | 平台级默认账号模型配置表单（新账号默认模型映射），管理员设置页用 | ☐ 待定 |
| P0 | `frontend/src/components/admin/account/ReAuthAccountModal.vue`<br>`frontend/src/components/admin/account/__tests__/ReAuthAccountModal.spec.ts`<br>`frontend/src/components/admin/account/__tests__/ReAuthAccountModal.grok.spec.ts` | +553/-143；测 +1020/-0、+6/-3 | 管理端账号重新授权弹窗（多平台 OAuth 续期主入口），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/account/AccountActionMenu.vue`<br>`frontend/src/components/admin/account/__tests__/AccountActionMenu.spec.ts` | +46/-18；测 +108/-0 | 账号行操作菜单（新增测试/重授权/临时摘除等入口），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/account/AccountCyberEventsModal.vue`<br>`frontend/src/components/admin/account/__tests__/AccountCyberEventsModal.spec.ts` | +124/-0；测 +98/-0 | 账号风控/异常事件时间线弹窗，管理员排障用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/account/AccountStatsModal.vue`<br>`frontend/src/components/admin/account/__tests__/AccountStatsModal.spec.ts` | +3/-2；测 +85/-0 | 账号统计弹窗微调 + 补测试，管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/account/AccountTableFilters.vue` | +9/-1 | 账号列表筛选条（新增平台/状态筛选项），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/account/AccountTestModal.vue`<br>`frontend/src/components/admin/account/__tests__/AccountTestModal.spec.ts` | +354/-739；测 +74/-21 | 管理端账号测试弹窗重构（与账号目录内组件整合、删冗余），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/account/ImportDataModal.vue`<br>`frontend/src/__tests__/integration/data-import.spec.ts` | +190/-66；测 +276/-3 | 账号数据批量导入弹窗（JSON 导入校验增强）及跨模块集成测试，管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/account/OpenAIOAuthCapacityDialog.vue`<br>`frontend/src/components/admin/account/__tests__/OpenAIOAuthCapacityDialog.spec.ts` | +798/-0；测 +354/-0 | OpenAI OAuth 账号容量总览对话框（席位/配额分布），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/account/PlatformCapacityDialog.vue`<br>`frontend/src/components/admin/account/__tests__/PlatformCapacityDialog.spec.ts` | +885/-0；测 +205/-0 | 全平台容量总览对话框（各平台账号池水位），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/account/ScheduledTestsPanel.vue` | +6/-6 | 定时测试面板微调，管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/proxy/ImportDataModal.vue` | +5/-3 | 代理批量导入弹窗微调，管理员用 | ☐ 待定 |
| P0 | `frontend/src/composables/useAccountOAuth.ts`<br>`frontend/src/composables/__tests__/useAccountOAuth.spec.ts` | +23/-15；测 +73/-0 | 通用账号 OAuth 流程 composable（各平台授权共用状态机） | ☐ 待定 |
| P0 | `frontend/src/composables/useKiroOAuth.ts`<br>`frontend/src/composables/__tests__/useKiroOAuth.spec.ts` | +345/-0；测 +124/-0 | Kiro 平台 OAuth composable（设备码授权轮询，新增平台核心） | ☐ 待定 |
| P1 | `frontend/src/composables/useAntigravityOAuth.ts`<br>`frontend/src/composables/__tests__/useAntigravityOAuth.spec.ts` | +46/-9；测 +25/-21 | Antigravity OAuth composable 增强 | ☐ 待定 |
| P1 | `frontend/src/composables/useGeminiOAuth.ts`<br>`frontend/src/composables/__tests__/useGeminiOAuth.spec.ts` | +175/-30；测 +308/-0 | Gemini OAuth composable（区分 OAuth 类型/项目选择） | ☐ 待定 |
| P1 | `frontend/src/composables/useGrokOAuth.ts`<br>`frontend/src/composables/__tests__/useGrokOAuth.spec.ts` | +49/-52；测 +51/-15 | Grok OAuth composable 重构 | ☐ 待定 |
| P1 | `frontend/src/composables/useOpenAIOAuth.ts`<br>`frontend/src/composables/__tests__/useOpenAIOAuth.spec.ts` | +48/-3；测 +33/-0 | OpenAI OAuth composable 增强（容量/邀请相关） | ☐ 待定 |
| P1 | `frontend/src/composables/useModelWhitelist.ts`<br>`frontend/src/composables/__tests__/useModelWhitelist.spec.ts` | +168/-35；测 +78/-26 | 模型白名单 composable（多平台模型清单来源整合） | ☐ 待定 |
| P1 | `frontend/src/utils/accountUsageRefresh.ts`<br>`frontend/src/utils/__tests__/accountUsageRefresh.spec.ts` | +29/-1；测 +27/-0 | 账号用量刷新节流/批处理逻辑 | ☐ 待定 |
| P1 | `frontend/src/utils/geminiExtra.ts`<br>`frontend/src/utils/__tests__/geminiExtra.spec.ts` | +53/-0；测 +43/-0 | Gemini 账号附加字段（项目/tier）解析工具 | ☐ 待定 |
| P1 | `frontend/src/utils/geminiOAuthType.ts`<br>`frontend/src/utils/__tests__/geminiOAuthType.spec.ts` | +59/-0；测 +30/-0 | Gemini OAuth 类型判别工具（Code Assist / AI Studio 等） | ☐ 待定 |
| P2 | `frontend/src/utils/accountConcurrency.ts`<br>`frontend/src/utils/__tests__/accountConcurrency.spec.ts` | +7/-0；测 +10/-0 | 账号并发数展示小工具 | ☐ 待定 |
| P2 | `frontend/src/utils/accountSelection.ts`<br>`frontend/src/utils/__tests__/accountSelection.spec.ts` | +4/-1；测 +25/-0 | 账号列表勾选辅助逻辑小改 | ☐ 待定 |
| P2 | `frontend/src/utils/oauthAccountName.ts` | +47/-0 | OAuth 授权后自动生成账号名的工具 | ☐ 待定 |
| P0 | `frontend/src/views/admin/AccountsView.vue`<br>`frontend/src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts`<br>`frontend/src/views/admin/__tests__/AccountsView.schedulerScore.spec.ts`<br>`frontend/src/views/admin/__tests__/AccountsView.sparkShadow.spec.ts`<br>`frontend/src/views/admin/__tests__/AccountsView.usageBatch.spec.ts` | +928/-217；测 +571/-410、+90/-4、+175/-14、+551/-0 | 管理端账号池主页面（本 fork 最常用页面）：批量编辑、调度评分列、用量批查、容量入口等 | ☐ 待定 |
| P1 | `frontend/src/views/admin/ProxiesView.vue`<br>`frontend/src/views/admin/__tests__/ProxiesView.spec.ts` | +152/-61；测 +446/-0 | 管理端代理池页面（批量导入/轮换/健康检查），管理员用 | ☐ 待定 |

## 2. 通用组件 / 图表 / 公告 / 仪表盘 / API Key（建议批次 1，整体重要度 P1）

随基础设施先行落地的通用 UI 层，共 80 个文件：common 目录约 30 个小改组件、图表组件、公告系统、管理员与用户仪表盘、API Key 管理页、合规确认弹窗及少量通用工具。无独立后端模块（公告/仪表盘对应后端已有接口的小改）。建议随批次 1 一次性合入，改动大多为小 diff，冲突风险低；其中 sanitize/safeImageUrl 属 XSS 防护性质，优先保留。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `frontend/src/api/admin/dashboard.ts` | +7/-3 | 管理员仪表盘 API 小改（新统计字段） | ☐ 待定 |
| P1 | `frontend/src/views/admin/DashboardView.vue`<br>`frontend/src/views/admin/__tests__/DashboardView.spec.ts` | +257/-102；测 +127/-20 | 管理员总览仪表盘（收入/用量/账号健康卡片扩展），管理员用 | ☐ 待定 |
| P1 | `frontend/src/views/user/DashboardView.vue`<br>`frontend/src/views/user/__tests__/DashboardView.spec.ts` | +106/-16；测 +117/-0 | 用户仪表盘首页（余额/配额/近期用量概览），普通用户用 | ☐ 待定 |
| P1 | `frontend/src/components/user/dashboard/UserDashboardStats.vue` | +80/-85 | 用户仪表盘统计卡片重构（多平台配额展示），普通用户用 | ☐ 待定 |
| P2 | `frontend/src/components/user/dashboard/UserDashboardRecentUsage.vue` | +10/-2 | 用户仪表盘近期用量列表小改，普通用户用 | ☐ 待定 |
| P1 | `frontend/src/components/charts/EndpointDistributionChart.vue`<br>`frontend/src/components/charts/__tests__/EndpointDistributionChart.spec.ts` | +20/-2；测 +76/-0 | 端点分布饼图（仪表盘用）+ 补测试 | ☐ 待定 |
| P1 | `frontend/src/components/charts/GroupDistributionChart.vue`<br>`frontend/src/components/charts/__tests__/GroupDistributionChart.spec.ts` | +46/-7；测 +31/-6 | 分组用量分布图增强（仪表盘用） | ☐ 待定 |
| P1 | `frontend/src/components/charts/ModelDistributionChart.vue`<br>`frontend/src/components/charts/__tests__/ModelDistributionChart.spec.ts` | +44/-5；测 +44/-1 | 模型用量分布图增强（仪表盘用） | ☐ 待定 |
| P2 | `frontend/src/api/announcements.ts`<br>`frontend/src/api/admin/announcements.ts` | +3/-4、+3/-0 | 公告 API 小改（已读状态等） | ☐ 待定 |
| P2 | `frontend/src/components/common/AnnouncementBell.vue` | +103/-15 | 顶栏公告铃铛（未读角标/下拉列表），全体用户用 | ☐ 待定 |
| P2 | `frontend/src/components/common/AnnouncementPopup.vue`<br>`frontend/src/components/common/__tests__/AnnouncementPopup.spec.ts` | +47/-13；测 +6/-3 | 公告弹窗展示增强，全体用户用 | ☐ 待定 |
| P2 | `frontend/src/components/admin/announcements/AnnouncementReadStatusDialog.vue`<br>`frontend/src/components/admin/announcements/__tests__/AnnouncementReadStatusDialog.spec.ts` | +23/-2；测 +43/-0 | 公告已读名单对话框，管理员用 | ☐ 待定 |
| P2 | `frontend/src/stores/announcements.ts`<br>`frontend/src/stores/__tests__/announcements.spec.ts` | +26/-7；测 +56/-0 | 公告 Pinia store（未读计数/弹窗节奏） | ☐ 待定 |
| P2 | `frontend/src/views/admin/AnnouncementsView.vue` | +61/-2 | 管理端公告管理页（已读统计入口等），管理员用 | ☐ 待定 |
| P1 | `frontend/src/api/keys.ts` | +21/-0 | 用户 API Key 接口扩展（额度/限速字段） | ☐ 待定 |
| P1 | `frontend/src/views/user/KeysView.vue`<br>`frontend/src/views/user/__tests__/KeysView.spec.ts` | +146/-13；测 +96/-1 | 用户 API Key 管理页（创建/限额/统计入口），普通用户用 | ☐ 待定 |
| P1 | `frontend/src/components/keys/UseKeyModal.vue`<br>`frontend/src/components/keys/__tests__/UseKeyModal.spec.ts` | +265/-307；测 +180/-357 | 「使用 Key」示例弹窗重构（多端点/多客户端接入示例），普通用户用 | ☐ 待定 |
| P1 | `frontend/src/views/KeyUsageView.vue`<br>`frontend/src/views/__tests__/KeyUsageView.spec.ts` | +44/-4；测 +51/-9 | Key 用量公开查询页（凭 Key 查用量），Key 持有者用 | ☐ 待定 |
| P2 | `frontend/src/components/admin/AdminComplianceDialog.vue`<br>`frontend/src/stores/adminCompliance.ts` | +87/-50、+39/-2 | 管理员合规承诺确认弹窗及 store（423 强制确认门），管理员首次进入用 | ☐ 待定 |
| P1 | `frontend/src/components/common/BaseDialog.vue`<br>`frontend/src/components/common/__tests__/BaseDialog.spec.ts` | +5/-2；测 +1/-1 | 基础对话框微调（z-index/宽度档位），全站复用 | ☐ 待定 |
| P1 | `frontend/src/components/common/DataTable.vue` | +5/-5 | 通用数据表格微调，全站复用 | ☐ 待定 |
| P1 | `frontend/src/components/common/GroupSelector.vue` | +12/-3 | 分组选择器增强（平台过滤），管理端复用 | ☐ 待定 |
| P2 | `frontend/src/components/common/GroupBadge.vue` | +7/-0 | 分组徽章小改 | ☐ 待定 |
| P2 | `frontend/src/components/common/GroupCapacityBadge.vue` | +7/-46 | 分组容量徽章瘦身（逻辑上移） | ☐ 待定 |
| P2 | `frontend/src/components/common/GroupOptionItem.vue` | +2/-0 | 分组下拉选项微调 | ☐ 待定 |
| P2 | `frontend/src/components/common/HelpTooltip.vue`<br>`frontend/src/components/common/__tests__/HelpTooltip.spec.ts` | +4/-1；测 +12/-1 | 帮助提示组件微调 + 补测试 | ☐ 待定 |
| P2 | `frontend/src/components/common/ImageUpload.vue` | +6/-9 | 图片上传组件小改 | ☐ 待定 |
| P2 | `frontend/src/components/common/ModelIcon.vue` | +6/-3 | 模型图标组件小改（新模型识别） | ☐ 待定 |
| P2 | `frontend/src/components/common/NavigationProgress.vue`<br>`frontend/src/components/common/__tests__/NavigationProgress.spec.ts` | +3/-1；测 +11/-0 | 路由切换进度条微调 + 补测试 | ☐ 待定 |
| P2 | `frontend/src/components/common/Pagination.vue` | +1/-1 | 分页器微调 | ☐ 待定 |
| P1 | `frontend/src/components/common/PlatformIcon.vue`<br>`frontend/src/components/common/__tests__/PlatformIcon.spec.ts` | +12/-3；测 +37/-0 | 平台图标组件（新增 Kiro/Antigravity 等图标），全站复用 | ☐ 待定 |
| P1 | `frontend/src/components/common/PlatformTypeBadge.vue`<br>`frontend/src/components/common/__tests__/PlatformTypeBadge.spec.ts` | +181/-61；测 +142/-0 | 平台/账号类型徽章重构（多平台配色体系），全站复用 | ☐ 待定 |
| P1 | `frontend/src/components/common/platformBrandIcons.ts` | +16/-0 | 平台品牌图标注册表（新平台 SVG 映射） | ☐ 待定 |
| P1 | `frontend/src/components/common/ProxyRotationSelector.vue` | +127/-0 | 代理轮换策略选择器（账号/代理表单复用），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/common/ProxySelector.vue` | +17/-1 | 代理选择器增强（轮换选项联动），管理员用 | ☐ 待定 |
| P2 | `frontend/src/components/common/SearchInput.vue` | +24/-7 | 搜索输入框增强（防抖/清空按钮） | ☐ 待定 |
| P2 | `frontend/src/components/common/Select.vue` | +4/-3 | 下拉选择组件微调 | ☐ 待定 |
| P2 | `frontend/src/components/common/SupportQRCodesButton.vue`<br>`frontend/src/components/common/__tests__/SupportQRCodesButton.spec.ts` | +78/-0；测 +116/-0 | 「联系客服」二维码按钮（多二维码配置展示），全体用户用 | ☐ 待定 |
| P2 | `frontend/src/components/common/Toast.vue` | +3/-1 | 全局提示组件微调 | ☐ 待定 |
| P2 | `frontend/src/components/common/Toggle.vue` | +15/-5 | 开关组件微调（尺寸/禁用态） | ☐ 待定 |
| P2 | `frontend/src/components/Guide/steps.ts` | +1/-1 | 新手引导步骤文案微调 | ☐ 待定 |
| P2 | `frontend/src/composables/useOnboardingTour.ts` | +16/-2 | 新手引导 composable 小改 | ☐ 待定 |
| P1 | `frontend/src/composables/useAutoRefresh.ts`<br>`frontend/src/composables/__tests__/useAutoRefresh.spec.ts` | +25/-4；测 +63/-0 | 自动刷新 composable（页面可见性感知），列表页复用 | ☐ 待定 |
| P1 | `frontend/src/composables/useTableLoader.ts`<br>`frontend/src/composables/__tests__/useTableLoader.spec.ts` | +27/-2；测 +6/-2 | 表格加载 composable 增强（竞态防护） | ☐ 待定 |
| P2 | `frontend/src/composables/usePersistedPageSize.ts` | +4/-0 | 分页大小持久化小改 | ☐ 待定 |
| P2 | `frontend/src/utils/branding.ts`<br>`frontend/src/utils/__tests__/branding.spec.ts` | +6/-4；测 +9/-0 | 站点品牌名/标题工具小改 | ☐ 待定 |
| P2 | `frontend/src/utils/embedded-url.ts`<br>`frontend/src/utils/__tests__/embedded-url.spec.ts` | +1/-6；测 +3/-4 | 内嵌 URL 工具简化 | ☐ 待定 |
| P1 | `frontend/src/utils/platformColors.ts` | +30/-2 | 平台配色表（新增平台配色） | ☐ 待定 |
| P1 | `frontend/src/utils/safeImageUrl.ts`<br>`frontend/src/utils/__tests__/safeImageUrl.spec.ts` | +30/-0；测 +29/-0 | 图片 URL 安全校验工具（防 javascript:/data: 注入），安全性质 | ☐ 待定 |
| P1 | `frontend/src/utils/sanitize.ts`<br>`frontend/src/utils/__tests__/sanitize.spec.ts` | +32/-0；测 +32/-0 | HTML 内容净化工具（公告/自定义页 XSS 防护），安全性质 | ☐ 待定 |
| P1 | `frontend/src/utils/url.ts`<br>`frontend/src/utils/__tests__/url.spec.ts` | +37/-2；测 +33/-0 | URL 拼接/校验工具增强 | ☐ 待定 |
| P2 | `frontend/src/views/HomeView.vue` | +19/-41 | 站点落地页精简（按运行模式重定向调整），访客用 | ☐ 待定 |
| P2 | `frontend/src/views/NotFoundView.vue` | +5/-5 | 404 页微调 | ☐ 待定 |
| P2 | `frontend/src/views/user/CustomPageView.vue` | +38/-6 | 自定义内容页（渲染管理员配置的富文本页面，配合净化工具），普通用户用 | ☐ 待定 |

## 3. 管理端分组 / 用户 / 用量（建议批次 2，整体重要度 P1）

管理端三大资源页的增强，共 36 个文件：分组（渠道）配置大改（模型清单、OpenAI 图片计价、推理档位、平台回退、Codex 图片桥接开关）、用户管理（平台配额、余额历史）、用量查询（筛选/清理/请求类型标注）。对应后端「用户平台配额/分组 + 用量计费底座」模块。GroupsView 改动巨大（+1258/-1209），建议以 personal-dev 版本为底、对照上游增量人工合并。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `frontend/src/api/admin/groups.ts` | +11/-5 | 分组 API 小改（平台回退/图片计价字段） | ☐ 待定 |
| P1 | `frontend/src/views/admin/GroupsView.vue`<br>`frontend/src/views/admin/__tests__/GroupsView.columnSettings.spec.ts`<br>`frontend/src/views/admin/__tests__/GroupsView.editHydration.spec.ts`<br>`frontend/src/views/admin/__tests__/groupsReasoningEffort.spec.ts` | +1258/-1209；测 +28/-0、+734/-0、+3/-0 | 分组（渠道）管理主页面大改：列设置、编辑回填、推理档位、计价配置等，管理员用 | ☐ 待定 |
| P1 | `frontend/src/views/admin/groupsModelsList.ts`<br>`frontend/src/views/admin/__tests__/groupsModelsList.spec.ts` | +1/-1；测 +16/-0 | 分组模型清单构建逻辑微调 + 补测试 | ☐ 待定 |
| P1 | `frontend/src/views/admin/groupsOpenAIImagePricing.ts`<br>`frontend/src/views/admin/__tests__/groupsOpenAIImagePricing.spec.ts` | +114/-0；测 +154/-0 | 分组级 OpenAI 图片生成计价配置逻辑（尺寸/质量矩阵） | ☐ 待定 |
| P1 | `frontend/src/components/admin/group/GroupRPMOverridesModal.vue` | +7/-2 | 分组 RPM 覆盖弹窗微调，管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/group/GroupRateMultipliersModal.vue` | +7/-2 | 分组费率倍率弹窗微调，管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/group/__tests__/GroupPlatformFallback.spec.ts` | +172/-0 | 分组平台回退（主平台不可用时切换）行为测试 | ☐ 待定 |
| P1 | `frontend/src/components/admin/channel/codexImageGenerationBridge.ts`<br>`frontend/src/components/admin/channel/__tests__/codexImageGenerationBridge.spec.ts` | +26/-0；测 +21/-0 | Codex 图片生成桥接开关的三态（继承/启用/禁用）转换逻辑，分组配置用 | ☐ 待定 |
| P1 | `frontend/src/api/admin/users.ts` | +3/-1 | 用户管理 API 小改 | ☐ 待定 |
| P1 | `frontend/src/views/admin/UsersView.vue`<br>`frontend/src/views/admin/__tests__/UsersView.spec.ts` | +326/-217；测 +868/-53 | 用户管理页增强（平台配额、余额历史、批量操作），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/user/UserEditModal.vue`<br>`frontend/src/components/admin/user/UserEditModal.spec.ts` | +75/-12；测 +131/-0 | 用户编辑弹窗增强（配额/角色字段），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/user/UserPlatformQuotaModal.vue`<br>`frontend/src/components/admin/user/__tests__/UserPlatformQuotaModal.spec.ts` | +76/-26；测 +206/-8 | 用户平台配额设置弹窗（按平台限额），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/user/UserPlatformQuotaCell.vue` | +1/-1 | 用户平台配额单元格微调 | ☐ 待定 |
| P1 | `frontend/src/components/user/UserBalanceHistoryModal.vue` | +284/-0 | 用户余额变动历史弹窗（充值/扣费流水），管理员从用户页打开 | ☐ 待定 |
| P1 | `frontend/src/api/admin/usage.ts` | +8/-2 | 管理端用量查询 API 小改（新筛选参数） | ☐ 待定 |
| P1 | `frontend/src/api/usage.ts` | +6/-2 | 用户用量 API 小改 | ☐ 待定 |
| P1 | `frontend/src/views/admin/UsageView.vue`<br>`frontend/src/views/admin/__tests__/UsageView.spec.ts` | +206/-131；测 +414/-217 | 管理端用量日志页增强（筛选/详情/清理入口），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/usage/UsageCleanupDialog.vue`<br>`frontend/src/components/admin/usage/__tests__/UsageCleanupDialog.spec.ts` | +6/-0；测 +115/-0 | 用量日志清理确认弹窗 + 补测试，管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/usage/UsageFilters.vue`<br>`frontend/src/components/admin/usage/__tests__/UsageFilters.spec.ts` | +54/-37；测 +90/-220 | 用量筛选条重构（平台/请求类型/服务档位筛选），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/usage/UsageTable.vue`<br>`frontend/src/components/admin/usage/__tests__/UsageTable.spec.ts` | +93/-30；测 +199/-51 | 用量明细表格增强（新列/详情展开），管理员用 | ☐ 待定 |
| P1 | `frontend/src/utils/usageRequestType.ts`<br>`frontend/src/utils/__tests__/usageRequestType.spec.ts` | +12/-2；测 +18/-0 | 用量请求类型（文本/图片/实时等）显示映射工具 | ☐ 待定 |
| P2 | `frontend/src/views/user/UsageView.vue` | +3/-0 | 用户用量页微调，普通用户用 | ☐ 待定 |

## 4. AI 创作工作台 Studio（建议批次 3，整体重要度 P1）

全新的 AI 创作平台，共 40 个文件：五个工作台（图片/音频/视频/实时对话/生产编排）、用户侧 AI 对话/画廊/提示词库、管理端治理页（作品/提示词/存储计费），以及实时音频协议、WebSocket、运行时等一整套 utils。对应后端 AI-Studio-Skill 模块（ai/media 路由）。文件几乎全部为新增、与上游零冲突，可整目录搬迁；治理页可视需要延后。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `frontend/src/api/ai.ts`<br>`frontend/src/api/__tests__/ai.spec.ts` | +943/-0；测 +262/-0 | 用户侧 AI 生成 API 客户端（对话/图片/音频/视频/实时会话） | ☐ 待定 |
| P1 | `frontend/src/api/admin/ai.ts` | +320/-0 | 管理端 AI 治理 API（作品/提示词审核、存储计费） | ☐ 待定 |
| P1 | `frontend/src/api/admin/media.ts` | +30/-0 | 管理端媒体资源 API（存储对象管理） | ☐ 待定 |
| P1 | `frontend/src/api/studioProduction.ts`<br>`frontend/src/api/__tests__/studioProduction.progress.spec.ts` | +571/-0；测 +103/-0 | Studio 生产编排 API（任务提交/进度轮询/产物获取） | ☐ 待定 |
| P1 | `frontend/src/stores/aiStudio.ts`<br>`frontend/src/stores/__tests__/aiStudio.spec.ts` | +542/-0；测 +285/-0 | Studio 全局 store（会话/参数/历史/配额状态） | ☐ 待定 |
| P1 | `frontend/src/components/ai/AiPromptEditorDialog.vue` | +150/-0 | 提示词编辑对话框（保存到提示词库），Studio 用户用 | ☐ 待定 |
| P1 | `frontend/src/views/studio/StudioProductionWorkbench.vue` | +2171/-0 | 生产编排工作台（多步骤生成流水线编排与监控），Studio 主力页面 | ☐ 待定 |
| P1 | `frontend/src/views/studio/StudioImageWorkbench.vue` | +866/-0 | 图片生成工作台（参数面板/批量出图/画廊联动），Studio 用户用 | ☐ 待定 |
| P1 | `frontend/src/views/studio/StudioAudioWorkbench.vue` | +882/-0 | 音频生成工作台（TTS/音频参数与试听），Studio 用户用 | ☐ 待定 |
| P1 | `frontend/src/views/studio/StudioVideoWorkbench.vue`<br>`frontend/src/views/studio/__tests__/StudioVideoWorkbench.mount.spec.ts` | +665/-0；测 +153/-0 | 视频生成工作台（Sora 类任务提交/进度/预览），Studio 用户用 | ☐ 待定 |
| P1 | `frontend/src/views/studio/StudioRealtimeWorkbench.vue` | +1296/-0 | 实时语音对话工作台（WebRTC/WS 双向音频），Studio 用户用 | ☐ 待定 |
| P1 | `frontend/src/views/user/AIChatView.vue`<br>`frontend/src/views/user/__tests__/AIChatView.spec.ts` | +548/-0；测 +246/-0 | AI 对话页（流式聊天界面），普通用户用 | ☐ 待定 |
| P1 | `frontend/src/views/user/AIGalleryView.vue`<br>`frontend/src/views/user/__tests__/AIGalleryView.spec.ts` | +627/-0；测 +410/-0 | AI 作品画廊（个人生成历史浏览/管理），普通用户用 | ☐ 待定 |
| P1 | `frontend/src/views/user/AIPromptLibraryView.vue`<br>`frontend/src/views/user/__tests__/AIPromptLibraryView.spec.ts` | +441/-0；测 +188/-0 | 提示词库页（收藏/复用提示词），普通用户用 | ☐ 待定 |
| P2 | `frontend/src/views/admin/AIArtworkGovernanceView.vue` | +331/-0 | AI 作品治理页（违规内容审查/下架），管理员用 | ☐ 待定 |
| P2 | `frontend/src/views/admin/AIPromptGovernanceView.vue` | +335/-0 | AI 提示词治理页（敏感提示词审查），管理员用 | ☐ 待定 |
| P2 | `frontend/src/views/admin/AIStorageChargesView.vue` | +268/-0 | AI 存储计费页（媒体存储占用与扣费），管理员用 | ☐ 待定 |
| P1 | `frontend/src/utils/studioLive.ts`<br>`frontend/src/utils/__tests__/studioLive.spec.ts` | +254/-0；测 +115/-0 | Studio 实时会话生命周期管理（连接/重连/状态机） | ☐ 待定 |
| P1 | `frontend/src/utils/studioRealtimeAudio.ts`<br>`frontend/src/utils/__tests__/studioRealtimeAudio.spec.ts` | +539/-0；测 +136/-0 | 实时音频采集/播放/PCM 编解码工具 | ☐ 待定 |
| P1 | `frontend/src/utils/studioRealtimeProtocol.ts`<br>`frontend/src/utils/__tests__/studioRealtimeProtocol.spec.ts` | +316/-0；测 +127/-0 | 实时对话协议（事件序列化/服务端事件解析） | ☐ 待定 |
| P1 | `frontend/src/utils/studioRealtimeWs.ts`<br>`frontend/src/utils/__tests__/studioRealtimeWs.spec.ts` | +48/-0；测 +38/-0 | 实时 WebSocket 封装（鉴权/心跳） | ☐ 待定 |
| P1 | `frontend/src/utils/studioRuntime.ts`<br>`frontend/src/utils/__tests__/studioRuntime.spec.ts` | +67/-0；测 +114/-0 | Studio 运行时能力探测（可用模型/模态开关） | ☐ 待定 |
| P1 | `frontend/src/utils/studioQuery.ts`<br>`frontend/src/utils/__tests__/studioQuery.spec.ts` | +37/-0；测 +49/-0 | Studio 路由查询参数（工作台间跳转携带上下文）工具 | ☐ 待定 |
| P1 | `frontend/src/utils/studioVisibility.ts`<br>`frontend/src/utils/__tests__/studioVisibility.spec.ts` | +37/-0；测 +35/-0 | Studio 功能可见性判断（按公共设置开关显隐） | ☐ 待定 |
| P1 | `frontend/src/utils/studioKeyMask.ts` | +23/-0 | Studio 内 API Key 掩码显示工具 | ☐ 待定 |
| P1 | `frontend/src/utils/__tests__/studioModalityFlags.spec.ts` | +107/-0 | Studio 模态开关（图片/音频/视频/实时逐项启停）行为测试 | ☐ 待定 |

## 5. 技能中心 Skills（建议批次 4，整体重要度 P1）

技能（Prompt/工作流封装）市场平台，共 41 个文件：用户侧市场/详情/编辑器/我的技能/运行记录/版本管理，管理端治理/评审/运行监控/结算。对应后端 AI Skill 模块。全部为新增文件、零冲突，可整目录搬迁；若个人部署不做技能分成，结算/收益类页面可降级放弃。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `frontend/src/api/skills.ts`<br>`frontend/src/api/__tests__/skills.spec.ts` | +946/-0；测 +332/-0 | 用户侧技能 API（市场/安装/运行/版本） | ☐ 待定 |
| P1 | `frontend/src/api/admin/skills.ts`<br>`frontend/src/api/__tests__/admin.skills.spec.ts` | +726/-0；测 +280/-0 | 管理端技能 API（评审/治理/结算） | ☐ 待定 |
| P1 | `frontend/src/stores/skillsCenter.ts`<br>`frontend/src/stores/__tests__/skillsCenter.spec.ts` | +721/-0；测 +153/-0 | 技能中心 store（市场列表/已装技能/运行状态） | ☐ 待定 |
| P1 | `frontend/src/types/skills.ts` | +349/-0 | 技能域类型定义（技能/版本/运行/结算） | ☐ 待定 |
| P1 | `frontend/src/components/skills/SkillCard.vue`<br>`frontend/src/components/skills/__tests__/SkillCard.spec.ts` | +129/-0；测 +65/-0 | 技能卡片（市场/我的技能列表项），用户用 | ☐ 待定 |
| P1 | `frontend/src/components/skills/SkillCenterNav.vue` | +146/-0 | 技能中心子导航（市场/我的/运行/收益切换），用户用 | ☐ 待定 |
| P1 | `frontend/src/components/skills/SkillTypeEditor.vue` | +202/-0 | 技能类型编辑器（Prompt/工作流类型配置），技能作者用 | ☐ 待定 |
| P1 | `frontend/src/components/skills/SkillVariableForm.vue` | +138/-0 | 技能运行变量填写表单（按 schema 渲染），使用者用 | ☐ 待定 |
| P1 | `frontend/src/components/skills/SkillVariableSchemaEditor.vue` | +297/-0 | 技能变量 schema 编辑器（定义输入参数），技能作者用 | ☐ 待定 |
| P1 | `frontend/src/components/skills/paths.ts` | +12/-0 | 技能中心路由路径常量 | ☐ 待定 |
| P1 | `frontend/src/components/skills/presentation.ts` | +173/-0 | 技能展示辅助（状态/类型文案与配色映射） | ☐ 待定 |
| P2 | `frontend/src/components/skills/admin/SkillActionDialog.vue` | +144/-0 | 技能治理操作对话框（上架/下架/驳回），管理员用 | ☐ 待定 |
| P2 | `frontend/src/components/skills/admin/SkillAdminMetricGrid.vue` | +52/-0 | 技能管理指标网格（调用量/收益概览），管理员用 | ☐ 待定 |
| P2 | `frontend/src/components/skills/admin/SkillAdminStatusBadge.vue` | +72/-0 | 技能审核状态徽章，管理员用 | ☐ 待定 |
| P2 | `frontend/src/components/skills/admin/SkillAdminTimelineCard.vue` | +75/-0 | 技能审核时间线卡片，管理员用 | ☐ 待定 |
| P2 | `frontend/src/components/skills/admin/SkillReviewDetailCard.vue` | +141/-0 | 技能评审详情卡片（版本 diff/内容审查），管理员用 | ☐ 待定 |
| P1 | `frontend/src/views/user/SkillMarketView.vue`<br>`frontend/src/views/user/__tests__/SkillMarketView.spec.ts` | +388/-0；测 +144/-0 | 技能市场页（浏览/搜索/安装技能），普通用户用 | ☐ 待定 |
| P1 | `frontend/src/views/user/SkillDetailView.vue`<br>`frontend/src/views/user/__tests__/SkillDetailView.spec.ts` | +458/-0；测 +243/-0 | 技能详情页（说明/版本/评价/运行入口），普通用户用 | ☐ 待定 |
| P1 | `frontend/src/views/user/SkillEditorView.vue` | +313/-0 | 技能编辑器页（创建/编辑技能内容），技能作者用 | ☐ 待定 |
| P1 | `frontend/src/views/user/SkillMySkillsView.vue`<br>`frontend/src/views/user/__tests__/SkillMySkillsView.spec.ts` | +497/-0；测 +287/-0 | 我的技能页（已装/已发布管理），普通用户用 | ☐ 待定 |
| P1 | `frontend/src/views/user/SkillRunsView.vue`<br>`frontend/src/views/user/__tests__/SkillRunsView.spec.ts` | +316/-0；测 +268/-0 | 技能运行记录页（历史执行与结果），普通用户用 | ☐ 待定 |
| P1 | `frontend/src/views/user/SkillVersionsView.vue`<br>`frontend/src/views/user/__tests__/SkillVersionsView.spec.ts` | +488/-0；测 +236/-0 | 技能版本管理页（发版/回滚），技能作者用 | ☐ 待定 |
| P2 | `frontend/src/views/user/SkillRevenueView.vue` | +183/-0 | 技能收益页（作者分成流水），技能作者用 | ☐ 待定 |
| P1 | `frontend/src/views/user/__tests__/skillRoutePermissions.spec.ts` | +328/-0 | 技能路由权限（登录/角色门禁）测试 | ☐ 待定 |
| P2 | `frontend/src/views/admin/SkillGovernanceView.vue` | +607/-0 | 技能治理总览页（全量技能管控），管理员用 | ☐ 待定 |
| P2 | `frontend/src/views/admin/SkillReviewView.vue` | +486/-0 | 技能评审页（待审队列/审批操作），管理员用 | ☐ 待定 |
| P2 | `frontend/src/views/admin/SkillRuntimeMonitorView.vue`<br>`frontend/src/views/admin/__tests__/SkillRuntimeMonitorView.spec.ts` | +495/-0；测 +261/-0 | 技能运行时监控页（执行成功率/耗时），管理员用 | ☐ 待定 |
| P2 | `frontend/src/views/admin/SkillSettlementView.vue`<br>`frontend/src/views/admin/__tests__/SkillSettlementView.spec.ts` | +450/-0；测 +191/-0 | 技能结算页（作者分成结算处理），管理员用 | ☐ 待定 |
| P1 | `frontend/src/router/__tests__/skills-routes.spec.ts` | +168/-0 | 技能中心路由注册测试 | ☐ 待定 |
| P1 | `frontend/src/router/__tests__/skills-installed-entry.spec.ts` | +430/-0 | 「已安装技能」入口路由/跳转行为测试 | ☐ 待定 |

## 6. 工单系统（建议批次 5，整体重要度 P2）

用户与管理员双端工单（客服）系统，共 28 个文件：用户提单（费率/并发/退款/咨询等分类表单）、对话式回复、管理端处理与回复模板。对应后端 Ticket 模块。全部为新增文件、零冲突，可整目录搬迁；个人部署若无客服需求可整体放弃。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P2 | `frontend/src/api/tickets.ts` | +103/-0 | 用户侧工单 API（提单/回复/关闭） | ☐ 待定 |
| P2 | `frontend/src/api/adminTickets.ts` | +71/-0 | 管理端工单 API（处理/模板/分类） | ☐ 待定 |
| P2 | `frontend/src/utils/tickets.ts` | +89/-0 | 工单状态/分类显示工具 | ☐ 待定 |
| P2 | `frontend/src/components/tickets/TicketConversationPane.vue`<br>`frontend/src/components/tickets/__tests__/TicketConversationPane.spec.ts` | +342/-0；测 +489/-0 | 工单对话面板（双方消息流+回复框），双端共用 | ☐ 待定 |
| P2 | `frontend/src/components/tickets/TicketDetailPane.vue`<br>`frontend/src/components/tickets/__tests__/TicketDetailPane.spec.ts` | +69/-0；测 +80/-0 | 工单详情信息面板（元数据/状态），双端共用 | ☐ 待定 |
| P2 | `frontend/src/components/tickets/TicketEditorCard.vue`<br>`frontend/src/components/tickets/__tests__/TicketEditorCard.spec.ts` | +150/-0；测 +91/-0 | 工单回复编辑卡片（富文本/附件），双端共用 | ☐ 待定 |
| P2 | `frontend/src/components/tickets/TicketCategoryForm.vue` | +46/-0 | 工单分类选择表单，用户提单用 | ☐ 待定 |
| P2 | `frontend/src/components/tickets/TicketCreateDialog.vue` | +62/-0 | 快速提单对话框，用户用 | ☐ 待定 |
| P2 | `frontend/src/components/tickets/TicketInfoItem.vue` | +13/-0 | 工单信息条目小组件 | ☐ 待定 |
| P2 | `frontend/src/components/tickets/TicketReplyTemplatesDialog.vue` | +144/-0 | 回复模板管理对话框（常用话术），管理员用 | ☐ 待定 |
| P2 | `frontend/src/components/tickets/forms/TicketFormRate.vue` | +241/-0 | 「费率问题」提单表单（带用量证据字段），用户用 | ☐ 待定 |
| P2 | `frontend/src/components/tickets/forms/TicketFormConcurrency.vue` | +53/-0 | 「并发问题」提单表单，用户用 | ☐ 待定 |
| P2 | `frontend/src/components/tickets/forms/TicketFormRefund.vue` | +38/-0 | 「退款申请」提单表单，用户用 | ☐ 待定 |
| P2 | `frontend/src/components/tickets/forms/TicketFormConsult.vue` | +29/-0 | 「使用咨询」提单表单，用户用 | ☐ 待定 |
| P2 | `frontend/src/components/tickets/forms/TicketFormOther.vue` | +29/-0 | 「其他问题」提单表单，用户用 | ☐ 待定 |
| P2 | `frontend/src/views/user/TicketsView.vue`<br>`frontend/src/views/user/__tests__/TicketsView.spec.ts` | +353/-0；测 +305/-0 | 用户工单列表页，普通用户用 | ☐ 待定 |
| P2 | `frontend/src/views/user/TicketCreateView.vue`<br>`frontend/src/views/user/__tests__/TicketCreateView.spec.ts` | +100/-0；测 +113/-0 | 用户提单页（分类表单入口），普通用户用 | ☐ 待定 |
| P2 | `frontend/src/views/user/TicketDetailView.vue`<br>`frontend/src/views/user/__tests__/TicketDetailView.spec.ts` | +258/-0；测 +379/-0 | 用户工单详情/对话页，普通用户用 | ☐ 待定 |
| P2 | `frontend/src/views/admin/TicketsView.vue`<br>`frontend/src/views/admin/__tests__/TicketsView.spec.ts` | +258/-0；测 +75/-0 | 管理端工单队列页（待处理/指派），管理员用 | ☐ 待定 |
| P2 | `frontend/src/views/admin/TicketDetailView.vue`<br>`frontend/src/views/admin/__tests__/TicketDetailView.spec.ts` | +379/-0；测 +551/-0 | 管理端工单处理页（回复/改状态/退款联动），管理员用 | ☐ 待定 |

## 7. 发票 / 订单 / 支付增强（建议批次 6，整体重要度 P1）

SaaS 计费链路的全面增强，共 74 个文件：发票申请（用户申请+管理端审批）、订单页改造（用户/管理端）、支付流程加固（Stripe 内嵌/微信恢复/多币种显示）、订阅套餐微调、兑换码小改。对应后端 Invoice/Payment 模块。修改类文件较多，建议对照上游逐文件合并；simple 模式部署可整体降级。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `frontend/src/api/payment.ts` | +49/-3 | 用户支付 API 扩展（发票申请/订单状态） | ☐ 待定 |
| P1 | `frontend/src/api/admin/payment.ts`<br>`frontend/src/api/__tests__/admin.payment.spec.ts` | +60/-23；测 +12/-0 | 管理端支付 API（退款/发票审批/看板） | ☐ 待定 |
| P1 | `frontend/src/api/redeem.ts` | +25/-1 | 兑换码 API 小改 | ☐ 待定 |
| P1 | `frontend/src/types/payment.ts` | +63/-17 | 支付域类型扩展（发票/多币种/订单状态） | ☐ 待定 |
| P1 | `frontend/src/components/payment/paymentFlow.ts`<br>`frontend/src/components/payment/__tests__/paymentFlow.spec.ts` | +133/-16；测 +111/-0 | 支付流程状态机（下单→支付→轮询→结果）增强 | ☐ 待定 |
| P1 | `frontend/src/utils/paymentPlanValidity.ts` | +17/-0 | 套餐有效期计算工具 | ☐ 待定 |
| P1 | `frontend/src/utils/__tests__/paymentMethodDisplayKey.spec.ts` | +13/-0 | 支付方式显示键映射测试 | ☐ 待定 |
| P1 | `frontend/src/components/payment/OrderTable.vue`<br>`frontend/src/components/payment/__tests__/OrderTable.spec.ts` | +53/-16；测 +94/-0 | 用户订单表格增强（发票状态列/操作），普通用户用 | ☐ 待定 |
| P1 | `frontend/src/components/payment/PaymentMethodSelector.vue`<br>`frontend/src/components/payment/__tests__/PaymentMethodSelector.spec.ts` | +13/-25；测 +18/-47 | 支付方式选择器精简，普通用户用 | ☐ 待定 |
| P1 | `frontend/src/components/payment/PaymentProviderDialog.vue`<br>`frontend/src/components/payment/__tests__/PaymentProviderDialog.spec.ts` | +33/-41；测 +76/-0 | 支付渠道配置对话框调整，管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/payment/PaymentProviderList.vue`<br>`frontend/src/components/payment/__tests__/PaymentProviderList.spec.ts` | +3/-2；测 +72/-0 | 支付渠道列表微调 + 补测试，管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/payment/PaymentQRDialog.vue`<br>`frontend/src/components/payment/__tests__/PaymentQRDialog.spec.ts` | +8/-6；测 +35/-0 | 扫码支付对话框微调 + 补测试，普通用户用 | ☐ 待定 |
| P1 | `frontend/src/components/payment/PaymentStatusPanel.vue`<br>`frontend/src/components/payment/__tests__/PaymentStatusPanel.spec.ts` | +7/-6；测 +3/-3 | 支付状态轮询面板微调，普通用户用 | ☐ 待定 |
| P1 | `frontend/src/components/payment/ProviderCard.vue`<br>`frontend/src/components/payment/__tests__/ProviderCard.spec.ts` | +2/-1；测 +66/-0 | 支付渠道卡片微调 + 补测试，管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/payment/StripePaymentInline.vue`<br>`frontend/src/components/payment/__tests__/StripePaymentInline.spec.ts` | +6/-4；测 +148/-0 | Stripe 内嵌支付组件微调 + 补测试，普通用户用 | ☐ 待定 |
| P2 | `frontend/src/components/payment/SubscriptionPlanCard.vue`<br>`frontend/src/components/payment/__tests__/SubscriptionPlanCard.spec.ts` | 0/-1；测 +9/-9 | 订阅套餐卡片微调，普通用户用 | ☐ 待定 |
| P1 | `frontend/src/components/user/orders/OrdersTabBar.vue`<br>`frontend/src/components/user/orders/__tests__/OrdersTabBar.spec.ts` | +40/-0；测 +47/-0 | 订单页页签栏（订单/发票切换），普通用户用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/payment/AdminOrderDetail.vue`<br>`frontend/src/components/admin/payment/__tests__/AdminOrderDetail.spec.ts` | +12/-9；测 +51/-0 | 管理端订单详情抽屉（多币种金额显示），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/payment/AdminOrderTable.vue` | +11/-8 | 管理端订单表格微调，管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/payment/AdminRefundDialog.vue` | +40/-18 | 管理端退款对话框增强（部分退款/原因），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/payment/OrderStatsCards.vue`<br>`frontend/src/components/admin/payment/__tests__/OrderStatsCards.spec.ts` | +4/-2；测 +46/-0 | 订单统计卡片微调 + 补测试，管理员用 | ☐ 待定 |
| P2 | `frontend/src/components/admin/payment/PaymentMethodChart.vue` | +7/-4 | 支付方式占比图微调，管理员用 | ☐ 待定 |
| P2 | `frontend/src/components/admin/payment/TopUsersLeaderboard.vue` | +6/-3 | 充值排行榜微调，管理员用 | ☐ 待定 |
| P2 | `frontend/src/components/admin/payment/__tests__/PaymentDashboardWidgets.spec.ts` | +43/-0 | 支付看板小组件聚合测试 | ☐ 待定 |
| P1 | `frontend/src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts` | +140/-0 | 订单多币种金额显示逻辑测试 | ☐ 待定 |
| P1 | `frontend/src/views/admin/orders/AdminInvoiceApplicationsView.vue`<br>`frontend/src/views/admin/orders/__tests__/AdminInvoiceApplicationsView.spec.ts` | +310/-0；测 +224/-0 | 发票申请审批页（新功能），管理员用 | ☐ 待定 |
| P1 | `frontend/src/views/admin/orders/AdminOrdersView.vue`<br>`frontend/src/views/admin/orders/__tests__/AdminOrdersView.spec.ts` | +195/-56；测 +800/-0 | 管理端订单列表页增强（筛选/退款/发票联动），管理员用 | ☐ 待定 |
| P2 | `frontend/src/views/admin/orders/AdminPaymentDashboardView.vue`<br>`frontend/src/views/admin/orders/__tests__/AdminPaymentDashboardView.spec.ts` | +10/-5；测 +89/-0 | 支付数据看板页微调 + 补测试，管理员用 | ☐ 待定 |
| P1 | `frontend/src/views/admin/orders/AdminPaymentPlansView.vue` | +15/-1 | 套餐管理页微调，管理员用 | ☐ 待定 |
| P1 | `frontend/src/views/admin/orders/PlanEditDialog.vue` | +8/-6 | 套餐编辑对话框微调，管理员用 | ☐ 待定 |
| P2 | `frontend/src/views/admin/SubscriptionsView.vue` | +4/-7 | 订阅管理页微调，管理员用 | ☐ 待定 |
| P2 | `frontend/src/views/admin/subscriptionPlatformFilterOptions.ts`<br>`frontend/src/views/admin/__tests__/subscriptionPlatformFilterOptions.spec.ts` | +17/-0；测 +18/-0 | 订阅列表平台筛选选项构建逻辑 | ☐ 待定 |
| P2 | `frontend/src/stores/__tests__/subscriptions.spec.ts` | +6/-2 | 订阅 store 测试小改 | ☐ 待定 |
| P2 | `frontend/src/views/admin/PromoCodesView.vue` | +1/-0 | 优惠码管理页微调，管理员用 | ☐ 待定 |
| P2 | `frontend/src/views/admin/RedeemView.vue` | +3/-1 | 管理端兑换码页微调，管理员用 | ☐ 待定 |
| P2 | `frontend/src/views/user/RedeemView.vue` | +4/-19 | 用户兑换码页精简，普通用户用 | ☐ 待定 |
| P1 | `frontend/src/views/user/PaymentView.vue`<br>`frontend/src/views/user/__tests__/PaymentView.spec.ts` | +127/-67；测 +273/-104 | 用户充值/购买页改造（套餐+支付方式整合），普通用户用 | ☐ 待定 |
| P1 | `frontend/src/views/user/PaymentResultView.vue`<br>`frontend/src/views/user/__tests__/PaymentResultView.spec.ts` | +4/-2；测 +49/-1 | 支付结果页微调 + 补测试，普通用户用 | ☐ 待定 |
| P1 | `frontend/src/views/user/PaymentQRCodeView.vue`<br>`frontend/src/views/user/__tests__/PaymentQRCodeView.spec.ts` | +2/-1；测 +108/-0 | 扫码支付页微调 + 补测试，普通用户用 | ☐ 待定 |
| P1 | `frontend/src/views/user/StripePaymentView.vue`<br>`frontend/src/views/user/__tests__/StripePaymentView.spec.ts` | +100/-16；测 +171/-1 | Stripe 支付页增强（内嵌表单/3DS 回跳），普通用户用 | ☐ 待定 |
| P1 | `frontend/src/views/user/StripePopupView.vue`<br>`frontend/src/views/user/__tests__/StripePopupView.spec.ts` | +13/-44；测 +152/-0 | Stripe 弹窗支付页精简 + 补测试，普通用户用 | ☐ 待定 |
| P2 | `frontend/src/views/user/AirwallexPaymentView.vue`<br>`frontend/src/views/user/__tests__/AirwallexPaymentView.spec.ts` | +9/-5；测 +4/-3 | Airwallex 支付页微调，普通用户用 | ☐ 待定 |
| P1 | `frontend/src/views/user/UserInvoicesView.vue`<br>`frontend/src/views/user/__tests__/UserInvoicesView.spec.ts` | +159/-0；测 +118/-0 | 用户发票列表/申请页（新功能），普通用户用 | ☐ 待定 |
| P1 | `frontend/src/views/user/UserInvoiceDetailView.vue`<br>`frontend/src/views/user/__tests__/UserInvoiceDetailView.spec.ts` | +145/-0；测 +135/-0 | 用户发票详情页（状态/下载），普通用户用 | ☐ 待定 |
| P1 | `frontend/src/views/user/UserOrdersView.vue`<br>`frontend/src/views/user/__tests__/UserOrdersView.spec.ts` | +334/-12；测 +317/-0 | 用户订单页改造（页签/发票申请入口），普通用户用 | ☐ 待定 |
| P1 | `frontend/src/views/user/paymentWechatResume.ts`<br>`frontend/src/views/user/__tests__/paymentWechatResume.spec.ts` | +7/-8；测 +1/-1 | 微信内支付中断恢复逻辑微调 | ☐ 待定 |

## 8. 联盟返利（建议批次 7，可并入批次 6，整体重要度 P2）

推荐返利功能的小型增强，共 7 个文件：用户联盟页、管理端返利记录表、路由测试。对应后端 Affiliate 模块。规模小、独立性强，可随支付批次顺带迁移。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P2 | `frontend/src/api/admin/affiliate.ts` | +63/-0 | 管理端联盟返利 API（记录/结算） | ☐ 待定 |
| P2 | `frontend/src/api/admin/affiliates.ts` | +1/-0 | 联盟 API 旧文件微调（与新文件并存，迁移时确认合并） | ☐ 待定 |
| P2 | `frontend/src/views/user/AffiliateView.vue`<br>`frontend/src/views/user/__tests__/AffiliateView.spec.ts` | +117/-9；测 +106/-70 | 用户联盟推广页（邀请链接/返利流水），普通用户用 | ☐ 待定 |
| P2 | `frontend/src/views/admin/affiliates/AdminAffiliateRecordsTable.vue`<br>`frontend/src/views/admin/affiliates/__tests__/AdminAffiliateRecordsTable.spec.ts` | +4/-2；测 +131/-0 | 管理端返利记录表微调 + 补测试，管理员用 | ☐ 待定 |
| P2 | `frontend/src/router/__tests__/affiliate-routes.spec.ts` | +19/-0 | 联盟路由注册测试 | ☐ 待定 |

## 9. 渠道监控 v2 / Ops 运维观测（建议批次 8，整体重要度 P1）

上游已有渠道监控与 Ops 骨架，本模块是深度定制，共 62 个文件：channel-monitor-v2 设计体系重构（脉冲矩阵/趋势图/指标格瘦身）、监控配置对话框、可用性人工校准、用户端渠道状态页 v2、Ops 仪表盘（错误日志/请求详情/切换率趋势/Token 统计）。对应后端「运维观测 + 渠道监控」模块。注意：多数是对上游文件的修改，应做定制 diff 合并而非整文件搬迁。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `frontend/src/api/admin/channelMonitor.ts` | +33/-1 | 渠道监控 API 扩展（可用性校准/模板） | ☐ 待定 |
| P1 | `frontend/src/api/channelMonitorV2.ts` | 0/-12（删除） | 废弃的 v2 API 文件删除（逻辑并回主 API），迁移时同步删除 | ☐ 待定 |
| P1 | `frontend/src/constants/channelMonitor.ts` | +2/-0 | 渠道监控常量微调 | ☐ 待定 |
| P1 | `frontend/src/composables/useChannelMonitorFormat.ts`<br>`frontend/src/composables/__tests__/useChannelMonitorFormat.spec.ts` | +28/-10；测 +39/-0 | 监控数据格式化 composable（成功率/延迟展示） | ☐ 待定 |
| P1 | `frontend/src/router/__tests__/channel-monitor-routes.spec.ts` | +179/-0 | 渠道监控路由（含 feature flag 门禁）测试 | ☐ 待定 |
| P1 | `frontend/src/views/admin/ChannelMonitorView.vue`<br>`frontend/src/views/admin/__tests__/ChannelMonitorView.duplicate.spec.ts`<br>`frontend/src/views/admin/__tests__/ChannelMonitorView.grok.spec.ts` | +473/-104；测 +22/-11、+3/-3 | 管理端渠道监控主页面（监控项管理/复制/模板），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/monitor/MonitorActionsCell.vue`<br>`frontend/src/components/admin/monitor/MonitorActionsCell.spec.ts` | +23/-5；测 +15/-3 | 监控项行操作单元格（校准/复制入口），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/monitor/MonitorAvailabilityAdjustDialog.vue`<br>`frontend/src/components/admin/monitor/__tests__/MonitorAvailabilityAdjustDialog.spec.ts` | +117/-0；测 +146/-0 | 可用性人工校准对话框（修正误报），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/monitor/MonitorFiltersBar.vue`<br>`frontend/src/components/admin/monitor/__tests__/MonitorFiltersBar.spec.ts` | +8/-10；测 +64/-0 | 监控筛选条微调 + 补测试，管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/monitor/MonitorFormDialog.vue`<br>`frontend/src/components/admin/monitor/__tests__/MonitorFormDialog.spec.ts` | +129/-102；测 +497/-0 | 监控项创建/编辑对话框重构（多平台探测参数），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/admin/monitor/MonitorTemplateManagerDialog.vue`<br>`frontend/src/components/admin/monitor/__tests__/MonitorTemplateManagerDialog.spec.ts` | +21/-9；测 +182/-0 | 监控模板管理对话框增强，管理员用 | ☐ 待定 |
| P1 | `frontend/src/features/channel-monitor-v2/RelayPulseMatrix.vue`<br>`frontend/src/features/channel-monitor-v2/__tests__/RelayPulseMatrix.spec.ts` | +79/-219；测 +40/-88 | 渠道脉冲矩阵（时间片健康格子图）重构瘦身，状态页核心视觉 | ☐ 待定 |
| P1 | `frontend/src/features/channel-monitor-v2/MonitorSettingsPanel.vue` | +72/-108 | 监控展示设置面板重构，用户状态页用 | ☐ 待定 |
| P1 | `frontend/src/features/channel-monitor-v2/MonitorTrendChart.vue` | +48/-95 | 监控趋势图重构（延迟/成功率曲线），状态页用 | ☐ 待定 |
| P1 | `frontend/src/features/channel-monitor-v2/FilterMultiSelect.vue` | +15/-47 | 多选筛选器精简，状态页用 | ☐ 待定 |
| P1 | `frontend/src/features/channel-monitor-v2/MetricCell.vue`<br>`frontend/src/features/channel-monitor-v2/__tests__/MetricCell.spec.ts` | +5/-49；测 0/-16（测试删除） | 指标单元格瘦身，旧测试删除 | ☐ 待定 |
| P1 | `frontend/src/features/channel-monitor-v2/monitorFormat.ts`<br>`frontend/src/features/channel-monitor-v2/__tests__/monitorFormat.spec.ts` | +33/-46；测 0/-27（测试删除） | v2 内格式化逻辑收敛（部分逻辑移至 composable），旧测试删除 | ☐ 待定 |
| P2 | `frontend/src/features/channel-monitor-v2/__tests__/designSystem.structure.spec.ts` | +48/-22 | v2 设计体系结构约束测试（类名/布局规范） | ☐ 待定 |
| P1 | `frontend/src/components/user/monitor/MonitorCard.vue` | +2/-1 | 用户端监控卡片微调 | ☐ 待定 |
| P1 | `frontend/src/components/user/monitor/MonitorCardGrid.vue` | +39/-10 | 用户端监控卡片栅格增强（分组/排序） | ☐ 待定 |
| P1 | `frontend/src/components/user/monitor/ProviderIcon.vue` | +12/-4 | 监控供应商图标扩展（新平台） | ☐ 待定 |
| P1 | `frontend/src/views/user/ChannelStatusV2View.vue`<br>`frontend/src/views/user/__tests__/ChannelStatusView.spec.ts` | +273/-458；测 +255/-0 | 用户渠道状态页 v2 大改（信息密度/视觉重构），普通用户用 | ☐ 待定 |
| P2 | `frontend/src/views/user/ChannelStatusV1View.vue` | +8/-10 | 旧版状态页微调（保留兼容），普通用户用 | ☐ 待定 |
| P1 | `frontend/src/views/admin/ChannelsView.vue` | +49/-78 | 管理端可用渠道页调整（与监控/分组联动精简），管理员用 | ☐ 待定 |
| P1 | `frontend/src/components/channels/SupportedModelChip.vue`<br>`frontend/src/components/channels/__tests__/SupportedModelChip.spec.ts` | +3/-3；测 +53/-0 | 渠道支持模型标签微调 + 补测试 | ☐ 待定 |
| P1 | `frontend/src/api/admin/ops.ts`<br>`frontend/src/api/admin/__tests__/ops.spec.ts` | +138/-25；测 +67/-0 | Ops 运维 API 扩展（错误日志详情/请求追踪/切换率） | ☐ 待定 |
| P1 | `frontend/src/views/admin/ops/OpsDashboard.vue`<br>`frontend/src/views/admin/ops/__tests__/OpsDashboard.spec.ts` | +272/-80；测 +362/-0 | Ops 运维仪表盘主页面（错误/日志/趋势总览），管理员排障用 | ☐ 待定 |
| P1 | `frontend/src/views/admin/ops/components/OpsDashboardHeader.vue`<br>`frontend/src/views/admin/ops/components/__tests__/OpsDashboardHeader.spec.ts` | +24/-6；测 +149/-0 | Ops 仪表盘头部（时间范围/刷新控制），管理员用 | ☐ 待定 |
| P1 | `frontend/src/views/admin/ops/components/OpsErrorLogTable.vue`<br>`frontend/src/views/admin/ops/components/__tests__/OpsErrorLogTable.spec.ts` | +366/-221；测 +61/-2 | 错误日志表格大改（分级/筛选/详情入口），管理员用 | ☐ 待定 |
| P1 | `frontend/src/views/admin/ops/components/OpsErrorDetailModal.vue`<br>`frontend/src/views/admin/ops/components/__tests__/OpsErrorDetailModal.spec.ts` | +152/-36；测 +273/-0 | 单条错误详情弹窗（请求/响应上下文），管理员用 | ☐ 待定 |
| P1 | `frontend/src/views/admin/ops/components/OpsErrorDetailsModal.vue`<br>`frontend/src/views/admin/ops/components/__tests__/OpsErrorDetailsModal.spec.ts` | +66/-24；测 +120/-0 | 错误批量详情弹窗（聚合视图），管理员用 | ☐ 待定 |
| P1 | `frontend/src/views/admin/ops/components/OpsRequestDetailsModal.vue`<br>`frontend/src/views/admin/ops/components/__tests__/OpsRequestDetailsModal.spec.ts` | +53/-4；测 +168/-0 | 请求详情弹窗（trace/上游账号信息），管理员用 | ☐ 待定 |
| P1 | `frontend/src/views/admin/ops/components/OpsErrorDistributionChart.vue` | +43/-4 | 错误分布图增强，管理员用 | ☐ 待定 |
| P1 | `frontend/src/views/admin/ops/components/OpsErrorTrendChart.vue` | +23/-4 | 错误趋势图增强，管理员用 | ☐ 待定 |
| P1 | `frontend/src/views/admin/ops/components/__tests__/OpsErrorScopeCharts.spec.ts` | +141/-0 | 错误范围（平台/分组维度）图表测试 | ☐ 待定 |
| P1 | `frontend/src/views/admin/ops/components/OpsSwitchRateTrendChart.vue`<br>`frontend/src/views/admin/ops/components/__tests__/OpsSwitchRateTrendChart.spec.ts` | +22/-3；测 +66/-0 | 账号切换率趋势图（调度健康度指标），管理员用 | ☐ 待定 |
| P1 | `frontend/src/views/admin/ops/components/OpsOpenAITokenStatsCard.vue`<br>`frontend/src/views/admin/ops/components/__tests__/OpsOpenAITokenStatsCard.spec.ts` | +3/-1；测 +29/-6 | OpenAI Token 统计卡片微调，管理员用 | ☐ 待定 |
| P1 | `frontend/src/views/admin/ops/components/OpsSystemLogTable.vue`<br>`frontend/src/views/admin/ops/components/__tests__/OpsSystemLogTable.spec.ts` | +43/-43；测 +7/-7 | 系统日志表格重构，管理员用 | ☐ 待定 |
| P1 | `frontend/src/views/admin/ops/components/OpsSettingsDialog.vue` | +31/-1 | Ops 设置对话框增强（保留天数/采样），管理员用 | ☐ 待定 |

## 10. 认证增强（建议批次 9，整体重要度 P1）

登录/注册/第三方 OAuth 与个人资料安全，共 51 个文件：钉钉（含邮箱补全）/微信/LinuxDO/OIDC 回调、待定 OAuth 账号领取、TOTP 揭示/禁用、验证码、注册邮箱策略、会话管理，以及个人资料页改造。对应后端「前台 OAuth 登录 + Auth」模块，独立低风险，可较晚迁移。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `frontend/src/api/auth.ts` | +92/-85 | 认证 API 重构（多 OAuth 源/会话/TOTP 接口） | ☐ 待定 |
| P1 | `frontend/src/api/__tests__/auth-session.spec.ts` | +49/-0 | 会话续期/失效 API 行为测试 | ☐ 待定 |
| P1 | `frontend/src/api/__tests__/auth-oauth-adoption.spec.ts` | +3/-1 | OAuth 账号领取（adoption）API 测试小改 | ☐ 待定 |
| P1 | `frontend/src/api/__tests__/settings.authSourceDefaults.spec.ts` | +199/-132 | 认证源默认配置（各 OAuth 开关默认值）契约测试重构 | ☐ 待定 |
| P1 | `frontend/src/api/user.ts` | +7/-0 | 用户资料 API 小改（绑定/通知设置） | ☐ 待定 |
| P1 | `frontend/src/utils/authSession.ts` | +153/-0 | 前端会话管理工具（过期检测/静默续期/登出清理） | ☐ 待定 |
| P1 | `frontend/src/utils/registrationEmailPolicy.ts` | +1/-1 | 注册邮箱域策略工具微调 | ☐ 待定 |
| P1 | `frontend/src/components/CaptchaChallenge.vue` | +1/-1 | 验证码组件微调 | ☐ 待定 |
| P1 | `frontend/src/components/auth/PendingOAuthCreateAccountForm.vue`<br>`frontend/src/components/auth/__tests__/PendingOAuthCreateAccountForm.spec.ts` | +34/-111；测 +28/-144 | OAuth 首登补建账号表单精简（流程收敛），新用户用 | ☐ 待定 |
| P1 | `frontend/src/components/auth/TotpRevealDialog.vue` | +150/-0 | TOTP 密钥揭示对话框（需密码验证后展示），用户安全设置用 | ☐ 待定 |
| P1 | `frontend/src/composables/useTotpReveal.ts`<br>`frontend/src/composables/__tests__/useTotpReveal.spec.ts` | +144/-0；测 +94/-0 | TOTP 揭示流程 composable（验证/倒计时/遮罩） | ☐ 待定 |
| P1 | `frontend/src/components/auth/WechatOAuthSection.vue`<br>`frontend/src/components/auth/__tests__/WechatOAuthSection.spec.ts` | +2/-9；测 +7/-0 | 微信登录区块微调，登录页用 | ☐ 待定 |
| P2 | `frontend/src/components/layout/AuthLayout.vue`<br>`frontend/src/components/layout/__tests__/AuthLayout.spec.ts` | +5/-3；测 +21/-0 | 认证页布局微调 + 补测试 | ☐ 待定 |
| P1 | `frontend/src/views/auth/LoginView.vue` | +26/-11 | 登录页增强（多 OAuth 入口排布），访客用 | ☐ 待定 |
| P1 | `frontend/src/views/auth/RegisterView.vue`<br>`frontend/src/views/auth/__tests__/RegisterView.spec.ts` | +60/-104；测 +128/-97 | 注册页改造（邮箱策略/验证流程收敛），访客用 | ☐ 待定 |
| P1 | `frontend/src/views/auth/EmailVerifyView.vue`<br>`frontend/src/views/auth/__tests__/EmailVerifyView.spec.ts` | +143/-224；测 +45/-261 | 邮箱验证页重构（验证码输入/重发节流），新用户用 | ☐ 待定 |
| P1 | `frontend/src/views/auth/ForgotPasswordView.vue` | +20/-7 | 忘记密码页增强，访客用 | ☐ 待定 |
| P2 | `frontend/src/views/auth/ResetPasswordView.vue` | +3/-1 | 重置密码页微调，访客用 | ☐ 待定 |
| P1 | `frontend/src/views/auth/OAuthCallbackView.vue`<br>`frontend/src/views/auth/__tests__/OAuthCallbackView.spec.ts` | +33/-13；测 +69/-0 | 通用 OAuth 回调页（状态处理/错误提示），访客用 | ☐ 待定 |
| P1 | `frontend/src/views/auth/OidcCallbackView.vue`<br>`frontend/src/views/auth/__tests__/OidcCallbackView.spec.ts` | +14/-13；测 +111/-3 | OIDC 回调页增强，访客用 | ☐ 待定 |
| P1 | `frontend/src/views/auth/LinuxDoCallbackView.vue`<br>`frontend/src/views/auth/__tests__/LinuxDoCallbackView.spec.ts` | +14/-13；测 +68/-3 | LinuxDO 回调页增强，访客用 | ☐ 待定 |
| P1 | `frontend/src/views/auth/WechatCallbackView.vue`<br>`frontend/src/views/auth/__tests__/WechatCallbackView.spec.ts` | +19/-17；测 +18/-5 | 微信回调页增强，访客用 | ☐ 待定 |
| P1 | `frontend/src/views/auth/DingTalkCallbackView.vue`<br>`frontend/src/views/auth/__tests__/DingTalkCallbackView.spec.ts` | +24/-15；测 +142/-0 | 钉钉回调页（扫码登录落地），访客用 | ☐ 待定 |
| P1 | `frontend/src/views/auth/DingTalkEmailCompletionView.vue`<br>`frontend/src/views/auth/__tests__/DingTalkEmailCompletionView.spec.ts` | +14/-24；测 +148/-0 | 钉钉登录后邮箱补全页（钉钉无邮箱时收集），新用户用 | ☐ 待定 |
| P1 | `frontend/src/views/auth/dingtalkEmailCompletionPayload.ts`<br>`frontend/src/views/auth/__tests__/dingtalkEmailCompletionPayload.spec.ts` | +39/-0；测 +30/-0 | 钉钉邮箱补全 payload 构建/校验逻辑 | ☐ 待定 |
| P1 | `frontend/src/router/__tests__/dingtalk-route.spec.ts` | +72/-0 | 钉钉相关路由注册测试 | ☐ 待定 |
| P1 | `frontend/src/views/user/ProfileView.vue`<br>`frontend/src/views/user/__tests__/ProfileView.spec.ts` | +61/-47；测 +91/-0 | 个人资料页改版（卡片化布局/绑定管理），普通用户用 | ☐ 待定 |
| P1 | `frontend/src/components/user/profile/ProfileInfoCard.vue`<br>`frontend/src/components/user/profile/__tests__/ProfileInfoCard.spec.ts` | +192/-173；测 +225/-24 | 资料信息卡重构（昵称/密码/TOTP 集中管理），普通用户用 | ☐ 待定 |
| P2 | `frontend/src/components/user/profile/ProfileAvatarCard.vue`<br>`frontend/src/components/user/profile/__tests__/ProfileAvatarCard.spec.ts` | +14/-4；测 +74/-0 | 头像卡片微调 + 补测试，普通用户用 | ☐ 待定 |
| P1 | `frontend/src/components/user/profile/ProfileBalanceNotifyCard.vue`<br>`frontend/src/components/user/profile/__tests__/ProfileBalanceNotifyCard.spec.ts` | +22/-10；测 +179/-0 | 余额提醒设置卡片（低额通知阈值），普通用户用 | ☐ 待定 |
| P1 | `frontend/src/components/user/profile/ProfileIdentityBindingsSection.vue`<br>`frontend/src/components/user/profile/__tests__/ProfileIdentityBindingsSection.spec.ts` | +39/-2；测 +28/-0 | 第三方身份绑定区块（钉钉/微信/OIDC 绑定解绑），普通用户用 | ☐ 待定 |
| P2 | `frontend/src/components/user/profile/TotpDisableDialog.vue` | +1/-1 | TOTP 停用对话框微调，普通用户用 | ☐ 待定 |
| P2 | `frontend/src/components/user/profile/TotpSetupModal.vue` | +1/-1 | TOTP 设置弹窗微调，普通用户用 | ☐ 待定 |

## 11. 备份 / IP 安全 / 风控（建议批次 10，整体重要度 P1）

管理端安全运维三件套，共 7 个文件：数据备份页（下载步升认证——安全回退项）、IP 安全 API、风控中心页（规则/事件）。对应后端 Backup / 风控 / ip_security 模块。规模小、独立性强。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `frontend/src/api/admin/backup.ts` | +23/-8 | 备份 API 增强（步升认证 token/下载） | ☐ 待定 |
| P0 | `frontend/src/views/admin/BackupView.vue`<br>`frontend/src/views/admin/__tests__/BackupView.stepUp.spec.ts` | +296/-62；测 +180/-0 | 备份管理页（下载前强制步升认证，防备份文件泄露的安全回退），管理员用 | ☐ 待定 |
| P1 | `frontend/src/api/admin/ipSecurity.ts` | +53/-0 | IP 安全 API（黑白名单/异常 IP 事件） | ☐ 待定 |
| P1 | `frontend/src/api/admin/riskControl.ts` | +102/-0 | 风控 API（规则/事件/处置） | ☐ 待定 |
| P1 | `frontend/src/views/admin/RiskControlView.vue`<br>`frontend/src/views/admin/__tests__/RiskControlView.spec.ts` | +761/-50；测 +330/-13 | 风控中心页（IP 安全/风险事件/规则管理整合），管理员用 | ☐ 待定 |

## 12. i18n（每批伴随迁移，整体重要度 P0）

**重大风险项**：personal-dev 同时存在 monolith（locales/{en,zh}.ts 与 .json，合计约 3.5 万行）与上游的模块化目录（locales/en/、locales/zh/），由 i18n/index.ts 做三路运行时合并。**迁移策略：迁移前先把 monolith 中的新增键去重归位到模块化目录（按本明细各模块划分、随对应批次落地），然后删除 monolith 四个文件、恢复上游模块化 loader**——不要把 monolith 原样搬到新分支。锁定该策略后，下表中 monolith 文件与三路合并相关文件的取舍即为「内容保留、文件形态不保留」。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `frontend/src/i18n/index.ts` | +108/-22 | i18n 入口：当前实现三路合并（模块化+ts monolith+json）；迁移时改回上游模块化 loader，仅保留必要增强 | ☐ 待定 |
| P1 | `frontend/src/utils/i18n.ts` | +108/-0 | 翻译键映射工具（账号状态/平台名显示键），与 monolith 无关，可直接迁移 | ☐ 待定 |
| P1 | `frontend/src/i18n/__tests__/adminLocaleParity.spec.ts` | +178/-0 | 中英文管理端键位对齐测试（防漏译），去重归位后仍有价值 | ☐ 待定 |
| P3 | `frontend/src/i18n/__tests__/localeRuntimeMerge.spec.ts` | +333/-0 | 三路运行时合并行为测试——合并机制取消后即失效，建议放弃 | ☐ 待定 |
| P1 | `frontend/src/i18n/__tests__/localesNoKeyCollision.spec.ts` | +14/-4 | 键冲突检测测试，改造为纯模块化目录检查后保留 | ☐ 待定 |
| P1 | `frontend/src/i18n/__tests__/opsLocaleKeys.spec.ts` | +6/-16 | Ops 文案键存在性测试小改 | ☐ 待定 |
| P1 | `frontend/src/i18n/__tests__/tlsCollectorLocales.spec.ts` | +82/-0 | TLS 路由器（collector）文案键测试 | ☐ 待定 |
| P1 | `frontend/src/i18n/__tests__/usageServiceTierLocales.spec.ts` | +2/-2 | 用量服务档位文案键测试微调 | ☐ 待定 |
| P3 | `frontend/src/features/prompt-audit/__tests__/integrationSurface.spec.ts` | +2/-2 | 仅因 monolith 引入的 locale import 路径改动；恢复上游 loader 后无需迁移 | ☐ 待定 |

<details>
<summary>monolith 与模块化 locale 文件明细（12 个修改 + 4 个 monolith）</summary>

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `frontend/src/i18n/locales/en.ts` | +9105/-0 | 英文 monolith（新功能全部文案在此）：内容 P0，先去重归位到模块化目录后删除本文件 | ☐ 待定 |
| P0 | `frontend/src/i18n/locales/zh.ts` | +9284/-0 | 中文 monolith：同上，去重归位后删除 | ☐ 待定 |
| P0 | `frontend/src/i18n/locales/en.json` | +8354/-0 | 英文 JSON monolith（与 .ts 大量重复）：去重归位后删除 | ☐ 待定 |
| P0 | `frontend/src/i18n/locales/zh.json` | +8543/-0 | 中文 JSON monolith：同上，去重归位后删除 | ☐ 待定 |
| P1 | `frontend/src/i18n/locales/en/admin/accounts.ts` | +170/-126 | 模块化英文·管理端账号文案（TLS/Kiro/容量等），随批次 1 迁移 | ☐ 待定 |
| P1 | `frontend/src/i18n/locales/zh/admin/accounts.ts` | +171/-125 | 模块化中文·管理端账号文案，随批次 1 迁移 | ☐ 待定 |
| P1 | `frontend/src/i18n/locales/en/admin/channels.ts` | +4/-2 | 模块化英文·渠道文案微调，随批次 8 迁移 | ☐ 待定 |
| P1 | `frontend/src/i18n/locales/zh/admin/channels.ts` | +4/-2 | 模块化中文·渠道文案微调，随批次 8 迁移 | ☐ 待定 |
| P1 | `frontend/src/i18n/locales/en/admin/overview.ts` | +6/-34 | 模块化英文·管理端总览文案调整 | ☐ 待定 |
| P1 | `frontend/src/i18n/locales/zh/admin/overview.ts` | +6/-34 | 模块化中文·管理端总览文案调整 | ☐ 待定 |
| P1 | `frontend/src/i18n/locales/en/admin/settings.ts` | +97/-93 | 模块化英文·设置页文案（随 SettingsView 分区迁移） | ☐ 待定 |
| P1 | `frontend/src/i18n/locales/zh/admin/settings.ts` | +89/-89 | 模块化中文·设置页文案（随 SettingsView 分区迁移） | ☐ 待定 |
| P1 | `frontend/src/i18n/locales/en/common.ts` | +10/-3 | 模块化英文·通用文案，随批次 1 迁移 | ☐ 待定 |
| P1 | `frontend/src/i18n/locales/zh/common.ts` | +10/-3 | 模块化中文·通用文案，随批次 1 迁移 | ☐ 待定 |
| P1 | `frontend/src/i18n/locales/en/dashboard.ts` | +50/-20 | 模块化英文·仪表盘文案，随批次 1 迁移 | ☐ 待定 |
| P1 | `frontend/src/i18n/locales/zh/dashboard.ts` | +50/-19 | 模块化中文·仪表盘文案，随批次 1 迁移 | ☐ 待定 |

</details>

## 13. 公共缠绕文件（每批都要碰，整体重要度 P0）

**按批次增量插入，禁止整体覆盖。** 这些文件被所有功能模块共同缠绕：router/types/api-client/stores/侧边栏在每个批次都要新增自己的片段；SettingsView 近乎重写，须单独立项、逐配置分区迁移、禁止整体粘贴。建议做法：每迁移一个模块，只把该模块相关的路由项、类型定义、API 导出、导航项、设置分区 diff 进来，并跑一次 typecheck + 相关测试。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `frontend/src/router/index.ts` | +670/-71 | 全部新页面的路由注册（Studio/Skills/工单/发票/监控/认证等）——按批次增量插入对应路由块 | ☐ 待定 |
| P0 | `frontend/src/router/meta.d.ts` | +43/-1 | 路由 meta 类型扩展（权限/feature flag/badge 字段） | ☐ 待定 |
| P1 | `frontend/src/router/__tests__/feature-access.spec.ts` | +110/-10 | 路由级功能开关（feature flag 门禁）测试，随对应路由增量补 | ☐ 待定 |
| P1 | `frontend/src/router/__tests__/guards.spec.ts` | +169/-66 | 路由守卫（登录/角色/合规门）测试，随守卫改动增量合并 | ☐ 待定 |
| P0 | `frontend/src/types/index.ts` | +626/-134 | 全局类型缠绕点——迁移时按功能拆分为独立类型文件，禁止整体覆盖 | ☐ 待定 |
| P0 | `frontend/src/api/client.ts`<br>`frontend/src/api/__tests__/client.spec.ts` | +313/-45；测 +214/-65 | Axios 客户端增强（会话续期、423 合规拦截、错误规范化）——各批次共同依赖，最早迁 | ☐ 待定 |
| P1 | `frontend/src/api/index.ts` | +5/-1 | API barrel 导出，按批次增量加行 | ☐ 待定 |
| P1 | `frontend/src/api/url.ts` | +5/-4 | API base URL 工具小改 | ☐ 待定 |
| P1 | `frontend/src/api/admin/index.ts` | +25/-0 | 管理端 API barrel 导出，按批次增量加行 | ☐ 待定 |
| P0 | `frontend/src/api/admin/settings.ts` | +587/-120 | 管理端设置 API（所有模块的设置字段都缠绕在此）——随 SettingsView 分区逐段迁移 | ☐ 待定 |
| P1 | `frontend/src/App.vue` | +9/-6 | 根组件小改（合规弹窗/公告挂载点） | ☐ 待定 |
| P1 | `frontend/src/__tests__/public-settings-types.spec.ts` | +16/-0 | 公共设置类型与注入配置一致性测试 | ☐ 待定 |
| P0 | `frontend/src/components/layout/AppSidebar.vue`<br>`frontend/src/components/layout/__tests__/AppSidebar.spec.ts` | +394/-76；测 +69/-0 | 侧边栏：所有新模块的导航入口 + 角标——按批次增量插入菜单项 | ☐ 待定 |
| P1 | `frontend/src/components/layout/AppHeader.vue`<br>`frontend/src/components/layout/__tests__/AppHeader.spec.ts` | +38/-75；测 +33/-0 | 顶栏调整（公告铃铛/客服按钮/精简），全体用户用 | ☐ 待定 |
| P1 | `frontend/src/components/layout/SidebarNavBadge.vue`<br>`frontend/src/components/layout/__tests__/SidebarNavBadge.spec.ts` | +57/-0；测 +27/-0 | 侧边栏角标组件（未读工单/待审数字），批次 1 随导航落地 | ☐ 待定 |
| P1 | `frontend/src/composables/useNavigationBadges.ts`<br>`frontend/src/composables/__tests__/useNavigationBadges.spec.ts` | +157/-0；测 +139/-0 | 导航角标数据源 composable（聚合各模块待办数），批次 1 落地、各批次增量注册 | ☐ 待定 |
| P1 | `frontend/src/navigation/simpleMode.ts` | +33/-0 | simple 模式下的导航裁剪清单，批次 1 落地、各批次增量维护 | ☐ 待定 |
| P0 | `frontend/src/stores/app.ts`<br>`frontend/src/stores/__tests__/app.spec.ts` | +44/-14；测 +19/-16 | 应用全局 store（公共设置缓存/注入配置初始化）——feature flag 体系依赖 | ☐ 待定 |
| P0 | `frontend/src/stores/auth.ts`<br>`frontend/src/stores/__tests__/auth.spec.ts` | +100/-95；测 +208/-22 | 认证 store 重构（会话生命周期/角色判断）——全站依赖，最早迁 | ☐ 待定 |
| P1 | `frontend/src/stores/index.ts` | +2/-0 | store barrel 导出增量 | ☐ 待定 |
| P1 | `frontend/src/stores/adminSettings.ts` | +4/-0 | 管理端设置 store 小改 | ☐ 待定 |
| P3 | `frontend/src/stores/README.md` | +4/-3 | store 目录说明文档小改，可放弃 | ☐ 待定 |
| P1 | `frontend/src/utils/featureFlags.ts`<br>`frontend/src/utils/__tests__/channelMonitorFeatureFlags.spec.ts` | +59/-9；测 +108/-0 | 功能开关注册表（opt-in/opt-out 语义，修复刷新后菜单消失 bug）——侧边栏/路由共同依赖 | ☐ 待定 |
| P0 | `frontend/src/views/admin/SettingsView.vue`<br>`frontend/src/views/admin/__tests__/SettingsView.spec.ts`<br>`frontend/src/views/admin/__tests__/SettingsView.structure.spec.ts` | +7335/-4936；测 +2055/-769、+19/-0 | 管理端设置页（近乎重写，所有模块的配置分区缠绕于此）——**单独立项，逐配置分区迁移，禁止整体粘贴** | ☐ 待定 |

## 14. 工程配置与依赖（建议批次 1，整体重要度 P1）

前端工程层的少量改动，共 5 个文件。package.json 与上游完全一致；核心是 pnpm-workspace.yaml 的安全 overrides，补上后 `pnpm install` 重新生成锁文件即可，锁文件本身不做搬迁。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `frontend/pnpm-workspace.yaml` | +11/-0 | 新增：安全 overrides（js-cookie 3.0.7、form-data>=4.0.6、postcss>=8.5.18）与 allowBuilds 白名单 | ☐ 待定 |
| P3 | `frontend/pnpm-lock.yaml` | +661/-710 | 锁文件——不搬迁，补齐 workspace overrides 后由 `pnpm install` 重新生成 | ☐ 待定 |
| P2 | `frontend/audit.json` | +3/-115 | pnpm audit 豁免清单瘦身（大量过期豁免删除），与安全扫描工具链配套 | ☐ 待定 |
| P2 | `frontend/vitest.config.ts` | +3/-0 | vitest 配置微调（超时/环境） | ☐ 待定 |
| P1 | `frontend/src/__tests__/setup.ts` | +26/-37 | 全局测试 setup 重构（mock/i18n 装配），几乎所有前端测试依赖 | ☐ 待定 |

## 完整性核对

核对方法：用临时脚本（/tmp/check_frontend_md.py，不留在仓库）从本文档提取所有 `frontend/...` 路径，与 /tmp/migration-analysis/numstat-C-frontend.txt 的 609 条路径做集合 diff，并检查文档内路径无重复。

核对结果（2026-08-13 执行）：

- numstat 文件数：609；文档提取路径数（去重）：609，总出现次数 609（每个文件恰好出现一次）
- 遗漏（numstat 有、文档无）：0
- 多余（文档有、numstat 无）：0
- 重复出现：0
- 重要度分布（按文件计）：P0 = 59，P1 = 423，P2 = 123，P3 = 4，合计 609

结论：**609/609，零遗漏、零多余、零重复**。
