# Upstream Semantic Merge Preflight Audit And Approval Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 生成并独立复核固定上游 60 个提交的逐行语义矩阵、`observe + pre-hash` 决策记录、冲突预演和 digest 绑定审批记录，使 No-Merge Gate 可被确定性验证。

**Architecture:** 本阶段是合并前 preflight，不执行 source merge。Python 标准库工具复用既有审计库存，通过固定 Git refs 和现有 `commits.tsv`、`commit-files.tsv`、`features.tsv` 生成结构化 run；三条 ownership lane 分别完成语义字段，交叉 reviewer 复核后由 validator 验证 60/985 集合、冲突覆盖、审批 digest 和 owner/reviewer 分离。所有 treatment 批准后，另写一份以真实矩阵为输入的 merge/remediation 实施计划。

**Tech Stack:** Python 3.12 标准库、Git 2.50 `merge-tree`、现有审计 TSV/JSON、Go/Vue 源码只读分析、Markdown 决策记录。

## Global Constraints

- Audited product HEAD 固定为 `d0140cde50c92f8bd8eed52d5b50661b0fec980e`。
- 当前文档 HEAD 为 `4bd23e35164b7d9fdcda329ed137f594391228a3`；计划提交后以新的 documentation-only HEAD 作为 `integration_base`。
- Upstream 固定为 `e316ebf52838a89d57fc790981cce7520f819ac8`，merge base 固定为 `12d811bd76572836d6df6e1fa8aa5ff91be3b12e`。执行前必须 fetch；若 `upstream/main` 移动则新建 run 并重新计算提交数，不能继续使用 60 行结论。
- `integration_base` 相对 audited product HEAD 只允许设计规范和本计划两条文档路径。
- 本阶段不得创建 worktree、不得运行 `git merge`、不得修改产品源码、测试、migration、配置或生成物。
- `upstream-commit-matrix.tsv` 必须恰好 60 行、32 列，一行一个完整 SHA；集合与 `merge-base..upstream` 双向差集为空。
- 五种 treatment 仅为 `adopt_upstream|retain_local|manual_compose|locally_superseded|no_impact`。
- 所有 critical/high 行的 owner 与 reviewer 必须不同；提交标题或自动合并成功不能单独作为分析证据。
- `observe + pre-hash` 真值表必须先获用户批准，随后才允许批准其他 merge treatment。
- 985 行本地 feature ledger 在本阶段只读，不能因文档提交变成 986 行产品功能台账。
- 真实 provider、支付、GitHub release 和生产环境验证不在本阶段范围。
- 新审计文件放在 `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/upstream-merge-remediation/runs/`；该目录不是 Git 仓库，每个 reviewer gate 后以 manifest SHA-256 固化内容。
- 因外部审计目录不是 Git 仓库，Tasks 1-12 的 checkpoint 是通过测试、更新 manifest tooling/artifact SHA-256 并写入 `progress.md`；不得假装存在 Git commit。
- 前端命令只能使用 pnpm；本阶段不安装依赖、不运行产品 build。

## File Structure

### Repository Documentation

- Create: `docs/superpowers/plans/2026-07-10-upstream-semantic-merge-preflight.md`

### Durable Audit Tooling

- Create: `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/scripts/upstream_merge_schema.py`
- Create: `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/scripts/test_upstream_merge_schema.py`
- Create: `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/scripts/prepare_upstream_merge_run.py`
- Create: `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/scripts/test_prepare_upstream_merge_run.py`
- Create: `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/scripts/rehearse_upstream_merge.py`
- Create: `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/scripts/test_rehearse_upstream_merge.py`
- Create: `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/scripts/record_upstream_merge_approval.py`
- Create: `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/scripts/test_record_upstream_merge_approval.py`
- Create: `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/scripts/validate_upstream_merge_run.py`
- Create: `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/scripts/test_validate_upstream_merge_run.py`

### Generated Run

- Create: `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/upstream-merge-remediation/runs/2026-07-10-d0140cde-e316ebf5/manifest.json`
- Create: same run root `upstream-commit-matrix.tsv`, `upstream-commit-matrix.md`, `semantic-groups.md`, `approval-events.tsv`, `conflict-rehearsal.tsv`.
- Create: same run root `decisions/OBSERVE-PREHASH-001.md`.
- Create: same run root `work/gateway-upstream-matrix.tsv`, `work/platform-upstream-matrix.tsv`, `work/frontend-ops-upstream-matrix.tsv`.
- Create: same run root `work/gateway-commits.txt`, `work/platform-commits.txt`, `work/frontend-ops-commits.txt`.
- Create: same run root `evidence/merge-tree.stdout`, `evidence/merge-tree.stderr`, and one `evidence/commits/{commit_sha}.patch` per matrix row.
- Modify: external `upstream-merge-remediation/task_plan.md`, `findings.md`, `progress.md`.

---

### Task 1: Freeze The Matrix Schema And Historical Commit Groups

**Files:**
- Create: `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/scripts/upstream_merge_schema.py`
- Create: `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/scripts/test_upstream_merge_schema.py`

**Interfaces:**
- Produces: `MATRIX_FIELDS`, `JSON_ARRAY_FIELDS`, enums, `HISTORICAL_GROUPS`, `commit_to_group()`, `canonical_json_array()`.
- Consumed by: Tasks 2-5 and all lane reviewers.

