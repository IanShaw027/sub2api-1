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

const (
	openAIWSH2DefaultMaxFrameSize = 16384
	openAIWSH2InitialWindowSize   = 65535
	openAIWSH2MaxWindowSize       = 1<<31 - 1
	openAIWSH2ControlWriteTimeout = 5 * time.Second
	openAIWSH2CloseTimeout        = time.Second
)

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
	peerInitialWindowSize := int64(openAIWSH2InitialWindowSize)
	connSendWindow := int64(openAIWSH2InitialWindowSize)
	streamSendWindow := int64(openAIWSH2InitialWindowSize)
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
					switch s.ID {
					case http2.SettingMaxFrameSize:
						if v := int(s.Val); v >= openAIWSH2DefaultMaxFrameSize && v <= 16777215 {
							peerMaxFrameSize = v
						}
					case http2.SettingInitialWindowSize:
						next := int64(s.Val)
						streamSendWindow += next - peerInitialWindowSize
						if streamSendWindow > openAIWSH2MaxWindowSize {
							return fmt.Errorf("websocket h2 stream send window overflow during handshake")
						}
						peerInitialWindowSize = next
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
			clientConn := &openAIWSH2ClientConn{
				conn:                  conn,
				readFr:                readFr,
				writeFr:               writeFr,
				streamID:              1,
				peerMaxFrameSize:      peerMaxFrameSize,
				peerInitialWindowSize: peerInitialWindowSize,
				connSendWindow:        connSendWindow,
				streamSendWindow:      streamSendWindow,
				stateChanged:          make(chan struct{}),
				pendingPings:          make(map[string]chan error),
				messageWriteToken:     make(chan struct{}, 1),
			}
			clientConn.messageWriteToken <- struct{}{}
			go clientConn.readLoop()
			return clientConn, 0, respHeaders, nil
		case *http2.WindowUpdateFrame:
			if f.StreamID == 0 {
				connSendWindow += int64(f.Increment)
			} else if f.StreamID == 1 {
				streamSendWindow += int64(f.Increment)
			}
			if connSendWindow > openAIWSH2MaxWindowSize || streamSendWindow > openAIWSH2MaxWindowSize {
				return nil, 0, nil, fmt.Errorf("websocket h2 send window overflow during handshake")
			}
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
	closeMu sync.Mutex
	stateMu sync.Mutex

	closed                bool
	readErr               error
	peerMaxFrameSize      int
	peerInitialWindowSize int64
	connSendWindow        int64
	streamSendWindow      int64
	stateChanged          chan struct{}
	pendingPings          map[string]chan error
	messages              [][]byte
	messageWriteToken     chan struct{}
	readBuffer            []byte
	fragmentOpcode        byte
	messageBuffer         []byte
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
	for {
		c.stateMu.Lock()
		if len(c.messages) > 0 {
			message := c.messages[0]
			c.messages[0] = nil
			c.messages = c.messages[1:]
			c.stateMu.Unlock()
			return message, nil
		}
		if c.readErr != nil {
			err := c.readErr
			c.stateMu.Unlock()
			return nil, err
		}
		changed := c.stateChanged
		c.stateMu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-changed:
		}
	}
}

func (c *openAIWSH2ClientConn) Ping(ctx context.Context) error {
	if c == nil || c.conn == nil {
		return errOpenAIWSConnClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var payload [8]byte
	if _, err := rand.Read(payload[:]); err != nil {
		return err
	}
	key := string(payload[:])
	result := make(chan error, 1)
	c.stateMu.Lock()
	if c.readErr != nil {
		err := c.readErr
		c.stateMu.Unlock()
		return err
	}
	c.pendingPings[key] = result
	c.stateMu.Unlock()
	if err := c.writeWebSocketFrame(ctx, 0x9, payload[:]); err != nil {
		c.removePendingPing(key, result)
		return err
	}
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		c.removePendingPing(key, result)
		return ctx.Err()
	}
}

func (*openAIWSH2ClientConn) SupportsIdlePingWithoutReader() bool { return true }

func (c *openAIWSH2ClientConn) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	c.stateMu.Lock()
	if c.closed {
		c.stateMu.Unlock()
		return nil
	}
	c.stateMu.Unlock()

	closeCtx, cancel := context.WithTimeout(context.Background(), openAIWSH2CloseTimeout)
	_ = c.writeWebSocketFrame(closeCtx, 0x8, openAIWSWebSocketClosePayload(coderws.StatusNormalClosure, ""))
	cancel()
	c.stateMu.Lock()
	c.closed = true
	c.failLocked(errOpenAIWSConnClosed)
	c.stateMu.Unlock()
	_ = c.conn.Close()
	return nil
}

