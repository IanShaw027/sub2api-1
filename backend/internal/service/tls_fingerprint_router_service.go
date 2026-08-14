package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// TLSFingerprintRouterRepository 定义 TLS 指纹路由的数据访问接口。
type TLSFingerprintRouterRepository interface {
	List(ctx context.Context) ([]*model.TLSFingerprintRouter, error)
	GetByID(ctx context.Context, id int64) (*model.TLSFingerprintRouter, error)
	Create(ctx context.Context, router *model.TLSFingerprintRouter) (*model.TLSFingerprintRouter, error)
	Update(ctx context.Context, router *model.TLSFingerprintRouter) (*model.TLSFingerprintRouter, error)
	Delete(ctx context.Context, id int64) error
}

// TLSFingerprintRouterCache 定义 TLS 指纹路由的缓存接口。
type TLSFingerprintRouterCache interface {
	Get(ctx context.Context) ([]*model.TLSFingerprintRouter, bool)
	Set(ctx context.Context, routers []*model.TLSFingerprintRouter) error
	Invalidate(ctx context.Context) error
	NotifyUpdate(ctx context.Context) error
	SubscribeUpdates(ctx context.Context, handler func())
}

// TLSFingerprintRouterMatchResult 表示一次路由命中。
type TLSFingerprintRouterMatchResult struct {
	ProfileID          int64
	OS                 string
	ClientType         string
	Protocol           string
	UpstreamUserAgent  string
	UpstreamOriginator string
}

type cachedTLSFingerprintRouter struct {
	router  *model.TLSFingerprintRouter
	regexes []*regexp.Regexp
}

// TLSFingerprintRouterService 管理 TLS 指纹路由规则。
type TLSFingerprintRouterService struct {
	repo  TLSFingerprintRouterRepository
	cache TLSFingerprintRouterCache

	localCache map[int64]*cachedTLSFingerprintRouter
	localReady bool
	localMu    sync.RWMutex
}

// NewTLSFingerprintRouterService 创建 TLS 指纹路由服务。
func NewTLSFingerprintRouterService(
	repo TLSFingerprintRouterRepository,
	cache TLSFingerprintRouterCache,
) *TLSFingerprintRouterService {
	svc := &TLSFingerprintRouterService{
		repo:       repo,
		cache:      cache,
		localCache: make(map[int64]*cachedTLSFingerprintRouter),
	}

	ctx := context.Background()
	if err := svc.reloadFromDB(ctx); err != nil {
		logger.LegacyPrintf("service.tls_fp_router", "[TLSFPRouterService] Failed to load routers from DB on startup: %v", err)
		if fallbackErr := svc.refreshLocalCache(ctx); fallbackErr != nil {
			logger.LegacyPrintf("service.tls_fp_router", "[TLSFPRouterService] Failed to load routers from cache fallback on startup: %v", fallbackErr)
		}
	}

	if cache != nil {
		cache.SubscribeUpdates(ctx, func() {
			if err := svc.refreshLocalCache(context.Background()); err != nil {
				logger.LegacyPrintf("service.tls_fp_router", "[TLSFPRouterService] Failed to refresh cache on notification: %v", err)
			}
		})
	}

	return svc
}

// List 获取所有路由。
func (s *TLSFingerprintRouterService) List(ctx context.Context) ([]*model.TLSFingerprintRouter, error) {
	return s.repo.List(ctx)
}

// GetByID 根据 ID 获取路由。
func (s *TLSFingerprintRouterService) GetByID(ctx context.Context, id int64) (*model.TLSFingerprintRouter, error) {
	return s.repo.GetByID(ctx, id)
}

// Create 创建路由。
func (s *TLSFingerprintRouterService) Create(ctx context.Context, router *model.TLSFingerprintRouter) (*model.TLSFingerprintRouter, error) {
	if router == nil {
		return nil, errors.New("tls fingerprint router is required")
	}
	if err := router.Validate(); err != nil {
		return nil, err
	}
	created, err := s.repo.Create(ctx, router)
	if err != nil {
		return nil, err
	}
	refreshCtx, cancel := s.newCacheRefreshContext()
	defer cancel()
	s.invalidateAndNotify(refreshCtx)
	return created, nil
}

// Update 更新路由。
func (s *TLSFingerprintRouterService) Update(ctx context.Context, router *model.TLSFingerprintRouter) (*model.TLSFingerprintRouter, error) {
	if router == nil {
		return nil, errors.New("tls fingerprint router is required")
	}
	if err := router.Validate(); err != nil {
		return nil, err
	}
	updated, err := s.repo.Update(ctx, router)
	if err != nil {
		return nil, err
	}
	refreshCtx, cancel := s.newCacheRefreshContext()
	defer cancel()
	s.invalidateAndNotify(refreshCtx)
	return updated, nil
}

