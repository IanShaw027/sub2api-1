export interface TLSFingerprintProfileOption {
  id: number
  name: string
  platform?: string | null
}

export interface SelectableTLSFingerprintProfileOption extends TLSFingerprintProfileOption {
  platform: string
  isPlatformMismatch: boolean
}

export function normalizeTLSFingerprintProfilePlatform(platform: unknown): string {
  return typeof platform === 'string' ? platform.trim().toLowerCase() : ''
}

export function getSelectableTLSFingerprintProfiles(
  profiles: TLSFingerprintProfileOption[],
  accountPlatform: string | null | undefined,
  selectedProfileID: number | null | undefined
): SelectableTLSFingerprintProfileOption[] {
  const normalizedAccountPlatform = normalizeTLSFingerprintProfilePlatform(accountPlatform)
  const normalizedSelectedID = typeof selectedProfileID === 'number' && Number.isFinite(selectedProfileID)
    ? selectedProfileID
    : null

  return profiles
    .map((profile) => {
      const platform = normalizeTLSFingerprintProfilePlatform(profile.platform)
      return {
        ...profile,
        platform,
        isPlatformMismatch: Boolean(platform && platform !== normalizedAccountPlatform)
      }
    })
    .filter((profile) => {
      if (!profile.platform || profile.platform === normalizedAccountPlatform) {
        return true
      }
      return normalizedSelectedID != null && profile.id === normalizedSelectedID
    })
}

export function formatTLSFingerprintProfileOptionLabel(
  profile: SelectableTLSFingerprintProfileOption,
  labels: { shared: string; mismatch: string }
): string {
  const scope = profile.platform || labels.shared
  const mismatch = profile.isPlatformMismatch ? ` (${labels.mismatch})` : ''
  return `[${scope}] ${profile.name}${mismatch}`
}
