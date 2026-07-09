//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPaymentConfigService_NilRepoReadPathsUseSafeDefaults(t *testing.T) {
	svc := &PaymentConfigService{}

	require.False(t, svc.IsPaymentEnabled(context.Background()))

	cfg, err := svc.GetPaymentConfig(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.False(t, cfg.Enabled)
	require.Equal(t, 1.0, cfg.MinAmount)
	require.Equal(t, defaultOrderTimeoutMin, cfg.OrderTimeoutMin)
	require.Equal(t, defaultMaxPendingOrders, cfg.MaxPendingOrders)
}

func TestPaymentConfigService_UpdatePaymentConfig_NilRepoReturnsError(t *testing.T) {
	svc := &PaymentConfigService{}

	err := svc.UpdatePaymentConfig(context.Background(), UpdatePaymentConfigRequest{})

	require.Error(t, err)
	require.Contains(t, err.Error(), "setting repository unavailable")
}
