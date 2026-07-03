//go:build unit

package skillkit

import (
	"context"
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

func TestServiceVersionToDomainMapsSubmittedToPendingReview(t *testing.T) {
	t.Parallel()

	version := &service.AISkillVersion{
		Status: service.AISkillVersionStatusSubmitted,
	}

	entity := serviceVersionToDomain(version)
	require.NotNil(t, entity)
	require.Equal(t, domain.AISkillVersionReviewStatusPending, entity.ReviewStatus)
}

func TestServiceVersionToDomainNormalizesBillingPolicyModeToPerRequest(t *testing.T) {
	t.Parallel()

	version := &service.AISkillVersion{
		BillingPolicy: service.AISkillBillingPolicy{
			Mode:                   service.AISkillBillingModePerRun,
			PricePerRun:            12.5,
			PlatformCommissionRate: 0.15,
			Currency:               "credit",
		},
	}

	entity := serviceVersionToDomain(version)
	require.NotNil(t, entity)

	policy := mustMap(entity.Metadata[metaKeySkillBillingPolicy])
	require.Equal(t, domain.AISkillBillingModePerRequest, policy["mode"])
}

func TestDomainVersionToServiceNormalizesBillingPolicyModeToPerRun(t *testing.T) {
	t.Parallel()

	version := &domain.AISkillVersion{
		Metadata: map[string]any{
			metaKeySkillBillingPolicy: map[string]any{
				"mode":                     domain.AISkillBillingModePerRequest,
				"price_per_run":            9.9,
				"platform_commission_rate": 0.2,
				"currency":                 "credit",
			},
		},
	}

	entity := domainVersionToService(version)
	require.NotNil(t, entity)
	require.Equal(t, service.AISkillBillingModePerRun, entity.BillingPolicy.Mode)
}

func TestRunAndSettlementBillingModesTranslateAcrossAdapterBoundary(t *testing.T) {
	t.Parallel()

	runEntity := serviceRunToDomain(&service.AISkillRun{
		BillingMode: service.AISkillBillingModePerRun,
	})
	require.NotNil(t, runEntity)
	require.Equal(t, domain.AISkillBillingModePerRequest, runEntity.BillingMode)

	runDTO := domainRunToService(&domain.AISkillRun{
		BillingMode: domain.AISkillBillingModePerRequest,
	})
	require.NotNil(t, runDTO)
	require.Equal(t, service.AISkillBillingModePerRun, runDTO.BillingMode)

	settlementEntity := serviceSettlementToDomain(&service.AISkillSettlement{
		BillingMode: service.AISkillBillingModePerRun,
	})
	require.NotNil(t, settlementEntity)
	require.Equal(t, domain.AISkillBillingModePerRequest, settlementEntity.BillingMode)

	settlementDTO := domainSettlementToService(&domain.AISkillSettlement{
		BillingMode: domain.AISkillBillingModePerRequest,
	})
	require.NotNil(t, settlementDTO)
	require.Equal(t, service.AISkillBillingModePerRun, settlementDTO.BillingMode)
}

type balanceChargerRepoStub struct {
	balance            float64
	refundCalls        []float64
	updateBalanceCalls []float64
}

func (s *balanceChargerRepoStub) DeductBalance(_ context.Context, _ int64, amount float64) error {
	s.balance -= amount
	return nil
}

func (s *balanceChargerRepoStub) AddBalanceWithoutRecharge(_ context.Context, _ int64, amount float64) error {
	s.refundCalls = append(s.refundCalls, amount)
	s.balance += amount
	return nil
}

func (s *balanceChargerRepoStub) UpdateBalance(_ context.Context, _ int64, amount float64) error {
	s.updateBalanceCalls = append(s.updateBalanceCalls, amount)
	return nil
}

func (s *balanceChargerRepoStub) GetByID(_ context.Context, id int64) (*service.User, error) {
	return &service.User{ID: id, Balance: s.balance}, nil
}

func TestBalanceChargerAdapterRefundDoesNotUseRechargeBalancePath(t *testing.T) {
	t.Parallel()

	repo := &balanceChargerRepoStub{balance: 20}
	adapter := &balanceChargerAdapter{userRepo: repo}

	result, err := adapter.RefundUserBalance(t.Context(), service.AISkillBalanceRefundInput{
		UserID: 9,
		Amount: 4.25,
	})

	require.NoError(t, err)
	require.Equal(t, []float64{4.25}, repo.refundCalls)
	require.Empty(t, repo.updateBalanceCalls)
	require.InDelta(t, 4.25, result.RefundedAmount, 1e-9)
	require.InDelta(t, 24.25, result.BalanceAfter, 1e-9)
}
