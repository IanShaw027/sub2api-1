//go:build unit

package service

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	utls "github.com/refraction-networking/utls"
	"github.com/stretchr/testify/require"
)

func TestNativeTLSCapturePersistsRawClientHelloAndMetadata(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets:    map[string]int{"openai": 1},
		UAKeywords: []string{"codex"},
	})
	require.NoError(t, err)

	rawClientHello := nativeClientHelloBytes(t)
	result, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		UserAgent:   "codex_cli_rs/0.140.0",
		Originator:  "codex_cli_rs",
		ClientHello: rawClientHello,
	})
	require.NoError(t, err)
	require.True(t, result.Accepted)
	require.False(t, result.Duplicate)
	require.NotNil(t, result.Sample)
	require.Equal(t, "codex_cli_rs/0.140.0", result.Sample.UserAgent)
	require.Equal(t, "codex_cli_rs", result.Sample.Originator)
	require.NotEmpty(t, result.Sample.RawClientHello)
	require.Equal(t, rawClientHello, result.Sample.RawClientHello)
	require.NotEmpty(t, result.Sample.FingerprintHash)
	require.NotNil(t, result.Sample.Profile)
	require.Equal(t, []uint16{utls.VersionTLS13, utls.VersionTLS12}, result.Sample.Profile.SupportedVersions)
	require.Contains(t, result.Sample.Profile.Extensions, uint16(0))
	require.Equal(t, 1, result.Counts["openai"])
}

func TestNativeTLSCaptureRejectsEmptyClientHello(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets: map[string]int{"openai": 1},
	})
	require.NoError(t, err)

	result, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:      task.Token,
		Platform:   "openai",
		UserAgent:  "codex_cli_rs/0.140.0",
		Originator: "codex_cli_rs",
	})
	require.NoError(t, err)
	require.False(t, result.Accepted)
	require.Equal(t, "client_hello_required", result.IgnoredReason)
}

func TestNativeTLSCaptureListenerCapturesClientHelloAndHeaders(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets:    map[string]int{"openai": 1},
		UAKeywords: []string{"codex"},
	})
	require.NoError(t, err)

	listener := NewTLSFingerprintNativeCaptureListener(TLSFingerprintNativeCaptureListenerConfig{
		Address: "127.0.0.1:0",
		Service: svc,
	})
	require.NoError(t, listener.Start())
	t.Cleanup(func() { stopNativeCaptureListener(t, listener) })

	conn, err := net.DialTimeout("tcp", listener.Addr().String(), 5*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	defer func() { _ = conn.Close() }()

	uconn := utls.UClient(conn, &utls.Config{
		ServerName:         "localhost",
		InsecureSkipVerify: true,
		NextProtos:         []string{"h2", "http/1.1"},
	}, utls.HelloCustom)
	require.NoError(t, uconn.ApplyPreset(nativeClientHelloSpec()))
	require.NoError(t, uconn.Handshake())
	_, err = fmt.Fprintf(
		uconn,
		"GET /capture?token=%s&platform=openai HTTP/1.1\r\nHost: %s\r\nUser-Agent: codex_cli_rs/0.140.0\r\nOriginator: codex_cli_rs\r\nConnection: close\r\n\r\n",
		task.Token,
		listener.Addr().String(),
	)
	require.NoError(t, err)
	resp, err := http.ReadResponse(bufio.NewReader(uconn), nil)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	require.Eventually(t, func() bool {
		samples, err := svc.ListSamplesByTask(context.Background(), task.ID)
		if err != nil || len(samples) != 1 {
			return false
		}
		return samples[0].UserAgent == "codex_cli_rs/0.140.0" &&
			samples[0].Originator == "codex_cli_rs" &&
			len(samples[0].RawClientHello) > 0 &&
			samples[0].Profile != nil
	}, time.Second, 10*time.Millisecond)
}

