//go:build unit

package service

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifyClientVisibleUpstreamError_ContextWindow(t *testing.T) {
	vis, ok := ClassifyClientVisibleUpstreamError("OpenAI upstream SSE context window exceeded", nil)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, vis.StatusCode)
	require.Equal(t, "invalid_request_error", vis.ErrorType)
	require.Equal(t, openAIContextWindowClientMessage, vis.Message)

	vis, ok = ClassifyClientVisibleUpstreamError(
		"Your input exceeds the context window of this model. Please adjust your input and try again.",
		[]byte(`{"error":{"code":"context_length_exceeded","message":"Your input exceeds the context window of this model. Please adjust your input and try again."}}`),
	)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, vis.StatusCode)
	require.Equal(t, "invalid_request_error", vis.ErrorType)
	require.Contains(t, strings.ToLower(vis.Message), "context window")
}

func TestClassifyClientVisibleUpstreamError_UsageLimitNotContext(t *testing.T) {
	body := []byte(`{"error":{"message":"The usage limit has been reached","plan_type":"k12"}}`)
	vis, ok := ClassifyClientVisibleUpstreamError("The usage limit has been reached", body)
	require.True(t, ok)
	require.Equal(t, http.StatusTooManyRequests, vis.StatusCode)
	require.Equal(t, "rate_limit_error", vis.ErrorType)
	require.Equal(t, openAIBillingLimitClientMessage(), vis.Message)
}

func TestClassifyClientVisibleUpstreamError_DeactivatedWorkspace(t *testing.T) {
	body := []byte(`{"detail":{"code":"deactivated_workspace"}}`)
	vis, ok := ClassifyClientVisibleUpstreamError("Upstream compact response failed", body)
	require.True(t, ok)
	require.Equal(t, http.StatusBadGateway, vis.StatusCode)
	require.Equal(t, "upstream_error", vis.ErrorType)
	require.Equal(t, openAIUpstreamAccessUnavailableClientMessage, vis.Message)
}

func TestClassifyClientVisibleUpstreamError_AccessStateDetailsAreRedacted(t *testing.T) {
	for _, body := range []string{
		`{"error":{"message":"Your account is deactivated"}}`,
		`{"error":{"message":"The organization is disabled"}}`,
		`{"detail":{"message":"This workspace is suspended"}}`,
	} {
		vis, ok := ClassifyClientVisibleUpstreamError("", []byte(body))
		require.True(t, ok)
		require.Equal(t, http.StatusBadGateway, vis.StatusCode)
		require.Equal(t, "upstream_error", vis.ErrorType)
		require.Equal(t, openAIUpstreamAccessUnavailableClientMessage, vis.Message)
		require.NotContains(t, strings.ToLower(vis.Message), "deactiv")
		require.NotContains(t, strings.ToLower(vis.Message), "disab")
		require.NotContains(t, strings.ToLower(vis.Message), "suspend")
	}
}

func TestPreferClientVisibleUpstreamError_OpsDetailTransportFallback(t *testing.T) {
	vis, ok := PreferClientVisibleUpstreamError(
		nil,
		"upstream_transport_error",
		"Upstream transport error",
		"OpenAI upstream SSE context window exceeded",
	)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, vis.StatusCode)
	require.Equal(t, "invalid_request_error", vis.ErrorType)
}

