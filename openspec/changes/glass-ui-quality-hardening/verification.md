# Verification

## Required commands

```bash
cd frontend
pnpm run typecheck
pnpm run lint:check
pnpm run lint:ui
pnpm run check:contrast
pnpm run i18n:diff
pnpm run test:run
```

## Required evidence

- `route-diff.json`: before/after route set
- `nav-diff.json`: user/admin/mobile navigation set
- `column-diff.json`: affected table columns/default visibility
- `anchor-diff.json`: `data-tour`, `data-testid`, id and aria labels
- Screenshots for `/dashboard`, `/admin/dashboard`, `/admin/accounts`, `/keys`, `/admin/settings`, `/login`, `/home`
- Each core route: light/dark at 1440px and 390px; tablet checks at 1024px and 768px
- Interaction screenshots: one modal, one dropdown, one toast, one empty state, one populated table

## Acceptance thresholds

- No unexplained route, navigation, column, action or anchor loss.
- No text overlap or clipped primary action in the required viewport matrix.
- No duplicate P0 issue remains.
- All required commands pass under the pinned pnpm version.
- Any intentional visual deviation is recorded in `deviations.md` and synchronized to the design reference.

## Current verification note

The final automated suite is green: `vue-tsc --noEmit`, UI lint, contrast, i18n parity, the preservation-audit tests, 372 Vitest files / 2649 tests, and the production build all passed. ESLint returned 0 errors and 16 warnings in the seeded mock's existing unused fixture fields. The final preservation audit reports equal route/navigation/column/API/store inventories; remaining action/link/menu candidates are covered by the equivalence ledgers and targeted tests.

The 56 core matrix screenshots and interaction evidence were generated and reviewed. After the final 768px Settings layout correction, both `admin-settings/t768-light.png` and `admin-settings/t768-dark.png` were recaptured against the seeded mock and reviewed. Their diagnostics report viewport width 768, `scrollWidth` 768, no page errors, no error toasts and no broken images; the checker overlay node is present but hidden.
