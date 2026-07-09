package service

import (
	"context"
	"errors"
	"math/rand/v2"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

// TLSFingerprintProfileRepository 定义 TLS 指纹模板的数据访问接口
type TLSFingerprintProfileRepository interface {
	List(ctx context.Context) ([]*model.TLSFingerprintProfile, error)
	GetByID(ctx context.Context, id int64) (*model.TLSFingerprintProfile, error)
	Create(ctx context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error)
	Update(ctx context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error)
	Delete(ctx context.Context, id int64) error
}

// TLSFingerprintProfileCache 定义 TLS 指纹模板的缓存接口
type TLSFingerprintProfileCache interface {
	Get(ctx context.Context) ([]*model.TLSFingerprintProfile, bool)
	Set(ctx context.Context, profiles []*model.TLSFingerprintProfile) error
	Invalidate(ctx context.Context) error
	NotifyUpdate(ctx context.Context) error
	SubscribeUpdates(ctx context.Context, handler func())
}

// TLSFingerprintProfileService TLS 指纹模板管理服务
type TLSFingerprintProfileService struct {
	repo  TLSFingerprintProfileRepository
	cache TLSFingerprintProfileCache

	// 本地 ID→Profile 映射缓存，用于 DoWithTLS 热路径快速查找
	localCache map[int64]*model.TLSFingerprintProfile
	localMu    sync.RWMutex
}

// NewTLSFingerprintProfileService 创建 TLS 指纹模板服务
func NewTLSFingerprintProfileService(
	repo TLSFingerprintProfileRepository,
	cache TLSFingerprintProfileCache,
) *TLSFingerprintProfileService {
	svc := &TLSFingerprintProfileService{
		repo:       repo,
		cache:      cache,
		localCache: make(map[int64]*model.TLSFingerprintProfile),
	}

	ctx := context.Background()
	if repo != nil {
		if err := svc.reloadFromDB(ctx); err != nil {
			logger.LegacyPrintf("service.tls_fp_profile", "[TLSFPProfileService] Failed to load profiles from DB on startup: %v", err)
			if fallbackErr := svc.refreshLocalCache(ctx); fallbackErr != nil {
				logger.LegacyPrintf("service.tls_fp_profile", "[TLSFPProfileService] Failed to load profiles from cache fallback on startup: %v", fallbackErr)
			}
		}
	}

	if cache != nil {
		cache.SubscribeUpdates(ctx, func() {
			if err := svc.refreshLocalCache(context.Background()); err != nil {
				logger.LegacyPrintf("service.tls_fp_profile", "[TLSFPProfileService] Failed to refresh cache on notification: %v", err)
			}
		})
	}

	return svc
}

func (s *TLSFingerprintProfileService) requireRepo() (TLSFingerprintProfileRepository, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("tls fingerprint profile repository is unavailable")
	}
	return s.repo, nil
}

// --- CRUD ---

// List 获取所有模板
func (s *TLSFingerprintProfileService) List(ctx context.Context) ([]*model.TLSFingerprintProfile, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}
	return repo.List(ctx)
}

// GetByID 根据 ID 获取模板
func (s *TLSFingerprintProfileService) GetByID(ctx context.Context, id int64) (*model.TLSFingerprintProfile, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}
	return repo.GetByID(ctx, id)
}

// Create 创建模板
func (s *TLSFingerprintProfileService) Create(ctx context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error) {
	if profile == nil {
		return nil, errors.New("tls fingerprint profile is required")
	}
	if h2fp, err := canonicalizeHTTP2Fingerprint(profile.HTTP2Fingerprint); err != nil {
		return nil, err
	} else {
		profile.HTTP2Fingerprint = h2fp
	}
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}

	created, err := repo.Create(ctx, profile)
	if err != nil {
		return nil, err
	}

	refreshCtx, cancel := s.newCacheRefreshContext()
	defer cancel()
	s.invalidateAndNotify(refreshCtx)

	return created, nil
}

// Update 更新模板
func (s *TLSFingerprintProfileService) Update(ctx context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error) {
	if profile == nil {
		return nil, errors.New("tls fingerprint profile is required")
	}
	if h2fp, err := canonicalizeHTTP2Fingerprint(profile.HTTP2Fingerprint); err != nil {
		return nil, err
	} else {
		profile.HTTP2Fingerprint = h2fp
	}
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}

	updated, err := repo.Update(ctx, profile)
	if err != nil {
		return nil, err
	}

	refreshCtx, cancel := s.newCacheRefreshContext()
	defer cancel()
	s.invalidateAndNotify(refreshCtx)

	return updated, nil
}

