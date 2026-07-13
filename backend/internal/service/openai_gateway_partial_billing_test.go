//go:build unit

package service

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIStreamingErrorBillsPartial_ExcludesFailover(t *testing.T) {
	t.Parallel()
	require.False(t, openaiStreamingErrorBillsPartial(nil))
	require.True(t, openaiStreamingErrorBillsPartial(errors.New("upstream response failed")))
	require.False(t, openaiStreamingErrorBillsPartial(&UpstreamFailoverError{
		StatusCode: 429,
	}))
}

func TestBuildOpenAIStreamingPartialFromPassthroughShape(t *testing.T) {
	t.Parallel()
	usage := &OpenAIUsage{InputTokens: 11, OutputTokens: 7}
	passthrough := &openaiStreamingResultPassthrough{
		usage:      usage,
		responseID: "resp_partial",
		imageCount: 0,
		searchCount: 1,
	}
	// Conversion path used by forwardOpenAIPassthrough on terminal stream errors.
	streamResult := &openaiStreamingResult{
		usage:       passthrough.usage,
		firstTokenMs: passthrough.firstTokenMs,
		responseID:  passthrough.responseID,
		imageCount:  passthrough.imageCount,
		searchCount: passthrough.searchCount,
	}
	resp := &http.Response{Header: http.Header{"X-Request-Id": []string{"req-1"}}}
	partial := buildOpenAIStreamingPartialForwardResult(
		resp,
		[]byte(`{"model":"gpt-5","stream":true}`),
		"gpt-5",
		"gpt-5",
		streamResult,
		2*time.Second,
	)
	require.NotNil(t, partial)
	require.Equal(t, 11, partial.Usage.InputTokens)
	require.Equal(t, 7, partial.Usage.OutputTokens)
	require.Equal(t, "resp_partial", partial.ResponseID)
	require.True(t, partial.Stream)
	require.Equal(t, 1, partial.SearchCount)
}

func TestOpenAIWSPartialForwardResultCarriesUsage(t *testing.T) {
	t.Parallel()
	usage := &OpenAIUsage{InputTokens: 3, OutputTokens: 1}
	partial := buildOpenAIWSPartialForwardResult(openAIWSPartialForwardInput{
		responseID:     "resp_ws",
		usage:          usage,
		originalModel:  "gpt-5",
		mappedModel:    "gpt-5",
		reqStream:      true,
		duration:       time.Second,
		imageCount:     0,
		clientDisc:     false,
	})
	require.NotNil(t, partial)
	require.Equal(t, 3, partial.Usage.InputTokens)
	require.Equal(t, 1, partial.Usage.OutputTokens)
	require.True(t, partial.OpenAIWSMode)
	require.True(t, partial.Stream)
}
