<template>
  <section class="rounded-3xl border border-line bg-card p-5 shadow-xs dark:border-dark-700 dark:bg-dark-900">
    <div class="mb-5">
      <h2 class="text-lg font-semibold text-ink dark:text-white">{{ t('skills.editor.sourceConfig', '源内容配置') }}</h2>
      <p class="mt-1 text-sm text-ink-soft dark:text-dark-400">{{ typeHint }}</p>
    </div>

    <div v-if="modelValue.type === 'prompt_chat'" class="space-y-4">
      <TextArea
        :model-value="modelValue.system_prompt"
        :rows="6"
        :label="t('skills.editor.systemPrompt', 'System Prompt')"
        :placeholder="t('skills.editor.systemPromptPlaceholder', '定义这个技能的角色、约束、输出要求。')"
        @update:model-value="(value) => patch({ system_prompt: value })"
      />
      <TextArea
        :model-value="modelValue.user_prompt_template"
        :rows="8"
        :label="t('skills.editor.userPromptTemplate', 'User Prompt Template')"
        :placeholder="t('skills.editor.userPromptTemplatePlaceholder', '支持用 {{variable_name}} 引用变量。')"
        @update:model-value="(value) => patch({ user_prompt_template: value })"
      />
      <TextArea
        :model-value="modelValue.assistant_prefill || ''"
        :rows="4"
        :label="t('skills.editor.assistantPrefill', 'Assistant Prefill')"
        :placeholder="t('skills.editor.assistantPrefillPlaceholder', '可选，预填助手回复的开头。')"
        @update:model-value="(value) => patch({ assistant_prefill: value })"
      />
      <div class="grid gap-4 lg:grid-cols-3">
        <Input
          :model-value="modelValue.model || ''"
          :label="t('skills.editor.model', '模型')"
          :placeholder="t('skills.editor.modelPlaceholder', '例如 gpt-4.1-mini')"
          @update:model-value="(value) => patch({ model: value })"
        />
        <Input
          :model-value="toInputValue(modelValue.temperature)"
          type="number"
          :label="t('skills.editor.temperature', 'Temperature')"
          @update:model-value="(value) => patch({ temperature: toNullableNumber(value) })"
        />
        <Input
          :model-value="toInputValue(modelValue.max_tokens)"
          type="number"
          :label="t('skills.editor.maxTokens', 'Max Tokens')"
          @update:model-value="(value) => patch({ max_tokens: toNullableNumber(value) })"
        />
      </div>
    </div>

    <div v-else-if="modelValue.type === 'prompt_image'" class="space-y-4">
      <TextArea
        :model-value="modelValue.prompt_template"
        :rows="8"
        :label="t('skills.editor.imagePromptTemplate', 'Image Prompt Template')"
        :placeholder="t('skills.editor.imagePromptTemplatePlaceholder', '描述主体、风格、镜头、材质与光线。')"
        @update:model-value="(value) => patch({ prompt_template: value })"
      />
      <TextArea
        :model-value="modelValue.negative_prompt_template || ''"
        :rows="5"
        :label="t('skills.editor.negativePromptTemplate', 'Negative Prompt Template')"
        :placeholder="t('skills.editor.negativePromptTemplatePlaceholder', '可选，不希望出现的元素。')"
        @update:model-value="(value) => patch({ negative_prompt_template: value })"
      />
      <div class="grid gap-4 lg:grid-cols-4">
        <Input
          :model-value="modelValue.style || ''"
          :label="t('skills.editor.style', '风格')"
          :placeholder="t('skills.editor.stylePlaceholder', '例如 cinematic')"
          @update:model-value="(value) => patch({ style: value })"
        />
        <Input
          :model-value="modelValue.size || ''"
          :label="t('skills.editor.size', '尺寸')"
          :placeholder="t('skills.editor.sizePlaceholder', '例如 1024x1024')"
          @update:model-value="(value) => patch({ size: value })"
        />
        <Input
          :model-value="modelValue.quality || ''"
          :label="t('skills.editor.quality', '质量')"
          :placeholder="t('skills.editor.qualityPlaceholder', '例如 high')"
          @update:model-value="(value) => patch({ quality: value })"
        />
        <Input
          :model-value="toInputValue(modelValue.image_count)"
          type="number"
          :label="t('skills.editor.imageCount', '输出数量')"
          @update:model-value="(value) => patch({ image_count: toNullableNumber(value) })"
        />
      </div>
    </div>

    <div v-else class="space-y-4">
      <div class="grid gap-4 lg:grid-cols-4">
        <Input
          :model-value="modelValue.language"
          :label="t('skills.editor.language', '语言')"
          :placeholder="t('skills.editor.languagePlaceholder', '例如 javascript')"
          @update:model-value="(value) => patch({ language: value })"
        />
        <Input
          :model-value="modelValue.runtime || ''"
          :label="t('skills.editor.runtime', '运行时')"
          :placeholder="t('skills.editor.runtimePlaceholder', '例如 node20')"
          @update:model-value="(value) => patch({ runtime: value })"
        />
        <Input
          :model-value="modelValue.entrypoint || ''"
          :label="t('skills.editor.entrypoint', '入口函数')"
          :placeholder="t('skills.editor.entrypointPlaceholder', '例如 main.mjs')"
          @update:model-value="(value) => patch({ entrypoint: value })"
        />
        <Input
          :model-value="toInputValue(modelValue.timeout_seconds)"
          type="number"
          :label="t('skills.editor.timeoutSeconds', '超时秒数')"
          @update:model-value="(value) => patch({ timeout_seconds: toNullableNumber(value) })"
        />
      </div>
      <Input
        :model-value="dependenciesText"
        :label="t('skills.editor.dependencies', '依赖')"
        :placeholder="t('skills.editor.dependenciesPlaceholder', '用逗号分隔，例如 axios,lodash')"
        @update:model-value="updateDependencies"
      />
      <TextArea
        :model-value="modelValue.source_code"
        :rows="18"
        :label="t('skills.editor.sourceCode', '源码')"
        :placeholder="t('skills.editor.sourceCodePlaceholder', '在这里输入脚本源码。')"
        @update:model-value="(value) => patch({ source_code: value })"
      />
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Input from '@/components/common/Input.vue'
import TextArea from '@/components/common/TextArea.vue'
import type { SkillContent, SkillPromptChatContent, SkillPromptImageContent, SkillScriptContent } from '@/types/skills'

