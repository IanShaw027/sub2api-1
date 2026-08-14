# Task 4 fix 4 report

## Scope

Worktree: `/Users/ianshaw/Documents/code/personal/sub2api/.worktrees/task-4-pool`  
Branch: `feat/account-identity-pinning-p0-pool`  
Base: `8c000ada7`

Fixed the two GPT rereview2 Importants only. Claude residuals were not expanded. `DoWithTLS` signature unchanged. `TestAccountProxyIsolation_DifferentProxy` still expects 2 clients.

## Fixes

1. **Drain in-flight siblings.** Removed the `inFlight != 0` skip in `removeSiblingClientsLocked`. A proxy/TLS/family switch now calls `removeClientLocked` / `CloseIdleConnections` even while a request is active. Go close-idle does not interrupt active connections.

2. **Same-ID TLS config replacement.** Canonical identity stays the numeric ID (`tlsProfileCacheKey` still returns `"101"`). Pool identity appends a ClientHello content-hash revision (`101@<8-byte hex>`). `shouldReuseEntry` sees the pool-key change and rebuilds the transport. Distinct IDs under proxy isolation still coexist (2 clients).

## Covering tests

- `TestAccountModeDrainsInFlightSiblingOnProxyChange` — `inFlight=1`, account-mode proxy switch, stale entry removed and idle closed.
- `TestTLSProfileSameIDConfigReplacementRebuildsClient` — profile `101` edited in place rebuilds and leaves one pool.
- `TestTLSPoolKey_ProxyIsolationSeparatesProfilesWithIdenticalContentsButDifferentIDs` — still 2 clients for IDs 101/202; same-ID edit of 101 does not drain 202.
- `TestTLSProfilePoolIdentity_SameIDConfigReplacementKeepsCanonicalID` — cache key stays `"101"`; pool identities differ.

## Commands and output

### Red

Command:

```bash
cd /Users/ianshaw/Documents/code/personal/sub2api/.worktrees/task-4-pool/backend
go test -tags=unit ./internal/repository -count=1 -run 'TestHTTPUpstreamSuite/TestAccountModeDrainsInFlightSiblingOnProxyChange|TestHTTPUpstreamSuite/TestTLSProfileSameIDConfigReplacementRebuildsClient|TestTLSPoolKey_ProxyIsolationSeparatesProfilesWithIdenticalContentsButDifferentIDs'
```

Exit code: `1`

Output:

```text
--- FAIL: TestTLSPoolKey_ProxyIsolationSeparatesProfilesWithIdenticalContentsButDifferentIDs (0.00s)
    http_upstream_test.go:1088:
        Error: Expected and actual point to the same object
        Messages: same-ID TLS config replacement must rebuild the ClientHello transport
--- FAIL: TestHTTPUpstreamSuite (0.00s)
    --- FAIL: TestHTTPUpstreamSuite/TestAccountModeDrainsInFlightSiblingOnProxyChange (0.00s)
        Error: expected 1 actual 0
        Messages: in-flight stale sibling must still CloseIdleConnections
    --- FAIL: TestHTTPUpstreamSuite/TestTLSProfileSameIDConfigReplacementRebuildsClient (0.00s)
        Error: Expected and actual point to the same object
        Messages: same-ID TLS config replacement must not reuse the old ClientHello transport
FAIL
FAIL    github.com/Wei-Shaw/sub2api/internal/repository    1.117s
```

### Green

Command:

```bash
cd /Users/ianshaw/Documents/code/personal/sub2api/.worktrees/task-4-pool/backend
gofmt -w ./internal/repository/http_upstream.go ./internal/repository/http_upstream_test.go
go test -tags=unit ./internal/repository -count=1
```

Exit code: `0`

Output:

```text
ok  	github.com/Wei-Shaw/sub2api/internal/repository	2.940s
```
