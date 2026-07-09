package repository

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	tlsfpHTTP2 "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/http2"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

type http2FingerprintReplayRoundTripper struct {
	settings           poolSettings
	proxyURL           *url.URL
	profile            *tlsfingerprint.Profile
	parsedFingerprint  *tlsfpHTTP2.ParsedFingerprint
	validateResolvedIP bool
	transportMu        sync.Mutex
	transport          *http2.Transport
}

func (rt *http2FingerprintReplayRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt == nil {
		return nil, fmt.Errorf("http2 fingerprint round tripper is required")
	}
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	transport, err := rt.getOrCreateTransport()
	if err != nil {
		return nil, err
	}

	base := http.RoundTripper(transport)
	if rt.settings.responseHeaderTimeout > 0 {
		base = responseHeaderTimeoutRoundTripper{base: transport, timeout: rt.settings.responseHeaderTimeout}
	}
	resp, err := base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (rt *http2FingerprintReplayRoundTripper) getOrCreateTransport() (*http2.Transport, error) {
	if rt == nil {
		return nil, fmt.Errorf("http2 fingerprint round tripper is required")
	}
	rt.transportMu.Lock()
	defer rt.transportMu.Unlock()
	if rt.transport != nil {
		return rt.transport, nil
	}
	dialTLS, fallbackProxy, err := tlsFingerprintDialTLSFunc(rt.profile, rt.proxyURL, rt.validateResolvedIP)
	if err != nil {
		return nil, err
	}
	if fallbackProxy {
		return nil, fmt.Errorf("unsupported proxy scheme for http2 fingerprint replay: %s", rt.proxyURL.Scheme)
	}
	rt.transport = &http2.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			conn, err := dialTLS(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			return newHTTP2FingerprintReplayConn(conn, rt.parsedFingerprint), nil
		},
		IdleConnTimeout: rt.settings.idleConnTimeout,
	}
	return rt.transport, nil
}

func (rt *http2FingerprintReplayRoundTripper) CloseIdleConnections() {
	if rt == nil {
		return
	}
	rt.transportMu.Lock()
	transport := rt.transport
	rt.transportMu.Unlock()
	if transport != nil {
		transport.CloseIdleConnections()
	}
}

type http2FingerprintReplayConn struct {
	net.Conn

	mu          sync.Mutex
	parsed      *tlsfpHTTP2.ParsedFingerprint
	buffer      bytes.Buffer
	passthrough bool
}

func newHTTP2FingerprintReplayConn(conn net.Conn, parsed *tlsfpHTTP2.ParsedFingerprint) net.Conn {
	if conn == nil || parsed == nil {
		return conn
	}
	return &http2FingerprintReplayConn{Conn: conn, parsed: parsed}
}

