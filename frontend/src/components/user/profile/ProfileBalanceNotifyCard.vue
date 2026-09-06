<template>
 <div v-if="!props.embedded" class="glass-card">
 <div class="border-b border-line px-6 py-4">
 <h2 class="text-lg font-medium text-foreground">
 {{ t('profile.balanceNotify.title') }}
 </h2>
 <p class="mt-1 text-sm text-muted">
 {{ t('profile.balanceNotify.description') }}
 </p>
 </div>
 <div class="px-6 py-6 space-y-6">
 <!-- Enable toggle -->
 <div class="flex items-center justify-between">
 <label class="input-label mb-0">{{ t('profile.balanceNotify.enabled') }}</label>
 <label class="relative inline-flex items-center cursor-pointer">
 <input type="checkbox" v-model="notifyEnabled" @change="handleToggle" class="sr-only peer" />
 <div class="w-11 h-6 bg-surface-2 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-accent rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-surface after:border-line after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-accent"></div>
 </label>
 </div>

 <template v-if="notifyEnabled">
 <!-- Custom threshold with save button -->
 <div>
 <label class="input-label">
 {{ t('profile.balanceNotify.threshold') }}
 <span class="text-xs text-muted ml-2">{{ t('profile.balanceNotify.thresholdHint') }}</span>
 </label>
 <div class="flex items-center gap-2">
 <span class="text-muted">$</span>
 <input
 v-model.number="customThreshold"
 type="number"
 min="0"
 step="0.01"
 class="input flex-1"
 :placeholder="systemDefaultThreshold > 0 ? `${t('profile.balanceNotify.systemDefault')} $${systemDefaultThreshold}` : t('profile.balanceNotify.thresholdPlaceholder')"
 />
 <button
 @click="handleThresholdUpdate"
 :disabled="savingThreshold"
 class="btn btn-primary btn-sm whitespace-nowrap"
 >
 {{ savingThreshold ? t('common.saving') : t('common.save') }}
 </button>
 </div>
 </div>

 <!-- Email list with toggles -->
 <div>
 <label class="input-label">{{ t('profile.balanceNotify.extraEmails') }}</label>
 <p class="mb-2 text-xs text-warning-600">{{ t('profile.balanceNotify.extraEmailsHint') }}</p>

 <!-- Saved email entries -->
 <div v-if="emailEntries.length > 0" class="space-y-2 mb-3">
 <div v-for="(entry, idx) in emailEntries" :key="idx"
 class="flex items-center justify-between px-3 py-2 bg-surface-2 rounded-lg">
 <div class="flex items-center gap-2 min-w-0 flex-1">
 <label class="relative inline-flex items-center cursor-pointer shrink-0">
 <input type="checkbox" :checked="!entry.disabled" @change="handleEmailToggle(entry)" class="sr-only peer" />
 <div class="w-9 h-5 bg-surface-2 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-surface after:border-line after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-accent"></div>
 </label>
 <span class="text-sm text-foreground truncate">{{ entry.email }}</span>
 </div>
 <div class="flex items-center gap-2 shrink-0">
 <template v-if="!entry.verified">
 <!-- Inline verify flow for saved unverified emails -->
 <template v-if="verifyingEmail === entry.email">
 <input
 v-model="verifyCode"
 type="text"
 maxlength="6"
 class="w-20 rounded border border-line px-2 py-1 text-xs"
 :placeholder="t('profile.balanceNotify.codePlaceholder')"
 />
 <button @click="verifySavedEmail(entry.email)" :disabled="!verifyCode || verifyCode.length !== 6 || verifyingSaved" class="text-xs text-accent hover:text-accent">
 {{ t('profile.balanceNotify.verify') }}
 </button>
 <span v-if="verifyCountdown > 0" class="text-xs text-muted">{{ verifyCountdown }}s</span>
 <button v-else @click="sendCodeForSaved(entry.email)" :disabled="sendingSavedCode" class="text-xs text-muted hover:text-foreground">
 {{ t('profile.balanceNotify.resend') }}
 </button>
 <button @click="verifyingEmail = ''" class="text-xs text-muted hover:text-muted">
 {{ t('common.cancel') }}
 </button>
 </template>
 <template v-else>
 <button @click="sendCodeForSaved(entry.email)" :disabled="sendingSavedCode" class="text-xs text-accent hover:text-accent">
 {{ t('profile.balanceNotify.verify') }}
 </button>
 <span class="text-xs text-warning-500">{{ t('profile.balanceNotify.unverified') }}</span>
 </template>
 </template>
 <span v-else class="text-xs text-success-500">{{ t('profile.balanceNotify.verified') }}</span>
 <button @click="handleRemoveEmail(entry.email)" class="text-danger-500 hover:text-danger-700 text-xs">
 {{ t('profile.balanceNotify.removeEmail') }}
 </button>
 </div>
 </div>
 </div>

 <!-- Pending (unverified) emails in verification flow -->
 <div v-if="pendingEmails.length > 0" class="space-y-2 mb-3">
 <div v-for="(pe, idx) in pendingEmails" :key="pe.email"
 class="flex items-center gap-2 px-3 py-2 bg-warning-50 rounded-lg border border-warning-200">
 <span class="flex-1 text-sm text-foreground">{{ pe.email }}</span>
 <div v-if="!pe.codeSent" class="flex items-center gap-1">
 <button @click="sendCodeFor(idx)" :disabled="pe.sending" class="text-xs text-accent hover:text-accent">
 {{ t('profile.balanceNotify.sendCode') }}
 </button>
 <button @click="pendingEmails.splice(idx, 1)" class="text-xs text-danger-500 hover:text-danger-700 ml-1">
 {{ t('profile.balanceNotify.removeEmail') }}
 </button>
 </div>
 <div v-else class="flex items-center gap-1">
 <input
 v-model="pe.code"
 type="text"
 maxlength="6"
 class="w-20 rounded border border-line px-2 py-1 text-xs"
 :placeholder="t('profile.balanceNotify.codePlaceholder')"
 />
 <button @click="verifyPending(idx)" :disabled="!pe.code || pe.code.length !== 6 || pe.verifying" class="text-xs text-accent hover:text-accent">
 {{ t('profile.balanceNotify.verify') }}
 </button>
 <span v-if="pe.countdown > 0" class="text-xs text-muted">{{ pe.countdown }}s</span>
 <button v-else @click="sendCodeFor(idx)" :disabled="pe.sending" class="text-xs text-muted hover:text-foreground">
 {{ t('profile.balanceNotify.resend') }}
 </button>
 </div>
 </div>
 </div>

 <!-- Add new email input (hidden when at limit) -->
 <div v-if="canAddMore" class="flex gap-2">
 <input
 v-model="newEmail"
 type="email"
 class="input flex-1"
 :placeholder="t('profile.balanceNotify.emailPlaceholder')"
 @keyup.enter="addPendingEmail"
 />
 <button
 @click="addPendingEmail"
 :disabled="!newEmail"
 class="btn btn-secondary whitespace-nowrap"
 >
 {{ t('common.add') }}
 </button>
 </div>
 <p v-else class="text-xs text-muted">
 {{ t('profile.balanceNotify.maxEmailsReached') }}
 </p>
 </div>
 </template>
 </div>
 </div>

 <template v-else>
 <!-- 通知开关 -->
 <SettingRow :label="t('profile.balanceNotify.enabled')">
 <ToggleSwitch :model-value="notifyEnabled" @update:model-value="onToggleChange" />
 </SettingRow>

 <template v-if="notifyEnabled">
 <!-- 阈值 -->
 <SettingRow
 :label="t('profile.balanceNotify.threshold')"
 :description="t('profile.balanceNotify.thresholdHint')"
 >
 <div class="flex items-center gap-2">
 <span class="text-muted">$</span>
 <TextInput
 :model-value="customThreshold ?? ''"
 type="number"
 min="0"
 step="0.01"
 :placeholder="thresholdPlaceholder"
 @update:model-value="onThresholdInput"
 />
 <button
 type="button"
 @click="handleThresholdUpdate"
 :disabled="savingThreshold"
 class="btn btn-primary btn-sm whitespace-nowrap"
 >
 {{ savingThreshold ? t('common.saving') : t('common.save') }}
 </button>
 </div>
 </SettingRow>

 <!-- 备用通知邮箱 -->
 <SettingRow
 :label="t('profile.balanceNotify.extraEmails')"
 :description="t('profile.balanceNotify.extraEmailsHint')"
 >
 <div class="space-y-3">
 <!-- Saved email entries -->
 <div v-if="emailEntries.length > 0" class="space-y-2">
 <div v-for="(entry, idx) in emailEntries" :key="idx"
 class="flex items-center justify-between px-3 py-2 bg-surface-2 rounded-lg">
 <div class="flex items-center gap-2 min-w-0 flex-1">
 <label class="relative inline-flex items-center cursor-pointer shrink-0">
 <input type="checkbox" :checked="!entry.disabled" @change="handleEmailToggle(entry)" class="sr-only peer" />
 <div class="w-9 h-5 bg-surface-2 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-surface after:border-line after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-accent"></div>
 </label>
 <span class="text-sm text-foreground truncate">{{ entry.email }}</span>
 </div>
 <div class="flex items-center gap-2 shrink-0">
 <template v-if="!entry.verified">
 <!-- Inline verify flow for saved unverified emails -->
 <template v-if="verifyingEmail === entry.email">
 <input
 v-model="verifyCode"
 type="text"
 maxlength="6"
 class="w-20 rounded border border-line px-2 py-1 text-xs"
 :placeholder="t('profile.balanceNotify.codePlaceholder')"
 />
 <button @click="verifySavedEmail(entry.email)" :disabled="!verifyCode || verifyCode.length !== 6 || verifyingSaved" class="text-xs text-accent hover:text-accent">
 {{ t('profile.balanceNotify.verify') }}
 </button>
 <span v-if="verifyCountdown > 0" class="text-xs text-muted">{{ verifyCountdown }}s</span>
 <button v-else @click="sendCodeForSaved(entry.email)" :disabled="sendingSavedCode" class="text-xs text-muted hover:text-foreground">
 {{ t('profile.balanceNotify.resend') }}
 </button>
 <button @click="verifyingEmail = ''" class="text-xs text-muted hover:text-muted">
 {{ t('common.cancel') }}
 </button>
 </template>
 <template v-else>
 <button @click="sendCodeForSaved(entry.email)" :disabled="sendingSavedCode" class="text-xs text-accent hover:text-accent">
 {{ t('profile.balanceNotify.verify') }}
 </button>
 <span class="text-xs text-warning-500">{{ t('profile.balanceNotify.unverified') }}</span>
 </template>
 </template>
 <span v-else class="text-xs text-success-500">{{ t('profile.balanceNotify.verified') }}</span>
 <button @click="handleRemoveEmail(entry.email)" class="text-danger-500 hover:text-danger-700 text-xs">
 {{ t('profile.balanceNotify.removeEmail') }}
 </button>
 </div>
 </div>
 </div>

 <!-- Pending (unverified) emails in verification flow -->
 <div v-if="pendingEmails.length > 0" class="space-y-2">
 <div v-for="(pe, idx) in pendingEmails" :key="pe.email"
 class="flex items-center gap-2 px-3 py-2 bg-warning-50 rounded-lg border border-warning-200">
 <span class="flex-1 text-sm text-foreground">{{ pe.email }}</span>
 <div v-if="!pe.codeSent" class="flex items-center gap-1">
 <button @click="sendCodeFor(idx)" :disabled="pe.sending" class="text-xs text-accent hover:text-accent">
 {{ t('profile.balanceNotify.sendCode') }}
 </button>
 <button @click="pendingEmails.splice(idx, 1)" class="text-xs text-danger-500 hover:text-danger-700 ml-1">
 {{ t('profile.balanceNotify.removeEmail') }}
 </button>
 </div>
 <div v-else class="flex items-center gap-1">
 <input
 v-model="pe.code"
 type="text"
 maxlength="6"
 class="w-20 rounded border border-line px-2 py-1 text-xs"
 :placeholder="t('profile.balanceNotify.codePlaceholder')"
 />
 <button @click="verifyPending(idx)" :disabled="!pe.code || pe.code.length !== 6 || pe.verifying" class="text-xs text-accent hover:text-accent">
 {{ t('profile.balanceNotify.verify') }}
 </button>
 <span v-if="pe.countdown > 0" class="text-xs text-muted">{{ pe.countdown }}s</span>
 <button v-else @click="sendCodeFor(idx)" :disabled="pe.sending" class="text-xs text-muted hover:text-foreground">
 {{ t('profile.balanceNotify.resend') }}
 </button>
 </div>
 </div>
 </div>

 <!-- Add new email input (hidden when at limit) -->
 <div v-if="canAddMore" class="flex gap-2">
 <input
 v-model="newEmail"
 type="email"
 class="input flex-1"
 :placeholder="t('profile.balanceNotify.emailPlaceholder')"
 @keyup.enter="addPendingEmail"
 />
 <button
 @click="addPendingEmail"
 :disabled="!newEmail"
 class="btn btn-secondary whitespace-nowrap"
 >
 {{ t('common.add') }}
 </button>
 </div>
 <p v-else class="text-xs text-muted">
 {{ t('profile.balanceNotify.maxEmailsReached') }}
 </p>
 </div>
 </SettingRow>
 </template>
 </template>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { userAPI } from '@/api'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { NotifyEmailEntry } from '@/types'
