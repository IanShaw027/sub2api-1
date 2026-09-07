export default {
  batchImageGuide: {
    title: '图片批量生成',
    description: '一次提交多条提示词，任务完成后可统一下载图片结果'
  },
  // Home Page
  home: {
    viewOnGithub: '在 GitHub 上查看',
    viewDocs: '查看文档',
    docs: '文档',
    switchToLight: '切换到浅色模式',
    switchToDark: '切换到深色模式',
    dashboard: '控制台',
    login: '登录',
    getStarted: '立即开始',
    goToDashboard: '进入控制台',
    // 新增：面向用户的价值主张
    heroSubtitle: '一个密钥，畅用多个 AI 模型',
    heroDescription: '无需管理多个订阅账号，一站式接入 Claude、GPT、Gemini 等主流 AI 服务。',
    tags: {
      subscriptionToApi: '订阅转 API',
      stickySession: '会话保持',
      realtimeBilling: '按量计费'
    },
    // 用户痛点区块
    painPoints: {
      title: '你是否也遇到这些问题？',
      items: {
        expensive: {
          title: '订阅费用高',
          desc: '每个 AI 服务都要单独订阅，每月支出越来越多'
        },
        complex: {
          title: '多账号难管理',
          desc: '不同平台的账号、密钥分散各处，管理起来很麻烦'
        },
        unstable: {
          title: '服务不稳定',
          desc: '单一账号容易触发限制，影响正常使用'
        },
        noControl: {
          title: '用量无法控制',
          desc: '不知道钱花在哪了，也无法限制团队成员的使用'
        }
      }
    },
    // 解决方案区块
    solutions: {
      title: '我们帮你解决',
      subtitle: '简单三步，开始省心使用 AI'
    },
    features: {
      unifiedGateway: '一键接入',
      unifiedGatewayDesc: '获取一个 API 密钥，即可调用所有已接入的 AI 模型，无需分别申请。',
      multiAccount: '稳定可靠',
      multiAccountDesc: '智能调度多个上游账号，自动切换和负载均衡，告别频繁报错。',
      balanceQuota: '用多少付多少',
      balanceQuotaDesc: '按实际使用量计费，支持设置配额上限，团队用量一目了然。'
    },
    // 优势对比
    comparison: {
      title: '为什么选择我们？',
      heading: '对比官方订阅',
      headingDesc: '智能调度多个上游账号，自动切换和负载均衡；按实际使用量计费，支持配额上限，团队用量一目了然。',
      headers: {
        feature: '对比项',
        official: '官方订阅',
        us: '本平台'
      },
      items: {
        pricing: {
          feature: '付费方式',
          official: '固定月费，用不完也付',
          us: '按量付费，用多少付多少'
        },
        models: {
          feature: '模型选择',
          official: '单一服务商',
          us: '多模型随意切换'
        },
        management: {
          feature: '账号管理',
          official: '每个服务单独管理',
          us: '统一密钥，一站管理'
        },
        stability: {
          feature: '服务稳定性',
          official: '单账号易触发限制',
          us: '多账号池，自动切换'
        },
        control: {
          feature: '用量控制',
          official: '无法限制',
          us: '可设配额、查明细'
        }
      }
    },
    providers: {
      title: '已支持的 AI 模型',
      description: '一个 API，多种选择',
      supported: '已支持',
      soon: '即将推出',
      claude: 'Claude',
      gemini: 'Gemini',
      antigravity: 'Antigravity',
      more: '更多',
      line: 'Claude · OpenAI · Gemini · Antigravity · Grok'
    },
    // CTA 区块
    cta: {
      title: '准备好开始了吗？',
      description: '注册即可获得免费试用额度，体验一站式 AI 服务',
      button: '免费注册'
    },
    // ---- Glass redesign additions ----
    nav: {
      modelPlaza: '模型广场',
      docs: '文档',
      keyUsage: '用量查询',
      status: '服务状态'
    },
    heroTitle1: '一个密钥，',
    heroTitle2: '畅用多个 AI 模型',
    heroDescriptionExt: '兼容 Anthropic 与 OpenAI 协议，Claude Code、Codex CLI 改一个 Base URL 即可使用。',
    socialProof: '面向 Claude Code · Codex CLI · Cursor 等客户端的开发者',
    console: {
      url: 'console.sub2api.dev/dashboard',
      uptimeLabel: '服务状态 · 近 30 天可用性',
      allOk: '全部正常',
      rpmLabel: '请求 / 分钟 · 24h',
      firstToken: '首 token 620 ms',
      stickyHit: '会话保持命中 98.2%'
    },
    marquee: {
      protocols: '原生兼容',
      platforms: '支持平台',
      modelsLabel: '模型',
      clientsLabel: '客户端'
    },
    highlights: {
      upstream: '上游平台',
      upstreamDesc: 'OAuth 与 API Key 并存',
      protocols: '原生协议',
      protocolsDesc: 'Anthropic · OpenAI',
      clients: '客户端零改动',
      clientsDesc: '改一个 Base URL 即可',
      payments: '自助支付',
      paymentsDesc: '多种支付方式可选'
    },
    flow: {
      kicker: '企业级稳定',
      title: '一条链路，把每次调用稳稳送达',
      description: '鉴权、并发控制、智能调度、故障转移与 token 级计费在网关一次完成。',
      nodes: {
        client: '客户端',
        gateway: '统一网关',
        scheduler: '智能调度',
        upstream: '上游账号池',
        billing: '透明计费'
      }
    },
    steps: {
      kicker: '接入只需三步',
      title: '改一个 Base URL，其余照旧',
      items: {
        create: { title: '注册并创建 API 密钥', desc: '按分组选择模型池，可设置并发、速率与过期时间' },
        baseUrl: { title: '把 Base URL 指向本站', desc: '同时兼容 /v1/messages 与 /v1/chat/completions' },
        client: { title: '在任意客户端使用', desc: '会话保持保证同一对话始终命中同一上游账号' }
      },
      copy: '复制',
      copied: '已复制',
      comment: {
        shell: '# ~/.zshrc',
        start: '# 启动',
        codex: '# ~/.codex/config.toml'
      }
    },
    pricing: {
      kicker: '模型与定价',
      title: '按量付费，价格透明',
      link: '前往模型广场',
      headers: {
        model: '模型',
        vendor: '提供商',
        input: '输入 / 1M',
        output: '输出 / 1M',
        context: '上下文',
        status: '状态'
      },
      normal: '正常',
      limited: '限流',
      footnote: '价格按每 1M tokens 计，缓存读取按输入价 10% 计费；高峰倍率与分组倍率以密钥所属分组为准。',
      empty: '暂无公开定价，登录后可在模型广场查看'
    },
    footerLinks: {
      terms: '服务条款',
      privacy: '隐私政策',
      contact: '联系客服'
    },
    footer: {
      allRightsReserved: '保留所有权利。'
    }
  },

  // Key Usage Query Page
  keyUsage: {
    title: 'API Key 用量查询',
    subtitle: '输入您的 API Key 以查看实时消费金额与使用状态',
    placeholder: 'sk-ant-mirror-xxxxxxxxxxxx',
    query: '查询',
    querying: '查询中...',
    showKey: '显示 Key',
    hideKey: '隐藏 Key',
    privacyNote: '您的 Key 仅在浏览器本地处理，不会被存储',
    dateRange: '统计范围:',
    dateRangeToday: '今日',
    dateRange7d: '7 天',
    dateRange30d: '30 天',
    dateRange90d: '90 天',
    dateRangeCustom: '自定义',
    apply: '应用',
    used: '已使用',
    detailInfo: '详细信息',
    tokenStats: 'Token 统计',
    dailyDetail: '按日明细',
    modelStats: '模型用量统计',
    // Table headers
    date: '日期',
    model: '模型',
    requests: '请求数',
    inputTokens: '输入 Tokens',
    outputTokens: '输出 Tokens',
    cacheCreationTokens: '缓存创建',
    cacheReadTokens: '缓存读取',
    cacheWriteTokens: '缓存写入',
    totalTokens: '总 Tokens',
    cost: '费用',
    // Status
    quotaMode: 'Key 限额模式',
    walletBalance: '钱包余额',
    // Ring card titles
    totalQuota: '总额度',
    limit5h: '5 小时限额',
    limitDaily: '日限额',
    limit7d: '7 天限额',
    limitWeekly: '周限额',
    limitMonthly: '月限额',
    // Detail rows
    remainingQuota: '剩余额度',
    expiresAt: '过期时间',
    todayExpires: '(今日到期)',
    daysLeft: '({days} 天)',
    usedQuota: '已用额度',
    resetNow: '即将重置',
    subscriptionType: '订阅类型',
    subscriptionExpires: '订阅到期',
    // Usage stat cells
    todayRequests: '今日请求',
    todayInputTokens: '今日输入',
    todayOutputTokens: '今日输出',
    todayTokens: '今日 Tokens',
    todayCacheCreation: '今日缓存创建',
    todayCacheRead: '今日缓存读取',
    todayCost: '今日费用',
    rpmTpm: 'RPM / TPM',
    totalRequests: '累计请求',
    totalInputTokens: '累计输入',
    totalOutputTokens: '累计输出',
    totalTokensLabel: '累计 Tokens',
    totalCacheCreation: '累计缓存创建',
    totalCacheRead: '累计缓存读取',
    totalCost: '累计费用',
    avgDuration: '平均耗时',
    // Messages
    enterApiKey: '请输入 API Key',
    querySuccess: '查询成功',
    queryFailed: '查询失败',
    queryFailedRetry: '查询失败，请稍后重试',
    noDailyUsage: '暂无按日用量数据',
  },

  // 404 Not Found Page
  notFound: {
    description: '您访问的页面不存在或已被移动。',
    backHome: '返回首页',
    goBack: '返回上一页',
  },

  // Setup Wizard
  setup: {
    title: 'Sub2API 安装向导',
    description: '配置您的 Sub2API 实例',
    database: {
      title: '数据库配置',
      description: '连接到您的 PostgreSQL 数据库',
      host: '主机',
      port: '端口',
      username: '用户名',
      password: '密码',
      databaseName: '数据库名称',
      sslMode: 'SSL 模式',
      passwordPlaceholder: '密码',
      ssl: {
        disable: '禁用',
        require: '要求',
        verifyCa: '验证 CA',
        verifyFull: '完全验证'
      }
    },
    redis: {
      title: 'Redis 配置',
      description: '连接到您的 Redis 服务器',
      host: '主机',
      port: '端口',
      username: '用户名（可选）',
      password: '密码（可选）',
      database: '数据库',
      usernamePlaceholder: '默认用户留空',
      passwordPlaceholder: '密码',
      enableTls: '启用 TLS',
      enableTlsHint: '连接 Redis 时使用 TLS（公共 CA 证书）'
    },
    admin: {
      title: '管理员账户',
      description: '创建您的管理员账户',
      email: '邮箱',
      password: '密码',
      confirmPassword: '确认密码',
      passwordPlaceholder: '至少 8 个字符',
      confirmPasswordPlaceholder: '确认密码',
      passwordMismatch: '密码不匹配'
    },
    ready: {
      title: '准备安装',
      description: '检查您的配置并完成安装',
      database: '数据库',
      redis: 'Redis',
      adminEmail: '管理员邮箱'
    },
    status: {
      testing: '测试中...',
      success: '连接成功',
      testConnection: '测试连接',
      installing: '安装中...',
      completeInstallation: '完成安装',
      completed: '安装完成！',
      redirecting: '正在跳转到登录页面...',
      restarting: '服务正在重启，请稍候...',
      timeout: '服务重启时间超出预期，请手动刷新页面。'
    }
  },

  // Common
}
