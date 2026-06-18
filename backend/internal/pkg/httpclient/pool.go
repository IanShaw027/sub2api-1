// Package httpclient 提供共享 HTTP 客户端池
//
// 性能优化说明：
// 原实现在多个服务中重复创建 http.Client：
// 1. proxy_probe_service.go: 每次探测创建新客户端
// 2. pricing_service.go: 每次请求创建新客户端
// 3. turnstile_service.go: 每次验证创建新客户端
// 4. github_release_service.go: 每次请求创建新客户端
// 5. claude_usage_service.go: 每次请求创建新客户端
//
// 新实现使用统一的客户端池：
// 1. 相同配置复用同一 http.Client 实例
// 2. 复用 Transport 连接池，减少 TCP/TLS 握手开销
// 3. 支持 HTTP/HTTPS/SOCKS5/SOCKS5H 代理
// 4. 代理配置失败时直接返回错误，不会回退到直连（避免 IP 关联风险）
package httpclient

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyurl"
	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyutil"
	"golang.org/x/net/proxy"
)

// Transport 连接池默认配置
const (
	defaultMaxIdleConns        = 100              // 最大空闲连接数
	defaultMaxIdleConnsPerHost = 10               // 每个主机最大空闲连接数
	defaultIdleConnTimeout     = 90 * time.Second // 空闲连接超时时间（建议小于上游 LB 超时）
	defaultDialTimeout         = 5 * time.Second  // TCP 连接超时（含代理握手），代理不通时快速失败
	defaultTLSHandshakeTimeout = 5 * time.Second  // TLS 握手超时
	validatedHostTTL           = 30 * time.Second // DNS Rebinding 校验缓存 TTL
)

var blockedResolvedIPPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
}

// Options 定义共享 HTTP 客户端的构建参数
type Options struct {
	ProxyURL              string        // 代理 URL（支持 http/https/socks5/socks5h）
	Timeout               time.Duration // 请求总超时时间
	ResponseHeaderTimeout time.Duration // 等待响应头超时时间
	InsecureSkipVerify    bool          // 是否跳过 TLS 证书验证（已禁用，不允许设置为 true）
	ValidateResolvedIP    bool          // 是否校验解析后的 IP（防止 DNS Rebinding）
	AllowPrivateHosts     bool          // 允许私有地址解析（与 ValidateResolvedIP 一起使用）

	// 可选的连接池参数（不设置则使用默认值）
	MaxIdleConns        int // 最大空闲连接总数（默认 100）
	MaxIdleConnsPerHost int // 每主机最大空闲连接（默认 10）
	MaxConnsPerHost     int // 每主机最大连接数（默认 0 无限制）
}

// sharedClients 存储按配置参数缓存的 http.Client 实例
var sharedClients sync.Map

// 允许测试替换 DNS 解析入口，生产默认走系统解析器。
var validatedLookupIPAddr = net.DefaultResolver.LookupIPAddr

// 允许测试替换底层直连拨号入口，生产默认使用 net.Dialer。
var validatedBaseDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
	return (&net.Dialer{Timeout: defaultDialTimeout}).DialContext(ctx, network, addr)
}

// GetClient 返回共享的 HTTP 客户端实例
// 性能优化：相同配置复用同一客户端，避免重复创建 Transport
// 安全说明：代理配置失败时直接返回错误，不会回退到直连，避免 IP 关联风险
func GetClient(opts Options) (*http.Client, error) {
	key := buildClientKey(opts)
	if cached, ok := sharedClients.Load(key); ok {
		if client, ok := cached.(*http.Client); ok {
			return client, nil
		}
	}

	client, err := buildClient(opts)
	if err != nil {
		return nil, err
	}

	actual, _ := sharedClients.LoadOrStore(key, client)
	if c, ok := actual.(*http.Client); ok {
		return c, nil
	}
	return client, nil
}

func buildClient(opts Options) (*http.Client, error) {
	transport, err := buildTransport(opts)
	if err != nil {
		return nil, err
	}

	var rt http.RoundTripper = transport
	if opts.ValidateResolvedIP && !opts.AllowPrivateHosts {
		_, parsed, err := proxyurl.Parse(opts.ProxyURL)
		if err != nil {
			return nil, err
		}
		if parsed != nil {
			rt = newValidatedProxyRoundTripper(parsed)
		}
	}
	return &http.Client{
		Transport: rt,
		Timeout:   opts.Timeout,
	}, nil
}

