package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	tlsfpHTTP2 "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/http2"
	tlsfpTransport "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/transport"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

func NewTLSCaptureListener(cfg TLSCaptureListenerConfig) *TLSCaptureListener {
	return &TLSCaptureListener{
		cfg:       cfg,
		rawByConn: make(map[net.Conn][]byte),
	}
}

func ProvideTLSCaptureListener(cfg *config.Config, captureSvc *TLSFingerprintCaptureService) (*TLSCaptureListener, error) {
	if cfg == nil || !cfg.TLSFingerprintCapture.Enabled {
		return nil, nil
	}
	listener := NewTLSCaptureListener(TLSCaptureListenerConfig{
		Address:  cfg.TLSFingerprintCapture.Address(),
		CertFile: cfg.TLSFingerprintCapture.CertFile,
		KeyFile:  cfg.TLSFingerprintCapture.KeyFile,
		Service:  captureSvc,
	})
	if err := listener.Start(); err != nil {
		return nil, err
	}
	return listener, nil
}

func (l *TLSCaptureListener) Start() error {
	if l == nil {
		return nil
	}
	if l.cfg.Service == nil {
		return fmt.Errorf("tls fingerprint capture service is not configured")
	}
	addr := strings.TrimSpace(l.cfg.Address)
	if addr == "" {
		return nil
	}

	cert, err := l.loadCertificate()
	if err != nil {
		return err
	}
	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h2", "http/1.1"},
	}
	tlsConfig.GetConfigForClient = func(info *tls.ClientHelloInfo) (*tls.Config, error) {
		if info != nil && info.Conn != nil {
			if captureConn, ok := info.Conn.(*tlsFingerprintNativeCaptureConn); ok {
				l.storeRawClientHello(info.Conn, captureConn.RawClientHello())
			}
		}
		return tlsConfig, nil
	}

	baseListener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("bind native TLS fingerprint capture listener %s: %w", addr, err)
	}
	captureListener := &tlsFingerprintNativeCaptureNetListener{
		Listener: baseListener,
		onClose:  l.deleteRawClientHello,
	}
	tlsListener := tls.NewListener(captureListener, tlsConfig)

	server := &http.Server{
		Handler:           http.HandlerFunc(l.handleCapture),
		ReadHeaderTimeout: tlsFingerprintNativeCaptureHeaderTimeout,
		IdleTimeout:       tlsFingerprintNativeCaptureIdleTimeout,
	}
	server.TLSNextProto = map[string]func(*http.Server, *tls.Conn, http.Handler){
		"h2": func(_ *http.Server, conn *tls.Conn, _ http.Handler) {
			l.handleCaptureH2(conn)
		},
	}
	server.ConnContext = func(ctx context.Context, conn net.Conn) context.Context {
		if tlsConn, ok := conn.(*tls.Conn); ok {
			conn = tlsConn.NetConn()
		}
		return context.WithValue(ctx, tlsFingerprintNativeCaptureConnContextKey{}, conn)
	}
	server.ConnState = func(conn net.Conn, state http.ConnState) {
		if state != http.StateClosed && state != http.StateHijacked {
			return
		}
		if tlsConn, ok := conn.(*tls.Conn); ok {
			conn = tlsConn.NetConn()
		}
		l.deleteRawClientHello(conn)
	}

	l.mu.Lock()
	l.listener = tlsListener
	l.server = server
	l.mu.Unlock()

	go func() {
		if err := server.Serve(tlsListener); err != nil && err != http.ErrServerClosed {
			slog.Error("tls fingerprint native capture listener stopped unexpectedly", "addr", addr, "error", err)
		}
	}()
	slog.Info("tls fingerprint native capture listener started", "addr", baseListener.Addr().String())
	return nil
}

func (l *TLSCaptureListener) Stop(ctx context.Context) error {
	if l == nil {
		return nil
	}
	l.mu.RLock()
	server := l.server
	l.mu.RUnlock()
	if server == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return server.Shutdown(ctx)
}

func (l *TLSCaptureListener) Addr() net.Addr {
	if l == nil {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.listener == nil {
		return nil
	}
	return l.listener.Addr()
}

func (l *TLSCaptureListener) storeRawClientHello(conn net.Conn, raw []byte) {
	if l == nil || conn == nil || len(raw) == 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rawByConn[conn] = append([]byte(nil), raw...)
}

func (l *TLSCaptureListener) deleteRawClientHello(conn net.Conn) {
	if l == nil || conn == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.rawByConn, conn)
}

func (l *TLSCaptureListener) takeRawClientHello(ctx context.Context) []byte {
	if l == nil {
		return nil
	}
	if ctx != nil {
		if conn, ok := ctx.Value(tlsFingerprintNativeCaptureConnContextKey{}).(net.Conn); ok {
			l.mu.Lock()
			defer l.mu.Unlock()
			return append([]byte(nil), l.rawByConn[conn]...)
		}
	}
	return nil
}

