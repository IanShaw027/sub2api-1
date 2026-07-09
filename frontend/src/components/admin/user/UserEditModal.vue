<template>
  <BaseDialog
    :show="show"
    :title="t('admin.users.editUser')"
    width="normal"
    @close="$emit('close')"
  >
    <form v-if="user" id="edit-user-form" @submit.prevent="handleUpdateUser" class="space-y-5">
      <div class="flex items-center gap-4 rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-800/60">
        <div class="h-14 w-14 overflow-hidden rounded-full bg-primary-100 dark:bg-primary-900/30">
          <img
            v-if="safeImageUrl(user.avatar_url)"
            data-test="user-avatar"
            :src="safeImageUrl(user.avatar_url)"
            :alt="user.email"
            class="h-full w-full object-cover"
            referrerpolicy="no-referrer"
            loading="lazy"
          />
          <div v-else class="flex h-full w-full items-center justify-center text-lg font-semibold text-primary-700 dark:text-primary-300">
            {{ user.email.charAt(0).toUpperCase() }}
          </div>
        </div>
        <div class="min-w-0">
          <div class="text-sm font-medium text-gray-900 dark:text-white">{{ user.email }}</div>
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ user.username || '-' }}</div>
          <div v-if="identityCards.length > 0" class="mt-2 flex flex-wrap gap-2">
            <div
              v-for="card in identityCards"
              :key="card.provider"
              class="flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-2 py-1 dark:border-dark-700 dark:bg-dark-900"
            >
              <img
                v-if="safeImageUrl(card.avatar_url)"
                :data-test="`identity-avatar-${card.provider}`"
                :src="safeImageUrl(card.avatar_url)"
                :alt="card.display_name || card.provider"
                class="h-6 w-6 rounded-full object-cover"
                referrerpolicy="no-referrer"
                loading="lazy"
              />
              <div class="min-w-0">
                <div class="truncate text-xs font-medium text-gray-700 dark:text-gray-200">
                  {{ card.display_name || card.provider }}
                </div>
                <div class="truncate text-[11px] text-gray-500 dark:text-gray-400">
                  {{ card.subject_hint || card.provider_key || '-' }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
      <div>
        <label class="input-label">{{ t('admin.users.email') }}</label>
        <input v-model="form.email" type="email" class="input" />
      </div>
      <div>
        <label class="input-label">{{ t('admin.users.password') }}</label>
        <div class="flex gap-2">
          <div class="relative flex-1">
            <input v-model="form.password" type="text" class="input pr-10" :placeholder="t('admin.users.enterNewPassword')" />
            <button v-if="form.password" type="button" @click="copyPassword" class="absolute right-2 top-1/2 -translate-y-1/2 rounded-lg p-1 transition-colors hover:bg-gray-100 dark:hover:bg-dark-700" :class="passwordCopied ? 'text-green-500' : 'text-gray-400'">
              <svg v-if="passwordCopied" class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" /></svg>
              <svg v-else class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M15.666 3.888A2.25 2.25 0 0013.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 01-.75.75H9a.75.75 0 01-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 01-2.25 2.25H6.75A2.25 2.25 0 014.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 011.927-.184" /></svg>
            </button>
          </div>
          <button type="button" @click="generatePassword" class="btn btn-secondary px-3">
            <Icon name="refresh" size="md" />
          </button>
        </div>
      </div>
      <div>
        <label class="input-label">{{ t('admin.users.username') }}</label>
        <input v-model="form.username" type="text" class="input" />
      </div>
      <div>
        <label class="input-label">{{ t('admin.users.notes') }}</label>
        <textarea v-model="form.notes" rows="3" class="input"></textarea>
      </div>
      <div>
        <label class="input-label">{{ t('admin.users.columns.concurrency') }}</label>
        <input v-model.number="form.concurrency" type="number" class="input" />
      </div>
      <div>
        <label class="input-label">{{ t('admin.users.form.rpmLimit') }}</label>
        <input
          v-model.number="form.rpm_limit"
          type="number"
          min="0"
          step="1"
          class="input"
          :placeholder="t('admin.users.form.rpmLimitPlaceholder')"
        />
        <p class="input-hint">{{ t('admin.users.form.rpmLimitHint') }}</p>
      </div>
      <UserAttributeForm v-model="form.customAttributes" :user-id="user?.id" />
    </form>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button @click="$emit('close')" type="button" class="btn btn-secondary">{{ t('common.cancel') }}</button>
        <button type="submit" form="edit-user-form" :disabled="submitting" class="btn btn-primary">
          {{ submitting ? t('admin.users.updating') : t('common.update') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { adminAPI } from '@/api/admin'
import type { AdminUser, UserAttributeValuesMap } from '@/types'
import { safeImageUrl } from '@/utils/safeImageUrl'
import BaseDialog from '@/components/common/BaseDialog.vue'
import UserAttributeForm from '@/components/user/UserAttributeForm.vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{ show: boolean, user: AdminUser | null }>()
const emit = defineEmits(['close', 'success'])
const { t } = useI18n(); const appStore = useAppStore(); const { copyToClipboard } = useClipboard()

const submitting = ref(false); const passwordCopied = ref(false)
const identityCards = computed(() => {
  const bindings = props.user?.auth_bindings || props.user?.identity_bindings || {}
  return Object.entries(bindings)
    .map(([provider, binding]) => {
      if (provider === 'email') {
        return null
      }
      if (!binding || typeof binding !== 'object') {
        return null
      }
      const raw = binding as Record<string, string | boolean | null | undefined>
      if (raw.bound !== true) {
        return null
      }
      return {
        provider,
        display_name: typeof raw.display_name === 'string' ? raw.display_name : provider,
        avatar_url: typeof raw.avatar_url === 'string' ? raw.avatar_url : '',
        subject_hint: typeof raw.subject_hint === 'string' ? raw.subject_hint : '',
        provider_key: typeof raw.provider_key === 'string' ? raw.provider_key : ''
      }
    })
    .filter((item): item is NonNullable<typeof item> => Boolean(item))
})
const form = reactive({ email: '', password: '', username: '', notes: '', concurrency: 1, rpm_limit: 0, customAttributes: {} as UserAttributeValuesMap })

watch(() => props.user, (u) => {
  if (u) {
    Object.assign(form, { email: u.email, password: '', username: u.username || '', notes: u.notes || '', concurrency: u.concurrency, rpm_limit: u.rpm_limit ?? 0, customAttributes: {} })
    passwordCopied.value = false
  }
}, { immediate: true })

const generatePassword = () => {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789!@#$%^&*'
  let p = ''; for (let i = 0; i < 16; i++) p += chars.charAt(Math.floor(Math.random() * chars.length))
  form.password = p
}
const copyPassword = async () => {
  if (form.password && await copyToClipboard(form.password, t('common.copiedToClipboard'))) {
    passwordCopied.value = true; setTimeout(() => passwordCopied.value = false, 2000)
  }
}
const handleUpdateUser = async () => {
  if (!props.user) return
  if (!form.email.trim()) {
    appStore.showError(t('admin.users.emailRequired'))
    return
  }
  if (form.concurrency < 1) {
    appStore.showError(t('admin.users.concurrencyMin'))
    return
  }
  submitting.value = true
  try {
    const data: any = { email: form.email, username: form.username, notes: form.notes, concurrency: form.concurrency, rpm_limit: form.rpm_limit }
    if (form.password.trim()) data.password = form.password.trim()
    await adminAPI.users.update(props.user.id, data)
    if (Object.keys(form.customAttributes).length > 0) await adminAPI.userAttributes.updateUserAttributeValues(props.user.id, form.customAttributes)
    appStore.showSuccess(t('admin.users.userUpdated'))
    emit('success'); emit('close')
  } catch (e: any) {
    appStore.showError(e.response?.data?.detail || t('admin.users.failedToUpdate'))
  } finally { submitting.value = false }
}
</script>
