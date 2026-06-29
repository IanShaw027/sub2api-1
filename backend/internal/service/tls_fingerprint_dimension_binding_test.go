//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/stretchr/testify/require"
)

func TestGetTLSFingerprintBindings(t *testing.T) {
	t.Run("nil when absent", func(t *testing.T) {
		a := &Account{Extra: map[string]any{}}
		require.Nil(t, a.GetTLSFingerprintBindings())
	})

	t.Run("parses mixed numeric types and normalizes keys", func(t *testing.T) {
		a := &Account{Extra: map[string]any{
			"tls_fingerprint_bindings": map[string]any{
				"Windows":         float64(11),
				"macos/Codex-CLI": float64(42),
				"linux":           int64(9),
			},
		}}
		got := a.GetTLSFingerprintBindings()
		require.Equal(t, int64(11), got["windows"])
		require.Equal(t, int64(42), got["macos/codex-cli"])
		require.Equal(t, int64(9), got["linux"])
	})
}

func TestGetTLSFingerprintProfileIDForDimension(t *testing.T) {
	t.Run("os+client beats os", func(t *testing.T) {
		a := &Account{Extra: map[string]any{
			"tls_fingerprint_bindings": map[string]any{
				"macos":           float64(1),
				"macos/codex-cli": float64(2),
			},
		}}
		require.Equal(t, int64(2), a.GetTLSFingerprintProfileIDForDimension("macos", "codex-cli"))
		require.Equal(t, int64(1), a.GetTLSFingerprintProfileIDForDimension("macos", "chatgpt-desktop"))
	})

	t.Run("falls back to legacy single value when no binding hit", func(t *testing.T) {
		a := &Account{Extra: map[string]any{
			"tls_fingerprint_profile_id": float64(99),
			"tls_fingerprint_bindings": map[string]any{
				"windows": float64(5),
			},
		}}
		// macos not in matrix → legacy 99
		require.Equal(t, int64(99), a.GetTLSFingerprintProfileIDForDimension("macos", ""))
	})

	t.Run("falls back to legacy when no matrix at all", func(t *testing.T) {
		a := &Account{Extra: map[string]any{
			"tls_fingerprint_profile_id": float64(7),
		}}
		require.Equal(t, int64(7), a.GetTLSFingerprintProfileIDForDimension("linux", "claude-code"))
	})

	t.Run("zero binding value is ignored and degrades to legacy", func(t *testing.T) {
		a := &Account{Extra: map[string]any{
			"tls_fingerprint_profile_id": float64(3),
			"tls_fingerprint_bindings": map[string]any{
				"macos": float64(0),
			},
		}}
		require.Equal(t, int64(3), a.GetTLSFingerprintProfileIDForDimension("macos", ""))
	})
}

func TestGetTLSFingerprintDefaultOS(t *testing.T) {
	require.Equal(t, "macos", (&Account{Extra: map[string]any{"tls_fingerprint_default_os": "macOS"}}).GetTLSFingerprintDefaultOS())
	require.Equal(t, "", (&Account{Extra: map[string]any{"tls_fingerprint_default_os": "solaris"}}).GetTLSFingerprintDefaultOS())
	require.Equal(t, "", (&Account{Extra: map[string]any{}}).GetTLSFingerprintDefaultOS())
}

func TestResolveTLSProfileForDimension(t *testing.T) {
	svc := &TLSFingerprintProfileService{
		localCache: map[int64]*model.TLSFingerprintProfile{
			2:  {ID: 2, Name: "macos-codex", Platform: "openai", OS: "macos", ClientType: "codex-cli"},
			50: {ID: 50, Name: "legacy", Platform: "openai"},
		},
	}

	t.Run("disabled returns nil", func(t *testing.T) {
		a := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{}}
		require.Nil(t, svc.ResolveTLSProfileForDimension(a, "macos", "codex-cli"))
	})

	t.Run("resolves matrix dimension", func(t *testing.T) {
		a := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{
			"enable_tls_fingerprint":   true,
			"tls_fingerprint_bindings": map[string]any{"macos/codex-cli": float64(2)},
		}}
		p := svc.ResolveTLSProfileForDimension(a, "macos", "codex-cli")
		require.NotNil(t, p)
		require.Equal(t, "macos-codex", p.Name)
	})

	t.Run("degrades to legacy single value", func(t *testing.T) {
		a := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{
			"enable_tls_fingerprint":     true,
			"tls_fingerprint_profile_id": float64(50),
		}}
		p := svc.ResolveTLSProfileForDimension(a, "windows", "")
		require.NotNil(t, p)
		require.Equal(t, "legacy", p.Name)
	})

	t.Run("unknown id yields built-in default", func(t *testing.T) {
		a := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{
			"enable_tls_fingerprint":     true,
			"tls_fingerprint_profile_id": float64(99999),
		}}
		p := svc.ResolveTLSProfileForDimension(a, "linux", "")
		require.NotNil(t, p)
		require.Contains(t, p.Name, "Built-in Default")
	})
}

func TestResolveTLSProfileForTransport_DefaultOSRespectsTransport(t *testing.T) {
	svc := &TLSFingerprintProfileService{
		localCache: map[int64]*model.TLSFingerprintProfile{
			88: {ID: 88, Name: "WS Only", Platform: "openai", OS: "macos", Transport: "websocket-h2"},
		},
	}
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{
		"enable_tls_fingerprint":     true,
		"tls_fingerprint_default_os": "macos",
		"tls_fingerprint_bindings":   map[string]any{"macos": float64(88)},
	}}

	profile := svc.ResolveTLSProfileForTransport(account, "http")

	require.NotNil(t, profile)
	require.NotEqual(t, "WS Only", profile.Name)
	require.Contains(t, profile.Name, "Built-in Default")
}
