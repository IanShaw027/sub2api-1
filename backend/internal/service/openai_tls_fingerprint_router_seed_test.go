//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

func TestSeedTLSFingerprintRuntimeHeaders(t *testing.T) {
	t.Run("seeds empty runtime from template", func(t *testing.T) {
		runtime := openAITLSFingerprintRuntime{}
		seedTLSFingerprintRuntimeHeaders(&runtime, &tlsfingerprint.Profile{
			UserAgent:  "codex_cli_rs/1.0 (macOS 15.3; arm64)",
			Originator: "codex_cli_rs",
		})
		require.Equal(t, "codex_cli_rs/1.0 (macOS 15.3; arm64)", runtime.UpstreamUserAgent)
		require.Equal(t, "codex_cli_rs", runtime.UpstreamOriginator)
	})

	t.Run("existing runtime values win over template", func(t *testing.T) {
		runtime := openAITLSFingerprintRuntime{
			UpstreamUserAgent:  "router-override-ua",
			UpstreamOriginator: "router-override-origin",
		}
		seedTLSFingerprintRuntimeHeaders(&runtime, &tlsfingerprint.Profile{
			UserAgent:  "template-ua",
			Originator: "template-origin",
		})
		require.Equal(t, "router-override-ua", runtime.UpstreamUserAgent)
		require.Equal(t, "router-override-origin", runtime.UpstreamOriginator)
	})

	t.Run("empty router value falls back to template per field", func(t *testing.T) {
		runtime := openAITLSFingerprintRuntime{UpstreamUserAgent: "router-ua"}
		seedTLSFingerprintRuntimeHeaders(&runtime, &tlsfingerprint.Profile{
			UserAgent:  "template-ua",
			Originator: "template-origin",
		})
		require.Equal(t, "router-ua", runtime.UpstreamUserAgent)
		require.Equal(t, "template-origin", runtime.UpstreamOriginator)
	})

	t.Run("nil profile is a no-op", func(t *testing.T) {
		runtime := openAITLSFingerprintRuntime{}
		seedTLSFingerprintRuntimeHeaders(&runtime, nil)
		require.Empty(t, runtime.UpstreamUserAgent)
		require.Empty(t, runtime.UpstreamOriginator)
	})
}
