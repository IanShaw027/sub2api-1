//go:build unit

package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type authRepoStub struct {
	getByKeyForAuth   func(ctx context.Context, key string) (*APIKey, error)
	listKeysByUserID  func(ctx context.Context, userID int64) ([]string, error)
	listKeysByGroupID func(ctx context.Context, groupID int64) ([]string, error)
}

func (s *authRepoStub) Create(ctx context.Context, key *APIKey) error {
	panic("unexpected Create call")
}

func (s *authRepoStub) GetByID(ctx context.Context, id int64) (*APIKey, error) {
	panic("unexpected GetByID call")
}

func (s *authRepoStub) GetKeyAndOwnerID(ctx context.Context, id int64) (string, int64, error) {
	panic("unexpected GetKeyAndOwnerID call")
}

func (s *authRepoStub) GetByKey(ctx context.Context, key string) (*APIKey, error) {
	panic("unexpected GetByKey call")
}

func (s *authRepoStub) GetByKeyForAuth(ctx context.Context, key string) (*APIKey, error) {
	if s.getByKeyForAuth == nil {
		panic("unexpected GetByKeyForAuth call")
	}
	return s.getByKeyForAuth(ctx, key)
}

func (s *authRepoStub) Update(ctx context.Context, key *APIKey) error {
	panic("unexpected Update call")
}

func (s *authRepoStub) Delete(ctx context.Context, id int64) error {
	panic("unexpected Delete call")
}

func (s *authRepoStub) DeleteWithAudit(ctx context.Context, id int64) error {
	panic("unexpected DeleteWithAudit call")
}

func (s *authRepoStub) ListByUserID(ctx context.Context, userID int64, params pagination.PaginationParams, filters APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	panic("unexpected ListByUserID call")
}

func (s *authRepoStub) VerifyOwnership(ctx context.Context, userID int64, apiKeyIDs []int64) ([]int64, error) {
	panic("unexpected VerifyOwnership call")
}

func (s *authRepoStub) CountByUserID(ctx context.Context, userID int64) (int64, error) {
	panic("unexpected CountByUserID call")
}

func (s *authRepoStub) ExistsByKey(ctx context.Context, key string) (bool, error) {
	panic("unexpected ExistsByKey call")
}

func (s *authRepoStub) ListByGroupID(ctx context.Context, groupID int64, params pagination.PaginationParams) ([]APIKey, *pagination.PaginationResult, error) {
	panic("unexpected ListByGroupID call")
}

func (s *authRepoStub) SearchAPIKeys(ctx context.Context, userID int64, keyword string, limit int) ([]APIKey, error) {
	panic("unexpected SearchAPIKeys call")
}

func (s *authRepoStub) ClearGroupIDByGroupID(ctx context.Context, groupID int64) (int64, error) {
	panic("unexpected ClearGroupIDByGroupID call")
}
func (s *authRepoStub) UpdateGroupIDByUserAndGroup(ctx context.Context, userID, oldGroupID, newGroupID int64) (int64, error) {
	panic("unexpected UpdateGroupIDByUserAndGroup call")
}

func (s *authRepoStub) CountByGroupID(ctx context.Context, groupID int64) (int64, error) {
	panic("unexpected CountByGroupID call")
}

func (s *authRepoStub) CountActiveByGroupID(ctx context.Context, groupID int64) (int64, error) {
	panic("unexpected CountActiveByGroupID call")
}

func (s *authRepoStub) ListKeysByUserID(ctx context.Context, userID int64) ([]string, error) {
	if s.listKeysByUserID == nil {
		panic("unexpected ListKeysByUserID call")
	}
	return s.listKeysByUserID(ctx, userID)
}

func (s *authRepoStub) ListKeysByGroupID(ctx context.Context, groupID int64) ([]string, error) {
	if s.listKeysByGroupID == nil {
		panic("unexpected ListKeysByGroupID call")
	}
	return s.listKeysByGroupID(ctx, groupID)
}

