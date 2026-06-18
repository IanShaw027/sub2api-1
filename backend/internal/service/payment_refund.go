package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/paymentproviderinstance"
	"github.com/Wei-Shaw/sub2api/ent/usagelog"
	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// --- Refund Flow ---

// getOrderProviderInstance looks up the provider instance that processed this order.
// For legacy orders without provider_instance_id, it resolves only when the
// historical instance is uniquely identifiable from the stored order fields.
func (s *PaymentService) getOrderProviderInstance(ctx context.Context, o *dbent.PaymentOrder) (*dbent.PaymentProviderInstance, error) {
	if s == nil || s.entClient == nil || o == nil {
		return nil, nil
	}

	if snapshot := psOrderProviderSnapshot(o); snapshot != nil {
		return s.resolveSnapshotOrderProviderInstance(ctx, o, snapshot)
	}

	instIDStr := strings.TrimSpace(psStringValue(o.ProviderInstanceID))
	if instIDStr == "" {
		return s.resolveUniqueLegacyOrderProviderInstance(ctx, o)
	}

	instID, err := strconv.ParseInt(instIDStr, 10, 64)
	if err != nil {
		return nil, nil
	}
	return s.entClient.PaymentProviderInstance.Get(ctx, instID)
}

// getRefundOrderProviderInstance resolves the provider instance for refund paths.
// Snapshot-backed and pinned orders stay strict. Legacy orders without either
// binding may fall back only when a single compatible historical instance can
// be recovered from stored order fields.
func (s *PaymentService) getRefundOrderProviderInstance(ctx context.Context, o *dbent.PaymentOrder) (*dbent.PaymentProviderInstance, error) {
	if s == nil || s.entClient == nil || o == nil {
		return nil, nil
	}

	if snapshot := psOrderProviderSnapshot(o); snapshot != nil {
		return s.resolveSnapshotOrderProviderInstance(ctx, o, snapshot)
	}

	instIDStr := strings.TrimSpace(psStringValue(o.ProviderInstanceID))
	if instIDStr == "" {
		return s.resolveUniqueLegacyOrderProviderInstance(ctx, o)
	}

	instID, err := strconv.ParseInt(instIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("order %d refund provider instance id is invalid: %s", o.ID, instIDStr)
	}
	inst, err := s.entClient.PaymentProviderInstance.Get(ctx, instID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, fmt.Errorf("order %d refund provider instance %s is missing", o.ID, instIDStr)
		}
		return nil, err
	}
	return inst, nil
}

func (s *PaymentService) resolveUniqueLegacyOrderProviderInstance(ctx context.Context, o *dbent.PaymentOrder) (*dbent.PaymentProviderInstance, error) {
	paymentType := payment.GetBasePaymentType(strings.TrimSpace(o.PaymentType))
	providerKey := strings.TrimSpace(psStringValue(o.ProviderKey))
	if providerKey != "" {
		instances, err := s.entClient.PaymentProviderInstance.Query().
			Where(paymentproviderinstance.ProviderKeyEQ(providerKey)).
			All(ctx)
		if err != nil {
			return nil, err
		}
		matched := psFilterLegacyOrderProviderInstances(paymentType, instances)
		if len(matched) == 1 {
			return matched[0], nil
		}
		return nil, nil
	}

	if paymentType == "" {
		return nil, nil
	}

	instances, err := s.entClient.PaymentProviderInstance.Query().
		All(ctx)
	if err != nil {
		return nil, err
	}

	matched := psFilterLegacyOrderProviderInstances(paymentType, instances)
	if len(matched) == 1 {
		return matched[0], nil
	}
	return nil, nil
}

func psFilterLegacyOrderProviderInstances(orderPaymentType string, instances []*dbent.PaymentProviderInstance) []*dbent.PaymentProviderInstance {
	if len(instances) == 0 {
		return nil
	}
	if strings.TrimSpace(orderPaymentType) == "" {
		return instances
	}
	var matched []*dbent.PaymentProviderInstance
	for _, inst := range instances {
		if psLegacyOrderMatchesInstance(orderPaymentType, inst) {
			matched = append(matched, inst)
		}
	}
	return matched
}

