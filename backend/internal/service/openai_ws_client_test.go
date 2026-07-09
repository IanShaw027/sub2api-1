package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

func TestWrapOpenAIWSHandshakeResponseBodyErrorCapturesBody(t *testing.T) {
	baseErr := errors.New("failed to WebSocket dial")
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(`{"error":{"code":"token_revoked","message":"revoked"}}`)),
	}

	err := wrapOpenAIWSHandshakeResponseBodyError(baseErr, resp)

	require.ErrorIs(t, err, baseErr)
	require.JSONEq(t, `{"error":{"code":"token_revoked","message":"revoked"}}`, string(openAIWSHandshakeBodyFromError(err)))
}

func TestCoderOpenAIWSClientDialer_ProxyHTTPClientReuse(t *testing.T) {
	dialer := newDefaultOpenAIWSClientDialer()
	impl, ok := dialer.(*coderOpenAIWSClientDialer)
	require.True(t, ok)

	c1, err := impl.proxyHTTPClient("http://127.0.0.1:8080")
	require.NoError(t, err)
	c2, err := impl.proxyHTTPClient("http://127.0.0.1:8080")
	require.NoError(t, err)
	require.Same(t, c1, c2, "同一代理地址应复用同一个 HTTP 客户端")

	c3, err := impl.proxyHTTPClient("http://127.0.0.1:8081")
	require.NoError(t, err)
	require.NotSame(t, c1, c3, "不同代理地址应分离客户端")
}

func TestCoderOpenAIWSClientDialer_ProxyHTTPClientInvalidURL(t *testing.T) {
	dialer := newDefaultOpenAIWSClientDialer()
	impl, ok := dialer.(*coderOpenAIWSClientDialer)
	require.True(t, ok)

	_, err := impl.proxyHTTPClient("://bad")
	require.Error(t, err)
}

func TestCoderOpenAIWSClientDialer_TransportMetricsSnapshot(t *testing.T) {
	dialer := newDefaultOpenAIWSClientDialer()
	impl, ok := dialer.(*coderOpenAIWSClientDialer)
	require.True(t, ok)

	_, err := impl.proxyHTTPClient("http://127.0.0.1:18080")
	require.NoError(t, err)
	_, err = impl.proxyHTTPClient("http://127.0.0.1:18080")
	require.NoError(t, err)
	_, err = impl.proxyHTTPClient("http://127.0.0.1:18081")
	require.NoError(t, err)

	snapshot := impl.SnapshotTransportMetrics()
	require.Equal(t, int64(1), snapshot.ProxyClientCacheHits)
	require.Equal(t, int64(2), snapshot.ProxyClientCacheMisses)
	require.InDelta(t, 1.0/3.0, snapshot.TransportReuseRatio, 0.0001)
}

func TestCoderOpenAIWSClientDialer_ProxyClientCacheCapacity(t *testing.T) {
	dialer := newDefaultOpenAIWSClientDialer()
	impl, ok := dialer.(*coderOpenAIWSClientDialer)
	require.True(t, ok)

	total := openAIWSProxyClientCacheMaxEntries + 32
	for i := 0; i < total; i++ {
		_, err := impl.proxyHTTPClient(fmt.Sprintf("http://127.0.0.1:%d", 20000+i))
		require.NoError(t, err)
	}

	impl.proxyMu.Lock()
	cacheSize := len(impl.proxyClients)
	impl.proxyMu.Unlock()

	require.LessOrEqual(t, cacheSize, openAIWSProxyClientCacheMaxEntries, "代理客户端缓存应受容量上限约束")
}

func TestCoderOpenAIWSClientDialer_ProxyClientCacheIdleTTL(t *testing.T) {
	dialer := newDefaultOpenAIWSClientDialer()
	impl, ok := dialer.(*coderOpenAIWSClientDialer)
	require.True(t, ok)

	oldProxy := "http://127.0.0.1:28080"
	_, err := impl.proxyHTTPClient(oldProxy)
	require.NoError(t, err)

	impl.proxyMu.Lock()
	oldEntry := impl.proxyClients[oldProxy]
	require.NotNil(t, oldEntry)
	oldEntry.lastUsedUnixNano = time.Now().Add(-openAIWSProxyClientCacheIdleTTL - time.Minute).UnixNano()
	impl.proxyMu.Unlock()

	// 触发一次新的代理获取，驱动 TTL 清理。
	_, err = impl.proxyHTTPClient("http://127.0.0.1:28081")
	require.NoError(t, err)

	impl.proxyMu.Lock()
	_, exists := impl.proxyClients[oldProxy]
	impl.proxyMu.Unlock()

	require.False(t, exists, "超过空闲 TTL 的代理客户端应被回收")
}

