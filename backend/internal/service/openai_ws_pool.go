package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

const (
	openAIWSConnMaxAge             = 60 * time.Minute
	openAIWSConnHealthCheckIdle    = 90 * time.Second
	openAIWSConnHealthCheckTO      = 2 * time.Second
	openAIWSConnPrewarmExtraDelay  = 2 * time.Second
	openAIWSAcquireCleanupInterval = 3 * time.Second
	openAIWSBackgroundPingInterval = 30 * time.Second
	openAIWSBackgroundSweepTicker  = 30 * time.Second

	openAIWSPrewarmFailureWindow   = 30 * time.Second
	openAIWSPrewarmFailureSuppress = 2

	// openAIWSConnLimitTTLEvictThreshold: 当连接命中 websocket_connection_limit_reached
	// 且连接年龄已达该阈值（接近 60min 单连接 TTL）时，视为 TTL 到期而非账户级并发上限，
	// 触发 evict + 同账号重拨而非账户级 failover。
	openAIWSConnLimitTTLEvictThreshold = 55 * time.Minute
)

// 连接池运行时可配参数默认值与上界。
const (
	defaultOpenAIWSMinIdlePerAccount        = 1
	defaultOpenAIWSMaxIdlePerAccount        = 4
	defaultOpenAIWSStickyReservePercent     = 30
	defaultOpenAIWSNeutralPrewarmPercent    = 20
	defaultOpenAIWSSessionIdleTTLSeconds    = 300
	openAIWSMaxIdlePerAccountUpperBound     = 64
	openAIWSSessionIdleTTLSecondsUpperBound = 3600
)

// openAIWSPoolRuntimeSettings 是连接池运行时可配参数的进程内快照，
// 由 SettingService.refreshCachedSettings 在设置变更时写入；读取走 atomic 零锁。
type openAIWSPoolRuntimeSettings struct {
	neutralPrewarmPercent int
	sessionIdleTTLSeconds int
	legacyMinIdle         int
	legacyMaxIdle         int
	legacyStickyReserve   int
	legacyIdleConfigured  bool
}

var openAIWSPoolRuntimeSettingsCache atomic.Value // *openAIWSPoolRuntimeSettings

// StoreOpenAIWSPoolRuntimeSettings 由 SettingService 在设置刷新时调用，写入连接池运行时快照。
func StoreOpenAIWSPoolRuntimeSettings(neutralPrewarmPercent, sessionIdleTTLSeconds int, legacyStickyReserve ...int) {
	settings := &openAIWSPoolRuntimeSettings{
		neutralPrewarmPercent: neutralPrewarmPercent,
		sessionIdleTTLSeconds: sessionIdleTTLSeconds,
	}
	if len(legacyStickyReserve) > 0 {
		settings.legacyMinIdle = neutralPrewarmPercent
		settings.legacyMaxIdle = sessionIdleTTLSeconds
		settings.legacyStickyReserve = legacyStickyReserve[0]
		settings.legacyIdleConfigured = true
	}
	openAIWSPoolRuntimeSettingsCache.Store(settings)
}

func loadOpenAIWSPoolRuntimeSettings() (*openAIWSPoolRuntimeSettings, bool) {
	cached, ok := openAIWSPoolRuntimeSettingsCache.Load().(*openAIWSPoolRuntimeSettings)
	if !ok || cached == nil {
		return nil, false
	}
	return cached, true
}

func resetOpenAIWSPoolRuntimeSettingsCacheForTest() {
	openAIWSPoolRuntimeSettingsCache = atomic.Value{}
}

// loadOpenAIWSPoolRuntimeSettingsForCompare 返回当前快照值用于变更检测。
func loadOpenAIWSPoolRuntimeSettingsForCompare() (neutralPrewarmPercent, sessionIdleTTLSeconds int, ok bool) {
	cached, found := loadOpenAIWSPoolRuntimeSettings()
	if !found {
		return 0, 0, false
	}
	return cached.neutralPrewarmPercent, cached.sessionIdleTTLSeconds, true
}

// openAIWSReconcileHook 由持有连接池的服务（OpenAIGatewayService）注册，
// 在连接池相关系统设置变更后异步触发收缩 + 冷账号预热。SettingService 不直接引用连接池。
var openAIWSReconcileHook atomic.Pointer[func()]

// RegisterOpenAIWSPoolReconcileHook 注册 reconcile 回调（幂等覆盖）。
func RegisterOpenAIWSPoolReconcileHook(hook func()) {
	if hook == nil {
		openAIWSReconcileHook.Store(nil)
		return
	}
	openAIWSReconcileHook.Store(&hook)
}

// TriggerOpenAIWSPoolReconcile 异步触发已注册的 reconcile 回调；未注册则静默返回。
func TriggerOpenAIWSPoolReconcile() {
	hookPtr := openAIWSReconcileHook.Load()
	if hookPtr == nil || *hookPtr == nil {
		return
	}
	hook := *hookPtr
	go hook()
}

var openAIWSConnEvictHook atomic.Pointer[func(connID string)]

// RegisterOpenAIWSConnEvictHook 注册连接淘汰回调（幂等覆盖）。
// 用于让 conn_id -> last_response_id 等外部映射在连接删除时同步失效。
func RegisterOpenAIWSConnEvictHook(hook func(connID string)) {
	if hook == nil {
		openAIWSConnEvictHook.Store(nil)
		return
	}
	openAIWSConnEvictHook.Store(&hook)
}

func triggerOpenAIWSConnEvict(connID string) {
	id := stringsTrim(connID)
	if id == "" {
		return
	}
	hookPtr := openAIWSConnEvictHook.Load()
	if hookPtr == nil || *hookPtr == nil {
		return
	}
	(*hookPtr)(id)
}

// deleteOpenAIWSAccountConnLocked 统一删除 ap.conns 中的连接并触发淘汰回调，
// 保证任何删除路径都不会泄漏陈旧的 conn_id -> last_response_id 映射。调用方须持有 ap.mu。
func deleteOpenAIWSAccountConnLocked(ap *openAIWSAccountPool, connID string) {
	if ap == nil {
		return
	}
	id := stringsTrim(connID)
	if id == "" {
		return
	}
	delete(ap.conns, id)
	triggerOpenAIWSConnEvict(id)
}

var (
	errOpenAIWSConnClosed               = errors.New("openai ws connection closed")
	errOpenAIWSConnQueueFull            = errors.New("openai ws connection queue full")
	errOpenAIWSPreferredConnUnavailable = errors.New("openai ws preferred connection unavailable")
)

type openAIWSDialError struct {
	StatusCode      int
	ResponseHeaders http.Header
	Err             error
}

func (e *openAIWSDialError) Error() string {
	if e == nil {
		return ""
	}
	if e.StatusCode > 0 {
		return fmt.Sprintf("openai ws dial failed: status=%d err=%v", e.StatusCode, e.Err)
	}
	return fmt.Sprintf("openai ws dial failed: %v", e.Err)
}

