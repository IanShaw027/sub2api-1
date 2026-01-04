<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { GroupAvailabilityConfig, GroupAvailabilityStatus, AlertSeverity } from '../types'
import { opsAPI } from '@/api/admin/ops'

const { t } = useI18n()

const props = defineProps<{
  show: boolean
  focusGroupId?: number | null
}>()

const emit = defineEmits<{
  close: []
}>()

const loading = ref(false)

// Group Availability
const groupConfigs = ref<GroupAvailabilityStatus[]>([])
const selectedGroups = ref<GroupAvailabilityStatus[]>([])
const showBatchThreshold = ref(false)
const batchThreshold = ref(1)
const showBatchSeverity = ref(false)
const batchSeverity = ref<AlertSeverity>('warning')
const showTemplateDialog = ref(false)
const selectedTemplate = ref('')

interface ConfigTemplate {
  translationKey: string
  config: {
    min_available_accounts: number
    severity: AlertSeverity
    cooldown_minutes: number
    notify_email: boolean
  }
}

const templates: ConfigTemplate[] = [
  {
    translationKey: 'strictMode',
    config: {
      min_available_accounts: 5,
      severity: 'critical',
      cooldown_minutes: 15,
      notify_email: true
    }
  },
  {
    translationKey: 'standardMode',
    config: {
      min_available_accounts: 3,
      severity: 'warning',
      cooldown_minutes: 30,
      notify_email: true
    }
  },
  {
    translationKey: 'looseMode',
    config: {
      min_available_accounts: 1,
      severity: 'info',
      cooldown_minutes: 60,
      notify_email: false
    }
  }
]

const getSeverityLabel = (value: string) => {
  if (value === 'critical') return t('common.critical')
  if (value === 'warning') return t('common.warning')
  if (value === 'info') return t('common.info')
  return value
}

const severityLevels: AlertSeverity[] = ['critical', 'warning', 'info']

async function loadGroupConfigs() {
  loading.value = true
  try {
    groupConfigs.value = await opsAPI.listGroupAvailabilityStatus()
  } catch (err) {
    console.error(t('admin.ops.config.loadConfigFailed'), err)
  } finally {
    loading.value = false
  }
}

function applyFocusGroupSelection() {
  const focusGroupId = props.focusGroupId
  if (!focusGroupId) return

  const target = groupConfigs.value.find(g => g.group_id === focusGroupId)
  if (!target) return

  selectedGroups.value = [target]
}

function toggleGroupSelection(group: GroupAvailabilityStatus) {
  const index = selectedGroups.value.findIndex(g => g.group_id === group.group_id)
  if (index > -1) {
    selectedGroups.value.splice(index, 1)
  } else {
    selectedGroups.value.push(group)
  }
}

function isSelected(group: GroupAvailabilityStatus) {
  return selectedGroups.value.some(g => g.group_id === group.group_id)
}

function getGroupConfig(group: GroupAvailabilityStatus): GroupAvailabilityConfig {
  return {
    id: group.config?.id,
    group_id: group.group_id,
    enabled: group.config?.enabled ?? group.monitoring_enabled ?? false,
    min_available_accounts: group.config?.min_available_accounts ?? group.min_available_accounts ?? 1,
    severity: group.config?.severity ?? 'warning',
    notify_email: group.config?.notify_email ?? false,
    cooldown_minutes: group.config?.cooldown_minutes ?? 0,
    created_at: group.config?.created_at,
    updated_at: group.config?.updated_at
  }
}

function buildConfigUpdate(group: GroupAvailabilityStatus, patch: Partial<GroupAvailabilityConfig>) {
  return {
    ...getGroupConfig(group),
    ...patch,
    group_id: group.group_id
  }
}

async function batchEnableGroups() {
  if (selectedGroups.value.length === 0) return
  loading.value = true
  try {
    await Promise.all(selectedGroups.value.map(group =>
      opsAPI.updateGroupAvailabilityConfig(group.group_id, buildConfigUpdate(group, { enabled: true }))
    ))
    alert(t('admin.ops.config.batchEnableSuccess', { count: selectedGroups.value.length }))
    await loadGroupConfigs()
    selectedGroups.value = []
  } catch (err) {
    alert(t('admin.ops.config.batchEnableFailed'))
  } finally {
    loading.value = false
  }
}

