package webfetch

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyurl"
	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyutil"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"golang.org/x/net/html"
	"golang.org/x/net/proxy"
)

const (
	defaultMaxRedirects   = 3
	defaultMaxContentSize = 64 * 1024
	defaultRequestTimeout = 20 * time.Second
	defaultDialTimeout    = 5 * time.Second
	defaultTLSHandshake   = 5 * time.Second
)

var errTooManyRedirects = errors.New("webfetch: too many redirects")
var validateResolvedFetchHost = urlvalidator.ValidateResolvedIP
var fetchLookupIPAddr = net.DefaultResolver.LookupIPAddr
var fetchBaseDialContext = (&net.Dialer{Timeout: defaultDialTimeout}).DialContext

type fetchDialContextFunc func(context.Context, string, string) (net.Conn, error)
type fetchLookupIPAddrFunc func(context.Context, string) ([]net.IPAddr, error)
type clientFactoryFunc func(FetchRequest) (*http.Client, error)

// Fetcher executes outbound web fetches with bounded redirects/content.
type Fetcher struct {
	clientFactory clientFactoryFunc
}

// NewFetcher creates a fetcher with the default proxy-aware HTTP client factory.
func NewFetcher() *Fetcher {
	return &Fetcher{clientFactory: newHTTPClientForRequest}
}

// Fetch executes a single GET request and returns a structured success/error result.
func (f *Fetcher) Fetch(ctx context.Context, req FetchRequest) *FetchResult {
	result := &FetchResult{RequestedURL: req.URL}

	normalizedURL, _, err := validateFetchURL(req.URL, req)
	if err != nil {
		result.Error = err
		return result
	}

	maxRedirects := req.MaxRedirects
	if maxRedirects < 0 {
		maxRedirects = 0
	}
	if maxRedirects == 0 {
		maxRedirects = defaultMaxRedirects
	}

	maxContentBytes := req.MaxContentBytes
	if maxContentBytes <= 0 {
		maxContentBytes = defaultMaxContentSize
	}

	factory := f.clientFactory
	if factory == nil {
		factory = newHTTPClientForRequest
	}

	client, clientErr := factory(req)
	if clientErr != nil {
		result.Error = &FetchError{
			Code:    ErrorCodeClientConfig,
			Message: clientErr.Error(),
			Reason:  "client_config",
		}
		return result
	}
	client.CheckRedirect = func(nextReq *http.Request, via []*http.Request) error {
		if len(via) > maxRedirects {
			return errTooManyRedirects
		}
		if nextReq == nil || nextReq.URL == nil {
			return nil
		}
		if _, _, redirectErr := validateFetchURL(nextReq.URL.String(), req); redirectErr != nil {
			return redirectErr
		}
		return nil
	}

	httpReq, buildErr := http.NewRequestWithContext(ctx, http.MethodGet, normalizedURL, nil)
	if buildErr != nil {
		result.Error = &FetchError{
			Code:    ErrorCodeInvalidURL,
			Message: buildErr.Error(),
		}
		return result
	}
	httpReq.Header.Set("Accept", "text/html, text/plain;q=0.9, */*;q=0.1")
	httpReq.Header.Set("User-Agent", "sub2api-webfetch/1.0")

	resp, doErr := client.Do(httpReq)
	if doErr != nil {
		result.Error = mapRequestError(doErr)
		return result
	}
	defer func() { _ = resp.Body.Close() }()

	result.FinalURL = normalizedURL
	if resp.Request != nil && resp.Request.URL != nil {
		result.FinalURL = resp.Request.URL.String()
	}
	if _, _, redirectErr := validateFetchURL(result.FinalURL, req); redirectErr != nil {
		result.Error = redirectErr
		return result
	}
	result.StatusCode = resp.StatusCode
	result.ContentType = canonicalContentType(resp.Header.Get("Content-Type"))

	body, truncated, readErr := readBoundedBody(resp.Body, maxContentBytes)
	if readErr != nil {
		result.Error = &FetchError{
			Code:      ErrorCodeReadFailed,
			Message:   readErr.Error(),
			Reason:    "read_failed",
			Retryable: true,
		}
		return result
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Error = &FetchError{
			Code:       ErrorCodeHTTPStatus,
			Message:    fmt.Sprintf("unexpected HTTP status %d", resp.StatusCode),
			StatusCode: resp.StatusCode,
			Reason:     "http_status",
			Retryable:  resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests,
		}
		return result
	}

	title, text := extractReadableContent(result.ContentType, body)
	if trimmedText, didTruncate := truncateUTF8String(text, maxContentBytes); didTruncate {
		text = trimmedText
		truncated = true
	}

	result.Title = title
	result.Text = text
	result.Truncated = truncated
	return result
}

