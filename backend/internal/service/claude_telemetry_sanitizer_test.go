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

	got, ok := SanitizeClaudeTelemetryBatch(body, ClaudeTelemetrySanitizeOptions{
		DeviceID:          "canonical-device",
		Email:             "canonical@example.com",
		CanonicalEnv:      map[string]any{"platform": "darwin", "arch": "arm64", "is_ci": false},
		ConstrainedMemory: 8589934592,
		RSSRange:          [2]int64{300000000, 300000001},
		HeapTotalRange:    [2]int64{100000000, 100000001},
		HeapUsedRange:     [2]int64{50000000, 50000001},
	})

	require.True(t, ok)
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

// fail-closed：无法保证脱敏时必须返回 ok=false，调用方据此丢弃、绝不转发未脱敏原文。
func TestSanitizeClaudeTelemetryBatch_FailsClosedOnInvalidJSON(t *testing.T) {
	got, ok := SanitizeClaudeTelemetryBatch([]byte(`{"events":[ not-json `), ClaudeTelemetrySanitizeOptions{})
	require.False(t, ok)
	require.Nil(t, got)

	got, ok = SanitizeClaudeTelemetryBatch(nil, ClaudeTelemetrySanitizeOptions{})
	require.False(t, ok)
	require.Nil(t, got)
}

// process 中会泄露真实部署路径/启动命令或宿主运行时版本的字段必须被剥离
// （此前只改内存数值、遗漏了它们）。
func TestSanitizeClaudeTelemetryBatch_StripsLeakyProcessFields(t *testing.T) {
	body := []byte(`{"events":[{"event_data":{"process":{"cwd":"/opt/real/app","argv":["node","server.js"],"execPath":"/usr/bin/node","execArgv":["--x"],"title":"real-proc","ppid":42,"pid":7,"rss":2,"version":"v24.14.0"}}}]}`)

	got, ok := SanitizeClaudeTelemetryBatch(body, ClaudeTelemetrySanitizeOptions{})
	require.True(t, ok)
	require.True(t, json.Valid(got), string(got))

	proc := gjson.GetBytes(got, "events.0.event_data.process")
	require.False(t, proc.Get("cwd").Exists())
	require.False(t, proc.Get("argv").Exists())
	require.False(t, proc.Get("execPath").Exists())
	require.False(t, proc.Get("execArgv").Exists())
	require.False(t, proc.Get("title").Exists())
	require.False(t, proc.Get("ppid").Exists())
	require.False(t, proc.Get("version").Exists())
	// pid 等无泄露语义的字段保留
	require.Equal(t, int64(7), proc.Get("pid").Int())
}

// process.platform/arch 存在时同步为 profile 值，避免与 env.platform 自相矛盾；
// 原本不含这两个字段的 process 不被凭空添加。
func TestSanitizeClaudeTelemetryBatch_SyncsProcessPlatformArch(t *testing.T) {
	withFields := []byte(`{"events":[{"event_data":{"process":{"platform":"linux","arch":"x64","pid":9}}}]}`)
	got, ok := SanitizeClaudeTelemetryBatch(withFields, ClaudeTelemetrySanitizeOptions{Platform: "darwin", Arch: "arm64"})
	require.True(t, ok)
	proc := gjson.GetBytes(got, "events.0.event_data.process")
	require.Equal(t, "darwin", proc.Get("platform").String())
	require.Equal(t, "arm64", proc.Get("arch").String())

	withoutFields := []byte(`{"events":[{"event_data":{"process":{"pid":9}}}]}`)
	got, ok = SanitizeClaudeTelemetryBatch(withoutFields, ClaudeTelemetrySanitizeOptions{Platform: "darwin", Arch: "arm64"})
	require.True(t, ok)
	proc = gjson.GetBytes(got, "events.0.event_data.process")
	require.False(t, proc.Get("platform").Exists())
	require.False(t, proc.Get("arch").Exists())
}

