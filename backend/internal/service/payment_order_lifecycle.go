package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/provider"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// --- Cancel & Expire ---

// Cancel rate limit configuration constants.
const (
	rateLimitUnitDay                = "day"
	rateLimitUnitMinute             = "minute"
	rateLimitUnitHour               = "hour"
	rateLimitModeFixed              = "fixed"
	checkPaidResultAlreadyPaid      = "already_paid"
	checkPaidResultCancelled        = "cancelled"
	checkPaidResultAlreadyProcessed = "already_processed"
	pendingWxpayReconcileLimit      = 20
)

func (s *PaymentService) checkCancelRateLimit(ctx context.Context, userID int64, cfg *PaymentConfig) error {
	if !cfg.CancelRateLimitEnabled || cfg.CancelRateLimitMax <= 0 {
		return nil
	}
	windowStart := cancelRateLimitWindowStart(cfg)
	operator := fmt.Sprintf("user:%d", userID)
	count, err := s.entClient.PaymentAuditLog.Query().
		Where(
			paymentauditlog.ActionEQ("ORDER_CANCELLED"),
			paymentauditlog.OperatorEQ(operator),
			paymentauditlog.CreatedAtGTE(windowStart),
		).Count(ctx)
	if err != nil {
		slog.Error("check cancel rate limit failed", "userID", userID, "error", err)
		return nil // fail open
	}
	if count >= cfg.CancelRateLimitMax {
		return infraerrors.TooManyRequests("CANCEL_RATE_LIMITED", "cancel rate limited").
			WithMetadata(map[string]string{
				"max":    strconv.Itoa(cfg.CancelRateLimitMax),
				"window": strconv.Itoa(cfg.CancelRateLimitWindow),
				"unit":   cfg.CancelRateLimitUnit,
			})
	}
	return nil
}

func cancelRateLimitWindowStart(cfg *PaymentConfig) time.Time {
	now := time.Now()
	w := cfg.CancelRateLimitWindow
	if w <= 0 {
		w = 1
	}
	unit := cfg.CancelRateLimitUnit
	if unit == "" {
		unit = rateLimitUnitDay
	}
	if cfg.CancelRateLimitMode == rateLimitModeFixed {
		switch unit {
		case rateLimitUnitMinute:
			t := now.Truncate(time.Minute)
			return t.Add(-time.Duration(w-1) * time.Minute)
		case rateLimitUnitDay:
			y, m, d := now.Date()
			t := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
			return t.AddDate(0, 0, -(w - 1))
		default: // hour
			t := now.Truncate(time.Hour)
			return t.Add(-time.Duration(w-1) * time.Hour)
		}
	}
	// rolling window
	switch unit {
	case rateLimitUnitMinute:
		return now.Add(-time.Duration(w) * time.Minute)
	case rateLimitUnitDay:
		return now.AddDate(0, 0, -w)
	default: // hour
		return now.Add(-time.Duration(w) * time.Hour)
	}
}

func (s *PaymentService) CancelOrder(ctx context.Context, orderID, userID int64) (string, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, orderID)
	if err != nil {
		return "", infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.UserID != userID {
		return "", infraerrors.Forbidden("FORBIDDEN", "no permission for this order")
	}
	if o.Status != OrderStatusPending {
		return "", infraerrors.BadRequest("INVALID_STATUS", "order cannot be cancelled in current status")
	}
	return cancelOutcomeForCaller(s.cancelCore(ctx, o, OrderStatusCancelled, fmt.Sprintf("user:%d", userID), "user cancelled order"))
}

func (s *PaymentService) AdminCancelOrder(ctx context.Context, orderID int64) (string, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, orderID)
	if err != nil {
		return "", infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.Status != OrderStatusPending {
		return "", infraerrors.BadRequest("INVALID_STATUS", "order cannot be cancelled in current status")
	}
	return cancelOutcomeForCaller(s.cancelCore(ctx, o, OrderStatusCancelled, "admin", "admin cancelled order"))
}

