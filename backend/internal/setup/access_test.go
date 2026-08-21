package setup

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestResolveServerAddressGeneratesSecretForRemoteListener(t *testing.T) {
	t.Setenv(SetupAllowRemoteEnv, "")
	t.Setenv(SetupBootstrapSecretEnv, "")

	got, err := ResolveServerAddress("0.0.0.0:8080")
	if err != nil {
		t.Fatalf("ResolveServerAddress() error = %v", err)
	}
	if got != "0.0.0.0:8080" {
		t.Fatalf("ResolveServerAddress() = %q, want configured listener", got)
	}
	if len(BootstrapSecret()) < minSetupBootstrapSecretBytes {
		t.Fatalf("BootstrapSecret() length = %d, want at least %d", len(BootstrapSecret()), minSetupBootstrapSecretBytes)
	}
}

func TestResolveServerAddressLoopbackNeedsNoSecret(t *testing.T) {
	t.Setenv(SetupAllowRemoteEnv, "")
	t.Setenv(SetupBootstrapSecretEnv, "")

	got, err := ResolveServerAddress("127.0.0.1:8080")
	if err != nil {
		t.Fatalf("ResolveServerAddress() error = %v", err)
	}
	if got != "127.0.0.1:8080" || BootstrapSecret() != "" {
		t.Fatalf("ResolveServerAddress() = %q, secret=%q", got, BootstrapSecret())
	}
}

func TestResolveServerAddressRemoteRequiresSecret(t *testing.T) {
	t.Setenv(SetupAllowRemoteEnv, "true")
	t.Setenv(SetupBootstrapSecretEnv, "too-short")

	if _, err := ResolveServerAddress("0.0.0.0:8080"); err == nil {
		t.Fatal("ResolveServerAddress() error = nil, want missing bootstrap secret rejection")
	}
}

func TestResolveServerAddressRemoteExplicitlyEnabled(t *testing.T) {
	t.Setenv(SetupAllowRemoteEnv, "true")
	t.Setenv(SetupBootstrapSecretEnv, "0123456789abcdef0123456789abcdef")

	got, err := ResolveServerAddress("0.0.0.0:8080")
	if err != nil {
		t.Fatalf("ResolveServerAddress() error = %v", err)
	}
	if got != "0.0.0.0:8080" {
		t.Fatalf("ResolveServerAddress() = %q, want configured remote address", got)
	}
}

func TestSetupGuardDefaultAllowsOnlyLoopback(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	t.Setenv(SetupAllowRemoteEnv, "")
	t.Setenv(SetupBootstrapSecretEnv, "")
	if _, err := ResolveServerAddress("127.0.0.1:8080"); err != nil {
		t.Fatalf("ResolveServerAddress() error = %v", err)
	}

	if got := runSetupGuardRequest(t, "127.0.0.1:41000", ""); got != http.StatusNoContent {
		t.Fatalf("loopback status = %d, want %d", got, http.StatusNoContent)
	}
	if got := runSetupGuardRequest(t, "203.0.113.10:41000", ""); got != http.StatusForbidden {
		t.Fatalf("remote status = %d, want %d", got, http.StatusForbidden)
	}
}

func TestSetupGuardGeneratedSecretAllowsDockerStyleRemoteAccess(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	t.Setenv(SetupAllowRemoteEnv, "")
	t.Setenv(SetupBootstrapSecretEnv, "")
	if _, err := ResolveServerAddress("0.0.0.0:8080"); err != nil {
		t.Fatalf("ResolveServerAddress() error = %v", err)
	}
	secret := BootstrapSecret()
	if got := runSetupGuardRequest(t, "172.17.0.1:41000", ""); got != http.StatusForbidden {
		t.Fatalf("missing secret status = %d, want %d", got, http.StatusForbidden)
	}
	if got := runSetupGuardRequest(t, "172.17.0.1:41000", secret); got != http.StatusNoContent {
		t.Fatalf("matching generated secret status = %d, want %d", got, http.StatusNoContent)
	}
}

func TestSetupGuardRemoteRequiresMatchingSecret(t *testing.T) {
	const secret = "0123456789abcdef0123456789abcdef"
	t.Setenv("DATA_DIR", t.TempDir())
	t.Setenv(SetupAllowRemoteEnv, "yes")
	t.Setenv(SetupBootstrapSecretEnv, secret)

	if got := runSetupGuardRequest(t, "203.0.113.10:41000", ""); got != http.StatusForbidden {
		t.Fatalf("missing secret status = %d, want %d", got, http.StatusForbidden)
	}
	if got := runSetupGuardRequest(t, "203.0.113.10:41000", "wrong-secret"); got != http.StatusForbidden {
		t.Fatalf("wrong secret status = %d, want %d", got, http.StatusForbidden)
	}
	if got := runSetupGuardRequest(t, "203.0.113.10:41000", secret); got != http.StatusNoContent {
		t.Fatalf("matching secret status = %d, want %d", got, http.StatusNoContent)
	}
}

func TestSetupGuardRejectsInstalledSystemBeforeAccessCheck(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv(SetupAllowRemoteEnv, "true")
	t.Setenv(SetupBootstrapSecretEnv, "0123456789abcdef0123456789abcdef")
	if err := os.WriteFile(filepath.Join(dataDir, InstallLockFile), nil, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if got := runSetupGuardRequest(t, "203.0.113.10:41000", "0123456789abcdef0123456789abcdef"); got != http.StatusForbidden {
		t.Fatalf("installed status = %d, want %d", got, http.StatusForbidden)
	}
}

func runSetupGuardRequest(t *testing.T, remoteAddr, secret string) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/setup/install", setupGuard(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodPost, "/setup/install", nil)
	req.RemoteAddr = remoteAddr
	if secret != "" {
		req.Header.Set(SetupBootstrapSecretHeader, secret)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder.Code
}
