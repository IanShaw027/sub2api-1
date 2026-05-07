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
    expect(messages.common.tryAgain).toBe('请重试')
    expect(messages.common.sending).toBe('发送中...')
    expect(messages.common.creating).toBe('创建中...')
    expect(messages.common.clear).toBe('清除')
    expect(messages.common.date).toBe('日期')
    expect(messages.common.required).toBe('必填')
    expect(messages.nav.skillGovernance).toBe('技能治理')
    expect(messages.profile.identity.source.avatar).toBe('头像当前来自 {providerName}')
    expect(messages.profile.identity.source.username).toBe('昵称当前来自 {providerName}')
    expect(messages.admin.users.passwordCopied).toContain('密码已复制')
    expect(messages.admin.channels.noGroupsSelected).toContain('未选择任何分组')
    expect(messages.admin.channels.emptyModelsInPricing).toContain('空模型配置')
    expect(messages.admin.accounts.fromModel).toBe('源模型')
    expect(messages.admin.accounts.toModel).toBe('目标模型')
    expect(messages.admin.accounts.noMappingsConfigured).toContain('暂无映射配置')
    expect(messages.admin.accounts.oauth.failedToGenerateUrl).toContain('生成授权链接失败')
    expect(messages.admin.accounts.oauth.openai.accessTokenAuth).toBe('手动输入 AT')
    expect(messages.admin.accounts.oauth.openai.mobileRefreshTokenAuth).toContain('RT')
    expect(messages.admin.ops.runtime.metricThresholds).toContain('指标阈值')
    expect(messages.admin.ops.runtime.slaMinPercent).toContain('SLA')
    expect(messages.admin.ops.runtime.ttftP99MaxMs).toContain('TTFT')
    expect(messages.admin.ops.runtime.requestErrorRateMaxPercent).toContain('请求错误率')
    expect(messages.admin.ops.runtime.upstreamErrorRateMaxPercent).toContain('上游错误率')
    expect(messages.admin.settings.rateLimit429Cooldown.title).toContain('429')
    expect(messages.admin.settings.rateLimit429Cooldown.cooldownSecondsHint).toContain('7200')
    expect(messages.admin.settings.openaiFastPolicy.title).toContain('OpenAI Fast/Flex')
    expect(messages.admin.settings.openaiFastPolicy.actionBlock).toContain('拦截')
    expect(messages.admin.settings.openaiFastPolicy.scopeAPIKey).toContain('API Key')
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
