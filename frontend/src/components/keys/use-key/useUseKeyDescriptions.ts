import { computed, type ComputedRef, type Ref } from 'vue'
import type { GroupPlatform } from '@/types'

export interface UseUseKeyDescriptionsOptions {
  platform: () => GroupPlatform | null
  activeClientTab: Ref<string>
  activeTab: Ref<string>
  t: (key: string) => string
}

export interface UseUseKeyDescriptionsResult {
  platformDescription: ComputedRef<string>
  platformNote: ComputedRef<string>
  showPlatformNote: ComputedRef<boolean>
}

/**
 * Selects the platform + active-tab specific description/note copy shown above
 * and below the instruction code blocks in UseKeyModal.
 */
export function useUseKeyDescriptions(options: UseUseKeyDescriptionsOptions): UseUseKeyDescriptionsResult {
  const { platform, activeClientTab, activeTab, t } = options

  const platformDescription = computed(() => {
    if (activeClientTab.value === 'codex' &&
      platform() !== 'openai' &&
      platform() !== 'grok' &&
      platform() !== 'deepseek' &&
      platform() !== 'composite') {
      return t('keys.useKeyModal.routedCodex.description')
    }
    switch (platform()) {
      case 'openai':
        if (activeClientTab.value === 'claude') {
          return t('keys.useKeyModal.description')
        }
        return t('keys.useKeyModal.openai.description')
      case 'gemini':
        return t('keys.useKeyModal.gemini.description')
      case 'antigravity':
        return t('keys.useKeyModal.antigravity.description')
      case 'grok':
        if (activeClientTab.value === 'claude') {
          return t('keys.useKeyModal.grok.claudeDescription')
        }
        if (activeClientTab.value === 'codex') {
          return t('keys.useKeyModal.grok.codexDescription')
        }
        return t('keys.useKeyModal.grok.description')
      case 'deepseek':
        return activeClientTab.value === 'codex'
          ? t('keys.useKeyModal.deepseek.codexDescription')
          : t('keys.useKeyModal.deepseek.description')
      case 'composite':
        return activeClientTab.value === 'codex'
          ? t('keys.useKeyModal.composite.codexDescription')
          : t('keys.useKeyModal.composite.description')
      default:
        return t('keys.useKeyModal.description')
    }
  })

  const platformNote = computed(() => {
    if (activeClientTab.value === 'codex' &&
      platform() !== 'openai' &&
      platform() !== 'grok' &&
      platform() !== 'deepseek' &&
      platform() !== 'composite') {
      return t('keys.useKeyModal.routedCodex.note')
    }
    switch (platform()) {
      case 'openai':
        if (activeClientTab.value === 'claude') {
          return t('keys.useKeyModal.note')
        }
        return activeTab.value === 'windows'
          ? t('keys.useKeyModal.openai.noteWindows')
          : t('keys.useKeyModal.openai.note')
      case 'gemini':
        return t('keys.useKeyModal.gemini.note')
      case 'antigravity':
        return activeClientTab.value === 'claude'
          ? t('keys.useKeyModal.antigravity.claudeNote')
          : t('keys.useKeyModal.antigravity.geminiNote')
      case 'grok':
        if (activeClientTab.value === 'claude') {
          return t('keys.useKeyModal.grok.claudeNote')
        }
        if (activeClientTab.value === 'codex') {
          return activeTab.value === 'windows'
            ? t('keys.useKeyModal.grok.codexNoteWindows')
            : t('keys.useKeyModal.grok.codexNote')
        }
        // Grok CLI: shell-specific path guidance (env + ~/.grok/config.toml).
        if (activeClientTab.value === 'grok' && (activeTab.value === 'cmd' || activeTab.value === 'powershell')) {
          return t('keys.useKeyModal.grok.noteWindows')
        }
        if (activeClientTab.value === 'grok' && activeTab.value === 'windows') {
          return t('keys.useKeyModal.grok.noteWindows')
        }
        return t('keys.useKeyModal.grok.note')
      case 'deepseek':
        return activeClientTab.value === 'codex'
          ? t('keys.useKeyModal.deepseek.codexNote')
          : t('keys.useKeyModal.note')
      case 'composite':
        return activeClientTab.value === 'codex'
          ? t('keys.useKeyModal.composite.codexNote')
          : t('keys.useKeyModal.note')
      default:
        return t('keys.useKeyModal.note')
    }
  })

  const showPlatformNote = computed(() => activeClientTab.value !== 'opencode')

  return { platformDescription, platformNote, showPlatformNote }
}
