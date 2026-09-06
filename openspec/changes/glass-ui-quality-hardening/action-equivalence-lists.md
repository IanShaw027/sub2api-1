# List Action Equivalence Review

## Scope And Result

- Baseline: `f1c8ab7da6284179461613cec8a30a8f87f2c545`.
- Candidate source: `actions-candidates.json`; reviewed IDs: `action-115..122`, `action-199..213`, `action-234..243`, `action-254..263`, `action-319..340` (65 candidates).
- Result: all 65 have a traceable current action equivalent. No confirmed action loss was found in this set. This is a source-level reachability review, not a claim that browser interaction or all data contracts have been tested.
- Initial audit was read-only; a follow-up implementation restored the baseline direct commands described below. API/store contracts, routes/links and the separate 31-anchor audit are intentionally not repeated here.
- Direct commands must not be certified solely because a replacement `More` menu exists. Baseline direct commands were restored; menus that already existed before Glass remain menus. Existing new More menus are retained as supplemental entries.
- Paths below are repository-relative. Line numbers reflect the shared workspace at review time and may shift; component and handler names provide stable lookup keys.

## Shared Pagination And Key Helpers

| Candidates | Before | Current equivalent and evidence |
| --- | --- | --- |
| action-115 | Numeric guard inside pagination click | `frontend/src/components/common/Pagination.vue:88`: non-number ellipsis renders as a `span`; numeric `v-else` button calls `goToPage(pageNum)` at line 95. The guard moved to the template branch, not removed. |
| action-116, action-117, action-118, action-119 | Endpoint code click / Enter / Space and adjacent copy icon | `frontend/src/components/ui/EndpointCard.vue:9`: native labeled copy button emits `copy(url)` and supports native Enter/Space activation. `frontend/src/views/user/KeysView.vue:43` receives `copyEndpoint` for every endpoint card. The URL code itself is no longer interactive; its adjacent visible copy button is the equivalent command. |
| action-120 | Copy generated configuration file | `frontend/src/components/keys/use-key/CodeFileBlocks.vue:19` emits `(file.content, index)`; `frontend/src/components/keys/UseKeyModal.vue:55` receives `copyContent`. Copied-index feedback remains. |
| action-121 | Download Codex model manifest | `frontend/src/components/keys/use-key/CodexModelCatalogSection.vue:17` ready-state button emits `download`; `UseKeyModal.vue:65` receives `downloadCodexModelManifest`. |
| action-122 | Fetch/retry Codex model manifest | `CodexModelCatalogSection.vue:26` non-ready button emits `fetch`, retaining loading/no-key disable conditions and error retry label; `UseKeyModal.vue:64` receives `loadCodexModelManifest`. |

## Proxies

All source candidates in this section refer to baseline `frontend/src/views/admin/ProxiesView.vue`.

| Candidates | Before | Current equivalent and evidence |
| --- | --- | --- |
| action-199 | `handleBatchQualityCheck` toolbar button | Restored direct header Button `@click="handleBatchQualityCheck"`, preserving loading/checking disable conditions; More equivalent remains supplemental. |
| action-200 | `openBatchDelete` toolbar button | Restored direct header Button `@click="openBatchDelete"`; `selectedCount === 0` remains disabled and confirmation remains. |
| action-201 | Open import | Restored direct header upload button setting `showImportData = true`; import dialog remains. |
| action-202 | Open export | Restored direct header download button setting `showExportDataDialog = true`; selected/all export label remains in its accessible name/title. |
| action-203 | `handleTestConnection(row)` | Restored direct row play button calling the same handler and preserving `testingProxyIds.has(row.id)` disabled state; row More remains supplemental. |
| action-204 | `handleQualityCheck(row)` | Restored direct row shield button calling the same handler and preserving `qualityCheckingProxyIds.has(row.id)` disabled state. |
| action-205, action-208 | Separate create/edit form submits | `frontend/src/components/admin/proxies/ProxyFormModal.vue:52` shared form dispatches `isEdit ? handleUpdateProxy() : handleCreateProxy()`. Footer submit buttons target `proxy-form`; create/edit branches and submitting disable conditions remain. Parent mounts this modal at `ProxiesView.vue:330`. |
| action-206, action-210 | Separate create/edit password visibility toggles | `ProxyFormModal.vue:118` shared `passwordVisible = !passwordVisible`; the shared field switches its type, and create/edit initialization resets visibility. |
| action-207, action-211 | Create/edit expiry day presets | `ProxyFormModal.vue:139` shared preset buttons set `expiresDays = d`; computed `baseDate` distinguishes creation from editing and `expiresDays` setter updates `form.expires_at`. |
| action-209 | Mark edited password dirty | `ProxyFormModal.vue:113` input sets `passwordDirty = true`; edit submit includes a password change only when this flag is set. |
| action-212 | Close quality report | `frontend/src/components/admin/proxies/ProxyQualityReportModal.vue:6` dialog close and line 84 footer both emit `close`; parent `ProxiesView.vue:381` receives `closeQualityReportDialog`. |
| action-213 | Close attached-accounts dialog | `frontend/src/components/admin/proxies/ProxyAccountsModal.vue:6` dialog close and line 39 footer emit `close`; parent `ProxiesView.vue:388` receives `closeAccountsModal`. |