// cancelOutcomeForCaller maps the cancelCore result string into the response
// contract used by the user/admin cancel handlers. A discovered-paid outcome is
// surfaced as a 409 Conflict so callers get an accurate "already paid, cannot
// cancel" signal instead of a misleading success message.
func cancelOutcomeForCaller(result string, err error) (string, error) {
	if err != nil {
		return "", err
	}
	if result == checkPaidResultAlreadyPaid {
		return "", infraerrors.Conflict("ORDER_ALREADY_PAID", "order was already paid and cannot be cancelled")
	}
	return result, nil
}

func (s *PaymentService) cancelCore(ctx context.Context, o *dbent.PaymentOrder, fs, op, ad string) (string, error) {
	if o.PaymentTradeNo != "" || o.PaymentType != "" {
		result, err := s.checkPaid(ctx, o)
		if err != nil {
			return "", err
		}
		if result == checkPaidResultAlreadyPaid {
			return checkPaidResultAlreadyPaid, nil
		}
	}
	c, err := s.entClient.PaymentOrder.Update().Where(paymentorder.IDEQ(o.ID), paymentorder.StatusEQ(OrderStatusPending)).SetStatus(fs).Save(ctx)
	if err != nil {
		return "", fmt.Errorf("update order status: %w", err)
	}
	if c == 0 {
		// CAS missed: the order is no longer PENDING. A concurrent webhook (or
		// another cancel/expire) won the race. Report the order's real current
		// state instead of falsely claiming a successful cancellation, and do
		// not attempt to cancel the upstream order — it may have been paid.
		return s.cancelCASMissResult(ctx, o.ID), nil
	}
	auditAction := "ORDER_CANCELLED"
	if fs == OrderStatusExpired {
		auditAction = "ORDER_EXPIRED"
	}
	s.writeAuditLog(ctx, o.ID, auditAction, op, map[string]any{"detail": ad})
	if o.PaymentTradeNo != "" || o.PaymentType != "" {
		if err := s.cancelUnpaidUpstreamOrder(ctx, o); err != nil {
			slog.Warn("cancel upstream payment failed", "orderID", o.ID, "error", err)
		}
	}
	return checkPaidResultCancelled, nil
}

// cancelCASMissResult reloads the order whose pending-guarded status update
// matched zero rows and classifies the outcome the caller should report:
//   - a paid/fulfilled state -> checkPaidResultAlreadyPaid (cannot cancel)
//   - an already-terminal state (cancelled/expired/failed) -> already_processed (no-op)
//   - anything else (including a failed reload) -> already_processed as a safe default
func (s *PaymentService) cancelCASMissResult(ctx context.Context, orderID int64) string {
	current, err := s.entClient.PaymentOrder.Get(ctx, orderID)
	if err != nil {
		slog.Warn("reload order after cancel CAS miss failed", "orderID", orderID, "error", err)
		return checkPaidResultAlreadyProcessed
	}
	switch current.Status {
	case OrderStatusPaid, OrderStatusRecharging, OrderStatusCompleted,
		OrderStatusPartiallyRefunded, OrderStatusRefunded:
		return checkPaidResultAlreadyPaid
	default:
		// Cancelled / Expired / Failed (or any other non-pending state): the
		// order is already in a terminal state, so the cancel is a no-op.
		return checkPaidResultAlreadyProcessed
	}
}

