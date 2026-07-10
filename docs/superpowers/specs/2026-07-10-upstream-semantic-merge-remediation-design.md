# 上游语义合并、防回退与问题修复设计

## 文档状态

- 方案状态：概念方案已批准，本文等待用户书面复核。
- 产品代码状态：尚未合并上游，尚未修复产品代码。
- 本文批准后的下一步：编写实施计划，生成完整的 60 行上游提交矩阵并逐组请求批准。
- 硬门禁：60 个提交的处理方式和 `observe + pre-hash` 契约未全部批准前，不得创建合并提交。

## 1. 目标

本设计把一次高分叉上游同步变成可追溯、可复核、可重复执行的语义合并流程，目标如下：

1. 对固定上游中的 60 个上游独有提交逐个分析，不能按提交标题或文本冲突数量抽样。
2. 识别文本冲突、无文本冲突的语义冲突，以及对本分支 985 个独有提交所形成契约的回退风险。
3. 为每个上游提交指定唯一处理方式、owner、reviewer、证据和验证命令，并按依赖语义组取得用户批准。
4. 先明确 `observe + pre-hash` 的优先级，再开始合并执行。
5. 在隔离工作树中保留上游祖先关系完成 merge；禁止用 cherry-pick 集合冒充上游同步。
6. 合并后重新判定已知问题，复用上游已有修复，仅对仍存在的问题做独立、可测试的修复。
7. 让全部本地自动化测试、生成检查和生产构建在同一个最终树上通过。
8. 在最终代码树上完成至少三轮相互独立的审查；任何代码变化都会使受影响的审查结论失效。
9. 把提交历史、决策、冲突处理、验证和审查结果固化到 `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api`，供以后上游同步和冲突处理复用。

## 2. 固定基线

本设计依据以下不可变审计快照：

| 名称 | 固定值 |
|---|---|
| 仓库 | `/Users/ianshaw/Documents/code/personal/sub2api` |
| 分支 | `personal-dev` |
| audited product HEAD | `d0140cde50c92f8bd8eed52d5b50661b0fec980e` |
| upstream | `e316ebf52838a89d57fc790981cce7520f819ac8` |
| merge base | `12d811bd76572836d6df6e1fa8aa5ff91be3b12e` |
| 本地独有提交 | 985 |
| 上游独有提交 | 60 |

本文本身需要提交到仓库，因此执行阶段另记 `integration_base`。从 audited product HEAD 到 `integration_base` 之间只允许本文及其直接批准的计划/审计文档；执行门禁必须用路径差异证明没有产品代码变化。出现任何其他变化时，旧分析不得自动沿用，必须重新冻结基线并重新判断受影响矩阵行。

现有 `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api` 是固定审计输入，不得覆盖。后续运行使用版本化子目录保存新证据。

## 3. 范围

### 3.1 包含

- 60 个上游独有提交的逐提交影响分析。
- 985 个本地独有提交的合并后存续复核。
- Gateway/protocol、账号调度、计费与 usage、repository/schema/migration、moderation、安全、frontend、CI/release、生成物和文档契约。
- 读操作形式的三树冲突预演、历史追踪和本地自动化验证。
- 本地 unit、integration/testcontainers、无需真实供应商的 hermetic E2E、lint、生成检查、前后端生产构建和本地 Docker 构建。
- 已知 P1/P2 问题的合并后重判和修复。

### 3.2 不包含

- 真实供应商账号调用。
- 当前依赖运行中网关和 Claude/Gemini key 的 `make test-e2e-local` live-provider flow。
- 真实支付、退款或回调验证。
- GitHub release 发布或真实 tag 发布。
- 生产数据库迁移和生产环境部署。
- 线上监控窗口和真实流量验收。

上述排除项不能被记录为已通过；最终报告必须明确标记为 `OUT_OF_SCOPE_NOT_RUN`。本地可自动执行的 integration、hermetic E2E 或构建失败不能借用这些排除项豁免。

## 4. 术语与判定原则

