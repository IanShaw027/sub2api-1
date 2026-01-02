package admin

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type OpsWSProxyConfig struct {
	TrustProxy     bool
	TrustedProxies []netip.Prefix
	OriginPolicy   string
}

const (
	envOpsWSTrustProxy     = "OPS_WS_TRUST_PROXY"
	envOpsWSTrustedProxies = "OPS_WS_TRUSTED_PROXIES"
	envOpsWSOriginPolicy   = "OPS_WS_ORIGIN_POLICY"
)

const (
	OriginPolicyStrict     = "strict"
	OriginPolicyPermissive = "permissive"
)

var opsWSProxyConfig = loadOpsWSProxyConfigFromEnv()

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return isAllowedOpsWSOrigin(r)
	},
}

const (
	qpsWSPushInterval       = 2 * time.Second
	qpsWSRefreshInterval    = 5 * time.Second
	qpsWSRequestCountWindow = 1 * time.Minute

	// qpsWSOverviewTimeRange is only used to reuse the existing overview logic for
	// TPS semantics (currently based on input+output tokens in usage_logs).
	qpsWSOverviewTimeRange = "5m"
)

type opsWSQPSCache struct {
	refreshInterval    time.Duration
	requestCountWindow time.Duration
	overviewTimeRange  string

	lastUpdatedUnixNano atomic.Int64
	payload             atomic.Value // []byte
	lastTPS             atomic.Value // float64

	mu sync.Mutex
}

var qpsWSCache = &opsWSQPSCache{
	refreshInterval:    qpsWSRefreshInterval,
	requestCountWindow: qpsWSRequestCountWindow,
	overviewTimeRange:  qpsWSOverviewTimeRange,
}

func roundTo1DP(v float64) float64 {
	return math.Round(v*10) / 10
}

