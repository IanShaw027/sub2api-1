//go:build integration

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
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type EmailCacheSuite struct {
	IntegrationRedisSuite
	cache service.EmailCache
}

func (s *EmailCacheSuite) SetupTest() {
	s.IntegrationRedisSuite.SetupTest()
	s.cache = NewEmailCache(s.rdb)
}

func (s *EmailCacheSuite) TestGetVerificationCode_Missing() {
	_, err := s.cache.GetVerificationCode(s.ctx, "nonexistent@example.com")
	require.True(s.T(), errors.Is(err, redis.Nil), "expected redis.Nil for missing verification code")
}

func (s *EmailCacheSuite) TestSetAndGetVerificationCode() {
	email := "a@example.com"
	emailTTL := 2 * time.Minute
	data := &service.VerificationCodeData{Code: "123456", Attempts: 1, CreatedAt: time.Now()}

	require.NoError(s.T(), s.cache.SetVerificationCode(s.ctx, email, data, emailTTL), "SetVerificationCode")

	got, err := s.cache.GetVerificationCode(s.ctx, email)
	require.NoError(s.T(), err, "GetVerificationCode")
	require.Equal(s.T(), "123456", got.Code)
	require.Equal(s.T(), 1, got.Attempts)
}

func (s *EmailCacheSuite) TestVerificationCode_TTL() {
	email := "ttl@example.com"
	emailTTL := 2 * time.Minute
	data := &service.VerificationCodeData{Code: "654321", Attempts: 0, CreatedAt: time.Now()}

	require.NoError(s.T(), s.cache.SetVerificationCode(s.ctx, email, data, emailTTL), "SetVerificationCode")

	emailKey := verifyCodeKeyPrefix + email
	ttl, err := s.rdb.TTL(s.ctx, emailKey).Result()
	require.NoError(s.T(), err, "TTL emailKey")
	s.AssertTTLWithin(ttl, 1*time.Second, emailTTL)
}

func (s *EmailCacheSuite) TestDeleteVerificationCode() {
	email := "delete@example.com"
	data := &service.VerificationCodeData{Code: "999999", Attempts: 0, CreatedAt: time.Now()}

	require.NoError(s.T(), s.cache.SetVerificationCode(s.ctx, email, data, 2*time.Minute), "SetVerificationCode")

	// Verify it exists
	_, err := s.cache.GetVerificationCode(s.ctx, email)
	require.NoError(s.T(), err, "GetVerificationCode before delete")

	// Delete
	require.NoError(s.T(), s.cache.DeleteVerificationCode(s.ctx, email), "DeleteVerificationCode")

	// Verify it's gone
	_, err = s.cache.GetVerificationCode(s.ctx, email)
	require.True(s.T(), errors.Is(err, redis.Nil), "expected redis.Nil after delete")
}

func (s *EmailCacheSuite) TestDeleteVerificationCode_NonExistent() {
	// Deleting a non-existent key should not error
	require.NoError(s.T(), s.cache.DeleteVerificationCode(s.ctx, "nonexistent@example.com"), "DeleteVerificationCode non-existent")
}

func (s *EmailCacheSuite) TestGetVerificationCode_JSONCorruption() {
	emailKey := verifyCodeKeyPrefix + "corrupted@example.com"

	require.NoError(s.T(), s.rdb.Set(s.ctx, emailKey, "not-json", 1*time.Minute).Err(), "Set invalid JSON")

	_, err := s.cache.GetVerificationCode(s.ctx, "corrupted@example.com")
	require.Error(s.T(), err, "expected error for corrupted JSON")
	require.False(s.T(), errors.Is(err, redis.Nil), "expected decoding error, not redis.Nil")
}

