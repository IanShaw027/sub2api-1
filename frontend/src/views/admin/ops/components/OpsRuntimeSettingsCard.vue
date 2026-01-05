<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { opsAPI } from '@/api/admin/ops'
import type { OpsAlertRuntimeSettings, OpsGroupAvailabilityRuntimeSettings } from '../types'
import BaseDialog from '@/components/common/BaseDialog.vue'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)

const alertSettings = ref<OpsAlertRuntimeSettings | null>(null)
const groupAvailabilitySettings = ref<OpsGroupAvailabilityRuntimeSettings | null>(null)

const showAlertEditor = ref(false)
const showGroupAvailabilityEditor = ref(false)
const saving = ref(false)

const draftAlert = ref<OpsAlertRuntimeSettings | null>(null)
const draftGroupAvailability = ref<OpsGroupAvailabilityRuntimeSettings | null>(null)

type ValidationResult = { valid: boolean, errors: string[] }

function validateRuntimeSettings(
  settings: OpsAlertRuntimeSettings | OpsGroupAvailabilityRuntimeSettings,
  opts: { requireKeyPrefix?: string } = {}
): ValidationResult {
  const errors: string[] = []
  const evalSeconds = settings.evaluation_interval_seconds
  if (!Number.isFinite(evalSeconds) || evalSeconds < 1 || evalSeconds > 86400) {
    errors.push(t('admin.ops.runtime.validation.evalIntervalRange'))
  }

  const lock = settings.distributed_lock
  if (lock.enabled) {
    if (!lock.key || lock.key.trim().length < 3) {
      errors.push(t('admin.ops.runtime.validation.lockKeyRequired'))
    } else if (opts.requireKeyPrefix && !lock.key.startsWith(opts.requireKeyPrefix)) {
      errors.push(t('admin.ops.runtime.validation.lockKeyPrefix', { prefix: opts.requireKeyPrefix }))
    }
    if (!Number.isFinite(lock.ttl_seconds) || lock.ttl_seconds < 1 || lock.ttl_seconds > 86400) {
      errors.push(t('admin.ops.runtime.validation.lockTtlRange'))
    }
  }

  // Alert-only runtime settings extensions
  if ('webhook' in settings) {
    const alertSettings = settings as OpsAlertRuntimeSettings
    const webhook = alertSettings.webhook
    if (webhook?.enabled) {
      const url = (webhook.url || '').trim()
      if (!url) errors.push(t('admin.ops.runtime.webhook.validation.urlRequired'))
      if (url && !(url.startsWith('https://') || url.startsWith('http://'))) {
        errors.push(t('admin.ops.runtime.webhook.validation.urlFormat'))
      }
      if (!Number.isFinite(webhook.timeout_seconds) || webhook.timeout_seconds < 1 || webhook.timeout_seconds > 300) {
        errors.push(t('admin.ops.runtime.webhook.validation.timeoutRange'))
      }
      if (!Number.isFinite(webhook.max_retries) || webhook.max_retries < 0 || webhook.max_retries > 10) {
        errors.push(t('admin.ops.runtime.webhook.validation.retriesRange'))
      }
    }

    const silencing = alertSettings.silencing
    if (silencing?.enabled) {
      const until = (silencing.global_until_rfc3339 || '').trim()
      if (until) {
        const parsed = Date.parse(until)
        if (!Number.isFinite(parsed)) errors.push(t('admin.ops.runtime.silencing.validation.timeFormat'))
      }
    }
  }

  return { valid: errors.length === 0, errors }
}

const alertValidation = computed(() => {
  if (!draftAlert.value) return { valid: true, errors: [] as string[] }
  return validateRuntimeSettings(draftAlert.value, { requireKeyPrefix: 'ops:' })
})

const groupAvailabilityValidation = computed(() => {
  if (!draftGroupAvailability.value) return { valid: true, errors: [] as string[] }
  return validateRuntimeSettings(draftGroupAvailability.value, { requireKeyPrefix: 'ops:' })
})

