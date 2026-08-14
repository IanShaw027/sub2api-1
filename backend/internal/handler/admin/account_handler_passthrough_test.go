package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountHandler_Create_AnthropicAPIKeyPassthroughExtraForwarded(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminSvc := newStubAdminService()
	handler := NewAccountHandler(
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
	)

	router := gin.New()
	router.POST("/api/v1/admin/accounts", handler.Create)

	body := map[string]any{
		"name":     "anthropic-key-1",
		"platform": "anthropic",
		"type":     "apikey",
		"credentials": map[string]any{
			"api_key":  "sk-ant-xxx",
			"base_url": "https://api.anthropic.com",
		},
		"extra": map[string]any{
			"anthropic_passthrough": true,
		},
		"concurrency": 1,
		"priority":    1,
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, adminSvc.createdAccounts, 1)

	created := adminSvc.createdAccounts[0]
	require.Equal(t, "anthropic", created.Platform)
	require.Equal(t, "apikey", created.Type)
	require.NotNil(t, created.Extra)
	require.Equal(t, true, created.Extra["anthropic_passthrough"])
}

func TestBatchCreateRejectsTLSFingerprintProfileIDMinusOne(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminSvc := newStubAdminService()
	handler := NewAccountHandler(
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
	)

	router := gin.New()
	router.POST("/api/v1/admin/accounts/batch", handler.BatchCreate)

	body := map[string]any{
		"accounts": []map[string]any{
			{
				"name":     "bad-tls",
				"platform": "anthropic",
				"type":     "apikey",
				"credentials": map[string]any{
					"api_key": "sk-ant-xxx",
				},
				"extra": map[string]any{
					"tls_fingerprint_profile_id": -1,
				},
			},
		},
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/batch", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, adminSvc.createdAccounts)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Success int `json:"success"`
			Failed  int `json:"failed"`
			Results []struct {
				Name    string `json:"name"`
				Success bool   `json:"success"`
				Error   string `json:"error"`
			} `json:"results"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, 0, resp.Data.Success)
	require.Equal(t, 1, resp.Data.Failed)
	require.Len(t, resp.Data.Results, 1)
	require.False(t, resp.Data.Results[0].Success)
	require.Contains(t, resp.Data.Results[0].Error, "tls_fingerprint_profile_id")
}
