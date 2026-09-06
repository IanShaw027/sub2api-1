import { reactive } from 'vue'

/**
 * Prompt-cell hover/focus popover for the /batch-image detail table.
 * Extracted from useBatchImageGuide.ts (glass-ui-redesign task 6.4) so the
 * composition root stays under the line budget; behaviour is unchanged.
 */
export interface UseBatchImagePromptPopoverOptions {
  copyToClipboard: (text: string, message?: string) => unknown
  t: (key: string) => string
}

export function useBatchImagePromptPopover(options: UseBatchImagePromptPopoverOptions) {
  const { copyToClipboard, t } = options

  const promptPopover = reactive({
    visible: false,
    text: '',
    style: {} as Record<string, string>,
  })

  let promptPopoverCloseTimer: ReturnType<typeof setTimeout> | null = null
  let promptPopoverOpenTimer: ReturnType<typeof setTimeout> | null = null
  let activePromptPopoverTarget: HTMLElement | null = null

  function cancelPromptPopoverClose() {
    if (!promptPopoverCloseTimer) return
    clearTimeout(promptPopoverCloseTimer)
    promptPopoverCloseTimer = null
  }

  function cancelPromptPopoverOpen() {
    if (!promptPopoverOpenTimer) return
    clearTimeout(promptPopoverOpenTimer)
    promptPopoverOpenTimer = null
  }

  function closePromptPopover() {
    cancelPromptPopoverOpen()
    cancelPromptPopoverClose()
    promptPopover.visible = false
    promptPopover.text = ''
    promptPopover.style = {}
    activePromptPopoverTarget = null
  }

  function schedulePromptPopoverClose() {
    cancelPromptPopoverOpen()
    cancelPromptPopoverClose()
    promptPopoverCloseTimer = setTimeout(() => {
      closePromptPopover()
    }, 180)
  }

  function schedulePromptPopoverOpen(event: MouseEvent | PointerEvent, text: string) {
    const target = event.currentTarget as HTMLElement | null
    if (!target) return
    const value = String(text || '').trim()
    if (!value || value === '-') return
    activePromptPopoverTarget = target
    cancelPromptPopoverOpen()
    cancelPromptPopoverClose()
    promptPopoverOpenTimer = setTimeout(() => {
      if (activePromptPopoverTarget !== target || !document.body.contains(target)) return
      openPromptPopover(target, value)
    }, 520)
  }

  function showPromptPopover(event: MouseEvent | FocusEvent, text: string) {
    const value = String(text || '').trim()
    if (!value || value === '-') return
    const target = event.currentTarget as HTMLElement | null
    cancelPromptPopoverClose()
    cancelPromptPopoverOpen()
    if (!target) return
    activePromptPopoverTarget = target
    openPromptPopover(target, value)
  }

  function openPromptPopover(target: HTMLElement, value: string) {
    const rect = target.getBoundingClientRect()
    if (!rect) return
    const viewportWidth = window.innerWidth || 1280
    const viewportHeight = window.innerHeight || 720
    const width = Math.min(440, Math.max(320, viewportWidth - 32))
    const left = Math.max(16, Math.min(rect.left, viewportWidth - width - 16))
    const estimatedHeight = 178
    const preferredTop = rect.bottom + 8
    const top = preferredTop + estimatedHeight > viewportHeight
      ? Math.max(16, rect.top - estimatedHeight - 8)
      : preferredTop
    promptPopover.text = value
    promptPopover.style = {
      left: `${left}px`,
      top: `${top}px`,
      width: `${width}px`,
    }
    promptPopover.visible = true
  }

  function copyPromptPopover() {
    if (!promptPopover.text) return
    void copyToClipboard(promptPopover.text, t('batchImage.promptPopover.copied'))
  }

  return {
    promptPopover,
    cancelPromptPopoverClose,
    cancelPromptPopoverOpen,
    closePromptPopover,
    schedulePromptPopoverClose,
    schedulePromptPopoverOpen,
    showPromptPopover,
    copyPromptPopover,
  }
}
