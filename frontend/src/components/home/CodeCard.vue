<template>
  <div
    class="code-card overflow-hidden rounded-hero border border-white/10 bg-ink shadow-xl shadow-ink/20 dark:border-line dark:bg-[#0b1526]"
    data-testid="home-code-card"
  >
    <!-- Header: language tabs + copy -->
    <div
      class="flex items-center justify-between gap-2 border-b border-white/10 px-4 py-2.5"
    >
      <div
        class="flex items-center gap-1"
        role="tablist"
        :aria-label="t('home.codeCard.languages')"
        @keydown="onTablistKeydown"
      >
        <button
          v-for="tab in tabs"
          :key="tab.id"
          type="button"
          role="tab"
          :tabindex="activeTab === tab.id ? 0 : -1"
          :aria-selected="activeTab === tab.id"
          class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
          :class="
            activeTab === tab.id
              ? 'bg-white/10 text-white'
              : 'text-white/50 hover:bg-white/5 hover:text-white/80'
          "
          @click="activeTab = tab.id"
        >
          {{ tab.label }}
        </button>
      </div>
      <button
        type="button"
        class="inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-medium text-white/60 transition-colors hover:bg-white/10 hover:text-white"
        :title="copied ? t('home.codeCard.copied') : t('home.codeCard.copy')"
        @click="copyCode"
      >
        <Icon :name="copied ? 'check' : 'copy'" size="xs" />
        <span>{{ copied ? t('home.codeCard.copied') : t('home.codeCard.copy') }}</span>
      </button>
    </div>

    <!-- Code body -->
    <div class="relative overflow-x-auto px-4 py-4">
      <pre
        class="m-0 whitespace-pre font-mono text-[12px] leading-relaxed text-white/85 sm:text-[13px]"
      ><code>{{ activeSnippet }}</code></pre>
    </div>

    <!-- Footer hint -->
    <div
      class="flex items-center gap-2 border-t border-white/10 px-4 py-2 text-[11px] text-white/40"
    >
      <span
        class="inline-flex items-center rounded bg-brand/20 px-1.5 py-0.5 font-medium text-brand-cyan"
      >OpenAI-compatible</span>
      <span class="truncate">{{ t('home.codeCard.hint') }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(
  defineProps<{
    /** API base URL from public settings; falls back to placeholder */
    apiBaseUrl?: string
  }>(),
  {
    apiBaseUrl: ''
  }
)

const { t } = useI18n()

type TabId = 'curl' | 'python' | 'js'

const tabs: { id: TabId; label: string }[] = [
  { id: 'curl', label: 'cURL' },
  { id: 'python', label: 'Python' },
  { id: 'js', label: 'JavaScript' }
]

const activeTab = ref<TabId>('curl')
const copied = ref(false)
let copyTimer: ReturnType<typeof setTimeout> | null = null

const baseUrl = computed(() => {
  const raw = (props.apiBaseUrl || '').trim().replace(/\/+$/, '')
  return raw || 'https://api.example.com'
})

const modelId = 'your-model-id'
/** Generic env var name — avoids hardcoding brand in white-label installs */
const envKeyName = 'API_KEY'

const snippets = computed(() => {
  const base = baseUrl.value
  const curl = `# OpenAI-compatible
curl ${base}/v1/chat/completions \\
  -H "Authorization: Bearer $${envKeyName}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "${modelId}",
    "messages": [{"role": "user", "content": "Hello"}]
  }'`

  const python = `# OpenAI-compatible (openai SDK)
from openai import OpenAI

client = OpenAI(
    api_key="YOUR_${envKeyName}",
    base_url="${base}/v1",
)

resp = client.chat.completions.create(
    model="${modelId}",
    messages=[{"role": "user", "content": "Hello"}],
)
print(resp.choices[0].message.content)`

  const js = `// OpenAI-compatible (openai SDK)
import OpenAI from "openai";

const client = new OpenAI({
  apiKey: process.env.${envKeyName},
  baseURL: "${base}/v1",
});

const resp = await client.chat.completions.create({
  model: "${modelId}",
  messages: [{ role: "user", content: "Hello" }],
});
console.log(resp.choices[0].message.content);`

  return { curl, python, js } as const
})

const activeSnippet = computed(() => snippets.value[activeTab.value])

function onTablistKeydown(event: KeyboardEvent) {
  const key = event.key
  if (key !== 'ArrowLeft' && key !== 'ArrowRight' && key !== 'Home' && key !== 'End') {
    return
  }
  event.preventDefault()
  const ids = tabs.map((tab) => tab.id)
  const current = ids.indexOf(activeTab.value)
  let next = current
  if (key === 'ArrowRight') next = (current + 1) % ids.length
  else if (key === 'ArrowLeft') next = (current - 1 + ids.length) % ids.length
  else if (key === 'Home') next = 0
  else if (key === 'End') next = ids.length - 1
  activeTab.value = ids[next]

  const target = event.currentTarget
  if (!(target instanceof HTMLElement)) return
  const buttons = target.querySelectorAll<HTMLElement>('[role="tab"]')
  buttons[next]?.focus()
}

async function copyCode() {
  const text = activeSnippet.value
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    // Fallback for restricted clipboard environments
    const ta = document.createElement('textarea')
    ta.value = text
    ta.setAttribute('readonly', '')
    ta.style.position = 'fixed'
    ta.style.left = '-9999px'
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    document.body.removeChild(ta)
  }
  copied.value = true
  if (copyTimer) clearTimeout(copyTimer)
  copyTimer = setTimeout(() => {
    copied.value = false
    copyTimer = null
  }, 1800)
}

onBeforeUnmount(() => {
  if (copyTimer) {
    clearTimeout(copyTimer)
    copyTimer = null
  }
})
</script>
