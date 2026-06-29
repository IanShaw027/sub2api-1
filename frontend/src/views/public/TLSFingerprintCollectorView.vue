<template>
  <div class="min-h-screen overflow-hidden bg-[#09110f] text-stone-100">
    <div class="pointer-events-none fixed inset-0">
      <div class="absolute left-1/2 top-[-18rem] h-[36rem] w-[36rem] -translate-x-1/2 rounded-full bg-emerald-400/20 blur-3xl" />
      <div class="absolute bottom-[-14rem] right-[-8rem] h-[32rem] w-[32rem] rounded-full bg-amber-300/10 blur-3xl" />
      <div class="absolute inset-0 bg-[radial-gradient(circle_at_20%_20%,rgba(255,255,255,0.08)_0,transparent_28%),linear-gradient(135deg,rgba(20,83,45,0.18),transparent_38%)]" />
    </div>

    <main class="relative mx-auto flex min-h-screen max-w-6xl flex-col px-4 py-8 sm:px-6 lg:px-8">
      <section class="rounded-[2rem] border border-emerald-200/15 bg-stone-950/75 p-6 shadow-2xl shadow-emerald-950/40 backdrop-blur md:p-8">
        <div class="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <p class="text-xs font-black uppercase tracking-[0.3em] text-emerald-300">
              {{ t('tlsCollector.eyebrow') }}
            </p>
            <h1 class="mt-3 max-w-3xl text-3xl font-black tracking-tight text-white sm:text-5xl">
              {{ t('tlsCollector.title') }}
            </h1>
            <p class="mt-4 max-w-3xl text-sm leading-6 text-stone-300">
              {{ t('tlsCollector.description') }}
            </p>
          </div>
          <RouterLink
            to="/home"
            class="rounded-full border border-white/10 px-4 py-2 text-sm font-semibold text-stone-200 transition hover:border-emerald-300/60 hover:text-emerald-200"
          >
            {{ t('tlsCollector.backHome') }}
          </RouterLink>
        </div>

        <div class="mt-7 grid gap-3 lg:grid-cols-3">
          <label class="collector-card">
            <span class="collector-label">{{ t('tlsCollector.captureURL') }}</span>
            <input v-model="form.captureURL" class="collector-input" />
          </label>
          <label class="collector-card">
            <span class="collector-label">{{ t('tlsCollector.platform') }}</span>
            <select v-model="form.platform" class="collector-input">
              <option
                v-if="normalizedPlatform && !selectedPlatformListed"
                :value="normalizedPlatform"
              >
                {{ normalizedPlatform }}
              </option>
              <option v-for="option in platformOptions" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
          </label>
          <label class="collector-card">
            <span class="collector-label">{{ t('tlsCollector.token') }}</span>
            <input v-model="form.token" class="collector-input font-mono" />
          </label>
        </div>

        <div class="mt-6 grid gap-3 lg:grid-cols-2">
          <ConfigTile :label="t('tlsCollector.platformBaseURL')" :value="platformBaseURL" :copy-label="t('common.copy')" @copy="copyText(platformBaseURL)" />
          <ConfigTile :label="t('tlsCollector.probeURL')" :value="probeURL" :copy-label="t('common.copy')" @copy="copyText(probeURL)" />
        </div>

        <div class="mt-6 rounded-2xl border border-amber-200/20 bg-amber-950/20 p-4 text-sm leading-6 text-amber-50">
          <p class="font-bold">{{ t('tlsCollector.howItWorksTitle') }}</p>
          <p class="mt-1 text-amber-100/85">{{ t('tlsCollector.howItWorksBody') }}</p>
        </div>
      </section>

      <section class="relative mt-5 grid gap-5 lg:grid-cols-[0.9fr_1.1fr]">
        <div class="space-y-5">
          <section class="rounded-[1.75rem] border border-white/10 bg-stone-950/70 p-5 backdrop-blur">
            <h2 class="text-lg font-black text-white">{{ t('tlsCollector.requiredTitle') }}</h2>
            <div class="mt-4 space-y-3">
              <CheckItem :text="t('tlsCollector.required.token')" />
              <CheckItem :text="t('tlsCollector.required.url')" />
              <CheckItem :text="t('tlsCollector.required.request')" />
              <CheckItem :text="t('tlsCollector.required.headers')" />
              <CheckItem :text="t('tlsCollector.required.cert')" />
            </div>
          </section>

          <section class="rounded-[1.75rem] border border-white/10 bg-stone-950/70 p-5 backdrop-blur">
            <h2 class="text-lg font-black text-white">{{ t('tlsCollector.capturesTitle') }}</h2>
            <div class="mt-4 grid gap-2 text-sm text-stone-300">
              <div class="rounded-xl bg-white/5 px-3 py-2">{{ t('tlsCollector.captures.rawClientHello') }}</div>
              <div class="rounded-xl bg-white/5 px-3 py-2">{{ t('tlsCollector.captures.userAgent') }}</div>
              <div class="rounded-xl bg-white/5 px-3 py-2">{{ t('tlsCollector.captures.originator') }}</div>
              <div class="rounded-xl bg-white/5 px-3 py-2">{{ t('tlsCollector.captures.platform') }}</div>
            </div>
          </section>

          <section class="rounded-[1.75rem] border border-white/10 bg-stone-950/70 p-5 backdrop-blur">
            <h2 class="text-lg font-black text-white">{{ t('tlsCollector.supportedTransportsTitle') }}</h2>
            <p class="mt-1 text-sm text-stone-400">{{ t('tlsCollector.supportedTransportsHint') }}</p>
            <div class="mt-4 grid gap-2 text-sm text-stone-300">
              <div class="rounded-xl bg-white/5 px-3 py-2">{{ t('tlsCollector.supportedTransports.http1') }}</div>
              <div class="rounded-xl bg-white/5 px-3 py-2">{{ t('tlsCollector.supportedTransports.h2') }}</div>
              <div class="rounded-xl bg-white/5 px-3 py-2">{{ t('tlsCollector.supportedTransports.websocketHttp1') }}</div>
              <div class="rounded-xl bg-white/5 px-3 py-2">{{ t('tlsCollector.supportedTransports.websocketH2') }}</div>
            </div>
          </section>

          <section class="rounded-[1.75rem] border border-white/10 bg-stone-950/70 p-5 backdrop-blur">
            <h2 class="text-lg font-black text-white">{{ t('tlsCollector.successBehaviorTitle') }}</h2>
            <p class="mt-1 text-sm text-stone-400">{{ t('tlsCollector.successBehaviorHint') }}</p>
            <div class="mt-4 grid gap-2 text-sm text-stone-300">
              <div class="rounded-xl bg-white/5 px-3 py-2">{{ t('tlsCollector.successBehavior.json') }}</div>
              <div class="rounded-xl bg-white/5 px-3 py-2">{{ t('tlsCollector.successBehavior.sse') }}</div>
              <div class="rounded-xl bg-white/5 px-3 py-2">{{ t('tlsCollector.successBehavior.filtered') }}</div>
            </div>
          </section>
        </div>

        <section class="rounded-[1.75rem] border border-white/10 bg-stone-950/70 p-5 backdrop-blur">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 class="text-lg font-black text-white">{{ t('tlsCollector.clientGuidesTitle') }}</h2>
              <p class="mt-1 text-sm text-stone-400">{{ t('tlsCollector.clientGuidesHint') }}</p>
            </div>
            <button type="button" class="collector-button" @click="copyText(allCommands)">
              {{ t('tlsCollector.copyAll') }}
            </button>
          </div>

          <div class="mt-5 space-y-4">
            <GuideBlock
              v-for="guide in platformGuides"
              :key="guide.key"
              :title="guide.title"
              :body="guide.body"
              :command="guide.command"
              :copy-label="t('common.copy')"
              :open-label="t('common.expand')"
              :collapse-label="t('common.collapse')"
              @copy="copyText(guide.command)"
            />
          </div>
        </section>
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, reactive, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useClipboard } from '@/composables/useClipboard'

