package repository

import (
	"bufio"
	"compress/flate"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	"golang.org/x/net/http2"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyurl"
	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyutil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/servertiming"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	tlsfpHTTP2 "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/http2"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"golang.org/x/mod/semver"
)

// 默认配置常量
// 这些值在配置文件未指定时作为回退默认值使用
const (
	// directProxyKey: 无代理时的缓存键标识
	directProxyKey = "direct"
	// defaultMaxIdleConns: 默认最大空闲连接总数
	// HTTP/2 场景下，单连接可多路复用，240 足以支撑高并发
	defaultMaxIdleConns = 240
	// defaultMaxIdleConnsPerHost: 默认每主机最大空闲连接数
	defaultMaxIdleConnsPerHost = 120
	// defaultMaxConnsPerHost: 默认每主机最大连接数（含活跃连接）
	// 达到上限后新请求会等待，而非无限创建连接
	defaultMaxConnsPerHost = 240
	// defaultIdleConnTimeout: 默认空闲连接超时时间（90秒）
	// 超时后连接会被关闭，释放系统资源（建议小于上游 LB 超时）
	defaultIdleConnTimeout = 90 * time.Second
	// defaultResponseHeaderTimeout: 默认等待响应头超时时间（5分钟）
	// LLM 请求可能排队较久，需要较长超时
	defaultResponseHeaderTimeout = 300 * time.Second
	// defaultMaxUpstreamClients: 默认最大客户端缓存数量
	// 超出后会淘汰最久未使用的客户端
	defaultMaxUpstreamClients = 5000
	// defaultClientIdleTTLSeconds: 默认客户端空闲回收阈值（15分钟）
	defaultClientIdleTTLSeconds = 900
	// OpenAI HTTP/2 代理回退策略默认值
	defaultOpenAIHTTP2FallbackErrorThreshold = 2
	defaultOpenAIHTTP2FallbackWindow         = 60 * time.Second
	defaultOpenAIHTTP2FallbackTTL            = 10 * time.Minute
	// OpenAI HTTP/2 pooled connections can be silently dropped by proxies or NAT.
	// Configure active PING probes only on the standard OpenAI H2 transport.
	openAIHTTP2ReadIdleTimeout = 15 * time.Second
	openAIHTTP2PingTimeout     = 15 * time.Second

	grokCLIProxyHost       = "cli-chat-proxy.grok.com"
	grokCLIStableVersion   = "0.2.93"
	grokCLIVersionOverride = "XAI_GROK_CLI_VERSION"
	// Upstream TCP dialer defaults. Keep these explicit so upstream connection
	// behavior is controlled by this layer instead of net/http zero-value drift.
	defaultUpstreamDialTimeout       = 10 * time.Second
	defaultUpstreamDialKeepAlive     = 30 * time.Second
	defaultUpstreamDialFallbackDelay = 300 * time.Millisecond
	defaultUpstreamDNSCacheTTL       = 60 * time.Second
)

const (
	upstreamProtocolModeDefault          = "default"
	upstreamProtocolModeOpenAIH1         = "openai_h1"
	upstreamProtocolModeOpenAIH2         = "openai_h2"
	upstreamProtocolModeOpenAIH1Fallback = "openai_h1_fallback"
)

var errUpstreamClientLimitReached = errors.New("upstream client cache limit reached")

// poolSettings 连接池配置参数
// 封装 Transport 所需的各项连接池参数
type poolSettings struct {
	maxIdleConns          int           // 最大空闲连接总数
	maxIdleConnsPerHost   int           // 每主机最大空闲连接数
	maxConnsPerHost       int           // 每主机最大连接数（含活跃）
	idleConnTimeout       time.Duration // 空闲连接超时时间
	responseHeaderTimeout time.Duration // 等待响应头超时时间
}

type openAIHTTP2Settings struct {
	enabled                   bool
	allowProxyFallbackToHTTP1 bool
	fallbackErrorThreshold    int
	fallbackWindow            time.Duration
	fallbackTTL               time.Duration
}

// upstreamClientEntry 上游客户端缓存条目
// 记录客户端实例及其元数据，用于连接池管理和淘汰策略
type upstreamClientEntry struct {
	client       *http.Client // HTTP 客户端实例
	proxyKey     string       // 代理标识（用于检测代理变更）
	poolKey      string       // 连接池配置标识（用于检测配置变更）
	protocolMode string       // 协议模式（default/openai_h1/openai_h2/openai_h1_fallback）
	lastUsed     int64        // 最后使用时间戳（纳秒），用于 LRU 淘汰
	inFlight     int64        // 当前进行中的请求数，>0 时不可淘汰
}

type openAIHTTP2FallbackState struct {
	mu            sync.Mutex
	windowStart   time.Time
	errorCount    int
	fallbackUntil time.Time
}

// httpUpstreamService 通用 HTTP 上游服务
// 用于向任意 HTTP API（Claude、OpenAI 等）发送请求，支持可选代理
//
// 架构设计：
// - 根据隔离策略（proxy/account/account_proxy）缓存客户端实例
// - 每个客户端拥有独立的 Transport 连接池
// - 支持 LRU + 空闲时间双重淘汰策略
//
// 性能优化：
// 1. 根据隔离策略缓存客户端实例，避免频繁创建 http.Client
// 2. 复用 Transport 连接池，减少 TCP 握手和 TLS 协商开销
// 3. 支持账号级隔离与空闲回收，降低连接层关联风险
// 4. 达到最大连接数后等待可用连接，而非无限创建
// 5. 仅回收空闲客户端，避免中断活跃请求
// 6. HTTP/2 多路复用，连接上限不等于并发请求上限
// 7. 代理变更时清空旧连接池，避免复用错误代理
// 8. 账号并发数与连接池上限对应（账号隔离策略下）
type httpUpstreamService struct {
	cfg     *config.Config                  // 全局配置
	mu      sync.RWMutex                    // 保护 clients map 的读写锁
	clients map[string]*upstreamClientEntry // 客户端缓存池，key 由隔离策略决定

	failureSinkMu sync.RWMutex
	failureSink   service.OpsUpstreamFailureSink
	// OpenAI 走 HTTP/HTTPS 代理时的 H2->H1 回退状态（key=标准化 proxyKey）
	openAIHTTP2Fallbacks sync.Map
}

const opsUpstreamFailureResponseBodyLimit = 20 * 1024

type opsUpstreamFailureCaptureBody struct {
	body     io.ReadCloser
	mu       sync.Mutex
	captured []byte
	once     sync.Once
	report   func(string)
}

func (b *opsUpstreamFailureCaptureBody) Read(p []byte) (int, error) {
	n, err := b.body.Read(p)
	if n > 0 {
		b.mu.Lock()
		remaining := opsUpstreamFailureResponseBodyLimit - len(b.captured)
		if remaining > 0 {
			if n < remaining {
				remaining = n
			}
			b.captured = append(b.captured, p[:remaining]...)
		}
		b.mu.Unlock()
	}
	if errors.Is(err, io.EOF) {
		b.finish()
	}
	return n, err
}

func (b *opsUpstreamFailureCaptureBody) Close() error {
	b.finish()
	return b.body.Close()
}

func (b *opsUpstreamFailureCaptureBody) finish() {
	b.once.Do(func() {
		b.mu.Lock()
		body := string(append([]byte(nil), b.captured...))
		b.mu.Unlock()
		if b.report != nil {
			b.report(body)
		}
	})
}

func (s *httpUpstreamService) SetOpsUpstreamFailureSink(sink service.OpsUpstreamFailureSink) {
	if s == nil {
		return
	}
	s.failureSinkMu.Lock()
	s.failureSink = sink
	s.failureSinkMu.Unlock()
}

func (s *httpUpstreamService) reportUpstreamFailure(req *http.Request, accountID int64, resp *http.Response, err error) {
	if s == nil || (err == nil && (resp == nil || resp.StatusCode < http.StatusBadRequest)) {
		return
	}
	s.failureSinkMu.RLock()
	sink := s.failureSink
	s.failureSinkMu.RUnlock()
	if sink == nil {
		return
	}
	ctx := context.Background()
	method := ""
	rawURL := ""
	if req != nil {
		ctx = req.Context()
		method = req.Method
		if req.URL != nil {
			rawURL = req.URL.String()
		}
	}
	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
		if rawURL == "" && resp.Request != nil && resp.Request.URL != nil {
			rawURL = resp.Request.URL.String()
		}
	}
	failure := service.OpsUpstreamFailure{
		AccountID:  accountID,
		Method:     method,
		URL:        rawURL,
		Kind:       "http_attempt",
		StatusCode: statusCode,
		Err:        err,
	}
	if req != nil && service.HTTPUpstreamProfileFromContext(req.Context()) == service.HTTPUpstreamProfileOpenAI {
		failure.Platform = service.PlatformOpenAI
	}
	if err == nil && resp != nil && resp.Body != nil {
		originalBody := resp.Body
		resp.Body = &opsUpstreamFailureCaptureBody{
			body: originalBody,
			report: func(responseBody string) {
				failure.ResponseBody = responseBody
				sink.EnqueueOpsUpstreamFailure(ctx, failure)
			},
		}
		return
	}
	sink.EnqueueOpsUpstreamFailure(ctx, failure)
}

