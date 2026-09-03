<template>
  <div class="code-tabs glass-card-flat">
    <div class="code-tabs-bar">
      <button
        v-for="tab in tabs"
        :key="tab.key"
        type="button"
        class="code-tab"
        :class="{ 'is-active': tab.key === active }"
        @click="active = tab.key"
      >
        {{ tab.label }}
      </button>
      <button type="button" class="code-tabs-copy" @click="copy">
        <svg v-if="!copied" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M8 8h12v12H8zM16 8V4H4v12h4" /></svg>
        <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
        {{ copied ? t('home.steps.copied') : t('home.steps.copy') }}
      </button>
    </div>
    <div class="code-tabs-body">
      <div v-for="(ln, i) in lines" :key="i" class="code-line" :class="ln.kind">
        <span v-for="(seg, j) in ln.segs" :key="j" :class="seg.cls">{{ seg.text }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useClipboard } from '@/composables/useClipboard'

const props = defineProps<{ apiBaseUrl: string }>()
const { t } = useI18n()
const { copyToClipboard } = useClipboard()

type Seg = { text: string; cls?: string }
type Line = { kind?: 'comment' | 'blank'; segs: Seg[] }

const active = ref<'claude' | 'codex' | 'openai' | 'curl'>('claude')
const copied = ref(false)

const tabs = [
  { key: 'claude' as const, label: 'Claude Code' },
  { key: 'codex' as const, label: 'Codex CLI' },
  { key: 'openai' as const, label: 'OpenAI SDK' },
  { key: 'curl' as const, label: 'cURL' }
]

const base = computed(() => props.apiBaseUrl.replace(/\/+$/, '') || 'https://api.example.com')
const kw = (text: string): Seg => ({ text, cls: 'code-kw' })
const str = (text: string): Seg => ({ text, cls: 'code-str' })
const plain = (text: string): Seg => ({ text })
const comment = (text: string): Line => ({ kind: 'comment', segs: [{ text }] })
const blank: Line = { kind: 'blank', segs: [{ text: ' ' }] }

const snippets = computed<Record<typeof active.value, Line[]>>(() => ({
  claude: [
    comment(t('home.steps.comment.shell')),
    { segs: [kw('export'), plain(' ANTHROPIC_BASE_URL='), str(`"${base.value}"`)] },
    { segs: [kw('export'), plain(' ANTHROPIC_AUTH_TOKEN='), str('"sk-s2a-••••••••••••••••3f2a"')] },
    { segs: [kw('export'), plain(' ANTHROPIC_MODEL='), str('"claude-sonnet-4-5"')] },
    blank,
    comment(t('home.steps.comment.start')),
    { segs: [plain('claude')] }
  ],
  codex: [
    comment(t('home.steps.comment.codex')),
    { segs: [plain('model_provider = '), str('"sub2api"')] },
    { segs: [plain('model = '), str('"gpt-5-codex"')] },
    blank,
    { segs: [kw('[model_providers.sub2api]')] },
    { segs: [plain('name = '), str('"Sub2API"')] },
    { segs: [plain('base_url = '), str(`"${base.value}/v1"`)] },
    { segs: [plain('env_key = '), str('"SUB2API_KEY"')] }
  ],
  openai: [
    { segs: [kw('from'), plain(' openai '), kw('import'), plain(' OpenAI')] },
    blank,
    { segs: [plain('client = OpenAI(')] },
    { segs: [plain('    base_url='), str(`"${base.value}/v1"`), plain(',')] },
    { segs: [plain('    api_key='), str('"sk-s2a-••••••••3f2a"')] },
    { segs: [plain(')')] },
    { segs: [plain('r = client.chat.completions.create(model='), str('"gpt-5"'), plain(', messages=[...])')] }
  ],
  curl: [
    { segs: [plain('curl '), str(`${base.value}/v1/messages`), plain(' \\')] },
    { segs: [plain('  -H '), str('"x-api-key: sk-s2a-••••••••3f2a"'), plain(' \\')] },
    { segs: [plain('  -H '), str('"anthropic-version: 2023-06-01"'), plain(' \\')] },
    { segs: [plain('  -H '), str('"content-type: application/json"'), plain(' \\')] },
    { segs: [plain('  -d '), str(`'{"model":"claude-sonnet-4-5","max_tokens":1024,"messages":[{"role":"user","content":"Hi"}]}'`)] }
  ]
}))

const lines = computed(() => snippets.value[active.value])

async function copy() {
  const text = lines.value.map((l) => l.segs.map((s) => s.text).join('')).join('\n')
  await copyToClipboard(text)
  copied.value = true
  setTimeout(() => (copied.value = false), 1600)
}
</script>

<style scoped>
.code-tabs {
  position: relative;
  border-radius: 16px;
  overflow: hidden;
  box-shadow: var(--shadow);
  color: var(--foreground);
}

.code-tabs-bar {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
  font-size: 12px;
  overflow-x: auto;
  scrollbar-width: none;
}

.code-tabs-bar::-webkit-scrollbar {
  display: none;
}

.code-tab {
  padding: 5px 10px;
  border-radius: 7px;
  border: 0;
  background: transparent;
  color: var(--muted);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  white-space: nowrap;
  transition: background 0.15s ease, color 0.15s ease;
}

.code-tab.is-active {
  background: var(--surface-secondary);
  color: var(--foreground);
  font-weight: 600;
  box-shadow: inset 0 0 0 1px var(--border);
}

.code-tabs-copy {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  border: 0;
  background: transparent;
  color: var(--muted);
  font-size: 12px;
  cursor: pointer;
  white-space: nowrap;
  padding: 4px 6px;
  border-radius: 6px;
}

.code-tabs-copy:hover {
  color: var(--foreground);
  background: color-mix(in oklch, var(--foreground) 6%, transparent);
}

.code-tabs-body {
  margin: 0;
  padding: 20px 22px;
  font-family: var(--font-mono);
  font-size: 13px;
  line-height: 1.75;
  overflow-x: auto;
}

.code-line {
  display: block;
  white-space: pre;
  min-height: 1.75em;
}

.code-line.comment {
  color: var(--muted);
}

.code-kw {
  color: var(--accent);
  font-weight: 500;
}

.code-str {
  color: var(--success-text);
}

@media (max-width: 640px) {
  .code-tabs-body {
    padding: 16px;
    font-size: 12px;
  }
}
</style>
