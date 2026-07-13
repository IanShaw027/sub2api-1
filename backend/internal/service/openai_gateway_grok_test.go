//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func TestPatchGrokResponsesBodySetsMappedModelAndDropsUnsupportedFields(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model": "grok",
		"input": "hello",
		"prompt_cache_retention": "24h",
		"safety_identifier": "user-1",
		"reasoning": {"effort": "high"}
	}`)

	patched, err := patchGrokResponsesBody(body, "grok-4.3")
	require.NoError(t, err)
	require.True(t, json.Valid(patched))
	require.Equal(t, "grok-4.3", gjson.GetBytes(patched, "model").String())
	require.False(t, gjson.GetBytes(patched, "prompt_cache_retention").Exists())
	require.False(t, gjson.GetBytes(patched, "safety_identifier").Exists())
	require.Equal(t, "high", gjson.GetBytes(patched, "reasoning.effort").String())
}

func TestPatchGrokResponsesBodyMapsLegacyMaxTokens(t *testing.T) {
	t.Parallel()

	patched, err := patchGrokResponsesBody(
		[]byte(`{"model":"grok","input":"hello","max_tokens":321}`),
		"grok-4.3",
	)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(patched, "max_tokens").Exists(), string(patched))
	require.Equal(t, int64(321), gjson.GetBytes(patched, "max_output_tokens").Int())
}

func TestPatchGrokResponsesBodyPrefersMaxOutputTokens(t *testing.T) {
	t.Parallel()

	patched, err := patchGrokResponsesBody(
		[]byte(`{"model":"grok","input":"hello","max_tokens":321,"max_output_tokens":123}`),
		"grok-4.3",
	)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(patched, "max_tokens").Exists(), string(patched))
	require.Equal(t, int64(123), gjson.GetBytes(patched, "max_output_tokens").Int())
}

func TestPatchGrokResponsesBodyDropsEmptyContentBlocks(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model":"grok",
		"input":[
			{"role":"system","content":""},
			{"role":"user","content":[{"type":"input_text","text":""},{"type":"input_text","text":"hello"},{"type":"input_image","image_url":""}]},
			{"type":"function_call_output","call_id":"call_1","output":""}
		]
	}`)
	patched, err := patchGrokResponsesBody(body, "grok-4.3")
	require.NoError(t, err)
	require.Len(t, gjson.GetBytes(patched, "input").Array(), 2, string(patched))
	require.Equal(t, "hello", gjson.GetBytes(patched, "input.0.content.0.text").String())
	require.Equal(t, "(empty)", gjson.GetBytes(patched, "input.1.output").String())
}

func TestPatchGrokResponsesBodyRejectsAllEmptyInput(t *testing.T) {
	t.Parallel()

	_, err := patchGrokResponsesBody(
		[]byte(`{"model":"grok","input":[{"role":"user","content":[{"type":"input_text","text":""}]}]}`),
		"grok-4.3",
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "at least one non-empty content block")
}

func TestSanitizeGrokChatCompletionsMessagesPreservesToolSemantics(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model":"grok-build-0.1",
		"messages":[
			{"role":"system","content":""},
			{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},
			{"role":"tool","tool_call_id":"call_1","content":""},
			{"role":"user","content":[{"type":"text","text":""},{"type":"text","text":"continue"}]}
		]
	}`)
	patched, err := sanitizeGrokChatCompletionsMessages(body)
	require.NoError(t, err)
	require.Len(t, gjson.GetBytes(patched, "messages").Array(), 3, string(patched))
	require.False(t, gjson.GetBytes(patched, "messages.0.content").Exists(), string(patched))
	require.Equal(t, "(empty)", gjson.GetBytes(patched, "messages.1.content").String())
	require.Equal(t, "continue", gjson.GetBytes(patched, "messages.2.content.0.text").String())
}

func TestSanitizeGrokChatCompletionsMessagesRejectsAllEmptyInput(t *testing.T) {
	t.Parallel()

	_, err := sanitizeGrokChatCompletionsMessages(
		[]byte(`{"model":"grok-build-0.1","messages":[{"role":"user","content":""}]}`),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "at least one non-empty content block")
}

func TestPatchGrokResponsesBodyAppliesCPAThinkingSuffix(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"grok-4.3","input":"hello"}`)

	patched, err := patchGrokResponsesBody(body, "grok-4.3(low)")
	require.NoError(t, err)
	require.Equal(t, "grok-4.3", gjson.GetBytes(patched, "model").String())
	require.Equal(t, "low", gjson.GetBytes(patched, "reasoning.effort").String())
}

func TestExtractGrokResponsesReasoningEffortSupportsOpenAICompatibleField(t *testing.T) {
	t.Parallel()

	effort := extractOpenAIReasoningEffortFromBody(
		[]byte(`{"model":"grok-4.3","reasoning_effort":"high"}`),
		"grok-4.3",
	)
	require.NotNil(t, effort)
	require.Equal(t, "high", *effort)
}

func TestPatchGrokResponsesBodyKeepsExplicitReasoningEffortOverSuffix(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"grok-4.3","input":"hello","reasoning":{"effort":"high"}}`)

	patched, err := patchGrokResponsesBody(body, "grok-4.3(low)")
	require.NoError(t, err)
	require.Equal(t, "grok-4.3", gjson.GetBytes(patched, "model").String())
	require.Equal(t, "high", gjson.GetBytes(patched, "reasoning.effort").String())
}

func TestPatchGrokResponsesBodyDropsReasoningEffortForModelsWithoutCPAThinking(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		upstreamModel string
		wantEffort    bool
	}{
		{name: "grok 4.3 supports thinking", upstreamModel: "grok-4.3", wantEffort: true},
		{name: "grok 3 mini supports thinking", upstreamModel: "grok-3-mini", wantEffort: true},
		{name: "grok 4.20 reasoning supports thinking", upstreamModel: "grok-4.20-0309-reasoning", wantEffort: true},
		{name: "grok 4.20 multi agent supports thinking", upstreamModel: "grok-4.20-multi-agent-0309", wantEffort: true},
		{name: "composer has no CPA thinking metadata", upstreamModel: "grok-composer-2.5-fast", wantEffort: false},
		{name: "non reasoning 4.20 has no CPA thinking metadata", upstreamModel: "grok-4.20-0309-non-reasoning", wantEffort: false},
		{name: "legacy grok 4 has no CPA thinking metadata", upstreamModel: "grok-4", wantEffort: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(`{"model":"grok","input":"hello","reasoning":{"effort":"high"}}`)
			patched, err := patchGrokResponsesBody(body, tt.upstreamModel)
			require.NoError(t, err)
			require.Equal(t, tt.wantEffort, gjson.GetBytes(patched, "reasoning.effort").Exists(), string(patched))
			if !tt.wantEffort {
				require.False(t, gjson.GetBytes(patched, "reasoning").Exists(), string(patched))
			}
		})
	}
}

func TestPatchGrokResponsesBodyDropsNestedUnsupportedFields(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model": "grok",
		"input": "hello",
		"external_web_access": true,
		"tools": [
			{"type": "function", "name": "kept_fn", "external_web_access": true, "parameters": {"type": "object", "properties": {"q": {"type": "string", "external_web_access": true}}}}
		],
		"metadata": {"external_web_access": false}
	}`)

	patched, err := patchGrokResponsesBody(body, "grok-4.3")
	require.NoError(t, err)
	require.True(t, json.Valid(patched))
	require.False(t, strings.Contains(string(patched), "external_web_access"))
	require.Equal(t, "kept_fn", gjson.GetBytes(patched, "tools.0.name").String())
}

func TestPatchGrokResponsesBodyDropsUnsupportedNamespaceTools(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model": "grok",
		"input": "hello",
		"tools": [
			{"type": "namespace", "namespace": "functions", "tools": [{"type": "function", "name": "inner"}]},
			{"type": "function", "name": "kept_fn", "parameters": {"type": "object"}},
			{"type": "shell", "name": "kept_shell"}
		],
		"tool_choice": {"type": "function", "name": "kept_fn"}
	}`)

	patched, err := patchGrokResponsesBody(body, "grok-4.3")
	require.NoError(t, err)
	require.True(t, json.Valid(patched))
	require.Equal(t, "grok-4.3", gjson.GetBytes(patched, "model").String())
	require.Len(t, gjson.GetBytes(patched, "tools").Array(), 2)
	require.False(t, gjson.GetBytes(patched, `tools.#(type=="namespace")`).Exists())
	require.True(t, gjson.GetBytes(patched, `tools.#(type=="function")`).Exists())
	require.True(t, gjson.GetBytes(patched, `tools.#(type=="shell")`).Exists())
	require.Equal(t, "kept_fn", gjson.GetBytes(patched, "tool_choice.name").String())
}

