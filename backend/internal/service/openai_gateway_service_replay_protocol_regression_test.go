package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIReplayProtocol_PassthroughStreamingPreservesSSEControlFields(t *testing.T) {
	setGinTestMode()

	rec, c := newOpenAIReplayProtocolContext()
	body := strings.Join([]string{
		`: keepalive`,
		`id: evt_1`,
		`retry: 1500`,
		`event: response.output_text.delta`,
		`data: {"type":"response.output_text.delta","delta":"ok"}`,
		``,
		`trace: upstream-meta`,
		`data: {"type":"response.completed","response":{"id":"resp_ctrl","object":"response","model":"gpt-5.4","status":"completed","output":[],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	result, err := invokeOpenAIReplayProtocolPassthroughStream(c, openAIReplayProtocolTestAccount(1), body)
	require.NoError(t, err)
	require.NotNil(t, result)

	out := rec.Body.String()
	require.Contains(t, out, ": keepalive")
	require.Contains(t, out, "id: evt_1")
	require.Contains(t, out, "retry: 1500")
	require.Contains(t, out, "trace: upstream-meta")
	require.Contains(t, out, `"delta":"ok"`)
}

func TestOpenAIReplayProtocol_PassthroughRetryDedupeIgnoresNestedToolCallIDsOnSameAccountRetry(t *testing.T) {
	setGinTestMode()

	rec, c := newOpenAIReplayProtocolContext()
	account := openAIReplayProtocolTestAccount(11)

	_, err := invokeOpenAIReplayProtocolPassthroughStream(c, account, strings.Join([]string{
		`event: response.output_item.done`,
		`data: ` + openAIReplayProtocolToolCallDoneEvent("resp_retry_1", "fc_retry_1", "call_retry_same_account"),
		``,
		`data: ` + openAIReplayProtocolOverloadFailedEvent("resp_retry_1"),
		``,
	}, "\n"))
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)

	result, err := invokeOpenAIReplayProtocolPassthroughStream(c, account, strings.Join([]string{
		`event: response.output_item.done`,
		`data: ` + openAIReplayProtocolToolCallDoneEvent("resp_retry_2", "fc_retry_2", "call_retry_same_account"),
		``,
		`data: ` + openAIReplayProtocolCompletedEvent("resp_retry_2"),
		``,
		`data: [DONE]`,
		``,
	}, "\n"))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, strings.Count(rec.Body.String(), `"call_id":"call_retry_same_account"`))
}

func TestOpenAIReplayProtocol_PassthroughAccountSwitchDoesNotReuseReplayState(t *testing.T) {
	setGinTestMode()

	rec, c := newOpenAIReplayProtocolContext()

	_, err := invokeOpenAIReplayProtocolPassthroughStream(c, openAIReplayProtocolTestAccount(21), strings.Join([]string{
		`event: response.output_item.done`,
		`data: ` + openAIReplayProtocolToolCallDoneEvent("resp_switch_1", "fc_switch_1", "call_cross_account"),
		``,
		`data: ` + openAIReplayProtocolOverloadFailedEvent("resp_switch_1"),
		``,
	}, "\n"))
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)

	result, err := invokeOpenAIReplayProtocolPassthroughStream(c, openAIReplayProtocolTestAccount(22), strings.Join([]string{
		`event: response.output_item.done`,
		`data: ` + openAIReplayProtocolToolCallDoneEvent("resp_switch_2", "fc_switch_1", "call_cross_account"),
		``,
		`data: ` + openAIReplayProtocolCompletedEvent("resp_switch_2"),
		``,
		`data: [DONE]`,
		``,
	}, "\n"))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 2, strings.Count(rec.Body.String(), `"call_id":"call_cross_account"`))
}

func TestOpenAIReplayProtocol_BufferedTerminalIgnoresTrailingOverloadAfterCompletion(t *testing.T) {
	setGinTestMode()

	body := []byte(strings.Join([]string{
		`data: {"type":"response.completed","response":{"id":"resp_buffered_terminal","object":"response","model":"gpt-5.4","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":7,"output_tokens":3,"total_tokens":10}}}`,
		`data: ` + openAIReplayProtocolOverloadFailedEvent("resp_buffered_terminal"),
		`data: [DONE]`,
	}, "\n"))

	invokers := []struct {
		name        string
		passthrough bool
	}{
		{name: "standard", passthrough: false},
		{name: "passthrough", passthrough: true},
	}

	for _, invoker := range invokers {
		t.Run(invoker.name, func(t *testing.T) {
			result, err := invokeOpenAIReplayProtocolBufferedSSE(t, body, invoker.passthrough)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result.usage)
			require.Equal(t, 7, result.usage.InputTokens)
			require.Equal(t, 3, result.usage.OutputTokens)
			require.Contains(t, result.body, `"id":"resp_buffered_terminal"`)
			require.Contains(t, result.body, `"text":"ok"`)
		})
	}
}

type openAIReplayProtocolBufferedResult struct {
	usage *OpenAIUsage
	body  string
}

func newOpenAIReplayProtocolContext() (*httptest.ResponseRecorder, *gin.Context) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	return rec, c
}

func openAIReplayProtocolTestAccount(id int64) *Account {
	return &Account{
		ID:          id,
		Name:        fmt.Sprintf("openai-oauth-%d", id),
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
	}
}

func invokeOpenAIReplayProtocolPassthroughStream(c *gin.Context, account *Account, body string) (*openaiStreamingResultPassthrough, error) {
	svc := &OpenAIGatewayService{}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
			"x-request-id": []string{fmt.Sprintf("rid_%d", time.Now().UnixNano())},
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}
	return svc.handleStreamingResponsePassthrough(context.Background(), resp, c, account, time.Now(), "gpt-5.4", "gpt-5.4")
}

func invokeOpenAIReplayProtocolBufferedSSE(t *testing.T, body []byte, passthrough bool) (*openAIReplayProtocolBufferedResult, error) {
	t.Helper()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	account := openAIReplayProtocolTestAccount(99)
	svc := &OpenAIGatewayService{}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}

	var usage *OpenAIUsage
	var err error
	if passthrough {
		var result *openaiNonStreamingResultPassthrough
		result, err = svc.handlePassthroughSSEToJSON(resp, c, account, body, "gpt-5.4", "gpt-5.4")
		if result != nil {
			usage = result.usage
		}
	} else {
		var result *openaiNonStreamingResult
		result, err = svc.handleSSEToJSON(resp, c, account, body, "gpt-5.4", "gpt-5.4")
		if result != nil {
			usage = result.usage
		}
	}

	return &openAIReplayProtocolBufferedResult{
		usage: usage,
		body:  rec.Body.String(),
	}, err
}

func openAIReplayProtocolToolCallDoneEvent(responseID, itemID, callID string) string {
	return fmt.Sprintf(`{"type":"response.output_item.done","response_id":"%s","item":{"type":"function_call","id":"%s","call_id":"%s","name":"webfetch","arguments":"{\"url\":\"https://example.com\"}"}}`, responseID, itemID, callID)
}

func openAIReplayProtocolCompletedEvent(responseID string) string {
	return fmt.Sprintf(`{"type":"response.completed","response":{"id":"%s","object":"response","model":"gpt-5.4","status":"completed","output":[],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}`, responseID)
}

func openAIReplayProtocolOverloadFailedEvent(responseID string) string {
	return fmt.Sprintf(`{"type":"response.failed","response":{"id":"%s","status":"failed","error":{"type":"service_unavailable_error","code":"server_is_overloaded","message":"Our servers are currently overloaded. Please try again later."}}}`, responseID)
}

func TestOpenAIReplayProtocol_PassthroughRetryDedupesFunctionCallArgumentDeltaPrefix(t *testing.T) {
	setGinTestMode()

	rec, c := newOpenAIReplayProtocolContext()
	account := openAIReplayProtocolTestAccount(31)

	firstResp := strings.Join([]string{
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"fc_tool_1","call_id":"call_tool_1","name":"webfetch","arguments":""}}`,
		"",
		`data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":"prefix-"}`,
		"",
		`data: ` + openAIReplayProtocolOverloadFailedEvent("resp_tool_retry_1"),
		"",
	}, "\n")

	_, err := invokeOpenAIReplayProtocolPassthroughStream(c, account, firstResp)
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.True(t, failoverErr.RetryableOnSameAccount)

	secondResp := strings.Join([]string{
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"fc_tool_1","call_id":"call_tool_1","name":"webfetch","arguments":""}}`,
		"",
		`data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":"prefix-suffix"}`,
		"",
		`data: {"type":"response.function_call_arguments.done","output_index":0}`,
		"",
		`data: ` + openAIReplayProtocolCompletedEvent("resp_tool_retry_2"),
		"",
		`data: [DONE]`,
		"",
	}, "\n")

	result, err := invokeOpenAIReplayProtocolPassthroughStream(c, account, secondResp)
	require.NoError(t, err)
	require.NotNil(t, result)

	body := rec.Body.String()
	require.Equal(t, 1, strings.Count(body, `"delta":"prefix-"`))
	require.Contains(t, body, `"delta":"suffix"`)
}
