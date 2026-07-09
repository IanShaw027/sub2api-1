package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	tlsfpHTTP2 "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/http2"
	coderws "github.com/coder/websocket"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

const openAIWSH2DefaultMaxFrameSize = 16384

func openAIWSTLSProfileWantsWebSocketH2(profile *tlsfingerprint.Profile) bool {
	if profile == nil {
		return false
	}
	var hasH2 bool
	for _, proto := range profile.ALPNProtocols {
		if strings.EqualFold(strings.TrimSpace(proto), "h2") {
			hasH2 = true
			break
		}
	}
	if !hasH2 {
		return false
	}
	parsed, err := tlsfpHTTP2.ParseFingerprint(strings.TrimSpace(profile.HTTP2Fingerprint))
	if err != nil || parsed == nil {
		return false
	}
	for _, header := range parsed.PseudoHeaderOrder {
		if header == ":protocol" {
			return true
		}
	}
	return false
}

func dialOpenAIWSH2(ctx context.Context, wsURL string, headers http.Header, proxyURL string, profile *tlsfingerprint.Profile) (openAIWSClientConn, int, http.Header, error) {
	rawConn, err := dialOpenAIWSH2TLSConn(ctx, wsURL, proxyURL, profile)
	if err != nil {
		return nil, 0, nil, err
	}
	conn, status, respHeaders, handshakeErr := dialOpenAIWSH2WithConn(ctx, wsURL, headers, profile, rawConn)
	if handshakeErr != nil {
		_ = rawConn.Close()
	}
	return conn, status, respHeaders, handshakeErr
}

func dialOpenAIWSH2TLSConn(ctx context.Context, wsURL string, proxyURL string, profile *tlsfingerprint.Profile) (net.Conn, error) {
	if profile == nil {
		return nil, fmt.Errorf("tls fingerprint profile is required")
	}
	parsedURL, err := url.Parse(strings.TrimSpace(wsURL))
	if err != nil {
		return nil, fmt.Errorf("invalid ws url: %w", err)
	}
	hostPort := parsedURL.Host
	if parsedURL.Port() == "" {
		hostPort = net.JoinHostPort(parsedURL.Hostname(), "443")
	}
	normalizedProxy := strings.TrimSpace(proxyURL)
	if normalizedProxy == "" {
		dialer := tlsfingerprint.NewDialer(profile, nil)
		return dialer.DialTLSContext(ctx, "tcp", hostPort)
	}
	parsedProxyURL, err := url.Parse(normalizedProxy)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy url: %w", err)
	}
	switch strings.ToLower(parsedProxyURL.Scheme) {
	case "http", "https":
		dialer := tlsfingerprint.NewHTTPProxyDialer(profile, parsedProxyURL)
		return dialer.DialTLSContext(ctx, "tcp", hostPort)
	case "socks5", "socks5h":
		dialer := tlsfingerprint.NewSOCKS5ProxyDialer(profile, parsedProxyURL)
		return dialer.DialTLSContext(ctx, "tcp", hostPort)
	default:
		return nil, fmt.Errorf("unsupported proxy scheme for tls fingerprint ws h2 dial: %s", parsedProxyURL.Scheme)
	}
}

