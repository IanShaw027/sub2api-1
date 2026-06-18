//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAISkillServiceRepositoryAdapterPreservesApprovedArtifactDigest(t *testing.T) {
	db, mock := newSQLMock(t)
	adapter := &aiSkillServiceRepositoryAdapter{db: db}

	serviceVersion := &service.AISkillVersion{
		ID:                     42,
		SkillID:                7,
		CreatorUserID:          11,
		Version:                3,
		Type:                   service.AISkillTypeScript,
		Status:                 service.AISkillVersionStatusApproved,
		ApprovedArtifactDigest: " sha256:abc123 ",
		ExecutionSpec: service.AISkillExecutionSpec{
			Type: service.AISkillTypeScript,
			Script: &service.AISkillScriptSpec{
				Runtime:    "node20",
				ScriptName: "echo_script",
				EntryPoint: "main.mjs",
				Arguments:  []map[string]any{{"name": "input"}},
			},
		},
		BillingPolicy: service.AISkillBillingPolicy{Mode: service.AISkillBillingModeFree},
		ChangeNote:    "approved snapshot",
		Metadata: map[string]any{
			"source_code": "console.log('hello')",
		},
	}

	domainVersion := serviceVersionToDomain(serviceVersion)
	require.Equal(t, "sha256:abc123", domainVersion.ApprovedArtifactDigest)

	roundTripped := domainVersionToService(domainVersion)
	require.Equal(t, "sha256:abc123", roundTripped.ApprovedArtifactDigest)

	now := time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)UPDATE ai_skill_versions.*approved_artifact_digest = \$14`).
		WithArgs(
			domainVersion.ID,
			nullableString(domainVersion.ContentFormat),
			nullableString(domainVersion.Runtime),
			domainVersion.SourceContent,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			nullableString(domainVersion.ChangeNote),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			"sha256:abc123",
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"skill_id", "user_id", "version", "review_status", "submitted_at",
			"reviewed_at", "reviewer_user_id", "review_note", "created_at", "updated_at",
		}).AddRow(
			domainVersion.SkillID,
			domainVersion.UserID,
			domainVersion.Version,
			domain.AISkillVersionReviewStatusApproved,
			nil,
			nil,
			nil,
			"approved",
			now,
			now,
		))

	require.NoError(t, adapter.updateVersionSnapshot(context.Background(), domainVersion))
	require.Equal(t, "sha256:abc123", domainVersion.ApprovedArtifactDigest)
	require.NoError(t, mock.ExpectationsWereMet())
}
