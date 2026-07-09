//go:build unit

package service

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAntigravityGatewayServiceForwardRejectsNilAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	svc := &AntigravityGatewayService{}

	result, err := svc.Forward(context.Background(), c, nil, []byte(`{}`), false)

	require.Nil(t, result)
	require.EqualError(t, err, "account is required")
}

func TestAntigravityGatewayServiceForwardGeminiRejectsNilAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	svc := &AntigravityGatewayService{}

	result, err := svc.ForwardGemini(context.Background(), c, nil, "gemini-2.5-flash", "generateContent", false, []byte(`{}`), false)

	require.Nil(t, result)
	require.EqualError(t, err, "account is required")
}

func TestAntigravityGatewayServiceForwardUpstreamRejectsNilAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	svc := &AntigravityGatewayService{}

	result, err := svc.ForwardUpstream(context.Background(), c, nil, []byte(`{}`))

	require.Nil(t, result)
	require.EqualError(t, err, "account is required")
}

func TestAntigravityGatewayServiceTestConnectionRejectsNilAccount(t *testing.T) {
	svc := &AntigravityGatewayService{}

	result, err := svc.TestConnection(context.Background(), nil, "claude-sonnet-4-5")

	require.Nil(t, result)
	require.EqualError(t, err, "account is required")
}
