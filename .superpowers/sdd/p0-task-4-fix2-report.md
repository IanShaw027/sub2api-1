# Task 4 fix 2 report

## Summary

Restored idle drain when a proxy/TLS/transport-family change produces a new upstream pool cache key.
The canonical `tls_profile_id` keying from `a38e69e79` was preserved; the fix adds sibling-pool draining on cache miss rather than reverting key composition.

## Root cause

The earlier keying changes moved proxy, burst, and canonical TLS profile identity into `cacheKey`.
When a proxy or TLS/transport-family change produced a new key, the old path no longer hit `shouldReuseEntry()` and therefore skipped `removeClientLocked()`, leaving the previous idle client cached and its idle connections undrained.

## Coverage

- `TestAccountModeProxyChangeDrainsPreviousIdleClient`
- `TestTLSProfileChangeDrainsPreviousIdleClient`
- Updated proxy-isolation TLS key tests to assert replacement/drain behavior instead of retaining both idle pools.
- Documented that `resolvePoolSettings()` and `getClientEntryWithTLS()` accept raw/Effective `N`; Burst expansion remains internal to this package.

## Commands and output

### Red

Command:

```bash
cd /Users/ianshaw/Documents/code/personal/sub2api/.worktrees/task-4-pool/backend
go test -tags=unit ./internal/repository -run TestHTTPUpstreamSuite -testify.m 'TestAccountModeProxyChangeDrainsPreviousIdleClient|TestTLSProfileChangeDrainsPreviousIdleClient' -count=1
```

Output:

```text
--- FAIL: TestHTTPUpstreamSuite (0.00s)
    --- FAIL: TestHTTPUpstreamSuite/TestAccountModeProxyChangeDrainsPreviousIdleClient (0.00s)
        Error: expected 1 actual 0
        Messages: proxy change should drain the prior idle client
    --- FAIL: TestHTTPUpstreamSuite/TestTLSProfileChangeDrainsPreviousIdleClient (0.00s)
        Error: expected 1 actual 0
        Messages: TLS profile change should drain the prior idle client
FAIL
```

### Green

Command:

```bash
cd /Users/ianshaw/Documents/code/personal/sub2api/.worktrees/task-4-pool/backend
gofmt -w ./internal/repository/http_upstream.go ./internal/repository/http_upstream_test.go
go test -tags=unit ./internal/repository -run 'CacheKey|Pool|TLS|ProxyChange|Drain' -count=1
go test -tags=unit ./internal/repository -run TestHTTPUpstreamSuite -testify.m 'TestAccountModeProxyChangeDrainsPreviousIdleClient|TestTLSProfileChangeDrainsPreviousIdleClient' -count=1
```

Output:

```text
ok  	github.com/Wei-Shaw/sub2api/internal/repository	1.960s
ok  	github.com/Wei-Shaw/sub2api/internal/repository	2.030s
```

### Lints

`ReadLints` on the edited repository files reported no linter errors.
