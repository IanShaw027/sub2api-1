package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponsesToAnthropicRequest_ReturnsErrorForMalformedFunctionCallArguments(t *testing.T) {
	req := &ResponsesRequest{
		Model: "gpt-5.4",
		Input: json.RawMessage(`[
			{"type":"function_call","call_id":"call_123","name":"lookup","arguments":"{\"city\":"}
		]`),
		MaxOutputTokens: intPtr(256),
	}

	_, err := ResponsesToAnthropicRequest(req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid function_call arguments")
}
