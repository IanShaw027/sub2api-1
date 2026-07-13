package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

func (s *AISkillSettlementService) ReplaySettlement(ctx context.Context, runID int64) (*AISkillSettlement, error) {
	if s == nil || s.repo == nil {
		return nil, ErrAISkillSettlementUnavailable
	}
	if runID <= 0 {
		return nil, infraerrors.BadRequest("AI_SKILL_SETTLEMENT_INPUT_INVALID", "ai skill settlement input is invalid")
	}

	settlement, err := s.repo.GetSettlementByRunID(ctx, runID)
	if err != nil {
		return nil, err
	}
	if settlement == nil {
		return nil, ErrAISkillSettlementNotFound
	}

	return s.Settle(ctx, AISkillSettleInput{
		RunID:         settlement.RunID,
		SkillID:       settlement.SkillID,
		VersionID:     settlement.VersionID,
		BuyerUserID:   settlement.BuyerUserID,
		CreatorUserID: settlement.CreatorUserID,
		BillingPolicy: AISkillBillingPolicy{
			Mode:                   settlement.BillingMode,
			PricePerRun:            settlement.TotalAmount,
			PlatformCommissionRate: settlement.PlatformCommissionRate,
			Currency:               settlement.Currency,
		},
		Metadata: cloneAIMap(settlement.Metadata),
		Trace: AIWriteTrace{
			RequestID:  settlement.Trace.RequestID,
			UsageLogID: settlement.Trace.UsageLogID,
			APIKeyID:   settlement.Trace.APIKeyID,
			GroupID:    settlement.Trace.GroupID,
		},
	})
}

func (s *AISkillSettlementService) Settle(ctx context.Context, input AISkillSettleInput) (*AISkillSettlement, error) {
	if s == nil || s.repo == nil {
		return nil, ErrAISkillSettlementUnavailable
	}
	if input.RunID <= 0 || input.SkillID <= 0 || input.VersionID <= 0 || input.BuyerUserID <= 0 || input.CreatorUserID <= 0 {
		return nil, infraerrors.BadRequest("AI_SKILL_SETTLEMENT_INPUT_INVALID", "ai skill settlement input is invalid")
	}
	now := s.nowOrDefault()
	var settlement *AISkillSettlement
	if existing, err := s.repo.GetSettlementByRunID(ctx, input.RunID); err == nil && existing != nil {
		status := strings.ToLower(strings.TrimSpace(existing.Status))
		if status == AISkillSettlementStatusSettled || status == AISkillSettlementStatusSkipped {
			return existing, nil
		}
		if status == AISkillSettlementStatusPending {
			// Fresh concurrent settle — reject. Stale pending (crash after charge /
			// before status flip) can be reclaimed; ChargeUserBalance is keyed by
			// run reference and must be idempotent.
			if time.Since(existing.UpdatedAt) < 2*time.Minute {
				return nil, ErrAISkillSettlementInProgress
			}
			settlement = cloneAISkillSettlement(existing)
			settlement.FailureReason = ""
			settlement.UpdatedAt = now
			if err := s.repo.UpdateSettlement(ctx, settlement); err != nil {
				return nil, err
			}
		}
		if status == AISkillSettlementStatusFailed {
			if !canReplayFailedAISkillSettlement(existing) {
				return nil, ErrAISkillSettlementReplayUnsafe
			}
			settlement = cloneAISkillSettlement(existing)
			settlement.Status = AISkillSettlementStatusPending
			settlement.FailureReason = ""
			settlement.UpdatedAt = now
			if err := s.repo.UpdateSettlement(ctx, settlement); err != nil {
				return nil, err
			}
		}
	} else if err != nil && !errorsIsAISkillSettlementNotFound(err) {
		return nil, err
	}
	quote, err := s.Quote(input.BillingPolicy)
	if err != nil {
		return nil, err
	}
	if settlement == nil {
		settlement = &AISkillSettlement{
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
	} else {
		settlement.Metadata = cloneAIMap(input.Metadata)
		settlement.Trace = normalizeAIWriteTrace(ctx, input.Trace)
		settlement.BillingMode = quote.BillingMode
		settlement.Currency = quote.Currency
		settlement.TotalAmount = quote.TotalAmount
		settlement.PlatformAmount = quote.PlatformAmount
		settlement.CreatorAmount = quote.CreatorAmount
		settlement.PlatformCommissionRate = quote.PlatformCommissionRate
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
		// Buyer has already been charged. Retry status flip so the settlement
		// does not remain permanently stuck in pending (Replay used to reject that).
		var lastErr error = err
		for attempt := 0; attempt < 3; attempt++ {
			time.Sleep(time.Duration(attempt+1) * 50 * time.Millisecond)
			settlement.UpdatedAt = s.nowOrDefault()
			if lastErr = s.repo.UpdateSettlement(ctx, settlement); lastErr == nil {
				return settlement, nil
			}
		}
		return nil, fmt.Errorf("ai skill settlement charged but finalize failed: %w", lastErr)
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
			remaining := settlement.CreatorCreditedAmount - reversed
			if remaining < 0 {
				remaining = 0
			}
			settlement.CreatorCreditedAmount = roundTo(remaining, 8)
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

func errorsIsAISkillSettlementNotFound(err error) bool {
	return err == nil || errors.Is(err, ErrAISkillSettlementNotFound)
}

func canReplayFailedAISkillSettlement(settlement *AISkillSettlement) bool {
	if settlement == nil || strings.ToLower(strings.TrimSpace(settlement.Status)) != AISkillSettlementStatusFailed {
		return false
	}
	if settlement.CreatorCreditedAmount > 0 {
		return false
	}
	reason := strings.ToLower(strings.TrimSpace(settlement.FailureReason))
	if strings.Contains(reason, "buyer refund failed") || strings.Contains(reason, "creator reversal failed") {
		return false
	}
	return true
}

func cloneAISkillSettlement(settlement *AISkillSettlement) *AISkillSettlement {
	if settlement == nil {
		return nil
	}
	copy := *settlement
	copy.Metadata = cloneAIMap(settlement.Metadata)
	if settlement.BalanceAfter != nil {
		balanceAfter := *settlement.BalanceAfter
		copy.BalanceAfter = &balanceAfter
	}
	return &copy
}
