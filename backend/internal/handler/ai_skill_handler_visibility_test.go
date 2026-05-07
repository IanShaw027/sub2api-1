//go:build unit

package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSelectSkillVersionPointersForViewerUsesPublishedVersionForNonOwner(t *testing.T) {
	t.Parallel()

	publishedID := int64(2)
	currentID := int64(1)
	skill := &domain.AISkill{
		UserID:             7,
		CurrentVersionID:   &currentID,
		PublishedVersionID: &publishedID,
	}
	versions := []domain.AISkillVersion{
		{
			ID:           currentID,
			ReviewStatus: domain.AISkillVersionReviewStatusPending,
		},
		{
			ID:           publishedID,
			ReviewStatus: domain.AISkillVersionReviewStatusApproved,
		},
	}

	latest, current := selectSkillVersionPointersForViewer(skill, versions, 9001)
	require.NotNil(t, latest)
	require.NotNil(t, current)
	require.Equal(t, publishedID, latest.ID)
	require.Equal(t, publishedID, current.ID)
}

func TestSelectSkillVersionPointersForViewerKeepsPendingVersionForOwner(t *testing.T) {
	t.Parallel()

	currentID := int64(1)
	skill := &domain.AISkill{
		UserID:           7,
		CurrentVersionID: &currentID,
	}
	versions := []domain.AISkillVersion{
		{
			ID:           currentID,
			ReviewStatus: domain.AISkillVersionReviewStatusPending,
		},
	}

	latest, current := selectSkillVersionPointersForViewer(skill, versions, 7)
	require.NotNil(t, latest)
	require.NotNil(t, current)
	require.Equal(t, currentID, latest.ID)
	require.Equal(t, currentID, current.ID)
}

func TestSkillVersionVisibleToViewerRejectsPendingVersionForNonOwner(t *testing.T) {
	t.Parallel()

	pending := &domain.AISkillVersion{ReviewStatus: domain.AISkillVersionReviewStatusPending}
	approved := &domain.AISkillVersion{ReviewStatus: domain.AISkillVersionReviewStatusApproved}

	require.False(t, skillVersionVisibleToViewer(7, 9001, pending))
	require.True(t, skillVersionVisibleToViewer(7, 9001, approved))
	require.True(t, skillVersionVisibleToViewer(7, 7, pending))
}
