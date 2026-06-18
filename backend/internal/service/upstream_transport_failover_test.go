package service

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIForward_TransportErrorsReturnFailoverWithoutWriting(t *testing.T) {
	setGinTestMode()

	tests := []struct {
		name        string
		body        []byte
		passthrough bool
	}{
		{
			name: "responses",
			body: []byte(`{"model":"gpt-5.4","input":[{"role":"user","content":[{"type":"input_text","text":"hello"}]}],"stream":false}`),
		},
		{
			name:        "passthrough",
			body:        []byte(`{"model":"gpt-5.4","input":[{"role":"user","content":[{"type":"input_text","text":"hello"}]}],"stream":false}`),
			passthrough: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")
			SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

			account := &Account{
				ID:          101,
				Name:        "openai-oauth",
				Platform:    PlatformOpenAI,
				Type:        AccountTypeOAuth,
				Concurrency: 1,
				Credentials: map[string]any{
					"access_token":       "oauth-token",
					"chatgpt_account_id": "chatgpt-account",
				},
			}
			if tt.passthrough {
				account.Extra = map[string]any{"openai_passthrough": true}
			}
			svc := &OpenAIGatewayService{
				cfg:          &config.Config{},
				httpUpstream: &httpUpstreamRecorder{err: errors.New("dial tcp timeout")},
			}

			result, err := svc.Forward(context.Background(), c, account, tt.body)

			require.Nil(t, result)
			require.Error(t, err)
			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr)
			require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
			require.False(t, c.Writer.Written(), "transport failover must let the handler own the response")
			require.Empty(t, rec.Body.String())
		})
	}
}

func TestOpenAIForwardAsChatCompletions_TransportErrorReturnsFailoverWithoutWriting(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: &httpUpstreamRecorder{err: errors.New("dial tcp timeout")},
	}
	account := &Account{
		ID:          102,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-account",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.4")

	require.Nil(t, result)
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.False(t, c.Writer.Written(), "transport failover must let the handler own the response")
	require.Empty(t, rec.Body.String())
}

func TestGatewayForwardAsChatCompletions_TransportErrorReturnsFailoverWithoutWriting(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"claude-3-7-sonnet-20250219","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	svc := &GatewayService{
		cfg:          &config.Config{},
		httpUpstream: &anthropicHTTPUpstreamRecorder{err: errors.New("dial tcp timeout")},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, newAnthropicAPIKeyAccountForTest(), body, &ParsedRequest{
		Model:  "claude-3-7-sonnet-20250219",
		Stream: false,
	})

	require.Nil(t, result)
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.False(t, c.Writer.Written(), "transport failover must let the handler own the response")
	require.Empty(t, rec.Body.String())
}

func TestGatewayForwardAsResponses_TransportErrorReturnsFailoverWithoutWriting(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"claude-3-7-sonnet-20250219","input":[{"role":"user","content":[{"type":"input_text","text":"hello"}]}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	svc := &GatewayService{
		cfg:          &config.Config{},
		httpUpstream: &anthropicHTTPUpstreamRecorder{err: errors.New("dial tcp timeout")},
	}

	result, err := svc.ForwardAsResponses(context.Background(), c, newAnthropicAPIKeyAccountForTest(), body, &ParsedRequest{
		Model:  "claude-3-7-sonnet-20250219",
		Stream: false,
	})

	require.Nil(t, result)
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.False(t, c.Writer.Written(), "transport failover must let the handler own the response")
	require.Empty(t, rec.Body.String())
}
