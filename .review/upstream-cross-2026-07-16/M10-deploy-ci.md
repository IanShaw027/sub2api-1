# Module 10: 部署 CI 工具与运维面

## 范围摘要
- 对比基线: `BASE=da85cc7e` → `HEAD=eb64a5c1`，对照 `UPSTREAM=b960ec19`（`upstream/main`）
- 审查路径: `deploy/**`、`.github/**`、`tools/**`、`Dockerfile*`、`Makefile`、`deploy.sh`、`SECURITY.md`、`.goreleaser*`、`backend/cmd/sync_checksums/**`（运维入口）
- 相对上游的主要能力差异:
  - 源码部署脚本 `deploy.sh`：前端/后端身份哈希、deploy lock、原子换二进制、`sync_checksums` 备份/同步/失败回滚
  - 安全扫描流水线：`govulncheck`（版本钉死）+ `tools/secret_scan.py` + pnpm audit 异常清单（含 owner/过期）
  - skill-runner 示例隔离栈（`network_mode: none` / non-root / read-only / 资源限额）
  - Apple container 部署脚本与 shell 测试
  - 双 Dockerfile（root / deploy）+ goreleaser 打包镜像 + CI 镜像 build 冒烟
  - personal-dev 侧 compose.dev 引入 batch-image Vertex/GCS 生产资源默认值、本地安全默认偏松

## 关键路径图

### 源码热更新（systemd）
```
运维执行 deploy.sh
  → acquire lock
  → 计算 frontend_input / source / dist hash
  → 必要时 pnpm install --frozen + build
  → 必要时 go build -tags embed → BINARY.new
  → 从 config.yaml 拼 DSN（env 优先）
  → SUB2API_DATABASE_DSN=… go run ./cmd/sync_checksums --backup-file
       （仅改写 IsMigrationChecksumCompatible allowlist 内的 checksum）
  → 备份旧二进制 + mv 新二进制
  → systemctl restart
  → 校验 ActiveState + 运行中 binary -version 身份
  失败 → 恢复二进制 + restore checksum 快照 → 再 restart
       （不回滚已 Apply 的 schema DDL）
```

### 容器发布
```
tag v* / workflow_dispatch
  → update VERSION artifact + build frontend dist
  → release job: build_metadata.sh → goreleaser release --skip=validate
  → GHCR（+ 可选 DockerHub）多架构 manifest
  → 可选 Telegram 通知 / DockerHub description
  → sync VERSION 回 default branch
```

### Docker 一键准备
```
curl raw.main/deploy/docker-deploy.sh | bash
  → 下载 docker-compose.local.yml（另存 docker-compose.yml）+ .env.example
  → openssl 生成 JWT/TOTP/POSTGRES 密钥写入 .env（chmod 600）
  → 明文打印密钥到 stdout
  → 用户 docker compose up（默认 BIND 0.0.0.0，local 变体 URL allowlist 默认关闭）
```

### 安全门禁
```
push/PR/周一 cron → security-scan.yml
  → govulncheck ./...
  → python tools/secret_scan.py（git ls-files 高置信模式）
  → pnpm audit --prod high + check_pnpm_audit_exceptions.py
```

## 发现清单

### [P0] `deploy.sh` 失败回滚会制造 schema/二进制/checksum 三方错位
- **位置**: `deploy.sh:533-596`，`backend/cmd/sync_checksums/main.go:171-196`、`198-227`
- **相对上游**: 本地增强的源码部署路径（相对 upstream 的运维面分叉热点）
- **问题**: 停服前先 `sync_checksums` 改写 `schema_migrations.checksum`；新二进制启动后会 **forward-only Apply 新迁移**。若启动后因非迁移原因失败，`rollback_and_restart` 会：
  1. 恢复旧二进制；
  2. `restoreDatabaseChecksums` 把 checksum **整表写回部署前快照**；
  3. **不回滚已提交的 schema 变更**。  
  结果可能是：schema 已新、binary 已旧、checksum 被改回旧值。旧二进制可能再次因 checksum mismatch 起不来，或带着“未声明”的新列/约束运行。
- **影响**: 生产升级失败后自动回滚不可信；人工恢复成本高，存在计费/约束语义不一致窗口。属数据与可用性双重风险。
- **证据**:
  - 成功路径先 sync 再 restart：`deploy.sh:533-564`
  - 失败路径 `restore_checksum_snapshot` + 二进制回滚，无 schema down：`deploy.sh:277-338`、`571-596`
  - restore 无条件 `UPDATE schema_migrations SET checksum`：`sync_checksums/main.go:171-196`
  - sync 本身虽 allowlist 收紧，但 restore 不校验 allowlist
