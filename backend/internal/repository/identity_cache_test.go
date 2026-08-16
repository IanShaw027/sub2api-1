//go:build unit

package repository

import (
	"context"
	"math"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newTestIdentityCache(t *testing.T) *identityCache {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewIdentityCache(rdb).(*identityCache)
}

func TestDeviceProfile_GetMissingReturnsNilNil(t *testing.T) {
	cache := newTestIdentityCache(t)
	got, err := cache.GetDeviceProfile(context.Background(), 42)
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestDeviceProfile_SetThenGetRoundTrip(t *testing.T) {
	cache := newTestIdentityCache(t)
	profile := &service.AccountDeviceProfile{
		AccountID:          7,
		DeviceID:           "dev-abc",
		GatewayAccountUUID: "11111111-2222-4333-8444-555555555555",
		SessionNamespace:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	require.NoError(t, cache.SetDeviceProfile(context.Background(), 7, profile))

	got, err := cache.GetDeviceProfile(context.Background(), 7)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "dev-abc", got.DeviceID)
	require.Equal(t, "11111111-2222-4333-8444-555555555555", got.GatewayAccountUUID)
	require.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", got.SessionNamespace)
}

func TestDeviceProfile_SetNilIsError(t *testing.T) {
	cache := newTestIdentityCache(t)
	err := cache.SetDeviceProfile(context.Background(), 7, nil)
	require.Error(t, err)

	got, getErr := cache.GetDeviceProfile(context.Background(), 7)
	require.NoError(t, getErr)
	require.Nil(t, got)
}

func TestDeviceProfile_SetDeletesLeftoverFingerprint(t *testing.T) {
	cache := newTestIdentityCache(t)
	ctx := context.Background()
	require.NoError(t, cache.SetFingerprint(ctx, 9, &service.Fingerprint{ClientID: "old"}))

	require.NoError(t, cache.SetDeviceProfile(ctx, 9, &service.AccountDeviceProfile{
		AccountID:          9,
		DeviceID:           "dev-9",
		GatewayAccountUUID: "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee",
		SessionNamespace:   "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}))

	fp, err := cache.GetFingerprint(ctx, 9)
	require.ErrorIs(t, err, redis.Nil)
	require.Nil(t, fp)
}

func TestDeviceProfile_DeleteRemovesProjection(t *testing.T) {
	cache := newTestIdentityCache(t)
	ctx := context.Background()
	require.NoError(t, cache.SetDeviceProfile(ctx, 11, &service.AccountDeviceProfile{
		AccountID:          11,
		DeviceID:           "dev-11",
		GatewayAccountUUID: "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee",
		SessionNamespace:   "cccccccccccccccccccccccccccccccc",
	}))

	require.NoError(t, cache.SetFingerprint(ctx, 11, &service.Fingerprint{ClientID: "old"}))
	require.NoError(t, cache.DeleteDeviceProfile(ctx, 11))
	got, err := cache.GetDeviceProfile(ctx, 11)
	require.NoError(t, err)
	require.Nil(t, got)

	fp, err := cache.GetFingerprint(ctx, 11)
	require.ErrorIs(t, err, redis.Nil)
	require.Nil(t, fp)

	require.NoError(t, cache.DeleteDeviceProfile(ctx, 11))
}

func TestFingerprintKey(t *testing.T) {
	tests := []struct {
		name      string
		accountID int64
		expected  string
	}{
		{
			name:      "normal_account_id",
			accountID: 123,
			expected:  "fingerprint:123",
		},
		{
			name:      "zero_account_id",
			accountID: 0,
			expected:  "fingerprint:0",
		},
		{
			name:      "negative_account_id",
			accountID: -1,
			expected:  "fingerprint:-1",
		},
		{
			name:      "max_int64",
			accountID: math.MaxInt64,
			expected:  "fingerprint:9223372036854775807",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := fingerprintKey(tc.accountID)
			require.Equal(t, tc.expected, got)
		})
	}
}
