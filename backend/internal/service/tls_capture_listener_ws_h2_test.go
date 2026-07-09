//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net/http"
	"strings"
	"testing"

	tlsfpTransport "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/transport"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

func TestTLSCaptureWebSocketH2MultiTurnCreatesReplayableSamplePerTurn(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets:          map[string]int{"openai": 2},
		TransportTargets: map[string]int{string(tlsfpTransport.WebSocketH2): 2},
		UAKeywords:       []string{"codex"},
	})
	require.NoError(t, err)

	listener := NewTLSCaptureListener(TLSCaptureListenerConfig{
		Address: "127.0.0.1:0",
		Service: svc,
	})
	require.NoError(t, listener.Start())
	t.Cleanup(func() { stopNativeCaptureListener(t, listener) })

	conn, fr := newTLSCaptureH2Client(t, listener.Addr().String())
	t.Cleanup(func() { _ = conn.Close() })

	require.NoError(t, writeTLSCaptureH2PrefaceAndSettings(conn, fr,
		http2.Setting{ID: http2.SettingHeaderTableSize, Val: 4096},
		http2.Setting{ID: http2.SettingMaxConcurrentStreams, Val: 100},
		http2.Setting{ID: http2.SettingInitialWindowSize, Val: 65535},
	))

	require.NoError(t, writeTLSCaptureH2WebSocketConnect(fr, 1, tlsCaptureH2WebSocketConnectRequest{
		Path:       "/capture/openai/v1/responses",
		Authority:  listener.Addr().String(),
		Token:      task.Token,
		UserAgent:  "codex_exec/0.140.0",
		Originator: "codex_exec",
		SessionID:  "ws-h2-session-123",
		Protocol:   "openai-responses-v1",
	}))

	handshake := readTLSCaptureH2WebSocketHandshake(t, fr, 1)
	require.Equal(t, "200", handshake.Status)
	require.Equal(t, "openai-responses-v1", handshake.Headers.Get("sec-websocket-protocol"))

	firstPayload := `{"type":"response.create","model":"gpt-5.4","input":"capture-1","stream":true}`
	require.NoError(t, writeTLSCaptureH2WebSocketClientMessage(fr, 1, firstPayload))
	var readBuffer []byte
	firstCreated := readTLSCaptureH2WebSocketServerMessage(t, fr, 1, &readBuffer)
	firstCompleted := readTLSCaptureH2WebSocketServerMessage(t, fr, 1, &readBuffer)
	require.Contains(t, firstCreated, `"type":"response.created"`)
	require.Contains(t, firstCompleted, `"type":"response.completed"`)

	secondPayload := `{"type":"response.create","model":"gpt-5.4","input":"capture-2","stream":true}`
	require.NoError(t, writeTLSCaptureH2WebSocketClientMessage(fr, 1, secondPayload))
	secondCreated := readTLSCaptureH2WebSocketServerMessage(t, fr, 1, &readBuffer)
	secondCompleted := readTLSCaptureH2WebSocketServerMessage(t, fr, 1, &readBuffer)
	require.Contains(t, secondCreated, `"type":"response.created"`)
	require.Contains(t, secondCompleted, `"type":"response.completed"`)

	require.Len(t, repo.samples, 2)
	require.Len(t, repo.sessions, 1)
	require.Len(t, repo.sessionEvents, 2)
	require.Equal(t, string(tlsfpTransport.WebSocketH2), repo.samples[0].Transport)
	require.Equal(t, string(tlsfpTransport.WebSocketH2), repo.samples[1].Transport)
	require.True(t, repo.samples[0].IsWebsocket)
	require.True(t, repo.samples[1].IsWebsocket)
	require.Equal(t, "openai-responses-v1", repo.samples[0].WebsocketProtocol)
	require.Equal(t, "openai-responses-v1", repo.samples[1].WebsocketProtocol)
	require.Equal(t, "websocket", repo.samples[0].ResponseMode)
	require.Equal(t, "websocket", repo.samples[1].ResponseMode)
	require.Equal(t, "1:4096,3:100,4:65535|ph::method,:scheme,:authority,:path,:protocol", repo.samples[0].HTTP2Fingerprint)
	require.Equal(t, "1:4096,3:100,4:65535|ph::method,:scheme,:authority,:path,:protocol", repo.samples[1].HTTP2Fingerprint)
	require.JSONEq(t, firstPayload, repo.samples[0].RawPayload)
	require.JSONEq(t, secondPayload, repo.samples[1].RawPayload)
	require.Equal(t, "ws-h2-session-123", repo.samples[0].SessionID)
	require.Equal(t, "ws-h2-session-123", repo.samples[1].SessionID)
	require.Equal(t, http.MethodConnect, repo.samples[0].HTTPMethod)
	require.Equal(t, http.MethodConnect, repo.samples[1].HTTPMethod)

	require.Equal(t, "ws-h2-session-123", repo.sessions[0].SessionID)
	require.Equal(t, "h2", repo.sessions[0].ALPNNegotiated)

	require.Equal(t, 1, repo.sessionEvents[0].RequestSequence)
	require.Equal(t, 2, repo.sessionEvents[1].RequestSequence)
	require.Equal(t, string(tlsfpTransport.WebSocketH2), repo.sessionEvents[0].Transport)
	require.Equal(t, string(tlsfpTransport.WebSocketH2), repo.sessionEvents[1].Transport)
	require.True(t, repo.sessionEvents[0].IsWebsocket)
	require.True(t, repo.sessionEvents[1].IsWebsocket)
	require.Equal(t, "websocket", repo.sessionEvents[0].ResponseMode)
	require.Equal(t, "websocket", repo.sessionEvents[1].ResponseMode)
	require.JSONEq(t, firstPayload, repo.sessionEvents[0].RawPayload)
	require.JSONEq(t, secondPayload, repo.sessionEvents[1].RawPayload)
	require.Contains(t, repo.sessionEvents[1].BodySummary, `"capture-2"`)
	require.Equal(t, "1", repo.sessionEvents[0].StreamID)
	require.Equal(t, "1", repo.sessionEvents[1].StreamID)
}

