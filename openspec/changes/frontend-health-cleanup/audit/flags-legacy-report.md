# Feature flags, old/new branches, legacy markers, dead code, tests, type debt

Read-only audit of `/home/box/code/sub2api/frontend`. Builds on
`scratchpad/audit/refs-report.md` (import graph) — not repeated here except where
directly relevant to a finding.

## 1. Feature flags and mode switches

Central registry: `src/utils/featureFlags.ts`. Public settings reach the frontend via
SSR injection (`window.__APP_CONFIG__`, applied in `stores/app.ts:501-507
initFromInjectedConfig`) or the async `GET` in `fetchPublicSettings` (`stores/app.ts:380-486`).
**Default-when-unanswered** for most flags comes from the synchronous fallback object at
`src/stores/app.ts:398-455`, returned only when `!publicSettingsLoaded && no cache && no injected config`.

| Flag / switch | Backend key | Read at | Mode / default before backend answers | Hides/shows |
|---|---|---|---|---|
| `channelMonitor` | `channel_monitor_enabled` | `utils/featureFlags.ts:97-101`; consumed `components/layout/AppSidebar.vue:541,570,632` | opt-out → **true** default (`stores/app.ts:441: channel_monitor_enabled: true`) | User "渠道状态" nav, admin "渠道管理→渠道监控" nav, `/monitor` & `/admin/channels/monitor` routes |
| `availableChannels` | `available_channels_enabled` | `featureFlags.ts:102-106`; `AppSidebar.vue:543,569` | opt-in → **false** default | "可用渠道" nav item (hidden in simple mode too) |
| `modelPlaza` | `model_plaza_enabled` | `featureFlags.ts:107-111`; `components/layout/AppHeader.vue:233`, `views/HomeView.vue:268` | opt-in → **false** | Header "Model Plaza" entry / home page plaza preview |
| `pluginManagement` | `plugin_management_enabled` | `featureFlags.ts:112-116`; `AppSidebar.vue:544,637` | opt-in → **false** | Admin "插件管理" nav, `/admin/plugins` |
| `payment` | `payment_enabled` | `featureFlags.ts:117-121`; `AppSidebar.vue:542,572-574`; router `requiresPayment` guard `router/index.ts:1028-1035` | opt-out → **true** (`stores/app.ts:421`) | Purchase/orders/invoices nav + routes; router redirects to dashboard only once settings are confirmed loaded and `payment_enabled === false` |
| `riskControl` | `risk_control_enabled` | `featureFlags.ts:122-126`; `AppSidebar.vue:645`; router `requiresRiskControl` `router/index.ts:1037-1044` | opt-in → **false** | Admin "安全审计" (content moderation, prompt audit) nav + routes |
| `affiliate` | `affiliate_enabled` | `featureFlags.ts:127-131`; `AppSidebar.vue:577,659` | opt-in → **false** | User "推广"/"Affiliate" nav, admin affiliate management nav |
| `ticket` | `ticket_enabled` | `featureFlags.ts:132-136`; `AppSidebar.vue:145,575,680`; router `requiresTicket` `router/index.ts:1046-1053` | opt-out → **true** (`stores/app.ts:451`) | Tickets nav (user + admin) and routes |
| `creationCenter` | `creation_center_enabled` | `featureFlags.ts:137-141`; `AppSidebar.vue:566`; router `requiresCreationCenter` `router/index.ts:1055-1062` | opt-in → **false**; router only *disables* on explicit `!== true` (`stores/app.ts:452: creation_center_enabled: false`) | "/studio" Creation Studio nav + route |
| `opsMonitoringEnabled` | `ops_monitoring_enabled` (admin-only, not public settings) | `stores/adminSettings.ts:48,64-65`; `AppSidebar.vue:551,621` | Cached in `localStorage` (`ops_monitoring_enabled_cached`), default **true** if never cached (`adminSettings.ts:48`) | Admin "运维监控" (`/admin/ops`) nav |
| `paymentEnabled` (admin store) | `/admin/payment/config` `enabled` field | `stores/adminSettings.ts:51,75-76`; `AppSidebar.vue:552,672` | Cached (`payment_enabled_cached`), default **false** if never cached | Admin "订单管理" nav+children (separate from the public `payment` flag above — this one gates the *admin* payment console specifically) |
| `batchImageAccess` | not a settings flag — derived from the user's own API keys | `composables/useBatchImageAccess.ts` (module-level singleton `hasAllowedBatchImageKey`); `AppSidebar.vue:553,567` | **false** until the key list (`keysAPI.list`) resolves and finds an active Gemini key with `group.allow_batch_image_generation === true` | "批量图片" nav + `/batch-image`; also used directly in `UserDashboardQuickActions.vue`, `DashboardView.vue` |
| `isSimpleMode` | `user.run_mode === 'simple'` (per-user, not a global setting) | `stores/auth.ts:84,99,318,460`; consumed in `AppSidebar.vue`, `AppHeader.vue:249`, `MobileDrawer.vue:216`, `router/index.ts:1065-1078`, `useOnboardingTour.ts:146-147,594`, `EditAccountModal.vue:3059`, `CreateAccountModal.vue:3670`, `AccountsView.vue:1960`, `DashboardView.vue:24` | Defaults to `'standard'` (`auth.ts:84`) until user/session loads | Hides ~15 nav items tagged `hideInSimpleMode` (groups, subscriptions, usage, redeem, tickets, payment…); router hard-redirects away from `/admin/groups`, `/admin/subscriptions`, `/admin/redeem`, `/subscriptions`, `/redeem` |
| `channel_monitor_mode` | `channel_monitor_mode: 'v1'\|'v2'` | `featureFlags.ts:176-191 getChannelMonitorMode/isChannelMonitorV1Mode/isChannelMonitorV2Mode` | **Not present** in the `stores/app.ts` fallback object at all (`types/index.ts:275` marks it optional) → `undefined` → treated as `'v1'` (`featureFlags.ts:182: mode === 'v2' ? 'v2' : 'v1'`) | Selects `ChannelStatusV1View` vs `ChannelStatusV2View` (user) and default admin tab (see §2) |
| `channel_monitor_hide_throughput` | same | `featureFlags.ts:200-203 isChannelMonitorThroughputHidden` | Also absent from the fallback object → `undefined` → `Boolean(undefined)` = **false** (shown) | Hides RPM/TPM on the user-facing V2 monitor (`ChannelStatusV2View.vue:479`) |
| `channel_monitor_show_quota` | same | `featureFlags.ts:205-214 isChannelMonitorQuotaVisible` | Also absent → **false** (hidden) — comment notes backend already strips the field server-side, this is defense-in-depth | Quota/balance snapshot on `MonitorCard.vue` |
| `hide_ccs_import_button` | `hide_ccs_import_button` | `stores/app.ts:420` (default `false`); `views/user/KeysView.vue:19,708` | opt-out semantics inline (`!publicSettings?.hide_ccs_import_button`) → **false** default (button shown) | The "Import from Claude Code / CCS" quick-import button on the Keys page |
| `backendModeEnabled` | `backend_mode_enabled` | `stores/app.ts:81`; `AppSidebar.vue` `sections` computed (returns `[]` for non-admin) | default **false** (`stores/app.ts:435`) | When true, hides the entire regular-user sidebar nav (presumably an API-only/headless deployment mode) |

