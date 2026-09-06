export type GlassCardVariant = 'glass' | 'solid' | 'transparent' | 'flat'
export type GlassCardPadding = 'sm' | 'md' | 'lg'

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'success' | 'warning' | 'icon'
/** `xs` 26px · `sm` 32px · default (unset) 34px · `md` 42px (hero) */
export type ButtonSize = 'xs' | 'sm' | 'md' | 'lg'

export type StatusBadgeTone = 'success' | 'warning' | 'danger' | 'muted' | 'accent'

export type ToggleSwitchSize = 'compact' | 'form'

export type PageHeaderVariant = 'compact' | 'hero'

export type StatDeltaTone = 'up' | 'down' | 'warn' | 'neutral'

export type DrawerSide = 'right' | 'left'

export type ModalWidth = 'sm' | 'md' | 'lg' | 'xl'

export type SegmentedOption<T extends string = string> = {
  value: T
  label: string
  disabled?: boolean
}

export type ChipOption<T extends string = string> = SegmentedOption<T>

export type MiniStatItem = {
  label: string
  value: string | number
  sub?: string
  tone?: StatusBadgeTone
}
