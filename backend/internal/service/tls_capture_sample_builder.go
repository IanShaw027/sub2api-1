package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

func firstNonEmptyHeader(header http.Header, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func nativeCaptureRequestToken(r *http.Request) string {
	if r == nil {
		return ""
	}
	if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" {
		return token
	}
	if token := strings.TrimSpace(r.Header.Get("X-TLS-Fingerprint-Token")); token != "" {
		return token
	}
	if token := strings.TrimSpace(r.Header.Get("X-API-Key")); token != "" {
		return token
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("Bearer "):])
	}
	return ""
}

func nativeCaptureRequestPlatform(r *http.Request) string {
	if r == nil {
		return ""
	}
	if platform := strings.TrimSpace(r.URL.Query().Get("platform")); platform != "" {
		return platform
	}
	if platform := strings.TrimSpace(r.Header.Get("X-TLS-Fingerprint-Platform")); platform != "" {
		return platform
	}
	path := strings.Trim(r.URL.Path, "/")
	if path == "" || path == "capture" {
		return ""
	}
	if strings.HasPrefix(path, "capture/") {
		path = strings.TrimPrefix(path, "capture/")
	}
	if before, _, ok := strings.Cut(path, "/"); ok {
		return strings.TrimSpace(before)
	}
	return strings.TrimSpace(path)
}

func nativeCaptureRequestTransport(r *http.Request) string {
	if r == nil {
		return ""
	}
	if transport := strings.TrimSpace(r.URL.Query().Get("transport")); transport != "" {
		return transport
	}
	if transport := strings.TrimSpace(firstNonEmptyHeader(r.Header, "X-TLS-Fingerprint-Transport", "X-Transport")); transport != "" {
		return transport
	}
	return ""
}

func nativeCaptureRequestSessionID(r *http.Request) string {
	if r == nil {
		return ""
	}
	return strings.TrimSpace(firstNonEmptyHeader(
		r.Header,
		"X-TLS-Fingerprint-Session-Id",
		"X-Session-Id",
		"X-Claude-Code-Session-Id",
		"OAI-Session-Id",
	))
}

func nativeCaptureNegotiatedALPN(r *http.Request) string {
	if r == nil || r.TLS == nil {
		return ""
	}
	return strings.TrimSpace(r.TLS.NegotiatedProtocol)
}

func remoteIPFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func nativeCaptureIsWebsocketRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket")
}

func nativeCaptureRequestClientType(r *http.Request) string {
	if r == nil {
		return ""
	}
	if clientType := strings.TrimSpace(firstNonEmptyHeader(r.Header, "X-TLS-Fingerprint-Client-Type", "X-Client-Type")); clientType != "" {
		return clientType
	}
	originator := strings.ToLower(strings.TrimSpace(firstNonEmptyHeader(r.Header, "Originator", "originator")))
	userAgent := strings.ToLower(strings.TrimSpace(r.Header.Get("User-Agent")))
	switch {
	case strings.Contains(originator, "codex") || strings.Contains(userAgent, "codex"):
		return "codex"
	case strings.Contains(originator, "claude") || strings.Contains(userAgent, "claude"):
		return "claude"
	default:
		return "unknown"
	}
}

func nativeCaptureRequestModel(r *http.Request) string {
	if r == nil {
		return ""
	}
	if model := strings.TrimSpace(r.URL.Query().Get("model")); model != "" {
		return model
	}
	body := nativeCaptureParsedRequestBody(r)
	if body == nil {
		return ""
	}
	switch value := body["model"].(type) {
	case string:
		return strings.TrimSpace(value)
	default:
		return ""
	}
}

func nativeCaptureRequestKind(r *http.Request) string {
	if r == nil {
		return ""
	}
	path := strings.Trim(strings.ToLower(strings.TrimSpace(r.URL.Path)), "/")
	switch {
	case strings.Contains(path, "/v1/responses"):
		return "responses"
	case strings.Contains(path, "/v1/chat/completions"):
		return "chat_completions"
	default:
		return ""
	}
}

func nativeCaptureRequestStreaming(r *http.Request) bool {
	if r == nil {
		return false
	}
	if nativeCaptureRequestGenerateFalse(r) {
		return false
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("stream")); raw != "" {
		return strings.EqualFold(raw, "true") || raw == "1"
	}
	body := nativeCaptureParsedRequestBody(r)
	if body == nil {
		return false
	}
	switch value := body["stream"].(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "true") || strings.TrimSpace(value) == "1"
	default:
		return false
	}
}

