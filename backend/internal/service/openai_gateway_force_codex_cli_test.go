package service

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDetectOpenAIPassthroughInstructionsRejectReason_RespectsForceCodexCLI(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)

	body := []byte(`{"model":"gpt-5.4"}`)

	require.Empty(t, detectOpenAIPassthroughInstructionsRejectReason(c, "gpt-5.4", body, false))
	require.Equal(t, "instructions_missing", detectOpenAIPassthroughInstructionsRejectReason(c, "gpt-5.4", body, true))
}