func TestPatchGrokResponsesBodyFlattensChatStyleFunctionTool(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model":"grok",
		"input":"hello",
		"tools":[{"type":"function","function":{"name":"lookup","description":"Lookup a value","parameters":{"type":"object"}}}],
		"tool_choice":{"type":"function","function":{"name":"lookup"}}
	}`)
	patched, err := patchGrokResponsesBody(body, "grok-4.3")
	require.NoError(t, err)
	require.Equal(t, "lookup", gjson.GetBytes(patched, "tools.0.name").String(), string(patched))
	require.Equal(t, "Lookup a value", gjson.GetBytes(patched, "tools.0.description").String(), string(patched))
	require.False(t, gjson.GetBytes(patched, "tools.0.function").Exists(), string(patched))
	require.Equal(t, "lookup", gjson.GetBytes(patched, "tool_choice.name").String(), string(patched))
	require.False(t, gjson.GetBytes(patched, "tool_choice.function").Exists(), string(patched))
}

func TestPatchGrokResponsesBodyRejectsFunctionToolWithoutName(t *testing.T) {
	t.Parallel()

	_, err := patchGrokResponsesBody(
		[]byte(`{"model":"grok","input":"hello","tools":[{"type":"function","parameters":{"type":"object"}}]}`),
		"grok-4.3",
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "tools[0].name is required")
}

func TestPatchGrokResponsesBodyRejectsMoreThan250SupportedTools(t *testing.T) {
	t.Parallel()

	tools := make([]map[string]any, 251)
	for i := range tools {
		tools[i] = map[string]any{"type": "function", "name": fmt.Sprintf("tool_%d", i), "parameters": map[string]any{"type": "object"}}
	}
	body, err := json.Marshal(map[string]any{"model": "grok", "input": "hello", "tools": tools})
	require.NoError(t, err)

	_, err = patchGrokResponsesBody(body, "grok-4.3")
	require.Error(t, err)
	require.Contains(t, err.Error(), "at most 250")
}

func TestPatchGrokResponsesBodyDropsToolChoiceWhenNoSupportedToolsRemain(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model": "grok",
		"input": "hello",
		"tools": [
			{"type": "namespace", "namespace": "functions"},
			{"type": "image_generation", "model": "gpt-image-2"}
		],
		"tool_choice": {"type": "namespace", "namespace": "functions"}
	}`)

	patched, err := patchGrokResponsesBody(body, "grok-4.3")
	require.NoError(t, err)
	require.True(t, json.Valid(patched))
	require.False(t, gjson.GetBytes(patched, "tools").Exists())
	require.False(t, gjson.GetBytes(patched, "tool_choice").Exists())
}

func TestPatchGrokResponsesBodyDropsParallelToolCallsWhenToolsEmptyOrMissing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body []byte
	}{
		{
			name: "tools filtered empty",
			body: []byte(`{"model":"grok-4.3","input":"hi","tools":[{"type":"image_generation"}],"tool_choice":"auto","parallel_tool_calls":true}`),
		},
		{
			name: "tools missing",
			body: []byte(`{"model":"grok-4.3","input":"hi","tool_choice":"auto","parallel_tool_calls":true}`),
		},
		{
			name: "orphaned parallel only",
			body: []byte(`{"model":"grok-4.3","input":"hi","parallel_tool_calls":true}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patched, err := patchGrokResponsesBody(tt.body, "grok-4.3")
			require.NoError(t, err)
			require.False(t, gjson.GetBytes(patched, "tools").Exists(), string(patched))
			require.False(t, gjson.GetBytes(patched, "tool_choice").Exists(), string(patched))
			require.False(t, gjson.GetBytes(patched, "parallel_tool_calls").Exists(), string(patched))
		})
	}
}

func TestPatchGrokResponsesBodyRemovesEncryptedReasoningInclude(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"grok-4.3","input":"hello","include":["reasoning.encrypted_content","response.output_text.delta"]}`)

	patched, err := patchGrokResponsesBody(body, "grok-4.3")
	require.NoError(t, err)
	require.False(t, strings.Contains(string(patched), "reasoning.encrypted_content"))
	require.Equal(t, "response.output_text.delta", gjson.GetBytes(patched, "include.0").String())
	require.False(t, gjson.GetBytes(patched, "include.1").Exists())
}

func TestPatchGrokResponsesBodyDropsInvalidEncryptedContentLikeCPA(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"grok-4.3","input":[{"type":"reasoning","summary":[{"type":"summary_text","text":"keep"}],"content":null,"encrypted_content":"gAAAAABinvalid-gpt-shape"},{"type":"compaction","encrypted_content":"gAAAAABforeign-codex-replay"},{"role":"user","content":"hi"}]}`)

	patched, err := patchGrokResponsesBody(body, "grok-4.3")
	require.NoError(t, err)
	require.Equal(t, "reasoning", gjson.GetBytes(patched, "input.0.type").String())
	require.Equal(t, "keep", gjson.GetBytes(patched, "input.0.summary.0.text").String())
	require.False(t, gjson.GetBytes(patched, "input.0.encrypted_content").Exists())
	require.False(t, gjson.GetBytes(patched, "input.0.content").Exists())
	require.Equal(t, "user", gjson.GetBytes(patched, "input.1.role").String())
	require.False(t, gjson.GetBytes(patched, "input.2").Exists())
	require.False(t, strings.Contains(string(patched), "compaction"))
}

func TestPatchGrokResponsesBodyMergesAdjacentReasoningSummariesAfterSanitize(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"grok-4.3","input":[{"type":"reasoning","summary":[{"type":"summary_text","text":"first"}]},{"type":"reasoning","summary":[{"type":"summary_text","text":"second"}],"encrypted_content":"gAAAAABforeign-codex-replay"},{"role":"user","content":"hi"}]}`)

	patched, err := patchGrokResponsesBody(body, "grok-4.3")
	require.NoError(t, err)
	require.Equal(t, "reasoning", gjson.GetBytes(patched, "input.0.type").String())
	require.Equal(t, "first", gjson.GetBytes(patched, "input.0.summary.0.text").String())
	require.Equal(t, "second", gjson.GetBytes(patched, "input.0.summary.1.text").String())
	require.Equal(t, "user", gjson.GetBytes(patched, "input.1.role").String())
	require.False(t, gjson.GetBytes(patched, "input.2").Exists())
}

func TestPatchGrokResponsesBodyRejectsLowEntropyEncryptedContent(t *testing.T) {
	t.Parallel()

	lowEntropy := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	body := []byte(`{"model":"grok-4.3","input":[{"type":"reasoning","summary":[{"type":"summary_text","text":"low entropy"}],"encrypted_content":""},{"role":"user","content":"hi"}]}`)
	body, err := sjson.SetBytes(body, "input.0.encrypted_content", lowEntropy)
	require.NoError(t, err)

	patched, err := patchGrokResponsesBody(body, "grok-4.3")
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(patched, "input.0.encrypted_content").Exists(), string(patched))
	require.Equal(t, "low entropy", gjson.GetBytes(patched, "input.0.summary.0.text").String())
}

func TestBuildGrokResponsesRequestUsesAccountBaseURLAndBearerToken(t *testing.T) {
	t.Setenv(xai.EnvAllowUnsafeURLOverrides, "true")

	account := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"base_url": "https://xai.test/v1/",
		},
	}

	req, err := buildGrokResponsesRequest(context.Background(), nil, account, []byte(`{"model":"grok-4.3"}`), "access-token", nil)
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, req.Method)
	require.Equal(t, "https://xai.test/v1/responses", req.URL.String())
	require.Equal(t, "Bearer access-token", req.Header.Get("Authorization"))
	require.Equal(t, "application/json", req.Header.Get("Content-Type"))
	require.Contains(t, req.Header.Get("Accept"), "text/event-stream")

	data, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, `{"model":"grok-4.3"}`, strings.TrimSpace(string(data)))
}

func TestBuildGrokResponsesRequestIsolatesXGrokConversationIDByAPIKey(t *testing.T) {
	account := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"base_url": xai.DefaultCLIBaseURL,
		},
	}
	build := func(apiKeyID int64) *http.Request {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		c.Request.Header.Set("x-grok-conv-id", "shared-conversation")
		c.Set("api_key", &APIKey{ID: apiKeyID, Group: &Group{Platform: PlatformGrok}})
		req, err := buildGrokResponsesRequest(context.Background(), c, account, []byte(`{"model":"grok-4.3"}`), "access-token", nil)
		require.NoError(t, err)
		return req
	}

	first := build(101).Header.Get("x-grok-conv-id")
	second := build(202).Header.Get("x-grok-conv-id")
	require.Equal(t, isolateOpenAISessionID(101, "shared-conversation"), first)
	require.Equal(t, isolateOpenAISessionID(202, "shared-conversation"), second)
	require.NotEqual(t, first, second)
}

func TestBuildGrokResponsesRequestRejectsUnsafeAccountBaseURL(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"base_url": "https://xai.test/v1",
		},
	}

	_, err := buildGrokResponsesRequest(context.Background(), nil, account, []byte(`{"model":"grok-4.3"}`), "access-token", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid base url")
}

func TestGrokMediaGenerationGateCoversImagesAndVideo(t *testing.T) {
	tests := []struct {
		name     string
		endpoint GrokMediaEndpoint
		want     bool
	}{
		{name: "image generation", endpoint: GrokMediaEndpointImagesGenerations, want: true},
		{name: "image edit", endpoint: GrokMediaEndpointImagesEdits, want: true},
		{name: "video generation", endpoint: GrokMediaEndpointVideosGenerations, want: true},
		{name: "video status", endpoint: GrokMediaEndpointVideoStatus, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.endpoint.IsGenerationRequest())
		})
	}
}

