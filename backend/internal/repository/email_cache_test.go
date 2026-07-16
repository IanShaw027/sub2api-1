//go:build unit

package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newPasswordResetCacheTest(t *testing.T) (*emailCache, context.Context) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return &emailCache{rdb: rdb}, context.Background()
}

func TestVerifyCodeKey(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected string
	}{
		{name: "normal_email", email: "user@example.com", expected: "verify_code:user@example.com"},
		{name: "empty_email", email: "", expected: "verify_code:"},
		{name: "email_with_plus", email: "user+tag@example.com", expected: "verify_code:user+tag@example.com"},
		{name: "email_with_special_chars", email: "user.name+tag@sub.domain.com", expected: "verify_code:user.name+tag@sub.domain.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, verifyCodeKey(tc.email))
		})
	}
}

func TestEmailCachePasswordResetClaimIsAtomic(t *testing.T) {
	cache, ctx := newPasswordResetCacheTest(t)
	const (
		email = "reset@example.com"
		token = "valid-reset-token"
	)
	require.NoError(t, cache.SetPasswordResetToken(ctx, email, &service.PasswordResetTokenData{
		Token:     token,
		CreatedAt: time.Now().UTC(),
	}, time.Hour))

	var successes atomic.Int32
	var winner atomic.Value
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(claimID string) {
			defer wg.Done()
			err := cache.ClaimPasswordResetToken(ctx, email, token, claimID)
			if err == nil {
				successes.Add(1)
				winner.Store(claimID)
				return
			}
			if !errors.Is(err, service.ErrInvalidResetToken) {
				t.Errorf("ClaimPasswordResetToken() error = %v", err)
			}
		}(fmt.Sprintf("claim-%d", i))
	}
	wg.Wait()

	require.Equal(t, int32(1), successes.Load())
	_, err := cache.GetPasswordResetToken(ctx, email)
	require.ErrorIs(t, err, redis.Nil)
	require.NoError(t, cache.CompletePasswordResetToken(ctx, email, winner.Load().(string)))
}

func TestEmailCachePasswordResetClaimRollbackRestoresOriginalToken(t *testing.T) {
	cache, ctx := newPasswordResetCacheTest(t)
	const (
		email   = "rollback@example.com"
		token   = "rollback-reset-token"
		claimID = "rollback-claim"
	)
	require.NoError(t, cache.SetPasswordResetToken(ctx, email, &service.PasswordResetTokenData{
		Token:     token,
		CreatedAt: time.Now().UTC(),
	}, time.Hour))
	require.NoError(t, cache.ClaimPasswordResetToken(ctx, email, token, claimID))
	require.NoError(t, cache.RestorePasswordResetToken(ctx, email, claimID))

	restored, err := cache.GetPasswordResetToken(ctx, email)
	require.NoError(t, err)
	require.Equal(t, token, restored.Token)
	require.NoError(t, cache.ClaimPasswordResetToken(ctx, email, token, "second-claim"))
}
