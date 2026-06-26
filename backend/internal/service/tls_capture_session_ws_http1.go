package service

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	tlsfpTransport "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/transport"
	"github.com/gorilla/websocket"
)

func (l *TLSCaptureListener) handleCaptureWSHTTP1(w http.ResponseWriter, r *http.Request, raw []byte) {
	if l == nil || l.cfg.Service == nil || w == nil || r == nil {
		http.Error(w, "capture submission failed", http.StatusBadRequest)
		return
	}

	sessionID, err := ensureTLSCaptureSessionID(nativeCaptureRequestSessionID(r))
	if err != nil {
		http.Error(w, "capture submission failed", http.StatusBadRequest)
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin:  func(*http.Request) bool { return true },
		Subprotocols: websocket.Subprotocols(r),
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	requestSequence := 0
	for {
		msgType, payload, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return
			}
			return
		}
		if msgType != websocket.TextMessage && msgType != websocket.BinaryMessage {
			continue
		}
		requestSequence++

		submitReq := buildTLSCaptureWSHTTP1SubmitRequest(r, raw, sessionID, requestSequence, conn.Subprotocol(), payload)
		result, err := l.cfg.Service.SubmitNativeCapture(r.Context(), submitReq)
		if err != nil {
			return
		}
		if err := writeNativeCaptureWSResponse(conn, submitReq, result); err != nil {
			return
		}
	}
}

func buildTLSCaptureWSHTTP1SubmitRequest(
	r *http.Request,
	raw []byte,
	sessionID string,
	requestSequence int,
	subprotocol string,
	payload []byte,
) TLSFingerprintCaptureNativeSubmitRequest {
	clone := r.Clone(r.Context())
	clone.Header = r.Header.Clone()
	clone.Body = io.NopCloser(bytes.NewReader(payload))
	clone.ContentLength = int64(len(payload))

	req := buildTLSCaptureHTTP1SubmitRequest(clone, raw)
	req.Transport = string(tlsfpTransport.WebSocketH1)
	req.SessionID = strings.TrimSpace(sessionID)
	req.WebsocketProtocol = strings.TrimSpace(subprotocol)
	req.ResponseMode = "websocket"
	req.RequestSequence = requestSequence
	req.RawPayload = string(payload)
	return req
}
