# Account Device Profiles P1a+P1b Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Each upstream account presents as one stable official-client install: a validated `account_device_profiles` row is the authority, Redis is only a projection, outbound identity is read from the profile, and optional per-account/platform learning may CAS-upgrade the software bundle when the version is in the registry and higher.

**Architecture:** PostgreSQL is the single authority. Service-layer `AccountDeviceProfile` + `Validate*` sit in front of every write. `GetOrCreate` builds a baseline with `learning_enabled=false`. Shadow accounts read the parent row and never insert. Learning (P1b) replaces the whole software bundle via CAS + `SoftwareBundleRegistry`; device IDs never change on upgrade.

**Tech Stack:** Go, Ent, Atlas SQL migrations, PostgreSQL, Redis, Wire, Gin, existing `internal/service` ports.

## Global Constraints

- Table name is `account_device_profiles` only. Do not create `auth_identity` or reuse login Identity tables.
- One row per **canonical** `account_id` (UNIQUE). Shadow (`parent_account_id` set) must not insert a second row; resolve via parent.
- `revision` is CAS. `schema_version` and `client_version` are not CAS tokens.
- `tls_profile_id` is NULL or a positive FK. **-1 is illegal** on write and on the profile.
- `transport_family` is `h1` or `h2`. Do not claim `h2` unless ALPN and the HTTP/2 transport are both real (P1c). P1 baselines write `h1`.
- `gateway_account_uuid` is gateway-generated. **Never** copy `extra.account_uuid`. `RewriteUserID` must not use `extra.account_uuid`.
- `session_namespace` is 32–64 hex and immutable after insert.
- OS / arch / installation / machine / device / `gateway_account_uuid`: first-write-wins. Learning must not change them.
- Claude real account UUID: never learn.
- TLS ClientHello: never learn from inbound. Runtime family change may swap a registry-compatible template and must drain that account's idle pool (call existing drain; do not reimplement pool keys).
- Illegal write: **reject the whole package**, keep the old row, do not emit a half bundle. Metric/log name `identity_reject`.
- Learning default is **off**. Do not enable learning for the whole fleet. `learning_enabled` is per profile, default false.
- Admin has no silent reset.
- Do not copy official system prompts / CCH. Do not write WAF bypass.
- Do not change Ent `Account.Concurrency` `Default(3)`.
- Do not reopen P0 slot ladder, HTTP pool key composition, or RPM formula except to **read** `EffectiveConcurrency` / profile TLS id.
- Implement = Grok 4.6. Review = Claude Opus 5 + GPT-5.6 Sol. Dual approve before merge.
- File ownership is exclusive. If a file is listed under another in-flight task, stop.

## File ownership (anti-drift)

| Task | Exclusive create/modify | Forbidden |
|---|---|---|
| 1 Schema | `ent/schema/account_device_profile.go`, `ent/schema/account.go` (one edge only), `migrations/234_account_device_profiles.sql`, generated `ent/` + `wire` only if generate requires it | service/, handler/, frontend/ |
| 2 Validate profile | `service/account_device_profile.go`, `service/account_device_profile_test.go` | ent/schema, migrations, handlers |
| 3 Registry | `service/software_bundle_registry.go`, `service/software_bundle_registry_test.go` | ent, handlers, identity_service |
| 4 Extra capacity | `service/account_extra_identity_validate.go` (+test), wire into Create/Update/Bulk/Import in `account_service.go` / `admin_account.go` / admin handlers that already write extra | device profile table, outbound mimic |
| 5 Repo + GetOrCreate | `repository/account_device_profile_repo.go`, `service/account_device_service.go` (+tests) | outbound header writers |
| 6 Redis projection | `repository/identity_cache.go` (+test): profile projection key; invalidate on DB write | Do not keep `fingerprint:` as a second authority |
| 7 Shadow | `account_device_service.go` GetOrCreate: `IsShadow()` → parent profile | Do not insert shadow rows |
| 8 Claude user_id | `identity_service.go` RewriteUserID uses `gateway_account_uuid` | extra.account_uuid |
| 9–13 Outbound | one platform family per task (see tasks) | other platforms' header files |
| 14 Learning CAS | `account_device_service.go` LearnIfOfficial + CAS | Do not change device IDs |
| 15 learning_enabled extra | admin extra + DTO only | Do not default true |
| 16 Wire | `cmd/server/wire.go` + generate | Do not change validator rules |
| 17 Session/thread | identity session derivation from `session_namespace` | Do not bring account A's thread to B |

## Shared types (Task 2 owns the struct; everyone else imports it)

