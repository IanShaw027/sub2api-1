package service

import "testing"

func TestIsCompatibleKiroModelMappingPair_CustomAliasPassthrough(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{
			name: "custom prefixed alias passes through to valid target",
			from: "anthropic-claude-opus-4-8",
			to:   "claude-opus-4.8",
			want: true,
		},
		{
			name: "unknown short alias passes through to valid target",
			from: "kirp-opus-4.8",
			to:   "claude-opus-4.8",
			want: true,
		},
		{
			name: "custom alias mapped to unresolvable target is rejected",
			from: "kirp-opus-4.8",
			to:   "totally-not-a-model",
			want: false,
		},
		{
			name: "known cross-family mapping stays blocked",
			from: "claude-opus-4",
			to:   "claude-sonnet-4.6",
			want: false,
		},
		{
			name: "same-family mapping still allowed",
			from: "claude-opus-4-6",
			to:   "claude-opus-4.8",
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isCompatibleKiroModelMappingPair(tc.from, tc.to); got != tc.want {
				t.Fatalf("isCompatibleKiroModelMappingPair(%q, %q) = %v, want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

func TestKiroAccountResolvesCustomAliasMapping(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformKiro,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"anthropic-claude-opus-4-8": "claude-opus-4.8",
			},
		},
	}

	mapping := account.GetModelMapping()
	if got := mapping["anthropic-claude-opus-4-8"]; got != "claude-opus-4.8" {
		t.Fatalf("custom alias not preserved in mapping: got %q, want %q", got, "claude-opus-4.8")
	}

	mapped, matched := account.ResolveMappedModel("anthropic-claude-opus-4-8")
	if !matched {
		t.Fatalf("expected custom alias to match account mapping")
	}
	if mapped != "claude-opus-4.8" {
		t.Fatalf("ResolveMappedModel = %q, want %q", mapped, "claude-opus-4.8")
	}

	if got := mapKiroModel(account, "anthropic-claude-opus-4-8"); got != "claude-opus-4.8" {
		t.Fatalf("mapKiroModel = %q, want %q", got, "claude-opus-4.8")
	}
}

func TestKiroAccountNormalizesClaude48HyphenAndDotAliases(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformKiro,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"claude-sonnet-4-6": "claude-sonnet-4.6",
				"claude-opus-4-6":   "claude-opus-4.6",
				"claude-opus-4-7":   "claude-opus-4.7",
				"claude-opus-4-8":   "claude-opus-4.8",
			},
		},
	}

	tests := map[string]string{
		"claude-sonnet-4-6": "claude-sonnet-4.6",
		"claude-opus-4-6":   "claude-opus-4.6",
		"claude-opus-4-7":   "claude-opus-4.7",
		"claude-opus-4-8":   "claude-opus-4.8",
		"claude-opus-4.8":   "claude-opus-4.8",
	}
	for requested, want := range tests {
		mapped, matched := account.ResolveMappedModel(requested)
		if !matched {
			t.Fatalf("ResolveMappedModel(%q) did not match", requested)
		}
		if mapped != want {
			t.Fatalf("ResolveMappedModel(%q) = %q, want %q", requested, mapped, want)
		}
	}
}
