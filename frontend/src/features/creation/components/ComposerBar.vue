<script setup lang="ts">
import { computed, ref, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import Button from '@/components/ui/Button.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import Icon from '@/components/icons/Icon.vue'
import ModelMenu from './ModelMenu.vue'
import { useCreationStore } from '../stores/creation'

const { t } = useI18n()
const store = useCreationStore()

const draft = ref('')
const composing = ref(false)
const inputRef = ref<HTMLTextAreaElement | null>(null)
const inputDisabled = computed(() => store.streaming || store.sessionLoading || store.messagesLoading || store.modelUpdating || !store.generationAvailable || store.hasUnsavedExchange || (store.isImageSession && !store.hasImageModels))

async function setDraft(text: string) {
  if (inputDisabled.value) return false
  draft.value = text
  await nextTick()
  inputRef.value?.focus()
  return true
}

defineExpose({ setDraft })

const groupOptions = computed(() =>
  store.groups.map((group) => ({ value: group.id, label: group.name })),
)

const canSubmit = computed(() => {
  if (!draft.value.trim() || store.streaming || store.sessionLoading || store.messagesLoading || store.modelUpdating || !store.selectedSessionId || !store.generationAvailable || store.hasUnsavedExchange) return false
  if (store.isImageSession && !store.hasImageModels) return false
  return true
})

async function onGroupChange(value: string | number | boolean | null) {
  if (typeof value === 'number') {
    try {
      await store.setGroupId(value)
    } catch {
      // Navigation errors are exposed by the store; superseded requests are ignored.
    }
  }
}

function onEnter(event: KeyboardEvent) {
  if (composing.value || event.isComposing || event.keyCode === 229) return
  event.preventDefault()
  void submit()
}

async function submit() {
  const text = draft.value.trim()
  if (!canSubmit.value) return
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
    <div class="studio-composer-toolbar">
      <UiSelect
        class="studio-composer-group"
        :model-value="store.groupId"
        :options="groupOptions"
        :placeholder="t('studio.selectGroup')"
        :aria-label="t('studio.groups')"
        searchable="auto"
        @update:model-value="onGroupChange"
      />
      <ModelMenu compact class="studio-composer-model" />
    </div>
    <textarea
      ref="inputRef"
      v-model="draft"
      class="field studio-composer-input"
      rows="3"
      :aria-label="t('studio.a11y.composerInput')"
      :placeholder="store.isImageSession ? t('studio.composer.placeholderImage') : t('studio.composer.placeholderChat')"
      :disabled="inputDisabled"
      @compositionstart="composing = true"
      @compositionend="composing = false"
      @keydown.enter.exact="onEnter"
    />
    <div class="studio-composer-actions">
      <Button
        class="studio-composer-send"
        variant="primary"
        :loading="store.streaming"
        :aria-busy="store.streaming"
        :disabled="!canSubmit"
        @click="submit"
      >
        <Icon v-if="!store.streaming" name="arrowUp" size="sm" />
        {{ store.streaming ? t('studio.composer.sending') : t('studio.composer.send') }}
      </Button>
      <Button v-if="store.lastFailedSend" variant="secondary" :loading="store.saving" :disabled="store.streaming || store.sessionLoading || store.modelUpdating || store.saving" @click="store.retryLastFailed">
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

.studio-composer-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.studio-composer-group,
.studio-composer-model {
  flex: 1;
  min-width: 140px;
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
