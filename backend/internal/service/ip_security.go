package service

import (
	"context"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	IPSecuritySourceWeb            = "web"
	IPSecuritySourceAPIKey         = "apikey"
	defaultIPSecurityWindowMinutes = 10
	defaultIPSecurityThreshold     = 4
	userIPCacheTTL                 = 365 * 24 * time.Hour
	ipSecurityConfigCacheTTL       = 5 * time.Minute
	maxUserIPHistory               = 256
	ipSecurityRedisTimeout         = 20 * time.Millisecond
	ipSecurityRedisCircuitDuration = 5 * time.Second
)

var (
	userIPKnownScript = redis.NewScript(`
local state = redis.call('GET', KEYS[1])
if not state then return -1 end
if state == 'saturated' then return 2 end
if redis.call('SISMEMBER', KEYS[2], ARGV[1]) == 1 then return 1 end
return 0`)
	ipSecurityStateScript = redis.NewScript(`
if not redis.call('GET', KEYS[1]) then return -1 end
if redis.call('SISMEMBER', KEYS[2], ARGV[1]) == 1 then return 2 end
if redis.call('SISMEMBER', KEYS[3], ARGV[1]) == 1 then return 1 end
return 0`)
	newAccountWindowScript = redis.NewScript(`
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
redis.call('ZADD', KEYS[1], 'NX', ARGV[2], ARGV[3])
redis.call('EXPIRE', KEYS[1], ARGV[4])
return redis.call('ZCARD', KEYS[1])`)
)

type ipSecurityStatus uint8

const (
	ipSecurityStatusNone ipSecurityStatus = iota
	ipSecurityStatusBanned
	ipSecurityStatusWhitelisted
)

type IPSecurityConfig struct {
	Enabled          bool      `json:"enabled"`
	WindowMinutes    int       `json:"window_minutes"`
	AccountThreshold int       `json:"account_threshold"`
	LearningUntil    time.Time `json:"learning_until"`
}

