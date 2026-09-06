<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import SegmentedControl from '@/components/ui/SegmentedControl.vue'
import UiModal from '@/components/ui/UiModal.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'
import { listPublications, withdrawPublication, type Publication, type PublicationKind } from '../publicationApi'
import { usePublicationText } from '../publicationText'
import ReferenceInspiration from './ReferenceInspiration.vue'

const emit = defineEmits<{ create: [payload: { kind: PublicationKind; prompt: string; model?: string }] }>()
const auth = useAuthStore()
const text = usePublicationText()
const collection = ref<'featured' | 'community'>('featured')
const collections = computed(() => [
  { value: 'featured' as const, label: text('featured') },
  { value: 'community' as const, label: text('community') },
])
const kind = ref<'all' | PublicationKind>('all')
const search = ref('')
const query = ref('')
const items = ref<Publication[]>([])
const total = ref(0)
const page = ref(0)
const pages = ref(0)
const loading = ref(false)
const error = ref('')
const selected = ref<Publication | null>(null)
const withdrawTarget = ref<Publication | null>(null)
const withdrawing = ref(false)
const withdrawError = ref('')
const brokenMedia = ref(new Set<number>())
const options = computed(() => [
  { value: 'all' as const, label: text('all') },
  { value: 'image' as const, label: text('images') },
  { value: 'video' as const, label: text('videos') },
])
let controller: AbortController | undefined
let generation = 0
let searchTimer: ReturnType<typeof setTimeout> | undefined
let disposed = false

async function load(append = false) {
  if (append && (loading.value || page.value >= pages.value)) return
  controller?.abort()
  controller = new AbortController()
  const current = ++generation
  const nextPage = append ? page.value + 1 : 1
  loading.value = true
  error.value = ''
  if (!append) { items.value = []; page.value = 0; pages.value = 0; total.value = 0; brokenMedia.value = new Set() }
  try {
    const response = await listPublications({ page: nextPage, page_size: 24, kind: kind.value === 'all' ? undefined : kind.value, search: query.value || undefined }, controller.signal)
    if (current !== generation) return
    const combined = append ? [...items.value, ...response.items] : response.items
    items.value = [...new Map(combined.map(item => [item.id, item])).values()]
    total.value = response.total
    page.value = response.page
    pages.value = response.pages
  } catch (cause) {
    if (current === generation) error.value = extractApiErrorMessage(cause, text('loadFailed'))
  } finally {
    if (current === generation) loading.value = false
  }
}
watch(search, value => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => { query.value = value.trim() }, 300)
})
watch([kind, query, collection], () => {
  if (collection.value === 'community') void load()
  else { generation++; controller?.abort(); loading.value = false }
}, { immediate: true })

function create(item: Publication) {
  selected.value = null
  emit('create', { kind: item.kind, prompt: item.prompt, model: item.model || undefined })
}
function requestWithdraw(item: Publication) {
  if (auth.user?.id !== item.owner_user_id) return
  withdrawTarget.value = item
  withdrawError.value = ''
}
async function withdraw() {
  const target = withdrawTarget.value
  if (!target || withdrawing.value || target.owner_user_id !== auth.user?.id) return
  withdrawing.value = true
  withdrawError.value = ''
  try {
    await withdrawPublication(target.id)
    if (disposed) return
    if (selected.value?.id === target.id) selected.value = null
    withdrawTarget.value = null
    await load()
  } catch (cause) {
    if (!disposed) {
      withdrawError.value = extractApiErrorMessage(cause, text('withdrawFailed'))
      // Withdrawal may already be effective even when object cleanup failed.
      if (selected.value?.id === target.id) selected.value = null
      await load()
    }
  } finally {
    if (!disposed) withdrawing.value = false
  }
}
function cancelWithdraw() {
  if (!withdrawing.value) withdrawTarget.value = null
}
onBeforeUnmount(() => { disposed = true; generation++; controller?.abort(); clearTimeout(searchTimer) })
defineExpose({ refresh: () => {
  if (collection.value === 'community') return load()
  collection.value = 'community'
} })
</script>

