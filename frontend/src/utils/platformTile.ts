/**
 * Brand tile backgrounds for provider logos.
 *
 * The design system renders every provider mark on a 20–24px rounded tile
 * (radius 6–7px) with a 1px inner hairline. The tile colour is the official
 * brand colour and never changes with the theme; the glyph itself is white.
 */
import type { GroupPlatform } from '@/types'

export const PLATFORM_TILE_BG: Record<string, string> = {
  anthropic: '#D97757',
  claude: '#D97757',
  openai: 'linear-gradient(135deg, oklch(40% 0.012 262), oklch(24% 0.01 262))',
  gemini: 'linear-gradient(135deg, #4E8DF5, #9B72CB)',
  antigravity: 'oklch(58% 0.11 190)',
  grok: 'linear-gradient(135deg, oklch(36% 0.01 262), oklch(18% 0.01 262))',
  kiro: 'linear-gradient(135deg, oklch(55% 0.16 300), oklch(42% 0.16 300))',
  kimi: 'oklch(30% 0.03 262)',
  zhipu: 'linear-gradient(135deg, #3B6CF6, #7A4DF5)',
  deepseek: '#4D6BFE',
  composite: 'var(--accent)'
}

export const PLATFORM_LABEL: Record<string, string> = {
  anthropic: 'Claude',
  claude: 'Claude',
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

export function platformTileBackground(platform: string | undefined | null): string {
  if (!platform) return 'var(--surface-tertiary)'
  return PLATFORM_TILE_BG[platform.toLowerCase()] ?? 'var(--surface-tertiary)'
}

export function platformLabel(platform: string | undefined | null): string {
  if (!platform) return ''
  return PLATFORM_LABEL[platform.toLowerCase()] ?? platform
}

/** Map a vendor / model name to the platform key used by PlatformIcon. */
export function platformFromModel(model: string, fallback?: string): GroupPlatform | string {
  const m = model.toLowerCase()
  if (m.startsWith('claude')) return 'anthropic'
  if (m.startsWith('gpt') || m.startsWith('o1') || m.startsWith('o3') || m.startsWith('o4') || m.includes('codex'))
    return 'openai'
  if (m.startsWith('gemini')) return 'gemini'
  if (m.startsWith('grok')) return 'grok'
  if (m.startsWith('kimi') || m.startsWith('moonshot')) return 'kimi'
  if (m.startsWith('glm')) return 'zhipu'
  if (m.startsWith('deepseek')) return 'deepseek'
  return fallback ?? 'openai'
}