func psLegacyOrderMatchesInstance(orderPaymentType string, inst *dbent.PaymentProviderInstance) bool {
	if inst == nil {
		return false
	}

	baseType := payment.GetBasePaymentType(strings.TrimSpace(orderPaymentType))
	instanceProviderKey := strings.TrimSpace(inst.ProviderKey)
	if baseType == "" {
		return false
	}

	if baseType == payment.TypeStripe {
		return instanceProviderKey == payment.TypeStripe
	}
	if instanceProviderKey == payment.TypeStripe {
		return false
	}
	if instanceProviderKey == baseType {
		return true
	}
	return payment.InstanceSupportsType(inst.SupportedTypes, baseType)
}

func (s *PaymentService) RequestRefund(ctx context.Context, oid, uid int64, amount float64, reason string) (*RefundResult, error) {
	_, inst, preview, err := s.validateRefundRequest(ctx, oid, uid)
	if err != nil {
		return nil, err
	}
	nr := strings.TrimSpace(reason)
	if amount <= 0 {
		amount = preview.MaxRefundAmount
	}
	if amount <= 0 {
		return nil, infraerrors.BadRequest("NO_REFUNDABLE_AMOUNT", "no refundable amount")
	}
	if amountCentsGreaterThan(amount, preview.MaxRefundAmount) {
		return nil, infraerrors.BadRequest("REFUND_AMOUNT_EXCEEDED", "refund amount exceeds refundable amount")
	}
	if inst.AllowUserRefund {
		plan, earlyResult, err := s.PrepareRefund(ctx, oid, amount, nr, false, true)
		if err != nil {
			return nil, err
		}
		if earlyResult != nil {
			return earlyResult, nil
		}
		return s.ExecuteRefund(ctx, plan)
	}
	now := time.Now()
	by := fmt.Sprintf("%d", uid)
	c, err := s.entClient.PaymentOrder.Update().
		Where(paymentorder.IDEQ(oid), paymentorder.UserIDEQ(uid), paymentorder.StatusEQ(OrderStatusCompleted)).
		SetStatus(OrderStatusRefundRequested).
		SetRefundRequestedAt(now).
		SetRefundRequestedAmount(amount).
		SetRefundRequestReason(nr).
		SetRefundRequestedBy(by).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("update: %w", err)
	}
	if c == 0 {
		return nil, infraerrors.Conflict("CONFLICT", "order status changed")
	}
	s.writeAuditLog(ctx, oid, "REFUND_REQUESTED", fmt.Sprintf("user:%d", uid), map[string]any{"amount": amount, "reason": nr})
	return &RefundResult{Success: false}, nil
}

func (s *PaymentService) validateRefundRequest(ctx context.Context, oid, uid int64) (*dbent.PaymentOrder, *dbent.PaymentProviderInstance, *RefundPreview, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return nil, nil, nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.UserID != uid {
		return nil, nil, nil, infraerrors.Forbidden("FORBIDDEN", "no permission")
	}
	if o.Status != OrderStatusCompleted {
		return nil, nil, nil, infraerrors.BadRequest("INVALID_STATUS", "only completed orders can request refund")
	}
	inst, err := s.getRefundOrderProviderInstance(ctx, o)
	if err != nil || inst == nil {
		return nil, nil, nil, infraerrors.Forbidden("REFUND_DISABLED", "refund is not available for this order")
	}
	if !inst.RefundEnabled {
		return nil, nil, nil, infraerrors.Forbidden("REFUND_DISABLED", "refund is not enabled for this provider")
	}
	if _, hasActiveInvoice, invoiceErr := s.orderHasActiveInvoice(ctx, o.ID); invoiceErr != nil {
		return nil, nil, nil, infraerrors.Forbidden("INVOICE_STATE_UNAVAILABLE", "invoice state is unavailable")
	} else if hasActiveInvoice {
		return nil, nil, nil, infraerrors.BadRequest("INVOICE_ACTIVE", "order has an active invoice application")
	}
	preview, err := s.refundPreviewForOrder(ctx, o, inst)
	if err != nil {
		return nil, nil, nil, err
	}
	return o, inst, preview, nil
}

func (s *PaymentService) GetRefundPreview(ctx context.Context, oid, uid int64) (*RefundPreview, error) {
	_, _, preview, err := s.validateRefundRequest(ctx, oid, uid)
	if err != nil {
		return nil, err
	}
	return preview, nil
}