func dialOpenAIWSH2WithConn(ctx context.Context, wsURL string, headers http.Header, profile *tlsfingerprint.Profile, conn net.Conn) (openAIWSClientConn, int, http.Header, error) {
	if conn == nil {
		return nil, 0, nil, fmt.Errorf("h2 websocket connection is required")
	}
	parsedURL, err := url.Parse(strings.TrimSpace(wsURL))
	if err != nil {
		return nil, 0, nil, fmt.Errorf("invalid ws url: %w", err)
	}
	parsedFingerprint, err := tlsfpHTTP2.ParseFingerprint(strings.TrimSpace(profile.HTTP2Fingerprint))
	if err != nil {
		return nil, 0, nil, fmt.Errorf("parse http2 fingerprint: %w", err)
	}
	if parsedFingerprint == nil {
		return nil, 0, nil, fmt.Errorf("http2 fingerprint is required")
	}

	readFr := http2.NewFramer(nil, conn)
	readFr.ReadMetaHeaders = hpack.NewDecoder(4096, nil)
	writeFr := http2.NewFramer(conn, nil)

	if err := setConnWriteDeadlineFromContext(ctx, conn); err != nil {
		return nil, 0, nil, err
	}
	if _, err := io.WriteString(conn, http2.ClientPreface); err != nil {
		return nil, 0, nil, err
	}
	if len(parsedFingerprint.Settings) > 0 {
		if err := writeFr.WriteSettings(parsedFingerprint.Settings...); err != nil {
			return nil, 0, nil, err
		}
	} else {
		if err := writeFr.WriteSettings(http2.Setting{ID: http2.SettingHeaderTableSize, Val: 4096}); err != nil {
			return nil, 0, nil, err
		}
	}
	for _, increment := range parsedFingerprint.ConnWindowUpdates {
		if err := writeFr.WriteWindowUpdate(0, increment); err != nil {
			return nil, 0, nil, err
		}
	}
	for _, priority := range parsedFingerprint.Priorities {
		if err := writeFr.WritePriority(priority.StreamID, http2.PriorityParam{
			StreamDep: priority.Dependency,
			Exclusive: priority.Exclusive,
			Weight:    priority.Weight,
		}); err != nil {
			return nil, 0, nil, err
		}
	}

	requestHeader, err := buildOpenAIWSH2RequestHeaders(parsedURL, headers, parsedFingerprint.PseudoHeaderOrder)
	if err != nil {
		return nil, 0, nil, err
	}
	if err := writeOpenAIWSH2Headers(writeFr, 1, requestHeader); err != nil {
		return nil, 0, nil, err
	}
	clearConnWriteDeadline(conn)

	statusCode := 0
	respHeaders := make(http.Header)
	peerMaxFrameSize := openAIWSH2DefaultMaxFrameSize
	for {
		if err := setConnReadDeadlineFromContext(ctx, conn); err != nil {
			return nil, 0, nil, err
		}
		frame, err := readFr.ReadFrame()
		clearConnReadDeadline(conn)
		if err != nil {
			return nil, 0, nil, err
		}
		switch f := frame.(type) {
		case *http2.SettingsFrame:
			if !f.IsAck() {
				err := f.ForeachSetting(func(s http2.Setting) error {
					if s.ID == http2.SettingMaxFrameSize {
						if v := int(s.Val); v >= openAIWSH2DefaultMaxFrameSize && v <= 16777215 {
							peerMaxFrameSize = v
						}
					}
					return nil
				})
				if err != nil {
					return nil, 0, nil, err
				}
				if err := writeFr.WriteSettingsAck(); err != nil {
					return nil, 0, nil, err
				}
			}
		case *http2.MetaHeadersFrame:
			if f.StreamID != 1 {
				continue
			}
			statusCode, err = strconv.Atoi(f.PseudoValue("status"))
			if err != nil {
				return nil, 0, nil, fmt.Errorf("invalid h2 websocket status: %w", err)
			}
			respHeaders = metaHeadersToHTTPHeader(f)
			if statusCode < 200 || statusCode >= 300 {
				return nil, statusCode, respHeaders, fmt.Errorf("websocket h2 handshake failed with status %d", statusCode)
			}
			return &openAIWSH2ClientConn{
				conn:             conn,
				readFr:           readFr,
				writeFr:          writeFr,
				streamID:         1,
				peerMaxFrameSize: peerMaxFrameSize,
			}, 0, respHeaders, nil
		case *http2.GoAwayFrame:
			return nil, 0, nil, fmt.Errorf("websocket h2 received GOAWAY during handshake")
		}
	}
}

type openAIWSH2ClientConn struct {
	conn     net.Conn
	readFr   *http2.Framer
	writeFr  *http2.Framer
	streamID uint32

	writeMu sync.Mutex
	readMu  sync.Mutex
	closeMu sync.Mutex
	closed  bool

	peerMaxFrameSize int
	readBuffer       []byte
	fragmentOpcode   byte
	messageBuffer    []byte
}

