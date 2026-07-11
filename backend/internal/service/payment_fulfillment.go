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

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// ErrOrderNotFound is returned by HandlePaymentNotification when the webhook
// references an out_trade_no that does not exist in our DB. Callers (webhook
// handlers) should treat this as a terminal, non-retryable condition and still
// respond with a 2xx success to the provider — otherwise the provider will keep
// retrying forever (e.g. when a foreign environment's webhook endpoint is
// misconfigured to point at us, or when our orders table has been wiped).
var ErrOrderNotFound = errors.New("payment order not found")

const paymentFulfillmentLeaseDuration = 5 * time.Minute

type paymentFulfillmentLease struct {
	version time.Time
}

// --- Payment Notification & Fulfillment ---

func (s *PaymentService) HandlePaymentNotification(ctx context.Context, n *payment.PaymentNotification, pk string) error {
	if n.Status != payment.NotificationStatusSuccess {
		return nil
	}
	// Look up order by out_trade_no (the external order ID we sent to the provider)
	order, err := s.entClient.PaymentOrder.Query().Where(paymentorder.OutTradeNo(n.OrderID)).Only(ctx)
	if err != nil {
		// Fallback only for true legacy "sub2_N" DB-ID payloads when the
		// current out_trade_no lookup genuinely did not find an order.
		if oid, ok := parseLegacyPaymentOrderID(n.OrderID, err); ok {
			return s.confirmPayment(ctx, oid, n.TradeNo, n.Amount, pk, n.Metadata)
		}
		if dbent.IsNotFound(err) {
			return fmt.Errorf("%w: out_trade_no=%s", ErrOrderNotFound, n.OrderID)
		}
		return fmt.Errorf("lookup order failed for out_trade_no %s: %w", n.OrderID, err)
	}
	return s.confirmPayment(ctx, order.ID, n.TradeNo, n.Amount, pk, n.Metadata)
}

func parseLegacyPaymentOrderID(orderID string, lookupErr error) (int64, bool) {
	if !dbent.IsNotFound(lookupErr) {
		return 0, false
	}
	orderID = strings.TrimSpace(orderID)
	if !strings.HasPrefix(orderID, orderIDPrefix) {
		return 0, false
	}
	trimmed := strings.TrimPrefix(orderID, orderIDPrefix)
	if trimmed == "" || trimmed == orderID {
		return 0, false
	}
	oid, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || oid <= 0 {
		return 0, false
	}
	return oid, true
}

func (s *PaymentService) confirmPayment(ctx context.Context, oid int64, tradeNo string, paid float64, pk string, metadata map[string]string) error {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		if dbent.IsNotFound(err) {
			return fmt.Errorf("%w: out_trade_no=%s", ErrOrderNotFound, orderIDPrefix+strconv.FormatInt(oid, 10))
		}
		return fmt.Errorf("get payment order %d: %w", oid, err)
	}
	instanceProviderKey := ""
	if inst, instErr := s.getOrderProviderInstance(ctx, o); instErr == nil && inst != nil {
		instanceProviderKey = inst.ProviderKey
	}
	expectedProviderKey := expectedNotificationProviderKeyForOrder(s.registry, o, instanceProviderKey)
	if expectedProviderKey != "" && strings.TrimSpace(pk) != "" && !strings.EqualFold(expectedProviderKey, strings.TrimSpace(pk)) {
		s.writeAuditLog(ctx, o.ID, "PAYMENT_PROVIDER_MISMATCH", pk, map[string]any{
			"expectedProvider": expectedProviderKey,
			"actualProvider":   pk,
			"tradeNo":          tradeNo,
		})
		return fmt.Errorf("provider mismatch: expected %s, got %s", expectedProviderKey, pk)
	}
	if err := validateProviderNotificationMetadata(o, pk, metadata); err != nil {
		s.writeAuditLog(ctx, o.ID, "PAYMENT_PROVIDER_METADATA_MISMATCH", pk, map[string]any{
			"detail":  err.Error(),
			"tradeNo": tradeNo,
		})
		return err
	}
	if !isValidProviderAmount(paid) {
		s.writeAuditLog(ctx, o.ID, "PAYMENT_INVALID_AMOUNT", pk, map[string]any{
			"expected": o.PayAmount,
			"paid":     paid,
			"tradeNo":  tradeNo,
		})
		return fmt.Errorf("invalid paid amount from provider: %v", paid)
	}
	if math.Abs(paid-o.PayAmount) > paymentAmountToleranceForCurrency(PaymentOrderCurrency(o)) {
		s.writeAuditLog(ctx, o.ID, "PAYMENT_AMOUNT_MISMATCH", pk, map[string]any{"expected": o.PayAmount, "paid": paid, "tradeNo": tradeNo})
		return fmt.Errorf("amount mismatch: expected %s, got %s", strconv.FormatFloat(o.PayAmount, 'f', -1, 64), strconv.FormatFloat(paid, 'f', -1, 64))
	}
	return s.toPaid(ctx, o, tradeNo, paid, pk)
}

func paymentAmountToleranceForCurrency(currency string) float64 {
	minorUnit := payment.CurrencyMinorUnit(currency)
	if minorUnit <= 2 {
		return amountToleranceCNY
	}
	return math.Pow10(-minorUnit) / 2
}

