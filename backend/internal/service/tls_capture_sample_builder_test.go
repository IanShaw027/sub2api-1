//go:build unit

package service

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildTLSCaptureHTTP1SubmitRequestCapturesMetadata(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://collector.example/capture/openai/v1/responses?stream=true", strings.NewReader(`{"model":"gpt-5.4","stream":true}`))
	req.RemoteAddr = "127.0.0.1:43210"
	req.TLS = &tlsConnectionStateHTTP1
	req.Header.Set("Authorization", "Bearer capture-token")
	req.Header.Set("User-Agent", "codex_exec/0.140.0")
	req.Header.Set("Originator", "codex_exec")
	req.Header.Set("X-Claude-Code-Session-Id", "session-123")
	req.Header.Set("X-Client-Request-Id", "req-123")
	req.Header.Set("X-Stainless-Lang", "js")
	req.Header.Set("Accept", "text/event-stream")

	captured := buildTLSCaptureHTTP1SubmitRequest(req, []byte{0x16, 0x03, 0x01, 0x00, 0x00})

	require.Equal(t, "capture-token", captured.Token)
	require.Equal(t, "openai", captured.Platform)
	require.Equal(t, "session-123", captured.SessionID)
	require.Equal(t, "127.0.0.1", captured.ClientIP)
	require.Equal(t, "http/1.1", captured.ALPNNegotiated)
	require.Equal(t, "codex_exec/0.140.0", captured.UserAgent)
	require.Equal(t, "codex_exec", captured.Originator)
	require.Equal(t, "/v1/responses", captured.RequestPath)
	require.Equal(t, http.MethodPost, captured.HTTPMethod)
	require.Equal(t, "responses", captured.RequestKind)
	require.True(t, captured.Streaming)
	require.Equal(t, "stream", captured.ResponseMode)
	require.Equal(t, "gpt-5.4", captured.Model)
	require.Equal(t, "codex", captured.ClientType)
	require.Equal(t, "req-123", captured.EventID)
	require.Equal(t, "js", captured.StainlessMetadata["lang"])
	require.Equal(t, []byte{0x16, 0x03, 0x01, 0x00, 0x00}, captured.ClientHello)
	require.JSONEq(t, `{"model":"gpt-5.4","stream":true}`, captured.RawPayload)
}

func TestWriteNativeCaptureJSONSuccessIncludesAcceptedAndHash(t *testing.T) {
	rec := httptest.NewRecorder()

	writeNativeCaptureJSONSuccess(rec, &TLSFingerprintCaptureSubmitResult{
		Accepted:        true,
		FingerprintHash: "hash-123",
	})

	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	require.Contains(t, string(body), "\"status\":\"completed\"")
	require.Contains(t, string(body), "\"accepted\":true")
	require.Contains(t, string(body), "\"fingerprint_hash\":\"hash-123\"")
}

func TestWriteNativeCaptureSSEResponseWritesMinimalSequence(t *testing.T) {
	rec := httptest.NewRecorder()

	writeNativeCaptureSSEResponse(rec)

	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")
	require.Contains(t, string(body), "event: response.created")
	require.Contains(t, string(body), "event: response.completed")
}

func TestBuildTLSCaptureHTTP1SubmitRequestMarksGenerateFalseAsPrewarm(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://collector.example/capture/openai/v1/responses", strings.NewReader(`{"model":"gpt-5.4","generate":false}`))
	req.RemoteAddr = "127.0.0.1:43210"
	req.TLS = &tlsConnectionStateHTTP1
	req.Header.Set("Authorization", "Bearer capture-token")
	req.Header.Set("User-Agent", "codex_exec/0.140.0")
	req.Header.Set("Originator", "codex_exec")
	req.Header.Set("Accept", "text/event-stream")

	captured := buildTLSCaptureHTTP1SubmitRequest(req, []byte{0x16, 0x03, 0x01, 0x00, 0x00})

	require.Equal(t, "prewarm", captured.ResponseMode)
	require.False(t, captured.Streaming)
	require.True(t, nativeCaptureShouldReturnStream(captured))
}

var tlsConnectionStateHTTP1 = tls.ConnectionState{
	NegotiatedProtocol: "http/1.1",
}