func (c *openAIWSH2ClientConn) WriteJSON(ctx context.Context, value any) error {
	if c == nil || c.conn == nil {
		return errOpenAIWSConnClosed
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.writeWebSocketFrame(ctx, 0x1, payload)
}

func (c *openAIWSH2ClientConn) ReadMessage(ctx context.Context) ([]byte, error) {
	if c == nil || c.conn == nil {
		return nil, errOpenAIWSConnClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	c.readMu.Lock()
	defer c.readMu.Unlock()

	for {
		opcode, fin, payload, remaining, ok, err := decodeOpenAIWSWebSocketServerFrame(c.readBuffer)
		if err != nil {
			return nil, err
		}
		if ok {
			c.readBuffer = remaining
			switch opcode {
			case 0x1, 0x2:
				if fin {
					return payload, nil
				}
				c.fragmentOpcode = opcode
				c.messageBuffer = append(c.messageBuffer[:0], payload...)
			case 0x0:
				if c.fragmentOpcode == 0 {
					continue
				}
				c.messageBuffer = append(c.messageBuffer, payload...)
				if fin {
					out := append([]byte(nil), c.messageBuffer...)
					c.fragmentOpcode = 0
					c.messageBuffer = nil
					return out, nil
				}
			case 0x8:
				_ = c.Close()
				return nil, errOpenAIWSConnClosed
			case 0x9:
				if err := c.writeWebSocketFrame(ctx, 0xA, payload); err != nil {
					return nil, err
				}
			case 0xA:
			}
			continue
		}

		if err := setConnReadDeadlineFromContext(ctx, c.conn); err != nil {
			return nil, err
		}
		frame, err := c.readFr.ReadFrame()
		clearConnReadDeadline(c.conn)
		if err != nil {
			return nil, err
		}
		switch f := frame.(type) {
		case *http2.DataFrame:
			if f.StreamID != c.streamID {
				continue
			}
			if err := c.writeWindowUpdates(uint32(len(f.Data()))); err != nil {
				return nil, err
			}
			c.readBuffer = append(c.readBuffer, f.Data()...)
		case *http2.SettingsFrame:
			if !f.IsAck() {
				err := f.ForeachSetting(func(s http2.Setting) error {
					if s.ID == http2.SettingMaxFrameSize {
						if v := int(s.Val); v >= openAIWSH2DefaultMaxFrameSize && v <= 16777215 {
							c.peerMaxFrameSize = v
						}
					}
					return nil
				})
				if err != nil {
					return nil, err
				}
				c.writeMu.Lock()
				err = c.writeFr.WriteSettingsAck()
				c.writeMu.Unlock()
				if err != nil {
					return nil, err
				}
			}
		case *http2.PingFrame:
			if !f.Flags.Has(http2.FlagPingAck) {
				c.writeMu.Lock()
				err := c.writeFr.WritePing(true, f.Data)
				c.writeMu.Unlock()
				if err != nil {
					return nil, err
				}
			}
		case *http2.RSTStreamFrame, *http2.GoAwayFrame:
			_ = c.Close()
			return nil, errOpenAIWSConnClosed
		}
	}
}

func (c *openAIWSH2ClientConn) Ping(ctx context.Context) error {
	if c == nil || c.conn == nil {
		return errOpenAIWSConnClosed
	}
	return c.writeWebSocketFrame(ctx, 0x9, nil)
}

func (c *openAIWSH2ClientConn) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	c.closeMu.Lock()
	if c.closed {
		c.closeMu.Unlock()
		return nil
	}
	c.closed = true
	c.closeMu.Unlock()

	_ = c.writeWebSocketFrame(context.Background(), 0x8, openAIWSWebSocketClosePayload(coderws.StatusNormalClosure, ""))
	return c.conn.Close()
}

func (c *openAIWSH2ClientConn) writeWebSocketFrame(ctx context.Context, opcode byte, payload []byte) error {
	if c == nil || c.conn == nil {
		return errOpenAIWSConnClosed
	}
	frame, err := encodeOpenAIWSWebSocketClientFrame(opcode, payload)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := setConnWriteDeadlineFromContext(ctx, c.conn); err != nil {
		return err
	}
	defer clearConnWriteDeadline(c.conn)
	maxFrameSize := c.peerMaxFrameSize
	if maxFrameSize < openAIWSH2DefaultMaxFrameSize || maxFrameSize > 16777215 {
		maxFrameSize = openAIWSH2DefaultMaxFrameSize
	}
	for len(frame) > 0 {
		chunkSize := maxFrameSize
		if len(frame) < chunkSize {
			chunkSize = len(frame)
		}
		chunk := frame[:chunkSize]
		frame = frame[chunkSize:]
		if err := c.writeFr.WriteData(c.streamID, false, chunk); err != nil {
			return err
		}
	}
	return nil
}

func (c *openAIWSH2ClientConn) writeWindowUpdates(size uint32) error {
	if c == nil || size == 0 {
		return nil
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.writeFr.WriteWindowUpdate(0, size); err != nil {
		return err
	}
	return c.writeFr.WriteWindowUpdate(c.streamID, size)
}

func buildOpenAIWSH2RequestHeaders(parsedURL *url.URL, headers http.Header, pseudoHeaderOrder []string) ([]hpack.HeaderField, error) {
	fields := make([]hpack.HeaderField, 0, 16)
	pseudoValues := map[string]string{
		":method":    http.MethodConnect,
		":protocol":  "websocket",
		":scheme":    "https",
		":authority": parsedURL.Host,
		":path":      parsedURL.RequestURI(),
	}
	if parsedURL.Scheme == "ws" {
		pseudoValues[":scheme"] = "http"
	}
	if pseudoValues[":path"] == "" {
		pseudoValues[":path"] = "/"
	}

	addedPseudo := make(map[string]bool)
	for _, name := range pseudoHeaderOrder {
		value := pseudoValues[name]
		if value == "" {
			continue
		}
		fields = append(fields, hpack.HeaderField{Name: name, Value: value})
		addedPseudo[name] = true
	}
	defaultPseudoOrder := []string{":method", ":protocol", ":scheme", ":authority", ":path"}
	for _, name := range defaultPseudoOrder {
		if addedPseudo[name] {
			continue
		}
		fields = append(fields, hpack.HeaderField{Name: name, Value: pseudoValues[name]})
	}

	requestHeaders := cloneHeader(headers)
	if requestHeaders == nil {
		requestHeaders = make(http.Header)
	}
	requestHeaders.Del("Connection")
	requestHeaders.Del("Upgrade")
	requestHeaders.Del("Host")
	requestHeaders.Set("Sec-WebSocket-Version", "13")
	if requestHeaders.Get("Sec-WebSocket-Key") == "" {
		key, err := newOpenAIWSSecWebSocketKey()
		if err != nil {
			return nil, err
		}
		requestHeaders.Set("Sec-WebSocket-Key", key)
	}
	for key, values := range requestHeaders {
		name := strings.ToLower(strings.TrimSpace(key))
		if name == "" || strings.HasPrefix(name, ":") {
			continue
		}
		for _, value := range values {
			fields = append(fields, hpack.HeaderField{Name: name, Value: value})
		}
	}
	return fields, nil
}

func writeOpenAIWSH2Headers(fr *http2.Framer, streamID uint32, fields []hpack.HeaderField) error {
	var block bytes.Buffer
	enc := hpack.NewEncoder(&block)
	for _, field := range fields {
		if err := enc.WriteField(field); err != nil {
			return err
		}
	}
	return fr.WriteHeaders(http2.HeadersFrameParam{
		StreamID:      streamID,
		BlockFragment: block.Bytes(),
		EndHeaders:    true,
		EndStream:     false,
	})
}

func metaHeadersToHTTPHeader(frame *http2.MetaHeadersFrame) http.Header {
	header := make(http.Header)
	if frame == nil {
		return header
	}
	for _, field := range frame.RegularFields() {
		header.Add(field.Name, field.Value)
	}
	return header
}

func newOpenAIWSSecWebSocketKey() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw[:]), nil
}