import SettingRow from '@/components/ui/SettingRow.vue'
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue'
import TextInput from '@/components/ui/TextInput.vue'

const maxTotalEmails = 3

interface PendingEmail {
 email: string
 codeSent: boolean
 code: string
 sending: boolean
 verifying: boolean
 countdown: number
 timer: ReturnType<typeof setInterval> | null
}

const props = withDefaults(defineProps<{
 enabled: boolean
 threshold: number | null
 extraEmails: NotifyEmailEntry[]
 systemDefaultThreshold: number
 userEmail: string
 embedded?: boolean
}>(), {
 embedded: false,
})

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const notifyEnabled = ref(props.enabled)
const customThreshold = ref<number | null>(props.threshold)
const emailEntries = ref<NotifyEmailEntry[]>([...props.extraEmails])
const pendingEmails = ref<PendingEmail[]>([])
const newEmail = ref('')
const savingThreshold = ref(false)

// State for verifying saved unverified emails
const verifyingEmail = ref('')
const verifyCode = ref('')
const verifyingSaved = ref(false)
const sendingSavedCode = ref(false)
const verifyCountdown = ref(0)
let verifyTimer: ReturnType<typeof setInterval> | null = null

const canAddMore = computed(() => {
 return emailEntries.value.length + pendingEmails.value.length < maxTotalEmails
})

