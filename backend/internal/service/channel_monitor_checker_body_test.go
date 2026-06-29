//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// swapMonitorHTTPClient 临时替换 monitorHTTPClient 为不带 SSRF 校验的普通 client，
// 让 httptest (127.0.0.1) 能连通。测试结束后恢复。
func swapMonitorHTTPClient(t *testing.T) {
	t.Helper()
	orig := monitorHTTPClient
	monitorHTTPClient = &http.Client{Timeout: 5 * time.Second, CheckRedirect: blockMonitorRedirect}
	t.Cleanup(func() { monitorHTTPClient = orig })
}

// captureHandler 把每次收到的请求 body 和 headers 存起来，测试断言用。
type captureHandler struct {
	lastBody       map[string]any
	lastHeaders    http.Header
	respondText    string // 写到 Anthropic content[0].text 里（校验用）
	respondContent []map[string]any
	status         int
}

func (h *captureHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.lastHeaders = r.Header.Clone()
	defer func() { _ = r.Body.Close() }()
	var parsed map[string]any
	_ = json.NewDecoder(r.Body).Decode(&parsed)
	h.lastBody = parsed

	if h.status == 0 {
		h.status = 200
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(h.status)
	content := h.respondContent
	if len(content) == 0 {
		// 构造 Anthropic 格式的响应：content[0].text = h.respondText
		content = []map[string]any{
			{"type": "text", "text": h.respondText},
		}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"content": content,
	})
}

func setupFakeAnthropic(t *testing.T, handler *captureHandler) string {
	t.Helper()
	swapMonitorHTTPClient(t)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv.URL
}

type openAICaptureHandler struct {
	lastBody                  map[string]any
	lastHeaders               http.Header
	lastPath                  string
	status                    int
	responsesLeadingReasoning bool
	responsesEnvelope         bool
}

func (h *openAICaptureHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.lastHeaders = r.Header.Clone()
	h.lastPath = r.URL.Path
	defer func() { _ = r.Body.Close() }()
	var parsed map[string]any
	_ = json.NewDecoder(r.Body).Decode(&parsed)
	h.lastBody = parsed

	if h.status == 0 {
		h.status = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(h.status)

	answer := answerFromOpenAIRequest(parsed)
	if h.lastPath == providerOpenAIResponsesPath {
		output := []map[string]any{}
		if h.responsesLeadingReasoning {
			output = append(output, map[string]any{
				"type":    "reasoning",
				"summary": []any{},
			})
		}
		output = append(output, map[string]any{
			"type":   "message",
			"status": "completed",
			"role":   "assistant",
			"content": []map[string]any{
				{"type": "output_text", "text": answer},
			},
		})
		if h.responsesEnvelope {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"type":     "response.completed",
				"response": map[string]any{"output": output},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": output,
		})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"choices": []map[string]any{{"message": map[string]any{"content": answer}}},
	})
}

func setupFakeOpenAI(t *testing.T, handler *openAICaptureHandler) string {
	t.Helper()
	swapMonitorHTTPClient(t)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv.URL
}

func answerFromOpenAIRequest(body map[string]any) string {
	prompt, _ := body["input"].(string)
	if prompt == "" {
		if messages, ok := body["messages"].([]any); ok && len(messages) > 0 {
			if msg, ok := messages[0].(map[string]any); ok {
				prompt, _ = msg["content"].(string)
			}
		}
	}
	return answerFromChallengePrompt(prompt)
}

var challengeQuestionRegex = regexp.MustCompile(`Q: (\d+) ([+-]) (\d+) = \?\nA:$`)

func answerFromChallengePrompt(prompt string) string {
	m := challengeQuestionRegex.FindStringSubmatch(prompt)
	if len(m) != 4 {
		return "0"
	}
	left, _ := strconv.Atoi(m[1])
	right, _ := strconv.Atoi(m[3])
	if m[2] == "+" {
		return strconv.Itoa(left + right)
	}
	return strconv.Itoa(left - right)
}