// NewHTTPUpstream 创建通用 HTTP 上游服务
// 使用配置中的连接池参数构建 Transport
//
// 参数:
//   - cfg: 全局配置，包含连接池参数和隔离策略
//
// 返回:
//   - service.HTTPUpstream 接口实现
func NewHTTPUpstream(cfg *config.Config) service.HTTPUpstream {
	return &httpUpstreamService{
		cfg:     cfg,
		clients: make(map[string]*upstreamClientEntry),
	}
}

// Do 执行 HTTP 请求
// 根据隔离策略获取或创建客户端，并跟踪请求生命周期
//
// 参数:
//   - req: HTTP 请求对象
//   - proxyURL: 代理地址，空字符串表示直连
//   - accountID: 账户 ID，用于账户级隔离
//   - accountConcurrency: 账户并发限制，用于动态调整连接池大小
//
// 返回:
//   - *http.Response: HTTP 响应（Body 已包装，关闭时自动更新计数）
//   - error: 请求错误
//
// 注意:
//   - 调用方必须关闭 resp.Body，否则会导致 inFlight 计数泄漏
//   - inFlight > 0 的客户端不会被淘汰，确保活跃请求不被中断
func (s *httpUpstreamService) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	applyGrokCLIProxyHeaders(req)
	if err := s.validateRequestHost(req); err != nil {
		return nil, err
	}
	profile := service.HTTPUpstreamProfileDefault
	if req != nil {
		profile = service.HTTPUpstreamProfileFromContext(req.Context())
	}
	if resp, err, handled := s.doWithRequestOverrides(req, proxyURL, accountID, accountConcurrency, nil, profile); handled {
		s.reportUpstreamFailure(req, accountID, resp, err)
		return resp, err
	}

	// 获取或创建对应的客户端，并标记请求占用
	entry, err := s.acquireClientWithProfile(proxyURL, accountID, accountConcurrency, profile)
	if err != nil {
		return nil, err
	}

	// 执行请求
	resp, err := servertiming.Do(entry.client, req)
	if err != nil {
		s.reportUpstreamFailure(req, accountID, resp, err)
		s.recordOpenAIHTTP2Failure(profile, entry.protocolMode, entry.proxyKey, err)
		// 请求失败，立即减少计数
		atomic.AddInt64(&entry.inFlight, -1)
		atomic.StoreInt64(&entry.lastUsed, time.Now().UnixNano())
		return nil, err
	}
	s.recordOpenAIHTTP2Success(profile, entry.protocolMode, entry.proxyKey)

	// 如果上游返回了压缩内容，解压后再交给业务层
	decompressResponseBody(resp)
	s.reportUpstreamFailure(req, accountID, resp, nil)

	// 包装响应体，在关闭时自动减少计数并更新时间戳
	// 这确保了流式响应（如 SSE）在完全读取前不会被淘汰
	resp.Body = wrapTrackedBody(resp.Body, func() {
		atomic.AddInt64(&entry.inFlight, -1)
		atomic.StoreInt64(&entry.lastUsed, time.Now().UnixNano())
	})

	return resp, nil
}

// DoWithTLS 执行带 TLS 指纹伪装的 HTTP 请求
//
// profile 为 nil 时不启用 TLS 指纹，行为与 Do 方法相同。
// profile 非 nil 时使用指定的 Profile 进行 TLS 指纹伪装。
func (s *httpUpstreamService) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	// 不再对 chatgpt.com 自动套用硬编码的过时 Chrome 指纹兜底：经典 ChromeProfile 缺 PQ
	// keyshare/ALPS，与真实 Chrome 不符，主动套用反而是可识别的伪造信号。无显式 captured
	// profile 时走原生 Do(不伪装 TLS)，交由账号绑定的 profile 显式驱动指纹。
	if profile == nil {
		return s.Do(req, proxyURL, accountID, accountConcurrency)
	}
	applyGrokCLIProxyHeaders(req)
	upstreamProfile := service.HTTPUpstreamProfileDefault
	if req != nil {
		upstreamProfile = service.HTTPUpstreamProfileFromContext(req.Context())
	}

	targetHost := ""
	if req != nil && req.URL != nil {
		targetHost = req.URL.Host
	}
	proxyInfo := "direct"
	if proxyURL != "" {
		proxyInfo = proxyURL
	}
	slog.Debug("tls_fingerprint_enabled", "account_id", accountID, "target", targetHost, "proxy", proxyInfo, "profile", profile.Name)

	if err := s.validateRequestHost(req); err != nil {
		return nil, err
	}
	if resp, err, handled := s.doWithRequestOverrides(req, proxyURL, accountID, accountConcurrency, profile, upstreamProfile); handled {
		s.reportUpstreamFailure(req, accountID, resp, err)
		return resp, err
	}

	entry, err := s.acquireClientWithTLS(proxyURL, accountID, accountConcurrency, profile, upstreamProfile)
	if err != nil {
		slog.Debug("tls_fingerprint_acquire_client_failed", "account_id", accountID, "error", err)
		return nil, err
	}

	resp, err := servertiming.Do(entry.client, req)
	if err != nil {
		s.reportUpstreamFailure(req, accountID, resp, err)
		atomic.AddInt64(&entry.inFlight, -1)
		atomic.StoreInt64(&entry.lastUsed, time.Now().UnixNano())
		slog.Debug("tls_fingerprint_request_failed", "account_id", accountID, "error", err)
		return nil, err
	}

	decompressResponseBody(resp)
	s.reportUpstreamFailure(req, accountID, resp, nil)

	resp.Body = wrapTrackedBody(resp.Body, func() {
		atomic.AddInt64(&entry.inFlight, -1)
		atomic.StoreInt64(&entry.lastUsed, time.Now().UnixNano())
	})

	return resp, nil
}

func applyGrokCLIProxyHeaders(req *http.Request) {
	if req == nil || req.URL == nil || !strings.EqualFold(strings.TrimSpace(req.URL.Hostname()), grokCLIProxyHost) {
		return
	}
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	version := strings.TrimSpace(os.Getenv(grokCLIVersionOverride))
	if !isSupportedGrokCLIVersion(version) {
		version = grokCLIStableVersion
	}
	req.Header.Set("X-XAI-Token-Auth", "xai-grok-cli")
	req.Header.Set("x-grok-client-version", version)
	req.Header.Set("User-Agent", "xai-grok-workspace/"+version)
}

func isSupportedGrokCLIVersion(version string) bool {
	canonical := "v" + version
	minimum := "v" + grokCLIStableVersion
	return semver.IsValid(canonical) &&
		semver.Canonical(canonical) == canonical &&
		semver.Compare(canonical, minimum) >= 0
}

func (s *httpUpstreamService) doWithRequestOverrides(
	req *http.Request,
	proxyURL string,
	accountID int64,
	accountConcurrency int,
	profile *tlsfingerprint.Profile,
	upstreamProfile service.HTTPUpstreamProfile,
) (*http.Response, error, bool) {
	if req == nil {
		return nil, nil, false
	}
	opts := service.HTTPUpstreamRequestOptionsFromContext(req.Context())
	if !opts.RequiresDedicatedClient() {
		return nil, nil, false
	}

	proxyKey, parsedProxy, err := normalizeProxyURL(proxyURL)
	if err != nil {
		return nil, err, true
	}
	settings := s.resolvePoolSettings(s.getIsolationMode(), accountConcurrency)
	settings = s.applyProfilePoolSettings(settings, upstreamProfile)
	protocolMode := upstreamProtocolModeDefault
	var roundTripper http.RoundTripper
	// h1Transport 指向可设置 DisableKeepAlives 等 HTTP/1.x 专属字段的 *http.Transport；
	// 当 TLS 指纹经路走 HTTP/2(*http2.Transport) 时为 nil。
	var h1Transport *http.Transport
	if len(opts.RawHTTP1HeaderOrder) > 0 {
		roundTripper = &http1HeaderReplayRoundTripper{
			headerOrder:        append([]string(nil), opts.RawHTTP1HeaderOrder...),
			proxyURL:           parsedProxy,
			profile:            profile,
			validateResolvedIP: s.shouldValidateResolvedIP(),
		}
		req = req.Clone(req.Context())
		req.Close = true
	} else if profile != nil {
		roundTripper, h1Transport, err = buildUpstreamRoundTripperWithTLSFingerprint(settings, parsedProxy, profile, s.shouldValidateResolvedIP())
	} else {
		protocolMode = s.resolveProtocolMode(upstreamProfile, proxyKey, parsedProxy)
		h1Transport, err = buildUpstreamTransport(settings, parsedProxy, protocolMode, s.shouldValidateResolvedIP())
		roundTripper = h1Transport
	}
	if err != nil {
		return nil, err, true
	}
	if opts.DisableKeepAlives {
		if h1Transport != nil {
			h1Transport.DisableKeepAlives = true
		}
		req = req.Clone(req.Context())
		req.Close = true
	}

	slog.Debug("http_upstream_request_overrides_enabled",
		"account_id", accountID,
		"fresh_client", opts.FreshClient,
		"disable_keep_alives", opts.DisableKeepAlives,
		"tls_profile_enabled", profile != nil)

	client := &http.Client{Transport: roundTripper}
	if s.shouldValidateResolvedIP() {
		client.CheckRedirect = s.redirectChecker
	}

	resp, err := servertiming.Do(client, req)
	if err != nil {
		if profile == nil {
			s.recordOpenAIHTTP2Failure(upstreamProfile, protocolMode, proxyKey, err)
		}
		client.CloseIdleConnections()
		return nil, err, true
	}
	if profile == nil {
		s.recordOpenAIHTTP2Success(upstreamProfile, protocolMode, proxyKey)
	}

	decompressResponseBody(resp)
	resp.Body = wrapTrackedBody(resp.Body, func() {
		client.CloseIdleConnections()
	})
	return resp, nil, true
}

