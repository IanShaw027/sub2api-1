package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type kiroHandlerRefreshExecutorStub struct {
	t            *testing.T
	credentials  map[string]any
	refreshCalls int
}

func (s *kiroHandlerRefreshExecutorStub) CanRefresh(account *service.Account) bool {
	return account != nil && account.Platform == service.PlatformKiro
}

func (s *kiroHandlerRefreshExecutorStub) NeedsRefresh(_ *service.Account, _ time.Duration) bool {
	return true
}

func (s *kiroHandlerRefreshExecutorStub) Refresh(_ context.Context, account *service.Account) (map[string]any, error) {
	s.refreshCalls++
	require.Equal(s.t, service.PlatformKiro, account.Platform)
	return s.credentials, nil
}

func (s *kiroHandlerRefreshExecutorStub) CacheKey(account *service.Account) string {
	return service.KiroTokenCacheKey(account)
}

type kiroHandlerTokenCacheInvalidatorStub struct {
	calls []int64
}

func (s *kiroHandlerTokenCacheInvalidatorStub) InvalidateToken(_ context.Context, account *service.Account) error {
	if account != nil {
		s.calls = append(s.calls, account.ID)
	}
	return nil
}

type grokHandlerOAuthServiceStub struct {
	refreshCalls int
	lastAccount  *service.Account
}

func (s *grokHandlerOAuthServiceStub) RefreshAccountToken(_ context.Context, account *service.Account) (*service.GrokTokenInfo, error) {
	s.refreshCalls++
	s.lastAccount = account
	return &service.GrokTokenInfo{
		AccessToken:  "grok-access-token",
		RefreshToken: "grok-refresh-token",
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		ClientID:     "grok-client-id",
	}, nil
}

func (s *grokHandlerOAuthServiceStub) BuildAccountCredentials(tokenInfo *service.GrokTokenInfo) map[string]any {
	return map[string]any{
		"access_token":  tokenInfo.AccessToken,
		"refresh_token": tokenInfo.RefreshToken,
		"expires_at":    time.Unix(tokenInfo.ExpiresAt, 0).UTC().Format(time.RFC3339),
		"client_id":     tokenInfo.ClientID,
		"base_url":      "https://grok.example",
	}
}

type kiroReauthAdminServiceStub struct {
	*stubAdminService
	getAccountResp *service.Account
	updateResp     *service.Account
	clearResp      *service.Account
	clearCalls     int
	lastUpdateID   int64
	lastUpdate     *service.UpdateAccountInput
}

func (s *kiroReauthAdminServiceStub) GetAccount(_ context.Context, id int64) (*service.Account, error) {
	if s.getAccountResp != nil {
		account := *s.getAccountResp
		account.ID = id
		return &account, nil
	}
	return s.stubAdminService.GetAccount(context.Background(), id)
}

func (s *kiroReauthAdminServiceStub) UpdateAccount(_ context.Context, id int64, input *service.UpdateAccountInput) (*service.Account, error) {
	s.lastUpdateID = id
	s.lastUpdate = input
	if s.updateResp != nil {
		account := *s.updateResp
		account.ID = id
		return &account, nil
	}
	return s.stubAdminService.UpdateAccount(context.Background(), id, input)
}

func (s *kiroReauthAdminServiceStub) ClearAccountError(_ context.Context, id int64) (*service.Account, error) {
	s.clearCalls++
	if s.clearResp != nil {
		account := *s.clearResp
		account.ID = id
		return &account, nil
	}
	return s.stubAdminService.ClearAccountError(context.Background(), id)
}

func TestAccountHandlerRefreshSingleAccount_KiroUsesTokenProvider(t *testing.T) {
	t.Parallel()

	adminSvc := newStubAdminService()

	provider := service.NewKiroTokenProvider(nil, nil)
	executor := &kiroHandlerRefreshExecutorStub{
		t: t,
		credentials: map[string]any{
			"access_token":  "fresh-access-token",
			"refresh_token": "fresh-refresh-token",
			"expires_at":    "2030-01-01T00:00:00Z",
			"machine_id":    "machine-1",
		},
	}
	provider.SetRefreshAPI(nil, executor)
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, provider, nil, nil, nil, nil, nil, nil, nil, nil)

	account := &service.Account{
		ID:       101,
		Platform: service.PlatformKiro,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "old-refresh-token",
			"machine_id":    "machine-1",
		},
	}

	updated, warning, err := handler.refreshSingleAccount(context.Background(), account)

	require.NoError(t, err)
	require.Empty(t, warning)
	require.Equal(t, 1, executor.refreshCalls)
	require.NotNil(t, updated)
}

