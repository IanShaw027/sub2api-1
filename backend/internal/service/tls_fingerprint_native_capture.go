package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const (
	tlsFingerprintNativeCaptureHeaderTimeout    = 10 * time.Second
	tlsFingerprintNativeCaptureIdleTimeout      = 30 * time.Second
	tlsFingerprintNativeCaptureBodySummaryLimit = 16 << 10
)

type TLSFingerprintNativeCaptureListenerConfig struct {
	Address  string
	CertFile string
	KeyFile  string
	Service  *TLSFingerprintCaptureService
}

type TLSFingerprintNativeCaptureListener struct {
	cfg TLSFingerprintNativeCaptureListenerConfig

	mu        sync.RWMutex
	listener  net.Listener
	server    *http.Server
	rawByConn map[net.Conn][]byte
}

func NewTLSFingerprintNativeCaptureListener(cfg TLSFingerprintNativeCaptureListenerConfig) *TLSFingerprintNativeCaptureListener {
	return &TLSFingerprintNativeCaptureListener{
		cfg:       cfg,
		rawByConn: make(map[net.Conn][]byte),
	}
}

func ProvideTLSFingerprintNativeCaptureListener(cfg *config.Config, captureSvc *TLSFingerprintCaptureService) (*TLSFingerprintNativeCaptureListener, error) {
	if cfg == nil || !cfg.TLSFingerprintCapture.Enabled {
		return nil, nil
	}
	listener := NewTLSFingerprintNativeCaptureListener(TLSFingerprintNativeCaptureListenerConfig{
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

func (l *TLSFingerprintNativeCaptureListener) Start() error {
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
		NextProtos:   []string{"http/1.1"},
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

func (l *TLSFingerprintNativeCaptureListener) Stop(ctx context.Context) error {
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

func (l *TLSFingerprintNativeCaptureListener) Addr() net.Addr {
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

func (l *TLSFingerprintNativeCaptureListener) handleCapture(w http.ResponseWriter, r *http.Request) {
	if r == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	raw := l.takeRawClientHello(r.Context())
	if len(raw) == 0 {
		http.Error(w, "client hello was not captured", http.StatusBadRequest)
		return
	}
	req := TLSFingerprintCaptureNativeSubmitRequest{
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
		EventID:           strings.TrimSpace(firstNonEmptyHeader(r.Header, "X-Client-Request-Id", "x-client-request-id", "X-Request-Id", "x-request-id")),
		RequestSequence:   nativeCaptureRequestSequence(r),
		StreamID:          strings.TrimSpace(firstNonEmptyHeader(r.Header, "X-Stream-Id", "x-stream-id")),
		ClientHello:       raw,
	}
	result, err := l.cfg.Service.SubmitNativeCapture(r.Context(), req)
	if err != nil {
		slog.Warn("tls fingerprint native capture submit failed", "error", err)
		http.Error(w, "capture submission failed", http.StatusBadRequest)
		return
	}
	if result != nil && !result.Accepted {
		http.Error(w, result.IgnoredReason, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (l *TLSFingerprintNativeCaptureListener) storeRawClientHello(conn net.Conn, raw []byte) {
	if l == nil || conn == nil || len(raw) == 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rawByConn[conn] = append([]byte(nil), raw...)
}

func (l *TLSFingerprintNativeCaptureListener) deleteRawClientHello(conn net.Conn) {
	if l == nil || conn == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.rawByConn, conn)
}

func (l *TLSFingerprintNativeCaptureListener) takeRawClientHello(ctx context.Context) []byte {
	if l == nil {
		return nil
	}
	if ctx != nil {
		if conn, ok := ctx.Value(tlsFingerprintNativeCaptureConnContextKey{}).(net.Conn); ok {
			l.mu.Lock()
			defer l.mu.Unlock()
			raw := append([]byte(nil), l.rawByConn[conn]...)
			delete(l.rawByConn, conn)
			return raw
		}
	}
	return nil
}

func (l *TLSFingerprintNativeCaptureListener) loadCertificate() (tls.Certificate, error) {
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
	if nativeCaptureRequestStreaming(r) {
		return "stream"
	}
	if strings.Contains(strings.ToLower(strings.TrimSpace(r.Header.Get("Accept"))), "text/event-stream") {
		return "sse"
	}
	return "json"
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

type tlsFingerprintNativeCaptureBodyContextKey struct{}

type tlsFingerprintNativeCaptureConnContextKey struct{}

type tlsFingerprintNativeCaptureNetListener struct {
	net.Listener
	onClose func(net.Conn)
}

func (l *tlsFingerprintNativeCaptureNetListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &tlsFingerprintNativeCaptureConn{Conn: conn, onClose: l.onClose}, nil
}

type tlsFingerprintNativeCaptureConn struct {
	net.Conn

	mu      sync.Mutex
	record  []byte
	done    bool
	onClose func(net.Conn)
}

func (c *tlsFingerprintNativeCaptureConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		c.capture(p[:n])
	}
	return n, err
}

func (c *tlsFingerprintNativeCaptureConn) RawClientHello() []byte {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte(nil), c.record...)
}

func (c *tlsFingerprintNativeCaptureConn) Close() error {
	if c != nil && c.onClose != nil {
		c.onClose(c)
	}
	return c.Conn.Close()
}

func (c *tlsFingerprintNativeCaptureConn) capture(chunk []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.done || len(chunk) == 0 {
		return
	}
	for len(chunk) > 0 {
		if len(c.record) == 0 && chunk[0] != 22 {
			c.done = true
			return
		}
		need := tlsFingerprintNativeCaptureRecordLength(c.record)
		if need == 0 {
			limit := len(chunk)
			if limit > 5-len(c.record) {
				limit = 5 - len(c.record)
			}
			c.record = append(c.record, chunk[:limit]...)
			chunk = chunk[limit:]
			continue
		}
		remaining := need - len(c.record)
		if remaining <= 0 {
			c.done = true
			return
		}
		if remaining > len(chunk) {
			remaining = len(chunk)
		}
		c.record = append(c.record, chunk[:remaining]...)
		chunk = chunk[remaining:]
		if len(c.record) >= need {
			c.done = true
			return
		}
	}
}

func tlsFingerprintNativeCaptureRecordLength(record []byte) int {
	if len(record) < 5 {
		return 0
	}
	return 5 + (int(record[3]) << 8) + int(record[4])
}

func generateTLSFingerprintNativeCaptureSelfSignedCert() (tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate TLS capture key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate TLS capture serial: %w", err)
	}
	template := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "sub2api tls fingerprint capture"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate TLS capture certificate: %w", err)
	}
	keyDER := x509.MarshalPKCS1PrivateKey(key)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("load generated TLS capture certificate: %w", err)
	}
	return cert, nil
}
