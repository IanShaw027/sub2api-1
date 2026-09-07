# `components/ui`

Shared glass-design primitives. Pages MUST compose these instead of hand-rolling
cards, fields, badges, modals, or feedback surfaces — see
`openspec/changes/glass-ui-redesign/ui-standards.md` for the full numeric spec
this table summarizes.

| Component | Sizes / variants | Used on pages |
|---|---|---|
| `GlassCard` | `variant`: glass · solid · transparent · flat; `padding`: sm 12 · md 16–18 · lg 20–24; `hover`, `ring` (`.glass-ring`) | Every page — the base surface for stats, settings, lists, detail panels |
| `StatCard` | padding `14px 16px`; value 28/800 tabular; optional delta pill (`deltaTone` up · down · warn · neutral) + `#sparkline` (96×28) | `/dashboard`, `/admin/dashboard`, `/admin/ops` |
| `MiniStatCard` | single stat or 3-column `items` group; value 22/800 | `/keys` top row, ticket/detail side stats |
| `EndpointCard` | label + `.tag`, mono URL + 28px copy button, description | `/keys` (Anthropic/OpenAI endpoints) |
| `ProgressBar` | 6px track (`showLabel` adds 30px right-aligned %); tone by threshold (≥90 danger, ≥70 warning, else accent) | Accounts capacity/usage windows, quota cards |
| `PageHeader` | `variant`: compact (24/800) · hero (30/800, dot texture); `#actions` slot; <768px actions wrap under a long description | Every page header |
| `FilterBar` | `#search #filters #trailing` slots; collapses to 44px search + drawer toggle <768px | List pages (`/admin/accounts`, `/keys`, `/admin/users`, …) |
| `SettingsSection` | `GlassCard` + `.card-header`; wraps a stack of `SettingRow` | `/admin/settings`, `/profile` |
| `SettingRow` | grid `240px 1fr`, `14px 20px`, 1px row divider; single column <768px | Every settings row across `/admin/settings`, `/profile` |
| `UiModal` | widths sm 440 · md 560 · lg 720 · xl 960; header/body/footer + `#subtitle`; bottom sheet <768px | Create/edit dialogs across list & detail pages |
| `UiDrawer` | `width`: sm 480 · md 640; `side`: right (default) · left; full width <768px | Mobile filter drawers, nav drawer |
| `ConfirmDialog` (`components/common`) | 440px `UiModal`-style wrapper, tone icon (danger/warning/accent) | Delete/disable confirmations everywhere |
| `BaseDialog` (`components/common`) | compatibility wrapper: `width` narrow·normal·wide·extra-wide·full → sm·md·lg·xl | Legacy call sites migrating to `UiModal` |
| `UiPagination` | 28×28 r8 items, accent current page; wraps `common/Pagination` | Every `DataTable` footer |
| `Fab` | 52px r16 accent button, fixed bottom-right | Mobile list pages (`/keys`, `/admin/*` list views) |
| `ChipScroller` | horizontal `.chip-filter`-style row, active = foreground fill | Mobile status/category filters |
| `ListFade` | absolute bottom fade overlay, 120px | Mobile scrollable lists under a FAB |
| `FieldLabel` | `htmlFor`, `hint`, `required`; 12.5/600 label, 12px muted hint, 6px gap below | `TextInput`, key/group forms (`KeyFormModal`) |
| `Button`, `TextInput`, `UiSelect`, `ToggleSwitch`, `Checkbox`, `SegmentedControl`, `StatusBadge` | See each component's own spec | Not owned here — documented for cross-reference only |

## Layout skeletons (`components/layout`)

| Skeleton | Structure | Routes |
|---|---|---|
| `PublicPageLayout` | `.public-page` surface, optional `#nav`, centered content | `/home`, `/login`, `/register`, `/setup`, callback pages |
| `DashboardPageLayout` | `#header #stats #charts #lists` (or default slot), gap 14 | `/dashboard`, `/admin/dashboard`, `/admin/ops`, `/monitor` |
| `TablePageLayout` | fixed actions/filters, scrolling table card, fixed pagination | ListPage routes (`/keys`, `/admin/accounts`, `/admin/users`, …) |
| `DetailPageLayout` | `#header` + `1fr 320px` grid (`#main`, `#side`, `#composer`); 1 col <1024px | `/tickets/:id`, `/invoices/:id`, `/admin/tickets/:id` |
| `SettingsPageLayout` | `#header` + `224px 1fr` grid (`#nav`, `#content`); nav stacks on top <768px | `/admin/settings`, `/profile` |

Page content padding (`8px 24px 24px 20px`) is already applied by
`AppLayout`'s `.app-shell-content` — these skeletons only own the internal
section rhythm (gaps, grid columns, responsive breakpoints), not outer page
padding, to avoid double-padding pages rendered inside `AppLayout`.

## Global surface classes (`src/style.css`)

Pages MUST reach for these instead of writing colour/radius/shadow in
`<style scoped>` (see spec "页面级样式只允许布局声明"):

- Cards: `.glass-card`, `.glass-card-solid`, `.glass-card-flat`, `.glass-ring`,
  `.glass-inset`, `.card-header`, `.card-title`, `.card-subtitle`,
  `.card-body`, `.card-footer`
- Stats/summaries: `StatCard`, `.summary-chip`, `.summary-row` (5/3/1-col
  responsive grid)
- Filters: `.filter-row`, `.filter-search` (260px, 100% <768px), `.filter-count`
- Tables: `.table*`, `.table-footer`
- Overlays: `.modal-*`, `.dialog-*`, `.toast*`
- Feedback: `.notice-*`, `.empty-state*`, `.skeleton`, `.spinner`,
  `.tooltip-bubble`
- Misc: `.code`, `.code-block`, `.log-block`, `.divider`, `.progress*`,
  `.page-header*`, `.section-*`

## Touch targets (<768px)

Icon-only controls (`.icon-btn` 28, `.header-icon-btn` 34, `.btn-icon`, `UiModal`'s close
button 32) keep their visual size and gain a `::after` hit-slop of 8px on every side, so the
tappable area is ≥44px without disturbing table row heights or the 36px filter row. Text
buttons, `.filter-pill`, `ChipScroller` chips, pagination and `UiSelect` triggers stay at
30–36px (WCAG 2.5.8 needs 24px); the mobile drawer's nav items are 44px.

## Focus and control borders

`:focus-visible` draws a 2px `--focus-ring` outline (≥3:1 on the canvas in both themes);
glass primitives replace it with the 3px 18%-accent halo. Controls whose border is their
only boundary (checkbox, switch, radio) use `--border-strong`; decorative card/table edges
keep the prototype `--border` hairline.