async function batchDisableGroups() {
  if (selectedGroups.value.length === 0) return
  loading.value = true
  try {
    await Promise.all(selectedGroups.value.map(group =>
      opsAPI.updateGroupAvailabilityConfig(group.group_id, buildConfigUpdate(group, { enabled: false }))
    ))
    alert(t('admin.ops.config.batchDisableSuccess', { count: selectedGroups.value.length }))
    await loadGroupConfigs()
    selectedGroups.value = []
  } catch (err) {
    alert(t('admin.ops.config.batchDisableFailed'))
  } finally {
    loading.value = false
  }
}

async function confirmBatchThreshold() {
  loading.value = true
  try {
    await Promise.all(selectedGroups.value.map(group =>
      opsAPI.updateGroupAvailabilityConfig(group.group_id, buildConfigUpdate(group, {
        min_available_accounts: batchThreshold.value
      }))
    ))
    alert(t('admin.ops.config.batchSetThresholdSuccess', { count: selectedGroups.value.length }))
    showBatchThreshold.value = false
    await loadGroupConfigs()
    selectedGroups.value = []
  } catch (err) {
    alert(t('admin.ops.config.batchSetThresholdFailed'))
  } finally {
    loading.value = false
  }
}

async function confirmBatchSeverity() {
  loading.value = true
  try {
    await Promise.all(selectedGroups.value.map(group =>
      opsAPI.updateGroupAvailabilityConfig(group.group_id, buildConfigUpdate(group, { severity: batchSeverity.value }))
    ))
    alert(t('admin.ops.config.batchSetSeveritySuccess', { count: selectedGroups.value.length }))
    showBatchSeverity.value = false
    await loadGroupConfigs()
    selectedGroups.value = []
  } catch (err) {
    alert(t('admin.ops.config.batchSetSeverityFailed'))
  } finally {
    loading.value = false
  }
}

async function confirmApplyTemplate() {
  if (!selectedTemplate.value) return
  const template = templates.find(t => t.translationKey === selectedTemplate.value)
  if (!template) return

  loading.value = true
  try {
    await Promise.all(selectedGroups.value.map(group =>
      opsAPI.updateGroupAvailabilityConfig(group.group_id, buildConfigUpdate(group, {
        min_available_accounts: template.config.min_available_accounts,
        severity: template.config.severity,
        cooldown_minutes: template.config.cooldown_minutes,
        notify_email: template.config.notify_email
      }))
    ))
    alert(t('admin.ops.config.applyTemplateSuccess', { count: selectedGroups.value.length }))
    showTemplateDialog.value = false
    await loadGroupConfigs()
    selectedGroups.value = []
  } catch (err) {
    alert(t('admin.ops.config.applyTemplateFailed'))
  } finally {
    loading.value = false
  }
}

