# Account Identity Pinning P0 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the P0 hemostasis in `docs/superpowers/specs/2026-08-14-account-identity-pinning-design.md` without creating `account_device_profiles`.

**Architecture:** One official-install shape per upstream account. Slot decisions use `EffectiveConcurrency()` (N) and `BurstConcurrency()` (~1.2N). Sticky sessions wait 30s for a normal slot on the bound account, then may take a burst slot, then may switch accounts while preserving the original binding. Connection pools and outbound identity stop flickering.

**Tech Stack:** Go (Gin, existing Redis concurrency Lua), Vue 3 + Vitest for the RPM UI change. Unit tests: `cd backend && go test -tags=unit`.

## Global Constraints

- Do **not** change `ent/schema/account.go` `Concurrency` `Default(3)`.
- Do **not** create `account_device_profiles` or any new table (P1a).
- `EffectiveConcurrency()` is the only admission number. Never treat `concurrency <= 0` as unlimited.
- Platform fallback when concurrency is `<=0` or `>32`: anthropic/openai OAuth or setup-token → **12**; grok OAuth → **1**; else → **3**.
- `overflow = max(1, round(N * 0.2))`; `Burst = N + overflow` (10→12, 12→14, 15→18).
- Wait 30s polling **only** `TryAcquire(N)`. Burst only at the 30s deadline, after a final `TryAcquire(N)`.
- `PreserveStickyBinding` ships in the same batch as the slot ladder. Do not overwrite the sticky account ID with the failover account.
- `streamStarted=true` → never switch accounts.
- `LoadFactor` / `EffectiveLoadFactor()` sort only; never admit.
- Filter candidates with `in_flight < Burst`, not `LoadRate < 100`. Sticky-bound accounts stay in the candidate set even at LoadRate 100.
- H1 `maxIdleConns`, `maxIdleConnsPerHost`, and `maxConnsPerHost` all equal Burst. `cacheKey` and `poolKey` both include `tls_profile_id` and `transport_family`.
- `connection_pool_isolation=proxy` must not put `account_id` or per-account Burst into the key.
- `concurrencyService == nil` is fail-closed.
- Do not copy official system prompts / CCH. Do not add WAF-bypass logic.
- pnpm only for frontend. Do not use npm.
- Follow handler → service → repository layering. Do not import repository from handler.
- Commit after each task. Do not push.
- **Models (required):** Implement and fix with Grok 4.6 (`cursor-grok-4.6-high-fast`) only. Review and acceptance with Claude Opus 5 (`claude-opus-5-thinking-high`) and GPT-5.6 Sol (`gpt-5.6-sol-medium`) in parallel. Do not use GPT/Claude to write or patch production code for this plan.
- **Review gate (required):** After each task commit, dispatch two read-only reviewers **in parallel**: Claude Opus 5 and GPT-5.6 Sol. The task is not done until both return, Critical/Important findings are fixed by a Grok implementer, and both re-approve.
- **Phase parallelism:** Tasks in the same phase run concurrently in **isolated git worktrees**. Do not edit another task's owned files. Merge into `feat/account-identity-pinning-p0` only after that task's dual review is clean. The next phase starts after every task in the current phase is merged.

## Phases

| Phase | Parallel tasks | Worktrees | File ownership (do not cross) |
|---|---|---|---|
| **A** | Task 1 capacity, Task 4 pool, Task 5 identity | main checkout / `.worktrees/task-4-pool` / `.worktrees/task-5-identity` | T1: `account.go` helpers + `account_service.go` Create + `admin_account.go` concurrency normalize. T4: `repository/http_upstream*.go` only. T5: mimic/TLS/identity/fingerprint files; extra validators in a **new** `account_extra_identity_validate.go` — do not edit T1 files. |
| **B** | Task 2 slots, Task 6 RPM | after A merge | T2: concurrency_service + slot callers except RPM. T6: `GetRPMStickyBuffer` + DTO + frontend. |
| **C** | Task 3 ladder+Preserve | after B (needs Task 2) | handlers + scheduling + sticky bind |

---

### Task 1: EffectiveConcurrency and BurstConcurrency

**Files:**
- Modify: `backend/internal/service/account.go` (add methods near `EffectiveLoadFactor`)
- Modify: `backend/internal/service/account_service.go` (`Create` default write)
- Modify: `backend/internal/service/admin_account.go` (`normalizeAccountConcurrency` only for create/import defaults; do not rewrite stored 3s)
- Test: `backend/internal/service/account_concurrency_test.go` (create)
- Test: existing create-account unit tests if they assert concurrency 0/3 on OAuth create

