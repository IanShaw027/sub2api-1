//go:build unit

package service

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayServiceForwardRejectsNilAccount(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)

	svc := &OpenAIGatewayService{}
	body := []byte(`{"model":"gpt-5.4","stream":false,"input":[{"type":"message","role":"user","content":"hello"}]}`)

	result, err := svc.Forward(context.Background(), c, nil, body)

	require.Nil(t, result)
	require.EqualError(t, err, "account is required")
}