interface Props {
  modelValue: SkillContent
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: SkillContent): void
}>()

const { t } = useI18n()

const typeHint = computed(() => {
  switch (props.modelValue.type) {
    case 'prompt_image':
      return t('skills.editor.promptImageHint', '定义生图主提示词、反向提示词和默认图片参数。')
    case 'script':
      return t('skills.editor.scriptHint', '脚本技能可配置运行时、入口函数、依赖和源码。')
    case 'prompt_chat':
    default:
      return t('skills.editor.promptChatHint', '对话技能通常由 system prompt 和用户模板组成。')
  }
})

const dependenciesText = computed(() =>
  props.modelValue.type === 'script' ? props.modelValue.dependencies.join(', ') : ''
)

function patch(
  updates: Partial<SkillPromptChatContent> | Partial<SkillPromptImageContent> | Partial<SkillScriptContent>
): void {
  emit('update:modelValue', {
    ...props.modelValue,
    ...updates
  } as SkillContent)
}

function updateDependencies(value: string): void {
  if (props.modelValue.type !== 'script') return
  patch({
    dependencies: value
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean)
  })
}

function toNullableNumber(value: string): number | null {
  if (!value.trim()) return null
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : null
}

function toInputValue(value: number | null | undefined): string | number {
  return typeof value === 'number' ? value : ''
}
</script>