func TestTLSCaptureWebSocketH2QueryParametersAreParsed(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets:          map[string]int{"openai": 1},
		TransportTargets: map[string]int{string(tlsfpTransport.WebSocketH2): 1},
	})
	require.NoError(t, err)

	listener := NewTLSCaptureListener(TLSCaptureListenerConfig{
		Address: "127.0.0.1:0",
		Service: svc,
	})
	require.NoError(t, listener.Start())
	t.Cleanup(func() { stopNativeCaptureListener(t, listener) })

	conn, fr := newTLSCaptureH2Client(t, listener.Addr().String())
	t.Cleanup(func() { _ = conn.Close() })

	require.NoError(t, writeTLSCaptureH2PrefaceAndSettings(conn, fr))
	require.NoError(t, writeTLSCaptureH2WebSocketConnect(fr, 1, tlsCaptureH2WebSocketConnectRequest{
		Path:       "/capture/openai/v1/responses?token=" + task.Token + "&model=gpt-query&stream=true",
		Authority:  listener.Addr().String(),
		UserAgent:  "codex_exec/0.140.0",
		Originator: "codex_exec",
		SessionID:  "ws-h2-query-session",
		Protocol:   "openai-responses-v1",
	}))

	handshake := readTLSCaptureH2WebSocketHandshake(t, fr, 1)
	require.Equal(t, "200", handshake.Status)

	payload := `{"type":"response.create","input":"capture-query"}`
	require.NoError(t, writeTLSCaptureH2WebSocketClientMessage(fr, 1, payload))
	var readBuffer []byte
	_ = readTLSCaptureH2WebSocketServerMessage(t, fr, 1, &readBuffer)
	_ = readTLSCaptureH2WebSocketServerMessage(t, fr, 1, &readBuffer)

	require.Len(t, repo.samples, 1)
	require.Equal(t, "/v1/responses", repo.samples[0].RequestPath)
	require.Equal(t, "gpt-query", repo.samples[0].Model)
	require.True(t, repo.samples[0].Streaming)
}

