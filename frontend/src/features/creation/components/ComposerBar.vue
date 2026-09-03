<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Button from '@/components/ui/Button.vue'
import { useCreationStore } from '../stores/creation'

const { t } = useI18n()
const store = useCreationStore()

const draft = ref('')

const canSubmit = computed(() => {
  if (!draft.value.trim() || store.streaming) return false
  if (store.isImageSession && !store.hasImageModels) return false
  return true
})

async function submit() {
  const text = draft.value.trim()
  if (!text || store.streaming) return
  if (store.isImageSession && !store.hasImageModels) return
  draft.value = ''
  try {
    await store.submitText(text)
  } catch {
    draft.value = text
  }
}
</script>

<template>
  <div class="studio-composer">
    <textarea
      v-model="draft"
      class="field studio-composer-input"
      rows="3"
      :aria-label="t('studio.a11y.composerInput')"
      :placeholder="store.isImageSession ? t('studio.composer.placeholderImage') : t('studio.composer.placeholderChat')"
      :disabled="store.streaming || (store.isImageSession && !store.hasImageModels)"
      @keydown.enter.exact.prevent="submit"
    />
    <div class="studio-composer-actions">
      <Button
        variant="primary"
        :loading="store.streaming"
        :aria-busy="store.streaming"
        :disabled="!canSubmit"
        @click="submit"
      >
        {{ store.streaming ? t('studio.composer.sending') : t('studio.composer.send') }}
      </Button>
      <Button v-if="store.lastFailedSend" variant="secondary" @click="store.retryLastFailed">
        {{ t('studio.retry') }}
      </Button>
    </div>
    <p
      v-if="store.isImageSession && !store.hasImageModels"
      class="text-xs text-danger"
      role="alert"
      aria-live="assertive"
    >
      {{ t('studio.errors.noImageModels') }}
    </p>
    <p v-else-if="store.error" class="text-xs text-danger" role="alert" aria-live="assertive">
      {{ store.error }}
    </p>
  </div>
</template>

<style scoped>
.studio-composer {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.studio-composer-input {
  min-height: 96px;
  padding: 10px 12px;
  font-size: 13px;
}

.studio-composer-actions {
  display: flex;
  gap: 8px;
  align-items: center;
  justify-content: flex-end;
}
</style>
