import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AliyunCaptchaWidget from '../AliyunCaptchaWidget.vue'

interface CapturedInitOptions {
  SceneId: string
  prefix: string
  mode: string
  element: string
  button?: string
  captchaVerifyCallback: (param: string) => { captchaResult: boolean }
  onFallback?: (error?: unknown) => void
  getInstance: (instance: { refresh?: () => void; destroy?: () => void }) => void
  slideStyle?: { width: number; height: number }
  language?: string
}

const i18nStub = {
  install(app: { config: { globalProperties: Record<string, unknown> } }) {
    app.config.globalProperties.$t = (key: string) => key
  }
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ locale: { value: 'zh-CN' }, t: (key: string) => key })
}))

describe('AliyunCaptchaWidget', () => {
  let initOptions: CapturedInitOptions | null

  beforeEach(() => {
    initOptions = null
    window.initAliyunCaptcha = vi.fn((options: CapturedInitOptions) => {
      initOptions = options
      options.getInstance({})
    }) as unknown as typeof window.initAliyunCaptcha
  })

  afterEach(() => {
    vi.restoreAllMocks()
    delete window.initAliyunCaptcha
    delete window.AliyunCaptchaConfig
  })

  function mountWidget() {
    return mount(AliyunCaptchaWidget, {
      props: { sceneId: 'scene-1', prefix: 'prefix-1', region: 'cn' as const },
      attachTo: document.body,
      global: { plugins: [i18nStub] }
    })
  }

  it('以 embed 模式初始化，不展示点击人机验证按钮', async () => {
    const wrapper = mountWidget()
    await Promise.resolve()
    await Promise.resolve()

    expect(wrapper.find('.aliyun-captcha-embed').exists()).toBe(true)
    expect(wrapper.find('button').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('auth.captchaClickToVerify')
    expect(window.AliyunCaptchaConfig).toEqual({ region: 'cn', prefix: 'prefix-1' })
    expect(initOptions).not.toBeNull()
    expect(initOptions!.mode).toBe('embed')
    expect(initOptions!.button).toBeUndefined()
    expect(initOptions!.SceneId).toBe('scene-1')
    expect(initOptions!.slideStyle).toEqual({ width: 360, height: 40 })
    expect(initOptions!.language).toBe('cn')

    wrapper.unmount()
  })

  it('窄屏下按容器实际宽度初始化验证码', async () => {
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({
      width: 280,
      height: 40,
      top: 0,
      right: 280,
      bottom: 40,
      left: 0,
      x: 0,
      y: 0,
      toJSON: () => ({})
    } as DOMRect)

    const wrapper = mountWidget()
    await Promise.resolve()
    await Promise.resolve()

    expect(initOptions?.slideStyle).toEqual({ width: 280, height: 40 })

    wrapper.unmount()
  })

  it('容器变窄后按新宽度重建验证码并在卸载时停止监听', async () => {
    let width = 360
    let resizeCallback: ResizeObserverCallback | null = null
    const observe = vi.fn()
    const disconnect = vi.fn()
    const destroy = vi.fn()
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(
      () =>
        ({
          width,
          height: 40,
          top: 0,
          right: width,
          bottom: 40,
          left: 0,
          x: 0,
          y: 0,
          toJSON: () => ({})
        }) as DOMRect
    )
    vi.stubGlobal(
      'ResizeObserver',
      class {
        constructor(callback: ResizeObserverCallback) {
          resizeCallback = callback
        }
        observe = observe
        disconnect = disconnect
        unobserve = vi.fn()
      }
    )
    window.initAliyunCaptcha = vi.fn((options: CapturedInitOptions) => {
      initOptions = options
      options.getInstance({ destroy })
    }) as unknown as typeof window.initAliyunCaptcha

    const wrapper = mountWidget()
    await flushPromises()
    expect(initOptions?.slideStyle).toEqual({ width: 360, height: 40 })
    expect(observe).toHaveBeenCalledOnce()

    width = 260
    resizeCallback?.([], {} as ResizeObserver)
    await flushPromises()

    expect(window.initAliyunCaptcha).toHaveBeenCalledTimes(2)
    expect(destroy).toHaveBeenCalledOnce()
    expect(initOptions?.slideStyle).toEqual({ width: 260, height: 40 })

    wrapper.unmount()
    expect(disconnect).toHaveBeenCalledOnce()
  })

  it('嵌入验证通过后 emit verify 并显示已完成', async () => {
    const wrapper = mountWidget()
    await Promise.resolve()
    await Promise.resolve()

    const result = initOptions!.captchaVerifyCallback('captcha-param-1')
    expect(result).toEqual({ captchaResult: true })
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('verify')).toEqual([['captcha-param-1']])
    expect(wrapper.text()).toContain('auth.captchaVerified')

    wrapper.unmount()
  })

  it('verify() 在已预验证时直接复用缓存的 captchaVerifyParam', async () => {
    const wrapper = mountWidget()
    await Promise.resolve()
    await Promise.resolve()

    initOptions!.captchaVerifyCallback('captcha-param-2')

    const vm = wrapper.vm as unknown as { verify: () => Promise<string | null> }
    await expect(vm.verify()).resolves.toBe('captcha-param-2')

    wrapper.unmount()
  })

  it('verify() 在未完成嵌入验证时等待回调，不弹出点击按钮', async () => {
    const wrapper = mountWidget()
    await Promise.resolve()
    await Promise.resolve()

    const vm = wrapper.vm as unknown as { verify: () => Promise<string | null> }
    const pending = vm.verify()
    await Promise.resolve()
    expect(wrapper.text()).not.toContain('auth.captchaClickToVerify')

    initOptions!.captchaVerifyCallback('captcha-param-3')
    await expect(pending).resolves.toBe('captcha-param-3')

    wrapper.unmount()
  })

  it('reset() 清空缓存并刷新嵌入实例', async () => {
    const refresh = vi.fn()
    window.initAliyunCaptcha = vi.fn((options: CapturedInitOptions) => {
      initOptions = options
      options.getInstance({ refresh })
    }) as unknown as typeof window.initAliyunCaptcha

    const wrapper = mountWidget()
    await Promise.resolve()
    await Promise.resolve()

    initOptions!.captchaVerifyCallback('captcha-param-4')
    await wrapper.vm.$nextTick()

    const vm = wrapper.vm as unknown as {
      verify: () => Promise<string | null>
      reset: () => void
    }
    vm.reset()
    await wrapper.vm.$nextTick()
    expect(refresh).toHaveBeenCalled()
    expect(wrapper.text()).not.toContain('auth.captchaVerified')

    const pending = vm.verify()
    await Promise.resolve()
    initOptions!.captchaVerifyCallback('captcha-param-5')
    await expect(pending).resolves.toBe('captcha-param-5')

    wrapper.unmount()
  })

  it('onFallback 时展示加载失败文案', async () => {
    const wrapper = mountWidget()
    await Promise.resolve()
    await Promise.resolve()

    initOptions!.onFallback?.('Network Error')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('auth.captchaLoadFailed')
    expect(wrapper.emitted('error')).toBeTruthy()

    wrapper.unmount()
  })

  it('初始化失败时只 emit 一次 error', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    window.initAliyunCaptcha = vi.fn(() =>
      Promise.reject(new Error('init failed'))
    ) as unknown as typeof window.initAliyunCaptcha

    const wrapper = mountWidget()
    await flushPromises()

    expect(wrapper.emitted('error')).toHaveLength(1)
    expect(wrapper.text()).toContain('auth.captchaLoadFailed')

    wrapper.unmount()
  })
})

declare global {
  interface Window {
    initAliyunCaptcha?: (options: unknown) => void
    AliyunCaptchaConfig?: { region: string; prefix: string }
  }
}
