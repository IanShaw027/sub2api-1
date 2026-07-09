package service

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	utls "github.com/refraction-networking/utls"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

func TestTLSCaptureHTTP2RejectsStreamsBeyondConcurrentLimit_Regression(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets: map[string]int{"openai": tlsFingerprintNativeCaptureMaxConcurrentStreams + 1},
	})
	require.NoError(t, err)

	listener := NewTLSCaptureListener(TLSCaptureListenerConfig{
		Address: "127.0.0.1:0",
		Service: svc,
	})
	require.NoError(t, listener.Start())
	t.Cleanup(func() { stopNativeCaptureListenerRegression(t, listener) })

	conn, fr := newTLSCaptureH2RegressionClient(t, listener.Addr().String())
	t.Cleanup(func() { _ = conn.Close() })

	require.NoError(t, writeTLSCaptureH2RegressionPrefaceAndSettings(conn, fr))
	for i := range tlsFingerprintNativeCaptureMaxConcurrentStreams {
		streamID := uint32(i*2 + 1)
		require.NoError(t, writeTLSCaptureH2RegressionHeadersOnly(fr, streamID, tlsCaptureH2RegressionRequest{
			Method:      http.MethodPost,
			Path:        "/capture/openai/v1/responses",
			Authority:   listener.Addr().String(),
			Token:       task.Token,
			UserAgent:   "codex_exec/0.140.0",
			Originator:  "codex_exec",
			ContentType: "application/json",
		}))
	}

	overflowStreamID := uint32(tlsFingerprintNativeCaptureMaxConcurrentStreams*2 + 1)
	require.NoError(t, writeTLSCaptureH2RegressionHeadersOnly(fr, overflowStreamID, tlsCaptureH2RegressionRequest{
		Method:      http.MethodPost,
		Path:        "/capture/openai/v1/responses",
		Authority:   listener.Addr().String(),
		Token:       task.Token,
		UserAgent:   "codex_exec/0.140.0",
		Originator:  "codex_exec",
		ContentType: "application/json",
	}))

	resp := readTLSCaptureH2RegressionResponse(t, fr, overflowStreamID)
	require.Equal(t, "429", resp.Status)
	require.Contains(t, resp.Body, "too many concurrent streams")
	require.Empty(t, repo.samples)
	require.Empty(t, repo.sessionEvents)
}

type tlsCaptureH2RegressionRequest struct {
	Method      string
	Path        string
	Authority   string
	Token       string
	UserAgent   string
	Originator  string
	ContentType string
}

type tlsCaptureH2RegressionResponse struct {
	Status  string
	Headers http.Header
	Body    string
}

func newTLSCaptureH2RegressionClient(t *testing.T, addr string) (*utls.UConn, *http2.Framer) {
	t.Helper()

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	require.NoError(t, err)

	uconn := utls.UClient(conn, &utls.Config{
		ServerName:         "localhost",
		InsecureSkipVerify: true,
		NextProtos:         []string{"h2"},
	}, utls.HelloCustom)
	require.NoError(t, uconn.ApplyPreset(nativeClientHelloSpecRegression()))
	require.NoError(t, uconn.Handshake())
	require.Equal(t, "h2", uconn.ConnectionState().NegotiatedProtocol)

	fr := http2.NewFramer(uconn, uconn)
	fr.ReadMetaHeaders = hpack.NewDecoder(4096, nil)
	return uconn, fr
}

func writeTLSCaptureH2RegressionPrefaceAndSettings(conn net.Conn, fr *http2.Framer) error {
	if _, err := io.WriteString(conn, http2.ClientPreface); err != nil {
		return err
	}
	return fr.WriteSettings()
}

func writeTLSCaptureH2RegressionHeadersOnly(fr *http2.Framer, streamID uint32, req tlsCaptureH2RegressionRequest) error {
	var headerBlock bytes.Buffer
	encoder := hpack.NewEncoder(&headerBlock)
	fields := []hpack.HeaderField{
		{Name: ":method", Value: req.Method},
		{Name: ":scheme", Value: "https"},
		{Name: ":authority", Value: req.Authority},
		{Name: ":path", Value: req.Path},
		{Name: "user-agent", Value: req.UserAgent},
		{Name: "originator", Value: req.Originator},
	}
	if req.Token != "" {
		fields = append(fields, hpack.HeaderField{Name: "authorization", Value: "Bearer " + req.Token})
	}
	if req.ContentType != "" {
		fields = append(fields, hpack.HeaderField{Name: "content-type", Value: req.ContentType})
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

func readTLSCaptureH2RegressionResponse(t *testing.T, fr *http2.Framer, streamID uint32) tlsCaptureH2RegressionResponse {
	t.Helper()

	resp := tlsCaptureH2RegressionResponse{Headers: make(http.Header)}
	var body strings.Builder
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
			if f.StreamEnded() {
				resp.Body = body.String()
				return resp
			}
		case *http2.DataFrame:
			if f.StreamID != streamID {
				continue
			}
			body.Write(f.Data())
			if f.StreamEnded() {
				resp.Body = body.String()
				return resp
			}
		}
	}
}

func stopNativeCaptureListenerRegression(t *testing.T, listener *TLSCaptureListener) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, listener.Stop(ctx))
}

func nativeClientHelloSpecRegression() *utls.ClientHelloSpec {
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
	}
}
