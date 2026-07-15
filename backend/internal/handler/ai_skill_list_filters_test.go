//go:build unit

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/handler/skillkit"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type aiSkillListFilterRepoStub struct {
	service.AISkillDomainRepository

	viewerUserID int64
	isAdmin      bool
	params       pagination.PaginationParams
	filter       domain.AISkillListFilter
}

func (s *aiSkillListFilterRepoStub) ListSkills(_ context.Context, viewerUserID int64, isAdmin bool, params pagination.PaginationParams, filter domain.AISkillListFilter) ([]domain.AISkill, *pagination.PaginationResult, error) {
	s.viewerUserID = viewerUserID
	s.isAdmin = isAdmin
	s.params = params
	s.filter = filter
	return []domain.AISkill{}, &pagination.PaginationResult{
		Total:    0,
		Page:     params.Page,
		PageSize: params.Limit(),
		Pages:    1,
	}, nil
}

func (s *aiSkillListFilterRepoStub) GetSkillInstallStates(_ context.Context, _ int64, _ []int64) (map[int64]bool, error) {
	return map[int64]bool{}, nil
}

func (s *aiSkillListFilterRepoStub) GetSkillInstallCounts(_ context.Context, _ []int64) (map[int64]int, error) {
	return map[int64]int{}, nil
}

func TestAIHandlerListSkillsMapsInstalledCategoryAndStatusFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &aiSkillListFilterRepoStub{}
	handler := &AIHandler{
		skillModule: &skillkit.Module{
			DomainService: service.NewAISkillDomainService(repo),
		},
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/skills?scope=market&category=automation&status=ARCHIVED&installed=not_installed&page=2&page_size=5", nil)
	c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 8801})

	handler.ListSkills(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, int64(8801), repo.viewerUserID)
	require.False(t, repo.isAdmin)
	require.Equal(t, 2, repo.params.Page)
	require.Equal(t, 5, repo.params.PageSize)
	require.Equal(t, "updated_at", repo.params.SortBy)
	require.Equal(t, "desc", repo.params.SortOrder)
	require.Equal(t, domain.AISkillScopeLibrary, repo.filter.Scope)
	require.True(t, repo.filter.PublishedOnly)
	require.Equal(t, "automation", repo.filter.Category)
	require.Equal(t, "archived", repo.filter.Status)
	require.NotNil(t, repo.filter.Installed)
	require.False(t, *repo.filter.Installed)
}

func TestAIHandlerListSkillsMapsPriceModeFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &aiSkillListFilterRepoStub{}
	handler := &AIHandler{
		skillModule: &skillkit.Module{
			DomainService: service.NewAISkillDomainService(repo),
		},
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/skills?scope=market&price_mode=FREE", nil)
	c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 8801})

	handler.ListSkills(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, domain.AISkillPriceModeFree, repo.filter.PriceMode)
	require.Empty(t, repo.filter.BillingMode)
}
