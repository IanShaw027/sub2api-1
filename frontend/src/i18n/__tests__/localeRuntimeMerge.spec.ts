import { beforeEach, describe, expect, it, vi } from 'vitest'

const setLocaleMessage = vi.fn()

vi.mock('vue-i18n', () => ({
  createI18n: () => ({
    global: {
      locale: { value: 'zh' },
      setLocaleMessage
    }
  })
}))

describe('i18n runtime locale merge', () => {
  beforeEach(() => {
    setLocaleMessage.mockReset()
    localStorage.clear()
    localStorage.setItem('sub2api_locale', 'zh')
    document.documentElement.setAttribute('lang', '')
    vi.resetModules()
  })

  it('merges ts locale additions into runtime messages for image pricing', async () => {
    const { loadLocaleMessages } = await import('../index')

    await loadLocaleMessages('zh')

    expect(setLocaleMessage).toHaveBeenCalledTimes(1)
    const [, messages] = setLocaleMessage.mock.calls[0]
    expect(messages.admin.groups.imagePricing.allowImageGeneration).toBe('允许当前分组生图')
    expect(messages.admin.groups.imagePricing.independentMultiplier).toBe('生图倍率独立')
    expect(messages.admin.groups.imagePricing.imageMultiplier).toBe('生图独立倍率')
    expect(messages.admin.groups.imagePricing.modeHint).toContain('图片费用')
    expect(messages.admin.groups.imagePricing.finalPricePreview).toBe('最终单张价格预览')
    expect(messages.admin.groups.imagePricing.notConfigured).toBe('未配置')
    expect(messages.profile.overviewDescription).toBe('快速查看账号状态、资料来源与常用设置。')
    expect(messages.common.autoRefresh.countdown).toBe('自动刷新: {seconds}s')
    expect(messages.admin.settings.features.availableChannels.enabled).toBe('启用可用渠道')
    expect(messages.admin.ops.healthHelp).toBe('基于 SLA、错误率和资源使用情况的系统整体健康评分')
    expect(messages.admin.accounts.listPendingSyncHint).toBe('列表存在待同步变更，点击同步可补齐最新数据。')
    expect(messages.admin.channels.form.applyPricingToAccountStats).toBe('应用模型定价到账号统计')
  })

  it('loads english fallback messages before the current locale during init', async () => {
    const { initI18n } = await import('../index')

    await initI18n()

    expect(setLocaleMessage).toHaveBeenCalledTimes(2)
    expect(setLocaleMessage.mock.calls[0][0]).toBe('en')
    expect(setLocaleMessage.mock.calls[1][0]).toBe('zh')
    expect(document.documentElement.getAttribute('lang')).toBe('zh')
  })
})