## Subscriptions

All source candidates in this section refer to baseline `frontend/src/views/admin/SubscriptionsView.vue`.

| Candidates | Before | Current equivalent and evidence |
| --- | --- | --- |
| action-234 | Reset quota from row | Restored direct active-only row refresh button in `SubscriptionsView.vue` calling `handleResetQuota(row)` with baseline `resettingQuota && resettingSubscription?.id === row.id` busy guard. Confirmation and supplemental More item remain. |
| action-235 | Revoke subscription from row | Restored direct active-only row ban button calling `handleRevoke(row)`; destructive confirmation and supplemental More item remain. |
| action-236, action-237 | Assignment dialog close/cancel | `frontend/src/components/admin/subscriptions/SubscriptionAssignDialog.vue:6` and line 100 call local `close()`. At line 247 it emits `close` then clears user/group/day and search state, matching baseline `closeAssignModal`; parent `SubscriptionsView.vue:406` closes visibility. |
| action-238 | Adjustment dialog cancel | `frontend/src/components/admin/subscriptions/SubscriptionAdjustDialog.vue:54` emits `close`; parent `SubscriptionsView.vue:414` calls `closeExtendModal`, clearing visibility and selected subscription. |
| action-239, action-240, action-241, action-242, action-243 | Guide backdrop mousedown, backdrop click, close icon, group link click, footer close | `frontend/src/components/admin/subscriptions/SubscriptionGuideDialog.vue:7`, `:9`, `:11`, `:38`, `:84` preserve the five respective close emissions. `SubscriptionsView.vue:451` handles all via `showGuideModal = false`; guide opening remains a visible header button at line 39. |

## Users

All source candidates in this section refer to baseline `frontend/src/views/admin/UsersView.vue`.

| Candidates | Before | Current equivalent and evidence |
| --- | --- | --- |
| action-254 | Toggle filter-configuration dropdown | Restored dedicated filter icon Button named `admin.users.filterSettings` in the header. It opens the same built-in/attribute filter dropdown directly via `showMoreDropdown`; no generic More click is required. Filter configuration was already a dropdown before Glass. |
| action-255 | Open attribute configuration | Restored dedicated cog Button named `admin.users.attributes.configButton` setting `showAttributesModal = true`; the existing More entry remains supplemental. |
| action-256 | Open per-user action menu | `UsersView.vue:566` uses `ActionsCell` with `getUserActionItems(row)`. `frontend/src/components/common/cells/ActionsCell.vue` provides the visible More trigger and menu; the old page-owned positioning state is replaced. |
| action-257 | View API keys and close menu | `UsersView.vue:915` item calls `handleViewApiKeys(user)`; `ActionsCell.vue:155` `handleItemClick` runs the item then closes. |
| action-258 | Allowed groups and close menu | `UsersView.vue:916` item calls `handleAllowedGroups(user)` with the same shared close sequence. |
| action-259 | Deposit and close menu | `UsersView.vue:917` item calls `handleDeposit(user)` with shared close. Direct balance-cell deposit remains as an additional entry. |
| action-260 | Withdraw and close menu | `UsersView.vue:918` item calls `handleWithdraw(user)` with shared close. |
| action-261 | Platform quota and close menu | `UsersView.vue:919` item calls `handlePlatformQuota(user)` with shared close; quota modal remains mounted. |
| action-262 | Balance history and close menu | `UsersView.vue:920` item calls `handleBalanceHistory(user)` with shared close; direct balance-cell entry remains. |
| action-263 | Delete and close menu | `UsersView.vue:922` preserves `user.role !== 'admin'`; delete item calls `handleDelete(user)` with shared close, followed by existing delete confirmation. |

## Keys

All source candidates in this section refer to baseline `frontend/src/views/user/KeysView.vue`.

