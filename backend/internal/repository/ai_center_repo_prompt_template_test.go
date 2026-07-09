//go:build integration

package repository

import (
	"context"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/suite"
)

// PromptTemplateListSuite 验证 ListPromptTemplates 的 SQL 下推过滤 + DB 分页
type PromptTemplateListSuite struct {
	suite.Suite
	ctx  context.Context
	tx   *dbent.Tx
	repo *aiCenterRepository
}

func (s *PromptTemplateListSuite) SetupTest() {
	s.ctx = context.Background()
	tx := testEntTx(s.T())
	s.tx = tx
	s.repo = &aiCenterRepository{client: tx.Client()}
}

func TestPromptTemplateListSuite(t *testing.T) {
	suite.Run(t, new(PromptTemplateListSuite))
}

// createTemplate 插入一条模板并返回其 ID
func (s *PromptTemplateListSuite) createTemplate(tmpl *service.AIPromptTemplate) int64 {
	s.T().Helper()
	err := s.repo.CreatePromptTemplate(s.ctx, tmpl, nil)
	s.Require().NoError(err, "CreatePromptTemplate")
	s.Require().NotZero(tmpl.ID)
	return tmpl.ID
}

// listMine 以 mine scope + OwnerUserID 隔离查询，避免跨测试污染
func (s *PromptTemplateListSuite) listMine(userID int64, params pagination.PaginationParams, filter service.AIListPromptTemplatesFilter) ([]service.AIPromptTemplate, *pagination.PaginationResult) {
	s.T().Helper()
	filter.Scope = "mine"
	filter.OwnerUserID = &userID
	items, result, err := s.repo.ListPromptTemplates(s.ctx, userID, false, params, filter)
	s.Require().NoError(err, "ListPromptTemplates")
	return items, result
}

// TestStatusPushdown 校验派生 status 过滤各态与 metadata.status 覆盖
func (s *PromptTemplateListSuite) TestStatusPushdown() {
	const userID int64 = 700001
	publishedID := s.createTemplate(&service.AIPromptTemplate{
		UserID: userID, Title: "published one", Content: "c", CurrentVersion: 1,
		Visibility: domain.AIVisibilityPublic, ModerationState: domain.AIModerationStateNormal,
	})
	draftID := s.createTemplate(&service.AIPromptTemplate{
		UserID: userID, Title: "draft one", Content: "c", CurrentVersion: 1,
		Visibility: domain.AIVisibilityPrivate, ModerationState: domain.AIModerationStateNormal,
	})
	hiddenID := s.createTemplate(&service.AIPromptTemplate{
		UserID: userID, Title: "hidden one", Content: "c", CurrentVersion: 1,
		Visibility: domain.AIVisibilityPublic, ModerationState: domain.AIModerationStateBlocked,
	})
	archivedID := s.createTemplate(&service.AIPromptTemplate{
		UserID: userID, Title: "archived one", Content: "c", CurrentVersion: 1,
		Visibility: domain.AIVisibilityPrivate, ModerationState: domain.AIModerationStateForcedPrivate,
	})
	// metadata.status 覆盖：派生本为 published，但 metadata 强制 archived
	overrideID := s.createTemplate(&service.AIPromptTemplate{
		UserID: userID, Title: "override one", Content: "c", CurrentVersion: 1,
		Visibility: domain.AIVisibilityPublic, ModerationState: domain.AIModerationStateNormal,
		Metadata: map[string]any{"status": "archived"},
	})

	cases := []struct {
		status  string
		wantIDs []int64
	}{
		{"published", []int64{publishedID}},
		{"draft", []int64{draftID}},
		{"hidden", []int64{hiddenID}},
		{"archived", []int64{archivedID, overrideID}},
	}
	for _, tc := range cases {
		items, result := s.listMine(userID, pagination.PaginationParams{Page: 1, PageSize: 50}, service.AIListPromptTemplatesFilter{Status: tc.status})
		s.Require().Equal(int64(len(tc.wantIDs)), result.Total, "total for status=%s", tc.status)
		s.Require().ElementsMatch(tc.wantIDs, idsOf(items), "ids for status=%s", tc.status)
	}

	// status=all 不过滤
	_, allResult := s.listMine(userID, pagination.PaginationParams{Page: 1, PageSize: 50}, service.AIListPromptTemplatesFilter{Status: "all"})
	s.Require().Equal(int64(5), allResult.Total, "status=all returns everything")
}