- **建议**:
  1. 迁移与切流量解耦：migrate job 成功后再换二进制；
  2. 回滚策略改为 **schema forward-only**：禁止 restore 旧 checksum；只回滚应用层；
  3. 部署前门禁：逻辑备份/快照完成才允许升级；
  4. 失败时输出“当前 schema head / binary version / checksum snapshot path”三联诊断。
- **交叉关注**: M08 Schema/Migration；生产运维手册

### [P0] 推荐生产路径 `docker-compose.local.yml` 安全默认值显著弱于 `docker-compose.yml`
- **位置**:
  - `deploy/docker-compose.local.yml:158-162`（allowlist 默认 false / 允许 insecure HTTP / 允许 private hosts）
  - `deploy/docker-compose.yml:148-155`（allowlist 默认 true / 禁止 insecure / 禁止 private）
  - `deploy/docker-deploy.sh:78-88`（下载 **local** 变体并另存为 `docker-compose.yml`）
  - `deploy/README.md:50-86`（把 local + docker-deploy 标为推荐）
- **相对上游**: 本地/分叉运维文档与 compose 变体长期并存；推荐路径选了宽松默认
- **问题**: 一键脚本与 README 引导用户走 **local** 配置，但该文件把 SSRF/上游 URL 防护默认关闭，且 `BIND_HOST` 默认 `0.0.0.0` 全网暴露管理端与网关。命名体积的 `docker-compose.yml` 反而更严。
- **影响**: 按官方推荐步骤部署的生产实例默认暴露管理面，且 URL allowlist 关闭 → SSRF/私网探测面扩大。
- **证据**:
```158:162:deploy/docker-compose.local.yml
      - SECURITY_URL_ALLOWLIST_ENABLED=${SECURITY_URL_ALLOWLIST_ENABLED:-false}
      - SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP=${SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP:-true}
      - SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS=${SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS:-true}
```
  对比 main compose 的 fail-closed 默认（`deploy/docker-compose.yml:149-153`）。
- **建议**:
  1. local/named/standalone **安全默认对齐**为 fail-closed；
  2. 仅 `docker-compose.dev.yml` 允许宽松默认，并明确 “dev only”；
  3. `docker-deploy.sh` 生成的 compose 写入生产 hardening 注释，或启动时检查危险默认并 warning。
- **交叉关注**: M05 鉴权安全；SSRF/URL allowlist 业务实现

### [P1] root/deploy 镜像 HEALTHCHECK 与 goreleaser 镜像工具链不一致；CI 只 build 不 run
- **位置**: `Dockerfile:145-146`，`deploy/Dockerfile:139-140`，`Dockerfile.goreleaser:23,58-59`，`.github/workflows/backend-ci.yml:63-106`
- **相对上游**: 三份 Dockerfile 漂移
- **问题**: root/deploy 用 `wget` 做 HEALTHCHECK；goreleaser 显式装 `curl` 并用 `curl -f`。CI `docker-image` job 只 `build-push-action`（`load: false`）与临时 goreleaser context build，**从不 `docker run` / 不等待 healthy**。Alpine busybox 可能提供 wget，但该假设未被门禁验证，且与发布镜像路径不一致。
- **影响**: 自建镜像与发布镜像健康检查行为漂移；编排依赖 health 时可能误杀或假绿。
- **证据**: CI 仅 build 不 load/run（`backend-ci.yml:69-106`）；HEALTHCHECK 命令三份文件不一致。
- **建议**: 统一安装 `curl`（或统一 busybox wget）并同一 HEALTHCHECK；CI 增加 `docker run` + `/health` + health state 冒烟。
- **交叉关注**: 发布可用性

### [P1] `install.sh` 在 checksums.txt 缺失时仅 warning 仍继续安装
- **位置**: `deploy/install.sh:587-602`
- **相对上游**: 安装脚本共有面
- **问题**: 下载 release archive 后，若 `checksums.txt` 拉取失败，打印 warning 后仍 `tar` 安装并替换 `/opt/sub2api/sub2api`。配合文档/ GoReleaser footer 的 `curl | bash` 路径，完整性校验变成 best-effort。
- **影响**: 供应链降级：MITM/残缺下载/错误资产可被安装。simple release 配置甚至 `checksum.disable: true`（`.goreleaser.simple.yaml:35-37`），会系统性触发该降级路径。
- **证据**:
```600:602:deploy/install.sh
    else
        print_warning "$(msg 'checksum_not_found')"
    fi
```
- **建议**: checksum 缺失或 mismatch 一律 fail-closed；simple release 若跳过 checksum 则禁止被 install.sh 消费，或单独产物通道。
- **交叉关注**: 发布供应链