func (s *authRepoStub) IncrementQuotaUsed(ctx context.Context, id int64, amount float64) (float64, error) {
	panic("unexpected IncrementQuotaUsed call")
}

func (s *authRepoStub) UpdateLastUsed(ctx context.Context, id int64, usedAt time.Time) error {
	panic("unexpected UpdateLastUsed call")
}
func (s *authRepoStub) IncrementRateLimitUsage(ctx context.Context, id int64, cost float64) error {
	panic("unexpected IncrementRateLimitUsage call")
}
func (s *authRepoStub) ResetRateLimitWindows(ctx context.Context, id int64) error {
	panic("unexpected ResetRateLimitWindows call")
}
func (s *authRepoStub) GetRateLimitData(ctx context.Context, id int64) (*APIKeyRateLimitData, error) {
	panic("unexpected GetRateLimitData call")
}

type authCacheStub struct {
	getAuthCache   func(ctx context.Context, key string) (*APIKeyAuthCacheEntry, error)
	setAuthKeys    []string
	deleteAuthKeys []string
}

func (s *authCacheStub) GetCreateAttemptCount(ctx context.Context, userID int64) (int, error) {
	return 0, nil
}

func (s *authCacheStub) IncrementCreateAttemptCount(ctx context.Context, userID int64) error {
	return nil
}

func (s *authCacheStub) DeleteCreateAttemptCount(ctx context.Context, userID int64) error {
	return nil
}

func (s *authCacheStub) IncrementDailyUsage(ctx context.Context, apiKey string) error {
	return nil
}

func (s *authCacheStub) SetDailyUsageExpiry(ctx context.Context, apiKey string, ttl time.Duration) error {
	return nil
}

func (s *authCacheStub) GetAuthCache(ctx context.Context, key string) (*APIKeyAuthCacheEntry, error) {
	if s.getAuthCache == nil {
		return nil, redis.Nil
	}
	return s.getAuthCache(ctx, key)
}

func (s *authCacheStub) SetAuthCache(ctx context.Context, key string, entry *APIKeyAuthCacheEntry, ttl time.Duration) error {
	s.setAuthKeys = append(s.setAuthKeys, key)
	return nil
}

func (s *authCacheStub) DeleteAuthCache(ctx context.Context, key string) error {
	s.deleteAuthKeys = append(s.deleteAuthKeys, key)
	return nil
}

func (s *authCacheStub) PublishAuthCacheInvalidation(ctx context.Context, cacheKey string) error {
	return nil
}

func (s *authCacheStub) SubscribeAuthCacheInvalidation(ctx context.Context, handler func(cacheKey string)) error {
	return nil
}

func TestAPIKeyService_GetByKey_UsesL2Cache(t *testing.T) {
	cache := &authCacheStub{}
	repo := &authRepoStub{
		getByKeyForAuth: func(ctx context.Context, key string) (*APIKey, error) {
			return nil, errors.New("unexpected repo call")
		},
	}
	cfg := &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{
			L2TTLSeconds:       60,
			NegativeTTLSeconds: 30,
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, cfg)

	groupID := int64(9)
	cacheEntry := &APIKeyAuthCacheEntry{
		Snapshot: &APIKeyAuthSnapshot{
			Version:  apiKeyAuthSnapshotVersion,
			APIKeyID: 1,
			UserID:   2,
			GroupID:  &groupID,
			Status:   StatusActive,
			User: APIKeyAuthUserSnapshot{
				ID:          2,
				Status:      StatusActive,
				Role:        RoleUser,
				Balance:     10,
				Concurrency: 3,
			},
			Group: &APIKeyAuthGroupSnapshot{
				ID:                  groupID,
				Name:                "g",
				Platform:            PlatformAnthropic,
				Status:              StatusActive,
				SubscriptionType:    SubscriptionTypeStandard,
				RateMultiplier:      1,
				ModelRoutingEnabled: true,
				ModelRouting: map[string][]int64{
					"claude-opus-*": {1, 2},
				},
			},
		},
	}
	cache.getAuthCache = func(ctx context.Context, key string) (*APIKeyAuthCacheEntry, error) {
		return cacheEntry, nil
	}

	apiKey, err := svc.GetByKey(context.Background(), "k1")
	require.NoError(t, err)
	require.Equal(t, int64(1), apiKey.ID)
	require.Equal(t, int64(2), apiKey.User.ID)
	require.Equal(t, groupID, apiKey.Group.ID)
	require.True(t, apiKey.Group.ModelRoutingEnabled)
	require.Equal(t, map[string][]int64{"claude-opus-*": {1, 2}}, apiKey.Group.ModelRouting)
}