func (s *PaymentService) refundPreviewForOrder(ctx context.Context, o *dbent.PaymentOrder, inst *dbent.PaymentProviderInstance) (*RefundPreview, error) {
	remaining := remainingRefundAmount(o)
	p := &RefundPreview{
		OrderID:         o.ID,
		OrderType:       o.OrderType,
		OrderAmount:     o.Amount,
		AlreadyRefunded: o.RefundAmount,
		MaxRefundAmount: remaining,
		RefundEnabled:   inst != nil && inst.RefundEnabled,
		AutoRefund:      inst != nil && inst.AllowUserRefund,
	}
	if remaining <= 0 {
		return p, nil
	}
	switch o.OrderType {
	case payment.OrderTypeBalance:
		if s.userRepo == nil {
			return p, nil
		}
		u, err := s.userRepo.GetByID(ctx, o.UserID)
		if err != nil {
			return nil, fmt.Errorf("get user: %w", err)
		}
		p.BalanceAvailable = u.Balance
		p.MaxRefundAmount = math.Min(remaining, u.Balance)
		return p, nil
	case payment.OrderTypeSubscription:
		usage, subRate, refundRate, err := s.subscriptionRefundUsage(ctx, o)
		if err != nil {
			return nil, err
		}
		p.UsageAmount = usage
		p.SubscriptionRateMultiplier = subRate
		p.RefundRateMultiplier = refundRate
		if subRate <= 0 {
			subRate = 1
		}
		if refundRate <= 0 {
			refundRate = 1
		}
		p.UsedRefundValue = usage / subRate * refundRate
		p.MaxRefundAmount = math.Max(remaining-p.UsedRefundValue, 0)
		return p, nil
	default:
		return nil, infraerrors.BadRequest("INVALID_ORDER_TYPE", "unsupported order type")
	}
}

func (s *PaymentService) subscriptionRefundUsage(ctx context.Context, o *dbent.PaymentOrder) (usage, subscriptionRate, refundRate float64, err error) {
	subscriptionRate = 1
	refundRate = 1
	if o.SubscriptionGroupID == nil {
		return 0, subscriptionRate, refundRate, infraerrors.BadRequest("INVALID_ORDER_TYPE", "subscription order missing group")
	}
	g, err := s.entClient.Group.Get(ctx, *o.SubscriptionGroupID)
	if err != nil {
		return 0, subscriptionRate, refundRate, fmt.Errorf("get subscription group: %w", err)
	}
	subscriptionRate = g.RateMultiplier
	refundRate = normalizeRefundRateMultiplier(g.RefundRateMultiplier)
	sub, err := s.entClient.UserSubscription.Query().
		Where(
			usersubscription.UserIDEQ(o.UserID),
			usersubscription.GroupIDEQ(*o.SubscriptionGroupID),
			usersubscription.DeletedAtIsNil(),
		).
		Order(dbent.Desc(usersubscription.FieldID)).
		First(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return 0, subscriptionRate, refundRate, nil
		}
		return 0, subscriptionRate, refundRate, err
	}
	start := o.CreatedAt
	if o.CompletedAt != nil {
		start = *o.CompletedAt
	} else if o.PaidAt != nil {
		start = *o.PaidAt
	}
	var rows []struct {
		Sum float64 `json:"sum"`
	}
	err = s.entClient.UsageLog.Query().
		Where(
			usagelog.UserIDEQ(o.UserID),
			usagelog.GroupIDEQ(*o.SubscriptionGroupID),
			usagelog.SubscriptionIDEQ(sub.ID),
			usagelog.CreatedAtGTE(start),
		).
		Aggregate(dbent.As(dbent.Sum(usagelog.FieldActualCost), "sum")).
		Scan(ctx, &rows)
	if err != nil {
		return 0, subscriptionRate, refundRate, err
	}
	if len(rows) > 0 {
		usage = rows[0].Sum
	}
	return usage, subscriptionRate, refundRate, nil
}