func (l *TLSCaptureListener) loadCertificate() (tls.Certificate, error) {
	certFile := strings.TrimSpace(l.cfg.CertFile)
	keyFile := strings.TrimSpace(l.cfg.KeyFile)
	if certFile != "" || keyFile != "" {
		if certFile == "" || keyFile == "" {
			return tls.Certificate{}, fmt.Errorf("both TLS capture cert_file and key_file are required")
		}
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("load TLS capture certificate: %w", err)
		}
		return cert, nil
	}
	return generateTLSFingerprintNativeCaptureSelfSignedCert()
}

type tlsCaptureH2StreamState struct {
	method                    string
	path                      string
	authority                 string
	protocol                  string
	header                    http.Header
	body                      bytes.Buffer
	websocketSessionID        string
	websocketProtocolSelected string
	websocketHandshakeSent    bool
	websocketRequestSequence  int
	websocketReadBuffer       []byte
	websocketMessageBuffer    []byte
	websocketFragmentOpcode   byte
}

func (l *TLSCaptureListener) handleCaptureH2(conn *tls.Conn) {
	if l == nil || conn == nil {
		return
	}
	defer func() { _ = conn.Close() }()

	baseConn := conn.NetConn()
	raw := l.rawClientHelloForH2Conn(baseConn)
	if len(raw) == 0 {
		return
	}

	fr := http2.NewFramer(conn, conn)
	fr.ReadMetaHeaders = hpack.NewDecoder(4096, nil)
	if err := fr.WriteSettings(http2.Setting{ID: http2.SettingEnableConnectProtocol, Val: 1}); err != nil {
		return
	}

	preface := make([]byte, len(http2.ClientPreface))
	if _, err := io.ReadFull(conn, preface); err != nil || !bytes.Equal(preface, []byte(http2.ClientPreface)) {
		return
	}

	sessionID, err := ensureTLSCaptureSessionID("")
	if err != nil {
		return
	}
	streams := map[uint32]*tlsCaptureH2StreamState{}
	http2Fingerprint := ""

	for {
		if err := conn.SetReadDeadline(time.Now().Add(tlsFingerprintNativeCaptureIdleTimeout)); err != nil {
			return
		}
		frame, err := fr.ReadFrame()
		if err != nil {
			return
		}
		switch f := frame.(type) {
		case *http2.SettingsFrame:
			if f.IsAck() {
				continue
			}
			settings := map[uint16]uint32{}
			_ = f.ForeachSetting(func(s http2.Setting) error {
				settings[uint16(s.ID)] = s.Val
				return nil
			})
			http2Fingerprint = tlsfpHTTP2.FingerprintFromSettings(settings)
			if err := fr.WriteSettingsAck(); err != nil {
				return
			}
		case *http2.MetaHeadersFrame:
			state := streams[f.StreamID]
			if state == nil {
				state = &tlsCaptureH2StreamState{header: make(http.Header)}
				streams[f.StreamID] = state
			}
			state.method = f.PseudoValue("method")
			state.path = f.PseudoValue("path")
			state.authority = f.PseudoValue("authority")
			state.protocol = f.PseudoValue("protocol")
			for _, field := range f.RegularFields() {
				state.header.Add(field.Name, field.Value)
			}
			if isTLSCaptureH2WebSocketConnect(state) {
				if err := l.initCaptureH2WebSocketStream(fr, f.StreamID, state); err != nil {
					return
				}
				if f.StreamEnded() {
					delete(streams, f.StreamID)
				}
				continue
			}
			if f.StreamEnded() {
				if err := l.finishCaptureH2Stream(fr, f.StreamID, conn, raw, sessionID, http2Fingerprint, state); err != nil {
					return
				}
				delete(streams, f.StreamID)
			}
		case *http2.DataFrame:
			if err := writeTLSCaptureH2WindowUpdates(fr, f.StreamID, len(f.Data())); err != nil {
				return
			}
			state := streams[f.StreamID]
			if state == nil {
				state = &tlsCaptureH2StreamState{header: make(http.Header)}
				streams[f.StreamID] = state
			}
			if isTLSCaptureH2WebSocketConnect(state) {
				closeStream, err := l.handleCaptureH2WebSocketDataFrame(fr, f.StreamID, raw, http2Fingerprint, state, f.Data())
				if err != nil {
					return
				}
				if closeStream || f.StreamEnded() {
					delete(streams, f.StreamID)
				}
				continue
			}
			_, _ = state.body.Write(f.Data())
			if f.StreamEnded() {
				if err := l.finishCaptureH2Stream(fr, f.StreamID, conn, raw, sessionID, http2Fingerprint, state); err != nil {
					return
				}
				delete(streams, f.StreamID)
			}
		case *http2.PingFrame:
			if !f.Flags.Has(http2.FlagPingAck) {
				if err := fr.WritePing(true, f.Data); err != nil {
					return
				}
			}
		case *http2.RSTStreamFrame:
			delete(streams, f.StreamID)
		}
	}
}

