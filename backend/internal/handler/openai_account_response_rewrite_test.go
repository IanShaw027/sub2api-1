package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type openAIAccountRewritePassthroughRepoStub struct {
	service.ErrorPassthroughRepository
	rules []*model.ErrorPassthroughRule
}

func (s openAIAccountRewritePassthroughRepoStub) List(context.Context) ([]*model.ErrorPassthroughRule, error) {
	return s.rules, nil
}

func TestRewriteOpenAIUpstreamErrorForAccount_UsesRawUpstreamStatus(t *testing.T) {
	account := &service.Account{
		Platform: service.PlatformOpenAI,
		Credentials: map[string]any{
			"response_rewrite_rules": []any{
				map[string]any{
					"status_code":      float64(401),
					"keywords":         []any{"token_invalidated"},
					"match_mode":       "all",
					"response_message": "Service temporarily unavailable",
				},
			},
		},
	}

	h := &OpenAIGatewayHandler{}
	body := []byte(`{"error":{"message":"Your authentication token has been invalidated.","code":"token_invalidated"}}`)

	msg, matched := h.rewriteOpenAIUpstreamErrorForAccount(account, 401, body)
	require.True(t, matched)
	require.Equal(t, "Service temporarily unavailable", msg)

	msg, matched = h.rewriteOpenAIUpstreamErrorForAccount(account, 502, body)
	require.False(t, matched)
	require.Empty(t, msg)
}

func TestHandleFailoverExhausted_UsesPassedFinalAccountRule(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	accountA := &service.Account{
		Platform: service.PlatformOpenAI,
		Credentials: map[string]any{
			"response_rewrite_rules": []any{
				map[string]any{
					"status_code":      float64(429),
					"keywords":         []any{"too many requests"},
					"match_mode":       "all",
					"response_message": "A message",
				},
			},
		},
	}
	accountB := &service.Account{
		Platform: service.PlatformOpenAI,
		Credentials: map[string]any{
			"response_rewrite_rules": []any{
				map[string]any{
					"status_code":      float64(429),
					"keywords":         []any{"too many requests"},
					"match_mode":       "all",
					"response_message": "B message",
				},
			},
		},
	}

	h := &OpenAIGatewayHandler{}
	body := []byte(`{"error":{"message":"Too many requests, please wait before trying again."}}`)

	h.handleFailoverExhausted(c, &service.UpstreamFailoverError{
		StatusCode:   http.StatusTooManyRequests,
		ResponseBody: body,
	}, accountB, false)

	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Contains(t, w.Body.String(), `"message":"B message"`)
	require.NotEqual(t, accountA.MatchResponseRewriteRule(http.StatusTooManyRequests, body).ResponseMessage, "B message")
}

func TestHandleFailoverExhausted_AccountRewritePrecedesGlobalPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	globalStatus := http.StatusBadGateway
	globalMessage := "Global passthrough message"
	passthroughService := service.NewErrorPassthroughService(openAIAccountRewritePassthroughRepoStub{
		rules: []*model.ErrorPassthroughRule{
			{
				ID:              1,
				Name:            "global-openai-429",
				Enabled:         true,
				Priority:        1,
				ErrorCodes:      []int{http.StatusTooManyRequests},
				Keywords:        []string{"too many requests"},
				MatchMode:       model.MatchModeAll,
				Platforms:       []string{model.PlatformOpenAI},
				PassthroughCode: false,
				ResponseCode:    &globalStatus,
				PassthroughBody: false,
				CustomMessage:   &globalMessage,
			},
		},
	}, nil)

	account := &service.Account{
		Platform: service.PlatformOpenAI,
		Credentials: map[string]any{
			"response_rewrite_rules": []any{
				map[string]any{
					"status_code":      float64(429),
					"keywords":         []any{"too many requests"},
					"match_mode":       "all",
					"response_message": "Account rewrite message",
				},
			},
		},
	}

	h := &OpenAIGatewayHandler{errorPassthroughService: passthroughService}
	h.handleFailoverExhausted(c, &service.UpstreamFailoverError{
		StatusCode:   http.StatusTooManyRequests,
		ResponseBody: []byte(`{"error":{"message":"Too many requests, please wait before trying again."}}`),
	}, account, false)

	require.Equal(t, http.StatusTooManyRequests, w.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	errObj, ok := payload["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "rate_limit_error", errObj["type"])
	require.Equal(t, "Account rewrite message", errObj["message"])
}

func TestHandleFailoverExhausted_AppliesGrokScopedPassthroughForGrokAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	responseCode := http.StatusTeapot
	customMessage := "Grok global passthrough"
	passthroughService := service.NewErrorPassthroughService(openAIAccountRewritePassthroughRepoStub{
		rules: []*model.ErrorPassthroughRule{
			{
				ID:              2,
				Name:            "global-grok-429",
				Enabled:         true,
				Priority:        1,
				ErrorCodes:      []int{http.StatusTooManyRequests},
				Keywords:        []string{"too many requests"},
				MatchMode:       model.MatchModeAll,
				Platforms:       []string{model.PlatformGrok},
				PassthroughCode: false,
				ResponseCode:    &responseCode,
				PassthroughBody: false,
				CustomMessage:   &customMessage,
			},
		},
	}, nil)

	account := &service.Account{Platform: service.PlatformGrok}
	h := &OpenAIGatewayHandler{errorPassthroughService: passthroughService}
	h.handleFailoverExhausted(c, &service.UpstreamFailoverError{
		StatusCode:   http.StatusTooManyRequests,
		ResponseBody: []byte(`{"error":{"message":"Too many requests, please wait before trying again."}}`),
	}, account, false)

	require.Equal(t, http.StatusTeapot, w.Code)
	require.Contains(t, w.Body.String(), "Grok global passthrough")
}