async function loadSettings() {
  loading.value = true
  try {
    const [alertCfg, groupCfg] = await Promise.all([
      opsAPI.getAlertRuntimeSettings(),
      opsAPI.getGroupAvailabilityRuntimeSettings()
    ])
    alertSettings.value = alertCfg
    groupAvailabilitySettings.value = groupCfg
  } catch (err: any) {
    console.error('[OpsRuntimeSettingsCard] Failed to load runtime settings', err)
    appStore.showError(err?.response?.data?.detail || t('admin.ops.runtime.loadFailed'))
  } finally {
    loading.value = false
  }
}

function openAlertEditor() {
  if (!alertSettings.value) return
  draftAlert.value = JSON.parse(JSON.stringify(alertSettings.value))
  // Backwards-compat: ensure nested settings exist even if API payload is older.
  if (draftAlert.value) {
    if (!draftAlert.value.webhook) {
      draftAlert.value.webhook = {
        enabled: false,
        url: '',
        secret: '',
        timeout_seconds: 5,
        max_retries: 2,
        include_resolved: false,
        min_severity: ''
      }
    }
    if (!draftAlert.value.silencing) {
      draftAlert.value.silencing = {
        enabled: false,
        global_until_rfc3339: '',
        global_reason: '',
        entries: []
      }
    }
  }
  showAlertEditor.value = true
}

function openGroupAvailabilityEditor() {
  if (!groupAvailabilitySettings.value) return
  draftGroupAvailability.value = JSON.parse(JSON.stringify(groupAvailabilitySettings.value))
  showGroupAvailabilityEditor.value = true
}