// Delete 删除路由。
func (s *TLSFingerprintRouterService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.removeFromLocalCache(id)
	refreshCtx, cancel := s.newCacheRefreshContext()
	defer cancel()
	s.invalidateAndNotify(refreshCtx)
	return nil
}

// MatchRequest 按 first-match-wins 匹配入站请求。规则内条件为 AND。
func (s *TLSFingerprintRouterService) MatchRequest(ctx context.Context, routerID int64, platform, userAgent, transport, protocol string) (TLSFingerprintRouterMatchResult, bool) {
	if s == nil || routerID <= 0 {
		return TLSFingerprintRouterMatchResult{}, false
	}
	cached, ready := s.cachedRouter(routerID)
	if cached == nil && !ready {
		if err := s.refreshLocalCache(ctx); err != nil {
			logger.LegacyPrintf("service.tls_fp_router", "[TLSFPRouterService] Failed to refresh cache during match: %v", err)
		}
		cached, _ = s.cachedRouter(routerID)
	}
	if cached == nil || cached.router == nil || !cached.router.Enabled {
		return TLSFingerprintRouterMatchResult{}, false
	}
	inferredOS := inferTLSFingerprintOS(userAgent)
	inferredClient := inferTLSFingerprintClientType(platform, userAgent, "")
	for i, rule := range cached.router.Rules {
		var compiled *regexp.Regexp
		if i < len(cached.regexes) {
			compiled = cached.regexes[i]
		}
		if !tlsFingerprintRouterRuleMatches(rule, userAgent, transport, protocol, inferredOS, inferredClient, compiled) {
			continue
		}
		return TLSFingerprintRouterMatchResult{
			ProfileID:          rule.TLSFingerprintProfileID,
			OS:                 firstNonEmpty(strings.ToLower(strings.TrimSpace(rule.OS)), inferredOS),
			ClientType:         firstNonEmpty(strings.ToLower(strings.TrimSpace(rule.ClientType)), inferredClient),
			Protocol:           strings.ToLower(strings.TrimSpace(rule.Protocol)),
			UpstreamUserAgent:  strings.TrimSpace(rule.UpstreamUserAgent),
			UpstreamOriginator: strings.TrimSpace(rule.UpstreamOriginator),
		}, true
	}
	return TLSFingerprintRouterMatchResult{}, false
}

func (s *TLSFingerprintRouterService) cachedRouter(routerID int64) (*cachedTLSFingerprintRouter, bool) {
	s.localMu.RLock()
	defer s.localMu.RUnlock()
	return s.localCache[routerID], s.localReady
}

func tlsFingerprintRouterRuleMatches(rule model.TLSFingerprintRouterRule, userAgent, transport, protocol, inferredOS, inferredClient string, compiled *regexp.Regexp) bool {
	if !rule.Enabled {
		return false
	}
	if ruleOS := strings.ToLower(strings.TrimSpace(rule.OS)); ruleOS != "" && ruleOS != inferredOS {
		return false
	}
	if ruleClient := strings.ToLower(strings.TrimSpace(rule.ClientType)); ruleClient != "" && ruleClient != inferredClient {
		return false
	}
	if ruleProtocol := strings.ToLower(strings.TrimSpace(rule.Protocol)); ruleProtocol != "" && ruleProtocol != strings.ToLower(strings.TrimSpace(protocol)) {
		return false
	}
	if !tlsFingerprintRouterRuleTransportAllowed(rule, transport) {
		return false
	}
	if strings.TrimSpace(rule.Pattern) != "" && !tlsFingerprintRouterRuleUAMatches(rule, userAgent, compiled) {
		return false
	}
	return true
}

func tlsFingerprintRouterRuleUAMatches(rule model.TLSFingerprintRouterRule, userAgent string, compiled *regexp.Regexp) bool {
	pattern := strings.TrimSpace(rule.Pattern)
	if pattern == "" {
		return true
	}
	ua := userAgent
	if !rule.CaseSensitive {
		ua = strings.ToLower(ua)
		pattern = strings.ToLower(pattern)
	}
	switch strings.TrimSpace(rule.MatchType) {
	case "", model.TLSFingerprintRouterMatchContains:
		return strings.Contains(ua, pattern)
	case model.TLSFingerprintRouterMatchPrefix:
		return strings.HasPrefix(ua, pattern)
	case model.TLSFingerprintRouterMatchExact:
		return ua == pattern
	case model.TLSFingerprintRouterMatchRegex:
		if compiled != nil {
			return compiled.MatchString(userAgent)
		}
		rePattern := strings.TrimSpace(rule.Pattern)
		if !rule.CaseSensitive {
			rePattern = "(?i)" + rePattern
		}
		re, err := regexp.Compile(rePattern)
		return err == nil && re.MatchString(userAgent)
	default:
		return false
	}
}