### [P1] Release 使用 `goreleaser ... --skip=validate`，且 tag body 写入 `GITHUB_OUTPUT` 分隔脆弱
- **位置**: `.github/workflows/release.yml:152-169`、`187-191`
- **相对上游**: 发布流水线共有
- **问题**:
  1. `--skip=validate` 允许配置/模板错误进入真实 release；
  2. tag message 用固定 `EOF` delimiter 写入 `$GITHUB_OUTPUT`：`echo "$TAG_MESSAGE" >> $GITHUB_OUTPUT`。若 annotated tag body 含单独一行 `EOF`，会截断/污染后续输出，进而污染 Release notes 与 Telegram 文案。
- **影响**: 错误配置可发布；恶意/疏忽 tag 注解可干扰发布产物描述（完整性/社工面）。
- **建议**: 去掉 `--skip=validate` 或改为仅 debug 可开；`GITHUB_OUTPUT` 使用随机 heredoc 定界符；对 `workflow_dispatch` 的 tag 输入做 `^v[0-9]` 校验。
- **交叉关注**: 供应链/发布治理

### [P1] secret scan 覆盖窄，且 tools 单测/部署脚本单测未进入 CI
- **位置**:
  - `tools/secret_scan.py:33-63`（模式集合）
  - `.github/workflows/security-scan.yml:39-40`
  - `tools/test_security_scan_tools.py`、`tools/test_deploy_script.py`（存在但未被 workflow 调用）
  - `Makefile:53-54` 仅本地 `secret-scan` target
- **相对上游**: 本地增强工具
- **问题**:
  1. 扫描为“高置信少量模式”：无私钥以外的 PEM 变体、无 Slack/Discord/Telegram bot、无 Stripe test、无 Anthropic/OpenAI 非 sk- 形态、无 `.env` base64 熵检测；`configured secret` 仅匹配特定 key 名。
  2. `is_test_fixture_path` 跳过 `fixtures/` 与大量 `*_test.*`，合理，但 **CI 未跑** `test_security_scan_tools.py` / `test_deploy_script.py`，回归靠自觉。
  3. `backend-ci.yml` 不跑 e2e、不跑 tools 单测、不跑 `make secret-scan`（secret 只在 security-scan workflow）。
- **影响**: 凭证误提交漏检概率高；deploy/secret 工具回归无门禁。
- **证据**: SECURITY.md 自身也写明 high-confidence only；audit exceptions 有单测，但 workflow 未 `python -m unittest tools/test_*.py`。
- **建议**: security-scan 增加 tools unittest；扩展 provider token 模式；对 `deploy/**` 与 `*.env*` 提高敏感度；backend-ci 至少挂 `test_deploy_script.py`。
- **交叉关注**: M05；既有 AUDIT_REPORT P1-X6

### [P1] `docker-compose.dev.yml` 硬编码生产向 GCP project / GCS bucket 默认值
- **位置**: `deploy/docker-compose.dev.yml:79-87`
- **相对上游**: 本地 personal-dev 增量（上游通常不应带租户资源）
- **问题**: 默认 `BATCH_IMAGE_VERTEX_ENABLED=true`，并内置 `project-28424c50-...` 与 `sub2-batch-image-prod-...` bucket/prefix。这不是密钥，但是 **生产云资源标识与 prod 路径**，开发 compose 一键启用会把本地流量/元数据指向 prod 资源命名空间（若本机 ADC/SA 可用则更危险）。
- **影响**: 环境串扰、费用与数据隔离破坏；仓库泄露租户拓扑。
- **证据**: 全仓仅此处出现该 project id。
- **建议**: 默认 `ENABLED=false`；project/bucket 置空并 `:?required`；prod 值只进私有 env / secret manager。
- **交叉关注**: M07 batch-image / AI Studio

