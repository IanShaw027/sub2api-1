package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strings"

	tlsfpHTTP2 "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/http2"
	tlsfpTransport "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/transport"
	"github.com/gorilla/websocket"
	"golang.org/x/net/http2"
)

func isTLSCaptureH2WebSocketConnect(state *tlsCaptureH2StreamState) bool {
	if state == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(state.method), http.MethodConnect) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(state.protocol), "websocket") {
		return true
	}
	return strings.TrimSpace(firstNonEmptyHeader(state.header, "Sec-WebSocket-Version", "Sec-WebSocket-Protocol")) != ""
}

func (l *TLSCaptureListener) initCaptureH2WebSocketStream(fr *http2.Framer, streamID uint32, state *tlsCaptureH2StreamState) error {
	if l == nil || fr == nil || state == nil {
		return nil
	}
	if state.websocketSessionID == "" {
		sessionID, err := ensureTLSCaptureSessionID(strings.TrimSpace(firstNonEmptyHeader(
			state.header,
			"X-TLS-Fingerprint-Session-Id",
			"X-Session-Id",
			"X-Claude-Code-Session-Id",
			"OAI-Session-Id",
		)))
		if err != nil {
			return err
		}
		state.websocketSessionID = sessionID
	}
	if state.websocketProtocolSelected == "" {
		state.websocketProtocolSelected = firstWebSocketSubprotocol(firstNonEmptyHeader(state.header, "Sec-WebSocket-Protocol"))
	}
	if state.websocketHandshakeSent {
		return nil
	}
	header := make(http.Header)
	if state.websocketProtocolSelected != "" {
		header.Set("Sec-WebSocket-Protocol", state.websocketProtocolSelected)
	}
	if err := writeTLSCaptureH2Headers(fr, streamID, http.StatusOK, header, false); err != nil {
		return err
	}
	state.websocketHandshakeSent = true
	return nil
}

func (l *TLSCaptureListener) handleCaptureH2WebSocketDataFrame(
	fr *http2.Framer,
	streamID uint32,
	raw []byte,
	connFingerprint tlsCaptureH2ConnectionFingerprint,
	state *tlsCaptureH2StreamState,
	data []byte,
) (bool, error) {
	if l == nil || l.cfg.Service == nil || fr == nil || state == nil {
		return false, nil
	}
	if len(state.websocketReadBuffer)+len(data) > tlsFingerprintNativeCaptureBodyLimit {
		return true, writeTLSCaptureWSH2Close(fr, streamID, websocket.CloseMessageTooBig, "message too large")
	}
	state.websocketReadBuffer = append(state.websocketReadBuffer, data...)
	for {
		opcode, fin, payload, remaining, ok, err := decodeTLSCaptureWebSocketClientFrame(state.websocketReadBuffer)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
		state.websocketReadBuffer = remaining

		switch opcode {
		case 0x1:
			if fin {
				state.websocketRequestSequence++
				submitReq := buildTLSCaptureWSH2SubmitRequest(raw, connFingerprint, state, streamID, state.websocketRequestSequence, payload)
				result, err := l.cfg.Service.SubmitNativeCapture(context.Background(), submitReq)
				if err != nil {
					return false, err
				}
				if err := writeNativeCaptureWSH2Response(fr, streamID, submitReq, result); err != nil {
					return false, err
				}
				continue
			}
			if len(payload) > tlsFingerprintNativeCaptureBodyLimit {
				return true, writeTLSCaptureWSH2Close(fr, streamID, websocket.CloseMessageTooBig, "message too large")
			}
			state.websocketFragmentOpcode = opcode
			state.websocketMessageBuffer = append(state.websocketMessageBuffer[:0], payload...)
		case 0x0:
			if state.websocketFragmentOpcode != 0x1 {
				continue
			}
			if len(state.websocketMessageBuffer)+len(payload) > tlsFingerprintNativeCaptureBodyLimit {
				return true, writeTLSCaptureWSH2Close(fr, streamID, websocket.CloseMessageTooBig, "message too large")
			}
			state.websocketMessageBuffer = append(state.websocketMessageBuffer, payload...)
			if !fin {
				continue
			}
			state.websocketRequestSequence++
			submitReq := buildTLSCaptureWSH2SubmitRequest(raw, connFingerprint, state, streamID, state.websocketRequestSequence, state.websocketMessageBuffer)
			result, err := l.cfg.Service.SubmitNativeCapture(context.Background(), submitReq)
			if err != nil {
				return false, err
			}
			if err := writeNativeCaptureWSH2Response(fr, streamID, submitReq, result); err != nil {
				return false, err
			}
			state.websocketMessageBuffer = nil
			state.websocketFragmentOpcode = 0
		case 0x8:
			if err := fr.WriteData(streamID, false, encodeTLSCaptureWebSocketServerFrame(0x8, payload)); err != nil {
				return false, err
			}
			if err := fr.WriteData(streamID, true, nil); err != nil {
				return false, err
			}
			return true, nil
		case 0x9:
			if err := fr.WriteData(streamID, false, encodeTLSCaptureWebSocketServerFrame(0xA, payload)); err != nil {
				return false, err
			}
		}
	}
}

