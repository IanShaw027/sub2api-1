package service

import "testing"

func TestNormalizeVideoBillingTierOrDefault_LowResolutionDimensionsClampTo480p(t *testing.T) {
	if got := NormalizeVideoBillingTierOrDefault("640x360"); got != VideoBillingTier480p {
		t.Fatalf("NormalizeVideoBillingTierOrDefault(640x360) = %q, want %q", got, VideoBillingTier480p)
	}
}

func TestNormalizeVideoBillingTierOrDefault_1440pDimensionsClampTo4K(t *testing.T) {
	if got := NormalizeVideoBillingTierOrDefault("2560x1440"); got != VideoBillingTier4K {
		t.Fatalf("NormalizeVideoBillingTierOrDefault(2560x1440) = %q, want %q", got, VideoBillingTier4K)
	}
}
