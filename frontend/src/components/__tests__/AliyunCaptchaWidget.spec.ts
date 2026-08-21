import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AliyunCaptchaWidget from '../AliyunCaptchaWidget.vue'

interface CapturedInitOptions {
  SceneId: string
  mode: string
  element: string
  button: string
  captchaVerifyCallback: (
    param: string
  ) =>
    | { captchaResult: boolean; bizResult?: boolean }
    | Promise<{ captchaResult: boolean; bizResult?: boolean }>
  onBizResultCallback: (bizResult: boolean) => void
  getInstance: (instance: { refresh?: () => void; destroy?: () => void }) => void
  slideStyle?: { width: number; height: number }
  language?: string
  autoRefresh?: boolean
  rem?: number
  onError?: (error?: unknown) => void
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

  it('以 embed 模式初始化，包含官方 button/onError/autoRefresh=false', async () => {
    const wrapper = mountWidget()
    await flushPromises()

    expect(wrapper.find('.aliyun-captcha-embed').exists()).toBe(true)
    expect(wrapper.find('button.aliyun-captcha-trigger').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('auth.captchaClickToVerify')
    expect(window.AliyunCaptchaConfig).toEqual({ region: 'cn', prefix: 'prefix-1' })
    expect(initOptions).not.toBeNull()
    expect(initOptions!.mode).toBe('embed')
    expect(initOptions!.button).toMatch(/^#aliyun-captcha-button-/)
    expect(initOptions!.SceneId).toBe('scene-1')
    expect(initOptions!.slideStyle).toEqual({ width: 360, height: 40 })
    expect(initOptions!.language).toBe('cn')
    expect(initOptions!.autoRefresh).toBe(false)
    expect(typeof initOptions!.onError).toBe('function')
    expect(initOptions!.rem).toBe(1)

    wrapper.unmount()
  })

  it('窄屏下用 rem 缩放，slideStyle 宽度不低于 320', async () => {
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
    await flushPromises()

    expect(initOptions?.slideStyle).toEqual({ width: 360, height: 40 })
    expect(initOptions?.rem).toBe(0.77)

    wrapper.unmount()
  })

  it('run() 通过隐藏 button 触发，并在 captchaVerifyCallback 中执行业务', async () => {
    const wrapper = mountWidget()
    await flushPromises()

    const trigger = wrapper.find('button.aliyun-captcha-trigger').element as HTMLButtonElement
    const clickSpy = vi.spyOn(trigger, 'click')
    const business = vi.fn(async (param: string) => {
      expect(param).toBe('captcha-param-1')
      return { captchaResult: true, bizResult: true }
    })

    const vm = wrapper.vm as unknown as {
      run: (
        fn: (param: string) => Promise<{ captchaResult: boolean; bizResult?: boolean }>
      ) => Promise<{ captchaResult: boolean; bizResult?: boolean }>
    }
    const pending = vm.run(business)
    await flushPromises()
    expect(clickSpy).toHaveBeenCalled()

    const callbackResult = await initOptions!.captchaVerifyCallback('captcha-param-1')
    expect(callbackResult).toEqual({ captchaResult: true, bizResult: true })
    await expect(pending).resolves.toEqual({ captchaResult: true, bizResult: true })
    expect(business).toHaveBeenCalledWith('captcha-param-1')
    expect(wrapper.emitted('verify')).toEqual([['captcha-param-1']])

    wrapper.unmount()
  })

  it('run() 定时器不截断超过 2.5s 的业务请求', async () => {
    const wrapper = mountWidget()
    await flushPromises()
    vi.useFakeTimers()

    const vm = wrapper.vm as unknown as {
      run: (
        fn: (param: string) => Promise<{ captchaResult: boolean; bizResult?: boolean }>
      ) => Promise<{ captchaResult: boolean; bizResult?: boolean }>
    }

    let finishBusiness: ((value: { captchaResult: boolean; bizResult?: boolean }) => void) | null =
      null
    const pending = vm.run(
      () =>
        new Promise((resolve) => {
          finishBusiness = resolve
        })
    )
    await flushPromises()
    const callbackPromise = initOptions!.captchaVerifyCallback('slow-param')

    await vi.advanceTimersByTimeAsync(3000)
    finishBusiness?.({ captchaResult: true, bizResult: true })
    await flushPromises()

    await expect(callbackPromise).resolves.toEqual({ captchaResult: true, bizResult: true })
    await expect(pending).resolves.toEqual({ captchaResult: true, bizResult: true })

    wrapper.unmount()
    vi.useRealTimers()
  })

  it('run() 拒绝重入且旧回调不会执行后启动的业务', async () => {
    const wrapper = mountWidget()
    await flushPromises()

    const vm = wrapper.vm as unknown as {
      run: (
        fn: (param: string) => Promise<{ captchaResult: boolean; bizResult?: boolean }>
      ) => Promise<{ captchaResult: boolean; bizResult?: boolean }>
      reset: () => void
    }
    let finishFirst: ((value: { captchaResult: boolean; bizResult?: boolean }) => void) | null = null
    const firstBusiness = vi.fn(
      () =>
        new Promise<{ captchaResult: boolean; bizResult?: boolean }>((resolve) => {
          finishFirst = resolve
        })
    )
    const secondBusiness = vi.fn(async () => ({ captchaResult: true, bizResult: true }))

    const firstRun = vm.run(firstBusiness)
    await flushPromises()
    const firstCallback = initOptions!.captchaVerifyCallback('first-param')
    await flushPromises()

    // 业务请求的 finally 会刷新验证码，但当前回调完成前仍必须保持重入锁。
    vm.reset()
    await expect(vm.run(secondBusiness)).resolves.toEqual({
      captchaResult: false,
      bizResult: false
    })
    expect(secondBusiness).not.toHaveBeenCalled()

    finishFirst?.({ captchaResult: true, bizResult: true })
    await expect(firstCallback).resolves.toEqual({ captchaResult: true, bizResult: true })
    await expect(firstRun).resolves.toEqual({ captchaResult: true, bizResult: true })
    expect(firstBusiness).toHaveBeenCalledWith('first-param')

    wrapper.unmount()
  })

  it('无 pending 业务时 captchaVerifyCallback 返回失败，避免假通过', async () => {
    const wrapper = mountWidget()
    await flushPromises()

    const result = await initOptions!.captchaVerifyCallback('orphan-param')
    expect(result).toEqual({ captchaResult: false, bizResult: false })

    wrapper.unmount()
  })

  it('verify() 通过 run 取参', async () => {
    const wrapper = mountWidget()
    await flushPromises()

    const vm = wrapper.vm as unknown as { verify: () => Promise<string | null> }
    const pending = vm.verify()
    await flushPromises()

    await initOptions!.captchaVerifyCallback('captcha-param-2')
    await expect(pending).resolves.toBe('captcha-param-2')

    wrapper.unmount()
  })

  it('reset() 刷新嵌入实例', async () => {
    const refresh = vi.fn()
    window.initAliyunCaptcha = vi.fn((options: CapturedInitOptions) => {
      initOptions = options
      options.getInstance({ refresh })
    }) as unknown as typeof window.initAliyunCaptcha

    const wrapper = mountWidget()
    await flushPromises()

    const vm = wrapper.vm as unknown as { reset: () => void }
    vm.reset()
    await wrapper.vm.$nextTick()
    expect(refresh).toHaveBeenCalled()

    wrapper.unmount()
  })

  it('onError 时展示加载失败文案', async () => {
    const wrapper = mountWidget()
    await flushPromises()

    initOptions!.onError?.('Network Error')
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

  it('初始化临时失败后下一次 run 会重试', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    window.initAliyunCaptcha = vi
      .fn()
      .mockImplementationOnce(() => Promise.reject(new Error('temporary init failure')))
      .mockImplementation((options: CapturedInitOptions) => {
        initOptions = options
        options.getInstance({})
      }) as unknown as typeof window.initAliyunCaptcha

    const wrapper = mountWidget()
    await flushPromises()
    expect(wrapper.text()).toContain('auth.captchaLoadFailed')

    const business = vi.fn(async () => ({ captchaResult: true, bizResult: true }))
    const vm = wrapper.vm as unknown as {
      run: (fn: typeof business) => Promise<{ captchaResult: boolean; bizResult?: boolean }>
    }
    const pending = vm.run(business)
    await flushPromises()
    expect(initOptions).not.toBeNull()
    await initOptions!.captchaVerifyCallback('retry-param')

    await expect(pending).resolves.toEqual({ captchaResult: true, bizResult: true })
    expect(business).toHaveBeenCalledWith('retry-param')
    wrapper.unmount()
  })
})

declare global {
  interface Window {
    initAliyunCaptcha?: (options: unknown) => void
    AliyunCaptchaConfig?: { region: string; prefix: string }
  }
}
