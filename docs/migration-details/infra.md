# 迁移明细 D：工具与基础设施（tools / deploy / CI / docs / 根级）

> 基准：git diff upstream/main...personal-dev --numstat，共 154 个文件。
> 重要度：P0 核心（缺了核心工作流不可用或安全回退）/ P1 重要（明显功能或稳定性增强）/ P2 可选（便利、锦上添花）/ P3 建议放弃（一次性产物、被上游取代、过时）。
> 取舍栏默认 ☐ 待定，由用户填写。

---

## 1. CI 工作流与安全扫描工具链（建议批次：CI / 安全扫描（早期），整体重要度 P0）

CI 是所有后续批次的质量闸门，建议最早迁移。backend-ci.yml 带 pnpm 构建校验、go mod tidy 漂移检测、embed 构建校验和 docker-image job；security-scan.yml 与两个豁免清单、三个校验脚本构成完整的安全扫描闭环（govulncheck 固定版本 + 豁免校验 + secret scan + pnpm audit），少任何一件扫描 job 都会失败，需同批迁移。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `.github/workflows/backend-ci.yml` | +69/-1 | 后端 CI 主工作流：pnpm 构建校验、go mod tidy 漂移检测、embed 构建校验、docker-image job。所有批次的合并闸门 | ☐ 待定 |
| P0 | `.github/workflows/security-scan.yml` | +15/-2 | 安全扫描工作流：govulncheck 固定版本 + 豁免校验 + secret scan。与本节两个豁免清单、三个 tools 脚本同批缺一不可 | ☐ 待定 |
| P0 | `.github/audit-exceptions.yml` | +10/-17 | pnpm audit 豁免清单，由 `tools/check_pnpm_audit_exceptions.py` 消费；security-scan.yml 引用，同批缺一不可 | ☐ 待定 |
| P0 | `.github/govuln-exceptions.yml` | +8/-0 | govulncheck 豁免清单（新增），由 `tools/check_govuln_exceptions.py` 校验；security-scan.yml 引用，同批缺一不可 | ☐ 待定 |
| P0 | `tools/check_govuln_exceptions.py` | +115/-0 | govulncheck 豁免校验脚本（新增），解析扫描输出并核对 `.github/govuln-exceptions.yml`；测试在 `tools/test_security_scan_tools.py`，同批缺一不可 | ☐ 待定 |
| P0 | `tools/secret_scan.py` | +237/-0 | 密钥泄漏扫描脚本（新增），security-scan.yml 与 Makefile secret-scan target 均调用；测试在 `tools/test_security_scan_tools.py`，同批缺一不可 | ☐ 待定 |
| P0 | `tools/test_security_scan_tools.py` | +227/-0 | 上述安全扫描脚本的单元测试（新增），覆盖 `secret_scan.py` 与 `check_govuln_exceptions.py`，同批缺一不可 | ☐ 待定 |
| P1 | `tools/check_pnpm_audit_exceptions.py` | +26/-1 | pnpm audit 豁免校验脚本（小改），适配 `.github/audit-exceptions.yml` 新格式，随安全扫描批次一起走 | ☐ 待定 |
| P1 | `.github/workflows/release.yml` | +31/-26 | 发布工作流：RELEASE_REF/TAG 统一、shell 注入修复、`deploy/build_metadata.sh` 集成。依赖第 3 节构建溯源文件 | ☐ 待定 |

## 2. 生产部署主脚本（建议批次：部署（较早），整体重要度 P0）

deploy.sh 是个人生产环境的部署入口，实现部署锁、备份回滚、sha256 校验等安全机制，628 行纯新增；配套 649 行 pytest 测试保证脚本行为不回退。两文件互为依赖，同批缺一不可。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `deploy.sh` | +628/-0 | 生产部署主脚本（新增）：部署锁 / 备份回滚 / sha256 校验。与 `tools/test_deploy_script.py` 同批缺一不可 | ☐ 待定 |
| P0 | `tools/test_deploy_script.py` | +649/-0 | `deploy.sh` 的 pytest 测试套件（新增），覆盖锁、回滚、校验逻辑。与 `deploy.sh` 同批缺一不可 | ☐ 待定 |

