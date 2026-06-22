package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
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
	tlsFingerprintNativeCaptureHeaderTimeout = 10 * time.Second
	tlsFingerprintNativeCaptureIdleTimeout   = 30 * time.Second
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
		Token:       nativeCaptureRequestToken(r),
		Platform:    nativeCaptureRequestPlatform(r),
		UserAgent:   strings.TrimSpace(r.Header.Get("User-Agent")),
		Originator:  firstNonEmptyHeader(r.Header, "Originator", "originator"),
		ClientHello: raw,
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