```go
type AccountDeviceProfile struct {
    ID                  int64
    AccountID           int64
    Revision            int64
    SchemaVersion       int
    Platform            string
    ClientFamily        string
    InstallationID      string
    DeviceID            string
    ClientID            string
    MachineID           string
    GatewayAccountUUID  string
    SessionNamespace    string
    OSFamily            string
    Arch                string
    Runtime             string
    RuntimeVersion      string
    ClientVersion       string
    TLSProfileID        *int64
    TransportFamily     string
    ProfilePayload      map[string]any
    LearnedFrom         string // baseline | official_traffic | baseline_floor
    LearningEnabled     bool
    CreatedAt           time.Time
    UpdatedAt           time.Time
    VersionUpgradedAt   *time.Time
}

const (
    ClientFamilyClaudeCode   = "claude-code"
    ClientFamilyCodexCLI     = "codex-cli"
    ClientFamilyGrokCLI      = "grok-cli"
    ClientFamilyKiroIDE      = "kiro-ide"
    ClientFamilyGeminiCLI    = "gemini-cli"
    ClientFamilyAntigravity  = "antigravity"
    TransportH1              = "h1"
    TransportH2              = "h2"
    LearnedFromBaseline      = "baseline"
    LearnedFromOfficial      = "official_traffic"
    LearnedFromBaselineFloor = "baseline_floor"
)

func DefaultClientFamily(platform string) string
func ValidateAccountDeviceProfile(p *AccountDeviceProfile) error
func ValidateOutboundBundle(p *AccountDeviceProfile) error
func ValidateAccountCapacityExtra(extra map[string]any) error // Task 4; may live beside identity extra
```

`DefaultClientFamily`: anthropic→claude-code, openai→codex-cli, grok→grok-cli, kiro→kiro-ide, gemini→gemini-cli, antigravity→antigravity.

---

### Task 1: Schema + migration 234 + generate

**Files:**
- Create: `backend/ent/schema/account_device_profile.go`
- Create: `backend/migrations/234_account_device_profiles.sql`
- Modify: `backend/ent/schema/account.go` — add `edge.To("device_profile", AccountDeviceProfile.Type).Unique()` only
- Generate: `cd backend && go generate ./ent` (and `./cmd/server` only if wire fails without it)

**Interfaces:**
- Produces: Ent type `AccountDeviceProfile` mapped to table `account_device_profiles`
- Consumes: none

SQL must include: `account_id BIGINT NOT NULL UNIQUE REFERENCES accounts(id)`, `revision BIGINT NOT NULL`, `schema_version INT NOT NULL`, platform/client_family/os/arch/runtime/transport CHECKs, `tls_profile_id BIGINT REFERENCES tls_fingerprint_profiles(id)` (NULL ok), `profile_payload JSONB NOT NULL DEFAULT '{}'`, `learned_from` CHECK, `learning_enabled BOOLEAN NOT NULL DEFAULT false`, timestamps. Ent annotations must match CHECKs. Do not rely on ent auto-migrate alone.

- [ ] **Step 1:** Write schema + SQL
- [ ] **Step 2:** `go generate ./ent`
- [ ] **Step 3:** `go test -tags=unit ./ent/...` or compile `go build ./ent`
- [ ] **Step 4:** Commit `feat: add account_device_profiles schema and migration`

---

### Task 2: ValidateAccountDeviceProfile

**Files:**
- Create: `backend/internal/service/account_device_profile.go`
- Create: `backend/internal/service/account_device_profile_test.go`

**Interfaces:**
- Produces: struct + `ValidateAccountDeviceProfile` + `ValidateOutboundBundle` + `DefaultClientFamily`
- Consumes: none (no ent import)

Rules (reject whole package):
- platform ∈ anthropic/openai/gemini/antigravity/grok/kiro and equals the account platform when provided
- client_family matches platform via `DefaultClientFamily` or the matching table
- session_namespace 32–64 hex
- tls_profile_id nil or >0 (never -1)
- transport_family h1|h2
- profile_payload object ≤8KB, keys ⊆ whitelist (start with: user_agent, originator, stainless_*, grok_token_auth, grok_identifier, kiro_system_version, kiro_node_version, kiro_commit)
- strings trimmed, no CTL/NUL; UA≤256; single header ≤512
- runtime_version / client_version length ≤64; reject `-local`, `-dev`, `+build` in those fields
- learned_from ∈ baseline|official_traffic|baseline_floor
- schema_version 1–100

Unknown os/arch/runtime: ValidateOutbound does **not** fail (skip-learn is Task 14). Baseline create should pick known defaults.