func TestRunCheckForModel_OffMode_PreservesDefaultBody(t *testing.T) {
	h := &captureHandler{respondText: "the answer is 42"}
	endpoint := setupFakeAnthropic(t, h)

	// 跑一次 off 模式（opts=nil），确认默认 body 行为未变
	_ = runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-x", nil)

	if h.lastBody["model"] != "claude-x" {
		t.Errorf("default body should contain model=claude-x, got %v", h.lastBody["model"])
	}
	if _, ok := h.lastBody["messages"]; !ok {
		t.Error("default body should contain messages")
	}
	if h.lastHeaders.Get("x-api-key") != "sk-fake" {
		t.Errorf("expected adapter's x-api-key header, got %q", h.lastHeaders.Get("x-api-key"))
	}
}

func TestRunCheckForModel_AnthropicDefaultRequestAddsChannelMonitorProbeHeader(t *testing.T) {
	h := &captureHandler{respondText: "the answer is 42"}
	endpoint := setupFakeAnthropic(t, h)

	_ = runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-haiku-4-5", nil)

	if h.lastHeaders.Get("X-Sub2API-Channel-Monitor") != "1" {
		t.Fatalf("default Anthropic monitor request should mark itself as a channel monitor probe, headers=%v", h.lastHeaders)
	}
}

func TestRunCheckForModel_KiroDefaultRequestAddsChannelMonitorProbeHeader(t *testing.T) {
	h := &captureHandler{respondText: "the answer is 42"}
	endpoint := setupFakeAnthropic(t, h)

	_ = runCheckForModel(context.Background(), MonitorProviderKiro, endpoint, "sk-fake", "claude-haiku-4-5", nil)

	if h.lastHeaders.Get("X-Sub2API-Channel-Monitor") != "1" {
		t.Fatalf("default Kiro monitor request should mark itself as a channel monitor probe, headers=%v", h.lastHeaders)
	}
}

func TestRunCheckForModel_OpenAI_DefaultChatRequest(t *testing.T) {
	h := &openAICaptureHandler{}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-test", nil)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("default chat request should pass challenge, got status=%s message=%q", res.Status, res.Message)
	}
	if h.lastPath != providerOpenAIPath {
		t.Fatalf("expected chat completions path %q, got %q", providerOpenAIPath, h.lastPath)
	}
	if h.lastBody["model"] != "gpt-test" {
		t.Errorf("chat body should contain model=gpt-test, got %v", h.lastBody["model"])
	}
	if _, ok := h.lastBody["messages"]; !ok {
		t.Error("chat body should contain messages")
	}
	if _, ok := h.lastBody["instructions"]; ok {
		t.Error("chat body must not contain top-level instructions")
	}
	if h.lastBody["stream"] != false {
		t.Errorf("chat body should set stream=false, got %v", h.lastBody["stream"])
	}
	if h.lastHeaders.Get("Authorization") != "Bearer sk-openai" {
		t.Errorf("expected bearer auth header, got %q", h.lastHeaders.Get("Authorization"))
	}
}

func TestRunCheckForModel_GrokDefaultChatRequest(t *testing.T) {
	h := &openAICaptureHandler{}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderGrok, endpoint, "xai-grok-key", "grok-4.3", nil)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("grok default chat request should pass challenge, got status=%s message=%q", res.Status, res.Message)
	}
	if h.lastPath != providerOpenAIPath {
		t.Fatalf("expected chat completions path %q, got %q", providerOpenAIPath, h.lastPath)
	}
	if h.lastBody["model"] != "grok-4.3" {
		t.Errorf("chat body should contain model=grok-4.3, got %v", h.lastBody["model"])
	}
	if _, ok := h.lastBody["messages"]; !ok {
		t.Error("grok chat body should contain messages")
	}
	if _, ok := h.lastBody["instructions"]; ok {
		t.Error("grok chat body must not contain top-level instructions")
	}
	if h.lastBody["stream"] != false {
		t.Errorf("grok chat body should set stream=false, got %v", h.lastBody["stream"])
	}
	if h.lastHeaders.Get("Authorization") != "Bearer xai-grok-key" {
		t.Errorf("expected bearer auth header, got %q", h.lastHeaders.Get("Authorization"))
	}
	if h.lastHeaders.Get(ChannelMonitorProbeHeaderName) != "" {
		t.Errorf("grok monitor requests should not add internal probe header, got %q", h.lastHeaders.Get(ChannelMonitorProbeHeaderName))
	}
}