func (s *PaymentService) checkPaid(ctx context.Context, o *dbent.PaymentOrder) (string, error) {
	prov, err := s.getOrderProvider(ctx, o)
	if err != nil {
		return "", nil
	}
	queryRef := paymentOrderQueryReference(o, prov)
	if queryRef == "" {
		return "", nil
	}
	resp, err := prov.QueryOrder(ctx, queryRef)
	if err != nil {
		slog.Warn("query upstream failed", "orderID", o.ID, "error", err)
		return "", nil
	}
	if resp.Status == payment.ProviderStatusPaid {
		if !isValidProviderAmount(resp.Amount) {
			s.writeAuditLog(ctx, o.ID, "PAYMENT_INVALID_AMOUNT", prov.ProviderKey(), map[string]any{
				"expected": o.PayAmount,
				"paid":     resp.Amount,
				"tradeNo":  resp.TradeNo,
				"queryRef": queryRef,
			})
			slog.Warn("query upstream returned invalid paid amount", "orderID", o.ID, "queryRef", queryRef, "paid", resp.Amount)
			retriedResp, retryOK := requeryPaidOrderOnce(ctx, prov, queryRef)
			if !retryOK {
				return "", nil
			}
			resp = retriedResp
		}
		if o.Status == OrderStatusFailed {
			notificationTradeNo := o.PaymentTradeNo
			if upstreamTradeNo := strings.TrimSpace(resp.TradeNo); paymentOrderShouldPersistUpstreamTradeNo(queryRef, upstreamTradeNo, notificationTradeNo) {
				notificationTradeNo = upstreamTradeNo
			}
			if err := s.HandlePaymentNotification(ctx, &payment.PaymentNotification{TradeNo: notificationTradeNo, OrderID: o.OutTradeNo, Amount: resp.Amount, Status: payment.ProviderStatusSuccess, Metadata: resp.Metadata}, prov.ProviderKey()); err != nil {
				slog.Error("fulfillment failed during checkPaid", "orderID", o.ID, "error", err)
				return "", fmt.Errorf("local payment confirmation failed: %w", err)
			}
			return checkPaidResultAlreadyPaid, nil
		}
		notificationTradeNo := o.PaymentTradeNo
		if upstreamTradeNo := strings.TrimSpace(resp.TradeNo); paymentOrderShouldPersistUpstreamTradeNo(queryRef, upstreamTradeNo, notificationTradeNo) {
			if _, updateErr := s.entClient.PaymentOrder.Update().
				Where(paymentorder.IDEQ(o.ID)).
				SetPaymentTradeNo(upstreamTradeNo).
				Save(ctx); updateErr != nil {
				slog.Error("persist upstream trade no during checkPaid failed", "orderID", o.ID, "tradeNo", upstreamTradeNo, "error", updateErr)
			} else {
				o.PaymentTradeNo = upstreamTradeNo
			}
			notificationTradeNo = upstreamTradeNo
		}
		if err := s.HandlePaymentNotification(ctx, &payment.PaymentNotification{TradeNo: notificationTradeNo, OrderID: o.OutTradeNo, Amount: resp.Amount, Status: payment.ProviderStatusSuccess, Metadata: resp.Metadata}, prov.ProviderKey()); err != nil {
			slog.Error("fulfillment failed during checkPaid", "orderID", o.ID, "error", err)
			return "", fmt.Errorf("local payment confirmation failed: %w", err)
		}
		// Still return already_paid — order was paid, fulfillment can be retried
		return checkPaidResultAlreadyPaid, nil
	}
	return "", nil
}

func (s *PaymentService) cancelUnpaidUpstreamOrder(ctx context.Context, o *dbent.PaymentOrder) error {
	prov, err := s.getOrderProvider(ctx, o)
	if err != nil {
		return err
	}
	queryRef := paymentOrderQueryReference(o, prov)
	if queryRef == "" {
		return nil
	}
	if cp, ok := prov.(payment.CancelableProvider); ok {
		if err := cp.CancelPayment(ctx, queryRef); err != nil {
			return fmt.Errorf("cancel upstream payment %s: %w", queryRef, err)
		}
	}
	return nil
}

