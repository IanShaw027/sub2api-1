package handler

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

func newCompactBodySignalTestContext(t *testing.T, path string, body []byte) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func TestNormalizeOpenAIResponsesCompactRequest_RemoteV2StaysOnResponses(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	body := []byte(`{
		"model":"gpt-5.6-sol",
		"stream":true,
		"store":true,
		"prompt_cache_key":"pck-signal-1",
		"reasoning":{"effort":"max","context":"all_turns"},
		"input":[
			{"type":"message","role":"user","content":"hello"},
			{"type":"compaction_trigger"}
		]
	}`)
	c := newCompactBodySignalTestContext(t, "/v1/responses", body)
	c.Request.Header.Set("x-codex-beta-features", "responses_websockets_v2, remote_compaction_v2, another_feature")

	normalized, ok := h.normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), body)
	require.True(t, ok)

	require.Equal(t, "/v1/responses", c.Request.URL.Path)
	require.False(t, isOpenAIRemoteCompactPath(c))
	require.Equal(t, body, normalized)
	require.True(t, gjson.GetBytes(normalized, "stream").Bool())
	require.True(t, gjson.GetBytes(normalized, "store").Bool())
	require.Equal(t, "pck-signal-1", gjson.GetBytes(normalized, "prompt_cache_key").String())
	require.Equal(t, "max", gjson.GetBytes(normalized, "reasoning.effort").String())
	require.Equal(t, "all_turns", gjson.GetBytes(normalized, "reasoning.context").String())

	reqStream, streamOK := parseOpenAICompatibleStream(normalized)
	require.True(t, streamOK)
	require.True(t, reqStream)

	_, seedExists := c.Get(service.OpenAICompactSessionSeedKeyForTest())
	require.False(t, seedExists)
	_, streamMarkerExists := c.Get(service.OpenAICompactClientStreamKeyForTest())
	require.False(t, streamMarkerExists)
}

func TestNormalizeOpenAIResponsesCompactRequest_RemoteV2PathAliasesStayOnResponses(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	body := []byte(`{"model":"gpt-5.6-sol","stream":true,"input":[{"type":"compaction_trigger"}]}`)
	for _, path := range []string{"/v1/responses/", "/backend-api/codex/responses"} {
		t.Run(path, func(t *testing.T) {
			c := newCompactBodySignalTestContext(t, path, body)
			c.Request.Header.Set("x-codex-beta-features", "remote_compaction_v2")

			normalized, ok := h.normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), body)
			require.True(t, ok)
			require.Equal(t, path, c.Request.URL.Path)
			require.Equal(t, body, normalized)
		})
	}
}

func TestNormalizeOpenAIResponsesCompactRequest_BodySignalTrailingSlashPromoted(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	body := []byte(`{"model":"gpt-5.5","input":[{"type":"compaction_trigger"}]}`)
	c := newCompactBodySignalTestContext(t, "/v1/responses/", body)

	_, ok := h.normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), body)
	require.True(t, ok)
	require.Equal(t, "/v1/responses/compact", c.Request.URL.Path)
}

func TestNormalizeOpenAIResponsesCompactRequest_CodexDirectAliasPromoted(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	body := []byte(`{"model":"gpt-5.5","input":[{"type":"compaction_trigger"}]}`)
	c := newCompactBodySignalTestContext(t, "/backend-api/codex/responses", body)

	_, ok := h.normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), body)
	require.True(t, ok)
	require.Equal(t, "/backend-api/codex/responses/compact", c.Request.URL.Path)
}