- [ ] **Step 1: Write schema tests that require 32 fields and exactly 60 unique grouped SHAs**

```python
from __future__ import annotations

import importlib.util
from pathlib import Path
import unittest

SCRIPT = Path(__file__).with_name("upstream_merge_schema.py")

def load_schema():
    spec = importlib.util.spec_from_file_location("upstream_merge_schema", SCRIPT)
    if spec is None or spec.loader is None:
        raise AssertionError("cannot load upstream_merge_schema.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module

class UpstreamMergeSchemaTests(unittest.TestCase):
    def test_matrix_schema_is_exact(self) -> None:
        schema = load_schema()
        self.assertEqual(len(schema.MATRIX_FIELDS), 32)
        self.assertEqual(schema.MATRIX_FIELDS[0], "sequence")
        self.assertEqual(schema.MATRIX_FIELDS[-1], "notes")
        self.assertEqual(len(schema.MATRIX_FIELDS), len(set(schema.MATRIX_FIELDS)))

    def test_historical_groups_cover_sixty_unique_commits(self) -> None:
        schema = load_schema()
        commits = [sha for group in schema.HISTORICAL_GROUPS.values() for sha in group]
        self.assertEqual(len(commits), 60)
        self.assertEqual(len(set(commits)), 60)
        expected = set((SCRIPT.parent.parent / "data/incoming-upstream.txt").read_text().splitlines())
        self.assertEqual(set(commits), expected)
        lane_counts = {
            lane: sum(len(schema.HISTORICAL_GROUPS[group]) for group in groups)
            for lane, groups in schema.LANE_GROUPS.items()
        }
        self.assertEqual(lane_counts, {"gateway": 36, "platform": 15, "frontend_ops": 9})

    def test_json_arrays_are_canonical(self) -> None:
        schema = load_schema()
        self.assertEqual(schema.canonical_json_array(["b", "a", "b"]), '["a","b"]')
```

- [ ] **Step 2: Run the schema tests and verify the module is absent**

Run: `cd /Users/ianshaw/Documents/code/personal/changeLogs/sub2api && python3 -m unittest -v scripts/test_upstream_merge_schema.py`

Expected: FAIL because `upstream_merge_schema.py` does not exist.

- [ ] **Step 3: Implement the exact field and enum constants**

```python
MATRIX_FIELDS = [
    "sequence", "commit_sha", "parent_shas_json", "authored_at", "subject",
    "primary_group_id", "dependent_group_ids_json", "modules_json",
    "changed_paths_json", "upstream_intent", "upstream_dependencies_json",
    "local_overlap_paths_json", "local_overlap_commits_json", "text_conflict",
    "semantic_conflict", "local_contracts_at_risk_json",
    "migration_config_generated_json", "known_issue_refs_json",
    "proposed_treatment", "treatment_detail", "risk", "owner", "reviewer",
    "focused_tests_json", "analysis_evidence_json", "analysis_status",
    "decision_status", "approval_event_id", "execution_commit",
    "post_merge_status", "survival_evidence_json", "notes",
]
JSON_ARRAY_FIELDS = {field for field in MATRIX_FIELDS if field.endswith("_json")}
TREATMENTS = {"adopt_upstream", "retain_local", "manual_compose", "locally_superseded", "no_impact"}
ANALYSIS_STATUSES = {"pending", "needs_evidence", "analyzed"}
DECISION_STATUSES = {"pending", "approved", "rejected", "reopened"}
POST_MERGE_STATUSES = {"not_executed", "implemented", "equivalent", "failed", "reopened"}
RISKS = {"critical", "high", "medium", "low"}
LANE_GROUPS = {
    "gateway": {"UG-01-compact-sse", "UG-02-codex-client-floor", "UG-03-gpt56-max-effort", "UG-05-parallel-tool-calls", "UG-06-effort-model-candidates", "UG-12-image-namespace", "UG-13-billing-hardening", "UG-14-grok-effort", "UG-16-final-ua-originator", "UG-19-cache-creation-bridge", "UG-20-mcp-tools-bridge"},
    "platform": {"UG-07-payment-concurrency", "UG-09-gpt56-billing", "UG-11-setup-token-refresh", "UG-18-ops-capture-writer"},
    "frontend_ops": {"UG-04-en-locale-keys", "UG-08-user-breakdown-request-type", "UG-10-version-0150", "UG-15-user-fast-flex", "UG-17-version-0151"},
}
```

- [ ] **Step 4: Encode the exact 20 historical groups**

The module must encode this table verbatim; the SHA count in the last column sums to 60.