function exportConfig() {
  const config = selectedGroups.value.length > 0
    ? selectedGroups.value.map(g => getGroupConfig(g))
    : groupConfigs.value.map(g => getGroupConfig(g))

  const blob = new Blob([JSON.stringify(config, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `group-availability-config-${Date.now()}.json`
  a.click()
  URL.revokeObjectURL(url)
  alert(t('admin.ops.config.configExported'))
}

async function importConfig(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return

  try {
    const text = await file.text()
    const configs = JSON.parse(text) as GroupAvailabilityConfig[]

    let success = 0
    let failed = 0

    for (const config of configs) {
      try {
        await opsAPI.updateGroupAvailabilityConfig(config.group_id, config)
        success++
      } catch {
        failed++
      }
    }

    alert(t('admin.ops.config.importComplete', { success, failed }))
    await loadGroupConfigs()
  } catch (err) {
    alert(t('admin.ops.config.importFailedFormat'))
  }
}

// Load data when dialog opens
watch(
  () => props.show,
  async (isOpen) => {
    if (!isOpen) {
      selectedGroups.value = []
      return
    }

    await loadGroupConfigs()
    applyFocusGroupSelection()
  },
  { immediate: true }
)

watch(
  () => props.focusGroupId,
  () => {
    if (!props.show) return
    applyFocusGroupSelection()
  }
)
</script>

<template>
  <BaseDialog :show="show" :title="t('admin.ops.config.title')" width="extra-wide" @close="emit('close')">
    <!-- Batch Actions Toolbar -->
    <div v-if="selectedGroups.length > 0" class="mb-4 flex items-center justify-between rounded-lg bg-blue-50 p-3 dark:bg-blue-900/20">
      <span class="text-sm font-medium text-blue-700 dark:text-blue-300">
        {{ t('admin.ops.config.selectedGroups', { count: selectedGroups.length }) }}
      </span>
      <div class="flex gap-2">
        <button @click="batchEnableGroups" class="btn btn-sm btn-secondary">{{ t('admin.ops.config.batchEnable') }}</button>
        <button @click="batchDisableGroups" class="btn btn-sm btn-secondary">{{ t('admin.ops.config.batchDisable') }}</button>
        <button @click="showBatchThreshold = true" class="btn btn-sm btn-secondary">{{ t('admin.ops.config.batchSetThreshold') }}</button>
        <button @click="showBatchSeverity = true" class="btn btn-sm btn-secondary">{{ t('admin.ops.config.batchSetSeverity') }}</button>
        <button @click="showTemplateDialog = true" class="btn btn-sm btn-primary">{{ t('admin.ops.config.applyTemplate') }}</button>
        <button @click="selectedGroups = []" class="btn btn-sm btn-secondary">{{ t('admin.ops.config.cancelSelection') }}</button>
      </div>
    </div>

    <!-- Export/Import -->
    <div class="mb-4 flex gap-2">
      <button @click="exportConfig" class="btn btn-sm btn-secondary">{{ t('admin.ops.config.exportConfig') }}</button>
      <label class="btn btn-sm btn-secondary cursor-pointer">
        {{ t('admin.ops.config.importConfig') }}
        <input type="file" accept=".json" class="hidden" @change="importConfig" />
      </label>
    </div>

    <!-- Groups Table -->
    <div class="overflow-x-auto">
      <table class="w-full">
        <thead class="bg-gray-50 dark:bg-dark-700">
          <tr>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">
              <input
                type="checkbox"
                @change="(e) => selectedGroups = (e.target as HTMLInputElement).checked ? [...groupConfigs] : []"
              />
            </th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">{{ t('admin.ops.availability.groupName') }}</th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">{{ t('common.status') }}</th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">{{ t('admin.ops.availability.availableAccounts') }}</th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">{{ t('admin.ops.config.minThreshold') }}</th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">{{ t('admin.ops.config.monitoringStatus') }}</th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">{{ t('admin.ops.config.alertStatus') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200 dark:divide-dark-700">
          <tr v-for="group in groupConfigs" :key="group.group_id" class="hover:bg-gray-50 dark:hover:bg-dark-700">
            <td class="px-4 py-3">
              <input type="checkbox" :checked="isSelected(group)" @change="toggleGroupSelection(group)" />
            </td>
            <td class="px-4 py-3 text-sm font-medium text-gray-900 dark:text-white">{{ group.group_name }}</td>
            <td class="px-4 py-3 text-sm text-gray-500 dark:text-dark-400">
              <span class="badge badge-gray">{{ group.platform }}</span>
            </td>
            <td class="px-4 py-3 text-sm text-gray-500 dark:text-dark-400">
              {{ group.available_accounts }} / {{ group.total_accounts }}
            </td>
            <td class="px-4 py-3 text-sm text-gray-500 dark:text-dark-400">
              {{ getGroupConfig(group).min_available_accounts }}
            </td>
            <td class="px-4 py-3">
              <span :class="[
                'inline-flex rounded-full px-2 py-1 text-xs font-medium',
                getGroupConfig(group).enabled ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' : 'bg-gray-100 text-gray-700 dark:bg-gray-900/30 dark:text-gray-400'
              ]">
                {{ getGroupConfig(group).enabled ? t('common.enabled') : t('common.disabled') }}
              </span>
            </td>
            <td class="px-4 py-3">
              <span :class="[
                'inline-flex rounded-full px-2 py-1 text-xs font-medium',
                group.is_healthy ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' : 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
              ]">
                {{ group.is_healthy ? t('admin.ops.availability.healthy') : t('admin.ops.availability.alert') }}
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Batch Threshold Dialog -->
    <BaseDialog :show="showBatchThreshold" :title="t('admin.ops.config.batchSetThresholdTitle')" width="normal" @close="showBatchThreshold = false">
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-dark-300 mb-2">{{ t('admin.ops.config.minAvailableAccounts') }}</label>
          <input v-model.number="batchThreshold" type="number" min="1" class="input" />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-dark-300 mb-2">{{ t('admin.ops.config.applyToGroups') }}</label>
          <div class="flex flex-wrap gap-2">
            <span v-for="group in selectedGroups" :key="group.group_id" class="inline-flex rounded-full bg-blue-100 px-3 py-1 text-xs font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
              {{ group.group_name }}
            </span>
          </div>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <button @click="showBatchThreshold = false" class="btn btn-secondary">{{ t('common.cancel') }}</button>
          <button @click="confirmBatchThreshold" class="btn btn-primary" :disabled="loading">{{ t('common.confirm') }}</button>
        </div>
      </template>
    </BaseDialog>

    <!-- Batch Severity Dialog -->
    <BaseDialog :show="showBatchSeverity" :title="t('admin.ops.config.batchSetSeverityTitle')" width="normal" @close="showBatchSeverity = false">
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-dark-300 mb-2">{{ t('admin.ops.config.severity') }}</label>
          <select v-model="batchSeverity" class="input">
            <option v-for="s in severityLevels" :key="s" :value="s">{{ getSeverityLabel(s) }}</option>
          </select>
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-dark-300 mb-2">{{ t('admin.ops.config.applyToGroups') }}</label>
          <div class="flex flex-wrap gap-2">
            <span v-for="group in selectedGroups" :key="group.group_id" class="inline-flex rounded-full bg-blue-100 px-3 py-1 text-xs font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
              {{ group.group_name }}
            </span>
          </div>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <button @click="showBatchSeverity = false" class="btn btn-secondary">{{ t('common.cancel') }}</button>
          <button @click="confirmBatchSeverity" class="btn btn-primary" :disabled="loading">{{ t('common.confirm') }}</button>
        </div>
      </template>
    </BaseDialog>

    <!-- Template Dialog -->
    <BaseDialog :show="showTemplateDialog" :title="t('admin.ops.config.applyTemplateTitle')" width="wide" @close="showTemplateDialog = false">
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-dark-300 mb-3">{{ t('admin.ops.config.selectTemplate') }}</label>
          <div class="space-y-3">
            <label v-for="template in templates" :key="template.translationKey" class="flex items-start gap-3 rounded-lg border border-gray-200 p-4 cursor-pointer hover:bg-gray-50 dark:border-dark-600 dark:hover:bg-dark-700">
              <input type="radio" v-model="selectedTemplate" :value="template.translationKey" class="mt-1" />
              <div class="flex-1">
                <div class="font-semibold text-gray-900 dark:text-white">{{ t(`admin.ops.config.${template.translationKey}`) }}</div>
                <div class="text-sm text-gray-500 dark:text-dark-400">{{ t(`admin.ops.config.${template.translationKey}Desc`) }}</div>
                <div class="mt-2 text-xs text-gray-400 dark:text-dark-500">
                  {{ t('admin.ops.config.threshold') }}: {{ template.config.min_available_accounts }} |
                  {{ t('admin.ops.config.severity') }}: {{ getSeverityLabel(template.config.severity) }} |
                  {{ t('admin.ops.config.cooldown') }}: {{ template.config.cooldown_minutes }}{{ t('common.time.minutesAgo').replace('{n}', '') }} |
                  {{ t('admin.ops.config.email') }}: {{ template.config.notify_email ? t('common.enabled') : t('common.disabled') }}
                </div>
              </div>
            </label>
          </div>
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-dark-300 mb-2">{{ t('admin.ops.config.applyTo') }}</label>
          <div class="flex flex-wrap gap-2">
            <span v-for="group in selectedGroups" :key="group.group_id" class="inline-flex rounded-full bg-blue-100 px-3 py-1 text-xs font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
              {{ group.group_name }}
            </span>
          </div>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <button @click="showTemplateDialog = false" class="btn btn-secondary">{{ t('common.cancel') }}</button>
          <button @click="confirmApplyTemplate" class="btn btn-primary" :disabled="!selectedTemplate || loading">{{ t('common.confirm') }}</button>
        </div>
      </template>
    </BaseDialog>
  </BaseDialog>
</template>
