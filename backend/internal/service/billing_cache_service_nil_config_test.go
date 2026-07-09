//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewBillingCacheService_NilConfigDoesNotPanic(t *testing.T) {
	require.NotPanics(t, func() {
		svc := NewBillingCacheService(nil, nil, nil, nil, nil, nil, nil, nil)
		require.NotNil(t, svc)
		svc.Stop()
	})
}

func TestCheckBillingEligibility_NilConfigUsesSafeDefaults(t *testing.T) {
	cache := &balanceEligibilityCacheStub{balance: 1}
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, nil, nil)
	t.Cleanup(svc.Stop)

	require.NotPanics(t, func() {
		err := svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, nil, nil, nil, "")
		require.NoError(t, err)
	})
}
