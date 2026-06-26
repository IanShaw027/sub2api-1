package handler

import (
	"context"
	"errors"
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

type openAISelectionErrorAccountRepoStub struct {
	service.AccountRepository
	accounts []service.Account
}

func (s openAISelectionErrorAccountRepoStub) ListSchedulableByGroupIDAndPlatform(_ context.Context, _ int64, platform string) ([]service.Account, error) {
	return s.listByPlatform(platform), nil
}

func (s openAISelectionErrorAccountRepoStub) ListSchedulableByPlatform(_ context.Context, platform string) ([]service.Account, error) {
	return s.listByPlatform(platform), nil
}

func (s openAISelectionErrorAccountRepoStub) ListByGroup(_ context.Context, _ int64) ([]service.Account, error) {
	out := make([]service.Account, len(s.accounts))
	copy(out, s.accounts)
	return out, nil
}

func (s openAISelectionErrorAccountRepoStub) ListByPlatform(_ context.Context, platform string) ([]service.Account, error) {
	return s.listByPlatform(platform), nil
}

func (s openAISelectionErrorAccountRepoStub) GetByID(_ context.Context, id int64) (*service.Account, error) {
	for i := range s.accounts {
		if s.accounts[i].ID == id {
			account := s.accounts[i]
			return &account, nil
		}
	}
	return nil, nil
}

func (s openAISelectionErrorAccountRepoStub) listByPlatform(platform string) []service.Account {
	out := make([]service.Account, 0, len(s.accounts))
	for _, account := range s.accounts {
		if account.Platform == platform {
			out = append(out, account)
		}
	}
	return out
}

func newOpenAISelectionErrorTestHandler(t *testing.T, accounts []service.Account) *OpenAIGatewayHandler {
	t.Helper()

	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Gateway.Scheduling.LoadBatchEnabled = false

	cache := &concurrencyCacheMock{
		acquireUserSlotFn: func(ctx context.Context, userID int64, maxConcurrency int, requestID string) (bool, error) {
			return true, nil
		},
		acquireAccountSlotFn: func(ctx context.Context, accountID int64, maxConcurrency int, requestID string) (bool, error) {
			return true, nil
		},
	}

	billingCacheService := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingCacheService.Stop)

	gatewayService := service.NewOpenAIGatewayService(
		openAISelectionErrorAccountRepoStub{accounts: accounts},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		cfg,
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

	return &OpenAIGatewayHandler{
		gatewayService:      gatewayService,
		billingCacheService: billingCacheService,
		apiKeyService:       &service.APIKeyService{},
		concurrencyHelper:   NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second),
		cfg:                 cfg,
	}
}

func newOpenAISelectionErrorTestHandlerWithAccountAcquire(
	t *testing.T,
	accounts []service.Account,
	acquireAccountSlotFn func(ctx context.Context, accountID int64, maxConcurrency int, requestID string) (bool, error),
) *OpenAIGatewayHandler {
	t.Helper()

	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	cfg.Gateway.Scheduling.FallbackWaitTimeout = 5 * time.Millisecond
	cfg.Gateway.Scheduling.FallbackMaxWaiting = 1

	cache := &concurrencyCacheMock{
		acquireUserSlotFn: func(ctx context.Context, userID int64, maxConcurrency int, requestID string) (bool, error) {
			return true, nil
		},
		acquireAccountSlotFn: acquireAccountSlotFn,
	}

	billingCacheService := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingCacheService.Stop)

	concurrencySvc := service.NewConcurrencyService(cache)
	gatewayService := service.NewOpenAIGatewayService(
		openAISelectionErrorAccountRepoStub{accounts: accounts},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		cfg,
		nil,
		concurrencySvc,
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

	return &OpenAIGatewayHandler{
		gatewayService:      gatewayService,
		billingCacheService: billingCacheService,
		apiKeyService:       &service.APIKeyService{},
		concurrencyHelper:   NewConcurrencyHelper(concurrencySvc, SSEPingFormatNone, time.Millisecond),
		cfg:                 cfg,
	}
}

func newOpenAISelectionErrorTestContext(path string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	groupID := int64(2)
	c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
		ID:      101,
		GroupID: &groupID,
		User:    &service.User{ID: 1},
		Group: &service.Group{
			ID:                   groupID,
			AllowImageGeneration: true,
		},
	})
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{
		UserID:      1,
		Concurrency: 1,
	})

	return c, w
}

func TestBuildOpenAISelectionFailureMessage_SupportingModel(t *testing.T) {
	err := errors.New("no available OpenAI accounts supporting model: gpt-5")
	got := buildOpenAISelectionFailureMessage(err, "Service temporarily unavailable")
	require.Equal(t, "No available accounts supporting model: gpt-5", got)
}

