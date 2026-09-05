<template>
 <div :class="props.embedded ? '' : 'card'">
 <div
 v-if="!props.embedded"
 class="border-b border-line px-6 py-4"
 >
 <h2 class="text-lg font-medium text-foreground">
 {{ t('profile.changePassword') }}
 </h2>
 </div>
 <div :class="props.embedded ? '' : 'px-6 py-6'">
 <form v-if="!props.embedded" @submit.prevent="handleChangePassword" class="space-y-4">
 <div>
 <label for="old_password" class="input-label">
 {{ t('profile.currentPassword') }}
 </label>
 <input
 id="old_password"
 v-model="form.old_password"
 type="password"
 required
 autocomplete="current-password"
 class="input"
 />
 </div>

 <div>
 <label for="new_password" class="input-label">
 {{ t('profile.newPassword') }}
 </label>
 <input
 id="new_password"
 v-model="form.new_password"
 type="password"
 required
 autocomplete="new-password"
 class="input"
 />
 <p class="input-hint">
 {{ t('profile.passwordHint') }}
 </p>
 </div>

 <div>
 <label for="confirm_password" class="input-label">
 {{ t('profile.confirmNewPassword') }}
 </label>
 <input
 id="confirm_password"
 v-model="form.confirm_password"
 type="password"
 required
 autocomplete="new-password"
 class="input"
 />
 </div>

 <div class="flex justify-end pt-4">
 <button type="submit" :disabled="loading" class="btn btn-primary">
 {{ loading ? t('profile.changingPassword') : t('profile.changePasswordButton') }}
 </button>
 </div>
 </form>

 <!-- embedded: each field gets its own settings-row-style grid line instead of
 being packed into a single tall SettingRow control column (which vertically
 centers a 3-field form far from its single label). Mirrors `.ui-setting-row`'s
 240px/1fr grid without depending on the shared component instance. -->
 <form v-else @submit.prevent="handleChangePassword" class="flex flex-col">
 <div class="grid grid-cols-1 items-center gap-2 border-b border-line px-5 py-3.5 md:grid-cols-[240px_minmax(0,1fr)] md:gap-6">
 <label for="old_password" class="text-[13px] font-semibold text-foreground">
 {{ t('profile.currentPassword') }}
 </label>
 <input
 id="old_password"
 v-model="form.old_password"
 type="password"
 required
 autocomplete="current-password"
 class="input max-w-[420px]"
 />
 </div>

 <div class="grid grid-cols-1 items-center gap-2 border-b border-line px-5 py-3.5 md:grid-cols-[240px_minmax(0,1fr)] md:gap-6">
 <label for="new_password" class="text-[13px] font-semibold text-foreground">
 {{ t('profile.newPassword') }}
 </label>
 <div class="min-w-0">
 <input
 id="new_password"
 v-model="form.new_password"
 type="password"
 required
 autocomplete="new-password"
 class="input max-w-[420px]"
 />
 <p class="input-hint">
 {{ t('profile.passwordHint') }}
 </p>
 </div>
 </div>

 <div class="grid grid-cols-1 items-center gap-2 px-5 py-3.5 md:grid-cols-[240px_minmax(0,1fr)] md:gap-6">
 <label for="confirm_password" class="text-[13px] font-semibold text-foreground">
 {{ t('profile.confirmNewPassword') }}
 </label>
 <input
 id="confirm_password"
 v-model="form.confirm_password"
 type="password"
 required
 autocomplete="new-password"
 class="input max-w-[420px]"
 />
 </div>

 <div class="flex justify-end px-5 pb-4 pt-3">
 <button type="submit" :disabled="loading" class="btn btn-primary">
 {{ loading ? t('profile.changingPassword') : t('profile.changePasswordButton') }}
 </button>
 </div>
 </form>
 </div>
 </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { userAPI } from '@/api'

const { t } = useI18n()
const appStore = useAppStore()
const props = withDefaults(defineProps<{
 embedded?: boolean
}>(), {
 embedded: false,
})

const loading = ref(false)
const form = ref({
 old_password: '',
 new_password: '',
 confirm_password: ''
})

const handleChangePassword = async () => {
 if (form.value.new_password !== form.value.confirm_password) {
 appStore.showError(t('profile.passwordsNotMatch'))
 return
 }

 if (form.value.new_password.length < 8) {
 appStore.showError(t('profile.passwordTooShort'))
 return
 }

 loading.value = true
 try {
 await userAPI.changePassword(form.value.old_password, form.value.new_password)
 form.value = { old_password: '', new_password: '', confirm_password: '' }
 appStore.showSuccess(t('profile.passwordChangeSuccess'))
 } catch (error: any) {
 appStore.showError(error.response?.data?.detail || t('profile.passwordChangeFailed'))
 } finally {
 loading.value = false
 }
}
</script>