## 3. 构建溯源与镜像构建（建议批次：构建溯源，整体重要度 P1）

围绕 `deploy/build_metadata.sh` 的构建元数据（版本 / commit / 构建时间）注入链：Dockerfile 系列与 goreleaser 配置把 ldflags 打进二进制。⚠️ 需要 backend `main` 包的变量支持（跨模块依赖），应在对应后端改动落地后同批迁移。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `deploy/build_metadata.sh` | +98/-0 | 构建元数据生成脚本（新增），产出版本 / commit / 时间等 ldflags；被 release.yml、Dockerfile*、goreleaser 消费，本节核心 | ☐ 待定 |
| P1 | `Dockerfile` | +6/-1 | 根级镜像构建：接入 build_metadata ldflags 注入，与本节其余文件同批 | ☐ 待定 |
| P1 | `Dockerfile.goreleaser` | +3/-1 | goreleaser 专用镜像 Dockerfile：同步 ldflags / 元数据注入改动 | ☐ 待定 |
| P1 | `deploy/Dockerfile` | +33/-13 | deploy 目录镜像 Dockerfile：ldflags 注入 + 运行时依赖调整（如 ffmpeg，见 `deploy/DOCKER.md`） | ☐ 待定 |
| P1 | `.goreleaser.yaml` | +3/-0 | goreleaser 主配置：补充构建溯源 ldflags | ☐ 待定 |
| P1 | `.goreleaser.simple.yaml` | +3/-0 | goreleaser simple 模式配置：同上，与 `.goreleaser.yaml` 同批 | ☐ 待定 |
| P2 | `deploy/build_image.sh` | +7/-0 | 本地快速构建镜像的便利脚本（新增），封装构建参数，非必需但顺手迁移成本低 | ☐ 待定 |

## 4. 部署编排与安全默认值（建议批次：部署安全默认值 / 部署模板，整体重要度 P1）

docker-compose 系列与安装脚本的安全加固：绑定地址收紧到 127.0.0.1、checksum 强制、REDIS_PASSWORD 必填。⚠️ 其中 MEDIA_* 环境变量段依赖 Media 模块，需在 Media 迁移后再补。config.example.yaml 是全量配置参考，跟踪 backend 每个新配置项，建议随各后端批次持续增量更新。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P0 | `deploy/config.example.yaml` | +144/-68 | 全量注释配置参考，包含个人分支所有新配置段。缺失会导致新功能无法正确配置；建议随各后端批次增量同步 | ☐ 待定 |
| P1 | `deploy/install.sh` | +35/-14 | 一键安装脚本：绑定地址收紧 127.0.0.1、checksum 强制、REDIS_PASSWORD 必填等安全默认值 | ☐ 待定 |
| P1 | `deploy/docker-compose.yml` | +52/-26 | 主 compose 编排：端口绑定收紧、密码必填、健康检查等加固（MEDIA_* 段待 Media 批次） | ☐ 待定 |
| P1 | `deploy/docker-compose.local.yml` | +47/-17 | 本地开发 compose：同步安全默认值与数据目录约定 | ☐ 待定 |
| P1 | `deploy/docker-compose.standalone.yml` | +29/-3 | 单机模式 compose：同步安全默认值 | ☐ 待定 |
| P1 | `deploy/.env.example` | +68/-35 | 环境变量示例：新配置项与安全默认值说明，与 compose 系列同批 | ☐ 待定 |
| P1 | `deploy/Caddyfile` | +18/-0 | Caddy 反代模板（新增）：header 大小 / 超时等加固；末尾 media 段按需裁剪 | ☐ 待定 |
| P1 | `deploy/sub2api.service` | +5/-0 | systemd 单元加固（依赖声明、加固选项），与 `deploy/install.sh` 配套 | ☐ 待定 |
| P2 | `deploy/docker-compose.dev.yml` | +36/-10 | 开发环境 compose 变体，便利性质，随本节顺带迁移 | ☐ 待定 |
| P2 | `deploy/apple-container.sh` | +1/-1 | Apple Container 运行脚本的单行修正，成本极低 | ☐ 待定 |
| P2 | `deploy/.gitignore` | +1/-0 | deploy 目录忽略规则补一行（运行时数据目录），无风险 | ☐ 待定 |
| P2 | `deploy/README.md` | +27/-47 | deploy 目录说明文档更新，与本节脚本行为保持一致即可 | ☐ 待定 |
| P2 | `deploy/DOCKER.md` | +2/-0 | Docker 镜像说明补两行（ffmpeg 运行时依赖注记），与 `deploy/Dockerfile` 对应 | ☐ 待定 |

