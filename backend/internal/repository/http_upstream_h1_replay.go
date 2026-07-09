package repository

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

type http1HeaderReplayRoundTripper struct {
	headerOrder        []string
	proxyURL           *url.URL
	profile            *tlsfingerprint.Profile
	validateResolvedIP bool
}

func (rt *http1HeaderReplayRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt == nil {
		return nil, fmt.Errorf("http1 header replay round tripper is required")
	}
	if req == nil || req.URL == nil {
		return nil, fmt.Errorf("http1 replay request url is required")
	}

	body, err := readHTTP1ReplayRequestBody(req)
	if err != nil {
		return nil, err
	}

	conn, err := rt.dialConn(req.Context(), req)
	if err != nil {
		return nil, err
	}

	if err := writeHTTP1ReplayRequest(conn, req, body, rt.headerOrder); err != nil {
		_ = conn.Close()
		return nil, err
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if resp.Body != nil {
		resp.Body = wrapTrackedBody(resp.Body, func() { _ = conn.Close() })
	} else {
		_ = conn.Close()
	}
	return resp, nil
}

func (rt *http1HeaderReplayRoundTripper) CloseIdleConnections() {}

func (rt *http1HeaderReplayRoundTripper) dialConn(ctx context.Context, req *http.Request) (net.Conn, error) {
	targetAddr := req.URL.Host
	if req.URL.Port() == "" {
		switch strings.ToLower(req.URL.Scheme) {
		case "https":
			targetAddr = net.JoinHostPort(req.URL.Hostname(), "443")
		default:
			targetAddr = net.JoinHostPort(req.URL.Hostname(), "80")
		}
	}

	switch strings.ToLower(req.URL.Scheme) {
	case "https":
		if rt.profile != nil {
			dialTLS, fallbackProxy, err := tlsFingerprintDialTLSFunc(rt.profile, rt.proxyURL, rt.validateResolvedIP)
			if err != nil {
				return nil, err
			}
			if fallbackProxy {
				return nil, fmt.Errorf("unsupported proxy scheme for http1 header replay: %s", rt.proxyURL.Scheme)
			}
			return dialTLS(ctx, "tcp", targetAddr)
		}
		return dialDefaultTLS(ctx, targetAddr, req.URL.Hostname(), rt.validateResolvedIP)
	case "http":
		if rt.proxyURL != nil {
			return nil, fmt.Errorf("raw http1 replay for plain http does not support proxy scheme %s", rt.proxyURL.Scheme)
		}
		return newUpstreamDialer(rt.validateResolvedIP).DialContext(ctx, "tcp", targetAddr)
	default:
		return nil, fmt.Errorf("unsupported scheme for http1 header replay: %s", req.URL.Scheme)
	}
}

func dialDefaultTLS(ctx context.Context, addr, serverName string, validateResolvedIP bool) (net.Conn, error) {
	rawConn, err := newUpstreamDialer(validateResolvedIP).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	cfg := &tls.Config{ServerName: serverName, MinVersion: tls.VersionTLS12}
	tlsConn := tls.Client(rawConn, cfg)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = rawConn.Close()
		return nil, err
	}
	return tlsConn, nil
}

func readHTTP1ReplayRequestBody(req *http.Request) ([]byte, error) {
	if req == nil || req.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func writeHTTP1ReplayRequest(w io.Writer, req *http.Request, body []byte, headerOrder []string) error {
	if req == nil || req.URL == nil {
		return fmt.Errorf("http1 replay request url is required")
	}
	path := req.URL.RequestURI()
	if path == "" {
		path = "/"
	}
	if _, err := fmt.Fprintf(w, "%s %s HTTP/1.1\r\n", req.Method, path); err != nil {
		return err
	}
	host := strings.TrimSpace(req.Host)
	if host == "" {
		host = req.URL.Host
	}
	if host != "" {
		if _, err := fmt.Fprintf(w, "Host: %s\r\n", host); err != nil {
			return err
		}
	}

	emitted := make(map[string]struct{}, len(req.Header)+2)
	for _, name := range headerOrder {
		if err := writeHTTP1ReplayHeaderValues(w, req.Header, name, emitted); err != nil {
			return err
		}
	}

	remaining := make([]string, 0, len(req.Header))
	for key := range req.Header {
		lower := strings.ToLower(strings.TrimSpace(key))
		if lower == "" || lower == "host" {
			continue
		}
		if _, ok := emitted[lower]; ok {
			continue
		}
		remaining = append(remaining, key)
	}
	sort.Slice(remaining, func(i, j int) bool {
		return strings.ToLower(remaining[i]) < strings.ToLower(remaining[j])
	})
	for _, key := range remaining {
		if err := writeHTTP1ReplayHeaderValues(w, req.Header, key, emitted); err != nil {
			return err
		}
	}

	if len(body) > 0 {
		if _, ok := emitted["content-length"]; !ok {
			if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n", len(body)); err != nil {
				return err
			}
		}
	}
	if req.Close {
		if _, ok := emitted["connection"]; !ok {
			if _, err := io.WriteString(w, "connection: close\r\n"); err != nil {
				return err
			}
		}
	}
	if _, err := io.WriteString(w, "\r\n"); err != nil {
		return err
	}
	if len(body) > 0 {
		_, err := w.Write(body)
		return err
	}
	return nil
}

func writeHTTP1ReplayHeaderValues(w io.Writer, header http.Header, desiredName string, emitted map[string]struct{}) error {
	desiredName = strings.TrimSpace(desiredName)
	if desiredName == "" {
		return nil
	}
	lower := strings.ToLower(desiredName)
	if lower == "host" {
		emitted[lower] = struct{}{}
		return nil
	}
	var values []string
	for key, currentValues := range header {
		if strings.EqualFold(strings.TrimSpace(key), desiredName) {
			values = currentValues
			break
		}
	}
	if len(values) == 0 {
		return nil
	}
	for _, value := range values {
		if _, err := fmt.Fprintf(w, "%s: %s\r\n", desiredName, value); err != nil {
			return err
		}
	}
	emitted[lower] = struct{}{}
	return nil
}
