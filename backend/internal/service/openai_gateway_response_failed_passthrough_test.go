//go:build unit

package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func buildContextLengthFailedSSE() string {
	failed := `{"type":"response.failed","response":{"id":"resp_err","object":"response","status":"failed","error":{"code":"context_length_exceeded","type":"invalid_request_error","message":"Your input exceeds the context window of this model. Please adjust your input and try again."},"output":[],"usage":{"input_tokens":100000,"output_tokens":0,"total_tokens":100000}}}`
	return fmt.Sprintf("data: %s\n\n", failed)
}

func buildCyberPolicyFailedSSE(withOutput bool) string {
	frames := []string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_cyber_passthrough","status":"in_progress"}}`,
		``,
	}
	if withOutput {
		frames = append(frames,
			`event: response.output_text.delta`,
			`data: {"type":"response.output_text.delta","delta":"partial"}`,
			``,
		)
	}
	frames = append(frames,
		`event: response.failed`,
		`data: {"type":"response.failed","response":{"id":"resp_cyber_passthrough","status":"failed","error":{"code":"cyber_policy","message":"blocked by cyber policy"},"usage":{"input_tokens":17,"output_tokens":3,"total_tokens":20}}}`,
		``,
	)
	return strings.Join(frames, "\n")
}

func bindPassthroughRule(c *gin.Context, platform string, keywords []string, responseCode int) {
	svc := &ErrorPassthroughService{}
	rules := make([]*cachedPassthroughRule, 0, len(keywords))
	for i, kw := range keywords {
		code := responseCode
		rules = append(rules, &cachedPassthroughRule{
			ErrorPassthroughRule: &model.ErrorPassthroughRule{
				ID:              int64(i + 1),
				Enabled:         true,
				Platforms:       []string{platform},
				MatchMode:       model.MatchModeAny,
				Keywords:        []string{kw},
				ResponseCode:    &code,
				PassthroughBody: true,
			},
			lowerKeywords:  []string{strings.ToLower(kw)},
			lowerPlatforms: []string{strings.ToLower(platform)},
		})
	}
	svc.localCacheMu.Lock()
	svc.localCache = rules
	svc.localCacheMu.Unlock()
	BindErrorPassthroughService(c, svc)
}

func TestForwardAsChatCompletions_ResponseFailed_PassthroughRule(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	bindPassthroughRule(c, "openai", []string{"context_length_exceeded"}, 400)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(buildContextLengthFailedSSE())),
	}}
	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}

	account := rawChatCompletionsTestAccount()
	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")

	require.Error(t, err)
	require.Contains(t, err.Error(), "passthrough")
	require.Equal(t, 400, rec.Code, "passthrough rule should override 502 to 400")

	respBody := rec.Body.String()
	errType := gjson.Get(respBody, "error.type").String()
	require.Equal(t, "upstream_error", errType)
	errMsg := gjson.Get(respBody, "error.message").String()
	require.NotEmpty(t, errMsg, "passthrough should preserve error message")
	require.Contains(t, errMsg, "context window")
}

func TestForwardAsAnthropic_ResponseFailed_PassthroughRule(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"gpt-5.4","max_tokens":32,"messages":[{"role":"user","content":"hello"}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	bindPassthroughRule(c, "openai", []string{"context_length_exceeded"}, 400)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(buildContextLengthFailedSSE())),
	}}
	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}

	account := rawChatCompletionsTestAccount()
	_, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "")

	require.Error(t, err)
	require.Contains(t, err.Error(), "passthrough")
	require.Equal(t, 400, rec.Code, "passthrough rule should override 502 to 400")
	respBody := rec.Body.String()
	errMsg := gjson.Get(respBody, "error.message").String()
	require.NotEmpty(t, errMsg, "passthrough should preserve error message")
}

