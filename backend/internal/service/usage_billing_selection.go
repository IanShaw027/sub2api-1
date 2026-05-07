package service

import "math"

type usageBillingSelection struct {
	Cost                         *CostBreakdown
	BilledByHigherPricedUpstream bool
}

func chooseHigherPricedUsageCost(requestedModel string, requestedCost *CostBreakdown, upstreamModel string, upstreamCost *CostBreakdown) usageBillingSelection {
	if !isUsableCostBreakdown(upstreamCost) {
		return usageBillingSelection{Cost: requestedCost}
	}
	if !isUsableCostBreakdown(requestedCost) {
		return usageBillingSelection{
			Cost:                         upstreamCost,
			BilledByHigherPricedUpstream: true,
		}
	}

	if requestedModel == upstreamModel || upstreamModel == "" {
		return usageBillingSelection{Cost: requestedCost}
	}

	if upstreamCost.TotalCost > requestedCost.TotalCost+1e-12 {
		return usageBillingSelection{
			Cost:                         upstreamCost,
			BilledByHigherPricedUpstream: true,
		}
	}

	return usageBillingSelection{Cost: requestedCost}
}

func isUsableCostBreakdown(cost *CostBreakdown) bool {
	if cost == nil {
		return false
	}
	return !(math.IsNaN(cost.TotalCost) || math.IsInf(cost.TotalCost, 0))
}
