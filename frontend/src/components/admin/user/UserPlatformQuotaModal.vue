<template>
  <BaseDialog
    :show="show"
    :title="t('admin.users.platformQuota.title')"
    width="wide"
    @close="$emit('close')"
  >
    <div v-if="user" class="space-y-4">
      <div
        v-if="hasActiveSubscription"
        class="rounded-card border border-warning/40 bg-warning-soft px-4 py-3 text-sm text-warning dark:border-warning/30 dark:bg-warning-soft dark:text-warning"
      >
        {{ t('admin.users.platformQuota.subscriptionWarning') }}
      </div>
      <p class="text-sm text-ink-body dark:text-dark-400">
        {{ t('admin.users.platformQuota.subtitle', { email: user.email }) }}
      </p>
      <div v-if="loading" class="py-10 text-center text-ink-soft">{{ t('common.loading') }}</div>
      <div v-else class="overflow-x-auto">
        <table class="min-w-full text-sm">
          <thead>
            <tr class="border-b border-line text-ink-body dark:border-dark-700 dark:text-dark-200">
              <th class="px-3 py-2 text-left font-medium">{{ t('admin.users.platformQuota.columns.platform') }}</th>
              <th class="px-3 py-2 text-left font-medium">{{ t('admin.users.platformQuota.columns.daily') }}</th>
              <th class="px-3 py-2 text-left font-medium">{{ t('admin.users.platformQuota.columns.weekly') }}</th>
              <th class="px-3 py-2 text-left font-medium">{{ t('admin.users.platformQuota.columns.monthly') }}</th>
              <th class="px-3 py-2 text-left font-medium">{{ t('admin.users.platformQuota.columns.usage') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in quotas" :key="row.platform" class="border-b border-divider dark:border-dark-800">
              <td class="px-3 py-2 font-mono text-ink dark:text-white">{{ row.platform }}</td>
              <td class="px-3 py-2">
                <div class="flex items-center gap-1">
                  <input
                    v-model.number="row.daily_limit_usd"
                    type="number"
                    min="0"
                    step="0.01"
                    class="input w-24"
                    :placeholder="t('admin.users.platformQuota.placeholder')"
                  />
                  <button
                    type="button"
                    class="text-xs text-ink-faint hover:text-warning disabled:opacity-50"
                    :disabled="isResetting(`${row.platform}.daily`)"
                    :title="t('admin.users.platformQuota.reset.button')"
                    @click="onReset(row.platform, 'daily')"
                  >↻</button>
                </div>
              </td>
              <td class="px-3 py-2">
                <div class="flex items-center gap-1">
                  <input
                    v-model.number="row.weekly_limit_usd"
                    type="number"
                    min="0"
                    step="0.01"
                    class="input w-24"
                    :placeholder="t('admin.users.platformQuota.placeholder')"
                  />
                  <button
                    type="button"
                    class="text-xs text-ink-faint hover:text-warning disabled:opacity-50"
                    :disabled="isResetting(`${row.platform}.weekly`)"
                    :title="t('admin.users.platformQuota.reset.button')"
                    @click="onReset(row.platform, 'weekly')"
                  >↻</button>
                </div>
              </td>
              <td class="px-3 py-2">
                <div class="flex items-center gap-1">
                  <input
                    v-model.number="row.monthly_limit_usd"
                    type="number"
                    min="0"
                    step="0.01"
                    class="input w-24"
                    :placeholder="t('admin.users.platformQuota.placeholder')"
                  />
                  <button
                    type="button"
                    class="text-xs text-ink-faint hover:text-warning disabled:opacity-50"
                    :disabled="isResetting(`${row.platform}.monthly`)"
                    :title="t('admin.users.platformQuota.reset.button')"
                    @click="onReset(row.platform, 'monthly')"
                  >↻</button>
                </div>
              </td>
              <td class="px-3 py-2 text-xs text-ink-soft dark:text-dark-400">
                {{ formatUsage(row.daily_usage_usd) }} / {{ formatUsage(row.weekly_usage_usd) }} / {{ formatUsage(row.monthly_usage_usd) }}
              </td>
            </tr>
          </tbody>
        </table>
        <p class="mt-3 text-xs text-ink-soft">{{ t('admin.users.platformQuota.hint') }}</p>
        <div class="mt-3">
          <button type="button" class="btn btn-secondary text-sm" @click="onClearAll">
            {{ t('admin.users.platformQuota.clearAll') }}
          </button>
        </div>
      </div>
    </div>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="$emit('close')">
          {{ t('admin.users.platformQuota.cancel') }}
        </button>
        <button type="button" class="btn btn-primary" :disabled="submitting || loading" @click="onSave">
          {{ submitting ? t('admin.users.platformQuota.saving') : t('admin.users.platformQuota.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, reactive, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { AdminUser, PlatformQuotaItem, PlatformQuotaPlatform, PlatformQuotaWindow } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'

const props = defineProps<{ show: boolean; user: AdminUser | null }>()
const emit = defineEmits(['close', 'success'])

const { t } = useI18n()
const appStore = useAppStore()

const PLATFORMS: PlatformQuotaPlatform[] = ['anthropic', 'openai', 'gemini', 'antigravity', 'kiro', 'grok']

interface QuotaRow {
  platform: PlatformQuotaPlatform
  daily_limit_usd: number | null
  weekly_limit_usd: number | null
  monthly_limit_usd: number | null
  daily_usage_usd: number
  weekly_usage_usd: number
  monthly_usage_usd: number
}

const hasActiveSubscription = computed(() =>
  props.user?.subscriptions?.some((s) => s.status === 'active') ?? false
)

const loading = ref(false)
const submitting = ref(false)
const resetting = reactive<Record<string, number | undefined>>({})
const quotas = ref<QuotaRow[]>([])
const modalSessionID = ref(0)

function emptyRow(p: PlatformQuotaPlatform): QuotaRow {
  return {
    platform: p,
    daily_limit_usd: null,
    weekly_limit_usd: null,
    monthly_limit_usd: null,
    daily_usage_usd: 0,
    weekly_usage_usd: 0,
    monthly_usage_usd: 0,
  }
}

function normalize(items: PlatformQuotaItem[]): QuotaRow[] {
  const byPlatform = new Map<PlatformQuotaPlatform, PlatformQuotaItem>()
  for (const it of items) byPlatform.set(it.platform, it)
  return PLATFORMS.map((p) => {
    const it = byPlatform.get(p)
    if (!it) return emptyRow(p)
    return {
      platform: p,
      daily_limit_usd: it.daily_limit_usd ?? null,
      weekly_limit_usd: it.weekly_limit_usd ?? null,
      monthly_limit_usd: it.monthly_limit_usd ?? null,
      daily_usage_usd: it.daily_usage_usd ?? 0,
      weekly_usage_usd: it.weekly_usage_usd ?? 0,
      monthly_usage_usd: it.monthly_usage_usd ?? 0,
    }
  })
}

function formatUsage(n: number): string {
  if (n == null || Number.isNaN(n)) return '-'
  return n.toFixed(2)
}

function nextModalSessionID(): number {
  modalSessionID.value += 1
  return modalSessionID.value
}

function isActiveSession(sessionID: number, userID: number): boolean {
  return props.show && props.user?.id === userID && modalSessionID.value === sessionID
}

function clearResettingState() {
  for (const key of Object.keys(resetting)) {
    delete resetting[key]
  }
}

function resetModalState() {
  loading.value = false
  submitting.value = false
  clearResettingState()
  quotas.value = []
}

function isResetting(key: string): boolean {
  return resetting[key] === modalSessionID.value
}

async function load(userID: number, sessionID: number) {
  loading.value = true
  try {
    const data = await adminAPI.users.getPlatformQuotas(userID)
    if (!isActiveSession(sessionID, userID)) return
    quotas.value = normalize(data.platform_quotas || [])
  } catch {
    if (!isActiveSession(sessionID, userID)) return
    appStore.showError(t('admin.users.platformQuota.loadFailed'))
    quotas.value = PLATFORMS.map(emptyRow)
  } finally {
    if (isActiveSession(sessionID, userID)) {
      loading.value = false
    }
  }
}

watch(
  () => [props.show, props.user?.id] as const,
  ([show, userID]) => {
    const sessionID = nextModalSessionID()
    resetModalState()
    if (!show || userID == null) return
    void load(userID, sessionID)
  },
  { immediate: true },
)

function onClearAll() {
  // 二次确认：一键清空全部平台的 daily/weekly/monthly 限额属于高风险批量操作，
  // 误点后所有平台变为"无限额"，且本地无 undo 机制（需要逐个手动重填或取消保存）。
  const confirmed = window.confirm(t('admin.users.platformQuota.clearAllConfirm'))
  if (!confirmed) return
  for (const row of quotas.value) {
    row.daily_limit_usd = null
    row.weekly_limit_usd = null
    row.monthly_limit_usd = null
  }
}

async function onSave() {
  const userID = props.user?.id
  if (userID == null) return
  // 空字符串表示用户主动清空；其他非法中间态必须拦住，不能静默降级成 null。
  const invalid: string[] = []
  for (const row of quotas.value) {
    for (const win of ['daily', 'weekly', 'monthly'] as const) {
      const v = row[`${win}_limit_usd` as const]
      if (!isQuotaInputValid(v)) {
        invalid.push(`${row.platform}.${win}`)
      }
    }
  }
  if (invalid.length > 0) {
    appStore.showError(t('admin.users.platformQuota.invalidNumber', { fields: invalid.join(', ') }))
    return
  }

  const sessionID = modalSessionID.value
  submitting.value = true
  try {
    const payload = quotas.value.map((r) => ({
      platform: r.platform,
      daily_limit_usd: normalizeLimit(r.daily_limit_usd),
      weekly_limit_usd: normalizeLimit(r.weekly_limit_usd),
      monthly_limit_usd: normalizeLimit(r.monthly_limit_usd),
    }))
    await adminAPI.users.updatePlatformQuotas(userID, payload)
    if (!isActiveSession(sessionID, userID)) return
    appStore.showSuccess(t('admin.users.platformQuota.updateSuccess'))
    emit('success')
    emit('close')
  } catch (e: any) {
    if (!isActiveSession(sessionID, userID)) return
    appStore.showError(e?.response?.data?.message || t('admin.users.platformQuota.updateFailed'))
  } finally {
    if (isActiveSession(sessionID, userID)) {
      submitting.value = false
    }
  }
}

function isQuotaInputValid(v: unknown): boolean {
  return v === '' || v === null || v === undefined || (typeof v === 'number' && Number.isFinite(v) && v >= 0)
}

// 仅在合法输入下返回数字：null/undefined/空字符串/非法值 → null（视为"无限额"）。
// 调用方负责先拦截非法中间态（见 onSave）。
function normalizeLimit(v: number | string | null | undefined): number | null {
  if (v === '' || v === null || v === undefined) return null
  if (typeof v === 'number' && Number.isFinite(v) && v >= 0) return v
  return null
}

async function onReset(platform: PlatformQuotaPlatform, quotaWindow: PlatformQuotaWindow) {
  const userID = props.user?.id
  if (userID == null) return
  const windowLabel = t(`admin.users.platformQuota.window${quotaWindow.charAt(0).toUpperCase() + quotaWindow.slice(1)}`)
  const confirmed = window.confirm(
    t('admin.users.platformQuota.reset.confirm', { platform, window: windowLabel })
  )
  if (!confirmed) return
  const key = `${platform}.${quotaWindow}`
  const sessionID = modalSessionID.value
  resetting[key] = sessionID
  try {
    const data = await adminAPI.users.resetPlatformQuotaWindow(userID, platform, quotaWindow)
    if (!isActiveSession(sessionID, userID)) return
    quotas.value = normalize(data.platform_quotas || [])
    appStore.showSuccess(t('admin.users.platformQuota.reset.success', { platform, window: windowLabel }))
  } catch (e: any) {
    if (!isActiveSession(sessionID, userID)) return
    appStore.showError(e?.response?.data?.message || t('admin.users.platformQuota.reset.failed'))
  } finally {
    if (resetting[key] === sessionID) {
      delete resetting[key]
    }
  }
}
</script>