func isValidProviderAmount(amount float64) bool {
	return amount > 0 && !math.IsNaN(amount) && !math.IsInf(amount, 0)
}

func validateProviderNotificationMetadata(order *dbent.PaymentOrder, providerKey string, metadata map[string]string) error {
	return validateProviderSnapshotMetadata(order, providerKey, metadata)
}

func expectedNotificationProviderKey(registry *payment.Registry, orderPaymentType string, orderProviderKey string, instanceProviderKey string) string {
	if key := strings.TrimSpace(instanceProviderKey); key != "" {
		return key
	}
	if key := strings.TrimSpace(orderProviderKey); key != "" {
		return key
	}
	if registry != nil {
		if key := strings.TrimSpace(registry.GetProviderKey(payment.PaymentType(orderPaymentType))); key != "" {
			return key
		}
	}
	return strings.TrimSpace(orderPaymentType)
}

func (s *PaymentService) toPaid(ctx context.Context, o *dbent.PaymentOrder, tradeNo string, paid float64, pk string) error {
	previousStatus := o.Status
	now := time.Now()
	grace := now.Add(-paymentGraceMinutes * time.Minute)
	c, err := s.entClient.PaymentOrder.Update().Where(
		paymentorder.IDEQ(o.ID),
		paymentorder.Or(
			paymentorder.StatusEQ(OrderStatusPending),
			paymentorder.StatusEQ(OrderStatusFailed),
			paymentorder.And(
				paymentorder.StatusEQ(OrderStatusExpired),
				paymentorder.UpdatedAtGTE(grace),
			),
			paymentorder.And(
				paymentorder.StatusEQ(OrderStatusCancelled),
				paymentorder.UpdatedAtGTE(grace),
			),
		),
	).SetStatus(OrderStatusPaid).SetPayAmount(paid).SetPaymentTradeNo(tradeNo).SetPaidAt(now).ClearFailedAt().ClearFailedReason().Save(ctx)
	if err != nil {
		return fmt.Errorf("update to PAID: %w", err)
	}
	if c == 0 {
		return s.alreadyProcessed(ctx, o, tradeNo, paid, pk)
	}
	if previousStatus != OrderStatusPending {
		slog.Info("order recovered from webhook payment success",
			"orderID", o.ID,
			"previousStatus", previousStatus,
			"tradeNo", tradeNo,
			"provider", pk,
		)
		s.writeAuditLog(ctx, o.ID, "ORDER_RECOVERED", pk, map[string]any{
			"previous_status": previousStatus,
			"tradeNo":         tradeNo,
			"paidAmount":      paid,
			"reason":          "webhook payment success received after order " + previousStatus,
		})
	}
	s.writeAuditLog(ctx, o.ID, "ORDER_PAID", pk, map[string]any{"tradeNo": tradeNo, "paidAmount": paid})
	return s.executeFulfillment(ctx, o.ID)
}

func (s *PaymentService) alreadyProcessed(ctx context.Context, o *dbent.PaymentOrder, tradeNo string, paid float64, pk string) error {
	cur, err := s.entClient.PaymentOrder.Get(ctx, o.ID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return fmt.Errorf("%w: payment order id=%d", ErrOrderNotFound, o.ID)
		}
		return fmt.Errorf("reload payment order %d after concurrent status change: %w", o.ID, err)
	}
	switch cur.Status {
	case OrderStatusCompleted, OrderStatusRefunded:
		return nil
	case OrderStatusFailed:
		recovered, recoverErr := s.markFailedOrderPaidAndReload(ctx, o.ID, tradeNo, paid)
		if recoverErr != nil {
			return recoverErr
		}
		return s.executeFulfillment(ctx, recovered.ID)
	case OrderStatusPaid, OrderStatusRecharging:
		return s.executeFulfillment(ctx, cur.ID)
	case OrderStatusCancelled:
		slog.Warn("webhook payment success for cancelled order",
			"orderID", o.ID,
			"status", cur.Status,
			"tradeNo", tradeNo,
			"provider", pk,
		)
		s.writeAuditLog(ctx, o.ID, "PAYMENT_AFTER_CANCELLED", pk, map[string]any{
			"status":     cur.Status,
			"updatedAt":  cur.UpdatedAt,
			"tradeNo":    tradeNo,
			"paidAmount": paid,
			"reason":     "payment arrived after cancellation",
		})
		return nil
	case OrderStatusExpired:
		slog.Warn("webhook payment success for expired order beyond grace period",
			"orderID", o.ID,
			"status", cur.Status,
			"updatedAt", cur.UpdatedAt,
			"tradeNo", tradeNo,
			"provider", pk,
		)
		s.writeAuditLog(ctx, o.ID, "PAYMENT_AFTER_EXPIRY", pk, map[string]any{
			"status":     cur.Status,
			"updatedAt":  cur.UpdatedAt,
			"tradeNo":    tradeNo,
			"paidAmount": paid,
			"reason":     "payment arrived after expiry grace period",
		})
		return nil
	default:
		return nil
	}
}

func (s *PaymentService) executeFulfillment(ctx context.Context, oid int64) error {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return fmt.Errorf("get order: %w", err)
	}
	if o.OrderType == payment.OrderTypeSubscription {
		return s.ExecuteSubscriptionFulfillment(ctx, oid)
	}
	return s.ExecuteBalanceFulfillment(ctx, oid)
}

