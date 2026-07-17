<template>
  <div class="space-y-4">
    <div
      v-for="field in schema"
      :key="field.key"
      class="rounded-card border border-line bg-card p-4 dark:border-dark-700 dark:bg-dark-900"
    >
      <div class="mb-3">
        <div class="flex items-center gap-2">
          <h3 class="text-sm font-semibold text-ink dark:text-white">{{ field.label }}</h3>
          <span v-if="field.required" class="rounded-full bg-rose-50 px-2 py-0.5 text-[11px] font-medium text-rose-700 dark:bg-rose-900/20 dark:text-rose-200">
            {{ t('skills.editor.required', '必填') }}
          </span>
          <span class="rounded-full bg-line px-2 py-0.5 text-[11px] text-ink-body dark:bg-dark-800 dark:text-dark-300">
            {{ field.type }}
          </span>
        </div>
        <p v-if="field.description" class="mt-1 text-xs text-ink-soft dark:text-dark-400">{{ field.description }}</p>
      </div>

      <Input
        v-if="field.type === 'string' || field.type === 'number' || field.type === 'image' || field.type === 'file'"
        :model-value="toInputValue(resolveValue(field))"
        :type="field.type === 'number' ? 'number' : 'text'"
        :placeholder="field.placeholder || ''"
        :readonly="readonly"
        @update:model-value="(value) => updateField(field.key, field.type === 'number' ? toNumberOrString(value) : value)"
      />

      <TextArea
        v-else-if="field.type === 'text' || field.type === 'json'"
        :model-value="toTextAreaValue(resolveValue(field))"
        :rows="field.type === 'json' ? 6 : 4"
        :placeholder="field.placeholder || ''"
        :readonly="readonly"
        @update:model-value="(value) => updateField(field.key, value)"
      />

      <Select
        v-else-if="field.type === 'select'"
        :model-value="toInputValue(resolveValue(field))"
        :disabled="readonly"
        :options="selectOptions(field)"
        @update:model-value="(value) => updateField(field.key, typeof value === 'string' ? value : String(value ?? ''))"
      />

      <label
        v-else-if="field.type === 'boolean'"
        class="inline-flex items-center gap-3 rounded-xl border border-line px-4 py-3 text-sm text-ink-body dark:border-dark-700 dark:text-dark-200"
      >
        <input
          :checked="Boolean(resolveValue(field))"
          :disabled="readonly"
          type="checkbox"
          class="h-4 w-4 rounded border-line text-brand-600 focus:ring-accent/25"
          @change="updateField(field.key, ($event.target as HTMLInputElement).checked)"
        />
        {{ t('skills.form.booleanToggle', '启用该变量') }}
      </label>
    </div>

    <div
      v-if="schema.length === 0"
      class="rounded-card border border-dashed border-line bg-page px-4 py-8 text-center text-sm text-ink-soft dark:border-dark-700 dark:bg-dark-900"
    >
      {{ t('skills.form.noVariables', '这个技能没有暴露变量，安装后可直接运行。') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Input from '@/components/common/Input.vue'
import Select from '@/components/common/Select.vue'
import TextArea from '@/components/common/TextArea.vue'
import type { SkillVariableSchemaItem } from '@/types/skills'

interface Props {
  schema: SkillVariableSchemaItem[]
  modelValue: Record<string, unknown>
  readonly?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  readonly: false
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: Record<string, unknown>): void
}>()

const { t } = useI18n()

function resolveValue(field: SkillVariableSchemaItem): unknown {
  if (field.key in props.modelValue) {
    return props.modelValue[field.key]
  }
  return field.default_value ?? (field.type === 'boolean' ? false : '')
}

function updateField(key: string, value: unknown): void {
  emit('update:modelValue', {
    ...props.modelValue,
    [key]: value
  })
}

function selectOptions(field: SkillVariableSchemaItem) {
  return (field.options ?? []).map((option) => ({
    value: option.value,
    label: option.label
  }))
}

function toInputValue(value: unknown): string | number {
  if (typeof value === 'number') return value
  if (typeof value === 'string') return value
  if (typeof value === 'boolean') return value ? 'true' : 'false'
  return ''
}

function toTextAreaValue(value: unknown): string {
  if (typeof value === 'string') return value
  if (typeof value === 'object' && value !== null) {
    return JSON.stringify(value, null, 2)
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value)
  }
  return ''
}

function toNumberOrString(value: string): number | string {
  if (!value.trim()) return ''
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : value
}
</script>
