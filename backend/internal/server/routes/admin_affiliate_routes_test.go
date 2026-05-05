package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterAdminRoutesIncludesAffiliateEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	v1 := router.Group("/api/v1")

	RegisterAdminRoutes(
		v1,
		&handler.Handlers{
			Admin: &handler.AdminHandlers{
				Affiliate: &adminhandler.AffiliateHandler{},
			},
		},
		servermiddleware.AdminAuthMiddleware(func(c *gin.Context) {
			c.Next()
		}),
		nil,
	)

	expected := map[string]struct{}{
		"GET /api/v1/admin/affiliates/users":                   {},
		"PUT /api/v1/admin/affiliates/users/:user_id":          {},
		"DELETE /api/v1/admin/affiliates/users/:user_id":       {},
		"POST /api/v1/admin/affiliates/users/batch-rate":       {},
		"GET /api/v1/admin/affiliates/users/lookup":            {},
		"GET /api/v1/admin/affiliates/users/:user_id/overview": {},
		"GET /api/v1/admin/affiliates/invites":                 {},
		"GET /api/v1/admin/affiliates/rebates":                 {},
		"GET /api/v1/admin/affiliates/transfers":               {},
	}

	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		delete(expected, key)
	}

	require.Empty(t, expected, "expected affiliate admin routes to be registered")
}
