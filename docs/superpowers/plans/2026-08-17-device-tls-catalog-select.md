# Device TLS Catalog Select Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a learning-enabled account pick one complete catalog TLS set; changing the pick remints the whole device identity and outbound uses that set only.

**Architecture:** Parse `pin:{family}:{os}:{transport}[:variant]` and completeness in service helpers. `extra.device_tls_profile_id` is the operator choice; `account_device_profiles.tls_profile_id` remains outbound truth. GetOrCreate applies the chosen id and OS from the name. Changing the extra id after a row exists calls existing `Reset` so device ids remint. Admin lists complete options for the account platform; the edit modal shows the full summary.

**Tech Stack:** Go unit tests (`-tags=unit`), Vue 3 + Vitest, existing admin TLS profile API.

## Global Constraints

- Extra key is exactly `device_tls_profile_id` (positive int). Do not write `tls_fingerprint_profile_id` for this feature.
- Learn still must not change `tls_profile_id` / device ids. Only operator change remints.
- Incomplete catalog rows never appear in the picker and cannot be saved.
- Auto-pin without a choice still uses unversioned `pin:{family}:{os}:{transport}` only.
- Turning learning off does not remint or clear the current pin.
- Handler must not import `repository`. pnpm only. TDD: failing test first.
- Do not commit unless the user asked in that session; skip commit steps if they have not.

---

## File structure

| File | Responsibility |
|---|---|
| `backend/internal/service/account_device_tls_catalog.go` | Parse pin name, completeness, option DTO, software label |
| `backend/internal/service/account_device_tls_catalog_test.go` | Unit tests for parse / complete / platform filter |
| `backend/internal/service/account_device_tls_pin.go` | Apply chosen id on baseline |
| `backend/internal/service/account_device_service.go` | Baseline OS from choice; GetOrCreate |
| `backend/internal/service/account_extra_identity_validate.go` | Shape-check `device_tls_profile_id` |
| `backend/internal/service/tls_fingerprint_profile_service.go` | `ListCompleteOptions(ctx, platform)` |
| `backend/internal/handler/admin/tls_fingerprint_profile_handler.go` | `GET .../complete?platform=` |
| `backend/internal/handler/admin/account_handler.go` | After update, remint if choice changed |
| `frontend/src/components/account/EditAccountModal.vue` | Picker under device learning |
| `frontend/src/i18n/locales/{zh,en}/admin/accounts.ts` | Copy |
| `docs/device-tls-pin-and-learn.md` | Document shipped behavior |

---

### Task 1: Parse pin names and completeness

**Files:**
- Create: `backend/internal/service/account_device_tls_catalog.go`
- Test: `backend/internal/service/account_device_tls_catalog_test.go`

**Interfaces:**
- Consumes: `ClientFamilyClaudeCode` and siblings in `account_device_profile.go`; `DefaultClientFamily`
- Produces:
  - `type ParsedTLSPinName struct { ClientFamily, OSFamily, Transport, Variant string }`
  - `func ParseTLSPinProfileName(name string) (ParsedTLSPinName, bool)`
  - `func TLSPinCatalogComplete(name string, cipherSuites []uint16, extensions []uint16, alpn []string) bool`
  - `func TLSPinFamilyMatchesPlatform(family, platform string) bool`
  - `func ArchForPinnedOS(platform, osFamily string) string`
  - `const DeviceTLSProfileIDExtraKey = "device_tls_profile_id"`

- [ ] **Step 1: Write the failing tests**

