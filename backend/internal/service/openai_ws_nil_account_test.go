//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayServiceBuildOpenAIResponsesWSURLRejectsNilAccount(t *testing.T) {
	svc := &OpenAIGatewayService{}

	wsURL, err := svc.buildOpenAIResponsesWSURL(nil)

	require.Empty(t, wsURL)
	require.EqualError(t, err, "account is required")
}

func TestOpenAIGatewayServiceProxyOpenAIWSHTTPBridgeTurnRejectsNilAccount(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/ws-bridge", nil)

	svc := &OpenAIGatewayService{
		httpUpstream: &httpUpstreamRecorder{},
	}

	result, err := svc.proxyOpenAIWSHTTPBridgeTurn(
		context.Background(),
		c,
		nil,
		"token",
		[]byte(`{"type":"response.create","model":"gpt-5","stream":true,"input":"hi"}`),
		64,
		"gpt-5",
		"",
		"",
		"",
		1,
		func([]byte) error { return nil },
	)

	require.Nil(t, result)
	require.EqualError(t, err, "account is required")
}

func TestOpenAIGatewayServiceProxyResponsesWebSocketFromClientRejectsNilAccount(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/ws", nil)

	svc := &OpenAIGatewayService{}

	err := svc.ProxyResponsesWebSocketFromClient(
		context.Background(),
		c,
		&coderws.Conn{},
		nil,
		"token",
		[]byte(`{"type":"response.create","model":"gpt-5","stream":true,"input":"hi"}`),
		nil,
	)

	require.EqualError(t, err, "account is required")
}

func TestOpenAIGatewayServiceProxyResponsesWebSocketFromClientRejectsNilService(t *testing.T) {
	err := (*OpenAIGatewayService)(nil).ProxyResponsesWebSocketFromClient(
		context.Background(),
		nil,
		nil,
		nil,
		"token",
		[]byte(`{"type":"response.create","model":"gpt-5","stream":true,"input":"hi"}`),
		nil,
	)

	require.EqualError(t, err, "service is required")
}

func TestOpenAIGatewayServiceProxyResponsesWebSocketFromClientRejectsNilGinContext(t *testing.T) {
	svc := &OpenAIGatewayService{}

	err := svc.ProxyResponsesWebSocketFromClient(
		context.Background(),
		nil,
		nil,
		nil,
		"token",
		[]byte(`{"type":"response.create","model":"gpt-5","stream":true,"input":"hi"}`),
		nil,
	)

	require.EqualError(t, err, "gin context is required")
}

func TestOpenAIGatewayServiceProxyResponsesWebSocketFromClientRejectsNilClientWebsocket(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/ws", nil)

	svc := &OpenAIGatewayService{}

	err := svc.ProxyResponsesWebSocketFromClient(
		context.Background(),
		c,
		nil,
		nil,
		"token",
		[]byte(`{"type":"response.create","model":"gpt-5","stream":true,"input":"hi"}`),
		nil,
	)

	require.EqualError(t, err, "client websocket is required")
}

func TestOpenAIGatewayServiceProxyResponsesWebSocketFromClientRejectsNilConnPool(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/ws", nil)

	svc := &OpenAIGatewayService{}
	svc.cfg = &config.Config{}
	svc.cfg.Gateway.OpenAIWS.Enabled = true
	svc.cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	svc.cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	svc.cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	svc.openaiWSPoolOnce.Do(func() {})

	account := &Account{
		ID:          1,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
	}

	err := svc.ProxyResponsesWebSocketFromClient(
		context.Background(),
		c,
		&coderws.Conn{},
		account,
		"token",
		[]byte(`{"type":"response.create","model":"gpt-5","stream":true,"input":"hi"}`),
		nil,
	)

	require.EqualError(t, err, "openai ws conn pool is required")
}

func TestOpenAIGatewayServiceForwardOpenAIWSV2RejectsNilAccount(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)

	svc := &OpenAIGatewayService{}

	result, err := svc.forwardOpenAIWSV2(
		context.Background(),
		c,
		nil,
		map[string]any{"model": "gpt-5.1", "stream": true},
		"token",
		OpenAIWSProtocolDecision{Transport: OpenAIUpstreamTransportResponsesWebsocketV2},
		true,
		true,
		"gpt-5.1",
		"gpt-5.1",
		time.Now(),
		1,
		"",
	)

	require.Nil(t, result)

	var fallbackErr *openAIWSFallbackError
	require.True(t, errors.As(err, &fallbackErr))
	require.Equal(t, "invalid_state", fallbackErr.Reason)
	require.EqualError(t, fallbackErr.Err, "account is required")
}

func TestOpenAIGatewayServiceForwardOpenAIWSV2RejectsNilService(t *testing.T) {
	result, err := (*OpenAIGatewayService)(nil).forwardOpenAIWSV2(
		context.Background(),
		nil,
		nil,
		map[string]any{"model": "gpt-5.1", "stream": true},
		"token",
		OpenAIWSProtocolDecision{Transport: OpenAIUpstreamTransportResponsesWebsocketV2},
		true,
		true,
		"gpt-5.1",
		"gpt-5.1",
		time.Now(),
		1,
		"",
	)

	require.Nil(t, result)

	var fallbackErr *openAIWSFallbackError
	require.True(t, errors.As(err, &fallbackErr))
	require.Equal(t, "invalid_state", fallbackErr.Reason)
	require.EqualError(t, fallbackErr.Err, "service is required")
}

func TestOpenAIGatewayServiceProxyResponsesWebSocketV2PassthroughRejectsNilAccount(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/ws", nil)

	svc := &OpenAIGatewayService{}

	err := svc.proxyResponsesWebSocketV2Passthrough(
		context.Background(),
		c,
		&coderws.Conn{},
		nil,
		"token",
		[]byte(`{"type":"response.create","model":"gpt-5.1","stream":true}`),
		nil,
		OpenAIWSProtocolDecision{Transport: OpenAIUpstreamTransportResponsesWebsocketV2},
	)

	require.EqualError(t, err, "account is required")
}
