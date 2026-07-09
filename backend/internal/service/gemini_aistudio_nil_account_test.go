//go:build unit

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeminiMessagesCompatServiceForwardAIStudioGETRejectsNilAccount(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)

	svc := &GeminiMessagesCompatService{}

	result, err := svc.ForwardAIStudioGET(context.Background(), c, nil, "/v1beta/models")

	require.Nil(t, result)
	require.EqualError(t, err, "account is required")
}