func (s *PaymentService) ExecuteBalanceFulfillment(ctx context.Context, oid int64) error {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.Status == OrderStatusCompleted {
		return nil
	}
	if psIsRefundStatus(o.Status) {
		return infraerrors.BadRequest("INVALID_STATUS", "refund-related order cannot fulfill")
	}
	if o.Status != OrderStatusPaid && o.Status != OrderStatusFailed && o.Status != OrderStatusRecharging {
		return infraerrors.BadRequest("INVALID_STATUS", "order cannot fulfill in status "+o.Status)
	}
	lease, err := s.acquirePaymentFulfillmentLease(ctx, o)
	if err != nil {
		return err
	}
	if lease == nil {
		return nil
	}
	if err := s.doBalance(ctx, o, lease); err != nil {
		s.markFailed(ctx, oid, lease, err)
		return err
	}
	return nil
}

func (s *PaymentService) acquirePaymentFulfillmentLease(ctx context.Context, o *dbent.PaymentOrder) (*paymentFulfillmentLease, error) {
	if o == nil {
		return nil, infraerrors.BadRequest("INVALID_STATUS", "nil payment order")
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	staleBefore := now.Add(-paymentFulfillmentLeaseDuration)
	updated, err := s.entClient.PaymentOrder.Update().
		Where(
			paymentorder.IDEQ(o.ID),
			paymentorder.Or(
				paymentorder.StatusIn(OrderStatusPaid, OrderStatusFailed),
				paymentorder.And(
					paymentorder.StatusEQ(OrderStatusRecharging),
					paymentorder.UpdatedAtLTE(staleBefore),
				),
			),
		).
		SetStatus(OrderStatusRecharging).
		SetUpdatedAt(now).
		ClearFailedAt().
		ClearFailedReason().
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire fulfillment lease: %w", err)
	}
	if updated == 0 {
		current, getErr := s.entClient.PaymentOrder.Get(ctx, o.ID)
		if getErr != nil {
			return nil, fmt.Errorf("reload fulfillment lease: %w", getErr)
		}
		if current.Status == OrderStatusCompleted {
			return nil, nil
		}
		if current.Status == OrderStatusRecharging {
			return nil, infraerrors.Conflict("CONFLICT", "order is being processed")
		}
		return nil, infraerrors.Conflict("CONFLICT", "order status changed while acquiring fulfillment lease")
	}

	claimed, err := s.entClient.PaymentOrder.Get(ctx, o.ID)
	if err != nil {
		return nil, fmt.Errorf("reload acquired fulfillment lease: %w", err)
	}
	if claimed.Status != OrderStatusRecharging {
		return nil, infraerrors.Conflict("CONFLICT", "fulfillment lease was lost")
	}
	return &paymentFulfillmentLease{version: claimed.UpdatedAt}, nil
}

// redeemAction represents the idempotency decision for balance fulfillment.
type redeemAction int

const (
	// redeemActionCreate: code does not exist — create it, then redeem.
	redeemActionCreate redeemAction = iota
	// redeemActionRedeem: code exists but is unused — skip creation, redeem only.
	redeemActionRedeem
	// redeemActionSkipCompleted: code exists and is already used — skip to mark completed.
	redeemActionSkipCompleted
)

// resolveRedeemAction decides the idempotency action based on an existing redeem code lookup.
// existing is the result of GetByCode; lookupErr is the error from that call.
func resolveRedeemAction(existing *RedeemCode, lookupErr error) redeemAction {
	if existing == nil || lookupErr != nil {
		return redeemActionCreate
	}
	if existing.IsUsed() {
		return redeemActionSkipCompleted
	}
	return redeemActionRedeem
}

func (s *PaymentService) doBalance(ctx context.Context, o *dbent.PaymentOrder, lease *paymentFulfillmentLease) error {
	if s.entClient != nil {
		return s.doBalanceInTx(ctx, o, lease)
	}
	// Idempotency: check if redeem code already exists (from a previous partial run)
	existing, lookupErr := s.redeemService.GetByCode(ctx, o.RechargeCode)
	action := resolveRedeemAction(existing, lookupErr)

	switch action {
	case redeemActionSkipCompleted:
		// Code already created and redeemed — just mark completed
		if err := s.markCompleted(ctx, o, lease, "RECHARGE_SUCCESS"); err != nil {
			return err
		}
		s.applyAffiliateRebateBestEffort(ctx, o)
		return nil
	case redeemActionCreate:
		rc := &RedeemCode{Code: o.RechargeCode, Type: RedeemTypeBalance, Value: o.Amount, Status: StatusUnused}
		if err := s.redeemService.CreateCode(ctx, rc); err != nil {
			return fmt.Errorf("create redeem code: %w", err)
		}
	case redeemActionRedeem:
		// Code exists but unused — skip creation, proceed to redeem
	}
	if _, err := s.redeemService.Redeem(ContextSkipRedeemAffiliate(ctx), o.UserID, o.RechargeCode); err != nil {
		return fmt.Errorf("redeem balance: %w", err)
	}
	if err := s.markCompleted(ctx, o, lease, "RECHARGE_SUCCESS"); err != nil {
		return err
	}
	s.applyAffiliateRebateBestEffort(ctx, o)
	return nil
}

