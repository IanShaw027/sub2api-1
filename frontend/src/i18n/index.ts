import { createI18n } from 'vue-i18n'

type LocaleCode = 'en' | 'zh'

type LocaleMessages = Record<string, any>

const LOCALE_KEY = 'sub2api_locale'
const DEFAULT_LOCALE: LocaleCode = 'en'

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

export function mergeLocaleMessages(
  base: LocaleMessages,
  override: LocaleMessages
): LocaleMessages {
  const merged: LocaleMessages = { ...base }

  for (const [key, value] of Object.entries(override)) {
    const current = merged[key]
    if (isPlainObject(current) && isPlainObject(value)) {
      merged[key] = mergeLocaleMessages(current, value)
      continue
    }
    merged[key] = value
  }

  return merged
}

export function collectLocaleConflicts(
  base: LocaleMessages,
  override: LocaleMessages,
  path: string[] = []
): string[] {
  const conflicts: string[] = []

  for (const [key, value] of Object.entries(override)) {
    const nextPath = [...path, key]
    const current = base[key]
    if (isPlainObject(current) && isPlainObject(value)) {
      conflicts.push(...collectLocaleConflicts(current, value, nextPath))
      continue
    }
    if (key in base && JSON.stringify(current) !== JSON.stringify(value)) {
      conflicts.push(nextPath.join('.'))
    }
  }

  return conflicts
}

const localeLoaders: Record<LocaleCode, () => Promise<LocaleMessages>> = {
  // Runtime messages are currently split across JSON and TS locale sources.
  // Merge them so newer TS-only keys do not disappear from the shipped UI.
  en: async () => {
    const [jsonModule, tsModule] = await Promise.all([
      import('./locales/en.json'),
      import('./locales/en.ts')
    ])
    const conflicts = collectLocaleConflicts(jsonModule.default, tsModule.default)
    if (conflicts.length > 0) {
      console.warn(`[i18n] locale conflicts detected for en: ${conflicts.join(', ')}`)
    }
    return mergeLocaleMessages(jsonModule.default, tsModule.default)
  },
  zh: async () => {
    const [jsonModule, tsModule] = await Promise.all([
      import('./locales/zh.json'),
      import('./locales/zh.ts')
    ])
    const conflicts = collectLocaleConflicts(jsonModule.default, tsModule.default)
    if (conflicts.length > 0) {
      console.warn(`[i18n] locale conflicts detected for zh: ${conflicts.join(', ')}`)
    }
    return mergeLocaleMessages(jsonModule.default, tsModule.default)
  }
}

function isLocaleCode(value: string): value is LocaleCode {
  return value === 'en' || value === 'zh'
}

function getDefaultLocale(): LocaleCode {
  const saved = localStorage.getItem(LOCALE_KEY)
  if (saved && isLocaleCode(saved)) {
    return saved
  }

  const browserLang = navigator.language.toLowerCase()
  if (browserLang.startsWith('zh')) {
    return 'zh'
  }

  return DEFAULT_LOCALE
}

export const i18n = createI18n({
  legacy: false,
  locale: getDefaultLocale(),
  fallbackLocale: DEFAULT_LOCALE,
  messages: {},
  // 禁用 HTML 消息警告 - 引导步骤使用富文本内容（driver.js 支持 HTML）
  // 这些内容是内部定义的，不存在 XSS 风险
  warnHtmlMessage: false
})

const loadedLocales = new Set<LocaleCode>()

export async function loadLocaleMessages(locale: LocaleCode): Promise<void> {
  if (loadedLocales.has(locale)) {
    return
  }

  const loader = localeLoaders[locale]
  const messages = await loader()
  i18n.global.setLocaleMessage(locale, messages)
  loadedLocales.add(locale)
}

export async function initI18n(): Promise<void> {
  const current = getLocale()
  await loadLocaleMessages(DEFAULT_LOCALE)
  if (current !== DEFAULT_LOCALE) {
    await loadLocaleMessages(current)
  }
  document.documentElement.setAttribute('lang', current)
}

export async function setLocale(locale: string): Promise<void> {
  if (!isLocaleCode(locale)) {
    return
  }

  await loadLocaleMessages(locale)
  i18n.global.locale.value = locale
  localStorage.setItem(LOCALE_KEY, locale)
  document.documentElement.setAttribute('lang', locale)

  // 同步更新浏览器页签标题，使其跟随语言切换
  const { resolveRouteDocumentTitle } = await import('@/router/title')
  const { default: router } = await import('@/router')
  const { useAppStore } = await import('@/stores/app')
  const { useAuthStore } = await import('@/stores/auth')
  const { useAdminSettingsStore } = await import('@/stores/adminSettings')
  const route = router.currentRoute.value
  const appStore = useAppStore()
  const authStore = useAuthStore()
  const adminSettingsStore = useAdminSettingsStore()
  const customMenuItems = [
    ...(appStore.cachedPublicSettings?.custom_menu_items ?? []),
    ...(authStore.isAdmin ? adminSettingsStore.customMenuItems : []),
  ]
  document.title = resolveRouteDocumentTitle(route, appStore.siteName, customMenuItems)
}

export function getLocale(): LocaleCode {
  const current = i18n.global.locale.value
  return isLocaleCode(current) ? current : DEFAULT_LOCALE
}

export const availableLocales = [
  { code: 'en', name: 'English', flag: '🇺🇸' },
  { code: 'zh', name: '中文', flag: '🇨🇳' }
] as const

export default i18n