func (c *openAIWSH2ClientConn) writeWebSocketFrame(ctx context.Context, opcode byte, payload []byte) error {
	if c == nil || c.conn == nil {
		return errOpenAIWSConnClosed
	}
	frame, err := encodeOpenAIWSWebSocketClientFrame(opcode, payload)
	if err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.messageWriteToken:
	}
	defer func() { c.messageWriteToken <- struct{}{} }()
	for len(frame) > 0 {
		chunkSize, err := c.reserveSendWindow(ctx, len(frame))
		if err != nil {
			return err
		}
		chunk := frame[:chunkSize]
		frame = frame[chunkSize:]
		c.writeMu.Lock()
		if err := setConnWriteDeadlineFromContext(ctx, c.conn); err != nil {
			c.writeMu.Unlock()
			return err
		}
		if err := c.writeFr.WriteData(c.streamID, false, chunk); err != nil {
			clearConnWriteDeadline(c.conn)
			c.writeMu.Unlock()
			return err
		}
		clearConnWriteDeadline(c.conn)
		c.writeMu.Unlock()
	}
	return nil
}

func (c *openAIWSH2ClientConn) writeWindowUpdates(size uint32) error {
	if c == nil || size == 0 {
		return nil
	}
	return c.writeControlFrames(func() error {
		if err := c.writeFr.WriteWindowUpdate(0, size); err != nil {
			return err
		}
		return c.writeFr.WriteWindowUpdate(c.streamID, size)
	})
}

func (c *openAIWSH2ClientConn) readLoop() {
	defer func() { _ = c.conn.Close() }()
	for {
		frame, err := c.readFr.ReadFrame()
		if err != nil {
			c.fail(err)
			return
		}
		if err := c.handleH2Frame(frame); err != nil {
			c.fail(err)
			return
		}
	}
}

func (c *openAIWSH2ClientConn) handleH2Frame(frame http2.Frame) error {
	switch f := frame.(type) {
	case *http2.DataFrame:
		if f.StreamID != c.streamID {
			return nil
		}
		if err := c.writeWindowUpdates(uint32(len(f.Data()))); err != nil {
			return err
		}
		if err := c.handleWebSocketBytes(f.Data()); err != nil {
			return err
		}
		if f.StreamEnded() {
			return errOpenAIWSConnClosed
		}
	case *http2.SettingsFrame:
		if f.IsAck() {
			return nil
		}
		c.stateMu.Lock()
		err := f.ForeachSetting(func(s http2.Setting) error {
			switch s.ID {
			case http2.SettingMaxFrameSize:
				if v := int(s.Val); v >= openAIWSH2DefaultMaxFrameSize && v <= 16777215 {
					c.peerMaxFrameSize = v
				}
			case http2.SettingInitialWindowSize:
				next := int64(s.Val)
				c.streamSendWindow += next - c.peerInitialWindowSize
				if c.streamSendWindow > openAIWSH2MaxWindowSize {
					return fmt.Errorf("websocket h2 stream send window overflow")
				}
				c.peerInitialWindowSize = next
			}
			return nil
		})
		c.notifyStateLocked()
		c.stateMu.Unlock()
		if err != nil {
			return err
		}
		return c.writeControlFrames(c.writeFr.WriteSettingsAck)
	case *http2.WindowUpdateFrame:
		c.stateMu.Lock()
		var window *int64
		if f.StreamID == 0 {
			window = &c.connSendWindow
		} else if f.StreamID == c.streamID {
			window = &c.streamSendWindow
		}
		if window != nil {
			if *window+int64(f.Increment) > openAIWSH2MaxWindowSize {
				c.stateMu.Unlock()
				return fmt.Errorf("websocket h2 send window overflow")
			}
			*window += int64(f.Increment)
			c.notifyStateLocked()
		}
		c.stateMu.Unlock()
	case *http2.PingFrame:
		if !f.Flags.Has(http2.FlagPingAck) {
			return c.writeControlFrames(func() error { return c.writeFr.WritePing(true, f.Data) })
		}
	case *http2.RSTStreamFrame:
		if f.StreamID == c.streamID {
			return fmt.Errorf("websocket h2 stream reset: %v", f.ErrCode)
		}
	case *http2.GoAwayFrame:
		return fmt.Errorf("websocket h2 received GOAWAY: %v", f.ErrCode)
	}
	return nil
}