- **文本冲突**：Git 三树合并不能自动生成结果。
- **语义冲突**：Git 可自动合并，但组合结果破坏任一已批准行为、数据、配置、生成物或测试契约。
- **功能回退**：本地已存在且未明确批准移除的用户可见行为、兼容性、安全性、计费、数据或运维能力在合并后消失或降级。
- **上游完整性**：每个上游提交的意图都被采纳、等价实现、明确保留本地替代，或证明无影响；不能仅因 SHA 成为祖先就视为完成。
- **语义组**：需要作为一个契约共同决策和验证的一组提交。提交仍逐行分析，分组只减少审批噪声。
- **同一最终树**：所有最终 gate 和三轮审查引用相同的 tree SHA。任何产品代码、migration、配置、生成物或测试变化都会产生新 tree SHA。

分析以行为契约为单位，不以“ours/theirs 哪边行数更多”为依据。对于重组后的大文件，禁止整文件选边后直接放行。

## 5. 交付物与目录结构

设计规范保存在仓库中；执行证据保存在用户指定的审计目录：

```text
/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/
  upstream-merge-remediation/
    task_plan.md
    findings.md
    progress.md
    runs/
      2026-07-10-d0140cde-e316ebf5/
        manifest.json
        upstream-commit-matrix.tsv
        upstream-commit-matrix.md
        semantic-groups.md
        approval-events.tsv
        conflict-rehearsal.tsv
        known-issues.tsv
        local-feature-survival-post.tsv
        decisions/
          OBSERVE-PREHASH-001.md
        resolutions/
          MERGE-YYYYMMDD-NNN.md
        verification/
          matrix.md
          logs/
        reviews/
          round-1-module-correctness.md
          round-2-feature-survival.md
          round-3-release-risk.md
        final-report.md
```

`manifest.json` 至少固定 audited product HEAD、integration base、upstream、merge base、commit counts、生成时间、工具版本和每个核心 TSV/Markdown 的 SHA-256。已批准文件内容变化后必须更新 digest，并让对应批准状态变为 `reopened`。

## 6. 60 行上游提交矩阵

### 6.1 集合与顺序

矩阵正文必须恰好 60 行，一行对应一个完整 SHA，不允许合并行、拆分行或遗漏 merge commit。规范集合由下式生成：

```bash
git rev-list --reverse --topo-order \
  12d811bd76572836d6df6e1fa8aa5ff91be3b12e..e316ebf52838a89d57fc790981cce7520f819ac8
```

完成条件：

1. 矩阵 SHA 集合与命令输出双向差集为空。
2. 正文行数为 60，SHA 唯一，`sequence` 连续为 `001..060`。
3. 每行有且仅有一个 `primary_group_id`；跨组依赖放入 `dependent_group_ids_json`。
4. 多路径、多模块或多提交引用使用单行 JSON 数组，不能使用含义不明的逗号文本。

### 6.2 精确 schema

`upstream-commit-matrix.tsv` 使用 UTF-8、tab 分隔、首行为 header。列顺序固定如下：

