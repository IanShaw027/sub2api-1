package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettingHandler_UpdateSettings_ValidatesKiroTTLAgainstSavedValues(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		values         map[string]string
		body           map[string]any
		expectedStatus int
	}{
		{
			name: "rejects prefix update above saved independent ttl",
			values: map[string]string{
				service.SettingKeyKiroCacheIndependentTTLSeconds: "300",
				service.SettingKeyKiroCachePrefixTTLSeconds:      "300",
			},
			body:           map[string]any{"cache_prefix_ttl_seconds": 400},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "rejects independent update below saved prefix ttl",
			values: map[string]string{
				service.SettingKeyKiroCacheIndependentTTLSeconds: "3600",
				service.SettingKeyKiroCachePrefixTTLSeconds:      "600",
			},
			body:           map[string]any{"cache_independent_ttl_seconds": 500},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "accepts compatible effective ttl pair",
			values: map[string]string{
				service.SettingKeyKiroCacheIndependentTTLSeconds: "3600",
				service.SettingKeyKiroCachePrefixTTLSeconds:      "300",
			},
			body:           map[string]any{"cache_prefix_ttl_seconds": 600},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "rejects cache min block tokens above contract max",
			body:           map[string]any{"cache_min_block_tokens": service.KiroCacheMinBlockTokensMax + 1},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &settingHandlerRepoStub{values: tt.values}
			svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
			handler := NewSettingHandler(svc, nil, nil, nil, nil, nil)
			rawBody, err := json.Marshal(tt.body)
			require.NoError(t, err)

			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
			c.Request.Header.Set("Content-Type", "application/json")

			handler.UpdateSettings(c)

			require.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

func TestSettingHandler_UpdateSettings_KiroRuntimeRequestResponseAndRuntimeMatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &settingHandlerRepoStub{}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil)
	body := map[string]any{
		"kiro_version":                        " 0.11.0 ",
		"kiro_commit":                         " commit-123 ",
		"system_version":                      " linux#6.8.0 ",
		"node_version":                        " 22.22.0 ",
		"cache_hit_rate_scale":                88,
		"cache_min_block_tokens":              service.KiroCacheMinBlockTokensMax,
		"cache_independent_ttl_seconds":       7200,
		"cache_prefix_ttl_seconds":            600,
		"kiro_code_execution_sandbox_command": "  sandbox-run --kiro  ",
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			KiroVersion                     string `json:"kiro_version"`
			KiroCommit                      string `json:"kiro_commit"`
			SystemVersion                   string `json:"system_version"`
			NodeVersion                     string `json:"node_version"`
			KiroCacheHitRateScale           int    `json:"cache_hit_rate_scale"`
			KiroCacheMinBlockTokens         int    `json:"cache_min_block_tokens"`
			KiroCacheIndependentTTLSeconds  int    `json:"cache_independent_ttl_seconds"`
			KiroCachePrefixTTLSeconds       int    `json:"cache_prefix_ttl_seconds"`
			KiroCodeExecutionSandboxCommand string `json:"kiro_code_execution_sandbox_command"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, "0.11.0", resp.Data.KiroVersion)
	require.Equal(t, "commit-123", resp.Data.KiroCommit)
	require.Equal(t, "linux#6.8.0", resp.Data.SystemVersion)
	require.Equal(t, "22.22.0", resp.Data.NodeVersion)
	require.Equal(t, 88, resp.Data.KiroCacheHitRateScale)
	require.Equal(t, service.KiroCacheMinBlockTokensMax, resp.Data.KiroCacheMinBlockTokens)
	require.Equal(t, 7200, resp.Data.KiroCacheIndependentTTLSeconds)
	require.Equal(t, 600, resp.Data.KiroCachePrefixTTLSeconds)
	require.Equal(t, "sandbox-run --kiro", resp.Data.KiroCodeExecutionSandboxCommand)

	runtime := svc.GetKiroRuntimeSettings(context.Background())
	require.Equal(t, resp.Data.KiroVersion, runtime.KiroVersion)
	require.Equal(t, resp.Data.KiroCommit, runtime.KiroCommit)
	require.Equal(t, resp.Data.SystemVersion, runtime.SystemVersion)
	require.Equal(t, resp.Data.NodeVersion, runtime.NodeVersion)
	require.Equal(t, resp.Data.KiroCacheHitRateScale, runtime.CacheHitRateScale)
	require.Equal(t, resp.Data.KiroCacheMinBlockTokens, runtime.CacheMinBlockTokens)
	require.Equal(t, resp.Data.KiroCacheIndependentTTLSeconds, runtime.CacheIndependentTTLSecs)
	require.Equal(t, resp.Data.KiroCachePrefixTTLSeconds, runtime.CachePrefixTTLSecs)
	require.Equal(t, resp.Data.KiroCodeExecutionSandboxCommand, runtime.CodeExecutionSandboxCommand)
}

func TestSettingHandler_UpdateSettings_ReturnsOpenAIStickyWaitTimeoutSeconds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &settingHandlerRepoStub{}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil)
	rawBody, err := json.Marshal(map[string]any{
		"openai_sticky_wait_timeout_seconds": 45,
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			OpenAIStickyWaitTimeoutSeconds *int `json:"openai_sticky_wait_timeout_seconds"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.NotNil(t, resp.Data.OpenAIStickyWaitTimeoutSeconds)
	require.Equal(t, 45, *resp.Data.OpenAIStickyWaitTimeoutSeconds)
}

func TestSettingHandler_UpdateSettings_IgnoresLegacyKiroThinkingFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &settingHandlerRepoStub{}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil)
	body := map[string]any{
		"kiro_thinking_mode":                "model_and_simulate",
		"kiro_thinking_effort_threshold":    "max",
		"kiro_thinking_simulation_template": "legacy template",
		"kiro_thinking_free_prompt":         "legacy prompt",
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, float64(0), resp["code"])
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	for _, key := range []string{
		"kiro_thinking_mode",
		"kiro_thinking_effort_threshold",
		"kiro_thinking_simulation_template",
		"kiro_thinking_free_prompt",
	} {
		require.NotContains(t, data, key)
		require.NotContains(t, repo.lastUpdates, key)
	}
}