**Interfaces:**
- Consumes: `Account.Concurrency`, `Account.Platform`, `Account.Type`, `Platform*` / `AccountType*` constants
- Produces:
  - `func (a *Account) EffectiveConcurrency() int`
  - `func (a *Account) BurstConcurrency() int`
  - `func (a *Account) OverflowConcurrency() int`
  - `func DefaultConcurrencyForPlatform(platform, accountType string) int`

- [ ] **Step 1: Write the failing test**

Create `backend/internal/service/account_concurrency_test.go` with `//go:build unit`:

```go
func TestEffectiveConcurrency_UsesStoredWhenInRange(t *testing.T) {
    a := &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth, Concurrency: 15}
    if a.EffectiveConcurrency() != 15 {
        t.Fatalf("got %d", a.EffectiveConcurrency())
    }
}

func TestEffectiveConcurrency_Fallback(t *testing.T) {
    cases := []struct {
        platform, typ string
        conc, want    int
    }{
        {PlatformAnthropic, AccountTypeOAuth, 0, 12},
        {PlatformAnthropic, AccountTypeSetupToken, -1, 12},
        {PlatformOpenAI, AccountTypeOAuth, 99, 12},
        {PlatformGrok, AccountTypeOAuth, 0, 1},
        {PlatformGemini, AccountTypeOAuth, 0, 3},
        {PlatformAnthropic, AccountTypeAPIKey, 0, 3},
    }
    for _, tc := range cases {
        a := &Account{Platform: tc.platform, Type: tc.typ, Concurrency: tc.conc}
        if got := a.EffectiveConcurrency(); got != tc.want {
            t.Fatalf("%s/%s conc=%d: got %d want %d", tc.platform, tc.typ, tc.conc, got, tc.want)
        }
    }
}

func TestBurstConcurrency(t *testing.T) {
    // 10→12, 12→14, 15→18
    if (&Account{Concurrency: 10}).BurstConcurrency() != 12 {
        t.Fatal("10")
    }
    if (&Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth, Concurrency: 0}).BurstConcurrency() != 14 {
        t.Fatal("fallback 12 → burst 14")
    }
    if (&Account{Concurrency: 15}).BurstConcurrency() != 18 {
        t.Fatal("15")
    }
}

func TestCreateAccount_WritesPlatformDefaultWhenConcurrencyOmitted(t *testing.T) {
    // If a focused Create unit test harness already exists, extend it.
    // Otherwise test DefaultConcurrencyForPlatform + the Create assignment
    // via a small helper used by Create:
    // applyCreateConcurrency(platform, typ, reqConcurrency int) int
    if DefaultConcurrencyForPlatform(PlatformOpenAI, AccountTypeOAuth) != 12 {
        t.Fatal()
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test -tags=unit ./internal/service -run 'TestEffectiveConcurrency|TestBurstConcurrency|TestCreateAccount_WritesPlatformDefault' -count=1`

Expected: FAIL — methods undefined.

- [ ] **Step 3: Write minimal implementation**

```go
func DefaultConcurrencyForPlatform(platform, accountType string) int {
    if (platform == PlatformAnthropic || platform == PlatformOpenAI) &&
        (accountType == AccountTypeOAuth || accountType == AccountTypeSetupToken) {
        return 12
    }
    if platform == PlatformGrok && accountType == AccountTypeOAuth {
        return 1
    }
    return 3
}

func (a *Account) EffectiveConcurrency() int {
    if a == nil {
        return 3
    }
    if a.Concurrency >= 1 && a.Concurrency <= 32 {
        return a.Concurrency
    }
    return DefaultConcurrencyForPlatform(a.Platform, a.Type)
}

func (a *Account) OverflowConcurrency() int {
    n := a.EffectiveConcurrency()
    overflow := int(math.Round(float64(n) * 0.2))
    if overflow < 1 {
        overflow = 1
    }
    return overflow
}

func (a *Account) BurstConcurrency() int {
    return a.EffectiveConcurrency() + a.OverflowConcurrency()
}

func applyCreateConcurrency(platform, accountType string, requested int) int {
    if requested >= 1 && requested <= 32 {
        return requested
    }
    return DefaultConcurrencyForPlatform(platform, accountType)
}
```

In `AccountService.Create`, set `Concurrency: applyCreateConcurrency(req.Platform, req.Type, req.Concurrency)`.

Do **not** migrate existing rows. Do **not** change Ent default.

- [ ] **Step 4: Run test to verify it passes**

Same command. Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/account.go backend/internal/service/account_service.go backend/internal/service/account_concurrency_test.go
git commit -m "$(cat <<'EOF'
feat: add EffectiveConcurrency and BurstConcurrency helpers