// acquireClientWithTLS 获取或创建带 TLS 指纹的客户端
func (s *httpUpstreamService) acquireClientWithTLS(proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile, upstreamProfile service.HTTPUpstreamProfile) (*upstreamClientEntry, error) {
	return s.getClientEntryWithTLS(proxyURL, accountID, accountConcurrency, profile, upstreamProfile, true, true)
}

// getClientEntryWithTLS 获取或创建带 TLS 指纹的客户端条目
// TLS 指纹客户端使用独立的缓存键，与普通客户端隔离
func (s *httpUpstreamService) getClientEntryWithTLS(proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile, upstreamProfile service.HTTPUpstreamProfile, markInFlight bool, enforceLimit bool) (*upstreamClientEntry, error) {
	isolation := s.getIsolationMode()
	proxyKey, parsedProxy, err := normalizeProxyURL(proxyURL)
	if err != nil {
		return nil, err
	}
	settings := s.resolvePoolSettings(isolation, accountConcurrency)
	settings = s.applyProfilePoolSettings(settings, upstreamProfile)
	profileKey := tlsFingerprintProfileCacheKey(profile)
	// TLS 指纹经路即使在 proxy 隔离模式下，也必须保持账号级连接池隔离：
	// 同一代理+同一 JA3 若被不同账号共用同一底层长连接/HTTP keep-alive，会把多账号请求
	// 关联到同一 TLS 会话足迹，削弱反封禁的“每账号独立身份”目标。普通 HTTP 经路仍遵从用户
	// 配置的 isolation；仅 TLS 伪装经路强制把 accountID 编入缓存键，避免跨账号复用。
	cacheKey := "tls:" + profileKey + ":" + buildTLSFingerprintCacheKey(proxyKey, accountID, upstreamProtocolModeDefault)
	poolKey := buildPoolKey(settings, upstreamProtocolModeDefault) + ":tls:" + profileKey

	now := time.Now()
	nowUnix := now.UnixNano()

	// 读锁快速路径
	s.mu.RLock()
	if entry, ok := s.clients[cacheKey]; ok && s.shouldReuseEntry(entry, isolation, proxyKey, poolKey) {
		atomic.StoreInt64(&entry.lastUsed, nowUnix)
		if markInFlight {
			atomic.AddInt64(&entry.inFlight, 1)
		}
		s.mu.RUnlock()
		slog.Debug("tls_fingerprint_reusing_client", "account_id", accountID, "cache_key", cacheKey)
		return entry, nil
	}
	s.mu.RUnlock()

	// 写锁慢路径
	s.mu.Lock()
	if entry, ok := s.clients[cacheKey]; ok {
		if s.shouldReuseEntry(entry, isolation, proxyKey, poolKey) {
			atomic.StoreInt64(&entry.lastUsed, nowUnix)
			if markInFlight {
				atomic.AddInt64(&entry.inFlight, 1)
			}
			s.mu.Unlock()
			slog.Debug("tls_fingerprint_reusing_client", "account_id", accountID, "cache_key", cacheKey)
			return entry, nil
		}
		slog.Debug("tls_fingerprint_evicting_stale_client",
			"account_id", accountID,
			"cache_key", cacheKey,
			"proxy_changed", entry.proxyKey != proxyKey,
			"pool_changed", entry.poolKey != poolKey)
		s.removeClientLocked(cacheKey, entry)
	}

	// 超出缓存上限时尝试淘汰
	if enforceLimit && s.maxUpstreamClients() > 0 {
		s.evictIdleLocked(now)
		if len(s.clients) >= s.maxUpstreamClients() {
			if !s.evictOldestIdleLocked() {
				s.mu.Unlock()
				return nil, errUpstreamClientLimitReached
			}
		}
	}

	// 创建带 TLS 指纹的 Transport（profile ALPN 含 h2 时自动走 HTTP/2 出站）
	slog.Debug("tls_fingerprint_creating_new_client", "account_id", accountID, "cache_key", cacheKey, "proxy", proxyKey)
	roundTripper, _, err := buildUpstreamRoundTripperWithTLSFingerprint(settings, parsedProxy, profile, s.shouldValidateResolvedIP())
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("build TLS fingerprint transport: %w", err)
	}

	client := &http.Client{Transport: roundTripper}
	if s.shouldValidateResolvedIP() {
		client.CheckRedirect = s.redirectChecker
	}

	entry := &upstreamClientEntry{
		client:   client,
		proxyKey: proxyKey,
		poolKey:  poolKey,
	}
	atomic.StoreInt64(&entry.lastUsed, nowUnix)
	if markInFlight {
		atomic.StoreInt64(&entry.inFlight, 1)
	}
	s.clients[cacheKey] = entry

	s.evictIdleLocked(now)
	s.evictOverLimitLocked()
	s.mu.Unlock()
	return entry, nil
}

func tlsFingerprintProfileCacheKey(profile *tlsfingerprint.Profile) string {
	if profile == nil {
		return "default"
	}
	payload, err := json.Marshal(profile)
	if err != nil {
		return "profile:" + strings.TrimSpace(profile.Name)
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:8])
}

func buildTLSFingerprintCacheKey(proxyKey string, accountID int64, protocolMode string) string {
	return buildCacheKey(config.ConnectionPoolIsolationAccountProxy, proxyKey, accountID, protocolMode)
}

func (s *httpUpstreamService) shouldValidateResolvedIP() bool {
	if s.cfg == nil {
		return false
	}
	return !s.cfg.Security.URLAllowlist.AllowPrivateHosts
}

func (s *httpUpstreamService) validateRequestHost(req *http.Request) error {
	if !s.shouldValidateResolvedIP() {
		return nil
	}
	if req == nil || req.URL == nil {
		return errors.New("request url is required")
	}
	host := strings.TrimSpace(req.URL.Hostname())
	if host == "" {
		return errors.New("request host is empty")
	}
	if err := urlvalidator.ValidateResolvedIP(host); err != nil {
		return err
	}
	return nil
}

func (s *httpUpstreamService) redirectChecker(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	return s.validateRequestHost(req)
}

// acquireClient 获取或创建客户端，并标记为进行中请求
// 用于请求路径，避免在获取后被淘汰
func (s *httpUpstreamService) acquireClient(proxyURL string, accountID int64, accountConcurrency int) (*upstreamClientEntry, error) {
	return s.acquireClientWithProfile(proxyURL, accountID, accountConcurrency, service.HTTPUpstreamProfileDefault)
}

// acquireClientWithProfile 获取或创建客户端，并按请求 profile 选择协议策略。
func (s *httpUpstreamService) acquireClientWithProfile(proxyURL string, accountID int64, accountConcurrency int, profile service.HTTPUpstreamProfile) (*upstreamClientEntry, error) {
	return s.getClientEntry(proxyURL, accountID, accountConcurrency, profile, true, true)
}

// getOrCreateClient 获取或创建客户端
// 根据隔离策略和参数决定缓存键，处理代理变更和配置变更
//
// 参数:
//   - proxyURL: 代理地址
//   - accountID: 账户 ID
//   - accountConcurrency: 账户并发限制
//
// 返回:
//   - *upstreamClientEntry: 客户端缓存条目
//
// 隔离策略说明:
//   - proxy: 按代理地址隔离，同一代理共享客户端
//   - account: 按账户隔离，同一账户共享客户端（代理变更时重建）
//   - account_proxy: 按账户+代理组合隔离，最细粒度
func (s *httpUpstreamService) getOrCreateClient(proxyURL string, accountID int64, accountConcurrency int) (*upstreamClientEntry, error) {
	return s.getClientEntry(proxyURL, accountID, accountConcurrency, service.HTTPUpstreamProfileDefault, false, false)
}