| Group | Commits | Count |
|---|---|---:|
| `UG-01-compact-sse` | `2cffe1cf`, `ae9a01d8`, `000f6dc6`, `30993620` | 4 |
| `UG-02-codex-client-floor` | `657c4f97`, `066a8594` | 2 |
| `UG-03-gpt56-max-effort` | `80b3d4c1`, `72ef0b38` | 2 |
| `UG-04-en-locale-keys` | `e984b4e2`, `9389503c` | 2 |
| `UG-05-parallel-tool-calls` | `ad8afc8a`, `cddcf490` | 2 |
| `UG-06-effort-model-candidates` | `c3ae5fc3`, `301c99a2`, `b9b013a0`, `312ab1f0` | 4 |
| `UG-07-payment-concurrency` | `fc66a30f`, `ddb1a210` | 2 |
| `UG-08-user-breakdown-request-type` | `dda8f787`, `ea9f40b6`, `5c9748ce` | 3 |
| `UG-09-gpt56-billing` | `4a2b10c9`, `383f61d0`, `062af81f`, `0a5f34a2`, `5c15d32f`, `8b96acde`, `0dec1ad2` | 7 |
| `UG-10-version-0150` | `9a2f11b4` | 1 |
| `UG-11-setup-token-refresh` | `99da3081`, `a495d5e3`, `6a2063b3` | 3 |
| `UG-12-image-namespace` | `d3a1835e`, `d0f6f27d` | 2 |
| `UG-13-billing-hardening` | `de28eba3`, `6dd3274a` | 2 |
| `UG-14-grok-effort` | `0fa1eb85`, `5a0dd510`, `815516d8` | 3 |
| `UG-15-user-fast-flex` | `f2966530`, `5260a42a` | 2 |
| `UG-16-final-ua-originator` | `8a51119e`, `deff3123` | 2 |
| `UG-17-version-0151` | `6c588bb9` | 1 |
| `UG-18-ops-capture-writer` | `89a551b9`, `bc3cb290`, `151b9265` | 3 |
| `UG-19-cache-creation-bridge` | `0d28f7f9`, `83f169e4`, `07fac347` | 3 |
| `UG-20-mcp-tools-bridge` | `75fb3c41`, `27e29f05`, `18e26c12`, `79423383`, `f1082bb7`, `eb4d0050`, `a2cdaa64`, `e2b68d1f`, `90e9d03d`, `e316ebf5` | 10 |

Use the full 40-character SHAs from `data/incoming-upstream.txt`, never the display prefixes in the table.

- [ ] **Step 5: Implement helpers and rerun tests**

```python
import json

def canonical_json_array(values):
    return json.dumps(sorted(set(values)), ensure_ascii=True, separators=(",", ":"))

def commit_to_group(commit: str) -> str:
    matches = [group for group, commits in HISTORICAL_GROUPS.items() if commit in commits]
    if len(matches) != 1:
        raise ValueError(f"expected one historical group for {commit}, got {matches}")
    return matches[0]
```

Expected: 3 schema tests pass.

### Task 2: Build A Deterministic Draft Run

**Files:**
- Create: `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/scripts/prepare_upstream_merge_run.py`
- Create: `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/scripts/test_prepare_upstream_merge_run.py`

**Interfaces:**
- Consumes: Task 1 schema and fixed audit inventories.
- Produces: `initialize_run(...)`, a draft matrix, manifest and analysis packets.

- [ ] **Step 1: Write synthetic-repository tests**

Create base, local product, approved docs and upstream commits. Assert initialization rejects any product delta between `audited_head` and `integration_base`, accepts exactly the approved spec/plan paths, preserves topology order and emits pending/not-executed states.

Run: `python3 -m unittest -v scripts/test_prepare_upstream_merge_run.py`

Expected: FAIL because the initializer does not exist.

- [ ] **Step 2: Implement strict Git helpers**

```python
def git(repository: Path, *arguments: str, allowed={0}):
    result = subprocess.run(["git", *arguments], cwd=repository, capture_output=True, check=False)
    if result.returncode not in allowed:
        stderr = result.stderr.decode("utf-8", errors="replace").strip()
        raise RuntimeError(f"git {' '.join(arguments)} failed ({result.returncode}): {stderr}")
    return result

def resolve(repository: Path, revision: str) -> str:
    return git(repository, "rev-parse", "--verify", f"{revision}^{{commit}}").stdout.decode().strip()

def canonical_incoming(repository: Path, merge_base: str, upstream: str) -> list[str]:
    raw = git(repository, "rev-list", "--reverse", "--topo-order", f"{merge_base}..{upstream}").stdout
    return [line for line in raw.decode().splitlines() if line]
```

- [ ] **Step 3: Enforce the exact documentation delta**

```python
APPROVED_DOCUMENT_PATHS = {
    "docs/superpowers/specs/2026-07-10-upstream-semantic-merge-remediation-design.md",
    "docs/superpowers/plans/2026-07-10-upstream-semantic-merge-preflight.md",
}

def require_documentation_only(repository, audited_head, integration_base):
    raw = git(repository, "diff", "--name-only", "-z", audited_head, integration_base).stdout
    paths = sorted(item.decode("utf-8", errors="surrogateescape") for item in raw.split(b"\0") if item)
    if set(paths) != APPROVED_DOCUMENT_PATHS:
        raise ValueError(f"unapproved integration-base paths: {paths}")
    return paths
```

- [ ] **Step 4: Assemble draft rows from existing TSVs**

Use `commits.tsv` for parent/time/subject, `commit-files.tsv` for paths/modules and local `all_local=1` rows indexed by path for overlap commits. Structured cells use canonical JSON. Semantic cells are empty only while `analysis_status=pending`; `decision_status=pending` and `post_merge_status=not_executed` are explicit.

- [ ] **Step 5: Write atomically and create the manifest**

