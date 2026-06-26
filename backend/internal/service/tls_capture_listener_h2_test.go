//go:build unit

package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	tlsfpTransport "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/transport"
	utls "github.com/refraction-networking/utls"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

func TestTLSCaptureHTTP2JSONSuccessCapturesFingerprint(t *testing.T) {
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

	conn, fr := newTLSCaptureH2Client(t, listener.Addr().String())
	t.Cleanup(func() { _ = conn.Close() })

	settings := []http2.Setting{
		{ID: http2.SettingHeaderTableSize, Val: 4096},
		{ID: http2.SettingMaxConcurrentStreams, Val: 100},
		{ID: http2.SettingInitialWindowSize, Val: 65535},
	}
	require.NoError(t, writeTLSCaptureH2PrefaceAndSettings(conn, fr, settings...))
	require.NoError(t, writeTLSCaptureH2Request(fr, 1, tlsCaptureH2Request{
		Method:      http.MethodPost,
		Path:        "/capture/openai/v1/responses",
		Authority:   listener.Addr().String(),
		Token:       task.Token,
		UserAgent:   "codex_exec/0.140.0",
		Originator:  "codex_exec",
		ContentType: "application/json",
		Body:        `{}`,
	}))

	resp := readTLSCaptureH2Response(t, fr, 1)
	require.Equal(t, "200", resp.Status)
	require.Contains(t, resp.Headers.Get("content-type"), "application/json")
	require.Contains(t, resp.Body, `"status":"completed"`)
	require.Contains(t, resp.Body, `"accepted":true`)

	require.Len(t, repo.samples, 1)
	require.Len(t, repo.sessions, 1)
	require.Len(t, repo.sessionEvents, 1)
	require.Equal(t, string(tlsfpTransport.H2), repo.samples[0].Transport)
	require.Equal(t, "1:4096,3:100,4:65535", repo.samples[0].HTTP2Fingerprint)
	require.Equal(t, "h2", repo.sessions[0].ALPNNegotiated)
	require.Equal(t, string(tlsfpTransport.H2), repo.sessionEvents[0].Transport)
	require.Equal(t, "1", repo.sessionEvents[0].StreamID)
	require.Equal(t, "replayable_sample_recorded", repo.sessionEvents[0].EventType)
	require.Equal(t, repo.sessions[0].SessionID, repo.sessionEvents[0].SessionID)
}

func TestTLSCaptureHTTP2QueryParametersAreParsed(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets: map[string]int{"openai": 1},
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
	require.NoError(t, writeTLSCaptureH2Request(fr, 1, tlsCaptureH2Request{
		Method:      http.MethodPost,
		Path:        "/capture/openai/v1/responses?token=" + task.Token + "&stream=true&model=gpt-query",
		Authority:   listener.Addr().String(),
		UserAgent:   "codex_exec/0.140.0",
		Originator:  "codex_exec",
		ContentType: "application/json",
		Body:        `{}`,
	}))

	resp := readTLSCaptureH2Response(t, fr, 1)
	require.Equal(t, "200", resp.Status)
	require.Len(t, repo.samples, 1)
	require.Equal(t, "/v1/responses", repo.samples[0].RequestPath)
	require.True(t, repo.samples[0].Streaming)
	require.Equal(t, "gpt-query", repo.samples[0].Model)
}

func TestTLSCaptureHTTP2DistinctSessionHeadersCreateDistinctSessions(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets: map[string]int{"openai": 2},
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
	require.NoError(t, writeTLSCaptureH2Request(fr, 1, tlsCaptureH2Request{
		Method:      http.MethodPost,
		Path:        "/capture/openai/v1/responses",
		Authority:   listener.Addr().String(),
		Token:       task.Token,
		UserAgent:   "codex_exec/0.140.0",
		Originator:  "codex_exec",
		SessionID:   "h2-session-a",
		ContentType: "application/json",
		Body:        `{"input":"a"}`,
	}))
	first := readTLSCaptureH2Response(t, fr, 1)
	require.Equal(t, "200", first.Status)

	require.NoError(t, writeTLSCaptureH2Request(fr, 3, tlsCaptureH2Request{
		Method:      http.MethodPost,
		Path:        "/capture/openai/v1/responses",
		Authority:   listener.Addr().String(),
		Token:       task.Token,
		UserAgent:   "codex_exec/0.140.0",
		Originator:  "codex_exec",
		SessionID:   "h2-session-b",
		ContentType: "application/json",
		Body:        `{"input":"b"}`,
	}))
	second := readTLSCaptureH2Response(t, fr, 3)
	require.Equal(t, "200", second.Status)

	require.Len(t, repo.sessions, 2)
	require.Equal(t, "h2-session-a", repo.sessions[0].SessionID)
	require.Equal(t, "h2-session-b", repo.sessions[1].SessionID)
	require.Equal(t, "h2-session-a", repo.samples[0].SessionID)
	require.Equal(t, "h2-session-b", repo.samples[1].SessionID)
}

