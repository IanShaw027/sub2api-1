//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGetLinuxDoConnectOAuthConfig_NilRepoFallsBackToConfig(t *testing.T) {
	cfg := &config.Config{
		LinuxDo: config.LinuxDoConnectConfig{
			Enabled:             true,
			ClientID:            "linuxdo-client",
			ClientSecret:        "linuxdo-secret",
			AuthorizeURL:        "https://linux.do/oauth/authorize",
			TokenURL:            "https://linux.do/oauth/token",
			UserInfoURL:         "https://linux.do/oauth/userinfo",
			RedirectURL:         "https://app.example.com/api/v1/auth/oauth/linuxdo/callback",
			FrontendRedirectURL: "/auth/linuxdo/callback",
			TokenAuthMethod:     "client_secret_post",
		},
	}
	svc := NewSettingService(nil, cfg)

	got, err := svc.GetLinuxDoConnectOAuthConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, cfg.LinuxDo.ClientID, got.ClientID)
	require.Equal(t, cfg.LinuxDo.ClientSecret, got.ClientSecret)
	require.Equal(t, cfg.LinuxDo.RedirectURL, got.RedirectURL)
	require.Equal(t, cfg.LinuxDo.FrontendRedirectURL, got.FrontendRedirectURL)
}

func TestGetDingTalkConnectOAuthConfig_NilRepoFallsBackToConfig(t *testing.T) {
	cfg := &config.Config{
		DingTalk: config.DingTalkConnectConfig{
			Enabled:               true,
			ClientID:              "dt-client",
			ClientSecret:          "dt-secret",
			AuthorizeURL:          "https://dingtalk.example.com/oauth/authorize",
			TokenURL:              "https://dingtalk.example.com/oauth/token",
			UserInfoURL:           "https://dingtalk.example.com/oauth/userinfo",
			RedirectURL:           "https://app.example.com/api/v1/auth/oauth/dingtalk/callback",
			FrontendRedirectURL:   "/auth/dingtalk/callback",
			DingTalkAppKind:       "internal_app",
			AppType:               "internal",
			CorpRestrictionPolicy: "internal_only",
			BypassRegistration:    true,
			SyncCorpEmail:         true,
			SyncDisplayName:       true,
			SyncDept:              true,
		},
	}
	svc := NewSettingService(nil, cfg)

	got, err := svc.GetDingTalkConnectOAuthConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, cfg.DingTalk.ClientID, got.ClientID)
	require.Equal(t, cfg.DingTalk.RedirectURL, got.RedirectURL)
	require.Equal(t, cfg.DingTalk.CorpRestrictionPolicy, got.CorpRestrictionPolicy)
	require.True(t, got.BypassRegistration)
}

func TestGetWeChatConnectOAuthConfig_NilRepoFallsBackToConfig(t *testing.T) {
	cfg := &config.Config{
		WeChat: config.WeChatConnectConfig{
			Enabled:             true,
			OpenEnabled:         true,
			MPEnabled:           true,
			Mode:                "open",
			OpenAppID:           "wx-open-config",
			OpenAppSecret:       "wx-open-secret",
			MPAppID:             "wx-mp-config",
			MPAppSecret:         "wx-mp-secret",
			FrontendRedirectURL: "/auth/wechat/config-callback",
		},
	}
	svc := NewSettingService(nil, cfg)

	got, err := svc.GetWeChatConnectOAuthConfig(context.Background())
	require.NoError(t, err)
	require.True(t, got.Enabled)
	require.True(t, got.OpenEnabled)
	require.True(t, got.MPEnabled)
	require.Equal(t, "wx-open-config", got.AppIDForMode("open"))
	require.Equal(t, "wx-mp-config", got.AppIDForMode("mp"))
	require.Equal(t, "/auth/wechat/config-callback", got.FrontendRedirectURL)
}

func TestGetOIDCConnectOAuthConfig_NilRepoFallsBackToConfig(t *testing.T) {
	cfg := &config.Config{
		OIDC: config.OIDCConnectConfig{
			Enabled:                 true,
			ProviderName:            "OIDC",
			ClientID:                "oidc-client",
			ClientSecret:            "oidc-secret",
			IssuerURL:               "https://issuer.example.com",
			AuthorizeURL:            "https://issuer.example.com/auth",
			TokenURL:                "https://issuer.example.com/token",
			UserInfoURL:             "https://issuer.example.com/userinfo",
			JWKSURL:                 "https://issuer.example.com/jwks",
			RedirectURL:             "https://app.example.com/api/v1/auth/oauth/oidc/callback",
			FrontendRedirectURL:     "/auth/oidc/callback",
			Scopes:                  "openid email profile",
			TokenAuthMethod:         "client_secret_post",
			UsePKCE:                 true,
			UsePKCEExplicit:         true,
			ValidateIDToken:         true,
			ValidateIDTokenExplicit: true,
			AllowedSigningAlgs:      "RS256",
			ClockSkewSeconds:        120,
		},
	}
	svc := NewSettingService(nil, cfg)

	got, err := svc.GetOIDCConnectOAuthConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, cfg.OIDC.ClientID, got.ClientID)
	require.Equal(t, cfg.OIDC.AuthorizeURL, got.AuthorizeURL)
	require.Equal(t, cfg.OIDC.TokenURL, got.TokenURL)
	require.Equal(t, cfg.OIDC.FrontendRedirectURL, got.FrontendRedirectURL)
	require.True(t, got.UsePKCE)
	require.True(t, got.ValidateIDToken)
}

func TestGetEmailOAuthProviderConfig_NilRepoFallsBackToConfig(t *testing.T) {
	cfg := &config.Config{
		GitHubOAuth: config.EmailOAuthProviderConfig{
			Enabled:             true,
			ClientID:            "github-client",
			ClientSecret:        "github-secret",
			AuthorizeURL:        "https://github.com/login/oauth/authorize",
			TokenURL:            "https://github.com/login/oauth/access_token",
			UserInfoURL:         "https://api.github.com/user",
			EmailsURL:           "https://api.github.com/user/emails",
			Scopes:              "read:user user:email",
			RedirectURL:         "https://app.example.com/api/v1/auth/oauth/github/callback",
			FrontendRedirectURL: "/auth/oauth/callback",
		},
	}
	svc := NewSettingService(nil, cfg)

	got, err := svc.GetEmailOAuthProviderConfig(context.Background(), "github")
	require.NoError(t, err)
	require.True(t, got.Enabled)
	require.Equal(t, cfg.GitHubOAuth.ClientID, got.ClientID)
	require.Equal(t, cfg.GitHubOAuth.RedirectURL, got.RedirectURL)
	require.Equal(t, cfg.GitHubOAuth.FrontendRedirectURL, got.FrontendRedirectURL)
}