func (c *opsWSQPSCache) getPayload(opsService *service.OpsService) []byte {
	if c == nil || opsService == nil {
		return nil
	}

	nowUnixNano := time.Now().UnixNano()
	if cached, ok := c.payload.Load().([]byte); ok && cached != nil {
		last := c.lastUpdatedUnixNano.Load()
		if last > 0 && time.Duration(nowUnixNano-last) < c.refreshInterval {
			return cached
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UTC()
	if cached, ok := c.payload.Load().([]byte); ok && cached != nil {
		last := c.lastUpdatedUnixNano.Load()
		if last > 0 && time.Duration(now.UnixNano()-last) < c.refreshInterval {
			return cached
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Exact last-1m stats used for both QPS and request_count to keep semantics consistent.
	windowStats, err := opsService.GetWindowStats(ctx, now.Add(-c.requestCountWindow), now)
	if err != nil || windowStats == nil {
		if err != nil {
			log.Printf("[OpsWS] get window stats failed: %v", err)
		}
		if cached, ok := c.payload.Load().([]byte); ok && cached != nil {
			return cached
		}
		return nil
	}

	requestCount := windowStats.SuccessCount + windowStats.ErrorCount
	qps := 0.0
	if c.requestCountWindow > 0 {
		qps = roundTo1DP(float64(requestCount) / c.requestCountWindow.Seconds())
	}

	// TPS comes from the dashboard overview to preserve existing TPS semantics.
	tps := 0.0
	if v := c.lastTPS.Load(); v != nil {
		if prev, ok := v.(float64); ok {
			tps = prev
		}
	}
	if overview, err := opsService.GetDashboardOverview(ctx, c.overviewTimeRange); err != nil {
		log.Printf("[OpsWS] get overview failed: %v", err)
	} else if overview != nil {
		tps = overview.TPS.Current
		c.lastTPS.Store(tps)
	}

	payload := gin.H{
		"type":      "qps_update",
		"timestamp": now.Format(time.RFC3339),
		"data": gin.H{
			"qps":           qps,
			"tps":           tps,
			"request_count": requestCount,
		},
	}

	msg, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[OpsWS] marshal payload failed: %v", err)
		if cached, ok := c.payload.Load().([]byte); ok && cached != nil {
			return cached
		}
		return nil
	}

	c.payload.Store(msg)
	c.lastUpdatedUnixNano.Store(now.UnixNano())
	return msg
}

// QPSWSHandler handles realtime QPS push via WebSocket.
// GET /api/v1/admin/ops/ws/qps
func (h *OpsHandler) QPSWSHandler(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[OpsWS] upgrade failed: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	// Set pong handler
	if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
		log.Printf("[OpsWS] set read deadline failed: %v", err)
		return
	}
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	// Push QPS data every 2 seconds (values are globally cached and refreshed at most once per qpsWSRefreshInterval).
	ticker := time.NewTicker(qpsWSPushInterval)
	defer ticker.Stop()

	// Heartbeat ping every 30 seconds
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	for {
		select {
		case <-ticker.C:
			// Fetch cached payload (built from exact 1m window stats + 5m dashboard overview TPS).
			msg := qpsWSCache.getPayload(h.opsService)
			if msg == nil {
				continue
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("[OpsWS] write failed: %v", err)
				return
			}
		case <-pingTicker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("[OpsWS] ping failed: %v", err)
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func isAllowedOpsWSOrigin(r *http.Request) bool {
	if r == nil {
		return false
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		switch strings.ToLower(strings.TrimSpace(opsWSProxyConfig.OriginPolicy)) {
		case OriginPolicyStrict:
			return false
		case OriginPolicyPermissive, "":
			return true
		default:
			return true
		}
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Hostname() == "" {
		return false
	}
	originHost := strings.ToLower(parsed.Hostname())

	trustProxyHeaders := shouldTrustOpsWSProxyHeaders(r)
	reqHost := hostWithoutPort(r.Host)
	if trustProxyHeaders {
		xfHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
		if xfHost != "" {
			xfHost = strings.TrimSpace(strings.Split(xfHost, ",")[0])
			if xfHost != "" {
				reqHost = hostWithoutPort(xfHost)
			}
		}
	}
	reqHost = strings.ToLower(reqHost)
	if reqHost == "" {
		return false
	}
	return originHost == reqHost
}

func shouldTrustOpsWSProxyHeaders(r *http.Request) bool {
	if r == nil {
		return false
	}
	if !opsWSProxyConfig.TrustProxy {
		return false
	}
	peerIP, ok := requestPeerIP(r)
	if !ok {
		return false
	}
	return isAddrInTrustedProxies(peerIP, opsWSProxyConfig.TrustedProxies)
}

func requestPeerIP(r *http.Request) (netip.Addr, bool) {
	if r == nil {
		return netip.Addr{}, false
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	if host == "" {
		return netip.Addr{}, false
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

func isAddrInTrustedProxies(addr netip.Addr, trusted []netip.Prefix) bool {
	if !addr.IsValid() {
		return false
	}
	for _, p := range trusted {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

func loadOpsWSProxyConfigFromEnv() OpsWSProxyConfig {
	cfg := OpsWSProxyConfig{
		TrustProxy:     true,
		TrustedProxies: defaultTrustedProxies(),
		OriginPolicy:   OriginPolicyPermissive,
	}

	if v := strings.TrimSpace(os.Getenv(envOpsWSTrustProxy)); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			cfg.TrustProxy = parsed
		} else {
			log.Printf("[OpsWS] invalid %s=%q (expected bool); using default=%v", envOpsWSTrustProxy, v, cfg.TrustProxy)
		}
	}

	if raw := strings.TrimSpace(os.Getenv(envOpsWSTrustedProxies)); raw != "" {
		prefixes, invalid := parseTrustedProxyList(raw)
		if len(invalid) > 0 {
			log.Printf("[OpsWS] invalid %s entries ignored: %s", envOpsWSTrustedProxies, strings.Join(invalid, ", "))
		}
		cfg.TrustedProxies = prefixes
	}

	if v := strings.TrimSpace(os.Getenv(envOpsWSOriginPolicy)); v != "" {
		normalized := strings.ToLower(v)
		switch normalized {
		case OriginPolicyStrict, OriginPolicyPermissive:
			cfg.OriginPolicy = normalized
		default:
			log.Printf("[OpsWS] invalid %s=%q (expected %q or %q); using default=%q", envOpsWSOriginPolicy, v, OriginPolicyStrict, OriginPolicyPermissive, cfg.OriginPolicy)
		}
	}

	return cfg
}

func defaultTrustedProxies() []netip.Prefix {
	prefixes, _ := parseTrustedProxyList("127.0.0.0/8,::1/128")
	return prefixes
}

func parseTrustedProxyList(raw string) (prefixes []netip.Prefix, invalid []string) {
	for _, token := range strings.Split(raw, ",") {
		item := strings.TrimSpace(token)
		if item == "" {
			continue
		}

		var (
			p   netip.Prefix
			err error
		)
		if strings.Contains(item, "/") {
			p, err = netip.ParsePrefix(item)
		} else {
			var addr netip.Addr
			addr, err = netip.ParseAddr(item)
			if err == nil {
				addr = addr.Unmap()
				bits := 128
				if addr.Is4() {
					bits = 32
				}
				p = netip.PrefixFrom(addr, bits)
			}
		}

		if err != nil || !p.IsValid() {
			invalid = append(invalid, item)
			continue
		}

		prefixes = append(prefixes, p.Masked())
	}
	return prefixes, invalid
}

func hostWithoutPort(hostport string) string {
	hostport = strings.TrimSpace(hostport)
	if hostport == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(hostport); err == nil {
		return host
	}
	if strings.HasPrefix(hostport, "[") && strings.HasSuffix(hostport, "]") {
		return strings.Trim(hostport, "[]")
	}
	parts := strings.Split(hostport, ":")
	return parts[0]
}
