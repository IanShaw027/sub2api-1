//go:build unit

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gatewaySelectionErrorAccountRepoStub struct {
	service.AccountRepository
	accounts []service.Account
}

type gatewaySelectionErrorGroupRepoStub struct {
	service.GroupRepository
	group service.Group
}

func (s gatewaySelectionErrorAccountRepoStub) ListSchedulableByGroupIDAndPlatform(_ context.Context, _ int64, platform string) ([]service.Account, error) {
	return s.listByPlatform(platform), nil
}

func (s gatewaySelectionErrorAccountRepoStub) ListSchedulableByGroupIDAndPlatforms(_ context.Context, _ int64, platforms []string) ([]service.Account, error) {
	return s.listByPlatforms(platforms), nil
}

func (s gatewaySelectionErrorAccountRepoStub) ListSchedulableByPlatform(_ context.Context, platform string) ([]service.Account, error) {
	return s.listByPlatform(platform), nil
}

func (s gatewaySelectionErrorAccountRepoStub) ListSchedulableByPlatforms(_ context.Context, platforms []string) ([]service.Account, error) {
	return s.listByPlatforms(platforms), nil
}

func (s gatewaySelectionErrorAccountRepoStub) ListByGroup(_ context.Context, _ int64) ([]service.Account, error) {
	out := make([]service.Account, len(s.accounts))
	copy(out, s.accounts)
	return out, nil
}

func (s gatewaySelectionErrorAccountRepoStub) ListByPlatform(_ context.Context, platform string) ([]service.Account, error) {
	return s.listByPlatform(platform), nil
}

func (s gatewaySelectionErrorAccountRepoStub) GetByID(_ context.Context, id int64) (*service.Account, error) {
	for i := range s.accounts {
		if s.accounts[i].ID == id {
			account := s.accounts[i]
			return &account, nil
		}
	}
	return nil, nil
}

func (s gatewaySelectionErrorAccountRepoStub) listByPlatform(platform string) []service.Account {
	out := make([]service.Account, 0, len(s.accounts))
	for _, account := range s.accounts {
		if account.Platform == platform {
			out = append(out, account)
		}
	}
	return out
}

func (s gatewaySelectionErrorAccountRepoStub) listByPlatforms(platforms []string) []service.Account {
	allowed := make(map[string]struct{}, len(platforms))
	for _, platform := range platforms {
		allowed[platform] = struct{}{}
	}
	out := make([]service.Account, 0, len(s.accounts))
	for _, account := range s.accounts {
		if _, ok := allowed[account.Platform]; ok {
			out = append(out, account)
		}
	}
	return out
}

func (s gatewaySelectionErrorGroupRepoStub) GetByID(_ context.Context, id int64) (*service.Group, error) {
	if s.group.ID == id {
		group := s.group
		return &group, nil
	}
	return nil, service.ErrGroupNotFound
}

func (s gatewaySelectionErrorGroupRepoStub) GetByIDLite(ctx context.Context, id int64) (*service.Group, error) {
	return s.GetByID(ctx, id)
}

func newGatewaySelectionErrorTestHandler(t *testing.T, accounts []service.Account) *GatewayHandler {
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

	gatewayService := service.NewGatewayService(
		gatewaySelectionErrorAccountRepoStub{accounts: accounts},
		gatewaySelectionErrorGroupRepoStub{group: service.Group{
			ID:       2,
			Platform: service.PlatformAnthropic,
		}},
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
		billingCacheService,
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
		nil, // fingerprintNormalizer
	)

	return &GatewayHandler{
		gatewayService:      gatewayService,
		billingCacheService: billingCacheService,
		apiKeyService:       &service.APIKeyService{},
		concurrencyHelper:   NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second),
	}
}

func newGatewaySelectionErrorTestContext(path string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	groupID := int64(2)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID:      101,
		GroupID: &groupID,
		User:    &service.User{ID: 1},
		Group: &service.Group{
			ID: groupID,
		},
	})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{
		UserID:      1,
		Concurrency: 1,
	})

	return c, w
}

func TestGatewayChatCompletions_GroupModelUnsupported_ReturnsPermissionError(t *testing.T) {
	c, rec := newGatewaySelectionErrorTestContext("/v1/chat/completions", `{"model":"claude-missing","messages":[{"role":"user","content":"hello"}]}`)
	h := newGatewaySelectionErrorTestHandler(t, []service.Account{unsupportedAnthropicTestAccount()})

	h.ChatCompletions(c)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), `"type":"permission_error"`)
	require.Contains(t, rec.Body.String(), `requested model \"claude-missing\"`)
	require.Contains(t, rec.Body.String(), "claude-3-5-haiku-20241022")
}

func TestGatewayResponses_GroupModelUnsupported_ReturnsPermissionError(t *testing.T) {
	c, rec := newGatewaySelectionErrorTestContext("/v1/responses", `{"model":"claude-missing","input":"hello"}`)
	h := newGatewaySelectionErrorTestHandler(t, []service.Account{unsupportedAnthropicTestAccount()})

	h.Responses(c)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), `"type":"permission_error"`)
	require.Contains(t, rec.Body.String(), `requested model \"claude-missing\"`)
	require.Contains(t, rec.Body.String(), "claude-3-5-haiku-20241022")
}

func unsupportedAnthropicTestAccount() service.Account {
	return service.Account{
		ID:          1,
		Platform:    service.PlatformAnthropic,
		Status:      service.StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"claude-3-5-haiku-20241022": "claude-3-5-haiku-20241022",
			},
		},
	}
}
