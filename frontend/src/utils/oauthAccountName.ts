const cleanNamePart = (value?: string | null): string => {
  const trimmed = value?.trim() || ''
  if (!trimmed || trimmed.startsWith('arn:')) return ''
  return trimmed
}

const firstClean = (values: Array<string | null | undefined>): string => {
  for (const value of values) {
    const cleaned = cleanNamePart(value)
    if (cleaned) return cleaned
  }
  return ''
}

export interface OAuthAccountNameOptions {
  manualName?: string | null
  primary?: string | null
  details?: Array<string | null | undefined>
  platformLabel: string
  fallbackDetail?: string | null
  defaultName: string
}

export const formatOAuthAccountName = ({
  manualName,
  primary,
  details = [],
  platformLabel,
  fallbackDetail,
  defaultName
}: OAuthAccountNameOptions): string => {
  const manual = cleanNamePart(manualName)
  if (manual) return manual

  const main = cleanNamePart(primary)
  const detail = firstClean(details)
  if (main && detail && main !== detail) return `${main} (${detail})`
  if (main) return main

  const fallback = cleanNamePart(fallbackDetail)
  if (fallback) {
    return fallback.toLowerCase().startsWith(platformLabel.toLowerCase())
      ? fallback
      : `${platformLabel} ${fallback}`
  }
  return defaultName
}
