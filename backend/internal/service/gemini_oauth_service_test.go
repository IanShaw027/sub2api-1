//go:build unit

package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// =====================
// 保留原有测试
// =====================

func TestGeminiOAuthService_GenerateAuthURL_RedirectURIStrategy(t *testing.T) {
	// NOTE: This test sets process env; it must not run in parallel.
	// The built-in Gemini CLI client secret is not embedded in this repository.
	// Tests set a dummy secret via env to simulate operator-provided configuration.
	t.Setenv(geminicli.GeminiCLIOAuthClientSecretEnv, "test-built-in-secret")

	type testCase struct {
		name          string
		cfg           *config.Config
		oauthType     string
		projectID     string
		wantClientID  string
		wantRedirect  string
		wantScope     string
		wantProjectID string
		wantErrSubstr string
	}

	tests := []testCase{
		{
			name: "google_one uses built-in client when not configured and redirects to upstream",
			cfg: &config.Config{
				Gemini: config.GeminiConfig{
					OAuth: config.GeminiOAuthConfig{},
				},
			},
			oauthType:     "google_one",
			wantClientID:  geminicli.GeminiCLIOAuthClientID,
			wantRedirect:  geminicli.GeminiCLIRedirectURI,
			wantScope:     geminicli.DefaultCodeAssistScopes,
			wantProjectID: "",
		},
		{
			name: "google_one still uses built-in client when custom client is configured",
			cfg: &config.Config{
				Gemini: config.GeminiConfig{
					OAuth: config.GeminiOAuthConfig{
						ClientID:     "custom-client-id",
						ClientSecret: "custom-client-secret",
					},
				},
			},
			oauthType:     "google_one",
			wantClientID:  geminicli.GeminiCLIOAuthClientID,
			wantRedirect:  geminicli.GeminiCLIRedirectURI,
			wantScope:     geminicli.DefaultCodeAssistScopes,
			wantProjectID: "",
		},
		{
			name: "code_assist uses custom client when fully configured",
			cfg: &config.Config{
				Gemini: config.GeminiConfig{
					OAuth: config.GeminiOAuthConfig{
						ClientID:     "custom-client-id",
						ClientSecret: "custom-client-secret",
					},
				},
			},
			oauthType:     "code_assist",
			projectID:     "my-gcp-project",
			wantClientID:  "custom-client-id",
			wantRedirect:  "https://example.com/auth/callback",
			wantScope:     geminicli.DefaultCodeAssistScopes,
			wantProjectID: "my-gcp-project",
		},
		{
			name: "missing oauth type is rejected instead of guessing from project",
			cfg: &config.Config{
				Gemini: config.GeminiConfig{
					OAuth: config.GeminiOAuthConfig{},
				},
			},
			projectID:     "my-gcp-project",
			wantErrSubstr: "missing oauth_type",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := NewGeminiOAuthService(nil, nil, nil, tt.cfg)
			got, err := svc.GenerateAuthURL(context.Background(), nil, "https://example.com/auth/callback", tt.projectID, tt.oauthType, "")
			if tt.wantErrSubstr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrSubstr)
				}
				if !strings.Contains(err.Error(), tt.wantErrSubstr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErrSubstr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("GenerateAuthURL returned error: %v", err)
			}

			parsed, err := url.Parse(got.AuthURL)
			if err != nil {
				t.Fatalf("failed to parse auth_url: %v", err)
			}
			q := parsed.Query()

			if gotState := q.Get("state"); gotState != got.State {
				t.Fatalf("state mismatch: query=%q result=%q", gotState, got.State)
			}
			if gotClientID := q.Get("client_id"); gotClientID != tt.wantClientID {
				t.Fatalf("client_id mismatch: got=%q want=%q", gotClientID, tt.wantClientID)
			}
			if gotRedirect := q.Get("redirect_uri"); gotRedirect != tt.wantRedirect {
				t.Fatalf("redirect_uri mismatch: got=%q want=%q", gotRedirect, tt.wantRedirect)
			}
			if gotScope := q.Get("scope"); gotScope != tt.wantScope {
				t.Fatalf("scope mismatch: got=%q want=%q", gotScope, tt.wantScope)
			}
			if gotProjectID := q.Get("project_id"); gotProjectID != tt.wantProjectID {
				t.Fatalf("project_id mismatch: got=%q want=%q", gotProjectID, tt.wantProjectID)
			}
		})
	}
}

func TestGeminiOAuthServiceGenerateAuthURLFailsWhenProxyMissing(t *testing.T) {
	t.Setenv(geminicli.GeminiCLIOAuthClientSecretEnv, "test-built-in-secret")
	proxyID := int64(404)
	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, &mockGeminiOAuthClient{}, nil, &config.Config{})

	_, err := svc.GenerateAuthURL(context.Background(), &proxyID, "https://example.com/auth/callback", "", "code_assist", "")

	if err == nil || !strings.Contains(err.Error(), "proxy not found") {
		t.Fatalf("expected proxy not found error, got %v", err)
	}
}

func TestGeminiOAuthServiceExchangeCodeUsesSessionProxyAndRejectsOverride(t *testing.T) {
	t.Setenv(geminicli.GeminiCLIOAuthClientSecretEnv, "test-built-in-secret")
	proxyID := int64(1)
	overrideProxyID := int64(2)
	var gotProxyURL string
	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*Proxy, error) {
			if id == proxyID {
				return &Proxy{ID: id, Protocol: "http", Host: "session.proxy", Port: 8080}, nil
			}
			if id == overrideProxyID {
				return &Proxy{ID: id, Protocol: "http", Host: "override.proxy", Port: 8080}, nil
			}
			return nil, fmt.Errorf("proxy not found")
		},
	}, &mockGeminiOAuthClient{
		exchangeCodeFunc: func(ctx context.Context, oauthType, code, codeVerifier, redirectURI, proxyURL string) (*geminicli.TokenResponse, error) {
			gotProxyURL = proxyURL
			return &geminicli.TokenResponse{AccessToken: "access", RefreshToken: "refresh", TokenType: "Bearer", ExpiresIn: 3600}, nil
		},
	}, &mockGeminiCodeAssistClient{
		loadCodeAssistFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
			return &geminicli.LoadCodeAssistResponse{CloudAICompanionProject: "project-1"}, nil
		},
	}, &config.Config{})

	result, err := svc.GenerateAuthURL(context.Background(), &proxyID, "https://example.com/auth/callback", "project-1", "code_assist", "")
	if err != nil {
		t.Fatalf("GenerateAuthURL returned error: %v", err)
	}

	_, err = svc.ExchangeCode(context.Background(), &GeminiExchangeCodeInput{
		SessionID: result.SessionID,
		Code:      "code-1",
		State:     result.State,
		ProxyID:   &overrideProxyID,
	})
	if err != nil {
		t.Fatalf("ExchangeCode returned error: %v", err)
	}
	if gotProxyURL != "http://session.proxy:8080" {
		t.Fatalf("ExchangeCode proxy = %q, want session proxy", gotProxyURL)
	}
}

func TestGeminiOAuthServiceExchangeCodeRejectsNilInput(t *testing.T) {
	t.Setenv(geminicli.GeminiCLIOAuthClientSecretEnv, "test-built-in-secret")

	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, &mockGeminiOAuthClient{}, &mockGeminiCodeAssistClient{}, &config.Config{})

	_, err := svc.ExchangeCode(context.Background(), nil)
	if err == nil {
		t.Fatal("expected nil input error")
	}
	if !strings.Contains(err.Error(), "oauth input is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =====================
// 新增测试：validateTierID
// =====================

func TestValidateTierID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tierID  string
		wantErr bool
	}{
		{name: "空字符串合法", tierID: "", wantErr: false},
		{name: "正常 tier_id", tierID: "google_one_free", wantErr: false},
		{name: "包含斜杠", tierID: "tier/sub", wantErr: false},
		{name: "包含连字符", tierID: "gcp-standard", wantErr: false},
		{name: "纯数字", tierID: "12345", wantErr: false},
		{name: "超长字符串（65个字符）", tierID: strings.Repeat("a", 65), wantErr: true},
		{name: "刚好64个字符", tierID: strings.Repeat("b", 64), wantErr: false},
		{name: "非法字符_空格", tierID: "tier id", wantErr: true},
		{name: "非法字符_中文", tierID: "tier_中文", wantErr: true},
		{name: "非法字符_特殊符号", tierID: "tier@id", wantErr: true},
		{name: "非法字符_感叹号", tierID: "tier!id", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateTierID(tt.tierID)
			if tt.wantErr && err == nil {
				t.Fatalf("期望返回错误，但返回 nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("不期望返回错误，但返回: %v", err)
			}
		})
	}
}

// =====================
// 新增测试：canonicalGeminiTierID
// =====================