| # | 列 | 规则 |
|---:|---|---|
| 1 | `sequence` | `001..060` |
| 2 | `commit_sha` | 40 位完整 SHA |
| 3 | `parent_shas_json` | 父提交 SHA 数组 |
| 4 | `authored_at` | ISO-8601 UTC |
| 5 | `subject` | 原始提交标题 |
| 6 | `primary_group_id` | 唯一语义组 ID |
| 7 | `dependent_group_ids_json` | 跨组依赖数组 |
| 8 | `modules_json` | 受影响模块数组 |
| 9 | `changed_paths_json` | 相对父树的路径数组 |
| 10 | `upstream_intent` | 可验证的行为意图，不复述标题 |
| 11 | `upstream_dependencies_json` | 前置提交或契约数组 |
| 12 | `local_overlap_paths_json` | 与本地当前树重叠路径数组 |
| 13 | `local_overlap_commits_json` | 相关本地完整 SHA 数组 |
| 14 | `text_conflict` | `none|expected|confirmed` |
| 15 | `semantic_conflict` | `none|possible|confirmed` |
| 16 | `local_contracts_at_risk_json` | 可能回退的具体行为数组 |
| 17 | `migration_config_generated_json` | migration/config/DI/生成物影响数组 |
| 18 | `known_issue_refs_json` | `P1-xx`、`P2-xx` 等引用数组 |
| 19 | `proposed_treatment` | 五种允许处理之一 |
| 20 | `treatment_detail` | 预期最终契约和组合边界 |
| 21 | `risk` | `critical|high|medium|low` |
| 22 | `owner` | 实现/分析 owner |
| 23 | `reviewer` | 与 owner 区分的 reviewer |
| 24 | `focused_tests_json` | 精确命令或 suite 数组 |
| 25 | `analysis_evidence_json` | diff、测试、文档、历史证据路径数组 |
| 26 | `analysis_status` | `pending|needs_evidence|analyzed` |
| 27 | `decision_status` | `pending|approved|rejected|reopened` |
| 28 | `approval_event_id` | 对应审批事件；未批准为空 |
| 29 | `execution_commit` | 实际落地提交；执行前为空 |
| 30 | `post_merge_status` | `not_executed|implemented|equivalent|failed|reopened` |
| 31 | `survival_evidence_json` | 合并后契约证据数组 |
| 32 | `notes` | 仅存无法结构化的补充信息 |

### 6.3 允许的处理方式

`proposed_treatment` 只能取以下值：

| 处理方式 | 含义 | 必需证据 |
|---|---|---|
| `adopt_upstream` | 上游契约成为最终契约，本地冲突行为明确被替换 | 上游测试、与本地调用链兼容证明 |
| `retain_local` | 保留本地契约，上游该部分不适用或会回退功能 | 上游意图未丢失的说明、保留理由、回归测试 |
| `manual_compose` | 上游与本地行为都必须存续，手工组合 | 两侧契约清单、组合设计、双侧测试 |
| `locally_superseded` | 本地已有等价或更完整实现，无需重复代码 | 行为等价对照和测试，不能只看代码相似 |
| `no_impact` | 提交对当前产品树无行为影响 | 路径/依赖/生成物检查证明 |

`retain_local` 和 `locally_superseded` 仍然通过真正 merge 保留上游祖先关系；它们不是丢弃上游历史的理由。

### 6.4 单提交分析步骤

每行按同一流程完成：

1. 阅读相对每个父提交的完整 patch、测试、配置、migration 和生成物变化。
2. 识别提交依赖，不能脱离前后提交判断半成品状态。
3. 将受影响路径与 `features.tsv`、`commit-files.tsv`、59 行 merge ledger 交叉关联。
4. 对照 base、audited product HEAD、upstream 三棵树列出两侧行为契约。
5. 运行三树冲突预演并记录文本冲突；同时检查自动合并可能形成的语义冲突。
6. 明确对本地功能的回退风险、数据/计费/安全影响和需要保留的边界。
7. 提出五选一处理方式及 focused tests。
8. 由非 owner reviewer 核对证据后把 `analysis_status` 设为 `analyzed`。

提交标题、自动合并成功、已有类似 helper、或上游测试通过，都不能单独作为 `analyzed` 证据。

### 6.5 985 行本地功能存续台账

`local-feature-survival-post.tsv` 必须与固定 `data/features.tsv` 的 985 个 `commit` 一一对应，不能因文档提交或 merge commit 改变集合。列顺序固定为：

```text
commit
primary_module
feature_id
role
baseline_status
baseline_confidence
post_merge_status
contract_evidence_json
upstream_matrix_refs_json
decision_event_id
reviewer
verification_status
notes
```

`post_merge_status` 只能取 `retained|modified_equivalent|approved_superseded|approved_intentional_removal|partial|reverted|unable_to_verify`；`verification_status` 只能取 `pending|verified|failed|reopened`。最终 gate 不允许 `partial|reverted|unable_to_verify`，除非对应行为已被重新分类为用户批准的 superseded/removal 并有 approval event。

