import type { OpsMetricThresholds } from '@/api/admin/ops'

export type ThresholdLevel = 'normal' | 'warning' | 'critical'

export interface UseOpsThresholdsOptions {
  thresholds: () => OpsMetricThresholds | null | undefined
}

export function useOpsThresholds(options: UseOpsThresholdsOptions) {
  function getSLAThresholdLevel(slaPercent: number | null): ThresholdLevel {
    if (slaPercent == null) return 'normal'
    const threshold = options.thresholds()?.sla_percent_min
    if (threshold == null) return 'normal'

    // SLA is "higher is better":
    // - below threshold => critical
    // - within +0.1% buffer => warning
    const warningBuffer = 0.1

    if (slaPercent < threshold) return 'critical'
    if (slaPercent < threshold + warningBuffer) return 'warning'
    return 'normal'
  }

  function getTTFTThresholdLevel(ttftMs: number | null): ThresholdLevel {
    if (ttftMs == null) return 'normal'
    const threshold = options.thresholds()?.ttft_p99_ms_max
    if (threshold == null) return 'normal'
    if (ttftMs >= threshold) return 'critical'
    if (ttftMs >= threshold * 0.8) return 'warning'
    return 'normal'
  }

  function getRequestErrorRateThresholdLevel(errorRatePercent: number | null): ThresholdLevel {
    if (errorRatePercent == null) return 'normal'
    const threshold = options.thresholds()?.request_error_rate_percent_max
    if (threshold == null) return 'normal'
    if (errorRatePercent >= threshold) return 'critical'
    if (errorRatePercent >= threshold * 0.8) return 'warning'
    return 'normal'
  }

  function getUpstreamErrorRateThresholdLevel(upstreamErrorRatePercent: number | null): ThresholdLevel {
    if (upstreamErrorRatePercent == null) return 'normal'
    const threshold = options.thresholds()?.upstream_error_rate_percent_max
    if (threshold == null) return 'normal'
    if (upstreamErrorRatePercent >= threshold) return 'critical'
    if (upstreamErrorRatePercent >= threshold * 0.8) return 'warning'
    return 'normal'
  }

  function getThresholdColorClass(level: ThresholdLevel): string {
    switch (level) {
      case 'critical':
        return 'text-danger-text '
      case 'warning':
        return 'text-warning-text '
      default:
        return 'text-success-text '
    }
  }

  return {
    getSLAThresholdLevel,
    getTTFTThresholdLevel,
    getRequestErrorRateThresholdLevel,
    getUpstreamErrorRateThresholdLevel,
    getThresholdColorClass
  }
}
