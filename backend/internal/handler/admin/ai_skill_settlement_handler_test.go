//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/handler/skillkit"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type adminAISkillSettlementDomainRepoStub struct {
	repository.AISkillRepository
	settlement *domain.AISkillSettlement
}

func (s *adminAISkillSettlementDomainRepoStub) GetSkillSettlementByID(_ context.Context, id int64) (*domain.AISkillSettlement, error) {
	if s.settlement != nil && s.settlement.ID == id {
		copy := *s.settlement
		copy.Metadata = cloneAdminAIMap(s.settlement.Metadata)
		return &copy, nil
	}
	return nil, service.ErrAISkillSettlementNotFound
}

type adminAISkillSettlementRepoStub struct {
	settlement *service.AISkillSettlement
}

func (s *adminAISkillSettlementRepoStub) CreateSettlement(context.Context, *service.AISkillSettlement) error {
	return nil
}

func (s *adminAISkillSettlementRepoStub) GetSettlementByRunID(_ context.Context, runID int64) (*service.AISkillSettlement, error) {
	if s.settlement != nil && s.settlement.RunID == runID {
		copy := *s.settlement
		copy.Metadata = cloneAdminAIMap(s.settlement.Metadata)
		if s.settlement.BalanceAfter != nil {
			value := *s.settlement.BalanceAfter
			copy.BalanceAfter = &value
		}
		return &copy, nil
	}
	return nil, service.ErrAISkillSettlementNotFound
}

func (s *adminAISkillSettlementRepoStub) UpdateSettlement(_ context.Context, settlement *service.AISkillSettlement) error {
	copy := *settlement
	copy.Metadata = cloneAdminAIMap(settlement.Metadata)
	if settlement.BalanceAfter != nil {
		value := *settlement.BalanceAfter
		copy.BalanceAfter = &value
	}
	s.settlement = &copy
	return nil
}

type adminAISkillSettlementBalanceStub struct {
	charges int
}

func (s *adminAISkillSettlementBalanceStub) ChargeUserBalance(_ context.Context, input service.AISkillBalanceChargeInput) (*service.AISkillBalanceChargeResult, error) {
	s.charges++
	return &service.AISkillBalanceChargeResult{ChargedAmount: input.Amount, BalanceAfter: 87.5}, nil
}

func (s *adminAISkillSettlementBalanceStub) RefundUserBalance(_ context.Context, input service.AISkillBalanceRefundInput) (*service.AISkillBalanceRefundResult, error) {
	return &service.AISkillBalanceRefundResult{RefundedAmount: input.Amount, BalanceAfter: 100}, nil
}

type adminAISkillSettlementCreatorStub struct {
	credits int
}

func (s *adminAISkillSettlementCreatorStub) CreditCreatorEarnings(_ context.Context, input service.AISkillCreatorEarningsInput) (float64, error) {
	s.credits++
	return input.Amount, nil
}

func (s *adminAISkillSettlementCreatorStub) ReverseCreatorEarnings(_ context.Context, input service.AISkillCreatorEarningsReversalInput) (float64, error) {
	return input.Amount, nil
}

func newAdminAISkillSettlementContext(t *testing.T, path string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 9001})
	c.Params = gin.Params{{Key: "id", Value: "55"}}
	return c, rec
}

func TestReplaySkillSettlement_ReplaysSafeFailedSettlement(t *testing.T) {
	repo := &adminAISkillSettlementRepoStub{
		settlement: &service.AISkillSettlement{
			ID:                     55,
			RunID:                  3007,
			SkillID:                1007,
			VersionID:              2007,
			BuyerUserID:            906,
			CreatorUserID:          706,
			BillingMode:            service.AISkillBillingModePerRun,
			Currency:               "credit",
			TotalAmount:            12.5,
			PlatformAmount:         2.5,
			CreatorAmount:          10,
			PlatformCommissionRate: 0.2,
			Status:                 service.AISkillSettlementStatusFailed,
			FailureReason:          "credit down",
			Metadata:               map[string]any{"scene": "admin-replay"},
			CreatedAt:              time.Now().UTC().Add(-time.Minute),
			UpdatedAt:              time.Now().UTC().Add(-time.Minute),
		},
	}
	balance := &adminAISkillSettlementBalanceStub{}
	creator := &adminAISkillSettlementCreatorStub{}
	handler := &AIHandler{
		skillModule: &skillkit.Module{
			DomainRepo: &adminAISkillSettlementDomainRepoStub{
				settlement: &domain.AISkillSettlement{ID: 55, RunID: 3007},
			},
			SettlementService: service.NewAISkillSettlementService(repo, balance, creator),
		},
	}

	c, rec := newAdminAISkillSettlementContext(t, "/api/v1/admin/skills/settlements/55/replay", `{}`)
	handler.ReplaySkillSettlement(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, 1, balance.charges)
	require.Equal(t, 1, creator.credits)
	require.Equal(t, service.AISkillSettlementStatusSettled, repo.settlement.Status)

	var envelope map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	data, ok := envelope["data"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "settled", data["status"])
}

func TestReplaySkillSettlement_RejectsUnsafeFailedSettlement(t *testing.T) {
	repo := &adminAISkillSettlementRepoStub{
		settlement: &service.AISkillSettlement{
			ID:                     55,
			RunID:                  3008,
			SkillID:                1008,
			VersionID:              2008,
			BuyerUserID:            907,
			CreatorUserID:          707,
			BillingMode:            service.AISkillBillingModePerRun,
			Currency:               "credit",
			TotalAmount:            12.5,
			PlatformAmount:         2.5,
			CreatorAmount:          10,
			PlatformCommissionRate: 0.2,
			Status:                 service.AISkillSettlementStatusFailed,
			FailureReason:          "buyer refund failed: refund down",
			Metadata:               map[string]any{"scene": "admin-replay-unsafe"},
			CreatedAt:              time.Now().UTC().Add(-time.Minute),
			UpdatedAt:              time.Now().UTC().Add(-time.Minute),
		},
	}
	balance := &adminAISkillSettlementBalanceStub{}
	creator := &adminAISkillSettlementCreatorStub{}
	handler := &AIHandler{
		skillModule: &skillkit.Module{
			DomainRepo: &adminAISkillSettlementDomainRepoStub{
				settlement: &domain.AISkillSettlement{ID: 55, RunID: 3008},
			},
			SettlementService: service.NewAISkillSettlementService(repo, balance, creator),
		},
	}

	c, rec := newAdminAISkillSettlementContext(t, "/api/v1/admin/skills/settlements/55/replay", `{}`)
	handler.ReplaySkillSettlement(c)

	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	require.Equal(t, 0, balance.charges)
	require.Equal(t, 0, creator.credits)
	require.Equal(t, service.AISkillSettlementStatusFailed, repo.settlement.Status)
	require.Contains(t, rec.Body.String(), "AI_SKILL_SETTLEMENT_REPLAY_UNSAFE")
}