Tests: valid anthropic baseline; tls -1 rejected; payload unknown key rejected; shadow is not this task.

- [ ] TDD then commit `feat: validate account device profiles`

---

### Task 3: SoftwareBundleRegistry

**Files:**
- Create: `backend/internal/service/software_bundle_registry.go`
- Create: `backend/internal/service/software_bundle_registry_test.go`

**Interfaces:**
- Produces:
```go
type SoftwareBundle struct {
    Platform, ClientFamily, ClientVersion, UserAgent, Originator string
    Runtime, RuntimeVersion string
    Payload map[string]any
}
func CompareSemver(a, b string) int // standard semver including prerelease; do not use CompareVersions
func (r *SoftwareBundleRegistry) Lookup(platform, family, version string) (SoftwareBundle, bool)
func ValidateSoftwareBundle(b SoftwareBundle) error
```
- Consumes: none

Compile-time baselines for current official floors (one row per family, versions you can copy from existing constants: Codex ≥0.144.0 via `NormalizeCodexClientVersion`; Claude from `claude` package defaults; Grok from `IsSupportedCLIVersion`). Panel-published versions can be empty in P1.

Tests: unknown high version not in registry → Lookup false; CompareSemver("2.1.0","2.0.9")>0; prerelease ordering; ValidateSoftwareBundle rejects UA/originator/version mismatch for Codex.

- [ ] TDD then commit `feat: add software bundle registry for identity learning`

---

### Task 4: Extra capacity + identity write validation

**Files:**
- Modify: `backend/internal/service/account_extra_identity_validate.go` (+test)
- Modify: Create/Update/Bulk/Import extra write paths in `account_service.go`, `admin_account.go`, and the admin handlers that already call create/update (only add `ValidateAccountExtraIdentity` + `ValidateAccountCapacityExtra` calls)

**Interfaces:**
- Produces: `ValidateAccountCapacityExtra(extra map[string]any) error`
- Consumes: existing `ValidateAccountExtraIdentity`

Capacity: concurrency empty or 1–32 (do not treat ≤0 as unlimited; reject 999); max_sessions 0–10000; idle 1–1440; rpm_sticky_buffer empty or 1–10000; `codex_fingerprint_mode` enum if present; `enable_tls_fingerprint` bool; `tls_fingerprint_profile_id` ≠ -1; `parseExtraInt` / `identityExtraInt64` must reject non-integer floats (do not truncate 3.7 → 3).

Wire every extra write: Create, Update, Bulk, Import, UpdateExtra, Codex PAT/import.

Tests: Bulk/Import `-1` TLS rejected; concurrency 999 rejected; float 3.7 rejected.

- [ ] TDD then commit `fix: reject illegal identity and capacity extras on all write paths`

---

### Task 5: Repository + GetOrCreate baseline

**Depends:** Task 1 merged, Task 2 merged.

**Files:**
- Create: `backend/internal/repository/account_device_profile_repo.go`
- Create: `backend/internal/service/account_device_service.go` (+tests)
- Modify: `service` port interface in `account_device_service.go`

```go
type AccountDeviceProfileRepository interface {
    GetByAccountID(ctx context.Context, accountID int64) (*AccountDeviceProfile, error)
    InsertBaseline(ctx context.Context, p *AccountDeviceProfile) (*AccountDeviceProfile, error)
    UpdateCAS(ctx context.Context, accountID, expectedRevision int64, next *AccountDeviceProfile) (bool, error)
}

func (s *AccountDeviceService) GetOrCreate(ctx context.Context, account *Account) (*AccountDeviceProfile, error)
```

GetOrCreate: if row exists, return it. Else build baseline (`learning_enabled=false`, `learned_from=baseline`, `transport_family=h1`, generate UUIDs + 32-hex namespace, `gateway_account_uuid` new UUID, `DefaultClientFamily`, Validate, insert). Never insert for shadow (Task 7).

- [ ] TDD then commit `feat: get-or-create baseline account device profiles`

---

### Task 6: Redis is a projection

**Depends:** Task 5.

**Files:**
- Modify: `backend/internal/repository/identity_cache.go` (+test)

New key `device_profile:{accountID}` (JSON of service struct). `Set` on every successful DB write. `Get` is a cache; miss loads DB. On process start, do **not** treat leftover `fingerprint:` keys as authority. Optional: one-shot ignore/delete `fingerprint:` on profile write for that account.

- [ ] TDD then commit `fix: project device profiles to redis and stop treating fingerprint keys as authority`

---

### Task 7: Shadow reads parent

**Depends:** Task 5.

**Files:**
- Modify: `account_device_service.go` only

