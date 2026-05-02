package routes

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterMediaRoutesIncludesPublicUserAndAdminEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	v1 := router.Group("/api/v1")

	RegisterMediaRoutes(
		v1,
		&handler.Handlers{
			Media: &handler.MediaHandler{},
			Admin: &handler.AdminHandlers{
				Media: &admin.MediaHandler{},
			},
		},
		servermiddleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() }),
		servermiddleware.AdminAuthMiddleware(func(c *gin.Context) { c.Next() }),
		nil,
	)

	expected := map[string]struct{}{
		http.MethodGet + " /api/v1/media/public/:id":                   {},
		http.MethodGet + " /api/v1/media/public/:id/thumbnail":         {},
		http.MethodGet + " /api/v1/media/download/:id":                 {},
		http.MethodGet + " /api/v1/media/download/:id/thumbnail":       {},
		http.MethodPost + " /api/v1/media/upload":                      {},
		http.MethodGet + " /api/v1/media/:id":                          {},
		http.MethodDelete + " /api/v1/media/:id":                       {},
		http.MethodPost + " /api/v1/media/:id/visibility":              {},
		http.MethodPost + " /api/v1/media/:id/presign-download":        {},
		http.MethodGet + " /api/v1/admin/media":                        {},
		http.MethodPost + " /api/v1/admin/media/upload":                {},
		http.MethodGet + " /api/v1/admin/media/:id":                    {},
		http.MethodDelete + " /api/v1/admin/media/:id":                 {},
		http.MethodPost + " /api/v1/admin/media/:id/visibility":        {},
		http.MethodPost + " /api/v1/admin/media/:id/presign-download":  {},
	}

	for _, route := range router.Routes() {
		delete(expected, route.Method+" "+route.Path)
	}
	require.Empty(t, expected)
}
