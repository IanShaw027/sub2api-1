<template>
  <UiModal
    :open="open"
    :title="isEdit ? t('keys.editKey') : t('keys.createKey')"
    width="lg"
    :close-label="t('common.close')"
    @close="$emit('close')"
  >
    <form id="key-form" class="keys-form" @submit.prevent="handleSubmit">
      <TextInput
        v-model="formData.name"
        :label="t('keys.nameLabel')"
        :placeholder="t('keys.namePlaceholder')"
        required
        data-tour="key-form-name"
      />

      <div class="keys-field">
        <FieldLabel>{{ t('keys.groupLabel') }}</FieldLabel>
        <UiSelect
          v-model="formData.group_id"
          :options="groupOptions"
          :placeholder="t('keys.selectGroup')"
          :searchable="true"
          :search-placeholder="t('keys.searchGroup')"
          data-tour="key-form-group"
        >
          <template #selected="{ option }">
            <GroupBadge
              v-if="option"
              :name="(option as unknown as GroupOption).label"
              :platform="(option as unknown as GroupOption).platform"
              :subscription-type="(option as unknown as GroupOption).subscriptionType"
              :rate-multiplier="(option as unknown as GroupOption).rate"
              :user-rate-multiplier="(option as unknown as GroupOption).userRate"
              :peak-rate-enabled="(option as unknown as GroupOption).peakRateEnabled"
              :peak-start="(option as unknown as GroupOption).peakStart"
              :peak-end="(option as unknown as GroupOption).peakEnd"
              :peak-rate-multiplier="(option as unknown as GroupOption).peakRateMultiplier"
            />
            <span v-else class="text-muted">{{ t('keys.selectGroup') }}</span>
          </template>
          <template #option="{ option, selected }">
            <GroupOptionItem
              :name="(option as unknown as GroupOption).label"
              :platform="(option as unknown as GroupOption).platform"
              :subscription-type="(option as unknown as GroupOption).subscriptionType"
              :rate-multiplier="(option as unknown as GroupOption).rate"
              :user-rate-multiplier="(option as unknown as GroupOption).userRate"
              :peak-rate-enabled="(option as unknown as GroupOption).peakRateEnabled"
              :peak-start="(option as unknown as GroupOption).peakStart"
              :peak-end="(option as unknown as GroupOption).peakEnd"
              :peak-rate-multiplier="(option as unknown as GroupOption).peakRateMultiplier"
              :description="(option as unknown as GroupOption).description"
              :selected="selected"
            />
          </template>
        </UiSelect>
      </div>

      <div v-if="!isEdit" class="keys-field">
        <div class="keys-toggle-row">
          <span class="keys-toggle-label">{{ t('keys.customKeyLabel') }}</span>
          <ToggleSwitch v-model="formData.use_custom_key" />
        </div>
        <TextInput
          v-if="formData.use_custom_key"
          v-model="formData.custom_key"
          class="keys-mono-input"
          :placeholder="t('keys.customKeyPlaceholder')"
          :error="customKeyError"
          :hint="customKeyError ? undefined : t('keys.customKeyHint')"
        />
      </div>

      <div v-if="isEdit" class="keys-field">
        <FieldLabel>{{ t('keys.statusLabel') }}</FieldLabel>
        <UiSelect
          v-model="formData.status"
          :options="statusOptions"
          :placeholder="t('keys.selectStatus')"
        />
      </div>

      <div class="keys-field">
        <div class="keys-toggle-row">
          <span class="keys-toggle-label">{{ t('keys.ipRestriction') }}</span>
          <ToggleSwitch v-model="formData.enable_ip_restriction" />
        </div>
        <div v-if="formData.enable_ip_restriction" class="keys-field-stack">
          <div>
            <FieldLabel :hint="t('keys.ipWhitelistHint')">{{ t('keys.ipWhitelist') }}</FieldLabel>
            <textarea
              v-model="formData.ip_whitelist"
              rows="3"
              class="field keys-textarea"
              :placeholder="t('keys.ipWhitelistPlaceholder')"
            />
          </div>
          <div>
            <FieldLabel :hint="t('keys.ipBlacklistHint')">{{ t('keys.ipBlacklist') }}</FieldLabel>
            <textarea
              v-model="formData.ip_blacklist"
              rows="3"
              class="field keys-textarea"
              :placeholder="t('keys.ipBlacklistPlaceholder')"
            />
          </div>
        </div>
      </div>

      <div class="keys-field">
        <TextInput
          :model-value="formData.quota ?? ''"
          type="number"
          step="0.01"
          min="0"
          :label="t('keys.quotaLimit')"
          :hint="t('keys.quotaAmountHint')"
          :placeholder="t('keys.quotaAmountPlaceholder')"
          @update:model-value="(v) => (formData.quota = v === '' ? null : Number(v))"
        />
        <div v-if="isEdit && editingKey && editingKey.quota > 0" class="keys-usage-row">
          <div class="glass-inset keys-usage-readout">
            <b>{{ formatCost(editingKey.quota_used, 4) }}</b>
            <span class="keys-usage-sep">/</span>
            <span class="text-muted">{{ formatCost(editingKey.quota) }}</span>
          </div>
          <Button variant="secondary" :title="t('keys.resetQuotaUsed')" @click="confirmResetQuota">
            {{ t('keys.reset') }}
          </Button>
        </div>
      </div>

      <div class="keys-field">
        <div class="keys-toggle-row">
          <span class="keys-toggle-label">{{ t('keys.rateLimitSection') }}</span>
          <ToggleSwitch v-model="formData.enable_rate_limit" />
        </div>
        <div v-if="formData.enable_rate_limit" class="keys-field-stack">
          <p class="input-hint">{{ t('keys.rateLimitHint') }}</p>
          <div v-for="window in rateLimitWindows" :key="window.key">
            <TextInput
              :model-value="formData[window.key] ?? ''"
              type="number"
              step="0.01"
              min="0"
              :label="window.label"
              placeholder="0"
              @update:model-value="(v) => (formData[window.key] = v === '' ? null : Number(v))"
            />
            <div v-if="isEdit && editingKey && (editingKey[window.limitField] ?? 0) > 0" class="keys-window-usage">
              <div class="keys-window-readout">
                <b :class="windowToneClass(editingKey, window)">
                  {{ formatCost(editingKey[window.usageField], 4) }}
                </b>
                <span class="keys-usage-sep">/</span>
                <span class="text-muted">{{ formatCost(editingKey[window.limitField]) }}</span>
              </div>
              <ProgressBar :value="windowPercent(editingKey, window)" />
            </div>
          </div>
          <div v-if="isEdit && editingKey && hasRateLimit(editingKey)">
            <Button variant="secondary" @click="confirmResetRateLimit">
              {{ t('keys.resetRateLimitUsage') }}
            </Button>
          </div>
        </div>
      </div>

      <div class="keys-field">
        <div class="keys-toggle-row">
          <span class="keys-toggle-label">{{ t('keys.expiration') }}</span>
          <ToggleSwitch v-model="formData.enable_expiration" />
        </div>
        <div v-if="formData.enable_expiration" class="keys-field-stack">
          <div class="keys-expiry-presets">
            <button
              v-for="days in ['7', '30', '90']"
              :key="days"
              type="button"
              class="chip chip-filter"
              :class="{ 'is-active': formData.expiration_preset === days }"
              @click="setExpirationDays(parseInt(days))"
            >
              {{ isEdit ? t('keys.extendDays', { days }) : t('keys.expiresInDays', { days }) }}
            </button>
            <button
              type="button"
              class="chip chip-filter"
              :class="{ 'is-active': formData.expiration_preset === 'custom' }"
              @click="formData.expiration_preset = 'custom'"
            >
              {{ t('keys.customDate') }}
            </button>
          </div>
          <TextInput
            v-model="formData.expiration_date"
            type="datetime-local"
            :label="t('keys.expirationDate')"
            :hint="t('keys.expirationDateHint')"
          />
          <p v-if="isEdit && editingKey?.expires_at" class="keys-current-expiry">
            <span class="text-muted">{{ t('keys.currentExpiration') }}: </span>
            <b>{{ formatDateTime(editingKey.expires_at) }}</b>
          </p>
        </div>
      </div>
    </form>
    <template #footer>
      <Button variant="secondary" @click="$emit('close')">{{ t('common.cancel') }}</Button>
      <Button
        native-type="submit"
        form="key-form"
        :loading="submitting"
        data-tour="key-form-submit"
      >
        {{ submitting ? t('keys.saving') : isEdit ? t('common.update') : t('common.create') }}
      </Button>
    </template>
  </UiModal>

  <!-- Reset quota / reset rate limit confirmations -->
  <UiModal
    :open="confirmDialog !== null"
    :title="confirmDialog?.title || ''"
    width="sm"
    :close-label="t('common.close')"
    @close="confirmDialog = null"
  >
    <p class="keys-confirm-text">{{ confirmDialog?.message }}</p>
    <template #footer>
      <Button variant="secondary" @click="confirmDialog = null">{{ t('common.cancel') }}</Button>
      <Button variant="danger" @click="runConfirm">{{ confirmDialog?.confirmText }}</Button>
    </template>
  </UiModal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useOnboardingStore } from '@/stores/onboarding'