func TestTLSCaptureWebSocketH2FragmentedTextMessageCreatesSingleReplayableSample(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets:          map[string]int{"openai": 1},
		TransportTargets: map[string]int{string(tlsfpTransport.WebSocketH2): 1},
		UAKeywords:       []string{"codex"},
	})
	require.NoError(t, err)

	listener := NewTLSCaptureListener(TLSCaptureListenerConfig{
		Address: "127.0.0.1:0",
		Service: svc,
	})
	require.NoError(t, listener.Start())
	t.Cleanup(func() { stopNativeCaptureListener(t, listener) })

	conn, fr := newTLSCaptureH2Client(t, listener.Addr().String())
	t.Cleanup(func() { _ = conn.Close() })

	require.NoError(t, writeTLSCaptureH2PrefaceAndSettings(conn, fr))
	require.NoError(t, writeTLSCaptureH2WebSocketConnect(fr, 1, tlsCaptureH2WebSocketConnectRequest{
		Path:       "/capture/openai/v1/responses",
		Authority:  listener.Addr().String(),
		Token:      task.Token,
		UserAgent:  "codex_exec/0.140.0",
		Originator: "codex_exec",
		SessionID:  "ws-h2-fragment-session",
		Protocol:   "openai-responses-v1",
	}))

	handshake := readTLSCaptureH2WebSocketHandshake(t, fr, 1)
	require.Equal(t, "200", handshake.Status)

	payload := `{"type":"response.create","model":"gpt-5.4","input":"fragmented","stream":true}`
	require.NoError(t, fr.WriteData(1, false, encodeTLSCaptureWebSocketClientFrame(0x1, false, []byte(payload[:30]))))
	require.NoError(t, fr.WriteData(1, false, encodeTLSCaptureWebSocketClientFrame(0x0, true, []byte(payload[30:]))))

	var readBuffer []byte
	created := readTLSCaptureH2WebSocketServerMessage(t, fr, 1, &readBuffer)
	completed := readTLSCaptureH2WebSocketServerMessage(t, fr, 1, &readBuffer)
	require.Contains(t, created, `"type":"response.created"`)
	require.Contains(t, completed, `"type":"response.completed"`)

	require.Len(t, repo.samples, 1)
	require.JSONEq(t, payload, repo.samples[0].RawPayload)
	require.Len(t, repo.sessionEvents, 1)
	require.JSONEq(t, payload, repo.sessionEvents[0].RawPayload)
}

func TestTLSCaptureWebSocketH2RejectsOversizedFragmentedMessage(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets:          map[string]int{"openai": 1},
		TransportTargets: map[string]int{string(tlsfpTransport.WebSocketH2): 1},
	})
	require.NoError(t, err)

	listener := NewTLSCaptureListener(TLSCaptureListenerConfig{
		Address: "127.0.0.1:0",
		Service: svc,
	})
	require.NoError(t, listener.Start())
	t.Cleanup(func() { stopNativeCaptureListener(t, listener) })

	conn, fr := newTLSCaptureH2Client(t, listener.Addr().String())
	t.Cleanup(func() { _ = conn.Close() })

	require.NoError(t, writeTLSCaptureH2PrefaceAndSettings(conn, fr))
	require.NoError(t, writeTLSCaptureH2WebSocketConnect(fr, 1, tlsCaptureH2WebSocketConnectRequest{
		Path:       "/capture/openai/v1/responses",
		Authority:  listener.Addr().String(),
		Token:      task.Token,
		UserAgent:  "codex_exec/0.140.0",
		Originator: "codex_exec",
		SessionID:  "ws-h2-too-large",
		Protocol:   "openai-responses-v1",
	}))

	handshake := readTLSCaptureH2WebSocketHandshake(t, fr, 1)
	require.Equal(t, "200", handshake.Status)

	payload := strings.Repeat("a", tlsFingerprintNativeCaptureBodyLimit+1024)
	const chunkSize = 60000
	for offset := 0; offset < len(payload); offset += chunkSize {
		end := offset + chunkSize
		if end > len(payload) {
			end = len(payload)
		}
		opcode := byte(0x0)
		fin := false
		if offset == 0 {
			opcode = 0x1
		}
		if end == len(payload) {
			fin = true
		}
		require.NoError(t, fr.WriteData(1, false, encodeTLSCaptureWebSocketClientFrame(opcode, fin, []byte(payload[offset:end]))))
	}

	opcode, _, endStream := readTLSCaptureH2WebSocketServerControlFrame(t, fr, 1)
	require.Equal(t, byte(0x8), opcode)
	require.False(t, endStream)
	require.True(t, readTLSCaptureH2StreamEnd(t, fr, 1))
	require.Empty(t, repo.samples)
	require.Empty(t, repo.sessionEvents)
}

