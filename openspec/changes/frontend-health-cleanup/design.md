## Context

见 `proposal.md - Why` 与 `baseline.md`。设计需要知道的事实：

- i18n 由 `src/i18n/index.ts` 用 `import('./locales/zh')` / `import('./locales/en')` 整包懒加载；`locales/{zh,en}/index.ts` 把 8 个模块 spread 成一个对象，`admin/index.ts` 再 spread 9 个子模块。Vite 别名把 `vue-i18n` 指向 runtime 版并开启 JIT 编译以规避 CSP `unsafe-eval`；没有构建期预编译插件。产物里语言块以 `index-*.js` 命名（zh 419KB / en 437KB 原始）。
- `vite.config.ts` 的 `manualChunks` 只按 node_modules 分 5 个 vendor 块；`xlsx`（7.3MB）与 `@vueuse/core` 同在 `vendor-ui`（431KB），首屏 `index.html` 直接引用 `vendor-misc`（292KB）。应用代码按路由自动分块，但大视图静态导入了自己的全部弹层。
- `package.json` 的 `build` 为 `vue-tsc -b && vite build`，串行；`vite-plugin-checker` 在 dev 与 build 都跑 vue-tsc。
- 入口可达性分析（`scripts/refs.py` 原型）：931 个文件（含 307 个 spec），非测试 622 个，可达 597，不可达 25（3.7k 行）；其中 14 个无人引用、6 个仅测试引用、5 个被其它不可达文件引用。
- 重复组件引用计数：`BaseDialog` 82 / `UiModal` 7；`common/Select` 61 / `UiSelect` 4；`common/Pagination` 31 / `UiPagination` 6；`common/Toggle` 15 / `ToggleSwitch` 3；`common/StatusBadge` 0 / `ui/StatusBadge` 14；`common/StatCard` 1 / `ui/StatCard` 5；`common/Skeleton` 0。
- `glass-ui-redesign` 正在同一分支上进行，其组件层以 `components/ui/*` 为准。

## Goals / Non-Goals

**Goals:**

- 首屏语言资源 ≤ 120KB 原始（当前 ~420KB）；无引用键为 0；消息构建期预编译。
- `vite build` ≤ 20s、`vue-tsc` 与构建并行；最大路由块 ≤ 300KB 原始；首屏 JS（入口 + vendor + 语言 + 布局）≤ 700KB 原始 / 250KB gzip。
- 入口不可达源文件为 0；`components/common` 与 `components/ui` 同职责组件只剩一个实现；未使用依赖为 0。
- 门禁脚本可在 CI 与本地一键执行，并输出与基线的差值。

**Non-Goals:**

- 不做 Tailwind → 其它方案迁移；不换构建工具；不改后端。
- 不在本变更内完成 Glass 视觉改造（属于 `glass-ui-redesign`）。
- 不重写业务逻辑；拆分大文件只做机械抽取，不改行为。

## Decisions

1. **语言包按"导航域"拆分，而不是按页面。**
   域：`core`（common / nav / errors / auth / dates / table，首屏必需）、`landing`、`user`（dashboard / keys / usage / orders / tickets / profile / payment / redeem / affiliate / subscriptions / availableChannels / channelStatus / studio / batchImage）、`admin-core`（overview / resources / users / groups / accounts）、`admin-settings`、`admin-ops`（ops / audit / promptAudit / riskControl）、`admin-channels`（channels / plugins / channelMonitor）。路由 `meta.i18n: ['user']` 声明所需域，路由守卫在 `beforeResolve` 里 `await loadNamespaces(locale, domains)`，未加载完成前不渲染页面（避免键名闪现）。
   备选：按页面 60+ 个文件——请求碎片化、维护成本高；整包不拆——无法达标。

2. **构建期预编译消息（`@intlify/unplugin-vue-i18n`，`strictMessage: false`，`escapeHtml: false`），保留 runtime 版 vue-i18n。**
   现状为 JIT 编译以满足 CSP；预编译后运行时不再需要编译器，CSP 同样满足且更快。onboarding 使用的 HTML 富文本消息继续允许（`warnHtmlMessage: false`）。
   备选：保持 JIT——每次切换语言都在浏览器编译 8.8k 条消息。

