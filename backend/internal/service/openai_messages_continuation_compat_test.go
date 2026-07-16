package service

import (
	"net/http"
	"testing"
)

func TestIsOpenAICompatPreviousResponseNotFoundAcceptsResponseIDMessage(t *testing.T) {
	body := []byte(`{"error":{"type":"not_found_error","message":"Response with id=resp_missing not found"}}`)
	if !isOpenAICompatPreviousResponseNotFound(http.StatusNotFound, "Response with id=resp_missing not found", body) {
		t.Fatal("expected response id not-found error to be recoverable")
	}
}

func TestIsGrokPreviousResponseRecoveryErrorAcceptsResponseIDMessage(t *testing.T) {
	body := []byte(`{"error":{"code":"not-found","message":"Response with id=resp_missing not found"}}`)
	if !isGrokPreviousResponseRecoveryError(http.StatusNotFound, "not-found", "Response with id=resp_missing not found", body) {
		t.Fatal("expected Grok response id not-found error to trigger full replay recovery")
	}
}
