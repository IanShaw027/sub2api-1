//go:build unit

package handler

import (
	"net/http"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyRevealTOTPCodeOnlyAcceptsHeaderOrBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("header", func(t *testing.T) {
		c, _ := gin.CreateTestContext(nil)
		c.Request = newHandlerRequest(t, `{"totp_code":"body-code"}`)
		c.Request.Header.Set("X-TOTP-Code", " header-code ")
		require.Equal(t, "header-code", apiKeyRevealTOTPCode(c))
	})

	t.Run("json body", func(t *testing.T) {
		c, _ := gin.CreateTestContext(nil)
		c.Request = newHandlerRequest(t, `{"totp_code":" body-code "}`)
		require.Equal(t, "body-code", apiKeyRevealTOTPCode(c))
	})

	t.Run("query is ignored", func(t *testing.T) {
		c, _ := gin.CreateTestContext(nil)
		c.Request = newHandlerRequest(t, "")
		c.Request.URL.RawQuery = "totp_code=query-secret"
		require.Empty(t, apiKeyRevealTOTPCode(c))
	})

	t.Run("malformed body is ignored", func(t *testing.T) {
		c, _ := gin.CreateTestContext(nil)
		c.Request = newHandlerRequest(t, `{"totp_code":`)
		require.Empty(t, apiKeyRevealTOTPCode(c))
	})
}

func TestAPIKeyRevealStepUpFailsClosedWhenGuardIsNotConfigured(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	c.Request = newHandlerRequest(t, "")

	err := (&APIKeyHandler{}).requireAPIKeyRevealStepUp(c, 7)

	require.Error(t, err)
	require.Equal(t, "TOTP_GUARD_NOT_CONFIGURED", infraerrors.Reason(err))
}

func newHandlerRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "/api/v1/keys/1/value", strings.NewReader(body))
	require.NoError(t, err)
	return req
}
