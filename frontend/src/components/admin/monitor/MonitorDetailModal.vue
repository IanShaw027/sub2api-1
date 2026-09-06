<template>
  <BaseDialog
    :show="show"
    :title="t('admin.channelMonitor.detail.title', { name: monitor?.name ?? '' })"
    width="wide"
    @close="$emit('close')"
  >
    <div v-if="loading" class="py-8 text-center text-sm text-muted">
      {{ t('common.loading') }}
    </div>
    <EmptyState
      v-else-if="modelGroups.length === 0"
      :title="t('admin.channelMonitor.detail.noHistory')"
    />
    <div v-else class="monitor-detail-grid">
      <GlassCard v-for="group in modelGroups" :key="group.model" padding="sm">
        <div class="flex items-center justify-between gap-2">
          <span class="truncate text-sm font-medium text-foreground">{{ formatMonitorModel(group.model) }}</span>
          <StatusCell
            :status="group.latest.status"
            :tone="statusTone(group.latest.status)"
            :label="statusLabel(group.latest.status)"
            dot
            :pulse="group.latest.status === 'operational'"
          />
        </div>
        <div class="mt-3 flex items-end justify-between gap-3">
          <div class="min-w-0 text-xs text-muted">
            <div class="text-foreground">{{ formatLatency(group.latest.latency_ms) }} ms</div>
            <div class="mt-1">{{ t('admin.channelMonitor.detail.latestCheck', { time: formatRelativeTime(group.latest.checked_at) }) }}</div>
            <div class="mt-0.5">{{ t('admin.channelMonitor.detail.checksCount', { n: group.entries.length }) }}</div>
          </div>
          <div class="monitor-detail-spark">
            <MonitorHistorySparkline :values="group.latencies" :tone="statusTone(group.latest.status)" />
          </div>
        </div>
      </GlassCard>
    </div>
    <template #footer>
      <div class="flex justify-end">
        <button type="button" class="btn btn-primary" @click="$emit('close')">
          {{ t('common.close') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
/**
 * Detail modal for a single channel monitor: a glass-card grid, one card per
 * model (primary first, then extra models), each showing the latest status
 * as a dot+pulse badge plus a 96x28 latency sparkline built from real
 * /admin/channel-monitors/:id/history data. Legacy-tab only (task 11.6);
 * the v2 tab (features/channel-monitor-v2/) is out of this task's scope.
 */
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { channelMonitorAPI } from '@/api/admin/channelMonitor'
import type { ChannelMonitor, HistoryItem } from '@/api/admin/channelMonitor'
import BaseDialog from '@/components/common/BaseDialog.vue'
import GlassCard from '@/components/ui/GlassCard.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import { StatusCell } from '@/components/common/cells'
import { monitorStatusTone as statusTone } from './monitorStatusTone'
import MonitorHistorySparkline from './MonitorHistorySparkline.vue'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'

const HISTORY_LIMIT = 100

const props = defineProps<{
  show: boolean
  monitor: ChannelMonitor | null
}>()

defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()
const { statusLabel, formatLatency, formatMonitorModel, formatRelativeTime } = useChannelMonitorFormat()

const loading = ref(false)
const history = ref<HistoryItem[]>([])

interface ModelGroup {
  model: string
  latest: HistoryItem
  entries: HistoryItem[]
  latencies: (number | null)[]
}

const modelGroups = computed<ModelGroup[]>(() => {
  const byModel = new Map<string, HistoryItem[]>()
  for (const item of history.value) {
    const list = byModel.get(item.model) ?? []
    list.push(item)
    byModel.set(item.model, list)
  }

  const groups: ModelGroup[] = []
  for (const [model, items] of byModel) {
    // History API returns newest-first; sort ascending so the sparkline
    // reads left(oldest) to right(latest), matching DashSparkline's convention.
    const ascending = [...items].sort((a, b) => Date.parse(a.checked_at) - Date.parse(b.checked_at))
    groups.push({
      model,
      latest: ascending[ascending.length - 1],
      entries: ascending,
      latencies: ascending.map(entry => entry.latency_ms),
    })
  }

  return groups.sort((a, b) => {
    if (props.monitor) {
      if (a.model === props.monitor.primary_model) return -1
      if (b.model === props.monitor.primary_model) return 1
    }
    return a.model.localeCompare(b.model)
  })
})

async function loadHistory() {
  if (!props.monitor) return
  loading.value = true
  history.value = []
  try {
    const res = await channelMonitorAPI.listHistory(props.monitor.id, { limit: HISTORY_LIMIT })
    history.value = res.items || []
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.detail.loadError')))
  } finally {
    loading.value = false
  }
}

watch(() => props.show, (val) => {
  if (val) void loadHistory()
})
</script>

<style scoped>
.monitor-detail-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 12px;
}

.monitor-detail-spark {
  flex: none;
  width: 96px;
  height: 28px;
}
</style>
