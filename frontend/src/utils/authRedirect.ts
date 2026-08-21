const AUTH_CONTROL_PATHS = new Set([
  '/login',
  '/register',
  '/email-verify',
  '/forgot-password',
  '/reset-password',
  '/setup',
])

/**
 * Accept only same-origin application destinations that cannot re-enter an
 * authentication callback or credential-handling route.
 */
export function sanitizeAuthRedirect(
  value: unknown,
  fallback = '/dashboard'
): string {
  if (typeof value !== 'string') return fallback

  const candidate = value.trim()
  if (!candidate.startsWith('/') || candidate.startsWith('//') || hasControlCharacter(candidate)) {
    return fallback
  }

  let parsed: URL
  try {
    parsed = new URL(candidate, 'https://sub2api.invalid')
  } catch {
    return fallback
  }
  if (parsed.origin !== 'https://sub2api.invalid') return fallback

  let pathname = parsed.pathname
  let decodedCandidate: string
  try {
    pathname = decodeURIComponent(pathname)
    decodedCandidate = decodeURIComponent(candidate)
  } catch {
    return fallback
  }
  if (
    hasControlCharacter(decodedCandidate) ||
    pathname.startsWith('//') ||
    pathname.startsWith('/\\')
  ) {
    return fallback
  }
  const normalizedPath = pathname.replace(/\/+$/, '').toLowerCase() || '/'
  if (
    normalizedPath === '/auth' ||
    normalizedPath.startsWith('/auth/') ||
    AUTH_CONTROL_PATHS.has(normalizedPath)
  ) {
    return fallback
  }

  return candidate
}

function hasControlCharacter(value: string): boolean {
  return Array.from(value).some((character) => {
    const code = character.charCodeAt(0)
    return code <= 0x1f || code === 0x7f
  })
}
