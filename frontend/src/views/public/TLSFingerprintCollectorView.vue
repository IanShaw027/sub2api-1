<template>
  <div class="min-h-screen bg-slate-950 text-slate-100">
    <main class="mx-auto flex min-h-screen max-w-5xl flex-col px-4 py-8 sm:px-6 lg:px-8">
      <section class="rounded-3xl border border-cyan-300/20 bg-slate-900/80 p-6 shadow-2xl shadow-cyan-950/40">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <p class="text-xs font-semibold uppercase tracking-[0.28em] text-cyan-300">
              {{ t('tlsCollector.eyebrow') }}
            </p>
            <h1 class="mt-3 text-3xl font-black tracking-tight text-white sm:text-4xl">
              {{ t('tlsCollector.title') }}
            </h1>
            <p class="mt-3 max-w-3xl text-sm leading-6 text-slate-300">
              {{ t('tlsCollector.description') }}
            </p>
          </div>
          <RouterLink
            to="/home"
            class="rounded-full border border-white/10 px-4 py-2 text-sm font-semibold text-slate-200 transition hover:border-cyan-300/60 hover:text-cyan-200"
          >
            {{ t('tlsCollector.backHome') }}
          </RouterLink>
        </div>

        <div class="mt-6 grid gap-4 lg:grid-cols-3">
          <label class="block">
            <span class="mb-1 block text-xs font-semibold uppercase tracking-[0.16em] text-slate-400">
              {{ t('tlsCollector.endpoint') }}
            </span>
            <input v-model="form.endpoint" class="collector-input" />
          </label>
          <label class="block">
            <span class="mb-1 block text-xs font-semibold uppercase tracking-[0.16em] text-slate-400">
              {{ t('tlsCollector.platform') }}
            </span>
            <input v-model="form.platform" class="collector-input" placeholder="openai" />
          </label>
          <label class="block">
            <span class="mb-1 block text-xs font-semibold uppercase tracking-[0.16em] text-slate-400">
              {{ t('tlsCollector.token') }}
            </span>
            <input v-model="form.token" class="collector-input font-mono" />
          </label>
        </div>

        <label class="mt-4 block">
          <span class="mb-1 block text-xs font-semibold uppercase tracking-[0.16em] text-slate-400">
            {{ t('tlsCollector.userAgent') }}
          </span>
          <input v-model="form.userAgent" class="collector-input font-mono" />
        </label>

        <label class="mt-4 block">
          <span class="mb-1 block text-xs font-semibold uppercase tracking-[0.16em] text-slate-400">
            {{ t('tlsCollector.payload') }}
          </span>
          <textarea
            v-model="form.payload"
            rows="12"
            class="collector-input font-mono text-xs leading-5"
            :placeholder="t('tlsCollector.payloadPlaceholder')"
          />
        </label>

        <div class="mt-5 flex flex-wrap items-center gap-3">
          <button
            type="button"
            class="rounded-full bg-cyan-300 px-5 py-2 text-sm font-black text-slate-950 transition hover:bg-cyan-200 disabled:cursor-not-allowed disabled:opacity-60"
            :disabled="submitting"
            @click="submitCapture"
          >
            {{ submitting ? t('tlsCollector.submitting') : t('tlsCollector.submit') }}
          </button>
          <button
            type="button"
            class="rounded-full border border-white/10 px-5 py-2 text-sm font-semibold text-slate-200 transition hover:border-cyan-300/60 hover:text-cyan-200"
            @click="copySubmitJSON"
          >
            {{ t('tlsCollector.copyJSON') }}
          </button>
        </div>

        <div
          v-if="message"
          :class="[
            'mt-5 rounded-2xl border px-4 py-3 text-sm',
            messageKind === 'success'
              ? 'border-emerald-300/30 bg-emerald-950/40 text-emerald-100'
              : 'border-red-300/30 bg-red-950/40 text-red-100'
          ]"
        >
          {{ message }}
        </div>
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'

const route = useRoute()
const { t } = useI18n()

const defaultEndpoint = () => {
  const raw = String(route.query.endpoint || '')
  if (raw) return raw
  const base = typeof window === 'undefined' ? '' : window.location.origin
  return `${base}/api/v1/tls-fingerprint-captures/submit`
}

const form = reactive({
  endpoint: defaultEndpoint(),
  token: String(route.query.token || ''),
  platform: String(route.query.platform || 'openai'),
  userAgent: String(route.query.user_agent || ''),
  payload: ''
})

const submitting = ref(false)
const message = ref('')
const messageKind = ref<'success' | 'error'>('success')

const setMessage = (kind: 'success' | 'error', text: string) => {
  messageKind.value = kind
  message.value = text
}

const submitCapture = async () => {
  if (!form.token.trim() || !form.platform.trim() || !form.payload.trim()) {
    setMessage('error', t('tlsCollector.required'))
    return
  }
  submitting.value = true
  try {
    const result = await adminAPI.tlsFingerprintProfiles.submitCaptureToEndpoint(form.endpoint.trim(), {
      token: form.token.trim(),
      platform: form.platform.trim(),
      user_agent: form.userAgent.trim() || navigator.userAgent,
      payload: form.payload.trim()
    })
    if (!result.accepted) {
      setMessage('error', t(`tlsCollector.ignored.${result.ignored_reason || 'unknown'}`))
      return
    }
    const counts = Object.entries(result.counts || {})
      .map(([platform, count]) => `${platform}: ${count}`)
      .join(', ')
    setMessage('success', result.duplicate
      ? t('tlsCollector.duplicate', { counts })
      : t('tlsCollector.accepted', { counts }))
  } catch (error: any) {
    setMessage('error', error?.message || t('tlsCollector.failed'))
  } finally {
    submitting.value = false
  }
}

const copySubmitJSON = async () => {
  const text = JSON.stringify({
    endpoint: form.endpoint,
    token: form.token,
    platform: form.platform,
    user_agent: form.userAgent || navigator.userAgent,
    payload: form.payload
  }, null, 2)
  try {
    await navigator.clipboard.writeText(text)
    setMessage('success', t('tlsCollector.copied'))
  } catch {
    setMessage('error', t('tlsCollector.copyFailed'))
  }
}
</script>

<style scoped>
.collector-input {
  @apply w-full rounded-2xl border border-white/10 bg-slate-950/70 px-4 py-3 text-sm text-white outline-none transition placeholder:text-slate-500 focus:border-cyan-300/70 focus:ring-2 focus:ring-cyan-300/20;
}
</style>