## 5. 配套运行时组件（datamanagementd / Codex 指令模板，整体重要度 P1）

两组跨模块配套文件：datamanagementd 安装脚本 / 文档依赖 backend 的数据管理功能（Unix Socket 探测），必须跟随该后端批次；两份 codex-instructions 模板内容相同（`data/` 为运行时默认、`deploy/` 随镜像分发），依赖 backend Codex 指令注入功能，同批缺一不可。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `deploy/install-datamanagementd.sh` | +9/-33 | datamanagementd 宿主机安装脚本（简化改动），依赖 backend 数据管理功能，随该功能批次走；与 `deploy/DATAMANAGEMENTD_CN.md` 同批 | ☐ 待定 |
| P1 | `deploy/DATAMANAGEMENTD_CN.md` | +6/-8 | datamanagementd 部署说明（socket 路径、健康检查约束），与 `deploy/install-datamanagementd.sh` 同批缺一不可 | ☐ 待定 |
| P1 | `data/codex-instructions.md.tmpl` | +8/-0 | Codex 系统指令 Go 模板（运行时默认，新增），依赖 backend 指令注入逻辑；与 `deploy/codex-instructions.md.tmpl` 内容一致，同批缺一不可 | ☐ 待定 |
| P1 | `deploy/codex-instructions.md.tmpl` | +4/-1 | 同上模板的 deploy 分发副本，与 `data/codex-instructions.md.tmpl` 同批缺一不可 | ☐ 待定 |

## 6. Skill Runner 沙箱示例（建议批次：跟随 backend skillrunner，整体重要度 P2）

`backend/internal/integration/skillrunner` 的脚本沙箱契约示例：非 root 固定 UID、network none、只读根文件系统等。纯示例性质，但对理解和部署 skill 沙箱有参考价值，建议跟随 skillrunner 后端批次迁移。（runtime 下两个 request.json 占位样例见 P3 节。）

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P2 | `deploy/skill-runner/README.md` | +72/-0 | 沙箱示例说明：阐述 phase-1 脚本沙箱契约（非 root、无网络），随 skillrunner 批次 | ☐ 待定 |
| P2 | `deploy/skill-runner/docker-compose.example.yml` | +50/-0 | 沙箱 compose 示例，演示安全约束配置，与 README 同批 | ☐ 待定 |
| P2 | `deploy/skill-runner/prepare-scratch.sh` | +40/-0 | 准备 python/node 运行时 scratch 目录的脚本，与示例配套 | ☐ 待定 |
| P2 | `deploy/skill-runner/runtime/.gitignore` | +2/-0 | 沙箱运行时目录忽略规则，随本节一并迁移 | ☐ 待定 |

## 7. 根级构建与开发文档（建议批次：低风险早迁，整体重要度 P1）

低风险、无依赖的根级文件，建议在最早批次一次迁完：Makefile 的 test-frontend 全量化与 secret-scan target（注意与前端批次和第 1 节 `tools/secret_scan.py` 协调）、.gitignore、SECURITY.md、DEV_GUIDE.md 的 fork 工作流更新。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `Makefile` | +25/-12 | test-frontend 全量化、新增 secret-scan target（依赖第 1 节 `tools/secret_scan.py`）；与前端批次协调 | ☐ 待定 |
| P1 | `.gitignore` | +23/-1 | 根级忽略规则：本地配置、运行时产物、审查快照目录等，低风险早迁 | ☐ 待定 |
| P1 | `SECURITY.md` | +89/-0 | 安全政策文档（新增），通用内容无代码依赖，低风险早迁 | ☐ 待定 |
| P1 | `DEV_GUIDE.md` | +15/-12 | 开发指南更新：fork 仓库指向、Go 版本、CI 说明、上游合并流程改为专用指南。注意合并流程段落需按新迁移策略改写 | ☐ 待定 |