func (e *openAIWSDialError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// openAIWSConnProfile 标识连接的握手身份类别：
// session_bound（零值）握手头携带请求级会话身份，仅同会话粘性复用；
// neutral 握手头只含账号级字段，可在无会话请求间任意复用。
// 两类连接严格互不借用，避免握手身份串话。
type openAIWSConnProfile string

const (
	openAIWSConnProfileSessionBound openAIWSConnProfile = ""
	openAIWSConnProfileNeutral      openAIWSConnProfile = "neutral"
)

func openAIWSProfileUsageString(profile openAIWSConnProfile) string {
	if profile == openAIWSConnProfileSessionBound {
		return "session_bound"
	}
	return string(profile)
}

type openAIWSAcquireRequest struct {
	Account         *Account
	WSURL           string
	Headers         http.Header
	ProxyURL        string
	TLSProfile      *tlsfingerprint.Profile
	IdentityKey     string
	PreferredConnID string
	Profile         openAIWSConnProfile
	// ForceNewConn: 强制本次获取新连接（避免复用导致连接内续链状态互相污染）。
	ForceNewConn bool
	// ForcePreferredConn: 强制本次只使用 PreferredConnID，禁止漂移到其它连接。
	ForcePreferredConn bool
	// AffinityOnlyReuse: 只允许复用 PreferredConnID；未命中时新建，禁止借用其它同 profile 连接。
	AffinityOnlyReuse bool
}

type openAIWSConnLease struct {
	pool      *openAIWSConnPool
	accountID int64
	conn      *openAIWSConn
	queueWait time.Duration
	connPick  time.Duration
	reused    bool
	released  atomic.Bool
}

func (l *openAIWSConnLease) activeConn() (*openAIWSConn, error) {
	if l == nil || l.conn == nil {
		return nil, errOpenAIWSConnClosed
	}
	if l.released.Load() {
		return nil, errOpenAIWSConnClosed
	}
	return l.conn, nil
}

func (l *openAIWSConnLease) ConnID() string {
	if l == nil || l.conn == nil {
		return ""
	}
	return l.conn.id
}

func (l *openAIWSConnLease) QueueWaitDuration() time.Duration {
	if l == nil {
		return 0
	}
	return l.queueWait
}

func (l *openAIWSConnLease) ConnPickDuration() time.Duration {
	if l == nil {
		return 0
	}
	return l.connPick
}

func (l *openAIWSConnLease) Reused() bool {
	if l == nil {
		return false
	}
	return l.reused
}

// ConnAge 返回租约绑定连接自建立以来的时长；连接缺失时返回 0。
func (l *openAIWSConnLease) ConnAge() time.Duration {
	if l == nil || l.conn == nil {
		return 0
	}
	return l.conn.age(time.Now())
}

func (l *openAIWSConnLease) HandshakeHeader(name string) string {
	if l == nil || l.conn == nil {
		return ""
	}
	return l.conn.handshakeHeader(name)
}

func (l *openAIWSConnLease) HandshakeHeaders() http.Header {
	if l == nil || l.conn == nil {
		return nil
	}
	return cloneHeader(l.conn.handshakeHeaders)
}

func (l *openAIWSConnLease) IsPrewarmed() bool {
	if l == nil || l.conn == nil {
		return false
	}
	return l.conn.isPrewarmed()
}

func (l *openAIWSConnLease) MarkPrewarmed() {
	if l == nil || l.conn == nil {
		return
	}
	l.conn.markPrewarmed()
}

func (l *openAIWSConnLease) WriteJSON(value any, timeout time.Duration) error {
	conn, err := l.activeConn()
	if err != nil {
		return err
	}
	return conn.writeJSONWithTimeout(context.Background(), value, timeout)
}

func (l *openAIWSConnLease) WriteJSONWithContextTimeout(ctx context.Context, value any, timeout time.Duration) error {
	conn, err := l.activeConn()
	if err != nil {
		return err
	}
	return conn.writeJSONWithTimeout(ctx, value, timeout)
}

func (l *openAIWSConnLease) WriteJSONContext(ctx context.Context, value any) error {
	conn, err := l.activeConn()
	if err != nil {
		return err
	}
	return conn.writeJSON(value, ctx)
}

func (l *openAIWSConnLease) ReadMessage(timeout time.Duration) ([]byte, error) {
	conn, err := l.activeConn()
	if err != nil {
		return nil, err
	}
	return conn.readMessageWithTimeout(timeout)
}

func (l *openAIWSConnLease) ReadMessageContext(ctx context.Context) ([]byte, error) {
	conn, err := l.activeConn()
	if err != nil {
		return nil, err
	}
	return conn.readMessage(ctx)
}

func (l *openAIWSConnLease) ReadMessageWithContextTimeout(ctx context.Context, timeout time.Duration) ([]byte, error) {
	conn, err := l.activeConn()
	if err != nil {
		return nil, err
	}
	return conn.readMessageWithContextTimeout(ctx, timeout)
}

func (l *openAIWSConnLease) PingWithTimeout(timeout time.Duration) error {
	conn, err := l.activeConn()
	if err != nil {
		return err
	}
	return conn.pingWithTimeout(timeout)
}

func (l *openAIWSConnLease) MarkBroken() {
	if l == nil || l.pool == nil || l.conn == nil || l.released.Load() {
		return
	}
	l.pool.evictConn(l.accountID, l.conn.id)
}

func (l *openAIWSConnLease) Release() {
	if l == nil || l.conn == nil {
		return
	}
	if !l.released.CompareAndSwap(false, true) {
		return
	}
	l.conn.release()
}

type openAIWSConn struct {
	id string
	ws openAIWSClientConn

	handshakeHeaders http.Header

	leaseCh   chan struct{}
	closedCh  chan struct{}
	closeOnce sync.Once

	readMu  sync.Mutex
	writeMu sync.Mutex

	waiters       atomic.Int32
	createdAtNano atomic.Int64
	lastUsedNano  atomic.Int64
	prewarmed     atomic.Bool
	profile       openAIWSConnProfile
	reuseKey      string
	identityKey   string
}

func newOpenAIWSConn(id string, _ int64, ws openAIWSClientConn, handshakeHeaders http.Header) *openAIWSConn {
	return newOpenAIWSConnWithProfileAndReuseKey(id, ws, handshakeHeaders, openAIWSConnProfileSessionBound, "")
}

func newOpenAIWSConnWithProfile(id string, ws openAIWSClientConn, handshakeHeaders http.Header, profile openAIWSConnProfile) *openAIWSConn {
	return newOpenAIWSConnWithProfileAndReuseKey(id, ws, handshakeHeaders, profile, "")
}

func newOpenAIWSConnWithProfileAndReuseKey(id string, ws openAIWSClientConn, handshakeHeaders http.Header, profile openAIWSConnProfile, reuseKey string) *openAIWSConn {
	now := time.Now()
	conn := &openAIWSConn{
		id:               id,
		ws:               ws,
		handshakeHeaders: cloneHeader(handshakeHeaders),
		leaseCh:          make(chan struct{}, 1),
		closedCh:         make(chan struct{}),
		profile:          profile,
		reuseKey:         stringsTrim(reuseKey),
	}
	conn.leaseCh <- struct{}{}
	conn.createdAtNano.Store(now.UnixNano())
	conn.lastUsedNano.Store(now.UnixNano())
	return conn
}

func (c *openAIWSConn) matchesAcquire(req openAIWSAcquireRequest) bool {
	if c == nil || c.profile != req.Profile {
		return false
	}
	useIdentity := stringsTrim(req.IdentityKey) != "" || req.TLSProfile != nil
	if useIdentity {
		requiredIdentity := openAIWSAcquireIdentityKey(req)
		return c.identityKey != "" && c.identityKey == requiredIdentity
	}
	if req.Profile == openAIWSConnProfileNeutral {
		return stringsTrim(c.reuseKey) == stringsTrim(openAIWSConnReuseKeyForAcquire(req))
	}
	return true
}

func (c *openAIWSConn) tryAcquire() bool {
	if c == nil {
		return false
	}
	select {
	case <-c.closedCh:
		return false
	default:
	}
	select {
	case <-c.leaseCh:
		select {
		case <-c.closedCh:
			c.release()
			return false
		default:
		}
		return true
	default:
		return false
	}
}

func (c *openAIWSConn) acquire(ctx context.Context) error {
	if c == nil {
		return errOpenAIWSConnClosed
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.closedCh:
			return errOpenAIWSConnClosed
		case <-c.leaseCh:
			select {
			case <-c.closedCh:
				c.release()
				return errOpenAIWSConnClosed
			default:
			}
			return nil
		}
	}
}

func (c *openAIWSConn) release() {
	if c == nil {
		return
	}
	select {
	case c.leaseCh <- struct{}{}:
	default:
	}
	c.touch()
}

func (c *openAIWSConn) close() {
	if c == nil {
		return
	}
	c.closeOnce.Do(func() {
		close(c.closedCh)
		if c.ws != nil {
			_ = c.ws.Close()
		}
		select {
		case c.leaseCh <- struct{}{}:
		default:
		}
	})
}

func (c *openAIWSConn) writeJSONWithTimeout(parent context.Context, value any, timeout time.Duration) error {
	if c == nil {
		return errOpenAIWSConnClosed
	}
	select {
	case <-c.closedCh:
		return errOpenAIWSConnClosed
	default:
	}

	writeCtx := parent
	if writeCtx == nil {
		writeCtx = context.Background()
	}
	if timeout <= 0 {
		return c.writeJSON(value, writeCtx)
	}
	var cancel context.CancelFunc
	writeCtx, cancel = context.WithTimeout(writeCtx, timeout)
	defer cancel()
	return c.writeJSON(value, writeCtx)
}

func (c *openAIWSConn) writeJSON(value any, writeCtx context.Context) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.ws == nil {
		return errOpenAIWSConnClosed
	}
	if writeCtx == nil {
		writeCtx = context.Background()
	}
	if err := c.ws.WriteJSON(writeCtx, value); err != nil {
		return err
	}
	c.touch()
	return nil
}

func (c *openAIWSConn) readMessageWithTimeout(timeout time.Duration) ([]byte, error) {
	return c.readMessageWithContextTimeout(context.Background(), timeout)
}

func (c *openAIWSConn) readMessageWithContextTimeout(parent context.Context, timeout time.Duration) ([]byte, error) {
	if c == nil {
		return nil, errOpenAIWSConnClosed
	}
	select {
	case <-c.closedCh:
		return nil, errOpenAIWSConnClosed
	default:
	}

	if parent == nil {
		parent = context.Background()
	}
	if timeout <= 0 {
		return c.readMessage(parent)
	}
	readCtx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	return c.readMessage(readCtx)
}

