package service

import (
	"encoding/json"
	"testing"
)

func TestGetBaseRPM(t *testing.T) {
	tests := []struct {
		name     string
		extra    map[string]any
		expected int
	}{
		{"nil extra", nil, 0},
		{"no key", map[string]any{}, 0},
		{"zero", map[string]any{"base_rpm": 0}, 0},
		{"int value", map[string]any{"base_rpm": 15}, 15},
		{"float value", map[string]any{"base_rpm": 15.0}, 15},
		{"string value", map[string]any{"base_rpm": "15"}, 15},
		{"negative value", map[string]any{"base_rpm": -5}, 0},
		{"int64 value", map[string]any{"base_rpm": int64(20)}, 20},
		{"json.Number value", map[string]any{"base_rpm": json.Number("25")}, 25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Account{Extra: tt.extra}
			if got := a.GetBaseRPM(); got != tt.expected {
				t.Errorf("GetBaseRPM() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestGetRPMStrategy(t *testing.T) {
	tests := []struct {
		name     string
		extra    map[string]any
		expected string
	}{
		{"nil extra", nil, "tiered"},
		{"no key", map[string]any{}, "tiered"},
		{"tiered", map[string]any{"rpm_strategy": "tiered"}, "tiered"},
		{"sticky_exempt", map[string]any{"rpm_strategy": "sticky_exempt"}, "sticky_exempt"},
		{"invalid", map[string]any{"rpm_strategy": "foobar"}, "tiered"},
		{"empty string fallback", map[string]any{"rpm_strategy": ""}, "tiered"},
		{"numeric value fallback", map[string]any{"rpm_strategy": 123}, "tiered"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Account{Extra: tt.extra}
			if got := a.GetRPMStrategy(); got != tt.expected {
				t.Errorf("GetRPMStrategy() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestCheckRPMSchedulability(t *testing.T) {
	tests := []struct {
		name        string
		concurrency int
		extra       map[string]any
		currentRPM  int
		expected    WindowCostSchedulability
	}{
		{"disabled", 0, map[string]any{}, 100, WindowCostSchedulable},
		{"green zone", 3, map[string]any{"base_rpm": 15}, 10, WindowCostSchedulable},
		{"yellow zone tiered", 3, map[string]any{"base_rpm": 15}, 15, WindowCostStickyOnly},
		{"red zone tiered", 3, map[string]any{"base_rpm": 15}, 18, WindowCostNotSchedulable},
		{"sticky_exempt at limit", 3, map[string]any{"base_rpm": 15, "rpm_strategy": "sticky_exempt"}, 15, WindowCostStickyOnly},
		{"sticky_exempt over limit", 3, map[string]any{"base_rpm": 15, "rpm_strategy": "sticky_exempt"}, 100, WindowCostStickyOnly},
		{"custom buffer", 3, map[string]any{"base_rpm": 10, "rpm_sticky_buffer": 5}, 14, WindowCostStickyOnly},
		{"custom buffer red", 3, map[string]any{"base_rpm": 10, "rpm_sticky_buffer": 5}, 15, WindowCostNotSchedulable},
		{"base_rpm=1 green", 1, map[string]any{"base_rpm": 1}, 0, WindowCostSchedulable},
		{"base_rpm=1 yellow (at limit)", 1, map[string]any{"base_rpm": 1}, 1, WindowCostStickyOnly},
		{"base_rpm=1 red (at limit+buffer)", 1, map[string]any{"base_rpm": 1}, 2, WindowCostNotSchedulable},
		{"negative currentRPM", 3, map[string]any{"base_rpm": 15}, -1, WindowCostSchedulable},
		{"base_rpm negative disabled", 3, map[string]any{"base_rpm": -5}, 10, WindowCostSchedulable},
		{"very high currentRPM", 3, map[string]any{"base_rpm": 10}, 9999, WindowCostNotSchedulable},
		{"sticky_exempt very high currentRPM", 3, map[string]any{"base_rpm": 10, "rpm_strategy": "sticky_exempt"}, 9999, WindowCostStickyOnly},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Account{Concurrency: tt.concurrency, Extra: tt.extra}
			if got := a.CheckRPMSchedulability(tt.currentRPM); got != tt.expected {
				t.Errorf("CheckRPMSchedulability(%d) = %d, want %d", tt.currentRPM, got, tt.expected)
			}
		})
	}
}

func TestGetRPMStickyBuffer(t *testing.T) {
	tests := []struct {
		name        string
		platform    string
		typ         string
		concurrency int
		extra       map[string]any
		expected    int
	}{
		{"nil extra", "", "", 0, nil, 0},
		{"no keys", "", "", 0, map[string]any{}, 0},
		{"base_rpm=0", "", "", 0, map[string]any{"base_rpm": 0}, 0},

		// max(EffectiveConcurrency(), max(baseRPM/5, 1)); never add max_sessions
		{"conc=3 sess=10 ignores sessions", "", "", 3, map[string]any{"base_rpm": 15, "max_sessions": 10}, 3},
		{"conc=2 sess=5 ignores sessions", "", "", 2, map[string]any{"base_rpm": 10, "max_sessions": 5}, 2},
		{"conc=3 sess=15 uses base/5 floor", "", "", 3, map[string]any{"base_rpm": 30, "max_sessions": 15}, 6},

		{"conc=0 base=15 uses EffectiveConcurrency fallback", "", "", 0, map[string]any{"base_rpm": 15}, 3},
		{"conc=0 base=10 uses EffectiveConcurrency over floor", "", "", 0, map[string]any{"base_rpm": 10}, 3},
		{"conc=0 base=1 uses EffectiveConcurrency over floor", "", "", 0, map[string]any{"base_rpm": 1}, 3},
		{"conc=0 base=4 uses EffectiveConcurrency over floor", "", "", 0, map[string]any{"base_rpm": 4}, 3},
		{"conc=1 base=15 uses floor 3", "", "", 1, map[string]any{"base_rpm": 15}, 3},

		{"anthropic oauth conc=0 uses EffectiveConcurrency 12", PlatformAnthropic, AccountTypeOAuth, 0, map[string]any{"base_rpm": 15}, 12},
		{"grok oauth conc=0 uses floor over EffectiveConcurrency 1", PlatformGrok, AccountTypeOAuth, 0, map[string]any{"base_rpm": 15}, 3},

		{"custom buffer=5", "", "", 3, map[string]any{"base_rpm": 10, "rpm_sticky_buffer": 5, "max_sessions": 10}, 5},
		{"custom buffer=0 fallback", "", "", 3, map[string]any{"base_rpm": 10, "rpm_sticky_buffer": 0, "max_sessions": 10}, 3},
		{"custom buffer negative fallback", "", "", 3, map[string]any{"base_rpm": 10, "rpm_sticky_buffer": -1, "max_sessions": 10}, 3},
		{"custom buffer with float", "", "", 3, map[string]any{"base_rpm": 10, "rpm_sticky_buffer": float64(7)}, 7},
		{"custom buffer=10000 accepted", "", "", 3, map[string]any{"base_rpm": 10, "rpm_sticky_buffer": 10000}, 10000},
		{"custom buffer over 10000 fallback", "", "", 3, map[string]any{"base_rpm": 10, "rpm_sticky_buffer": 10001}, 3},

		{"negative concurrency uses platform fallback", "", "", -5, map[string]any{"base_rpm": 15, "max_sessions": 10}, 3},
		{"negative maxSessions ignored", "", "", 3, map[string]any{"base_rpm": 15, "max_sessions": -5}, 3},

		{"conc=10 sess=5 ignores sessions", "", "", 10, map[string]any{"base_rpm": 10, "max_sessions": 5}, 10},

		{"json.Number base_rpm", "", "", 3, map[string]any{"base_rpm": json.Number("10"), "max_sessions": json.Number("5")}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Account{Platform: tt.platform, Type: tt.typ, Concurrency: tt.concurrency, Extra: tt.extra}
			if got := a.GetRPMStickyBuffer(); got != tt.expected {
				t.Errorf("GetRPMStickyBuffer() = %d, want %d", got, tt.expected)
			}
		})
	}
}
