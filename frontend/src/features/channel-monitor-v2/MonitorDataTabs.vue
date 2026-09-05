<template>
  <section class="glass-card flex min-h-0 flex-col overflow-hidden">
    <div class="border-b border-line px-5 pt-4 sm:px-6">
      <SegmentedControl
        :model-value="activeTab"
        :options="tabs"
        :aria-label="t('channelMonitorV2.tabs.aria')"
        @update:model-value="emit('update:activeTab', $event)"
      />
    </div>
    <div class="min-h-0 max-h-[min(52vh,520px)] overflow-auto p-4 sm:p-5">
      <div v-if="activeTab === 'models'" class="table-container border-0">
        <table class="table monitor-table min-w-[720px]">
          <thead>
            <tr>
              <th>{{ t('channelMonitorV2.table.platformModel') }}</th>
              <th>{{ t('channelMonitorV2.metrics.successRate') }}</th>
              <th>{{ t('channelMonitorV2.metrics.ttftP50') }}</th>
              <th v-if="showThroughput">{{ t('channelMonitorV2.metrics.tps') }}</th>
              <th>{{ t('channelMonitorV2.metrics.cacheRate') }}</th>
              <th v-if="showThroughput">{{ t('channelMonitorV2.metrics.rpm') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="row in modelRows"
              :key="`${row.platform}:${row.model}`"
              class="cursor-pointer"
              @click="emit('drillModel', row)"
            >
              <td>
                <div class="flex items-center gap-2">
                  <span :class="statusDot(row.health)" aria-hidden="true"></span>
                  <div>
                    <span class="block text-xs text-muted">{{ row.platform }}</span>
                    <strong class="font-semibold text-foreground">
                      {{ row.model === '__other__' ? t('channelMonitorV2.otherModels') : row.model }}
                    </strong>
                  </div>
                </div>
              </td>
              <td class="tabular-nums">
                <span class="block">{{ formatPercent(1 - row.metrics.error_rate) }}</span>
                <small class="text-xs text-muted">{{ t('channelMonitorV2.metrics.errorRateValue', { value: formatPercent(row.metrics.error_rate) }) }}</small>
              </td>
              <td class="tabular-nums">
                <span class="block">{{ formatMs(row.metrics.ttft.p50_ms) }}</span>
                <small class="text-xs text-muted">{{ latencyDetail(row.metrics.ttft) }}</small>
              </td>
              <td v-if="showThroughput" class="tabular-nums" :title="exactTps(row.metrics.tpm)">{{ formatTps(row.metrics.tpm) }}</td>
              <td class="tabular-nums">{{ formatPercent(row.metrics.cache_rate) }}</td>
              <td v-if="showThroughput" class="tabular-nums">{{ formatRate(row.metrics.rpm) }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-else-if="activeTab === 'errors'" class="space-y-3">
        <div
          v-for="row in errorRows"
          :key="row.category"
          class="rounded-[var(--radius-card)] bg-surface-2 p-4 text-sm"
          :class="row.ignored ? 'opacity-60' : ''"
        >
          <button
            type="button"
            class="grid w-full grid-cols-[minmax(100px,200px)_1fr_auto_auto] items-center gap-3 text-left"
            @click="emit('toggleError', row.category)"
          >
            <span class="flex min-w-0 items-center gap-1.5 truncate text-foreground">
              <span class="truncate">{{ errorLabel(row.category) }}</span>
              <span v-if="row.ignored" class="badge badge-gray shrink-0 !px-1.5 !py-0 text-[10px]">{{ t('channelMonitorV2.ignored') }}</span>
            </span>
            <span class="h-2 overflow-hidden rounded-full bg-surface-3">
              <i
                class="block h-full rounded-full"
                :class="row.ignored ? 'bg-muted' : 'bg-danger'"
                :style="{ width: `${Math.max(2, row.rate * 100)}%` }"
              ></i>
            </span>
            <small class="w-14 text-right text-xs tabular-nums text-muted">{{ formatPercent(row.rate) }}</small>
            <Icon name="chevronDown" size="sm" :class="['text-muted transition-transform', expandedErrors.has(row.category) ? 'rotate-180' : '']" />
          </button>
          <div v-if="expandedErrors.has(row.category)" class="mt-3 space-y-2 border-t border-line pt-3">
            <template v-if="isAdmin && (row.details || []).length">
              <div
                v-for="(detail, index) in row.details || []"
                :key="`${row.category}:${index}:${detail.message}`"
                class="rounded-lg bg-surface-2 px-3 py-2 text-xs text-muted"
              >
                <div class="mb-1 flex flex-wrap items-center gap-2">
                  <span class="badge badge-gray !px-1.5 !py-0 text-[10px]">{{ detail.platform || '-' }}</span>
                  <span class="truncate font-medium">{{ detail.model || '-' }}</span>
                  <span v-if="detail.status_code" class="text-muted">{{ t('channelMonitorV2.errorDetail.http', { code: detail.status_code }) }}</span>
                  <span v-if="detail.upstream_status_code" class="text-muted">{{ t('channelMonitorV2.errorDetail.upstream', { code: detail.upstream_status_code }) }}</span>
                  <span class="ml-auto text-muted">×{{ detail.count }}</span>
                </div>
                <p class="break-words leading-relaxed">{{ detail.message || detail.error_type || t('channelMonitorV2.errorDetail.noMessage') }}</p>
              </div>
            </template>
            <p v-else class="text-xs text-muted">{{ t('channelMonitorV2.errorDetail.empty') }}</p>
          </div>
        </div>
      </div>

      <div v-else class="table-container border-0">
        <table class="table monitor-table min-w-[640px]">
          <thead>
            <tr>
              <th class="w-16">{{ t('channelMonitorV2.table.rank') }}</th>
              <th>{{ t('channelMonitorV2.table.user') }}</th>
              <th>{{ t('channelMonitorV2.metrics.successRate') }}</th>
              <th>{{ t('channelMonitorV2.metrics.ttftP50') }}</th>
              <th v-if="showThroughput">{{ t('channelMonitorV2.metrics.tps') }}</th>
              <th>{{ t('channelMonitorV2.metrics.cacheRate') }}</th>
              <th v-if="showThroughput">{{ t('channelMonitorV2.metrics.rpm') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="row in userRows"
              :key="row.user_id || row.display_label"
              :class="row.is_self
                ? 'bg-surface-2 ring-1 ring-inset ring-accent'
                : ''"
            >
              <td><MonitorRankBadge :rank="row.rank" /></td>
              <td>
                <strong
                  class="font-semibold"
                  :class="row.is_self ? 'text-accent' : 'text-foreground'"
                >
                  {{ row.display_label }}
                  <span
                    v-if="row.is_self"
                    class="badge badge-primary ml-2 !px-1.5 !py-0 text-[10px]"
                  >{{ t('channelMonitorV2.currentUser') }}</span>
                </strong>
              </td>
              <td class="tabular-nums">
                <span class="block">{{ formatPercent(1 - row.metrics.error_rate) }}</span>
                <small class="text-xs text-muted">{{ t('channelMonitorV2.metrics.errorRateValue', { value: formatPercent(row.metrics.error_rate) }) }}</small>
              </td>
              <td class="tabular-nums">
                <span class="block">{{ formatMs(row.metrics.ttft.p50_ms) }}</span>
                <small class="text-xs text-muted">{{ latencyDetail(row.metrics.ttft) }}</small>
              </td>
              <td v-if="showThroughput" class="tabular-nums" :title="exactTps(row.metrics.tpm)">{{ formatTps(row.metrics.tpm) }}</td>
              <td class="tabular-nums">{{ formatPercent(row.metrics.cache_rate) }}</td>
              <td v-if="showThroughput" class="tabular-nums">{{ formatRate(row.metrics.rpm) }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="tabLoading" class="empty-state py-10 text-sm text-muted">{{ t('common.loading') }}</div>
      <div v-else-if="activeRowsEmpty" class="empty-state py-10">
        <p class="empty-state-title text-base">
          {{
            bootstrapActive
              ? t('channelMonitorV2.bootstrap.title')
              : t('channelMonitorV2.empty.title')
          }}
        </p>
        <p class="empty-state-description">
          {{
            bootstrapActive
              ? t('channelMonitorV2.bootstrap.description')
              : t('channelMonitorV2.empty.description')
          }}
        </p>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import SegmentedControl from '@/components/ui/SegmentedControl.vue'
import MonitorRankBadge from '@/features/channel-monitor-v2/MonitorRankBadge.vue'
import type { MonitorErrorRow, MonitorHealth, MonitorModelRow, MonitorUserRow, HealthState } from '@/api/channelMonitorV2'
import type { MonitorTab } from '@/features/channel-monitor-v2/useChannelMonitorV2'

defineProps<{
  activeTab: MonitorTab
  tabs: { value: MonitorTab; label: string }[]
  modelRows: MonitorModelRow[]
  errorRows: MonitorErrorRow[]
  userRows: MonitorUserRow[]
  showThroughput: boolean
  isAdmin: boolean
  tabLoading: boolean
  activeRowsEmpty: boolean
  bootstrapActive: boolean
  expandedErrors: Set<string>
  formatPercent: (value: number) => string
  formatMs: (value: number | null) => string
  formatTps: (tpm: number | null | undefined) => string
  exactTps: (tpm: number | null | undefined) => string
  formatRate: (value: number) => string
  latencyDetail: (metric: { p50_ms: number | null; p90_ms?: number | null; p95_ms: number | null; avg_ms?: number | null }) => string
  statusDot: (health?: MonitorHealth | HealthState) => string
  errorLabel: (value: string) => string
}>()

const emit = defineEmits<{
  'update:activeTab': [value: MonitorTab]
  drillModel: [row: MonitorModelRow]
  toggleError: [category: string]
}>()

const { t } = useI18n()
</script>

<style scoped>
.monitor-table td {
  height: 58px;
  padding-top: 10px;
  padding-bottom: 10px;
  /* two-line "value + detail" cells fit the ≤61px ListPage row */
  line-height: 1.3;
}
.status-dot {
  display: inline-block;
  height: 0.5rem;
  width: 0.5rem;
  flex: none;
  border-radius: 9999px;
}

/* Multi-stop green -> yellow -> red (score10 best ... score0 worst).
   Token-composed via color-mix() over --success/--warning/--danger so the
   whole gradient stays theme- and accent-reactive; zero literal hex.
   Kept in sync with RelayPulseMatrix.vue's identical block. */
.health-score10 { background: var(--success); }
.health-score9 { background: color-mix(in oklch, var(--success) 85%, var(--warning) 15%); }
.health-score8 { background: color-mix(in oklch, var(--success) 70%, var(--warning) 30%); }
.health-score7 { background: color-mix(in oklch, var(--success) 45%, var(--warning) 55%); }
.health-score6 { background: color-mix(in oklch, var(--warning) 85%, var(--success) 15%); }
.health-score5 { background: var(--warning); }
.health-score4 { background: color-mix(in oklch, var(--warning) 80%, var(--danger) 20%); }
.health-score3 { background: color-mix(in oklch, var(--warning) 55%, var(--danger) 45%); }
.health-score2 { background: color-mix(in oklch, var(--warning) 30%, var(--danger) 70%); }
.health-score1 { background: color-mix(in oklch, var(--danger) 85%, var(--warning) 15%); }
.health-score0 { background: var(--danger); }
/* Coarse fallbacks (older payloads without score) */
.health-healthy { background: var(--success); }
.health-warning { background: var(--warning); }
.health-critical { background: var(--danger); }
.health-unknown { background: var(--muted); }

details > summary::-webkit-details-marker {
  display: none;
}
</style>
