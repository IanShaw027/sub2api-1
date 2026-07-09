package service

import (
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	tlsFingerprintNativeCaptureHeaderTimeout        = 10 * time.Second
	tlsFingerprintNativeCaptureReadTimeout          = 30 * time.Second
	tlsFingerprintNativeCaptureIdleTimeout          = 30 * time.Second
	tlsFingerprintNativeCaptureBodyLimit            = 1 << 20
	tlsFingerprintNativeCaptureBodySummaryLimit     = 16 << 10
	tlsFingerprintNativeCaptureMaxConcurrentStreams = 32
	tlsFingerprintNativeCaptureMaxFrameSize         = 1 << 20
)

type TLSCaptureListenerConfig struct {
	Address  string
	CertFile string
	KeyFile  string
	Service  *TLSFingerprintCaptureService
}

type TLSCaptureListener struct {
	cfg TLSCaptureListenerConfig

	mu        sync.RWMutex
	listener  net.Listener
	server    *http.Server
	rawByConn map[net.Conn][]byte
}

type tlsFingerprintNativeCaptureBodyContextKey struct{}

type tlsFingerprintNativeCaptureConnContextKey struct{}

type tlsFingerprintNativeCaptureNetListener struct {
	net.Listener
	onClose func(net.Conn)
}

type tlsFingerprintNativeCaptureConn struct {
	net.Conn

	mu      sync.Mutex
	record  []byte
	done    bool
	onClose func(net.Conn)
}