// getClientEntry 获取或创建客户端条目
// markInFlight=true 时会标记进行中请求，用于请求路径防止被淘汰
// enforceLimit=true 时会限制客户端数量，超限且无法淘汰时返回错误
func (s *httpUpstreamService) getClientEntry(proxyURL string, accountID int64, accountConcurrency int, profile service.HTTPUpstreamProfile, markInFlight bool, enforceLimit bool) (*upstreamClientEntry, error) {
	// 获取隔离模式
	isolation := s.getIsolationMode()
	// 标准化代理 URL 并解析
	proxyKey, parsedProxy, err := normalizeProxyURL(proxyURL)
	if err != nil {
		return nil, err
	}
	// 根据请求 profile（例如 OpenAI）选择协议模式
	protocolMode := s.resolveProtocolMode(profile, proxyKey, parsedProxy)
	settings := s.resolvePoolSettings(isolation, accountConcurrency)
	settings = s.applyProfilePoolSettings(settings, profile)
	// 构建缓存键（根据隔离策略不同）
	cacheKey := buildCacheKey(isolation, proxyKey, accountID, protocolMode)
	// 构建连接池配置键（用于检测配置变更）
	poolKey := buildPoolKey(settings, protocolMode)

	now := time.Now()
	nowUnix := now.UnixNano()

	// 读锁快速路径：命中缓存直接返回，减少锁竞争
	s.mu.RLock()
	if entry, ok := s.clients[cacheKey]; ok && s.shouldReuseEntry(entry, isolation, proxyKey, poolKey) {
		atomic.StoreInt64(&entry.lastUsed, nowUnix)
		if markInFlight {
			atomic.AddInt64(&entry.inFlight, 1)
		}
		s.mu.RUnlock()
		return entry, nil
	}
	s.mu.RUnlock()

	// 写锁慢路径：创建或重建客户端
	s.mu.Lock()
	if entry, ok := s.clients[cacheKey]; ok {
		if s.shouldReuseEntry(entry, isolation, proxyKey, poolKey) {
			atomic.StoreInt64(&entry.lastUsed, nowUnix)
			if markInFlight {
				atomic.AddInt64(&entry.inFlight, 1)
			}
			s.mu.Unlock()
			return entry, nil
		}
		s.removeClientLocked(cacheKey, entry)
	}

	// 超出缓存上限时尝试淘汰，无法淘汰则拒绝新建
	if enforceLimit && s.maxUpstreamClients() > 0 {
		s.evictIdleLocked(now)
		if len(s.clients) >= s.maxUpstreamClients() {
			if !s.evictOldestIdleLocked() {
				s.mu.Unlock()
				return nil, errUpstreamClientLimitReached
			}
		}
	}

	// 缓存未命中或需要重建，创建新客户端
	transport, err := buildUpstreamTransport(settings, parsedProxy, protocolMode, s.shouldValidateResolvedIP())
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("build transport: %w", err)
	}
	client := &http.Client{Transport: transport}
	if s.shouldValidateResolvedIP() {
		client.CheckRedirect = s.redirectChecker
	}
	entry := &upstreamClientEntry{
		client:       client,
		proxyKey:     proxyKey,
		poolKey:      poolKey,
		protocolMode: protocolMode,
	}
	atomic.StoreInt64(&entry.lastUsed, nowUnix)
	if markInFlight {
		atomic.StoreInt64(&entry.inFlight, 1)
	}
	s.clients[cacheKey] = entry

	// 执行淘汰策略：先淘汰空闲超时的，再淘汰超出数量限制的
	s.evictIdleLocked(now)
	s.evictOverLimitLocked()
	s.mu.Unlock()
	return entry, nil
}

// shouldReuseEntry 判断缓存条目是否可复用
// 若代理或连接池配置发生变化，则需要重建客户端
func (s *httpUpstreamService) shouldReuseEntry(entry *upstreamClientEntry, isolation, proxyKey, poolKey string) bool {
	if entry == nil {
		return false
	}
	if isolation == config.ConnectionPoolIsolationAccount && entry.proxyKey != proxyKey {
		return false
	}
	if entry.poolKey != poolKey {
		return false
	}
	return true
}

// removeClientLocked 移除客户端（需持有锁）
// 从缓存中删除并关闭空闲连接
//
// 参数:
//   - key: 缓存键
//   - entry: 客户端条目
func (s *httpUpstreamService) removeClientLocked(key string, entry *upstreamClientEntry) {
	delete(s.clients, key)
	if entry != nil && entry.client != nil {
		// 关闭空闲连接，释放系统资源
		// 注意：这不会中断活跃连接
		entry.client.CloseIdleConnections()
	}
}

// evictIdleLocked 淘汰空闲超时的客户端（需持有锁）
// 遍历所有客户端，移除超过 TTL 且无活跃请求的条目
//
// 参数:
//   - now: 当前时间
func (s *httpUpstreamService) evictIdleLocked(now time.Time) {
	ttl := s.clientIdleTTL()
	if ttl <= 0 {
		return
	}
	// 计算淘汰截止时间
	cutoff := now.Add(-ttl).UnixNano()
	for key, entry := range s.clients {
		// 跳过有活跃请求的客户端
		if atomic.LoadInt64(&entry.inFlight) != 0 {
			continue
		}
		// 淘汰超时的空闲客户端
		if atomic.LoadInt64(&entry.lastUsed) <= cutoff {
			s.removeClientLocked(key, entry)
		}
	}
}

// evictOldestIdleLocked 淘汰最久未使用且无活跃请求的客户端（需持有锁）
func (s *httpUpstreamService) evictOldestIdleLocked() bool {
	var (
		oldestKey   string
		oldestEntry *upstreamClientEntry
		oldestTime  int64
	)
	// 查找最久未使用且无活跃请求的客户端
	for key, entry := range s.clients {
		// 跳过有活跃请求的客户端
		if atomic.LoadInt64(&entry.inFlight) != 0 {
			continue
		}
		lastUsed := atomic.LoadInt64(&entry.lastUsed)
		if oldestEntry == nil || lastUsed < oldestTime {
			oldestKey = key
			oldestEntry = entry
			oldestTime = lastUsed
		}
	}
	// 所有客户端都有活跃请求，无法淘汰
	if oldestEntry == nil {
		return false
	}
	s.removeClientLocked(oldestKey, oldestEntry)
	return true
}

// evictOverLimitLocked 淘汰超出数量限制的客户端（需持有锁）
// 使用 LRU 策略，优先淘汰最久未使用且无活跃请求的客户端
func (s *httpUpstreamService) evictOverLimitLocked() bool {
	maxClients := s.maxUpstreamClients()
	if maxClients <= 0 {
		return false
	}
	evicted := false
	// 循环淘汰直到满足数量限制
	for len(s.clients) > maxClients {
		if !s.evictOldestIdleLocked() {
			return evicted
		}
		evicted = true
	}
	return evicted
}

// getIsolationMode 获取连接池隔离模式
// 从配置中读取，无效值回退到 account_proxy 模式
//
// 返回:
//   - string: 隔离模式（proxy/account/account_proxy）
func (s *httpUpstreamService) getIsolationMode() string {
	if s.cfg == nil {
		return config.ConnectionPoolIsolationAccountProxy
	}
	mode := strings.ToLower(strings.TrimSpace(s.cfg.Gateway.ConnectionPoolIsolation))
	if mode == "" {
		return config.ConnectionPoolIsolationAccountProxy
	}
	switch mode {
	case config.ConnectionPoolIsolationProxy, config.ConnectionPoolIsolationAccount, config.ConnectionPoolIsolationAccountProxy:
		return mode
	default:
		return config.ConnectionPoolIsolationAccountProxy
	}
}

// maxUpstreamClients 获取最大客户端缓存数量
// 从配置中读取，无效值使用默认值
func (s *httpUpstreamService) maxUpstreamClients() int {
	if s.cfg == nil {
		return defaultMaxUpstreamClients
	}
	if s.cfg.Gateway.MaxUpstreamClients > 0 {
		return s.cfg.Gateway.MaxUpstreamClients
	}
	return defaultMaxUpstreamClients
}

// clientIdleTTL 获取客户端空闲回收阈值
// 从配置中读取，无效值使用默认值
func (s *httpUpstreamService) clientIdleTTL() time.Duration {
	if s.cfg == nil {
		return time.Duration(defaultClientIdleTTLSeconds) * time.Second
	}
	if s.cfg.Gateway.ClientIdleTTLSeconds > 0 {
		return time.Duration(s.cfg.Gateway.ClientIdleTTLSeconds) * time.Second
	}
	return time.Duration(defaultClientIdleTTLSeconds) * time.Second
}

// resolvePoolSettings 解析连接池配置
// 根据隔离策略和账户并发数动态调整连接池参数
//
// 参数:
//   - isolation: 隔离模式
//   - accountConcurrency: 账户并发限制
//
// 返回:
//   - poolSettings: 连接池配置
//
// 说明:
//   - 账户隔离模式下，连接池大小与账户并发数对应
//   - 这确保了单账户不会占用过多连接资源
func (s *httpUpstreamService) resolvePoolSettings(isolation string, accountConcurrency int) poolSettings {
	settings := defaultPoolSettings(s.cfg)
	// 账户隔离模式下，根据账户并发数调整连接池大小
	if (isolation == config.ConnectionPoolIsolationAccount || isolation == config.ConnectionPoolIsolationAccountProxy) && accountConcurrency > 0 {
		settings.maxIdleConns = accountConcurrency
		settings.maxIdleConnsPerHost = accountConcurrency
		settings.maxConnsPerHost = accountConcurrency
	}
	return settings
}

