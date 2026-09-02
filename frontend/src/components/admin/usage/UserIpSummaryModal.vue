<template>
  <BaseDialog
    :show="show"
    :title="title"
    width="wide"
    @close="$emit('close')"
  >
    <div class="space-y-4">
      <div class="flex flex-wrap items-center gap-3 text-sm text-muted">
        <span v-if="summary?.top_ip" class="font-mono">
          Top IP: <strong>{{ summary.top_ip }}</strong>
        </span>
        <span
          class="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium ring-1 ring-inset"
          :class="summary?.pin_known_ips
 ? 'bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] text-amber-700 ring-amber-200'
 : 'bg-surface-2 text-muted ring-line'"
        >
          {{ summary?.pin_known_ips ? t('admin.usage.ipPin.enabled') : t('admin.usage.ipPin.disabled') }}
        </span>
        <button type="button" class="btn btn-secondary btn-xs" :disabled="loading || pinning" @click="togglePin">
          {{ summary?.pin_known_ips ? t('admin.usage.ipPin.disable') : t('admin.usage.ipPin.enable') }}
        </button>
      </div>

      <div v-if="loading" class="py-8 text-center text-sm text-muted">{{ t('common.loading') }}</div>
      <div v-else-if="!summary?.items?.length" class="py-8 text-center text-sm text-muted">
        {{ t('admin.usage.ipSummaryEmpty') }}
      </div>
      <div v-else class="overflow-x-auto rounded border border-line">
        <table class="min-w-full text-left text-sm">
          <thead class="bg-surface-2">
            <tr>
              <th class="px-3 py-2">IP</th>
              <th class="px-3 py-2">{{ t('admin.usage.ipRequests') }}</th>
              <th class="px-3 py-2">{{ t('admin.usage.ipCost') }}</th>
              <th class="px-3 py-2">{{ t('common.status') }}</th>
              <th class="px-3 py-2">{{ t('admin.usage.sharedUsers') }}</th>
              <th class="px-3 py-2"></th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="item in summary.items"
              :key="item.ip_address"
              class="border-t border-line"
              :class="item.is_top ? 'bg-[color-mix(in_oklch,var(--accent)_12%,transparent)]/40' : ''"
            >
              <td class="px-3 py-2 font-mono text-xs">
                {{ item.ip_address }}
                <span v-if="item.is_top" class="ml-1 text-[10px] text-accent">TOP</span>
              </td>
              <td class="px-3 py-2 tabular-nums">{{ item.request_count.toLocaleString() }}</td>
              <td class="px-3 py-2 tabular-nums">${{ item.total_cost.toFixed(2) }}</td>
              <td class="px-3 py-2">
                <span class="inline-flex rounded px-1.5 py-0.5 text-[11px] font-medium ring-1 ring-inset" :class="statusClass(item)">
                  {{ statusLabel(item) }}
                </span>
              </td>
              <td class="px-3 py-2 text-xs text-muted">
                <template v-if="item.shared_users?.length">
                  <div v-for="u in item.shared_users" :key="u.id" class="truncate" :title="u.email">
                    #{{ u.id }} {{ u.email }} ({{ u.request_count }})
                  </div>
                </template>
                <span v-else class="text-muted">-</span>
              </td>
              <td class="px-3 py-2 text-right whitespace-nowrap">
                <button
                  v-if="item.ban_status !== 'active'"
                  type="button"
                  class="btn btn-danger btn-xs"
                  :disabled="actingIp === item.ip_address"
                  @click="banIp(item.ip_address)"
                >
                  {{ t('admin.usage.banIp') }}
                </button>
                <button
                  v-else
                  type="button"
                  class="btn btn-secondary btn-xs"
                  :disabled="actingIp === item.ip_address"
                  @click="releaseIp(item.ip_address)"
                >
                  {{ t('admin.usage.releaseIp') }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI } from '@/api/admin'
import ipSecurityAPI from '@/api/admin/ipSecurity'
import { useAppStore } from '@/stores/app'
import type { UserIPSummary, UserIPSummaryItem } from '@/api/admin/users'

const props = defineProps<{
  show: boolean
  userId: number | null
  userEmail?: string
  days?: number
}>()
const emit = defineEmits<{ close: []; changed: [] }>()

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const pinning = ref(false)
const actingIp = ref('')
const summary = ref<UserIPSummary | null>(null)

const title = computed(() => {
  const email = props.userEmail || `#${props.userId || ''}`
  return t('admin.usage.ipSummaryTitle', { user: email })
})

async function load() {
  if (!props.userId) return
  loading.value = true
  try {
    summary.value = await adminAPI.users.getIpSummary(props.userId, { days: props.days ?? 30 })
  } catch (e: any) {
    appStore.showError(e?.message || t('admin.usage.ipSummaryLoadFailed'))
    summary.value = null
  } finally {
    loading.value = false
  }
}

watch(() => [props.show, props.userId], ([show]) => {
  if (show) void load()
})

function statusLabel(item: UserIPSummaryItem): string {
  if (item.ban_status === 'active') {
    if (item.ban_reason?.startsWith('pinned-user')) return t('admin.usage.ipStatus.pinnedBan')
    if (item.ban_reason?.startsWith('manual:')) return t('admin.usage.ipStatus.manualBan')
    return t('admin.usage.ipStatus.autoBan')
  }
  if (item.ban_status === 'whitelisted') return t('admin.usage.ipStatus.whitelisted')
  if (item.ban_status === 'released') return t('admin.usage.ipStatus.released')
  return t('admin.usage.ipStatus.normal')
}

function statusClass(item: UserIPSummaryItem): string {
  if (item.ban_status === 'active') return 'bg-rose-50 text-rose-700 ring-rose-200'
  if (item.ban_status === 'whitelisted') return 'bg-[color-mix(in_oklch,var(--success)_16%,transparent)] text-success-text ring-emerald-200'
  if (item.ban_status === 'released') return 'bg-slate-50 text-slate-600 ring-slate-200'
  return 'bg-surface-2 text-muted ring-line'
}

async function togglePin() {
  if (!props.userId || !summary.value) return
  const enable = !summary.value.pin_known_ips
  if (enable && !window.confirm(t('admin.usage.ipPin.confirmEnable'))) return
  pinning.value = true
  try {
    await adminAPI.users.setIpPin(props.userId, enable)
    await load()
    emit('changed')
    appStore.showSuccess(enable ? t('admin.usage.ipPin.enabledToast') : t('admin.usage.ipPin.disabledToast'))
  } catch (e: any) {
    appStore.showError(e?.message || t('admin.usage.ipPin.failed'))
  } finally {
    pinning.value = false
  }
}

async function banIp(ip: string) {
  actingIp.value = ip
  try {
    await ipSecurityAPI.createBan({ ip_address: ip, reason: `manual: usage console ban (user #${props.userId})` })
    await load()
    emit('changed')
    appStore.showSuccess(t('admin.usage.banIpSuccess'))
  } catch (e: any) {
    appStore.showError(e?.message || t('admin.usage.banIpFailed'))
  } finally {
    actingIp.value = ''
  }
}

async function releaseIp(ip: string) {
  actingIp.value = ip
  try {
    await ipSecurityAPI.releaseBanByIP(ip)
    await load()
    emit('changed')
    appStore.showSuccess(t('admin.usage.releaseIpSuccess'))
  } catch (e: any) {
    appStore.showError(e?.message || t('admin.usage.releaseIpFailed'))
  } finally {
    actingIp.value = ''
  }
}
</script>