## 8. 功能特性文档（建议批次：跟随对应功能模块，整体重要度 P1）

各功能模块的设计 / 运维 / 接入文档，本身无代码依赖，但脱离对应功能就没有意义，应跟随各自后端 / 前端批次迁移；对应功能若最终放弃，则文档一并放弃。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P1 | `docs/AI_STUDIO_USAGE_API_CN.md` | +277/-0 | AI 创作中心使用 / 接口文档（runtime、chat、artworks、gallery 链路），跟随 AI 创作中心批次 | ☐ 待定 |
| P1 | `docs/OPENAI_IMAGE_COMPAT_PLAYBOOK_CN.md` | +538/-0 | OpenAI 图片兼容与 AI 创作中心接入手册（codex-image、计费口径、操作流程），跟随 OpenAI 图片批次 | ☐ 待定 |
| P1 | `docs/MINIO_SOURCE_QAZWC_RESOURCE_SPEC_CN.md` | +182/-0 | MinIO 与统一资源域名规范（头像 / 公告 / 工单 / AI / 论坛图片），跟随对象存储 / Media 批次 | ☐ 待定 |
| P1 | `docs/PAYMENT.md` | +13/-1 | 支付文档补订阅 USD→CNY 汇率配置说明（fail-closed 语义），跟随订阅支付批次；与 `docs/PAYMENT_CN.md` 同批 | ☐ 待定 |
| P1 | `docs/PAYMENT_CN.md` | +13/-1 | 上文的中文版，与 `docs/PAYMENT.md` 同批缺一不可 | ☐ 待定 |
| P1 | `docs/channel-monitor-v2-design.md` | +314/-0 | 渠道监控 V2 完整设计稿（事实表、去重、覆盖语义），跟随渠道监控 V2 批次；根级摘要见 `CHANNEL_MONITOR_V2_DESIGN.md` | ☐ 待定 |
| P1 | `docs/channel-monitor-v2-safe-defaults.md` | +3/-3 | 渠道监控 V2 安全默认值与温和回填决策记录，与设计稿同批 | ☐ 待定 |
| P1 | `TLS_FINGERPRINT_ARCHITECTURE.md` | +59/-0 | TLS 指纹架构与运维文档（服务边界、采集器联动），跟随 TLS 指纹批次 | ☐ 待定 |
| P1 | `skills/sub2api-admin/references/admin-cli.md` | +29/-0 | sub2api-admin skill 的管理 API 参考（鉴权、curl 示例），跟随 skills / admin 功能批次 | ☐ 待定 |
| P2 | `CHANNEL_MONITOR_V2_DESIGN.md` | +7/-0 | 根级摘要指针，指向 `docs/channel-monitor-v2-design.md`（同批引用）；可迁可并入 docs | ☐ 待定 |
| P2 | `docs/OPENAI_CLAUDE_COMPAT_ROLLOUT.md` | +121/-0 | OpenAI/Claude 兼容工作的 rollout / 验证 / 合并顺序记录，绑定历史分支，参考价值随迁移完成递减 | ☐ 待定 |
| P2 | `docs/OPENAI_OAUTH_CAPACITY_PLANNING_CN.md` | +109/-0 | OpenAI OAuth 分组容量规划设计（看板口径），跟随容量看板批次；若该功能不迁则放弃 | ☐ 待定 |
| P2 | `openspec/changes/add-openai-compatible-prompt-audit/source-freeze/aicodex-prompt-audit-tracked.patch` | +1/-1 | openspec 变更的 source-freeze 证据补丁单行修正，跟随 prompt-audit 功能批次；若 openspec 变更目录不迁则放弃 | ☐ 待定 |

## 9. 待定项（附录 D 标注需用户决策，共 15 个文件）

