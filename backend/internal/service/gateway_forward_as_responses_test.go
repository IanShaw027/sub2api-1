//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestExtractResponsesReasoningEffortFromBody(t *testing.T) {
	t.Parallel()

	got := ExtractResponsesReasoningEffortFromBody([]byte(`{"model":"claude-sonnet-4.5","reasoning":{"effort":"HIGH"}}`))
	require.NotNil(t, got)
	require.Equal(t, "high", *got)

	maxGot := ExtractResponsesReasoningEffortFromBody([]byte(`{"model":"deepseek-v4-pro","reasoning":{"effort":"max"}}`))
	require.NotNil(t, maxGot)
	require.Equal(t, "xhigh", *maxGot)

	require.Nil(t, ExtractResponsesReasoningEffortFromBody([]byte(`{"model":"claude-sonnet-4.5"}`)))
}

func TestHandleResponsesBufferedStreamingResponse_PreservesMessageStartCacheUsage(t *testing.T) {
	t.Parallel()
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	resp := &http.Response{
		Header: http.Header{"x-request-id": []string{"rid_buffered"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4.5","stop_reason":"","usage":{"input_tokens":12,"cache_read_input_tokens":9,"cache_creation_input_tokens":3}}}`,
			``,
			`event: content_block_start`,
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":"hello"}}`,
			``,
			`event: message_delta`,
			`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}`,
			``,
			`event: message_stop`,
			`data: {"type":"message_stop"}`,
			``,
		}, "\n"))),
	}

	svc := &GatewayService{}
	result, err := svc.handleResponsesBufferedStreamingResponse(resp, c, "claude-sonnet-4.5", "claude-sonnet-4.5", nil, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 12, result.Usage.InputTokens)
	require.Equal(t, 7, result.Usage.OutputTokens)
	require.Equal(t, 9, result.Usage.CacheReadInputTokens)
	require.Equal(t, 3, result.Usage.CacheCreationInputTokens)
	require.Contains(t, rec.Body.String(), `"cached_tokens":9`)
}

func TestHandleResponsesStreamingResponse_PreservesMessageStartCacheUsage(t *testing.T) {
	t.Parallel()
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	resp := &http.Response{
		Header: http.Header{"x-request-id": []string{"rid_stream"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"id":"msg_2","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4.5","stop_reason":"","usage":{"input_tokens":20,"cache_read_input_tokens":11,"cache_creation_input_tokens":4}}}`,
			``,
			`event: content_block_start`,
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":"hello"}}`,
			``,
			`event: message_delta`,
			`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":8}}`,
			``,
			`event: message_stop`,
			`data: {"type":"message_stop"}`,
			``,
		}, "\n"))),
	}

	svc := &GatewayService{}
	result, err := svc.handleResponsesStreamingResponse(resp, c, "claude-sonnet-4.5", "claude-sonnet-4.5", nil, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 20, result.Usage.InputTokens)
	require.Equal(t, 8, result.Usage.OutputTokens)
	require.Equal(t, 11, result.Usage.CacheReadInputTokens)
	require.Equal(t, 4, result.Usage.CacheCreationInputTokens)
	require.Contains(t, rec.Body.String(), `response.completed`)
}

func TestHandleResponsesStreamingResponse_ReadErrorAfterMessageStartEmitsFailed(t *testing.T) {
	t.Parallel()
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	resp := &http.Response{
		Header: http.Header{"x-request-id": []string{"rid_stream_truncated"}},
		Body: &errAfterDataReadCloser{
			data: []byte(strings.Join([]string{
				`event: message_start`,
				`data: {"type":"message_start","message":{"id":"msg_truncated","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4.5","stop_reason":"","usage":{"input_tokens":20}}}`,
				``,
				`event: content_block_start`,
				`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":"partial"}}`,
				``,
			}, "\n")),
			err: io.ErrUnexpectedEOF,
		},
	}

	svc := &GatewayService{}
	result, err := svc.handleResponsesStreamingResponse(resp, c, "claude-sonnet-4.5", "claude-sonnet-4.5", nil, time.Now())

	require.Error(t, err)
	require.Nil(t, result)
	body := rec.Body.String()
	require.Contains(t, body, "event: response.failed\n")
	require.Contains(t, body, `"type":"response.failed"`)
	require.Contains(t, body, "stream_read_error")
	require.NotContains(t, body, "event: response.completed\n")
}

