package service

import "testing"

func TestIsUpstreamModelUnavailableForFallback(t *testing.T) {
	t.Run("openai unsupported model triggers fallback", func(t *testing.T) {
		body := []byte(`{"error":{"message":"The 'codex-mini-latest' model is not supported when using Codex with a ChatGPT account."}}`)
		if !isUpstreamModelUnavailableForFallback(400, body) {
			t.Fatal("expected unsupported model error to trigger fallback")
		}
	})

	t.Run("generic previous response missing does not trigger fallback", func(t *testing.T) {
		body := []byte(`{"error":{"message":"previous response does not exist"}}`)
		if isUpstreamModelUnavailableForFallback(404, body) {
			t.Fatal("expected non-model missing-resource error to skip fallback")
		}
	})

	t.Run("empty 404 body does not trigger fallback", func(t *testing.T) {
		if isUpstreamModelUnavailableForFallback(404, nil) {
			t.Fatal("expected empty 404 body to skip fallback")
		}
	})
}