func (c *openAIWSConn) readMessage(readCtx context.Context) ([]byte, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	if c.ws == nil {
		return nil, errOpenAIWSConnClosed
	}
	if readCtx == nil {
		readCtx = context.Background()
	}
	payload, err := c.ws.ReadMessage(readCtx)
	if err != nil {
		return nil, err
	}
	c.touch()
	return payload, nil
}

func (c *openAIWSConn) pingWithTimeout(timeout time.Duration) error {
	if c == nil {
		return errOpenAIWSConnClosed
	}
	select {
	case <-c.closedCh:
		return errOpenAIWSConnClosed
	default:
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.ws == nil {
		return errOpenAIWSConnClosed
	}
	if timeout <= 0 {
		timeout = openAIWSConnHealthCheckTO
	}
	pingCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := c.ws.Ping(pingCtx); err != nil {
		return err
	}
	return nil
}

func (c *openAIWSConn) touch() {
	if c == nil {
		return
	}
	c.lastUsedNano.Store(time.Now().UnixNano())
}

func (c *openAIWSConn) createdAt() time.Time {
	if c == nil {
		return time.Time{}
	}
	nano := c.createdAtNano.Load()
	if nano <= 0 {
		return time.Time{}
	}
	return time.Unix(0, nano)
}

func (c *openAIWSConn) lastUsedAt() time.Time {
	if c == nil {
		return time.Time{}
	}
	nano := c.lastUsedNano.Load()
	if nano <= 0 {
		return time.Time{}
	}
	return time.Unix(0, nano)
}

func (c *openAIWSConn) idleDuration(now time.Time) time.Duration {
	if c == nil {
		return 0
	}
	last := c.lastUsedAt()
	if last.IsZero() {
		return 0
	}
	return now.Sub(last)
}

func (c *openAIWSConn) age(now time.Time) time.Duration {
	if c == nil {
		return 0
	}
	created := c.createdAt()
	if created.IsZero() {
		return 0
	}
	return now.Sub(created)
}

func (c *openAIWSConn) isLeased() bool {
	if c == nil {
		return false
	}
	return len(c.leaseCh) == 0
}

func (c *openAIWSConn) handshakeHeader(name string) string {
	if c == nil || c.handshakeHeaders == nil {
		return ""
	}
	return strings.TrimSpace(c.handshakeHeaders.Get(strings.TrimSpace(name)))
}

func (c *openAIWSConn) isPrewarmed() bool {
	if c == nil {
		return false
	}
	return c.prewarmed.Load()
}

func (c *openAIWSConn) markPrewarmed() {
	if c == nil {
		return
	}
	c.prewarmed.Store(true)
}

type openAIWSAccountPool struct {
	mu                   sync.Mutex
	conns                map[string]*openAIWSConn
	pinnedConns          map[string]int
	creating             int
	creatingNeutral      int
	creatingSessionBound int
	lastCleanupAt        time.Time
	// lastAcquire 仅保存 session_bound 请求快照（供 session 预热克隆）；
	// lastNeutralAcquire 保存最近一次 neutral 请求快照（供中性预热克隆），互不污染。
	lastAcquire        *openAIWSAcquireRequest
	lastNeutralAcquire *openAIWSAcquireRequest
	prewarmActive      bool
	prewarmUntil       time.Time
	prewarmFails       int
	prewarmFailAt      time.Time
}

func (ap *openAIWSAccountPool) creatingForProfileLocked(profile openAIWSConnProfile) int {
	if ap == nil {
		return 0
	}
	if profile == openAIWSConnProfileNeutral {
		return ap.creatingNeutral
	}
	return ap.creatingSessionBound
}

func (ap *openAIWSAccountPool) addCreatingLocked(profile openAIWSConnProfile, delta int) {
	if ap == nil || delta <= 0 {
		return
	}
	ap.creating += delta
	if profile == openAIWSConnProfileNeutral {
		ap.creatingNeutral += delta
		return
	}
	ap.creatingSessionBound += delta
}

func (ap *openAIWSAccountPool) doneCreatingLocked(profile openAIWSConnProfile) {
	if ap == nil {
		return
	}
	if ap.creating > 0 {
		ap.creating--
	}
	if profile == openAIWSConnProfileNeutral {
		if ap.creatingNeutral > 0 {
			ap.creatingNeutral--
		}
		return
	}
	if ap.creatingSessionBound > 0 {
		ap.creatingSessionBound--
	}
}

type OpenAIWSPoolMetricsSnapshot struct {
	AcquireTotal            int64
	AcquireReuseTotal       int64
	AcquireCreateTotal      int64
	AcquireQueueWaitTotal   int64
	AcquireQueueWaitMsTotal int64
	ConnPickTotal           int64
	ConnPickMsTotal         int64
	ScaleUpTotal            int64
	ScaleDownTotal          int64
}

type openAIWSPoolMetrics struct {
	acquireTotal          atomic.Int64
	acquireReuseTotal     atomic.Int64
	acquireCreateTotal    atomic.Int64
	acquireQueueWaitTotal atomic.Int64
	acquireQueueWaitMs    atomic.Int64
	connPickTotal         atomic.Int64
	connPickMs            atomic.Int64
	scaleUpTotal          atomic.Int64
	scaleDownTotal        atomic.Int64
}

type openAIWSConnPool struct {
	cfg *config.Config
	// 通过接口解耦底层 WS 客户端实现，默认使用 coder/websocket。
	clientDialer openAIWSClientDialer

	accounts sync.Map // key: int64(accountID), value: *openAIWSAccountPool
	seq      atomic.Uint64

	metrics openAIWSPoolMetrics

	workerStopCh chan struct{}
	workerWg     sync.WaitGroup
	closeOnce    sync.Once
}

func newOpenAIWSConnPool(cfg *config.Config) *openAIWSConnPool {
	pool := &openAIWSConnPool{
		cfg:          cfg,
		clientDialer: newDefaultOpenAIWSClientDialer(),
		workerStopCh: make(chan struct{}),
	}
	pool.startBackgroundWorkers()
	return pool
}

func (p *openAIWSConnPool) SnapshotMetrics() OpenAIWSPoolMetricsSnapshot {
	if p == nil {
		return OpenAIWSPoolMetricsSnapshot{}
	}
	return OpenAIWSPoolMetricsSnapshot{
		AcquireTotal:            p.metrics.acquireTotal.Load(),
		AcquireReuseTotal:       p.metrics.acquireReuseTotal.Load(),
		AcquireCreateTotal:      p.metrics.acquireCreateTotal.Load(),
		AcquireQueueWaitTotal:   p.metrics.acquireQueueWaitTotal.Load(),
		AcquireQueueWaitMsTotal: p.metrics.acquireQueueWaitMs.Load(),
		ConnPickTotal:           p.metrics.connPickTotal.Load(),
		ConnPickMsTotal:         p.metrics.connPickMs.Load(),
		ScaleUpTotal:            p.metrics.scaleUpTotal.Load(),
		ScaleDownTotal:          p.metrics.scaleDownTotal.Load(),
	}
}

func (p *openAIWSConnPool) SnapshotTransportMetrics() OpenAIWSTransportMetricsSnapshot {
	if p == nil {
		return OpenAIWSTransportMetricsSnapshot{}
	}
	if dialer, ok := p.clientDialer.(openAIWSTransportMetricsDialer); ok {
		return dialer.SnapshotTransportMetrics()
	}
	return OpenAIWSTransportMetricsSnapshot{}
}

func (p *openAIWSConnPool) setClientDialerForTest(dialer openAIWSClientDialer) {
	if p == nil || dialer == nil {
		return
	}
	p.clientDialer = dialer
}

// Close 停止后台 worker 并关闭所有空闲连接，应在优雅关闭时调用。
func (p *openAIWSConnPool) Close() {
	if p == nil {
		return
	}
	p.closeOnce.Do(func() {
		if p.workerStopCh != nil {
			close(p.workerStopCh)
		}
		p.workerWg.Wait()
		// 遍历所有账户池，关闭全部空闲连接。
		p.accounts.Range(func(key, value any) bool {
			ap, ok := value.(*openAIWSAccountPool)
			if !ok || ap == nil {
				return true
			}
			ap.mu.Lock()
			for _, conn := range ap.conns {
				if conn != nil && !conn.isLeased() {
					conn.close()
				}
			}
			ap.mu.Unlock()
			return true
		})
	})
}

func (p *openAIWSConnPool) startBackgroundWorkers() {
	if p == nil || p.workerStopCh == nil {
		return
	}
	p.workerWg.Add(2)
	go func() {
		defer p.workerWg.Done()
		p.runBackgroundPingWorker()
	}()
	go func() {
		defer p.workerWg.Done()
		p.runBackgroundCleanupWorker()
	}()
}

type openAIWSIdlePingCandidate struct {
	accountID int64
	conn      *openAIWSConn
}

func (p *openAIWSConnPool) runBackgroundPingWorker() {
	if p == nil {
		return
	}
	ticker := time.NewTicker(openAIWSBackgroundPingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			p.runBackgroundPingSweep()
		case <-p.workerStopCh:
			return
		}
	}
}