func TestExtractGrokMediaModelSupportsJSONAndMultipart(t *testing.T) {
	require.Equal(t, "grok-imagine", ExtractGrokMediaModel("application/json", []byte(`{"model":"grok-imagine"}`)))

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	require.NoError(t, writer.WriteField("prompt", "draw a cat"))
	require.NoError(t, writer.WriteField("model", "grok-imagine-edit"))
	require.NoError(t, writer.Close())

	require.Equal(t, "grok-imagine-edit", ExtractGrokMediaModel(writer.FormDataContentType(), buf.Bytes()))
}

func TestParseGrokMediaRequestBuildsMultipartModerationBody(t *testing.T) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	require.NoError(t, writer.WriteField("prompt", "edit this private image"))
	require.NoError(t, writer.WriteField("model", "grok-imagine-edit"))
	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", `form-data; name="image"; filename="input.png"`)
	partHeader.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(partHeader)
	require.NoError(t, err)
	_, err = part.Write([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a})
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	info := ParseGrokMediaRequest(writer.FormDataContentType(), buf.Bytes())
	require.Equal(t, "grok-imagine-edit", info.Model)
	require.Equal(t, "edit this private image", info.Prompt)

	moderationBody := info.ModerationBody()
	require.NotEmpty(t, moderationBody)
	require.Equal(t, "edit this private image", gjson.GetBytes(moderationBody, "prompt").String())
	require.True(t, strings.HasPrefix(gjson.GetBytes(moderationBody, "images.0.image_url").String(), "data:image/"))
}

func TestNormalizeGrokMediaModelForEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint GrokMediaEndpoint
		model    string
		want     string
	}{
		{name: "image generation alias", endpoint: GrokMediaEndpointImagesGenerations, model: "grok-imagine", want: "grok-imagine-image-quality"},
		{name: "image edit alias", endpoint: GrokMediaEndpointImagesEdits, model: "grok-imagine", want: "grok-imagine-image-quality"},
		{name: "image quality passthrough", endpoint: GrokMediaEndpointImagesGenerations, model: "grok-imagine-image-quality", want: "grok-imagine-image-quality"},
		{name: "image fast passthrough", endpoint: GrokMediaEndpointImagesGenerations, model: "grok-imagine-image", want: "grok-imagine-image"},
		{name: "video passthrough", endpoint: GrokMediaEndpointVideosGenerations, model: "grok-imagine-video", want: "grok-imagine-video"},
		{name: "video 1.5 legacy alias maps to CPA preview id", endpoint: GrokMediaEndpointVideosGenerations, model: "grok-imagine-video-1.5", want: "grok-imagine-video-1.5-preview"},
		{name: "video 1.5 preview passthrough", endpoint: GrokMediaEndpointVideosGenerations, model: "grok-imagine-video-1.5-preview", want: "grok-imagine-video-1.5-preview"},
		{name: "video 1.5 short alias maps to CPA preview id", endpoint: GrokMediaEndpointVideosGenerations, model: "grok-video-1.5", want: "grok-imagine-video-1.5-preview"},
		{name: "provider-prefixed image alias", endpoint: GrokMediaEndpointImagesGenerations, model: "xai/grok-imagine", want: "grok-imagine-image-quality"},
		{name: "provider-prefixed video preview", endpoint: GrokMediaEndpointVideosGenerations, model: "x-ai/grok-imagine-video-1.5-preview", want: "grok-imagine-video-1.5-preview"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, normalizeGrokMediaModelForEndpoint(tt.endpoint, tt.model))
		})
	}
}

func TestForwardGrokMediaImagesGenerationNormalizesImagineAlias(t *testing.T) {
	t.Setenv(xai.EnvAllowUnsafeURLOverrides, "true")
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok-imagine","prompt":"draw a cat"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	account := &Account{
		ID:          61,
		Name:        "grok",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "api-key",
			"base_url": "https://xai.test/v1",
		},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":   []string{"application/json"},
			"Xai-Request-Id": []string{"xai-image-req"},
		},
		Body: io.NopCloser(strings.NewReader(`{"data":[]}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	result, err := svc.ForwardGrokMedia(context.Background(), c, account, GrokMediaEndpointImagesGenerations, "", body, "application/json")
	require.NoError(t, err)
	require.Equal(t, "https://xai.test/v1/images/generations", upstream.lastReq.URL.String())
	require.Equal(t, http.MethodPost, upstream.lastReq.Method)
	require.Equal(t, "Bearer api-key", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "application/json", upstream.lastReq.Header.Get("Content-Type"))
	require.JSONEq(t, `{"model":"grok-imagine-image-quality","prompt":"draw a cat"}`, string(upstream.lastBody))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"data":[]}`, recorder.Body.String())
	require.Equal(t, "xai-image-req", result.RequestID)
	require.Equal(t, "grok-imagine", result.Model)
	require.Equal(t, "grok-imagine", result.BillingModel)
	require.Equal(t, "grok-imagine-image-quality", result.UpstreamModel)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, ImageBillingSize2K, result.ImageSize)
}