## 7. 语义分组与审批

初始依赖顺序如下。矩阵分析可以拆分组，但不能为了减少行数把无依赖的风险混在一个决策中；任何新增组都必须在合并门禁前固定。

| 顺序 | 初始语义组 | 重点保留的本地契约 |
|---:|---|---|
| 1 | final UA/originator pairing | custom UA、ForceCodexCLI、Messages 例外、probe profile |
| 2 | image namespace intent/strip | 本地 image route、group controls、Spark policy |
| 3 | cache creation 与计费 | GPT-5.6 价格、raw chat、Messages、usage/logging |
| 4 | Grok effort metadata | composer、media、session bridge |
| 5 | MCP/tool_search/custom tools | namespace flattening、collision rejection、各 transport |
| 6 | `parallel_tool_calls` | Responses/Chat/Messages 双向转换 |
| 7 | effort candidate/model suffix | alias、mapping、usage metadata |
| 8 | client floor/WS reset | WS v1/v2、replay、platform errors |
| 9 | compact SSE/raw item | compact handler、heartbeat、failed passthrough |
| 10 | user-scoped Fast/Flex | tabbed/lazy Settings、simple/standard mode |
| 11 | public settings/payment concurrency | route guard、invoice/CNY、idempotency、claim lock |
| 12 | setup-token/ops/request types/pricing | credential preservation、ops lifecycle、cyber 类型、本地价格 |
| 13 | 其余已证明独立的 upstream maintenance | 由矩阵逐行证明无遗漏、无隐含依赖 |

审批以 `semantic-groups.md` 为阅读入口，但每组必须列出其全部 SHA、逐行处理方式、风险、冲突和测试。用户可以批准整组，也可以拒绝或修改单行；审批结果写入 `approval-events.tsv`。

每个审批事件包含：`event_id`、UTC 时间、matrix digest、scope type、scope ID、批准/拒绝/重开、approver、原始回复引用和备注。批准后出现以下任一变化时，相关组自动变为 `reopened`：

- 提交处理方式、最终契约或 focused tests 改变；
- 新发现文本/语义冲突；
- upstream、integration base 或依赖组改变；
- owner/reviewer 发现原证据不足；
- 合并预演与矩阵预测不一致。

## 8. `observe + pre-hash` 前置决策

### 8.1 已知矛盾

当前证据不能支持静默选择：

- `observe` 中英文描述均声明请求直接放行并异步审核。
- pre-hash 描述声明历史命中会被前置拦截。
- `content_moderation.go` 在进入 observe 异步分支之前执行哈希查询，并在任何非 `off` mode 命中时返回 block。
- 现有 pre-hash 测试只显式覆盖 `pre_block`；没有 observe + hash hit 契约测试。

### 8.2 决策证据

`decisions/OBSERVE-PREHASH-001.md` 必须在任何 merge 执行前完成：

1. 追踪 mode 与 pre-hash 的引入提交、后续修改、blame 和测试历史。
2. 对照后台文案、API schema、保存/加载行为、service 分支、handler 和 metrics。
3. 比较三种候选契约：mode 优先、pre-hash 优先、或仅允许 `pre_block` 开启 pre-hash。
4. 对每种候选列出兼容性、已有配置迁移、UI 交互、审计、通知、封禁计数和性能影响。
5. 给出推荐项，但必须由用户明确批准最终真值表。

真值表至少覆盖以下维度：

```text
enabled × mode(off|observe|pre_block)
× pre_hash_check_enabled(false|true)
× hash_result(hit|miss|cache_error)
→ lookup / allow-or-block / sync-or-async moderation
  / persisted action / email / ban count / metrics / client status
```

批准后先写失败测试，覆盖 3 种 mode × hit/miss，并增加 cache error、empty current input、keyword-only 和 handler-before-account-selection 边界。实现、UI 文案和测试必须遵循同一真值表。该决策若在后续审查中改变，相关 moderation 修复和三轮审查全部失效。