func TestHandleResponsesStreamingResponse_CleanEOFWithoutMessageStopEmitsFailed(t *testing.T) {
	t.Parallel()
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	resp := &http.Response{
		Header: http.Header{"x-request-id": []string{"rid_stream_clean_eof"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"id":"msg_clean_eof","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4.5","stop_reason":"","usage":{"input_tokens":20}}}`,
			``,
			`event: content_block_start`,
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":"partial"}}`,
			``,
		}, "\n"))),
	}

	svc := &GatewayService{}
	result, err := svc.handleResponsesStreamingResponse(resp, c, "claude-sonnet-4.5", "claude-sonnet-4.5", nil, time.Now())

	require.Error(t, err)
	require.Nil(t, result)
	body := rec.Body.String()
	require.Contains(t, body, "event: response.failed\n")
	require.Contains(t, body, "stream_read_error")
	require.NotContains(t, body, "event: response.completed\n")
}

func TestHandleResponsesBufferedStreamingResponse_ReadErrorAfterMessageStartFails(t *testing.T) {
	t.Parallel()
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	resp := &http.Response{
		Header: http.Header{"x-request-id": []string{"rid_buffered_truncated"}},
		Body: &errAfterDataReadCloser{
			data: []byte(strings.Join([]string{
				`event: message_start`,
				`data: {"type":"message_start","message":{"id":"msg_truncated_buffered","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4.5","stop_reason":"","usage":{"input_tokens":12}}}`,
				``,
				`event: content_block_start`,
				`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":"partial"}}`,
				``,
			}, "\n")),
			err: io.ErrUnexpectedEOF,
		},
	}

	svc := &GatewayService{}
	result, err := svc.handleResponsesBufferedStreamingResponse(resp, c, "claude-sonnet-4.5", "claude-sonnet-4.5", nil, time.Now())

	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusBadGateway, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, "stream_read_error")
	require.NotContains(t, body, `"status":"completed"`)
}

func TestHandleResponsesBufferedStreamingResponse_CleanEOFWithoutMessageStopFails(t *testing.T) {
	t.Parallel()
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	resp := &http.Response{
		Header: http.Header{"x-request-id": []string{"rid_buffered_clean_eof"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"id":"msg_clean_eof_buffered","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4.5","stop_reason":"","usage":{"input_tokens":12}}}`,
			``,
			`event: content_block_start`,
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":"partial"}}`,
			``,
		}, "\n"))),
	}

	svc := &GatewayService{}
	result, err := svc.handleResponsesBufferedStreamingResponse(resp, c, "claude-sonnet-4.5", "claude-sonnet-4.5", nil, time.Now())

	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusBadGateway, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, "stream_read_error")
	require.NotContains(t, body, `"status":"completed"`)
}

func TestPrepareResponsesAnthropicIngress_MergesMessagesWhenInputAlreadyExists(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body []byte
	}{
		{
			name: "empty messages",
			body: []byte(`{"model":"claude-sonnet-4.5","input":[{"type":"function_call_output","call_id":"call_1","output":"ok"}],"messages":[],"previous_response_id":"resp_stale"}`),
		},
		{
			name: "developer messages",
			body: []byte(`{"model":"claude-sonnet-4.5","input":[{"type":"function_call_output","call_id":"call_1","output":"ok"}],"messages":[{"role":"developer","content":"ignore me"}],"previous_response_id":"resp_stale"}`),
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := prepareResponsesAnthropicIngress(tt.body)
			require.NoError(t, err)
			require.Equal(t, string(tt.body), string(plan.PrimaryBody))
			require.True(t, plan.CanRetryWithFullReplay())
			require.False(t, gjson.GetBytes(plan.FullReplayBody, "messages").Exists())
			require.False(t, gjson.GetBytes(plan.FullReplayBody, "previous_response_id").Exists())
			if tt.name == "empty messages" {
				require.Equal(t, "input", plan.FullReplaySource)
				require.Equal(t, "function_call_output", gjson.GetBytes(plan.FullReplayBody, "input.0.type").String())
				require.Equal(t, "call_1", gjson.GetBytes(plan.FullReplayBody, "input.0.call_id").String())
				return
			}
			require.Equal(t, "input+messages", plan.FullReplaySource)
			require.NotEmpty(t, gjson.GetBytes(plan.FullReplayBody, "input.0.role").String())
			require.Equal(t, "function_call_output", gjson.GetBytes(plan.FullReplayBody, "input.1.type").String())
			require.Equal(t, "call_1", gjson.GetBytes(plan.FullReplayBody, "input.1.call_id").String())
		})
	}
}

