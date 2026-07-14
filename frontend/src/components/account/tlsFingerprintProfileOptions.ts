export interface TLSFingerprintProfileOption {
  id: number
  name: string
  platform?: string | null
  transport?: string | null
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

// Matches backend tlsFingerprintProfileTransportMatches:
// empty profile/context = agnostic; coarse http/websocket include H1 and H2 variants.
// Backend DoWithTLS can replay h2 / websocket-h2 when http2_fingerprint is present.
function tlsFingerprintTransportMatchesContext(profileTransport: unknown, contextTransport: unknown): boolean {
  const profile = normalizeDimensionValue(profileTransport)
  const context = normalizeDimensionValue(contextTransport)
  if (!profile || !context) {
    return true
  }
  if (profile === context) {
    return true
  }
  if (context === 'http') {
    return profile === 'http1' || profile === 'h2'
  }
  if (context === 'websocket') {
    return profile === 'websocket-http1' || profile === 'websocket-h2'
  }
  return false
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
      if (normalizedSelectedID != null && profile.id === normalizedSelectedID) {
        return true
      }
      if (!tlsFingerprintTransportMatchesContext(profile.transport, '')) {
        return false
      }
      if (!profile.platform || profile.platform === normalizedAccountPlatform) {
        return true
      }
      return false
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
  transport?: string | null
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
  const wantTransport = normalizeDimensionValue(filter.transport)
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
    if (!tlsFingerprintTransportMatchesContext(profile.transport, wantTransport)) {
      return false
    }
    if (wantOS) {
      const pOS = normalizeDimensionValue(profile.os)
      if (pOS && pOS !== wantOS) {
        return false
      }
    }
    const pClient = normalizeDimensionValue(profile.client_type)
    if (wantClient) {
      if (pClient && pClient !== wantClient) {
        return false
      }
    } else if (pClient) {
      return false
    }
    return true
  })
}
