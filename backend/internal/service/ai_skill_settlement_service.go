package service

import (
	"context"
	"fmt"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type AISkillSettlementService struct {
	repo            AISkillSettlementRepository
	balanceCharger  AISkillBalanceCharger
	creatorCreditor AISkillCreatorEarningsCreditor
	now             func() time.Time
}

func NewAISkillSettlementService(repo AISkillSettlementRepository, balanceCharger AISkillBalanceCharger, creatorCreditor AISkillCreatorEarningsCreditor) *AISkillSettlementService {
	return &AISkillSettlementService{
		repo:            repo,
		balanceCharger:  balanceCharger,
		creatorCreditor: creatorCreditor,
		now:             time.Now,
	}
}

func (s *AISkillSettlementService) Quote(policy AISkillBillingPolicy) (*AISkillSettlementQuote, error) {
	return buildAISkillSettlementQuote(policy)
}

func (s *AISkillSettlementService) QuoteVersion(version *AISkillVersion) (*AISkillSettlementQuote, error) {
	if version == nil {
		return nil, ErrAISkillVersionNotFound
	}
	return s.Quote(version.BillingPolicy)
}

func (s *AISkillSettlementService) Settle(ctx context.Context, input AISkillSettleInput) (*AISkillSettlement, error) {
	if s == nil || s.repo == nil {
		return nil, ErrAISkillSettlementUnavailable
	}
	if input.RunID <= 0 || input.SkillID <= 0 || input.VersionID <= 0 || input.BuyerUserID <= 0 || input.CreatorUserID <= 0 {
		return nil, infraerrors.BadRequest("AI_SKILL_SETTLEMENT_INPUT_INVALID", "ai skill settlement input is invalid")
	}
	quote, err := s.Quote(input.BillingPolicy)
	if err != nil {
		return nil, err
	}
	now := s.nowOrDefault()
	settlement := &AISkillSettlement{
		RunID:                  input.RunID,
		SkillID:                input.SkillID,
		VersionID:              input.VersionID,
		BuyerUserID:            input.BuyerUserID,
		CreatorUserID:          input.CreatorUserID,
		BillingMode:            quote.BillingMode,
		Currency:               quote.Currency,
		TotalAmount:            quote.TotalAmount,
		PlatformAmount:         quote.PlatformAmount,
		CreatorAmount:          quote.CreatorAmount,
		PlatformCommissionRate: quote.PlatformCommissionRate,
		Status:                 AISkillSettlementStatusPending,
		Metadata:               cloneAIMap(input.Metadata),
		Trace:                  normalizeAIWriteTrace(ctx, input.Trace),
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if quote.TotalAmount == 0 {
		settlement.Status = AISkillSettlementStatusSkipped
	}
	if err := s.repo.CreateSettlement(ctx, settlement); err != nil {
		return nil, err
	}
	if quote.TotalAmount == 0 {
		return settlement, nil
	}
	if s.balanceCharger == nil {
		settlement.FailureReason = ErrAISkillBalanceServiceUnavailable.Error()
		if updateErr := s.markFailed(ctx, settlement); updateErr != nil {
			return nil, updateErr
		}
		return nil, ErrAISkillBalanceServiceUnavailable
	}
	if quote.CreatorAmount > 0 && s.creatorCreditor == nil {
		settlement.FailureReason = ErrAISkillCreatorEarningsUnavailable.Error()
		if updateErr := s.markFailed(ctx, settlement); updateErr != nil {
			return nil, updateErr
		}
		return nil, ErrAISkillCreatorEarningsUnavailable
	}

	chargeRef := fmt.Sprintf("ai_skill_run:%d", input.RunID)
	charge, err := s.balanceCharger.ChargeUserBalance(ctx, AISkillBalanceChargeInput{
		UserID:    input.BuyerUserID,
		Amount:    quote.TotalAmount,
		Reference: chargeRef,
		Metadata:  cloneAIMap(input.Metadata),
		Trace:     input.Trace,
	})
	if err != nil {
		settlement.FailureReason = err.Error()
		if updateErr := s.markFailed(ctx, settlement); updateErr != nil {
			return nil, updateErr
		}
		return nil, err
	}
	if charge != nil {
		settlement.BalanceAfter = &charge.BalanceAfter
	}
	chargedAmount := quote.TotalAmount
	if charge != nil && charge.ChargedAmount > 0 {
		chargedAmount = charge.ChargedAmount
	}

	if quote.CreatorAmount > 0 {
		credited, creditErr := s.creatorCreditor.CreditCreatorEarnings(ctx, AISkillCreatorEarningsInput{
			CreatorUserID: input.CreatorUserID,
			BuyerUserID:   input.BuyerUserID,
			SkillID:       input.SkillID,
			VersionID:     input.VersionID,
			RunID:         input.RunID,
			Amount:        quote.CreatorAmount,
			Currency:      quote.Currency,
			Metadata:      cloneAIMap(input.Metadata),
			Trace:         input.Trace,
		})
		if creditErr != nil {
			if updateErr := s.failAfterBuyerCharge(ctx, settlement, AISkillBalanceRefundInput{
				UserID:    input.BuyerUserID,
				Amount:    chargedAmount,
				Reference: chargeRef,
				Metadata:  cloneAIMap(input.Metadata),
				Trace:     input.Trace,
			}, creditErr); updateErr != nil {
				return nil, updateErr
			}
			return nil, creditErr
		}
		if roundTo(credited, 8) != roundTo(quote.CreatorAmount, 8) {
			settlement.CreatorCreditedAmount = roundTo(credited, 8)
			if updateErr := s.failAfterBuyerCharge(ctx, settlement, AISkillBalanceRefundInput{
				UserID:    input.BuyerUserID,
				Amount:    chargedAmount,
				Reference: chargeRef,
				Metadata:  cloneAIMap(input.Metadata),
				Trace:     input.Trace,
			}, ErrAISkillCreatorEarningsMismatch); updateErr != nil {
				return nil, updateErr
			}
			return nil, ErrAISkillCreatorEarningsMismatch
		}
		settlement.CreatorCreditedAmount = roundTo(credited, 8)
	}

	settlement.Status = AISkillSettlementStatusSettled
	settlement.UpdatedAt = s.nowOrDefault()
	if err := s.repo.UpdateSettlement(ctx, settlement); err != nil {
		return nil, err
	}
	return settlement, nil
}

func (s *AISkillSettlementService) failAfterBuyerCharge(ctx context.Context, settlement *AISkillSettlement, refund AISkillBalanceRefundInput, cause error) error {
	if settlement == nil {
		return nil
	}
	if cause != nil {
		settlement.FailureReason = cause.Error()
	}
	if settlement.CreatorCreditedAmount > 0 && s != nil && s.creatorCreditor != nil {
		if reversed, err := s.creatorCreditor.ReverseCreatorEarnings(ctx, AISkillCreatorEarningsReversalInput{
			CreatorUserID: settlement.CreatorUserID,
			RunID:         settlement.RunID,
			Amount:        settlement.CreatorCreditedAmount,
			Currency:      settlement.Currency,
			Metadata:      cloneAIMap(settlement.Metadata),
			Trace:         settlement.Trace,
		}); err != nil {
			settlement.FailureReason = appendAISkillSettlementFailure(settlement.FailureReason, fmt.Errorf("creator reversal failed: %w", err))
		} else if reversed > 0 {
			settlement.CreatorCreditedAmount = roundTo(reversed, 8)
		}
	}
	if refund.Amount > 0 {
		if s == nil || s.balanceCharger == nil {
			settlement.FailureReason = appendAISkillSettlementFailure(settlement.FailureReason, ErrAISkillBalanceServiceUnavailable)
		} else if refundResult, err := s.balanceCharger.RefundUserBalance(ctx, refund); err != nil {
			settlement.FailureReason = appendAISkillSettlementFailure(settlement.FailureReason, fmt.Errorf("buyer refund failed: %w", err))
		} else if refundResult != nil {
			settlement.BalanceAfter = &refundResult.BalanceAfter
		}
	}
	return s.markFailed(ctx, settlement)
}

func appendAISkillSettlementFailure(current string, err error) string {
	if err == nil {
		return current
	}
	if current == "" {
		return err.Error()
	}
	return fmt.Sprintf("%s; %s", current, err.Error())
}

func (s *AISkillSettlementService) nowOrDefault() time.Time {
	if s != nil && s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *AISkillSettlementService) markFailed(ctx context.Context, settlement *AISkillSettlement) error {
	if settlement == nil {
		return nil
	}
	settlement.Status = AISkillSettlementStatusFailed
	settlement.UpdatedAt = s.nowOrDefault()
	return s.repo.UpdateSettlement(ctx, settlement)
}
