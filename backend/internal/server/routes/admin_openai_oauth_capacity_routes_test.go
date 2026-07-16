package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterAdminRoutesIncludesOpenAIOAuthCapacityEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	v1 := router.Group("/api/v1")
	RegisterAdminRoutes(
		v1,
		&handler.Handlers{Admin: &handler.AdminHandlers{}},
		servermiddleware.AdminAuthMiddleware(func(c *gin.Context) { c.Next() }),
		nil,
	)

	registered := false
	timeseriesRegistered := false
	for _, route := range router.Routes() {
		if route.Method == "GET" && route.Path == "/api/v1/admin/accounts/openai-oauth-capacity" {
			registered = true
		}
		if route.Method == "GET" && route.Path == "/api/v1/admin/accounts/openai-oauth-capacity/timeseries" {
			timeseriesRegistered = true
		}
	}
	require.True(t, registered, "expected OpenAI OAuth capacity route to be registered")
	require.True(t, timeseriesRegistered, "expected OpenAI OAuth capacity timeseries route to be registered")
}
