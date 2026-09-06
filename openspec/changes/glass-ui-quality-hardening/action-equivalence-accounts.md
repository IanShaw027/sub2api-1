# Account And Group Action Equivalence

## Scope And Method

Baseline: `f1c8ab7da6284179461613cec8a30a8f87f2c545`. Candidate IDs refer to `actions-candidates.json`, generated before these repairs. This audit covers exactly 121 candidates: `action-004` through `action-094` and `action-169` through `action-198`.

`source-equivalent` below means the old event, reachable replacement control, host binding, and replacement state mutation were inspected. It is not an end-to-end or visual pass. Component paths below are relative to `frontend/src/components/`; view paths are relative to `frontend/src/views/`. Lines refer to the current working tree at review time and may move after integration.

Two concrete default-visibility regressions were confirmed against old source: group sorting and the formerly inline group row actions moved into More menus. This change restores explicit sort and row buttons in `views/admin/GroupsView.vue`; existing callbacks, permissions/platform condition, duplicate loading state, and delete confirmation are retained. The restored row buttons include delete as well as composite routes, rate multipliers, and RPM overrides.

## Shared Evidence

- `account/shared/InlineToggleSwitch.vue:49` calls `onClick`, emits click, and assigns `modelValue.value = !modelValue.value` when the default `autoToggle: true` applies. All switch rows below use that default and bind to the old boolean through `defineModel` and host `v-model`.
- Create hosts remain mounted in `account/CreateAccountModal.vue`: platform selector at 39, Gemini at 90, Antigravity at 105, Vertex at 116, API key at 130, Bedrock at 166, quota card at 187, Grok URL at 212, temporary rules at 242, quota control at 258, TLS at 278, advanced options at 312, footer at 436, help dialog at 451.
- Edit hosts remain mounted in `account/EditAccountModal.vue`: API-key/OAuth section at 50, Bedrock section at 162, temporary rules at 202, advanced options at 227. The advanced section receives the shared `quotaNotifyState` object at 290 and each boolean via explicit `v-model`.
- Group create/edit hosts remain mounted in `views/admin/GroupsView.vue` at `GroupCreateModal` and `GroupEditModal`. Their event bindings retain `created/updated -> loadGroups`, `unsupported-live -> pendingLiveForm`, and close/open state. `confirmUnsupportedLive` at 832 delegates to exposed `confirmLive` before clearing pending state.
- This document intentionally does not cover API/store/link candidates, unrelated account components, or the separate 31-anchor review.

## CreateAccountModal Candidates

