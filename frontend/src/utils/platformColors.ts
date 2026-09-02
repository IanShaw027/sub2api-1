/**
 * Centralized platform color definitions.
 *
 * All components that need platform-specific styling should import from here
 * instead of defining their own color mappings.
 */

export type Platform =
  | 'anthropic'
  | 'openai'
  | 'antigravity'
  | 'gemini'
  | 'grok'
  | 'kimi'
  | 'zhipu'
  | 'deepseek'
  | 'composite'

// ── Badge (bg + text + border, for inline badges with border) ───────
const BADGE: Record<Platform, string> = {
  anthropic: 'bg-orange-500/10 text-orange-600 border-orange-500/30',
  openai: 'bg-green-500/10 text-green-600 border-green-500/30',
  antigravity: 'bg-purple-500/10 text-purple-600 border-purple-500/30',
  gemini: 'bg-blue-500/10 text-blue-600 border-blue-500/30',
  grok: 'bg-zinc-500/10 text-zinc-600 border-zinc-500/30',
  kimi: 'bg-pink-500/10 text-pink-600 border-pink-500/30',
  zhipu: 'bg-indigo-500/10 text-indigo-600 border-indigo-500/30',
  deepseek: 'bg-teal-500/10 text-teal-600 border-teal-500/30',
  composite: 'bg-cyan-500/10 text-cyan-700 border-cyan-500/30',
}
const BADGE_DEFAULT = 'bg-slate-500/10 text-slate-600 border-slate-500/30'

// ── Light badge (softer bg, no border) ──────────────────────────────
const BADGE_LIGHT: Record<Platform, string> = {
  anthropic: 'bg-orange-500/10 text-orange-600',
  openai: 'bg-green-500/10 text-green-600',
  antigravity: 'bg-purple-500/10 text-purple-600',
  gemini: 'bg-blue-500/10 text-blue-600',
  grok: 'bg-zinc-500/10 text-zinc-600',
  kimi: 'bg-pink-500/10 text-pink-600',
  zhipu: 'bg-indigo-500/10 text-indigo-600',
  deepseek: 'bg-teal-500/10 text-teal-600',
  composite: 'bg-cyan-500/10 text-cyan-700',
}

// ── Border ──────────────────────────────────────────────────────────
const BORDER: Record<Platform, string> = {
  anthropic: 'border-orange-500/20',
  openai: 'border-green-500/20',
  antigravity: 'border-purple-500/20',
  gemini: 'border-blue-500/20',
  grok: 'border-zinc-500/20',
  kimi: 'border-pink-500/20',
  zhipu: 'border-indigo-500/20',
  deepseek: 'border-teal-500/20',
  composite: 'border-cyan-500/20',
}
const BORDER_DEFAULT = 'border-line'

// ── Border strong (higher-contrast platform tint, e.g. plaza group cards) ──
const BORDER_STRONG: Record<Platform, string> = {
  anthropic: 'border-orange-500/35',
  openai: 'border-green-500/35',
  antigravity: 'border-purple-500/35',
  gemini: 'border-blue-500/35',
  grok: 'border-zinc-500/35',
  kimi: 'border-pink-500/35',
  zhipu: 'border-indigo-500/35',
  deepseek: 'border-teal-500/35',
  composite: 'border-cyan-500/35',
}
const BORDER_STRONG_DEFAULT = 'border-line'

// ── Accent (single raw color per platform; consumers derive washes/tints
//    from it via CSS color-mix, e.g. plaza paid-price zone) ──
const ACCENT: Record<Platform, string> = {
  anthropic: '#f97316', // orange-500
  openai: '#22c55e', // green-500
  antigravity: '#a855f7', // purple-500
  gemini: '#3b82f6', // blue-500
  grok: '#71717a', // zinc-500
  kimi: '#ec4899', // pink-500
  zhipu: '#6366f1', // indigo-500
  deepseek: '#14b8a6', // teal-500
  composite: '#06b6d4', // cyan-500
}
const ACCENT_DEFAULT = 'var(--accent)'