func TestAPIKeyService_SnapshotRoundTrip_PreservesGroupFeatureConfig(t *testing.T) {
	svc := NewAPIKeyService(nil, nil, nil, nil, nil, nil, &config.Config{})
	groupID := int64(9)
	video480p := 0.01
	video720p := 0.02
	video1080p := 0.03
	video4k := 0.04
	searchPrice := 1.23
	audioRealtimePrice := 2.34
	audioTTSPrice := 3.45
	audioSTTPrice := 4.56
	apiKey := &APIKey{
		ID:      1,
		UserID:  2,
		GroupID: &groupID,
		Key:     "k-roundtrip",
		Name:    "Audit Key",
		Status:  StatusActive,
		User: &User{
			ID:          2,
			Status:      StatusActive,
			Role:        RoleUser,
			Balance:     10,
			Concurrency: 3,
		},
		Group: &Group{
			ID:                           groupID,
			Name:                         "grok",
			Platform:                     PlatformGrok,
			Status:                       StatusActive,
			SubscriptionType:             SubscriptionTypeStandard,
			RateMultiplier:               1,
			AllowImageGeneration:         true,
			ImageGenerationRoute:         "native",
			AllowVideoGeneration:         true,
			VideoGenerationRoute:         "native",
			VideoPrice480pPerSec:         &video480p,
			VideoPrice720pPerSec:         &video720p,
			VideoPrice1080pPerSec:        &video1080p,
			VideoPrice4kPerSec:           &video4k,
			SearchPricePer1k:             &searchPrice,
			AudioRealtimePricePerMin:     &audioRealtimePrice,
			AudioTTSPricePerMillionChars: &audioTTSPrice,
			AudioSTTPricePerHour:         &audioSTTPrice,
			AllowMessagesDispatch:        true,
			DefaultMappedModel:           "gpt-5.4",
			MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{
				OpusMappedModel:   "gpt-5.4-nano",
				SonnetMappedModel: "gpt-5.3-codex",
				HaikuMappedModel:  "gpt-5.4-mini",
				ExactModelMappings: map[string]string{
					"claude-sonnet-4.5": "gpt-5.4-nano",
				},
			},
		},
	}

	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	roundTrip := svc.snapshotToAPIKey(apiKey.Key, snapshot)

	require.NotNil(t, roundTrip)
	require.Equal(t, apiKey.Name, roundTrip.Name)
	require.NotNil(t, roundTrip.Group)
	require.Equal(t, apiKey.Group.MessagesDispatchModelConfig, roundTrip.Group.MessagesDispatchModelConfig)
	require.Equal(t, "native", roundTrip.Group.ImageGenerationRoute)
	require.True(t, roundTrip.Group.AllowVideoGeneration)
	require.Equal(t, "native", roundTrip.Group.VideoGenerationRoute)
	require.Equal(t, apiKey.Group.VideoPrice480pPerSec, roundTrip.Group.VideoPrice480pPerSec)
	require.Equal(t, apiKey.Group.VideoPrice720pPerSec, roundTrip.Group.VideoPrice720pPerSec)
	require.Equal(t, apiKey.Group.VideoPrice1080pPerSec, roundTrip.Group.VideoPrice1080pPerSec)
	require.Equal(t, apiKey.Group.VideoPrice4kPerSec, roundTrip.Group.VideoPrice4kPerSec)
	require.Equal(t, apiKey.Group.SearchPricePer1k, roundTrip.Group.SearchPricePer1k)
	require.Equal(t, apiKey.Group.AudioRealtimePricePerMin, roundTrip.Group.AudioRealtimePricePerMin)
	require.Equal(t, apiKey.Group.AudioTTSPricePerMillionChars, roundTrip.Group.AudioTTSPricePerMillionChars)
	require.Equal(t, apiKey.Group.AudioSTTPricePerHour, roundTrip.Group.AudioSTTPricePerHour)
}