| Candidate IDs | Old action | Reachable replacement and binding evidence | Classification |
|---|---|---|---|
| action-004, action-005, action-006, action-007, action-008, action-009 | Set `form.platform` to anthropic/openai/gemini/antigravity/grok/kiro | `account/create/PlatformSelector.vue:8,21,46,71,84,97` sets `platform`; `defineModel('platform')` at 170; host `v-model:platform="form.platform"` at 41 | source-equivalent |
| action-010, action-011, action-012 | `selectCNPlatform(kimi/zhipu/deepseek)` | Selector buttons at 113,126,139 emit `selectCnPlatform` with the same provider; host `@select-cn-platform="selectCNPlatform"` at 42 | source-equivalent |
| action-013 | Open Gemini help | `account/platform/GeminiPanel.vue:7` sets `showHelpDialog`; model declared at 403; host binds `showGeminiHelpDialog` at 95 | source-equivalent |
| action-014, action-015, action-016 | Select Google One / Code Assist / AI Studio OAuth | GeminiPanel buttons at 151,194,277 call `selectOAuthType`; implementation at 416 preserves the AI Studio configuration guard, error toast, and selected type assignment; host binds `geminiOAuthType` at 93 | source-equivalent |
| action-017, action-018 | Select Antigravity OAuth/upstream | `account/platform/AntigravityPanel.vue:7,33` sets `accountType`, model at 215; host binds `antigravityAccountType` at 108 | source-equivalent |
| action-019 | Read selected Vertex JSON file | `account/platform/VertexServiceAccountPanel.vue:10` -> `handleFileChange` at 114 -> `file.text()` -> `file-text-received`; host at 122 invokes `applyVertexServiceAccountJson`. Parser moved unchanged to `account/create/useCreateAccountVertexServiceAccount.ts:15`; input reset remains in finally | source-equivalent |
| action-020, action-021, action-022 | Vertex drag enter/over/leave feedback | Vertex panel at 19,20,21 toggles local `dragActive`; same drag prevent modifiers and visual state, no form field lost | source-equivalent |
| action-023 | Drop Vertex JSON file | Vertex panel at 22 -> `handleDrop` at 125 resets drag state, reads first file, emits text; same parent JSON validator as action-019 | source-equivalent |
| action-024 | Open Vertex file picker | Vertex panel at 37 invokes `fileInputRef?.click()`; ref is attached to the hidden file input at 5 | source-equivalent |
| action-025, action-027 | Toggle API-key/Bedrock pool mode | `account/create/CreateApiKeySection.vue:101` and `CreateBedrockCredentialsSection.vue:139` mount `PoolModeSection`; `shared/PoolModeSection.vue:10,67` binds switch/model; both section hosts bind the original `poolModeEnabled` | source-equivalent |
| action-026 | Toggle custom error-code filtering | CreateApiKeySection at 108 mounts `CustomErrorCodesSection`; `shared/CustomErrorCodesSection.vue:10,100` binds switch/model to host `customErrorCodesEnabled` | source-equivalent |
| action-028, action-029, action-030, action-031, action-032, action-033, action-034, action-035, action-036 | Daily/weekly/total notification enabled, threshold, threshold type (first quota branch) | `shared/QuotaLimitCardSection.vue:32` through 40 forwards all nine `update:quotaNotify*` events; models at 84 through 92; Create host at 201 through 209 uses matching `v-model` on `quotaNotifyState.daily/weekly/total` fields | source-equivalent |
| action-037, action-038, action-039, action-040, action-041, action-042, action-043, action-044, action-045 | Same nine notification updates (second quota branch) | Same forwarding chain as action-028..036. Old Anthropic/non-Anthropic quota blocks are unified under host `form.type === 'apikey' || form.type === 'bedrock'` at 189; the hint remains conditional by platform. No notification branch is excluded | source-equivalent |
| action-046 | Toggle Grok OAuth custom base URL | `shared/GrokCustomBaseUrlSection.vue:10,35` switch/model; host at 215 binds `grokOAuthCustomBaseUrlEnabled` and retains Grok OAuth guard | source-equivalent |
| action-047 | Toggle temporary unschedulable rules | `shared/TempUnschedulableRulesSection.vue:11,149` switch/model; host at 244 binds original `tempUnschedEnabled` and retains rule mutation callbacks | source-equivalent |
| action-048, action-049, action-050 | Toggle window cost / session limit / RPM limit | `shared/QuotaControlPanel.vue:19,65,108` switches with models at 271,274,277; host quota-control block binds original three fields | source-equivalent |
| action-051, action-052, action-053 | Toggle session masking / cache TTL override / custom base URL | QuotaControlPanel at 207,220,246; models at 282,283,285; host at 271,272,274 binds original fields | source-equivalent |
| action-054 | Toggle TLS fingerprint | `shared/TlsFingerprintPanel.vue:11,83` switch/model; host at 280 binds `tlsFingerprintEnabled` | source-equivalent |
| action-055 | Toggle Anthropic passthrough | `create/CreateOpenAIAnthropicOptionsSection.vue:69,313` switch/model; host at 321 binds `anthropicPassthroughEnabled` | source-equivalent |
| action-056 | Toggle pause-on-expiry | Host at 356 directly uses InlineToggleSwitch `v-model="autoPauseOnExpired"`; same ref remains used by submit/reset logic | source-equivalent |
| action-057 | Return to basic information | `create/CreateAccountFooter.vue:43` emits `back`; host at 445 invokes `goBackToBasicInfo`; `create/useCreateAccountOAuthFlows.ts:153` resets step to 1 and all six OAuth states plus OAuth flow ref, matching old function | source-equivalent |
| action-058, action-059 | Close Gemini help via dialog or button | `create/GeminiHelpDialog.vue:7,212` sets `show=false`; `defineModel('show')` at 230; host at 451 binds `showGeminiHelpDialog` | source-equivalent |

## EditAccountModal Candidates