## 9. No-Merge Gate

只有以下条件全部为真，实施计划才可执行实际 merge：

```text
[ ] source checkout clean
[ ] audited product HEAD、upstream、merge base 与 manifest 一致
[ ] integration_base 相对 audited product HEAD 只有批准的文档路径
[ ] upstream SHA 集合与 60 行矩阵完全相等
[ ] 60 行 analysis_status 全部为 analyzed
[ ] 60 行 decision_status 全部为 approved
[ ] 所有 semantic group 均有有效 approval event
[ ] OBSERVE-PREHASH-001 有批准的真值表
[ ] 所有 critical/high 行有不同 owner 与 reviewer
[ ] 三树预演没有未记录冲突
[ ] 每行都有 focused tests 和证据
[ ] matrix、groups、decision 文件 digest 与审批时一致
```

门禁由脚本或确定性查询验证，不能靠人工“看起来齐了”。任何一项失败，状态是 `BLOCKED_BEFORE_MERGE`，不得创建 merge commit。

## 10. 隔离合并与实施顺序

### 10.1 隔离环境

批准后使用 `superpowers:using-git-worktrees` 创建 `fix/` 前缀的独立分支和工作树。源 `personal-dev` checkout 保持干净。分支从批准的 `integration_base` 创建，并在工作树内再次验证 refs 和矩阵 digest。

### 10.2 合并

使用固定 SHA，保留两个父提交并在自动提交前暂停：

```bash
git merge --no-ff --no-commit e316ebf52838a89d57fc790981cce7520f819ac8
```

规则：

1. 所有文本冲突只按批准的矩阵和 resolution record 处理。
2. 高风险大文件禁止整文件 `ours/theirs` 后放行。
3. merge commit 前检查 index、combined diff、父树差异和所有冲突标记。
4. merge commit 的父提交必须包含批准的 `integration_base` 与固定 upstream SHA。
5. 合并时出现未预测冲突或需要改变契约，立即停止；记录证据，终止本次未提交 merge，重开受影响矩阵组并重新审批。
6. 不能用 rebase、squash 或 60 个 cherry-pick 替代该 merge。

文本冲突必须在 merge commit 中解决；自动合并后仍需组合的语义变更按组使用独立 follow-up commit，便于 review、bisect 和 revert。每个 follow-up commit 只能引用已经批准的 treatment；新需求需要新审批。

### 10.3 合并后重基线

merge commit 形成后立即：

1. 记录实际父 SHA、tree SHA、conflict resolutions 和 execution commit。
2. 将 60 行 `post_merge_status` 逐行更新并附证据。
3. 对 985 行本地 feature ledger 重新做存续比较。
4. 比较 local parent → merge tree、upstream parent → merge tree、base → 三棵结果树。
5. 重新运行 baseline 红灯，区分合并已修复、仍存在、合并新引入和证据不足。

## 11. 已知问题重判与修复

`known-issues.tsv` 每行包含 finding ID、基线证据、相关 upstream SHA、合并后复现命令、状态、处理提交、测试和 reviewer。合并后状态只能取：

- `resolved_by_upstream`
- `still_present`
- `introduced_or_worsened_by_merge`
- `superseded_by_approved_contract`
- `out_of_scope`

不能因提交标题声称 `fix` 就标记解决。`still_present` 和 `introduced_or_worsened_by_merge` 必须先增加或确认失败测试，再进行最小修复。修复使用独立语义 commit，并遵循 repository 层次、接口 test doubles、Ent/Wire 生成和 pnpm lockfile 约束。

本轮至少重新判定：

- Go release/toolchain pin 不一致。
- 七组 frontend 实现/测试分裂与 22 个失败。
- server repository stubs 编译失败。
- 17 组 migration prefix contract。
- cache-write billing。
- OAuth final UA/originator pairing。
- image namespace intent/strip。
- foreign-model routing guard。
- moderation notification/log persistence ordering。
- Grok `reasoning_effort` metadata。
- non-batch image route-limit scheduler。
- pnpm pin 与 lockfile override。
- documented Wire generation。
- `observe + pre-hash` 批准契约。

