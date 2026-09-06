<!-- Adapted from chat-vue@80649c38 RatioResolutionPopover/QualityPopover; see MediaReferenceLicense.txt. -->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { SlidersHorizontal, X } from '@lucide/vue'
import type { MediaMode, MediaSettings, VideoOperation } from '../localMedia'
import { mediaMessages } from './mediaMessages'
import { GEMINI_IMAGE_ASPECT_RATIOS, isGeminiImageModel } from '../mediaModels'

const props = defineProps<{ mode: MediaMode; model: string; settings: MediaSettings; videoOperation?: VideoOperation; disabled?: boolean }>()
const emit = defineEmits<{ 'update:settings': [settings: MediaSettings] }>()
const { t } = useI18n({ useScope: 'local', messages: mediaMessages })
const open = ref(false)
const root = ref<HTMLElement | null>(null)
const isGpt = computed(() => /^gpt-image/i.test(props.model))
const isGpt2 = computed(() => /^gpt-image-2/i.test(props.model))
const isDalle = computed(() => /^dall-e/i.test(props.model))
const isDalle2 = computed(() => /^dall-e-2/i.test(props.model))
const isGrok = computed(() => /grok/i.test(props.model))
const isGemini = computed(() => isGeminiImageModel(props.model))
const ratios = computed(() => isGemini.value ? [...GEMINI_IMAGE_ASPECT_RATIOS, 'auto'] : isGpt2.value ? ['1:1', '3:2', '16:9', '21:9', '9:16', '4:3', '3:4', 'auto'] : isGrok.value ? ['1:1', '3:2', '16:9', '9:16', '4:3', '3:4', 'auto'] : isDalle2.value ? ['1:1'] : isDalle.value ? ['1:1', '7:4', '4:7'] : isGpt.value ? ['1:1', '3:2', '2:3', 'auto'] : ['auto'])
const ratio = computed(() => props.settings.ratio || props.settings.aspectRatio || 'auto')
const qualityOptions = computed(() => isDalle2.value ? ['standard'] : isDalle.value ? ['standard', 'hd'] : ['low', 'medium', 'high'])
const label = computed(() => props.mode === 'video' ? props.videoOperation === 'edit' ? t('sourceSettings') : props.videoOperation === 'extension' ? `+${props.settings.duration ?? 6}s` : `${props.settings.duration || 5}s · ${props.settings.resolution || '720p'}` : [ratio.value === 'auto' ? t('auto') : ratio.value, isGemini.value || (isGpt2.value && ratio.value !== 'auto') ? props.settings.resolution || '1K' : null].filter(Boolean).join(' · '))

function patch(value: Partial<MediaSettings>) {
  if (props.disabled) return
  const next = { ...props.settings, ...value }
  if (props.mode === 'image') {
    const selectedRatio = next.ratio || 'auto'
    if (isGemini.value) {
      next.aspectRatio = selectedRatio === 'auto' ? undefined : selectedRatio
      next.resolution = ['1K', '2K', '4K'].includes(next.resolution || '') ? next.resolution : '1K'
      next.size = undefined
      next.quality = undefined
    } else if (isGrok.value) {
      next.aspectRatio = selectedRatio === 'auto' ? undefined : selectedRatio
      next.size = undefined
      next.quality = undefined
    } else if (isGpt2.value) {
      next.aspectRatio = undefined
      const resolution = next.resolution || '1K'
      const dimensions: Record<string, Record<string, string>> = {
        '1K': { '1:1': '1024x1024', '3:2': '1024x683', '16:9': '1024x576', '21:9': '1024x439', '9:16': '576x1024', '4:3': '1024x768', '3:4': '768x1024' },
        '2K': { '1:1': '2048x2048', '3:2': '2048x1365', '16:9': '2048x1152', '21:9': '2048x878', '9:16': '1152x2048', '4:3': '2048x1536', '3:4': '1536x2048' },
        '4K': { '1:1': '2880x2880', '3:2': '3456x2304', '16:9': '3840x2160', '21:9': '3808x1632', '9:16': '2160x3840', '4:3': '3264x2448', '3:4': '2448x3264' },
      }
      const size = dimensions[resolution]?.[selectedRatio]
      next.size = size ? size.split('x').map(value => Math.round(Number(value) / 16) * 16).join('x') : 'auto'
    } else if (isGpt.value || isDalle.value) {
      next.aspectRatio = undefined
      next.size = isDalle2.value || selectedRatio === '1:1' ? '1024x1024' : selectedRatio === '7:4' && isDalle.value ? '1792x1024' : selectedRatio === '4:7' && isDalle.value ? '1024x1792' : selectedRatio === '3:2' ? '1536x1024' : selectedRatio === '2:3' ? '1024x1536' : (isDalle.value ? '1024x1024' : 'auto')
    } else {
      next.aspectRatio = undefined
      next.size = undefined
      next.quality = undefined
    }
  }
  emit('update:settings', next)
}

watch(() => props.model, () => {
  if (props.mode !== 'image') return
  patch({ ratio: ratios.value.includes(ratio.value) ? ratio.value : ratios.value[0], quality: isGpt.value ? (['low', 'medium', 'high'].includes(props.settings.quality || '') ? props.settings.quality : 'high') : isDalle.value ? (qualityOptions.value.includes(props.settings.quality || '') ? props.settings.quality : 'standard') : undefined })
})
watch(() => props.disabled, disabled => { if (disabled) open.value = false })
function outside(event: MouseEvent) { if (!root.value?.contains(event.target as Node)) open.value = false }
onMounted(() => document.addEventListener('click', outside))
onBeforeUnmount(() => document.removeEventListener('click', outside))
</script>

