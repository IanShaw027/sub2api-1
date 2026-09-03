//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCreationKeyResolver_CreatesAndReusesInternalKey(t *testing.T) {
	byPurpose := map[string]*APIKey{}
	repo := &authRepoStub{
		create: func(ctx context.Context, key *APIKey) error {
			byPurpose[key.Key] = key
			key.ID = int64(len(byPurpose))
			return nil
		},
		getByUserGroupAndPurpose: func(ctx context.Context, userID, groupID int64, purpose string) (*APIKey, error) {
			for _, key := range byPurpose {
				if key.UserID == userID && key.GroupID != nil && *key.GroupID == groupID && key.Purpose == purpose {
					return key, nil
				}
			}
			return nil, nil
		},
		getByKeyForAuth: func(ctx context.Context, key string) (*APIKey, error) {
			found := byPurpose[key]
			if found == nil {
				return nil, ErrAPIKeyNotFound
			}
			groupID := int64(9)
			return &APIKey{
				ID:      found.ID,
				UserID:  found.UserID,
				Key:     found.Key,
				Purpose: found.Purpose,
				User: &User{
					ID:          found.UserID,
					Role:        "user",
					Status:      StatusActive,
					Concurrency: 2,
				},
				Group: &Group{
					ID:       groupID,
					Platform: PlatformOpenAI,
					Status:   StatusActive,
				},
			}, nil
		},
	}
	cfg := &config.Config{Default: config.DefaultConfig{APIKeyPrefix: "sk-"}}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)
	resolver := NewCreationKeyResolver(repo, svc)

	first, err := resolver.ResolveAuthKey(context.Background(), 42, 9)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Equal(t, APIKeyPurposeCreation, first.Purpose)

	second, err := resolver.ResolveAuthKey(context.Background(), 42, 9)
	require.NoError(t, err)
	require.Equal(t, first.Key, second.Key)
	require.Len(t, byPurpose, 1)
}

func TestCreationKeyResolver_ReactivatesDisabledInternalKey(t *testing.T) {
	var updated bool
	key := &APIKey{
		ID:      3,
		UserID:  42,
		Key:     "sk-hidden",
		Purpose: APIKeyPurposeCreation,
		Status:  StatusAPIKeyDisabled,
	}
	groupID := int64(9)
	key.GroupID = &groupID
	repo := &authRepoStub{
		getByUserGroupAndPurpose: func(ctx context.Context, userID, gid int64, purpose string) (*APIKey, error) {
			return key, nil
		},
		getByKeyForAuth: func(ctx context.Context, raw string) (*APIKey, error) {
			return &APIKey{
				ID:      key.ID,
				UserID:  key.UserID,
				Key:     key.Key,
				Purpose: key.Purpose,
				Status:  key.Status,
				User:    &User{ID: key.UserID, Role: "user", Status: StatusActive, Concurrency: 2},
				Group:   &Group{ID: groupID, Platform: PlatformOpenAI, Status: StatusActive},
			}, nil
		},
		update: func(ctx context.Context, apiKey *APIKey, fields APIKeyUpdateFields) error {
			require.True(t, fields.Status)
			require.Equal(t, StatusAPIKeyActive, apiKey.Status)
			key.Status = StatusAPIKeyActive
			updated = true
			return nil
		},
	}
	cfg := &config.Config{Default: config.DefaultConfig{APIKeyPrefix: "sk-"}}
	resolver := NewCreationKeyResolver(repo, NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg))

	got, err := resolver.ResolveAuthKey(context.Background(), 42, 9)
	require.NoError(t, err)
	require.True(t, updated)
	require.Equal(t, StatusAPIKeyActive, got.Status)
}

func TestCreationKeyResolver_RejectsMissingGroup(t *testing.T) {
	cfg := &config.Config{Default: config.DefaultConfig{APIKeyPrefix: "sk-"}}
	resolver := NewCreationKeyResolver(&authRepoStub{}, NewAPIKeyService(&authRepoStub{}, nil, nil, nil, nil, nil, cfg))
	_, err := resolver.ResolveAuthKey(context.Background(), 1, 0)
	require.ErrorIs(t, err, ErrCreationGroupRequired)
}