The manifest records resolved refs, counts, approved document paths, Python/Git versions, source inventories, schema version `1` and SHA-256 for generated artifacts. Use a sibling temporary directory plus `os.replace`; refuse to overwrite a non-empty run except via `--refresh-digests`.

- [ ] **Step 6: Re-run initializer tests**

Expected: all ref, set, overlap and atomic-write tests pass.

### Task 3: Rehearse Text Conflicts Without A Merge Commit

**Files:**
- Create: `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/scripts/rehearse_upstream_merge.py`
- Create: `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/scripts/test_rehearse_upstream_merge.py`

**Interfaces:**
- Consumes: refs from manifest.
- Produces: `conflict-rehearsal.tsv` and raw merge-tree streams.

- [ ] **Step 1: Write clean/conflict parser tests**

Assert return code 0 means clean, 1 means conflicts, other codes raise; preserve raw streams byte-for-byte; conflict paths are unique and sorted.

- [ ] **Step 2: Run tests and verify failure**

Run: `python3 -m unittest -v scripts/test_rehearse_upstream_merge.py`

Expected: FAIL because the rehearsal module does not exist.

- [ ] **Step 3: Implement fixed-base merge-tree**

```python
import os
import subprocess

object_directory = run_root / "evidence" / "merge-tree-objects"
object_directory.mkdir(parents=True, exist_ok=True)
source_objects = subprocess.run(
    ["git", "rev-parse", "--git-path", "objects"],
    cwd=repository, check=True, capture_output=True, text=True,
).stdout.strip()
environment = {
    **os.environ,
    "GIT_OBJECT_DIRECTORY": str(object_directory),
    "GIT_ALTERNATE_OBJECT_DIRECTORIES": str((repository / source_objects).resolve()),
}
result = subprocess.run(
    ["git", "merge-tree", "--write-tree", "--messages", "--name-only",
     "--merge-base", merge_base, integration_base, upstream],
    cwd=repository, env=environment, capture_output=True, check=False,
)
if result.returncode not in {0, 1}:
    raise RuntimeError(result.stderr.decode("utf-8", errors="replace"))
```

Save stdout/stderr before parsing. The test compares the source object-directory file list before/after and requires no change. Do not invoke `git merge`, update refs or modify the worktree.

- [ ] **Step 4: Emit structured conflict rows**

Schema: `path`, `conflict_type`, `matrix_commits_json`, `local_contract_refs_json`, `predicted_treatment`, `owner`, `reviewer`, `evidence_path`, `status`. Initial status is `predicted`; cross-review must change every row to `reviewed`.

- [ ] **Step 5: Re-run tests**

Expected: clean/conflict synthetic tests pass.

### Task 4: Implement Digest-Bound Approval Events

**Files:**
- Create: `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/scripts/record_upstream_merge_approval.py`
- Create: `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/scripts/test_record_upstream_merge_approval.py`

**Interfaces:**
- Consumes: canonical matrix analysis digest, decision-file digest and exact user reply.
- Produces: append-only approval TSV and deterministic matrix decision states.

- [ ] **Step 1: Write stale-digest and append-only tests**

Require sequential `APP-0001`, `APP-0002`; reject stale digests and duplicate scope/digest approvals; record reopening without deleting prior events. Prove changing only approval/execution status leaves the analysis digest stable, while changing treatment/evidence changes it.

- [ ] **Step 2: Run tests and verify failure**

Expected: FAIL because the recorder does not exist.

- [ ] **Step 3: Implement the exact event contract**

```python
import hashlib
import json

APPROVAL_FIELDS = [
    "event_id", "created_at_utc", "matrix_sha256", "decision_sha256",
    "scope_type", "scope_id", "outcome", "approver", "source_ref", "reply_text",
]
OUTCOMES = {"approved", "rejected", "reopened"}
SCOPE_TYPES = {"observe_pre_hash", "semantic_group", "matrix_row"}

APPROVAL_MUTABLE_FIELDS = {
    "decision_status", "approval_event_id", "execution_commit",
    "post_merge_status", "survival_evidence_json",
}

def matrix_analysis_digest(rows) -> str:
    projection = [
        {field: row[field] for field in MATRIX_FIELDS if field not in APPROVAL_MUTABLE_FIELDS}
        for row in rows
    ]
    payload = json.dumps(projection, ensure_ascii=True, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(payload.encode("utf-8")).hexdigest()
```

`matrix_sha256` means the canonical analysis projection above, not the full TSV file hash. The manifest separately records the full file hash after status updates. Use UTC, `csv.DictWriter` with tab delimiters and atomic replacement. Store exact user text; never synthesize approval language.

- [ ] **Step 4: Update matching matrix rows**

Group approval changes all matching rows; row override changes one SHA. Any content edit after approval requires a reopen event before digest refresh.

- [ ] **Step 5: Re-run recorder tests**

Expected: append, stale-digest, reopen and atomic-write tests pass.

### Task 5: Implement The Three-Phase No-Merge Validator

**Files:**
- Create: `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/scripts/validate_upstream_merge_run.py`
- Create: `/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/scripts/test_validate_upstream_merge_run.py`

**Interfaces:**
- Consumes: repo, run root, parent audit and phase.
- Produces: JSON summary; only approved phase can return `merge_allowed=true`.

- [ ] **Step 1: Write negative and positive fixtures**

