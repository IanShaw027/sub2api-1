# Settings, Risk, Ticket and Payment Action Equivalence

- Baseline: `f1c8ab7da6284179461613cec8a30a8f87f2c545` (pre-Glass).
- Candidate source: `actions-candidates.json`; this report covers 47 IDs listed below.
- Method: compare baseline templates with current component callers, emitted events, model bindings and guards. This is a bounded action audit, not an API/store/link or anchor inventory.
- `equivalent` means a reachable action has an equivalent current binding; it does not claim every responsive rendering or payment transaction was exercised.
- `unused-baseline` means the removed component was not used by any production component in the baseline, verified using `git grep` on that revision.

## Findings

1. Risk date filters had lost time precision: baseline `datetime-local` fields became `DateRangePicker` date-only fields, and query normalization expanded values to whole-day bounds. Restored the two local date-time fields and the baseline `new Date(value).toISOString()` behavior. `action-226` and `action-227` passed the new focused regression test below.
2. Ticket template actions had a real focus regression: a formerly unique `templateTriggerRef` moved onto a `v-for`, but apply/save still called `.focus()` on that ref. Template body previews also became title-only hover text. The shell agent repaired this: visible two-line previews, apply restores focus to the actual event target, and template management owns a unique trigger ref. `TicketDetailView.templates.spec.ts` passed 2 tests covering multiple template choices and saving after deleting all templates. The updated source was re-inspected; `action-244` through `action-252` are closed with those repairs.
3. Test-email input moved from the settings page into a clearly labeled send-test-email modal. The parent explicitly accepted this layout difference: open the existing action, enter a recipient, submit. The action and validation remain available; the field is not removed.

## Risk Control

All paths in this table are relative to `frontend/src/`.

| Candidate | Baseline action | Current reachable equivalent / evidence | Result |
| --- | --- | --- | --- |
| action-221 | `click:loadLogs` | `components/admin/risk-control/RiskRecordsTable.vue:17` emits `refresh`; `views/admin/RiskControlView.vue:212` calls `loadLogs` | equivalent |
| action-222 | result `change:reloadLogsFromFirstPage` | `RiskRecordsTable.vue:25` updates `filters.result`, emits `reload-first-page`; parent `RiskControlView.vue:213` | equivalent |
| action-223 | group `change:reloadLogsFromFirstPage` | `RiskRecordsTable.vue:26` updates `filters.group_id`, same parent handler | equivalent |
| action-224 | endpoint `change:reloadLogsFromFirstPage` | `RiskRecordsTable.vue:27` updates `filters.endpoint`, same parent handler | equivalent |
| action-225 | search Enter `reloadLogsFromFirstPage` | `RiskRecordsTable.vue:28` retains trimmed search and `keyup.enter` | equivalent |
| action-226 | from `datetime-local` change | `RiskRecordsTable.vue:29` restored exact local time; `useRiskControlData.ts` uses `normalizeDateTimeLocal` | repaired, verified |
| action-227 | to `datetime-local` change | `RiskRecordsTable.vue:30` restored exact local time, same normalization | repaired, verified |
| action-228 | `unbanUser(row)` | `RiskRecordsTable.vue` emits `unban` with row; parent `RiskControlView.vue:214`; retains `canUnbanRow` and in-flight disabled guard | equivalent |
| action-229 | `openInputDetail(row)` | `RiskRecordsTable.vue` emits `open-input-detail` with row; parent `RiskControlView.vue:215` | equivalent |
| action-230 | `clearFlaggedHashes` | `RiskControlSettingsModal.vue:468` emits `clear-flagged-hashes`; parent `RiskControlView.vue:234` | equivalent |
| action-231 | `deleteFlaggedHash` | `RiskControlSettingsModal.vue:485` emits `delete-flagged-hash`; parent `RiskControlView.vue:233` | equivalent |
| action-232 | `closeInputDetail` | `InputDetailModal.vue:6` and `:54` emit close through dialog/close button; parent `RiskControlView.vue:237` | equivalent |