func TestCanonicalGeminiTierID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		// 空值
		{name: "空字符串", raw: "", want: ""},
		{name: "纯空白", raw: "   ", want: ""},

		// 已规范化的值（直接返回）
		{name: "google_one_free", raw: "google_one_free", want: GeminiTierGoogleOneFree},
		{name: "google_ai_pro", raw: "google_ai_pro", want: GeminiTierGoogleAIPro},
		{name: "google_ai_ultra", raw: "google_ai_ultra", want: GeminiTierGoogleAIUltra},
		{name: "gcp_standard", raw: "gcp_standard", want: GeminiTierGCPStandard},
		{name: "gcp_enterprise", raw: "gcp_enterprise", want: GeminiTierGCPEnterprise},
		{name: "aistudio_free", raw: "aistudio_free", want: GeminiTierAIStudioFree},
		{name: "aistudio_paid", raw: "aistudio_paid", want: GeminiTierAIStudioPaid},
		{name: "google_one_unknown", raw: "google_one_unknown", want: GeminiTierGoogleOneUnknown},

		// 大小写不敏感
		{name: "Google_One_Free 大写", raw: "Google_One_Free", want: GeminiTierGoogleOneFree},
		{name: "GCP_STANDARD 全大写", raw: "GCP_STANDARD", want: GeminiTierGCPStandard},

		// legacy 映射: Google One
		{name: "AI_PREMIUM -> google_ai_pro", raw: "AI_PREMIUM", want: GeminiTierGoogleAIPro},
		{name: "FREE -> google_one_free", raw: "FREE", want: GeminiTierGoogleOneFree},
		{name: "GOOGLE_ONE_BASIC -> google_one_free", raw: "GOOGLE_ONE_BASIC", want: GeminiTierGoogleOneFree},
		{name: "GOOGLE_ONE_STANDARD -> google_one_free", raw: "GOOGLE_ONE_STANDARD", want: GeminiTierGoogleOneFree},
		{name: "GOOGLE_ONE_UNLIMITED -> google_ai_ultra", raw: "GOOGLE_ONE_UNLIMITED", want: GeminiTierGoogleAIUltra},
		{name: "GOOGLE_ONE_UNKNOWN -> google_one_unknown", raw: "GOOGLE_ONE_UNKNOWN", want: GeminiTierGoogleOneUnknown},
		{name: "free-tier -> google_one_free", raw: "free-tier", want: GeminiTierGoogleOneFree},
		{name: "g1-pro-tier -> google_ai_pro", raw: "g1-pro-tier", want: GeminiTierGoogleAIPro},
		{name: "g1-ultra-tier -> google_ai_ultra", raw: "g1-ultra-tier", want: GeminiTierGoogleAIUltra},

		// legacy 映射: Code Assist
		{name: "STANDARD -> gcp_standard", raw: "STANDARD", want: GeminiTierGCPStandard},
		{name: "PRO -> gcp_standard", raw: "PRO", want: GeminiTierGCPStandard},
		{name: "LEGACY -> gcp_standard", raw: "LEGACY", want: GeminiTierGCPStandard},
		{name: "ENTERPRISE -> gcp_enterprise", raw: "ENTERPRISE", want: GeminiTierGCPEnterprise},
		{name: "ULTRA -> gcp_enterprise", raw: "ULTRA", want: GeminiTierGCPEnterprise},

		// kebab-case
		{name: "standard-tier -> gcp_standard", raw: "standard-tier", want: GeminiTierGCPStandard},
		{name: "pro-tier -> gcp_standard", raw: "pro-tier", want: GeminiTierGCPStandard},
		{name: "ultra-tier -> gcp_enterprise", raw: "ultra-tier", want: GeminiTierGCPEnterprise},

		// 未知值
		{name: "unknown_value -> 空", raw: "unknown_value", want: ""},
		{name: "random-text -> 空", raw: "random-text", want: ""},

		// 带空白
		{name: "带前后空白", raw: "  google_one_free  ", want: GeminiTierGoogleOneFree},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := canonicalGeminiTierID(tt.raw)
			if got != tt.want {
				t.Fatalf("canonicalGeminiTierID(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// =====================
// 新增测试：canonicalGeminiTierIDForOAuthType
// =====================

func TestCanonicalGeminiTierIDForOAuthType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		oauthType string
		tierID    string
		want      string
	}{
		// google_one 类型过滤
		{name: "google_one + google_one_free", oauthType: "google_one", tierID: "google_one_free", want: GeminiTierGoogleOneFree},
		{name: "google_one + google_ai_pro", oauthType: "google_one", tierID: "google_ai_pro", want: GeminiTierGoogleAIPro},
		{name: "google_one + google_ai_ultra", oauthType: "google_one", tierID: "google_ai_ultra", want: GeminiTierGoogleAIUltra},
		{name: "google_one + free-tier", oauthType: "google_one", tierID: "free-tier", want: GeminiTierGoogleOneFree},
		{name: "google_one + g1-pro-tier", oauthType: "google_one", tierID: "g1-pro-tier", want: GeminiTierGoogleAIPro},
		{name: "google_one + g1-ultra-tier", oauthType: "google_one", tierID: "g1-ultra-tier", want: GeminiTierGoogleAIUltra},
		{name: "google_one + gcp_standard 被过滤", oauthType: "google_one", tierID: "gcp_standard", want: ""},
		{name: "google_one + aistudio_free 被过滤", oauthType: "google_one", tierID: "aistudio_free", want: ""},
		{name: "google_one + AI_PREMIUM 遗留映射", oauthType: "google_one", tierID: "AI_PREMIUM", want: GeminiTierGoogleAIPro},

		// code_assist 类型过滤
		{name: "code_assist + gcp_standard", oauthType: "code_assist", tierID: "gcp_standard", want: GeminiTierGCPStandard},
		{name: "code_assist + gcp_enterprise", oauthType: "code_assist", tierID: "gcp_enterprise", want: GeminiTierGCPEnterprise},
		{name: "code_assist + google_one_free 被过滤", oauthType: "code_assist", tierID: "google_one_free", want: ""},
		{name: "code_assist + aistudio_free 被过滤", oauthType: "code_assist", tierID: "aistudio_free", want: ""},
		{name: "code_assist + STANDARD 遗留映射", oauthType: "code_assist", tierID: "STANDARD", want: GeminiTierGCPStandard},
		{name: "code_assist + standard-tier kebab", oauthType: "code_assist", tierID: "standard-tier", want: GeminiTierGCPStandard},

		// 空值
		{name: "空 tierID", oauthType: "google_one", tierID: "", want: ""},
		{name: "空 oauthType + 有效 tierID", oauthType: "", tierID: "gcp_standard", want: GeminiTierGCPStandard},
		{name: "未知 oauthType 接受规范化值", oauthType: "unknown_type", tierID: "gcp_standard", want: GeminiTierGCPStandard},

		// oauthType 大小写和空白
		{name: "GOOGLE_ONE 大写", oauthType: "GOOGLE_ONE", tierID: "google_one_free", want: GeminiTierGoogleOneFree},
		{name: "oauthType 带空白", oauthType: "  code_assist  ", tierID: "gcp_standard", want: GeminiTierGCPStandard},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := canonicalGeminiTierIDForOAuthType(tt.oauthType, tt.tierID)
			if got != tt.want {
				t.Fatalf("canonicalGeminiTierIDForOAuthType(%q, %q) = %q, want %q", tt.oauthType, tt.tierID, got, tt.want)
			}
		})
	}
}

// =====================
// 新增测试：extractTierIDFromAllowedTiers
// =====================

func TestExtractTierIDFromAllowedTiers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		allowedTiers []geminicli.AllowedTier
		want         string
	}{
		{
			name:         "nil 列表返回 LEGACY",
			allowedTiers: nil,
			want:         "LEGACY",
		},
		{
			name:         "空列表返回 LEGACY",
			allowedTiers: []geminicli.AllowedTier{},
			want:         "LEGACY",
		},
		{
			name: "有 IsDefault 的 tier",
			allowedTiers: []geminicli.AllowedTier{
				{ID: "STANDARD", IsDefault: false},
				{ID: "PRO", IsDefault: true},
				{ID: "ENTERPRISE", IsDefault: false},
			},
			want: "PRO",
		},
		{
			name: "没有 IsDefault 取第一个非空",
			allowedTiers: []geminicli.AllowedTier{
				{ID: "STANDARD", IsDefault: false},
				{ID: "ENTERPRISE", IsDefault: false},
			},
			want: "STANDARD",
		},
		{
			name: "IsDefault 的 ID 为空，取第一个非空",
			allowedTiers: []geminicli.AllowedTier{
				{ID: "", IsDefault: true},
				{ID: "PRO", IsDefault: false},
			},
			want: "PRO",
		},
		{
			name: "所有 ID 都为空返回 LEGACY",
			allowedTiers: []geminicli.AllowedTier{
				{ID: "", IsDefault: false},
				{ID: "   ", IsDefault: false},
			},
			want: "LEGACY",
		},
		{
			name: "ID 带空白会被 trim",
			allowedTiers: []geminicli.AllowedTier{
				{ID: "  STANDARD  ", IsDefault: true},
			},
			want: "STANDARD",
		},
		{
			name: "单个 tier 且 IsDefault",
			allowedTiers: []geminicli.AllowedTier{
				{ID: "ENTERPRISE", IsDefault: true},
			},
			want: "ENTERPRISE",
		},
		{
			name: "单个 tier 非 IsDefault",
			allowedTiers: []geminicli.AllowedTier{
				{ID: "ENTERPRISE", IsDefault: false},
			},
			want: "ENTERPRISE",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractTierIDFromAllowedTiers(tt.allowedTiers)
			if got != tt.want {
				t.Fatalf("extractTierIDFromAllowedTiers() = %q, want %q", got, tt.want)
			}
		})
	}
}

// =====================
// 新增测试：isNonRetryableGeminiOAuthError
// =====================

