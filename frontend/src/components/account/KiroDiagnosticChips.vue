<template>
  <div v-if="items.length" class="flex flex-wrap gap-2 text-xs">
    <span
      v-for="item in items"
      :key="item.key"
      :class="chipClass"
      :title="item.value"
    >
      {{ item.label }}: {{ item.value }}
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = withDefaults(defineProps<{
  credentials?: Record<string, unknown> | null
  extra?: Record<string, unknown> | null
  usageInfo?: Record<string, unknown> | null
  includeProfileMode?: boolean
  chipClass?: string
}>(), {
  credentials: null,
  extra: null,
  usageInfo: null,
  includeProfileMode: true,
  chipClass: 'inline-flex rounded-control bg-page px-1.5 py-0.5 text-ink-soft dark:bg-dark-800 dark:text-dark-300'
})

const { t } = useI18n()

const trimmedString = (record: Record<string, unknown>, key: string): string => {
  const value = record[key]
  return typeof value === 'string' ? value.trim() : ''
}

const items = computed(() => {
  const credentials = props.credentials || {}
  const extra = props.extra || {}
  const usageInfo = props.usageInfo || {}
  const out: Array<{ key: string; label: string; value: string }> = []

  const profileArn = trimmedString(credentials, 'profile_arn')
  const profileID = trimmedString(usageInfo, 'kiro_profile_id')
    || trimmedString(credentials, 'profile_id')
    || trimmedString(extra, 'profile_id')
  const loginProvider = trimmedString(usageInfo, 'kiro_login_provider')
    || trimmedString(credentials, 'login_provider')
    || trimmedString(extra, 'login_provider')
  const statusReason = trimmedString(usageInfo, 'kiro_status_reason')
    || trimmedString(credentials, 'status_reason')
    || trimmedString(credentials, 'kiro_status_reason')
    || trimmedString(extra, 'kiro_status_reason')

  if (props.includeProfileMode) {
    out.push({
      key: 'profile_mode',
      label: t('admin.accounts.kiro.profileModeShort'),
      value: profileArn ? t('admin.accounts.kiro.profileStateManual') : t('admin.accounts.kiro.profileStateAuto')
    })
  }
  if (profileID) {
    out.push({
      key: 'profile_id',
      label: t('admin.accounts.kiro.profileIdShort'),
      value: profileID
    })
  }
  if (loginProvider) {
    out.push({
      key: 'login_provider',
      label: t('admin.accounts.kiro.loginProviderShort'),
      value: loginProvider
    })
  }
  if (statusReason) {
    out.push({
      key: 'status_reason',
      label: t('admin.accounts.kiro.statusReasonShort'),
      value: statusReason
    })
  }
  return out
})
</script>
