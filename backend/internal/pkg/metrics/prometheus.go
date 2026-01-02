package metrics

import "sync/atomic"

var usageLogsFailedTotal atomic.Uint64

// IncUsageLogsFailed increments the Prometheus counter usage_logs_failed_total.
func IncUsageLogsFailed() {
	usageLogsFailedTotal.Add(1)
}

// UsageLogsFailedTotal returns the current counter value.
func UsageLogsFailedTotal() uint64 {
	return usageLogsFailedTotal.Load()
}