func validateFetchURL(rawURL string, req FetchRequest) (normalizedURL, host string, fetchErr *FetchError) {
	normalizedURL, err := urlvalidator.ValidateHTTPURL(rawURL, req.AllowInsecureHTTP, urlvalidator.ValidationOptions{
		AllowedHosts: req.AllowedHosts,
		AllowPrivate: req.AllowPrivate,
	})
	if err != nil {
		code := ErrorCodeInvalidURL
		msg := err.Error()
		reason := "invalid_url"
		parsedHost := normalizedFetchHost(rawURL)
		if strings.Contains(msg, "host is not allowed") || strings.Contains(msg, "allowlist") {
			code = ErrorCodeDomainBlocked
			if isPrivateLiteralHost(parsedHost) {
				reason = "private_host"
			} else {
				reason = "blocked_host"
			}
		}
		return "", "", &FetchError{Code: code, Message: msg, Reason: reason}
	}

	parsed, err := url.Parse(normalizedURL)
	if err != nil || parsed.Hostname() == "" {
		return "", "", &FetchError{Code: ErrorCodeInvalidURL, Message: "invalid normalized url", Reason: "invalid_url"}
	}
	host = strings.ToLower(parsed.Hostname())
	if matchesHost(host, req.BlockedHosts) {
		return "", "", &FetchError{
			Code:    ErrorCodeDomainBlocked,
			Message: fmt.Sprintf("host is blocked: %s", host),
			Reason:  "blocked_host",
		}
	}
	if !req.AllowPrivate {
		if err := validateResolvedFetchHost(host); err != nil {
			return "", "", &FetchError{
				Code:    ErrorCodeDomainBlocked,
				Message: err.Error(),
				Reason:  "resolved_private_ip",
			}
		}
	}
	return normalizedURL, host, nil
}

// ValidateRequestURL applies the same URL/domain/SSRF checks used by Fetch,
// without performing the HTTP request. Callers that execute fetches through a
// remote backend can use this to enforce the local tool contract first.
func ValidateRequestURL(rawURL string, req FetchRequest) (normalizedURL, host string, fetchErr *FetchError) {
	return validateFetchURL(rawURL, req)
}

func newHTTPClientForRequest(req FetchRequest) (*http.Client, error) {
	return newHTTPClient(req.ProxyURL, req.AllowPrivate)
}

func newHTTPClient(proxyURL string, allowPrivate bool) (*http.Client, error) {
	_, parsedProxy, err := proxyurl.Parse(proxyURL)
	if err != nil {
		return nil, err
	}
	if parsedProxy != nil && !allowPrivate {
		return &http.Client{
			Transport: newFetchValidatedProxyRoundTripper(parsedProxy),
			Timeout:   defaultRequestTimeout,
		}, nil
	}

	transport := &http.Transport{
		DialContext:           fetchBaseDialContext,
		TLSHandshakeTimeout:   defaultTLSHandshake,
		ResponseHeaderTimeout: defaultRequestTimeout / 2,
		ForceAttemptHTTP2:     true,
	}
	if parsedProxy == nil && !allowPrivate {
		transport.DialContext = newFetchValidatedDialContext(fetchBaseDialContext).DialContext
	}
	if err := proxyutil.ConfigureTransportProxy(transport, parsedProxy); err != nil {
		return nil, fmt.Errorf("configure proxy: %w", err)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   defaultRequestTimeout,
	}, nil
}

