# Preservation Review Findings

## Admin personal navigation is collapsed by default (P1, resolved)

`frontend/src/constants/sidebar.ts` now returns `true` for all sections. Explicit persisted user choices still win, so `/keys`, `/profile`, billing and support items remain discoverable by default without removing collapse preference.

## Accounts default column visibility changed (P1, resolved)

`useAccountColumnVisibility.ts` restores the pre-Glass default hidden set and no longer force-hides readable fields during the theme migration. The legacy scheduler-score opt-in behavior remains preserved because it controls an expensive backend query.

## Deleted files need migration evidence (P1)

The branch deletes or moves account/payment/ticket/key components such as `AccountStatsModal.vue`, `PaymentQRDialog.vue`, and `EndpointPopover.vue`. Lazy replacements may exist, but every deletion needs a source-to-target mapping and targeted workflow test. The audit reports file changes separately from inventory sets.

## Audit limitations

The script does not claim complete coverage of dynamic navigation, computed columns, CSS breakpoints, permissions, localStorage state, modal focus, network payloads, or runtime behavior. Browser/test evidence is required before marking preservation complete.
