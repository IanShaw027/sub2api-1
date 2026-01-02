package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseGatewayRequest(t *testing.T) {
	body := []byte(`{"model":"claude-3-7-sonnet","stream":true,"metadata":{"user_id":"session_123e4567-e89b-12d3-a456-426614174000"},"system":[{"type":"text","text":"hello","cache_control":{"type":"ephemeral"}}],"messages":[{"content":"hi"}]}`)
	parsed, err := ParseGatewayRequest(body)
	require.NoError(t, err)
	require.Equal(t, "claude-3-7-sonnet", parsed.Model)
	require.True(t, parsed.Stream)
	require.Equal(t, "session_123e4567-e89b-12d3-a456-426614174000", parsed.MetadataUserID)
	require.True(t, parsed.HasSystem)
	require.NotNil(t, parsed.System)
	require.Len(t, parsed.Messages, 1)
}

func TestParseGatewayRequest_SystemNull(t *testing.T) {
	body := []byte(`{"model":"claude-3","system":null}`)
	parsed, err := ParseGatewayRequest(body)
	require.NoError(t, err)
	// 显式传入 system:null 也应视为“字段已存在”，避免默认 system 被注入。
	require.True(t, parsed.HasSystem)
	require.Nil(t, parsed.System)
}

func TestParseGatewayRequest_InvalidModelType(t *testing.T) {
	body := []byte(`{"model":123}`)
	_, err := ParseGatewayRequest(body)
	require.Error(t, err)
}

func TestParseGatewayRequest_InvalidStreamType(t *testing.T) {
	body := []byte(`{"stream":"true"}`)
	_, err := ParseGatewayRequest(body)
	require.Error(t, err)
}

func TestFilterThinkingBlocks(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		shouldFilter bool
		expectError  bool
	}{
		{
			name: "filters thinking blocks",
			input: `{"model":"claude-3-5-sonnet-20241022","messages":[{"role":"user","content":[{"type":"text","text":"Hello"},{"type":"thinking","thinking":"internal","signature":"invalid"},{"type":"text","text":"World"}]}]}`,
			shouldFilter: true,
		},
		{
			name: "handles no thinking blocks",
			input: `{"model":"claude-3-5-sonnet-20241022","messages":[{"role":"user","content":[{"type":"text","text":"Hello"}]}]}`,
			shouldFilter: false,
		},
		{
			name: "handles invalid JSON gracefully",
			input: `{invalid json`,
			shouldFilter: false,
			expectError: true,
		},
		{
			name: "handles multiple messages with thinking blocks",
			input: `{"messages":[{"role":"user","content":[{"type":"text","text":"A"}]},{"role":"assistant","content":[{"type":"thinking","thinking":"think"},{"type":"text","text":"B"}]}]}`,
			shouldFilter: true,
		},
		{
			name: "handles empty messages array",
			input: `{"messages":[]}`,
			shouldFilter: false,
		},
		{
			name: "handles missing messages field",
			input: `{"model":"claude-3"}`,
			shouldFilter: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterThinkingBlocks([]byte(tt.input))

			if tt.expectError {
				// For invalid JSON, should return original
				require.Equal(t, tt.input, string(result))
				return
			}

			if tt.shouldFilter {
				// Verify thinking blocks removed
				require.NotContains(t, string(result), `"type":"thinking"`)
			}

			// Verify valid JSON returned (unless input was invalid)
			var parsed map[string]any
			err := json.Unmarshal(result, &parsed)
			require.NoError(t, err)
		})
	}
}
