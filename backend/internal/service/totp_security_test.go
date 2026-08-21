//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
)

type totpSecurityCacheStub struct {
	TotpCache
	reserved      int
	reserveErr    error
	reserveCalled bool
	clearCalled   bool
}

func (s *totpSecurityCacheStub) ReserveVerifyAttempt(context.Context, int64, int) (int, error) {
	s.reserveCalled = true
	return s.reserved, s.reserveErr
}

func (s *totpSecurityCacheStub) ClearVerifyAttempts(context.Context, int64) error {
	s.clearCalled = true
	return nil
}

type totpSecurityUserRepoStub struct {
	UserRepository
	user      *User
	getErr    error
	getCalled bool
}

func (s *totpSecurityUserRepoStub) GetByID(context.Context, int64) (*User, error) {
	s.getCalled = true
	return s.user, s.getErr
}

type totpSecurityEncryptorStub struct{}

func (totpSecurityEncryptorStub) Encrypt(value string) (string, error) { return value, nil }
func (totpSecurityEncryptorStub) Decrypt(value string) (string, error) { return value, nil }

func TestTotpVerifyCodeFailsClosedWhenAttemptReservationFails(t *testing.T) {
	cache := &totpSecurityCacheStub{reserveErr: errors.New("redis unavailable")}
	secret := "totp-secret"
	users := &totpSecurityUserRepoStub{user: &User{ID: 1, TotpEnabled: true, TotpSecretEncrypted: &secret}}
	svc := NewTotpService(users, totpSecurityEncryptorStub{}, cache, nil, nil, nil)

	err := svc.VerifyCode(context.Background(), 1, "123456")
	require.Error(t, err)
	require.Equal(t, 503, infraerrors.Code(err))
	require.True(t, users.getCalled)
	require.True(t, cache.reserveCalled)
}

func TestTotpVerifyCodeRejectsAttemptPastLimitAfterLoadingSecret(t *testing.T) {
	cache := &totpSecurityCacheStub{reserved: maxTotpAttempts + 1}
	secret := "totp-secret"
	users := &totpSecurityUserRepoStub{user: &User{ID: 1, TotpEnabled: true, TotpSecretEncrypted: &secret}}
	svc := NewTotpService(users, totpSecurityEncryptorStub{}, cache, nil, nil, nil)

	err := svc.VerifyCode(context.Background(), 1, "123456")
	require.ErrorIs(t, err, ErrTotpTooManyAttempts)
	require.True(t, users.getCalled)
}

func TestTotpVerifyCodeDoesNotReserveAttemptWhenUserLookupFails(t *testing.T) {
	cache := &totpSecurityCacheStub{}
	users := &totpSecurityUserRepoStub{getErr: errors.New("database unavailable")}
	svc := NewTotpService(users, totpSecurityEncryptorStub{}, cache, nil, nil, nil)

	err := svc.VerifyCode(context.Background(), 1, "123456")
	require.Error(t, err)
	require.False(t, cache.reserveCalled)
}

func TestTotpVerifyCodeClearsReservedAttemptsOnSuccess(t *testing.T) {
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "test", AccountName: "user@example.com"})
	require.NoError(t, err)
	code, err := totp.GenerateCode(key.Secret(), time.Now())
	require.NoError(t, err)
	secret := key.Secret()
	cache := &totpSecurityCacheStub{reserved: maxTotpAttempts}
	users := &totpSecurityUserRepoStub{user: &User{
		ID:                  1,
		TotpEnabled:         true,
		TotpSecretEncrypted: &secret,
	}}
	svc := NewTotpService(users, totpSecurityEncryptorStub{}, cache, nil, nil, nil)

	require.NoError(t, svc.VerifyCode(context.Background(), 1, code))
	require.True(t, cache.clearCalled)
}