func TestTLSCaptureHTTP2LargeBodySendsWindowUpdates(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets: map[string]int{"openai": 1},
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
	require.NoError(t, writeTLSCaptureH2Request(fr, 1, tlsCaptureH2Request{
		Method:      http.MethodPost,
		Path:        "/capture/openai/v1/responses",
		Authority:   listener.Addr().String(),
		Token:       task.Token,
		UserAgent:   "codex_exec/0.140.0",
		Originator:  "codex_exec",
		ContentType: "application/json",
		Body:        `{"input":"` + strings.Repeat("a", 70000) + `"}`,
	}))

	resp, sawConnWindowUpdate, sawStreamWindowUpdate := readTLSCaptureH2ResponseAndWindowUpdates(t, fr, 1)
	require.Equal(t, "200", resp.Status)
	require.True(t, sawConnWindowUpdate)
	require.True(t, sawStreamWindowUpdate)
}

func TestTLSCaptureH2SSEAdditionalStreamCreatesReplayableSamplePerStream(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets:    map[string]int{"openai": 2},
		UAKeywords: []string{"codex"},
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
	))
	require.NoError(t, writeTLSCaptureH2Request(fr, 1, tlsCaptureH2Request{
		Method:      http.MethodPost,
		Path:        "/capture/openai/v1/responses",
		Authority:   listener.Addr().String(),
		Token:       task.Token,
		UserAgent:   "codex_exec/0.140.0",
		Originator:  "codex_exec",
		Accept:      "text/event-stream",
		ContentType: "application/json",
		Body:        `{"model":"gpt-5.4","stream":true,"input":"capture"}`,
	}))
	first := readTLSCaptureH2Response(t, fr, 1)
	require.Equal(t, "200", first.Status)
	require.Contains(t, first.Headers.Get("content-type"), "text/event-stream")
	require.Contains(t, first.Body, "event: response.created")
	require.Contains(t, first.Body, "event: response.completed")

	require.NoError(t, writeTLSCaptureH2Request(fr, 3, tlsCaptureH2Request{
		Method:      http.MethodPost,
		Path:        "/capture/openai/v1/responses",
		Authority:   listener.Addr().String(),
		Token:       task.Token,
		UserAgent:   "codex_exec/0.140.0",
		Originator:  "codex_exec",
		Accept:      "text/event-stream",
		ContentType: "application/json",
		Body:        `{"model":"gpt-5.4","stream":true,"input":"capture-2"}`,
	}))
	second := readTLSCaptureH2Response(t, fr, 3)
	require.Equal(t, "200", second.Status)
	require.Contains(t, second.Headers.Get("content-type"), "text/event-stream")
	require.Contains(t, second.Body, "event: response.created")
	require.Contains(t, second.Body, "event: response.completed")

	require.Len(t, repo.samples, 2)
	require.Len(t, repo.sessions, 1)
	require.Len(t, repo.sessionEvents, 2)
	require.Equal(t, string(tlsfpTransport.H2), repo.samples[0].Transport)
	require.Equal(t, string(tlsfpTransport.H2), repo.samples[1].Transport)
	require.Equal(t, repo.sessions[0].SessionID, repo.samples[0].SessionID)
	require.Equal(t, repo.sessions[0].SessionID, repo.samples[1].SessionID)
	require.JSONEq(t, `{"model":"gpt-5.4","stream":true,"input":"capture"}`, repo.samples[0].RawPayload)
	require.JSONEq(t, `{"model":"gpt-5.4","stream":true,"input":"capture-2"}`, repo.samples[1].RawPayload)
	require.Equal(t, repo.sessions[0].SessionID, repo.sessionEvents[0].SessionID)
	require.Equal(t, repo.sessions[0].SessionID, repo.sessionEvents[1].SessionID)
	require.Equal(t, string(tlsfpTransport.H2), repo.sessionEvents[0].Transport)
	require.Equal(t, string(tlsfpTransport.H2), repo.sessionEvents[1].Transport)
	require.Equal(t, "1", repo.sessionEvents[0].StreamID)
	require.Equal(t, "3", repo.sessionEvents[1].StreamID)
}

