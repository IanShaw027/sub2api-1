# Sub2API frontend

Vue 3 (`<script setup>` + TypeScript) · Vite 5 · Tailwind 3 · vue-i18n (zh / en) · Pinia · vitest.

```bash
npm install
npm run dev          # Vite dev server (API proxy target from VITE_DEV_PROXY_TARGET)
npm run build        # vue-tsc -b && vite build
npm run test:run     # vitest
```

## Design system (Glass UI)

The UI follows the "Sub2API Redesign" glass prototype. The numeric spec lives in
`openspec/changes/glass-ui-redesign/ui-standards.md`; deviations from the prototype are
logged in `deviations.md` next to it, and every task's evidence in `verification.md`.

### Tokens and themes

- `src/styles/tokens.css` — all colours as `oklch()` custom properties, mounted on
  `html[data-theme="glass-light" | "glass-dark"]` and `html[data-accent="blue|sky|indigo|teal|violet"]`.
  `src/composables/useTheme.ts` is the single writer of those attributes.
- Semantic tones only: `--accent`, `--success`, `--warning`, `--danger`, `--muted`, plus the
  `-text` variants tuned for 4.5:1 on every surface. Surfaces: `--canvas`, `--background`,
  `--surface`, `--surface-secondary`, `--surface-tertiary`. Borders: `--border` (prototype
  hairline), `--border-strong` (controls whose border is their only boundary), `--focus-ring`.
- `tailwind.config.js` remaps the raw palette names (`red-*`, `emerald-*`, `blue-*`, `zinc-*` …)
  onto those tones, so a stray `text-red-500` still renders on-token — but new code must use the
  semantic scales (`text-danger-text`, `bg-success-100`, `border-line`, `bg-surface-2`,
  `text-muted`; note `accent` has no `-text` step — use `text-accent-600/700`). `npm run lint:ui`
  (`ui-lint.mjs --scoped --palette`) enforces this over the whole tree and
  `src/__tests__/designTokens.spec.ts` runs the same gate under vitest.
- Typography: `--display` (Manrope) for titles, body `Inter`, mono `JetBrains Mono`; the type
  scale is in `ui-standards.md` §2.

### Components and page skeletons

- `src/components/ui/` — glass primitives (`GlassCard`, `StatCard`, `PageHeader`, `FilterBar`,
  `SettingsSection`/`SettingRow`, `UiModal`, `UiDrawer`, `UiSelect`, `Button`, …). See its
  `README.md` for sizes and which pages use each.
- `src/components/layout/` — page skeletons (`TablePageLayout`, `DashboardPageLayout`,
  `DetailPageLayout`, `SettingsPageLayout`, `PublicPageLayout`) and the app shell.
- `src/components/common/` — data table, pagination, dialogs, fields, badges; recipes in its
  `README.md`.
- Global surface classes (`.glass-card`, `.btn btn-primary`, `.field`, `.badge-*`, `.filter-pill`,
  `.icon-btn`, `.page-header`, …) live in `src/style.css`. Page-level `<style scoped>` blocks may
  only contain layout declarations.

### Breakpoints

| Width | Behaviour |
|---|---|
| ≥ 1024 | prototype desktop geometry (sidebar 212, content left 244, h1 top 74) |
| 768–1023 | sidebar collapses to a 60px icon rail; stats grids drop to 2–3 columns |
| < 768 | single column, bottom-sheet modals, FAB + `ChipScroller` filters, `FilterBar` collapses to a 44px toggle, icon-only buttons get a 44px hit-slop |

### Quality gates

All of these run from `frontend/`; the mock harness below must be up for the browser-based ones.

| Command | What it checks |
|---|---|
| `npm run typecheck` / `npm run lint:check` / `npm run test:run` | vue-tsc · eslint · vitest |
| `npm run lint:ui` (`node scripts/ui-lint.mjs --scoped --palette [files]`) | legacy classes, raw colour literals, raw palette classes, non-layout declarations in `views/**` scoped styles; whole tree when no files are given |
| `npm run check:contrast` | WCAG text (4.5:1) and non-text (3:1) pairs across both themes, badges and tooltips included |
| `npm run i18n:diff` | zh/en key parity and duplicate keys |
| `node scripts/anchor-diff.mjs <file.vue> --base <rev> --scope src` | no lost `data-testid` / `id` / `aria-label` / handlers / `v-model` / `t()` keys after a rewrite (`--scope src` so anchors that moved into extracted components count as kept) |
| `node scripts/keyname-leak.mjs [--width 390]` | walks every route in `scripts/ui/routes.txt`, fails on rendered raw i18n keys; with `--width 390` also fails on horizontal overflow |
| `scripts/ui/ui-shots.sh all\|desktop\|mobile` | screenshot matrix into `screens/<route>/{d,m}-{light,dark}.png` |
| `node scripts/ui/pixel-diff.mjs` | diff against the prototype boards in `openspec/changes/glass-ui-redesign/reference/` |

### Mock harness for screenshots

```bash
node scripts/mock/server.js                                     # seeded API on :8091
VITE_DEV_PROXY_TARGET=http://127.0.0.1:8091 npx vite --port 3777
UI_SHOTS_PROBE='h1|.glass-card' node scripts/ui/shot.cjs <name> <route> 1440 900 <role> glass-light zh 1
UI_SHOTS_PROBE='tbody tr' UI_SHOTS_PROBE_ALL=1 node scripts/ui/shot.cjs rows /admin/orders 1440 900 admin glass-light zh 1
```

`shot.cjs` seeds the role/theme/locale through the mock's `/setup/seed`, prints the bounding
boxes of the probed selectors plus `scrollWidth`, and writes `.shots/<name>.png`. Always verify
against seeded data — empty states hide row-height and overflow problems. `UI_SHOTS_PROBE_ALL=1`
prints a height histogram over every match (`58x1 59x17`), which is the table row-height gate
(ListPage rows must stay ≤ 61px).