func TestForwardGrokMediaImagesEditMultipartConvertsToJSON(t *testing.T) {
	t.Setenv(xai.EnvAllowUnsafeURLOverrides, "true")
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	require.NoError(t, writer.WriteField("model", "grok-imagine-edit"))
	require.NoError(t, writer.WriteField("prompt", "edit this private image"))
	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", `form-data; name="image"; filename="input.png"`)
	partHeader.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(partHeader)
	require.NoError(t, err)
	_, err = part.Write([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a})
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(buf.Bytes()))
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	account := &Account{
		ID:          62,
		Name:        "grok",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "api-key",
			"base_url": "https://xai.test/v1",
		},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(strings.NewReader(`{"data":[]}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	_, err = svc.ForwardGrokMedia(context.Background(), c, account, GrokMediaEndpointImagesEdits, "", buf.Bytes(), writer.FormDataContentType())
	require.NoError(t, err)
	require.Equal(t, "https://xai.test/v1/images/edits", upstream.lastReq.URL.String())
	require.Equal(t, "application/json", upstream.lastReq.Header.Get("Content-Type"))
	require.True(t, json.Valid(upstream.lastBody))
	require.Equal(t, "grok-imagine-image-quality", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "edit this private image", gjson.GetBytes(upstream.lastBody, "prompt").String())
	require.True(t, strings.HasPrefix(gjson.GetBytes(upstream.lastBody, "image.url").String(), "data:image/png;base64,"))
	require.Equal(t, "image_url", gjson.GetBytes(upstream.lastBody, "image.type").String())
}

func TestForwardGrokMediaVideoGenerationReturnsUsageAndResponseID(t *testing.T) {
	t.Setenv(xai.EnvAllowUnsafeURLOverrides, "true")
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok-imagine-video-1.5","prompt":"waves"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	account := &Account{
		ID:          63,
		Name:        "grok",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "api-key",
			"base_url": "https://xai.test/v1",
		},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":   []string{"application/json"},
			"Xai-Request-Id": []string{"xai-video-generate-req"},
		},
		Body: io.NopCloser(strings.NewReader(`{"request_id":"video-request-123","usage":{"prompt_tokens":3,"completion_tokens":4}}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	result, err := svc.ForwardGrokMedia(context.Background(), c, account, GrokMediaEndpointVideosGenerations, "", body, "application/json")
	require.NoError(t, err)
	require.Equal(t, "https://xai.test/v1/videos/generations", upstream.lastReq.URL.String())
	require.Equal(t, "video-request-123", result.ResponseID)
	require.Equal(t, "grok-imagine-video-1.5-preview", result.BillingModel)
	require.Equal(t, 3, result.Usage.InputTokens)
	require.Equal(t, 4, result.Usage.OutputTokens)
	require.Equal(t, 1, result.VideoCount)
}

func TestForwardGrokMediaVideoStatusUsesGETWithoutBody(t *testing.T) {
	t.Setenv(xai.EnvAllowUnsafeURLOverrides, "true")
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/request-123", nil)

	account := &Account{
		ID:          62,
		Name:        "grok",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "api-key",
			"base_url": "https://xai.test/v1",
		},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":   []string{"application/json"},
			"Xai-Request-Id": []string{"xai-video-req"},
		},
		Body: io.NopCloser(strings.NewReader(`{"id":"request-123","status":"completed"}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	result, err := svc.ForwardGrokMedia(context.Background(), c, account, GrokMediaEndpointVideoStatus, "request-123", nil, "")
	require.NoError(t, err)
	require.Equal(t, "https://xai.test/v1/videos/request-123", upstream.lastReq.URL.String())
	require.Equal(t, http.MethodGet, upstream.lastReq.Method)
	require.Equal(t, "Bearer api-key", upstream.lastReq.Header.Get("Authorization"))
	require.Empty(t, upstream.lastReq.Header.Get("Content-Type"))
	require.Empty(t, upstream.lastBody)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"object":"video","id":"request-123","model":"grok-imagine-video","status":"completed"}`, recorder.Body.String())
	require.Equal(t, "xai-video-req", result.RequestID)
}

func TestBindGrokMediaVideoRequestAccountUsesRequestIDStickyHash(t *testing.T) {
	ctx := context.Background()
	groupID := int64(7)
	cache := &stubGatewayCache{}
	svc := &OpenAIGatewayService{cache: cache}

	hash := GrokMediaVideoRequestSessionHash("video-request-123")
	require.NotEmpty(t, hash)
	require.NoError(t, svc.BindGrokMediaVideoRequestAccount(ctx, &groupID, "video-request-123", 63))

	accountID, err := svc.getStickySessionAccountID(ctx, &groupID, hash)
	require.NoError(t, err)
	require.Equal(t, int64(63), accountID)
}

func TestBindGrokMediaVideoRequestModelUsesRequestIDStickyHash(t *testing.T) {
	ctx := context.Background()
	groupID := int64(7)
	cache := &stubGatewayCache{}
	svc := &OpenAIGatewayService{cache: cache}

	require.NoError(t, svc.BindGrokMediaVideoRequestModel(ctx, &groupID, "video-request-preview", "grok-imagine-video-1.5-preview"))
	got, err := svc.GetGrokMediaVideoRequestModel(ctx, &groupID, "video-request-preview")
	require.NoError(t, err)
	require.Equal(t, "grok-imagine-video-1.5-preview", got)
}

func TestForwardGrokMediaErrorHonorsCustomErrorCodes(t *testing.T) {
	t.Setenv(xai.EnvAllowUnsafeURLOverrides, "true")
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok-imagine","prompt":"draw a cat"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	account := &Account{
		ID:          64,
		Name:        "grok",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":                    "api-key",
			"base_url":                   "https://xai.test/v1",
			"custom_error_codes_enabled": true,
			"custom_error_codes":         []any{float64(http.StatusTooManyRequests)},
		},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header: http.Header{
			"Content-Type":   []string{"application/json"},
			"Xai-Request-Id": []string{"xai-error-req"},
		},
		Body: io.NopCloser(strings.NewReader(`{"error":{"message":"do not expose this upstream detail"}}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	result, err := svc.ForwardGrokMedia(context.Background(), c, account, GrokMediaEndpointImagesGenerations, "", body, "application/json")
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Upstream gateway error")
	require.NotContains(t, recorder.Body.String(), "do not expose")
}

func TestForwardGrokMediaRejectsNilAccount(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader([]byte(`{"model":"grok-imagine","prompt":"draw a cat"}`)))
	c.Request.Header.Set("Content-Type", "application/json")

	svc := &OpenAIGatewayService{}

	result, err := svc.ForwardGrokMedia(
		context.Background(),
		c,
		nil,
		GrokMediaEndpointImagesGenerations,
		"",
		[]byte(`{"model":"grok-imagine","prompt":"draw a cat"}`),
		"application/json",
	)

	require.Nil(t, result)
	require.EqualError(t, err, "account is required")
}

func TestForwardAsChatCompletionsForGrokUsesXAIChatCompletionsAndSnapshots(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok","messages":[{"role":"user","content":"hi"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))

	account := &Account{
		ID:          51,
		Name:        "grok",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"base_url":     xai.DefaultCLIBaseURL,
		},
	}
	repo := &grokQuotaAccountRepo{
		mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
			accountsByID: map[int64]*Account{51: account},
		},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":                   []string{"application/json"},
			"Xai-Request-Id":                 []string{"xai-req"},
			"X-Ratelimit-Limit-Requests":     []string{"10"},
			"X-Ratelimit-Remaining-Requests": []string{"9"},
			"X-Ratelimit-Limit-Tokens":       []string{"1000"},
			"X-Ratelimit-Remaining-Tokens":   []string{"990"},
		},
		Body: io.NopCloser(strings.NewReader(`{"id":"chatcmpl","object":"chat.completion","model":"grok-4.3","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":2}}`)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream:      upstream,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		accountRepo:       repo,
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.NoError(t, err)
	require.Equal(t, xai.DefaultCLIBaseURL+"/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer access-token", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, xai.DefaultTextModel, gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "grok", result.Model)
	require.Equal(t, xai.DefaultTextModel, result.UpstreamModel)
	require.Equal(t, 1, result.Usage.InputTokens)
	require.Equal(t, 2, result.Usage.OutputTokens)
	require.NotNil(t, repo.updates[51][grokQuotaSnapshotExtraKey])
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestForwardGrokResponsesStreamingUsesXAIResponsesAndSnapshots(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok","input":"hi","stream":true,"prompt_cache_key":"grok-session-1","reasoning_effort":"high"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("OpenAI-Beta", "responses=experimental")

	account := &Account{
		ID:          52,
		Name:        "grok",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"base_url":     xai.DefaultCLIBaseURL,
		},
	}
	repo := &grokQuotaAccountRepo{
		mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
			accountsByID: map[int64]*Account{52: account},
		},
	}
	upstreamBody := strings.Join([]string{
		`data: {"type":"response.output_text.delta","sequence_number":0,"delta":"ok"}`,
		"",
		`data: {"type":"response.completed","sequence_number":1,"response":{"id":"resp_grok","model":"grok-4.3","usage":{"input_tokens":5,"output_tokens":3,"input_tokens_details":{"cached_tokens":2}}}}`,
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":                   []string{"text/event-stream"},
			"Xai-Request-Id":                 []string{"xai-stream-req"},
			"X-Ratelimit-Limit-Requests":     []string{"10"},
			"X-Ratelimit-Remaining-Requests": []string{"8"},
			"X-Ratelimit-Limit-Tokens":       []string{"1000"},
			"X-Ratelimit-Remaining-Tokens":   []string{"990"},
		},
		Body: io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream:      upstream,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		accountRepo:       repo,
	}

	result, err := svc.forwardGrokResponses(context.Background(), c, account, body, "grok", true, time.Now())
	require.NoError(t, err)
	require.Equal(t, xai.DefaultCLIBaseURL+"/responses", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer access-token", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "high", gjson.GetBytes(upstream.lastBody, "reasoning_effort").String())
	require.Equal(t, "Keep-Alive", upstream.lastReq.Header.Get("Connection"))
	require.Equal(t, "responses=experimental", upstream.lastReq.Header.Get("OpenAI-Beta"))
	require.NotEmpty(t, upstream.lastReq.Header.Get("session_id"))
	require.Equal(t, upstream.lastReq.Header.Get("session_id"), upstream.lastReq.Header.Get("x-grok-conv-id"))
	require.NotEqual(t, "grok-session-1", upstream.lastReq.Header.Get("x-grok-conv-id"))
	require.Equal(t, xai.DefaultTextModel, gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "grok-session-1", gjson.GetBytes(upstream.lastBody, "prompt_cache_key").String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.True(t, result.Stream)
	require.Equal(t, "resp_grok", result.ResponseID)
	require.Equal(t, "xai-stream-req", result.RequestID)
	require.Equal(t, 5, result.Usage.InputTokens)
	require.Equal(t, 3, result.Usage.OutputTokens)
	require.Equal(t, 2, result.Usage.CacheReadInputTokens)
	require.NotNil(t, result.ReasoningEffort)
	require.Equal(t, "high", *result.ReasoningEffort)
	require.Contains(t, recorder.Header().Get("Content-Type"), "text/event-stream")
	require.Contains(t, recorder.Body.String(), "response.output_text.delta")
	require.NotNil(t, repo.updates[52][grokQuotaSnapshotExtraKey])
}

func TestForwardGrokResponsesStreamingNormalizesXAIReasoningTextEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok","input":"hi","stream":true}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	account := &Account{
		ID:          152,
		Name:        "grok",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"base_url":     xai.DefaultCLIBaseURL,
		},
	}
	repo := &grokQuotaAccountRepo{mockAccountRepoForPlatform: &mockAccountRepoForPlatform{accountsByID: map[int64]*Account{152: account}}}
	upstreamBody := strings.Join([]string{
		`event: response.output_item.added`,
		`data: {"type":"response.output_item.added","sequence_number":1,"output_index":0,"item":{"id":"rs_1","type":"reasoning","status":"in_progress","summary":[]}}`,
		``,
		`event: response.content_part.added`,
		`data: {"type":"response.content_part.added","sequence_number":2,"item_id":"rs_1","output_index":0,"content_index":0,"part":{"type":"reasoning_text","text":""}}`,
		``,
		`event: response.reasoning_text.delta`,
		`data: {"type":"response.reasoning_text.delta","sequence_number":3,"item_id":"rs_1","output_index":0,"content_index":0,"delta":"thinking"}`,
		``,
		`event: response.reasoning_text.done`,
		`data: {"type":"response.reasoning_text.done","sequence_number":4,"item_id":"rs_1","output_index":0,"content_index":0,"text":"thinking"}`,
		``,
		`event: response.output_item.done`,
		`data: {"type":"response.output_item.done","sequence_number":5,"output_index":0,"item":{"id":"rs_1","type":"reasoning","status":"completed","summary":[],"content":[{"type":"reasoning_text","text":"thinking"}]}}`,
		``,
		`event: response.completed`,
		`data: {"type":"response.completed","sequence_number":6,"response":{"id":"resp_reasoning","object":"response","status":"completed","model":"grok-4.3","output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
		``,
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream, grokTokenProvider: NewGrokTokenProvider(repo, nil), accountRepo: repo}

	result, err := svc.forwardGrokResponses(context.Background(), c, account, body, "grok", true, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	output := recorder.Body.String()
	require.NotContains(t, output, "reasoning_text")
	for _, want := range []string{
		"event: response.reasoning_summary_part.added",
		"event: response.reasoning_summary_text.delta",
		"event: response.reasoning_summary_text.done",
		"event: response.reasoning_summary_part.done",
		`"part":{"type":"summary_text","text":"thinking"}`,
		`"summary_index":0`,
		`"summary":[{"type":"summary_text","text":"thinking"}]`,
	} {
		require.Contains(t, output, want)
	}
	require.Less(t,
		strings.Index(output, `"type":"response.reasoning_summary_text.done"`),
		strings.Index(output, `"type":"response.reasoning_summary_part.done"`),
		"reasoning text done must be emitted before part done",
	)
}

func TestForwardGrokResponsesNonStreamNormalizesXAIReasoningOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok","input":"hi","stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	account := &Account{
		ID:          153,
		Name:        "grok",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"base_url":     xai.DefaultCLIBaseURL,
		},
	}
	repo := &grokQuotaAccountRepo{mockAccountRepoForPlatform: &mockAccountRepoForPlatform{accountsByID: map[int64]*Account{153: account}}}
	upstreamBody := strings.Join([]string{
		`data: {"type":"response.output_item.done","sequence_number":1,"output_index":0,"item":{"id":"rs_1","type":"reasoning","status":"completed","summary":[],"content":[{"type":"reasoning_text","text":"thinking"}]}}`,
		``,
		`data: {"type":"response.completed","sequence_number":2,"response":{"id":"resp_reasoning","object":"response","status":"completed","model":"grok-4.3","output":[{"id":"rs_1","type":"reasoning","status":"completed","summary":[],"content":[{"type":"reasoning_text","text":"thinking"}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
		``,
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream, grokTokenProvider: NewGrokTokenProvider(repo, nil), accountRepo: repo}

	result, err := svc.forwardGrokResponses(context.Background(), c, account, body, "grok", false, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	payload := recorder.Body.String()
	require.NotContains(t, payload, "reasoning_text")
	require.Equal(t, "summary_text", gjson.Get(payload, "output.0.summary.0.type").String())
	require.Equal(t, "thinking", gjson.Get(payload, "output.0.summary.0.text").String())
	require.False(t, gjson.Get(payload, "output.0.content").Exists())
}

func TestForwardGrokResponsesAPIKeyUsesXAIResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok","input":"hi","stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	account := &Account{
		ID:          55,
		Name:        "grok-apikey",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "xai-key",
			"base_url": xai.DefaultCLIBaseURL,
		},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":   []string{"application/json"},
			"Xai-Request-Id": []string{"xai-apikey-req"},
		},
		Body: io.NopCloser(strings.NewReader(`{"id":"resp_apikey","object":"response","model":"grok-4.3","output":[],"usage":{"input_tokens":2,"output_tokens":1}}`)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
	}

	result, err := svc.forwardGrokResponses(context.Background(), c, account, body, "grok", false, time.Now())
	require.NoError(t, err)
	require.Equal(t, xai.DefaultCLIBaseURL+"/responses", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer xai-key", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, xai.DefaultTextModel, gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "resp_apikey", result.ResponseID)
	require.True(t, upstream.tlsCalled)
}

func TestForwardGrokResponsesExposesXAIClientError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok","input":"hello","stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	account := &Account{
		ID:          155,
		Name:        "grok-apikey",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "xai-key", "base_url": xai.DefaultCLIBaseURL},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusUnprocessableEntity,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"code":"invalid-argument","error":"tools[0].name is required"}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	result, err := svc.forwardGrokResponses(context.Background(), c, account, body, "grok", false, time.Now())
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	require.Equal(t, "invalid_request_error", gjson.Get(recorder.Body.String(), "error.type").String())
	require.Equal(t, "tools[0].name is required", gjson.Get(recorder.Body.String(), "error.message").String())
}

func TestForwardGrokResponsesRejectsEmptyInputBeforeUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok","input":"","stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))

	account := &Account{
		ID:          156,
		Name:        "grok-apikey",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "xai-key", "base_url": xai.DefaultCLIBaseURL},
	}
	upstream := &httpUpstreamRecorder{}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	result, err := svc.forwardGrokResponses(context.Background(), c, account, body, "grok", false, time.Now())
	require.Error(t, err)
	require.Nil(t, result)
	require.Nil(t, upstream.lastReq)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, gjson.Get(recorder.Body.String(), "error.message").String(), "at least one non-empty content block")
}

func TestForwardGrokResponsesRetriesCompactionBlobErrorWithSanitizedReplay(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"gpt-5.4","stream":false,"previous_response_id":"resp_stale","input":[{"type":"reasoning","encrypted_content":"gAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","summary":[{"type":"summary_text","text":"keep summary"}]},{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	account := &Account{
		ID:          59,
		Name:        "grok-oauth",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"base_url":     xai.DefaultCLIBaseURL,
		},
	}
	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_compaction_bad"}},
				Body:       io.NopCloser(strings.NewReader(`{"code":"invalid-argument","error":"Could not decode the compaction blob. Ensure it is unmodified from the compact response."}`)),
			},
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}, "Xai-Request-Id": []string{"rid_compaction_retry_ok"}},
				Body:       io.NopCloser(strings.NewReader(`{"id":"resp_retry_ok","object":"response","model":"grok-4.3","output":[],"usage":{"input_tokens":3,"output_tokens":1}}`)),
			},
		},
	}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	result, err := svc.forwardGrokResponses(context.Background(), c, account, body, "gpt-5.4", false, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 2)
	require.True(t, gjson.GetBytes(upstream.bodies[0], "previous_response_id").Exists())
	require.False(t, strings.Contains(string(upstream.bodies[0]), "encrypted_content"))
	require.False(t, gjson.GetBytes(upstream.bodies[1], "previous_response_id").Exists())
	require.True(t, gjson.GetBytes(upstream.bodies[1], "store").Exists())
	require.False(t, gjson.GetBytes(upstream.bodies[1], "store").Bool())
	require.False(t, strings.Contains(string(upstream.bodies[1]), "encrypted_content"))
	require.Equal(t, "keep summary", gjson.GetBytes(upstream.bodies[1], "input.0.summary.0.text").String())
	require.Equal(t, "continue", gjson.GetBytes(upstream.bodies[1], "input.1.content.0.text").String())
	require.Equal(t, "rid_compaction_retry_ok", result.RequestID)
	require.Equal(t, "resp_retry_ok", result.ResponseID)
}

func TestForwardGrokResponsesIsolatesPromptCacheKeyConversationByAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	account := &Account{
		ID:          60,
		Name:        "grok-apikey",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "xai-key",
			"base_url": xai.DefaultCLIBaseURL,
		},
	}
	body := []byte(`{"model":"grok","prompt_cache_key":"shared-session","input":"hi","stream":false}`)
	run := func(apiKeyID int64) string {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("api_key", &APIKey{ID: apiKeyID})

		upstream := &httpUpstreamRecorder{resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"resp_grok","object":"response","model":"grok-4.3","output":[],"usage":{"input_tokens":1,"output_tokens":1}}`)),
		}}
		svc := &OpenAIGatewayService{httpUpstream: upstream}

		result, err := svc.forwardGrokResponses(context.Background(), c, account, body, "grok", false, time.Now())
		require.NoError(t, err)
		require.NotNil(t, result)
		convID := upstream.lastReq.Header.Get("x-grok-conv-id")
		require.NotEmpty(t, convID)
		require.NotEqual(t, "shared-session", convID)
		return convID
	}

	require.NotEqual(t, run(101), run(202))
}