func TestDecodeTLSCaptureWebSocketClientFrame_AllowsExtendedPayloadLengthWithinCaptureLimit(t *testing.T) {
	payload := bytes.Repeat([]byte("a"), 70_000)
	frame := encodeTLSCaptureWebSocketClientFrameExtended(0x1, true, payload)

	opcode, fin, decoded, remaining, ok, err := decodeTLSCaptureWebSocketClientFrame(frame)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, fin)
	require.Equal(t, byte(0x1), opcode)
	require.Equal(t, payload, decoded)
	require.Empty(t, remaining)
}

func TestEncodeTLSCaptureWebSocketServerFrame_AllowsExtendedPayloadLengthWithinCaptureLimit(t *testing.T) {
	payload := bytes.Repeat([]byte("b"), 70_000)

	var frame []byte
	require.NotPanics(t, func() {
		frame = encodeTLSCaptureWebSocketServerFrame(0x1, payload)
	})
	require.Len(t, frame, 10+len(payload))
	require.Equal(t, byte(0x81), frame[0])
	require.Equal(t, byte(127), frame[1])
	require.Equal(t, uint64(len(payload)), binary.BigEndian.Uint64(frame[2:10]))
	require.Equal(t, payload, frame[10:])
}

type tlsCaptureH2WebSocketConnectRequest struct {
	Path       string
	Authority  string
	Token      string
	UserAgent  string
	Originator string
	SessionID  string
	Protocol   string
}

func writeTLSCaptureH2WebSocketConnect(fr *http2.Framer, streamID uint32, req tlsCaptureH2WebSocketConnectRequest) error {
	var headerBlock bytes.Buffer
	encoder := hpack.NewEncoder(&headerBlock)
	fields := []hpack.HeaderField{
		{Name: ":method", Value: http.MethodConnect},
		{Name: ":scheme", Value: "https"},
		{Name: ":authority", Value: req.Authority},
		{Name: ":path", Value: req.Path},
		{Name: ":protocol", Value: "websocket"},
		{Name: "authorization", Value: "Bearer " + req.Token},
		{Name: "user-agent", Value: req.UserAgent},
		{Name: "originator", Value: req.Originator},
		{Name: "x-claude-code-session-id", Value: req.SessionID},
		{Name: "x-stainless-lang", Value: "js"},
		{Name: "x-stainless-runtime", Value: "node"},
		{Name: "sec-websocket-version", Value: "13"},
		{Name: "sec-websocket-protocol", Value: req.Protocol},
	}
	for _, field := range fields {
		if err := encoder.WriteField(field); err != nil {
			return err
		}
	}
	return fr.WriteHeaders(http2.HeadersFrameParam{
		StreamID:      streamID,
		EndHeaders:    true,
		EndStream:     false,
		BlockFragment: headerBlock.Bytes(),
	})
}

func readTLSCaptureH2WebSocketHandshake(t *testing.T, fr *http2.Framer, streamID uint32) tlsCaptureH2Response {
	t.Helper()

	resp := tlsCaptureH2Response{Headers: make(http.Header)}
	for {
		frame, err := fr.ReadFrame()
		require.NoError(t, err)
		switch f := frame.(type) {
		case *http2.SettingsFrame:
			if !f.IsAck() {
				require.NoError(t, fr.WriteSettingsAck())
			}
		case *http2.MetaHeadersFrame:
			if f.StreamID != streamID {
				continue
			}
			resp.Status = f.PseudoValue("status")
			for _, field := range f.RegularFields() {
				resp.Headers.Add(strings.ToLower(field.Name), field.Value)
			}
			return resp
		}
	}
}

func writeTLSCaptureH2WebSocketClientMessage(fr *http2.Framer, streamID uint32, payload string) error {
	return fr.WriteData(streamID, false, encodeTLSCaptureWebSocketClientFrame(0x1, true, []byte(payload)))
}

func readTLSCaptureH2WebSocketServerMessage(t *testing.T, fr *http2.Framer, streamID uint32, buffer *[]byte) string {
	t.Helper()

	for {
		frame, err := fr.ReadFrame()
		require.NoError(t, err)
		switch f := frame.(type) {
		case *http2.SettingsFrame:
			if !f.IsAck() {
				require.NoError(t, fr.WriteSettingsAck())
			}
		case *http2.DataFrame:
			if f.StreamID != streamID {
				continue
			}
			*buffer = append(*buffer, f.Data()...)
			payload, remaining, ok, decodeErr := decodeTLSCaptureWebSocketServerFrame(*buffer)
			require.NoError(t, decodeErr)
			if !ok {
				continue
			}
			*buffer = remaining
			return string(payload)
		}
	}
}