func TestNormalizeOpenAIResponsesCompactRequest_NonRemoteV2BodySignalPromoted(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	tests := []struct {
		name       string
		body       []byte
		betaHeader string
		wantMarked bool
	}{
		{
			name:       "no_header",
			body:       []byte(`{"model":"gpt-5.5","stream":true,"input":[{"type":"compaction_trigger"}]}`),
			wantMarked: true,
		},
		{
			name:       "unrelated_header",
			body:       []byte(`{"model":"gpt-5.5","stream":true,"input":[{"type":"compaction_trigger"}]}`),
			betaHeader: "responses_websockets_v2",
			wantMarked: true,
		},
		{
			name:       "wrong_case_header",
			body:       []byte(`{"model":"gpt-5.5","stream":true,"input":[{"type":"compaction_trigger"}]}`),
			betaHeader: "REMOTE_COMPACTION_V2",
			wantMarked: true,
		},
		{
			name:       "stream_false",
			body:       []byte(`{"model":"gpt-5.5","stream":false,"input":[{"type":"compaction_trigger"}]}`),
			betaHeader: "remote_compaction_v2",
		},
		{
			name:       "stream_absent",
			body:       []byte(`{"model":"gpt-5.5","input":[{"type":"compaction_trigger"}]}`),
			betaHeader: "remote_compaction_v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCompactBodySignalTestContext(t, "/v1/responses", tt.body)
			if tt.betaHeader != "" {
				c.Request.Header.Set("x-codex-beta-features", tt.betaHeader)
			}

			normalized, ok := h.normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), tt.body)
			require.True(t, ok)
			require.Equal(t, "/v1/responses/compact", c.Request.URL.Path)
			require.False(t, gjson.GetBytes(normalized, "stream").Exists())

			marked, exists := c.Get(service.OpenAICompactClientStreamKeyForTest())
			require.Equal(t, tt.wantMarked, exists)
			if tt.wantMarked {
				require.Equal(t, true, marked)
			}
		})
	}
}

func TestNormalizeOpenAIResponsesCompactRequest_NoTriggerUntouched(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	body := []byte(`{"model":"gpt-5.5","stream":true,"input":[{"type":"message","role":"user","content":"hello"}]}`)
	c := newCompactBodySignalTestContext(t, "/v1/responses", body)

	normalized, ok := h.normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), body)
	require.True(t, ok)
	require.Equal(t, "/v1/responses", c.Request.URL.Path)
	require.False(t, isOpenAIRemoteCompactPath(c))
	require.Equal(t, body, normalized)
	require.True(t, gjson.GetBytes(normalized, "stream").Bool())
}

func TestNormalizeOpenAIResponsesCompactRequest_PathBasedNoDoubleSuffix(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	body := []byte(`{"model":"gpt-5.5","stream":true,"store":true,"input":[{"type":"message","role":"user","content":"hello"}]}`)
	c := newCompactBodySignalTestContext(t, "/v1/responses/compact", body)
	c.Request.Header.Set("x-codex-beta-features", "remote_compaction_v2")

	normalized, ok := h.normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), body)
	require.True(t, ok)
	require.Equal(t, "/v1/responses/compact", c.Request.URL.Path)
	require.False(t, gjson.GetBytes(normalized, "stream").Exists())
	require.False(t, gjson.GetBytes(normalized, "store").Exists())
}

func TestNormalizeOpenAIResponsesCompactRequest_SubpathNotPromoted(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	body := []byte(`{"model":"gpt-5.5","input":[{"type":"compaction_trigger"}]}`)
	c := newCompactBodySignalTestContext(t, "/v1/responses/resp_123/cancel", body)

	normalized, ok := h.normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), body)
	require.True(t, ok)
	require.Equal(t, "/v1/responses/resp_123/cancel", c.Request.URL.Path)
	require.Equal(t, body, normalized)
}

// path-based compact（Codex v1 unary 协议）即使 body 带 stream:true 也不标记，
// 保持 JSON 写回行为不变。
func TestNormalizeOpenAIResponsesCompactRequest_PathBasedStreamTrueNotMarked(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	body := []byte(`{"model":"gpt-5.5","stream":true,"input":[{"type":"message","role":"user","content":"hello"}]}`)
	c := newCompactBodySignalTestContext(t, "/v1/responses/compact", body)

	_, ok := h.normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), body)
	require.True(t, ok)
	_, exists := c.Get(service.OpenAICompactClientStreamKeyForTest())
	require.False(t, exists)
}

// Production-entry regression: the Responses handler itself must invoke compact
// promotion before validating request fields. Helper-only coverage would miss a
// dropped call site during a merge.
func TestOpenAIResponses_BodySignalPromotesAtHandlerEntry(t *testing.T) {
	for name, path := range map[string]string{
		"public responses": "/v1/responses",
		"codex backend":    "/backend-api/codex/responses",
	} {
		t.Run(name, func(t *testing.T) {
			body := []byte(`{"stream":true,"prompt_cache_key":"pck-handler-entry","input":[{"type":"compaction_trigger"}]}`)
			c := newCompactBodySignalTestContext(t, path, body)

			groupID := int64(2)
			c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
				ID:      101,
				GroupID: &groupID,
				User:    &service.User{ID: 1},
			})
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1, Concurrency: 1})

			h := newOpenAIHandlerForPreviousResponseIDValidation(t, nil)
			h.cfg = &config.Config{}
			h.cfg.Gateway.StreamKeepaliveInterval = 1
			h.Responses(c)

			require.Equal(t, http.StatusBadRequest, c.Writer.Status())
			require.Equal(t, path+"/compact", c.Request.URL.Path)
			marked, exists := c.Get(service.OpenAICompactClientStreamKeyForTest())
			require.True(t, exists)
			require.Equal(t, true, marked)
			_, keepaliveStarted := c.Get("openai_compact_sse_keepalive")
			require.True(t, keepaliveStarted, "handler entry must wire the compact keepalive")
		})
	}
}

func newCompactKeepaliveHandlerTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	service.MarkOpenAICompactClientStream(c)
	return c, rec
}

func waitForCompactKeepaliveCommit(t *testing.T, c *gin.Context) {
	t.Helper()
	require.Eventually(t, c.Writer.Written, time.Second, time.Millisecond)
}

func TestErrorResponse_CompactKeepalivePreservesPreAndPostCommitSemantics(t *testing.T) {
	t.Run("before first heartbeat keeps JSON status", func(t *testing.T) {
		c, rec := newCompactKeepaliveHandlerTestContext(t)
		stop := service.StartOpenAICompactSSEKeepalive(c, time.Hour)
		defer stop()

		(&OpenAIGatewayHandler{}).errorResponse(c, http.StatusForbidden, "permission_error", "blocked")

		require.Equal(t, http.StatusForbidden, rec.Code)
		require.Equal(t, "permission_error", gjson.GetBytes(rec.Body.Bytes(), "error.type").String())
		require.NotContains(t, rec.Body.String(), "event: response.failed")
		_, hasStreamErr := service.GetOpsStreamError(c)
		require.False(t, hasStreamErr)
	})

	t.Run("after heartbeat terminates SSE in band", func(t *testing.T) {
		c, rec := newCompactKeepaliveHandlerTestContext(t)
		stop := service.StartOpenAICompactSSEKeepalive(c, time.Millisecond)
		defer stop()
		waitForCompactKeepaliveCommit(t, c)

		(&OpenAIGatewayHandler{}).errorResponse(c, http.StatusForbidden, "permission_error", "blocked")

		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), ": keepalive\n\n")
		require.Contains(t, rec.Body.String(), "event: response.failed\n")
		require.Contains(t, rec.Body.String(), `"message":"blocked"`)
		streamErr, hasStreamErr := service.GetOpsStreamError(c)
		require.True(t, hasStreamErr)
		require.Equal(t, "permission_error", streamErr.ErrType)
		require.Equal(t, http.StatusForbidden, streamErr.IntendedStatus)
	})
}

func TestHandleStreamingAwareError_CompactHeartbeatMarksSemanticFailure(t *testing.T) {
	c, rec := newCompactKeepaliveHandlerTestContext(t)
	stop := service.StartOpenAICompactSSEKeepalive(c, time.Millisecond)
	defer stop()
	waitForCompactKeepaliveCommit(t, c)

	(&OpenAIGatewayHandler{}).handleStreamingAwareError(c, http.StatusTooManyRequests, "rate_limit_error", "retry later", false)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "event: response.failed\n")
	streamErr, ok := service.GetOpsStreamError(c)
	require.True(t, ok)
	require.Equal(t, "rate_limit_error", streamErr.ErrType)
	require.Equal(t, http.StatusTooManyRequests, streamErr.IntendedStatus)
}

func TestOpenAIForwardErrorAlreadyCommunicated_IgnoresCompactHeartbeatBytes(t *testing.T) {
	c, _ := newCompactKeepaliveHandlerTestContext(t)
	stop := service.StartOpenAICompactSSEKeepalive(c, time.Millisecond)
	defer stop()
	before := service.OpenAICompactKeepaliveAdjustedWrittenSize(c)
	waitForCompactKeepaliveCommit(t, c)

	forwardErr := errors.New("upstream response failed: policy rejection")
	require.False(t, openAIForwardErrorAlreadyCommunicated(c, before, forwardErr), "heartbeat bytes are not an upstream response")

	_, err := c.Writer.WriteString("event: response.failed\n\n")
	require.NoError(t, err)
	require.True(t, openAIForwardErrorAlreadyCommunicated(c, before, forwardErr), "real response bytes must still suppress a duplicate fallback")
	require.True(t, strings.Contains(c.Writer.Header().Get("Content-Type"), "text/event-stream"))
}
