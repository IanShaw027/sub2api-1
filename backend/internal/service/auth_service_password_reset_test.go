//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type passwordResetAtomicCacheStub struct {
	emailCacheStub
	token       string
	claimID     string
	restoreCall int
	finalCall   int
}

func (s *passwordResetAtomicCacheStub) ClaimPasswordResetToken(_ context.Context, _ string, token, claimID string) (bool, error) {
	if s.token == "" || s.claimID != "" || token != s.token {
		return false, nil
	}
	s.claimID = claimID
	s.token = ""
	return true, nil
}

func (s *passwordResetAtomicCacheStub) RestorePasswordResetTokenClaim(_ context.Context, _ string, claimID string) error {
	if claimID == "" || claimID != s.claimID {
		return errors.New("claim ownership mismatch")
	}
	s.token = "reset-token"
	s.claimID = ""
	s.restoreCall++
	return nil
}

func (s *passwordResetAtomicCacheStub) FinalizePasswordResetTokenClaim(_ context.Context, _ string, claimID string) error {
	if claimID == "" || claimID != s.claimID {
		return errors.New("claim ownership mismatch")
	}
	s.claimID = ""
	s.finalCall++
	return nil
}

func newPasswordResetAuthService(t *testing.T, cache *passwordResetAtomicCacheStub, repo *mockUserRepo) *AuthService {
	t.Helper()
	settings := &settingRepoStub{values: map[string]string{
		SettingKeyEmailVerifyEnabled:   "true",
		SettingKeyPasswordResetEnabled: "true",
	}}
	cfg := &config.Config{}
	return NewAuthService(
		nil,
		repo,
		nil,
		&refreshTokenCacheStub{},
		cfg,
		NewSettingService(settings, cfg),
		NewEmailService(settings, cache),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
}

func TestResetPasswordRestoresClaimWhenDatabaseUpdateFails(t *testing.T) {
	cache := &passwordResetAtomicCacheStub{token: "reset-token"}
	repo := &mockUserRepo{
		getByEmailUser: &User{ID: 42, Email: "user@example.com", Status: StatusActive, PasswordHash: "old-hash"},
		updateFn: func(context.Context, *User) error {
			return errors.New("database unavailable")
		},
	}
	auth := newPasswordResetAuthService(t, cache, repo)

	err := auth.ResetPassword(context.Background(), "user@example.com", "reset-token", "new-password")
	require.ErrorIs(t, err, ErrServiceUnavailable)
	require.Equal(t, 1, cache.restoreCall)
	require.Equal(t, "reset-token", cache.token)
	require.Empty(t, cache.claimID)
}

func TestResetPasswordFinalizesClaimWhenAmbiguousUpdateWasCommitted(t *testing.T) {
	cache := &passwordResetAtomicCacheStub{token: "reset-token"}
	repo := &mockUserRepo{
		getByEmailUser: &User{ID: 42, Email: "user@example.com", Status: StatusActive, PasswordHash: "old-hash"},
	}
	repo.updateFn = func(_ context.Context, updated *User) error {
		persisted := *updated
		repo.getByIDUser = &persisted
		return errors.New("response lost after commit")
	}
	auth := newPasswordResetAuthService(t, cache, repo)

	require.NoError(t, auth.ResetPassword(context.Background(), "user@example.com", "reset-token", "new-password"))
	require.Equal(t, 1, cache.finalCall)
	require.Zero(t, cache.restoreCall)
}

func TestResetPasswordFinalizesClaimWhenAmbiguousUpdateCannotBeVerified(t *testing.T) {
	cache := &passwordResetAtomicCacheStub{token: "reset-token"}
	repo := &mockUserRepo{
		getByEmailUser: &User{ID: 42, Email: "user@example.com", Status: StatusActive, PasswordHash: "old-hash"},
		getByIDErr:     errors.New("database unavailable"),
		updateFn: func(context.Context, *User) error {
			return errors.New("database timeout")
		},
	}
	auth := newPasswordResetAuthService(t, cache, repo)

	err := auth.ResetPassword(context.Background(), "user@example.com", "reset-token", "new-password")
	require.ErrorIs(t, err, ErrServiceUnavailable)
	require.Equal(t, 1, cache.finalCall)
	require.Zero(t, cache.restoreCall)
}

func TestResetPasswordFinalizesClaimAfterDatabaseUpdate(t *testing.T) {
	cache := &passwordResetAtomicCacheStub{token: "reset-token"}
	user := &User{ID: 43, Email: "user@example.com", Status: StatusActive, PasswordHash: "old-hash"}
	var storedHash string
	repo := &mockUserRepo{
		getByEmailUser: user,
		updateFn: func(_ context.Context, updated *User) error {
			storedHash = updated.PasswordHash
			return nil
		},
	}
	auth := newPasswordResetAuthService(t, cache, repo)

	require.NoError(t, auth.ResetPassword(context.Background(), user.Email, "reset-token", "new-password"))
	require.Equal(t, 1, cache.finalCall)
	require.Zero(t, cache.restoreCall)
	require.Empty(t, cache.token)
	require.True(t, auth.CheckPassword("new-password", storedHash))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.ErrorIs(t, auth.ResetPassword(ctx, user.Email, "reset-token", "another-password"), ErrInvalidResetToken)
}
