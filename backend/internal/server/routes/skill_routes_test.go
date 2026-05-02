package routes

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterUserRoutesIncludesSkillEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	v1 := router.Group("/api/v1")

	RegisterUserRoutes(
		v1,
		&handler.Handlers{
			AI: &handler.AIHandler{},
		},
		servermiddleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() }),
		nil,
	)

	expected := map[string]struct{}{
		http.MethodGet + " /api/v1/user/skills":                                {},
		http.MethodPost + " /api/v1/user/skills":                               {},
		http.MethodGet + " /api/v1/user/skills/:id":                            {},
		http.MethodPut + " /api/v1/user/skills/:id":                            {},
		http.MethodPost + " /api/v1/user/skills/:id/install":                   {},
		http.MethodPost + " /api/v1/user/skills/:id/uninstall":                 {},
		http.MethodGet + " /api/v1/user/skills/:id/versions":                   {},
		http.MethodPost + " /api/v1/user/skills/:id/versions":                  {},
		http.MethodPut + " /api/v1/user/skills/:id/versions/:versionId":        {},
		http.MethodPost + " /api/v1/user/skills/:id/versions/:versionId/submit": {},
		http.MethodPost + " /api/v1/user/skills/versions/:versionId/publish":   {},
		http.MethodGet + " /api/v1/user/skills/:id/runs":                       {},
		http.MethodPost + " /api/v1/user/skills/:id/runs":                      {},
		http.MethodPost + " /api/v1/user/skills/:id/test":                      {},
		http.MethodPost + " /api/v1/user/skills/:id/use":                       {},
		http.MethodGet + " /api/v1/user/skills/:id/revenue":                    {},
	}

	for _, route := range router.Routes() {
		delete(expected, route.Method+" "+route.Path)
	}
	require.Empty(t, expected)
}

func TestRegisterAdminRoutesIncludesSkillGovernanceEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	v1 := router.Group("/api/v1")

	RegisterAdminRoutes(
		v1,
		&handler.Handlers{
			Admin: &handler.AdminHandlers{
				AI: &adminhandler.AIHandler{},
			},
		},
		servermiddleware.AdminAuthMiddleware(func(c *gin.Context) { c.Next() }),
		nil,
	)

	expected := map[string]struct{}{
		http.MethodGet + " /api/v1/admin/skills/reviews":              {},
		http.MethodPost + " /api/v1/admin/skills/reviews/:id/approve": {},
		http.MethodPost + " /api/v1/admin/skills/reviews/:id/reject":  {},
		http.MethodGet + " /api/v1/admin/skills/governance":           {},
		http.MethodPost + " /api/v1/admin/skills/:id/disable":         {},
		http.MethodPost + " /api/v1/admin/skills/:id/force-private":   {},
		http.MethodGet + " /api/v1/admin/skills/runtime":              {},
		http.MethodGet + " /api/v1/admin/skills/settlements":          {},
	}

	for _, route := range router.Routes() {
		delete(expected, route.Method+" "+route.Path)
	}
	require.Empty(t, expected)
}
