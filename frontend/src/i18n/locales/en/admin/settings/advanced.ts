export default {

  // Error Passthrough Rules
  errorPassthrough: {
    title: 'Error Passthrough Rules',
    description: 'Configure how upstream errors are returned to clients',
    createRule: 'Create Rule',
    editRule: 'Edit Rule',
    deleteRule: 'Delete Rule',
    noRules: 'No rules configured',
    createFirstRule: 'Create your first error passthrough rule',
    allPlatforms: 'All Platforms',
    passthrough: 'Passthrough',
    custom: 'Custom',
    code: 'Code',
    body: 'Body',
    skipMonitoring: 'Skip Monitoring',

    // Columns
    columns: {
      priority: 'Priority',
      name: 'Name',
      conditions: 'Conditions',
      platforms: 'Platforms',
      behavior: 'Behavior',
      status: 'Status',
      actions: 'Actions'
    },

    // Match Mode
    matchMode: {
      any: 'Code OR Keyword',
      all: 'Code AND Keyword',
      anyHint: 'Status code matches any error code, OR message contains any keyword',
      allHint: 'Status code matches any error code, AND message contains any keyword'
    },

    // Form
    form: {
      name: 'Rule Name',
      namePlaceholder: 'e.g., Context Limit Passthrough',
      priority: 'Priority',
      priorityHint: 'Lower values have higher priority',
      description: 'Description',
      descriptionPlaceholder: 'Describe the purpose of this rule...',
      matchConditions: 'Match Conditions',
      errorCodes: 'Error Codes',
      errorCodesPlaceholder: '422, 400, 429',
      errorCodesHint: 'Separate multiple codes with commas',
      keywords: 'Keywords',
      keywordsPlaceholder: 'One keyword per line\ncontext limit\nmodel not supported',
      keywordsHint: 'One keyword per line, case-insensitive',
      matchMode: 'Match Mode',
      platforms: 'Platforms',
      platformsHint: 'Leave empty to apply to all platforms',
      responseBehavior: 'Response Behavior',
      passthroughCode: 'Passthrough upstream status code',
      responseCode: 'Custom status code',
      passthroughBody: 'Passthrough upstream error message',
      customMessage: 'Custom error message',
      customMessagePlaceholder: 'Error message to return to client...',
      skipMonitoring: 'Skip monitoring',
      skipMonitoringHint: 'When enabled, errors matching this rule will not be recorded in ops monitoring',
      enabled: 'Enable this rule'
    },

    // Messages
    nameRequired: 'Please enter rule name',
    conditionsRequired: 'Please configure at least one error code or keyword',
    ruleCreated: 'Rule created successfully',
    ruleUpdated: 'Rule updated successfully',
    ruleDeleted: 'Rule deleted successfully',
    deleteConfirm: 'Are you sure you want to delete rule "{name}"?',
    failedToLoad: 'Failed to load rules',
    failedToSave: 'Failed to save rule',
    failedToDelete: 'Failed to delete rule',
    failedToToggle: 'Failed to toggle status'
  },

  // TLS Fingerprint Profiles
  tlsFingerprintProfiles: {
    title: 'TLS Fingerprint Profiles',
    description: 'Manage TLS fingerprint profiles for simulating specific client TLS handshake characteristics',
    createProfile: 'Create Profile',
    editProfile: 'Edit Profile',
    deleteProfile: 'Delete Profile',
    noProfiles: 'No profiles configured',
    createFirstProfile: 'Create your first TLS fingerprint profile',

    columns: {
      name: 'Name',
      description: 'Description',
      grease: 'GREASE',
      alpn: 'ALPN',
      actions: 'Actions'
    },

    form: {
      pasteYaml: 'Paste YAML Configuration',
      pasteYamlPlaceholder: 'Paste YAML output from TLS Fingerprint Collector here...',
      pasteYamlHint: 'Paste the YAML copied from TLS Fingerprint Collector to auto-fill all fields.',
      openCollector: 'Open Collector',
      parseYaml: 'Parse YAML',
      yamlParsed: 'YAML parsed successfully, fields auto-filled',
      yamlParseFailed: 'Failed to parse YAML: name field not found',
      name: 'Profile Name',
      namePlaceholder: 'e.g. macOS Node.js v24',
      description: 'Description',
      descriptionPlaceholder: 'Optional description for this profile',
      enableGrease: 'Enable GREASE',
      enableGreaseHint: 'Insert GREASE values in TLS ClientHello extensions',
      cipherSuites: 'Cipher Suites',
      cipherSuitesHint: 'Comma-separated hex values, e.g. 0x1301, 0x1302, 0xc02c',
      curves: 'Elliptic Curves',
      curvesHint: 'Comma-separated curve IDs',
      pointFormats: 'Point Formats',
      signatureAlgorithms: 'Signature Algorithms',
      alpnProtocols: 'ALPN Protocols',
      alpnProtocolsHint: 'Comma-separated, e.g. h2, http/1.1',
      supportedVersions: 'Supported TLS Versions',
      keyShareGroups: 'Key Share Groups',
      pskModes: 'PSK Modes',
      extensions: 'Extensions'
    },

    deleteConfirm: 'Delete Profile',
    deleteConfirmMessage: 'Are you sure you want to delete profile "{name}"? Accounts using this profile will fall back to the built-in default.',
    createSuccess: 'Profile created successfully',
    updateSuccess: 'Profile updated successfully',
    deleteSuccess: 'Profile deleted successfully',
    loadFailed: 'Failed to load profiles',
    saveFailed: 'Failed to save profile',
    deleteFailed: 'Failed to delete profile'
  },
  tlsFingerprintRouters: {
    title: 'TLS Fingerprint Routers',
    description: 'Route fingerprints by OS, client, protocol, or User-Agent. Conditions inside a rule are AND; rules are first-match-wins. A rule with only a profile is the unique-fingerprint fallback.',
    create: 'Create router',
    empty: 'No routers yet',
    rules: 'rules',
    name: 'Router name',
    descriptionField: 'Description (optional)',
    enabled: 'Enabled',
    ruleName: 'Rule name',
    noProfile: 'No profile (use account bindings)',
    anyOS: 'Any OS',
    client: 'Client, e.g. codex-cli',
    protocol: 'Protocol, e.g. responses / messages',
    uaPattern: 'Optional UA pattern',
    ruleEnabled: 'Rule enabled',
    transport: 'Transport',
    anyTransport: 'Any transport',
    matchType: 'UA match type',
    caseSensitive: 'Case sensitive',
    upstreamUserAgent: 'Upstream User-Agent (optional)',
    upstreamOriginator: 'Upstream Originator (optional)',
    addRule: 'Add rule',
    removeRule: 'Remove rule',
    deleteRouter: 'Delete router',
    deleteConfirmMessage: 'Delete router "{name}"? Accounts still pointing at it will stop matching until you pick another router.',
    loadFailed: 'Failed to load routers',
    saveSuccess: 'Router saved',
    saveFailed: 'Failed to save router',
    deleteSuccess: 'Router deleted',
    deleteFailed: 'Failed to delete router'
  }
}
