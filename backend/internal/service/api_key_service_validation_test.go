//go:build unit

package service

import (
	"context"
	"math"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestValidateCreateAPIKeyRequestNumericLimits(t *testing.T) {
	positiveExpiry := 1
	require.NoError(t, validateCreateAPIKeyRequest(CreateAPIKeyRequest{
		Quota: 1e100, RateLimit5h: 1e100, ExpiresInDays: &positiveExpiry,
	}))
	require.NoError(t, validateCreateAPIKeyRequest(CreateAPIKeyRequest{}))

	invalidExpiry := 0
	tests := []CreateAPIKeyRequest{
		{Quota: -1},
		{Quota: math.NaN()},
		{Quota: math.Inf(1)},
		{RateLimit5h: -1},
		{RateLimit1d: math.NaN()},
		{RateLimit7d: math.Inf(-1)},
		{ExpiresInDays: &invalidExpiry},
	}
	for _, req := range tests {
		require.Error(t, validateCreateAPIKeyRequest(req))
	}
}

func TestValidateCustomKeyRejectsObviouslyPredictableValues(t *testing.T) {
	svc := &APIKeyService{}
	for _, key := range []string{
		"aaaaaaaaaaaaaaaa",
		"abababababababab",
		"abcdabcdabcdabcd",
		"passwordpassword",
		"abcdefghijklmnop",
		"ponmlkjihgfedcba",
	} {
		t.Run(key, func(t *testing.T) {
			require.ErrorIs(t, svc.ValidateCustomKey(key), ErrAPIKeyWeak)
		})
	}
}

func TestValidateCustomKeyPreservesReasonableExistingFormats(t *testing.T) {
	svc := &APIKeyService{}
	for _, key := range []string{
		"sk_custom_1234567890",
		"team-prod-key_2026_A7f9",
		"0123456789abcdef0123456789abcdef",
	} {
		t.Run(key, func(t *testing.T) {
			require.NoError(t, svc.ValidateCustomKey(key))
		})
	}
}

func TestCreateCustomAPIKeyDoesNotExposeCrossTenantCollision(t *testing.T) {
	customKey := "team-prod-key_2026_A7f9"
	userRepo := &mockUserRepo{getByIDUser: &User{ID: 7, Status: StatusActive}}

	t.Run("preflight collision", func(t *testing.T) {
		repo := &authRepoStub{existsByKey: func(_ context.Context, key string) (bool, error) {
			require.Equal(t, customKey, key)
			return true, nil
		}}
		svc := NewAPIKeyService(repo, userRepo, nil, nil, nil, nil, &config.Config{})
		_, err := svc.Create(context.Background(), 7, CreateAPIKeyRequest{Name: "key", CustomKey: &customKey})
		require.ErrorIs(t, err, ErrAPIKeyCustomUnavailable)
		require.NotErrorIs(t, err, ErrAPIKeyExists)
	})

	t.Run("concurrent unique collision", func(t *testing.T) {
		repo := &authRepoStub{
			existsByKey: func(context.Context, string) (bool, error) { return false, nil },
			create:      func(context.Context, *APIKey) error { return ErrAPIKeyExists },
		}
		svc := NewAPIKeyService(repo, userRepo, nil, nil, nil, nil, &config.Config{})
		_, err := svc.Create(context.Background(), 7, CreateAPIKeyRequest{Name: "key", CustomKey: &customKey})
		require.ErrorIs(t, err, ErrAPIKeyCustomUnavailable)
		require.NotErrorIs(t, err, ErrAPIKeyExists)
	})
}

func TestValidateUpdateAPIKeyRequestNumericLimits(t *testing.T) {
	zero, large, negative, nan, inf := 0.0, 1e100, -1.0, math.NaN(), math.Inf(1)
	require.NoError(t, validateUpdateAPIKeyRequest(UpdateAPIKeyRequest{Quota: &zero, RateLimit7d: &large}))

	for _, req := range []UpdateAPIKeyRequest{
		{Quota: &negative},
		{RateLimit5h: &nan},
		{RateLimit1d: &inf},
		{RateLimit7d: &negative},
	} {
		require.Error(t, validateUpdateAPIKeyRequest(req))
	}
}
