# Glass UI Migration Checklist

Branch: `feat/glass-ui-redesign` (base: `personal-main`)

## Verification gates (Phase 3 exit)

| Gate | Status | Notes |
|------|--------|-------|
| `rg "dark:" frontend/src --glob '*.vue'` | ✅ 0 matches | Completed in p3 cleanup |
| `rg "bg-gray-" frontend/src` | ✅ 0 matches in `.vue` | Completed in p3 cleanup |
| `pnpm run typecheck` | ✅ | |
| `pnpm run test:run` | ✅ 2051 tests | |
| `html.dark` dual-track removed | ✅ `22db4f47b` | `useTheme` uses `data-theme` only |
| Settings mobile nav (`<768` dropdown) | ✅ | `SettingsView.vue` |

## Phase commits

| Phase | Commits |
|-------|---------|
| 1 — Tokens + shell | `cddbd355e`, `05fc5401f`, `115093940`, `eb1a10d73` |
| 2 — `ui/` library | `954da13b1`, `18e826873`, `41ee7cae9` |
| 3 — Page batches | `2f0024263` … `84f7e1067`, `3a3129e6a` … `22db4f47b` |
| 4 — Creation center | `2be1db2f3`, `3573bffd9`, `6eb56123d`, `4fbd3bdea` |

## Route checklist

Legend: **Template** = design pattern; **Batch** = migration batch; **390** = narrow layout reviewed.

### Batch 1 — pixel baseline

| Route | Template | Batch | Commit | 390 |
|-------|----------|-------|--------|-----|
| `/home` | Landing / custom | 1 | `2f0024263` | ☐ manual |
| `/login` | AuthLayout | 1 | `2f0024263` | ☐ manual |
| `/admin/dashboard` | DashboardPage | 1 | `2f0024263` | ☐ manual |
| `/admin/accounts` | ListPage | 1 | `2f0024263` | ☐ manual |
| `/keys` | ListPage | 1 | `2f0024263` | ☐ manual |
| `/admin/settings` | SettingsSection | 1 | `2f0024263` | ☐ manual |

### Batch 2 — admin lists

| Route | Template | Batch | Commit | 390 |
|-------|----------|-------|--------|-----|
| `/admin/users` | ListPage | 2 | `9a4741dd5` | ☐ |
| `/admin/groups` | ListPage | 2 | `9a4741dd5` | ☐ |
| `/admin/subscriptions` | ListPage | 2 | `9a4741dd5` | ☐ |
| `/admin/proxies` | ListPage | 2 | `9a4741dd5` | ☐ |
| `/admin/channels/pricing` | ListPage | 2 | `9a4741dd5` | ☐ |
| `/admin/channels/monitor` | ListPage | 2 | `9a4741dd5` | ☐ |
| `/admin/plugins` | ListPage | 2 | `9a4741dd5` | ☐ |
| `/admin/announcements` | ListPage | 2 | `9a4741dd5` | ☐ |
| `/admin/redeem` | ListPage | 2 | `9a4741dd5` | ☐ |
| `/admin/promo-codes` | ListPage | 2 | `9a4741dd5` | ☐ |
| `/admin/usage` | ListPage | 2 | `9a4741dd5` | ☐ |
| `/admin/audit-logs` | ListPage | 2 | `9a4741dd5` | ☐ |
| `/admin/risk-control` | ListPage | 2 | `9a4741dd5` | ☐ |
| `/admin/prompt-audit` | FeaturePage | 2 | `9a4741dd5` | ☐ |
| `/admin/affiliates/*` | ListPage | 2 | `9a4741dd5` | ☐ |
| `/admin/orders/*` | ListPage | 2 | `9a4741dd5` | ☐ |
| `/admin/tickets` | ListPage | 2 | `9a4741dd5` | ☐ |

### Batch 3 — user lists + dashboards

| Route | Template | Batch | Commit | 390 |
|-------|----------|-------|--------|-----|
| `/dashboard` | DashboardPage | 3 | `5e9b31d04` | ☐ |
| `/usage` | ListPage | 3 | `5e9b31d04` | ☐ |
| `/orders` | ListPage | 3 | `5e9b31d04` | ☐ |
| `/invoices` | ListPage | 3 | `5e9b31d04` | ☐ |
| `/tickets` | ListPage | 3 | `5e9b31d04` | ☐ |
| `/subscriptions` | ListPage | 3 | `5e9b31d04` | ☐ |
| `/available-channels` | ListPage | 3 | `5e9b31d04` | ☐ |
| `/admin/ops` | DashboardPage | 3 | `5e9b31d04` | ☐ |
| `/admin/orders/dashboard` | DashboardPage | 3 | `5e9b31d04` | ☐ |
| `/monitor` | FeaturePage (v1/v2) | 3 | `5e9b31d04` | ☐ |

### Batch 4 — detail, auth, payment, misc

| Route | Template | Batch | Commit | 390 |
|-------|----------|-------|--------|-----|
| `/tickets/:id`, `/tickets/new` | DetailPage | 4 | `84f7e1067` | ☐ |
| `/invoices/:id` | DetailPage | 4 | `84f7e1067` | ☐ |
| `/register`, `/email-verify` | AuthLayout | 4 | `84f7e1067` | ☐ |
| `/forgot-password`, `/reset-password` | AuthLayout | 4 | `84f7e1067` | ☐ |
| `/auth/dingtalk/email-completion` | AuthLayout | 4 | `84f7e1067` | ☐ |
| `/purchase`, `/payment/*` | PaymentFlow | 4 | `84f7e1067` | ☐ |
| `/auth/*/callback` (7 routes) | CallbackStatus | 4 | `84f7e1067` | ☐ |
| `/setup` | SetupWizard | 4 | `84f7e1067` | ☐ |
| `/custom/:id` | EmbedPage | 4 | `84f7e1067` | ☐ |
| `/*` NotFound | NotFound | 4 | `84f7e1067` | ☐ |
| `/model-plaza` | ModelPlaza | 4 | `84f7e1067` | ☐ |
| `/profile`, `/redeem`, `/affiliate` | Profile/List | 4 | `84f7e1067` | ☐ |
| `/batch-image` | ListPage | 4 | `84f7e1067` | ☐ |

### Phase 4 — Creation center (new)

| Route | Template | Batch | Commit | 390 |
|-------|----------|-------|--------|-----|
| `/studio` | StudioPage (3-col) | 4+ | `3573bffd9` | ✅ responsive CSS |

## 390px review notes

- **Shell**: mobile drawer `<768px`, tablet icon rail `768–1023px` (`05fc5401f`, `115093940`).
- **Studio** (`/studio`): single-column stack ≤1100px; composer sticky bottom ≤767px; session list capped 180px height on mobile.
- **Batch 1 baseline pages** (`/home`, `/admin/dashboard`, `/keys`): require manual screenshot compare against design refs 01–06, 08 when available.

## Remaining optional work

- [ ] Manual 390px screenshot pass for all routes (checkboxes above).
- [ ] Wire creation-center `fetch` calls through token-refresh helper (medium priority).
- [ ] CI: ensure `pnpm run lint:check` passes after `OpenAIOAuthCapacityDialog.spec.ts` fix.