Slot admission needs a single fail-closed N and a 120% burst cap without changing the Ent default of 3.
EOF
)"
```

---

### Task 2: Fail-closed slot acquire and burst acquire

**Files:**
- Modify: `backend/internal/service/concurrency_service.go`
- Modify: `backend/internal/repository/concurrency_cache.go` only if Lua must accept a caller-supplied max (it already does)
- Modify: every caller that passes `account.Concurrency` into acquire / WaitPlan.MaxConcurrency / `tryAcquireAccountSlot` / WS live / `resolvePoolSettings` **that lives in service/handler for slots** — Task 2 covers service acquire + existing unit tests. Pool settings are Task 4.
- Test: `backend/internal/service/concurrency_service_test.go`

**Interfaces:**
- Consumes: `EffectiveConcurrency()`, `BurstConcurrency()`
- Produces:
  - `AcquireAccountSlotForGroup` / `AcquireAccountSlot`: if `maxConcurrency <= 0`, return `Acquired: false` and a non-nil error (`ErrInvalidConcurrency`), never a no-op success
  - `AcquireUserSlot`: keep existing user-limit semantics unless tests require fail-closed there too; **account** path is the contract
  - `TryAcquireAccountSlot(ctx, accountID, groupID, n int)` — existing acquire with max=n
  - Callers that currently pass raw `account.Concurrency` for **account** slots must pass `account.EffectiveConcurrency()` after this task for the normal path. Burst wait is Task 3.

- [ ] **Step 1: Write the failing test**

Add tests:

```go
func TestAcquireAccountSlotForGroup_RejectsNonPositiveMax(t *testing.T) {
    // maxConcurrency 0 and -1 must NOT return Acquired=true
}

func TestAcquireAccountSlotForGroup_BurstLimitUsesSameKey(t *testing.T) {
    // fill N=2, third TryAcquire(2) fails, TryAcquire(3) succeeds
}
```

- [ ] **Step 2: Run failing tests**

`cd backend && go test -tags=unit ./internal/service -run 'TestAcquireAccountSlotForGroup_RejectsNonPositiveMax|TestAcquireAccountSlotForGroup_BurstLimitUsesSameKey' -count=1`

- [ ] **Step 3: Implement**

Replace the `if maxConcurrency <= 0 { return Acquired: true }` branch in `AcquireAccountSlotForGroup` with fail-closed.

Grep `account.Concurrency` used as maxConcurrency in:

- `backend/internal/service/gateway_scheduling.go`
- `backend/internal/service/openai_gateway_scheduling.go`
- `backend/internal/service/openai_account_scheduler.go`
- `backend/internal/handler/gateway_helper.go` callers
- WS live lease / turn slot paths that use account concurrency

Change those **account slot** call sites to `EffectiveConcurrency()`. Leave `LoadFactor` / sort keys alone.

If `concurrencyService == nil` in helper `tryAcquireAccountSlot`, return error (fail-closed), not success.

- [ ] **Step 4: Pass tests** including existing `concurrency_service_test.go`

- [ ] **Step 5: Commit**

```bash
git commit -m "$(cat <<'EOF'
fix: fail-closed account slots and admit with EffectiveConcurrency

Zero concurrency must not mean unlimited, and all account slot callers must share one N.
EOF
)"
```

---

### Task 3: Slot ladder, failover, PreserveStickyBinding, candidate filter

This is one batch. Do not split across commits that leave timeout→429 without failover, or failover without Preserve.

**Files:**
- Modify: `backend/internal/handler/gateway_helper.go`
- Modify: `backend/internal/handler/failover_loop.go`
- Modify: `backend/internal/handler/gateway_handler.go`
- Modify: `backend/internal/handler/gateway_handler_responses.go`
- Modify: `backend/internal/handler/gateway_handler_chat_completions.go`
- Modify: `backend/internal/handler/openai_gateway_handler.go`
- Modify: `backend/internal/handler/gemini_v1beta_handler.go`
- Modify: `backend/internal/handler/gateway_web_search.go`
- Modify: `backend/internal/service/gateway_scheduling.go` (`LoadRate < 100` filters around lines 441, 703)
- Modify: `backend/internal/service/openai_gateway_scheduling.go` (`LoadRate < 100` around 1106)
- Modify: `backend/internal/service/openai_account_scheduler.go` (concurrency_full immediate escape ~551)
- Modify: `backend/internal/service/gateway_service.go` (`bindGatewayStickySessionDuringSelection`)
- Modify: `backend/internal/service/openai_profit_control.go` (`bindOpenAIStickySessionDuringSelection`)
- Test: `backend/internal/handler/gateway_helper_hotpath_test.go`
- Test: new `backend/internal/handler/slot_ladder_test.go` and/or scheduling unit tests

**Interfaces:**
- Consumes: `EffectiveConcurrency()`, `BurstConcurrency()`, existing `AcquireAccountSlotForGroup`
- Produces result enum used by helper + failover:

```go
type SlotDecision int

