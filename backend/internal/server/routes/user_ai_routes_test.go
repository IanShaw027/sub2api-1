package routes

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterUserRoutesIncludesProductionAIRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	v1 := router.Group("/api/v1")

	RegisterUserRoutes(
		v1,
		&handler.Handlers{
			AI: &handler.AIHandler{},
		},
		middleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() }),
		nil,
	)

	expected := map[string]struct{}{
		http.MethodGet + " /api/v1/user/ai/runtime":                             {},
		http.MethodGet + " /api/v1/user/ai/sessions":                            {},
		http.MethodPost + " /api/v1/user/ai/sessions":                           {},
		http.MethodGet + " /api/v1/user/ai/sessions/:id":                        {},
		http.MethodPut + " /api/v1/user/ai/sessions/:id":                        {},
		http.MethodDelete + " /api/v1/user/ai/sessions/:id":                     {},
		http.MethodGet + " /api/v1/user/ai/sessions/:id/messages":               {},
		http.MethodPost + " /api/v1/user/ai/sessions/:id/messages":              {},
		http.MethodGet + " /api/v1/user/ai/prompt-templates":                    {},
		http.MethodPost + " /api/v1/user/ai/prompt-templates":                   {},
		http.MethodGet + " /api/v1/user/ai/prompt-templates/:id":                {},
		http.MethodPut + " /api/v1/user/ai/prompt-templates/:id":                {},
		http.MethodDelete + " /api/v1/user/ai/prompt-templates/:id":             {},
		http.MethodGet + " /api/v1/user/ai/prompts":                             {},
		http.MethodPost + " /api/v1/user/ai/prompts":                            {},
		http.MethodPut + " /api/v1/user/ai/prompts/:id":                         {},
		http.MethodDelete + " /api/v1/user/ai/prompts/:id":                      {},
		http.MethodPost + " /api/v1/user/ai/prompts/:id/clone":                  {},
		http.MethodGet + " /api/v1/user/ai/generation-jobs":                     {},
		http.MethodPost + " /api/v1/user/ai/generation-jobs":                    {},
		http.MethodGet + " /api/v1/user/ai/generation-jobs/:id":                 {},
		http.MethodPost + " /api/v1/user/ai/chat":                               {},
		http.MethodGet + " /api/v1/user/ai/artworks":                            {},
		http.MethodPost + " /api/v1/user/ai/artworks":                           {},
		http.MethodPost + " /api/v1/user/ai/artworks/edit":                      {},
		http.MethodPut + " /api/v1/user/ai/artworks/:id":                        {},
		http.MethodDelete + " /api/v1/user/ai/artworks/:id":                     {},
		http.MethodGet + " /api/v1/user/ai/gallery":                             {},
		http.MethodGet + " /api/v1/user/ai/assets/:id":                          {},
		http.MethodGet + " /api/v1/user/skills":                                 {},
		http.MethodPost + " /api/v1/user/skills":                                {},
		http.MethodGet + " /api/v1/user/skills/:id":                             {},
		http.MethodPut + " /api/v1/user/skills/:id":                             {},
		http.MethodPost + " /api/v1/user/skills/:id/install":                    {},
		http.MethodPost + " /api/v1/user/skills/:id/uninstall":                  {},
		http.MethodGet + " /api/v1/user/skills/:id/versions":                    {},
		http.MethodPost + " /api/v1/user/skills/:id/versions":                   {},
		http.MethodPut + " /api/v1/user/skills/:id/versions/:versionId":         {},
		http.MethodPost + " /api/v1/user/skills/:id/versions/:versionId/submit": {},
		http.MethodPost + " /api/v1/user/skills/versions/:versionId/publish":    {},
		http.MethodGet + " /api/v1/user/skills/:id/runs":                        {},
		http.MethodPost + " /api/v1/user/skills/:id/runs":                       {},
		http.MethodPost + " /api/v1/user/skills/:id/test":                       {},
		http.MethodPost + " /api/v1/user/skills/:id/use":                        {},
		http.MethodGet + " /api/v1/user/skills/:id/revenue":                     {},
	}

	for _, route := range router.Routes() {
		delete(expected, route.Method+" "+route.Path)
	}
	require.Empty(t, expected)
}