const thresholdPlaceholder = computed(() =>
 props.systemDefaultThreshold > 0
 ? `${t('profile.balanceNotify.systemDefault')} $${props.systemDefaultThreshold}`
 : t('profile.balanceNotify.thresholdPlaceholder')
)

watch(() => props.enabled, (val) => { notifyEnabled.value = val })
watch(() => props.threshold, (val) => { customThreshold.value = val })
watch(() => props.extraEmails, (val) => { emailEntries.value = [...val] })

// When list is empty on mount, pre-fill the add input with user's email
onMounted(() => {
 if (emailEntries.value.length === 0 && props.userEmail) {
 newEmail.value = props.userEmail
 }
})

onUnmounted(() => {
 for (const pe of pendingEmails.value) {
 if (pe.timer) clearInterval(pe.timer)
 }
 if (verifyTimer) clearInterval(verifyTimer)
})

const handleToggle = async () => {
 try {
 const updated = await userAPI.updateProfile({ balance_notify_enabled: notifyEnabled.value })
 authStore.user = updated
 } catch (err: unknown) {
 appStore.showError(extractApiErrorMessage(err, t('common.error')))
 notifyEnabled.value = !notifyEnabled.value
 }
}

// ToggleSwitch 通过 update:model-value 传新值，语义等价于原生 checkbox 的 v-model + @change 组合
function onToggleChange(next: boolean) {
 notifyEnabled.value = next
 handleToggle()
}