func (p *openAIWSConnPool) runBackgroundPingSweep() {
	// coder/websocket Ping must run concurrently with Reader so the pong can be
	// consumed. Idle pooled upstream conns have no reader, so active ping would
	// turn healthy reusable conns into false failures. Actual write/read errors,
	// max-age, and idle-TTL cleanup still evict stale connections.
}

func (p *openAIWSConnPool) snapshotIdleConnsForPing() []openAIWSIdlePingCandidate {
	if p == nil {
		return nil
	}
	candidates := make([]openAIWSIdlePingCandidate, 0)
	p.accounts.Range(func(key, value any) bool {
		accountID, ok := key.(int64)
		if !ok || accountID <= 0 {
			return true
		}
		ap, ok := value.(*openAIWSAccountPool)
		if !ok || ap == nil {
			return true
		}
		ap.mu.Lock()
		for _, conn := range ap.conns {
			if conn == nil || conn.isLeased() || conn.waiters.Load() > 0 {
				continue
			}
			candidates = append(candidates, openAIWSIdlePingCandidate{
				accountID: accountID,
				conn:      conn,
			})
		}
		ap.mu.Unlock()
		return true
	})
	return candidates
}

func (p *openAIWSConnPool) runBackgroundCleanupWorker() {
	if p == nil {
		return
	}
	ticker := time.NewTicker(openAIWSBackgroundSweepTicker)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			p.runBackgroundCleanupSweep(time.Now())
		case <-p.workerStopCh:
			return
		}
	}
}

func (p *openAIWSConnPool) runBackgroundCleanupSweep(now time.Time) {
	if p == nil {
		return
	}
	type cleanupResult struct {
		evicted []*openAIWSConn
	}
	results := make([]cleanupResult, 0)
	p.accounts.Range(func(_ any, value any) bool {
		ap, ok := value.(*openAIWSAccountPool)
		if !ok || ap == nil {
			return true
		}
		accountConcurrency := 0
		ap.mu.Lock()
		if ap.lastNeutralAcquire != nil && ap.lastNeutralAcquire.Account != nil {
			accountConcurrency = ap.lastNeutralAcquire.Account.Concurrency
		} else if ap.lastAcquire != nil && ap.lastAcquire.Account != nil {
			accountConcurrency = ap.lastAcquire.Account.Concurrency
		}
		evicted := p.cleanupAccountLocked(ap, now, accountConcurrency)
		ap.lastCleanupAt = now
		ap.mu.Unlock()
		if len(evicted) > 0 {
			results = append(results, cleanupResult{evicted: evicted})
		}
		return true
	})
	for _, result := range results {
		closeOpenAIWSConns(result.evicted)
	}
}

func (p *openAIWSConnPool) Acquire(ctx context.Context, req openAIWSAcquireRequest) (*openAIWSConnLease, error) {
	if p != nil {
		p.metrics.acquireTotal.Add(1)
	}
	return p.acquire(ctx, cloneOpenAIWSAcquireRequest(req), 0)
}

// ReconcileShrink 立即对所有账号池按当前 max_idle / sticky_reserve 收缩多余空闲连接。
// 设置变更后调用，使收缩即时生效而非等待下一个后台 sweep tick。
func (p *openAIWSConnPool) ReconcileShrink() {
	if p == nil {
		return
	}
	p.runBackgroundCleanupSweep(time.Now())
}

// PrewarmNeutral 为指定账号补足 neutral 空闲连接到 targetIdle（受 neutralMax 与抑制窗约束）。
// 因冷账号无请求快照会在 ensureTargetIdleAsync 早退，这里先 seed lastNeutralAcquire 再补连。
// req 必须是 neutral profile 的合成请求；阻塞拨号，调用方应在后台 goroutine 中执行。
func (p *openAIWSConnPool) PrewarmNeutral(accountID int64, req openAIWSAcquireRequest, targetIdle int) {
	if p == nil || accountID <= 0 || req.Account == nil || stringsTrim(req.WSURL) == "" {
		return
	}
	if targetIdle <= 0 {
		return
	}
	req.Profile = openAIWSConnProfileNeutral
	req = cloneOpenAIWSAcquireRequest(req)

	neutralTarget := p.neutralPrewarmTargetForAccount(req.Account)
	if neutralTarget <= 0 {
		return
	}
	if targetIdle > neutralTarget {
		targetIdle = neutralTarget
	}

	ap := p.getOrCreateAccountPool(accountID)
	if ap == nil {
		return
	}
	ap.mu.Lock()
	// seed neutral 快照，供后续 ensureTargetIdleAsync / cleanup 使用账号信息。
	ap.lastNeutralAcquire = cloneOpenAIWSAcquireRequestPtr(&req)
	reqReuseKey := openAIWSConnReuseKeyForAcquire(req)
	now := time.Now()
	if p.shouldSuppressPrewarmLocked(ap, now) {
		ap.mu.Unlock()
		return
	}
	current := countConnsByProfileAndReuseKeyLocked(ap, openAIWSConnProfileNeutral, reqReuseKey) + ap.creatingForProfileLocked(openAIWSConnProfileNeutral)
	need := targetIdle - current
	if need <= 0 {
		ap.mu.Unlock()
		return
	}
	ap.addCreatingLocked(openAIWSConnProfileNeutral, need)
	p.metrics.scaleUpTotal.Add(int64(need))
	ap.mu.Unlock()

	p.prewarmConns(accountID, req, need)
}