func tlsFingerprintRouterRuleTransportAllowed(rule model.TLSFingerprintRouterRule, transport string) bool {
	ruleTransport := strings.ToLower(strings.TrimSpace(rule.Transport))
	transport = strings.ToLower(strings.TrimSpace(transport))
	if ruleTransport == "" || transport == "" {
		return true
	}
	switch transport {
	case model.TLSFingerprintRouterTransportHTTP:
		return ruleTransport == model.TLSFingerprintRouterTransportHTTP ||
			ruleTransport == model.TLSFingerprintRouterTransportHTTP1 ||
			ruleTransport == model.TLSFingerprintRouterTransportH2
	case model.TLSFingerprintRouterTransportWebSocket:
		return ruleTransport == model.TLSFingerprintRouterTransportWebSocket ||
			ruleTransport == model.TLSFingerprintRouterTransportWSHTTP1 ||
			ruleTransport == model.TLSFingerprintRouterTransportWSH2
	}
	return ruleTransport == transport
}

func (s *TLSFingerprintRouterService) refreshLocalCache(ctx context.Context) error {
	if s.cache != nil {
		if routers, ok := s.cache.Get(ctx); ok {
			s.setLocalCache(routers)
			return nil
		}
	}
	return s.reloadFromDB(ctx)
}

func (s *TLSFingerprintRouterService) reloadFromDB(ctx context.Context) error {
	if s.repo == nil {
		s.setLocalCache(nil)
		return nil
	}
	routers, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	s.setLocalCache(routers)
	if s.cache != nil {
		if err := s.cache.Set(ctx, routers); err != nil {
			logger.LegacyPrintf("service.tls_fp_router", "[TLSFPRouterService] Failed to write cache: %v", err)
		}
	}
	return nil
}

func (s *TLSFingerprintRouterService) setLocalCache(routers []*model.TLSFingerprintRouter) {
	m := make(map[int64]*cachedTLSFingerprintRouter, len(routers))
	for _, router := range routers {
		if router == nil {
			continue
		}
		cached := &cachedTLSFingerprintRouter{
			router:  router,
			regexes: make([]*regexp.Regexp, len(router.Rules)),
		}
		for i, rule := range router.Rules {
			cached.regexes[i] = compileTLSFingerprintRouterRuleRegex(rule)
		}
		m[router.ID] = cached
	}
	s.localMu.Lock()
	s.localCache = m
	s.localReady = true
	s.localMu.Unlock()
}

func (s *TLSFingerprintRouterService) removeFromLocalCache(id int64) {
	s.localMu.Lock()
	if s.localCache != nil {
		delete(s.localCache, id)
	}
	s.localReady = true
	s.localMu.Unlock()
}

func compileTLSFingerprintRouterRuleRegex(rule model.TLSFingerprintRouterRule) *regexp.Regexp {
	if strings.TrimSpace(rule.MatchType) != model.TLSFingerprintRouterMatchRegex {
		return nil
	}
	pattern := strings.TrimSpace(rule.Pattern)
	if pattern == "" {
		return nil
	}
	if !rule.CaseSensitive {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	return re
}

func (s *TLSFingerprintRouterService) newCacheRefreshContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 3*time.Second)
}

func (s *TLSFingerprintRouterService) invalidateAndNotify(ctx context.Context) {
	if s.cache != nil {
		if err := s.cache.Invalidate(ctx); err != nil {
			logger.LegacyPrintf("service.tls_fp_router", "[TLSFPRouterService] Failed to invalidate cache: %v", err)
		}
	}
	if err := s.reloadFromDB(ctx); err != nil {
		logger.LegacyPrintf("service.tls_fp_router", "[TLSFPRouterService] Failed to refresh local cache: %v", err)
	}
	if s.cache != nil {
		if err := s.cache.NotifyUpdate(ctx); err != nil {
			logger.LegacyPrintf("service.tls_fp_router", "[TLSFPRouterService] Failed to notify cache update: %v", err)
		}
	}
}
