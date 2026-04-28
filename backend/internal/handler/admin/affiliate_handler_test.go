package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestParseAffiliateDateQueryUsesUserTimezone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/?start_date=2026-04-28&timezone=America/Los_Angeles",
		nil,
	)

	got, ok := parseAffiliateDateQuery(ctx, "start_date", "America/Los_Angeles")

	require.True(t, ok)
	require.NotNil(t, got)
	expectedLoc, err := time.LoadLocation("America/Los_Angeles")
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 4, 28, 0, 0, 0, 0, expectedLoc), *got)
}
