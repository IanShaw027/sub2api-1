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

func TestRegisterAdminRoutesIncludesAIGovernanceEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	v1 := router.Group("/api/v1")

	RegisterAdminRoutes(
		v1,
		&handler.Handlers{
			Admin: &handler.AdminHandlers{
				AI: &admin.AIHandler{},
			},
		},
		servermiddleware.AdminAuthMiddleware(func(c *gin.Context) { c.Next() }),
		nil,
	)

	expected := map[string]struct{}{
		http.MethodGet + " /api/v1/admin/ai/prompt-templates":                {},
		http.MethodGet + " /api/v1/admin/ai/prompt-templates/:id":            {},
		http.MethodPut + " /api/v1/admin/ai/prompt-templates/:id/moderation": {},
		http.MethodGet + " /api/v1/admin/ai/prompts":                         {},
		http.MethodPut + " /api/v1/admin/ai/prompts/:id":                      {},
		http.MethodDelete + " /api/v1/admin/ai/prompts/:id":                   {},
		http.MethodGet + " /api/v1/admin/ai/generation-jobs":                  {},
		http.MethodGet + " /api/v1/admin/ai/generation-jobs/:id":              {},
		http.MethodPut + " /api/v1/admin/ai/generation-jobs/:id/status":       {},
		http.MethodGet + " /api/v1/admin/ai/assets":                           {},
		http.MethodGet + " /api/v1/admin/ai/assets/:id":                       {},
		http.MethodPut + " /api/v1/admin/ai/assets/:id/moderation":            {},
		http.MethodGet + " /api/v1/admin/ai/artworks":                         {},
		http.MethodPut + " /api/v1/admin/ai/artworks/:id":                     {},
		http.MethodDelete + " /api/v1/admin/ai/artworks/:id":                  {},
		http.MethodGet + " /api/v1/admin/ai/audit-logs":                       {},
	}

	for _, route := range router.Routes() {
		delete(expected, route.Method+" "+route.Path)
	}
	require.Empty(t, expected)
}