以下文件附录 D 未给结论，逐文件给出建议供拍板。总体建议：superpowers 设计稿跟随各自功能批次（功能迁则迁，功能弃则弃）；两篇语义合并方法论文档为旧策略而写，建议放弃或改写；CHANGELOG 建议从空白开始；README 系列不整体迁移、只手动补实质修改。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| 待定(建议 P2) | `docs/superpowers/specs/2026-04-22-openai-claude-golden-fixtures-design.md` | +139/-0 | OpenAI/Claude 黄金夹具设计稿。建议：跟随兼容层测试批次迁移，功能弃则弃 | ☐ 待定 |
| 待定(建议 P2) | `docs/superpowers/specs/2026-06-01-openai-account-image-generation-toggle-design.md` | +296/-0 | OpenAI 账号图片生成开关设计稿。建议：跟随 OpenAI 图片批次迁移 | ☐ 待定 |
| 待定(建议 P2) | `docs/superpowers/specs/2026-06-03-openai-oauth-scheduling-threshold-temp-unsched-reason-design.md` | +378/-0 | OAuth 调度阈值与临时不可调度原因设计稿。建议：跟随调度器批次迁移 | ☐ 待定 |
| 待定(建议 P2) | `docs/superpowers/specs/2026-06-03-openai-response-rewrite-and-model-support-errors-design.md` | +427/-0 | OpenAI 响应重写与模型支持错误设计稿。建议：跟随网关 OpenAI 批次迁移 | ☐ 待定 |
| 待定(建议 P2) | `docs/superpowers/specs/2026-06-08-kiro-server-tools-bridge-design.md` | +454/-0 | Kiro server tools 桥接设计稿。建议：跟随 Kiro 批次迁移 | ☐ 待定 |
| 待定(建议 P2) | `docs/superpowers/specs/2026-06-22-go-tls-capture-unification-design.md` | +529/-0 | Go TLS 采集统一化设计稿。建议：跟随 TLS 指纹批次迁移（与 `TLS_FINGERPRINT_ARCHITECTURE.md` 呼应） | ☐ 待定 |
| 待定(建议 P2) | `docs/superpowers/specs/2026-06-26-openai-oauth-continuation-unification-design.md` | +495/-0 | OAuth continuation 统一化设计稿。建议：跟随 OAuth 批次迁移；与同名 plan 同批 | ☐ 待定 |
| 待定(建议 P2) | `docs/superpowers/plans/2026-06-26-openai-oauth-continuation-unification.md` | +504/-0 | 上一条设计稿对应的实施计划。建议：与设计稿同批迁移或一并放弃 | ☐ 待定 |
| 待定(建议 P3) | `docs/superpowers/specs/2026-07-10-upstream-semantic-merge-remediation-design.md` | +563/-0 | 上游语义合并修复设计稿，服务于旧的整体合并策略。建议：放弃（新策略已改为模块化迁移） | ☐ 待定 |
| 待定(建议 P3) | `docs/superpowers/plans/2026-07-10-upstream-semantic-merge-preflight.md` | +817/-0 | 上游语义合并预检计划，同上为旧策略而写。建议：放弃 | ☐ 待定 |
| 待定(建议 P2) | `docs/UPSTREAM_MERGE_GUIDE.md` | +301/-0 | 上游语义合并 runbook，为 personal-dev 长期分支策略写的。建议：不原样迁移，改写为模块化迁移版后再入库（`DEV_GUIDE.md` 中的引用需同步） | ☐ 待定 |
| 待定(建议 P3) | `CHANGELOG.md` | +28/-0 | 个人分支版本变更记录（0.1.x 体系），与新分支版本号不连续。建议：从空白开始，旧记录仅归档留存 | ☐ 待定 |
| 待定(建议 P2) | `README.md` | +101/-124 | 九成是上游赞助商 churn。建议：不整体迁移，手动补实质修改（Go 徽章、SECURITY 链接等）；与 CN/JA 版同步处理 | ☐ 待定 |
| 待定(建议 P2) | `README_CN.md` | +129/-58 | 同上，中文版。建议：与 `README.md` 同步只补实质修改 | ☐ 待定 |
| 待定(建议 P2) | `README_JA.md` | +43/-8 | 同上，日文版。建议：与 `README.md` 同步只补实质修改 | ☐ 待定 |