func TestCompactResponseFailedPassthroughAfterKeepaliveCommitStaysSSE(t *testing.T) {
	c, rec := newCompactBridgeTestContext(t, true)
	bindPassthroughRule(c, PlatformOpenAI, []string{"context_length_exceeded"}, http.StatusBadRequest)
	stop := StartOpenAICompactSSEKeepalive(c, keepaliveTestInterval)
	defer stop()
	waitForKeepaliveBeats()

	payload := []byte(`{"type":"response.failed","response":{"status":"failed","error":{"code":"context_length_exceeded","message":"context window exceeded"}}}`)
	svc := &OpenAIGatewayService{}
	err := svc.writeOpenAIResponseFailedProtocolError(
		&http.Response{Header: make(http.Header)},
		c,
		&Account{Platform: PlatformOpenAI},
		payload,
		"context window exceeded",
	)

	require.Error(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	events := parseCompactBridgeSSE(t, stripKeepaliveComments(rec.Body.String()))
	require.Len(t, events, 1)
	require.Equal(t, "response.failed", events[0][0])
	require.Equal(t, "upstream_error", gjson.Get(events[0][1], "response.error.code").String())
	require.Contains(t, gjson.Get(events[0][1], "response.error.message").String(), "context window")
	streamErr, ok := GetOpsStreamError(c)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, streamErr.IntendedStatus)
}

func TestCompatCyberPolicyMarkedBeforeResponseFailedPassthrough(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		body      []byte
		forward   func(*OpenAIGatewayService, *gin.Context, *Account, []byte) (*OpenAIForwardResult, error)
		wantUsage bool
	}{
		{
			name: "chat buffered",
			path: "/v1/chat/completions",
			body: []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`),
			forward: func(s *OpenAIGatewayService, c *gin.Context, account *Account, body []byte) (*OpenAIForwardResult, error) {
				return s.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
			},
			wantUsage: true,
		},
		{
			name: "chat streaming",
			path: "/v1/chat/completions",
			body: []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":true}`),
			forward: func(s *OpenAIGatewayService, c *gin.Context, account *Account, body []byte) (*OpenAIForwardResult, error) {
				return s.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
			},
			wantUsage: true,
		},
		{
			name: "messages buffered",
			path: "/v1/messages",
			body: []byte(`{"model":"gpt-5.4","max_tokens":32,"messages":[{"role":"user","content":"hello"}],"stream":false}`),
			forward: func(s *OpenAIGatewayService, c *gin.Context, account *Account, body []byte) (*OpenAIForwardResult, error) {
				return s.ForwardAsAnthropic(context.Background(), c, account, body, "", "")
			},
			wantUsage: true,
		},
		{
			name: "messages streaming",
			path: "/v1/messages",
			body: []byte(`{"model":"gpt-5.4","max_tokens":32,"messages":[{"role":"user","content":"hello"}],"stream":true}`),
			forward: func(s *OpenAIGatewayService, c *gin.Context, account *Account, body []byte) (*OpenAIForwardResult, error) {
				return s.ForwardAsAnthropic(context.Background(), c, account, body, "", "")
			},
			wantUsage: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, tt.path, bytes.NewReader(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")
			bindPassthroughRule(c, PlatformOpenAI, []string{"cyber_policy"}, http.StatusBadRequest)

			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       io.NopCloser(strings.NewReader(buildCyberPolicyFailedSSE(false))),
			}}
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}

			result, err := tt.forward(svc, c, rawChatCompletionsTestAccount(), tt.body)
			require.Error(t, err)
			require.Nil(t, result)
			mark := GetOpsCyberPolicy(c)
			require.NotNil(t, mark, "cyber mark must survive a matching passthrough rule")
			require.Equal(t, "cyber_policy", mark.Code)
			if tt.wantUsage {
				require.Equal(t, 17, mark.UpstreamInTok)
				require.Equal(t, 3, mark.UpstreamOutTok)
			}
		})
	}
}