func encodeTLSCaptureWebSocketClientFrame(opcode byte, fin bool, payload []byte) []byte {
	frame := make([]byte, 0, len(payload)+8)
	first := opcode & 0x0f
	if fin {
		first |= 0x80
	}
	frame = append(frame, first)
	switch {
	case len(payload) < 126:
		frame = append(frame, byte(0x80|len(payload)))
	case len(payload) <= 0xffff:
		frame = append(frame, 0x80|126, byte(len(payload)>>8), byte(len(payload)))
	default:
		panic("client websocket payload too large for test helper")
	}
	mask := []byte{0x11, 0x22, 0x33, 0x44}
	frame = append(frame, mask...)
	for i, b := range payload {
		frame = append(frame, b^mask[i%4])
	}
	return frame
}

func encodeTLSCaptureWebSocketClientFrameExtended(opcode byte, fin bool, payload []byte) []byte {
	frame := make([]byte, 0, len(payload)+14)
	first := opcode & 0x0f
	if fin {
		first |= 0x80
	}
	frame = append(frame, first)
	frame = append(frame, 0x80|127)
	var lenBuf [8]byte
	binary.BigEndian.PutUint64(lenBuf[:], uint64(len(payload)))
	frame = append(frame, lenBuf[:]...)
	mask := []byte{0x11, 0x22, 0x33, 0x44}
	frame = append(frame, mask...)
	for i, b := range payload {
		frame = append(frame, b^mask[i%4])
	}
	return frame
}

func readTLSCaptureH2WebSocketServerControlFrame(t *testing.T, fr *http2.Framer, streamID uint32) (byte, []byte, bool) {
	t.Helper()

	var buffer []byte
	for {
		frame, err := fr.ReadFrame()
		require.NoError(t, err)
		switch f := frame.(type) {
		case *http2.SettingsFrame:
			if !f.IsAck() {
				require.NoError(t, fr.WriteSettingsAck())
			}
		case *http2.DataFrame:
			if f.StreamID != streamID {
				continue
			}
			buffer = append(buffer, f.Data()...)
			if len(buffer) < 2 {
				if f.StreamEnded() {
					return 0, nil, true
				}
				continue
			}
			opcode := buffer[0] & 0x0f
			payloadLen := int(buffer[1] & 0x7f)
			offset := 2
			if payloadLen == 126 {
				require.GreaterOrEqual(t, len(buffer), 4)
				payloadLen = int(buffer[2])<<8 | int(buffer[3])
				offset = 4
			}
			require.GreaterOrEqual(t, len(buffer), offset+payloadLen)
			payload := append([]byte(nil), buffer[offset:offset+payloadLen]...)
			return opcode, payload, f.StreamEnded()
		}
	}
}

func readTLSCaptureH2StreamEnd(t *testing.T, fr *http2.Framer, streamID uint32) bool {
	t.Helper()

	for {
		frame, err := fr.ReadFrame()
		require.NoError(t, err)
		switch f := frame.(type) {
		case *http2.SettingsFrame:
			if !f.IsAck() {
				require.NoError(t, fr.WriteSettingsAck())
			}
		case *http2.DataFrame:
			if f.StreamID != streamID {
				continue
			}
			if f.StreamEnded() {
				return true
			}
		}
	}
}

func decodeTLSCaptureWebSocketServerFrame(buffer []byte) ([]byte, []byte, bool, error) {
	if len(buffer) < 2 {
		return nil, buffer, false, nil
	}
	payloadLen := int(buffer[1] & 0x7f)
	offset := 2
	switch payloadLen {
	case 126:
		if len(buffer) < offset+2 {
			return nil, buffer, false, nil
		}
		payloadLen = int(buffer[offset])<<8 | int(buffer[offset+1])
		offset += 2
	case 127:
		return nil, buffer, false, fmt.Errorf("websocket payload too large")
	}
	if buffer[1]&0x80 != 0 {
		return nil, buffer, false, fmt.Errorf("server websocket frame must not be masked")
	}
	if len(buffer) < offset+payloadLen {
		return nil, buffer, false, nil
	}
	opcode := buffer[0] & 0x0f
	if opcode != 0x1 {
		return nil, buffer[offset+payloadLen:], false, fmt.Errorf("unexpected websocket opcode: %d", opcode)
	}
	payload := append([]byte(nil), buffer[offset:offset+payloadLen]...)
	return payload, buffer[offset+payloadLen:], true, nil
}