type fetchValidatedDialer struct {
	base   fetchDialContextFunc
	lookup fetchLookupIPAddrFunc
}

func newFetchValidatedDialContext(base fetchDialContextFunc) *fetchValidatedDialer {
	if base == nil {
		base = fetchBaseDialContext
	}
	return &fetchValidatedDialer{
		base:   base,
		lookup: fetchLookupIPAddr,
	}
}

func (d *fetchValidatedDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if d == nil || d.base == nil {
		return nil, &FetchError{
			Code:    ErrorCodeClientConfig,
			Message: "validated dialer base is nil",
			Reason:  "client_config",
		}
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return nil, &FetchError{Code: ErrorCodeInvalidURL, Message: "validated dialer host is empty", Reason: "invalid_url"}
	}

	selectedIP, err := resolveFetchValidatedIP(ctx, host, d.lookup)
	if err != nil {
		return nil, &FetchError{
			Code:    ErrorCodeDomainBlocked,
			Message: err.Error(),
			Reason:  "resolved_private_ip",
		}
	}
	return d.base(ctx, network, net.JoinHostPort(selectedIP, port))
}

func resolveFetchValidatedIP(ctx context.Context, host string, lookup fetchLookupIPAddrFunc) (string, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return "", fmt.Errorf("validated dialer host is empty")
	}

	if addr, err := netip.ParseAddr(host); err == nil {
		if urlvalidator.IsBlockedResolvedAddr(addr) {
			return "", fmt.Errorf("resolved ip %s is not allowed", addr.String())
		}
		return addr.Unmap().String(), nil
	}

	if lookup == nil {
		lookup = fetchLookupIPAddr
	}
	addrs, err := lookup(ctx, host)
	if err != nil {
		return "", fmt.Errorf("dns resolution failed: %w", err)
	}
	if len(addrs) == 0 {
		return "", fmt.Errorf("dns resolution returned no addresses for %s", host)
	}

	selected := ""
	for _, addr := range addrs {
		if isBlockedResolvedFetchIP(addr.IP) {
			return "", fmt.Errorf("resolved ip %s is not allowed", addr.IP.String())
		}
		if selected == "" {
			selected = addr.IP.String()
		}
	}
	if selected == "" {
		return "", fmt.Errorf("dns resolution returned no usable addresses for %s", host)
	}
	return selected, nil
}

func isBlockedResolvedFetchIP(ip net.IP) bool {
	return urlvalidator.IsBlockedResolvedIP(ip)
}

type fetchValidatedProxyRoundTripper struct {
	proxyURL *url.URL
	baseDial fetchDialContextFunc
	lookup   fetchLookupIPAddrFunc
}

func newFetchValidatedProxyRoundTripper(proxyURL *url.URL) *fetchValidatedProxyRoundTripper {
	return &fetchValidatedProxyRoundTripper{
		proxyURL: proxyURL,
		baseDial: fetchBaseDialContext,
		lookup:   fetchLookupIPAddr,
	}
}

func (rt *fetchValidatedProxyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt == nil || rt.proxyURL == nil {
		return nil, &FetchError{Code: ErrorCodeClientConfig, Message: "validated proxy URL is nil", Reason: "client_config"}
	}
	if req == nil || req.URL == nil {
		return nil, &FetchError{Code: ErrorCodeInvalidURL, Message: "request URL is nil", Reason: "invalid_url"}
	}

	originalHost := strings.ToLower(strings.TrimSpace(req.URL.Hostname()))
	if originalHost == "" {
		return nil, &FetchError{Code: ErrorCodeInvalidURL, Message: "validated proxy target host is empty", Reason: "invalid_url"}
	}
	selectedIP, err := resolveFetchValidatedIP(req.Context(), originalHost, rt.lookup)
	if err != nil {
		return nil, &FetchError{Code: ErrorCodeDomainBlocked, Message: err.Error(), Reason: "resolved_private_ip"}
	}

	outboundURL := *req.URL
	outboundURL.Host = replaceURLHostWithIP(req.URL, selectedIP)
	targetAddr := canonicalFetchTargetAddr(req.URL, selectedIP)
	if targetAddr == "" {
		return nil, &FetchError{Code: ErrorCodeInvalidURL, Message: "target address is empty", Reason: "invalid_url"}
	}

	scheme := strings.ToLower(rt.proxyURL.Scheme)
	switch scheme {
	case "http", "https":
		return rt.roundTripViaHTTPProxy(req, &outboundURL, targetAddr, originalHost)
	case "socks5", "socks5h":
		return rt.roundTripViaSOCKSProxy(req, targetAddr, originalHost)
	default:
		return nil, &FetchError{Code: ErrorCodeClientConfig, Message: "unsupported proxy scheme: " + rt.proxyURL.Scheme, Reason: "client_config"}
	}
}