Cover duplicate/missing SHA, wrong header order, malformed JSON, invalid enum, stale digest, unreviewed conflict, blank semantic evidence, same high-risk owner/reviewer, missing observe decision and incomplete approvals.

- [ ] **Step 2: Run tests and verify failure**

Expected: FAIL because the validator does not exist.

- [ ] **Step 3: Implement common validation**

```python
assert resolved_refs == manifest["refs"]
assert documentation_delta == manifest["approved_document_paths"]
assert matrix_fields == MATRIX_FIELDS
assert matrix_commits == canonical_incoming(repository, merge_base, upstream)
assert len(matrix_rows) == 60
assert feature_commit_set == fixed_all_local_set
assert len(feature_commit_set) == 985
```

Parse every JSON cell as an array of strings and verify manifest hashes before statuses.

- [ ] **Step 4: Add analyzed-phase requirements**

Require nonblank intent/treatment/owner/reviewer, valid enums, nonempty focused tests/evidence, different high/critical reviewer, reviewed conflicts and `analysis_status=analyzed`.

- [ ] **Step 5: Add approved-phase requirements**

Require approved `OBSERVE-PREHASH-001`, all rows approved, events bound to the current canonical analysis digest and no latest rejected/reopened event. Only then print:

```json
{"local_features":985,"matrix_rows":60,"merge_allowed":true,"phase":"approved"}
```

- [ ] **Step 6: Re-run validator tests**

Expected: every invalid fixture reports one precise reason and the valid fixture passes.

### Task 6: Initialize And Verify The Real 60-Commit Run

**Files:**
- Create: generated run files listed above.
- Modify: external `task_plan.md`, `findings.md`, `progress.md`.

**Interfaces:**
- Consumes: Tasks 1-5 tools and fixed parent audit.
- Produces: a validated draft run ready for semantic review.

- [ ] **Step 1: Fetch and revalidate upstream**

```bash
cd /Users/ianshaw/Documents/code/personal/sub2api
git fetch upstream --prune
git status --short --branch
git rev-parse HEAD upstream/main
git merge-base HEAD upstream/main
git rev-list --left-right --count upstream/main...d0140cde50c92f8bd8eed52d5b50661b0fec980e
```

Expected for this run: clean checkout, upstream `e316ebf...`, merge base `12d811bd...`, audited product divergence `60 985`. If upstream differs, create a new run ID and regenerate the inventory before analysis.

- [ ] **Step 2: Run every audit tool test**

```bash
cd /Users/ianshaw/Documents/code/personal/changeLogs/sub2api
python3 -m unittest -v \
  scripts/test_generate_inventory.py \
  scripts/test_build_audit_artifacts.py \
  scripts/test_validate_deliverables.py \
  scripts/test_upstream_merge_schema.py \
  scripts/test_prepare_upstream_merge_run.py \
  scripts/test_rehearse_upstream_merge.py \
  scripts/test_record_upstream_merge_approval.py \
  scripts/test_validate_upstream_merge_run.py
```

Expected: all tests pass.

- [ ] **Step 3: Revalidate the parent audit**

```bash
cd /Users/ianshaw/Documents/code/personal/changeLogs/sub2api
python3 scripts/validate_deliverables.py \
  --root /Users/ianshaw/Documents/code/personal/changeLogs/sub2api \
  --head d0140cde50c92f8bd8eed52d5b50661b0fec980e \
  --upstream e316ebf52838a89d57fc790981cce7520f819ac8
```

Expected: 1,284 commits, 985 local features, 60 incoming commits, 59 merges and 17 migration conflicts.

- [ ] **Step 4: Initialize using the approved documentation HEAD**

```bash
cd /Users/ianshaw/Documents/code/personal/changeLogs/sub2api
python3 scripts/prepare_upstream_merge_run.py \
  --repo /Users/ianshaw/Documents/code/personal/sub2api \
  --audit-root /Users/ianshaw/Documents/code/personal/changeLogs/sub2api \
  --run-root /Users/ianshaw/Documents/code/personal/changeLogs/sub2api/upstream-merge-remediation/runs/2026-07-10-d0140cde-e316ebf5 \
  --audited-head d0140cde50c92f8bd8eed52d5b50661b0fec980e \
  --integration-base HEAD \
  --upstream e316ebf52838a89d57fc790981cce7520f819ac8 \
  --merge-base 12d811bd76572836d6df6e1fa8aa5ff91be3b12e
```

Expected: 60 draft rows, 20 groups and zero overwritten files.

- [ ] **Step 5: Rehearse and validate draft phase**

```bash
cd /Users/ianshaw/Documents/code/personal/changeLogs/sub2api
python3 scripts/rehearse_upstream_merge.py \
  --run-root /Users/ianshaw/Documents/code/personal/changeLogs/sub2api/upstream-merge-remediation/runs/2026-07-10-d0140cde-e316ebf5
python3 scripts/validate_upstream_merge_run.py \
  --phase draft \
  --repo /Users/ianshaw/Documents/code/personal/sub2api \
  --audit-root /Users/ianshaw/Documents/code/personal/changeLogs/sub2api \
  --run-root /Users/ianshaw/Documents/code/personal/changeLogs/sub2api/upstream-merge-remediation/runs/2026-07-10-d0140cde-e316ebf5
```

