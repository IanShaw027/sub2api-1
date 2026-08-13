<template>
  <div class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t('admin.settings.ipSecurity.title') }}
      </h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.settings.ipSecurity.description') }}
      </p>
    </div>
    <div class="space-y-5 p-6">
      <div class="flex items-center justify-between">
        <div>
          <label class="font-medium text-gray-900 dark:text-white">{{ t('admin.settings.ipSecurity.enable') }}</label>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.settings.ipSecurity.enableHint') }}</p>
        </div>
        <Toggle v-model="form.enabled" />
      </div>
      <div v-if="form.enabled" class="grid gap-4 border-t border-gray-100 pt-4 dark:border-dark-700 md:grid-cols-2">
        <div>
          <label class="input-label">{{ t('admin.settings.ipSecurity.windowMinutes') }}</label>
          <input v-model.number="form.window_minutes" type="number" min="1" max="1440" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.settings.ipSecurity.accountThreshold') }}</label>
          <input v-model.number="form.account_threshold" type="number" min="2" max="100" class="input" />
        </div>
        <div class="md:col-span-2">
          <label class="input-label">{{ t('admin.settings.ipSecurity.learningUntil') }}</label>
          <input v-model="form.learning_until" type="text" placeholder="2026-08-16T00:00:00Z" class="input" />
        </div>
      </div>
      <div class="flex justify-end">
        <button type="button" class="btn btn-primary btn-sm" :disabled="saving" @click="saveConfig">
          {{ t('common.save') }}
        </button>
      </div>

      <div class="flex flex-wrap items-center gap-3 border-t border-gray-100 pt-4 dark:border-dark-700">
        <select v-model="status" class="input w-auto" @change="loadBans(1)">
          <option value="active">{{ t('admin.settings.ipSecurity.statusActive') }}</option>
          <option value="whitelisted">{{ t('admin.settings.ipSecurity.statusWhitelisted') }}</option>
          <option value="released">{{ t('admin.settings.ipSecurity.statusReleased') }}</option>
          <option value="">{{ t('common.all') }}</option>
        </select>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="loadBans(page)">
          <Icon name="search" size="sm" />
          {{ t('common.refresh') }}
        </button>
        <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.ipSecurity.listHint') }}</span>
      </div>

      <div v-if="bans.length" class="overflow-x-auto rounded border border-gray-200 dark:border-dark-700">
        <table class="min-w-full text-left text-sm">
          <thead class="bg-gray-50 dark:bg-dark-800">
            <tr>
              <th class="px-3 py-2">IP</th>
              <th class="px-3 py-2">{{ t('common.status') }}</th>
              <th class="px-3 py-2">{{ t('admin.settings.ipSecurity.accounts') }}</th>
              <th class="px-3 py-2">{{ t('admin.settings.ipSecurity.createdAt') }}</th>
              <th class="px-3 py-2"></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="ban in bans" :key="ban.id" class="border-t border-gray-100 dark:border-dark-700">
              <td class="px-3 py-2 font-mono">{{ ban.ip_address }}</td>
              <td class="px-3 py-2">{{ formatStatus(ban.status) }}</td>
              <td class="px-3 py-2">{{ ban.detected_account_count }} / {{ ban.account_threshold }}</td>
              <td class="px-3 py-2 text-xs">{{ formatTime(ban.created_at) }}</td>
              <td class="px-3 py-2 text-right">
                <button type="button" class="btn btn-secondary btn-xs" @click="openBan(ban.id)">
                  {{ t('admin.settings.ipSecurity.details') }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="total" class="flex items-center justify-between text-sm text-gray-500">
        <span>{{ t('admin.settings.ipSecurity.total', { count: total }) }}</span>
        <div class="flex items-center gap-2">
          <button type="button" class="btn btn-secondary btn-xs" :disabled="page <= 1 || loading" @click="loadBans(page - 1)">
            <Icon name="chevronLeft" size="sm" />
          </button>
          <span>{{ page }} / {{ pages }}</span>
          <button type="button" class="btn btn-secondary btn-xs" :disabled="page >= pages || loading" @click="loadBans(page + 1)">
            <Icon name="chevronRight" size="sm" />
          </button>
        </div>
      </div>

      <div v-if="detail" class="rounded border border-gray-200 p-4 dark:border-dark-700">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <div class="font-mono font-semibold">{{ detail.ban.ip_address }}</div>
            <div class="text-xs text-gray-500">{{ formatStatus(detail.ban.status) }} · {{ detail.ban.reason }}</div>
          </div>
          <button
            v-if="detail.ban.status === 'active'"
            type="button"
            class="btn btn-danger btn-sm"
            @click="releaseBan(detail.ban.id)"
          >
            {{ t('admin.settings.ipSecurity.releaseAndWhitelist') }}
          </button>
          <button
            v-else-if="detail.ban.status === 'whitelisted'"
            type="button"
            class="btn btn-secondary btn-sm"
            @click="removeWhitelist(detail.ban.id)"
          >
            {{ t('admin.settings.ipSecurity.restoreDetection') }}
          </button>
        </div>
        <div class="mt-3 space-y-2 text-sm">
          <div
            v-for="activity in detail.activities"
            :key="`${activity.user_id}-${activity.source}-${activity.api_key_id}-${activity.request_id}-${activity.path}`"
            class="rounded bg-gray-50 p-2 dark:bg-dark-800"
          >
            <div class="flex flex-wrap justify-between gap-2">
              <span>{{ activity.user_email || activity.user_username || `#${activity.user_id}` }}</span>
              <span class="font-mono text-xs">
                {{ activity.source }}
                <span v-if="activity.api_key_id"> · key #{{ activity.api_key_id }}</span>
              </span>
            </div>
            <div class="mt-1 break-all font-mono text-xs text-gray-500">{{ activity.method }} {{ activity.path }}</div>
            <div class="mt-1 text-[11px] text-gray-400">
              {{ t('admin.settings.ipSecurity.calls') }}: {{ activity.request_count }} ·
              {{ formatTime(activity.first_seen_at) }} - {{ formatTime(activity.last_seen_at) }}
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import ipSecurityAPI, { type IPSecurityActivityDetail, type IPSecurityBan } from '@/api/admin/ipSecurity'
import { useAppStore } from '@/stores/app'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'

const { t } = useI18n()
const appStore = useAppStore()

const form = reactive({
  enabled: false,
  window_minutes: 10,
  account_threshold: 4,
  learning_until: ''
})
const saving = ref(false)
const loading = ref(false)
const status = ref('active')
const page = ref(1)
const pages = ref(1)
const total = ref(0)
const bans = ref<IPSecurityBan[]>([])
const detail = ref<{ ban: IPSecurityBan; activities: IPSecurityActivityDetail[] } | null>(null)

function formatTime(value: string): string {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

function formatStatus(value: string): string {
  if (value === 'active') return t('admin.settings.ipSecurity.statusActive')
  if (value === 'whitelisted') return t('admin.settings.ipSecurity.statusWhitelisted')
  if (value === 'released') return t('admin.settings.ipSecurity.statusReleased')
  return value
}

async function loadConfig() {
  const cfg = await ipSecurityAPI.getConfig()
  form.enabled = cfg.enabled
  form.window_minutes = cfg.window_minutes
  form.account_threshold = cfg.account_threshold
  form.learning_until = cfg.learning_until || ''
}

async function saveConfig() {
  saving.value = true
  try {
    await adminAPI.settings.updateSettings({
      ip_multi_account_ban_enabled: form.enabled,
      ip_multi_account_ban_window_minutes: Number(form.window_minutes) || 10,
      ip_multi_account_ban_threshold: Number(form.account_threshold) || 4,
      ip_multi_account_ban_learning_until: form.learning_until || ''
    })
    appStore.showSuccess(t('common.saved'))
    await loadConfig()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.settings.ipSecurity.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function loadBans(nextPage = 1) {
  loading.value = true
  try {
    const res = await ipSecurityAPI.listBans({
      status: status.value || undefined,
      page: nextPage,
      page_size: 20
    })
    bans.value = res.items || []
    page.value = res.page || nextPage
    pages.value = res.pages || 1
    total.value = res.total || 0
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.settings.ipSecurity.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function openBan(id: number) {
  try {
    const res = await ipSecurityAPI.getBan(id)
    detail.value = { ban: res.ban, activities: res.activities || [] }
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.settings.ipSecurity.loadFailed'))
  }
}

async function releaseBan(id: number) {
  try {
    await ipSecurityAPI.releaseBan(id)
    await loadBans(page.value)
    await openBan(id)
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.settings.ipSecurity.saveFailed'))
  }
}

async function removeWhitelist(id: number) {
  try {
    await ipSecurityAPI.removeWhitelist(id)
    await loadBans(page.value)
    await openBan(id)
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.settings.ipSecurity.saveFailed'))
  }
}

onMounted(async () => {
  try {
    await loadConfig()
    await loadBans(1)
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.settings.ipSecurity.loadFailed'))
  }
})
</script>