type tlsCaptureH2Request struct {
	Method      string
	Path        string
	Authority   string
	Token       string
	UserAgent   string
	Originator  string
	SessionID   string
	Accept      string
	ContentType string
	Body        string
}

type tlsCaptureH2Response struct {
	Status  string
	Headers http.Header
	Body    string
}

func newTLSCaptureH2Client(t *testing.T, addr string) (*utls.UConn, *http2.Framer) {
	t.Helper()

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	require.NoError(t, err)

	uconn := utls.UClient(conn, &utls.Config{
		ServerName:         "localhost",
		InsecureSkipVerify: true,
		NextProtos:         []string{"h2"},
	}, utls.HelloCustom)
	require.NoError(t, uconn.ApplyPreset(nativeClientHelloSpec()))
	require.NoError(t, uconn.Handshake())
	require.Equal(t, "h2", uconn.ConnectionState().NegotiatedProtocol)

	fr := http2.NewFramer(uconn, uconn)
	fr.ReadMetaHeaders = hpack.NewDecoder(4096, nil)
	return uconn, fr
}

func writeTLSCaptureH2PrefaceAndSettings(conn net.Conn, fr *http2.Framer, settings ...http2.Setting) error {
	if _, err := io.WriteString(conn, http2.ClientPreface); err != nil {
		return err
	}
	return fr.WriteSettings(settings...)
}

func writeTLSCaptureH2Request(fr *http2.Framer, streamID uint32, req tlsCaptureH2Request) error {
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
	if req.SessionID != "" {
		fields = append(fields, hpack.HeaderField{Name: "x-claude-code-session-id", Value: req.SessionID})
	}
	if req.Accept != "" {
		fields = append(fields, hpack.HeaderField{Name: "accept", Value: req.Accept})
	}
	if req.ContentType != "" {
		fields = append(fields, hpack.HeaderField{Name: "content-type", Value: req.ContentType})
	}
	if req.Body != "" {
		fields = append(fields, hpack.HeaderField{Name: "content-length", Value: fmt.Sprintf("%d", len(req.Body))})
	}
	for _, field := range fields {
		if err := encoder.WriteField(field); err != nil {
			return err
		}
	}
	if err := fr.WriteHeaders(http2.HeadersFrameParam{
		StreamID:      streamID,
		EndHeaders:    true,
		EndStream:     req.Body == "",
		BlockFragment: headerBlock.Bytes(),
	}); err != nil {
		return err
	}
	if req.Body != "" {
		return fr.WriteData(streamID, true, []byte(req.Body))
	}
	return nil
}

func readTLSCaptureH2Response(t *testing.T, fr *http2.Framer, streamID uint32) tlsCaptureH2Response {
	t.Helper()

	resp := tlsCaptureH2Response{Headers: make(http.Header)}
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

func readTLSCaptureH2ResponseAndWindowUpdates(t *testing.T, fr *http2.Framer, streamID uint32) (tlsCaptureH2Response, bool, bool) {
	t.Helper()

	resp := tlsCaptureH2Response{Headers: make(http.Header)}
	var body strings.Builder
	var sawConnWindowUpdate bool
	var sawStreamWindowUpdate bool

	for {
		frame, err := fr.ReadFrame()
		require.NoError(t, err)
		switch f := frame.(type) {
		case *http2.SettingsFrame:
			if !f.IsAck() {
				require.NoError(t, fr.WriteSettingsAck())
			}
		case *http2.WindowUpdateFrame:
			if f.StreamID == 0 {
				sawConnWindowUpdate = true
			}
			if f.StreamID == streamID {
				sawStreamWindowUpdate = true
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
				return resp, sawConnWindowUpdate, sawStreamWindowUpdate
			}
		case *http2.DataFrame:
			if f.StreamID != streamID {
				continue
			}
			body.Write(f.Data())
			if f.StreamEnded() {
				resp.Body = body.String()
				return resp, sawConnWindowUpdate, sawStreamWindowUpdate
			}
		}
	}
}