const (
    SlotAcquiredNormal SlotDecision = iota
    SlotAcquiredBurst
    SlotWaitThenRetry
    SlotSwitchAccountPreserveBinding
    SlotAbortRequest
    SlotInfrastructureError
)
```

Wait contract (`waitForSlotWithPingTimeout` account path):

1. Immediate `Acquire(..., N)`.
2. Else wait ≤30s, poll only `Acquire(..., N)`.
3. On deadline: `Acquire(..., N)` then `Acquire(..., Burst)`.
4. Burst fail + `streamStarted==false` → `SlotSwitchAccountPreserveBinding`.
5. Burst fail + `streamStarted==true` → `SlotAbortRequest` (existing `handleConcurrencyError` OK).
6. Wait-queue full (`IncrementAccountWaitCount` false) → switch, not 429.
7. After switch: only immediate `Acquire(..., N)` on the new account. No 30s, no burst.
8. Sticky bind functions: if context has `PreserveStickyBinding`, do not `SetSessionAccountID` to the new account.

Candidate filter:

- Replace `loadInfo.LoadRate < 100` inclusion with `loadInfo.CurrentConcurrency < burst` where burst comes from the account's `BurstConcurrency()`.
- If a sticky account ID is already bound, keep that account in the candidate list even when `CurrentConcurrency >= N`.
- New sessions: prefer accounts with `CurrentConcurrency < N` (instant fan-out). Only enter the 30s ladder when the chosen account is at N.

OpenAI scheduler: delete the immediate `concurrency_full` escape; use this ladder.

`MaxWaiting` ≥ `max(3, overflow+2)`.

- [ ] **Step 1: Write failing tests** covering:
  - 13th request on N=12 waits; does not Burst before 30s
  - wait interval never calls acquire with Burst
  - deadline two-shot: N then Burst
  - both fail → switch + original binding unchanged
  - wait_queue_full → switch, not 429
  - post-switch immediate N only
  - `streamStarted=true` → no switch
  - sticky LoadRate=100 still selectable
  - new session with a free account (`in_flight < N`) does not wait

Use fake clocks / short timeouts in tests (inject wait timeout; production default 30s). Do not sleep 30s in CI.

- [ ] **Step 2: Run tests, confirm RED**

- [ ] **Step 3: Implement the ladder, failover `RecordConcurrencyTimeout` continue, Preserve, filter**

Handler rule: `streamStarted=false` + switch decision → `FailedAccountIDs` + `continue`. Never `handleConcurrencyError` as the terminal path in that case.

- [ ] **Step 4: PASS focused handler + scheduling tests, then `go test -tags=unit ./internal/handler ./internal/service -count=1`**

- [ ] **Step 5: Commit**

```bash
git commit -m "$(cat <<'EOF'
feat: wait 30s for a normal slot before burst or sticky failover

Full accounts must queue on the bound install, overflow only after the wait, and keep the original sticky binding.
EOF
)"
```

---

### Task 4: Connection pool identity and H1 Burst

**Files:**
- Modify: `backend/internal/repository/http_upstream.go` (`buildCacheKey`, `buildPoolKey`, `resolvePoolSettings`, TLS client entry)
- Test: `backend/internal/repository/http_upstream_test.go`

**Interfaces:**
- Consumes: `BurstConcurrency()` (pass burst int into resolvePoolSettings; repository must not import a circular service helper if already decoupled — pass the int from the service caller)
- Produces: cacheKey/poolKey composition

| isolation | cacheKey / poolKey must include |
|---|---|
| `account` / `account_proxy` | `account_id, proxy_id, tls_profile_id, transport_family, Burst` |
| `proxy` | `proxy_id, tls_profile_id, transport_family` — no account, no per-account Burst |

`resolvePoolSettings` for account/account_proxy: set all three of `maxIdleConns`, `maxIdleConnsPerHost`, `maxConnsPerHost` to Burst (already sets all three to `accountConcurrency`; caller must pass Burst not raw Concurrency).

TLS path `cacheKey := "tls:" + buildCacheKey(...)` must also include profile id + transport family.

- [ ] **Step 1: Failing tests** for key composition and proxy isolation not changing Burst when two accounts alternate
- [ ] **Step 2: RED**
- [ ] **Step 3: Implement**
- [ ] **Step 4: PASS `go test -tags=unit ./internal/repository -run 'CacheKey|Pool' -count=1` plus existing http_upstream tests**
- [ ] **Step 5: Commit**

```bash
git commit -m "$(cat <<'EOF'
fix: pin HTTP pool keys to TLS profile and size H1 pools to Burst