func (s *PaymentService) doBalanceInTx(ctx context.Context, o *dbent.PaymentOrder, lease *paymentFulfillmentLease) error {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin balance fulfillment transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txClient := tx.Client()
	existing, lookupErr := txClient.RedeemCode.Query().Where(redeemcode.CodeEQ(o.RechargeCode)).Only(ctx)
	if lookupErr != nil && !dbent.IsNotFound(lookupErr) {
		return fmt.Errorf("get redeem code: %w", lookupErr)
	}

	if existing != nil && existing.Status == StatusUsed {
		completed, err := s.markCompletedWithClient(ctx, txClient, o, lease, "RECHARGE_SUCCESS")
		if err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		if completed {
			s.dispatchPaymentFulfillmentNotification(o, "RECHARGE_SUCCESS")
		}
		s.applyAffiliateRebateBestEffort(ctx, o)
		return nil
	}

	if existing == nil {
		existing, err = txClient.RedeemCode.Create().
			SetCode(o.RechargeCode).
			SetType(RedeemTypeBalance).
			SetValue(o.Amount).
			SetStatus(StatusUnused).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("create redeem code: %w", err)
		}
	}

	now := time.Now()
	affected, err := txClient.RedeemCode.Update().
		Where(redeemcode.IDEQ(existing.ID), redeemcode.StatusEQ(StatusUnused)).
		SetStatus(StatusUsed).
		SetUsedBy(o.UserID).
		SetUsedAt(now).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("mark code as used: %w", err)
	}
	if affected == 0 {
		return ErrRedeemCodeUsed
	}

	userBalance, err := txClient.User.Query().Where(user.IDEQ(o.UserID)).Only(ctx)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	amount := existing.Value
	if amount < 0 && userBalance.Balance+amount < 0 {
		amount = -userBalance.Balance
	}
	update := txClient.User.Update().Where(user.IDEQ(o.UserID)).AddBalance(amount)
	if amount > 0 {
		update = update.AddTotalRecharged(amount)
	}
	c, err := update.Save(ctx)
	if err != nil {
		return fmt.Errorf("update user balance: %w", err)
	}
	if c == 0 {
		return ErrUserNotFound
	}

	completed, err := s.markCompletedWithClient(ctx, txClient, o, lease, "RECHARGE_SUCCESS")
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if completed {
		s.dispatchPaymentFulfillmentNotification(o, "RECHARGE_SUCCESS")
	}
	s.applyAffiliateRebateBestEffort(ctx, o)
	return nil
}

func (s *PaymentService) applyAffiliateRebateBestEffort(ctx context.Context, o *dbent.PaymentOrder) {
	if err := s.applyAffiliateRebateForOrder(ctx, o); err != nil {
		slog.Error("affiliate rebate failed after payment fulfillment", "orderID", o.ID, "error", err)
	}
}

func (s *PaymentService) markCompleted(ctx context.Context, o *dbent.PaymentOrder, lease *paymentFulfillmentLease, auditAction string) error {
	completed, err := s.markCompletedWithClient(ctx, s.entClient, o, lease, auditAction)
	if err == nil && completed {
		s.dispatchPaymentFulfillmentNotification(o, auditAction)
	}
	return err
}

func (s *PaymentService) markCompletedWithoutAudit(ctx context.Context, o *dbent.PaymentOrder, lease *paymentFulfillmentLease) (bool, error) {
	return completePaymentOrderWithLease(ctx, s.entClient, o, lease)
}

func completePaymentOrderWithLease(ctx context.Context, client *dbent.Client, o *dbent.PaymentOrder, lease *paymentFulfillmentLease) (bool, error) {
	if client == nil || o == nil || lease == nil {
		return false, errors.New("missing payment fulfillment lease")
	}
	now := time.Now()
	updated, err := client.PaymentOrder.Update().
		Where(
			paymentorder.IDEQ(o.ID),
			paymentorder.StatusEQ(OrderStatusRecharging),
			paymentorder.UpdatedAtEQ(lease.version),
		).
		SetStatus(OrderStatusCompleted).
		SetCompletedAt(now).
		Save(ctx)
	if err != nil {
		return false, fmt.Errorf("mark completed: %w", err)
	}
	if updated > 0 {
		return true, nil
	}
	current, getErr := client.PaymentOrder.Get(ctx, o.ID)
	if getErr == nil && current.Status == OrderStatusCompleted {
		return false, nil
	}
	return false, infraerrors.Conflict("CONFLICT", "fulfillment lease was lost before completion")
}

func (s *PaymentService) markCompletedWithClient(ctx context.Context, client *dbent.Client, o *dbent.PaymentOrder, lease *paymentFulfillmentLease, auditAction string) (bool, error) {
	completed, err := completePaymentOrderWithLease(ctx, client, o, lease)
	if err != nil {
		return false, err
	}
	if !completed {
		return false, nil
	}
	exists, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(o.ID, 10)), paymentauditlog.ActionEQ(auditAction)).
		Exist(ctx)
	if err != nil {
		return false, fmt.Errorf("check payment success audit: %w", err)
	}
	if !exists {
		if err := s.writeAuditLogErrWithClient(ctx, client, o.ID, auditAction, "system", map[string]any{
			"rechargeCode":   o.RechargeCode,
			"creditedAmount": o.Amount,
			"payAmount":      o.PayAmount,
		}); err != nil {
			return false, fmt.Errorf("write payment success audit: %w", err)
		}
	}
	return true, nil
}