// TextInput 在 type=number 且清空时会 emit 空字符串，这里归一化为 null
function onThresholdInput(value: string | number) {
 customThreshold.value = value === '' ? null : Number(value)
}

const handleThresholdUpdate = async () => {
 savingThreshold.value = true
 try {
 const threshold = customThreshold.value && customThreshold.value > 0 ? customThreshold.value : 0
 const updated = await userAPI.updateProfile({ balance_notify_threshold: threshold })
 authStore.user = updated
 appStore.showSuccess(t('common.saved'))
 } catch (err: unknown) {
 appStore.showError(extractApiErrorMessage(err, t('common.error')))
 } finally {
 savingThreshold.value = false
 }
}

async function handleEmailToggle(entry: NotifyEmailEntry) {
 const newDisabled = !entry.disabled
 try {
 const updated = await userAPI.toggleNotifyEmail(entry.email, newDisabled)
 authStore.user = updated
 emailEntries.value = [...updated.balance_notify_extra_emails]
 } catch (err: unknown) {
 appStore.showError(extractApiErrorMessage(err, t('common.error')))
 }
}

function addPendingEmail() {
 const email = newEmail.value.trim()
 if (!email) return
 // Check duplicates
 const isDuplicate = emailEntries.value.some(e => e.email.toLowerCase() === email.toLowerCase())
 || pendingEmails.value.some(p => p.email.toLowerCase() === email.toLowerCase())
 if (isDuplicate) {
 appStore.showError(t('profile.balanceNotify.emailDuplicate'))
 return
 }
 pendingEmails.value.push({ email, codeSent: false, code: '', sending: false, verifying: false, countdown: 0, timer: null })
 newEmail.value = ''
}

