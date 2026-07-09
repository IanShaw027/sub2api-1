//go:build unit

package service

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayServiceForwardAsChatCompletionsRejectsNilAccount(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"gpt-5.1","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	svc := &OpenAIGatewayService{}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, nil, body, "", "")

	require.Nil(t, result)
	require.EqualError(t, err, "account is required")
}

func TestOpenAIGatewayServiceForwardAsAnthropicRejectsNilAccount(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"gpt-5.1","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	svc := &OpenAIGatewayService{}

	result, err := svc.ForwardAsAnthropic(context.Background(), c, nil, body, "", "")

	require.Nil(t, result)
	require.EqualError(t, err, "account is required")
}

func TestOpenAIGatewayServiceForwardEmbeddingsRejectsNilAccount(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"text-embedding-3-small","input":"hello"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	svc := &OpenAIGatewayService{}

	result, err := svc.ForwardEmbeddings(context.Background(), c, nil, body, "")

	require.Nil(t, result)
	require.EqualError(t, err, "account is required")
}