所有 in-scope P1/P2 都必须解决；任何保留问题都需要用户单独明确改变范围，不能由实现者自批 waiver。

## 12. 自动化验证矩阵

### 12.1 提交级 focused gate

每个 semantic group 先执行矩阵中列出的 focused tests。涉及以下边界时必须成对覆盖：

- HTTP、passthrough、WS v1/v2、compact、Messages、raw Chat。
- request conversion、response conversion、usage extraction、计费和 persisted log。
- account capability/probe、route limit、fallback 和 sticky state。
- frontend request wiring、旧设置迁移、lazy mount、partial success 和 URL sanitization。
- repository interface/test doubles、migration order、schema/DI、standard/simple mode。

focused gate 通过不能替代最终 full gate。

### 12.2 最终必过 gate

所有命令在同一最终 tree SHA 上重新执行并记录 exit code、工具版本、开始/结束时间和完整日志：

```bash
# Audit integrity
cd /Users/ianshaw/Documents/code/personal/changeLogs/sub2api
python3 -m unittest -v scripts/test_generate_inventory.py
python3 -m unittest -v scripts/test_build_audit_artifacts.py

# Frontend
cd /Users/ianshaw/Documents/code/personal/sub2api
corepack pnpm@9.15.9 --dir frontend install --frozen-lockfile
corepack pnpm@9.15.9 --dir frontend run typecheck
corepack pnpm@9.15.9 --dir frontend run lint:check
corepack pnpm@9.15.9 --dir frontend run test:run
corepack pnpm@9.15.9 --dir frontend run build

# Backend
cd backend
go test -tags=unit ./... -count=1
go test -tags=integration ./... -count=1
go vet ./...
golangci-lint run ./...

# Generation drift, from a clean generated baseline
go generate ./ent
go generate ./cmd/server
git diff --exit-code

# Repository-level build/test
cd ..
# PATH 中的 pnpm 必须先与 frontend/package.json 的 packageManager 完全一致
pnpm --version
make test
make build

# Production embedded binary
cd backend
go build -tags embed \
  -ldflags="-s -w -X main.Version=$(cat cmd/server/VERSION)" \
  -trimpath -o bin/server ./cmd/server
go test -tags=embed ./internal/web -count=1

# Moderation concurrency contract
go test -race -tags=unit ./internal/service \
  -run 'ContentModeration|CyberPolicy' -count=1

# Tracked shell syntax
cd ..
git ls-files -z '*.sh' | xargs -0 -n1 bash -n

# Local production-container builds
docker build -f Dockerfile -t sub2api:semantic-review-root .
docker build -f deploy/Dockerfile -t sub2api:semantic-review-deploy .
```

还必须执行并记录：

- migration prefix exact-set、依赖和 lexicographic order 检查。
- release/setup/verify、`go.mod`、Dockerfile 和 CI Go patch version 一致性检查。
- `packageManager`、CI/Docker pnpm 与 lockfile schema 一致性检查。
- tracked shell 脚本 `bash -n`。
- `pnpm --version` 必须与 `frontend/package.json` 的 `packageManager` 完全一致；`make test` 不得落到其他全局 pnpm。
- moderation race-focused suite 必须实际覆盖批准后的 email-enabled persistence path，不能只复用当前未接线的 suite。

如果 Docker、testcontainers 或已识别的 hermetic E2E 所需本地服务不可用，最终状态是 blocked，不得写成 pass。真实供应商、支付、GitHub release 和生产部署保持明确的 scope exclusion。

仅不依赖真实账号、真实支付或生产服务的 hermetic E2E 属于必过 gate。当前 `make test-e2e-local` 会访问运行中网关并按环境使用 Claude/Gemini key，因此本轮记录为 `OUT_OF_SCOPE_NOT_RUN`，不得记为 pass，也不得因未运行阻断本地 unit/integration/build 结论。以后若拆出完全 hermetic 的 E2E target，该 target 自动进入必过 gate。

