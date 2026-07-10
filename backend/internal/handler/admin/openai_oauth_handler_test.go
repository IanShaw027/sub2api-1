//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/imroc/req/v3"
	"github.com/stretchr/testify/require"
)

type openAIOAuthHandlerClientStub struct{}

func (openAIOAuthHandlerClientStub) ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI, proxyURL, clientID string) (*openai.TokenResponse, error) {
	return &openai.TokenResponse{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ExpiresIn:    3600,
	}, nil
}

func (openAIOAuthHandlerClientStub) RefreshToken(ctx context.Context, refreshToken, proxyURL string) (*openai.TokenResponse, error) {
	return &openai.TokenResponse{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ExpiresIn:    3600,
	}, nil
}

func (openAIOAuthHandlerClientStub) RefreshTokenWithClientID(ctx context.Context, refreshToken, proxyURL string, clientID string) (*openai.TokenResponse, error) {
	return &openai.TokenResponse{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ExpiresIn:    3600,
	}, nil
}

type openAIOAuthPrivacyAdminService struct {
	*stubAdminService
	account          service.Account
	updateExtraCalls int
}

func (s *openAIOAuthPrivacyAdminService) GetAccount(_ context.Context, id int64) (*service.Account, error) {
	if s.account.ID == id {
		acc := s.account
		return &acc, nil
	}
	return s.stubAdminService.GetAccount(context.Background(), id)
}

func (s *openAIOAuthPrivacyAdminService) UpdateAccount(_ context.Context, id int64, input *service.UpdateAccountInput) (*service.Account, error) {
	account := s.account
	account.ID = id
	if input.Credentials != nil {
		if account.Credentials == nil {
			account.Credentials = map[string]any{}
		}
		for k, v := range input.Credentials {
			account.Credentials[k] = v
		}
	}
	s.account = account
	acc := account
	return &acc, nil
}

func (s *openAIOAuthPrivacyAdminService) UpdateAccountExtra(_ context.Context, id int64, updates map[string]any) error {
	s.updateExtraCalls++
	if s.account.ID == id {
		if s.account.Extra == nil {
			s.account.Extra = map[string]any{}
		}
		for k, v := range updates {
			s.account.Extra[k] = v
		}
	}
	return nil
}

func (s *openAIOAuthPrivacyAdminService) EnsureOpenAIPrivacy(context.Context, *service.Account) string {
	return ""
}

func newOpenAIOAuthPrivacyClientFactory(serverURL string) func(string) (*req.Client, error) {
	base, err := url.Parse(serverURL)
	if err != nil {
		panic(err)
	}
	return func(proxyURL string) (*req.Client, error) {
		client := req.C()
		client.OnBeforeRequest(func(_ *req.Client, r *req.Request) error {
			parsed, err := url.Parse(r.RawURL)
			if err != nil {
				return err
			}
			parsed.Scheme = base.Scheme
			parsed.Host = base.Host
			r.SetURL(parsed.String())
			return nil
		})
		return client, nil
	}
}

func newOpenAIOAuthPrivacyServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/backend-api/accounts/check"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"accounts":{"acct-1":{"account":{"is_default":true},"entitlement":{"subscription_plan":"pro","expires_at":"2026-05-02T20:32:12+00:00"}}}}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/backend-api/settings/account_user_setting":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			t.Errorf("unexpected privacy request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
}

func TestOpenAIOAuthHandler_CreateFromOAuthPersistsPrivacyModeAndCodexIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	privacyServer := newOpenAIOAuthPrivacyServer(t)
	defer privacyServer.Close()

	openaiSvc := service.NewOpenAIOAuthService(nil, openAIOAuthHandlerClientStub{})
	openaiSvc.SetPrivacyClientFactory(newOpenAIOAuthPrivacyClientFactory(privacyServer.URL))

	adminSvc := newStubAdminService()
	handler := NewOpenAIOAuthHandler(openaiSvc, adminSvc, nil)

	authResult, err := openaiSvc.GenerateAuthURL(context.Background(), nil, "", service.PlatformOpenAI)
	require.NoError(t, err)
	parsedAuthURL, err := url.Parse(authResult.AuthURL)
	require.NoError(t, err)
	state := parsedAuthURL.Query().Get("state")

	router := gin.New()
	router.POST("/api/v1/admin/openai/create-from-oauth", handler.CreateAccountFromOAuth)

	body, err := json.Marshal(map[string]any{
		"session_id":   authResult.SessionID,
		"code":         "auth-code",
		"state":        state,
		"redirect_uri": "",
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/openai/create-from-oauth", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, adminSvc.createdAccounts, 1)

	created := adminSvc.createdAccounts[0]
	require.NotNil(t, created.Extra)
	require.Equal(t, "training_off", created.Extra["privacy_mode"])

	require.NotEmpty(t, created.Extra["openai_device_id"])
	require.NotEmpty(t, created.Extra["openai_session_id"])
	require.NotContains(t, created.Extra, "web_profile")
}

func TestAccountHandlerRefreshSingleAccountPersistsOpenAIPrivacyMode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	privacyServer := newOpenAIOAuthPrivacyServer(t)
	defer privacyServer.Close()

	openaiSvc := service.NewOpenAIOAuthService(nil, openAIOAuthHandlerClientStub{})
	openaiSvc.SetPrivacyClientFactory(newOpenAIOAuthPrivacyClientFactory(privacyServer.URL))

	adminSvc := &openAIOAuthPrivacyAdminService{
		stubAdminService: newStubAdminService(),
		account: service.Account{
			ID:       88,
			Platform: service.PlatformOpenAI,
			Type:     service.AccountTypeOAuth,
			Credentials: map[string]any{
				"refresh_token": "refresh-token",
				"client_id":     "client-id",
			},
		},
	}

	handler := &AccountHandler{
		adminService:       adminSvc,
		openaiOAuthService: openaiSvc,
	}

	updated, warning, err := handler.refreshSingleAccount(context.Background(), &adminSvc.account)
	require.NoError(t, err)
	require.Empty(t, warning)
	require.NotNil(t, updated)
	require.NotNil(t, updated.Extra)
	require.Equal(t, "training_off", adminSvc.account.Extra["privacy_mode"])
	require.Equal(t, "training_off", updated.Extra["privacy_mode"])
	require.Greater(t, adminSvc.updateExtraCalls, 0)
}