func encodeOpenAIWSWebSocketClientFrame(opcode byte, payload []byte) ([]byte, error) {
	var mask [4]byte
	if _, err := rand.Read(mask[:]); err != nil {
		return nil, err
	}
	frame := make([]byte, 0, len(payload)+14)
	frame = append(frame, 0x80|opcode)
	switch {
	case len(payload) < 126:
		frame = append(frame, 0x80|byte(len(payload)))
	case len(payload) <= 0xffff:
		frame = append(frame, 0x80|126, byte(len(payload)>>8), byte(len(payload)))
	default:
		frame = append(frame, 0x80|127)
		var lenBuf [8]byte
		binary.BigEndian.PutUint64(lenBuf[:], uint64(len(payload)))
		frame = append(frame, lenBuf[:]...)
	}
	frame = append(frame, mask[:]...)
	for i, b := range payload {
		frame = append(frame, b^mask[i%4])
	}
	return frame, nil
}

func decodeOpenAIWSWebSocketServerFrame(buffer []byte) (opcode byte, fin bool, payload []byte, remaining []byte, ok bool, err error) {
	if len(buffer) < 2 {
		return 0, false, nil, buffer, false, nil
	}
	opcode = buffer[0] & 0x0f
	fin = buffer[0]&0x80 != 0
	payloadLen := int(buffer[1] & 0x7f)
	offset := 2
	switch payloadLen {
	case 126:
		if len(buffer) < offset+2 {
			return 0, false, nil, buffer, false, nil
		}
		payloadLen = int(buffer[offset])<<8 | int(buffer[offset+1])
		offset += 2
	case 127:
		if len(buffer) < offset+8 {
			return 0, false, nil, buffer, false, nil
		}
		payloadLen64 := binary.BigEndian.Uint64(buffer[offset : offset+8])
		if payloadLen64 > uint64(openAIWSMessageReadLimitBytes) {
			return 0, false, nil, buffer, false, fmt.Errorf("websocket payload too large")
		}
		payloadLen = int(payloadLen64)
		offset += 8
	}
	if buffer[1]&0x80 != 0 {
		return 0, false, nil, buffer, false, fmt.Errorf("server websocket frame must not be masked")
	}
	if len(buffer) < offset+payloadLen {
		return 0, false, nil, buffer, false, nil
	}
	payload = append([]byte(nil), buffer[offset:offset+payloadLen]...)
	return opcode, fin, payload, buffer[offset+payloadLen:], true, nil
}

func openAIWSWebSocketClosePayload(code coderws.StatusCode, reason string) []byte {
	payload := make([]byte, 2+len(reason))
	binary.BigEndian.PutUint16(payload[:2], uint16(code))
	copy(payload[2:], reason)
	return payload
}

func setConnReadDeadlineFromContext(ctx context.Context, conn net.Conn) error {
	if conn == nil || ctx == nil {
		return nil
	}
	if deadline, ok := ctx.Deadline(); ok {
		return conn.SetReadDeadline(deadline)
	}
	return nil
}

func setConnWriteDeadlineFromContext(ctx context.Context, conn net.Conn) error {
	if conn == nil || ctx == nil {
		return nil
	}
	if deadline, ok := ctx.Deadline(); ok {
		return conn.SetWriteDeadline(deadline)
	}
	return nil
}

func clearConnReadDeadline(conn net.Conn) {
	if conn != nil {
		_ = conn.SetReadDeadline(time.Time{})
	}
}

func clearConnWriteDeadline(conn net.Conn) {
	if conn != nil {
		_ = conn.SetWriteDeadline(time.Time{})
	}
}

var _ openAIWSClientConn = (*openAIWSH2ClientConn)(nil)