Expected: set integrity is 60/985 and every conflict path has a rehearsal row.

### Task 7: Decide `observe + pre-hash` Before Other Treatments

**Files:**
- Create: run `decisions/OBSERVE-PREHASH-001.md`.
- Update: run manifest digest.

**Interfaces:**
- Produces: an evidence-backed choice and exact truth table submitted before semantic-group approval.

- [ ] **Step 1: Capture introducing and hardening history**

```bash
cd /Users/ianshaw/Documents/code/personal/sub2api
git show --stat --oneline fff4a300c6ab0c0cc6fd394925f2811a0e17c882
git show --stat --oneline 12543cacd5c63ef6f3be6262569fb6f94e57cfc6
git log --all --format='%H%x09%aI%x09%s' -S'PreHashCheckEnabled' -- \
  backend/internal/service/content_moderation.go \
  backend/internal/service/content_moderation_test.go \
  frontend/src/views/admin/RiskControlView.vue \
  frontend/src/i18n/locales/en.ts frontend/src/i18n/locales/zh.ts
git blame -L 1088,1295 d0140cde50c92f8bd8eed52d5b50661b0fec980e -- \
  backend/internal/service/content_moderation.go
```

- [ ] **Step 2: Capture current copy, code and tests**

```bash
rg -n 'modeObserveDesc|preHashCheckHint' frontend/src/i18n/locales/en.ts frontend/src/i18n/locales/zh.ts
rg -n 'PreHash|HasFlaggedInputHash' backend/internal/service/content_moderation_test.go backend/internal/handler/openai_embeddings_test.go
cd backend
go test -tags=unit ./internal/service ./internal/handler \
  -run 'PreHash|ContentModeration.*Observe' -count=1
```

Expected evidence: observe copy promises pass-through, pre-hash copy promises blocking, code blocks before the observe branch, and no test covers observe+hit.

- [ ] **Step 3: Write all three candidate contracts**

1. `mode_precedence` (recommended): off never checks; observe never blocks; pre_block may block. Observe+hit records a nonblocking hash observation, sends no email, increments no ban count and skips duplicate upstream moderation.
2. `pre_hash_precedence`: observe+hit blocks; UI warns that pre-hash overrides observe; existing runtime behavior remains.
3. `pre_block_only_toggle`: backend rejects or normalizes the toggle outside pre_block; UI disables it outside that mode; existing observe configs require migration.

For each fill every `enabled × mode × toggle × hit/miss/cache_error` cell with lookup, allow/block, sync/async, persisted action, email, ban count, metrics and client status.

- [ ] **Step 4: Request and record the exact user choice**

Do not infer this approval from design approval. After receiving a direct choice, run the recorder with the exact reply as a single safely quoted argv value:

```bash
cd /Users/ianshaw/Documents/code/personal/changeLogs/sub2api
test -n "$APPROVAL_REPLY"
python3 scripts/record_upstream_merge_approval.py \
  --run-root /Users/ianshaw/Documents/code/personal/changeLogs/sub2api/upstream-merge-remediation/runs/2026-07-10-d0140cde-e316ebf5 \
  --scope-type observe_pre_hash \
  --scope-id OBSERVE-PREHASH-001 \
  --outcome approved \
  --approver user \
  --source-ref codex-thread-019f4c89-7a08-7d53-b35d-6b4faec462fa \
  --reply-text "$APPROVAL_REPLY"
```

Before this command, set `APPROVAL_REPLY` to the exact received message and verify `test -n "$APPROVAL_REPLY"`; the executor may not invent or normalize it.

### Task 8: Analyze The Gateway And Protocol Lane (36 Commits)

**Files:**
- Create: run `work/gateway-upstream-matrix.tsv`.
- Create: one run `evidence/commits/{commit_sha}.patch` for each assigned row.

**Interfaces:**
- Owns: `UG-01`, `UG-02`, `UG-03`, `UG-05`, `UG-06`, `UG-12`, `UG-13`, `UG-14`, `UG-16`, `UG-19`, `UG-20`.
- Owner: `gateway_routing_review`; reviewer: platform for billing and frontend/ops for client-facing rows.

- [ ] **Step 1: Export all 36 parent-relative patches**

Export deterministically from the lane commit list:

```bash
cd /Users/ianshaw/Documents/code/personal/sub2api
RUN_ROOT=/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/upstream-merge-remediation/runs/2026-07-10-d0140cde-e316ebf5
while IFS= read -r commit_sha; do
  git show --remerge-diff --format=fuller --stat --patch "$commit_sha" \
    > "$RUN_ROOT/evidence/commits/$commit_sha.patch"
done < "$RUN_ROOT/work/gateway-commits.txt"
```

For merge commits record every parent and whether the child behavior already appears in earlier rows.

- [ ] **Step 2: Complete all 32 columns**

Every row states upstream before/after behavior, dependencies, local contracts at risk, HTTP/passthrough/WS/compact/Messages/raw-Chat impact, usage/billing/log impact, treatment and exact focused test command. A merge row cannot be `no_impact` solely because its child commits are separately listed.

- [ ] **Step 3: Run read-only focused evidence tests**

