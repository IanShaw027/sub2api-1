## Purpose

定义前端国际化资源的组织、按需加载、构建期预编译、无引用键清理与 zh/en 一致性的契约，使语言资源不再成为首屏与构建的负担，同时保证任何界面不出现缺失或原始键名。

## ADDED Requirements

### Requirement: 语言资源按导航域拆分并按路由按需加载
系统 SHALL 把语言资源划分为 `core`（common / nav / errors / auth / dates / table / version）、`landing`、`user`、`admin-core`、`admin-settings`、`admin-ops`、`admin-channels` 七个域；每条路由 MUST 通过路由元信息声明所需域（`core` 隐含）；路由解析前 MUST 完成所需域的加载；首屏（`/home`、`/login`）加载的语言资源 MUST ≤ 120KB 原始体积。

#### Scenario: 访问登录页
- **WHEN** 首次打开 `/login`
- **THEN** 只加载 `core` 与 `landing`（若登录页复用其文案）域的语言块，网络面板中 MUST 不出现 `admin-*` 或 `user` 域语言块

#### Scenario: 从用户页跳转到管理员页
- **WHEN** 已加载 `user` 域的会话导航到 `/admin/accounts`
- **THEN** 系统 MUST 在路由解析阶段加载 `admin-core` 域并等待完成后再渲染，页面 MUST 不出现原始键名闪现

#### Scenario: 切换语言
- **WHEN** 用户在 zh 下已加载 `core + user`，切换到 en
- **THEN** 系统 MUST 只加载 en 的 `core + user` 域，切换后当前页面文案全部为 en，`document.documentElement.lang` 为 `en`

### Requirement: 消息在构建期预编译
语言资源 MUST 在构建期编译为消息函数，运行时 MUST NOT 包含消息编译器；CSP 环境（无 `unsafe-eval`）下所有插值、复数与富文本消息 MUST 正常工作。

#### Scenario: CSP 严格模式渲染
- **WHEN** 在 `Content-Security-Policy: script-src 'self'` 下打开任意页面
- **THEN** 控制台 MUST 无 `unsafe-eval` 违规，带插值的文案（如"显示 {from}–{to}，共 {total} 条"）MUST 正确渲染

#### Scenario: 产物检查
- **WHEN** 检查构建产物
- **THEN** MUST 不存在 `@intlify/message-compiler` 的运行时代码，`vendor-i18n` 块 MUST 小于 40KB 原始

### Requirement: 无引用键为零且动态键有白名单
系统 SHALL 提供 `scripts/i18n-unused.mjs`，识别 `t / $t / te / tm / i18n.global.t` 的字面量键与模板字符串前缀；无法静态解析的动态键 MUST 登记在 `scripts/i18n-dynamic-keys.json`（键前缀 + 说明）；脚本报告的无引用键数 MUST 为 0。

#### Scenario: 基线
- **WHEN** 在 2026-09-03 基线上运行脚本
- **THEN** 报告约 1,193 个无引用键（`admin.*` 826、`payment.*` 110、`tickets.*` 49 …），作为清理清单

#### Scenario: 新增动态键
- **WHEN** 开发者写入 `` t(`admin.accounts.platforms.${platform}`) `` 而未登记
- **THEN** 脚本 MUST 报告该前缀下的键为"疑似动态引用"，并要求登记后才通过

### Requirement: zh 与 en 键集合一致且无重复键
`scripts/i18n-diff.mjs` MUST 输出 zh-only 与 en-only 键数均为 0；每个语言文件 MUST 无重复对象键（`vue-tsc` TS1117 为 0）；新增键 MUST 同时写入两种语言。

#### Scenario: 只加了 zh
- **WHEN** 提交只在 zh 中新增 `keys.newLabel`
- **THEN** 门禁 MUST 失败并列出 `en-only: 0, zh-only: 1 (keys.newLabel)`

### Requirement: 界面不得出现原始键名
任何路由在任何语言下的渲染文本 MUST 不匹配 `^[a-z][A-Za-z0-9]*(\.[A-Za-z0-9_$-]+)+$`；缺失键 MUST 回退到 en，且开发模式下 MUST 在控制台告警。

#### Scenario: 截图文本抓取
- **WHEN** 对全部路由执行 `scripts/keyname-leak.mjs`
- **THEN** 命中数 MUST 为 0