func TestForwardGrokResponsesUsesTLSRouterProfileAndHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok","input":"hi","stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "GrokDesktop/1.0")

	account := &Account{
		ID:          56,
		Name:        "grok-oauth",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "grok-token",
			"base_url":     xai.DefaultCLIBaseURL,
		},
		Extra: map[string]any{
			"enable_tls_fingerprint":    true,
			"tls_fingerprint_router_id": float64(30),
		},
	}
	router := &model.TLSFingerprintRouter{
		ID:      30,
		Name:    "grok-router",
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{{
			Name:                    "desktop",
			Enabled:                 true,
			Transport:               model.TLSFingerprintRouterTransportHTTP,
			MatchType:               model.TLSFingerprintRouterMatchContains,
			Pattern:                 "GrokDesktop",
			TLSFingerprintProfileID: 91,
			UpstreamUserAgent:       "grok-native/1.0",
			UpstreamOriginator:      "grok_desktop",
		}},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":   []string{"application/json"},
			"Xai-Request-Id": []string{"xai-grok-tls"},
		},
		Body: io.NopCloser(strings.NewReader(`{"id":"resp_grok_tls","object":"response","model":"grok-4.3","output":[],"usage":{"input_tokens":2,"output_tokens":1}}`)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		tlsFPRouterService: NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{
			routers: []*model.TLSFingerprintRouter{router},
		}, nil),
		tlsFPProfileService: &TLSFingerprintProfileService{localCache: map[int64]*model.TLSFingerprintProfile{
			91: {ID: 91, Name: "Grok Routed", Platform: "grok", Transport: "h2", UserAgent: "profile-ua", Originator: "profile-origin", ALPNProtocols: []string{"h2", "http/1.1"}, HTTP2Fingerprint: "1:4096|ph::method,:scheme,:authority,:path"},
		}},
	}

	_, err := svc.forwardGrokResponses(context.Background(), c, account, body, "grok", false, time.Now())

	require.NoError(t, err)
	require.True(t, upstream.tlsCalled)
	require.NotNil(t, upstream.lastTLSProfile)
	require.Equal(t, "Grok Routed", upstream.lastTLSProfile.Name)
	require.Equal(t, "grok-native/1.0", upstream.lastReq.Header.Get("User-Agent"))
	require.Equal(t, "grok_desktop", upstream.lastReq.Header.Get("Originator"))
}