func (s *PaymentService) dispatchPaymentFulfillmentNotification(o *dbent.PaymentOrder, auditAction string) {
	if s != nil && s.notificationDispatchHook != nil {
		s.notificationDispatchHook(o, auditAction)
		return
	}
	if s == nil || s.notificationEmailService == nil || o == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), emailSendTimeout)
		defer cancel()
		var err error
		switch auditAction {
		case "RECHARGE_SUCCESS":
			err = s.sendBalanceRechargeSuccessNotification(ctx, o)
		case "SUBSCRIPTION_SUCCESS":
			err = s.sendSubscriptionPurchaseSuccessNotification(ctx, o)
		default:
			return
		}
		if err != nil {
			slog.Warn("payment fulfillment notification email failed", "order_id", o.ID, "action", auditAction, "err", err.Error())
		}
	}()
}

func (s *PaymentService) sendBalanceRechargeSuccessNotification(ctx context.Context, o *dbent.PaymentOrder) error {
	currentBalance := ""
	if s.userRepo != nil {
		if user, err := s.userRepo.GetByID(ctx, o.UserID); err == nil && user != nil {
			currentBalance = fmt.Sprintf("%.2f", user.Balance)
		}
	}
	return s.notificationEmailService.Send(ctx, NotificationEmailSendInput{
		Event:          NotificationEmailEventBalanceRechargeSuccess,
		RecipientEmail: o.UserEmail,
		RecipientName:  firstNonEmpty(o.UserName, o.UserEmail),
		UserID:         o.UserID,
		SourceType:     "payment_order",
		SourceID:       strconv.FormatInt(o.ID, 10),
		Variables: map[string]string{
			"recharge_amount": fmt.Sprintf("%.2f", o.Amount),
			"current_balance": currentBalance,
			"order_id":        strconv.FormatInt(o.ID, 10),
		},
	})
}

func (s *PaymentService) sendSubscriptionPurchaseSuccessNotification(ctx context.Context, o *dbent.PaymentOrder) error {
	variables := map[string]string{
		"subscription_group": "Subscription",
		"subscription_days":  "",
		"expiry_time":        "",
		"order_id":           strconv.FormatInt(o.ID, 10),
	}
	if o.SubscriptionDays != nil {
		variables["subscription_days"] = strconv.Itoa(*o.SubscriptionDays)
	}
	if o.SubscriptionGroupID != nil {
		if s.groupRepo != nil {
			if group, err := s.groupRepo.GetByID(ctx, *o.SubscriptionGroupID); err == nil && group != nil && strings.TrimSpace(group.Name) != "" {
				variables["subscription_group"] = group.Name
			}
		}
		if s.subscriptionSvc != nil {
			if sub, err := s.subscriptionSvc.GetActiveSubscription(ctx, o.UserID, *o.SubscriptionGroupID); err == nil && sub != nil {
				variables["expiry_time"] = sub.ExpiresAt.Format("2006-01-02 15:04")
			}
		}
	}
	return s.notificationEmailService.Send(ctx, NotificationEmailSendInput{
		Event:          NotificationEmailEventSubscriptionPurchaseSuccess,
		RecipientEmail: o.UserEmail,
		RecipientName:  firstNonEmpty(o.UserName, o.UserEmail),
		UserID:         o.UserID,
		SourceType:     "payment_order",
		SourceID:       strconv.FormatInt(o.ID, 10),
		Variables:      variables,
	})
}

func (s *PaymentService) ExecuteSubscriptionFulfillment(ctx context.Context, oid int64) error {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.Status == OrderStatusCompleted {
		return nil
	}
	if psIsRefundStatus(o.Status) {
		return infraerrors.BadRequest("INVALID_STATUS", "refund-related order cannot fulfill")
	}
	if o.Status != OrderStatusPaid && o.Status != OrderStatusFailed && o.Status != OrderStatusRecharging {
		return infraerrors.BadRequest("INVALID_STATUS", "order cannot fulfill in status "+o.Status)
	}
	if o.SubscriptionGroupID == nil || o.SubscriptionDays == nil {
		return infraerrors.BadRequest("INVALID_STATUS", "missing subscription info")
	}
	lease, err := s.acquirePaymentFulfillmentLease(ctx, o)
	if err != nil {
		return err
	}
	if lease == nil {
		return nil
	}
	if err := s.doSub(ctx, o, lease); err != nil {
		s.markFailed(ctx, oid, lease, err)
		return err
	}
	return nil
}

