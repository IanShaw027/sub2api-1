# Design Reference Update

Updated 2026-09-07. This tracked addendum supersedes the listed values in the supplied `Sub2API Redesign.dc.html` and `Glass UI Review.dc.html`. The user's original Downloads artifacts remain unchanged. A runnable synchronized copy is provided in `reference/Sub2API Redesign.hardened.dc.html` with its required `support.js`.

| Area | Final contract | Reason |
| --- | --- | --- |
| Light muted | `oklch(50% .006 259.82)` | Readable supporting text on translucent surfaces |
| Structural border | Light 90%, dark 28% | Preserve quiet glass hierarchy |
| Control border | Light 59%, dark 51% | Distinguish input/checkbox/toggle outlines without brightening all dividers |
| Ambient background | One workspace ambient layer | Avoid duplicate light and scroll misalignment |
| Status pulse | One keyframes definition, persistent 3px static halo | Consistent motion and reduced-motion fallback |
| Pagination | Within table container footer | Shared list template, unchanged pagination events |
| Header | 34px controls, one line at compact widths, explicit More access | Stable alignment without losing actions |
| Settings | Shared SettingsPageLayout, semantic rows, one primary save | Preserve dirty/reset/loading and full settings content |
| Information | No newly hidden columns or removed actions | Product/function contract takes priority over visual simplification |

The updated prototype copy carries the token/motion correction and an embedded machine-readable layout contract. It does not rewrite the original prototype's static example markup into a production application. Production screenshots in `output/playwright/glass-hardening/` are the responsive implementation evidence. Therefore this addendum must accompany the reference, rather than claiming the original static mock represents every new responsive behavior.