3. **无引用键删除以静态分析 + 白名单为准。**
   `scripts/i18n-unused.mjs` 识别 `t('a.b')`、`$t`、`te`、`tm`、`i18n.global.t`，以及模板字符串前缀 `` t(`a.b.${x}`) `` → 保留 `a.b.*`；无法静态解析的动态键必须写进 `scripts/i18n-dynamic-keys.json` 白名单（带解释）。删除分两轮：先删 `admin.dataManagement`、`payment.errors`、`tickets.errors` 等整块无引用命名空间（经人工确认），再删零散键。
   备选：运行时收集使用键——覆盖不全，否决。

4. **组件收敛按"实现在哪就留在哪，只保留一份逻辑"处理，分三类。**
   (a) 逻辑重复、API 不兼容：`common/BaseDialog.vue`（props `show / width: narrow|normal|wide|extra-wide|full / closeOnClickOutside / showCloseButton`，自带焦点管理）vs `ui/UiModal.vue`（`open / width: sm|md|lg|xl / closeOnOverlay / showClose`，用 `ui/focusTrap.ts`）——`BaseDialog` 改为 `UiModal` 的薄包装做 prop 映射，82 处引用无需同时改，逐页迁移后删除包装；`common/Toggle.vue`（仅 `modelValue`）→ `ui/ToggleSwitch.vue`（超集）直接替换 15 处。
   (b) 已是外观包装、无逻辑重复：`ui/UiSelect.vue` 只是 `common/Select.vue`（766 行）的样式外壳，`ui/UiPagination.vue` 同理——把 Glass 样式并入 `common/Select.vue` / `Pagination.vue` 本体，`ui/*` 版本改为纯 re-export 并补齐缺失的 props 透传（`clearable / creatable / remote / variant=pill / valueKey / labelKey`），不迁移调用方。
   (c) 无引用：`common/StatusBadge`、`common/Skeleton`、`common/StatCard`（API 与 `ui/StatCard` 完全不同）直接删。
   审计还确认 `OpsErrorDetailModal`（单条详情）与 `OpsErrorDetailsModal`（可筛选列表）是协作的列表 / 详情对，不是重复实现——只做重命名（`OpsErrorListModal`）消歧，不合并。
   备选：脚本一次性替换 `BaseDialog`——宽度枚举与关闭语义不同，风险高。

5. **超大文件按"平台 / tab"机械抽取为懒加载子组件。**
   `CreateAccountModal` / `EditAccountModal` 按平台面板（Anthropic / OpenAI / Gemini / Antigravity / Grok / Kiro / Ollama …）抽成 `components/account/platform/*.vue`，主弹层用 `defineAsyncComponent`；`SettingsView` 按 tab 抽成 `components/admin/settings/tabs/*.vue`（与 `glass-ui-redesign` 任务 10 合并执行，避免两次重排）；`GroupsView` 的弹层同法。抽取只移动模板与对应 script 片段，props 用 `defineModel` 直通。
   备选：保持大文件——`AccountsView` 块 836KB 无法降下来。

6. **重依赖按需动态导入并单独分块。**
   `xlsx`（导出）、`chart.js` + `vue-chartjs`（仪表盘）、`driver.js`（引导）、`qrcode`、`marked` + `dompurify`（公告 / 法律文档）在使用点 `await import()`；`manualChunks` 把它们各自成块并从 `vendor-ui` / `vendor-misc` 移出；`index.html` 首屏只保留 `vendor-vue` + `vendor-i18n` + 入口。
   备选：全部留在 vendor——首屏多 400KB。

7. **类型检查与构建并行；checker 插件只在 dev 启用。**
   `build` 改为 `vite build`，`typecheck` 独立；CI job 矩阵并行跑 `typecheck`、`lint`、`test`、`build`。`vite-plugin-checker` 仅 `mode === 'development'` 注册（当前 build 也在跑，造成双重 vue-tsc）。
   备选：保持串行——每次构建多 38s。