func (c *http2FingerprintReplayConn) Write(p []byte) (int, error) {
	if c == nil || c.Conn == nil || c.parsed == nil {
		return c.Conn.Write(p)
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.passthrough {
		return c.Conn.Write(p)
	}
	if _, err := c.buffer.Write(p); err != nil {
		return 0, err
	}
	if err := c.flushBufferedIfReady(); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *http2FingerprintReplayConn) flushBufferedIfReady() error {
	buffered := c.buffer.Bytes()
	if len(buffered) < len(http2.ClientPreface) {
		return nil
	}
	if !bytes.Equal(buffered[:len(http2.ClientPreface)], []byte(http2.ClientPreface)) {
		c.passthrough = true
		_, err := c.Conn.Write(buffered)
		c.buffer.Reset()
		return err
	}

	parsed, consumed, err := parseInitialHTTP2ClientWrite(buffered)
	if err != nil {
		return err
	}
	if parsed == nil {
		return nil
	}

	rewritten, err := rewriteInitialHTTP2ClientWrite(c.parsed, parsed)
	if err != nil {
		return err
	}
	if _, err := c.Conn.Write(rewritten); err != nil {
		return err
	}
	if consumed < len(buffered) {
		if _, err := c.Conn.Write(buffered[consumed:]); err != nil {
			return err
		}
	}
	c.buffer.Reset()
	c.passthrough = true
	return nil
}

type parsedInitialHTTP2ClientWrite struct {
	preface        []byte
	settingsFrames []http2FrameEnvelope
	windowUpdates  []http2FrameEnvelope
	priorities     []http2FrameEnvelope
	headers        http2HeaderSequence
}

type http2FrameEnvelope struct {
	Type     http2.FrameType
	Flags    http2.Flags
	StreamID uint32
	Payload  []byte
}

type http2HeaderSequence struct {
	StreamID          uint32
	EndStream         bool
	DecodedFields     []hpack.HeaderField
	OriginalFragments []byte
}

func parseInitialHTTP2ClientWrite(raw []byte) (*parsedInitialHTTP2ClientWrite, int, error) {
	if len(raw) < len(http2.ClientPreface) {
		return nil, 0, nil
	}
	offset := len(http2.ClientPreface)
	out := &parsedInitialHTTP2ClientWrite{
		preface: append([]byte(nil), raw[:offset]...),
	}
	var headerFragments []byte
	var headerStreamID uint32
	var headerEndStream bool
	sawHeaders := false
	for offset < len(raw) {
		if len(raw[offset:]) < 9 {
			return nil, 0, nil
		}
		length := int(raw[offset])<<16 | int(raw[offset+1])<<8 | int(raw[offset+2])
		total := 9 + length
		if len(raw[offset:]) < total {
			return nil, 0, nil
		}
		frameType := http2.FrameType(raw[offset+3])
		flags := http2.Flags(raw[offset+4])
		streamID := binary.BigEndian.Uint32(raw[offset+5:offset+9]) & 0x7fffffff
		payload := append([]byte(nil), raw[offset+9:offset+total]...)
		env := http2FrameEnvelope{
			Type:     frameType,
			Flags:    flags,
			StreamID: streamID,
			Payload:  payload,
		}
		offset += total

		switch frameType {
		case http2.FrameSettings:
			if !flags.Has(http2.FlagSettingsAck) {
				out.settingsFrames = append(out.settingsFrames, env)
			}
		case http2.FrameWindowUpdate:
			if streamID == 0 {
				out.windowUpdates = append(out.windowUpdates, env)
			}
		case http2.FramePriority:
			out.priorities = append(out.priorities, env)
		case http2.FrameHeaders:
			sawHeaders = true
			headerStreamID = streamID
			headerEndStream = flags.Has(http2.FlagHeadersEndStream)
			fragment, err := headerBlockFragmentFromHeadersPayload(payload, flags)
			if err != nil {
				return nil, 0, err
			}
			headerFragments = append(headerFragments, fragment...)
			if flags.Has(http2.FlagHeadersEndHeaders) {
				fields, err := decodeHTTP2HeaderBlock(headerFragments)
				if err != nil {
					return nil, 0, err
				}
				out.headers = http2HeaderSequence{
					StreamID:          headerStreamID,
					EndStream:         headerEndStream,
					DecodedFields:     fields,
					OriginalFragments: append([]byte(nil), headerFragments...),
				}
				return out, offset, nil
			}
		case http2.FrameContinuation:
			if !sawHeaders {
				continue
			}
			headerFragments = append(headerFragments, payload...)
			if flags.Has(http2.FlagHeadersEndHeaders) {
				fields, err := decodeHTTP2HeaderBlock(headerFragments)
				if err != nil {
					return nil, 0, err
				}
				out.headers = http2HeaderSequence{
					StreamID:          headerStreamID,
					EndStream:         headerEndStream,
					DecodedFields:     fields,
					OriginalFragments: append([]byte(nil), headerFragments...),
				}
				return out, offset, nil
			}
		}
	}
	return nil, 0, nil
}

func rewriteInitialHTTP2ClientWrite(parsed *tlsfpHTTP2.ParsedFingerprint, initial *parsedInitialHTTP2ClientWrite) ([]byte, error) {
	if parsed == nil || initial == nil {
		return nil, fmt.Errorf("http2 fingerprint replay data is required")
	}

	var out bytes.Buffer
	out.Write(initial.preface)
	fr := http2.NewFramer(&out, nil)

	if len(parsed.Settings) > 0 {
		if err := fr.WriteSettings(parsed.Settings...); err != nil {
			return nil, err
		}
	} else {
		for _, frame := range initial.settingsFrames {
			writeRawHTTP2Frame(&out, frame)
		}
	}

	if len(parsed.ConnWindowUpdates) > 0 {
		for _, increment := range parsed.ConnWindowUpdates {
			if err := fr.WriteWindowUpdate(0, increment); err != nil {
				return nil, err
			}
		}
	} else {
		for _, frame := range initial.windowUpdates {
			writeRawHTTP2Frame(&out, frame)
		}
	}

	if len(parsed.Priorities) > 0 {
		for _, priority := range parsed.Priorities {
			if err := fr.WritePriority(priority.StreamID, http2.PriorityParam{
				StreamDep: priority.Dependency,
				Exclusive: priority.Exclusive,
				Weight:    priority.Weight,
			}); err != nil {
				return nil, err
			}
		}
	} else {
		for _, frame := range initial.priorities {
			writeRawHTTP2Frame(&out, frame)
		}
	}

	headerFields := reorderHTTP2HeaderFields(initial.headers.DecodedFields, parsed.PseudoHeaderOrder)
	headerBlock, err := encodeHTTP2HeaderBlock(headerFields)
	if err != nil {
		return nil, err
	}
	maxFrameSize := http2FingerprintMaxFrameSize(parsed.Settings)
	if len(headerBlock) == 0 {
		headerBlock = append([]byte(nil), initial.headers.OriginalFragments...)
	}
	if err := writeHTTP2HeaderBlock(fr, initial.headers.StreamID, initial.headers.EndStream, maxFrameSize, headerBlock); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func writeRawHTTP2Frame(w io.Writer, frame http2FrameEnvelope) {
	if w == nil {
		return
	}
	header := []byte{
		byte(len(frame.Payload) >> 16),
		byte(len(frame.Payload) >> 8),
		byte(len(frame.Payload)),
		byte(frame.Type),
		byte(frame.Flags),
		0, 0, 0, 0,
	}
	binary.BigEndian.PutUint32(header[5:], frame.StreamID&0x7fffffff)
	_, _ = w.Write(header)
	_, _ = w.Write(frame.Payload)
}

func headerBlockFragmentFromHeadersPayload(payload []byte, flags http2.Flags) ([]byte, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	offset := 0
	if flags.Has(http2.FlagHeadersPadded) {
		if len(payload) < 1 {
			return nil, fmt.Errorf("invalid padded headers payload")
		}
		padLength := int(payload[0])
		offset++
		if len(payload) < offset+padLength {
			return nil, fmt.Errorf("invalid padded headers payload")
		}
		payload = payload[:len(payload)-padLength]
	}
	if flags.Has(http2.FlagHeadersPriority) {
		if len(payload[offset:]) < 5 {
			return nil, fmt.Errorf("invalid priority headers payload")
		}
		offset += 5
	}
	return append([]byte(nil), payload[offset:]...), nil
}

func decodeHTTP2HeaderBlock(block []byte) ([]hpack.HeaderField, error) {
	fields := make([]hpack.HeaderField, 0, 16)
	decoder := hpack.NewDecoder(4096, func(field hpack.HeaderField) {
		fields = append(fields, field)
	})
	if _, err := decoder.Write(block); err != nil {
		return nil, err
	}
	if err := decoder.Close(); err != nil {
		return nil, err
	}
	return fields, nil
}

func reorderHTTP2HeaderFields(fields []hpack.HeaderField, pseudoHeaderOrder []string) []hpack.HeaderField {
	if len(fields) == 0 || len(pseudoHeaderOrder) == 0 {
		return fields
	}
	pseudoByName := make(map[string][]hpack.HeaderField)
	pseudoOriginalOrder := make([]string, 0, len(fields))
	regular := make([]hpack.HeaderField, 0, len(fields))
	for _, field := range fields {
		if strings.HasPrefix(field.Name, ":") {
			if _, ok := pseudoByName[field.Name]; !ok {
				pseudoOriginalOrder = append(pseudoOriginalOrder, field.Name)
			}
			pseudoByName[field.Name] = append(pseudoByName[field.Name], field)
			continue
		}
		regular = append(regular, field)
	}

	reordered := make([]hpack.HeaderField, 0, len(fields))
	used := make(map[string]bool)
	for _, name := range pseudoHeaderOrder {
		if len(pseudoByName[name]) == 0 {
			continue
		}
		reordered = append(reordered, pseudoByName[name]...)
		used[name] = true
	}
	for _, name := range pseudoOriginalOrder {
		if used[name] {
			continue
		}
		reordered = append(reordered, pseudoByName[name]...)
	}
	reordered = append(reordered, regular...)
	return reordered
}

func encodeHTTP2HeaderBlock(fields []hpack.HeaderField) ([]byte, error) {
	var out bytes.Buffer
	encoder := hpack.NewEncoder(&out)
	for _, field := range fields {
		if err := encoder.WriteField(field); err != nil {
			return nil, err
		}
	}
	return out.Bytes(), nil
}

func writeHTTP2HeaderBlock(fr *http2.Framer, streamID uint32, endStream bool, maxFrameSize int, block []byte) error {
	if fr == nil {
		return fmt.Errorf("http2 framer is required")
	}
	if maxFrameSize < 16384 || maxFrameSize > 16777215 {
		maxFrameSize = 16384
	}
	if len(block) <= maxFrameSize {
		return fr.WriteHeaders(http2.HeadersFrameParam{
			StreamID:      streamID,
			BlockFragment: block,
			EndStream:     endStream,
			EndHeaders:    true,
		})
	}

	if err := fr.WriteHeaders(http2.HeadersFrameParam{
		StreamID:      streamID,
		BlockFragment: block[:maxFrameSize],
		EndStream:     endStream,
		EndHeaders:    false,
	}); err != nil {
		return err
	}
	remaining := block[maxFrameSize:]
	for len(remaining) > 0 {
		chunkSize := min(len(remaining), maxFrameSize)
		chunk := remaining[:chunkSize]
		remaining = remaining[chunkSize:]
		if err := fr.WriteContinuation(streamID, len(remaining) == 0, chunk); err != nil {
			return err
		}
	}
	return nil
}

func http2FingerprintMaxFrameSize(settings []http2.Setting) int {
	for _, setting := range settings {
		if setting.ID == http2.SettingMaxFrameSize {
			val := int(setting.Val)
			if val >= 16384 && val <= 16777215 {
				return val
			}
			break
		}
	}
	return 16384
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