func (s *httpUpstreamService) applyProfilePoolSettings(settings poolSettings, profile service.HTTPUpstreamProfile) poolSettings {
	if profile != service.HTTPUpstreamProfileOpenAI {
		return settings
	}
	settings.responseHeaderTimeout = 0
	if s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIResponseHeaderTimeout > 0 {
		settings.responseHeaderTimeout = time.Duration(s.cfg.Gateway.OpenAIResponseHeaderTimeout) * time.Second
	}
	return settings
}

// buildPoolKey 构建连接池配置键，用于检测连接池配置变更。
func buildPoolKey(settings poolSettings, protocolMode string) string {
	base := fmt.Sprintf(
		"idle:%d|idle_host:%d|max:%d|idle_timeout:%s|header_timeout:%s",
		settings.maxIdleConns,
		settings.maxIdleConnsPerHost,
		settings.maxConnsPerHost,
		settings.idleConnTimeout,
		settings.responseHeaderTimeout,
	)
	if protocolMode == "" || protocolMode == upstreamProtocolModeDefault {
		return base
	}
	return base + "|proto:" + protocolMode
}

// buildCacheKey 构建客户端缓存键
// 根据隔离策略决定缓存键的组成
//
// 参数:
//   - isolation: 隔离模式
//   - proxyKey: 代理标识
//   - accountID: 账户 ID
//
// 返回:
//   - string: 缓存键
//
// 缓存键格式:
//   - proxy 模式: "proxy:{proxyKey}"
//   - account 模式: "account:{accountID}"
//   - account_proxy 模式: "account:{accountID}|proxy:{proxyKey}"
func buildCacheKey(isolation, proxyKey string, accountID int64, protocolMode string) string {
	var base string
	switch isolation {
	case config.ConnectionPoolIsolationAccount:
		base = fmt.Sprintf("account:%d", accountID)
	case config.ConnectionPoolIsolationAccountProxy:
		base = fmt.Sprintf("account:%d|proxy:%s", accountID, proxyKey)
	default:
		base = fmt.Sprintf("proxy:%s", proxyKey)
	}
	if protocolMode != "" && protocolMode != upstreamProtocolModeDefault {
		base += "|proto:" + protocolMode
	}
	return base
}

func (s *httpUpstreamService) resolveOpenAIHTTP2Settings() openAIHTTP2Settings {
	settings := openAIHTTP2Settings{
		enabled:                   false,
		allowProxyFallbackToHTTP1: true,
		fallbackErrorThreshold:    defaultOpenAIHTTP2FallbackErrorThreshold,
		fallbackWindow:            defaultOpenAIHTTP2FallbackWindow,
		fallbackTTL:               defaultOpenAIHTTP2FallbackTTL,
	}
	if s == nil || s.cfg == nil {
		return settings
	}
	cfg := s.cfg.Gateway.OpenAIHTTP2
	settings.enabled = cfg.Enabled
	settings.allowProxyFallbackToHTTP1 = cfg.AllowProxyFallbackToHTTP1
	if cfg.FallbackErrorThreshold > 0 {
		settings.fallbackErrorThreshold = cfg.FallbackErrorThreshold
	}
	if cfg.FallbackWindowSeconds > 0 {
		settings.fallbackWindow = time.Duration(cfg.FallbackWindowSeconds) * time.Second
	}
	if cfg.FallbackTTLSeconds > 0 {
		settings.fallbackTTL = time.Duration(cfg.FallbackTTLSeconds) * time.Second
	}
	return settings
}

func (s *httpUpstreamService) resolveProtocolMode(profile service.HTTPUpstreamProfile, proxyKey string, parsedProxy *url.URL) string {
	if profile != service.HTTPUpstreamProfileOpenAI {
		return upstreamProtocolModeDefault
	}
	settings := s.resolveOpenAIHTTP2Settings()
	if !settings.enabled {
		return upstreamProtocolModeOpenAIH1
	}
	if parsedProxy == nil {
		return upstreamProtocolModeOpenAIH2
	}
	scheme := strings.ToLower(parsedProxy.Scheme)
	if scheme != "http" && scheme != "https" {
		return upstreamProtocolModeOpenAIH2
	}
	if settings.allowProxyFallbackToHTTP1 && s.isOpenAIHTTP2FallbackActive(proxyKey) {
		return upstreamProtocolModeOpenAIH1Fallback
	}
	return upstreamProtocolModeOpenAIH2
}

func (s *httpUpstreamService) isOpenAIHTTP2FallbackActive(proxyKey string) bool {
	raw, ok := s.openAIHTTP2Fallbacks.Load(proxyKey)
	if !ok {
		return false
	}
	state, ok := raw.(*openAIHTTP2FallbackState)
	if !ok || state == nil {
		return false
	}
	return state.isFallbackActive(time.Now())
}

func (s *httpUpstreamService) getOrCreateOpenAIHTTP2FallbackState(proxyKey string) *openAIHTTP2FallbackState {
	state := &openAIHTTP2FallbackState{}
	actual, _ := s.openAIHTTP2Fallbacks.LoadOrStore(proxyKey, state)
	cached, ok := actual.(*openAIHTTP2FallbackState)
	if !ok || cached == nil {
		return state
	}
	return cached
}

func isHTTPProxyKey(proxyKey string) bool {
	return strings.HasPrefix(proxyKey, "http://") || strings.HasPrefix(proxyKey, "https://")
}

