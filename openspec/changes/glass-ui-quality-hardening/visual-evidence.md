# Visual Evidence

Reviewed on 2026-09-07 against the current working tree, using local seeded mock data. This document records visual evidence, not production integration certification or a claim that all 64 routes were tested.

## Scope

Evidence root: `output/playwright/glass-hardening/` (repository-relative).

| Route | Artifact folder | 1440 light/dark | 1024 light/dark | 768 light/dark | 390 light/dark |
| --- | --- | --- | --- | --- | --- |
| `/home` | `home` | Captured/reviewed | Captured/reviewed | Captured/reviewed | Captured/reviewed |
| `/login` | `login` | Captured/reviewed | Captured/reviewed | Captured/reviewed | Captured/reviewed |
| `/dashboard` | `dashboard` | Captured/reviewed | Captured/reviewed | Captured/reviewed | Captured/reviewed |
| `/admin/dashboard` | `admin-dashboard` | Captured/reviewed | Captured/reviewed | Captured/reviewed | Captured/reviewed |
| `/admin/accounts` | `admin-accounts` | Captured/reviewed | Captured/reviewed | Captured/reviewed | Captured/reviewed |
| `/keys` | `keys` | Captured/reviewed | Captured/reviewed | Captured/reviewed | Captured/reviewed |
| `/admin/settings` | `admin-settings` | Captured/reviewed | Captured/reviewed | Captured; narrow-form defect found | Captured/reviewed |

The core matrix contains 56 PNGs and matching diagnostic JSON sidecars. Names are `d-light`, `d-dark`, `t1024-light`, `t1024-dark`, `t768-light`, `t768-dark`, `m-light`, `m-dark`. Viewports are 1440x900, 1024x900, 768x1024 and 390x844. Full-page screenshots include off-screen content; fixed footers/FABs retain their viewport position in the resulting tall image.

Additional coverage:

- `auth/`: Register, Forgot Password and Reset Password, each at 1440/390 in both themes (12 PNGs). Reset uses an explicit mock `email` and `token` query so it captures the actual reset form, not the invalid-link fallback.
- `profile/`: 1440/390 in both themes (4 PNGs), after restoration of role, status, balance, concurrency and membership date.
- `interactions/`: More/locale menu, support modal, mobile drawer, account import/bulk actions, Keys creation, Usage filters, Settings security. Interaction ownership and assertions are detailed in the functional/accessibility evidence.
- `interactions/keys-empty-light-1440.png` and `keys-empty-dark-1440.png`: browser-intercepted `/api/v1/keys` response with `{items:[],total:0,page:1,page_size:20,pages:1}`; actual application empty state, not injected DOM.
- `interactions/keys-copy-toast-light-1440.png` and `keys-copy-toast-dark-1440.png`: real endpoint Copy click; visible toast text was `已复制`.

## Corrections Driven By Screenshots

1. The old screenshot helper removed Vite error overlays. It now preserves overlays, emits diagnostics, and rejects visible overlays, runtime exceptions, unexpected redirects and error toasts.
2. The checker briefly retained a fixed TextInput type error during concurrent edits. The owned Vite process was restarted; affected artifacts were replaced rather than accepted.
3. Settings exposed `Cannot read properties of undefined (reading 'prefix')`. This was traced to missing mock backup endpoints, whose fallback was a paginated object. Fixtures now match the existing backup API contracts for image storage, S3 settings, schedules and backup lists. Production code was not weakened to accept malformed mock data.
4. Off-screen Chart.js canvases could be captured before series finished drawing. Final user dashboard captures explicitly wait for canvas attachment, scroll it into view, wait for rendering, then return to the top. Final canvas checks show 11,637-56,222 colored pixels across the user matrix. Administrator mobile charts likewise have more than 9,300 colored pixels. The scrolled chart captures remain as supplementary evidence.
5. Administrator mobile request-trend labels overlapped. The owning component now samples axis labels based on width while keeping all bars and their hover information; mobile screenshots were replaced after the fix.
6. Keys and Accounts screenshots were replaced after restoration of direct actions. Keys mobile keeps endpoint copy/speed-test, description, group picker, rate windows/reset and direct row actions. Wide desktop tables use local scrolling, not deletion of selected columns.
7. Settings at 768px still exposed a narrow-form layout: side navigation plus a two-column setting row left insufficient width for the logo upload control and homepage textarea. This is a real responsive defect and must not be marked passed until corrected and re-captured.

## Reproduction

Start the existing mock backend on 8091 and Vite with `VITE_DEV_PROXY_TARGET=http://127.0.0.1:8091` on 3777. Use the pinned project pnpm version for dependency checks.

```bash
cd frontend
UI_SHOTS_CHROME="$HOME/Library/Caches/ms-playwright/chromium_headless_shell-1234/chrome-headless-shell-mac-arm64/chrome-headless-shell" \
UI_SHOTS_ONLY=home,login,dashboard,admin-dashboard,admin-accounts,keys,admin-settings \
UI_SHOTS_OUT=../output/playwright/glass-hardening \
bash scripts/ui/ui-shots.sh matrix
```

The helper supports `UI_SHOTS_READY_SELECTOR`; Settings defaults to `#settings-form` so its initial skeleton does not count as final content. For chart review, additionally scroll each canvas into view and wait for its series to render before the full-page capture. A nonempty canvas containing only grid lines is not sufficient; use the supplied visual evidence and pixel diagnostic alongside the route/theme checks.

## Documentation And Limits

- `ui-standards.md` now records muted 50%, structural/control border separation, single pulse and ambient layer, table-footer pagination and compact header behavior.
- `design-reference-update.md` records differences from the historical design. A synchronized runnable copy is in `reference/Sub2API Redesign.hardened.dc.html`; the original Downloads design was not overwritten.
- Auth `VISUAL_GUIDE.md`, `USAGE_EXAMPLES.md` and layout `EXAMPLES.md` no longer prescribe the legacy color utility palette. Their canonical links point to the shared Glass contract.
- OpenSpec uses `specs/glass-quality-hardening/spec.md` with ADDED requirements and SHALL/scenario structure. Cached OpenSpec CLI strict validation passed.
- Real provider OAuth/CAPTCHA, email delivery, hardware passkeys, payment, upstream account mutation and production data behavior are not proved by these fixtures or static screenshots. Cross-browser and real-device performance results are not claimed.
- Final unit/build/lint/preservation status is maintained by the root verification report, not inferred from these images.
