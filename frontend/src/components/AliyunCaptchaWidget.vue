<template>
  <div v-if="sceneId && prefix" class="aliyun-captcha-wrapper">
    <div
      :id="elementId"
      class="aliyun-captcha-embed"
      :class="{ 'aliyun-captcha-embed--verified': state === 'verified' }"
    ></div>
    <button
      :id="buttonId"
      type="button"
      class="aliyun-captcha-trigger"
      tabindex="-1"
      aria-hidden="true"
    ></button>
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

export interface AliyunCaptchaBizResult {
  captchaResult: boolean
  bizResult?: boolean
}

type AliyunCaptchaBusinessFn = (captchaVerifyParam: string) => Promise<AliyunCaptchaBizResult>

interface AliyunCaptchaInstance {
  refresh?: () => void
  destroy?: () => void
}

interface AliyunCaptchaInitOptions {
  SceneId: string
  mode: 'popup' | 'embed'
  element: string
  button: string
  captchaVerifyCallback: (
    captchaVerifyParam: string
  ) => AliyunCaptchaBizResult | Promise<AliyunCaptchaBizResult>
  onBizResultCallback: (bizResult: boolean) => void
  getInstance: (instance: AliyunCaptchaInstance) => void
  slideStyle?: { width: number; height: number }
  language?: string
  immediate?: boolean
  autoRefresh?: boolean
  timeout?: number
  rem?: number
  onError?: (error?: unknown) => void
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
  (e: 'biz-result', ok: boolean): void
}>()

const { t, locale } = useI18n()

const uid = Math.random().toString(36).slice(2, 10)
const elementId = `aliyun-captcha-element-${uid}`
const buttonId = `aliyun-captcha-button-${uid}`

const state = ref<'loading' | 'idle' | 'verified' | 'error'>('loading')

const SCRIPT_SRC = 'https://o.alicdn.com/captcha-frontend/aliyunCaptcha/AliyunCaptcha.js'
const BASE_SLIDE_WIDTH = 360
const MIN_SLIDE_WIDTH = 320
const TRIGGER_WAIT_MS = 2500

let readyPromise: Promise<void> | null = null
let captchaInstance: AliyunCaptchaInstance | null = null
let pendingRun: {
  resolve: (value: AliyunCaptchaBizResult) => void
  triggerTimer: ReturnType<typeof setTimeout> | null
  business: AliyunCaptchaBusinessFn
} | null = null

