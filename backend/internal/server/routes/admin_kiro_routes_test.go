package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterAdminRoutesIncludesKiroOAuthEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	v1 := router.Group("/api/v1")

	RegisterAdminRoutes(
		v1,
		&handler.Handlers{
			Admin: &handler.AdminHandlers{},
		},
		servermiddleware.AdminAuthMiddleware(func(c *gin.Context) {
			c.Next()
		}),
		nil,
		nil,
		nil,
		nil,
	)

	expected := map[string]struct{}{
		"POST /api/v1/admin/kiro/oauth/auth-url":          {},
		"POST /api/v1/admin/kiro/oauth/exchange-callback": {},
		"POST /api/v1/admin/kiro/oauth/device-complete":   {},
		"POST /api/v1/admin/kiro/oauth/refresh-token":     {},
		"POST /api/v1/admin/kiro/oauth/discover-profiles": {},
	}

	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		delete(expected, key)
	}

	require.Empty(t, expected, "expected kiro oauth routes to be registered")
}
