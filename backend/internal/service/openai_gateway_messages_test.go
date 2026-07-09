package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayServiceBridgeGatewayServicePrefersInjectedGateway(t *testing.T) {
	injected := &GatewayService{cfg: &config.Config{}}
	svc := &OpenAIGatewayService{}

	svc.SetGatewayService(injected)

	require.Same(t, injected, svc.bridgeGatewayService())
}

func TestOpenAIGatewayServiceBridgeGatewayServiceFallsBackToLegacySubsetWhenUnset(t *testing.T) {
	cfg := &config.Config{}
	concurrency := &ConcurrencyService{}
	billing := &BillingService{}
	billingCache := &BillingCacheService{}
	rateLimit := &RateLimitService{}
	setting := NewSettingService(nil, cfg)
	channel := &ChannelService{}
	resolver := &ModelPricingResolver{}
	tlsProfiles := &TLSFingerprintProfileService{}
	balanceNotify := &BalanceNotifyService{}
	fingerprintNormalizer := &FingerprintNormalizer{}
	svc := &OpenAIGatewayService{
		cfg:                   cfg,
		concurrencyService:    concurrency,
		billingService:        billing,
		rateLimitService:      rateLimit,
		billingCacheService:   billingCache,
		settingService:        setting,
		channelService:        channel,
		resolver:              resolver,
		tlsFPProfileService:   tlsProfiles,
		balanceNotifyService:  balanceNotify,
		fingerprintNormalizer: fingerprintNormalizer,
	}

	bridge := svc.bridgeGatewayService()

	require.NotNil(t, bridge)
	require.Equal(t, cfg, bridge.cfg)
	require.Equal(t, concurrency, bridge.concurrencyService)
	require.Equal(t, billing, bridge.billingService)
	require.Equal(t, rateLimit, bridge.rateLimitService)
	require.Equal(t, billingCache, bridge.billingCacheService)
	require.Equal(t, setting, bridge.settingService)
	require.Equal(t, channel, bridge.channelService)
	require.Equal(t, resolver, bridge.resolver)
	require.Equal(t, tlsProfiles, bridge.tlsFPProfileService)
	require.Equal(t, balanceNotify, bridge.balanceNotifyService)
	require.Equal(t, fingerprintNormalizer, bridge.fingerprintNormalizer)
}
