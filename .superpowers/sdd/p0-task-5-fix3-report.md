# Task 5 Fix 3 Report

## Scope

Worktree: `/Users/ianshaw/Documents/code/personal/sub2api/.worktrees/task-5-identity`  
Branch: `feat/account-identity-pinning-p0-identity`

Stopped `count_tokens` OAuth+mimic from forwarding inbound identity headers, aligned beta merge with `/v1/messages` mimic, and deleted unused `generateRandomUUID`. Did not reopen inbound-UA TLS routing.

## Code Changes

- `backend/internal/service/gateway_count_tokens.go`
  - Skip whitelist client-header passthrough when `tokenType == oauth && mimicClaudeCode`, matching `/v1/messages`.
- `backend/internal/service/gateway_upstream_request.go`
  - `computeFinalCountTokensAnthropicBeta` now ignores inbound `anthropic-beta` on OAuth mimic and uses only FullClaudeCodeMimicryBetas + BetaTokenCounting.
- `backend/internal/service/identity_service.go`
  - Removed dead `generateRandomUUID` (no callers after session-mask change). `generateClientID` / `generateUUIDFromSeed` remain.

## Test Coverage

- `TestComputeFinalCountTokensAnthropicBeta_OAuthMimic_IgnoresClientBeta`
  - Client `anthropic-beta` tokens are not merged into the upstream mimic beta set.
- `TestBuildCountTokensRequest_OAuthMimic_DoesNotPassThroughInboundIdentityHeaders`
  - Inbound `x-app`, `x-stainless-retry-count`, `x-stainless-timeout`, `anthropic-dangerous-direct-browser-access`, and `User-Agent` are replaced by Claude Code defaults, not forwarded.

## Commands And Output

### Red

Production files reverted to `HEAD`; new tests kept.

Command:

```bash
cd /Users/ianshaw/Documents/code/personal/sub2api/.worktrees/task-5-identity/backend && go test -tags=unit ./internal/service -run 'CountTokens|Mimic|SessionMask|RandomUUID|TLSFingerprint' -count=1
```

Exit code: `1`

Output:

```text
--- FAIL: TestComputeFinalCountTokensAnthropicBeta_OAuthMimic_IgnoresClientBeta (0.00s)
    gateway_context_management_test.go:242:
        Error: Should be false
        Messages: count_tokens mimic 不应把客户端自带 beta 混入上游身份
--- FAIL: TestBuildCountTokensRequest_OAuthMimic_DoesNotPassThroughInboundIdentityHeaders (0.00s)
    gateway_context_management_test.go:623:
        Error: Should be false
FAIL
FAIL    github.com/Wei-Shaw/sub2api/internal/service    3.322s
FAIL
```

### Green

Command:

```bash
cd /Users/ianshaw/Documents/code/personal/sub2api/.worktrees/task-5-identity/backend && go test -tags=unit ./internal/service -run 'CountTokens|Mimic|SessionMask|RandomUUID|TLSFingerprint' -count=1
```

Exit code: `0`

Output:

```text
ok      github.com/Wei-Shaw/sub2api/internal/service    3.380s
```

## Verification Notes

- Focused grep found no remaining `generateRandomUUID`.
- `ReadLints` reported no diagnostics for the edited production and test files.
- Router `MatchRequest("")` was not reopened.
