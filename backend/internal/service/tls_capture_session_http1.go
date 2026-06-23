package service

import (
	"net/http"
	"strings"
)

func (l *TLSCaptureListener) handleCapture(w http.ResponseWriter, r *http.Request) {
	if r == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	raw := l.takeRawClientHello(r.Context())
	if len(raw) == 0 {
		http.Error(w, "client hello was not captured", http.StatusBadRequest)
		return
	}
	if nativeCaptureIsWebsocketRequest(r) {
		l.handleCaptureWSHTTP1(w, r, raw)
		return
	}
	req := buildTLSCaptureHTTP1SubmitRequest(r, raw)
	result, err := l.cfg.Service.SubmitNativeCapture(r.Context(), req)
	if err != nil {
		http.Error(w, "capture submission failed", http.StatusBadRequest)
		return
	}
	if nativeCaptureShouldReturnStream(req) {
		writeNativeCaptureSSEResponse(w)
		return
	}
	writeNativeCaptureJSONSuccess(w, result)
}

func buildTLSCaptureHTTP1SubmitRequest(r *http.Request, raw []byte) TLSFingerprintCaptureNativeSubmitRequest {
	return TLSFingerprintCaptureNativeSubmitRequest{
		Token:             nativeCaptureRequestToken(r),
		Platform:          nativeCaptureRequestPlatform(r),
		Transport:         nativeCaptureRequestTransport(r),
		SessionID:         nativeCaptureRequestSessionID(r),
		ClientIP:          remoteIPFromRequest(r),
		ALPNNegotiated:    nativeCaptureNegotiatedALPN(r),
		UserAgent:         strings.TrimSpace(r.Header.Get("User-Agent")),
		Originator:        firstNonEmptyHeader(r.Header, "Originator", "originator"),
		RequestPath:       nativeCaptureRequestPath(r),
		HTTPMethod:        r.Method,
		IsWebsocket:       nativeCaptureIsWebsocketRequest(r),
		WebsocketProtocol: strings.TrimSpace(r.Header.Get("Sec-WebSocket-Protocol")),
		ClientType:        nativeCaptureRequestClientType(r),
		Model:             nativeCaptureRequestModel(r),
		RequestKind:       nativeCaptureRequestKind(r),
		Streaming:         nativeCaptureRequestStreaming(r),
		ResponseMode:      nativeCaptureRequestResponseMode(r),
		HTTP2Fingerprint:  strings.TrimSpace(firstNonEmptyHeader(r.Header, "X-TLS-Fingerprint-HTTP2", "X-HTTP2-Fingerprint")),
		StainlessMetadata: nativeCaptureStainlessMetadata(r.Header),
		HeadersSnapshot:   nativeCaptureHeadersSnapshot(r.Header),
		BodySummary:       nativeCaptureBodySummary(r),
		RawPayload:        string(nativeCaptureRawBody(r)),
		EventID:           strings.TrimSpace(firstNonEmptyHeader(r.Header, "X-Client-Request-Id", "x-client-request-id", "X-Request-Id", "x-request-id")),
		RequestSequence:   nativeCaptureRequestSequence(r),
		StreamID:          strings.TrimSpace(firstNonEmptyHeader(r.Header, "X-Stream-Id", "x-stream-id")),
		ClientHello:       raw,
	}
}