func (s *PaymentService) reconcilePaid(ctx context.Context, o *dbent.PaymentOrder) (string, error) {
	prov, err := s.getOrderProvider(ctx, o)
	if err != nil {
		return "", nil
	}
	queryRef := paymentOrderQueryReference(o, prov)
	if queryRef == "" {
		return "", nil
	}
	resp, err := prov.QueryOrder(ctx, queryRef)
	if err != nil {
		slog.Warn("query upstream failed", "orderID", o.ID, "error", err)
		return "", nil
	}
	if resp.Status == payment.ProviderStatusPaid {
		if !isValidProviderAmount(resp.Amount) {
			s.writeAuditLog(ctx, o.ID, "PAYMENT_INVALID_AMOUNT", prov.ProviderKey(), map[string]any{
				"expected": o.PayAmount,
				"paid":     resp.Amount,
				"tradeNo":  resp.TradeNo,
				"queryRef": queryRef,
			})
			slog.Warn("query upstream returned invalid paid amount", "orderID", o.ID, "queryRef", queryRef, "paid", resp.Amount)
			retriedResp, retryOK := requeryPaidOrderOnce(ctx, prov, queryRef)
			if !retryOK {
				return "", nil
			}
			resp = retriedResp
		}
		if o.Status == OrderStatusFailed {
			notificationTradeNo := o.PaymentTradeNo
			if upstreamTradeNo := strings.TrimSpace(resp.TradeNo); paymentOrderShouldPersistUpstreamTradeNo(queryRef, upstreamTradeNo, notificationTradeNo) {
				notificationTradeNo = upstreamTradeNo
			}
			if err := s.HandlePaymentNotification(ctx, &payment.PaymentNotification{TradeNo: notificationTradeNo, OrderID: o.OutTradeNo, Amount: resp.Amount, Status: payment.ProviderStatusSuccess, Metadata: resp.Metadata}, prov.ProviderKey()); err != nil {
				slog.Error("fulfillment failed during reconcilePaid", "orderID", o.ID, "error", err)
				return "", fmt.Errorf("local payment confirmation failed: %w", err)
			}
			return checkPaidResultAlreadyPaid, nil
		}
		notificationTradeNo := o.PaymentTradeNo
		if upstreamTradeNo := strings.TrimSpace(resp.TradeNo); paymentOrderShouldPersistUpstreamTradeNo(queryRef, upstreamTradeNo, notificationTradeNo) {
			if _, updateErr := s.entClient.PaymentOrder.Update().
				Where(paymentorder.IDEQ(o.ID)).
				SetPaymentTradeNo(upstreamTradeNo).
				Save(ctx); updateErr != nil {
				slog.Error("persist upstream trade no during reconcilePaid failed", "orderID", o.ID, "tradeNo", upstreamTradeNo, "error", updateErr)
			} else {
				o.PaymentTradeNo = upstreamTradeNo
			}
			notificationTradeNo = upstreamTradeNo
		}
		if err := s.HandlePaymentNotification(ctx, &payment.PaymentNotification{TradeNo: notificationTradeNo, OrderID: o.OutTradeNo, Amount: resp.Amount, Status: payment.ProviderStatusSuccess, Metadata: resp.Metadata}, prov.ProviderKey()); err != nil {
			slog.Error("fulfillment failed during checkPaid", "orderID", o.ID, "error", err)
			return "", fmt.Errorf("local payment confirmation failed: %w", err)
		}
		return checkPaidResultAlreadyPaid, nil
	}
	return "", nil
}

func requeryPaidOrderOnce(ctx context.Context, prov payment.Provider, queryRef string) (*payment.QueryOrderResponse, bool) {
	if prov == nil || strings.TrimSpace(queryRef) == "" {
		return nil, false
	}
	resp, err := prov.QueryOrder(ctx, queryRef)
	if err != nil {
		slog.Warn("query upstream retry failed", "queryRef", queryRef, "error", err)
		return nil, false
	}
	if resp == nil || resp.Status != payment.ProviderStatusPaid || !isValidProviderAmount(resp.Amount) {
		return nil, false
	}
	return resp, true
}

func paymentOrderQueryReference(order *dbent.PaymentOrder, prov payment.Provider) string {
	if order == nil {
		return ""
	}

	providerKey := ""
	if prov != nil {
		providerKey = strings.TrimSpace(prov.ProviderKey())
	}
	if providerKey == "" {
		if snapshot := psOrderProviderSnapshot(order); snapshot != nil {
			providerKey = strings.TrimSpace(snapshot.ProviderKey)
		}
	}
	if providerKey == "" {
		providerKey = strings.TrimSpace(psStringValue(order.ProviderKey))
	}
	if providerKey == "" {
		providerKey = strings.TrimSpace(order.PaymentType)
	}

	switch payment.GetBasePaymentType(providerKey) {
	case payment.TypeAlipay, payment.TypeEasyPay, payment.TypeWxpay:
		return strings.TrimSpace(order.OutTradeNo)
	default:
		if tradeNo := strings.TrimSpace(order.PaymentTradeNo); tradeNo != "" {
			return tradeNo
		}
		return strings.TrimSpace(order.OutTradeNo)
	}
}

