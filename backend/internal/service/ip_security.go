package service

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	IPSecuritySourceWeb             = "web"
	IPSecuritySourceAPIKey          = "apikey"
	defaultIPSecurityWindowMinutes  = 10
	defaultIPSecurityThreshold      = 4
	defaultIPSecurityWindow2Minutes = 0
	defaultIPSecurityThreshold2     = 2
	maxIPSecurityWindow2Minutes     = 10080
	userIPCacheTTL                  = 365 * 24 * time.Hour
	ipSecurityConfigCacheTTL        = 5 * time.Minute
	maxUserIPHistory                = 256
	ipSecurityRedisTimeout          = 20 * time.Millisecond
	ipSecurityRedisCircuitDuration  = 5 * time.Second
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
	Enabled                     bool      `json:"enabled"`
	WindowMinutes               int       `json:"window_minutes"`
	AccountThreshold            int       `json:"account_threshold"`
	Window2Minutes              int       `json:"window2_minutes"`
	AccountThreshold2           int       `json:"account_threshold2"`
	BlockDatacenterRegistration bool      `json:"block_datacenter_registration"`
	LearningUntil               time.Time `json:"learning_until"`
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

type UserIPPinState struct {
	Pinned    bool       `json:"pinned"`
	EnabledAt *time.Time `json:"enabled_at,omitempty"`
	IPs       []string   `json:"ips,omitempty"`
	Saturated bool       `json:"saturated"`
}

type UserIPSummaryItem struct {
	IPAddress    string             `json:"ip_address"`
	RequestCount int64              `json:"request_count"`
	TotalCost    float64            `json:"total_cost"`
	FirstSeenAt  time.Time          `json:"first_seen_at"`
	LastSeenAt   time.Time          `json:"last_seen_at"`
	IsTop        bool               `json:"is_top"`
	BanStatus    string             `json:"ban_status"` // normal|active|whitelisted|released
	BanReason    string             `json:"ban_reason,omitempty"`
	BanID        int64              `json:"ban_id,omitempty"`
	SharedUsers  []UserIPSharedUser `json:"shared_users"`
}

type UserIPSharedUser struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	RequestCount int64  `json:"request_count"`
}

type UserIPSummary struct {
	UserID      int64               `json:"user_id"`
	PinKnownIPs bool                `json:"pin_known_ips"`
	TopIP       string              `json:"top_ip,omitempty"`
	Items       []UserIPSummaryItem `json:"items"`
}

type IPSecurityRepository interface {
	GetUserIPs(ctx context.Context, userID int64) ([]string, bool, error)
	GetUserIPPinState(ctx context.Context, userID int64) (*UserIPPinState, error)
	SetUserIPPin(ctx context.Context, userID int64, pinned bool, ips []string, enabledAt *time.Time) error
	AppendUserAllowedIP(ctx context.Context, userID int64, ip string) error
	ListUsageIPsForUser(ctx context.Context, userID int64, since time.Time) ([]string, error)
	ListUserIPSummary(ctx context.Context, userID int64, since time.Time) (*UserIPSummary, error)
	AddUserIPActivity(ctx context.Context, activity IPSecurityActivity, now time.Time) (added, saturated bool, err error)
	CreateBan(ctx context.Context, ban *IPSecurityBan) (bool, error)
	ForceCreateBan(ctx context.Context, ban *IPSecurityBan) (bool, error)
	IsIPStatus(ctx context.Context, ip, status string) (bool, error)
	LoadIPStatuses(ctx context.Context) (active, whitelisted []string, err error)
	ListBans(ctx context.Context, status string, limit, offset int) ([]IPSecurityBan, int64, error)
	GetBan(ctx context.Context, id int64) (*IPSecurityBan, error)
	GetBanByIP(ctx context.Context, ip string) (*IPSecurityBan, error)
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
	pinMu             sync.RWMutex
	pinStates         map[int64]cachedUserIPPinState
	redisFailureUntil atomic.Int64
}

// Keep policy propagation bounded across service instances while avoiding a
// database round trip on every authenticated request.
const userIPPinCacheTTL = 5 * time.Second

type cachedUserIPPinState struct {
	state   *UserIPPinState
	expires time.Time
}

var globalIPSecurityService atomic.Pointer[IPSecurityService]

func SetGlobalIPSecurityService(svc *IPSecurityService) { globalIPSecurityService.Store(svc) }
func GlobalIPSecurityService() *IPSecurityService       { return globalIPSecurityService.Load() }

