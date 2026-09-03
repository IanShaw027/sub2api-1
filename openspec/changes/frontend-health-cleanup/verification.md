# 验证记录

## 目标阈值

| 指标 | 基线（2026-09-03） | 目标 | 脚本 |
|---|---|---|---|
| 入口不可达文件 | 25 | 0 | `dead-files` |
| 未使用依赖 | 1（`@lobehub/icons`） | 0 | `deps-check` |
| zh/en 差集 | 5 / 5 | 0 / 0 | `i18n-diff` |
| 无引用 i18n 键 | 1,193 | 0（白名单前缀除外） | `i18n-unused` |
| 首屏语言块 | ~420KB 原始 | ≤ 120KB | `bundle-report` |
| 首屏 JS 合计 | ~1.1MB 原始 | ≤ 700KB 原始 / 250KB gzip | `bundle-report` |
| 最大路由块 | 836KB（AccountsView） | ≤ 300KB | `bundle-report` |
| 最大 vendor 块 | 431KB（vendor-ui） | ≤ 250KB | `bundle-report` |
| chunk 数 | 225 | ≤ 180 | `bundle-report` |
| `vite build` | 24.9s | ≤ 20s | 计时 |
| `build` 脚本总时长 | ~63s（串行 tsc） | ≤ 20s（tsc 并行） | 计时 |
| `vitest run` | 60.3s | ≤ 45s | 计时 |
| `vendor-i18n` | 63KB | < 40KB | `bundle-report` |
| `> 1,500` 行文件 | 21 | 0 | `file-size` |
| `any` | 357 | ≤ 150 | `any-count` |
| `legacy` 无原因标记 | 132 | 0 | grep |
| `console.log` | 1 | 0 | grep |

## 执行记录

| 日期 | 任务组 | health | bundle | typecheck | test | 提交 |
|---|---|---|---|---|---|---|
| | | | | | | |
