<template>
  <div v-if="sceneId && prefix" class="aliyun-captcha-wrapper">
    <div
      :id="elementId"
      class="aliyun-captcha-embed"
      :class="{ 'aliyun-captcha-embed--verified': state === 'verified' }"
    ></div>
    <p v-if="state === 'loading'" class="aliyun-captcha-status aliyun-captcha-status--muted">
      {{ t('auth.captchaLoading') }}
    </p>
    <p v-else-if="state === 'verified'" class="aliyun-captcha-status">
      {{ t('auth.captchaVerified') }}
    </p>
    <p v-else-if="state === 'error'" class="aliyun-captcha-status aliyun-captcha-status--error">
      {{ t('auth.captchaLoadFailed') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

interface AliyunCaptchaVerifyResult {
  captchaResult: boolean
  bizResult?: boolean
}

interface AliyunCaptchaInstance {
  refresh?: () => void
  destroy?: () => void
}

interface AliyunCaptchaInitOptions {
  SceneId: string
  prefix: string
  mode: 'popup' | 'embed'
  element: string
  captchaVerifyCallback: (
    captchaVerifyParam: string
  ) => AliyunCaptchaVerifyResult | Promise<AliyunCaptchaVerifyResult>
  onBizResultCallback: (bizResult: boolean) => void
  onFallback?: (error?: unknown) => void
  getInstance: (instance: AliyunCaptchaInstance) => void
  slideStyle?: { width: number; height: number }
  language?: string
  region?: string
  isShowErrorTip?: boolean
}

declare global {
  interface Window {
    initAliyunCaptcha?: (options: AliyunCaptchaInitOptions) => void | Promise<unknown>
    AliyunCaptchaConfig?: { region: string; prefix: string }
  }
}

const props = withDefaults(
  defineProps<{
    sceneId: string
    prefix: string
    region?: 'cn' | 'sgp'
  }>(),
  {
    region: 'cn'
  }
)

const emit = defineEmits<{
  (e: 'verify', param: string): void
  (e: 'expire'): void
  (e: 'error'): void
}>()

const { t, locale } = useI18n()

const uid = Math.random().toString(36).slice(2, 10)
const elementId = `aliyun-captcha-element-${uid}`

const state = ref<'loading' | 'idle' | 'verified' | 'error'>('loading')

const SCRIPT_SRC = 'https://o.alicdn.com/captcha-frontend/aliyunCaptcha/AliyunCaptcha.js'
const MAX_CAPTCHA_WIDTH = 360

let cachedParam: string | null = null
let pending: { resolve: (value: string | null) => void } | null = null
let readyPromise: Promise<void> | null = null
let captchaInstance: AliyunCaptchaInstance | null = null
let resizeObserver: ResizeObserver | null = null
let resizeScheduled = false
let initializedWidth = 0

const loadScript = (): Promise<void> => {
  return new Promise((resolve, reject) => {
    window.AliyunCaptchaConfig = { region: props.region, prefix: props.prefix }

    if (window.initAliyunCaptcha) {
      resolve()
      return
    }

    const existingScript = document.querySelector<HTMLScriptElement>(
      'script[src*="aliyunCaptcha/AliyunCaptcha"]'
    )
    if (existingScript) {
      existingScript.addEventListener('load', () => resolve())
      existingScript.addEventListener('error', () =>
        reject(new Error('Failed to load Aliyun captcha script'))
      )
      return
    }

    const script = document.createElement('script')
    script.src = SCRIPT_SRC
    script.async = true
    script.onload = () => resolve()
    script.onerror = () => reject(new Error('Failed to load Aliyun captcha script'))
    document.head.appendChild(script)
  })
}

function markError(): void {
  state.value = 'error'
  emit('error')
  settlePending(null)
}

function captchaWidth(): number {
  const element = document.getElementById(elementId)
  const width = Math.floor(
    element?.parentElement?.getBoundingClientRect().width ||
      element?.getBoundingClientRect().width ||
      0
  )
  return width > 0 ? Math.min(width, MAX_CAPTCHA_WIDTH) : MAX_CAPTCHA_WIDTH
}

function destroyCaptchaInstance(): void {
  try {
    captchaInstance?.destroy?.()
  } catch {
    // SDK 实例销毁失败不影响重建或卸载
  }
  captchaInstance = null
}

async function initCaptcha(): Promise<void> {
  if (!window.initAliyunCaptcha) {
    throw new Error('Aliyun captcha script not ready')
  }
  await nextTick()
  if (!document.getElementById(elementId)) {
    throw new Error('Aliyun captcha element is missing')
  }
  initializedWidth = captchaWidth()
  const result = window.initAliyunCaptcha({
    SceneId: props.sceneId,
    prefix: props.prefix,
    region: props.region,
    mode: 'embed',
    element: `#${elementId}`,
    captchaVerifyCallback: (captchaVerifyParam: string) => {
      onCaptchaParam(captchaVerifyParam)
      return { captchaResult: true }
    },
    onBizResultCallback: () => {},
    onFallback: () => {
      markError()
    },
    getInstance: (instance) => {
      captchaInstance = instance
      if (state.value === 'loading') {
        state.value = 'idle'
      }
    },
    slideStyle: { width: initializedWidth, height: 40 },
    language: locale.value.toLowerCase().startsWith('zh') ? 'cn' : 'en',
    isShowErrorTip: true
  })
  await Promise.resolve(result)
  if (state.value === 'loading') {
    state.value = 'idle'
  }
}

async function rebuildCaptchaForWidth(): Promise<void> {
  const element = document.getElementById(elementId)
  if (!element) return

  settlePending(null)
  cachedParam = null
  state.value = 'loading'
  destroyCaptchaInstance()
  element.replaceChildren()
  try {
    await initCaptcha()
  } catch (error) {
    console.error('Failed to resize Aliyun captcha:', error)
    markError()
  }
}

function setupResizeObserver(): void {
  if (typeof ResizeObserver === 'undefined' || resizeObserver) return
  const element = document.getElementById(elementId)
  if (!element) return
  const container = element.parentElement || element

  resizeObserver = new ResizeObserver(() => {
    if (resizeScheduled || state.value !== 'idle') return
    resizeScheduled = true
    void nextTick().then(() => {
      resizeScheduled = false
      const width = captchaWidth()
      if (state.value === 'idle' && width !== initializedWidth) {
        void rebuildCaptchaForWidth()
      }
    })
  })
  resizeObserver.observe(container)
}

function ensureReady(): Promise<void> {
  if (!readyPromise) {
    readyPromise = loadScript().then(() => initCaptcha())
    readyPromise.catch((error) => {
      console.error('Failed to initialize Aliyun captcha:', error)
      readyPromise = null
      markError()
    })
  }
  return readyPromise
}

function onCaptchaParam(param: string): void {
  cachedParam = param
  state.value = 'verified'
  emit('verify', param)
  const current = pending
  pending = null
  current?.resolve(param)
}

function settlePending(value: string | null): void {
  const current = pending
  pending = null
  current?.resolve(value)
}

async function verify(): Promise<string | null> {
  if (state.value === 'verified' && cachedParam) {
    return cachedParam
  }
  if (state.value === 'error') {
    return null
  }
  settlePending(null)
  await ensureReady()
  if (state.value === 'verified' && cachedParam) {
    return cachedParam
  }
  return new Promise<string | null>((resolve) => {
    pending = { resolve }
  })
}

function reset(): void {
  settlePending(null)
  cachedParam = null
  state.value = 'idle'
  emit('expire')
  try {
    captchaInstance?.refresh?.()
  } catch {
    readyPromise = null
    state.value = 'loading'
    void ensureReady()
  }
}

defineExpose({ verify, reset })

onMounted(() => {
  if (!props.sceneId || !props.prefix) {
    return
  }
  void ensureReady().then(setupResizeObserver, () => {})
})

onUnmounted(() => {
  settlePending(null)
  resizeObserver?.disconnect()
  resizeObserver = null
  destroyCaptchaInstance()
})
</script>

<style scoped>
.aliyun-captcha-wrapper {
  width: 100%;
}

.aliyun-captcha-embed {
  min-height: 44px;
  width: 100%;
  overflow: hidden;
}

.aliyun-captcha-embed--verified {
  opacity: 0.85;
}

.aliyun-captcha-status {
  margin-top: 0.5rem;
  font-size: 0.75rem;
  font-weight: 500;
  color: rgb(21 128 61);
}

.aliyun-captcha-status--muted {
  color: rgb(107 114 128);
}

.aliyun-captcha-status--error {
  color: rgb(185 28 28);
}

:root.dark .aliyun-captcha-status,
.dark .aliyun-captcha-status {
  color: rgb(134 239 172);
}

:root.dark .aliyun-captcha-status--muted,
.dark .aliyun-captcha-status--muted {
  color: rgb(156 163 175);
}

:root.dark .aliyun-captcha-status--error,
.dark .aliyun-captcha-status--error {
  color: rgb(252 165 165);
}
</style>
