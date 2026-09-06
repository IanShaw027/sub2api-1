# Functional Preservation Baseline

## Revision choice

- **Review target:** `c4efeec16` (`docs(creation): record workspace contracts and glass migration tools`).
- **Pre-Glass reference:** `f1c8ab7da6284179461613cec8a30a8f87f2c545`, the parent of the first Glass shell/token commit `cddbd355e`.
- `65529a248` and `f06a13b24^` are not trustworthy pre-redesign baselines: both already contain part of the Glass work.

## Reproducible command

```bash
cd frontend
node scripts/preservation-audit.mjs --base pre-glass --target worktree --out /tmp/preservation-audit.json
```

The report is evidence for review and intentionally has `status: manual-review-required` even when sets match. It does not prove runtime reachability, permissions, responsive rendering, API success, or equivalent behavior.

## Inventory schema

The audit extracts `routes`, `navigation`, `columns`, `anchors`, `actions`, `models`, `apiCalls`, `visibility`, and `imports` with source file and line evidence. Vue templates use `vue/compiler-sfc`; TypeScript uses the TypeScript compiler. Historical revisions are read through one `git archive` snapshot.