```bash
cd /Users/ianshaw/Documents/code/personal/sub2api/backend
go test -tags=unit -count=1 \
  ./internal/pkg/apicompat ./internal/pkg/openai ./internal/service \
  -run 'Compact|Cache|Usage|Billing|Reasoning|Tool|Originator|Image|Grok|WebSocket'
```

Record current failures honestly; existing tests do not prove incoming tests.

- [ ] **Step 4: Review high-risk rows**

Reviewer compares patch, local behavior and proposed tests. Missing transport/billing boundaries return to `needs_evidence`; complete rows become analyzed/pending.

### Task 9: Analyze The Backend Platform Lane (15 Commits)

**Files:**
- Create: run `work/platform-upstream-matrix.tsv`.

**Interfaces:**
- Owns: `UG-07`, `UG-09`, `UG-11`, `UG-18`.
- Owner: `backend_platform_review`; reviewer: gateway for usage paths and frontend/ops for admin contracts.

- [ ] **Step 1: Export and inspect all 15 patches**

Trace payment recovery/idempotency, GPT-5.6 cache-write pricing, Windows WS reset, repository lock reconciliation, setup-token lifecycle and ops capture contracts.

```bash
cd /Users/ianshaw/Documents/code/personal/sub2api
RUN_ROOT=/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/upstream-merge-remediation/runs/2026-07-10-d0140cde-e316ebf5
while IFS= read -r commit_sha; do
  git show --remerge-diff --format=fuller --stat --patch "$commit_sha" \
    > "$RUN_ROOT/evidence/commits/$commit_sha.patch"
done < "$RUN_ROOT/work/platform-commits.txt"
```

- [ ] **Step 2: Complete data/concurrency fields**

Each row states transaction/lock order, presence semantics, migration/config/generated impact, interface/test-double impact and standard/simple mode behavior.

- [ ] **Step 3: Run focused evidence tests**

```bash
cd /Users/ianshaw/Documents/code/personal/sub2api/backend
go test -tags=unit -count=1 ./internal/repository ./internal/service ./internal/handler \
  -run 'Payment|Billing|Usage|SetupToken|TokenRefresh|OpsCapture|ExpiredLock|WebSocketReset'
```

- [ ] **Step 4: Review transactional claims**

Claims based only on handler output remain `needs_evidence`; repository/service ordering and tests are required.

### Task 10: Analyze The Frontend And Operations Lane (9 Commits)

**Files:**
- Create: run `work/frontend-ops-upstream-matrix.tsv`.

**Interfaces:**
- Owns: `UG-04`, `UG-08`, `UG-10`, `UG-15`, `UG-17`.
- Owner: `frontend_ops_review`; reviewer: `backend_platform_review`.

- [ ] **Step 1: Export and inspect all 9 patches**

Trace locale keys, backend/frontend request-type pairing, version files, user-scoped Fast/Flex API/store/UI wiring, simple/standard mode and old-setting defaults.

```bash
cd /Users/ianshaw/Documents/code/personal/sub2api
RUN_ROOT=/Users/ianshaw/Documents/code/personal/changeLogs/sub2api/upstream-merge-remediation/runs/2026-07-10-d0140cde-e316ebf5
while IFS= read -r commit_sha; do
  git show --remerge-diff --format=fuller --stat --patch "$commit_sha" \
    > "$RUN_ROOT/evidence/commits/$commit_sha.patch"
done < "$RUN_ROOT/work/frontend-ops-commits.txt"
```

- [ ] **Step 2: Complete frontend/backend contract pairs**

Record API type, serialization, store/config migration, component mount path, i18n keys and focused Vitest. Preserve local lazy/tabbed SettingsView and route/store behavior.

- [ ] **Step 3: Run focused evidence tests with pinned pnpm**

```bash
cd /Users/ianshaw/Documents/code/personal/sub2api
corepack pnpm@9.15.9 --dir frontend exec vitest run \
  src/views/admin/__tests__/SettingsView.spec.ts \
  src/views/admin/__tests__/UsageView.spec.ts \
  src/stores/__tests__/app.spec.ts
```

Record baseline failures; do not repair them during preflight.

- [ ] **Step 4: Review implementation/test pairing**

Every test/SFC/API/store/i18n dependency must appear. Rows that recreate a test/implementation split remain `needs_evidence`.

### Task 11: Consolidate Lanes And Cross-Review

**Files:**
- Modify: run matrix, rendered matrix, groups summary and conflict rehearsal.

**Interfaces:**
- Consumes: three disjoint lane files totaling 60 rows.
- Produces: one analyzed matrix and approval summary.

- [ ] **Step 1: Prove lane set equality**

Lane union equals canonical 60, pairwise intersections are empty, headers match and all source rows are analyzed.

- [ ] **Step 2: Merge rows in topology order**

Use draft `sequence`; reject metadata changes to SHA, parents, time, subject or paths unless a new snapshot is initialized.

- [ ] **Step 3: Reconcile cross-lane dependencies**

Review cache creation across `UG-09/UG-13/UG-19`, effort across `UG-03/UG-06/UG-14`, tools across `UG-05/UG-20`, and UI/backend pairs across `UG-08/UG-15`. Every merge-tree path cites affected rows and treatment.

- [ ] **Step 4: Render approval summaries**

Each group shows full SHAs, intent, conflicts, local risks, treatment, tests, owner/reviewer and residual risk, with row appendix.

