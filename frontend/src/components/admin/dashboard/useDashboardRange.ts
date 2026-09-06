/** Dashboard date-range + granularity state, and the auto-granularity-select handler for range changes. */
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

const formatLocalDate = (date: Date): string => {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

const getLast24HoursRangeDates = (): { start: string; end: string } => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return {
    start: formatLocalDate(start),
    end: formatLocalDate(end)
  }
}

/**
 * Date-range + granularity state for the dashboard. `applyRangeGranularity`
 * only updates the granularity for a newly-picked range (pure state update);
 * the caller composes the actual `onDateRangeChange` handler together with
 * whatever reload it needs to trigger, so this composable has no dependency
 * on the stats-loading composable.
 */
export function useDashboardRange() {
  const { t } = useI18n()

  const granularity = ref<'day' | 'hour'>('hour')
  const defaultRange = getLast24HoursRangeDates()
  const startDate = ref(defaultRange.start)
  const endDate = ref(defaultRange.end)

  const granularityOptions = computed(() => [
    { value: 'day', label: t('admin.dashboard.day') },
    { value: 'hour', label: t('admin.dashboard.hour') }
  ])

  const applyRangeGranularity = (range: {
    startDate: string
    endDate: string
    preset: string | null
  }) => {
    // Auto-select granularity based on date range
    const start = new Date(range.startDate)
    const end = new Date(range.endDate)
    const daysDiff = Math.ceil((end.getTime() - start.getTime()) / (1000 * 60 * 60 * 24))

    // If range is 1 day, use hourly granularity
    if (daysDiff <= 1) {
      granularity.value = 'hour'
    } else {
      granularity.value = 'day'
    }
  }

  return {
    granularity,
    startDate,
    endDate,
    granularityOptions,
    applyRangeGranularity
  }
}
