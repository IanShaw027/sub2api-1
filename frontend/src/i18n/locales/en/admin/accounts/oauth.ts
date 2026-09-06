export default {
  // OAuth flow
  oauth: {
    title: 'Claude Account Authorization',
    authMethod: 'Authorization Method',
    manualAuth: 'Manual Authorization',
    cookieAutoAuth: 'Cookie Auto-Auth',
    cookieAutoAuthDesc:
      'Use claude.ai sessionKey to automatically complete OAuth authorization without manually opening browser.',
    sessionKey: 'sessionKey',
    keysCount: '{count} keys',
    batchCreateAccounts: 'Will batch create {count} accounts',
    sessionKeyPlaceholder:
      'One sessionKey per line, e.g.:\nsk-ant-sid01-xxxxx...\nsk-ant-sid01-yyyyy...',
    sessionKeyPlaceholderSingle: 'sk-ant-sid01-xxxxx...',
    howToGetSessionKey: 'How to get sessionKey',
    step1: 'Login to claude.ai in your browser',
    step2: 'Press F12 to open Developer Tools',
    step3: 'Go to Application tab',
    step4: 'Find Cookies → https://claude.ai',
    step5: 'Find the row with key sessionKey',
    step6: 'Copy the Value',
    sessionKeyFormat: 'sessionKey usually starts with sk-ant-sid01-',
    startAutoAuth: 'Start Auto-Auth',
    authorizing: 'Authorizing...',
    followSteps: 'Follow these steps to authorize your Claude account:',
    step1GenerateUrl: 'Click the button below to generate the authorization URL',
    generateAuthUrl: 'Generate Auth URL',
    generating: 'Generating...',
    regenerate: 'Regenerate',
    step2OpenUrl: 'Open the URL in your browser and complete authorization',
    openUrlDesc:
      'Open the authorization URL in a new tab, log in to your Claude account and authorize.',
    proxyWarning:
      'Note: If you configured a proxy, make sure your browser uses the same proxy to access the authorization page.',
    step3EnterCode: 'Enter the Authorization Code',
    authCodeDesc:
      'After authorization is complete, the page will display an Authorization Code. Copy and paste it below:',
    authCode: 'Authorization Code',
    authCodePlaceholder: 'Paste the Authorization Code from Claude page...',
    authCodeHint: 'Paste the Authorization Code copied from the Claude page',
    completeAuth: 'Complete Authorization',
    verifying: 'Verifying...',
    pleaseEnterSessionKey: 'Please enter at least one valid sessionKey',
    authFailed: 'Authorization failed',
    cookieAuthFailed: 'Cookie authorization failed',
    keyAuthFailed: 'Key {index}: {error}',
    successCreated: 'Successfully created {count} account(s)',
    batchSuccess: 'Successfully created {count} account(s)',
    batchPartialSuccess: 'Partial success: {success} succeeded, {failed} failed',
    batchFailed: 'Batch creation failed',
    // OpenAI specific
    openai: {
      title: 'OpenAI Account Authorization',
      followSteps: 'Follow these steps to complete OpenAI account authorization:',
      step1GenerateUrl: 'Click the button below to generate the authorization URL',
      generateAuthUrl: 'Generate Auth URL',
      step2OpenUrl: 'Open the URL in your browser and complete authorization',
      openUrlDesc:
        'Open the authorization URL in a new tab, log in to your OpenAI account and authorize.',
      importantNotice:
        'Important: The page may take a while to load after authorization. Please wait patiently. When the browser address bar changes to http://localhost..., the authorization is complete.',
      step3EnterCode: 'Enter Authorization URL or Code',
      authCodeDesc:
        'After authorization is complete, when the page URL becomes http://localhost:xxx/auth/callback?code=...:',
      authCode: 'Authorization URL or Code',
      authCodePlaceholder:
        'Option 1: Copy the complete URL\n(http://localhost:xxx/auth/callback?code=...)\nOption 2: Copy only the code parameter value',
      authCodeHint:
        'You can copy the entire URL or just the code parameter value, the system will auto-detect',
      failedToGenerateUrl: 'Failed to generate OpenAI auth URL',
      failedToExchangeCode: 'Failed to exchange OpenAI auth code',
      failedToValidateRT: 'Failed to validate refresh token',
      errors: {
        OPENAI_OAUTH_PROXY_REQUIRED:
          'No proxy is configured and this server could not reach OpenAI directly, so the OpenAI OAuth request failed. Select a proxy that can access OpenAI and retry; if the authorization code has expired, regenerate the authorization URL.'
      },
      // Refresh Token auth
      refreshTokenAuth: 'Manual RT Input',
      refreshTokenDesc: 'Enter your existing OpenAI Refresh Token(s). Supports batch input (one per line). The system will automatically validate and create accounts.',
      refreshTokenPlaceholder: 'Paste your OpenAI Refresh Token...\nSupports multiple, one per line',
      mobileRefreshTokenAuth: 'Manual Mobile RT Input',
      accessTokenAuth: 'Manual AT Input',
      codexSessionAuth: 'Codex OAuth auth.json / AT Import',
      codexSessionDesc: 'Paste a Codex OAuth auth.json or an accessToken. Accounts use the step 1 settings.',
      codexSessionInputLabel: 'Codex OAuth auth.json or accessToken',
      codexSessionPlaceholder: 'Multiple lines supported, one token or auth.json object per line',
      codexSessionHint: 'OAuth session/access-token imports retain their existing expiration behavior.',
      codexSessionImportAndCreate: 'Import & Create Account',
      codexSessionEmpty: 'Please enter a Codex auth.json or accessToken',
      codexSessionImportFailed: 'Failed to import Codex account',
      codexSessionImportSuccess: 'Import completed: created {created}, updated {updated}, skipped {skipped}',
      codexSessionImportPartial: 'Partial success: created {created}, updated {updated}, skipped {skipped}, failed {failed}',
      agentIdentityAuth: 'Agent Identity auth.json',
      agentIdentityDesc: 'Import a Codex Agent Identity auth.json. No OAuth access or refresh token is stored.',
      agentIdentityInputLabel: 'Agent Identity auth.json',
      agentIdentityPlaceholder: 'Paste one Agent Identity auth.json object',
      agentIdentityHint: 'The file must use auth_mode=agentIdentity. Upstream requests are signed dynamically.',
      agentIdentityInvalid: 'Use a Codex auth.json with auth_mode=agentIdentity.',
      codexPatAuth: 'Codex Personal Access Token',
      codexPatDesc: 'Enter a Codex at- personal access token. The system validates it with OpenAI whoami before creating the account.',
      codexPatInputLabel: 'Codex PAT',
      codexPatPlaceholder: 'at-...',
      codexPatHint: 'This is a separate auth mode. It does not save refresh_token or write an OAuth access_token expiration.',
      codexPatImportAndCreate: 'Validate & Create Codex PAT Account',
      codexPatEmpty: 'Please enter a Codex personal access token',
      codexPatImportFailed: 'Failed to create Codex PAT account',
      sessionTokenAuth: 'Manual ST Input',
      sessionTokenDesc: 'Enter your existing Session Token(s). Supports batch input (one per line). The system will automatically validate and create accounts.',
      sessionTokenPlaceholder: 'Paste your Session Token...\nSupports multiple, one per line',
      sessionTokenRawLabel: 'Raw Input',
      sessionTokenRawPlaceholder: 'Paste /api/auth/session raw payload or Session Token...',
      sessionTokenRawHint: 'You can paste full JSON. The system will auto-parse ST and AT.',
      openSessionUrl: 'Open Fetch URL',
      copySessionUrl: 'Copy URL',
      sessionUrlHint: 'This URL usually returns AT. If sessionToken is absent, copy __Secure-next-auth.session-token from browser cookies as ST.',
      parsedSessionTokensLabel: 'Parsed ST',
      parsedSessionTokensEmpty: 'No ST parsed. Please check your input.',
      parsedAccessTokensLabel: 'Parsed AT',
      validating: 'Validating...',
      validateAndCreate: 'Validate & Create Account',
      pleaseEnterRefreshToken: 'Please enter Refresh Token',
      pleaseEnterSessionToken: 'Please enter Session Token'
    },
    grok: {
      title: 'Grok Account Authorization',
      followSteps: 'Follow these steps to authorize your xAI/Grok account:',
      step1GenerateUrl: 'Generate the xAI authorization URL',
      generateAuthUrl: 'Generate Auth URL',
      step2OpenUrl: 'Open the URL in your browser and complete authorization',
      openUrlDesc: 'Open the authorization URL in a new tab, sign in to xAI, and authorize API access.',
      importantNotice: 'When the browser reaches the local callback URL, copy the full URL or the code query parameter back here.',
      step3EnterCode: 'Enter Authorization URL or Code',
      authCodeDesc: 'After authorization, paste the callback URL, query string, or authorization code:',
      authCode: 'Authorization URL or Code',
      authCodePlaceholder: 'Paste the full callback URL, ?code=... query string, or code value',
      authCodeHint: 'Full callback URLs, query strings, and bare codes are accepted.',
      refreshTokenAuth: 'Manual RT Input',
      refreshTokenDesc: 'Enter existing xAI refresh token(s). Supports batch input, one per line.',
      refreshTokenPlaceholder: 'Paste your xAI refresh token...\nSupports multiple, one per line',
      ssoCookieAuth: 'SSO Cookie Import',
      ssoCookieDesc: 'Paste one Grok Web SSO key per line. The server will complete the xAI Device Flow and convert them into Grok Build OAuth credentials.',
      ssoCookieLabel: 'Grok Web SSO Key',
      ssoCookiePlaceholder: 'One SSO key per line\nSupports multiple, one per line',
      ssoCookieHint: 'One SSO key per line. Multiple keys are imported with 3-way concurrency; expect about 90 seconds per batch. Use a matching-region proxy if needed.',
      emailPasswordAuth: 'Email + password',
      emailPasswordDesc:
        'Sign in with a Grok web email and password. The server uses the password only to obtain an ephemeral SSO cookie, then converts it to Build OAuth credentials. Neither the password nor raw SSO is stored on the account.',
      emailPasswordInputLabel: 'email----password',
      emailPasswordPlaceholder: "user{'@'}example.com----your-password\nMultiple lines supported",
      emailPasswordHint:
        'Format: email----password (password may contain -). Requires YesCaptcha keys; use a matching-region proxy when needed.',
      pleaseEnterPassword: 'Please enter email----password (one per line)',
      pleaseEnterSSOToken: 'Please enter an SSO token',
      failedToValidateSSO: 'Failed to validate Grok SSO',
      failedToAuthorizePassword: 'Grok password authorization failed',
      convertingSSO: 'Converting...',
      convertSSOAndCreate: 'Convert & Create Account',
      validating: 'Validating...',
      validateAndCreate: 'Validate & Create Account',
      pleaseEnterRefreshToken: 'Please enter Refresh Token',
      failedToGenerateUrl: 'Failed to generate Grok auth URL',
      missingExchangeParams: 'Missing authorization code, state, or OAuth session',
      failedToExchangeCode: 'Failed to exchange Grok authorization code',
      failedToValidateRT: 'Failed to validate Grok refresh token',
      failedToConvertSSO: 'Failed to convert Grok SSO cookie',
      errors: {
        GROK_OAUTH_SESSION_NOT_FOUND:
          'Grok OAuth session was not found or has expired. Generate a new auth URL and paste the newest callback URL.',
        GROK_OAUTH_INVALID_STATE:
          'Grok OAuth state does not match this session. Paste the callback URL from the same generated auth link.',
        GROK_OAUTH_STATE_REQUIRED:
          'The callback URL is missing the OAuth state. Paste the full callback URL, not only the code.',
        GROK_OAUTH_CODE_REQUIRED:
          'The Grok authorization code is missing. Paste the full callback URL, query string, or code value.',
        GROK_OAUTH_NO_REFRESH_TOKEN:
          'The Grok response did not include a refresh token. Generate a new auth URL and approve offline access again.',
        GROK_OAUTH_PROXY_NOT_AVAILABLE:
          'Grok OAuth proxy lookup is unavailable. Check the selected proxy and retry.',
        GROK_OAUTH_PROXY_NOT_FOUND:
          'The selected proxy could not be found. Choose an available proxy and retry.'
      },
      oauthOnlyHint: 'Initial Grok support is OAuth subscription-backed Responses API text and reasoning traffic only.'
    },
    // Gemini specific
	        gemini: {
	          title: 'Gemini Account Authorization',
	          followSteps: 'Follow these steps to authorize your Gemini account:',
	          step1GenerateUrl: 'Generate the authorization URL',
	          generateAuthUrl: 'Generate Auth URL',
	          projectIdLabel: 'Project ID (optional)',
	          projectIdPlaceholder: 'e.g. my-gcp-project or cloud-ai-companion-xxxxx',
	          projectIdHint:
	            'Leave empty to auto-detect after code exchange. If auto-detection fails, fill it in and re-generate the auth URL to try again.',
	          howToGetProjectId: 'How to get',
	          step2OpenUrl: 'Open the URL in your browser and complete authorization',
	          openUrlDesc:
	            'Open the authorization URL in a new tab, log in to your Google account and authorize.',
	          step3EnterCode: 'Enter Authorization URL or Code',
	          authCodeDesc:
	            'After authorization, copy the callback URL (recommended) or just the code and paste it below.',
	          authCode: 'Callback URL or Code',
	          authCodePlaceholder:
	            'Option 1 (recommended): Paste the callback URL\nOption 2: Paste only the code value',
	          authCodeHint: 'The system will auto-extract code/state from the URL.',
      redirectUri: 'Redirect URI',
      redirectUriHint:
        'This must be configured in your Google OAuth client and must match exactly.',
      confirmRedirectUri:
        'I have configured this Redirect URI in the Google OAuth client (must match exactly)',
	          invalidRedirectUri: 'Redirect URI must be a valid http(s) URL',
	          redirectUriNotConfirmed: 'Please confirm the Redirect URI is configured correctly',
	          missingRedirectUri: 'Missing redirect URI',
	          failedToGenerateUrl: 'Failed to generate Gemini auth URL',
	          missingExchangeParams: 'Missing auth code, session ID, or state',
	          failedToExchangeCode: 'Failed to exchange Gemini auth code',
	          missingProjectId: 'GCP Project ID retrieval failed: Your Google account is not linked to an active GCP project. Please activate GCP and bind a credit card in Google Cloud Console, or manually enter the Project ID during authorization.',
	          modelPassthrough: 'Gemini Model Passthrough',
	          modelPassthroughDesc:
	            'All model requests are forwarded directly to the Gemini API without model restrictions or mappings.',
	          stateWarningTitle: 'Note',
	          stateWarningDesc: 'Recommended: paste the full callback URL (includes code & state).',
	          oauthTypeLabel: 'OAuth Type',
      needsProjectId: 'Built-in OAuth (Code Assist)',
      needsProjectIdDesc: 'Requires GCP project and Project ID',
      noProjectIdNeeded: 'Custom OAuth (AI Studio)',
      noProjectIdNeededDesc: 'Requires admin-configured OAuth client',
	          aiStudioNotConfiguredShort: 'Not configured',
	          aiStudioNotConfiguredTip:
	            'AI Studio OAuth is not configured: set GEMINI_OAUTH_CLIENT_ID / GEMINI_OAUTH_CLIENT_SECRET and add Redirect URI: http://localhost:1455/auth/callback (Consent screen scopes must include https://www.googleapis.com/auth/generative-language.retriever)',
	          aiStudioNotConfigured:
	            'AI Studio OAuth is not configured: set GEMINI_OAUTH_CLIENT_ID / GEMINI_OAUTH_CLIENT_SECRET and add Redirect URI: http://localhost:1455/auth/callback'
	        },
    // Antigravity specific
    antigravity: {
      title: 'Antigravity Account Authorization',
      followSteps: 'Follow these steps to authorize your Antigravity account:',
      step1GenerateUrl: 'Generate the authorization URL',
      generateAuthUrl: 'Generate Auth URL',
      step2OpenUrl: 'Open the URL in your browser and complete authorization',
      openUrlDesc: 'Open the authorization URL in a new tab, log in to your Google account and authorize.',
      importantNotice:
        'Important: The page may take a while to load after authorization. Please wait patiently. When the browser address bar shows http://localhost..., authorization is complete.',
      step3EnterCode: 'Enter Authorization URL or Code',
      authCodeDesc:
        'After authorization, when the page URL becomes http://localhost:xxx/auth/callback?code=...:',
      authCode: 'Authorization URL or Code',
      authCodePlaceholder:
        'Option 1: Copy the complete URL\n(http://localhost:xxx/auth/callback?code=...)\nOption 2: Copy only the code parameter value',
                authCodeHint: 'You can copy the entire URL or just the code parameter value, the system will auto-detect',
                failedToGenerateUrl: 'Failed to generate Antigravity auth URL',
                missingExchangeParams: 'Missing code, session ID, or state',
                failedToExchangeCode: 'Failed to exchange Antigravity auth code',
                // Refresh Token auth
                refreshTokenAuth: 'Manual RT',
                refreshTokenDesc: 'Enter your existing Antigravity Refresh Token. Supports batch input (one per line). The system will automatically validate and create accounts.',
                refreshTokenPlaceholder: 'Paste your Antigravity Refresh Token...\nSupports multiple tokens, one per line',
                validating: 'Validating...',
                validateAndCreate: 'Validate & Create',
                pleaseEnterRefreshToken: 'Please enter Refresh Token',
                failedToValidateRT: 'Failed to validate Refresh Token'
              }
            },      // Gemini specific (platform-wide)
  gemini: {
    helpButton: 'Help',
    helpDialog: {
      title: 'Gemini Usage Guide',
      apiKeySection: 'API Key Links'
    },
    modelPassthrough: 'Gemini Model Passthrough',
    modelPassthroughDesc:
      'All model requests are forwarded directly to the Gemini API without model restrictions or mappings.',
    baseUrlHint: 'Leave default for official Gemini API',
    apiKeyHint: 'Your Gemini API Key (starts with AIza)',
    tier: {
      label: 'Account Tier',
      hint: 'Tip: The system will try to auto-detect the tier first; if auto-detection is unavailable or fails, your selected tier is used as a fallback (simulated quota).',
      aiStudioHint:
        'AI Studio quotas are per-model (Pro/Flash are limited independently). If billing is enabled, choose Pay-as-you-go.',
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
      oauthTitle: 'OAuth (Gemini)',
      oauthDesc: 'Authorize with your Google account and choose an OAuth type.',
      apiKeyTitle: 'API Key (AI Studio)',
      apiKeyDesc: 'Fastest setup. Use an AIza API key.',
      apiKeyNote:
        'Best for light testing. Free tier has strict rate limits and data may be used for training.',
      apiKeyLink: 'Get API Key',
      quotaLink: 'Quota guide'
    },
    oauthType: {
      builtInTitle: 'Built-in OAuth (Gemini CLI / Code Assist)',
      builtInDesc: 'Uses Google built-in client ID. No admin configuration required.',
      builtInRequirement: 'Requires a GCP project and Project ID.',
      googleOneDesc: 'Personal account with Google One subscription quota',
      codeAssistDesc: 'Enterprise-grade, requires a GCP project',
      codeAssistRequirement: 'Requires an active GCP project with billing enabled',
      showAdvanced: 'Show advanced options (custom OAuth Client)',
      hideAdvanced: 'Hide advanced options (custom OAuth Client)',
      gcpProjectLink: 'Create project',
      customTitle: 'Custom OAuth (AI Studio OAuth)',
      customDesc: 'Uses admin-configured OAuth client for org management.',
      customRequirement: 'Admin must configure Client ID and add you as a test user.',
      badges: {
        recommended: 'Recommended',
        highConcurrency: 'High concurrency',
        individuals: 'Recommended for individuals',
        noGcp: 'No GCP required',
        enterprise: 'Enterprise users',
        noAdmin: 'No admin setup',
        orgManaged: 'Org managed',
        adminRequired: 'Admin required'
      }
    },
    setupGuide: {
      title: 'Gemini Setup Checklist',
      checklistTitle: 'Checklist',
      checklistItems: {
        usIp: 'Use a US IP and ensure your account country is set to US.',
        age: 'Account must be 18+.'
      },
      activationTitle: 'One-click Activation',
      activationItems: {
        geminiWeb: 'Activate Gemini Web to avoid User not initialized.',
        gcpProject: 'Activate a GCP project and get the Project ID for Code Assist.'
      },
      links: {
        countryCheck: 'Check country association',
        countryChange: 'Change country association',
        geminiWebActivation: 'Activate Gemini Web',
        gcpProject: 'Open GCP Console'
      }
    },
    quotaPolicy: {
      title: 'Gemini Quota & Limit Policy (Reference)',
      note: 'Note: Gemini does not provide an official quota inquiry API. The "Daily Quota" shown here is an estimate simulated by the system based on account tiers for scheduling reference only. Please refer to official Google errors for actual limits.',
      columns: {
        channel: 'Auth Channel',
        account: 'Account Status',
        limits: 'Limit Policy',
        docs: 'Official Docs'
      },
      docs: {
        codeAssist: 'Code Assist Quotas',
        aiStudio: 'AI Studio Pricing',
        vertex: 'Vertex AI Quotas'
      },
      simulatedNote: 'Simulated quota, for reference only',
      rows: {
        googleOne: {
          channel: 'Google One OAuth (Individuals / Code Assist for Individuals)',
          limitsFree: 'Shared pool: 1000 RPD / 60 RPM',
          limitsPro: 'Shared pool: 1500 RPD / 120 RPM',
          limitsUltra: 'Shared pool: 2000 RPD / 120 RPM'
        },
        gcp: {
          channel: 'GCP Code Assist OAuth (Enterprise)',
          limitsStandard: 'Shared pool: 1500 RPD / 120 RPM',
          limitsEnterprise: 'Shared pool: 2000 RPD / 120 RPM'
        },
        cli: {
          channel: 'Gemini CLI (Official Google Login / Code Assist)',
          free: 'Free Google Account',
          premium: 'Google One AI Premium',
          limitsFree: 'RPD ~1000; RPM ~60 (soft)',
          limitsPremium: 'RPD ~1500+; RPM ~60+ (priority queue)'
        },
        gcloud: {
          channel: 'GCP Code Assist (gcloud auth)',
          account: 'No Code Assist subscription',
          limits: 'RPD ~1000; RPM ~60 (preview)'
        },
        aiStudio: {
          channel: 'AI Studio API Key / OAuth',
          free: 'No billing (free tier)',
          paid: 'Billing enabled (pay-as-you-go)',
          limitsFree: 'RPD 50; RPM 2 (Pro) / 15 (Flash)',
          limitsPaid: 'RPD unlimited; RPM 1000 (Pro) / 2000 (Flash) (per model)'
        },
        customOAuth: {
          channel: 'Custom OAuth Client (GCP)',
          free: 'Project not billed',
          paid: 'Project billed',
          limitsFree: 'RPD 50; RPM 2 (project quota)',
          limitsPaid: 'RPD unlimited; RPM 1000+ (project quota)'
        }
      }
    },
    rateLimit: {
      ok: 'Not rate limited',
      unlimited: 'Unlimited',
      limited: 'Rate limited {time}',
      now: 'now'
    }
  },
  // Re-Auth Modal
  reAuthorizeAccount: 'Re-Authorize Account',
  claudeCodeAccount: 'Claude Code Account',
  openaiAccount: 'OpenAI Account',
  geminiAccount: 'Gemini Account',
  antigravityAccount: 'Antigravity Account',
  kiroAccount: 'Kiro Account',
  grokAccount: 'Grok Account',
  inputMethod: 'Input Method',
  reAuthorizeUnavailableKiro:
    'Kiro accounts do not support the browser-based admin re-authorization flow. Try refreshing the stored refresh token first, or update credentials in the account editor.',
  reAuthorizedSuccess: 'Account re-authorized successfully',
  // Test Modal
  testAccountConnection: 'Test Account Connection',
  errorPrefix: 'Error: {message}',
  imagePreviewAlt: 'Test image {index}',
  imageLightboxAlt: 'Image preview',
  account: 'Account',
  readyToTest: 'Ready to test. Click "Start Test" to begin...',
  connectingToApi: 'Connecting to API...',
  testCompleted: 'Test completed successfully!',
  testFailed: 'Test failed',
  connectedToApi: 'Connected to API',
  usingModel: 'Using model: {model}',
  sendingTestMessage: 'Sending test message: "hi"',
  sendingImageRequest: 'Sending image generation test request...',
  response: 'Response:',
  startTest: 'Start Test',
  testing: 'Testing...',
  retry: 'Retry',
  copyOutput: 'Copy output',
  outputCopied: 'Output copied',
  startingTestForAccount: 'Starting test for account: {name}',
  testAccountTypeLabel: 'Account type: {type}',
  selectTestModel: 'Select Test Model',
  testModel: 'Test model',
  testPrompt: 'Prompt: "hi"',
  imagePromptLabel: 'Image prompt',
  imagePromptPlaceholder: 'Example: Generate an orange cat astronaut sticker in pixel-art style on a solid background.',
  imagePromptDefault: 'Generate a cute orange cat astronaut sticker on a clean pastel background.',
  imageTestHint:
    'Calls standalone /v1/images/generations and shows the returned image below.',
  imageTestMode: 'Mode: Image generation test',
  videoPromptLabel: 'Video prompt',
  videoPromptPlaceholder: 'Example: A red ball bouncing once on a white floor, short simple motion.',
  videoPromptDefault: 'A red ball bouncing once on a white floor, short simple motion.',
  videoTestHint:
    'Calls standalone /v1/videos/generations, polls until done, then downloads the finished video for on-page preview.',
  videoTestMode: 'Mode: Video generation test',
  sendingVideoRequest: 'Sending video generation request...',
  imagePreview: 'Generated images:',
  imageReceived: 'Received test image #{count}',
  audioPreview: 'Generated audio:',
  audioReceived: 'Received test audio #{count}',
  videoPreview: 'Generated video:',
  videoReceived: 'Received test video #{count}',
  // Stats Modal
  viewStats: 'View Stats',
  usageStatistics: 'Usage Statistics',
  last30DaysUsage: 'Last 30 days usage statistics (based on actual usage days)',
  stats: {
    totalCost: '30-Day Total Cost',
    accumulatedCost: 'Accumulated cost',
    standardCost: 'Standard',
    totalRequests: '30-Day Total Requests',
    totalCalls: 'Total API calls',
    avgDailyCost: 'Daily Avg Cost',
    basedOnActualDays: 'Based on {days} actual usage days',
    avgDailyRequests: 'Daily Avg Requests',
    avgDailyUsage: 'Average daily usage',
    todayOverview: 'Today Overview',
    cost: 'Cost',
    requests: 'Requests',
    requestsUnit: 'reqs',
    userBilledShort: 'user cost',
    tokens: 'Tokens',
    highestCostDay: 'Highest Cost Day',
    highestRequestDay: 'Highest Request Day',
    date: 'Date',
    accumulatedTokens: 'Accumulated Tokens',
    totalTokens: '30-Day Total',
    dailyAvgTokens: 'Daily Average',
    performance: 'Performance',
    avgResponseTime: 'Avg Response',
    daysActive: 'Days Active',
    recentActivity: 'Recent Activity',
    todayRequests: 'Today Requests',
    todayTokens: 'Today Tokens',
    todayCost: 'Today Cost',
    usageTrend: '30-Day Cost & Request Trend',
    noData: 'No usage data available for this account'
  },
  usageWindow: {
    statsTitle: '5-Hour Window Usage Statistics',
    statsTitleDaily: 'Daily Usage Statistics',
    geminiProDaily: 'Pro',
    geminiFlashDaily: 'Flash',
    gemini3Pro: 'G3P',
    gemini3Flash: 'G3F',
    gemini3Image: 'G31FI',
    claude: 'Claude',
    grokRequests: 'Req',
    grokTokens: 'Tok',
    grokFreeQuota24hHint: 'Estimated from local token usage over the rolling 24-hour window ({limit} limit)',
    grokWeeklyUsage: 'Weekly {percent}%',
    grokUsed: 'Used $',
    grokBalance: 'Bal $',
    grokPrepaid: 'Prepaid balance',
    grokMonthlyLimit: 'Monthly used / limit (USD)',
    grokOverage: 'Overage onDemandUsed/onDemandCap',
    grokOverageShort: 'OD $',
    grokUnknown: 'Grok quota is unknown until the first upstream response includes xAI rate-limit headers.',
    grokRetryAfter: 'Retry after {time}',
    grokProbe: 'Probe',
    grokProbeTooltip: 'Send a minimal xAI Responses probe and read quota headers',
    grokResetUnsupported: 'Reset unsupported',
    grokResetUnsupportedTooltip: 'xAI does not expose reset credits for Grok OAuth accounts',
    grokNoHeaders: 'No quota headers observed',
    grokLastStatus: 'Status {status}',
    grokLastProbe: 'Probe {time}',
    grokLastHeadersSeen: 'Headers {time}',
    passiveSampled: 'Passive',
    activeQuery: 'Query'
  },
  openaiQuotaReset: {
    count: 'Credits',
    reset: 'Reset',
    countTooltipLoad: 'Click to load the available reset-credit count',
    countTooltipRefresh: 'Click to refresh the available reset-credit count',
    resetTooltipReady: 'Consume 1 reset credit to immediately restore the window',
    resetTooltipNeedQuery: 'Click Credits first to load the available count',
    resetTooltipNoCredits: 'No reset credits available',
    resetTooltipShadow: 'Spark shadow accounts cannot reset credits; reset on the parent account',
    expiresAt: 'Expires {time}',
    expiresAtFull: 'Reset credit expires at {time}',
    expandExpirations: 'Expand the other {count} reset credit expiration(s)',
    collapseExpirations: 'Collapse reset credit expirations',
    expirationDetails: 'Reset credit expiration details',
    noCreditsAvailable: 'No reset credits available',
    resetSuccess: 'Reset {windows} window(s); credits and account state updated',
    resetCacheRefreshFailed: 'The window was reset and account state recovered, but the reset-credit count could not be read back. Query it again.',
    resetAccountRecoveryFailed: 'The window was reset, but account state recovery failed. Recover the account state manually.',
    resetAccountRefreshFailed: 'The window, account state, and reset-credit cache were updated, but the latest account display could not be loaded.',
    refreshCachePersistFailed: 'Showing the live count, but its expiration details were unavailable, so the cached details were kept.',
    autoStatus: {
      checking: 'Checking',
      available: 'Credit available',
      resetting: 'Auto-resetting',
      success: 'Auto-reset succeeded',
      noCredit: 'No credit',
      failed: 'Auto-reset failed'
    },
    confirmTitle: 'Confirm Weekly Limit Reset',
    confirmMessage: 'This will consume 1 reset credit to immediately restore the current window ({count} remaining). This action cannot be undone. Continue?'
  },
  tier: {
    free: 'Free',
    pro: 'Pro',
    ultra: 'Ultra',
    aiPremium: 'AI Premium',
    standard: 'Standard',
    basic: 'Basic',
    personal: 'Personal',
    unlimited: 'Unlimited'
  },
  ineligibleWarning:
    'This account is not eligible for Antigravity, but API forwarding still works. Use at your own risk.',
  forbidden: 'Forbidden',
  forbiddenValidation: 'Verification Required',
  forbiddenViolation: 'Violation Ban',
  openVerification: 'Open Verification Link',
  copyLink: 'Copy Link',
  linkCopied: 'Link Copied',
  needsReauth: 'Re-auth Required',
  rateLimited: 'Rate Limited',
  usageError: 'Fetch Error'
}
