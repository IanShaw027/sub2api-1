<template>
  <AppLayout>
    <div class="mx-auto flex w-full max-w-7xl flex-col gap-6">
      <div class="card overflow-hidden">
        <div class="border-b border-line bg-gradient-to-r from-amber-100 via-orange-50 to-rose-100 px-6 py-5 dark:border-dark-700 dark:from-amber-900/20 dark:via-dark-900 dark:to-rose-900/20">
          <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <p class="text-xs uppercase tracking-[0.35em] text-ink-soft dark:text-dark-400">{{ t('ai.center.label', 'AI 创作中心') }}</p>
              <h1 class="mt-2 text-2xl font-bold text-ink dark:text-white">{{ t('ai.image.title', 'AI 生图') }}</h1>
              <p class="mt-1 text-sm text-ink-body dark:text-dark-400">{{ t('ai.image.subtitle', '支持生成和编辑，提交后进入画廊。') }}</p>
            </div>
            <div class="flex gap-3">
              <button class="btn btn-secondary" :disabled="loading" @click="loadRecentArtworks">
                <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
              </button>
              <button class="btn btn-primary" :disabled="!canSubmit || generating" @click="submitArtwork">
                <Icon name="sparkles" size="sm" class="mr-2" />
                {{ generating ? t('common.processing', '处理中') : submitLabel }}
              </button>
            </div>
          </div>
        </div>

        <div class="grid gap-6 p-6 xl:grid-cols-[minmax(0,420px)_minmax(0,1fr)]">
          <div class="space-y-4">
            <div>
              <label class="input-label mb-1.5 block">{{ t('ai.line.selector', '线路') }}</label>
              <Select
                :model-value="aiStore.selectedLineId"
                :options="lineOptions"
                @update:model-value="(value) => aiStore.setSelectedLine(typeof value === 'number' ? value : null)"
              />
              <div class="mt-3">
                <label class="input-label mb-1.5 block">{{ t('ai.line.key', '密钥') }}</label>
                <Select
                  :model-value="aiStore.selectedKeyId"
                  :options="keyOptions"
                  @update:model-value="(value) => aiStore.setSelectedKey(typeof value === 'number' ? value : null)"
                />
              </div>
              <p
                v-if="showRuntimeLineNotice"
                class="mt-2 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-900/40 dark:bg-amber-900/20 dark:text-amber-100"
              >
                当前 runtime 还没有返回可用线路；前端不会再从其他接口补线路。
              </p>
            </div>

            <div>
              <label class="input-label mb-1.5 block">{{ t('ai.image.mode', '模式') }}</label>
              <div class="inline-flex w-full overflow-hidden rounded-xl border border-line bg-card p-1 dark:border-dark-700 dark:bg-dark-900">
                <button
                  type="button"
                  class="flex-1 rounded-lg px-3 py-2 text-sm font-medium transition"
                  :class="mode === 'generate' ? 'bg-brand-500 text-white shadow-xs' : 'text-ink-body hover:text-ink dark:text-dark-300 dark:hover:text-white'"
                  @click="mode = 'generate'"
                >
                  {{ t('ai.image.modeGenerate', '生成') }}
                </button>
                <button
                  type="button"
                  :disabled="!editAvailable"
                  class="flex-1 rounded-lg px-3 py-2 text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-50"
                  :class="mode === 'edit' ? 'bg-brand-500 text-white shadow-xs' : 'text-ink-body hover:text-ink dark:text-dark-300 dark:hover:text-white'"
                  @click="mode = 'edit'"
                >
                  {{ t('ai.image.modeEdit', '编辑') }}
                </button>
              </div>
              <p class="mt-2 text-xs text-ink-soft dark:text-dark-400">{{ modeHint }}</p>
              <p class="mt-1 text-[11px] text-ink-faint dark:text-dark-500">{{ editStatusLabel }}</p>
            </div>

            <Input v-model="form.title" :label="t('ai.image.artworkTitle', '作品标题')" :placeholder="t('ai.image.artworkTitlePlaceholder', '可选，默认取提示词前缀')" />

            <div v-if="mode === 'edit'" class="space-y-4 rounded-card border border-dashed border-line bg-page p-4 dark:border-dark-700 dark:bg-dark-900/60">
              <div>
                <label class="input-label mb-2 block">{{ t('ai.image.sourceImage', '原图') }}</label>
                <ImageUpload
                  v-model="form.source_image"
                  mode="image"
                  :upload-label="t('ai.image.uploadSource', '上传原图')"
                  :remove-label="t('ai.image.removeSource', '移除原图')"
                  :hint="t('ai.image.sourceHint', '编辑模式需要原图，支持上传图片数据 URL。')"
                  :max-size="8 * 1024 * 1024"
                />
              </div>
              <div>
                <label class="input-label mb-2 block">{{ t('ai.image.maskImage', '遮罩（可选）') }}</label>
                <ImageUpload
                  v-model="form.mask_image"
                  mode="image"
                  :upload-label="t('ai.image.uploadMask', '上传遮罩')"
                  :remove-label="t('ai.image.removeMask', '移除遮罩')"
                  :hint="t('ai.image.maskHint', '遮罩可选；不上传时，后端按原图编辑。')"
                  :max-size="8 * 1024 * 1024"
                />
              </div>
            </div>

            <TextArea
              v-model="form.prompt"
              :rows="6"
              :label="t('ai.image.prompt', '正向提示词')"
              :placeholder="t('ai.image.promptPlaceholder', '描述画面、主体、镜头、质感与光线。')"
            />

            <TextArea
              v-model="form.negative_prompt"
              :rows="4"
              :label="t('ai.image.negativePrompt', '反向提示词')"
              :placeholder="t('ai.image.negativePromptPlaceholder', '可选，输入不希望出现的元素。')"
            />

            <div class="grid gap-4 md:grid-cols-2">
              <div>
                <label class="input-label mb-1.5 block">{{ t('ai.image.style', '风格') }}</label>
                <Select
                  :model-value="form.style"
                  :options="styleOptions"
                  @update:model-value="(value) => (form.style = String(value || 'cinematic'))"
                />
              </div>
              <div>
                <label class="input-label mb-1.5 block">{{ t('ai.image.size', '尺寸') }}</label>
                <Select
                  :model-value="form.size"
                  :options="sizeOptions"
                  @update:model-value="(value) => (form.size = String(value || '1024x1024'))"
                />
              </div>
            </div>

            <div>
              <label class="input-label mb-1.5 block">{{ t('ai.image.visibility', '可见性') }}</label>
              <Select
                :model-value="form.visibility"
                :options="visibilityOptions"
                @update:model-value="(value) => (form.visibility = value === 'private' ? 'private' : 'public')"
              />
            </div>
          </div>

          <div class="space-y-6">
            <div v-if="mode === 'edit'" class="rounded-card border border-line bg-card p-4 shadow-xs dark:border-dark-700 dark:bg-dark-900">
              <div class="flex items-center justify-between gap-3">
                <div>
                  <h2 class="text-sm font-semibold text-ink dark:text-white">{{ t('ai.image.assetRefs', '引用已有资产') }}</h2>
                  <p class="mt-1 text-xs text-ink-soft dark:text-dark-400">{{ t('ai.image.assetRefsHint', '可直接把现有作品作为原图或遮罩。') }}</p>
                </div>
              </div>
              <div class="mt-3 grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                <article
                  v-for="artwork in recentArtworks.slice(0, 6)"
                  :key="`ref-${artwork.id}`"
                  class="overflow-hidden rounded-card border border-line bg-page dark:border-dark-700 dark:bg-dark-950"
                >
                  <img :src="artwork.thumbnail_url || artwork.image_url" :alt="artwork.title" class="h-32 w-full object-cover" />
                  <div class="space-y-2 p-3">
                    <div class="min-w-0">
                      <h3 class="line-clamp-1 text-sm font-semibold text-ink dark:text-white">{{ artwork.title }}</h3>
                      <p class="text-[11px] text-ink-soft dark:text-dark-400">{{ resolveLineLabel(artwork.line_id, artwork.line_name) }}</p>
                    </div>
                    <div class="flex flex-wrap gap-2">
                      <button class="btn btn-secondary btn-xs" type="button" @click="useArtworkAsSource(artwork)">{{ t('ai.image.useAsSource', '用作原图') }}</button>
                      <button class="btn btn-secondary btn-xs" type="button" @click="useArtworkAsMask(artwork)">{{ t('ai.image.useAsMask', '用作遮罩') }}</button>
                    </div>
                  </div>
                </article>
                <EmptyState
                  v-if="recentArtworks.length === 0 && !loading"
                  :title="t('ai.image.noRecent', '还没有作品')"
                  :description="t('ai.image.noRecentDesc', '生成或编辑后的作品会自动出现在这里。')"
                />
              </div>
            </div>

            <div v-if="featuredArtwork" class="rounded-card border border-line bg-card p-4 shadow-xs dark:border-dark-700 dark:bg-dark-900">
              <div class="mb-3 flex items-center justify-between">
                <div>
                  <h2 class="text-lg font-semibold text-ink dark:text-white">{{ featuredArtwork.title }}</h2>
                  <p class="mt-1 text-sm text-ink-soft dark:text-dark-400">{{ resolveLineLabel(featuredArtwork.line_id, featuredArtwork.line_name) }} · {{ featuredArtwork.style || 'cinematic' }} · {{ featuredArtwork.size || '1024x1024' }}</p>
                </div>
                <span class="rounded-full bg-brand-50 px-3 py-1 text-xs font-medium text-brand-700 dark:bg-brand-900/30 dark:text-brand-200">
                  {{ featuredArtwork.visibility === 'private' ? t('ai.prompt.private', '私有') : t('ai.prompt.public', '公开') }}
                </span>
              </div>
              <div class="overflow-hidden rounded-card bg-page dark:bg-dark-800">
                <img :src="featuredArtwork.image_url" :alt="featuredArtwork.title" class="h-[420px] w-full object-cover" />
              </div>
              <p class="mt-3 whitespace-pre-wrap text-sm text-ink-body dark:text-dark-400">{{ featuredArtwork.prompt }}</p>
            </div>

            <div class="rounded-card border border-line bg-page p-4 dark:border-dark-700 dark:bg-dark-900">
              <div class="mb-3 flex items-center justify-between">
                <h2 class="text-sm font-semibold text-ink dark:text-white">{{ t('ai.image.recent', '最近作品') }}</h2>
                <RouterLink to="/ai/gallery" class="text-sm font-medium text-brand-600 hover:text-brand-700 dark:text-brand-400">
                  {{ t('ai.gallery.open', '打开画廊') }}
                </RouterLink>
              </div>
              <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                <article
                  v-for="artwork in recentArtworks"
                  :key="artwork.id"
                  class="overflow-hidden rounded-card border border-line bg-card transition-transform hover:-translate-y-0.5 dark:border-dark-700 dark:bg-dark-950"
                >
                  <img :src="artwork.thumbnail_url || artwork.image_url" :alt="artwork.title" class="h-48 w-full object-cover" />
                  <div class="space-y-2 p-3">
                    <div class="flex items-start justify-between gap-3">
                      <h3 class="line-clamp-1 text-sm font-semibold text-ink dark:text-white">{{ artwork.title }}</h3>
                      <span class="rounded-full bg-line px-2 py-0.5 text-[11px] text-ink-body dark:bg-dark-800 dark:text-dark-300">{{ artwork.style }}</span>
                    </div>
                    <p class="line-clamp-2 text-xs text-ink-soft dark:text-dark-400">{{ artwork.prompt }}</p>
                  </div>
                </article>
                <EmptyState
                  v-if="!loading && recentArtworks.length === 0"
                  :title="t('ai.image.noRecent', '还没有作品')"
                  :description="t('ai.image.noRecentDesc', '生成或编辑后的作品会自动出现在这里。')"
                />
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import ImageUpload from '@/components/common/ImageUpload.vue'
import Icon from '@/components/icons/Icon.vue'
import Input from '@/components/common/Input.vue'
import Select from '@/components/common/Select.vue'
import TextArea from '@/components/common/TextArea.vue'
import { createAIArtwork, editAIArtwork, listAIArtworks } from '@/api'
import { useAiStudioStore, useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { AiArtwork, AiVisibility } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const aiStore = useAiStudioStore()

const loading = ref(false)
const generating = ref(false)
const recentArtworks = ref<AiArtwork[]>([])
const featuredArtwork = ref<AiArtwork | null>(null)
const mode = ref<'generate' | 'edit'>('generate')

const form = reactive({
  title: '',
  prompt: '',
  negative_prompt: '',
  source_image: '',
  mask_image: '',
  style: 'cinematic',
  size: '1024x1024',
  visibility: 'public' as AiVisibility
})

const lineOptions = computed(() =>
  aiStore.availableLines.map((line) => ({
    value: line.group_id,
    label: `${line.label} · ${line.key_count}`
  }))
)

const keyOptions = computed(() => {
  const currentLine = aiStore.selectedLine
  if (!currentLine) {
    return [{ value: null, label: t('ai.line.noKey', '暂无可用密钥') }]
  }
  const keys = currentLine.keys.length > 0
    ? currentLine.keys
    : currentLine.key_ids.map((id) => ({ id, name: `Key ${id}` }))
  return keys.length > 0
    ? keys.map((key) => ({
        value: key.id,
        label: `${key.name} (#${key.id})`
      }))
    : [{ value: null, label: t('ai.line.noKey', '暂无可用密钥') }]
})

const styleOptions = [
  { value: 'cinematic', label: t('ai.image.styles.cinematic', '电影感') },
  { value: 'illustration', label: t('ai.image.styles.illustration', '插画') },
  { value: 'editorial', label: t('ai.image.styles.editorial', '杂志大片') },
  { value: 'product', label: t('ai.image.styles.product', '产品视觉') },
  { value: 'anime', label: t('ai.image.styles.anime', '动漫') },
  { value: 'sketch', label: t('ai.image.styles.sketch', '草图') }
]

const sizeOptions = [
  { value: '1024x1024', label: '1024 x 1024' },
  { value: '1536x1024', label: '1536 x 1024' },
  { value: '1024x1536', label: '1024 x 1536' },
  { value: '1792x1024', label: '1792 x 1024' },
  { value: '1024x1792', label: '1024 x 1792' }
]

const visibilityOptions = [
  { value: 'public', label: t('ai.prompt.public', '公开') },
  { value: 'private', label: t('ai.prompt.private', '私有') }
]

const isEditMode = computed(() => mode.value === 'edit')
const editAvailable = computed(() => aiStore.runtimeInfo?.image_edit?.enabled === true)
const showRuntimeLineNotice = computed(() => !loading.value && !!aiStore.runtimeInfo && aiStore.availableLines.length === 0)
const canSubmit = computed(() => {
  if (!form.prompt.trim() || !aiStore.selectedLineId || !aiStore.selectedKeyId) return false
  if (!isEditMode.value) return true
  if (!editAvailable.value) return false
  return !!form.source_image.trim()
})

const submitLabel = computed(() => (isEditMode.value ? t('ai.image.submitEdit', '提交编辑') : t('ai.image.submitGenerate', '立即生成')))
const modeHint = computed(() =>
  isEditMode.value
    ? (editAvailable.value
        ? t('ai.image.editHint', '上传原图后可选遮罩编辑；key 仍由线路内自动选取。')
        : t('ai.image.editUnavailable', '当前 runtime 未启用编辑能力'))
    : t('ai.image.generateHint', '仅填写提示词即可生成，key 不会在界面中暴露。')
)
const editStatusLabel = computed(() =>
  editAvailable.value
    ? t('ai.image.editAvailable', '编辑能力已启用')
    : t('ai.image.editUnavailable', '当前 runtime 未启用编辑能力')
)

function resolveLineLabel(lineId?: number | null, fallback?: string | null): string {
  if (fallback) return fallback
  if (typeof lineId === 'number') {
    return aiStore.availableLines.find((line) => line.group_id === lineId)?.label ?? t('ai.line.unassigned', '未分配线路')
  }
  return t('ai.line.unassigned', '未分配线路')
}

async function loadRecentArtworks() {
  loading.value = true
  try {
    await aiStore.loadRuntimeLines()
    const response = await listAIArtworks(1, 9, {
      line_id: aiStore.selectedLineId ?? 'all',
      status: 'ready'
    })
    recentArtworks.value = response.items
    featuredArtwork.value = response.items[0] ?? null
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    loading.value = false
  }
}

function resetDraft(): void {
  form.title = ''
  form.prompt = ''
  form.negative_prompt = ''
  form.source_image = ''
  form.mask_image = ''
}

function useArtworkAsSource(artwork: AiArtwork): void {
  form.source_image = artwork.image_url
  if (!form.title.trim()) {
    form.title = `${artwork.title} Edit`
  }
}

function useArtworkAsMask(artwork: AiArtwork): void {
  form.mask_image = artwork.image_url
}

async function submitArtwork() {
  if (!canSubmit.value) return
  generating.value = true
  try {
    const payload = {
      title: form.title.trim() || undefined,
      prompt: form.prompt.trim(),
      negative_prompt: form.negative_prompt.trim() || undefined,
      line_id: aiStore.selectedLineId,
      key_id: aiStore.selectedKeyId,
      style: form.style,
      size: form.size,
      visibility: form.visibility,
      mode: mode.value,
      source_image: isEditMode.value ? form.source_image.trim() || null : undefined,
      mask_image: isEditMode.value ? form.mask_image.trim() || null : undefined
    }
    const created = isEditMode.value ? await editAIArtwork(payload) : await createAIArtwork(payload)
    featuredArtwork.value = created
    recentArtworks.value = [created, ...recentArtworks.value.filter((item) => item.id !== created.id)].slice(0, 9)
    resetDraft()
    appStore.showSuccess(isEditMode.value ? t('ai.image.edited', '编辑请求已提交') : t('ai.image.created', '作品已生成'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    generating.value = false
  }
}

onMounted(async () => {
  await loadRecentArtworks()
})
</script>
