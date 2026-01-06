package metrics

import "sync/atomic"

var usageLogsFailedTotal atomic.Uint64

var (
	opsPreaggFallbackWindowStatsNotPopulatedTotal    atomic.Uint64
	opsPreaggFallbackWindowStatsUnexpectedErrorTotal atomic.Uint64

	opsPreaggFallbackOverviewStatsNotPopulatedTotal    atomic.Uint64
	opsPreaggFallbackOverviewStatsUnexpectedErrorTotal atomic.Uint64

	opsPreaggFallbackProviderStatsNotPopulatedTotal    atomic.Uint64
	opsPreaggFallbackProviderStatsUnexpectedErrorTotal atomic.Uint64

	opsPreaggFallbackLatencyHistogramNotPopulatedTotal    atomic.Uint64
	opsPreaggFallbackLatencyHistogramUnexpectedErrorTotal atomic.Uint64

	opsPreaggFallbackUnknownMethodTotal atomic.Uint64
)

// IncUsageLogsFailed increments the Prometheus counter usage_logs_failed_total.
func IncUsageLogsFailed() {
	usageLogsFailedTotal.Add(1)
}

// UsageLogsFailedTotal returns the current counter value.
func UsageLogsFailedTotal() uint64 {
	return usageLogsFailedTotal.Load()
}

func IncOpsPreaggFallbackWindowStatsNotPopulated() {
	opsPreaggFallbackWindowStatsNotPopulatedTotal.Add(1)
}

func OpsPreaggFallbackWindowStatsNotPopulatedTotal() uint64 {
	return opsPreaggFallbackWindowStatsNotPopulatedTotal.Load()
}

func IncOpsPreaggFallbackWindowStatsUnexpectedError() {
	opsPreaggFallbackWindowStatsUnexpectedErrorTotal.Add(1)
}

func OpsPreaggFallbackWindowStatsUnexpectedErrorTotal() uint64 {
	return opsPreaggFallbackWindowStatsUnexpectedErrorTotal.Load()
}

func IncOpsPreaggFallbackOverviewStatsNotPopulated() {
	opsPreaggFallbackOverviewStatsNotPopulatedTotal.Add(1)
}

func OpsPreaggFallbackOverviewStatsNotPopulatedTotal() uint64 {
	return opsPreaggFallbackOverviewStatsNotPopulatedTotal.Load()
}

func IncOpsPreaggFallbackOverviewStatsUnexpectedError() {
	opsPreaggFallbackOverviewStatsUnexpectedErrorTotal.Add(1)
}

func OpsPreaggFallbackOverviewStatsUnexpectedErrorTotal() uint64 {
	return opsPreaggFallbackOverviewStatsUnexpectedErrorTotal.Load()
}

func IncOpsPreaggFallbackProviderStatsNotPopulated() {
	opsPreaggFallbackProviderStatsNotPopulatedTotal.Add(1)
}

func OpsPreaggFallbackProviderStatsNotPopulatedTotal() uint64 {
	return opsPreaggFallbackProviderStatsNotPopulatedTotal.Load()
}

func IncOpsPreaggFallbackProviderStatsUnexpectedError() {
	opsPreaggFallbackProviderStatsUnexpectedErrorTotal.Add(1)
}

func OpsPreaggFallbackProviderStatsUnexpectedErrorTotal() uint64 {
	return opsPreaggFallbackProviderStatsUnexpectedErrorTotal.Load()
}

func IncOpsPreaggFallbackLatencyHistogramNotPopulated() {
	opsPreaggFallbackLatencyHistogramNotPopulatedTotal.Add(1)
}

func OpsPreaggFallbackLatencyHistogramNotPopulatedTotal() uint64 {
	return opsPreaggFallbackLatencyHistogramNotPopulatedTotal.Load()
}

func IncOpsPreaggFallbackLatencyHistogramUnexpectedError() {
	opsPreaggFallbackLatencyHistogramUnexpectedErrorTotal.Add(1)
}

func OpsPreaggFallbackLatencyHistogramUnexpectedErrorTotal() uint64 {
	return opsPreaggFallbackLatencyHistogramUnexpectedErrorTotal.Load()
}

func IncOpsPreaggFallbackUnknownMethod() {
	opsPreaggFallbackUnknownMethodTotal.Add(1)
}

func OpsPreaggFallbackUnknownMethodTotal() uint64 {
	return opsPreaggFallbackUnknownMethodTotal.Load()
}
