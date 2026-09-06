<script lang="ts">
import type { HighlighterCore } from 'shiki/core'

const languages = {
  javascript: () => import('shiki/langs/javascript.mjs'),
  typescript: () => import('shiki/langs/typescript.mjs'),
  json: () => import('shiki/langs/json.mjs'),
  html: () => import('shiki/langs/html.mjs'),
  css: () => import('shiki/langs/css.mjs'),
  python: () => import('shiki/langs/python.mjs'),
  shellscript: () => import('shiki/langs/shellscript.mjs'),
  sql: () => import('shiki/langs/sql.mjs'),
  go: () => import('shiki/langs/go.mjs'),
  rust: () => import('shiki/langs/rust.mjs'),
  java: () => import('shiki/langs/java.mjs'),
  c: () => import('shiki/langs/c.mjs'),
  cpp: () => import('shiki/langs/cpp.mjs'),
  yaml: () => import('shiki/langs/yaml.mjs'),
  xml: () => import('shiki/langs/xml.mjs'),
  markdown: () => import('shiki/langs/markdown.mjs'),
  diff: () => import('shiki/langs/diff.mjs'),
  dockerfile: () => import('shiki/langs/dockerfile.mjs'),
  ruby: () => import('shiki/langs/ruby.mjs'),
  php: () => import('shiki/langs/php.mjs'),
  swift: () => import('shiki/langs/swift.mjs'),
  kotlin: () => import('shiki/langs/kotlin.mjs'),
  toml: () => import('shiki/langs/toml.mjs'),
  graphql: () => import('shiki/langs/graphql.mjs'),
  vue: () => import('shiki/langs/vue.mjs'),
  jsx: () => import('shiki/langs/jsx.mjs'),
  tsx: () => import('shiki/langs/tsx.mjs'),
}
const aliases: Record<string, string> = { js: 'javascript', ts: 'typescript', py: 'python', sh: 'shellscript', bash: 'shellscript', shell: 'shellscript', zsh: 'shellscript', yml: 'yaml', md: 'markdown', rb: 'ruby', rs: 'rust', golang: 'go', 'c++': 'cpp', kt: 'kotlin', gql: 'graphql', docker: 'dockerfile' }
let highlighterPromise: Promise<HighlighterCore> | undefined
const loadingLanguages = new Map<string, Promise<void>>()

async function highlightCode(source: string, language: keyof typeof languages, isCurrent: () => boolean): Promise<string> {
  highlighterPromise ??= (async () => {
    const { createHighlighterCore } = await import('shiki/core')
    const { createJavaScriptRegexEngine } = await import('shiki/engine/javascript')
    return createHighlighterCore({
      themes: [import('shiki/themes/github-light.mjs'), import('shiki/themes/github-dark.mjs')],
      langs: [], engine: createJavaScriptRegexEngine(),
    })
  })().catch(error => { highlighterPromise = undefined; throw error })
  const highlighter = await highlighterPromise
  if (!loadingLanguages.has(language)) {
    loadingLanguages.set(language, languages[language]().then(module => highlighter.loadLanguage(...module.default)).catch(error => { loadingLanguages.delete(language); throw error }))
  }
  await loadingLanguages.get(language)
  if (!isCurrent()) return ''
  return highlighter.codeToHtml(source, { lang: language, themes: { light: 'github-light', dark: 'github-dark' }, defaultColor: false })
}
</script>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { Check, Copy } from '@lucide/vue'
import { useI18n } from 'vue-i18n'
import { useClipboard } from '@/composables/useClipboard'
import creationAPI from '../api'
import TokenStats from './TokenStats.vue'
import type { CreationMessageRole } from '../types'

const props = defineProps<{
  role: CreationMessageRole | 'assistant' | 'user' | 'system'
  content: unknown
  streaming?: boolean
  inputTokens?: number | null
  outputTokens?: number | null
}>()

const text = computed(() => creationAPI.extractMessageText(props.content))
const { locale } = useI18n()
const { copyToClipboard } = useClipboard()
const html = ref('')
const markdownRoot = ref<HTMLElement | null>(null)
const blocks = shallowRef<Array<{ target: HTMLElement; source: string; language: string; id: number }>>([])
const copiedBlock = ref<number | null>(null)
const labels = computed(() => locale.value.startsWith('zh') ? { copy: '复制代码', copied: '已复制', plain: '纯文本' } : { copy: 'Copy code', copied: 'Copied', plain: 'Plain text' })
let revision = 0
let copiedTimer: ReturnType<typeof setTimeout> | undefined