func (rt *fetchValidatedProxyRoundTripper) roundTripViaHTTPProxy(req *http.Request, outboundURL *url.URL, targetAddr, serverName string) (*http.Response, error) {
	conn, err := rt.dialHTTPProxy(req.Context())
	if err != nil {
		return nil, err
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			_ = conn.Close()
		}
	}()

	if strings.EqualFold(req.URL.Scheme, "https") {
		if err := writeFetchConnectRequest(conn, targetAddr, proxyAuthorizationHeader(rt.proxyURL)); err != nil {
			return nil, err
		}
		connectResp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
		if err != nil {
			return nil, err
		}
		_ = connectResp.Body.Close()
		if connectResp.StatusCode < 200 || connectResp.StatusCode >= 300 {
			return nil, fmt.Errorf("proxy CONNECT failed with status %d", connectResp.StatusCode)
		}
		tlsConn := tls.Client(conn, &tls.Config{ServerName: serverName})
		if err := tlsConn.HandshakeContext(req.Context()); err != nil {
			return nil, err
		}
		conn = tlsConn
		if err := writeFetchOriginRequest(conn, req, req.URL, req.URL.Host); err != nil {
			return nil, err
		}
	} else {
		if err := writeFetchProxyRequest(conn, req, outboundURL, req.URL.Host, proxyAuthorizationHeader(rt.proxyURL)); err != nil {
			return nil, err
		}
	}

	resp, err := readFetchResponse(conn, req)
	if err != nil {
		return nil, err
	}
	closeOnError = false
	return resp, nil
}

func (rt *fetchValidatedProxyRoundTripper) roundTripViaSOCKSProxy(req *http.Request, targetAddr, serverName string) (*http.Response, error) {
	dialer, err := proxy.FromURL(rt.proxyURL, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("create socks5 dialer: %w", err)
	}

	var conn net.Conn
	if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
		conn, err = contextDialer.DialContext(req.Context(), "tcp", targetAddr)
	} else {
		conn, err = dialer.Dial("tcp", targetAddr)
	}
	if err != nil {
		return nil, err
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			_ = conn.Close()
		}
	}()

	if strings.EqualFold(req.URL.Scheme, "https") {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: serverName})
		if err := tlsConn.HandshakeContext(req.Context()); err != nil {
			return nil, err
		}
		conn = tlsConn
	}
	if err := writeFetchOriginRequest(conn, req, req.URL, req.URL.Host); err != nil {
		return nil, err
	}
	resp, err := readFetchResponse(conn, req)
	if err != nil {
		return nil, err
	}
	closeOnError = false
	return resp, nil
}