// ── Accent bar (gradient) ───────────────────────────────────────────
const ACCENT_BAR: Record<Platform, string> = {
  anthropic: 'bg-gradient-to-r from-orange-400 to-orange-500',
  openai: 'bg-gradient-to-r from-emerald-400 to-emerald-500',
  antigravity: 'bg-gradient-to-r from-purple-400 to-purple-500',
  gemini: 'bg-gradient-to-r from-blue-400 to-blue-500',
  grok: 'bg-gradient-to-r from-zinc-700 to-zinc-900',
  kimi: 'bg-gradient-to-r from-pink-400 to-pink-500',
  zhipu: 'bg-gradient-to-r from-indigo-400 to-indigo-500',
  deepseek: 'bg-gradient-to-r from-teal-400 to-teal-500',
  composite: 'bg-gradient-to-r from-slate-500 to-cyan-500',
}
const ACCENT_BAR_DEFAULT = 'bg-gradient-to-r from-[var(--accent)] to-[color-mix(in_oklch,var(--accent)_80%,black)]'

// ── Text (price, icon) ─────────────────────────────────────────────
const TEXT: Record<Platform, string> = {
  anthropic: 'text-orange-600',
  openai: 'text-emerald-600',
  antigravity: 'text-purple-600',
  gemini: 'text-blue-600',
  grok: 'text-zinc-600',
  kimi: 'text-pink-600',
  zhipu: 'text-indigo-600',
  deepseek: 'text-teal-600',
  composite: 'text-cyan-700',
}
const TEXT_DEFAULT = 'text-accent'

// ── Icon (check mark etc.) ──────────────────────────────────────────
const ICON: Record<Platform, string> = {
  anthropic: 'text-orange-500',
  openai: 'text-emerald-500',
  antigravity: 'text-purple-500',
  gemini: 'text-blue-500',
  grok: 'text-zinc-600',
  kimi: 'text-pink-500',
  zhipu: 'text-indigo-500',
  deepseek: 'text-teal-500',
  composite: 'text-cyan-600',
}
const ICON_DEFAULT = 'text-accent'

// ── Button (solid bg) ───────────────────────────────────────────────
const BUTTON: Record<Platform, string> = {
  anthropic: 'bg-orange-500 text-white hover:bg-orange-600 active:bg-orange-700',
  openai: 'bg-green-600 text-white hover:bg-green-700 active:bg-green-800',
  antigravity: 'bg-purple-500 text-white hover:bg-purple-600 active:bg-purple-700',
  gemini: 'bg-blue-500 text-white hover:bg-blue-600 active:bg-blue-700',
  grok: 'bg-zinc-800 text-white hover:bg-zinc-900 active:bg-black',
  kimi: 'bg-pink-500 text-white hover:bg-pink-600 active:bg-pink-700',
  zhipu: 'bg-indigo-500 text-white hover:bg-indigo-600 active:bg-indigo-700',
  deepseek: 'bg-teal-500 text-white hover:bg-teal-600 active:bg-teal-700',
  composite: 'bg-cyan-700 text-white hover:bg-cyan-800 active:bg-cyan-900',
}
const BUTTON_DEFAULT = 'bg-accent text-white hover:opacity-90 active:opacity-80'

// ── Discount badge ──────────────────────────────────────────────────
const DISCOUNT: Record<Platform, string> = {
  anthropic: 'bg-orange-500/15 text-orange-700',
  openai: 'bg-emerald-500/15 text-emerald-700',
  antigravity: 'bg-purple-500/15 text-purple-700',
  gemini: 'bg-blue-500/15 text-blue-700',
  grok: 'bg-zinc-500/15 text-zinc-700',
  kimi: 'bg-pink-500/15 text-pink-700',
  zhipu: 'bg-indigo-500/15 text-indigo-700',
  deepseek: 'bg-teal-500/15 text-teal-700',
  composite: 'bg-cyan-500/15 text-cyan-800',
}
const DISCOUNT_DEFAULT = 'bg-red-500/15 text-red-700'

// ── Header gradient (subscription confirm) ─────────────────────────
const GRADIENT: Record<Platform, string> = {
  anthropic: 'from-orange-500 to-orange-600',
  openai: 'from-emerald-500 to-emerald-600',
  antigravity: 'from-purple-500 to-purple-600',
  gemini: 'from-blue-500 to-blue-600',
  grok: 'from-zinc-700 to-zinc-900',
  kimi: 'from-pink-500 to-pink-600',
  zhipu: 'from-indigo-500 to-indigo-600',
  deepseek: 'from-teal-500 to-teal-600',
  composite: 'from-slate-600 to-cyan-600',
}
const GRADIENT_DEFAULT = 'from-[var(--accent)] to-[color-mix(in_oklch,var(--accent)_75%,black)]'

