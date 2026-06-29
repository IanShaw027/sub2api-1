export interface TLSFingerprintProfileOption {
  id: number
  name: string
  platform?: string | null
  os?: string | null
  client_type?: string | null
}

export interface SelectableTLSFingerprintProfileOption extends TLSFingerprintProfileOption {
  platform: string
  isPlatformMismatch: boolean
}

export function normalizeTLSFingerprintProfilePlatform(platform: unknown): string {
  return typeof platform === 'string' ? platform.trim().toLowerCase() : ''
}

function normalizeDimensionValue(value: unknown): string {
  return typeof value === 'string' ? value.trim().toLowerCase() : ''
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

export interface TLSFingerprintProfileDimensionFilter {
  platform?: string | null
  os?: string | null
  clientType?: string | null
}

// getTLSFingerprintProfilesForDimension filters profiles for a specific
// platform + os(+client_type) binding slot. A profile matches a dimension when
// the profile's own value is empty (agnostic/shared) or equals the requested
// value. The currently-selected profile is always kept so existing bindings
// never silently disappear.
export function getTLSFingerprintProfilesForDimension(
  profiles: TLSFingerprintProfileOption[],
  filter: TLSFingerprintProfileDimensionFilter,
  selectedProfileID: number | null | undefined
): TLSFingerprintProfileOption[] {
  const wantPlatform = normalizeTLSFingerprintProfilePlatform(filter.platform)
  const wantOS = normalizeDimensionValue(filter.os)
  const wantClient = normalizeDimensionValue(filter.clientType)
  const normalizedSelectedID = typeof selectedProfileID === 'number' && Number.isFinite(selectedProfileID)
    ? selectedProfileID
    : null

  return profiles.filter((profile) => {
    if (normalizedSelectedID != null && profile.id === normalizedSelectedID) {
      return true
    }
    const pPlatform = normalizeTLSFingerprintProfilePlatform(profile.platform)
    if (pPlatform && wantPlatform && pPlatform !== wantPlatform) {
      return false
    }
    if (wantOS) {
      const pOS = normalizeDimensionValue(profile.os)
      if (pOS && pOS !== wantOS) {
        return false
      }
    }
    if (wantClient) {
      const pClient = normalizeDimensionValue(profile.client_type)
      if (pClient && pClient !== wantClient) {
        return false
      }
    }
    return true
  })
}
