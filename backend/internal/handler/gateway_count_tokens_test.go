package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type countTokensNoAccountRepoStub struct {
	service.AccountRepository
}

func (countTokensNoAccountRepoStub) ListSchedulableUngroupedByPlatforms(ctx context.Context, platforms []string) ([]service.Account, error) {
	return nil, nil
}

func TestGatewayCountTokens_SelectionFailureWritesSingleErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	cfg.RunMode = config.RunModeStandard
	cfg.Default.RateMultiplier = 1
	cfg.Gateway.Scheduling.LoadBatchEnabled = false

	gatewaySvc := service.NewGatewayService(
		countTokensNoAccountRepoStub{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		cfg,
		nil,
		nil,
		service.NewBillingService(cfg, nil),
		nil,
		nil,
		nil,
		nil,
		&service.DeferredService{},
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
	billingCacheSvc := service.NewBillingCacheService(
		nil,
		openAIChatCompletionsBalanceUserRepoStub{user: service.User{ID: 1, Balance: 100, Status: service.StatusActive}},
		nil,
		nil,
		nil,
		nil,
		cfg,
		nil,
	)
	t.Cleanup(billingCacheSvc.Stop)

	h := &GatewayHandler{
		gatewayService:      gatewaySvc,
		billingCacheService: billingCacheSvc,
		concurrencyHelper:   NewConcurrencyHelper(service.NewConcurrencyService(&concurrencyCacheMock{}), SSEPingFormatNone, time.Second),
		cfg:                 cfg,
	}

	body := `{"model":"claude-3-5-sonnet-20241022","messages":[{"role":"user","content":"hello"}]}`
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
		ID:   101,
		User: &service.User{ID: 1, Balance: 100, Status: service.StatusActive},
	})
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1, Concurrency: 1})

	h.CountTokens(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Equal(t, 1, strings.Count(rec.Body.String(), "Service temporarily unavailable"), "must not concatenate multiple JSON error responses")
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &parsed), "response body must be a single JSON object")
}