func (c *openAIWSH2ClientConn) handleWebSocketBytes(data []byte) error {
	c.readBuffer = append(c.readBuffer, data...)
	for {
		opcode, fin, payload, remaining, ok, err := decodeOpenAIWSWebSocketServerFrame(c.readBuffer)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		c.readBuffer = remaining
		switch opcode {
		case 0x1, 0x2:
			if fin {
				c.enqueueMessage(payload)
				continue
			}
			c.fragmentOpcode = opcode
			c.messageBuffer = append(c.messageBuffer[:0], payload...)
		case 0x0:
			if c.fragmentOpcode == 0 {
				continue
			}
			c.messageBuffer = append(c.messageBuffer, payload...)
			if fin {
				c.enqueueMessage(append([]byte(nil), c.messageBuffer...))
				c.fragmentOpcode = 0
				c.messageBuffer = nil
			}
		case 0x8:
			return errOpenAIWSConnClosed
		case 0x9:
			pongCtx, cancel := context.WithTimeout(context.Background(), openAIWSH2ControlWriteTimeout)
			err := c.writeWebSocketFrame(pongCtx, 0xA, payload)
			cancel()
			if err != nil {
				return err
			}
		case 0xA:
			c.resolvePendingPing(string(payload), nil)
		}
	}
}

func (c *openAIWSH2ClientConn) enqueueMessage(message []byte) {
	c.stateMu.Lock()
	c.messages = append(c.messages, append([]byte(nil), message...))
	c.notifyStateLocked()
	c.stateMu.Unlock()
}

func (c *openAIWSH2ClientConn) reserveSendWindow(ctx context.Context, remaining int) (int, error) {
	for {
		c.stateMu.Lock()
		if c.readErr != nil {
			err := c.readErr
			c.stateMu.Unlock()
			return 0, err
		}
		available := c.connSendWindow
		if c.streamSendWindow < available {
			available = c.streamSendWindow
		}
		maxFrameSize := c.peerMaxFrameSize
		if maxFrameSize < openAIWSH2DefaultMaxFrameSize || maxFrameSize > 16777215 {
			maxFrameSize = openAIWSH2DefaultMaxFrameSize
		}
		if available > 0 {
			chunkSize := int(available)
			if chunkSize > maxFrameSize {
				chunkSize = maxFrameSize
			}
			if chunkSize > remaining {
				chunkSize = remaining
			}
			c.connSendWindow -= int64(chunkSize)
			c.streamSendWindow -= int64(chunkSize)
			c.stateMu.Unlock()
			return chunkSize, nil
		}
		changed := c.stateChanged
		c.stateMu.Unlock()
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-changed:
		}
	}
}

func (c *openAIWSH2ClientConn) writeControlFrames(write func() error) error {
	ctx, cancel := context.WithTimeout(context.Background(), openAIWSH2ControlWriteTimeout)
	defer cancel()
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := setConnWriteDeadlineFromContext(ctx, c.conn); err != nil {
		return err
	}
	defer clearConnWriteDeadline(c.conn)
	return write()
}

func (c *openAIWSH2ClientConn) notifyStateLocked() {
	close(c.stateChanged)
	c.stateChanged = make(chan struct{})
}

func (c *openAIWSH2ClientConn) fail(err error) {
	if err == nil {
		err = errOpenAIWSConnClosed
	}
	c.stateMu.Lock()
	c.failLocked(err)
	c.stateMu.Unlock()
}

func (c *openAIWSH2ClientConn) failLocked(err error) {
	if c.readErr != nil {
		return
	}
	c.readErr = err
	for key, result := range c.pendingPings {
		result <- err
		delete(c.pendingPings, key)
	}
	c.notifyStateLocked()
}

func (c *openAIWSH2ClientConn) resolvePendingPing(key string, err error) {
	c.stateMu.Lock()
	result := c.pendingPings[key]
	if result != nil {
		delete(c.pendingPings, key)
		result <- err
	}
	c.stateMu.Unlock()
}

func (c *openAIWSH2ClientConn) removePendingPing(key string, result chan error) {
	c.stateMu.Lock()
	if c.pendingPings[key] == result {
		delete(c.pendingPings, key)
	}
	c.stateMu.Unlock()
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
