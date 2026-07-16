//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

type invitationAtomicUserRepoStub struct {
	*userRepoStub
	mu         sync.Mutex
	inviteUsed bool
	createErr  error
	calls      int
}

func (s *invitationAtomicUserRepoStub) CreateWithInvitation(ctx context.Context, user *User, _ int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.createErr != nil {
		return s.createErr
	}
	if s.inviteUsed {
		return ErrRedeemCodeUsed
	}
	s.inviteUsed = true
	return s.userRepoStub.Create(ctx, user)
}

type passwordResetClaimCacheStub struct {
	*emailCacheStub
	mu            sync.Mutex
	token         string
	claimed       bool
	claimCalls    int
	completeCalls int
	restoreCalls  int
}

func (s *passwordResetClaimCacheStub) ClaimPasswordResetToken(_ context.Context, _ string, token, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claimCalls++
	if s.claimed || token != s.token || token == "" {
		return ErrInvalidResetToken
	}
	s.claimed = true
	return nil
}

func (s *passwordResetClaimCacheStub) CompletePasswordResetToken(context.Context, string, string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completeCalls++
	s.token = ""
	s.claimed = false
	return nil
}

func (s *passwordResetClaimCacheStub) RestorePasswordResetToken(context.Context, string, string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restoreCalls++
	s.claimed = false
	return nil
}

func TestRegisterInvitationConsumptionFailureDoesNotCreateUser(t *testing.T) {
	baseRepo := &userRepoStub{nextID: 42}
	userRepo := &invitationAtomicUserRepoStub{
		userRepoStub: baseRepo,
		createErr:    ErrRedeemCodeUsed,
	}
	redeemRepo := &redeemCodeRepoStub{codesByCode: map[string]*RedeemCode{
		"INVITE123": {
			ID:     7,
			Code:   "INVITE123",
			Type:   RedeemTypeInvitation,
			Status: StatusUnused,
		},
	}}
	svc := newAuthService(baseRepo, map[string]string{
		SettingKeyRegistrationEnabled:   "true",
		SettingKeyInvitationCodeEnabled: "true",
	}, nil, nil)
	svc.userRepo = userRepo
	svc.redeemRepo = redeemRepo

	token, user, err := svc.RegisterWithVerification(
		context.Background(),
		"invite@example.com",
		"password8",
		"",
		"",
		"INVITE123",
		"",
	)

	require.ErrorIs(t, err, ErrInvitationCodeInvalid)
	require.Empty(t, token)
	require.Nil(t, user)
	require.Empty(t, baseRepo.created)
	require.Equal(t, 1, userRepo.calls)
}

func TestRegisterInvitationAllowsExactlyOneConcurrentUser(t *testing.T) {
	baseRepo := &userRepoStub{nextID: 42}
	userRepo := &invitationAtomicUserRepoStub{userRepoStub: baseRepo}
	redeemRepo := &redeemCodeRepoStub{codesByCode: map[string]*RedeemCode{
		"INVITE123": {
			ID:     7,
			Code:   "INVITE123",
			Type:   RedeemTypeInvitation,
			Status: StatusUnused,
		},
	}}
	svc := newAuthService(baseRepo, map[string]string{
		SettingKeyRegistrationEnabled:   "true",
		SettingKeyInvitationCodeEnabled: "true",
	}, nil, nil)
	svc.userRepo = userRepo
	svc.redeemRepo = redeemRepo

	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, _, err := svc.RegisterWithVerification(
				context.Background(),
				fmt.Sprintf("invite-%d@example.com", index),
				"password8",
				"",
				"",
				"INVITE123",
				"",
			)
			if err == nil {
				successes.Add(1)
				return
			}
			if !errors.Is(err, ErrInvitationCodeInvalid) {
				t.Errorf("RegisterWithVerification() error = %v", err)
			}
		}(i)
	}
	wg.Wait()

	require.Equal(t, int32(1), successes.Load())
	require.Len(t, baseRepo.created, 1)
}

func TestResetPasswordRestoresClaimWhenUserUpdateFails(t *testing.T) {
	oldHash, err := (&AuthService{}).HashPassword("old-pass8")
	require.NoError(t, err)
	user := &User{ID: 9, Email: "reset@example.com", PasswordHash: oldHash, Status: StatusActive}
	userRepo := &userRepoStub{
		user:          user,
		usersByEmail:  map[string]*User{user.Email: user},
		updateErr:     errors.New("db unavailable"),
		getByEmailErr: nil,
	}
	cache := &passwordResetClaimCacheStub{
		emailCacheStub: &emailCacheStub{},
		token:          "reset-token",
	}
	svc := newAuthService(userRepo, map[string]string{
		SettingKeyEmailVerifyEnabled:   "true",
		SettingKeyPasswordResetEnabled: "true",
	}, cache, nil)

	err = svc.ResetPassword(context.Background(), user.Email, "reset-token", "new-pass8")
	require.ErrorIs(t, err, ErrServiceUnavailable)
	require.Equal(t, 1, cache.claimCalls)
	require.Equal(t, 1, cache.restoreCalls)
	require.Equal(t, 0, cache.completeCalls)

	userRepo.updateErr = nil
	user.PasswordHash = oldHash
	user.TokenVersion = 0
	err = svc.ResetPassword(context.Background(), user.Email, "reset-token", "new-pass8")
	require.NoError(t, err)
	require.Equal(t, 2, cache.claimCalls)
	require.Equal(t, 1, cache.restoreCalls)
	require.Equal(t, 1, cache.completeCalls)

	err = svc.ResetPassword(context.Background(), user.Email, "reset-token", "another8")
	require.ErrorIs(t, err, ErrInvalidResetToken)
	require.Len(t, userRepo.updated, 1)
}

func TestAuthPasswordMinimumLengthIsEight(t *testing.T) {
	userRepo := &userRepoStub{}
	svc := newAuthService(userRepo, map[string]string{
		SettingKeyRegistrationEnabled: "true",
	}, nil, nil)

	_, _, err := svc.Register(context.Background(), "short@example.com", "1234567")
	require.ErrorIs(t, err, ErrPasswordTooShort)
	require.Empty(t, userRepo.created)

	cache := &passwordResetClaimCacheStub{
		emailCacheStub: &emailCacheStub{},
		token:          "reset-token",
	}
	resetUser := &User{ID: 10, Email: "reset@example.com", Status: StatusActive}
	userRepo.user = resetUser
	userRepo.usersByEmail = map[string]*User{resetUser.Email: resetUser}
	svc = newAuthService(userRepo, map[string]string{
		SettingKeyEmailVerifyEnabled:   "true",
		SettingKeyPasswordResetEnabled: "true",
	}, cache, nil)

	err = svc.ResetPassword(context.Background(), resetUser.Email, "reset-token", "1234567")
	require.ErrorIs(t, err, ErrPasswordTooShort)
	require.Equal(t, 0, cache.claimCalls)
}
