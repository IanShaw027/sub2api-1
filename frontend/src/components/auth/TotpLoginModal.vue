<template>
  <UiModal
    open
    :title="t('profile.totp.loginTitle')"
    width="sm"
    :close-on-overlay="false"
    @close="emit('cancel')"
  >
    <div class="totp-body">
      <p class="totp-hint">{{ t('profile.totp.loginHint') }}</p>
      <p v-if="userEmailMasked" class="totp-email">{{ userEmailMasked }}</p>

      <input
        ref="codeInputRef"
        v-model="code"
        type="text"
        inputmode="numeric"
        autocomplete="one-time-code"
        pattern="[0-9]*"
        maxlength="6"
        :disabled="verifying"
        class="field totp-code"
        placeholder="000000"
        @input="handleCodeInput"
        @paste="handlePaste"
      />

      <div v-if="verifying" class="totp-verifying">
        <span class="spinner" aria-hidden="true"></span>
        {{ t('common.verifying') }}
      </div>
    </div>

    <template #footer>
      <button
        type="button"
        class="btn btn-secondary btn-md totp-cancel"
        :disabled="verifying"
        @click="emit('cancel')"
      >
        {{ t('common.cancel') }}
      </button>
    </template>
  </UiModal>
</template>

<script setup lang="ts">
import { ref, watch, nextTick, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import UiModal from '@/components/ui/UiModal.vue'

defineProps<{
  tempToken: string
  userEmailMasked?: string
}>()

const emit = defineEmits<{
  verify: [code: string]
  cancel: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const verifying = ref(false)
const code = ref<string>('')
const codeInputRef = ref<HTMLInputElement | null>(null)

// Auto-submit as soon as six digits are present
watch(code, (newCode) => {
  if (newCode.length === 6 && !verifying.value) {
    emit('verify', newCode)
  }
})

defineExpose({
  setVerifying: (value: boolean) => {
    verifying.value = value
  },
  setError: (message: string) => {
    if (message) {
      appStore.showError(message)
    }
    code.value = ''
    if (codeInputRef.value) {
      codeInputRef.value.value = ''
    }
    nextTick(() => {
      codeInputRef.value?.focus()
    })
  }
})

function normalize(value: string): string {
  return value.replace(/[^0-9]/g, '').slice(0, 6)
}

function handleCodeInput(event: Event): void {
  const input = event.target as HTMLInputElement
  const normalized = normalize(input.value)
  input.value = normalized
  code.value = normalized
}

function handlePaste(event: ClipboardEvent): void {
  event.preventDefault()
  const normalized = normalize(event.clipboardData?.getData('text') || '')
  code.value = normalized
  if (codeInputRef.value) {
    codeInputRef.value.value = normalized
  }
}

onMounted(() => {
  nextTick(() => {
    codeInputRef.value?.focus()
  })
})
</script>

<style scoped>
.totp-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.totp-hint {
  margin: 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--muted);
}

.totp-email {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 13px;
  font-weight: 600;
}

.totp-code {
  height: 44px;
  line-height: 42px;
  border-radius: var(--radius-field);
  font-family: var(--font-mono);
  font-size: 18px;
  font-weight: 600;
  letter-spacing: 0.5em;
  text-indent: 0.5em;
  text-align: center;
}

.totp-verifying {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  font-size: 12.5px;
  color: var(--muted);
}

.totp-cancel {
  width: 100%;
}
</style>