Overflow slots need idle connections, and different TLS profiles must never share a client.
EOF
)"
```

---

### Task 5: Outbound identity hemostasis

**Files:**
- Modify: `backend/internal/service/gateway_upstream_request.go` (`applyClaudeCodeMimicHeaders` — stop overwriting with `claude.DefaultHeaders`; use fingerprint/account headers already applied)
- Modify: `backend/internal/service/account_tls_fingerprint.go` (remove inbound-UA OS inference for template pick; reject profile id `-1`; do not set `X-Originator`)
- Modify: Codex originator writers (search `X-Originator`, `originator`) — outbound `originator` lowercase only
- Modify: `backend/internal/service/identity_service.go` — if `session_id_masking_enabled`, stop generating a new 15-minute random ID; keep the key, do not mint new random session IDs
- Modify: extra write validation for `openai_device_id` (`uuid.Parse` + version/variant) and `tls_fingerprint_profile_id != -1` on Create/Update/Bulk/Import/`UpdateExtra`
- Test: existing mimic / TLS / identity / extra validation tests

- [ ] **Step 1: Failing tests**
  - mimic does not force `claude.DefaultHeaders` over a fingerprint UA
  - inbound Windows UA + account macos TLS extra → macos template
  - `tls_fingerprint_profile_id: -1` rejected on write
  - `openai_device_id: "dev-xyz"` rejected on write
  - originator header is lowercase; `X-Originator` absent
  - masking enabled does not rotate a new random session id

- [ ] **Step 2: RED**
- [ ] **Step 3: Implement**
- [ ] **Step 4: PASS focused unit tests**
- [ ] **Step 5: Commit**

```bash
git commit -m "$(cat <<'EOF'
fix: stop outbound identity flicker on mimic, TLS, and session mask

Pinned accounts must keep one OS/TLS/device shape instead of inheriting each inbound request.
EOF
)"
```

---

### Task 6: RPM sticky buffer formula and frontend writeback

**Files:**
- Modify: `backend/internal/service/account.go` `GetRPMStickyBuffer`
- Modify: `backend/internal/service/account_rpm_test.go` (update expected values)
- Modify: DTO mapper so computed buffer is not persisted as if it were manual (`backend/internal/handler/dto/mappers.go`)
- Modify: `frontend/src/components/account/EditAccountModal.vue` — do not write computed buffer back into extra
- Modify: `frontend/src/components/account/AccountCapacityCell.vue` — `max(EffectiveConcurrency, max(baseRPM/5, 1))` when no manual key
- Optional one-shot cleanup migration only if you can detect keys equal to old `conc+sess` or `base/5`; skip if unsafe. Prefer leaving stale manual keys and documenting.

**Formula:**

- extra has `rpm_sticky_buffer` in 1–10000 → use it
- else `max(EffectiveConcurrency(), max(baseRPM/5, 1))` when `baseRPM > 0`, else 0
- never add `max_sessions`

Existing tests that expect `conc+sess` (e.g. `conc=3 sess=10 → 13`) **must be rewritten** to the new formula. That shrink is intended.

- [ ] **Step 1: Rewrite `account_rpm_test.go` expectations first (TDD)**
- [ ] **Step 2: RED**
- [ ] **Step 3: Implement backend + frontend**
- [ ] **Step 4:** `cd backend && go test -tags=unit ./internal/service -run RPM -count=1` and `pnpm --dir frontend exec vitest run` on the affected component tests if present
- [ ] **Step 5: Commit**

```bash
git commit -m "$(cat <<'EOF'
fix: compute RPM sticky buffer from EffectiveConcurrency only

Session caps are not concurrency and must not inflate the sticky RPM reservation.
EOF
)"
```

---

## Self-review coverage

| Spec P0 item | Task |
|---|---|
| mimic / DefaultHeaders | 5 |
| TLS -1 / inbound UA OS | 5 |
| pool key + H1 Burst | 4 |
| originator / X-Originator | 5 |
| EffectiveConcurrency + fail-closed | 1, 2 |
| RPM + frontend | 6 |
| openai_device_id write validate | 5 |
| session mask stop random | 5 |
| 30s ladder + Preserve + failover | 3 |
| LoadRate filter / LoadFactor sort-only | 3 |
| P1a table | out of scope |
