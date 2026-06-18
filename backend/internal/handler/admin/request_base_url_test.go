package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestBaseURLIgnoresInvalidForwardedHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "https://admin.good.example/path", nil)
	c.Request.Header.Set("X-Forwarded-Proto", "javascript")
	c.Request.Header.Set("X-Forwarded-Host", "evil.example/path")

	require.Equal(t, "https://admin.good.example", requestBaseURL(c))
}

func TestRequestBaseURLDoesNotTrustForwardedHost(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "https://admin.good.example/path", nil)
	c.Request.Header.Set("X-Forwarded-Proto", "https")
	c.Request.Header.Set("X-Forwarded-Host", "evil.example")

	require.Equal(t, "https://admin.good.example", requestBaseURL(c))
}