// Delete 删除模板
func (s *TLSFingerprintProfileService) Delete(ctx context.Context, id int64) error {
	repo, err := s.requireRepo()
	if err != nil {
		return err
	}
	if err := repo.Delete(ctx, id); err != nil {
		return err
	}

	refreshCtx, cancel := s.newCacheRefreshContext()
	defer cancel()
	s.invalidateAndNotify(refreshCtx)

	return nil
}

// --- 热路径：运行时 Profile 查找 ---

// GetProfileByID 根据 ID 从本地缓存获取 Profile（用于 DoWithTLS 热路径）
// 返回 nil 表示未找到或该模板当前运行时不可用，调用方应 fallback 到内置默认 Profile
func (s *TLSFingerprintProfileService) GetProfileByID(id int64) *tlsfingerprint.Profile {
	if p := s.getProfileModelByID(id); p != nil {
		if !tlsFingerprintProfileRuntimeSupported(p) || tlsFingerprintProfileUsesUnsupportedH2Transport(p) {
			return nil
		}
		return p.ToTLSProfile()
	}
	return nil
}

func (s *TLSFingerprintProfileService) getProfileModelByID(id int64) *model.TLSFingerprintProfile {
	if s == nil || id <= 0 {
		return nil
	}
	s.localMu.RLock()
	p, ok := s.localCache[id]
	s.localMu.RUnlock()
	if ok && p != nil {
		return p
	}
	return nil
}

// ResolveTLSProfileByID 根据模板 ID 解析运行时 TLS Profile。
func (s *TLSFingerprintProfileService) ResolveTLSProfileByID(id int64) *tlsfingerprint.Profile {
	if s == nil || id <= 0 {
		return nil
	}
	return s.GetProfileByID(id)
}

func (s *TLSFingerprintProfileService) resolveProfileByIDForAccount(id int64, account *Account, transport string) *tlsfingerprint.Profile {
	p := s.getProfileModelByID(id)
	if p == nil || !tlsFingerprintProfileRuntimeSupported(p) || tlsFingerprintProfileUsesUnsupportedH2Transport(p) || !tlsFingerprintProfileMatchesAccount(p, account, transport) {
		return nil
	}
	return p.ToTLSProfile()
}

// 路由规则显式指定的 ProfileID 是管理员有意的强绑定，解析时只校验平台归属；真正的
// transport 能力收口交给 runtimeSupported/transport 匹配。
func (s *TLSFingerprintProfileService) resolveRouterProfileByIDForAccount(id int64, account *Account) *tlsfingerprint.Profile {
	p := s.getProfileModelByID(id)
	if p == nil || !tlsFingerprintProfileRuntimeSupported(p) {
		return nil
	}
	if account != nil {
		profilePlatform := strings.ToLower(strings.TrimSpace(p.Platform))
		accountPlatform := strings.ToLower(strings.TrimSpace(account.Platform))
		if profilePlatform != "" && profilePlatform != accountPlatform {
			return nil
		}
	}
	if tlsFingerprintProfileUsesUnsupportedH2Transport(p) {
		return builtinDefaultTLSProfile()
	}
	return p.ToTLSProfile()
}

// pickStableProfile 在候选模板中按账号稳定选择一个：先按模板 ID 排序消除 map 遍历的
// 顺序抖动，再用 accountID 取模定位。同一账号恒定选中同一 profile，保证单凭证的
// JA3/JA4 跨请求稳定（不像每请求 rand 现掷会暴露异常指纹轮换）。accountID<=0 时无
// 账号上下文，退化为随机选择。
func pickStableProfile(profiles []*model.TLSFingerprintProfile, accountID int64) *tlsfingerprint.Profile {
	if len(profiles) == 0 {
		return nil
	}
	if accountID <= 0 {
		return profiles[rand.IntN(len(profiles))].ToTLSProfile()
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })
	idx := int(uint64(accountID) % uint64(len(profiles)))
	return profiles[idx].ToTLSProfile()
}

func (s *TLSFingerprintProfileService) getRandomProfileForPlatformTransport(platform, transport string, accountID int64) *tlsfingerprint.Profile {
	if s == nil {
		return nil
	}
	s.localMu.RLock()
	defer s.localMu.RUnlock()

	if len(s.localCache) == 0 {
		return nil
	}

	normalizedPlatform := strings.ToLower(strings.TrimSpace(platform))
	profiles := make([]*model.TLSFingerprintProfile, 0, len(s.localCache))
	for _, p := range s.localCache {
		if p == nil {
			continue
		}
		if !tlsFingerprintProfileRuntimeSupported(p) {
			continue
		}
		if tlsFingerprintProfileUsesUnsupportedH2Transport(p) {
			continue
		}
		profilePlatform := strings.ToLower(strings.TrimSpace(p.Platform))
		if profilePlatform != "" && profilePlatform != normalizedPlatform {
			continue
		}
		if !tlsFingerprintProfileTransportMatches(p.Transport, transport) {
			continue
		}
		profiles = append(profiles, p)
	}
	if len(profiles) == 0 {
		return nil
	}

	return pickStableProfile(profiles, accountID)
}

