<template>
  <section class="rounded-3xl border border-line bg-card p-5 shadow-xs dark:border-dark-700 dark:bg-dark-900">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h2 class="text-lg font-semibold text-ink dark:text-white">{{ t('skills.editor.variableSchema', '变量 Schema') }}</h2>
        <p class="mt-1 text-sm text-ink-soft dark:text-dark-400">{{ t('skills.editor.variableSchemaHint', '收费技能默认只暴露这里定义的变量，不直接暴露源内容。') }}</p>
      </div>
      <button class="btn btn-primary btn-sm" type="button" @click="addField">
        <Icon name="plus" size="sm" class="mr-1" />
        {{ t('skills.editor.addVariable', '新增变量') }}
      </button>
    </div>

    <div class="mt-5 space-y-4">
      <article
        v-for="(field, index) in modelValue"
        :key="`${field.key}-${index}`"
        class="rounded-card border border-line bg-page p-4 dark:border-dark-700 dark:bg-dark-950"
      >
        <div class="mb-4 flex flex-wrap items-center justify-between gap-2">
          <div class="flex items-center gap-2">
            <span class="rounded-full bg-ink dark:bg-dark-700 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.2em] text-white">{{ index + 1 }}</span>
            <span class="text-sm font-medium text-ink dark:text-white">{{ field.label || field.key || t('skills.editor.untitledVariable', '未命名变量') }}</span>
          </div>
          <div class="flex flex-wrap gap-2">
            <button class="btn btn-secondary btn-xs" type="button" :disabled="index === 0" @click="moveField(index, -1)">
              <Icon name="arrowUp" size="sm" />
            </button>
            <button class="btn btn-secondary btn-xs" type="button" :disabled="index === modelValue.length - 1" @click="moveField(index, 1)">
              <Icon name="arrowDown" size="sm" />
            </button>
            <button class="btn btn-danger btn-xs" type="button" @click="removeField(index)">
              <Icon name="trash" size="sm" />
            </button>
          </div>
        </div>

        <div class="grid gap-4 lg:grid-cols-2">
          <Input
            :model-value="field.key"
            :label="t('skills.editor.variableKey', '变量 Key')"
            :placeholder="t('skills.editor.variableKeyPlaceholder', '例如 product_name')"
            @update:model-value="(value) => updateField(index, { key: sanitizeKey(value) })"
          />
          <Input
            :model-value="field.label"
            :label="t('common.name', '名称')"
            :placeholder="t('skills.editor.variableLabelPlaceholder', '例如 产品名称')"
            @update:model-value="(value) => updateField(index, { label: value })"
          />
          <div>
            <label class="input-label mb-1.5 block">{{ t('skills.editor.variableType', '变量类型') }}</label>
            <Select
              :model-value="field.type"
              :options="typeOptions"
              @update:model-value="(value) => updateField(index, { type: String(value) as FieldType })"
            />
          </div>
          <div>
            <label class="input-label mb-1.5 block">{{ t('skills.editor.requiredFlag', '是否必填') }}</label>
            <label class="flex h-[44px] items-center gap-3 rounded-xl border border-line bg-card px-4 text-sm text-ink-body dark:border-dark-700 dark:bg-dark-900 dark:text-dark-200">
              <input
                :checked="field.required"
                type="checkbox"
                class="h-4 w-4 rounded border-line text-brand-600 focus:ring-accent/25"
                @change="updateField(index, { required: ($event.target as HTMLInputElement).checked })"
              />
              {{ t('skills.editor.requiredFlagHint', '运行时必须提供这个变量') }}
            </label>
          </div>
        </div>

        <div class="mt-4 grid gap-4 lg:grid-cols-2">
          <Input
            :model-value="field.placeholder || ''"
            :label="t('skills.editor.placeholder', '占位提示')"
            :placeholder="t('skills.editor.placeholderValue', '给用户看的输入提示')"
            @update:model-value="(value) => updateField(index, { placeholder: value })"
          />
          <Input
            v-if="field.type !== 'boolean'"
            :model-value="defaultValueText(field.default_value)"
            :label="t('skills.editor.defaultValue', '默认值')"
            :type="field.type === 'number' ? 'number' : 'text'"
            @update:model-value="(value) => updateField(index, { default_value: castDefaultValue(field.type, value) })"
          />
          <label
            v-else
            class="flex h-[72px] flex-col justify-center rounded-card border border-line bg-card px-4 dark:border-dark-700 dark:bg-dark-900"
          >
            <span class="input-label mb-2 block">{{ t('skills.editor.defaultValue', '默认值') }}</span>
            <span class="inline-flex items-center gap-3 text-sm text-ink-body dark:text-dark-200">
              <input
                :checked="Boolean(field.default_value)"
                type="checkbox"
                class="h-4 w-4 rounded border-line text-brand-600 focus:ring-accent/25"
                @change="updateField(index, { default_value: ($event.target as HTMLInputElement).checked })"
              />
              {{ t('skills.form.booleanToggle', '启用该变量') }}
            </span>
          </label>
        </div>

        <div class="mt-4">
          <TextArea
            :model-value="field.description || ''"
            :rows="3"
            :label="t('skills.editor.description', '说明')"
            :placeholder="t('skills.editor.descriptionPlaceholder', '解释变量的业务含义、格式要求或上下文。')"
            @update:model-value="(value) => updateField(index, { description: value })"
          />
        </div>

        <div v-if="field.type === 'select'" class="mt-4 rounded-card border border-dashed border-line bg-card p-4 dark:border-dark-700 dark:bg-dark-900">
          <div class="mb-3 flex items-center justify-between gap-3">
            <div>
              <h3 class="text-sm font-semibold text-ink dark:text-white">{{ t('skills.editor.selectOptions', '下拉选项') }}</h3>
              <p class="mt-1 text-xs text-ink-soft dark:text-dark-400">{{ t('skills.editor.selectOptionsHint', '定义显示给用户的选项标签和值。') }}</p>
            </div>
            <button class="btn btn-secondary btn-xs" type="button" @click="addOption(index)">
              <Icon name="plus" size="sm" class="mr-1" />
              {{ t('skills.editor.addOption', '新增选项') }}
            </button>
          </div>

          <div class="space-y-3">
            <div
              v-for="(option, optionIndex) in field.options || []"
              :key="`${field.key}-option-${optionIndex}`"
              class="grid gap-3 rounded-xl border border-line p-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] dark:border-dark-700"
            >
              <Input
                :model-value="option.label"
                :label="t('common.name', '名称')"
                @update:model-value="(value) => updateOption(index, optionIndex, { label: value })"
              />
              <Input
                :model-value="option.value"
                :label="t('skills.editor.optionValue', '值')"
                @update:model-value="(value) => updateOption(index, optionIndex, { value })"
              />
              <div class="flex items-end">
                <button class="btn btn-danger btn-sm w-full" type="button" @click="removeOption(index, optionIndex)">
                  <Icon name="trash" size="sm" class="mr-1" />
                  {{ t('common.delete', '删除') }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </article>

      <div
        v-if="modelValue.length === 0"
        class="rounded-card border border-dashed border-line bg-page px-4 py-10 text-center text-sm text-ink-soft dark:border-dark-700 dark:bg-dark-950"
      >
        {{ t('skills.editor.noVariables', '还没有变量，适合直接包装成固定技能的场景。') }}
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Input from '@/components/common/Input.vue'
import Select from '@/components/common/Select.vue'
import TextArea from '@/components/common/TextArea.vue'
import Icon from '@/components/icons/Icon.vue'
import type { SkillVariableOption, SkillVariableSchemaItem, SkillVariableType } from '@/types/skills'

type FieldType = SkillVariableType

interface Props {
  modelValue: SkillVariableSchemaItem[]
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: SkillVariableSchemaItem[]): void
}>()