| Candidates | Before | Current equivalent and evidence |
| --- | --- | --- |
| action-319 | Copy key from cell | `frontend/src/components/keys/KeysDataTable.vue` copy button emits `(value, row.id)`; parent `KeysView.vue` receives `@copy="copyToClipboard"`. `KeysMobileList` forwards the same key/id payload from mobile cards. |
| action-320 | Open row group selector | Desktop group button remains. Restored mobile group button plus `setGroupButtonRef` forwarding through `KeysMobileList`, so both emit `open-group-selector(row)` to the same parent handler. Baseline `DataTable.vue` mobile `dataColumns` rendered the same `cell-group` slot, confirming mobile inline editing was an existing capability. |
| action-321 | Reset rate limit from row cell | Restored direct refresh button in desktop rate-limit cell and mobile rate-limit section, visible when any window usage is positive. Both emit `reset-rate-limit(row)` to `confirmResetRateLimitFromTable`; supplemental More action remains. |
| action-322 | Use key | `frontend/src/components/keys/KeyInlineActions.vue` terminal button emits `use`; both `KeysDataTable` and `KeysMobileList` forward the current row to `openUseKeyModal`. No More click required. |
| action-323 | Import row key to CC Switch | Restored direct upload button in shared `KeyInlineActions`, preserving `!hideCcsImport`; desktop/mobile forward `import-ccs(row)` to `importToCcswitch`. |
| action-324 | Toggle key status | Restored direct ban/check button in shared `KeyInlineActions`; desktop/mobile forward `toggle-status(row)` to `toggleKeyStatus`, retaining active/inactive label. |
| action-325 | Edit key | Restored direct edit button in shared `KeyInlineActions`; desktop/mobile forward `edit(row)` to `editKey` -> mounted `KeyFormModal`. |
| action-326 | Delete key | Restored direct trash button in shared `KeyInlineActions`; desktop/mobile forward `delete(row)` to `confirmDelete` -> destructive confirmation. |
| action-327 | Toggle custom key | `frontend/src/components/keys/KeyFormModal.vue:64` `ToggleSwitch v-model="formData.use_custom_key"`; custom field conditional and submit validation remain. |
| action-328 | Toggle IP restrictions | `KeyFormModal.vue:88` `ToggleSwitch v-model="formData.enable_ip_restriction"`; white/blacklist inputs remain under the same conditional. |
| action-329 | Toggle rate limits | `KeyFormModal.vue:138` `ToggleSwitch v-model="formData.enable_rate_limit"`; all configured limit inputs remain. |
| action-330 | Toggle expiration | `KeyFormModal.vue:174` `ToggleSwitch v-model="formData.enable_expiration"`; expiration controls remain. |
| action-331 | Cancel create/edit form | `KeyFormModal.vue:211` footer and dialog close emit `close`; parent `KeysView.vue` receives `closeModals`. |
| action-332, action-333 | Reset quota confirmation/cancel | `KeyFormModal.vue:129` opens `confirmResetQuota`; lines 225-234 render confirm/cancel modal; `runConfirm` executes captured `onConfirm` while cancel clears `confirmDialog`. At line 526 callback targets the editing key and emits saved after reset. |
| action-334, action-335 | Reset rate limit confirmation/cancel | `KeyFormModal.vue:164` opens `confirmResetRateLimit`; same confirm/cancel UI executes its distinct callback at line 550. Row reset uses parent-owned confirm state at `KeysView.vue:795`; both are still available. |
| action-336, action-339 | Close CC Switch client selection from dialog/cancel | `frontend/src/components/keys/CcSwitchModals.vue:28` dialog and line 44 footer emit `close-client-select`; `KeysView.vue:235` receives `closeCcsClientSelect`, preserving parent cleanup. |
| action-337 | Select Claude client | `CcSwitchModals.vue:32` emits `select-client('claude')`; parent receives `handleCcsClientSelect`. |
| action-338 | Select Gemini client | `CcSwitchModals.vue:37` emits `select-client('gemini')`; same parent handler. |
| action-340 | Choose new group | `frontend/src/components/keys/KeyGroupPicker.vue` option button emits selected value; parent `KeysView.vue` receives `@select="(value) => changeGroup(selectedKeyForGroup!, value)"`. |

## Remaining Verification Boundary

Additional baseline direct commands were checked even when absent from the syntax candidate list: proxy header `handleBatchTest` and proxy row `handleDelete(row)` were also restored. Proxy edit, subscription adjust/restore, Keys copy, and Users edit/status were already direct and remain so. Users API keys/groups/deposit/withdraw/quota/history/delete were already in a row More menu before Glass and were deliberately not moved out.

Restoration totals: Keys five shared direct row commands (four newly restored on desktop, all five on mobile), one rate reset command on both form factors, and mobile inline group editing; Proxies five header tools and three row commands; Subscriptions two active-row commands; Users two dedicated configuration triggers. Existing extra More entries are not removed. Direct icon commands use existing icons, accessible names and hover titles; Keys mobile action touch targets are 44px, and proxy header tools wrap rather than disappear.

Tests: `frontend/src/views/admin/__tests__/listDirectActions.preservation.spec.ts` renders actual header/action slot templates with More closed, checks handler payloads and baseline active/busy guards. `frontend/src/components/keys/__tests__/KeyMobileCard.preservation.spec.ts` covers desktop/mobile direct event forwarding, import feature visibility and mobile group/reset access. Full browser visibility/overflow verification remains in the separate interaction matrix. No production mutation was executed during this review.