import { keysAPI } from '@/api'
import UiModal from '@/components/ui/UiModal.vue'
import Button from '@/components/ui/Button.vue'
import FieldLabel from '@/components/ui/FieldLabel.vue'
import TextInput from '@/components/ui/TextInput.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue'
import ProgressBar from '@/components/ui/ProgressBar.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'
import type { ApiKey, Group, SubscriptionType, GroupPlatform, UpdateApiKeyRequest } from '@/types'
import { formatDateTime } from '@/utils/format'
import {
  formatCost,
  windowPercent,
  windowToneClass,
  hasRateLimit,
  RATE_LIMIT_WINDOW_DEFS,
  formatDateTimeLocal
} from './keyUtils'

const props = defineProps<{
  open: boolean
  editingKey: ApiKey | null
  groups: Group[]
  userGroupRates: Record<number, number>
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const { t } = useI18n()
const appStore = useAppStore()
const onboardingStore = useOnboardingStore()

interface GroupOption {
  value: number
  label: string
  description: string | null
  rate: number
  userRate: number | null
  peakRateEnabled: boolean
  peakStart: string
  peakEnd: string
  peakRateMultiplier: number
  subscriptionType: SubscriptionType
  platform: GroupPlatform
  [key: string]: unknown
}

const isEdit = computed(() => !!props.editingKey)
const submitting = ref(false)

const groupOptions = computed<GroupOption[]>(() =>
  props.groups.map((group) => ({
    value: group.id,
    label: group.name,
    description: group.description,
    rate: group.rate_multiplier,
    userRate: props.userGroupRates[group.id] ?? null,
    peakRateEnabled: group.peak_rate_enabled,
    peakStart: group.peak_start,
    peakEnd: group.peak_end,
    peakRateMultiplier: group.peak_rate_multiplier,
    subscriptionType: group.subscription_type,
    platform: group.platform
  }))
)

const statusOptions = computed(() => [
  { value: 'active', label: t('common.active') },
  { value: 'inactive', label: t('common.inactive') }
])

const rateLimitWindows = computed(() =>
  RATE_LIMIT_WINDOW_DEFS.map((window) => ({
    ...window,
    label: t(
      window.key === 'rate_limit_5h'
        ? 'keys.rateLimit5h'
        : window.key === 'rate_limit_1d'
          ? 'keys.rateLimit1d'
          : 'keys.rateLimit7d'
    )
  }))
)

const emptyFormData = () => ({
  name: '',
  group_id: null as number | null,
  status: 'active' as 'active' | 'inactive',
  use_custom_key: false,
  custom_key: '',
  enable_ip_restriction: false,
  ip_whitelist: '',
  ip_blacklist: '',
  enable_quota: false,
  quota: null as number | null,
  enable_rate_limit: false,
  rate_limit_5h: null as number | null,
  rate_limit_1d: null as number | null,
  rate_limit_7d: null as number | null,
  enable_expiration: false,
  expiration_preset: '30' as '7' | '30' | '90' | 'custom',
  expiration_date: ''
})

const formData = ref(emptyFormData())

const customKeyError = computed(() => {
  if (!formData.value.use_custom_key || !formData.value.custom_key) {
    return ''
  }
  const key = formData.value.custom_key
  if (key.length < 16) {
    return t('keys.customKeyTooShort')
  }
  if (!/^[a-zA-Z0-9_-]+$/.test(key)) {
    return t('keys.customKeyInvalidChars')
  }
  return ''
})

const populateForEdit = (key: ApiKey) => {
  const hasIPRestriction = (key.ip_whitelist?.length > 0) || (key.ip_blacklist?.length > 0)
  const hasExpiration = !!key.expires_at
  formData.value = {
    name: key.name,
    group_id: key.group_id,
    status: key.status === 'active' ? 'active' : 'inactive',
    use_custom_key: false,
    custom_key: '',
    enable_ip_restriction: hasIPRestriction,
    ip_whitelist: (key.ip_whitelist || []).join('\n'),
    ip_blacklist: (key.ip_blacklist || []).join('\n'),
    enable_quota: key.quota > 0,
    quota: key.quota > 0 ? key.quota : null,
    enable_rate_limit: (key.rate_limit_5h > 0) || (key.rate_limit_1d > 0) || (key.rate_limit_7d > 0),
    rate_limit_5h: key.rate_limit_5h || null,
    rate_limit_1d: key.rate_limit_1d || null,
    rate_limit_7d: key.rate_limit_7d || null,
    enable_expiration: hasExpiration,
    expiration_preset: 'custom',
    expiration_date: key.expires_at ? formatDateTimeLocal(key.expires_at) : ''
  }
}

watch(
  () => [props.open, props.editingKey] as const,
  ([open, editingKey]) => {
    if (!open) return
    if (editingKey) {
      populateForEdit(editingKey)
    } else {
      formData.value = emptyFormData()
    }
  },
  { immediate: true }
)

const shouldSubmitEditStatus = (key: ApiKey, status: 'active' | 'inactive') => {
  if (key.status === 'quota_exhausted' || key.status === 'expired') {
    return status === 'active'
  }
  return true
}

const setExpirationDays = (days: number) => {
  formData.value.expiration_preset = days.toString() as '7' | '30' | '90'
  const expDate = new Date()
  expDate.setDate(expDate.getDate() + days)
  formData.value.expiration_date = formatDateTimeLocal(expDate.toISOString())
}

const handleSubmit = async () => {
  if (formData.value.group_id === null) {
    appStore.showError(t('keys.groupRequired'))
    return
  }

  if (!isEdit.value && formData.value.use_custom_key) {
    if (!formData.value.custom_key) {
      appStore.showError(t('keys.customKeyRequired'))
      return
    }
    if (customKeyError.value) {
      appStore.showError(customKeyError.value)
      return
    }
  }

  const parseIPList = (text: string): string[] =>
    text.split('\n').map((ip) => ip.trim()).filter((ip) => ip.length > 0)
  const ipWhitelist = formData.value.enable_ip_restriction ? parseIPList(formData.value.ip_whitelist) : []
  const ipBlacklist = formData.value.enable_ip_restriction ? parseIPList(formData.value.ip_blacklist) : []

  const quota = formData.value.quota && formData.value.quota > 0 ? formData.value.quota : 0

  let expiresInDays: number | undefined
  let expiresAt: string | null | undefined
  if (formData.value.enable_expiration && formData.value.expiration_date) {
    if (!isEdit.value) {
      const expDate = new Date(formData.value.expiration_date)
      const now = new Date()
      const diffDays = Math.ceil((expDate.getTime() - now.getTime()) / (1000 * 60 * 60 * 24))
      expiresInDays = diffDays > 0 ? diffDays : 1
    } else {
      expiresAt = new Date(formData.value.expiration_date).toISOString()
    }
  } else if (isEdit.value) {
    expiresAt = ''
  }

  const rateLimitData = formData.value.enable_rate_limit ? {
    rate_limit_5h: formData.value.rate_limit_5h && formData.value.rate_limit_5h > 0 ? formData.value.rate_limit_5h : 0,
    rate_limit_1d: formData.value.rate_limit_1d && formData.value.rate_limit_1d > 0 ? formData.value.rate_limit_1d : 0,
    rate_limit_7d: formData.value.rate_limit_7d && formData.value.rate_limit_7d > 0 ? formData.value.rate_limit_7d : 0
  } : { rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0 }

  submitting.value = true
  try {
    if (isEdit.value && props.editingKey) {
      const updates: UpdateApiKeyRequest = {
        name: formData.value.name,
        group_id: formData.value.group_id,
        ip_whitelist: ipWhitelist,
        ip_blacklist: ipBlacklist,
        quota: quota,
        expires_at: expiresAt,
        rate_limit_5h: rateLimitData.rate_limit_5h,
        rate_limit_1d: rateLimitData.rate_limit_1d,
        rate_limit_7d: rateLimitData.rate_limit_7d
      }
      if (shouldSubmitEditStatus(props.editingKey, formData.value.status)) {
        updates.status = formData.value.status
      }
      await keysAPI.update(props.editingKey.id, updates)
      appStore.showSuccess(t('keys.keyUpdatedSuccess'))
    } else {
      const customKey = formData.value.use_custom_key ? formData.value.custom_key : undefined
      await keysAPI.create(
        formData.value.name,
        formData.value.group_id,
        customKey,
        ipWhitelist,
        ipBlacklist,
        quota,
        expiresInDays,
        rateLimitData
      )
      appStore.showSuccess(t('keys.keyCreatedSuccess'))
      if (onboardingStore.isCurrentStep('[data-tour="key-form-submit"]')) {
        onboardingStore.nextStep(500)
      }
    }
    emit('saved')
    emit('close')
  } catch (error: any) {
    const errorMsg = error.response?.data?.detail || t('keys.failedToSave')
    appStore.showError(errorMsg)
  } finally {
    submitting.value = false
  }
}

// Reset quota / rate limit confirmations (scoped to this modal)
const confirmDialog = ref<{
  title: string
  message: string
  confirmText: string
  onConfirm: () => void | Promise<void>
} | null>(null)

const runConfirm = async () => {
  const dialog = confirmDialog.value
  if (!dialog) return
  confirmDialog.value = null
  await dialog.onConfirm()
}

const confirmResetQuota = () => {
  if (!props.editingKey) return
  const key = props.editingKey
  confirmDialog.value = {
    title: t('keys.resetQuotaTitle'),
    message: t('keys.resetQuotaConfirmMessage', {
      name: key.name,
      used: key.quota_used?.toFixed(4)
    }),
    confirmText: t('keys.reset'),
    onConfirm: async () => {
      try {
        await keysAPI.update(key.id, { reset_quota: true })
        appStore.showSuccess(t('keys.quotaResetSuccess'))
        key.quota_used = 0
        emit('saved')
      } catch (error: any) {
        const errorMsg = error.response?.data?.detail || t('keys.failedToResetQuota')
        appStore.showError(errorMsg)
      }
    }
  }
}

const confirmResetRateLimit = () => {
  if (!props.editingKey) return
  const key = props.editingKey
  confirmDialog.value = {
    title: t('keys.resetRateLimitTitle'),
    message: t('keys.resetRateLimitConfirmMessage', { name: key.name }),
    confirmText: t('keys.reset'),
    onConfirm: async () => {
      try {
        await keysAPI.update(key.id, { reset_rate_limit_usage: true })
        appStore.showSuccess(t('keys.rateLimitResetSuccess'))
        emit('saved')
      } catch (error: any) {
        const errorMsg = error.response?.data?.detail || t('keys.failedToResetRateLimit')
        appStore.showError(errorMsg)
      }
    }
  }
}

</script>

<style scoped>
.keys-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.keys-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.keys-field-stack {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.keys-toggle-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.keys-toggle-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
}

.keys-mono-input :deep(.field) {
  font-family: var(--font-mono);
}

.keys-textarea {
  height: auto;
  min-height: 76px;
  padding: 8px 12px;
  font-family: var(--font-mono);
  font-size: 12.5px;
  line-height: 1.6;
  resize: vertical;
}

.keys-usage-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.keys-usage-readout,
.keys-window-readout {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1;
  min-width: 0;
  padding: 0 12px;
  height: 36px;
  border-radius: var(--radius-field);
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}

.keys-window-usage {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 8px;
}

.keys-usage-sep {
  color: var(--muted);
}

.keys-expiry-presets {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.keys-current-expiry {
  font-size: 12.5px;
}

.keys-tone-danger {
  color: var(--danger-text);
}

.keys-tone-warning {
  color: var(--warning-text);
}

.keys-confirm-text {
  font-size: 13px;
  color: var(--muted);
  line-height: 1.6;
}
</style>
