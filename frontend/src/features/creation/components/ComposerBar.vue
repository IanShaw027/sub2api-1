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
      class="studio-composer-input"
      rows="3"
      :placeholder="store.isImageSession ? t('studio.composer.placeholderImage') : t('studio.composer.placeholderChat')"
      :disabled="store.streaming || (store.isImageSession && !store.hasImageModels)"
      @keydown.enter.exact.prevent="submit"
    />
    <div class="studio-composer-actions">
      <Button
        variant="primary"
        :loading="store.streaming"
        :disabled="!canSubmit"
        @click="submit"
      >
        {{ store.streaming ? t('studio.composer.sending') : t('studio.composer.send') }}
      </Button>
      <Button v-if="store.lastFailedSend" variant="secondary" @click="store.retryLastFailed">
        {{ t('studio.retry') }}
      </Button>
    </div>
    <p v-if="store.isImageSession && !store.hasImageModels" class="text-xs text-danger">
      {{ t('studio.errors.noImageModels') }}
    </p>
    <p v-else-if="store.error" class="text-xs text-danger">{{ store.error }}</p>
  </div>
</template>

<style scoped>
.studio-composer {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.studio-composer-input {
  width: 100%;
  min-height: 88px;
  resize: vertical;
  border-radius: var(--radius-field);
  border: 1px solid var(--border);
  background: color-mix(in oklch, var(--surface) 85%, transparent);
  color: var(--foreground);
  padding: 12px 14px;
  font-size: 14px;
  box-shadow: var(--field-shadow);
}

.studio-composer-input:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: var(--field-shadow), 0 0 0 3px color-mix(in oklch, var(--accent) 18%, transparent);
}

.studio-composer-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}
</style>