func nativeCaptureRequestResponseMode(r *http.Request) string {
	if r == nil {
		return ""
	}
	if nativeCaptureIsWebsocketRequest(r) {
		return "websocket"
	}
	if nativeCaptureRequestGenerateFalse(r) {
		return "prewarm"
	}
	if nativeCaptureRequestStreaming(r) {
		return "stream"
	}
	if strings.Contains(strings.ToLower(strings.TrimSpace(r.Header.Get("Accept"))), "text/event-stream") {
		return "sse"
	}
	return "json"
}

func nativeCaptureRequestGenerateFalse(r *http.Request) bool {
	if r == nil {
		return false
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("generate")); raw != "" {
		return strings.EqualFold(raw, "false") || raw == "0"
	}
	body := nativeCaptureParsedRequestBody(r)
	if body == nil {
		return false
	}
	switch value := body["generate"].(type) {
	case bool:
		return !value
	case string:
		trimmed := strings.TrimSpace(value)
		return strings.EqualFold(trimmed, "false") || trimmed == "0"
	default:
		return false
	}
}

func nativeCaptureRequestPath(r *http.Request) string {
	if r == nil || r.URL == nil {
		return "/"
	}
	path := strings.TrimSpace(r.URL.Path)
	if path == "" {
		return "/"
	}
	platform := nativeCaptureRequestPlatform(r)
	trimmed := strings.Trim(path, "/")
	if trimmed == "capture" {
		return "/"
	}
	if strings.HasPrefix(trimmed, "capture/") {
		rest := strings.TrimPrefix(trimmed, "capture/")
		if platform != "" {
			if rest == platform {
				return "/"
			}
			prefix := platform + "/"
			if strings.HasPrefix(rest, prefix) {
				rest = strings.TrimPrefix(rest, prefix)
			}
		}
		if rest == "" {
			return "/"
		}
		return "/" + strings.TrimLeft(rest, "/")
	}
	return path
}

func nativeCaptureRequestSequence(r *http.Request) int {
	if r == nil {
		return 0
	}
	raw := strings.TrimSpace(firstNonEmptyHeader(r.Header, "X-Request-Sequence", "x-request-sequence"))
	if raw == "" {
		return 0
	}
	var seq int
	_, _ = fmt.Sscanf(raw, "%d", &seq)
	return seq
}

func nativeCaptureStainlessMetadata(header http.Header) map[string]any {
	if header == nil {
		return nil
	}
	values := map[string]any{}
	add := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			values[key] = strings.TrimSpace(value)
		}
	}
	add("lang", header.Get("X-Stainless-Lang"))
	add("package_version", header.Get("X-Stainless-Package-Version"))
	add("os", header.Get("X-Stainless-OS"))
	add("arch", header.Get("X-Stainless-Arch"))
	add("runtime", header.Get("X-Stainless-Runtime"))
	add("runtime_version", header.Get("X-Stainless-Runtime-Version"))
	add("retry_count", header.Get("X-Stainless-Retry-Count"))
	add("timeout", header.Get("X-Stainless-Timeout"))
	add("helper_method", firstNonEmptyHeader(header, "x-stainless-helper-method", "X-Stainless-Helper-Method"))
	if len(values) == 0 {
		return nil
	}
	return values
}

func nativeCaptureHeadersSnapshot(header http.Header) map[string]any {
	if header == nil {
		return nil
	}
	out := make(map[string]any, len(header))
	for key, values := range header {
		lowerKey := strings.ToLower(strings.TrimSpace(key))
		if lowerKey == "" {
			continue
		}
		redacted := make([]string, 0, len(values))
		for _, value := range values {
			if isSensitiveKey(lowerKey) {
				redacted = append(redacted, "[REDACTED]")
				continue
			}
			redacted = append(redacted, strings.TrimSpace(value))
		}
		out[lowerKey] = redacted
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func nativeCaptureBodySummary(r *http.Request) string {
	body := nativeCaptureRawBody(r)
	if len(body) == 0 {
		return ""
	}
	summary, _, _ := sanitizeAndTrimRequestBody(body, tlsFingerprintNativeCaptureBodySummaryLimit)
	return summary
}

func nativeCaptureRawBody(r *http.Request) []byte {
	if r == nil || r.Body == nil {
		return nil
	}
	if body, ok := r.Context().Value(tlsFingerprintNativeCaptureBodyContextKey{}).([]byte); ok {
		return append([]byte(nil), body...)
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return nil
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	return raw
}

func nativeCaptureParsedRequestBody(r *http.Request) map[string]any {
	body := nativeCaptureRawBody(r)
	if len(body) == 0 {
		return nil
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil
	}
	return decoded
}