func TestCoderOpenAIWSClientDialer_ProxyTransportTLSHandshakeTimeout(t *testing.T) {
	dialer := newDefaultOpenAIWSClientDialer()
	impl, ok := dialer.(*coderOpenAIWSClientDialer)
	require.True(t, ok)

	client, err := impl.proxyHTTPClient("http://127.0.0.1:38080")
	require.NoError(t, err)
	require.NotNil(t, client)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.NotNil(t, transport)
	require.Equal(t, 10*time.Second, transport.TLSHandshakeTimeout)
}

func TestOpenAIWSTLSProfileWantsWebSocketH2(t *testing.T) {
	require.False(t, openAIWSTLSProfileWantsWebSocketH2(nil))
	require.False(t, openAIWSTLSProfileWantsWebSocketH2(&tlsfingerprint.Profile{ALPNProtocols: []string{"http/1.1"}}))
	require.False(t, openAIWSTLSProfileWantsWebSocketH2(&tlsfingerprint.Profile{
		ALPNProtocols:    []string{"h2", "http/1.1"},
		HTTP2Fingerprint: "1:4096|ph::method,:scheme,:authority,:path",
	}))
	require.True(t, openAIWSTLSProfileWantsWebSocketH2(&tlsfingerprint.Profile{
		ALPNProtocols:    []string{"h2", "http/1.1"},
		HTTP2Fingerprint: "1:4096|ph::method,:protocol,:scheme,:authority,:path",
	}))
}