const route = useRoute()
const { t } = useI18n()
const { copyToClipboard } = useClipboard()

const inferCaptureURL = () => {
	const queryURL = String(route.query.capture_url || route.query.endpoint || '').trim()
	const fallback = defaultCaptureURL()
	if (!queryURL) return fallback
	const safeURL = safeQueryCaptureURL(queryURL)
	return safeURL || fallback
}

const form = reactive({
  captureURL: inferCaptureURL(),
  token: String(route.query.token || ''),
  platform: String(route.query.platform || 'openai')
})

const normalizedCaptureURL = computed(() => trimTrailingSlash(form.captureURL) || 'https://localhost:8444/capture')
const normalizedPlatform = computed(() => form.platform.trim() || 'openai')
const platformOptions = [
  { value: 'openai', label: 'OpenAI / Codex' },
  { value: 'anthropic', label: 'Anthropic / Claude' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'kiro', label: 'Kiro' },
  { value: 'grok', label: 'Grok / xAI' },
  { value: 'antigravity', label: 'Antigravity' },
  { value: 'custom', label: 'Custom' }
]
const selectedPlatformListed = computed(() => platformOptions.some(option => option.value === normalizedPlatform.value))
const platformBaseURL = computed(() => platformURL(normalizedPlatform.value))
const probeURL = computed(() => `${platformBaseURL.value}/responses`)
const openAIPlatformBaseURL = computed(() => platformURL(platformOrDefault('openai')))
const anthropicPlatformBaseURL = computed(() => platformURL(platformOrDefault('anthropic')))

