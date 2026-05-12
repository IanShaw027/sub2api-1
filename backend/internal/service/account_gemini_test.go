package service

import "testing"

func TestAccount_GeminiOAuthType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		account *Account
		want    string
	}{
		{
			name: "non_gemini_returns_empty",
			account: &Account{
				Platform: PlatformOpenAI,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"oauth_type": "code_assist",
				},
			},
			want: "",
		},
		{
			name: "non_oauth_returns_empty",
			account: &Account{
				Platform: PlatformGemini,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					"oauth_type": "code_assist",
				},
			},
			want: "",
		},
		{
			name: "explicit_code_assist",
			account: &Account{
				Platform: PlatformGemini,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"oauth_type": "code_assist",
					"project_id": "proj-1",
				},
			},
			want: "code_assist",
		},
		{
			name: "explicit_google_one",
			account: &Account{
				Platform: PlatformGemini,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"oauth_type": "google_one",
					"project_id": "proj-1",
				},
			},
			want: "google_one",
		},
		{
			name: "project_id_only_does_not_guess",
			account: &Account{
				Platform: PlatformGemini,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"project_id": "proj-1",
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.account.GeminiOAuthType(); got != tt.want {
				t.Fatalf("GeminiOAuthType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAccount_HasExplicitGeminiOAuthType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		account *Account
		want    bool
	}{
		{
			name: "explicit_type_present",
			account: &Account{
				Platform: PlatformGemini,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"oauth_type": "code_assist",
				},
			},
			want: true,
		},
		{
			name: "project_id_only_not_explicit",
			account: &Account{
				Platform: PlatformGemini,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"project_id": "proj-1",
				},
			},
			want: false,
		},
		{
			name: "extra_oauth_type_counts_as_explicit",
			account: &Account{
				Platform: PlatformGemini,
				Type:     AccountTypeOAuth,
				Extra: map[string]any{
					"oauth_type": "google_one",
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.account.HasExplicitGeminiOAuthType(); got != tt.want {
				t.Fatalf("HasExplicitGeminiOAuthType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAccount_GeminiOAuthTypeSafe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		account *Account
		want    string
	}{
		{
			name: "explicit_type_wins",
			account: &Account{
				Platform: PlatformGemini,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"oauth_type": "google_one",
					"tier_id":    "STANDARD",
				},
			},
			want: "google_one",
		},
		{
			name: "infer_code_assist_from_tier_only",
			account: &Account{
				Platform: PlatformGemini,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"tier_id": "STANDARD",
				},
			},
			want: "code_assist",
		},
		{
			name: "infer_google_one_from_tier_only",
			account: &Account{
				Platform: PlatformGemini,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"tier_id": "google_ai_pro",
				},
			},
			want: "google_one",
		},
		{
			name: "project_id_only_stays_unknown",
			account: &Account{
				Platform: PlatformGemini,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"project_id": "proj-1",
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.account.GeminiOAuthTypeSafe(); got != tt.want {
				t.Fatalf("GeminiOAuthTypeSafe() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAccount_IsGeminiCodeAssist(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		account *Account
		want    bool
	}{
		{
			name: "explicit_code_assist",
			account: &Account{
				Platform: PlatformGemini,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"oauth_type": "code_assist",
					"project_id": "proj-1",
				},
			},
			want: true,
		},
		{
			name: "google_one_not_code_assist",
			account: &Account{
				Platform: PlatformGemini,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"oauth_type": "google_one",
					"project_id": "proj-1",
				},
			},
			want: false,
		},
		{
			name: "project_id_only_not_code_assist",
			account: &Account{
				Platform: PlatformGemini,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"project_id": "proj-1",
				},
			},
			want: false,
		},
		{
			name: "non_oauth_not_code_assist",
			account: &Account{
				Platform: PlatformGemini,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					"oauth_type": "code_assist",
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.account.IsGeminiCodeAssist(); got != tt.want {
				t.Fatalf("IsGeminiCodeAssist() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAccount_UsesGeminiCLIProjectRouting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		account *Account
		want    bool
	}{
		{
			name: "explicit_code_assist",
			account: &Account{
				Platform: PlatformGemini,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"oauth_type": "code_assist",
				},
			},
			want: true,
		},
		{
			name: "google_one_does_not_use_code_assist_routing",
			account: &Account{
				Platform: PlatformGemini,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"oauth_type": "google_one",
				},
			},
			want: false,
		},
		{
			name: "non_oauth_not_routed",
			account: &Account{
				Platform: PlatformGemini,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					"oauth_type": "code_assist",
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.account.UsesGeminiCLIProjectRouting(); got != tt.want {
				t.Fatalf("UsesGeminiCLIProjectRouting() = %v, want %v", got, tt.want)
			}
		})
	}
}
