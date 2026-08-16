# Task 3 fix 1 — Preserve end-to-end + 30s cap

**Worktree:** `.worktrees/task-3-ladder`  
**Branch:** `feat/account-identity-pinning-p0-ladder`  
**Base:** `d25f3f8aa`  
**Verdict:** Critical + Important fixed. Claude minors not expanded.

## Fixes

| ID | Change |
|---|---|
| C1 | `BindStickySessionAfterProfitAdmission` (gateway + OpenAI) early-returns on `PreserveStickyBindingFromContext`. `setStickySessionAccountID` also honors Preserve. The three `openai_gateway_scheduling.go` writers (~740, ~1165, ~1204) are additionally gated. |
| I1 | `runAccountSlotLadder` / `accountSlotLadderParams` clamp `WaitPlan.Timeout` with `min(timeout, 30s)`. 45s/120s plans two-shot at 30s; shorter plans are kept. |
| I2 | Scheduler sticky paths no longer preflight `GetAccountWaitingCount`. Queue-full returns the sticky account + WaitPlan so the ladder's `IncrementAccountWaitCount(false)` can switch + Preserve. |
| I3 | `acquireWebSearchAccountSlot` sets `WithPreserveStickyBinding` on switch; the next hop is immediate N only. |
| I4 | Grok Realtime pre-accept uses a bounded 4-attempt selection loop (`runOpenAISlotSwitchSelection`) instead of writing 503 on the first switch. |
| I5 | `previous_response_id` WaitPlan is built via `waitPlanUnlessPostSwitch` / `slotLadderMaxWaiting` (N=12 → MaxWaiting 4). |

## RED

Handler (`go test -tags=unit ./internal/handler -run 'TestSlotLadder_WaitPlanTimeoutClampedTo30s|TestAcquireResponsesAccountSlot_PostSwitchAdmissionBindPreservesOriginal|TestAcquireWebSearchAccountSlot_SwitchSetsPreserveAndImmediateNOnly|TestSelectAndAcquireGrokRealtimeAccount_SwitchContinuesSelection' -count=1`):

```
--- FAIL: TestSelectAndAcquireGrokRealtimeAccount_SwitchContinuesSelection
    expected: 0 (openAISlotAcquireOK)
    actual  : 1 (openAISlotAcquireFailed)
    Messages: pre-accept switch must continue selection instead of 503
--- FAIL: TestSlotLadder_WaitPlanTimeoutClampedTo30s
    expected: 30s
    actual  : 45s
    Messages: 45s sticky default must two-shot at 30s
--- FAIL: TestAcquireResponsesAccountSlot_PostSwitchAdmissionBindPreservesOriginal
    expected: 11
    actual  : 22
    Messages: switch → immediate acquire → admission-bind must keep the original sticky account
--- FAIL: TestAcquireWebSearchAccountSlot_SwitchSetsPreserveAndImmediateNOnly
    Should be true
    Messages: web-search switch must set Preserve
FAIL
```

Service (`go test -tags=unit ./internal/service -run 'TestBindStickySessionAfterProfitAdmission_PreserveDoesNotOverwrite|TestOpenAIBindStickySessionAfterProfitAdmission_PreserveDoesNotOverwrite|TestSelectAccountForModelWithExclusions_PreserveDoesNotOverwrite|TestOpenAISelectAccountWithLoadAwareness_PreserveDoesNotSetSticky|TestSelectAccountWithLoadAwareness_QueueFullEntersLadder|TestOpenAISelectAccountWithLoadAwareness_QueueFullEntersLadder|TestSelectAccountByPreviousResponseID_WaitPlanUsesSlotLadderMaxWaiting' -count=1`):

```
--- FAIL: TestBindStickySessionAfterProfitAdmission_PreserveDoesNotOverwrite
    expected: 11  actual: 22
--- FAIL: TestOpenAIBindStickySessionAfterProfitAdmission_PreserveDoesNotOverwrite
    expected: 11  actual: 22
--- FAIL: TestSelectAccountForModelWithExclusions_PreserveDoesNotOverwrite
    expected: 11  actual: 22
--- FAIL: TestOpenAISelectAccountWithLoadAwareness_PreserveDoesNotSetSticky
    expected: 11  actual: 22
--- FAIL: TestSelectAccountWithLoadAwareness_QueueFullEntersLadder
    expected: 1   actual: 2
    Messages: queue-full must still return the sticky account so the ladder can switch
--- FAIL: TestOpenAISelectAccountWithLoadAwareness_QueueFullEntersLadder
    expected: 1   actual: 2
--- FAIL: TestSelectAccountByPreviousResponseID_WaitPlanUsesSlotLadderMaxWaiting
    "3" is not greater than or equal to "4"
    Messages: N=12 must floor MaxWaiting at overflow+2
FAIL
```

Failures matched the missing behavior (overwrite, 45s window, queue-full skip, raw MaxWaiting=3, 503-on-switch). Not compile/typo failures.

## GREEN

```
cd backend && go test -tags=unit ./internal/handler -run 'TestSlotLadder_|TestFailoverState_RecordConcurrencyTimeout|TestWaitForSlot|TestAcquireAccountSlot|TestAcquireUserSlot|TestAcquireResponsesAccountSlot_PostSwitchAdmissionBindPreservesOriginal|TestAcquireWebSearchAccountSlot_SwitchSetsPreserveAndImmediateNOnly|TestSelectAndAcquireGrokRealtimeAccount_SwitchContinuesSelection' -count=1
ok  	github.com/Wei-Shaw/sub2api/internal/handler	2.016s

cd backend && go test -tags=unit ./internal/service -run 'TestAccountIsSlotCandidate_|TestPreferInstantFanout_|TestSlotLadderMaxWaiting_|TestSelectAccountWithLoadAwareness_StickyLoadRate100|TestSelectAccountWithLoadAwareness_NewSessionFree|TestBindGatewayStickySessionDuringSelection_Preserve|TestBindOpenAIStickySessionDuringSelection_Preserve|TestSelectAccountWithLoadAwareness_PostSwitch|TestGatewayService_SelectAccountWithLoadAwareness|TestOpenAISelectAccountWithLoadAwareness_AllFullWaitPlan|TestOpenAIGatewayService_SelectAccountWithScheduler_SessionSticky|TestSelectAccountWithLoadAwareness_WaitPlanUsesEffectiveConcurrency|TestBindStickySessionAfterProfitAdmission_PreserveDoesNotOverwrite|TestOpenAIBindStickySessionAfterProfitAdmission_PreserveDoesNotOverwrite|TestSelectAccountForModelWithExclusions_PreserveDoesNotOverwrite|TestOpenAISelectAccountWithLoadAwareness_PreserveDoesNotSetSticky|TestSelectAccountWithLoadAwareness_QueueFullEntersLadder|TestOpenAISelectAccountWithLoadAwareness_QueueFullEntersLadder|TestSelectAccountByPreviousResponseID_WaitPlanUsesSlotLadderMaxWaiting' -count=1
ok  	github.com/Wei-Shaw/sub2api/internal/service	1.261s

go vet -tags=unit ./internal/handler ./internal/service
# exit 0
```

Existing focused ladder tests stayed green. New Preserve / 30s-cap / queue-full / web-search / Grok / previous_response_id tests are included in the runs above.

## Out of scope

Claude minors M1–M8 (vacuous assertion, logging, duplicated math, WS fail-fast, dead helper, etc.) were not expanded.