func TestBuildOpenAISelectionFailureMessage_GenericNoAvailable(t *testing.T) {
	got := buildOpenAISelectionFailureMessage(service.ErrNoAvailableAccounts, "Service temporarily unavailable")
	require.Equal(t, "No available accounts", got)
}

func TestBuildOpenAISelectionExhaustedMessage_LocalExclusionPrefersFallback(t *testing.T) {
	err := errors.New("no available OpenAI accounts supporting model: gpt-5")
	require.Equal(t, "No available accounts", buildOpenAISelectionExhaustedMessage(err, "No available accounts", true))
	require.Equal(t, "No available compatible accounts", buildOpenAISelectionExhaustedMessage(err, "No available compatible accounts", true))
}

func TestOpenAIResponses_SelectionFailure_ReturnsSupportingModelMessage(t *testing.T) {
	c, rec := newOpenAISelectionErrorTestContext("/v1/responses", `{"model":"gpt-5","input":"hello"}`)
	h := newOpenAISelectionErrorTestHandler(t, nil)

	h.Responses(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), `"message":"No available accounts supporting model: gpt-5"`)
}

func TestOpenAIResponses_SelectionFailureAfterLocalExclusion_ReturnsNoAvailableAccounts(t *testing.T) {
	c, rec := newOpenAISelectionErrorTestContext("/v1/responses", `{"model":"gpt-5","input":"hello"}`)
	accounts := []service.Account{
		{
			ID:          1,
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeAPIKey,
			Status:      service.StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Credentials: map[string]any{
				"model_mapping": map[string]any{"gpt-5": "gpt-5"},
			},
			Extra: map[string]any{
				"openai_responses_supported": true,
			},
		},
	}
	h := newOpenAISelectionErrorTestHandlerWithAccountAcquire(
		t,
		accounts,
		func(ctx context.Context, accountID int64, maxConcurrency int, requestID string) (bool, error) {
			return false, nil
		},
	)

	h.Responses(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), `"message":"No available accounts"`)
	require.NotContains(t, rec.Body.String(), `"message":"No available accounts supporting model: gpt-5"`)
}

func TestOpenAIChatCompletions_SelectionFailure_ReturnsSupportingModelMessage(t *testing.T) {
	c, rec := newOpenAISelectionErrorTestContext("/v1/chat/completions", `{"model":"gpt-5","messages":[{"role":"user","content":"hello"}]}`)
	h := newOpenAISelectionErrorTestHandler(t, nil)

	h.ChatCompletions(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), `"message":"No available accounts supporting model: gpt-5"`)
}

func TestOpenAIResponses_GroupModelUnsupported_ReturnsPermissionError(t *testing.T) {
	c, rec := newOpenAISelectionErrorTestContext("/v1/responses", `{"model":"gpt-5","input":"hello"}`)
	h := newOpenAISelectionErrorTestHandler(t, []service.Account{unsupportedOpenAITestAccount()})

	h.Responses(c)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), `"type":"permission_error"`)
	require.Contains(t, rec.Body.String(), `requested model \"gpt-5\"`)
	require.Contains(t, rec.Body.String(), "gpt-5.4-mini")
}

func TestOpenAIChatCompletions_GroupModelUnsupported_ReturnsPermissionError(t *testing.T) {
	c, rec := newOpenAISelectionErrorTestContext("/v1/chat/completions", `{"model":"gpt-5","messages":[{"role":"user","content":"hello"}]}`)
	h := newOpenAISelectionErrorTestHandler(t, []service.Account{unsupportedOpenAITestAccount()})

	h.ChatCompletions(c)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), `"type":"permission_error"`)
	require.Contains(t, rec.Body.String(), `requested model \"gpt-5\"`)
	require.Contains(t, rec.Body.String(), "gpt-5.4-mini")
}

func TestOpenAIEmbeddings_GroupModelUnsupported_ReturnsPermissionError(t *testing.T) {
	c, rec := newOpenAISelectionErrorTestContext("/v1/embeddings", `{"model":"gpt-5","input":"hello"}`)
	h := newOpenAISelectionErrorTestHandler(t, []service.Account{unsupportedOpenAITestAccount()})

	h.Embeddings(c)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), `"type":"permission_error"`)
	require.Contains(t, rec.Body.String(), `requested model \"gpt-5\"`)
	require.Contains(t, rec.Body.String(), "gpt-5.4-mini")
}

func unsupportedOpenAITestAccount() service.Account {
	return service.Account{
		ID:          1,
		Platform:    service.PlatformOpenAI,
		Status:      service.StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"gpt-5.4-mini": "gpt-5.4-mini",
			},
		},
	}
}