const { t } = useI18n()

const typeOptions: Array<{ value: SkillVariableType; label: string }> = [
  { value: 'string', label: 'string' },
  { value: 'text', label: 'text' },
  { value: 'number', label: 'number' },
  { value: 'boolean', label: 'boolean' },
  { value: 'select', label: 'select' },
  { value: 'json', label: 'json' },
  { value: 'image', label: 'image' },
  { value: 'file', label: 'file' }
]

function cloneSchema(): SkillVariableSchemaItem[] {
  return props.modelValue.map((field) => ({
    ...field,
    options: field.options ? field.options.map((option) => ({ ...option })) : []
  }))
}

function commit(next: SkillVariableSchemaItem[]): void {
  emit('update:modelValue', next)
}

function sanitizeKey(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_]+/g, '_')
    .replace(/^_+|_+$/g, '')
}

function addField(): void {
  const next = cloneSchema()
  next.push({
    key: `variable_${next.length + 1}`,
    label: '',
    type: 'string',
    required: false,
    description: '',
    placeholder: '',
    default_value: '',
    options: []
  })
  commit(next)
}

function updateField(index: number, patch: Partial<SkillVariableSchemaItem>): void {
  const next = cloneSchema()
  const current = next[index]
  if (!current) return
  next[index] = {
    ...current,
    ...patch
  }
  if (patch.type !== 'select') {
    next[index].options = []
  }
  commit(next)
}

function removeField(index: number): void {
  const next = cloneSchema()
  next.splice(index, 1)
  commit(next)
}

function moveField(index: number, direction: -1 | 1): void {
  const next = cloneSchema()
  const target = index + direction
  if (target < 0 || target >= next.length) return
  const [current] = next.splice(index, 1)
  next.splice(target, 0, current)
  commit(next)
}

function addOption(fieldIndex: number): void {
  const next = cloneSchema()
  const field = next[fieldIndex]
  if (!field) return
  field.options = [...(field.options ?? []), { label: '', value: '' }]
  commit(next)
}

function updateOption(fieldIndex: number, optionIndex: number, patch: Partial<SkillVariableOption>): void {
  const next = cloneSchema()
  const field = next[fieldIndex]
  const option = field?.options?.[optionIndex]
  if (!field || !option) return
  field.options = field.options?.map((item, index) => (index === optionIndex ? { ...item, ...patch } : item))
  commit(next)
}

function removeOption(fieldIndex: number, optionIndex: number): void {
  const next = cloneSchema()
  const field = next[fieldIndex]
  if (!field?.options) return
  field.options.splice(optionIndex, 1)
  commit(next)
}

function defaultValueText(value: SkillVariableSchemaItem['default_value']): string | number {
  if (typeof value === 'string' || typeof value === 'number') return value
  if (typeof value === 'boolean') return value ? 'true' : 'false'
  return ''
}

function castDefaultValue(type: SkillVariableType, value: string): string | number | boolean {
  if (type === 'number') {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : 0
  }
  return value
}
</script>
