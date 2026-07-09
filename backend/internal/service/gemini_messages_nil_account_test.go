//go:build unit

package service

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeminiMessagesCompatServiceForwardRejectsNilAccount(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)

	svc := &GeminiMessagesCompatService{}
	body := []byte(`{"model":"claude-sonnet-4","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)

	result, err := svc.Forward(context.Background(), c, nil, body)

	require.Nil(t, result)
	require.EqualError(t, err, "account is required")
}

func TestGeminiMessagesCompatServiceForwardNativeRejectsNilAccount(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/v1beta/models/gemini-2.5-pro:generateContent", nil)

	svc := &GeminiMessagesCompatService{}
	body := []byte(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`)

	result, err := svc.ForwardNative(context.Background(), c, nil, "gemini-2.5-pro", "generateContent", false, body)

	require.Nil(t, result)
	require.EqualError(t, err, "account is required")
}