## Settings

| Candidate | Baseline action | Current reachable equivalent / evidence | Result |
| --- | --- | --- | --- |
| action-233 | `click:sendTestEmail` | `components/admin/settings/tabs/EmailTab.vue:205` opens modal; its `send-test-email-form` submits `sendTestEmail` at `:227`; footer submit targets the same form at `:254`; recipient/busy validation and email-verification visibility retained | equivalent, accepted layout difference |

## Ticket Detail

The old menu-specific open/close listeners are unnecessary only once the replacement rail is usable by pointer and keyboard and the focus regression is repaired. They must not be treated as missing business actions: all template choices and template management are directly rendered in the composer action slot.

| Candidate | Baseline action | Current destination / verification condition | Result |
| --- | --- | --- | --- |
| action-244 | focusin opens template menu | `views/admin/TicketDetailView.vue` composer `template-rail`, directly focusable template buttons | repaired, verified |
| action-245 | focusout closes template menu | No hidden menu; leaving the always-visible rail needs no close handler | repaired, verified |
| action-246 | mouseenter opens template menu | Template choices directly visible without hover | repaired, verified |
| action-247 | mouseleave closes template menu | No hidden menu or hover-dependent action access | repaired, verified |
| action-248 | toggle template menu | Direct template choices plus explicit manage button | repaired, verified |
| action-249 | Enter opens/focuses menu | Native template buttons accept Enter once focus target repair is verified | repaired, verified |
| action-250 | Space opens/focuses menu | Native template buttons accept Space once focus target repair is verified | repaired, verified |
| action-251 | Down opens/focuses menu | Native Tab traversal replaces navigation into a now-removed menu | repaired, verified |
| action-252 | Escape closes menu/restores focus | No menu to close; template-management dialog retains close; unique trigger focus restoration must be repaired | repaired, verified |
| action-253 | `updateStatus(status)` | `UiSelect` binds same `availableAdminStatuses` through `statusActionOptions`; `handleStatusSelect` calls `updateStatus`; action-loading guard retained | equivalent |

## Admin Payments

`git grep -n -e AdminOrderDetail -e AdminOrderTable f1c8ab7da -- frontend/src` found only their old unit tests and a documentation comment, not production imports. The active baseline page already implemented its own filters/detail/actions. Current `views/admin/orders/AdminOrdersView.vue` retains those active paths.

| Candidate | Removed component action | Current active page evidence | Result |
| --- | --- | --- | --- |
| action-104 | unused `AdminOrderDetail` cancel | `AdminOrdersView.vue:122` calls `handleCancelOrder(row)` for PENDING | unused-baseline |
| action-105 | unused detail retry | `AdminOrdersView.vue:132` calls `handleRetryOrder(row)` for FAILED | unused-baseline |
| action-106 | unused detail refund | `AdminOrdersView.vue:140`, `:152`, `:173` route eligible refund states to `openRefundDialog` | unused-baseline |
| action-107 | unused `AdminOrderTable` status filter | `AdminOrdersView.vue:30` status Select and `loadOrders` | unused-baseline |
| action-108 | unused table payment-type filter | `AdminOrdersView.vue:38` payment-type Select and `loadOrders` | unused-baseline |
| action-109 | unused table order-type filter | `AdminOrdersView.vue:46` order-type Select and `loadOrders` | unused-baseline |
| action-110 | unused table detail | `AdminOrdersView.vue:112` calls `showOrderDetail(row)` | unused-baseline |
| action-111 | unused table cancel | Active PENDING row action remains at `AdminOrdersView.vue:122` | unused-baseline |
| action-112 | unused table retry | Active FAILED row action remains at `AdminOrdersView.vue:132` | unused-baseline |
| action-113 | unused table refund | Active refund/status-query actions remain at `AdminOrdersView.vue:140` onward | unused-baseline |
| action-264 | dashboard `days = d` | `AdminPaymentDashboardView.vue:7` SegmentedControl binds `daysStr`; same `[7,30,90]`; watcher reloads and `Number(daysStr)` preserves numeric request value | equivalent |
| action-265 | list `toggleForSale(row)` | `AdminPaymentPlansView.vue:54` ToggleSwitch update calls the same `toggleForSale(row)` | equivalent |
| action-266 | dialog `planForm.for_sale = !planForm.for_sale` | `PlanEditDialog.vue:69` ToggleSwitch v-model binds `planForm.for_sale`; persisted field unchanged | equivalent |

