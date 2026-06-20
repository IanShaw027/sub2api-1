package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

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