func TestForwardGrokResponsesDoesNotInjectImageBridgeForTextOnlyRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok","input":"hi","stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	account := &Account{
		ID:          54,
		Name:        "grok",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"base_url":     xai.DefaultCLIBaseURL,
		},
	}
	repo := &grokQuotaAccountRepo{
		mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
			accountsByID: map[int64]*Account{54: account},
		},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":                   []string{"application/json"},
			"Xai-Request-Id":                 []string{"xai-text-req"},
			"X-Ratelimit-Limit-Requests":     []string{"10"},
			"X-Ratelimit-Remaining-Requests": []string{"8"},
			"X-Ratelimit-Limit-Tokens":       []string{"1000"},
			"X-Ratelimit-Remaining-Tokens":   []string{"990"},
		},
		Body: io.NopCloser(strings.NewReader(`{"id":"resp_grok","object":"response","model":"grok-4.3","output":[],"usage":{"input_tokens":1,"output_tokens":0}}`)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream:      upstream,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		accountRepo:       repo,
	}

	_, err := svc.forwardGrokResponses(context.Background(), c, account, body, "grok", false, time.Now())
	require.NoError(t, err)
	require.NotNil(t, upstream.lastBody)
	require.False(t, gjson.GetBytes(upstream.lastBody, "tools").Exists(), "text-only Grok responses request should not get image_generation tool injected")
	require.False(t, strings.Contains(string(upstream.lastBody), "sub2api-codex-image-generation"), "text-only Grok responses request should not get bridge instructions")
}

func TestForwardAsChatCompletionsForGrokStreamingUsesRawXAIChatCompletions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok","messages":[{"role":"user","content":"hi"}],"stream":true}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	account := &Account{
		ID:          53,
		Name:        "grok",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"base_url":     xai.DefaultCLIBaseURL,
		},
	}
	repo := &grokQuotaAccountRepo{
		mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
			accountsByID: map[int64]*Account{53: account},
		},
	}
	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_grok","object":"chat.completion.chunk","model":"grok-4.3","choices":[{"index":0,"delta":{"content":"ok"}}]}`,
		"",
		`data: {"id":"chatcmpl_grok","object":"chat.completion.chunk","model":"grok-4.3","choices":[],"usage":{"prompt_tokens":6,"completion_tokens":4,"total_tokens":10,"prompt_tokens_details":{"cached_tokens":1}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":                   []string{"text/event-stream"},
			"X-Request-Id":                   []string{"chat-stream-req"},
			"X-Ratelimit-Limit-Requests":     []string{"10"},
			"X-Ratelimit-Remaining-Requests": []string{"7"},
		},
		Body: io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	svc := &OpenAIGatewayService{
		cfg:               rawChatCompletionsTestConfig(),
		httpUpstream:      upstream,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		accountRepo:       repo,
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.NoError(t, err)
	require.Equal(t, xai.DefaultCLIBaseURL+"/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer access-token", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "text/event-stream", upstream.lastReq.Header.Get("Accept"))
	require.Equal(t, defaultGrokUpstreamUserAgent, upstream.lastReq.Header.Get("User-Agent"))
	require.Equal(t, xai.DefaultTextModel, gjson.GetBytes(upstream.lastBody, "model").String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream_options.include_usage").Bool())
	require.True(t, result.Stream)
	require.Equal(t, 6, result.Usage.InputTokens)
	require.Equal(t, 4, result.Usage.OutputTokens)
	require.Equal(t, 1, result.Usage.CacheReadInputTokens)
	require.Contains(t, recorder.Body.String(), "data: [DONE]")
	require.NotNil(t, repo.updates[53][grokQuotaSnapshotExtraKey])
}

func TestForwardAsChatCompletionsForGrokExposesXAIClientError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok-4.5","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	account := &Account{
		ID:          157,
		Name:        "grok-apikey",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "xai-key", "base_url": xai.DefaultCLIBaseURL},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"code":"invalid-argument","error":"Image dimensions 1x1 are too small"}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, "invalid_request_error", gjson.Get(recorder.Body.String(), "error.type").String())
	require.Equal(t, "Image dimensions 1x1 are too small", gjson.Get(recorder.Body.String(), "error.message").String())
}

func TestForwardAsChatCompletionsForGrokRejectsEmptyInputBeforeUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok-build-0.1","messages":[{"role":"user","content":""}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))

	account := &Account{
		ID:          158,
		Name:        "grok-apikey",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "xai-key", "base_url": xai.DefaultCLIBaseURL},
	}
	upstream := &httpUpstreamRecorder{}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.Error(t, err)
	require.Nil(t, result)
	require.Nil(t, upstream.lastReq)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, gjson.Get(recorder.Body.String(), "error.message").String(), "at least one non-empty content block")
}

func TestForwardAsChatCompletionsForGrokUsesXGrokConvIDHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok","messages":[{"role":"user","content":"hi"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("x-grok-conv-id", "grok-conv-raw-1")
	c.Set("api_key", &APIKey{ID: 77, Group: &Group{Platform: PlatformGrok}})

	account := &Account{
		ID:          531,
		Name:        "grok",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"base_url":     xai.DefaultCLIBaseURL,
		},
	}
	repo := &grokQuotaAccountRepo{
		mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
			accountsByID: map[int64]*Account{531: account},
		},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":   []string{"application/json"},
			"Xai-Request-Id": []string{"xai-req"},
		},
		Body: io.NopCloser(strings.NewReader(`{"id":"chatcmpl","object":"chat.completion","model":"grok-4.3","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream:      upstream,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		accountRepo:       repo,
	}

	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "grok-conv-raw-1", "")
	require.NoError(t, err)
	require.Equal(t, isolateOpenAISessionID(77, "grok-conv-raw-1"), upstream.lastReq.Header.Get("x-grok-conv-id"))
	require.NotEmpty(t, upstream.lastReq.Header.Get("session_id"))
}

func TestForwardAsChatCompletionsForGrokUsesPromptCacheKeyArgumentForAffinity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok","messages":[{"role":"user","content":"hi"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("api_key", &APIKey{ID: 78, Group: &Group{Platform: PlatformGrok}})

	account := &Account{
		ID:          532,
		Name:        "grok",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"base_url":     xai.DefaultCLIBaseURL,
		},
	}
	repo := &grokQuotaAccountRepo{
		mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
			accountsByID: map[int64]*Account{532: account},
		},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":   []string{"application/json"},
			"Xai-Request-Id": []string{"xai-req"},
		},
		Body: io.NopCloser(strings.NewReader(`{"id":"chatcmpl","object":"chat.completion","model":"grok-4.3","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream:      upstream,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		accountRepo:       repo,
	}

	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "pcache-grok-raw-1", "")
	require.NoError(t, err)
	require.Equal(t, isolateOpenAISessionID(78, "pcache-grok-raw-1"), upstream.lastReq.Header.Get("x-grok-conv-id"))
	require.NotEmpty(t, upstream.lastReq.Header.Get("session_id"))
}

func TestForwardAsChatCompletionsForGrokDoesNotDeriveAffinityWhenPromptCacheKeyMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newService := func(upstream *httpUpstreamRecorder) *OpenAIGatewayService {
		account := &Account{
			ID:          533,
			Name:        "grok",
			Platform:    PlatformGrok,
			Type:        AccountTypeOAuth,
			Concurrency: 1,
			Credentials: map[string]any{
				"access_token": "access-token",
				"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
				"base_url":     xai.DefaultCLIBaseURL,
			},
		}
		repo := &grokQuotaAccountRepo{
			mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
				accountsByID: map[int64]*Account{533: account},
			},
		}
		return &OpenAIGatewayService{
			httpUpstream:      upstream,
			grokTokenProvider: NewGrokTokenProvider(repo, nil),
			accountRepo:       repo,
		}
	}

	buildCtx := func(body []byte) (*gin.Context, *Account, *httpUpstreamRecorder, *OpenAIGatewayService) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("api_key", &APIKey{ID: 79, Group: &Group{Platform: PlatformGrok}})

		account := &Account{
			ID:          533,
			Name:        "grok",
			Platform:    PlatformGrok,
			Type:        AccountTypeOAuth,
			Concurrency: 1,
			Credentials: map[string]any{
				"access_token": "access-token",
				"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
				"base_url":     xai.DefaultCLIBaseURL,
			},
		}
		upstream := &httpUpstreamRecorder{resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type":   []string{"application/json"},
				"Xai-Request-Id": []string{"xai-req"},
			},
			Body: io.NopCloser(strings.NewReader(`{"id":"chatcmpl","object":"chat.completion","model":"grok-4.3","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)),
		}}
		svc := newService(upstream)
		return c, account, upstream, svc
	}

	body := []byte(`{"model":"grok","messages":[{"role":"user","content":"hello grok"}],"stream":false}`)
	c1, account1, upstream1, svc1 := buildCtx(body)
	_, err := svc1.ForwardAsChatCompletions(context.Background(), c1, account1, body, "", "")
	require.NoError(t, err)

	c2, account2, upstream2, svc2 := buildCtx(body)
	_, err = svc2.ForwardAsChatCompletions(context.Background(), c2, account2, body, "", "")
	require.NoError(t, err)

	require.Empty(t, upstream1.lastReq.Header.Get("x-grok-conv-id"))
	require.Empty(t, upstream2.lastReq.Header.Get("x-grok-conv-id"))
	require.Empty(t, upstream1.lastReq.Header.Get("session_id"))
}