func TestNativeTLSCaptureListenerAcceptsBearerTokenAndPathPlatform(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets:    map[string]int{"openai": 1},
		UAKeywords: []string{"codex"},
	})
	require.NoError(t, err)

	listener := NewTLSFingerprintNativeCaptureListener(TLSFingerprintNativeCaptureListenerConfig{
		Address: "127.0.0.1:0",
		Service: svc,
	})
	require.NoError(t, listener.Start())
	t.Cleanup(func() { stopNativeCaptureListener(t, listener) })

	conn, err := net.DialTimeout("tcp", listener.Addr().String(), 5*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	defer func() { _ = conn.Close() }()

	uconn := utls.UClient(conn, &utls.Config{
		ServerName:         "localhost",
		InsecureSkipVerify: true,
		NextProtos:         []string{"h2", "http/1.1"},
	}, utls.HelloCustom)
	require.NoError(t, uconn.ApplyPreset(nativeClientHelloSpec()))
	require.NoError(t, uconn.Handshake())
	_, err = fmt.Fprintf(
		uconn,
		"POST /capture/openai/v1/responses HTTP/1.1\r\nHost: %s\r\nAuthorization: Bearer %s\r\nUser-Agent: codex_exec/0.140.0\r\noriginator: codex_exec\r\nContent-Length: 2\r\nConnection: close\r\n\r\n{}",
		listener.Addr().String(),
		task.Token,
	)
	require.NoError(t, err)
	resp, err := http.ReadResponse(bufio.NewReader(uconn), nil)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	samples, err := svc.ListSamplesByTask(context.Background(), task.ID)
	require.NoError(t, err)
	require.Len(t, samples, 1)
	require.Equal(t, "openai", samples[0].Platform)
	require.Equal(t, "codex_exec/0.140.0", samples[0].UserAgent)
	require.Equal(t, "codex_exec", samples[0].Originator)
	require.NotEmpty(t, samples[0].RawClientHello)
}

func TestNativeTLSCaptureListenerDoesNotLeakInternalSubmitErrors(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)
	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets:    map[string]int{"openai": 1},
		UAKeywords: []string{"codex"},
	})
	require.NoError(t, err)
	repo.FailGetRunningTaskByToken(errors.New("db password secret-host.internal connection refused"))

	listener := NewTLSFingerprintNativeCaptureListener(TLSFingerprintNativeCaptureListenerConfig{
		Address: "127.0.0.1:0",
		Service: svc,
	})
	require.NoError(t, listener.Start())
	t.Cleanup(func() { stopNativeCaptureListener(t, listener) })

	conn, err := net.DialTimeout("tcp", listener.Addr().String(), 5*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	defer func() { _ = conn.Close() }()

	uconn := utls.UClient(conn, &utls.Config{
		ServerName:         "localhost",
		InsecureSkipVerify: true,
		NextProtos:         []string{"h2", "http/1.1"},
	}, utls.HelloCustom)
	require.NoError(t, uconn.ApplyPreset(nativeClientHelloSpec()))
	require.NoError(t, uconn.Handshake())
	_, err = fmt.Fprintf(
		uconn,
		"GET /capture?token=%s&platform=openai HTTP/1.1\r\nHost: %s\r\nUser-Agent: codex_cli_rs/0.140.0\r\nConnection: close\r\n\r\n",
		task.Token,
		listener.Addr().String(),
	)
	require.NoError(t, err)
	resp, err := http.ReadResponse(bufio.NewReader(uconn), nil)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Contains(t, string(body), "capture submission failed")
	require.NotContains(t, string(body), "db password")
	require.NotContains(t, string(body), "secret-host.internal")
}

func TestNativeTLSCaptureConnCloseClearsStoredClientHello(t *testing.T) {
	listener := NewTLSFingerprintNativeCaptureListener(TLSFingerprintNativeCaptureListenerConfig{})
	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() { _ = clientConn.Close() })

	captureConn := &tlsFingerprintNativeCaptureConn{
		Conn:    serverConn,
		onClose: listener.deleteRawClientHello,
	}
	listener.storeRawClientHello(captureConn, []byte{22, 3, 1, 0, 0})
	require.Equal(t, 1, nativeCaptureRawClientHelloEntryCount(listener))

	require.NoError(t, captureConn.Close())
	require.Equal(t, 0, nativeCaptureRawClientHelloEntryCount(listener))
}