func (s *PaymentService) PrepareRefund(ctx context.Context, oid int64, amt float64, reason string, force, deduct bool) (*RefundPlan, *RefundResult, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return nil, nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	ok := []string{OrderStatusCompleted, OrderStatusRefundRequested, OrderStatusPartiallyRefunded, OrderStatusRefundFailed}
	if !psSliceContains(ok, o.Status) {
		return nil, nil, infraerrors.BadRequest("INVALID_STATUS", "order status does not allow refund")
	}
	// Check provider instance allows admin refund
	inst, instErr := s.getRefundOrderProviderInstance(ctx, o)
	if instErr != nil {
		slog.Warn("refund: provider instance lookup failed", "orderID", oid, "error", instErr)
		return nil, nil, infraerrors.InternalServer("PROVIDER_LOOKUP_FAILED", "failed to look up payment provider for this order")
	}
	if inst == nil {
		// Legacy order without provider_instance_id — block refund
		return nil, nil, infraerrors.Forbidden("REFUND_DISABLED", "refund is not available for this order")
	}
	if !inst.RefundEnabled {
		return nil, nil, infraerrors.Forbidden("REFUND_DISABLED", "refund is not enabled for this provider")
	}
	if math.IsNaN(amt) || math.IsInf(amt, 0) {
		return nil, nil, infraerrors.BadRequest("INVALID_AMOUNT", "invalid refund amount")
	}
	if amt <= 0 {
		amt = remainingRefundAmount(o)
	}
	remaining := remainingRefundAmount(o)
	if amountCentsGreaterThan(amt, remaining) {
		return nil, nil, infraerrors.BadRequest("REFUND_AMOUNT_EXCEEDED", "refund amount exceeds remaining refundable amount")
	}
	if preview, previewErr := s.refundPreviewForOrder(ctx, o, inst); previewErr == nil && amountCentsGreaterThan(amt, preview.MaxRefundAmount) {
		return nil, nil, infraerrors.BadRequest("REFUND_AMOUNT_EXCEEDED", "refund amount exceeds refundable amount")
	}
	// Compliance gate: an order that already has an ISSUED (tax) invoice must not
	// be silently refunded — refunding after issuance requires a VAT credit note
	// (红冲). Force the admin to acknowledge this explicitly. APPLIED-only invoices
	// (no file uploaded yet) are auto-cancelled later in markRefundOk.
	if issuedInvoiceID, hasIssued, lookupErr := s.orderHasIssuedInvoice(ctx, o.ID); lookupErr != nil && !force {
		s.writeAuditLog(ctx, o.ID, "REFUND_BLOCKED_INVOICE_LOOKUP_FAILED", "admin", map[string]any{
			"error": lookupErr.Error(),
		})
		return nil, &RefundResult{
			Success:      false,
			RequireForce: true,
			Warning:      "invoice state is unavailable; refund requires force after manual verification",
		}, nil
	} else if hasIssued && !force {
		s.writeAuditLog(ctx, o.ID, "REFUND_BLOCKED_ISSUED_INVOICE", "admin", map[string]any{
			"invoiceID": issuedInvoiceID,
		})
		return nil, &RefundResult{
			Success:      false,
			RequireForce: true,
			Warning:      "order has an issued invoice; refund requires a credit note (use force)",
		}, nil
	}
	orderCurrency := PaymentOrderCurrency(o)
	ga := calculateGatewayRefundAmount(o.Amount, o.PayAmount, amt, orderCurrency)
	rr := strings.TrimSpace(reason)
	if rr == "" && o.RefundRequestReason != nil {
		rr = *o.RefundRequestReason
	}
	if rr == "" {
		rr = fmt.Sprintf("refund order:%d", o.ID)
	}
	p := &RefundPlan{OrderID: oid, Order: o, RefundAmount: amt, GatewayAmount: ga, Reason: rr, Force: force, DeductBalance: deduct, DeductionType: payment.DeductionTypeNone}
	if deduct {
		if er := s.prepDeduct(ctx, o, p, force); er != nil {
			return nil, er, nil
		}
	}
	return p, nil, nil
}

func (s *PaymentService) prepDeduct(ctx context.Context, o *dbent.PaymentOrder, p *RefundPlan, force bool) *RefundResult {
	if o.OrderType == payment.OrderTypeSubscription {
		p.DeductionType = payment.DeductionTypeSubscription
		if o.SubscriptionGroupID != nil && o.SubscriptionDays != nil {
			p.SubDaysToDeduct = calculateSubscriptionRefundDays(*o.SubscriptionDays, o.Amount, p.RefundAmount)
			sub, err := s.subscriptionSvc.GetActiveSubscription(ctx, o.UserID, *o.SubscriptionGroupID)
			if err == nil && sub != nil {
				p.SubscriptionID = sub.ID
			} else if !force {
				return &RefundResult{Success: false, Warning: "cannot find active subscription for deduction, use force", RequireForce: true}
			}
		}
		return nil
	}
	u, err := s.userRepo.GetByID(ctx, o.UserID)
	if err != nil {
		if !force {
			return &RefundResult{Success: false, Warning: "cannot fetch user balance, use force", RequireForce: true}
		}
		return nil
	}
	p.DeductionType = payment.DeductionTypeBalance
	p.BalanceToDeduct = math.Min(p.RefundAmount, u.Balance)
	return nil
}

