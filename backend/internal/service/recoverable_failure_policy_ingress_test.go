package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifyResponsesAnthropicFailure_UsesRecoverablePolicyHelper(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		body       string
		reason     string
		retry      bool
	}{
		{
			name:       "previous_response_not_found",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"type":"invalid_request_error","message":"previous response not found for continuation anchor"}}`,
			reason:     "previous_response_not_found",
			retry:      true,
		},
		{
			name:       "invalid_anchor",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"type":"invalid_request_error","message":"missing anchor for replay"}}`,
			reason:     "invalid_anchor",
			retry:      true,
		},
		{
			name:       "invalid_continuation",
			statusCode: http.StatusUnprocessableEntity,
			body:       `{"error":{"type":"invalid_request_error","message":"stale continuation token"}}`,
			reason:     "invalid_continuation",
			retry:      true,
		},
		{
			name:       "tool_context",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"type":"invalid_request_error","message":"tool context missing"}}`,
			reason:     "tool_context",
			retry:      true,
		},
		{
			name:       "tool_continuation",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"type":"invalid_request_error","message":"tool_result block missing matching tool_use block"}}`,
			reason:     "tool_continuation",
			retry:      true,
		},
		{
			name:       "non_invalid_request_error",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"type":"authentication_error","message":"previous response not found for continuation anchor"}}`,
			reason:     "",
			retry:      false,
		},
		{
			name:       "server_error_status",
			statusCode: http.StatusInternalServerError,
			body:       `{"error":{"type":"invalid_request_error","message":"tool context missing"}}`,
			reason:     "",
			retry:      false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			classification := classifyResponsesAnthropicFailure(tt.statusCode, []byte(tt.body))
			require.Equal(t, tt.reason, classification.Reason)
			require.Equal(t, tt.retry, classification.RetryWithFullReplay)
		})
	}
}