const loadScript = (): Promise<void> => {
  return new Promise((resolve, reject) => {
    // region/prefix 只能放在全局 AliyunCaptchaConfig，且必须在加载 JS 之前设置
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
  settleRun({ captchaResult: false, bizResult: false })
}

function clearTriggerTimer(run = pendingRun): void {
  if (run?.triggerTimer != null) {
    clearTimeout(run.triggerTimer)
    run.triggerTimer = null
  }
}

function settleRun(
  result: AliyunCaptchaBizResult,
  expected: typeof pendingRun = pendingRun
): void {
  if (!expected || pendingRun !== expected) {
    return
  }
  const current = expected
  pendingRun = null
  if (current.triggerTimer != null) {
    clearTimeout(current.triggerTimer)
  }
  current.resolve(result)
}

function destroyCaptchaInstance(): void {
  try {
    captchaInstance?.destroy?.()
  } catch {
    // SDK 实例销毁失败不影响卸载
  }
  captchaInstance = null
}

function captchaRem(): number {
  const element = document.getElementById(elementId)
  const width = Math.floor(
    element?.parentElement?.getBoundingClientRect().width ||
      element?.getBoundingClientRect().width ||
      0
  )
  if (width <= 0 || width >= BASE_SLIDE_WIDTH) {
    return 1
  }
  return Math.max(0.5, Math.floor((width / BASE_SLIDE_WIDTH) * 100) / 100)
}

async function initCaptcha(): Promise<void> {
  if (!window.initAliyunCaptcha) {
    throw new Error('Aliyun captcha script not ready')
  }
  await nextTick()
  if (!document.getElementById(elementId) || !document.getElementById(buttonId)) {
    throw new Error('Aliyun captcha element is missing')
  }

  const rem = captchaRem()
  const result = window.initAliyunCaptcha({
    SceneId: props.sceneId,
    mode: 'embed',
    element: `#${elementId}`,
    button: `#${buttonId}`,
    captchaVerifyCallback: async (captchaVerifyParam: string) => {
      // 定时器只覆盖 SDK 触发；业务请求可能超过 TRIGGER_WAIT_MS
      const current = pendingRun
      clearTriggerTimer(current)
      emit('verify', captchaVerifyParam)
      state.value = 'verified'
      if (!current) {
        return { captchaResult: false, bizResult: false }
      }
      try {
        const verifyResult = await current.business(captchaVerifyParam)
        settleRun(verifyResult, current)
        return verifyResult
      } catch {
        const failed = { captchaResult: false, bizResult: false }
        settleRun(failed, current)
        return failed
      }
    },
    onBizResultCallback: (bizResult: boolean) => {
      emit('biz-result', bizResult === true)
    },
    getInstance: (instance) => {
      captchaInstance = instance
      if (state.value === 'loading') {
        state.value = 'idle'
      }
    },
    slideStyle: { width: Math.max(MIN_SLIDE_WIDTH, BASE_SLIDE_WIDTH), height: 40 },
    language: locale.value.toLowerCase().startsWith('zh') ? 'cn' : 'en',
    immediate: false,
    autoRefresh: false,
    timeout: 5000,
    rem,
    onError: () => {
      markError()
    }
  })
  await Promise.resolve(result)
  if (state.value === 'loading') {
    state.value = 'idle'
  }
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

function isCaptchaErrorState(): boolean {
  return state.value === 'error'
}

async function run(business: AliyunCaptchaBusinessFn): Promise<AliyunCaptchaBizResult> {
  if (isCaptchaErrorState()) {
    if (readyPromise !== null) {
      return { captchaResult: false, bizResult: false }
    }
    state.value = 'loading'
  }
  try {
    await ensureReady()
  } catch {
    return { captchaResult: false, bizResult: false }
  }
  // ensureReady 失败会把 state 置为 error
  if (isCaptchaErrorState()) {
    return { captchaResult: false, bizResult: false }
  }

  // 同一个 SDK 实例一次只能承载一个 captchaVerifyCallback。拒绝重入，
  // 避免旧回调取得或结算后启动请求的业务函数。
  if (pendingRun) {
    return { captchaResult: false, bizResult: false }
  }

  return new Promise<AliyunCaptchaBizResult>((resolve) => {
    const timer = setTimeout(() => {
      if (pendingRun?.resolve === resolve && pendingRun.triggerTimer != null) {
        settleRun({ captchaResult: false, bizResult: false }, pendingRun)
      }
    }, TRIGGER_WAIT_MS)

    pendingRun = { resolve, triggerTimer: timer, business }
    document.getElementById(buttonId)?.click()
  })
}

async function verify(): Promise<string | null> {
  let captured: string | null = null
  const result = await run(async (param) => {
    captured = param
    return { captchaResult: true, bizResult: true }
  })
  if (!result.captchaResult || !captured) {
    return null
  }
  return captured
}

function reset(): void {
  // captchaVerifyCallback 已开始后 triggerTimer 会被清空。业务函数常在
  // finally 中刷新验证码，此时必须等当前回调自行结算，不能释放重入锁。
  if (pendingRun?.triggerTimer != null) {
    settleRun({ captchaResult: false, bizResult: false }, pendingRun)
  }
  if (state.value === 'error' && readyPromise === null) {
    state.value = 'loading'
    void ensureReady()
  } else {
    state.value = state.value === 'error' ? 'error' : 'idle'
  }
  emit('expire')
  try {
    captchaInstance?.refresh?.()
  } catch {
    readyPromise = null
    state.value = 'loading'
    void ensureReady()
  }
}

defineExpose({ run, verify, reset })

onMounted(() => {
  if (!props.sceneId || !props.prefix) {
    return
  }
  void ensureReady()
})

onUnmounted(() => {
  settleRun({ captchaResult: false, bizResult: false })
  destroyCaptchaInstance()
})
</script>

<style scoped>
.aliyun-captcha-wrapper {
  position: relative;
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

.aliyun-captcha-trigger {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
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

:global(:root[data-theme='glass-dark']) .aliyun-captcha-status {
  color: rgb(134 239 172);
}

:global(:root[data-theme='glass-dark']) .aliyun-captcha-status--muted {
  color: rgb(156 163 175);
}

:global(:root[data-theme='glass-dark']) .aliyun-captcha-status--error {
  color: rgb(252 165 165);
}
</style>
