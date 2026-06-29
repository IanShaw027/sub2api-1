package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPlatformFingerprintManager_AnthropicUserAgentMatchesClaudeDefaultHeaders(t *testing.T) {
	mgr := NewPlatformFingerprintManager(nil)

	require.Equal(t, claude.DefaultHeaders["User-Agent"], mgr.Get(PlatformAnthropic).CanonicalUA)
}

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

func TestFingerprintNormalizer_DefaultTriggerPhrasePairsKeepPreviouslyFilteredDefaults(t *testing.T) {
	pairs := buildTriggerPhrasePairs(map[string]string{
		"cursor": "",
		"devin":  "",
		"zed":    "",
	})

	require.Contains(t, pairs, [2]string{"cursor", ""})
	require.Contains(t, pairs, [2]string{"devin", ""})
	require.Contains(t, pairs, [2]string{"zed", ""})
}

func TestFingerprintNormalizer_DefaultTriggerPhrasePairsKeepConcreteHarnessPhrases(t *testing.T) {
	pairs := buildTriggerPhrasePairs(map[string]string{
		"cursor":              "",
		"devin":               "",
		"zed":                 "",
		"third-party harness": "canonical client",
	})
	body := []byte(`{"messages":[{"role":"system","content":"Use cursor pagination with zed ordering and ask Devin for review. Remove third-party harness marker."}]}`)

	out := sanitizeTriggerPhrases(body, pairs)
	content := gjson.GetBytes(out, "messages.0.content").String()
	require.NotContains(t, content, "cursor")
	require.NotContains(t, content, "zed")
	require.NotContains(t, content, "Devin")
	require.Contains(t, content, "canonical client")
	require.NotContains(t, content, "third-party harness")
}

func TestFingerprintNormalizer_ApplyToRequestAndStripProxyHeadersUseSameStrippingRules(t *testing.T) {
	cfg := &CanonicalFingerprintConfig{
		Enabled:           true,
		AntiBanEnabled:    true,
		EnabledByPlatform: map[string]bool{"anthropic": true},
		StripProxyHeaders: []string{"x-litellm", "x-forwarded", "via"},
		PlatformProfiles: map[string]PlatformProfile{
			"anthropic": {StripExtra: []string{"x-stainless"}},
		},
	}
	n := NewFingerprintNormalizer(nil, nil, nil, cfg, nil)
	buildReq := func() *http.Request {
		req := httptest.NewRequest("POST", "/v1/messages", nil)
		req.Header.Set("X-Litellm-Trace", "drop")
		req.Header.Set("X-Forwarded-For", "drop")
		req.Header.Set("Via", "drop")
		req.Header.Set("X-Stainless-Arch", "drop")
		req.Header.Set("X-Anthropic-Billing-Header", "drop")
		req.Header.Set("X-Anthropic-Attribution", "drop")
		req.Header.Set("X-Keep", "keep")
		return req
	}

	stripReq := buildReq()
	n.StripProxyHeaders(stripReq, PlatformAnthropic)
	applyReq := buildReq()
	_, _, err := n.ApplyToRequest(applyReq, []byte(`{"messages":[{"role":"user","content":"hi"}]}`), &CanonicalFingerprint{Platform: PlatformAnthropic})
	require.NoError(t, err)

	require.Equal(t, stripReq.Header, applyReq.Header)
	require.Equal(t, "keep", stripReq.Header.Get("X-Keep"))
}
