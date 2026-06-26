//go:build unit

package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	utls "github.com/refraction-networking/utls"
	"github.com/stretchr/testify/require"
)

func TestTLSCaptureListenerHTTP1JSONSuccess(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets:    map[string]int{"openai": 1},
		UAKeywords: []string{"codex"},
	})
	require.NoError(t, err)

	listener := NewTLSCaptureListener(TLSCaptureListenerConfig{
		Address: "127.0.0.1:0",
		Service: svc,
	})
	require.NoError(t, listener.Start())
	t.Cleanup(func() { stopNativeCaptureListener(t, listener) })

	conn, err := net.DialTimeout("tcp", listener.Addr().String(), 5*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	uconn := utls.UClient(conn, &utls.Config{
		ServerName:         "localhost",
		InsecureSkipVerify: true,
		NextProtos:         []string{"http/1.1"},
	}, utls.HelloCustom)
	require.NoError(t, uconn.ApplyPreset(nativeClientHelloSpecHTTP1Only()))
	require.NoError(t, uconn.Handshake())

	_, err = fmt.Fprintf(
		uconn,
		"POST /capture/openai/v1/responses HTTP/1.1\r\nHost: %s\r\nAuthorization: Bearer %s\r\nUser-Agent: codex_exec/0.140.0\r\nOriginator: codex_exec\r\nContent-Length: 2\r\nConnection: close\r\n\r\n{}",
		listener.Addr().String(),
		task.Token,
	)
	require.NoError(t, err)

	resp, err := http.ReadResponse(bufio.NewReader(uconn), nil)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	require.Contains(t, string(body), "\"status\":\"completed\"")
	require.Contains(t, string(body), "\"accepted\":true")
	require.Len(t, repo.samples, 1)
	require.Equal(t, "openai", repo.samples[0].Platform)
}

func TestTLSCaptureListenerHTTP1StreamSuccess(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets:    map[string]int{"openai": 1},
		UAKeywords: []string{"codex"},
	})
	require.NoError(t, err)

	listener := NewTLSCaptureListener(TLSCaptureListenerConfig{
		Address: "127.0.0.1:0",
		Service: svc,
	})
	require.NoError(t, listener.Start())
	t.Cleanup(func() { stopNativeCaptureListener(t, listener) })

	conn, err := net.DialTimeout("tcp", listener.Addr().String(), 5*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	uconn := utls.UClient(conn, &utls.Config{
		ServerName:         "localhost",
		InsecureSkipVerify: true,
		NextProtos:         []string{"http/1.1"},
	}, utls.HelloCustom)
	require.NoError(t, uconn.ApplyPreset(nativeClientHelloSpecHTTP1Only()))
	require.NoError(t, uconn.Handshake())

	payload := `{"model":"gpt-5.4","stream":true,"input":"capture"}`
	_, err = fmt.Fprintf(
		uconn,
		"POST /capture/openai/v1/responses HTTP/1.1\r\nHost: %s\r\nAuthorization: Bearer %s\r\nUser-Agent: codex_exec/0.140.0\r\nOriginator: codex_exec\r\nAccept: text/event-stream\r\ncontent-type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
		listener.Addr().String(),
		task.Token,
		len(payload),
		payload,
	)
	require.NoError(t, err)

	resp, err := http.ReadResponse(bufio.NewReader(uconn), nil)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")
	require.Contains(t, string(body), "event: response.created")
	require.Contains(t, string(body), "event: response.completed")
	require.Len(t, repo.samples, 1)
}

func TestTLSCaptureListenerHTTP1PrewarmStreamSuppressesVisibleOutput(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets:    map[string]int{"openai": 1},
		UAKeywords: []string{"codex"},
	})
	require.NoError(t, err)

	listener := NewTLSCaptureListener(TLSCaptureListenerConfig{
		Address: "127.0.0.1:0",
		Service: svc,
	})
	require.NoError(t, listener.Start())
	t.Cleanup(func() { stopNativeCaptureListener(t, listener) })

	conn, err := net.DialTimeout("tcp", listener.Addr().String(), 5*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	uconn := utls.UClient(conn, &utls.Config{
		ServerName:         "localhost",
		InsecureSkipVerify: true,
		NextProtos:         []string{"http/1.1"},
	}, utls.HelloCustom)
	require.NoError(t, uconn.ApplyPreset(nativeClientHelloSpecHTTP1Only()))
	require.NoError(t, uconn.Handshake())

	payload := `{"model":"gpt-5.4","generate":false,"input":"capture"}`
	_, err = fmt.Fprintf(
		uconn,
		"POST /capture/openai/v1/responses HTTP/1.1\r\nHost: %s\r\nAuthorization: Bearer %s\r\nUser-Agent: codex_exec/0.140.0\r\nOriginator: codex_exec\r\nAccept: text/event-stream\r\ncontent-type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
		listener.Addr().String(),
		task.Token,
		len(payload),
		payload,
	)
	require.NoError(t, err)

	resp, err := http.ReadResponse(bufio.NewReader(uconn), nil)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")
	require.Contains(t, string(body), "event: response.created")
	require.Contains(t, string(body), "event: response.completed")
	require.NotContains(t, string(body), "output_text")
	require.Len(t, repo.samples, 1)
	require.Equal(t, "prewarm", repo.samples[0].ResponseMode)
	require.False(t, repo.samples[0].Streaming)
}