func TestResponsesResponseFailedPassthroughAfterSSEFlushStaysSSEAndMarksCyber(t *testing.T) {
	tests := []struct {
		name    string
		forward func(*OpenAIGatewayService, context.Context, *http.Response, *gin.Context, *Account) error
	}{
		{
			name: "responses standard",
			forward: func(s *OpenAIGatewayService, ctx context.Context, resp *http.Response, c *gin.Context, account *Account) error {
				_, err := s.handleStreamingResponse(ctx, resp, c, account, time.Now(), "gpt-5.4", "gpt-5.4")
				return err
			},
		},
		{
			name: "responses passthrough",
			forward: func(s *OpenAIGatewayService, ctx context.Context, resp *http.Response, c *gin.Context, account *Account) error {
				_, err := s.handleStreamingResponsePassthrough(ctx, resp, c, account, time.Now(), "gpt-5.4", "gpt-5.4")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			bindPassthroughRule(c, PlatformOpenAI, []string{"cyber_policy"}, http.StatusBadRequest)
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       io.NopCloser(strings.NewReader(buildCyberPolicyFailedSSE(true))),
			}
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}
			account := &Account{ID: 1, Platform: PlatformOpenAI, Name: "responses-cyber"}

			err := tt.forward(svc, c.Request.Context(), resp, c, account)
			require.Error(t, err)
			mark := GetOpsCyberPolicy(c)
			require.NotNil(t, mark)
			require.Equal(t, 17, mark.UpstreamInTok)
			require.Equal(t, 3, mark.UpstreamOutTok)
			body := rec.Body.String()
			require.Contains(t, body, `"delta":"partial"`)
			require.Contains(t, body, `"type":"response.failed"`)
			require.NotContains(t, body, `{"error":{"message":`)
			require.NotContains(t, rec.Header().Get("Content-Type"), "application/json")
		})
	}
}

func TestForwardAsChatCompletions_ResponseFailed_NoRule_Still502(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(buildContextLengthFailedSSE())),
	}}
	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}

	account := rawChatCompletionsTestAccount()
	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")

	require.Error(t, err)
	require.Equal(t, http.StatusBadGateway, rec.Code, "without passthrough rule should still be 502")
}

// bindStatusCodePassthroughRule 绑定一条按错误码+关键词双条件(MatchModeAll)匹配的规则。
// 此类规则依赖语义状态码推断才能在协议转换路径命中（response.failed 无真实 HTTP 状态码）。
func bindStatusCodePassthroughRule(c *gin.Context, platform string, statusCode int, keyword string, responseCode int) {
	rule := &model.ErrorPassthroughRule{
		ID:              1,
		Name:            "status-code-rule",
		Enabled:         true,
		Priority:        1,
		Platforms:       []string{platform},
		ErrorCodes:      []int{statusCode},
		Keywords:        []string{keyword},
		MatchMode:       model.MatchModeAll,
		ResponseCode:    &responseCode,
		PassthroughBody: true,
	}
	svc := &ErrorPassthroughService{}
	svc.setLocalCache([]*model.ErrorPassthroughRule{rule})
	BindErrorPassthroughService(c, svc)
}

func TestForwardAsChatCompletions_ResponseFailed_ErrorCodeRuleMatchesViaSemanticStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	bindStatusCodePassthroughRule(c, "openai", http.StatusBadRequest, "context_length_exceeded", http.StatusBadRequest)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(buildContextLengthFailedSSE())),
	}}
	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}

	account := rawChatCompletionsTestAccount()
	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, rec.Code, "error-code-conditioned rule should match via semantic status inference")
	respBody := rec.Body.String()
	require.Equal(t, "upstream_error", gjson.Get(respBody, "error.type").String())
	require.Contains(t, gjson.Get(respBody, "error.message").String(), "context window")
}

func TestForwardAsAnthropic_ResponseFailed_ErrorCodeRuleMatchesViaSemanticStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"gpt-5.4","max_tokens":32,"messages":[{"role":"user","content":"hello"}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	bindStatusCodePassthroughRule(c, "openai", http.StatusBadRequest, "context_length_exceeded", http.StatusBadRequest)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(buildContextLengthFailedSSE())),
	}}
	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}

	account := rawChatCompletionsTestAccount()
	_, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "")

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, rec.Code, "error-code-conditioned rule should match via semantic status inference")
	respBody := rec.Body.String()
	require.NotEmpty(t, gjson.Get(respBody, "error.message").String())
}