func TestProvideTLSFingerprintNativeCaptureListenerHonorsEnabledConfig(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	disabled, err := ProvideTLSFingerprintNativeCaptureListener(nil, svc)
	require.NoError(t, err)
	require.Nil(t, disabled)

	enabled, err := ProvideTLSFingerprintNativeCaptureListener(&config.Config{
		TLSFingerprintCapture: config.TLSFingerprintCaptureConfig{
			Enabled: true,
			Host:    "127.0.0.1",
			Port:    0,
		},
	}, svc)
	require.NoError(t, err)
	require.NotNil(t, enabled)
	require.NotNil(t, enabled.Addr())
	stopNativeCaptureListener(t, enabled)
}

func TestNativeTLSCaptureListenerConfiguresIdleTimeout(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	listener := NewTLSFingerprintNativeCaptureListener(TLSFingerprintNativeCaptureListenerConfig{
		Address: "127.0.0.1:0",
		Service: svc,
	})
	require.NoError(t, listener.Start())
	t.Cleanup(func() { stopNativeCaptureListener(t, listener) })

	listener.mu.RLock()
	server := listener.server
	listener.mu.RUnlock()
	require.NotNil(t, server)
	require.Equal(t, 30*time.Second, server.IdleTimeout)
}

func stopNativeCaptureListener(t *testing.T, listener *TLSFingerprintNativeCaptureListener) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, listener.Stop(ctx))
}

func nativeCaptureRawClientHelloEntryCount(listener *TLSFingerprintNativeCaptureListener) int {
	listener.mu.RLock()
	defer listener.mu.RUnlock()
	return len(listener.rawByConn)
}

func nativeClientHelloBytes(t *testing.T) []byte {
	t.Helper()

	spec := nativeClientHelloSpec()
	uconn := utls.UClient(&net.TCPConn{}, &utls.Config{ServerName: "cloud.example"}, utls.HelloCustom)
	require.NoError(t, uconn.ApplyPreset(spec))
	require.NoError(t, uconn.MarshalClientHello())
	require.NotEmpty(t, uconn.HandshakeState.Hello.Raw)

	return prependTLSRecordHeader(uconn.HandshakeState.Hello.Raw, utls.VersionTLS12)
}

func nativeClientHelloSpec() *utls.ClientHelloSpec {
	return &utls.ClientHelloSpec{
		CipherSuites: []uint16{
			utls.TLS_AES_128_GCM_SHA256,
			utls.TLS_AES_256_GCM_SHA384,
			utls.TLS_CHACHA20_POLY1305_SHA256,
			utls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
		CompressionMethods: []uint8{0},
		Extensions: []utls.TLSExtension{
			&utls.SNIExtension{},
			&utls.SupportedCurvesExtension{Curves: []utls.CurveID{utls.X25519, utls.CurveP256}},
			&utls.SupportedPointsExtension{SupportedPoints: []uint8{0}},
			&utls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []utls.SignatureScheme{0x0403, 0x0804, 0x0401}},
			&utls.ALPNExtension{AlpnProtocols: []string{"h2", "http/1.1"}},
			&utls.SupportedVersionsExtension{Versions: []uint16{utls.VersionTLS13, utls.VersionTLS12}},
			&utls.PSKKeyExchangeModesExtension{Modes: []uint8{utls.PskModeDHE}},
			&utls.KeyShareExtension{KeyShares: []utls.KeyShare{{Group: utls.X25519}}},
		},
		TLSVersMin: utls.VersionTLS12,
		TLSVersMax: utls.VersionTLS13,
	}
}

func prependTLSRecordHeader(handshake []byte, recordVersion uint16) []byte {
	record := make([]byte, 5+len(handshake))
	record[0] = 22
	record[1] = byte(recordVersion >> 8)
	record[2] = byte(recordVersion)
	record[3] = byte(len(handshake) >> 8)
	record[4] = byte(len(handshake))
	copy(record[5:], handshake)
	return record
}
