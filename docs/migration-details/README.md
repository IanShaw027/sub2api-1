# personal-dev 迁移明细：文件级取舍决策清单

> 生成时间：2026-08-13。基准：`git diff upstream/main...personal-dev --numstat`（merge-base = upstream v0.1.175 `5935e674a`，personal-dev 存档 tag `archive/personal-dev-20260813`）。
> 上层规划见 `../../PERSONAL_MIGRATION_INVENTORY.md`（批次与风险），流程见 `../../PERSONAL_FORK_WORKFLOW.md` 第五节。
> 全部 2894 个差异文件已逐一列出，每份文档末尾附完整性核对结论（与 numstat 双向 diff 零遗漏零多余）。

## 四份明细文档

| 文档 | 范围 | 文件数 | 模块数 |
|---|---|---|---|
| [backend-service.md](./backend-service.md) | `backend/internal/service/` | 980 | 23 |
| [backend-data.md](./backend-data.md) | `backend/` 其余（ent / handler / repository / pkg / server / migrations / 装配依赖） | 1151 | 33 |
| [frontend.md](./frontend.md) | `frontend/` | 609 | 14 |
| [infra.md](./infra.md) | 根级 / .github / deploy / docs / tools / .review | 154 | 10 |

## 重要度汇总

| 领域 | 文件数 | P0 核心 | P1 重要 | P2 可选 | P3 建议放弃 | 待定 |
|---|---|---|---|---|---|---|
| A 后端 Service 层 | 980 | 86 | 532 | 361 | 1 | — |
| B 后端数据/接入层 | 1151 | 547 | 545 | 34 | 25 | — |
| C 前端 | 609 | 59 | 423 | 123 | 4 | — |
| D 基础设施 | 154 | 10 | 32 | 14 | 83 | 15 |
| **合计** | **2894** | **702** | **1532** | **532** | **113** | **15** |

说明：

- **P0 核心**：缺了核心工作流不可用或产生安全回退。B 领域的 P0 偏高是因为 ent 生成代码继承了对应 schema 的重要度——实际决策点只在 34 个 `ent/schema/*.go`，生成代码随 `go generate ./ent` 重建，不手工迁移。
- **P1 重要**：明显的功能或稳定性增强；**P2 可选**：便利性、调试工具、纯测试补充；**P3 建议放弃**：一次性产物、被上游取代或过时（D 领域的 83 个 P3 主要是 `.review` 历史审查快照和一次性联调脚本）。
- **待定（15 个）**：需要你拍板的文档类条目（superpowers 设计稿、CHANGELOG、README 系列等），每条已附建议，见 infra.md 第 9 节。

## 使用方式

1. 从 P3 开始过一遍（最快）：确认放弃项，把 `☐` 改成 `✗ 放弃`。
2. P0 基本都是必迁项，快速确认后改 `✓ 保留`。
3. 决策重心放在 P1/P2：结合"这个功能我现在还用不用"逐模块过，不确定的先留 `☐`。
4. 每个模块小节头部有建议批次和迁移方式（整体搬运 / 增量 patch / cherry-pick），与 `PERSONAL_MIGRATION_INVENTORY.md` 的 10 个批次对应。
5. 迁移某批前重读该模块小节的耦合提示，特别是脊柱文件（`gateway_service.go`、`openai_gateway_service.go` 等 13 个共享文件，见清单"共享文件缠绕矩阵"）。
