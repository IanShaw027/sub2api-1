//go:build unit

package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type authEpochRefreshCache struct {
	tokens           map[string]*RefreshTokenData
	epoch            string
	rotationResponse string
}

func (c *authEpochRefreshCache) StoreRefreshToken(_ context.Context, hash string, data *RefreshTokenData, _ time.Duration) error {
	if c.tokens == nil {
		c.tokens = make(map[string]*RefreshTokenData)
	}
	copyData := *data
	c.tokens[hash] = &copyData
	return nil
}

func (c *authEpochRefreshCache) GetRefreshToken(_ context.Context, hash string) (*RefreshTokenData, error) {
	data := c.tokens[hash]
	if data == nil {
		return nil, ErrRefreshTokenNotFound
	}
	copyData := *data
	return &copyData, nil
}

func (c *authEpochRefreshCache) DeleteRefreshToken(_ context.Context, hash string) error {
	delete(c.tokens, hash)
	return nil
}

func (c *authEpochRefreshCache) DeleteUserRefreshTokens(context.Context, int64) error {
	clear(c.tokens)
	return nil
}

func (c *authEpochRefreshCache) DeleteTokenFamily(_ context.Context, familyID string) error {
	for hash, data := range c.tokens {
		if data.FamilyID == familyID {
			delete(c.tokens, hash)
		}
	}
	return nil
}

func (*authEpochRefreshCache) AddToUserTokenSet(context.Context, int64, string, time.Duration) error {
	return nil
}

func (*authEpochRefreshCache) AddToFamilyTokenSet(context.Context, string, string, time.Duration) error {
	return nil
}

func (*authEpochRefreshCache) GetUserTokenHashes(context.Context, int64) ([]string, error) {
	return nil, nil
}

func (*authEpochRefreshCache) GetFamilyTokenHashes(context.Context, string) ([]string, error) {
	return nil, nil
}

func (*authEpochRefreshCache) IsTokenInFamily(context.Context, string, string) (bool, error) {
	return false, nil
}

func (c *authEpochRefreshCache) RotateRefreshToken(_ context.Context, oldHash, newHash string, newData *RefreshTokenData, _ time.Duration, _ time.Duration, response string) (*RefreshTokenRotationResult, error) {
	if c.tokens[oldHash] == nil {
		return &RefreshTokenRotationResult{Status: RefreshTokenRotationMissing}, nil
	}
	delete(c.tokens, oldHash)
	copyData := *newData
	c.tokens[newHash] = &copyData
	c.rotationResponse = response
	return &RefreshTokenRotationResult{Status: RefreshTokenRotationSucceeded, Response: response}, nil
}

func (c *authEpochRefreshCache) GetRefreshTokenRotationState(context.Context, string) (*RefreshTokenRotationResult, error) {
	if c.rotationResponse != "" {
		return &RefreshTokenRotationResult{Status: RefreshTokenRotationRetry, Response: c.rotationResponse}, nil
	}
	return &RefreshTokenRotationResult{Status: RefreshTokenRotationMissing}, nil
}

func (c *authEpochRefreshCache) GetUserAuthEpoch(context.Context, int64) (string, error) {
	return c.epoch, nil
}

func (c *authEpochRefreshCache) EnsureUserAuthEpoch(_ context.Context, _ int64, candidate string, _ time.Duration) (string, error) {
	if c.epoch == "" {
		c.epoch = candidate
	}
	return c.epoch, nil
}

func (c *authEpochRefreshCache) RevokeUserSessions(_ context.Context, _ int64, epoch string, _ time.Duration) error {
	c.epoch = epoch
	clear(c.tokens)
	return nil
}

func TestMissingAuthEpochFailsClosedForIssuedSession(t *testing.T) {
	ctx := context.Background()
	user := &User{ID: 42, Email: "epoch@example.com", PasswordHash: "hash", Status: StatusActive, Role: RoleUser}
	repo := &mockUserRepo{getByIDUser: user}
	cache := &authEpochRefreshCache{tokens: make(map[string]*RefreshTokenData), epoch: "post-revoke-epoch"}
	cfg := &config.Config{JWT: config.JWTConfig{
		Secret:                   "auth-epoch-test-secret",
		AccessTokenExpireMinutes: 60,
		RefreshTokenExpireDays:   7,
	}}
	auth := NewAuthService(nil, repo, nil, cache, cfg, nil, nil, nil, nil, nil, nil, nil, nil)

	pair, err := auth.GenerateTokenPair(ctx, user, "family-after-revoke")
	require.NoError(t, err)

	require.ErrorIs(t, auth.ValidateAccessTokenState(ctx, &JWTClaims{UserID: user.ID}), ErrTokenRevoked)
	require.NoError(t, auth.ValidateAccessTokenState(ctx, &JWTClaims{UserID: user.ID, AuthEpoch: cache.epoch}))

	cache.epoch = "" // Simulate Redis flush or eviction before the issued tokens expire.
	_, err = auth.RefreshTokenPair(ctx, pair.RefreshToken)
	require.ErrorIs(t, err, ErrServiceUnavailable)
	require.ErrorIs(t, auth.ValidateAccessTokenState(ctx, &JWTClaims{UserID: user.ID, AuthEpoch: "post-revoke-epoch"}), ErrServiceUnavailable)
}

func TestRefreshRotationRetryIsEncryptedAndBoundToRequestFingerprint(t *testing.T) {
	user := &User{ID: 51, Email: "retry@example.com", PasswordHash: "hash", Status: StatusActive, Role: RoleUser}
	repo := &mockUserRepo{getByIDUser: user}
	cache := &authEpochRefreshCache{tokens: make(map[string]*RefreshTokenData)}
	cfg := &config.Config{JWT: config.JWTConfig{
		Secret:                   "refresh-retry-encryption-test-secret",
		AccessTokenExpireMinutes: 60,
		RefreshTokenExpireDays:   7,
	}}
	auth := NewAuthService(nil, repo, nil, cache, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	ctx := WithSessionBinding(context.Background(), &SessionBinding{IP: "203.0.113.10", UserAgent: "test-agent"})

	initial, err := auth.GenerateTokenPair(ctx, user, "retry-family")
	require.NoError(t, err)
	rotated, err := auth.RefreshTokenPair(ctx, initial.RefreshToken)
	require.NoError(t, err)
	require.NotEmpty(t, cache.rotationResponse)
	require.False(t, strings.Contains(cache.rotationResponse, rotated.AccessToken))
	require.False(t, strings.Contains(cache.rotationResponse, rotated.RefreshToken))

	retried, err := auth.RefreshTokenPair(ctx, initial.RefreshToken)
	require.NoError(t, err)
	require.Equal(t, rotated.TokenPair, retried.TokenPair)

	otherCtx := WithSessionBinding(context.Background(), &SessionBinding{IP: "203.0.113.11", UserAgent: "test-agent"})
	_, err = auth.RefreshTokenPair(otherCtx, initial.RefreshToken)
	require.ErrorIs(t, err, ErrRefreshTokenReused)
}
