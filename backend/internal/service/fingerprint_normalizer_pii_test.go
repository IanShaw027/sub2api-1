package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// ---------------------------------------------------------------------------
// TestBuildAccountEnvProfile
// ---------------------------------------------------------------------------

func TestBuildAccountEnvProfile_Deterministic(t *testing.T) {
	fp := &Fingerprint{StainlessOS: "MacOS"}
	p1 := buildAccountEnvProfile(42, fp)
	p2 := buildAccountEnvProfile(42, fp)

	if p1.Email != p2.Email || p1.GitUser != p2.GitUser || p1.WorkDir != p2.WorkDir ||
		p1.Platform != p2.Platform || p1.Shell != p2.Shell || p1.OSVersion != p2.OSVersion ||
		p1.Arch != p2.Arch || p1.StainlessOS != p2.StainlessOS || p1.StainlessArch != p2.StainlessArch {
		t.Error("same accountID+fp should produce identical profiles")
	}
}

func TestBuildAccountEnvProfile_DifferentAccounts(t *testing.T) {
	fp := &Fingerprint{StainlessOS: "Linux"}
	p1 := buildAccountEnvProfile(100, fp)
	p2 := buildAccountEnvProfile(200, fp)

	if p1.Email == p2.Email {
		t.Error("different accounts should have different emails")
	}
	if p1.GitUser == p2.GitUser && p1.WorkDir == p2.WorkDir {
		t.Error("different accounts should likely differ in gitUser or workDir")
	}
}

func TestBuildAccountEnvProfile_DarwinConsistency(t *testing.T) {
	fp := &Fingerprint{StainlessOS: "MacOS"}
	p := buildAccountEnvProfile(1, fp)

	if p.Platform != "darwin" {
		t.Errorf("platform: got %q, want darwin", p.Platform)
	}
	if p.Shell != "zsh" {
		t.Errorf("shell: got %q, want zsh", p.Shell)
	}
	if p.StainlessOS != "MacOS" {
		t.Errorf("stainlessOS: got %q, want MacOS", p.StainlessOS)
	}
	if !strings.HasPrefix(p.WorkDir, "/Users/") {
		t.Errorf("workDir should start with /Users/, got %q", p.WorkDir)
	}
	if !strings.HasPrefix(p.OSVersion, "Darwin ") {
		t.Errorf("osVersion should start with 'Darwin ', got %q", p.OSVersion)
	}
	if !strings.Contains(p.Email, "@claude-code.local") {
		t.Errorf("email should be @claude-code.local, got %q", p.Email)
	}
	if p.Arch != "arm64" && p.Arch != "x64" {
		t.Errorf("arch should be arm64 or x64, got %q", p.Arch)
	}
	if p.StainlessArch != p.Arch {
		t.Errorf("stainlessArch should match arch: %q vs %q", p.StainlessArch, p.Arch)
	}
}

func TestBuildAccountEnvProfile_LinuxConsistency(t *testing.T) {
	fp := &Fingerprint{StainlessOS: "Linux"}
	p := buildAccountEnvProfile(2, fp)

	if p.Platform != "linux" {
		t.Errorf("platform: got %q, want linux", p.Platform)
	}
	if p.Shell != "bash" {
		t.Errorf("shell: got %q, want bash", p.Shell)
	}
	if p.StainlessOS != "Linux" {
		t.Errorf("stainlessOS: got %q, want Linux", p.StainlessOS)
	}
	if !strings.HasPrefix(p.WorkDir, "/home/") {
		t.Errorf("workDir should start with /home/, got %q", p.WorkDir)
	}
	if !strings.HasPrefix(p.OSVersion, "Linux ") {
		t.Errorf("osVersion should start with 'Linux ', got %q", p.OSVersion)
	}
}