func (l *TLSCaptureListener) finishCaptureH2Stream(
	fr *http2.Framer,
	streamID uint32,
	conn *tls.Conn,
	raw []byte,
	sessionID string,
	http2Fingerprint string,
	state *tlsCaptureH2StreamState,
) error {
	if l == nil || l.cfg.Service == nil || fr == nil || conn == nil || state == nil {
		return nil
	}
	requestURL := buildTLSCaptureRequestURL(state.authority, state.path)
	req := &http.Request{
		Method:     defaultString(strings.TrimSpace(state.method), http.MethodGet),
		URL:        requestURL,
		Host:       state.authority,
		Header:     state.header.Clone(),
		RemoteAddr: conn.RemoteAddr().String(),
		TLS: &tls.ConnectionState{
			NegotiatedProtocol: "h2",
		},
		Body: io.NopCloser(strings.NewReader(state.body.String())),
	}
	submitReq := buildTLSCaptureHTTP1SubmitRequest(req, raw)
	submitReq.Transport = string(tlsfpTransport.H2)
	if strings.TrimSpace(submitReq.SessionID) == "" {
		submitReq.SessionID = sessionID
	}
	submitReq.HTTP2Fingerprint = http2Fingerprint
	submitReq.StreamID = fmt.Sprintf("%d", streamID)

	result, err := l.cfg.Service.SubmitNativeCapture(context.Background(), submitReq)

	rec := httptestNewRecorder()
	if err != nil {
		http.Error(rec, "capture submission failed", http.StatusBadRequest)
	} else if nativeCaptureShouldReturnStream(submitReq) {
		writeNativeCaptureSSEResponse(rec)
	} else {
		writeNativeCaptureJSONSuccess(rec, result)
	}
	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return readErr
	}
	return writeTLSCaptureH2Response(fr, streamID, resp.StatusCode, resp.Header, body)
}

func buildTLSCaptureRequestURL(authority, rawPath string) *url.URL {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		rawPath = "/"
	}
	parsed, err := url.ParseRequestURI(rawPath)
	if err != nil {
		return &url.URL{
			Scheme: "https",
			Host:   authority,
			Path:   rawPath,
		}
	}
	parsed.Scheme = "https"
	parsed.Host = authority
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed
}

func writeTLSCaptureH2WindowUpdates(fr *http2.Framer, streamID uint32, size int) error {
	if fr == nil || size <= 0 {
		return nil
	}
	increment := uint32(size)
	if err := fr.WriteWindowUpdate(0, increment); err != nil {
		return err
	}
	if streamID == 0 {
		return nil
	}
	return fr.WriteWindowUpdate(streamID, increment)
}

func (l *TLSCaptureListener) rawClientHelloForH2Conn(conn net.Conn) []byte {
	if captureConn, ok := conn.(*tlsFingerprintNativeCaptureConn); ok {
		raw := captureConn.RawClientHello()
		l.deleteRawClientHello(conn)
		return raw
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	raw := append([]byte(nil), l.rawByConn[conn]...)
	delete(l.rawByConn, conn)
	return raw
}

func writeTLSCaptureH2Response(fr *http2.Framer, streamID uint32, statusCode int, header http.Header, body []byte) error {
	if err := writeTLSCaptureH2Headers(fr, streamID, statusCode, header, len(body) == 0); err != nil {
		return err
	}
	if len(body) > 0 {
		return fr.WriteData(streamID, true, body)
	}
	return nil
}

func writeTLSCaptureH2Headers(fr *http2.Framer, streamID uint32, statusCode int, header http.Header, endStream bool) error {
	var block bytes.Buffer
	encoder := hpack.NewEncoder(&block)
	if err := encoder.WriteField(hpack.HeaderField{Name: ":status", Value: fmt.Sprintf("%d", statusCode)}); err != nil {
		return err
	}
	for key, values := range header {
		name := strings.ToLower(strings.TrimSpace(key))
		if name == "" {
			continue
		}
		for _, value := range values {
			if err := encoder.WriteField(hpack.HeaderField{Name: name, Value: value}); err != nil {
				return err
			}
		}
	}
	if err := fr.WriteHeaders(http2.HeadersFrameParam{
		StreamID:      streamID,
		EndHeaders:    true,
		EndStream:     endStream,
		BlockFragment: block.Bytes(),
	}); err != nil {
		return err
	}
	return nil
}

type tlsCaptureResponseRecorder struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func httptestNewRecorder() *tlsCaptureResponseRecorder {
	return &tlsCaptureResponseRecorder{
		header: make(http.Header),
		status: http.StatusOK,
	}
}

func (r *tlsCaptureResponseRecorder) Header() http.Header {
	return r.header
}

func (r *tlsCaptureResponseRecorder) Write(data []byte) (int, error) {
	return r.body.Write(data)
}

func (r *tlsCaptureResponseRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
}

func (r *tlsCaptureResponseRecorder) Result() *http.Response {
	header := make(http.Header, len(r.header))
	for key, values := range r.header {
		header[key] = append([]string(nil), values...)
	}
	return &http.Response{
		StatusCode: r.status,
		Status:     fmt.Sprintf("%d %s", r.status, http.StatusText(r.status)),
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(r.body.Bytes())),
	}
}