func TestClassifyClientVisibleUpstreamError_BusinessProtocolSignals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		message    string
		body       string
		wantOK     bool
		wantStatus int
		wantType   string
		wantMsg    string
		msgContain string
	}{
		{
			name:       "multi agent chat completions rejection",
			message:    "",
			body:       `"Multi Agent requests are not allowed on chat completions"`,
			wantOK:     true,
			wantStatus: http.StatusBadRequest,
			wantType:   "invalid_request_error",
			wantMsg:    "Multi Agent requests are not allowed on chat completions",
		},
		{
			name:       "group deleted code",
			message:    "Upstream request failed",
			body:       `{"error":{"type":"permission_error","code":"GROUP_DELETED","message":"API Key 所属分组已删除"}}`,
			wantOK:     true,
			wantStatus: http.StatusForbidden,
			wantType:   "permission_error",
			wantMsg:    "API Key 所属分组已删除",
		},
		{
			name:       "group deleted chinese message without code",
			message:    "API Key 所属分组已删除",
			body:       "",
			wantOK:     true,
			wantStatus: http.StatusForbidden,
			wantType:   "permission_error",
			wantMsg:    "API Key 所属分组已删除",
		},
		{
			name:       "image generation disabled for group",
			message:    "Image generation is not enabled for this group",
			body:       "",
			wantOK:     true,
			wantStatus: http.StatusForbidden,
			wantType:   "permission_error",
			wantMsg:    "Image generation is not enabled for this group",
		},
		{
			name:       "short policy block",
			message:    "Your request was blocked",
			body:       "",
			wantOK:     true,
			wantStatus: http.StatusForbidden,
			wantType:   "invalid_request_error",
			wantMsg:    "Your request was blocked",
		},
		{
			name:       "no account available retry later",
			message:    "no account is available, please try again later",
			body:       "",
			wantOK:     true,
			wantStatus: http.StatusServiceUnavailable,
			wantType:   "upstream_error",
			wantMsg:    "no account is available, please try again later",
		},
		{
			name:       "grok credits exhausted",
			message:    "",
			body:       `{"error":{"message":"You have run out of credits or need a Grok subscription. Add credits at grok.com"}}`,
			wantOK:     true,
			wantStatus: http.StatusPaymentRequired,
			wantType:   "upstream_error",
			msgContain: "run out of credits",
		},
		{
			name:       "model not supported by configured accounts",
			message:    `Model "gpt-missing" is not supported by any configured account in this group`,
			body:       "",
			wantOK:     true,
			wantStatus: http.StatusNotFound,
			wantType:   "model_not_found",
			msgContain: "not supported by any configured account",
		},
		{
			name:       "model_not_found code",
			message:    "Upstream request failed",
			body:       `{"error":{"code":"model_not_found","message":"The model 'gpt-missing' does not exist"}}`,
			wantOK:     true,
			wantStatus: http.StatusNotFound,
			wantType:   "model_not_found",
			msgContain: "does not exist",
		},
		{
			name:       "image model not on responses endpoint",
			message:    "model grok-imagine-image is an image model and is not available on the Responses endpoint; use /v1/images/generations instead",
			body:       "",
			wantOK:     true,
			wantStatus: http.StatusBadRequest,
			wantType:   "invalid_request_error",
			msgContain: "not available on the Responses endpoint",
		},
		{
			name:       "aspect_ratio validation stays 400",
			message:    "",
			body:       "Failed to deserialize the JSON body into the target type: aspect_ratio: unknown variant `5:4`, expected one of `1:1`, `16:9`, `auto`",
			wantOK:     true,
			wantStatus: http.StatusBadRequest,
			wantType:   "invalid_request_error",
			msgContain: "aspect_ratio: unknown variant `5:4`",
		},
		{
			name:    "incorrect api key stays opaque",
			message: "Incorrect API key provided: sk-abc",
			body:    `{"error":{"message":"Incorrect API key provided: sk-abc","type":"invalid_request_error","code":"invalid_api_key"}}`,
			wantOK:  false,
		},
		{
			name:    "cloudflare 1010 html stays opaque",
			message: "Upstream request failed",
			body:    `<!DOCTYPE html><html><body>Error code 1010. Attention Required! Cloudflare</body></html>`,
			wantOK:  false,
		},
		{
			name:    "html 502 gateway page stays opaque",
			message: "bad gateway",
			body:    `<html><head><title>502 Bad Gateway</title></head><body>Bad gateway</body></html>`,
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			vis, ok := ClassifyClientVisibleUpstreamError(tt.message, []byte(tt.body))
			require.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				return
			}
			require.Equal(t, tt.wantStatus, vis.StatusCode)
			require.Equal(t, tt.wantType, vis.ErrorType)
			if tt.wantMsg != "" {
				require.Equal(t, tt.wantMsg, vis.Message)
			}
			if tt.msgContain != "" {
				require.Contains(t, vis.Message, tt.msgContain)
			}
		})
	}
}