func TestBuildAccountEnvProfile_WindowsConsistency(t *testing.T) {
	fp := &Fingerprint{StainlessOS: "Windows"}
	p := buildAccountEnvProfile(3, fp)

	if p.Platform != "win32" {
		t.Errorf("platform: got %q, want win32", p.Platform)
	}
	if p.Shell != "unknown" {
		t.Errorf("shell: got %q, want unknown", p.Shell)
	}
	if p.StainlessOS != "Windows" {
		t.Errorf("stainlessOS: got %q, want Windows", p.StainlessOS)
	}
	if !strings.HasPrefix(p.WorkDir, `C:\Users\`) {
		t.Errorf("workDir should start with C:\\Users\\, got %q", p.WorkDir)
	}
	if !strings.HasPrefix(p.OSVersion, "Windows ") {
		t.Errorf("osVersion should start with 'Windows ', got %q", p.OSVersion)
	}
	if p.Arch != "x64" {
		t.Errorf("arch should be x64 for win32, got %q", p.Arch)
	}
}

func TestBuildAccountEnvProfile_NilFingerprint(t *testing.T) {
	p := buildAccountEnvProfile(99, nil)
	if p == nil {
		t.Fatal("nil fingerprint should still produce a profile")
	}
	if p.Platform != "darwin" && p.Platform != "linux" {
		t.Errorf("nil fp should fallback to darwin or linux, got %q", p.Platform)
	}
	if p.Email == "" || p.GitUser == "" || p.WorkDir == "" {
		t.Error("nil fp profile should still have email/gitUser/workDir")
	}
}

// ---------------------------------------------------------------------------
// TestNormalizeSystemPromptPII
// ---------------------------------------------------------------------------

func TestNormalizeSystemPromptPII_StringSystem(t *testing.T) {
	profile := &AccountEnvProfile{
		Email:     "user-abc123@claude-code.local",
		GitUser:   "Jordan",
		WorkDir:   "/Users/jordan/projects/webapp",
		Platform:  "darwin",
		Shell:     "zsh",
		OSVersion: "Darwin 24.3.0",
	}

	body := []byte(`{
		"model": "claude-opus-4-6",
		"system": "You are Claude Code.\nThe user's email address is alice@example.com.\nGit user: Alice\n# Environment\nYou have been invoked in the following environment:\n - Primary working directory: /Users/alice/real-project\n - Platform: darwin\n - Shell: bash\n - OS Version: Darwin 23.1.0\nDo your best.",
		"messages": [{"role": "user", "content": "hi"}]
	}`)

	result, wdr := normalizeSystemPromptPII(body, profile)
	if !json.Valid(result) {
		t.Fatal("result is not valid JSON")
	}

	sys := gjson.GetBytes(result, "system").String()

	// Email replaced (not deleted)
	if !strings.Contains(sys, "user-abc123@claude-code.local") {
		t.Error("email should be replaced with profile email")
	}
	if strings.Contains(sys, "alice@example.com") {
		t.Error("original email should not remain")
	}

	// Git user replaced
	if !strings.Contains(sys, "Git user: Jordan") {
		t.Error("git user should be replaced with profile gitUser")
	}
	if strings.Contains(sys, "Git user: Alice") {
		t.Error("original git user should not remain")
	}

	// Working directory replaced with the full fake path (no real project basename leaked).
	if !strings.Contains(sys, "/Users/jordan/projects/webapp") {
		t.Error("working directory should be replaced with the full profile workDir")
	}
	if strings.Contains(sys, "/Users/alice/real-project") {
		t.Error("original working directory should not remain")
	}

	// Platform replaced
	if strings.Contains(sys, "Platform: bash") {
		t.Error("platform should not be 'bash'") // sanity check regression
	}

	// Shell replaced
	if !strings.Contains(sys, "Shell: zsh") {
		t.Error("shell should be replaced with profile shell")
	}

	// OS Version replaced
	if !strings.Contains(sys, "Darwin 24.3.0") {
		t.Error("OS version should be replaced with profile osVersion")
	}
	if strings.Contains(sys, "Darwin 23.1.0") {
		t.Error("original OS version should not remain")
	}

	// Non-PII preserved
	if !strings.Contains(sys, "You are Claude Code.") {
		t.Error("non-PII content should be preserved")
	}
	if !strings.Contains(sys, "Do your best.") {
		t.Error("non-PII content should be preserved")
	}

	// Model preserved
	if !gjson.GetBytes(result, "model").Exists() {
		t.Error("model field was lost")
	}

	// WorkDirRewrite captured
	if wdr == nil {
		t.Fatal("WorkDirRewrite should be captured")
	}
	if wdr.RealDir != "/Users/alice/real-project" {
		t.Errorf("wdr.RealDir: got %q, want /Users/alice/real-project", wdr.RealDir)
	}
	// FakeDir 现为完整假路径（不再保留真实 basename real-project，避免真实项目名外发）。
	if wdr.FakeDir != "/Users/jordan/projects/webapp" {
		t.Errorf("wdr.FakeDir: got %q, want /Users/jordan/projects/webapp", wdr.FakeDir)
	}
	if strings.Contains(sys, "real-project") {
		t.Error("real project basename must not leak into the fake work dir")
	}
}

func TestNormalizeSystemPromptPII_StripsGitStatusMetadata(t *testing.T) {
	profile := &AccountEnvProfile{
		Email:     "user-abc123@claude-code.local",
		GitUser:   "Jordan",
		WorkDir:   "/Users/jordan/projects/webapp",
		Platform:  "darwin",
		Shell:     "zsh",
		OSVersion: "Darwin 24.3.0",
	}

	body := []byte(`{
		"model": "claude-opus-4-6",
		"system": "You are Claude Code.\nIs directory a git repo: Yes\nCurrent branch: feature/real-secret\nMain branch: main\nRecent commits:\n- a1b2c3d fix internal project codename\n- d4e5f6g add private adapter\nStatus:\nM backend/internal/service/private.go\n?? secrets/roadmap.md\n# Environment\nYou have been invoked in the following environment:\n - Primary working directory: /Users/alice/real-project\n - Platform: darwin\n - Shell: bash\n - OS Version: Darwin 23.1.0\nDo your best.",
		"messages": [{"role": "user", "content": "hi"}]
	}`)

	result, _ := normalizeSystemPromptPII(body, profile)
	require.True(t, json.Valid(result))

	sys := gjson.GetBytes(result, "system").String()
	require.Contains(t, sys, "Is directory a git repo: Yes")
	require.NotContains(t, sys, "feature/real-secret")
	require.NotContains(t, sys, "fix internal project codename")
	require.NotContains(t, sys, "backend/internal/service/private.go")
	require.NotContains(t, sys, "secrets/roadmap.md")
	require.NotContains(t, sys, "Current branch:")
	require.NotContains(t, sys, "Main branch:")
	require.NotContains(t, sys, "Recent commits:")
	require.NotContains(t, sys, "\nStatus:\n")
	require.Contains(t, sys, "# Environment")
	require.Contains(t, sys, "Do your best.")
}

func TestNormalizeSystemPromptPII_NilProfileStillStripsGitStatusMetadata(t *testing.T) {
	body := []byte(`{
		"system": "Is directory a git repo: Yes\nCurrent branch: feature/real-secret\nRecent commits:\n- abc secret\nStatus:\nM private/file.go\nDo your best.",
		"messages": []
	}`)

	result, wdr := normalizeSystemPromptPII(body, nil)
	require.True(t, json.Valid(result))
	require.Nil(t, wdr)

	sys := gjson.GetBytes(result, "system").String()
	require.Contains(t, sys, "Is directory a git repo: Yes")
	require.NotContains(t, sys, "feature/real-secret")
	require.NotContains(t, sys, "private/file.go")
	require.Contains(t, sys, "Do your best.")
}

func TestNormalizeSystemPromptPII_ArraySystem(t *testing.T) {
	profile := &AccountEnvProfile{
		Email:     "user-def456@claude-code.local",
		GitUser:   "Sam",
		WorkDir:   "/home/sam/projects/api-server",
		Platform:  "linux",
		Shell:     "bash",
		OSVersion: "Linux 6.8.0",
	}

	body := []byte(`{
		"model": "claude-opus-4-6",
		"system": [
			{"type": "text", "text": "You are Claude Code.", "cache_control": {"type": "ephemeral"}},
			{"type": "text", "text": "The user's email address is bob@corp.io.\nGit user: Bob Smith\nPrimary working directory: /home/bob/myproject"},
			{"type": "text", "text": "# Environment\n - Platform: darwin\n - Shell: zsh\n - OS Version: Darwin 24.3.0"}
		],
		"messages": [{"role": "user", "content": "hi"}]
	}`)

	result, wdr := normalizeSystemPromptPII(body, profile)
	if !json.Valid(result) {
		t.Fatal("result is not valid JSON")
	}

	// Block 0: untouched (no PII)
	block0 := gjson.GetBytes(result, "system.0.text").String()
	if block0 != "You are Claude Code." {
		t.Errorf("block 0 was incorrectly modified: %q", block0)
	}
	// cache_control preserved
	if !gjson.GetBytes(result, "system.0.cache_control").Exists() {
		t.Error("cache_control was lost from block 0")
	}

	// Block 1: email/git/workdir replaced
	block1 := gjson.GetBytes(result, "system.1.text").String()
	if !strings.Contains(block1, "user-def456@claude-code.local") {
		t.Errorf("block 1 should contain profile email, got: %q", block1)
	}
	if !strings.Contains(block1, "Git user: Sam") {
		t.Errorf("block 1 should contain profile git user, got: %q", block1)
	}
	// 完整假路径，不再保留真实 basename myproject。
	if !strings.Contains(block1, "/home/sam/projects/api-server") {
		t.Errorf("block 1 should contain full profile workDir, got: %q", block1)
	}
	if strings.Contains(block1, "myproject") {
		t.Errorf("real project basename must not leak into block 1, got: %q", block1)
	}
	if strings.Contains(block1, "bob@corp.io") || strings.Contains(block1, "Bob Smith") {
		t.Error("original PII should not remain in block 1")
	}

	// Block 2: platform/shell/os replaced
	block2 := gjson.GetBytes(result, "system.2.text").String()
	if !strings.Contains(block2, "Platform: linux") {
		t.Errorf("block 2 should contain profile platform, got: %q", block2)
	}
	if !strings.Contains(block2, "Shell: bash") {
		t.Errorf("block 2 should contain profile shell, got: %q", block2)
	}
	if !strings.Contains(block2, "Linux 6.8.0") {
		t.Errorf("block 2 should contain profile OS version, got: %q", block2)
	}

	// WorkDirRewrite
	if wdr == nil {
		t.Fatal("WorkDirRewrite should be captured")
	}
	if wdr.RealDir != "/home/bob/myproject" {
		t.Errorf("wdr.RealDir: got %q", wdr.RealDir)
	}
}

func TestNormalizeSystemPromptPII_NilProfile(t *testing.T) {
	body := []byte(`{
		"system": "The user's email address is test@test.com.\nGit user: Test",
		"messages": []
	}`)

	result, wdr := normalizeSystemPromptPII(body, nil)
	if !json.Valid(result) {
		t.Fatal("result is not valid JSON")
	}

	sys := gjson.GetBytes(result, "system").String()
	// nil profile should fall back to deletion (not replacement)
	if strings.Contains(sys, "test@test.com") {
		t.Error("email should be deleted when profile is nil")
	}
	if strings.Contains(sys, "Git user:") {
		t.Error("git user should be deleted when profile is nil")
	}
	if wdr != nil {
		t.Error("wdr should be nil when profile is nil")
	}
}

func TestNormalizeSystemPromptPII_NoSystem(t *testing.T) {
	body := []byte(`{"model": "claude-opus-4-6", "messages": []}`)
	profile := &AccountEnvProfile{Email: "x@y.z", GitUser: "X", WorkDir: "/x", Platform: "linux"}

	result, wdr := normalizeSystemPromptPII(body, profile)
	if string(result) != string(body) {
		t.Error("body without system field should be returned unchanged")
	}
	if wdr != nil {
		t.Error("wdr should be nil when no system field")
	}
}

func TestNormalizeSystemPromptPII_EmptyBody(t *testing.T) {
	profile := &AccountEnvProfile{Email: "x@y.z"}

	result, _ := normalizeSystemPromptPII(nil, profile)
	if result != nil {
		t.Error("nil body should return nil")
	}
	result, _ = normalizeSystemPromptPII([]byte{}, profile)
	if len(result) != 0 {
		t.Error("empty body should return empty")
	}
}

// ---------------------------------------------------------------------------
// TestWorkDirRewrite
// ---------------------------------------------------------------------------

func TestReplaceWorkDirInBody(t *testing.T) {
	wdr := &WorkDirRewrite{
		RealDir: "/Users/alice/real-project",
		FakeDir: "/Users/jordan/projects/webapp",
	}

	body := []byte(`{"messages":[{"role":"user","content":"Read /Users/alice/real-project/main.go"}]}`)
	result := replaceWorkDirInBody(body, wdr)

	if strings.Contains(string(result), "/Users/alice/real-project") {
		t.Error("real dir should be replaced")
	}
	if !strings.Contains(string(result), "/Users/jordan/projects/webapp") {
		t.Error("fake dir should be present")
	}
}

func TestReplaceWorkDirInBody_Windows(t *testing.T) {
	wdr := &WorkDirRewrite{
		RealDir: `C:\Users\alice\project`,
		FakeDir: `C:\Users\sam\projects\webapp`,
	}

	// JSON body with escaped backslashes
	body := []byte(`{"messages":[{"role":"user","content":"Read C:\\Users\\alice\\project\\main.go"}]}`)
	result := replaceWorkDirInBody(body, wdr)

	if strings.Contains(string(result), `C:\\Users\\alice\\project`) {
		t.Error("real dir (escaped) should be replaced")
	}
	if !strings.Contains(string(result), `C:\\Users\\sam\\projects\\webapp`) {
		t.Error("fake dir (escaped) should be present")
	}
}

func TestReverseWorkDir(t *testing.T) {
	wdr := &WorkDirRewrite{
		RealDir: "/Users/alice/real-project",
		FakeDir: "/Users/jordan/projects/webapp",
	}

	chunk := []byte(`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"file at /Users/jordan/projects/webapp/main.go"}}`)
	result := doReverseWorkDir(chunk, wdr)

	if strings.Contains(string(result), "/Users/jordan/projects/webapp") {
		t.Error("fake dir should be reversed")
	}
	if !strings.Contains(string(result), "/Users/alice/real-project") {
		t.Error("real dir should be restored")
	}
}

func TestReplaceWorkDirInBody_NilWdr(t *testing.T) {
	body := []byte(`{"foo":"bar"}`)
	result := replaceWorkDirInBody(body, nil)
	if string(result) != string(body) {
		t.Error("nil wdr should return body unchanged")
	}
}

func TestReplaceWorkDirInBody_SameDir(t *testing.T) {
	wdr := &WorkDirRewrite{RealDir: "/same/path", FakeDir: "/same/path"}
	body := []byte(`{"foo":"bar"}`)
	result := replaceWorkDirInBody(body, wdr)
	if string(result) != string(body) {
		t.Error("same real/fake should return body unchanged")
	}
}

// ---------------------------------------------------------------------------
// TestClientMetadataNotInjected
// ---------------------------------------------------------------------------

func TestClientMetadataDeletedForAnthropic(t *testing.T) {
	// Simulate a body that a third-party harness might send with client_metadata
	body := []byte(`{"model":"claude-opus-4-6","client_metadata":{"process":{"memory":8192},"env":{"OS":"Mac OS X"}},"messages":[]}`)

	result := safeDeleteJSONKey(body, "client_metadata")
	if gjson.GetBytes(result, "client_metadata").Exists() {
		t.Error("client_metadata should be deleted")
	}
	// Other fields preserved
	if !gjson.GetBytes(result, "model").Exists() {
		t.Error("model field should be preserved")
	}
}

func TestClientMetadataNotPresentInCleanBody(t *testing.T) {
	// A real CC CLI body doesn't have client_metadata
	body := []byte(`{"model":"claude-opus-4-6","messages":[],"metadata":{"user_id":"test"}}`)

	result := safeDeleteJSONKey(body, "client_metadata")
	if string(result) != string(body) {
		t.Error("body without client_metadata should be unchanged after delete attempt")
	}
}

func TestRewriteSystemReminderEnvBlocksWithWorkDirRewrite_CapturesDirectoryForResponseRestore(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"<system-reminder>Platform: linux\nShell: bash\nOS Version: Linux 6.8\nWorking directory: /opt/sub2api\n</system-reminder>\nRead /opt/sub2api/CLAUDE.md"}]}]}`)
	profile := &AccountEnvProfile{
		GitUser:   "Jordan",
		WorkDir:   "/Users/jordan/projects/webapp",
		Platform:  "darwin",
		Shell:     "zsh",
		OSVersion: "Darwin 24.3.0",
	}

	got, wdr := RewriteSystemReminderEnvBlocksWithWorkDirRewrite(body, profile)
	if wdr == nil {
		t.Fatal("system-reminder directory rewrite should be captured for response restore")
	}
	if wdr.RealDir != "/opt/sub2api" {
		t.Fatalf("wdr.RealDir = %q, want /opt/sub2api", wdr.RealDir)
	}
	// 完整假路径，不再保留真实 basename sub2api。
	if wdr.FakeDir != "/Users/jordan/projects/webapp" {
		t.Fatalf("wdr.FakeDir = %q, want /Users/jordan/projects/webapp", wdr.FakeDir)
	}
	replaced := replaceWorkDirInBody(got, wdr)
	if strings.Contains(string(replaced), "/opt/sub2api") {
		t.Fatalf("real directory should be replaced throughout body: %s", replaced)
	}
	if !strings.Contains(string(replaced), "/Users/jordan/projects/webapp/CLAUDE.md") {
		t.Fatalf("non-reminder body paths should use fake directory after full-body replacement: %s", replaced)
	}
}

