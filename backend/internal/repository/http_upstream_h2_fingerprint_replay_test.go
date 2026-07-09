package repository

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	tlsfpHTTP2 "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/http2"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

type recordingConn struct {
	bytes.Buffer
}

func (c *recordingConn) Read(_ []byte) (int, error)       { return 0, io.EOF }
func (c *recordingConn) Close() error                     { return nil }
func (c *recordingConn) LocalAddr() net.Addr              { return dummyAddr("local") }
func (c *recordingConn) RemoteAddr() net.Addr             { return dummyAddr("remote") }
func (c *recordingConn) SetDeadline(time.Time) error      { return nil }
func (c *recordingConn) SetReadDeadline(time.Time) error  { return nil }
func (c *recordingConn) SetWriteDeadline(time.Time) error { return nil }

type dummyAddr string

func (a dummyAddr) Network() string { return "tcp" }
func (a dummyAddr) String() string  { return string(a) }

func TestHTTP2FingerprintReplayConnRewritesInitialFrames(t *testing.T) {
	parsed, err := tlsfpHTTP2.ParseFingerprint("4:65535,1:4096,3:100|wu:12345|p:1:0:1:200|ph::method,:scheme,:authority,:path")
	require.NoError(t, err)

	base := &recordingConn{}
	conn := newHTTP2FingerprintReplayConn(base, parsed)

	raw, originalFields := buildInitialHTTP2ClientWrite(t)
	n, err := conn.Write(raw)
	require.NoError(t, err)
	require.Equal(t, len(raw), n)

	written := base.Bytes()
	require.True(t, bytes.HasPrefix(written, []byte(http2.ClientPreface)))

	settings, windowUpdates, priorities, fields := decodeInitialHTTP2ClientWrite(t, written)
	require.Equal(t, []http2.Setting{
		{ID: http2.SettingInitialWindowSize, Val: 65535},
		{ID: http2.SettingHeaderTableSize, Val: 4096},
		{ID: http2.SettingMaxConcurrentStreams, Val: 100},
	}, settings)
	require.Equal(t, []uint32{12345}, windowUpdates)
	require.Len(t, priorities, 1)
	require.Equal(t, tlsfpHTTP2.PriorityFrame{StreamID: 1, Dependency: 0, Exclusive: true, Weight: 200}, priorities[0])
	require.Equal(t, []string{":method", ":scheme", ":authority", ":path"}, pseudoHeaderNames(fields))
	require.Equal(t, regularHeaderNames(originalFields), regularHeaderNames(fields))
}

func TestHTTP2FingerprintReplayRoundTripperReusesTransport(t *testing.T) {
	rt := &http2FingerprintReplayRoundTripper{
		settings:          poolSettings{idleConnTimeout: time.Minute},
		profile:           &tlsfingerprint.Profile{Name: "test"},
		parsedFingerprint: &tlsfpHTTP2.ParsedFingerprint{},
	}

	first, err := rt.getOrCreateTransport()
	require.NoError(t, err)
	second, err := rt.getOrCreateTransport()
	require.NoError(t, err)
	require.Same(t, first, second)
}

func buildInitialHTTP2ClientWrite(t *testing.T) ([]byte, []hpack.HeaderField) {
	t.Helper()

	var out bytes.Buffer
	out.WriteString(http2.ClientPreface)
	fr := http2.NewFramer(&out, nil)
	require.NoError(t, fr.WriteSettings(http2.Setting{ID: http2.SettingHeaderTableSize, Val: 4096}))
	require.NoError(t, fr.WriteWindowUpdate(0, 65535))

	originalFields := []hpack.HeaderField{
		{Name: ":method", Value: http.MethodPost},
		{Name: ":scheme", Value: "https"},
		{Name: ":path", Value: "/v1/responses"},
		{Name: ":authority", Value: "api.openai.com"},
		{Name: "content-type", Value: "application/json"},
		{Name: "user-agent", Value: "test-client"},
	}
	block, err := encodeHTTP2HeaderBlock(originalFields)
	require.NoError(t, err)
	require.NoError(t, writeHTTP2HeaderBlock(fr, 1, true, 16384, block))
	return out.Bytes(), originalFields
}

func decodeInitialHTTP2ClientWrite(t *testing.T, raw []byte) ([]http2.Setting, []uint32, []tlsfpHTTP2.PriorityFrame, []hpack.HeaderField) {
	t.Helper()
	reader := bytes.NewReader(raw[len(http2.ClientPreface):])
	fr := http2.NewFramer(nil, reader)
	fr.ReadMetaHeaders = hpack.NewDecoder(4096, nil)

	var (
		settings      []http2.Setting
		windowUpdates []uint32
		priorities    []tlsfpHTTP2.PriorityFrame
		fields        []hpack.HeaderField
	)
	for {
		frame, err := fr.ReadFrame()
		require.NoError(t, err)
		switch f := frame.(type) {
		case *http2.SettingsFrame:
			require.False(t, f.IsAck())
			err := f.ForeachSetting(func(s http2.Setting) error {
				settings = append(settings, s)
				return nil
			})
			require.NoError(t, err)
		case *http2.WindowUpdateFrame:
			if f.StreamID == 0 {
				windowUpdates = append(windowUpdates, f.Increment)
			}
		case *http2.PriorityFrame:
			priorities = append(priorities, tlsfpHTTP2.PriorityFrame{
				StreamID:   f.StreamID,
				Dependency: f.StreamDep,
				Exclusive:  f.Exclusive,
				Weight:     f.Weight,
			})
		case *http2.MetaHeadersFrame:
			fields = append(fields, f.Fields...)
			return settings, windowUpdates, priorities, fields
		}
	}
}

func pseudoHeaderNames(fields []hpack.HeaderField) []string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		if field.IsPseudo() {
			names = append(names, field.Name)
		}
	}
	return names
}

func regularHeaderNames(fields []hpack.HeaderField) []string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		if !field.IsPseudo() {
			names = append(names, field.Name)
		}
	}
	return names
}