<template>
  <section class="inspiration-wall">
    <header class="inspiration-header">
      <div class="inspiration-heading"><span v-if="collection === 'community' && !loading && !error" class="text-muted">{{ total }}</span></div>
      <div v-if="collection === 'community'" class="inspiration-tools">
        <label class="inspiration-search"><Icon name="search" size="sm" /><input v-model="search" type="search" maxlength="200" :placeholder="text('search')" :aria-label="text('search')" /></label>
        <button class="icon-btn" type="button" :title="text('refresh')" :aria-label="text('refresh')" :disabled="loading" @click="load()"><Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" /></button>
      </div>
    </header>
    <SegmentedControl v-model="collection" :options="collections" class="inspiration-kinds" />
    <ReferenceInspiration v-if="collection === 'featured'" @create="emit('create', $event)" />
    <template v-else>
    <SegmentedControl v-model="kind" :options="options" size="sm" class="inspiration-kinds" />

    <div v-if="items.length" class="inspiration-grid">
      <article v-for="item in items" :key="item.id" class="inspiration-work">
        <button type="button" class="inspiration-media" :aria-label="`${text('preview')}: ${item.title}`" @click="selected = item">
          <span v-if="brokenMedia.has(item.id)" class="inspiration-unavailable"><Icon name="eyeOff" size="lg" />{{ text('unavailable') }}</span>
          <img v-else-if="item.kind === 'image'" :src="item.media_url" :alt="item.title" loading="lazy" decoding="async" @error="brokenMedia.add(item.id)" />
          <template v-else><video :src="item.media_url" preload="metadata" muted playsinline @error="brokenMedia.add(item.id)" /><span class="inspiration-play"><Icon name="play" size="lg" /></span></template>
        </button>
        <div class="inspiration-work-body">
          <div class="inspiration-work-title"><h3>{{ item.title }}</h3><button v-if="auth.user?.id === item.owner_user_id" type="button" class="icon-btn" :title="text('withdraw')" :aria-label="text('withdraw')" @click="requestWithdraw(item)"><Icon name="eyeOff" size="sm" /></button></div>
          <p class="inspiration-work-model text-muted">{{ item.model || (item.kind === 'image' ? text('images') : text('videos')) }}</p>
          <button type="button" class="inspiration-create" :disabled="!item.prompt.trim()" @click="create(item)"><Icon name="sparkles" size="sm" />{{ text('create') }}<Icon name="arrowRight" size="sm" /></button>
        </div>
      </article>
    </div>
    <div v-else-if="loading" class="inspiration-empty" role="status"><Icon name="refresh" size="lg" class="animate-spin" />{{ text('loading') }}</div>
    <div v-else-if="!error" class="inspiration-empty"><Icon name="globe" size="xl" /><h3>{{ query || kind !== 'all' ? text('noResults') : text('empty') }}</h3></div>
    <div v-if="error" class="inspiration-error" role="alert"><p>{{ error }}</p><button type="button" class="btn-glass-secondary" @click="load(page > 0)">{{ text('retry') }}</button></div>
    <button v-if="page < pages" type="button" class="btn-glass-secondary inspiration-more" :disabled="loading" @click="load(true)">{{ loading ? text('loading') : text('more') }}</button>
    </template>

    <UiModal :open="!!selected" :title="selected?.title || text('preview')" :close-label="text('close')" width="lg" @close="selected = null">
      <template v-if="selected">
        <div class="inspiration-preview"><img v-if="selected.kind === 'image'" :src="selected.media_url" :alt="selected.title" /><video v-else :src="selected.media_url" controls playsinline preload="metadata" /></div>
        <div class="inspiration-detail"><span class="text-muted">{{ selected.model }}</span><h3>{{ text('prompt') }}</h3><p>{{ selected.prompt }}</p></div>
      </template>
      <template #footer><button v-if="selected" type="button" class="btn-glass-primary" :disabled="!selected.prompt.trim()" @click="create(selected)"><Icon name="sparkles" size="sm" />{{ text('create') }}</button></template>
    </UiModal>
    <ConfirmDialog :show="!!withdrawTarget" :title="text('withdraw')" :message="text('withdrawNotice')" :confirm-text="withdrawing ? text('withdrawing') : text('withdraw')" :cancel-text="text('cancel')" :confirming="withdrawing" tone="danger" @confirm="withdraw" @cancel="cancelWithdraw"><p v-if="withdrawError" class="text-danger-text" role="alert">{{ withdrawError }}</p></ConfirmDialog>
  </section>
