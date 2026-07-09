package service

import "sync/atomic"

var usageUserDailyCostRollupReady atomic.Bool

// IsUsageUserDailyCostRollupReady reports whether the usage_user_daily_cost
// rollup has finished its initial backfill and can safely serve admin sorting.
func IsUsageUserDailyCostRollupReady() bool {
	return usageUserDailyCostRollupReady.Load()
}

// SetUsageUserDailyCostRollupReady updates the in-process readiness gate for
// usage_user_daily_cost based sorting.
func SetUsageUserDailyCostRollupReady(ready bool) {
	usageUserDailyCostRollupReady.Store(ready)
}
