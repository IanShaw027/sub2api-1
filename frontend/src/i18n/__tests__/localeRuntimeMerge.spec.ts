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
    expect(messages.admin.settings.features.ticket.title).toBe('工单模块')
    expect(messages.admin.settings.features.ticket.description).toBe(
      '控制用户端和管理端工单入口。默认关闭，开启后用户可提交工单，管理员可处理工单。'
    )
    expect(messages.admin.settings.features.ticket.enabled).toBe('启用工单模块')
    expect(messages.admin.settings.features.ticket.enabledHint).toBe(
      '关闭后用户侧和管理侧工单菜单隐藏，工单页面会重定向到仪表盘。'
    )
    expect(messages.admin.dashboard.description).toBe('系统概览与统计数据')
    expect(messages.admin.users.columns.lastLogin).toBe('最后登录时间')
    expect(messages.admin.users.typeAffiliateBalance).toBe('余额（返利转入）')
    expect(messages.admin.groups.imagePricing.description).toBe(
      '配置图片生成能力和图片基础单价，留空则使用默认价格'
    )
    expect(messages.admin.groups.imagePricing.allowImageGeneration).toBe('允许当前分组生图')
    expect(messages.admin.groups.platforms.sora).toBe('Sora')
    expect(messages.admin.redeem.status.active).toBe('未使用')
    expect(messages.admin.accounts.accountNameOAuthPlaceholder).toContain('OAuth 可留空')
    expect(messages.admin.accounts.imageTestRouteLabel).toBe('测试路径')
    expect(messages.admin.accounts.vertexLabel).toBe('Vertex')
    expect(messages.admin.accounts.openai.responsesMode).toBe('文本生成协议')
    expect(messages.admin.accounts.openai.responsesModeForceResponses).toBe('强制 Responses')
    expect(messages.admin.accounts.openai.endpointCapabilities).toBe('允许的端点能力')
    expect(messages.admin.accounts.openai.endpointCapabilityChatCompletions).toBe('文本生成')
    expect(messages.admin.accounts.openai.endpointCapabilityEmbeddings).toBe('Embeddings')
    expect(messages.admin.accounts.oauth.gemini.aiStudioNotConfiguredShort).toBe('未配置')
    expect(messages.admin.accounts.gemini.oauthType.badges.adminRequired).toBe('需要管理员')
    expect(messages.admin.accounts.kiro.refreshNowTitle).toContain('已保存的 refresh token')
    expect(messages.admin.proxies.description).toBe('管理代理服务器配置')
    expect(messages.admin.settings.gatewayForwarding.description).toBe(
      '控制请求转发到上游 OAuth 账号时的行为'
    )
    expect(messages.admin.settings.gatewayForwarding.debugTimelineHint).toContain(
      '详细阶段耗时 JSONL 日志'
    )
    expect(messages.admin.settings.gatewayForwarding.debugTimelineDirectory).toBe('日志目录')
    expect(messages.admin.settings.gatewayForwarding.debugTimelineDirectoryHint).toContain(
      'gateway-timeline-YYYY-MM-DD.log'
    )
    expect(messages.admin.settings.gatewayForwarding.debugTimelineMaxSizeMB).toBe('最大占用 MB')
    expect(messages.admin.settings.gatewayForwarding.anthropicCacheTTL1hInjection).toBe(
      'Anthropic 缓存 TTL 注入'
    )
    expect(messages.admin.settings.gatewayForwarding.anthropicCacheTTL1hInjectionHint).toContain(
      'ephemeral 缓存块强制写入 1h'
    )
    expect(messages.admin.settings.gatewayForwarding.claudeTelemetryMode).toBe(
      'Claude Code 遥测处理'
    )
    expect(messages.admin.settings.gatewayForwarding.claudeTelemetryModeHint).toContain(
      'API Key 账号不会参与'
    )
    expect(messages.admin.ops.settings.title).toBe('运维监控设置')
    expect(messages.admin.ops.settings.retentionDaysHint).toContain(
      '填 0 表示每次定时清理时清空所有历史'
    )
    expect(messages.admin.ops.settings.validation.title).toBe('请修复以下问题')
    expect(messages.admin.ops.settings.validation.retentionDaysRange).toBe(
      '保留天数必须在 0-365 天之间（0 = 每次清理都清空历史）'
    )
    expect(messages.admin.riskControl.apiKeysModeAppend).toBe('增量添加')
    expect(messages.admin.riskControl.apiKeysPlaceholderReplace).toContain('覆盖保存')
    expect(messages.admin.riskControl.apiKeyPendingDelete).toBe('待删除')
    expect(messages.admin.riskControl.modelFilterInclude).toBe('仅指定模型')
  })

  it('keeps runtime english overrides aligned with locale json semantics', async () => {
    const { loadLocaleMessages } = await import('../index')

    await loadLocaleMessages('en')

    expect(setLocaleMessage).toHaveBeenCalledTimes(1)
    const [, messages] = setLocaleMessage.mock.calls[0]
    expect(messages.admin.settings.features.ticket.title).toBe('Ticket Module')
    expect(messages.admin.settings.features.ticket.description).toBe(
      'Controls user and admin ticket entry points. Disabled by default; enable it to let users submit tickets and admins process them.'
    )
    expect(messages.admin.settings.features.ticket.enabled).toBe('Enable Tickets')
    expect(messages.admin.settings.features.ticket.enabledHint).toBe(
      'When off, ticket menus are hidden and ticket pages redirect to the dashboard.'
    )
    expect(messages.admin.dashboard.description).toBe('System overview and real-time statistics')
    expect(messages.admin.users.columns.lastLogin).toBe('Last Login')
    expect(messages.admin.users.typeAffiliateBalance).toBe('Balance (Affiliate Transfer)')
    expect(messages.admin.groups.imagePricing.description).toBe(
      'Configure image generation access and base image prices. Leave empty to use default prices.'
    )
    expect(messages.admin.groups.imagePricing.allowImageGeneration).toBe(
      'Allow image generation for this group'
    )
    expect(messages.admin.groups.platforms.sora).toBe('Sora')
    expect(messages.admin.redeem.status.active).toBe('Unused')
    expect(messages.admin.accounts.accountNameOAuthPlaceholder).toContain(
      'Optional for OAuth'
    )
    expect(messages.admin.accounts.imageTestRouteLabel).toBe('Test route')
    expect(messages.admin.accounts.vertexLabel).toBe('Vertex')
    expect(messages.admin.accounts.openai.responsesMode).toBe('Text generation protocol')
    expect(messages.admin.accounts.openai.responsesModeForceResponses).toBe('Force Responses')
    expect(messages.admin.accounts.openai.endpointCapabilities).toBe('Accepted endpoint capabilities')
    expect(messages.admin.accounts.openai.endpointCapabilityChatCompletions).toBe('Text generation')
    expect(messages.admin.accounts.openai.endpointCapabilityEmbeddings).toBe('Embeddings')
    expect(messages.admin.accounts.oauth.gemini.aiStudioNotConfiguredShort).toBe(
      'Not configured'
    )
    expect(messages.admin.accounts.gemini.oauthType.badges.adminRequired).toBe(
      'Admin required'
    )
    expect(messages.admin.accounts.kiro.refreshNowTitle).toContain(
      'stored refresh token'
    )
    expect(messages.admin.proxies.description).toBe('Manage proxy servers for accounts')
    expect(messages.admin.settings.gatewayForwarding.description).toBe(
      'Control how requests are forwarded to upstream OAuth accounts'
    )
    expect(messages.admin.settings.gatewayForwarding.debugTimelineHint).toContain(
      'per-stage JSONL timing logs'
    )
    expect(messages.admin.settings.gatewayForwarding.debugTimelineDirectory).toBe('Log Directory')
    expect(messages.admin.settings.gatewayForwarding.debugTimelineDirectoryHint).toContain(
      'gateway-timeline-YYYY-MM-DD.log'
    )
    expect(messages.admin.settings.gatewayForwarding.debugTimelineMaxSizeMB).toBe('Max Size MB')
    expect(messages.admin.settings.gatewayForwarding.anthropicCacheTTL1hInjection).toBe(
      'Anthropic Cache TTL Injection'
    )
    expect(messages.admin.settings.gatewayForwarding.anthropicCacheTTL1hInjectionHint).toContain(
      'forced to 1h'
    )
    expect(messages.admin.settings.gatewayForwarding.claudeTelemetryMode).toBe(
      'Claude Code Telemetry Handling'
    )
    expect(messages.admin.settings.gatewayForwarding.claudeTelemetryModeHint).toContain(
      'never API key accounts'
    )
    expect(messages.admin.ops.settings.title).toBe('Ops Monitoring Settings')
    expect(messages.admin.ops.settings.retentionDaysHint).toContain(
      'Set to 0 to wipe all history on every scheduled cleanup'
    )
    expect(messages.admin.ops.settings.validation.title).toBe(
      'Please fix the following issues'
    )
    expect(messages.admin.ops.settings.validation.retentionDaysRange).toBe(
      'Retention days must be between 0 and 365 (0 = wipe all on every cleanup)'
    )
    expect(messages.admin.riskControl.apiKeysModeAppend).toBe('Add')
    expect(messages.admin.riskControl.apiKeysPlaceholderReplace).toContain('Replace API Keys')
    expect(messages.admin.riskControl.apiKeyPendingDelete).toBe('Pending delete')
    expect(messages.admin.riskControl.modelFilterInclude).toBe('Only selected')
  })

  it('loads english fallback messages before the current locale during init', async () => {
    const { initI18n } = await import('../index')

    await initI18n()

    expect(setLocaleMessage).toHaveBeenCalledTimes(2)
    expect(setLocaleMessage.mock.calls[0][0]).toBe('en')
    expect(setLocaleMessage.mock.calls[1][0]).toBe('zh')
    expect(document.documentElement.getAttribute('lang')).toBe('zh')
  })

  it('reports conflicting locale keys explicitly', async () => {
    const { collectLocaleConflicts } = await import('../index')

    expect(
      collectLocaleConflicts(
        { a: { b: 'base', c: 'same' }, d: 'keep' },
        { a: { b: 'override', c: 'same' }, d: 'keep', e: 'new' }
      )
    ).toEqual(['a.b'])
  })
})
