//go:build unit

package service

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestKiroGatewayServiceForwardRejectsNilAccount(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)

	svc := &KiroGatewayService{}
	parsed := &ParsedRequest{
		Body:  NewRequestBodyRef([]byte(`{"model":"claude-sonnet-4-5-20250929","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)),
		Model: "claude-sonnet-4-5-20250929",
	}

	result, err := svc.Forward(context.Background(), c, nil, parsed)

	require.Nil(t, result)
	require.EqualError(t, err, "account is required")
}

func TestKiroGatewayServiceForwardCountTokensRejectsNilAccount(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/v1/messages/count_tokens", nil)

	svc := &KiroGatewayService{}
	parsed := &ParsedRequest{
		Body:  NewRequestBodyRef([]byte(`{"model":"claude-sonnet-4-5-20250929","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)),
		Model: "claude-sonnet-4-5-20250929",
	}

	err := svc.ForwardCountTokens(context.Background(), c, nil, parsed)

	require.EqualError(t, err, "account is required")
}
