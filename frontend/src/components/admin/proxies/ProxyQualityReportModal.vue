<template>
  <BaseDialog
    :show="open"
    :title="t('admin.proxies.qualityReportTitle')"
    width="normal"
    @close="$emit('close')"
  >
    <div v-if="report" class="space-y-4">
      <div class="rounded-lg border border-line bg-surface-2 p-4">
        <div class="flex items-center justify-between gap-4">
          <div>
            <div class="text-sm text-muted">
              {{ proxy?.name || '-' }}
            </div>
            <div class="mt-1 text-sm text-foreground">
              {{ report.summary }}
            </div>
          </div>
          <div class="text-right">
            <div class="text-2xl font-semibold text-foreground">
              {{ report.score }}
            </div>
            <div class="text-xs text-muted">
              {{ t('admin.proxies.qualityGrade', { grade: report.grade }) }}
            </div>
          </div>
        </div>
        <div class="mt-3 grid grid-cols-2 gap-2 text-xs text-muted">
          <div>{{ t('admin.proxies.qualityExitIP') }}: {{ report.exit_ip || '-' }}</div>
          <div>{{ t('admin.proxies.qualityCountry') }}: {{ report.country || '-' }}</div>
          <div>
            {{ t('admin.proxies.qualityBaseLatency') }}:
            {{ typeof report.base_latency_ms === 'number' ? `${report.base_latency_ms}ms` : '-' }}
          </div>
          <div>
            {{ t('admin.proxies.qualityCheckedAt') }}:
            {{ new Date(report.checked_at * 1000).toLocaleString() }}
          </div>
        </div>
      </div>

      <div class="max-h-80 overflow-auto rounded-lg border border-line">
        <table class="min-w-full divide-y divide-line text-sm">
          <thead class="bg-surface-2 text-xs uppercase text-muted">
            <tr>
              <th class="whitespace-nowrap px-3 py-2 text-left">
                {{ t('admin.proxies.qualityTableTarget') }}
              </th>
              <th class="whitespace-nowrap px-3 py-2 text-left">
                {{ t('admin.proxies.qualityTableStatus') }}
              </th>
              <th class="whitespace-nowrap px-3 py-2 text-left">HTTP</th>
              <th class="whitespace-nowrap px-3 py-2 text-left">
                {{ t('admin.proxies.qualityTableLatency') }}
              </th>
              <th class="px-3 py-2 text-left">{{ t('admin.proxies.qualityTableMessage') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-line bg-surface">
            <tr v-for="item in report.items" :key="item.target">
              <td class="whitespace-nowrap px-3 py-2 text-foreground">
                {{ qualityTargetLabel(item.target) }}
              </td>
              <td class="whitespace-nowrap px-3 py-2">
                <span class="badge whitespace-nowrap" :class="qualityStatusClass(item.status)">
                  {{ qualityStatusLabel(item.status) }}
                </span>
              </td>
              <td class="whitespace-nowrap px-3 py-2 text-muted">{{ item.http_status ?? '-' }}</td>
              <td class="whitespace-nowrap px-3 py-2 text-muted">
                {{ typeof item.latency_ms === 'number' ? `${item.latency_ms}ms` : '-' }}
              </td>
              <td class="px-3 py-2 text-muted">
                <span>{{ item.message || '-' }}</span>
                <span v-if="item.cf_ray" class="ml-1 text-xs text-muted">(cf-ray: {{ item.cf_ray }})</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    <template #footer>
      <div class="flex justify-end">
        <button type="button" class="btn-glass-secondary" @click="$emit('close')">
          {{ t('common.close') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { Proxy, ProxyQualityCheckResult } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'

defineProps<{
  open: boolean
  proxy: Proxy | null
  report: ProxyQualityCheckResult | null
}>()

defineEmits<{
  close: []
}>()

const { t } = useI18n()

const qualityStatusClass = (status: string) => {
  if (status === 'pass') return 'badge-success'
  if (status === 'warn') return 'badge-warning'
  if (status === 'challenge') return 'badge-danger'
  return 'badge-danger'
}

const qualityStatusLabel = (status: string) => {
  if (status === 'pass') return t('admin.proxies.qualityStatusPass')
  if (status === 'warn') return t('admin.proxies.qualityStatusWarn')
  if (status === 'challenge') return t('admin.proxies.qualityStatusChallenge')
  return t('admin.proxies.qualityStatusFail')
}

const qualityTargetLabel = (target: string) => {
  switch (target) {
    case 'base_connectivity':
      return t('admin.proxies.qualityTargetBase')
    case 'openai':
      return 'OpenAI'
    case 'anthropic':
      return 'Anthropic'
    case 'gemini':
      return 'Gemini'
    case 'grok':
      return 'Grok'
    default:
      return target
  }
}
</script>
