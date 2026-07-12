package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterAdminRoutesIncludesKiroAccountEndpoints(t *testing.T) {
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
		"GET /api/v1/admin/accounts/:id/profiles":         {},
		"POST /api/v1/admin/accounts/:id/kiro/overage":   {},
		"POST /api/v1/admin/accounts/kiro/overage/enable-all": {},
	}

	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		delete(expected, key)
	}

	require.Empty(t, expected, "expected Kiro account routes to be registered")
}
