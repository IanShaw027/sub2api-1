//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestRevalidateLongLivedAPIKeyUsesAuthLookupAndEnforcesState(t *testing.T) {
	groupID := int64(9)
	original := &APIKey{ID: 11, UserID: 22, Key: "sk-live-session-key", GroupID: &groupID}

	newKey := func() *APIKey {
		return &APIKey{
			ID:          original.ID,
			UserID:      original.UserID,
			Key:         original.Key,
			GroupID:     &groupID,
			Status:      StatusActive,
			IPWhitelist: []string{"203.0.113.7"},
			User:        &User{ID: original.UserID, Status: StatusActive, Concurrency: 3, AllowedGroups: []int64{groupID}},
			Group:       &Group{ID: groupID, Status: StatusActive, IsExclusive: true, Hydrated: true},
		}
	}

	current := newKey()
	repo := &authRepoStub{getByKeyForAuth: func(_ context.Context, key string) (*APIKey, error) {
		require.Equal(t, original.Key, key)
		clone := *current
		return &clone, nil
	}}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, &config.Config{})

	fresh, err := svc.RevalidateLongLivedAPIKey(context.Background(), original.Key, original, "203.0.113.7")
	require.NoError(t, err)
	require.Equal(t, 3, fresh.User.Concurrency)

	current = newKey()
	current.Status = StatusAPIKeyDisabled
	_, err = svc.RevalidateLongLivedAPIKey(context.Background(), original.Key, original, "203.0.113.7")
	require.ErrorIs(t, err, ErrAPIKeySessionInvalid)

	current = newKey()
	current.User.Status = StatusDisabled
	_, err = svc.RevalidateLongLivedAPIKey(context.Background(), original.Key, original, "203.0.113.7")
	require.ErrorIs(t, err, ErrAPIKeySessionInvalid)

	current = newKey()
	_, err = svc.RevalidateLongLivedAPIKey(context.Background(), original.Key, original, "198.51.100.8")
	require.ErrorIs(t, err, ErrAPIKeySessionInvalid)
}

func TestRevalidateLongLivedAPIKeyRejectsDeletionAndAuthorizationChanges(t *testing.T) {
	groupID := int64(9)
	original := &APIKey{ID: 11, UserID: 22, Key: "sk-live-session-key", GroupID: &groupID}
	currentErr := error(nil)
	current := &APIKey{
		ID:      original.ID,
		UserID:  original.UserID,
		GroupID: &groupID,
		Status:  StatusActive,
		User:    &User{ID: original.UserID, Status: StatusActive},
		Group:   &Group{ID: groupID, Status: StatusActive, IsExclusive: true, Hydrated: true},
	}
	repo := &authRepoStub{getByKeyForAuth: func(context.Context, string) (*APIKey, error) {
		if currentErr != nil {
			return nil, currentErr
		}
		clone := *current
		return &clone, nil
	}}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, &config.Config{})

	_, err := svc.RevalidateLongLivedAPIKey(context.Background(), original.Key, original, "203.0.113.7")
	require.ErrorIs(t, err, ErrAPIKeySessionInvalid, "exclusive group authorization was not present")

	current.User.AllowedGroups = []int64{groupID}
	otherGroupID := int64(10)
	current.GroupID = &otherGroupID
	_, err = svc.RevalidateLongLivedAPIKey(context.Background(), original.Key, original, "203.0.113.7")
	require.ErrorIs(t, err, ErrAPIKeySessionInvalid)

	currentErr = ErrAPIKeyNotFound
	_, err = svc.RevalidateLongLivedAPIKey(context.Background(), original.Key, original, "203.0.113.7")
	require.ErrorIs(t, err, ErrAPIKeySessionInvalid)
}

func TestLegacyWeakAPIKeyRemainsValidForAuthentication(t *testing.T) {
	const legacyKey = "aaaaaaaaaaaaaaaa"
	repo := &authRepoStub{getByKeyForAuth: func(_ context.Context, key string) (*APIKey, error) {
		require.Equal(t, legacyKey, key)
		return &APIKey{
			ID:     31,
			UserID: 41,
			Status: StatusActive,
			User:   &User{ID: 41, Status: StatusActive},
		}, nil
	}}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, &config.Config{})

	// Weak-key validation is a create-time policy only. Existing credentials
	// must continue to authenticate and survive long-lived-session rechecks.
	got, err := svc.GetByKey(context.Background(), legacyKey)
	require.NoError(t, err)
	require.Equal(t, legacyKey, got.Key)

	fresh, err := svc.RevalidateLongLivedAPIKey(context.Background(), legacyKey, got, "203.0.113.7")
	require.NoError(t, err)
	require.Equal(t, got.ID, fresh.ID)
}