func buildTransport(opts Options) (*http.Transport, error) {
	// 使用自定义值或默认值
	maxIdleConns := opts.MaxIdleConns
	if maxIdleConns <= 0 {
		maxIdleConns = defaultMaxIdleConns
	}
	maxIdleConnsPerHost := opts.MaxIdleConnsPerHost
	if maxIdleConnsPerHost <= 0 {
		maxIdleConnsPerHost = defaultMaxIdleConnsPerHost
	}

	transport := &http.Transport{
		DialContext:           validatedBaseDialContext,
		TLSHandshakeTimeout:   defaultTLSHandshakeTimeout,
		MaxIdleConns:          maxIdleConns,
		MaxIdleConnsPerHost:   maxIdleConnsPerHost,
		MaxConnsPerHost:       opts.MaxConnsPerHost, // 0 表示无限制
		IdleConnTimeout:       defaultIdleConnTimeout,
		ResponseHeaderTimeout: opts.ResponseHeaderTimeout,
	}

	if opts.InsecureSkipVerify {
		// 安全要求：禁止跳过证书验证，避免中间人攻击。
		return nil, fmt.Errorf("insecure_skip_verify is not allowed; install a trusted certificate instead")
	}

	_, parsed, err := proxyurl.Parse(opts.ProxyURL)
	if err != nil {
		return nil, err
	}
	if parsed == nil {
		if opts.ValidateResolvedIP && !opts.AllowPrivateHosts {
			transport.DialContext = newValidatedDialContext(validatedBaseDialContext).DialContext
		}
		return transport, nil
	}

	if err := proxyutil.ConfigureTransportProxy(transport, parsed); err != nil {
		return nil, err
	}

	return transport, nil
}

func buildClientKey(opts Options) string {
	return fmt.Sprintf("%s|%s|%s|%t|%t|%t|%d|%d|%d",
		strings.TrimSpace(opts.ProxyURL),
		opts.Timeout.String(),
		opts.ResponseHeaderTimeout.String(),
		opts.InsecureSkipVerify,
		opts.ValidateResolvedIP,
		opts.AllowPrivateHosts,
		opts.MaxIdleConns,
		opts.MaxIdleConnsPerHost,
		opts.MaxConnsPerHost,
	)
}

type validatedTransport struct {
	base   func(context.Context, string, string) (net.Conn, error)
	lookup func(context.Context, string) ([]net.IPAddr, error)
	now    func() time.Time
	cache  sync.Map // map[string]validatedHostEntry
}

type validatedHostEntry struct {
	ip       string
	expireAt time.Time
}

func newValidatedDialContext(base func(context.Context, string, string) (net.Conn, error)) *validatedTransport {
	if base == nil {
		base = validatedBaseDialContext
	}
	return &validatedTransport{
		base:   base,
		lookup: validatedLookupIPAddr,
		now:    time.Now,
	}
}

type validatedProxyRoundTripper struct {
	proxyURL *url.URL
	base     func(context.Context, string, string) (net.Conn, error)
	lookup   func(context.Context, string) ([]net.IPAddr, error)
	now      func() time.Time
	cache    sync.Map // map[string]validatedHostEntry
}

func newValidatedProxyRoundTripper(proxyURL *url.URL) *validatedProxyRoundTripper {
	return &validatedProxyRoundTripper{
		proxyURL: proxyURL,
		base:     validatedBaseDialContext,
		lookup:   validatedLookupIPAddr,
		now:      time.Now,
	}
}

func (t *validatedProxyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if t == nil || t.proxyURL == nil {
		return nil, fmt.Errorf("validated proxy URL is nil")
	}
	if req == nil || req.URL == nil {
		return nil, fmt.Errorf("validated proxy request URL is nil")
	}

	originalHost := strings.ToLower(strings.TrimSpace(req.URL.Hostname()))
	if originalHost == "" {
		return nil, fmt.Errorf("validated proxy target host is empty")
	}
	selectedIP, err := t.resolveTargetIP(req.Context(), originalHost)
	if err != nil {
		return nil, err
	}

	targetAddr := canonicalTargetAddr(req.URL, selectedIP)
	if targetAddr == "" {
		return nil, fmt.Errorf("validated proxy target address is empty")
	}

	scheme := strings.ToLower(t.proxyURL.Scheme)
	switch scheme {
	case "http", "https":
		outboundURL := *req.URL
		outboundURL.Host = replaceURLHostWithIP(req.URL, selectedIP)
		return t.roundTripViaHTTPProxy(req, &outboundURL, targetAddr, originalHost)
	case "socks5", "socks5h":
		return t.roundTripViaSOCKSProxy(req, targetAddr, originalHost)
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s", t.proxyURL.Scheme)
	}
}