func TestForwardAsChatCompletionsForGrokComposerBridgesImageInput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok-composer-2.5-fast","messages":[{"role":"system","content":"You are concise."},{"role":"user","content":[{"type":"text","text":"What is shown?"},{"type":"image_url","image_url":{"url":"data:image/png;base64,QUJD"}}]}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	account := &Account{
		ID:          55,
		Name:        "grok",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"base_url":     xai.DefaultCLIBaseURL,
		},
	}
	repo := &grokQuotaAccountRepo{
		mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
			accountsByID: map[int64]*Account{55: account},
		},
	}
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "xai-request-id": []string{"vision-req"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"resp_vision","object":"response","model":"grok-build-0.1","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"A small diagram with ABC letters."}]}],"usage":{"input_tokens":11,"output_tokens":7,"total_tokens":18}}`)),
		},
		{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type":                   []string{"application/json"},
				"X-Request-Id":                   []string{"composer-req"},
				"X-Ratelimit-Limit-Requests":     []string{"10"},
				"X-Ratelimit-Remaining-Requests": []string{"9"},
				"X-Ratelimit-Limit-Tokens":       []string{"1000"},
				"X-Ratelimit-Remaining-Tokens":   []string{"980"},
			},
			Body: io.NopCloser(strings.NewReader(`{"id":"chatcmpl_composer","object":"chat.completion","model":"grok-composer-2.5-fast","choices":[{"index":0,"message":{"role":"assistant","content":"It shows ABC."},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":5,"total_tokens":8}}`)),
		},
	}}
	svc := &OpenAIGatewayService{
		cfg:               rawChatCompletionsTestConfig(),
		httpUpstream:      upstream,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		accountRepo:       repo,
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.requests, 2)
	require.Equal(t, xai.DefaultCLIBaseURL+"/responses", upstream.requests[0].URL.String())
	require.Equal(t, "grok-build-0.1", gjson.GetBytes(upstream.bodies[0], "model").String())
	require.Equal(t, "input_image", gjson.GetBytes(upstream.bodies[0], "input.0.content.1.type").String())
	require.Equal(t, xai.DefaultCLIBaseURL+"/chat/completions", upstream.requests[1].URL.String())
	require.Equal(t, "grok-composer-2.5-fast", gjson.GetBytes(upstream.bodies[1], "model").String())
	require.False(t, strings.Contains(string(upstream.bodies[1]), "image_url"))
	require.Contains(t, gjson.GetBytes(upstream.bodies[1], "messages.1.content").String(), "Image 1 description")
	require.Contains(t, gjson.GetBytes(upstream.bodies[1], "messages.1.content").String(), "A small diagram with ABC letters.")
	require.Equal(t, 14, result.Usage.InputTokens)
	require.Equal(t, 12, result.Usage.OutputTokens)
	require.Equal(t, "It shows ABC.", gjson.Get(recorder.Body.String(), "choices.0.message.content").String())
	require.NotNil(t, repo.updates[55][grokQuotaSnapshotExtraKey])
}