```go
//go:build unit

package service

func TestParseTLSPinProfileName(t *testing.T) {
	got, ok := ParseTLSPinProfileName("pin:claude-code:macos:h1")
	require.True(t, ok)
	require.Equal(t, ParsedTLSPinName{ClientFamily: ClientFamilyClaudeCode, OSFamily: "macos", Transport: TransportH1}, got)

	got, ok = ParseTLSPinProfileName("pin:claude-code:macos:h1:node-26")
	require.True(t, ok)
	require.Equal(t, "node-26", got.Variant)

	_, ok = ParseTLSPinProfileName("claude-code-macos")
	require.False(t, ok)
}

func TestTLSPinCatalogComplete(t *testing.T) {
	require.True(t, TLSPinCatalogComplete("pin:grok-cli:linux:h1", []uint16{1}, []uint16{2}, []string{"http/1.1"}))
	require.False(t, TLSPinCatalogComplete("pin:grok-cli:linux:h1", nil, []uint16{2}, []string{"http/1.1"}))
	require.False(t, TLSPinCatalogComplete("pin:grok-cli:linux:h1", []uint16{1}, []uint16{2}, nil))
}

func TestTLSPinFamilyMatchesPlatform(t *testing.T) {
	require.True(t, TLSPinFamilyMatchesPlatform(ClientFamilyClaudeCode, PlatformAnthropic))
	require.False(t, TLSPinFamilyMatchesPlatform(ClientFamilyClaudeCode, PlatformGrok))
}

func TestArchForPinnedOS(t *testing.T) {
	require.Equal(t, "arm64", ArchForPinnedOS(PlatformAnthropic, "macos"))
	require.Equal(t, "arm64", ArchForPinnedOS(PlatformAnthropic, "linux"))
	require.Equal(t, "x64", ArchForPinnedOS(PlatformOpenAI, "linux"))
	require.Equal(t, "x64", ArchForPinnedOS(PlatformGemini, "windows"))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test -tags=unit -count=1 ./internal/service -run 'TestParseTLSPinProfileName|TestTLSPinCatalogComplete|TestTLSPinFamilyMatchesPlatform|TestArchForPinnedOS'`

Expected: FAIL compile, `ParseTLSPinProfileName` undefined

- [ ] **Step 3: Write minimal implementation**

`ParseTLSPinProfileName`: trim, require prefix `pin:`, split on `:`, need at least family/os/transport; transport must be `h1` or `h2`; family must be a known `DefaultClientFamily` value; os one of `linux|macos|windows|ios|android`. Extra parts join as `Variant`.

`TLSPinCatalogComplete`: parse ok AND `len(cipherSuites)>0` AND `len(extensions)>0` AND `len(alpn)>0`.

`TLSPinFamilyMatchesPlatform`: `DefaultClientFamily(platform) == family`.

`ArchForPinnedOS`: `macos` → `arm64`; `windows` → `x64`; `linux` → `baselineOSArch(platform)` arch (export or duplicate the arch half). If `baselineOSArch` stays unexported, call it and ignore its os, or add `func baselineArch(platform string) string` next to it.

- [ ] **Step 4: Run tests to verify they pass**

Run: same command as Step 2. Expected: PASS

- [ ] **Step 5: Commit** (only if the user asked)

```bash
git add backend/internal/service/account_device_tls_catalog.go backend/internal/service/account_device_tls_catalog_test.go
git commit -m "$(cat <<'EOF'
feat: parse complete TLS catalog pin names

EOF
)"
```

---

### Task 2: Validate extra.device_tls_profile_id shape

**Files:**
- Modify: `backend/internal/service/account_extra_identity_validate.go`
- Test: `backend/internal/service/account_extra_identity_validate_test.go`

**Interfaces:**
- Consumes: `DeviceTLSProfileIDExtraKey`
- Produces: `ValidateAccountExtraIdentity` rejects non-positive / non-int `device_tls_profile_id`; missing/nil/0/empty string is unset

- [ ] **Step 1: Write the failing tests**

```go
func TestValidateAccountExtraIdentityAcceptsDeviceTLSProfileID(t *testing.T) {
	require.NoError(t, ValidateAccountExtraIdentity(map[string]any{}))
	require.NoError(t, ValidateAccountExtraIdentity(map[string]any{DeviceTLSProfileIDExtraKey: nil}))
	require.NoError(t, ValidateAccountExtraIdentity(map[string]any{DeviceTLSProfileIDExtraKey: 0}))
	require.NoError(t, ValidateAccountExtraIdentity(map[string]any{DeviceTLSProfileIDExtraKey: int64(29)}))
}

func TestValidateAccountExtraIdentityRejectsBadDeviceTLSProfileID(t *testing.T) {
	for _, value := range []any{int64(-1), "29", true} {
		err := ValidateAccountExtraIdentity(map[string]any{DeviceTLSProfileIDExtraKey: value})
		require.Error(t, err, "%v", value)
		require.ErrorContains(t, err, DeviceTLSProfileIDExtraKey)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test -tags=unit -count=1 ./internal/service -run 'TestValidateAccountExtraIdentityAcceptsDeviceTLSProfileID|TestValidateAccountExtraIdentityRejectsBadDeviceTLSProfileID'`

