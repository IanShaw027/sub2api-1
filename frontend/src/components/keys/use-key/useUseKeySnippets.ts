import { computed, type ComputedRef, type Ref } from 'vue'
import type { GroupPlatform } from '@/types'
import {
  findCodexCatalogModel,
  formatCodexReasoningEffortTomlLine,
  parseCodexCatalogModels,
  selectCodexConfigReasoningEffort
} from '@/utils/codexCatalogConfig'
import type { CodexAuthMode, FileConfig } from './types'
import {
  antigravityGeminiModels,
  claudeModels,
  geminiModels,
  grokModels,
  openaiModels
} from './opencodeModelCatalogs'

export interface UseUseKeySnippetsProps {
  baseUrl: string
  apiKey: string
  platform: GroupPlatform | null
}

export interface UseUseKeySnippetsOptions {
  props: UseUseKeySnippetsProps
  activeTab: Ref<string>
  activeClientTab: Ref<string>
  codexAuthMode: Ref<CodexAuthMode>
  codexModelManifestContent: Ref<string>
  t: (key: string) => string
}

export interface UseUseKeySnippetsResult {
  currentFiles: ComputedRef<FileConfig[]>
  codexModelCatalogPath: ComputedRef<string>
}

// Syntax highlighting helpers (pure, no reactive dependency).
const escapeHtml = (value: string) => value
  .replace(/&/g, '&amp;')
  .replace(/</g, '&lt;')
  .replace(/>/g, '&gt;')
  .replace(/"/g, '&quot;')
  .replace(/'/g, '&#39;')

const wrapToken = (className: string, value: string) =>
  `<span class="${className}">${escapeHtml(value)}</span>`

const keyword = (value: string) => wrapToken('text-success-300', value)
const variable = (value: string) => wrapToken('text-accent-200', value)
const operator = (value: string) => wrapToken('text-muted', value)
const string = (value: string) => wrapToken('text-warning-200', value)
const comment = (value: string) => wrapToken('text-muted', value)

/**
 * Builds the per-platform/per-tool instruction file snippets (env vars, config.toml,
 * settings.json, opencode.json, ...) shown in UseKeyModal's code blocks.
 */
export function useUseKeySnippets(options: UseUseKeySnippetsOptions): UseUseKeySnippetsResult {
  const { props, activeTab, activeClientTab, codexAuthMode, codexModelManifestContent, t } = options

const codexModelCatalogPath = computed(() => {
 const isWindows = activeTab.value === 'windows'
 const configDir = isWindows ? '%userprofile%\\.codex' : '~/.codex'
 return joinConfigPath(configDir, 'codex-models.json', isWindows)
})
const codexCatalogModelSlugs = computed(() =>
 parseCodexCatalogModels(codexModelManifestContent.value).map((model) => model.slug)
)

function selectCodexCatalogModel(preferredModel: string): string {
 if (codexCatalogModelSlugs.value.includes(preferredModel)) return preferredModel
 return codexCatalogModelSlugs.value[0] || preferredModel
}

function codexReasoningEffortTomlLine(modelSlug: string): string {
 return formatCodexReasoningEffortTomlLine(
 selectCodexConfigReasoningEffort(findCodexCatalogModel(codexModelManifestContent.value, modelSlug))
 )
}
const currentFiles = computed((): FileConfig[] => {
 const baseUrl = props.baseUrl || window.location.origin
 const apiKey = props.apiKey
 const baseRoot = baseUrl.replace(/\/v1\/?$/, '').replace(/\/+$/, '')
 const ensureV1 = (value: string) => {
 const trimmed = value.replace(/\/+$/, '')
 return trimmed.endsWith('/v1') ? trimmed : `${trimmed}/v1`
 }
 const apiBase = ensureV1(baseRoot)
 const antigravityBase = ensureV1(`${baseRoot}/antigravity`)
 const antigravityGeminiBase = (() => {
 const trimmed = `${baseRoot}/antigravity`.replace(/\/+$/, '')
 return trimmed.endsWith('/v1beta') ? trimmed : `${trimmed}/v1beta`
 })()
 const geminiBase = (() => {
 const trimmed = baseRoot.replace(/\/+$/, '')
 return trimmed.endsWith('/v1beta') ? trimmed : `${trimmed}/v1beta`
 })()

 if (activeClientTab.value === 'opencode') {
 switch (props.platform) {
 case 'anthropic':
 return [generateOpenCodeConfig('anthropic', apiBase, apiKey)]
 case 'openai':
 return [generateOpenCodeConfig('openai', apiBase, apiKey)]
 case 'gemini':
 return [generateOpenCodeConfig('gemini', geminiBase, apiKey)]
 case 'antigravity':
 return [
 generateOpenCodeConfig('antigravity-claude', antigravityBase, apiKey, 'opencode.json (Claude)'),
 generateOpenCodeConfig('antigravity-gemini', antigravityGeminiBase, apiKey, 'opencode.json (Gemini)')
 ]
 case 'grok':
 return [generateOpenCodeConfig('grok', apiBase, apiKey)]
 default:
 return [generateOpenCodeConfig('openai', apiBase, apiKey)]
 }
 }

 switch (props.platform) {
 case 'openai':
 if (activeClientTab.value === 'claude') {
 return generateAnthropicFiles(baseUrl, apiKey)
 }
 if (activeClientTab.value === 'codex-ws') {
 return generateOpenAIWsFiles(baseUrl, apiKey)
 }
 return generateOpenAIFiles(baseUrl, apiKey)
 case 'gemini':
 if (activeClientTab.value === 'codex') {
 return generateRoutedCodexFiles(apiBase, apiKey, 'gemini')
 }
 return [generateGeminiCliContent(baseUrl, apiKey)]
 case 'antigravity':
 if (activeClientTab.value === 'codex') {
 return generateRoutedCodexFiles(apiBase, apiKey, 'antigravity')
 }
 if (activeClientTab.value === 'gemini') {
 return [generateGeminiCliContent(`${baseUrl}/antigravity`, apiKey)]
 }
 return generateAnthropicFiles(`${baseUrl}/antigravity`, apiKey)
 case 'grok':
 if (activeClientTab.value === 'claude') {
 return generateGrokClaudeFiles(baseRoot, apiKey)
 }
 if (activeClientTab.value === 'codex') {
 return generateGrokCodexFiles(apiBase, apiKey)
 }
 return generateGrokFiles(apiBase, apiKey)
 case 'deepseek':
 if (activeClientTab.value === 'codex') {
 return generateRoutedCodexFiles(apiBase, apiKey, 'deepseek')
 }
 return generateAnthropicFiles(baseRoot, apiKey)
 case 'composite':
 if (activeClientTab.value === 'codex') {
 return generateRoutedCodexFiles(apiBase, apiKey, 'composite')
 }
 return generateAnthropicFiles(baseRoot, apiKey)
 default:
 if (activeClientTab.value === 'codex' && props.platform) {
 return generateRoutedCodexFiles(apiBase, apiKey, props.platform)
 }
 return generateAnthropicFiles(baseUrl, apiKey)
 }
})
function generateAnthropicFiles(baseUrl: string, apiKey: string): FileConfig[] {
 let path: string
 let content: string

 switch (activeTab.value) {
 case 'unix':
 path = 'Terminal'
 content = `export ANTHROPIC_BASE_URL="${baseUrl}"
export ANTHROPIC_AUTH_TOKEN="${apiKey}"
export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`
 break
 case 'cmd':
 path = 'Command Prompt'
 content = `set ANTHROPIC_BASE_URL=${baseUrl}
set ANTHROPIC_AUTH_TOKEN=${apiKey}
set CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`
 break
 case 'powershell':
 path = 'PowerShell'
 content = `$env:ANTHROPIC_BASE_URL="${baseUrl}"
$env:ANTHROPIC_AUTH_TOKEN="${apiKey}"
$env:CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`
 break
 default:
 path = 'Terminal'
 content = ''
 }

 const vscodeSettingsPath = activeTab.value === 'unix'
 ? '~/.claude/settings.json'
 : '%USERPROFILE%\\.claude\\settings.json'

 const vscodeContent = `{
 "$schema": "https://json.schemastore.org/claude-code-settings.json",
 "env": {
 "ANTHROPIC_BASE_URL": "${baseUrl}",
 "ANTHROPIC_AUTH_TOKEN": "${apiKey}",
 "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1"
 }
}`

 return [
 { path, content },
 {
 path: vscodeSettingsPath,
 content: vscodeContent,
 hint: t('keys.useKeyModal.claudeSettingsHint')
 }
 ]
}

function generateGrokClaudeFiles(baseUrl: string, apiKey: string): FileConfig[] {
 const environment = {
 ANTHROPIC_BASE_URL: baseUrl,
 ANTHROPIC_AUTH_TOKEN: apiKey,
 ANTHROPIC_MODEL: 'grok-4.5',
 ANTHROPIC_DEFAULT_OPUS_MODEL: 'grok-4.5',
 ANTHROPIC_DEFAULT_SONNET_MODEL: 'grok-4.5',
 ANTHROPIC_DEFAULT_HAIKU_MODEL: 'grok-4.5',
 ANTHROPIC_DEFAULT_FABLE_MODEL: 'grok-4.5',
 CLAUDE_CODE_SUBAGENT_MODEL: 'grok-4.5',
 CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC: '1'
 }
 let path: string
 let content: string

 switch (activeTab.value) {
 case 'unix':
 path = 'Terminal'
 content = Object.entries(environment)
 .map(([name, value]) => `export ${name}="${value}"`)
 .join('\n')
 break
 case 'cmd':
 path = 'Command Prompt'
 content = Object.entries(environment)
 .map(([name, value]) => `set ${name}=${value}`)
 .join('\n')
 break
 case 'powershell':
 path = 'PowerShell'
 content = Object.entries(environment)
 .map(([name, value]) => `$env:${name}="${value}"`)
 .join('\n')
 break
 default:
 path = 'Terminal'
 content = ''
 }

 const settingsPath = activeTab.value === 'unix'
 ? '~/.claude/settings.json'
 : '%USERPROFILE%\\.claude\\settings.json'

 return [
 { path, content },
 {
 path: settingsPath,
 content: JSON.stringify({
 $schema: 'https://json.schemastore.org/claude-code-settings.json',
 env: environment
 }, null, 2),
 hint: t('keys.useKeyModal.claudeSettingsHint')
 }
 ]
}

function generateGeminiCliContent(baseUrl: string, apiKey: string): FileConfig {
 const model = 'gemini-2.0-flash'
 const modelComment = t('keys.useKeyModal.gemini.modelComment')
 let path: string
 let content: string
 let highlighted: string

 switch (activeTab.value) {
 case 'unix':
 path = 'Terminal'
 content = `export GOOGLE_GEMINI_BASE_URL="${baseUrl}"
export GEMINI_API_KEY="${apiKey}"
export GEMINI_MODEL="${model}" # ${modelComment}`
 highlighted = `${keyword('export')} ${variable('GOOGLE_GEMINI_BASE_URL')}${operator('=')}${string(`"${baseUrl}"`)}
${keyword('export')} ${variable('GEMINI_API_KEY')}${operator('=')}${string(`"${apiKey}"`)}
${keyword('export')} ${variable('GEMINI_MODEL')}${operator('=')}${string(`"${model}"`)} ${comment(`# ${modelComment}`)}`
 break
 case 'cmd':
 path = 'Command Prompt'
 content = `set GOOGLE_GEMINI_BASE_URL=${baseUrl}
set GEMINI_API_KEY=${apiKey}
set GEMINI_MODEL=${model}`
 highlighted = `${keyword('set')} ${variable('GOOGLE_GEMINI_BASE_URL')}${operator('=')}${string(baseUrl)}
${keyword('set')} ${variable('GEMINI_API_KEY')}${operator('=')}${string(apiKey)}
${keyword('set')} ${variable('GEMINI_MODEL')}${operator('=')}${string(model)}
${comment(`REM ${modelComment}`)}`
 break
 case 'powershell':
 path = 'PowerShell'
 content = `$env:GOOGLE_GEMINI_BASE_URL="${baseUrl}"
$env:GEMINI_API_KEY="${apiKey}"
$env:GEMINI_MODEL="${model}" # ${modelComment}`
 highlighted = `${keyword('$env:')}${variable('GOOGLE_GEMINI_BASE_URL')}${operator('=')}${string(`"${baseUrl}"`)}
${keyword('$env:')}${variable('GEMINI_API_KEY')}${operator('=')}${string(`"${apiKey}"`)}
${keyword('$env:')}${variable('GEMINI_MODEL')}${operator('=')}${string(`"${model}"`)} ${comment(`# ${modelComment}`)}`
 break
 default:
 path = 'Terminal'
 content = ''
 highlighted = ''
 }

 return { path, content, highlighted }
}

function generateOpenAIFiles(baseUrl: string, apiKey: string): FileConfig[] {
 const isWindows = activeTab.value === 'windows'
 const configDir = isWindows ? '%userprofile%\\.codex' : '~/.codex'

 const model = selectCodexCatalogModel('gpt-5.5')
 const reasoningEffortLine = codexReasoningEffortTomlLine(model)

 // config.toml content
 const configContent = `model_provider = "OpenAI"
model = "${model}"
review_model = "${model}"
${reasoningEffortLine}disable_response_storage = true
model_catalog_json = "${escapeTomlBasicString(codexModelCatalogPath.value)}"
network_access = "enabled"
windows_wsl_setup_acknowledged = true

[model_providers.OpenAI]
name = "OpenAI"
base_url = "${baseUrl}"
wire_api = "responses"
${generateCodexProviderAuthConfig(apiKey)}

[features]
goals = true`

 return buildOpenAICodexFileConfigs(configDir, configContent, apiKey)
}

function generateCodexProviderAuthConfig(apiKey: string): string {
 if (codexAuthMode.value === 'api-key') {
 return `requires_openai_auth = false
experimental_bearer_token = "${escapeTomlBasicString(apiKey)}"
http_headers = { "x-openai-actor-authorization" = "local-image-extension" }`
 }

 return 'requires_openai_auth = true'
}

function buildOpenAICodexFileConfigs(
 configDir: string,
 configContent: string,
 apiKey: string
): FileConfig[] {
 const files: FileConfig[] = [
 {
 path: `${configDir}/config.toml`,
 content: configContent,
 hint: t('keys.useKeyModal.openai.configTomlHint')
 }
 ]

 if (codexAuthMode.value === 'legacy') {
 files.push({
 path: `${configDir}/auth.json`,
 content: JSON.stringify({ OPENAI_API_KEY: apiKey }, null, 2)
 })
 }

 return files
}

function joinConfigPath(dir: string, file: string, windows: boolean): string {
 if (!windows) return `${dir}/${file}`
 return `${dir}\\${file}`
}

function escapeTomlBasicString(value: string): string {
 return value.replace(/\\/g, '\\\\').replace(/"/g, '\\"')
}

function generateGrokFiles(baseUrl: string, apiKey: string): FileConfig[] {
 // Prefer unix/cmd/powershell when shell tabs are shown; fall back to windows tab.
 const shell = activeTab.value
 const isWindowsPath = shell === 'windows' || shell === 'cmd' || shell === 'powershell'
 const configDir = isWindowsPath ? '%userprofile%\\.grok' : '~/.grok'

 let envPath: string
 let envContent: string
 switch (shell) {
 case 'cmd':
 envPath = 'Command Prompt'
 envContent = `set GROK_MODELS_BASE_URL=${baseUrl}
set XAI_API_KEY=${apiKey}`
 break
 case 'powershell':
 case 'windows':
 envPath = 'PowerShell'
 envContent = `$env:GROK_MODELS_BASE_URL="${baseUrl}"
$env:XAI_API_KEY="${apiKey}"`
 break
 default:
 envPath = 'Terminal'
 envContent = `export GROK_MODELS_BASE_URL="${baseUrl}"
export XAI_API_KEY="${apiKey}"`
 }

 // Shape follows Grok Build user guide (~/.grok/docs + custom-models) and production-ready Sub2API setups.
 // Text models only (Responses). Image/video: Imagine model IDs on media endpoints / feature overrides.
 // Credential order: api_key field → env_key → signed-in session → XAI_API_KEY global fallback.
 const modelsListUrl = `${baseUrl.replace(/\/+$/, '')}/models`
 const configContent = `# Grok Build CLI → Sub2API Grok group (API key auth).
# Docs: ~/.grok/docs/user-guide/05-configuration.md + 11-custom-models.md
# Verify after save: grok inspect
#
# IMPORTANT: api_backend must be "responses" for Sub2API Grok (POST /v1/responses).
# If omitted, Grok Build defaults to chat_completions (/v1/chat/completions).
# Keep api_backend = "responses" on every model entry.
#
# Prefer env_key over hardcoding api_key (never commit secrets).
# Also export GROK_MODELS_BASE_URL + XAI_API_KEY in the shell block above.

# Global inference / catalog endpoints (same role as env GROK_MODELS_BASE_URL).
# When models_base_url is set, Grok uses API-key Bearer auth (no grok login required).
[endpoints]
models_base_url = "${baseUrl}" # inference base; model list defaults to {base}/models
models_list_url = "${modelsListUrl}" # optional override (env: GROK_MODELS_LIST_URL)
xai_api_base_url = "${baseUrl}" # public xAI API base override for gateway routing
cli_chat_proxy_base_url = "${baseUrl}" # CLI chat-proxy base (env: GROK_CLI_CHAT_PROXY_BASE_URL)

# Prefer API key when using a custom gateway (matches Sub2API).
# Requires XAI_API_KEY env or per-model env_key / api_key.
[auth]
preferred_method = "api_key"

[model."grok-4.5"]
model = "grok-4.5" # id sent to the API
name = "Grok 4.5" # shown in /model picker
description = "Grok 4.5 via Sub2API (Responses)"
# base_url inherits from [endpoints].models_base_url; override only if needed:
# base_url = "${baseUrl}"
env_key = "XAI_API_KEY" # or: api_key = "${apiKey}" (not recommended)
api_backend = "responses" # chat_completions | responses | messages
context_window = 500000 # drives auto-compaction timing
# Optional sampling (global defaults can live under [models] instead):
# temperature = 0.7
# top_p = 0.95
# max_completion_tokens = 8192
# Server-side (backend) web_search tools — only if your gateway exposes them:
supports_backend_search = true

[model."grok-build-0.1"]
model = "grok-build-0.1"
name = "Grok Build"
description = "Coding / agent sessions (xAI recommends grok-build* for coding)"
env_key = "XAI_API_KEY"
api_backend = "responses"
context_window = 256000
supports_backend_search = true

# Text multi-agent / client web_search sub-agent (NOT Imagine image/video).
[model."grok-4.20-multi-agent-0309"]
model = "grok-4.20-multi-agent-0309"
name = "Grok 4.20 Multi Agent (text / web_search)"
description = "Text multi-agent; use for web_search sub-agent, not image/video"
env_key = "XAI_API_KEY"
api_backend = "responses"
context_window = 1000000
supports_backend_search = true

[model."grok-4.3"]
model = "grok-4.3"
name = "Grok 4.3"
env_key = "XAI_API_KEY"
api_backend = "responses"
context_window = 1000000
supports_backend_search = true

# Optional short alias for /model grok:
# [model."grok"]
# model = "grok-4.5"
# name = "Grok"
# env_key = "XAI_API_KEY"
# api_backend = "responses"
# context_window = 1000000
# supports_backend_search = true

[models]
# xAI recommends grok-build* for coding/agent sessions; use grok-4.5 for general chat.
default = "grok-4.5"
web_search = "grok-4.5" # client-side web_search tool model (must exist as [model.*])
image_description = "grok-4.5" # vision/describe-image helper model
# Optional environment-wide sampling defaults (per-model values win):
# temperature = 0.7
# top_p = 0.95
# max_completion_tokens = 8192
# max_retries = 8

[session]
auto_compact_threshold_percent = 80 # auto-compact at this % of context_window (default 85)

# Imagine tools: model IDs go to Sub2API media endpoints (not the text [model.*] catalog).
# Enable only if the Grok group allows image/video generation.
[features]
image_gen = true
video_gen = true
image_gen_model_override = "grok-imagine-image-quality" # or grok-imagine-image
image_edit_model_override = "grok-imagine-edit"
# Optional feature flags (defaults shown in docs):
# telemetry = false
# remote_fetch = true # set false for air-gapped / pure-gateway catalogs
# lsp_tools = false`

 return [
 { path: envPath, content: envContent },
 {
 path: joinConfigPath(configDir, 'config.toml', isWindowsPath),
 content: configContent,
 hint: t('keys.useKeyModal.grok.configTomlHint')
 }
 ]
}

function generateGrokCodexFiles(baseUrl: string, apiKey: string): FileConfig[] {
 // Codex config reference: wire_api = "responses" only; prefer env_key over experimental_bearer_token.
 // Non-OpenAI gateways should set supports_websockets = false (HTTP/SSE).
 const shell = activeTab.value
 const isWindowsPath = shell === 'windows' || shell === 'cmd' || shell === 'powershell'
 const configDir = isWindowsPath ? '%userprofile%\\.codex' : '~/.codex'
 const model = selectCodexCatalogModel('grok-4.5')

 let envPath: string
 let envContent: string
 switch (shell) {
 case 'cmd':
 envPath = 'Command Prompt'
 envContent = `set SUB2API_API_KEY=${apiKey}`
 break
 case 'powershell':
 case 'windows':
 envPath = 'PowerShell'
 envContent = `$env:SUB2API_API_KEY="${apiKey}"`
 break
 default:
 envPath = 'Terminal'
 envContent = `export SUB2API_API_KEY="${apiKey}"`
 }

 const configContent = `# Codex CLI → Sub2API Grok group
# Docs: Codex config reference (model_providers.*, wire_api = "responses")
#
# Text models only. Image/video: grok-imagine-image / grok-imagine-video on media endpoints.
# Switch model: grok-4.5 | grok-4.3 | grok-build-0.1 | grok-4.20-multi-agent-0309 (text / web_search)

model_provider = "sub2api"
model = "${model}"
model_catalog_json = "${escapeTomlBasicString(codexModelCatalogPath.value)}"
# Optional:
# review_model = "${model}"
# model_reasoning_effort = "medium"
# model_context_window = 500000
# disable_response_storage = true
# network_access = "enabled"
# windows_wsl_setup_acknowledged = true

[model_providers.sub2api]
name = "Sub2API Grok"
base_url = "${baseUrl}"
# Prefer env_key (variable NAME). Do not combine with experimental_bearer_token.
env_key = "SUB2API_API_KEY"
# Fallback only if you cannot set env (discouraged — keeps secret on disk):
# experimental_bearer_token = "${apiKey}"
wire_api = "responses"
# API-key providers: do not require ChatGPT OAuth login
requires_openai_auth = false
# Grok/Sub2API path is HTTP/SSE; disable WS (Codex may otherwise try WebSocket first)
supports_websockets = false

# Optional:
# [features]
# goals = true`

 return [
 { path: envPath, content: envContent },
 {
 path: joinConfigPath(configDir, 'config.toml', isWindowsPath),
 content: configContent,
 hint: t('keys.useKeyModal.grok.codexConfigTomlHint')
 }
 ]
}

function generateRoutedCodexFiles(
 baseUrl: string,
 apiKey: string,
 platform: GroupPlatform
): FileConfig[] {
 const isWindows = activeTab.value === 'windows'
 const configDir = isWindows ? '%userprofile%\\.codex' : '~/.codex'
 const preferredModels: Partial<Record<GroupPlatform, string>> = {
 openai: 'gpt-5.5',
 anthropic: 'claude-sonnet-4-6',
 gemini: 'gemini-2.5-pro',
 antigravity: 'claude-sonnet-4-6',
 grok: 'grok-4.5',
 kiro: 'claude-sonnet-4-6',
 kimi: 'kimi-k2.5',
 zhipu: 'glm-4.7',
 deepseek: 'deepseek-v4-pro',
 composite: 'gpt-5.5'
 }
 const preferredModel = preferredModels[platform] || ''
 const model = selectCodexCatalogModel(preferredModel)
 const labels: Record<GroupPlatform, string> = {
 anthropic: 'Anthropic',
 openai: 'OpenAI',
 gemini: 'Gemini',
 antigravity: 'Antigravity',
 grok: 'Grok',
 kiro: 'Kiro',
 kimi: 'Kimi',
 zhipu: 'Zhipu',
 deepseek: 'DeepSeek',
 composite: 'Composite'
 }
 const label = labels[platform]
 const envContent = isWindows
 ? `$env:SUB2API_API_KEY="${apiKey}"`
 : `export SUB2API_API_KEY="${apiKey}"`

 const configContent = `# Codex CLI -> Sub2API ${label} group
model_provider = "sub2api"
model = "${model}"
review_model = "${model}"
disable_response_storage = true
model_catalog_json = "${escapeTomlBasicString(codexModelCatalogPath.value)}"

[model_providers.sub2api]
name = "Sub2API ${label}"
base_url = "${baseUrl}"
env_key = "SUB2API_API_KEY"
wire_api = "responses"
requires_openai_auth = false
supports_websockets = false`

 return [
 { path: isWindows ? 'PowerShell' : 'Terminal', content: envContent },
 {
 path: joinConfigPath(configDir, 'config.toml', isWindows),
 content: configContent,
 hint: t(
 platform === 'deepseek' || platform === 'composite'
 ? `keys.useKeyModal.${platform}.codexConfigTomlHint`
 : 'keys.useKeyModal.routedCodex.configTomlHint'
 )
 }
 ]
}

function generateOpenAIWsFiles(baseUrl: string, apiKey: string): FileConfig[] {
 const isWindows = activeTab.value === 'windows'
 const configDir = isWindows ? '%userprofile%\\.codex' : '~/.codex'
 const model = selectCodexCatalogModel('gpt-5.5')
 const reasoningEffortLine = codexReasoningEffortTomlLine(model)

 // config.toml content with WebSocket v2
 const configContent = `model_provider = "OpenAI"
model = "${model}"
review_model = "${model}"
${reasoningEffortLine}disable_response_storage = true
model_catalog_json = "${escapeTomlBasicString(codexModelCatalogPath.value)}"
network_access = "enabled"
windows_wsl_setup_acknowledged = true

[model_providers.OpenAI]
name = "OpenAI"
base_url = "${baseUrl}"
wire_api = "responses"
supports_websockets = true
${generateCodexProviderAuthConfig(apiKey)}

[features]
responses_websockets_v2 = true
goals = true`

 return buildOpenAICodexFileConfigs(configDir, configContent, apiKey)
}
function generateOpenCodeConfig(platform: string, baseUrl: string, apiKey: string, pathLabel?: string): FileConfig {
 const provider: Record<string, any> = {
 [platform]: {
 options: {
 baseURL: baseUrl,
 apiKey
 }
 }
 }

 if (platform === 'gemini') {
 provider[platform].npm = '@ai-sdk/google'
 provider[platform].models = geminiModels
 } else if (platform === 'anthropic') {
 provider[platform].npm = '@ai-sdk/anthropic'
 } else if (platform === 'antigravity-claude') {
 provider[platform].npm = '@ai-sdk/anthropic'
 provider[platform].name = 'Antigravity (Claude)'
 provider[platform].models = claudeModels
 } else if (platform === 'antigravity-gemini') {
 provider[platform].npm = '@ai-sdk/google'
 provider[platform].name = 'Antigravity (Gemini)'
 provider[platform].models = antigravityGeminiModels
 } else if (platform === 'openai') {
 provider[platform].models = openaiModels
 } else if (platform === 'grok') {
 // Custom provider pointing at Sub2API OpenAI-compatible Responses/Chat endpoints.
 provider[platform].npm = '@ai-sdk/openai-compatible'
 provider[platform].name = 'Grok via Sub2API'
 provider[platform].models = grokModels
 }

 const agent =
 platform === 'openai'
 ? {
 build: {
 options: {
 store: false
 }
 },
 plan: {
 options: {
 store: false
 }
 }
 }
 : undefined

 const content = JSON.stringify(
 {
 provider,
 ...(agent ? { agent } : {}),
 $schema: 'https://opencode.ai/config.json'
 },
 null,
 2
 )

 return {
 path: pathLabel ?? 'opencode.json',
 content,
 hint: t('keys.useKeyModal.opencode.hint')
 }
}

  return { currentFiles, codexModelCatalogPath }
}
