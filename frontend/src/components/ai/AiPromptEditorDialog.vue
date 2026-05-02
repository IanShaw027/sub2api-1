<template>
  <BaseDialog :show="show" :title="title" width="wide" @close="emit('close')">
    <form class="space-y-4" @submit.prevent="handleSubmit">
      <div class="grid gap-4 md:grid-cols-2">
        <Input v-model="form.title" :label="t('ai.prompt.title', '标题')" required />
        <div>
          <label class="input-label mb-1.5 block">{{ t('ai.prompt.visibility', '可见性') }}</label>
          <Select
            :model-value="form.visibility"
            :options="visibilityOptions"
            @update:model-value="updateVisibility"
          />
        </div>
      </div>

      <div class="grid gap-4 md:grid-cols-2">
        <div>
          <label class="input-label mb-1.5 block">{{ t('ai.prompt.status', '状态') }}</label>
          <Select
            :model-value="form.status"
            :options="statusOptions"
            @update:model-value="updateStatus"
          />
        </div>
        <div>
          <label class="input-label mb-1.5 block">{{ t('ai.prompt.line', '线路') }}</label>
          <Select
            :model-value="form.line_id"
            :options="lineOptions"
            @update:model-value="(value) => (form.line_id = typeof value === 'number' ? value : null)"
          />
        </div>
      </div>

      <Input v-model="tagsText" :label="t('ai.prompt.tags', '标签')" :placeholder="t('ai.prompt.tagsPlaceholder', '用逗号分隔')" />

      <TextArea
        v-model="form.description"
        :label="t('ai.prompt.description', '简介')"
        :placeholder="t('ai.prompt.descriptionPlaceholder', '可选说明')"
      />

      <TextArea
        v-model="form.content"
        :label="t('ai.prompt.content', '提示词')"
        :placeholder="t('ai.prompt.contentPlaceholder', '输入提示词内容')"
        :rows="10"
        required
      />

      <div class="flex justify-end gap-3 pt-2">
        <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button type="submit" class="btn btn-primary">{{ submitLabel }}</button>
      </div>
    </form>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Input from '@/components/common/Input.vue'
import Select from '@/components/common/Select.vue'
import TextArea from '@/components/common/TextArea.vue'
import type { AiPromptStatus, AiPromptTemplate, AiVisibility } from '@/types'

const { t } = useI18n()

const props = defineProps<{
  show: boolean
  title: string
  submitLabel?: string
  prompt?: AiPromptTemplate | null
  lineOptions: Array<{ value: number | null; label: string }>
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'save', payload: {
    title: string
    content: string
    description?: string
    tags: string[]
    visibility: AiVisibility
    status: AiPromptStatus
    line_id: number | null
  }): void
}>()

const form = reactive({
  title: '',
  content: '',
  description: '',
  visibility: 'private' as AiVisibility,
  status: 'draft' as AiPromptStatus,
  line_id: null as number | null
})

const tagsText = ref('')

watch(
  () => props.prompt,
  (prompt) => {
    form.title = prompt?.title ?? ''
    form.content = prompt?.content ?? ''
    form.description = prompt?.description ?? ''
    form.visibility = prompt?.visibility ?? 'private'
    form.status = prompt?.status ?? 'draft'
    form.line_id = prompt?.line_id ?? null
    tagsText.value = (prompt?.tags ?? []).join(', ')
  },
  { immediate: true }
)

const visibilityOptions = [
  { value: 'private', label: t('ai.prompt.private', '私有') },
  { value: 'public', label: t('ai.prompt.public', '公开') }
]

const statusOptions = [
  { value: 'draft', label: t('ai.prompt.draft', '草稿') },
  { value: 'published', label: t('ai.prompt.published', '已发布') },
  { value: 'archived', label: t('ai.prompt.archived', '已归档') },
  { value: 'hidden', label: t('ai.prompt.hidden', '隐藏') }
]

const submitLabel = computed(() => props.submitLabel ?? t('common.save'))

function handleSubmit() {
  emit('save', {
    title: form.title.trim(),
    content: form.content.trim(),
    description: form.description.trim() || undefined,
    tags: tagsText.value ? tagsText.value.split(',').map((item) => item.trim()).filter(Boolean) : [],
    visibility: form.visibility,
    status: form.status,
    line_id: form.line_id
  })
}

function updateVisibility(value: string | number | boolean | null) {
  form.visibility = value === 'public' ? 'public' : 'private'
}

function updateStatus(value: string | number | boolean | null) {
  const next = String(value ?? '')
  form.status = next === 'published' || next === 'archived' || next === 'hidden' ? next : 'draft'
}
</script>