func (s *PaymentService) doSub(ctx context.Context, o *dbent.PaymentOrder, lease *paymentFulfillmentLease) error {
	gid := *o.SubscriptionGroupID
	days := *o.SubscriptionDays
	g, err := s.groupRepo.GetByID(ctx, gid)
	if err != nil || g.Status != payment.EntityStatusActive {
		return fmt.Errorf("group %d no longer exists or inactive", gid)
	}
	claimed, err := s.tryClaimSubscriptionFulfillmentAudit(ctx, o, gid, days)
	if err != nil {
		return fmt.Errorf("claim subscription fulfillment: %w", err)
	}
	if !claimed {
		state, err := s.subscriptionFulfillmentAuditState(ctx, o.ID)
		if err != nil {
			return fmt.Errorf("inspect subscription fulfillment claim: %w", err)
		}
		switch state {
		case subscriptionFulfillmentAuditStateSucceeded:
			slog.Info("subscription already assigned for order, skipping", "orderID", o.ID, "groupID", gid)
			// The entitlement and success sentinel are already committed, but a
			// previous post-commit cache invalidation may have failed. Retry it on
			// every recovery without turning cache availability into fulfillment
			// failure.
			s.invalidateSubscriptionCachesBestEffort(o.UserID, gid)
			completed, err := s.markCompletedWithoutAudit(ctx, o, lease)
			if err != nil {
				return err
			}
			if completed {
				s.dispatchPaymentFulfillmentNotification(o, "SUBSCRIPTION_SUCCESS")
			}
			s.applyAffiliateRebateBestEffort(ctx, o)
			return nil
		case subscriptionFulfillmentAuditStateClaimed:
			slog.Warn("resuming subscription fulfillment from orphaned claim", "orderID", o.ID, "groupID", gid)
		default:
			return errors.New("subscription fulfillment claim missing after claim attempt")
		}
	}
	orderNote := fmt.Sprintf("payment order %d", o.ID)
	if err := s.assignSubscriptionExactlyOnce(ctx, o, gid, days, orderNote); err != nil {
		s.releaseSubscriptionFulfillmentClaim(ctx, o.ID)
		return err
	}
	// 订单完成标记刻意留在事务外、且在 SUCCESS 哨兵提交之后：保持「哨兵先于完成」的恢复语义——
	// 完成标记失败时哨兵已在库，重试走 doSub 的 state==Succeeded 分支只补完成、不重复发放。
	completed, err := s.markCompletedWithoutAudit(ctx, o, lease)
	if err != nil {
		return err
	}
	if completed {
		s.dispatchPaymentFulfillmentNotification(o, "SUBSCRIPTION_SUCCESS")
	}
	s.applyAffiliateRebateBestEffort(ctx, o)
	return nil
}

// assignSubscriptionExactlyOnce 把「订阅发放 + SUCCESS 哨兵审计」收进单个事务，二者同提交同回滚。
//
// 修复要害：此前 AssignOrExtendSubscription 先自行提交、SUCCESS 哨兵再单独写，二者之间的崩溃窗口
// 会丢失「已发放」信号；重试时 orphaned-claim 恢复路径（doSub 中 state==Claimed 分支）会把订单当成
// 未发放再次 AssignOrExtendSubscription，对已有订阅无条件 AddDate 累加 → 订阅天数重复发放。收进单
// 事务后：崩溃前未提交即整体回滚（含 assign），重试重新发放不叠加；崩溃后已提交则哨兵在库，
// state==Succeeded 直接跳过。订单完成标记不在本事务内（见调用方），以保留「哨兵先于完成」语义。
func (s *PaymentService) assignSubscriptionExactlyOnce(ctx context.Context, o *dbent.PaymentOrder, groupID int64, days int, orderNote string) error {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin subscription fulfillment transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Subscription assignment reuses this transaction and defers cache
	// invalidation until the assignment and success sentinel are both committed.
	txCtx := dbent.NewTxContext(ctx, tx)
	if _, _, err := s.subscriptionSvc.assignOrExtendSubscription(txCtx, &AssignSubscriptionInput{
		UserID:       o.UserID,
		GroupID:      groupID,
		ValidityDays: days,
		AssignedBy:   0,
		Notes:        orderNote,
	}, true); err != nil {
		return fmt.Errorf("assign subscription: %w", err)
	}
	if err := s.writeAuditLogErrWithClient(txCtx, tx.Client(), o.ID, "SUBSCRIPTION_SUCCESS", "system", map[string]any{
		"rechargeCode":   o.RechargeCode,
		"creditedAmount": o.Amount,
		"payAmount":      o.PayAmount,
	}); err != nil {
		return fmt.Errorf("write subscription success audit: %w", err)
	}
	if err := s.commitTx(tx); err != nil {
		return fmt.Errorf("commit subscription fulfillment transaction: %w", err)
	}
	s.invalidateSubscriptionCachesBestEffort(o.UserID, groupID)
	return nil
}

func (s *PaymentService) invalidateSubscriptionCachesBestEffort(userID, groupID int64) {
	if s == nil || s.subscriptionSvc == nil {
		return
	}
	if err := s.subscriptionSvc.invalidateSubscriptionCaches(userID, groupID); err != nil {
		slog.Warn("invalidate subscription cache after fulfillment failed",
			"userID", userID,
			"groupID", groupID,
			"error", err,
		)
	}
}

type subscriptionFulfillmentAuditState int

const (
	subscriptionFulfillmentAuditStateNone subscriptionFulfillmentAuditState = iota
	subscriptionFulfillmentAuditStateClaimed
	subscriptionFulfillmentAuditStateSucceeded
)

