<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowUpRight, ChevronLeft, ChevronRight, Copy, ExternalLink, ImageOff, Search } from '@lucide/vue'
import Button from '@/components/ui/Button.vue'
import UiModal from '@/components/ui/UiModal.vue'
import { useClipboard } from '@/composables/useClipboard'
import { referenceCases, type ReferenceCase, type ReferenceCategory } from './referenceCases'

const emit = defineEmits<{ create: [payload: { kind: 'image'; prompt: string; model?: string }] }>()
const { locale } = useI18n()
const { copyToClipboard } = useClipboard()
const language = computed(() => locale.value.startsWith('zh') ? 'zh' : 'en')
const search = ref('')
const category = ref<'all' | ReferenceCategory>('all')
const selected = ref<ReferenceCase | null>(null)
const brokenImages = ref(new Set<string>())
const page = ref(1)
const pageSize = 24
const grid = ref<HTMLElement | null>(null)
const text = computed(() => language.value === 'zh' ? {
  search: '搜索灵感、提示词或作者', categories: '案例分类', all: '全部', product: '产品设计', poster: '海报设计',
  illustration: '插画', architecture: '建筑空间', photography: '摄影创意', preview: '查看案例',
  create: '一键创作', copy: '复制提示词', prompt: '完整提示词', promptAuthor: '提示词', imageAuthor: '图片',
  original: '原始来源', repository: '案例档案', close: '关闭', empty: '没有匹配的案例', clear: '清除筛选',
  source: '精选来源', resized: '预览图仅缩小并转为 JPEG；提示词保留原文。', unavailable: '预览图暂不可用',
  previous: '上一页', next: '下一页', pagination: '案例分页', attribution: '署名记录',
} : {
  search: 'Search ideas, prompts, or authors', categories: 'Case categories', all: 'All', product: 'Products', poster: 'Posters',
  illustration: 'Illustration', architecture: 'Architecture', photography: 'Photography', preview: 'Preview case',
  create: 'Create with prompt', copy: 'Copy prompt', prompt: 'Full prompt', promptAuthor: 'Prompt', imageAuthor: 'Image',
  original: 'Original source', repository: 'Case archive', close: 'Close', empty: 'No matching cases', clear: 'Clear filters',
  source: 'Curated source', resized: 'Preview images are resized JPEGs; prompts retain their original text.', unavailable: 'Preview unavailable',
  previous: 'Previous page', next: 'Next page', pagination: 'Case pages', attribution: 'Attribution record',
})
const categories: Array<'all' | ReferenceCategory> = ['all', 'product', 'poster', 'illustration', 'architecture', 'photography']
const filteredCases = computed(() => {
  const query = search.value.trim().toLocaleLowerCase()
  return referenceCases.filter(item => (category.value === 'all' || item.category === category.value) &&
    (!query || [item.title.zh, item.title.en, item.prompt.zh, item.prompt.en, item.promptAuthor, item.imageAuthor]
      .some(value => value.toLocaleLowerCase().includes(query))))
})
const pageCount = computed(() => Math.max(1, Math.ceil(filteredCases.value.length / pageSize)))
const visibleCases = computed(() => filteredCases.value.slice((page.value - 1) * pageSize, page.value * pageSize))
watch([search, category], () => { page.value = 1 })
function changePage(value: number) {
  page.value = Math.max(1, Math.min(pageCount.value, value))
  grid.value?.scrollIntoView?.({ block: 'start' })
}

function create(item: ReferenceCase) {
  selected.value = null
  emit('create', { kind: 'image', prompt: item.prompt[language.value] })
}

function resetFilters() {
  search.value = ''
  category.value = 'all'
}
</script>