- [ ] **Step 5: Refresh digests and validate analyzed phase**

```bash
cd /Users/ianshaw/Documents/code/personal/changeLogs/sub2api
python3 scripts/prepare_upstream_merge_run.py --refresh-digests \
  --run-root /Users/ianshaw/Documents/code/personal/changeLogs/sub2api/upstream-merge-remediation/runs/2026-07-10-d0140cde-e316ebf5
python3 scripts/validate_upstream_merge_run.py \
  --phase analyzed \
  --repo /Users/ianshaw/Documents/code/personal/sub2api \
  --audit-root /Users/ianshaw/Documents/code/personal/changeLogs/sub2api \
  --run-root /Users/ianshaw/Documents/code/personal/changeLogs/sub2api/upstream-merge-remediation/runs/2026-07-10-d0140cde-e316ebf5
```

Expected: 60 analyzed rows, 985 fixed features, all conflicts reviewed and no treatment approval claimed yet.

### Task 12: Obtain And Record Every Treatment Approval

**Files:**
- Modify: run approval events, matrix decision fields and manifest.

**Interfaces:**
- Consumes: analyzed matrix and approved observe/pre-hash event.
- Produces: approved validator success or explicit blocked evidence.

- [ ] **Step 1: Present groups in dependency order**

Order: identity/image, cache billing, effort, MCP/tools, parallel calls, client/WS/compact, user settings/request types, payment/setup-token/ops/version. Include every row; no blanket approval request without treatment tables.

- [ ] **Step 2: Record every exact reply**

Use the recorder for each approved/rejected/reopened scope. Row changes require a row event, summary/digest refresh and affected group reapproval.

```bash
cd /Users/ianshaw/Documents/code/personal/changeLogs/sub2api
test -n "$GROUP_ID"
test -n "$APPROVAL_REPLY"
python3 scripts/record_upstream_merge_approval.py \
  --run-root /Users/ianshaw/Documents/code/personal/changeLogs/sub2api/upstream-merge-remediation/runs/2026-07-10-d0140cde-e316ebf5 \
  --scope-type semantic_group \
  --scope-id "$GROUP_ID" \
  --outcome approved \
  --approver user \
  --source-ref codex-thread-019f4c89-7a08-7d53-b35d-6b4faec462fa \
  --reply-text "$APPROVAL_REPLY"
```

- [ ] **Step 3: Validate after each approval batch**

Before the last approval expect nonzero exit naming remaining scopes. After all approvals expect:

```json
{"local_features":985,"matrix_rows":60,"merge_allowed":true,"phase":"approved"}
```

- [ ] **Step 4: Freeze the approved run**

Record final matrix/decision digests, event count, refs and validator JSON in `progress.md`. Later edits require reopening.

### Task 13: Write The Decision-Specific Merge And Remediation Plan

**Files:**
- Create later: `docs/superpowers/plans/2026-07-10-upstream-semantic-merge-remediation-execution.md`.

**Interfaces:**
- Consumes: approved matrix, truth table, conflicts and known-issue links.
- Produces: exact worktree, merge resolution, TDD remediation, verification and three-review tasks.

- [ ] **Step 1: Verify No-Merge Gate without merging**

Run approved validator on unchanged refs/hashes. Any mismatch returns to Task 11 or 12.

- [ ] **Step 2: Invoke `superpowers:writing-plans` with approved treatments**

The next plan names exact conflict paths, product/test files, resolution records, upstream-resolved versus remaining findings, generation commands, full tests/builds and three independent review restart rules.

- [ ] **Step 3: Commit only the next plan and request review**

No worktree or merge is created until the decision-specific plan is written, self-reviewed and explicitly approved.

## Specification Coverage

| Design requirement | Implemented by this plan |
|---|---|
| Versioned artifacts and manifest digests | Tasks 1-6 |
| Exact 60-row/32-column matrix and 985-set preservation | Tasks 1, 2, 5, 6, 11 |
| Five treatment choices and per-row evidence | Tasks 1, 8-11 |
| Semantic grouping and digest-bound approvals | Tasks 4, 11, 12 |
| `observe + pre-hash` first-decision gate | Task 7 |
| Deterministic No-Merge Gate | Tasks 3, 5, 12 |
| Owner/reviewer separation | Tasks 8-11 |
| Ref drift, error and reopen handling | Tasks 2, 4-6, 12 |
| Isolated ancestry-preserving merge | Deferred to Task 13's decision-specific plan because treatment inputs are not yet approved |
| Known-issue remediation and TDD | Deferred to Task 13's plan after upstream-resolved findings are known |
| Full tests/builds and three review rounds | Required content of Task 13's plan; no product validation is claimed by preflight |

## Plan Completion Evidence

- Schema/tool tests pass.
- Parent audit still reports 985 local features, 60 incoming commits, 59 merges and 17 migration conflicts.
- Draft and analyzed validators pass at their phases.
- `OBSERVE-PREHASH-001` has an explicit approved truth table.
- All 60 rows are analyzed with independent high-risk review.
- Every treatment has a current-digest approval event.
- Approved validator returns `merge_allowed=true`.
- No product source, worktree or merge commit was created.
- The decision-specific merge/remediation plan exists for separate approval.
