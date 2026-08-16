# Task 4 Fix Report

## Scope

Worktree: `/Users/ianshaw/Documents/code/personal/sub2api/.worktrees/task-4-pool`  
Branch: `feat/account-identity-pinning-p0-pool`

Implemented the Task 4 fix so TLS client cache keys and pool keys use canonical `tls_profile_id` for DB-backed profiles, while built-in defaults use a stable `builtin:<name>` key instead of a content hash.

## Code Changes

- `backend/internal/pkg/tlsfingerprint/dialer.go`
  - Added `Profile.ID int64` with `json:"-"` so runtime profile identity is available to cache keying without changing the ClientHello payload.
- `backend/internal/service/tls_fingerprint_profile_service.go`
  - Updated `GetProfileByID()` to copy the canonical DB ID onto the runtime `tlsfingerprint.Profile`.
- `backend/internal/repository/http_upstream.go`
  - Updated `tlsProfileCacheKey()`:
    - `nil` profile -> existing `none`
    - `profile.ID != 0` -> decimal ID string only
    - `profile.ID == 0` -> stable `builtin:<name>`

## Covering Tests

Added:

- `TestTLSProfileCacheKey_UsesBuiltinNameForDefaultProfile`
  - Verifies the built-in fallback profile no longer hashes its contents.
- `TestTLSPoolKey_ProxyIsolationSeparatesProfilesWithIdenticalContentsButDifferentIDs`
  - Verifies two DB-backed profiles with identical contents but IDs `101` and `202` produce different TLS cache keys, different pool keys, and different cached client entries.

Existing guard still relevant:

- `TestTLSPoolKey_ProxyIsolationReusesPoolAcrossAccounts`
  - Verifies proxy isolation still reuses the same TLS pool when the canonical TLS profile key is the same.

## Commands And Output

### Red

Command:

```bash
cd /Users/ianshaw/Documents/code/personal/sub2api/.worktrees/task-4-pool/backend && go test -tags=unit ./internal/repository -run 'CacheKey|Pool|TLS' -count=1
```

Exit code: `1`

Output:

```text
2026/08/14 13:11:13 WARN database connection pool duration clamped key=database.conn_max_lifetime_minutes before=0 after=30
2026/08/14 13:11:13 WARN database connection pool duration clamped key=database.conn_max_idle_time_minutes before=0 after=5
2026/08/14 13:11:13 WARN database connection pool duration clamped key=database.conn_max_lifetime_minutes before=-1 after=30
2026/08/14 13:11:13 WARN database connection pool duration clamped key=database.conn_max_idle_time_minutes before=-5 after=5
2026/08/14 13:11:13 WARN database connection pool duration clamped key=database.conn_max_lifetime_minutes before=1441 after=30
2026/08/14 13:11:13 WARN database connection pool duration clamped key=database.conn_max_idle_time_minutes before=1441 after=5
2026/08/14 13:11:13 INFO database connection pool configured effective.max_open=40 effective.max_idle=8 effective.max_lifetime=15m0s effective.max_idle_time=3m0s
--- FAIL: TestTLSProfileCacheKey_UsesBuiltinNameForDefaultProfile (0.00s)
    http_upstream_test.go:945:
        expected: "builtin:Built-in Default (Node.js 24.x)"
        actual  : "Built-in Default (Node.js 24.x):3efc06aad33727c"
--- FAIL: TestTLSPoolKey_ProxyIsolationSeparatesProfilesWithIdenticalContentsButDifferentIDs (0.00s)
    http_upstream_test.go:986:
        expected: "101"
        actual  : "shared-profile:9727caf8661a137e"
FAIL
FAIL    github.com/Wei-Shaw/sub2api/internal/repository    2.613s
FAIL
```

### Green

Command:

```bash
cd /Users/ianshaw/Documents/code/personal/sub2api/.worktrees/task-4-pool/backend && go test -tags=unit ./internal/repository -run 'CacheKey|Pool|TLS' -count=1
```

Exit code: `0`

Output:

```text
ok      github.com/Wei-Shaw/sub2api/internal/repository    3.128s
```

## Verification Notes

- The red run proved built-in defaults still used hashed keys and DB-backed profiles still collapsed to content-based keys before the fix.
- The green run confirmed the required repository TLS suite passed after the change.
- `ReadLints` reported no diagnostics for the edited files.
