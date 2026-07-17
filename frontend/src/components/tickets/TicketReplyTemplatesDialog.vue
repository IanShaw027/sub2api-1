<template>
  <BaseDialog :show="show" :title="t('tickets.templates.manageTitle')" width="wide" @close="emit('close')">
    <div class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <p class="text-sm text-ink-soft">{{ t('tickets.templates.manageDescription') }}</p>
        <div class="flex items-center gap-2">
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="selectedIds.size === 0 || saving"
            @click="removeSelected"
          >
            {{ t('tickets.templates.batchDelete') }}
          </button>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="saving" @click="addTemplate">
            {{ t('tickets.templates.add') }}
          </button>
        </div>
      </div>

      <div v-if="localTemplates.length === 0" class="rounded-card border border-dashed p-6 text-center text-sm text-ink-soft dark:border-dark-700">
        {{ t('tickets.templates.empty') }}
      </div>

      <div v-else class="max-h-[60vh] space-y-4 overflow-y-auto pr-1">
        <div
          v-for="(template, index) in localTemplates"
          :key="template.id"
          class="rounded-card border border-line bg-page/60 p-4 dark:border-dark-700 dark:bg-dark-900/40"
        >
          <div class="mb-3 flex items-start gap-3">
            <input
              v-model="selectedTemplateIds"
              class="mt-1 h-4 w-4 rounded border-line text-brand-600 focus:ring-accent/25"
              type="checkbox"
              :value="template.id"
            />
            <div class="grid flex-1 gap-3">
              <div>
                <label class="mb-1 block text-xs font-medium text-ink-soft">{{ t('tickets.templates.title') }}</label>
                <input
                  v-model="template.title"
                  class="input"
                  :placeholder="t('tickets.templates.titlePlaceholder')"
                />
              </div>
              <div>
                <label class="mb-1 block text-xs font-medium text-ink-soft">{{ t('tickets.templates.content') }}</label>
                <textarea
                  v-model="template.content"
                  class="input min-h-[120px]"
                  :placeholder="t('tickets.templates.contentPlaceholder')"
                />
              </div>
            </div>
            <button
              type="button"
              class="rounded-lg px-2 py-1 text-sm text-red-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20"
              :aria-label="t('tickets.templates.deleteSingle')"
              @click="removeTemplate(index)"
            >
              ×
            </button>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex items-center justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="saving" @click="emit('close')">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-primary" :disabled="saving" @click="submit">
          {{ saving ? t('common.submitting') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { TicketReplyTemplate } from '@/api/adminTickets'

const props = defineProps<{
  show: boolean
  templates: TicketReplyTemplate[]
  saving?: boolean
}>()

const emit = defineEmits<{
  close: []
  save: [templates: TicketReplyTemplate[]]
}>()

const { t } = useI18n()
const localTemplates = ref<TicketReplyTemplate[]>([])
const selectedTemplateIds = ref<string[]>([])
const selectedIds = computed(() => new Set(selectedTemplateIds.value))

watch(() => props.show, (show) => {
  if (show) {
    localTemplates.value = props.templates.map((template) => ({ ...template }))
    selectedTemplateIds.value = []
  }
}, { immediate: true })

function createLocalTemplate(): TicketReplyTemplate {
  return {
    id: `local-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    title: '',
    content: '',
  }
}

function addTemplate() {
  localTemplates.value = [...localTemplates.value, createLocalTemplate()]
}

function removeTemplate(index: number) {
  const target = localTemplates.value[index]
  localTemplates.value = localTemplates.value.filter((_, idx) => idx !== index)
  if (target) {
    selectedTemplateIds.value = selectedTemplateIds.value.filter((id) => id !== target.id)
  }
}

function removeSelected() {
  const selected = selectedIds.value
  localTemplates.value = localTemplates.value.filter((template) => !selected.has(template.id))
  selectedTemplateIds.value = []
}

function submit() {
  emit('save', localTemplates.value.map((template) => ({
    id: template.id.startsWith('local-') ? '' : template.id,
    title: template.title,
    content: template.content,
  })))
}
</script>