func TestIsNonRetryableGeminiOAuthError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "invalid_grant", err: fmt.Errorf("error: invalid_grant"), want: true},
		{name: "invalid_client", err: fmt.Errorf("oauth error: invalid_client"), want: true},
		{name: "unauthorized_client", err: fmt.Errorf("unauthorized_client: mismatch"), want: true},
		{name: "access_denied", err: fmt.Errorf("access_denied by user"), want: true},
		{name: "普通网络错误", err: fmt.Errorf("connection timeout"), want: false},
		{name: "HTTP 500 错误", err: fmt.Errorf("server error 500"), want: false},
		{name: "空错误信息", err: fmt.Errorf(""), want: false},
		{name: "包含 invalid 但不是完整匹配", err: fmt.Errorf("invalid request"), want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isNonRetryableGeminiOAuthError(tt.err)
			if got != tt.want {
				t.Fatalf("isNonRetryableGeminiOAuthError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// =====================
// 新增测试：BuildAccountCredentials
// =====================

func TestGeminiOAuthService_BuildAccountCredentials(t *testing.T) {
	t.Parallel()

	svc := NewGeminiOAuthService(nil, nil, nil, &config.Config{})
	defer svc.Stop()

	t.Run("完整字段", func(t *testing.T) {
		t.Parallel()
		tokenInfo := &GeminiTokenInfo{
			AccessToken:  "access-123",
			RefreshToken: "refresh-456",
			IDToken:      "id-token-789",
			ExpiresIn:    3600,
			ExpiresAt:    1700000000,
			TokenType:    "Bearer",
			Scope:        "openid email",
			ProjectID:    "my-project",
			Email:        "user@example.com",
			AuthID:       "subject-123",
			Name:         "Example User",
			PlanName:     "Gemini Code Assist in Google One AI Pro",
			TierID:       "STANDARD",
			OAuthType:    "code_assist",
			Extra: map[string]any{
				"gemini_paid_tier_id":             "g1-pro-tier",
				"gemini_has_onboarded_previously": true,
				"gemini_available_credits": []geminicli.AvailableCredit{
					{CreditType: "GOOGLE_ONE_AI", CreditAmount: "100"},
				},
			},
		}

		creds := svc.BuildAccountCredentials(tokenInfo)

		assertCredStr(t, creds, "access_token", "access-123")
		assertCredStr(t, creds, "refresh_token", "refresh-456")
		assertCredStr(t, creds, "id_token", "id-token-789")
		assertCredStr(t, creds, "token_type", "Bearer")
		assertCredStr(t, creds, "scope", "openid email")
		assertCredStr(t, creds, "project_id", "my-project")
		assertCredStr(t, creds, "email", "user@example.com")
		assertCredStr(t, creds, "auth_id", "subject-123")
		assertCredStr(t, creds, "subject", "subject-123")
		assertCredStr(t, creds, "name", "Example User")
		assertCredStr(t, creds, "plan_name", "Gemini Code Assist in Google One AI Pro")
		assertCredStr(t, creds, "tier_id", "STANDARD")
		assertCredStr(t, creds, "oauth_type", "code_assist")
		assertCredStr(t, creds, "expires_at", "1700000000")

		if _, ok := creds["gemini_paid_tier_id"]; !ok {
			t.Fatal("extra 字段 gemini_paid_tier_id 未包含在 creds 中")
		}
		if _, ok := creds["gemini_available_credits"]; !ok {
			t.Fatal("extra 字段 gemini_available_credits 未包含在 creds 中")
		}
	})

	t.Run("最小字段（仅 access_token 和 expires_at）", func(t *testing.T) {
		t.Parallel()
		tokenInfo := &GeminiTokenInfo{
			AccessToken: "token-only",
			ExpiresAt:   1700000000,
		}

		creds := svc.BuildAccountCredentials(tokenInfo)

		assertCredStr(t, creds, "access_token", "token-only")
		assertCredStr(t, creds, "expires_at", "1700000000")

		// 可选字段不应存在
		for _, key := range []string{"refresh_token", "id_token", "token_type", "scope", "project_id", "email", "auth_id", "subject", "name", "plan_name", "tier_id", "oauth_type"} {
			if _, ok := creds[key]; ok {
				t.Fatalf("不应包含空字段 %q", key)
			}
		}
	})

	t.Run("无效 tier_id 被静默跳过", func(t *testing.T) {
		t.Parallel()
		tokenInfo := &GeminiTokenInfo{
			AccessToken: "token",
			ExpiresAt:   1700000000,
			TierID:      "tier with spaces",
		}

		creds := svc.BuildAccountCredentials(tokenInfo)

		if _, ok := creds["tier_id"]; ok {
			t.Fatal("无效 tier_id 不应被存入 creds")
		}
	})

	t.Run("超长 tier_id 被静默跳过", func(t *testing.T) {
		t.Parallel()
		tokenInfo := &GeminiTokenInfo{
			AccessToken: "token",
			ExpiresAt:   1700000000,
			TierID:      strings.Repeat("x", 65),
		}

		creds := svc.BuildAccountCredentials(tokenInfo)

		if _, ok := creds["tier_id"]; ok {
			t.Fatal("超长 tier_id 不应被存入 creds")
		}
	})

	t.Run("无 extra 字段", func(t *testing.T) {
		t.Parallel()
		tokenInfo := &GeminiTokenInfo{
			AccessToken:  "token",
			ExpiresAt:    1700000000,
			RefreshToken: "rt",
		}

		creds := svc.BuildAccountCredentials(tokenInfo)

		// 基础字段 + 显式清空状态字段，避免旧状态残留
		if len(creds) != 5 { // access_token, expires_at, refresh_token, gemini_status, gemini_status_reason
			t.Fatalf("creds 字段数量不匹配: got=%d want=5, keys=%v", len(creds), credKeys(creds))
		}
		assertCredStr(t, creds, "gemini_status", "")
		assertCredStr(t, creds, "gemini_status_reason", "")
	})

	t.Run("空状态字段会显式清空避免残留", func(t *testing.T) {
		t.Parallel()
		tokenInfo := &GeminiTokenInfo{
			AccessToken: "token",
			ExpiresAt:   1700000000,
		}

		creds := svc.BuildAccountCredentials(tokenInfo)

		assertCredStr(t, creds, "gemini_status", "")
		assertCredStr(t, creds, "gemini_status_reason", "")
	})
}

func TestBuildGeminiCodeAssistExtra(t *testing.T) {
	t.Parallel()

	hasOnboarded := true
	extra := buildGeminiCodeAssistExtra(&geminicli.LoadCodeAssistResponse{
		CurrentTier: &geminicli.TierInfo{
			ID:                     "g1-pro-tier",
			Name:                   "Google One AI Pro",
			HasOnboardedPreviously: &hasOnboarded,
		},
		PaidTier: &geminicli.TierInfo{
			ID:   "g1-pro-tier",
			Name: "Gemini Code Assist in Google One AI Pro",
			AvailableCredits: []geminicli.AvailableCredit{
				{CreditType: "GOOGLE_ONE_AI", CreditAmount: "100"},
			},
		},
	})

	assertCredStr(t, extra, "gemini_current_tier_id", "g1-pro-tier")
	assertCredStr(t, extra, "gemini_current_tier_name", "Google One AI Pro")
	assertCredStr(t, extra, "gemini_paid_tier_id", "g1-pro-tier")
	assertCredStr(t, extra, "gemini_paid_tier_name", "Gemini Code Assist in Google One AI Pro")
	if onboarded, ok := extra["gemini_has_onboarded_previously"].(bool); !ok || !onboarded {
		t.Fatalf("gemini_has_onboarded_previously mismatch: %#v", extra["gemini_has_onboarded_previously"])
	}
	if _, ok := extra["gemini_available_credits"]; !ok {
		t.Fatal("gemini_available_credits should be present")
	}
}

func TestExtractEmailFromGeminiIDToken(t *testing.T) {
	t.Parallel()

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"email":"user@example.com","email_verified":true}`))

	got := extractGeminiProfileFromIDToken(header + "." + payload + ".").Email
	if got != "user@example.com" {
		t.Fatalf("email mismatch: got=%q", got)
	}
}

func TestGeminiTokenScopeHasUserInfoEmail(t *testing.T) {
	t.Parallel()

	if !geminiTokenScopeHasUserInfo("https://www.googleapis.com/auth/cloud-platform https://www.googleapis.com/auth/userinfo.email") {
		t.Fatal("expected userinfo.email scope to be detected")
	}
	if geminiTokenScopeHasUserInfo("https://www.googleapis.com/auth/cloud-platform") {
		t.Fatal("did not expect userinfo.email scope")
	}
}

// =====================
// 新增测试：GetOAuthConfig
// =====================

func TestGeminiOAuthService_GetOAuthConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		cfg         *config.Config
		wantEnabled bool
	}{
		{
			name: "无自定义 OAuth 客户端",
			cfg: &config.Config{
				Gemini: config.GeminiConfig{
					OAuth: config.GeminiOAuthConfig{},
				},
			},
			wantEnabled: false,
		},
		{
			name: "仅 ClientID 无 ClientSecret",
			cfg: &config.Config{
				Gemini: config.GeminiConfig{
					OAuth: config.GeminiOAuthConfig{
						ClientID: "custom-id",
					},
				},
			},
			wantEnabled: false,
		},
		{
			name: "仅 ClientSecret 无 ClientID",
			cfg: &config.Config{
				Gemini: config.GeminiConfig{
					OAuth: config.GeminiOAuthConfig{
						ClientSecret: "custom-secret",
					},
				},
			},
			wantEnabled: false,
		},
		{
			name: "使用内置 Gemini CLI ClientID（不算自定义）",
			cfg: &config.Config{
				Gemini: config.GeminiConfig{
					OAuth: config.GeminiOAuthConfig{
						ClientID:     geminicli.GeminiCLIOAuthClientID,
						ClientSecret: "some-secret",
					},
				},
			},
			wantEnabled: false,
		},
		{
			name: "自定义 OAuth 客户端（非内置 ID）",
			cfg: &config.Config{
				Gemini: config.GeminiConfig{
					OAuth: config.GeminiOAuthConfig{
						ClientID:     "my-custom-client-id",
						ClientSecret: "my-custom-client-secret",
					},
				},
			},
			wantEnabled: false,
		},
		{
			name: "带空白的自定义客户端",
			cfg: &config.Config{
				Gemini: config.GeminiConfig{
					OAuth: config.GeminiOAuthConfig{
						ClientID:     "  my-custom-client-id  ",
						ClientSecret: "  my-custom-client-secret  ",
					},
				},
			},
			wantEnabled: false,
		},
		{
			name: "纯空白字符串不算配置",
			cfg: &config.Config{
				Gemini: config.GeminiConfig{
					OAuth: config.GeminiOAuthConfig{
						ClientID:     "   ",
						ClientSecret: "   ",
					},
				},
			},
			wantEnabled: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := NewGeminiOAuthService(nil, nil, nil, tt.cfg)
			defer svc.Stop()

			result := svc.GetOAuthConfig()
			if result.AIStudioOAuthEnabled != tt.wantEnabled {
				t.Fatalf("AIStudioOAuthEnabled = %v, want %v", result.AIStudioOAuthEnabled, tt.wantEnabled)
			}
			// RequiredRedirectURIs 始终包含 AI Studio redirect URI
			if len(result.RequiredRedirectURIs) != 1 || result.RequiredRedirectURIs[0] != geminicli.AIStudioOAuthRedirectURI {
				t.Fatalf("RequiredRedirectURIs 不匹配: got=%v", result.RequiredRedirectURIs)
			}
		})
	}
}

// =====================
// 新增测试：GeminiOAuthService.Stop
// =====================

func TestGeminiOAuthService_Stop_NoPanic(t *testing.T) {
	t.Parallel()

	svc := NewGeminiOAuthService(nil, nil, nil, &config.Config{})

	// 调用 Stop 不应 panic
	svc.Stop()
	// 多次调用也不应 panic
	svc.Stop()
}

// =====================
// mock: GeminiOAuthClient
// =====================

type mockGeminiOAuthClient struct {
	exchangeCodeFunc func(ctx context.Context, oauthType, code, codeVerifier, redirectURI, proxyURL string) (*geminicli.TokenResponse, error)
	refreshTokenFunc func(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error)
}

func (m *mockGeminiOAuthClient) ExchangeCode(ctx context.Context, oauthType, code, codeVerifier, redirectURI, proxyURL string) (*geminicli.TokenResponse, error) {
	if m.exchangeCodeFunc != nil {
		return m.exchangeCodeFunc(ctx, oauthType, code, codeVerifier, redirectURI, proxyURL)
	}
	panic("ExchangeCode not implemented")
}

func (m *mockGeminiOAuthClient) RefreshToken(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error) {
	if m.refreshTokenFunc != nil {
		return m.refreshTokenFunc(ctx, oauthType, refreshToken, proxyURL)
	}
	panic("RefreshToken not implemented")
}

// =====================
// mock: GeminiCliCodeAssistClient
// =====================

type mockGeminiCodeAssistClient struct {
	loadCodeAssistFunc func(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error)
	onboardUserFunc    func(ctx context.Context, accessToken, proxyURL string, req *geminicli.OnboardUserRequest) (*geminicli.OnboardUserResponse, error)
	getOperationFunc   func(ctx context.Context, accessToken, proxyURL, name string) (*geminicli.OnboardUserResponse, error)
	retrieveQuotaFunc  func(ctx context.Context, accessToken, proxyURL string, req *geminicli.RetrieveUserQuotaRequest) (*geminicli.RetrieveUserQuotaResponse, error)
}

func (m *mockGeminiCodeAssistClient) LoadCodeAssist(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
	if m.loadCodeAssistFunc != nil {
		return m.loadCodeAssistFunc(ctx, accessToken, proxyURL, req)
	}
	panic("LoadCodeAssist not implemented")
}

func (m *mockGeminiCodeAssistClient) OnboardUser(ctx context.Context, accessToken, proxyURL string, req *geminicli.OnboardUserRequest) (*geminicli.OnboardUserResponse, error) {
	if m.onboardUserFunc != nil {
		return m.onboardUserFunc(ctx, accessToken, proxyURL, req)
	}
	panic("OnboardUser not implemented")
}

func (m *mockGeminiCodeAssistClient) GetOperation(ctx context.Context, accessToken, proxyURL, name string) (*geminicli.OnboardUserResponse, error) {
	if m.getOperationFunc != nil {
		return m.getOperationFunc(ctx, accessToken, proxyURL, name)
	}
	panic("GetOperation not implemented")
}

func (m *mockGeminiCodeAssistClient) RetrieveUserQuota(ctx context.Context, accessToken, proxyURL string, req *geminicli.RetrieveUserQuotaRequest) (*geminicli.RetrieveUserQuotaResponse, error) {
	if m.retrieveQuotaFunc != nil {
		return m.retrieveQuotaFunc(ctx, accessToken, proxyURL, req)
	}
	panic("RetrieveUserQuota not implemented")
}

// =====================
// mock: ProxyRepository (最小实现)
// =====================

type mockGeminiProxyRepo struct {
	getByIDFunc func(ctx context.Context, id int64) (*Proxy, error)
}

func (m *mockGeminiProxyRepo) Create(ctx context.Context, proxy *Proxy) error { panic("not impl") }
func (m *mockGeminiProxyRepo) GetByID(ctx context.Context, id int64) (*Proxy, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, fmt.Errorf("proxy not found")
}
func (m *mockGeminiProxyRepo) ListByIDs(ctx context.Context, ids []int64) ([]Proxy, error) {
	panic("not impl")
}
func (m *mockGeminiProxyRepo) Update(ctx context.Context, proxy *Proxy) error { panic("not impl") }
func (m *mockGeminiProxyRepo) Delete(ctx context.Context, id int64) error     { panic("not impl") }
func (m *mockGeminiProxyRepo) List(ctx context.Context, params pagination.PaginationParams) ([]Proxy, *pagination.PaginationResult, error) {
	panic("not impl")
}
func (m *mockGeminiProxyRepo) ListWithFilters(ctx context.Context, params pagination.PaginationParams, protocol, status, search string) ([]Proxy, *pagination.PaginationResult, error) {
	panic("not impl")
}
func (m *mockGeminiProxyRepo) ListWithFiltersAndAccountCount(ctx context.Context, params pagination.PaginationParams, protocol, status, search string) ([]ProxyWithAccountCount, *pagination.PaginationResult, error) {
	panic("not impl")
}
func (m *mockGeminiProxyRepo) ListActive(ctx context.Context) ([]Proxy, error) { panic("not impl") }
func (m *mockGeminiProxyRepo) ListActiveWithAccountCount(ctx context.Context) ([]ProxyWithAccountCount, error) {
	panic("not impl")
}
func (m *mockGeminiProxyRepo) ExistsByHostPortAuth(ctx context.Context, host string, port int, username, password string) (bool, error) {
	panic("not impl")
}
func (m *mockGeminiProxyRepo) CountAccountsByProxyID(ctx context.Context, proxyID int64) (int64, error) {
	panic("not impl")
}
func (m *mockGeminiProxyRepo) ListAccountSummariesByProxyID(ctx context.Context, proxyID int64) ([]ProxyAccountSummary, error) {
	panic("not impl")
}
func (m *mockGeminiProxyRepo) SweepExpiredProxies(ctx context.Context, now time.Time) (int64, error) {
	panic("not impl")
}
func (m *mockGeminiProxyRepo) ListAllForFallback(ctx context.Context) ([]Proxy, error) {
	panic("not impl")
}
func (m *mockGeminiProxyRepo) CountExpired(ctx context.Context) (int64, error) {
	panic("not impl")
}
func (m *mockGeminiProxyRepo) CountExpiringSoon(ctx context.Context, now time.Time) (int64, error) {
	panic("not impl")
}

// =====================
// 新增测试：GeminiOAuthService.RefreshToken（含重试逻辑）
// =====================

func TestGeminiOAuthService_RefreshToken_Success(t *testing.T) {
	t.Parallel()

	client := &mockGeminiOAuthClient{
		refreshTokenFunc: func(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error) {
			return &geminicli.TokenResponse{
				AccessToken:  "new-access",
				RefreshToken: "new-refresh",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
				Scope:        "openid",
			}, nil
		},
	}

	svc := NewGeminiOAuthService(nil, client, nil, &config.Config{})
	defer svc.Stop()

	info, err := svc.RefreshToken(context.Background(), "code_assist", "old-refresh", "")
	if err != nil {
		t.Fatalf("RefreshToken 返回错误: %v", err)
	}
	if info.AccessToken != "new-access" {
		t.Fatalf("AccessToken 不匹配: got=%q", info.AccessToken)
	}
	if info.RefreshToken != "new-refresh" {
		t.Fatalf("RefreshToken 不匹配: got=%q", info.RefreshToken)
	}
	if info.ExpiresAt == 0 {
		t.Fatal("ExpiresAt 不应为 0")
	}
}

func TestGeminiOAuthService_RefreshToken_NonRetryableError(t *testing.T) {
	t.Parallel()

	client := &mockGeminiOAuthClient{
		refreshTokenFunc: func(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error) {
			return nil, fmt.Errorf("invalid_grant: token revoked")
		},
	}

	svc := NewGeminiOAuthService(nil, client, nil, &config.Config{})
	defer svc.Stop()

	_, err := svc.RefreshToken(context.Background(), "code_assist", "revoked-token", "")
	if err == nil {
		t.Fatal("RefreshToken 应返回错误（不可重试的 invalid_grant）")
	}
	if !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("错误应包含 invalid_grant: got=%q", err.Error())
	}
}

func TestGeminiOAuthService_RefreshToken_RetryableError(t *testing.T) {
	t.Parallel()

	callCount := 0
	client := &mockGeminiOAuthClient{
		refreshTokenFunc: func(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error) {
			callCount++
			if callCount <= 2 {
				return nil, fmt.Errorf("temporary network error")
			}
			return &geminicli.TokenResponse{
				AccessToken: "recovered",
				ExpiresIn:   3600,
			}, nil
		},
	}

	svc := NewGeminiOAuthService(nil, client, nil, &config.Config{})
	defer svc.Stop()

	info, err := svc.RefreshToken(context.Background(), "code_assist", "rt", "")
	if err != nil {
		t.Fatalf("RefreshToken 应在重试后成功: %v", err)
	}
	if info.AccessToken != "recovered" {
		t.Fatalf("AccessToken 不匹配: got=%q", info.AccessToken)
	}
	if callCount < 3 {
		t.Fatalf("应至少调用 3 次（2 次失败 + 1 次成功）: got=%d", callCount)
	}
}

// =====================
// 新增测试：GeminiOAuthService.RefreshAccountToken
// =====================

func TestGeminiOAuthService_RefreshAccountToken_NotGeminiOAuth(t *testing.T) {
	t.Parallel()

	svc := NewGeminiOAuthService(nil, nil, nil, &config.Config{})
	defer svc.Stop()

	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
	}

	_, err := svc.RefreshAccountToken(context.Background(), account)
	if err == nil {
		t.Fatal("应返回错误（非 Gemini OAuth 账号）")
	}
	if !strings.Contains(err.Error(), "not a Gemini OAuth account") {
		t.Fatalf("错误信息不匹配: got=%q", err.Error())
	}
}

func TestGeminiOAuthService_RefreshAccountToken_NilAccount(t *testing.T) {
	t.Parallel()

	svc := NewGeminiOAuthService(nil, nil, nil, &config.Config{})
	defer svc.Stop()

	_, err := svc.RefreshAccountToken(context.Background(), nil)
	if err == nil {
		t.Fatal("应返回错误（nil account）")
	}
	if !strings.Contains(err.Error(), "not a Gemini OAuth account") {
		t.Fatalf("错误信息不匹配: got=%q", err.Error())
	}
}

func TestGeminiOAuthService_RefreshAccountToken_NoRefreshToken(t *testing.T) {
	t.Parallel()

	svc := NewGeminiOAuthService(nil, nil, nil, &config.Config{})
	defer svc.Stop()

	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "at",
			"oauth_type":   "code_assist",
		},
	}

	_, err := svc.RefreshAccountToken(context.Background(), account)
	if err == nil {
		t.Fatal("应返回错误（无 refresh_token）")
	}
	if !strings.Contains(err.Error(), "no refresh token") {
		t.Fatalf("错误信息不匹配: got=%q", err.Error())
	}
}

func TestGeminiOAuthService_RefreshAccountToken_AIStudio(t *testing.T) {
	t.Parallel()

	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, nil, nil, &config.Config{})
	defer svc.Stop()

	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "old-at",
			"refresh_token": "old-rt",
			"oauth_type":    "ai_studio",
			"tier_id":       "aistudio_free",
			"email":         "existing@example.com",
		},
	}

	_, err := svc.RefreshAccountToken(context.Background(), account)
	if err == nil {
		t.Fatal("legacy ai_studio 账号应被拒绝")
	}
	if !strings.Contains(err.Error(), "missing oauth_type and unable to infer") {
		t.Fatalf("错误信息不匹配: got=%q", err.Error())
	}
}

func TestGeminiOAuthService_RefreshAccountToken_CodeAssist_WithProjectID(t *testing.T) {
	t.Parallel()

	client := &mockGeminiOAuthClient{
		refreshTokenFunc: func(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error) {
			return &geminicli.TokenResponse{
				AccessToken:  "refreshed",
				RefreshToken: "new-rt",
				ExpiresIn:    3600,
			}, nil
		},
	}

	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, client, nil, &config.Config{})
	defer svc.Stop()

	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "old-at",
			"refresh_token": "old-rt",
			"oauth_type":    "code_assist",
			"project_id":    "my-project",
			"tier_id":       "gcp_standard",
		},
	}

	info, err := svc.RefreshAccountToken(context.Background(), account)
	if err != nil {
		t.Fatalf("RefreshAccountToken 返回错误: %v", err)
	}
	if info.ProjectID != "my-project" {
		t.Fatalf("ProjectID 应保留: got=%q", info.ProjectID)
	}
	if info.TierID != GeminiTierGCPStandard {
		t.Fatalf("TierID 不匹配: got=%q want=%q", info.TierID, GeminiTierGCPStandard)
	}
	if info.OAuthType != "code_assist" {
		t.Fatalf("OAuthType 不匹配: got=%q", info.OAuthType)
	}
}

func TestGeminiOAuthService_RefreshAccountToken_InferOAuthTypeFromTier(t *testing.T) {
	t.Parallel()

	client := &mockGeminiOAuthClient{
		refreshTokenFunc: func(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error) {
			if oauthType != "code_assist" {
				t.Errorf("应按 tier 推断 oauthType=code_assist: got=%q", oauthType)
			}
			return &geminicli.TokenResponse{
				AccessToken: "refreshed",
				ExpiresIn:   3600,
			}, nil
		},
	}

	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, client, nil, &config.Config{})
	defer svc.Stop()

	// 无 oauth_type 凭据的旧账号
	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "old-rt",
			"project_id":    "proj",
			"tier_id":       "STANDARD",
		},
	}

	info, err := svc.RefreshAccountToken(context.Background(), account)
	if err != nil {
		t.Fatalf("RefreshAccountToken 返回错误: %v", err)
	}
	if info.OAuthType != "code_assist" {
		t.Fatalf("OAuthType 应推断为 code_assist: got=%q", info.OAuthType)
	}
}

func TestGeminiOAuthService_RefreshAccountToken_InferGoogleOneOAuthType(t *testing.T) {
	t.Parallel()

	client := &mockGeminiOAuthClient{
		refreshTokenFunc: func(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error) {
			if oauthType != "google_one" {
				t.Errorf("应按 tier 推断 oauthType=google_one: got=%q", oauthType)
			}
			return &geminicli.TokenResponse{
				AccessToken: "refreshed",
				ExpiresIn:   3600,
			}, nil
		},
	}

	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, client, nil, &config.Config{})
	defer svc.Stop()

	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "old-rt",
			"tier_id":       "google_ai_pro",
		},
	}

	info, err := svc.RefreshAccountToken(context.Background(), account)
	if err != nil {
		t.Fatalf("RefreshAccountToken 返回错误: %v", err)
	}
	if info.OAuthType != "google_one" {
		t.Fatalf("OAuthType 应推断为 google_one: got=%q", info.OAuthType)
	}
}

func TestGeminiOAuthService_RefreshAccountToken_RejectsUnknownOAuthTypeHeuristic(t *testing.T) {
	t.Parallel()

	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, nil, nil, &config.Config{})
	defer svc.Stop()

	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "old-rt",
			"project_id":    "proj",
		},
	}

	_, err := svc.RefreshAccountToken(context.Background(), account)
	if err == nil {
		t.Fatal("缺少 oauth_type/tier_id 的旧账号应返回错误")
	}
	if !strings.Contains(err.Error(), "unable to infer") {
		t.Fatalf("错误信息不匹配: got=%q", err.Error())
	}
}

func TestResolveGeminiOAuthType_DoesNotGuessFromProjectID(t *testing.T) {
	t.Parallel()

	if got := resolveGeminiOAuthType("", "", "project-1", ""); got != "" {
		t.Fatalf("resolveGeminiOAuthType should not guess from project_id: got=%q", got)
	}
	if got := resolveGeminiOAuthType("", "aistudio_free", "", ""); got != "" {
		t.Fatalf("resolveGeminiOAuthType should not resurrect ai_studio: got=%q", got)
	}
}

func TestGeminiOAuthService_RefreshAccountToken_WithProxy(t *testing.T) {
	t.Parallel()

	proxyRepo := &mockGeminiProxyRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*Proxy, error) {
			return &Proxy{
				Protocol: "http",
				Host:     "proxy.test",
				Port:     3128,
			}, nil
		},
	}

	client := &mockGeminiOAuthClient{
		refreshTokenFunc: func(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error) {
			if proxyURL != "http://proxy.test:3128" {
				t.Errorf("proxyURL 不匹配: got=%q", proxyURL)
			}
			return &geminicli.TokenResponse{
				AccessToken: "refreshed",
				ExpiresIn:   3600,
			}, nil
		},
	}

	svc := NewGeminiOAuthService(proxyRepo, client, nil, &config.Config{})
	defer svc.Stop()

	proxyID := int64(5)
	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		ProxyID:  &proxyID,
		Credentials: map[string]any{
			"refresh_token": "rt",
			"oauth_type":    "code_assist",
			"project_id":    "proj",
		},
	}

	_, err := svc.RefreshAccountToken(context.Background(), account)
	if err != nil {
		t.Fatalf("RefreshAccountToken 返回错误: %v", err)
	}
}

func TestGeminiOAuthService_RefreshAccountToken_CodeAssist_NoProjectID_AutoDetect(t *testing.T) {
	t.Parallel()

	client := &mockGeminiOAuthClient{
		refreshTokenFunc: func(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error) {
			return &geminicli.TokenResponse{
				AccessToken: "at",
				ExpiresIn:   3600,
			}, nil
		},
	}

	codeAssist := &mockGeminiCodeAssistClient{
		loadCodeAssistFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
			return &geminicli.LoadCodeAssistResponse{
				CloudAICompanionProject: "auto-project-123",
				CurrentTier:             &geminicli.TierInfo{ID: "STANDARD"},
			}, nil
		},
		retrieveQuotaFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.RetrieveUserQuotaRequest) (*geminicli.RetrieveUserQuotaResponse, error) {
			return &geminicli.RetrieveUserQuotaResponse{}, nil
		},
	}

	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, client, codeAssist, &config.Config{})
	defer svc.Stop()

	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "rt",
			"oauth_type":    "code_assist",
			// 无 project_id，触发 fetchProjectID
		},
	}

	info, err := svc.RefreshAccountToken(context.Background(), account)
	if err != nil {
		t.Fatalf("RefreshAccountToken 返回错误: %v", err)
	}
	if info.ProjectID != "auto-project-123" {
		t.Fatalf("ProjectID 应为自动检测值: got=%q", info.ProjectID)
	}
	if canonicalGeminiTierIDForOAuthType("code_assist", info.TierID) != GeminiTierGCPStandard {
		t.Fatalf("TierID 不匹配: got=%q", info.TierID)
	}
}

func TestGeminiOAuthService_RefreshAccountToken_CodeAssist_RefreshesQuotaSnapshot(t *testing.T) {
	t.Parallel()

	client := &mockGeminiOAuthClient{
		refreshTokenFunc: func(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error) {
			return &geminicli.TokenResponse{
				AccessToken: "at",
				ExpiresIn:   3600,
			}, nil
		},
	}

	var gotQuotaReq *geminicli.RetrieveUserQuotaRequest
	codeAssist := &mockGeminiCodeAssistClient{
		retrieveQuotaFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.RetrieveUserQuotaRequest) (*geminicli.RetrieveUserQuotaResponse, error) {
			gotQuotaReq = req
			return &geminicli.RetrieveUserQuotaResponse{
				Buckets: []geminicli.RetrieveUserQuotaBucket{
					{
						ModelID:           "gemini-2.5-pro",
						RemainingAmount:   "42",
						RemainingFraction: func() *float64 { v := 0.42; return &v }(),
						ResetTime:         "2026-05-08T00:00:00Z",
						TokenType:         "REQUEST",
					},
				},
			}, nil
		},
	}

	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, client, codeAssist, &config.Config{})
	defer svc.Stop()

	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "rt",
			"oauth_type":    "code_assist",
			"project_id":    "proj-123",
			"tier_id":       "STANDARD",
		},
	}

	info, err := svc.RefreshAccountToken(context.Background(), account)
	if err != nil {
		t.Fatalf("RefreshAccountToken 返回错误: %v", err)
	}
	if gotQuotaReq == nil {
		t.Fatal("应调用 RetrieveUserQuota")
	}
	if gotQuotaReq.Project != "proj-123" {
		t.Fatalf("quota request project 不匹配: got=%q", gotQuotaReq.Project)
	}
	if gotQuotaReq.UserAgent != geminicli.GeminiCLIUserAgent {
		t.Fatalf("quota request userAgent 不匹配: got=%q", gotQuotaReq.UserAgent)
	}
	if info.UsageRaw == nil {
		t.Fatal("应写入 UsageRaw")
	}
	if _, ok := info.Extra["gemini_usage_raw"]; !ok {
		t.Fatal("应写入 gemini_usage_raw extra")
	}
	if got := info.Extra["quota_query_last_error"]; got != "" {
		t.Fatalf("成功后应清空 quota_query_last_error: got=%v", got)
	}
}

func TestGeminiOAuthService_RefreshAccountToken_CodeAssist_NoProjectID_ReturnsRegisteredTierError(t *testing.T) {
	t.Parallel()

	client := &mockGeminiOAuthClient{
		refreshTokenFunc: func(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error) {
			return &geminicli.TokenResponse{
				AccessToken: "at",
				ExpiresIn:   3600,
			}, nil
		},
	}

	// 返回有 currentTier 但无 cloudaicompanionProject 的响应，
	// 使 fetchProjectID 走"已注册用户"路径（尝试 Cloud Resource Manager -> 失败 -> 返回错误），
	// 避免走 onboardUser 路径（5 次重试 x 2 秒 = 10 秒超时）
	codeAssist := &mockGeminiCodeAssistClient{
		loadCodeAssistFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
			return &geminicli.LoadCodeAssistResponse{
				CurrentTier: &geminicli.TierInfo{ID: "STANDARD"},
				// 无 CloudAICompanionProject
			}, nil
		},
	}

	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, client, codeAssist, &config.Config{})
	defer svc.Stop()

	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "rt",
			"oauth_type":    "code_assist",
		},
	}

	info, err := svc.RefreshAccountToken(context.Background(), account)
	if err == nil {
		t.Fatalf("应返回 companion project 缺失错误: %#v", info)
	}
	if !strings.Contains(err.Error(), "registered_tier_missing_companion_project") {
		t.Fatalf("错误信息不匹配: %v", err)
	}
}

func TestGeminiOAuthService_RefreshAccountToken_CodeAssist_Quota403SetsForbiddenStatus(t *testing.T) {
	t.Parallel()

	client := &mockGeminiOAuthClient{
		refreshTokenFunc: func(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error) {
			return &geminicli.TokenResponse{
				AccessToken: "at",
				ExpiresIn:   3600,
			}, nil
		},
	}

	codeAssist := &mockGeminiCodeAssistClient{
		retrieveQuotaFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.RetrieveUserQuotaRequest) (*geminicli.RetrieveUserQuotaResponse, error) {
			return nil, fmt.Errorf("retrieveUserQuota failed: status 403, body: forbidden")
		},
	}

	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, client, codeAssist, &config.Config{})
	defer svc.Stop()

	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "rt",
			"oauth_type":    "code_assist",
			"project_id":    "proj-123",
			"tier_id":       "STANDARD",
		},
	}

	info, err := svc.RefreshAccountToken(context.Background(), account)
	if err != nil {
		t.Fatalf("RefreshAccountToken 返回错误: %v", err)
	}
	if info.Status != "forbidden" {
		t.Fatalf("Status 应标记 forbidden: got=%q", info.Status)
	}
	if !strings.Contains(info.StatusReason, "status 403") {
		t.Fatalf("StatusReason 应保留 403 错误: got=%q", info.StatusReason)
	}
	if got := info.Extra["quota_query_last_error"]; got == nil || got == "" {
		t.Fatalf("应写入 quota_query_last_error: got=%v", got)
	}
	if got := info.Extra["usage_updated_at"]; got != "" {
		t.Fatalf("403 时应清空 usage_updated_at: got=%v", got)
	}
}

func TestGeminiOAuthService_RefreshAccountToken_CodeAssist_ValidationRequiredReturnsError(t *testing.T) {
	t.Parallel()

	client := &mockGeminiOAuthClient{
		refreshTokenFunc: func(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error) {
			return &geminicli.TokenResponse{
				AccessToken: "at",
				ExpiresIn:   3600,
			}, nil
		},
	}

	codeAssist := &mockGeminiCodeAssistClient{
		loadCodeAssistFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
			return &geminicli.LoadCodeAssistResponse{
				IneligibleTiers: []geminicli.IneligibleTier{
					{
						ReasonCode:             geminicli.IneligibleTierReasonCodeValidationRequired,
						ValidationErrorMessage: "Account verification required",
						ValidationURL:          "https://accounts.google.com/verify",
					},
				},
			}, nil
		},
	}

	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, client, codeAssist, &config.Config{})
	defer svc.Stop()

	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "rt",
			"oauth_type":    "code_assist",
		},
	}

	_, err := svc.RefreshAccountToken(context.Background(), account)
	if err == nil {
		t.Fatal("RefreshAccountToken 应返回 validation_required 错误")
	}
	if !strings.Contains(err.Error(), "validation_required:") {
		t.Fatalf("错误信息应包含 validation_required: got=%q", err.Error())
	}
}

func TestGeminiOAuthService_RefreshAccountToken_CodeAssist_ValidationRequiredWithoutURLStillReturnsValidationError(t *testing.T) {
	t.Parallel()

	client := &mockGeminiOAuthClient{
		refreshTokenFunc: func(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error) {
			return &geminicli.TokenResponse{
				AccessToken: "at",
				ExpiresIn:   3600,
			}, nil
		},
	}

	codeAssist := &mockGeminiCodeAssistClient{
		loadCodeAssistFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
			return &geminicli.LoadCodeAssistResponse{
				IneligibleTiers: []geminicli.IneligibleTier{
					{
						ReasonCode:             geminicli.IneligibleTierReasonCodeValidationRequired,
						ValidationErrorMessage: "Account verification required",
					},
				},
			}, nil
		},
	}

	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, client, codeAssist, &config.Config{})
	defer svc.Stop()

	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "rt",
			"oauth_type":    "code_assist",
		},
	}

	_, err := svc.RefreshAccountToken(context.Background(), account)
	if err == nil {
		t.Fatal("RefreshAccountToken 应返回 validation_required 错误")
	}
	if !strings.Contains(err.Error(), "validation_required:") {
		t.Fatalf("错误信息应包含 validation_required: got=%q", err.Error())
	}
}

func TestGeminiOAuthService_ExchangeCode_CodeAssist_NoProjectID_Fails(t *testing.T) {
	t.Setenv(geminicli.GeminiCLIOAuthClientSecretEnv, "test-built-in-secret")

	client := &mockGeminiOAuthClient{
		exchangeCodeFunc: func(ctx context.Context, oauthType, code, codeVerifier, redirectURI, proxyURL string) (*geminicli.TokenResponse, error) {
			return &geminicli.TokenResponse{
				AccessToken:  "at",
				RefreshToken: "rt",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			}, nil
		},
	}
	codeAssist := &mockGeminiCodeAssistClient{
		loadCodeAssistFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
			return &geminicli.LoadCodeAssistResponse{
				CurrentTier: &geminicli.TierInfo{ID: "STANDARD"},
			}, nil
		},
	}
	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, client, codeAssist, &config.Config{})
	defer svc.Stop()

	result, err := svc.GenerateAuthURL(context.Background(), nil, "https://example.com/auth/callback", "", "code_assist", "")
	if err != nil {
		t.Fatalf("GenerateAuthURL 返回错误: %v", err)
	}

	_, err = svc.ExchangeCode(context.Background(), &GeminiExchangeCodeInput{
		SessionID: result.SessionID,
		State:     result.State,
		Code:      "code-1",
		OAuthType: "code_assist",
	})
	if err == nil {
		t.Fatal("ExchangeCode 应返回 project_id 缺失错误")
	}
	if !strings.Contains(err.Error(), "registered_tier_missing_companion_project") {
		t.Fatalf("错误信息应包含 registered_tier_missing_companion_project: got=%q", err.Error())
	}
}

func TestGeminiOAuthService_ExchangeCode_GoogleOne_OnboardOperationPollingReturnsProjectID(t *testing.T) {
	client := &mockGeminiOAuthClient{
		exchangeCodeFunc: func(ctx context.Context, oauthType, code, codeVerifier, redirectURI, proxyURL string) (*geminicli.TokenResponse, error) {
			return &geminicli.TokenResponse{
				AccessToken:  "at",
				RefreshToken: "rt",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			}, nil
		},
	}

	var onboardReq *geminicli.OnboardUserRequest
	getOperationCalls := 0
	codeAssist := &mockGeminiCodeAssistClient{
		loadCodeAssistFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
			return &geminicli.LoadCodeAssistResponse{
				AllowedTiers: []geminicli.AllowedTier{{ID: "free-tier", IsDefault: true}},
			}, nil
		},
		onboardUserFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.OnboardUserRequest) (*geminicli.OnboardUserResponse, error) {
			onboardReq = req
			return &geminicli.OnboardUserResponse{
				Name: "operations/123",
				Done: false,
			}, nil
		},
		getOperationFunc: func(ctx context.Context, accessToken, proxyURL, name string) (*geminicli.OnboardUserResponse, error) {
			getOperationCalls++
			if name != "operations/123" {
				t.Fatalf("operation name mismatch: got=%q", name)
			}
			return &geminicli.OnboardUserResponse{
				Name: "operations/123",
				Done: true,
				Response: &geminicli.OnboardUserResultData{
					CloudAICompanionProject: map[string]any{"id": "awesome-height-482718-e9"},
				},
			}, nil
		},
	}
	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, client, codeAssist, &config.Config{})
	defer svc.Stop()

	result, err := svc.GenerateAuthURL(context.Background(), nil, "https://example.com/auth/callback", "", "google_one", "")
	if err != nil {
		t.Fatalf("GenerateAuthURL 返回错误: %v", err)
	}

	info, err := svc.ExchangeCode(context.Background(), &GeminiExchangeCodeInput{
		SessionID: result.SessionID,
		State:     result.State,
		Code:      "code-1",
		OAuthType: "google_one",
	})
	if err != nil {
		t.Fatalf("ExchangeCode 不应返回错误: %v", err)
	}
	if info.ProjectID != "awesome-height-482718-e9" {
		t.Fatalf("ProjectID 应来自 operation 轮询结果: got=%q", info.ProjectID)
	}
	if getOperationCalls != 1 {
		t.Fatalf("应轮询一次 operation: got=%d", getOperationCalls)
	}
	if onboardReq == nil {
		t.Fatal("onboardUser request should be captured")
	}
	if onboardReq.TierID != "free-tier" {
		t.Fatalf("unexpected onboard tier: got=%q", onboardReq.TierID)
	}
	if onboardReq.CloudAICompanionProject != "" {
		t.Fatalf("free-tier onboard request should not set companion project: got=%q", onboardReq.CloudAICompanionProject)
	}
	if onboardReq.Metadata.DuetProject != "" {
		t.Fatalf("free-tier onboard request should not set duetProject: got=%q", onboardReq.Metadata.DuetProject)
	}
}

func TestGeminiOAuthService_ExchangeCode_CodeAssist_ProjectHintDoesNotOverrideDetectedCompanionProject(t *testing.T) {
	t.Setenv(geminicli.GeminiCLIOAuthClientSecretEnv, "test-built-in-secret")

	client := &mockGeminiOAuthClient{
		exchangeCodeFunc: func(ctx context.Context, oauthType, code, codeVerifier, redirectURI, proxyURL string) (*geminicli.TokenResponse, error) {
			return &geminicli.TokenResponse{
				AccessToken:  "at",
				RefreshToken: "rt",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			}, nil
		},
	}

	var gotLoadReq *geminicli.LoadCodeAssistRequest
	codeAssist := &mockGeminiCodeAssistClient{
		loadCodeAssistFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
			gotLoadReq = req
			return &geminicli.LoadCodeAssistResponse{
				CloudAICompanionProject: "server-companion-project",
				CurrentTier:             &geminicli.TierInfo{ID: "STANDARD"},
			}, nil
		},
	}

	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, client, codeAssist, &config.Config{})
	defer svc.Stop()

	result, err := svc.GenerateAuthURL(context.Background(), nil, "https://example.com/auth/callback", "my-gcp-project", "code_assist", "")
	if err != nil {
		t.Fatalf("GenerateAuthURL 返回错误: %v", err)
	}

	info, err := svc.ExchangeCode(context.Background(), &GeminiExchangeCodeInput{
		SessionID: result.SessionID,
		State:     result.State,
		Code:      "code-1",
		OAuthType: "code_assist",
	})
	if err != nil {
		t.Fatalf("ExchangeCode 不应返回错误: %v", err)
	}
	if gotLoadReq == nil {
		t.Fatal("LoadCodeAssist request should be captured")
	}
	if gotLoadReq.CloudAICompanionProject != "my-gcp-project" {
		t.Fatalf("request should carry project hint: got=%q", gotLoadReq.CloudAICompanionProject)
	}
	if info.ProjectID != "server-companion-project" {
		t.Fatalf("ProjectID 应以上游 companion project 为准: got=%q", info.ProjectID)
	}
}

func TestGeminiOAuthService_ExchangeCode_CodeAssist_UsesProjectHintWhenTierExistsButProjectMissing(t *testing.T) {
	t.Setenv(geminicli.GeminiCLIOAuthClientSecretEnv, "test-built-in-secret")

	client := &mockGeminiOAuthClient{
		exchangeCodeFunc: func(ctx context.Context, oauthType, code, codeVerifier, redirectURI, proxyURL string) (*geminicli.TokenResponse, error) {
			return &geminicli.TokenResponse{
				AccessToken:  "at",
				RefreshToken: "rt",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			}, nil
		},
	}

	var gotLoadReq *geminicli.LoadCodeAssistRequest
	onboardCalled := false
	codeAssist := &mockGeminiCodeAssistClient{
		loadCodeAssistFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
			gotLoadReq = req
			return &geminicli.LoadCodeAssistResponse{
				CurrentTier: &geminicli.TierInfo{ID: "STANDARD"},
			}, nil
		},
		onboardUserFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.OnboardUserRequest) (*geminicli.OnboardUserResponse, error) {
			onboardCalled = true
			return nil, fmt.Errorf("onboard should not be called for registered tier fallback")
		},
	}

	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, client, codeAssist, &config.Config{})
	defer svc.Stop()

	result, err := svc.GenerateAuthURL(context.Background(), nil, "https://example.com/auth/callback", "my-hint-project", "code_assist", "")
	if err != nil {
		t.Fatalf("GenerateAuthURL 返回错误: %v", err)
	}

	info, err := svc.ExchangeCode(context.Background(), &GeminiExchangeCodeInput{
		SessionID: result.SessionID,
		State:     result.State,
		Code:      "code-1",
		OAuthType: "code_assist",
	})
	if err != nil {
		t.Fatalf("ExchangeCode 不应返回错误: %v", err)
	}
	if gotLoadReq == nil {
		t.Fatal("LoadCodeAssist request should be captured")
	}
	if gotLoadReq.CloudAICompanionProject != "my-hint-project" {
		t.Fatalf("request should carry project hint: got=%q", gotLoadReq.CloudAICompanionProject)
	}
	if info.ProjectID != "my-hint-project" {
		t.Fatalf("应回退使用 project hint: got=%q", info.ProjectID)
	}
	if info.TierID != "STANDARD" {
		t.Fatalf("TierID 不匹配: got=%q", info.TierID)
	}
	if onboardCalled {
		t.Fatal("registered tier fallback should not call onboardUser")
	}
}

func TestGeminiOAuthService_ExchangeCode_CodeAssist_ValidationRequiredErrorBubblesUp(t *testing.T) {
	t.Setenv(geminicli.GeminiCLIOAuthClientSecretEnv, "test-built-in-secret")

	client := &mockGeminiOAuthClient{
		exchangeCodeFunc: func(ctx context.Context, oauthType, code, codeVerifier, redirectURI, proxyURL string) (*geminicli.TokenResponse, error) {
			return &geminicli.TokenResponse{
				AccessToken:  "at",
				RefreshToken: "rt",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			}, nil
		},
	}

	codeAssist := &mockGeminiCodeAssistClient{
		loadCodeAssistFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
			return &geminicli.LoadCodeAssistResponse{
				IneligibleTiers: []geminicli.IneligibleTier{
					{
						ReasonCode:             geminicli.IneligibleTierReasonCodeValidationRequired,
						ValidationErrorMessage: "Account verification required",
						ValidationURL:          "https://accounts.google.com/verify",
					},
				},
			}, nil
		},
	}

	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, client, codeAssist, &config.Config{})
	defer svc.Stop()

	result, err := svc.GenerateAuthURL(context.Background(), nil, "https://example.com/auth/callback", "", "code_assist", "")
	if err != nil {
		t.Fatalf("GenerateAuthURL 返回错误: %v", err)
	}

	_, err = svc.ExchangeCode(context.Background(), &GeminiExchangeCodeInput{
		SessionID: result.SessionID,
		State:     result.State,
		Code:      "code-1",
		OAuthType: "code_assist",
	})
	if err == nil {
		t.Fatal("ExchangeCode 应返回 validation_required 错误")
	}
	if !strings.Contains(err.Error(), "validation_required:") {
		t.Fatalf("错误信息应包含 validation_required: got=%q", err.Error())
	}
	if !strings.Contains(err.Error(), "validation_url=https://accounts.google.com/verify") {
		t.Fatalf("错误信息应包含 validation_url: got=%q", err.Error())
	}
}

func TestGeminiOAuthService_ExchangeCode_CodeAssist_ValidationRequiredWithoutURLStillBubblesUp(t *testing.T) {
	t.Setenv(geminicli.GeminiCLIOAuthClientSecretEnv, "test-built-in-secret")

	client := &mockGeminiOAuthClient{
		exchangeCodeFunc: func(ctx context.Context, oauthType, code, codeVerifier, redirectURI, proxyURL string) (*geminicli.TokenResponse, error) {
			return &geminicli.TokenResponse{
				AccessToken:  "at",
				RefreshToken: "rt",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			}, nil
		},
	}

	codeAssist := &mockGeminiCodeAssistClient{
		loadCodeAssistFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
			return &geminicli.LoadCodeAssistResponse{
				IneligibleTiers: []geminicli.IneligibleTier{
					{
						ReasonCode:             geminicli.IneligibleTierReasonCodeValidationRequired,
						ValidationErrorMessage: "Account verification required",
					},
				},
			}, nil
		},
	}

	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, client, codeAssist, &config.Config{})
	defer svc.Stop()

	result, err := svc.GenerateAuthURL(context.Background(), nil, "https://example.com/auth/callback", "", "code_assist", "")
	if err != nil {
		t.Fatalf("GenerateAuthURL 返回错误: %v", err)
	}

	_, err = svc.ExchangeCode(context.Background(), &GeminiExchangeCodeInput{
		SessionID: result.SessionID,
		State:     result.State,
		Code:      "code-1",
		OAuthType: "code_assist",
	})
	if err == nil {
		t.Fatal("ExchangeCode 应返回 validation_required 错误")
	}
	if !strings.Contains(err.Error(), "validation_required:") {
		t.Fatalf("错误信息应包含 validation_required: got=%q", err.Error())
	}
}

func TestGeminiOAuthService_ExchangeCode_GoogleOne_IneligibleTierIncludesReasonCode(t *testing.T) {
	client := &mockGeminiOAuthClient{
		exchangeCodeFunc: func(ctx context.Context, oauthType, code, codeVerifier, redirectURI, proxyURL string) (*geminicli.TokenResponse, error) {
			return &geminicli.TokenResponse{
				AccessToken:  "at",
				RefreshToken: "rt",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			}, nil
		},
	}

	codeAssist := &mockGeminiCodeAssistClient{
		loadCodeAssistFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
			return &geminicli.LoadCodeAssistResponse{
				IneligibleTiers: []geminicli.IneligibleTier{
					{
						ReasonCode:    geminicli.IneligibleTierReasonCodeRestrictedAge,
						ReasonMessage: "Your current account is not eligible for Gemini Code Assist for individuals.",
					},
				},
			}, nil
		},
	}

	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, client, codeAssist, &config.Config{})
	defer svc.Stop()

	result, err := svc.GenerateAuthURL(context.Background(), nil, "https://example.com/auth/callback", "", "google_one", "")
	if err != nil {
		t.Fatalf("GenerateAuthURL 返回错误: %v", err)
	}

	_, err = svc.ExchangeCode(context.Background(), &GeminiExchangeCodeInput{
		SessionID: result.SessionID,
		State:     result.State,
		Code:      "code-1",
		OAuthType: "google_one",
	})
	if err == nil {
		t.Fatal("ExchangeCode 应返回 ineligible_tier 错误")
	}
	if !strings.Contains(err.Error(), "ineligible_tier[RESTRICTED_AGE]:") {
		t.Fatalf("错误信息应包含 reason code: got=%q", err.Error())
	}
}

func TestGeminiOAuthService_ExchangeCode_CodeAssist_OnboardMissingProjectFallsBackToHint(t *testing.T) {
	t.Setenv(geminicli.GeminiCLIOAuthClientSecretEnv, "test-built-in-secret")

	client := &mockGeminiOAuthClient{
		exchangeCodeFunc: func(ctx context.Context, oauthType, code, codeVerifier, redirectURI, proxyURL string) (*geminicli.TokenResponse, error) {
			return &geminicli.TokenResponse{
				AccessToken:  "at",
				RefreshToken: "rt",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			}, nil
		},
	}

	var onboardReq *geminicli.OnboardUserRequest
	codeAssist := &mockGeminiCodeAssistClient{
		loadCodeAssistFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
			return &geminicli.LoadCodeAssistResponse{
				AllowedTiers: []geminicli.AllowedTier{{ID: "standard-tier", IsDefault: true}},
			}, nil
		},
		onboardUserFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.OnboardUserRequest) (*geminicli.OnboardUserResponse, error) {
			onboardReq = req
			return &geminicli.OnboardUserResponse{Done: true}, nil
		},
	}

	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, client, codeAssist, &config.Config{})
	defer svc.Stop()

	result, err := svc.GenerateAuthURL(context.Background(), nil, "https://example.com/auth/callback", "hint-project", "code_assist", "")
	if err != nil {
		t.Fatalf("GenerateAuthURL 返回错误: %v", err)
	}

	info, err := svc.ExchangeCode(context.Background(), &GeminiExchangeCodeInput{
		SessionID: result.SessionID,
		State:     result.State,
		Code:      "code-1",
		OAuthType: "code_assist",
	})
	if err != nil {
		t.Fatalf("ExchangeCode 不应返回错误: %v", err)
	}
	if onboardReq == nil {
		t.Fatal("onboardUser request should be captured")
	}
	if onboardReq.CloudAICompanionProject != "hint-project" {
		t.Fatalf("onboard request should carry project hint: got=%q", onboardReq.CloudAICompanionProject)
	}
	if info.ProjectID != "hint-project" {
		t.Fatalf("应在 onboard 未返回项目时回退使用 hint: got=%q", info.ProjectID)
	}
	if info.TierID != "standard-tier" {
		t.Fatalf("TierID 不匹配: got=%q", info.TierID)
	}
}

func TestGeminiPlanNameForToken_PrefersUpstreamRawName(t *testing.T) {
	t.Parallel()

	planName := geminiPlanNameForToken("STANDARD", map[string]any{
		"gemini_paid_tier_name": "Gemini Code Assist Standard",
	})
	if planName != "Gemini Code Assist Standard" {
		t.Fatalf("PlanName 应优先使用上游 paid tier name: got=%q", planName)
	}

	planName = geminiPlanNameForToken("STANDARD", map[string]any{
		"gemini_current_tier_name": "Current Tier Name",
	})
	if planName != "Current Tier Name" {
		t.Fatalf("PlanName 应在无 paid tier name 时回退到 current tier name: got=%q", planName)
	}

	planName = geminiPlanNameForToken("STANDARD", nil)
	if planName != "STANDARD" {
		t.Fatalf("PlanName 应在无上游名称时回退到原始 tier_id: got=%q", planName)
	}
}

func TestGeminiOAuthService_RefreshAccountToken_GoogleOne_NoTierID_DefaultsFree(t *testing.T) {
	t.Parallel()

	client := &mockGeminiOAuthClient{
		refreshTokenFunc: func(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error) {
			return &geminicli.TokenResponse{
				AccessToken: "at",
				ExpiresIn:   3600,
			}, nil
		},
	}

	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, client, nil, &config.Config{})
	defer svc.Stop()

	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "rt",
			"oauth_type":    "google_one",
			"project_id":    "proj",
			// 无 tier_id
		},
	}

	info, err := svc.RefreshAccountToken(context.Background(), account)
	if err != nil {
		t.Fatalf("RefreshAccountToken 返回错误: %v", err)
	}
	if info.TierID != GeminiTierGoogleOneFree {
		t.Fatalf("TierID 应为默认 free: got=%q", info.TierID)
	}
}

func TestGeminiOAuthService_RefreshAccountToken_UnauthorizedClient_NoLegacyFallback(t *testing.T) {
	t.Parallel()

	client := &mockGeminiOAuthClient{
		refreshTokenFunc: func(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error) {
			return nil, fmt.Errorf("unauthorized_client: client mismatch")
		},
	}

	cfg := &config.Config{
		Gemini: config.GeminiConfig{
			OAuth: config.GeminiOAuthConfig{
				ClientID:     "custom-id",
				ClientSecret: "custom-secret",
			},
		},
	}

	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, client, nil, cfg)
	defer svc.Stop()

	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "rt",
			"oauth_type":    "code_assist",
			"project_id":    "proj",
			"tier_id":       "gcp_standard",
		},
	}

	info, err := svc.RefreshAccountToken(context.Background(), account)
	if err == nil {
		t.Fatalf("legacy fallback 已移除，不应成功: %#v", info)
	}
	if !strings.Contains(err.Error(), "OAuth client mismatch") {
		t.Fatalf("错误应包含 OAuth client mismatch: got=%q", err.Error())
	}
}

func TestGeminiOAuthService_RefreshAccountToken_UnauthorizedClient_NoFallback(t *testing.T) {
	t.Parallel()

	client := &mockGeminiOAuthClient{
		refreshTokenFunc: func(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error) {
			return nil, fmt.Errorf("unauthorized_client: client mismatch")
		},
	}

	// 无自定义 OAuth 客户端，无法 fallback
	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, client, nil, &config.Config{})
	defer svc.Stop()

	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "rt",
			"oauth_type":    "code_assist",
			"project_id":    "proj",
		},
	}

	_, err := svc.RefreshAccountToken(context.Background(), account)
	if err == nil {
		t.Fatal("应返回错误（无 fallback）")
	}
	if !strings.Contains(err.Error(), "OAuth client mismatch") {
		t.Fatalf("错误应包含 OAuth client mismatch: got=%q", err.Error())
	}
}

// =====================
// 新增测试：GeminiOAuthService.ExchangeCode
// =====================

func TestGeminiOAuthService_ExchangeCode_SessionNotFound(t *testing.T) {
	t.Parallel()

	svc := NewGeminiOAuthService(nil, nil, nil, &config.Config{})
	defer svc.Stop()

	_, err := svc.ExchangeCode(context.Background(), &GeminiExchangeCodeInput{
		SessionID: "nonexistent",
		State:     "some-state",
		Code:      "some-code",
	})
	if err == nil {
		t.Fatal("应返回错误（session 不存在）")
	}
	if !strings.Contains(err.Error(), "session not found") {
		t.Fatalf("错误信息不匹配: got=%q", err.Error())
	}
}

func TestGeminiOAuthService_ExchangeCode_InvalidState(t *testing.T) {
	t.Parallel()

	svc := NewGeminiOAuthService(nil, nil, nil, &config.Config{})
	defer svc.Stop()

	// 手动创建 session（必须设置 CreatedAt，否则会因 TTL 过期被拒绝）
	svc.sessionStore.Set("test-session", &geminicli.OAuthSession{
		State:        "correct-state",
		CodeVerifier: "verifier",
		OAuthType:    "ai_studio",
		CreatedAt:    time.Now(),
	})

	_, err := svc.ExchangeCode(context.Background(), &GeminiExchangeCodeInput{
		SessionID: "test-session",
		State:     "wrong-state",
		Code:      "code",
	})
	if err == nil {
		t.Fatal("应返回错误（state 不匹配）")
	}
	if !strings.Contains(err.Error(), "invalid state") {
		t.Fatalf("错误信息不匹配: got=%q", err.Error())
	}
}

func TestGeminiOAuthService_ExchangeCode_EmptyState(t *testing.T) {
	t.Parallel()

	svc := NewGeminiOAuthService(nil, nil, nil, &config.Config{})
	defer svc.Stop()

	svc.sessionStore.Set("test-session", &geminicli.OAuthSession{
		State:        "correct-state",
		CodeVerifier: "verifier",
		CreatedAt:    time.Now(),
	})

	_, err := svc.ExchangeCode(context.Background(), &GeminiExchangeCodeInput{
		SessionID: "test-session",
		State:     "",
		Code:      "code",
	})
	if err == nil {
		t.Fatal("应返回错误（空 state）")
	}
}

func TestGeminiOAuthService_ExchangeCode_RejectsLegacyAIStudioSession(t *testing.T) {
	t.Parallel()

	svc := NewGeminiOAuthService(nil, nil, nil, &config.Config{})
	defer svc.Stop()

	svc.sessionStore.Set("legacy-session", &geminicli.OAuthSession{
		State:        "state",
		CodeVerifier: "verifier",
		OAuthType:    "ai_studio",
		CreatedAt:    time.Now(),
	})

	_, err := svc.ExchangeCode(context.Background(), &GeminiExchangeCodeInput{
		SessionID: "legacy-session",
		State:     "state",
		Code:      "code",
	})
	if err == nil {
		t.Fatal("legacy ai_studio session 应被拒绝")
	}
	if !strings.Contains(err.Error(), "missing oauth_type") {
		t.Fatalf("错误信息不匹配: got=%q", err.Error())
	}
}

// =====================
// 辅助函数
// =====================

func assertCredStr(t *testing.T, creds map[string]any, key, want string) {
	t.Helper()
	raw, ok := creds[key]
	if !ok {
		t.Fatalf("creds 缺少 key=%q", key)
	}
	got, ok := raw.(string)
	if !ok {
		t.Fatalf("creds[%q] 不是 string: %T", key, raw)
	}
	if got != want {
		t.Fatalf("creds[%q] = %q, want %q", key, got, want)
	}
}

func TestGeminiOAuthService_ExchangeCode_GoogleOne_UsesDetectedTier(t *testing.T) {
	client := &mockGeminiOAuthClient{
		exchangeCodeFunc: func(ctx context.Context, oauthType, code, codeVerifier, redirectURI, proxyURL string) (*geminicli.TokenResponse, error) {
			return &geminicli.TokenResponse{
				AccessToken:  "at-google-one",
				RefreshToken: "rt-google-one",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			}, nil
		},
	}

	codeAssist := &mockGeminiCodeAssistClient{
		loadCodeAssistFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
			return &geminicli.LoadCodeAssistResponse{
				CloudAICompanionProject: "auto-project-123",
				AllowedTiers:            []geminicli.AllowedTier{{ID: "g1-pro-tier", IsDefault: true}},
			}, nil
		},
	}

	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, client, codeAssist, &config.Config{})
	defer svc.Stop()

	result, err := svc.GenerateAuthURL(context.Background(), nil, "https://example.com/auth/callback", "", "google_one", "")
	if err != nil {
		t.Fatalf("GenerateAuthURL failed: %v", err)
	}

	info, err := svc.ExchangeCode(context.Background(), &GeminiExchangeCodeInput{
		SessionID: result.SessionID,
		State:     result.State,
		Code:      "auth-code",
		OAuthType: "google_one",
	})
	if err != nil {
		t.Fatalf("ExchangeCode should not fail: %v", err)
	}
	if info.ProjectID != "auto-project-123" {
		t.Fatalf("ProjectID should come from LoadCodeAssist: got=%q", info.ProjectID)
	}
	if info.TierID != GeminiTierGoogleAIPro {
		t.Fatalf("TierID should use detected Code Assist tier %q: got=%q", GeminiTierGoogleAIPro, info.TierID)
	}
}

func credKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
