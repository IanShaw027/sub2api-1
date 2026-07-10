package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMergePaymentConfigUpdatePreservesSubscriptionExchangeRate(t *testing.T) {
	current := &service.PaymentConfig{
		SubscriptionUSDToCNYRate: 7.15,
		RechargeFeeRate:          2.5,
	}

	merged := mergePaymentConfigUpdate(UpdateSettingsRequest{}, current)

	require.NotNil(t, merged.SubscriptionUSDToCNYRate)
	require.Equal(t, 7.15, *merged.SubscriptionUSDToCNYRate)
	require.NotNil(t, merged.RechargeFeeRate)
	require.Equal(t, 2.5, *merged.RechargeFeeRate)
}

func TestMergePaymentConfigUpdateAppliesSubscriptionExchangeRate(t *testing.T) {
	rate := 7.25
	current := &service.PaymentConfig{
		SubscriptionUSDToCNYRate: 7.15,
	}

	merged := mergePaymentConfigUpdate(UpdateSettingsRequest{
		PaymentSubscriptionUSDToCNYRate: &rate,
	}, current)

	require.NotNil(t, merged.SubscriptionUSDToCNYRate)
	require.Equal(t, 7.25, *merged.SubscriptionUSDToCNYRate)
}