func calculateSubscriptionRefundDays(subscriptionDays int, orderAmount, refundAmount float64) int {
	if subscriptionDays <= 0 || orderAmount <= 0 || refundAmount <= 0 {
		return 0
	}
	if amountCentsEqual(refundAmount, orderAmount) || amountCentsGreaterThan(refundAmount, orderAmount) {
		return subscriptionDays
	}
	days := int(math.Ceil(float64(subscriptionDays) * refundAmount / orderAmount))
	if days < 1 {
		return 1
	}
	if days > subscriptionDays {
		return subscriptionDays
	}
	return days
}

func (s *PaymentService) ExecuteRefund(ctx context.Context, p *RefundPlan) (*RefundResult, error) {
	c, err := s.entClient.PaymentOrder.Update().
		Where(
			paymentorder.IDEQ(p.OrderID),
			paymentorder.StatusIn(OrderStatusCompleted, OrderStatusRefundRequested, OrderStatusPartiallyRefunded, OrderStatusRefundFailed),
			paymentorder.RefundAmountEQ(p.Order.RefundAmount),
		).
		SetStatus(OrderStatusRefunding).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("lock: %w", err)
	}
	if c == 0 {
		return nil, infraerrors.Conflict("CONFLICT", "order refund state changed")
	}
	p.Order.Status = OrderStatusRefunding
	if p.DeductionType == payment.DeductionTypeBalance && p.BalanceToDeduct > 0 {
		// Skip balance deduction on retry if previous attempt already deducted
		// but failed to roll back (REFUND_ROLLBACK_FAILED in audit log).
		if !s.hasAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED") {
			if err := s.userRepo.DeductBalance(ctx, p.Order.UserID, p.BalanceToDeduct); err != nil {
				s.restoreStatus(ctx, p)
				return nil, fmt.Errorf("deduction: %w", err)
			}
		} else {
			slog.Warn("skipping balance deduction on retry (previous rollback failed)", "orderID", p.OrderID)
			p.BalanceToDeduct = 0
		}
	}
	if p.DeductionType == payment.DeductionTypeSubscription && p.SubDaysToDeduct > 0 && p.SubscriptionID > 0 {
		if !s.hasAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED") {
			_, err := s.subscriptionSvc.ExtendSubscription(ctx, p.SubscriptionID, -p.SubDaysToDeduct)
			if err != nil {
				if errors.Is(err, ErrAdjustWouldExpire) {
					// Deduction would expire the subscription — revoke it entirely
					slog.Info("subscription deduction would expire, revoking", "orderID", p.OrderID, "subID", p.SubscriptionID, "days", p.SubDaysToDeduct)
					if revokeErr := s.subscriptionSvc.RevokeSubscription(ctx, p.SubscriptionID); revokeErr != nil {
						s.restoreStatus(ctx, p)
						return nil, fmt.Errorf("revoke subscription: %w", revokeErr)
					}
				} else {
					// Other errors (DB failure, not found) — abort refund
					s.restoreStatus(ctx, p)
					return nil, fmt.Errorf("deduct subscription days: %w", err)
				}
			}
		} else {
			slog.Warn("skipping subscription deduction on retry (previous rollback failed)", "orderID", p.OrderID)
			p.SubDaysToDeduct = 0
		}
	}
	refundResp, err := s.gwRefund(ctx, p)
	if err != nil {
		return s.handleGwFail(ctx, p, err)
	}
	return s.handleRefundProviderResponse(ctx, p, refundResp)
}

func (s *PaymentService) gwRefund(ctx context.Context, p *RefundPlan) (*payment.RefundResponse, error) {
	if p.Order.PaymentTradeNo == "" {
		s.writeAuditLog(ctx, p.Order.ID, "REFUND_NO_TRADE_NO", "admin", map[string]any{"detail": "skipped"})
		return &payment.RefundResponse{Status: payment.ProviderStatusSuccess}, nil
	}

	// Use the exact provider instance that created this order, not a random one
	// from the registry. Each instance has its own merchant credentials.
	prov, err := s.getRefundProvider(ctx, p.Order)
	if err != nil {
		return nil, fmt.Errorf("get refund provider: %w", err)
	}
	if err := validateProviderSnapshotMetadata(p.Order, prov.ProviderKey(), providerMerchantIdentityMetadata(prov)); err != nil {
		s.writeAuditLog(ctx, p.Order.ID, "REFUND_PROVIDER_METADATA_MISMATCH", "admin", map[string]any{
			"detail": err.Error(),
		})
		return nil, err
	}
	refundResp, err := prov.Refund(ctx, payment.RefundRequest{
		TradeNo: p.Order.PaymentTradeNo,
		OrderID: p.Order.OutTradeNo,
		Amount:  formatGatewayRefundAmount(p.GatewayAmount, p.Order),
		Reason:  p.Reason,
	})
	if err != nil {
		return nil, err
	}
	if err := validateRefundProviderResponse(refundResp); err != nil {
		return nil, err
	}
	return refundResp, nil
}

