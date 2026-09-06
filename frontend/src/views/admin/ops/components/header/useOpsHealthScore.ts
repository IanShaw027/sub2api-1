import { computed } from 'vue'
import type { OpsDashboardOverview } from '@/api/admin/ops'

export interface DiagnosisItem {
  type: 'critical' | 'warning' | 'info'
  message: string
  impact: string
  action?: string
}

export interface UseOpsHealthScoreOptions {
  overview: () => OpsDashboardOverview | null
  fullscreen: () => boolean | undefined
  t: (key: string, params?: Record<string, unknown>) => string
}

export function useOpsHealthScore(options: UseOpsHealthScoreOptions) {
  const { t } = options

  const isSystemIdle = computed(() => {
    const ov = options.overview()
    if (!ov) return true
    const qps = ov.qps?.current
    const errorRate = ov.error_rate ?? 0
    return (qps ?? 0) === 0 && errorRate === 0
  })

  const healthScoreValue = computed<number | null>(() => {
    const v = options.overview()?.health_score
    return typeof v === 'number' && Number.isFinite(v) ? v : null
  })

  const healthScoreColor = computed(() => {
    if (isSystemIdle.value) return 'var(--muted)'
    const score = healthScoreValue.value
    if (score == null) return 'var(--muted)'
    if (score >= 90) return 'var(--success)'
    if (score >= 60) return 'var(--warning)'
    return 'var(--danger)'
  })

  const healthScoreClass = computed(() => {
    if (isSystemIdle.value) return 'text-muted'
    const score = healthScoreValue.value
    if (score == null) return 'text-muted'
    if (score >= 90) return 'text-success-500'
    if (score >= 60) return 'text-warning-500'
    return 'text-danger-500'
  })

  const circleSize = computed(() => options.fullscreen() ? 140 : 100)
  const strokeWidth = computed(() => options.fullscreen() ? 10 : 8)
  const radius = computed(() => (circleSize.value - strokeWidth.value) / 2)
  const circumference = computed(() => 2 * Math.PI * radius.value)
  const dashOffset = computed(() => {
    if (isSystemIdle.value) return 0
    if (healthScoreValue.value == null) return 0
    const score = Math.max(0, Math.min(100, healthScoreValue.value))
    return circumference.value - (score / 100) * circumference.value
  })

  const diagnosisReport = computed<DiagnosisItem[]>(() => {
    const ov = options.overview()
    if (!ov) return []

    const report: DiagnosisItem[] = []

    if (isSystemIdle.value) {
      report.push({
        type: 'info',
        message: t('admin.ops.diagnosis.idle'),
        impact: t('admin.ops.diagnosis.idleImpact')
      })
      return report
    }

    // Resource diagnostics (highest priority)
    const sm = ov.system_metrics
    if (sm) {
      if (sm.db_ok === false) {
        report.push({
          type: 'critical',
          message: t('admin.ops.diagnosis.dbDown'),
          impact: t('admin.ops.diagnosis.dbDownImpact'),
          action: t('admin.ops.diagnosis.dbDownAction')
        })
      }
      if (sm.redis_ok === false) {
        report.push({
          type: 'warning',
          message: t('admin.ops.diagnosis.redisDown'),
          impact: t('admin.ops.diagnosis.redisDownImpact'),
          action: t('admin.ops.diagnosis.redisDownAction')
        })
      }

      const cpuPct = sm.cpu_usage_percent ?? 0
      if (cpuPct > 90) {
        report.push({
          type: 'critical',
          message: t('admin.ops.diagnosis.cpuCritical', { usage: cpuPct.toFixed(1) }),
          impact: t('admin.ops.diagnosis.cpuCriticalImpact'),
          action: t('admin.ops.diagnosis.cpuCriticalAction')
        })
      } else if (cpuPct > 80) {
        report.push({
          type: 'warning',
          message: t('admin.ops.diagnosis.cpuHigh', { usage: cpuPct.toFixed(1) }),
          impact: t('admin.ops.diagnosis.cpuHighImpact'),
          action: t('admin.ops.diagnosis.cpuHighAction')
        })
      }

      const memPct = sm.memory_usage_percent ?? 0
      if (memPct > 90) {
        report.push({
          type: 'critical',
          message: t('admin.ops.diagnosis.memoryCritical', { usage: memPct.toFixed(1) }),
          impact: t('admin.ops.diagnosis.memoryCriticalImpact'),
          action: t('admin.ops.diagnosis.memoryCriticalAction')
        })
      } else if (memPct > 85) {
        report.push({
          type: 'warning',
          message: t('admin.ops.diagnosis.memoryHigh', { usage: memPct.toFixed(1) }),
          impact: t('admin.ops.diagnosis.memoryHighImpact'),
          action: t('admin.ops.diagnosis.memoryHighAction')
        })
      }
    }

    const ttftP99 = ov.ttft?.p99_ms ?? 0
    if (ttftP99 > 500) {
      report.push({
        type: 'warning',
        message: t('admin.ops.diagnosis.ttftHigh', { ttft: ttftP99.toFixed(0) }),
        impact: t('admin.ops.diagnosis.ttftHighImpact'),
        action: t('admin.ops.diagnosis.ttftHighAction')
      })
    }

    // Error rate diagnostics (adjusted thresholds)
    const upstreamRatePct = (ov.upstream_error_rate ?? 0) * 100
    if (upstreamRatePct > 5) {
      report.push({
        type: 'critical',
        message: t('admin.ops.diagnosis.upstreamCritical', { rate: upstreamRatePct.toFixed(2) }),
        impact: t('admin.ops.diagnosis.upstreamCriticalImpact'),
        action: t('admin.ops.diagnosis.upstreamCriticalAction')
      })
    } else if (upstreamRatePct > 2) {
      report.push({
        type: 'warning',
        message: t('admin.ops.diagnosis.upstreamHigh', { rate: upstreamRatePct.toFixed(2) }),
        impact: t('admin.ops.diagnosis.upstreamHighImpact'),
        action: t('admin.ops.diagnosis.upstreamHighAction')
      })
    }

    const errorPct = (ov.error_rate ?? 0) * 100
    if (errorPct > 3) {
      report.push({
        type: 'critical',
        message: t('admin.ops.diagnosis.errorHigh', { rate: errorPct.toFixed(2) }),
        impact: t('admin.ops.diagnosis.errorHighImpact'),
        action: t('admin.ops.diagnosis.errorHighAction')
      })
    } else if (errorPct > 0.5) {
      report.push({
        type: 'warning',
        message: t('admin.ops.diagnosis.errorElevated', { rate: errorPct.toFixed(2) }),
        impact: t('admin.ops.diagnosis.errorElevatedImpact'),
        action: t('admin.ops.diagnosis.errorElevatedAction')
      })
    }

    // SLA diagnostics
    const slaPct = (ov.sla ?? 0) * 100
    if (slaPct < 90) {
      report.push({
        type: 'critical',
        message: t('admin.ops.diagnosis.slaCritical', { sla: slaPct.toFixed(2) }),
        impact: t('admin.ops.diagnosis.slaCriticalImpact'),
        action: t('admin.ops.diagnosis.slaCriticalAction')
      })
    } else if (slaPct < 98) {
      report.push({
        type: 'warning',
        message: t('admin.ops.diagnosis.slaLow', { sla: slaPct.toFixed(2) }),
        impact: t('admin.ops.diagnosis.slaLowImpact'),
        action: t('admin.ops.diagnosis.slaLowAction')
      })
    }

    // Health score diagnostics (lowest priority)
    if (healthScoreValue.value != null) {
      if (healthScoreValue.value < 60) {
        report.push({
          type: 'critical',
          message: t('admin.ops.diagnosis.healthCritical', { score: healthScoreValue.value }),
          impact: t('admin.ops.diagnosis.healthCriticalImpact'),
          action: t('admin.ops.diagnosis.healthCriticalAction')
        })
      } else if (healthScoreValue.value < 90) {
        report.push({
          type: 'warning',
          message: t('admin.ops.diagnosis.healthLow', { score: healthScoreValue.value }),
          impact: t('admin.ops.diagnosis.healthLowImpact'),
          action: t('admin.ops.diagnosis.healthLowAction')
        })
      }
    }

    if (report.length === 0) {
      report.push({
        type: 'info',
        message: t('admin.ops.diagnosis.healthy'),
        impact: t('admin.ops.diagnosis.healthyImpact')
      })
    }

    return report
  })

  return {
    isSystemIdle,
    healthScoreValue,
    healthScoreColor,
    healthScoreClass,
    circleSize,
    strokeWidth,
    radius,
    circumference,
    dashOffset,
    diagnosisReport
  }
}
