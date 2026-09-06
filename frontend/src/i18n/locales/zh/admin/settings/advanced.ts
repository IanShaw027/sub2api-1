export default {

  // Error Passthrough Rules
  errorPassthrough: {
    title: '错误透传规则',
    description: '配置上游错误如何返回给客户端',
    createRule: '创建规则',
    editRule: '编辑规则',
    deleteRule: '删除规则',
    noRules: '暂无规则',
    createFirstRule: '创建第一条错误透传规则',
    allPlatforms: '所有平台',
    passthrough: '透传',
    custom: '自定义',
    code: '状态码',
    body: '消息体',
    skipMonitoring: '跳过监控',

    // Columns
    columns: {
      priority: '优先级',
      name: '名称',
      conditions: '匹配条件',
      platforms: '平台',
      behavior: '响应行为',
      status: '状态',
      actions: '操作'
    },

    // Match Mode
    matchMode: {
      any: '错误码 或 关键词',
      all: '错误码 且 关键词',
      anyHint: '状态码匹配任一错误码，或消息包含任一关键词',
      allHint: '状态码匹配任一错误码，且消息包含任一关键词'
    },

    // Form
    form: {
      name: '规则名称',
      namePlaceholder: '例如：上下文超限透传',
      priority: '优先级',
      priorityHint: '数值越小优先级越高，优先匹配',
      description: '规则描述',
      descriptionPlaceholder: '描述此规则的用途...',
      matchConditions: '匹配条件',
      errorCodes: '错误码',
      errorCodesPlaceholder: '422, 400, 429',
      errorCodesHint: '多个错误码用逗号分隔',
      keywords: '关键词',
      keywordsPlaceholder: '每行一个关键词\ncontext limit\nmodel not supported',
      keywordsHint: '每行一个关键词，不区分大小写',
      matchMode: '匹配模式',
      platforms: '适用平台',
      platformsHint: '不选择表示适用于所有平台',
      responseBehavior: '响应行为',
      passthroughCode: '透传上游状态码',
      responseCode: '自定义状态码',
      passthroughBody: '透传上游错误信息',
      customMessage: '自定义错误信息',
      customMessagePlaceholder: '返回给客户端的错误信息...',
      skipMonitoring: '跳过运维监控记录',
      skipMonitoringHint: '开启后，匹配此规则的错误不会被记录到运维监控中',
      enabled: '启用此规则'
    },

    // Messages
    nameRequired: '请输入规则名称',
    conditionsRequired: '请至少配置一个错误码或关键词',
    ruleCreated: '规则创建成功',
    ruleUpdated: '规则更新成功',
    ruleDeleted: '规则删除成功',
    deleteConfirm: '确定要删除规则 "{name}" 吗？',
    failedToLoad: '加载规则失败',
    failedToSave: '保存规则失败',
    failedToDelete: '删除规则失败',
    failedToToggle: '切换状态失败'
  },

  // TLS 指纹模板
  tlsFingerprintProfiles: {
    title: 'TLS 指纹模板',
    description: '管理 TLS 指纹模板，用于模拟特定客户端的 TLS 握手特征',
    createProfile: '创建模板',
    editProfile: '编辑模板',
    deleteProfile: '删除模板',
    noProfiles: '暂无模板',
    createFirstProfile: '创建你的第一个 TLS 指纹模板',

    columns: {
      name: '名称',
      description: '描述',
      grease: 'GREASE',
      alpn: 'ALPN',
      actions: '操作'
    },

    form: {
      pasteYaml: '粘贴 YAML 配置',
      pasteYamlPlaceholder: '将 TLS 指纹采集器复制的 YAML 粘贴到这里...',
      pasteYamlHint: '粘贴从 TLS 指纹采集器复制的 YAML 配置，自动填充所有字段。',
      openCollector: '打开采集器',
      parseYaml: '解析 YAML',
      yamlParsed: 'YAML 解析成功，字段已自动填充',
      yamlParseFailed: 'YAML 解析失败：未找到 name 字段',
      name: '模板名称',
      namePlaceholder: '例如 macOS Node.js v24',
      description: '描述',
      descriptionPlaceholder: '可选的模板描述',
      enableGrease: '启用 GREASE',
      enableGreaseHint: '在 TLS ClientHello 扩展中插入 GREASE 值',
      cipherSuites: '密码套件',
      cipherSuitesHint: '逗号分隔的十六进制值，例如 0x1301, 0x1302, 0xc02c',
      curves: '椭圆曲线',
      curvesHint: '逗号分隔的曲线 ID',
      pointFormats: '点格式',
      signatureAlgorithms: '签名算法',
      alpnProtocols: 'ALPN 协议',
      alpnProtocolsHint: '逗号分隔，例如 h2, http/1.1',
      supportedVersions: '支持的 TLS 版本',
      keyShareGroups: '密钥共享组',
      pskModes: 'PSK 模式',
      extensions: '扩展'
    },

    deleteConfirm: '删除模板',
    deleteConfirmMessage: '确定要删除模板 "{name}" 吗？使用此模板的账号将回退到内置默认值。',
    createSuccess: '模板创建成功',
    updateSuccess: '模板更新成功',
    deleteSuccess: '模板删除成功',
    loadFailed: '加载模板失败',
    saveFailed: '保存模板失败',
    deleteFailed: '删除模板失败'
  },
  tlsFingerprintRouters: {
    title: 'TLS 指纹路由',
    description: '按操作系统、客户端、协议或 User-Agent 选择指纹。规则内条件为 AND，规则之间先匹配先生效。不填条件只填模板即唯一指纹。',
    create: '创建路由',
    empty: '暂无路由',
    rules: '条规则',
    name: '路由名称',
    descriptionField: '描述（可选）',
    enabled: '启用',
    ruleName: '规则名称',
    noProfile: '不指定模板（走账号绑定）',
    anyOS: '任意操作系统',
    client: '客户端，如 codex-cli',
    protocol: '协议，如 responses / messages',
    uaPattern: '可选 UA 匹配',
    ruleEnabled: '启用规则',
    transport: '传输',
    anyTransport: '任意传输',
    matchType: 'UA 匹配方式',
    caseSensitive: '区分大小写',
    upstreamUserAgent: '上游 User-Agent（可选）',
    upstreamOriginator: '上游 Originator（可选）',
    addRule: '添加规则',
    removeRule: '删除规则',
    deleteRouter: '删除路由',
    deleteConfirmMessage: '确定删除路由“{name}”？仍指向它的账号在改绑之前不会再命中规则。',
    loadFailed: '加载路由失败',
    saveSuccess: '路由已保存',
    saveFailed: '保存路由失败',
    deleteSuccess: '路由已删除',
    deleteFailed: '删除路由失败'
  }
}