const shellToken = computed(() => shellQuote(form.token.trim() || '<capture-token>'))
const shellProbeURL = computed(() => shellQuote(probeURL.value))
const shellAnthropicPlatformBaseURL = computed(() => shellQuote(anthropicPlatformBaseURL.value))
const shellAuthorizationHeader = computed(() => shellQuote(`Authorization: Bearer ${form.token.trim() || '<capture-token>'}`))
const jsPlatformBaseURL = computed(() => jsString(openAIPlatformBaseURL.value))
const jsToken = computed(() => jsString(form.token.trim() || '<capture-token>'))
const pyPlatformBaseURL = computed(() => pyString(openAIPlatformBaseURL.value))
const pyToken = computed(() => pyString(form.token.trim() || '<capture-token>'))

const guides = computed(() => [
  {
    key: 'codex-cli',
    title: t('tlsCollector.guides.codexCli.title'),
    body: t('tlsCollector.guides.codexCli.body'),
    command: [
      `export SUB2API_TLS_CAPTURE_TOKEN=${shellToken.value}`,
      'codex --config model_provider=\'"sub2api_capture"\' \\',
      '  --config \'model_providers.sub2api_capture.name="sub2api TLS capture"\' \\',
      `  --config 'model_providers.sub2api_capture.base_url=${tomlString(openAIPlatformBaseURL.value)}' \\`,
      '  --config \'model_providers.sub2api_capture.wire_api="responses"\' \\',
      '  --config \'model_providers.sub2api_capture.env_key="SUB2API_TLS_CAPTURE_TOKEN"\' \\',
      '  "send one minimal request for TLS fingerprint capture"'
    ].join('\n')
  },
  {
    key: 'codex-exec',
    title: t('tlsCollector.guides.codexExec.title'),
    body: t('tlsCollector.guides.codexExec.body'),
    command: [
      `SUB2API_TLS_CAPTURE_TOKEN=${shellToken.value} codex exec \\`,
      '  --config model_provider=\'"sub2api_capture"\' \\',
      '  --config \'model_providers.sub2api_capture.name="sub2api TLS capture"\' \\',
      `  --config 'model_providers.sub2api_capture.base_url=${tomlString(openAIPlatformBaseURL.value)}' \\`,
      '  --config \'model_providers.sub2api_capture.wire_api="responses"\' \\',
      '  --config \'model_providers.sub2api_capture.env_key="SUB2API_TLS_CAPTURE_TOKEN"\' \\',
      '  "reply with one short sentence"'
    ].join('\n')
  },
  {
    key: 'codex-desktop',
    title: t('tlsCollector.guides.codexDesktop.title'),
    body: t('tlsCollector.guides.codexDesktop.body'),
    command: [
      '# Add this provider in Codex Desktop config, then start one short chat.',
      'model_provider = "sub2api_capture"',
      '',
      '[model_providers.sub2api_capture]',
      'name = "sub2api TLS capture"',
      `base_url = ${tomlString(openAIPlatformBaseURL.value)}`,
      'wire_api = "responses"',
      'env_key = "SUB2API_TLS_CAPTURE_TOKEN"',
      '',
      `# Shell environment before launching Desktop:`,
      `export SUB2API_TLS_CAPTURE_TOKEN=${shellToken.value}`
    ].join('\n')
  },
  {
    key: 'claude-code',
    title: t('tlsCollector.guides.claudeCode.title'),
    body: t('tlsCollector.guides.claudeCode.body'),
    command: [
      `ANTHROPIC_BASE_URL=${shellAnthropicPlatformBaseURL.value} \\`,
      `ANTHROPIC_AUTH_TOKEN=${shellToken.value} \\`,
      'claude "reply with one short sentence"'
    ].join('\n')
  },
  {
    key: 'claude-print',
    title: t('tlsCollector.guides.claudePrint.title'),
    body: t('tlsCollector.guides.claudePrint.body'),
    command: [
      `ANTHROPIC_BASE_URL=${shellAnthropicPlatformBaseURL.value} \\`,
      `ANTHROPIC_AUTH_TOKEN=${shellToken.value} \\`,
      'claude -p "reply with one short sentence"'
    ].join('\n')
  },
  {
    key: 'node',
    title: t('tlsCollector.guides.node.title'),
    body: t('tlsCollector.guides.node.body'),
    command: [
      'node --input-type=module <<\'EOF\'',
      'import OpenAI from "openai";',
      '',
      'const client = new OpenAI({',
      `  baseURL: ${jsPlatformBaseURL.value},`,
      `  apiKey: ${jsToken.value},`,
      '  defaultHeaders: { originator: "node_tls_capture" },',
      '});',
      '',
      'await client.responses.create({',
      '  model: "gpt-5",',
      '  input: "TLS fingerprint capture probe",',
      '});',
      'EOF'
    ].join('\n')
  },
  {
    key: 'python',
    title: t('tlsCollector.guides.python.title'),
    body: t('tlsCollector.guides.python.body'),
    command: [
      'python - <<\'PY\'',
      'from openai import OpenAI',
      '',
      'client = OpenAI(',
      `    base_url=${pyPlatformBaseURL.value},`,
      `    api_key=${pyToken.value},`,
      '    default_headers={"originator": "python_tls_capture"},',
      ')',
      '',
      'client.responses.create(',
      '    model="gpt-5",',
      '    input="TLS fingerprint capture probe",',
      ')',
      'PY'
    ].join('\n')
  },
	  {
	    key: 'curl',
	    title: t('tlsCollector.guides.curl.title'),
	    body: t('tlsCollector.guides.curl.body'),
	    command: [
	      `curl -v ${shellProbeURL.value} \\`,
	      `  -H ${shellAuthorizationHeader.value} \\`,
	      '  -H "Content-Type: application/json" \\',
	      '  -H "User-Agent: curl-tls-capture/1.0" \\',
	      '  -H "originator: curl_tls_capture" \\',
      '  -d \'{"model":"gpt-5","input":"TLS fingerprint capture probe"}\''
    ].join('\n')
  }
])