func TestRunCheckForModel_GrokResponsesModeIsRejected(t *testing.T) {
	h := &openAICaptureHandler{}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderGrok, endpoint, "xai-grok-key", "grok-4.3", &CheckOptions{
		APIMode: MonitorAPIModeResponses,
	})

	require.Equal(t, MonitorStatusError, res.Status)
	require.Contains(t, res.Message, "unsupported grok api_mode")
}

func TestRunCheckForModel_OpenAIResponses_DefaultRequest(t *testing.T) {
	h := &openAICaptureHandler{}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-test", &CheckOptions{
		APIMode: MonitorAPIModeResponses,
	})

	if res.Status != MonitorStatusOperational {
		t.Fatalf("default responses request should pass challenge, got status=%s message=%q", res.Status, res.Message)
	}
	if h.lastPath != providerOpenAIResponsesPath {
		t.Fatalf("expected responses path %q, got %q", providerOpenAIResponsesPath, h.lastPath)
	}
	if h.lastBody["model"] != "gpt-test" {
		t.Errorf("responses body should contain model=gpt-test, got %v", h.lastBody["model"])
	}
	instructions, _ := h.lastBody["instructions"].(string)
	if strings.TrimSpace(instructions) == "" {
		t.Error("responses body should contain non-empty instructions")
	}
	input, _ := h.lastBody["input"].(string)
	if strings.TrimSpace(input) == "" {
		t.Error("responses body should contain non-empty input")
	}
	if _, ok := h.lastBody["messages"]; ok {
		t.Error("responses body must not contain chat messages")
	}
	if h.lastBody["stream"] != false {
		t.Errorf("responses body should set stream=false, got %v", h.lastBody["stream"])
	}
	if h.lastHeaders.Get("Authorization") != "Bearer sk-openai" {
		t.Errorf("expected bearer auth header, got %q", h.lastHeaders.Get("Authorization"))
	}
}

func TestRunCheckForModel_OpenAIResponses_SkipsLeadingReasoningItem(t *testing.T) {
	h := &openAICaptureHandler{responsesLeadingReasoning: true}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-5.5", &CheckOptions{
		APIMode: MonitorAPIModeResponses,
	})

	if res.Status != MonitorStatusOperational {
		t.Fatalf("responses request should find text after leading reasoning item, got status=%s message=%q", res.Status, res.Message)
	}
	if h.lastPath != providerOpenAIResponsesPath {
		t.Fatalf("expected responses path %q, got %q", providerOpenAIResponsesPath, h.lastPath)
	}
}

func TestRunCheckForModel_OpenAIResponses_ExtractsResponseCompletedEnvelope(t *testing.T) {
	h := &openAICaptureHandler{responsesEnvelope: true}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-5.3-codex-spark", &CheckOptions{
		APIMode: MonitorAPIModeResponses,
	})

	if res.Status != MonitorStatusOperational {
		t.Fatalf("responses event envelope should pass challenge, got status=%s message=%q", res.Status, res.Message)
	}
	if h.lastPath != providerOpenAIResponsesPath {
		t.Fatalf("expected responses path %q, got %q", providerOpenAIResponsesPath, h.lastPath)
	}
}