func NewIPSecurityService(repo IPSecurityRepository, settings IPSecuritySettings, rdb *redis.Client) *IPSecurityService {
	return &IPSecurityService{repo: repo, settings: settings, rdb: rdb,
		userIPs: make(map[int64]userIPLocalState), pinStates: make(map[int64]cachedUserIPPinState)}
}

func DefaultIPSecurityConfig() IPSecurityConfig {
	return IPSecurityConfig{
		WindowMinutes:     defaultIPSecurityWindowMinutes,
		AccountThreshold:  defaultIPSecurityThreshold,
		Window2Minutes:    defaultIPSecurityWindow2Minutes,
		AccountThreshold2: defaultIPSecurityThreshold2,
	}
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
	if cfg.Window2Minutes < 0 {
		cfg.Window2Minutes = 0
	}
	if cfg.Window2Minutes > maxIPSecurityWindow2Minutes {
		cfg.Window2Minutes = maxIPSecurityWindow2Minutes
	}
	if cfg.Window2Minutes > 0 {
		if cfg.AccountThreshold2 < 2 {
			cfg.AccountThreshold2 = 2
		}
		if cfg.AccountThreshold2 > 100 {
			cfg.AccountThreshold2 = 100
		}
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

func readEnabledUnlessFalse(ctx context.Context, settings IPSecuritySettings, key string, fallback bool) bool {
	if settings == nil {
		return fallback
	}
	raw, err := settings.GetValue(ctx, key)
	if err != nil {
		return fallback
	}
	return settingEnabledUnlessFalse(raw, fallback)
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
	cfg.Window2Minutes = readIntSetting(ctx, s.settings, SettingKeyIPMultiAccountBanWindow2Minutes, cfg.Window2Minutes)
	cfg.AccountThreshold2 = readIntSetting(ctx, s.settings, SettingKeyIPMultiAccountBanThreshold2, cfg.AccountThreshold2)
	cfg.BlockDatacenterRegistration = readEnabledUnlessFalse(ctx, s.settings, SettingKeyRegistrationBlockDatacenterIP, false)
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
	count, err := s.addNewAccountToWindow(ctx, windowKey(activity.IPAddress), activity.UserID, now, cfg.WindowMinutes)
	hit := err == nil && count >= cfg.AccountThreshold
	windowMinutes, threshold := cfg.WindowMinutes, cfg.AccountThreshold
	reason := "short-window multi-account activity"
	if cfg.Window2Minutes > 0 {
		count2, err2 := s.addNewAccountToWindow(ctx, window2Key(activity.IPAddress), activity.UserID, now, cfg.Window2Minutes)
		if err2 == nil && count2 >= cfg.AccountThreshold2 {
			if !hit || cfg.Window2Minutes >= windowMinutes {
				count = count2
				windowMinutes = cfg.Window2Minutes
				threshold = cfg.AccountThreshold2
				reason = "long-window multi-account activity"
			}
			hit = true
		}
	}
	if !hit {
		return
	}
	s.enforceBan(ctx, activity.IPAddress, count, threshold, windowMinutes, reason, now)
}

func (s *IPSecurityService) enforceBan(ctx context.Context, ip string, count, threshold, windowMinutes int, reason string, now time.Time) {
	if s.IsBlocked(ctx, ip) {
		return
	}
	firstSeen, lastSeen := now.Add(-time.Duration(windowMinutes)*time.Minute), now
	ban := &IPSecurityBan{
		IPAddress: ip, Status: "active", Reason: reason,
		AccountThreshold: threshold, WindowMinutes: windowMinutes,
		DetectedAccountCount: count, FirstSeenAt: firstSeen, LastSeenAt: lastSeen, CreatedAt: now,
	}
	created, err := s.repo.CreateBan(ctx, ban)
	if err != nil || !created {
		return
	}
	s.applyActiveBanLocalAndRedis(ctx, ip)
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

func (s *IPSecurityService) addNewAccountToWindow(ctx context.Context, key string, userID int64, now time.Time, windowMinutes int) (int, error) {
	if s.rdb == nil || strings.TrimSpace(key) == "" || windowMinutes < 1 {
		return 0, redis.Nil
	}
	redisCtx, cancel, ok := s.redisContext(ctx)
	if !ok {
		return 0, redis.Nil
	}
	defer cancel()
	window := time.Duration(windowMinutes) * time.Minute
	result, err := newAccountWindowScript.Run(redisCtx, s.rdb, []string{key},
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

// EnforcePinnedUserIP rejects requests from unknown IPs when the user has
// pin_known_ips enabled, and globally bans that IP. Returns true when the
// caller should abort the request with IP_NOT_ALLOWED.
func (s *IPSecurityService) EnforcePinnedUserIP(ctx context.Context, userID int64, rawIP string) (blocked bool, clientIP string, err error) {
	if s == nil || userID <= 0 {
		return false, "", nil
	}
	clientIP = normalizePublicIP(rawIP)
	if clientIP == "" {
		// Non-public / unparseable IPs cannot be pinned-checked reliably.
		return false, "", nil
	}
	state, err := s.getUserIPPinState(ctx, userID)
	if err != nil {
		return false, clientIP, err
	}
	if state == nil || !state.Pinned {
		return false, clientIP, nil
	}
	if containsIP(state.IPs, clientIP) {
		return false, clientIP, nil
	}
	reason := fmt.Sprintf("pinned-user unknown ip (user #%d)", userID)
	ban := &IPSecurityBan{
		IPAddress: clientIP, Status: "active", Reason: reason,
		AccountThreshold: 1, DetectedAccountCount: 1,
		FirstSeenAt: time.Now(), LastSeenAt: time.Now(), CreatedAt: time.Now(),
	}
	// A user-level pin rejection must not override an administrator whitelist.
	// The request is still denied, but the shared IP remains whitelisted.
	created, banErr := s.repo.CreateBan(ctx, ban)
	if banErr != nil {
		// Still block the request even if ban persistence races.
		return true, clientIP, banErr
	}
	if created {
		s.applyActiveBanLocalAndRedis(ctx, clientIP)
	}
	return true, clientIP, nil
}

func (s *IPSecurityService) getUserIPPinState(ctx context.Context, userID int64) (*UserIPPinState, error) {
	now := time.Now()
	s.pinMu.RLock()
	cached, ok := s.pinStates[userID]
	s.pinMu.RUnlock()
	if ok && cached.state != nil && now.Before(cached.expires) {
		return cloneUserIPPinState(cached.state), nil
	}
	state, err := s.repo.GetUserIPPinState(ctx, userID)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
	s.pinMu.Lock()
	s.pinStates[userID] = cachedUserIPPinState{state: cloneUserIPPinState(state), expires: now.Add(userIPPinCacheTTL)}
	s.pinMu.Unlock()
	return state, nil
}

func cloneUserIPPinState(state *UserIPPinState) *UserIPPinState {
	if state == nil {
		return nil
	}
	clone := *state
	clone.IPs = append([]string(nil), state.IPs...)
	return &clone
}

func (s *IPSecurityService) invalidateUserIPPinState(userID int64) {
	if s == nil {
		return
	}
	s.pinMu.Lock()
	delete(s.pinStates, userID)
	s.pinMu.Unlock()
}

func (s *IPSecurityService) SetPinKnownIPs(ctx context.Context, userID int64, enabled bool) (*UserIPPinState, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user id")
	}
	state, err := s.getUserIPPinState(ctx, userID)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, fmt.Errorf("user not found")
	}
	if !enabled {
		if err := s.repo.SetUserIPPin(ctx, userID, false, state.IPs, nil); err != nil {
			return nil, err
		}
		s.invalidateUserIPPinState(userID)
		_ = s.seedUserIPs(ctx, userID, state.IPs, state.Saturated)
		out, err := s.getUserIPPinState(ctx, userID)
		return out, err
	}

	merged := make(map[string]struct{}, len(state.IPs)+16)
	ips := make([]string, 0, len(state.IPs)+16)
	for _, ipAddr := range state.IPs {
		if normalized := normalizePublicIP(ipAddr); normalized != "" {
			if _, exists := merged[normalized]; !exists {
				merged[normalized] = struct{}{}
				ips = append(ips, normalized)
			}
		}
	}
	usageIPs, err := s.repo.ListUsageIPsForUser(ctx, userID, time.Now().Add(-90*24*time.Hour))
	if err != nil {
		return nil, err
	}
	for _, ipAddr := range usageIPs {
		if normalized := normalizePublicIP(ipAddr); normalized != "" {
			if _, exists := merged[normalized]; !exists {
				merged[normalized] = struct{}{}
				ips = append(ips, normalized)
			}
		}
	}
	if len(ips) > maxUserIPHistory {
		ips = ips[:maxUserIPHistory]
	}
	now := time.Now()
	if err := s.repo.SetUserIPPin(ctx, userID, true, ips, &now); err != nil {
		return nil, err
	}
	s.invalidateUserIPPinState(userID)
	_ = s.seedUserIPs(ctx, userID, ips, len(ips) >= maxUserIPHistory)
	return s.getUserIPPinState(ctx, userID)
}

func (s *IPSecurityService) AppendAllowedIP(ctx context.Context, userID int64, rawIP string) error {
	ipAddr := normalizePublicIP(rawIP)
	if ipAddr == "" {
		return fmt.Errorf("invalid public ip address")
	}
	if err := s.repo.AppendUserAllowedIP(ctx, userID, ipAddr); err != nil {
		return err
	}
	s.invalidateUserIPPinState(userID)
	state, err := s.getUserIPPinState(ctx, userID)
	if err == nil && state != nil {
		_ = s.seedUserIPs(ctx, userID, state.IPs, state.Saturated)
	}
	return nil
}

func (s *IPSecurityService) GetUserIPSummary(ctx context.Context, userID int64, days int) (*UserIPSummary, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user id")
	}
	if days <= 0 {
		days = 30
	}
	if days > 90 {
		days = 90
	}
	return s.repo.ListUserIPSummary(ctx, userID, time.Now().Add(-time.Duration(days)*24*time.Hour))
}

// ManualBan creates or re-activates a global IP ban (overrides whitelist).
// reason should be a stable prefix such as "manual: ..." or "pinned-user unknown ip ...".
func (s *IPSecurityService) ManualBan(ctx context.Context, rawIP, reason string) (*IPSecurityBan, error) {
	ip := normalizePublicIP(rawIP)
	if ip == "" {
		return nil, fmt.Errorf("invalid public ip address")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "manual: admin ban"
	}
	now := time.Now()
	ban := &IPSecurityBan{
		IPAddress:            ip,
		Status:               "active",
		Reason:               reason,
		AccountThreshold:     1,
		WindowMinutes:        0,
		DetectedAccountCount: 1,
		FirstSeenAt:          now,
		LastSeenAt:           now,
		CreatedAt:            now,
	}
	created, err := s.repo.ForceCreateBan(ctx, ban)
	if err != nil {
		return nil, err
	}
	if !created {
		return nil, fmt.Errorf("failed to create ip ban")
	}
	s.applyActiveBanLocalAndRedis(ctx, ip)
	return ban, nil
}

func (s *IPSecurityService) applyActiveBanLocalAndRedis(ctx context.Context, ip string) {
	s.stateMu.Lock()
	if s.fallback == nil {
		s.fallback = make(map[string]struct{})
	}
	s.fallback[ip] = struct{}{}
	if s.whitelist != nil {
		delete(s.whitelist, ip)
	}
	s.stateMu.Unlock()
	if s.rdb == nil {
		return
	}
	if redisCtx, cancel, ok := s.redisContext(ctx); ok {
		defer cancel()
		pipe := s.rdb.TxPipeline()
		pipe.SAdd(redisCtx, "ipsec:{state}:banned", ip)
		pipe.SRem(redisCtx, "ipsec:{state}:whitelisted", ip)
		if _, err := pipe.Exec(redisCtx); err != nil {
			s.noteRedisFailure(err)
		}
	}
}

func (s *IPSecurityService) GetBanByIP(ctx context.Context, rawIP string) (*IPSecurityBan, error) {
	ip := normalizePublicIP(rawIP)
	if ip == "" {
		return nil, fmt.Errorf("invalid public ip address")
	}
	return s.repo.GetBanByIP(ctx, ip)
}

func (s *IPSecurityService) WhitelistBanByIP(ctx context.Context, rawIP string, releasedBy int64) error {
	ban, err := s.GetBanByIP(ctx, rawIP)
	if err != nil {
		return err
	}
	if ban.Status == "whitelisted" {
		return nil
	}
	if ban.Status == "released" {
		// Releasing an already released ban is idempotent. Do not reactivate it
		// briefly just to whitelist it again.
		return nil
	}
	if ban.Status != "active" {
		if _, err := s.ManualBan(ctx, ban.IPAddress, ban.Reason); err != nil {
			return err
		}
		ban, err = s.GetBanByIP(ctx, ban.IPAddress)
		if err != nil {
			return err
		}
	}
	return s.WhitelistBan(ctx, ban.ID, releasedBy)
}

func (s *IPSecurityService) ActivateBanByIP(ctx context.Context, rawIP, reason string) (*IPSecurityBan, error) {
	if strings.TrimSpace(reason) == "" {
		reason = "manual: admin re-activate"
	}
	return s.ManualBan(ctx, rawIP, reason)
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
			pipe.Del(redisCtx, windowKey(ban.IPAddress), window2Key(ban.IPAddress))
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

func windowKey(ip string) string  { return "ipsec:window:{" + ip + "}" }
func window2Key(ip string) string { return "ipsec:window2:{" + ip + "}" }

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
