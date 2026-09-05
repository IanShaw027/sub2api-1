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
  anthropic: 'bg-warning-500/10 text-warning-text border-warning-500/30',
  openai: 'bg-success-500/10 text-success-text border-success-500/30',
  antigravity: 'bg-accent-500/10 text-accent-600 border-accent-500/30',
  gemini: 'bg-accent-500/10 text-accent-600 border-accent-500/30',
  grok: 'bg-muted/10 text-muted border-muted/30',
  kimi: 'bg-accent-500/10 text-accent-600 border-accent-500/30',
  zhipu: 'bg-accent-500/10 text-accent-600 border-accent-500/30',
  deepseek: 'bg-success-500/10 text-success-text border-success-500/30',
  composite: 'bg-accent-500/10 text-accent-700 border-accent-500/30',
}
const BADGE_DEFAULT = 'bg-muted/10 text-muted border-muted/30'

// ── Light badge (softer bg, no border) ──────────────────────────────
const BADGE_LIGHT: Record<Platform, string> = {
  anthropic: 'bg-warning-500/10 text-warning-text',
  openai: 'bg-success-500/10 text-success-text',
  antigravity: 'bg-accent-500/10 text-accent-600',
  gemini: 'bg-accent-500/10 text-accent-600',
  grok: 'bg-muted/10 text-muted',
  kimi: 'bg-accent-500/10 text-accent-600',
  zhipu: 'bg-accent-500/10 text-accent-600',
  deepseek: 'bg-success-500/10 text-success-text',
  composite: 'bg-accent-500/10 text-accent-700',
}

// ── Border ──────────────────────────────────────────────────────────
const BORDER: Record<Platform, string> = {
  anthropic: 'border-warning-500/20',
  openai: 'border-success-500/20',
  antigravity: 'border-accent-500/20',
  gemini: 'border-accent-500/20',
  grok: 'border-muted/20',
  kimi: 'border-accent-500/20',
  zhipu: 'border-accent-500/20',
  deepseek: 'border-success-500/20',
  composite: 'border-accent-500/20',
}
const BORDER_DEFAULT = 'border-line'

// ── Border strong (higher-contrast platform tint, e.g. plaza group cards) ──
const BORDER_STRONG: Record<Platform, string> = {
  anthropic: 'border-warning-500/35',
  openai: 'border-success-500/35',
  antigravity: 'border-accent-500/35',
  gemini: 'border-accent-500/35',
  grok: 'border-muted/35',
  kimi: 'border-accent-500/35',
  zhipu: 'border-accent-500/35',
  deepseek: 'border-success-500/35',
  composite: 'border-accent-500/35',
}
const BORDER_STRONG_DEFAULT = 'border-line'

// ── Accent (single raw color per platform; consumers derive washes/tints
//    from it via CSS color-mix, e.g. plaza paid-price zone) ──
const ACCENT: Record<Platform, string> = {
  anthropic: 'var(--warning)', // was orange-500 → warning tone (matches the remapped badge/text classes)
  openai: 'var(--success)', // was green-500
  antigravity: 'var(--accent)', // was purple-500
  gemini: 'var(--accent)', // was blue-500
  grok: 'var(--muted)', // was zinc-500
  kimi: 'var(--accent)', // was pink-500
  zhipu: 'var(--accent)', // was indigo-500
  deepseek: 'var(--accent)', // was teal-500
  composite: 'var(--accent)', // was cyan-500
}
const ACCENT_DEFAULT = 'var(--accent)'

// ── Accent bar (gradient) ───────────────────────────────────────────
const ACCENT_BAR: Record<Platform, string> = {
  anthropic: 'bg-gradient-to-r from-warning-400 to-warning-500',
  openai: 'bg-gradient-to-r from-success-400 to-success-500',
  antigravity: 'bg-gradient-to-r from-accent-400 to-accent-500',
  gemini: 'bg-gradient-to-r from-accent-400 to-accent-500',
  grok: 'bg-gradient-to-r from-[color-mix(in_oklch,var(--muted)_40%,var(--foreground))] to-foreground',
  kimi: 'bg-gradient-to-r from-accent-400 to-accent-500',
  zhipu: 'bg-gradient-to-r from-accent-400 to-accent-500',
  deepseek: 'bg-gradient-to-r from-success-400 to-success-500',
  composite: 'bg-gradient-to-r from-muted to-accent-500',
}
const ACCENT_BAR_DEFAULT = 'bg-gradient-to-r from-[var(--accent)] to-[color-mix(in_oklch,var(--accent)_80%,black)]'