### 12.3 失败处理

- 确定性失败：记录最小复现、根因和 responsible group；修复后重跑 focused 与全部最终 gate。
- flaky failure：保留失败日志，重复运行只用于确认，不得用“重跑变绿”关闭；必须消除根因或作为未解决问题阻断。
- migration 已可能在生产执行：禁止盲目重命名，采用兼容 migration、精确 allowlist 或 forward-fix，并记录选择。
- 生成物漂移：判断源文件还是生成文件错误，执行规范生成命令并提交完整生成结果。
- 工具版本漂移：先按仓库 pin 恢复工具链，再判断代码失败。

## 13. 三轮独立审查

三轮均在所有必过 gate 通过后开始，并引用同一 tree SHA。审查报告以 finding 优先，包含 severity、文件/行、复现、影响、建议处理、owner 和状态；“未发现问题”也必须记录剩余覆盖边界。

### Round 1：模块正确性与测试充分性

- Gateway owner：protocol、transport、routing、scheduler、provider compatibility、usage/billing 边界。
- Platform owner：repository、schema、migration、billing、moderation、安全、config、DI。
- Frontend/ops owner：UI/API wiring、i18n、persisted settings、CI/release、build。
- Integration owner：跨层 contract 和测试覆盖汇总。

reviewer 不能批准自己实现的 high/critical resolution。目标是确认实现完整、边界一致、测试能在修复前失败且覆盖所有 transport/mode。

### Round 2：本地功能存续与上游完整性

逐行复核 985 行本地 ledger 和 60 行 upstream matrix：

- 每个本地 contract 在最终树中 retained、modified-equivalent、approved-superseded 或 approved-intentional-removal。
- 不允许最终留下 `unable_to_verify` 或未批准的 `partial/reverted`。
- 每个 upstream commit 的 treatment 与实际 diff、测试和 execution commit 一致。
- 对高风险历史 merge 做 base、两个 parent、merge result、final tree 五点比较。
- 查找“测试保留但实现消失”“backend/helper 存在但 UI 未接线”“配置存在但 runtime 不消费”等 split contract。

### Round 3：发布级对抗审查

从失败和滥用角度复核：

- release/toolchain、Docker、embed、生成物和 clean-clone reproducibility。
- authentication/OAuth identity、URL/secret handling、moderation、通知和并发安全。
- cache read/write、价格、余额、退款、幂等、usage persistence 和 reconciliation。
- migration 编号、checksum、执行顺序、index/constraint、已执行迁移兼容性。
- rollback/revert 能力和 out-of-scope 真实环境风险声明。

### 13.4 重启规则

1. 任一审查导致产品代码、测试、migration、配置或生成物变化，旧 tree SHA 上的相关结论立即失效。
2. Round 1 finding 修复后，重跑受影响 focused/full gate 和完整 Round 1。
3. Round 2 finding 修复后，重跑受影响 Round 1、完整 Round 2；若 Round 3 已开始，也作废 Round 3。
4. Round 3 finding 修复后，重跑受影响 Round 1；若改变本地/upstream contract，重跑 Round 2；最后完整重跑 Round 3。
5. refs、matrix treatment 或 `observe + pre-hash` 契约变化，三轮全部作废并回到相应前置审批门禁。
6. 最终放行需要同一 tree SHA 上连续三轮无未解决 in-scope finding。

## 14. 角色与责任

| 角色 | 责任 |
|---|---|
| Decision approver | 用户；批准每个 semantic group、`observe + pre-hash` 契约和任何范围变化 |
| Gateway owner | transport、conversion、routing、scheduler、provider、usage/billing 边界 |
| Platform owner | repository、schema、migration、billing/admin/security/config/DI |
| Frontend/ops owner | frontend、i18n、deploy、CI、release、docs |
| Integration owner | 固定 refs、矩阵完整性、冲突记录、跨模块验证、最终 gate |