If `account.IsShadow()` / `parent_account_id != nil`: GetOrCreate parent, return parent profile, do not insert for shadow id. Slots stay on the shadow account's `EffectiveConcurrency()` (do not change spark).

Test: shadow does not insert a second row.

- [ ] TDD then commit `fix: resolve shadow device profiles from the parent account`

---

### Task 8: Claude gateway_account_uuid in RewriteUserID

**Depends:** Task 5.

**Files:**
- Modify: `backend/internal/service/identity_service.go` (+existing tests)

Outbound `user_id` = `user_{device_id}_account_{gateway_account_uuid}_session_{derived}`.
Stop passing `extra.account_uuid` into RewriteUserID. Callers that still pass accountUUID must pass profile.GatewayAccountUUID.

Tests: RewriteUserID body does not contain the real extra UUID; contains gateway UUID.

- [ ] TDD then commit `fix: derive Claude user_id from gateway_account_uuid`

---

### Task 9: Outbound Anthropic reads profile

**Depends:** Task 5, Task 8.

**Files:** `gateway_upstream_request.go`, `gateway_count_tokens.go` (already mimic-aligned in P0). Apply profile UA / device / user_id. Do not force `claude.DefaultHeaders`.

---

### Task 10: Outbound OpenAI/Codex reads profile

**Files:** `openai_codex_identity.go` and Codex header helpers only. originator lowercase, no X-Originator (P0). Device/installation from profile.

---

### Task 11: Outbound Grok reads profile

**Files:** grok header helpers only. Internal consistency for that request. Probes may stay `grok-pager`. Drop unknown `x-grok-*`. No gateway affinity headers to cli-chat-proxy.

---

### Task 12: Outbound Kiro reads profile

**Files:** `kiro_tls_profile.go` / kiro header helpers. machine_id from profile, not derived from refresh_token.

---

### Task 13: Outbound Gemini/Antigravity + probes/WS/passthrough

**Files:** remaining outbound entry points that still mint identity from inbound or extra. Each must call `GetOrCreate` + `ValidateOutboundBundle` and abort to last good profile (do not send half bundle).

---

### Task 14: Learning CAS (P1b)

**Depends:** Task 3, Task 5.

```go
func (s *AccountDeviceService) LearnIfOfficial(ctx context.Context, account *Account, inbound OfficialInbound) (*AccountDeviceProfile, error)
```

Gate: `learning_enabled`; official client only (Claude: exact `claude-cli`; Codex: `codex_cli_rs` TUI/CLI); `Lookup` registry; `CompareSemver(candidate, profile.ClientVersion)>0`; `ValidateSoftwareBundle`; then `UpdateCAS` replacing **software** fields only (UA, originator, versions, payload). Do not change installation/device/machine/gateway_account_uuid/os/arch/session_namespace.

Unknown high version not in registry → no write.
UA change without Stainless change → reject (Claude).

Per-account mutex in-process.

- [ ] TDD then commit `feat: cas-upgrade device software bundles from official traffic`

---

### Task 15: learning_enabled extra / admin

**Files:** DTO + extra key `device_learning_enabled` bool, default absent/false. Do not default the fleet on. No silent reset button.

---

### Task 16: Wire DI

**Files:** `cmd/server/wire.go` + `go generate ./cmd/server`. Update IdentityService / GatewayService constructors and **all** test stubs (`grep type.*Stub.*struct`).

---

### Task 17: Session/thread from session_namespace

**Files:** identity session derivation only.

```
session_id = UUID(HMAC(session_namespace, "sess:"+anchor))
thread_id  = UUID(HMAC(session_namespace, "thread:"+session_id))
window_id  = thread_id + ":0"
```

Two different anchors → two thread_ids. Failover to B uses B's namespace (new window), never A's thread.

---

## Phase schedule (max 20 agents, no file overlap)

| Phase | Parallel tasks | Gate |
|---|---|---|
| A | 1, 2, 3, 4 | merge 1 before 5; merge 2 before 5; merge 3 before 14; merge 4 anytime |
| B | 5 then 6+7 | after 1+2 |
| C | 8, 9, 10, 11, 12, 13 | after 5; one platform per agent |
| D | 14, 15, 17 | after 3+5; 16 last |

## Self-review

- Spec P1a: table, Validate, extra, GetOrCreate, Redis projection, gateway_account_uuid, outbound read, shadow — all tasked.
- Spec P1b: registry, CAS, learning_enabled — tasked.
- P1c h2 wiring and P2 Kiro global version / Codex header order — **out of scope**.
- P0 items already shipped — do not reimplement.
