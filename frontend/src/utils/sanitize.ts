import DOMPurify from 'dompurify'

export function sanitizeSvg(svg: string): string {
  if (!svg) return ''
  return DOMPurify.sanitize(svg, { USE_PROFILES: { svg: true, svgFilters: true } })
}

export function sanitizeHtml(html: string): string {
  if (!html) return ''
  return DOMPurify.sanitize(html)
}

/**
 * Coerce an untrusted redirect value (e.g. route query) into a same-site path.
 * Accepts unknown because Vue Router query values may be string | string[] | null.
 * Never throws; unsafe or unusable values fall back to `fallback`.
 */
export function sanitizeRedirectPath(path: unknown, fallback = '/dashboard'): string {
  let candidate: string | null = null

  if (typeof path === 'string') {
    candidate = path
  } else if (Array.isArray(path)) {
    const first = path.find((item): item is string => typeof item === 'string')
    candidate = first ?? null
  }

  if (!candidate) return fallback

  const trimmed = candidate.trim()
  if (!trimmed) return fallback
  if (!trimmed.startsWith('/')) return fallback
  if (trimmed.startsWith('//')) return fallback
  if (trimmed.includes('://')) return fallback
  if (trimmed.includes('\n') || trimmed.includes('\r')) return fallback

  return trimmed
}
