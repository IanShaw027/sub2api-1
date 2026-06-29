package service

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSanitizeClaudeLeakFields_RemovesStructuredProxyAndClientMetadataFields(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4-5",
		"baseUrl":"https://gateway.example.com",
		"base_url":"https://gateway.example.com/v1",
		"gateway":"custom-proxy",
		"metadata":{"baseUrl":"https://gateway.example.com","base_url":"https://gateway.example.com/v1","gateway":"proxy","keep":"ok"},
		"client_metadata":{"process":{"rss":123},"env":{"HOSTNAME":"real"},"baseUrl":"https://gateway.example.com","gateway":"proxy"},
		"messages":[{"role":"user","content":"do not remove word gateway from user text"}]
	}`)

	got := SanitizeClaudeOAuthBody(body, nil)
	gotStr := string(got)

	require.True(t, json.Valid(got), gotStr)
	require.False(t, gjson.GetBytes(got, "baseUrl").Exists())
	require.False(t, gjson.GetBytes(got, "base_url").Exists())
	require.False(t, gjson.GetBytes(got, "gateway").Exists())
	require.False(t, gjson.GetBytes(got, "metadata.baseUrl").Exists())
	require.False(t, gjson.GetBytes(got, "metadata.base_url").Exists())
	require.False(t, gjson.GetBytes(got, "metadata.gateway").Exists())
	require.False(t, gjson.GetBytes(got, "client_metadata").Exists())
	require.Equal(t, "ok", gjson.GetBytes(got, "metadata.keep").String())
	require.Contains(t, gotStr, "do not remove word gateway from user text")
}

func TestSanitizeClaudeTelemetryBatch_RewritesIdentityProcessAndAdditionalMetadata(t *testing.T) {
	additional := base64.StdEncoding.EncodeToString([]byte(`{"baseUrl":"https://gateway.example.com","base_url":"https://gateway.example.com/v1","gateway":"proxy","keep":"ok"}`))
	body := []byte(`{"events":[{"event_data":{"device_id":"real-device","email":"real@example.com","env":{"platform":"linux","arch":"x64"},"process":{"constrainedMemory":1,"rss":2,"heapTotal":3,"heapUsed":4,"pid":123},"baseUrl":"https://gateway.example.com","base_url":"https://gateway.example.com/v1","gateway":"proxy","additional_metadata":"` + additional + `"}}]}`)

	got := SanitizeClaudeTelemetryBatch(body, ClaudeTelemetrySanitizeOptions{
		DeviceID:          "canonical-device",
		Email:             "canonical@example.com",
		CanonicalEnv:      map[string]any{"platform": "darwin", "arch": "arm64", "is_ci": false},
		ConstrainedMemory: 8589934592,
		RSSRange:          [2]int64{300000000, 300000001},
		HeapTotalRange:    [2]int64{100000000, 100000001},
		HeapUsedRange:     [2]int64{50000000, 50000001},
	})

	require.True(t, json.Valid(got), string(got))
	event := gjson.GetBytes(got, "events.0.event_data")
	require.Equal(t, "canonical-device", event.Get("device_id").String())
	require.Equal(t, "canonical@example.com", event.Get("email").String())
	require.Equal(t, "darwin", event.Get("env.platform").String())
	require.Equal(t, "arm64", event.Get("env.arch").String())
	require.False(t, event.Get("baseUrl").Exists())
	require.False(t, event.Get("base_url").Exists())
	require.False(t, event.Get("gateway").Exists())
	require.Equal(t, int64(8589934592), event.Get("process.constrainedMemory").Int())
	require.Equal(t, int64(300000000), event.Get("process.rss").Int())
	require.Equal(t, int64(100000000), event.Get("process.heapTotal").Int())
	require.Equal(t, int64(50000000), event.Get("process.heapUsed").Int())
	require.Equal(t, int64(123), event.Get("process.pid").Int())

	decoded, err := base64.StdEncoding.DecodeString(event.Get("additional_metadata").String())
	require.NoError(t, err)
	require.True(t, json.Valid(decoded), string(decoded))
	require.False(t, gjson.GetBytes(decoded, "baseUrl").Exists())
	require.False(t, gjson.GetBytes(decoded, "base_url").Exists())
	require.False(t, gjson.GetBytes(decoded, "gateway").Exists())
	require.Equal(t, "ok", gjson.GetBytes(decoded, "keep").String())
}

func TestRewriteSystemReminderEnvBlocks_RewritesOnlyReminderEnvironment(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"before Platform: linux <system-reminder>Platform: linux\nShell: bash\nOS Version: Linux 6.8\nWorking directory: /home/alice/project\n</system-reminder> after Platform: linux"}]}]}`)
	profile := &AccountEnvProfile{Platform: "darwin", Shell: "zsh", OSVersion: "Darwin 24.3.0", WorkDir: "/Users/alex/projects/webapp"}

	got := RewriteSystemReminderEnvBlocks(body, profile)
	text := gjson.GetBytes(got, "messages.0.content.0.text").String()

	require.Contains(t, text, "before Platform: linux")
	require.Contains(t, text, "after Platform: linux")
	require.Contains(t, text, "<system-reminder>Platform: darwin\nShell: zsh\nOS Version: Darwin 24.3.0\nWorking directory: /Users/alex/projects/webapp")
	require.NotContains(t, text, "Shell: bash")
}