func TestRunCheckForModel_OpenAIResponses_MergeMode_ProtectsChallengeFields(t *testing.T) {
	h := &openAICaptureHandler{}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-5.5", &CheckOptions{
		APIMode:          MonitorAPIModeResponses,
		BodyOverrideMode: MonitorBodyOverrideModeMerge,
		BodyOverride: map[string]any{
			"metadata":          map[string]any{"source": "monitor"},
			"max_output_tokens": float64(64),
			"model":             "user-forced-model",
			"instructions":      "user-forced-instructions",
			"input":             "user-forced-input",
			"stream":            true,
		},
	})

	if res.Status != MonitorStatusOperational {
		t.Fatalf("responses merge request should pass challenge, got status=%s message=%q", res.Status, res.Message)
	}
	if h.lastPath != providerOpenAIResponsesPath {
		t.Fatalf("expected responses path %q, got %q", providerOpenAIResponsesPath, h.lastPath)
	}
	if h.lastBody["model"] != "gpt-5.5" {
		t.Errorf("responses merge should keep adapter model, got %v", h.lastBody["model"])
	}
	if h.lastBody["instructions"] == "user-forced-instructions" {
		t.Error("responses merge should protect adapter instructions")
	}
	if h.lastBody["input"] == "user-forced-input" {
		t.Error("responses merge should protect adapter input")
	}
	if h.lastBody["stream"] != false {
		t.Errorf("responses merge should keep stream=false, got %v", h.lastBody["stream"])
	}
	if h.lastBody["metadata"] == nil {
		t.Error("responses merge should allow non-protected keys like metadata")
	}
	if got := h.lastBody["max_output_tokens"]; got != float64(64) {
		t.Errorf("responses merge should allow max_output_tokens override, got %v", got)
	}
}

func TestRunCheckForModel_OpenAIResponsesReplaceMissingInstructionsFailsLocally(t *testing.T) {
	h := &openAICaptureHandler{}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-test", &CheckOptions{
		APIMode:          MonitorAPIModeResponses,
		BodyOverrideMode: MonitorBodyOverrideModeReplace,
		BodyOverride: map[string]any{
			"model": "gpt-test",
			"input": "hello",
		},
	})

	if res.Status != MonitorStatusError {
		t.Fatalf("invalid responses replace body should fail locally as error, got status=%s", res.Status)
	}
	if !strings.Contains(res.Message, "non-empty instructions and input for responses") {
		t.Errorf("expected local validation message about instructions/input, got %q", res.Message)
	}
	if h.lastPath != "" {
		t.Errorf("invalid replace body should fail before HTTP request, got path %q", h.lastPath)
	}
}

func TestRunCheckForModel_OpenAIResponsesReplaceMissingModelFailsLocally(t *testing.T) {
	h := &openAICaptureHandler{}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-test", &CheckOptions{
		APIMode:          MonitorAPIModeResponses,
		BodyOverrideMode: MonitorBodyOverrideModeReplace,
		BodyOverride: map[string]any{
			"instructions": "Return a number.",
			"input":        "What is 1 + 1?",
		},
	})

	if res.Status != MonitorStatusError {
		t.Fatalf("missing model should fail locally as error, got status=%s", res.Status)
	}
	if !strings.Contains(res.Message, "non-empty model") {
		t.Errorf("expected local validation message about model, got %q", res.Message)
	}
	if h.lastPath != "" {
		t.Errorf("invalid replace body should fail before HTTP request, got path %q", h.lastPath)
	}
}

