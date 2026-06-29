package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestFingerprintNormalizer_ApplyToRequest_OpenAIAndGrokKeepUserAndSkipClientMetadata(t *testing.T) {
	n := NewFingerprintNormalizer(nil, nil, nil, &CanonicalFingerprintConfig{
		Enabled:             true,
		AntiBanEnabled:      true,
		EnabledByPlatform:   map[string]bool{"openai": true, "grok": true, "anthropic": true},
		SpoofProcessMetrics: true,
	}, NewPlatformFingerprintManager(nil))

	cases := []struct {
		name     string
		platform string
		body     string
	}{
		{
			name:     "openai",
			platform: PlatformOpenAI,
			body:     `{"user":"user-123","messages":[{"role":"user","content":"hi"}]}`,
		},
		{
			name:     "grok",
			platform: PlatformGrok,
			body:     `{"user":"user-456","messages":[{"role":"user","content":"hi"}]}`,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, got, err := n.ApplyToRequest(nil, []byte(tt.body), &CanonicalFingerprint{
				Platform: tt.platform,
			})
			require.NoError(t, err)
			require.Equal(t, tt.body, string(got))
		})
	}
}

func TestFingerprintNormalizer_ApplyToRequest_AnthropicDeletesClientMetadata(t *testing.T) {
	n := NewFingerprintNormalizer(nil, nil, nil, &CanonicalFingerprintConfig{
		Enabled:             true,
		AntiBanEnabled:      true,
		EnabledByPlatform:   map[string]bool{"anthropic": true},
		SpoofProcessMetrics: true,
	}, nil)

	_, got, err := n.ApplyToRequest(nil, []byte(`{"messages":[{"role":"user","content":"hi"}],"client_metadata":{"process":"bad","env":{"HOSTNAME":"bad"}}}`), &CanonicalFingerprint{
		Platform: PlatformAnthropic,
	})
	require.NoError(t, err)
	require.NotContains(t, string(got), `"client_metadata"`)
	require.NotContains(t, string(got), `"process"`)
	require.NotContains(t, string(got), `"env"`)
}

func TestFingerprintNormalizer_ApplyToRequest_OpenAIAndGrokDoNotRewritePromptText(t *testing.T) {
	n := NewFingerprintNormalizer(nil, nil, nil, &CanonicalFingerprintConfig{
		Enabled:            true,
		AntiBanEnabled:     true,
		EnabledByPlatform:  map[string]bool{"openai": true, "grok": true},
		TriggerScanEnabled: true,
		TriggerPhrases: buildTriggerPhrasePairs(map[string]string{
			"cursor":              "",
			"third-party harness": "canonical client",
		}),
	}, NewPlatformFingerprintManager(nil))

	body := `{"messages":[{"role":"system","content":"Keep cch=abc123 for diagnostics and mention cursor plus third-party harness literally."},{"role":"user","content":"hi"}]}`
	for _, platform := range []string{PlatformOpenAI, PlatformGrok} {
		t.Run(platform, func(t *testing.T) {
			_, got, err := n.ApplyToRequest(nil, []byte(body), &CanonicalFingerprint{Platform: platform})
			require.NoError(t, err)
			require.Equal(t, body, string(got))
		})
	}
}

func TestFingerprintNormalizer_ApplyToRequest_AnthropicStillRewritesAttributionPromptText(t *testing.T) {
	n := NewFingerprintNormalizer(nil, nil, nil, &CanonicalFingerprintConfig{
		Enabled:            true,
		AntiBanEnabled:     true,
		EnabledByPlatform:  map[string]bool{"anthropic": true},
		TriggerScanEnabled: true,
		TriggerPhrases: buildTriggerPhrasePairs(map[string]string{
			"third-party harness": "canonical client",
		}),
	}, nil)

	body := `{"messages":[{"role":"system","content":"Keep cch=abc123 only."},{"role":"developer","content":"Mention third-party harness marker."},{"role":"user","content":"hi"}]}`
	_, got, err := n.ApplyToRequest(nil, []byte(body), &CanonicalFingerprint{Platform: PlatformAnthropic})
	require.NoError(t, err)
	systemText := gjson.GetBytes(got, "messages.0.content").String()
	developerText := gjson.GetBytes(got, "messages.1.content").String()
	require.NotContains(t, systemText, "cch=abc123")
	require.Contains(t, developerText, "canonical client")
}

func TestFingerprintNormalizer_DefaultTriggerPhrasesDoNotRemoveCommonProductWords(t *testing.T) {
	cfg := &CanonicalFingerprintConfig{
		Enabled:            true,
		AntiBanEnabled:     true,
		TriggerScanEnabled: true,
		TriggerPhrases: buildTriggerPhrasePairs(map[string]string{
			"cursor": "",
			"devin":  "",
			"zed":    "",
		}),
	}
	body := []byte(`{"messages":[{"role":"system","content":"Use cursor pagination with zed ordering and ask Devin for review."}]}`)

	out := sanitizeTriggerPhrases(body, cfg.TriggerPhrases)

	require.Equal(t, "Use cursor pagination with zed ordering and ask Devin for review.", gjson.GetBytes(out, "messages.0.content").String())
}