func buildTLSCaptureWSH2SubmitRequest(
	raw []byte,
	connFingerprint tlsCaptureH2ConnectionFingerprint,
	state *tlsCaptureH2StreamState,
	streamID uint32,
	requestSequence int,
	payload []byte,
) TLSFingerprintCaptureNativeSubmitRequest {
	requestURL := buildTLSCaptureRequestURL(state.authority, state.path)
	header := state.header.Clone()
	header.Set("Upgrade", "websocket")
	if state.websocketProtocolSelected != "" {
		header.Set("Sec-WebSocket-Protocol", state.websocketProtocolSelected)
	}
	req := &http.Request{
		Method:     http.MethodConnect,
		URL:        requestURL,
		Host:       state.authority,
		Header:     header,
		RemoteAddr: "",
		Body:       io.NopCloser(bytes.NewReader(payload)),
		TLS: &tls.ConnectionState{
			NegotiatedProtocol: "h2",
		},
	}
	submitReq := buildTLSCaptureHTTP1SubmitRequest(req, raw)
	submitReq.Transport = string(tlsfpTransport.WebSocketH2)
	submitReq.SessionID = strings.TrimSpace(state.websocketSessionID)
	submitReq.WebsocketProtocol = strings.TrimSpace(state.websocketProtocolSelected)
	submitReq.ResponseMode = "websocket"
	submitReq.RequestSequence = requestSequence
	submitReq.StreamID = fmt.Sprintf("%d", streamID)
	submitReq.HTTP2Fingerprint = strings.TrimSpace(tlsfpHTTP2.Fingerprint(
		connFingerprint.settings,
		connFingerprint.connWindowUpdates,
		connFingerprint.priorities,
		state.pseudoHeaderOrder,
	))
	submitReq.RawPayload = string(payload)
	submitReq.IsWebsocket = true
	submitReq.HTTPMethod = http.MethodConnect
	return submitReq
}

func writeNativeCaptureWSH2Response(fr *http2.Framer, streamID uint32, req TLSFingerprintCaptureNativeSubmitRequest, result *TLSFingerprintCaptureSubmitResult) error {
	created, err := buildNativeCaptureWSEventPayload("response.created", "in_progress", req, nil)
	if err != nil {
		return err
	}
	if err := fr.WriteData(streamID, false, encodeTLSCaptureWebSocketServerFrame(0x1, created)); err != nil {
		return err
	}
	completed, err := buildNativeCaptureWSEventPayload("response.completed", "completed", req, result)
	if err != nil {
		return err
	}
	return fr.WriteData(streamID, false, encodeTLSCaptureWebSocketServerFrame(0x1, completed))
}

func writeTLSCaptureWSH2Close(fr *http2.Framer, streamID uint32, code int, reason string) error {
	if fr == nil {
		return nil
	}
	closePayload := websocket.FormatCloseMessage(code, reason)
	if err := fr.WriteData(streamID, false, encodeTLSCaptureWebSocketServerFrame(0x8, closePayload)); err != nil {
		return err
	}
	return fr.WriteData(streamID, true, nil)
}

func firstWebSocketSubprotocol(raw string) string {
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func decodeTLSCaptureWebSocketClientFrame(buffer []byte) (opcode byte, fin bool, payload []byte, remaining []byte, ok bool, err error) {
	if len(buffer) < 2 {
		return 0, false, nil, buffer, false, nil
	}
	opcode = buffer[0] & 0x0f
	fin = buffer[0]&0x80 != 0
	payloadLen := int(buffer[1] & 0x7f)
	offset := 2
	switch payloadLen {
	case 126:
		if len(buffer) < offset+2 {
			return 0, false, nil, buffer, false, nil
		}
		payloadLen = int(buffer[offset])<<8 | int(buffer[offset+1])
		offset += 2
	case 127:
		if len(buffer) < offset+8 {
			return 0, false, nil, buffer, false, nil
		}
		payloadLen64 := binary.BigEndian.Uint64(buffer[offset : offset+8])
		if payloadLen64 > uint64(tlsFingerprintNativeCaptureBodyLimit) {
			return 0, false, nil, buffer, false, fmt.Errorf("websocket payload too large")
		}
		payloadLen = int(payloadLen64)
		offset += 8
	}
	if buffer[1]&0x80 == 0 {
		return 0, false, nil, buffer, false, fmt.Errorf("client websocket frame must be masked")
	}
	if len(buffer) < offset+4 {
		return 0, false, nil, buffer, false, nil
	}
	mask := buffer[offset : offset+4]
	offset += 4
	if len(buffer) < offset+payloadLen {
		return 0, false, nil, buffer, false, nil
	}
	payload = append([]byte(nil), buffer[offset:offset+payloadLen]...)
	for i := range payload {
		payload[i] ^= mask[i%4]
	}
	return opcode, fin, payload, buffer[offset+payloadLen:], true, nil
}

func encodeTLSCaptureWebSocketServerFrame(opcode byte, payload []byte) []byte {
	frame := make([]byte, 0, len(payload)+10)
	frame = append(frame, 0x80|opcode)
	switch {
	case len(payload) < 126:
		frame = append(frame, byte(len(payload)))
	case len(payload) <= 0xffff:
		frame = append(frame, 126, byte(len(payload)>>8), byte(len(payload)))
	default:
		var lenBuf [8]byte
		frame = append(frame, 127)
		binary.BigEndian.PutUint64(lenBuf[:], uint64(len(payload)))
		frame = append(frame, lenBuf[:]...)
	}
	frame = append(frame, payload...)
	return frame
}