| Candidate IDs | Old action | Reachable replacement and binding evidence | Classification |
|---|---|---|---|
| action-060, action-063 | Toggle API-key/Bedrock pool mode | `edit/EditApiKeyOAuthFieldsSection.vue:163` and `EditBedrockCredentialsSection.vue:143` mount shared PoolModeSection; each binds same `poolModeEnabled` passed from Edit host | source-equivalent |
| action-061 | Toggle custom error codes | EditApiKeyOAuthFieldsSection at 170 mounts shared CustomErrorCodesSection; same switch/model chain as create action-026 | source-equivalent |
| action-062 | Toggle Grok custom URL | EditApiKeyOAuthFieldsSection at 201 binds shared Grok URL component; model at 283; Edit host at 67 binds original `grokOAuthCustomBaseUrlEnabled` | source-equivalent |
| action-064 | Toggle temporary unschedulable rules | Edit host at 202 mounts shared TempUnschedulableRulesSection and binds `tempUnschedEnabled`; shared switch at 11 and model at 149 | source-equivalent |
| action-065 | Toggle Anthropic passthrough | `edit/EditAdvancedOptionsSection.vue:207` uses InlineToggleSwitch; model at 775; Edit host at 233 binds same field | source-equivalent |
| action-066, action-067, action-068, action-069, action-070, action-071, action-072, action-073, action-074 | First branch daily/weekly/total notification fields | EditAdvancedOptionsSection at 267 through 275 binds all nine notification fields; shared quota wrapper forwards all nine updates at 32 through 40; host passes original reactive object at 290 | source-equivalent |
| action-075, action-076, action-077, action-078, action-079, action-080, action-081, action-082, action-083 | Second branch daily/weekly/total notification fields | EditAdvancedOptionsSection at 291 through 299 binds the same nine fields through second quota block; branch-specific account conditions remain at 254/278 | source-equivalent |
| action-084, action-085, action-086, action-087 | Toggle expiry pause / 5h pause disabled / 7d pause disabled / credit reset | EditAdvancedOptionsSection at 454,465,486,518 binds InlineToggleSwitch; models at 794,795,797,799; Edit host binds each field at 252 through 259; IDs for 5h/7d/credit controls are retained | source-equivalent |
| action-088, action-089, action-090 | Toggle window cost / session limit / RPM limit | EditAdvancedOptionsSection at 552 mounts QuotaControlPanel with same three models; models declared at 802,805,808; Edit host forwards original refs | source-equivalent |
| action-091, action-092, action-093 | Toggle session masking / cache TTL / custom URL | Same QuotaControlPanel at 552; EditAdvancedOptionsSection models at 813,814,816; Edit host at 271,272,274 binds original refs | source-equivalent |
| action-094 | Toggle TLS fingerprint | EditAdvancedOptionsSection at 572 mounts TlsFingerprintPanel; model at 818; Edit host at 276 binds same ref | source-equivalent |

## GroupsView Candidates

