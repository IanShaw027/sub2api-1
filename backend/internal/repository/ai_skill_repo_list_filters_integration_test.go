//go:build integration

package repository

import (
	"context"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAISkillRepositoryListSkillsFiltersByCategory(t *testing.T) {
	ctx, repo := newAISkillRepoFilterTestContext(t)
	owner := mustCreateUser(t, repo.client, &service.User{Email: "ai-skill-filter-category@example.com"})

	first := createAISkillRepoFilterTestSkill(t, ctx, repo, &domain.AISkill{
		UserID:           owner.ID,
		Type:             domain.AISkillTypeScript,
		Title:            "Automation One",
		Category:         "automation",
		Visibility:       domain.AIVisibilityPrivate,
		SourceVisibility: domain.AISkillSourceVisibilityPublic,
		BillingMode:      domain.AISkillBillingModePerRequest,
	})
	createAISkillRepoFilterTestSkill(t, ctx, repo, &domain.AISkill{
		UserID:           owner.ID,
		Type:             domain.AISkillTypeScript,
		Title:            "Writing One",
		Category:         "writing",
		Visibility:       domain.AIVisibilityPrivate,
		SourceVisibility: domain.AISkillSourceVisibilityPublic,
		BillingMode:      domain.AISkillBillingModePerRequest,
	})
	second := createAISkillRepoFilterTestSkill(t, ctx, repo, &domain.AISkill{
		UserID:           owner.ID,
		Type:             domain.AISkillTypePromptChat,
		Title:            "Automation Two",
		Category:         "automation",
		Visibility:       domain.AIVisibilityPrivate,
		SourceVisibility: domain.AISkillSourceVisibilityPublic,
		BillingMode:      domain.AISkillBillingModePerRequest,
	})

	items, result, err := repo.ListSkills(ctx, owner.ID, false, pagination.PaginationParams{Page: 1, PageSize: 20}, domain.AISkillListFilter{
		Scope:    domain.AISkillScopeMine,
		Category: "automation",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(2), result.Total)
	require.ElementsMatch(t, []int64{first.ID, second.ID}, aiSkillRepoFilterTestIDs(items))
}

func TestAISkillRepositoryListSkillsFiltersByStatus(t *testing.T) {
	ctx, repo := newAISkillRepoFilterTestContext(t)
	owner := mustCreateUser(t, repo.client, &service.User{Email: "ai-skill-filter-status@example.com"})

	draft := createAISkillRepoFilterTestSkill(t, ctx, repo, &domain.AISkill{
		UserID:           owner.ID,
		Type:             domain.AISkillTypeScript,
		Title:            "Draft Skill",
		Category:         "automation",
		Visibility:       domain.AIVisibilityPrivate,
		SourceVisibility: domain.AISkillSourceVisibilityPublic,
		BillingMode:      domain.AISkillBillingModePerRequest,
	})
	archived := createAISkillRepoFilterTestSkill(t, ctx, repo, &domain.AISkill{
		UserID:           owner.ID,
		Type:             domain.AISkillTypeScript,
		Title:            "Archived Skill",
		Category:         "automation",
		Visibility:       domain.AIVisibilityPrivate,
		SourceVisibility: domain.AISkillSourceVisibilityPublic,
		BillingMode:      domain.AISkillBillingModePerRequest,
		Metadata: map[string]any{
			"status": "archived",
		},
	})
	published := createAISkillRepoFilterTestSkill(t, ctx, repo, &domain.AISkill{
		UserID:           owner.ID,
		Type:             domain.AISkillTypePromptChat,
		Title:            "Published Skill",
		Category:         "automation",
		Visibility:       domain.AIVisibilityPublic,
		SourceVisibility: domain.AISkillSourceVisibilityPublic,
		BillingMode:      domain.AISkillBillingModePerRequest,
	})

	draftItems, _, err := repo.ListSkills(ctx, owner.ID, false, pagination.PaginationParams{Page: 1, PageSize: 20}, domain.AISkillListFilter{
		Scope:  domain.AISkillScopeMine,
		Status: "draft",
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{draft.ID}, aiSkillRepoFilterTestIDs(draftItems))

	archivedItems, _, err := repo.ListSkills(ctx, owner.ID, false, pagination.PaginationParams{Page: 1, PageSize: 20}, domain.AISkillListFilter{
		Scope:  domain.AISkillScopeMine,
		Status: "archived",
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{archived.ID}, aiSkillRepoFilterTestIDs(archivedItems))

	publishedItems, _, err := repo.ListSkills(ctx, owner.ID, false, pagination.PaginationParams{Page: 1, PageSize: 20}, domain.AISkillListFilter{
		Scope:  domain.AISkillScopeMine,
		Status: "published",
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{published.ID}, aiSkillRepoFilterTestIDs(publishedItems))
}

func TestAISkillRepositoryListSkillsFiltersByInstalled(t *testing.T) {
	ctx, repo := newAISkillRepoFilterTestContext(t)
	installedOwner := mustCreateUser(t, repo.client, &service.User{Email: "ai-skill-filter-installed-owner@example.com"})
	notInstalledOwner := mustCreateUser(t, repo.client, &service.User{Email: "ai-skill-filter-not-installed-owner@example.com"})
	viewer := mustCreateUser(t, repo.client, &service.User{Email: "ai-skill-filter-viewer@example.com"})

	installedSkill := createAISkillRepoFilterPublishedSkill(t, ctx, repo, installedOwner.ID, "Installed Skill", "automation")
	notInstalledSkill := createAISkillRepoFilterPublishedSkill(t, ctx, repo, notInstalledOwner.ID, "Not Installed Skill", "automation")
	require.NoError(t, repo.SetSkillInstall(ctx, installedSkill.ID, viewer.ID, true))

	installed := true
	installedItems, result, err := repo.ListSkills(ctx, viewer.ID, false, pagination.PaginationParams{Page: 1, PageSize: 20}, domain.AISkillListFilter{
		Scope:     domain.AISkillScopeLibrary,
		Installed: &installed,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(1), result.Total)
	require.ElementsMatch(t, []int64{installedSkill.ID}, aiSkillRepoFilterTestIDs(installedItems))

	notInstalled := false
	notInstalledItems, _, err := repo.ListSkills(ctx, viewer.ID, false, pagination.PaginationParams{Page: 1, PageSize: 20}, domain.AISkillListFilter{
		Scope:     domain.AISkillScopeLibrary,
		Installed: &notInstalled,
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{notInstalledSkill.ID}, aiSkillRepoFilterTestIDs(notInstalledItems))
}

func TestAISkillRepositoryListSkillsFiltersByPriceMode(t *testing.T) {
	ctx, repo := newAISkillRepoFilterTestContext(t)
	freeOwner := mustCreateUser(t, repo.client, &service.User{Email: "ai-skill-filter-free-owner@example.com"})
	paidOwner := mustCreateUser(t, repo.client, &service.User{Email: "ai-skill-filter-paid-owner@example.com"})
	viewer := mustCreateUser(t, repo.client, &service.User{Email: "ai-skill-filter-price-viewer@example.com"})

	freeSkill := createAISkillRepoFilterPublishedSkillWithPrice(t, ctx, repo, freeOwner.ID, "Free Skill", "automation", 0)
	paidSkill := createAISkillRepoFilterPublishedSkillWithPrice(t, ctx, repo, paidOwner.ID, "Paid Skill", "automation", 19.9)

	paidItems, result, err := repo.ListSkills(ctx, viewer.ID, false, pagination.PaginationParams{Page: 1, PageSize: 20}, domain.AISkillListFilter{
		Scope:     domain.AISkillScopeLibrary,
		PriceMode: domain.AISkillPriceModePaid,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(1), result.Total)
	require.ElementsMatch(t, []int64{paidSkill.ID}, aiSkillRepoFilterTestIDs(paidItems))

	freeItems, _, err := repo.ListSkills(ctx, viewer.ID, false, pagination.PaginationParams{Page: 1, PageSize: 20}, domain.AISkillListFilter{
		Scope:     domain.AISkillScopeLibrary,
		PriceMode: domain.AISkillPriceModeFree,
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{freeSkill.ID}, aiSkillRepoFilterTestIDs(freeItems))
}

func newAISkillRepoFilterTestContext(t *testing.T) (context.Context, *aiSkillRepository) {
	t.Helper()

	tx := testEntTx(t)
	ctx := dbent.NewTxContext(context.Background(), tx)
	return ctx, &aiSkillRepository{client: tx.Client()}
}

func createAISkillRepoFilterTestSkill(t *testing.T, ctx context.Context, repo *aiSkillRepository, skill *domain.AISkill) *domain.AISkill {
	t.Helper()

	require.NoError(t, repo.CreateSkill(ctx, skill))
	return skill
}

func createAISkillRepoFilterPublishedSkill(t *testing.T, ctx context.Context, repo *aiSkillRepository, ownerUserID int64, title, category string) *domain.AISkill {
	t.Helper()

	skill := createAISkillRepoFilterTestSkill(t, ctx, repo, &domain.AISkill{
		UserID:           ownerUserID,
		Type:             domain.AISkillTypeScript,
		Title:            title,
		Category:         category,
		Visibility:       domain.AIVisibilityPublic,
		SourceVisibility: domain.AISkillSourceVisibilityPublic,
		BillingMode:      domain.AISkillBillingModePerRequest,
		Metadata: map[string]any{
			"pricing": map[string]any{
				"mode":   domain.AISkillPriceModeFree,
				"amount": 0,
			},
			"price_mode":   domain.AISkillPriceModeFree,
			"price_amount": 0,
		},
	})
	version := &domain.AISkillVersion{
		SkillID:       skill.ID,
		UserID:        ownerUserID,
		Version:       1,
		ReviewStatus:  domain.AISkillVersionReviewStatusApproved,
		Runtime:       "python3.11",
		SourceContent: "print('ok')",
	}
	require.NoError(t, repo.CreateSkillVersion(ctx, version))

	currentVersionID := version.ID
	skill.CurrentVersionID = &currentVersionID
	skill.PublishedVersionID = &currentVersionID
	skill.LatestApprovedVersionID = &currentVersionID
	skill.LatestVersion = 1
	require.NoError(t, repo.UpdateSkill(ctx, skill))
	return skill
}

func createAISkillRepoFilterPublishedSkillWithPrice(t *testing.T, ctx context.Context, repo *aiSkillRepository, ownerUserID int64, title, category string, price float64) *domain.AISkill {
	t.Helper()

	priceMode := domain.AISkillPriceModeFree
	if price > 0 {
		priceMode = domain.AISkillPriceModePaid
	}
	skill := createAISkillRepoFilterTestSkill(t, ctx, repo, &domain.AISkill{
		UserID:           ownerUserID,
		Type:             domain.AISkillTypeScript,
		Title:            title,
		Category:         category,
		Visibility:       domain.AIVisibilityPublic,
		SourceVisibility: domain.AISkillSourceVisibilityPublic,
		BillingMode:      domain.AISkillBillingModePerRequest,
		Price:            price,
		Metadata: map[string]any{
			"pricing": map[string]any{
				"mode":   priceMode,
				"amount": price,
			},
			"price_mode":   priceMode,
			"price_amount": price,
		},
	})
	version := &domain.AISkillVersion{
		SkillID:       skill.ID,
		UserID:        ownerUserID,
		Version:       1,
		ReviewStatus:  domain.AISkillVersionReviewStatusApproved,
		Runtime:       "python3.11",
		SourceContent: "print('ok')",
	}
	require.NoError(t, repo.CreateSkillVersion(ctx, version))

	currentVersionID := version.ID
	skill.CurrentVersionID = &currentVersionID
	skill.PublishedVersionID = &currentVersionID
	skill.LatestApprovedVersionID = &currentVersionID
	skill.LatestVersion = 1
	require.NoError(t, repo.UpdateSkill(ctx, skill))
	return skill
}

func aiSkillRepoFilterTestIDs(items []domain.AISkill) []int64 {
	ids := make([]int64, 0, len(items))
	for i := range items {
		ids = append(ids, items[i].ID)
	}
	return ids
}
