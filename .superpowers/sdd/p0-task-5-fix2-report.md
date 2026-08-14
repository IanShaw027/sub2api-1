# Task 5 Fix 2 Report

## Scope

Worktree: `/Users/ianshaw/Documents/code/personal/sub2api/.worktrees/task-5-identity`  
Branch: `feat/account-identity-pinning-p0-identity`

Applied the second Task 5 review fix so Kiro TLS runtime and the Codex PAT validator both write lowercase raw `originator`, never `X-Originator`, and the affected assertions read the raw header map instead of Go's canonical accessor.

## Code Changes

- `backend/internal/service/kiro_tls_profile.go`
  - `applyKiroTLSFingerprintRuntime()` now deletes any `X-Originator` variant and writes raw lowercase `originator`.
- `backend/internal/service/openai_codex_pat_service.go`
  - Extracted `newCodexPATWhoamiRequest()` so the request can be tested before transport normalization.
  - The PAT whoami request now uses `setCodexOriginator()` instead of `Header.Set("originator", ...)`.
- Updated Codex-originator assertions in:
  - `backend/internal/service/openai_compat_model_test.go`
  - `backend/internal/service/openai_gateway_service_test.go`
  - `backend/internal/service/openai_oauth_passthrough_test.go`
  - `backend/internal/service/openai_ws_forwarder_success_test.go`

## Test Coverage Added

- `TestApplyKiroTLSFingerprintRuntime_UsesLowercaseOriginator`
  - Verifies Kiro runtime writes raw lowercase `originator`, preserves `User-Agent`, and leaves no `X-Originator` key behind.
- `TestNewCodexPATWhoamiRequest_UsesLowercaseOriginator`
  - Verifies the Codex PAT whoami request uses lowercase raw `originator` and strips `X-Originator` before any transport canonicalization can hide the bug.

## Commands And Output

### Red

Command:

```bash
cd /Users/ianshaw/Documents/code/personal/sub2api/.worktrees/task-5-identity/backend && go test -tags=unit ./internal/service -run 'Originator|KiroTLS|CodexPAT|CodexIdentity|TLSFingerprint' -count=1
```

Exit code: `1`

Output:

```text
# github.com/Wei-Shaw/sub2api/internal/service [github.com/Wei-Shaw/sub2api/internal/service.test]
internal/service/openai_codex_pat_service_test.go:14:14: undefined: newCodexPATWhoamiRequest
FAIL    github.com/Wei-Shaw/sub2api/internal/service [build failed]
FAIL
```

### Green

Command:

```bash
cd /Users/ianshaw/Documents/code/personal/sub2api/.worktrees/task-5-identity/backend && go test -tags=unit ./internal/service -run 'Originator|KiroTLS|CodexPAT|CodexIdentity|TLSFingerprint' -count=1
```

Exit code: `0`

Output:

```text
ok      github.com/Wei-Shaw/sub2api/internal/service    3.218s
```

## Verification Notes

- Focused grep over allowed identity/TLS/Codex/Kiro service files found no remaining outbound `Header.Set("Originator"...)`, `Header.Set("originator"...)`, or `X-Originator` writes beyond explicit cleanup helpers/tests.
- `ReadLints` reported no diagnostics for the edited production and test files.