func (s *EmailCacheSuite) TestVerificationCodeAtomicAttemptLimitAndConsumption() {
	atomicCache := s.cache.(service.EmailAtomicCache)
	email := "atomic-code@example.com"
	data := &service.VerificationCodeData{Code: "123456", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Minute)}
	require.NoError(s.T(), s.cache.SetVerificationCode(s.ctx, email, data, time.Minute))

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(guess int) {
			defer wg.Done()
			_, _ = atomicCache.VerifyAndConsumeVerificationCode(context.Background(), email, fmt.Sprintf("%06d", guess), 5)
		}(i)
	}
	wg.Wait()
	stored, err := s.cache.GetVerificationCode(s.ctx, email)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 5, stored.Attempts)

	data = &service.VerificationCodeData{Code: "654321", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Minute)}
	require.NoError(s.T(), s.cache.SetVerificationCode(s.ctx, email, data, time.Minute))
	var accepted atomic.Int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, verifyErr := atomicCache.VerifyAndConsumeVerificationCode(context.Background(), email, "654321", 5)
			if verifyErr == nil && result == service.VerificationCodeAccepted {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(s.T(), int32(1), accepted.Load())
}

func (s *EmailCacheSuite) TestPasswordResetTokenClaimRestoreAndFinalize() {
	atomicCache := s.cache.(service.EmailAtomicCache)
	email := "atomic-reset@example.com"
	data := &service.PasswordResetTokenData{Token: "secret-token", CreatedAt: time.Now()}
	require.NoError(s.T(), s.cache.SetPasswordResetToken(s.ctx, email, data, time.Minute))

	var wg sync.WaitGroup
	var claimed atomic.Int32
	var claimedID atomic.Value
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			candidateID := fmt.Sprintf("claim-%d", id)
			ok, claimErr := atomicCache.ClaimPasswordResetToken(context.Background(), email, data.Token, candidateID)
			if claimErr == nil && ok {
				claimedID.Store(candidateID)
				claimed.Add(1)
			}
		}(i)
	}
	wg.Wait()
	require.Equal(s.T(), int32(1), claimed.Load())
	require.NoError(s.T(), atomicCache.FinalizePasswordResetTokenClaim(s.ctx, email, claimedID.Load().(string)))

	// Use a deterministic claim to verify the failure compensation lifecycle.
	require.NoError(s.T(), s.cache.SetPasswordResetToken(s.ctx, email, data, time.Minute))
	ok, err := atomicCache.ClaimPasswordResetToken(s.ctx, email, data.Token, "restore-me")
	require.NoError(s.T(), err)
	require.True(s.T(), ok)
	_, err = atomicCache.GetOrCreatePasswordResetToken(s.ctx, email, &service.PasswordResetTokenData{Token: "second-token"}, time.Minute)
	require.Error(s.T(), err, "a claimed token must block issuing a second reset token")
	require.Error(s.T(), atomicCache.RestorePasswordResetTokenClaim(s.ctx, email, "wrong-owner"))
	require.NoError(s.T(), atomicCache.RestorePasswordResetTokenClaim(s.ctx, email, "restore-me"))
	ok, err = atomicCache.ClaimPasswordResetToken(s.ctx, email, data.Token, "finalize-me")
	require.NoError(s.T(), err)
	require.True(s.T(), ok)
	require.NoError(s.T(), atomicCache.FinalizePasswordResetTokenClaim(s.ctx, email, "finalize-me"))
	ok, err = atomicCache.ClaimPasswordResetToken(s.ctx, email, data.Token, "replay")
	require.NoError(s.T(), err)
	require.False(s.T(), ok)
}

func (s *EmailCacheSuite) TestEmailSendReservationIsAtomic() {
	atomicCache := s.cache.(service.EmailAtomicCache)
	var wg sync.WaitGroup
	var reserved atomic.Int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ok, reserveErr := atomicCache.ReserveVerificationCodeSend(context.Background(), "reserve@example.com", fmt.Sprintf("worker-%d", id), time.Minute)
			if reserveErr == nil && ok {
				reserved.Add(1)
			}
		}(i)
	}
	wg.Wait()
	require.Equal(s.T(), int32(1), reserved.Load())
}

func TestEmailCacheSuite(t *testing.T) {
	suite.Run(t, new(EmailCacheSuite))
}
