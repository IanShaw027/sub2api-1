//go:build unit

package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestDiffSettings_DetectsAccountSchedulingThresholdsChange(t *testing.T) {
	before := &service.SystemSettings{
		AccountSchedulingThresholds: map[string]int{
			service.PlatformOpenAI: 100,
		},
	}
	after := &service.SystemSettings{
		AccountSchedulingThresholds: map[string]int{
			service.PlatformOpenAI: 91,
		},
	}

	changed := diffSettings(before, after, nil, nil, nil, nil, UpdateSettingsRequest{})
	found := false
	for _, key := range changed {
		if key == service.SettingKeyAccountSchedulingThresholds {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %q in changes, got %v", service.SettingKeyAccountSchedulingThresholds, changed)
	}
}

func TestEqualAccountSchedulingThresholds_DefaultHundred(t *testing.T) {
	before := map[string]int{
		service.PlatformOpenAI: 100,
	}
	after := map[string]int{}
	if !equalAccountSchedulingThresholds(before, after) {
		t.Fatal("missing platform values should default to 100 and compare equal")
	}
}