Expected: FAIL, `-1` currently accepted or key ignored

- [ ] **Step 3: Write minimal implementation**

In `ValidateAccountExtraIdentity`, after learning bool check, parse optional int via `optionalCapacityInt`. If present and not 0: must be `> 0` (not `-1`). JSON numbers may arrive as `float64`; `optionalCapacityInt` already handles that.

- [ ] **Step 4: Run tests to verify they pass**

Expected: PASS

- [ ] **Step 5: Commit** (only if asked)

---

### Task 3: GetOrCreate honors chosen catalog id

**Files:**
- Modify: `backend/internal/service/account_device_tls_pin.go`
- Modify: `backend/internal/service/account_device_service.go` (`buildAccountDeviceBaseline`, `applyTLSProfilePin` call sites)
- Test: `backend/internal/service/account_device_tls_pin_test.go`

**Interfaces:**
- Consumes: `ParseTLSPinProfileName`, `ArchForPinnedOS`, `DeviceTLSProfileIDExtraKey`, `accountExtraDeviceLearningEnabled`
- Produces:
  - `func ChosenDeviceTLSProfileID(account *Account) *int64` — learning on and extra id `> 0`
  - `applyTLSProfilePin(p *AccountDeviceProfile, account *Account)` — if chosen id set, write that id; else existing name lookup
  - baseline uses chosen pin's OS/transport/arch when `ChosenDeviceTLSProfileID` is set **and** caller passed the catalog name (see below)

Chosen OS cannot be known from id alone inside `applyTLSProfilePin`. Two options — use this one:

`func ApplyChosenTLSPinMeta(p *AccountDeviceProfile, pinName string)` sets `ClientFamily`, `OSFamily`, `TransportFamily`, `Arch` from `ParseTLSPinProfileName` + `ArchForPinnedOS`.

GetOrCreate / baseline: if chosen id is set, tests register a `SetTLSProfilePinLookup` **and** a new `SetTLSProfilePinNameLookup(func(id int64) string)`. Production: `TLSFingerprintProfileService.setLocalCache` also registers `lookupNameByID`.

```go
func lookupTLSProfilePinName(id int64) string
func SetTLSProfilePinNameLookup(fn func(int64) string)
```

`buildAccountDeviceBaseline`: after default OS, if `id := ChosenDeviceTLSProfileID(account); id != nil { if name := lookupTLSProfilePinName(*id); name != "" { ApplyChosenTLSPinMeta(p, name); p.TLSProfileID = id } }` then `applyTLSProfilePin` becomes a no-op because id already set.

- [ ] **Step 1: Write the failing test**

In `account_device_tls_pin_test.go`, add a test that creates an anthropic account with `device_learning_enabled: true` and `device_tls_profile_id: <macos pin id>`. Seed a real `tls_fingerprint_profiles` row named `pin:claude-code:macos:h1` (same FK pattern as existing pin tests). `GetOrCreate` must return `OSFamily=="macos"`, `TLSProfileID` equal to that id, `ClientVersion==claude.CLICurrentVersion`. A second GetOrCreate must not change device id.

Also keep an account **without** the extra key: still linux + standard linux pin if that row exists.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test -tags=unit -count=1 ./internal/service -run TestGetOrCreateHonorsChosenDeviceTLSProfileID`

Expected: FAIL, OS is linux / pin is baseline

- [ ] **Step 3: Write minimal implementation**

Register name-by-id on catalog cache. Thread chosen id through baseline as specified. Do not change `LearnIfOfficial`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test -tags=unit -count=1 ./internal/service -run 'TestGetOrCreateHonorsChosenDeviceTLSProfileID|TestLearnIfOfficialDoesNotChangePinnedTLSProfileID|TestGetOrCreate'`

Expected: PASS (do not break existing pin-once tests)

- [ ] **Step 5: Commit** (only if asked)

---

### Task 4: Changing the choice remints the whole set

**Files:**
- Modify: `backend/internal/handler/admin/account_handler.go` `Update`
- Test: `backend/internal/handler/admin/account_handler_reset_device_profile_test.go` (or new `account_handler_device_tls_select_test.go`)

**Interfaces:**
- Consumes: `DeviceTLSProfileIDExtraKey`, `AccountDeviceService.Reset`, `ChosenDeviceTLSProfileID`
- Produces: after successful `UpdateAccount`, if previous extra id != new extra id (and new account has a device row or new id is set), call `deviceSvc.Reset(ctx, updatedAccount)`

