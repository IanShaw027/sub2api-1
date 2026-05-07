//go:build unit

package skillkit

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestServiceVersionToDomainUsesScriptSourceFromNestedContent(t *testing.T) {
	t.Parallel()

	sourceCode := "print('skill metadata bridge')\n"
	version := &service.AISkillVersion{
		Type: service.AISkillTypeScript,
		ExecutionSpec: service.AISkillExecutionSpec{
			Type: service.AISkillTypeScript,
			Script: &service.AISkillScriptSpec{
				Runtime:    "python3.11",
				ScriptName: "skill_bridge",
				EntryPoint: "main.py",
			},
		},
		Metadata: map[string]any{
			metaKeySkillContent: map[string]any{
				"type":        "script",
				"source_code": sourceCode,
			},
		},
	}

	entity := serviceVersionToDomain(version)
	require.NotNil(t, entity)
	require.Equal(t, sourceCode, entity.SourceContent)
}

func TestDomainVersionToServiceHydratesScriptSourceBackIntoMetadata(t *testing.T) {
	t.Parallel()

	sourceCode := "console.log('rehydrated source')\n"
	version := &domain.AISkillVersion{
		ContentFormat: service.AISkillTypeScript,
		SourceContent: sourceCode,
		Metadata: map[string]any{
			metaKeySkillContent: map[string]any{
				"type": "script",
			},
		},
	}

	entity := domainVersionToService(version)
	require.NotNil(t, entity)
	require.Equal(t, sourceCode, readRawString(entity.Metadata, "source_code"))

	content, ok := entity.Metadata[metaKeySkillContent].(map[string]any)
	require.True(t, ok)
	require.Equal(t, sourceCode, readRawString(content, "source_code"))
}

func TestServiceRepoAdapterCreateReviewPersistsDisabledAuditRow(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	adapter := &serviceRepoAdapter{
		domainRepo: repository.NewAISkillRepository(nil, db),
		db:         db,
	}
	review := &service.AISkillReview{
		SkillID:        11,
		VersionID:      22,
		OperatorUserID: 33,
		Action:         service.AISkillReviewActionDisabled,
		StatusFrom:     service.AISkillVersionStatusApproved,
		StatusTo:       service.AISkillVersionStatusDisabled,
		Comment:        "manual disable",
		Metadata:       map[string]any{"reason": "policy"},
		Trace:          service.AITraceRef{RequestID: "req-disable"},
	}

	mock.ExpectExec(`INSERT INTO ai_skill_reviews`).
		WithArgs(
			int64(11),
			int64(22),
			int64(33),
			int64(33),
			"canceled",
			nil,
			"manual disable",
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			"req-disable",
			nil,
			nil,
			nil,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, adapter.CreateReview(t.Context(), review))
	require.NoError(t, mock.ExpectationsWereMet())
}
