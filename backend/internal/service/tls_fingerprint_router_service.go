package service

import (
	"context"
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

// TLSFingerprintRouterMatchResult 表示一次 UA 路由命中的结果。
type TLSFingerprintRouterMatchResult struct {
	ProfileID          int64
	UpstreamUserAgent  string
	UpstreamOriginator string
}

// TLSFingerprintRouterService 管理 TLS 指纹路由规则。
type TLSFingerprintRouterService struct {
	repo  TLSFingerprintRouterRepository
	cache TLSFingerprintRouterCache

	localCache map[int64]*model.TLSFingerprintRouter
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
		localCache: make(map[int64]*model.TLSFingerprintRouter),
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
	refreshCtx, cancel := s.newCacheRefreshContext()
	defer cancel()
	s.invalidateAndNotify(refreshCtx)
	return nil
}

// MatchUserAgent 匹配入站 User-Agent，采用 first-match-wins。
func (s *TLSFingerprintRouterService) MatchUserAgent(ctx context.Context, routerID int64, userAgent string) (TLSFingerprintRouterMatchResult, bool) {
	return s.MatchRequest(ctx, routerID, userAgent, "")
}

// MatchRequest 匹配入站请求头，采用 first-match-wins。规则若配置上游
// Originator 覆写，入站 Originator 必须先匹配该值，避免仅凭 UA 触发身份头覆写。
func (s *TLSFingerprintRouterService) MatchRequest(ctx context.Context, routerID int64, userAgent string, originator string) (TLSFingerprintRouterMatchResult, bool) {
	if s == nil || routerID <= 0 {
		return TLSFingerprintRouterMatchResult{}, false
	}
	router := s.routerFromLocalCache(routerID)
	if router == nil {
		if err := s.refreshLocalCache(ctx); err != nil {
			logger.LegacyPrintf("service.tls_fp_router", "[TLSFPRouterService] Failed to refresh cache during match: %v", err)
		}
		router = s.routerFromLocalCache(routerID)
	}
	if router == nil || !router.Enabled {
		return TLSFingerprintRouterMatchResult{}, false
	}
	for _, rule := range router.Rules {
		if !rule.Enabled || !tlsFingerprintRouterRuleMatches(rule, userAgent) {
			continue
		}
		if !tlsFingerprintRouterRuleOriginatorAllowed(rule, originator) {
			continue
		}
		return TLSFingerprintRouterMatchResult{
			ProfileID:          rule.TLSFingerprintProfileID,
			UpstreamUserAgent:  strings.TrimSpace(rule.UpstreamUserAgent),
			UpstreamOriginator: strings.TrimSpace(rule.UpstreamOriginator),
		}, true
	}
	return TLSFingerprintRouterMatchResult{}, false
}

func (s *TLSFingerprintRouterService) routerFromLocalCache(routerID int64) *model.TLSFingerprintRouter {
	s.localMu.RLock()
	router := s.localCache[routerID]
	s.localMu.RUnlock()
	return router
}

func tlsFingerprintRouterRuleMatches(rule model.TLSFingerprintRouterRule, userAgent string) bool {
	pattern := strings.TrimSpace(rule.Pattern)
	if pattern == "" {
		return false
	}
	ua := userAgent
	if !rule.CaseSensitive {
		ua = strings.ToLower(ua)
		pattern = strings.ToLower(pattern)
	}
	switch strings.TrimSpace(rule.MatchType) {
	case model.TLSFingerprintRouterMatchContains:
		return strings.Contains(ua, pattern)
	case model.TLSFingerprintRouterMatchPrefix:
		return strings.HasPrefix(ua, pattern)
	case model.TLSFingerprintRouterMatchExact:
		return ua == pattern
	case model.TLSFingerprintRouterMatchRegex:
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

func tlsFingerprintRouterRuleOriginatorAllowed(rule model.TLSFingerprintRouterRule, originator string) bool {
	upstreamOriginator := strings.TrimSpace(rule.UpstreamOriginator)
	if upstreamOriginator == "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(originator), upstreamOriginator)
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
	if s.cache != nil {
		if err := s.cache.Set(ctx, routers); err != nil {
			logger.LegacyPrintf("service.tls_fp_router", "[TLSFPRouterService] Failed to set cache: %v", err)
		}
	}
	s.setLocalCache(routers)
	return nil
}

func (s *TLSFingerprintRouterService) setLocalCache(routers []*model.TLSFingerprintRouter) {
	m := make(map[int64]*model.TLSFingerprintRouter, len(routers))
	for _, router := range routers {
		if router != nil {
			m[router.ID] = router
		}
	}
	s.localMu.Lock()
	s.localCache = m
	s.localMu.Unlock()
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
		s.localMu.Lock()
		s.localCache = make(map[int64]*model.TLSFingerprintRouter)
		s.localMu.Unlock()
	}
	if s.cache != nil {
		if err := s.cache.NotifyUpdate(ctx); err != nil {
			logger.LegacyPrintf("service.tls_fp_router", "[TLSFPRouterService] Failed to notify cache update: %v", err)
		}
	}
}
