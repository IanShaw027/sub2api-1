//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractUpstreamErrorMessageTopLevelJSONString(t *testing.T) {
	t.Parallel()

	body := []byte(`"Multi Agent requests are not allowed on chat completions"`)
	require.Equal(t, "Multi Agent requests are not allowed on chat completions", ExtractUpstreamErrorMessage(body))
}

func TestExtractUpstreamErrorMessageXAIStringErrorField(t *testing.T) {
	t.Parallel()

	body := []byte(`{"code":"invalid-argument","error":"Empty content block"}`)
	require.Equal(t, "Empty content block", ExtractUpstreamErrorMessage(body))
}

func TestExtractUpstreamErrorMessageNestedErrorMessage(t *testing.T) {
	t.Parallel()

	body := []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"messages: text content blocks must be non-empty"}}`)
	require.Equal(t, "messages: text content blocks must be non-empty", ExtractUpstreamErrorMessage(body))
}

func TestExtractUpstreamErrorMessagePlainTextBody(t *testing.T) {
	t.Parallel()

	body := []byte("Multi Agent requests are not allowed on chat completions")
	require.Equal(t, "Multi Agent requests are not allowed on chat completions", ExtractUpstreamErrorMessage(body))
}

func TestExtractUpstreamErrorMessageDoesNotExposeHTMLGatewayBody(t *testing.T) {
	t.Parallel()

	body := []byte("<!doctype html><html><title>502 Bad Gateway</title><body>cloudflare</body></html>")
	require.Empty(t, ExtractUpstreamErrorMessage(body))
}
