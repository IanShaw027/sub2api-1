# Task 5 Report: Outbound Identity Hemostasis

## Scope

Implemented Task 5 only in `.worktrees/task-5-identity` on `feat/account-identity-pinning-p0-identity`.

Changed files:
- `backend/internal/service/gateway_upstream_request.go`
- `backend/internal/service/account_tls_fingerprint.go`
- `backend/internal/service/identity_service.go`
- `backend/internal/service/openai_codex_identity.go`
- `backend/internal/service/account_extra_identity_validate.go`
- Focused tests for the above files

Did not edit Task 1 or Task 4 owned files.

## TDD RED Evidence

Initial focused RED with validator tests:

```text
go test -tags=unit ./internal/service -run 'TestApplyClaudeCodeMimicHeaders_DoesNotOverwriteFingerprintIdentity|TestResolveAccountTLSFingerprintRuntime_UsesAccountOSNotInboundUA|TestResolveAccountTLSFingerprintRuntime_RejectsRandomProfileID|TestApplyTLSFingerprintRuntimeHeaders_UsesLowercaseOriginator|TestIdentityService_RewriteUserIDWithMasking_DoesNotMintRandomSessionID|TestValidateAccountExtraIdentity' -count=1

internal/service/account_extra_identity_validate_test.go:13:9: undefined: ValidateAccountExtraIdentity
FAIL github.com/Wei-Shaw/sub2api/internal/service [build failed]
```

After adding only the validator, the behavioral RED failures were:

```text
TestResolveAccountTLSFingerprintRuntime_UsesAccountOSNotInboundUA:
expected "bound-macos", actual "bound-windows"

TestResolveAccountTLSFingerprintRuntime_RejectsRandomProfileID:
Should not be: "random-candidate"

TestApplyTLSFingerprintRuntimeHeaders_UsesLowercaseOriginator:
expected "codex_cli_rs", actual ""

TestApplyClaudeCodeMimicHeaders_DoesNotOverwriteFingerprintIdentity:
expected "claude-cli/2.1.221 (external, cli)", actual "claude-cli/2.1.220 (external, cli)"

TestIdentityService_RewriteUserIDWithMasking_DoesNotMintRandomSessionID:
expected deterministic RewriteUserID session, actual generated random session
```

Codex originator RED:

```text
go test -tags=unit ./internal/service -run 'TestCodexIdentityHeadersUseLowercaseOriginatorAndStripXOriginator' -count=1

http.Header{..."Originator":..., "X-Originator":...} does not contain "originator"
FAIL github.com/Wei-Shaw/sub2api/internal/service
```

## Implementation Summary

- Claude mimic headers now fill missing OAuth defaults but no longer overwrite already-applied fingerprint/account identity headers with `claude.DefaultHeaders`.
- TLS fingerprint runtime no longer derives OS binding selection from inbound `User-Agent`; account default OS/bindings drive the profile pick.
- TLS profile id `-1` is guarded in the Task 5 hot path and rejected by the exported extra validator.
- TLS/Codex originator writers now write raw lowercase `originator` and strip `X-Originator`.
- `session_id_masking_enabled` no longer mints a new 15-minute random session id when no cached mask exists; deterministic `RewriteUserID` output remains.
- Added `ValidateAccountExtraIdentity` for `openai_device_id` UUID parsing/version/variant and `tls_fingerprint_profile_id != -1`. Per task instruction, it is exported and tested but not wired into create/update services in this task.

## GREEN Evidence

Focused behavior suite:

```text
go test -tags=unit ./internal/service -run 'Test(ApplyClaudeCodeMimicHeaders|ResolveAccountTLSFingerprintRuntime|ApplyTLSFingerprintRuntimeHeaders|IdentityService_RewriteUserIDWithMasking|ValidateAccountExtraIdentity|EnsureCodexIdentityHeaders|CodexIdentityHeadersUseLowercaseOriginatorAndStripXOriginator|EnforceCodexIdentityHeaders)' -count=1

ok github.com/Wei-Shaw/sub2api/internal/service 1.151s
```

Lint diagnostics for edited production files:

```text
No linter errors found.
```

## Self-Review

- File ownership checked after an initial local correction: `tls_fingerprint_profile_service.go` was restored and the `-1` outbound guard was kept in `account_tls_fingerprint.go`.
- No Task 1 files (`account.go`, `account_service.go`, `admin_account.go`) were edited.
- No Task 4 repository pool files were edited.
- The validator is intentionally not wired into Create/Update/Bulk/Import/UpdateExtra in this task, matching the brief's later-merge instruction.

## Concerns

- `ValidateAccountExtraIdentity` is exported and tested but not invoked by write paths yet; invalid existing data can still persist until the merge task wires it into account writes.
- Direct callers of `TLSFingerprintProfileService.ResolveTLSProfile` still retain legacy `-1` random behavior; Task 5's outbound runtime path avoids it, and the new validator rejects future writes once wired.