func TestDialOpenAIWSH2WithConn_SendsExtendedConnectAndExchangesFrames(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	defer func() { _ = serverConn.Close() }()

	profile := &tlsfingerprint.Profile{
		Name:             "ws-h2",
		ALPNProtocols:    []string{"h2", "http/1.1"},
		HTTP2Fingerprint: "1:4096,4:65535|wu:12345|p:1:0:1:200|ph::method,:protocol,:scheme,:authority,:path",
	}
	headers := http.Header{
		"Authorization":          []string{"Bearer test-token"},
		"OpenAI-Beta":            []string{"responses_websockets=2026-02-06"},
		"Sec-WebSocket-Protocol": []string{"realtime"},
	}

	serverDone := make(chan struct{})
	clientDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		runOpenAIWSH2TestServer(t, serverConn, clientDone)
	}()

	wsURL := "wss://api.openai.com/v1/realtime?model=gpt-5"
	conn, status, respHeaders, err := dialOpenAIWSH2WithConn(context.Background(), wsURL, headers, profile, clientConn)
	require.NoError(t, err)
	require.Equal(t, 0, status)
	require.Equal(t, "realtime", respHeaders.Get("Sec-WebSocket-Protocol"))
	require.NotNil(t, conn)
	defer func() { _ = conn.Close() }()

	require.NoError(t, conn.WriteJSON(context.Background(), map[string]any{
		"type": "response.create",
	}))
	msg, err := conn.ReadMessage(context.Background())
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"response.created"}`, string(msg))

	close(clientDone)
	<-serverDone
}

func TestDialOpenAIWSH2WithConn_AutoPongsAndReassemblesFragmentedMessages(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	defer func() { _ = serverConn.Close() }()

	profile := &tlsfingerprint.Profile{
		Name:             "ws-h2",
		ALPNProtocols:    []string{"h2", "http/1.1"},
		HTTP2Fingerprint: "1:4096,4:65535|wu:12345|p:1:0:1:200|ph::method,:protocol,:scheme,:authority,:path",
	}
	headers := http.Header{
		"Authorization":          []string{"Bearer test-token"},
		"OpenAI-Beta":            []string{"responses_websockets=2026-02-06"},
		"Sec-WebSocket-Protocol": []string{"realtime"},
	}

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		fr := acceptOpenAIWSH2Handshake(t, serverConn, []http2.Setting{
			{ID: http2.SettingHeaderTableSize, Val: 4096},
			{ID: http2.SettingInitialWindowSize, Val: 65535},
		})

		pongSeen := make(chan struct{})
		drainErr := make(chan error, 1)
		go func() {
			for {
				frame, err := fr.ReadFrame()
				if err != nil {
					return
				}
				switch f := frame.(type) {
				case *http2.WindowUpdateFrame:
				case *http2.DataFrame:
					opcode, fin, payload, remaining, ok, err := decodeTLSCaptureWebSocketClientFrame(f.Data())
					if err != nil {
						select {
						case drainErr <- err:
						default:
						}
						return
					}
					if !ok || len(remaining) > 0 {
						select {
						case drainErr <- fmt.Errorf("unexpected client websocket frame state: ok=%v remaining=%d", ok, len(remaining)):
						default:
						}
						return
					}
					if opcode == 0xA {
						if !fin || !bytes.Equal(payload, []byte("ping")) {
							select {
							case drainErr <- fmt.Errorf("unexpected pong frame fin=%v payload=%q", fin, string(payload)):
							default:
							}
							return
						}
						select {
						case <-pongSeen:
						default:
							close(pongSeen)
						}
					}
				}
			}
		}()

		require.NoError(t, fr.WriteData(1, false, encodeTLSCaptureWebSocketServerFrame(0x9, []byte("ping"))))
		<-pongSeen
		select {
		case err := <-drainErr:
			require.NoError(t, err)
		default:
		}

		require.NoError(t, fr.WriteData(1, false, encodeTLSCaptureWebSocketServerFrameWithFIN(0x1, false, []byte(`{"type":"response.`))))
		require.NoError(t, fr.WriteData(1, false, encodeTLSCaptureWebSocketServerFrameWithFIN(0x0, true, []byte(`created"}`))))
	}()

	wsURL := "wss://api.openai.com/v1/realtime?model=gpt-5"
	conn, status, _, err := dialOpenAIWSH2WithConn(context.Background(), wsURL, headers, profile, clientConn)
	require.NoError(t, err)
	require.Equal(t, 0, status)
	require.NotNil(t, conn)
	defer func() { _ = conn.Close() }()

	msg, err := conn.ReadMessage(context.Background())
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"response.created"}`, string(msg))

	<-serverDone
}

func TestDialOpenAIWSH2WithConn_CloseFrameClosesClient(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	defer func() { _ = serverConn.Close() }()

	profile := &tlsfingerprint.Profile{
		Name:             "ws-h2",
		ALPNProtocols:    []string{"h2", "http/1.1"},
		HTTP2Fingerprint: "1:4096|ph::method,:protocol,:scheme,:authority,:path",
	}
	headers := http.Header{
		"Authorization":          []string{"Bearer test-token"},
		"OpenAI-Beta":            []string{"responses_websockets=2026-02-06"},
		"Sec-WebSocket-Protocol": []string{"realtime"},
	}

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		fr := acceptOpenAIWSH2Handshake(t, serverConn, []http2.Setting{
			{ID: http2.SettingHeaderTableSize, Val: 4096},
		})
		closeFrame := encodeTLSCaptureWebSocketServerFrame(0x8, openAIWSWebSocketClosePayload(1000, "bye"))
		require.NoError(t, fr.WriteData(1, false, closeFrame))
		for {
			if _, err := fr.ReadFrame(); err != nil {
				return
			}
		}
	}()

	conn, status, _, err := dialOpenAIWSH2WithConn(context.Background(), "wss://api.openai.com/v1/realtime?model=gpt-5", headers, profile, clientConn)
	require.NoError(t, err)
	require.Equal(t, 0, status)
	require.NotNil(t, conn)

	msg, err := conn.ReadMessage(context.Background())
	require.Nil(t, msg)
	require.ErrorIs(t, err, errOpenAIWSConnClosed)

	<-serverDone
}

func TestBuildOpenAIWSH2RequestHeaders_AllowsNilHeaders(t *testing.T) {
	fields, err := buildOpenAIWSH2RequestHeaders(
		mustParseOpenAIWSURL(t, "wss://api.openai.com/v1/realtime?model=gpt-5"),
		nil,
		[]string{":method", ":protocol", ":scheme", ":authority", ":path"},
	)
	require.NoError(t, err)

	asHTTP := make(http.Header)
	for _, field := range fields {
		if strings.HasPrefix(field.Name, ":") {
			continue
		}
		asHTTP.Add(field.Name, field.Value)
	}
	require.Equal(t, "13", asHTTP.Get("Sec-WebSocket-Version"))
	require.NotEmpty(t, asHTTP.Get("Sec-WebSocket-Key"))
}

func runOpenAIWSH2TestServer(t *testing.T, conn net.Conn, clientDone <-chan struct{}) {
	t.Helper()
	defer func() { _ = conn.Close() }()

	fr := acceptOpenAIWSH2Handshake(t, conn, []http2.Setting{
		{ID: http2.SettingHeaderTableSize, Val: 4096},
		{ID: http2.SettingInitialWindowSize, Val: 65535},
	})

	var wsPayload []byte
	for len(wsPayload) == 0 {
		frame, err := fr.ReadFrame()
		require.NoError(t, err)
		switch f := frame.(type) {
		case *http2.SettingsFrame:
			if !f.IsAck() {
				require.NoError(t, fr.WriteSettingsAck())
			}
		case *http2.DataFrame:
			opcode, fin, payload, remaining, ok, err := decodeTLSCaptureWebSocketClientFrame(f.Data())
			require.NoError(t, err)
			require.True(t, ok)
			require.True(t, fin)
			require.Equal(t, byte(0x1), opcode)
			require.Empty(t, remaining)
			wsPayload = payload
		}
	}
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(wsPayload, &decoded))
	require.Equal(t, "response.create", decoded["type"])

	respFrame := encodeTLSCaptureWebSocketServerFrame(0x1, []byte(`{"type":"response.created"}`))
	require.NoError(t, fr.WriteData(1, false, respFrame))
	go func() {
		for {
			frame, err := fr.ReadFrame()
			if err != nil {
				return
			}
			switch f := frame.(type) {
			case *http2.WindowUpdateFrame:
				_ = f
			case *http2.PingFrame:
				_ = f
			case *http2.RSTStreamFrame:
				return
			case *http2.DataFrame:
				_ = f
			}
		}
	}()
	<-clientDone
}

func acceptOpenAIWSH2Handshake(t *testing.T, conn net.Conn, expectedSettings []http2.Setting) *http2.Framer {
	t.Helper()

	preface := make([]byte, len(http2.ClientPreface))
	_, err := io.ReadFull(conn, preface)
	require.NoError(t, err)
	require.Equal(t, http2.ClientPreface, string(preface))

	fr := http2.NewFramer(conn, conn)
	fr.ReadMetaHeaders = hpack.NewDecoder(4096, nil)

	var sawHeaders bool
	for !sawHeaders {
		frame, err := fr.ReadFrame()
		require.NoError(t, err)
		switch f := frame.(type) {
		case *http2.SettingsFrame:
			if !f.IsAck() {
				var settings []http2.Setting
				err := f.ForeachSetting(func(s http2.Setting) error {
					settings = append(settings, s)
					return nil
				})
				require.NoError(t, err)
				if expectedSettings != nil {
					require.Equal(t, expectedSettings, settings)
				}
			}
		case *http2.WindowUpdateFrame:
			require.Equal(t, uint32(0), f.StreamID)
			require.Equal(t, uint32(12345), f.Increment)
		case *http2.PriorityFrame:
			require.Equal(t, uint32(1), f.StreamID)
			require.Equal(t, uint8(200), f.Weight)
			require.True(t, f.Exclusive)
		case *http2.MetaHeadersFrame:
			respHeader := metaHeadersToHTTPHeader(f)
			require.Equal(t, "CONNECT", f.PseudoValue("method"))
			require.Equal(t, "websocket", f.PseudoValue("protocol"))
			require.Equal(t, "https", f.PseudoValue("scheme"))
			require.Equal(t, "api.openai.com", f.PseudoValue("authority"))
			require.Equal(t, "/v1/realtime?model=gpt-5", f.PseudoValue("path"))
			require.Equal(t, "Bearer test-token", respHeader.Get("Authorization"))
			require.Equal(t, "responses_websockets=2026-02-06", respHeader.Get("OpenAI-Beta"))
			require.Equal(t, "realtime", respHeader.Get("Sec-WebSocket-Protocol"))
			require.Equal(t, "13", respHeader.Get("Sec-WebSocket-Version"))
			require.NotEmpty(t, respHeader.Get("Sec-WebSocket-Key"))
			var block bytes.Buffer
			enc := hpack.NewEncoder(&block)
			require.NoError(t, enc.WriteField(hpack.HeaderField{Name: ":status", Value: "200"}))
			require.NoError(t, enc.WriteField(hpack.HeaderField{Name: "sec-websocket-protocol", Value: "realtime"}))
			require.NoError(t, fr.WriteHeaders(http2.HeadersFrameParam{
				StreamID:      f.StreamID,
				BlockFragment: block.Bytes(),
				EndHeaders:    true,
				EndStream:     false,
			}))
			sawHeaders = true
		}
	}
	return fr
}

func encodeTLSCaptureWebSocketServerFrameWithFIN(opcode byte, fin bool, payload []byte) []byte {
	frame := encodeTLSCaptureWebSocketServerFrame(opcode, payload)
	if len(frame) == 0 {
		return frame
	}
	if fin {
		frame[0] |= 0x80
	} else {
		frame[0] &^= 0x80
	}
	return frame
}

func mustParseOpenAIWSURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	require.NoError(t, err)
	return parsed
}