func TestPrepareResponsesAnthropicIngress_MergesSupportedMessagesIntoFullReplay(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"claude-sonnet-4.5","input":[{"type":"function_call_output","call_id":"call_1","output":"ok"}],"messages":[{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"hello\"}"}}]},{"role":"tool","tool_call_id":"call_1","content":"ok"}],"previous_response_id":"resp_stale"}`)

	plan, err := prepareResponsesAnthropicIngress(body)
	require.NoError(t, err)
	require.Equal(t, string(body), string(plan.PrimaryBody))
	require.True(t, plan.CanRetryWithFullReplay())
	require.Equal(t, "input+messages", plan.FullReplaySource)
	require.Equal(t, "function_call", gjson.GetBytes(plan.FullReplayBody, "input.0.type").String())
	require.Equal(t, "call_1", gjson.GetBytes(plan.FullReplayBody, "input.0.call_id").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(plan.FullReplayBody, "input.1.type").String())
	require.Equal(t, "call_1", gjson.GetBytes(plan.FullReplayBody, "input.1.call_id").String())
	require.True(t, plan.CanRetryWithFullReplayForReason(string(recoverableFailureToolContinuation)))
}

func TestPrepareResponsesAnthropicIngress_FallsBackToLegacyMessagesWhenInputMissing(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"claude-sonnet-4.5","messages":[{"role":"system","content":"repo policy"},{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"hello\"}"}}]},{"role":"tool","tool_call_id":"call_1","content":"ok"}],"previous_response_id":"resp_stale"}`)

	plan, err := prepareResponsesAnthropicIngress(body)
	require.NoError(t, err)
	require.NotEqual(t, string(body), string(plan.PrimaryBody))
	require.Empty(t, plan.FullReplayBody)
	require.Empty(t, plan.FullReplaySource)
	require.False(t, gjson.GetBytes(plan.PrimaryBody, "messages").Exists())
	require.False(t, gjson.GetBytes(plan.PrimaryBody, "previous_response_id").Exists())
	require.False(t, gjson.GetBytes(plan.PrimaryBody, "instructions").Exists())
	require.Equal(t, "system", gjson.GetBytes(plan.PrimaryBody, "input.0.role").String())
	require.Equal(t, "repo policy", gjson.GetBytes(plan.PrimaryBody, "input.0.content").String())
	require.Equal(t, "function_call", gjson.GetBytes(plan.PrimaryBody, "input.1.type").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(plan.PrimaryBody, "input.2.type").String())
}

func TestClassifyResponsesAnthropicFailure(t *testing.T) {
	t.Parallel()

	classification := classifyResponsesAnthropicFailure(http.StatusBadRequest, []byte(`{"error":{"type":"invalid_request_error","message":"previous response not found for continuation anchor"}}`))
	require.Equal(t, "previous_response_not_found", classification.Reason)
	require.True(t, classification.RetryWithFullReplay)

	classification = classifyResponsesAnthropicFailure(http.StatusBadRequest, []byte(`{"error":{"type":"invalid_request_error","message":"tool_result block missing matching tool_use block"}}`))
	require.Equal(t, "tool_continuation", classification.Reason)
	require.True(t, classification.RetryWithFullReplay)

	classification = classifyResponsesAnthropicFailure(http.StatusBadRequest, []byte(`{"error":{"type":"invalid_request_error","message":"No tool call found for function call output with call_id call_1."}}`))
	require.Equal(t, "tool_continuation", classification.Reason)
	require.True(t, classification.RetryWithFullReplay)

	classification = classifyResponsesAnthropicFailure(http.StatusBadRequest, []byte(`{"error":{"type":"invalid_request_error","message":"messages: at least one message is required"}}`))
	require.Equal(t, "empty_messages", classification.Reason)
	require.True(t, classification.RetryWithFullReplay)

	classification = classifyResponsesAnthropicFailure(http.StatusBadRequest, []byte(`{"error":{"type":"invalid_request_error","message":"messages.0.content must be non-empty"}}`))
	require.Empty(t, classification.Reason)
	require.False(t, classification.RetryWithFullReplay)

	classification = classifyResponsesAnthropicFailure(http.StatusUnprocessableEntity, []byte(`{"error":{"type":"invalid_request_error","message":"previous response not found for continuation anchor"}}`))
	require.Equal(t, "previous_response_not_found", classification.Reason)
	require.True(t, classification.RetryWithFullReplay)

	classification = classifyResponsesAnthropicFailure(http.StatusUnauthorized, []byte(`{"error":{"type":"authentication_error","message":"bad auth"}}`))
	require.Empty(t, classification.Reason)
	require.False(t, classification.RetryWithFullReplay)
}

