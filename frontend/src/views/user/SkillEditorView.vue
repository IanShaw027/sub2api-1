<template>
  <AppLayout>
    <div v-if="showEditor" class="mx-auto flex w-full max-w-7xl flex-col gap-6">
      <SkillCenterNav
        active="editor"
        :skill-id="skillsStore.editorSkillId"
        :can-edit-skill="Boolean(skillsStore.detail?.editable)"
        :can-view-runs="Boolean(skillsStore.detail?.owned)"
        :can-view-revenue="Boolean(skillsStore.detail?.owned)"
      />

      <section class="card overflow-hidden">
        <div class="border-b border-line bg-gradient-to-r from-dark-900 via-brand-950 to-accent-950 px-6 py-5 text-white dark:border-dark-700">
          <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <p class="text-xs uppercase tracking-[0.35em] text-white/60">{{ t('skills.center.label', 'Skill Center') }}</p>
              <h1 class="mt-2 text-2xl font-bold">{{ isEditing ? t('skills.editor.edit', '编辑技能') : t('skills.editor.create', '创建技能') }}</h1>
              <p class="mt-1 text-sm text-white/70">{{ t('skills.editor.subtitle', '支持 prompt_chat / prompt_image，并可编辑变量 schema。') }}</p>
            </div>
            <div class="flex flex-wrap gap-3">
              <button class="btn btn-secondary bg-white/10 text-white hover:bg-white/15" :disabled="skillsStore.loadingEditor" @click="reloadEditor">
                <Icon name="refresh" size="sm" class="mr-2" :class="skillsStore.loadingEditor ? 'animate-spin' : ''" />
                {{ t('common.refresh', '刷新') }}
              </button>
              <button class="btn btn-primary" :disabled="skillsStore.savingEditor" @click="saveSkillItem">
                {{ skillsStore.savingEditor ? t('common.processing', '处理中') : t('common.save', '保存') }}
              </button>
            </div>
          </div>
        </div>

        <div class="grid gap-6 p-6 xl:grid-cols-[minmax(0,1fr)_380px]">
          <div class="space-y-6">
            <section class="rounded-3xl border border-line bg-card p-5 shadow-xs dark:border-dark-700 dark:bg-dark-900">
              <div class="grid gap-4 lg:grid-cols-2">
                <Input
                  v-model="skillsStore.editorDraft.name"
                  :label="t('common.name', '名称')"
                  :placeholder="t('skills.editor.namePlaceholder', '例如 电商商品描述增强器')"
                />
                <Input
                  v-model="skillsStore.editorDraft.slug"
                  :label="t('skills.editor.slug', 'Slug')"
                  :placeholder="t('skills.editor.slugPlaceholder', '例如 ecommerce-copywriter')"
                />
                <div class="lg:col-span-2">
                  <Input
                    v-model="skillsStore.editorDraft.tagline"
                    :label="t('skills.editor.tagline', '一句话简介')"
                    :placeholder="t('skills.editor.taglinePlaceholder', '快速说明技能解决什么问题。')"
                  />
                </div>
                <div class="lg:col-span-2">
                  <TextArea
                    v-model="skillsStore.editorDraft.description"
                    :rows="5"
                    :label="t('skills.editor.description', '说明')"
                    :placeholder="t('skills.editor.descriptionPlaceholder', '描述适用场景、输入输出、注意事项。')"
                  />
                </div>
                <div>
                  <label class="input-label mb-1.5 block">{{ t('skills.editor.type', '类型') }}</label>
                  <Select
                    :model-value="skillsStore.editorDraft.type"
                    :options="typeOptions"
                    @update:model-value="(value) => updateType(value)"
                  />
                </div>
                <div>
                  <label class="input-label mb-1.5 block">{{ t('skills.my.visibility', '可见性') }}</label>
                  <Select
                    :model-value="skillsStore.editorDraft.visibility"
                    :options="visibilityOptions"
                    @update:model-value="(value) => (skillsStore.editorDraft.visibility = value === 'public' ? 'public' : 'private')"
                  />
                </div>
                <div>
                  <label class="input-label mb-1.5 block">{{ t('common.status', '状态') }}</label>
                  <Select
                    :model-value="skillsStore.editorDraft.status"
                    :options="statusOptions"
                    @update:model-value="(value) => (skillsStore.editorDraft.status = normalizeStatus(value))"
                  />
                </div>
                <Input
                  v-model="skillsStore.editorDraft.category"
                  :label="t('skills.market.category', '分类')"
                  :placeholder="t('skills.editor.categoryPlaceholder', '例如 营销 / 绘图 / 自动化')"
                />
                <div class="lg:col-span-2">
                  <Input
                    :model-value="tagsInput"
                    :label="t('skills.editor.tags', '标签')"
                    :placeholder="t('skills.editor.tagsPlaceholder', '逗号分隔，例如 marketing,seo,copy')"
                    @update:model-value="updateTags"
                  />
                </div>
              </div>
            </section>

            <SkillTypeEditor
              :model-value="skillsStore.editorDraft.content"
              @update:model-value="(value) => (skillsStore.editorDraft.content = value)"
            />

            <SkillVariableSchemaEditor
              :model-value="skillsStore.editorDraft.variable_schema"
              @update:model-value="(value) => (skillsStore.editorDraft.variable_schema = value)"
            />
          </div>

          <aside class="space-y-6">
            <section class="rounded-3xl border border-line bg-card p-5 shadow-xs dark:border-dark-700 dark:bg-dark-900">
              <h2 class="text-lg font-semibold text-ink dark:text-white">{{ t('skills.editor.pricing', '定价与可见性') }}</h2>
              <div class="mt-4 space-y-4">
                <div>
                  <label class="input-label mb-1.5 block">{{ t('skills.market.priceMode', '收费方式') }}</label>
                  <Select
                    :model-value="skillsStore.editorDraft.pricing.mode"
                    :options="priceModeOptions"
                    @update:model-value="updatePriceMode"
                  />
                </div>
                <div class="grid gap-4 lg:grid-cols-2">
                  <Input
                    :model-value="skillsStore.editorDraft.pricing.amount"
                    type="number"
                    :label="t('skills.editor.priceAmount', '价格')"
                    @update:model-value="(value) => (skillsStore.editorDraft.pricing.amount = Number(value || 0))"
                  />
                  <Input
                    v-model="skillsStore.editorDraft.pricing.currency"
                    :label="t('skills.editor.currency', '币种')"
                    :placeholder="'CNY'"
                  />
                </div>
                <label class="flex items-start gap-3 rounded-card border border-line bg-page px-4 py-4 text-sm text-ink-body dark:border-dark-700 dark:bg-dark-950 dark:text-dark-200">
                  <input
                    v-model="skillsStore.editorDraft.source_locked"
                    type="checkbox"
                    class="mt-0.5 h-4 w-4 rounded border-line text-brand-600 focus:ring-accent/25"
                  />
                  <span>
                    <span class="block font-medium">{{ t('skills.editor.sourceLocked', '隐藏源内容') }}</span>
                    <span class="mt-1 block text-xs text-ink-soft dark:text-dark-400">{{ t('skills.editor.sourceLockedHint', '付费技能切换到 paid 时会默认开启；详情页仅展示变量表单和元信息。') }}</span>
                  </span>
                </label>
                <Input
                  v-model="skillsStore.editorDraft.cover_image_url"
                  :label="t('skills.editor.coverImage', '封面图 URL')"
                  :placeholder="t('skills.editor.coverImagePlaceholder', '可选，用于市场卡片与详情头图')"
                />
              </div>
            </section>

            <section class="rounded-3xl border border-line bg-card p-5 shadow-xs dark:border-dark-700 dark:bg-dark-900">
              <h2 class="text-lg font-semibold text-ink dark:text-white">{{ t('skills.editor.extra', '附加说明') }}</h2>
              <div class="mt-4 space-y-4">
                <TextArea
                  v-model="skillsStore.editorDraft.readme"
                  :rows="7"
                  :label="t('skills.detail.readme', '使用说明')"
                  :placeholder="t('skills.editor.readmePlaceholder', '告诉用户如何使用、适合什么输入。')"
                />
                <TextArea
                  v-model="skillsStore.editorDraft.install_note"
                  :rows="5"
                  :label="t('skills.detail.installNote', '安装说明')"
                  :placeholder="t('skills.editor.installNotePlaceholder', '安装后给用户的额外提示。')"
                />
              </div>
            </section>
          </aside>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, watch, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Input from '@/components/common/Input.vue'
