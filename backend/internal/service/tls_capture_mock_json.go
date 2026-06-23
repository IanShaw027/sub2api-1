package service

import (
	"encoding/json"
	"net/http"
)

func writeNativeCaptureJSONSuccess(w http.ResponseWriter, result *TLSFingerprintCaptureSubmitResult) {
	if w == nil {
		return
	}
	body := map[string]any{
		"id":     "resp_capture_mock",
		"object": "response",
		"status": "completed",
	}
	if result != nil {
		body["accepted"] = result.Accepted
		if result.IgnoredReason != "" {
			body["ignored_reason"] = result.IgnoredReason
		}
		if result.FingerprintHash != "" {
			body["fingerprint_hash"] = result.FingerprintHash
		}
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		http.Error(w, "capture response encode failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
}