const allCommands = computed(() => platformGuides.value.map(guide => `# ${guide.title}\n${guide.command}`).join('\n\n'))

// 每个客户端采集指令归属的平台，使采集器页按所选平台展示契合的指令，不再混在一起。
const guidePlatform: Record<string, string> = {
  'codex-cli': 'openai',
  'codex-exec': 'openai',
  'codex-desktop': 'openai',
  'claude-code': 'anthropic',
  'claude-print': 'anthropic',
  'node': 'openai',
  'python': 'openai',
  'curl': 'openai'
}

const platformGuides = computed(() => {
  const platform = normalizedPlatform.value
  const matched = guides.value.filter(guide => guidePlatform[guide.key] === platform)
  // 未知平台（无专属指令）时回退展示通用 curl 指令，避免空白。
  if (matched.length === 0) {
    return guides.value.filter(guide => guide.key === 'curl')
  }
  return matched
})

const copyText = async (text: string) => {
  await copyToClipboard(text, t('tlsCollector.copied'))
}

function trimTrailingSlash(value: string): string {
	return value.trim().replace(/\/+$/, '')
}

function defaultCaptureURL(): string {
	if (typeof window === 'undefined') return 'https://localhost:8444/capture'
	return `https://${window.location.hostname}:8444/capture`
}

function safeQueryCaptureURL(value: string): string {
	if (typeof window === 'undefined') return ''
	try {
		const url = new URL(value)
		if (url.protocol !== 'https:' || url.hostname !== window.location.hostname) {
			return ''
		}
		url.hash = ''
		url.search = ''
		return trimTrailingSlash(url.toString())
	} catch {
		return ''
	}
}