import Select from '@/components/common/Select.vue'
import TextArea from '@/components/common/TextArea.vue'
import SkillCenterNav from '@/components/skills/SkillCenterNav.vue'
import SkillTypeEditor from '@/components/skills/SkillTypeEditor.vue'
import SkillVariableSchemaEditor from '@/components/skills/SkillVariableSchemaEditor.vue'
import { skillPaths } from '@/components/skills/paths'
import { useSkillsCenterStore } from '@/stores/skillsCenter'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { SkillPriceMode, SkillStatus, SkillType } from '@/types/skills'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const appStore = useAppStore()
const skillsStore = useSkillsCenterStore()

const skillId = computed(() => {
  const value = Number(route.params.id)
  return Number.isFinite(value) && value > 0 ? value : null
})
const isEditing = computed(() => skillId.value !== null)
const loadedSkillId = ref<number | null>(null)
const showEditor = computed(() => !isEditing.value || loadedSkillId.value === skillId.value)
const tagsInput = computed(() => skillsStore.editorDraft.tags.join(', '))

const typeOptions = [
  { value: 'prompt_chat', label: 'prompt_chat' },
  { value: 'prompt_image', label: 'prompt_image' }
]

const visibilityOptions = [
  { value: 'private', label: 'private' },
  { value: 'public', label: 'public' }
]

