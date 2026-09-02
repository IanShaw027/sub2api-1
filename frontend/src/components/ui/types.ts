export type GlassCardVariant = 'glass' | 'solid' | 'transparent'
export type GlassCardPadding = 'sm' | 'md' | 'lg'

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'icon'
export type ButtonSize = 'sm' | 'md'

export type StatusBadgeTone = 'success' | 'warning' | 'danger' | 'muted' | 'accent'

export type ToggleSwitchSize = 'compact' | 'form'

export type PageHeaderVariant = 'compact' | 'hero'

export type SegmentedOption<T extends string = string> = {
  value: T
  label: string
  disabled?: boolean
}