func (p *openAIWSConnPool) acquire(ctx context.Context, req openAIWSAcquireRequest, retry int) (*openAIWSConnLease, error) {
	if p == nil || req.Account == nil || req.Account.ID <= 0 {
		return nil, errors.New("invalid ws acquire request")
	}
	if stringsTrim(req.WSURL) == "" {
		return nil, errors.New("ws url is empty")
	}

	accountID := req.Account.ID
	accountConcurrency := req.Account.Concurrency
	var evicted []*openAIWSConn
	ap := p.getOrCreateAccountPool(accountID)
	ap.mu.Lock()
	if req.Profile == openAIWSConnProfileNeutral {
		ap.lastNeutralAcquire = cloneOpenAIWSAcquireRequestPtr(&req)
	} else {
		ap.lastAcquire = cloneOpenAIWSAcquireRequestPtr(&req)
	}
	now := time.Now()
	if ap.lastCleanupAt.IsZero() || now.Sub(ap.lastCleanupAt) >= openAIWSAcquireCleanupInterval {
		evicted = p.cleanupAccountLocked(ap, now, accountConcurrency)
		ap.lastCleanupAt = now
	}
	pickStartedAt := time.Now()
	allowReuse := !req.ForceNewConn
	preferredConnID := stringsTrim(req.PreferredConnID)
	forcePreferredConn := allowReuse && req.ForcePreferredConn
	affinityOnlyReuse := allowReuse && req.AffinityOnlyReuse
	if allowReuse && req.Profile == openAIWSConnProfileSessionBound {
		affinityOnlyReuse = true
	}
	if allowReuse {
		if forcePreferredConn {
			if preferredConnID == "" {
				p.recordConnPickDuration(time.Since(pickStartedAt))
				ap.mu.Unlock()
				closeOpenAIWSConns(evicted)
				return nil, errOpenAIWSPreferredConnUnavailable
			}
			preferredConn, ok := ap.conns[preferredConnID]
			if !ok || preferredConn == nil || !preferredConn.matchesAcquire(req) {
				p.recordConnPickDuration(time.Since(pickStartedAt))
				ap.mu.Unlock()
				closeOpenAIWSConns(evicted)
				return nil, errOpenAIWSPreferredConnUnavailable
			}
			if preferredConn.tryAcquire() {
				connPick := time.Since(pickStartedAt)
				p.recordConnPickDuration(connPick)
				ap.mu.Unlock()
				closeOpenAIWSConns(evicted)
				if p.shouldHealthCheckConn(preferredConn) {
					if err := preferredConn.pingWithTimeout(openAIWSConnHealthCheckTO); err != nil {
						preferredConn.close()
						p.evictConn(accountID, preferredConn.id)
						if retry < 1 {
							return p.acquire(ctx, req, retry+1)
						}
						return nil, err
					}
				}
				lease := &openAIWSConnLease{
					pool:      p,
					accountID: accountID,
					conn:      preferredConn,
					connPick:  connPick,
					reused:    true,
				}
				p.metrics.acquireReuseTotal.Add(1)
				p.ensureTargetIdleAsync(accountID)
				return lease, nil
			}

			connPick := time.Since(pickStartedAt)
			p.recordConnPickDuration(connPick)
			if int(preferredConn.waiters.Load()) >= p.queueLimitPerConn() {
				ap.mu.Unlock()
				closeOpenAIWSConns(evicted)
				return nil, errOpenAIWSConnQueueFull
			}
			preferredConn.waiters.Add(1)
			ap.mu.Unlock()
			closeOpenAIWSConns(evicted)
			defer preferredConn.waiters.Add(-1)
			waitStart := time.Now()
			p.metrics.acquireQueueWaitTotal.Add(1)

			if err := preferredConn.acquire(ctx); err != nil {
				if errors.Is(err, errOpenAIWSConnClosed) && retry < 1 {
					return p.acquire(ctx, req, retry+1)
				}
				return nil, err
			}
			if p.shouldHealthCheckConn(preferredConn) {
				if err := preferredConn.pingWithTimeout(openAIWSConnHealthCheckTO); err != nil {
					preferredConn.release()
					preferredConn.close()
					p.evictConn(accountID, preferredConn.id)
					if retry < 1 {
						return p.acquire(ctx, req, retry+1)
					}
					return nil, err
				}
			}

			queueWait := time.Since(waitStart)
			p.metrics.acquireQueueWaitMs.Add(queueWait.Milliseconds())
			lease := &openAIWSConnLease{
				pool:      p,
				accountID: accountID,
				conn:      preferredConn,
				queueWait: queueWait,
				connPick:  connPick,
				reused:    true,
			}
			p.metrics.acquireReuseTotal.Add(1)
			p.ensureTargetIdleAsync(accountID)
			return lease, nil
		}

		if preferredConnID != "" {
			if conn, ok := ap.conns[preferredConnID]; ok && conn.matchesAcquire(req) {
				if conn.tryAcquire() {
					connPick := time.Since(pickStartedAt)
					p.recordConnPickDuration(connPick)
					ap.mu.Unlock()
					closeOpenAIWSConns(evicted)
					if p.shouldHealthCheckConn(conn) {
						if err := conn.pingWithTimeout(openAIWSConnHealthCheckTO); err != nil {
							conn.close()
							p.evictConn(accountID, conn.id)
							if retry < 1 {
								return p.acquire(ctx, req, retry+1)
							}
							return nil, err
						}
					}
					lease := &openAIWSConnLease{pool: p, accountID: accountID, conn: conn, connPick: connPick, reused: true}
					p.metrics.acquireReuseTotal.Add(1)
					p.ensureTargetIdleAsync(accountID)
					return lease, nil
				}
				if affinityOnlyReuse {
					connPick := time.Since(pickStartedAt)
					p.recordConnPickDuration(connPick)
					if int(conn.waiters.Load()) >= p.queueLimitPerConn() {
						ap.mu.Unlock()
						closeOpenAIWSConns(evicted)
						return nil, errOpenAIWSConnQueueFull
					}
					conn.waiters.Add(1)
					ap.mu.Unlock()
					closeOpenAIWSConns(evicted)
					defer conn.waiters.Add(-1)
					waitStart := time.Now()
					p.metrics.acquireQueueWaitTotal.Add(1)

					if err := conn.acquire(ctx); err != nil {
						if errors.Is(err, errOpenAIWSConnClosed) && retry < 1 {
							return p.acquire(ctx, req, retry+1)
						}
						return nil, err
					}
					if p.shouldHealthCheckConn(conn) {
						if err := conn.pingWithTimeout(openAIWSConnHealthCheckTO); err != nil {
							conn.release()
							conn.close()
							p.evictConn(accountID, conn.id)
							if retry < 1 {
								return p.acquire(ctx, req, retry+1)
							}
							return nil, err
						}
					}

					queueWait := time.Since(waitStart)
					p.metrics.acquireQueueWaitMs.Add(queueWait.Milliseconds())
					lease := &openAIWSConnLease{
						pool:      p,
						accountID: accountID,
						conn:      conn,
						queueWait: queueWait,
						connPick:  connPick,
						reused:    true,
					}
					p.metrics.acquireReuseTotal.Add(1)
					p.ensureTargetIdleAsync(accountID)
					return lease, nil
				}
			}
		}

		if !affinityOnlyReuse {
			best := p.pickLeastBusyConnLocked(ap, "", req)
			if best != nil && best.tryAcquire() {
				connPick := time.Since(pickStartedAt)
				p.recordConnPickDuration(connPick)
				ap.mu.Unlock()
				closeOpenAIWSConns(evicted)
				if p.shouldHealthCheckConn(best) {
					if err := best.pingWithTimeout(openAIWSConnHealthCheckTO); err != nil {
						best.close()
						p.evictConn(accountID, best.id)
						if retry < 1 {
							return p.acquire(ctx, req, retry+1)
						}
						return nil, err
					}
				}
				lease := &openAIWSConnLease{pool: p, accountID: accountID, conn: best, connPick: connPick, reused: true}
				p.metrics.acquireReuseTotal.Add(1)
				p.ensureTargetIdleAsync(accountID)
				return lease, nil
			}
			for _, conn := range ap.conns {
				if conn == nil || conn == best || !conn.matchesAcquire(req) {
					continue
				}
				if conn.tryAcquire() {
					connPick := time.Since(pickStartedAt)
					p.recordConnPickDuration(connPick)
					ap.mu.Unlock()
					closeOpenAIWSConns(evicted)
					if p.shouldHealthCheckConn(conn) {
						if err := conn.pingWithTimeout(openAIWSConnHealthCheckTO); err != nil {
							conn.close()
							p.evictConn(accountID, conn.id)
							if retry < 1 {
								return p.acquire(ctx, req, retry+1)
							}
							return nil, err
						}
					}
					lease := &openAIWSConnLease{pool: p, accountID: accountID, conn: conn, connPick: connPick, reused: true}
					p.metrics.acquireReuseTotal.Add(1)
					p.ensureTargetIdleAsync(accountID)
					return lease, nil
				}
			}
		}
	}

	connPick := time.Since(pickStartedAt)
	p.recordConnPickDuration(connPick)
	ap.addCreatingLocked(req.Profile, 1)
	ap.mu.Unlock()
	closeOpenAIWSConns(evicted)

	conn, dialErr := p.dialConn(ctx, req)

	ap = p.getOrCreateAccountPool(accountID)
	ap.mu.Lock()
	ap.doneCreatingLocked(req.Profile)
	if dialErr != nil {
		ap.prewarmFails++
		ap.prewarmFailAt = time.Now()
		ap.mu.Unlock()
		return nil, dialErr
	}
	ap.conns[conn.id] = conn
	ap.prewarmFails = 0
	ap.prewarmFailAt = time.Time{}
	ap.mu.Unlock()
	p.metrics.acquireCreateTotal.Add(1)

	if !conn.tryAcquire() {
		if err := conn.acquire(ctx); err != nil {
			conn.close()
			p.evictConn(accountID, conn.id)
			return nil, err
		}
	}
	lease := &openAIWSConnLease{pool: p, accountID: accountID, conn: conn, connPick: connPick}
	p.ensureTargetIdleAsync(accountID)
	return lease, nil
}

func (p *openAIWSConnPool) recordConnPickDuration(duration time.Duration) {
	if p == nil {
		return
	}
	if duration < 0 {
		duration = 0
	}
	p.metrics.connPickTotal.Add(1)
	p.metrics.connPickMs.Add(duration.Milliseconds())
}

func (p *openAIWSConnPool) pickOldestIdleConnLocked(ap *openAIWSAccountPool) *openAIWSConn {
	if ap == nil || len(ap.conns) == 0 {
		return nil
	}
	var oldest *openAIWSConn
	for _, conn := range ap.conns {
		if conn == nil || conn.isLeased() || conn.waiters.Load() > 0 || p.isConnPinnedLocked(ap, conn.id) {
			continue
		}
		if oldest == nil || conn.lastUsedAt().Before(oldest.lastUsedAt()) {
			oldest = conn
		}
	}
	return oldest
}

func (p *openAIWSConnPool) pickOldestIdleNeutralConnLocked(ap *openAIWSAccountPool) *openAIWSConn {
	if ap == nil || len(ap.conns) == 0 {
		return nil
	}
	var oldest *openAIWSConn
	for _, conn := range ap.conns {
		if conn == nil || conn.profile != openAIWSConnProfileNeutral {
			continue
		}
		if conn.isLeased() || conn.waiters.Load() > 0 || p.isConnPinnedLocked(ap, conn.id) {
			continue
		}
		if oldest == nil || conn.lastUsedAt().Before(oldest.lastUsedAt()) {
			oldest = conn
		}
	}
	return oldest
}

func countConnsByProfileLocked(ap *openAIWSAccountPool, profile openAIWSConnProfile) int {
	if ap == nil {
		return 0
	}
	count := 0
	for _, conn := range ap.conns {
		if conn != nil && conn.profile == profile {
			count++
		}
	}
	return count
}