</template>

<style scoped>
.inspiration-wall { display: flex; flex-direction: column; gap: 16px; padding: 0; min-width: 0; }
.inspiration-header, .inspiration-heading, .inspiration-tools { display: flex; align-items: center; gap: 12px; }
.inspiration-header { justify-content: space-between; flex-wrap: wrap; }
.inspiration-heading h2 { font-size: 22px; font-weight: 600; }
.inspiration-heading span { font-size: 13px; font-variant-numeric: tabular-nums; }
.inspiration-tools { min-width: 0; }
.inspiration-search { display: flex; align-items: center; gap: 8px; border: 1px solid var(--border); background: var(--surface); border-radius: var(--radius-field); padding: 8px 12px; box-shadow: var(--field-shadow); width: 280px; max-width: 100%; color: var(--muted); }
.inspiration-search input { background: transparent; min-width: 0; width: 100%; font-size: 13px; color: var(--foreground); outline: none; }
.inspiration-search:focus-within { border-color: var(--accent); }
.inspiration-kinds { align-self: flex-start; }
.inspiration-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
.inspiration-work { border: 1px solid var(--border); border-radius: var(--radius-card); overflow: hidden; background: color-mix(in oklch, var(--surface) 85%, transparent); box-shadow: var(--shadow); min-width: 0; }
.inspiration-media { position: relative; display: flex; width: 100%; aspect-ratio: 4 / 3; background: var(--surface-secondary); align-items: center; justify-content: center; }
.inspiration-media img, .inspiration-media video { width: 100%; height: 100%; object-fit: contain; }
.inspiration-media video { pointer-events: none; }
.inspiration-play { position: absolute; display: flex; align-items: center; justify-content: center; border-radius: 50%; padding: 12px; color: var(--foreground); background: var(--surface); }
.inspiration-work-body { padding: 14px; }
.inspiration-work-title { display: flex; align-items: center; justify-content: space-between; gap: 8px; min-height: 32px; }
.inspiration-work-title h3 { font-size: 14px; font-weight: 600; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.inspiration-work-model { font-size: 12px; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; margin: 2px 0 14px; }
.inspiration-create { display: flex; align-items: center; gap: 8px; color: var(--accent); width: 100%; font-size: 13px; }
.inspiration-create svg:last-child { margin-left: auto; }
.inspiration-create:disabled { color: var(--muted); cursor: not-allowed; }
.inspiration-empty, .inspiration-unavailable { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; color: var(--muted); font-size: 14px; }
.inspiration-empty { min-height: 280px; }
.inspiration-error { display: flex; align-items: center; gap: 12px; color: var(--danger-text); }
.inspiration-more { align-self: center; }
.inspiration-preview { display: flex; justify-content: center; background: var(--surface-secondary); }
.inspiration-preview img, .inspiration-preview video { max-height: 58vh; max-width: 100%; object-fit: contain; }
.inspiration-detail { display: flex; flex-direction: column; gap: 10px; margin-top: 16px; font-size: 13px; }
.inspiration-detail p { white-space: pre-wrap; overflow-wrap: anywhere; max-height: 180px; overflow-y: auto; }
@media (min-width: 1500px) { .inspiration-grid { grid-template-columns: repeat(4, minmax(0, 1fr)); } }
@media (max-width: 1000px) { .inspiration-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 600px) { .inspiration-wall { padding: 0; } .inspiration-tools { width: 100%; } .inspiration-search { width: 100%; flex: 1; } .inspiration-grid { grid-template-columns: minmax(0, 1fr); gap: 16px; } }
</style>
