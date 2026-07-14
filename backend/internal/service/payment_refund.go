package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/paymentproviderinstance"
	"github.com/Wei-Shaw/sub2api/ent/usagelog"
	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/provider"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/servertiming"
)

// --- Refund Flow ---

var createPaymentProviderFromInstance = provider.CreateProvider

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

func (s *PaymentService) GetAdminRefundPreview(ctx context.Context, oid int64) (*RefundPreview, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	ok := []string{OrderStatusCompleted, OrderStatusRefundRequested, OrderStatusPartiallyRefunded, OrderStatusRefundFailed}
	if !psSliceContains(ok, o.Status) {
		return nil, infraerrors.BadRequest("INVALID_STATUS", "order status does not allow refund")
	}
	inst, instErr := s.getRefundOrderProviderInstance(ctx, o)
	if instErr != nil {
		slog.Warn("refund preview: provider instance lookup failed", "orderID", oid, "error", instErr)
		return nil, infraerrors.InternalServer("PROVIDER_LOOKUP_FAILED", "failed to look up payment provider for this order")
	}
	if inst == nil {
		return nil, infraerrors.Forbidden("REFUND_DISABLED", "refund is not available for this order")
	}
	if !inst.RefundEnabled {
		return nil, infraerrors.Forbidden("REFUND_DISABLED", "refund is not enabled for this provider")
	}
	return s.refundPreviewForOrder(ctx, o, inst)
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
		currency := PaymentOrderCurrency(o)
		p.UsageAmount = usage
		p.SubscriptionRateMultiplier = subRate
		p.RefundRateMultiplier = refundRate
		p.UsedRefundValue, p.MaxRefundAmount = calculateSubscriptionRefundAmounts(remaining, usage, subRate, refundRate, currency)
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
	if o.Status == OrderStatusRefundPending {
		return nil, nil, infraerrors.BadRequest("REFUND_PENDING", "refund is pending; query and finalize instead")
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
		if o.Status == OrderStatusRefundRequested && o.RefundRequestedAmount > 0 {
			amt = o.RefundRequestedAmount
		} else {
			amt = remainingRefundAmount(o)
		}
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
	p := &RefundPlan{OrderID: oid, Order: o, OriginalStatus: o.Status, RefundAmount: amt, GatewayAmount: ga, Reason: rr, Force: force, DeductBalance: deduct, DeductionType: payment.DeductionTypeNone}
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
	p.DeductionType = payment.DeductionTypeBalance
	if err != nil {
		if !force {
			return &RefundResult{Success: false, Warning: "cannot fetch user balance, use force", RequireForce: true}
		}
		p.BalanceToDeduct = 0
		return nil
	}
	if amountCentsGreaterThan(p.RefundAmount, u.Balance) && !force {
		return &RefundResult{
			Success:      false,
			Warning:      "user balance is lower than refund amount, use force",
			RequireForce: true,
		}
	}
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
	now := time.Now()
	c, err := s.entClient.PaymentOrder.Update().
		Where(
			paymentorder.IDEQ(p.OrderID),
			paymentorder.StatusIn(OrderStatusCompleted, OrderStatusRefundRequested, OrderStatusPartiallyRefunded, OrderStatusRefundFailed),
			paymentorder.RefundAmountEQ(p.Order.RefundAmount),
		).
		SetStatus(OrderStatusRefunding).
		SetRefundRequestedAt(now).
		SetRefundRequestedAmount(p.RefundAmount).
		SetRefundReason(p.Reason).
		SetForceRefund(p.Force).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("lock: %w", err)
	}
	if c == 0 {
		return nil, infraerrors.Conflict("CONFLICT", "order refund state changed")
	}
	p.Order.Status = OrderStatusRefunding
	p.Order.RefundRequestedAt = &now
	p.Order.RefundRequestedAmount = p.RefundAmount
	if p.DeductionType == payment.DeductionTypeBalance && p.BalanceToDeduct > 0 {
		// Skip balance deduction on retry if previous attempt already deducted
		// but failed to roll back (REFUND_ROLLBACK_FAILED in audit log).
		if !s.hasAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED") {
			if err := s.userRepo.DeductBalance(ctx, p.Order.UserID, p.BalanceToDeduct); err != nil {
				s.restoreStatus(ctx, p)
				return nil, fmt.Errorf("deduction: %w", err)
			}
			s.invalidateBalanceCacheBestEffort(ctx, p.Order.UserID)
			p.DeductionApplied = true
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
					p.DeductionApplied = true
				} else {
					// Other errors (DB failure, not found) — abort refund
					s.restoreStatus(ctx, p)
					return nil, fmt.Errorf("deduct subscription days: %w", err)
				}
			}
			p.DeductionApplied = true
		} else {
			slog.Warn("skipping subscription deduction on retry (previous rollback failed)", "orderID", p.OrderID)
			p.SubDaysToDeduct = 0
		}
	}
	resp, err := s.gwRefund(ctx, p)
	if err != nil {
		return s.handleGwFail(ctx, p, err)
	}
	return s.finishRefund(ctx, p, resp)
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
	finishProviderCall := servertiming.ObserveDependency(ctx, "payment")
	refundResp, err := prov.Refund(ctx, payment.RefundRequest{
		TradeNo:   p.Order.PaymentTradeNo,
		OrderID:   p.Order.OutTradeNo,
		Amount:    formatGatewayRefundAmount(p.GatewayAmount, p.Order),
		Reason:    p.Reason,
		RequestID: refundProviderRequestID(p),
	})
	finishProviderCall()
	if err != nil {
		if refundResp != nil && strings.TrimSpace(refundResp.Status) == payment.ProviderStatusPending {
			return refundResp, nil
		}
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

func refundProviderRequestID(p *RefundPlan) string {
	if p == nil || p.Order == nil {
		return ""
	}
	currency := PaymentOrderCurrency(p.Order)
	refundMinor, err := payment.AmountToMinorUnit(formatGatewayRefundAmount(p.GatewayAmount, p.Order), currency)
	if err != nil {
		refundMinor = 0
	}
	refundedMinor, err := payment.AmountToMinorUnit(payment.FormatAmountForCurrency(p.Order.RefundAmount, currency), currency)
	if err != nil {
		refundedMinor = 0
	}
	return fmt.Sprintf("sub2api-refund-%d-%d-%d", p.OrderID, refundedMinor, refundMinor)
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

func (s *PaymentService) finishRefund(ctx context.Context, p *RefundPlan, resp *payment.RefundResponse) (*RefundResult, error) {
	if err := validateRefundProviderResponse(resp); err != nil {
		return s.handleGwFail(ctx, p, err)
	}
	switch strings.TrimSpace(resp.Status) {
	case payment.ProviderStatusSuccess, payment.ProviderStatusRefunded:
		return s.markRefundOk(ctx, p)
	case payment.ProviderStatusPending:
		return s.markRefundPending(ctx, p, resp)
	default:
		return s.handleGwFail(ctx, p, fmt.Errorf("payment refund returned unknown status: %s", strings.TrimSpace(resp.Status)))
	}
}

func (s *PaymentService) QueryAndFinalizeRefund(ctx context.Context, oid int64) (*RefundResult, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.Status != OrderStatusRefundPending {
		return nil, infraerrors.BadRequest("INVALID_STATUS", "only refund pending orders can be finalized")
	}

	prov, err := s.getRefundProvider(ctx, o)
	if err != nil {
		return nil, fmt.Errorf("get refund provider: %w", err)
	}
	queryProvider, ok := prov.(payment.RefundQueryProvider)
	if !ok {
		return nil, infraerrors.BadRequest("REFUND_QUERY_UNSUPPORTED", "this payment provider does not support refund status query; please verify manually")
	}

	pendingDetail, pendingAuditFound := s.latestRefundPendingDetail(ctx, oid)
	// 无法确定冻结持有金额时必须转人工，禁止按 held=0 自动终结：
	//  1) 审计缺失（旧版最佳努力写审计失败留下的在途单）——按 0 处理会漏回补/误退；
	//  2) 旧“回滚”语义格式（挂起时已退回余额，字段与新“冻结持有”语义不兼容）。
	if !pendingAuditFound || pendingDetail.isLegacyPendingFormat() {
		reason := "pending audit missing"
		if pendingAuditFound {
			reason = "pending order predates hold-frozen refund semantics"
		}
		s.writeAuditLog(ctx, oid, "REFUND_PENDING_LEGACY_MANUAL", "admin", map[string]any{
			"detail": reason + "; finalize manually",
		})
		return nil, infraerrors.BadRequest("REFUND_PENDING_LEGACY", "this pending refund cannot be finalized automatically ("+reason+"); please finalize it manually")
	}
	plan := s.refundFinalizePlan(o, pendingDetail)
	if err := s.claimPendingRefundFinalization(ctx, o); err != nil {
		return nil, err
	}
	finishProviderCall := servertiming.ObserveDependency(ctx, "payment")
	resp, err := queryProvider.QueryRefund(ctx, payment.RefundQueryRequest{
		TradeNo:   o.PaymentTradeNo,
		OrderID:   o.OutTradeNo,
		RefundID:  pendingDetail.RefundID,
		Amount:    formatGatewayRefundAmount(plan.GatewayAmount, o),
		RequestID: refundProviderRequestID(plan),
	})
	finishProviderCall()
	if err != nil {
		s.restorePendingRefundFinalization(ctx, o)
		return nil, fmt.Errorf("query refund: %w", err)
	}
	if err := validateRefundProviderResponse(resp); err != nil {
		return s.finalizePendingRefundFailed(ctx, o, pendingDetail, err)
	}

	// 余额/订阅扣减已在挂起期间冻结持有，终结成功不再重复扣减；plan.BalanceToDeduct
	// 与 SubDaysToDeduct 保持为 0（见 refundFinalizePlan）。终结失败才回补。
	switch strings.TrimSpace(resp.Status) {
	case payment.ProviderStatusSuccess, payment.ProviderStatusRefunded:
		if err := s.applyRefundFinalDeduction(ctx, plan); err != nil {
			s.restorePendingRefundFinalization(ctx, o)
			s.writeAuditLog(ctx, oid, "REFUND_FINAL_DEDUCTION_FAILED", "admin", map[string]any{
				"refundID": resp.RefundID,
				"detail":   psErrMsg(err),
			})
			return nil, err
		}
		result, err := s.markRefundOk(ctx, plan)
		if err == nil {
			return result, nil
		}
		rollbackOK := s.RollbackRefund(ctx, plan, err)
		if rollbackOK {
			s.restorePendingRefundFinalization(ctx, o)
		} else {
			now := time.Now()
			_, _ = s.entClient.PaymentOrder.UpdateOneID(o.ID).
				SetStatus(OrderStatusRefundFailed).
				SetFailedAt(now).
				SetFailedReason(psErrMsg(err)).
				Save(ctx)
		}
		s.writeAuditLog(ctx, oid, "REFUND_FINALIZE_PERSIST_FAILED", "admin", map[string]any{
			"refundID":           resp.RefundID,
			"detail":             psErrMsg(err),
			"rollbackOK":         rollbackOK,
			"balanceDeducted":    plan.BalanceToDeduct,
			"subscriptionDeduct": plan.SubDaysToDeduct,
		})
		return nil, err
	case payment.ProviderStatusPending:
		s.restorePendingRefundFinalization(ctx, o)
		s.writeAuditLog(ctx, oid, "REFUND_QUERY_PENDING", "admin", map[string]any{"refundID": resp.RefundID})
		return &RefundResult{Success: false, Warning: "gateway refund is still pending confirmation"}, nil
	default:
		return s.finalizePendingRefundFailed(ctx, o, pendingDetail, fmt.Errorf("payment refund returned unknown status: %s", strings.TrimSpace(resp.Status)))
	}
}

func (s *PaymentService) RetryRefundAffiliateRebateReversal(ctx context.Context, oid int64) error {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.Status != OrderStatusRefunded && o.Status != OrderStatusPartiallyRefunded {
		return infraerrors.BadRequest("INVALID_STATUS", "only refunded orders can retry affiliate rebate reversal")
	}
	if o.OrderType != payment.OrderTypeBalance || o.Amount <= 0 || o.RefundAmount <= 0 {
		return nil
	}
	if s == nil || s.affiliateService == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	s.writeAuditLog(ctx, oid, "AFFILIATE_REBATE_REVERSAL_RETRY", "admin", map[string]any{
		"totalRefunded": o.RefundAmount,
		"orderAmount":   o.Amount,
		"status":        o.Status,
	})
	s.reverseAffiliateRebateBestEffort(ctx, o, o.RefundAmount)
	return nil
}

func (s *PaymentService) claimPendingRefundFinalization(ctx context.Context, o *dbent.PaymentOrder) error {
	c, err := s.entClient.PaymentOrder.Update().
		Where(
			paymentorder.IDEQ(o.ID),
			paymentorder.StatusEQ(OrderStatusRefundPending),
			paymentorder.RefundAmountEQ(o.RefundAmount),
			paymentorder.RefundRequestedAmountEQ(o.RefundRequestedAmount),
		).
		SetStatus(OrderStatusRefunding).
		ClearFailedAt().
		ClearFailedReason().
		Save(ctx)
	if err != nil {
		return fmt.Errorf("claim refund finalization: %w", err)
	}
	if c == 0 {
		return infraerrors.Conflict("CONFLICT", "order refund state changed")
	}
	o.Status = OrderStatusRefunding
	return nil
}

func (s *PaymentService) restorePendingRefundFinalization(ctx context.Context, o *dbent.PaymentOrder) {
	_, _ = s.entClient.PaymentOrder.Update().
		Where(paymentorder.IDEQ(o.ID), paymentorder.StatusEQ(OrderStatusRefunding)).
		SetStatus(OrderStatusRefundPending).
		Save(ctx)
	o.Status = OrderStatusRefundPending
}

func (s *PaymentService) refundFinalizePlan(o *dbent.PaymentOrder, pendingDetail refundPendingAuditDetail) *RefundPlan {
	refundAmount := pendingDetail.RefundAmount
	if refundAmount <= 0 {
		refundAmount = o.RefundRequestedAmount
	}
	if refundAmount <= 0 {
		refundAmount = o.RefundAmount
	}
	reason := strings.TrimSpace(psStringValue(o.RefundReason))
	if reason == "" {
		reason = fmt.Sprintf("refund order:%d", o.ID)
	}
	return &RefundPlan{
		OrderID:       o.ID,
		Order:         o,
		RefundAmount:  refundAmount,
		GatewayAmount: calculateGatewayRefundAmount(o.Amount, o.PayAmount, refundAmount, PaymentOrderCurrency(o)),
		Reason:        reason,
		Force:         o.ForceRefund,
		DeductBalance: true,
		DeductionType: payment.DeductionTypeBalance,
		// 余额/订阅扣减已在挂起期间“冻结持有”，终结成功不再重复扣减（保持扣减即可）；
		// 终结失败由 retryPendingRefundDeductionRollback 按 balanceHeld/subDaysHeld 回补。
		BalanceToDeduct: 0,
	}
}

func (s *PaymentService) applyRefundFinalDeduction(ctx context.Context, p *RefundPlan) error {
	if p.DeductionType == payment.DeductionTypeBalance && p.BalanceToDeduct > 0 {
		if err := s.userRepo.DeductBalance(ctx, p.Order.UserID, p.BalanceToDeduct); err != nil {
			return fmt.Errorf("deduction: %w", err)
		}
		s.invalidateBalanceCacheBestEffort(ctx, p.Order.UserID)
		p.DeductionApplied = true
	}
	if p.DeductionType == payment.DeductionTypeSubscription && p.SubDaysToDeduct > 0 && p.SubscriptionID > 0 {
		if _, err := s.subscriptionSvc.ExtendSubscription(ctx, p.SubscriptionID, -p.SubDaysToDeduct); err != nil {
			if errors.Is(err, ErrAdjustWouldExpire) {
				if revokeErr := s.subscriptionSvc.RevokeSubscription(ctx, p.SubscriptionID); revokeErr != nil {
					return fmt.Errorf("revoke subscription: %w", revokeErr)
				}
			} else {
				return fmt.Errorf("deduct subscription days: %w", err)
			}
		}
		p.DeductionApplied = true
	}
	return nil
}

func (s *PaymentService) finalizeRefundFailed(ctx context.Context, o *dbent.PaymentOrder, gErr error) (*RefundResult, error) {
	now := time.Now()
	_, _ = s.entClient.PaymentOrder.UpdateOneID(o.ID).SetStatus(OrderStatusRefundFailed).SetFailedAt(now).SetFailedReason(psErrMsg(gErr)).Save(ctx)
	s.writeAuditLog(ctx, o.ID, "REFUND_FAILED", "admin", map[string]any{"detail": psErrMsg(gErr)})
	return &RefundResult{Success: false, Warning: "gateway refund failed: " + psErrMsg(gErr)}, nil
}

func (s *PaymentService) finalizePendingRefundFailed(ctx context.Context, o *dbent.PaymentOrder, pendingDetail refundPendingAuditDetail, gErr error) (*RefundResult, error) {
	if ok, warning := s.retryPendingRefundDeductionRollback(ctx, o, pendingDetail, gErr); !ok {
		s.restorePendingRefundFinalization(ctx, o)
		s.writeAuditLog(ctx, o.ID, "REFUND_FAILED_PENDING_MANUAL", "admin", map[string]any{
			"detail":          psErrMsg(gErr),
			"rollbackWarning": warning,
		})
		return &RefundResult{
			Success: false,
			Warning: "gateway refund failed: " + psErrMsg(gErr) + "; refund deduction rollback still failed: " + warning,
		}, nil
	}
	return s.finalizeRefundFailed(ctx, o, gErr)
}

func (s *PaymentService) retryPendingRefundDeductionRollback(ctx context.Context, o *dbent.PaymentOrder, pendingDetail refundPendingAuditDetail, gErr error) (bool, string) {
	// 挂起期间扣减为冻结持有，退款最终失败必须回补给用户，故无条件按 balanceHeld/subDaysHeld 回补。
	failures := make([]string, 0, 2)
	if pendingDetail.BalanceHeld > 0 {
		if s.userRepo == nil {
			failures = append(failures, "user repository unavailable")
		} else if err := restoreUserBalanceWithoutRecharge(ctx, s.userRepo, o.UserID, pendingDetail.BalanceHeld); err != nil {
			failures = append(failures, psErrMsg(err))
		} else {
			s.invalidateBalanceCacheBestEffort(ctx, o.UserID)
			s.writeAuditLog(ctx, o.ID, "REFUND_ROLLBACK_RETRIED", "admin", map[string]any{
				"gatewayError":    psErrMsg(gErr),
				"balanceRestored": pendingDetail.BalanceHeld,
			})
		}
	}
	if pendingDetail.SubDaysHeld > 0 {
		if s.subscriptionSvc == nil {
			failures = append(failures, "subscription service unavailable")
		} else if pendingDetail.SubscriptionID <= 0 {
			failures = append(failures, "subscription id unavailable")
		} else if _, err := s.subscriptionSvc.ExtendSubscription(ctx, pendingDetail.SubscriptionID, pendingDetail.SubDaysHeld); err != nil {
			failures = append(failures, psErrMsg(err))
		} else {
			s.writeAuditLog(ctx, o.ID, "REFUND_ROLLBACK_RETRIED", "admin", map[string]any{
				"gatewayError": psErrMsg(gErr),
				"subDays":      pendingDetail.SubDaysHeld,
				"subscription": pendingDetail.SubscriptionID,
			})
		}
	}
	if len(failures) > 0 {
		warning := strings.Join(failures, "; ")
		s.writeAuditLog(ctx, o.ID, "REFUND_ROLLBACK_RETRY_FAILED", "admin", map[string]any{
			"gatewayError": psErrMsg(gErr),
			"detail":       warning,
		})
		return false, warning
	}
	return true, ""
}

type refundPendingAuditDetail struct {
	RefundID       string  `json:"refundID"`
	RefundAmount   float64 `json:"refundAmount"`
	BalanceHeld    float64 `json:"balanceHeld"`
	SubDaysHeld    int     `json:"subDaysHeld"`
	SubscriptionID int64   `json:"subscriptionID"`
	// LegacyDeductionRollbackOK 仅用于识别“冻结持有”语义上线前创建的旧挂起审计
	// （旧语义在挂起时回滚了扣减，字段含义与新语义不兼容）。旧审计带该键→指针非 nil；
	// 新审计不写该键→nil。非 nil 时终结逻辑拒绝自动处理，转人工，避免误退/漏退。
	LegacyDeductionRollbackOK *bool `json:"deductionRollbackOK"`
}

// isLegacyPendingFormat 判断挂起审计是否为旧“回滚”语义格式（需人工终结）。
func (d refundPendingAuditDetail) isLegacyPendingFormat() bool {
	return d.LegacyDeductionRollbackOK != nil
}

// latestRefundPendingDetail 返回最近一条 REFUND_PENDING 审计详情，第二个返回值表示
// 该审计是否存在。审计缺失时无法得知冻结持有的金额，终结逻辑必须转人工而非按 0 处理。
func (s *PaymentService) latestRefundPendingDetail(ctx context.Context, oid int64) (refundPendingAuditDetail, bool) {
	logEntry, err := s.entClient.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(oid, 10)), paymentauditlog.ActionEQ("REFUND_PENDING")).
		Order(paymentauditlog.ByCreatedAt(sql.OrderDesc())).
		First(ctx)
	if err != nil || logEntry == nil {
		return refundPendingAuditDetail{}, false
	}
	detail := refundPendingAuditDetail{}
	// 审计存在但 Detail 无法解析（损坏）时视作不可用：无法得知冻结持有金额，转人工。
	if err := json.Unmarshal([]byte(logEntry.Detail), &detail); err != nil {
		return refundPendingAuditDetail{}, false
	}
	detail.RefundID = strings.TrimSpace(detail.RefundID)
	return detail, true
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
	// Balance top-ups and subscription purchases both accrue invite rebates on
	// fulfillment; refunds of either order type must claw the proportional rebate back.
	if (o.OrderType != payment.OrderTypeBalance && o.OrderType != payment.OrderTypeSubscription) || o.Amount <= 0 || totalRefunded <= 0 {
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

func (s *PaymentService) markRefundPending(ctx context.Context, p *RefundPlan, resp *payment.RefundResponse) (*RefundResult, error) {
	// 挂起期间不回滚扣减：保持余额/订阅扣减为“冻结持有”状态，避免网关退款确认前
	// 用户消费掉被退还的余额造成套利。终结成功时扣减保留（不重复扣），终结失败时由
	// retryPendingRefundDeductionRollback 按 balanceHeld/subDaysHeld 回补。
	//
	// balanceHeld/subDaysHeld 是终结失败时回补的唯一依据，必须先可靠持久化审计、
	// 再让订单进入 refund_pending 状态：这样“订单处于挂起态”蕴含“持有金额已可回补”，
	// 避免审计丢失导致漏回补资损（审计写失败则不进入挂起态，余额保持持有，返回错误）。
	detail := map[string]any{
		"refundID":       refundResponseID(resp),
		"refundAmount":   p.RefundAmount,
		"reason":         p.Reason,
		"force":          p.Force,
		"balanceHeld":    p.BalanceToDeduct,
		"subDaysHeld":    p.SubDaysToDeduct,
		"subscriptionID": p.SubscriptionID,
	}

	rollbackPendingPersistFailure := func(persistErr error) (*RefundResult, error) {
		s.RollbackRefund(ctx, p, persistErr)
		s.restoreStatus(ctx, p)
		return nil, persistErr
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return rollbackPendingPersistFailure(fmt.Errorf("begin refund pending tx: %w", err))
	}
	txCtx := dbent.NewTxContext(ctx, tx)
	if err := s.writeAuditLogErrWithClient(txCtx, tx.Client(), p.OrderID, "REFUND_PENDING", "admin", detail); err != nil {
		_ = tx.Rollback()
		return rollbackPendingPersistFailure(fmt.Errorf("persist refund pending audit: %w", err))
	}

	_, err = tx.PaymentOrder.UpdateOneID(p.OrderID).
		SetStatus(OrderStatusRefundPending).
		SetRefundAmount(p.Order.RefundAmount).
		SetRefundRequestedAmount(p.RefundAmount).
		SetRefundReason(p.Reason).
		ClearRefundAt().
		SetForceRefund(p.Force).
		ClearFailedAt().
		ClearFailedReason().
		Save(txCtx)
	if err != nil {
		_ = tx.Rollback()
		return rollbackPendingPersistFailure(fmt.Errorf("mark refund pending: %w", err))
	}
	if err := tx.Commit(); err != nil {
		return rollbackPendingPersistFailure(fmt.Errorf("commit refund pending: %w", err))
	}

	return &RefundResult{Success: false, Warning: "gateway refund is pending confirmation"}, nil
}

func refundResponseID(resp *payment.RefundResponse) string {
	if resp == nil {
		return ""
	}
	return strings.TrimSpace(resp.RefundID)
}

func (s *PaymentService) RollbackRefund(ctx context.Context, p *RefundPlan, gErr error) bool {
	if p != nil && !p.DeductionApplied {
		return true
	}
	if p.DeductionType == payment.DeductionTypeBalance && p.BalanceToDeduct > 0 {
		err := restoreUserBalanceWithoutRecharge(ctx, s.userRepo, p.Order.UserID, p.BalanceToDeduct)
		if err != nil {
			slog.Error("[CRITICAL] rollback failed", "orderID", p.OrderID, "amount", p.BalanceToDeduct, "error", err)
			s.writeAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED", "admin", map[string]any{"gatewayError": psErrMsg(gErr), "rollbackError": psErrMsg(err), "balanceDeducted": p.BalanceToDeduct})
			return false
		}
		s.invalidateBalanceCacheBestEffort(ctx, p.Order.UserID)
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

func restoreUserBalanceWithoutRecharge(ctx context.Context, userRepo UserRepository, userID int64, amount float64) error {
	return userRepo.AddBalanceWithoutRecharge(ctx, userID, amount)
}

func (s *PaymentService) restoreStatus(ctx context.Context, p *RefundPlan) {
	rs := OrderStatusCompleted
	if p.OriginalStatus == OrderStatusRefundRequested || p.OriginalStatus == OrderStatusPartiallyRefunded || p.OriginalStatus == OrderStatusRefundFailed {
		rs = p.OriginalStatus
	}
	_, _ = s.entClient.PaymentOrder.UpdateOneID(p.OrderID).SetStatus(rs).Save(ctx)
}