func (s *PaymentService) tryClaimSubscriptionFulfillmentAudit(ctx context.Context, o *dbent.PaymentOrder, groupID int64, days int) (bool, error) {
	if s == nil || s.entClient == nil || o == nil {
		return false, errors.New("nil payment service")
	}
	oid := strconv.FormatInt(o.ID, 10)
	detail, _ := json.Marshal(map[string]any{
		"groupID":      groupID,
		"days":         days,
		"rechargeCode": o.RechargeCode,
		"status":       "reserved",
	})
	rows, err := s.entClient.QueryContext(ctx, `
	INSERT INTO payment_audit_logs (order_id, action, detail, operator, created_at)
	SELECT CAST($1 AS VARCHAR(64)), 'SUBSCRIPTION_FULFILLMENT_CLAIMED', CAST($2 AS TEXT), 'system', CURRENT_TIMESTAMP
	WHERE NOT EXISTS (
		SELECT 1
		FROM payment_audit_logs
		WHERE order_id = CAST($1 AS VARCHAR(64))
		  AND action IN ('SUBSCRIPTION_FULFILLMENT_CLAIMED', 'SUBSCRIPTION_SUCCESS')
	)
	RETURNING id`, oid, string(detail))
	if err != nil {
		if isSubscriptionFulfillmentClaimConflict(err) {
			return false, nil
		}
		return false, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, nil
	}
	var claimID int64
	if err := rows.Scan(&claimID); err != nil {
		return false, err
	}
	return true, nil
}

func (s *PaymentService) subscriptionFulfillmentAuditState(ctx context.Context, orderID int64) (subscriptionFulfillmentAuditState, error) {
	logs, err := s.entClient.PaymentAuditLog.Query().
		Where(
			paymentauditlog.OrderIDEQ(strconv.FormatInt(orderID, 10)),
			paymentauditlog.ActionIn("SUBSCRIPTION_FULFILLMENT_CLAIMED", "SUBSCRIPTION_SUCCESS"),
		).
		All(ctx)
	if err != nil {
		return subscriptionFulfillmentAuditStateNone, err
	}
	state := subscriptionFulfillmentAuditStateNone
	for _, log := range logs {
		switch log.Action {
		case "SUBSCRIPTION_SUCCESS":
			return subscriptionFulfillmentAuditStateSucceeded, nil
		case "SUBSCRIPTION_FULFILLMENT_CLAIMED":
			state = subscriptionFulfillmentAuditStateClaimed
		}
	}
	return state, nil
}

func isSubscriptionFulfillmentClaimConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") &&
		(strings.Contains(msg, "payment_audit_logs") || strings.Contains(msg, "idx_payment_audit_logs_order_action_uniq"))
}

func (s *PaymentService) releaseSubscriptionFulfillmentClaim(ctx context.Context, orderID int64) {
	if s == nil || s.entClient == nil {
		return
	}
	_, err := s.entClient.PaymentAuditLog.Delete().
		Where(
			paymentauditlog.OrderIDEQ(strconv.FormatInt(orderID, 10)),
			paymentauditlog.ActionEQ("SUBSCRIPTION_FULFILLMENT_CLAIMED"),
		).
		Exec(ctx)
	if err != nil {
		slog.Error("release subscription fulfillment claim failed", "orderID", orderID, "error", err)
	}
}

func (s *PaymentService) hasAuditLog(ctx context.Context, orderID int64, action string) bool {
	oid := strconv.FormatInt(orderID, 10)
	c, _ := s.entClient.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(oid), paymentauditlog.ActionEQ(action)).
		Limit(1).Count(ctx)
	return c > 0
}

func (s *PaymentService) applyAffiliateRebateForOrder(ctx context.Context, o *dbent.PaymentOrder) error {
	if o == nil || (o.OrderType != payment.OrderTypeBalance && o.OrderType != payment.OrderTypeSubscription) || o.Amount <= 0 {
		return nil
	}
	if s.affiliateService == nil {
		return nil
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		s.logAffiliateRebateFailure(ctx, o.ID, "begin affiliate rebate tx", err)
		return fmt.Errorf("begin affiliate rebate tx: %w", err)
	}
	txDone := false
	defer func() {
		if !txDone {
			_ = tx.Rollback()
		}
	}()
	failAfterRollback := func(stage string, err error) error {
		if !txDone {
			_ = tx.Rollback()
			txDone = true
		}
		s.logAffiliateRebateFailure(ctx, o.ID, stage, err)
		return fmt.Errorf("%s: %w", stage, err)
	}

	txCtx := dbent.NewTxContext(ctx, tx)
	claimed, err := s.tryClaimAffiliateRebateAudit(txCtx, tx.Client(), o.ID, o.Amount)
	if err != nil {
		return failAfterRollback("claim affiliate rebate audit", err)
	}
	if !claimed {
		_ = tx.Rollback()
		txDone = true
		return nil
	}

	rebateAmount, err := s.affiliateService.AccrueInviteRebateForOrder(txCtx, o.UserID, o.ID, o.Amount)
	if err != nil {
		return failAfterRollback("accrue invite rebate", err)
	}

	if rebateAmount <= 0 {
		if err := s.updateClaimedAffiliateRebateAudit(txCtx, tx.Client(), o.ID, "AFFILIATE_REBATE_SKIPPED", map[string]any{
			"baseAmount": o.Amount,
			"reason":     "no inviter bound or rebate amount <= 0",
		}); err != nil {
			return failAfterRollback("update skipped affiliate rebate audit", err)
		}
		if err := s.commitTx(tx); err != nil {
			_ = tx.Rollback()
			txDone = true
			s.logAffiliateRebateFailure(ctx, o.ID, "commit affiliate rebate tx", err)
			return fmt.Errorf("commit affiliate rebate tx: %w", err)
		}
		txDone = true
		return nil
	}

	if err := s.updateClaimedAffiliateRebateAudit(txCtx, tx.Client(), o.ID, "AFFILIATE_REBATE_APPLIED", map[string]any{
		"baseAmount":   o.Amount,
		"rebateAmount": rebateAmount,
	}); err != nil {
		return failAfterRollback("update applied affiliate rebate audit", err)
	}

	if err := s.commitTx(tx); err != nil {
		_ = tx.Rollback()
		txDone = true
		s.logAffiliateRebateFailure(ctx, o.ID, "commit affiliate rebate tx", err)
		return fmt.Errorf("commit affiliate rebate tx: %w", err)
	}
	txDone = true
	return nil
}

