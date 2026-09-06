<template>
 <div :class="props.embedded ? 'space-y-1' : 'glass-card'">
 <div
 v-if="!props.embedded"
 class="border-b border-line px-6 py-4"
 >
 <h2 class="text-lg font-medium text-foreground">
 {{ t('profile.totp.title') }}
 </h2>
 <p class="mt-1 text-sm text-muted">
 {{ t('profile.totp.description') }}
 </p>
 </div>
 <div :class="props.embedded ? '' : 'px-6 py-6'">
 <!-- Loading state -->
 <div v-if="loading" class="flex items-center gap-3 py-2">
 <div class="h-5 w-5 animate-spin rounded-full border-b-2 border-accent"></div>
 <span class="text-sm text-muted">{{ t('common.loading') }}</span>
 </div>

 <!-- Feature disabled globally -->
 <div v-else-if="status && !status.feature_enabled" class="flex items-center justify-between gap-4">
 <div>
 <p class="font-medium text-foreground">
 {{ t('profile.totp.featureDisabled') }}
 </p>
 <p class="text-sm text-muted">
 {{ t('profile.totp.featureDisabledHint') }}
 </p>
 </div>
 <ToggleSwitch :model-value="false" disabled />
 </div>

 <!-- 2FA toggle: on click, open the matching wizard/confirm flow instead of flipping state directly -->
 <div v-else class="flex items-center justify-between gap-4">
 <div>
 <p class="font-medium text-foreground">
 {{ status?.enabled ? t('profile.totp.enabled') : t('profile.totp.notEnabled') }}
 </p>
 <p v-if="status?.enabled && status.enabled_at" class="text-sm text-muted">
 {{ t('profile.totp.enabledAt') }}: {{ formatDate(status.enabled_at) }}
 </p>
 <p v-else-if="!status?.enabled" class="text-sm text-muted">
 {{ t('profile.totp.notEnabledHint') }}
 </p>
 </div>
 <ToggleSwitch
 :model-value="!!status?.enabled"
 @update:model-value="handleToggle"
 />
 </div>
 </div>

 <!-- Setup Modal -->
 <TotpSetupModal
 :open="showSetupModal"
 @close="showSetupModal = false"
 @success="handleSetupSuccess"
 @changed="loadStatus"
 />

 <!-- Disable Dialog -->
 <TotpDisableDialog
 :open="showDisableDialog"
 @close="showDisableDialog = false"
 @success="handleDisableSuccess"
 @changed="loadStatus"
 />
 </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { totpAPI } from '@/api'
import type { TotpStatus } from '@/types'
import TotpSetupModal from './TotpSetupModal.vue'
import TotpDisableDialog from './TotpDisableDialog.vue'
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue'

const props = withDefaults(defineProps<{ embedded?: boolean }>(), {
 embedded: false,
})

const { t } = useI18n()

const loading = ref(true)
const status = ref<TotpStatus | null>(null)
const showSetupModal = ref(false)
const showDisableDialog = ref(false)
let statusRequestId = 0

// 点击开关不会直接切换状态，而是按当前状态打开设置向导或禁用确认弹窗；
// 真正的状态变更只会在弹窗流程成功后由 loadStatus() 刷新。
function handleToggle(next: boolean): void {
 if (next) {
 showSetupModal.value = true
 } else {
 showDisableDialog.value = true
 }
}

const loadStatus = async () => {
 const requestId = ++statusRequestId
 loading.value = true
 try {
 const result = await totpAPI.getStatus()
 if (requestId === statusRequestId) status.value = result
 } catch (error) {
 if (requestId !== statusRequestId) return
 console.error('Failed to load TOTP status:', error)
 } finally {
 if (requestId === statusRequestId) loading.value = false
 }
}

const handleSetupSuccess = () => {
 showSetupModal.value = false
}

const handleDisableSuccess = () => {
 showDisableDialog.value = false
}

const formatDate = (timestamp: number) => {
 // Backend returns Unix timestamp in seconds, convert to milliseconds
 const date = new Date(timestamp * 1000)
 return date.toLocaleDateString(undefined, {
 year: 'numeric',
 month: 'long',
 day: 'numeric',
 hour: '2-digit',
 minute: '2-digit'
 })
}

onMounted(() => {
 loadStatus()
})

onBeforeUnmount(() => { statusRequestId++ })
</script>