func TestForwardAsAnthropicForGrokUsesXAIResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok","max_tokens":32,"stream":false,"messages":[{"role":"user","content":"hi"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))

	account := &Account{
		ID:          54,
		Name:        "grok",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"base_url":     xai.DefaultCLIBaseURL,
		},
	}
	repo := &grokQuotaAccountRepo{
		mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
			accountsByID: map[int64]*Account{54: account},
		},
	}
	upstream := &httpUpstreamRecorder{resp: openAICompatSSECompletedResponse("resp_grok_messages", "grok-4.3")}
	svc := &OpenAIGatewayService{
		httpUpstream:      upstream,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		accountRepo:       repo,
	}

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "")
	require.NoError(t, err)
	require.Equal(t, xai.DefaultCLIBaseURL+"/responses", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer access-token", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, defaultGrokUpstreamUserAgent, upstream.lastReq.Header.Get("User-Agent"))
	require.Equal(t, xai.DefaultTextModel, gjson.GetBytes(upstream.lastBody, "model").String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.NotContains(t, string(upstream.lastBody), "chatgpt.com")
	require.Equal(t, "grok", result.Model)
	require.Equal(t, xai.DefaultTextModel, result.UpstreamModel)
	require.Equal(t, 5, result.Usage.InputTokens)
	require.Equal(t, 2, result.Usage.OutputTokens)
	require.Contains(t, recorder.Body.String(), `"type":"message"`)
	require.Contains(t, recorder.Body.String(), "ok")
}

func TestBuildGrokSchedulerExtraUpdates_FeedsThresholdEvaluator(t *testing.T) {
	int64p := func(v int64) *int64 { return &v }
	resetUnix := time.Now().Add(90 * time.Minute).Unix()
	snapshot := &xai.QuotaSnapshot{
		Requests: &xai.QuotaWindow{Limit: int64p(100), Remaining: int64p(30)},                         // 70% used
		Tokens:   &xai.QuotaWindow{Limit: int64p(1000), Remaining: int64p(50), ResetUnix: &resetUnix}, // 95% used (most constrained)
	}

	updates := buildGrokSchedulerExtraUpdates(snapshot)
	require.NotNil(t, updates)
	require.InDelta(t, 95.0, updates["grok_sched_utilization"], 0.001, "picks the most-constrained window")
	require.Contains(t, updates, "grok_sched_reset_at")

	// The written extras must actually drive EvaluateAccountSchedulingThreshold
	// (proves the previously-dead read side is now fed).
	account := &Account{Platform: PlatformGrok, Extra: updates}
	decision := EvaluateAccountSchedulingThreshold(account, map[string]int{PlatformGrok: 90}, time.Now())
	require.True(t, decision.ShouldPause)
	require.InDelta(t, 95.0, decision.UsedPercent, 0.001)
	require.NotNil(t, decision.Until)
}

func TestBuildGrokSchedulerExtraUpdates_NilWhenNoQuotaWindows(t *testing.T) {
	require.Nil(t, buildGrokSchedulerExtraUpdates(&xai.QuotaSnapshot{}))
	require.Nil(t, buildGrokSchedulerExtraUpdates(nil))
}

// newGrokErrorTestService wires an OpenAIGatewayService with a RateLimitService
// so Grok 401/429 exercise the unified upstream-error pipeline.
func newGrokErrorTestService(repo *grokQuotaAccountRepo) *OpenAIGatewayService {
	svc := &OpenAIGatewayService{accountRepo: repo}
	svc.rateLimitService = &RateLimitService{
		accountRepo:    repo,
		cfg:            &config.Config{},
		runtimeBlocker: svc,
	}
	return svc
}

func TestHandleGrokAccountUpstreamError_401RoutesThroughUnifiedPipeline(t *testing.T) {
	// OAuth account missing a refresh_token must be permanently disabled (not
	// endlessly reselected every cooldown), matching every other platform.
	account := &Account{ID: 61, Platform: PlatformGrok, Type: AccountTypeOAuth}
	repo := &grokQuotaAccountRepo{mockAccountRepoForPlatform: &mockAccountRepoForPlatform{accountsByID: map[int64]*Account{61: account}}}
	svc := newGrokErrorTestService(repo)

	svc.handleGrokAccountUpstreamError(context.Background(), account, http.StatusUnauthorized, nil, []byte(`{"error":{"message":"unauthorized"}}`))

	require.True(t, svc.isOpenAIAccountRuntimeBlocked(account), "401 with no refresh_token should block scheduling")
	require.Equal(t, 1, repo.setErrorCalls, "401 with no refresh_token permanently disables the account via SetError")
	require.Contains(t, repo.lastSetErrorMsg, "no refresh_token")
}

func TestHandleGrokAccountUpstreamError_401OAuthCooldownWhenRefreshable(t *testing.T) {
	// With a refresh_token the account is temp-unschedulable for the configured
	// OAuth 401 window (default 10m), not permanently disabled.
	account := &Account{ID: 63, Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{"refresh_token": "rt"}}
	repo := &grokQuotaAccountRepo{mockAccountRepoForPlatform: &mockAccountRepoForPlatform{accountsByID: map[int64]*Account{63: account}}}
	svc := newGrokErrorTestService(repo)
	before := time.Now()

	svc.handleGrokAccountUpstreamError(context.Background(), account, http.StatusUnauthorized, nil, []byte(`{"error":{"message":"expired"}}`))

	require.Equal(t, 1, repo.tempUnschedCalls)
	require.Equal(t, account.ID, repo.lastTempUnschedID)
	require.True(t, repo.lastTempUnschedUntil.After(before.Add(10*time.Minute-time.Second)))
	require.True(t, repo.lastTempUnschedUntil.Before(before.Add(10*time.Minute+time.Second)))
}

func TestHandleGrokAccountUpstreamError_429UsesResetHeaderViaRateLimited(t *testing.T) {
	account := &Account{ID: 64, Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{"refresh_token": "rt"}}
	repo := &grokQuotaAccountRepo{mockAccountRepoForPlatform: &mockAccountRepoForPlatform{accountsByID: map[int64]*Account{64: account}}}
	svc := newGrokErrorTestService(repo)
	before := time.Now()

	// Relative retry-after 45s → SetRateLimited ~45s out, runtime blocked.
	svc.handleGrokAccountUpstreamError(context.Background(), account, http.StatusTooManyRequests, http.Header{"Retry-After": []string{"45"}}, nil)

	require.Equal(t, 1, repo.rateLimitedCalls, "429 marks rate-limited (not temp-unschedulable)")
	require.Equal(t, account.ID, repo.lastRateLimitedID)
	require.True(t, repo.lastRateLimitedUntil.After(before.Add(44*time.Second)))
	require.True(t, repo.lastRateLimitedUntil.Before(before.Add(46*time.Second)))
	require.True(t, svc.isOpenAIAccountRuntimeBlocked(account))
}

func TestHandleGrokAccountUpstreamError_429CapsHugeRetryAfter(t *testing.T) {
	account := &Account{ID: 65, Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{"refresh_token": "rt"}}
	repo := &grokQuotaAccountRepo{mockAccountRepoForPlatform: &mockAccountRepoForPlatform{accountsByID: map[int64]*Account{65: account}}}
	svc := newGrokErrorTestService(repo)
	before := time.Now()

	// A hostile retry-after of one day must be clamped to grokMaxUpstreamCooldown.
	svc.handleGrokAccountUpstreamError(context.Background(), account, http.StatusTooManyRequests, http.Header{"Retry-After": []string{"86400"}}, nil)

	require.Equal(t, 1, repo.rateLimitedCalls)
	require.True(t, repo.lastRateLimitedUntil.Before(before.Add(grokMaxUpstreamCooldown+time.Second)))
	require.True(t, repo.lastRateLimitedUntil.After(before.Add(grokMaxUpstreamCooldown-time.Second)))
}

func TestHandleGrokAccountUpstreamError_403BoundedCooldownNotPermanent(t *testing.T) {
	account := &Account{ID: 66, Platform: PlatformGrok, Type: AccountTypeOAuth}
	repo := &grokQuotaAccountRepo{}
	svc := &OpenAIGatewayService{accountRepo: repo}
	before := time.Now()

	svc.handleGrokAccountUpstreamError(context.Background(), account, http.StatusForbidden, nil, nil)

	require.True(t, svc.isOpenAIAccountRuntimeBlocked(account))
	require.Equal(t, 1, repo.tempUnschedCalls)
	require.Equal(t, "grok entitlement or subscription tier denied", repo.lastTempUnschedReason)
	require.True(t, repo.lastTempUnschedUntil.After(before.Add(grokMaxUpstreamCooldown-time.Second)))
	require.True(t, repo.lastTempUnschedUntil.Before(before.Add(grokMaxUpstreamCooldown+time.Second)))
	require.Equal(t, 0, repo.setErrorCalls, "transient 403 must not permanently disable the account")
}

func TestHandleGrokAccountUpstreamError_403SpendingLimitUsesRateLimitedState(t *testing.T) {
	account := &Account{ID: 67, Platform: PlatformGrok, Type: AccountTypeOAuth}
	repo := &grokQuotaAccountRepo{mockAccountRepoForPlatform: &mockAccountRepoForPlatform{accountsByID: map[int64]*Account{67: account}}}
	svc := newGrokErrorTestService(repo)
	before := time.Now()

	svc.handleGrokAccountUpstreamError(
		context.Background(),
		account,
		http.StatusForbidden,
		nil,
		[]byte(`{"code":"personal-team-blocked:spending-limit","error":"You have run out of credits"}`),
	)

	require.Equal(t, 1, repo.rateLimitedCalls)
	require.Equal(t, account.ID, repo.lastRateLimitedID)
	require.True(t, repo.lastRateLimitedUntil.After(before))
	require.Zero(t, repo.tempUnschedCalls)
	require.Zero(t, repo.setErrorCalls)
	require.True(t, svc.isOpenAIAccountRuntimeBlocked(account))
}

func TestIsGrokSpendingLimitError(t *testing.T) {
	require.True(t, isGrokSpendingLimitError([]byte(`{"code":"personal-team-blocked:spending-limit"}`)))
	require.True(t, isGrokSpendingLimitError([]byte(`{"error":{"code":"PERSONAL-TEAM-BLOCKED:SPENDING-LIMIT"}}`)))
	require.False(t, isGrokSpendingLimitError([]byte(`{"code":"personal-team-blocked:subscription-required"}`)))
	require.False(t, isGrokSpendingLimitError([]byte(`{"error":"You have run out of credits"}`)))
}

func TestHandleGrokAccountUpstreamError_5xxShortCooldownDoesNotShortenExistingPause(t *testing.T) {
	existingUntil := time.Now().Add(15 * time.Minute)
	account := &Account{
		ID:                      62,
		Platform:                PlatformGrok,
		Type:                    AccountTypeOAuth,
		TempUnschedulableUntil:  &existingUntil,
		TempUnschedulableReason: "existing pause",
	}
	repo := &grokQuotaAccountRepo{}
	svc := &OpenAIGatewayService{accountRepo: repo}

	// A short 5xx cooldown (2m) must not shorten a longer existing pause (15m).
	svc.handleGrokAccountUpstreamError(context.Background(), account, http.StatusBadGateway, nil, nil)

	require.Equal(t, 1, repo.tempUnschedCalls)
	require.WithinDuration(t, existingUntil, repo.lastTempUnschedUntil, time.Second)
	value, ok := svc.openaiAccountRuntimeBlockUntil.Load(account.ID)
	require.True(t, ok)
	runtimeUntil, ok := value.(time.Time)
	require.True(t, ok)
	require.WithinDuration(t, existingUntil, runtimeUntil, time.Second)
}

func TestForwardGrokResponses_ReportsSearchCountForWebSearchCall(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok","input":"hi","stream":true,"tools":[{"type":"web_search"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))

	account := &Account{
		ID:          53,
		Name:        "grok",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"base_url":     xai.DefaultCLIBaseURL,
		},
	}
	repo := &grokQuotaAccountRepo{mockAccountRepoForPlatform: &mockAccountRepoForPlatform{accountsByID: map[int64]*Account{53: account}}}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "Xai-Request-Id": []string{"xai-req"}},
		Body: io.NopCloser(strings.NewReader(
			`data: {"type":"response.output_text.delta","delta":"ok"}` + "\n\n" +
				`data: {"type":"response.completed","response":{"id":"resp_1","model":"grok-4.3","output":[{"type":"web_search_call","id":"ws_1"}],"usage":{"input_tokens":1,"output_tokens":1}}}` + "\n\n" +
				`data: [DONE]\n\n`,
		)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream, grokTokenProvider: NewGrokTokenProvider(repo, nil), accountRepo: repo}

	result, err := svc.forwardGrokResponses(context.Background(), c, account, body, "grok", true, time.Now())
	require.NoError(t, err)
	require.Equal(t, 1, result.SearchCount)
}

func TestForwardGrokResponsesKeepsTokenUsageWhenWebSearchCallPresent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok","input":"hi","stream":true,"tools":[{"type":"web_search"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))

	account := &Account{
		ID:          54,
		Name:        "grok",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"base_url":     xai.DefaultCLIBaseURL,
		},
	}
	repo := &grokQuotaAccountRepo{mockAccountRepoForPlatform: &mockAccountRepoForPlatform{accountsByID: map[int64]*Account{54: account}}}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "Xai-Request-Id": []string{"xai-req"}},
		Body: io.NopCloser(strings.NewReader(
			`data: {"type":"response.output_text.delta","delta":"ok"}` + "\n\n" +
				`data: {"type":"response.completed","response":{"id":"resp_1","model":"grok-4.3","output":[{"type":"web_search_call","id":"ws_1"}],"usage":{"input_tokens":9,"output_tokens":4}}}` + "\n\n" +
				`data: [DONE]\n\n`,
		)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream, grokTokenProvider: NewGrokTokenProvider(repo, nil), accountRepo: repo}

	result, err := svc.forwardGrokResponses(context.Background(), c, account, body, "grok", true, time.Now())
	require.NoError(t, err)
	require.Equal(t, 1, result.SearchCount)
	require.Equal(t, 9, result.Usage.InputTokens)
	require.Equal(t, 4, result.Usage.OutputTokens)
}