func (rt *fetchValidatedProxyRoundTripper) dialHTTPProxy(ctx context.Context) (net.Conn, error) {
	addr := canonicalProxyAddr(rt.proxyURL)
	if addr == "" {
		return nil, &FetchError{Code: ErrorCodeClientConfig, Message: "proxy address is empty", Reason: "client_config"}
	}
	baseDial := rt.baseDial
	if baseDial == nil {
		baseDial = fetchBaseDialContext
	}
	conn, err := baseDial(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(rt.proxyURL.Scheme, "https") {
		return conn, nil
	}
	tlsConn := tls.Client(conn, &tls.Config{ServerName: rt.proxyURL.Hostname()})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return tlsConn, nil
}

func replaceURLHostWithIP(u *url.URL, ip string) string {
	if u == nil {
		return ip
	}
	port := u.Port()
	if port != "" {
		return net.JoinHostPort(ip, port)
	}
	if strings.Contains(ip, ":") {
		return "[" + ip + "]"
	}
	return ip
}

func canonicalFetchTargetAddr(u *url.URL, ip string) string {
	if u == nil || ip == "" {
		return ""
	}
	port := u.Port()
	if port == "" {
		switch strings.ToLower(u.Scheme) {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return ""
		}
	}
	return net.JoinHostPort(ip, port)
}

func canonicalProxyAddr(proxyURL *url.URL) string {
	if proxyURL == nil || proxyURL.Hostname() == "" {
		return ""
	}
	port := proxyURL.Port()
	if port == "" {
		if strings.EqualFold(proxyURL.Scheme, "https") {
			port = "443"
		} else {
			port = "80"
		}
	}
	return net.JoinHostPort(proxyURL.Hostname(), port)
}

func writeFetchConnectRequest(w io.Writer, targetAddr, proxyAuth string) error {
	if _, err := fmt.Fprintf(w, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n", targetAddr, targetAddr); err != nil {
		return err
	}
	if proxyAuth != "" {
		if _, err := fmt.Fprintf(w, "Proxy-Authorization: %s\r\n", proxyAuth); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "\r\n")
	return err
}

func writeFetchProxyRequest(w io.Writer, req *http.Request, outboundURL *url.URL, host, proxyAuth string) error {
	if _, err := fmt.Fprintf(w, "%s %s HTTP/1.1\r\n", req.Method, outboundURL.String()); err != nil {
		return err
	}
	return writeFetchHeaders(w, req, host, proxyAuth)
}

func writeFetchOriginRequest(w io.Writer, req *http.Request, targetURL *url.URL, host string) error {
	requestURI := "/"
	if targetURL != nil && targetURL.RequestURI() != "" {
		requestURI = targetURL.RequestURI()
	}
	if _, err := fmt.Fprintf(w, "%s %s HTTP/1.1\r\n", req.Method, requestURI); err != nil {
		return err
	}
	return writeFetchHeaders(w, req, host, "")
}

func writeFetchHeaders(w io.Writer, req *http.Request, host, proxyAuth string) error {
	if _, err := fmt.Fprintf(w, "Host: %s\r\nConnection: close\r\n", host); err != nil {
		return err
	}
	if proxyAuth != "" {
		if _, err := fmt.Fprintf(w, "Proxy-Authorization: %s\r\n", proxyAuth); err != nil {
			return err
		}
	}
	exclude := map[string]bool{
		"Connection":          true,
		"Host":                true,
		"Proxy-Authorization": true,
	}
	if err := req.Header.WriteSubset(w, exclude); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\r\n")
	return err
}

func readFetchResponse(conn net.Conn, req *http.Request) (*http.Response, error) {
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return nil, err
	}
	resp.Request = req
	resp.Body = &fetchConnBody{ReadCloser: resp.Body, conn: conn}
	return resp, nil
}

type fetchConnBody struct {
	io.ReadCloser
	conn net.Conn
}

func (b *fetchConnBody) Close() error {
	bodyErr := b.ReadCloser.Close()
	connErr := b.conn.Close()
	if bodyErr != nil {
		return bodyErr
	}
	return connErr
}

func proxyAuthorizationHeader(proxyURL *url.URL) string {
	if proxyURL == nil || proxyURL.User == nil {
		return ""
	}
	username := proxyURL.User.Username()
	password, _ := proxyURL.User.Password()
	token := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	return "Basic " + token
}