const statusOptions = [
  { value: 'draft', label: 'draft' },
  { value: 'published', label: 'published' },
  { value: 'archived', label: 'archived' },
  { value: 'hidden', label: 'hidden' }
]

const priceModeOptions = [
  { value: 'free', label: 'free' },
  { value: 'paid', label: 'paid' }
]

function normalizeStatus(value: string | number | boolean | null): SkillStatus {
  return value === 'published' || value === 'archived' || value === 'hidden' ? value : 'draft'
}

function updateTags(value: string): void {
  skillsStore.setEditorTags(
    value
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean)
  )
}

function updateType(value: string | number | boolean | null): void {
  const type: SkillType = value === 'prompt_image' ? value : 'prompt_chat'
  skillsStore.setEditorType(type)
}

function updatePriceMode(value: string | number | boolean | null): void {
  const mode: SkillPriceMode = value === 'paid' ? 'paid' : 'free'
  skillsStore.editorDraft.pricing.mode = mode
  if (mode === 'paid') {
    skillsStore.editorDraft.source_locked = true
  }
}

async function loadDraft(force = false): Promise<void> {
  loadedSkillId.value = null
  if (!isEditing.value) {
    await skillsStore.loadEditor(null, force)
    return
  }

  try {
    const currentSkillId = skillId.value
    if (currentSkillId === null) {
      return
    }
    await skillsStore.loadEditor(currentSkillId, force)
    if (!skillsStore.detail?.editable) {
      await router.replace(skillPaths.detail(currentSkillId))
      return
    }
    loadedSkillId.value = currentSkillId
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

async function reloadEditor(): Promise<void> {
  await loadDraft(true)
}

async function saveSkillItem(): Promise<void> {
  if (!skillsStore.editorDraft.name.trim()) {
    appStore.showError(t('skills.editor.nameRequired', '请先填写技能名称'))
    return
  }
  if (!skillsStore.editorDraft.slug.trim()) {
    appStore.showError(t('skills.editor.slugRequired', '请先填写技能 slug'))
    return
  }

  try {
    await skillsStore.saveEditor()
    appStore.showSuccess(t('common.saved', '保存成功'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

watch(skillId, () => {
  void loadDraft()
})

onMounted(() => {
  void loadDraft()
})
</script>