async function sendCodeFor(idx: number) {
 const pe = pendingEmails.value[idx]
 if (!pe) return
 pe.sending = true
 try {
 await userAPI.sendNotifyEmailCode(pe.email)
 pe.codeSent = true
 pe.countdown = 60
 pe.timer = setInterval(() => {
 pe.countdown--
 if (pe.countdown <= 0 && pe.timer) {
 clearInterval(pe.timer)
 pe.timer = null
 }
 }, 1000)
 appStore.showSuccess(t('profile.balanceNotify.codeSent'))
 } catch (err: unknown) {
 appStore.showError(extractApiErrorMessage(err, t('common.error')))
 } finally {
 pe.sending = false
 }
}

async function verifyPending(idx: number) {
 const pe = pendingEmails.value[idx]
 if (!pe || !pe.code || pe.code.length !== 6) return
 pe.verifying = true
 try {
 await userAPI.verifyNotifyEmail(pe.email, pe.code)
 if (pe.timer) clearInterval(pe.timer)
 pendingEmails.value.splice(idx, 1)
 appStore.showSuccess(t('profile.balanceNotify.verifySuccess'))
 const updated = await userAPI.getProfile()
 authStore.user = updated
 emailEntries.value = [...updated.balance_notify_extra_emails]
 } catch (err: unknown) {
 appStore.showError(extractApiErrorMessage(err, t('common.error')))
 } finally {
 pe.verifying = false
 }
}

const handleRemoveEmail = async (email: string) => {
 try {
 await userAPI.removeNotifyEmail(email)
 appStore.showSuccess(t('profile.balanceNotify.removeSuccess'))
 const updated = await userAPI.getProfile()
 authStore.user = updated
 emailEntries.value = [...updated.balance_notify_extra_emails]
 } catch (err: unknown) {
 appStore.showError(extractApiErrorMessage(err, t('common.error')))
 }
}

// Verify saved unverified emails
async function sendCodeForSaved(email: string) {
 sendingSavedCode.value = true
 try {
 await userAPI.sendNotifyEmailCode(email)
 verifyingEmail.value = email
 verifyCode.value = ''
 verifyCountdown.value = 60
 if (verifyTimer) clearInterval(verifyTimer)
 verifyTimer = setInterval(() => {
 verifyCountdown.value--
 if (verifyCountdown.value <= 0 && verifyTimer) {
 clearInterval(verifyTimer)
 verifyTimer = null
 }
 }, 1000)
 appStore.showSuccess(t('profile.balanceNotify.codeSent'))
 } catch (err: unknown) {
 appStore.showError(extractApiErrorMessage(err, t('common.error')))
 } finally {
 sendingSavedCode.value = false
 }
}

async function verifySavedEmail(email: string) {
 if (!verifyCode.value || verifyCode.value.length !== 6) return
 verifyingSaved.value = true
 try {
 await userAPI.verifyNotifyEmail(email, verifyCode.value)
 verifyingEmail.value = ''
 verifyCode.value = ''
 if (verifyTimer) { clearInterval(verifyTimer); verifyTimer = null }
 appStore.showSuccess(t('profile.balanceNotify.verifySuccess'))
 const updated = await userAPI.getProfile()
 authStore.user = updated
 emailEntries.value = [...updated.balance_notify_extra_emails]
 } catch (err: unknown) {
 appStore.showError(extractApiErrorMessage(err, t('common.error')))
 } finally {
 verifyingSaved.value = false
 }
}
</script>
