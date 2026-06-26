package service

import (
	"io"
	"net/http"
	"strings"
)

func nativeCaptureShouldReturnStream(req TLSFingerprintCaptureNativeSubmitRequest) bool {
	if req.Streaming {
		return true
	}
	mode := strings.TrimSpace(req.ResponseMode)
	return strings.EqualFold(mode, "stream") ||
		strings.EqualFold(mode, "sse") ||
		strings.EqualFold(mode, "prewarm")
}

func writeNativeCaptureSSEResponse(w http.ResponseWriter) {
	if w == nil {
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "event: response.created\ndata: {\"id\":\"resp_capture_mock\",\"status\":\"in_progress\"}\n\n")
	_, _ = io.WriteString(w, "event: response.completed\ndata: {\"id\":\"resp_capture_mock\",\"status\":\"completed\"}\n\n")
}