| Candidate IDs | Old action | Reachable replacement and binding evidence | Classification |
|---|---|---|---|
| action-169, action-170, action-171 | Platform/type/status change -> `loadGroups` | The same Select controls now call `applyFilter`; GroupsView at 692 resets page to 1 and calls `loadGroups`. This additionally prevents filters retaining an out-of-range page | source-equivalent with page reset |
| action-172 | Open sorting dialog | Old GroupsView at 96 was a directly visible button, not a More item. Restored explicit `Button` with `aria-label=admin.groups.sortOrder`; sets `showSortModal`. `GroupSortModal.vue:102` watches show and loads/sorts all groups; errors toast. Load timing differs: modal is now open while its data loads | visibility repaired; loading-state browser check required |
| action-173 | Open composite routes | Old button at 409 was visible only for composite groups. `getGroupActionItems` retains the same platform guard/callback; rendered directly through ActionsCell extra slot, no More menu. GroupCompositeRoutesModal receives selected group and show state | visibility repaired |
| action-174 | Open rate multipliers | Same existing `handleRateMultipliers` sets selected group/show; restored direct icon button with title and accessible name; GroupRateMultipliersModal remains mounted | visibility repaired |
| action-175 | Open RPM overrides | Same existing `handleRPMOverrides` sets selected group/show; restored direct icon button with title and accessible name; GroupRPMOverridesModal remains mounted | visibility repaired |
| action-176, action-184 | Toggle create/edit live capability | GroupCreateModal at 913 / GroupEditModal at 912 -> per-form `toggleLive`; `useCreateGroupForm.ts:325` / `useEditGroupForm.ts:344` preserve disable, supported enable, unsupported confirmation branches. Host `unsupported-live` sets correct pending target; confirm delegates to matching exposed `confirmLive` | source-equivalent |
| action-177, action-185 | Toggle Messages dispatch | `admin/group/GroupMessagesDispatchFields.vue:18` mutates `form.allow_messages_dispatch`; `defineModel('form')` at 244; create/edit mount at 930/929 with original createForm/editForm; platform guard retained | source-equivalent |
| action-178, action-180, action-186, action-188 | Add exact Messages mapping from empty or populated UI | Shared MessagesDispatchFields at 146/219 -> `addMapping` at 257 pushes the same empty claude_model/target_model object into the bound form; both UI states retain add buttons | source-equivalent |
| action-179, action-187 | Remove exact mapping row | Shared MessagesDispatchFields at 206 -> `removeMapping` at 261 uses object identity and splice, matching old create/edit routines; create/edit object isolated by component instance | source-equivalent |
| action-181, action-189 | Toggle model routing | `admin/group/GroupModelRoutingRulesFields.vue:36` toggles `enabled`; model at 213; GroupCreateModal at 1035 / GroupEditModal at 1034 bind respective `form.model_routing_enabled`; Anthropic guard retained | source-equivalent |
| action-182, action-194 | Remove create/edit routing rule | Shared ModelRoutingRulesFields at 175 -> `removeRule` at 307 finds index, clears per-rule debounced search and result state, splices original bound rules; matches old functions at 5435/5451 | source-equivalent |
| action-183, action-195 | Add create/edit routing rule | Shared ModelRoutingRulesFields at 188 -> `addRule` at 302 pushes pattern/accounts blank rule; respective create/edit rules bound at GroupCreateModal:1036 / GroupEditModal:1035 | source-equivalent |
| action-190 | Remove selected edit-route account | Shared ModelRoutingRulesFields at 112 -> `removeSelectedAccount` at 285 filters same `rule.accounts`. Old third `isEdit` parameter was unused in this mutation; per-instance state removes need for it | source-equivalent |
| action-191 | Search edit-route accounts | Shared ModelRoutingRulesFields at 132 -> `searchAccountsByRule` at 265; same per-rule search runner and keyword state. Old isEdit namespace is replaced by separate create/edit component instances | source-equivalent |
| action-192 | Focus edit-route search | Shared ModelRoutingRulesFields at 133 -> `onAccountSearchFocus` at 292 opens result dropdown and requests initial results when empty; same old behavior at 5417 | source-equivalent |
| action-193 | Select edit-route account | Shared ModelRoutingRulesFields at 150 -> `selectAccount` at 270 deduplicates by ID, appends to same rule, clears keyword and closes dropdown; old isEdit namespace replaced by separate instances | source-equivalent |
| action-196, action-197 | Close sort dialog via header or cancel | `GroupSortModal.vue:2,54` -> `handleClose` at 113 -> emits close; GroupsView sets show false; child's show watcher at 102 clears sortableGroups, retaining old closeSortModal cleanup | source-equivalent |
| action-198 | Close composite routes via button | `GroupCompositeRoutesModal.vue:352` -> `handleClose` at 521 -> emits close; GroupsView `@close="closeCompositeRoutesModal"` clears show and selected group | source-equivalent |

## Verification And Remaining Limits

- Source triage covers all 121 IDs without interpreting a repository-wide matching action elsewhere as proof.
- `GroupsView.duplicate.spec.ts` now exercises directly available sorting/rate/RPM/delete controls, direct composite-route control and callback wiring, as well as existing duplicate and edit error behavior. Result: **7/7 passed**, one test file, Vitest 2.1.9. This test uses API mocks and does not prove real server persistence or browser layout.
- Run: `pnpm exec vitest run src/views/admin/__tests__/GroupsView.duplicate.spec.ts` from frontend with the repository's pinned pnpm.
- No new complete workflow tests for every account platform were run by this focused audit. File reading, unsupported-live confirmation, quota forwarding, all platform credentials, actual server operations and visual accessibility still require their existing tests/browser matrix.
- Sorting is source-reachable but loading/error display is not equivalent to the pre-Glass sequencing. Check populated and failed sort-load states in the browser; do not mark this candidate a complete runtime pass from this table alone.