Compare with `optionalCapacityInt` semantics: missing and `0` and `nil` are all “unset”. Unset → unset does not remint. `29` → `25` remints. `29` → unset does **not** remint (spec: turning off / clearing choice keeps current pin). Only remint when the new id is a **positive different** id.

Before Reset, load the catalog row (via `TLSFingerprintProfileService.GetByID` if the handler already has it; otherwise add a tiny helper on device service `func CatalogSelectionUsable(platform string, id int64) error` that uses name lookup + completeness). If the new id is set and not complete or family mismatch, `Update` must fail **before** persist — do this in admin `UpdateAccount` after `ValidateAccountExtraWrites` by injecting a `DeviceTLSCatalogGuard` interface:

```go
type DeviceTLSCatalogGuard interface {
    RejectUnusableDeviceTLSSelection(platform string, extra map[string]any) error
}
```

If wiring `adminServiceImpl` to the TLS profile service is smaller than a new interface, call `TLSFingerprintProfileService` from admin update in the same package (`service`). Prefer that: `func (s *TLSFingerprintProfileService) RejectUnusableDeviceTLSSelection(platform string, extra map[string]any) error` using in-memory cache + `TLSPinCatalogComplete` + `TLSPinFamilyMatchesPlatform`.

Admin `UpdateAccount` already validates extra. Add the catalog guard there so a bad id never hits the DB. Handler remint stays after successful update.

- [ ] **Step 1: Write failing tests**

Service: `TestRejectUnusableDeviceTLSSelection` — incomplete name / wrong platform / missing fields → error containing `device_tls_profile_id`; complete matching pin → nil.

Handler or service: `TestUpdateAccountRemintsWhenDeviceTLSProfileIDChanges` — existing device row + extra 25, update extra to 29, after update `DeviceID` changed and `TLSProfileID==29` and `OSFamily==macos`.

`TestUpdateAccountClearingDeviceTLSProfileIDDoesNotRemint` — id 29 → deleted key, same DeviceID.

- [ ] **Step 2: Run tests to verify they fail**

Expected: FAIL, update does not remint

- [ ] **Step 3: Write minimal implementation**

Guard in `UpdateAccount` / `UpdateAccountExtra` / bulk extra merge when the key is present. Remint in handler `Update` (and `UpdateAccountExtra` path if the UI uses it) using returned account.

If `deviceSvc == nil`, skip remint (same as reset endpoint).

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test -tags=unit -count=1 ./internal/service ./internal/handler/admin -run 'TestRejectUnusable|TestUpdateAccountRemints|TestUpdateAccountClearing'`

Expected: PASS

- [ ] **Step 5: Commit** (only if asked)

---

### Task 5: List complete catalog options

**Files:**
- Modify: `backend/internal/service/tls_fingerprint_profile_service.go`
- Modify: `backend/internal/handler/admin/tls_fingerprint_profile_handler.go`
- Modify: `backend/internal/handler` routes (admin TLS routes)
- Test: existing TLS profile service/handler tests, or new `tls_fingerprint_profile_complete_test.go`

**Interfaces:**
- Consumes: `ParseTLSPinProfileName`, `TLSPinCatalogComplete`, `TLSPinFamilyMatchesPlatform`, compile-time `NewSoftwareBundleRegistry().Lookup(platform, family, currentVersion)`
- Produces:

```go
type DeviceTLSCatalogOption struct {
    ID            int64  `json:"id"`
    Name          string `json:"name"`
    Description   string `json:"description,omitempty"`
    ClientFamily  string `json:"client_family"`
    OSFamily      string `json:"os_family"`
    Transport     string `json:"transport"`
    SoftwareLabel string `json:"software_label"`
}