<template>
  <section class="reference-inspiration">
    <div class="reference-tools">
      <label class="reference-search">
        <Search :size="16" aria-hidden="true" />
        <input v-model="search" type="search" maxlength="200" :placeholder="text.search" :aria-label="text.search" />
      </label>
      <span class="reference-count">{{ filteredCases.length }} / {{ referenceCases.length }}</span>
    </div>

    <div class="reference-categories" role="group" :aria-label="text.categories">
      <button v-for="item in categories" :key="item" type="button" :aria-pressed="category === item" :class="{ selected: category === item }" @click="category = item">
        {{ text[item] }}
      </button>
    </div>

    <div v-if="filteredCases.length" ref="grid" class="reference-grid">
      <article v-for="item in visibleCases" :key="item.id" class="reference-case" :data-case-id="item.id">
        <button type="button" class="reference-case-image" :aria-label="`${text.preview}: ${item.title[language]}`" @click="selected = item">
          <span v-if="brokenImages.has(item.id)" class="reference-image-error"><ImageOff :size="24" aria-hidden="true" />{{ text.unavailable }}</span>
          <img v-else :src="item.image" :alt="item.title[language]" :width="item.width" :height="item.height" loading="lazy" decoding="async" @error="brokenImages.add(item.id)" />
        </button>
        <div class="reference-case-body">
          <h3 :title="item.title[language]">{{ item.title[language] }}</h3>
          <p class="reference-case-author">{{ text.promptAuthor }}: <a :href="item.promptAuthorUrl" target="_blank" rel="noopener noreferrer">{{ item.promptAuthor }}</a></p>
          <p class="reference-case-author">{{ text.imageAuthor }}: <a :href="item.imageAuthorUrl" target="_blank" rel="noopener noreferrer">{{ item.imageAuthor }}</a></p>
          <div class="reference-case-actions">
            <a href="https://creativecommons.org/licenses/by/4.0/" target="_blank" rel="noopener noreferrer" class="reference-license">{{ item.license }}</a>
            <a :href="item.repositoryUrl" target="_blank" rel="noopener noreferrer" class="reference-license">{{ text.repository }}</a>
            <button type="button" class="reference-create" :title="text.create" :aria-label="`${text.create}: ${item.title[language]}`" @click="create(item)">
              <ArrowUpRight :size="18" aria-hidden="true" />
            </button>
          </div>
        </div>
      </article>
    </div>
    <div v-else class="reference-empty" role="status">
      <p>{{ text.empty }}</p>
      <Button variant="secondary" @click="resetFilters">{{ text.clear }}</Button>
    </div>

    <nav v-if="pageCount > 1" class="reference-pagination" :aria-label="text.pagination">
      <button type="button" :disabled="page === 1" :title="text.previous" :aria-label="text.previous" @click="changePage(page - 1)"><ChevronLeft :size="18" /></button>
      <span aria-live="polite">{{ page }} / {{ pageCount }}</span>
      <button type="button" :disabled="page === pageCount" :title="text.next" :aria-label="text.next" @click="changePage(page + 1)"><ChevronRight :size="18" /></button>
    </nav>

    <footer class="reference-attribution">
      <span>{{ text.source }}: <a href="https://github.com/jamez-bondos/awesome-gpt4o-images" target="_blank" rel="noopener noreferrer">awesome-gpt4o-images</a></span>
      <a href="/creation/inspiration/LICENSE.txt" target="_blank" rel="noopener noreferrer">CC-BY-4.0</a>
      <span>{{ text.resized }}</span>
    </footer>

    <UiModal :open="selected !== null" :title="selected?.title[language] || text.preview" width="xl" :close-label="text.close" @close="selected = null">
      <div v-if="selected" class="reference-preview">
        <div class="reference-preview-image">
          <img :src="selected.image" :alt="selected.title[language]" :width="selected.width" :height="selected.height" />
        </div>
        <div class="reference-preview-details">
          <div class="reference-source-links">
            <p>{{ text.promptAuthor }}: <a :href="selected.promptAuthorUrl" target="_blank" rel="noopener noreferrer">{{ selected.promptAuthor }}</a></p>
            <p>{{ text.imageAuthor }}: <a :href="selected.imageAuthorUrl" target="_blank" rel="noopener noreferrer">{{ selected.imageAuthor }}</a></p>
            <div class="reference-link-row">
              <a :href="selected.sourceUrl" target="_blank" rel="noopener noreferrer">{{ text.original }} <ExternalLink :size="12" aria-hidden="true" /></a>
              <a :href="selected.repositoryUrl" target="_blank" rel="noopener noreferrer">{{ text.repository }} <ExternalLink :size="12" aria-hidden="true" /></a>
              <a :href="selected.attributionUrl" target="_blank" rel="noopener noreferrer">{{ text.attribution }} <ExternalLink :size="12" aria-hidden="true" /></a>
              <a href="https://creativecommons.org/licenses/by/4.0/" target="_blank" rel="noopener noreferrer">{{ selected.license }}</a>
            </div>
          </div>
          <h3>{{ text.prompt }}</h3>
          <pre class="reference-prompt">{{ selected.prompt[language] }}</pre>
        </div>
      </div>
      <template #footer>
        <Button v-if="selected" variant="secondary" @click="copyToClipboard(selected.prompt[language])"><Copy :size="16" aria-hidden="true" />{{ text.copy }}</Button>
        <Button v-if="selected" @click="create(selected)"><ArrowUpRight :size="16" aria-hidden="true" />{{ text.create }}</Button>
      </template>
    </UiModal>
  </section>
</template>