## User Payments

| Candidate | Baseline action | Current reachable equivalent / evidence | Result |
| --- | --- | --- | --- |
| action-341 | `activeTab = tab.key` | `PurchaseTabSwitcher.vue:9` emits same tab key; `views/user/PaymentView.vue:14` updates activeTab; same select-phase visibility | equivalent |
| action-342 | recharge method select | `RechargePanel.vue:25` forwards selected method; `PaymentView.vue:61` updates `selectedMethod` | equivalent |
| action-343 | `handleSubmitRecharge` | `RechargePanel.vue:51` emits submit with original canSubmit/submitting guards; `PaymentView.vue:62` calls handler | equivalent |
| action-344 | subscription method select | `SubscriptionConfirmCard.vue:54` forwards method; `PaymentView.vue:86` updates selectedMethod | equivalent |
| action-345 | `confirmSubscribe` | `SubscriptionConfirmCard.vue:73` emits submit; `PaymentView.vue:87` calls same handler with eligibility/busy guards | equivalent |
| action-346 | `selectedPlan = null` | `SubscriptionConfirmCard.vue:80` emits cancel; `PaymentView.vue:88` clears selectedPlan | equivalent |
| action-347 | preview checkout help image | `CheckoutHelpCard.vue:6` emits actual helpImageUrl; `PaymentView.vue:113` updates previewImage and renders existing preview overlay | equivalent |
| action-348 | renewal overlay closes modal | `RenewalPlanModal.vue:2` UiModal close emits to `PaymentView.vue:122` closeRenewalModal | equivalent |
| action-349 | renewal close button | Same UiModal exposes its accessible close button, forwarding to closeRenewalModal | equivalent |
| action-355 | cancel invoice | `UserInvoiceDetailView.vue:74` opens explicit confirmation; `:96` calls `confirmCancelInvoice`, which cancels current invoice at `:178`; same APPLIED eligibility/busy guards | equivalent, confirmation added |
| action-356 | order-status Select change | `UserOrdersView.vue:17` SegmentedControl and `:23` mobile ChipScroller update the same filter then `fetchOrders`; both derive from the full statusFilters set | equivalent |

## Verification

- Inspection covered 47 / 47 assigned candidate IDs: 26 equivalent actions, 10 unused baseline component actions, and 11 candidates closed by the risk-time/ticket-template repairs. No unresolved candidate remains in this assigned set.
- Runtime targeted suite: `pnpm exec vitest run src/views/admin/__tests__/RiskControlView.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/views/admin/orders/__tests__/AdminPaymentPlansView.spec.ts`: 3 files, 26 tests passed (before adding the new time regression).
- New RiskControlView regression test: local `10:23` and `11:47` inputs retain exact minutes in ISO query values, request page 1, and clear to undefined bounds. `pnpm exec vitest run src/views/admin/__tests__/RiskControlView.spec.ts`: 12 tests passed, including the new regression.
- Ticket repair and focused test results: `frontend/src/views/admin/__tests__/TicketDetailView.templates.spec.ts`, 2 tests passed as reported by the shell agent; updated template and focus code independently inspected here.
- Targeted ESLint on `RiskRecordsTable.vue`, `riskControlUtils.ts`, `useRiskControlData.ts` and `RiskControlView.spec.ts` passed.
- This report does not claim a full production payment exercise or cover unrelated API/store/link/anchor audits.