func TestAPIKeyService_GenerateKey_NilConfigFallsBackToDefaultPrefix(t *testing.T) {
	svc := NewAPIKeyService(nil, nil, nil, nil, nil, nil, nil)

	var key string
	require.NotPanics(t, func() {
		var err error
		key, err = svc.GenerateKey()
		require.NoError(t, err)
	})
	require.True(t, strings.HasPrefix(key, "sk-"), "nil cfg should fall back to sk- prefix")
	require.Len(t, key, 67, "expected sk- + 64 hex chars")
}

func TestAPIKeyService_List_NilRepoReturnsError(t *testing.T) {
	svc := NewAPIKeyService(nil, nil, nil, nil, nil, nil, &config.Config{})

	_, _, err := svc.List(context.Background(), 1, pagination.PaginationParams{}, APIKeyListFilters{})

	require.Error(t, err)
	require.Contains(t, err.Error(), "api key repository is unavailable")
}

func TestAPIKeyService_GetByKey_NilRepoReturnsError(t *testing.T) {
	svc := NewAPIKeyService(nil, nil, nil, nil, nil, nil, &config.Config{})

	_, err := svc.GetByKey(context.Background(), "sk-test")

	require.Error(t, err)
	require.Contains(t, err.Error(), "api key repository is unavailable")
}

func TestAPIKeyService_SearchAPIKeys_NilRepoReturnsError(t *testing.T) {
	svc := NewAPIKeyService(nil, nil, nil, nil, nil, nil, &config.Config{})

	_, err := svc.SearchAPIKeys(context.Background(), 1, "", 10)

	require.Error(t, err)
	require.Contains(t, err.Error(), "api key repository is unavailable")
}

func TestAPIKeyService_Delete_NilRepoReturnsError(t *testing.T) {
	svc := NewAPIKeyService(nil, nil, nil, nil, nil, nil, &config.Config{})

	err := svc.Delete(context.Background(), 1, 1)

	require.Error(t, err)
	require.Contains(t, err.Error(), "api key repository is unavailable")
}

func TestAPIKeyService_TouchLastUsed_NilRepoReturnsError(t *testing.T) {
	svc := NewAPIKeyService(nil, nil, nil, nil, nil, nil, &config.Config{})

	err := svc.TouchLastUsed(context.Background(), 1)

	require.Error(t, err)
	require.Contains(t, err.Error(), "api key repository is unavailable")
}

func TestAPIKeyService_Update_NilRepoReturnsError(t *testing.T) {
	svc := NewAPIKeyService(nil, nil, nil, nil, nil, nil, &config.Config{})

	_, err := svc.Update(context.Background(), 1, 1, UpdateAPIKeyRequest{})

	require.Error(t, err)
	require.Contains(t, err.Error(), "api key repository is unavailable")
}

func TestAPIKeyService_UpdateQuotaUsed_NilRepoReturnsError(t *testing.T) {
	svc := NewAPIKeyService(nil, nil, nil, nil, nil, nil, &config.Config{})

	err := svc.UpdateQuotaUsed(context.Background(), 1, 0.5)

	require.Error(t, err)
	require.Contains(t, err.Error(), "api key repository is unavailable")
}

func TestAPIKeyService_GetRateLimitData_NilRepoReturnsError(t *testing.T) {
	svc := NewAPIKeyService(nil, nil, nil, nil, nil, nil, &config.Config{})

	_, err := svc.GetRateLimitData(context.Background(), 1)

	require.Error(t, err)
	require.Contains(t, err.Error(), "api key repository is unavailable")
}