func (s *TLSFingerprintProfileService) getRandomProfileForDimensionTransport(platform, os, clientType, transport string, accountID int64) *tlsfingerprint.Profile {
	if s == nil {
		return nil
	}
	s.localMu.RLock()
	defer s.localMu.RUnlock()

	if len(s.localCache) == 0 {
		return nil
	}

	normPlatform := strings.ToLower(strings.TrimSpace(platform))
	normOS := strings.ToLower(strings.TrimSpace(os))
	normClient := strings.ToLower(strings.TrimSpace(clientType))

	profiles := make([]*model.TLSFingerprintProfile, 0, len(s.localCache))
	for _, p := range s.localCache {
		if p == nil {
			continue
		}
		if !tlsFingerprintProfileRuntimeSupported(p) {
			continue
		}
		if tlsFingerprintProfileUsesUnsupportedH2Transport(p) {
			continue
		}
		pPlatform := strings.ToLower(strings.TrimSpace(p.Platform))
		if pPlatform != "" && pPlatform != normPlatform {
			continue
		}
		if normOS != "" {
			pOS := strings.ToLower(strings.TrimSpace(p.OS))
			if pOS != "" && pOS != normOS {
				continue
			}
		}
		if normClient != "" {
			pClient := strings.ToLower(strings.TrimSpace(p.ClientType))
			if pClient != "" && pClient != normClient {
				continue
			}
		}
		if !tlsFingerprintProfileTransportMatches(p.Transport, transport) {
			continue
		}
		profiles = append(profiles, p)
	}
	if len(profiles) == 0 {
		return nil
	}
	return pickStableProfile(profiles, accountID)
}

// ResolveTLSProfileForDimension 按账号的「OS×client 绑定矩阵」解析运行时 Profile。
//
// 逻辑：
//  1. 未启用 TLS 指纹 → nil
//  2. 按 (os, clientType) 维度解析出 profileID（含旧单值降级）
//  3. profileID>0 → 查模板；==-1 → 同维度随机；否则 → 空 Profile（内置默认）
func (s *TLSFingerprintProfileService) ResolveTLSProfileForDimension(account *Account, os, clientType string) *tlsfingerprint.Profile {
	profile, _ := s.resolveTLSProfileForDimension(account, os, clientType, "", true)
	return profile
}

func (s *TLSFingerprintProfileService) ResolveTLSProfileForDimensionMatch(account *Account, os, clientType, transport string) (*tlsfingerprint.Profile, bool) {
	return s.resolveTLSProfileForDimension(account, os, clientType, transport, false)
}

func (s *TLSFingerprintProfileService) resolveTLSProfileForDimension(account *Account, os, clientType, transport string, defaultWhenUnresolved bool) (*tlsfingerprint.Profile, bool) {
	if account == nil || !account.IsTLSFingerprintEnabled() {
		return nil, false
	}
	id := account.GetTLSFingerprintProfileIDForDimension(os, clientType)
	if id > 0 {
		if p := s.resolveProfileByIDForAccount(id, account, transport); p != nil {
			return p, true
		}
		if !defaultWhenUnresolved {
			return nil, false
		}
		return builtinDefaultTLSProfile(), true
	}
	if id == -1 {
		if p := s.getRandomProfileForDimensionTransport(account.Platform, os, clientType, transport, account.ID); p != nil {
			return p, true
		}
		if !defaultWhenUnresolved {
			return nil, false
		}
		return builtinDefaultTLSProfile(), true
	}
	if defaultWhenUnresolved {
		return builtinDefaultTLSProfile(), true
	}
	return nil, false
}

// ResolveTLSProfile 根据 Account 的配置解析出运行时 TLS Profile。
//
// 逻辑：
//  1. 未启用 TLS 指纹 → 返回 nil（不伪装）
//  2. 启用 + 绑定了 profile_id → 从缓存查找对应 profile
//  3. 启用 + 未绑定或找不到 → 返回空 Profile（使用代码内置默认值）
func (s *TLSFingerprintProfileService) ResolveTLSProfile(account *Account) *tlsfingerprint.Profile {
	return s.ResolveTLSProfileForTransport(account, "")
}