func countConnsByProfileAndReuseKeyLocked(ap *openAIWSAccountPool, profile openAIWSConnProfile, reuseKey string) int {
	if ap == nil {
		return 0
	}
	count := 0
	for _, conn := range ap.conns {
		if conn != nil && openAIWSConnMatchesProfileAndReuseKey(conn, profile, reuseKey) {
			count++
		}
	}
	return count
}

func openAIWSConnMatchesProfileAndReuseKey(conn *openAIWSConn, profile openAIWSConnProfile, reuseKey string) bool {
	if conn == nil || conn.profile != profile {
		return false
	}
	if profile != openAIWSConnProfileNeutral {
		return true
	}
	return stringsTrim(conn.reuseKey) == stringsTrim(reuseKey)
}

func openAIWSConnReuseKeyForAcquire(req openAIWSAcquireRequest) string {
	if stringsTrim(req.IdentityKey) != "" || req.TLSProfile != nil {
		return openAIWSAcquireIdentityKey(req)
	}
	if req.Profile != openAIWSConnProfileNeutral {
		return ""
	}
	headerValue := func(name string) string {
		if req.Headers == nil {
			return ""
		}
		return stringsTrim(req.Headers.Get(name))
	}
	parts := []string{
		stringsTrim(req.WSURL),
		stringsTrim(req.ProxyURL),
		headerValue("authorization"),
		headerValue("OpenAI-Beta"),
		headerValue("originator"),
		headerValue("user-agent"),
		headerValue("chatgpt-account-id"),
	}
	return strings.Join(parts, "\x1f")
}

func (p *openAIWSConnPool) neutralPrewarmPercent() int {
	if rt, ok := loadOpenAIWSPoolRuntimeSettings(); ok {
		return boundedIntOrDefault(rt.neutralPrewarmPercent, 0, 100, defaultOpenAIWSNeutralPrewarmPercent)
	}
	return defaultOpenAIWSNeutralPrewarmPercent
}

func (p *openAIWSConnPool) sessionIdleTTL() time.Duration {
	if rt, ok := loadOpenAIWSPoolRuntimeSettings(); ok {
		seconds := boundedIntOrDefault(rt.sessionIdleTTLSeconds, 1, openAIWSSessionIdleTTLSecondsUpperBound, defaultOpenAIWSSessionIdleTTLSeconds)
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(defaultOpenAIWSSessionIdleTTLSeconds) * time.Second
}

func (p *openAIWSConnPool) neutralPrewarmTargetForAccount(account *Account) int {
	if account == nil {
		return 0
	}
	return p.neutralPrewarmTargetForConcurrency(account.Concurrency)
}

func (p *openAIWSConnPool) neutralPrewarmTargetForConcurrency(concurrency int) int {
	if concurrency <= 0 {
		return 0
	}
	return concurrency * p.neutralPrewarmPercent() / 100
}

// neutralMaxConns is retained for legacy tests/callers; new pool behavior uses
// neutralPrewarmTargetForAccount and does not treat this as a hard cap.
func (p *openAIWSConnPool) neutralMaxConns(accountConcurrency int) int {
	return p.neutralPrewarmTargetForConcurrency(accountConcurrency)
}

// stickyReservePercent 优先读运行时设置（openai_sticky_reserve_percent），
// 其次静态 cfg，最后回退默认 30%。
func (p *openAIWSConnPool) stickyReservePercent() int {
	if rt, ok := loadOpenAIWSPoolRuntimeSettings(); ok {
		if !rt.legacyIdleConfigured {
			return defaultOpenAIWSStickyReservePercent
		}
		if rt.legacyStickyReserve < 0 {
			return 0
		}
		if rt.legacyStickyReserve > 100 {
			return 100
		}
		return rt.legacyStickyReserve
	}
	if p != nil && p.cfg != nil {
		return p.cfg.Gateway.OpenAIWS.StickyReservePercent
	}
	return defaultOpenAIWSStickyReservePercent
}

func (p *openAIWSConnPool) getOrCreateAccountPool(accountID int64) *openAIWSAccountPool {
	if p == nil || accountID <= 0 {
		return nil
	}
	if existing, ok := p.accounts.Load(accountID); ok {
		if ap, typed := existing.(*openAIWSAccountPool); typed && ap != nil {
			return ap
		}
	}
	ap := &openAIWSAccountPool{
		conns:       make(map[string]*openAIWSConn),
		pinnedConns: make(map[string]int),
	}
	actual, _ := p.accounts.LoadOrStore(accountID, ap)
	if typed, ok := actual.(*openAIWSAccountPool); ok && typed != nil {
		return typed
	}
	return ap
}

// ensureAccountPoolLocked 兼容旧调用。
func (p *openAIWSConnPool) ensureAccountPoolLocked(accountID int64) *openAIWSAccountPool {
	return p.getOrCreateAccountPool(accountID)
}

func (p *openAIWSConnPool) getAccountPool(accountID int64) (*openAIWSAccountPool, bool) {
	if p == nil || accountID <= 0 {
		return nil, false
	}
	value, ok := p.accounts.Load(accountID)
	if !ok || value == nil {
		return nil, false
	}
	ap, typed := value.(*openAIWSAccountPool)
	return ap, typed && ap != nil
}

func (p *openAIWSConnPool) isConnPinnedLocked(ap *openAIWSAccountPool, connID string) bool {
	if ap == nil || connID == "" || len(ap.pinnedConns) == 0 {
		return false
	}
	return ap.pinnedConns[connID] > 0
}

func (p *openAIWSConnPool) logConnEvict(conn *openAIWSConn, reason string) {
	if conn == nil {
		return
	}
	logOpenAIWSModeInfo(
		"conn_evict conn_id=%s profile=%s reason=%s",
		normalizeOpenAIWSLogValue(conn.id),
		normalizeOpenAIWSLogValue(string(conn.profile)),
		normalizeOpenAIWSLogValue(reason),
	)
}

func (p *openAIWSConnPool) cleanupAccountLocked(ap *openAIWSAccountPool, now time.Time, accountConcurrency int) []*openAIWSConn {
	if ap == nil {
		return nil
	}
	maxAge := p.maxConnAge()
	sessionIdleTTL := p.sessionIdleTTL()

	evicted := make([]*openAIWSConn, 0)
	for id, conn := range ap.conns {
		if conn == nil {
			deleteOpenAIWSAccountConnLocked(ap, id)
			if len(ap.pinnedConns) > 0 {
				delete(ap.pinnedConns, id)
			}
			continue
		}
		select {
		case <-conn.closedCh:
			p.logConnEvict(conn, "closed")
			deleteOpenAIWSAccountConnLocked(ap, id)
			if len(ap.pinnedConns) > 0 {
				delete(ap.pinnedConns, id)
			}
			evicted = append(evicted, conn)
			continue
		default:
		}
		if p.isConnPinnedLocked(ap, id) || conn.isLeased() || conn.waiters.Load() > 0 {
			continue
		}
		if maxAge > 0 && conn.age(now) > maxAge {
			p.logConnEvict(conn, "conn_max_age")
			deleteOpenAIWSAccountConnLocked(ap, id)
			if len(ap.pinnedConns) > 0 {
				delete(ap.pinnedConns, id)
			}
			evicted = append(evicted, conn)
			p.metrics.scaleDownTotal.Add(1)
			continue
		}
		if conn.profile == openAIWSConnProfileSessionBound && sessionIdleTTL > 0 && now.Sub(conn.lastUsedAt()) > sessionIdleTTL {
			p.logConnEvict(conn, "session_idle_ttl")
			deleteOpenAIWSAccountConnLocked(ap, id)
			if len(ap.pinnedConns) > 0 {
				delete(ap.pinnedConns, id)
			}
			evicted = append(evicted, conn)
			p.metrics.scaleDownTotal.Add(1)
			continue
		}
	}

	neutralTarget := p.neutralPrewarmTargetForConcurrency(accountConcurrency)
	for countConnsByProfileLocked(ap, openAIWSConnProfileNeutral) > neutralTarget {
		idle := p.pickOldestIdleNeutralConnLocked(ap)
		if idle == nil {
			break
		}
		p.logConnEvict(idle, "neutral_over_target")
		deleteOpenAIWSAccountConnLocked(ap, idle.id)
		if len(ap.pinnedConns) > 0 {
			delete(ap.pinnedConns, idle.id)
		}
		evicted = append(evicted, idle)
		p.metrics.scaleDownTotal.Add(1)
	}

	maxIdle := p.maxIdlePerAccount()
	if accountConcurrency <= 0 && maxIdle == 0 && p != nil && p.cfg != nil && p.cfg.Gateway.OpenAIWS.MaxConnsPerAccount > 0 {
		for len(ap.conns) > 0 {
			idle := p.pickOldestIdleConnLocked(ap)
			if idle == nil {
				break
			}
			p.logConnEvict(idle, "idle_over_max")
			deleteOpenAIWSAccountConnLocked(ap, idle.id)
			if len(ap.pinnedConns) > 0 {
				delete(ap.pinnedConns, idle.id)
			}
			evicted = append(evicted, idle)
			p.metrics.scaleDownTotal.Add(1)
		}
	}

	return evicted
}

func (p *openAIWSConnPool) pickLeastBusyConnLocked(ap *openAIWSAccountPool, preferredConnID string, req openAIWSAcquireRequest) *openAIWSConn {
	if ap == nil || len(ap.conns) == 0 {
		return nil
	}
	preferredConnID = stringsTrim(preferredConnID)
	if preferredConnID != "" {
		if conn, ok := ap.conns[preferredConnID]; ok && conn != nil && conn.matchesAcquire(req) {
			return conn
		}
	}
	var best *openAIWSConn
	var bestWaiters int32
	var bestLastUsed time.Time
	for _, conn := range ap.conns {
		if conn == nil || !conn.matchesAcquire(req) {
			continue
		}
		waiters := conn.waiters.Load()
		lastUsed := conn.lastUsedAt()
		if best == nil ||
			waiters < bestWaiters ||
			(waiters == bestWaiters && lastUsed.Before(bestLastUsed)) {
			best = conn
			bestWaiters = waiters
			bestLastUsed = lastUsed
		}
	}
	return best
}

func accountPoolLoadLocked(ap *openAIWSAccountPool) (inflight int, waiters int) {
	if ap == nil {
		return 0, 0
	}
	for _, conn := range ap.conns {
		if conn == nil {
			continue
		}
		if conn.isLeased() {
			inflight++
		}
		waiters += int(conn.waiters.Load())
	}
	return inflight, waiters
}

// AccountPoolLoad 返回指定账号连接池的并发与排队快照。
func (p *openAIWSConnPool) AccountPoolLoad(accountID int64) (inflight int, waiters int, conns int) {
	if p == nil || accountID <= 0 {
		return 0, 0, 0
	}
	ap, ok := p.getAccountPool(accountID)
	if !ok || ap == nil {
		return 0, 0, 0
	}
	ap.mu.Lock()
	defer ap.mu.Unlock()
	inflight, waiters = accountPoolLoadLocked(ap)
	return inflight, waiters, len(ap.conns)
}

func (p *openAIWSConnPool) ensureTargetIdleAsync(accountID int64) {
	if p == nil || accountID <= 0 {
		return
	}

	ap, ok := p.getAccountPool(accountID)
	if !ok || ap == nil {
		return
	}
	ap.mu.Lock()
	defer ap.mu.Unlock()
	if ap.lastNeutralAcquire == nil {
		return
	}
	if ap.prewarmActive {
		return
	}
	now := time.Now()
	if !ap.prewarmUntil.IsZero() && now.Before(ap.prewarmUntil) {
		return
	}
	if p.shouldSuppressPrewarmLocked(ap, now) {
		return
	}
	account := ap.lastNeutralAcquire.Account
	neutralNeed := 0
	neutralReuseKey := openAIWSConnReuseKeyForAcquire(*ap.lastNeutralAcquire)
	target := p.neutralPrewarmTargetForAccount(account)
	current := countConnsByProfileAndReuseKeyLocked(ap, openAIWSConnProfileNeutral, neutralReuseKey) + ap.creatingForProfileLocked(openAIWSConnProfileNeutral)
	if current < target {
		neutralNeed = target - current
	}
	if neutralNeed <= 0 {
		return
	}
	neutralReq := cloneOpenAIWSAcquireRequest(*ap.lastNeutralAcquire)
	ap.prewarmActive = true
	if cooldown := p.prewarmCooldown(); cooldown > 0 {
		ap.prewarmUntil = now.Add(cooldown)
	}
	ap.addCreatingLocked(openAIWSConnProfileNeutral, neutralNeed)
	p.metrics.scaleUpTotal.Add(int64(neutralNeed))

	go func() {
		defer func() {
			if ap, ok := p.getAccountPool(accountID); ok && ap != nil {
				ap.mu.Lock()
				ap.prewarmActive = false
				ap.mu.Unlock()
			}
		}()
		p.prewarmConns(accountID, neutralReq, neutralNeed)
	}()
}

func (p *openAIWSConnPool) targetNeutralConnCountLocked(accountConcurrency int) int {
	return p.neutralPrewarmTargetForConcurrency(accountConcurrency)
}

func (p *openAIWSConnPool) targetConnCountLocked(ap *openAIWSAccountPool, maxConns int) int {
	if ap == nil {
		return 0
	}

	if maxConns <= 0 {
		return 0
	}

	minIdle := p.minIdlePerAccount()
	if minIdle < 0 {
		minIdle = 0
	}
	if minIdle > maxConns {
		minIdle = maxConns
	}

	inflight, waiters := accountPoolLoadLocked(ap)
	utilization := p.targetUtilization()
	demand := inflight + waiters
	if demand <= 0 {
		return minIdle
	}

	target := 1
	if demand > 1 {
		target = int(math.Ceil(float64(demand) / utilization))
	}
	if waiters > 0 && target < len(ap.conns)+1 {
		target = len(ap.conns) + 1
	}
	if target < minIdle {
		target = minIdle
	}
	if target > maxConns {
		target = maxConns
	}
	return target
}

// prewarmConns 顺序拨号补足缺口；prewarmActive 的重置由调用方负责。
func (p *openAIWSConnPool) prewarmConns(accountID int64, req openAIWSAcquireRequest, total int) {
	for i := 0; i < total; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), p.dialTimeout()+openAIWSConnPrewarmExtraDelay)
		conn, err := p.dialConn(ctx, req)
		cancel()

		ap, ok := p.getAccountPool(accountID)
		if !ok || ap == nil {
			if conn != nil {
				conn.close()
			}
			return
		}
		ap.mu.Lock()
		ap.doneCreatingLocked(req.Profile)
		if err != nil {
			ap.prewarmFails++
			ap.prewarmFailAt = time.Now()
			ap.mu.Unlock()
			continue
		}
		ap.conns[conn.id] = conn
		ap.prewarmFails = 0
		ap.prewarmFailAt = time.Time{}
		ap.mu.Unlock()
	}
}

