//go:build unit

package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// wsTurnSlotHTTPUpstream 为 HTTP-bridge 多 turn 提供独立响应体。
type wsTurnSlotHTTPUpstream struct {
	turns atomic.Int64
}

func (u *wsTurnSlotHTTPUpstream) Do(*http.Request, string, int64, int) (*http.Response, error) {
	turn := u.turns.Add(1)
	body := fmt.Sprintf(
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_slot_turn_%d\",\"model\":\"gpt-5.1\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n"+
			"data: [DONE]\n\n",
		turn,
	)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func (u *wsTurnSlotHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

// TestOpenAIResponsesWebSocket_TurnReacquireUsesEffectiveConcurrencyWhenStoredZero
// 回归：调度器已用 EffectiveConcurrency() 抢到 turn 1（WaitPlan 为 nil）时，
// 后续 turn 不得把 stored Concurrency=0 传给 TryAcquireAccountSlotForGroup。
// HTTP-bridge 会调用 hooks.BeforeTurn，这是 handler 重抢槽位的路径。
func TestOpenAIResponsesWebSocket_TurnReacquireUsesEffectiveConcurrencyWhenStoredZero(t *testing.T) {
	gin.SetMode(gin.TestMode)

	account := service.Account{
		ID:          9902,
		Name:        "openai-ws-fallback-concurrency",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeOAuth,
		Status:      service.StatusActive,
		Schedulable: true,
		Concurrency: 0,
		Credentials: map[string]any{
			"access_token": "oauth-token",
		},
		Extra: map[string]any{
			"openai_oauth_responses_websockets_v2_enabled": true,
			"openai_oauth_responses_websockets_v2_mode":    service.OpenAIWSIngressModeHTTPBridge,
		},
	}
	wantMax := account.EffectiveConcurrency()
	require.Equal(t, 12, wantMax, "OpenAI OAuth stored 0 must fall back to platform default 12")

	cfg := &config.Config{}
	cfg.RunMode = config.RunModeSimple
	cfg.Default.RateMultiplier = 1
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3

	var (
		mu             sync.Mutex
		accountSlotMax []int
	)
	cache := &concurrencyCacheMock{
		acquireUserSlotFn: func(context.Context, int64, int, string) (bool, error) {
			return true, nil
		},
		acquireAccountSlotFn: func(_ context.Context, _ int64, maxConcurrency int, _ string) (bool, error) {
			mu.Lock()
			accountSlotMax = append(accountSlotMax, maxConcurrency)
			mu.Unlock()
			return true, nil
		},
	}
	concurrencySvc := service.NewConcurrencyService(cache)
	accountRepo := &openAIWSUsageHandlerAccountRepoStub{account: account}
	billingCacheSvc := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingCacheSvc.Stop)

	gatewaySvc := service.NewOpenAIGatewayService(
		accountRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		cfg,
		nil,
		concurrencySvc,
		service.NewBillingService(cfg, nil),
		nil,
		billingCacheSvc,
		&wsTurnSlotHTTPUpstream{},
		&service.DeferredService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	h := &OpenAIGatewayHandler{
		gatewayService:      gatewaySvc,
		billingCacheService: billingCacheSvc,
		apiKeyService:       &service.APIKeyService{},
		concurrencyHelper:   NewConcurrencyHelper(concurrencySvc, SSEPingFormatNone, time.Second),
		cfg:                 cfg,
	}

	groupID := int64(4202)
	apiKey := &service.APIKey{
		ID:      1802,
		GroupID: &groupID,
		User:    &service.User{ID: 1702, Status: service.StatusActive},
		Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI, Status: service.StatusActive},
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), apiKey)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.User.ID, Concurrency: 1})
		c.Next()
	})
	router.GET("/openai/v1/responses", h.ResponsesWebSocket)
	handlerServer := httptest.NewServer(router)
	defer handlerServer.Close()

	dialCtx, cancelDial := context.WithTimeout(context.Background(), 3*time.Second)
	clientConn, _, err := coderws.Dial(
		dialCtx,
		"ws"+strings.TrimPrefix(handlerServer.URL, "http")+"/openai/v1/responses",
		&coderws.DialOptions{CompressionMode: coderws.CompressionContextTakeover},
	)
	cancelDial()
	require.NoError(t, err)
	defer func() { _ = clientConn.CloseNow() }()

	writeMessage := func(payload string) {
		t.Helper()
		writeCtx, cancelWrite := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancelWrite()
		require.NoError(t, clientConn.Write(writeCtx, coderws.MessageText, []byte(payload)))
	}
	readCompleted := func(wantID string) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for {
			readCtx, cancelRead := context.WithTimeout(context.Background(), time.Until(deadline))
			_, event, readErr := clientConn.Read(readCtx)
			cancelRead()
			require.NoError(t, readErr)
			if gjson.GetBytes(event, "type").String() == "response.completed" {
				require.Equal(t, wantID, gjson.GetBytes(event, "response.id").String())
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("did not receive response.completed %s", wantID)
			}
		}
	}

	writeMessage(`{"type":"response.create","model":"gpt-5.1","stream":false}`)
	readCompleted("resp_slot_turn_1")

	writeMessage(`{"type":"response.create","model":"gpt-5.1","stream":false,"previous_response_id":"resp_slot_turn_1"}`)
	readCompleted("resp_slot_turn_2")
	_ = clientConn.Close(coderws.StatusNormalClosure, "done")

	mu.Lock()
	gotMax := append([]int(nil), accountSlotMax...)
	mu.Unlock()
	require.NotEmpty(t, gotMax, "调度器或 turn 重抢必须调用账号槽位")
	for _, maxConcurrency := range gotMax {
		require.Equal(t, wantMax, maxConcurrency, "stored 0 必须按 EffectiveConcurrency() 抢槽，不能把 0 传给 fail-closed 路径")
	}
}