func (s *TLSFingerprintProfileService) ResolveTLSProfileForTransport(account *Account, transport string) *tlsfingerprint.Profile {
	if account == nil || !account.IsTLSFingerprintEnabled() {
		return nil
	}
	// 非 OpenAI 路径没有入站 UA 上下文，无法用路由判定维度。
	// 若账号配置了默认 OS，则按该 OS 维度从绑定矩阵解析（每账号可不同）。
	if defaultOS := account.GetTLSFingerprintDefaultOS(); defaultOS != "" {
		if p, resolved := s.resolveTLSProfileForDimension(account, defaultOS, "", transport, true); resolved && p != nil {
			return p
		}
	}
	id := account.GetTLSFingerprintProfileID()
	if id > 0 {
		if p := s.resolveProfileByIDForAccount(id, account, transport); p != nil {
			return p
		}
		return builtinDefaultTLSProfile()
	}
	if id == -1 {
		// 随机选择一个同平台或 shared profile
		if p := s.getRandomProfileForPlatformTransport(account.Platform, transport, account.ID); p != nil {
			return p
		}
	}
	// TLS 启用但无绑定 profile → 空 Profile → dialer 使用内置默认值
	return builtinDefaultTLSProfile()
}

func builtinDefaultTLSProfile() *tlsfingerprint.Profile {
	return &tlsfingerprint.Profile{Name: "Built-in Default (Node.js 24.x)"}
}

func tlsFingerprintProfileMatchesAccount(p *model.TLSFingerprintProfile, account *Account, transport string) bool {
	if p == nil {
		return false
	}
	if account != nil {
		profilePlatform := strings.ToLower(strings.TrimSpace(p.Platform))
		accountPlatform := strings.ToLower(strings.TrimSpace(account.Platform))
		if profilePlatform != "" && profilePlatform != accountPlatform {
			return false
		}
	}
	return tlsFingerprintProfileTransportMatches(p.Transport, transport)
}

func tlsFingerprintProfileRuntimeSupported(p *model.TLSFingerprintProfile) bool {
	if p == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(p.Transport)) {
	case "h2", "websocket-h2":
		if strings.TrimSpace(p.HTTP2Fingerprint) == "" {
			return false
		}
	}
	return true
}

func tlsFingerprintProfileUsesUnsupportedH2Transport(p *model.TLSFingerprintProfile) bool {
	return false
}

func tlsFingerprintProfileTransportMatches(profileTransport, runtimeTransport string) bool {
	profileTransport = strings.ToLower(strings.TrimSpace(profileTransport))
	runtimeTransport = strings.ToLower(strings.TrimSpace(runtimeTransport))
	if profileTransport == "" || runtimeTransport == "" {
		return true
	}
	if profileTransport == runtimeTransport {
		return true
	}
	switch runtimeTransport {
	case "http":
		return profileTransport == "http1" || profileTransport == "h2"
	case "websocket":
		return profileTransport == "websocket-http1" || profileTransport == "websocket-h2"
	default:
		return false
	}
}

// --- 缓存管理 ---

func (s *TLSFingerprintProfileService) refreshLocalCache(ctx context.Context) error {
	if s.cache != nil {
		if profiles, ok := s.cache.Get(ctx); ok {
			s.setLocalCache(profiles)
			return nil
		}
	}
	return s.reloadFromDB(ctx)
}

func (s *TLSFingerprintProfileService) reloadFromDB(ctx context.Context) error {
	repo, err := s.requireRepo()
	if err != nil {
		return err
	}
	profiles, err := repo.List(ctx)
	if err != nil {
		return err
	}

	if s.cache != nil {
		if err := s.cache.Set(ctx, profiles); err != nil {
			logger.LegacyPrintf("service.tls_fp_profile", "[TLSFPProfileService] Failed to set cache: %v", err)
		}
	}

	s.setLocalCache(profiles)
	return nil
}

func (s *TLSFingerprintProfileService) setLocalCache(profiles []*model.TLSFingerprintProfile) {
	m := make(map[int64]*model.TLSFingerprintProfile, len(profiles))
	for _, p := range profiles {
		m[p.ID] = p
	}

	s.localMu.Lock()
	s.localCache = m
	s.localMu.Unlock()
}

func (s *TLSFingerprintProfileService) newCacheRefreshContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 3*time.Second)
}

func (s *TLSFingerprintProfileService) invalidateAndNotify(ctx context.Context) {
	if s.cache != nil {
		if err := s.cache.Invalidate(ctx); err != nil {
			logger.LegacyPrintf("service.tls_fp_profile", "[TLSFPProfileService] Failed to invalidate cache: %v", err)
		}
	}

	if err := s.reloadFromDB(ctx); err != nil {
		logger.LegacyPrintf("service.tls_fp_profile", "[TLSFPProfileService] Failed to refresh local cache: %v", err)
		s.localMu.Lock()
		s.localCache = make(map[int64]*model.TLSFingerprintProfile)
		s.localMu.Unlock()
	}

	if s.cache != nil {
		if err := s.cache.NotifyUpdate(ctx); err != nil {
			logger.LegacyPrintf("service.tls_fp_profile", "[TLSFPProfileService] Failed to notify cache update: %v", err)
		}
	}
}
