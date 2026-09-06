export default {
  // OAuth flow
  oauth: {
    title: 'Claude 账号授权',
    authMethod: '授权方式',
    manualAuth: '手动授权',
    cookieAutoAuth: 'Cookie 自动授权',
    cookieAutoAuthDesc: '使用 claude.ai sessionKey 自动完成 OAuth 授权，无需手动打开浏览器。',
    sessionKey: 'sessionKey',
    keysCount: '{count} 个密钥',
    batchCreateAccounts: '将批量创建 {count} 个账号',
    sessionKeyPlaceholder:
      '每行一个 sessionKey，例如：\nsk-ant-sid01-xxxxx...\nsk-ant-sid01-yyyyy...',
    sessionKeyPlaceholderSingle: 'sk-ant-sid01-xxxxx...',
    howToGetSessionKey: '如何获取 sessionKey',
    step1: '在浏览器中登录 claude.ai',
    step2: '按 F12 打开开发者工具',
    step3: '切换到 Application 标签',
    step4: '找到 Cookies → https://claude.ai',
    step5: '找到 sessionKey 所在行',
    step6: '复制 Value 列的值',
    sessionKeyFormat: 'sessionKey 通常以 sk-ant-sid01- 开头',
    startAutoAuth: '开始自动授权',
    authorizing: '授权中...',
    followSteps: '按照以下步骤授权您的 Claude 账号：',
    step1GenerateUrl: '点击下方按钮生成授权 URL',
    generateAuthUrl: '生成授权 URL',
    generating: '生成中...',
    regenerate: '重新生成',
    step2OpenUrl: '在浏览器中打开 URL 并完成授权',
    openUrlDesc: '在新标签页中打开授权 URL，登录您的 Claude 账号并授权。',
    proxyWarning: '注意：如果您配置了代理，请确保浏览器使用相同的代理访问授权页面。',
    step3EnterCode: '输入授权码',
    authCodeDesc: '授权完成后，页面会显示一个授权码。复制并粘贴到下方：',
    authCode: '授权码',
    authCodePlaceholder: '粘贴 Claude 页面的授权码...',
    authCodeHint: '粘贴从 Claude 页面复制的授权码',
    completeAuth: '完成授权',
    verifying: '验证中...',
    pleaseEnterSessionKey: '请输入至少一个有效的 sessionKey',
    authFailed: '授权失败',
    cookieAuthFailed: 'Cookie 授权失败',
    keyAuthFailed: '密钥 {index}: {error}',
    successCreated: '成功创建 {count} 个账号',
    batchSuccess: '成功创建 {count} 个账号',
    batchPartialSuccess: '部分成功：{success} 个成功，{failed} 个失败',
    batchFailed: '批量创建失败',
    // OpenAI specific
    openai: {
      title: 'OpenAI 账户授权',
      followSteps: '请按照以下步骤完成 OpenAI 账户的授权：',
      step1GenerateUrl: '点击下方按钮生成授权链接',
      generateAuthUrl: '生成授权链接',
      step2OpenUrl: '在浏览器中打开链接并完成授权',
      openUrlDesc: '请在新标签页中打开授权链接，登录您的 OpenAI 账户并授权。',
      importantNotice:
        '重要提示：授权后页面可能会加载较长时间，请耐心等待。当浏览器地址栏变为 http://localhost... 开头时，表示授权已完成。',
      step3EnterCode: '输入授权链接或 Code',
      authCodeDesc:
        '授权完成后，当页面地址变为 http://localhost:xxx/auth/callback?code=... 时：',
      authCode: '授权链接或 Code',
      authCodePlaceholder:
        '方式1：复制完整的链接\n(http://localhost:xxx/auth/callback?code=...)\n方式2：仅复制 code 参数的值',
      authCodeHint: '您可以直接复制整个链接或仅复制 code 参数值，系统会自动识别',
      failedToGenerateUrl: '生成 OpenAI 授权链接失败',
      failedToExchangeCode: 'OpenAI 授权码兑换失败',
      failedToValidateRT: '验证 Refresh Token 失败',
      errors: {
        OPENAI_OAUTH_PROXY_REQUIRED:
          '未设置代理，当前服务器无法直连 OpenAI，导致 OpenAI OAuth 请求失败。请先选择可访问 OpenAI 的代理后重试；如果授权码已失效，请重新生成授权链接。'
      },
      // Refresh Token auth
      refreshTokenAuth: '手动输入 RT',
      refreshTokenDesc: '输入您已有的 OpenAI Refresh Token，支持批量输入（每行一个），系统将自动验证并创建账号。',
      refreshTokenPlaceholder: '粘贴您的 OpenAI Refresh Token...\n支持多个，每行一个',
      mobileRefreshTokenAuth: '手动输入 Mobile RT',
      accessTokenAuth: '手动输入 AT',
      codexSessionAuth: 'Codex OAuth auth.json / AT 导入',
      codexSessionDesc: '粘贴 Codex OAuth auth.json 或 accessToken，按第一步配置创建账号。',
      codexSessionInputLabel: 'Codex OAuth auth.json 或 accessToken',
      codexSessionPlaceholder: '支持多行，每行一个 token 或 auth.json 对象',
      codexSessionHint: 'OAuth session/accessToken 导入继续沿用原有过期规则。',
      codexSessionImportAndCreate: '导入并创建账号',
      codexSessionEmpty: '请输入 Codex auth.json 或 accessToken',
      codexSessionImportFailed: 'Codex 账号导入失败',
      codexSessionImportSuccess: '导入完成：新增 {created}，更新 {updated}，跳过 {skipped}',
      codexSessionImportPartial: '部分成功：新增 {created}，更新 {updated}，跳过 {skipped}，失败 {failed}',
      agentIdentityAuth: 'Agent Identity auth.json',
      agentIdentityDesc: '导入 Codex Agent Identity auth.json，不保存 OAuth access token 或 refresh token。',
      agentIdentityInputLabel: 'Agent Identity auth.json',
      agentIdentityPlaceholder: '粘贴一个 Agent Identity auth.json 对象',
      agentIdentityHint: '文件必须使用 auth_mode=agentIdentity；每次上游请求都会动态签名。',
      agentIdentityInvalid: '请选择 auth_mode=agentIdentity 的 Codex auth.json。',
      codexPatAuth: 'Codex Personal Access Token',
      codexPatDesc: '输入 Codex at- Personal Access Token，系统会先调用 OpenAI whoami 校验后再创建账号。',
      codexPatInputLabel: 'Codex PAT',
      codexPatPlaceholder: 'at-...',
      codexPatHint: '这是独立认证模式，不保存 refresh_token，也不会写入 OAuth access_token 过期时间。',
      codexPatImportAndCreate: '校验并创建 Codex PAT 账号',
      codexPatEmpty: '请输入 Codex Personal Access Token',
      codexPatImportFailed: 'Codex PAT 账号创建失败',
      sessionTokenAuth: '手动输入 ST',
      sessionTokenDesc: '输入您已有的 Session Token，支持批量输入（每行一个），系统将自动验证并创建账号。',
      sessionTokenPlaceholder: '粘贴您的 Session Token...\n支持多个，每行一个',
      sessionTokenRawLabel: '原始字符串',
      sessionTokenRawPlaceholder: '粘贴 /api/auth/session 原始数据或 Session Token...',
      sessionTokenRawHint: '支持粘贴完整 JSON，系统会自动解析 ST 和 AT。',
      openSessionUrl: '打开获取链接',
      copySessionUrl: '复制链接',
      sessionUrlHint: '该链接通常可获取 AT。若返回中无 sessionToken，请从浏览器 Cookie 复制 __Secure-next-auth.session-token 作为 ST。',
      parsedSessionTokensLabel: '解析出的 ST',
      parsedSessionTokensEmpty: '未解析到 ST，请检查输入内容',
      parsedAccessTokensLabel: '解析出的 AT',
      validating: '验证中...',
      validateAndCreate: '验证并创建账号',
      pleaseEnterRefreshToken: '请输入 Refresh Token',
      pleaseEnterSessionToken: '请输入 Session Token'
    },
    grok: {
      title: 'Grok 账号授权',
      followSteps: '请按照以下步骤授权您的 xAI/Grok 账号：',
      step1GenerateUrl: '生成 xAI 授权链接',
      generateAuthUrl: '生成授权链接',
      step2OpenUrl: '在浏览器中打开链接并完成授权',
      openUrlDesc: '在新标签页中打开授权链接，登录 xAI 并授权 API 访问。',
      importantNotice: '当浏览器跳转到本地 callback URL 后，请复制完整 URL 或 code 参数回填到这里。',
      step3EnterCode: '输入授权链接或 Code',
      authCodeDesc: '授权完成后，粘贴 callback URL、查询字符串或授权码：',
      authCode: '授权链接或 Code',
      authCodePlaceholder: '粘贴完整 callback URL、?code=... 查询字符串或 code 值',
      authCodeHint: '支持完整 callback URL、查询字符串或裸 code。',
      refreshTokenAuth: '手动输入 RT',
      refreshTokenDesc: '输入已有的 xAI refresh token，支持批量输入（每行一个）。',
      refreshTokenPlaceholder: '粘贴您的 xAI refresh token...\n支持多个，每行一个',
      ssoCookieAuth: 'SSO Cookie 导入',
      ssoCookieDesc: '每行粘贴一个 Grok Web SSO key，系统会自动走 xAI Device Flow 并转换为 Grok Build OAuth 凭据。',
      ssoCookieLabel: 'Grok Web SSO Key',
      ssoCookiePlaceholder: '每行一个 SSO key\n支持多个，每行一个',
      ssoCookieHint: '每行一个 SSO key；多个 key 会 3 路并发导入，耗时约 90 秒 × 批次数，建议使用对应地区代理。',
      emailPasswordAuth: '邮箱密码登录',
      emailPasswordDesc:
        '使用 Grok 网页邮箱与密码登录。服务端仅用密码换取临时 SSO 再转 Build OAuth；密码与 raw SSO 均不会写入账号凭据。',
      emailPasswordInputLabel: '邮箱----密码',
      emailPasswordPlaceholder: "user{'@'}example.com----your-password\n支持多个，每行一组",
      emailPasswordHint: '格式：email----password（密码可含 -）。需要配置 YesCaptcha 密钥；建议搭配代理。',
      pleaseEnterPassword: '请输入 email----password（每行一组）',
      pleaseEnterSSOToken: '请输入 SSO Token',
      failedToValidateSSO: '校验 Grok SSO 失败',
      failedToAuthorizePassword: 'Grok 密码授权失败',
      convertingSSO: '转换中...',
      convertSSOAndCreate: '转换并创建账号',
      validating: '验证中...',
      validateAndCreate: '验证并创建账号',
      pleaseEnterRefreshToken: '请输入 Refresh Token',
      failedToGenerateUrl: '生成 Grok 授权链接失败',
      missingExchangeParams: '缺少授权码、state 或 OAuth 会话',
      failedToExchangeCode: 'Grok 授权码兑换失败',
      failedToValidateRT: '验证 Grok refresh token 失败',
      failedToConvertSSO: 'Grok SSO 转换失败',
      errors: {
        GROK_OAUTH_SESSION_NOT_FOUND:
          'Grok OAuth 会话不存在或已过期。请重新生成授权链接，并粘贴最新的回调链接。',
        GROK_OAUTH_INVALID_STATE:
          'Grok OAuth state 与当前会话不匹配。请粘贴同一次生成的授权链接返回的回调 URL。',
        GROK_OAUTH_STATE_REQUIRED:
          '回调链接缺少 OAuth state。请粘贴完整 callback URL，不要只粘贴 code。',
        GROK_OAUTH_CODE_REQUIRED:
          '缺少 Grok 授权码。请粘贴完整 callback URL、查询字符串或 code 值。',
        GROK_OAUTH_NO_REFRESH_TOKEN:
          'Grok 响应未返回 refresh token。请重新生成授权链接，并再次确认 offline access 授权。',
        GROK_OAUTH_PROXY_NOT_AVAILABLE:
          '无法查询 Grok OAuth 代理配置。请检查选择的代理后重试。',
        GROK_OAUTH_PROXY_NOT_FOUND:
          '找不到所选代理。请选择可用代理后重试。'
      },
      oauthOnlyHint: '首版 Grok 支持仅包含 OAuth 订阅的 Responses API 文本/推理转发。'
    },
    // Gemini specific
    gemini: {
      title: 'Gemini 账户授权',
      followSteps: '请按照以下步骤完成 Gemini 账户的授权：',
      step1GenerateUrl: '生成授权链接',
      generateAuthUrl: '生成授权链接',
      projectIdLabel: 'Project ID（可选）',
      projectIdPlaceholder: '例如：my-gcp-project 或 cloud-ai-companion-xxxxx',
      projectIdHint:
        '留空则在兑换授权码后自动探测；若自动探测失败，可填写后重新生成授权链接再授权。',
      howToGetProjectId: '如何获取',
      step2OpenUrl: '在浏览器中打开链接并完成授权',
      openUrlDesc: '请在新标签页中打开授权链接，登录您的 Google 账户并授权。',
      step3EnterCode: '输入回调链接或 Code',
      authCodeDesc:
        '授权完成后，复制浏览器跳转后的回调链接（推荐）或仅复制 code，粘贴到下方即可。',
      authCode: '回调链接或 Code',
      authCodePlaceholder: '方式1（推荐）：粘贴回调链接\n方式2：仅粘贴 code 参数的值',
      authCodeHint: '系统会自动从链接中解析 code/state。',
      redirectUri: 'Redirect URI',
      redirectUriHint: '需要在 Google OAuth Client 中配置，且必须与此处完全一致。',
      confirmRedirectUri: '我已在 Google OAuth Client 中配置了该 Redirect URI（必须完全一致）',
      invalidRedirectUri: 'Redirect URI 必须是合法的 http(s) URL',
      redirectUriNotConfirmed: '请确认 Redirect URI 已在 Google OAuth Client 中正确配置',
      missingRedirectUri: '缺少 Redirect URI',
      failedToGenerateUrl: '生成 Gemini 授权链接失败',
      missingExchangeParams: '缺少 code / session_id / state',
      failedToExchangeCode: 'Gemini 授权码兑换失败',
      missingProjectId:
        'GCP Project ID 获取失败：您的 Google 账号未关联有效的 GCP 项目。请前往 Google Cloud Console 激活 GCP 并绑定信用卡，或在授权时手动填写 Project ID。',
      modelPassthrough: 'Gemini 直接转发模型',
      modelPassthroughDesc: '所有模型请求将直接转发至 Gemini API，不进行模型限制或映射。',
      stateWarningTitle: '提示',
      stateWarningDesc: '建议粘贴完整回调链接（包含 code 和 state）。',
      oauthTypeLabel: 'OAuth 类型',
      needsProjectId: '内置授权（Code Assist）',
      needsProjectIdDesc: '需要 GCP 项目与 Project ID',
      noProjectIdNeeded: '自定义授权（AI Studio）',
      noProjectIdNeededDesc: '需管理员配置 OAuth Client',
      aiStudioNotConfiguredShort: '未配置',
      aiStudioNotConfiguredTip:
        'AI Studio OAuth 未配置：请先设置 GEMINI_OAUTH_CLIENT_ID / GEMINI_OAUTH_CLIENT_SECRET，并在 Google OAuth Client 添加 Redirect URI：http://localhost:1455/auth/callback（Consent Screen scopes 需包含 https://www.googleapis.com/auth/generative-language.retriever）',
      aiStudioNotConfigured:
        'AI Studio OAuth 未配置：请先设置 GEMINI_OAUTH_CLIENT_ID / GEMINI_OAUTH_CLIENT_SECRET，并在 Google OAuth Client 添加 Redirect URI：http://localhost:1455/auth/callback'
    },
    // Antigravity specific
    antigravity: {
      title: 'Antigravity 账户授权',
      followSteps: '请按照以下步骤完成 Antigravity 账户的授权：',
      step1GenerateUrl: '生成授权链接',
      generateAuthUrl: '生成授权链接',
      step2OpenUrl: '在浏览器中打开链接并完成授权',
      openUrlDesc: '请在新标签页中打开授权链接，登录您的 Google 账户并授权。',
      importantNotice:
        '重要提示：授权后页面可能会加载较长时间，请耐心等待。当浏览器地址栏变为 http://localhost... 开头时，表示授权已完成。',
      step3EnterCode: '输入授权链接或 Code',
      authCodeDesc:
        '授权完成后，当页面地址变为 http://localhost:xxx/auth/callback?code=... 时：',
      authCode: '授权链接或 Code',
      authCodePlaceholder:
        '方式1：复制完整的链接\n(http://localhost:xxx/auth/callback?code=...)\n方式2：仅复制 code 参数的值',
      authCodeHint: '您可以直接复制整个链接或仅复制 code 参数值，系统会自动识别',
      failedToGenerateUrl: '生成 Antigravity 授权链接失败',
      missingExchangeParams: '缺少 code / session_id / state',
      failedToExchangeCode: 'Antigravity 授权码兑换失败',
      // Refresh Token auth
      refreshTokenAuth: '手动输入 RT',
      refreshTokenDesc: '输入您已有的 Antigravity Refresh Token，支持批量输入（每行一个），系统将自动验证并创建账号。',
      refreshTokenPlaceholder: '粘贴您的 Antigravity Refresh Token...\n支持多个，每行一个',
      validating: '验证中...',
      validateAndCreate: '验证并创建账号',
      pleaseEnterRefreshToken: '请输入 Refresh Token',
      failedToValidateRT: '验证 Refresh Token 失败'
    }
  },
  // Gemini specific (platform-wide)
  gemini: {
    helpButton: '使用帮助',
    helpDialog: {
      title: 'Gemini 使用指南',
      apiKeySection: 'API Key 相关链接'
    },
    modelPassthrough: 'Gemini 直接转发模型',
    modelPassthroughDesc: '所有模型请求将直接转发至 Gemini API，不进行模型限制或映射。',
    baseUrlHint: '留空使用官方 Gemini API',
    apiKeyHint: '您的 Gemini API Key（以 AIza 开头）',
    tier: {
      label: '账号等级',
      hint: '提示：系统会优先尝试自动识别账号等级；若自动识别不可用或失败，则使用你选择的等级作为回退（本地模拟配额）。',
      aiStudioHint:
        'AI Studio 的配额是按模型分别限流（Pro/Flash 独立）。若已绑卡（按量付费），请选 Pay-as-you-go。',
      googleOne: {
        free: 'Google One Free',
        pro: 'Google One Pro',
        ultra: 'Google One Ultra'
      },
      gcp: {
        standard: 'GCP Standard',
        enterprise: 'GCP Enterprise'
      },
      aiStudio: {
        free: 'Google AI Free',
        paid: 'Google AI Pay-as-you-go'
      }
    },
    accountType: {
      oauthTitle: 'OAuth 授权（Gemini）',
      oauthDesc: '使用 Google 账号授权，并选择 OAuth 子类型。',
      apiKeyTitle: 'API 密钥（AI Studio）',
      apiKeyDesc: '最快接入方式，使用 AIza API Key。',
      apiKeyNote: '适合轻量测试。免费层限流严格，数据可能用于训练。',
      apiKeyLink: '获取 API Key',
      quotaLink: '配额说明'
    },
    oauthType: {
      builtInTitle: '内置授权（Gemini CLI / Code Assist）',
      builtInDesc: '使用 Google 内置客户端 ID，无需管理员配置。',
      builtInRequirement: '需要 GCP 项目并填写 Project ID。',
      googleOneDesc: '个人账号，享受 Google One 订阅配额',
      codeAssistDesc: '企业级，需要 GCP 项目',
      codeAssistRequirement: '需要激活 GCP 项目并绑定信用卡',
      showAdvanced: '显示高级选项（自建 OAuth Client）',
      hideAdvanced: '隐藏高级选项（自建 OAuth Client）',
      gcpProjectLink: '创建项目',
      customTitle: '自定义授权（AI Studio OAuth）',
      customDesc: '使用管理员预设的 OAuth 客户端，适合组织管理。',
      customRequirement: '需管理员配置 Client ID 并加入测试用户白名单。',
      badges: {
        recommended: '推荐',
        highConcurrency: '高并发',
        individuals: '推荐个人用户',
        noGcp: '无需 GCP',
        enterprise: '企业用户',
        noAdmin: '无需管理员配置',
        orgManaged: '组织管理',
        adminRequired: '需要管理员'
      }
    },
    setupGuide: {
      title: 'Gemini 使用准备',
      checklistTitle: '准备工作',
      checklistItems: {
        usIp: '使用美国 IP，并确保账号归属地为美国。',
        age: '账号需满 18 岁。'
      },
      activationTitle: '服务激活',
      activationItems: {
        geminiWeb: '激活 Gemini Web，避免 User not initialized。',
        gcpProject: '激活 GCP 项目，获取 Code Assist 所需 Project ID。'
      },
      links: {
        countryCheck: '检查归属地',
        countryChange: '修改归属地',
        geminiWebActivation: '激活 Gemini Web',
        gcpProject: '打开 GCP 控制台'
      }
    },
    quotaPolicy: {
      title: 'Gemini 配额与限流政策（参考）',
      note: '注意：Gemini 官方未提供用量查询接口。此处显示的“每日配额”是由系统根据账号等级模拟计算的估算值，仅供调度参考，请以 Google 官方实际报错为准。',
      columns: {
        channel: '授权通道',
        account: '账号状态',
        limits: '限流政策',
        docs: '官方文档'
      },
      docs: {
        codeAssist: 'Code Assist 配额',
        aiStudio: 'AI Studio 定价',
        vertex: 'Vertex AI 配额'
      },
      simulatedNote: '本地模拟配额，仅供参考',
      rows: {
        googleOne: {
          channel: 'Google One OAuth（个人版 / Code Assist for Individuals）',
          limitsFree: '共享池：1000 RPD / 60 RPM（不分模型）',
          limitsPro: '共享池：1500 RPD / 120 RPM（不分模型）',
          limitsUltra: '共享池：2000 RPD / 120 RPM（不分模型）'
        },
        gcp: {
          channel: 'GCP Code Assist OAuth（企业版）',
          limitsStandard: '共享池：1500 RPD / 120 RPM（不分模型）',
          limitsEnterprise: '共享池：2000 RPD / 120 RPM（不分模型）'
        },
        cli: {
          channel: 'Gemini CLI（官方 Google 登录 / Code Assist）',
          free: '免费 Google 账号',
          premium: 'Google One AI Premium',
          limitsFree: 'RPD ~1000；RPM ~60（软限制）',
          limitsPremium: 'RPD ~1500+；RPM ~60+（优先队列）'
        },
        gcloud: {
          channel: 'GCP Code Assist（gcloud 登录）',
          account: '未购买 Code Assist 订阅',
          limits: 'RPD ~1000；RPM ~60（预览期）'
        },
        aiStudio: {
          channel: 'AI Studio API Key / OAuth',
          free: '未绑卡（免费层）',
          paid: '已绑卡（按量付费）',
          limitsFree: 'RPD 50；RPM 2（Pro）/ 15（Flash）',
          limitsPaid: 'RPD 不限；RPM 1000（Pro）/ 2000（Flash）（按模型配额）'
        },
        customOAuth: {
          channel: 'Custom OAuth Client（GCP）',
          free: '项目未绑卡',
          paid: '项目已绑卡',
          limitsFree: 'RPD 50；RPM 2（项目配额）',
          limitsPaid: 'RPD 不限；RPM 1000+（项目配额）'
        }
      }
    },
    rateLimit: {
      ok: '未限流',
      unlimited: '无限流',
      limited: '限流 {time}',
      now: '现在'
    }
  },
  // Re-Auth Modal
  reAuthorizeAccount: '重新授权账号',
  claudeCodeAccount: 'Claude Code 账号',
  openaiAccount: 'OpenAI 账号',
  geminiAccount: 'Gemini 账号',
  antigravityAccount: 'Antigravity 账号',
  kiroAccount: 'Kiro 账号',
  grokAccount: 'Grok 账号',
  inputMethod: '输入方式',
  reAuthorizeUnavailableKiro:
    'Kiro 账号不支持浏览器式的管理端重新授权流程。你可以先尝试刷新已保存的 refresh token，失败后再到编辑页手动更新凭据。',
  reAuthorizedSuccess: '账号重新授权成功',
  // Test Modal
  testAccountConnection: '测试账号连接',
  errorPrefix: '错误：{message}',
  imagePreviewAlt: '测试图片 {index}',
  imageLightboxAlt: '图片预览',
  account: '账号',
  readyToTest: '准备测试。点击"开始测试"按钮开始...',
  connectingToApi: '连接 API 中...',
  testCompleted: '测试完成！',
  connectedToApi: '已连接到 API',
  usingModel: '使用模型：{model}',
  sendingTestMessage: '发送测试消息："hi"',
  sendingImageRequest: '发送生图测试请求...',
  response: '响应：',
  startTest: '开始测试',
  retry: '重试',
  copyOutput: '复制输出',
  outputCopied: '输出已复制',
  startingTestForAccount: '开始测试账号：{name}',
  testAccountTypeLabel: '账号类型：{type}',
  selectTestModel: '选择测试模型',
  testModel: '测试模型',
  testPrompt: '提示词："hi"',
  imagePromptLabel: '生图提示词',
  imagePromptPlaceholder: '例如：生成一只戴宇航员头盔的橘猫，像素插画风格，纯色背景。',
  imagePromptDefault: 'Generate a cute orange cat astronaut sticker on a clean pastel background.',
  imageTestHint:
    '调用独立 /v1/images/generations 生图，并在下方预览返回图片。',
  imageTestMode: '模式：生图测试',
  videoPromptLabel: '视频提示词',
  videoPromptPlaceholder: '例如：一只红球在白地板上弹跳一次，动作简短。',
  videoPromptDefault: 'A red ball bouncing once on a white floor, short simple motion.',
  videoTestHint:
    '调用独立 /v1/videos/generations，轮询至完成后下载成品视频并在页面上预览。',
  videoTestMode: '模式：视频生成测试',
  sendingVideoRequest: '正在发送视频生成测试请求...',
  imagePreview: '生成结果：',
  imageReceived: '已收到第 {count} 张测试图片',
  audioPreview: '生成音频：',
  audioReceived: '已收到第 {count} 段测试音频',
  videoPreview: '生成视频：',
  videoReceived: '已收到第 {count} 段测试视频',
  // Stats Modal
  viewStats: '查看统计',
  usageStatistics: '使用统计',
  last30DaysUsage: '近30天使用统计（日均基于实际使用天数）',
  stats: {
    totalCost: '30天总费用',
    accumulatedCost: '累计成本',
    standardCost: '标准计费',
    totalRequests: '30天总请求',
    totalCalls: '累计调用次数',
    avgDailyCost: '日均费用',
    basedOnActualDays: '基于 {days} 天实际使用',
    avgDailyRequests: '日均请求',
    avgDailyUsage: '平均每日调用',
    todayOverview: '今日概览',
    cost: '费用',
    requests: '请求',
    requestsUnit: '次',
    userBilledShort: '用户费用',
    tokens: 'Token',
    highestCostDay: '最高费用日',
    highestRequestDay: '最高请求日',
    date: '日期',
    accumulatedTokens: '累计 Token',
    totalTokens: '30天总计',
    dailyAvgTokens: '日均 Token',
    performance: '性能',
    avgResponseTime: '平均响应',
    daysActive: '活跃天数',
    recentActivity: '最近统计',
    todayRequests: '今日请求',
    todayTokens: '今日 Token',
    todayCost: '今日费用',
    usageTrend: '30天费用与请求趋势',
    noData: '该账号暂无使用数据'
  }
}