func (p *openAIWSConnPool) evictConn(accountID int64, connID string, reasons ...string) {
	if p == nil || accountID <= 0 || stringsTrim(connID) == "" {
		return
	}
	reason := "evict"
	if len(reasons) > 0 && stringsTrim(reasons[0]) != "" {
		reason = stringsTrim(reasons[0])
	}
	var conn *openAIWSConn
	ap, ok := p.getAccountPool(accountID)
	if ok && ap != nil {
		ap.mu.Lock()
		if c, exists := ap.conns[connID]; exists {
			conn = c
			deleteOpenAIWSAccountConnLocked(ap, connID)
			if len(ap.pinnedConns) > 0 {
				delete(ap.pinnedConns, connID)
			}
		}
		ap.mu.Unlock()
	}
	if conn != nil {
		p.logConnEvict(conn, reason)
		conn.close()
	}
}

func (p *openAIWSConnPool) PinConn(accountID int64, connID string) bool {
	if p == nil || accountID <= 0 {
		return false
	}
	connID = stringsTrim(connID)
	if connID == "" {
		return false
	}
	ap, ok := p.getAccountPool(accountID)
	if !ok || ap == nil {
		return false
	}
	ap.mu.Lock()
	defer ap.mu.Unlock()
	if _, exists := ap.conns[connID]; !exists {
		return false
	}
	if ap.pinnedConns == nil {
		ap.pinnedConns = make(map[string]int)
	}
	ap.pinnedConns[connID]++
	return true
}

func (p *openAIWSConnPool) UnpinConn(accountID int64, connID string) {
	if p == nil || accountID <= 0 {
		return
	}
	connID = stringsTrim(connID)
	if connID == "" {
		return
	}
	ap, ok := p.getAccountPool(accountID)
	if !ok || ap == nil {
		return
	}
	ap.mu.Lock()
	defer ap.mu.Unlock()
	if len(ap.pinnedConns) == 0 {
		return
	}
	count := ap.pinnedConns[connID]
	if count <= 1 {
		delete(ap.pinnedConns, connID)
		return
	}
	ap.pinnedConns[connID] = count - 1
}

func (p *openAIWSConnPool) dialConn(ctx context.Context, req openAIWSAcquireRequest) (*openAIWSConn, error) {
	if p == nil || p.clientDialer == nil {
		return nil, errors.New("openai ws client dialer is nil")
	}
	conn, status, handshakeHeaders, err := p.clientDialer.Dial(ctx, req.WSURL, req.Headers, req.ProxyURL, req.TLSProfile)
	if err != nil {
		return nil, &openAIWSDialError{
			StatusCode:      status,
			ResponseHeaders: cloneHeader(handshakeHeaders),
			Err:             err,
		}
	}
	if conn == nil {
		return nil, &openAIWSDialError{
			StatusCode:      status,
			ResponseHeaders: cloneHeader(handshakeHeaders),
			Err:             errors.New("openai ws dialer returned nil connection"),
		}
	}
	id := p.nextConnID(req.Account.ID)
	wsConn := newOpenAIWSConnWithProfileAndReuseKey(id, conn, handshakeHeaders, req.Profile, openAIWSConnReuseKeyForAcquire(req))
	wsConn.identityKey = openAIWSAcquireIdentityKey(req)
	return wsConn, nil
}

