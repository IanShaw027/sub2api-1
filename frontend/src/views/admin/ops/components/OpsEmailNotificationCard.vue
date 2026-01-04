<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { opsAPI } from '@/api/admin/ops'
import type { EmailNotificationConfig, AlertSeverity } from '../types'
import BaseDialog from '@/components/common/BaseDialog.vue'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const config = ref<EmailNotificationConfig | null>(null)

const showEditor = ref(false)
const saving = ref(false)
const draft = ref<EmailNotificationConfig | null>(null)
const alertRecipientInput = ref('')
const reportRecipientInput = ref('')

const severityOptions: Array<{ value: AlertSeverity | ''; label: string }> = [
  { value: '', label: t('admin.ops.email.minSeverityAll') },
  { value: 'critical', label: t('common.critical') },
  { value: 'warning', label: t('common.warning') },
  { value: 'info', label: t('common.info') }
]

async function loadConfig() {
  loading.value = true
  try {
    const data = await opsAPI.getEmailNotificationConfig()
    config.value = data
  } catch (err: any) {
    console.error('[OpsEmailNotificationCard] Failed to load config', err)
    appStore.showError(err?.response?.data?.detail || t('admin.ops.email.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function saveConfig() {
  if (!draft.value) return
  saving.value = true
  try {
    config.value = await opsAPI.updateEmailNotificationConfig(draft.value)
    showEditor.value = false
    appStore.showSuccess(t('admin.ops.email.saveSuccess'))
  } catch (err: any) {
    console.error('[OpsEmailNotificationCard] Failed to save config', err)
    appStore.showError(err?.response?.data?.detail || t('admin.ops.email.saveFailed'))
  } finally {
    saving.value = false
  }
}

function openEditor() {
  if (!config.value) return
  draft.value = JSON.parse(JSON.stringify(config.value))
  alertRecipientInput.value = ''
  reportRecipientInput.value = ''
  showEditor.value = true
}

function addRecipient(target: 'alert' | 'report') {
  if (!draft.value) return
  const raw = (target === 'alert' ? alertRecipientInput.value : reportRecipientInput.value).trim()
  if (!raw) return
  if (!raw.includes('@')) return

  const normalized = raw.toLowerCase()
  const list = target === 'alert' ? draft.value.alert.recipients : draft.value.report.recipients
  if (!list.includes(normalized)) {
    list.push(normalized)
  }
  if (target === 'alert') alertRecipientInput.value = ''
  else reportRecipientInput.value = ''
}

function removeRecipient(target: 'alert' | 'report', email: string) {
  if (!draft.value) return
  const list = target === 'alert' ? draft.value.alert.recipients : draft.value.report.recipients
  const idx = list.indexOf(email)
  if (idx >= 0) list.splice(idx, 1)
}

onMounted(() => {
  loadConfig()
})
</script>

<template>
  <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="mb-4 flex items-start justify-between gap-4">
      <div>
        <h3 class="text-sm font-bold text-gray-900 dark:text-white">{{ t('admin.ops.email.title') }}</h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.email.description') }}</p>
      </div>
      <div class="flex items-center gap-2">
        <button
          class="flex items-center gap-1.5 rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-bold text-gray-700 transition-colors hover:bg-gray-200 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600"
          :disabled="loading"
          @click="loadConfig"
        >
          <svg class="h-3.5 w-3.5" :class="{ 'animate-spin': loading }" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          {{ t('common.refresh') }}
        </button>
        <button class="btn btn-sm btn-secondary" :disabled="!config" @click="openEditor">{{ t('common.edit') }}</button>
      </div>
    </div>

    <div v-if="!config" class="text-sm text-gray-500 dark:text-gray-400">
      <span v-if="loading">{{ t('admin.ops.email.loading') }}</span>
      <span v-else>{{ t('admin.ops.email.noData') }}</span>
    </div>

    <div v-else class="space-y-6">
      <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-700/50">
        <h4 class="mb-2 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.ops.email.alertTitle') }}</h4>
        <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
          <div class="text-xs text-gray-600 dark:text-gray-300">
            {{ t('common.enabled') }}:
            <span class="ml-1 font-medium text-gray-900 dark:text-white">{{ config.alert.enabled ? t('common.enabled') : t('common.disabled') }}</span>
          </div>
          <div class="text-xs text-gray-600 dark:text-gray-300">
            {{ t('admin.ops.email.minSeverity') }}:
            <span class="ml-1 font-medium text-gray-900 dark:text-white">{{ config.alert.min_severity || t('admin.ops.email.minSeverityAll') }}</span>
          </div>
          <div class="text-xs text-gray-600 dark:text-gray-300 md:col-span-2">
            {{ t('admin.ops.email.recipients') }}:
            <span class="ml-1 font-medium text-gray-900 dark:text-white">{{ config.alert.recipients.length }}</span>
          </div>
        </div>
      </div>

      <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-700/50">
        <h4 class="mb-2 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.ops.email.reportTitle') }}</h4>
        <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
          <div class="text-xs text-gray-600 dark:text-gray-300">
            {{ t('common.enabled') }}:
            <span class="ml-1 font-medium text-gray-900 dark:text-white">{{ config.report.enabled ? t('common.enabled') : t('common.disabled') }}</span>
          </div>
          <div class="text-xs text-gray-600 dark:text-gray-300">
            {{ t('admin.ops.email.recipients') }}:
            <span class="ml-1 font-medium text-gray-900 dark:text-white">{{ config.report.recipients.length }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>

  <BaseDialog
    :show="showEditor"
    :title="t('admin.ops.email.title')"
    width="extra-wide"
    @close="showEditor = false"
  >
    <div v-if="draft" class="space-y-6">
      <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-700/50">
        <h4 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.ops.email.alertTitle') }}</h4>
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('common.enabled') }}</div>
            <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="draft.alert.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300" />
              <span>{{ draft.alert.enabled ? t('common.enabled') : t('common.disabled') }}</span>
            </label>
          </div>

          <div>
            <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.ops.email.minSeverity') }}</div>
            <select v-model="draft.alert.min_severity" class="input">
              <option v-for="opt in severityOptions" :key="opt.value || 'all'" :value="opt.value">{{ opt.label }}</option>
            </select>
          </div>

          <div class="md:col-span-2">
            <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.ops.email.recipients') }}</div>
            <div class="flex gap-2">
              <input
                v-model="alertRecipientInput"
                type="email"
                class="input"
                :placeholder="t('admin.ops.email.recipients')"
                @keydown.enter.prevent="addRecipient('alert')"
              />
              <button class="btn btn-secondary whitespace-nowrap" type="button" @click="addRecipient('alert')">
                {{ t('common.add') }}
              </button>
            </div>
            <div class="mt-2 flex flex-wrap gap-2">
              <span
                v-for="email in draft.alert.recipients"
                :key="email"
                class="inline-flex items-center gap-2 rounded-full bg-blue-100 px-3 py-1 text-xs font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
              >
                {{ email }}
                <button
                  type="button"
                  class="text-blue-700/80 hover:text-blue-900 dark:text-blue-300"
                  @click="removeRecipient('alert', email)"
                >
                  ×
                </button>
              </span>
            </div>
            <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.email.recipientsHint') }}</div>
          </div>

          <div>
            <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.ops.email.rateLimitPerHour') }}</div>
            <input v-model.number="draft.alert.rate_limit_per_hour" type="number" min="0" max="100000" class="input" />
          </div>

          <div>
            <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.ops.email.batchWindowSeconds') }}</div>
            <input v-model.number="draft.alert.batching_window_seconds" type="number" min="0" max="86400" class="input" />
          </div>

          <div>
            <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.ops.email.includeResolved') }}</div>
            <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="draft.alert.include_resolved_alerts" type="checkbox" class="h-4 w-4 rounded border-gray-300" />
              <span>{{ draft.alert.include_resolved_alerts ? t('common.enabled') : t('common.disabled') }}</span>
            </label>
          </div>
        </div>
      </div>

      <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-700/50">
        <h4 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.ops.email.reportTitle') }}</h4>
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('common.enabled') }}</div>
            <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="draft.report.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300" />
              <span>{{ draft.report.enabled ? t('common.enabled') : t('common.disabled') }}</span>
            </label>
          </div>

          <div class="md:col-span-2">
            <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.ops.email.recipients') }}</div>
            <div class="flex gap-2">
              <input
                v-model="reportRecipientInput"
                type="email"
                class="input"
                :placeholder="t('admin.ops.email.recipients')"
                @keydown.enter.prevent="addRecipient('report')"
              />
              <button class="btn btn-secondary whitespace-nowrap" type="button" @click="addRecipient('report')">
                {{ t('common.add') }}
              </button>
            </div>
            <div class="mt-2 flex flex-wrap gap-2">
              <span
                v-for="email in draft.report.recipients"
                :key="email"
                class="inline-flex items-center gap-2 rounded-full bg-blue-100 px-3 py-1 text-xs font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
              >
                {{ email }}
                <button
                  type="button"
                  class="text-blue-700/80 hover:text-blue-900 dark:text-blue-300"
                  @click="removeRecipient('report', email)"
                >
                  ×
                </button>
              </span>
            </div>
          </div>

          <div class="md:col-span-2">
            <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
              <div>
                <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.ops.email.dailySummary') }}</div>
                <div class="flex items-center gap-2">
                  <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                    <input v-model="draft.report.daily_summary_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300" />
                  </label>
                  <input v-model="draft.report.daily_summary_schedule" type="text" class="input" :placeholder="t('admin.ops.email.cronPlaceholder')" />
                </div>
              </div>
              <div>
                <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.ops.email.weeklySummary') }}</div>
                <div class="flex items-center gap-2">
                  <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                    <input v-model="draft.report.weekly_summary_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300" />
                  </label>
                  <input v-model="draft.report.weekly_summary_schedule" type="text" class="input" :placeholder="t('admin.ops.email.cronPlaceholder')" />
                </div>
              </div>
              <div>
                <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.ops.email.errorDigest') }}</div>
                <div class="flex items-center gap-2">
                  <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                    <input v-model="draft.report.error_digest_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300" />
                  </label>
                  <input v-model="draft.report.error_digest_schedule" type="text" class="input" :placeholder="t('admin.ops.email.cronPlaceholder')" />
                </div>
              </div>
              <div>
                <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.ops.email.errorDigestMinCount') }}</div>
                <input v-model.number="draft.report.error_digest_min_count" type="number" min="0" max="1000000" class="input" />
              </div>
              <div>
                <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.ops.email.accountHealth') }}</div>
                <div class="flex items-center gap-2">
                  <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                    <input v-model="draft.report.account_health_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300" />
                  </label>
                  <input v-model="draft.report.account_health_schedule" type="text" class="input" :placeholder="t('admin.ops.email.cronPlaceholder')" />
                </div>
              </div>
              <div>
                <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.ops.email.accountHealthThreshold') }}</div>
                <input v-model.number="draft.report.account_health_error_rate_threshold" type="number" min="0" max="100" step="0.1" class="input" />
              </div>
            </div>
            <div class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.email.reportHint') }}</div>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-2">
        <button class="btn btn-secondary" @click="showEditor = false">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" :disabled="saving" @click="saveConfig">
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>