func mapRequestError(err error) *FetchError {
	var fetchErr *FetchError
	if errors.As(err, &fetchErr) && fetchErr != nil {
		return fetchErr
	}
	switch {
	case errors.Is(err, errTooManyRedirects):
		return &FetchError{
			Code:      ErrorCodeTooManyRedirects,
			Message:   "redirect limit exceeded",
			Reason:    "too_many_redirects",
			Retryable: true,
		}
	case errors.Is(err, context.DeadlineExceeded):
		return &FetchError{
			Code:      ErrorCodeRequestFailed,
			Message:   err.Error(),
			Reason:    "request_failed",
			Retryable: true,
		}
	default:
		return &FetchError{
			Code:      ErrorCodeRequestFailed,
			Message:   err.Error(),
			Reason:    "request_failed",
			Retryable: !errors.Is(err, context.Canceled),
		}
	}
}

func normalizedFetchHost(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(parsed.Hostname()))
}

func isPrivateLiteralHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return isBlockedResolvedFetchIP(ip)
}

func readBoundedBody(r io.Reader, maxBytes int) ([]byte, bool, error) {
	limited, err := io.ReadAll(io.LimitReader(r, int64(maxBytes+1)))
	if err != nil {
		return nil, false, err
	}
	if len(limited) > maxBytes {
		return limited[:maxBytes], true, nil
	}
	return limited, false, nil
}

func canonicalContentType(raw string) string {
	if raw == "" {
		return "application/octet-stream"
	}
	mediaType, _, err := mime.ParseMediaType(raw)
	if err != nil || mediaType == "" {
		return raw
	}
	return strings.ToLower(mediaType)
}

func extractReadableContent(contentType string, body []byte) (title, text string) {
	switch {
	case contentType == "text/html" || contentType == "application/xhtml+xml":
		return extractHTMLText(body)
	case strings.HasPrefix(contentType, "text/"):
		return "", bytesToText(body)
	default:
		return "", bytesToText(body)
	}
}

func extractHTMLText(body []byte) (title, text string) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", normalizeWhitespace(bytesToText(body))
	}

	title = strings.TrimSpace(findTitle(doc))
	parts := make([]string, 0, 16)
	var walk func(*html.Node, bool)
	walk = func(n *html.Node, skip bool) {
		if n == nil {
			return
		}
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "head", "script", "style", "noscript", "template", "svg":
				skip = true
			case "br", "p", "div", "section", "article", "main", "li", "ul", "ol", "h1", "h2", "h3", "h4", "h5", "h6":
				parts = append(parts, "\n")
			}
		}
		if !skip && n.Type == html.TextNode {
			if cleaned := normalizeWhitespace(n.Data); cleaned != "" {
				parts = append(parts, cleaned)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child, skip)
		}
	}
	walk(doc, false)

	text = normalizeWhitespace(strings.Join(parts, " "))
	if text == "" {
		text = normalizeWhitespace(bytesToText(body))
	}
	return title, text
}

func findTitle(n *html.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == html.ElementNode && strings.EqualFold(n.Data, "title") {
		return extractNodeText(n)
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if title := findTitle(child); title != "" {
			return title
		}
	}
	return ""
}

func extractNodeText(n *html.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == html.TextNode {
		return n.Data
	}
	var parts []string
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if text := extractNodeText(child); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " ")
}

func bytesToText(body []byte) string {
	return string(bytes.ToValidUTF8(body, []byte{}))
}

func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func truncateUTF8String(s string, maxBytes int) (string, bool) {
	if len(s) <= maxBytes {
		return s, false
	}
	cut := 0
	for idx := range s {
		if idx > maxBytes {
			break
		}
		cut = idx
	}
	if cut == 0 {
		return "", true
	}
	return s[:cut], true
}

func matchesHost(host string, patterns []string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	for _, raw := range patterns {
		pattern := normalizeHostPattern(raw)
		if pattern == "" {
			continue
		}
		if strings.HasPrefix(pattern, "*.") {
			suffix := strings.TrimPrefix(pattern, "*.")
			if host == suffix || strings.HasSuffix(host, "."+suffix) {
				return true
			}
			continue
		}
		if host == pattern {
			return true
		}
	}
	return false
}

func normalizeHostPattern(value string) string {
	pattern := strings.ToLower(strings.TrimSpace(value))
	if pattern == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(pattern); err == nil {
		pattern = host
	}
	return pattern
}