func (t *validatedProxyRoundTripper) resolveTargetIP(ctx context.Context, host string) (string, error) {
	now := time.Now()
	if t.now != nil {
		now = t.now()
	}
	if cached, ok := t.cachedIP(host, now); ok {
		return cached, nil
	}

	selected, err := resolveValidatedIP(ctx, host, t.lookup)
	if err != nil {
		return "", err
	}
	t.cache.Store(host, validatedHostEntry{ip: selected, expireAt: now.Add(validatedHostTTL)})
	return selected, nil
}

func (t *validatedProxyRoundTripper) cachedIP(host string, now time.Time) (string, bool) {
	if t == nil {
		return "", false
	}
	raw, ok := t.cache.Load(host)
	if !ok {
		return "", false
	}
	entry, ok := raw.(validatedHostEntry)
	if !ok {
		t.cache.Delete(host)
		return "", false
	}
	if !now.Before(entry.expireAt) {
		t.cache.Delete(host)
		return "", false
	}
	if entry.ip == "" {
		t.cache.Delete(host)
		return "", false
	}
	return entry.ip, true
}

func (t *validatedProxyRoundTripper) roundTripViaHTTPProxy(req *http.Request, outboundURL *url.URL, targetAddr, serverName string) (*http.Response, error) {
	conn, err := t.dialHTTPProxy(req.Context())
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
		if err := writeConnectRequest(conn, targetAddr, proxyAuthorizationHeader(t.proxyURL)); err != nil {
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
		if err := writeOriginRequest(conn, req, req.URL.Host); err != nil {
			return nil, err
		}
	} else {
		if err := writeProxyRequest(conn, req, outboundURL, req.URL.Host, proxyAuthorizationHeader(t.proxyURL)); err != nil {
			return nil, err
		}
	}

	resp, err := readProxyResponse(conn, req)
	if err != nil {
		return nil, err
	}
	closeOnError = false
	return resp, nil
}

func (t *validatedProxyRoundTripper) roundTripViaSOCKSProxy(req *http.Request, targetAddr, serverName string) (*http.Response, error) {
	dialer, err := proxy.FromURL(t.proxyURL, proxy.Direct)
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
	if err := writeOriginRequest(conn, req, req.URL.Host); err != nil {
		return nil, err
	}
	resp, err := readProxyResponse(conn, req)
	if err != nil {
		return nil, err
	}
	closeOnError = false
	return resp, nil
}

func (t *validatedProxyRoundTripper) dialHTTPProxy(ctx context.Context) (net.Conn, error) {
	addr := canonicalProxyAddr(t.proxyURL)
	if addr == "" {
		return nil, fmt.Errorf("proxy address is empty")
	}
	base := t.base
	if base == nil {
		base = validatedBaseDialContext
	}
	conn, err := base(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(t.proxyURL.Scheme, "https") {
		return conn, nil
	}
	tlsConn := tls.Client(conn, &tls.Config{ServerName: t.proxyURL.Hostname()})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return tlsConn, nil
}

func (t *validatedTransport) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if t == nil || t.base == nil {
		return nil, fmt.Errorf("validated dialer base is nil")
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return nil, fmt.Errorf("validated dialer host is empty")
	}

	now := time.Now()
	if t.now != nil {
		now = t.now()
	}

	if cached, ok := t.cachedIP(host, now); ok {
		return t.base(ctx, network, net.JoinHostPort(cached, port))
	}

	selected, err := resolveValidatedIP(ctx, host, t.lookup)
	if err != nil {
		return nil, err
	}

	t.cache.Store(host, validatedHostEntry{ip: selected, expireAt: now.Add(validatedHostTTL)})
	return t.base(ctx, network, net.JoinHostPort(selected, port))
}

func (t *validatedTransport) cachedIP(host string, now time.Time) (string, bool) {
	if t == nil {
		return "", false
	}
	raw, ok := t.cache.Load(host)
	if !ok {
		return "", false
	}
	entry, ok := raw.(validatedHostEntry)
	if !ok {
		t.cache.Delete(host)
		return "", false
	}
	if !now.Before(entry.expireAt) {
		t.cache.Delete(host)
		return "", false
	}
	if entry.ip == "" {
		t.cache.Delete(host)
		return "", false
	}
	return entry.ip, true
}

func isBlockedResolvedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	return isBlockedResolvedAddr(addr)
}