func TestAPIKeyService_UpdateRateLimitUsage_NilRepoReturnsError(t *testing.T) {
	svc := NewAPIKeyService(nil, nil, nil, nil, nil, nil, &config.Config{})

	err := svc.UpdateRateLimitUsage(context.Background(), 1, 0.5)

	require.Error(t, err)
	require.Contains(t, err.Error(), "api key repository is unavailable")
}

func TestAPIKeyService_GetAvailableGroups_NilUserRepoReturnsError(t *testing.T) {
	svc := NewAPIKeyService(nil, nil, nil, nil, nil, nil, &config.Config{})

	_, err := svc.GetAvailableGroups(context.Background(), 1)

	require.Error(t, err)
	require.Contains(t, err.Error(), "user repository is unavailable")
}

func TestAPIKeyService_ValidateKey_NilUserRepoReturnsError(t *testing.T) {
	repo := &authRepoStub{
		getByKeyForAuth: func(ctx context.Context, key string) (*APIKey, error) {
			return &APIKey{
				ID:     1,
				UserID: 7,
				Key:    key,
				Status: StatusActive,
				User: &User{
					ID:     7,
					Status: StatusActive,
				},
			}, nil
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, &config.Config{})

	_, _, err := svc.ValidateKey(context.Background(), "sk-test")

	require.Error(t, err)
	require.Contains(t, err.Error(), "user repository is unavailable")
}

func TestAPIKeyService_Create_NilUserRepoReturnsError(t *testing.T) {
	svc := NewAPIKeyService(nil, nil, nil, nil, nil, nil, &config.Config{})

	_, err := svc.Create(context.Background(), 1, CreateAPIKeyRequest{})

	require.Error(t, err)
	require.Contains(t, err.Error(), "user repository is unavailable")
}

func TestAPIKeyService_Create_CustomKeyNilRepoReturnsError(t *testing.T) {
	customKey := "custom-key-123456"
	svc := NewAPIKeyService(nil, &userRepoStub{user: &User{ID: 1, Status: StatusActive}}, nil, nil, nil, nil, &config.Config{})

	_, err := svc.Create(context.Background(), 1, CreateAPIKeyRequest{
		CustomKey: &customKey,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "api key repository is unavailable")
}

func TestAPIKeyService_GetAvailableRouteGroups_NilGroupRepoReturnsError(t *testing.T) {
	svc := NewAPIKeyService(&authRepoStub{}, nil, nil, nil, nil, nil, &config.Config{})

	_, err := svc.GetAvailableRouteGroups(context.Background(), 1)

	require.Error(t, err)
	require.Contains(t, err.Error(), "group repository is unavailable")
}

func TestAPIKeyService_SnapshotRoundTrip_ClearsNonGrokVideoConfig(t *testing.T) {
	svc := NewAPIKeyService(nil, nil, nil, nil, nil, nil, &config.Config{})
	groupID := int64(10)
	video480p := 0.01
	video720p := 0.02
	video1080p := 0.03
	video4k := 0.04
	apiKey := &APIKey{
		ID:      1,
		UserID:  2,
		GroupID: &groupID,
		Key:     "k-openai-stale-video",
		Name:    "OpenAI stale video",
		Status:  StatusActive,
		User: &User{
			ID:          2,
			Status:      StatusActive,
			Role:        RoleUser,
			Balance:     10,
			Concurrency: 3,
		},
		Group: &Group{
			ID:                    groupID,
			Name:                  "openai",
			Platform:              PlatformOpenAI,
			Status:                StatusActive,
			SubscriptionType:      SubscriptionTypeStandard,
			RateMultiplier:        1,
			AllowVideoGeneration:  true,
			VideoGenerationRoute:  GroupVideoGenerationRouteNative,
			VideoPrice480pPerSec:  &video480p,
			VideoPrice720pPerSec:  &video720p,
			VideoPrice1080pPerSec: &video1080p,
			VideoPrice4kPerSec:    &video4k,
		},
	}

	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	require.NotNil(t, snapshot.Group)
	require.False(t, snapshot.Group.AllowVideoGeneration)
	require.Equal(t, GroupVideoGenerationRouteNative, snapshot.Group.VideoGenerationRoute)
	require.Nil(t, snapshot.Group.VideoPrice480pPerSec)
	require.Nil(t, snapshot.Group.VideoPrice720pPerSec)
	require.Nil(t, snapshot.Group.VideoPrice1080pPerSec)
	require.Nil(t, snapshot.Group.VideoPrice4kPerSec)

	staleSnapshot := *snapshot
	staleGroup := *snapshot.Group
	staleGroup.AllowVideoGeneration = true
	staleGroup.VideoPrice480pPerSec = &video480p
	staleGroup.VideoPrice720pPerSec = &video720p
	staleGroup.VideoPrice1080pPerSec = &video1080p
	staleGroup.VideoPrice4kPerSec = &video4k
	staleSnapshot.Group = &staleGroup

	roundTrip := svc.snapshotToAPIKey(apiKey.Key, &staleSnapshot)
	require.NotNil(t, roundTrip)
	require.NotNil(t, roundTrip.Group)
	require.False(t, roundTrip.Group.AllowVideoGeneration)
	require.Equal(t, GroupVideoGenerationRouteNative, roundTrip.Group.VideoGenerationRoute)
	require.Nil(t, roundTrip.Group.VideoPrice480pPerSec)
	require.Nil(t, roundTrip.Group.VideoPrice720pPerSec)
	require.Nil(t, roundTrip.Group.VideoPrice1080pPerSec)
	require.Nil(t, roundTrip.Group.VideoPrice4kPerSec)
}

func TestAPIKeyService_GetByKey_IgnoresLegacyAuthCacheSnapshotWithoutMessagesDispatchConfig(t *testing.T) {
	cache := &authCacheStub{}
	var repoCalls int32
	repo := &authRepoStub{
		getByKeyForAuth: func(ctx context.Context, key string) (*APIKey, error) {
			atomic.AddInt32(&repoCalls, 1)
			groupID := int64(9)
			return &APIKey{
				ID:      1,
				UserID:  2,
				GroupID: &groupID,
				Status:  StatusActive,
				User: &User{
					ID:          2,
					Status:      StatusActive,
					Role:        RoleUser,
					Balance:     10,
					Concurrency: 3,
				},
				Group: &Group{
					ID:                    groupID,
					Name:                  "openai",
					Platform:              PlatformOpenAI,
					Status:                StatusActive,
					Hydrated:              true,
					SubscriptionType:      SubscriptionTypeStandard,
					RateMultiplier:        1,
					AllowMessagesDispatch: true,
					DefaultMappedModel:    "gpt-5.4",
					MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{
						OpusMappedModel: "gpt-5.4-nano",
					},
				},
			}, nil
		},
	}
	cfg := &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{
			L2TTLSeconds: 60,
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, cfg)

	groupID := int64(9)
	cache.getAuthCache = func(ctx context.Context, key string) (*APIKeyAuthCacheEntry, error) {
		return &APIKeyAuthCacheEntry{
			Snapshot: &APIKeyAuthSnapshot{
				APIKeyID: 1,
				UserID:   2,
				GroupID:  &groupID,
				Status:   StatusActive,
				User: APIKeyAuthUserSnapshot{
					ID:          2,
					Status:      StatusActive,
					Role:        RoleUser,
					Balance:     10,
					Concurrency: 3,
				},
				Group: &APIKeyAuthGroupSnapshot{
					ID:                    groupID,
					Name:                  "openai",
					Platform:              PlatformOpenAI,
					Status:                StatusActive,
					SubscriptionType:      SubscriptionTypeStandard,
					RateMultiplier:        1,
					AllowMessagesDispatch: true,
					DefaultMappedModel:    "gpt-5.4",
				},
			},
		}, nil
	}

	apiKey, err := svc.GetByKey(context.Background(), "k-legacy")
	require.NoError(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&repoCalls))
	require.NotNil(t, apiKey.Group)
	require.Equal(t, "gpt-5.4-nano", apiKey.Group.MessagesDispatchModelConfig.OpusMappedModel)
}

func TestAPIKeyService_GetByKey_NegativeCache(t *testing.T) {
	cache := &authCacheStub{}
	repo := &authRepoStub{
		getByKeyForAuth: func(ctx context.Context, key string) (*APIKey, error) {
			return nil, errors.New("unexpected repo call")
		},
	}
	cfg := &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{
			L2TTLSeconds:       60,
			NegativeTTLSeconds: 30,
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, cfg)
	cache.getAuthCache = func(ctx context.Context, key string) (*APIKeyAuthCacheEntry, error) {
		return &APIKeyAuthCacheEntry{NotFound: true}, nil
	}

	_, err := svc.GetByKey(context.Background(), "missing")
	require.ErrorIs(t, err, ErrAPIKeyNotFound)
}

func TestAPIKeyService_GetByKey_CacheMissStoresL2(t *testing.T) {
	cache := &authCacheStub{}
	repo := &authRepoStub{
		getByKeyForAuth: func(ctx context.Context, key string) (*APIKey, error) {
			return &APIKey{
				ID:     5,
				UserID: 7,
				Status: StatusActive,
				User: &User{
					ID:          7,
					Status:      StatusActive,
					Role:        RoleUser,
					Balance:     12,
					Concurrency: 2,
				},
			}, nil
		},
	}
	cfg := &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{
			L2TTLSeconds:       60,
			NegativeTTLSeconds: 30,
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, cfg)
	cache.getAuthCache = func(ctx context.Context, key string) (*APIKeyAuthCacheEntry, error) {
		return nil, redis.Nil
	}

	apiKey, err := svc.GetByKey(context.Background(), "k2")
	require.NoError(t, err)
	require.Equal(t, int64(5), apiKey.ID)
	require.Len(t, cache.setAuthKeys, 1)
}

func TestAPIKeyService_GetByKey_UsesL1Cache(t *testing.T) {
	var calls int32
	cache := &authCacheStub{}
	repo := &authRepoStub{
		getByKeyForAuth: func(ctx context.Context, key string) (*APIKey, error) {
			atomic.AddInt32(&calls, 1)
			return &APIKey{
				ID:     21,
				UserID: 3,
				Status: StatusActive,
				User: &User{
					ID:          3,
					Status:      StatusActive,
					Role:        RoleUser,
					Balance:     5,
					Concurrency: 2,
				},
			}, nil
		},
	}
	cfg := &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{
			L1Size:       1000,
			L1TTLSeconds: 60,
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, cfg)
	require.NotNil(t, svc.authCacheL1)

	_, err := svc.GetByKey(context.Background(), "k-l1")
	require.NoError(t, err)
	svc.authCacheL1.Wait()
	cacheKey := svc.authCacheKey("k-l1")
	_, ok := svc.authCacheL1.Get(cacheKey)
	require.True(t, ok)
	_, err = svc.GetByKey(context.Background(), "k-l1")
	require.NoError(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestAPIKeyService_InvalidateAuthCacheByUserID(t *testing.T) {
	cache := &authCacheStub{}
	repo := &authRepoStub{
		listKeysByUserID: func(ctx context.Context, userID int64) ([]string, error) {
			return []string{"k1", "k2"}, nil
		},
	}
	cfg := &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{
			L2TTLSeconds:       60,
			NegativeTTLSeconds: 30,
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, cfg)

	svc.InvalidateAuthCacheByUserID(context.Background(), 7)
	require.Len(t, cache.deleteAuthKeys, 2)
}

func TestAPIKeyService_InvalidateAuthCacheByGroupID(t *testing.T) {
	cache := &authCacheStub{}
	repo := &authRepoStub{
		listKeysByGroupID: func(ctx context.Context, groupID int64) ([]string, error) {
			return []string{"k1", "k2"}, nil
		},
	}
	cfg := &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{
			L2TTLSeconds: 60,
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, cfg)

	svc.InvalidateAuthCacheByGroupID(context.Background(), 9)
	require.Len(t, cache.deleteAuthKeys, 2)
}

func TestAPIKeyService_InvalidateAuthCacheByUserID_NilRepoIsNoop(t *testing.T) {
	cache := &authCacheStub{}
	svc := NewAPIKeyService(nil, nil, nil, nil, nil, cache, &config.Config{})

	require.NotPanics(t, func() {
		svc.InvalidateAuthCacheByUserID(context.Background(), 7)
	})
	require.Empty(t, cache.deleteAuthKeys)
}

func TestAPIKeyService_InvalidateAuthCacheByGroupID_NilRepoIsNoop(t *testing.T) {
	cache := &authCacheStub{}
	svc := NewAPIKeyService(nil, nil, nil, nil, nil, cache, &config.Config{})

	require.NotPanics(t, func() {
		svc.InvalidateAuthCacheByGroupID(context.Background(), 9)
	})
	require.Empty(t, cache.deleteAuthKeys)
}

func TestAPIKeyService_InvalidateAuthCacheByKey(t *testing.T) {
	cache := &authCacheStub{}
	repo := &authRepoStub{
		listKeysByUserID: func(ctx context.Context, userID int64) ([]string, error) {
			return nil, nil
		},
	}
	cfg := &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{
			L2TTLSeconds: 60,
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, cfg)

	svc.InvalidateAuthCacheByKey(context.Background(), "k1")
	require.Len(t, cache.deleteAuthKeys, 1)
}

func TestInvalidateAuthCacheByLookupHashDoesNotDoubleHashForLegacyInvalidator(t *testing.T) {
	invalidator := &authCacheInvalidatorStub{}

	invalidateAuthCacheByLookupHash(invalidator, context.Background(), HashAPIKeyLookup("sk-test"))

	require.Empty(t, invalidator.keys)
}

func TestAPIKeyService_GetByKey_CachesNegativeOnRepoMiss(t *testing.T) {
	cache := &authCacheStub{}
	repo := &authRepoStub{
		getByKeyForAuth: func(ctx context.Context, key string) (*APIKey, error) {
			return nil, ErrAPIKeyNotFound
		},
	}
	cfg := &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{
			L2TTLSeconds:       60,
			NegativeTTLSeconds: 30,
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, cfg)
	cache.getAuthCache = func(ctx context.Context, key string) (*APIKeyAuthCacheEntry, error) {
		return nil, redis.Nil
	}

	_, err := svc.GetByKey(context.Background(), "missing")
	require.ErrorIs(t, err, ErrAPIKeyNotFound)
	require.Len(t, cache.setAuthKeys, 1)
}

func TestAPIKeyService_GetByKey_SingleflightCollapses(t *testing.T) {
	var calls int32
	cache := &authCacheStub{}
	repo := &authRepoStub{
		getByKeyForAuth: func(ctx context.Context, key string) (*APIKey, error) {
			atomic.AddInt32(&calls, 1)
			time.Sleep(50 * time.Millisecond)
			return &APIKey{
				ID:     11,
				UserID: 2,
				Status: StatusActive,
				User: &User{
					ID:          2,
					Status:      StatusActive,
					Role:        RoleUser,
					Balance:     1,
					Concurrency: 1,
				},
			}, nil
		},
	}
	cfg := &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{
			Singleflight: true,
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, cfg)

	start := make(chan struct{})
	wg := sync.WaitGroup{}
	errs := make([]error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			_, err := svc.GetByKey(context.Background(), "k1")
			errs[idx] = err
		}(i)
	}
	close(start)
	wg.Wait()

	for _, err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
}
