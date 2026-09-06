<template>
 <BaseDialog
 :show="show"
 :title="t('keys.useKeyModal.title')"
 width="wide"
 @close="emit('close')"
 >
 <div class="space-y-4">
 <!-- No Group Assigned Warning -->
 <div v-if="!platform" class="flex items-start gap-3 p-4 rounded-lg bg-warning-50 border border-warning-200">
 <svg class="w-5 h-5 text-warning-500 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
 <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
 </svg>
 <div>
 <p class="text-sm font-medium text-warning-800">
 {{ t('keys.useKeyModal.noGroupTitle') }}
 </p>
 <p class="text-sm text-warning-text mt-1">
 {{ t('keys.useKeyModal.noGroupDescription') }}
 </p>
 </div>
 </div>

 <!-- Platform-specific content -->
 <template v-else>
 <!-- Description -->
 <p class="text-sm text-muted">
 {{ platformDescription }}
 </p>

 <!-- Client Tabs -->
 <ClientTabsNav
 v-if="clientTabs.length"
 v-model="activeClientTab"
 :tabs="clientTabs"
 />

 <!-- Codex Authentication Mode -->
 <CodexAuthModeSection
 v-if="showCodexAuthMode"
 v-model="codexAuthMode"
 />

 <!-- OS/Shell Tabs -->
 <ShellTabsNav
 v-if="showShellTabs"
 v-model="activeTab"
 :tabs="currentTabs"
 />

 <!-- Code Blocks (Stacked for multi-file platforms) -->
 <CodeFileBlocks
 :files="currentFiles"
 :copied-index="copiedIndex"
 @copy="copyContent"
 />

 <CodexModelCatalogSection
 v-if="showCodexModelCatalog"
 :api-key="apiKey"
 :path="codexModelCatalogPath"
 :state="codexModelManifestState"
 :model-count="codexModelManifestModelCount"
 @fetch="loadCodexModelManifest"
 @download="downloadCodexModelManifest"
 />

 <!-- Usage Note -->
 <div v-if="showPlatformNote" class="flex items-start gap-3 p-3 rounded-lg bg-accent-50 border border-accent-100">
 <Icon name="infoCircle" size="md" class="text-accent-500 flex-shrink-0 mt-0.5" />
 <p class="text-sm text-accent-700">
 {{ platformNote }}
 </p>
 </div>
 </template>
 </div>

 <template #footer>
 <div class="flex justify-end">
 <button
 @click="emit('close')"
 class="btn btn-secondary"
 >
 {{ t('common.close') }}
 </button>
 </div>
 </template>
 </BaseDialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import type { GroupPlatform } from '@/types'
import ClientTabsNav from './use-key/ClientTabsNav.vue'
import ShellTabsNav from './use-key/ShellTabsNav.vue'
import CodexAuthModeSection from './use-key/CodexAuthModeSection.vue'
import CodeFileBlocks from './use-key/CodeFileBlocks.vue'
import CodexModelCatalogSection from './use-key/CodexModelCatalogSection.vue'
import { AppleIcon, WindowsIcon, TerminalIcon, SparkleIcon } from './use-key/tabIcons'
import { useCodexModelManifest } from './use-key/useCodexModelManifest'
import { useUseKeyDescriptions } from './use-key/useUseKeyDescriptions'
import { useUseKeySnippets } from './use-key/useUseKeySnippets'
import type { CodexAuthMode, TabConfig } from './use-key/types'

interface Props {
 show: boolean
 apiKey: string
 baseUrl: string
 platform: GroupPlatform | null
 allowMessagesDispatch?: boolean
}