### [P2] compose 多文件配置漂移（安全、媒体、Redis health、端口绑定）
- **位置**: `deploy/docker-compose.yml` / `.local.yml` / `.standalone.yml` / `.dev.yml` / `.env.example`
- **相对上游**: 多变体并行维护
- **问题**:
  1. 安全默认不一致（见 P0）；
  2. Redis 设 `requirepass` 时 healthcheck 仍是 `redis-cli ping`，未使用 `REDISCLI_AUTH`（env 设了但 CMD 未引用）→ 设密码后 health 可能长期失败（`docker-compose.yml:268-283` 等）；
  3. 生产示例 `BIND_HOST=0.0.0.0`（`.env.example:18`）；
  4. `config.example.yaml` 仍有 `admin_password: "admin123"`（`deploy/config.example.yaml:1078`）——示例可接受，但与 docker 自动生成密码叙事并存，易误用。
- **影响**: 运维抄错文件即踩坑；Redis 密码加固反而让编排判定不健康。
- **建议**: 抽出 compose anchor/片段生成；Redis health 改为 `redis-cli ping` 前导出 auth 或用 `redis-cli -a` 安全替代；示例 admin 密码改为明显 placeholder 并文档强调。
- **交叉关注**: 运维文档

### [P2] Dockerfile 双源与供应链默认（GOPROXY/Node 版本）漂移
- **位置**: `Dockerfile` vs `deploy/Dockerfile`；CI Node 20 vs 镜像 `node:24-alpine`
- **相对上游**: 构建面分叉
- **问题**:
  - root Dockerfile 有 BuildKit pnpm cache、`NPM_CONFIG_REGISTRY`、`scripts/resolve-version.sh`；deploy/Dockerfile 无 cache、VERSION 直接读文件。
  - 默认 `GOPROXY=https://goproxy.cn,direct` + `GOSUMDB=sum.golang.google.cn` 绑定中国镜像信任根。
  - CI frontend Node 20，镜像 frontend-builder Node 24 → 构建差异风险。
- **影响**: “CI 绿 / 本地 deploy Dockerfile 红” 或产物不一致；非 CN 构建可用性与供应链审计成本上升。
- **建议**: 单一 Dockerfile 源；默认官方 proxy/sumdb，CN 用 build-arg；Node 主版本对齐。
- **交叉关注**: 发布可复现性

### [P2] skill-runner 示例隔离良好，但未与主栈/CI 强制集成
- **位置**: `deploy/skill-runner/**`，`backend/internal/integration/skillrunner/**`
- **相对上游**: 本地新增
- **问题**: compose 示例正确使用 non-root `65532`、`network_mode: none`、`read_only`、`cap_drop: ALL`、`no-new-privileges`、pids/mem/cpu 限制、skill ro + scratch rw。但：
  1. 与主 `docker-compose*.yml` 完全分离（文档意图如此）；
  2. `prepare-scratch.sh` 将 output 设为 `0733`（任意 uid 可写）——为固定容器 uid 服务，host 多租户场景需知悉；
  3. CI 未跑该 compose 冒烟（仅 integration 包可能覆盖 runner 逻辑，不验证部署清单）。
- **影响**: 生产若“参考示例直接改”漏掉某一 hardening 项，隔离会塌缩；目前缺少部署契约测试。
- **建议**: 增加 compose config 断言测试（必须含 network_mode none 等）；生产 runner 清单从 example 生成而非手抄。
- **交叉关注**: M07 Skills

### [P2] `docker-deploy.sh` / 安装文档 `curl | bash` + 明文打印密钥
- **位置**: `deploy/docker-deploy.sh:133-144`，`deploy/README.md:58`，`.goreleaser.yaml:204-207`
- **相对上游**: 共有安装体验
- **问题**: 从 `raw.githubusercontent.com/.../main/...` 拉脚本与 compose（非 pin commit/tag）；生成的 JWT/TOTP/POSTGRES 打印到 stdout，易进 shell history / 录屏 / CI 日志。
- **影响**: 供应链与凭证泄漏运营风险（非代码 RCE，但是高频事故源）。
- **建议**: pin 到 release tag；密钥只写 `.env`，stdout 仅提示路径；提供 `--quiet`。
- **交叉关注**: 运维安全

### [P2] migration checksum sync 约束本身正确，但与 deploy 生命周期耦合过紧
- **位置**: `backend/internal/repository/migrations_runner.go:77-80,646-664`，`backend/cmd/sync_checksums/main.go:28-32,209-211`
- **相对上游**: personal-dev 为消化历史 in-place 改迁移而膨胀的 allowlist
- **问题**: sync **只**在 `IsMigrationChecksumCompatible` 精确对上时改写，单测覆盖 allowlist/非 allowlist/事务回滚，设计正确。风险在于：
  1. allowlist 持续膨胀意味着“已发布 SQL 仍被改”的流程未冻结；
  2. deploy 在每次发布强制跑 sync，把历史兼容问题塞进热路径；
  3. restore 路径不走 allowlist（见 P0）。
