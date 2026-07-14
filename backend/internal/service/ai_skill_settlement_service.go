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

const (
	aiSkillSettlementDispatchStateMetadataKey = "_sub2api_dispatch_state"
	aiSkillSettlementDispatchStateAwaiting    = "awaiting"
	aiSkillSettlementDispatchStateConfirmed   = "confirmed"
	aiSkillSettlementDispatchStateFailed      = "failed"
	aiSkillSettlementDispatchFailureReason    = "upstream dispatch did not succeed; settlement must not be replayed"
)

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
	return s.settle(ctx, input, 0)
}

// CreateIntent persists a settlement before the upstream request is dispatched.
// The awaiting marker makes the row non-replayable until the owning Execute call
// confirms that dispatch succeeded.
func (s *AISkillSettlementService) CreateIntent(ctx context.Context, input AISkillSettleInput) (*AISkillSettlement, error) {
	if err := s.validateSettleInput(input); err != nil {
		return nil, err
	}
	quote, err := s.Quote(input.BillingPolicy)
	if err != nil {
		return nil, err
	}
	now := s.nowOrDefault()
	metadata := cloneAIMap(input.Metadata)
	metadata[aiSkillSettlementDispatchStateMetadataKey] = aiSkillSettlementDispatchStateAwaiting
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
		Metadata:               metadata,
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
	return settlement, nil
}

// SettleIntent lets only the Execute call that created intentID claim a fresh
// pending row. Public Settle and ReplaySettlement retain the pending-row guard.
func (s *AISkillSettlementService) SettleIntent(ctx context.Context, input AISkillSettleInput, intentID int64) (*AISkillSettlement, error) {
	if err := s.validateSettleInput(input); err != nil {
		return nil, err
	}
	if _, err := s.ConfirmIntent(ctx, input.RunID, intentID); err != nil {
		return nil, err
	}
	return s.settle(ctx, input, intentID)
}

// ConfirmIntent records that the upstream accepted the execution before the
// intent becomes eligible for immediate settlement or later replay.
func (s *AISkillSettlementService) ConfirmIntent(ctx context.Context, runID, intentID int64) (*AISkillSettlement, error) {
	if s == nil || s.repo == nil {
		return nil, ErrAISkillSettlementUnavailable
	}
	if runID <= 0 || intentID <= 0 {
		return nil, infraerrors.BadRequest("AI_SKILL_SETTLEMENT_INPUT_INVALID", "ai skill settlement input is invalid")
	}
	intent, err := s.repo.GetSettlementByRunID(ctx, runID)
	if err != nil {
		return nil, err
	}
	if intent == nil || intent.ID != intentID {
		return nil, ErrAISkillSettlementNotFound
	}
	if aiSkillSettlementDispatchState(intent) == aiSkillSettlementDispatchStateConfirmed {
		return intent, nil
	}
	metadata := cloneAIMap(intent.Metadata)
	metadata[aiSkillSettlementDispatchStateMetadataKey] = aiSkillSettlementDispatchStateConfirmed
	intent.Metadata = metadata
	intent.UpdatedAt = s.nowOrDefault()
	if err := s.repo.UpdateSettlement(ctx, intent); err != nil {
		return nil, err
	}
	return intent, nil
}

// FailIntentForDispatch terminalizes a pre-dispatch intent. The failure marker
// is also checked by replay, so a failed upstream request can never charge later.
func (s *AISkillSettlementService) FailIntentForDispatch(ctx context.Context, runID, intentID int64, cause error) (*AISkillSettlement, error) {
	if s == nil || s.repo == nil {
		return nil, ErrAISkillSettlementUnavailable
	}
	if runID <= 0 || intentID <= 0 {
		return nil, infraerrors.BadRequest("AI_SKILL_SETTLEMENT_INPUT_INVALID", "ai skill settlement input is invalid")
	}
	settlement, err := s.repo.GetSettlementByRunID(ctx, runID)
	if err != nil {
		return nil, err
	}
	if settlement == nil || settlement.ID != intentID {
		return nil, ErrAISkillSettlementNotFound
	}
	settlement.Status = AISkillSettlementStatusFailed
	settlement.FailureReason = aiSkillSettlementDispatchFailureReason
	if cause != nil {
		settlement.FailureReason = appendAISkillSettlementFailure(settlement.FailureReason, cause)
	}
	metadata := cloneAIMap(settlement.Metadata)
	metadata[aiSkillSettlementDispatchStateMetadataKey] = aiSkillSettlementDispatchStateFailed
	settlement.Metadata = metadata
	settlement.UpdatedAt = s.nowOrDefault()
	if err := s.repo.UpdateSettlement(ctx, settlement); err != nil {
		return nil, err
	}
	return settlement, nil
}