func (p *openAIWSConnPool) nextConnID(accountID int64) string {
	seq := p.seq.Add(1)
	buf := make([]byte, 0, 32)
	buf = append(buf, "oa_ws_"...)
	buf = strconv.AppendInt(buf, accountID, 10)
	buf = append(buf, '_')
	buf = strconv.AppendUint(buf, seq, 10)
	return string(buf)
}

func (p *openAIWSConnPool) shouldHealthCheckConn(conn *openAIWSConn) bool {
	// Active Ping is unsafe for pooled idle conns because no Reader is running
	// to consume pong frames. Let real write/read operations prove liveness.
	_ = conn
	return false
}

func (p *openAIWSConnPool) maxConnsHardCap() int {
	if p != nil && p.cfg != nil && p.cfg.Gateway.OpenAIWS.MaxConnsPerAccount > 0 {
		return p.cfg.Gateway.OpenAIWS.MaxConnsPerAccount
	}
	return 8
}

func (p *openAIWSConnPool) dynamicMaxConnsEnabled() bool {
	if p != nil && p.cfg != nil {
		return p.cfg.Gateway.OpenAIWS.DynamicMaxConnsByAccountConcurrencyEnabled
	}
	return false
}

func (p *openAIWSConnPool) modeRouterV2Enabled() bool {
	if p != nil && p.cfg != nil {
		return p.cfg.Gateway.OpenAIWS.ModeRouterV2Enabled
	}
	return false
}

func (p *openAIWSConnPool) maxConnsFactorByAccount(account *Account) float64 {
	if p == nil || p.cfg == nil || account == nil {
		return 1.0
	}
	switch account.Type {
	case AccountTypeOAuth:
		if p.cfg.Gateway.OpenAIWS.OAuthMaxConnsFactor > 0 {
			return p.cfg.Gateway.OpenAIWS.OAuthMaxConnsFactor
		}
	case AccountTypeAPIKey:
		if p.cfg.Gateway.OpenAIWS.APIKeyMaxConnsFactor > 0 {
			return p.cfg.Gateway.OpenAIWS.APIKeyMaxConnsFactor
		}
	}
	return 1.0
}

func (p *openAIWSConnPool) effectiveMaxConnsByAccount(account *Account) int {
	hardCap := p.maxConnsHardCap()
	if hardCap <= 0 {
		return 0
	}
	if p.modeRouterV2Enabled() {
		if account == nil {
			return hardCap
		}
		if account.Concurrency <= 0 {
			return 0
		}
		return account.Concurrency
	}
	if account == nil || !p.dynamicMaxConnsEnabled() {
		return hardCap
	}
	if account.Concurrency <= 0 {
		// 0/-1 等“无限制”并发场景下，仍由全局硬上限兜底。
		return hardCap
	}
	factor := p.maxConnsFactorByAccount(account)
	if factor <= 0 {
		factor = 1.0
	}
	effective := int(math.Ceil(float64(account.Concurrency) * factor))
	if effective < 1 {
		effective = 1
	}
	if effective > hardCap {
		effective = hardCap
	}
	return effective
}

func (p *openAIWSConnPool) minIdlePerAccount() int {
	if rt, ok := loadOpenAIWSPoolRuntimeSettings(); ok {
		if rt.legacyIdleConfigured && rt.legacyMinIdle >= 0 {
			return rt.legacyMinIdle
		}
	}
	if p != nil && p.cfg != nil && p.cfg.Gateway.OpenAIWS.MinIdlePerAccount >= 0 {
		return p.cfg.Gateway.OpenAIWS.MinIdlePerAccount
	}
	return 0
}

func (p *openAIWSConnPool) maxIdlePerAccount() int {
	if rt, ok := loadOpenAIWSPoolRuntimeSettings(); ok {
		if rt.legacyIdleConfigured && rt.legacyMaxIdle >= 0 {
			return rt.legacyMaxIdle
		}
	}
	if p != nil && p.cfg != nil && p.cfg.Gateway.OpenAIWS.MaxIdlePerAccount >= 0 {
		return p.cfg.Gateway.OpenAIWS.MaxIdlePerAccount
	}
	return 4
}

func (p *openAIWSConnPool) maxConnAge() time.Duration {
	return openAIWSConnMaxAge
}

func (p *openAIWSConnPool) queueLimitPerConn() int {
	if p != nil && p.cfg != nil && p.cfg.Gateway.OpenAIWS.QueueLimitPerConn > 0 {
		return p.cfg.Gateway.OpenAIWS.QueueLimitPerConn
	}
	return 256
}

func (p *openAIWSConnPool) targetUtilization() float64 {
	if p != nil && p.cfg != nil {
		ratio := p.cfg.Gateway.OpenAIWS.PoolTargetUtilization
		if ratio > 0 && ratio <= 1 {
			return ratio
		}
	}
	return 0.7
}

func (p *openAIWSConnPool) prewarmCooldown() time.Duration {
	if p != nil && p.cfg != nil && p.cfg.Gateway.OpenAIWS.PrewarmCooldownMS > 0 {
		return time.Duration(p.cfg.Gateway.OpenAIWS.PrewarmCooldownMS) * time.Millisecond
	}
	return 0
}

func (p *openAIWSConnPool) shouldSuppressPrewarmLocked(ap *openAIWSAccountPool, now time.Time) bool {
	if ap == nil {
		return true
	}
	if ap.prewarmFails <= 0 {
		return false
	}
	if ap.prewarmFailAt.IsZero() {
		ap.prewarmFails = 0
		return false
	}
	if now.Sub(ap.prewarmFailAt) > openAIWSPrewarmFailureWindow {
		ap.prewarmFails = 0
		ap.prewarmFailAt = time.Time{}
		return false
	}
	return ap.prewarmFails >= openAIWSPrewarmFailureSuppress
}

func (p *openAIWSConnPool) dialTimeout() time.Duration {
	if p != nil && p.cfg != nil && p.cfg.Gateway.OpenAIWS.DialTimeoutSeconds > 0 {
		return time.Duration(p.cfg.Gateway.OpenAIWS.DialTimeoutSeconds) * time.Second
	}
	return 10 * time.Second
}

func cloneOpenAIWSAcquireRequest(req openAIWSAcquireRequest) openAIWSAcquireRequest {
	copied := req
	copied.Headers = cloneHeader(req.Headers)
	copied.WSURL = stringsTrim(req.WSURL)
	copied.ProxyURL = stringsTrim(req.ProxyURL)
	copied.IdentityKey = stringsTrim(req.IdentityKey)
	copied.PreferredConnID = stringsTrim(req.PreferredConnID)
	return copied
}

func openAIWSAcquireIdentityKey(req openAIWSAcquireRequest) string {
	if key := stringsTrim(req.IdentityKey); key != "" {
		return key
	}
	parts := []string{
		"profile=" + string(req.Profile),
		"ws=" + stringsTrim(req.WSURL),
		"proxy=" + stringsTrim(req.ProxyURL),
		"tls=" + openAIWSTLSProfileIdentity(req.TLSProfile),
		"auth=" + strings.TrimSpace(req.Headers.Get("authorization")),
		"chatgpt_account=" + strings.TrimSpace(req.Headers.Get("chatgpt-account-id")),
		"ua=" + strings.TrimSpace(req.Headers.Get("user-agent")),
		"originator=" + strings.TrimSpace(req.Headers.Get("originator")),
		"beta=" + strings.TrimSpace(req.Headers.Get("openai-beta")),
	}
	return strings.Join(parts, "\n")
}

func openAIWSTLSProfileIdentity(profile *tlsfingerprint.Profile) string {
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

func cloneOpenAIWSAcquireRequestPtr(req *openAIWSAcquireRequest) *openAIWSAcquireRequest {
	if req == nil {
		return nil
	}
	copied := cloneOpenAIWSAcquireRequest(*req)
	return &copied
}

func cloneHeader(src http.Header) http.Header {
	if src == nil {
		return nil
	}
	dst := make(http.Header, len(src))
	for k, vals := range src {
		if len(vals) == 0 {
			dst[k] = nil
			continue
		}
		copied := make([]string, len(vals))
		copy(copied, vals)
		dst[k] = copied
	}
	return dst
}

func closeOpenAIWSConns(conns []*openAIWSConn) {
	if len(conns) == 0 {
		return
	}
	for _, conn := range conns {
		if conn == nil {
			continue
		}
		conn.close()
	}
}

func stringsTrim(value string) string {
	return strings.TrimSpace(value)
}