- **影响**: 与上游合并时 checksum 规则冲突面大；运维误以为 sync 能修复任意 drift。
- **建议**: CI immutability：BASE 已存在的 `migrations/*.sql` 内容变更即 fail；禁止新增 compatibility rule（或需双人审批注释）；deploy 仅在检测到 known mismatch 时运行 sync。
- **交叉关注**: M08

### [P3] CI 小一致性 / 文档
- **位置**: `.github/workflows/backend-ci.yml:25` 使用 `pnpm/action-setup@v4`，同文件 frontend job 与其他 workflow 用 `@v6`；`SECURITY.md` 与实现基本一致；`Makefile` 的 `FRONTEND_CI_VITEST` 烟测列表未被 `test-frontend-ci` 使用（后者跑全量 vitest）。
- **相对上游**: 维护性
- **问题**: action 主版本不一致增加供给面；文档/Makefile 目标语义略混。
- **影响**: 低。
- **建议**: 统一 action 版本；明确 smoke vs full。
- **交叉关注**: 无

### [P3] `build_metadata.sh` 通过 `eval "$(…)"` 导出变量
- **位置**: `deploy/build_metadata.sh:96-98`，调用方 `release.yml:177`、`deploy/build_image.sh:11`
- **相对上游**: 本地构建元数据
- **问题**: 当前 `DIRTY∈{clean,dirty}`、hash 为 hex，注入面实际很低；但 `eval` 模式对未来字段扩展不稳健。
- **影响**: 低（现状安全）。
- **建议**: 改为 `source <(…)` 或写 env 文件再 `set -a; source`。
- **交叉关注**: 无

## 与上游合并风险
- **冲突热点文件**:
  - `deploy/docker-compose*.yml`（环境变量块巨大，local/dev 安全默认与 batch-image 段）
  - `Dockerfile` / `deploy/Dockerfile`（双源）
  - `.github/workflows/{backend-ci,release,security-scan}.yml`
  - `deploy.sh` + `backend/cmd/sync_checksums/**`（上游若无对等路径，合入成本高）
  - `.goreleaser.yaml` / `.goreleaser.simple.yaml`
- **语义漂移点**:
  - “推荐生产 compose” 在 local 与 named 之间安全默认相反
  - checksum **兼容** vs checksum **修复**：sync 不会重放 SQL，只改表内哈希
  - simple release 无 checksums.txt，与 install.sh 校验假设冲突
  - skill-runner 仅为示例契约，不是主部署的一部分

## 测试与验证缺口
- CI **未**执行: `tools/test_deploy_script.py`、`tools/test_security_scan_tools.py`、`make test-e2e-local`、镜像 run/health 冒烟、空库→HEAD 与 BASE→HEAD 迁移升级 job
- security-scan 与 backend-ci 分离合理，但 PR 若只看 CI 名可能忽略安全门（两者都 on push/PR，尚可）
- Redis 带密码 health、compose 安全默认、install.sh 无 checksum 路径均无自动化断言
- skill-runner compose hardening 无 contract test
- release `--skip=validate` 使 goreleaser 配置回归靠人工

## 模块结论
- **整体风险评级: High**
- **是否建议合入上游 / 继续分叉 / 先修再合**: **先修再合**（至少处理 P0 安全默认与部署回滚语义；P1 供应链/发布门禁建议同期修）。skill-runner 示例与 secret_scan 工具可继续分叉演进，但不要带着 local compose 宽松默认与 prod GCP 默认值合入上游。
- **Top 3 必须处理项**:
  1. **对齐并收紧推荐生产 compose 的安全默认**（local 与 docker-deploy 路径 fail-closed，BIND 与 allowlist）
  2. **重做 `deploy.sh` 失败回滚策略**（schema forward-only；禁止 restore 旧 checksum 制造三方错位；迁移与切二进制解耦）
  3. **补齐发布/安装完整性与 CI 冒烟**（install checksum fail-closed；去掉或限制 `--skip=validate`；镜像 run+health；tools/deploy 单测进 CI；移除 compose.dev 生产 GCP 默认）