func TestRunCheckForModel_OpenAIResponses_ReplaceMode_UsesResponsesBodyAndExtraction(t *testing.T) {
	h := &openAICaptureHandler{responsesLeadingReasoning: true}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-5.5", &CheckOptions{
		APIMode:          MonitorAPIModeResponses,
		BodyOverrideMode: MonitorBodyOverrideModeReplace,
		BodyOverride: map[string]any{
			"model":        "user-forced-model",
			"instructions": "Return the number 42.",
			"input":        "What is 40 + 2?",
			"stream":       false,
		},
	})

	if res.Status != MonitorStatusOperational {
		t.Fatalf("responses replace request should treat non-empty output text as operational, got status=%s message=%q", res.Status, res.Message)
	}
	if h.lastPath != providerOpenAIResponsesPath {
		t.Fatalf("expected responses path %q, got %q", providerOpenAIResponsesPath, h.lastPath)
	}
	if h.lastBody["model"] != "user-forced-model" {
		t.Errorf("responses replace should use user model, got %v", h.lastBody["model"])
	}
	if h.lastBody["instructions"] != "Return the number 42." {
		t.Errorf("responses replace should use user instructions, got %v", h.lastBody["instructions"])
	}
	if h.lastBody["input"] != "What is 40 + 2?" {
		t.Errorf("responses replace should use user input, got %v", h.lastBody["input"])
	}
}

func TestRunCheckForModel_MergeMode_UserFieldsWinButDenyListProtects(t *testing.T) {
	h := &captureHandler{respondText: "the answer is 42"}
	endpoint := setupFakeAnthropic(t, h)

	opts := &CheckOptions{
		BodyOverrideMode: MonitorBodyOverrideModeMerge,
		BodyOverride: map[string]any{
			"system":     "You are Claude Code...",
			"max_tokens": float64(999),   // 应该覆盖默认 50
			"model":      "hacked-model", // 应该被黑名单挡住，保留原 model
			"messages":   []any{},        // 同上，被挡
		},
		ExtraHeaders: map[string]string{
			"User-Agent":        "claude-cli/1.0",
			"Content-Length":    "999", // 黑名单
			"x-api-key":         "attacker",
			"Authorization":     "Bearer attacker",
			"anthropic-version": "bad-version",
			"x-custom":          "ok",
		},
	}
	_ = runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-x", opts)

	if h.lastBody["system"] != "You are Claude Code..." {
		t.Errorf("merge mode should inject system, got %v", h.lastBody["system"])
	}
	// max_tokens 覆盖生效
	if mt, ok := h.lastBody["max_tokens"].(float64); !ok || mt != 999 {
		t.Errorf("merge mode should override max_tokens to 999, got %v", h.lastBody["max_tokens"])
	}
	// model 在黑名单 — 应该保留默认值
	if h.lastBody["model"] != "claude-x" {
		t.Errorf("model should be protected by deny list, got %v", h.lastBody["model"])
	}
	// messages 在黑名单 — 应该保留默认值（非空）
	msgs, _ := h.lastBody["messages"].([]any)
	if len(msgs) == 0 {
		t.Error("messages should be protected by deny list (kept default, non-empty)")
	}
	// header 合并
	if h.lastHeaders.Get("User-Agent") != "claude-cli/1.0" {
		t.Errorf("extra User-Agent should override, got %q", h.lastHeaders.Get("User-Agent"))
	}
	if h.lastHeaders.Get("x-custom") != "ok" {
		t.Errorf("extra custom header should be present, got %q", h.lastHeaders.Get("x-custom"))
	}
	if h.lastHeaders.Get("x-api-key") != "sk-fake" {
		t.Errorf("x-api-key should keep adapter credential, got %q", h.lastHeaders.Get("x-api-key"))
	}
	if h.lastHeaders.Get("Authorization") != "" {
		t.Errorf("authorization should be blocked for Anthropic checks, got %q", h.lastHeaders.Get("Authorization"))
	}
	if h.lastHeaders.Get("anthropic-version") != monitorAnthropicAPIVersion {
		t.Errorf("anthropic-version should keep adapter protocol value, got %q", h.lastHeaders.Get("anthropic-version"))
	}
	// Content-Length 黑名单：会被 net/http 自动重算，但不应由用户的 "999" 决定。
	// 我们无法直接断言丢弃（http.Client 总会填上），只断言请求成功即可。
}

