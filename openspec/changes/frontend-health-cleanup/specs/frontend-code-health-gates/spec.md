## Purpose

定义前端代码健康度的持续度量与门禁：哪些指标必须被脚本化、阈值是什么、在 CI 与本地如何执行，使清理成果不回退并让技术债可见。

## ADDED Requirements

### Requirement: 统一的健康检查入口
系统 SHALL 提供 `npm run health`，依次执行 `dead-files`、`deps-check`、`i18n-diff`、`i18n-unused`、`any-count`、`file-size`、`bundle-report`（需先构建），输出一张"指标 / 基线 / 当前 / 阈值 / 状态"表；任一超阈值 MUST 以非零退出。

#### Scenario: 本地执行
- **WHEN** 运行 `npm run health`
- **THEN** 终端 MUST 输出 7 行指标表，并在末尾给出 PASS / FAIL

### Requirement: 指标阈值
阈值 MUST 为：不可达文件 0；未使用依赖 0；zh/en 差集 0；无引用 i18n 键 0；`: any` / `as any` 总数 ≤ 150（基线 357，按季度收紧）；> 1,500 行文件 0（过渡期白名单逐步清空）；chunk 预算见 `frontend-build-performance`；`legacy` 标记每处附原因；`console.log` 0；`eslint-disable` ≤ 5 且每处附原因。

#### Scenario: any 增长
- **WHEN** PR 使 `any` 总数从 150 增至 160
- **THEN** `any-count` MUST 失败并列出新增位置

### Requirement: CI 报告差值
CI MUST 在每个 PR 上以评论或检查摘要形式输出 `health` 表与 `bundle-report` 的"基线 → 当前"差值；差值文件 `scripts/health-baseline.json` MUST 随每次达标提交更新。

#### Scenario: PR 检查
- **WHEN** 打开一个改动了 `views/admin/UsersView.vue` 的 PR
- **THEN** 检查摘要 MUST 显示 `UsersView` 块大小变化与 `any` 数变化

### Requirement: 大文件与孤儿文件的过渡白名单
`scripts/health-allowlist.json` MUST 列出暂未达标的文件及其对应任务编号；白名单 MUST 只减不增；CI MUST 在白名单条目变为达标时提示移除。

#### Scenario: 拆分完成
- **WHEN** `SettingsView.vue` 降到 1,200 行
- **THEN** `file-size` MUST 提示白名单中该条目可移除，PR 若未移除则告警