<template>
  <div ref="root" class="media-parameters" @keydown.esc.stop="open = false">
    <button type="button" class="btn btn-secondary media-param-trigger" :disabled="disabled" :title="t('parameters')" :aria-expanded="open" :aria-label="t('parameters')" @click="open = !open"><SlidersHorizontal :size="15" /><span>{{ label }}</span></button>
    <div v-if="open" class="media-param-panel" role="group" :aria-label="t('parameters')">
      <header><strong>{{ t('parameters') }}</strong><button type="button" :aria-label="t('close')" @click="open = false"><X :size="16" /></button></header>
      <template v-if="mode === 'image'">
        <span class="media-param-label">{{ t('ratio') }}</span>
        <div class="media-ratios">
          <button v-for="option in ratios" :key="option" type="button" :aria-pressed="ratio === option" @click="patch({ ratio: option })"><span v-if="option !== 'auto'" class="media-ratio-shape" :style="{ aspectRatio: option.replace(':', '/') }" /><span v-else class="media-ratio-auto">A</span>{{ option === 'auto' ? t('auto') : option }}</button>
        </div>
        <template v-if="isGemini || (isGpt2 && ratio !== 'auto')"><span class="media-param-label">{{ t('resolution') }}</span><div class="media-param-segments"><button v-for="option in ['1K', '2K', '4K']" :key="option" type="button" :aria-pressed="(settings.resolution || '1K') === option" @click="patch({ resolution: option })">{{ option }}</button></div></template>
        <template v-if="isGpt || isDalle"><span class="media-param-label">{{ t('quality') }}</span><div class="media-param-segments"><button v-for="option in qualityOptions" :key="option" type="button" :aria-pressed="settings.quality === option" @click="patch({ quality: option })">{{ ['low', 'medium', 'high'].includes(option) ? t(option) : option }}</button></div></template>
        <p v-if="settings.size" class="media-param-size">{{ t('size') }} <span>{{ settings.size }}</span></p>
      </template>
      <template v-else-if="videoOperation === 'edit'">
        <span class="media-param-label">{{ t('sourceSettings') }}</span>
      </template>
      <template v-else-if="videoOperation === 'extension'">
        <label class="media-param-label" for="media-extension-duration">{{ t('extensionDuration') }}</label>
        <input id="media-extension-duration" class="field media-duration-input" type="number" min="2" max="10" step="1" :disabled="disabled" :value="settings.duration ?? 6" @input="emit('update:settings', { duration: Number(($event.target as HTMLInputElement).value) })" />
      </template>
      <template v-else>
        <span class="media-param-label">{{ t('duration') }}</span><div class="media-param-segments"><button v-for="option in [5, 10, 15]" :key="option" type="button" :aria-pressed="(settings.duration || 5) === option" @click="patch({ duration: option })">{{ t('seconds', { count: option }) }}</button></div>
        <span class="media-param-label">{{ t('resolution') }}</span><div class="media-param-segments"><button v-for="option in ['480p', '720p']" :key="option" type="button" :aria-pressed="(settings.resolution || '720p') === option" @click="patch({ resolution: option })">{{ option }}</button></div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.media-duration-input { width: 100%; }
.media-parameters { position: relative; }
.media-param-trigger { max-width: 180px; height: 36px; }
.media-param-trigger span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.media-param-panel { position: absolute; right: 0; bottom: calc(100% + 14px); z-index: 35; width: 304px; padding: 15px; border: 1px solid var(--border); border-radius: var(--radius-hero); box-shadow: var(--shadow-pop); background: var(--surface); max-height: 65dvh; overflow-y: auto; }
.media-param-panel header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; color: var(--foreground); font-size: 13px; }
.media-param-panel header button { border: 0; background: none; color: var(--muted); padding: 3px; }
.media-param-label { display: block; margin: 14px 0 8px; color: var(--muted); font-size: 11px; }
.media-ratios { display: grid; grid-template-columns: repeat(4, 1fr); gap: 6px; }
.media-ratios button { display: flex; flex-direction: column; justify-content: center; align-items: center; gap: 7px; height: 61px; border: 1px solid var(--border); border-radius: 6px; background: none; color: var(--foreground); font-size: 11px; }
.media-ratio-shape { width: 22px; max-height: 22px; border: 1px solid currentColor; border-radius: 2px; }
.media-ratio-auto { height: 22px; font-size: 15px; }
.media-param-segments { display: flex; gap: 6px; }
.media-param-segments button { flex: 1; padding: 8px 6px; border: 1px solid var(--border); border-radius: 6px; background: none; color: var(--foreground); font-size: 12px; }
.media-param-panel button[aria-pressed="true"] { border-color: var(--accent); background: color-mix(in oklch, var(--accent) 10%, transparent); color: var(--accent); }
.media-param-size { display: flex; justify-content: space-between; margin-top: 16px; font-size: 10px; color: var(--muted); }
.media-param-size span { font-family: var(--font-mono); }
@media (max-width: 600px) { .media-param-panel { position: fixed; left: 16px; right: 16px; bottom: 160px; width: auto; } }
</style>