func paymentOrderShouldPersistUpstreamTradeNo(queryRef, upstreamTradeNo, currentTradeNo string) bool {
	upstreamTradeNo = strings.TrimSpace(upstreamTradeNo)
	if upstreamTradeNo == "" {
		return false
	}
	if strings.EqualFold(upstreamTradeNo, strings.TrimSpace(currentTradeNo)) {
		return false
	}
	if strings.EqualFold(upstreamTradeNo, strings.TrimSpace(queryRef)) {
		return false
	}
	return true
}

func (s *PaymentService) markFailedOrderPaidAndReload(ctx context.Context, oid int64, tradeNo string, paid float64) (*dbent.PaymentOrder, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return nil, fmt.Errorf("get failed order: %w", err)
	}
	if o.Status != OrderStatusFailed {
		return o, nil
	}
	if !isValidProviderAmount(paid) {
		s.writeAuditLog(ctx, oid, "PAYMENT_INVALID_AMOUNT", "system", map[string]any{
			"expected": o.PayAmount,
			"paid":     paid,
			"tradeNo":  strings.TrimSpace(tradeNo),
		})
		return nil, fmt.Errorf("invalid paid amount from provider: %v", paid)
	}
	if !amountEqualForCurrency(paid, o.PayAmount, PaymentOrderCurrency(o)) {
		s.writeAuditLog(ctx, oid, "PAYMENT_AMOUNT_MISMATCH", "system", map[string]any{
			"expected": o.PayAmount,
			"paid":     paid,
			"tradeNo":  strings.TrimSpace(tradeNo),
		})
		return nil, fmt.Errorf("amount mismatch: expected %s, got %s", strconv.FormatFloat(o.PayAmount, 'f', -1, 64), strconv.FormatFloat(paid, 'f', -1, 64))
	}

	now := time.Now()
	update := s.entClient.PaymentOrder.Update().
		Where(paymentorder.IDEQ(oid), paymentorder.StatusEQ(OrderStatusFailed)).
		SetStatus(OrderStatusPaid).
		SetPayAmount(paid).
		ClearFailedAt().
		ClearFailedReason()
	if strings.TrimSpace(tradeNo) != "" {
		update = update.SetPaymentTradeNo(strings.TrimSpace(tradeNo))
	}
	if o.PaidAt == nil {
		update = update.SetPaidAt(now)
	}
	updated, err := update.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("backfill failed order paid metadata: %w", err)
	}
	if updated > 0 {
		s.writeAuditLog(ctx, oid, "ORDER_PAID", "system", map[string]any{
			"tradeNo":         strings.TrimSpace(tradeNo),
			"paidAmount":      paid,
			"previous_status": OrderStatusFailed,
			"reason":          "upstream reconciliation recovered a failed order before fulfillment",
		})
	}
	reloaded, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return nil, fmt.Errorf("reload recovered order: %w", err)
	}
	return reloaded, nil
}

func paymentOrderStatusAllowsPaidReconciliation(status string) bool {
	switch status {
	case OrderStatusPending, OrderStatusExpired, OrderStatusFailed, OrderStatusCancelled:
		return true
	default:
		return false
	}
}

func paymentOrderStatusAllowsPublicPaidReconciliation(status string) bool {
	switch status {
	case OrderStatusExpired, OrderStatusFailed, OrderStatusCancelled:
		return true
	default:
		return false
	}
}

