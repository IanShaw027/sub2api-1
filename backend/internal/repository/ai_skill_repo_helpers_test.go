package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

func TestAISkillPrepareSkillRejectsInvalidSourceVisibility(t *testing.T) {
	t.Parallel()

	skill := &domain.AISkill{
		Type:             domain.AISkillTypePromptChat,
		Title:            "Unsafe source visibility",
		Visibility:       domain.AIVisibilityPublic,
		SourceVisibility: "friends_only",
		BillingMode:      domain.AISkillBillingModePerRequest,
		Price:            0,
	}

	err := aiSkillPrepareSkill(skill)
	if err != domain.ErrAISkillSourceVisibilityInvalid {
		t.Fatalf("expected ErrAISkillSourceVisibilityInvalid, got %v", err)
	}
}
