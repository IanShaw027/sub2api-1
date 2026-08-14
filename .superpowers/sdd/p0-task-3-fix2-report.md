# Task 3 fix 2 Report — Grok Realtime pre-accept must not stream

**Status:** done  
**Worktree:** `/Users/ianshaw/Documents/code/personal/sub2api/.worktrees/task-3-ladder`  
**Branch:** `feat/account-identity-pinning-p0-ladder`  
**Base HEAD:** `eec0779f7` (clean at start; leftover untracked `p0-task-3-report.md` from prior task left untouched)

## What shipped

`selectAndAcquireGrokRealtimeAccount` now passes `reqStream=false` into `acquireResponsesAccountSlot`. Pre-accept waits no longer emit comment-format SSE pings, so `streamStarted` stays false until `coderws.Accept`. Deadline Burst failure returns `openAISlotAcquireSwitchAccount` and the retry loop continues.

Did not expand Claude minors.

## RED

Added `TestSelectAndAcquireGrokRealtimeAccount_PreAcceptWaitDoesNotStreamPing` first. It drives the real wait/ping path (comment-format helper, 10ms ping, 80ms WaitPlan, account 1 stays full) through `selectAndAcquireGrokRealtimeAccount`, not a mocked retry status.

```
cd backend && go test -tags=unit ./internal/handler -run 'TestSelectAndAcquireGrokRealtimeAccount_PreAcceptWaitDoesNotStreamPing' -count=1
```

Result: **FAIL** (expected) — production still passed `reqStream=true`.

```
Error: ":\n\n:\n\n:\n\n:\n\n:\n\n:\n\n:\n\nevent: error\ndata: {\"error\":{\"message\":\"Concurrency limit exceeded for account, please retry later\",\"type\":\"rate_limit_error\"}}\n\n" should not contain ":\n\n"
Messages: pre-accept wait must not write SSE pings
```

Pings committed the response and the ladder aborted instead of switching to account 2.

## GREEN

Changed the Grok pre-accept acquire to `reqStream=false`. Same test:

```
cd backend && go test -tags=unit ./internal/handler -run 'TestSelectAndAcquireGrokRealtimeAccount_PreAcceptWaitDoesNotStreamPing' -count=1
```

Result: **PASS**

```
ok  	github.com/Wei-Shaw/sub2api/internal/handler	1.303s
```

No SSE ping, response not written, deadline miss switched to account 2 (`openAISlotAcquireOK`).

Prior focused suites:

```
cd backend && go test -tags=unit ./internal/handler -run 'TestSelectAndAcquireGrokRealtimeAccount_|TestSlotLadder_|TestFailoverState_RecordConcurrencyTimeout|TestWaitForSlot|TestAcquireAccountSlot|TestAcquireUserSlot|TestAcquireResponsesAccountSlot_|TestAcquireWebSearchAccountSlot_' -count=1
cd backend && go test -tags=unit ./internal/service -run 'TestAccountIsSlotCandidate_|TestPreferInstantFanout_|TestSlotLadderMaxWaiting_|TestSelectAccountWithLoadAwareness_StickyLoadRate100|TestSelectAccountWithLoadAwareness_NewSessionFree|TestBindGatewayStickySessionDuringSelection_Preserve|TestBindOpenAIStickySessionDuringSelection_Preserve|TestSelectAccountWithLoadAwareness_PostSwitch|TestGatewayService_SelectAccountWithLoadAwareness|TestOpenAISelectAccountWithLoadAwareness_AllFullWaitPlan|TestOpenAIGatewayService_SelectAccountWithScheduler_SessionSticky|TestSelectAccountWithLoadAwareness_WaitPlanUsesEffectiveConcurrency' -count=1
```

Result: **PASS**

```
ok  	github.com/Wei-Shaw/sub2api/internal/handler	2.215s
ok  	github.com/Wei-Shaw/sub2api/internal/service	2.195s
```

## Files

- `backend/internal/handler/grok_audio.go` — pre-accept acquire uses non-streaming wait
- `backend/internal/handler/grok_audio_test.go` — real wait/ping path test