func TestRunCheckForModel_ReplaceMode_FullBodyUsedAndChallengeSkipped(t *testing.T) {
	// replace 模式下我们的 body 完全自定义，challenge 数学题不会出现在请求里，
	// 上游也不会回正确答案 — 但只要 2xx + 响应文本非空，就算 operational
	h := &captureHandler{respondText: "any non-empty text"}
	endpoint := setupFakeAnthropic(t, h)

	userBody := map[string]any{
		"model":      "user-forced-model",
		"messages":   []any{map[string]any{"role": "user", "content": "hi"}},
		"max_tokens": float64(10),
		"system":     "You are someone else",
	}
	opts := &CheckOptions{
		BodyOverrideMode: MonitorBodyOverrideModeReplace,
		BodyOverride:     userBody,
	}
	res := runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-x", opts)

	// 请求 body = 用户提供的原样
	if h.lastBody["model"] != "user-forced-model" {
		t.Errorf("replace mode should use user's model, got %v", h.lastBody["model"])
	}
	if h.lastBody["system"] != "You are someone else" {
		t.Errorf("replace mode should use user's system, got %v", h.lastBody["system"])
	}
	// challenge 虽然没命中，但由于 replace 模式跳过 challenge 校验 + 响应非空 → operational
	if res.Status != MonitorStatusOperational {
		t.Errorf("replace mode with 2xx + non-empty text should be operational, got status=%s message=%q",
			res.Status, res.Message)
	}
}

func TestRunCheckForModel_ReplaceMode_EmptyResponseIsFailed(t *testing.T) {
	h := &captureHandler{respondText: ""} // 上游 200 但 content[0].text 为空
	endpoint := setupFakeAnthropic(t, h)

	opts := &CheckOptions{
		BodyOverrideMode: MonitorBodyOverrideModeReplace,
		BodyOverride:     map[string]any{"model": "x", "messages": []any{}},
	}
	res := runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-x", opts)

	if res.Status != MonitorStatusFailed {
		t.Errorf("replace mode with empty text should be failed, got status=%s", res.Status)
	}
	if !strings.Contains(res.Message, "replace-mode") {
		t.Errorf("failure message should hint replace-mode, got %q", res.Message)
	}
}

func TestRunCheckForModel_ReplaceMode_AnthropicThinkingFallbackUsesTextBlock(t *testing.T) {
	h := &captureHandler{
		respondContent: []map[string]any{
			{"type": "thinking", "thinking": "internal reasoning"},
			{"type": "text", "text": "Hey there. What can I help you with?"},
		},
	}
	endpoint := setupFakeAnthropic(t, h)

	opts := &CheckOptions{
		BodyOverrideMode: MonitorBodyOverrideModeReplace,
		BodyOverride:     map[string]any{"model": "x", "messages": []any{}},
	}
	res := runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-x", opts)

	if res.Status != MonitorStatusOperational {
		t.Errorf("replace mode should use first text block after thinking, got status=%s message=%q", res.Status, res.Message)
	}
}

func TestPostRawJSON_DoesNotFollowRedirects(t *testing.T) {
	swapMonitorHTTPClient(t)

	targetHit := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHit = true
		if r.Header.Get("x-api-key") != "" {
			t.Errorf("credential header leaked to redirect target: %q", r.Header.Get("x-api-key"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(target.Close)

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	t.Cleanup(redirector.Close)

	_, status, err := postRawJSON(context.Background(), redirector.URL, []byte(`{}`), map[string]string{"x-api-key": "sk-fake"})
	if err != nil {
		t.Fatalf("postRawJSON should return the redirect response without error, got %v", err)
	}
	if status != http.StatusFound {
		t.Fatalf("expected redirect status %d, got %d", http.StatusFound, status)
	}
	if targetHit {
		t.Fatal("monitor HTTP client followed redirect to target")
	}
}