func (s *TLSFingerprintProfileService) ListCompleteOptions(ctx context.Context, platform string) ([]DeviceTLSCatalogOption, error)
```

`SoftwareLabel`: `bundle.ClientVersion` plus Claude ` " / " + claude.CLIStainlessPackageVersion` when family is claude-code. Skip rows whose family has no registry bundle.

`GET /api/v1/admin/tls-fingerprint-profiles/complete?platform=anthropic`

Register this route **before** `/:id` so `complete` is not parsed as an id.

- [ ] **Step 1: Write the failing test**

In-memory / sqlite catalog: one complete `pin:claude-code:macos:h1`, one missing ALPN, one `pin:grok-cli:macos:h1`. `ListCompleteOptions(ctx, PlatformAnthropic)` returns only the Claude row with `SoftwareLabel` containing `claude.CLICurrentVersion`.

Handler test: GET complete?platform=anthropic → 200 and that JSON shape.

- [ ] **Step 2: Run tests to verify they fail**

Expected: FAIL, method/route missing

- [ ] **Step 3: Write minimal implementation**

Filter `s.localCache` (or `List` then filter). Do not require a DB migration.

- [ ] **Step 4: Run tests to verify they pass**

Expected: PASS

- [ ] **Step 5: Commit** (only if asked)

---

### Task 6: Account edit picker

**Files:**
- Modify: `frontend/src/api/admin/tlsFingerprintProfile.ts` — add `listComplete(platform: string)`
- Modify: `frontend/src/types/index.ts` — `device_tls_profile_id?: number | null` on account extra if that is where extras are typed
- Modify: `frontend/src/components/account/EditAccountModal.vue`
- Modify: `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/admin/accounts.ts`
- Modify: `frontend/src/i18n/locales/en/admin/accounts.ts`

**Interfaces:**
- Consumes: `GET /admin/tls-fingerprint-profiles/complete?platform=`
- Produces: when `deviceLearningEnabled` is true, a `<select>` of complete options with full label; writes `extra.device_tls_profile_id` (omit key when empty)

Label format (zh/en via i18n params):

`{name} · {family} · {os}/{transport} · {software} · 完整可用` plus description if present.

Example: `pin:claude-code:macos:h1 · claude-code · macos/h1 · 2.1.233 / 0.112.1 · 完整可用`

Empty option: “自动（平台默认）” / “Automatic (platform default)”.

Do not show incomplete rows. Do not bind this select to `tls_fingerprint_profile_id`. Leave the old quota TLS router block as-is (outbound already ignores it when a device row exists).

Load options when modal opens and platform is known; reload if platform changes.

- [ ] **Step 1: Write the failing Vue test**

In `EditAccountModal.spec.ts`: with learning toggle on and mocked `listComplete` returning one option, the select is visible and choosing it puts `device_tls_profile_id` on the update payload. Learning off: select absent, payload has no `device_tls_profile_id`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm exec vitest run src/components/account/__tests__/EditAccountModal.spec.ts`

Expected: FAIL, select not found

- [ ] **Step 3: Write minimal implementation**

Wire the select under the existing device learning toggle (`EditAccountModal.vue` around the `deviceLearning` block at line 2805). Persist next to `device_learning_enabled` in the extra payload (~5412).

- [ ] **Step 4: Run tests to verify they pass**

Run: same vitest command. Also `pnpm --dir frontend run typecheck` if types changed.

Expected: PASS

- [ ] **Step 5: Commit** (only if asked)

---

### Task 7: Living docs

**Files:**
- Modify: `docs/device-tls-pin-and-learn.md`
- Modify: `docs/superpowers/specs/2026-08-17-device-tls-catalog-select-design.md` status → 已实现（after code lands）

- [ ] **Step 1: Update the living doc**

Replace “当前代码仍是平台基线自动钉，界面还不能选” with the shipped rules: learning off = auto pin; learning on = optional complete picker; change remints whole set; list display fields; `extra.device_tls_profile_id`.

- [ ] **Step 2: Mark the spec implemented**

Set `状态：已实现` only after Tasks 1–6 are green.

- [ ] **Step 3: Commit** (only if asked)

---

## Spec coverage

| Spec section | Task |
|---|---|
| Directory naming + variant | 1 |
| Complete usable definition | 1, 4, 5 |
| Auto pin without choice | 3 |
| Chosen id on first GetOrCreate | 3 |
| Change = remint whole set | 4 |
| Learning off does not remint | 4 |
| Reject incomplete / wrong platform | 4 |
| Picker full summary | 5, 6 |
| extra.device_tls_profile_id | 2, 3, 6 |
| Learn does not change pin | 3 (existing test kept) |
| Docs | 7 |

## Type consistency

- Extra key: `device_tls_profile_id` / `DeviceTLSProfileIDExtraKey` everywhere
- Parse result: `ParsedTLSPinName`
- List DTO: `DeviceTLSCatalogOption`
- Arch helper: `ArchForPinnedOS`