func TestForwardAsResponses_RetriesFullReplayOnceOnRecoverableIngressFailures(t *testing.T) {
	t.Parallel()
	setGinTestMode()

	cases := []struct {
		name       string
		message    string
		responseID string
		text       string
	}{
		{
			name:       "previous_response_not_found",
			message:    "previous response not found for continuation anchor",
			responseID: "msg_replay_continuation",
			text:       "continuation replay ok",
		},
		{
			name:       "tool_continuation",
			message:    "No tool call found for function call output with call_id call_1.",
			responseID: "msg_replay_tool_continuation",
			text:       "tool continuation replay ok",
		},
		{
			name:       "empty_messages",
			message:    "messages: at least one message is required",
			responseID: "msg_replay_empty_messages",
			text:       "empty messages replay ok",
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runForwardAsResponsesFullReplayRetryTest(t, tt.message, tt.responseID, tt.text)
		})
	}
}

func runForwardAsResponsesFullReplayRetryTest(t *testing.T, upstreamMessage string, replayResponseID string, replayText string) {
	t.Helper()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"claude-sonnet-4.5","stream":false,"input":[{"type":"function_call_output","call_id":"call_1","output":"ok"}],"messages":[{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"hello\"}"}}]},{"role":"tool","tool_call_id":"call_1","content":"ok"}],"previous_response_id":"resp_anchor_1"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(string(body)))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamSequenceRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"` + upstreamMessage + `"}}`)),
			},
			{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"text/event-stream"},
					"x-request-id": []string{"rid_replay_ok"},
				},
				Body: io.NopCloser(strings.NewReader(strings.Join([]string{
					`event: message_start`,
					`data: {"type":"message_start","message":{"id":"` + replayResponseID + `","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4.5","stop_reason":"","usage":{"input_tokens":12}}}`,
					``,
					`event: content_block_start`,
					`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":"` + replayText + `"}}`,
					``,
					`event: message_delta`,
					`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":4}}`,
					``,
					`event: message_stop`,
					`data: {"type":"message_stop"}`,
					``,
				}, "\n"))),
			},
		},
	}

	svc := &GatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled:           false,
					AllowInsecureHTTP: true,
				},
			},
		},
		httpUpstream: upstream,
	}
	account := newAnthropicAPIKeyAccountForTest()
	account.Extra = nil

	result, err := svc.ForwardAsResponses(context.Background(), c, account, body, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 2, upstream.callCount)
	require.True(t, gjson.GetBytes(upstream.bodies[0], "messages").IsArray())
	require.Equal(t, int64(0), gjson.GetBytes(upstream.bodies[0], "messages.#").Int())
	require.NotContains(t, string(upstream.bodies[0]), `"tool_result"`)
	require.Equal(t, "assistant", gjson.GetBytes(upstream.bodies[1], "messages.0.role").String())
	require.Equal(t, "tool_use", gjson.GetBytes(upstream.bodies[1], "messages.0.content.0.type").String())
	require.Equal(t, "user", gjson.GetBytes(upstream.bodies[1], "messages.1.role").String())
	require.Equal(t, "tool_result", gjson.GetBytes(upstream.bodies[1], "messages.1.content.0.type").String())
	require.False(t, gjson.GetBytes(upstream.bodies[1], "messages.2").Exists())
	require.Contains(t, rec.Body.String(), `"`+replayText+`"`)
}
