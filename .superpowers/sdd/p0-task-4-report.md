# Task 4 Report: Connection pool identity and H1 Burst

## Scope

- Worktree: `/Users/ianshaw/Documents/code/personal/sub2api/.worktrees/task-4-pool`
- Branch: `feat/account-identity-pinning-p0-pool`
- Owned code files changed:
  - `backend/internal/repository/http_upstream.go`
  - `backend/internal/repository/http_upstream_test.go`

## What changed

- Added local repository burst sizing for account and account-proxy isolation:
  - `overflow = max(1, round(n * 0.2))`
  - `Burst = n + overflow`
  - `n <= 0` falls back to `3`, so local pool fallback becomes `4`
- Updated account/account-proxy H1 pool sizing so:
  - `maxIdleConns = Burst`
  - `maxIdleConnsPerHost = Burst`
  - `maxConnsPerHost = Burst`
- Expanded client cache/pool keys to carry TLS and transport identity:
  - account/account_proxy: account, proxy, burst, TLS profile key, transport family
  - proxy: proxy, TLS profile key, transport family
- Preserved proxy isolation pool sharing across accounts by excluding account ID and per-account burst from proxy-mode cache keys.

## TDD Evidence

### RED

Command:

```bash
cd backend && go test -tags=unit ./internal/repository -run 'CacheKey|Pool' -count=1
```

Observed failure before implementation:

```text
internal/repository/http_upstream_test.go:874:3: too many arguments in call to buildCacheKey
have (string, string, number, number, string, unknown type, string)
want (string, string, int64, string)
internal/repository/http_upstream_test.go:875:3: undefined: transportFamilyH1
...
FAIL    github.com/Wei-Shaw/sub2api/internal/repository [build failed]
```

Why this was the expected RED:

- Tests demanded TLS profile + transport family-aware cache key inputs that did not exist yet.
- Tests also asserted burst-sized H1 pools and proxy-isolation reuse semantics not represented in the repository code.

### GREEN

Focused verification:

```bash
cd backend && go test -tags=unit ./internal/repository -run 'CacheKey|Pool' -count=1
```

Result:

```text
ok      github.com/Wei-Shaw/sub2api/internal/repository   3.361s
```

Broader repository verification:

```bash
cd backend && go test -tags=unit ./internal/repository -count=1
```

Result:

```text
ok      github.com/Wei-Shaw/sub2api/internal/repository   9.030s
```

Post-review follow-up RED/GREEN:

```bash
cd backend && go test -tags=unit ./internal/repository -run 'TestBuildCacheKey_AccountIsolationIncludesTLSProfileAndTransportFamily' -count=1
```

RED:

```text
--- FAIL: TestBuildCacheKey_AccountIsolationIncludesTLSProfileAndTransportFamily (0.00s)
Error: Should not be: "account:17|burst:14|tls_profile:profile-101|family:h1"
```

After adding proxy identity to account-mode cache keys and updating the legacy proxy-change expectation:

```bash
cd backend && go test -tags=unit ./internal/repository -count=1
```

GREEN:

```text
ok      github.com/Wei-Shaw/sub2api/internal/repository   2.679s
```

Lint verification:

```text
ReadLints on edited repository files: no linter errors found.
```

## Added/updated tests

- `TestBuildCacheKey_AccountIsolationIncludesTLSProfileAndTransportFamily`
- `TestBuildCacheKey_ProxyIsolationOmitsAccountAndBurst`
- `TestPoolSettings_AccountIsolationUsesBurstConcurrency`
- `TestPoolSettings_AccountIsolationFallsBackToBurstOfThree`
- `TestTLSPoolKey_ProxyIsolationSeparatesTLSProfiles`
- `TestTLSPoolKey_ProxyIsolationReusesPoolAcrossAccounts`
- Updated existing repository tests that asserted pre-burst account pool sizing.

## Self-review

- Verified account/account_proxy pool sizing now uses burst, including the local `<=0 -> 3 -> 4` fallback.
- Verified account-isolation cache keys now include proxy identity as required.
- Verified proxy isolation still reuses a shared pool across alternating accounts even when their incoming concurrency differs.
- Verified TLS paths no longer reuse a client across distinct TLS profiles.
- Verified transport family is encoded into both cache and pool identity material.

## Notes

- The repository port currently receives a runtime `*tlsfingerprint.Profile`, not the numeric DB `tls_profile_id`. To preserve the required isolation without changing out-of-scope service interfaces, the repository now derives a stable TLS profile cache key from the runtime profile contents plus name.
