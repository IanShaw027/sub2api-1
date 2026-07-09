package service

import (
	"math"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/shopspring/decimal"
)

const defaultBalanceRechargeMultiplier = 1.0

func normalizeBalanceRechargeMultiplier(multiplier float64) float64 {
	if math.IsNaN(multiplier) || math.IsInf(multiplier, 0) || multiplier <= 0 {
		return defaultBalanceRechargeMultiplier
	}
	return multiplier
}

// normalizeSubscriptionUSDToCNYRate 将非法值归一为 0（换算关闭）。
// 与余额倍率不同，0 是合法状态：表示订阅保持 price 直付的存量行为。
func normalizeSubscriptionUSDToCNYRate(rate float64) float64 {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 {
		return 0
	}
	return rate
}

func calculateCreditedBalance(paymentAmount, multiplier float64) float64 {
	return decimal.NewFromFloat(paymentAmount).
		Mul(decimal.NewFromFloat(normalizeBalanceRechargeMultiplier(multiplier))).
		Round(2).
		InexactFloat64()
}

func amountToCents(amount float64) int64 {
	return decimal.NewFromFloat(amount).
		Round(2).
		Mul(decimal.NewFromInt(100)).
		IntPart()
}

func amountCentsEqual(left, right float64) bool {
	return amountToCents(left) == amountToCents(right)
}

func amountEqualForCurrency(left, right float64, currency string) bool {
	fractionDigits := int32(payment.CurrencyMaxFractionDigits(currency))
	return decimal.NewFromFloat(left).Round(fractionDigits).Equal(decimal.NewFromFloat(right).Round(fractionDigits))
}

func amountCentsGreaterThan(left, right float64) bool {
	return amountToCents(left) > amountToCents(right)
}

func calculateGatewayRefundAmount(orderAmount, payAmount, refundAmount float64, currency string) float64 {
	if orderAmount <= 0 || payAmount <= 0 || refundAmount <= 0 {
		return 0
	}
	fractionDigits := int32(payment.CurrencyMaxFractionDigits(currency))
	if math.Abs(refundAmount-orderAmount) <= paymentAmountToleranceForCurrency(currency) {
		return decimal.NewFromFloat(payAmount).Round(fractionDigits).InexactFloat64()
	}
	return decimal.NewFromFloat(payAmount).
		Mul(decimal.NewFromFloat(refundAmount)).
		Div(decimal.NewFromFloat(orderAmount)).
		Round(fractionDigits).
		InexactFloat64()
}

func calculateSubscriptionRefundAmounts(remaining, usage, subscriptionRate, refundRate float64, currency string) (usedRefundValue, maxRefundAmount float64) {
	if remaining <= 0 {
		return 0, 0
	}
	subscriptionRate = normalizeBalanceRechargeMultiplier(subscriptionRate)
	refundRate = normalizeRefundRateMultiplier(refundRate)
	fractionDigits := int32(payment.CurrencyMaxFractionDigits(currency))

	used := decimal.NewFromFloat(usage).
		Div(decimal.NewFromFloat(subscriptionRate)).
		Mul(decimal.NewFromFloat(refundRate)).
		Round(fractionDigits)
	maxRefund := decimal.NewFromFloat(remaining).
		Sub(used)
	if maxRefund.IsNegative() {
		maxRefund = decimal.Zero
	}
	maxRefund = maxRefund.Round(fractionDigits)
	return used.InexactFloat64(), maxRefund.InexactFloat64()
}