// VerifyOrderByOutTradeNo actively queries the upstream provider to check
// if a payment was made, and processes it if so. This handles the case where
// the provider's notify callback was missed (e.g. EasyPay popup mode).
func (s *PaymentService) VerifyOrderByOutTradeNo(ctx context.Context, outTradeNo string, userID int64) (*dbent.PaymentOrder, error) {
	outTradeNo, err := normalizeOrderLookupOutTradeNo(outTradeNo)
	if err != nil {
		return nil, err
	}
	o, err := s.entClient.PaymentOrder.Query().
		Where(paymentorder.OutTradeNo(outTradeNo)).
		Only(ctx)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.UserID != userID {
		return nil, infraerrors.Forbidden("FORBIDDEN", "no permission for this order")
	}
	if paymentOrderStatusAllowsPaidReconciliation(o.Status) {
		result, err := s.checkPaid(ctx, o)
		if err != nil {
			return nil, err
		}
		if result == checkPaidResultAlreadyPaid {
			// Reload order to get updated status
			o, err = s.entClient.PaymentOrder.Get(ctx, o.ID)
			if err != nil {
				return nil, fmt.Errorf("reload order: %w", err)
			}
		}
	}
	return o, nil
}

// VerifyOrderPublic returns the public order state and preserves the legacy
// anonymous reconciliation behavior for older result pages that only carry
// out_trade_no. Signed resume-token recovery remains the stricter preferred path.
func (s *PaymentService) VerifyOrderPublic(ctx context.Context, outTradeNo string) (*dbent.PaymentOrder, error) {
	outTradeNo, err := normalizeOrderLookupOutTradeNo(outTradeNo)
	if err != nil {
		return nil, err
	}
	o, err := s.entClient.PaymentOrder.Query().
		Where(paymentorder.OutTradeNo(outTradeNo)).
		Only(ctx)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if paymentOrderStatusAllowsPublicPaidReconciliation(o.Status) {
		result, err := s.checkPaid(ctx, o)
		if err != nil {
			return nil, err
		}
		if result == checkPaidResultAlreadyPaid {
			o, err = s.entClient.PaymentOrder.Get(ctx, o.ID)
			if err != nil {
				return nil, fmt.Errorf("reload order after public verify reconciliation: %w", err)
			}
		}
	}
	return o, nil
}

func normalizeOrderLookupOutTradeNo(raw string) (string, error) {
	outTradeNo := strings.TrimSpace(raw)
	if outTradeNo == "" {
		return "", infraerrors.BadRequest("INVALID_OUT_TRADE_NO", "out_trade_no is required")
	}
	if len(outTradeNo) > 64 {
		return "", infraerrors.BadRequest("INVALID_OUT_TRADE_NO", "out_trade_no is invalid")
	}
	for _, ch := range outTradeNo {
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch >= '0' && ch <= '9':
		case ch == '_' || ch == '-':
		default:
			return "", infraerrors.BadRequest("INVALID_OUT_TRADE_NO", "out_trade_no is invalid")
		}
	}
	return outTradeNo, nil
}

// ReconcilePendingWxpayOrders actively checks recent pending WeChat orders so
// missed provider notifications do not wait until order expiry to fulfill.
func (s *PaymentService) ReconcilePendingWxpayOrders(ctx context.Context) (int, error) {
	now := time.Now()
	orders, err := s.entClient.PaymentOrder.Query().
		Where(
			paymentorder.StatusEQ(OrderStatusPending),
			paymentorder.ExpiresAtGT(now),
			paymentorder.Or(
				paymentorder.PaymentTypeEQ(payment.TypeWxpay),
				paymentorder.PaymentTypeHasPrefix(payment.TypeWxpay+"_"),
				paymentorder.ProviderKeyEQ(payment.TypeWxpay),
				paymentorder.ProviderKeyHasPrefix(payment.TypeWxpay+"_"),
			),
		).
		Order(dbent.Asc(paymentorder.FieldCreatedAt)).
		Limit(pendingWxpayReconcileLimit).
		All(ctx)
	if err != nil {
		return 0, fmt.Errorf("query pending wxpay orders: %w", err)
	}

	recovered := 0
	var firstErr error
	for _, order := range orders {
		result, err := s.reconcilePaid(ctx, order)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if result == checkPaidResultAlreadyPaid {
			recovered++
		}
	}
	return recovered, firstErr
}

