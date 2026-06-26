//go:build unit

package service

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	tlsfpTransport "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/transport"
	"github.com/gorilla/websocket"
	utls "github.com/refraction-networking/utls"
	"github.com/stretchr/testify/require"
)

func TestTLSCaptureWebSocketHTTP1MultiTurnCreatesReplayableSamplePerTurn(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets:          map[string]int{"openai": 2},
		TransportTargets: map[string]int{string(tlsfpTransport.WebSocketH1): 2},
		UAKeywords:       []string{"codex"},
	})
	require.NoError(t, err)

	listener := NewTLSCaptureListener(TLSCaptureListenerConfig{
		Address: "127.0.0.1:0",
		Service: svc,
	})
	require.NoError(t, listener.Start())
	t.Cleanup(func() { stopNativeCaptureListener(t, listener) })

	header := make(http.Header)
	header.Set("Authorization", "Bearer "+task.Token)
	header.Set("User-Agent", "codex_exec/0.140.0")
	header.Set("Originator", "codex_exec")
	header.Set("X-Claude-Code-Session-Id", "ws-session-123")
	header.Set("X-Stainless-Lang", "js")
	header.Set("X-Stainless-Runtime", "node")

	conn, resp := newTLSCaptureWSHTTP1Client(t, listener.Addr().String(), "/capture/openai/v1/responses", header, []string{"openai-responses-v1"})
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	t.Cleanup(func() {
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_ = conn.Close()
	})

	firstPayload := `{"type":"response.create","model":"gpt-5.4","input":"capture-1","stream":true}`
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(firstPayload)))
	firstCreated := readTLSCaptureWSMessage(t, conn)
	firstCompleted := readTLSCaptureWSMessage(t, conn)
	require.Contains(t, firstCreated, `"type":"response.created"`)
	require.Contains(t, firstCompleted, `"type":"response.completed"`)

	secondPayload := `{"type":"response.create","model":"gpt-5.4","input":"capture-2","stream":true}`
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(secondPayload)))
	secondCreated := readTLSCaptureWSMessage(t, conn)
	secondCompleted := readTLSCaptureWSMessage(t, conn)
	require.Contains(t, secondCreated, `"type":"response.created"`)
	require.Contains(t, secondCompleted, `"type":"response.completed"`)

	require.Len(t, repo.samples, 2)
	require.Len(t, repo.sessions, 1)
	require.Len(t, repo.sessionEvents, 2)

	require.Equal(t, string(tlsfpTransport.WebSocketH1), repo.samples[0].Transport)
	require.Equal(t, string(tlsfpTransport.WebSocketH1), repo.samples[1].Transport)
	require.True(t, repo.samples[0].IsWebsocket)
	require.True(t, repo.samples[1].IsWebsocket)
	require.Equal(t, "openai-responses-v1", repo.samples[0].WebsocketProtocol)
	require.Equal(t, "openai-responses-v1", repo.samples[1].WebsocketProtocol)
	require.Equal(t, "websocket", repo.samples[0].ResponseMode)
	require.Equal(t, "websocket", repo.samples[1].ResponseMode)
	require.Equal(t, "gpt-5.4", repo.samples[0].Model)
	require.Equal(t, "gpt-5.4", repo.samples[1].Model)
	require.JSONEq(t, firstPayload, repo.samples[0].RawPayload)
	require.JSONEq(t, secondPayload, repo.samples[1].RawPayload)
	require.Equal(t, "ws-session-123", repo.samples[0].SessionID)
	require.Equal(t, "ws-session-123", repo.samples[1].SessionID)

	require.Equal(t, "ws-session-123", repo.sessions[0].SessionID)
	require.Equal(t, "http/1.1", repo.sessions[0].ALPNNegotiated)

	require.Equal(t, 1, repo.sessionEvents[0].RequestSequence)
	require.Equal(t, 2, repo.sessionEvents[1].RequestSequence)
	require.Equal(t, string(tlsfpTransport.WebSocketH1), repo.sessionEvents[0].Transport)
	require.Equal(t, string(tlsfpTransport.WebSocketH1), repo.sessionEvents[1].Transport)
	require.True(t, repo.sessionEvents[0].IsWebsocket)
	require.True(t, repo.sessionEvents[1].IsWebsocket)
	require.Equal(t, "websocket", repo.sessionEvents[0].ResponseMode)
	require.Equal(t, "websocket", repo.sessionEvents[1].ResponseMode)
	require.JSONEq(t, firstPayload, repo.sessionEvents[0].RawPayload)
	require.JSONEq(t, secondPayload, repo.sessionEvents[1].RawPayload)
	require.Contains(t, repo.sessionEvents[1].BodySummary, `"capture-2"`)
}

func newTLSCaptureWSHTTP1Client(t *testing.T, addr, path string, header http.Header, subprotocols []string) (*websocket.Conn, *http.Response) {
	t.Helper()

	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
		Subprotocols:     subprotocols,
	}
	dialer.NetDialTLSContext = func(ctx context.Context, network, remoteAddr string) (net.Conn, error) {
		conn, err := net.DialTimeout(network, remoteAddr, 5*time.Second)
		if err != nil {
			return nil, err
		}
		uconn := utls.UClient(conn, &utls.Config{
			ServerName:         "localhost",
			InsecureSkipVerify: true,
			NextProtos:         []string{"http/1.1"},
		}, utls.HelloCustom)
		if err := uconn.ApplyPreset(nativeClientHelloSpecHTTP1Only()); err != nil {
			_ = conn.Close()
			return nil, err
		}
		if err := uconn.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return nil, err
		}
		return uconn, nil
	}

	wsURL := url.URL{
		Scheme: "wss",
		Host:   addr,
		Path:   path,
	}
	conn, resp, err := dialer.Dial(wsURL.String(), header)
	require.NoError(t, err)
	require.NotNil(t, resp)
	return conn, resp
}

func nativeClientHelloSpecHTTP1Only() *utls.ClientHelloSpec {
	spec := nativeClientHelloSpec()
	extensions := make([]utls.TLSExtension, 0, len(spec.Extensions))
	for _, ext := range spec.Extensions {
		if _, ok := ext.(*utls.ALPNExtension); ok {
			extensions = append(extensions, &utls.ALPNExtension{AlpnProtocols: []string{"http/1.1"}})
			continue
		}
		extensions = append(extensions, ext)
	}
	spec.Extensions = extensions
	return spec
}

func readTLSCaptureWSMessage(t *testing.T, conn *websocket.Conn) string {
	t.Helper()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	msgType, payload, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.TextMessage, msgType)
	return string(payload)
}