func formatGatewayRefundAmount(amount float64, order *dbent.PaymentOrder) string {
	return payment.FormatAmountForCurrency(amount, PaymentOrderCurrency(order))
}

func validateRefundProviderResponse(resp *payment.RefundResponse) error {
	if resp == nil {
		return fmt.Errorf("payment refund response missing")
	}
	status := strings.TrimSpace(resp.Status)
	switch status {
	case payment.ProviderStatusSuccess, payment.ProviderStatusRefunded, payment.ProviderStatusPending:
		return nil
	case payment.ProviderStatusFailed:
		return fmt.Errorf("payment refund failed: status %s", status)
	default:
		return fmt.Errorf("payment refund returned unknown status: %s", status)
	}
}

func (s *PaymentService) handleRefundProviderResponse(ctx context.Context, p *RefundPlan, resp *payment.RefundResponse) (*RefundResult, error) {
	status := ""
	refundID := ""
	if resp != nil {
		status = strings.TrimSpace(strings.ToLower(resp.Status))
		refundID = strings.TrimSpace(resp.RefundID)
	}
	if status == "" || status == payment.ProviderStatusSuccess || status == payment.ProviderStatusRefunded {
		return s.markRefundOk(ctx, p)
	}
	if status == payment.ProviderStatusPending {
		if p.Order != nil && p.Order.Status == OrderStatusRefunding {
			if !s.RollbackRefund(ctx, p, fmt.Errorf("gateway refund status: %s", status)) {
				now := time.Now()
				_, _ = s.entClient.PaymentOrder.UpdateOneID(p.OrderID).SetStatus(OrderStatusRefundFailed).SetFailedAt(now).SetFailedReason("gateway refund rollback failed").Save(ctx)
				s.writeAuditLog(ctx, p.OrderID, "REFUND_GATEWAY_PENDING_ROLLBACK_FAILED", "admin", map[string]any{"refundID": refundID, "refundAmount": p.RefundAmount, "reason": p.Reason})
				return nil, infraerrors.InternalServer("REFUND_ROLLBACK_FAILED", "gateway refund pending rollback failed")
			}
		}
		s.restoreStatus(ctx, p)
		s.writeAuditLog(ctx, p.OrderID, "REFUND_GATEWAY_PENDING", "admin", map[string]any{"refundID": refundID, "refundAmount": p.RefundAmount, "reason": p.Reason})
		return &RefundResult{Success: false, Warning: "gateway refund is pending confirmation"}, nil
	}
	return s.handleGwFail(ctx, p, fmt.Errorf("gateway refund status: %s", status))
}

// getRefundProvider creates a provider using the order's original instance config.
// Delegates to getOrderProvider which handles instance lookup and fallback.
func (s *PaymentService) getRefundProvider(ctx context.Context, o *dbent.PaymentOrder) (payment.Provider, error) {
	inst, err := s.getRefundOrderProviderInstance(ctx, o)
	if err != nil {
		return nil, err
	}
	if inst == nil {
		return nil, fmt.Errorf("refund provider instance is unavailable for order %d", o.ID)
	}
	return s.createProviderFromInstance(ctx, inst)
}

func (s *PaymentService) handleGwFail(ctx context.Context, p *RefundPlan, gErr error) (*RefundResult, error) {
	if s.RollbackRefund(ctx, p, gErr) {
		s.restoreStatus(ctx, p)
		s.writeAuditLog(ctx, p.OrderID, "REFUND_GATEWAY_FAILED", "admin", map[string]any{"detail": psErrMsg(gErr)})
		return &RefundResult{Success: false, Warning: "gateway failed: " + psErrMsg(gErr) + ", rolled back"}, nil
	}
	now := time.Now()
	_, _ = s.entClient.PaymentOrder.UpdateOneID(p.OrderID).SetStatus(OrderStatusRefundFailed).SetFailedAt(now).SetFailedReason(psErrMsg(gErr)).Save(ctx)
	s.writeAuditLog(ctx, p.OrderID, "REFUND_FAILED", "admin", map[string]any{"detail": psErrMsg(gErr)})
	return nil, infraerrors.InternalServer("REFUND_FAILED", psErrMsg(gErr))
}