func TestAccountHandlerRefreshSingleAccount_GrokUsesGrokOAuthService(t *testing.T) {
	t.Parallel()

	adminSvc := newStubAdminService()
	grokSvc := &grokHandlerOAuthServiceStub{}
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, grokSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	account := &service.Account{
		ID:       202,
		Platform: service.PlatformGrok,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "old-refresh-token",
			"base_url":      "https://grok.old",
		},
	}

	updated, warning, err := handler.refreshSingleAccount(context.Background(), account)

	require.NoError(t, err)
	require.Empty(t, warning)
	require.Equal(t, 1, grokSvc.refreshCalls)
	require.NotNil(t, grokSvc.lastAccount)
	require.Equal(t, service.PlatformGrok, grokSvc.lastAccount.Platform)
	require.NotNil(t, updated)
	require.Len(t, adminSvc.updatedAccounts, 1)
	require.Equal(t, "grok-access-token", adminSvc.updatedAccounts[0].Credentials["access_token"])
	require.Equal(t, "https://grok.old", adminSvc.updatedAccounts[0].Credentials["base_url"])
}

func TestAccountHandlerReauthorizeKiroOAuthClearsRecoverableState(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	adminSvc := &kiroReauthAdminServiceStub{
		stubAdminService: newStubAdminService(),
		getAccountResp: &service.Account{
			ID:       66,
			Platform: service.PlatformKiro,
			Type:     service.AccountTypeOAuth,
			Status:   service.StatusError,
		},
		updateResp: &service.Account{
			ID:           66,
			Platform:     service.PlatformKiro,
			Type:         service.AccountTypeOAuth,
			Status:       service.StatusError,
			ErrorMessage: "expired refresh token",
		},
		clearResp: &service.Account{
			ID:       66,
			Name:     "kiro-account",
			Platform: service.PlatformKiro,
			Type:     service.AccountTypeOAuth,
			Status:   service.StatusActive,
		},
	}
	invalidator := &kiroHandlerTokenCacheInvalidatorStub{}
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, invalidator)

	router := gin.New()
	router.POST("/api/v1/admin/accounts/:id/reauthorize-kiro-oauth", handler.ReauthorizeKiroOAuth)

	body, err := json.Marshal(KiroReauthorizeAccountRequest{
		Name: "kiro-account",
		Credentials: map[string]any{
			"refresh_token": "fresh-refresh-token",
		},
		Extra: map[string]any{
			"email": "kiro@example.com",
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/66/reauthorize-kiro-oauth", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, adminSvc.lastUpdate)
	require.Equal(t, int64(66), adminSvc.lastUpdateID)
	require.True(t, adminSvc.lastUpdate.AllowSensitiveCredentials)
	require.Equal(t, 1, adminSvc.clearCalls)
	require.Equal(t, []int64{66}, invalidator.calls)

	var resp struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, service.StatusActive, resp.Data.Status)
}

func TestAccountHandlerGetKiroProfiles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalFetch := fetchKiroDiscoveredProfiles
	fetchKiroDiscoveredProfiles = func(ctx context.Context, account *service.Account, provider *service.KiroTokenProvider, usageSvc *service.AccountUsageService) ([]service.KiroAvailableProfile, error) {
		require.NotNil(t, account)
		require.Equal(t, service.PlatformKiro, account.Platform)
		return []service.KiroAvailableProfile{
			{ARN: "arn:aws:codewhisperer:eu-central-1:123:profile/EU", ProfileName: "eu"},
			{ARN: "arn:aws:codewhisperer:us-east-1:123:profile/US", ProfileName: "us"},
		}, nil
	}
	t.Cleanup(func() { fetchKiroDiscoveredProfiles = originalFetch })

	adminSvc := &kiroReauthAdminServiceStub{
		stubAdminService: newStubAdminService(),
		getAccountResp: &service.Account{
			ID:       77,
			Platform: service.PlatformKiro,
			Type:     service.AccountTypeOAuth,
			Status:   service.StatusActive,
		},
	}
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.GET("/api/v1/admin/accounts/:id/profiles", handler.GetKiroProfiles)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/77/profiles", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Data struct {
			Profiles []service.KiroAvailableProfile `json:"profiles"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Data.Profiles, 2)
	require.Equal(t, "arn:aws:codewhisperer:eu-central-1:123:profile/EU", resp.Data.Profiles[0].ARN)
}

func TestAccountHandlerSetKiroOverage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalSetter := setKiroOveragePreference
	setKiroOveragePreference = func(ctx context.Context, account *service.Account, provider *service.KiroTokenProvider, usageSvc *service.AccountUsageService, enabled bool) error {
		require.NotNil(t, account)
		require.Equal(t, int64(78), account.ID)
		require.True(t, enabled)
		return nil
	}
	t.Cleanup(func() { setKiroOveragePreference = originalSetter })

	adminSvc := &kiroReauthAdminServiceStub{
		stubAdminService: newStubAdminService(),
		getAccountResp: &service.Account{
			ID:       78,
			Platform: service.PlatformKiro,
			Type:     service.AccountTypeOAuth,
			Status:   service.StatusActive,
		},
	}
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, &service.AccountUsageService{}, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/api/v1/admin/accounts/:id/kiro/overage", handler.SetKiroOverage)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/78/kiro/overage", bytes.NewReader([]byte(`{"enabled":true}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAccountHandlerEnableAllKiroOverage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalSetter := setKiroOveragePreference
	originalUsage := fetchAdminKiroUsage
	setCalls := []int64{}
	setKiroOveragePreference = func(ctx context.Context, account *service.Account, provider *service.KiroTokenProvider, usageSvc *service.AccountUsageService, enabled bool) error {
		setCalls = append(setCalls, account.ID)
		return nil
	}
	fetchAdminKiroUsage = func(ctx context.Context, usageSvc *service.AccountUsageService, accountID int64) (*service.UsageInfo, error) {
		switch accountID {
		case 201:
			enabled := false
			return &service.UsageInfo{KiroOverageCapability: "SUPPORTED", KiroOverageEnabled: &enabled}, nil
		case 202:
			enabled := true
			return &service.UsageInfo{KiroOverageCapability: "SUPPORTED", KiroOverageEnabled: &enabled}, nil
		case 204:
			enabled := false
			return &service.UsageInfo{KiroOverageCapability: "INELIGIBLE", KiroOverageEnabled: &enabled}, nil
		case 205:
			enabled := false
			return &service.UsageInfo{KiroOverageCapability: "AVAILABLE", KiroOverageEnabled: &enabled}, nil
		default:
			return &service.UsageInfo{}, nil
		}
	}
	t.Cleanup(func() {
		setKiroOveragePreference = originalSetter
		fetchAdminKiroUsage = originalUsage
	})

	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{
		{ID: 201, Platform: service.PlatformKiro, Type: service.AccountTypeOAuth, Status: service.StatusActive},
		{ID: 202, Platform: service.PlatformKiro, Type: service.AccountTypeOAuth, Status: service.StatusActive},
		{ID: 203, Platform: service.PlatformKiro, Type: service.AccountTypeOAuth, Status: service.StatusActive},
		{ID: 204, Platform: service.PlatformKiro, Type: service.AccountTypeOAuth, Status: service.StatusActive},
		{ID: 205, Platform: service.PlatformKiro, Type: service.AccountTypeOAuth, Status: service.StatusActive},
	}
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, &service.AccountUsageService{}, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/api/v1/admin/accounts/kiro/overage/enable-all", handler.EnableAllKiroOverage)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/kiro/overage/enable-all", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, []int64{201, 205}, setCalls)
}