// ── Header text (light text on gradient bg) ────────────────────────
const GRADIENT_TEXT: Record<Platform, string> = {
  anthropic: 'text-orange-100',
  openai: 'text-emerald-100',
  antigravity: 'text-purple-100',
  gemini: 'text-blue-100',
  grok: 'text-zinc-100',
  kimi: 'text-pink-100',
  zhipu: 'text-indigo-100',
  deepseek: 'text-teal-100',
  composite: 'text-cyan-100',
}
const GRADIENT_TEXT_DEFAULT = 'text-white/90'

const GRADIENT_SUBTEXT: Record<Platform, string> = {
  anthropic: 'text-orange-200',
  openai: 'text-emerald-200',
  antigravity: 'text-purple-200',
  gemini: 'text-blue-200',
  grok: 'text-zinc-300',
  kimi: 'text-pink-200',
  zhipu: 'text-indigo-200',
  deepseek: 'text-teal-200',
  composite: 'text-cyan-200',
}
const GRADIENT_SUBTEXT_DEFAULT = 'text-white/70'

// ── Public API ──────────────────────────────────────────────────────

function isPlatform(p: string): p is Platform {
  return (
    p === 'anthropic' ||
    p === 'openai' ||
    p === 'antigravity' ||
    p === 'gemini' ||
    p === 'grok' ||
    p === 'kimi' ||
    p === 'zhipu' ||
    p === 'deepseek' ||
    p === 'composite'
  )
}

export function platformBadgeClass(p: string): string {
  return isPlatform(p) ? BADGE[p] : BADGE_DEFAULT
}

export function platformBadgeLightClass(p: string): string {
  return isPlatform(p) ? BADGE_LIGHT[p] : BADGE_DEFAULT
}

export function platformBorderClass(p: string): string {
  return isPlatform(p) ? BORDER[p] : BORDER_DEFAULT
}

export function platformBorderStrongClass(p: string): string {
  return isPlatform(p) ? BORDER_STRONG[p] : BORDER_STRONG_DEFAULT
}

export function platformAccentColor(p: string): string {
  return isPlatform(p) ? ACCENT[p] : ACCENT_DEFAULT
}

export function platformAccentBarClass(p: string): string {
  return isPlatform(p) ? ACCENT_BAR[p] : ACCENT_BAR_DEFAULT
}

export function platformTextClass(p: string): string {
  return isPlatform(p) ? TEXT[p] : TEXT_DEFAULT
}

export function platformIconClass(p: string): string {
  return isPlatform(p) ? ICON[p] : ICON_DEFAULT
}

export function platformButtonClass(p: string): string {
  return isPlatform(p) ? BUTTON[p] : BUTTON_DEFAULT
}

export function platformDiscountClass(p: string): string {
  return isPlatform(p) ? DISCOUNT[p] : DISCOUNT_DEFAULT
}

export function platformGradientClass(p: string): string {
  return isPlatform(p) ? GRADIENT[p] : GRADIENT_DEFAULT
}

export function platformGradientTextClass(p: string): string {
  return isPlatform(p) ? GRADIENT_TEXT[p] : GRADIENT_TEXT_DEFAULT
}

export function platformGradientSubtextClass(p: string): string {
  return isPlatform(p) ? GRADIENT_SUBTEXT[p] : GRADIENT_SUBTEXT_DEFAULT
}

export function platformLabel(p: string): string {
  switch (p) {
    case 'anthropic': return 'Anthropic'
    case 'openai': return 'OpenAI'
    case 'antigravity': return 'Antigravity'
    case 'gemini': return 'Gemini'
    case 'grok': return 'Grok'
    case 'kimi': return 'Kimi'
    case 'zhipu': return 'Zhipu GLM'
    case 'deepseek': return 'DeepSeek'
    case 'composite': return 'Composite'
    default: return p || 'API'
  }
}