func (s *PaymentService) ExpireTimedOutOrders(ctx context.Context) (int, error) {
	now := time.Now()
	orders, err := s.entClient.PaymentOrder.Query().Where(paymentorder.StatusEQ(OrderStatusPending), paymentorder.ExpiresAtLTE(now)).All(ctx)
	if err != nil {
		return 0, fmt.Errorf("query expired: %w", err)
	}
	n := 0
	var firstErr error
	for _, o := range orders {
		// Check upstream payment status before expiring — the user may have
		// paid just before timeout and the webhook hasn't arrived yet.
		outcome, err := s.cancelCore(ctx, o, OrderStatusExpired, "system", "order expired")
		if err != nil && firstErr == nil {
			firstErr = err
		}
		if outcome == checkPaidResultAlreadyPaid {
			slog.Info("order was paid during expiry", "orderID", o.ID)
			continue
		}
		if outcome != "" {
			n++
		}
	}
	return n, firstErr
}

// getOrderProvider creates a provider using the order's original instance config.
// Falls back to registry lookup if instance ID is missing (legacy orders).
func (s *PaymentService) getOrderProvider(ctx context.Context, o *dbent.PaymentOrder) (payment.Provider, error) {
	inst, err := s.getOrderProviderInstance(ctx, o)
	if err != nil {
		return nil, fmt.Errorf("load order provider instance: %w", err)
	}
	if inst != nil {
		return s.createProviderFromInstance(ctx, inst)
	}
	if !paymentOrderAllowsRegistryFallback(o) {
		return nil, fmt.Errorf("order %d provider instance is unresolved", o.ID)
	}
	providerKey := paymentOrderFallbackProviderKey(s.registry, o)
	if providerKey == "" {
		return nil, fmt.Errorf("order %d provider fallback key is missing", o.ID)
	}
	if !s.webhookRegistryFallbackAllowed(ctx, providerKey) {
		return nil, fmt.Errorf("order %d provider fallback is ambiguous for %s", o.ID, providerKey)
	}
	s.EnsureProviders(ctx)
	return s.registry.GetProvider(o.PaymentType)
}

func paymentOrderAllowsRegistryFallback(order *dbent.PaymentOrder) bool {
	if order == nil {
		return false
	}
	if psOrderProviderSnapshot(order) != nil {
		return false
	}
	if strings.TrimSpace(psStringValue(order.ProviderInstanceID)) != "" {
		return false
	}
	if strings.TrimSpace(psStringValue(order.ProviderKey)) != "" {
		return false
	}
	return true
}

func paymentOrderFallbackProviderKey(registry *payment.Registry, order *dbent.PaymentOrder) string {
	if order == nil {
		return ""
	}
	if registry != nil {
		if key := strings.TrimSpace(registry.GetProviderKey(payment.PaymentType(order.PaymentType))); key != "" {
			return key
		}
	}
	return strings.TrimSpace(payment.GetBasePaymentType(strings.TrimSpace(order.PaymentType)))
}

func (s *PaymentService) createProviderFromInstance(ctx context.Context, inst *dbent.PaymentProviderInstance) (payment.Provider, error) {
	if inst == nil {
		return nil, fmt.Errorf("payment provider instance is missing")
	}

	cfg, err := s.loadBalancer.GetInstanceConfig(ctx, int64(inst.ID))
	if err != nil {
		return nil, fmt.Errorf("load provider instance config: %w", err)
	}
	if inst.PaymentMode != "" {
		cfg["paymentMode"] = inst.PaymentMode
	}

	instID := strconv.FormatInt(int64(inst.ID), 10)
	prov, err := provider.CreateProvider(inst.ProviderKey, instID, cfg)
	if err != nil {
		return nil, fmt.Errorf("create provider from instance: %w", err)
	}
	return prov, nil
}

func psStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