interface Emits {
 (e: 'close'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t } = useI18n()
const { copyToClipboard: clipboardCopy } = useClipboard()

const copiedIndex = ref<number | null>(null)
const activeTab = ref<string>('unix')
const activeClientTab = ref<string>('claude')
const codexAuthMode = ref<CodexAuthMode>('legacy')

const showCodexModelCatalog = computed(() =>
 props.show &&
 (activeClientTab.value === 'codex' ||
 (props.platform === 'openai' && activeClientTab.value === 'codex-ws'))
)

const {
 codexModelManifestState,
 codexModelManifestContent,
 codexModelManifestModelCount,
 resetCodexModelManifest,
 loadCodexModelManifest,
 downloadCodexModelManifest
} = useCodexModelManifest({
 baseUrl: () => props.baseUrl,
 apiKey: () => props.apiKey,
 canLoad: () => showCodexModelCatalog.value
})

const codexManifestContext = computed(() => {
 if (!showCodexModelCatalog.value) return ''
 return `${props.platform}|${props.baseUrl}|${props.apiKey}`
})

// Reset tabs when platform changes
const defaultClientTab = computed(() => {
 switch (props.platform) {
 case 'openai':
 return 'codex'
 case 'grok':
 return 'grok'
 case 'gemini':
 return 'gemini'
 case 'antigravity':
 return 'claude'
 default:
 return 'claude'
 }
})

watch(() => props.platform, () => {
 activeTab.value = 'unix'
 activeClientTab.value = defaultClientTab.value
 codexAuthMode.value = 'legacy'
}, { immediate: true })

watch(() => props.show, (show) => {
 if (show) {
 codexAuthMode.value = 'legacy'
 } else {
 resetCodexModelManifest()
 }
})

watch(codexManifestContext, (context, previousContext) => {
 if (context !== previousContext) {
 resetCodexModelManifest()
 }
})

// Reset shell tab when client changes
watch(activeClientTab, () => {
 activeTab.value = 'unix'
})

const clientTabs = computed((): TabConfig[] => {
 if (!props.platform) return []
 switch (props.platform) {
 case 'openai': {
 const tabs: TabConfig[] = [
 { id: 'codex', label: t('keys.useKeyModal.cliTabs.codexCli'), icon: TerminalIcon },
 { id: 'codex-ws', label: t('keys.useKeyModal.cliTabs.codexCliWs'), icon: TerminalIcon },
 ]
 if (props.allowMessagesDispatch) {
 tabs.push({ id: 'claude', label: t('keys.useKeyModal.cliTabs.claudeCode'), icon: TerminalIcon })
 }
 tabs.push({ id: 'opencode', label: t('keys.useKeyModal.cliTabs.opencode'), icon: TerminalIcon })
 return tabs
 }
 case 'gemini':
 return [
 { id: 'gemini', label: t('keys.useKeyModal.cliTabs.geminiCli'), icon: SparkleIcon },
 { id: 'codex', label: t('keys.useKeyModal.cliTabs.codexCli'), icon: TerminalIcon },
 { id: 'opencode', label: t('keys.useKeyModal.cliTabs.opencode'), icon: TerminalIcon }
 ]
 case 'antigravity':
 return [
 { id: 'claude', label: t('keys.useKeyModal.cliTabs.claudeCode'), icon: TerminalIcon },
 { id: 'gemini', label: t('keys.useKeyModal.cliTabs.geminiCli'), icon: SparkleIcon },
 { id: 'codex', label: t('keys.useKeyModal.cliTabs.codexCli'), icon: TerminalIcon },
 { id: 'opencode', label: t('keys.useKeyModal.cliTabs.opencode'), icon: TerminalIcon }
 ]
 case 'grok':
 return [
 { id: 'grok', label: t('keys.useKeyModal.cliTabs.grokCli'), icon: TerminalIcon },
 { id: 'claude', label: t('keys.useKeyModal.cliTabs.claudeCode'), icon: TerminalIcon },
 { id: 'codex', label: t('keys.useKeyModal.cliTabs.codexCli'), icon: TerminalIcon },
 { id: 'opencode', label: t('keys.useKeyModal.cliTabs.opencode'), icon: TerminalIcon }
 ]
 case 'deepseek':
 case 'composite':
 return [
 { id: 'claude', label: t('keys.useKeyModal.cliTabs.claudeCode'), icon: TerminalIcon },
 { id: 'codex', label: t('keys.useKeyModal.cliTabs.codexCli'), icon: TerminalIcon },
 { id: 'opencode', label: t('keys.useKeyModal.cliTabs.opencode'), icon: TerminalIcon }
 ]
 default:
 return [
 { id: 'claude', label: t('keys.useKeyModal.cliTabs.claudeCode'), icon: TerminalIcon },
 { id: 'codex', label: t('keys.useKeyModal.cliTabs.codexCli'), icon: TerminalIcon },
 { id: 'opencode', label: t('keys.useKeyModal.cliTabs.opencode'), icon: TerminalIcon }
 ]
 }
})

// Shell tabs (3 types for environment variable based configs)
const shellTabs: TabConfig[] = [
 { id: 'unix', label: 'macOS / Linux', icon: AppleIcon },
 { id: 'cmd', label: 'Windows CMD', icon: WindowsIcon },
 { id: 'powershell', label: 'PowerShell', icon: WindowsIcon }
]

// OpenAI tabs (2 OS types)
const openaiTabs: TabConfig[] = [
 { id: 'unix', label: 'macOS / Linux', icon: AppleIcon },
 { id: 'windows', label: 'Windows', icon: WindowsIcon }
]

const showShellTabs = computed(() => activeClientTab.value !== 'opencode')

const showCodexAuthMode = computed(() =>
 props.platform === 'openai' &&
 (activeClientTab.value === 'codex' || activeClientTab.value === 'codex-ws')
)

const currentTabs = computed(() => {
 if (!showShellTabs.value) return []
 if (activeClientTab.value === 'codex' || activeClientTab.value === 'codex-ws' || activeClientTab.value === 'grok') {
 return openaiTabs
 }
 return shellTabs
})

const { platformDescription, platformNote, showPlatformNote } = useUseKeyDescriptions({
 platform: () => props.platform,
 activeClientTab,
 activeTab,
 t
})

const { currentFiles, codexModelCatalogPath } = useUseKeySnippets({
 props,
 activeTab,
 activeClientTab,
 codexAuthMode,
 codexModelManifestContent,
 t
})

const copyContent = async (content: string, index: number) => {
 const success = await clipboardCopy(content, t('keys.copied'))
 if (success) {
 copiedIndex.value = index
 setTimeout(() => {
 copiedIndex.value = null
 }, 2000)
 }
}
</script>