// ── Text (price, icon) ─────────────────────────────────────────────
const TEXT: Record<Platform, string> = {
  anthropic: 'text-warning-text',
  openai: 'text-success-text',
  antigravity: 'text-accent-600',
  gemini: 'text-accent-600',
  grok: 'text-muted',
  kimi: 'text-accent-600',
  zhipu: 'text-accent-600',
  deepseek: 'text-success-text',
  composite: 'text-accent-700',
}
const TEXT_DEFAULT = 'text-accent'

// ── Icon (check mark etc.) ──────────────────────────────────────────
const ICON: Record<Platform, string> = {
  anthropic: 'text-warning-500',
  openai: 'text-success-500',
  antigravity: 'text-accent-500',
  gemini: 'text-accent-500',
  grok: 'text-muted',
  kimi: 'text-accent-500',
  zhipu: 'text-accent-500',
  deepseek: 'text-success-500',
  composite: 'text-accent-600',
}
const ICON_DEFAULT = 'text-accent'

// ── Button (solid bg) ───────────────────────────────────────────────
const BUTTON: Record<Platform, string> = {
  anthropic: 'bg-warning-500 text-white hover:bg-warning-600 active:bg-warning-700',
  openai: 'bg-success-600 text-white hover:bg-success-700 active:bg-success-800',
  antigravity: 'bg-accent-500 text-white hover:bg-accent-600 active:bg-accent-700',
  gemini: 'bg-accent-500 text-white hover:bg-accent-600 active:bg-accent-700',
  grok: 'bg-foreground text-white hover:bg-foreground active:bg-foreground',
  kimi: 'bg-accent-500 text-white hover:bg-accent-600 active:bg-accent-700',
  zhipu: 'bg-accent-500 text-white hover:bg-accent-600 active:bg-accent-700',
  deepseek: 'bg-success-500 text-white hover:bg-success-600 active:bg-success-700',
  composite: 'bg-accent-700 text-white hover:bg-accent-800 active:bg-accent-900',
}
const BUTTON_DEFAULT = 'bg-accent text-white hover:opacity-90 active:opacity-80'

// ── Discount badge ──────────────────────────────────────────────────
const DISCOUNT: Record<Platform, string> = {
  anthropic: 'bg-warning-500/15 text-warning-text',
  openai: 'bg-success-500/15 text-success-text',
  antigravity: 'bg-accent-500/15 text-accent-700',
  gemini: 'bg-accent-500/15 text-accent-700',
  grok: 'bg-muted/15 text-[color-mix(in_oklch,var(--muted)_40%,var(--foreground))]',
  kimi: 'bg-accent-500/15 text-accent-700',
  zhipu: 'bg-accent-500/15 text-accent-700',
  deepseek: 'bg-success-500/15 text-success-text',
  composite: 'bg-accent-500/15 text-accent-800',
}
const DISCOUNT_DEFAULT = 'bg-danger-500/15 text-danger-text'

// ── Header gradient (subscription confirm) ─────────────────────────
const GRADIENT: Record<Platform, string> = {
  anthropic: 'from-warning-500 to-warning-600',
  openai: 'from-success-500 to-success-600',
  antigravity: 'from-accent-500 to-accent-600',
  gemini: 'from-accent-500 to-accent-600',
  grok: 'from-[color-mix(in_oklch,var(--muted)_40%,var(--foreground))] to-foreground',
  kimi: 'from-accent-500 to-accent-600',
  zhipu: 'from-accent-500 to-accent-600',
  deepseek: 'from-success-500 to-success-600',
  composite: 'from-muted to-accent-600',
}
const GRADIENT_DEFAULT = 'from-[var(--accent)] to-[color-mix(in_oklch,var(--accent)_75%,black)]'

// ── Header text (light text on gradient bg) ────────────────────────
const GRADIENT_TEXT: Record<Platform, string> = {
  anthropic: 'text-warning-100',
  openai: 'text-success-100',
  antigravity: 'text-accent-100',
  gemini: 'text-accent-100',
  grok: 'text-surface-2',
  kimi: 'text-accent-100',
  zhipu: 'text-accent-100',
  deepseek: 'text-success-100',
  composite: 'text-accent-100',
}
const GRADIENT_TEXT_DEFAULT = 'text-white/90'

const GRADIENT_SUBTEXT: Record<Platform, string> = {
  anthropic: 'text-warning-200',
  openai: 'text-success-200',
  antigravity: 'text-accent-200',
  gemini: 'text-accent-200',
  grok: 'text-line',
  kimi: 'text-accent-200',
  zhipu: 'text-accent-200',
  deepseek: 'text-success-200',
  composite: 'text-accent-200',
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