func (s *PaymentService) markRefundOk(ctx context.Context, p *RefundPlan) (*RefundResult, error) {
	totalRefunded := p.Order.RefundAmount + p.RefundAmount
	fs := OrderStatusRefunded
	if amountCentsGreaterThan(p.Order.Amount, totalRefunded) {
		fs = OrderStatusPartiallyRefunded
	}
	now := time.Now()
	_, err := s.entClient.PaymentOrder.UpdateOneID(p.OrderID).SetStatus(fs).SetRefundAmount(totalRefunded).SetRefundReason(p.Reason).SetRefundAt(now).SetForceRefund(p.Force).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("mark refund: %w", err)
	}
	s.writeAuditLog(ctx, p.OrderID, "REFUND_SUCCESS", "admin", map[string]any{"refundAmount": p.RefundAmount, "totalRefunded": totalRefunded, "reason": p.Reason, "balanceDeducted": p.BalanceToDeduct, "force": p.Force})
	s.reverseAffiliateRebateBestEffort(ctx, p.Order, totalRefunded)
	s.voidInvoicesAfterRefund(ctx, p.Order, totalRefunded)
	return &RefundResult{Success: true, BalanceDeducted: p.BalanceToDeduct, SubDaysDeducted: p.SubDaysToDeduct}, nil
}

// reverseAffiliateRebateBestEffort claws back affiliate rebate proportional to the
// cumulative refunded amount. Affiliate balance lives in a separate ledger/quota
// system, so a reversal failure must not fail the (already gateway-confirmed)
// refund; we record an audit row for manual follow-up instead. The reversal itself
// is idempotent (keyed off the order's accrual/reverse ledger), so retries are safe.
func (s *PaymentService) reverseAffiliateRebateBestEffort(ctx context.Context, o *dbent.PaymentOrder, totalRefunded float64) {
	if s == nil || s.affiliateService == nil || o == nil {
		return
	}
	if o.OrderType != payment.OrderTypeBalance || o.Amount <= 0 || totalRefunded <= 0 {
		return
	}
	reversed, err := s.affiliateService.ReverseInviteRebateForOrder(ctx, o.ID, totalRefunded, o.Amount)
	if err != nil {
		slog.Error("affiliate rebate reversal failed after refund", "orderID", o.ID, "error", err)
		s.writeAuditLog(ctx, o.ID, "AFFILIATE_REBATE_REVERSAL_FAILED", "system", map[string]any{
			"totalRefunded": totalRefunded,
			"orderAmount":   o.Amount,
			"error":         err.Error(),
		})
		return
	}
	if reversed > 0 {
		s.writeAuditLog(ctx, o.ID, "AFFILIATE_REBATE_REVERSED", "system", map[string]any{
			"reversedAmount": reversed,
			"totalRefunded":  totalRefunded,
			"orderAmount":    o.Amount,
		})
	}
}

// orderHasIssuedInvoice reports whether the order is covered by an active invoice
// in ISSUED state (file uploaded, externally valid). Returns the invoice ID for
// audit context, plus an error if invoice state could not be determined. The
// admin refund entry point treats lookup errors as fail-closed unless force is
// explicitly supplied.
func (s *PaymentService) orderHasIssuedInvoice(ctx context.Context, orderID int64) (int64, bool, error) {
	if s == nil || s.invoiceVoider == nil {
		return 0, false, nil
	}
	links, err := s.invoiceVoider.GetActiveLinksByOrderIDs(ctx, []int64{orderID})
	if err != nil {
		slog.Warn("refund: invoice lookup failed", "orderID", orderID, "error", err)
		return 0, false, err
	}
	link, ok := links[orderID]
	if !ok || link.InvoiceStatus != InvoiceStatusIssued {
		return 0, false, nil
	}
	return link.InvoiceID, true, nil
}

func (s *PaymentService) orderHasActiveInvoice(ctx context.Context, orderID int64) (int64, bool, error) {
	if s == nil || s.invoiceVoider == nil {
		return 0, false, nil
	}
	links, err := s.invoiceVoider.GetActiveLinksByOrderIDs(ctx, []int64{orderID})
	if err != nil {
		slog.Warn("refund request: invoice lookup failed", "orderID", orderID, "error", err)
		return 0, false, err
	}
	link, ok := links[orderID]
	if !ok {
		return 0, false, nil
	}
	return link.InvoiceID, true, nil
}