func isOpenAIHTTP2CompatibilityError(err error) bool {
	if err == nil {
		return false
	}
	if isUpstreamTimeoutError(err) {
		return false
	}
	msg := strings.ToLower(err.Error())
	if msg == "" {
		return false
	}
	markers := []string{
		"alpn",
		"no application protocol",
		"protocol error",
		"stream error",
		"goaway",
		"refused_stream",
		"frame too large",
	}
	for _, marker := range markers {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

func isUpstreamTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	if msg == "" {
		return false
	}
	timeoutMarkers := []string{
		"timeout awaiting response headers",
		"i/o timeout",
		"context deadline exceeded",
		"client.timeout exceeded while awaiting headers",
		"tls handshake timeout",
	}
	for _, marker := range timeoutMarkers {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

func (s *httpUpstreamService) recordOpenAIHTTP2Failure(profile service.HTTPUpstreamProfile, protocolMode, proxyKey string, err error) {
	if profile != service.HTTPUpstreamProfileOpenAI || protocolMode != upstreamProtocolModeOpenAIH2 {
		return
	}
	settings := s.resolveOpenAIHTTP2Settings()
	if !settings.enabled || !settings.allowProxyFallbackToHTTP1 {
		return
	}
	if !isHTTPProxyKey(proxyKey) || !isOpenAIHTTP2CompatibilityError(err) {
		return
	}
	state := s.getOrCreateOpenAIHTTP2FallbackState(proxyKey)
	activated, until := state.recordFailure(time.Now(), settings.fallbackErrorThreshold, settings.fallbackWindow, settings.fallbackTTL)
	if activated {
		slog.Warn("openai_http2_proxy_fallback_activated",
			"proxy", proxyKey,
			"fallback_until", until.Format(time.RFC3339))
	}
}

func (s *httpUpstreamService) recordOpenAIHTTP2Success(profile service.HTTPUpstreamProfile, protocolMode, proxyKey string) {
	if profile != service.HTTPUpstreamProfileOpenAI || protocolMode != upstreamProtocolModeOpenAIH2 {
		return
	}
	if !isHTTPProxyKey(proxyKey) {
		return
	}
	raw, ok := s.openAIHTTP2Fallbacks.Load(proxyKey)
	if !ok {
		return
	}
	state, ok := raw.(*openAIHTTP2FallbackState)
	if !ok || state == nil {
		return
	}
	state.resetErrorWindow()
}

func (s *openAIHTTP2FallbackState) isFallbackActive(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fallbackUntil.IsZero() {
		return false
	}
	if now.Before(s.fallbackUntil) {
		return true
	}
	s.fallbackUntil = time.Time{}
	return false
}

func (s *openAIHTTP2FallbackState) resetErrorWindow() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.windowStart = time.Time{}
	s.errorCount = 0
}

func (s *openAIHTTP2FallbackState) recordFailure(now time.Time, threshold int, window, ttl time.Duration) (bool, time.Time) {
	if threshold <= 0 {
		threshold = defaultOpenAIHTTP2FallbackErrorThreshold
	}
	if window <= 0 {
		window = defaultOpenAIHTTP2FallbackWindow
	}
	if ttl <= 0 {
		ttl = defaultOpenAIHTTP2FallbackTTL
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.fallbackUntil.IsZero() && now.Before(s.fallbackUntil) {
		return false, s.fallbackUntil
	}
	if !s.fallbackUntil.IsZero() && !now.Before(s.fallbackUntil) {
		s.fallbackUntil = time.Time{}
	}

	if s.windowStart.IsZero() || now.Sub(s.windowStart) > window {
		s.windowStart = now
		s.errorCount = 0
	}
	s.errorCount++
	if s.errorCount < threshold {
		return false, time.Time{}
	}

	s.fallbackUntil = now.Add(ttl)
	s.windowStart = time.Time{}
	s.errorCount = 0
	return true, s.fallbackUntil
}

// normalizeProxyURL 标准化代理 URL
// 处理空值和解析错误，返回标准化的键和解析后的 URL
//
// 参数:
//   - raw: 原始代理 URL 字符串
//
// 返回:
//   - string: 标准化的代理键（空返回 "direct"）
//   - *url.URL: 解析后的 URL（空返回 nil）
//   - error: 非空代理 URL 解析失败时返回错误（禁止回退到直连）
func normalizeProxyURL(raw string) (string, *url.URL, error) {
	_, parsed, err := proxyurl.Parse(raw)
	if err != nil {
		return "", nil, err
	}
	if parsed == nil {
		return directProxyKey, nil, nil
	}
	// 规范化：小写 scheme/host，去除路径和查询参数
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = ""
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.ForceQuery = false
	if hostname := parsed.Hostname(); hostname != "" {
		port := parsed.Port()
		if (parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443") {
			port = ""
		}
		hostname = strings.ToLower(hostname)
		if port != "" {
			parsed.Host = net.JoinHostPort(hostname, port)
		} else {
			parsed.Host = hostname
		}
	}
	return parsed.String(), parsed, nil
}

// defaultPoolSettings 获取默认连接池配置
// 从全局配置中读取，无效值使用常量默认值
//
// 参数:
//   - cfg: 全局配置
//
// 返回:
//   - poolSettings: 连接池配置
func defaultPoolSettings(cfg *config.Config) poolSettings {
	maxIdleConns := defaultMaxIdleConns
	maxIdleConnsPerHost := defaultMaxIdleConnsPerHost
	maxConnsPerHost := defaultMaxConnsPerHost
	idleConnTimeout := defaultIdleConnTimeout
	responseHeaderTimeout := defaultResponseHeaderTimeout

	if cfg != nil {
		if cfg.Gateway.MaxIdleConns > 0 {
			maxIdleConns = cfg.Gateway.MaxIdleConns
		}
		if cfg.Gateway.MaxIdleConnsPerHost > 0 {
			maxIdleConnsPerHost = cfg.Gateway.MaxIdleConnsPerHost
		}
		if cfg.Gateway.MaxConnsPerHost >= 0 {
			maxConnsPerHost = cfg.Gateway.MaxConnsPerHost
		}
		if cfg.Gateway.IdleConnTimeoutSeconds > 0 {
			idleConnTimeout = time.Duration(cfg.Gateway.IdleConnTimeoutSeconds) * time.Second
		}
		if cfg.Gateway.ResponseHeaderTimeout >= 0 {
			responseHeaderTimeout = time.Duration(cfg.Gateway.ResponseHeaderTimeout) * time.Second
		}
	}

	return poolSettings{
		maxIdleConns:          maxIdleConns,
		maxIdleConnsPerHost:   maxIdleConnsPerHost,
		maxConnsPerHost:       maxConnsPerHost,
		idleConnTimeout:       idleConnTimeout,
		responseHeaderTimeout: responseHeaderTimeout,
	}
}

type upstreamDialer struct {
	base         *net.Dialer
	lookupIPAddr func(context.Context, string) ([]net.IPAddr, error)
	isBlockedIP  func(net.IP) bool
	validateIP   bool
	now          func() time.Time
	cache        sync.Map // map[string]upstreamDNSCacheEntry
}

type upstreamDNSCacheEntry struct {
	addrs     []net.IPAddr
	expiresAt time.Time
}

func newUpstreamNetDialer() *net.Dialer {
	return &net.Dialer{
		Timeout:       defaultUpstreamDialTimeout,
		KeepAlive:     defaultUpstreamDialKeepAlive,
		FallbackDelay: defaultUpstreamDialFallbackDelay,
	}
}

func newUpstreamDialer(validateResolvedIP bool) *upstreamDialer {
	return &upstreamDialer{
		base:         newUpstreamNetDialer(),
		lookupIPAddr: net.DefaultResolver.LookupIPAddr,
		isBlockedIP:  urlvalidator.IsBlockedResolvedIP,
		validateIP:   validateResolvedIP,
		now:          time.Now,
	}
}

func (d *upstreamDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	base := newUpstreamNetDialer()
	if d != nil && d.base != nil {
		base = d.base
	}
	if !strings.HasPrefix(strings.ToLower(network), "tcp") {
		return base.DialContext(ctx, network, address)
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" {
		return base.DialContext(ctx, network, address)
	}
	host = strings.Trim(host, "[]")
	if ip := net.ParseIP(host); ip != nil {
		if d.shouldValidateResolvedIP() && d.isBlockedIP(ip) {
			return nil, fmt.Errorf("resolved ip %s is not allowed", ip.String())
		}
		return base.DialContext(ctx, network, address)
	}
	addrs, err := d.lookupCachedIPAddrs(ctx, host)
	if err != nil {
		return nil, err
	}
	addrs, err = d.filterResolvedIPAddrs(addrs)
	if err != nil {
		return nil, err
	}
	ordered := orderUpstreamIPAddrs(addrs, network)
	if len(ordered) == 0 {
		return nil, fmt.Errorf("resolve %s: no IPs matching %s", host, network)
	}
	return dialResolvedUpstreamIPAddrs(ctx, base, network, port, ordered)
}

func (d *upstreamDialer) shouldValidateResolvedIP() bool {
	return d != nil && d.validateIP
}

func (d *upstreamDialer) filterResolvedIPAddrs(addrs []net.IPAddr) ([]net.IPAddr, error) {
	if !d.shouldValidateResolvedIP() || len(addrs) == 0 {
		return addrs, nil
	}
	isBlockedIP := urlvalidator.IsBlockedResolvedIP
	if d != nil && d.isBlockedIP != nil {
		isBlockedIP = d.isBlockedIP
	}
	for _, addr := range addrs {
		if isBlockedIP(addr.IP) {
			return nil, fmt.Errorf("resolved ip %s is not allowed", addr.IP.String())
		}
	}
	return addrs, nil
}

type upstreamDialResult struct {
	conn net.Conn
	err  error
}

func dialResolvedUpstreamIPAddrs(ctx context.Context, base *net.Dialer, network string, port string, ordered []net.IPAddr) (net.Conn, error) {
	if base == nil {
		base = newUpstreamNetDialer()
	}
	fallbackDelay := base.FallbackDelay
	return dialResolvedUpstreamIPAddrsWithDial(ctx, network, port, ordered, fallbackDelay, base.DialContext)
}

type upstreamDialContextFunc func(ctx context.Context, network, address string) (net.Conn, error)

func dialResolvedUpstreamIPAddrsWithDial(ctx context.Context, network string, port string, ordered []net.IPAddr, fallbackDelay time.Duration, dial upstreamDialContextFunc) (net.Conn, error) {
	if len(ordered) == 0 {
		return nil, fmt.Errorf("dial: no address attempted")
	}
	if dial == nil {
		return nil, fmt.Errorf("dial: no dialer")
	}
	if fallbackDelay <= 0 {
		fallbackDelay = defaultUpstreamDialFallbackDelay
	}
	dialCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan upstreamDialResult, len(ordered))
	startDial := func(addr net.IPAddr) {
		target := net.JoinHostPort(addr.IP.String(), port)
		conn, err := dial(dialCtx, network, target)
		results <- upstreamDialResult{conn: conn, err: err}
	}
	drainStarted := func(remaining int) {
		if remaining <= 0 {
			return
		}
		go func() {
			for i := 0; i < remaining; i++ {
				result := <-results
				if result.conn != nil {
					_ = result.conn.Close()
				}
			}
		}()
	}

	go startDial(ordered[0])
	started := 1
	completed := 0
	var lastErr error
	var timer *time.Timer
	for completed < len(ordered) {
		var timerC <-chan time.Time
		if started < len(ordered) {
			if timer == nil {
				timer = time.NewTimer(fallbackDelay)
			}
			timerC = timer.C
		}
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			cancel()
			drainStarted(started - completed)
			return nil, ctx.Err()
		case <-timerC:
			go startDial(ordered[started])
			started++
			timer = nil
		case result := <-results:
			completed++
			if result.err == nil && result.conn != nil {
				if timer != nil {
					timer.Stop()
				}
				cancel()
				drainStarted(started - completed)
				return result.conn, nil
			}
			if result.err != nil {
				lastErr = result.err
			}
			if started < len(ordered) && timer == nil {
				go startDial(ordered[started])
				started++
			}
		}
	}
	if timer != nil {
		timer.Stop()
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("dial: no address attempted")
}

func (d *upstreamDialer) lookupCachedIPAddrs(ctx context.Context, host string) ([]net.IPAddr, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return nil, errors.New("resolve host is empty")
	}
	now := time.Now()
	if d != nil && d.now != nil {
		now = d.now()
	}
	if d != nil {
		if raw, ok := d.cache.Load(host); ok {
			entry, ok := raw.(upstreamDNSCacheEntry)
			if ok && now.Before(entry.expiresAt) && len(entry.addrs) > 0 {
				return cloneUpstreamIPAddrs(entry.addrs), nil
			}
			d.cache.Delete(host)
		}
	}
	lookup := net.DefaultResolver.LookupIPAddr
	if d != nil && d.lookupIPAddr != nil {
		lookup = d.lookupIPAddr
	}
	addrs, err := lookup(ctx, host)
	if err != nil {
		return nil, err
	}
	addrs = cloneUpstreamIPAddrs(addrs)
	if len(addrs) == 0 {
		return nil, fmt.Errorf("resolve %s: no IPs", host)
	}
	if d != nil {
		d.cache.Store(host, upstreamDNSCacheEntry{
			addrs:     cloneUpstreamIPAddrs(addrs),
			expiresAt: now.Add(defaultUpstreamDNSCacheTTL),
		})
	}
	return addrs, nil
}

func cloneUpstreamIPAddrs(addrs []net.IPAddr) []net.IPAddr {
	if len(addrs) == 0 {
		return nil
	}
	out := make([]net.IPAddr, 0, len(addrs))
	for _, addr := range addrs {
		copied := net.IPAddr{Zone: addr.Zone}
		if addr.IP != nil {
			copied.IP = append(net.IP(nil), addr.IP...)
		}
		out = append(out, copied)
	}
	return out
}

func orderUpstreamIPAddrs(addrs []net.IPAddr, network string) []net.IPAddr {
	network = strings.ToLower(network)
	ordered := make([]net.IPAddr, 0, len(addrs))
	for _, addr := range addrs {
		if addr.IP.To4() != nil && upstreamIPMatchesNetwork(addr.IP, network) {
			ordered = append(ordered, addr)
		}
	}
	for _, addr := range addrs {
		if addr.IP.To4() == nil && upstreamIPMatchesNetwork(addr.IP, network) {
			ordered = append(ordered, addr)
		}
	}
	return ordered
}

func upstreamIPMatchesNetwork(ip net.IP, network string) bool {
	switch network {
	case "tcp4":
		return ip.To4() != nil
	case "tcp6":
		return ip.To4() == nil
	default:
		return true
	}
}

// buildUpstreamTransport 构建上游请求的 Transport
// 使用配置文件中的连接池参数，支持生产环境调优
//
// 参数:
//   - settings: 连接池配置
//   - proxyURL: 代理 URL（nil 表示直连）
//
// 返回:
//   - *http.Transport: 配置好的 Transport 实例
//   - error: 代理配置错误
//
// Transport 参数说明:
//   - MaxIdleConns: 所有主机的最大空闲连接总数
//   - MaxIdleConnsPerHost: 每主机最大空闲连接数（影响连接复用率）
//   - MaxConnsPerHost: 每主机最大连接数（达到后新请求等待）
//   - IdleConnTimeout: 空闲连接超时（超时后关闭）
//   - ResponseHeaderTimeout: 等待响应头超时（不影响流式传输）
func buildUpstreamTransport(settings poolSettings, proxyURL *url.URL, protocolMode string, validateResolvedIP bool) (*http.Transport, error) {
	transport := &http.Transport{
		DialContext:           newUpstreamDialer(validateResolvedIP).DialContext,
		MaxIdleConns:          settings.maxIdleConns,
		MaxIdleConnsPerHost:   settings.maxIdleConnsPerHost,
		MaxConnsPerHost:       settings.maxConnsPerHost,
		IdleConnTimeout:       settings.idleConnTimeout,
		ResponseHeaderTimeout: settings.responseHeaderTimeout,
	}
	switch protocolMode {
	case upstreamProtocolModeOpenAIH2:
		transport.ForceAttemptHTTP2 = true
		// 显式配置 http2 并启用 PING 健康探测，剔除代理/NAT 静默掐断的死连接，
		// 避免请求挂在死连接上直到 TCP 重传超时（分钟级）。
		if _, err := enableOpenAIHTTP2KeepAlive(transport); err != nil {
			return nil, err
		}
	case upstreamProtocolModeOpenAIH1:
		transport.ForceAttemptHTTP2 = false
		transport.TLSNextProto = make(map[string]func(string, *tls.Conn) http.RoundTripper)
	case upstreamProtocolModeOpenAIH1Fallback:
		// 显式禁用 HTTP/2，确保代理不兼容场景回退到 HTTP/1.1。
		transport.ForceAttemptHTTP2 = false
		transport.TLSNextProto = make(map[string]func(string, *tls.Conn) http.RoundTripper)
	}
	if err := proxyutil.ConfigureTransportProxy(transport, proxyURL); err != nil {
		return nil, err
	}
	return transport, nil
}

// enableOpenAIHTTP2KeepAlive 在 http.Transport 上显式配置 HTTP/2 并启用连接健康探测。
// Go 默认惰性配置 http2 且 ReadIdleTimeout=0（不发健康 PING），无法检测被代理/NAT
// 静默掐断的死连接。此处主动设置 ReadIdleTimeout/PingTimeout，让死连接被提前 PING
// 出并关闭，请求得以重建连接而非挂到 TCP 重传超时。返回底层 *http2.Transport 便于测试。
func enableOpenAIHTTP2KeepAlive(transport *http.Transport) (*http2.Transport, error) {
	h2, err := http2.ConfigureTransports(transport)
	if err != nil {
		return nil, err
	}
	if h2 != nil {
		h2.ReadIdleTimeout = openAIHTTP2ReadIdleTimeout
		h2.PingTimeout = openAIHTTP2PingTimeout
	}
	return h2, nil
}

// buildUpstreamTransportWithTLSFingerprint 构建带 TLS 指纹伪装的 Transport
// 使用 utls 库模拟 Claude CLI 的 TLS 指纹
//
// 参数:
//   - settings: 连接池配置
//   - proxyURL: 代理 URL（nil 表示直连）
//   - profile: TLS 指纹配置
//
// 返回:
//   - *http.Transport: 配置好的 Transport 实例
//   - error: 配置错误
//
// 代理类型处理:
//   - nil/空: 直连，使用 TLSFingerprintDialer
//   - http/https: HTTP 代理，使用 HTTPProxyDialer（CONNECT 隧道 + utls 握手）
//   - socks5: SOCKS5 代理，使用 SOCKS5ProxyDialer（SOCKS5 隧道 + utls 握手）
func buildUpstreamTransportWithTLSFingerprint(settings poolSettings, proxyURL *url.URL, profile *tlsfingerprint.Profile, validateResolvedIP bool) (*http.Transport, error) {
	transportProfile := tlsFingerprintHTTPTransportProfile(profile)
	transport := &http.Transport{
		DialContext:           newUpstreamDialer(validateResolvedIP).DialContext,
		MaxIdleConns:          settings.maxIdleConns,
		MaxIdleConnsPerHost:   settings.maxIdleConnsPerHost,
		MaxConnsPerHost:       settings.maxConnsPerHost,
		IdleConnTimeout:       settings.idleConnTimeout,
		ResponseHeaderTimeout: settings.responseHeaderTimeout,
		// 自定义 uTLS DialTLSContext 目前只交给 net/http 的 HTTP/1.x RoundTrip 路径。
		ForceAttemptHTTP2: false,
	}

	dialTLS, fallbackProxy, err := tlsFingerprintDialTLSFunc(transportProfile, proxyURL, validateResolvedIP)
	if err != nil {
		return nil, err
	}
	if dialTLS != nil {
		transport.DialTLSContext = dialTLS
	} else if fallbackProxy {
		// 未知代理类型，回退到普通代理配置（无 TLS 指纹）
		if err := proxyutil.ConfigureTransportProxy(transport, proxyURL); err != nil {
			return nil, err
		}
	}

	return transport, nil
}

// tlsFingerprintDialTLSFunc 根据代理类型构建带 TLS 指纹的 DialTLSContext。
// 直连/HTTP CONNECT/SOCKS5 三种经路共享，供 h1(*http.Transport) 与 h2(*http2.Transport) 复用。
// 返回 (dialTLS, fallbackProxy, err)：dialTLS==nil 且 fallbackProxy==true 表示未知代理类型，
// 调用方应回退到无指纹的普通代理 Transport。
func tlsFingerprintDialTLSFunc(
	transportProfile *tlsfingerprint.Profile,
	proxyURL *url.URL,
	validateResolvedIP bool,
) (func(ctx context.Context, network, addr string) (net.Conn, error), bool, error) {
	if proxyURL == nil {
		slog.Debug("tls_fingerprint_transport_direct")
		dialer := tlsfingerprint.NewDialer(transportProfile, newUpstreamDialer(validateResolvedIP).DialContext)
		return dialer.DialTLSContext, false, nil
	}
	switch strings.ToLower(proxyURL.Scheme) {
	case "socks5", "socks5h":
		slog.Debug("tls_fingerprint_transport_socks5", "proxy", proxyURL.Host)
		socks5Dialer := tlsfingerprint.NewSOCKS5ProxyDialer(transportProfile, proxyURL)
		return socks5Dialer.DialTLSContext, false, nil
	case "http", "https":
		slog.Debug("tls_fingerprint_transport_http_connect", "proxy", proxyURL.Host)
		httpDialer := tlsfingerprint.NewHTTPProxyDialer(transportProfile, proxyURL)
		return httpDialer.DialTLSContext, false, nil
	default:
		slog.Debug("tls_fingerprint_transport_unknown_scheme_fallback", "scheme", proxyURL.Scheme)
		return nil, true, nil
	}
}

// tlsFingerprintProfileWantsH2 判断 profile 是否在 ALPN 中显式提供 h2。
// 仅当模板显式配置 h2 时才走 HTTP/2 出站，现有仅 http/1.1 的模板自然走 h1 经路（零回归）。
func tlsFingerprintProfileWantsH2(profile *tlsfingerprint.Profile) bool {
	if profile == nil {
		return false
	}
	for _, proto := range profile.ALPNProtocols {
		if strings.EqualFold(strings.TrimSpace(proto), "h2") {
			return true
		}
	}
	return false
}

// buildUpstreamRoundTripperWithTLSFingerprint 按 profile ALPN 是否含 h2 分流构建出站 RoundTripper。
//   - 不含 h2（现有绝大多数模板）→ 现有 *http.Transport（HTTP/1.x），字节级零变化。
//   - 含 h2 但缺少可回放的 HTTP2Fingerprint → 回退 *http.Transport，ALPN 收敛为 http/1.1。
//   - 含 h2 且带 HTTP2Fingerprint → 走自定义 h2 replay RoundTripper，消费 settings/wu/priority/ph。
//
// 第二个返回值是 h1 经路的 *http.Transport（用于设置 DisableKeepAlives 等 h1 专属字段）；
// h2 经路时为 nil。
func buildUpstreamRoundTripperWithTLSFingerprint(
	settings poolSettings,
	proxyURL *url.URL,
	profile *tlsfingerprint.Profile,
	validateResolvedIP bool,
) (http.RoundTripper, *http.Transport, error) {
	if !tlsFingerprintProfileWantsH2(profile) {
		transport, err := buildUpstreamTransportWithTLSFingerprint(settings, proxyURL, profile, validateResolvedIP)
		if err != nil {
			return nil, nil, err
		}
		return transport, transport, nil
	}
	parsedFingerprint, err := tlsfpHTTP2.ParseFingerprint(strings.TrimSpace(profile.HTTP2Fingerprint))
	if err != nil {
		return nil, nil, fmt.Errorf("parse http2 fingerprint: %w", err)
	}
	if parsedFingerprint == nil || (len(parsedFingerprint.Settings) == 0 && len(parsedFingerprint.ConnWindowUpdates) == 0 && len(parsedFingerprint.Priorities) == 0 && len(parsedFingerprint.PseudoHeaderOrder) == 0) {
		slog.Debug("tls_fingerprint_h2_profile_missing_replay_data_forced_h1", "profile", profile.Name)
		transport, err := buildUpstreamTransportWithTLSFingerprint(settings, proxyURL, profile, validateResolvedIP)
		if err != nil {
			return nil, nil, err
		}
		return transport, transport, nil
	}
	if _, fallbackProxy, err := tlsFingerprintDialTLSFunc(profile, proxyURL, validateResolvedIP); err != nil {
		return nil, nil, err
	} else if fallbackProxy {
		return nil, nil, fmt.Errorf("unsupported proxy scheme: %s", proxyURL.Scheme)
	}
	slog.Debug("tls_fingerprint_transport_h2_replay_enabled", "profile", profile.Name)
	return &http2FingerprintReplayRoundTripper{
		settings:           settings,
		proxyURL:           proxyURL,
		profile:            profile,
		parsedFingerprint:  parsedFingerprint,
		validateResolvedIP: validateResolvedIP,
	}, nil, nil
}

type responseHeaderTimeoutRoundTripper struct {
	base    http.RoundTripper
	timeout time.Duration
}

func (rt responseHeaderTimeoutRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.base == nil {
		return nil, errors.New("base round tripper is required")
	}
	if req == nil || rt.timeout <= 0 {
		return rt.base.RoundTrip(req)
	}
	ctx, cancel := context.WithCancel(req.Context())
	timer := time.AfterFunc(rt.timeout, cancel)
	reqWithTimeout := req.Clone(ctx)
	resp, err := rt.base.RoundTrip(reqWithTimeout)
	if err != nil {
		timer.Stop()
		cancel()
		return resp, err
	}
	if !timer.Stop() {
		cancel()
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return nil, context.DeadlineExceeded
	}
	if resp != nil && resp.Body != nil {
		resp.Body = cancelOnCloseReadCloser{ReadCloser: resp.Body, cancel: cancel}
	} else {
		cancel()
	}
	return resp, nil
}

func (rt responseHeaderTimeoutRoundTripper) CloseIdleConnections() {
	if closer, ok := rt.base.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

type cancelOnCloseReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (r cancelOnCloseReadCloser) Close() error {
	err := r.ReadCloser.Close()
	r.cancel()
	return err
}

func tlsFingerprintHTTPTransportProfile(profile *tlsfingerprint.Profile) *tlsfingerprint.Profile {
	if profile == nil {
		return nil
	}
	transportProfile := *profile
	if len(profile.ALPNProtocols) == 0 {
		return &transportProfile
	}
	alpn := make([]string, 0, len(profile.ALPNProtocols))
	for _, proto := range profile.ALPNProtocols {
		if strings.EqualFold(strings.TrimSpace(proto), "h2") {
			continue
		}
		alpn = append(alpn, proto)
	}
	if len(alpn) == 0 {
		alpn = []string{"http/1.1"}
	}
	transportProfile.ALPNProtocols = alpn
	return &transportProfile
}

// trackedBody 带跟踪功能的响应体包装器
// 在 Close 时执行回调，用于更新请求计数
type trackedBody struct {
	io.ReadCloser // 原始响应体
	once          sync.Once
	onClose       func() // 关闭时的回调函数
}

// Close 关闭响应体并执行回调
// 使用 sync.Once 确保回调只执行一次
func (b *trackedBody) Close() error {
	err := b.ReadCloser.Close()
	if b.onClose != nil {
		b.once.Do(b.onClose)
	}
	return err
}

// wrapTrackedBody 包装响应体以跟踪关闭事件
// 用于在响应体关闭时更新 inFlight 计数
//
// 参数:
//   - body: 原始响应体
//   - onClose: 关闭时的回调函数
//
// 返回:
//   - io.ReadCloser: 包装后的响应体
func wrapTrackedBody(body io.ReadCloser, onClose func()) io.ReadCloser {
	if body == nil {
		return body
	}
	return &trackedBody{ReadCloser: body, onClose: onClose}
}

// decompressResponseBody 根据 Content-Encoding 解压响应体。
// 当请求显式设置了 accept-encoding 时，Go 的 Transport 不会自动解压，需要手动处理。
// 解压成功后会删除 Content-Encoding 和 Content-Length header（长度已不准确）。
func decompressResponseBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	ce := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	if ce == "" {
		return
	}

	originalBody := resp.Body
	var reader io.Reader
	switch ce {
	case "gzip":
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return // 解压失败，保持原样
		}
		reader = gr
	case "br":
		reader = brotli.NewReader(resp.Body)
	case "deflate":
		reader = flate.NewReader(resp.Body)
	case "zstd":
		bufferedBody := bufio.NewReader(resp.Body)
		resp.Body = &decompressedBody{reader: bufferedBody, closer: originalBody}

		headerBytes, _ := bufferedBody.Peek(zstd.HeaderMaxSize)
		var header zstd.Header
		if err := header.Decode(headerBytes); err != nil {
			slog.Warn("zstd_decompress_failed", "error", err)
			return
		}

		zr, err := zstd.NewReader(bufferedBody)
		if err != nil {
			slog.Warn("zstd_decompress_failed", "error", err)
			return
		}
		reader = &zstdResponseReader{ReadCloser: zr.IOReadCloser()}
	default:
		return
	}

	resp.Body = &decompressedBody{reader: reader, closer: originalBody}
	resp.Header.Del("Content-Encoding")
	resp.Header.Del("Content-Length") // 解压后长度不确定
	resp.ContentLength = -1
}

type zstdResponseReader struct {
	io.ReadCloser
	warnOnce sync.Once
}

func (r *zstdResponseReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		r.warnOnce.Do(func() {
			slog.Warn("zstd_decompress_failed", "error", err)
		})
	}
	return n, err
}

// decompressedBody 组合解压 reader 和原始 body 的 close。
type decompressedBody struct {
	reader io.Reader
	closer io.Closer
}

func (d *decompressedBody) Read(p []byte) (int, error) {
	return d.reader.Read(p)
}

func (d *decompressedBody) Close() error {
	// 如果 reader 本身也是 Closer（如 gzip.Reader），先关闭它
	if rc, ok := d.reader.(io.Closer); ok {
		_ = rc.Close()
	}
	return d.closer.Close()
}