func TestShouldApplyClaudeAntiBanBodyTransformsForAccount_RequiresOAuthAndEnabled(t *testing.T) {
	oauth := &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	setup := &Account{Platform: PlatformAnthropic, Type: AccountTypeSetupToken}
	apiKey := &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
	openAI := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	if !shouldApplyClaudeAntiBanBodyTransformsForAccount(oauth, true) {
		t.Fatal("enabled Anthropic OAuth should apply Claude anti-ban body transforms")
	}
	if !shouldApplyClaudeAntiBanBodyTransformsForAccount(setup, true) {
		t.Fatal("enabled Anthropic setup-token should apply Claude anti-ban body transforms")
	}
	if shouldApplyClaudeAntiBanBodyTransformsForAccount(apiKey, true) {
		t.Fatal("Anthropic APIKey must not apply Claude anti-ban body transforms")
	}
	if shouldApplyClaudeAntiBanBodyTransformsForAccount(oauth, false) {
		t.Fatal("disabled anti-ban platform toggle must disable Claude body transforms")
	}
	if shouldApplyClaudeAntiBanBodyTransformsForAccount(openAI, true) {
		t.Fatal("non-Anthropic OAuth must not apply Claude body transforms")
	}
}

// ---------------------------------------------------------------------------
// Helpers (shared across tests)
// ---------------------------------------------------------------------------