// voidInvoicesAfterRefund reconciles invoices tied to a refunded order:
//   - APPLIED (no file uploaded): auto-cancel the application so the order is
//     released and the user is not left with a pending request for refunded money.
//   - ISSUED (tax invoice already delivered): we cannot programmatically void a
//     filed invoice; we record AFFILIATE-style audit (INVOICE_NEEDS_CREDIT_NOTE)
//     so finance issues a VAT credit note (红冲). No DB enum change is made.
//
// Best-effort: failures are logged/audited but never fail the completed refund.
func (s *PaymentService) voidInvoicesAfterRefund(ctx context.Context, o *dbent.PaymentOrder, totalRefunded float64) {
	if s == nil || s.invoiceVoider == nil || o == nil {
		return
	}
	links, err := s.invoiceVoider.GetActiveLinksByOrderIDs(ctx, []int64{o.ID})
	if err != nil {
		slog.Warn("refund: invoice void lookup failed", "orderID", o.ID, "error", err)
		s.writeAuditLog(ctx, o.ID, "INVOICE_VOID_LOOKUP_FAILED", "system", map[string]any{"error": err.Error()})
		return
	}
	link, ok := links[o.ID]
	if !ok {
		return
	}
	switch link.InvoiceStatus {
	case InvoiceStatusApplied:
		if _, err := s.invoiceVoider.CancelByAdmin(ctx, link.InvoiceID); err != nil {
			slog.Error("refund: auto-cancel applied invoice failed", "orderID", o.ID, "invoiceID", link.InvoiceID, "error", err)
			s.writeAuditLog(ctx, o.ID, "INVOICE_AUTO_CANCEL_FAILED", "system", map[string]any{
				"invoiceID": link.InvoiceID,
				"error":     err.Error(),
			})
			return
		}
		s.writeAuditLog(ctx, o.ID, "INVOICE_AUTO_CANCELLED_AFTER_REFUND", "system", map[string]any{
			"invoiceID": link.InvoiceID,
		})
	case InvoiceStatusIssued:
		// Issued (filed) invoice — flag for a manual credit note. Refund was forced
		// past the PrepareRefund gate, so this is expected and recorded for finance.
		s.writeAuditLog(ctx, o.ID, "INVOICE_NEEDS_CREDIT_NOTE", "system", map[string]any{
			"invoiceID":     link.InvoiceID,
			"totalRefunded": totalRefunded,
			"orderAmount":   o.Amount,
		})
	}
}

func remainingRefundAmount(o *dbent.PaymentOrder) float64 {
	if o == nil {
		return 0
	}
	remaining := o.Amount - o.RefundAmount
	if remaining <= 0 {
		return 0
	}
	return remaining
}

func (s *PaymentService) RollbackRefund(ctx context.Context, p *RefundPlan, gErr error) bool {
	if p.DeductionType == payment.DeductionTypeBalance && p.BalanceToDeduct > 0 {
		if err := s.userRepo.UpdateBalance(ctx, p.Order.UserID, p.BalanceToDeduct); err != nil {
			slog.Error("[CRITICAL] rollback failed", "orderID", p.OrderID, "amount", p.BalanceToDeduct, "error", err)
			s.writeAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED", "admin", map[string]any{"gatewayError": psErrMsg(gErr), "rollbackError": psErrMsg(err), "balanceDeducted": p.BalanceToDeduct})
			return false
		}
	}
	if p.DeductionType == payment.DeductionTypeSubscription && p.SubDaysToDeduct > 0 && p.SubscriptionID > 0 {
		if _, err := s.subscriptionSvc.ExtendSubscription(ctx, p.SubscriptionID, p.SubDaysToDeduct); err != nil {
			slog.Error("[CRITICAL] subscription rollback failed", "orderID", p.OrderID, "subID", p.SubscriptionID, "days", p.SubDaysToDeduct, "error", err)
			s.writeAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED", "admin", map[string]any{"gatewayError": psErrMsg(gErr), "rollbackError": psErrMsg(err), "subDaysDeducted": p.SubDaysToDeduct})
			return false
		}
	}
	return true
}

func (s *PaymentService) restoreStatus(ctx context.Context, p *RefundPlan) {
	rs := OrderStatusCompleted
	if p.Order.Status == OrderStatusRefundRequested || p.Order.Status == OrderStatusPartiallyRefunded {
		rs = p.Order.Status
	}
	_, _ = s.entClient.PaymentOrder.UpdateOneID(p.OrderID).SetStatus(rs).Save(ctx)
}