8. **健康度量固化为脚本并在 CI 报告差值。**
   `dead-files`（入口可达性）、`deps-check`（knip 或自研）、`i18n-unused`、`i18n-diff`、`any-count`、`file-size`（> 1500 行告警、> 3000 行失败，白名单逐步收紧）、`bundle-report`（`rollup-plugin-visualizer` JSON + 预算表）。PR 评论输出"基线 → 当前"。

9. **执行顺序与 `glass-ui-redesign` 的关系。**
   先做本变更的 0–3 组（基线脚本、依赖与死文件、i18n 拆分、构建并行）——这些不碰视图模板，可与 Glass 组 0–4 并行；组件收敛（4 组）与大文件拆分（5 组）与 Glass 的页面任务合并到同一批代理执行，避免同一文件被两个变更先后重写。

## Risks / Trade-offs

- [静态分析漏判动态拼接的 i18n 键，删错文案] → 白名单机制 + 删除前用截图矩阵 + `keyname-leak` 门禁全量跑一遍；分两轮删除；每轮单独提交便于回滚。
- [按域懒加载导致切换路由时闪现键名] → 路由守卫等待命名空间加载；`t` 缺键时回退 `en`；`keyname-leak` 门禁。
- [组件包装层引入细微行为差异（焦点、Esc、动画）] → 包装层保留原 spec 全部断言；先包装再迁移。
- [大文件拆分引发响应式引用丢失] → 只用 `defineModel` / props 直通，拆分后跑该视图全部 spec 与 `anchor-diff`。
- [删除仅测试引用的文件会丢失测试覆盖的行为] → 6 个文件逐一判定：`OpenAIOAuthCapacityDialog`（587 行，带 API）疑似未接线功能，需用户确认保留或删除；其余按 tasks 处理。
- [并行类型检查后，构建不再拦截类型错误] → CI 把 `typecheck` 设为必需检查；本地 `pre-push` 钩子可选。

## Migration Plan

1. 组 0：固化基线脚本与数字（不改产品代码）。提交。
2. 组 1：删除无引用依赖与不可达文件、死测试。提交（可独立回滚）。
3. 组 2：i18n 预编译 + 无引用键第一轮（整块命名空间）。提交。
4. 组 3：构建并行化、重依赖动态导入、chunk 预算。提交。
5. 组 4：i18n 按域拆分与路由守卫；无引用键第二轮。提交。
6. 组 5：组件收敛（包装层）。提交。
7. 组 6：大文件拆分（与 Glass 对应任务同批）。提交。
8. 组 7：CI 门禁接入与文档。提交。

回滚：每组一个提交；i18n 拆分保留整包入口作为回退开关（`VITE_I18N_SPLIT=0`）直到组 7 结束。

## Open Questions

- `OpenAIOAuthCapacityDialog.vue` + `api/admin/oauthCapacity.ts`：已查明（2026-09-03）——它是 63a440736 引入的"OpenAI OAuth 容量预警"弹层（按分组 / 时间范围展示 OAuth 账号套餐数与限流桶时序），在 c664f063c 被有意从账号页移除，理由是与共享的 `PlatformCapacityDialog`（容量预测）功能重叠。后端端点 `/admin/accounts/openai-oauth-capacity*` 仍在，但前端入口已被产品决策删除 → 判定为死代码，连同 spec 与 API 客户端一起删除（任务 1.3）。
- 渠道状态（已决定 2026-09-03：**V1 与 V2 都保留**，不退役、不翻默认；本变更只做 `defineAsyncComponent` 拆块与回退默认值补齐）：审计确认 `channel_monitor_mode` 不在 `stores/app.ts` 的回退默认里，未设置时 `featureFlags.ts` 视为 `'v1'`，因此 **V1 是当前默认**；管理端 `ChannelMonitorView` 也刻意同时保留 `v2` / `legacy` 两个 tab。是否把默认翻到 v2 并退役 V1（`ChannelStatusV1View.vue` 172 行 + `isChannelMonitorV1Mode` + 管理端 legacy tab + `channelStatus.*` 文案）需要用户决定；未决定前本变更只做 Glass 令牌化，不删 V1。