## 10. P3 建议放弃（一次性产物 / 过时快照，共 83 个文件）

### 10.1 审查快照 `.review/upstream-cross-2026-07-16/`（14 个）

2026-07-16 一轮交叉审查的模块报告，绑定当时的 commit SHA，脱离旧分支历史即失效。整体放弃。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P3 | `.review/upstream-cross-2026-07-16/BRIEF.md` | +70/-0 | 该轮审查的任务简报，绑定历史快照 | ☐ 待定 |
| P3 | `.review/upstream-cross-2026-07-16/CROSS-SYNTHESIS.md` | +197/-0 | 交叉审查综合结论，问题应已修复或过时 | ☐ 待定 |
| P3 | `.review/upstream-cross-2026-07-16/M01-gateway-openai.md` | +329/-0 | 网关 OpenAI 模块审查报告，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-cross-2026-07-16/M02-platforms.md` | +238/-0 | 平台模块审查报告，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-cross-2026-07-16/M03-apicompat.md` | +243/-0 | API 兼容模块审查报告，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-cross-2026-07-16/M04-billing-payment.md` | +191/-0 | 计费支付模块审查报告，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-cross-2026-07-16/M05-auth-security.md` | +229/-0 | 认证安全模块审查报告，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-cross-2026-07-16/M06-scheduler-accounts.md` | +236/-0 | 调度器账号模块审查报告，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-cross-2026-07-16/M07-ai-skills.md` | +210/-0 | AI skills 模块审查报告，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-cross-2026-07-16/M08-schema-migrations.md` | +274/-0 | schema 迁移模块审查报告，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-cross-2026-07-16/M09-frontend.md` | +219/-0 | 前端模块审查报告，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-cross-2026-07-16/M10-deploy-ci.md` | +258/-0 | 部署 CI 模块审查报告，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-cross-2026-07-16/META.txt` | +3/-0 | 该轮审查的元数据（SHA 等），随快照一并放弃 | ☐ 待定 |
| P3 | `.review/upstream-cross-2026-07-16/TASKS.md` | +54/-0 | 该轮审查任务分配清单，一次性产物 | ☐ 待定 |

### 10.2 审查快照 `.review/upstream-full-2026-07-19/`（51 个）

