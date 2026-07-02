package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type dataResponse struct {
	Code int         `json:"code"`
	Data dataPayload `json:"data"`
}

type dataImportResponse struct {
	Code int              `json:"code"`
	Data DataImportResult `json:"data"`
}

type dataPayload struct {
	Type           string        `json:"type"`
	Version        int           `json:"version"`
	Proxies        []dataProxy   `json:"proxies"`
	Accounts       []dataAccount `json:"accounts"`
	SkippedShadows int           `json:"skipped_shadows"`
}

type dataProxy struct {
	ProxyKey string `json:"proxy_key"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Status   string `json:"status"`
}

type dataAccount struct {
	Name        string         `json:"name"`
	Platform    string         `json:"platform"`
	Type        string         `json:"type"`
	Credentials map[string]any `json:"credentials"`
	Extra       map[string]any `json:"extra"`
	ProxyKey    *string        `json:"proxy_key"`
	Concurrency int            `json:"concurrency"`
	Priority    int            `json:"priority"`
}

func setupAccountDataRouter() (*gin.Engine, *stubAdminService) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	adminSvc := newStubAdminService()

	h := NewAccountHandler(
		adminSvc,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	router.GET("/api/v1/admin/accounts/data", h.ExportData)
	router.POST("/api/v1/admin/accounts/data", h.ImportData)
	router.POST("/api/v1/admin/accounts/import/archive", h.ImportArchive)
	return router, adminSvc
}

func TestExportDataIncludesSecrets(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()

	proxyID := int64(11)
	adminSvc.proxies = []service.Proxy{
		{
			ID:       proxyID,
			Name:     "proxy",
			Protocol: "http",
			Host:     "127.0.0.1",
			Port:     8080,
			Username: "user",
			Password: "pass",
			Status:   service.StatusActive,
		},
		{
			ID:       12,
			Name:     "orphan",
			Protocol: "https",
			Host:     "10.0.0.1",
			Port:     443,
			Username: "o",
			Password: "p",
			Status:   service.StatusActive,
		},
	}
	adminSvc.accounts = []service.Account{
		{
			ID:          21,
			Name:        "account",
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeOAuth,
			Credentials: map[string]any{"token": "secret"},
			Extra:       map[string]any{"note": "x"},
			ProxyID:     &proxyID,
			Concurrency: 3,
			Priority:    50,
			Status:      service.StatusDisabled,
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/data", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp dataResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Empty(t, resp.Data.Type)
	require.Equal(t, 0, resp.Data.Version)
	require.Len(t, resp.Data.Proxies, 1)
	require.Equal(t, "pass", resp.Data.Proxies[0].Password)
	require.Len(t, resp.Data.Accounts, 1)
	require.Equal(t, "secret", resp.Data.Accounts[0].Credentials["token"])
}

func TestExportDataWithoutProxies(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()

	proxyID := int64(11)
	adminSvc.proxies = []service.Proxy{
		{
			ID:       proxyID,
			Name:     "proxy",
			Protocol: "http",
			Host:     "127.0.0.1",
			Port:     8080,
			Username: "user",
			Password: "pass",
			Status:   service.StatusActive,
		},
	}
	adminSvc.accounts = []service.Account{
		{
			ID:          21,
			Name:        "account",
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeOAuth,
			Credentials: map[string]any{"token": "secret"},
			ProxyID:     &proxyID,
			Concurrency: 3,
			Priority:    50,
			Status:      service.StatusDisabled,
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/data?include_proxies=false", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp dataResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data.Proxies, 0)
	require.Len(t, resp.Data.Accounts, 1)
	require.Nil(t, resp.Data.Accounts[0].ProxyKey)
}

// TestExportDataExcludesSparkShadow 验证外审第5轮 P1/P2:导出时排除 spark 影子账号
// (影子无凭据、导入侧强制 credentials 非空,混入会产出无法还原的坏备份),并透出跳过计数。
func TestExportDataExcludesSparkShadow(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()

	parentID := int64(21)
	adminSvc.accounts = []service.Account{
		{
			ID:          parentID,
			Name:        "mother",
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeOAuth,
			Credentials: map[string]any{"token": "secret"},
			Status:      service.StatusActive,
		},
		{
			ID:              22,
			Name:            "mother (Spark)",
			Platform:        service.PlatformOpenAI,
			Type:            service.AccountTypeOAuth,
			Credentials:     map[string]any{}, // 影子恒空凭据
			ParentAccountID: &parentID,        // 影子标记
			QuotaDimension:  service.QuotaDimensionSpark,
			Status:          service.StatusActive,
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/data?include_proxies=false", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp dataResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data.Accounts, 1, "影子应被排除,仅导出母账号")
	require.Equal(t, "mother", resp.Data.Accounts[0].Name)
	require.Equal(t, 1, resp.Data.SkippedShadows, "跳过的影子数量应透出")
}

func TestExportDataPassesAccountFiltersAndSort(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()
	adminSvc.accounts = []service.Account{
		{ID: 1, Name: "acc-1", Status: service.StatusActive},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/admin/accounts/data?platform=openai&type=oauth&status=active&group=12&privacy_mode=blocked&search=keyword&sort_by=priority&sort_order=desc",
		nil,
	)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	require.Equal(t, 1, adminSvc.lastListAccounts.calls)
	require.Equal(t, "openai", adminSvc.lastListAccounts.platform)
	require.Equal(t, "oauth", adminSvc.lastListAccounts.accountType)
	require.Equal(t, "active", adminSvc.lastListAccounts.status)
	require.Equal(t, int64(12), adminSvc.lastListAccounts.groupID)
	require.Equal(t, "blocked", adminSvc.lastListAccounts.privacyMode)
	require.Equal(t, "keyword", adminSvc.lastListAccounts.search)
	require.Equal(t, "priority", adminSvc.lastListAccounts.sortBy)
	require.Equal(t, "desc", adminSvc.lastListAccounts.sortOrder)
}

func TestExportDataSelectedIDsOverrideFilters(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/admin/accounts/data?ids=1,2&platform=openai&search=keyword&sort_by=priority&sort_order=desc",
		nil,
	)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp dataResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data.Accounts, 2)
	require.Equal(t, 0, adminSvc.lastListAccounts.calls)
}

func TestImportDataReusesProxyAndSkipsDefaultGroup(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()

	adminSvc.proxies = []service.Proxy{
		{
			ID:       1,
			Name:     "proxy",
			Protocol: "socks5",
			Host:     "1.2.3.4",
			Port:     1080,
			Username: "u",
			Password: "p",
			Status:   service.StatusActive,
		},
	}

	dataPayload := map[string]any{
		"data": map[string]any{
			"type":    dataType,
			"version": dataVersion,
			"proxies": []map[string]any{
				{
					"proxy_key": "socks5|1.2.3.4|1080|u|p",
					"name":      "proxy",
					"protocol":  "socks5",
					"host":      "1.2.3.4",
					"port":      1080,
					"username":  "u",
					"password":  "p",
					"status":    "active",
				},
			},
			"accounts": []map[string]any{
				{
					"name":        "acc",
					"platform":    service.PlatformOpenAI,
					"type":        service.AccountTypeOAuth,
					"credentials": map[string]any{"token": "x"},
					"proxy_key":   "socks5|1.2.3.4|1080|u|p",
					"concurrency": 3,
					"priority":    50,
				},
			},
		},
		"skip_default_group_bind": true,
	}

	body, _ := json.Marshal(dataPayload)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/data", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	require.Len(t, adminSvc.createdProxies, 0)
	require.Len(t, adminSvc.createdAccounts, 1)
	require.True(t, adminSvc.createdAccounts[0].SkipDefaultGroupBind)
}

func TestImportDataAcceptsGrokAccount(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()

	dataPayload := map[string]any{
		"data": map[string]any{
			"type":    dataType,
			"version": dataVersion,
			"proxies": []map[string]any{},
			"accounts": []map[string]any{
				{
					"name":     "grok-oauth",
					"platform": service.PlatformGrok,
					"type":     service.AccountTypeOAuth,
					"credentials": map[string]any{
						"access_token":  "grok-at",
						"refresh_token": "grok-rt",
						"email":         "owner@example.com",
					},
					"concurrency": 3,
					"priority":    50,
				},
			},
		},
		"skip_default_group_bind": true,
	}

	body, _ := json.Marshal(dataPayload)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/data", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, adminSvc.createdAccounts, 1)
	require.Equal(t, service.PlatformGrok, adminSvc.createdAccounts[0].Platform)
	require.Equal(t, service.AccountTypeOAuth, adminSvc.createdAccounts[0].Type)
	require.True(t, adminSvc.createdAccounts[0].SkipDefaultGroupBind)
}

func TestImportDataAcceptsKiroRawArray(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()

	longRT := strings.Repeat("A", 512)
	rawArray := []map[string]any{
		{
			"id":             "kiro_abc",
			"email":          "jeno@example.com",
			"login_provider": "ExternalIdp",
			"access_token":   "at-1",
			"refresh_token":  longRT,
			"token_type":     "Bearer",
			"expires_at":     1782908966,
			"issuer_url":     "https://login.microsoftonline.com/tenant/v2.0",
			"client_id":      "e491fadf-0239-44f9-be3b-d3e1ff193c79",
			"scopes":         "api://x/codewhisperer:conversations offline_access",
			"login_hint":     "jeno@example.com",
			"plan_name":      "Credit",
			"plan_tier":      "CREDIT",
			"status":         "banned",
			"status_reason":  "Invalid ARN e491fadf-0239-44f9-be3b-d3e1ff193c79",
			"kiro_auth_token_raw": map[string]any{
				"authMethod":    "external_idp",
				"clientId":      "e491fadf-0239-44f9-be3b-d3e1ff193c79",
				"issuerUrl":     "https://login.microsoftonline.com/tenant/v2.0",
				"refreshToken":  longRT,
				"tokenEndpoint": "https://login.microsoftonline.com/tenant/oauth2/v2.0/token",
			},
			"kiro_profile_raw": map[string]any{
				"arn":  "arn:aws:codewhisperer:us-east-1:904962390873:profile/CQRAXYDP9YVD",
				"name": "KiroProfile-us-east-1",
			},
			"kiro_usage_raw": map[string]any{"hasBeenInstalled": true},
		},
		{
			"id":            "kiro_def",
			"email":         "inigo@example.com",
			"refresh_token": longRT,
			"access_token":  "at-2",
			"expires_at":    1782907857,
			"client_id":     "e491fadf-0239-44f9-be3b-d3e1ff193c79",
			"kiro_auth_token_raw": map[string]any{
				"authMethod":    "external_idp",
				"tokenEndpoint": "https://login.microsoftonline.com/tenant/oauth2/v2.0/token",
			},
			"kiro_profile_raw": map[string]any{
				"arn": "arn:aws:codewhisperer:us-east-1:904962390873:profile/CQRAXYDP9YVD",
			},
		},
	}

	dataPayload := map[string]any{
		"data":                    rawArray,
		"skip_default_group_bind": true,
		"dedup_mode":              "none",
	}

	body, _ := json.Marshal(dataPayload)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/data", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, adminSvc.createdAccounts, 2)

	first := adminSvc.createdAccounts[0]
	require.Equal(t, service.PlatformKiro, first.Platform)
	require.Equal(t, service.AccountTypeOAuth, first.Type)
	require.Equal(t, "jeno@example.com", first.Name)
	require.True(t, first.SkipDefaultGroupBind)
	// 嵌套原始字段整体保留在 credentials 中，交给后端规范化提取 profile_arn。
	require.Contains(t, first.Credentials, "kiro_profile_raw")
	require.Contains(t, first.Credentials, "kiro_auth_token_raw")
	require.Equal(t, longRT, first.Credentials["refresh_token"])
	// 纯展示/元信息字段转入 extra，不污染 credentials。
	require.NotContains(t, first.Credentials, "status")
	require.NotContains(t, first.Credentials, "plan_name")
	require.NotContains(t, first.Credentials, "kiro_usage_raw")
	require.Equal(t, "banned", first.Extra["status"])
	require.Equal(t, "Credit", first.Extra["plan_name"])
}

func TestImportDataKiroRawArrayDedupUsesNormalizedCredentials(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()
	longRT := strings.Repeat("R", 32)
	adminSvc.accounts = []service.Account{
		{
			ID:       81,
			Name:     "existing-kiro",
			Platform: service.PlatformKiro,
			Type:     service.AccountTypeOAuth,
			Credentials: map[string]any{
				"profile_id":    "CQRAXYDP9YVD",
				"refresh_token": "old-refresh-token",
			},
		},
	}

	rawArray := []map[string]any{
		{
			"id":            "kiro_abc",
			"email":         "jeno@example.com",
			"refresh_token": longRT,
			"kiro_auth_token_raw": map[string]any{
				"authMethod":   "external_idp",
				"clientId":     "microsoft-public-client",
				"refreshToken": longRT,
			},
			"kiro_profile_raw": map[string]any{
				"arn": "arn:aws:codewhisperer:us-east-1:904962390873:profile/CQRAXYDP9YVD",
			},
		},
	}
	body, _ := json.Marshal(map[string]any{
		"data":       rawArray,
		"dedup_mode": "ignore",
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/data", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp dataImportResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 1, resp.Data.AccountSkipped)
	require.Len(t, adminSvc.createdAccounts, 0)
}

func TestImportDataDedupIgnoreSkipsExistingAccount(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()
	adminSvc.accounts = []service.Account{
		{
			ID:          10,
			Name:        "existing",
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeOAuth,
			Credentials: map[string]any{"refresh_token": "rt-1"},
		},
	}

	dataPayload := map[string]any{
		"data": map[string]any{
			"type":    dataType,
			"version": dataVersion,
			"proxies": []map[string]any{},
			"accounts": []map[string]any{
				{
					"name":        "imported",
					"platform":    service.PlatformOpenAI,
					"type":        service.AccountTypeOAuth,
					"credentials": map[string]any{"refresh_token": "rt-1", "access_token": "new-at"},
					"concurrency": 3,
					"priority":    50,
				},
			},
		},
		"skip_default_group_bind": true,
		"dedup_mode":              dataImportDedupModeIgnore,
	}

	body, _ := json.Marshal(dataPayload)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/data", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp dataImportResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 1, resp.Data.AccountSkipped)
	require.Len(t, adminSvc.createdAccounts, 0)
	require.Len(t, adminSvc.updatedAccounts, 0)
}

func TestImportDataDedupOverwriteUpdatesExistingAccount(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()
	adminSvc.accounts = []service.Account{
		{
			ID:          10,
			Name:        "existing",
			Platform:    service.PlatformKiro,
			Type:        service.AccountTypeOAuth,
			Credentials: map[string]any{"profile_id": "PROFILE1", "refresh_token": "old-rt"},
		},
	}

	dataPayload := map[string]any{
		"data": map[string]any{
			"type":    dataType,
			"version": dataVersion,
			"proxies": []map[string]any{},
			"accounts": []map[string]any{
				{
					"name":        "imported",
					"platform":    service.PlatformKiro,
					"type":        service.AccountTypeOAuth,
					"credentials": map[string]any{"profile_id": "PROFILE1", "refresh_token": "new-rt"},
					"extra":       map[string]any{"plan_name": "Pro"},
					"concurrency": 5,
					"priority":    60,
				},
			},
		},
		"skip_default_group_bind": true,
		"dedup_mode":              dataImportDedupModeOverwrite,
	}

	body, _ := json.Marshal(dataPayload)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/data", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp dataImportResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 1, resp.Data.AccountUpdated)
	require.Len(t, adminSvc.createdAccounts, 0)
	require.Len(t, adminSvc.updatedAccounts, 1)
	require.Equal(t, int64(10), adminSvc.updatedAccountIDs[0])
	require.Equal(t, "imported", adminSvc.updatedAccounts[0].Name)
	require.Equal(t, service.AccountTypeOAuth, adminSvc.updatedAccounts[0].Type)
	require.Equal(t, map[string]any{"profile_id": "PROFILE1", "refresh_token": "new-rt"}, adminSvc.updatedAccounts[0].Credentials)
	require.Equal(t, map[string]any{"plan_name": "Pro"}, adminSvc.updatedAccounts[0].Extra)
}

func TestImportDataDedupOverwriteMatchesExistingEmailWhenImportedAccountAddsStableID(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()
	adminSvc.accounts = []service.Account{
		{
			ID:       10,
			Name:     "existing",
			Platform: service.PlatformOpenAI,
			Type:     service.AccountTypeOAuth,
			Credentials: map[string]any{
				"email": "user@example.com",
			},
		},
	}

	dataPayload := map[string]any{
		"data": map[string]any{
			"type":    dataType,
			"version": dataVersion,
			"proxies": []map[string]any{},
			"accounts": []map[string]any{
				{
					"name":     "imported",
					"platform": service.PlatformOpenAI,
					"type":     service.AccountTypeOAuth,
					"credentials": map[string]any{
						"email":              "user@example.com",
						"chatgpt_account_id": "acct-123",
					},
					"concurrency": 3,
					"priority":    50,
				},
			},
		},
		"skip_default_group_bind": true,
		"dedup_mode":              dataImportDedupModeOverwrite,
	}

	body, _ := json.Marshal(dataPayload)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/data", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp dataImportResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 1, resp.Data.AccountUpdated)
	require.Len(t, adminSvc.createdAccounts, 0)
	require.Len(t, adminSvc.updatedAccounts, 1)
	require.Equal(t, int64(10), adminSvc.updatedAccountIDs[0])
}

func TestImportDataDedupOverwriteMatchesExistingStableIDWhenImportedRefreshTokenRotates(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()
	adminSvc.accounts = []service.Account{
		{
			ID:       11,
			Name:     "existing",
			Platform: service.PlatformOpenAI,
			Type:     service.AccountTypeOAuth,
			Credentials: map[string]any{
				"chatgpt_account_id": "acct-456",
				"refresh_token":      "old-rt",
			},
		},
	}

	dataPayload := map[string]any{
		"data": map[string]any{
			"type":    dataType,
			"version": dataVersion,
			"proxies": []map[string]any{},
			"accounts": []map[string]any{
				{
					"name":     "imported",
					"platform": service.PlatformOpenAI,
					"type":     service.AccountTypeOAuth,
					"credentials": map[string]any{
						"chatgpt_account_id": "acct-456",
						"refresh_token":      "new-rt",
					},
					"concurrency": 3,
					"priority":    50,
				},
			},
		},
		"skip_default_group_bind": true,
		"dedup_mode":              dataImportDedupModeOverwrite,
	}

	body, _ := json.Marshal(dataPayload)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/data", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp dataImportResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 1, resp.Data.AccountUpdated)
	require.Len(t, adminSvc.createdAccounts, 0)
	require.Len(t, adminSvc.updatedAccounts, 1)
	require.Equal(t, int64(11), adminSvc.updatedAccountIDs[0])
}

func TestImportDataDedupOverwriteDoesNotClearMissingOptionalFields(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()
	adminSvc.accounts = []service.Account{
		{
			ID:          10,
			Name:        "existing",
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeOAuth,
			Credentials: map[string]any{"chatgpt_account_id": "acct-1"},
			Extra:       map[string]any{"keep": "existing"},
		},
	}

	dataPayload := map[string]any{
		"data": map[string]any{
			"type":    dataType,
			"version": dataVersion,
			"proxies": []map[string]any{},
			"accounts": []map[string]any{
				{
					"name":        "imported",
					"platform":    service.PlatformOpenAI,
					"type":        service.AccountTypeOAuth,
					"credentials": map[string]any{"chatgpt_account_id": "acct-1", "access_token": "new-at"},
					"concurrency": 5,
					"priority":    60,
				},
			},
		},
		"skip_default_group_bind": true,
		"dedup_mode":              dataImportDedupModeOverwrite,
	}

	body, _ := json.Marshal(dataPayload)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/data", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	require.Len(t, adminSvc.updatedAccounts, 1)
	require.Nil(t, adminSvc.updatedAccounts[0].ProxyID)
	require.Nil(t, adminSvc.updatedAccounts[0].ExpiresAt)
	require.Nil(t, adminSvc.updatedAccounts[0].Extra)
}

func TestImportDataDedupOverwriteFailsOnAmbiguousWeakIdentifier(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()
	adminSvc.accounts = []service.Account{
		{
			ID:       21,
			Name:     "existing-a",
			Platform: service.PlatformOpenAI,
			Type:     service.AccountTypeOAuth,
			Credentials: map[string]any{
				"email":              "shared@example.com",
				"chatgpt_account_id": "acct-a",
			},
		},
		{
			ID:       22,
			Name:     "existing-b",
			Platform: service.PlatformOpenAI,
			Type:     service.AccountTypeOAuth,
			Credentials: map[string]any{
				"email":              "shared@example.com",
				"chatgpt_account_id": "acct-b",
			},
		},
	}

	dataPayload := map[string]any{
		"data": map[string]any{
			"type":    dataType,
			"version": dataVersion,
			"proxies": []map[string]any{},
			"accounts": []map[string]any{
				{
					"name":     "imported",
					"platform": service.PlatformOpenAI,
					"type":     service.AccountTypeOAuth,
					"credentials": map[string]any{
						"email": "shared@example.com",
					},
					"concurrency": 3,
					"priority":    50,
				},
			},
		},
		"skip_default_group_bind": true,
		"dedup_mode":              dataImportDedupModeOverwrite,
	}

	body, _ := json.Marshal(dataPayload)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/data", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp dataImportResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 1, resp.Data.AccountFailed)
	require.Equal(t, 0, resp.Data.AccountUpdated)
	require.Equal(t, 0, resp.Data.AccountCreated)
	require.Len(t, adminSvc.updatedAccounts, 0)
	require.Len(t, adminSvc.createdAccounts, 0)
	require.Len(t, resp.Data.Errors, 1)
	require.Contains(t, resp.Data.Errors[0].Message, "ambiguous")
}