async function saveAlertSettings() {
  if (!draftAlert.value) return
  if (!alertValidation.value.valid) {
    appStore.showError(alertValidation.value.errors[0] || t('admin.ops.runtime.validation.invalid'))
    return
  }
  saving.value = true
  try {
    alertSettings.value = await opsAPI.updateAlertRuntimeSettings(draftAlert.value)
    showAlertEditor.value = false
    appStore.showSuccess(t('admin.ops.runtime.saveSuccess'))
  } catch (err: any) {
    console.error('[OpsRuntimeSettingsCard] Failed to save alert runtime settings', err)
    appStore.showError(err?.response?.data?.detail || t('admin.ops.runtime.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function saveGroupAvailabilitySettings() {
  if (!draftGroupAvailability.value) return
  if (!groupAvailabilityValidation.value.valid) {
    appStore.showError(groupAvailabilityValidation.value.errors[0] || t('admin.ops.runtime.validation.invalid'))
    return
  }
  saving.value = true
  try {
    groupAvailabilitySettings.value = await opsAPI.updateGroupAvailabilityRuntimeSettings(draftGroupAvailability.value)
    showGroupAvailabilityEditor.value = false
    appStore.showSuccess(t('admin.ops.runtime.saveSuccess'))
  } catch (err: any) {
    console.error('[OpsRuntimeSettingsCard] Failed to save group availability runtime settings', err)
    appStore.showError(err?.response?.data?.detail || t('admin.ops.runtime.saveFailed'))
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  loadSettings()
})
</script>

<template>
  <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="mb-4 flex items-start justify-between gap-4">
      <div>
        <h3 class="text-sm font-bold text-gray-900 dark:text-white">{{ t('admin.ops.runtime.title') }}</h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.runtime.description') }}</p>
      </div>
      <button
        class="flex items-center gap-1.5 rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-bold text-gray-700 transition-colors hover:bg-gray-200 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600"
        :disabled="loading"
        @click="loadSettings"
      >
        <svg class="h-3.5 w-3.5" :class="{ 'animate-spin': loading }" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
        {{ t('common.refresh') }}
      </button>
    </div>

    <div v-if="!alertSettings || !groupAvailabilitySettings" class="text-sm text-gray-500 dark:text-gray-400">
      <span v-if="loading">{{ t('admin.ops.runtime.loading') }}</span>
      <span v-else>{{ t('admin.ops.runtime.noData') }}</span>
    </div>

    <div v-else class="space-y-6">
      <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-700/50">
        <div class="mb-3 flex items-center justify-between">
          <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.ops.runtime.alertTitle') }}</h4>
          <button class="btn btn-sm btn-secondary" @click="openAlertEditor">{{ t('common.edit') }}</button>
        </div>
        <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
          <div class="text-xs text-gray-600 dark:text-gray-300">
            {{ t('admin.ops.runtime.evalIntervalSeconds') }}:
            <span class="ml-1 font-medium text-gray-900 dark:text-white">{{ alertSettings.evaluation_interval_seconds }}s</span>
          </div>
          <div class="text-xs text-gray-600 dark:text-gray-300">
            {{ t('admin.ops.runtime.webhook.enabled') }}:
            <span class="ml-1 font-medium text-gray-900 dark:text-white">
              {{ alertSettings.webhook?.enabled ? t('common.enabled') : t('common.disabled') }}
            </span>
          </div>
          <div
            v-if="alertSettings.silencing?.enabled && alertSettings.silencing.global_until_rfc3339"
            class="text-xs text-gray-600 dark:text-gray-300 md:col-span-2"
          >
            {{ t('admin.ops.runtime.silencing.globalUntil') }}:
            <span class="ml-1 font-mono text-gray-900 dark:text-white">{{ alertSettings.silencing.global_until_rfc3339 }}</span>
          </div>
          <!-- 隐藏的高级设置 -->
          <details class="col-span-1 md:col-span-2">
            <summary class="cursor-pointer text-xs font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400">
              {{ t('admin.ops.runtime.showAdvancedDeveloperSettings') }}
            </summary>
            <div class="mt-2 grid grid-cols-1 gap-3 rounded-lg bg-gray-100 p-3 dark:bg-dark-800 md:grid-cols-2">
              <div class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.runtime.lockEnabled') }}:
                <span class="ml-1 font-mono text-gray-700 dark:text-gray-300">{{ alertSettings.distributed_lock.enabled }}</span>
              </div>
              <div class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.runtime.lockKey') }}:
                <span class="ml-1 font-mono text-gray-700 dark:text-gray-300">{{ alertSettings.distributed_lock.key }}</span>
              </div>
              <div class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.runtime.lockTTLSeconds') }}:
                <span class="ml-1 font-mono text-gray-700 dark:text-gray-300">{{ alertSettings.distributed_lock.ttl_seconds }}s</span>
              </div>
            </div>
          </details>
        </div>
      </div>

      <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-700/50">
        <div class="mb-3 flex items-center justify-between">
          <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.ops.runtime.groupAvailabilityTitle') }}</h4>
          <button class="btn btn-sm btn-secondary" @click="openGroupAvailabilityEditor">{{ t('common.edit') }}</button>
        </div>
        <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
          <div class="text-xs text-gray-600 dark:text-gray-300">
            {{ t('admin.ops.runtime.evalIntervalSeconds') }}:
            <span class="ml-1 font-medium text-gray-900 dark:text-white">{{ groupAvailabilitySettings.evaluation_interval_seconds }}s</span>
          </div>
          <!-- 隐藏的高级设置 -->
          <details class="col-span-1 md:col-span-2">
            <summary class="cursor-pointer text-xs font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400">
              {{ t('admin.ops.runtime.showAdvancedDeveloperSettings') }}
            </summary>
            <div class="mt-2 grid grid-cols-1 gap-3 rounded-lg bg-gray-100 p-3 dark:bg-dark-800 md:grid-cols-2">
              <div class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.runtime.lockEnabled') }}:
                <span class="ml-1 font-mono text-gray-700 dark:text-gray-300">{{ groupAvailabilitySettings.distributed_lock.enabled }}</span>
              </div>
              <div class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.runtime.lockKey') }}:
                <span class="ml-1 font-mono text-gray-700 dark:text-gray-300">{{ groupAvailabilitySettings.distributed_lock.key }}</span>
              </div>
              <div class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.runtime.lockTTLSeconds') }}:
                <span class="ml-1 font-mono text-gray-700 dark:text-gray-300">{{ groupAvailabilitySettings.distributed_lock.ttl_seconds }}s</span>
              </div>
            </div>
          </details>
        </div>
      </div>
    </div>
  </div>

  <BaseDialog
    :show="showAlertEditor"
    :title="t('admin.ops.runtime.alertTitle')"
    width="wide"
    @close="showAlertEditor = false"
  >
    <div v-if="draftAlert" class="space-y-4">
      <div v-if="!alertValidation.valid" class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800 dark:border-amber-900/50 dark:bg-amber-900/20 dark:text-amber-200">
        <div class="font-bold">{{ t('admin.ops.runtime.validation.title') }}</div>
        <ul class="mt-1 list-disc space-y-1 pl-4">
          <li v-for="msg in alertValidation.errors" :key="msg">{{ msg }}</li>
        </ul>
      </div>
      <div>
        <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.ops.runtime.evalIntervalSeconds') }}</div>
        <input
          v-model.number="draftAlert.evaluation_interval_seconds"
          type="number"
          min="1"
          max="86400"
          class="input"
          :aria-invalid="!alertValidation.valid"
        />
        <p class="mt-1 text-xs text-gray-500">{{ t('admin.ops.runtime.evalIntervalHint') }}</p>
      </div>

      <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-700/50">
        <div class="mb-2 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.ops.runtime.webhook.title') }}</div>
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div class="md:col-span-2">
            <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="draftAlert.webhook.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300" />
              <span>{{ draftAlert.webhook.enabled ? t('common.enabled') : t('common.disabled') }}</span>
            </label>
          </div>

          <div class="md:col-span-2">
            <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.ops.runtime.webhook.url') }}</div>
            <input v-model="draftAlert.webhook.url" type="url" class="input font-mono text-sm" :placeholder="t('admin.ops.runtime.webhook.urlPlaceholder')" />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.runtime.webhook.urlHint') }}</p>
          </div>

          <div class="md:col-span-2">
            <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.ops.runtime.webhook.secret') }}</div>
            <input v-model="draftAlert.webhook.secret" type="password" class="input font-mono text-sm" :placeholder="t('admin.ops.runtime.webhook.secretPlaceholder')" />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.runtime.webhook.secretHint') }}</p>
          </div>

          <div>
            <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.ops.runtime.webhook.timeoutSeconds') }}</div>
            <input v-model.number="draftAlert.webhook.timeout_seconds" type="number" min="1" max="300" class="input" />
          </div>

          <div>
            <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.ops.runtime.webhook.maxRetries') }}</div>
            <input v-model.number="draftAlert.webhook.max_retries" type="number" min="0" max="10" class="input" />
          </div>

          <div class="md:col-span-2">
            <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="draftAlert.webhook.include_resolved" type="checkbox" class="h-4 w-4 rounded border-gray-300" />
              <span>{{ t('admin.ops.runtime.webhook.includeResolved') }}</span>
            </label>
          </div>
        </div>
      </div>

      <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-700/50">
        <div class="mb-2 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.ops.runtime.silencing.title') }}</div>
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div class="md:col-span-2">
            <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="draftAlert.silencing.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300" />
              <span>{{ t('admin.ops.runtime.silencing.enabled') }}</span>
            </label>
          </div>

          <div class="md:col-span-2">
            <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.ops.runtime.silencing.globalUntil') }}</div>
            <input v-model="draftAlert.silencing.global_until_rfc3339" type="text" class="input font-mono text-sm" :placeholder="t('admin.ops.runtime.silencing.untilPlaceholder')" />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.runtime.silencing.untilHint') }}</p>
          </div>

          <div class="md:col-span-2">
            <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.ops.runtime.silencing.reason') }}</div>
            <input v-model="draftAlert.silencing.global_reason" type="text" class="input" :placeholder="t('admin.ops.runtime.silencing.reasonPlaceholder')" />
          </div>
        </div>
      </div>

      <details class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-600 dark:bg-dark-800">
        <summary class="cursor-pointer text-xs font-medium text-gray-600 dark:text-gray-400">{{ t('admin.ops.runtime.advancedSettingsSummary') }}</summary>
        <div class="mt-3 grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="inline-flex items-center gap-2 text-xs text-gray-700 dark:text-gray-300">
              <input v-model="draftAlert.distributed_lock.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300" />
              <span>{{ t('admin.ops.runtime.lockEnabled') }}</span>
            </label>
          </div>
            <div class="md:col-span-2">
              <div class="mb-1 text-xs font-medium text-gray-500">{{ t('admin.ops.runtime.lockKey') }}</div>
              <input v-model="draftAlert.distributed_lock.key" type="text" class="input text-xs font-mono" />
              <p v-if="draftAlert.distributed_lock.enabled" class="mt-1 text-[11px] text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.runtime.validation.lockKeyHint', { prefix: 'ops:' }) }}
              </p>
            </div>
          <div>
            <div class="mb-1 text-xs font-medium text-gray-500">{{ t('admin.ops.runtime.lockTTLSeconds') }}</div>
            <input
              v-model.number="draftAlert.distributed_lock.ttl_seconds"
              type="number"
              min="1"
              max="86400"
              class="input text-xs font-mono"
            />
          </div>
        </div>
      </details>
    </div>
    <template #footer>
      <div class="flex justify-end gap-2">
        <button class="btn btn-secondary" @click="showAlertEditor = false">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" :disabled="saving || !alertValidation.valid" @click="saveAlertSettings">
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <BaseDialog
    :show="showGroupAvailabilityEditor"
    :title="t('admin.ops.runtime.groupAvailabilityTitle')"
    width="wide"
    @close="showGroupAvailabilityEditor = false"
  >
    <div v-if="draftGroupAvailability" class="space-y-4">
      <div v-if="!groupAvailabilityValidation.valid" class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800 dark:border-amber-900/50 dark:bg-amber-900/20 dark:text-amber-200">
        <div class="font-bold">{{ t('admin.ops.runtime.validation.title') }}</div>
        <ul class="mt-1 list-disc space-y-1 pl-4">
          <li v-for="msg in groupAvailabilityValidation.errors" :key="msg">{{ msg }}</li>
        </ul>
      </div>
      <div>
        <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.ops.runtime.evalIntervalSeconds') }}</div>
        <input
          v-model.number="draftGroupAvailability.evaluation_interval_seconds"
          type="number"
          min="1"
          max="86400"
          class="input"
          :aria-invalid="!groupAvailabilityValidation.valid"
        />
        <p class="mt-1 text-xs text-gray-500">{{ t('admin.ops.runtime.evalIntervalHint') }}</p>
      </div>
      
      <details class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-600 dark:bg-dark-800">
        <summary class="cursor-pointer text-xs font-medium text-gray-600 dark:text-gray-400">{{ t('admin.ops.runtime.advancedSettingsSummary') }}</summary>
        <div class="mt-3 grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="inline-flex items-center gap-2 text-xs text-gray-700 dark:text-gray-300">
              <input v-model="draftGroupAvailability.distributed_lock.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300" />
              <span>{{ t('admin.ops.runtime.lockEnabled') }}</span>
            </label>
          </div>
          <div class="md:col-span-2">
            <div class="mb-1 text-xs font-medium text-gray-500">{{ t('admin.ops.runtime.lockKey') }}</div>
            <input v-model="draftGroupAvailability.distributed_lock.key" type="text" class="input text-xs font-mono" />
            <p v-if="draftGroupAvailability.distributed_lock.enabled" class="mt-1 text-[11px] text-gray-500 dark:text-gray-400">
              {{ t('admin.ops.runtime.validation.lockKeyHint', { prefix: 'ops:' }) }}
            </p>
          </div>
          <div>
            <div class="mb-1 text-xs font-medium text-gray-500">{{ t('admin.ops.runtime.lockTTLSeconds') }}</div>
            <input
              v-model.number="draftGroupAvailability.distributed_lock.ttl_seconds"
              type="number"
              min="1"
              max="86400"
              class="input text-xs font-mono"
            />
          </div>
        </div>
      </details>
    </div>
    <template #footer>
      <div class="flex justify-end gap-2">
        <button class="btn btn-secondary" @click="showGroupAvailabilityEditor = false">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" :disabled="saving || !groupAvailabilityValidation.valid" @click="saveGroupAvailabilitySettings">
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>