Notes:
- `router/index.ts:1015-1024` explicitly awaits `fetchPublicSettings()` inside the guard before evaluating `requiresPayment/RiskControl/Ticket/CreationCenter`, and only lets settings *disable* a route once `publicSettingsLoaded` is confirmed true — a transient fetch failure never hides a route. This same care is not applied to the sidebar's `isFeatureFlagEnabled`, which instead relies purely on the opt-in/opt-out default baked into `FeatureFlags`.
- Two flags share very similar names but different sources and semantics: `flagOpsMonitoring`/`flagAdminPayment` (admin-only, `adminSettingsStore`, cached in `localStorage`) vs. the public-settings-backed registry flags (`FeatureFlags.*`, no localStorage cache, resolved purely from `cachedPublicSettings`). They are not part of the same registry despite `AppSidebar.vue`'s comment implying a single system.

## 2. Old/new implementation pairs

| Pair | Which is default / current | Status |
|---|---|---|
| `views/user/ChannelStatusView.vue` (13 lines) vs `ChannelStatusV1View.vue` (172) vs `ChannelStatusV2View.vue` (947) + `features/channel-monitor-v2/*` (2,629 lines across 8 components + tests) | `ChannelStatusView.vue` is a pure switch: `<ChannelStatusV1View v-if="isV1" /><ChannelStatusV2View v-else />` (`ChannelStatusView.vue:1-13`) driven by `isChannelMonitorV1Mode()`. Since `channel_monitor_mode` is **absent from the app.ts fallback defaults**, V1 is the default whenever the backend hasn't set it explicitly to `'v2'`. | Two fully separate implementations kept side by side, switched by one backend setting. Not dead — `ChannelStatusV1View.vue` is still the shipped default. Converge to V2 (`features/channel-monitor-v2`) once V1 is no longer needed by any deployment; requires flipping the backend default and deleting `ChannelStatusV1View.vue` + `isChannelMonitorV1Mode`. |
| `views/admin/ChannelMonitorView.vue` admin tabs `'v2'` vs `'legacy'` | `adminMonitorTab = ref(isChannelMonitorV1Mode() ? 'legacy' : 'v2')` (`ChannelMonitorView.vue:199`) — default tab tracks the same backend mode. | Admin UI deliberately keeps **both** UIs live as tabs regardless of which one end users see (label switches between "V1 Active" and "V1 History", `ChannelMonitorView.vue:36`), so admins can always manage the old CRUD-style monitor list. Not dead code — intentional dual-maintenance surface. Converge to a single admin UI once the legacy monitor CRUD flow is retired. |
| `useOnboardingTour.ts` `storageVersion = 'v4_interactive'` (`useOnboardingTour.ts:89`) | Only the current version string exists; there is **no** code path that reads or migrates older `onboarding_tour_<id>_<role>_v1/v2/v3_interactive` (or pre-versioned) localStorage keys (`grep` for `v1_interactive`/`v2_interactive`/`v3_interactive` returns nothing). | Old versioned keys are simply orphaned in users' browsers forever (harmless dead storage, not cleaned up). Converge: none needed functionally; optionally add a one-time cleanup of stale prefixed keys. |
| `components/common/BaseDialog.vue` (165 lines, 82 importers) vs `components/ui/UiModal.vue` (213 lines, 7 importers) | `ui/` is the new Glass design-system component. | **Not API-compatible.** BaseDialog: prop `show`, `width: 'narrow'\|'normal'\|'wide'\|'extra-wide'\|'full'`, `closeOnClickOutside`, `showCloseButton`. UiModal: prop `open`, `width: 'sm'\|'md'\|'lg'\|'xl'` (`components/ui/types.ts:17`), `closeOnOverlay`, `showClose`. Both independently implement focus trapping and the overlay z-index stack (`components/ui/overlayLock.ts` is shared, but BaseDialog's own focus-management code duplicates `ui/focusTrap.ts`). Converging 82 call sites requires a prop-rename codemod (`show→open`, remapped width enum) plus visual re-check, not a drop-in swap. Converge to `ui/UiModal.vue`. |
| `components/common/Select.vue` (766 lines, 61 importers) vs `components/ui/UiSelect.vue` (61 lines, 4 importers) | `ui/UiSelect.vue` is a **thin styling wrapper around `common/Select.vue`** (`UiSelect.vue:1-37`: imports `Select` from `common/`, re-exports a subset of props, restyles the trigger with glass CSS). | Already converged at the logic layer — this is not a duplicate implementation, just a themed facade exposing fewer props (no `clearable`, `creatable`, `remote`, `variant='pill'`, `valueKey/labelKey`). No migration cost beyond adding missing prop passthroughs if a caller needs them. |
| `components/common/Toggle.vue` (42 lines, 15 importers) vs `components/ui/ToggleSwitch.vue` (114 lines, 3 importers) | `ui/ToggleSwitch.vue` is the Glass version. | Independent implementations (not a wrapper), but ToggleSwitch's prop set (`modelValue`, `size`, `disabled`) is a **superset** of Toggle's (`modelValue` only) — Toggle callers can switch to ToggleSwitch with no prop changes needed, just CSS class differences (`switch/toggle-switch` global recipe vs scoped `.ui-toggle-*`). Converge to `ui/ToggleSwitch.vue`. |
| `components/common/Pagination.vue` (311 lines, 31 importers) vs `components/ui/UiPagination.vue` (67 lines, 6 importers) | `ui/UiPagination.vue` is a **thin styling wrapper around `common/Pagination.vue`** (`UiPagination.vue:1-24`, same prop/emit names, restyled). | Already converged at the logic layer, same pattern as Select. No migration cost. |
| `components/common/StatCard.vue` (81 lines, 1 non-test importer per refs-report) vs `components/ui/StatCard.vue` (117 lines, ~7 direct importers found: `UserDashboardStats.vue`, `DashboardView.vue`, `AffiliateView.vue`, `RedeemView.vue`, plus `MiniStatCard.vue` used elsewhere) | `ui/StatCard.vue` is current, wraps `GlassCard`. | **Not API-compatible** — totally different prop shapes: common (`title, value, icon: Component, iconVariant, change, changeType, formatValue`) vs ui (`label, value, sub, delta, deltaTone, variant, padding, hover`, plus a `sparkline` slot). `common/StatCard.vue` is effectively dead (its only reachability is via the `common/index.ts` barrel per refs-report, i.e., no direct feature importer). Converge by deleting `common/StatCard.vue` and its barrel export; no rename needed since it's unused. |
| `views/admin/ops/components/OpsErrorDetailModal.vue` (436 lines) vs `OpsErrorDetailsModal.vue` (300 lines) | **Both are live and used together**, not a legacy/new pair. | `OpsErrorDetailsModal` (plural) is the filterable **list** view (`props: show, timeRange, customStartTime, customEndTime, platform, groupId, errorType, resumeState`; emits `openErrorDetail`) rendered in `OpsDashboard.vue:111`. `OpsErrorDetailModal` (singular) is the **single-record detail** view (`props: show, errorId, errorType, backToList`; emits `back`) rendered right after it at `OpsDashboard.vue:124`, and independently reused from `AccountsView.vue:528` and `UsageView.vue:169`. The near-identical names are confusing but this is a coordinated list/detail pair, not duplicate implementations — no convergence needed. |
| `components/ticket/TicketCategoryForm.vue` (74 lines, singular dir) vs `components/tickets/TicketCategoryForm.vue` (44 lines, plural dir) | `components/tickets/TicketCategoryForm.vue` is current: it dispatches via `<component :is="currentComponent">` to per-category components in `components/tickets/forms/` (`TicketFormConsult/Refund/Concurrency/Rate/Other.vue`). Imported by `TicketEditorCard.vue` and `TicketDetailPane.vue`. | `components/ticket/TicketCategoryForm.vue` (old monolithic `v-if/else-if` template, one big form) has **zero importers** anywhere in `src` (confirmed by direct grep, matches refs-report). Pure dead code — delete the whole `components/ticket/` (singular) directory. |

## 3. The `legacy` markers

Case-insensitive `legacy` occurs in **89 files** (non-test + test). Counting only real source (excluding `__tests__` and i18n locale string files), the meaningful clusters are:

| File | Count | What "legacy" means here |
|---|---|---|
| `views/auth/WechatCallbackView.vue` | 11 | `legacyPendingOAuthToken`, `readLegacyFragmentLogin()` (`:450-472`) — parses `access_token`/`refresh_token` out of the URL **hash fragment**, the old OAuth callback delivery mechanism, kept as a fallback (`:1024-1043`) before/alongside the current server-side pending-token exchange. |
| `views/auth/OidcCallbackView.vue` | 11 | Same `readLegacyFragmentLogin` pattern, near-identical code duplicated per provider. |
| `views/auth/LinuxDoCallbackView.vue` | / `DingTalkCallbackView.vue` | 11 / 10 | Same pattern again — the fragment-token fallback logic is copy-pasted across all four OAuth callback views rather than shared. |
| `components/keys/UseKeyModal.vue` | 10 | `codexAuthMode: 'legacy' \| 'api-key'` (`:307-308`) — a **live, user-selectable** radio choice for Codex/OpenAI OAuth ("legacy" auth mode vs API-key mode), not dead code; default is `'legacy'`. |
| `views/user/PaymentResultView.vue` | 9 | `hasLegacyFallbackContext`, `legacyOrder` (`:397-439`) — fallback path that resolves a payment result via `trade_status`/`out_trade_no` query params (old payment redirect scheme) when the newer order-ID-based lookup fails or is absent. |
| `views/admin/UsageView.vue` | 9 | `requestTypeToLegacyStream` (from `utils/usageRequestType.ts:25`) converts the new `UsageRequestType` enum back into the old boolean `stream` filter param for the usage list API, called at 4 sites (`:406,433,471,522`). |
| `views/admin/SettingsView.vue` | 8 | WeChat "legacy mode" save/load plumbing (see below) plus scattered UI copy. |
| `components/account/EditAccountModal.vue` | 6 | `legacyBaseUrl`/`legacyProtocol` (`:4377-4403`) — migrates an account's old single `credentials.base_url` field into the newer per-protocol `adaptiveBaseUrls` map. |
| `views/admin/ChannelMonitorView.vue` | 6 | The `'legacy'` admin tab id, see §2. |
| `components/tickets/forms/TicketFormRate.vue` | 6 | (rate-plan legacy field handling) |
| `components/account/AccountUsageCell.vue` | 2 | Comments at `:997,1014` — fallback classification for old/unknown account-tier usage payload shapes ("Backward compatibility (legacy tier markers)"). |
| `types/index.ts` | 3 | `'LEGACY'` status literal (`:1016`); `CodexUsageSnapshot` legacy vs canonical fields (`:1515-1529`, see below); `ImageSizeSource` includes `'legacy'` (`:1761`). |
| `utils/imageUsage.ts` | 3 | Renders a `usage.imageSizeSourceLegacy` / `imageSizeLegacyUnstandardized` label when the backend reports an image whose size wasn't normalized by the current logic. |
| `api/admin/ops.ts` | 2 | Comments marking "Legacy unified endpoints" (`:1117,1324`) still exported/used for older ops API routes alongside newer split endpoints. |
| `api/auth.ts` | 5 | `legacyEnabled = settings?.wechat_oauth_enabled ?? false` (`:396,426`) — fallback when the newer split `wechat_oauth_open/mp/mobile_enabled` flags are absent. |
| `utils/apiError.ts` | 1 | Comment: "Legacy axios shape: `{ response.data.detail }`" — still parsed as one of several error-shape fallbacks. |
| `stores/auth.ts` | 1 | Comment describing an access-token-only legacy login response shape. |
| `composables/useModelWhitelist.ts` | 1 | A hardcoded rename-mapping entry `'Composer legacy'` (model migration label, not code legacy). |
| `components/admin/channel/types.ts` | 3 | `LEGACY_CLOCK_TIME` regex (`HH:MM` without seconds) normalized to `HH:MM:00` for backward-compatible period objects. |

**`src/api/admin/settings.ts` legacy fields in detail** (4 occurrences, `:379-417`):
- `resolveWeChatConnectModeCapabilities(openEnabled, mpEnabled, mobileEnabled, legacyMode)` — if none of the three new booleans (`wechat_oauth_open_enabled`, `wechat_oauth_mp_enabled`, `wechat_oauth_mobile_enabled`) are set, it falls back to decoding the old single `wechat_connect_mode: 'open'|'mp'|'mobile'` string (`legacyMode`) via `normalizeWeChatConnectMode`.
- `deriveWeChatConnectStoredMode(openEnabled, mpEnabled, mobileEnabled, legacyMode)` — the inverse: derives the string to still **write back** to `wechat_connect_mode` for old backends/DB rows that only read that field.
- **Still actively read and written**: `views/admin/SettingsView.vue` calls both functions at load (`:11517-11530`, `:12231-12248`) and save time (`:11814-11955` — `wechat_connect_mode: wechatStoredMode` is included in the save payload alongside the three new booleans). This is live dual-write compatibility code, not dead.
- The only UI that exists purely to support this legacy field is the derivation logic itself — there's no separate "legacy WeChat mode" form control in the UI; the three boolean toggles are primary and `wechat_connect_mode` is maintained silently underneath for old-backend compatibility.

## 4. Dead branches inside the four big files

Checked `views/admin/SettingsView.vue` (14,077 lines), `components/account/CreateAccountModal.vue` (7,464), `components/account/EditAccountModal.vue` (5,932), `views/admin/GroupsView.vue` (7,015):

- **`v-if="false"` / `v-else-if="false"` / `v-show="false"`**: none found in any of the four files.
- **Commented-out template blocks >10 lines**: none found (`awk` scan for `<!-- ... -->` spans >10 lines returned nothing in all four files).
- **Unused `ref`/`computed`**: `eslint --ext .vue,.ts` (rule `@typescript-eslint/no-unused-vars`, configured as `"warn"` with `argsIgnorePattern/varsIgnorePattern: "^_"` in `.eslintrc.cjs:22-25`) against these four files produced **zero warnings**. `tsconfig.json` also has `noUnusedLocals`/`noUnusedParameters: true` (`:15-16`); a full `vue-tsc --noEmit` run produced errors only in three unrelated files (`views/admin/RedeemView.vue`, `views/admin/TicketDetailView.vue`, `views/user/KeysView.vue` — pre-existing `Property 'x' does not exist on type CreateComponentPublicInstanceWithMixins…` errors, unrelated to unused-variable detection and likely a `vue-tsc`/component-typing drift issue, not scoped to this audit's four files). No unused ref/computed evidence in the four target files.
- **Platform-specific panels for removed platforms**: current enums are `GroupPlatform`/`AccountPlatform` = `anthropic | openai | gemini | antigravity | grok | kiro | kimi | zhipu | deepseek (+ composite for groups)` (`types/index.ts:550,920`). Every `platform === '...'` branch found in the four files (240+ occurrences scanned) matches one of these current values — no stale platform checks (e.g. no leftover `'claude'`, `'chatgpt'`, `'copilot'`, `'qwen'`, `'azure'` branches) were found.

## 5. Test health

- **Broken imports**: scanned all 307 `*.spec.ts` files' `from '...'` specifiers (both relative and `@/` alias) against the filesystem — **0 broken imports** found.
- **Total spec files**: 307.
- **Spec-for-dead-component candidates** (component has 0 non-test importers, kept alive only by its own spec — from `refs-report.md`'s "referenced only by tests" list, deletable together with their spec):
  - `components/admin/account/OpenAIOAuthCapacityDialog.vue` (587 lines) ← `__tests__/OpenAIOAuthCapacityDialog.spec.ts`
  - `components/payment/PaymentQRDialog.vue` (311) ← `__tests__/PaymentQRDialog.spec.ts`
  - `components/admin/payment/AdminOrderTable.vue` (238) ← `__tests__/orderCurrencyDisplay.spec.ts`
  - `components/admin/payment/AdminOrderDetail.vue` (165) ← `__tests__/orderCurrencyDisplay.spec.ts` (same spec covers both)
  - `components/keys/EndpointPopover.vue` (141) ← `__tests__/EndpointPopover.spec.ts`
  - `composables/useForm.ts` (43) ← `__tests__/useForm.spec.ts`
  - Additionally, the fully-unreferenced components (`OpsRuntimeSettingsCard.vue`, `OpsEmailNotificationCard.vue`, `StripePaymentInline.vue`, `PlatformUsageBreakdown.vue`, `PaymentMethodChart.vue`, `TicketCategoryForm.vue` [ticket/, dead per §2], `TopUsersLeaderboard.vue`, `Skeleton.vue`, `ProfileAccountBindingsCard.vue`, `StatusBadge.vue` [common/], `TicketInfoItem.vue` [tickets/]) have **no spec files at all** — checked each, none has a matching `__tests__/*.spec.ts`.
- **Slowest last vitest run** (log at `scratchpad/vitest.log`, `vitest v2.1.9`, snapshot: 307 test files, 295 passed/12 failed, 2114 tests, 2088 passed/26 failed, wall duration 60.28s):

| Rank | Duration | Spec file | Tests |
|---|---|---|---|
| 1 | 5157ms | `views/admin/__tests__/SettingsView.spec.ts` | 38 |
| 2 | 2739ms | `composables/__tests__/useRoutePrefetch.spec.ts` | 15 |
| 3 | 1834ms | `components/account/__tests__/EditAccountModal.spec.ts` | 58 |
| 4 | 1721ms | `components/layout/__tests__/MobileDrawer.spec.ts` | 19 |
| 5 | 1256ms | `api/__tests__/client.spec.ts` | 20 |
| 6 | 1253ms | `components/account/__tests__/BulkEditAccountModal.spec.ts` | 51 |
| 7 | 1154ms | `components/account/__tests__/CreateAccountModal.spec.ts` | 27 |
| 8 | 847ms | `components/layout/__tests__/AppSidebar.sections.spec.ts` | 15 |
| 9 | 595ms | `views/admin/__tests__/UsageView.spec.ts` | 13 |
| 10 | 416ms | `views/auth/__tests__/WechatCallbackView.spec.ts` | 25 (tie: `MonitorFormDialog.accountSelector.spec.ts`, 10 tests) |

  This log snapshot also recorded 12 **failing** spec files at capture time (unclear if still failing on current `HEAD`): `src/__tests__/designTokens.spec.ts`, `components/common/__tests__/BaseDialog.spec.ts`, `components/layout/__tests__/TablePageLayout.spec.ts`, `components/payment/__tests__/PaymentMethodSelector.spec.ts`, `views/__tests__/HomeView.compact.spec.ts`, `views/admin/__tests__/AccountsView.bulkEdit.spec.ts`, `views/admin/__tests__/AccountsView.sparkShadow.spec.ts`, `views/admin/__tests__/AccountsView.usageWindowsHint.spec.ts`, `views/admin/__tests__/RedeemView.batchUpdate.spec.ts`, `views/auth/__tests__/TencentCaptchaActionGate.spec.ts`, `views/user/__tests__/AffiliateView.spec.ts`, `views/user/__tests__/KeysView.spec.ts` (this last one's failure trace shows `Cannot read properties of undefined (reading 'length')` at `KeysView.vue:37`, `endpointCards.length`, in every "column settings" test — looks like a real regression rather than a flaky test).

## 6. Type-safety debt

`: any` / `as any` occurrences (`grep -E`, non-test source only): **365** across **91 files** (test-file occurrences bring the raw total to 599; the task's cited figure of 357 is close to this and likely reflects a slightly different count method or minor code drift since that count was taken).

By directory (non-test, occurrence count):

| Directory | Count |
|---|---|
| `src/views/admin` (top-level files) | 96 |
| `src/views/admin/ops/components` | 44 |
| `src/components/account` | 38 |
| `src/components/common` | 34 |
| `src/composables` | 30 |
| `src/components/admin/account` | 25 |
| `src/views/user` | 23 |
| `src/components/user/profile` | 10 |
| `src/components/admin/user` | 10 |
| `src/views/admin/ops` (top-level) | 9 |
| `src/components/admin/usage` | 7 |
| `src/components/charts` | 6 |
| `src/components/admin/settings` | 6 |
| `src/api` | 6 |
| `src/components/admin` (top-level) | 5 |
| `src/stores` | 3 |
| `src/components/user` (top-level) | 3 |
| `src/components/admin/announcements` | 3 |
| `src/components/auth` | 2 |
| `src/api/admin` | 2 |

Top 15 files by raw occurrence count (non-test):

| File | Count |
|---|---|
| `views/admin/AccountsView.vue` | 21 |
| `views/admin/ProxiesView.vue` | 18 |
| `components/common/DataTable.vue` | 18 |
| `components/admin/account/ReAuthAccountModal.vue` | 17 |
| `components/account/CreateAccountModal.vue` | 17 |
| `views/user/BatchImageGuideView.vue` | 16 |
| `views/admin/ops/components/OpsRuntimeSettingsCard.vue` | 14 |
| `views/admin/GroupsView.vue` | 13 |
| `views/admin/ops/OpsDashboard.vue` | 9 |
| `components/common/Select.vue` | 9 |
| `views/admin/SubscriptionsView.vue` | 7 |
| `views/admin/AnnouncementsView.vue` | 7 |
| `components/account/ReAuthAccountModal.vue` | 7 |
| `views/admin/ops/components/OpsAlertEventsCard.vue` | 6 |
| `views/admin/RedeemView.vue` | 6 |

`eslint-disable` sites (non-test source, all 5 found repo-wide):

| File:line | Rule disabled |
|---|---|
| `components/admin/TLSFingerprintProfilesModal.vue:347` | `@typescript-eslint/no-unused-vars` |
| `components/admin/ErrorPassthroughRulesModal.vue:451` | `@typescript-eslint/no-unused-vars` |
| `components/account/credentialsBuilder.ts:102` | `no-control-regex` |
| `views/KeyUsageView.vue:454` | `@typescript-eslint/no-explicit-any` |
| `views/KeyUsageView.vue:804` | `@typescript-eslint/no-explicit-any` |

Note: the project's `.eslintrc.cjs:20` globally sets `"@typescript-eslint/no-explicit-any": "off"`, so the 365 non-test `any` usages produce no lint signal at all today — the two explicit `eslint-disable` comments for that rule in `KeyUsageView.vue` are vestigial (disabling an already-off rule).
