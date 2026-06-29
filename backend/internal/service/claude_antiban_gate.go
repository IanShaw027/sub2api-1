package service

import "context"

func shouldApplyClaudeAntiBanBodyTransformsForAccount(account *Account, antiBanEnabled bool) bool {
	if account == nil || !antiBanEnabled {
		return false
	}
	return normalizePlatform(account.Platform) == PlatformAnthropic && account.IsOAuth()
}

func (s *GatewayService) shouldApplyClaudeAntiBanBodyTransforms(ctx context.Context, account *Account) bool {
	if account == nil || s == nil || s.fingerprintNormalizer == nil {
		return false
	}
	return shouldApplyClaudeAntiBanBodyTransformsForAccount(
		account,
		s.fingerprintNormalizer.isAntiBanEnabledFor(account.Platform),
	)
}