2026-07-19 全量审查快照：BASE_SHA 锚定、18 个文件分桶清单、30 份审查结果。全部绑定历史 SHA，整体放弃。**注意：CONSOLIDATED.md 记录的 bug 清单在迁移对应模块时值得人工核对一遍再丢**（附录 D 原注）。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P3 | `.review/upstream-full-2026-07-19/BASE_SHA` | +1/-0 | 审查基准 SHA 锚点，仅对旧分支有意义 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/CONSOLIDATED.md` | +82/-0 | 该轮审查合并结论。放弃前建议人工核对其 bug 清单是否已在对应模块修复 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/VERIFIED-ISSUES.md` | +99/-0 | 已验证问题清单，同上建议核对后放弃 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/buckets/backend_misc.txt` | +51/-0 | 审查分桶文件清单（后端杂项），一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/buckets/frontend_core.txt` | +107/-0 | 审查分桶文件清单（前端核心），一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/buckets/frontend_test.txt` | +206/-0 | 审查分桶文件清单（前端测试），一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/buckets/frontend_ui.txt` | +229/-0 | 审查分桶文件清单（前端 UI），一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/buckets/handler.txt` | +247/-0 | 审查分桶文件清单（handler 层），一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/buckets/infra_docs.txt` | +96/-0 | 审查分桶文件清单（基础设施与文档），一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/buckets/pkg.txt` | +134/-0 | 审查分桶文件清单（pkg 层），一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/buckets/repository.txt` | +176/-0 | 审查分桶文件清单（repository 层），一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/buckets/schema_migrations.txt` | +155/-0 | 审查分桶文件清单（schema 迁移），一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/buckets/server.txt` | +49/-0 | 审查分桶文件清单（server），一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/buckets/svc_account.txt` | +72/-0 | 审查分桶文件清单（账号服务），一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/buckets/svc_billing.txt` | +57/-0 | 审查分桶文件清单（计费服务），一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/buckets/svc_commerce.txt` | +59/-0 | 审查分桶文件清单（商务服务），一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/buckets/svc_gateway.txt` | +205/-0 | 审查分桶文件清单（网关服务），一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/buckets/svc_misc_a.txt` | +186/-0 | 审查分桶文件清单（服务杂项 A），一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/buckets/svc_misc_b.txt` | +169/-0 | 审查分桶文件清单（服务杂项 B），一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/buckets/svc_providers.txt` | +98/-0 | 审查分桶文件清单（providers），一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/buckets/svc_security.txt` | +58/-0 | 审查分桶文件清单（安全服务），一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/account-scheduler.md` | +24/-0 | 账号调度器审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/auth-oauth-handlers.md` | +10/-0 | OAuth handler 审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/billing-main.md` | +26/-0 | 计费主流程审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/billing-recordusage.md` | +15/-0 | 用量记录审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/commerce.md` | +20/-0 | 商务模块审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/frontend.md` | +15/-0 | 前端审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/gateway-images.md` | +9/-0 | 网关图片链路审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/gateway-kiro.md` | +6/-0 | 网关 Kiro 审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/gateway-parent-ws-core.md` | +20/-0 | 网关 WS 核心审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/gateway-protocol.md` | +13/-0 | 网关协议审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/gateway-service-core.md` | +9/-0 | 网关服务核心审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/gateway-ws-failover.md` | +10/-0 | 网关 WS failover 审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/handler-account.md` | +12/-0 | 账号 handler 审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/handler-admin.md` | +16/-0 | 管理 handler 审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/handler-parent.md` | +18/-0 | handler 汇总审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/handler-settings.md` | +18/-0 | 设置 handler 审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/pkg-httpclient-tls.md` | +14/-0 | httpclient TLS 审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/pkg-oauth-tokenclients.md` | +17/-0 | OAuth token 客户端审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/pkg-protocol-clients.md` | +12/-0 | 协议客户端审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/providers-gemini.md` | +18/-0 | Gemini provider 审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/providers-kiro.md` | +19/-0 | Kiro provider 审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/repo-pkg-parent.md` | +4/-0 | repo/pkg 汇总审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/schema-migrations-infra.md` | +20/-0 | schema 迁移与基础设施审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/svc-backup-objectstorage.md` | +14/-0 | 备份与对象存储服务审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/svc-channel-monitor.md` | +16/-0 | 渠道监控服务审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/svc-misc-a-parent.md` | +16/-0 | 服务杂项 A 汇总审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/svc-misc-b-parent.md` | +10/-0 | 服务杂项 B 汇总审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/svc-security.md` | +12/-0 | 安全服务审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/svc-setting-user-cluster.md` | +15/-0 | 设置/用户/集群服务审查结果，一次性产物 | ☐ 待定 |
| P3 | `.review/upstream-full-2026-07-19/results/svc-tls-fingerprint.md` | +3/-0 | TLS 指纹服务审查结果，一次性产物 | ☐ 待定 |

### 10.3 根级历史审计报告（5 个）

2026-07 两轮审计 / 代码审查的历史报告（~2,400 行），问题应已处理，报告本身绑定当时代码状态，整体放弃。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P3 | `AUDIT_REPORT_2026-07-14.md` | +871/-0 | 2026-07-14 审计报告，历史产物，问题已随后续修复消化 | ☐ 待定 |
| P3 | `AUDIT_REPORT_2026-07-16.md` | +646/-0 | 2026-07-16 审计报告，历史产物 | ☐ 待定 |
| P3 | `AUDIT_RETEST_2026-07-16.md` | +285/-0 | 2026-07-16 审计复测记录，历史产物 | ☐ 待定 |
| P3 | `CODE_REVIEW_REPORT_2026-07-14.md` | +239/-0 | 2026-07-14 代码审查报告，历史产物 | ☐ 待定 |
| P3 | `REVIEW_FINDINGS_VALIDATION_2026-07-14.md` | +362/-0 | 审查发现验证记录，历史产物 | ☐ 待定 |

### 10.4 旧分支上游合并操作记录 `docs/upstream-merge/`（3 个）

personal-dev 三次上游合并的操作日志，绑定旧分支的 commit 与冲突上下文，对新分支已无参考意义。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P3 | `docs/upstream-merge/2026-07-29-personal-dev.md` | +616/-0 | 2026-07-29 合并操作记录，已失去上下文 | ☐ 待定 |
| P3 | `docs/upstream-merge/2026-07-31-personal-dev.md` | +219/-0 | 2026-07-31 合并操作记录，已失去上下文 | ☐ 待定 |
| P3 | `docs/upstream-merge/2026-08-09-personal-dev.md` | +103/-0 | 2026-08-09 合并操作记录，已失去上下文 | ☐ 待定 |

### 10.5 一次性联调 / 调试产物（tools，8 个）

OpenAI 图片 / OAuth 联调期间的探针脚本、Apifox 导出和数据修补 SQL，均服务于已完成的一次性任务。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P3 | `tools/openai_completions_apifox.json` | +246/-0 | Apifox 接口调试导出（completions），一次性联调数据 | ☐ 待定 |
| P3 | `tools/openai_images_apifox.json` | +890/-0 | Apifox 接口调试导出（images），一次性联调数据 | ☐ 待定 |
| P3 | `tools/openai_images_test.py` | +1154/-0 | OpenAI 图片接口联调脚本，任务已完成；与 `tools/test_openai_images_test.py` 成对放弃 | ☐ 待定 |
| P3 | `tools/test_openai_images_test.py` | +252/-0 | 上述联调脚本的测试，与 `tools/openai_images_test.py` 成对放弃 | ☐ 待定 |
| P3 | `tools/openai_oauth_responses_probe.py` | +513/-0 | OAuth responses 探针脚本，一次性调试产物；与 `tools/test_openai_oauth_responses_probe.py` 成对放弃 | ☐ 待定 |
| P3 | `tools/test_openai_oauth_responses_probe.py` | +67/-0 | 上述探针脚本的测试，与 `tools/openai_oauth_responses_probe.py` 成对放弃 | ☐ 待定 |
| P3 | `tools/crs_capture_proxy.js` | +769/-0 | CRS 请求抓包代理脚本，服务于历史抓包分析任务 | ☐ 待定 |
| P3 | `tools/seed_openai_temp_unsched_rules.sql` | +175/-0 | 一次性数据修补 SQL（默认 ROLLBACK 的账号规则种子），已执行完毕 | ☐ 待定 |

### 10.6 Skill Runner 示例占位数据（2 个）

沙箱示例的输入占位样例（各 3 行 JSON），无实质内容，随第 6 节示例目录也可现场重新生成。

| 重要度 | 文件 | 行数 | 说明 | 取舍 |
|---|---|---|---|---|
| P3 | `deploy/skill-runner/runtime/node/input/request.json` | +3/-0 | node 沙箱输入占位样例，示例数据可随时重建 | ☐ 待定 |
| P3 | `deploy/skill-runner/runtime/python/input/request.json` | +3/-0 | python 沙箱输入占位样例，示例数据可随时重建 | ☐ 待定 |

---

## 完整性核对

核对方法：临时脚本（`/tmp/check_infra_md.py`，不入库）提取本文档所有表格行中的文件路径与 `+新增/-删除` 行数，与权威输入 `/tmp/migration-analysis/numstat-D-infra.txt`（`git diff upstream/main...personal-dev --numstat`）逐一比对。

核对结果（2026-08-13）：

```
numstat 文件数: 154
文档表格行数(含重复): 154
文档唯一路径数: 154
重复: 无
遗漏 (0): 无
多余 (0): 无
行数不符 (0): 无
RESULT: PASS 154/154
```

结论：**154/154 全覆盖，每个文件出现且仅出现一次，行数与 numstat 完全一致，零遗漏零多余。**

各重要度分布：P0 × 10、P1 × 32、P2 × 14、待定 × 15（含逐文件建议值）、P3 × 83，合计 154。