// TestSearchPushdown 校验 title/description/category/content/tag 各命中一条 + 大小写不敏感
func (s *PromptTemplateListSuite) TestSearchPushdown() {
	const userID int64 = 700002
	byTitle := s.createTemplate(&service.AIPromptTemplate{
		UserID: userID, Title: "zeta headline", Content: "x", CurrentVersion: 1,
		Visibility: domain.AIVisibilityPrivate, ModerationState: domain.AIModerationStateNormal,
	})
	byDesc := s.createTemplate(&service.AIPromptTemplate{
		UserID: userID, Title: "d", Description: "contains zeta here", Content: "x", CurrentVersion: 1,
		Visibility: domain.AIVisibilityPrivate, ModerationState: domain.AIModerationStateNormal,
	})
	byCategory := s.createTemplate(&service.AIPromptTemplate{
		UserID: userID, Title: "c", Category: "zeta-cat", Content: "x", CurrentVersion: 1,
		Visibility: domain.AIVisibilityPrivate, ModerationState: domain.AIModerationStateNormal,
	})
	byContent := s.createTemplate(&service.AIPromptTemplate{
		UserID: userID, Title: "b", Content: "body mentions zeta inline", CurrentVersion: 1,
		Visibility: domain.AIVisibilityPrivate, ModerationState: domain.AIModerationStateNormal,
	})
	byTag := s.createTemplate(&service.AIPromptTemplate{
		UserID: userID, Title: "t", Content: "x", Tags: []string{"other", "ZeTa-tag"}, CurrentVersion: 1,
		Visibility: domain.AIVisibilityPrivate, ModerationState: domain.AIModerationStateNormal,
	})
	// 干扰项：不含关键字
	s.createTemplate(&service.AIPromptTemplate{
		UserID: userID, Title: "nomatch", Content: "nothing", CurrentVersion: 1,
		Visibility: domain.AIVisibilityPrivate, ModerationState: domain.AIModerationStateNormal,
	})

	// 用大写输入校验大小写不敏感
	items, result := s.listMine(userID, pagination.PaginationParams{Page: 1, PageSize: 50}, service.AIListPromptTemplatesFilter{Search: "ZETA"})
	s.Require().Equal(int64(5), result.Total)
	s.Require().ElementsMatch([]int64{byTitle, byDesc, byCategory, byContent, byTag}, idsOf(items))
}

// TestPaginationPushdown 校验 DB 层 Offset/Limit 切片正确且 total 为全量
func (s *PromptTemplateListSuite) TestPaginationPushdown() {
	const userID int64 = 700003
	allIDs := make([]int64, 0, 5)
	for i := 0; i < 5; i++ {
		id := s.createTemplate(&service.AIPromptTemplate{
			UserID: userID, Title: "page item", Content: "x", CurrentVersion: 1,
			Visibility: domain.AIVisibilityPrivate, ModerationState: domain.AIModerationStateNormal,
		})
		allIDs = append(allIDs, id)
	}

	seen := make([]int64, 0, 5)
	for _, page := range []struct {
		page      int
		wantCount int
	}{{1, 2}, {2, 2}, {3, 1}} {
		items, result := s.listMine(userID, pagination.PaginationParams{Page: page.page, PageSize: 2}, service.AIListPromptTemplatesFilter{})
		s.Require().Equal(int64(5), result.Total, "total on page %d", page.page)
		s.Require().Len(items, page.wantCount, "count on page %d", page.page)
		seen = append(seen, idsOf(items)...)
	}
	// 三页并集应无重叠地覆盖全部
	s.Require().ElementsMatch(allIDs, seen)
}

// TestLibraryScope 校验 library scope 只返回 public + normal
func (s *PromptTemplateListSuite) TestLibraryScope() {
	const userID int64 = 700004
	publicNormal := s.createTemplate(&service.AIPromptTemplate{
		UserID: userID, Title: "lib visible", Content: "x", CurrentVersion: 1,
		Visibility: domain.AIVisibilityPublic, ModerationState: domain.AIModerationStateNormal,
	})
	// private+normal 与 public+blocked 都不应出现在 library
	s.createTemplate(&service.AIPromptTemplate{
		UserID: userID, Title: "lib private", Content: "x", CurrentVersion: 1,
		Visibility: domain.AIVisibilityPrivate, ModerationState: domain.AIModerationStateNormal,
	})
	s.createTemplate(&service.AIPromptTemplate{
		UserID: userID, Title: "lib blocked", Content: "x", CurrentVersion: 1,
		Visibility: domain.AIVisibilityPublic, ModerationState: domain.AIModerationStateBlocked,
	})

	items, result, err := s.repo.ListPromptTemplates(s.ctx, userID, false, pagination.PaginationParams{Page: 1, PageSize: 50}, service.AIListPromptTemplatesFilter{Scope: "library"})
	s.Require().NoError(err)
	s.Require().Equal(int64(1), result.Total)
	s.Require().ElementsMatch([]int64{publicNormal}, idsOf(items))
}

func idsOf(items []service.AIPromptTemplate) []int64 {
	out := make([]int64, 0, len(items))
	for i := range items {
		out = append(out, items[i].ID)
	}
	return out
}
