//go:build unit

package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterAdminRoutesIncludesGrokOAuthEndpoints(t *testing.T) {
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
	)

	expected := map[string]struct{}{
		"POST /api/v1/admin/grok/oauth/auth-url":           {},
		"POST /api/v1/admin/grok/oauth/exchange-code":      {},
		"POST /api/v1/admin/grok/oauth/refresh-token":      {},
		"POST /api/v1/admin/grok/oauth/create-from-oauth":  {},
		"POST /api/v1/admin/grok/accounts/:id/refresh":     {},
		"GET /api/v1/admin/grok/accounts/:id/quota":        {},
		"POST /api/v1/admin/grok/accounts/:id/reset-quota": {},
		"GET /api/v1/admin/grok/runtime-sanity":            {},
	}

	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		delete(expected, key)
	}

	require.Empty(t, expected, "expected grok oauth routes to be registered")
}
