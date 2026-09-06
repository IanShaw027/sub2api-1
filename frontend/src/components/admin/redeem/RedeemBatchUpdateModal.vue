<template>
  <UiModal
    :open="open"
    :title="t('admin.redeem.batchUpdateTitle')"
    width="md"
    :close-label="t('common.cancel')"
    @close="$emit('close')"
  >
    <p class="mb-4 text-sm text-muted">{{ t('admin.redeem.selectedCount', { count: selectedIds.length }) }}</p>

    <form id="batch-update-redeem-form" data-test="batch-update-form" class="space-y-5" @submit.prevent="handleBatchUpdate">
      <div>
        <div class="flex items-center gap-2">
          <input
            id="batch-field-status"
            data-test="batch-field-status"
            v-model="batchUpdateForm.update_status"
            type="checkbox"
            class="h-4 w-4 rounded border-line text-accent focus:ring-accent"
          />
          <label for="batch-field-status" class="text-sm text-foreground">
            {{ t('admin.redeem.batchFields.status') }}
          </label>
        </div>
        <Select
          v-if="batchUpdateForm.update_status"
          v-model="batchUpdateForm.status"
          data-test="batch-status-select"
          class="mt-2"
          :options="batchStatusOptions"
        />
      </div>

      <div>
        <div class="flex items-center gap-2">
          <input
            id="batch-field-expires-at"
            v-model="batchUpdateForm.update_expires_at"
            type="checkbox"
            class="h-4 w-4 rounded border-line text-accent focus:ring-accent"
          />
          <label for="batch-field-expires-at" class="text-sm text-foreground">
            {{ t('admin.redeem.batchFields.expiresAt') }}
          </label>
        </div>
        <template v-if="batchUpdateForm.update_expires_at">
          <Select v-model="batchUpdateForm.expires_mode" class="mt-2" :options="batchExpiryModeOptions" />
          <input
            v-if="batchUpdateForm.expires_mode === 'custom'"
            v-model="batchUpdateForm.expires_at_local"
            type="datetime-local"
            class="field mt-2"
          />
          <p v-if="batchUpdateForm.expires_mode === 'custom'" class="input-hint">
            {{ t('admin.redeem.localTimeZoneHint', { timezone: browserTimeZone }) }}
          </p>
        </template>
      </div>

      <div>
        <div class="flex items-center gap-2">
          <input
            id="batch-field-notes"
            data-test="batch-field-notes"
            v-model="batchUpdateForm.update_notes"
            type="checkbox"
            class="h-4 w-4 rounded border-line text-accent focus:ring-accent"
          />
          <label for="batch-field-notes" class="text-sm text-foreground">
            {{ t('admin.redeem.batchFields.notes') }}
          </label>
        </div>
        <textarea
          v-if="batchUpdateForm.update_notes"
          data-test="batch-notes-input"
          v-model="batchUpdateForm.notes"
          rows="3"
          class="field mt-2"
          :placeholder="t('admin.redeem.batchNotesPlaceholder')"
        ></textarea>
      </div>

      <div>
        <div class="flex items-center gap-2">
          <input
            id="batch-field-group"
            v-model="batchUpdateForm.update_group_id"
            type="checkbox"
            class="h-4 w-4 rounded border-line text-accent focus:ring-accent"
          />
          <label for="batch-field-group" class="text-sm text-foreground">
            {{ t('admin.redeem.batchFields.group') }}
          </label>
        </div>
        <Select
          v-if="batchUpdateForm.update_group_id"
          v-model="batchUpdateForm.group_id"
          class="mt-2"
          :options="batchGroupOptions"
          :placeholder="t('admin.redeem.selectGroupPlaceholder')"
        />
      </div>
    </form>

    <template #footer>
      <button type="button" class="btn btn-secondary" @click="$emit('close')">
        {{ t('common.cancel') }}
      </button>
      <button
        data-test="batch-update-submit"
        type="submit"
        form="batch-update-redeem-form"
        :disabled="batchUpdating"
        class="btn btn-primary"
      >
        {{ batchUpdating ? t('common.submitting') : t('admin.redeem.batchUpdate') }}
      </button>
    </template>
  </UiModal>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { getBrowserTimeZone, parseDateTimeLocalInput } from '@/utils/format'
