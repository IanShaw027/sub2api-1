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

The full automated suite is green. `ws` is now declared as a pinned frontend dev dependency and `shot.cjs` supports `UI_SHOTS_CHROME` plus automatic Chromium discovery. Core screenshots were captured for Dashboard (desktop/mobile), Keys (tablet/mobile), and Settings (dark desktop); the Keys tablet layout was corrected after visual inspection so summary cards no longer clip. The complete route screenshot matrix remains a follow-up because it is broader than this implementation batch.