watch([text, () => props.role], async () => {
  const current = ++revision
  blocks.value = []
  copiedBlock.value = null
  clearTimeout(copiedTimer)
  if (props.role !== 'assistant') { html.value = ''; return }
  const raw = marked.parse(text.value || '', { breaks: true }) as string
  // Untrusted Markdown cannot provide styles or interactive controls. Shiki's trusted styles are sanitized separately.
  html.value = DOMPurify.sanitize(raw, { FORBID_TAGS: ['style', 'form', 'button', 'input', 'textarea', 'select'], FORBID_ATTR: ['style'] })
  await nextTick()
  if (current !== revision || !markdownRoot.value) return
  for (const link of markdownRoot.value.querySelectorAll('a[href]')) {
    if (/^https?:\/\//i.test(link.getAttribute('href') || '')) {
      link.setAttribute('target', '_blank')
      link.setAttribute('rel', 'noopener noreferrer')
    }
  }
  for (const [id, code] of [...markdownRoot.value.querySelectorAll('pre > code')].entries()) {
    const pre = code.parentElement!
    const source = code.textContent || ''
    const declared = [...code.classList].find(name => name.startsWith('language-'))?.slice(9).toLowerCase() || ''
    const language = aliases[declared] || declared
    const wrapper = document.createElement('div')
    wrapper.className = 'studio-code-block'
    const header = document.createElement('div')
    header.className = 'studio-code-header'
    pre.replaceWith(wrapper)
    wrapper.append(header, pre)
    blocks.value = [...blocks.value, { target: header, source, language: declared, id }]
    // Large or unfinished streams remain immediately readable; stale highlighting never overwrites newer content.
    if (!Object.prototype.hasOwnProperty.call(languages, language) || source.length > 50000) continue
    void highlightCode(source, language as keyof typeof languages, () => current === revision).then(result => {
      if (current !== revision || !markdownRoot.value?.contains(wrapper)) return
      const parsed = document.createElement('template')
      parsed.innerHTML = DOMPurify.sanitize(result, { ALLOWED_TAGS: ['pre', 'code', 'span'], ALLOWED_ATTR: ['class', 'style', 'tabindex'] })
      const highlighted = parsed.content.firstElementChild
      if (highlighted?.tagName === 'PRE') pre.replaceWith(highlighted)
    }).catch(() => { /* The escaped, copyable plain code remains available if a grammar cannot load. */ })
  }
}, { immediate: true })

async function copyBlock(block: { id: number; source: string }) {
  const current = revision
  if (await copyToClipboard(block.source) && current === revision) {
    copiedBlock.value = block.id
    clearTimeout(copiedTimer)
    copiedTimer = setTimeout(() => { copiedBlock.value = null }, 2000)
  }
}

onBeforeUnmount(() => {
  revision += 1
  clearTimeout(copiedTimer)
})
</script>

<template>
  <div class="studio-message-content">
    <div v-if="role === 'assistant'" ref="markdownRoot" class="studio-markdown text-foreground" v-html="html" />
    <p v-else class="studio-message-text text-foreground">{{ text }}</p>
    <Teleport v-for="block in blocks" :key="block.id" :to="block.target">
      <span>{{ block.language || labels.plain }}</span>
      <button type="button" class="studio-code-copy" :title="copiedBlock === block.id ? labels.copied : labels.copy" :aria-label="copiedBlock === block.id ? labels.copied : labels.copy" @click="copyBlock(block)"><Check v-if="copiedBlock === block.id" :size="14" /><Copy v-else :size="14" /></button>
    </Teleport>
    <span v-if="streaming" class="studio-cursor" aria-hidden="true">▍</span>
    <TokenStats
      v-if="role === 'assistant' && !streaming"
      :input-tokens="inputTokens"
      :output-tokens="outputTokens"
    />
  </div>
</template>

<style scoped>
.studio-message-content {
  min-width: 0;
  font-size: 13px;
  line-height: 1.6;
}

.studio-message-text {
  margin: 0;
  white-space: pre-wrap;
}

.studio-markdown :deep(p) {
  margin: 0 0 8px;
}

.studio-markdown :deep(p:last-child) {
  margin-bottom: 0;
}

.studio-markdown :deep(pre) {
  background: var(--code-bg);
  border-radius: 0;
  margin: 0;
  padding: 10px 12px;
  overflow: auto;
  font-family: var(--font-mono);
  font-size: 12px;
}

.studio-markdown :deep(.studio-code-block) { min-width: 0; max-width: 100%; margin: 10px 0; overflow: hidden; border: 1px solid var(--border); border-radius: 8px; }
.studio-markdown :deep(.studio-code-header) { display: flex; justify-content: space-between; align-items: center; min-height: 34px; padding: 3px 10px; background: var(--surface-secondary); color: var(--muted); font-size: 11px; }
.studio-code-copy { display: grid; place-items: center; width: 28px; height: 28px; padding: 0; border: 0; border-radius: 4px; background: transparent; color: var(--muted); }
.studio-code-copy:hover { background: var(--surface); color: var(--foreground); }
.studio-markdown :deep(.shiki), .studio-markdown :deep(.shiki span) { color: var(--shiki-light); background-color: var(--shiki-light-bg); }
:global(html[data-theme='glass-dark'] .studio-markdown .shiki), :global(html[data-theme='glass-dark'] .studio-markdown .shiki span) { color: var(--shiki-dark); background-color: var(--shiki-dark-bg); }
.studio-markdown :deep(a) { color: var(--accent); overflow-wrap: anywhere; }
.studio-markdown :deep(img) { max-width: 100%; height: auto; }
.studio-markdown :deep(table) { display: block; max-width: 100%; overflow: auto; }
.studio-markdown :deep(th), .studio-markdown :deep(td) { border: 1px solid var(--border); padding: 6px 10px; }

.studio-markdown :deep(code) {
  font-family: var(--font-mono);
  font-size: 12px;
}

.studio-cursor {
  display: inline-block;
  margin-left: 2px;
  animation: studio-cursor-pulse 1s step-end infinite;
}

@keyframes studio-cursor-pulse {
  50% {
    opacity: 0;
  }
}
</style>
