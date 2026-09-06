# Interaction Verification

Date: 2026-09-07 (Asia/Taipei). Browser: isolated Playwright Chrome session
`glass-a11y`; frontend `http://127.0.0.1:3777`, seeded mock `:8091`.
No production account, credentials, or writes were used.

## Static Audit Contract

`frontend/scripts/preservation-audit.mjs` schema 2 now inventories links,
object-based menu actions, and imported store calls in addition to template
handlers, routes, navigation, columns, anchors and API calls. It reports both:

- `removedCandidates`: repository-wide value differences, useful for moves.
- `removedOccurrences`: file + value + API/store arguments + occurrence count,
  so another page's same-name button cannot hide a lost local action.

Commands (from `frontend`):

```sh
node --test scripts/preservation-audit.test.mjs
node scripts/preservation-audit.mjs --out /tmp/hardening-audit.json
node scripts/preservation-audit.mjs --base pre-glass --out /tmp/pre-glass-audit.json
```

Six parser/diff tests pass, including aliases, portal-independent template
links, same-name operations on different pages, duplicate handlers, and API
argument changes. Parse errors fail the audit rather than silently yielding
empty inventories. This is not proof of runtime success: indirect composables,
destructured store methods, dynamic slots, authorization and CSS require review.

The initial hardening comparison had no removed per-file anchor, link,
menu-action, API or store occurrences. The one removed dashboard event
`change:loadCharts` maps to `change:onDateRangeChange`, which invokes both
`loadCharts` and `loadRecent`. Later wrapper/field migrations need final audit
refresh. Pre-glass differences are tracked separately in
`legacy-equivalence.md`; its unresolved entries must not be treated as passed.

## Completed Browser Checks

| Surface | Interaction and observed result | Evidence |
| --- | --- | --- |
| Header, 1024 x 900 | Header content width 800px; search height 34px; no document horizontal overflow. More opens with focus on Docs; visible command labels retained. | `header-locale-1024.png` |
| Nested locale menu | Open locale inside More; first Escape closes only locale, restores locale trigger and keeps More expanded. Second Escape closes More and restores More trigger; actual `aria-expanded` transitions are `true`, then `false`. | `header-locale-1024.png` and browser keyboard checks |
| Teleported support dialog | Open support from More, click inside dialog, Escape; More remains expanded and focus returns to the visible support trigger. | `support-modal-1024.png` |
| Mobile drawer, 390 x 844 | Open focuses close; Shift+Tab and 45 Tab steps remain inside drawer; Escape removes drawer after transition and returns focus to hamburger. | `mobile-drawer-390.png` |
| Accounts mobile filtering | Search issues `/admin/accounts?...search=openai`; mock returns zero items and UI renders no-data; clearing restores eight accounts. No page overflow. | Browser network response and DOM |
| Accounts bulk editing | Select one Claude account, open bulk edit; available fields and batch update button render. Shift+Tab from close wraps to batch update; Tab wraps to close. Escape restores bulk-edit trigger. No cross-platform batch was performed. | `accounts-bulk-390.png` |
| Accounts export | Select one account, open Import/Export, Export Selected, confirm; real browser download event produces `sub2api-account-20260907015208.json`. | Isolated mock download |
| Accounts import | Import menu opens file-upload dialog with Cancel and Start Import. No data import submitted. | `accounts-import-390.png` |
| Keys mobile filtering | Filters expands to group and status choices; refresh, column chooser, create FAB and original card actions remain present. | Browser DOM |
| Keys create modal | Create opens title, name, group, custom key, IP restriction, quota, rate limit and expiry controls with Cancel/Create footer. | `keys-create-390.png` |
| Keys Select portal | Open group search; Tab resumes at custom-key switch; Shift+Tab returns to Name. Escape closes modal and returns focus to visible create FAB. | Real keyboard checks plus `Select.spec.ts` |
| Keys mobile endpoints | Default Anthropic and configured OpenAI endpoint, full URLs and configured description render. Copy produces success toast; both encoded `tcptest.cn` links are present. No document overflow. | `keys-endpoints-390.png` |
| Keys mobile pagination | Browser-only interception changes mock metadata to `total=40, pages=2`. Next sends `page=2&page_size=20`, renders page 2/2 and disables Next. Previous sends page 1, renders page 1/2 and disables Previous. Interception is removed afterward; it does not validate server-side slicing. | Browser network requests and DOM |
| Usage mobile export and errors | Export CSV produces browser download `usage_2026-09-06_to_2026-09-07.csv`; switching to Errors retains key/model/category/status filters and renders explicit empty state. No document overflow. | `usage-errors-390.png` |
| Settings mobile sections | Change Site Name enables Reset; Reset restores original value. Native section select opens Security and updates hash to `#security`. No document overflow. | `settings-security-390.png` |
| Settings mobile save | Save sends `PUT /api/v1/admin/settings` with entered `site_name`. After loading there is exactly one Save Changes button and Reset is disabled. Mock echoes the payload without persisting settings. | Browser request payload and DOM |
| Reduced motion | Playwright `emulateMedia({ reducedMotion: 'reduce' })` matches the media query. More animation and transition durations both compute to `0.00001s`; Escape restores the trigger. | Browser computed style and keyboard checks |
| Groups restored actions, 390 and 768 | Four rendered groups retain Edit/Copy/Exclusive Rate/Exclusive RPM/Delete. Exclusive RPM is reachable after scroll at x=127 (390 viewport), x=676 (768 viewport), 28px wide; no document overflow. | `groups-actions-390.png`, `groups-actions-768.png` |

Screenshots live in `output/playwright/glass-hardening/interactions/`.
They supplement, not replace, the full light/dark viewport matrix.

## Fixes From Verification

- Header compact CSS now follows actual header width instead of only viewport
  width, and secondary commands have visible labels in More.
- Locale trigger has explicit name/expanded/controls state, keyboard focus
  entry and nested Escape restoration. Escape is consumed only while its
  submenu is open; a second Escape reaches the parent More container.
- Support exposes dialog-open state so its teleported modal does not close
  the parent More menu and hide its return-focus target.
- Select restores its trigger before native Tab traversal from the body
  portal. This preserves correct order with the shared modal focus trap.

## Boundaries

Component regression coverage: `LocaleSwitcher.spec.ts` (1), `Select.spec.ts`
(10), `SupportQRCodesButton.spec.ts` (6), all pass. ESLint passed on changed
audit/interaction sources and tests. The existing mobile-shell source test was
narrowed to reject only standalone `.header-icon-btn` redefinitions, while
allowing a contextual More-menu descendant rule.

The seeded mock intentionally acknowledges unsupported routes and is not a
backend contract test. An export download proves the UI command chain, not
production data correctness. These checks do not claim screen-reader output,
every permission/feature-flag combination, account mutation success, or full
coverage of every historical action. Tasks 4.5-4.6 have representative browser
evidence above; tasks 4.3-4.4 additionally require the complete legacy
equivalence reports. Pagination and bulk editing are not features of Settings;
Usage has no bulk write operation, and Keys has no bulk operation in the
pre-glass baseline. Those checks are not invented as new features.

The Groups supplementary check initially observed mock-only usage/capacity
summary array-shape errors. Its screenshots prove restored actions and
layout, not correctness of the placeholder zero usage/capacity values; the
mock maintainer was notified to provide the missing summary fixtures.
