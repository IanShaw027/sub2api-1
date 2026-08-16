package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterAdminRoutesIncludesDeviceProfileEndpoints(t *testing.T) {
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
		"GET /api/v1/admin/accounts/:id/device-profile":       {},
		"POST /api/v1/admin/accounts/:id/reset-device-profile": {},
	}

	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		delete(expected, key)
	}

	require.Empty(t, expected, "expected device-profile routes to be registered")
}