func (s *AISkillSettlementService) validateSettleInput(input AISkillSettleInput) error {
	if s == nil || s.repo == nil {
		return ErrAISkillSettlementUnavailable
	}
	if input.RunID <= 0 || input.SkillID <= 0 || input.VersionID <= 0 || input.BuyerUserID <= 0 || input.CreatorUserID <= 0 {
		return infraerrors.BadRequest("AI_SKILL_SETTLEMENT_INPUT_INVALID", "ai skill settlement input is invalid")
	}
	return nil
}

func (s *AISkillSettlementService) settle(ctx context.Context, input AISkillSettleInput, ownedIntentID int64) (*AISkillSettlement, error) {
	if err := s.validateSettleInput(input); err != nil {
		return nil, err
	}
	now := s.nowOrDefault()
	var settlement *AISkillSettlement
	if existing, err := s.repo.GetSettlementByRunID(ctx, input.RunID); err == nil && existing != nil {
		status := strings.ToLower(strings.TrimSpace(existing.Status))
		if status == AISkillSettlementStatusSettled || status == AISkillSettlementStatusSkipped {
			return existing, nil
		}
		if status == AISkillSettlementStatusPending {
			if ownedIntentID > 0 && existing.ID == ownedIntentID {
				settlement = cloneAISkillSettlement(existing)
			} else if aiSkillSettlementDispatchState(existing) == aiSkillSettlementDispatchStateAwaiting {
				return nil, ErrAISkillSettlementReplayUnsafe
			}
			// Fresh concurrent settle — reject. Stale pending (crash after charge /
			// before status flip) can be reclaimed; ChargeUserBalance is keyed by
			// run reference and must be idempotent.
			if settlement == nil && time.Since(existing.UpdatedAt) < 2*time.Minute {
				return nil, ErrAISkillSettlementInProgress
			}
			if settlement == nil {
				settlement = cloneAISkillSettlement(existing)
			}
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
	} else if ownedIntentID > 0 {
		return nil, ErrAISkillSettlementNotFound
	}
	quote, err := s.Quote(input.BillingPolicy)
	if err != nil {
		return nil, err
	}
	inputMetadata := cloneAIMap(input.Metadata)
	delete(inputMetadata, aiSkillSettlementDispatchStateMetadataKey)
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
			Metadata:               inputMetadata,
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
		dispatchState := aiSkillSettlementDispatchState(settlement)
		settlement.Metadata = inputMetadata
		if dispatchState != "" {
			settlement.Metadata[aiSkillSettlementDispatchStateMetadataKey] = dispatchState
		}
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
	if aiSkillSettlementDispatchState(settlement) == aiSkillSettlementDispatchStateFailed {
		return false
	}
	reason := strings.ToLower(strings.TrimSpace(settlement.FailureReason))
	if strings.Contains(reason, strings.ToLower(aiSkillSettlementDispatchFailureReason)) || strings.Contains(reason, "buyer refund failed") || strings.Contains(reason, "creator reversal failed") {
		return false
	}
	return true
}

func aiSkillSettlementDispatchState(settlement *AISkillSettlement) string {
	if settlement == nil || settlement.Metadata == nil {
		return ""
	}
	state, _ := settlement.Metadata[aiSkillSettlementDispatchStateMetadataKey].(string)
	return strings.ToLower(strings.TrimSpace(state))
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
