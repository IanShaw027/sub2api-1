<template>
 <BaseDialog :show="show" :title="t('tickets.templates.manageTitle')" width="wide" @close="emit('close')">
 <div class="space-y-4">
 <div class="flex flex-wrap items-center justify-between gap-3">
 <p class="text-sm text-muted">{{ t('tickets.templates.manageDescription') }}</p>
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

 <div v-if="localTemplates.length === 0" class="rounded-xl border border-dashed p-6 text-center text-sm text-muted">
 {{ t('tickets.templates.empty') }}
 </div>

 <div v-else class="max-h-[60vh] space-y-4 overflow-y-auto pr-1">
 <div
 v-for="(template, index) in localTemplates"
 :key="template.clientId"
 class="rounded-xl border border-line bg-surface-2/60 p-4"
 >
 <div class="mb-3 flex items-start gap-3">
 <input
 v-model="selectedClientIds"
 class="mt-1 h-4 w-4 rounded border-line text-accent focus:ring-accent"
 type="checkbox"
 :value="template.clientId"
 />
 <div class="grid flex-1 gap-3">
 <div>
 <label class="mb-1 block text-xs font-medium text-muted">{{ t('tickets.templates.title') }}</label>
 <input
 v-model="template.title"
 class="input"
 :placeholder="t('tickets.templates.titlePlaceholder')"
 />
 </div>
 <div>
 <label class="mb-1 block text-xs font-medium text-muted">{{ t('tickets.templates.content') }}</label>
 <textarea
 v-model="template.content"
 class="input min-h-[120px]"
 :placeholder="t('tickets.templates.contentPlaceholder')"
 />
 </div>
 </div>
 <button
 type="button"
 class="rounded-lg px-2 py-1 text-sm text-red-500 transition-colors hover:bg-red-50 hover:text-red-600"
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
import type { TicketReplyTemplate } from '@/types/ticket'

interface EditableTemplate {
 clientId: string
 id: number
 title: string
 content: string
 sort_order: number
}

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
const localTemplates = ref<EditableTemplate[]>([])
const selectedClientIds = ref<string[]>([])
const selectedIds = computed(() => new Set(selectedClientIds.value))

watch(() => props.show, (show) => {
 if (show) {
 localTemplates.value = props.templates.map((template) => ({
 clientId: `saved-${template.id}`,
 id: template.id,
 title: template.title,
 content: template.content,
 sort_order: template.sort_order,
 }))
 selectedClientIds.value = []
 }
}, { immediate: true })

function addTemplate() {
 const clientId = `local-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
 localTemplates.value = [...localTemplates.value, {
 clientId,
 id: 0,
 title: '',
 content: '',
 sort_order: localTemplates.value.length,
 }]
}

function removeTemplate(index: number) {
 const target = localTemplates.value[index]
 localTemplates.value = localTemplates.value.filter((_, idx) => idx !== index)
 if (target) {
 selectedClientIds.value = selectedClientIds.value.filter((id) => id !== target.clientId)
 }
}

function removeSelected() {
 const selected = selectedIds.value
 localTemplates.value = localTemplates.value.filter((template) => !selected.has(template.clientId))
 selectedClientIds.value = []
}

function submit() {
 emit('save', localTemplates.value.map((template, index) => ({
 id: template.id > 0 ? template.id : 0,
 title: template.title,
 content: template.content,
 sort_order: index,
 })))
}
</script>