func (s *PaymentService) logAffiliateRebateFailure(ctx context.Context, orderID int64, stage string, err error) {
	s.writeAuditLog(ctx, orderID, "AFFILIATE_REBATE_FAILED", "system", map[string]any{
		"stage": stage,
		"error": err.Error(),
	})
}

func (s *PaymentService) tryClaimAffiliateRebateAudit(ctx context.Context, client *dbent.Client, orderID int64, baseAmount float64) (bool, error) {
	if client == nil {
		return false, errors.New("nil payment client")
	}
	oid := strconv.FormatInt(orderID, 10)
	detail, _ := json.Marshal(map[string]any{
		"baseAmount": baseAmount,
		"status":     "reserved",
	})
	rows, err := client.QueryContext(ctx, `
INSERT INTO payment_audit_logs (order_id, action, detail, operator, created_at)
SELECT CAST($1 AS VARCHAR(64)), 'AFFILIATE_REBATE_APPLIED', CAST($2 AS TEXT), 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (
	SELECT 1
	FROM payment_audit_logs
	WHERE order_id = CAST($1 AS VARCHAR(64))
	  AND action IN ('AFFILIATE_REBATE_APPLIED', 'AFFILIATE_REBATE_SKIPPED')
)
RETURNING id`, oid, string(detail))
	if err != nil {
		if isAffiliateRebateClaimConflict(err) {
			return false, nil
		}
		return false, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, nil
	}
	var claimID int64
	if err := rows.Scan(&claimID); err != nil {
		return false, err
	}
	return true, nil
}

func (s *PaymentService) updateClaimedAffiliateRebateAudit(ctx context.Context, client *dbent.Client, orderID int64, action string, detail map[string]any) error {
	if client == nil {
		return errors.New("nil payment client")
	}
	oid := strconv.FormatInt(orderID, 10)
	detailJSON, _ := json.Marshal(detail)
	updated, err := client.PaymentAuditLog.Update().
		Where(
			paymentauditlog.OrderIDEQ(oid),
			paymentauditlog.ActionEQ("AFFILIATE_REBATE_APPLIED"),
		).
		SetAction(action).
		SetDetail(string(detailJSON)).
		SetOperator("system").
		Save(ctx)
	if err != nil {
		return err
	}
	if updated == 0 {
		return errors.New("affiliate rebate claim log not found")
	}
	return nil
}

func isAffiliateRebateClaimConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") &&
		(strings.Contains(msg, "payment_audit_logs") || strings.Contains(msg, "idx_payment_audit_logs_order_action_uniq"))
}

func (s *PaymentService) markFailed(ctx context.Context, oid int64, lease *paymentFulfillmentLease, cause error) {
	if lease == nil {
		slog.Error("mark FAILED without fulfillment lease", "orderID", oid)
		return
	}
	now := time.Now()
	r := psErrMsg(cause)
	// The lease version prevents a stale worker from overwriting a newer owner.
	c, e := s.entClient.PaymentOrder.Update().
		Where(
			paymentorder.IDEQ(oid),
			paymentorder.StatusEQ(OrderStatusRecharging),
			paymentorder.UpdatedAtEQ(lease.version),
		).
		SetStatus(OrderStatusFailed).SetFailedAt(now).SetFailedReason(r).Save(ctx)
	if e != nil {
		slog.Error("mark FAILED", "orderID", oid, "error", e)
	}
	if c > 0 {
		s.writeAuditLog(ctx, oid, "FULFILLMENT_FAILED", "system", map[string]any{"reason": r})
	}
}

func (s *PaymentService) RetryFulfillment(ctx context.Context, oid int64) error {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.PaidAt == nil {
		return infraerrors.BadRequest("INVALID_STATUS", "order is not paid")
	}
	if psIsRefundStatus(o.Status) {
		return infraerrors.BadRequest("INVALID_STATUS", "refund-related order cannot retry")
	}
	if o.Status == OrderStatusCompleted {
		s.applyAffiliateRebateBestEffort(ctx, o)
		return nil
	}
	if o.Status != OrderStatusFailed && o.Status != OrderStatusPaid && o.Status != OrderStatusRecharging {
		return infraerrors.BadRequest("INVALID_STATUS", "only paid, failed, and recoverable recharging orders can retry")
	}
	s.writeAuditLog(ctx, oid, "RECHARGE_RETRY", "admin", map[string]any{"detail": "admin manual retry"})
	return s.executeFulfillment(ctx, oid)
}
