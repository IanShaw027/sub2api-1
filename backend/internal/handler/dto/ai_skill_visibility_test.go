package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSkillDetailFromDomainHidesSourceAndRawMetadataWithoutSourceAccess(t *testing.T) {
	currentVersionID := int64(11)
	skill := &domain.AISkill{
		ID:                 7,
		UserID:             7001,
		Type:               domain.AISkillTypeScript,
		Title:              "Paid Skill",
		Description:        "private implementation",
		Visibility:         domain.AIVisibilityPublic,
		SourceVisibility:   domain.AISkillSourceVisibilityHidden,
		BillingMode:        domain.AISkillBillingModePerRequest,
		Price:              19.9,
		CurrentVersionID:   &currentVersionID,
		PublishedVersionID: &currentVersionID,
		LatestVersion:      1,
		Metadata: map[string]any{
			"slug":         "paid-skill",
			"status":       "published",
			"readme":       "safe readme",
			"install_note": "safe install note",
			"examples":     []any{"example prompt"},
			"variable_schema": []any{
				map[string]any{"key": "topic", "type": "string"},
			},
			"content": map[string]any{
				"type":        "script",
				"source_code": "print('secret source')",
			},
		},
	}
	version := &domain.AISkillVersion{
		ID:           currentVersionID,
		SkillID:      skill.ID,
		UserID:       skill.UserID,
		Version:      1,
		ReviewStatus: domain.AISkillVersionReviewStatusApproved,
		Metadata: map[string]any{
			"content": map[string]any{
				"type":        "script",
				"source_code": "print('version secret')",
			},
		},
	}

	detail := SkillDetailFromDomain(skill, 9001, false, version, version)
	require.NotNil(t, detail)
	require.False(t, detail.CanViewSource)
	require.Nil(t, detail.Content)
	require.Len(t, detail.VariableSchema, 1)
	require.Equal(t, "safe readme", detail.Readme)
	require.Equal(t, "safe install note", detail.InstallNote)
	require.Empty(t, detail.Metadata)
	require.Empty(t, detail.Examples)
}

func TestSkillVersionRecordFromDomainHidesSourceAndMetadataWithoutSourceAccess(t *testing.T) {
	currentVersionID := int64(11)
	version := &domain.AISkillVersion{
		ID:           currentVersionID,
		SkillID:      7,
		UserID:       7001,
		Version:      1,
		ReviewStatus: domain.AISkillVersionReviewStatusApproved,
		Metadata: map[string]any{
			"variable_schema": []any{
				map[string]any{"key": "topic", "type": "string"},
			},
			"content": map[string]any{
				"type":        "script",
				"source_code": "print('secret source')",
			},
		},
	}

	record := SkillVersionRecordFromDomain(version, &currentVersionID, false)
	require.NotNil(t, record)
	require.Nil(t, record.Content)
	require.Empty(t, record.VariableSchema)
	require.Empty(t, record.Metadata)
}
