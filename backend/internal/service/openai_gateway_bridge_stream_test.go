package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type bridgeErrAfterDataReadCloser struct {
	data []byte
	done bool
}

func (r *bridgeErrAfterDataReadCloser) Read(p []byte) (int, error) {
	if !r.done {
		r.done = true
		return copy(p, r.data), nil
	}
	return 0, errors.New("upstream stream truncated")
}

func (r *bridgeErrAfterDataReadCloser) Close() error { return nil }

func TestStreamCCResponseAsAnthropicReturnsScannerError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	resp := &http.Response{
		Body: &bridgeErrAfterDataReadCloser{data: []byte(`data: {"choices":[{"delta":{"content":"hello"}}]}` + "\n\n")},
	}

	_, _, _, err := (&GatewayService{}).streamCCResponseAsAnthropic(
		t.Context(),
		c,
		resp,
		"claude-test",
		false,
		time.Now(),
	)

	require.ErrorContains(t, err, "upstream stream truncated")
}

func TestStreamAnthropicResponseAsCCReturnsScannerError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	resp := &http.Response{
		Body: &bridgeErrAfterDataReadCloser{data: []byte(`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[],"model":"claude-test","usage":{"input_tokens":10,"output_tokens":0}}}` + "\n\n")},
	}

	_, err := (&OpenAIGatewayService{}).streamAnthropicResponseAsCC(
		t.Context(),
		c,
		resp,
		"gpt-test",
		"claude-test",
		false,
		true,
		time.Now(),
	)

	require.ErrorContains(t, err, "upstream stream truncated")
}

func TestStreamAnthropicResponseAsCCClientDisconnectDoesNotWriteJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[],"model":"claude-test","usage":{"input_tokens":10,"output_tokens":0}}}` + "\n\n")),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := (&OpenAIGatewayService{}).streamAnthropicResponseAsCC(
		ctx,
		c,
		resp,
		"gpt-test",
		"claude-test",
		true,
		true,
		time.Now(),
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.ClientDisconnect)
	require.NotContains(t, w.Body.String(), `"choices"`)
	require.NotContains(t, w.Body.String(), `"usage"`)
	require.NotContains(t, w.Body.String(), "data: [DONE]")
}

func TestOpenAIUsageFromStateIncludesAnthropicCacheDetails(t *testing.T) {
	state := apicompat.NewAnthropicToCCChunkState("gpt-test")
	state.Usage = &apicompat.AnthropicUsage{
		InputTokens:              10,
		OutputTokens:             2,
		CacheReadInputTokens:     7,
		CacheCreationInputTokens: 3,
	}

	got := openAIUsageFromState(state)

	require.Equal(t, 10, got.InputTokens)
	require.Equal(t, 2, got.OutputTokens)
	require.Equal(t, 7, got.CacheReadInputTokens)
	require.Equal(t, 3, got.CacheCreationInputTokens)
}

var _ io.ReadCloser = (*bridgeErrAfterDataReadCloser)(nil)
