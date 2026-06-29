package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type openAIChatCompletionsBalanceUserRepoStub struct {
	service.UserRepository
	user service.User
}

func (s openAIChatCompletionsBalanceUserRepoStub) GetByID(ctx context.Context, id int64) (*service.User, error) {
	user := s.user
	if user.ID == 0 {
		user.ID = id
	}
	return &user, nil
}

type openAIChatCompletionsHTTPUpstreamStub struct{}

func (openAIChatCompletionsHTTPUpstreamStub) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	return http.DefaultClient.Do(req)
}

func (openAIChatCompletionsHTTPUpstreamStub) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	return http.DefaultClient.Do(req)
}

func TestOpenAIChatCompletions_RejectsInvalidStreamType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5","stream":"true","messages":[{"role":"user","content":"hi"}]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	groupID := int64(2)
	c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
		ID:      101,
		GroupID: &groupID,
		User:    &service.User{ID: 1},
	})
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{
		UserID:      1,
		Concurrency: 1,
	})

	cache := &concurrencyCacheMock{
		acquireUserSlotFn: func(ctx context.Context, userID int64, maxConcurrency int, requestID string) (bool, error) {
			return true, nil
		},
		acquireAccountSlotFn: func(ctx context.Context, accountID int64, maxConcurrency int, requestID string) (bool, error) {
			return true, nil
		},
	}
	h := &OpenAIGatewayHandler{
		gatewayService:      &service.OpenAIGatewayService{},
		billingCacheService: &service.BillingCacheService{},
		apiKeyService:       &service.APIKeyService{},
		concurrencyHelper:   NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second),
	}

	h.ChatCompletions(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "invalid stream field type")
}

func TestOpenAIChatCompletions_ClearsCompatRequestStateOnEarlyReturn(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("openai_stream_retry_replay_state", map[string]any{
		"visibleFrameSignatures": []string{"response.created"},
		"emittedTextPrefix":      "partial output",
	})

	h := &OpenAIGatewayHandler{}
	h.ChatCompletions(c)

	_, exists := c.Get("openai_stream_retry_replay_state")
	require.False(t, exists)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOpenAIChatCompletionsRequiredCapabilityUsesResponsesIngressForResponsesShape(t *testing.T) {
	require.Equal(t,
		service.OpenAIEndpointCapabilityResponsesIngress,
		openAIChatCompletionsRequiredCapability([]byte(`{"model":"gpt-5.5","input":"hello"}`)),
	)
	require.Equal(t,
		service.OpenAIEndpointCapabilityChatCompletions,
		openAIChatCompletionsRequiredCapability([]byte(`{"model":"gpt-5.5","messages":[{"role":"user","content":"hello"}]}`)),
	)
}

func TestOpenAIChatCompletions_RecordUsageIncludesRequestPayloadHash(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamBody []byte
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		var err error
		upstreamBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-request-id", "chatcmpl-hash-test")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-hash-test",
			"object":"chat.completion",
			"created":1710000000,
			"model":"gpt-5.1",
			"choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}
		}`))
	}))
	defer upstreamServer.Close()

	groupID := int64(2)
	apiKey := &service.APIKey{
		ID:      101,
		GroupID: &groupID,
		User:    &service.User{ID: 1, Balance: 100, Status: service.StatusActive},
		Group:   &service.Group{ID: groupID, RateMultiplier: 1},
	}
	account := service.Account{
		ID:          501,
		Name:        "openai-chat-hash-test",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": upstreamServer.URL,
		},
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesSupported: false,
		},
	}
	cfg := &config.Config{}
	cfg.RunMode = config.RunModeStandard
	cfg.Default.RateMultiplier = 1
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.Scheduling.LoadBatchEnabled = false

	accountRepo := openAISelectionErrorAccountRepoStub{accounts: []service.Account{account}}
	usageRepo := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 1)}
	billingRepo := &openAIWSUsageHandlerBillingRepoStub{applied: make(chan *service.UsageBillingCommand, 1)}
	gatewaySvc := service.NewOpenAIGatewayService(
		accountRepo,
		usageRepo,
		billingRepo,
		nil,
		nil,
		nil,
		nil,
		cfg,
		nil,
		nil,
		service.NewBillingService(cfg, nil),
		nil,
		service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil),
		openAIChatCompletionsHTTPUpstreamStub{},
		&service.DeferredService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil, // fingerprintNormalizer
	)
	billingCacheSvc := service.NewBillingCacheService(
		nil,
		openAIChatCompletionsBalanceUserRepoStub{user: *apiKey.User},
		nil,
		nil,
		nil,
		nil,
		cfg,
		nil,
	)
	t.Cleanup(billingCacheSvc.Stop)
	concurrencySvc := service.NewConcurrencyService(&concurrencyCacheMock{
		acquireUserSlotFn: func(ctx context.Context, userID int64, maxConcurrency int, requestID string) (bool, error) {
			return true, nil
		},
		acquireAccountSlotFn: func(ctx context.Context, accountID int64, maxConcurrency int, requestID string) (bool, error) {
			return true, nil
		},
	})
	h := &OpenAIGatewayHandler{
		gatewayService:      gatewaySvc,
		billingCacheService: billingCacheSvc,
		apiKeyService:       &service.APIKeyService{},
		concurrencyHelper:   NewConcurrencyHelper(concurrencySvc, SSEPingFormatNone, time.Second),
		cfg:                 cfg,
	}

	body := []byte(`{"model":"gpt-5.1","messages":[{"role":"user","content":"hash me"}]}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.User.ID, Concurrency: 1})

	h.ChatCompletions(c)

	require.Equalf(t, http.StatusOK, w.Code, "response body: %s", w.Body.String())
	require.Equal(t, "gpt-5.1", gjson.GetBytes(upstreamBody, "model").String())
	require.Equal(t, "hash me", gjson.GetBytes(upstreamBody, "messages.0.content").String())
	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, "chatcmpl-hash-test", response["id"])

	var cmd *service.UsageBillingCommand
	select {
	case cmd = <-billingRepo.applied:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for billing command")
	}
	require.Equal(t, service.HashUsageRequestPayload(body), cmd.RequestPayloadHash)
}
