package service

import "testing"

func TestOpenAIStreamFailedEventShouldFailover_ContextWindow(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		message string
	}{
		{
			name: "context_length_exceeded code",
			payload: `{"type":"response.failed","response":{"error":{"code":"context_length_exceeded",` +
				`"message":"This model's maximum context length is 200000 tokens, however you requested 250000."}}}`,
			message: "",
		},
		{
			name: "your input exceeds the context window",
			payload: `{"type":"response.failed","response":{"error":{"code":"invalid_request_error",` +
				`"message":"Your input exceeds the context window of this model. Please adjust your input and try again."}}}`,
			message: "",
		},
		{
			name: "model_context_window_exceeded code",
			payload: `{"type":"response.failed","response":{"error":{"code":"model_context_window_exceeded",` +
				`"message":"context window exceeded"}}}`,
			message: "",
		},
		{
			name:    "raw message only",
			payload: `{"type":"response.failed","response":{"error":{}}}`,
			message: "Your input exceeds the context window of this model. Please adjust your input and try again.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if openAIStreamFailedEventShouldFailover([]byte(tc.payload), tc.message) {
				t.Fatalf("expected context-window error to be non-retryable, got shouldFailover=true")
			}
		})
	}
}

func TestOpenAIStreamFailedEventShouldFailover_RetryablesUnchanged(t *testing.T) {
	payload := `{"type":"response.failed","response":{"error":{"code":"server_error","message":"transient upstream failure"}}}`
	if !openAIStreamFailedEventShouldFailover([]byte(payload), "") {
		t.Fatal("expected generic server_error to remain failover-eligible")
	}
}
