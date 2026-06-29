package service

import "testing"

func TestResolveImageRateMultiplier_GrokIndependentUsesCustomMultiplier(t *testing.T) {
	apiKey := &APIKey{
		GroupID: i64p(1),
		Group: &Group{
			ID:                   1,
			Platform:             PlatformGrok,
			ImageRateIndependent: true,
			ImageRateMultiplier:  0.42,
		},
	}

	got := resolveImageRateMultiplier(apiKey, 0.15)

	if got != 0.42 {
		t.Fatalf("expected custom image multiplier 0.42, got %v", got)
	}
}
