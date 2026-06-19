package main

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestRunMainServerRequiresRuntimeJWTSecret(t *testing.T) {
	viper.Reset()
	t.Setenv("DATA_DIR", t.TempDir())
	t.Setenv("JWT_SECRET", "")

	err := runMainServer()
	if err == nil {
		t.Fatal("runMainServer() should reject missing jwt.secret before application initialization")
	}
	if !strings.Contains(err.Error(), "jwt.secret is required") {
		t.Fatalf("runMainServer() error = %v, want jwt.secret is required", err)
	}
}

func TestServeApplicationCleansUpOnBindError(t *testing.T) {
	var cleaned atomic.Bool
	app := &Application{
		Server: &http.Server{
			Addr: "\n",
		},
		Cleanup: func() {
			cleaned.Store(true)
		},
	}

	err := serveApplication(app, nil)
	if err == nil {
		t.Fatal("serveApplication() should fail for invalid listen address")
	}
	if !cleaned.Load() {
		t.Fatal("serveApplication() did not run cleanup after bind error")
	}
}

func TestServerShutdownTimeoutFromEnv(t *testing.T) {
	t.Setenv("SERVER_SHUTDOWN_TIMEOUT", "")
	if got := serverShutdownTimeout(); got != 10*time.Second {
		t.Fatalf("default serverShutdownTimeout() = %s, want 10s", got)
	}

	t.Setenv("SERVER_SHUTDOWN_TIMEOUT", "10s")
	if got := serverShutdownTimeout(); got != 10*time.Second {
		t.Fatalf("serverShutdownTimeout() = %s, want 10s", got)
	}

	t.Setenv("SERVER_SHUTDOWN_TIMEOUT", "invalid")
	if got := serverShutdownTimeout(); got != 10*time.Second {
		t.Fatalf("invalid serverShutdownTimeout() = %s, want 10s fallback", got)
	}
}
