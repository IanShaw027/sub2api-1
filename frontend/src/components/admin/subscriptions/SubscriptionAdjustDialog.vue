<template>
  <BaseDialog
    :show="open"
    :title="t('admin.subscriptions.adjustSubscription')"
    width="narrow"
    @close="$emit('close')"
  >
    <form
      v-if="subscription"
      id="extend-subscription-form"
      @submit.prevent="handleExtendSubscription"
      class="space-y-5"
    >
      <div class="rounded-lg bg-surface-2 p-4">
        <p class="text-sm text-muted">
          {{ t('admin.subscriptions.adjustingFor') }}
          <span class="font-medium text-foreground">{{
            subscription.user?.email
          }}</span>
        </p>
        <p class="mt-1 text-sm text-muted">
          {{ t('admin.subscriptions.currentExpiration') }}:
          <span class="font-medium text-foreground">
            {{
              subscription.expires_at
                ? formatDateTimeToMinute(subscription.expires_at)
                : t('admin.subscriptions.noExpiration')
            }}
          </span>
        </p>
        <p v-if="subscription.expires_at" class="mt-1 text-sm text-muted">
          {{ t('admin.subscriptions.remainingDays') }}:
          <span class="font-medium text-foreground">
            {{ getDaysRemaining(subscription.expires_at) ?? 0 }}
          </span>
        </p>
      </div>
      <div>
        <label class="input-label">{{ t('admin.subscriptions.form.adjustDays') }}</label>
        <div class="flex items-center gap-2">
          <input
            v-model.number="form.days"
            type="number"
            required
            class="input text-center"
            :placeholder="t('admin.subscriptions.adjustDaysPlaceholder')"
          />
        </div>
        <p class="input-hint">{{ t('admin.subscriptions.adjustHint') }}</p>
      </div>
    </form>
    <template #footer>
      <div v-if="subscription" class="flex justify-end gap-3">
        <button @click="$emit('close')" type="button" class="btn-glass-secondary">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="extend-subscription-form"
          :disabled="submitting"
          class="btn-glass-primary"
        >
          {{ submitting ? t('admin.subscriptions.adjusting') : t('admin.subscriptions.adjust') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { UserSubscription } from '@/types'
import { formatDateTimeToMinute } from '@/utils/format'
import BaseDialog from '@/components/common/BaseDialog.vue'

const props = defineProps<{
  open: boolean
  subscription: UserSubscription | null
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const submitting = ref(false)

const form = reactive({
  days: 30
})

watch(
  () => [props.open, props.subscription] as const,
  ([open, subscription]) => {
    if (open && subscription) {
      form.days = 30
    }
  },
  { immediate: true }
)

const getDaysRemaining = (expiresAt: string): number | null => {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  if (diff < 0) return null
  return Math.ceil(diff / (1000 * 60 * 60 * 24))
}

const handleExtendSubscription = async () => {
  if (!props.subscription) return

  // 前端验证：调整后的过期时间必须在未来
  if (props.subscription.expires_at) {
    const expiresAt = new Date(props.subscription.expires_at)
    const newExpiresAt = new Date(expiresAt.getTime() + form.days * 24 * 60 * 60 * 1000)
    if (newExpiresAt <= new Date()) {
      appStore.showError(t('admin.subscriptions.adjustWouldExpire'))
      return
    }
  }

  submitting.value = true
  try {
    await adminAPI.subscriptions.extend(props.subscription.id, {
      days: form.days
    })
    appStore.showSuccess(t('admin.subscriptions.subscriptionAdjusted'))
    emit('saved')
    emit('close')
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.subscriptions.failedToAdjust'))
    console.error('Error adjusting subscription:', error)
  } finally {
    submitting.value = false
  }
}
</script>
