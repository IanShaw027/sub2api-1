package setup

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
)

var runtimeSetupSecret struct {
	sync.RWMutex
	value string
}

const (
	SetupAllowRemoteEnv          = "SETUP_ALLOW_REMOTE"
	SetupBootstrapSecretEnv      = "SETUP_BOOTSTRAP_SECRET"
	SetupBootstrapSecretHeader   = "X-Setup-Bootstrap-Secret"
	minSetupBootstrapSecretBytes = 32
)

// ResolveServerAddress preserves the configured listener. Non-loopback setup
// listeners require either an explicit or an automatically generated bootstrap
// secret, so Docker port mapping remains usable without exposing setup actions.
func ResolveServerAddress(configuredAddress string) (string, error) {
	configuredAddress = strings.TrimSpace(configuredAddress)
	if configuredAddress == "" {
		return "", fmt.Errorf("setup server address is empty")
	}

	host, _, err := net.SplitHostPort(configuredAddress)
	if err != nil {
		return "", fmt.Errorf("invalid setup server address %q: %w", configuredAddress, err)
	}

	if remoteSetupEnabled() {
		if err := validateRemoteSetupSecret(); err != nil {
			return "", err
		}
		setRuntimeSetupSecret("")
		return configuredAddress, nil
	}

	if isLoopbackSetupHost(host) {
		setRuntimeSetupSecret("")
		return configuredAddress, nil
	}
	secretBytes := make([]byte, minSetupBootstrapSecretBytes)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", fmt.Errorf("generate setup bootstrap secret: %w", err)
	}
	setRuntimeSetupSecret(hex.EncodeToString(secretBytes))
	return configuredAddress, nil
}

func isLoopbackSetupHost(host string) bool {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func setRuntimeSetupSecret(secret string) {
	runtimeSetupSecret.Lock()
	runtimeSetupSecret.value = strings.TrimSpace(secret)
	runtimeSetupSecret.Unlock()
}

// BootstrapSecret returns the active setup secret for startup URL logging.
func BootstrapSecret() string {
	if remoteSetupEnabled() {
		return strings.TrimSpace(os.Getenv(SetupBootstrapSecretEnv))
	}
	runtimeSetupSecret.RLock()
	defer runtimeSetupSecret.RUnlock()
	return runtimeSetupSecret.value
}

func remoteSetupEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(SetupAllowRemoteEnv))) {
	case "true", "1", "yes":
		return true
	default:
		return false
	}
}

func validateRemoteSetupSecret() error {
	if len(strings.TrimSpace(os.Getenv(SetupBootstrapSecretEnv))) < minSetupBootstrapSecretBytes {
		return fmt.Errorf("%s must contain at least %d bytes when %s is enabled", SetupBootstrapSecretEnv, minSetupBootstrapSecretBytes, SetupAllowRemoteEnv)
	}
	return nil
}

func setupAccessAllowed(r *http.Request) bool {
	if r == nil {
		return false
	}
	expected := BootstrapSecret()
	if expected != "" {
		provided := strings.TrimSpace(r.Header.Get(SetupBootstrapSecretHeader))
		return len(expected) >= minSetupBootstrapSecretBytes &&
			len(expected) == len(provided) &&
			subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) == 1
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return false
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	return ip != nil && ip.IsLoopback()
}