func isBlockedResolvedAddr(addr netip.Addr) bool {
	if !addr.IsValid() {
		return true
	}
	addr = addr.Unmap()
	if addr.IsUnspecified() || addr.IsLoopback() || addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() || addr.IsMulticast() {
		return true
	}
	for _, prefix := range blockedResolvedIPPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func resolveValidatedIP(ctx context.Context, host string, lookup func(context.Context, string) ([]net.IPAddr, error)) (string, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return "", fmt.Errorf("validated dialer host is empty")
	}

	if ip := net.ParseIP(host); ip != nil {
		if isBlockedResolvedIP(ip) {
			return "", &net.AddrError{Err: "blocked by DNS rebinding policy", Addr: host}
		}
		return ip.String(), nil
	}

	if lookup == nil {
		lookup = validatedLookupIPAddr
	}

	addrs, err := lookup(ctx, host)
	if err != nil {
		return "", err
	}
	if len(addrs) == 0 {
		return "", &net.AddrError{Err: "no addresses for host", Addr: host}
	}

	selected := ""
	for _, addr := range addrs {
		if isBlockedResolvedIP(addr.IP) {
			return "", &net.AddrError{Err: "blocked by DNS rebinding policy", Addr: addr.IP.String()}
		}
		if selected == "" {
			selected = addr.IP.String()
		}
	}
	if selected == "" {
		return "", &net.AddrError{Err: "no usable addresses for host", Addr: host}
	}
	return selected, nil
}

func replaceURLHostWithIP(u *url.URL, ip string) string {
	if u == nil {
		return ip
	}
	if port := u.Port(); port != "" {
		return net.JoinHostPort(ip, port)
	}
	if strings.Contains(ip, ":") {
		return "[" + ip + "]"
	}
	return ip
}

func canonicalTargetAddr(u *url.URL, ip string) string {
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

func writeConnectRequest(w io.Writer, targetAddr, proxyAuth string) error {
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

func writeProxyRequest(w io.Writer, req *http.Request, outboundURL *url.URL, host, proxyAuth string) error {
	method := req.Method
	if method == "" {
		method = http.MethodGet
	}
	if _, err := fmt.Fprintf(w, "%s %s HTTP/1.1\r\n", method, outboundURL.String()); err != nil {
		return err
	}
	return writeRequestHeadersAndBody(w, req, host, proxyAuth)
}

func writeOriginRequest(w io.Writer, req *http.Request, host string) error {
	outReq := cloneRequestForWrite(req, host)
	outReq.Header.Del("Proxy-Authorization")
	return outReq.Write(w)
}

func cloneRequestForWrite(req *http.Request, host string) *http.Request {
	outReq := req.Clone(req.Context())
	outReq.Body = req.Body
	outReq.GetBody = req.GetBody
	outReq.Host = host
	outReq.RequestURI = ""
	outReq.Close = true
	outReq.Header = req.Header.Clone()
	return outReq
}

func writeRequestHeadersAndBody(w io.Writer, req *http.Request, host, proxyAuth string) error {
	if _, err := fmt.Fprintf(w, "Host: %s\r\nConnection: close\r\n", host); err != nil {
		return err
	}
	if proxyAuth != "" {
		if _, err := fmt.Fprintf(w, "Proxy-Authorization: %s\r\n", proxyAuth); err != nil {
			return err
		}
	}

	shouldChunk := req.Body != nil && req.Body != http.NoBody && req.ContentLength <= 0
	if req.ContentLength > 0 {
		if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n", req.ContentLength); err != nil {
			return err
		}
	} else if shouldChunk {
		if _, err := io.WriteString(w, "Transfer-Encoding: chunked\r\n"); err != nil {
			return err
		}
	}

	exclude := map[string]bool{
		"Connection":          true,
		"Content-Length":      true,
		"Host":                true,
		"Proxy-Authorization": true,
		"Transfer-Encoding":   true,
	}
	if err := req.Header.WriteSubset(w, exclude); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\r\n"); err != nil {
		return err
	}

	if req.Body == nil || req.Body == http.NoBody {
		return nil
	}
	defer func() { _ = req.Body.Close() }()
	if shouldChunk {
		chunked := httputil.NewChunkedWriter(w)
		if _, err := io.Copy(chunked, req.Body); err != nil {
			_ = chunked.Close()
			return err
		}
		return chunked.Close()
	}
	_, err := io.Copy(w, req.Body)
	return err
}

func readProxyResponse(conn net.Conn, req *http.Request) (*http.Response, error) {
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return nil, err
	}
	resp.Request = req
	resp.Body = &proxyConnBody{ReadCloser: resp.Body, conn: conn}
	return resp, nil
}

type proxyConnBody struct {
	io.ReadCloser
	conn net.Conn
}

func (b *proxyConnBody) Close() error {
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
