<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.inspectDeviceProfile')"
    width="normal"
    @close="handleClose"
  >
    <div class="space-y-4">
      <div v-if="loading" class="flex items-center justify-center py-8">
        <svg class="h-6 w-6 animate-spin text-muted" fill="none" viewBox="0 0 24 24">
          <circle
            class="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            stroke-width="4"
          ></circle>
          <path
            class="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          ></path>
        </svg>
      </div>

      <div
        v-else-if="errorMessage"
        class="rounded-lg border border-amber-200 bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] p-3 text-sm text-amber-800"
      >
        {{ errorMessage }}
      </div>

      <div v-else-if="profile" class="space-y-3" data-testid="device-profile-inspect">
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div class="rounded-lg border border-line p-3">
            <p class="text-xs text-muted">
              {{ t('admin.accounts.deviceProfile.platform') }}
            </p>
            <p class="mt-1 text-sm font-medium text-foreground">
              {{ display(profile.platform) }}
            </p>
          </div>
          <div class="rounded-lg border border-line p-3">
            <p class="text-xs text-muted">
              {{ t('admin.accounts.deviceProfile.clientVersion') }}
            </p>
            <p class="mt-1 text-sm font-medium text-foreground">
              {{ display(profile.client_version) }}
            </p>
          </div>
          <div class="rounded-lg border border-line p-3">
            <p class="text-xs text-muted">
              {{ t('admin.accounts.deviceProfile.revision') }}
            </p>
            <p class="mt-1 text-sm font-medium text-foreground">
              {{ profile.revision }}
            </p>
          </div>
          <div class="rounded-lg border border-line p-3">
            <p class="text-xs text-muted">
              {{ t('admin.accounts.deviceProfile.learnedFrom') }}
            </p>
            <p class="mt-1 text-sm font-medium text-foreground">
              {{ display(profile.learned_from) }}
            </p>
          </div>
          <div class="rounded-lg border border-line p-3">
            <p class="text-xs text-muted">
              {{ t('admin.accounts.deviceProfile.learningEnabled') }}
            </p>
            <p class="mt-1 text-sm font-medium text-foreground">
              {{
                profile.learning_enabled
                  ? t('admin.accounts.deviceProfile.learningOn')
                  : t('admin.accounts.deviceProfile.learningOff')
              }}
            </p>
          </div>
          <div class="rounded-lg border border-line p-3">
            <p class="text-xs text-muted">
              {{ t('admin.accounts.deviceProfile.transportFamily') }}
            </p>
            <p class="mt-1 text-sm font-medium text-foreground">
              {{ display(profile.transport_family) }}
            </p>
          </div>
        </div>
        <div class="rounded-lg border border-line p-3">
          <p class="text-xs text-muted">
            {{ t('admin.accounts.deviceProfile.userAgent') }}
          </p>
          <p class="mt-1 break-all text-sm font-medium text-foreground">
            {{ display(profile.user_agent) }}
          </p>
        </div>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { Account, AccountDeviceProfile } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{
  show: boolean
  account: Account | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()
const loading = ref(false)
const profile = ref<AccountDeviceProfile | null>(null)
const errorMessage = ref('')

const display = (value: string | null | undefined) => (value && value.trim() ? value : '—')

const loadProfile = async (accountId: number) => {
  loading.value = true
  profile.value = null
  errorMessage.value = ''
  try {
    profile.value = await adminAPI.accounts.getDeviceProfile(accountId)
  } catch (error: unknown) {
    errorMessage.value = extractApiErrorMessage(
      error,
      t('admin.accounts.deviceProfile.loadFailed')
    )
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.show, props.account?.id] as const,
  ([visible, accountId]) => {
    if (visible && typeof accountId === 'number') {
      void loadProfile(accountId)
    }
    if (!visible) {
      profile.value = null
      errorMessage.value = ''
    }
  },
  { immediate: true }
)

const handleClose = () => {
  emit('close')
}
</script>