function platformOrDefault(defaultPlatform: string): string {
	const platform = form.platform.trim()
	if (!platform || platform === 'openai') {
    return defaultPlatform
  }
  return platform
}

function platformURL(platform: string): string {
  return `${normalizedCaptureURL.value}/${encodeURIComponent(platform)}/v1`
}

function shellQuote(value: string): string {
  return `'${value.replace(/'/g, `'\\''`)}'`
}

function jsString(value: string): string {
  return JSON.stringify(value)
}

function pyString(value: string): string {
  return JSON.stringify(value)
}

function tomlString(value: string): string {
  return JSON.stringify(value)
}

const ConfigTile = defineComponent({
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
    copyLabel: { type: String, required: true }
  },
  emits: ['copy'],
  setup(props, { emit }) {
    return () => h('div', { class: 'rounded-2xl border border-white/10 bg-white/[0.04] p-4' }, [
      h('div', { class: 'mb-2 flex items-center justify-between gap-2' }, [
        h('span', { class: 'text-xs font-black uppercase tracking-[0.18em] text-stone-400' }, props.label),
        h('button', {
          type: 'button',
          class: 'rounded-full border border-white/10 px-3 py-1 text-xs font-bold text-stone-200 transition hover:border-emerald-300/60 hover:text-emerald-200',
          onClick: () => emit('copy')
        }, props.copyLabel)
      ]),
      h('code', { class: 'block break-all rounded-xl bg-black/35 px-3 py-2 text-xs text-emerald-100' }, props.value)
    ])
  }
})

const CheckItem = defineComponent({
  props: {
    text: { type: String, required: true }
  },
  setup(props) {
    return () => h('div', { class: 'flex gap-3 rounded-xl bg-white/[0.04] px-3 py-3 text-sm text-stone-300' }, [
      h('span', { class: 'mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-emerald-300 text-xs font-black text-stone-950' }, '✓'),
      h('span', props.text)
    ])
  }
})

const GuideBlock = defineComponent({
  props: {
    title: { type: String, required: true },
    body: { type: String, required: true },
    command: { type: String, required: true },
    copyLabel: { type: String, required: true },
    openLabel: { type: String, required: true },
    collapseLabel: { type: String, required: true }
  },
  emits: ['copy'],
  setup(props, { emit }) {
    const expanded = ref(false)

    return () => h('details', {
      class: 'group rounded-2xl border border-white/10 bg-white/[0.04] p-4',
      open: expanded.value,
      onToggle: (event: Event) => {
        expanded.value = (event.currentTarget as HTMLDetailsElement).open
      }
    }, [
      h('summary', { class: 'flex cursor-pointer list-none items-start justify-between gap-3' }, [
        h('span', [
          h('span', { class: 'block text-sm font-black text-white' }, props.title),
          h('span', { class: 'mt-1 block text-xs leading-5 text-stone-400' }, props.body)
        ]),
        h('span', { class: 'rounded-full border border-white/10 px-3 py-1 text-xs font-bold text-stone-300 group-open:bg-emerald-300 group-open:text-stone-950' }, expanded.value ? props.collapseLabel : props.openLabel)
      ]),
      h('div', { class: 'mt-4' }, [
        h('div', { class: 'mb-2 flex justify-end' }, [
          h('button', {
            type: 'button',
            class: 'collector-button',
            onClick: () => emit('copy')
          }, props.copyLabel)
        ]),
        h('pre', { class: 'max-h-80 overflow-auto rounded-2xl bg-black/55 p-4 text-xs leading-5 text-emerald-50' }, [
          h('code', props.command)
        ])
      ])
    ])
  }
})
</script>

<style scoped>
.collector-card {
  @apply block rounded-2xl border border-white/10 bg-white/[0.04] p-4;
}

.collector-label {
  @apply mb-2 block text-xs font-black uppercase tracking-[0.18em] text-stone-400;
}

.collector-input {
  @apply w-full rounded-xl border border-white/10 bg-black/35 px-3 py-2 text-sm text-white outline-none transition placeholder:text-stone-600 focus:border-emerald-300/70 focus:ring-2 focus:ring-emerald-300/20;
}

.collector-button {
  @apply rounded-full border border-white/10 px-3 py-1.5 text-xs font-bold text-stone-200 transition hover:border-emerald-300/60 hover:text-emerald-200;
}
</style>
