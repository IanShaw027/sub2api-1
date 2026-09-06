# Preservation Map

| Surface | Static inventory | Required manual proof | Known risk |
|---|---|---|---|
| Router | `routes`, lazy imports, aliases and meta | Guards, redirects, query/params, deep links | Dynamic expressions are not expanded |
| Sidebar / drawer | `navigation`, flags, section data | Default expanded state, keyboard and mobile reachability | Admin `myAccount` is default collapsed |
| Tables | `columns`, handlers, models, API calls | Chooser, default visibility, responsive fallback | Accounts default hidden set grew |
| Anchors | IDs, `data-tour`, `data-testid`, `aria-label` | Tour resolution, focus order, screen-reader labels | Set match can hide relocation |
| Actions | Vue `@handler` bindings and common calls | Disabled/loading/error/confirm/permission states | Indirect handlers require review |
| API | Imported API calls and conventional clients | Payload, auth, errors, retry semantics | Indirect composables are incomplete |
| Visibility | `v-if`, `v-show`, hidden classes, constants | CSS breakpoints, slots, flags, localStorage | Static visibility cannot prove rendering |
| Studio / prompt audit | Current route/navigation/API additions | Exercise all five Studio modes and prompt-audit workflows | Do not call additions losses |

## Interpretation

Removed candidates are triage items, not proof of deletion; moved components and renamed handlers are common. Matching sets are not proof of equivalent behavior, ownership, permissions, order, or visual readability. Default-hidden data must be compared explicitly. Browser evidence is required at 1440px, 1024px, 768px, and 390px in both themes for affected routes.