同一人可以承担多个 owner 角色，但 high/critical treatment 的 reviewer 必须与实现 owner 区分。Integration owner 不能绕过 Decision approver 修改已批准契约。

## 15. 异常与状态管理

| 条件 | 处理 |
|---|---|
| upstream 移动 | 新建 run snapshot；重新分析新增/变化提交，不覆盖旧 run |
| integration base 出现产品代码变化 | 重新冻结本地基线并重算 feature survival/冲突 |
| 矩阵缺行、重复或集合不等 | `BLOCKED_BEFORE_MERGE` |
| 用户拒绝 treatment | 修订该行及依赖组，重新 review/approval |
| merge 出现未预测冲突 | 停止未提交 merge，记录、重开矩阵、重新审批 |
| full gate 失败 | 归属 finding，修复并重跑；不得降级为 focused pass |
| review 发现新问题 | 按重启规则修复和重审 |
| 无法证明功能存续 | 状态保持 blocked，不得用 `unable_to_verify` 放行 |

所有错误写入 `progress.md` 和对应 run artifact，包含失败命令、attempt、根因和处理。不能重复执行同一失败动作而不改变诊断或方案。

## 16. 需求可追溯性

| 用户要求 | 规范落点 | 放行证据 |
|---|---|---|
| 合并前检查冲突点和上游逐提交回退风险 | 第 6、7、9 节 | 60 行矩阵、冲突预演、有效审批事件 |
| 每一部分确认处理方式后再合并 | 第 7、9 节 | 五选一 treatment、group approval、No-Merge Gate |
| 先处理 `observe + pre-hash` 冲突语义 | 第 8、9 节 | 批准的真值表及 digest，未批准时 blocked |
| 合并上游最新提交且保留历史 | 第 2、10 节 | ref drift 检查、双父 merge commit |
| 防止本分支历史改动被回退 | 第 6、10、13 节 | 985 行 post-survival ledger、Round 2 五点比较 |
| 合并后处理已知问题 | 第 11 节 | `known-issues.tsv`、失败测试、独立修复提交 |
| 测试与 build 全部通过 | 第 12 节 | 同一 tree SHA 的命令日志和零失败矩阵 |
| 至少三轮独立审查 | 第 13 节 | 三份 review 报告、SHA 和重启记录 |
| 按模块/功能保留历史变更记录 | 第 5、6 节 | versioned run artifacts、matrix、resolution records |
| 使用多人协同最佳实践 | 第 7、13、14 节 | owner/reviewer 分离、approval events、review ownership |
| 暂不做真实环境验证 | 第 3、12 节 | 明确的 `OUT_OF_SCOPE_NOT_RUN`，不伪装为 pass |

## 17. 完成与放行条件

只有以下条件全部满足，目标才算完成：

1. 用户批准本文和后续实施计划。
2. 固定 upstream 的 60 个提交全部分析、review、批准和执行，无缺行或隐式决策。
3. `observe + pre-hash` 真值表已批准，backend/frontend 文案与测试一致。
4. merge commit 真实保留批准的 integration base 和 upstream SHA 两侧祖先。
5. 所有文本/语义冲突都有 resolution record、owner、reviewer 和验证证据。
6. 985 个本地 feature contract 无未批准回退；最终无 `unable_to_verify`，任何 intentional removal 均有用户批准。
7. 60 个 upstream intent 全部为 implemented/equivalent 或经批准保留本地契约。
8. 所有 in-scope 已知 P1/P2 和合并新增 finding 均已解决。
9. 自动化验证矩阵在同一最终 tree SHA 上全部通过。
10. 三轮独立审查按重启规则完成，最终没有未解决 in-scope finding。
11. audit manifest、matrix、approval、resolution、verification、review 和 final report 完整且 digest 可复核。
12. 源 `personal-dev` checkout 未被临时构建产物或未提交修改污染。

最终报告必须区分 `PASS`、`BLOCKED` 和 `OUT_OF_SCOPE_NOT_RUN`。在上述条件未满足时，不得声称“无功能回退、无错误回归或可发布”。