import type { BatchUpdateRedeemCodeFields, Group } from '@/types'
import UiModal from '@/components/ui/UiModal.vue'
import Select from '@/components/common/Select.vue'

const props = defineProps<{
  open: boolean
  selectedIds: number[]
  subscriptionGroups: Group[]
}>()

const emit = defineEmits<{
  close: []
  updated: [updated: number]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const browserTimeZone = getBrowserTimeZone()

const batchUpdating = ref(false)

const batchUpdateForm = reactive({
  update_status: false,
  status: 'disabled' as 'unused' | 'disabled',
  update_expires_at: false,
  expires_mode: 'clear' as 'clear' | 'custom',
  expires_at_local: '',
  update_notes: false,
  notes: '',
  update_group_id: false,
  group_id: null as number | null
})

const batchStatusOptions = computed(() => [
  { value: 'unused', label: t('admin.redeem.status.unused') },
  { value: 'disabled', label: t('admin.redeem.status.disabled') }
])

const batchExpiryModeOptions = computed(() => [
  { value: 'clear', label: t('admin.redeem.neverExpires') },
  { value: 'custom', label: t('admin.redeem.customExpiry') }
])

const subscriptionGroupOptions = computed(() =>
  props.subscriptionGroups
    .filter((g) => g.subscription_type === 'subscription')
    .map((g) => ({ value: g.id, label: g.name }))
)

const batchGroupOptions = computed(() => [
  { value: null, label: t('admin.redeem.clearGroup') },
  ...subscriptionGroupOptions.value
])

const toDatetimeLocalInputValue = (date: Date) => {
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(
    date.getHours()
  )}:${pad(date.getMinutes())}`
}

const resetBatchUpdateForm = () => {
  batchUpdateForm.update_status = false
  batchUpdateForm.status = 'disabled'
  batchUpdateForm.update_expires_at = false
  batchUpdateForm.expires_mode = 'clear'
  batchUpdateForm.expires_at_local = toDatetimeLocalInputValue(
    new Date(Date.now() + 24 * 60 * 60 * 1000)
  )
  batchUpdateForm.update_notes = false
  batchUpdateForm.notes = ''
  batchUpdateForm.update_group_id = false
  batchUpdateForm.group_id = null
}

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) resetBatchUpdateForm()
  }
)

const buildBatchUpdateFields = (): BatchUpdateRedeemCodeFields | null => {
  const fields: BatchUpdateRedeemCodeFields = {}

  if (batchUpdateForm.update_status) {
    fields.status = batchUpdateForm.status
  }
  if (batchUpdateForm.update_expires_at) {
    if (batchUpdateForm.expires_mode === 'clear') {
      fields.expires_at = null
    } else {
      const expiresAt = parseDateTimeLocalInput(batchUpdateForm.expires_at_local)
      if (expiresAt === null) {
        appStore.showError(t('admin.redeem.expiryDateRequired'))
        return null
      }
      fields.expires_at = new Date(expiresAt * 1000).toISOString()
    }
  }
  if (batchUpdateForm.update_notes) {
    fields.notes = batchUpdateForm.notes
  }
  if (batchUpdateForm.update_group_id) {
    fields.group_id =
      batchUpdateForm.group_id == null ? null : Number(batchUpdateForm.group_id)
  }

  return Object.keys(fields).length > 0 ? fields : null
}

const handleBatchUpdate = async () => {
  const ids = props.selectedIds
  if (ids.length === 0) {
    appStore.showInfo(t('admin.redeem.selectCodesFirst'))
    return
  }

  const hasSelectedFields =
    batchUpdateForm.update_status ||
    batchUpdateForm.update_expires_at ||
    batchUpdateForm.update_notes ||
    batchUpdateForm.update_group_id
  if (!hasSelectedFields) {
    appStore.showError(t('admin.redeem.noBatchFieldsSelected'))
    return
  }

  const fields = buildBatchUpdateFields()
  if (!fields) {
    return
  }

  batchUpdating.value = true
  try {
    const result = await adminAPI.redeem.batchUpdate(ids, fields)
    appStore.showSuccess(t('admin.redeem.batchUpdateSuccess', { count: result.updated }))
    emit('updated', result.updated)
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToBatchUpdate'))
    console.error('Error batch updating codes:', error)
  } finally {
    batchUpdating.value = false
  }
}
</script>