// 内存三段必须满足 heapUsed <= heapTotal <= rss，即使配置区间重叠也不能产出物理不可能快照。
func TestSanitizeClaudeTelemetryBatch_MemoryPhysicalConstraints(t *testing.T) {
	body := []byte(`{"events":[{"event_data":{"process":{"rss":1,"heapTotal":2,"heapUsed":3,"pid":9}}}]}`)
	// 区间刻意重叠：heapTotal 上界(400M) > rss 固定值(300M)，heapUsed 上界(500M) > heapTotal。
	got, ok := SanitizeClaudeTelemetryBatch(body, ClaudeTelemetrySanitizeOptions{
		RSSRange:       [2]int64{300000000, 300000000},
		HeapTotalRange: [2]int64{400000000, 400000000},
		HeapUsedRange:  [2]int64{500000000, 500000000},
	})
	require.True(t, ok)
	proc := gjson.GetBytes(got, "events.0.event_data.process")
	rss := proc.Get("rss").Int()
	heapTotal := proc.Get("heapTotal").Int()
	heapUsed := proc.Get("heapUsed").Int()
	require.LessOrEqual(t, heapTotal, rss)
	require.LessOrEqual(t, heapUsed, heapTotal)
}

func TestSanitizeClaudeTelemetryBatch_RemovesTopLevelAndNestedLeakFields(t *testing.T) {
	body := []byte(`{
		"baseUrl":"https://gateway.example.com",
		"client_metadata":{"env":{"HOST":"real"}},
		"metadata":{"gateway":"proxy","keep":"ok"},
		"events":[{"event_data":{"device_id":"real-device","email":"real@example.com","metadata":{"baseUrl":"https://gateway.example.com","keep":"ok"},"client_metadata":{"process":{"rss":1}},"env":{"platform":"linux"}}}]
	}`)

	got, ok := SanitizeClaudeTelemetryBatch(body, ClaudeTelemetrySanitizeOptions{})

	require.True(t, ok)
	require.False(t, gjson.GetBytes(got, "baseUrl").Exists())
	require.False(t, gjson.GetBytes(got, "client_metadata").Exists())
	require.False(t, gjson.GetBytes(got, "metadata.gateway").Exists())
	require.Equal(t, "ok", gjson.GetBytes(got, "metadata.keep").String())
	require.False(t, gjson.GetBytes(got, "events.0.event_data.metadata.baseUrl").Exists())
	require.Equal(t, "ok", gjson.GetBytes(got, "events.0.event_data.metadata.keep").String())
	require.False(t, gjson.GetBytes(got, "events.0.event_data.client_metadata").Exists())
	require.False(t, gjson.GetBytes(got, "events.0.event_data.device_id").Exists())
	require.False(t, gjson.GetBytes(got, "events.0.event_data.email").Exists())
	require.False(t, gjson.GetBytes(got, "events.0.event_data.env").Exists())
}

func TestRewriteSystemReminderEnvBlocks_RewritesOnlyReminderEnvironment(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"before Platform: linux <system-reminder>Platform: linux\nShell: bash\nOS Version: Linux 6.8\nWorking directory: /home/alice/project\n</system-reminder> after Platform: linux"}]}]}`)
	profile := &AccountEnvProfile{Platform: "darwin", Shell: "zsh", OSVersion: "Darwin 24.3.0", WorkDir: "/Users/alex/projects/webapp"}

	got := RewriteSystemReminderEnvBlocks(body, profile)
	text := gjson.GetBytes(got, "messages.0.content.0.text").String()

	require.Contains(t, text, "before Platform: linux")
	require.Contains(t, text, "after Platform: linux")
	// 完整假路径，不再保留真实 basename project。
	require.Contains(t, text, "<system-reminder>Platform: darwin\nShell: zsh\nOS Version: Darwin 24.3.0\nWorking directory: /Users/alex/projects/webapp")
	require.NotContains(t, text, "Shell: bash")
	require.NotContains(t, text, "/Users/alex/projects/project")
}