type IPSecurityActivity struct {
	IPAddress    string         `json:"ip_address"`
	PeerIP       string         `json:"peer_ip"`
	ForwardedFor string         `json:"forwarded_for"`
	UserID       int64          `json:"user_id"`
	Source       string         `json:"source"`
	APIKeyID     int64          `json:"api_key_id"`
	Method       string         `json:"method"`
	Path         string         `json:"path"`
	RequestID    string         `json:"request_id"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type IPSecurityBan struct {
	ID                   int64      `json:"id"`
	IPAddress            string     `json:"ip_address"`
	Status               string     `json:"status"`
	Reason               string     `json:"reason"`
	AccountThreshold     int        `json:"account_threshold"`
	WindowMinutes        int        `json:"window_minutes"`
	DetectedAccountCount int        `json:"detected_account_count"`
	FirstSeenAt          time.Time  `json:"first_seen_at"`
	LastSeenAt           time.Time  `json:"last_seen_at"`
	CreatedAt            time.Time  `json:"created_at"`
	ReleasedAt           *time.Time `json:"released_at,omitempty"`
}

type IPSecurityActivityDetail struct {
	IPSecurityActivity
	UserEmail    string    `json:"user_email"`
	UserUsername string    `json:"user_username"`
	RequestCount int64     `json:"request_count"`
	FirstSeenAt  time.Time `json:"first_seen_at"`
	LastSeenAt   time.Time `json:"last_seen_at"`
}

type IPSecurityRepository interface {
	GetUserIPs(ctx context.Context, userID int64) ([]string, bool, error)
	AddUserIPActivity(ctx context.Context, activity IPSecurityActivity, now time.Time) (added, saturated bool, err error)
	CreateBan(ctx context.Context, ban *IPSecurityBan) (bool, error)
	IsIPStatus(ctx context.Context, ip, status string) (bool, error)
	LoadIPStatuses(ctx context.Context) (active, whitelisted []string, err error)
	ListBans(ctx context.Context, status string, limit, offset int) ([]IPSecurityBan, int64, error)
	GetBan(ctx context.Context, id int64) (*IPSecurityBan, error)
	ListActivity(ctx context.Context, ip string, since, until time.Time) ([]IPSecurityActivityDetail, error)
	WhitelistBan(ctx context.Context, id int64, releasedBy int64, now time.Time) (*IPSecurityBan, error)
	RemoveWhitelist(ctx context.Context, id, removedBy int64, now time.Time) (*IPSecurityBan, error)
}

type IPSecuritySettings interface {
	GetValue(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

type ipSecurityConfigCache struct {
	value IPSecurityConfig
	at    time.Time
}

type userIPLocalState struct {
	ips       map[string]struct{}
	saturated bool
}

type IPSecurityService struct {
	repo              IPSecurityRepository
	settings          IPSecuritySettings
	rdb               *redis.Client
	config            atomic.Pointer[ipSecurityConfigCache]
	configMu          sync.Mutex
	stateMu           sync.RWMutex
	fallback          map[string]struct{}
	whitelist         map[string]struct{}
	userMu            sync.RWMutex
	userIPs           map[int64]userIPLocalState
	redisFailureUntil atomic.Int64
}

var globalIPSecurityService atomic.Pointer[IPSecurityService]

func SetGlobalIPSecurityService(svc *IPSecurityService) { globalIPSecurityService.Store(svc) }
func GlobalIPSecurityService() *IPSecurityService       { return globalIPSecurityService.Load() }

func NewIPSecurityService(repo IPSecurityRepository, settings IPSecuritySettings, rdb *redis.Client) *IPSecurityService {
	return &IPSecurityService{repo: repo, settings: settings, rdb: rdb, userIPs: make(map[int64]userIPLocalState)}
}

func DefaultIPSecurityConfig() IPSecurityConfig {
	return IPSecurityConfig{WindowMinutes: defaultIPSecurityWindowMinutes, AccountThreshold: defaultIPSecurityThreshold}
}

func normalizeIPSecurityConfig(cfg IPSecurityConfig) IPSecurityConfig {
	if cfg.WindowMinutes < 1 {
		cfg.WindowMinutes = 1
	}
	if cfg.WindowMinutes > 1440 {
		cfg.WindowMinutes = 1440
	}
	if cfg.AccountThreshold < 2 {
		cfg.AccountThreshold = 2
	}
	if cfg.AccountThreshold > 100 {
		cfg.AccountThreshold = 100
	}
	return cfg
}

func readBoolSetting(ctx context.Context, settings IPSecuritySettings, key string, fallback bool) bool {
	raw, err := settings.GetValue(ctx, key)
	if err != nil {
		return fallback
	}
	return strings.EqualFold(strings.TrimSpace(raw), "true")
}

func readIntSetting(ctx context.Context, settings IPSecuritySettings, key string, fallback int) int {
	raw, err := settings.GetValue(ctx, key)
	if err != nil {
		return fallback
	}
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return v
}

func readTimeSetting(ctx context.Context, settings IPSecuritySettings, key string) time.Time {
	raw, err := settings.GetValue(ctx, key)
	if err != nil || strings.TrimSpace(raw) == "" {
		return time.Time{}
	}
	value, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}
	}
	return value
}

func (s *IPSecurityService) GetConfig(ctx context.Context) IPSecurityConfig {
	if s == nil || s.settings == nil {
		return DefaultIPSecurityConfig()
	}
	if cached := s.config.Load(); cached != nil && time.Since(cached.at) < ipSecurityConfigCacheTTL {
		return cached.value
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	if cached := s.config.Load(); cached != nil && time.Since(cached.at) < ipSecurityConfigCacheTTL {
		return cached.value
	}
	cfg := DefaultIPSecurityConfig()
	cfg.Enabled = readBoolSetting(ctx, s.settings, SettingKeyIPMultiAccountBanEnabled, false)
	cfg.WindowMinutes = readIntSetting(ctx, s.settings, SettingKeyIPMultiAccountBanWindowMinutes, cfg.WindowMinutes)
	cfg.AccountThreshold = readIntSetting(ctx, s.settings, SettingKeyIPMultiAccountBanThreshold, cfg.AccountThreshold)
	cfg.LearningUntil = readTimeSetting(ctx, s.settings, SettingKeyIPMultiAccountBanLearningUntil)
	cfg = normalizeIPSecurityConfig(cfg)
	s.config.Store(&ipSecurityConfigCache{value: cfg, at: time.Now()})
	return cfg
}

func (s *IPSecurityService) InvalidateConfig() { s.config.Store(nil) }

func (s *IPSecurityService) IsBlocked(ctx context.Context, rawIP string) bool {
	ip := normalizePublicIP(rawIP)
	if ip == "" || s == nil {
		return false
	}
	return s.resolveIPStatus(ctx, ip) == ipSecurityStatusBanned
}

// resolveIPStatus treats Redis as authoritative only when the loaded marker is
// present. A missing marker (for example after FLUSHDB/restart) or Redis error
// forces a DB lookup instead of trusting a possibly stale process-local miss.
// Local state is retained solely as a last-resort snapshot when both shared
// stores are unavailable.
func (s *IPSecurityService) resolveIPStatus(ctx context.Context, ip string) ipSecurityStatus {
	if status, ok := s.sharedIPStatus(ctx, ip); ok {
		s.rememberIPStatus(ip, status)
		return status
	}
	if status, ok := s.databaseIPStatus(ctx, ip); ok {
		s.rememberIPStatus(ip, status)
		return status
	}
	return s.localIPStatus(ip)
}

func (s *IPSecurityService) sharedIPStatus(ctx context.Context, ip string) (ipSecurityStatus, bool) {
	if s == nil || s.rdb == nil {
		return ipSecurityStatusNone, false
	}
	redisCtx, cancel, ready := s.redisContext(ctx)
	if !ready {
		return ipSecurityStatusNone, false
	}
	defer cancel()

	result, err := ipSecurityStateScript.Run(redisCtx, s.rdb, []string{
		"ipsec:{state}:loaded",
		"ipsec:{state}:whitelisted",
		"ipsec:{state}:banned",
	}, ip).Int()
	if err != nil {
		s.noteRedisFailure(err)
		return ipSecurityStatusNone, false
	}
	switch result {
	case int(ipSecurityStatusBanned):
		return ipSecurityStatusBanned, true
	case int(ipSecurityStatusWhitelisted):
		return ipSecurityStatusWhitelisted, true
	case int(ipSecurityStatusNone):
		return ipSecurityStatusNone, true
	default:
		// -1 means the shared snapshot has not been loaded.
		return ipSecurityStatusNone, false
	}
}

func (s *IPSecurityService) databaseIPStatus(ctx context.Context, ip string) (ipSecurityStatus, bool) {
	if s == nil || s.repo == nil {
		return ipSecurityStatusNone, false
	}
	whitelisted, err := s.repo.IsIPStatus(ctx, ip, "whitelisted")
	if err != nil {
		return ipSecurityStatusNone, false
	}
	if whitelisted {
		return ipSecurityStatusWhitelisted, true
	}
	banned, err := s.repo.IsIPStatus(ctx, ip, "active")
	if err != nil {
		return ipSecurityStatusNone, false
	}
	if banned {
		return ipSecurityStatusBanned, true
	}
	return ipSecurityStatusNone, true
}

func (s *IPSecurityService) localIPStatus(ip string) ipSecurityStatus {
	if s == nil {
		return ipSecurityStatusNone
	}
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	if _, ok := s.whitelist[ip]; ok {
		return ipSecurityStatusWhitelisted
	}
	if _, ok := s.fallback[ip]; ok {
		return ipSecurityStatusBanned
	}
	return ipSecurityStatusNone
}

func (s *IPSecurityService) rememberIPStatus(ip string, status ipSecurityStatus) {
	if s == nil || ip == "" {
		return
	}
	s.stateMu.Lock()
	if s.fallback == nil {
		s.fallback = make(map[string]struct{})
	}
	if s.whitelist == nil {
		s.whitelist = make(map[string]struct{})
	}
	delete(s.fallback, ip)
	delete(s.whitelist, ip)
	switch status {
	case ipSecurityStatusBanned:
		s.fallback[ip] = struct{}{}
	case ipSecurityStatusWhitelisted:
		s.whitelist[ip] = struct{}{}
	}
	s.stateMu.Unlock()
}

// Observe records only a user's first-ever use of an IP. The normal hot path is one Redis lookup.
func (s *IPSecurityService) Observe(ctx context.Context, activity IPSecurityActivity) {
	if s == nil || s.repo == nil || activity.UserID <= 0 {
		return
	}
	activity.IPAddress = normalizePublicIP(activity.IPAddress)
	activity.PeerIP = normalizeAnyIP(activity.PeerIP)
	if activity.IPAddress == "" {
		return
	}
	known, cacheReady := s.userIPKnown(ctx, activity.UserID, activity.IPAddress)
	if !cacheReady {
		ips, saturated, err := s.repo.GetUserIPs(ctx, activity.UserID)
		if err != nil {
			return
		}
		known = saturated || containsIP(ips, activity.IPAddress)
		_ = s.seedUserIPs(ctx, activity.UserID, ips, saturated)
	}
	if known {
		return
	}

	added, saturated, err := s.repo.AddUserIPActivity(ctx, activity, time.Now())
	if err != nil {
		return
	}
	_ = s.addCachedUserIP(ctx, activity.UserID, activity.IPAddress, saturated)
	if !added {
		return
	}

	cfg := s.GetConfig(ctx)
	if !cfg.Enabled || (!cfg.LearningUntil.IsZero() && time.Now().Before(cfg.LearningUntil)) || s.isWhitelisted(ctx, activity.IPAddress) {
		return
	}
	now := time.Now()
	count, err := s.addNewAccountToWindow(ctx, activity.IPAddress, activity.UserID, now, cfg.WindowMinutes)
	if err != nil || count < cfg.AccountThreshold {
		return
	}
	if s.IsBlocked(ctx, activity.IPAddress) {
		return
	}
	firstSeen, lastSeen := now.Add(-time.Duration(cfg.WindowMinutes)*time.Minute), now
	ban := &IPSecurityBan{
		IPAddress: activity.IPAddress, Status: "active", Reason: "short-window multi-account activity",
		AccountThreshold: cfg.AccountThreshold, WindowMinutes: cfg.WindowMinutes,
		DetectedAccountCount: count, FirstSeenAt: firstSeen, LastSeenAt: lastSeen, CreatedAt: now,
	}
	created, err := s.repo.CreateBan(ctx, ban)
	if err != nil || !created {
		return
	}
	s.stateMu.Lock()
	if s.fallback == nil {
		s.fallback = make(map[string]struct{})
	}
	s.fallback[activity.IPAddress] = struct{}{}
	s.stateMu.Unlock()
	if s.rdb != nil {
		if redisCtx, cancel, ok := s.redisContext(ctx); ok {
			defer cancel()
			if err := s.rdb.SAdd(redisCtx, "ipsec:{state}:banned", activity.IPAddress).Err(); err != nil {
				s.noteRedisFailure(err)
			}
		}
	}
}

func (s *IPSecurityService) userIPKnown(ctx context.Context, userID int64, ip string) (known, cacheReady bool) {
	s.userMu.RLock()
	local, localLoaded := s.userIPs[userID]
	_, localKnown := local.ips[ip]
	localSaturated := local.saturated
	s.userMu.RUnlock()
	if localLoaded && (localKnown || localSaturated) {
		return true, true
	}
	if s.rdb != nil {
		redisCtx, cancel, ok := s.redisContext(ctx)
		if !ok {
			return false, false
		}
		defer cancel()
		marker, set := userIPCacheKeys(userID)
		result, err := userIPKnownScript.Run(redisCtx, s.rdb, []string{marker, set}, ip).Int64()
		if err == nil && result >= 0 {
			if result == 2 {
				s.setLocalUserState(userID, nil, true)
				return true, true
			}
			if result == 1 {
				s.addLocalUserIP(userID, ip, false)
			}
			return result == 1, true
		}
		s.noteRedisFailure(err)
	}
	return false, false
}

func (s *IPSecurityService) seedUserIPs(ctx context.Context, userID int64, ips []string, saturated bool) error {
	local := make(map[string]struct{}, len(ips))
	for _, ip := range ips {
		if normalized := normalizePublicIP(ip); normalized != "" {
			local[normalized] = struct{}{}
		}
	}
	s.userMu.Lock()
	s.userIPs[userID] = userIPLocalState{ips: local, saturated: saturated}
	s.userMu.Unlock()
	if s.rdb == nil {
		return nil
	}
	redisCtx, cancel, ok := s.redisContext(ctx)
	if !ok {
		return nil
	}
	defer cancel()
	marker, set := userIPCacheKeys(userID)
	pipe := s.rdb.TxPipeline()
	if len(ips) > 0 {
		members := make([]any, 0, len(ips))
		for _, ip := range ips {
			if normalized := normalizePublicIP(ip); normalized != "" {
				members = append(members, normalized)
			}
		}
		if len(members) > 0 {
			pipe.SAdd(redisCtx, set, members...)
		}
	}
	markerValue := "1"
	if saturated {
		markerValue = "saturated"
	}
	pipe.Set(redisCtx, marker, markerValue, userIPCacheTTL)
	pipe.Expire(redisCtx, set, userIPCacheTTL)
	_, err := pipe.Exec(redisCtx)
	if err != nil {
		s.noteRedisFailure(err)
	}
	return err
}

func (s *IPSecurityService) redisContext(parent context.Context) (context.Context, context.CancelFunc, bool) {
	if s.rdb == nil || time.Now().UnixNano() < s.redisFailureUntil.Load() {
		return nil, func() {}, false
	}
	ctx, cancel := context.WithTimeout(parent, ipSecurityRedisTimeout)
	return ctx, cancel, true
}

func (s *IPSecurityService) noteRedisFailure(err error) {
	if err == nil {
		return
	}
	s.redisFailureUntil.Store(time.Now().Add(ipSecurityRedisCircuitDuration).UnixNano())
}

func (s *IPSecurityService) setLocalUserState(userID int64, ips []string, saturated bool) {
	state := userIPLocalState{ips: make(map[string]struct{}, len(ips)), saturated: saturated}
	for _, ip := range ips {
		if normalized := normalizePublicIP(ip); normalized != "" {
			state.ips[normalized] = struct{}{}
		}
	}
	s.userMu.Lock()
	s.userIPs[userID] = state
	s.userMu.Unlock()
}

func (s *IPSecurityService) addLocalUserIP(userID int64, ip string, saturated bool) {
	s.userMu.Lock()
	state := s.userIPs[userID]
	if state.ips == nil {
		state.ips = make(map[string]struct{})
	}
	if !state.saturated {
		state.ips[ip] = struct{}{}
	}
	state.saturated = state.saturated || saturated
	s.userIPs[userID] = state
	s.userMu.Unlock()
}

func (s *IPSecurityService) addCachedUserIP(ctx context.Context, userID int64, ip string, saturated bool) error {
	s.addLocalUserIP(userID, ip, saturated)
	s.userMu.RLock()
	state := s.userIPs[userID]
	s.userMu.RUnlock()
	if s.rdb == nil {
		return nil
	}
	redisCtx, cancel, ok := s.redisContext(ctx)
	if !ok {
		return nil
	}
	defer cancel()
	marker, set := userIPCacheKeys(userID)
	pipe := s.rdb.TxPipeline()
	if !state.saturated {
		pipe.SAdd(redisCtx, set, ip)
	}
	pipe.Expire(redisCtx, set, userIPCacheTTL)
	markerValue := "1"
	if state.saturated {
		markerValue = "saturated"
	}
	pipe.Set(redisCtx, marker, markerValue, userIPCacheTTL)
	_, err := pipe.Exec(redisCtx)
	if err != nil {
		s.noteRedisFailure(err)
	}
	return err
}

func (s *IPSecurityService) addNewAccountToWindow(ctx context.Context, ip string, userID int64, now time.Time, windowMinutes int) (int, error) {
	if s.rdb == nil {
		return 0, redis.Nil
	}
	redisCtx, cancel, ok := s.redisContext(ctx)
	if !ok {
		return 0, redis.Nil
	}
	defer cancel()
	window := time.Duration(windowMinutes) * time.Minute
	result, err := newAccountWindowScript.Run(redisCtx, s.rdb, []string{windowKey(ip)},
		now.Add(-window).Unix(), now.Unix(), userID, int((2*window)/time.Second)).Int()
	if err != nil {
		s.noteRedisFailure(err)
	}
	return result, err
}

func (s *IPSecurityService) isWhitelisted(ctx context.Context, ip string) bool {
	return s.resolveIPStatus(ctx, ip) == ipSecurityStatusWhitelisted
}

func (s *IPSecurityService) WarmCache(ctx context.Context) error {
	if s == nil || s.repo == nil {
		return nil
	}
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	active, whitelisted, err := s.repo.LoadIPStatuses(ctx)
	if err != nil {
		return err
	}
	s.fallback = make(map[string]struct{}, len(active))
	s.whitelist = make(map[string]struct{}, len(whitelisted))
	for _, ip := range active {
		s.fallback[ip] = struct{}{}
	}
	for _, ip := range whitelisted {
		s.whitelist[ip] = struct{}{}
	}
	if s.rdb == nil {
		return nil
	}
	redisCtx, cancel, ok := s.redisContext(ctx)
	if !ok {
		return nil
	}
	defer cancel()
	pipe := s.rdb.TxPipeline()
	pipe.Del(redisCtx, "ipsec:{state}:banned", "ipsec:{state}:whitelisted")
	if len(active) > 0 {
		pipe.SAdd(redisCtx, "ipsec:{state}:banned", stringsToAny(active)...)
	}
	if len(whitelisted) > 0 {
		pipe.SAdd(redisCtx, "ipsec:{state}:whitelisted", stringsToAny(whitelisted)...)
	}
	pipe.Set(redisCtx, "ipsec:{state}:loaded", "1", 0)
	_, err = pipe.Exec(redisCtx)
	if err != nil {
		s.noteRedisFailure(err)
	}
	return err
}

func (s *IPSecurityService) WhitelistBan(ctx context.Context, id, releasedBy int64) error {
	ban, err := s.repo.WhitelistBan(ctx, id, releasedBy, time.Now())
	if err != nil {
		return err
	}
	s.stateMu.Lock()
	if s.fallback != nil {
		delete(s.fallback, ban.IPAddress)
	}
	if s.whitelist == nil {
		s.whitelist = make(map[string]struct{})
	}
	s.whitelist[ban.IPAddress] = struct{}{}
	s.stateMu.Unlock()
	if s.rdb != nil {
		if redisCtx, cancel, ok := s.redisContext(ctx); ok {
			defer cancel()
			pipe := s.rdb.TxPipeline()
			pipe.SRem(redisCtx, "ipsec:{state}:banned", ban.IPAddress)
			pipe.SAdd(redisCtx, "ipsec:{state}:whitelisted", ban.IPAddress)
			pipe.Del(redisCtx, windowKey(ban.IPAddress))
			if _, err := pipe.Exec(redisCtx); err != nil {
				s.noteRedisFailure(err)
			}
		}
	}
	return nil
}

func (s *IPSecurityService) RemoveWhitelist(ctx context.Context, id, removedBy int64) error {
	ban, err := s.repo.RemoveWhitelist(ctx, id, removedBy, time.Now())
	if err != nil {
		return err
	}
	s.stateMu.Lock()
	if s.whitelist != nil {
		delete(s.whitelist, ban.IPAddress)
	}
	s.stateMu.Unlock()
	if s.rdb != nil {
		if redisCtx, cancel, ok := s.redisContext(ctx); ok {
			defer cancel()
			if err := s.rdb.SRem(redisCtx, "ipsec:{state}:whitelisted", ban.IPAddress).Err(); err != nil {
				s.noteRedisFailure(err)
			}
		}
	}
	return nil
}

func (s *IPSecurityService) ListBans(ctx context.Context, status string, page, pageSize int) ([]IPSecurityBan, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return s.repo.ListBans(ctx, status, pageSize, (page-1)*pageSize)
}

func (s *IPSecurityService) GetBan(ctx context.Context, id int64) (*IPSecurityBan, error) {
	return s.repo.GetBan(ctx, id)
}

func (s *IPSecurityService) ListActivity(ctx context.Context, ip string, since, until time.Time) ([]IPSecurityActivityDetail, error) {
	return s.repo.ListActivity(ctx, normalizePublicIP(ip), since, until)
}

func (s *IPSecurityService) ListActivityPage(ctx context.Context, ip string, since, until time.Time, page, pageSize int) ([]IPSecurityActivityDetail, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	items, err := s.ListActivity(ctx, ip, since, until)
	if err != nil {
		return nil, 0, err
	}
	total := int64(len(items))
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []IPSecurityActivityDetail{}, total, nil
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], total, nil
}

func userIPCacheKeys(userID int64) (string, string) {
	tag := "{" + strconv.FormatInt(userID, 10) + "}"
	return "ipsec:user:" + tag + ":loaded", "ipsec:user:" + tag + ":ips"
}

func windowKey(ip string) string { return "ipsec:window:{" + ip + "}" }

func containsIP(ips []string, target string) bool {
	for _, ip := range ips {
		if normalizePublicIP(ip) == target {
			return true
		}
	}
	return false
}

func stringsToAny(values []string) []any {
	result := make([]any, len(values))
	for i := range values {
		result[i] = values[i]
	}
	return result
}

func normalizeAnyIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}
	ip := net.ParseIP(raw)
	if ip == nil {
		return ""
	}
	return ip.String()
}

func isCGNAT(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	// 100.64.0.0/10
	return ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127
}

func normalizePublicIP(raw string) string {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || isCGNAT(ip) {
		return ""
	}
	return ip.String()
}
