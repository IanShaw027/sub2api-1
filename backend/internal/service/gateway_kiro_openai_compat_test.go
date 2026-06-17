//go:build unit

package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestEnsureKiroOpenAICompatSessionMetadata_UsesPromptCacheKeySeed(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"prompt_cache_key":"session-abc"}`))

	updated, parsed := ensureKiroOpenAICompatSessionMetadata(
		[]byte(`{"prompt_cache_key":"session-abc"}`),
		c,
		&ParsedRequest{},
		[]byte(`{"model":"claude-sonnet-4-6"}`),
	)

	require.NotNil(t, parsed)
	require.NotEmpty(t, parsed.MetadataUserID)
	require.Contains(t, string(updated), `"metadata"`)
	meta := ParseMetadataUserID(parsed.MetadataUserID)
	require.NotNil(t, meta)
	require.Equal(t, GenerateSessionUUID("kiro:session-abc"), meta.SessionID)
}