<style scoped>
.reference-pagination { display: flex; align-items: center; justify-content: center; gap: 14px; }
.reference-pagination button { display: grid; place-items: center; width: 36px; height: 36px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface); color: var(--foreground); }
.reference-pagination button:disabled { opacity: .4; cursor: default; }
.reference-pagination span { min-width: 60px; text-align: center; font-size: 12px; color: var(--muted); font-variant-numeric: tabular-nums; }
.reference-inspiration { display: flex; flex-direction: column; gap: 18px; min-width: 0; }
.reference-tools { display: flex; align-items: center; gap: 14px; }
.reference-search { display: flex; align-items: center; gap: 10px; width: min(100%, 440px); height: 36px; padding: 0 12px; border: 1px solid var(--border); border-radius: var(--radius-field); background: color-mix(in oklch, var(--surface) 85%, transparent); box-shadow: var(--field-shadow); color: var(--muted); }
.reference-search:focus-within { border-color: var(--accent); box-shadow: var(--field-shadow), 0 0 0 3px color-mix(in oklch, var(--accent) 18%, transparent); }
.reference-search input { flex: 1; min-width: 0; border: 0; outline: 0; background: transparent; color: var(--foreground); font-size: 13px; }
.reference-count { flex: none; font-size: 12px; color: var(--muted); font-variant-numeric: tabular-nums; }
.reference-categories { display: flex; gap: 18px; overflow-x: auto; border-bottom: 1px solid var(--border); }
.reference-categories button { flex: none; min-height: 40px; padding: 0 0 10px; border: 0; border-bottom: 2px solid transparent; background: transparent; color: var(--muted); font-size: 13px; cursor: pointer; }
.reference-categories button.selected { color: var(--foreground); border-bottom-color: var(--accent); }
.reference-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.reference-case { min-width: 0; overflow: hidden; border: 1px solid var(--border); border-radius: var(--radius-card); background: color-mix(in oklch, var(--surface) 85%, transparent); box-shadow: var(--shadow); }
.reference-case-image { display: flex; justify-content: center; align-items: center; width: 100%; aspect-ratio: 4 / 5; padding: 0; border: 0; background: var(--surface-secondary); cursor: pointer; overflow: hidden; }
.reference-case-image img { width: 100%; height: 100%; object-fit: contain; }
.reference-image-error { display: flex; flex-direction: column; align-items: center; gap: 10px; color: var(--muted); font-size: 12px; }
.reference-case-body { padding: 12px; }
.reference-case h3 { display: -webkit-box; min-height: 40px; margin: 0 0 8px; overflow: hidden; -webkit-line-clamp: 2; -webkit-box-orient: vertical; font-size: 14px; line-height: 20px; font-weight: 600; }
.reference-case-author { margin: 4px 0; color: var(--muted); font-size: 11px; overflow-wrap: anywhere; }
.reference-inspiration a { color: var(--muted); text-decoration: none; }
.reference-inspiration a:hover { color: var(--accent); text-decoration: underline; }
.reference-case-actions { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 6px; margin-top: 10px; }
.reference-license { font-size: 10px; }
.reference-create { display: inline-flex; justify-content: center; align-items: center; width: 36px; height: 36px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface-secondary); color: var(--foreground); cursor: pointer; }
.reference-create:hover { color: var(--accent); border-color: var(--accent); }
.reference-attribution { display: flex; flex-wrap: wrap; gap: 6px 12px; padding: 14px 0; border-top: 1px solid var(--border); font-size: 11px; color: var(--muted); }
.reference-empty { display: flex; flex-direction: column; align-items: center; gap: 14px; padding: 64px 16px; color: var(--muted); }
.reference-preview { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); align-items: start; gap: 24px; }
.reference-preview-image { display: flex; align-items: center; justify-content: center; background: var(--surface-secondary); }
.reference-preview-image img { display: block; width: 100%; height: auto; max-height: 60vh; object-fit: contain; }
.reference-preview-details { min-width: 0; }
.reference-preview-details h3 { margin: 20px 0 10px; font-size: 14px; font-weight: 600; }
.reference-source-links { display: flex; flex-direction: column; gap: 8px; color: var(--muted); font-size: 12px; overflow-wrap: anywhere; }
.reference-source-links p { margin: 0; }
.reference-source-links a { color: var(--accent); text-decoration: none; }
.reference-link-row { display: flex; flex-wrap: wrap; gap: 10px; }
.reference-link-row a { display: inline-flex; align-items: center; gap: 4px; }
.reference-prompt { max-height: 40vh; overflow: auto; margin: 0; white-space: pre-wrap; overflow-wrap: anywhere; font: inherit; font-size: 13px; line-height: 1.7; color: var(--foreground); }
@media (max-width: 1100px) { .reference-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
@media (max-width: 767px) {
  .reference-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
  .reference-case-body { padding: 10px; }
  .reference-create { width: 44px; height: 44px; }
  .reference-preview { grid-template-columns: minmax(0, 1fr); gap: 16px; }
  .reference-preview-image img { max-height: 42vh; }
  .reference-prompt { max-height: none; }
}
@media (max-width: 359px) { .reference-grid { grid-template-columns: minmax(0, 1fr); } }
</style>
