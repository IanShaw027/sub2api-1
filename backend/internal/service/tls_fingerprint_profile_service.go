package service

import (
	"context"
	"math/rand/v2"
	"sort"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
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

type DeviceTLSCatalogOption struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	ClientFamily  string `json:"client_family"`
	OSFamily      string `json:"os_family"`
	Transport     string `json:"transport"`
	SoftwareLabel string `json:"software_label"`
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
	if err := svc.reloadFromDB(ctx); err != nil {
		logger.LegacyPrintf("service.tls_fp_profile", "[TLSFPProfileService] Failed to load profiles from DB on startup: %v", err)
		if fallbackErr := svc.refreshLocalCache(ctx); fallbackErr != nil {
			logger.LegacyPrintf("service.tls_fp_profile", "[TLSFPProfileService] Failed to load profiles from cache fallback on startup: %v", fallbackErr)
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

// --- CRUD ---

// List 获取所有模板
func (s *TLSFingerprintProfileService) List(ctx context.Context) ([]*model.TLSFingerprintProfile, error) {
	return s.repo.List(ctx)
}

func (s *TLSFingerprintProfileService) ListCompleteOptions(ctx context.Context, platform string) ([]DeviceTLSCatalogOption, error) {
	_ = ctx
	registry := NewSoftwareBundleRegistry()
	options := make([]DeviceTLSCatalogOption, 0)

	s.localMu.RLock()
	defer s.localMu.RUnlock()

	for _, profile := range s.localCache {
		if profile == nil || !TLSPinCatalogComplete(profile.Name, profile.CipherSuites, profile.Extensions, profile.ALPNProtocols) {
			continue
		}
		parsed, ok := ParseTLSPinProfileName(profile.Name)
		if !ok || !TLSPinFamilyMatchesPlatform(parsed.ClientFamily, platform) {
			continue
		}
		if rejectUnusableTLSPin(parsed) != nil {
			continue
		}
		bundle, ok := lookupCurrentSoftwareBundle(registry, platform, parsed.ClientFamily)
		if !ok {
			continue
		}

		description := ""
		if profile.Description != nil {
			description = *profile.Description
		}
		softwareLabel := bundle.ClientVersion
		if parsed.ClientFamily == ClientFamilyClaudeCode {
			softwareLabel += " / " + claude.CLIStainlessPackageVersion
		}
		options = append(options, DeviceTLSCatalogOption{
			ID:            profile.ID,
			Name:          profile.Name,
			Description:   description,
			ClientFamily:  parsed.ClientFamily,
			OSFamily:      parsed.OSFamily,
			Transport:     parsed.Transport,
			SoftwareLabel: softwareLabel,
		})
	}

	sort.Slice(options, func(i, j int) bool {
		return options[i].ID < options[j].ID
	})
	return options, nil
}

func lookupCurrentSoftwareBundle(registry *SoftwareBundleRegistry, platform, family string) (SoftwareBundle, bool) {
	for _, bundle := range compileTimeSoftwareBundles() {
		if bundle.Platform == platform && bundle.ClientFamily == family {
			return registry.Lookup(platform, family, bundle.ClientVersion)
		}
	}
	return SoftwareBundle{}, false
}

// GetByID 根据 ID 获取模板
func (s *TLSFingerprintProfileService) GetByID(ctx context.Context, id int64) (*model.TLSFingerprintProfile, error) {
	return s.repo.GetByID(ctx, id)
}

// Create 创建模板
func (s *TLSFingerprintProfileService) Create(ctx context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	created, err := s.repo.Create(ctx, profile)
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
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	updated, err := s.repo.Update(ctx, profile)
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
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	refreshCtx, cancel := s.newCacheRefreshContext()
	defer cancel()
	s.invalidateAndNotify(refreshCtx)

	return nil
}

// --- 热路径：运行时 Profile 查找 ---

// GetProfileByID 根据 ID 从本地缓存获取 Profile（用于 DoWithTLS 热路径）
// 返回 nil 表示未找到，调用方应 fallback 到内置默认 Profile
func (s *TLSFingerprintProfileService) GetProfileByID(id int64) *tlsfingerprint.Profile {
	s.localMu.RLock()
	p, ok := s.localCache[id]
	s.localMu.RUnlock()

	if ok && p != nil {
		profile := p.ToTLSProfile()
		if profile != nil {
			profile.ID = p.ID
		}
		return profile
	}
	return nil
}

// getRandomProfile 从本地缓存中随机选择一个 Profile
func (s *TLSFingerprintProfileService) getRandomProfile() *tlsfingerprint.Profile {
	s.localMu.RLock()
	defer s.localMu.RUnlock()

	if len(s.localCache) == 0 {
		return nil
	}

	// 收集所有 profile
	profiles := make([]*model.TLSFingerprintProfile, 0, len(s.localCache))
	for _, p := range s.localCache {
		if p != nil {
			profiles = append(profiles, p)
		}
	}
	if len(profiles) == 0 {
		return nil
	}

	selected := profiles[rand.IntN(len(profiles))]
	profile := selected.ToTLSProfile()
	if profile != nil {
		profile.ID = selected.ID
	}
	return profile
}

// ResolveTLSProfile 根据 Account 的配置解析出运行时 TLS Profile
//
// 逻辑：
//  1. 未启用 TLS 指纹 → 返回 nil（不伪装）
//  2. 启用 + 绑定了 profile_id → 从缓存查找对应 profile
//  3. 启用 + 未绑定或找不到 → 返回空 Profile（使用代码内置默认值）
func (s *TLSFingerprintProfileService) ResolveTLSProfile(account *Account) *tlsfingerprint.Profile {
	if account == nil || !account.IsTLSFingerprintEnabled() {
		return nil
	}
	id := account.GetTLSFingerprintProfileID()
	if id > 0 {
		if p := s.GetProfileByID(id); p != nil {
			return p
		}
	}
	if id == -1 {
		// 随机选择一个 profile
		if p := s.getRandomProfile(); p != nil {
			return p
		}
	}
	// TLS 启用但无绑定 profile → 空 Profile → dialer 使用内置默认值
	return &tlsfingerprint.Profile{Name: "Built-in Default (Node.js 24.x)"}
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
	profiles, err := s.repo.List(ctx)
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
	SetTLSProfilePinLookup(s.lookupPin)
	SetTLSProfilePinNameLookup(s.lookupNameByID)
	setDeviceTLSCatalogGuard(s.RejectUnusableDeviceTLSSelection)
}

func (s *TLSFingerprintProfileService) lookupPin(family, osFamily, transport string) *int64 {
	if s == nil {
		return nil
	}
	want := TLSPinProfileName(family, osFamily, transport)
	s.localMu.RLock()
	defer s.localMu.RUnlock()
	byName := make(map[string]int64, len(s.localCache))
	for id, p := range s.localCache {
		if p != nil && p.Name != "" {
			byName[p.Name] = id
		}
	}
	if id, ok := byName[want]; ok && id > 0 {
		copied := id
		return &copied
	}
	return nil
}

func (s *TLSFingerprintProfileService) lookupNameByID(id int64) string {
	if s == nil || id <= 0 {
		return ""
	}
	s.localMu.RLock()
	defer s.localMu.RUnlock()
	if p := s.localCache[id]; p != nil {
		return p.Name
	}
	return ""
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
